## ADDED Requirements

### Requirement: The OCR action asks for a model before it runs

The interface SHALL open a dialog when a user chooses the OCR action, from every
place that offers it: the bulk action menu, the context menu of a card, and the
buttons of the book view and the reader. The OCR action SHALL NOT start before
the user answers the dialog.

The dialog SHALL offer two choices, in this order:

- "Normal", subtitled "PP-OCRv6", described as the frontier OCR model.
- "Extra", subtitled "PaddleOCR-VL", described as the visual model, for
  difficult cases like polytonic Greek.

The dialog SHALL preselect "Normal". It SHALL name how many books it will run on
when a user started it from a bulk action. It SHALL hold a control that starts
the action and a control that cancels it. Cancelling, pressing Escape, or
clicking away SHALL close the dialog and start nothing.

Starting SHALL run the OCR action on every book the dialog names, with the
chosen profile. Every other action SHALL start as it does now, with no dialog.

#### Scenario: A user chooses OCR from the action menu

- **WHEN** a user selects three books and chooses "OCR"
- **THEN** the dialog opens and names three books
- **AND** no OCR job has started

#### Scenario: A user chooses OCR from a context menu

- **WHEN** a user right clicks a ready card and chooses "OCR"
- **THEN** the dialog opens and names one book
- **AND** no OCR job has started

#### Scenario: A user chooses OCR in the book view

- **WHEN** a user activates the "OCR" button of a book view or a reader
- **THEN** the dialog opens
- **AND** no OCR job has started

#### Scenario: A user starts the normal profile

- **WHEN** a user opens the dialog and starts it without changing the choice
- **THEN** the system runs the OCR action with the profile `v6`
- **AND** the dialog closes

#### Scenario: A user starts the extra profile

- **WHEN** a user picks "Extra" and starts the dialog
- **THEN** the system runs the OCR action with the profile `vl`

#### Scenario: A user cancels the dialog

- **WHEN** a user cancels the dialog, presses Escape, or clicks away
- **THEN** the dialog closes
- **AND** no OCR job runs
- **AND** the selection does not change

#### Scenario: Another action needs no dialog

- **WHEN** a user chooses "Dewarp" or "Split", from any place that offers it
- **THEN** the action starts at once
- **AND** no dialog opens

## MODIFIED Requirements

### Requirement: Stages appear as tabs

The system SHALL show the stages as an ordered row of tabs at the top of the
screen. A stage names what a book already is. Selecting a tab SHALL show the
books of that stage. The system SHALL define the stage list on the server, so a
new stage needs no client change.

#### Scenario: The app opens

- **WHEN** a user opens the app
- **THEN** the tab row shows the tabs "Uploaded", "Dewarped", "Split" and "OCR"
  in that order
- **AND** the Uploaded tab is selected

#### Scenario: The selected tab survives a reload

- **WHEN** a user selects a stage tab and reloads the page
- **THEN** the same tab stays selected

#### Scenario: An unknown stage is requested

- **WHEN** a user opens a URL for a stage that does not exist
- **THEN** the system shows the first stage instead

### Requirement: A bulk action applies to every selected book

The action menu SHALL hold the actions of the current stage, which are the
actions that lead to the tabs to the right of the current tab. It SHALL hold the
same actions as the context menu of a single book, in the same order. It SHALL
NOT hold Reject: a bulk destroy is not offered.

Choosing an item SHALL run that action on every selected book. An action that
asks for an argument SHALL open its dialog first, and SHALL run on every
selected book once the user starts it. The system SHALL then clear the
selection. When the system rejects a book, the interface SHALL show the reason
and SHALL still run the action on the other books.

#### Scenario: The menu holds the later stages

- **WHEN** a user opens the action menu on the Uploaded tab
- **THEN** the menu holds "Dewarp" and "Split", in that order

#### Scenario: The menu on the middle tab

