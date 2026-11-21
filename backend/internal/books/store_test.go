package books

import (
	"context"
	"testing"

	"github.com/themylogin/ownocr/backend/internal/testdb"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()

	pool := testdb.New(t)

	return NewStore(pool)
}

func create(t *testing.T, s *Store, name string) Book {
	t.Helper()

	key, err := NewStorageKey()
	if err != nil {
		t.Fatalf("NewStorageKey: %v", err)
	}

	tx, err := s.pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(context.Background())

	book, err := s.CreateTx(context.Background(), tx, name, key)
	if err != nil {
		t.Fatalf("CreateTx: %v", err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatalf("commit: %v", err)
	}

	return book
}

func dewarpAction(t *testing.T) Action {
	t.Helper()

	action, ok := ActionByID(ActionDewarp)
	if !ok {
		t.Fatal("the dewarp action is missing")
	}
	return action
}

func phaseOf(book Book) string {
	if book.Progress == nil {
		return ""
	}
	return *book.Progress
}

func TestNewStorageKey(t *testing.T) {
	first, err := NewStorageKey()
	if err != nil {
		t.Fatalf("NewStorageKey: %v", err)
	}
	second, err := NewStorageKey()
	if err != nil {
		t.Fatalf("NewStorageKey: %v", err)
	}

	if first == second {
		t.Error("two keys are identical")
	}
	if len(first) != 32 {
		t.Errorf("key length = %d, want 32", len(first))
	}
}

func TestEmptyListsAreNotNil(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	all, err := s.ListAll(ctx)
	if err != nil || all == nil {
		t.Fatalf("ListAll = %v, %v", all, err)
	}
	staged, err := s.ListByStage(ctx, StageUploaded)
	if err != nil || staged == nil {
		t.Fatalf("ListByStage = %v, %v", staged, err)
	}
	pages, err := s.Pages(ctx, 1)
	if err != nil || pages == nil {
		t.Fatalf("Pages = %v, %v", pages, err)
	}
}

func TestCreateStartsPendingInTheFirstStage(t *testing.T) {
	s := newTestStore(t)
	book := create(t, s, "Moby Dick")

	if book.State != StatePending {
		t.Errorf("State = %q, want %q", book.State, StatePending)
	}
	if book.Stage != StageUploaded {
		t.Errorf("Stage = %q, want %q", book.Stage, StageUploaded)
	}
	if phaseOf(book) != PhaseQueued {
		t.Errorf("Progress = %q, want %q", phaseOf(book), PhaseQueued)
	}
	if book.Action != nil {
		t.Errorf("Action = %q, want none", *book.Action)
	}

	staged, err := s.ListByStage(context.Background(), StageUploaded)
	if err != nil {
		t.Fatalf("ListByStage: %v", err)
	}
	if len(staged) != 1 {
		t.Fatalf("ListByStage returned %d books, want 1", len(staged))
	}
}

// The stage is set once, at creation, and no state change may touch it.
func TestStateRoundTripKeepsTheStage(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	book := create(t, s, "Round Trip")

	processing, err := s.SetProcessing(ctx, book.ID, PhaseConverting)
	if err != nil {
		t.Fatalf("SetProcessing: %v", err)
	}
	if processing.State != StateProcessing || processing.Stage != StageUploaded {
		t.Errorf("processing = %+v", processing)
	}
	if phaseOf(processing) != PhaseConverting {
		t.Errorf("Progress = %q, want %q", phaseOf(processing), PhaseConverting)
	}

	complete, err := s.SetComplete(ctx, book.ID, 12)
	if err != nil {
		t.Fatalf("SetComplete: %v", err)
	}
	if complete.State != StateComplete || complete.Stage != StageUploaded {
		t.Errorf("complete = %+v", complete)
	}
	if complete.PageCount != 12 {
		t.Errorf("PageCount = %d, want 12", complete.PageCount)
	}
	if complete.Progress != nil {
		t.Errorf("a complete book kept the phase %q", *complete.Progress)
	}

	staged, err := s.ListByStage(ctx, StageUploaded)
	if err != nil || len(staged) != 1 {
		t.Fatalf("ListByStage returned %d books, %v", len(staged), err)
	}
	count, err := s.CountByStage(ctx, StageUploaded)
	if err != nil || count != 1 {
		t.Fatalf("CountByStage = %d, %v", count, err)
	}

	failed, err := s.SetFailed(ctx, book.ID, "boom")
	if err != nil {
		t.Fatalf("SetFailed: %v", err)
	}
	if failed.State != StateFailed || failed.Error == nil || *failed.Error != "boom" {
		t.Errorf("failed = %+v", failed)
	}
	if failed.Stage != StageUploaded {
		t.Errorf("a failed book left its stage: %q", failed.Stage)
	}
	if failed.Progress != nil {
		t.Errorf("a failed book kept the phase %q", *failed.Progress)
	}

	staged, err = s.ListByStage(ctx, StageUploaded)
	if err != nil || len(staged) != 1 {
		t.Fatalf("a failed book left its stage list: %d, %v", len(staged), err)
	}
}

func TestSetProgress(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	book := create(t, s, "Phased")

	updated, err := s.SetProgress(ctx, book.ID, PhaseWaitingAI)
	if err != nil {
		t.Fatalf("SetProgress: %v", err)
	}
	if phaseOf(updated) != PhaseWaitingAI {
		t.Errorf("Progress = %q, want %q", phaseOf(updated), PhaseWaitingAI)
	}
	if updated.State != StatePending || updated.Stage != StageUploaded {
		t.Errorf("SetProgress changed more than the phase: %+v", updated)
	}
}

// The five state groups must come back in the order the list shows them, and
// the newest book must lead its own group.
func TestListByStageOrder(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	action := dewarpAction(t)

	ready := create(t, s, "ready")
	if _, err := s.SetComplete(ctx, ready.ID, 1); err != nil {
		t.Fatalf("SetComplete: %v", err)
	}

	readyNewer := create(t, s, "ready-newer")
	if _, err := s.SetComplete(ctx, readyNewer.ID, 1); err != nil {
		t.Fatalf("SetComplete: %v", err)
	}

	working := create(t, s, "working")
	if _, err := s.SetComplete(ctx, working.ID, 1); err != nil {
		t.Fatalf("SetComplete: %v", err)
	}
	createDerived(t, s, working.Name, working.ID, action)

	done := create(t, s, "done")
	if _, err := s.SetComplete(ctx, done.ID, 1); err != nil {
		t.Fatalf("SetComplete: %v", err)
	}
	doneCopy := createDerived(t, s, done.Name, done.ID, action)
	if _, err := s.SetComplete(ctx, doneCopy.ID, 1); err != nil {
		t.Fatalf("SetComplete: %v", err)
	}

	running := create(t, s, "running")
	if _, err := s.SetProcessing(ctx, running.ID, PhaseConverting); err != nil {
		t.Fatalf("SetProcessing: %v", err)
	}

	failed := create(t, s, "failed")
	if _, err := s.SetFailed(ctx, failed.ID, "boom"); err != nil {
		t.Fatalf("SetFailed: %v", err)
	}

	list, err := s.ListByStage(ctx, StageUploaded)
	if err != nil {
		t.Fatalf("ListByStage: %v", err)
	}

	want := []string{"failed", "running", "ready-newer", "ready", "working", "done"}
	if len(list) != len(want) {
		t.Fatalf("ListByStage returned %d books, want %d", len(list), len(want))
	}
	for index, name := range want {
		if list[index].Name != name {
			names := make([]string, 0, len(list))
			for _, book := range list {
				names = append(names, book.Name)
			}
			t.Fatalf("order = %v, want %v", names, want)
		}
	}
}

func TestPagesRoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	book := create(t, s, "With Pages")

	for index := 3; index >= 1; index-- {
		err := s.InsertPage(ctx, Page{
			BookID:      book.ID,
			PageIndex:   index,
			StorageKey:  "key" + string(rune('0'+index)),
			OriginalExt: "jpg",
			Width:       100 * index,
			Height:      200 * index,
		})
		if err != nil {
			t.Fatalf("InsertPage: %v", err)
		}
	}

	pages, err := s.Pages(ctx, book.ID)
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
	}

	if err := s.DeletePages(ctx, book.ID); err != nil {
		t.Fatalf("DeletePages: %v", err)
	}
	pages, err = s.Pages(ctx, book.ID)
	if err != nil || len(pages) != 0 {
		t.Fatalf("after delete: %d pages, %v", len(pages), err)
	}
}

