## MODIFIED Requirements

### Requirement: Ingest runs as a background job

The system SHALL process an uploaded book in a background worker, not in the
upload request. The system SHALL move the book through the states `pending`,
`processing`, and then `complete` or `failed`. The book SHALL keep the stage
`uploaded` through the whole job.

#### Scenario: A queued book starts

- **WHEN** the worker picks up an ingest job for a book in state `pending`
- **THEN** the book state becomes `processing`
- **AND** the book keeps the stage `uploaded`
- **AND** the system broadcasts a `book.updated` event

#### Scenario: Ingest finishes

- **WHEN** the worker stores every page of a book
- **THEN** the book state becomes `complete`
- **AND** the book keeps the stage `uploaded`
- **AND** the system broadcasts a `book.updated` event

## ADDED Requirements

### Requirement: An uploaded book starts in the first stage

The system SHALL give an uploaded book the first stage when it creates the book
row, before ingest runs. The first stage SHALL be `uploaded`.

#### Scenario: A user uploads a book

- **WHEN** the system accepts an upload
- **THEN** the new book carries the stage `uploaded` in state `pending`
- **AND** the Uploaded tab shows it at once

#### Scenario: A book is still processing

- **WHEN** a book is in state `pending` or `processing`
- **THEN** its stage is `uploaded`
- **AND** it appears in the Uploaded tab and in no other tab

#### Scenario: Ingest fails

- **WHEN** a book reaches state `failed`
- **THEN** its stage is still `uploaded`
- **AND** the Uploaded tab shows it at the head of its list

## REMOVED Requirements

### Requirement: A complete book enters the first stage

**Reason**: A book now carries its stage from creation, so the stage no longer
depends on the state. A running book must appear in the tab it belongs to, not
in a separate area.

**Migration**: Replaced by "An uploaded book starts in the first stage" here and
"Every book carries a stage" in `book-stages`. The first stage is renamed from
`dewarp` to `uploaded`.
