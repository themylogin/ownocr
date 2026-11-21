## Why

The Split action does not split. `internal/split` writes two identical copies of
every source page, which proves the one-to-many path but produces no single
pages. A photo of a book spread holds two pages, and every stage after Split
reads one page at a time.

A working detector already exists in `output/split_gutter.py`. It found the
gutter on all 67 spreads of book 2 and passed the cover through untouched. The
sidecar service exists too, answers `/health`, and already carries numpy and
OpenCV. This change joins the two.

## What Changes

- The sidecar gains a `POST /split` method. It takes a list of image paths and
  returns one entry per image: `null` for a page that is not a spread, or a pair
  of cut columns for a page that is.
- The split worker calls the sidecar in batches of 20 pages, then cuts every
  spread into two pages and copies every other page whole.
- **BREAKING** A source page no longer maps to a fixed number of copy pages. The
  interim `pagesPerSource = 2` goes. A source book of 68 pages with 67 spreads
  becomes a copy of 135 pages, not 136.
- The detector moves from `output/split_gutter.py` into the sidecar with every
  option removed. `--mask`, `--straighten`, `--overlap`, `--min-prominence`,
  `--quality` and `--debug` all go. One behaviour remains: the plain crop that
  produced `output/full/pages`.
- `output/` is deleted. It holds 61 MB of sample images and a prototype whose
  code now lives in the sidecar.
- The backend gains `SIDECAR_URL`, which defaults to `http://sidecar:8000`.
- `fixturegen` gains a synthetic spread, so the end-to-end suite can assert a
  real cut instead of a pass-through.

## Capabilities

### New Capabilities
- `page-split-detection`: The sidecar method that finds the gutter of a spread
  and reports where to cut it, including how it tells a spread from a single
  page and how it fails.

### Modified Capabilities
- `book-split`: The worker cuts spreads instead of copying pages. Two of its
  requirements state the copying behaviour and both change. One scenario is
  already wrong today: it says a 120 page source gives a 120 page copy, while
  the interim worker writes 240.

## Impact

| Area | Change |
| --- | --- |
| `sidecar/app.py` | New `/split` method and the detector behind it. |
| `sidecar/requirements.txt` | No change. numpy and OpenCV are already there. |
| `backend/internal/split/` | The worker calls the sidecar and cuts. A client for the sidecar joins the package. |
| `backend/internal/ingest/image.go` | A rendition writer that takes a decoded image, because a cut page has no source file to copy. |
| `backend/internal/config/` | `SIDECAR_URL`. |
| `backend/tools/fixturegen/` | A synthetic spread fixture. |
| `e2e/tests/split.spec.ts` | Asserts a cut, not a copy. |
| `docker-compose*.yml` | The backend gains `SIDECAR_URL` and waits for the sidecar to be healthy. |
| `output/` | Deleted. |
| `README.md` | The Split flow section describes the real algorithm. |

Splitting stays on its own River queue with one worker, and needs no object
storage and no remote endpoint. The sidecar shares the `book_data` volume, so
page bytes never cross the HTTP boundary: the worker sends paths and receives
numbers.