func TestPageOCRInFollowsTheOrderOfTheModels(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	book := create(t, s, "Read")

	pageID, err := s.InsertPageID(ctx, Page{
		BookID: book.ID, PageIndex: 1, StorageKey: "one", OriginalExt: "jpg",
		Width: 100, Height: 200,
	})
	if err != nil {
		t.Fatalf("InsertPageID: %v", err)
	}

	for _, model := range []string{"v6", "vl"} {
		err := s.InsertPageOCR(ctx, PageOCR{
			PageID: pageID, Model: model, Text: model + " text",
			Data: []byte(`{"lines": []}`),
		}, nil)
		if err != nil {
			t.Fatalf("InsertPageOCR %s: %v", model, err)
		}
	}

	models := []string{"merged", "v6"}

	result, held, err := s.PageOCRIn(ctx, pageID, models)
	if err != nil || !held {
		t.Fatalf("PageOCRIn: %v, held %v", err, held)
	}
	if result.Model != "v6" {
		t.Errorf("the page answered %q, want the v6 result while it holds no merged one", result.Model)
	}

	err = s.InsertPageOCR(ctx, PageOCR{
		PageID: pageID, Model: "merged", Text: "merged text", Data: []byte(`{"lines": []}`),
	}, nil)
	if err != nil {
		t.Fatalf("InsertPageOCR merged: %v", err)
	}

	result, held, err = s.PageOCRIn(ctx, pageID, models)
	if err != nil || !held {
		t.Fatalf("PageOCRIn: %v, held %v", err, held)
	}
	if result.Model != "merged" {
		t.Errorf("the page answered %q, want the merged result", result.Model)
	}
	if result.Text != "merged text" {
		t.Errorf("the page answered the text %q, want the merged text", result.Text)
	}
}

