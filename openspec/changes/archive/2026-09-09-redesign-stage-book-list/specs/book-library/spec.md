## ADDED Requirements

### Requirement: A stage tab lists every book of that stage in one ordered list

A stage tab SHALL show one list that holds every book of that stage, whatever
its state. The system SHALL order that list by state group first, then by upload
time, newest first. The state groups SHALL be, in this order:

1. the book failed
2. the book still runs
3. the book is complete and has no copy
4. the book is complete and its copy runs or failed
5. the book is complete and its copy is complete

The system SHALL use the same list component for every stage tab.

#### Scenario: Books of several states share a tab

- **WHEN** a tab holds a failed book, a running book, a ready book, a book whose
  copy runs, and a book whose copy is complete
- **THEN** the list shows them in that order

#### Scenario: Two books share a state group

- **WHEN** two books of a tab are in the same state group
- **THEN** the book uploaded later comes first

#### Scenario: A book belongs to another stage

- **WHEN** the stage of a book is not the selected stage
- **THEN** that book does not appear in the list

#### Scenario: A running book appears in the tab it is going to

- **WHEN** a copy of the Dewarp action is in state `pending` or `processing`
- **THEN** it appears in the Dewarped tab
- **AND** it appears in no other tab

#### Scenario: The stage holds no book

- **WHEN** no book has the selected stage
- **THEN** the system shows an empty state that invites an upload

### Requirement: A card shows the state of its book

Every book in the list SHALL appear as a card. A card of a complete book SHALL
show the book name and the thumbnail of the book's first page as its cover.

A card of a running book SHALL show the book name only. Its cover SHALL be a
spinner icon of medium size that turns while the work runs, with the progress
phase of the book below the icon. The system SHALL hold the icon still for a
reader who asks for reduced motion.

A card of a failed book SHALL show the book name only. Its cover SHALL be a
warning icon of the same size, with the failure reason below the icon. The card
SHALL carry a retry control.

#### Scenario: A book is running

- **WHEN** a book in the list is in state `pending` or `processing`
- **THEN** its card shows the name, a spinner icon and the phase text
- **AND** the card shows no cover image

#### Scenario: The phase changes

- **WHEN** the phase of a running book changes
- **THEN** the text under the spinner icon changes with it
- **AND** the card keeps its place in the list

#### Scenario: A book fails

- **WHEN** a book in the list is in state `failed`
- **THEN** its card shows the name, a warning icon and the failure reason
- **AND** the card carries a retry control

#### Scenario: A book completes

- **WHEN** a running book reaches state `complete`
- **THEN** its card shows the cover thumbnail instead of the spinner icon
- **AND** the card moves to its new place in the list without a reload

### Requirement: A card is marked by the state of its copy

The system SHALL mark a complete book by the state of the copy that references
it. A book whose copy runs SHALL carry a turning spinner mark in the top right
corner of its card. A book whose copy failed SHALL carry a warning mark there. A
book whose copy is complete SHALL carry a green checkmark there. The system
SHALL hold the spinner still for a reader who asks for reduced motion.

A card with a mark SHALL fade. A card whose copy is complete SHALL fade more
than a card whose copy still runs.

The system SHALL apply every mark from a server event, without a reload.

#### Scenario: A user runs an action

- **WHEN** a user runs an action on a book
- **THEN** the checkbox of that card gives way to a spinner mark
- **AND** the card fades
- **AND** the view needs no reload

#### Scenario: The copy completes

- **WHEN** the copy of a book reaches state `complete`
- **THEN** the spinner mark gives way to a green checkmark
- **AND** the card fades further
- **AND** the card moves to the last state group

#### Scenario: The copy fails

- **WHEN** the copy of a book reaches state `failed`
- **THEN** the card of the source book carries a warning mark
- **AND** the card stays in the same state group

#### Scenario: Another user runs the action

- **WHEN** a second user runs an action on a book that the first user has in
  view
- **THEN** the first user's card for that book shows the spinner mark and fades
- **AND** the first user reloads nothing

#### Scenario: The marks survive a reload

- **WHEN** a user reloads a stage tab
- **THEN** every card still carries the mark that fits its copy

### Requirement: A book that is ready for an action carries a checkbox

A card SHALL carry a checkbox in its top right corner when the book is complete,
has no copy, and its stage offers at least one action. No other card SHALL carry
a checkbox. Selecting the checkbox SHALL NOT open the book.

#### Scenario: A ready book

- **WHEN** a complete book with no copy sits in a tab that offers an action
- **THEN** its card carries a checkbox in the top right corner

#### Scenario: A book that is not ready

- **WHEN** a book is running, failed, or already has a copy
- **THEN** its card carries no checkbox

#### Scenario: The last stage

- **WHEN** a user opens the Done tab
- **THEN** no card carries a checkbox

#### Scenario: A user selects a book

- **WHEN** a user activates the checkbox of a card
- **THEN** the book is selected
- **AND** the book view does not open

### Requirement: The top bar holds the selection controls

