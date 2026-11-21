## Why

The stage tabs name operations, not books. A tab called "Dewarp" holds books
that are not dewarped yet. That reading is backwards, and it breaks as soon as a
book may skip a step.

The book list is also two lists. Books that still run sit in a card above the
grid. Books that wait sit in the grid. A user watches two places at once, and
neither tells how far a job has come.

A user who uploads thirty books must right click thirty cards to move them on.

## What Changes

- **BREAKING** Rename the stages after what a book is, not after what happens
  next. The tabs become `Uploaded`, `Dewarped`, `Done`. A migration renames the
  stored values.
- Name the work a book can take an *action*. An action produces a copy in a
  later stage. A stage offers one action for every tab to its right. `Uploaded`
  offers Dewarp and Split. `Dewarped` offers Split. `Done` offers none.
- Add the Split action. Splitting is a noop for now: it copies the pages and the
  copy lands in `Done`. A user who needs no dewarping sends a book straight
  from `Uploaded` to `Done`.
- **BREAKING** Replace `POST /api/books/{id}/dewarp` with one endpoint for every
  action and any number of books: `POST /api/books/actions/{action}`.
- **BREAKING** Give a book its stage when it is created, not when it completes.
  A book that still runs appears in the tab it is going to, and nowhere else.
- Merge the two lists into one reusable book list. It orders books by state,
  then by upload time, newest first. The states are: failed, processing, ready
  for the next stage, moving to the next stage, and done with its action.
- Show a job's phase on a processing card. A worker reports one of "Waiting in
  local queue", "Converting", "Uploading", "Waiting for AI", "Processing with
  AI", or "Downloading".
- Mark a card by the state of its copy. A running copy gives a turning spinner
  and a light fade. A failed copy gives a warning mark. A complete copy gives
  the green checkmark and a deeper fade.
- Add selection. A book that is ready for an action carries a checkbox in its
  top right corner. The top bar gains "Select all". A selection adds "Deselect
  all" and an action menu that holds the same actions as the context menu.
- Show the pages of a book five to a row. The cells keep the shape the server
  computes from the pages.
- Replace every "Back" link with a breadcrumb widget:
  `Uploaded → <book name> → View`.
- Fit a reader page to the content column instead of the whole window. Drop the
  page number under each page.
- Add a Reject action to the context menu, below a divider and in the danger
  colour. It asks for confirmation, then destroys the book, every copy made from
  it, and their files. The source book it frees returns to the ready group and
  accepts an action again. Add `GET /api/books/{id}/rejection`,
  `POST /api/books/{id}/reject`, and a `book.deleted` event.
- Offer the actions and Reject in the book view and in the reader, where the
  sticky bar keeps them on screen at every page.

## Capabilities

### New Capabilities

- `book-stages`: The stage order, the actions a stage offers, the action
  endpoint, and the copy that an action creates and that marks its source book.
  Covers one action per book, the retry of a failed action, and the reject that
  destroys a book with every copy made from it.
- `book-split`: The Split action. Covers its noop worker, which copies the pages
  of the source book into a copy in the `Done` stage.
- `book-progress`: The phase a running job reports, its vocabulary, and how the
  phase reaches every connected client.

### Modified Capabilities

- `book-library`: The tabs are renamed and reordered. One list replaces the
  separate area for running books, with a fixed state order. Cards gain
  checkboxes, marks by copy state, and progress text. The top bar gains
  selection controls and a bulk action menu. The page grid holds five cells per
  row. Breadcrumbs replace back links. A reader page fits the content column and
  carries no page number. The context menu gains Reject. The book view and the
  reader offer the actions and Reject, and fade nothing.
- `book-dewarp`: The Dewarp action starts at the `uploaded` stage and its copy
  lands in `dewarped`. The generic action and copy requirements move to
  `book-stages`. The worker reports its phase.
- `book-ingest`: The first stage is `uploaded`. Ingest no longer assigns the
  stage, because a book carries its stage from creation.

## Impact

- Database: rename the stored stage values, make `stage` not null, add
  `progress` and `action` columns to `book`.
- API: `GET /api/stages` returns the actions of each stage. `GET /api/books`
  drops `state=active` and returns every book of a stage. A book summary
  replaces `processed` with `derivedState`, and gains `progress` and `action`.
  `POST /api/books/{id}/dewarp` is removed.
- Backend: new `books` action registry, new split worker and queue, phase
  reporting in the ingest and dewarp workers, two-pass download in the dewarp
  worker.
- Frontend: new `BookList`, `BookCard`, `BookMark`, `BookActions`,
  `Breadcrumbs` and `RejectDialog` components, a selection composable and an
  action composable, rewritten `StageView`, `BookView`, `ReaderView` and `App`
  top bar.
- Tests: the e2e helpers and every stage, dewarp, grid and reader test change
  with the renamed stages and the new endpoint.
