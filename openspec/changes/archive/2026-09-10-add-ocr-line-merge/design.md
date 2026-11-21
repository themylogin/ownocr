## Context

See proposal.md - Why.

Every number below comes from book 3292 in the development database: 124 pages,
4087 PP-OCRv6 lines, 558 PaddleOCR-VL blocks holding text. A Python model of this
design ran against all of it.

**The markdown is the wrong input.** `page_ocr.txt` of the `vl` row drops footnote
and page number blocks, as `book-ocr` already records. `page_ocr.data` does not:
`parsing_res_list` holds every block, each with `block_bbox`, `block_label` and
`block_content`. Page 10 shows 6 blocks there against 3 paragraphs in the
markdown. The merge reads `parsing_res_list`.

**The v6 side is `data.lines`.** It holds `bbox`, `text` and `score` per line,
already flattened from the raw `rec_boxes` and `rec_texts`.

**Both engines read the same pixels.** That makes geometry a free anchor. Of 4087
v6 lines, 3885 fall inside exactly one VL block with more than 0.9 of their area,
202 fall inside none, and none is ambiguous by any reading. A page therefore
splits into small independent problems, and the two engines are never asked to
agree on the order of a page.

**The two engines disagree in three ways.** VL reflows a paragraph that v6 breaks
into lines, and drops the hyphen v6 reports. VL reads polytonic Greek that v6
writes with Latin lookalikes. VL sometimes fails outright, which is the hard part.

## Goals / Non-Goals

**Goals:**

- A pure function from two engine readings of one page to one list of lines.
- An exact partition of the VL text, provable by counting characters.
- A refusal rule that survives a VL failure without refusing a VL correction.
- No dependency on any script, alphabet or language.

**Non-Goals:**

- Word boxes or glyph boxes. The merge cuts at v6 line boundaries.
- Reflowing the answer into paragraphs. The answer is lines, as v6 gave them.
- Choosing per character between the two engines. The block that aligns wins
  whole, or the line falls back whole.

## Decisions

### The merge groups by box overlap before it aligns anything

Each v6 line goes to the VL block covering the largest share of the line box, and
only when that share reaches 0.5. Alignment then runs per block, over a few
hundred characters instead of a few thousand.

The alternative is one alignment per page. It is simpler and it is wrong: it
assumes both engines walk the page in the same order. A page with footnotes gives
them a reason not to, and page 11 of the book proves v6 does not even walk its own
page in reading order. Grouping first makes an order disagreement between blocks
cost nothing.

The 0.5 floor never fires on this book, because every overlap is above 0.9 or
zero. It guards the case where a line clips the corner of a neighbouring block.

### The lines of a group are sorted, never trusted

Page 11 lists a line at `y=239` before one at `y=250`, then one at `y=292` after
one at `y=303`. The merge groups lines into rows by top edge, with a tolerance of
half the median line height of the group, then orders each row by left edge.

Two v6 lines can share a row and even overlap: page 11 holds boxes ending at
`x=958` and starting at `x=936` on one row. Left edge order handles it, and the
alignment absorbs the duplicated characters as a short gap.

### Needleman-Wunsch, with substitution cheaper than two gaps

| pair | cost |
| ---- | ---- |
| equal runes | 0 |
| both whitespace | 0 |
| equal after folding | 0.1 |
| anything else | 1.0 |
| gap | 0.8 |

Folding is NFKD, then removal of combining marks, then case folding.

The table holds tenths as integers, not floats. `0.8` has no exact binary form, so
a float table sums to values that differ from the true cost by a hair. Where many
paths tie, which they do all over a block the text engine filled by repeating
itself, that hair decides the cut. Integers make the choice reproducible.

A substitution costs 1.0 and a gap pair costs 1.6, so the alignment prefers to
pair two unlike characters over dropping both. That single relation is what makes
a Greek line work. `φiλoσo` against `φιλοσο` pairs six for six, three of them
exactly, instead of tearing into gaps.

The whole design turns on the difference between a substitution and a gap. A
substitution says both engines saw a glyph here and read it differently, which is
the case the merge exists to resolve. A gap says one engine saw text the other
never saw, which is the case the merge must be careful about.

Levenshtein alone was considered and rejected: it gives a distance, not a path,
and the path is the whole point. Anchor based diffing was considered and rejected
as unnecessary at these lengths.

### The cut is an exact partition, and two boundaries are easy to get wrong

The alignment path gives `cut[i]`, the block position reached at line-text position
`i`. Piece `k` is `block[lo:hi]` where `hi` is the `lo` of the next piece. Two
boundaries do not follow from that and must be written by hand:

- `lo` of the first piece is `0`, not `cut[0]`. Block characters inserted before
  the first line otherwise vanish. This lost 10 characters on page 2 alone.
- `hi` of the last piece is `len(block)`.

The line separator is the other trap. Joining lines with `\n` puts a character
between two lines that the block can align text against. Taking piece `k` as
`block[cut[start_k] : cut[end_k]]` drops whatever aligned to the separator. That
lost 184 characters across the book. Ending each piece where the next one begins
makes the partition exact by construction, and the character count then closes at
zero.

### The gate measures agreement after folding, per line

For each line, count the characters of the v6 line that the alignment paired with
a block character equal after folding, and divide by the length of the v6 line.
Refuse the piece below 0.30, unless the v6 line holds fewer than 20 characters.

Three alternatives were measured and rejected.

**Normalised alignment cost per block.** Rejected: it is actively harmful. It
rates a VL correction and a VL failure the same way, because both differ from v6
in most characters.

| block | normalised cost | truth |
| ----- | --------------- | ----- |
| p112 blk4 | 0.777 | VL wrote Mongolian script |
| p116 blk0 | 0.688 | VL repeated one phrase ~90 times |
| p119 blk1 | 0.652 | VL answered 411 chars for 2044 |
| p6 blk0 | 0.392 | correct, a table of contents |
| p14 blk5 | 0.318 | correct, a Greek quotation v6 destroyed |
| p18 blk1 | 0.264 | correct, a Greek quotation v6 destroyed |

The median block scores 0.0087. There is no threshold that keeps page 14 and
drops page 116.

**Longest unaligned run.** This works well: the two failures show runs of 935 and
492 characters, and the worst legitimate block shows 11. It was dropped as
redundant. A long run of block characters lands inside one line, whose agreement
then collapses, and a long run of line characters leaves those lines paired with
nothing, whose agreement is zero. The per line gate catches both, so the merge
carries one rule instead of two.

**Repetition or compressibility of the block.** `len(zlib(block))/len(block)`
scores 0.084 on page 116 and 0.303 on page 112, against 0.426 for the most
repetitive correct block. It separates, but the margin is thin and zlib on a
70 character string measures its own header as much as the text. Trigram diversity
does not separate at all: page 112 scores 0.353 against 0.346 for a correct block.
Neither is needed once the per line gate is in.

Folding is what makes agreement the right measure. Bare pairing does not work: on
page 116 the alignment pairs the repeated Greek one for one against correct
Spanish, so the pairing share is 1.0 and says nothing. Requiring the pair to be
equal after folding drops that to 0.13.

### A second gate on the length of a piece against its line

The agreement share answers "did the two engines read the same line". It cannot
answer "did the text engine stop at the end of that line". A line whose opening is
correct agrees well, so the share accepts it, and the cut then hands it every
character up to the next line that aligned. When the text engine ran on, that is
the whole run.

So a piece longer than `4 * line + 20` characters is refused, whatever it agreed.
Measured over all 3840 accepted pieces of the book, the gate trips exactly once:

| piece / line | page | line | piece | agreed | truth |
| ------------ | ---- | ---- | ----- | ------ | ----- |
| 10.92 | 116 | 64 | 699 | 0.53 | the text engine ran on |
| 2.39 | 24 | 56 | 134 | 0.79 | correct, a Greek word written as a formula |
| 2.00 | 34 | 62 | 124 | 0.84 | correct, the same |
| 1.45 | 85 | 31 | 45 | 1.00 | correct, accents the box engine dropped |
| 1.02 | median of the book | | | | |

The margin is wide in both directions: the one failure sits 2.5 times over its
allowance, and the widest correct piece sits at 0.55 of its own. What inflates a
correct piece is the text engine writing Greek or a formula as LaTeX, which runs
to about 2.4 times the line. Four leaves room for a denser page of formulas.

The 20 character allowance is what lets one rule cover a short line too. `SR`
against `2(30)(24)MAK` is a ratio of 6 and 12 characters against an allowance of
28, so it passes. Without the constant this gate would need the same short line
exemption the share gate has, and with it the gate needs no exception at all.

### The short line exemption at 20 characters

Sorted by agreement, the 30 worst lines of the book are all from pages 116, 119
and 112. The lowest correct scores are these:

| line | length | agreement | truth |
| ---- | ------ | --------- | ----- |
| `δoxovτα.` -> `δοχοῦντα.` | 9 | 0.500 | correct |
| `etcétera.` -> `etc.` | 9 | 0.444 | correct |
| `FELUIPE(  AR` -> `DE GRECIA Y LA FILOSOFÍA •` | 12 | 0.417 | correct |
| Mongolian, p112 | 39 | 0.243 | failure |
| repeated Greek, p116 | 55 to 63 | 0.131 to 0.262 | failure |

Every correct line under 0.55 is under 20 characters, and every failure over 20
characters is under 0.30. The exemption states the reason directly: a nine
character string carries too little evidence to judge, and refusing it costs a
line either way.

### Constants live in code

`MinOverlap 0.5`, `MinAgreement 0.30`, `ShortLine 20`, and the cost table. They
come from one book, so a named constant with the measurement beside it is honest.
A configuration file would invite tuning per book, which is how a generic
algorithm turns into a per book hack.

### The merge is a package with no dependencies

`backend/internal/ocrmerge` takes two Go structs and returns one. It reaches no
database, no object store and no endpoint. Its tests read JSON fixtures.
`internal/ocr/worker.go` decodes the two engine results, calls it, and writes the
row. The tool of the migration plan calls the same function.

### The merged row

`page_ocr.model` is `merged`. `txt` is the merged lines joined by newline, which
is what a reader wants. `data` holds the structure:

```json
{
  "lines": [{"bbox": [164,926,1359,972], "text": "...", "source": "vl", "score": 0.99}],
  "dropped": [{"text": "...", "reason": "line"}],
  "counts": {"lines": 22, "fromText": 22, "fromBoxes": 0, "droppedChars": 0}
}
```

`source` is `vl` or `v6`. `score` is the v6 confidence of that line, carried
through because a renderer wants it and the merge does not change it. `reason` is
`line`, `block` or `unaligned`, matching the three refusals in the spec.

No migration adds this. `page_ocr.model` is `TEXT` with no check constraint, and
the unique index on `(page_id, model)` already admits a third value.

### Runes, not bytes

Every index is a rune index. Greek is multibyte in UTF-8, and a byte alignment
would pair fragments of characters and report nonsense agreement.

## Risks / Trade-offs

- **VL fails part way through a line, and the share alone accepts the whole
  piece.** Page 116 holds one line reading `dualidad de ser y no ser.` followed by
  repeated Greek. Its first half is correct, so it agrees on 0.531. Mitigation: the
  second gate above refuses it on length. Raising the share threshold instead
  would need to pass 0.55, which would start refusing correct Greek quotations.
- **A page of formulas could trip the length gate.** The text engine writes Greek
  and mathematics as LaTeX, which runs a correct piece to about 2.4 times its
  line. A page denser in formulas than any in this book could pass 4. Mitigation:
  none. The line keeps its box engine text and the piece is reported, so the loss
  is visible in the counts rather than silent.
- **Alignment cost is the product of the two lengths.** A runaway VL result could
  be tens of thousands of characters. Mitigation: refuse a block whose lengths
  differ by more than eight times, and refuse a block whose product of lengths
  passes a cap, before any table is allocated. Both refusals report their text.
- **The thresholds come from one book.** A book in a different language or script
  could sit elsewhere in the distribution. Mitigation: the merged row reports its
  counts per page, so a bad book is visible without re-reading it. The measured
  margins are wide: refusals cluster below 0.27 and acceptances above 0.42.
- **The hyphen disappears.** `impor-` and `tantes` become `impor` and `tantes`.
  For a text layer this is right, and copy-paste yields the joined word.
  A renderer that wants the printed hyphen back cannot get it from this row.
- **A block covering a figure empties nothing but wins nothing.** Two lines in the
  book sit under a VL block with empty content. They keep their v6 text, which may
  be noise read out of a picture. The merge never deletes text, so noise survives.

## Migration Plan

Existing copies with the profile `vl` hold no merged row. A retry would rebuild
the copy from the endpoint, which uploads every page and spends GPU time to
recompute results the database already holds.

`backend/tools/ocrmerge` takes a book id, reads the `v6` and `vl` rows of every
page of that copy, runs the merge, and writes the `merged` row. It is idempotent:
`InsertPageOCR` replaces a result for the same page and model. It changes no
engine row, so a bad run is fixed by running it again after a fix.

Rollback is `DELETE FROM page_ocr WHERE model = 'merged'`. No other row and no
other table refers to it.

## Open Questions

- Whether a renderer wants the dropped text at all. The merged row records it
  either way, so a reader can show it, ignore it, or warn on it. This is decided
  when the reader is built, and changes nothing here.