// modelsOf answers the model of every result a page holds, in the order the
// store lists them.
func modelsOf(t *testing.T, s *Store, pageID int64) []string {
	t.Helper()

	results, err := s.PageOCRResults(context.Background(), pageID)
	if err != nil {
		t.Fatalf("PageOCRResults: %v", err)
	}

	models := make([]string, 0, len(results))
	for _, result := range results {
		models = append(models, result.Model)
	}
	return models
}

// seedReadPage answers a page holding one result per named model.
func seedReadPage(t *testing.T, s *Store, bookID int64, models ...string) int64 {
	t.Helper()
	ctx := context.Background()

	pageID, err := s.InsertPageID(ctx, Page{
		BookID: bookID, PageIndex: 1, StorageKey: "one", OriginalExt: "jpg",
		Width: 100, Height: 200,
	})
	if err != nil {
		t.Fatalf("InsertPageID: %v", err)
	}

	for _, model := range models {
		err := s.InsertPageOCR(ctx, PageOCR{
			PageID: pageID, Model: model, Text: model + " text",
			Data: []byte(`{"lines": []}`),
		}, nil)
		if err != nil {
			t.Fatalf("InsertPageOCR %s: %v", model, err)
		}
	}

	return pageID
}

// A normal rescan replaces the line reading of a page and takes the visual
// reading and the merged result with it. The merged result derives from boxes
// the new reading replaced, so it no longer describes the page.
func TestReplacePageOCRStoresOneResultAndRemovesTheNamedOnes(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	book := create(t, s, "Read")
	pageID := seedReadPage(t, s, book.ID, "v6", "vl", "merged")

	err := s.ReplacePageOCR(ctx, PageOCR{
		PageID: pageID, Model: "v6", Text: "read again", Data: []byte(`{"lines": [1]}`),
	}, []string{"vl", "merged"}, nil)
	if err != nil {
		t.Fatalf("ReplacePageOCR: %v", err)
	}

	if models := modelsOf(t, s, pageID); len(models) != 1 || models[0] != "v6" {
		t.Errorf("the page holds %v, want the v6 result alone", models)
	}

	result, held, err := s.PageOCRIn(ctx, pageID, []string{"merged", "v6"})
	if err != nil || !held {
		t.Fatalf("PageOCRIn: %v, held %v", err, held)
	}
	if result.Model != "v6" || result.Text != "read again" {
		t.Errorf("the page answered %q %q, want the v6 text of this reading",
			result.Model, result.Text)
	}
}

