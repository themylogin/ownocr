package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/riverqueue/river"

	"github.com/themylogin/ownocr/backend/internal/books"
	"github.com/themylogin/ownocr/backend/internal/dewarp"
	"github.com/themylogin/ownocr/backend/internal/filestore"
	"github.com/themylogin/ownocr/backend/internal/ocr"
	"github.com/themylogin/ownocr/backend/internal/queue"
	"github.com/themylogin/ownocr/backend/internal/split"
	"github.com/themylogin/ownocr/backend/internal/testdb"
	"github.com/themylogin/ownocr/backend/internal/ws"
)

type actionHarness struct {
	router http.Handler
	store  *books.Store
}

type idleWorker struct {
	river.WorkerDefaults[ocr.Args]
}

func (*idleWorker) Work(context.Context, *river.Job[ocr.Args]) error { return nil }

type idleDewarpWorker struct {
	river.WorkerDefaults[dewarp.Args]
}

func (*idleDewarpWorker) Work(context.Context, *river.Job[dewarp.Args]) error { return nil }

type idleSplitWorker struct {
	river.WorkerDefaults[split.Args]
}

func (*idleSplitWorker) Work(context.Context, *river.Job[split.Args]) error { return nil }

func newActionHarness(t *testing.T) *actionHarness {
	t.Helper()

	pool := testdb.New(t)

	files, err := filestore.New(t.TempDir())
	if err != nil {
		t.Fatalf("filestore: %v", err)
	}

	// River refuses to insert a job whose kind no worker claims, so the
	// registry holds one. Nothing runs the jobs off: the client never starts,
	// which is what lets a test read the queue a request wrote.
	workers := river.NewWorkers()
	river.AddWorker(workers, &idleWorker{})
	river.AddWorker(workers, &idleDewarpWorker{})
	river.AddWorker(workers, &idleSplitWorker{})

	client, err := queue.New(pool, workers, 1)
	if err != nil {
		t.Fatalf("queue: %v", err)
	}

	store := books.NewStore(pool)

	return &actionHarness{
		router: NewRouter(Deps{Pool: pool, Hub: ws.NewHub(nil), Files: files,
			Books: store, Queue: client}),
		store: store,
	}
}

// post sends an action. The OCR action takes a profile, so it carries one
// unless a test names its own with postWith.
func (h *actionHarness) post(t *testing.T, action string, bookIDs []int64) (int, actionResult) {
	t.Helper()

	profile := ""
	if action == books.ActionOCR {
		profile = ocr.ProfileExtra
	}
	return h.postWith(t, action, bookIDs, profile)
}

func (h *actionHarness) postWith(t *testing.T, action string, bookIDs []int64,
	ocrProfile string) (int, actionResult) {
	t.Helper()

	body, err := json.Marshal(actionRequest{BookIDs: bookIDs, OCRProfile: ocrProfile})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/books/actions/"+action,
		bytes.NewReader(body))
	recorder := httptest.NewRecorder()
	h.router.ServeHTTP(recorder, request)

	var result actionResult
	if recorder.Code == http.StatusOK {
		if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
			t.Fatalf("decode answer: %v", err)
		}
	}
	return recorder.Code, result
}

// completeBook makes a source book that is ready for an action.
func (h *actionHarness) completeBook(t *testing.T, stage string) books.Book {
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
	book, err := h.store.CreateTx(ctx, tx, "Atlas", key)
	if err != nil {
		t.Fatalf("CreateTx: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	if _, err := h.store.Pool().Exec(ctx, `UPDATE book SET stage = $2 WHERE id = $1`,
		book.ID, stage); err != nil {
		t.Fatalf("set stage: %v", err)
	}

	book, err = h.store.SetComplete(ctx, book.ID, 0)
	if err != nil {
		t.Fatalf("SetComplete: %v", err)
	}
	return book
}

// queuedKinds answers the kind and the queue of every job that names a book.
// The river tables outlive one test, so a job is found by the book it carries.
func (h *actionHarness) queuedKinds(t *testing.T, bookID int64) []string {
	t.Helper()

	rows, err := h.store.Pool().Query(context.Background(),
		`SELECT kind || ' on ' || queue FROM river_job
		WHERE (args->>'bookId')::bigint = $1 ORDER BY id`, bookID)
	if err != nil {
		t.Fatalf("query jobs: %v", err)
	}
	defer rows.Close()

	var kinds []string
	for rows.Next() {
		var kind string
		if err := rows.Scan(&kind); err != nil {
			t.Fatalf("scan: %v", err)
		}
		kinds = append(kinds, kind)
	}
	return kinds
}

func TestOCRActionCreatesADoneCopyAndOneJob(t *testing.T) {
	h := newActionHarness(t)
	source := h.completeBook(t, books.StageSplit)

	code, result := h.post(t, books.ActionOCR, []int64{source.ID})
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if len(result.Started) != 1 || len(result.Failed) != 0 {
		t.Fatalf("answer = %+v", result)
	}

	copyBook, err := h.store.Get(context.Background(), result.Started[0].ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if copyBook.Stage != books.StageOCR {
		t.Errorf("Stage = %q, want %q", copyBook.Stage, books.StageOCR)
	}
	if copyBook.State != books.StatePending {
		t.Errorf("State = %q, want pending", copyBook.State)
	}
	if copyBook.Action == nil || *copyBook.Action != books.ActionOCR {
		t.Errorf("the copy records the action %v", copyBook.Action)
	}
	if copyBook.SourceBookID == nil || *copyBook.SourceBookID != source.ID {
		t.Errorf("the copy references %v, want %d", copyBook.SourceBookID, source.ID)
	}

	kinds := h.queuedKinds(t, copyBook.ID)
	if len(kinds) != 1 || kinds[0] != "ocr on ocr" {
		t.Errorf("queued %v, want one ocr job on the ocr queue", kinds)
	}
}

func TestOCRActionIsRefusedInTheOCRStage(t *testing.T) {
	h := newActionHarness(t)
	source := h.completeBook(t, books.StageOCR)

	code, result := h.post(t, books.ActionOCR, []int64{source.ID})
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if len(result.Started) != 0 || len(result.Failed) != 1 {
		t.Fatalf("answer = %+v", result)
	}
	if kinds := h.queuedKinds(t, source.ID); len(kinds) != 0 {
		t.Errorf("a refused action queued %v", kinds)
	}
}

// The OCR action takes a profile. A request that omits it starts nothing, so
// the refusal lands before any book is touched.
func TestOCRActionNeedsAProfile(t *testing.T) {
	h := newActionHarness(t)
	source := h.completeBook(t, books.StageSplit)

	code, _ := h.postWith(t, books.ActionOCR, []int64{source.ID}, "")
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", code)
	}
	var copies int
	if err := h.store.Pool().QueryRow(context.Background(),
		`SELECT count(*) FROM book WHERE source_book_id = $1`, source.ID).Scan(&copies); err != nil {
		t.Fatalf("count copies: %v", err)
	}
	if copies != 0 {
		t.Errorf("a request with no profile made %d copies", copies)
	}
}

func TestOCRActionRefusesAnUnknownProfile(t *testing.T) {
	h := newActionHarness(t)
	source := h.completeBook(t, books.StageSplit)

	code, _ := h.postWith(t, books.ActionOCR, []int64{source.ID}, "nonsense")
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", code)
	}
}

