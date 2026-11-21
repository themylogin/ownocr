## Why

The sidecar decides where a spread is cut, and the user cannot correct it. A
gutter that the detector reads a few hundred columns off gives one page a strip
of its neighbour and takes a margin off the other.

The only remedy today rejects the whole copy and splits the book again, which
gives the same cut back. A user who sees a bad cut on one page needs to move
that one cut, on that one page.

## What Changes

- Add an Edit item to the page menu of a page that a split cut out of a spread.
  It stands first, above the divider and Delete.
- Add a modal that opens on the page the split was cut from, at the size the
  window allows. A vertical bar sits on the cut column. The part of the parent
  image that this page holds stays plain. The rest lies under a grey layer at
  half transparency.
- Let the user drag the bar across the parent image. Cancel closes the modal and
  changes nothing. Save sends the column the bar stands on.
- Cut the page out of the parent page again on Save, under a storage key of its
  own book. Build its three renditions again and remove the ones it replaced.
  The page keeps its index and the source page it names.
- Re-cut the named page alone. The other page of the spread keeps the cut it
  has, so the two pages may overlap or leave a strip of the spread in neither.
- Record the cut of every page a split cuts: which side of the gutter the page
  holds, and the column it was cut at. A page that reached the copy whole
  records no cut and offers no Edit.
- Refuse the re-cut while any book of the chain is `pending` or `processing`,
  and while the book has a copy of its own. Both rules already guard the page
  rollback.

## Capabilities

### New Capabilities

<!-- None. The re-cut is a page operation, which `book-pages` already holds. -->

### Modified Capabilities

- `book-pages`: a page cut out of a spread can be cut again at a column the user
  names. Two endpoints serve it. The in-flight refusal covers the re-cut.
- `book-split`: the split worker records the side and the cut column of every
  page it cuts.
- `book-library`: the page menu gains Edit, and the interface gains the modal
  that moves the cut.

## Impact

- Database: migration `0008_page_cut.sql` adds `book_page.cut_side` and
  `book_page.cut_column`, and backfills both for every pair of pages that one
  source page made.
- Backend: `internal/books` carries the two columns and gains the re-cut write.
  `internal/split` records them. `internal/httpapi/pages.go` gains two routes.
- API: `GET /api/books/{id}/pages/{index}/cut` answers the parent image and the
  current cut. `POST` of the same path re-cuts the page. `pageView` gains
  `cutSide`, and `bookDetail` gains `pageCut`.
- Frontend: new `PageCutDialog.vue`. `PageMenu.vue` gains the item, and both
  views pass the new book flag. `api/client.ts` gains the two calls.
- Tests: Go tests for the backfill, the refusals and the re-cut write. A
  Playwright spec for the menu item, the drag, Cancel and Save.
- Docs: the README gains the re-cut, the two routes and the two columns.
