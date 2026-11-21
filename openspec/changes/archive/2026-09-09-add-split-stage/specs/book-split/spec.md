## RENAMED Requirements

- FROM: `### Requirement: The Split action leads to the Done stage`
- TO: `### Requirement: The Split action leads to the Split stage`

## MODIFIED Requirements

### Requirement: The Split action leads to the Split stage

The system SHALL offer the Split action for a complete book in the `uploaded` or
the `dewarped` stage that has no copy. The copy of a Split action SHALL carry
the stage `split`.

A book that already carries the stage `split` SHALL be offered no action, because
no action comes after that stage.

#### Scenario: A user splits an uploaded book

- **WHEN** a user runs the Split action on a complete book in the `uploaded`
  stage
- **THEN** the system creates a copy in the `split` stage
- **AND** the copy appears in the Split tab while it runs

#### Scenario: A user splits a dewarped book

- **WHEN** a user runs the Split action on a complete book in the `dewarped`
  stage
- **THEN** the system creates a copy in the `split` stage

#### Scenario: A book is already split

- **WHEN** a user runs the Split action on a book in the `split` stage
- **THEN** the system rejects the request and names the reason

#### Scenario: A book is already done

- **WHEN** a user runs the Split action on a book in the `done` stage
- **THEN** the system rejects the request and names the reason

### Requirement: Splitting needs no external configuration

The Split action SHALL work with no object storage and no remote endpoint. The
system SHALL offer it whenever the stage allows it.

#### Scenario: The dewarp settings are absent

- **WHEN** the object storage and endpoint settings are absent
- **THEN** the Split action still works
- **AND** a book still reaches the `split` stage
