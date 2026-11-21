## ADDED Requirements

### Requirement: An action declares how it maps the pages of its source book

Every action SHALL declare whether it makes exactly one copy page from one
source page. The system SHALL offer a page rollback only for an action that
does, because a rollback puts one source page back in the place of one copy
page.

The Dewarp action SHALL declare that it maps one page to one page. The Split
action SHALL declare that it does not: splitting a spread makes two pages from
one. The declaration describes the contract of the action, not the state of its
worker.

The system SHALL report with a book whether its pages offer a rollback, so a
client needs no knowledge of the action list.

#### Scenario: A client reads a dewarped book

- **WHEN** a client reads a complete dewarped copy that has no copy of its own
- **THEN** the book reports that its pages offer a rollback

#### Scenario: A client reads a split book

- **WHEN** a client reads a book that the Split action made
- **THEN** the book reports that its pages offer no rollback

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