- **WHEN** a user opens the action menu on the Dewarped tab
- **THEN** the menu holds "Split" only

#### Scenario: A user runs a bulk action

- **WHEN** a user selects three books and chooses "Dewarp"
- **THEN** the system runs the Dewarp action on all three
- **AND** each of the three cards shows the spinner mark
- **AND** no book stays selected

#### Scenario: A user runs a bulk action that asks for an argument

- **WHEN** a user selects three books, chooses "OCR", and starts the dialog
- **THEN** the system runs the OCR action on all three with the chosen profile
- **AND** each of the three cards shows the spinner mark
- **AND** no book stays selected

#### Scenario: One book is rejected

- **WHEN** the system rejects one book of a bulk action
- **THEN** the interface shows the reason
- **AND** the other books still start

### Requirement: A right click offers the actions of the current stage

A right click on a card SHALL open a context menu at the pointer. The menu SHALL
hold the actions of the current stage when the book is ready for one, then a
divider, then a Reject item. The system SHALL show the Reject item in the colour
it uses for danger. A card that is ready for no action SHALL open a menu holding
Reject alone, so every card can be rejected. Pressing Escape or clicking away
SHALL close the menu and run nothing.

#### Scenario: A user right clicks a ready card

- **WHEN** a user right clicks a ready card on the Uploaded tab
- **THEN** a context menu opens at the pointer
- **AND** it holds "Dewarp", "Split", a divider, then "Reject"
- **AND** "Reject" is not the colour of the other items

#### Scenario: A user chooses an action

- **WHEN** a user chooses an action of the context menu that asks for no argument
- **THEN** the system runs that action on that book alone
- **AND** the selection does not change

#### Scenario: A user chooses an action that asks for an argument

- **WHEN** a user chooses an action of the context menu that asks for an argument
- **THEN** the dialog of that action opens for that book alone
- **AND** the selection does not change

#### Scenario: A user right clicks a card that is not ready

- **WHEN** a user right clicks a running card, or a card that already has a copy
- **THEN** the menu opens holding "Reject" alone
- **AND** it holds no action and no divider

#### Scenario: A user dismisses the menu

- **WHEN** the context menu is open and the user presses Escape or clicks away
- **THEN** the menu closes
- **AND** no action runs

### Requirement: A book view offers the actions of the book's stage

The book view and the reader SHALL show one button for every action of the
book's stage, in stage order, when the book is complete and has no copy. They
SHALL show the mark of the copy instead when a copy exists. They SHALL always
show a Reject control, in the colour the system uses for danger, beside the
actions.

Neither view SHALL fade any part of itself for a book that took its action. The
mark alone carries that state, so the pages stay at full strength.

#### Scenario: A user opens a ready book

- **WHEN** a user opens a complete book in the `uploaded` stage that has no copy
- **THEN** the view shows a "Dewarp" button and a "Split" button, in that order
- **AND** it shows a "Reject" control

#### Scenario: A user opens a book of the last stage

- **WHEN** a user opens a book in the `ocr` stage
- **THEN** the view shows no action button
- **AND** it still shows a "Reject" control

#### Scenario: A user runs an action from the book view

- **WHEN** a user activates an action button that asks for no argument
- **THEN** the buttons give way to a spinner mark
- **AND** the view needs no reload

#### Scenario: A user runs an action that asks for an argument

- **WHEN** a user activates an action button that asks for an argument, and
  starts its dialog
- **THEN** the buttons give way to a spinner mark
- **AND** the view needs no reload

#### Scenario: A user opens a book whose copy is complete

- **WHEN** a user opens a book whose copy is complete
- **THEN** the view shows a green checkmark and no action button
- **AND** the page thumbnails are not faded

#### Scenario: A user reads deep into a book

- **WHEN** a user scrolls the reader to page 40 of a 60 page book
- **THEN** the actions and the Reject control are still on screen
- **AND** the breadcrumb trail is still on screen
