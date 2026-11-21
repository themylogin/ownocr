## Context

See `proposal.md` for motivation.

The parts this change touches:

- `backend/internal/books`: the store, the stage list, the event shape. `stage`
  is nullable and only a complete book has one. `processed` is derived with
  `EXISTS (SELECT 1 FROM book derived WHERE derived.source_book_id = book.id)`.
- `backend/internal/httpapi`: `GET /api/books` answers three shapes, chosen by
  query string. `POST /api/books/{id}/dewarp` is the only action.
- `backend/internal/ingest` and `backend/internal/dewarp`: River workers. Each
  reports progress by writing `page_count` every ten pages.
- `frontend/src/views/StageView.vue`: 304 lines that hold two lists, the context
  menu, and the card markup.
- `e2e/tests`: seven Playwright files. `helpers.ts` posts to the dewarp
  endpoint and reads `processed`.

Constraints:

- One Postgres database, migrated by `db.Migrate` on start. Migrations run in
  file order and never run twice.
- River owns the job queues. A queue with `MaxWorkers: 1` runs one book at a
  time.
- The frontend has one WebSocket. Every view applies the same `book.updated`
  event.
- naive-ui supplies the components. The project uses no icon package.

## Goals / Non-Goals

**Goals:**

- One list component that every stage tab reuses, and that the book grid layout
  mirrors.
- One server-side registry that answers "which actions does this stage offer",
  so a new stage or action needs no client change.
- A phase text that is cheap to report and that survives a reconnect.

**Non-Goals:**

- Real splitting. The split worker copies pages; the page-cutting algorithm is
  a later change.
- More than one copy per book, or a branching pipeline.
- Undoing an action in place. A reject destroys the copy instead, and the source
  book takes its action again from the start.
- A progress bar or a page counter. The phase text carries no numbers.

## Decisions

### A stage names a state of the book; an action names the work

`books.Stages` becomes `uploaded`, `dewarped`, `done`. A second registry names
the actions:

```go
type Action struct {
    ID    string // dewarp
    Title string // Dewarp
    Stage string // the stage the copy carries: dewarped
}
```

`ActionsFor(stage)` returns every action whose `Stage` sits after `stage` in the
stage order. That one function answers the context menu, the bulk menu, and the
book view buttons. Adding a stage plus its action needs no client change.

*Alternative rejected*: keep the action name equal to the current stage name, as
today. It reads well with two stages and breaks the moment a stage offers two
actions, which `Uploaded` now does.

### `stage` is set at creation and never cleared

The column becomes `NOT NULL`. An upload writes `uploaded`. An action writes the
stage of that action. `SetProcessing`, `SetComplete` and the requeue path stop
touching `stage`.

This is what puts a running book in exactly one tab, and it deletes three code
paths: `ListActive`, the `state=active` query, and the separate "Processing"
card in `StageView`.

`SetComplete(id, stage, pageCount)` loses its stage argument.

*Alternative rejected*: keep `stage` null while a book runs and add a second
`target_stage` column. Two columns that must agree is one more thing to keep in
step, and every read would have to coalesce them.

### One endpoint for every action and any number of books

```
POST /api/books/actions/{action}
{"bookIds": [12, 13, 14]}

200 {"started": [<summary>, ...], "failed": [{"bookId": 13, "error": "..."}]}
404 when {action} is not a known action
```

Each book runs in its own transaction, with the same lock, checks and job insert
that `startDewarp` uses today. One rejected book does not roll back the others.
The single-book context menu posts a list of one. A retry posts the source book
id of the failed copy under the copy's own action.

*Alternative rejected*: `POST /api/books/{id}/actions/{action}` and let the
client loop. Thirty right clicks become thirty requests, thirty error toasts,
and no way to report "four started, one refused" as one answer.

### The book summary reports the copy state, not a boolean

```
processed: bool          ->  derivedState: "pending"|"processing"|"complete"|"failed"|null
                             progress:     string|null
                             action:       string|null
stage: string|null       ->  stage: string
```

