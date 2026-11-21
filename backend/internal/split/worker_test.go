package split

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
	"path/filepath"
	"strings"
	"testing"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/themylogin/ownocr/backend/internal/books"
	"github.com/themylogin/ownocr/backend/internal/filestore"
	"github.com/themylogin/ownocr/backend/internal/ingest"
	"github.com/themylogin/ownocr/backend/internal/testdb"
	"github.com/themylogin/ownocr/backend/internal/ws"
)

// The fake sidecar reports every page as a spread unless a test says otherwise,
// so a source page makes this many copy pages by default.
const pagesPerSource = 2

// fakeSidecar answers where to cut every page of a batch, as the real sidecar
// does, and records what it saw so a test can assert the batching too.
type fakeSidecar struct {
	server  *httptest.Server
	batches [][]string
	// spread decides the answer for one page. A nil answer names a page that is
	// not a spread, which the worker copies whole. The default cuts a 400 by 600
	// page into two overlapping 220 by 600 halves.
	spread func(path string) *Cut
	// refuse makes the sidecar reject the whole batch, as it does when it cannot
	// read one of its pages.
	refuse bool
}

func newFakeSidecar(t *testing.T) *fakeSidecar {
	t.Helper()

	fake := &fakeSidecar{
		spread: func(string) *Cut { return &Cut{Left: 220, Right: 180} },
	}
	fake.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var paths []string
		if err := json.NewDecoder(r.Body).Decode(&paths); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		fake.batches = append(fake.batches, paths)

		if fake.refuse {
			http.Error(w, `{"detail":"cannot read a page"}`, http.StatusBadRequest)
			return
		}

		answer := make([][]int, len(paths))
		for index, path := range paths {
			if _, err := os.Stat(path); err != nil {
				http.Error(w, fmt.Sprintf(`{"detail":"cannot read %s"}`, path),
					http.StatusBadRequest)
				return
			}
			if cut := fake.spread(path); cut != nil {
				answer[index] = []int{cut.Left, cut.Right}
			}
		}
		json.NewEncoder(w).Encode(answer)
	}))
	t.Cleanup(fake.server.Close)

	return fake
}

type harness struct {
	store   *books.Store
	files   *filestore.Store
	work    *Worker
	sidecar *fakeSidecar
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	pool := testdb.New(t)

	files, err := filestore.New(t.TempDir())
	if err != nil {
		t.Fatalf("filestore: %v", err)
	}

	store := books.NewStore(pool)
	sidecar := newFakeSidecar(t)

	return &harness{
		store:   store,
		files:   files,
		sidecar: sidecar,
		work:    NewWorker(store, files, ws.NewHub([]string{"*"}), NewClient(sidecar.server.URL)),
	}
}

func jpegBytes(width, height int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 40, A: 255})
		}
	}

	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, img, nil); err != nil {
		panic(err)
	}
	return buffer.Bytes()
}

// seed makes a complete source book with real page files, then the copy that a
// split job fills.
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

	return source, h.derive(t, source)
}

