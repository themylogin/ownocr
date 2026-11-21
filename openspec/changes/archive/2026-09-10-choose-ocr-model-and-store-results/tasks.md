## 1. The last stage becomes OCR

- [x] 1.1 Rename the last stage in `backend/internal/books/stages.go`: the entry
  becomes `{ID: "ocr", Title: "OCR"}`, `StageDone` becomes `StageOCR`, and the
  OCR action leads to it. Verify `go build ./...` passes with no `StageDone`
  left.
- [x] 1.2 Update `backend/internal/books/stages_test.go`,
  `store_test.go` and `split/worker_test.go` for the new last stage. Verify
  `go test ./internal/books/... ./internal/split/...` passes.
- [x] 1.3 Add `UPDATE book SET stage = 'ocr' WHERE stage = 'done'` to the
  migration in task 2.1, in the shape `0007_split_stage.sql` uses. Verify on a
  database holding a complete OCR copy that the copy reports `ocr` and no book
  reports `done`.
- [x] 1.4 Update the stage names in `e2e/tests/ocr.spec.ts`,
  `library.spec.ts`, `dewarp.spec.ts` and `reject.spec.ts`. Verify
  `./run-e2e.sh` passes and the tab row reads "Uploaded", "Dewarped", "Split",
  "OCR".
- [x] 1.5 Confirm the frontend needs no change: it takes the stage list from the
  server, and the `done` in `bookState.ts` is a card state group, not a stage.
  Verify the tab row and the deep links work with no edit to `useStages.ts`.

## 2. The schema

- [x] 2.1 Add `backend/internal/db/migrations/0009_page_ocr.sql`: the `page_ocr`
  table with `page_id` referencing `book_page` on delete cascade, `model`, `txt`
  and `data`, and a unique index on `(page_id, model)`. Verify `\d page_ocr` in
  `psql` shows the cascade and the unique index.
- [x] 2.2 Add `book.ocr_profile` in the same migration, with a check constraint
  that binds it to the action, in the shape `0008_page_cut.sql` uses. Verify the
  check refuses `ocr_profile` on a dewarp copy and refuses an unknown value.
- [x] 2.3 Backfill `ocr_profile = 'vl'` for every book whose action is `ocr`.
  Verify on a database holding a complete OCR copy that the copy reports `vl`
  and every other book reports null.
- [x] 2.4 Add `book_page.image_book_key` in the same migration, nullable, with no
  backfill. Verify every existing page row reports null.
- [x] 2.5 Add `OCRProfile *string` to `books.Book` and `ImageBookKey *string` to
  `books.Page` and `PageOfBook`, and carry them through `bookColumns`,
  `pageColumns`, `pageOfBookColumns`, `Pages`, `FirstPages` and `InsertPage`.
  Verify `go build ./...` passes and the column order still matches
  `RowToStructByPos`.

## 3. Reading a page image from another book

- [x] 3.1 Make `books.Event` build the cover URL from `ImageBookKey` when the
  page has one, and from `book.StorageKey` otherwise. Verify a unit test covers
  both.
- [x] 3.2 Do the same in `httpapi/view.go` `summaryOf` for the cover and
  `pageViewOf` for the three renditions. Verify a test that a page carrying
  `image_book_key` reports URLs under that key and a page without reports URLs
  under its own book.
- [x] 3.3 Add a test asserting that a book with a copy offers neither a page
  rollback nor a re-cut, and name this design in its comment. Verify
  `go test ./internal/httpapi/...` passes. This invariant is what stops a source
  image changing under an OCR copy; see design.md.

## 4. The endpoint contract

- [x] 4.1 Add `Modes []string` to `ocr.JobInput`, and the profile to engine list
  mapping: `v6` gives `["v6"]`, `vl` gives `["v6", "vl"]`. Verify a unit test on
  the mapping.
- [x] 4.2 Change `ocr.OutputPage` to read `output_keys` as a list, and derive the
  engine of an entry from the names it holds. Verify `decodeOutput` on a
  captured two engine response pairs each entry with the right engine.
- [x] 4.3 Replace the page count in `checkOutput` with a count per engine, so a
  copy is complete only when every engine of the profile returned every page.
  Verify tests for a missing page on one engine of two, and for a correct two
  engine result of 12 pages giving 24 entries.
- [x] 4.4 Key the map in `fetchPages` on the input key and the engine together,
  and download every object of an entry. Verify a test that two entries sharing
  one input key both survive.

## 5. The worker

- [x] 5.1 Read the profile of the copy in `ocr.Worker.Work` and put its engines
  in the job input. Verify a worker test that a `v6` copy sends one engine and a
  `vl` copy sends two.
