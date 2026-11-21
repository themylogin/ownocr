package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/themylogin/ownocr/backend/internal/books"
	"github.com/themylogin/ownocr/backend/internal/filestore"
	"github.com/themylogin/ownocr/backend/internal/ingest"
)

func (h *pageHarness) send(t *testing.T, method, path, body string) (int, []byte) {
	t.Helper()

	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	h.router.ServeHTTP(recorder, request)
	return recorder.Code, recorder.Body.Bytes()
}

// addCutPage adds a page that a split cut out of its source page, on the named
// side, at the named column.
func (h *pageHarness) addCutPage(t *testing.T, book books.Book, index int, source *int64,
	side string, column, width int, shade uint8) books.PageOfBook {
	t.Helper()
	ctx := context.Background()

	page := h.addPage(t, book, index, source, width, 600, shade)
	if _, err := h.store.Pool().Exec(ctx,
		`UPDATE book_page SET cut_side = $2, cut_column = $3 WHERE id = $1`,
		page.ID, side, column); err != nil {
		t.Fatalf("record the cut: %v", err)
	}

	stored, err := h.store.PageAt(ctx, book.ID, index)
	if err != nil {
		t.Fatalf("PageAt: %v", err)
	}
	return stored
}

// seedSplit builds a complete uploaded book of a cover and a spread, and the
// complete split copy that copied the first whole and cut the second in two.
// The two halves overlap, as a leaning gutter makes them.
func (h *pageHarness) seedSplit(t *testing.T) (books.Book, books.Book) {
	t.Helper()

	source := h.createBook(t, "Spreads")
	cover := h.addPage(t, source, 1, nil, 400, 600, 30)
	spread := h.addPage(t, source, 2, nil, 400, 600, 90)
	source = h.complete(t, source, 2)

	split := h.deriveBook(t, source, books.ActionSplit)
	h.addPage(t, split, 1, &cover.ID, 400, 600, 30)
	h.addCutPage(t, split, 2, &spread.ID, books.CutLeft, 220, 220, 60)
	h.addCutPage(t, split, 3, &spread.ID, books.CutRight, 180, 220, 120)

	return source, h.complete(t, split, 3)
}

func (h *pageHarness) cutOf(t *testing.T, bookID int64, index int) pageCutView {
	t.Helper()

	status, body := h.do(t, http.MethodGet, fmt.Sprintf("/api/books/%d/pages/%d/cut",
		bookID, index))
	if status != http.StatusOK {
		t.Fatalf("GET cut = %d: %s", status, body)
	}

	var view pageCutView
	if err := json.Unmarshal(body, &view); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return view
}

// storedWidth reads the width of the file itself, which is what the reader
// shows. The row may agree with it and still be wrong.
func (h *pageHarness) storedWidth(t *testing.T, book books.Book, page books.PageOfBook) int {
	t.Helper()

	img, err := ingest.Decode(h.files.PagePath(book.StorageKey, page.StorageKey,
		filestore.RenditionOriginal, page.OriginalExt))
	if err != nil {
		t.Fatalf("decode page %d: %v", page.PageIndex, err)
	}
	return img.Bounds().Dx()
}

func TestPageCutAnswersTheParentAndTheCut(t *testing.T) {
	h := newPageHarness(t)
	ctx := context.Background()
	source, split := h.seedSplit(t)

	view := h.cutOf(t, split.ID, 2)

	if view.Side != books.CutLeft || view.Column != 220 {
		t.Errorf("cut = %s at %d, want left at 220", view.Side, view.Column)
	}
	if view.Source.Width != 400 || view.Source.Height != 600 {
		t.Errorf("source = %dx%d, want the spread 400x600",
			view.Source.Width, view.Source.Height)
	}

	spread, err := h.store.PageAt(ctx, source.ID, 2)
	if err != nil {
		t.Fatalf("PageAt: %v", err)
	}
	want := filestore.PageURL(source.StorageKey, spread.StorageKey,
		filestore.RenditionDisplay, "jpg")
	if view.Source.DisplayURL != want {
		t.Errorf("displayUrl = %q, want the spread at %q", view.Source.DisplayURL, want)
	}

	if right := h.cutOf(t, split.ID, 3); right.Side != books.CutRight || right.Column != 180 {
		t.Errorf("the right page reports %s at %d, want right at 180", right.Side, right.Column)
	}
}

