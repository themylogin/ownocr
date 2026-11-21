## Purpose

Defines the ordered stages a book passes through, the actions each stage offers,
and the copy an action creates. A stage names what a book already is. An action
names the work that moves a book to a later stage.

## Requirements

### Requirement: The server defines the stage order

The system SHALL define an ordered list of stages on the server. The list SHALL
be `uploaded`, `dewarped`, `split`, `ocr`, in that order, with the titles
"Uploaded", "Dewarped", "Split" and "OCR". A client SHALL take the list from the
server, so a new stage needs no client change.

The `ocr` stage names the end of the pipeline. The OCR action leads to it, so its
tab holds the books an OCR job made. Every stage is named after what a book in it
already is, and a book in the last stage has been read.

#### Scenario: A client asks for the stages

- **WHEN** a client requests the stage list
- **THEN** the system answers with `uploaded`, `dewarped`, `split` and `ocr` in
  that order
- **AND** each entry carries its title and the number of books in it

#### Scenario: The last stage holds no book

- **WHEN** a client reads the books of the `ocr` stage before any OCR action ran
- **THEN** the system answers with no book
- **AND** the tab reports that it is empty

#### Scenario: The last stage holds a book

- **WHEN** a client reads the books of the `ocr` stage after an OCR action ran
- **THEN** the system answers with the copy that action made

#### Scenario: A stage name is unknown

- **WHEN** a client asks for books of a stage that is not in the list
- **THEN** the system rejects the request and names the stage

### Requirement: A stage offers one action per later stage

Every action SHALL name the stage a book enters when the action completes. A
stage SHALL offer every action whose stage comes after it in the order. The
system SHALL report the actions of each stage with the stage list.

The actions SHALL be `dewarp`, titled "Dewarp", which leads to `dewarped`,
`split`, titled "Split", which leads to `split`, and `ocr`, titled "OCR", which
leads to `ocr`.

A stage that no action comes after SHALL offer no action. Only `ocr` offers none.

#### Scenario: The first stage

- **WHEN** a client reads the actions of the `uploaded` stage
- **THEN** the system reports "Dewarp", "Split" and "OCR", in stage order

#### Scenario: The middle stage

- **WHEN** a client reads the actions of the `dewarped` stage
- **THEN** the system reports "Split" and "OCR", in stage order

#### Scenario: The split stage

- **WHEN** a client reads the actions of the `split` stage
- **THEN** the system reports "OCR" only

#### Scenario: The last stage

- **WHEN** a client reads the actions of the `ocr` stage
- **THEN** the system reports no action

#### Scenario: An action does not fit the stage

- **WHEN** a user runs the Dewarp action on a book in the `dewarped` stage
- **THEN** the system rejects the request and names the reason
- **AND** it creates no copy

### Requirement: One endpoint runs an action on one or more books

The system SHALL accept one request that names an action and a list of books.
The request MAY carry arguments for that action. The system SHALL reject a
request that omits an argument the action requires, or that carries an argument
value the action does not accept. The system SHALL apply the same arguments to
every book of the request.

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

#### Scenario: An action needs an argument the request omits

- **WHEN** a request names an action that requires an argument, and carries none
- **THEN** the system rejects the whole request and names the missing argument
- **AND** no copy exists for any book of the request

#### Scenario: An argument value is not accepted

- **WHEN** a request carries an argument value the action does not accept
- **THEN** the system rejects the whole request and names the reason

#### Scenario: Many books take the same arguments

- **WHEN** a user runs an action with an argument for five books
- **THEN** all five copies record that argument

### Requirement: An action creates its copy at once

An action SHALL create a second book immediately, before any work runs. The copy
SHALL carry the name of the source book, a reference to it, the name of the
action that made it, and the arguments that action was given. The copy SHALL
start in state `pending` and SHALL carry the stage of its action from the moment
it is created. The system SHALL create the copy and queue its job together, so
neither can exist without the other.

#### Scenario: A user runs the Dewarp action

- **WHEN** a user runs the Dewarp action on a book named "Atlas"
- **THEN** a second book named "Atlas" exists in state `pending`
- **AND** its stage is `dewarped`
- **AND** it references the source book and records the action `dewarp`
- **AND** the system broadcasts a `book.created` event for it

#### Scenario: An action with an argument

- **WHEN** a user runs an action that takes an argument
- **THEN** the copy records the action and that argument
- **AND** the argument is readable on the copy for as long as the copy exists

#### Scenario: An action without arguments

- **WHEN** a user runs an action that takes no argument
- **THEN** the copy records no argument

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
stage, its action, its arguments and its reference. The system SHALL offer a
retry for a failed copy. A retry SHALL run the same action on the same source
book, with the arguments the copy records, SHALL reuse the same copy, and SHALL
NOT create a third book.

The retry SHALL take the arguments from the copy, never from the request that
starts it. A copy therefore runs the same work on every attempt.

#### Scenario: A user retries a failed copy

- **WHEN** a user activates the retry control on a failed copy
- **THEN** the same copy returns to state `pending`
- **AND** the system queues the job of its action again
- **AND** no third book exists

#### Scenario: A retry keeps the arguments

- **WHEN** a user retries a failed copy of an action that took an argument
- **THEN** the job runs with the argument the copy records
- **AND** the copy still records that argument

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

### Requirement: An action declares whether its pages offer a rollback

Every action SHALL declare whether a single page of its copy can return to the
page it was made from. The system SHALL offer a page rollback only for an action
that declares it, and SHALL refuse a rollback of any other page.

The Dewarp action SHALL declare that its pages offer a rollback: it makes one
page from one page, and the page it made differs from its source. The Split
action SHALL declare that they do not: splitting a spread makes two pages from
one. The OCR action SHALL declare that they do not: it reads a page without
changing it, so the page a rollback would install is the page the copy already
shows. The declaration describes the contract of the action, not the state of
its worker.

The system SHALL report with a book whether its pages offer a rollback, so a
client needs no knowledge of the action list.

#### Scenario: A client reads a dewarped book

- **WHEN** a client reads a complete dewarped copy that has no copy of its own
- **THEN** the book reports that its pages offer a rollback

#### Scenario: A client reads a split book

- **WHEN** a client reads a book that the Split action made
- **THEN** the book reports that its pages offer no rollback

#### Scenario: A client reads an OCR book

- **WHEN** a client reads a complete copy that the OCR action made
- **THEN** the book reports that its pages offer no rollback

#### Scenario: A user asks to roll back a page of an OCR book

- **WHEN** a user asks for the rollback of a page of an OCR copy
- **THEN** the system refuses the request and names the reason
- **AND** the page keeps its image and its markdown

#### Scenario: A client reads an uploaded book

- **WHEN** a client reads an uploaded book
- **THEN** the book reports that its pages offer no rollback

### Requirement: A copy records the source page of every page it writes

Every action worker SHALL record, for each page it writes, the page of the
source book that page was made from. A worker of an action that makes two copy
pages from one source page SHALL record the same source page for both.

A retry rebuilds every page of a copy, so it SHALL record the references again.

#### Scenario: A dewarp completes

- **WHEN** the Dewarp worker writes the pages of a copy
- **THEN** each page it writes names the source page it was made from

#### Scenario: A split completes

- **WHEN** the Split worker writes the pages of a copy
- **THEN** each page it writes names the source page it was made from

#### Scenario: A retry rebuilds a copy

- **WHEN** a retried job replaces every page of a copy
- **THEN** every page of the copy names its source page
- **AND** no page names a source page that no longer exists
