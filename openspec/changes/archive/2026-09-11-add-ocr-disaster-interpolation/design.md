## Context

See proposal.md for motivation. What follows is the shape of the merge today and
what this change has to move.

`ocrmerge.Merge` allocates `result.Lines` with one entry per box engine line and
writes into it by that index. `spliceIn` then lays the filled lines around the
lines they belong to. Nothing in the pipeline removes a line, and the index is
the only link between a result line and the input line it came from.

`ocrmerge.Line` carries a `Box`, which is an axis-aligned rectangle. The PP-OCRv6
result holds the corners of each line under `raw.dt_polys`, and `ocr/merge.go`
reads only `lines[].bbox`. The slope of a line is therefore not available to the
merge at all today.

The line filler in `fill.go` already holds the two pieces the interpolation needs.
`wrapInto(text, n)` reads a column width back from a line count and answers the
text as exactly `n` lines. `box(block, centre, height)` answers a line box that
spans a block. Neither needs a change.

The filler is also unspecified. `page-text-merge` says the merge adds no line,
while the code has added `Estimated` lines since the `add-ocr-line-merge` change.
This change corrects that sentence because it has to touch it anyway.

All measurements below come from the 249 text blocks of books 17 and 18 in the
development database, which is a copy of production. Book 17 is 135 pages of a
photographed paperback with a curling spine; book 18 is 48 pages of a flatter
copy with polytonic Greek.

## Goals / Non-Goals

**Goals:**

- Detect the one failure the merge cannot see: box engine geometry that gives no
  valid reading order, accepted silently because the fragments are short.
- Leave every block the merge handles correctly today exactly as it is.
- Keep the accounting invariant: no character of the text engine lost in silence,
  and now no box discarded in silence either.

**Non-Goals:**

- No rotated or skewed line box. `Box` stays `[4]int`.
- No change to the alignment, the cut, or the agreement and length gates.
- No second pass over a page and no new engine call.

## Decisions

### The criterion is geometric, and the second leg names the broken engine

The first leg measures the one thing that actually breaks: two boxes of a single
printed row whose top edges the slope has driven apart.

A rectangle drawn round a sloping line reaches the highest point of that line, so
its top edge stands above the ink by however far the line rises from that point
to its own end. Two boxes covering different stretches of the column therefore
carry different errors. For a pair standing beside each other, the difference is

```
lie = |slope| * |highest(a) - highest(b)| / row pitch
```

where `highest` is the box's right edge when the lines rise to the right and its
left edge when they fall. `rowsOf` groups by the top edge, so once that difference
reaches one pitch the two boxes sit a whole row apart in a grouping that should
have put them in one. The threshold follows from what the number means; nothing is
fitted to a sample.

**Only boxes standing beside each other are compared.** This is the part that took
three tries to get right, and it is the whole criterion. A first attempt measured
`|slope| * block width / pitch`; a second measured `|slope| * (block width -
narrowest box) / pitch`. Both fire on shapes that merge correctly:

| block | shape | width leg | spread leg | beside leg | merges? |
|---|---|---|---|---|---|
| 17/87 blk 1 | 8 rows broken into 14 boxes | 2.12 | 1.66 | **1.66** | no |
| 17/103 blk 3 | 3 whole rows, slope 0.066 | 1.28 | 0.07 | **0** | yes |
| 17/102 blk 4 | footnote of 2 rows | 2.72 | 0.67 | **0** | yes |
| 17/90 blk 3 | footnote ending `nos puntos.` | 1.05 | 1.31 | **0** | yes |

A block whose every printed row is one box has every box displaced by the same
amount, so its order holds however far it slopes — page 103 slopes 1.34 pitches
across its own width and reads correctly. And a paragraph's last line is a short
box in a block of long ones, exactly like the head of a broken row, which is why
the spread leg could not tell them apart. The column separates them cleanly: a
broken row's head shares no column with its own tail, and a paragraph's last line
shares its whole column with the line above it.

The row pitch comes from the whole page, not from the block under test. Page 102's
footnote has two rows and therefore one step, and that step measures 24 pixels
where its print stands 60 apart. One step is no measurement.

`lie` alone cannot say which engine to trust. The second leg does that. `fill` is
the characters other than whitespace of the block against those of its box engine
lines. Over the 283 blocks that merge with no weak piece, `fill` lies in
[0.971, 1.317]. The four collapsed text engine blocks of book 18 score 0.08,
0.33, 1.75 and 44.86.

