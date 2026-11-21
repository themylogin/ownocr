package dewarp

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

// endpoint answers like the RunPod serverless API. It dewarps by writing an
// output object of a different size for every input object.
type endpoint struct {
	objects *fakeObjects
	server  *httptest.Server

	dropLast bool
	failPage string
	runCode  int
	status   string
	output   Output

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

	e.output = Output{}
	for _, key := range e.objects.keys(body.Input.InputPrefix) {
		name := strings.TrimSuffix(path.Base(key), path.Ext(key))

		if e.failPage != "" && name == e.failPage {
			e.output.Failed = append(e.output.Failed,
				OutputFailure{InputKey: key, Error: "could not decode"})
			continue
		}

		outputKey := body.Input.OutputPrefix + name + ".jpg"
		e.objects.data[outputKey] = jpegBytes(240, 320)
		e.output.Pages = append(e.output.Pages,
			OutputPage{InputKey: key, OutputKey: outputKey, Width: 240, Height: 320})
	}

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

// seed makes a complete source book with real page files, then the copy that a
// dewarp job fills.
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
	action, ok := books.ActionByID(books.ActionDewarp)
	if !ok {
		t.Fatal("the dewarp action is missing")
	}
	copyBook, err := h.store.CreateDerivedTx(ctx, tx, source.Name, copyKey, source.ID, action, "")
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
	if updated.Stage != books.StageDewarped {
		t.Errorf("Stage = %q, want %q", updated.Stage, books.StageDewarped)
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
	for i, page := range pages {
		if page.PageIndex != i+1 {
			t.Errorf("page %d has index %d", i, page.PageIndex)
		}
		if page.Width != 240 || page.Height != 320 {
			t.Errorf("page %d = %dx%d, want the dewarped 240x320",
				page.PageIndex, page.Width, page.Height)
		}
	}

	assertLineage(t, ctx, h.store, source.ID, pages)

	entries, err := os.ReadDir(h.files.PageDir(copyBook.StorageKey))
	if err != nil {
		t.Fatalf("read page dir: %v", err)
	}
	if len(entries) != 9 {
		t.Errorf("got %d files, want 9 (3 pages x 3 renditions)", len(entries))
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

	// The two pass download must leave nothing behind in the temp root.
	temp, err := os.ReadDir(h.files.TempRoot())
	if err != nil {
		t.Fatalf("read temp root: %v", err)
	}
	if len(temp) != 0 {
		t.Errorf("the temp root still holds %d entries", len(temp))
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
	if failed.SourceBookID == nil {
		t.Error("the failed copy lost its source reference")
	}
}

func TestWorkerFailsWhenAPageIsMissing(t *testing.T) {
	h := newHarness(t)
	_, copyBook := h.seed(t, 3)
	h.endpoint.dropLast = true

	err := h.run(copyBook.ID, 3, 3)
	if err == nil {
		t.Fatal("a short result did not fail the job")
	}
	if !strings.Contains(err.Error(), "2 pages") {
		t.Errorf("error %q does not say how many pages arrived", err)
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

	entries, err := os.ReadDir(h.files.PageDir(copyBook.StorageKey))
	if err != nil {
		t.Fatalf("read page dir: %v", err)
	}
	if len(entries) != 9 {
		t.Errorf("the retry left %d files, want 9", len(entries))
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

// A user watching a dewarp must see the work move through its phases in order.
// The phases are read off the event stream, the same way a browser reads them.
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
