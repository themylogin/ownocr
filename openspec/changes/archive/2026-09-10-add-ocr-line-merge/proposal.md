## Why

A page read with the `vl` profile holds two results that each answer half the
question. PP-OCRv6 reports a box for every line, which a renderer needs to draw
text over the image. PaddleOCR-VL reads characters v6 cannot: on book 3292 v6
writes `φiλoσo-φíα` with Latin lookalikes where VL writes `φιλοσοφία`.

Nothing joins them today. `choose-ocr-model-and-store-results` named that fusion
a non-goal, so a renderer must pick one result and lose the other half.

Book 3292 shows the fusion is worth doing and shows what it must survive.
PaddleOCR-VL fails on 2 of its 124 pages, and it fails in two ways. On page 116
it repeats `ἐπὶν αἰς ἐν ἀγαθίας,` about ninety times after a correct prefix. On
page 119 it answers 411 characters for a region holding 2044. A merge that trusts
VL blindly destroys those two pages.

## What Changes

- Add a merge that fills the line boxes of PP-OCRv6 with the text of
  PaddleOCR-VL. It groups v6 lines by the VL block whose box holds them, aligns
  the two character streams, and cuts the VL text at the v6 line boundaries.
- Add a third `page_ocr` row per page, with the model `merged`. The OCR worker
  writes it after it collects both engine results. Only a copy with the profile
  `vl` gets one, because a copy with the profile `v6` has nothing to merge.
- Keep the two engine rows exactly as they are. The merged row is derived, and
  the row it derives from stays verbatim.
- Refuse VL text per line when the alignment shows the two engines did not read
  the same line. Such a line keeps its v6 text, and the merged row records the
  refused text. Every VL character is rendered into a line or recorded as
  dropped, so nothing disappears without a record.
- Keep VL markup verbatim. A line reads `misma $ ^{1} $ está` where VL wrote it.
- Add `backend/tools/ocrmerge`, which writes the merged row for a copy that was
  read before this change. A retry would upload every page to the endpoint
  again, which costs GPU time for a result the database already holds.

## Capabilities

### New Capabilities

- `page-text-merge`: joining the line boxes of one engine with the text of
  another. It covers the grouping, the alignment, the cut, the refusal of text
  the two engines do not agree on, and the accounting that proves no text was
  lost in silence.

### Modified Capabilities

- `book-ocr`: a page of a copy with the profile `vl` holds a third result, whose
  model is `merged`. The rule that no result merges the text of another engine
  applies to the engine rows, which the merged row does not touch.

## Impact

- Database: no migration. `page_ocr.model` is plain `TEXT` with no check, and the
  unique index on `(page_id, model)` already admits a third value.
- Backend: new package `backend/internal/ocrmerge` holds the algorithm, with no
  dependency on the store or the endpoint. `internal/ocr/worker.go` decodes
  `data.lines` of the v6 result and `data.parsing_res_list` of the VL result,
  runs the merge, and writes the third row in `storePage`.
- Tools: new `backend/tools/ocrmerge` writes merged rows for an existing copy.
- Tests: Go tests for the merge, driven by fixtures taken from book 3292. They
  cover a clean page, a hyphenated line break, a Greek quotation v6 read wrong,
  a page where VL repeats itself, a page where VL answers too little, lines
  outside every VL block, and the accounting invariant.
- Docs: the README gains the `merged` row.

## Non-Goals

- No renderer. No API route serves the merged lines, and no page of the frontend
  draws them. This change fills the database, and a reader comes later.
- No change to the endpoint or to `runpod/paddleocr-vl`. Both engines already
  return what the merge needs.
- No merge for the `v6` profile. One engine leaves nothing to join.
- No repair of what VL got wrong. The merge refuses text it cannot trust and
  falls back to v6. It never rewrites a character of either engine.
- No word or glyph boxes. The merge cuts text at v6 line boundaries and reports
  one box per line, as v6 does.
