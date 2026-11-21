## RENAMED Requirements

- FROM: `### Requirement: An action declares how it maps the pages of its source book`
- TO: `### Requirement: An action declares whether its pages offer a rollback`

## MODIFIED Requirements

### Requirement: The server defines the stage order

The system SHALL define an ordered list of stages on the server. The list SHALL
be `uploaded`, `dewarped`, `split`, `done`, in that order, with the titles
"Uploaded", "Dewarped", "Split" and "Done". A client SHALL take the list from the
server, so a new stage needs no client change.

The `done` stage names the end of the pipeline. The OCR action leads to it, so
its tab holds the books an OCR job made.

#### Scenario: A client asks for the stages

- **WHEN** a client requests the stage list
- **THEN** the system answers with `uploaded`, `dewarped`, `split` and `done` in
  that order
- **AND** each entry carries its title and the number of books in it

#### Scenario: The last stage holds no book

- **WHEN** a client reads the books of the `done` stage before any OCR action
  ran
- **THEN** the system answers with no book
- **AND** the tab reports that it is empty

#### Scenario: The last stage holds a book

- **WHEN** a client reads the books of the `done` stage after an OCR action ran
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
leads to `done`.

A stage that no action comes after SHALL offer no action. Only `done` offers
none.

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

- **WHEN** a client reads the actions of the `done` stage
- **THEN** the system reports no action

#### Scenario: An action does not fit the stage

- **WHEN** a user runs the Dewarp action on a book in the `dewarped` stage
- **THEN** the system rejects the request and names the reason
- **AND** it creates no copy

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
