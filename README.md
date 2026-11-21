# ownocr

OCR for books. A book is an ordered collection of pages. Each book passes
through stages. Go backend, Vue 3 frontend, PostgreSQL, and a River job queue.
Docker runs everything.

## Stack

| Part      | Choice                          | Reason                                            |
| --------- | ------------------------------- | ------------------------------------------------- |
| Router    | `net/http` + chi                | Handlers stay plain `http.HandlerFunc`.           |
| WebSocket | coder/websocket                 | Small API, context aware.                         |
| Database  | pgx v5 pool                     | Native Postgres driver, no ORM.                   |
| Queue     | River                           | Jobs live in Postgres. One database, one backup.  |
| Frontend  | Vue 3 + TypeScript + Vite       | Hot reload in dev, static bundle in production.   |
| Static    | nginx (production only)         | The Go server never serves files.                 |
| PDF       | poppler-utils                   | `pdftoppm` renders any PDF, not just embedded art.|
| CPU images| Python sidecar, numpy + OpenCV  | Go has no equal. Alpine has no OpenCV wheel.      |
| Tests     | Playwright                      | Drives a real browser against the real stack.     |

## Run production

`docker-compose.yml` is the production stack, so it is what plain
`docker compose` commands act on.

```sh
cp .env.example .env      # then edit the password and ALLOWED_ORIGINS
docker compose up --build -d
```

nginx serves the built frontend on `APP_PORT`, proxies `/api` and `/api/ws` to
the backend, and serves `/files` from the data volume.

nginx publishes the only port of the stack. The backend and the database have
no `ports:` entry, so they are reachable on the internal network alone and
nothing reaches the API except through nginx. Put the host firewall or a reverse
proxy in front of `APP_PORT`, and nothing else needs closing.

## Start development

```sh
docker compose -f docker-compose.dev.yml up --build
```

Open http://localhost:5173. Vite proxies `/api` to the backend, WebSocket
included, and serves `/files` from the shared data volume.

| Service  | URL                              |
| -------- | -------------------------------- |
| Frontend | http://localhost:5173            |
| Backend  | http://localhost:8080/api/health |
| Sidecar  | http://localhost:8000/health     |
| Postgres | `localhost:5432`                 |

Development publishes all four, so you can reach the API with `curl` and the
database with `psql`. Production publishes none of them.

Vite reloads the browser when you edit frontend files. The Go server does not
reload itself. Restart it after a backend edit:

```sh
docker compose -f docker-compose.dev.yml restart backend
```

The sidecar does reload itself. Uvicorn watches the mounted source, so an edit
takes effect with no restart and no rebuild.

## The compose files

| File                          | Stack                                        |
| ----------------------------- | -------------------------------------------- |
| `docker-compose.yml`          | Production. The default.                     |
| `docker-compose.dev.yml`      | Development, with live reload and open ports. |
| `docker-compose.dev-e2e.yml`  | Overlay on development: MinIO, a fake endpoint, Playwright. |
| `docker-compose.prod-e2e.yml` | The same overlay on production.              |

The deploy file sets no project name, so it takes the one Compose derives from
the directory. Development and the two test overlays name themselves, which
keeps four sets of volumes apart: a test run cannot empty the database you
deploy or the one you develop against. Both stacks build from the same
Dockerfiles, at a different target.

## Books, stages, and actions

A book is a name plus an ordered list of pages. Upload a PDF or a set of JPEG
and PNG files. A worker turns the upload into pages.

A stage names what a book already is. The stages are `uploaded`, `dewarped`,
`split`, then `ocr`. A book carries its stage from the moment it is created and
keeps it for life, so a book that still runs is listed in the tab it is going
to.

