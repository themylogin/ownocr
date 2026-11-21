## MODIFIED Requirements

### Requirement: The server defines the stage order

The system SHALL define an ordered list of stages on the server. The list SHALL
be `uploaded`, `dewarped`, `split`, `done`, in that order, with the titles
"Uploaded", "Dewarped", "Split" and "Done". A client SHALL take the list from the
server, so a new stage needs no client change.

The system SHALL keep `done` in the list although no action leads to it yet. The
stage names the end of the pipeline. Its tab SHALL stay empty until an action
leads to it.

#### Scenario: A client asks for the stages

- **WHEN** a client requests the stage list
- **THEN** the system answers with `uploaded`, `dewarped`, `split` and `done` in
  that order
- **AND** each entry carries its title and the number of books in it

#### Scenario: The last stage holds no book

- **WHEN** a client reads the books of the `done` stage
- **THEN** the system answers with no book
- **AND** the tab reports that it is empty

#### Scenario: A stage name is unknown

- **WHEN** a client asks for books of a stage that is not in the list
- **THEN** the system rejects the request and names the stage

### Requirement: A stage offers one action per later stage

Every action SHALL name the stage a book enters when the action completes. A
stage SHALL offer every action whose stage comes after it in the order. The
system SHALL report the actions of each stage with the stage list.

The actions SHALL be `dewarp`, titled "Dewarp", which leads to `dewarped`, and
`split`, titled "Split", which leads to `split`.

A stage that no action comes after SHALL offer no action. Both `split` and `done`
offer none, because no action leads to `done`.

#### Scenario: The first stage

- **WHEN** a client reads the actions of the `uploaded` stage
- **THEN** the system reports "Dewarp" and "Split", in stage order

#### Scenario: The middle stage

- **WHEN** a client reads the actions of the `dewarped` stage
- **THEN** the system reports "Split" only

#### Scenario: The split stage

- **WHEN** a client reads the actions of the `split` stage
- **THEN** the system reports no action

#### Scenario: The last stage

- **WHEN** a client reads the actions of the `done` stage
- **THEN** the system reports no action

#### Scenario: An action does not fit the stage

- **WHEN** a user runs the Dewarp action on a book in the `dewarped` stage
- **THEN** the system rejects the request and names the reason
- **AND** it creates no copy