`derivedState` comes from a lateral join instead of the `EXISTS` sub-select:

```sql
LEFT JOIN LATERAL (
    SELECT state FROM book d WHERE d.source_book_id = book.id ORDER BY d.id LIMIT 1
) derived ON true
```

It stays derived, so the mark can never drift from the copy. `action` records
which action made this book, so a failed copy knows what to retry. Both the REST
summary and the WebSocket event carry the same fields, as today.

### The list order lives in SQL and in one client comparator

```sql
ORDER BY CASE
    WHEN book.state = 'failed'                      THEN 0
    WHEN book.state IN ('pending', 'processing')    THEN 1
    WHEN derived.state IS NULL                      THEN 2
    WHEN derived.state = 'complete'                 THEN 4
    ELSE 3
END, book.created_at DESC, book.id DESC
```

The client re-sorts with the same key after every event, instead of the current
`place()` logic that pins a card to its index. The key does not change on a
phase update, so a stable sort leaves the card where it is; it changes only when
the book or its copy changes state, which is exactly when the card should move.

### The phase is a plain column, written by the worker

`book.progress TEXT`. The six phrases are constants in `books`. The enqueue path
writes "Waiting in local queue". Each worker writes its phase at the start of
each part of its job, and each write publishes `book.updated`. `SetComplete` and
`SetFailed` clear the column.

That is at most six extra row writes and six extra broadcasts per book. A
reconnecting client reads the current phase from `GET /api/books`, so no phase
is lost with the socket.

*Alternative rejected*: derive the phase from the River job state. River knows
that a job waits, not that a worker is uploading, and it cannot see the RunPod
queue at all.

### The dewarp worker downloads in one pass and converts in another

Today the download loop fetches one page, writes its renditions, and deletes the
temporary file. That interleaving cannot report "Downloading" and then
"Converting" without flipping between them per page.

The loop splits in two: fetch every output object into the job temp directory,
then walk the directory and write the renditions. The RunPod poll loop gains a
callback that maps `IN_QUEUE` to "Waiting for AI" and `IN_PROGRESS` to
"Processing with AI"; `Wait` already notices every status change.

### The split worker reuses the ingest rendition writer

`Work` reads the source pages, and for each one calls
`ingest.WriteRenditions(originalPath, pageDir, newKey)` — the same call the
dewarp worker makes. The copy gets its own three renditions per page and its own
storage key, so deleting the source book cannot damage it. Split gets its own
River queue with `MaxWorkers: 1`, next to `ingest` and `dewarp`.

*Alternative rejected*: hard-link or share the page files. It is faster and it
makes the copy's files disappear when the source book is deleted.

### The frontend splits into a list, a card, and a selection store

- `components/BookList.vue` — takes `books` and `actions`, sorts, renders the
  grid, owns the context menu. Every tab uses it.
- `components/BookCard.vue` — one card. Picks its own shape from the book state
  and `derivedState`: cover image or icon box, checkbox or mark, phase or reason.
- `components/BookMark.vue`, `components/Breadcrumbs.vue` — the corner mark and
  the trail.
- `composables/useSelection.ts` — a module-level `Set<number>`, like
  `useStages`. `App.vue` owns the bar buttons, `BookList` owns the checkboxes,
  and both read the same set. A stage change clears it.
- `useStages` grows `actionsFor(stage)`, filled from the stage list the server
  already returns.

The three icons are inline SVG in the components. naive-ui's `NSpin` animates
and cannot be stopped cleanly, so the spinner is a static SVG with no
`animation` property. That keeps the dependency list unchanged.

### The page grid takes its width from the container and its shape from the server

`grid-template-columns: repeat(5, 1fr)` with `aspect-ratio` on the cell,
computed from the `gridCell` the server already sends. The 480 px number in
`ComputeGridCell` stops being a pixel size and becomes the shape. No backend
change, and `grid_test.go` still passes.