An action names the work that moves a book on. Every action names the stage its
result lands in, and a stage offers every action that leads to a stage after it.
The `uploaded` tab therefore offers Dewarp, Split and OCR, `dewarped` offers
Split and OCR, `split` offers OCR, and `ocr` offers none: a book that carries it
has been read. Add a stage to
`Stages` and its action to `Actions`, both in `internal/books/stages.go`, then
ship the worker. The client needs no change: it reads both lists from
`/api/stages`.

An action never changes the book it runs on. It creates a copy that holds
`source_book_id`, and that reference marks the source book. A book has at most
one copy, so it takes exactly one action.

One list holds every book of a stage, ordered by state group first and by upload
time within a group. The groups are: the book failed, the book still runs, the
book is ready for an action, its copy runs, its copy is done. A running card
shows a static spinner and the phase its job reports. A failed card shows a
warning and the reason, and offers a retry. A ready card carries a checkbox. A
card whose copy runs carries a spinner mark and fades; one whose copy is done
carries a green checkmark and fades further.

The top bar holds Upload, then Select all. A selection adds Deselect all and a
menu of the same actions the context menu holds, which then apply to every
selected book.

Selecting a book shows its pages five to a row, in cells whose shape the server
derives from the page dimensions. Selecting a page opens the reader, which
stacks every page down the content column.

## Page operations

A right click on a page, in the grid or in the reader, opens a menu that acts on
that page alone. The reader offers one item the grid does not: the rescan below.

Delete removes the page from this book and from every book made from it. It
never touches the book this one came from. Every page of a copy names the page
it was made from, in `book_page.source_page_id`, and the delete follows those
references down. A page index never changes, so a delete leaves a gap. The index
identifies a page inside its book; it does not number the pages for a reader.

Show original opens the page beside the page it was made from, sized to the
window. Use original replaces this page with that one, which rolls back one
stage for one page. The copy keeps its own bytes: the renditions are built again
under a storage key of this book.

A rolled back page carries `book_page.rolled_back` and offers the rollback no
more, because it already shows the page it would install. A worker that rebuilds
a copy writes new page rows, so a retry clears the mark for the whole book.

Edit moves the cut of a page a split made out of a spread. It opens the whole
spread, with a bar on the cut column and a grey layer over the part this page
leaves out. Save cuts the page out of the spread again at the column the bar
stands on. Cancel changes nothing.

A cut page records its cut in `book_page.cut_side` and `book_page.cut_column`.
The column counts in the columns of the stored image of the source page: the
left page holds every column before it, the right page holds it and every column
after. A page that reached the copy whole records neither, and offers no Edit.

Every save cuts out of the source page, never out of the page it replaces, so a
user who cut too far can cut back out and lose no column. A save moves one page:
the other half of the spread keeps the cut it holds, so the two may overlap or
leave a strip of the spread in neither. A retry of the split job writes new page
rows, which carry the cuts the sidecar reports.

Rescan with Extra effort reads one page again with PaddleOCR-VL. It appears on a
page of a complete `ocr` book that holds no `vl` result, on both halves of the
reader. The read half offers it alone, because every other item reaches the
image. Choosing it marks the page: the read half gains an orange outline and nothing is
sent. Marks accumulate, and a control at the right of the reader top bar counts
them. See "Rescan".

The rollback is abstract. An action declares `OffersRollback`, and only such an
action offers one. Dewarp does. Split does not, because splitting a spread makes
two pages from one. OCR does not either: it reads a page without changing it, so
the page a rollback would install is the page the copy already shows. The
interface reads one boolean off the book and names no action.

Both operations refuse while any book of the chain is `pending` or `processing`.
A running job reads the pages of a source book and writes the pages of a copy,
so changing either set underneath it would leave the copy disagreeing with its
source. A rollback also needs a book with no copy of its own: it undoes one
stage, and a copy has already consumed the page.

## API

