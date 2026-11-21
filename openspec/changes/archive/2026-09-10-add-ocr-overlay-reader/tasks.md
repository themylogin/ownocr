## 1. The lines of a page reach the API

- [x] 1.1 Add `LinesOf(data []byte) ([]Line, error)` to `internal/ocr`, beside
  `boxResult`, reading `lines[].bbox` and `lines[].text`. Verify a unit test
  reads a `v6` fixture and a `merged` fixture with the same call.
- [x] 1.2 Add `PageOCRResult(ctx, pageID, model)` or reuse `PageOCRResults` in
  `internal/books` to pick the `merged` row over the `v6` row. Verify a store
  test that a page with both rows answers `merged`, and a page with `v6` alone
  answers `v6`.
- [x] 1.3 Add `pageOCRView` to `internal/httpapi/view.go` with `model`, `width`,
  `height` and `lines[{box,text}]`. Verify `go build ./...` passes.
- [x] 1.4 Implement `GET /api/books/{id}/pages/{index}/ocr` in
  `internal/httpapi/pages.go` through `resolvePage`, and register it in the
  15 second group of `router.go`. Verify `curl` on a page of an OCR copy answers
  the model, the page size and the lines.
- [x] 1.5 Answer 404 with a reason for a page that holds no result, an unknown
  page index, and an unknown book. Verify handler tests get 404 and a reason for
  each of the three.
- [x] 1.6 Verify a handler test that the answered `width` and `height` equal the
  page dimensions of the book detail, and that a box of the stored result reaches
  the client unchanged.

## 2. The fake endpoint writes boxes worth drawing

- [x] 2.1 Make `backend/tools/fakerunpod` write several v6 lines per page, with
  boxes spread over the page size instead of `[0, 0, 10, 10]`. Verify the stored
  `v6` data of an e2e OCR copy holds more than one line, each with a box inside
  the page.
- [x] 2.2 Give the fake `vl` blocks a `block_bbox` that covers its v6 lines, so a
  copy with the profile `vl` produces a merged result with text from the blocks.
  Verify `pageResults` in an e2e test reports a `merged` row whose lines carry
  the block text.

## 3. The client asks for the lines

- [x] 3.1 Add `PageOcr`, `OcrLine` and `api.pageOcr(id, index)` to
  `frontend/src/api/client.ts`. Verify `npm run typecheck` passes in the frontend
  container.
- [x] 3.2 Add a composable that holds the lines of the pages already asked for,
  in a `Map` keyed by page index, and answers nothing for a page the server
  refused. Verify a page that answers 404 leaves no error in the console and no
  entry that blocks a later ask.

## 4. The split reader

- [x] 4.1 Add `OcrPage.vue`: the image, the white veil at 50 percent opacity, and
  the `svg` with `viewBox="0 0 width height"` holding one `text` per line with
  `textLength` and `lengthAdjust="spacingAndGlyphs"`. Verify one page renders
  every line of a stored result at its box.
- [x] 4.2 Give `ReaderView.vue` the split layout for a book in the `ocr` stage:
  one row per page, the plain image left, `OcrPage` right, both at the same
  size. Verify a book in another stage keeps the single column.
- [x] 4.3 Compute the per-page scale from the window width, as `design.md` states,
  with a resize listener like `PageCompare.vue`. Verify a narrow window scales
  both halves down and a wide window stops at the stored page size.
- [x] 4.4 Add the `wide` layout flag to `App.vue` and the composable that
  `ReaderView` sets for an OCR book. Verify the OCR reader uses the whole window
  and every other view keeps the 72 rem column.
- [x] 4.5 Watch every page row with one `IntersectionObserver`, a root margin of
  one window height. Ask for the lines of a row that enters, and drop the `svg`
  of a row that leaves. Verify in the browser that a book of many pages issues a
  few requests, not one per page.
- [x] 4.6 Verify that the drawn text is selectable and copies as the text of the
  lines, and that the left image carries an empty alt so a selection holds no
  alt text.

## 5. The tests

- [x] 5.1 Add an e2e case: the reader of an OCR book shows two images per page,
  the right one carrying the veil and the drawn text. Verify against a copy
  started with the profile `vl`.
- [x] 5.2 Add an e2e case: a box of a drawn line sits at the place its stored box
  names, within a pixel of the scale the image took. Read the stored result with
  `pageResults` and compare against the rendered box.
- [x] 5.3 Add an e2e case: a copy with the profile `v6` draws the v6 lines, so the
  fallback works with no merged row.
- [x] 5.4 Add an e2e case: the reader of a book in the `split` stage shows one
  image per page and no veil.
- [x] 5.5 Add an e2e case: an OCR book of many pages draws the boxes of the pages
  near the window only. Count the drawn line nodes before and after a scroll.
- [x] 5.6 Run `go test ./...` in the backend container and the whole e2e suite
  with `./run-e2e.sh`. Verify both pass.
