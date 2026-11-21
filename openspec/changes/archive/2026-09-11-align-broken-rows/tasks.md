## 1. The levelling

- [x] 1.1 Add a `Levelled` count to `Counts` and verify `go build ./...` passes
- [x] 1.2 Read the slope and the baseline of a row from the corners of every line
  of that row, at the horizontal middle of the row, and verify a unit test that
  a row of two pieces answers the baseline its widest piece carries at that point
- [x] 1.3 Compute the height as the box height less the slope times the box width,
  bounded by the row spacing, and verify unit tests that a row 124 tall sloping
  0.014 over 1641 on a page of spacing 80 answers 80, and that a row 44 tall with
  no slope on a page of spacing 53 answers 44
- [x] 1.4 Apply the levelling to every row after the reading order is settled,
  keeping each line's left and right edge, and verify a unit test that the left
  and right edges of every line are the ones the box engine gave
- [x] 1.5 Refuse the levelling for a page with no measurable row spacing and for a
  line with no corners, and verify unit tests that both keep every box

## 2. The measured pages

- [x] 2.1 Add a golden test over every fixture that records how many rows each
  levels and why, and verify it reproduces the measured table: page0010 0,
  page0011 1, book0018page0036 16, book0017page0085 33, book0017page0087 26,
  book0017page0090 31, book0017page0102 31, book0017page0103 34, and 0 for the
  three fixtures that report no corners
- [x] 2.2 Verify a test on `book0017page0085` that the pieces of the row holding
  `viaje;` and `etcétera. Incluso su figura` answer one top edge and one bottom
  edge, keep their own left and right edges, and stand 79 tall
- [x] 2.3 Verify that a page the merge levels nothing on answers every box the box
  engine drew, so a book it has nothing to fix is not rewritten
- [x] 2.4 Verify that no line the box engine drew stands taller than the distance
  to the next printed row, on every fixture that reports corners
- [x] 2.5 Verify that the existing golden test of page 10 still passes unchanged,
  so levelling changed no text and no order

## 3. The stored books

- [x] 3.1 Report the levelled count from `backend/tools/ocrmerge`, and verify a
  dry run against a full scratch copy of the development database reports it per
  book: book 17 levels 2552 rows of 3595, book 18 levels 583 of 1376
- [x] 3.2 Run it for real and verify by eye in the reader that book 17 page 85
  draws its broken rows as straight lines of one size and its rows no longer
  collide, and that page 87 does the same below its collapsed block

## 4. End to end

- [x] 4.1 Run the whole Go suite on a scratch database and verify it passes
- [x] 4.2 Run `./run-e2e.sh ocrreader.spec.ts` and verify it passes
- [x] 4.3 Run `openspec validate --strict align-broken-rows` and verify it passes

Note: `go test ./...` truncates `book CASCADE` on the development database. Follow
the scratch-database recipe before running the backend suite, and never point the
merge tool at the development database itself.
