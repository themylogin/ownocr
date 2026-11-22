## Purpose

Lets a user browse books by stage in one ordered list, act on them one at a time
or in bulk, see one book's pages as a regular grid, and read those pages in
order.

## Requirements

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

### Requirement: Selecting a page opens the reader at that page

Selecting a thumbnail SHALL open the reader view scrolled to that page.

#### Scenario: A user selects page 42

- **WHEN** a user selects the thumbnail of page 42
- **THEN** the reader opens
- **AND** page 42 is in view

#### Scenario: The reader URL is shared

- **WHEN** a user opens a reader URL for a specific page directly
- **THEN** the reader opens scrolled to that page

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

This requirement covers a book that is not in the `ocr` stage. A book in the
`ocr` stage follows the OCR reader requirements, which use the whole window
width.

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

#### Scenario: An OCR book uses the whole window

- **WHEN** a user opens the reader for a book in the `ocr` stage
- **THEN** the pages are not fitted to the content column

### Requirement: The reader of an OCR book shows the text beside the page

The reader SHALL split in two for a book in the `ocr` stage. The left half SHALL
show the page image. The right half SHALL show the same image, covered by a white
veil of 50 percent opacity, with the read text drawn on top of the veil.

The reader SHALL draw one text box per read line, at the place the line box
names. Each drawn line SHALL stand on a white ground the size of its box, so the
print under a line reaches no glyph. The text SHALL be the text of that line,
unchanged. The drawn text SHALL be selectable, so a user can copy it.

The reader SHALL draw every white ground below every drawn line. No ground SHALL
cover the text of another line, whatever the boxes of two lines do to one
another. A glyph that reaches out of its own box SHALL stay readable.

The two halves SHALL stand side by side and SHALL scroll together, so a line of
the right half stays beside the same line of the left half. The reader SHALL keep
the page order and SHALL show no page number.

The reader of a book in every other stage SHALL NOT split, SHALL show no veil,
and SHALL draw no text.

#### Scenario: A user opens the reader of an OCR book

- **WHEN** a user opens the reader for a book in the `ocr` stage
- **THEN** every page appears twice, side by side
- **AND** the right image carries the white veil and the drawn text

#### Scenario: The drawn text matches the lines

- **WHEN** a page holds a result of 40 lines
- **THEN** the right half draws 40 text boxes
- **AND** each one holds the text of its line

#### Scenario: A ground covers no text

- **WHEN** a page holds two lines whose boxes touch or overlap
- **THEN** the text of the upper line is drawn over the ground of the lower line
- **AND** no part of a glyph is hidden by a ground

#### Scenario: A user copies the text

- **WHEN** a user selects the drawn text of a line and copies it
- **THEN** the clipboard holds that text
- **AND** the lines read in page order

#### Scenario: A book of another stage is unchanged

- **WHEN** a user opens the reader for a book in the `split` stage
- **THEN** the reader shows one image per page, in one column
- **AND** no veil and no text appear

### Requirement: The OCR reader fits both halves in the window

The OCR reader SHALL use the whole window width and SHALL NOT stay inside the
72 rem content column. The two halves of one page, and the gap between them,
SHALL fit in the window width. The reader SHALL scale both halves down together
when they do not fit, so the two images always keep the same size.

The reader SHALL NOT enlarge a page beyond its stored size. The reader SHALL use
the 1280 px display rendition for both halves.

The line boxes SHALL follow the same scale as the image they lie on. The font
size of a drawn line SHALL follow the height of its box, and the text SHALL fill
the width of its box. A change of the window width SHALL move and resize the
boxes with the image.

#### Scenario: Two halves do not fit

- **WHEN** the two halves of a page are wider than the window
- **THEN** both halves are scaled down by the same factor
- **AND** together with the gap they fit in the window width
- **AND** neither half is cropped

#### Scenario: The page is smaller than half the window

- **WHEN** half the window is wider than the stored page
- **THEN** the page is shown at its stored size
- **AND** it is not enlarged

#### Scenario: A box follows its image

- **WHEN** the reader scales a page down by half
- **THEN** every box of that page is half as wide and half as high
- **AND** the drawn text is half as large

#### Scenario: A user resizes the window

- **WHEN** a user makes the window narrower
- **THEN** both halves and every box of the visible pages take the new scale

### Requirement: The OCR reader holds the text of every page

The reader SHALL ask for the lines of every page of the book in one request, when
it opens. It SHALL draw the text of every page it holds lines for, whether that
page is near the window or far from it. It SHALL remove the text of no page while
the reader stays open.