func TestReplacePageOCRRemovesNothingElse(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	book := create(t, s, "Read")
	pageID := seedReadPage(t, s, book.ID, "v6")

	err := s.ReplacePageOCR(ctx, PageOCR{
		PageID: pageID, Model: "v6", Text: "read again", Data: []byte(`{"lines": [1]}`),
	}, []string{"vl", "merged"}, nil)
	if err != nil {
		t.Fatalf("ReplacePageOCR: %v", err)
	}

	if models := modelsOf(t, s, pageID); len(models) != 1 || models[0] != "v6" {
		t.Errorf("the page holds %v, want the v6 result alone", models)
	}

	result, _, err := s.PageOCRIn(ctx, pageID, []string{"v6"})
	if err != nil {
		t.Fatalf("PageOCRIn: %v", err)
	}
	if result.Text != "read again" {
		t.Errorf("the page holds the text %q, want the text of this reading", result.Text)
	}
}

// A remove list naming the result being stored must not remove it. The page
// would otherwise end with no reading at all.
func TestReplacePageOCRKeepsTheResultItStores(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	book := create(t, s, "Read")
	pageID := seedReadPage(t, s, book.ID, "v6", "vl")

	err := s.ReplacePageOCR(ctx, PageOCR{
		PageID: pageID, Model: "v6", Text: "read again", Data: []byte(`{"lines": [1]}`),
	}, []string{"v6", "vl"}, nil)
	if err != nil {
		t.Fatalf("ReplacePageOCR: %v", err)
	}

	if models := modelsOf(t, s, pageID); len(models) != 1 || models[0] != "v6" {
		t.Errorf("the page holds %v, want the v6 result alone", models)
	}
}

// A write that cannot land must remove nothing. The page keeps the reading it
// held, which is the state a failed rescan leaves everywhere else.
func TestReplacePageOCRKeepsEveryResultWhenItFails(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	book := create(t, s, "Read")
	pageID := seedReadPage(t, s, book.ID, "v6", "vl", "merged")

	err := s.ReplacePageOCR(ctx, PageOCR{
		PageID: pageID, Model: "v6", Text: "read again", Data: []byte(`not json`),
	}, []string{"vl", "merged"}, nil)
	if err == nil {
		t.Fatal("an unreadable result was stored")
	}

	models := modelsOf(t, s, pageID)
	if len(models) != 3 {
		t.Fatalf("the page holds %v, want the three results it held", models)
	}

	result, _, err := s.PageOCRIn(ctx, pageID, []string{"v6"})
	if err != nil {
		t.Fatalf("PageOCRIn: %v", err)
	}
	if result.Text != "v6 text" {
		t.Errorf("the page holds the text %q, want the text it held", result.Text)
	}
}

