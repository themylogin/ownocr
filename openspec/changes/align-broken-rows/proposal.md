## Why

PP-OCRv6 draws a rectangle round every line it reads. Where a photographed page
slopes, that rectangle is not the line: it reaches the highest point of the ink
and the lowest, so it stands taller than the row by the slope times its own width.

The reader takes both the glyph size and the baseline off that rectangle. On a
sloping page it therefore draws every line too large, and the rows collide.
Measured against the page's own row spacing:

| page | slope | box height | rows stand apart | box as a share of the pitch |
| --- | --- | --- | --- | --- |
| page 10 of book 3292 | 0.000 | 44 | 53 | 0.83 |
| page 85 of book 17 | 0.014 | 124 | 80 | 1.55 |
| page 90 of book 17 | 0.027 | 142 | 76 | 1.88 |

A box 1.88 times the distance to the next row cannot be one row.

The same rectangle breaks a printed row the box engine read in pieces. The merge
reads such a row in the right order already, and page 85 runs correctly from
`durante el` through `viaje;` to `etcétera. Incluso`. It does not draw correctly,
because each piece carries its own rectangle and a sloping row makes them differ:

```
"viaje;"                  box [112, 375,  261, 461]   height  86
"etcétera. Incluso su..." box [231, 336, 1735, 459]   height 123
```

Those two pieces of one printed row draw 33 pixels apart at sizes half again
different, and they collide.

The cure is already on the page. The top block of page 87 is a disaster block, so
the merge answered boxes of its own that tile the block by its row spacing. That
block draws as straight lines of one size while the rest of the same page
staggers.

## What Changes

- The merge levels every row it answers onto the row grid of its page. A row
  takes one baseline, read from the corners the box engine reported, and one
  height. Each line keeps its own left and right edge.
- A row's height SHALL be the height of its box with the slope taken off, and
  SHALL NOT exceed the distance from one printed row to the next. Two printed
  rows cannot overlap, so the row spacing bounds it.
- The pieces of a printed row the box engine broke share that one baseline and
  that one height, so a broken row draws as one straight line.
- A flat page is untouched: its boxes already stand at 0.81 to 0.97 of its row
  spacing, which is under the bound.
- **BREAKING**: the merge gains a third case in which it answers a box the box
  engine did not report. The current requirement allows two.

## Capabilities

### Modified Capabilities

- `page-text-merge`: *A merge pairs the lines of a box engine with the blocks of
  a text engine* allows the merge to level the box of a row, alongside the two
  cases it already allows. A new requirement states what levelling answers, what
  bounds it, and when it refuses.

## Impact

- `backend/internal/ocrmerge`: `layout.go` already groups the lines of a printed
  row and already reads the slope off the corners; `disaster.go` already measures
  the row spacing of a page. The levelling reads all three and changes only the
  boxes it answers.
- Stored results hold the boxes they were merged with. `backend/tools/ocrmerge`
  rewrites a book from the engine results it already holds, so no page goes to
  the OCR endpoint again.
- A result stored before the merge read corners carries none. Such a line is
  level by definition and keeps its box, so an old book is left as it is.
- No change to the reader, to the API, or to any stored image. The reader draws
  what the box says, which is why the box is the whole of the fix.

## Non-Goals

- The text engine collapsing on a block. Rows 9 to 13 of page 87 hold a garbled
  false start that PaddleOCR-VL itself wrote, and no geometry repairs it. It
  shows on 8 of 237 measured pages and needs its own change.
- Drawing a line that follows the ink. A box is a rectangle; a row that slopes
  needs a shape that is not one. Levelling makes a row straight and legible, not
  slanted.
