## Context

See `proposal.md` for the motivation.

Three facts of the current system shape everything below.

A book has at most one copy, and `startAction` refuses a second, so a chain is a
line and not a tree. `Chain()` still recurses, and returns a book with every
book derived from it, deepest first.

`book_page` holds `(book_id, page_index, storage_key, original_ext, width,
height)`, unique on `(book_id, page_index)`. Nothing joins a page of a copy to
the page it came from. Both workers write a copy page at the same index as its
source page, so today the only link is equal `page_index`, held by convention.

A copy owns its own storage key and its own three renditions per page. That is
what lets the Split worker say that removing the source book cannot damage the
copy.

## Goals / Non-Goals

**Goals:**

- One lineage model that serves both page operations and every action added
  later, including one that makes two pages from one.
- A rollback flow whose backend and interface name no action. Adding a
  revertible action means adding one flag.
- No new failure mode for the workers.

**Non-Goals:**

- Page events over the WebSocket. A second client sees a deleted page after a
  reload.
- Zoom or pan in the comparison.
- Undoing a rollback. Getting the replaced page back means running the action
  for the whole book again.
- A marker on a page that was rolled back.
- Inserting or reordering pages.

## Decisions

### Page lineage replaces positional matching

Add `book_page.source_page_id`. A page of a copy names the page it was made
from.

Positional matching cannot survive either feature. Deleting a page of a copy
leaves the source book one page longer, so equal `page_index` stops meaning
equal page. Split will make two copy pages from one source page, which breaks
index equality on its own, whatever the delete does.

The column also makes both operations trivial to express: a delete walks the
references down, and a rollback follows one reference up.

Alternative considered: match by index and forbid a delete on a copy. That
keeps the schema still but forbids the obvious operation, and dies at Split
anyway.

### The reference is `ON DELETE SET NULL`, and the delete walks the chain itself

`ON DELETE CASCADE` looks like a free implementation of "delete from every book
derived from this one". Reject already destroys a chain deepest first, so a
cascade would do no harm there.

It is still wrong. `DeletePages(bookID)` runs at the start of every worker
rebuild, so a retry of a copy would reach into the pages of any book derived
from it. That is safe only while the action handler refuses to give a failed
book a copy. Page lifetime must not depend on an invariant enforced in a
different package.

The cascade also buys nothing. The delete needs the descendant rows anyway,
because each page owns three files under the storage key of its own book, and
the file store has no index from a book to its pages. So:

```
WITH RECURSIVE descendants AS (
    SELECT p.id, p.book_id, p.storage_key, p.original_ext, 0 AS depth
    FROM book_page p WHERE p.book_id = $1 AND p.page_index = $2
    UNION ALL
    SELECT c.id, c.book_id, c.storage_key, c.original_ext, d.depth + 1
    FROM book_page c JOIN descendants d ON c.source_page_id = d.id
)
SELECT ... FROM descendants ORDER BY depth DESC
```

Rows first, deepest first, then the files, then the page counts. The same order
`destroy()` uses for a book, and for the same reason.

`SET NULL` keeps one honest end state: a page whose source is gone reads back
with no source page and offers no rollback. That is exactly right, because the
bytes it would install are gone.

### `page_index` becomes immutable

A delete leaves gaps. Nothing renumbers.

Renumbering would have to run across every book of the chain in one
transaction, and a bare `UPDATE book_page SET page_index = page_index - 1 WHERE
page_index > $1` can violate the unique index, because PostgreSQL checks it per
row rather than at the end of the statement. Working around that needs a
deferrable constraint or a two-pass update, for no gain.

Nothing user facing counts on the index. The grid orders by it, the reader
scrolls to it, and no view prints it. So it costs nothing to treat it as an
identity inside a book. It also stops a reader link to page 400 from moving when
someone deletes page 2.

### A rollback copies bytes into a new storage key

The rollback reads the `orig` rendition of the source page, runs
`ingest.WriteRenditions` into the page directory of its own book under a **new**
page key, updates the row, then removes the three files the page held before.
This is the move `split.storePage` already makes.

