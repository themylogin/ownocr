## Why

The repository holds a scaffold. It queues a fake OCR job for a file name and
prints placeholder text. It has no concept of a book, a page, or a stored image.

The real product OCRs books. A book is an ordered collection of page images. Each
book passes through a series of stages. Nothing can start until the app can accept
a book, store its pages, and show them. This change builds that foundation.

## What Changes

- **BREAKING**: Remove the demo job pipeline. The `jobs` table, the
  `/api/jobs` endpoints, the placeholder recognizer, and the demo UI go away.
  Nothing depends on them.
- Add the `book` and `book_page` tables. A book has a name, a state, and a stage.
  A page has a book, an index, a source file, and pixel dimensions.
- Add `POST /api/books`. It accepts a name plus a PDF or a set of JPEG and PNG
  files. It creates the book row and queues one ingest job in one transaction.
- Add an ingest worker. It splits a PDF into page images with `pdftoppm`. It
  stores every page under a unique file name. It writes a 480 px thumbnail, a
  1280 px display image, and the original.
- Mark the book complete when the worker finishes. The book then enters the
  `dewarp` stage and appears in the "Dewarp" tab.
- Add a file store on a Docker volume. nginx serves it at `/files/` in
  production. The Vite dev server serves the same directory in development. The
  Go process serves no images.
- Replace the frontend with three views: a stage tab list of books, a page grid
  for one book, and a full width page reader.
- Add a generic stage mechanism. This change ships one stage, `dewarp`. The
  dewarp algorithm itself is out of scope.

## Capabilities

### New Capabilities

- `book-upload`: Accepting a named book as a PDF or an image set, and queuing it
  for ingest.
- `book-ingest`: Turning an uploaded source into stored, ordered page images with
  thumbnails, then completing the book.
- `book-library`: Browsing books by stage, viewing one book's pages as a grid, and
  reading pages full width.

### Modified Capabilities

None. `openspec/specs/` is empty, so this change introduces the first specs.

## Impact

**Removed code**: `backend/internal/jobs/`, `backend/internal/ocr/`, the
`/api/jobs` routes, `frontend/src/App.vue` demo body.

**New backend packages**: `internal/books` (store), `internal/ingest` (worker,
PDF split, image pipeline), `internal/filestore` (paths, writes, name sanitizing).

**Changed backend**: `cmd/server/main.go` wiring, `internal/httpapi/router.go`,
`internal/config/config.go` (new `DATA_DIR` variable),
`internal/db/migrations/` (new `0002` migration).

**New dependencies**: `poppler-utils` in the backend image for `pdftoppm`.
`golang.org/x/image` for thumbnails. `sirv` and `vue-router` on the frontend. No
new services.

**Changed infrastructure**: `docker-compose.yml` and `docker-compose.prod.yml`
gain a `book_data` volume, mounted read-only into the frontend in both.
`frontend/nginx.conf` gains a `/files/` location and a larger body limit.
`frontend/vite.config.ts` serves `/files` in development.

**Changed frontend**: a router, three views, and an upload dialog. The WebSocket
hub stays and carries `book.created` and `book.updated` events.

**Risk**: An upload is large. The current 1 MB JSON body limit and the 25 MB nginx
limit both block a real book. Both need raising, and the upload must stream to
disk instead of buffering in memory.
