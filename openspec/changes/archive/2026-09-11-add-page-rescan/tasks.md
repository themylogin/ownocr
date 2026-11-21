## 1. Report the offer from the server

- [x] 1.1 Add a store method answering which pages of a book hold a PaddleOCR-VL
  result, keyed by page index, and verify a store test covers a book with none, a
  book with all, and a book with some
- [x] 1.2 Add `hasVisualText` to `pageView` and `pageRescan` to `bookDetail`,
  where `pageRescan` is true for a book in the `ocr` stage in state `complete`,
  and verify a view test covers a `v6` book, a `vl` book, and a running book
- [x] 1.3 Wire the new page flag through `detailOf` and `getBook`, and verify
  `GET /api/books/{id}` reports it per page in a handler test

## 2. Move the book without losing its page count

- [x] 2.1 Add a store method setting state `processing` and a phase while leaving
  `page_count` untouched, and a matching one returning the book to `complete`,
  and verify a store test asserts the page count survives both
- [x] 2.2 Give the first method a transaction variant, as `RequeueTx` has, so the
  state change and the job insert commit together, and verify a test asserts a
  rolled back transaction leaves the book `complete`

## 3. The rescan job

- [x] 3.1 Add rescan job arguments carrying the book id and the page indexes, on
  the OCR queue with three attempts, and verify the job kind differs from the OCR
  job kind in a unit test
- [x] 3.2 Implement the worker up to the endpoint call: load the book, answer nil
  when it is gone, load the named pages, report "Uploading", clear the prefix,
  upload each named page from its own image key, and verify a worker test asserts
  only the named pages reach the bucket
- [x] 3.3 Run the endpoint job naming the PaddleOCR-VL engine alone, reporting the
  same phases the OCR worker reports, and verify a worker test asserts the job
  input names one engine
- [x] 3.4 Store each page that returned: read the stored PP-OCRv6 result, merge,
  write the `vl` and `merged` results, and verify a worker test asserts the page
  holds three results and that the `v6` result is unchanged byte for byte
- [x] 3.5 Skip and log a page the endpoint failed, and a page whose stored
  PP-OCRv6 result is absent or unreadable, without failing the job, and verify a
  worker test using the fake endpoint's `mode=page` asserts the other pages still
  gain their results
- [x] 3.6 Delete the prefix and return the book to `complete` on every path,
  including the one where no page was read, and verify a worker test asserts the
  book is `complete` with no error and no phase after a whole job failure
- [x] 3.7 Return the error on an attempt that is not the last, so River retries
  while the book stays `processing`, and verify a worker test asserts the book is
  never set to `failed` on any attempt
- [x] 3.8 Register the worker in the server wiring and verify the server starts
  with it

## 4. The rescan request

- [x] 4.1 Add `POST /api/books/{id}/rescan` taking `pageIndexes`, and verify the
  route answers 404 for an unknown book in a handler test
- [x] 4.2 Refuse a request that names no page, names an index the book does not
  hold, or names a page that already holds a PaddleOCR-VL result, refusing the
  whole request before anything changes, and verify a handler test covers each
  refusal and asserts no job is queued
- [x] 4.3 Refuse a request for a book outside the `ocr` stage, a book not in state
  `complete`, and a book whose chain holds work in flight, reusing the existing
  in-flight guard, and verify a handler test covers each
- [x] 4.4 Collapse a page index named twice to one page, and verify a handler test
  asserts the queued job names it once
- [x] 4.5 Lock the book, set it `processing` and queue the job in one transaction,
  then publish `book.updated`, and verify a handler test asserts the book state
  and the queued job both appear, or neither does

## 5. The client reads the offer

- [x] 5.1 Add `hasVisualText` to `Page`, `pageRescan` to `BookDetail`, and a
  `rescanPages` call to the API client, and verify the frontend type check passes
- [x] 5.2 Add a "Rescan with Extra effort" item to `PageMenu`, in its own group
  after "Download" and before the group holding "Edit" and "Show original",
  offered only when the book and the page both allow it, and verify the menu of a
  `v6` page holds it and the menu of a rescanned page does not

## 6. The reader marks and starts

- [x] 6.1 Open a menu on the `OcrPage` half holding the rescan item alone, and
  open none at all when that page offers no rescan, and verify by right clicking
  both halves of a `v6` page and of a rescanned page
- [x] 6.2 Hold the marks in the reader, add one per menu choice, and draw a border
  in the warning colour on every marked page, and verify marking three pages
  leaves three borders and leaving the reader drops them
- [x] 6.3 Show a control in the reader top bar naming the number of marked pages,
  in the colour of the marks, absent when none is marked and starting nothing
  while the book is not `complete`, and verify the count follows the marks
- [x] 6.4 Add a confirmation dialog naming the page count, beside the existing OCR
  and reject dialogs, and verify cancelling starts nothing and leaves every mark

## 7. The reader follows the rescan

- [x] 7.1 Subscribe the reader to the WebSocket through the existing singleton,
  and verify the reader shows the book state changing without a reload
- [x] 7.2 Add a way to drop cached page text in `usePageOcr`, and call it for the
  rescanned pages when the book returns to `complete`, and verify a rescanned page
  on screen redraws its merged text with no reload
- [x] 7.3 Clear the mark of a page that now answers `merged` and keep the mark of a
  page that still answers `v6`, and verify a partial rescan leaves exactly the
  pages it missed marked and the top bar counting them

## 8. Tests and documentation

> The Go suite empties the dev database: `internal/testdb` runs
> `TRUNCATE book CASCADE` against the same database the dev stack serves. Save
> anything you need first, or restore afterwards with `./pull-prod-db.sh`.

- [x] 8.1 Reverse the assertion in `e2e/tests/ocr.spec.ts:321`, which requires the
  read half to open no menu, and verify the case now expects a menu of one item
  and a second case expects none on a rescanned page
- [x] 8.2 Add `e2e/tests/rescan.spec.ts` covering the whole path: read a book with
  the `v6` profile, mark two pages, confirm, and verify both pages hold `v6`, `vl`
  and `merged` through the `pageResults` helper while an unmarked page holds `v6`
  alone
- [x] 8.3 Extend that spec with a partial rescan, using the fake endpoint's
  `mode=page`, and verify the book completes, the page that took loses its mark,
  and the page that did not keeps its border
- [x] 8.4 Extend that spec with a whole job failure, using `mode=job`, and verify
  the book is `complete`, carries no error, offers no retry control, and holds
  every result it held before
- [x] 8.5 Add a case asserting that a book rescanned once still records the profile
  `v6` and that its unrescanned pages still offer the rescan
- [x] 8.6 Run the full suites and verify they pass: `go test ./...` in the
  development backend, then `./run-e2e.sh`
- [x] 8.7 Update `README.md`: add the rescan to "Page operations", add the route to
  the API table, and correct the OCR flow section where it states that a copy
  whose profile names one engine holds no merged result
