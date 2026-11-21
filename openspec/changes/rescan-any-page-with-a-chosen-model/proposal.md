## Why

A rescan reads one engine and one direction only. It runs PaddleOCR-VL, and it
refuses a page that already holds a PaddleOCR-VL result. A user who finds that
the visual reading of a page is worse than the line reading has no way back. The
page keeps the visual text for good, and the whole book must be read again to
lose it.

The reading of a page is a choice, and the OCR action already offers that choice
as a dialog. A rescan is the same choice on a smaller subject, so it SHALL offer
the same dialog.

## What Changes

- Every page of a complete book in the `ocr` stage offers the rescan. The offer
  no longer depends on the results the page holds.
- The rescan dialog gains the model choice of the OCR dialog: "Normal"
  (PP-OCRv6) and "Extra" (PaddleOCR-VL). It keeps its own title, its own text and
  its "Rescan" button. It preselects "Extra", where the OCR dialog preselects
  "Normal": a user rescans a page because the normal model read it badly.
- The menu item and the top bar control lose the words "with Extra effort". The
  dialog names the model now, so the control that opens it cannot.
- The rescan request carries a profile. The system refuses a request that names
  no profile or an unknown one, as the OCR action does.
- The profile `vl` runs the visual engine alone and merges with the stored
  PP-OCRv6 result, as a rescan does today.
- The profile `v6` runs PP-OCRv6 alone. It replaces the stored PP-OCRv6 result of
  every page it reads, and removes the PaddleOCR-VL result and the merged result
  of that page.
- **BREAKING**: the API refuses a rescan request that names no profile. Every
  client must send one.
- **BREAKING**: the page view drops the `hasVisualText` field. It reported which
  pages the visual engine had read, and it existed to hide the rescan item. Every
  page offers the rescan now, so nothing reads it.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `page-rescan`: every page of a complete OCR book offers the rescan, the request
  carries a profile, the profile names the engines the rescan runs, and a normal
  rescan replaces the reading of the pages it reads.
- `book-ocr`: a page can lose an engine result. The merged result of a page goes
  with the engine result the page loses.
- `book-library`: the page menu offers the rescan on every page of an OCR reader,
  the item is named "Rescan", and the rescan dialog asks for a model.

## Impact

Backend:

- `backend/internal/ocr/args.go`: `RescanArgs` gains a profile.
- `backend/internal/ocr/endpoint.go`: a map from a profile to the engines a
  rescan runs, beside the map the OCR action uses.
- `backend/internal/ocr/rescan.go`: the worker runs the engines of its profile,
  stores the results of each, and removes the results a normal rescan drops.
- `backend/internal/httpapi/rescan.go`: the request carries a profile, and the
  page check drops the refusal of an already read page.
- `backend/internal/httpapi/handlers.go`, `view.go`: the book detail drops
  `hasVisualText` and the query that fills it.
- `backend/internal/books/store.go`: a method that removes named results of a
  page.

Frontend:

- `frontend/src/components/RescanDialog.vue`: the model choice.
- `frontend/src/components/PageMenu.vue`: every page offers the item, and the
  item is named "Rescan".
- `frontend/src/views/ReaderView.vue`: the dialog answers a profile, and the
  control loses the engine name.
- `frontend/src/api/client.ts`: `rescanPages` carries a profile, and `Page` drops
  `hasVisualText`.

Tests: `backend/internal/httpapi/rescan_test.go`,
`backend/internal/ocr/rescan_test.go`, `e2e/tests/rescan.spec.ts`,
`e2e/tests/helpers.ts`.
