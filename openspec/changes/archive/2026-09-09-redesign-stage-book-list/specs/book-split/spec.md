## Purpose

Splits the pages of a book into single pages. The result is a copy in the `done`
stage. The split itself does no work yet: it copies the pages unchanged, so a
book that needs no splitting can reach `done` today.

## ADDED Requirements

### Requirement: The Split action leads to the Done stage

The system SHALL offer the Split action for a complete book in the `uploaded` or
the `dewarped` stage that has no copy. The copy of a Split action SHALL carry
the stage `done`.

#### Scenario: A user splits an uploaded book

- **WHEN** a user runs the Split action on a complete book in the `uploaded`
  stage
- **THEN** the system creates a copy in the `done` stage
- **AND** the copy appears in the Done tab while it runs

#### Scenario: A user splits a dewarped book

- **WHEN** a user runs the Split action on a complete book in the `dewarped`
  stage
- **THEN** the system creates a copy in the `done` stage

#### Scenario: A book is already done

- **WHEN** a user runs the Split action on a book in the `done` stage
- **THEN** the system rejects the request and names the reason

### Requirement: Splitting runs as a background job

The system SHALL split a book in a background worker, not in the request that
starts it. The system SHALL move the copy through the states `pending`,
`processing`, and then `complete` or `failed`. The system SHALL run one split
job at a time, so a large book does not starve the other queues.

#### Scenario: The job starts

- **WHEN** the worker picks up a split job for a copy in state `pending`
- **THEN** the copy state becomes `processing`
- **AND** the system broadcasts a `book.updated` event

#### Scenario: The job finishes

- **WHEN** the worker stores every page of the copy
- **THEN** the copy state becomes `complete`
- **AND** the system broadcasts a `book.updated` event

#### Scenario: Several books are split at once

- **WHEN** a user starts split jobs for three books
- **THEN** the system runs them one after another
- **AND** the HTTP server and the other queues keep working meanwhile

### Requirement: Splitting copies the pages of the source book

The split worker SHALL give the copy one page for every page of the source book,
in the same order. Every page of the copy SHALL have its own stored files, so
removing the source book SHALL NOT damage the copy. The system SHALL record the
dimensions of every page of the copy.

#### Scenario: A book is split

- **WHEN** the worker splits a source book of 120 pages
- **THEN** the copy holds 120 pages
- **AND** page `n` of the copy shows the same image as page `n` of the source

#### Scenario: The pages are independent

- **WHEN** the pages of a copy are stored
- **THEN** no page file of the copy is a page file of the source book

#### Scenario: The source book is untouched

- **WHEN** a split job runs to its end, whatever the result
- **THEN** the source book keeps its state, its stage, and all of its pages

### Requirement: Splitting is repeatable for one book

A retried split job SHALL leave the copy with exactly one set of pages. The
system SHALL discard whatever an interrupted attempt left behind before it
builds the pages again.

#### Scenario: A job is retried

- **WHEN** a split job runs again after an interrupted attempt
- **THEN** the copy holds exactly one set of pages
- **AND** no page index appears twice

### Requirement: Splitting reports failure

When a split job cannot finish, the system SHALL set the copy state to `failed`
and SHALL store a readable reason. The system SHALL set `failed` only after the
job exhausts its retries.

#### Scenario: The job fails

- **WHEN** a split job fails on its last attempt
- **THEN** the copy state becomes `failed`
- **AND** the interface shows the copy and its reason

#### Scenario: Attempts remain

- **WHEN** a split attempt fails and further attempts remain
- **THEN** the copy state stays `processing`
- **AND** the system runs the job again

### Requirement: Splitting needs no external configuration

The Split action SHALL work with no object storage and no remote endpoint. The
system SHALL offer it whenever the stage allows it.

#### Scenario: The dewarp settings are absent

- **WHEN** the object storage and endpoint settings are absent
- **THEN** the Split action still works
- **AND** a book still reaches the `done` stage
