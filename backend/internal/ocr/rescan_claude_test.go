package ocr

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/themylogin/ownocr/backend/internal/books"
	"github.com/themylogin/ownocr/backend/internal/objectstore"
)

// claudeAPI answers like the Anthropic messages API. It hands back the text the
// test names, so a rescan runs with no metered call.
type claudeAPI struct {
	server *httptest.Server

	mu       sync.Mutex
	requests int
	keys     []string

	// text is what the model answers for every page. A test that needs one page
	// to fail names it in refuse or narrate.
	text string

	// refuse makes the model refuse every page, and narrate makes it describe the
	// page instead of reading it.
	refuse  bool
	narrate bool

	// status, when set, is the code the API answers with instead of a reading.
	status int
}

func newClaudeAPI(t *testing.T) *claudeAPI {
	t.Helper()

	api := &claudeAPI{text: "the first line of this page of text"}
	api.server = httptest.NewServer(http.HandlerFunc(api.answer))
	t.Cleanup(api.server.Close)
	return api
}

func (a *claudeAPI) answer(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)

	a.mu.Lock()
	a.requests++
	a.keys = append(a.keys, r.Header.Get("x-api-key"))
	a.mu.Unlock()

	if a.status != 0 {
		http.Error(w, `{"error": {"message": "no"}}`, a.status)
		return
	}

	if a.refuse {
		io.WriteString(w, `{"content": [], "stop_reason": "refusal", "usage": {}}`)
		return
	}

	text := a.text
	if a.narrate {
		// No letter of it pairs with a letter of the page, which is what an
		// answer describing some other thing amounts to.
		text = strings.Repeat("9184 7265 3810 5472 6093 ", 6)
	}
	if !a.narrate && !strings.Contains(string(body), "image") {
		text = "the request carried no image"
	}

	answer, _ := json.Marshal(map[string]any{
		"content":     []map[string]string{{"type": "text", "text": text}},
		"stop_reason": "end_turn",
		"usage":       map[string]int{"input_tokens": 4784, "output_tokens": 400},
	})
	w.Write(answer)
}

func (a *claudeAPI) count() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.requests
}

// ultraHarness is the rescan harness with the fake metered API behind it.
type ultraHarness struct {
	*rescanHarness
	claude *claudeAPI
}

func newUltraHarness(t *testing.T) *ultraHarness {
	t.Helper()

	h := newHarness(t)
	h.profile = ProfileNormal
	api := newClaudeAPI(t)

	return &ultraHarness{
		rescanHarness: &rescanHarness{
			harness: h,
			rescan: NewRescanWorker(h.store, h.files, h.hub, h.objects, Settings{
				Settings: objectstore.Settings{
					Bucket:          "ownocr",
					Region:          "us-east-1",
					AccessKeyID:     "key",
					SecretAccessKey: "secret",
				},
				APIURL:         h.endpoint.server.URL,
				APIToken:       "token",
				ClaudeAPIURL:   api.server.URL,
				ClaudeAPIToken: "claude-key",
			}),
		},
		claude: api,
	}
}

// The Ultra rescan runs wholly in the server process: no bucket, no endpoint.
func TestUltraRescanReachesNoBucketAndNoEndpoint(t *testing.T) {
	h := newUltraHarness(t)
	book := h.readBook(t, 4)
	h.objects.puts = nil
	h.endpoint.modes = nil

	if err := h.runRescan(book.ID, []int{2, 3}, ProfileUltra, 1, 3); err != nil {
		t.Fatalf("the Ultra rescan failed: %v", err)
	}

	if len(h.objects.puts) != 0 {
		t.Errorf("the Ultra rescan uploaded %v", h.objects.puts)
	}
	if h.endpoint.modes != nil {
		t.Errorf("the Ultra rescan started an endpoint job asking for %v", h.endpoint.modes)
	}
	if h.claude.count() != 2 {
		t.Errorf("the Ultra rescan sent %d requests, want one per named page", h.claude.count())
	}
}

