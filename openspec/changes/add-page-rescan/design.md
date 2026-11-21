## Context

See proposal.md for motivation.

Three properties of the system as built shape this design.

**An action makes a copy.** Every arrow in the pipeline creates a second book
that holds `source_book_id`. `ActionsFor("ocr")` returns nothing, because no
stage follows `ocr`, and `startAction` refuses a book that already has a copy
which has not failed. A rescan runs on the book itself and creates nothing, so
it cannot be an action. It fits the vocabulary the system already has for
Delete, Show original and Edit: a page operation.

**The results are already per page.** `page_ocr` is keyed `(page_id, model)` and
`InsertPageOCR` upserts on that pair. Writing a result for three pages of a
two hundred page book needs no schema change and no new table.

**The merge takes two raw results.** `MergeOf(boxData, textData)` unmarshals two
JSON blobs. Both live in `page_ocr.data`. Nothing in the merge requires the two
readings to arrive from one endpoint job.

The read path already prefers the merged result: `ocr.RenderModels` is
`{merged, v6}` and `PageOCRIn` orders by position in that list. A page that
gains a merged result therefore renders through the existing handler with no
change.

## Goals / Non-Goals

**Goals:**

- Read chosen pages again without rereading the book.
- Leave the book usable whatever the rescan does.
- Close every path by which a rescan can destroy a stored result.
- Add no database migration.

**Non-Goals:**

- Marks that survive a reload. They are a few integers in one view.
- A per-page error channel. The mark that stays is the report.
- Reusing the OCR worker. The two jobs share a queue and a bucket layout, not a
  control flow.

## Decisions

### The book never reaches state `failed`

This is the decision the rest rests on.

`canRetry` in `bookState.ts` is `state === 'failed' && sourceBookId !== null &&
action !== null`. An `ocr` copy satisfies the last two always. If a rescan set
`failed`, the stage card would offer Retry, and Retry reaches:

```
runAction('ocr', [sourceBookId])
  -> startAction -> DerivedTx finds the failed copy -> retry()
  -> requeue ocr.Args{BookID: copyID}
  -> worker.rebuild() -> store.DeletePages(copyID)
                          |
                          +-- CASCADE drops every page_ocr row of the book
  -> the whole book is read again with ocr_profile = "v6"
```

Every earlier rescan of that book is gone, with no warning. The book was whole
before the retry and degraded after it.

A rescan therefore returns the book to `complete` in every path, including the
one where it read no page at all. That closes the door completely, because of an
invariant the system already holds: **a complete copy is never rebuilt.**
`startAction` refuses a source book whose copy exists and has not failed, so the
only route to `rebuild()` is through `failed`. A rescan that never writes
`failed` makes `DeletePages` unreachable for a rescanned book.

*Alternatives.* Set `failed` and teach the retry to refuse — needs a column
telling "this copy never finished" from "this copy finished, then a rescan
failed", and leaves a dangerous control on screen meanwhile. Set `failed` and
leave the retry alone — loses data.

### The rescan runs the visual engine alone

The endpoint job names `["vl"]`, not the two engines the Extra profile names.
The PP-OCRv6 result of every page is already stored, so the merge reads it from
`page_ocr.data`.

This halves the endpoint work, and it leaves the PP-OCRv6 result of the page
byte for byte as it was. Rereading a page with v6 would overwrite a correct
result with another run of the same model, for nothing.

*Alternative.* Run the profile as the action does. Rejected on cost, and because
the rewrite gains nothing.

*Consequence.* A page whose stored PP-OCRv6 result is absent or unreadable
cannot be merged. The worker skips that page, logs it, and leaves its mark
standing. It does not fail the job.

### Results are stored page by page

`checkOutput` refuses a partial result for a whole book reading, and its reason
is written into the spec: a copy holding fewer pages than its source is worse
than no copy. That reason does not transfer. A rescan only adds rows, so two
pages stored beat zero stored. The rescan worker does not reuse `checkOutput`.

### Marks are client state, cleared by observation

The marks live in the reader, like the book selection in `useSelection`. Nothing
is stored and nothing is synchronised.

The clearing rule does the reporting work. When the rescan completes, the reader
re-reads the pages it named. A page answering `merged` drops its mark. A page
still answering `v6` keeps it. The user sees exactly which pages took, the top
bar count falls to what is left, and one more activation retries the rest.

This is why no per-page error channel is needed. The `page_ocr` rows already
hold the whole truth, and the mark reads it.

*Trade-off.* A mark is lost on reload, so a user who reloads during a failed
rescan loses the record of which pages did not take. Accepted: the pages still
offer the rescan, so nothing is unrecoverable.

### The rescan runs on the OCR queue

One worker, as today. A rescan and a whole book reading then never reach the
paid endpoint at once, and never share the `ocr/<bookKey>/` prefix. The book
state already prevents the two for one book; the queue prevents them across
books.

### Two new store methods

`SetProcessing` sets `page_count = 0`, which is right for a rebuild and wrong
here. A rescan changes no page count. It needs a transition that sets state and
phase and leaves the count alone, and a matching one that returns the book to
`complete`.

### The server reports the offer, the client reads it

`bookDetail` gains a book level flag, beside `pageRollback` and `pageCut`, and
`pageView` gains a per page flag saying whether that page holds a PaddleOCR-VL
result. The menu, the border and the top bar count all read them.

*Alternative.* Derive it in the client from `GET /pages/{index}/ocr`. Rejected:
that call is lazy and covers the pages near the window only, and the top bar
needs a count over the whole book.

## Risks / Trade-offs

- **A large rescan holds the OCR queue** → A user may mark two hundred pages,
  which blocks whole book readings behind it. This is the trade the OCR action
  already makes with its single worker, and the user chooses the page count at a
  dialog that names it.
- **Two clients rescan one book at once** → The second request refuses. The book
  is `processing` from the first transaction, and the in-flight guard reads it.
  The book state is the lock.
- **A client requests a page that gained VL a moment ago** → The request refuses
  the whole batch and names the page. The client then reloads the book and the
  marks it can no longer offer disappear.
- **The reader fails to re-read a page after the rescan** → The mark stays. A
  stale orange border costs one needless rescan; a wrongly cleared one hides a
  page that was never fixed. The conservative direction is correct here.
- **A reversed requirement** → `book-library` currently requires the right half
  to open no menu, and the e2e suite asserts it. The test changes with the code,
  not after it.
- **The source book is rejected while a rescan runs** → Reject destroys the
  chain. The worker must treat a missing book as nothing to do, as the OCR
  worker already does for a copy rejected while its job waited.

## Migration Plan

No database migration. `page_ocr` already holds what a rescan writes.

Deploy the backend first, then the frontend. A frontend that predates the change
shows no rescan item, which is the current behaviour.

**Rollback is safe.** A page a rescan wrote holds `v6`, `vl` and `merged` — the
same three rows an Extra copy holds for every page. `ocr.RenderModels` already
prefers `merged`, so code that predates this change renders a rescanned page
correctly and reads its text correctly. Rolling back removes the ability to
start a rescan and invalidates nothing a rescan wrote.

## Open Questions

- Whether an abandoned rescan should leave a record an operator can query later,
  beyond the log line. The specs require the reason to reach an operator and do
  not say where. Adding a record later changes no requirement here.
- Whether a rescan naming very many pages should be split into several endpoint
  jobs. The page count of the book bounds it today, which is the same bound a
  whole book reading carries, so this can wait for a book that shows the need.
