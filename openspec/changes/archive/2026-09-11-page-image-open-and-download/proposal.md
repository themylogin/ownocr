## Why

A right click on a page opens our own menu, so the browser menu never appears.
That menu is the only way to open a page image in a new tab or to save it. A
user viewing a book cannot do either.

The browser menu would not solve it well anyway. It acts on the rendition on
screen: the 480 px thumbnail in the grid, the 1280 px image in the reader. A
user who wants the image wants the uploaded bytes. Our menu can offer exactly
that.

## What Changes

- The page menu holds "Open in new tab" first and "Download" second, above a
  divider and the items it holds today.
- Both items act on the original rendition of the page, in the grid and in the
  reader alike. Neither acts on the rendition on screen.
- "Download" saves the file under a name built from the book name and the page
  index, not under the storage key.
- The right half of the OCR reader offers no menu at all: not ours, and not the
  browser's. That half is an image, a white veil and drawn text stacked
  together, so a saved copy would hold the page without one glyph of the
  reading.
- The page grid and the reader keep the menu they have today for every other
  item.

Nothing changes for book covers in a stage view, or for the images inside the
comparison dialog and the cut editor.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `book-library`: The page menu gains two items that always appear, so the rule
  that a page can open a menu holding "Delete" alone no longer holds. The OCR
  reader gains a rule that its right half opens no menu.

## Impact

Frontend only. The server already reports the original URL of every page in
`pageView`, and the client already carries it to the menu. No API changes, no
database changes and no new endpoint.

- `frontend/src/components/PageMenu.vue`: two new items, and the book name it
  needs to build a file name.
- `frontend/src/views/BookView.vue` and `frontend/src/views/ReaderView.vue`:
  pass the book name. The reader also moves its right click handler off the
  spread and onto the left image.
- `e2e/tests/pages.spec.ts` and `e2e/tests/cut.spec.ts`: both count the items of
  the menu, and both counts grow by two.

The download depends on the images staying on the origin that serves the app.
Nginx serves `/files/` in production and the Vite dev server serves it in
development, so they do. A browser ignores the download name for another origin
and shows the image instead of saving it.