func TestApplyPageCutMovesOneEdge(t *testing.T) {
	h := newPageHarness(t)
	ctx := context.Background()
	_, split := h.seedSplit(t)

	before, err := h.store.PageAt(ctx, split.ID, 2)
	if err != nil {
		t.Fatalf("PageAt: %v", err)
	}
	neighbour, err := h.store.PageAt(ctx, split.ID, 3)
	if err != nil {
		t.Fatalf("PageAt: %v", err)
	}

	status, body := h.send(t, http.MethodPost,
		fmt.Sprintf("/api/books/%d/pages/2/cut", split.ID), `{"column":300}`)
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
	if view.Width != 300 || view.Height != 600 {
		t.Errorf("size = %dx%d, want the new cut 300x600", view.Width, view.Height)
	}
	if view.CutSide == nil || *view.CutSide != books.CutLeft {
		t.Errorf("the page reports the side %v, want left", view.CutSide)
	}

	after, err := h.store.PageAt(ctx, split.ID, 2)
	if err != nil {
		t.Fatalf("PageAt: %v", err)
	}
	if after.ID != before.ID {
		t.Error("the page changed identity")
	}
	if after.StorageKey == before.StorageKey {
		t.Error("the page kept its storage key, so a cached image would survive")
	}
	if got := h.storedWidth(t, split, after); got != 300 {
		t.Errorf("the stored image is %d wide, want 300", got)
	}
	if after.CutColumn == nil || *after.CutColumn != 300 {
		t.Errorf("the page records the column %v, want 300", after.CutColumn)
	}

	// One save moves one page. The other half of the spread keeps its own cut.
	stillThere, err := h.store.PageAt(ctx, split.ID, 3)
	if err != nil {
		t.Fatalf("PageAt: %v", err)
	}
	if stillThere.StorageKey != neighbour.StorageKey || stillThere.Width != neighbour.Width {
		t.Error("the other page of the spread changed")
	}

	if got := h.fileCount(t, split); got != 9 {
		t.Errorf("the copy holds %d files, want the 9 of its three pages", got)
	}

	reloaded, err := h.store.Get(ctx, split.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if reloaded.PageCount != 3 {
		t.Errorf("PageCount = %d, want 3", reloaded.PageCount)
	}
}

// Every cut is taken out of the source page, so a user who cut too far can cut
// back out and lose no column.
func TestApplyPageCutAlwaysCutsFromTheSourcePage(t *testing.T) {
	h := newPageHarness(t)
	ctx := context.Background()
	_, split := h.seedSplit(t)

	for _, column := range []int{140, 330} {
		status, body := h.send(t, http.MethodPost,
			fmt.Sprintf("/api/books/%d/pages/2/cut", split.ID),
			fmt.Sprintf(`{"column":%d}`, column))
		if status != http.StatusOK {
			t.Fatalf("cut at %d = %d: %s", column, status, body)
		}
	}

	page, err := h.store.PageAt(ctx, split.ID, 2)
	if err != nil {
		t.Fatalf("PageAt: %v", err)
	}
	if page.Width != 330 {
		t.Errorf("the page is %d wide, want the 330 of the second cut", page.Width)
	}
	if got := h.storedWidth(t, split, page); got != 330 {
		t.Errorf("the stored image is %d wide, want 330", got)
	}
}

func TestApplyPageCutOnARightPageTakesTheColumnAndEveryColumnAfter(t *testing.T) {
	h := newPageHarness(t)
	ctx := context.Background()
	_, split := h.seedSplit(t)

	status, body := h.send(t, http.MethodPost,
		fmt.Sprintf("/api/books/%d/pages/3/cut", split.ID), `{"column":100}`)
	if status != http.StatusOK {
		t.Fatalf("status = %d: %s", status, body)
	}

	page, err := h.store.PageAt(ctx, split.ID, 3)
	if err != nil {
		t.Fatalf("PageAt: %v", err)
	}
	if page.Width != 300 {
		t.Errorf("the page is %d wide, want 400 - 100", page.Width)
	}
	if got := h.storedWidth(t, split, page); got != 300 {
		t.Errorf("the stored image is %d wide, want 300", got)
	}
}

func TestPageCutRefusesAPageThatRecordsNoCut(t *testing.T) {
	h := newPageHarness(t)
	_, split := h.seedSplit(t)

	status, body := h.do(t, http.MethodGet, fmt.Sprintf("/api/books/%d/pages/1/cut", split.ID))
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d: %s, want 422", status, body)
	}
	if errorOf(t, body) == "" {
		t.Error("the refusal named no reason")
	}

	status, body = h.send(t, http.MethodPost,
		fmt.Sprintf("/api/books/%d/pages/1/cut", split.ID), `{"column":200}`)
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("apply status = %d: %s, want 422", status, body)
	}
}

func TestPageCutRefusesABookWithACopy(t *testing.T) {
	h := newPageHarness(t)
	_, split := h.seedSplit(t)

	later := h.deriveBook(t, split, books.ActionSplit)
	h.complete(t, later, 0)

	status, body := h.do(t, http.MethodGet, fmt.Sprintf("/api/books/%d/pages/2/cut", split.ID))
	if status != http.StatusConflict {
		t.Fatalf("status = %d: %s, want 409", status, body)
	}
	if errorOf(t, body) == "" {
		t.Error("the refusal named no reason")
	}
}

func TestPageCutRefusesWhenTheSourcePageIsGone(t *testing.T) {
	h := newPageHarness(t)
	ctx := context.Background()
	source, split := h.seedSplit(t)

	spread, err := h.store.PageAt(ctx, source.ID, 2)
	if err != nil {
		t.Fatalf("PageAt: %v", err)
	}
	if _, err := h.store.Pool().Exec(ctx,
		`DELETE FROM book_page WHERE id = $1`, spread.ID); err != nil {
		t.Fatalf("delete source page: %v", err)
	}

	status, body := h.do(t, http.MethodGet, fmt.Sprintf("/api/books/%d/pages/2/cut", split.ID))
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d: %s, want 422", status, body)
	}
}

