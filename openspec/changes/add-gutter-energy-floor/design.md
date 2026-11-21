## Context

See proposal.md — Why.

`_gutter` in `sidecar/ownocr/split/gutter.py` resizes an image to 800 columns,
builds an energy map, and picks the strongest column of the map inside the band
from 0.30 to 0.70 of the width. Three tests then accept or reject the peak:
`MIN_PROMINENCE`, `MIN_COVERAGE` and `MIN_BANDS`.

All three are relative. `prominence` divides the peak by the median of the
search band, so a peak of any absolute size passes over blank paper. Page 41 of
book 6 scores 88.5, nearly thirty times the threshold, on a peak that carries
0.3 percent of the strongest column of its own page. Coverage counts the bands
that yield a centroid, and a block of text yields one in every band it covers:
both broken pages of book 6 pass at exactly 10 bands of 20, the minimum. No test
asks how strong the peak is.

Measurements below come from 181 pages of books 1, 3, 5 and 6, read from the
running installation. Books 1 and 3 are photographs. Books 5 and 6 are flat
scans.

## Goals / Non-Goals

**Goals:**

- Reject a peak too weak to be a gutter, with one scale free measure.
- Keep every correct answer of the 181 measured pages.
- Leave the shape of the answer and every caller untouched.

**Non-Goals:**

- Finding a gutter a flat scan does not record. Book 6 shows no gutter anywhere:
  a hard contrast stretch of its middle strip is blank paper, and the median
  brightness is 254 across the full width. The middle column rule answers those
  pages, and this change only stops a false gutter from overriding it.
- The accuracy of a peak that passes. Page 1 of book 6 carries the strongest
  peak of the book, 40.9, and still cuts 6.7 percent off the middle, because it
  locks onto the edge of the spine lettering rather than its centre. A floor
  rejects a weak peak; it does not move a strong one.
- Page 61 of book 1, a photograph of a tilted spread cut at 0.691 of its width.
  Its peak is 2.6, above any safe floor, and the middle column crosses its text,
  so neither rule answers it. It is a separate fault.

## Decisions

### Measure the peak in grey levels per row

The measure is the peak column of the rotated energy profile divided by the
height of the work image. The energy map holds an absolute deviation in grey
levels, and the profile sums it down a column, so the quotient is the mean
deviation per row at that column. It does not change with the size of the source
image, because every image is resized to 800 columns first.

Separation over the 181 pages:

| | the three broken pages | every other detection |
|---|---|---|
| grey levels per row | 0.129 to 0.164 | 0.413 to 92.1 |

Alternatives considered:

- **The peak against the strongest column of the image.** Rejected. The
  strongest column is the dark scanner border on both broken pages of book 6:
  column 783 scores 10292 on page 5, and column 0 scores 5064 on page 41. The
  denominator is an artefact, so the ratio measures the scanner, not the page.
- **A saturated search angle.** Rejected. It catches one of the three broken
  pages and rejects two correct ones. Pages 50 and 63 of book 1 saturate at 6.0
  degrees with peaks of 14.0 and 9.5 and cuts within 2 percent of the middle,
  while page 5 of book 6 settles at 3.5 degrees and is broken. Saturation
  follows a weak peak; it does not mark one.
- **Rejecting a gutter that crosses ink.** Rejected. A gutter is dark. The
  existing fixture in `test_split.py` draws it at grey 90 against paper at 235,
  darker than its own ink threshold of 180. The test rejects a real gutter at
  every threshold tried, from 120 to 200.

### Set the floor at 0.5 grey levels per row

The safe window has a floor and a ceiling, and both come from the corpus:

- **Above 0.164.** That is the strongest of the three false gutters, on page 24
  of book 5. A floor at or below it leaves the fault in place.
- **Below 2.199.** That is page 28 of book 6, whose middle column passes within
  3 pixels of ink. Its detection is the only thing that keeps the cut off the
  text, so a floor above it would break a page that works today. At 0.5 that
  page is untouched: it stands 4.4 times above the floor and keeps its cut of
  1483 and 1429. The number bounds how high the floor may ever go.

0.5 sits inside a window of 13.4. It stands 3.0 times above the strongest false
gutter and 4.4 times below the weakest detection the middle cannot replace. The
geometric centre of the window is 0.60, so 0.5 is close to central and leans to
the safe side.

The floor rejects 3 of the 34 detections of book 6 and 1 of the 24 of book 5.
Three of those four are the broken pages, and they move by 14.6 to 18.2 percent
of the width. The fourth is page 47 of book 6. Its outer column moves by 3.1
percent, which is 2.5 percent measured at the centre of the curve, and its
middle column stands 49 pixels clear of ink. Books 1 and 3 keep every detection:
their weakest peaks are 2.6 and 22.9.

Alternatives considered:

- **A floor near 0.25**, just above the broken pages. Rejected as too close to
  them. The corpus holds one strong false gutter and it may not be the strongest
  such page in the library.
- **A floor near 2.0.** Rejected. It reaches page 28 of book 6 and the margin to
  the ceiling disappears.

### The floor never leaves a page whole

A rejected candidate returns `None`, and `_middle` answers every landscape
image. All four pages the floor rejects are landscape: three at a ratio of 1.295
and one at 1.240, against a threshold of 1.2. Each therefore still cuts in two,
at the middle column instead of the false gutter.

Over the 181 pages, exactly one stays whole before the change and the same one
after it: page 1 of book 1, a cover, which fails the coverage test today and is
portrait besides. The floor moves no page between cut and whole.

### Keep the existing tests

`MIN_PROMINENCE`, `MIN_COVERAGE` and `MIN_BANDS` stay. They reject 28 of the 62
pages of book 6 before the floor sees them, and the spec still rests the
decision on coverage as well as strength.

## Risks / Trade-offs

- **The middle column is not a safe fallback everywhere.** On 92 of the 181
  pages it lands directly in ink, because a photographed spread is rarely
  square to the camera. Rejecting a detection is therefore not free, and the
  floor must stay low. → 0.5 rejects nothing that the middle cannot replace,
  and the ceiling of 2.199 is recorded so a later book can be checked.
- **The corpus holds four books and two scanners.** → The floor is absolute, so
  a book with a fainter gutter than book 6 would lose its detection and fall to
  the middle. That is the safe direction for a flat scan and the wrong one for a
  photograph. None of the 68 photographs of book 1 comes within a factor of 5 of
  the floor.
- **Three pages already hold copies cut at the wrong column.** → The page cut
  action corrects them. See the migration plan.

## Migration Plan

1. Deploy the sidecar. No backend, frontend or database change goes with it.
2. Correct the affected pages of the split copies with the page cut action,
   `POST /books/{id}/pages/{index}/cut`. It re-cuts one page out of the bytes of
   its source page, so the wrong halves are replaced, not edited.

   | source page | copy | copy pages | stored columns | correct column |
   |---|---|---|---|---|
   | book 6 page 5 | book 20 | 9 and 10 | 1932 and 1725 | 1465 |
   | book 6 page 41 | book 20 | 81 and 82 | 1997 and 1677 | 1465 |
   | book 5 page 24 | book 10 | 47 and 48 | — | 1503 |

   The source pages of book 6 are 2930 columns wide, and page 24 of book 5 is
   3007. The correct column is the middle of the source in each case.
3. Re-running the split action instead would build a fresh copy book and leave
   the current copy behind. The page cut action touches three spreads, so prefer
   it.
4. Rollback is the previous sidecar image. Nothing stored depends on the floor.
