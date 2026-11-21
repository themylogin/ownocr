## Context

See proposal.md — Why. The requirements are in the five delta specs under
`specs/`.

What the code holds today:

- `page_ocr(id, page_id, model, txt, data)`, unique on `(page_id, model)`.
  Models stored: `v6`, `vl`, `merged`.
- `ocr.RenderModels = [merged, v6]` is the fixed preference. Two call sites read
  it: `httpapi/pages.go:480` for one page and `httpapi/pages.go:580` for a whole
  book, through `books.PageOCRIn` and `books.BookOCRIn`.
- `ocrmerge.Merge` takes `Lines` (v6 boxes, with corners) and `Blocks` (text
  engine regions, each with a box). It decides everything from geometry and rune
  alignment; it holds no alphabet.
- `ocr.MergeOf(boxData, textData)` parses the v6 JSON and the VL
  `parsing_res_list`, then calls `ocrmerge.Merge`.
- The rescan worker uploads to the bucket, starts a RunPod job, polls, downloads.
  `config.OCR` is a RunPod `Endpoint`.

## Goals / Non-Goals

**Goals:**

- One rule for "this reading disagrees", computed where the whole book is already
  loaded, with no new column and no background job.
- An Ultra rescan that reuses the existing rescan control flow — queue, phases,
  never failing the book — and swaps only what happens per page.
- A stored choice that survives rescans and never silently overrides a reader.

**Non-Goals:**

- No change to `ocrmerge` itself. The merge is measured against two books; this
  change feeds it a different block and leaves its constants alone.
- No Ultra on the whole-book OCR action.
- No re-reading of pages already stored. The choice applies to what a page holds.

## Decisions

### The Ultra text becomes one page-wide block

`ocrmerge` needs blocks with boxes and Claude answers text with none. The block
is therefore the page: `Box = {0, 0, width, height}` from `book_page`, `Text` =
the transcription. The merge then reads the printed rows from the v6 boxes, which
is what it does for every block.

Alternative considered and rejected for now: a text-to-text alignment matcher
that maps the transcription onto the v6 line texts without geometry. It is the
more literal reading of "match against the v6 bboxes" and needs nothing from the
model, but it is a new matcher beside a merge already tuned on two books.

**The risk this carries, and how the plan answers it.** `ocrmerge` has a path for
a block whose boxes cannot say where its printed rows are: it discards that
block's boxes and places rows of its own. A block covering a whole page — two
columns, a running head, footnotes — may well trip it, and then the v6 boxes are
thrown away for the whole page. That is not a small failure; it is the merge
losing exactly what it was there for.

**Measured: the page-wide block holds.** The 11 pages with a stored transcription
merge with 0 discarded boxes and 0 lines the merge placed, each keeping its exact
v6 line count. Swept wider — every one of the 293 read pages of books 17, 18 and
24, each blocked whole against its own v6 text, which asks the geometry question
without asking the alignment one — again 0 trips of the disaster path. That sweep
holds 10 two-column pages and 9 pages with small type at the foot, and all 19 are
clean. The fallbacks below therefore stay unbuilt.

So the first task is a measurement, not an implementation: 11 pages already have
a stored Claude transcription and a stored v6 result, so the page-block merge can
be run offline against them and the discarded-box count read off before any
worker exists. If a page-wide block trips the disaster path, the fallback is one
block per paragraph of the transcription, boxed by the v6 lines that the
paragraph's text aligns to — and if that also fails, the alignment matcher above.
The task list keeps that decision point explicit.

### The model names

`claude` for the raw transcription, `claude_merged` for the renderable merge.
Naming the merged rows after their text engine keeps `merged` meaning what it
means in every stored row today, so no migration rewrites existing data.

| stored model | holds | renderable | profile shown |
|---|---|---|---|
| `v6` | lines + boxes | yes | Normal (PP-OCRv6) |
| `vl` | reflowed blocks | no | — |
| `merged` | v6 boxes + vl text | yes | Extra (PaddleOCR-VL) |
| `claude` | plain text | no | — |
| `claude_merged` | v6 boxes + claude text | yes | Ultra (Claude Opus 5) |

`RenderModels` becomes `[claude_merged, merged, v6]`, best first. It doubles as
the rank for auto-assignment, so one list is the single source of "better".

### `ocr_id` and who set it

