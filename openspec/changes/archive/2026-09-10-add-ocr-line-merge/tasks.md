## 1. Fixtures

- [x] 1.1 Add `backend/internal/ocrmerge/testdata/` and write the `v6` and `vl`
  results of pages 10, 11, 14, 116 and 119 of book 3292 into it, one JSON file per
  page holding both engines. Take them from `page_ocr.data`, unchanged. Verify
  each file parses and page 10 reports 22 v6 lines and 6 VL blocks.
- [x] 1.2 Record what each fixture page is for, in one line at the head of its
  file or in a table in the test: page 10 is clean with a hyphen and a footnote,
  page 11 has margin lines outside every block and lines out of reading order,
  page 14 is a Greek quotation v6 destroyed, page 116 is VL repeating itself, page
  119 is VL answering too little. Verify the list names every fixture.

## 2. The merge package

- [x] 2.1 Add `backend/internal/ocrmerge` with the input and output types: a box
  engine line with a box, text and score, a text engine block with a box and text,
  and an answered line with a box, text, source and score, plus the dropped
  entries and the counts. Verify `go build ./...` passes.
- [x] 2.2 Add rune folding: NFKD, drop combining marks, case fold. Verify a table
  test maps `í` to `i`, `Φ` to `φ`, `ὸ` to `ο` and leaves `φ` and `o` distinct.
- [x] 2.3 Add the grouping: assign each line to the block covering the largest
  share of its box, and none below 0.5. Verify on the page 11 fixture that the
  five blocks take their lines and the margin lines take none.
- [x] 2.4 Add the reading order of a group: row bands by top edge with a tolerance
  of half the median line height, then left edge inside a row. Verify on the page
  11 fixture that the answered order is top to bottom, and that the two lines
  sharing a row come out left then right.
- [x] 2.5 Add the Needleman-Wunsch alignment over runes with the cost table of
  design.md, returning the path. Verify a unit test that `φiλoσo-` against
  `φιλοσοφ` pairs the six letters and leaves the hyphen unpaired.
- [x] 2.6 Add the cut: piece `k` runs from its own start to the start of piece
  `k+1`, the first piece starts at 0, the last ends at the end of the block.
  Verify a unit test that the pieces concatenate to the block text exactly, for a
  block whose text both begins and ends with characters no line matches.
- [x] 2.7 Add whitespace normalisation of a piece: collapse runs to one space,
  trim both ends. Verify a unit test that a piece holding a newline comes out with
  one space and no leading space.
- [x] 2.8 Add the gate: refuse a piece when the share of v6 characters paired with
  a fold-equal block character is below 0.30 and the v6 line holds 20 characters
  or more. Verify on the page 14 fixture that every Greek line is accepted, and on
  the page 116 fixture that the repeated lines are refused.
- [x] 2.9 Add the fallbacks: a line with no block, a line whose piece is empty, a
  line under a block with empty text, and a refused line all keep the box engine
  text and name it as their source. Verify no fixture page answers an empty line
  for a line the box engine read.
- [x] 2.10 Add the block refusals: refuse a block whose lengths differ by more
  than eight times, and refuse a block whose product of lengths passes the cap,
  before allocating a table. Verify a unit test that a 40000 character block
  against 200 characters of lines returns at once with every line kept and the
  block reported as dropped.
- [x] 2.11 Add the dropped entries and the counts, with the reasons `line`,
  `block` and `unaligned`. Verify each reason is produced by a test.

## 3. The accounting invariant

- [x] 3.1 Add a test that, for every fixture page, the characters other than
  whitespace of every block equal those of the answered lines sourced from the
  text engine plus those reported as dropped. Verify it passes with a difference
  of zero on all five pages.
- [x] 3.2 Add a test that the answered lines are one per box engine line, in the
  same order, with the same boxes. Verify it passes on all five pages.
- [x] 3.3 Add the expected merged text of page 10 to the fixtures and assert it,
  so a change in the cut is visible. Verify the assertion covers the hyphen split
  of `(φιλοσο` and `φία,` and the footnote lines.

## 4. The worker writes the merged row

- [x] 4.1 Add decoding of `data.lines` of a `v6` result and `data.parsing_res_list`
  of a `vl` result in `backend/internal/ocr`. Verify a test decodes the page 10
  fixture into the merge input types.
- [x] 4.2 Call the merge in `storePage` after both engine rows are written, and
  insert the `merged` row with the joined lines as `txt` and the structure of
  design.md as `data`. Do this only when the profile names both engines. Verify a
  worker test that a copy with the profile `vl` gives every page three rows and a
  copy with the profile `v6` gives every page one.
- [x] 4.3 Confirm a retry replaces the merged row. `DeletePages` removes it
  through the cascade and `InsertPageOCR` replaces by `(page_id, model)`. Verify a
  test that runs the worker twice and finds one `merged` row per page.
- [x] 4.4 Confirm a page one engine read as empty still merges. Verify a test with
  a VL result holding no block that answers every v6 line with its v6 text.
- [x] 4.5 Confirm `checkOutput` is untouched by the third row. It counts results
  per engine from the endpoint, which never names `merged`. Verify
  `go test ./internal/ocr/...` passes.

## 5. The backfill tool

- [x] 5.1 Add `backend/tools/ocrmerge` taking a database URL and a book id. It
  reads the `v6` and `vl` rows of every page of that copy, runs the merge, and
  writes the `merged` row. Verify `go build ./...` passes and the tool refuses a
  book whose profile is not `vl`.
- [x] 5.2 Run it against book 3292 in the development database. Verify all 124
  pages gain a `merged` row, that the summed counts report 3839 lines from VL and
  248 from v6, and that the summed dropped characters are 2390. The gate of task 7
  moved one line of those totals from VL to v6.
- [x] 5.3 Spot check page 10 and page 116 of book 3292 in `psql`. Verify page 10
  reads correct Greek with `$ ^{1} $` kept, and page 116 keeps its v6 text on the
  repeated lines and reports the dropped text.

## 6. Documentation

- [x] 6.1 Add the `merged` row to the README beside the `page_ocr` table: what it
  holds, that it is derived, and that only the `vl` profile has one. Verify the
  README names the three models a page can hold.

## 7. The gate on the length of a piece

- [x] 7.1 Refuse a piece longer than `4 * line + 20` runes in
  `backend/internal/ocrmerge`, beside the agreement gate and independent of it.
  Name the two constants and record beside them that the gate trips once in the
  3840 accepted pieces of book 3292. Verify `go build ./...` passes.
- [x] 7.2 Add unit tests: a 64 rune line offered 699 runes is refused and reported,
  a 56 rune line offered 134 runes is accepted, and a 2 rune line offered 12 runes
  is accepted. Verify `go test ./internal/ocrmerge/` passes.
- [x] 7.3 Add a fixture test that page 116 no longer renders the run-on piece.
  Verify the merged line for `dualidad de ser y no ser.` holds the v6 text and the
  dropped text of the page grows by that piece.
- [x] 7.4 Re-run the whole book with the tool and record the new counts. Verify the
  accounting still closes at zero and that only the one piece moved from rendered
  to dropped. Book 3292 now reports 3839 lines from VL, 248 from v6, and 2390 runes
  dropped, against 183823 VL runes and a leak of zero.
- [x] 7.5 Update the golden text of page 10 if the gate changes it, and confirm the
  rest of the suite. Verify `go test ./...` passes.
