package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/riverqueue/river"

	"github.com/themylogin/ownocr/backend/internal/books"
	"github.com/themylogin/ownocr/backend/internal/ocr"
)

type rescanRequest struct {
	PageIndexes []int  `json:"pageIndexes"`
	Profile     string `json:"profile"`
}

// rescanPages reads the named pages of a book that has already been read, with
// the engines of the profile the request names. It refuses the whole request
// when any page fails a check, so a request that names one page it cannot read
// changes nothing.
func (a *api) rescanPages(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "no such book")
		return
	}

	var request rescanRequest
	if err := readJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "the request must hold a pageIndexes list")
		return
	}

	book, indexes, err := a.startRescan(r.Context(), id, request.PageIndexes, request.Profile)
	if errors.Is(err, books.ErrNotFound) {
		writeError(w, http.StatusNotFound, "no such book")
		return
	}
	if err != nil {
		writeRejection(w, err, "cannot rescan the pages")
		return
	}

	slog.Info("rescan queued", "book", book.ID, "pages", indexes,
		"profile", request.Profile)

	summary := a.summary(r.Context(), book)
	a.deps.Hub.Publish("book.updated", summary)

	writeJSON(w, http.StatusOK, summary)
}

// startRescan checks the book, the profile and every page it names, then sets
// the book running and queues the job. The state of the book and the job commit
// together or not at all.
func (a *api) startRescan(ctx context.Context, bookID int64, asked []int,
	profile string) (books.Book, []int, error) {
	if profile == "" {
		return books.Book{}, nil, reject(http.StatusUnprocessableEntity,
			"profile must name the model to read with")
	}
	if !ocr.KnownRescanProfile(profile) {
		return books.Book{}, nil, reject(http.StatusUnprocessableEntity,
			"profile %q is not a model this system reads with", profile)
	}

	tx, err := a.deps.Pool.Begin(ctx)
	if err != nil {
		return books.Book{}, nil, err
	}
	defer tx.Rollback(ctx)

	if err := a.deps.Books.LockChainTx(ctx, tx, bookID); err != nil {
		return books.Book{}, nil, err
	}

	book, err := a.deps.Books.GetTx(ctx, tx, bookID)
	if err != nil {
		return books.Book{}, nil, err
	}
	if book.Stage != books.StageOCR {
		return books.Book{}, nil, reject(http.StatusUnprocessableEntity,
			"the book has not been read yet")
	}
	if book.State != books.StateComplete {
		return books.Book{}, nil, reject(http.StatusUnprocessableEntity,
			"the book is not complete")
	}

	reason, err := a.inFlight(ctx, bookID)
	if err != nil {
		return books.Book{}, nil, err
	}
	if reason != "" {
		return books.Book{}, nil, reject(http.StatusUnprocessableEntity, "%s", reason)
	}

	indexes, err := a.checkPages(ctx, bookID, asked)
	if err != nil {
		return books.Book{}, nil, err
	}

	running, err := a.deps.Books.SetProcessingKeepingCountTx(ctx, tx, bookID, books.PhaseQueued)
	if err != nil {
		return books.Book{}, nil, err
	}

	_, err = a.deps.Queue.InsertTx(ctx, tx,
		ocr.RescanArgs{BookID: bookID, PageIndexes: indexes, Profile: profile},
		&river.InsertOpts{Queue: ocr.QueueName, MaxAttempts: ocr.MaxAttempts})
	if err != nil {
		return books.Book{}, nil, err
	}

	return running, indexes, tx.Commit(ctx)
}

// checkPages answers the pages to read, in page order and without a repeat. A
// page named twice is read once, so the book bounds the request.
//
// No page is refused for the results it holds. A reading is a choice, not a
// rank: a page the visual engine read can read worse than the page the line
// engine read, so either engine can read any page again.
func (a *api) checkPages(ctx context.Context, bookID int64, asked []int) ([]int, error) {
	if len(asked) == 0 {
		return nil, reject(http.StatusUnprocessableEntity,
			"pageIndexes must name at least one page")
	}

	pages, err := a.deps.Books.Pages(ctx, bookID)
	if err != nil {
		return nil, err
	}

	held := make(map[int]bool, len(pages))
	for _, page := range pages {
		held[page.PageIndex] = true
	}

	seen := make(map[int]bool, len(asked))
	indexes := make([]int, 0, len(asked))
	for _, index := range asked {
		if !held[index] {
			return nil, reject(http.StatusUnprocessableEntity,
				"the book holds no page %d", index)
		}
		if seen[index] {
			continue
		}
		seen[index] = true
		indexes = append(indexes, index)
	}

	sort.Ints(indexes)
	return indexes, nil
}
