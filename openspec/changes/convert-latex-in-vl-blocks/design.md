## Context

See proposal.md — Why. The requirements live in
`specs/page-latex-cleanup/spec.md` and `specs/book-ocr/spec.md`.

Two constraints shape everything below.

The first is what the engine actually writes. A survey of the fourteen fixtures
in `backend/internal/ocrmerge/testdata/` finds a dollar sign on five pages and a
backslash on none. Every dollar sign of those five is a footnote marker, always
`$ ^{nn} $`. The second shape comes from a page outside the fixtures, where the
engine reads `ἀλήθεια` as `$ ᄋλὴ $ \eta\theta\epsilon ῥ\eta $`. Those are the two
shapes, and there is no evidence of a third.

The second is where the text sits in the pipeline.

```
  vlocr.py -> vl JSON: blocks of whole paragraphs
                  |
                  v
              MergeOf()  in backend/internal/ocr/merge.go
                  |
                  v
        ocrmerge.Merge() -> lines, cut at the printed rows
                  |
                  v
              merged text -> API -> reader
```

`MergeOf` is the only place the system turns a PaddleOCR-VL result into blocks,
and both `ocr/worker.go` and `ocr/rescan.go` reach the merge through it.

## Goals / Non-Goals

Goals:

- Convert the two shapes above, and nothing else.
- Read a block whole, so no rule depends on where the merge cuts a line.
- Leave the stored engine results untouched.

