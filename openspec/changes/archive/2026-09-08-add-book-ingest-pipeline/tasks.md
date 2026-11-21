All commands run through Docker. This machine needs `sudo` for Docker.
Build check: `sudo docker compose exec backend go build ./... && sudo docker compose exec backend go vet ./...`
Test check: `sudo docker compose exec backend go test ./...`
Frontend check: `sudo docker compose exec frontend npm run typecheck`

## 1. Remove the demo pipeline

- [x] 1.1 Delete `backend/internal/jobs/` and `backend/internal/ocr/`, and verify
      `grep -r "internal/jobs\|internal/ocr" backend/` returns nothing
- [x] 1.2 Remove the `/api/jobs` routes and the `Jobs` and `Queue` fields from
      `httpapi.Deps` in `router.go`, and delete `listJobs`, `createJob`, and
      `enqueue` from `handlers.go`; verify `health` is the only handler left
- [x] 1.3 Strip `cmd/server/main.go` down to config, pool, migrations, hub, and
      server. River refuses to start with an empty worker bundle, so the queue
      client returns in 8.1 together with the ingest worker; verify the backend
      builds and `GET localhost:8080/api/health` returns `{"status":"ok",...}`
- [x] 1.4 Reduce `frontend/src/App.vue` to a shell with a header and a
      `<RouterView />` placeholder, and remove `Job` and the job calls from
      `src/api/client.ts`; verify `npm run typecheck` passes

## 2. Infrastructure and configuration

- [x] 2.1 Add `RUN apk add --no-cache poppler-utils` to the `dev` and `prod`
      stages of `backend/Dockerfile`; verify
      `sudo docker compose exec backend pdftoppm -v` prints a version
- [x] 2.2 Add a `book_data` volume to `docker-compose.yml`, mounted at `/data`
      read-write in `backend` and read-only in `frontend`, and set
      `DATA_DIR=/data` on `backend`; verify
      `sudo docker compose exec backend touch /data/probe` succeeds and the same
      command in `frontend` fails
- [x] 2.3 Mirror the volume mounts and `DATA_DIR` in `docker-compose.prod.yml`;
      verify `sudo docker compose -f docker-compose.prod.yml config` resolves
      with no error
- [x] 2.4 Add `DATA_DIR`, `MAX_UPLOAD_BYTES`, and `PDF_DPI` to
      `internal/config/config.go` with the defaults from design.md, and reject a
      non-positive `MAX_UPLOAD_BYTES` or `PDF_DPI`; verify a table test covers
      each default and each rejection
- [x] 2.5 Add a `location /files/` block to `frontend/nginx.conf` with
      `alias /data/pages/;`, `autoindex off`, and a one year cache header, and
      raise `client_max_body_size` to `2g`; verify
      `sudo docker compose -f docker-compose.prod.yml exec frontend nginx -t` passes
- [x] 2.6 Add `sirv` as a devDependency and serve `/data/pages` at `/files` from
      a `configureServer` middleware in `frontend/vite.config.ts`; verify a known
      page file returns 200 at `localhost:5173/files/...`, a directory request
      returns 404, and `/files/../sources/` reaches nothing outside `pages/`

## 3. Data model

- [x] 3.1 Write `backend/internal/db/migrations/0002_books.sql`: drop `jobs`,
      create `book` (id, name, storage_key, state, stage, page_count, error,
      created_at, updated_at) and `book_page` (id, book_id, page_index,
      storage_key, original_ext, width, height), with a unique index on
      `(book_id, page_index)`, a foreign key that cascades on delete, and an
      index on `(stage, created_at DESC)`; verify the backend starts and logs
      `migration applied name=0002_books.sql`
- [x] 3.2 Create `backend/internal/books/store.go` with the `Book` and `Page`
      types, the state constants, and `NewStorageKey` returning 16 hex-encoded
      random bytes; verify a test asserts two calls differ and each is 32 chars
- [x] 3.3 Add store reads: `ListByStage`, `ListActive`, `ListAll`, and `Get`
      returning a book with its pages ordered by `page_index`; verify a test
      against the dev database covers each and returns `[]` not `nil` when empty
- [x] 3.4 Add store writes: `CreateTx`, `SetProcessing`, `SetComplete` (sets
      `stage` to the first stage), `SetFailed`, `SetPageCount`, `DeletePages`,
      and `InsertPage`; verify a test round-trips a book through every state

## 4. File store

- [x] 4.1 Create `backend/internal/filestore/store.go` resolving `pages/`,
      `sources/`, and `tmp/` under `DATA_DIR`, creating each at startup; verify
      the three directories exist after the backend starts
