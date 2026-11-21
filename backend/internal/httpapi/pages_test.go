package httpapi

import (
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
	"testing"

	"github.com/riverqueue/river"

	"github.com/themylogin/ownocr/backend/internal/books"
	"github.com/themylogin/ownocr/backend/internal/filestore"
	"github.com/themylogin/ownocr/backend/internal/ingest"
	"github.com/themylogin/ownocr/backend/internal/ocr"
	"github.com/themylogin/ownocr/backend/internal/queue"
	"github.com/themylogin/ownocr/backend/internal/testdb"
	"github.com/themylogin/ownocr/backend/internal/ws"
)

type pageHarness struct {
	router http.Handler
	store  *books.Store
	files  *filestore.Store
}

// idleRescanWorker claims the rescan job kind. River refuses to insert a job no
// worker claims, and the client never starts, so a test reads the queue a
// request wrote without the job running off.
type idleRescanWorker struct {
	river.WorkerDefaults[ocr.RescanArgs]
}

func (*idleRescanWorker) Work(context.Context, *river.Job[ocr.RescanArgs]) error { return nil }

func newPageHarness(t *testing.T) *pageHarness {
	t.Helper()

	pool := testdb.New(t)

	files, err := filestore.New(t.TempDir())
	if err != nil {
		t.Fatalf("filestore: %v", err)
	}

	workers := river.NewWorkers()
	river.AddWorker(workers, &idleRescanWorker{})

	client, err := queue.New(pool, workers, 1)
	if err != nil {
		t.Fatalf("queue: %v", err)
	}

	store := books.NewStore(pool)
	hub := ws.NewHub(nil)

	return &pageHarness{
		router: NewRouter(Deps{Pool: pool, Hub: hub, Files: files, Books: store,
			Queue: client}),
		store: store,
		files: files,
	}
}

func (h *pageHarness) do(t *testing.T, method, path string) (int, []byte) {
	t.Helper()

	recorder := httptest.NewRecorder()
	h.router.ServeHTTP(recorder, httptest.NewRequest(method, path, nil))
	return recorder.Code, recorder.Body.Bytes()
}

func jpegOf(t *testing.T, width, height int, shade uint8) string {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: shade, G: shade, B: shade, A: 255})
		}
	}

	path := filepath.Join(t.TempDir(), fmt.Sprintf("page-%d.jpg", shade))
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer file.Close()

	if err := jpeg.Encode(file, img, nil); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return path
}

// addPage writes the three renditions of a page and inserts its row.
func (h *pageHarness) addPage(t *testing.T, book books.Book, index int, source *int64,
	width, height int, shade uint8) books.PageOfBook {
	t.Helper()
	ctx := context.Background()

	pageKey, err := books.NewStorageKey()
	if err != nil {
		t.Fatalf("storage key: %v", err)
	}

	ext, storedWidth, storedHeight, err := ingest.WriteRenditions(
		jpegOf(t, width, height, shade), h.files.PageDir(book.StorageKey), pageKey)
	if err != nil {
		t.Fatalf("WriteRenditions: %v", err)
	}

	err = h.store.InsertPage(ctx, books.Page{
		BookID: book.ID, PageIndex: index, StorageKey: pageKey, OriginalExt: ext,
		Width: storedWidth, Height: storedHeight, SourcePageID: source,
	})
	if err != nil {
		t.Fatalf("InsertPage: %v", err)
	}

	page, err := h.store.PageAt(ctx, book.ID, index)
	if err != nil {
		t.Fatalf("PageAt: %v", err)
	}
	return page
}

