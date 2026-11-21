## Context

See proposal.md - Why for the motivation.

The split cuts a spread at a column. The gutter of a spread is a curve, and
`sidecar/ownocr/split/gutter.py` fits a quadratic to it. It then reports two
whole numbers: the right half begins at the leftmost point of that curve, and the
left half ends at its rightmost point. Neither half can clip its own text that
way, and the price is that both halves carry the whole bow of the fold.

On the measured book that bow is 127 to 161 pixels of a spread 4032 pixels wide.
Where the curve stands away from the extreme, those pixels hold the ink of the
page beside this one.

The pages of an OCR copy carry no cut side. A copy holds the pages of its source
book, so `book_page.cut_side` is set on the split page and null on the page of
the copy that was read from it.

## Goals / Non-Goals

**Goals:**

- One rule that both the reader and a later export obey, with the arithmetic in
  one place.
- A rule that refuses rather than guesses. A wrong trim deletes text of the book.
- Evidence for every constant, measured on real pages.

**Non-Goals:**

- Cutting the strip at the source. Masking the split along the gutter curve would
  remove the cause instead of the symptom, but it needs the curve carried out of
  the sidecar and every book split again. It is a later change.
- Repairing the broken rows of a page photographed at an angle. That is a
  different defect of the same pages, described under Risks.
- Trimming the outer edge. A note written in the outer margin belongs to the
  page.
- Changing any stored image, page size, or OCR result.

## Decisions

### The trim is derived on request, not stored

The rule reads lines that are already stored and page geometry that is already
known. Storing the answer would add a column, a migration, and a rule about when
to recompute it. A rescan of one page would have to remember to.

Deriving costs one pass over the boxes of one page, which is 53 boxes on the
widest measured page. That is far below the cost of reading the result out of the
database, which the endpoint already pays.

Alternative considered: a `trim_column` written by the OCR worker. Rejected for
the migration and the staleness, not for the cost.

### The trim covers pixels and moves none

A trim names a column. It does not change the width, the height, or any box.

Cropping the page instead would shift every stored box by the trim. The reader
would do that arithmetic, and an export would do it again, and the two would have
to agree forever. Covering the strip removes the arithmetic: a renderer draws the
image, then draws one opaque rectangle over the strip.

The page keeps a wider inner margin as a result. Page widths in the measured book
already run from 2026 to 2107, so nothing depends on them matching.

### The server drops the fragments; a client only paints

The endpoint answers the lines that survive the trim. A client never sees a
fragment and never needs the rule.

This is what makes an export cheap. A text export takes the lines it is given. An
image export draws the same rectangle the reader draws. Neither repeats the rule,
so neither can disagree with the reader about what the page says.

### Touching the gutter edge is the criterion, not shortness

The three properties the strip shows - close to the edge, short, standing in a
column - also describe a defect that must not be cut.

Pages 52 and 84 to 90 of book 17 are photographed at an angle. The detector
breaks their printed rows in half, and the left part of each broken row is short
and stands in a column against the inner edge. Page 87 holds seven of them and no
strip at all. Cutting on those three properties would delete real text on seven
pages while cleaning seven.

The cut truncates a word of the neighbouring page mid-stroke, so a fragment's box
reaches the gutter edge. A broken row begins at the margin of its own page.
Measured: every fragment has a near inset of 0 to 3 pixels; every broken row part
has one of 65 to 137.

### The corridor places the trim, not the fragments

The detector does not read a whole strip. It found 2 fragments on page 29 and 23
on page 53, while the ink of both strips runs the height of the page. A trim
placed at the last fragment it found would leave the ink it missed.

The trim is therefore placed in the middle of the corridor between the fragments
and the body of the page. The corridor is empty by definition, so its middle
covers every fragment, read or not, and still cannot reach the body.

### A new package beside ocrmerge

The rule takes lines, a width and a side, and answers a column. It reads no file,
calls no engine, and knows no alphabet - the same shape as `internal/ocrmerge`.
Keeping it out of the handler is what lets a later export call it without going
through HTTP, and what lets it be tested on fixtures of boxes alone.

