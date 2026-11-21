## 1. Data model and stage registry

- [x] 1.1 Add `backend/internal/db/migrations/0004_stages.sql` with the statements
      from design.md. Verify a fresh `docker compose up postgres backend` starts
      and `\d book` shows `stage NOT NULL`, plus the `progress` and `action`
      columns.
- [x] 1.2 Rewrite `books.Stages` as `uploaded`, `dewarped`, `done` with their
      titles, and replace `StageDewarp` and `StageSplit` with `StageUploaded`,
      `StageDewarped` and `StageDone`. Verify `go build ./...` names every
      caller that must change.
- [x] 1.3 Add `books.Action` with `ID`, `Title` and `Stage`, the `Actions`
      registry for `dewarp` and `split`, plus `ActionByID` and `ActionsFor`.
      Verify a new `stages_test.go` covers: `uploaded` gives Dewarp then Split,
      `dewarped` gives Split, `done` gives none, and an unknown stage gives none.
- [x] 1.4 Add the six phase constants to `books`. Verify they read exactly as the
      six texts in `specs/book-progress/spec.md`.

## 2. Book store

- [x] 2.1 Replace the `EXISTS` sub-select in `bookColumns` with the lateral join
      on the copy state, and change `Book.Processed bool` to
      `Book.DerivedState *string`. Add `Progress *string` and `Action *string`.
      Verify `go test ./internal/books/...` passes after the fixtures are
      updated.
- [x] 2.2 Replace `ListByStage` and `ListActive` with one `ListByStage` that
      returns every book of a stage in the order from design.md. Verify a store
      test seeds one book per state group and asserts the returned order.
- [x] 2.3 Set the stage at creation: `CreateTx` writes `uploaded`, and
      `CreateDerivedTx` takes the action and writes its stage and its id. Verify
      a store test reads back the stage and the action of both.
- [x] 2.4 Stop clearing the stage: `SetProcessing`, `SetComplete` and the requeue
      path leave `stage` alone, and `SetComplete` loses its stage argument.
      Verify a store test moves a book through every state and asserts the stage
      never changes.
- [x] 2.5 Add `SetProgress(id, phase)` and clear `progress` in `SetComplete` and
      `SetFailed`. Verify a store test asserts a complete book and a failed book
      both report no phase.
- [x] 2.6 Rename the requeue helper to serve any action, and make it reset
      `progress` to "Waiting in local queue". Verify `go test ./internal/books/...`
      passes.
- [x] 2.7 Update `books.Event` to carry `stage`, `derivedState`, `progress` and
      `action` and to drop `processed`. Verify the event keys match
      `bookSummary` field for field.

## 3. HTTP API

- [x] 3.1 Add the actions of each stage to `GET /api/stages`, and count every
      book of a stage instead of only the complete ones. Verify
      `curl /api/stages` returns three stages with `[Dewarp, Split]`, `[Split]`
      and `[]`.
- [x] 3.2 Simplify `GET /api/books`: keep the `stage` filter, drop
      `state=active`, and reject an unknown stage. Verify
      `curl "/api/books?state=active"` answers 422 and
      `curl "/api/books?stage=uploaded"` answers the ordered list.
- [x] 3.3 Update `bookSummary` and `detailOf` to the new fields. Verify
      `curl /api/books/{id}` shows `derivedState`, `progress`, `action` and a
      non-null `stage`, and no `processed`.
- [x] 3.4 Add `POST /api/books/actions/{action}` with the `bookIds` body, the
      per-book transaction, and the `started` and `failed` answer. Verify it
      returns 404 for an unknown action, and a mixed answer when one of several
      books already has a copy.
- [x] 3.5 Move the dewarp preconditions into a shared action starter that checks
      the stage against `ActionsFor`, locks the source book, refuses a second
      copy, requeues a failed copy, and inserts the job of the chosen action.
      Verify `go test ./internal/httpapi/...` passes.
- [x] 3.6 Delete `POST /api/books/{id}/dewarp` and its handler. Verify the route
      answers 404.
- [x] 3.7 Publish `book.created` or `book.updated` for every copy that starts,
      and `book.updated` for every source book, in one bulk request. Verify a
      manual bulk call on three books produces six events.

## 4. Workers

- [x] 4.1 Report phases from the ingest worker: "Converting" at the start, and
      no phase at the end. Verify `go test ./internal/ingest/...` asserts the
      phase of a running book and of a complete one.
- [x] 4.2 Report phases from the dewarp worker: "Uploading", then the endpoint
      phases, then "Downloading", then "Converting". Verify
      `go test ./internal/dewarp/...` asserts the phase sequence of one job.
- [x] 4.3 Add a status callback to `Client.Wait` that maps `IN_QUEUE` to
      "Waiting for AI" and `IN_PROGRESS` to "Processing with AI". Verify
      `runpod_test.go` asserts both phases from a scripted status sequence.
