package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/themylogin/ownocr/backend/internal/books"
)

func (a *api) health(w http.ResponseWriter, r *http.Request) {
	status := "ok"
	code := http.StatusOK

	if err := a.deps.Pool.Ping(r.Context()); err != nil {
		slog.Error("health ping", "error", err)
		status = "degraded"
		code = http.StatusServiceUnavailable
	}

	writeJSON(w, code, map[string]string{"status": status, "service": "ownocr"})
}

type stageView struct {
	ID      string         `json:"id"`
	Title   string         `json:"title"`
	Books   int            `json:"books"`
	Actions []books.Action `json:"actions"`
}

func (a *api) listStages(w http.ResponseWriter, r *http.Request) {
	views := make([]stageView, 0, len(books.Stages))

	for _, stage := range books.Stages {
		count, err := a.deps.Books.CountByStage(r.Context(), stage.ID)
		if err != nil {
			slog.Error("count stage", "stage", stage.ID, "error", err)
			writeError(w, http.StatusInternalServerError, "cannot list stages")
			return
		}

		actions := books.ActionsFor(stage.ID)
		if actions == nil {
			actions = []books.Action{}
		}
		views = append(views, stageView{
			ID:      stage.ID,
			Title:   stage.Title,
			Books:   count,
			Actions: actions,
		})
	}

	writeJSON(w, http.StatusOK, views)
}

func (a *api) listBooks(w http.ResponseWriter, r *http.Request) {
	var (
		list []books.Book
		err  error
	)

	stage := r.URL.Query().Get("stage")

	// A book now carries its stage in every state, so there is nothing left to
	// filter by state. An old client asking for one learns that here.
	if r.URL.Query().Has("state") {
		writeError(w, http.StatusUnprocessableEntity, "books are listed by stage, not by state")
		return
	}

	if stage != "" {
		if !books.HasStage(stage) {
			writeError(w, http.StatusUnprocessableEntity, "unknown stage "+stage)
			return
		}
		list, err = a.deps.Books.ListByStage(r.Context(), stage)
	} else {
		list, err = a.deps.Books.ListAll(r.Context())
	}

	if err != nil {
		slog.Error("list books", "error", err)
		writeError(w, http.StatusInternalServerError, "cannot list books")
		return
	}

	views, err := a.summaries(r.Context(), list)
	if err != nil {
		slog.Error("list covers", "error", err)
		writeError(w, http.StatusInternalServerError, "cannot list books")
		return
	}

	writeJSON(w, http.StatusOK, views)
}

func (a *api) getBook(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "no such book")
		return
	}

	book, err := a.deps.Books.Get(r.Context(), id)
	if errors.Is(err, books.ErrNotFound) {
		writeError(w, http.StatusNotFound, "no such book")
		return
	}
	if err != nil {
		slog.Error("get book", "book", id, "error", err)
		writeError(w, http.StatusInternalServerError, "cannot load the book")
		return
	}

	pages, err := a.deps.Books.Pages(r.Context(), id)
	if err != nil {
		slog.Error("get pages", "book", id, "error", err)
		writeError(w, http.StatusInternalServerError, "cannot load the book")
		return
	}

	writeJSON(w, http.StatusOK, detailOf(book, pages))
}
