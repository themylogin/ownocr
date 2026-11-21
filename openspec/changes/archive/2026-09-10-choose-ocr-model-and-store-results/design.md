## Context

See proposal.md - Why.

Three facts in the current code shape this design.

**The endpoint contract drifted.** `runpod/paddleocr/handler.py` demands a
`modes` array, answers `output_keys` as a list, and returns one entry per engine
per page. `internal/ocr/endpoint.go` sends no `modes`, reads `output_key` as a
string, and `checkOutput` fails when the entry count differs from the page
count. Every job against the deployed endpoint fails today.

**The action pipeline carries no arguments.** `actionRequest` holds `bookIds`
alone. `startAction`, `queueAction` and `ocr.Args` all take the book id only.
`httpapi/actions.go` `retry` rebuilds a failed job from `existing.Action`, which
names the action and nothing else.

**An OCR copy duplicates every page image.** `ocr/worker.go` `storePage` calls
`ingest.WriteRenditions`, which writes an original, a 1280 px and a 480 px
rendition for every page. OCR changes no pixel, so the copy is a byte for byte
second copy of the book.

## Goals / Non-Goals

**Goals:**

- One `page_ocr` row per engine per page, holding what that engine produced.
- A profile that survives a retry, so a retried copy runs the same engines.
- An OCR copy that owns no files.
- An endpoint contract that matches the deployed handler.

**Non-Goals:**

- Reading the stored text back. No route, no view, no renderer.
- Migrating the markdown files of existing copies into rows. See Migration Plan.
- Any change to `runpod/paddleocr`. The handler is already correct.

## Decisions

### A page of an OCR copy points at the image of its source page

`book_page` gains `image_book_key TEXT`. It names the book whose directory holds
the files of this page. `NULL` means the page owns its files, which is every
page written before this change and every page of every other action. An OCR
copy sets it to the storage key of its source book, and reuses the storage key
and the extension of its source page.

Three call sites build a page URL from a book key and must read
`image_book_key` first: `books.Event` for the cover, `httpapi/view.go`
`summaryOf` for the cover, and `httpapi/view.go` `pageViewOf` for the three
renditions. Two others already take the owning key from `PageOfBook`
(`pages.go:458` and `pages.go:614`) and need no change.

`ocr/worker.go` then drops `ingest.WriteRenditions`, and `rebuild` drops the
`RemoveAll` and `MkdirAll` of the page directory. An OCR copy creates no
directory at all.

**This reverses a decision in `2026-09-10-add-book-ocr/design.md`**, which
rejected "page rows that point at the files of the source book" because "a
reject of the source book would then destroy the images of the copy". That
reason does not hold. `store.go` `Chain` walks from a book down to every copy
made from it, and `rejectBook` destroys the whole chain in one request. A reject
of the source book destroys the OCR copy in the same call, so no row ever
outlives the files it names.

The second condition is that a referenced image cannot change under the copy.
`view.go` `offersPageCut` and `offersPageRollback` both require
`book.DerivedState == nil`. Once the OCR copy exists, the source book accepts no
re-cut and no rollback. The images are frozen for as long as anything points at
them.

The archived design also rejected "a copy with no page rows", because the Done
tab would show a book with no cover and an empty grid. That objection does not
apply here. The page rows stay. Only the bytes are shared.

Alternative, rejected: resolve the owning key through `source_page_id` with a
join. Every page of an OCR copy comes from one source book, so a join returns
the same key for every row. A column on the page states the fact directly, and
`Page` already reaches all three URL sites without a new query.

Alternative, rejected: keep writing renditions. It costs 1500 image encodes and
a second copy of a 500 page book on disk, per run, for pixels that never differ.

### The last stage is renamed from `done` to `ocr`

`books.Stages` becomes `uploaded`, `dewarped`, `split`, `ocr`, with the title
"OCR". `books.ActionOCR` then leads to `StageOCR`, which is the shape `split`
already has: an action and the stage it fills carry one name.

`done` named the end of the pipeline rather than what a book in it is. Every
other stage names what a book already is, which `stages.go` states as its rule.
A book in the last stage has been read, so `ocr` is the name that follows the
rule.

`0007_split_stage.sql` is the precedent and the migration is the same shape.
Only the OCR action ever made a book carrying `done`, so
`UPDATE book SET stage = 'ocr' WHERE stage = 'done'` moves all of them and
touches nothing else.

The clients need no change. `book-stages` already requires that a client takes
the stage list from the server, and `useStages` does. The `done` in
`bookState.ts` is a card state group, not a stage, and stays as it is.

Alternative, rejected: keep `done` and let the OCR tab be a view inside it. It
leaves two names for one thing, and the stage list is the only place the
interface learns the tab order from.

### The profile lives on the book row

`book` gains `ocr_profile TEXT`, with a check constraint that binds it to the
action, in the shape `0008_page_cut.sql` already uses for `cut_side` and
`cut_column`.

It cannot be derived on a retry. `ocr/worker.go` `rebuild` calls
`store.DeletePages` as its first step, which removes the `page_ocr` rows through
the cascade before anything could read them back. A profile held only in the
HTTP request is lost the moment the first attempt fails.

The column also answers which engine holds the text of record, so no row needs a
flag of its own. `ocr_profile = v6` means the v6 row answers. `ocr_profile = vl`
means the VL row answers and the v6 row is geometry. `ocr_v6.py` states the same
rule: "treat its own text as geometry, not as the answer."

Alternative, rejected: `action_params JSONB` on `book`, general over every
action. No other action takes an argument. The repository adds concrete columns
for concrete facts.