| Method | Path                          | Purpose                                     |
| ------ | ----------------------------- | ------------------------------------------- |
| GET    | `/api/health`                 | Service state. Pings the database.          |
| GET    | `/api/stages`                 | Ordered stages, book counts, and actions.   |
| GET    | `/api/books`                  | Every book, newest first.                   |
| GET    | `/api/books?stage=uploaded`   | Every book of one stage, in list order.     |
| GET    | `/api/books/{id}`             | One book with its pages and grid cell size. |
| POST   | `/api/books`                  | Multipart upload. Creates and queues a book.|
| POST   | `/api/books/actions/{action}` | Runs an action on the named books.          |
| GET    | `/api/books/{id}/pages/{index}/deletion` | What a page delete would remove. |
| POST   | `/api/books/{id}/pages/{index}/delete`   | Deletes a page down the chain.   |
| GET    | `/api/books/{id}/pages/{index}/rollback` | The two pages a rollback compares.|
| POST   | `/api/books/{id}/pages/{index}/rollback` | Returns a page to its source.    |
| GET    | `/api/books/{id}/pages/{index}/cut`      | The parent image and the cut.    |
| POST   | `/api/books/{id}/pages/{index}/cut`      | Cuts a page out of its parent again.|
| POST   | `/api/books/{id}/rescan`      | Reads named pages again with Extra effort.  |
| GET    | `/api/ws`                     | WebSocket stream of book events.            |

`POST /api/books` takes `multipart/form-data` with a `name` field and one or
more `files` parts. A book is either exactly one PDF or a set of JPEG and PNG
files, never both. The page order is the order the files arrive in.

`POST /api/books/actions/{action}` takes `{"bookIds": [1, 2, 3]}`, plus the
arguments of that action. Only OCR takes one: `"ocrProfile"`, which is `"v6"` or
`"vl"`. A request that omits an argument its action needs is refused whole, with
422, before any book is touched.

Every book runs in its own transaction, so one refusal cannot undo the others.
The answer holds the copy of every book that started, and a reason for every
book that did not:

```json
{ "started": [{ "id": 4, "stage": "dewarped" }], "failed": [{ "bookId": 2, "error": "no such book" }] }
```

An unknown action answers `404`. A retry runs the action of a failed copy on the
book it came from, which reuses the same copy.

Every WebSocket message is one JSON object:

```json
{ "type": "book.updated", "payload": { "id": 1, "state": "complete", "stage": "uploaded" } }
```

Types are `book.created` and `book.updated`. The client reconnects with backoff.

## Book flow

1. `POST /api/books` streams every file to `tmp/<upload-id>` without buffering.
2. It validates the name and the file set, sniffing types from the bytes.
3. It moves the upload to `sources/<book-key>`, so the bytes are durable first.
4. One transaction inserts the book row and the River job. A rollback leaves
   neither. This is why River uses the same database.
5. The worker sets `processing` and the phase "Converting", renders or copies
   each page, writes three renditions, and inserts a page row per page.
6. The worker sets `complete` and clears the phase. The stage never changes.
7. Each step broadcasts a `book.updated` event to every WebSocket client.

A running book reports one phase, from a fixed list: "Waiting in local queue",
"Converting", "Uploading", "Waiting for AI", "Processing with AI", and
"Downloading". A complete or failed book reports none.

River retries a failed ingest three times. Only the last attempt marks the book
`failed`. Every attempt first deletes the book's pages and page files, so a
retry cannot leave duplicates.

## Dewarp flow

Dewarping needs a GPU, so it runs on a RunPod serverless endpoint built from
`runpod/dewarp/`. The two sides exchange pages through an S3 bucket.

1. `POST /api/books/actions/dewarp` takes the source book. One transaction
   creates the copy, in the `dewarped` stage, and its River job.
2. The copy holds `source_book_id`. That reference marks the source book, and
   the state of the copy decides which mark the source shows.
3. The worker uploads the original rendition of every source page to
   `dewarp/<copy key>/in/0001.jpg`, reporting "Uploading". The padded name
   carries the page order.
