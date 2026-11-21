## Context

See `proposal.md` — Why.

The split worker asks the sidecar for two columns per spread, cuts the left page
at `[0, left)` and the right page at `[right, width)`, and writes three
renditions per page under a storage key of the copy. It stores no trace of where
it cut. A page row holds `source_page_id`, its own dimensions, and nothing else
about the cut.

`internal/httpapi/pages.go` already holds the page operations. Both of them lock
the chain, check the states again inside the lock, write the renditions, and then
remove the files they replaced. The page rollback is the closest match: it
rebuilds one page from its source page under a new page key.

The row of a page carries the dimensions of its stored original rendition, which
is the image the split cut. A column of the source page is therefore a column of
`source.width`.

## Goals / Non-Goals

**Goals:**

- Keep the cut of every page addressable after the job that made it ended.
- Re-cut one page from the source bytes, so a run of edits loses no column.
- Reuse the page operation shape: preview endpoint, apply endpoint, chain lock.

**Non-Goals:**

- Moving the gutter for both pages of a spread at once.
- Cutting a page that reached the copy whole, or joining two pages back into one.
- A horizontal cut, a rotation, or any other correction of the parent image.
- Keeping an edited cut across a retry of the split job.

## Decisions

### Record the cut on the page row, as a side and one column

`book_page` gains `cut_side TEXT` and `cut_column INTEGER`. A page cut out of a
spread carries `'left'` or `'right'` and the column it was cut at. A page that
came through whole carries NULL in both. A check constraint keeps the pair
together and the side inside the two values.

One column is enough because the other edge of a cut page is always an edge of
the parent image. The left page is `[0, cut_column)`; the right page is
`[cut_column, source.width)`.

Alternatives considered:

- **Derive the side from the sibling order.** The two pages of a spread sit at
  neighbouring indexes, left first. It breaks as soon as one of them is deleted,
  and it says nothing about the column once an edit changed a width.
- **Store both crop edges.** `crop_left` and `crop_right` carry one number that
  is always 0 or `source.width`. The pair invites a crop this feature does not
  offer.

### Backfill the two columns from the stored widths

Migration `0008_page_cut.sql` fills both columns for every source page that has
exactly two pages made from it. The first by index is the left page, whose
column equals its own width. The second is the right page, whose column equals
`source.width - its width`.

The arithmetic is exact: the worker wrote each half as a crop of the source
original, so a stored width is a count of source columns. A dewarp copy is
untouched, because one source page there made one page. A spread that lost one
of its two pages stays NULL and offers no Edit; splitting the book again gives
it back.

### The column counts in source columns, on the wire as well

`GET .../cut` answers the display rendition of the source page together with the
stored width and height of that source page. The client scales the column by the
ratio between the width it renders and the width it was given. `POST .../cut`
takes the column back in source columns.

The alternative is a fraction between 0 and 1. It would make every save land on
a column the server rounds to, and the stored `cut_column` would stop matching
what the user saw. Integers keep one coordinate system from the sidecar to the
handle.

### Cut every attempt out of the source page

The apply handler decodes the original rendition of the source page, crops it at
the named column on the recorded side, writes the three renditions under a fresh
page key of this book, updates the row, then removes the files it replaced. It
never reads the page it replaces.

This is what makes an edit reversible in practice: a user who cut too far can
cut back out, because the columns are still in the parent. It also reuses
`ingest.Decode`, `ingest.Crop` and `ingest.WriteImageRenditions`, which is
exactly the path the split worker takes.

### Two endpoints beside the rollback pair

```
GET  /api/books/{id}/pages/{index}/cut   -> { source, side, column }
POST /api/books/{id}/pages/{index}/cut   -> the page view, after the cut
```

`source` carries `displayUrl`, `width` and `height`. The POST body is
`{"column": 1500}`. Both refuse with a reason when the page offers no re-cut:
422 for a page that cannot be cut, 409 while the chain runs. The apply handler
locks the chain with `LockChainTx`, checks `inFlight` again inside the lock, and
re-reads the offer, as the rollback does.

`pageView` gains `cutSide`, which is null for a page that records no cut.
`bookDetail` gains `pageCut`, true when the book is complete and has no copy of
its own. The menu shows Edit when both hold, which keeps the client free of any
knowledge of actions.

### One modal, one bar, two overlays

`PageCutDialog.vue` renders the display rendition of the source page at the
scale `PageCompare.vue` already computes, and lays two absolutely positioned
boxes over it: the plain part and the grey part, split at the bar. The bar
carries a handle and follows pointer events on the image box. The component
clamps the column so this page keeps at least one column, and converts to source
columns only when it sends.

Save replaces the page object in the held `BookDetail`, as the rollback does. The
grid cell shape is computed by the server at load, so an edited page keeps the
old cell until the next load.

## Risks / Trade-offs

- **The two pages of a spread can overlap or leave a gap.** A save moves one
  page alone, by decision. → The editor shows the parent image whole, so the
  user sees what the cut leaves out. The neighbour keeps its own Edit.
- **A retry of the split job discards every edit.** → The specs state it, and a
  retry only happens after a failure or a rejection of the copy.
- **The backfill cannot restore a pair that lost a page.** → That page keeps
  working and offers no Edit. Nothing is lost that a fresh split cannot give
  back.
- **A re-cut page stores its original as JPEG.** `WriteImageRenditions` encodes
  what it holds. → A cut page already went through that path in the split
  worker, so no page changes format.
- **The grid cell shape lags an edit until the next load.** → The cell is a
  shared shape for the whole book, and one page rarely moves it.
- **Two clients edit one page at once.** → The chain lock serialises them. The
  second cut lands on the source page again, so it cannot compound the first.

## Migration Plan

1. Ship `0008_page_cut.sql`. The server applies it at startup and the backfill
   runs once, inside that migration.
2. Ship the backend. Older page rows already carry their cuts.
3. Ship the frontend. A client that has not reloaded sees no Edit item, because
   its `BookDetail` carries no `cutSide`.

Rolling back means dropping the two columns. Nothing else reads them, and a page
keeps its bytes either way.