func (h *pageHarness) createBook(t *testing.T, name string) books.Book {
	t.Helper()
	ctx := context.Background()

	key, err := books.NewStorageKey()
	if err != nil {
		t.Fatalf("storage key: %v", err)
	}

	tx, err := h.store.Pool().Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)

	book, err := h.store.CreateTx(ctx, tx, name, key)
	if err != nil {
		t.Fatalf("CreateTx: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	return book
}

// profileFor answers the argument the copy of an action records. Only OCR takes
// one. Extra runs both engines, which is the case with two results a page.
func profileFor(action books.Action) string {
	if action.ID == books.ActionOCR {
		return ocr.ProfileExtra
	}
	return ""
}

func (h *pageHarness) deriveBook(t *testing.T, source books.Book, actionID string) books.Book {
	t.Helper()
	ctx := context.Background()

	action, ok := books.ActionByID(actionID)
	if !ok {
		t.Fatalf("the action %s is missing", actionID)
	}

	key, err := books.NewStorageKey()
	if err != nil {
		t.Fatalf("storage key: %v", err)
	}

	tx, err := h.store.Pool().Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)

	copyBook, err := h.store.CreateDerivedTx(ctx, tx, source.Name, key, source.ID, action,
		profileFor(action))
	if err != nil {
		t.Fatalf("CreateDerivedTx: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	return copyBook
}

func (h *pageHarness) complete(t *testing.T, book books.Book, pageCount int) books.Book {
	t.Helper()

	updated, err := h.store.SetComplete(context.Background(), book.ID, pageCount)
	if err != nil {
		t.Fatalf("SetComplete: %v", err)
	}
	return updated
}

// seedChain builds a complete uploaded book and a complete dewarped copy, each
// with three pages. The dewarped pages are half the size, as the endpoint
// returns them.
func (h *pageHarness) seedChain(t *testing.T) (books.Book, books.Book) {
	t.Helper()

	source := h.createBook(t, "Chain")
	dewarped := h.deriveBook(t, source, books.ActionDewarp)

	for index := 1; index <= 3; index++ {
		first := h.addPage(t, source, index, nil, 400, 600, uint8(40*index))
		h.addPage(t, dewarped, index, &first.ID, 200, 300, uint8(40*index))
	}

	return h.complete(t, source, 3), h.complete(t, dewarped, 3)
}

func (h *pageHarness) fileCount(t *testing.T, book books.Book) int {
	t.Helper()

	entries, err := os.ReadDir(h.files.PageDir(book.StorageKey))
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatalf("read page dir: %v", err)
	}
	return len(entries)
}

func (h *pageHarness) detail(t *testing.T, bookID int64) bookDetail {
	t.Helper()

	status, body := h.do(t, http.MethodGet, fmt.Sprintf("/api/books/%d", bookID))
	if status != http.StatusOK {
		t.Fatalf("GET book = %d: %s", status, body)
	}

	var detail bookDetail
	if err := json.Unmarshal(body, &detail); err != nil {
		t.Fatalf("decode book: %v", err)
	}
	return detail
}

func errorOf(t *testing.T, body []byte) string {
	t.Helper()

	var answer struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &answer); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	return answer.Error
}

func TestPageDeletionNamesEveryBookThatLosesAPage(t *testing.T) {
	h := newPageHarness(t)
	source, dewarped := h.seedChain(t)

	status, body := h.do(t, http.MethodGet,
		fmt.Sprintf("/api/books/%d/pages/2/deletion", source.ID))
	if status != http.StatusOK {
		t.Fatalf("status = %d: %s", status, body)
	}

	var view pageDeletionView
	if err := json.Unmarshal(body, &view); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(view.Books) != 2 {
		t.Fatalf("got %d books, want 2", len(view.Books))
	}
	if view.Books[0].ID != source.ID || view.Books[1].ID != dewarped.ID {
		t.Errorf("books = %d, %d, want %d, %d",
			view.Books[0].ID, view.Books[1].ID, source.ID, dewarped.ID)
	}
	if view.Books[0].Stage != books.StageUploaded || view.Books[1].Stage != books.StageDewarped {
		t.Errorf("stages = %q, %q", view.Books[0].Stage, view.Books[1].Stage)
	}
	if !view.Allowed || view.Reason != "" {
		t.Errorf("allowed = %v, reason = %q, want an allowed delete", view.Allowed, view.Reason)
	}
}

func TestPageDeletionOfABookWithNoCopy(t *testing.T) {
	h := newPageHarness(t)
	book := h.createBook(t, "Alone")
	h.addPage(t, book, 1, nil, 400, 600, 10)
	book = h.complete(t, book, 1)

	status, body := h.do(t, http.MethodGet,
		fmt.Sprintf("/api/books/%d/pages/1/deletion", book.ID))
	if status != http.StatusOK {
		t.Fatalf("status = %d: %s", status, body)
	}

	var view pageDeletionView
	if err := json.Unmarshal(body, &view); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(view.Books) != 1 || view.Books[0].ID != book.ID {
		t.Fatalf("got %d books, want the book alone", len(view.Books))
	}
}

