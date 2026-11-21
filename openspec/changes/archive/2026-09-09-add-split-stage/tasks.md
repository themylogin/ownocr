## 1. The stage list holds Split

- [x] 1.1 Add the stage `split`, titled "Split", between `dewarped` and `done` in
  `books/stages.go`, add the `StageSplit` constant, and point the Split action at
  it; verify `ActionsFor` offers Dewarp and Split for `uploaded`, Split for
  `dewarped`, and nothing for `split` or `done`
- [x] 1.2 Add a migration that moves every book with the stage `done` to `split`,
  and verify a book split before the change opens in the Split tab
- [x] 1.3 Run the backend unit tests, and verify the stage tests cover the four
  stages and the empty `done` stage

## 2. Tests follow the stage

- [x] 2.1 Point the split tests at `/stage/split` in `e2e/tests/split.spec.ts`,
  and assert the copy carries the stage `split`
- [x] 2.2 Point `/stage/done` at `/stage/split` in the dewarp, library and reject
  tests, where the tab is opened to find or to miss a split book; verify each
  assertion still tests a tab that can hold a book
- [x] 2.3 Leave the `data-group="done"` assertions alone: the book group marks a
  book whose copy is complete, in any stage, and is not the `done` stage
- [x] 2.4 Add an e2e test that the Done tab is empty and offers no selection
  control, so the placeholder stage stays visible and harmless
- [x] 2.5 Run the full e2e suite and verify it passes