A page SHALL reserve the size it takes before it is drawn, so the reader holds
its full height from the start. No page SHALL move while a user scrolls, and a
jump to a page SHALL land on that page.

A page that holds no result SHALL show the veiled image with no text, and SHALL
raise no error to the user. A book whose lines do not arrive SHALL show every
page as a veiled image, and SHALL raise no error.

The reader SHALL skip the drawing of a page that lies far from the window, and
SHALL draw it when it comes near. Skipping the drawing SHALL NOT remove the text
of that page, and SHALL NOT change the place or the size of any page.

#### Scenario: A large OCR book opens

- **WHEN** a user opens the reader for an OCR book of 500 pages
- **THEN** the reader makes one request for the lines of the book
- **AND** it makes no further request while it stays open

#### Scenario: A user scrolls on

- **WHEN** a user scrolls past a page that carried drawn text
- **THEN** that page keeps its text
- **AND** the pages that come near the window show their text
- **AND** no page moves

#### Scenario: A user opens the reader at a page

- **WHEN** a user opens the reader at page 300 of a book of 500 pages
- **THEN** the reader lands on page 300
- **AND** the pages around it show their text

#### Scenario: A page holds no result

- **WHEN** the reader reaches a page of a running OCR copy that holds no result
- **THEN** the right half shows the veiled image with no text
- **AND** the reader shows no error

#### Scenario: The lines do not arrive

- **WHEN** the request for the lines of the book fails
- **THEN** every page shows its veiled image with no text
- **AND** the reader shows no error

### Requirement: Find-in-page reaches every page of the OCR reader

The browser find SHALL reach the drawn text of every page of the book, not the
pages near the window only. A match on a page far from the window SHALL scroll
that page into view.

The count the browser reports SHALL count the matches of the whole book. A reader
who searches a word SHALL learn how often the book holds it.

The find SHALL match inside one drawn line. A drawn line stands at the place its
box names, so two lines are two separate pieces of text, and a phrase that
crosses a line break SHALL NOT match.

#### Scenario: A user searches a word on a far page

- **WHEN** a user opens the reader at page 1 of a book of 500 pages, and searches
  a word that only page 400 holds
- **THEN** the browser reports the match
- **AND** it scrolls page 400 into view

#### Scenario: The count covers the book

- **WHEN** a user searches a word the book holds 30 times, across 20 pages
- **THEN** the browser reports 30 matches

#### Scenario: A phrase inside one line

- **WHEN** a user searches a phrase that one drawn line holds in full
- **THEN** the browser reports the match

#### Scenario: A phrase across two lines

- **WHEN** a user searches a phrase whose words end one drawn line and begin the
  next
- **THEN** the browser reports no match

### Requirement: The right half of the OCR reader offers the reading of the page

A right click on the right half of the OCR reader SHALL open a menu about the
reading of that page and nothing else. It SHALL NOT open the page menu, and it
SHALL NOT open the menu of the browser.

The right half stacks an image, a white veil and drawn text, so a copy saved from
it would hold the page without one glyph of the reading. Every item that reaches
the image therefore stays on the left half, and everything about the reading
belongs here: a user who finds a badly read page is reading it on this half.

The menu SHALL hold, in this order:

- The readings that page can answer, one item each, named by the profile that made
  them and the engine behind them, ordered "Normal", "Extra", "Ultra". The
  `page-reading-choice` capability says which readings those are and what choosing
  one does.
- Why the reading of that page is doubted, when it is doubted, under the reading it
  judges rather than under all of them.
- The rescan item, named "Rescan" for a page that is not marked and "Do not
  rescan" for one that is. The `page-rescan` capability governs what choosing it
  does.

The readings SHALL be offered best last and not best first. A reader reads the
list as the dialog that starts a rescan reads it, and that dialog offers Normal,
then Extra, then Ultra.

A page of a book that has not been read holds no reading and is doubted for none,
so its menu SHALL hold the rescan item alone. Every page of a complete book in the
`ocr` stage offers the rescan, whatever results that page holds. A right click on
the right half of a page of a book that offers no rescan SHALL open no menu. An
empty menu says less than no menu.

A right click on the left half SHALL open the page menu, as a right click on any
other page does. A right click on the gap between the two halves SHALL open no
menu.

The drawn text SHALL stay selectable, and a user SHALL still copy a selection by
keyboard.

#### Scenario: A user right clicks the read half

- **WHEN** a user right clicks the right half of a page in the reader of a
  complete book whose profile is `v6`