func TestDeletePageRemovesItFromTheWholeChain(t *testing.T) {
	h := newPageHarness(t)
	ctx := context.Background()
	source, dewarped := h.seedChain(t)

	status, body := h.do(t, http.MethodPost,
		fmt.Sprintf("/api/books/%d/pages/2/delete", source.ID))
	if status != http.StatusOK {
		t.Fatalf("status = %d: %s", status, body)
	}

	for _, book := range []books.Book{source, dewarped} {
		pages, err := h.store.Pages(ctx, book.ID)
		if err != nil {
			t.Fatalf("Pages: %v", err)
		}
		if len(pages) != 2 {
			t.Fatalf("book %d holds %d pages, want 2", book.ID, len(pages))
		}
		if pages[0].PageIndex != 1 || pages[1].PageIndex != 3 {
			t.Errorf("book %d holds indexes %d, %d, want 1, 3",
				book.ID, pages[0].PageIndex, pages[1].PageIndex)
		}

		reloaded, err := h.store.Get(ctx, book.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if reloaded.PageCount != 2 {
			t.Errorf("book %d reports %d pages, want 2", book.ID, reloaded.PageCount)
		}
		if got := h.fileCount(t, book); got != 6 {
			t.Errorf("book %d left %d files, want 6", book.ID, got)
		}
	}
}

func TestDeletePageOfACopyLeavesTheSourceAlone(t *testing.T) {
	h := newPageHarness(t)
	ctx := context.Background()
	source, dewarped := h.seedChain(t)

	status, body := h.do(t, http.MethodPost,
		fmt.Sprintf("/api/books/%d/pages/1/delete", dewarped.ID))
	if status != http.StatusOK {
		t.Fatalf("status = %d: %s", status, body)
	}

	pages, err := h.store.Pages(ctx, source.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}
	if len(pages) != 3 {
		t.Errorf("the source book holds %d pages, want 3", len(pages))
	}
	if got := h.fileCount(t, source); got != 9 {
		t.Errorf("the source book holds %d files, want 9", got)
	}

	copyPages, err := h.store.Pages(ctx, dewarped.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}
	if len(copyPages) != 2 {
		t.Errorf("the copy holds %d pages, want 2", len(copyPages))
	}
}

func TestDeleteTheLastPageLeavesTheBook(t *testing.T) {
	h := newPageHarness(t)
	ctx := context.Background()

	book := h.createBook(t, "Only")
	h.addPage(t, book, 1, nil, 400, 600, 10)
	book = h.complete(t, book, 1)

	status, body := h.do(t, http.MethodPost,
		fmt.Sprintf("/api/books/%d/pages/1/delete", book.ID))
	if status != http.StatusOK {
		t.Fatalf("status = %d: %s", status, body)
	}

	reloaded, err := h.store.Get(ctx, book.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if reloaded.PageCount != 0 {
		t.Errorf("PageCount = %d, want 0", reloaded.PageCount)
	}

	detail := h.detail(t, book.ID)
	if len(detail.Pages) != 0 {
		t.Errorf("the book still reports %d pages", len(detail.Pages))
	}
	if detail.CoverURL != nil {
		t.Errorf("the book still reports the cover %q", *detail.CoverURL)
	}
	if detail.GridCell.Width <= 0 || detail.GridCell.Height <= 0 {
		t.Errorf("grid cell = %dx%d, want a usable shape",
			detail.GridCell.Width, detail.GridCell.Height)
	}
}

// A gap must not cost the book its cover. The cover is the first page that is
// left, not the page at index 1.
func TestDeletingTheFirstPageKeepsACover(t *testing.T) {
	h := newPageHarness(t)

	book := h.createBook(t, "Cover")
	for index := 1; index <= 3; index++ {
		h.addPage(t, book, index, nil, 400, 600, uint8(40*index))
	}
	book = h.complete(t, book, 3)

	status, body := h.do(t, http.MethodPost,
		fmt.Sprintf("/api/books/%d/pages/1/delete", book.ID))
	if status != http.StatusOK {
		t.Fatalf("status = %d: %s", status, body)
	}

	detail := h.detail(t, book.ID)
	if detail.CoverURL == nil {
		t.Fatal("the book lost its cover with page 1")
	}
	if len(detail.Pages) != 2 {
		t.Fatalf("the book holds %d pages, want 2", len(detail.Pages))
	}
	if want := detail.Pages[0].ThumbnailURL; *detail.CoverURL != want {
		t.Errorf("cover = %q, want the first page that is left, %q", *detail.CoverURL, want)
	}
}

func TestDeletePageRefusesWhileTheChainRuns(t *testing.T) {
	h := newPageHarness(t)
	ctx := context.Background()
	source, dewarped := h.seedChain(t)

	if _, err := h.store.SetProcessing(ctx, dewarped.ID, books.PhaseConverting); err != nil {
		t.Fatalf("SetProcessing: %v", err)
	}

	status, body := h.do(t, http.MethodGet,
		fmt.Sprintf("/api/books/%d/pages/2/deletion", source.ID))
	if status != http.StatusOK {
		t.Fatalf("preview status = %d: %s", status, body)
	}
	var view pageDeletionView
	if err := json.Unmarshal(body, &view); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if view.Allowed || view.Reason == "" {
		t.Errorf("the preview allows the delete while the copy runs")
	}

	status, body = h.do(t, http.MethodPost,
		fmt.Sprintf("/api/books/%d/pages/2/delete", source.ID))
	if status != http.StatusConflict {
		t.Fatalf("status = %d: %s, want 409", status, body)
	}

	pages, err := h.store.Pages(ctx, source.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}
	if len(pages) != 3 {
		t.Errorf("the refusal removed %d pages", 3-len(pages))
	}
}

func TestDeletePageAllowsAFailedCopy(t *testing.T) {
	h := newPageHarness(t)
	ctx := context.Background()
	source, dewarped := h.seedChain(t)

	if _, err := h.store.SetFailed(ctx, dewarped.ID, "the endpoint failed"); err != nil {
		t.Fatalf("SetFailed: %v", err)
	}

	status, body := h.do(t, http.MethodPost,
		fmt.Sprintf("/api/books/%d/pages/2/delete", source.ID))
	if status != http.StatusOK {
		t.Fatalf("status = %d: %s", status, body)
	}

	pages, err := h.store.Pages(ctx, source.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}
	if len(pages) != 2 {
		t.Errorf("the source book holds %d pages, want 2", len(pages))
	}
}

func TestDeleteAnUnknownPage(t *testing.T) {
	h := newPageHarness(t)
	source, _ := h.seedChain(t)

	status, _ := h.do(t, http.MethodPost,
		fmt.Sprintf("/api/books/%d/pages/9/delete", source.ID))
	if status != http.StatusNotFound {
		t.Errorf("status = %d, want 404", status)
	}

	status, _ = h.do(t, http.MethodGet, "/api/books/987654/pages/1/deletion")
	if status != http.StatusNotFound {
		t.Errorf("unknown book status = %d, want 404", status)
	}
}

func TestPageRollbackComparesBothPages(t *testing.T) {
	h := newPageHarness(t)
	_, dewarped := h.seedChain(t)

	status, body := h.do(t, http.MethodGet,
		fmt.Sprintf("/api/books/%d/pages/2/rollback", dewarped.ID))
	if status != http.StatusOK {
		t.Fatalf("status = %d: %s", status, body)
	}

	var view pageRollbackView
	if err := json.Unmarshal(body, &view); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if view.Source.StageTitle != "Uploaded" || view.Current.StageTitle != "Dewarped" {
		t.Errorf("stages = %q, %q, want Uploaded, Dewarped",
			view.Source.StageTitle, view.Current.StageTitle)
	}
	if view.Source.Width != 400 || view.Source.Height != 600 {
		t.Errorf("source = %dx%d, want 400x600", view.Source.Width, view.Source.Height)
	}
	if view.Current.Width != 200 || view.Current.Height != 300 {
		t.Errorf("current = %dx%d, want 200x300", view.Current.Width, view.Current.Height)
	}
	if view.Source.DisplayURL == "" || view.Current.DisplayURL == view.Source.DisplayURL {
		t.Errorf("the two pages share the display URL %q", view.Source.DisplayURL)
	}
}

func TestApplyPageRollbackInstallsTheSourcePage(t *testing.T) {
	h := newPageHarness(t)
	ctx := context.Background()
	source, dewarped := h.seedChain(t)

	before, err := h.store.PageAt(ctx, dewarped.ID, 2)
	if err != nil {
		t.Fatalf("PageAt: %v", err)
	}

	status, body := h.do(t, http.MethodPost,
		fmt.Sprintf("/api/books/%d/pages/2/rollback", dewarped.ID))
	if status != http.StatusOK {
		t.Fatalf("status = %d: %s", status, body)
	}

	var view pageView
	if err := json.Unmarshal(body, &view); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if view.Index != 2 {
		t.Errorf("Index = %d, want 2", view.Index)
	}
	if view.Width != 400 || view.Height != 600 {
		t.Errorf("size = %dx%d, want the uploaded 400x600", view.Width, view.Height)
	}

	after, err := h.store.PageAt(ctx, dewarped.ID, 2)
	if err != nil {
		t.Fatalf("PageAt: %v", err)
	}
	if after.ID != before.ID {
		t.Errorf("the page changed identity")
	}
	if after.StorageKey == before.StorageKey {
		t.Error("the page kept its storage key, so a cached image would survive")
	}
	if after.SourcePageID == nil || *after.SourcePageID != *before.SourcePageID {
		t.Error("the page lost the source page it names")
	}

	// The copy owns its bytes: the new files sit under its own key.
	sourcePage, err := h.store.PageAt(ctx, source.ID, 2)
	if err != nil {
		t.Fatalf("PageAt: %v", err)
	}
	if after.StorageKey == sourcePage.StorageKey {
		t.Error("the page points at the file of the source book")
	}
	if got := h.fileCount(t, dewarped); got != 9 {
		t.Errorf("the copy holds %d files, want 9", got)
	}
	if got := h.fileCount(t, source); got != 9 {
		t.Errorf("the source book holds %d files, want 9", got)
	}

	reloaded, err := h.store.Get(ctx, dewarped.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if reloaded.PageCount != 3 {
		t.Errorf("PageCount = %d, want 3", reloaded.PageCount)
	}
}

// A page that already shows the original has nothing left to roll back to.
func TestPageRollbackRefusesAPageThatAlreadyShowsTheOriginal(t *testing.T) {
	h := newPageHarness(t)
	_, dewarped := h.seedChain(t)

	status, body := h.do(t, http.MethodPost,
		fmt.Sprintf("/api/books/%d/pages/2/rollback", dewarped.ID))
	if status != http.StatusOK {
		t.Fatalf("the first rollback failed: %d: %s", status, body)
	}

	var view pageView
	if err := json.Unmarshal(body, &view); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !view.RolledBack {
		t.Error("the answer does not report the page as rolled back")
	}

	for _, method := range []string{http.MethodGet, http.MethodPost} {
		status, body := h.do(t, method,
			fmt.Sprintf("/api/books/%d/pages/2/rollback", dewarped.ID))
		if status != http.StatusUnprocessableEntity {
			t.Fatalf("%s status = %d: %s, want 422", method, status, body)
		}
		if errorOf(t, body) == "" {
			t.Errorf("%s named no reason", method)
		}
	}

	detail := h.detail(t, dewarped.ID)
	for _, page := range detail.Pages {
		want := page.Index == 2
		if page.RolledBack != want {
			t.Errorf("page %d reports rolledBack %v, want %v", page.Index, page.RolledBack, want)
		}
	}
}

func TestPageRollbackRefusesABookWithACopy(t *testing.T) {
	h := newPageHarness(t)
	_, dewarped := h.seedChain(t)

	split := h.deriveBook(t, dewarped, books.ActionSplit)
	h.complete(t, split, 0)

	for _, method := range []string{http.MethodGet, http.MethodPost} {
		status, body := h.do(t, method,
			fmt.Sprintf("/api/books/%d/pages/2/rollback", dewarped.ID))
		if status != http.StatusConflict {
			t.Fatalf("%s status = %d: %s, want 409", method, status, body)
		}
		if errorOf(t, body) == "" {
			t.Errorf("%s named no reason", method)
		}
	}
}

func TestPageRollbackRefusesAnUploadedBook(t *testing.T) {
	h := newPageHarness(t)
	source, _ := h.seedChain(t)

	status, body := h.do(t, http.MethodGet,
		fmt.Sprintf("/api/books/%d/pages/1/rollback", source.ID))
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d: %s, want 422", status, body)
	}
	if errorOf(t, body) == "" {
		t.Error("the refusal named no reason")
	}
}

func TestPageRollbackRefusesAnActionThatMakesTwoPages(t *testing.T) {
	h := newPageHarness(t)

	source := h.createBook(t, "Split source")
	first := h.addPage(t, source, 1, nil, 400, 600, 10)
	source = h.complete(t, source, 1)

	split := h.deriveBook(t, source, books.ActionSplit)
	h.addPage(t, split, 1, &first.ID, 200, 600, 10)
	split = h.complete(t, split, 1)

	status, body := h.do(t, http.MethodGet,
		fmt.Sprintf("/api/books/%d/pages/1/rollback", split.ID))
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d: %s, want 422", status, body)
	}
}

func TestPageRollbackRefusesWhenTheSourcePageIsGone(t *testing.T) {
	h := newPageHarness(t)
	ctx := context.Background()
	source, dewarped := h.seedChain(t)

	page, err := h.store.PageAt(ctx, source.ID, 2)
	if err != nil {
		t.Fatalf("PageAt: %v", err)
	}
	if _, err := h.store.Pool().Exec(ctx,
		`DELETE FROM book_page WHERE id = $1`, page.ID); err != nil {
		t.Fatalf("delete source page: %v", err)
	}

	status, body := h.do(t, http.MethodGet,
		fmt.Sprintf("/api/books/%d/pages/2/rollback", dewarped.ID))
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d: %s, want 422", status, body)
	}
}

