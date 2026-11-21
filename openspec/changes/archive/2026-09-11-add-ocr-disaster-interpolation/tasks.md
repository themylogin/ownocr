Any Go test package that imports `internal/testdb` truncates `book CASCADE` on the
live development database, and `./internal/ocr/` is one of them. Naming a single
package is no protection. `./internal/ocrmerge/` is safe: it reads its fixtures
from `testdata`. Copy out any rows the work depends on before running anything
else.

## 1. Carry the line corners into the merge

- [x] 1.1 Add the four corners to `ocrmerge.Line` as an optional field, and verify
  `go test ./internal/ocrmerge/` still passes with no other change
- [x] 1.2 Read `raw.dt_polys` in `ocr/merge.go` and pair each entry with the line
  of the same index, and verify a new test in `ocr/merge_test.go` asserts the
  corners of line 0 of the `vl` fixture match the stored `dt_polys[0]`
- [x] 1.3 Guard the pairing: a result whose `dt_polys` is absent, or shorter than
  `lines`, leaves the corners empty rather than failing, and verify a test feeds
  a `v6` result with no `raw` and gets a merge identical to today's
- [x] 1.4 Add `slopeOf` to `layout.go`, the median slope of the bottom edge over
  the full-width lines of a block, and verify unit tests cover a level block, a
  sloping block, and a block whose narrow fragments report steep slopes that the
  median must ignore

## 2. Record the two pages as fixtures

- [x] 2.1 Extract page 87 of book 17 from `page_ocr.data` into
  `testdata/book0017page0087.json`, keeping `v6.raw.dt_polys`, and add its purpose
  to `fixturePurpose`: the top block slopes past its row spacing, so v6 gives no
  valid reading order. Verify `TestFixturesParse` passes
- [x] 2.2 Extract page 85 of book 17 into `testdata/book0017page0085.json` the same
  way, with the purpose: one row broken into three boxes, which this change must
  leave alone. Verify `TestFixturesParse` passes
- [x] 2.3 Extract one block of book 18 whose text engine collapsed into repetition
  into `testdata/book0018page0036.json`, with the purpose: the text engine is the
  broken reading, so the classifier must not touch it. Verify `TestFixturesParse`
  passes
- [x] 2.4 Record in `fixturePurpose` that `page0052`, `page0432` and
  `book0017page0116` hold no `dt_polys`, so they classify at zero slope and guard
  the unchanged path. Verify a test asserts the merge of all three is unchanged

## 3. Classify a block

The criterion changed twice under measurement. Measuring the width of the block,
and then the spread of its box widths, both called blocks unusable that merge
correctly. design.md records all three and what each one changed. Pages 90, 102
and 103 are fixtures because of it.

- [x] 3.1 Add `boxLie`, the largest top-edge difference between two boxes of a
  block that stand beside each other, in row pitches, measured against the pitch
  of the whole page. Verify a unit test reproduces 1.66 for page 87 block 1, and
  0 for a block whose every printed row is one box however far it slopes
- [x] 3.2 Add `blockFill`, the non-whitespace characters of a block over those of its
  assigned lines, and verify a unit test gives 1.02 for page 87 block 1 and rejects
  the collapsed block of book 18 at 1.75
- [x] 3.3 Add the classifier that calls a block a disaster when `boxLie` reaches 1
  and `blockFill` lies in [0.7, 1.4], and verify a table test over every fixture
  names page 87 block 1 and nothing else
- [x] 3.4 Add the two named constants with the measured distributions recorded
  beside them, in the style of `ocrmerge.go`, and verify the package builds

## 4. Measure the page reference

- [x] 4.1 Add the reference measure over the blocks the classifier did not call a
  disaster: the row pitch and the characters a printed row holds. Verify a unit
  test gives pitch 81 and 56 characters for page 87
- [x] 4.2 Answer no reference when the page holds no surviving block, and verify a
  test built from a single-block disaster page interpolates nothing and keeps
  every v6 box
- [x] 4.3 Split classification from interpolation into two passes over the blocks,
  so the reference never comes from a disaster block, and verify
  `go test ./internal/ocrmerge/` passes

## 5. Interpolate a disaster block

- [x] 5.1 Add the two line-count estimates, from the characters and from the
  slope-corrected height, and verify a unit test gives 8.02 and 7.66 for page 87
  block 1, and that the naive height gives 9.44 and is not what the code uses
- [x] 5.2 Interpolate nothing unless both estimates round to the same integer, and
  verify a test drives them apart and gets every v6 box kept
- [x] 5.3 Build the interpolated lines with `wrapInto` and `box`, spaced evenly
  from the top of the block to its bottom, each naming the text engine, carrying
  no score and marked as placed by the merge. Verify a test asserts all five
  properties on the 8 lines of page 87 block 1
- [x] 5.4 Add a golden test in the style of `golden_test.go` recording the 8 lines
  of page 87 block 1 line for line, and verify the first and the last match the
  printed page exactly

## 6. Remove the lines of a disaster block

- [x] 6.1 Extend `spliceIn` to drop a set of line indexes and to put the
  interpolated lines where the first member of the block stood, and verify a unit
  test on a synthetic page asserts the surviving lines keep their order and boxes
- [x] 6.2 Verify the whole merge of page 87 answers no box the v6 result reported
  for block 1, and that block 2 of the same page keeps every one of its boxes
- [x] 6.3 Verify page 85 and the book 18 fixture merge exactly as they do today,
  by recording their current merged output as a golden test before task 3 lands

## 7. Account for the discarded boxes

- [x] 7.1 Add `Counts.Discarded` and fill it from the classifier, and verify a test
  reports 14 for page 87
- [x] 7.2 Extend the accounting test in `accounting_test.go` so the character
  equality holds over every fixture including page 87, and verify page 87 reports
  no dropped character for block 1
- [x] 7.3 Report the discarded count in the tool output of
  `backend/tools/ocrmerge`, and verify `-book 17 -dry-run` prints it

## 8. Rewrite the stored results

- [x] 8.1 Verify only book 17 reports a discarded box, on page 87 alone, and book
  18 reports none. Done by running `ocr.MergeOf` over the stored engine results of
  all 183 pages of both books and comparing every answer against the merged result
  the previous code stored: one page changes, 182 are byte-identical, book 18 does
  not move. The `-dry-run` form of the tool was not used, because the development
  database was emptied mid-task (see the note below)
- [x] 8.2 Run `backend/tools/ocrmerge -all` and verify the stored merged result of
  page 87 holds 8 full-width lines whose text reads in order. Both books rewrote
  from their stored engine results, no page was read again, and only page 87
  reports a discarded box: 14 of them, 9 lines placed, 35 lines answered

## 9. Note on the development database

Running `go test ./internal/ocr/` emptied the development database: that package
imports `internal/testdb`, whose `New` runs `TRUNCATE book CASCADE` against the
same database the dev stack serves. 12 books went with it. Production is
untouched, the image files under `/data` survive, and `./pull-prod-db.sh` restores
the rows.

Every engine result of books 17 and 18 was exported before that happened, so no
measurement in this change was lost, and the three new fixtures were cut from the
export. The database was restored with `./pull-prod-db.sh` before task 8 ran.
