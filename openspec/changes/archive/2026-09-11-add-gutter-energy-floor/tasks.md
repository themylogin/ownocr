## 1. The floor

- [x] 1.1 Add `MIN_ENERGY_PER_ROW = 0.5` beside the other thresholds in
      `sidecar/ownocr/split/gutter.py`, with a comment naming the window it sits
      in: above 0.164, the strongest false gutter, and below 2.199, the weakest
      detection the middle column cannot replace. Verify the constant reads as a
      count of grey levels, not a ratio.
- [x] 1.2 Return the peak of the chosen profile from `_coarse_line` beside the
      angle, the centre and the prominence. Verify `_gutter` receives it and the
      existing tests still pass.
- [x] 1.3 Reject the candidate in `_gutter` when the peak divided by the height
      of the work image falls below `MIN_ENERGY_PER_ROW`. Verify the check runs
      beside the prominence check, so a rejected page returns `None` and reaches
      `_middle`.

## 2. Tests

- [x] 2.1 Add a fixture to `sidecar/tests/test_split.py` for a spread with a
      blank facing page: a landscape canvas, no gutter drawn, one block of text
      rows in the right half. Verify it reproduces the fault without the floor.
- [x] 2.2 Add a test that the fixture answers the middle column of the image.
      Verify both numbers equal the middle and neither falls inside the block of
      text.
- [x] 2.3 Add a faint gutter fixture: the same page, with the gutter drawn at
      grey 231 against paper at 235. Draw it away from the middle, because a
      rejected gutter answers the middle and an assertion around the middle
      holds either way. It measures 1.94 grey levels per row there, above the
      floor of 0.5 and below the ceiling of 2.199.
- [x] 2.4 Verify task 2.3 covers the spec scenario *A faint gutter on a flat
      book*, which has no test today. Verify it passes at a floor of 1.2 and
      fails at 2.0, so a later raise of the floor fails this test first.
- [x] 2.5 Run the existing suite. Verify all 19 tests still pass. The gutter of
      the current fixture is drawn at grey 90 and measures 106 grey levels per
      row, so it clears the floor by a factor of 200.

## 3. The corpus

- [x] 3.1 Re-measure the 181 pages of books 1, 3, 5 and 6 with the floor in
      place. Verify exactly four cuts change: page 5 and page 41 of book 6 and
      page 24 of book 5, by 14.6 to 18.2 percent of their width, and page 47 of
      book 6 by 3.1 percent onto a middle column 49 pixels clear of ink.
- [x] 3.2 Verify no page of book 1 or book 3 loses its detection, and that the
      middle column of every page that does lose one stands clear of ink.
- [x] 3.3 Verify page 28 of book 6 keeps its cut of 1483 and 1429. It measures
      2.199, the ceiling of the window, and the floor must not reach it.
- [x] 3.4 Verify the count of whole pages does not change: one before and one
      after, page 1 of book 1. Every page the floor rejects is landscape, so it
      still cuts at the middle.

## 4. The six wrong pages

Not done, by the decision of the operator. The floor stops the fault from
happening again; these six pages hold the result of the fault as it was. Re-cut
them whenever it suits, or leave them.

- [ ] 4.1 Re-cut pages 9, 10, 81 and 82 of book 20 at column 1465 through
      `POST /books/20/pages/{index}/cut`. Verify each page comes back with the
      new width and holds no text of its neighbour.
- [ ] 4.2 Re-cut pages 47 and 48 of book 10 at column 1503. Verify the same.
- [ ] 4.3 Verify no other page of book 20 or book 10 changed, because the cut
      action touches one page at a time.
