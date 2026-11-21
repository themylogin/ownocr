## MODIFIED Requirements

### Requirement: Stages appear as tabs

The system SHALL show the processing stages as an ordered row of tabs at the top
of the screen. Selecting a tab SHALL show the books pending that stage. The
system SHALL define the stage list on the server, so a new stage needs no
client change.

#### Scenario: The app opens

- **WHEN** a user opens the app
- **THEN** the tab row shows the tabs "Dewarp" and "Split" in that order
- **AND** the Dewarp tab is selected

#### Scenario: The selected tab survives a reload

- **WHEN** a user selects a stage tab and reloads the page
- **THEN** the same tab stays selected

#### Scenario: An unknown stage is requested

- **WHEN** a user opens a URL for a stage that does not exist
- **THEN** the system shows the first stage instead

### Requirement: A stage tab lists the books pending that stage

A stage tab SHALL list every complete book whose current stage is that stage.
Each entry SHALL show the book name and its cover. The cover SHALL be the
thumbnail of the book's first page. A processed book SHALL stay in its list. It
SHALL show a checkmark in the top right corner of its entry, and its entry SHALL
be faded.

#### Scenario: Books wait in the Dewarp stage

- **WHEN** three complete books have the stage `dewarp`
- **THEN** the Dewarp tab shows three entries
- **AND** each entry shows its name and its first page thumbnail

#### Scenario: No books wait in a stage

- **WHEN** no book has the selected stage
- **THEN** the system shows an empty state that invites an upload

#### Scenario: A book belongs to a different stage

- **WHEN** a book's stage is not the selected stage
- **THEN** that book does not appear in the list

#### Scenario: A processed book waits in a stage

- **WHEN** one of three books in the Dewarp tab is processed
- **THEN** the tab still shows three entries
- **AND** the processed entry carries a checkmark in its top right corner
- **AND** the processed entry is faded

#### Scenario: A dewarped book reaches the Split stage

- **WHEN** the copy of a dewarped book becomes complete
- **THEN** the Split tab lists it with its name and its first page thumbnail