func TestUltraRescanStoresItsResults(t *testing.T) {
	h := newUltraHarness(t)
	book := h.readBook(t, 3)

	before := h.results(t, book)[2][ModeV6].Text

	if err := h.runRescan(book.ID, []int{2}, ProfileUltra, 1, 3); err != nil {
		t.Fatalf("the Ultra rescan failed: %v", err)
	}

	models := modelsOf(t, h.rescanHarness, book, 2)
	want := []string{ModeClaude, ModelClaudeMerged, ModeV6}
	sort.Strings(want)
	if strings.Join(models, ",") != strings.Join(want, ",") {
		t.Errorf("the page holds %v, want %v", models, want)
	}

	// The line reading it merged with is untouched.
	if after := h.results(t, book)[2][ModeV6].Text; after != before {
		t.Errorf("the %s result changed under the rescan", ModeV6)
	}

	// A page the rescan did not name gains nothing.
	if models := modelsOf(t, h.rescanHarness, book, 1); len(models) != 1 {
		t.Errorf("a page the rescan did not name holds %v", models)
	}
}

// The merged Ultra reading outranks the line reading, so the page answers it.
func TestUltraRescanMakesThePageAnswerItsReading(t *testing.T) {
	h := newUltraHarness(t)
	book := h.readBook(t, 2)

	if err := h.runRescan(book.ID, []int{1}, ProfileUltra, 1, 3); err != nil {
		t.Fatalf("the Ultra rescan failed: %v", err)
	}

	pages, err := h.store.Pages(context.Background(), book.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}

	answered, held, err := h.store.PageReading(context.Background(), pages[0].ID, RenderModels)
	if err != nil || !held {
		t.Fatalf("PageReading: %v, held %v", err, held)
	}
	if answered.Model != ModelClaudeMerged {
		t.Errorf("the page answers %q, want the merged Ultra reading", answered.Model)
	}
}

// A page the model would not read keeps every result it held, and the other pages
// still gain theirs.
func TestUltraRescanLeavesAPageItCouldNotRead(t *testing.T) {
	h := newUltraHarness(t)
	book := h.readBook(t, 2)

	h.claude.refuse = true
	if err := h.runRescan(book.ID, []int{1}, ProfileUltra, 1, 3); err != nil {
		t.Fatalf("the Ultra rescan failed: %v", err)
	}

	if models := modelsOf(t, h.rescanHarness, book, 1); len(models) != 1 || models[0] != ModeV6 {
		t.Errorf("a refused page holds %v, want the line reading alone", models)
	}

	// The book is whole again, and was never failed.
	if reloaded := h.reload(t, book.ID); reloaded.State != books.StateComplete {
		t.Errorf("the book is %q after a refused page", reloaded.State)
	}

	h.claude.refuse = false
	if err := h.runRescan(book.ID, []int{2}, ProfileUltra, 1, 3); err != nil {
		t.Fatalf("the second Ultra rescan failed: %v", err)
	}
	if models := modelsOf(t, h.rescanHarness, book, 2); len(models) != 3 {
		t.Errorf("the page named next holds %v, want three results", models)
	}
}

// A model that describes the page instead of reading it answers text that aligns
// with no line. Storing it would name a reading holding none of its own text.
//
// The page needs lines long enough to be measured: the merge accepts a line
// shorter than 20 characters without comparing the two readings at all, and the
// lines the fake endpoint writes are shorter than that.
func TestUltraRescanRefusesAnAnswerThatIsNoTranscription(t *testing.T) {
	h := newUltraHarness(t)
	book := h.readBook(t, 2)
	h.giveThePageLongLines(t, book, 1)

	h.claude.narrate = true
	if err := h.runRescan(book.ID, []int{1}, ProfileUltra, 1, 3); err != nil {
		t.Fatalf("the Ultra rescan failed: %v", err)
	}

	if models := modelsOf(t, h.rescanHarness, book, 1); len(models) != 1 || models[0] != ModeV6 {
		t.Errorf("a narrated page holds %v, want the line reading alone", models)
	}
}

