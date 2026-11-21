package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/themylogin/ownocr/backend/internal/books"
	"github.com/themylogin/ownocr/backend/internal/dewarp"
	"github.com/themylogin/ownocr/backend/internal/ocr"
	"github.com/themylogin/ownocr/backend/internal/split"
)

type actionRequest struct {
	BookIDs []int64 `json:"bookIds"`

	// OCRProfile is the argument the OCR action takes. Every other action
	// ignores it.
	OCRProfile string `json:"ocrProfile"`
}

type actionFailure struct {
	BookID int64  `json:"bookId"`
	Error  string `json:"error"`
}

type actionResult struct {
	Started []bookSummary   `json:"started"`
	Failed  []actionFailure `json:"failed"`
}

const maxActionBooks = 500

// runAction applies one action to every book in the request. Each book runs in
// its own transaction, so one refusal cannot undo the books that started.
func (a *api) runAction(w http.ResponseWriter, r *http.Request) {
	action, known := books.ActionByID(chi.URLParam(r, "action"))
	if !known {
		writeError(w, http.StatusNotFound, "no such action")
		return
	}

	var request actionRequest
	if err := readJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "the request must hold a bookIds list")
		return
	}
	if len(request.BookIDs) == 0 {
		writeError(w, http.StatusUnprocessableEntity, "bookIds must name at least one book")
		return
	}
	if len(request.BookIDs) > maxActionBooks {
		writeError(w, http.StatusUnprocessableEntity, "too many books in one request")
		return
	}
	if err := checkArguments(action, request); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	result := actionResult{Started: []bookSummary{}, Failed: []actionFailure{}}

	for _, id := range request.BookIDs {
		copyBook, created, err := a.startAction(r.Context(), id, action, request.OCRProfile)
		if err != nil {
			result.Failed = append(result.Failed, actionFailure{BookID: id, Error: reasonOf(id, action, err)})
			continue
		}

		summary := a.summary(r.Context(), copyBook)
		result.Started = append(result.Started, summary)

		event := "book.updated"
		if created {
			event = "book.created"
		}
		a.deps.Hub.Publish(event, summary)

		// The source book carries the mark of its copy from now on. Every
		// client learns it from this event, without a reload.
		if source, err := a.deps.Books.Get(r.Context(), id); err == nil {
			a.deps.Hub.Publish("book.updated", a.summary(r.Context(), source))
		}
	}

	writeJSON(w, http.StatusOK, result)
}

func reasonOf(id int64, action books.Action, err error) string {
	var invalid invalidUpload
	switch {
	case errors.Is(err, books.ErrNotFound):
		return "no such book"
	case errors.As(err, &invalid):
		return invalid.message
	default:
		slog.Error("start action", "book", id, "action", action.ID, "error", err)
		return "cannot start " + action.Title
	}
}

// startAction locks the source book, so two clients cannot make two copies. The
// copy row and its job commit together or not at all.
func (a *api) startAction(ctx context.Context, sourceID int64, action books.Action,
	ocrProfile string) (books.Book, bool, error) {
	tx, err := a.deps.Pool.Begin(ctx)
	if err != nil {
		return books.Book{}, false, err
	}
	defer tx.Rollback(ctx)

	if err := a.deps.Books.LockTx(ctx, tx, sourceID); err != nil {
		return books.Book{}, false, err
	}

	source, err := a.deps.Books.GetTx(ctx, tx, sourceID)
	if err != nil {
		return books.Book{}, false, err
	}
	if source.State != books.StateComplete {
		return books.Book{}, false, reject(http.StatusUnprocessableEntity, "the book is not complete")
	}
	if !books.OffersAction(source.Stage, action.ID) {
		return books.Book{}, false,
			reject(http.StatusUnprocessableEntity, "%s does not offer %s", source.Stage, action.Title)
	}

	existing, err := a.deps.Books.DerivedTx(ctx, tx, sourceID)
	switch {
	case err == nil && existing.State != books.StateFailed:
		return books.Book{}, false, reject(http.StatusConflict, "the book already has a copy")
	case err == nil:
		retried, err := a.retry(ctx, tx, existing)
		if err != nil {
			return books.Book{}, false, err
		}
		return retried, false, tx.Commit(ctx)
	case errors.Is(err, books.ErrNotFound):
	default:
		return books.Book{}, false, err
	}

	key, err := books.NewStorageKey()
	if err != nil {
		return books.Book{}, false, err
	}

	created, err := a.deps.Books.CreateDerivedTx(ctx, tx, source.Name, key, source.ID,
		action, argumentOf(action, ocrProfile))
	if err != nil {
		return books.Book{}, false, err
	}

	if err := a.queueAction(ctx, tx, action, created.ID); err != nil {
		return books.Book{}, false, err
	}

	return created, true, tx.Commit(ctx)
}

