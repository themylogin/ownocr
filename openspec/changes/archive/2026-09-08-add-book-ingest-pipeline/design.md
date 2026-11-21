## Context

See proposal.md - Why. The repository today is a working scaffold: chi router,
pgx pool, embedded SQL migrations, River queue, a WebSocket hub, and a
single-view Vue app. The demo `jobs` pipeline shows the intended shape - one
transaction inserts the row and the queue entry together - and this change
reuses that shape for books.

Constraints that shape the design:

- The README states that the Go server never serves files in production. nginx
  owns static delivery. The design keeps that split.
- One Postgres database holds both application data and the River queue.
- `readJSON` caps a request body at 1 MB. nginx caps a body at 25 MB. A book is
  far larger than either. Both limits change.
- The backend production image is `alpine:3.21` and runs as an unprivileged
  user. Any new binary must exist as an Alpine package.

## Goals / Non-Goals

**Goals:**

- One durable path from an upload to a browsable book, with no manual step.
- Bounded memory. Upload size and book size must not drive backend memory.
- A stage mechanism that takes a second stage without a model or UI rewrite.
- A retried ingest leaves clean state, never a half-ingested book.

**Non-Goals:**

- The dewarp algorithm. This change only lists books in the `dewarp` stage.
- Advancing a book out of `dewarp`. No stage transition exists yet.
- OCR of any kind. The placeholder recognizer goes away with nothing behind it.
- Delete, rename, reorder, or re-upload of a book.
- Authentication, users, and permissions. The app stays single tenant.
- Deduplication of identical uploads.

## Decisions

### Identifiers: serial primary keys plus a random storage key

`book` and `book_page` use `BIGSERIAL` primary keys, matching the existing
`jobs` table. Each row also carries a `storage_key`: 16 random bytes from
`crypto/rand`, hex encoded.

File names come from `storage_key`, never from the primary key and never from
the uploaded file name. Two books that each contain `scan.jpg` cannot collide.

*Alternative - UUID primary keys*: needs `github.com/google/uuid` plus pgx type
plumbing for `uuid.UUID`, and gives no benefit over a random text column.

*Alternative - name files by primary key*: file names become guessable and the
worker must insert the row before it can write the file. The random key lets the
worker write files first and insert the row after.

### Storage layout: one volume, three roots

```
$DATA_DIR/
  pages/<book-key>/<page-key>.orig.<ext>    original bytes or rendered PDF page
                  <page-key>.1280.jpg       display rendition
                  <page-key>.480.jpg        thumbnail rendition
  sources/<book-key>/<uploaded file name>   the uploaded PDF or images
  tmp/<upload-id>/                          in-flight upload, removed on failure
```

Only `pages/` is public. nginx maps `location /files/` to `$DATA_DIR/pages/`, so
a page URL is `/files/<book-key>/<page-key>.480.jpg`. `sources/` and `tmp/` sit
outside the served root and are unreachable over HTTP by construction, not by a
rule that could be misconfigured.

The volume mounts read-write into the backend and read-only into the frontend,
in development and in production alike.

*Alternative - one root with a path rule*: needs a regex location in nginx and a
matching rule in the Vite middleware. Two rules that must agree is a defect
waiting to happen. Separate roots need no rule at all.

### Serving files: a static server in both environments, never the Go process

Production maps nginx `location /files/` to `$DATA_DIR/pages/`. Development has
no nginx, so the Vite dev server serves the same directory through a connect
middleware in `vite.config.ts`:

```ts
server.middlewares.use('/files', sirv(pagesDir, { dev: true }))
```

`sirv` becomes a frontend devDependency. The `book_data` volume mounts read-only
into the frontend container in development as well as production, so both
environments mount the same volume at the same path.

The Go process serves no image byte in either environment, which keeps the
README rule intact. Content types, etags, and range requests come from a real
static server on both sides.

*Alternative - a Go handler behind a `SERVE_FILES` flag*: rejected. It adds a
code path that runs only in development, and the security-relevant part of it -
traversal and directory-listing suppression - would never run in production. A
guard placed where it does not matter is a guard nobody tests.

*Alternative - the Go server serves files in production too*: one code path, but
it puts every image byte through the Go process and contradicts the stated
architecture.

*Alternative - an nginx container in development running the production config*:
true parity, but an extra service and a second proxy hop for no gain over the
middleware.

### PDF rasterization: `pdftoppm`, one page per call

The ingest worker shells out to `pdfinfo` for the page count, then calls
`pdftoppm -jpeg -r 300 -singlefile -f N -l N` once per page. Both come from the
Alpine `poppler-utils` package, added to the `dev` and `prod` image stages.

