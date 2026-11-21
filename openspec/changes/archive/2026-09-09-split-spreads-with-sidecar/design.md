## Context

See `proposal.md` for motivation.

Three things already exist and shape the approach.

The sidecar runs, answers `/health`, and carries numpy and OpenCV. It mounts
`book_data` at `/data`, the same volume the backend writes, and production runs
it as uid 10001 so both services can write the same page directories.

The split worker already handles one source page becoming several. It numbers
the pages of the copy from one with `written++`, reports progress on a count it
learns as it goes, and names the source page on every page it writes. Only
`pagesPerSource = 2` is a placeholder.

A detector exists in `output/split_gutter.py`, measured against all 68 pages of
book 2. Its numbers are the evidence behind the decisions below.

## Goals / Non-Goals

**Goals:**
- One cut behaviour, no options, matching what produced `output/full/pages`.
- Page bytes never cross the HTTP boundary.
- A split book reports progress while it runs.

**Non-Goals:**
- Deskewing a cut page. The two pages of a spread often lean by different
  amounts; correcting that is separate work.
- Trimming the black border the dewarp leaves outside a page.
- Removing the strip of the facing page that an overlapping cut carries.
- Detecting a spread that a reader photographed sideways.

## Decisions

### The sidecar returns numbers, not images

The worker sends paths and receives two columns per spread. The alternative,
posting image bytes and receiving two cut images, doubles the bytes on the wire
and makes the sidecar own the file store layout.

The Go side keeps the crop. It already owns `WriteRenditions`, the page keys and
the page rows, and a crop is four numbers and a `SubImage` call.

This is why the shared volume matters: the sidecar must be able to open a path
the backend names. Both services already mount `book_data`.

### Coverage decides whether an image is a spread, not gutter strength

Measured over book 2:

| | Coverage | Prominence |
| --- | --- | --- |
| 67 spreads | 1.00, every one | 3.9 to 90.5 |
| the cover | 0.20 | 15.2 |

Prominence is the height of the gutter peak over the median of the search band.
It cannot separate the two: the cover scores 15.2, above 19 of the 67 real
spreads. The cover carries a short strong blob of energy around its title text.

Coverage is the share of the 20 horizontal bands whose window carries any gutter
energy. A gutter runs the whole height of a spread; a title block does not. The
gate is coverage at or above 0.5. A prominence floor of 3 stays as a second
guard, but it decides nothing on its own.

This is the one decision that a naive reading gets backwards, which is why the
spec states it as a requirement rather than leaving it to the implementation.

### The gutter is a signed-blind ridge, found over angle, fitted as a curve

The gutter is dark when the book is held open and bright when it lies flat. Over
book 2 the change is gradual: prominence falls from 90 near the front to 3.9 near
the end as the reader opens the book flatter. A detector keyed on darkness alone
failed every page after 50.

The energy map is therefore the absolute deviation from a wide horizontal blur,
opened with a tall thin structuring element. The opening deletes text, because
letters are short and horizontal, and keeps the gutter, which is tall.

Finding the line then runs in three steps: a search over angle from -6 to +6
degrees for the strongest column sum; a weighted centroid per band in a window
around that line; and a quadratic fit with outlier rejection. The quadratic is
not decoration. The gutter genuinely bows by up to 2 percent of the width, and
the quadratic halved the median residual against a straight line.

### The two returned columns are the extremes of the curve

A crop takes a column; the gutter is a curve. Cutting at the leftmost point of
the curve would clip the left page on every lower row.

```
left column  = min(width, ceil(max(curve)) + margin)
right column = max(0, floor(min(curve)) - margin)
```

The two rectangles therefore overlap, and each page keeps all of its own
columns. Measured overlap over book 2: 1.3 percent of the width at best, 2.7
percent median, 11.4 percent on the most skewed page.

The margin is a constant of 0.5 percent of the width, not a parameter.

Each half carries a slanted strip of the facing page along its inner edge. That
is deliberate and stays. Painting it out needs a photometric model of the paper,
which is a second thing to get wrong, and the alternative that removes the strip
outright shifts every row sideways. A crop leaves the pixels alone.

### Batches of 20

One call for a whole book is one multi-minute HTTP request with no progress and
a timeout that has to cover the worst book. One call per page costs a round trip
and a fresh Python entry per page.

Twenty pages per call gives the worker something to report on every batch, keeps
any single request to seconds, and keeps the sidecar's peak memory to one
decoded page at a time.

The worker keeps its existing `progressInterval` behaviour and reports after
each batch.

### A rendition writer that takes a decoded image

`ingest.WriteRenditions` copies the source file byte for byte as the original
rendition. A cut page has no source file: it exists only as a decoded, cropped
image.

The change adds a sibling that takes an `image.Image` and encodes the original
rendition as JPEG. An uncut page keeps the existing path-based writer, so its
bytes stay identical to the source and a PNG stays a PNG.

### `SIDECAR_URL` has a default

Every other required variable names something outside the stack: a database, a
bucket, a paid endpoint. The sidecar is a service in the same compose file at a
known name, so `SIDECAR_URL` defaults to `http://sidecar:8000` rather than
refusing to start. The compose files still set it, so the default is a fallback,
not the contract.

### The end-to-end suite needs a synthetic spread

`fixturegen` writes deterministic JPEGs with the standard library alone. None of
them has a gutter, so the current fixtures would all pass through as single
pages and the suite would assert nothing about cutting.

The change adds a spread fixture: two blocks of dark rows separated by a tall
vertical band of a different tone. That is exactly the structure the detector
keys on, and the standard library can draw it. Keeping the fixture synthetic
keeps the suite free of a photograph checked into the repository.

## Risks / Trade-offs

**The detector is tuned on one book** → Every constant comes from book 2, one
camera and one lighting setup. A book with a very dark paper stock, a glossy
page or a very tight binding may score differently. Mitigation: the coverage gate
fails safe. A page it cannot read as a spread passes through whole, which loses
nothing and is visible to the reader, rather than cutting in the wrong place.

**Split now depends on a second service** → A split job fails when the sidecar is
down, where before it could not. Mitigation: compose gains a health condition so
the backend waits for the sidecar, and River retries the job three times before
the copy is marked failed.

**A whole batch fails on one unreadable page** → Twenty pages are refused
because of one. Mitigation: this matches dewarp, where the copy fails if any page
fails. A page that cannot be decoded is a real fault, and the retry rebuilds the
copy from scratch anyway.

**Full-resolution decode in Go for every cut page** → A 4032 by 3024 page as
RGBA is about 49 MB, and the worker holds one at a time plus its two crops.
Mitigation: the split queue already runs one job at a time, and the existing
ingest path already decodes pages at this size.

**The strip of the facing page reaches OCR** → Up to 11 percent of the width of a
half can be the neighbour's line ends. Mitigation: none in this change. Whether
it hurts OCR is unmeasured, and measuring it needs an OCR stage that does not
exist yet. Recorded here so the later stage knows to check.

## Migration Plan

The Split action already exists and already produces copies. This change alters
what those copies contain, so there is no schema migration and no data to
convert.

A book split before this change keeps its duplicated pages. The interface offers
a retry only on a failed copy, so an existing copy is not rebuilt on its own. Re
running Split on the source book is the way to get real single pages, and the
existing rebuild path already clears the copy's pages and files first.

Rollback is the previous image. Nothing this change writes is unreadable to the
older code: the copies hold ordinary page rows and ordinary page files.

## Open Questions

- The overlap margin of 0.5 percent of the width is carried over from the
  prototype, where it was never varied. It may be larger than it needs to be. It
  can be tightened later without touching the specs or the call contract.