The reader drops the `100vw` breakout. `max-width: 100%` with the intrinsic
`width` and `height` attributes fits a page to the column and never enlarges it.
The `figcaption` goes.

### Reject destroys a chain, from one endpoint

`GET /api/books/{id}/rejection` answers the book and every copy made from it,
collected by a recursive CTE over `source_book_id` and ordered deepest first.
The confirmation names that list before anything is destroyed.
`POST /api/books/{id}/reject` walks the same list, deleting the row first and
the files second: a leftover file is waste, while a row that points at missing
files is a broken book. Each destroyed book is announced as `book.deleted`, and
the freed source follows as `book.updated`, so a source card returns to the
ready group in every open tab.

A worker whose book was rejected mid-flight now returns without an error. Its
row is gone, so there is nothing to fail and nothing to retry.

*Alternative rejected*: refuse to reject a book that has a copy. It is safer,
but it forces a user to walk a chain backwards to undo one mistake.

### One control set serves the book view and the reader

`BookActions` holds the action buttons, the copy mark and Reject, and both views
mount it. The reader's bar is already sticky, so the same component reaches
every page of a long book without repeating anything down the stack.

## Risks / Trade-offs

- **Peak temp disk grows by one dewarped book.** The two-pass download holds
  every output page before it converts any. → The directory is under
  `data/tmp`, is removed when the job ends, and holds no more than the copy's
  own pages, which already fit next to them under `data/pages`.
- **Split doubles the stored bytes of a book.** → It is the price of the copy
  model: a copy that owns its files survives the deletion of its source. A real
  split will write new pages anyway.
- **The stage rename is not reversible by re-running migrations.** → The
  reverse SQL is short and goes in the task list. The rename touches one column
  of one table.
- **A bulk action on thirty books queues thirty jobs on a one-worker queue.**
  → Already true for thirty right clicks. The list shows every queued book with
  "Waiting in local queue", so the wait is visible instead of silent.
- **Dropping `processed` and `/dewarp` breaks any client outside this repo.**
  → Only `frontend/` and `e2e/` read them. Both change in this task list.
- **A reject cannot be undone.** → The confirmation names every book it will
  destroy and says so. Nothing else in the app deletes a book.
- **Six extra broadcasts per book.** → Each carries one small summary, and the
  hub already broadcasts once per ten stored pages.

## Migration Plan

One new migration file, `0004_stages.sql`:

```sql
ALTER TABLE book ADD COLUMN IF NOT EXISTS progress TEXT;
ALTER TABLE book ADD COLUMN IF NOT EXISTS action TEXT;

UPDATE book SET action = 'dewarp' WHERE source_book_id IS NOT NULL;

UPDATE book SET stage = 'uploaded' WHERE stage = 'dewarp';
UPDATE book SET stage = 'dewarped' WHERE stage = 'split';
UPDATE book SET stage = 'dewarped' WHERE stage IS NULL AND source_book_id IS NOT NULL;
UPDATE book SET stage = 'uploaded' WHERE stage IS NULL;

ALTER TABLE book ALTER COLUMN stage SET NOT NULL;
ALTER TABLE book ALTER COLUMN stage SET DEFAULT 'uploaded';

DROP INDEX IF EXISTS book_state_idx;
DROP INDEX IF EXISTS book_stage_idx;
CREATE INDEX IF NOT EXISTS book_stage_order_idx ON book (stage, created_at DESC);
```

A book that failed before the migration keeps its null stage until the second
`UPDATE ... WHERE stage IS NULL` pair, which sends a failed copy to `dewarped`
and a failed upload to `uploaded`. That is where each belongs under the new
rules.

Deploy is one step: the server migrates on start and the new frontend ships in
the same image. Rollback needs the reverse rename, `dewarped -> split` then
`uploaded -> dewarp`, plus dropping the not-null constraint. The two new columns
are additive and can stay.
