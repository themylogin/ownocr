## Why

The sidecar reports no cut when it cannot find a gutter, and the page then
reaches the copy whole. A photographed spread whose gutter the detector misses
therefore stays a two page image, which no later stage can read as two pages.

A spread is wider than it is tall, and a single page is not. That shape is
evidence the detector already has and does not use. When it finds no gutter in a
landscape image, the middle is a better answer than no answer.

## What Changes

- Cut a landscape image in half when the detector finds no gutter in it. The
  sidecar answers a pair, as it does for a gutter it found.
- Call an image landscape when it is at least 1.2 times wider than it is tall.
  A square or near square image stays whole, so a slightly wide single page is
  not cut by accident.
- Put the cut at the middle column exactly. The two pages meet there: no column
  is in both pages and no column is lost. There is no gutter to lean, so nothing
  needs the overlap a found gutter takes.
- Keep every image that is not landscape as it is today. A portrait single page,
  a portrait cover and a square image all still report no cut.
- **BREAKING** A landscape image with no gutter now makes two pages instead of
  one. A landscape cover and a landscape single page are cut as well.

## Capabilities

### New Capabilities

<!-- None. The rule belongs to the sidecar, which `page-split-detection` holds. -->

### Modified Capabilities

- `page-split-detection`: an image with no gutter reports no cut only when it is
  not landscape. A landscape image with no gutter reports the middle column.

## Impact

- Sidecar: `ownocr/split/gutter.py` gains the ratio constant and the fallback in
  `columns`. No new dependency, no change to the HTTP contract.
- Backend: none. The worker cuts what the answer names, and a pair of equal
  columns is a pair it already handles.
- Tests: four sidecar tests build a 1200 by 900 canvas, which the new rule reads
  as landscape, and expect no cut from it. They move to a portrait canvas. New
  tests cover the fallback, the threshold and a found gutter.
- E2E: a new scenario splits the `landscape` fixture, which holds no gutter, and
  expects twice the pages. No existing e2e case splits a landscape book, so
  none of them changes.
- Docs: the README gains the fallback in the Split flow.