// Every other action takes no argument and starts without one.
func TestOtherActionsNeedNoProfile(t *testing.T) {
	h := newActionHarness(t)
	source := h.completeBook(t, books.StageUploaded)

	code, result := h.postWith(t, books.ActionDewarp, []int64{source.ID}, "")
	if code != http.StatusOK || len(result.Started) != 1 {
		t.Fatalf("status = %d, answer = %+v", code, result)
	}

	copyBook, err := h.store.Get(context.Background(), result.Started[0].ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if copyBook.OCRProfile != nil {
		t.Errorf("a dewarp copy records the profile %q", *copyBook.OCRProfile)
	}
}

// The copy records the profile the request named, and reports it.
func TestOCRCopyRecordsItsProfile(t *testing.T) {
	h := newActionHarness(t)

	for _, profile := range []string{ocr.ProfileNormal, ocr.ProfileExtra} {
		source := h.completeBook(t, books.StageSplit)

		code, result := h.postWith(t, books.ActionOCR, []int64{source.ID}, profile)
		if code != http.StatusOK || len(result.Started) != 1 {
			t.Fatalf("%s: status = %d, answer = %+v", profile, code, result)
		}

		if result.Started[0].OCRProfile == nil || *result.Started[0].OCRProfile != profile {
			t.Errorf("%s: the answer reports %v", profile, result.Started[0].OCRProfile)
		}

		copyBook, err := h.store.Get(context.Background(), result.Started[0].ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if copyBook.OCRProfile == nil || *copyBook.OCRProfile != profile {
			t.Errorf("%s: the copy records %v", profile, copyBook.OCRProfile)
		}
	}
}

// A retry runs what the copy records, never what the request that starts it
// names. The worker reads the profile from the copy, so the two cannot drift.
func TestRetryKeepsTheProfileOfTheCopy(t *testing.T) {
	h := newActionHarness(t)
	ctx := context.Background()
	source := h.completeBook(t, books.StageSplit)

	_, result := h.postWith(t, books.ActionOCR, []int64{source.ID}, ocr.ProfileExtra)
	if len(result.Started) != 1 {
		t.Fatalf("answer = %+v", result)
	}
	copyID := result.Started[0].ID

	if _, err := h.store.SetFailed(ctx, copyID, "the pod died"); err != nil {
		t.Fatalf("SetFailed: %v", err)
	}

	// The retry names the other profile. The copy must ignore it.
	code, retried := h.postWith(t, books.ActionOCR, []int64{source.ID}, ocr.ProfileNormal)
	if code != http.StatusOK || len(retried.Started) != 1 {
		t.Fatalf("status = %d, answer = %+v", code, retried)
	}
	if retried.Started[0].ID != copyID {
		t.Fatalf("the retry made a third book %d", retried.Started[0].ID)
	}

	copyBook, err := h.store.Get(ctx, copyID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if copyBook.OCRProfile == nil || *copyBook.OCRProfile != ocr.ProfileExtra {
		t.Errorf("the retried copy records %v, want %q",
			copyBook.OCRProfile, ocr.ProfileExtra)
	}
	if copyBook.State != books.StatePending {
		t.Errorf("State = %q, want pending", copyBook.State)
	}
}
