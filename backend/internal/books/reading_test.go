package books

import (
	"context"
	"errors"
	"testing"
)

// ranked is the order the production caller passes: best reading first. The
// store holds no such order of its own.
var ranked = []string{"glm_merged", "merged", "v6"}

func readingOf(t *testing.T, s *Store, pageID int64, models []string) string {
	t.Helper()

	result, held, err := s.PageReading(context.Background(), pageID, models)
	if err != nil || !held {
		t.Fatalf("PageReading: %v, held %v", err, held)
	}
	return result.Model
}

func resultID(t *testing.T, s *Store, pageID int64, model string) int64 {
	t.Helper()

	results, err := s.PageOCRResults(context.Background(), pageID)
	if err != nil {
		t.Fatalf("PageOCRResults: %v", err)
	}
	for _, result := range results {
		if result.Model == model {
			return result.ID
		}
	}
	t.Fatalf("the page holds no %s result", model)
	return 0
}

func namedReading(t *testing.T, s *Store, pageID int64) (int64, bool) {
	t.Helper()

	var id *int64
	err := s.pool.QueryRow(context.Background(),
		`SELECT ocr_id FROM book_page WHERE id = $1`, pageID).Scan(&id)
	if err != nil {
		t.Fatalf("read ocr_id: %v", err)
	}
	if id == nil {
		return 0, false
	}
	return *id, true
}

func TestPageAnsweringNoReadingFollowsTheRank(t *testing.T) {
	s := newTestStore(t)
	book := create(t, s, "Read")
	pageID := seedReadPage(t, s, book.ID, "v6", "merged")

	if _, named := namedReading(t, s, pageID); named {
		t.Fatal("a page seeded with no rank names a reading")
	}
	if model := readingOf(t, s, pageID, ranked); model != "merged" {
		t.Errorf("the page answered %q, want the merged reading by rank", model)
	}
}

func TestPageAnswersTheReadingItNames(t *testing.T) {
	s := newTestStore(t)
	book := create(t, s, "Read")
	pageID := seedReadPage(t, s, book.ID, "v6", "merged")

	if err := s.SetPageReading(context.Background(), pageID, resultID(t, s, pageID, "v6")); err != nil {
		t.Fatalf("SetPageReading: %v", err)
	}

	if model := readingOf(t, s, pageID, ranked); model != "v6" {
		t.Errorf("the page answered %q, want the line reading it names", model)
	}
}

// A page names a reading whatever the preference list holds. The list is the
// order a page falls back to, not a filter on a choice already made.
func TestPageAnswersANamedReadingOutsideTheList(t *testing.T) {
	s := newTestStore(t)
	book := create(t, s, "Read")
	pageID := seedReadPage(t, s, book.ID, "v6", "merged")

	if err := s.SetPageReading(context.Background(), pageID, resultID(t, s, pageID, "v6")); err != nil {
		t.Fatalf("SetPageReading: %v", err)
	}

	if model := readingOf(t, s, pageID, []string{"merged"}); model != "v6" {
		t.Errorf("the page answered %q, want the reading it names", model)
	}
}

func TestBookOCRInAnswersTheReadingEachPageNames(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	book := create(t, s, "Read")

	named := seedReadPage(t, s, book.ID, "v6", "merged")
	if err := s.SetPageReading(ctx, named, resultID(t, s, named, "v6")); err != nil {
		t.Fatalf("SetPageReading: %v", err)
	}

	free, err := s.InsertPageID(ctx, Page{
		BookID: book.ID, PageIndex: 2, StorageKey: "two", OriginalExt: "jpg",
		Width: 100, Height: 200,
	})
	if err != nil {
		t.Fatalf("InsertPageID: %v", err)
	}
	for _, model := range []string{"v6", "merged"} {
		err := s.InsertPageOCR(ctx, PageOCR{
			PageID: free, Model: model, Text: model + " text", Data: []byte(`{"lines": []}`),
		}, nil)
		if err != nil {
			t.Fatalf("InsertPageOCR %s: %v", model, err)
		}
	}

	byPage, err := s.BookReadings(ctx, book.ID, ranked)
	if err != nil {
		t.Fatalf("BookReadings: %v", err)
	}

	if byPage[named].Model != "v6" {
		t.Errorf("the named page answered %q, want v6", byPage[named].Model)
	}
	if byPage[free].Model != "merged" {
		t.Errorf("the page naming none answered %q, want merged", byPage[free].Model)
	}

	// The whole book answer and the one page answer may never disagree.
	for _, pageID := range []int64{named, free} {
		if alone := readingOf(t, s, pageID, ranked); alone != byPage[pageID].Model {
			t.Errorf("page %d answers %q alone and %q in the book", pageID, alone,
				byPage[pageID].Model)
		}
	}
}

