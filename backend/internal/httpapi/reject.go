package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/themylogin/ownocr/backend/internal/books"
)

type rejectionView struct {
	Books []bookSummary `json:"books"`
}

// rejection answers what a reject would destroy, so the interface can name it
// before it asks the user.
func (a *api) rejection(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "no such book")
		return
	}

	chain, err := a.deps.Books.Chain(r.Context(), id)
	if errors.Is(err, books.ErrNotFound) {
		writeError(w, http.StatusNotFound, "no such book")
		return
	}
	if err != nil {
		slog.Error("rejection", "book", id, "error", err)
		writeError(w, http.StatusInternalServerError, "cannot read the book")
		return
	}

	views, err := a.summaries(r.Context(), chain)
	if err != nil {
		slog.Error("rejection covers", "book", id, "error", err)
		writeError(w, http.StatusInternalServerError, "cannot read the book")
		return
	}

	writeJSON(w, http.StatusOK, rejectionView{Books: views})
}

// rejectBook destroys a book and every copy made from it. The source book it
// frees returns to the group of books that are ready for an action.
func (a *api) rejectBook(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "no such book")
		return
	}

	chain, err := a.deps.Books.Chain(r.Context(), id)
	if errors.Is(err, books.ErrNotFound) {
		writeError(w, http.StatusNotFound, "no such book")
		return
	}
	if err != nil {
		slog.Error("reject chain", "book", id, "error", err)
		writeError(w, http.StatusInternalServerError, "cannot reject the book")
		return
	}

	// The source of the whole chain outlives it. It loses its copy, so every
	// client must hear about it once the chain is gone.
	var freed *books.Book
	if source, ok := a.deps.Books.SourceOf(r.Context(), chain[len(chain)-1]); ok {
		freed = &source
	}

	destroyed := make([]int64, 0, len(chain))
	for _, book := range chain {
		if err := a.destroy(r.Context(), book); err != nil {
			slog.Error("reject", "book", book.ID, "error", err)
			writeError(w, http.StatusInternalServerError, "cannot reject the book")
			return
		}
		destroyed = append(destroyed, book.ID)
		a.deps.Hub.Publish("book.deleted", map[string]any{"id": book.ID, "stage": book.Stage})
	}

	slog.Info("books rejected", "books", destroyed)

	if freed != nil {
		if reloaded, err := a.deps.Books.Get(r.Context(), freed.ID); err == nil {
			a.deps.Hub.Publish("book.updated", a.summary(r.Context(), reloaded))
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"destroyed": destroyed})
}

// destroy removes the row first. A file left behind is waste; a row that points
// at missing files is a broken book.
func (a *api) destroy(ctx context.Context, book books.Book) error {
	if err := a.deps.Books.Delete(ctx, book.ID); err != nil {
		return err
	}

	for _, dir := range []string{
		a.deps.Files.PageDir(book.StorageKey),
		a.deps.Files.SourceDir(book.StorageKey),
	} {
		if err := os.RemoveAll(dir); err != nil {
			slog.Error("reject files", "book", book.ID, "dir", dir, "error", err)
		}
	}

	return nil
}