The gate `fill` in [0.7, 1.4] is what keeps book 18 untouched. Two engines that
read the same pixels and report the same amount of text disagree only about
placement, and `lie` says the box engine is the side that cannot place it.

### What the implementation was measured against

The criterion was run through the real merge over all 183 pages of books 17 and
18, and the answer compared against the merged result the previous code stored for
each one.

```
183 pages    1 page changes: book 17 page 87, 14 boxes discarded, 8 rows drawn
             182 pages byte-identical to the stored output, block for block
             book 18: no page changes, no box discarded
```

That comparison is what each of the three criteria above was judged on. The width
leg changed 3 pages, the spread leg changed 2, this one changes 1.

### Alternatives measured and rejected

**Box height against quad height.** The obvious measure of how much a rectangle
lies about its line. As a regional median it gives 1.96 on the disaster region and
about 1.6 on healthy regions of the same book. It does not separate, because a
wide box inflates with modest slope and the merge tolerates that fine.

**Rows against boxes.** A block holding more boxes than `rowsOf` found rows has a
broken row in it, which is close to the truth and rests on the wrong thing: the
row count is derived from the very top edges the criterion has just judged
unreliable. Page 87 reports 14 boxes over 11 rows only because the grouping
happened to pair three of its fragments; had it paired none, the counts would
match and the block would pass.

**Weak piece rate.** The share of members whose piece agrees with its line on less
than 0.30. Sharp in one direction: 283 of 293 blocks score zero. But it scores
0.51 to 0.92 on book 18's collapses and 0.21 on the disaster, so it ranks the
wrong pages first. It also cannot be read before the cut, and it needs `fill`
anyway to name the broken engine. Kept as evidence, not as the criterion.

**Alignment inversion rate.** Locate each member's text in the block
independently, then count the pairs whose position goes backwards in reading
order. This is the most direct measure of the failure and the cleanest signal
found: 0.00 for 188 of 190 blocks in book 17, and 0.40 on the disaster. Rejected
on cost. It needs one local alignment per member against the whole block, and it
still needs `fill`, so it buys nothing the geometry does not already give.

**Fragmentation on its own.** Narrow boxes at the left margin, counted. Fires on
12 blocks, two of them in book 18 and legitimate. Book 17 page 88 reports 34
boxes over 25 rows with 11 short fragments and merges correctly. Fragmentation is
what `rowsOf` exists for, so on its own it is not a fault; it is a fault only when
the slope has moved the fragments a whole row.

### The line count needs two estimates that agree

Neither estimate is safe alone.

```
from text  : 453 runes / 56 runes per printed row      = 8.02
from span  : (765 px - 144 px of slope) / 81 px pitch  = 7.66      -> both say 8
naive span : 765 px / 81 px                            = 9.44      -> wrong
```

The text estimate overstates a run that ends a paragraph, because the last printed
row is short. The span estimate is worthless without the slope correction, as the
naive figure shows. Demanding the same integer from both is the same discipline
`fillPiece` already uses when it answers nil rather than guess a row count.

### The reference measures come from the page's healthy blocks

The row pitch of a disaster block is measured from the anchors of rows that
`rowsOf` derived, and on a disaster block those rows are wrong. On page 87 block 1
that gives 68 px against the 81 px the rest of the page measures.

So both the pitch and the characters a printed row holds come from the blocks of
the same page that the merge did not call a disaster. A page with no such block
gets no interpolation. The alternative, a per-book or per-document pitch, was
rejected: a photographed page curls differently at every opening, and page 87
shows the pitch changing across one page.

Classification needs a pitch, and the reference needs the classification, so the
cycle has to break somewhere. It breaks on a provisional pitch measured over every
block of the page, disaster blocks included. A block whose rows are wrong reports
steps shorter than the page, and a handful of short steps does not move a median
that rests on every block: page 87 measures 77 px that way against the 81 px its
healthy blocks alone give, and both are far from block 1's own 68 px.

So the pass order is: rows for every block, then a provisional page pitch, then
the classification, then the reference on the survivors, then the interpolation.

### Removal is a rewrite of the line list, not a deletion in place

`result.Lines` is indexed by box engine line index for the whole of `Merge`, and
`added` keys off that index. Removing entries mid-pass would invalidate both.

So the disaster blocks are collected during the pass and applied in `spliceIn`,
which already rebuilds the list. It gains a set of indexes to drop and, for each
disaster block, the lines to put where its first member stood. The box engine
lines keep their order, and the interpolated lines stand where the block stood.

### The scope is one text engine block

