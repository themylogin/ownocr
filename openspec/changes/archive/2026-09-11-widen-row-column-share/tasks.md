`./internal/ocrmerge/` is safe to run: it reads its fixtures from `testdata`.
`./internal/ocr/` imports `internal/testdb`, which truncates `book CASCADE` on the
live development database, so run it with a `-run` filter that names the tests
wanted.

## 1. Widen the share

- [x] 1.1 Raise `maxRowShare` from 0.25 to 0.40 and record beside it the bimodal
  distribution it was read off, in the style of the constants around it. Verify
  `go test ./internal/ocrmerge/` builds and reports only the two tests that lock
  in the old fault

## 2. Lock in the fix

- [x] 2.1 Replace the test that asserted page 85 reads `pero no` twice with one
  that asserts each of its three boxes holds the text printed in it, that no line
  swallows the head of the row below, and that no box of the page falls back to
  box engine text. Verify it passes
- [x] 2.2 Correct the row count in the page 87 criterion test, which reads 10 rows
  where the tighter share gave 11. Verify the measured lie is still 1.66 and the
  block is still the only unusable one of every fixture

## 3. Measure the blast radius

- [x] 3.1 Run `ocr.MergeOf` over the stored engine results of all 183 pages of
  books 17 and 18 and compare every answer against the merged result the previous
  code stored. Verify 178 pages are byte-identical and no page is worse
- [x] 3.2 Read the five pages that changed, box by box, and record in the proposal
  what each one was and is. Verify each change puts the text of a box over the ink
  that box was drawn around. Pages 88, 89 and 106 report identical counts, so the
  tool totals do not show them
- [x] 3.3 Verify book 18 does not change at all

## 4. Rewrite the stored results

- [x] 4.1 Run `backend/tools/ocrmerge -all -dry-run` and verify it reports 3486
  lines from `vl` against the 3483 of the previous write, with the discarded count
  unmoved at 14
- [x] 4.2 Run `backend/tools/ocrmerge -all` and verify pages 85, 88, 89 and 106 of
  book 17 now report no line falling back to the box engine

## 5. Left undone

- [x] 5.1 Page 85 keeps two boxes whose text runs past their ink: `otro—, tien` in
  a box with room for 6 characters, and `cierto es qu` in another. PP-OCRv6 left
  118 and 145 pixels of ink uncovered in the middle of those two rows, and the
  cut gives the uncovered text to the box before the gap. Carried out by the
  `add-row-gap-boxes` change, which places a box over each gap
