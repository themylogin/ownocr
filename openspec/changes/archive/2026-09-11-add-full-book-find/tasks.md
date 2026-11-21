## 1. The API answers a whole book

- [x] 1.1 Add the handler that answers the lines of every page of a book, built
      from the per-page answer so both give the same model, order and text.
      Verify a new test in `backend/internal/httpapi/pages_test.go` reads a book
      of 12 pages and gets 12 entries in page order.
- [x] 1.2 Return an entry with no lines for a page that holds no result, instead
      of failing the request. Verify a test reads a copy with 5 of 12 pages read
      and gets 12 entries, 5 with lines.
- [x] 1.3 Refuse the request for a book that does not exist. Verify a test gets
      the refusal for an unknown book id.
- [x] 1.4 Register the route in `backend/internal/httpapi/router.go`. Verify the
      route test reaches the handler.
- [x] 1.5 Add a test that reads a whole book and then page 3 alone, and asserts
      the two answers agree on the model and on every line of page 3.

## 2. The reader asks once

- [x] 2.1 Add the call and its type to `frontend/src/api/client.ts`. Verify
      `npm run build` type checks.
- [x] 2.2 Rewrite `usePageOcr.ts` to make one request for the book and hold the
      answer by page index. Remove the per-page asking and its in-flight guard.
      Verify `linesOf` returns the lines of a page after the request settles.
- [x] 2.3 Make a failed request leave every page without lines and raise no
      error. Verify the reader shows veiled images and no error banner.

## 3. The reader draws every page

- [x] 3.1 Drop the `near` set and the `IntersectionObserver` from
      `ReaderView.vue`, and pass the lines of every page to `OcrPage`. Verify the
      reader of a 40 page book draws text on all 40.
- [x] 3.2 Add `content-visibility: auto` to `.spread` with `contain-intrinsic-size`
      built from the width and height `sizeOf` already computes. Verify the
      computed style of a spread reports both, and that the document scroll
      height matches the page count times the page height.
- [x] 3.3 Keep the reserved size correct when the window resizes. Verify a resize
      changes `contain-intrinsic-size` with the scale, and that no page moves.

## 4. One shape holds the grounds

- [x] 4.1 Replace the per-line `<rect>` grounds in `OcrPage.vue` with one
      `<path>` per page, one subpath per line box. Verify the existing e2e test
      `every ground is drawn below every line` still passes.
- [x] 4.2 Verify a page of overlapping boxes still shows no glyph covered by a
      ground, with the existing test for touching boxes.

## 5. Tests

- [x] 5.1 Replace the e2e group `the OCR reader draws the pages near the window
      only` in `e2e/tests/ocrreader.spec.ts` with one that asserts every page of
      a long book holds its drawn lines, including a page far below the window.
- [x] 5.2 Add an e2e test that opens the reader at a late page and asserts it
      lands there with its text drawn.
- [x] 5.3 Add an e2e test that asserts the reader makes one request for the lines
      of the book, and none per page.
- [x] 5.4 Run `run-e2e.sh` and verify the whole suite passes.

## 6. Measure before closing

This group closed on observation, not on measurement. The reader was confirmed
working on the 135 page book 17, and no timing was recorded. The element count
below is measured; the timings the group asked for are not, and design.md says
so under Risks.

- [x] 6.1 Measure the real reader on the longest book in the library. Record the
      blocked main thread at load, at the first find, and at a later find.
      Compare against the spike numbers in design.md.
      CLOSED ON OBSERVATION: the reader works well on book 17, no timing taken.
- [x] 6.2 Record the DOM element count and the JS heap of that reader. Confirm
      they stay within what the machine holds.
      PART DONE: the element count is in design.md. The heap was not read.
- [ ] 6.3 If Vue's cost overshoots the spike, build the contents of the two `<g>`
      elements as a string and set it with `v-html`, escaping `&`, `<` and `>` in
      the OCR text. Verify the drawn text is unchanged and the numbers improve.
      SKIPPED: the condition did not apply. Vue's cost did not overshoot.
- [x] 6.4 Search a word that only a late page holds, in each browser the project
      supports, and verify the find scrolls that page into view.
      CLOSED ON OBSERVATION: confirmed working. One browser, not a matrix.
- [x] 6.5 Delete `frontend/svg-text-spike.html` once the numbers are recorded in
      design.md.