- **THEN** a menu opens at the pointer holding "Normal (PP-OCRv6)" and "Rescan"
- **AND** no browser menu opens

#### Scenario: A user right clicks the read half of a page read three times

- **WHEN** a user right clicks the right half of a page holding the line reading,
  the merged PaddleOCR-VL reading and the merged Ultra reading
- **THEN** the menu holds "Normal (PP-OCRv6)", then "Extra (PaddleOCR-VL)", then
  "Ultra (GLM-5.3 Flash)", in that order
- **AND** the rescan item stands after all three

#### Scenario: A user right clicks the read half of a doubted page

- **WHEN** a user right clicks the right half of a page whose reading is doubted
- **THEN** the menu says why, under the reading that answers

#### Scenario: A user right clicks the read half of a marked page

- **WHEN** a user right clicks the right half of a page already marked for a
  rescan
- **THEN** the menu holds "Do not rescan"
- **AND** it holds no item named "Rescan"

#### Scenario: A user right clicks the read half of an already rescanned page

- **WHEN** a user right clicks the right half of a page that holds a
  PaddleOCR-VL result
- **THEN** a menu opens at the pointer holding the rescan item
- **AND** no browser menu opens

#### Scenario: A user right clicks the read half of a running book

- **WHEN** a user right clicks the right half of a page of a book in the `ocr`
  stage that is not complete
- **THEN** no menu opens
- **AND** no browser menu opens

#### Scenario: A user right clicks the page half

- **WHEN** a user right clicks the left half of a page in the reader of a book in
  the `ocr` stage
- **THEN** the page menu opens at the pointer
- **AND** it holds no reading, no rescan item and no explanation of a doubt

#### Scenario: A user copies the text of a read page

- **WHEN** a user selects the drawn text of a line and copies it by keyboard
- **THEN** the clipboard holds that text

#### Scenario: The reader of another stage keeps its menu

- **WHEN** a user right clicks a page in the reader of a book in the `split`
  stage
- **THEN** the page menu opens at the pointer

### Requirement: A right click on a page offers the page menu

A right click on a page SHALL open a context menu at the pointer. The page grid
and the reader SHALL offer the same menu.

This menu SHALL reach the image of the page and change the page. It SHALL hold
nothing about the reading of that page: no reading to choose, no rescan and no
explanation of a doubt. Those belong to the read half of the reader, which the
requirement above governs, because that half is where the reading is.

The menu SHALL open with "Open in new tab" and "Download", then a divider. Both
items SHALL appear for every page, so a page always opens a menu holding at least
those two and "Delete".

The menu SHALL then hold "Edit" when that page offers a re-cut, then "Show
original" when that page offers a rollback, then a divider, then a "Delete"
item. No page offers both, because an action makes either one page from one page
or two. The menu SHALL decide per page, not per book: a page that already shows
its original, that names no source page, or that records no cut, SHALL NOT hold
its item while its neighbours do. The system SHALL show "Delete" in the colour it
uses for danger. A page that offers none of the three SHALL open a menu holding
"Open in new tab", "Download", a divider and "Delete". Pressing Escape or
clicking away SHALL close the menu and run nothing.

The system SHALL take the offer of a rollback, the offer of a re-cut and the
offer of a rescan from the server, so a new action needs no client change. The
server reports the offer of a rescan for the book, so every page of a book that
offers it holds the item.

#### Scenario: A user right clicks a page of a book read with the normal profile

- **WHEN** a user right clicks the left half of a page of a complete book whose
  profile is `v6`
- **THEN** the menu opens holding "Open in new tab", "Download", a divider, then
  "Delete"
- **AND** it holds no reading, no "Rescan" and no explanation of a doubt
- **AND** it holds no "Edit" and no "Show original"

#### Scenario: A user right clicks the page half of a page read three times

- **WHEN** a user right clicks the left half of a page holding the line reading,
  the merged PaddleOCR-VL reading and the merged Ultra reading
- **THEN** the menu holds no reading item
- **AND** the read half of that page offers all three

#### Scenario: A page of an unread book

- **WHEN** a user right clicks a page of a book in the `split` stage
- **THEN** the menu opens with "Open in new tab"
- **AND** it holds no reading item

#### Scenario: A user right clicks an already rescanned page

- **WHEN** a user right clicks the left half of a page that holds a PaddleOCR-VL
  result
- **THEN** the menu opens holding "Open in new tab", "Download", a divider, then
  "Delete"

#### Scenario: A user right clicks a page of a book read with the extra profile

- **WHEN** a user right clicks the left half of a page in the reader of a
  complete book whose profile is `vl`
