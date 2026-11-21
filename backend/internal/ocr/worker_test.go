package ocr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/themylogin/ownocr/backend/internal/books"
	"github.com/themylogin/ownocr/backend/internal/filestore"
	"github.com/themylogin/ownocr/backend/internal/ingest"
	"github.com/themylogin/ownocr/backend/internal/objectstore"
	"github.com/themylogin/ownocr/backend/internal/testdb"
	"github.com/themylogin/ownocr/backend/internal/ws"
)

type fakeObjects struct {
	data map[string][]byte
	puts []string
}

func newFakeObjects() *fakeObjects {
	return &fakeObjects{data: map[string][]byte{}}
}

func (f *fakeObjects) PutFile(_ context.Context, key, path string) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	f.data[key] = body
	f.puts = append(f.puts, key)
	return nil
}

func (f *fakeObjects) GetFile(_ context.Context, key, target string) error {
	body, ok := f.data[key]
	if !ok {
		return fmt.Errorf("no such object %s", key)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return os.WriteFile(target, body, 0o644)
}

func (f *fakeObjects) DeletePrefix(_ context.Context, prefix string) error {
	for key := range f.data {
		if strings.HasPrefix(key, prefix) {
			delete(f.data, key)
		}
	}
	return nil
}

func (f *fakeObjects) keys(prefix string) []string {
	found := make([]string, 0, len(f.data))
	for key := range f.data {
		if strings.HasPrefix(key, prefix) {
			found = append(found, key)
		}
	}
	sort.Strings(found)
	return found
}

// textOf is what the fake endpoint writes for an input object, per engine. A
// test reads it back out of the stored results of the copy. The two engines
// write different shapes: v6 writes plain lines, VL writes markdown.
func textOf(inputKey, mode string) string {
	if mode == ModeV6 {
		return path.Base(inputKey) + "\nthe text of this page\n"
	}
	return "# " + path.Base(inputKey) + "\n\nthe text of this page\n"
}

// dataOf answers a result of the shape the merge reads: v6 reports a box per
// line, and VL reports a block whose box holds those lines and whose text reads
// the same two lines reflowed.
func dataOf(mode string) string {
	if mode == ModeV6 {
		return `{"raw": {}, "lines": [
			{"bbox": [100, 100, 900, 140], "text": "the first line of", "score": 0.99},
			{"bbox": [100, 150, 900, 190], "text": "this page of text", "score": 0.98}
		]}`
	}
	return `{"parsing_res_list": [{"block_label": "text",
		"block_bbox": [90, 90, 910, 200],
		"block_content": "the first line of this page of text"}]}`
}

// dataWithoutBlocks is a VL result that found no block, which every line of the
// box engine must survive.
const dataWithoutBlocks = `{"parsing_res_list": []}`

// endpoint answers like the RunPod serverless API. It reads a page by writing
// one markdown object for every input object.
type endpoint struct {
	objects *fakeObjects
	server  *httptest.Server

	dropLast  bool
	failPage  string
	emptyPage string
	// blocklessPage names a page the text engine reads without finding a block,
	// which every line of the box engine has to survive.
	blocklessPage string
	// loseObject names a page the endpoint reports as read, without writing
	// its object. A caller that trusts the result alone would build a copy
	// with a hole in it.
	loseObject string
	runCode    int
	status     string
	output     Output

	// modes records what the last job asked for.
	modes []string

	// inProgress answers this many polls with IN_PROGRESS before the real
	// status, so a test can watch a job that actually runs.
	inProgress int
}

func newEndpoint(t *testing.T, objects *fakeObjects) *endpoint {
	t.Helper()

	e := &endpoint{objects: objects, status: "COMPLETED"}
	mux := http.NewServeMux()
	mux.HandleFunc("/run", e.run)
	mux.HandleFunc("/status/", e.statusOf)
	e.server = httptest.NewServer(mux)
	t.Cleanup(e.server.Close)

	return e
}

func (e *endpoint) run(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer token" {
		http.Error(w, "no token", http.StatusUnauthorized)
		return
	}
	if e.runCode != 0 {
		http.Error(w, "endpoint is down", e.runCode)
		return
	}

	var body struct {
		Input JobInput `json:"input"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// The real handler refuses a job that names no engine.
	if len(body.Input.Modes) == 0 {
		http.Error(w, "'modes' is required", http.StatusBadRequest)
		return
	}
	e.modes = body.Input.Modes

	e.output = Output{}
	for _, key := range e.objects.keys(body.Input.InputPrefix) {
		name := strings.TrimSuffix(path.Base(key), path.Ext(key))

		if e.failPage != "" && name == e.failPage {
			e.output.Failed = append(e.output.Failed,
				OutputFailure{InputKey: key, Error: "cannot identify image file"})
			continue
		}

		// One entry per engine per page, as the handler answers.
		for _, mode := range body.Input.Modes {
			text := textOf(key, mode)
			if e.emptyPage != "" && name == e.emptyPage {
				text = ""
			}

			textName, dataName := name+".md", name+".json"
			if mode == ModeV6 {
				textName, dataName = name+".v6.txt", name+".v6.json"
			}
			textKey := body.Input.OutputPrefix + textName
			dataKey := body.Input.OutputPrefix + dataName

			data := dataOf(mode)
			if mode == ModeVL && name == e.blocklessPage {
				data = dataWithoutBlocks
			}

			if name != e.loseObject {
				e.objects.data[textKey] = []byte(text)
				e.objects.data[dataKey] = []byte(data)
			}
			e.output.Pages = append(e.output.Pages, OutputPage{
				InputKey: key, OutputKeys: []string{textKey, dataKey},
				Chars: len(text), Seconds: 1.5,
			})
		}
	}

	// Dropping one entry leaves that engine one page short, which is what a
	// per engine count must catch.
	if e.dropLast && len(e.output.Pages) > 0 {
		e.output.Pages = e.output.Pages[:len(e.output.Pages)-1]
	}

	writeJSON(w, map[string]any{"id": "job-1", "status": "IN_QUEUE"})
}

func (e *endpoint) statusOf(w http.ResponseWriter, _ *http.Request) {
	if e.inProgress > 0 {
		e.inProgress--
		writeJSON(w, map[string]any{"id": "job-1", "status": "IN_PROGRESS"})
		return
	}

	answer := map[string]any{"id": "job-1", "status": e.status}
	if e.status == "COMPLETED" {
		answer["output"] = e.output
	} else {
		answer["error"] = "the pod died"
	}
	writeJSON(w, answer)
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func jpegBytes(width, height int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 128, A: 255})
		}
	}

	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, img, nil); err != nil {
		panic(err)
	}
	return buffer.Bytes()
}

type harness struct {
	store    *books.Store
	files    *filestore.Store
	objects  *fakeObjects
	endpoint *endpoint
	hub      *ws.Hub
	work     *Worker

	// profile is what the copy carries. Extra runs both engines, which is the
	// case that returns two results per page.
	profile string
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	pool := testdb.New(t)

	files, err := filestore.New(t.TempDir())
	if err != nil {
		t.Fatalf("filestore: %v", err)
	}

	objects := newFakeObjects()
	h := &harness{
		profile:  ProfileExtra,
		store:    books.NewStore(pool),
		files:    files,
		objects:  objects,
		endpoint: newEndpoint(t, objects),
		hub:      ws.NewHub([]string{"*"}),
	}

	h.work = NewWorker(h.store, h.files, h.hub, objects, Settings{
		Settings: objectstore.Settings{
			Bucket:          "ownocr",
			Region:          "us-east-1",
			AccessKeyID:     "key",
			SecretAccessKey: "secret",
		},
		APIURL:   h.endpoint.server.URL,
		APIToken: "token",
	})

	return h
}

// seed makes a complete source book with real page files, then the copy that an
// OCR job fills.
// seed builds a complete source book and the OCR copy of it. The copy carries
// the profile the harness holds, which is Extra unless a test changes it.
func (h *harness) seed(t *testing.T, pageCount int) (books.Book, books.Book) {
	t.Helper()
	ctx := context.Background()

	sourceKey, err := books.NewStorageKey()
	if err != nil {
		t.Fatalf("storage key: %v", err)
	}

	tx, err := h.store.Pool().Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	source, err := h.store.CreateTx(ctx, tx, "Atlas", sourceKey)
	if err != nil {
		t.Fatalf("CreateTx: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	pageDir := h.files.PageDir(sourceKey)
	for index := 1; index <= pageCount; index++ {
		raw := filepath.Join(t.TempDir(), fmt.Sprintf("page%d.jpg", index))
		if err := os.WriteFile(raw, jpegBytes(400, 600), 0o644); err != nil {
			t.Fatalf("write page: %v", err)
		}

		pageKey, err := books.NewStorageKey()
		if err != nil {
			t.Fatalf("storage key: %v", err)
		}
		ext, width, height, err := ingest.WriteRenditions(raw, pageDir, pageKey)
		if err != nil {
			t.Fatalf("WriteRenditions: %v", err)
		}
		err = h.store.InsertPage(ctx, books.Page{
			BookID: source.ID, PageIndex: index, StorageKey: pageKey,
			OriginalExt: ext, Width: width, Height: height,
		})
		if err != nil {
			t.Fatalf("InsertPage: %v", err)
		}
	}

	source, err = h.store.SetComplete(ctx, source.ID, pageCount)
	if err != nil {
		t.Fatalf("SetComplete: %v", err)
	}

	copyKey, err := books.NewStorageKey()
	if err != nil {
		t.Fatalf("storage key: %v", err)
	}

	tx, err = h.store.Pool().Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	action, ok := books.ActionByID(books.ActionOCR)
	if !ok {
		t.Fatal("the ocr action is missing")
	}
	copyBook, err := h.store.CreateDerivedTx(ctx, tx, source.Name, copyKey, source.ID,
		action, h.profile)
	if err != nil {
		t.Fatalf("CreateDerivedTx: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	return source, copyBook
}

func (h *harness) run(bookID int64, attempt, maxAttempts int) error {
	job := &river.Job[Args]{
		JobRow: &rivertype.JobRow{Attempt: attempt, MaxAttempts: maxAttempts},
		Args:   Args{BookID: bookID},
	}
	return h.work.Work(context.Background(), job)
}

// results answers the stored result of every page of a book, by page index and
// then by engine.
func (h *harness) results(t *testing.T, book books.Book) map[int]map[string]books.PageOCR {
	t.Helper()
	ctx := context.Background()

	pages, err := h.store.Pages(ctx, book.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}

	found := make(map[int]map[string]books.PageOCR, len(pages))
	for _, page := range pages {
		rows, err := h.store.PageOCRResults(ctx, page.ID)
		if err != nil {
			t.Fatalf("PageOCRResults: %v", err)
		}
		byModel := make(map[string]books.PageOCR, len(rows))
		for _, row := range rows {
			byModel[row.Model] = row
		}
		found[page.PageIndex] = byModel
	}
	return found
}

// engineCount counts the results an engine produced, leaving out any result
// derived from them.
func engineCount(byModel map[string]books.PageOCR) int {
	count := 0
	for model := range byModel {
		if model != ModelMerged {
			count++
		}
	}
	return count
}

func TestWorkerUploadsEveryPageWithAPaddedName(t *testing.T) {
	h := newHarness(t)
	_, copyBook := h.seed(t, 3)

	if err := h.run(copyBook.ID, 1, 3); err != nil {
		t.Fatalf("Work: %v", err)
	}

	want := []string{
		InputPrefix(copyBook.StorageKey) + "0001.jpg",
		InputPrefix(copyBook.StorageKey) + "0002.jpg",
		InputPrefix(copyBook.StorageKey) + "0003.jpg",
	}
	if len(h.objects.puts) != len(want) {
		t.Fatalf("uploaded %v, want %v", h.objects.puts, want)
	}
	for i, key := range want {
		if h.objects.puts[i] != key {
			t.Errorf("upload %d = %q, want %q", i, h.objects.puts[i], key)
		}
	}
}

func TestWorkerFillsTheCopyAndClearsThePrefix(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	source, copyBook := h.seed(t, 3)

	if err := h.run(copyBook.ID, 1, 3); err != nil {
		t.Fatalf("Work: %v", err)
	}

	updated, err := h.store.Get(ctx, copyBook.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if updated.State != books.StateComplete {
		t.Errorf("State = %q, want complete", updated.State)
	}
	if updated.Stage != books.StageOCR {
		t.Errorf("Stage = %q, want %q", updated.Stage, books.StageOCR)
	}
	if updated.Progress != nil {
		t.Errorf("a complete copy kept the phase %q", *updated.Progress)
	}
	if updated.PageCount != 3 {
		t.Errorf("PageCount = %d, want 3", updated.PageCount)
	}

	pages, err := h.store.Pages(ctx, copyBook.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}
	if len(pages) != 3 {
		t.Fatalf("got %d pages, want 3", len(pages))
	}

	sourcePages, err := h.store.Pages(ctx, source.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}
	for i, page := range pages {
		if page.PageIndex != i+1 {
			t.Errorf("page %d has index %d", i, page.PageIndex)
		}
		if page.Width != sourcePages[i].Width || page.Height != sourcePages[i].Height {
			t.Errorf("page %d = %dx%d, want the source size %dx%d", page.PageIndex,
				page.Width, page.Height, sourcePages[i].Width, sourcePages[i].Height)
		}
		// The copy shows the image of its source page, so it keeps that key
		// and names the book that holds the file.
		if page.StorageKey != sourcePages[i].StorageKey {
			t.Errorf("page %d has the key %q, want the source key %q",
				page.PageIndex, page.StorageKey, sourcePages[i].StorageKey)
		}
		if page.ImageBookKey == nil || *page.ImageBookKey != source.StorageKey {
			t.Errorf("page %d names the image book %v, want %q",
				page.PageIndex, page.ImageBookKey, source.StorageKey)
		}
	}

	assertLineage(t, ctx, h.store, source.ID, pages)

	// OCR reads a page without changing it, so the copy writes no image and
	// creates no directory of its own.
	if entries, err := os.ReadDir(h.files.PageDir(copyBook.StorageKey)); !os.IsNotExist(err) {
		t.Errorf("the copy holds %d files, want a directory that was never made (%v)",
			len(entries), err)
	}
	if entries, err := os.ReadDir(h.files.PageDir(source.StorageKey)); err != nil {
		t.Fatalf("read the source page dir: %v", err)
	} else if len(entries) != 9 {
		t.Errorf("the source book holds %d files, want 9 (3 pages x 3 renditions)",
			len(entries))
	}

	// Extra runs both engines, so every page holds one result per engine, each
	// the text that engine wrote. The merged result derived from them is not one
	// of them and is checked on its own.
	results := h.results(t, copyBook)
	for index := 1; index <= 3; index++ {
		byModel := results[index]
		if got := engineCount(byModel); got != 2 {
			t.Fatalf("page %d holds %d engine results, want one per engine", index, got)
		}
		for _, mode := range []string{ModeV6, ModeVL} {
			want := textOf(InputKey(copyBook.StorageKey, index, "jpg"), mode)
			if byModel[mode].Text != want {
				t.Errorf("page %d, %s holds %q, want %q",
					index, mode, byModel[mode].Text, want)
			}
			if len(byModel[mode].Data) == 0 {
				t.Errorf("page %d, %s stored no result", index, mode)
			}
		}
	}

	if left := h.objects.keys(Prefix(copyBook.StorageKey)); len(left) != 0 {
		t.Errorf("the prefix still holds %v", left)
	}

	unchanged, err := h.store.Get(ctx, source.ID)
	if err != nil {
		t.Fatalf("Get source: %v", err)
	}
	if unchanged.State != books.StateComplete || unchanged.Stage != books.StageUploaded {
		t.Errorf("the source book changed: %+v", unchanged)
	}
	if unchanged.DerivedState == nil || *unchanged.DerivedState != books.StateComplete {
		t.Errorf("the source book reports the copy state %v, want complete", unchanged.DerivedState)
	}

	sourceEntries, err := os.ReadDir(h.files.PageDir(source.StorageKey))
	if err != nil {
		t.Fatalf("read source page dir: %v", err)
	}
	if len(sourceEntries) != 9 {
		t.Errorf("the source book holds %d files, want its own 9", len(sourceEntries))
	}

	// The two pass download must leave nothing behind in the temp root.
	temp, err := os.ReadDir(h.files.TempRoot())
	if err != nil {
		t.Fatalf("read temp root: %v", err)
	}
	if len(temp) != 0 {
		t.Errorf("the temp root still holds %d entries", len(temp))
	}
}

// A page the endpoint finds no text on still belongs to the copy.
func TestWorkerStoresAnEmptyPage(t *testing.T) {
	h := newHarness(t)
	_, copyBook := h.seed(t, 2)
	h.endpoint.emptyPage = "0002"

	if err := h.run(copyBook.ID, 1, 3); err != nil {
		t.Fatalf("Work: %v", err)
	}

	results := h.results(t, copyBook)
	if got := engineCount(results[2]); got != 2 {
		t.Fatalf("page 2 holds %d engine results, want one per engine", got)
	}
	for _, mode := range []string{ModeV6, ModeVL} {
		if results[2][mode].Text != "" {
			t.Errorf("page 2, %s holds %q, want an empty text",
				mode, results[2][mode].Text)
		}
	}

	book, err := h.store.Get(context.Background(), copyBook.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if book.State != books.StateComplete {
		t.Errorf("State = %q, want complete", book.State)
	}
}

func TestWorkerFailsWhenAPageFails(t *testing.T) {
	h := newHarness(t)
	_, copyBook := h.seed(t, 3)
	h.endpoint.failPage = "0002"

	err := h.run(copyBook.ID, 3, 3)
	if err == nil {
		t.Fatal("a failed page did not fail the job")
	}
	if !strings.Contains(err.Error(), "page 2") {
		t.Errorf("error %q does not name the failed page", err)
	}

	failed, err := h.store.Get(context.Background(), copyBook.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if failed.State != books.StateFailed {
		t.Errorf("State = %q, want failed", failed.State)
	}
	if failed.Error == nil || !strings.Contains(*failed.Error, "page 2") {
		t.Errorf("stored reason = %v", failed.Error)
	}
	if failed.Stage != books.StageOCR {
		t.Errorf("the failed copy left the %q stage for %q", books.StageOCR, failed.Stage)
	}
	if failed.SourceBookID == nil {
		t.Error("the failed copy lost its source reference")
	}
}

// The endpoint answers one entry per engine per page, so a profile naming two
// engines returns twice the page count. Only a count per engine catches one
// engine that came back a page short.
func TestWorkerFailsWhenOneEngineMissesAPage(t *testing.T) {
	h := newHarness(t)
	_, copyBook := h.seed(t, 3)
	h.endpoint.dropLast = true

	err := h.run(copyBook.ID, 3, 3)
	if err == nil {
		t.Fatal("a short result did not fail the job")
	}
	if !strings.Contains(err.Error(), ModeVL) {
		t.Errorf("error %q does not name the engine that came up short", err)
	}
	if !strings.Contains(err.Error(), "page 3") {
		t.Errorf("error %q does not name the missing page", err)
	}
	if !strings.Contains(err.Error(), "2 of 3") {
		t.Errorf("error %q does not say how many pages arrived", err)
	}
}

// A profile naming two engines returns two entries per page, which is not a
// short result.
func TestWorkerAcceptsTwoResultsPerPage(t *testing.T) {
	h := newHarness(t)
	_, copyBook := h.seed(t, 3)

	if err := h.run(copyBook.ID, 1, 3); err != nil {
		t.Fatalf("a correct two engine result was refused: %v", err)
	}
	if len(h.endpoint.output.Pages) != 6 {
		t.Errorf("the endpoint answered %d entries, want 6 for 3 pages and 2 engines",
			len(h.endpoint.output.Pages))
	}
}

// Normal runs one engine, so a page holds one result and the job asks for v6
// alone.
func TestWorkerRunsTheNormalProfileAlone(t *testing.T) {
	h := newHarness(t)
	h.profile = ProfileNormal
	_, copyBook := h.seed(t, 2)

	if err := h.run(copyBook.ID, 1, 3); err != nil {
		t.Fatalf("Work: %v", err)
	}

	if len(h.endpoint.modes) != 1 || h.endpoint.modes[0] != ModeV6 {
		t.Errorf("the job asked for %v, want the v6 engine alone", h.endpoint.modes)
	}

	results := h.results(t, copyBook)
	for index := 1; index <= 2; index++ {
		if len(results[index]) != 1 {
			t.Fatalf("page %d holds %d results, want one", index, len(results[index]))
		}
		if _, ok := results[index][ModeV6]; !ok {
			t.Errorf("page %d holds no v6 result", index)
		}
	}
}

// A copy whose profile the system does not know names no engines to run, so the
// job fails instead of guessing.
//
// The database refuses such a copy, so the test drops that guard to build one.
// The worker guard is what answers for a row that predates the constraint.
func TestWorkerRefusesAnUnknownProfile(t *testing.T) {
	h := newHarness(t)
	h.profile = ProfileExtra
	_, copyBook := h.seed(t, 1)

	// The whole suite shares one database, so the guard goes back before this
	// test ends. The suite lock is held until then, so no other test can see it
	// missing.
	ctx := context.Background()
	if _, err := h.store.Pool().Exec(ctx,
		`ALTER TABLE book DROP CONSTRAINT IF EXISTS book_ocr_profile_check`); err != nil {
		t.Fatalf("drop the constraint: %v", err)
	}
	t.Cleanup(func() {
		restore := context.Background()
		if _, err := h.store.Pool().Exec(restore,
			`UPDATE book SET ocr_profile = 'vl' WHERE action = 'ocr' AND ocr_profile IS NULL`); err != nil {
			t.Errorf("restore the profiles: %v", err)
		}
		if _, err := h.store.Pool().Exec(restore,
			`ALTER TABLE book ADD CONSTRAINT book_ocr_profile_check CHECK (
				(action IS DISTINCT FROM 'ocr' AND ocr_profile IS NULL)
				OR (action = 'ocr' AND ocr_profile IS NOT NULL
					AND ocr_profile IN ('v6', 'vl')))`); err != nil {
			t.Errorf("restore the constraint: %v", err)
		}
	})

	if _, err := h.store.Pool().Exec(ctx,
		`UPDATE book SET ocr_profile = NULL WHERE id = $1`, copyBook.ID); err != nil {
		t.Fatalf("clear the profile: %v", err)
	}

	err := h.run(copyBook.ID, 3, 3)
	if err == nil {
		t.Fatal("a copy with no profile ran")
	}
	if !strings.Contains(err.Error(), "profile") {
		t.Errorf("error %q does not name the profile", err)
	}
}

// A result that names every page but leaves one object out must not build a
// copy that is missing a page.
func TestWorkerFailsWhenAnOutputObjectIsGone(t *testing.T) {
	h := newHarness(t)
	_, copyBook := h.seed(t, 3)
	h.endpoint.loseObject = "0003"

	err := h.run(copyBook.ID, 3, 3)
	if err == nil {
		t.Fatal("a missing output object did not fail the job")
	}
	if !strings.Contains(err.Error(), "page 3") {
		t.Errorf("error %q does not name the missing page", err)
	}
}

func TestWorkerFailsWhenTheEndpointFails(t *testing.T) {
	h := newHarness(t)
	_, copyBook := h.seed(t, 2)
	h.endpoint.status = "FAILED"

	err := h.run(copyBook.ID, 3, 3)
	if err == nil {
		t.Fatal("a failed endpoint job did not fail the copy")
	}
	if !strings.Contains(err.Error(), "FAILED") || !strings.Contains(err.Error(), "the pod died") {
		t.Errorf("error %q does not repeat the endpoint reason", err)
	}
	if !strings.Contains(err.Error(), "OCR endpoint") {
		t.Errorf("error %q does not name the endpoint", err)
	}
}

func TestWorkerKeepsProcessingWhileAttemptsRemain(t *testing.T) {
	h := newHarness(t)
	_, copyBook := h.seed(t, 2)
	h.endpoint.runCode = http.StatusInternalServerError

	if err := h.run(copyBook.ID, 1, 3); err == nil {
		t.Fatal("a broken endpoint did not fail the attempt")
	}

	book, err := h.store.Get(context.Background(), copyBook.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if book.State != books.StateProcessing {
		t.Errorf("State = %q, want processing while attempts remain", book.State)
	}
	if book.Error != nil {
		t.Errorf("a retryable attempt stored the reason %q", *book.Error)
	}
}

func TestWorkerRetryLeavesOneSetOfPages(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	source, copyBook := h.seed(t, 3)

	h.endpoint.failPage = "0002"
	if err := h.run(copyBook.ID, 3, 3); err == nil {
		t.Fatal("the first run did not fail")
	}

	h.endpoint.failPage = ""
	if err := h.run(copyBook.ID, 1, 3); err != nil {
		t.Fatalf("the retry failed: %v", err)
	}

	pages, err := h.store.Pages(ctx, copyBook.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}
	if len(pages) != 3 {
		t.Fatalf("the retry left %d pages, want 3", len(pages))
	}

	seen := map[int]bool{}
	for _, page := range pages {
		if seen[page.PageIndex] {
			t.Fatalf("page index %d appears twice", page.PageIndex)
		}
		seen[page.PageIndex] = true
	}

	assertLineage(t, ctx, h.store, source.ID, pages)

	// The copy owns no files, so a retry can leave none behind.
	if entries, err := os.ReadDir(h.files.PageDir(copyBook.StorageKey)); !os.IsNotExist(err) {
		t.Errorf("the retry left %d files, want none", len(entries))
	}

	// Deleting the pages of the copy removes their results with them, so a
	// retry cannot leave two results for one engine.
	results := h.results(t, copyBook)
	if len(results) != 3 {
		t.Fatalf("the retry left results for %d pages, want 3", len(results))
	}
	for index, byModel := range results {
		if got := engineCount(byModel); got != 2 {
			t.Errorf("page %d holds %d engine results, want one per engine", index, got)
		}
	}
}

// assertLineage checks that every page of a copy names a distinct page of the
// source book, which is what both page operations follow.
func assertLineage(t *testing.T, ctx context.Context, store *books.Store,
	sourceID int64, pages []books.Page) {
	t.Helper()

	sourcePages, err := store.Pages(ctx, sourceID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}

	byIndex := make(map[int]int64, len(sourcePages))
	for _, page := range sourcePages {
		byIndex[page.PageIndex] = page.ID
	}

	claimed := map[int64]bool{}
	for _, page := range pages {
		if page.SourcePageID == nil {
			t.Fatalf("page %d names no source page", page.PageIndex)
		}
		if want := byIndex[page.PageIndex]; *page.SourcePageID != want {
			t.Errorf("page %d names source page %d, want %d",
				page.PageIndex, *page.SourcePageID, want)
		}
		if claimed[*page.SourcePageID] {
			t.Errorf("source page %d is named twice", *page.SourcePageID)
		}
		claimed[*page.SourcePageID] = true
	}
}

func TestWorkerNeedsASourceBook(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	key, err := books.NewStorageKey()
	if err != nil {
		t.Fatalf("storage key: %v", err)
	}
	tx, err := h.store.Pool().Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	lonely, err := h.store.CreateTx(ctx, tx, "Lonely", key)
	if err != nil {
		t.Fatalf("CreateTx: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	err = h.run(lonely.ID, 3, 3)
	if err == nil || !strings.Contains(err.Error(), "no source book") {
		t.Fatalf("Work returned %v", err)
	}
}

// A user watching an OCR job must see the work move through its phases in
// order. The phases are read off the event stream, the same way a browser reads
// them.
func TestWorkerReportsEveryPhaseInOrder(t *testing.T) {
	h := newHarness(t)
	_, copyBook := h.seed(t, 2)

	h.endpoint.inProgress = 1
	h.work.client.PollInterval = time.Millisecond

	watch := h.watchPhases(t, copyBook.ID)

	if err := h.run(copyBook.ID, 1, 3); err != nil {
		t.Fatalf("Work: %v", err)
	}

	want := []string{
		books.PhaseUploading,
		books.PhaseWaitingAI,
		books.PhaseProcessing,
		books.PhaseDownloading,
		books.PhaseConverting,
	}
	seen := watch()

	if len(seen) != len(want) {
		t.Fatalf("phases = %v, want %v", seen, want)
	}
	for index, phase := range want {
		if seen[index] != phase {
			t.Fatalf("phases = %v, want %v", seen, want)
		}
	}
}

// watchPhases subscribes to the hub and collects the phase of one book in the
// order it is broadcast. The returned function waits for the stream to settle,
// then answers with what it saw.
func (h *harness) watchPhases(t *testing.T, bookID int64) func() []string {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(h.hub.ServeHTTP))
	t.Cleanup(server.Close)

	url := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.Dial(context.Background(), url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.CloseNow() })

	var (
		mu    sync.Mutex
		seen  []string
		ready = make(chan struct{})
	)

	go func() {
		close(ready)
		for {
			_, message, err := conn.Read(context.Background())
			if err != nil {
				return
			}

			var event struct {
				Payload struct {
					ID       int64   `json:"id"`
					Progress *string `json:"progress"`
				} `json:"payload"`
			}
			if json.Unmarshal(message, &event) != nil {
				continue
			}
			if event.Payload.ID != bookID || event.Payload.Progress == nil {
				continue
			}

			mu.Lock()
			if len(seen) == 0 || seen[len(seen)-1] != *event.Payload.Progress {
				seen = append(seen, *event.Payload.Progress)
			}
			mu.Unlock()
		}
	}()

	<-ready

	// The hub registers the subscriber inside ServeHTTP, after the dial
	// returns. Publishing before that would reach nobody.
	deadline := time.Now().Add(2 * time.Second)
	for h.hub.Subscribers() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}

	return func() []string {
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			mu.Lock()
			settled := len(seen) >= 5
			mu.Unlock()
			if settled {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}

		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), seen...)
	}
}
