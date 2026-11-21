## ADDED Requirements

### Requirement: The Dewarp action leads to the Dewarped stage

The system SHALL offer the Dewarp action for a complete book in the `uploaded`
stage that has no copy. The copy of a Dewarp action SHALL carry the stage
`dewarped`.

#### Scenario: A user dewarps an uploaded book

- **WHEN** a user runs the Dewarp action on a complete book in the `uploaded`
  stage
- **THEN** the system creates a copy in the `dewarped` stage
- **AND** the copy appears in the Dewarped tab while it runs

#### Scenario: The copy finishes

- **WHEN** the dewarp job stores every page of the copy
- **THEN** the copy state becomes `complete`
- **AND** its stage is still `dewarped`
- **AND** it is ready for the Split action

#### Scenario: The book is already dewarped

- **WHEN** a user runs the Dewarp action on a book in the `dewarped` stage
- **THEN** the system rejects the request and names the reason

## MODIFIED Requirements

### Requirement: Dewarping runs as a background job

The system SHALL dewarp a book in a background worker, not in the request that
starts it. The system SHALL move the copy through the states `pending`,
`processing`, and then `complete` or `failed`. The copy SHALL keep the stage
`dewarped` through the whole job. The system SHALL NOT change the state, the
stage, or the pages of the source book.

#### Scenario: The job starts

- **WHEN** the worker picks up a dewarp job for a copy in state `pending`
- **THEN** the copy state becomes `processing`
- **AND** the copy keeps the stage `dewarped`
- **AND** the system broadcasts a `book.updated` event

#### Scenario: The source book is untouched

- **WHEN** a dewarp job runs to its end, whatever the result
- **THEN** the source book keeps its state, its stage, and all of its pages

#### Scenario: One job at a time

- **WHEN** a user starts dewarp jobs for three books
- **THEN** the system runs them one after another
- **AND** the HTTP server and the other queues keep working meanwhile

### Requirement: A failed dewarp keeps the copy and allows a retry

The system SHALL keep the copy when a dewarp job fails. The copy SHALL hold the
failure reason, its stage, and the reference to the source book. The system
SHALL set the copy to `failed` only after the job exhausts its retries.

#### Scenario: The job fails

- **WHEN** a dewarp job fails on its last attempt
- **THEN** the copy state becomes `failed`
- **AND** the copy keeps the stage `dewarped`
- **AND** the Dewarped tab shows the copy and its reason at the head of the list
- **AND** the source book carries a warning mark and offers no action

#### Scenario: Attempts remain

- **WHEN** a dewarp attempt fails and further attempts remain
- **THEN** the copy state stays `processing`
- **AND** the system runs the job again

#### Scenario: A user retries a failed copy

- **WHEN** a user activates the retry control on a failed copy
- **THEN** the same copy returns to state `pending`
- **AND** the system queues its dewarp job again
- **AND** no third book exists

#### Scenario: A retry succeeds

- **WHEN** a retried job stores every page
- **THEN** the copy holds exactly one set of pages
- **AND** no page index appears twice

### Requirement: Dewarp is optional configuration

The system SHALL start and serve every other feature when the object storage or
the dewarp endpoint is not configured. The Dewarp action SHALL then report that
dewarp is not configured, and SHALL create no copy. Every other action SHALL
keep working.

#### Scenario: The server starts without the settings

- **WHEN** the object storage and endpoint settings are absent
- **THEN** the server starts
- **AND** upload, ingest, splitting, and browsing all work

#### Scenario: A user runs the action without the settings

- **WHEN** a user runs the Dewarp action and dewarp is not configured
- **THEN** the system rejects the request with a message that says so
- **AND** it creates no copy

#### Scenario: A bulk action without the settings

- **WHEN** a user runs the Dewarp action on four selected books and dewarp is
  not configured
- **THEN** the system creates no copy
- **AND** the interface shows the reason once

## REMOVED Requirements

### Requirement: A book at the dewarp stage offers a Dewarp action

**Reason**: Which stage offers which action is no longer specific to dewarp. A
stage offers one action for every stage after it, and the interface derives the
book view buttons and the context menu from that list.

**Migration**: Replaced by "A stage offers one action per later stage" in
`book-stages`, "The Dewarp action leads to the Dewarped stage" here, and "A
right click offers the actions of the current stage" and "The book view offers
the actions of the book's stage" in `book-library`.

### Requirement: The Dewarp action creates a copy at once

**Reason**: Every action creates its copy the same way. The rule is no longer
specific to dewarp, and a copy now carries its stage from creation instead of
gaining it at the end.

**Migration**: Replaced by "An action creates its copy at once" and "Every book
carries a stage" in `book-stages`.

### Requirement: A reference marks the source book processed

**Reason**: Every action leaves the same reference, and the interface now needs
the state of the copy, not only its existence.

**Migration**: Replaced by "A copy marks its source book" in `book-stages`.

### Requirement: A processed book shows a checkmark without a reload

**Reason**: A single checkmark cannot tell a running copy from a complete one.
The mark now follows the state of the copy, for every action.

**Migration**: Replaced by "A card is marked by the state of its copy" in
`book-library`.