func TestPageRollbackRefusesWhileTheChainRuns(t *testing.T) {
	h := newPageHarness(t)
	ctx := context.Background()
	_, dewarped := h.seedChain(t)

	if _, err := h.store.SetProcessing(ctx, dewarped.ID, books.PhaseConverting); err != nil {
		t.Fatalf("SetProcessing: %v", err)
	}

	status, body := h.do(t, http.MethodPost,
		fmt.Sprintf("/api/books/%d/pages/2/rollback", dewarped.ID))
	if status != http.StatusConflict {
		t.Fatalf("status = %d: %s, want 409", status, body)
	}
}

func TestBookDetailReportsThePageRollback(t *testing.T) {
	h := newPageHarness(t)
	source, dewarped := h.seedChain(t)

	if detail := h.detail(t, dewarped.ID); !detail.PageRollback {
		t.Error("a complete dewarped copy with no copy offers no rollback")
	}
	if detail := h.detail(t, source.ID); detail.PageRollback {
		t.Error("an uploaded book offers a rollback")
	}

	split := h.deriveBook(t, dewarped, books.ActionSplit)
	h.complete(t, split, 0)

	if detail := h.detail(t, dewarped.ID); detail.PageRollback {
		t.Error("a book that already has a copy offers a rollback")
	}
	if detail := h.detail(t, split.ID); detail.PageRollback {
		t.Error("a split copy offers a rollback")
	}
}