func TestAReadingARendererCannotDrawTakesOverNothing(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	book := create(t, s, "Read")
	pageID := seedReadPage(t, s, book.ID, "v6")

	err := s.InsertPageOCR(ctx, PageOCR{
		PageID: pageID, Model: "vl", Text: "vl text", Data: []byte(`{"lines": []}`),
	}, ranked)
	if err != nil {
		t.Fatalf("InsertPageOCR vl: %v", err)
	}

	if model := readingOf(t, s, pageID, ranked); model != "v6" {
		t.Errorf("the page answered %q, want the line reading", model)
	}
}

// A better reading clears the choice a reader made, so the page answers the best
// reading it holds as though nothing had been chosen.
func TestABetterReadingClearsTheChoiceOfAReader(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	book := create(t, s, "Read")
	pageID := seedReadPage(t, s, book.ID, "v6", "merged")

	if err := s.SetPageReading(ctx, pageID, resultID(t, s, pageID, "v6")); err != nil {
		t.Fatalf("SetPageReading: %v", err)
	}
	if model := readingOf(t, s, pageID, ranked); model != "v6" {
		t.Fatalf("the page answered %q before the better reading arrived", model)
	}

	err := s.InsertPageOCR(ctx, PageOCR{
		PageID: pageID, Model: "glm_merged", Text: "ultra text",
		Data: []byte(`{"lines": []}`),
	}, ranked)
	if err != nil {
		t.Fatalf("InsertPageOCR glm_merged: %v", err)
	}

	if _, named := namedReading(t, s, pageID); named {
		t.Error("the page still names a reading after a better one arrived")
	}
	if model := readingOf(t, s, pageID, ranked); model != "glm_merged" {
		t.Errorf("the page answered %q, want the best reading it holds", model)
	}
}

// A reading no better than the chosen one leaves that choice standing.
func TestAWorseReadingLeavesTheChoiceOfAReader(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	book := create(t, s, "Read")
	pageID := seedReadPage(t, s, book.ID, "v6", "glm_merged")

	if err := s.SetPageReading(ctx, pageID, resultID(t, s, pageID, "glm_merged")); err != nil {
		t.Fatalf("SetPageReading: %v", err)
	}

	err := s.InsertPageOCR(ctx, PageOCR{
		PageID: pageID, Model: "merged", Text: "merged text", Data: []byte(`{"lines": []}`),
	}, ranked)
	if err != nil {
		t.Fatalf("InsertPageOCR merged: %v", err)
	}

	if _, named := namedReading(t, s, pageID); !named {
		t.Error("a worse reading cleared the choice of a reader")
	}
	if model := readingOf(t, s, pageID, ranked); model != "glm_merged" {
		t.Errorf("the page answered %q, want the reading the reader chose", model)
	}
}

// The same reading stored again is no better than itself, so it clears nothing.
func TestAReadingStoredAgainLeavesTheChoiceOfAReader(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	book := create(t, s, "Read")
	pageID := seedReadPage(t, s, book.ID, "v6", "merged")

	if err := s.SetPageReading(ctx, pageID, resultID(t, s, pageID, "v6")); err != nil {
		t.Fatalf("SetPageReading: %v", err)
	}

	err := s.InsertPageOCR(ctx, PageOCR{
		PageID: pageID, Model: "v6", Text: "read again", Data: []byte(`{"lines": []}`),
	}, ranked)
	if err != nil {
		t.Fatalf("InsertPageOCR v6: %v", err)
	}

	if model := readingOf(t, s, pageID, ranked); model != "v6" {
		t.Errorf("the page answered %q after its own reading was stored again", model)
	}
}

// A page that names nothing needs no write: it answers by rank already, so the
// reading it gains becomes its answer on its own.
func TestAPageNamingNothingNeedsNoWrite(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	book := create(t, s, "Read")
	pageID := seedReadPage(t, s, book.ID, "v6")

	err := s.InsertPageOCR(ctx, PageOCR{
		PageID: pageID, Model: "merged", Text: "merged text", Data: []byte(`{"lines": []}`),
	}, ranked)
	if err != nil {
		t.Fatalf("InsertPageOCR merged: %v", err)
	}

	if _, named := namedReading(t, s, pageID); named {
		t.Error("the page names a reading no reader chose")
	}
	if model := readingOf(t, s, pageID, ranked); model != "merged" {
		t.Errorf("the page answered %q, want the best reading it holds", model)
	}
}

