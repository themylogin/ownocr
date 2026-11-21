package books

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// profileFor answers the argument the copy of an action records. Only OCR takes
// one, and a copy of it must carry a known profile.
func profileFor(action Action) string {
	if action.ID == ActionOCR {
		return "vl"
	}
	return ""
}

func derive(t *testing.T, s *Store, source Book, action Action) Book {
	t.Helper()
	ctx := context.Background()

	key, err := NewStorageKey()
	if err != nil {
		t.Fatalf("NewStorageKey: %v", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)

	copyBook, err := s.CreateDerivedTx(ctx, tx, source.Name, key, source.ID, action,
		profileFor(action))
	if err != nil {
		t.Fatalf("CreateDerivedTx: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	return copyBook
}

func insertPage(t *testing.T, s *Store, bookID int64, index int, source *int64) Page {
	t.Helper()
	ctx := context.Background()

	key, err := NewStorageKey()
	if err != nil {
		t.Fatalf("NewStorageKey: %v", err)
	}

	page := Page{
		BookID:       bookID,
		PageIndex:    index,
		StorageKey:   key,
		OriginalExt:  "jpg",
		Width:        400,
		Height:       600,
		SourcePageID: source,
	}
	if err := s.InsertPage(ctx, page); err != nil {
		t.Fatalf("InsertPage: %v", err)
	}

	pages, err := s.Pages(ctx, bookID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}
	for _, stored := range pages {
		if stored.PageIndex == index {
			return stored
		}
	}

	t.Fatalf("page %d is missing after the insert", index)
	return Page{}
}

func TestAnUploadedBookNamesNoSourcePage(t *testing.T) {
	s := newTestStore(t)
	book := create(t, s, "Uploaded")

	for index := 1; index <= 3; index++ {
		page := insertPage(t, s, book.ID, index, nil)
		if page.SourcePageID != nil {
			t.Errorf("page %d names source page %d, want none", index, *page.SourcePageID)
		}
	}
}

func TestACopyNamesOneSourcePageEach(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	source := create(t, s, "Source")
	sourcePages := make([]Page, 0, 3)
	for index := 1; index <= 3; index++ {
		sourcePages = append(sourcePages, insertPage(t, s, source.ID, index, nil))
	}

	copyBook := derive(t, s, source, dewarpAction(t))
	for _, page := range sourcePages {
		insertPage(t, s, copyBook.ID, page.PageIndex, &page.ID)
	}

	pages, err := s.Pages(ctx, copyBook.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}
	if len(pages) != 3 {
		t.Fatalf("got %d pages, want 3", len(pages))
	}
	for index, page := range pages {
		if page.SourcePageID == nil {
			t.Fatalf("page %d names no source page", page.PageIndex)
		}
		if *page.SourcePageID != sourcePages[index].ID {
			t.Errorf("page %d names source page %d, want %d",
				page.PageIndex, *page.SourcePageID, sourcePages[index].ID)
		}
	}
}

// The migration matches the pages that workers wrote index for index. This runs
// the same statement over pages that name no source, which is what the upgrade
// finds.
func TestTheBackfillMatchesEveryPageOfACopy(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	source := create(t, s, "Source")
	sourcePages := make([]Page, 0, 4)
	for index := 1; index <= 4; index++ {
		sourcePages = append(sourcePages, insertPage(t, s, source.ID, index, nil))
	}

	copyBook := derive(t, s, source, dewarpAction(t))
	for index := 1; index <= 4; index++ {
		insertPage(t, s, copyBook.ID, index, nil)
	}

	_, err := s.pool.Exec(ctx, `UPDATE book_page c SET source_page_id = p.id
		FROM book b JOIN book_page p ON p.book_id = b.source_book_id
		WHERE c.book_id = b.id AND c.source_page_id IS NULL
		  AND p.page_index = c.page_index`)
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}

	pages, err := s.Pages(ctx, copyBook.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}
	for index, page := range pages {
		if page.SourcePageID == nil {
			t.Fatalf("the backfill left page %d without a source page", page.PageIndex)
		}
		if *page.SourcePageID != sourcePages[index].ID {
			t.Errorf("page %d names source page %d, want %d",
				page.PageIndex, *page.SourcePageID, sourcePages[index].ID)
		}
	}

	remaining, err := s.Pages(ctx, source.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}
	for _, page := range remaining {
		if page.SourcePageID != nil {
			t.Errorf("the backfill gave the source page %d a source", page.PageIndex)
		}
	}
}

// A copy owns its bytes, so it survives the loss of the page it was made from.
func TestAPageSurvivesTheLossOfItsSourcePage(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	source := create(t, s, "Source")
	sourcePage := insertPage(t, s, source.ID, 1, nil)

	copyBook := derive(t, s, source, dewarpAction(t))
	insertPage(t, s, copyBook.ID, 1, &sourcePage.ID)

	if _, err := s.pool.Exec(ctx, `DELETE FROM book_page WHERE id = $1`, sourcePage.ID); err != nil {
		t.Fatalf("delete source page: %v", err)
	}

	pages, err := s.Pages(ctx, copyBook.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("got %d pages, want the copy to keep its page", len(pages))
	}
	if pages[0].SourcePageID != nil {
		t.Errorf("the page still names source page %d", *pages[0].SourcePageID)
	}
}

// A page of an OCR copy carries no cut side of its own. It has to reach the
// split page it was read from, which is one step further up than its source.
func TestCutSideOfWalksUpToTheSplitPage(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	source := create(t, s, "Chain")
	dewarped := derive(t, s, source, dewarpAction(t))
	split := derive(t, s, dewarped, splitAction(t))

	ocrAction, ok := ActionByID(ActionOCR)
	if !ok {
		t.Fatal("the OCR action is missing")
	}
	read := derive(t, s, split, ocrAction)

	uploaded := insertPage(t, s, source.ID, 1, nil)
	straight := insertPage(t, s, dewarped.ID, 1, &uploaded.ID)
	left := insertCutPage(t, s, split.ID, 1, &straight.ID, CutLeft, 210, 210)
	right := insertCutPage(t, s, split.ID, 2, &straight.ID, CutRight, 190, 210)
	readLeft := insertPage(t, s, read.ID, 1, &left.ID)
	readRight := insertPage(t, s, read.ID, 2, &right.ID)

	for _, c := range []struct {
		name string
		page Page
		want string
	}{
		{"the split page itself", left, CutLeft},
		{"the other split page", right, CutRight},
		{"the OCR copy of the left half", readLeft, CutLeft},
		{"the OCR copy of the right half", readRight, CutRight},
	} {
		side, ok := s.CutSideOf(ctx, c.page.ID)
		if !ok {
			t.Errorf("%s reports no cut side, want %q", c.name, c.want)
			continue
		}
		if side != c.want {
			t.Errorf("%s reports the cut side %q, want %q", c.name, side, c.want)
		}
	}

	for _, c := range []struct {
		name string
		page Page
	}{
		{"an uploaded page", uploaded},
		{"a dewarped page", straight},
	} {
		if side, ok := s.CutSideOf(ctx, c.page.ID); ok {
			t.Errorf("%s reports the cut side %q, want none", c.name, side)
		}
	}
}

func TestCutSideOfAnUnknownPage(t *testing.T) {
	s := newTestStore(t)

	if side, ok := s.CutSideOf(context.Background(), 987654); ok {
		t.Errorf("an unknown page reports the cut side %q, want none", side)
	}
}

func splitAction(t *testing.T) Action {
	t.Helper()

	action, ok := ActionByID(ActionSplit)
	if !ok {
		t.Fatal("the split action is missing")
	}
	return action
}

// chain builds an uploaded book, a dewarped copy and a split copy, each with
// one page per page of the book before it.
func chain(t *testing.T, s *Store, pageCount int) (Book, Book, Book) {
	t.Helper()

	source := create(t, s, "Chain")
	dewarped := derive(t, s, source, dewarpAction(t))
	split := derive(t, s, dewarped, splitAction(t))

	for index := 1; index <= pageCount; index++ {
		first := insertPage(t, s, source.ID, index, nil)
		second := insertPage(t, s, dewarped.ID, index, &first.ID)
		insertPage(t, s, split.ID, index, &second.ID)
	}

	return source, dewarped, split
}

func TestPageDescendantsRunDeepestFirst(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	source, dewarped, split := chain(t, s, 3)

	page, err := s.PageAt(ctx, source.ID, 2)
	if err != nil {
		t.Fatalf("PageAt: %v", err)
	}

	found, err := s.PageDescendants(ctx, page.ID)
	if err != nil {
		t.Fatalf("PageDescendants: %v", err)
	}
	if len(found) != 3 {
		t.Fatalf("got %d pages, want 3", len(found))
	}

	wantBooks := []int64{split.ID, dewarped.ID, source.ID}
	for index, entry := range found {
		if entry.BookID != wantBooks[index] {
			t.Errorf("page %d belongs to book %d, want %d", index, entry.BookID, wantBooks[index])
		}
		if entry.PageIndex != 2 {
			t.Errorf("page %d has index %d, want 2", index, entry.PageIndex)
		}
		if entry.BookStorageKey == "" {
			t.Errorf("page %d carries no book storage key", index)
		}
	}
}

func TestPageDescendantsOfALeafIsThePageAlone(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_, _, split := chain(t, s, 2)

	page, err := s.PageAt(ctx, split.ID, 1)
	if err != nil {
		t.Fatalf("PageAt: %v", err)
	}

	found, err := s.PageDescendants(ctx, page.ID)
	if err != nil {
		t.Fatalf("PageDescendants: %v", err)
	}
	if len(found) != 1 || found[0].ID != page.ID {
		t.Fatalf("got %d pages, want the page alone", len(found))
	}
}

func TestPageDescendantsOfAnUnknownPage(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.PageDescendants(context.Background(), 987654); !errors.Is(err, ErrPageNotFound) {
		t.Fatalf("err = %v, want ErrPageNotFound", err)
	}
}

func TestSourcePageReportsBothAnswers(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	source, dewarped, _ := chain(t, s, 2)

	copyPage, err := s.PageAt(ctx, dewarped.ID, 1)
	if err != nil {
		t.Fatalf("PageAt: %v", err)
	}
	found, ok := s.SourcePage(ctx, copyPage)
	if !ok {
		t.Fatal("the copy page reports no source page")
	}
	if found.BookID != source.ID || found.PageIndex != 1 {
		t.Errorf("source page is book %d page %d, want book %d page 1",
			found.BookID, found.PageIndex, source.ID)
	}

	sourcePage, err := s.PageAt(ctx, source.ID, 1)
	if err != nil {
		t.Fatalf("PageAt: %v", err)
	}
	if _, ok := s.SourcePage(ctx, sourcePage); ok {
		t.Error("an uploaded page reports a source page")
	}
}

func TestDeletePage(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	book := create(t, s, "Delete")
	page := insertPage(t, s, book.ID, 2, nil)

	if err := s.DeletePage(ctx, page.ID); err != nil {
		t.Fatalf("DeletePage: %v", err)
	}
	if err := s.DeletePage(ctx, page.ID); !errors.Is(err, ErrPageNotFound) {
		t.Fatalf("second delete: err = %v, want ErrPageNotFound", err)
	}

	if _, err := s.PageAt(ctx, book.ID, 2); !errors.Is(err, ErrPageNotFound) {
		t.Fatalf("PageAt: err = %v, want ErrPageNotFound", err)
	}
}

func TestRollBackPageKeepsIndexAndLineage(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_, dewarped, _ := chain(t, s, 2)

	before, err := s.PageAt(ctx, dewarped.ID, 2)
	if err != nil {
		t.Fatalf("PageAt: %v", err)
	}

	after, err := s.RollBackPage(ctx, before.ID, "newkey", "png", 111, 222)
	if err != nil {
		t.Fatalf("RollBackPage: %v", err)
	}

	if after.ID != before.ID {
		t.Errorf("the page changed identity: %d, want %d", after.ID, before.ID)
	}
	if after.PageIndex != before.PageIndex {
		t.Errorf("PageIndex = %d, want %d", after.PageIndex, before.PageIndex)
	}
	if after.SourcePageID == nil || *after.SourcePageID != *before.SourcePageID {
		t.Error("the page lost the source page it names")
	}
	if after.StorageKey != "newkey" || after.OriginalExt != "png" {
		t.Errorf("file = %s.%s, want newkey.png", after.StorageKey, after.OriginalExt)
	}
	if after.Width != 111 || after.Height != 222 {
		t.Errorf("size = %dx%d, want 111x222", after.Width, after.Height)
	}
	if !after.RolledBack {
		t.Error("the page is not marked as rolled back")
	}
	if before.RolledBack {
		t.Error("the page was marked as rolled back before the rollback")
	}
}

func TestCountPagesCountsRowsNotIndexes(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	book := create(t, s, "Gaps")
	for _, index := range []int{1, 2, 3, 4} {
		insertPage(t, s, book.ID, index, nil)
	}

	page, err := s.PageAt(ctx, book.ID, 2)
	if err != nil {
		t.Fatalf("PageAt: %v", err)
	}
	if err := s.DeletePage(ctx, page.ID); err != nil {
		t.Fatalf("DeletePage: %v", err)
	}

	count, err := s.CountPages(ctx, book.ID)
	if err != nil {
		t.Fatalf("CountPages: %v", err)
	}
	if count != 3 {
		t.Errorf("CountPages = %d, want 3", count)
	}

	pages, err := s.Pages(ctx, book.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}
	want := []int{1, 3, 4}
	for index, page := range pages {
		if page.PageIndex != want[index] {
			t.Errorf("page %d has index %d, want %d", index, page.PageIndex, want[index])
		}
	}
}

// The second writer of a chain must wait for the first. Without the lock both
// would read the same state and act on it.
func TestLockChainTxSerializesTwoWriters(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	source, _, _ := chain(t, s, 1)

	first, err := s.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer first.Rollback(ctx)

	if err := s.LockChainTx(ctx, first, source.ID); err != nil {
		t.Fatalf("LockChainTx: %v", err)
	}

	entered := make(chan error, 1)
	var once sync.Once
	go func() {
		second, err := s.pool.Begin(context.Background())
		if err != nil {
			once.Do(func() { entered <- err })
			return
		}
		defer second.Rollback(context.Background())

		err = s.LockChainTx(context.Background(), second, source.ID)
		once.Do(func() { entered <- err })
	}()

	select {
	case err := <-entered:
		t.Fatalf("the second writer took the lock at once: %v", err)
	case <-time.After(300 * time.Millisecond):
	}

	if err := first.Rollback(ctx); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	select {
	case err := <-entered:
		if err != nil {
			t.Fatalf("the second writer failed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the second writer never took the lock")
	}
}

func TestLockChainTxOnAnUnknownBook(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)

	if err := s.LockChainTx(ctx, tx, 987654); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestOnlyDewarpOffersARollback(t *testing.T) {
	if !dewarpAction(t).OffersRollback {
		t.Error("Dewarp offers no rollback")
	}
	if splitAction(t).OffersRollback {
		t.Error("Split offers a rollback, but it makes two pages from one")
	}

	ocr, ok := ActionByID(ActionOCR)
	if !ok {
		t.Fatal("the ocr action is missing")
	}
	if ocr.OffersRollback {
		t.Error("OCR offers a rollback, but it leaves the page as it found it")
	}
}

// insertCutPage adds a page that a split cut out of its source page, on the
// named side, at the named column.
func insertCutPage(t *testing.T, s *Store, bookID int64, index int, source *int64,
	side string, column, width int) Page {
	t.Helper()
	ctx := context.Background()

	key, err := NewStorageKey()
	if err != nil {
		t.Fatalf("NewStorageKey: %v", err)
	}

	page := Page{
		BookID:       bookID,
		PageIndex:    index,
		StorageKey:   key,
		OriginalExt:  "jpg",
		Width:        width,
		Height:       600,
		SourcePageID: source,
		CutSide:      &side,
		CutColumn:    &column,
	}
	if err := s.InsertPage(ctx, page); err != nil {
		t.Fatalf("InsertPage: %v", err)
	}

	stored, err := s.PageAt(ctx, bookID, index)
	if err != nil {
		t.Fatalf("PageAt: %v", err)
	}
	return stored.Page
}

func TestReCutPageKeepsIndexLineageAndSide(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	source := create(t, s, "Spreads")
	spread := insertPage(t, s, source.ID, 1, nil)

	split := derive(t, s, source, splitAction(t))
	insertCutPage(t, s, split.ID, 1, &spread.ID, CutLeft, 220, 220)
	before := insertCutPage(t, s, split.ID, 2, &spread.ID, CutRight, 180, 220)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)

	after, err := s.ReCutPageTx(ctx, tx, before.ID, "newkey", "jpg", 260, 600, 140)
	if err != nil {
		t.Fatalf("ReCutPageTx: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	if after.ID != before.ID {
		t.Errorf("the page changed identity: %d, want %d", after.ID, before.ID)
	}
	if after.PageIndex != before.PageIndex {
		t.Errorf("PageIndex = %d, want %d", after.PageIndex, before.PageIndex)
	}
	if after.SourcePageID == nil || *after.SourcePageID != *before.SourcePageID {
		t.Error("the page lost the source page it names")
	}
	if after.CutSide == nil || *after.CutSide != CutRight {
		t.Errorf("CutSide = %v, want %q", after.CutSide, CutRight)
	}
	if after.CutColumn == nil || *after.CutColumn != 140 {
		t.Errorf("CutColumn = %v, want 140", after.CutColumn)
	}
	if after.StorageKey != "newkey" || after.Width != 260 || after.Height != 600 {
		t.Errorf("file = %s, size = %dx%d, want newkey at 260x600",
			after.StorageKey, after.Width, after.Height)
	}
	if after.RolledBack {
		t.Error("a re-cut marked the page as rolled back")
	}

	neighbour, err := s.PageAt(ctx, split.ID, 1)
	if err != nil {
		t.Fatalf("PageAt: %v", err)
	}
	if neighbour.CutColumn == nil || *neighbour.CutColumn != 220 {
		t.Errorf("the other page of the spread now records %v, want 220", neighbour.CutColumn)
	}
}
