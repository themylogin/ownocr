## 1. The recorded cut

- [x] 1.1 Add `backend/internal/db/migrations/0008_page_cut.sql` with
  `cut_side`, `cut_column`, the check that holds the pair together, and the
  backfill from `design.md`. Verify `\d book_page` in `psql` shows both columns
  and the constraint.
- [x] 1.2 Verify the backfill on real data: split a fixture book of spreads
  before the migration, then apply it, and check that every pair reports the
  left width and `source width - right width`.
- [x] 1.3 Add `CutSide *string` and `CutColumn *int` to `books.Page` and
  `PageOfBook`, and carry them through `pageColumns`, `pageOfBookColumns`,
  `Pages`, `FirstPages` and `InsertPage`. Verify `go build ./...` passes and the
  column order still matches `RowToStructByPos`.
- [x] 1.4 Record the side and the column in the split worker for every page it
  cuts, and leave both empty for a page it copies whole. Verify a worker test
  that a spread gives one left and one right page, and a cover gives neither.
- [x] 1.5 Add a store test that a re-cut write keeps the index, the source page
  and the side. Verify `go test ./internal/books/...` passes.

## 2. The re-cut endpoints

- [x] 2.1 Add `ReCutPageTx` to `internal/books`, updating the storage key, the
  extension, the dimensions and `cut_column` alone. Verify a test that the
  index, `source_page_id`, `cut_side` and `rolled_back` survive the write.
- [x] 2.2 Add `cutTarget` to `internal/httpapi/pages.go`, refusing a page that
  records no cut, a page whose source page is gone, and a book that has a copy.
  Verify a handler test gets 422 with a reason for each.
- [x] 2.3 Implement `GET /api/books/{id}/pages/{index}/cut`: the display URL,
  width and height of the source page, plus the side and the current column.
  Verify `curl` on a split page answers all five fields.
- [x] 2.4 Implement `POST` of the same path: validate the column against the
  source width and the side, lock the chain, check `inFlight` again, crop the
  source original, write the renditions, update the row, remove the replaced
  files, broadcast `book.updated`. Verify the page reads back with a new URL,
  the same index and the new width.
- [x] 2.5 Refuse a column that leaves the page with no column, and a column
  outside the source image. Verify handler tests get 422 for column 0 on a left
  page and for a column past the width.
- [x] 2.6 Refuse the re-cut while the book is `pending` or `processing`. Verify
  a handler test gets 409 and the page keeps its bytes.
- [x] 2.7 Register both routes inside the 15 second timeout group, beside the
  rollback pair. Verify an unknown page index answers 404 on both.
- [x] 2.8 Add `cutSide` to `pageView` and `pageCut` to `bookDetail`. Verify
  `GET /api/books/{id}` reports the side for a cut page, null for a page copied
  whole, and `pageCut` false once the book has a copy.

## 3. The client and the menu

- [x] 3.1 Add the two calls and their types to `frontend/src/api/client.ts`, and
  the new fields on `Page` and `BookDetail`. Verify `vue-tsc` reports no error.
- [x] 3.2 Add "Edit" to `PageMenu.vue`, first, above the divider and Delete, for
  a page that records a cut in a book that offers the re-cut. Verify the item
  appears on a cut page and is absent on a page the split copied whole.
- [x] 3.3 Pass the new book flag from `BookView.vue` and `ReaderView.vue`.
  Verify the same menu opens in the grid and in the reader.

## 4. The cut editor

- [x] 4.1 Add `PageCutDialog.vue` on `AppModal`, loading the cut and showing the
  parent image at the scale `PageCompare.vue` computes, refreshed on `resize`.
  Verify the image fills the window and is never enlarged past its display
  rendition.
- [x] 4.2 Add the grey layer over the part this page leaves out, at half
  transparency, and the plain part over what it holds. Verify the left page
  greys the right side of the spread and the right page greys the left side.
- [x] 4.3 Add the bar and its handle, dragged with pointer events, clamped so
  the page keeps at least one column. Verify the bar follows the pointer and
  stops at both ends.
- [x] 4.4 Add "Save" and "Cancel". Cancel closes and changes nothing. Save sends
  the column in source columns and replaces the page in the held `BookDetail`.
  Verify the grid shows the new image with no reload.
- [x] 4.5 Show a refusal in the modal and keep it open. Verify the reason
  appears when the book gains a copy while the modal is open.

## 5. Tests and docs

- [x] 5.1 Add `e2e/tests/cut.spec.ts` covering the menu item on a cut page and
  its absence on a whole page, the two overlays, Cancel, and a Save that widens
  a page. Verify `./run-e2e.sh cut.spec.ts` passes.
- [x] 5.2 Extend that spec: the neighbour page keeps its image after a save, and
  a second edit that cuts back out restores the columns the first one dropped.
  Verify both by the stored width of the page.
- [x] 5.3 Run the whole suite. Verify
  `docker compose -f docker-compose.dev.yml exec backend go test ./...` and
  `./run-e2e.sh` both pass.
- [x] 5.4 Update the README: the re-cut in "Page operations", the two routes in
  the API table, and the two columns in the Split flow. Verify the table lists
  both new paths.
