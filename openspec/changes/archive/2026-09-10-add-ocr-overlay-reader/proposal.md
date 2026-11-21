## Why

A book in the `ocr` stage holds the text of every page, and nothing shows it. A
user cannot see what the engines read without the database. The reader already
shows the page image, so it is the place to show the text beside it.

## What Changes

- The reader splits in two for a book in the `ocr` stage. The left half holds the
  page image. The right half holds the same image under a white veil, with the
  OCR text drawn in the box of every line.
- The OCR reader uses the whole window width, not the 72 rem content column.
  Both halves together stay inside the window.
- Both halves scale down together when the two images do not fit. The line boxes
  and the font size follow the same scale.
- The reader asks the server for the text of a page when that page comes near the
  window, and drops the drawn text of a page that goes far away. A book of 500
  pages therefore draws a few pages of boxes, not all of them.
- The API answers the line boxes and the text of one page. It answers the merged
  result when the page holds one, and the PP-OCRv6 result when it does not. It
  names which of the two it answered.
- The reader for every other stage does not change.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `book-ocr`: the stored results of a page reach a client. The API gains a page
  text endpoint, which picks the merged result over the engine result.
- `book-library`: the reader gains the split OCR layout, the veil, the drawn
  line boxes, and the rule that draws only the pages near the window.

## Impact

- Backend: `backend/internal/httpapi` gains a route and a view. It reads the
  results through `books.Store`, which already loads them.
- Frontend: `frontend/src/views/ReaderView.vue` gains the split layout. A new
  component draws the veil and the boxes of one page. `frontend/src/api/client.ts`
  gains the call and its types.
- Layout: the OCR reader breaks out of the `main` column in `App.vue`.
- Tests: `e2e/tests/ocr.spec.ts` and `e2e/tests/reader.spec.ts` gain cases. The
  backend gains handler tests.
- No migration. No change to the OCR worker, the engines, or the merge.