One call per page bounds peak memory and temp disk to a single page, isolates a
failure to the page that caused it, and makes per-page progress reportable.
`-singlefile` gives an exact output name, so the worker does not guess how
`pdftoppm` zero-pads.

300 DPI is the resolution floor for reliable OCR. It is configurable through
`PDF_DPI`.

*Alternative - one `pdftoppm` call for the whole document*: fewer processes, but
peak temp disk equals the whole rendered book, and one bad page fails everything
with no way to say which page.

*Alternative - pure Go*: `pdfcpu` extracts embedded images. It does not render.
Scanned books whose pages are tiled or vector-wrapped come out wrong or empty.

### Resizing: `golang.org/x/image/draw` with Catmull-Rom

Decode with the standard library (`image/jpeg`, `image/png`), scale with
`draw.CatmullRom`, encode JPEG at quality 85. No cgo, no unmaintained
dependency. Peak memory is roughly `width * height * 4` bytes per page, about
34 MB for a 300 DPI letter page. The worker processes one page at a time per
goroutine, so peak memory scales with `WORKERS`, not with book size.

Both renditions come from one decode of the page. Neither rendition upscales:
when the page's longest side is under the target, the worker re-encodes at the
original size.

*Alternative - `github.com/disintegration/imaging`*: nicer API, unmaintained.
*Alternative - libvips through cgo*: much faster, but adds cgo and drops the
static build.

### Ingest is delete-then-rebuild

Each attempt starts by deleting the book's page rows and removing
`$DATA_DIR/pages/<book-key>/`. Then it rebuilds from `sources/`. An interrupted
attempt therefore cannot leave duplicate indexes or orphan files, and the job
needs no resume bookkeeping.

The cost is repeated work on a retry. Ingest is minutes, not hours, so this is
the right trade for correctness.

River `MaxAttempts` is 3 for ingest. The default of 25 would retry a genuinely
broken PDF for hours. The worker `Timeout` is 6 hours, because a 500 page book
at 300 DPI takes far longer than the demo worker's 2 minutes.

### Book state and stage are separate columns

`state` is one of `pending`, `processing`, `complete`, `failed` and describes
ingest. `stage` is nullable and holds the pipeline position, `dewarp` today.

A book gets a stage only when `state` becomes `complete`. This is why "pending
this stage" and "still ingesting" are two different lists in the interface: they
are two different columns, not two values of one column.

The stage list lives in one Go slice and is exposed by `GET /api/stages`. Adding
a stage means appending to that slice and shipping its worker. The tab row needs
no client change.

*Alternative - one `status` column*: merges two independent facts. A book that
fails during a later stage would then need a status per stage.

### Upload: streaming multipart, never buffered

`POST /api/books` reads `multipart/form-data` through `r.MultipartReader()` and
copies each part straight to `$DATA_DIR/tmp/<upload-id>/`. It never calls
`ParseMultipartForm`, which buffers to memory and to the OS temp directory.

`http.MaxBytesReader` enforces `MAX_UPLOAD_BYTES` (2 GiB default) on the whole
body. nginx `client_max_body_size` rises to match. `readJSON`'s 1 MB cap stays
for JSON endpoints and does not apply to this handler.

The handler validates the name and the file set, moves `tmp/<upload-id>` to
`sources/<book-key>`, then commits the book row and the River job in one
transaction. A failure anywhere removes the temp directory. The order matters:
bytes land durably before the row exists, so no book row can reference missing
files.

Content type comes from sniffing the first 512 bytes, not from the client's
declared part header. A browser sends an unreliable type for many files.

### Page order for an image set: the upload order, preserved end to end

Page order is the order the multipart parts arrive in. The client owns the
order; the server never sorts. This needs the order to survive three hops:

1. The upload handler reads parts sequentially and writes them to
   `tmp/<upload-id>/` as `0001_<name>`, `0002_<name>`, and so on. The numeric
   prefix, not the uploaded name, records the position.
2. The move to `sources/<book-key>` keeps those prefixed names.
3. The worker lists the directory and sorts by the prefix, which is fixed width
   and zero padded, so a plain lexical sort is exact.

The prefix also removes the duplicate-name problem: two files both called
`scan.jpg` become `0003_scan.jpg` and `0007_scan.jpg`.

The uploaded name is sanitized before it joins the prefix - path separators
stripped, length capped - because it reaches the filesystem.

*Alternative - sort by file name*: rejected. It guesses at intent and breaks
books whose scans are not numbered.

