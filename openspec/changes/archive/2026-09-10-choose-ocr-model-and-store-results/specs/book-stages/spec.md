## MODIFIED Requirements

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
