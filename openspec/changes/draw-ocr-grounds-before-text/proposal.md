## Why

The OCR reader draws each line as a white ground with its text on top, one line
after another. SVG paints in document order, so the ground of a line covers the
text of the line above it. A glyph that reaches out of its own box, which is
every descender and every tall capital, is cut off by the next ground.

## What Changes

- The overlay draws every white ground first, then every line of text. No ground
  can then cover a glyph, whatever the boxes of two lines do to one another.
- The text stays in page order, so a selection still reads down the page.
- Nothing else moves: the boxes, the scale, the veil, and the fonts are the ones
  the reader uses now.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `book-library`: the OCR reader requirement gains the rule that a white ground
  covers no drawn text.

## Impact

- Frontend: `frontend/src/components/OcrPage.vue` alone. The grounds and the text
  become two passes over the same lines instead of one group per line.
- Tests: `e2e/tests/ocrreader.spec.ts` gains a case. The `ocr-line` test id stays
  on the text elements, so the cases that exist keep working.
- No backend change, no API change, no change to a stored result.
