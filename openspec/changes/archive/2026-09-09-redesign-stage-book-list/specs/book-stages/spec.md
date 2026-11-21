## Purpose

Defines the ordered stages a book passes through, the actions each stage offers,
and the copy an action creates. A stage names what a book already is. An action
names the work that moves a book to a later stage.

## ADDED Requirements

### Requirement: The server defines the stage order

The system SHALL define an ordered list of stages on the server. The list SHALL
be `uploaded`, `dewarped`, `done`, in that order, with the titles "Uploaded",
"Dewarped" and "Done". A client SHALL take the list from the server, so a new
stage needs no client change.

#### Scenario: A client asks for the stages

- **WHEN** a client requests the stage list
- **THEN** the system answers with `uploaded`, `dewarped` and `done` in that
  order
- **AND** each entry carries its title and the number of books in it

#### Scenario: A stage name is unknown

- **WHEN** a client asks for books of a stage that is not in the list
- **THEN** the system rejects the request and names the stage

### Requirement: A stage offers one action per later stage

Every action SHALL name the stage a book enters when the action completes. A
stage SHALL offer every action whose stage comes after it in the order. The
system SHALL report the actions of each stage with the stage list.

The actions SHALL be `dewarp`, titled "Dewarp", which leads to `dewarped`, and
`split`, titled "Split", which leads to `done`.

#### Scenario: The first stage

- **WHEN** a client reads the actions of the `uploaded` stage
- **THEN** the system reports "Dewarp" and "Split", in stage order

#### Scenario: The middle stage

- **WHEN** a client reads the actions of the `dewarped` stage
- **THEN** the system reports "Split" only

#### Scenario: The last stage

- **WHEN** a client reads the actions of the `done` stage
- **THEN** the system reports no action

#### Scenario: An action does not fit the stage

- **WHEN** a user runs the Dewarp action on a book in the `dewarped` stage
- **THEN** the system rejects the request and names the reason
- **AND** it creates no copy

### Requirement: One endpoint runs an action on one or more books

The system SHALL accept one request that names an action and a list of books.
The system SHALL handle each book on its own, so one rejected book SHALL NOT
stop the others. The answer SHALL hold the copy of every book that started, and
a reason for every book that did not.

#### Scenario: A user runs an action on one book

- **WHEN** a user runs an action for a single book that accepts it
- **THEN** the answer holds one copy
- **AND** the answer names no failure

#### Scenario: A user runs an action on many books

- **WHEN** a user runs an action for twelve books that accept it
- **THEN** the system creates twelve copies
- **AND** the answer holds all twelve

#### Scenario: One book of many is rejected

- **WHEN** a user runs an action for five books, and one of them already has a
  copy
- **THEN** the system creates four copies
- **AND** the answer names the fifth book and the reason

#### Scenario: The action is unknown

- **WHEN** a request names an action that does not exist
- **THEN** the system rejects the whole request

#### Scenario: A book does not exist

- **WHEN** a request names a book id that does not exist
- **THEN** the answer names that id and the reason
- **AND** the other books still start

### Requirement: An action creates its copy at once

An action SHALL create a second book immediately, before any work runs. The copy
SHALL carry the name of the source book, a reference to it, and the name of the
action that made it. The copy SHALL start in state `pending` and SHALL carry the
stage of its action from the moment it is created. The system SHALL create the
copy and queue its job together, so neither can exist without the other.

#### Scenario: A user runs the Dewarp action

- **WHEN** a user runs the Dewarp action on a book named "Atlas"
- **THEN** a second book named "Atlas" exists in state `pending`
- **AND** its stage is `dewarped`
- **AND** it references the source book and records the action `dewarp`
- **AND** the system broadcasts a `book.created` event for it

#### Scenario: The copy is visible while it runs

- **WHEN** the copy is in state `pending` or `processing`
- **THEN** it appears in the tab of its own stage
- **AND** it appears in no other tab

#### Scenario: The job cannot be queued

- **WHEN** the system cannot queue the job for a copy
- **THEN** no copy exists for that book

#### Scenario: The source book is not complete

- **WHEN** a user runs an action on a book that is not in state `complete`
- **THEN** the system rejects the request and names the reason

### Requirement: A copy marks its source book

A book SHALL be marked by the copy that references it. The system SHALL derive
the mark from that reference alone, and SHALL NOT keep a separate flag. The
system SHALL report the state of the copy with the source book, so a client can
tell a running copy from a complete one.

#### Scenario: No copy exists

- **WHEN** no book references a book
- **THEN** the source book reports no copy state
- **AND** it offers every action of its stage

#### Scenario: The copy runs

