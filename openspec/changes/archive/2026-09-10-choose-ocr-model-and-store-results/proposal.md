## Why

The OCR endpoint now runs two engines. PP-OCRv6 is fast and gives a box and a
confidence for every line. PaddleOCR-VL is slower and reads polytonic Greek,
which v6 cannot. A run against a Spanish page carrying Greek quotations shows
the gap: VL matched ten of ten Greek probes, v6 matched none. The user knows
which kind of book they hold, so the user picks the engine.

The backend cannot reach that endpoint at all today. The handler now demands a
`modes` array and answers with one entry per engine per page. `internal/ocr`
sends no `modes` and expects one entry per page, so every job fails.

A page also needs more than markdown. VL gives no recognition confidence, and
its markdown drops footnotes and page numbers. v6 gives the line boxes and the
confidence VL lacks. Keeping both results per page keeps both answers.

## What Changes

- Add a dialog to the OCR action. The user picks Normal, the frontier OCR model,
  or Extra, the visual model for difficult cases. Normal runs v6 alone. Extra
  runs VL and v6 together.
- Store the choice on the copy as `ocr_profile`. A retry reads it back, so a
  retried copy runs the engines the user picked.
- Add a `page_ocr` row for every engine that read a page. A row holds the
  engine, the text the engine wrote, and the result of the engine verbatim.
- Store the text of each engine as the engine wrote it. v6 writes plain lines.
  VL writes markdown. Neither is rewritten, and the verbatim result holds what
  the text drops.
- **BREAKING** Stop writing a markdown file per page. The text moves to
  `page_ocr`. No client reads these files today.
- Stop building renditions for a page of an OCR copy. The copy points at the
  image of its source page instead. An OCR copy owns no files.
- Carry `modes` in the endpoint job. Read `output_keys` as a list. Accept one
  result per engine per page, and count pages per engine.
- **BREAKING** Rename the last stage from `done` to `ocr`, titled "OCR". Every
  stage is named after what a book in it already is, and a book in the last
  stage has been read. Only the OCR action ever made a book carrying `done`, so
  every one of them belongs in `ocr`.

## Capabilities

### New Capabilities

<!-- None. `book-ocr` already holds the OCR action, its worker and its endpoint
     contract. The results of a page belong to it. -->

### Modified Capabilities

- `book-ocr`: the OCR action leads to the `ocr` stage and carries a model
  profile. The endpoint job names the engines it wants and answers per engine.
  Every page of a copy holds one result row per engine instead of one markdown
  file. A page of a copy shows the image of its source page and builds no
  renditions of its own.
- `book-stages`: the last stage is `ocr`, titled "OCR". The action endpoint
  carries the arguments of an action, and a copy records them. A retry runs the
  action with the arguments the copy holds.
- `book-library`: the tab row ends with "OCR". The OCR action opens a dialog
  that asks for the model before it starts. Every other action starts as before.
- `book-split`: the Split action names the `ocr` stage where it named `done`.
  Its statement that `split` offers no action is corrected: `split` offers OCR.

## Impact

- Database: migration `0009_page_ocr.sql` adds the `page_ocr` table, with one
  row per page per engine and a unique index on the pair. It adds
  `book.ocr_profile`. It backfills `ocr_profile` as `vl` for every copy the OCR
  action made, which is the only engine that ran until now. It moves every book
  carrying the stage `done` to `ocr`, as `0007_split_stage.sql` moved the books
  of the Split action.
- Backend: `internal/books/stages.go` renames the last stage and retargets the
  OCR action at it. `internal/ocr` gains `modes` in its job input, reads
  `output_keys` as
  a list, and pairs a result with its engine. Its worker drops
  `ingest.WriteRenditions` and writes rows instead of files. `internal/books`
  gains the `page_ocr` writes and `ocr_profile`. `internal/httpapi/actions.go`
  reads the arguments of an action and passes them to the queue.
  `internal/filestore` loses `PageMarkdownPath` and `PageMarkdownURL`.
- API: `POST /api/books/actions/ocr` accepts an `ocrProfile` field beside
  `bookIds`. `bookSummary` gains `ocrProfile`.
- Frontend: new `OcrDialog.vue`. `BookActions.vue` and `StageView.vue` open it
  for the OCR action. `useActions.ts` carries the arguments. `api/client.ts`
  sends them.
- Tests: Go tests for the migration backfill, the two profiles, the per-engine
  page count, and a retry keeping its profile. A Playwright spec for the dialog.
- Docs: the README gains the profiles, the `page_ocr` table and `ocr_profile`.

## Non-Goals

- No view of the stored text. The OCR tab lists books as the Done tab does now.
  A reader that shows the text of a page comes later.
- No overlay of the v6 line boxes and confidence on a page image.
- No new stage. The rename gives the last stage the name of the action that
  fills it. It adds no step to the pipeline.
- No fusion of the two results. Nothing merges v6 text into VL text, and nothing
  picks a winner per line. The profile alone says which engine answers.