func TestPageOCRInAnswersNothingForAPageWithNoResult(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	book := create(t, s, "Unread")

	pageID, err := s.InsertPageID(ctx, Page{
		BookID: book.ID, PageIndex: 1, StorageKey: "one", OriginalExt: "jpg",
		Width: 100, Height: 200,
	})
	if err != nil {
		t.Fatalf("InsertPageID: %v", err)
	}

	err = s.InsertPageOCR(ctx, PageOCR{
		PageID: pageID, Model: "vl", Text: "blocks", Data: []byte(`{"parsing_res_list": []}`),
	}, nil)
	if err != nil {
		t.Fatalf("InsertPageOCR: %v", err)
	}

	if _, held, err := s.PageOCRIn(ctx, pageID, []string{"merged", "v6"}); err != nil || held {
		t.Fatalf("a page holding the vl result alone answered held %v, %v", held, err)
	}
}

func TestDuplicatePageIndexRejected(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	book := create(t, s, "Duplicate")

	page := Page{BookID: book.ID, PageIndex: 1, StorageKey: "a", OriginalExt: "jpg", Width: 1, Height: 1}
	if err := s.InsertPage(ctx, page); err != nil {
		t.Fatalf("first insert: %v", err)
	}

	page.StorageKey = "b"
	if err := s.InsertPage(ctx, page); err == nil {
		t.Fatal("a duplicate page index was accepted")
	}
}

func TestGetUnknownBook(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.Get(context.Background(), 999999); err != ErrNotFound {
		t.Fatalf("Get returned %v, want ErrNotFound", err)
	}
}

