## MODIFIED Requirements

### Requirement: The Split action leads to the Split stage

The system SHALL offer the Split action for a complete book in the `uploaded` or
the `dewarped` stage that has no copy. The copy of a Split action SHALL carry
the stage `split`.

A book that already carries the stage `split` SHALL NOT be offered the Split
action again. It SHALL still be offered the OCR action, which comes after that
stage.

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

- **WHEN** a user runs the Split action on a book in the `ocr` stage
- **THEN** the system rejects the request and names the reason
