## MODIFIED Requirements

### Requirement: The system refuses a page operation while the chain holds work in flight

The system SHALL refuse to delete a page, SHALL refuse to roll a page back,
SHALL refuse to cut a page again, and SHALL refuse to rescan a page, while the
book or any book derived from it is in state `pending` or `processing`. It SHALL
name the reason.

A running job reads the pages of a source book and writes the pages of a copy.
Changing either set under the job would make the copy disagree with its source.

A rescan adds stored results to pages of the book it runs on. It SHALL also
refuse while its own book is in any state other than `complete`, so two rescans
of one book never run at once and a rescan never runs against a book a job is
still building.

A failed book SHALL NOT block the delete, the rollback or the re-cut. A retry
rebuilds the copy from the source book as it stands. A failed book SHALL block
the rescan of its own pages, because a rescan adds to a reading that is not
finished.

#### Scenario: A copy is running

- **WHEN** a user deletes a page of a book whose dewarp copy is in state
  `processing`
- **THEN** the system rejects the request and names the reason
- **AND** it removes nothing

#### Scenario: The book itself is running

- **WHEN** a user deletes a page of a book that is in state `processing`
- **THEN** the system rejects the request and names the reason

#### Scenario: A copy failed

- **WHEN** a user deletes a page of a book whose dewarp copy is in state
  `failed`
- **THEN** the system removes the page from both books

#### Scenario: The chain is complete

- **WHEN** every book of the chain is in state `complete`
- **THEN** the system carries out the request

#### Scenario: The book of the page is running

- **WHEN** a user re-cuts a page of a split copy that is in state `processing`
- **THEN** the system rejects the request and names the reason
- **AND** the page is unchanged

#### Scenario: A rescan already runs

- **WHEN** a user asks to rescan a page of a book whose rescan is in state
  `processing`
- **THEN** the system rejects the request and names the reason
- **AND** the running rescan is unaffected

#### Scenario: A page operation during a rescan

- **WHEN** a user deletes a page of a book whose rescan is in state `processing`
- **THEN** the system rejects the request and names the reason
- **AND** it removes nothing

#### Scenario: The reading of the book is not finished

- **WHEN** a user asks to rescan a page of a book in state `failed`
- **THEN** the system rejects the request and names the reason
