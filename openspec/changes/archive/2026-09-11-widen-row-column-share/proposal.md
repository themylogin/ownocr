## Why

Page 85 of book 17 reads `pero no` twice.

The printed row is `pero no en el sentido de que la actuación de Alcibíades`.
PP-OCRv6 drew it as three boxes, and drew them overlapping: the box holding `en`
reaches x 456 and the box holding `el sentido` starts at x 408. They share 48
pixels of a box 130 pixels wide, which is 0.37 of the narrower.

`maxRowShare` allowed two boxes of one row to share 0.25 of the narrower column.
At 0.37 the row grouping refused them, so the row split in two. The box above
then took the head of the row on top of its own text, the two boxes holding that
head kept what the box engine read, and the page carried the phrase twice:

```
line 22  '...con la ruina de Atenas, pero no en'
line 23  'pero no'     <- box engine text, drawn over the same words again
line 24  'en'          <- the same
```

The share is what the measure of a row rests on, and 0.25 was read off one page
of one book.

## What Changes

- `maxRowShare` rises from 0.25 to 0.40. Two boxes of one printed row may share
  up to 0.40 of the narrower column and still be read as one row.
- Nothing else changes. The vertical test that keeps two rows apart is untouched.

### Non-goals

- No change to how a row is ordered, cut, or filled.
- No repair of the two boxes of page 85 whose text runs past their ink. PP-OCRv6
  left 118 and 145 pixels of that page's ink uncovered in the middle of two rows,
  and the text of the uncovered stretch lands in the box before the gap. That is
  a different fault and wants a box placed over the gap.

### The measure

Over the 183 pages of books 17 and 18, the share two boxes of one column have of
each other is bimodal:

```
two halves of one printed row : up to 0.37
two different printed rows    : 1.00
between them                  : nothing
```

0.40 sits at the near end of that empty gap. Every value from 0.40 to 0.90
answers all 183 pages alike, because the vertical test refuses two rows that do
share a column. The threshold was chosen at the low end of the range that works,
not in the middle, so it grants the least it can.

Run through the real merge over all 183 pages and compared against the merged
result the previous code stored:

```
178 pages  byte-identical
  5 pages  changed, every one of them for the better
  0 pages  worse
book 18    no page changed
```

| page | was | now |
|---|---|---|
| 85 | `pero no` twice, 2 boxes kept box engine text | each of the 3 boxes holds its own ink |
| 88 | `ef` in one box and `efec` in the next | `ef` once, from the text engine |
| 89 | `con` split as `con` and `n`, and `tambié` short of its `n` | each box holds its own ink |
| 106 | the footnote marker box read `f` over ink that reads `43` | it reads `43` |
| 87 | the disaster block of `add-ocr-disaster-interpolation` | unchanged by this change |

## Capabilities

### Modified Capabilities

- `page-text-merge`: the requirement that the merge reads the lines of a block in
  reading order states the tolerance a row rests on. The share two boxes of one
  row may have of each other's column changes.

## Impact

- `backend/internal/ocrmerge/ocrmerge.go`: one constant, and the distribution
  recorded beside it.
- `maxRowShare` is read by `beside` in `layout.go` and by `besideInColumn` in
  `disaster.go`. The second decides which pairs of boxes the disaster criterion
  compares, so a looser share lets it weigh more pairs. Page 87 is still the only
  block of the two books it calls unusable.
- `backend/tools/ocrmerge -all` rewrites the stored merged results from the two
  engine results. No page is read again.
