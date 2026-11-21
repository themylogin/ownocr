## Context

See proposal.md for the fault and the measure. This records the approach that was
tried first and rejected, because it is the one a reader of `rowsOf` will reach
for next.

`rowsOf` groups boxes into printed rows by the top edge of the box. On a page
photographed with a curling spine that edge is a poor guide: a rectangle drawn
round a sloping line reaches the highest point of that line, so two boxes of one
row have tops that differ by the slope times the distance between their high ends.
The `add-ocr-disaster-interpolation` change measures exactly that difference and
gives up on a block once it reaches a whole row.

Page 85 is not such a block. Its difference is 0.42 of a row, and its boxes do
name their rows. Its row broke for a much smaller reason.

## Goals / Non-Goals

**Goals:**

- Stop one printed row splitting into two when its boxes overlap in the column.
- Change nothing on a page that reads correctly today.

**Non-Goals:**

- No new geometry. The row measure keeps both its tests and only one threshold
  moves.
- No repair of ink the box engine never covered. Page 85 keeps two boxes whose
  text runs past their ink, from 118 and 145 pixel gaps in the middle of two rows.

## Decisions

### One threshold, not a new ordering

The fault is that `maxRowShare` was too tight, and the fix is the threshold.
Nothing about the measure was wrong.

### Rejected: order rows by the quad baseline

The obvious alternative. PP-OCRv6 reports the corners of every line, so a block
slope and a projection give each box a deskewed row position that a sloping page
cannot corrupt:

```
refY(i) = bottomLeftY(i) + slope * (blockLeft - bottomLeftX(i))
```

On page 85 this looks decisive. Sorted by `refY`, the gaps separate cleanly:

```
within one printed row : 0, 4, 7, 7, 7, 8, 10, 12 px
between two rows       : 55, 58, 61, 62, 62, 63, 65, 76, 79, 82, 82, 84 px
```

It was rejected on measurement. Run end-to-end over both books at tolerances from
0.30 to 0.70 of a line height, it improves 3 blocks and breaks 3:

| block | now | deskewed |
|---|---|---|
| 17/85 blk 2 | 2 boxes keep box engine text | 0 |
| 17/87 blk 2 | 1 box keeps box engine text | 0 |
| 17/84 blk 2 | 1 box overruns its ink | 4 |
| 17/88 blk 2 | 2 boxes overrun their ink | 3 |
| 17/94 blk 2 | 1 box overruns its ink | 2 |

Page 84 shows why. Its fragments sit at the **right** of each row, not the left,
so projecting a fragment's baseline back to the block's left edge extrapolates
about 1100 pixels and multiplies every error in the slope. Every fragment shifts
up by one row:

```
box 6 covers the word `ostracismo` at the right of its row

  by box top        : box 6 -> 'ostracismo, '      correct
  by deskewed refY  : box 6 -> 'procedimiento, '   the row above
```

The same book fails in both directions, and the gap distribution over both books
bears that out: of 101 pairs that clear each other's column, 52 lie beyond 0.30
line heights, so no single tolerance separates them.

A projection to the middle of the block would halve the extrapolation for both
cases and is worth measuring if this is revisited. It was not tried here, because
the threshold answers the page this change exists for and costs nothing.

### The threshold sits at the low end of the range that works

Every value from 0.40 to 0.90 answers all 183 pages alike. That is because
`minRowOverlap` refuses two boxes that share a column unless they also stand at
each other's height, so the column test has room to give.

0.40 was chosen rather than the middle of that range. The observed maximum for two
halves of one row is 0.37, so 0.40 clears it and grants nothing more. A value
picked in the middle would rest on `minRowOverlap` catching everything else, and
that is not a property this change measured.

## Risks / Trade-offs

**Two genuine rows read as one** → Their texts concatenate into one box, which is
the worse fault of the two. `minRowOverlap` is the guard: two rows must also
overlap in height by more than half the shorter box, and over the measured books
two rows overlap by at most 0.32 of it. Nothing in either book merges.

**The constants come from two books** → Both are photographed paperbacks of the
same shape. A scanned page, or a two-column layout, could put two boxes of
different rows in the same column at the same height. The end-to-end comparison
against the stored merged results is what to re-run on a new book.

**The disaster criterion reads the same constant** → `besideInColumn` in
`disaster.go` uses `maxRowShare` to decide which pairs of boxes it compares, so a
looser share weighs more pairs and can only raise the measured lie. Page 87 is
still the only block of the two books it calls unusable, and its lie moved from
1.66 to 1.66.

## Migration Plan

1. Change the constant and its recorded distribution.
2. Run `backend/tools/ocrmerge -all -dry-run`, and compare page by page against
   the stored merged results rather than reading the totals. Three of the five
   pages that change report identical counts, so the totals hide them.
3. Run `backend/tools/ocrmerge -all` to rewrite the stored results.

Rollback is the old constant and the same tool run. The merged result is derived
and the engine results are never touched.