The system SHALL show a "Select all" control in the top bar, after the upload
control, on every tab that offers at least one action. Activating it SHALL
select every book of the tab that carries a checkbox.

While at least one book is selected, the system SHALL also show a "Deselect all"
control and an action menu, in that order, to the right of "Select all".
Activating "Deselect all" SHALL clear the selection.

The system SHALL clear the selection when the user leaves the tab.

#### Scenario: Nothing is selected

- **WHEN** a user opens a tab and selects no book
- **THEN** the top bar shows "Select all"
- **AND** it shows no "Deselect all" control and no action menu

#### Scenario: A user selects all books

- **WHEN** a user activates "Select all"
- **THEN** every card that carries a checkbox is selected
- **AND** no other card is selected

#### Scenario: A user selects one book

- **WHEN** a user selects a single book
- **THEN** the top bar shows "Deselect all" and the action menu

#### Scenario: A user clears the selection

- **WHEN** a user activates "Deselect all"
- **THEN** no book is selected
- **AND** the "Deselect all" control and the action menu disappear

#### Scenario: A user changes tab

- **WHEN** a user selects books and then opens another tab
- **THEN** no book is selected

#### Scenario: The last stage

- **WHEN** a user opens the Done tab
- **THEN** the top bar shows no selection control

### Requirement: A bulk action applies to every selected book

The action menu SHALL hold the actions of the current stage, which are the
actions that lead to the tabs to the right of the current tab. It SHALL hold the
same actions as the context menu of a single book, in the same order. It SHALL
NOT hold Reject: a bulk destroy is not offered.

Choosing an item SHALL run that action on every selected book. The system SHALL
then clear the selection. When the system rejects a book, the interface SHALL
show the reason and SHALL still run the action on the other books.

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

- **WHEN** a user chooses an action of the context menu
- **THEN** the system runs that action on that book alone
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

- **WHEN** a user opens a book in the `done` stage
- **THEN** the view shows no action button
- **AND** it still shows a "Reject" control

#### Scenario: A user runs an action from the book view

- **WHEN** a user activates an action button
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

### Requirement: Reject asks before it destroys

The system SHALL ask the user to confirm a Reject before it destroys anything.
The confirmation SHALL name how many books it destroys, SHALL list them, and
SHALL say that the step cannot be undone. It SHALL say that the source book
returns to its stage when there is a source to free. Dismissing the
confirmation SHALL destroy nothing.

The system SHALL take a rejected card out of every list that holds it, without a
reload, on every connected client. A view of a destroyed book SHALL return to
the stage list of that book.

#### Scenario: A user opens the confirmation

- **WHEN** a user chooses Reject for a book that has no copy of its own
- **THEN** the confirmation names one book and lists it
- **AND** it says the step cannot be undone

#### Scenario: A book with copies of its own

- **WHEN** a user chooses Reject for a book that two further copies came from
- **THEN** the confirmation names three books and lists all of them

#### Scenario: A user dismisses the confirmation

- **WHEN** a user dismisses the confirmation
- **THEN** the book is still there
- **AND** nothing was destroyed

#### Scenario: A user confirms

- **WHEN** a user confirms the Reject of a copy
- **THEN** the card of the copy leaves its list
- **AND** the card of the source book returns to the ready group with a checkbox
- **AND** neither list is reloaded

#### Scenario: Another user rejects a book in view

- **WHEN** a second user rejects a book that the first user has in a list
- **THEN** the card leaves the first user's list without a reload

#### Scenario: A user rejects the book being read

- **WHEN** a user confirms a Reject from the reader or the book view
- **THEN** the system opens the stage list of that book

### Requirement: Breadcrumbs show the path to the current view

The book view and the reader SHALL show a breadcrumb trail instead of a back
link. The trail of a book view SHALL be the title of the book's stage, then the
book name. The trail of the reader SHALL add a third step named "View". Every
step except the last SHALL open its view.

#### Scenario: A user opens a book

- **WHEN** a user opens a book whose stage is `dewarped`
- **THEN** the view shows the trail "Dewarped", then the book name

#### Scenario: A user opens the reader

- **WHEN** a user opens the reader of that book
- **THEN** the view shows the trail "Dewarped", the book name, then "View"

#### Scenario: A user returns to the stage

- **WHEN** a user activates the first step of the trail
- **THEN** the system opens the tab of that stage

#### Scenario: A user returns to the book

- **WHEN** a user activates the book name in the reader trail
- **THEN** the system opens the page grid of that book

### Requirement: The reader fits a page to the content column

The reader SHALL show every page of the book stacked vertically, in page order,
each fitted to the width of the content column. The reader SHALL NOT enlarge a
page beyond its stored size. The reader SHALL use the 1280 px display rendition.
The reader SHALL show no page number.

#### Scenario: A user scrolls the reader

- **WHEN** a user scrolls down in the reader
- **THEN** the next page follows directly below the current page
- **AND** pages stay in page order

#### Scenario: A page is wider than the column

- **WHEN** a page is wider than the content column
- **THEN** the page is scaled down to the column width
- **AND** the page is not cropped

