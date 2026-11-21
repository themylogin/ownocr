## Why

The OCR reader draws the text of the pages near the window only. Find-in-page
reads the DOM, so `Ctrl+F` reaches three pages of a book that holds hundreds. A
reader who searches a word gets a match count that describes the window, not the
book, and believes the rest of the book holds nothing.

A spike measured the cost of drawing every page at once. A book of 1000 pages
loads with 124 ms of blocked main thread, and the first search blocks for 957 ms.
Later searches cost a quarter of the first. The cost is affordable, so the reader
can hold the whole book and let the browser do the finding.

## What Changes

- The OCR reader draws the text of every page of the book, not the pages near
  the window only.
- The API answers the read lines of a whole book in one response. The reader
  makes one request instead of one per page.
- Each page of the reader skips its own rendering while it is off screen, and
  reserves the size it would take. Find-in-page still reaches the text it holds.
- The white grounds of one page become one shape instead of one shape per line.
  The spike shows the per-line shapes, not the text, carry the cost.
- `Ctrl+F` reaches every page of the book. A match on a far page scrolls that
  page into view.

Search matches one drawn line at a time. A phrase that crosses a line break
finds nothing. This change does not address that, and the design records why.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `book-library`: the OCR reader holds the text of every page instead of the
  pages near the window. Find-in-page reaches the whole book.
- `book-ocr`: the API answers the read lines of a whole book, beside the answer
  for one page it already gives.

## Impact

- `backend/internal/httpapi/router.go`, `pages.go`: a route that answers the
  lines of every page of a book.
- `frontend/src/api/client.ts`: the call for the new route, and its type.
- `frontend/src/composables/usePageOcr.ts`: one request for the book. The
  per-page asking and its guard against a second request go away.
- `frontend/src/views/ReaderView.vue`: the text of every page reaches
  `OcrPage`. The `IntersectionObserver` that tracked the near pages goes away.
  The spread reserves its size and skips its rendering off screen.
- `frontend/src/components/OcrPage.vue`: one shape holds the grounds of a page.
- `e2e/`: the reader tests that assert on the near pages only.

No database change. No change to what OCR stores or to how it runs.