func createDerived(t *testing.T, s *Store, name string, sourceID int64, action Action) Book {
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

	book, err := s.CreateDerivedTx(ctx, tx, name, key, sourceID, action, profileFor(action))
	if err != nil {
		t.Fatalf("CreateDerivedTx: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	return book
}

func TestDerivedBookReportsItsStateOnTheSource(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	action := dewarpAction(t)

	source := create(t, s, "Atlas")
	if source.DerivedState != nil {
		t.Fatalf("a book without a copy reports %q", *source.DerivedState)
	}

	copyBook := createDerived(t, s, source.Name, source.ID, action)
	if copyBook.Name != source.Name {
		t.Errorf("copy name = %q, want %q", copyBook.Name, source.Name)
	}
	if copyBook.SourceBookID == nil || *copyBook.SourceBookID != source.ID {
		t.Errorf("copy references %v, want %d", copyBook.SourceBookID, source.ID)
	}
	if copyBook.State != StatePending || copyBook.Stage != StageDewarped {
		t.Errorf("copy = %+v, want pending in the dewarped stage", copyBook)
	}
	if copyBook.Action == nil || *copyBook.Action != ActionDewarp {
		t.Errorf("copy action = %v, want %q", copyBook.Action, ActionDewarp)
	}
	if copyBook.DerivedState != nil {
		t.Error("the copy reports a copy of its own")
	}

	assertDerived := func(want string) {
		t.Helper()

		reloaded, err := s.Get(ctx, source.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if want == "" {
			if reloaded.DerivedState != nil {
				t.Errorf("DerivedState = %q, want none", *reloaded.DerivedState)
			}
			return
		}
		if reloaded.DerivedState == nil || *reloaded.DerivedState != want {
			t.Errorf("DerivedState = %v, want %q", reloaded.DerivedState, want)
		}
	}

	assertDerived(StatePending)

	if _, err := s.SetProcessing(ctx, copyBook.ID, PhaseUploading); err != nil {
		t.Fatalf("SetProcessing: %v", err)
	}
	assertDerived(StateProcessing)

	if _, err := s.SetFailed(ctx, copyBook.ID, "boom"); err != nil {
		t.Fatalf("SetFailed: %v", err)
	}
	assertDerived(StateFailed)

	if _, err := s.SetComplete(ctx, copyBook.ID, 3); err != nil {
		t.Fatalf("SetComplete: %v", err)
	}
	assertDerived(StateComplete)

	if _, err := s.pool.Exec(ctx, "DELETE FROM book WHERE id = $1", copyBook.ID); err != nil {
		t.Fatalf("delete copy: %v", err)
	}
	assertDerived("")
}

func TestDerivedStateSurvivesEveryListing(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	action := dewarpAction(t)

	source := create(t, s, "Listed")
	if _, err := s.SetComplete(ctx, source.ID, 1); err != nil {
		t.Fatalf("SetComplete: %v", err)
	}
	createDerived(t, s, source.Name, source.ID, action)

	staged, err := s.ListByStage(ctx, StageUploaded)
	if err != nil || len(staged) != 1 {
		t.Fatalf("ListByStage returned %d books, %v", len(staged), err)
	}
	if staged[0].DerivedState == nil {
		t.Error("ListByStage lost the copy state")
	}

	all, err := s.ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	for _, book := range all {
		if book.ID == source.ID && book.DerivedState == nil {
			t.Error("ListAll lost the copy state")
		}
	}

	inFlight, err := s.ListByStage(ctx, StageDewarped)
	if err != nil || len(inFlight) != 1 {
		t.Fatalf("the copy is not listed in its own stage: %d, %v", len(inFlight), err)
	}
	if inFlight[0].SourceBookID == nil {
		t.Error("the copy lost the source reference")
	}
}

func TestRequeueKeepsTheSameCopy(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	source := create(t, s, "Retried")
	copyBook := createDerived(t, s, source.Name, source.ID, dewarpAction(t))

	if _, err := s.SetFailed(ctx, copyBook.ID, "boom"); err != nil {
		t.Fatalf("SetFailed: %v", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)

	requeued, err := s.RequeueTx(ctx, tx, copyBook.ID)
	if err != nil {
		t.Fatalf("RequeueTx: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	if requeued.ID != copyBook.ID {
		t.Errorf("retry made book %d, want %d", requeued.ID, copyBook.ID)
	}
	if requeued.State != StatePending || requeued.Error != nil {
		t.Errorf("requeued = %+v", requeued)
	}
	if requeued.Stage != StageDewarped {
		t.Errorf("a retry moved the copy to stage %q", requeued.Stage)
	}
	if phaseOf(requeued) != PhaseQueued {
		t.Errorf("Progress = %q, want %q", phaseOf(requeued), PhaseQueued)
	}
	if requeued.SourceBookID == nil || *requeued.SourceBookID != source.ID {
		t.Error("the retry lost the source reference")
	}

	all, err := s.ListAll(ctx)
	if err != nil || len(all) != 2 {
		t.Fatalf("ListAll returned %d books, want 2", len(all))
	}
}

func TestDerivedTxAndLockTx(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	source := create(t, s, "Locked")

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)

	if err := s.LockTx(ctx, tx, source.ID); err != nil {
		t.Fatalf("LockTx: %v", err)
	}
	if err := s.LockTx(ctx, tx, 999999); err != ErrNotFound {
		t.Fatalf("LockTx on an unknown book returned %v", err)
	}
	if _, err := s.DerivedTx(ctx, tx, source.ID); err != ErrNotFound {
		t.Fatalf("DerivedTx returned %v, want ErrNotFound", err)
	}

	if _, err := s.CreateDerivedTx(ctx, tx, source.Name, "derivedkey", source.ID,
		dewarpAction(t), ""); err != nil {
		t.Fatalf("CreateDerivedTx: %v", err)
	}

	found, err := s.DerivedTx(ctx, tx, source.ID)
	if err != nil {
		t.Fatalf("DerivedTx: %v", err)
	}
	if found.SourceBookID == nil || *found.SourceBookID != source.ID {
		t.Errorf("DerivedTx returned %+v", found)
	}
}

// readPage stores a page and one result per named model, and answers the page id.
func readPage(t *testing.T, s *Store, bookID int64, index int, models ...string) int64 {
	t.Helper()
	ctx := context.Background()

	pageID, err := s.InsertPageID(ctx, Page{
		BookID: bookID, PageIndex: index, StorageKey: "page", OriginalExt: "jpg",
		Width: 100, Height: 200,
	})
	if err != nil {
		t.Fatalf("InsertPageID: %v", err)
	}

	for _, model := range models {
		err := s.InsertPageOCR(ctx, PageOCR{
			PageID: pageID, Model: model, Text: model + " text",
			Data: []byte(`{"lines": []}`),
		}, nil)
		if err != nil {
			t.Fatalf("InsertPageOCR %s: %v", model, err)
		}
	}

	return pageID
}

func TestPageIndexesWithModel(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	none := create(t, s, "Read with one engine")
	readPage(t, s, none.ID, 1, "v6")
	readPage(t, s, none.ID, 2, "v6")

	all := create(t, s, "Read with two engines")
	readPage(t, s, all.ID, 1, "v6", "vl", "merged")
	readPage(t, s, all.ID, 2, "v6", "vl", "merged")

	some := create(t, s, "Rescanned in part")
	readPage(t, s, some.ID, 1, "v6")
	readPage(t, s, some.ID, 2, "v6", "vl", "merged")
	readPage(t, s, some.ID, 3, "v6")

	for _, tc := range []struct {
		name string
		book int64
		want map[int]bool
	}{
		{"no page holds the visual result", none.ID, map[int]bool{}},
		{"every page holds it", all.ID, map[int]bool{1: true, 2: true}},
		{"one page of three holds it", some.ID, map[int]bool{2: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			held, err := s.PageIndexesWithModel(ctx, tc.book, "vl")
			if err != nil {
				t.Fatalf("PageIndexesWithModel: %v", err)
			}
			if len(held) != len(tc.want) {
				t.Fatalf("the book answered %v, want %v", held, tc.want)
			}
			for index := range tc.want {
				if !held[index] {
					t.Errorf("page %d is absent from %v", index, held)
				}
			}
		})
	}
}

func TestProcessingKeepingCountLeavesThePageCount(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	book := create(t, s, "Read")
	if _, err := s.SetComplete(ctx, book.ID, 200); err != nil {
		t.Fatalf("SetComplete: %v", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)

	running, err := s.SetProcessingKeepingCountTx(ctx, tx, book.ID, PhaseUploading)
	if err != nil {
		t.Fatalf("SetProcessingKeepingCountTx: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	if running.State != StateProcessing {
		t.Errorf("the book is %q, want %q", running.State, StateProcessing)
	}
	if running.PageCount != 200 {
		t.Errorf("the running book reports %d pages, want 200", running.PageCount)
	}
	if phaseOf(running) != PhaseUploading {
		t.Errorf("the running book reports the phase %q, want %q",
			phaseOf(running), PhaseUploading)
	}

	done, err := s.SetCompleteKeepingCount(ctx, book.ID)
	if err != nil {
		t.Fatalf("SetCompleteKeepingCount: %v", err)
	}
	if done.State != StateComplete {
		t.Errorf("the book is %q, want %q", done.State, StateComplete)
	}
	if done.PageCount != 200 {
		t.Errorf("the complete book reports %d pages, want 200", done.PageCount)
	}
	if phaseOf(done) != "" {
		t.Errorf("the complete book reports the phase %q, want none", phaseOf(done))
	}
	if done.Error != nil {
		t.Errorf("the complete book reports the error %q, want none", *done.Error)
	}
}

func TestProcessingKeepingCountRollsBackWithItsTransaction(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	book := create(t, s, "Read")
	if _, err := s.SetComplete(ctx, book.ID, 12); err != nil {
		t.Fatalf("SetComplete: %v", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if _, err := s.SetProcessingKeepingCountTx(ctx, tx, book.ID, PhaseUploading); err != nil {
		t.Fatalf("SetProcessingKeepingCountTx: %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	after, err := s.Get(ctx, book.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if after.State != StateComplete {
		t.Errorf("the book is %q after a rolled back transaction, want %q",
			after.State, StateComplete)
	}
	if after.PageCount != 12 {
		t.Errorf("the book reports %d pages, want 12", after.PageCount)
	}
}
