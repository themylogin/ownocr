## 1. The split worker writes several pages from one

- [x] 1.1 Give `storePage` the page index as an argument instead of reading
  `source.PageIndex`, and verify `go build ./...` succeeds
- [x] 1.2 Count the pages the worker writes in `rebuild`, use that counter as the
  page index, and return it, so a retry writes a dense index set with no
  duplicate; verify the split worker test passes
- [x] 1.3 Write two pages for every source page, each with its own storage key
  and its own renditions, and verify the two pages hold different files
- [x] 1.4 Pass the written count to `SetComplete` instead of `len(sourcePages)`,
  and verify a split copy of a 3 page book reports 6 pages
- [x] 1.5 Drop the `total` argument and the `done != total` guard from the split
  `reportProgress`, leaving the interval guard alone; verify progress still
  updates and `SetComplete` writes the final count. Leave `ingest` and `dewarp`
  untouched: both stay one page for one page

## 2. The delete answer names each book once

- [x] 2.1 Dedupe by book id in `pageDeletion` while keeping chain order, and
  verify the confirmation lists a split copy once
- [x] 2.2 Dedupe by book id in the `changed` list of `removePage`, and verify the
  response names each book once and publishes one `book.updated` per book
- [x] 2.3 Confirm the per-book recount still runs once per book and reports the
  right count after a copy loses two pages from one

## 3. Tests

- [x] 3.1 Update `a book is split` in `e2e/tests/split.spec.ts`: the copy holds
  twice the pages, indexes run 1..2n with no gap, and the two pages made from one
  source page carry the dimensions of that source page
- [x] 3.2 Add an e2e test that deletes a page of a book with a split copy: the
  dialog says `2 books`, the list holds 2 entries, and the copy reports two pages
  fewer
- [x] 3.3 Verify `a split copy offers no rollback` in `e2e/tests/pages.spec.ts`
  still passes, so `Action.OnePageEach` keeps the rollback off a split copy
- [x] 3.4 Run the backend unit tests and the full e2e suite, and verify the
  remaining split and pages tests pass unchanged
