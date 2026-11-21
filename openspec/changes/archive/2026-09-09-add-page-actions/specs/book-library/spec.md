## ADDED Requirements

### Requirement: A right click on a page offers the page menu

A right click on a page SHALL open a context menu at the pointer. The page grid
and the reader SHALL offer the same menu.

The menu SHALL hold "Show original" when that page offers a rollback, then a
divider, then a "Delete" item. The menu SHALL decide per page, not per book: a
page that already shows its original, or that names no source page, SHALL NOT
hold the item while its neighbours do. The system SHALL show "Delete" in the colour it
uses for danger. A page that offers no rollback SHALL open a menu holding
"Delete" alone, so every page can be deleted. Pressing Escape or clicking away
SHALL close the menu and run nothing.

The system SHALL take the offer of a rollback from the server, so a new action
needs no client change.

#### Scenario: A user right clicks a dewarped page

- **WHEN** a user right clicks a page in the grid of a dewarped copy that has no
  copy of its own
- **THEN** a context menu opens at the pointer
- **AND** it holds "Show original", a divider, then "Delete"
- **AND** "Delete" is not the colour of the other items

#### Scenario: A user right clicks an uploaded page

- **WHEN** a user right clicks a page of an uploaded book
- **THEN** the menu opens holding "Delete" alone
- **AND** it holds no divider

#### Scenario: A user right clicks a page that was rolled back

- **WHEN** a user right clicks a page that already shows its original
- **THEN** the menu opens holding "Delete" alone
- **AND** the menu of every other page of that book still holds "Show original"

#### Scenario: The reader offers the same menu

- **WHEN** a user right clicks a page in the reader
- **THEN** the same menu opens at the pointer

#### Scenario: A user dismisses the menu

- **WHEN** the page menu is open and the user presses Escape or clicks away
- **THEN** the menu closes
- **AND** nothing is deleted and nothing is rolled back

### Requirement: Deleting a page asks before it removes anything

The system SHALL ask the user to confirm a page delete before it removes
anything. The confirmation SHALL name how many books lose a page, SHALL list
them with their stages, and SHALL say that the step cannot be undone. Dismissing
the confirmation SHALL remove nothing.

When the system refuses the delete, the confirmation SHALL show the reason and
SHALL NOT offer to go on.

After a delete the page SHALL leave the grid or the reader of the user who ran
it, without a reload.

#### Scenario: A user opens the confirmation

- **WHEN** a user chooses Delete for a page of a book with a dewarped copy
- **THEN** the confirmation names two books and lists them with their stages
- **AND** it says the step cannot be undone

#### Scenario: A user dismisses the confirmation

- **WHEN** a user dismisses the confirmation
- **THEN** the page is still in the grid
- **AND** nothing was removed

#### Scenario: A user confirms

- **WHEN** a user confirms the delete
- **THEN** the page leaves the grid
- **AND** the pages that follow it keep their images
- **AND** the view is not reloaded

#### Scenario: The chain holds work in flight

- **WHEN** a user opens the confirmation for a book whose copy is running
- **THEN** the confirmation shows the reason the system refuses
- **AND** it offers no way to go on

#### Scenario: A user deletes from the reader

- **WHEN** a user confirms a delete from the reader
- **THEN** that page leaves the stack
- **AND** the reader stays where it was

### Requirement: The comparison shows the two pages side by side

Choosing "Show original" SHALL open a modal that holds both pages. The page the
rollback would install SHALL sit on the left, and the page it would replace on
the right. Each SHALL carry the title of the stage its book belongs to.

The modal SHALL use the screen. It SHALL scale both pages to the largest size
that fits them beside each other inside the window, and SHALL take no more room
than the pages and its own controls need. It SHALL NOT enlarge a page beyond its
stored display rendition. It SHALL follow a resize of the window.

The bottom of the modal SHALL hold "Use original", in the colour the system uses
for danger, then "Close". "Close" SHALL dismiss the modal and change nothing.

#### Scenario: A user compares a dewarped page

- **WHEN** a user chooses "Show original" for a page of a dewarped copy
- **THEN** a modal opens holding the uploaded page on the left and the dewarped
  page on the right
- **AND** each page carries the title of its stage
- **AND** the bottom holds "Use original" and "Close"

#### Scenario: The pages fill the window

- **WHEN** the comparison opens
- **THEN** both pages are as large as they fit beside each other in the window
- **AND** neither page is cropped
- **AND** neither page is larger than its stored display rendition

#### Scenario: The window is resized

- **WHEN** the user resizes the window while the comparison is open
- **THEN** both pages take the size the new window allows

#### Scenario: A user closes the comparison

- **WHEN** a user activates "Close"
- **THEN** the modal closes
- **AND** the page is unchanged

### Requirement: Using the original asks before it replaces the page

The system SHALL ask the user to confirm before it replaces a page. The
confirmation SHALL open over the comparison, which SHALL stay on screen beneath
it. The confirmation SHALL say that the step cannot be undone, and that getting
the current page back means running the action for the whole book again.
Dismissing the confirmation SHALL change nothing and SHALL leave the comparison
open.

After the replacement the comparison SHALL close, and the page SHALL show its
new image in the grid or the reader, without a reload. That page SHALL then
offer the rollback no more.

#### Scenario: The confirmation opens over the comparison

- **WHEN** a user activates "Use original"
- **THEN** the confirmation and the comparison are both on screen
- **AND** the confirmation is above the comparison

#### Scenario: A user confirms the replacement

- **WHEN** a user activates "Use original" and confirms
- **THEN** the comparison closes
- **AND** the page in the grid shows the image of the source page
- **AND** the view is not reloaded
- **AND** the menu of that page holds "Show original" no more

#### Scenario: A user dismisses the second confirmation

- **WHEN** a user activates "Use original" and dismisses the confirmation
- **THEN** the comparison is still open
- **AND** the page is unchanged

#### Scenario: The system refuses the replacement

- **WHEN** the system refuses the replacement
- **THEN** the comparison shows the reason
- **AND** the page is unchanged
