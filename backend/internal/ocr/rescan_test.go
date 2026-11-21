package ocr

import (
	"context"
	"sort"
	"testing"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/themylogin/ownocr/backend/internal/books"
	"github.com/themylogin/ownocr/backend/internal/objectstore"
)

// rescanHarness is the OCR harness with a rescan worker beside its OCR worker,
// so a test reads a book with the normal profile and then rescans pages of it.
type rescanHarness struct {
	*harness
	rescan *RescanWorker
}

func newRescanHarness(t *testing.T) *rescanHarness {
	t.Helper()

	h := newHarness(t)
	h.profile = ProfileNormal

	return &rescanHarness{
		harness: h,
		rescan: NewRescanWorker(h.store, h.files, h.hub, h.objects, Settings{
			Settings: objectstore.Settings{
				Bucket:          "ownocr",
				Region:          "us-east-1",
				AccessKeyID:     "key",
				SecretAccessKey: "secret",
			},
			APIURL:   h.endpoint.server.URL,
			APIToken: "token",
		}),
	}
}

// readBook seeds a book, reads it with the profile the harness holds, and
// clears the bucket, which is the state a rescan starts from.
func (h *rescanHarness) readBook(t *testing.T, pageCount int) books.Book {
	t.Helper()

	_, book := h.seed(t, pageCount)
	if err := h.run(book.ID, 1, 3); err != nil {
		t.Fatalf("the OCR job failed: %v", err)
	}

	loaded, err := h.store.Get(context.Background(), book.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	return loaded
}

// readBookWithBothEngines reads a book with the extra profile, so every page
// holds the three results a normal rescan cuts down to one.
func (h *rescanHarness) readBookWithBothEngines(t *testing.T, pageCount int) books.Book {
	t.Helper()

	h.profile = ProfileExtra
	return h.readBook(t, pageCount)
}

func (h *rescanHarness) runRescan(bookID int64, indexes []int, profile string,
	attempt, maxAttempts int) error {
	job := &river.Job[RescanArgs]{
		JobRow: &rivertype.JobRow{Attempt: attempt, MaxAttempts: maxAttempts},
		Args:   RescanArgs{BookID: bookID, PageIndexes: indexes, Profile: profile},
	}
	return h.rescan.Work(context.Background(), job)
}

// modelsOf answers the model of every result a page of a book holds, sorted.
func modelsOf(t *testing.T, h *rescanHarness, book books.Book, index int) []string {
	t.Helper()

	held := h.results(t, book)[index]
	models := make([]string, 0, len(held))
	for model := range held {
		models = append(models, model)
	}
	sort.Strings(models)
	return models
}

func (h *rescanHarness) reload(t *testing.T, bookID int64) books.Book {
	t.Helper()

	book, err := h.store.Get(context.Background(), bookID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	return book
}

func TestRescanUploadsOnlyTheNamedPages(t *testing.T) {
	h := newRescanHarness(t)
	book := h.readBook(t, 4)
	h.objects.puts = nil

	if err := h.runRescan(book.ID, []int{2, 3}, ProfileExtra, 1, 3); err != nil {
		t.Fatalf("the rescan failed: %v", err)
	}

	want := []string{
		InputKey(book.StorageKey, 2, "jpg"),
		InputKey(book.StorageKey, 3, "jpg"),
	}
	if len(h.objects.puts) != len(want) {
		t.Fatalf("the rescan uploaded %v, want %v", h.objects.puts, want)
	}
	for _, key := range want {
		found := false
		for _, put := range h.objects.puts {
			if put == key {
				found = true
			}
		}
		if !found {
			t.Errorf("%s is absent from %v", key, h.objects.puts)
		}
	}
}

// A rescan asks for the one engine of its profile. The extra profile leaves the
// box engine out: its result for the page is already in the store.
func TestRescanAsksForTheEngineOfItsProfile(t *testing.T) {
	for profile, want := range map[string]string{ProfileExtra: ModeVL, ProfileNormal: ModeV6} {
		t.Run(profile, func(t *testing.T) {
			h := newRescanHarness(t)
			book := h.readBook(t, 2)

			if err := h.runRescan(book.ID, []int{1}, profile, 1, 3); err != nil {
				t.Fatalf("the rescan failed: %v", err)
			}

			if len(h.endpoint.modes) != 1 || h.endpoint.modes[0] != want {
				t.Errorf("the endpoint job named %v, want [%s]", h.endpoint.modes, want)
			}
		})
	}
}

// A job carrying a profile the system does not know reads nothing. The book
// returns to complete and every page holds the results it held.
func TestRescanOfAnUnknownProfileChangesNothing(t *testing.T) {
	h := newRescanHarness(t)
	book := h.readBook(t, 2)
	before := h.results(t, book)
	h.endpoint.modes = nil

	if err := h.runRescan(book.ID, []int{1}, "nonsense", 1, 1); err == nil {
		t.Fatal("the rescan answered no error, want the unknown profile named")
	}

	if len(h.endpoint.modes) != 0 {
		t.Errorf("the endpoint job named %v, want no job at all", h.endpoint.modes)
	}

	after := h.reload(t, book.ID)
	if after.State != books.StateComplete {
		t.Errorf("the book is %q, want %q", after.State, books.StateComplete)
	}

	results := h.results(t, book)
	for index := range before {
		if len(results[index]) != len(before[index]) {
			t.Errorf("page %d holds %d results, want the %d it held before",
				index, len(results[index]), len(before[index]))
		}
	}
}

func TestRescanAddsTheVisualAndMergedResults(t *testing.T) {
	h := newRescanHarness(t)
	book := h.readBook(t, 3)

	before := h.results(t, book)
	firstV6 := before[1][ModeV6]

	if err := h.runRescan(book.ID, []int{1}, ProfileExtra, 1, 3); err != nil {
		t.Fatalf("the rescan failed: %v", err)
	}

	after := h.results(t, book)

	for _, model := range []string{ModeV6, ModeVL, ModelMerged} {
		if _, held := after[1][model]; !held {
			t.Errorf("page 1 holds no %s result", model)
		}
	}

	if got := after[1][ModeV6]; got.Text != firstV6.Text || string(got.Data) != string(firstV6.Data) {
		t.Error("the rescan changed the box result of the page it read")
	}

	if len(after[1][ModelMerged].Data) == 0 {
		t.Error("the merged result of page 1 holds no data")
	}

	for _, index := range []int{2, 3} {
		if _, held := after[index][ModeVL]; held {
			t.Errorf("page %d gained a visual result, and the rescan did not name it", index)
		}
		if _, held := after[index][ModelMerged]; held {
			t.Errorf("page %d gained a merged result, and the rescan did not name it", index)
		}
	}
}

// An extra rescan of a page the visual engine has read replaces that reading
// and builds the merged result again from the stored box result.
func TestRescanReplacesAnEarlierVisualReading(t *testing.T) {
	h := newRescanHarness(t)
	book := h.readBookWithBothEngines(t, 2)

	pages := h.results(t, book)
	if _, held := pages[1][ModeVL]; !held {
		t.Fatal("the book was read without the visual engine")
	}

	marked := books.PageOCR{
		PageID: pages[1][ModeVL].PageID, Model: ModeVL, Text: "an earlier reading",
		Data: []byte(`{"parsing_res_list": []}`),
	}
	if err := h.store.InsertPageOCR(context.Background(), marked, nil); err != nil {
		t.Fatalf("InsertPageOCR: %v", err)
	}

	if err := h.runRescan(book.ID, []int{1}, ProfileExtra, 1, 3); err != nil {
		t.Fatalf("the rescan failed: %v", err)
	}

	after := h.results(t, book)
	if after[1][ModeVL].Text == marked.Text {
		t.Error("the page holds the earlier visual reading, want the one this rescan read")
	}
	if models := modelsOf(t, h, book, 1); len(models) != 3 {
		t.Errorf("the page holds %v, want the three results of a page both engines read", models)
	}
	if len(after[1][ModelMerged].Data) == 0 {
		t.Error("the merged result of page 1 holds no data")
	}
}

// A normal rescan answers for the page with the line engine alone. The visual
// reading and the merged result derive from a reading it replaced, so they go.
func TestNormalRescanReplacesTheReadingOfThePage(t *testing.T) {
	h := newRescanHarness(t)
	book := h.readBookWithBothEngines(t, 3)

	if models := modelsOf(t, h, book, 1); len(models) != 3 {
		t.Fatalf("page 1 holds %v, want the three results of a page both engines read", models)
	}

	marked := books.PageOCR{
		PageID: h.results(t, book)[1][ModeV6].PageID, Model: ModeV6,
		Text: "an earlier reading", Data: []byte(`{"raw": {}, "lines": []}`),
	}
	if err := h.store.InsertPageOCR(context.Background(), marked, nil); err != nil {
		t.Fatalf("InsertPageOCR: %v", err)
	}

	if err := h.runRescan(book.ID, []int{1}, ProfileNormal, 1, 3); err != nil {
		t.Fatalf("the rescan failed: %v", err)
	}

	models := modelsOf(t, h, book, 1)
	if len(models) != 1 || models[0] != ModeV6 {
		t.Errorf("page 1 holds %v, want the v6 result alone", models)
	}

	after := h.results(t, book)
	if after[1][ModeV6].Text == marked.Text {
		t.Error("page 1 holds the earlier line reading, want the one this rescan read")
	}

	// A page the rescan did not name keeps every result it held.
	if models := modelsOf(t, h, book, 2); len(models) != 3 {
		t.Errorf("page 2 holds %v, and the rescan did not name it", models)
	}
}

// A page that holds the line reading alone gains the new one and loses nothing.
func TestNormalRescanOfAPageOneEngineRead(t *testing.T) {
	h := newRescanHarness(t)
	book := h.readBook(t, 2)

	if err := h.runRescan(book.ID, []int{1}, ProfileNormal, 1, 3); err != nil {
		t.Fatalf("the rescan failed: %v", err)
	}

	models := modelsOf(t, h, book, 1)
	if len(models) != 1 || models[0] != ModeV6 {
		t.Errorf("page 1 holds %v, want the v6 result alone", models)
	}
	if len(h.results(t, book)[1][ModeV6].Data) == 0 {
		t.Error("the v6 result of page 1 holds no data")
	}
}

// A page the endpoint could not read keeps every result it held. One page the
// endpoint refused must not cost a page it read.
func TestNormalRescanKeepsThePagesTheEndpointRefused(t *testing.T) {
	h := newRescanHarness(t)
	book := h.readBookWithBothEngines(t, 3)

	h.endpoint.failPage = "0002"

	if err := h.runRescan(book.ID, []int{1, 2}, ProfileNormal, 1, 3); err != nil {
		t.Fatalf("the rescan failed: %v", err)
	}

	if models := modelsOf(t, h, book, 1); len(models) != 1 || models[0] != ModeV6 {
		t.Errorf("page 1 holds %v, want the v6 result alone", models)
	}
	if models := modelsOf(t, h, book, 2); len(models) != 3 {
		t.Errorf("page 2 holds %v, want the three results it held before", models)
	}

	if state := h.reload(t, book.ID).State; state != books.StateComplete {
		t.Errorf("the book is %q after a part rescan, want %q", state, books.StateComplete)
	}
}

func TestRescanKeepsTheBookWholeAndComplete(t *testing.T) {
	h := newRescanHarness(t)
	book := h.readBook(t, 5)

	if err := h.runRescan(book.ID, []int{2}, ProfileExtra, 1, 3); err != nil {
		t.Fatalf("the rescan failed: %v", err)
	}

	after := h.reload(t, book.ID)
	if after.State != books.StateComplete {
		t.Errorf("the book is %q, want %q", after.State, books.StateComplete)
	}
	if after.PageCount != 5 {
		t.Errorf("the book reports %d pages, want 5", after.PageCount)
	}
	if after.Progress != nil {
		t.Errorf("the book reports the phase %q, want none", *after.Progress)
	}
	if after.OCRProfile == nil || *after.OCRProfile != ProfileNormal {
		t.Errorf("the book records the profile %v, want %q", after.OCRProfile, ProfileNormal)
	}
}

func TestRescanStoresThePagesTheEndpointRead(t *testing.T) {
	h := newRescanHarness(t)
	book := h.readBook(t, 3)

	// The fake endpoint names a page it refuses by the name of its object.
	h.endpoint.failPage = "0002"

	if err := h.runRescan(book.ID, []int{1, 2, 3}, ProfileExtra, 1, 3); err != nil {
		t.Fatalf("the rescan failed: %v", err)
	}

	after := h.results(t, book)
	for _, index := range []int{1, 3} {
		if _, held := after[index][ModelMerged]; !held {
			t.Errorf("page %d holds no merged result, and the endpoint read it", index)
		}
	}
	if _, held := after[2][ModeVL]; held {
		t.Error("page 2 gained a visual result, and the endpoint refused it")
	}
	if _, held := after[2][ModeV6]; !held {
		t.Error("page 2 lost the box result it held before the rescan")
	}

	if state := h.reload(t, book.ID).State; state != books.StateComplete {
		t.Errorf("the book is %q after a part rescan, want %q", state, books.StateComplete)
	}
}

func TestRescanAbandonsAWholeFailureWithoutFailingTheBook(t *testing.T) {
	h := newRescanHarness(t)
	book := h.readBook(t, 3)

	before := h.results(t, book)
	h.endpoint.status = "FAILED"

	if err := h.runRescan(book.ID, []int{1, 2}, ProfileExtra, 3, 3); err == nil {
		t.Fatal("the rescan answered no error, want the cause of the failure")
	}

	after := h.reload(t, book.ID)
	if after.State != books.StateComplete {
		t.Errorf("the book is %q after an abandoned rescan, want %q",
			after.State, books.StateComplete)
	}
	if after.Error != nil {
		t.Errorf("the book carries the reason %q, want none", *after.Error)
	}
	if after.Progress != nil {
		t.Errorf("the book carries the phase %q, want none", *after.Progress)
	}
	if after.PageCount != 3 {
		t.Errorf("the book reports %d pages, want 3", after.PageCount)
	}

	results := h.results(t, book)
	for index := range before {
		if len(results[index]) != len(before[index]) {
			t.Errorf("page %d holds %d results, want the %d it held before",
				index, len(results[index]), len(before[index]))
		}
	}
}

func TestRescanKeepsTheBookRunningWhileAttemptsRemain(t *testing.T) {
	h := newRescanHarness(t)
	book := h.readBook(t, 2)

	if _, err := h.store.SetProgress(context.Background(), book.ID,
		books.PhaseQueued); err != nil {
		t.Fatalf("SetProgress: %v", err)
	}
	if _, err := h.store.Pool().Exec(context.Background(),
		"UPDATE book SET state = $2 WHERE id = $1", book.ID, books.StateProcessing); err != nil {
		t.Fatalf("set processing: %v", err)
	}

	h.endpoint.status = "FAILED"

	if err := h.runRescan(book.ID, []int{1}, ProfileExtra, 1, 3); err == nil {
		t.Fatal("the rescan answered no error, want the cause of the failure")
	}

	after := h.reload(t, book.ID)
	if after.State != books.StateProcessing {
		t.Errorf("the book is %q while attempts remain, want %q",
			after.State, books.StateProcessing)
	}
	if after.State == books.StateFailed {
		t.Error("the rescan failed the book, which a retry would then rebuild")
	}
}

func TestRescanOfARejectedBookDoesNothing(t *testing.T) {
	h := newRescanHarness(t)

	if err := h.runRescan(999999, []int{1}, ProfileExtra, 1, 3); err != nil {
		t.Fatalf("the rescan of a missing book answered %v, want no error", err)
	}
}

func TestRescanRemovesItsExchangeObjects(t *testing.T) {
	h := newRescanHarness(t)
	book := h.readBook(t, 2)

	if err := h.runRescan(book.ID, []int{1}, ProfileExtra, 1, 3); err != nil {
		t.Fatalf("the rescan failed: %v", err)
	}

	if left := h.objects.keys(Prefix(book.StorageKey)); len(left) != 0 {
		t.Errorf("the rescan left %v in the bucket, want nothing", left)
	}
}
