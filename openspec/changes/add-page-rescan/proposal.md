## Why

A book read with the Normal profile holds PP-OCRv6 text on every page. Some
pages defeat that engine: polytonic Greek, heavy ligatures, damaged print. A
reader finds them one at a time, while reading.

The only remedy today is to run the OCR action again on the source book with the
Extra profile. That rereads every page. It costs minutes a page and money for
the pages that were already right, and it leaves a second copy of the book.

A reader who finds four bad pages in five hundred wants those four read again,
in the book already open.

## What Changes

- A new page operation, "Rescan with Extra effort", reads chosen pages of an
  `ocr` book again. It runs on the book itself. It creates no copy.
- The item appears in the page menu of a page that holds no PaddleOCR-VL result.
  A page that already holds one offers it no more.
- Choosing the item marks the page. A mark is client state and shows as an
  orange border. Marks accumulate across the book.
- The reader top bar holds an orange "Rescan N pages with Extra effort" button
  while any page is marked. It asks for confirmation before it starts.
- A new endpoint takes the book and the page indexes. It sets the book to
  `processing` and queues one job on the OCR queue.
- The job runs the PaddleOCR-VL engine alone. The PP-OCRv6 result of each page
  is already stored, so the merge reads it from the database instead of the
  endpoint. This halves the endpoint work and leaves the PP-OCRv6 result
  untouched.
- The job stores what it gets, page by page. A page that fails costs the pages
  that succeeded nothing.
- The book returns to `complete` whatever happens. A rescan never sets the book
  to `failed` and never changes its page count.
- The mark of a page clears when that page gains its new text. A page the
  rescan did not reach keeps its orange border, so the user sees what to try
  again.
- **BREAKING**: the right half of the OCR reader opens a menu of one item. It
  opened no menu before, and the current spec and its test both require that.
- **BREAKING**: `book.ocr_profile` records how the book was first read. It stops
  describing every page of the book. The `page_ocr` rows are the authority on
  what read each page.

### Non-goals

- No rescan from the page grid or the stage list. The operation belongs where a
  user finds a bad page, which is the reader.
- No choice of engine. Extra effort is the only reason to read a page again.
- No per-page failure message. An abandoned rescan leaves the orange border and
  nothing else.
- No rescan of a book read with the Extra profile. Every page already holds VL.

## Capabilities

### New Capabilities

- `page-rescan`: reading chosen pages of a book in the `ocr` stage again with
  the Extra effort engine, in place. Covers the marks, the request, the job, the
  storage of a partial result, and abandoning a failure without touching the
  book.

### Modified Capabilities

- `book-library`: "The right half of the OCR reader offers no menu" reverses.
  The right half opens a menu holding the rescan item alone. "A right click on a
  page offers the page menu" gains a rescan group, after the image group and
  before the group that holds "Edit" and "Show original".
- `book-ocr`: "A page read by two engines holds a merged result" states that a
  copy whose profile names one engine holds no merged result. A rescanned page
  of a `v6` copy holds one. "The OCR action carries a model profile" states that
  the profile says which engine holds the text of record for the copy. It says
  so for the pages the first read produced, not for a rescanned page.
- `book-pages`: "The system refuses a page operation while the chain holds work
  in flight" names three operations. The rescan is a fourth and obeys the same
  rule.
- `book-progress`: the spec holds one requirement per worker. The rescan worker
  needs one. It reports the same six phases and adds none.

## Impact

**Backend**

- `internal/httpapi`: a new route and handler for the rescan request. It
  validates the page indexes, applies the in-flight guard, and queues the job in
  one transaction.
- `internal/books`: two store methods. `SetProcessing` zeroes `page_count`,
  which is right for a rebuild and wrong here, because a rescan changes no page
  count. `InsertPageOCR` already upserts on page and model, so storing a result
  again needs nothing new.
- `internal/ocr`: a new River job and worker beside the existing one. It runs on
  the OCR queue, so a rescan and a whole-book read never meet at the endpoint.
  It reads the page rows of the `ocr` book alone and never loads the source
  book. `MergeOf` already takes two raw results and is reused unchanged.
- `internal/httpapi/view.go`: `pageView` gains a field saying whether the page
  holds a PaddleOCR-VL result. The menu, the border rule and the button count
  all read it.

**Frontend**

- `ReaderView.vue`: wires a context menu on the `OcrPage` half, holds the marks,
  draws the orange borders, and shows the rescan button in the top bar.
- `PageMenu.vue`: a rescan group, and an option list that holds the rescan item
  alone when the right half opens it.
- `usePageOcr.ts`: the cache keeps a page forever and never asks again. It needs
  to drop the rescanned pages, or the reader keeps drawing the stale PP-OCRv6
  lines over a page that now holds a merged result.
- `ReaderView.vue` subscribes to the WebSocket. It fetches the book once today
  and learns nothing after that. `useWebSocket` is a global singleton that
  `App.vue` already connects, so this is one `onEvent` call.
- A confirmation dialog, beside the existing OCR and reject dialogs.

**No database migration.** `page_ocr` is already keyed by page and model, and
already replaces a result of the same pair.

**Tests**

- The e2e scenario asserting that the right half opens no menu reverses.
- `tools/fakerunpod` already writes the objects of every engine a job names, and
  already refuses a job naming none. A one-engine rescan job needs no change to
  it.
