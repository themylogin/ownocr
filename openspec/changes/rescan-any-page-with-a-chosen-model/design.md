## Context

See proposal.md — Why. The requirements are in `specs/page-rescan/spec.md`,
`specs/book-ocr/spec.md` and `specs/book-library/spec.md`.

The rescan today is one path. `httpapi.startRescan` refuses a page that holds a
`vl` result, queues `ocr.RescanArgs{BookID, PageIndexes}`, and
`ocr.RescanWorker` runs an endpoint job with `Modes: []string{ModeVL}`. For each
page it reads the stored `v6` result, merges, and inserts `vl` and `merged`.

`InsertPageOCR` upserts on `(page_id, model)`, so a second result of one engine
replaces the first already. The store holds no method that removes a result.

`ocr.profileModes` maps a profile to the engines the OCR action runs. The rescan
does not read it.

The reader picks the result it draws with `ocr.RenderModels = [merged, v6]`, in
`PageOCRIn` / `BookOCRIn`. A page that holds no `merged` result therefore reads
as `v6` with no further rule.

## Goals / Non-Goals

**Goals:**

- One rescan path that runs the engines of the profile it carries.
- A normal rescan leaves a page holding exactly the results the line engine
  wrote, with nothing derived from a reading it replaced.
- No new failure mode for the book. A rescan still never sets the book to
  `failed`.

**Non-Goals:**

- No change to the merge itself, to the OCR action, or to the endpoint image.
- No record of which profile last read a page. The results of a page say what
  read it, and nothing in the interface asks for more.
- No rescan from the page grid. The reader alone offers it, as it does today.

## Decisions

### The extra rescan keeps reading the stored line result

An `vl` rescan runs the visual engine alone and merges with the stored `v6`
result, as today. PP-OCRv6 reads a page the same way every time, so a second
reading returns what the store already holds. Running it again costs endpoint
minutes for a result the system has.

Alternative: run both engines, as the `vl` profile of the OCR action does. It
makes one mapping serve both paths, and it removes the case where a page holds no
`v6` result to merge with. That case cannot happen: both profiles write a `v6`
result, so every page of a complete copy holds one. The cost buys nothing.

The two paths therefore need two maps. `profileModes` stays as it is, and a
second map names the engines a rescan runs:

```
rescanModes = {v6: [v6], vl: [vl]}
```

`ocr.RescanModesOf(profile)` reads it, beside `ocr.ModesOf`. A reader who sees
both names sees that a profile means one thing to an action and another to a
rescan, which is what the `book-ocr` delta states.

### The worker branches once, at the page

`RescanArgs` gains `Profile string`. `Work` reads `RescanModesOf` and fails the
job when the profile is unknown, as `ocr.Worker` does for a copy.

The upload, the endpoint job, the polling, the cleanup and the completion are
common. Only `storePage` differs:

- `vl`: download the markdown and the JSON, read the stored `v6` result, merge,
  write `vl` and `merged`.
- `v6`: download the text and the JSON, write `v6`, remove `vl` and `merged`.

The output keys already name their engine (`OutputPage.Mode()`), so
`storeResults` keys `byInput` by the mode it expects instead of by `ModeVL`.
`OutputPage.TextKey()` and `JSONKey()` already answer the right pair per engine.

Alternative: a second worker for the normal rescan. It would copy the upload,
the polling, the cleanup and the completion, which is most of the file, to branch
in one function.

### One statement stores the new reading and removes the old

A page must never hold a `merged` result built from a `v6` result it no longer
holds. The store gains:

```go
func (s *Store) ReplacePageOCR(ctx context.Context, result PageOCR, remove []string) error
```

It runs the upsert and a `DELETE FROM page_ocr WHERE page_id = $1 AND model = ANY($2)`
in one transaction. The normal rescan calls it with `remove = [vl, merged]`. The
extra rescan keeps `InsertPageOCR` for its two writes: it removes nothing, and
a page that holds a stale `merged` result for a moment between the two inserts
still draws a valid reading.

Alternative: delete first, then insert. A crash between the two leaves a page
with no reading at all, which is worse than the state it started in.

### The API takes the profile in the request body

`POST /api/books/{id}/rescan` carries `{"pageIndexes": [...], "profile": "vl"}`.
`startRescan` checks the profile with `ocr.KnownProfile` before it locks
anything, and passes it to `RescanArgs`. `checkPages` drops the
`PageIndexesWithModel` lookup and the refusal that used it.

The check order stays: the system checks the whole request before it changes
anything.

### The client sends the profile from the dialog

`RescanDialog.vue` gains the radio group of `OcrDialog.vue`, with `Extra`
preselected, and emits `start(profile)`. Its title, its text and its "Rescan"
button stay. `ReaderView.startRescan(profile)` passes it to
`api.rescanPages(id, indexes, profile)`.

`PageMenu.offersRescan` drops its `hasVisualText` test and answers `props.rescan`
alone, so every page of a book that offers the rescan holds the item. The item
and the top bar control read "Rescan" and "Rescan N pages".

The two dialogs do not share a component. They differ in title, body text,
default choice and button, which is most of what either holds. A shared
component would take four props to say so.

### `hasVisualText` goes

`offersRescan` was its one reader. `getBook` drops the `PageIndexesWithModel`
call, `pageView` drops the field, and `detailOf` drops its `visualText`
parameter. `PageIndexesWithModel` itself stays: it is a general store method, and
`store_test.go` covers it.

## Risks / Trade-offs

- [A user rescans with Normal by mistake and loses a good visual reading] → The
  loss is one page and one reading, and the page is still there to rescan with
  Extra. The dialog preselects Extra, so the mistake needs an act.
- [The stored `v6` result an extra rescan merges with can be the result of an
  earlier normal rescan] → That is correct: the merge takes the boxes of the
  reading the page holds. Nothing assumes a `v6` result came from the job that
  built the copy.
- [A normal rescan on a `vl` copy leaves the copy recording the profile `vl`
  while one of its pages holds no visual result] → The `book-ocr` spec already
  states that the profile describes the reading the action performed and not a
  page a rescan read again. The delta states it for both profiles.
- [An old client sends no profile and the server refuses it] → The frontend ships
  with the backend. No other client exists.

## Migration Plan

No data migration. No stored row changes shape, and no column is added. A page
that holds `v6`, `vl` and `merged` today keeps all three until a normal rescan
reads it.

A rollback of the code leaves any page a normal rescan read holding a `v6`
result alone, which is a state the old code already handles: it is the state of
every page of a `v6` copy.