- **WHEN** the copy is in state `pending` or `processing`
- **THEN** the source book reports that copy state

#### Scenario: The copy fails

- **WHEN** the copy reaches state `failed`
- **THEN** the source book reports the copy state `failed`

#### Scenario: The copy completes

- **WHEN** the copy reaches state `complete`
- **THEN** the source book reports the copy state `complete`

#### Scenario: The copy is deleted

- **WHEN** the copy is removed from the system
- **THEN** the source book reports no copy state again

### Requirement: A book takes exactly one action

A book SHALL have at most one copy. The system SHALL reject an action for a book
that already has a copy which has not failed, whichever action made that copy.

#### Scenario: The same action runs twice

- **WHEN** a user runs the Dewarp action on a book that already has a copy that
  has not failed
- **THEN** the system rejects the request
- **AND** it creates no second copy

#### Scenario: A second action follows the first

- **WHEN** a user runs the Split action on a book that already has a dewarp copy
  that has not failed
- **THEN** the system rejects the request
- **AND** it creates no second copy

#### Scenario: Two clients run the action together

- **WHEN** two clients run an action on the same book at the same time
- **THEN** exactly one copy exists
- **AND** the other client learns the reason

### Requirement: A failed copy allows a retry

The system SHALL keep a copy whose job failed, with its failure reason, its
stage, its action and its reference. The system SHALL offer a retry for a failed
copy. A retry SHALL run the same action on the same source book, SHALL reuse the
same copy, and SHALL NOT create a third book.

#### Scenario: A user retries a failed copy

- **WHEN** a user activates the retry control on a failed copy
- **THEN** the same copy returns to state `pending`
- **AND** the system queues the job of its action again
- **AND** no third book exists

#### Scenario: A retry keeps the stage

- **WHEN** a failed copy returns to state `pending`
- **THEN** it keeps the stage of its action
- **AND** it stays in the tab of that stage

#### Scenario: A retry succeeds

- **WHEN** a retried job stores every page
- **THEN** the copy holds exactly one set of pages
- **AND** no page index appears twice

### Requirement: Rejecting a book destroys it and every copy made from it

The system SHALL offer a reject for every book, whatever its stage and whatever
its state. A reject SHALL destroy the book, every copy made from it, and every
copy made from those, so no copy is ever left without its source. It SHALL
destroy their stored pages and their stored uploads with them.

The system SHALL answer, before it destroys anything, with the list of books a
reject would destroy.

The source book of a rejected chain SHALL survive. It SHALL then report no copy
state, which returns it to the books that are ready for an action, and it SHALL
accept an action again.

The system SHALL announce every destroyed book to connected clients.

#### Scenario: A copy is rejected

- **WHEN** a user rejects the copy of a dewarped book
- **THEN** the copy no longer exists
- **AND** its stored pages are gone
- **AND** the source book still exists with all of its pages

#### Scenario: The source book is freed

- **WHEN** the copy of a book is rejected
- **THEN** the source book reports no copy state
- **AND** it offers every action of its stage again
- **AND** running that action creates a new copy

#### Scenario: A chain is rejected

- **WHEN** a user rejects a book that a dewarped copy and a split copy came from
- **THEN** all three books no longer exist

#### Scenario: The system reports what a reject would destroy

- **WHEN** a client asks what rejecting a book would destroy
- **THEN** the system answers with that book and every copy made from it

#### Scenario: A failed book is rejected

- **WHEN** a user rejects a book whose ingest failed
- **THEN** the book no longer exists

#### Scenario: A running book is rejected

- **WHEN** a user rejects a book while its job still runs
- **THEN** the book no longer exists
- **AND** its job stops without marking anything failed

#### Scenario: The book does not exist

- **WHEN** a user rejects a book id that does not exist
- **THEN** the system answers `404 Not Found`

#### Scenario: Clients learn about it

- **WHEN** a reject destroys a book
- **THEN** the system broadcasts an event naming that book
- **AND** it broadcasts the freed source book

### Requirement: Every book carries a stage

The system SHALL give a book a stage when the book is created, and SHALL keep
that stage for the whole life of the book. A book SHALL never be without a
stage. The state of a book SHALL NOT change its stage.

#### Scenario: An uploaded book

- **WHEN** a user uploads a book
- **THEN** the book carries the stage `uploaded` at once, in state `pending`

#### Scenario: A copy

- **WHEN** an action creates a copy
- **THEN** the copy carries the stage of that action at once

#### Scenario: A job runs

- **WHEN** a job moves a book from `pending` to `processing`, and then to
  `complete` or `failed`
- **THEN** the stage of the book is the same throughout
