## ADDED Requirements

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

### Requirement: The right half of the OCR reader offers no menu

A right click on the right half of the OCR reader SHALL open no menu. It SHALL
open neither the page menu nor the menu of the browser. The right half stacks an
image, a white veil and drawn text, so a copy saved from it would hold the page
without one glyph of the reading.

A right click on the left half SHALL open the page menu, as a right click on any
other page does. A right click on the gap between the two halves SHALL open no
menu.

The drawn text SHALL stay selectable, and a user SHALL still copy a selection by
keyboard.

#### Scenario: A user right clicks the read half

- **WHEN** a user right clicks the right half of a page in the reader of a book
  in the `ocr` stage
- **THEN** no page menu opens
- **AND** no browser menu opens

#### Scenario: A user right clicks the page half

- **WHEN** a user right clicks the left half of a page in the reader of a book in
  the `ocr` stage
- **THEN** the page menu opens at the pointer

#### Scenario: A user copies the text of a read page

- **WHEN** a user selects the drawn text of a line and copies it by keyboard
- **THEN** the clipboard holds that text

#### Scenario: The reader of another stage keeps its menu

- **WHEN** a user right clicks a page in the reader of a book in the `split`
  stage
- **THEN** the page menu opens at the pointer

## MODIFIED Requirements

### Requirement: A right click on a page offers the page menu

A right click on a page SHALL open a context menu at the pointer. The page grid
and the reader SHALL offer the same menu.

The menu SHALL hold "Open in new tab" first and "Download" second, then a
divider. Both items SHALL appear for every page, so a page always opens a menu
holding at least those two and "Delete".

The menu SHALL then hold "Edit" when that page offers a re-cut, then "Show
original" when that page offers a rollback, then a divider, then a "Delete"
item. No page offers both, because an action makes either one page from one page
or two. The menu SHALL decide per page, not per book: a page that already shows
its original, that names no source page, or that records no cut, SHALL NOT hold
its item while its neighbours do. The system SHALL show "Delete" in the colour it
uses for danger. A page that offers neither SHALL open a menu holding "Open in
new tab", "Download", a divider and "Delete". Pressing Escape or clicking away
SHALL close the menu and run nothing.

The system SHALL take the offer of a rollback and the offer of a re-cut from the
server, so a new action needs no client change.

#### Scenario: A user right clicks a dewarped page

- **WHEN** a user right clicks a page in the grid of a dewarped copy that has no
  copy of its own
- **THEN** a context menu opens at the pointer
- **AND** it holds "Open in new tab", "Download", a divider, "Show original", a
  divider, then "Delete"
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

#### Scenario: A user dismisses the menu

- **WHEN** the page menu is open and the user presses Escape or clicks away
- **THEN** the menu closes
- **AND** nothing is deleted, nothing is rolled back and nothing is cut