// OCR reads a page without changing it, so a rollback would install the page
// the copy already shows.
func TestPageRollbackRefusesAnOCRCopy(t *testing.T) {
	h := newPageHarness(t)

	source := h.createBook(t, "OCR source")
	first := h.addPage(t, source, 1, nil, 400, 600, 10)
	source = h.complete(t, source, 1)

	read := h.deriveBook(t, source, books.ActionOCR)
	h.addPage(t, read, 1, &first.ID, 400, 600, 10)
	read = h.complete(t, read, 1)

	if detail := h.detail(t, read.ID); detail.PageRollback {
		t.Error("an OCR copy offers a rollback")
	}

	status, body := h.do(t, http.MethodGet,
		fmt.Sprintf("/api/books/%d/pages/1/rollback", read.ID))
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d: %s, want 422", status, body)
	}

	status, body = h.do(t, http.MethodPost,
		fmt.Sprintf("/api/books/%d/pages/1/rollback", read.ID))
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d: %s, want 422", status, body)
	}
}

func TestBookDetailReportsTheSourcePageOfEveryPage(t *testing.T) {
	h := newPageHarness(t)
	source, dewarped := h.seedChain(t)

	detail := h.detail(t, dewarped.ID)
	for _, page := range detail.Pages {
		if page.SourcePageID == nil {
			t.Errorf("page %d reports no source page", page.Index)
		}
	}

	detail = h.detail(t, source.ID)
	for _, page := range detail.Pages {
		if page.SourcePageID != nil {
			t.Errorf("uploaded page %d reports source page %d", page.Index, *page.SourcePageID)
		}
	}
}

