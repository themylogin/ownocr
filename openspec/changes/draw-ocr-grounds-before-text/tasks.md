## 1. The overlay draws the grounds first

- [x] 1.1 In `frontend/src/components/OcrPage.vue`, replace the per line `g` that
  holds a `rect` and a `text` with two passes over the same `lines`: one `g` of
  every `rect`, then one `g` of every `text`. Verify the rendered SVG holds every
  `rect` before the first `text`.
- [x] 1.2 Keep the `data-testid="ocr-line"` on the `text` elements, and keep the
  texts in page order, so a selection still reads down the page. Verify
  `npm run typecheck` passes in the frontend container.
- [x] 1.3 Keep the `rect` fill rule working after the regrouping: the grounds are
  white and the glyphs black. Verify in the browser that a page of a read book
  shows white grounds under black text.

## 2. The tests

- [x] 2.1 Add an e2e case to `e2e/tests/ocrreader.spec.ts`: inside the overlay
  SVG, every ground stands before every drawn line in document order, so no
  ground can paint over a glyph. Verify it fails against the per line grouping.
- [x] 2.2 Run `./run-e2e.sh ocrreader.spec.ts` and verify every case passes,
  including the cases that place a box and copy the text.