### `page_ocr` stores engine ids, not model titles

`page_ocr.model` holds `v6` or `vl`. These are the words the handler already
takes in `modes` and the words `KNOWN_MODES` lists, so no layer translates.

"PP-OCRv6" and "PaddleOCR-VL-1.6" are titles. `books.Stage` and `books.Action`
already separate an `ID` from a `Title`, and the OCR engines follow them. A
title carries a version, and a version moves: storing "PaddleOCR-VL-1.6" in
every row means an endpoint upgrade rewrites the table.

### The text of a row is what the engine wrote

`page_ocr.txt` holds the bytes of the engine output, unchanged. v6 writes plain
lines in detection order. VL writes markdown. `page_ocr.data` holds the pipeline
result of that engine verbatim.

The two are not the same kind of text, and the row does not pretend they are.
The engine is in the row, so a reader knows which it holds.

`txt` alone is lossy for VL, and the loss is measured. On the page this change
was designed against, VL detected four blocks with `block_order: None` - two
`footnote` at 0.912 and two `number` at 0.765 - recognised all four correctly,
and left all four out of its markdown. They survive in `data.parsing_res_list`.
Anything that later needs a complete page reads `data`, not `txt`.

This follows the handler, which states the same contract: "no fusion, no
post-processing, nothing dropped."

Alternative, rejected: rebuild VL markdown from `data` so the footnotes return.
It puts a reconstruction rule in the backend, and the blocks carry no order, so
the rule would invent one.

### A result pairs by input key and engine

The endpoint answers one entry per engine per page. An entry names its engine
only through the names of the files it uploaded: `<name>.v6.json` and
`<name>.v6.txt` for v6, `<name>.md` and `<name>.json` for VL.

The worker therefore pairs on the two together. `fetchPages` keys its map on the
input key and the engine, not the input key alone, which today lets the second
entry of a page overwrite the first.

`checkOutput` counts per engine. A copy is complete when every engine of the
profile returned every page. `len(output.Pages) != len(sourcePages)` is replaced
by that count, because a two engine profile returns twice the page count and is
correct.

### The dialog offers profiles, not model names

Normal runs `["v6"]`. Extra runs `["v6", "vl"]`.

Naming a choice after one engine would mislead: Extra produces a v6 row as well,
so a label naming only VL describes half the result. The title of a choice therefore
names what it is for, not what it runs: Normal reads "Frontier OCR model", Extra
reads "Visual model. Use for difficult cases like polytonic Greek."

The engine name stays as the subtitle of each choice, so an operator reading a
log or a `page_ocr.model` value still recognises it.

### Extra runs v6 as well as VL

VL gives no recognition confidence. It is generative, so there is no
`rec_score`. v6 gives a score and a box for every line, which is the only
per-line confidence available.

The measurement that matters for a later consumer: on the test page, v6 scored
0.945 on average over Greek lines and 0.995 over Latin lines, but wrong lines
scored high - `λéγεiv`, with three wrong characters, scored 0.989. The score
locates a line. It does not judge one.

## Risks / Trade-offs

- **The freeze invariant is now load-bearing** → `offersPageCut` and
  `offersPageRollback` return false while a copy exists, which is the only thing
  that stops a source image changing under an OCR copy. Both were written for
  another reason and could be relaxed by someone who does not know this depends
  on them. Add a test that asserts a book with a copy offers neither, and name
  this design in its comment.
- **`image_book_key` is a second way to find the files of a page** → Every new
  URL site must read it. The three that exist are listed above. Keeping the
  column `NULL` for every other action limits the blast radius: only OCR copies
  ever set it.
- **The stored result is large** → Measured on one page: 59 KB of v6 result and
  16 KB of VL result, so about 76 KB per page. A 500 page book holds roughly 38
  MB before TOAST compresses it. JSON compresses well and this is acceptable,
  but it is a table that grows with the library, not with the book count.
- **The endpoint contract is proven against one real run** → One page, one job,
  both engines. `tools/fakerunpod` must learn `modes` and the per-engine output
  names, or the e2e suite passes while the real endpoint fails.
- **An old copy holds no rows** → See Migration Plan.

## Migration Plan

1. `0009_page_ocr.sql` creates `page_ocr`, with a unique index on
   `(page_id, model)`, and a cascade from `book_page`. It adds
   `book.ocr_profile` with its check constraint, and `book_page.image_book_key`.
2. The migration backfills `ocr_profile = 'vl'` for every book whose action is
   `ocr`. The endpoint ran VL alone until now, so that names what happened.
3. The migration does **not** move the markdown files of existing copies into
   rows. `db/migrate.go` runs plain SQL and cannot read the filesystem. An
   existing complete copy therefore keeps `ocr_profile` and holds no `page_ocr`
   rows. Its markdown files stay on disk, unreferenced, until the book is
   rejected. An operator who wants rows runs OCR again.
4. `image_book_key` needs no backfill. Every page written before this change
   owns its files, which is what `NULL` means.
5. The migration moves every book carrying the stage `done` to `ocr`. Only the
   OCR action ever made one.
6. Rollback: an older backend ignores `ocr_profile` and `image_book_key`, and
   writes markdown files again. An OCR copy made by the new backend then shows
   broken page images, because the older code builds its URLs from the key of
   the copy. It also does not know the stage `ocr`, so those books fall out of
   every tab until their stage is set back to `done` by hand. Reject those
   copies instead.

## Open Questions

- Does a later reader show both rows of an Extra copy, or the VL row alone
  with the v6 boxes as an overlay? The answer changes no requirement here: both
  rows are stored either way.