- [x] 4.2 Add path helpers `PageDir`, `SourceDir`, `NewTempDir`, and
      `PageURL(bookKey, pageKey, rendition)` returning
      `/files/<bookKey>/<pageKey>.<rendition>`; verify unit tests pin the exact
      strings for all three renditions
- [x] 4.3 Add `SanitizeName` stripping path separators and control characters and
      capping length at 128 bytes; verify tests cover `../../etc/passwd`, a
      backslash, an empty result, and a name at the cap

## 5. Image pipeline

- [x] 5.1 Add `golang.org/x/image` to `go.mod` and create
      `backend/internal/ingest/image.go` with `Decode` handling JPEG and PNG and
      returning the pixel dimensions; verify a test decodes both formats and
      rejects a truncated file
- [x] 5.2 Implement `Resize(img, longestSide)` using `draw.CatmullRom`, which
      returns the source unchanged when it is already smaller; verify tests cover
      3000x2000 to 480x320, 2000x3000 to 320x480, and 300x200 staying 300x200
- [x] 5.3 Implement `WriteRenditions(page)` writing `.orig.<ext>`, `.1280.jpg`,
      and `.480.jpg` from one decode at JPEG quality 85; verify a test on a real
      JPEG produces three files with the expected dimensions and that the
      original bytes are unchanged

## 6. PDF splitting

- [x] 6.1 Create `backend/internal/ingest/pdf.go` with `PageCount` shelling out
      to `pdfinfo` and parsing the `Pages:` line; verify a test against a fixture
      PDF returns the right count and a non-PDF returns an error
- [x] 6.2 Implement `RenderPage(ctx, pdfPath, pageNumber, dpi, outBase)` calling
      `pdftoppm -jpeg -r <dpi> -singlefile -f N -l N` under
      `exec.CommandContext` with a 5 minute timeout; verify a test renders page 1
      of a fixture and that a cancelled context kills the child
- [x] 6.3 Add a multi-page fixture PDF under `backend/internal/ingest/testdata/`;
      verify `go test ./internal/ingest/...` passes inside the container

## 7. Upload endpoint

- [x] 7.1 Add `POST /api/books` to the router outside the 15 second timeout
      group, wrapped in `http.MaxBytesReader` with `MAX_UPLOAD_BYTES`; verify a
      body over the limit returns 413
- [x] 7.2 Implement the handler with `r.MultipartReader()`, streaming each file
      part to `tmp/<upload-id>/NNNN_<sanitized name>` and reading the `name`
      field; verify an 800 MB upload succeeds while backend RSS stays flat under
      `sudo docker stats`
- [x] 7.3 Validate the request: non-empty trimmed name, at least one file,
      content type sniffed from the first 512 bytes, and either exactly one PDF
      or only JPEG and PNG files; verify tests cover every rejection scenario in
      `specs/book-upload/spec.md` with the stated status codes
- [x] 7.4 On success, move `tmp/<upload-id>` to `sources/<book-key>`, then commit
      the book row and the River ingest job in one transaction; verify a forced
      commit failure leaves no row, no job, and no `sources/` directory
- [x] 7.5 Remove the temp directory on every failure path; verify `tmp/` is empty
      after a run of the rejection tests
- [x] 7.6 Broadcast `book.created` after the commit; verify a `websocat` client
      receives the event with the book id, name, and state

## 8. Ingest worker

- [x] 8.1 Create `backend/internal/ingest/worker.go` with `Args{BookID}`, kind
      `ingest`, a 6 hour `Timeout`, and registration on a dedicated `ingest`
      River queue with `MaxWorkers` 1 and `MaxAttempts` 3; verify the queue name
      appears in the River startup log
- [x] 8.2 Start each attempt by setting the book to `processing`, deleting its
      page rows, and removing `pages/<book-key>/`; verify a test that seeds
      partial pages ends with exactly the rebuilt set
- [x] 8.3 Implement the PDF branch: count pages, then render, decode, write
      renditions, and insert a page row one page at a time; verify a 10 page
      fixture yields 10 rows with indexes 1 to 10 and 30 files on disk
- [x] 8.4 Implement the image branch: list `sources/<book-key>` sorted by the
      numeric prefix, then write renditions and insert a row per file; verify a
      test with files uploaded as `b.jpg`, `a.jpg`, `c.jpg` keeps that order
- [x] 8.5 Set the book `complete` with its page count and the `dewarp` stage when
      every page lands, and broadcast `book.updated`; verify the book appears in
      `GET /api/books?stage=dewarp`