// checkArguments refuses a request that does not carry what its action needs.
// It runs before any book is touched, so a bad request starts nothing.
func checkArguments(action books.Action, request actionRequest) error {
	if action.ID != books.ActionOCR {
		return nil
	}
	if request.OCRProfile == "" {
		return errors.New("ocrProfile must name the model to read with")
	}
	if ocr.RescanOnlyProfile(request.OCRProfile) {
		return fmt.Errorf("ocrProfile %q reads one page at a time and only on a rescan",
			request.OCRProfile)
	}
	if !ocr.KnownProfile(request.OCRProfile) {
		return fmt.Errorf("ocrProfile %q is not a model this system reads with",
			request.OCRProfile)
	}
	return nil
}

// argumentOf answers the argument the copy of this action records. Only OCR
// takes one.
func argumentOf(action books.Action, ocrProfile string) string {
	if action.ID == books.ActionOCR {
		return ocrProfile
	}
	return ""
}

// retry runs the action that made the failed copy, not the action the caller
// asked for. A copy keeps its own identity, its own stage and its own
// arguments, so every attempt runs the same work.
func (a *api) retry(ctx context.Context, tx pgx.Tx, existing books.Book) (books.Book, error) {
	if existing.Action == nil {
		return books.Book{}, reject(http.StatusUnprocessableEntity, "the copy names no action")
	}

	action, known := books.ActionByID(*existing.Action)
	if !known {
		return books.Book{}, reject(http.StatusUnprocessableEntity,
			"the copy names the unknown action %s", *existing.Action)
	}

	// Requeueing leaves the arguments of the copy alone, and the worker reads
	// them from the copy. A retry therefore runs what the copy records, and no
	// argument of the request that started it can reach the job.
	requeued, err := a.deps.Books.RequeueTx(ctx, tx, existing.ID)
	if err != nil {
		return books.Book{}, err
	}
	if err := a.queueAction(ctx, tx, action, requeued.ID); err != nil {
		return books.Book{}, err
	}

	return requeued, nil
}

func (a *api) queueAction(ctx context.Context, tx pgx.Tx, action books.Action, bookID int64) error {
	var (
		args river.JobArgs
		opts *river.InsertOpts
	)

	switch action.ID {
	case books.ActionDewarp:
		args = dewarp.Args{BookID: bookID}
		opts = &river.InsertOpts{Queue: dewarp.QueueName, MaxAttempts: dewarp.MaxAttempts}
	case books.ActionSplit:
		args = split.Args{BookID: bookID}
		opts = &river.InsertOpts{Queue: split.QueueName, MaxAttempts: split.MaxAttempts}
	case books.ActionOCR:
		args = ocr.Args{BookID: bookID}
		opts = &river.InsertOpts{Queue: ocr.QueueName, MaxAttempts: ocr.MaxAttempts}
	default:
		return reject(http.StatusUnprocessableEntity, "%s has no worker", action.Title)
	}

	_, err := a.deps.Queue.InsertTx(ctx, tx, args, opts)
	return err
}
