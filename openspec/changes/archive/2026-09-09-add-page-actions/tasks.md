## 1. Page lineage

- [x] 1.1 Add `backend/internal/db/migrations/0005_page_lineage.sql` with the
  column, the index and the backfill from `design.md`. Verify the dev server
  starts and `\d book_page` in `psql` shows `source_page_id` and
  `book_page_source_idx`.
- [x] 1.2 Add `SourcePageID *int64` to `books.Page` and carry it through
  `InsertPage`, `Pages` and `FirstPages`. Verify `go build ./...` passes and the
  column order in every query still matches `RowToStructByPos`.
- [x] 1.3 Record the source page in the Dewarp worker for every page it writes.
  Verify by dewarping a fixture book in the dev stack and checking that every
  copy page names a source page.
- [x] 1.4 Record the source page in the Split worker for every page it writes.
  Verify the same way after a Split.
- [x] 1.5 Add a store test that a book uploaded with no source names no source
  page, and that a backfilled copy names one page each. Verify
  `go test ./internal/books/...` passes.

## 2. Page rules in the store

- [x] 2.1 Add `PageDescendants(ctx, bookID, index)` using the recursive query in
  `design.md`, returning the page rows deepest first with their book keys.
  Verify a test over a three book chain returns three rows in chain order.
- [x] 2.2 Add `SourcePage(ctx, page)` that returns the page a page was made
  from, and reports absence when the reference is NULL. Verify a unit test
  covers both answers.
- [x] 2.3 Add `DeletePage(ctx, id)` and `ReplacePageFile(ctx, id, key, ext, w,
  h)` writes. Verify a test that a replaced page keeps its index and its source
  page reference.
- [x] 2.4 Add `LockChainTx(ctx, tx, bookID)` that locks every book of the chain
  in ascending id order. Verify a test that two concurrent transactions
  serialize instead of both proceeding.
- [x] 2.5 Add `OnePageEach` to `books.Action`, set it on Dewarp and leave it off
  Split. Verify `go test ./internal/books/...` passes with a test that asserts
  both.

## 3. Page endpoints

- [x] 3.1 Add `internal/httpapi/pages.go` with the four routes from `design.md`,
  registered inside the 15 second timeout group. Verify `curl` reaches each and
  an unknown page index answers `404`.
- [x] 3.2 Implement `GET .../deletion`: the affected books with their stages,
  plus `allowed` and `reason` from the in-flight check. Verify the answer names
  two books for a book with a dewarped copy.
- [x] 3.3 Implement `POST .../delete`: lock the chain, re-check the states, walk
  the descendants, delete rows deepest first, remove the files, reduce every
  page count, broadcast `book.updated` per changed book. Verify a delete on a
  two book chain removes both pages and six files.
- [x] 3.4 Implement `GET .../rollback`: the source page and the current page,
  each with its display URL, width, height and stage title. Verify a page that
  offers no rollback is rejected with a reason.
- [x] 3.5 Implement `POST .../rollback`: check every condition from the spec,
  write the renditions under a new page key, update the row, remove the old
  files, broadcast the book. Verify the page reads back with a new URL and the
  same index.
- [x] 3.6 Add `pageRollback` to `bookDetail` and `sourcePageId` to `pageView`.
  Verify `GET /api/books/{id}` reports true for a complete dewarped copy with no
  copy of its own, and false for an uploaded book.
- [x] 3.7 Add handler tests for both refusals: a chain that holds a running
  book, and a rollback on a book that already has a copy. Verify
  `go test ./internal/httpapi/...` passes.

## 4. Client and page menu

- [x] 4.1 Add the four calls and their types to `frontend/src/api/client.ts`.
  Verify `vue-tsc` reports no error.
- [x] 4.2 Add `PageMenu.vue`: a pointer anchored dropdown holding "Show
  original" when the book offers a rollback, a divider, then "Delete" in the
  danger colour. Verify a right click in the grid opens it and Escape closes it.
- [x] 4.3 Mount `PageMenu` in `BookView.vue` and pass the page under the
  pointer. Verify the menu opens on any thumbnail and moves to another
  thumbnail on a second right click.
- [x] 4.4 Mount `PageMenu` in `ReaderView.vue`. Verify the same menu opens on a
  page in the reader.

## 5. Delete confirmation

- [x] 5.1 Add `PageDeleteDialog.vue` on the `RejectDialog.vue` pattern: a
  loading state, the book list with stages, the "cannot be undone" line, and a
  danger confirm. Verify the dialog names two books for a book with a copy.
- [x] 5.2 Show the refusal reason and disable the confirm when the preview
  reports `allowed` false. Verify the dialog refuses while a copy runs.
- [x] 5.3 Splice the deleted page out of the held `BookDetail` in both views.
  Verify the thumbnail leaves the grid and the page leaves the reader stack,
  with no reload.

## 6. Comparison and rollback

- [x] 6.1 Add `PageCompare.vue` with the two panes, their stage titles, and the
  sizing rule from `design.md`, recomputed on `resize`. Verify both pages fill
  the window and neither is enlarged past its display rendition.
- [x] 6.2 Add the bottom controls: "Use original" in the danger colour, then
  "Close". Verify "Close" changes nothing.
- [x] 6.3 Add the second confirmation, naming that the step cannot be undone and
  that the current page returns only by running the action again. Verify
  dismissing it leaves the comparison open and the page unchanged.
- [x] 6.4 Replace the page object in the held `BookDetail` from the answer.
  Verify the grid shows the new image without a reload.

## 7. Tests and docs

- [x] 7.1 Add `e2e/tests/pages.spec.ts` covering the menu in the grid and the
  reader, both confirmations, a delete down a two book chain, and the refusal
  while a copy runs. Verify `./run-e2e.sh pages.spec.ts` passes.
- [x] 7.2 Extend the e2e coverage of the comparison: the fake endpoint shrinks
  every page, so the two panes differ in size and a rollback is observable.
  Verify the rolled back page matches the uploaded page.
- [x] 7.3 Run the whole suite. Verify
  `docker compose -f docker-compose.dev.yml exec backend go test ./...` and
  `./run-e2e.sh` both pass.
- [x] 7.4 Update the README: the page operations, the four routes, and the note
  that a page index keeps its gaps. Verify the API table lists all four.

## 8. A rolled back page, and the stacked confirmation

- [x] 8.1 Add `0006_page_rolled_back.sql` with `book_page.rolled_back`. Verify
  `\d book_page` shows the column with default false.
- [x] 8.2 Carry `RolledBack` through `books.Page` and `PageOfBook`, and set it in
  the rollback write. Verify the store test asserts the mark before and after.
- [x] 8.3 Refuse a rollback on a page that carries the mark, on both the preview
  and the apply. Verify the handler test gets 422 twice with a reason.
- [x] 8.4 Report `rolledBack` with every page. Verify `GET /api/books/{id}`
  marks the rolled back page alone.
- [x] 8.5 Decide the menu per page: show "Show original" only when the book
  allows a rollback, the page names a source page, and it carries no mark.
  Verify the e2e test that the item leaves that page's menu and stays on its
  neighbours, before and after a reload.
- [x] 8.6 Restore the confirmation as a second modal over the comparison, closing
  through `update:show` instead of `mask-click`. Verify the e2e test that both
  are on screen and the confirmation has the higher z-index.
- [x] 8.7 Add `AppModal.vue` holding the one correct close wiring, and render
  every dialog through it: `UploadDialog`, `RejectDialog`, `PageDeleteDialog`
  and both modals of `PageCompare`. Verify no `n-modal` is left outside it and
  the whole e2e suite passes.