- [x] 8.6 Report failure only on the final attempt, storing the reason and naming
      the file that failed, and broadcast `book.updated`; verify a corrupt image
      fixture leaves state `failed` with the file name in the reason, and that an
      earlier attempt leaves state `processing`
- [x] 8.7 Broadcast a progress `book.updated` every 10 pages; verify a
      `websocat` client sees rising page counts during a 30 page ingest

## 9. Read API

- [x] 9.1 Add `internal/books/stages.go` holding the ordered stage slice with one
      entry, `dewarp` titled "Dewarp", and `GET /api/stages` returning each stage
      with its pending book count; verify the response lists one stage
- [x] 9.2 Add `GET /api/books` honouring `stage` and `state=active`, newest
      first, each entry carrying id, name, state, stage, page count, and the
      cover thumbnail URL; verify a book with no pages returns a null cover
- [x] 9.3 Implement `GridCell(pages)` in `internal/books` following the five step
      rule in design.md; verify tests cover all-landscape, all-portrait, mixed,
      an exactly square page, the 240 px clamp, and a single-page book
- [x] 9.4 Add `GET /api/books/{id}` returning the book, its ordered pages with
      width, height, and all three rendition URLs, and `gridCell`; verify an
      unknown id returns 404

## 10. Frontend shell and stage list

- [x] 10.1 Add `vue-router` and register the four routes from design.md, with
      `/` redirecting to the first stage; verify `npm run typecheck` passes and
      each URL renders its view
- [x] 10.2 Add the typed API client functions for stages, books, book detail, and
      upload; verify no `any` remains and typecheck passes
- [x] 10.3 Build the stage tab row from `GET /api/stages`, keeping the selected
      stage in the URL and falling back to the first stage for an unknown one;
      verify a reload keeps the tab and `/stage/nope` lands on Dewarp
- [x] 10.4 Build the stage book list showing name and cover, with an empty state
      that invites an upload; verify a book in `dewarp` renders with its first
      page thumbnail
- [x] 10.5 Build the separate active-books area above the list for `pending`,
      `processing`, and `failed` books, showing the reason on failure; verify a
      failed book shows its message and never appears in the stage list
- [x] 10.6 Subscribe to `book.created` and `book.updated` and move a book between
      the two areas without a reload; verify a book uploaded in a second browser
      tab appears in the first tab's Dewarp list when ingest completes

## 11. Frontend book grid and reader

- [x] 11.1 Build the page grid view using the server `gridCell` for
      `grid-template-columns` and a fixed cell height, with
      `object-fit: contain` on each thumbnail; verify a landscape book renders
      480x320 cells and a mixed book renders 480x480 cells in the inspector
- [x] 11.2 Link each thumbnail to `/books/:id/pages/:index`; verify selecting
      page 42 opens the reader with page 42 in view
- [x] 11.3 Build the reader stacking every 1280 rendition full width in order,
      with `loading="lazy"` on each image and a back control to the grid; verify
      a 500 page book issues only a handful of image requests on open in the
      network panel

## 12. Frontend upload

- [x] 12.1 Build the upload dialog reachable from every stage tab, with a
      required name field and a file selector accepting PDF, JPEG, and PNG;
      verify the submit control stays disabled while the name is blank
- [x] 12.2 Prefill the name from a selected PDF's file name without its
      extension, and leave it blank for an image set; verify selecting
      `Moby Dick (1851).pdf` fills `Moby Dick (1851)` and that an edit sticks
- [x] 12.3 List the selected file names in send order and post them as
      `multipart/form-data` in that order; verify the request parts match the
      listed order in the network panel
- [x] 12.4 Show upload progress and surface a server rejection message inline;
      verify a mixed PDF-plus-image selection shows the 422 message

## 13. Documentation

- [x] 13.1 Rewrite the README API table, job flow, and layout sections for books,
      stages, and files, and drop the "Add real OCR" section; verify no reference
      to `/api/jobs` or `internal/ocr` remains
- [x] 13.2 Document `DATA_DIR`, `MAX_UPLOAD_BYTES`, and `PDF_DPI` in the README
      table and in `.env.example`, state that nginx serves `/files/` in
      production and Vite serves it in development, and note the roughly 3 GB of
      disk a 500 page book uses; verify both files list all three variables

## 14. End-to-end verification

- [x] 14.1 Upload a real multi-page PDF through the browser and confirm the book
      moves from the active area into the Dewarp list, with a cover, a regular
      grid, and a readable reader
- [x] 14.2 Upload a set of images in a deliberate non-alphabetical order and
      confirm the page order matches the selection order