### The plain half stays the photo

The OCR reader draws the plain photo beside the read page. Only the read page is
painted. A user who wants to see what was cut looks left.

## Measured constants

Every number below comes from books 17, 18 and 19 of the development database,
read through the same model fallback the endpoint uses: the merged result of a
page where it holds one, and the PP-OCRv6 result where it does not. That is 236
pages cut out of spreads, holding 7132 lines.

| constant | value | evidence |
| --- | --- | --- |
| near inset of a fragment | at most 0.005 of the width | every fragment measured 0 to 3 pixels. The nearest part of a broken row measured 65 |
| far inset of a fragment | at most 0.10 of the width | fragments reach 0.048 of the width. The nearest line that touches the edge and is not a fragment reaches 0.21, and the next 0.28 |
| a body line | wider than 0.50 of the width | it only has to stand above the fragment ceiling of 0.10, and it does by five times. Line widths run continuously through 0.50, so no value there separates two populations |
| the corridor | at least 0.03 of the width | the seven corridors measured 0.050 to 0.110 of the width |
| fragments needed | 2 | the seven strips hold 2, 4, 7, 8, 18, 20 and 23 fragments |
| the trim column | the middle of the corridor | lands at 90, 94, 109, 109, 116, 122 and 127 pixels |

Running the whole rule over those 236 pages trims 7 pages, refuses 2, and touches
no other page.

| book | page | side | width | fragments | body edge | corridor | trim |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 17 | 29 | right | 2033 | 2 | 152 | 124 | 90 |
| 17 | 31 | right | 2090 | 8 | 145 | 102 | 94 |
| 17 | 33 | right | 2038 | 7 | 193 | 155 | 116 |
| 17 | 45 | right | 2083 | 18 | 182 | 121 | 122 |
| 17 | 53 | right | 2061 | 23 | 160 | 103 | 109 |
| 17 | 55 | right | 2026 | 4 | 190 | 162 | 109 |
| 19 | 51 | left | 1545 | 20 | 212 | 170 | 127 |
| 17 | 50 | left | 2067 | 1 | none | - | refused |
| 18 | 1 | left | 1595 | 3 | none | - | refused |

## Risks / Trade-offs

**The left side rests on one page.** Six of the seven trims are right halves.
Book 19 page 51 is the one left half, and its strip was read by eye as well as by
the rule: 20 fragments against the right edge, a corridor of 170 pixels, a trim at
127. The rule is written in insets and is symmetric by construction, but one page
is thin evidence for a whole side. -> The guards refuse rather than guess, so an
untested left half gets no trim instead of a wrong one.

**The rule sees lines, not ink.** A page whose strip the detector read not at all
holds no fragment, so it gets no trim and keeps its ink. -> Accepted. The rule
cleans what it can prove is there, and a page it cannot prove is left as it is
today. Masking the split along the gutter curve would end this, and that is the
later change named under Non-Goals.

**A page with no body line gets no trim.** Book 18 page 1 holds three fragments
and no line wider than half the page, so the rule refuses it and its hallucinated
markup survives. -> Accepted. A page with no body carries no evidence of where its
own text begins, and a trim placed without that evidence could cover the page.

**A broken row could one day reach the edge.** A page skewed far enough that a
broken row starts at the gutter edge would make a fragment out of real text. ->
Two guards stand between that and a cut: the body edge bounds the trim, and the
corridor must be 0.03 of the width wide. A broken row sits inside the body, so it
narrows the corridor to nothing and the trim is refused.

**The reader and the plain half now disagree.** The read page shows a clean inner
margin and the photo beside it shows the fragments. -> Intended. It is the only
place left that shows what was cut.

## Migration Plan

None. No schema changes, no image is rewritten, and no book is processed again.
The trim appears on the next request for the lines of an affected page.

Rolling back is reverting the change. Nothing stored depends on it.

## Open Questions

- Whether an image export covers the strip or crops it. The answer reports the
  column either way, so an exporter can do either. This can be settled when the
  export is designed.
- Whether the reader should offer a way to see a trimmed page untrimmed, beyond
  the plain half beside it.