4. It starts one endpoint job and polls it, reporting "Waiting for AI" while the
   job waits and "Processing with AI" while it runs. The job input names the
   bucket, the two prefixes, and the credentials the endpoint needs.
5. The endpoint writes one dewarped page per input under
   `dewarp/<copy key>/out/`.
6. The worker downloads every page, reporting "Downloading", then writes the
   three renditions and the page rows in the source order, reporting
   "Converting". The two passes keep the reported phase honest.
7. It deletes the whole prefix, then completes the copy.

The copy fails if any page fails. A failed copy keeps its reason and its
reference, and the interface offers a retry that reuses the same copy. Every
attempt first clears the prefix and the copy's pages, so a retry cannot mix two
attempts.

Dewarp runs on its own River queue with one worker. One book moves gigabytes,
and two at once help nobody.

The RunPod client is `internal/runpod`. It starts a job, polls it to its end,
and answers with the raw result. Dewarp and OCR share it and keep their own job
input and job output types, because the two endpoints share the transport, not
the contract.

## OCR flow

OCR reads the text off every page on a second RunPod endpoint, built from
`runpod/paddleocr/`. It is the last action of the pipeline: its copy lands in
`ocr`. The flow follows the dewarp flow object for object, with three
differences.

**The action takes a model.** The dialog offers two profiles. Normal runs
PP-OCRv6 alone: quick, with a box and a confidence for every line, and no
polytonic Greek. Extra runs PaddleOCR-VL as well, the visual model, which reads
the pages v6 cannot. The copy records the choice in `book.ocr_profile`, and the
worker reads it from there on every attempt, so a retry runs the model the copy
was started with. Every other action takes no argument.

**The endpoint answers text, not images.** It returns one result per engine per
page, so an Extra job returns two entries for every page and only the names of
the objects it wrote say which engine produced them. Each result becomes a
`page_ocr` row holding the engine, the text that engine wrote, and its full
pipeline result:

| column | v6 | vl |
|---|---|---|
| `txt` | the recognised lines, in detection order | markdown |
| `data` | the raw result plus `{text, score, bbox}` a line | the layout blocks, with a score a block |

Nothing rewrites either text. `txt` alone is lossy for VL, which leaves footnote
and page number blocks out of its markdown; they survive in `data`.

**A page read by both engines holds a third row.** Its model is `merged`, and it
carries the line boxes of v6 filled with the text of VL, so one row answers both
halves of the question. It is derived: the worker writes it from the two engine
rows after both are stored, and it changes neither. A page of a `v6` copy holds no
such row when the job ends, because one engine leaves nothing to join. A rescan
gives one to every page it reads. See "Rescan".

| column | merged |
|---|---|
| `txt` | the merged lines, one per line, in page order |
| `data` | `{bbox, text, source, score}` a line, plus `dropped` and `counts` |

`source` names the engine each line took its text from. Two tests refuse VL text
for a line, and either one is enough: too few of the characters of the v6 line
agree with it after folding, or the text offered is more than four times the length
of the line. The first catches a page VL misread, the second a line VL began
correctly and then ran on. A refused line keeps its v6 text, and the refused text
appears in `dropped` with its reason. Every VL character therefore reaches a
line or appears in `dropped`, and `counts` says how many did each. See
`internal/ocrmerge`. `backend/tools/ocrmerge` writes the row for a copy read
before it existed, without going back to the endpoint.

The reader draws these rows over the page. See "Page operations" and "Rescan".

**The copy owns no files.** OCR reads a page without changing it, so a page of
the copy keeps the storage key of its source page and names the book that holds
the image in `book_page.image_book_key`. No rendition is written and no byte is
copied. Two rules already in the system keep that safe: a book that has a copy
offers no page rollback and no re-cut, so the image cannot change underneath;
and rejecting a book destroys every copy made from it in the same request, so no
page can outlive the image it shows.