func (h *harness) derive(t *testing.T, source books.Book) books.Book {
	t.Helper()
	ctx := context.Background()

	action, ok := books.ActionByID(books.ActionSplit)
	if !ok {
		t.Fatal("the split action is missing")
	}

	copyKey, err := books.NewStorageKey()
	if err != nil {
		t.Fatalf("storage key: %v", err)
	}

	tx, err := h.store.Pool().Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)

	copyBook, err := h.store.CreateDerivedTx(ctx, tx, source.Name, copyKey, source.ID, action, "")
	if err != nil {
		t.Fatalf("CreateDerivedTx: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	return copyBook
}

func (h *harness) run(bookID int64, attempt, maxAttempts int) error {
	job := &river.Job[Args]{
		JobRow: &rivertype.JobRow{Attempt: attempt, MaxAttempts: maxAttempts},
		Args:   Args{BookID: bookID},
	}
	return h.work.Work(context.Background(), job)
}

func TestWorkerCopiesEveryPageInOrder(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	source, copyBook := h.seed(t, 12)

	if err := h.run(copyBook.ID, 1, MaxAttempts); err != nil {
		t.Fatalf("Work: %v", err)
	}

	updated, err := h.store.Get(ctx, copyBook.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if updated.State != books.StateComplete {
		t.Errorf("State = %q, want complete", updated.State)
	}
	if updated.Stage != books.StageSplit {
		t.Errorf("Stage = %q, want %q", updated.Stage, books.StageSplit)
	}
	want := 12 * pagesPerSource
	if updated.PageCount != want {
		t.Errorf("PageCount = %d, want %d", updated.PageCount, want)
	}
	if updated.Progress != nil {
		t.Errorf("a complete copy kept the phase %q", *updated.Progress)
	}

	pages, err := h.store.Pages(ctx, copyBook.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}
	if len(pages) != want {
		t.Fatalf("got %d pages, want %d", len(pages), want)
	}

	sourcePages, err := h.store.Pages(ctx, source.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}
	for index, page := range pages {
		if page.PageIndex != index+1 {
			t.Fatalf("page %d has index %d", index, page.PageIndex)
		}

		// Cutting a 400 by 600 spread at the overlapping columns 220 and 180
		// gives two 220 by 600 pages.
		origin := sourcePages[index/pagesPerSource]
		if page.Width != 220 || page.Height != origin.Height {
			t.Errorf("page %d = %dx%d, want 220x%d", page.PageIndex, page.Width, page.Height,
				origin.Height)
		}
	}

	assertLineage(t, ctx, h.store, source.ID, pages)

	entries, err := os.ReadDir(h.files.PageDir(copyBook.StorageKey))
	if err != nil {
		t.Fatalf("read page dir: %v", err)
	}
	if len(entries) != want*3 {
		t.Errorf("got %d files, want %d (%d pages x 3 renditions)",
			len(entries), want*3, want)
	}
}

// assertLineage checks that every page of a copy names the source page it was
// made from, which is what both page operations follow. The pages made from one
// source page sit next to each other, so the copy reads in the order of the
// source book.
func assertLineage(t *testing.T, ctx context.Context, store *books.Store,
	sourceID int64, pages []books.Page) {
	t.Helper()

	sourcePages, err := store.Pages(ctx, sourceID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}

	claimed := map[int64]int{}
	for index, page := range pages {
		if page.SourcePageID == nil {
			t.Fatalf("page %d names no source page", page.PageIndex)
		}

		want := sourcePages[index/pagesPerSource].ID
		if *page.SourcePageID != want {
			t.Errorf("page %d names source page %d, want %d",
				page.PageIndex, *page.SourcePageID, want)
		}
		claimed[*page.SourcePageID]++
	}

	for _, page := range sourcePages {
		if claimed[page.ID] != pagesPerSource {
			t.Errorf("source page %d made %d pages, want %d",
				page.PageIndex, claimed[page.ID], pagesPerSource)
		}
	}
}

// A copy must own its files, so deleting the source book cannot empty it.
func TestWorkerGivesTheCopyItsOwnFiles(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	source, copyBook := h.seed(t, 3)

	if err := h.run(copyBook.ID, 1, MaxAttempts); err != nil {
		t.Fatalf("Work: %v", err)
	}

	sourcePages, err := h.store.Pages(ctx, source.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}
	copyPages, err := h.store.Pages(ctx, copyBook.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}

	sourceKeys := map[string]bool{}
	for _, page := range sourcePages {
		sourceKeys[page.StorageKey] = true
	}

	// Two pages made from one source page must hold separate files. A shared key
	// would make deleting one of them break the other.
	copyKeys := map[string]bool{}
	for _, page := range copyPages {
		if sourceKeys[page.StorageKey] {
			t.Errorf("page %d shares the storage key of its source", page.PageIndex)
		}
		if copyKeys[page.StorageKey] {
			t.Errorf("page %d shares the storage key of another page", page.PageIndex)
		}
		copyKeys[page.StorageKey] = true
	}

	if err := os.RemoveAll(h.files.PageDir(source.StorageKey)); err != nil {
		t.Fatalf("remove source pages: %v", err)
	}
	for _, page := range copyPages {
		path := h.files.PagePath(copyBook.StorageKey, page.StorageKey,
			filestore.RenditionOriginal, page.OriginalExt)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("page %d vanished with its source: %v", page.PageIndex, err)
		}
	}
}

// The source book must come out of the job exactly as it went in.
func TestWorkerLeavesTheSourceBookAlone(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	source, copyBook := h.seed(t, 3)

	if err := h.run(copyBook.ID, 1, MaxAttempts); err != nil {
		t.Fatalf("Work: %v", err)
	}

	unchanged, err := h.store.Get(ctx, source.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if unchanged.State != books.StateComplete || unchanged.Stage != books.StageUploaded {
		t.Errorf("the source book changed: %+v", unchanged)
	}
	if unchanged.PageCount != 3 {
		t.Errorf("the source book has %d pages, want 3", unchanged.PageCount)
	}
	if unchanged.DerivedState == nil || *unchanged.DerivedState != books.StateComplete {
		t.Errorf("the source reports the copy state %v, want complete", unchanged.DerivedState)
	}
}

func TestWorkerRetryLeavesOneSetOfPages(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	_, copyBook := h.seed(t, 3)

	if err := h.run(copyBook.ID, 1, MaxAttempts); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if err := h.run(copyBook.ID, 1, MaxAttempts); err != nil {
		t.Fatalf("second run: %v", err)
	}

	pages, err := h.store.Pages(ctx, copyBook.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}

	want := 3 * pagesPerSource
	if len(pages) != want {
		t.Fatalf("the retry left %d pages, want %d", len(pages), want)
	}

	seen := map[int]bool{}
	for _, page := range pages {
		if seen[page.PageIndex] {
			t.Fatalf("page index %d appears twice", page.PageIndex)
		}
		seen[page.PageIndex] = true
	}

	entries, err := os.ReadDir(h.files.PageDir(copyBook.StorageKey))
	if err != nil {
		t.Fatalf("read page dir: %v", err)
	}
	if len(entries) != want*3 {
		t.Errorf("the retry left %d files, want %d", len(entries), want*3)
	}
}

func TestWorkerFailsOnLastAttemptOnly(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	source, copyBook := h.seed(t, 2)

	sourcePages, err := h.store.Pages(ctx, source.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}

	// A source page file that is gone cannot be read, so the sidecar refuses the
	// batch and names it.
	if err := os.RemoveAll(h.files.PageDir(source.StorageKey)); err != nil {
		t.Fatalf("remove source pages: %v", err)
	}

	if err := h.run(copyBook.ID, 1, MaxAttempts); err == nil {
		t.Fatal("Work accepted a missing page file")
	}

	middle, err := h.store.Get(ctx, copyBook.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if middle.State != books.StateProcessing {
		t.Errorf("State after attempt 1 = %q, want processing", middle.State)
	}
	if middle.Error != nil {
		t.Errorf("a retryable attempt stored the reason %q", *middle.Error)
	}

	if err := h.run(copyBook.ID, MaxAttempts, MaxAttempts); err == nil {
		t.Fatal("Work accepted a missing page file on the final attempt")
	}

	final, err := h.store.Get(ctx, copyBook.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if final.State != books.StateFailed {
		t.Errorf("State after the final attempt = %q, want failed", final.State)
	}
	if final.Error == nil || !strings.Contains(*final.Error, sourcePages[0].StorageKey) {
		t.Errorf("reason = %v, want it to name the page it could not read", final.Error)
	}
	if final.Stage != books.StageSplit {
		t.Errorf("a failed copy left its stage: %q", final.Stage)
	}
	if final.Progress != nil {
		t.Errorf("a failed copy kept the phase %q", *final.Progress)
	}
}

func TestWorkerReportsTheConvertingPhase(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	_, copyBook := h.seed(t, 2)

	if _, err := h.store.SetProcessing(ctx, copyBook.ID, books.PhaseConverting); err != nil {
		t.Fatalf("SetProcessing: %v", err)
	}

	running, err := h.store.Get(ctx, copyBook.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if running.Progress == nil || *running.Progress != books.PhaseConverting {
		t.Errorf("Progress = %v, want %q", running.Progress, books.PhaseConverting)
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

	err = h.run(lonely.ID, MaxAttempts, MaxAttempts)
	if err == nil || !strings.Contains(err.Error(), "no source book") {
		t.Fatalf("Work returned %v", err)
	}
}

func TestWorkerCopiesANonSpreadWhole(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.sidecar.spread = func(string) *Cut { return nil }
	source, copyBook := h.seed(t, 4)

	if err := h.run(copyBook.ID, 1, MaxAttempts); err != nil {
		t.Fatalf("Work: %v", err)
	}

	pages, err := h.store.Pages(ctx, copyBook.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}
	if len(pages) != 4 {
		t.Fatalf("got %d pages, want one for each of the 4 source pages", len(pages))
	}

	sourcePages, err := h.store.Pages(ctx, source.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}
	for index, page := range pages {
		origin := sourcePages[index]
		if page.Width != origin.Width || page.Height != origin.Height {
			t.Errorf("page %d = %dx%d, want the whole %dx%d", page.PageIndex,
				page.Width, page.Height, origin.Width, origin.Height)
		}
	}
}

// A page that is not cut keeps the bytes of its source, so a PNG stays a PNG
// and a JPEG is never re-encoded.
func TestWorkerKeepsTheBytesOfANonSpread(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.sidecar.spread = func(string) *Cut { return nil }
	source, copyBook := h.seed(t, 1)

	if err := h.run(copyBook.ID, 1, MaxAttempts); err != nil {
		t.Fatalf("Work: %v", err)
	}

	sourcePages, err := h.store.Pages(ctx, source.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}
	pages, err := h.store.Pages(ctx, copyBook.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}

	want, err := os.ReadFile(h.files.PagePath(source.StorageKey, sourcePages[0].StorageKey,
		filestore.RenditionOriginal, sourcePages[0].OriginalExt))
	if err != nil {
		t.Fatalf("read source original: %v", err)
	}
	got, err := os.ReadFile(h.files.PagePath(copyBook.StorageKey, pages[0].StorageKey,
		filestore.RenditionOriginal, pages[0].OriginalExt))
	if err != nil {
		t.Fatalf("read copy original: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Error("the original rendition of an uncut page is not the source bytes")
	}
}

func TestWorkerMixesSpreadsAndSinglePages(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	// The first source page is a cover, the rest are spreads.
	first := true
	h.sidecar.spread = func(string) *Cut {
		if first {
			first = false
			return nil
		}
		return &Cut{Left: 220, Right: 180}
	}
	_, copyBook := h.seed(t, 5)

	if err := h.run(copyBook.ID, 1, MaxAttempts); err != nil {
		t.Fatalf("Work: %v", err)
	}

	pages, err := h.store.Pages(ctx, copyBook.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}
	if len(pages) != 9 {
		t.Fatalf("got %d pages, want 9 for one cover and four spreads", len(pages))
	}
	if pages[0].Width != 400 {
		t.Errorf("page 1 = %d wide, want the whole 400", pages[0].Width)
	}
	if pages[1].Width != 220 {
		t.Errorf("page 2 = %d wide, want a cut 220", pages[1].Width)
	}

	updated, err := h.store.Get(ctx, copyBook.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if updated.PageCount != 9 {
		t.Errorf("PageCount = %d, want 9", updated.PageCount)
	}
}

func TestWorkerAsksTheSidecarInBatches(t *testing.T) {
	h := newHarness(t)
	_, copyBook := h.seed(t, batchSize+5)

	if err := h.run(copyBook.ID, 1, MaxAttempts); err != nil {
		t.Fatalf("Work: %v", err)
	}

	if len(h.sidecar.batches) != 2 {
		t.Fatalf("the worker sent %d batches, want 2", len(h.sidecar.batches))
	}
	if len(h.sidecar.batches[0]) != batchSize {
		t.Errorf("batch 1 held %d pages, want %d", len(h.sidecar.batches[0]), batchSize)
	}
	if len(h.sidecar.batches[1]) != 5 {
		t.Errorf("batch 2 held %d pages, want 5", len(h.sidecar.batches[1]))
	}
	for _, path := range h.sidecar.batches[0] {
		if !strings.Contains(path, filestore.RenditionOriginal) {
			t.Errorf("the worker sent %q, want the original rendition", path)
		}
	}
}

func TestWorkerReportsProgressAfterEachBatch(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	_, copyBook := h.seed(t, batchSize+5)

	counts := []int{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			book, err := h.store.Get(ctx, copyBook.ID)
			if err != nil || book.State == books.StateComplete {
				return
			}
			if book.PageCount > 0 && (len(counts) == 0 || counts[len(counts)-1] != book.PageCount) {
				counts = append(counts, book.PageCount)
			}
		}
	}()

	if err := h.run(copyBook.ID, 1, MaxAttempts); err != nil {
		t.Fatalf("Work: %v", err)
	}
	<-done

	for index := 1; index < len(counts); index++ {
		if counts[index] < counts[index-1] {
			t.Errorf("the reported page count fell from %d to %d",
				counts[index-1], counts[index])
		}
	}

	updated, err := h.store.Get(ctx, copyBook.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if updated.PageCount != (batchSize+5)*pagesPerSource {
		t.Errorf("PageCount = %d, want %d", updated.PageCount, (batchSize+5)*pagesPerSource)
	}
}

func TestWorkerFailsWhenTheSidecarRefuses(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.sidecar.refuse = true
	_, copyBook := h.seed(t, 3)

	for attempt := 1; attempt < MaxAttempts; attempt++ {
		if err := h.run(copyBook.ID, attempt, MaxAttempts); err == nil {
			t.Fatalf("attempt %d succeeded with a refusing sidecar", attempt)
		}
		book, err := h.store.Get(ctx, copyBook.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if book.State == books.StateFailed {
			t.Fatalf("attempt %d marked the copy failed while attempts remained", attempt)
		}
	}

	if err := h.run(copyBook.ID, MaxAttempts, MaxAttempts); err == nil {
		t.Fatal("the last attempt succeeded with a refusing sidecar")
	}

	book, err := h.store.Get(ctx, copyBook.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if book.State != books.StateFailed {
		t.Fatalf("State = %q, want failed", book.State)
	}
	if book.Error == nil || !strings.Contains(*book.Error, "sidecar") {
		t.Errorf("the reason %v does not name the sidecar", book.Error)
	}

	pages, err := h.store.Pages(ctx, copyBook.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}
	if len(pages) != 0 {
		t.Errorf("the failed copy kept %d pages", len(pages))
	}
}

// A cut page is narrower than the spread it came from, so the copy must record
// the size it really stores.
func TestWorkerStoresTheSizeOfEveryCutPage(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	source, copyBook := h.seed(t, 2)

	if err := h.run(copyBook.ID, 1, MaxAttempts); err != nil {
		t.Fatalf("Work: %v", err)
	}

	sourcePages, err := h.store.Pages(ctx, source.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}
	pages, err := h.store.Pages(ctx, copyBook.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}
	if len(pages) != 2*pagesPerSource {
		t.Fatalf("got %d pages, want %d", len(pages), 2*pagesPerSource)
	}

	for _, page := range pages {
		origin := sourcePages[(page.PageIndex-1)/pagesPerSource]
		if page.Width >= origin.Width {
			t.Errorf("page %d = %d wide, want narrower than its %d wide source page",
				page.PageIndex, page.Width, origin.Width)
		}

		stored, err := ingest.Decode(h.files.PagePath(copyBook.StorageKey, page.StorageKey,
			filestore.RenditionOriginal, page.OriginalExt))
		if err != nil {
			t.Fatalf("decode page %d: %v", page.PageIndex, err)
		}
		if bounds := stored.Bounds(); bounds.Dx() != page.Width || bounds.Dy() != page.Height {
			t.Errorf("page %d records %dx%d but stores %dx%d", page.PageIndex,
				page.Width, page.Height, bounds.Dx(), bounds.Dy())
		}
	}
}

func TestWorkerRecordsTheCutOfEveryPage(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	// The first source page is a cover, the second is a spread.
	first := true
	h.sidecar.spread = func(string) *Cut {
		if first {
			first = false
			return nil
		}
		return &Cut{Left: 220, Right: 180}
	}
	_, copyBook := h.seed(t, 2)

	if err := h.run(copyBook.ID, 1, MaxAttempts); err != nil {
		t.Fatalf("Work: %v", err)
	}

	pages, err := h.store.Pages(ctx, copyBook.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}
	if len(pages) != 3 {
		t.Fatalf("got %d pages, want a cover and two halves", len(pages))
	}

	if pages[0].CutSide != nil || pages[0].CutColumn != nil {
		t.Errorf("the cover records the cut %v at %v", pages[0].CutSide, pages[0].CutColumn)
	}

	want := []struct {
		side   string
		column int
	}{{books.CutLeft, 220}, {books.CutRight, 180}}

	for index, expected := range want {
		page := pages[index+1]
		if page.CutSide == nil || *page.CutSide != expected.side {
			t.Errorf("page %d records the side %v, want %q",
				page.PageIndex, page.CutSide, expected.side)
			continue
		}
		if page.CutColumn == nil || *page.CutColumn != expected.column {
			t.Errorf("page %d records the column %v, want %d",
				page.PageIndex, page.CutColumn, expected.column)
		}
	}

	// The recorded column is the column the crop took, so it reads back as the
	// width of a left page and as the width the right page leaves out.
	if pages[1].Width != 220 {
		t.Errorf("the left page is %d wide, want its recorded column 220", pages[1].Width)
	}
	if pages[2].Width != 400-180 {
		t.Errorf("the right page is %d wide, want %d", pages[2].Width, 400-180)
	}
}
