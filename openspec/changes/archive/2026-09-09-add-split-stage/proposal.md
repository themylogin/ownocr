## Why

A split book lands in the `done` stage, so it appears in the Done tab. A user who
runs the Split action looks for the result in a Split tab and does not find one.

The `done` stage does two jobs at once. It names the output of the Split action,
and it names the end of the pipeline. The two are not the same. Reading text off
a page comes later, and that is what `done` should name.

## What Changes

- The stage list gains `split`, titled "Split", between `dewarped` and `done`.
- **BREAKING** The Split action leads to the `split` stage, not to `done`. A
  split copy appears in the Split tab.
- No action leads to `done` yet, so the Done tab stays empty until one does. The
  stage stays in the list, because it names the end of the pipeline.
- A migration moves every book that carries the stage `done` to `split`. Only the
  Split action ever made such a book.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `book-stages`: the stage list holds four stages, and the Split action leads to
  `split`.
- `book-split`: the copy of a Split action carries the stage `split`.

## Impact

- `backend/internal/books/stages.go`: the stage list and the target of the Split
  action.
- `backend/internal/db/migrations/`: one migration that moves the books.
- `e2e/tests/`: the tests that open `/stage/done` to find a split book.

No frontend change. The client reads the stage list from the server, so a new
stage needs no client work.

The `done` book group in `frontend/src/bookState.ts` is a different thing: it
marks a book whose copy is complete, in any stage. This change SHALL NOT touch
it.