func TestApplyPageCutRefusesAColumnThatLeavesNoPage(t *testing.T) {
	h := newPageHarness(t)
	ctx := context.Background()
	_, split := h.seedSplit(t)

	before, err := h.store.PageAt(ctx, split.ID, 2)
	if err != nil {
		t.Fatalf("PageAt: %v", err)
	}

	cases := []struct {
		name  string
		index int
		body  string
	}{
		{"the left page ends at its first column", 2, `{"column":0}`},
		{"the right page starts past its last column", 3, `{"column":400}`},
		{"the column lies outside the spread", 2, `{"column":401}`},
		{"the column is below zero", 2, `{"column":-1}`},
	}

	for _, test := range cases {
		status, body := h.send(t, http.MethodPost,
			fmt.Sprintf("/api/books/%d/pages/%d/cut", split.ID, test.index), test.body)
		if status != http.StatusUnprocessableEntity {
			t.Errorf("%s: status = %d: %s, want 422", test.name, status, body)
			continue
		}
		if errorOf(t, body) == "" {
			t.Errorf("%s: the refusal named no reason", test.name)
		}
	}

	after, err := h.store.PageAt(ctx, split.ID, 2)
	if err != nil {
		t.Fatalf("PageAt: %v", err)
	}
	if after.StorageKey != before.StorageKey || after.Width != before.Width {
		t.Error("a refused cut changed the page")
	}
}

func TestApplyPageCutNeedsAColumn(t *testing.T) {
	h := newPageHarness(t)
	_, split := h.seedSplit(t)

	status, body := h.send(t, http.MethodPost,
		fmt.Sprintf("/api/books/%d/pages/2/cut", split.ID), `{}`)
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d: %s, want 422", status, body)
	}

	status, body = h.send(t, http.MethodPost,
		fmt.Sprintf("/api/books/%d/pages/2/cut", split.ID), `not json`)
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d: %s, want 400", status, body)
	}
}

func TestPageCutRefusesWhileTheBookRuns(t *testing.T) {
	h := newPageHarness(t)
	ctx := context.Background()
	_, split := h.seedSplit(t)

	before, err := h.store.PageAt(ctx, split.ID, 2)
	if err != nil {
		t.Fatalf("PageAt: %v", err)
	}
	if _, err := h.store.SetProcessing(ctx, split.ID, books.PhaseConverting); err != nil {
		t.Fatalf("SetProcessing: %v", err)
	}

	status, body := h.send(t, http.MethodPost,
		fmt.Sprintf("/api/books/%d/pages/2/cut", split.ID), `{"column":300}`)
	if status != http.StatusConflict {
		t.Fatalf("status = %d: %s, want 409", status, body)
	}

	after, err := h.store.PageAt(ctx, split.ID, 2)
	if err != nil {
		t.Fatalf("PageAt: %v", err)
	}
	if after.StorageKey != before.StorageKey {
		t.Error("the refused cut replaced the page files")
	}
}

func TestPageCutOfAnUnknownPage(t *testing.T) {
	h := newPageHarness(t)
	_, split := h.seedSplit(t)

	status, _ := h.do(t, http.MethodGet, fmt.Sprintf("/api/books/%d/pages/9/cut", split.ID))
	if status != http.StatusNotFound {
		t.Errorf("status = %d, want 404", status)
	}

	status, _ = h.send(t, http.MethodPost,
		fmt.Sprintf("/api/books/%d/pages/9/cut", split.ID), `{"column":200}`)
	if status != http.StatusNotFound {
		t.Errorf("apply status = %d, want 404", status)
	}

	status, _ = h.do(t, http.MethodGet, "/api/books/987654/pages/1/cut")
	if status != http.StatusNotFound {
		t.Errorf("unknown book status = %d, want 404", status)
	}
}

func TestBookDetailReportsTheCutOfEveryPage(t *testing.T) {
	h := newPageHarness(t)
	source, split := h.seedSplit(t)

	detail := h.detail(t, split.ID)
	if !detail.PageCut {
		t.Error("a complete split copy with no copy offers no re-cut")
	}

	sides := map[int]string{}
	for _, page := range detail.Pages {
		if page.CutSide != nil {
			sides[page.Index] = *page.CutSide
		}
	}
	if len(sides) != 2 || sides[2] != books.CutLeft || sides[3] != books.CutRight {
		t.Errorf("the pages report the sides %v, want left at 2 and right at 3", sides)
	}

	for _, page := range h.detail(t, source.ID).Pages {
		if page.CutSide != nil {
			t.Errorf("the uploaded page %d reports the side %q", page.Index, *page.CutSide)
		}
	}

	later := h.deriveBook(t, split, books.ActionSplit)
	h.complete(t, later, 0)

	if detail := h.detail(t, split.ID); detail.PageCut {
		t.Error("a book that already has a copy offers a re-cut")
	}
}
