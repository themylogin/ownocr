## Context

See proposal.md - Why for the motivation.

Three pieces of this already exist and the change reads all three:

- `layout.go` groups the lines of one printed row through `rowsOf`, and already
  tells two boxes of one row from two rows.
- `layout.go` reads the slope of a line off `Line.Corners`, which
  `ocr.MergeOf` fills from the `dt_polys` of PP-OCRv6.
- `disaster.go` measures the distance from one printed row to the next, over the
  blocks whose boxes it trusts, and answers zero when it cannot.

`interpolate` in `disaster.go` already answers boxes of its own for a disaster
block, tiling the block evenly by its row count. Those boxes draw correctly, and
that is the behaviour this change generalises.

## Goals / Non-Goals

**Goals:**

- One row draws as one straight line of one size, whether the box engine read it
  whole or in pieces.
- No row draws taller than the distance to the next row.
- A flat page is left exactly as it is.
- Every bound either follows from geometry or is already measured in the package.

**Non-Goals:**

- Following the ink of a sloping row. See proposal.md - Non-Goals.
- Repairing a collapsed text engine. See proposal.md - Non-Goals.
- Changing the reader. It already derives the baseline and the glyph size from
  the box, which is why moving the box is enough.

## Decisions

### The row, not the line, is what carries geometry

The merge levels a row and not a line. A line is what the box engine reported; a
row is what the printer set. Two pieces of one row that keep their own geometry
draw apart however good each piece is on its own.

`rowsOf` already answers the grouping, and the reading order already rests on it.
Levelling reads the same grouping, so the two cannot disagree about what a row is.

### The height is the box with the slope taken off, bounded by the row spacing

A rectangle round sloping ink stands taller than the ink by the slope times the
width of the rectangle. Taking that off is the correction, and `rowCount` in
`disaster.go` already writes it:

```go
span := float64(height) - math.Abs(slope)*float64(width)
```

Measured over the fixtures, that alone does not finish the job. It brings page 90
of book 17 from 1.88 of the row spacing to 1.20 and page 85 from 1.55 to 1.28,
where a flat page stands at 0.83:

| page | slope | box | de-skewed | spacing | de-skewed, as a share |
| --- | --- | --- | --- | --- | --- |
| page 10 of book 3292 | 0.000 | 44 | 44 | 53 | 0.83 |
| page 36 of book 18 | 0.004 | 50 | 50 | 52 | 0.97 |
| page 103 of book 17 | 0.019 | 108 | 79 | 76 | 1.03 |
| page 85 of book 17 | 0.014 | 124 | 102 | 80 | 1.28 |
| page 90 of book 17 | 0.027 | 142 | 91 | 76 | 1.20 |

The row spacing therefore bounds the height as well. That bound is not a measured
constant and not a tuned one: two printed rows do not overlap, so no row is taller
than the distance to the next. Every flat page measured stands under it, at 0.81
to 0.97, so the bound never touches one.

Alternative considered: a fixed share of the row spacing for every row, which is
what `interpolate` does. Rejected because the share a flat page shows runs 0.81
to 0.97 across two books, so pinning every row to one value would shrink the text
of a book that sets its rows tight.

Alternative considered: the corners alone, as the thickness of the quad. Rejected
because PP-OCRv6 pads a quad: on page 85 its quads are 110 tall where the rows
stand 80 apart, so they overlap each other.

### The baseline comes from the corners, read at the middle of the row

The bottom edge of the quad of a line is its baseline plus its descenders. A row
takes that edge from every piece it holds, and reads it at the horizontal middle
of the whole row. A row read in pieces therefore lands where its widest piece
says, because that piece carries most of the edge.

Reading at the middle rather than at an end is what keeps a levelled row over its
own ink: a horizontal box on a sloping row is right in the middle and wrong at
both ends, and the error at each end is half the slope times the width.

### A row the page cannot measure keeps its boxes

Levelling needs the row spacing of the page, and `measure` answers zero for a page
it cannot read. A row whose lines report no corners has no slope and no baseline
to read. Both cases keep the boxes the box engine gave.

That is what leaves an old result alone: a book merged before the package read
corners carries none, so it is level by definition and nothing moves.

## Risks / Trade-offs

**A levelled row stands off its ink at both ends.** A horizontal box over a
sloping row is right in the middle and out by half the slope times the width at
each end. On page 90, the worst measured page, that is 0.027 times 1750 over 2,
which is 24 pixels against a row spacing of 76. -> Accepted, and it is the
trade the reader already makes: it draws horizontal text whatever the box says.
Levelling makes the error symmetric instead of letting it fall on one end.

**The row spacing is a median over a page.** A page whose spacing genuinely
changes, such as a page carrying a footnote block set tighter, is bounded by the
median of the whole page. -> `measure` already leaves the blocks it does not
trust out of the median, which is the same guard the interpolation rests on. A
footnote row bounded a little tall draws a little large, which is what it does
today over the whole page.

**Every page of a sloping book changes.** This is not confined to the pages that
hold a broken row. -> Intended: the oversizing is on every sloping page, and the
broken rows are the part of it that was visible enough to report. The re-merge
tool rewrites a book from stored engine results, so the cost is one command and
no GPU time.

**A stored result cannot say which boxes were levelled.** `Estimated` marks a line
the merge placed, and a levelled line is not that. -> Accepted. The merge answers
the count of levelled rows, so a caller can see that levelling ran without a flag
on every line.

## Migration Plan

1. Ship the package change. New OCR runs level as they merge.
2. Run `backend/tools/ocrmerge -all` to rewrite the books already read. It reads
   the stored PP-OCRv6 and PaddleOCR-VL rows and writes only the merged row.
3. A book merged before corners were read carries none and is left as it is. Such
   a book is re-read, not re-merged, and that is out of scope here.

Rolling back is reverting the package and running the tool again.

## Open Questions

- Whether the reader should stop deriving the glyph size from the box height at
  all, and take it from the row spacing of the page instead. That would make the
  size independent of how tightly an engine draws, but it needs the page to carry
  its spacing. Levelling does not depend on the answer.