// seedRead builds an OCR copy of one page and answers the copy and the page of
// the copy. The page holds no result yet, so a test writes the ones it needs.
func (h *pageHarness) seedRead(t *testing.T) (books.Book, books.PageOfBook) {
	t.Helper()

	source := h.createBook(t, "Read")
	first := h.addPage(t, source, 1, nil, 400, 600, 10)
	h.complete(t, source, 1)

	read := h.deriveBook(t, source, books.ActionOCR)
	page := h.addPage(t, read, 1, &first.ID, 400, 600, 10)
	h.complete(t, read, 1)

	return read, page
}

func (h *pageHarness) addResult(t *testing.T, pageID int64, model, text, data string) {
	t.Helper()

	err := h.store.InsertPageOCR(context.Background(), books.PageOCR{
		PageID: pageID, Model: model, Text: text, Data: []byte(data),
	}, nil)
	if err != nil {
		t.Fatalf("InsertPageOCR %s: %v", model, err)
	}
}

func (h *pageHarness) pageOCRView(t *testing.T, bookID int64, index int) pageOCRView {
	t.Helper()

	status, body := h.do(t, http.MethodGet,
		fmt.Sprintf("/api/books/%d/pages/%d/ocr", bookID, index))
	if status != http.StatusOK {
		t.Fatalf("status = %d: %s", status, body)
	}

	var view pageOCRView
	if err := json.Unmarshal(body, &view); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return view
}

const v6Data = `{"raw": {}, "lines": [
	{"bbox": [100, 120, 300, 160], "text": "the first line", "score": 0.99},
	{"bbox": [100, 180, 320, 220], "text": "the second line", "score": 0.98}
]}`

const mergedData = `{"lines": [
	{"bbox": [100, 120, 300, 160], "text": "τὴν φιλοσοφίαν", "source": "vl", "score": 0.99},
	{"bbox": [100, 180, 320, 220], "text": "the second line", "source": "v6", "score": 0.98}
], "dropped": [], "counts": {"lines": 2, "fromText": 1, "fromBoxes": 1, "droppedChars": 0}}`

func TestPageOCRAnswersTheMergedResult(t *testing.T) {
	h := newPageHarness(t)
	read, page := h.seedRead(t)

	h.addResult(t, page.ID, ocr.ModeV6, "the first line\nthe second line", v6Data)
	h.addResult(t, page.ID, ocr.ModeVL, "# blocks", `{"parsing_res_list": []}`)
	h.addResult(t, page.ID, ocr.ModelMerged, "τὴν φιλοσοφίαν\nthe second line", mergedData)

	view := h.pageOCRView(t, read.ID, 1)

	if view.Model != ocr.ModelMerged {
		t.Errorf("model = %q, want %q", view.Model, ocr.ModelMerged)
	}
	if len(view.Lines) != 2 {
		t.Fatalf("the page answered %d lines, want 2", len(view.Lines))
	}
	if view.Lines[0].Text != "τὴν φιλοσοφίαν" {
		t.Errorf("line 0 reads %q, want the merged text unchanged", view.Lines[0].Text)
	}
	if view.Lines[0].Box != [4]int{100, 120, 300, 160} {
		t.Errorf("line 0 carries %v, want the stored box", view.Lines[0].Box)
	}
}

func TestPageOCRFallsBackToTheEngineResult(t *testing.T) {
	h := newPageHarness(t)
	read, page := h.seedRead(t)

	h.addResult(t, page.ID, ocr.ModeV6, "the first line\nthe second line", v6Data)

	view := h.pageOCRView(t, read.ID, 1)

	if view.Model != ocr.ModeV6 {
		t.Errorf("model = %q, want %q", view.Model, ocr.ModeV6)
	}
	if len(view.Lines) != 2 || view.Lines[1].Text != "the second line" {
		t.Fatalf("the page answered %+v, want the two v6 lines", view.Lines)
	}
}

