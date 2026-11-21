## Context

See proposal.md - Why.

**The stored results already hold what the reader needs.** `page_ocr.data` of the
`v6` row is `{"raw": ..., "lines": [{"text", "score", "bbox"}]}`. The `merged`
row is the `ocrmerge.Result`, whose `lines` carry `bbox`, `text`, `source` and
`score`. One decoder reads both, because both name the field `lines` and the box
`bbox`. The `vl` row holds blocks of reflowed text and no lines, so it is of no
use to a renderer.

**The boxes count in the original pixels.** The OCR worker uploads the `orig`
rendition of every source page. Ingest stores the width and the height of that
same image in `book_page`, and the OCR copy carries them over unchanged. The
`width` and `height` the book detail reports are therefore the space the boxes
lie in.

**The reader shows the 1280 px display rendition.** The image on screen is
smaller than the box space in almost every case, so the client needs one scale
factor per page: `rendered width / page.width`.

**The reader loads every page of the book at once.** `ReaderView` renders one
`figure` per page with a lazy image. This holds for images because the browser
loads them lazily and the intrinsic size reserves the box. It does not hold for
OCR boxes: 500 pages of 40 lines is 20000 nodes.

**The reader lives inside a 72 rem column.** `App.vue` puts every view inside
`main`, which carries `max-width: 72rem`. Two page images side by side need more.

## Goals / Non-Goals

**Goals:**

- One request per page that comes near the window, and none for the rest.
- Boxes and text that follow the image exactly at every scale.
- A layout the browser scales, not the client, so a resize costs one number.
- No change to the OCR worker, the engines, the merge, or the database.

**Non-Goals:**

- Editing the text of a line. This change only shows it.
- A word box or a character box. The line box is what the engines report.
- Showing the `vl` blocks, the dropped text, or the merge counts.
- A page thumbnail strip, a page number, or a jump control.

## Decisions

### The server picks the result, the client does not

`GET /api/books/{id}/pages/{index}/ocr` answers:

```json
{ "model": "merged", "width": 3000, "height": 4000,
  "lines": [{ "box": [120, 240, 2880, 300], "text": "..." }] }
```

The handler reads the results of the page, takes the `merged` row when it exists
and the `v6` row when it does not, and decodes `lines` out of `data`. It answers
404 with a reason when the page holds neither.

The client stays free of engine names and profiles. It reads `model` only to show
which reading it draws, if a later change wants that. The alternative - the
client asking for a model by name - moves the fallback rule of the `v6` profile
into the frontend and repeats it in every caller.

The decoder belongs in `internal/ocr` beside `boxResult`, which already reads the
same shape for the merge. `httpapi` imports `internal/ocr` today, so no new
dependency appears.

### One request per page, not one per book and not a range

The route follows the page routes that exist: `/api/books/{id}/pages/{index}/...`.
The reader holds at most a handful of pages near the window, so it opens a
handful of requests. Each answer caches on its own, in the browser and in the
client map.

A range endpoint (`?from=&to=`) would cut the number of round trips. It also adds
a second way to name pages, and a range that spans a deleted index needs rules of
its own. The gain is small at 6 pages. If the reader later prefetches 50 pages
ahead, a range endpoint can be added without changing this one.

A whole-book endpoint is out: a book of 500 pages carries several megabytes of
lines, which is what the proposal sets out to avoid.

### The overlay is one SVG per page, in image coordinates

The right half holds the image, a white veil, and an
`<svg viewBox="0 0 width height">` whose `text` elements sit at the box
coordinates of the lines. The viewBox does the scaling, so the client writes the
raw box numbers and never multiplies a coordinate.

Each line is one `<text>` with `textLength` set to the box width and
`lengthAdjust="spacingAndGlyphs"`. The browser fits the glyphs to the box with no
measurement by the client. The font size follows the box height, and the baseline
sits near the bottom of the box.

The alternatives:

- Absolutely positioned `div`s with a per-box `left/top/width/height` in scaled
  pixels. Every resize rewrites four numbers per line, and fitting the text needs
  a measurement per line: either a forced layout read or an offscreen canvas
  `measureText`. More code, more work per frame.
- `foreignObject` inside the SVG. It brings back the measurement problem and
  drops the one thing the SVG gives for free.

`lengthAdjust="spacingAndGlyphs"` stretches the glyphs when the engine text is
short and its box is wide. That is the honest rendering: the box is the extent
the engine reported for that text.

### The two halves are one row, so scrolling needs no code

Each page is one row holding two boxes of the same size. The document scrolls,
both halves move together, and no scroll listener exists to drift. The right
half is a second `<img>` with the same `src`, which costs one fetch and one
decode in the browser cache.

### The scale is a per-page number, computed from the window width

`scale = min(1, floor((window width - chrome) / 2) / page.width)`, with `chrome`
the gap between the halves plus the page padding, as `PageCompare.vue` already
does for its two panes. A resize listener updates the window width, and the two
boxes take `width: page.width * scale`.

The cap at 1 keeps the rule the reader follows: never enlarge a page beyond its
stored size. Both halves take the same scale by construction, because both read
the same page row.

### The reader draws the boxes of the pages near the window

One `IntersectionObserver` watches the row of every page, with a root margin of
one window height. A row that enters asks for its lines once and keeps them in a
`Map` keyed by page index. A row that leaves drops its SVG from the DOM and keeps
the map entry.

The lines of a page are small: 40 lines of 60 characters is about 3 KB, so a book
of 500 pages holds under 2 MB after a full scroll. The DOM is what costs, and the
observer bounds it to the pages near the window.

The rows keep their size whether the lines have arrived or not, because the size
comes from the page dimensions. Nothing moves under the reader while it scrolls.

### The OCR reader breaks out of the content column through a layout flag

`App.vue` keeps the `main` column and gains a `wide` modifier that drops the
`max-width`. `ReaderView` sets the flag while it shows an OCR book and clears it
when it leaves. The flag lives in a small composable beside `useStages`, which
already shares `bookStage` between the two.

The alternative - a CSS breakout in the view (`width: 100vw; margin-left:
calc(50% - 50vw)`) - needs no shared state and counts the scrollbar into `100vw`,
which gives a horizontal scrollbar on every desktop browser.

### The e2e fake writes line boxes worth drawing

`backend/tools/fakerunpod` writes one v6 line with the box `[0, 0, 10, 10]` for
every page. A test cannot tell a placed box from an unplaced one with that. The
fake gains a few lines per page, with boxes derived from the page size, so a test
can assert that a box lands where the line says.

This changes no behavior of the system under test: the fake stands in for the
endpoint alone.

## Risks / Trade-offs

- [A page with hundreds of lines still costs hundreds of SVG nodes] → The
  observer holds a few pages at a time, and an SVG `text` node is cheaper than a
  positioned `div` with its own font metrics.
- [`textLength` distorts a short text in a wide box] → The box is the extent the
  engine reported, so the distortion shows a bad box, which is worth seeing.
- [Text selection across the two halves selects the image alt text too] → The
  left image carries an empty alt in the OCR reader, so a selection of the right
  half stays text.
- [A running OCR copy holds pages with no result yet] → The endpoint answers 404
  with a reason, and the reader shows the veiled image with no text. The user
  sees no error.
- [The white veil hides part of the page it covers] → The veil sits at 50
  percent: at 20 percent the print showed through the drawn text and neither
  reading was legible. The opacity is one CSS
  number, so it can move again without touching the layout.
- [The two halves must hold the size the boxes were placed in] → Both halves are
  `flex: 0 0 auto`, and the scale reads `documentElement.clientWidth`, which
  leaves the scrollbar out. A half the flex row shrinks no longer holds the shape
  of its boxes, and the image inside the overlay uses `object-fit: fill`, so a
  letterbox cannot open between the image and the text.
