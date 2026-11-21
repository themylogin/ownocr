## Why

Two boxes of page 85 of book 17 hold twice the text they were drawn around:

```
box [119, 876, 273, 959]  is 154 px wide and reads `cierto` in print
                          and held `cierto es qu`, 12 characters in room for 6

box [112, 729, 275, 817]  is 163 px wide and reads `otro—,` in print
                          and held `otro—, tien`, 11 characters in room for 6
```

PP-OCRv6 drew each of those printed rows as two boxes and left ink uncovered
between them: 145 pixels holding `es qu`, and 118 pixels holding `tien`. The cut
partitions a block across the boxes it was given, so the text of the uncovered
stretch goes to the box before the gap. That box then holds more than its ink, and
a renderer squeezes it in.

Nothing is lost, and the text still reads in order. It reads in the wrong place.

## What Changes

- Where two boxes of one printed row leave ink uncovered between them, the merge
  draws a box over the gap and gives it the text that stands there. The box before
  the gap keeps only what it was drawn around.
- The new box names the text engine, carries no score, and is marked as a line the
  merge placed itself, like the lines the filler places for a row the box engine
  missed.
- Three tests all have to hold. The gap has room for at least 2 characters. The
  gap is no wider than the box beside it. And the piece of that box holds at least
  1.5 times what the box was drawn around.

### Non-goals

- No change to the cut, the row grouping, or the gates that refuse a line.
- No box over a gap at the end of a row. A row ends where its last box ends, and
  the merge has no second box there to measure a gap against.

### The measure

Over the 183 pages of books 17 and 18, 31 pairs of boxes of one printed row leave
a gap at all. The three tests together fire on 2 of them, both on page 85.

The second test is what makes the third one safe. The merge divides the text
between box and gap by width, which holds only where the ink runs at one density
across both. Ranked by how far a piece overruns its box:

```
gap no wider than its box          gap wider than its box
  2.00  page 85  `cierto`            1.71  book 18 page 4  `Notas..`
  1.83  page 85  `otro—,`            1.55  book 18 page 4  `6.— Φύσις, λόγος`
  1.23  book 18 page 4               1.30  book 18 page 4  `3.— Sobre φύσις`
  1.23  book 18 page 4
  1.17  book 18 page 4
```

The contents page of book 18 is why. Its gaps are rows of leader dots, sparse ink
over as much as 1011 pixels beside a box of 189, and dividing that by width moves
three letters of a title into the dots. Measured against every gap the threshold
would have to sit between 1.71 and 1.83. Measured against the gaps the merge can
trust, it sits between 1.23 and 1.83.

Run through the real merge over all 183 pages and compared against the stored
merged results:

```
178 pages  byte-identical
  5 pages  changed, every one for the better
book 18    no page changed
```

Page 85 now reads each row over three boxes, each holding its own ink:

```
[112, 729, 275, 817]  otro—,
[275, 726, 393, 814]  tien        <- placed over the gap
[393, 701, 1737, 805] e deseos costosos, etcétera). No vale simplificar la
```

## Capabilities

### Modified Capabilities

- `page-text-merge`: the requirement that the merge cuts the text of a block at
  the line boundaries. The pieces still partition the block, and a piece may now
  be divided between the box it was cut for and a box the merge draws beside it.
- `book-ocr`: the requirement that a merged result holds one line for every line
  of the PP-OCRv6 result. It already named two reasons the merge adds or removes
  one, and this is a third.

## Impact

- `backend/internal/ocrmerge/fill.go`: `gapBox` and `shareAcrossGap`.
- `backend/internal/ocrmerge/merge.go`: one branch in the per-member loop, after
  the length and agreement gates and before the piece is assigned.
- `backend/internal/ocrmerge/ocrmerge.go`: two constants with their distributions.
- `Counts.Estimated` now counts a gap box as well as a filled row. Page 85 reports
  2 where it reported none.
- `backend/tools/ocrmerge -all` rewrites the stored merged results from the two
  engine results. No page is read again.