// The boxes count in the stored page image, so a client needs no second size.
func TestPageOCRReportsTheSizeOfThePage(t *testing.T) {
	h := newPageHarness(t)
	read, page := h.seedRead(t)

	h.addResult(t, page.ID, ocr.ModeV6, "text", v6Data)

	view := h.pageOCRView(t, read.ID, 1)
	detail := h.detail(t, read.ID)

	if view.Width != detail.Pages[0].Width || view.Height != detail.Pages[0].Height {
		t.Errorf("the text reports %dx%d, the page %dx%d",
			view.Width, view.Height, detail.Pages[0].Width, detail.Pages[0].Height)
	}
	for _, line := range view.Lines {
		if line.Box[2] > view.Width || line.Box[3] > view.Height {
			t.Errorf("the box %v lies outside the page", line.Box)
		}
	}
}

// The vl result holds blocks of reflowed text and no lines, so a page holding it
// alone has nothing a renderer can draw.
func TestPageOCRRefusesAPageWithNoLines(t *testing.T) {
	h := newPageHarness(t)
	read, page := h.seedRead(t)

	h.addResult(t, page.ID, ocr.ModeVL, "# blocks", `{"parsing_res_list": []}`)

	status, body := h.do(t, http.MethodGet, fmt.Sprintf("/api/books/%d/pages/1/ocr", read.ID))
	if status != http.StatusNotFound {
		t.Fatalf("status = %d: %s, want 404", status, body)
	}
	if errorOf(t, body) == "" {
		t.Error("the refusal names no reason")
	}
}

func TestPageOCRRefusesAPageThatWasNotRead(t *testing.T) {
	h := newPageHarness(t)
	read, _ := h.seedRead(t)

	status, body := h.do(t, http.MethodGet, fmt.Sprintf("/api/books/%d/pages/1/ocr", read.ID))
	if status != http.StatusNotFound {
		t.Fatalf("status = %d: %s, want 404", status, body)
	}
	if errorOf(t, body) == "" {
		t.Error("the refusal names no reason")
	}
}

func TestPageOCRRefusesAnUnknownPageAndAnUnknownBook(t *testing.T) {
	h := newPageHarness(t)
	read, page := h.seedRead(t)

	h.addResult(t, page.ID, ocr.ModeV6, "text", v6Data)

	status, body := h.do(t, http.MethodGet, fmt.Sprintf("/api/books/%d/pages/9/ocr", read.ID))
	if status != http.StatusNotFound {
		t.Fatalf("an unknown page = %d: %s, want 404", status, body)
	}
	if errorOf(t, body) == "" {
		t.Error("the refusal of an unknown page names no reason")
	}

	status, body = h.do(t, http.MethodGet, "/api/books/999999/pages/1/ocr")
	if status != http.StatusNotFound {
		t.Fatalf("an unknown book = %d: %s, want 404", status, body)
	}
	if errorOf(t, body) == "" {
		t.Error("the refusal of an unknown book names no reason")
	}
}

// seedReadPages builds an OCR copy of count pages and answers the copy with its
// pages. No page holds a result yet, so a test writes the ones it needs.
func (h *pageHarness) seedReadPages(t *testing.T, count int) (books.Book, []books.PageOfBook) {
	t.Helper()

	source := h.createBook(t, "Read")
	sources := make([]books.PageOfBook, 0, count)
	for index := 1; index <= count; index++ {
		sources = append(sources, h.addPage(t, source, index, nil, 400, 600, 10))
	}
	h.complete(t, source, count)

	read := h.deriveBook(t, source, books.ActionOCR)
	pages := make([]books.PageOfBook, 0, count)
	for index := 1; index <= count; index++ {
		pages = append(pages, h.addPage(t, read, index, &sources[index-1].ID, 400, 600, 10))
	}
	h.complete(t, read, count)

	return read, pages
}

func (h *pageHarness) bookOCRView(t *testing.T, bookID int64) bookOCRView {
	t.Helper()

	status, body := h.do(t, http.MethodGet, fmt.Sprintf("/api/books/%d/ocr", bookID))
	if status != http.StatusOK {
		t.Fatalf("status = %d: %s", status, body)
	}

	var view bookOCRView
	if err := json.Unmarshal(body, &view); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return view
}

func TestBookOCRAnswersEveryPageInOrder(t *testing.T) {
	h := newPageHarness(t)
	read, pages := h.seedReadPages(t, 12)

	for _, page := range pages {
		h.addResult(t, page.ID, ocr.ModeV6, "the first line\nthe second line", v6Data)
	}

	view := h.bookOCRView(t, read.ID)

	if len(view.Pages) != 12 {
		t.Fatalf("the book answered %d pages, want 12", len(view.Pages))
	}
	for i, page := range view.Pages {
		if page.Index != i+1 {
			t.Fatalf("entry %d carries the index %d, want %d", i, page.Index, i+1)
		}
		if page.Model != ocr.ModeV6 {
			t.Errorf("page %d names the model %q, want %q", page.Index, page.Model, ocr.ModeV6)
		}
		if page.Width != 400 || page.Height != 600 {
			t.Errorf("page %d reports %dx%d, want the stored page size",
				page.Index, page.Width, page.Height)
		}
		if len(page.Lines) != 2 {
			t.Errorf("page %d holds %d lines, want 2", page.Index, len(page.Lines))
		}
	}
}