The text engine's layout detection already isolates the curled top region of
page 87 as its own block, with a polygon that follows the slope. Block 2 of the
same page scores `lie` 0.60 and merges correctly. So the block boundary is already
the boundary of the disaster, and no run detection is needed.

A run of rows inside a block was considered. It is strictly more capable, and it
is what page 85 would want. It also needs trustworthy piece boundaries at both
ends of the run, which a block like page 87 block 1 does not have anywhere.
Deferred to the change that addresses page 85.

### The quad reaches the merge as the corners of a line

`ocrmerge.Line` gains the four corners PP-OCRv6 reports. `ocr/merge.go` reads
`raw.dt_polys` alongside `lines[].bbox` and passes both. The merge derives the
slope as the median over the block's full-width lines of the slope of the bottom
edge.

The median over full-width lines only. A narrow fragment reports a wildly wrong
slope: on page 87 block 1 the fragments report -0.27 to -0.37 against a page slope
of -0.086, because the detector fitted a steep quad to a 200 pixel box.

A line whose corners are missing keeps its box, and its slope counts as zero. A
result stored before this change holds no corners, so such a page classifies as no
disaster and merges exactly as it does today.

## Risks / Trade-offs

**The interpolated boxes are level across a block whose ink slopes** → The text is
right and in reading order, which is the fault being repaired. The boxes are level
because `Box` is axis-aligned. On page 87 the ink slopes 4.9 degrees over 1678
pixels, so a box drawn level stands up to 72 pixels from its ink at the ends.
Accepted, and named in Open Questions.

**Greedy re-wrap puts a hyphenated word in the wrong box** → The text engine
de-hyphenates, so `wrapInto` cannot rebuild the printed breaks. On page 87 the
first and last of the 8 lines match the print exactly and the middle lines drift
by a word or two. This is the trade-off `wrapInto` already documents. It is a
large gain over six boxes holding another row's words.

**A block may own a line that belongs to its neighbour** → On page 87 the two text
engine blocks overlap vertically, and `groupLines` gives block 1 the first line of
the next paragraph. Interpolating block 1 discards that box. Today that line
already carries the wrong text, so nothing correct is lost here, but the general
case can lose a correct box. The discarded count is what makes it visible.

**The threshold rests on one observation above it** → Over the 183 pages of two
books exactly one block reaches 1.0, at 1.66. The next highest that merges
correctly is page 90's footnote at 0 under this measure, so the gap is the whole
range. The threshold is physical rather than fitted, so a wrong call needs a block
with a genuinely broken row whose lie lands near 1.0. Such a block gets a wrong
reading order today as well, so the failure mode is not new. The discarded count
and the two-estimate agreement test are the guards.

**Three false positives were found by measurement, not by reading** → The width
leg and the spread leg both looked right and both threw away correct boxes. Only
the page-by-page comparison against the stored output caught page 90, whose
footnote silently fell back to box engine text and dropped 139 characters while
reporting no discarded box at all. Any later change to this criterion wants that
same comparison run again, and it wants to check the dropped text and not only
the discarded count.

**A misfire discards boxes the box engine read correctly** → Both legs must hold
and both line counts must agree. A block that passes all three and was fine is a
block where the reading order was already wrong.

## Migration Plan

1. Plumb the corners through `ocr/merge.go` and `ocrmerge.Line`.
2. Add the classifier and the interpolation, with `Counts.Discarded`.
3. Run `backend/tools/ocrmerge -all -dry-run` and read the reported counts. The
   only book that should report a discarded box is book 17, and only on page 87.
   Book 18 must report none.
4. Run `backend/tools/ocrmerge -all` to rewrite the stored merged results from
   the two engine results. No page is read again and no GPU time is spent.

Rollback is the same tool run from the previous binary. The merged result is
derived, the engine results are never touched, and the tool rewrites either way.

## Open Questions

- Should an interpolated line follow the slope of its block? The text engine gives
  page 87 block 1 a real polygon, `[[1760,79],[642,214],[368,214],[83,304],
  [94,828],[156,843],[474,783],[1760,723]]`, which traces the sloping region. Using
  it needs a parallelogram in place of `Box`, which touches the stored result, the
  API and the reader. Answerable later: it changes how the lines are drawn, not
  when the merge decides to draw them.
- What fixes page 85? Ordering rows by the quad baseline projected to the left edge
  of the block recovers 20 of its 22 rows and pairs almost every fragment
  correctly. It is a change to `rowsOf` and it needs its own measurement over the
  book. It is independent of this change: it lowers how often a block reaches
  `lie` 1.0, and it does not alter what happens when one does.