- **THEN** the menu holds no "Rescan"
- **AND** the read half of that page holds it

#### Scenario: A user right clicks a dewarped page

- **WHEN** a user right clicks a page in the grid of a dewarped copy that has no
  copy of its own
- **THEN** a context menu opens at the pointer
- **AND** it holds "Open in new tab", "Download", a divider, "Show original", a
  divider, then "Delete"
- **AND** it holds no "Rescan"
- **AND** "Delete" is not the colour of the other items

#### Scenario: A user right clicks a page cut out of a spread

- **WHEN** a user right clicks a page of a split copy that was cut out of a
  spread, and that copy has no copy of its own
- **THEN** the menu opens holding "Open in new tab", "Download", a divider,
  "Edit", a divider, then "Delete"
- **AND** "Open in new tab" is the first item

#### Scenario: A user right clicks a page the split copied whole

- **WHEN** a user right clicks a page of a split copy that records no cut
- **THEN** the menu opens holding "Open in new tab", "Download", a divider, then
  "Delete"
- **AND** it holds no "Edit"
- **AND** the menu of a page of that book that was cut still holds "Edit"

#### Scenario: A user right clicks an uploaded page

- **WHEN** a user right clicks a page of an uploaded book
- **THEN** the menu opens holding "Open in new tab", "Download", a divider, then
  "Delete"
- **AND** it holds no "Edit" and no "Show original"

#### Scenario: A user right clicks a page that was rolled back

- **WHEN** a user right clicks a page that already shows its original
- **THEN** the menu opens holding "Open in new tab", "Download", a divider, then
  "Delete"
- **AND** the menu of every other page of that book still holds "Show original"

#### Scenario: The reader offers the same menu

- **WHEN** a user right clicks a page in the reader
- **THEN** the same menu opens at the pointer

#### Scenario: The grid offers no rescan

- **WHEN** a user right clicks a page in the grid of a complete book whose
  profile is `v6`
- **THEN** the menu opens holding "Open in new tab", "Download", a divider, then
  "Delete"
- **AND** it holds no "Rescan"

#### Scenario: A user dismisses the menu

- **WHEN** the page menu is open and the user presses Escape or clicks away
- **THEN** the menu closes
- **AND** nothing is deleted, nothing is rolled back, nothing is cut and no
  rescan starts

### Requirement: The page menu reaches the original rendition of the page

"Open in new tab" SHALL open the original rendition of the page in a new browser
tab. It SHALL leave the current tab on the view the user right clicked in.

"Download" SHALL save the original rendition of the page to the browser. It
SHALL save the file under a name built from the book name, a space, a hyphen, a
space, and the page index in four digits, with the extension of the original
rendition. A book named `Vergil` SHALL save its seventh page as
`Vergil - 0007.jpg`.

Both items SHALL act on the original rendition, whatever rendition the view
shows. The grid shows the 480 px thumbnail and the reader shows the 1280 px
display rendition, and neither item SHALL act on either one.

The original rendition keeps the extension of the uploaded file, so both items
SHALL take the extension from the page, not from a fixed value.

#### Scenario: A user opens the original from the grid

- **WHEN** a user right clicks a page in the grid and selects "Open in new tab"
- **THEN** a new tab opens on the original rendition of that page
- **AND** the grid stays open in the tab the user started in

#### Scenario: A user opens the original from the reader

- **WHEN** a user right clicks a page in the reader and selects "Open in new tab"
- **THEN** a new tab opens on the original rendition of that page
- **AND** the tab shows the original, not the 1280 px display rendition

#### Scenario: A user downloads a page

- **WHEN** a user right clicks the seventh page of the book `Vergil` and selects
  "Download"
- **THEN** the browser saves the original rendition of that page
- **AND** the saved file is named `Vergil - 0007.jpg`

#### Scenario: A page uploaded as PNG keeps its extension

- **WHEN** a user downloads a page whose original rendition is a PNG file
- **THEN** the saved file carries the `.png` extension

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

### Requirement: The cut editor shows the parent image and the cut

Choosing "Edit" SHALL open a modal that holds the page this page was cut out of,
whole. A vertical bar SHALL stand on the column the page is cut at.

The part of the parent image that this page holds SHALL stay plain. The rest of
the parent image SHALL lie under a grey layer at half transparency, so the user
sees both the page and what it leaves out.

The modal SHALL use the screen. It SHALL scale the parent image to the largest
size that fits inside the window, and SHALL take no more room than the image and
its own controls need. It SHALL NOT enlarge the image beyond its stored display
rendition. It SHALL follow a resize of the window.