1. `POST /api/books/actions/ocr` takes `bookIds` and `ocrProfile`. It refuses a
   request that names no profile, before any book is touched. It then creates
   the copy, in the `ocr` stage, and its River job, in one transaction.
2. The worker uploads the original rendition of every source page to
   `ocr/<copy key>/in/0001.jpg`. The prefix differs from the dewarp prefix, so
   two actions on one book never meet in the bucket.
3. It starts one endpoint job, naming the engines of the profile, and polls it,
   reporting the same phases dewarp reports.
4. The endpoint writes the objects of every engine under `ocr/<copy key>/out/`,
   named after the input: `0001.v6.txt` and `0001.v6.json` for v6, `0001.md` and
   `0001.json` for vl.
5. The worker downloads every object, pairing a result with its page and its
   engine, then writes the page rows and their results in the source order.
6. It deletes the whole prefix, then completes the copy.

The copy fails if any engine of the profile cannot read every page, counted per
engine, and a retry rebuilds the whole copy, as dewarp does. Deleting the pages
of a copy removes their results through the cascade, so a retry cannot leave two
results for one engine. OCR runs on its own River queue with one worker.

## Rescan

A book read with the Normal profile holds PP-OCRv6 text on every page. Some
pages defeat that engine. A rescan reads chosen pages again with PaddleOCR-VL,
on the book itself. It creates no copy.

This is the one operation that changes a book an action already finished. Three
rules keep it safe.

**It runs the visual engine alone.** The PP-OCRv6 result of every page is
already in `page_ocr`, so the merge reads it from there. The endpoint job names
`vl` and nothing else, which halves the work and leaves the PP-OCRv6 result
untouched.

**It stores page by page.** A whole book reading refuses a partial result,
because a copy with fewer pages than its source is worse than no copy. A rescan
only adds rows, so a page stored is a page improved. A page the endpoint refuses
costs the others nothing.

**It never fails the book.** The book returns to `complete` whatever happens,
including a rescan that read no page. This is not politeness. A failed `ocr`
copy offers a retry, and that retry deletes every page of the copy, which drops
every `page_ocr` row through the cascade. A rescan that set `failed` would offer
the control that destroys every earlier rescan of that book. Because
`startAction` refuses a source book whose copy exists and has not failed,
`failed` is the only route into a rebuild, so never writing it closes the path.

`book.ocr_profile` therefore records how the book was first read, not what read
each page. The `page_ocr` rows are the authority: a page holding a `vl` row has
been rescanned. `GET /api/books/{id}` reports that per page, as `hasVisualText`.

The reader marks and counts, then leaves. Confirming starts the rescan and
returns to the list of the `ocr` stage, where the book runs like any other. The
marks go with the reader: they are never stored.

A rescan that reached a page leaves that page offering the rescan no more. A
page it missed still offers it, because the offer reads the `page_ocr` rows
rather than any record of the attempt. Opening the reader again therefore shows
what took and what did not, one page menu at a time. Nothing else reports a
rescan that failed: the stored rows are the whole account.

A rescan runs on the OCR queue, so it and a whole book reading never reach the
endpoint at once. It refuses while the book is not `complete`, which is also
what stops two rescans of one book.

## Split flow

Splitting straightens a photographed spread and cuts it into its two pages. The
work needs numpy and OpenCV, so it runs in the `sidecar` service. The two sides
share the data volume: the worker sends page paths and a directory, the sidecar
writes the pages there, and no page bytes cross the HTTP boundary.

1. The worker makes a directory under `/data/tmp` for the batch, then sends the
   original renditions of the source pages and that directory to `POST /split`
   on the sidecar, twenty at a time.
2. The sidecar writes the pages of every image into the directory and answers
   one entry per page: the names it wrote, in reading order. One name for a page
   that is not a spread, two for a page that is.
3. The worker builds its three renditions from every file the answer names, in
   order. It numbers the pages of the copy from one, so a book of 68 pages with
   67 spreads gives a copy of 135. Both pages of a spread name the same source
   page.
