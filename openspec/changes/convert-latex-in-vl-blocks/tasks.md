## 1. The cleanup package

- [x] 1.1 Create `backend/internal/latex/latex.go` with one exported function
  that reads a text and answers the cleaned text, and verify `go build ./...`
  passes
- [x] 1.2 Add the marker rule, matching `[ \t]*\$\s*(\^|_)\{(\d+)\}\s*\$` whole
  and answering superscript or subscript digits, and verify unit tests for the
  four fixture shapes: `misma $ ^{1} $ está`, `se describen $ ^{50} $. Dentro`,
  `se ofrece$^{51}$ del`, and `el agua $ _{2} $ hierve`
- [x] 1.3 Verify a unit test that the marker rule leaves `la nota $ ^{a} $ del
  pie` and `it costs $5 and I paid $12` exactly as it read them
- [x] 1.4 Add the Greek map: the 24 lowercase letters, the `var` forms, and the
  uppercase letters that differ from Latin, and verify a unit test that every
  entry answers one rune and that `\etax` is not read as `\eta` followed by `x`
- [x] 1.5 Add the gate — the text holds both a rune of the Greek and Coptic or
  Greek Extended block and a known Greek command — and verify unit tests that
  `un texto con τὸ ἐόν pero $ sin $ comandos` and `la relectura $ \alpha $ del
  rasgo, sin griego` both come back unchanged
  (**superseded by 4.1**: the rune half of this gate was wrong and is gone)
- [x] 1.6 Add the rescue for a gated text: replace every known Greek command,
  replace every dollar sign with a space, collapse repeated spaces, drop a space
  in front of `,.;:!?»` and behind `«¿¡`, and verify a unit test that
  `es $ ᄋλὴ $ \eta\theta\epsilon ῥ\eta $ o $ τὸ $ $ ἐóν $, pero $ ἱ $ $ cómo $ se
  llama` answers `es ᄋλὴ ηθε ῥη o τὸ ἐóν, pero ἱ cómo se llama`
- [x] 1.7 Add the paragraph from the report as a fixture — the `ᄋλὴ
  \eta\theta\epsilon ῥ\eta` text of 27 dollar signs and 12 commands, joined into
  one block as the engine writes it — and verify a test that the answer holds no
  dollar sign, no backslash, no double space, and the words `μὴ εἴναι`
- [x] 1.8 Verify a unit test that the marker rule runs before the rescue: a gated
  text holding `$ ^{12} $` answers `¹²` joined to the word in front, and holds no
  dollar sign
- [x] 1.9 Verify unit tests for the untouched cases: ordinary Spanish prose, an
  empty text, `C:\temp\newfile.txt`, `la razón $\frac{a+b}{2}$ del texto`, and an
  unknown command inside a gated text (`\foobar` survives)
- [x] 1.10 Verify a unit test that the cleanup is repeatable: for every case
  above, a second reading of the answer changes nothing

## 2. The merge reads cleaned blocks

- [x] 2.1 Call the cleanup on `block.Text` in `MergeOf`
  (`backend/internal/ocr/merge.go`), while it builds `[]ocrmerge.Block`, and
  verify the existing `ocr` merge tests still pass
- [x] 2.2 Verify a test in `backend/internal/ocr/merge_test.go` that `MergeOf` on
  the page 10 fixture answers the line `Platón «socrático». Con la palabra
  misma¹ está vinculada una`, where it answered the marker before
- [x] 2.3 Verify a test that `MergeOf` changes neither input: the bytes of the
  `v6` result and of the `vl` result are the same after the call as before it
- [x] 2.4 Verify that `backend/internal/ocrmerge` is untouched — no import of the
  new package, and `golden_test.go` keeps `$ ^{1} $` in `pageTenMerged`, because
  it calls `Merge` directly and the cleanup never reaches it

## 3. The recorded reading

- [x] 3.1 Add a golden test at the `MergeOf` level for `page0014` (the Greek
  quotation PP-OCRv6 destroyed), recording every line it answers, so a later
  change to the cleanup shows up as a diff
- [x] 3.2 Verify a test that `³` folds onto `3` through `ocrmerge`'s NFKD fold, so
  the claimed alignment gain of design.md — Decision 3 is asserted and not
  assumed
- [x] 3.3 Run `gofmt -l backend/internal` and `go vet ./...` and verify both
  answer nothing

## 4. The evidence is a command alone

A second page showed a block that reads as Greek and holds no Greek rune, so the
gate of group 1 never fired on it. See design.md — Decision 1, 1b and 1c.

- [x] 4.1 Drop the Greek-rune half of the gate, so a known Greek command is the
  whole of the evidence, and verify a unit test that
  `la relectura $ \alpha $ del rasgo, sin griego` now answers
  `la relectura α del rasgo, sin griego`
- [x] 4.2 Add the accent unwrap — `\ddot{\alpha}` answers the letter alone — and
  the `misread` map that reads `\circ` as an omicron, and verify a unit test that
  the `contraposición de \lambda\eta\theta\epsilon\iota\eta` line answers
  `contraposición de ληθειη a δδξα o de τα éóv a ταδκουντα.`
- [x] 4.3 Verify a unit test that the fixture of 4.2 holds no Greek rune, so the
  test proves the rune is no part of the evidence
- [x] 4.4 Keep `\circ` out of the gate, and verify a unit test that
  `the map $ f \circ g $ and the angle $ 30 \circ $` keeps every dollar sign
- [x] 4.5 Verify that the repeatability test covers both new fixtures

## 5. The whole suite

- [x] 5.1 Run the backend suite on a scratch database, not the dev one, and
  verify every package passes