Non-Goals (beyond the proposal's):

- No LaTeX parser, no tokenizer, no grammar. Two regular expressions and a map.
- No configuration. The rules are the same for every book.
- No frontend work at all.

## Decisions

### Decision 1: Evidence gates the dollar signs; nothing pairs them

The rejected alternative is the obvious one: find `$...$` spans and convert their
contents. It fails on the real text, and the failure is measurable.

The sample paragraph holds **27** dollar signs. The count is odd, so no pairing
can read them as delimited spans. The engine does not use them as delimiters at
all: it wraps single words in them, and the words include `$ o $` and
`$ cómo $`, which are ordinary Spanish.

Worse, one unpairable dollar sign shifts the parity of every dollar sign behind
it:

```
  in   ... $ τὸ $ $ ἐóν $, pero $ ἱ $ $ cómo $ ...
             ^--^ ^-^     ^--------^   ^------^
                          this span holds prose, so a
                          "does it look like math" guard
                          correctly refuses it -- and the
                          dollar signs stay on the page
  out  ...  τὸ   ἐóν $, pero $ ἱ   cómo $ ...
```

A run of the proposed converter on the sample leaves 7 of the 27 dollar signs
and 14 double spaces.

The rule this change takes instead: decide once, for the whole block, whether the
block is one the engine mangled. The evidence is a LaTeX Greek command. On that
evidence every dollar sign of the block goes, whether it pairs or not. The same
run leaves 0 of 27.

The cost of the gate is stated in the spec: a block that holds dollar-wrapped
Greek but no command keeps its dollar signs. That is the price of never
rewriting a block on a guess.

**The evidence was two things and is now one.** The first draft also asked for a
Greek rune beside the command, because the paragraph above holds both. A second
page proved that wrong:

```
  contraposición de $ \lambda\eta\theta\epsilon\iota\eta $ a $ \delta\delta\xi\alpha $
  o de $ \tau\ddot{\alpha} $ éóv a $ \tau\ddot{\alpha}\delta\kappa\circ\bar{\upsilon}\nu\tau\alpha $.

  Greek runes ......... 0
  Greek commands ...... 20
```

The block reads as Greek and holds no Greek rune. `éóv` is a Latin e (U+00E9), a
Latin o (U+00F3) and a Latin v (U+0076), for a word the book prints `ἐόν` — the
engine writes Latin lookalikes wherever it does not write a command. `ocrmerge`
knows the same habit; its `page0010` fixture note calls it "Greek v6 wrote with
Latin lookalikes".

So a rune says nothing about whether the engine mangled a block, and asking for
one leaves the worst blocks untouched. A LaTeX command says everything: a printed
book holds none, so the engine wrote every one that appears.

### Decision 1b: An accent is the engine guessing, and the letter is what survives

The same block writes `\ddot{\alpha}` and `\bar{\upsilon}`. The book prints `τὰ`
and `οῦ`, so the accent the engine picked is not the accent on the page — a
diaeresis where the page has a grave. It is the engine reporting that it could
not place the letter.

The cleanup therefore answers the letter under the accent and drops the accent.
It loses nothing true and it takes `\ddot{α}` off the reader's page.

### Decision 1c: `\circ` is an omicron here and a ring operator everywhere else

The block spells `δοκοῦντα` as `\delta\kappa\circ\bar{\upsilon}\nu\tau\alpha`.
The engine saw the circle of an omicron and wrote the command for a circle.

Reading `\circ` as `ο` is right for this corpus and wrong for mathematics, so it
is converted but it is **not evidence**. A block earns the rescue through a
command in the Greek map and never through `\circ` alone, which keeps
`$ f \circ g $` and `$ 30 \circ $` intact in a block of real mathematics.

This is the one rule here that rests on the corpus rather than on the markup. It
is the reason `misread` is a separate map from `greek` in the code: the split is
what keeps it out of the gate.

### Decision 2: The marker is one regular expression, matched whole

```
  [ \t]*\$\s*(\^|_)\{(\d+)\}\s*\$
```

It matches the whole shape at once. It pairs nothing, so the parity problem of
Decision 1 cannot reach it, and it needs no gate: `$5` and `$12` are not
candidates, because a digit is not `^{`.

The leading `[ \t]*` is what joins the marker to its word, as the printed page
sets it: `misma $ ^{1} $ está` becomes `misma¹ está`.

The body is `\d+` and not `[^{}]*` on purpose. Every digit has a Unicode
superscript and subscript; most other characters do not. A body the converter
cannot spell in full is left whole rather than converted in half.

This rule runs first, so a block the Greek rule also rescues still gets `³³` and
not a stray `^{33}` after its dollar signs go.

### Decision 3: Clean at the block, before the merge

Three places could host the cleanup. The reader was the first suggestion.

| Where | Span intact? | Helps the merge? | Places to change |
|---|---|---|---|
| reader (Vue) | no — already cut into rows | no | 2 components, plus exports |
| `MergeOf` | yes — a block is a paragraph | yes | 1 function |
| `vlocr.py` | yes | yes | breaks the raw archive |

`MergeOf` wins on all three counts.

**Span intact.** After the merge the text is cut at the printed rows. The sample
shows a marker split across two rows (`μὴ $` / `εἴναι $`). A rule that reads one
row cannot see the whole fragment; a rule that reads a block always can.

**Helps the merge.** `ocrmerge/fold.go` folds with NFKD before it compares runes.
`³` folds to `3` — confirmed against the Unicode data — which is exactly what
PP-OCRv6 reads for a footnote marker. Today the ten runes of `$ ^{33} $` align
against nothing and pay `costGap` (8) for most of them. After the cleanup `³³`
folds onto v6's `33` at `costEqual`. Cleaning the markup is not only cosmetic.

**The raw archive.** `vlocr.py` says it: "Each engine writes everything it
produces; nothing is fused or post-processed." The `vl` row stays the archive,
and the `merged` row is derived, so the merged row is the right one to clean.

### Decision 4: A new `latex` package, not a change to `ocrmerge`

`ocrmerge` opens by saying it "holds no list of characters and knows no
alphabet". The cleanup is a list of characters and one alphabet. Putting it there
would contradict the one sentence that keeps that package honest.

`backend/internal/latex/` holds it, beside `guttertrim` and the other packages
that own one concern. It takes a string and answers a string. It reads no box, no
line and no engine name, which is what makes it cheap to test.

### Decision 5: The map holds Greek letters and nothing else

The converter this change came from carried about 100 commands: arrows,
operators, set theory, `\frac`, `\text`. None of them appears in any fixture.
Every entry is a chance to rewrite prose that was already correct, and each one
has to be right in both cases and all its variants.

The map holds the 24 Greek letters, their `var` forms, and the uppercase letters
that differ from Latin. An unknown command survives untouched, so a later book
that needs `\rightarrow` costs one line.

### Decision 6: A removed dollar sign becomes a space, then the spaces collapse

Deleting the dollar sign outright would join words that the markup separated.
Replacing it with a space cannot, and a collapse of repeated spaces afterwards
cleans up what the replacement left. Two small repairs follow: a space in front
of `,.;:!?»` goes, and a space behind `«¿¡` goes.

## Risks / Trade-offs

**A real price inside a rescued block loses its dollar sign.** → The block would
have to hold a price, a Greek rune and a LaTeX Greek command at once. Accepted.
The corpus is Spanish philosophy on ancient Greek.

**The cleanup changes the rune stream the merge aligns, so a page could merge
differently than it does today.** → Mostly for the better, by Decision 3, but
"mostly" is not "always". A golden test at the `MergeOf` level on `page0010`
(two footnote markers) and on `page0014` (a Greek quotation v6 destroyed) records
the new reading, so a regression shows up as a diff and not as a surprise in the
reader.

**`³` might not fold onto `3` and the alignment gain might be imagined.** → A
test asserts it directly rather than trusting the reasoning.

**A book already read keeps its markup.** → Out of scope by the proposal. A
rescan of a page rewrites its merged result, so the escape hatch exists today.

**The gate leaves some blocks half-clean.** → By design, Decision 1. The spec
records it as behavior, not as a defect, so nobody "fixes" it by removing the
gate.

## Migration Plan

No schema change, no data migration, no API change. Deploy is a backend release.

Rollback is a revert. A merged result written by the new code stays valid under
the old code, because it is only text; and either way the `vl` and `v6` rows are
untouched, so any page can be merged again from what it already holds.

## Open Questions

- Should the cleanup also convert a bare marker, one the engine wrote with no
  dollar signs around it? `page0014` holds `ὁνομα^{7}.` and
  `morfológica^{8}]`, and those two lines reach the reader with the markup on
  them. Across the fourteen fixtures the engine writes 8 dollar-wrapped markers
  on four pages and 2 bare ones on one page, so the shape is real and not a
  one-off.

  Left out on purpose. The marker rule matches its whole shape at once, and that
  wholeness is the argument for why it cannot misfire; a rule for
  `([\^_])\{([0-9]+)\}` alone gives that argument up, and it is a separate
  decision from the one this change makes. It would cost a second regular
  expression running after the first, and `page0014` records the reading it
  changes.

- Should an action re-merge a page from its stored `v6` and `vl` results, with no
  GPU work, so a whole book can be cleaned without a rescan? It would answer the
  backfill non-goal. It changes nothing here: the cleanup would sit in `MergeOf`
  either way.