4. The worker removes the directory when the batch ends, whether it succeeded or
   failed. A retry therefore never reads what an interrupted attempt left there.
5. A page that is not a spread keeps the bytes of its source, so a PNG stays a
   PNG. The sidecar copies that file rather than encoding it again.

The sidecar straightens a spread before it cuts. It turns the photo until the
gutter stands upright, then keeps the largest upright rectangle inside the
turned photo, so no page carries a blank corner. That trims the photo once, at
its four outer edges, and takes nothing off the gutter side.

The order matters. Cutting first and turning each half would trim twice, and the
gutter-side edge of a page would land where the trim put it rather than where
the detector found the gutter. A spread that already stands square within a
quarter of a degree is cut with no resample at all.

The sidecar finds the gutter as the only tall vertical structure in the image.
It reads a dark shadow band and a bright pale ridge alike, because a book held
open and a book lying flat show opposite gutters.

Whether an image is a spread rests on coverage, the share of the height the
gutter runs over, not on how strong the gutter is. Strength alone cannot decide
it: a cover carries a strong short mark around its title and would pass.

An image the sidecar finds no gutter in is cut at its middle when it is
landscape, which means at least 1.2 times wider than it is tall. A spread is
wider than it is tall, and its shape is evidence the gutter search does not use.
The threshold is not `width > height`: a page photographed with a little of the
desk on either side is wider by a few percent, and cutting that one would be
wrong.

A gutter the sidecar finds always decides the cut. The middle answers only when
it finds none. The two pages then meet at the middle column exactly, because
there is no curve to follow and nothing to overlap. A landscape single page is
cut as well, which the Edit item of the page menu puts back: delete one half and
move the cut of the other out to the full width.

The two columns overlap, because the gutter leans and bows while a crop takes a
column. Each half therefore keeps every column of its own page and carries a
slanted strip of the facing page along its inner edge. The strip stays: painting
it out needs a model of the paper, and removing it needs the pixels moved.

The worker records the cut of every page it cuts, in `book_page.cut_side` and
`book_page.cut_column`. A left page records `left` and the column it ends at; a
right page records `right` and the column it starts at. That record is what the
Edit item of the page menu moves. See "Page operations".

Split has its own River queue with one worker. It needs no object storage and no
remote endpoint, and it fails the copy when the sidecar cannot read a page.

The dewarp and the OCR variables are required. The server refuses to start
without them. See `runpod/dewarp/README.md` and `runpod/paddleocr/README.md`
to build the images and create the endpoints.

## File store

The `book_data` volume mounts at `DATA_DIR`, read-write in the backend and
read-only in the frontend.

```
$DATA_DIR/
  pages/<book-key>/<page-key>.orig.<ext>    original bytes or rendered PDF page
                  <page-key>.1280.jpg       display rendition
                  <page-key>.480.jpg        thumbnail rendition
  sources/<book-key>/<uploaded files>       the upload, kept for a re-render
  tmp/<upload-id>/                          in-flight upload, removed on failure
```

Only `pages/` is served, at `/files/`. `sources/` and `tmp/` sit outside the
served root and are unreachable over HTTP by construction.

A 500 page book at 300 DPI uses roughly 3 GB across the three renditions and
the source. Nothing prunes the store yet, so watch the volume.

## Tests

The Go tests need the toolchain and the mounted source, so they run in the
development backend:

```sh
docker compose -f docker-compose.dev.yml exec backend go test ./...
docker compose -f docker-compose.dev.yml exec sidecar pytest
./run-e2e.sh                    # Playwright, whole stack
./run-e2e.sh grid.spec.ts       # one file
```

The sidecar tests draw their own spreads and single pages, so they need no
photograph in the repository. They live in the development image alone: the
production image installs `requirements.txt` and nothing else.

