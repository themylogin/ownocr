## Context

See proposal.md - Why. The reader draws each line as one SVG `<text>` at the
place its box names, over the page image and a white veil. `OcrPage.vue` holds
that drawing. `ReaderView.vue` holds an `IntersectionObserver` that adds a page
to a near set and asks for its lines.

A spike measured the cost of drawing every page at once. It stands at
`frontend/svg-text-spike.html`. It builds a page of the reader with the real line
boxes and the real text of the 14 sample pages in
`backend/internal/ocrmerge/testdata`, which hold 38 lines per page. It takes the
same geometry as `OcrPage.vue`: `textLength` with `lengthAdjust="spacingAndGlyphs"`,
the font size from the box height, the grounds below the glyphs.

Measured on a book of 1000 pages:

| variant | main thread blocked |
|---|---|
| load, skipped rendering, one shape per page | 124 ms |
| first `Ctrl+F` over the book | 957 ms |
| later `Ctrl+F` | about 240 ms |
| load, no skipped rendering, 100 pages | 137 ms |

A book of 1000 pages with every line drawn now loads cheaper than a book of 100
pages does without the skipping.

## Goals / Non-Goals

**Goals:**

- The browser find reaches the whole book, with no find control of our own.
- A book of 1000 pages opens as fast as the reader opens today.
- The reader keeps the drawing it has. The text stays SVG, stays at the place
  its box names, and stays selectable.

**Non-Goals:**

- A find control of our own, a result list, or a match count we draw.
- A match across a line break. The last section says why.
- A search over books. This finds inside the book the reader holds.
- A change to what OCR stores, or to how the merge picks lines.

## Decisions

### The browser finds, we do not

The browser find already reaches SVG text. It fails today only because the text
of a far page is absent from the DOM. Putting the text there is the whole fix.

The alternative is a find control of our own, as PDF.js has: search a string in
memory, jump to the page, draw the highlight from `getStartPositionOfChar`. It
matches across line breaks and costs no layout. It also means a control, a result
list, a match count, a key binding that takes `Ctrl+F` from the browser, and
highlight geometry we maintain.

The measurements decide it. 957 ms once per session, then 240 ms, buys the whole
feature for no new interface. Take the browser's.

### The page skips its own rendering while it is off screen

`content-visibility: auto` on the spread, with `contain-intrinsic-size` holding
the size the reader already computes in `sizeOf`. The browser then skips the
style, the layout and the paint of a page that lies off screen, and still lets
the find reach the text inside it.

`contain-intrinsic-size` is not optional. Without it a skipped page falls to zero
height, every page lands in the window, and every page renders. The spike hit
this: the first run wrote `auto <w> auto <h>`, which the grammar rejects, the
browser dropped the declaration, and the skipping measured the same as no
skipping at all. The grammar is `auto? [ none | <length> ]{1,2}`. One `auto`,
then the lengths.

The reader knows the exact size of a page, so nothing here is a guess. The
scrollbar keeps its length and `scrollIntoView` keeps landing where it did.

`content-visibility: hidden` is the wrong one. It skips the subtree and the find
cannot reach it.

### The grounds of a page become one shape

Today each line draws a `<rect>`. On a book of 1000 pages that is 45,000 of them.
The spike shows they, not the text, carry the cost: with the rects the blocked
time climbed and did not settle; with one `<path>` per page holding one subpath
per line it settled at 957 ms. Same text, same `lengthAdjust`, same page count.

This is not a bonus. Without it the change does not hold at 1000 pages.

The paint order rule stands: one path for the grounds of a page still draws
before any glyph of that page, so no ground covers the text of the line above.

### One request for the book

The reader cannot draw a page it has no lines for, so lazy asking has to go. 1000
requests is not the replacement. The backend answers the lines of the whole book
at once, and the reader asks once when it opens.

The answer is about 1.5 MB for a long book before compression. One compressed
response beats hundreds of round trips.

The `IntersectionObserver` then has no work left and goes with it.

## Risks / Trade-offs

**A phrase across a line break never matches.** Each line is its own SVG `<text>`
and its own layout box, so the browser does not join them. A reader who searches
`the door and then` finds nothing when `and` ends a line. Single words always
match. This is the price of the browser's find, and no arrangement of SVG text
removes it. → State it in the spec so it is a known limit, not a bug report. The
escape is a find control of our own, which the spec change does not close off.

**The spike built its DOM with one `innerHTML`, Vue does not.** Skipping the
rendering skips layout and paint. It does not skip creating about 47,000
elements, and Vue's render path is slower than the HTML parser. Every number
above is a floor. → Measure the real reader on a long book before closing the
change. If Vue's cost bites, build the contents of the two `<g>` elements as a
string and set it with `v-html`, which is what the spike measured. That needs the
OCR text escaped for `&`, `<` and `>`.

**Only Chrome is measured.** Chrome reaches the text inside a skipped subtree.
Firefox and Safari support `content-visibility`, but whether their find reaches a
skipped subtree is untested here. → Test both before closing the change. A
browser that does not reach it leaves the reader where it is today, so the
failure is a quiet loss of reach, not a broken page.

**Memory is unmeasured.** A book of 1000 pages holds tens of thousands of SVG
elements for as long as the reader stays open. → Read the heap and the element
count in the spike, and on the real reader, on the longest book in the library.

**`add-gutter-trim` is in flight and touches the same answer.** It drops from the
per-page answer every line inside the trim. The book answer must drop the same
lines, or a reader would find text that the page does not show. → Whichever
change lands second carries the rule of the first. The book answer holds no rule
of its own: it is the per-page answer, once per page.

## Open Questions

- Which browsers must the find reach? The answer only widens or narrows the test
  matrix. It changes no requirement and no task.