- [x] 4.4 Split the dewarp download into a fetch pass and a convert pass, and
      remove the per-page delete between them. Verify the existing dewarp worker
      test still gets one set of pages, and the temp directory is empty when the
      job ends.
- [x] 4.5 Add `backend/internal/split` with `Args`, `QueueName`, `MaxAttempts`
      and a worker that copies every source page through
      `ingest.WriteRenditions`. Verify a new `worker_test.go` covers a twelve
      page book, a repeated job, and a failure that sets the copy failed only on
      the last attempt.
- [x] 4.6 Register the split queue with `MaxWorkers: 1` and the split worker in
      `queue.New` and `cmd/server/main.go`. Verify the server logs the queue at
      start and a queued split job runs.
- [x] 4.7 Make the upload handler and the action starter write "Waiting in local
      queue" with the new row. Verify a freshly uploaded book reports that phase
      before a worker picks it up.

## 5. Frontend model and shell

- [x] 5.1 Update `api/client.ts`: the new `Book` fields, `stage: string`,
      `listByStage` only, `runAction(action, bookIds)`, and `Stage.actions`.
      Verify `npm run typecheck` names every view that must change.
- [x] 5.2 Add `composables/useSelection.ts` with a module-level id set, and
      `actionsFor(stage)` in `useStages`. Verify a stage change clears the set.
- [x] 5.3 Add `components/Breadcrumbs.vue` and use it in `BookView` and
      `ReaderView` in place of the back links. Verify the trail reads
      "Dewarped → <name>" in the book view and adds "View" in the reader, and
      that each step navigates.
- [x] 5.4 Add "Select all", "Deselect all" and the action dropdown to the
      `App.vue` top bar, after the upload control, driven by the selection set
      and `actionsFor`. Verify the Done tab shows none of them, and the last two
      appear only with a selection.

## 6. Frontend list

- [x] 6.1 Add `components/BookMark.vue` with the static spinner, warning and
      check SVGs. Verify no icon animates.
- [x] 6.2 Add `components/BookCard.vue` that picks its shape from `state` and
      `derivedState`: icon box with phase or reason, cover with checkbox, cover
      with mark and fade. Verify each of the five shapes renders from a fixture
      book.
- [x] 6.3 Add `components/BookList.vue` that sorts by the comparator from
      design.md, renders the grid, and owns the context menu built from the
      stage actions. Verify a phase-only update leaves a card in place and a
      state change moves it.
- [x] 6.4 Rewrite `StageView.vue` on top of `BookList`: one list, no separate
      processing card, live events applied through the comparator, and the retry
      control on a failed card. Verify the file is well under its current 304
      lines and `npm run typecheck` passes.
- [x] 6.5 Wire the bulk action: run the chosen action on the selection, show the
      reason of every rejected book, then clear the selection. Verify a mixed
      answer shows the reason and still starts the other books.
- [x] 6.6 Update `BookView.vue`: one button per action of the book's stage, the
      copy mark instead when a copy exists, and a five column page grid that
      takes its shape from `gridCell`. Verify five cells fill each row at three
      window widths.
- [x] 6.7 Update `ReaderView.vue`: drop the `100vw` breakout and the page number,
      and fit each page to the content column without enlarging it. Verify a
      page narrower than the column keeps its own width.

## 7. Tests and documentation

- [x] 7.1 Update `e2e/tests/helpers.ts`: the `Book` type, `runAction`, the retry
      helper, and the stage names. Verify `npx playwright test --list` resolves.
- [x] 7.2 Update `library.spec.ts` for the new tab names, the single ordered
      list, the card shapes, and the empty state. Verify `./run-e2e.sh
      library.spec.ts` passes.
- [x] 7.3 Update `dewarp.spec.ts` for the new endpoint, the stage names, the
      marks by copy state, and the copy that appears in the Dewarped tab while
      it runs. Verify `./run-e2e.sh dewarp.spec.ts` passes.
- [x] 7.4 Add `e2e/tests/split.spec.ts`: split from `Uploaded`, split from
      `Dewarped`, the copy in the Done tab, and the pages of the copy. Verify
      `./run-e2e.sh split.spec.ts` passes.
- [x] 7.5 Add `e2e/tests/selection.spec.ts`: checkboxes on ready cards only,
      "Select all", "Deselect all", and a bulk action over three books. Verify
      `./run-e2e.sh selection.spec.ts` passes.
- [x] 7.6 Add a progress assertion to `ingest.spec.ts`: an uploaded book shows
      "Waiting in local queue", then "Converting". Verify `./run-e2e.sh
      ingest.spec.ts` passes.
- [x] 7.7 Update `grid.spec.ts` for five cells per row and `reader.spec.ts` for
      the column fit with no page number. Verify both specs pass.
- [x] 7.8 Run `go test ./...` in `backend` and `./run-e2e.sh` for the whole
      suite. Verify both are green.
- [x] 7.9 Update `README.md` for the new stages, the action endpoint, and the
      split queue. Verify the stage names in the document match `books.Stages`.