The user SHALL move the bar by dragging it or its handle across the image. The
plain part and the grey part SHALL follow the bar while the user drags. The bar
SHALL NOT leave the image, and SHALL NOT reach the edge that would leave this
page with no column.

#### Scenario: A user opens the editor on a left page

- **WHEN** a user chooses "Edit" for the left page of a split spread
- **THEN** a modal opens holding the whole spread
- **AND** a vertical bar stands where the page ends
- **AND** the part of the spread right of the bar lies under a grey layer

#### Scenario: A user opens the editor on a right page

- **WHEN** a user chooses "Edit" for the right page of a split spread
- **THEN** the part of the spread left of the bar lies under the grey layer

#### Scenario: The image fills the window

- **WHEN** the cut editor opens
- **THEN** the parent image is as large as it fits in the window
- **AND** it is not cropped
- **AND** it is not larger than its stored display rendition

#### Scenario: A user drags the bar

- **WHEN** a user drags the bar across the image
- **THEN** the bar follows the pointer
- **AND** the plain part of the image grows or shrinks with it

#### Scenario: The window is resized

- **WHEN** the user resizes the window while the editor is open
- **THEN** the image takes the size the new window allows
- **AND** the bar still stands on the same column of the image

### Requirement: The cut editor saves or cancels

The bottom of the cut editor SHALL hold "Save", then "Cancel". "Cancel" SHALL
close the modal and change nothing, wherever the user left the bar.

"Save" SHALL send the column the bar stands on. After the answer the modal SHALL
close, and the page SHALL show its new image in the grid or the reader, without
a reload. The other page of the spread SHALL be unchanged.

When the system refuses the save, the modal SHALL show the reason and SHALL stay
open. The page SHALL be unchanged.

#### Scenario: A user cancels

- **WHEN** a user moves the bar and then activates "Cancel"
- **THEN** the modal closes
- **AND** the page in the grid is unchanged

#### Scenario: A user saves

- **WHEN** a user moves the bar and then activates "Save"
- **THEN** the modal closes
- **AND** the page in the grid shows the image the new cut gives
- **AND** the view is not reloaded

#### Scenario: The neighbour page is untouched

- **WHEN** a user saves a new cut for one page of a spread
- **THEN** the other page of that spread shows the image it showed before

#### Scenario: The system refuses the save

- **WHEN** the system refuses the save
- **THEN** the modal stays open and shows the reason
- **AND** the page is unchanged

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

### Requirement: The rescan asks for a model before it runs

The interface SHALL ask for the model a rescan reads with, in the dialog that
starts the rescan. The rescan SHALL NOT start before the user answers the dialog.

The dialog SHALL keep its own title, which names the rescan and not the OCR
action. It SHALL say what a rescan does and why a user wants one. Its starting
control SHALL be named "Rescan".

The dialog SHALL offer the two choices the OCR dialog offers, in the same order
and the same words, and a third the OCR dialog does not offer:

- "Normal", subtitled "PP-OCRv6", described as the frontier OCR model.
- "Extra", subtitled "PaddleOCR-VL", described as the visual model, for
  difficult cases like polytonic Greek.
- "Ultra", subtitled "GLM-5.3 Flash", described as the frontier visual model, to
  use when even PaddleOCR-VL fails.

The dialog SHALL preselect the model after the one the book was read with, where
the OCR dialog preselects "Normal". A user rescans a page because the model it was
read with read that page badly, so the next model up is the common answer.

A book read with PP-OCRv6 SHALL therefore preselect "Extra", and a book read with
PaddleOCR-VL SHALL preselect "Ultra", which is the only model left to try.

Cancelling, pressing Escape, or clicking away SHALL close the dialog, start
nothing, and leave every mark standing.

Starting SHALL run the rescan of every marked page with the chosen profile. A
refusal from the server SHALL appear in the dialog, and the dialog SHALL stay
open with every mark standing.

#### Scenario: A user opens the dialog

- **WHEN** a user marks two pages of a book whose profile is `v6` and activates the
  rescan control
- **THEN** the dialog opens holding "Normal", "Extra" and "Ultra" in that order
- **AND** "Extra" is selected
- **AND** no rescan has started

#### Scenario: A user opens the dialog on a book read with PaddleOCR-VL

- **WHEN** a user marks a page of a book whose profile is `vl` and activates the
  rescan control
- **THEN** "Ultra" is selected
- **AND** no rescan has started

#### Scenario: A user starts the extra profile

- **WHEN** a user opens the dialog of a book whose profile is `v6` and starts it
  without changing the choice