func TestRemovingTheNamedReadingLeavesThePageNamingNone(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	book := create(t, s, "Read")
	pageID := seedReadPage(t, s, book.ID, "v6", "vl", "merged")

	if err := s.SetPageReading(ctx, pageID, resultID(t, s, pageID, "merged")); err != nil {
		t.Fatalf("SetPageReading: %v", err)
	}

	// A normal rescan replaces the line reading and removes what derived from it.
	err := s.ReplacePageOCR(ctx, PageOCR{
		PageID: pageID, Model: "v6", Text: "read again", Data: []byte(`{"lines": []}`),
	}, []string{"vl", "merged"}, ranked)
	if err != nil {
		t.Fatalf("ReplacePageOCR: %v", err)
	}

	if _, named := namedReading(t, s, pageID); named {
		t.Error("the page still names a reading it no longer holds")
	}
	if model := readingOf(t, s, pageID, ranked); model != "v6" {
		t.Errorf("the page answered %q, want the line reading by rank", model)
	}
}

// A choice cannot outlive the reading it pointed at: removing that reading leaves
// the page answering by rank, and the next reading it gains answers for it.
func TestAChoiceCannotVetoBeyondTheReadingItNamed(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	book := create(t, s, "Read")
	pageID := seedReadPage(t, s, book.ID, "v6", "vl", "merged")

	if err := s.SetPageReading(ctx, pageID, resultID(t, s, pageID, "merged")); err != nil {
		t.Fatalf("SetPageReading: %v", err)
	}

	err := s.ReplacePageOCR(ctx, PageOCR{
		PageID: pageID, Model: "v6", Text: "read again", Data: []byte(`{"lines": []}`),
	}, []string{"vl", "merged"}, ranked)
	if err != nil {
		t.Fatalf("ReplacePageOCR: %v", err)
	}

	err = s.InsertPageOCR(ctx, PageOCR{
		PageID: pageID, Model: "glm_merged", Text: "ultra text",
		Data: []byte(`{"lines": []}`),
	}, ranked)
	if err != nil {
		t.Fatalf("InsertPageOCR glm_merged: %v", err)
	}

	if model := readingOf(t, s, pageID, ranked); model != "glm_merged" {
		t.Errorf("the page answered %q, want the Ultra reading to take over", model)
	}
}

func TestSetPageReadingRefusesAResultOfAnotherPage(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	book := create(t, s, "Read")

	mine := seedReadPage(t, s, book.ID, "v6")
	theirs, err := s.InsertPageID(ctx, Page{
		BookID: book.ID, PageIndex: 2, StorageKey: "two", OriginalExt: "jpg",
		Width: 100, Height: 200,
	})
	if err != nil {
		t.Fatalf("InsertPageID: %v", err)
	}
	err = s.InsertPageOCR(ctx, PageOCR{
		PageID: theirs, Model: "v6", Text: "other", Data: []byte(`{"lines": []}`),
	}, nil)
	if err != nil {
		t.Fatalf("InsertPageOCR: %v", err)
	}

	err = s.SetPageReading(ctx, mine, resultID(t, s, theirs, "v6"))
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("SetPageReading answered %v, want ErrNotFound", err)
	}
	if _, named := namedReading(t, s, mine); named {
		t.Error("the page names a reading of another page")
	}
}

func TestPageReadingsMarksTheOneThatAnswers(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	book := create(t, s, "Read")
	pageID := seedReadPage(t, s, book.ID, "v6", "vl", "merged", "glm_merged")

	readings, err := s.PageReadings(ctx, pageID, ranked)
	if err != nil {
		t.Fatalf("PageReadings: %v", err)
	}

	if len(readings) != 3 {
		t.Fatalf("the page reports %d readings, want the 3 a renderer can draw", len(readings))
	}
	if readings[0].Model != "glm_merged" || !readings[0].Answers {
		t.Errorf("the best ranked reading is %q, answers %v", readings[0].Model,
			readings[0].Answers)
	}
	for _, reading := range readings[1:] {
		if reading.Answers {
			t.Errorf("%q answers beside the best ranked reading", reading.Model)
		}
	}

	if err := s.SetPageReading(ctx, pageID, resultID(t, s, pageID, "v6")); err != nil {
		t.Fatalf("SetPageReading: %v", err)
	}

	readings, err = s.PageReadings(ctx, pageID, ranked)
	if err != nil {
		t.Fatalf("PageReadings: %v", err)
	}
	answering := ""
	for _, reading := range readings {
		if reading.Answers {
			if answering != "" {
				t.Errorf("two readings answer: %q and %q", answering, reading.Model)
			}
			answering = reading.Model
		}
	}
	if answering != "v6" {
		t.Errorf("the mark sits on %q, want the reading the reader chose", answering)
	}
}
