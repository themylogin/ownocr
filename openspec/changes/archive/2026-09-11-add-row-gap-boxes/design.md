## Context

See proposal.md for the fault and the measure. This records how the three tests
were arrived at, because two of them look arbitrary and one was found by breaking
something.

The cut partitions a block across the boxes it was given. Where the box engine
covered a printed row completely, that is exactly right. Where it left ink
between two boxes, the partition has nowhere to put the text of that ink but the
box before the gap.

The filler already places a box for a printed row the box engine missed whole.
This is the same fault one step smaller: part of a row rather than all of it.

## Goals / Non-Goals

**Goals:**

- Put the text of uncovered ink in a box over that ink.
- Keep the accounting invariant: the box and the gap together hold the piece
  character for character, so the pieces still partition the block.

**Non-Goals:**

- No change to the cut, the alignment, or the row grouping.
- No box over a gap at the end of a row. A row ends where its last box ends, and
  there is no second box to measure against.
- No reading of what the text says. The merge holds no alphabet, so leader dots
  and letters are alike to it, and every test here is geometry or a count.

## Decisions

### Divide the piece rather than re-cut the block

The first idea was to insert a member with no text into the row and let the
alignment place the gap's text itself. It cannot. A member with no text consumes
no runes, so the alignment has no anchor for it and hands it a piece of zero
length; the gap's text stays with the box before it.

So the division happens after the cut, on the piece, by width. That mirrors
`shareOut`, which already divides a piece between a box and the filled lines
under it the same way.

### The gap must be no wider than the box beside it

This is the test that was missing, and its absence broke book 18.

Dividing by width assumes the ink runs at one density across the box and the gap.
The contents page of book 18 breaks that assumption: its gaps are rows of leader
dots, sparse ink over as much as 1011 pixels beside a box of 189. Dividing 12
characters by width there gives the box 2 and the gap 10, so `Notas` came out as
`No` in the title box and `tas .....` in the dots.

The first attempt had only the gap-width-in-characters test and the overflow test.
It placed two boxes on that page. Tightening the overflow threshold would also
have excluded it, but only just: measured against every gap the fault sits at 1.83
and 2.00 and the worst non-fault at 1.71, which is five percent of headroom on a
constant read off two books.

Restricting to gaps the width rule can speak for opens that distance up:

```
gap no wider than its box        gap wider than its box
  2.00  page 85  `cierto`          1.71  book 18 page 4  `Notas..`
  1.83  page 85  `otro—,`          1.55  book 18 page 4  `6.— Φύσις, λόγος`
  1.23  book 18 page 4             1.30  book 18 page 4  `3.— Sobre φύσις`
  1.23  book 18 page 4
  1.17  book 18 page 4
```

1.5 now falls in a gap between 1.23 and 1.83 rather than between 1.71 and 1.83.
The width test is doing the work, and the overflow test only has to say that the
gap holds ink at all.

### The split falls mid-word on purpose

`wrapInto` never breaks a word, because it is rebuilding printed lines whose
breaks it cannot know. This is the opposite case: the ink in the gap is mid-word,
and the box over it should hold exactly the glyphs standing there. The row this
exists for reads `otro—, tiene` as `otro—,` then `tien` then `e deseos`.

### The gap box stands where its neighbours say

The row stands where its two known boxes stand, so the centre of the gap is read
off theirs, weighted by how near each one lies. The height is the shorter of the
two: a box stands taller than its ink by the rise of its line over its own width,
and the gap is narrower than either neighbour, so the shorter box is the better
guide.

## Risks / Trade-offs

**A gap that holds nothing gets a box** → The overflow test is the guard. A piece
that fits the box it was cut for says the gap held no ink, and then no box is
drawn. Over both books the test refuses every gap but two.

**The division is wrong where the ink is not even** → It divides by width, so a
gap holding one wide glyph beside a box of many narrow ones takes too much. The
width test bounds how wrong that can be, because neither side can be more than
twice the other.

**The constants come from two books** → Both are photographed paperbacks of one
shape. The end-to-end comparison against the stored merged results is what to
re-run on a new book, and it must be read page by page: the first attempt's
regression on book 18 page 4 reported identical dropped-character counts and would
not have shown up in the tool totals.

## Migration Plan

1. Add `gapBox` and `shareAcrossGap`, and call the second from `Merge`.
2. Run `backend/tools/ocrmerge -all -dry-run` and compare page by page against the
   stored merged results. Read the boxes, not the totals.
3. Run `backend/tools/ocrmerge -all` to rewrite the stored results.

Rollback is the previous binary and the same tool run. The merged result is
derived and the engine results are never touched.