`run-e2e.sh` builds the stack, waits for health, runs the suite, and tears down.
Set `KEEP_STACK=1` to leave the containers running.

The e2e stack adds MinIO and `backend/tools/fakerunpod`, a stub endpoint that
shrinks every page instead of dewarping it, and writes the objects of every
engine a job names, holding the name of a page instead of reading it. It refuses
an OCR job that names no engine, as the real handler does. The input prefix names
the action. The whole dewarp path and the whole OCR path therefore run without a
GPU and without a RunPod account.

The e2e suite covers every user-observable scenario in
`openspec/specs/`. Fixtures come from `backend/tools/fixturegen`, which writes
deterministic JPEGs and minimal PDFs with the standard library alone:

```sh
docker compose -f docker-compose.dev.yml exec backend \
  go run ./tools/fixturegen -out /app/internal/ingest/testdata
```

## The Python sidecar

`sidecar/` is a Python service for CPU image work: numpy and OpenCV, which Go
has no equal for. It exists as its own image because the backend runs on Alpine,
and the OpenCV wheels on PyPI need glibc. numpy has a musl wheel; OpenCV has
none.

It answers `GET /health` and `POST /split`. The split method takes a list of
page paths and a destination directory. It writes the pages of every image into
that directory and returns one entry per page: the names it wrote, in reading
order. A page keeps the name of the image it came from, and the right page of a
spread gains `-1` before its extension.

The code is a package, `ownocr`. Every action that moves to Python becomes a
subpackage, `ownocr/<action>/`, as `internal/dewarp` and `internal/split` are
siblings on the Go side. `ownocr/split/` is the first of them.

It holds no state, runs one worker, and answers one request at a time. Handlers
are `def`, never `async def`: Starlette runs a `def` handler in a thread pool, so
a minute inside OpenCV leaves the event loop free. Uvicorn puts no limit on how
long a request may take, because a caller bounds the request, not the server.

The Dockerfile builds two targets from one base, as `backend/Dockerfile` does.
`prod` runs as `ownocr`, uid 10001, the backend's user: both write the data
volume, and a worker deletes a page directory before every attempt, which a file
of another owner would refuse. `dev` installs the reload watcher and returns to
root, which matches the development backend.

## What OCR still needs

The OCR action reads a book and stores one `page_ocr` row per engine per page,
plus a `merged` row for a page both engines read. See "OCR flow". Nothing reads
those rows back: no route serves them, no view shows them, and no export builds a
document out of them. A page of an `ocr` book therefore looks like any other page.

The reader is what waits. The `merged` row of an Extra copy already holds a box
and a text for every line, which is what a view needs to draw text over the page,
and nothing draws it yet. Such a view also decides what to do with the VL markup
the merge keeps verbatim, and whether to show what the merge dropped.

Adding an action follows the same three steps as before: a stage in `Stages`,
its action in `Actions`, and a River worker that fills the copy. `internal/ocr`,
`internal/dewarp` and `internal/split` are the three patterns: a job on its own
queue that fills a copy of a book. OCR and Dewarp reach a paid GPU endpoint,
and Split reaches the sidecar beside it, which is where CPU image work belongs.

## Layout