- [x] 5.2 Drop `ingest.WriteRenditions` from `storePage`, and the `RemoveAll` and
  `MkdirAll` of the page directory from `rebuild`. Give the page of the copy the
  storage key, the extension and the dimensions of its source page, and set
  `image_book_key` to the storage key of the source book. Verify a worker test
  that the copy creates no directory and its pages report the source key.
- [x] 5.3 Write one `page_ocr` row per engine per page instead of a markdown
  file, holding the text and the full result of that engine verbatim. Verify a
  worker test that a `vl` copy gives two rows per page and a `v6` copy gives one.
- [x] 5.4 Make a retry replace the rows of the copy. `DeletePages` already
  cascades, so verify a test that a second attempt leaves exactly one row per
  engine per page.
- [x] 5.5 Remove `PageMarkdownPath` and `PageMarkdownURL` from
  `internal/filestore`, and their tests. Verify `go build ./...` and
  `go test ./...` pass with no reference left.

## 6. The action argument

- [x] 6.1 Add an arguments field to `actionRequest` in `httpapi/actions.go`,
  carrying `ocrProfile`. Reject an OCR request that names no profile or an
  unknown one, before any book is touched. Verify handler tests get 422 with a
  reason for both, and that no copy exists after either.
- [x] 6.2 Pass the profile through `startAction` and `CreateDerivedTx` so the
  copy records it. Verify a test that the copy reports the profile the request
  named.
- [x] 6.3 Read the profile off the copy in the worker, so it lives in one place
  and a queued job cannot disagree with its book. Verify a test that a `v6` copy
  asks the endpoint for one engine and a `vl` copy asks for two.
- [x] 6.4 Make `retry` read the profile from the copy, never from the request.
  Verify a test that retrying a `vl` copy queues a `vl` job even when the request
  names `v6`. `StageView.vue:97` already retries by action id alone, so it needs
  no change.
- [x] 6.5 Add `ocrProfile` to `bookSummary` and to `books.Event`. Verify
  `GET /api/books/{id}` reports it for an OCR copy and null for every other book.

## 7. The dialog

- [x] 7.1 Add the arguments to `api.runAction` and its types in
  `frontend/src/api/client.ts`, and `ocrProfile` to `Book`. Verify `vue-tsc`
  reports no error.
- [x] 7.2 Carry the arguments through `run` in
  `frontend/src/composables/useActions.ts`. Verify the existing callers still
  compile with no argument.
- [x] 7.3 Add `OcrDialog.vue` on `AppModal`: the two profiles with their engine
  subtitles, "Normal" preselected, the book count, a start control and a cancel
  control. Verify it opens, preselects Normal, and closes on Escape without
  starting anything.
- [x] 7.4 Open the dialog for the OCR action from `BookActions.vue`, and leave
  every other action starting at once. Verify the book view and the reader open
  it, and Dewarp and Split do not.
- [x] 7.5 Open it from the bulk action menu and the card context menu in
  `StageView.vue` and `BookList.vue`, naming the number of selected books.
  Verify a bulk OCR on three books runs on all three with the chosen profile and
  clears the selection.

## 8. Tests and docs

- [x] 8.1 Teach `backend/tools/fakerunpod` the `modes` field and the per-engine
  output names: `.v6.json` and `.v6.txt` for v6, `.md` and `.json` for vl. Make
  it refuse a job naming no engine. Verify the OCR e2e run passes against it.
- [x] 8.2 Extend `e2e/tests/ocr.spec.ts`: the dialog opens from all three places,
  Cancel starts nothing, a Normal run stores one result per page, and an Extra
  run stores two. Verify `./run-e2e.sh ocr.spec.ts` passes.
- [x] 8.3 Add an e2e check that a page of an OCR copy loads its three renditions
  and its cover, and that the copy holds no directory of its own. Verify the
  images render in the grid and the reader.
- [x] 8.4 Run the whole suite. Verify
  `docker compose -f docker-compose.dev.yml exec backend go test ./...` and
  `./run-e2e.sh` both pass.
- [x] 8.5 Update the README: the two profiles, the `page_ocr` table,
  `ocr_profile`, `image_book_key`, the OCR tab, and the note that an OCR copy
  owns no files. Verify the API table shows the new request field.
- [x] 8.6 Update the `## Purpose` of `openspec/specs/book-ocr/spec.md` at
  archive time. It still names the `done` stage and "one markdown file per
  page", and a Purpose is not carried by a delta. Verify the archived spec names
  the `ocr` stage and the stored results.

## 9. Verify against the real endpoint

- [x] 9.1 Run an Extra OCR on a real book of a few pages against
  `PADDLEOCR_VL_API_URL`. Verify the copy completes, every page holds two rows,
  and the VL row of a page carrying Greek holds the accented text.
- [x] 9.2 Run a Normal OCR on the same book. Verify the copy completes and every
  page holds one row.
