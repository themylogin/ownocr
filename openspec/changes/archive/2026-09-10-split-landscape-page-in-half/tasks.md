## 1. The fallback in the sidecar

- [x] 1.1 Add `LANDSCAPE_RATIO = 1.2` beside the other thresholds in
  `sidecar/ownocr/split/gutter.py`, with the reason it is not `width > height`.
  Verify `pytest` still collects and the module imports.
- [x] 1.2 Answer the middle column in `columns` when `_gutter` finds nothing and
  the image is landscape, as `(width // 2, width // 2)`. Verify a 1600 by 1200
  image with no gutter answers the pair 800 and 800.
- [x] 1.3 Leave every other path untouched: a found gutter still decides the
  cut, and an image that is not landscape still answers null. Verify by reading
  the diff that no threshold and no other branch moved.

## 2. The tests the new rule contradicts

- [x] 2.1 Add a portrait canvas to `sidecar/tests/test_split.py`, beside the
  1200 by 900 one, so a test can build a single page the rule leaves whole.
  Verify the helper draws one block of text rows and no gutter.
- [x] 2.2 Move `test_single_page_returns_none` and
  `test_a_strong_mark_over_part_of_the_height_is_not_a_gutter` to that canvas.
  Verify both still assert null and both pass.
- [x] 2.3 Move the single pages of `test_split_answers_one_entry_per_image` and
  `test_split_keeps_the_order` to that canvas. Verify the HTTP answer still
  holds null for every one of them.

## 3. Tests for the rule itself

- [x] 3.1 Add a test that a landscape image with no gutter answers both numbers
  at the middle column. Verify the pair equals `(width // 2, width // 2)`.
- [x] 3.2 Add a test that the two pages meet: cutting at the answer gives two
  pages whose widths sum to the width of the image. Verify with a 1600 wide
  canvas.
- [x] 3.3 Add a test that a gutter away from the middle still wins. Verify the
  answer brackets the drawn gutter and does not name the middle.
- [x] 3.4 Add tests for the threshold: an image at ratio 1.2 with no gutter
  answers a pair, one at ratio 1.1 answers null, and a square one answers null.
  Verify all three.
- [x] 3.5 Run `docker compose -f docker-compose.dev.yml exec sidecar pytest`.
  Verify the whole file passes.

## 4. The whole pipeline

- [x] 4.1 Add a scenario to `e2e/tests/split.spec.ts` that splits the
  `landscape` fixture, which is 3000 by 2000 and holds no gutter, and expects
  twice the pages of the source book. Verify `./run-e2e.sh split.spec.ts`
  passes, and confirm first that the fixture really answers no gutter today.
- [x] 4.2 Check that the new pages record their cut: both halves of a fixture
  page name the same source page, one `left` and one `right`. Verify through
  `GET /api/books/{id}`.
- [x] 4.3 Run the whole suite. Verify `./run-e2e.sh`,
  `docker compose -f docker-compose.dev.yml exec backend go test ./...` and the
  sidecar `pytest` all pass.

## 5. Docs

- [x] 5.1 Add the fallback to the README, beside the paragraphs on how the
  sidecar decides a spread: the ratio, the middle cut, and that a found gutter
  wins. Verify the section names the 1.2 ratio.