```sql
ALTER TABLE book_page ADD COLUMN ocr_id BIGINT
    REFERENCES page_ocr (id) ON DELETE SET NULL;
```

`ON DELETE SET NULL` is what makes the "the named reading is removed" requirement
fall out of the schema rather than out of application code — a normal rescan that
deletes `vl` and `merged` leaves the page naming nothing, and it answers by
preference again.

One column and not two. A page names a reading because a reader chose it and for no
other reason, so `ocr_id IS NOT NULL` already says "a reader chose this" and a flag
beside it would hold the same fact twice. An earlier draft had the system name a
reading of its own accord and a flag to tell the two apart; migration 0011 drops
that flag.

Selection becomes: `ocr_id` when set, else first match in `RenderModels`. Both
`PageReading` and `BookReadings` take the same shape so the one-page and whole-book
answers cannot drift — the existing spec requires they never disagree.

A reading better than the one a page names clears that name, where a draft of this
change had it stand aside. A reader picks a reading because it is the best the page
holds, so a better one arriving answers the same want; leaving the older choice
standing would hide exactly the reading the reader would have picked. It happens
where a result is stored, in the same transaction as the write, and only for a page
that names something: a page naming nothing answers by rank already.

### The disagreement rule runs where the book is already loaded

Every test is page-local: the reading a page answers against the v6 reading of
that same page. Nothing reads the rest of the book.

The rule still runs in the book-lines endpoint, because that endpoint already
loads every page's results, so the rule costs one extra pass over data already in
memory. No column, no job, nothing to invalidate.

The count of disagreeing pages belongs to the book, and the reader asks for the
book lines already, so one answer carries the count and the per-page flags
together. The per-page answer does not report disagreement: a second reporting
site could drift from the first, and no screen needs it.

A new `internal/health` package holds the rule so it is testable without a
database or an HTTP handler.

### The Anthropic client mirrors the RunPod one

`internal/anthropic`, hand-rolled over `net/http`, same shape as
`internal/runpod`: a `Client` with a name, base URL and token. No new module
dependency, and the existing error and retry idiom carries over.

`config.Claude` joins `config.Dewarp` and `config.OCR`, and is required exactly as
both of them are: it reads `CLAUDE_API_KEY` through `e.require`, so the server
refuses to start without it. The worker therefore holds a key it can trust, and
no code path exists for a rescan that half-works.

The worker path splits at one point. `rescan.go` already branches per page by
mode; `claude` adds a third branch that skips upload, endpoint and download
entirely, and calls the API per page. Everything around it — the queue, the
phases, returning the book to `complete`, never failing it — is untouched.

## Risks / Trade-offs

- [A page-wide block trips the merge's disaster path and discards every v6 box]
  → measured offline against 11 real pages before any worker is written; two
  fallbacks named above, decided by that measurement.
- [Claude narrates instead of transcribing] → observed once already, on a blank
  page, where it explained that it should output nothing. The prompt asks for a
  sentinel for an empty page, and the worker treats a response that fails to look
  like a transcription as a page it could not read — the rescan already stores
  what it can and abandons the rest.
- [A book in a script PP-OCRv6 cannot read flags on every page] → accepted. The
  page-wise script test has no reference for such a book, and reading the rest of
  the book to rescue it would mean a page of a mixed book counting as foreign
  because its neighbours differ. The library holds no such book today, and the
  flag costs a reader a look, not a rescan.
- [The API key sits in a worker that reads user-supplied books] → the key is read
  from the environment into the config and never written to the database, the
  bucket, or any result row.
- [Cost runs away] → Ultra is rescan-only and never offered on a whole book. The
  worker reads the pages the request names and no others. The dialog of a book read
  with PaddleOCR-VL opens on Ultra, so a rescan of such a book spends by default:
  that is deliberate, because the model that read it has already failed the page,
  and the request still names the pages one at a time.
- [`ocr_chosen_by_reader` makes a page permanently immune to improvement] → the
  reader can always choose again, and the menu shows every reading including
  newer ones. The flag suppresses automatic takeover, not the reading itself.

## Migration Plan

One forward migration adds two nullable/defaulted columns; no data is rewritten
and no stored result changes shape. Every page reads exactly as it did until
something sets `ocr_id`.

Rolling back the code leaves the two columns in place and unread — a page that
names a reading simply answers by preference again, which is a state the old code
already handles.