func TestBookOCRAnswersTheMergedResult(t *testing.T) {
	h := newPageHarness(t)
	read, pages := h.seedReadPages(t, 2)

	for _, page := range pages {
		h.addResult(t, page.ID, ocr.ModeV6, "the first line\nthe second line", v6Data)
		h.addResult(t, page.ID, ocr.ModeVL, "# blocks", `{"parsing_res_list": []}`)
		h.addResult(t, page.ID, ocr.ModelMerged, "τὴν φιλοσοφίαν\nthe second line", mergedData)
	}

	view := h.bookOCRView(t, read.ID)

	for _, page := range view.Pages {
		if page.Model != ocr.ModelMerged {
			t.Errorf("page %d names the model %q, want %q",
				page.Index, page.Model, ocr.ModelMerged)
		}
		if page.Lines[0].Text != "τὴν φιλοσοφίαν" {
			t.Errorf("page %d line 0 reads %q, want the merged text unchanged",
				page.Index, page.Lines[0].Text)
		}
	}
}

// A copy that is still running holds unread pages. One of them must not fail the
// request, or a reader would see nothing of the pages the copy has read.
func TestBookOCRAnswersAnUnreadPageWithNoLines(t *testing.T) {
	h := newPageHarness(t)
	read, pages := h.seedReadPages(t, 12)

	for _, page := range pages[:5] {
		h.addResult(t, page.ID, ocr.ModeV6, "the first line\nthe second line", v6Data)
	}

	view := h.bookOCRView(t, read.ID)

	if len(view.Pages) != 12 {
		t.Fatalf("the book answered %d pages, want 12", len(view.Pages))
	}
	for _, page := range view.Pages {
		read := page.Index <= 5
		if read && len(page.Lines) != 2 {
			t.Errorf("the read page %d holds %d lines, want 2", page.Index, len(page.Lines))
		}
		if !read && len(page.Lines) != 0 {
			t.Errorf("the unread page %d holds %d lines, want none",
				page.Index, len(page.Lines))
		}
		if !read && page.Model != "" {
			t.Errorf("the unread page %d names the model %q, want none",
				page.Index, page.Model)
		}
	}
}

// The vl result holds blocks of reflowed text and no lines, so a page holding it
// alone answers no lines rather than the blocks of another model.
func TestBookOCRAnswersNoLinesForAPageWithoutThem(t *testing.T) {
	h := newPageHarness(t)
	read, pages := h.seedReadPages(t, 2)

	h.addResult(t, pages[0].ID, ocr.ModeVL, "# blocks", `{"parsing_res_list": []}`)
	h.addResult(t, pages[1].ID, ocr.ModeV6, "the first line\nthe second line", v6Data)

	view := h.bookOCRView(t, read.ID)

	if len(view.Pages[0].Lines) != 0 || view.Pages[0].Model != "" {
		t.Errorf("page 1 answered %+v, want no model and no lines", view.Pages[0])
	}
	if len(view.Pages[1].Lines) != 2 {
		t.Errorf("page 2 holds %d lines, want 2", len(view.Pages[1].Lines))
	}
}

// A client that reads one page out of the book answer must get what the answer
// for that page alone gives. Two readers of one page cannot disagree.
func TestBookOCRAgreesWithThePageAnswer(t *testing.T) {
	h := newPageHarness(t)
	read, pages := h.seedReadPages(t, 4)

	for _, page := range pages {
		h.addResult(t, page.ID, ocr.ModeV6, "the first line\nthe second line", v6Data)
	}
	h.addResult(t, pages[2].ID, ocr.ModeVL, "# blocks", `{"parsing_res_list": []}`)
	h.addResult(t, pages[2].ID, ocr.ModelMerged, "τὴν φιλοσοφίαν\nthe second line", mergedData)

	book := h.bookOCRView(t, read.ID)
	page := h.pageOCRView(t, read.ID, 3)
	entry := book.Pages[2]

	if entry.Model != page.Model {
		t.Errorf("the book names the model %q for page 3, the page names %q",
			entry.Model, page.Model)
	}
	if entry.Width != page.Width || entry.Height != page.Height {
		t.Errorf("the book reports %dx%d for page 3, the page reports %dx%d",
			entry.Width, entry.Height, page.Width, page.Height)
	}
	if len(entry.Lines) != len(page.Lines) {
		t.Fatalf("the book holds %d lines for page 3, the page holds %d",
			len(entry.Lines), len(page.Lines))
	}
	for i := range entry.Lines {
		if entry.Lines[i] != page.Lines[i] {
			t.Errorf("line %d of page 3 reads %+v in the book and %+v in the page",
				i, entry.Lines[i], page.Lines[i])
		}
	}
}

func TestBookOCRRefusesABookThatDoesNotExist(t *testing.T) {
	h := newPageHarness(t)

	status, body := h.do(t, http.MethodGet, "/api/books/999999/ocr")
	if status != http.StatusNotFound {
		t.Fatalf("status = %d: %s, want 404", status, body)
	}
	if errorOf(t, body) == "" {
		t.Error("the refusal names no reason")
	}
}