- **THEN** the system rescans the marked pages with the profile `vl`
- **AND** the dialog closes

#### Scenario: A user starts the Ultra profile

- **WHEN** a user picks "Ultra" and starts the dialog
- **THEN** the system rescans the marked pages with the profile `glm`

#### Scenario: The OCR dialog offers no Ultra

- **WHEN** a user opens the dialog of the OCR action
- **THEN** it offers "Normal" and "Extra" and no third choice

#### Scenario: A user starts the normal profile

- **WHEN** a user picks "Normal" and starts the dialog
- **THEN** the system rescans the marked pages with the profile `v6`

#### Scenario: A user cancels the dialog

- **WHEN** a user cancels the dialog, presses Escape, or clicks away
- **THEN** the dialog closes
- **AND** no rescan runs
- **AND** every marked page keeps its mark

#### Scenario: The server refuses the rescan

- **WHEN** the server refuses the rescan the dialog started
- **THEN** the dialog shows the reason
- **AND** the reader stays open with every mark standing

### Requirement: The reader reports the pages that disagree

The reader of a book whose pages hold more than one reading SHALL name the number
of pages that disagree, in the same block as the breadcrumbs and immediately after
the last of them. It SHALL say it in words, as "16 pages disagree", and SHALL say
"1 page disagrees" for one.

It SHALL read as part of the trail and not as a control of its own, so it stands
beside the last crumb and SHALL NOT be pushed to the far side of the bar.

It SHALL say nothing at all when no page of the book disagrees, and nothing for a
book whose pages hold one reading each. A count of zero is not news.

The reader SHALL draw a border in the colour the system uses for a warning round
the read half of every disagreeing page. The read half holds the text that
disagrees, so the mark sits on what is wrong. The border SHALL surround that half
alone, not the pair, and SHALL move no page.

The border SHALL show on all four edges of that half, in a window of any width.
The reader skips the layout of a page off screen, which clips whatever is drawn
outside the box of that half, so the border SHALL be drawn inside the box.

The `page-reading-health` capability decides which pages disagree.

A page carrying more than one reason SHALL carry a border for each, one inside the
other, and every one of them SHALL be visible. The mark of a rescan wears the
colour of success, because a user chose that page; this border wears the colour of
a warning, because the system doubts that reading; and the border of a page holding
an Ultra reading wears the colour the system uses for information. Each says a
different thing, so none may replace another.

#### Scenario: A book with sixteen disagreeing pages

- **WHEN** a user opens the reader of a book whose pages hold two readings, and
  sixteen of them disagree
- **THEN** the bar says "16 pages disagree" in the same block as the breadcrumbs,
  right of the last crumb
- **AND** the read half of each of those sixteen pages carries the warning border

#### Scenario: A book with one disagreeing page

- **WHEN** one page of the book disagrees
- **THEN** the bar says "1 page disagrees"

#### Scenario: A book where nothing disagrees

- **WHEN** no page of the book disagrees
- **THEN** the bar says nothing about disagreement
- **AND** no page carries the border

#### Scenario: A book read with the normal profile

- **WHEN** a user opens the reader of a book whose every page holds one reading
- **THEN** the bar says nothing about disagreement

#### Scenario: A page that is marked and disagrees

- **WHEN** a user marks a disagreeing page for a rescan
- **THEN** the read half of that page carries the warning border and the success
  border, one inside the other
- **AND** both are visible

#### Scenario: A page carrying every reason at once

- **WHEN** a user marks a disagreeing page that holds an Ultra reading
- **THEN** the read half of that page carries three borders, one inside the other
- **AND** all three are visible, in three colours

#### Scenario: The border of a narrow window and a wide one

- **WHEN** a user reads a book holding a disagreeing page in a window 1280 pixels
  wide, and then in one 4000 pixels wide
- **THEN** the border shows on the left, the right, the top and the bottom of the
  read half in both

### Requirement: The reader shows which lines the merge filled from the line reading

The reader SHALL draw a line of a merged reading whose text came from the line
engine on a grey ground, where it draws the rest of the reading on white. Its text
SHALL stay the colour of every other line. The `book-ocr` capability says which
lines those are.

The text and not the ground is the page: such a line is as much the reading as any
other, so colouring its glyphs would read as a mark upon the text rather than as a
note about where it came from. The ground SHALL stay light enough to read black text
on.

