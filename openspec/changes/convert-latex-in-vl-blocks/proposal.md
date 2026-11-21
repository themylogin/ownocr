## Why

PaddleOCR-VL writes LaTeX into the text of a page it reads. It wraps a footnote
marker as `$ ^{33} $`. It reads a Greek word as a mixture of Greek runes and
LaTeX commands, and wraps the pieces in dollar signs. The reader shows that
markup as it stands.

Five of the fourteen merge fixtures hold a dollar sign. Every one of them is a
footnote marker. A sixth page reads a Greek word as
`$ ᄋλὴ $ \eta\theta\epsilon ῥ\eta $`, where the book prints `ἀλήθεια`.

The markup costs more than its own ugliness. The merge aligns the runes of both
engines, and PP-OCRv6 never reads a dollar sign or a backslash. Ten runes of
`$ ^{33} $` therefore align against nothing, and each one pays the gap cost of a
rune one engine saw and the other missed.

## What Changes

- A cleanup step reads the text of every PaddleOCR-VL block before the merge
  joins it. It answers plain text for the two markup shapes the engine produces,
  and leaves every other block as it stands.
- **Footnote markers**: the cleanup replaces `$ ^{33} $` with `³³` and
  `$ _{2} $` with `₂`, in every block. It joins the marker to the word in front
  of it, as the printed page does.
- **Greek words**: the cleanup replaces a LaTeX Greek command with its Unicode
  letter, and removes every dollar sign of the block. It does this only for a
  block that holds both a Greek rune and a LaTeX Greek command.
- The merged result of a page holds the cleaned text. The PP-OCRv6 result and
  the PaddleOCR-VL result keep the text their engine wrote.
- The reader needs no change. It shows the text of the merged result.

Non-goals:

- No backfill. A page the system has already read keeps the markup in its merged
  result until something merges that page again.
- No general LaTeX converter. The cleanup knows the two shapes above and the
  Greek alphabet. It leaves `\frac`, `\text`, `$$`, `\(` and every other
  construct untouched.
- No change to the reader and no change to the API.

## Capabilities

### New Capabilities

- `page-latex-cleanup`: reads a text that holds LaTeX fragments and answers the
  plain text of it. It names the two shapes it converts, the evidence a block
  needs before it loses its dollar signs, and everything it refuses to touch.

### Modified Capabilities

- `book-ocr`: the merged result of a page derives from the cleaned text of the
  PaddleOCR-VL blocks, not from the text those blocks carry.

## Impact

Backend:

- `backend/internal/latex/latex.go`: new package. One function reads a block and
  answers the cleaned text.
- `backend/internal/ocr/merge.go`: `MergeOf` cleans the text of every block it
  reads from the PaddleOCR-VL result, before it hands the blocks to the merge.

Nothing else calls the endpoint or the merge, so `worker.go` and `rescan.go`
gain the cleanup through `MergeOf` and change in no other way.

The `ocrmerge` package does not change. It holds no list of characters and knows
no alphabet, and the cleanup is the opposite of both.

Tests: `backend/internal/latex/latex_test.go` (new),
`backend/internal/ocr/merge_test.go`.

The fixtures of `backend/internal/ocrmerge/testdata/` hold the result of both
engines as `page_ocr.data` holds it. They stay as they are, and the golden tests
of `ocrmerge` keep their recorded text. Those tests call the merge directly, so
the cleanup never reaches them. A golden test at the `MergeOf` level covers the
cleaned reading of page 10.
