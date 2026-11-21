## Purpose

Reports which phase a running job is in, so a user sees how far a book has come
without reading a log. Every worker reports its phase with the same small
vocabulary.

## ADDED Requirements

### Requirement: A running book carries a progress phase

The system SHALL store one progress phase with every book and SHALL report it
with the book. A book in state `pending` or `processing` SHALL carry a phase. A
book in state `complete` or `failed` SHALL carry no phase.

#### Scenario: A book waits for a worker

- **WHEN** a book enters state `pending`
- **THEN** its phase is "Waiting in local queue"

#### Scenario: A book completes

- **WHEN** a book reaches state `complete`
- **THEN** it carries no phase

#### Scenario: A book fails

- **WHEN** a book reaches state `failed`
- **THEN** it carries no phase
- **AND** it carries its failure reason instead

### Requirement: The phase vocabulary is fixed

The system SHALL report a phase as one of exactly these texts:

- "Waiting in local queue"
- "Converting"
- "Uploading"
- "Waiting for AI"
- "Processing with AI"
- "Downloading"

#### Scenario: A phase is shown

- **WHEN** the interface shows the phase of a running book
- **THEN** the text is one of the six phases, with no number and no suffix

### Requirement: Ingest reports its phase

The ingest worker SHALL report "Converting" while it turns the upload into
pages.

#### Scenario: A book is uploaded

- **WHEN** a user uploads a book
- **THEN** its phase is "Waiting in local queue" until a worker picks it up
- **AND** its phase is "Converting" while the worker builds the pages

### Requirement: Dewarp reports its phase

The dewarp worker SHALL report the phase of each part of its job, in this order:
"Uploading" while it sends pages to object storage, "Waiting for AI" while the
endpoint job waits in the endpoint queue, "Processing with AI" while the
endpoint job runs, "Downloading" while it fetches the results, and "Converting"
while it builds the pages of the copy.

#### Scenario: The worker sends the pages

- **WHEN** the worker uploads the source pages to object storage
- **THEN** the phase of the copy is "Uploading"

#### Scenario: The endpoint job waits

- **WHEN** the endpoint reports that the job is in its queue
- **THEN** the phase of the copy is "Waiting for AI"

#### Scenario: The endpoint job runs

- **WHEN** the endpoint reports that the job is running
- **THEN** the phase of the copy is "Processing with AI"

#### Scenario: The worker fetches the results

- **WHEN** the worker downloads the dewarped pages from object storage
- **THEN** the phase of the copy is "Downloading"

#### Scenario: The worker builds the pages

- **WHEN** the worker stores the pages of the copy after the download
- **THEN** the phase of the copy is "Converting"

### Requirement: Split reports its phase

The split worker SHALL report "Converting" while it builds the pages of the
copy.

#### Scenario: A book is split

- **WHEN** the split worker builds the pages of the copy
- **THEN** the phase of the copy is "Converting"

### Requirement: A phase change reaches every client at once

The system SHALL broadcast a `book.updated` event on every phase change. A
connected client SHALL show the new phase without a reload.

#### Scenario: A user watches a running book

- **WHEN** the phase of a running book changes
- **THEN** every connected client shows the new phase
- **AND** no client reloads the page

#### Scenario: A retried job starts over

- **WHEN** a failed copy is retried
- **THEN** its phase is "Waiting in local queue" again