Pointing the row at the file of the source book would be cheaper and wrong. It
breaks the rule that a copy owns its bytes, and it makes the file cleanup of a
reject unsafe.

The new page key changes the URL, so no client can show a cached image of the
page it replaced. That is worth more than the reuse.

### The rollback capability lives on the action, and reaches the client through the book

`books.Action` gains one field:

```go
type Action struct {
	ID, Title, Stage string
	OnePageEach      bool // one copy page per source page
}
```

Dewarp sets it. Split does not, because splitting a spread makes two pages from
one. The field describes the contract of the action, not the state of its
worker: Split copies pages one for one today, and declaring it revertible now
would have to be taken back later.

`bookDetail` reports one boolean, `pageRollback`, true when the book names an
action that sets the flag, the book is complete, and the book has no copy. The
client reads that alone. The words "Show original" and "Use original" are fixed
in the interface, and the two panes take their labels from stage titles, so the
whole flow names no action anywhere.

Alternative considered: per-action labels for the two menu items. Rejected. The
two words already describe every rollback: the left pane is what the book was
before this action ran.

### A rolled back page is marked, and offers the rollback no more

`book_page.rolled_back` records that a page returned to its source. A page that
carries it already shows the page a rollback would install, so the menu drops
the item and the endpoints refuse.

The mark cannot be derived. After a rollback the page holds a copy of the source
bytes under its own key, and nothing about the row says whether those bytes came
from the worker or from a rollback. Comparing files to find out would be both
slow and fragile.

It needs no reset. Every worker rebuild writes new page rows, which default to
false, so a retry returns the whole copy to its normal state.

The client decides per page rather than per book: the menu shows "Show original"
only when the book allows a rollback, the page still names a source page, and
the page carries no mark. That last pair also removes the case where a page
whose source is gone offered an item that could only fail.

### A rollback needs a book without a copy

A delete cascades down because "this page exists nowhere downstream" holds at
every depth. A rollback does not. Replacing a dewarped page under an existing
split copy would leave the split page derived from bytes that no longer exist,
and pushing the rollback down would undo the split rather than the dewarp.

So the rollback is offered at the end of a chain alone. In practice that is
where it is wanted: a user dewarps a book, reads it, and repairs the pages the
dewarper mangled, before running Split.

### Both operations refuse while the chain holds work in flight

Reject takes the opposite line: it destroys a chain mid-job, and the workers
absorb it through `errors.Is(err, books.ErrNotFound)`. That works because
removing the whole target of a job is one cheap check at the top of `Work`.

A page delete is a different hazard. It changes the input under a running job
instead of removing it. The Dewarp worker snapshots `sourcePages`, uploads the
originals, then inserts copy pages minutes later; a page deleted in between
leaves it inserting a page whose source row is gone. Absorbing that means every
worker re-checks every source page at insert time, forever, and every action
added later inherits the obligation.

Refusing costs one query. The check is: no book returned by `Chain(bookID)` is
in state `pending` or `processing`. Ancestors are always complete, because a
book that is not complete cannot take an action, so the chain from this book
down is the whole exposure.

The handler runs the check inside the transaction, after `SELECT ... FOR UPDATE`
on every book of the chain, in ascending id order. `startAction` already locks
its source book the same way, so an action that starts concurrently either waits
for the delete or is seen by it.

### The API mirrors reject

```
GET  /api/books/{id}/pages/{index}/deletion  -> { books: [...], allowed, reason }
POST /api/books/{id}/pages/{index}/delete
GET  /api/books/{id}/pages/{index}/rollback  -> { source: {...}, current: {...} }
POST /api/books/{id}/pages/{index}/rollback
```

The preview-then-apply pair is the shape `rejection` and `reject` already use,
and `RejectDialog.vue` already renders a preview with a loading state, an error
state and a danger button. Both dialogs follow it.

`GET .../deletion` reports the refusal rather than answering `409`, so the
confirmation can name the reason instead of the button failing under the user.

The comparison is lazy. `bookDetail` carries one boolean and no source URLs. A
500 page book does not need 500 extra URLs so that one modal can open without a
request.

### The comparison sizes itself

