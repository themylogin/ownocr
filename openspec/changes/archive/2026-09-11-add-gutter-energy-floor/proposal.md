## Why

The gutter detector accepts a peak of any strength. On a flat scan of a spread
whose facing page is blank, the search band holds no gutter at all. The detector
then locks onto the residue of the text block and reports a gutter inside the
paragraph. The cut slices every line of that page in two.

A sweep of 181 pages from four books found three such pages: pages 5 and 41 of
book 6, and page 24 of book 5. Each lost the inner fifth of its text.

## What Changes

- The detector measures the strength of its peak in grey levels per row. It
  rejects a peak below a floor, and reports no gutter.
- A rejected page falls to the middle column rule, which already holds. Every
  one of the three pages then cuts in the right place.
- The floor is absolute. It does not compare the peak against the strongest
  column of the image, because that column is often a dark scanner border.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `page-split-detection`: *The sidecar tells a spread from a single page* now
  forbids a gutter below a strength floor. The current text forbids strength to
  decide at all.

## Impact

- `sidecar/ownocr/split/gutter.py`: one constant and one test in `_gutter`.
- `sidecar/tests/test_split.py`: a case for a spread with a blank facing page.
- No change to the sidecar API, to the backend, or to the frontend. The answer
  keeps its shape, and a caller sees no new field.
- No migration and no new job. Books 20 and 10 hold six pages cut out of the
  three broken spreads. The page cut action already re-cuts a page from the
  bytes of its source, so it corrects them one at a time.
