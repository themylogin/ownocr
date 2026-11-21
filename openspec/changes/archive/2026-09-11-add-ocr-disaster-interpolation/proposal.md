## Why

A photographed page curls near the spine. Where it curls, PP-OCRv6 reports line
boxes that slope across the page, and the merge reads them as axis-aligned
rectangles. A box then stands taller than the space between two printed lines,
and its top edge names the wrong row.

The merge orders rows by that top edge. Above a certain slope the order it
derives is wrong, and the text of one printed row lands in the box of another.

Page 87 of book 17 shows it. PP-OCRv6 reports 14 boxes for 8 printed rows and
misses one row whole. Six boxes take the text of a row two rows away:

| box holds, in print | width | text the merge gave it | agreement |
|---|---|---|---|
| `cuestiones` | 211 | `expresa.` | 0.57 |
| `expresa.` | 208 | `hemos h` | 0.25 |
| `hemos` | 189 | `as de` | 0.12 |
| `nas la` | 171 | `tuvo` | 0.20 |
| `tuvo y e` | 154 | `n la A` | 0.67 |

Nothing refuses this. Every fragment is shorter than 20 characters, so the
agreement test waives itself, and each piece is short enough to pass the length
test. The page merges, reports no dropped text, and reads as nonsense.

PaddleOCR-VL reads the same region correctly, in one block, with a polygon that
follows the slope. Where PP-OCRv6 cannot say where the lines are, the merge
should stop asking it.

## What Changes

- The merge measures, for every block, how far the top edge of a full-width box
  can lie. It calls the block a disaster when that error reaches one line pitch
  and the text engine read about as much text as the box engine did.
- For a disaster block the merge discards the boxes of the box engine, splits the
  text of the block into the number of lines the geometry says, and places those
  lines in boxes evenly spaced down the block. The new lines span the block.
- The merge derives the line count two ways and demands they agree: from the
  characters of the block against the characters a printed row holds, and from
  the height of the block against the line pitch, corrected for slope. It fills
  nothing when they disagree.
- Both measures come from the blocks of the page the merge did not call a
  disaster. A page with no such block gets no interpolation.
- The merge reports the count of boxes it discarded, so a page never loses a box
  in silence.
- **BREAKING**: the merge can answer fewer lines than the box engine read. The
  current spec forbids removing a line.

### Non-goals

- No repair of a block below the threshold. Page 85 of the same book breaks one
  row of 22 and merges the other 21 correctly. Replacing its boxes would discard
  29 correct boxes to mend 2. That page wants a different fix, stated below.
- No change to the box model. The interpolated boxes stay axis-aligned, so they
  lie level across a block whose ink slopes. The text is right and the boxes are
  level. Following the slope needs a parallelogram, which is a wider change.
- No change where the text engine is the broken reading. Book 18 holds four
  blocks where PaddleOCR-VL collapsed into repetition or into nothing. The merge
  already keeps the box engine text there, and this change leaves all four alone.
- No new engine call, no new stored result. The merge stays derived from the two
  engine results the database holds.

### The measure

`lie = slope * block width / line pitch`, the line pitches of vertical error the
axis-aligned box of a full-width line carries. At one pitch the top edge of a box
names a different printed row, so ordering rows by that edge cannot be right.
The threshold is physical, not fitted.

Measured over the 249 text blocks of books 17 and 18:

```
p50  0.18        blocks at or above 0.50 : 11
p90  0.41        blocks at or above 0.75 :  3
p95  0.49        blocks at or above 1.00 :  1   <- page 87 block 1, at 2.12
p99  0.75
max  2.12        next highest: 0.85, and it merges cleanly
```

The second leg is `fill`, the characters other than whitespace of the block
against those of its box engine lines. Over the 283 blocks that merge with no
weak piece, `fill` lies between 0.97 and 1.32. The four collapses of book 18
score 0.08, 0.33, 1.75 and 44.86. The gate keeps them out.

Both legs together fire on one block of 249: page 87 block 1, at `lie` 2.12 and
`fill` 1.02.

## Capabilities

### Modified Capabilities

- `page-text-merge`: the merge may answer fewer lines than the box engine read,
  and it decides when the boxes of a region are unusable. Two requirements
  change. The requirement that the merge adds, removes and reorders no line now
  states the exceptions. The requirement on reading order gains the case where
  no order can be derived. Two requirements gain the disaster path: the
  accounting, and a new requirement for the criterion and the interpolation.
- `book-ocr`: the requirement that a merged result holds one line for every line
  of the `v6` result. That sentence is already wrong, because the line filler
  adds lines for rows the box engine missed, and no spec records it. This change
  corrects the sentence for both causes.

## Impact

- `backend/internal/ocrmerge`: `merge.go` gains the classifier and the
  interpolation. `fill.go` supplies `wrapInto` and `box`, which both already do
  what the interpolation needs. `layout.go` supplies the geometry.
- `ocrmerge.Counts` gains a field for the discarded boxes. `ocr.MergedText` and
  the merged result stored in `page_ocr.data` follow from it.
- `ocrmerge.Line` must carry the quad PP-OCRv6 reports, not only the
  axis-aligned box. `ocr/merge.go` reads `raw.dt_polys` today and drops it. The
  slope cannot be measured without it.
- `backend/tools/ocrmerge`: rewrites the stored merged result of a book from the
  two engine results, so no page needs reading again.
- The reader draws whatever the merged result holds, so it needs no change.

### The page this change does not fix

Page 85 of book 17 breaks one printed row of three boxes. Two of them share
48 pixels of a 130 pixel column, which is 0.37 of the narrower box against a
`maxRowShare` of 0.25. The row splits, one box takes `pero no en` on top of its
own text, and two boxes keep their box engine text. The page reads the words
`pero no` twice.

Ordering rows by the quad baseline, projected to the left edge of the block,
recovers 20 of its 22 rows and pairs almost every fragment correctly. The same
method fails on page 87, where the fragments report slopes of -0.27 to -0.37
against a page slope of -0.086, and the gaps within a row overlap the gaps
between rows. That is the evidence that page 87 needs this change and page 85
needs another one.
