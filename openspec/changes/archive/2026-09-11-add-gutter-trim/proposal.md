## Why

The split cuts a spread at a column, but the gutter of a spread is a curve. Each
half therefore runs to the far extreme of the bow, so that neither half clips its
own text. That band is 127 to 161 pixels wide on the measured book, and it
carries the truncated ends of the lines of the neighbouring page.

The reader shows that ink as a column of short fragments against the inner edge,
and reads it as text. Book 17 holds seven such pages. A future export would carry
the same fragments into the exported file.

## What Changes

- The API derives a trim from the read lines of a page. A trim is one column on
  the cut side of the page, and it covers the strip that holds no text of
  this page.
- The lines endpoint drops the fragments that lie in the trim, and reports the
  trim beside the lines it keeps. A client needs no rule of its own.
- The OCR reader paints the trimmed strip white over the read half. The plain
  half stays the untouched photo, so a user can still see what was cut.
- The page keeps its stored width and height. The trim covers pixels; it does not
  move them. Every stored box stays valid, so a renderer needs no arithmetic.
- **BREAKING**: the lines endpoint no longer answers every stored line. The
  current requirement forbids it to drop a line.

## Capabilities

### New Capabilities

- `page-gutter-trim`: reads the lines of one page and answers the column that
  cuts off the ink of the neighbouring page, or nothing when the page holds no
  such strip. It takes lines, a page width and a cut side, and touches no
  file and no engine.

### Modified Capabilities

- `book-ocr`: *The API answers the read lines of one page* gains the trim and
  drops the lines that lie in it. It also resolves the cut side of the page
  through the lineage of that page, because an OCR copy carries no cut side of
  its own.

## Impact

- New Go package beside `internal/ocrmerge`, holding the rule and its constants.
- `httpapi.pageOCR` and `pageOCRView` in `backend/internal/httpapi/pages.go`.
- A lineage read: the page of an OCR copy carries `cut_side` on its source page,
  not on itself.
- `frontend/src/components/OcrPage.vue` and the `PageOcr` type in
  `frontend/src/api/client.ts`.
- No migration. The trim is derived on request from lines that are already
  stored, so a rescan of a page recomputes it with no further work.
- No change to the split, to the sidecar, or to any stored image.