```
backend/
  cmd/server/          Startup, wiring, shutdown
  internal/books/      Book and page store, stages, grid cell rule
  internal/config/     Environment variables
  internal/dewarp/     Dewarp worker, endpoint contract, object keys
  internal/ocr/        OCR worker, rescan worker, endpoint contract, object keys
  internal/runpod/     RunPod job client, shared by both endpoints
  internal/split/      Split worker and the sidecar client
  internal/objectstore/ S3 client wrapper
  internal/db/         Pool, SQL migrations
  internal/filestore/  Path layout, name sanitizing
  internal/httpapi/    Router, handlers, upload, page operations, view models
  internal/ingest/     Worker, PDF split, image pipeline
  internal/queue/      River client and migrations
  internal/ws/         WebSocket hub
  tools/fixturegen/    Deterministic test fixtures
frontend/
  nginx.conf           Production static server and proxy
  src/api/             Typed REST client
  src/components/      Upload dialog
  src/composables/     Shared reconnecting WebSocket
  src/components/      Book list, card, mark, breadcrumbs, upload dialog
  src/views/           Stage list, page grid, reader
  tools/fakerunpod/    Stub dewarp and OCR endpoint for the tests
e2e/
  fixtures/            Generated PDFs and image sets
  tests/               Playwright specs, one per capability
runpod/
  dewarp/              GPU worker image, handler, and build script
sidecar/
  ownocr/              The Python package. One subpackage per action
  ownocr/app.py        FastAPI app. Health and split
  ownocr/split/        Straightens a spread and writes its two pages
  tests/               pytest, run in the development image
  pyproject.toml       Package metadata, dependencies read from requirements
  requirements.txt     Pinned numpy, OpenCV, FastAPI, uvicorn
  requirements-dev.txt Development only, the reload watcher and the tests
```

## Database migrations

Put a new file in `backend/internal/db/migrations/`. Use the next number, for
example `0003_add_ocr.sql`. The server applies pending files at startup and
records them in `schema_migrations`. Never edit an applied file.

River owns its own tables and migrates itself.

## Environment variables

| Name               | Default | Purpose                                     |
| ------------------ | ------- | ------------------------------------------- |
| `ADDR`             | `:8080` | Listen address.                             |
| `DATABASE_URL`     | —       | Required. Postgres connection string.       |
| `ALLOWED_ORIGINS`  | `*`     | WebSocket origin allowlist. Comma separated.|
| `WORKERS`          | `4`     | Worker goroutines per backend instance.     |
| `DATA_DIR`         | `/data` | Root of the file store.                     |
| `MAX_UPLOAD_BYTES` | 2 GiB   | Total upload body limit.                    |
| `PDF_DPI`          | `300`   | `pdftoppm` render resolution.               |
| `AWS_ENDPOINT_URL` | —       | S3 endpoint. Empty for real AWS.            |
| `AWS_REGION`       | `us-east-1` | Bucket region.                          |
| `AWS_ACCESS_KEY_ID`| —       | Required. Bucket key.                       |
| `AWS_SECRET_ACCESS_KEY` | —  | Required. Bucket secret.                    |
| `AWS_BUCKET`       | —       | Required. Bucket the pages travel through.  |
| `DEWARP_API_URL`   | —       | Required. RunPod endpoint base URL.         |
| `DEWARP_API_TOKEN` | —       | Required. RunPod API key.                   |
| `PADDLEOCR_VL_API_URL` | —   | Required. RunPod OCR endpoint base URL.     |
| `PADDLEOCR_VL_API_TOKEN` | — | Required. RunPod API key for that endpoint. |
| `SIDECAR_URL`      | `http://sidecar:8000` | Where the split worker sends its pages. |

Set `ALLOWED_ORIGINS` to your real hosts in production. `*` accepts any origin.

The server refuses to start when a required variable is empty. It reports every
one it is missing at once, so one pass fixes them all. Development needs real
credentials or a local S3.

`AWS_ENDPOINT_URL` stays optional: an empty value means real AWS. The endpoint
receives the bucket credentials in every job, so use a key pair that reaches
that bucket and nothing else.

Ingest runs on its own River queue with one worker, so one large book cannot
starve the rest of the queue or the HTTP server. Dewarp, Split and OCR have a
queue of their own for the same reason.

## Notes for Go beginners

- `internal/` is a Go rule. Only this module can import those packages.
- Handlers get a `*http.Request`. Always pass `r.Context()` to database calls.
- `defer tx.Rollback(ctx)` after `Begin` is safe. A commit makes it a no-op.
- Errors travel up as return values. `main.go` is the only place that exits.
