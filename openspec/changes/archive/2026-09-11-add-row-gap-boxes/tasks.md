`./internal/ocrmerge/` is safe to run: it reads its fixtures from `testdata`.
`./internal/ocr/` imports `internal/testdb`, which truncates `book CASCADE` on the
live development database, so run it with a `-run` filter that names the tests
wanted.

## 1. Place a box over the gap

- [x] 1.1 Add `gapBox` to `fill.go`, a box spanning the gap between two boxes of
  one row, standing where the row stands there. Verify a unit test puts it between
  its neighbours and gives it the shorter of their heights
- [x] 1.2 Add `shareAcrossGap` to `fill.go`, dividing a piece between its box and
  the gap beside it by width, under all three tests. Verify a table test covers
  the measured row, a gap too narrow, a piece that fits its box, a gap wider than
  its box, and a next box that overlaps rather than leaves a gap
- [x] 1.3 Add the two constants with their measured distributions beside them, in
  the style of the constants around them. Verify the package builds
- [x] 1.4 Call it from the per-member loop of `Merge`, after the length and
  agreement gates and before the piece is assigned, and put the new line in the
  `after` of that member's insert. Verify `go test ./internal/ocrmerge/` passes

## 2. Lock in the fix

- [x] 2.1 Add a test that page 85 holds a box over each of its two gaps, that each
  of the six boxes of those two rows holds the text printed in it, and that the
  gap boxes name the text engine, carry no score and are marked as placed. Verify
  it passes
- [x] 2.2 Rewrite the two tests that assert a page answers one line per box, so
  they check that every box the engine read survives in order rather than that the
  counts match. Page 85 legitimately gains two lines
- [x] 2.3 Key the page 85 row assertions by box rather than by position, so a
  placed line cannot shift them. Verify they pass

## 3. Measure the blast radius

- [x] 3.1 Run `ocr.MergeOf` over the stored engine results of all 183 pages of
  books 17 and 18 and compare every answer against the merged result the previous
  code stored. Verify 178 pages are byte-identical
- [x] 3.2 Verify book 18 does not change at all. The first attempt, without the
  test that a gap be no wider than its box, placed two boxes on its contents page
  and moved three letters of a title into a row of leader dots
- [x] 3.3 Verify the accounting invariant still holds over every fixture: the
  characters of every block equal the characters rendered plus the characters
  dropped

## 4. Rewrite the stored results

- [x] 4.1 Run `backend/tools/ocrmerge -all` and verify book 17 reports 18 lines
  placed against the 16 of the previous write, and book 18 reports none
- [x] 4.2 Verify the stored merged result of page 85 holds `otro—,` `tien`
  `e deseos costosos` over three boxes, and `cierto` `es qu` `e también es leal`
  over three more
