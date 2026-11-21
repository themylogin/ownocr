## Why

A book carries every page it was uploaded with, good or bad. A blank scan, a
duplicate shot, or a cover sheet stays in the book through every stage, and a
user cannot remove it.

Dewarping also fails on single pages while the rest of the book comes out well.
The only remedy today rejects the whole copy and runs the action again, which
loses every page that came out right.

Both problems ask for the same missing thing: an operation that acts on one
page instead of one book.

## What Changes

- Add a page context menu to the page grid and to the reader. A right click on
  a page opens it at the pointer.
- Add a Delete item to that menu. Delete removes the page from this book and
  from every book derived from it. The system asks for confirmation first and
  names every book that loses a page.
- Add a page lineage column, `book_page.source_page_id`. A page of a copy names
  the page it came from. The column replaces the positional match by equal
  `page_index`, which no longer holds once a page is deleted.
- **BREAKING** `page_index` keeps gaps. A delete never renumbers the pages that
  follow it. The index becomes a stable ordering key, not a position.
- Add an abstract page rollback. A page whose book took an action can return to
  the page it came from. Dewarp offers it. The flow serves every later action
  that maps one source page to one copy page.
- Add "Show original" to the menu of such a page. It opens a modal that shows
  the source page and the current page side by side, sized to use the screen.
- Add "Use original" to that modal, in the danger colour. It asks for
  confirmation, then replaces the page of the copy with the page of the source
  book.
- Refuse both operations while any book of the chain is `pending` or
  `processing`. Both change the input of a running job.
- Offer the rollback only while the book has no copy. It undoes exactly one
  stage, so it cannot run under a copy that already consumed the page.

## Capabilities

### New Capabilities

- `book-pages`: page lineage, page deletion down the chain, and the abstract
  rollback of one page to its source page. Holds the four page endpoints and
  the rules that guard them.

### Modified Capabilities

- `book-library`: the page grid and the reader gain a page context menu, two
  confirmations, and the side by side comparison modal.
- `book-stages`: an action declares whether it maps one source page to one copy
  page, which is what makes a page rollback possible. A copy records the source
  page of every page it writes.

## Impact

- Database: migration `0005_page_lineage.sql` adds `book_page.source_page_id`
  with `ON DELETE SET NULL`, an index on it, and a backfill that matches the
  existing pages by equal `page_index`.
- Backend: `internal/books` gains page lineage reads, the descendant walk, and
  the page writes. `internal/books/stages.go` gains the per-action flag.
  `internal/httpapi` gains `pages.go` with four routes. `internal/dewarp` and
  `internal/split` record the source page of every page they insert.
- API: four new routes under `/api/books/{id}/pages/{index}`. `pageView` gains
  the fields the menu and the modal need. No existing route changes.
- Frontend: new `PageMenu.vue`, `PageDeleteDialog.vue`, and `PageCompare.vue`.
  `BookView.vue` and `ReaderView.vue` mount the menu. `api/client.ts` gains the
  four calls.
- Tests: a new e2e spec for the page menu, both confirmations, and the
  comparison modal. Go tests for the descendant walk and the refusal rules.
- Docs: the README gains the page operations and the new routes.