#### Scenario: A page number is never shown

- **WHEN** a user reads any page
- **THEN** no page number appears with it

#### Scenario: A large book opens

- **WHEN** a user opens the reader for a book of 500 pages
- **THEN** the view appears without loading all 500 images at once
- **AND** images load as they approach the viewport

## MODIFIED Requirements

### Requirement: Stages appear as tabs

The system SHALL show the stages as an ordered row of tabs at the top of the
screen. A stage names what a book already is. Selecting a tab SHALL show the
books of that stage. The system SHALL define the stage list on the server, so a
new stage needs no client change.

#### Scenario: The app opens

- **WHEN** a user opens the app
- **THEN** the tab row shows the tabs "Uploaded", "Dewarped" and "Done" in that
  order
- **AND** the Uploaded tab is selected

#### Scenario: The selected tab survives a reload

- **WHEN** a user selects a stage tab and reloads the page
- **THEN** the same tab stays selected

#### Scenario: An unknown stage is requested

- **WHEN** a user opens a URL for a stage that does not exist
- **THEN** the system shows the first stage instead

### Requirement: Opening a book shows its pages in a regular grid

Selecting a book SHALL show every page of that book as a thumbnail in a grid of
five cells per row. Every grid cell SHALL have the same size. The cells SHALL
share the width of the container, so the grid of pages and the grid of books
have a similar cell size.

#### Scenario: A user opens a book

- **WHEN** a user selects a book with 120 pages
- **THEN** the system shows 120 thumbnails in page order
- **AND** every row holds five cells
- **AND** every cell has identical width and height

#### Scenario: The window is resized

- **WHEN** the width of the window changes
- **THEN** every row still holds five cells
- **AND** the cells keep their shape

#### Scenario: A page is smaller than its cell

- **WHEN** a page's thumbnail is smaller than the cell
- **THEN** the thumbnail is centered in the cell
- **AND** the thumbnail is not cropped and not enlarged

### Requirement: The grid cell size derives from the page dimensions

The system SHALL compute one cell shape for the whole book from the dimensions
of all its pages, and SHALL report it with the book. The reported shape SHALL
have a longest side of 480 px, and SHALL use the median page aspect ratio, where
aspect ratio is width divided by height. The page grid SHALL keep that shape and
SHALL take its width from the container.

#### Scenario: Every page is landscape

- **WHEN** every page of a book is wider than it is tall with median aspect 1.5
- **THEN** the reported shape is 480 x 320 px
- **AND** every grid cell is one and a half times as wide as it is tall

#### Scenario: Every page is portrait

- **WHEN** every page of a book is taller than it is wide with median aspect
  0.667
- **THEN** the reported shape is 320 x 480 px

#### Scenario: Pages mix orientations

- **WHEN** a book holds both landscape and portrait pages
- **THEN** the reported shape is 480 x 480 px
- **AND** every grid cell is square

#### Scenario: A page is square

- **WHEN** any page of a book is exactly square
- **THEN** the reported shape is 480 x 480 px

#### Scenario: Pages are extremely wide

- **WHEN** every page is landscape with median aspect 6.0
- **THEN** the short side of the shape is clamped to 240 px
- **AND** the reported shape is 480 x 240 px

#### Scenario: The book has one page

- **WHEN** a book holds a single page of 2000 x 3000 pixels
- **THEN** the reported shape is 320 x 480 px

### Requirement: Book lists update live

The system SHALL update the visible book list from server events without a page
reload. An event SHALL move a card to the place its new state gives it. A card
that only changes its phase SHALL keep its place.

#### Scenario: Another user uploads a book

- **WHEN** a second user uploads a book
- **THEN** the first user's Uploaded tab shows the new book without a reload

#### Scenario: A book changes state group

- **WHEN** a running book of the visible tab reaches state `complete`
- **THEN** its card moves from the running group to the ready group
- **AND** the user reloads nothing

#### Scenario: The connection drops

- **WHEN** the event connection closes
- **THEN** the interface shows the connection state
- **AND** it reconnects and refreshes the visible list

## REMOVED Requirements

### Requirement: A stage tab lists the books pending that stage

**Reason**: A stage no longer names pending work. It names what a book already
is, and its tab now holds every book of that stage in one ordered list.

**Migration**: Replaced by "A stage tab lists every book of that stage in one
ordered list", "A card shows the state of its book" and "A card is marked by the
state of its copy".

### Requirement: Books that are not complete appear apart from the stage list

**Reason**: The separate area forced a user to watch two places at once. A
running book now carries its stage from creation, so it belongs in one list with
every other book of that stage.

**Migration**: Replaced by "A stage tab lists every book of that stage in one
ordered list" and "A card shows the state of its book". A running book appears
in the tab of the stage it is going to.

### Requirement: The reader shows pages full width in order

**Reason**: A page that spans the whole window is larger than a screen can use
and breaks out of the app column. A page number under every page adds noise.

**Migration**: Replaced by "The reader fits a page to the content column".