// The key opens the API and reaches nothing else: no stored result, no object.
func TestTheClaudeKeyReachesTheAPIAlone(t *testing.T) {
	h := newUltraHarness(t)
	book := h.readBook(t, 2)
	h.objects.puts = nil

	if err := h.runRescan(book.ID, []int{1}, ProfileUltra, 1, 3); err != nil {
		t.Fatalf("the Ultra rescan failed: %v", err)
	}

	if len(h.claude.keys) == 0 {
		t.Fatal("the API was never called")
	}
	for _, key := range h.claude.keys {
		if key != "claude-key" {
			t.Errorf("the API was opened with %q", key)
		}
	}

	for model, held := range h.results(t, book)[1] {
		if strings.Contains(held.Text, "claude-key") ||
			strings.Contains(string(held.Data), "claude-key") {
			t.Errorf("the key is stored in the %s result", model)
		}
	}
	for _, key := range h.objects.puts {
		t.Errorf("the Ultra rescan uploaded %q", key)
	}
}

// An API that answers nothing at all is a failure of the job and not a page: it
// must be visible rather than reported as a rescan that read no page.
func TestUltraRescanFailsTheJobWhenTheAPIIsDown(t *testing.T) {
	h := newUltraHarness(t)
	book := h.readBook(t, 2)

	h.claude.status = http.StatusUnauthorized

	err := h.runRescan(book.ID, []int{1}, ProfileUltra, 1, 3)
	if err == nil {
		t.Fatal("a rescan whose API refused every request reported success")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error %q does not name the answer of the API", err)
	}

	// On the last attempt the book returns to complete and is never failed.
	if err := h.runRescan(book.ID, []int{1}, ProfileUltra, 3, 3); err == nil {
		t.Fatal("the last attempt reported success")
	}
	if reloaded := h.reload(t, book.ID); reloaded.State != books.StateComplete {
		t.Errorf("the book is %q after the last attempt", reloaded.State)
	}
}

// A normal rescan replaces the line boxes, so the merged Ultra reading no longer
// describes the page. The transcription cost money and survives, and its merged
// result is written again from the new boxes.
func TestANormalRescanKeepsTheUltraTextAndRebuildsItsMerge(t *testing.T) {
	h := newUltraHarness(t)
	book := h.readBookWithBothEngines(t, 2)

	if err := h.runRescan(book.ID, []int{1}, ProfileUltra, 1, 3); err != nil {
		t.Fatalf("the Ultra rescan failed: %v", err)
	}
	before := modelsOf(t, h.rescanHarness, book, 1)
	if len(before) != 5 {
		t.Fatalf("the page holds %v, want five results before the normal rescan", before)
	}

	ultraText := h.results(t, book)[1][ModeClaude].Text
	requests := h.claude.count()

	if err := h.runRescan(book.ID, []int{1}, ProfileNormal, 1, 3); err != nil {
		t.Fatalf("the normal rescan failed: %v", err)
	}

	models := modelsOf(t, h.rescanHarness, book, 1)
	want := []string{ModeClaude, ModelClaudeMerged, ModeV6}
	sort.Strings(want)
	if strings.Join(models, ",") != strings.Join(want, ",") {
		t.Errorf("the page holds %v, want %v", models, want)
	}

	if after := h.results(t, book)[1][ModeClaude].Text; after != ultraText {
		t.Errorf("the transcription changed: %q", after)
	}
	if h.claude.count() != requests {
		t.Errorf("the normal rescan sent %d requests to the metered API",
			h.claude.count()-requests)
	}
}


// giveThePageLongLines replaces the line reading of one page with lines long
// enough that the merge measures how well the two readings agree.
func (h *ultraHarness) giveThePageLongLines(t *testing.T, book books.Book, index int) {
	t.Helper()
	ctx := context.Background()

	pages, err := h.store.Pages(ctx, book.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}

	for _, page := range pages {
		if page.PageIndex != index {
			continue
		}
		err := h.store.InsertPageOCR(ctx, books.PageOCR{
			PageID: page.ID, Model: ModeV6,
			Text: "the first long line of this printed page\nthe second long line of that page",
			Data: []byte(`{"raw": {}, "lines": [
				{"bbox": [100, 100, 900, 140],
				 "text": "the first long line of this printed page", "score": 0.99},
				{"bbox": [100, 150, 900, 190],
				 "text": "the second long line of that page", "score": 0.98}
			]}`),
		}, RenderModels)
		if err != nil {
			t.Fatalf("InsertPageOCR: %v", err)
		}
		return
	}
	t.Fatalf("the book holds no page %d", index)
}
