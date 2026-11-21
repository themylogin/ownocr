## Why

The Split action will one day cut a spread into single pages. That makes several
copy pages from one source page. The worker today copies pages one for one, and
the code around it assumes that ratio. A copy page reuses the index of its source
page, which the unique index on `(book_id, page_index)` rejects as soon as one
source page makes two. The delete confirmation counts pages instead of books, so
a split copy appears twice in the list.

Nothing exercises the one-to-many path today, so these faults stay hidden. An
interim algorithm that writes two pages for every source page makes them
reproducible now, before the real algorithm arrives.

## What Changes

- The split worker gives the copy its own page indexes, counted over the pages it
  writes, not copied from the source page.
- The split worker reports the page count of the copy, not the page count of the
  source book.
- **BREAKING** The interim split algorithm writes two pages for every source
  page. A split copy holds twice the pages of its source book. This stands in for
  the real algorithm and exercises the one-to-many path end to end.
- The answer that says what a page delete would remove names each book once. It
  named a book once for every page it would lose.
- The delete response names each changed book once, for the same reason.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `book-split`: the copy holds one or more pages for every source page. The
  requirement states one page for every source page today.
- `book-pages`: the delete answer and the delete response name each book once,
  however many pages that book loses.

## Impact

- `backend/internal/split/worker.go`: page index, page count, progress.
- `backend/internal/httpapi/pages.go`: `pageDeletion` and `removePage` dedupe the
  books they name.
- `e2e/tests/split.spec.ts`: the test `a book is split` asserts a one-for-one
  copy.
- `e2e/tests/pages.spec.ts`: gains a delete test that runs through a split copy.
- A split book takes twice the file store of its source book. This is acceptable
  while the interim algorithm stands.

No change to the database schema, the HTTP routes, or the frontend. Per-page
rollback already refuses a split copy through `Action.OnePageEach`, and needs no
change.