### The server computes the grid cell size

`GET /api/books/{id}` returns `gridCell: {width, height}` next to the pages. The
server holds every page dimension already, and the rule then has one
implementation with unit tests rather than a duplicate in TypeScript.

The rule:

1. Compute `aspect = width / height` for every page.
2. If any aspect is exactly 1, or aspects appear on both sides of 1, the cell is
   480 x 480.
3. If every aspect is above 1, the cell is `480 x round(480 / median)`.
4. If every aspect is below 1, the cell is `round(480 * median) x 480`.
5. Clamp the short side to a 240 px minimum.

The median resists a single odd page skewing the whole grid. Thumbnails render
with `object-fit: contain`, so no page is cropped and no page is enlarged.

### Frontend: vue-router with three routes

```
/                                 redirects to the first stage
/stage/:stage                     book list for one stage
/books/:id                        page grid
/books/:id/pages/:index           reader, scrolled to that page
```

`vue-router` is the one new frontend dependency. The reader uses native
`loading="lazy"` on each image, so a 500 page book does not fetch 500 images.
No virtual-scroll library is needed.

Live updates reuse the existing `useWebSocket` composable. The hub gains
`book.created` and `book.updated`; the event envelope does not change.

### API surface

| Method | Path                       | Purpose                                     |
| ------ | -------------------------- | ------------------------------------------- |
| GET    | `/api/stages`              | Ordered stages with a pending book count.   |
| GET    | `/api/books?stage=dewarp`  | Complete books in one stage.                |
| GET    | `/api/books?state=active`  | Books that are pending, processing, failed. |
| GET    | `/api/books/{id}`          | One book with pages and the grid cell size. |
| POST   | `/api/books`               | Multipart upload. Creates and queues a book.|

`GET /api/books` without a filter returns every book, newest first.

### Configuration

| Name               | Default | Purpose                                 |
| ------------------ | ------- | --------------------------------------- |
| `DATA_DIR`         | `/data` | Root of the file store.                 |
| `MAX_UPLOAD_BYTES` | `2 GiB` | Total upload body limit.                |
| `PDF_DPI`          | `300`   | `pdftoppm` render resolution.           |

## Risks / Trade-offs

**A single ingest saturates the CPU.** Rasterizing and resizing 500 pages keeps
every worker busy, and the API shares the process. → Ingest runs on a dedicated
River queue with `MaxWorkers` of 1 by default, so one book at a time. The HTTP
server keeps its own goroutines and stays responsive.

**Disk fills silently.** A 500 page book at 300 DPI holds roughly 3 GB across
three renditions plus the source. Nothing prunes. → Out of scope to solve here.
The health endpoint reports free bytes under `DATA_DIR`, so the problem is
visible before it is fatal. Document the growth rate in the README.

**A `pdftoppm` failure mode is a stuck process, not an error.** A malformed PDF
can hang the child. → The worker runs the child with `exec.CommandContext` under
a per-page timeout of 5 minutes, so the context kills it.

**The database and the file store can disagree.** A crash between the file write
and the row insert leaves orphan files. → Delete-then-rebuild removes orphans on
the next attempt for a book that retries. A book that never retries can leave
files in `pages/<book-key>/`, wasting space but breaking nothing. A cleanup
command is deferred.

**Serving files read-only from a shared volume couples the containers.** The
frontend container must mount the same volume as the backend, in both
environments. → Accepted. It is one line in each of the two compose files, and
it is the price of keeping the Go process out of static delivery.

**A very large `MAX_UPLOAD_BYTES` invites a slow-loris upload.** → The server
already sets `ReadHeaderTimeout`. Add a `WriteTimeout` exemption for the upload
route only and rely on nginx `client_body_timeout` in production.

**Dropping `jobs` destroys data.** → The table holds only demo rows. The
migration drops it outright. This is stated as **BREAKING** in the proposal.

## Migration Plan

1. Migration `0002` drops `jobs` and creates `book` and `book_page`.
2. Deploy needs the new `book_data` volume before the backend starts. Compose
   creates it automatically on `up`.
3. Rollback: revert the images and restore the previous compose file. The
   `book` and `book_page` tables stay behind, unused and harmless. `jobs` is
   gone; the old build recreates it empty through migration `0001`.
4. No data migration exists, because no production data exists.

## Open Questions

- Whether to keep the `orig` rendition for PDF pages once dewarp lands. Dewarp
  will read a rendition and write a new one, and the choice of source resolution
  belongs to that change.
- The JPEG quality of 85 is a starting point. Tune it against real OCR accuracy
  after the recognizer exists.