```
paneW = (innerWidth  - 2*margin - gap) / 2
paneH =  innerHeight - 2*margin - header - footer
scale =  min(paneW / w, paneH / h, 1)   over both pages
```

Both panes take the smaller scale, so the two pages stay comparable. The modal
then shrinks to the size the scaled pages and the controls need, so no dead
space is left. A `resize` listener recomputes it. The `1` in the minimum keeps
the rule the reader already follows: never enlarge a page beyond its stored
rendition.

### The confirmation is a second modal, and the comparison must not close itself

Naive-ui stacks modals correctly: `zindexable` takes the next z-index from a
global counter on mount, so a modal opened later sits above one opened earlier.

The trap is `mask-click`. `Modal.mjs` calls `onMaskClick` for **any** click
outside that modal's own body, and only its built-in close path checks that the
click landed on the mask:

```js
function handleClickoutside(e) {
  const { onMaskClick } = props
  if (onMaskClick) onMaskClick(e)                  // any outside click
  if (props.maskClosable) {
    if (containerRef.value?.contains(getPreciseEventTarget(e))) doUpdateShow(false)
  }
}
```

So a modal that wires `@mask-click` straight to close closes itself on every
click inside a modal above it. It fires on mousedown, so the button under the
pointer never gets its click either. `@esc` has the same shape: naive-ui calls
`props.onEsc` before the `eventEffectNotPerformed` guard that gives the key to
the topmost modal alone.

Documenting that in each dialog would leave the trap in place. `AppModal.vue`
takes the one correct wiring instead: every dialog renders through it, closes
through `update:show`, and passes `busy` for the two guards. `UploadDialog`,
`RejectDialog`, `PageDeleteDialog` and both modals of `PageCompare` use it, and
no `n-modal` is left outside it. The comparison passes `sending || confirming`,
so while the confirmation is up the mask and Escape reach that modal alone.

### One menu component serves the grid and the reader

`PageMenu.vue` holds the dropdown, both dialogs and the comparison. `BookView`
and `ReaderView` mount it and hand it the page under the pointer. This is the
`BookList.vue` pattern: close for a tick, then reopen at the new coordinates, so
a right click on another page moves the menu.

Both views already load a `BookDetail` once and hold it in a ref. A delete
splices the page out of that array; a rollback replaces the page object with the
one the answer returns. Neither view reloads.

## Risks / Trade-offs

[The backfill mismatches a page] → It cannot, before the first delete ships:
every existing copy was written index for index by a worker that is still in the
tree. The migration matches on equal `page_index` within `source_book_id`, and
runs before any code can create a gap.

[The migration cannot be rolled back after a delete] → Dropping the column
returns the system to positional matching, which a gap breaks. The rollback
window closes the first time a user deletes a page. Accept it: the column is
additive and the code that fills it ships in the same release.

[A user must re-run the whole action to undo a rollback] → Stated in the
confirmation, and the reason the second confirmation exists at all. Keeping the
replaced renditions to allow an undo would double the storage of every page a
user touches.

[Deleting the last page leaves a book with no pages] → Allowed. `BookCard.vue`
already renders a complete book with no cover through its `.nocover` branch, and
`ComputeGridCell` must return a usable shape for an empty page list.

[The refusal surprises a user whose copy is running] → The confirmation names
the reason before the user commits, and a failed copy never blocks. The user
waits for the job or retries afterwards.

[Two clients delete the same page at once] → The second finds no row and gets
`404`. The row lock serializes them.

## Migration Plan

1. `0005_page_lineage.sql` adds the column, the index, and the backfill:

```sql
ALTER TABLE book_page ADD COLUMN IF NOT EXISTS source_page_id BIGINT
    REFERENCES book_page (id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS book_page_source_idx ON book_page (source_page_id);

UPDATE book_page c SET source_page_id = p.id
FROM book b JOIN book_page p ON p.book_id = b.source_book_id
WHERE c.book_id = b.id AND c.source_page_id IS NULL
  AND p.page_index = c.page_index;
```

2. The workers record the reference for every page they write from then on.
3. The page routes ship in the same release, so no build can create a gap before
   the column exists.

The server applies pending migrations at startup, so the deploy is the usual
one.
