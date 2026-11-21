## Context

See `proposal.md` — Why.

`ownocr/split/gutter.py` answers in two steps. `_gutter` builds an energy map,
searches a band of angles for the strongest vertical line, refines it over 20
horizontal bands, and fits a quadratic. `columns` turns that curve into the two
cut columns, adding a margin of 0.5 percent of the width so a leaning gutter
clips neither page.

`_gutter` returns None on three counts: the line covers less than half the
height, fewer than 6 bands hold it, or its prominence is below 3.0. `columns`
passes that None straight out, and the worker copies the page whole.

The answer travels as `[left, right]` or `null` per image. The Go worker cuts
`[0, left)` and `[right, width)`, and refuses a half with no column.

## Goals / Non-Goals

**Goals:**

- Answer a cut for a landscape image the detector finds no gutter in.
- Leave every other image exactly as it is today, including the thresholds.
- Keep the HTTP contract unchanged, so the worker needs no change.

**Non-Goals:**

- Marking a guessed cut apart from a found one, in the answer or the interface.
- Changing the coverage, band or prominence thresholds of the detector.
- Guessing where the gutter is beyond the middle, or cutting a portrait image.
- Deciding orientation from the book rather than the image.

## Decisions

### The fallback lives in `columns`, after `_gutter` gives up

`columns` already holds the step from curve to columns, and it is the one place
both `cut_columns` and a direct call pass through. A caller therefore cannot
reach the detector and miss the fallback.

The alternative puts it in `cut_columns`, beside the file read. That leaves
`columns` answering None for an image `cut_columns` would cut, which is two
answers for one image.

### Landscape is `width >= 1.2 * height`, on the stored image

A new constant, `LANDSCAPE_RATIO = 1.2`, sits with the other thresholds. The
test reads the shape of the image as it arrives, not the working copy: the
resize to `WORK_WIDTH` keeps the ratio, so either reads the same, and the
original is what the caller sees.

1.2 was chosen over `width > height`: a page photographed with a little of the
desk on each side is wider than tall by a few percent, and cutting it would be
wrong. It was chosen over 1.4 because a tightly cropped spread of two portrait
pages can fall below that.

### The middle is one column, taken as `width // 2`

The fallback answers `(half, half)` with `half = width // 2`. The left page is
`[0, half)` and the right page is `[half, width)`, which is every column once.

A found gutter takes a margin because the gutter leans and bows, and a crop
takes a column. A guess at the middle has no curve to follow, so a margin would
only put the same strip in both pages.

The smallest landscape image is 2 by 1, which gives `half = 1` and two pages of
one column. The worker's refusal of an empty half is therefore unreachable
through this path.

### The answer says nothing about being a guess

The pair carries no mark, so the worker, the API and the interface stay as they
are. A wrong cut is already recoverable: the user deletes one half and moves the
cut of the other out to the full width, which the page menu now offers.

Marking it would mean a third answer shape, a column on `book_page`, and a state
in the interface for a page that is "probably wrong". None of that is needed to
undo one bad cut on one page.

## Risks / Trade-offs

- **A landscape single page is now cut in two.** A wide diagram, a map, or a
  book in landscape format loses to this rule. → The 1.2 threshold keeps a
  near square page whole, and the page menu can put a cut page back to full
  width. The user asked for this trade explicitly.
- **A landscape cover of a book of spreads becomes two pages.** → The remedy is
  the same, one page at a time: delete one half, widen the other.
- **The sidecar tests read as landscape.** Their canvas is 1200 by 900, and four
  of them expect null from it. → They move to a portrait canvas, which is what
  they meant by "a single page" all along. The change makes that assumption
  explicit rather than accidental.
- **A missed gutter is now silent.** Today a missed gutter leaves a spread whole,
  which is visible in the grid. Now it leaves two pages cut down the middle,
  which looks right until the reader meets a clipped line. → The middle of a
  photographed spread is close to its gutter, and the cut is one Edit away.

## Migration Plan

1. Ship the sidecar. It holds no state and answers one request at a time, so a
   restart is the whole deployment.
2. Books already split keep the pages they hold. The new rule reaches a book
   only when a split job runs for it.

Rolling back means restoring the previous `columns`. Nothing else reads the
rule, and no stored data records it.