A merged reading holds the text of its reading engine where the two engines read a
line alike, and the text of the line engine where they did not. The page looks
whole either way, so a reader who is not told reads the weaker transcription of
some lines without knowing it — on a page whose body came from the reading engine
and whose footnotes did not, the footnote Greek is the line engine's and wrong,
and nothing on the page says so.

The colour SHALL stay dark enough to read as body text. These lines are the page
and not an annotation of it, so the reader SHALL NOT mark them up, box them or
move them.

The reader SHALL draw every line of an engine result in black. A reading that is
no merge holds one engine's text throughout, and a page drawn wholly in brown
says nothing.

#### Scenario: A page whose footnotes came from the line engine

- **WHEN** a user reads a page whose merged reading took the body from
  PaddleOCR-VL and the footnotes from PP-OCRv6
- **THEN** the footnote lines stand on a grey ground
- **AND** the body lines stand on a white one
- **AND** every line of both is drawn in the same colour

#### Scenario: A page that answers its line reading

- **WHEN** a user chooses the line reading of a page
- **THEN** every line of that page stands on a white ground

### Requirement: The reader holds a strip naming the pages worth returning to

The reader SHALL hold a strip down the right edge of the window, inside it and so
left of the scrollbar of the browser, holding one mark per page that carries a
reason: a page that disagrees, a page marked for a rescan, or a page holding an
Ultra reading. A book runs to many pages and the reader scrolls through all of them,
so a reader who finds such a page means to come back to it.

A mark SHALL stand where the thumb of the scrollbar stands when its page is in the
window, so a reader scrolls to a mark by dragging the scrollbar to it.

That is not the share of the book the page sits at. The pages do not begin at the
top of the scrollable height — the bar and the controls stand above them — and the
thumb is as tall a share of its track as the window is of the document and travels
the track less its own height. A mark placed by the share of the page count alone
therefore sits above the thumb that reaches it, and a mark placed by the share of
the document agrees with the thumb in the middle of a book and misses it at both
ends by half a thumb.

A page carrying no reason SHALL get no mark. A mark for every page would be a second
scrollbar, and would say nothing.

A mark SHALL wear the colours its page wears, and for the same reasons: the colour of
success for a page a user chose, the colour of a warning for a reading the system
doubts, the colour of information for a page holding an Ultra reading.

A page carrying more than one reason SHALL show a band per reason within its mark,
sharing the mark between them in the order the borders of that page stand in. Every
band SHALL be visible, and a page carrying one reason SHALL wear that colour
throughout its mark.

Choosing a mark SHALL bring its page into view. The strip SHALL take no room from
the pages and SHALL move none of them.

The strip SHALL hold nothing at all while no page of the book carries a reason.

#### Scenario: A book holding doubted pages

- **WHEN** a user opens the reader of a book whose pages 6 and 60 disagree
- **THEN** the strip holds two marks in the colour of a warning
- **AND** the mark of page 60 stands below the mark of page 6

#### Scenario: A user marks a page

- **WHEN** a user marks a page for a rescan
- **THEN** the strip holds a mark for it in the colour of success

#### Scenario: A page that is marked and disagrees

- **WHEN** a user marks a page whose reading disagrees
- **THEN** its mark shows both colours, one inside the other

#### Scenario: A page holding an Ultra reading

- **WHEN** an Ultra rescan reads a page and the reader is opened again
- **THEN** the strip holds a mark for it in the colour of information

#### Scenario: A mark of a page carrying every reason

- **WHEN** a user marks a disagreeing page that holds an Ultra reading
- **THEN** its mark shows three bands, in three colours
- **AND** all three are visible

#### Scenario: A user chooses a mark

- **WHEN** a user chooses the mark of page 60
- **THEN** page 60 comes into view

#### Scenario: The marks of the first and the last page

- **WHEN** a book whose first and last page both disagree stands in the reader
- **THEN** the mark of the first page stands where the thumb stands at the top of
  the book, and the mark of the last where it stands at the foot
- **AND** neither sits against the edge of the strip

#### Scenario: A book with nothing to mark

- **WHEN** a user opens the reader of a book whose every page holds one reading
- **THEN** the strip holds no mark

### Requirement: The page menu says why a reading is doubted

The menu of a page whose reading disagrees SHALL say why, under the reading it
judges. A reader who sees the border asks what is wrong with the page, and the menu
is where the page answers.

It SHALL stand with the reading that answers and not at the foot of the menu, where
it would read as a note on the rescan below it. A page whose merged PaddleOCR-VL
reading answers therefore explains itself under "Extra" and above "Ultra".

It SHALL be short. Each reason SHALL name the fault and its number and nothing
else, because a menu is read at a glance: "119 unexpected Cyrillic characters",
"Contains only 5% of the text".