- [x] 14.3 Run the production stack with
      `sudo docker compose -f docker-compose.prod.yml up --build -d` and confirm
      nginx serves `/files/` and that `/files/../../sources/` is unreachable
- [x] 14.4 Run `go build ./...`, `go vet ./...`, `go test ./...`, and
      `npm run typecheck`; verify all four pass

## 15. End-to-end test suite

Playwright drives a real browser against the real stack. Every user-observable
scenario in the three spec files gets a test. Pure functions stay unit tested in
groups 4, 5, and 9.

- [x] 15.1 Write `tools/fixturegen/main.go` emitting deterministic fixtures with
      the standard library only: JPEGs at given sizes, and minimal PDFs that
      embed them as `/DCTDecode` image XObjects; verify
      `sudo docker compose exec backend go run ./tools/fixturegen -out /data/tmp/fix`
      writes every file and `pdftoppm` renders each PDF
- [x] 15.2 Generate and commit the fixture set under `e2e/fixtures/`: a 10 page
      PDF, landscape, portrait, mixed, square, and extreme-aspect image sets, a
      60 page PDF, an image smaller than 480 px, a corrupt JPEG, and a truncated
      PDF; verify `pdfinfo` reports the expected page count for each PDF
- [x] 15.3 Create `e2e/` with `package.json`, `playwright.config.ts` pointing at
      `http://frontend:5173`, and `docker-compose.e2e.yml` adding a service on
      the official Playwright image; verify
      `sudo docker compose -f docker-compose.yml -f docker-compose.e2e.yml run --rm e2e npx playwright --version`
      prints a version
- [x] 15.4 Add a reset helper that truncates `book`, `book_page`, and the River
      job table and empties `pages/`, `sources/`, and `tmp/`, called before each
      test file; verify two consecutive full runs both pass
- [x] 15.5 Add an `uploadBook` helper and a `waitForComplete` helper that polls
      `GET /api/books/{id}` until the state leaves `pending` and `processing`;
      verify a smoke test uploads the 10 page PDF and reaches `complete`
- [x] 15.6 `upload.spec.ts` covering `specs/book-upload/spec.md`: PDF accepted,
      image set accepted, mixed rejected, two PDFs rejected, unsupported type
      rejected, no files rejected, name required, whitespace-only name rejected,
      name trimmed, duplicate names allowed, PDF name prefill, edited prefill
      sticks, image set leaves the name blank, selected files listed in send
      order, request parts in that order, oversize upload returns 413; verify
      every scenario in that spec file has a matching test
- [x] 15.7 `ingest.spec.ts` covering `specs/book-ingest/spec.md`: 10 page PDF
      yields 10 ordered pages, image set keeps upload order, files uploaded as
      `b,a,c` stay in that order, two files named `scan.jpg` both become pages,
      three renditions per page at the specified dimensions, no upscaling for a
      small image, original bytes unchanged, source kept, complete book enters
      `dewarp`, a book in progress has no stage, a truncated PDF fails with a
      reason, a corrupt image fails naming the file; verify every scenario in
      that spec file has a matching test
- [x] 15.8 `library.spec.ts` covering the stage and list scenarios in
      `specs/book-library/spec.md`: one Dewarp tab, tab survives reload, unknown
      stage falls back to the first, books listed with name and cover, empty
      state invites upload, a book of another stage is absent, the active area
      sits apart, a book moves out of it on completion without a reload, a
      failed book shows its reason, and a second browser context sees a new book
      live; verify every listed scenario has a matching test
- [x] 15.9 `grid.spec.ts` asserting the rendered cell box, not the API response:
      all-landscape 480x320, all-portrait 320x480, mixed 480x480, a square page
      480x480, extreme aspect clamped to 480x240, single page book 320x480, every
      cell identical, and a small thumbnail centered without cropping
- [x] 15.10 `reader.spec.ts`: selecting page 42 opens the reader with page 42 in
      view, a direct reader URL scrolls to its page, pages stack full width in
      order, back returns to the same book's grid, and opening the 60 page book
      issues far fewer image requests than pages (the same lazy-loading property
      as the spec's 500 page scenario, at a practical fixture size)
- [x] 15.11 `files.spec.ts`: a thumbnail URL returns 200 with an image content
      type, an unknown file returns 404, and a traversal path neither returns a
      file from `sources/` nor lists a directory; verify the suite passes against
      the development stack and against `docker-compose.prod.yml`
- [x] 15.12 Add a root `run-e2e.sh` that builds the stack, waits for health, runs
      the suite, and tears down, and document it in the README; verify it passes
      from a cold start with no containers running
