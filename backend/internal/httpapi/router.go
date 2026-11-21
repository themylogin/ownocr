package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/themylogin/ownocr/backend/internal/books"
	"github.com/themylogin/ownocr/backend/internal/filestore"
	"github.com/themylogin/ownocr/backend/internal/objectstore"
	"github.com/themylogin/ownocr/backend/internal/queue"
	"github.com/themylogin/ownocr/backend/internal/ws"
)

type Deps struct {
	Pool           *pgxpool.Pool
	Hub            *ws.Hub
	Files          *filestore.Store
	Books          *books.Store
	Queue          *queue.Client
	MaxUploadBytes int64
	Objects        *objectstore.Store
}

func NewRouter(deps Deps) http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)

	api := &api{deps: deps}

	router.Route("/api", func(r chi.Router) {
		r.Get("/health", api.health)

		r.Group(func(r chi.Router) {
			r.Use(middleware.Timeout(15 * time.Second))
			r.Get("/stages", api.listStages)
			r.Get("/books", api.listBooks)
			r.Get("/books/{id}", api.getBook)
			r.Post("/books/actions/{action}", api.runAction)
			r.Get("/books/{id}/rejection", api.rejection)
			r.Post("/books/{id}/reject", api.rejectBook)
			r.Get("/books/{id}/pages/{index}/deletion", api.pageDeletion)
			r.Post("/books/{id}/pages/{index}/delete", api.deletePage)
			r.Get("/books/{id}/pages/{index}/rollback", api.pageRollback)
			r.Post("/books/{id}/pages/{index}/rollback", api.applyPageRollback)
			r.Get("/books/{id}/pages/{index}/cut", api.pageCut)
			r.Post("/books/{id}/pages/{index}/cut", api.applyPageCut)
			r.Get("/books/{id}/pages/{index}/ocr", api.pageOCR)
			r.Post("/books/{id}/pages/{index}/reading", api.setPageReading)
			r.Get("/books/{id}/ocr", api.bookOCR)
			r.Post("/books/{id}/rescan", api.rescanPages)
		})

		// An upload runs far longer than a read, so it stays outside the
		// 15 second timeout group.
		r.Post("/books", api.createBook)

		r.Get("/ws", deps.Hub.ServeHTTP)
	})

	return router
}

type api struct {
	deps Deps
}