It SHALL wear the colour the system uses for a warning, which is the colour of the
border it explains, so the words and the mark on the page read as one thing. It
SHALL carry no heading: a reader who opened the menu of a bordered page needs no
sentence telling them the reading is doubted.

The text SHALL carry the left padding of an item of the menu, so it lines up with
the labels around it and not with the edge.

The explanation SHALL be read and not chosen. It SHALL answer no selection and
SHALL close nothing.

The menu of a page whose reading agrees SHALL say nothing about it. The
`page-reading-health` capability decides which pages disagree and holds the words.

#### Scenario: The menu of a page that turned Cyrillic

- **WHEN** a user opens the menu of a page whose reading holds 119 Cyrillic
  characters its line reading holds none of
- **THEN** the menu says "119 unexpected Cyrillic characters"
- **AND** it says it in the colour of a warning, under the reading that answers

#### Scenario: The menu of a page whose reading stopped early

- **WHEN** a user opens the menu of a page whose reading holds 5% of the text of
  its line reading
- **THEN** the menu says "Contains only 5% of the text"

#### Scenario: The explanation stands with the reading it judges

- **WHEN** a user opens the menu of a doubted page whose merged PaddleOCR-VL
  reading answers, and which also holds an Ultra reading
- **THEN** the explanation stands below "Extra (PaddleOCR-VL)" and above
  "Ultra (GLM-5.3 Flash)"

#### Scenario: The menu of a page that agrees

- **WHEN** a user opens the menu of a page whose reading agrees
- **THEN** the menu says nothing about a doubt

#### Scenario: The explanation answers no choice

- **WHEN** a user clicks the explanation
- **THEN** nothing runs and the menu stays open

### Requirement: The reader shows which pages hold an Ultra reading

The reader SHALL draw a border in the colour the system uses for information round
the read half of every page that holds a reading from GLM-5.3 Flash. A reading on a
metered API is paid for once, so a reader is told which pages already hold one and
can choose it without paying again.

It SHALL mark a page that holds such a reading, whether or not that reading is the
one the page answers. A reader who chose another reading of that page has not lost
the Ultra one.

It SHALL NOT mark a page that merely could be read by GLM-5.3 Flash. Every page of
a complete book could be, so such a border would mark every page and say nothing.

The border SHALL be drawn inside the box of the read half, as every other border of
a page is, and SHALL move no page. Where a page carries another reason as well, the
borders stand one inside the other and each stays visible.

#### Scenario: A page an Ultra rescan read

- **WHEN** an Ultra rescan reads page 12 and the reader is opened again
- **THEN** the read half of page 12 carries the border of information
- **AND** no other page carries it

#### Scenario: A page that answers another of its readings

- **WHEN** a reader chooses the line reading of a page that holds an Ultra reading
- **THEN** that page still carries the border of information

#### Scenario: A page no Ultra rescan has read

- **WHEN** a user opens the reader of a book whose pages hold the line reading and
  the merged PaddleOCR-VL reading
- **THEN** no page carries the border of information

### Requirement: The page grid marks the pages that disagree

The grid of pages of a book SHALL name the number of pages that disagree, in the
same block as its breadcrumbs and immediately after the last of them, in the same
words the reader uses. The grid is where a reader arrives before the reader, so it
says the same thing in the same place.

The grid of pages of a book SHALL draw a border in the colour the system uses for
a warning round the thumbnail of every page whose reading disagrees. A reader
opens the grid to find a page, so the grid says which pages are worth opening.

It SHALL mark nothing for a book whose pages hold one reading each, and nothing
for a book that has not been read.

The border SHALL move no thumbnail and SHALL NOT change the size of the cell it
sits in. The `page-reading-health` capability decides which pages disagree, and it
decides the same pages here as in the reader.

#### Scenario: A grid of a book holding disagreeing pages

- **WHEN** a user opens the grid of a book whose pages 6 and 18 disagree
- **THEN** the bar says "2 pages disagree" in the same block as the breadcrumbs
- **AND** the thumbnails of pages 6 and 18 carry the warning border
- **AND** no other thumbnail carries one
- **AND** every cell keeps the size it had

#### Scenario: A grid of a book read with the normal profile

- **WHEN** a user opens the grid of a book whose every page holds one reading
- **THEN** no thumbnail carries the border
- **AND** the bar says nothing about disagreement

#### Scenario: A grid of a book that has not been read

- **WHEN** a user opens the grid of a book in the `split` stage
- **THEN** no thumbnail carries the border
