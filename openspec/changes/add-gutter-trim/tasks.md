## 1. The trim rule

- [x] 1.1 Add `backend/internal/guttertrim` with the constants of design.md, each
  carrying the evidence that sets it, and verify `go build ./...` passes
- [x] 1.2 Implement the inset mapping for both gutter sides and verify a unit test
  that a box 0,273,55,331 on a 2083 wide `right` page gives the insets 0 and 55,
  and a box 2003,2667,2064,2728 on a 2067 wide `left` page gives 3 and 64
- [x] 1.3 Implement fragment classification and verify a unit test that a line
  touching the edge and under 0.10 of the width is a fragment, and that lines at
  the insets 65 to 137, at 0.21 of the width, and at 0.94 of the width are not
- [x] 1.4 Implement the body edge and refuse a page with no body line, and verify
  a unit test on a page of 33 body lines, a running head and a folio that the
  body edge is 182, and a test that a page with no body line answers nothing
- [x] 1.5 Implement the corridor guard, the two fragment minimum and the midpoint
  column, and verify unit tests that 18 fragments ending at 61 with the body edge
  182 give 122, that 2 fragments ending at 28 with the body edge 152 give 90, and
  that a single fragment answers nothing

## 2. Fixtures from the measured pages

- [x] 2.1 Add testdata fixtures of the line boxes of book 17 pages 29, 45, 50, 53
  and 87, book 18 page 1, and book 19 page 51, and verify each file parses in a
  test that loads it
- [x] 2.2 Add a golden test over those fixtures and verify it reproduces the
  verdict table of design.md: 29 trims at 90, 45 at 122, 53 at 109, 19/51 at 127
  against the right edge, and 50, 87 and 18/1 answer nothing

## 3. The endpoint

- [x] 3.1 Add store methods that answer the nearest page in the lineage of a page
  carrying a cut side, for one page and for a whole book, bounded against a cycle,
  and verify a test that an OCR copy page takes the side of the split page it was
  read from and that a page with no such ancestor answers nothing
- [x] 3.2 Call the rule from `pageOCR`, drop the lines whose box lies wholly
  inside the trim, and add the trim to `pageOCRView`, and verify a handler test
  that a page carrying a strip answers a trim of 122 against the left edge, drops
  its 3 fragments, and still reports the width 2083
- [x] 3.3 Verify a handler test that a page holding no trim answers no trim and
  every stored line, and that a page whose lineage holds no cut side does the same
- [x] 3.4 Verify a handler test that a folio, a running head and a one word last
  line outside the trim all survive
- [x] 3.5 Trim `bookOCR` through the same helper, reading the cut sides of the
  whole book in one query, and verify a handler test that the book answer reports
  the same trim and the same line count as the page answer for every page

  The reader asks for the whole book in one request, so the book answer is the
  one it draws. Trimming the page answer alone would leave the reader unchanged.

## 4. The reader

- [ ] 4.1 Add the trim to the `PageOcr` type in `frontend/src/api/client.ts` and
  verify `npm run build` passes in the frontend container
- [ ] 4.2 Paint the trimmed strip as an opaque rectangle in `OcrPage.vue`, under
  the glyphs and over the veil, and verify a component test that the rectangle
  covers the strip and that a page with no trim draws none
- [ ] 4.3 Verify by eye in the reader that book 17 page 45 shows a clean inner
  margin on the read half and the untouched photo on the plain half

## 5. End to end

- [ ] 5.1 Add an e2e case that a read book whose page carries a strip draws the
  trim rectangle and no fragment line, and verify `./run-e2e.sh` passes
- [ ] 5.2 Run `openspec validate --strict add-gutter-trim` and verify it passes

Note: `go test ./...` truncates `book CASCADE` on the development database. Books
17 and 18 are the evidence behind design.md. Dump them before running the backend
suite, or run it against a throwaway database.
