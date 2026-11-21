## RENAMED Requirements

- FROM: `### Requirement: The right half of the OCR reader offers no menu`
- TO: `### Requirement: The right half of the OCR reader offers the rescan alone`

## MODIFIED Requirements

### Requirement: The right half of the OCR reader offers the rescan alone

A right click on the right half of the OCR reader SHALL open a menu holding
"Rescan with Extra effort" and nothing else, when that page offers the rescan.
It SHALL NOT open the page menu, and it SHALL NOT open the menu of the browser.

The right half stacks an image, a white veil and drawn text, so a copy saved
from it would hold the page without one glyph of the reading. Every item that
reaches the image therefore stays on the left half. The rescan reaches neither
the image nor the text, which is why it alone belongs here: a user who finds a
badly read page reads it on this half.

A right click on the right half of a page that offers no rescan SHALL open no
menu. An empty menu says less than no menu.

A right click on the left half SHALL open the page menu, as a right click on any
other page does. A right click on the gap between the two halves SHALL open no
menu.

The drawn text SHALL stay selectable, and a user SHALL still copy a selection by
keyboard.

#### Scenario: A user right clicks the read half

- **WHEN** a user right clicks the right half of a page in the reader of a
  complete book whose profile is `v6`
- **THEN** a menu opens at the pointer holding "Rescan with Extra effort"
- **AND** it holds no other item
- **AND** no browser menu opens

#### Scenario: A user right clicks the read half of an already rescanned page

- **WHEN** a user right clicks the right half of a page that holds a
  PaddleOCR-VL result
- **THEN** no menu opens
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

### Requirement: A right click on a page offers the page menu

A right click on a page SHALL open a context menu at the pointer. The page grid
and the reader SHALL offer the same menu, but for the rescan named below, which
the reader alone offers.

The menu SHALL hold "Open in new tab" first and "Download" second, then a
divider. Both items SHALL appear for every page, so a page always opens a menu
holding at least those two and "Delete".

The menu SHALL then hold "Rescan with Extra effort" when that page offers a
rescan, then a divider. The rescan stands in a group of its own, after the two
items that reach the image and before the items that change the page. It changes
no page: it adds a second reading of one the system already holds.

The rescan is the one item the reader offers and the grid does not. A rescan is
marked, counted and started in the reader, because that is where a user finds a
page the engine misread. The grid shows a page too small to read. The menu of a
page in the grid therefore SHALL hold every other item and SHALL NOT hold the
rescan.

The menu SHALL then hold "Edit" when that page offers a re-cut, then "Show
original" when that page offers a rollback, then a divider, then a "Delete"
item. No page offers both, because an action makes either one page from one page
or two. The menu SHALL decide per page, not per book: a page that already shows
its original, that names no source page, that records no cut, or that the visual
engine has already read, SHALL NOT hold its item while its neighbours do. The
system SHALL show "Delete" in the colour it uses for danger. A page that offers
none of the three SHALL open a menu holding "Open in new tab", "Download", a
divider and "Delete". Pressing Escape or clicking away SHALL close the menu and
run nothing.

The system SHALL take the offer of a rollback, the offer of a re-cut and the
offer of a rescan from the server, so a new action needs no client change.

#### Scenario: A user right clicks a page of a book read with the normal profile

- **WHEN** a user right clicks the left half of a page of a complete book whose
  profile is `v6`
- **THEN** the menu opens holding "Open in new tab", "Download", a divider,
  "Rescan with Extra effort", a divider, then "Delete"
- **AND** it holds no "Edit" and no "Show original"

#### Scenario: A user right clicks an already rescanned page

- **WHEN** a user right clicks the left half of a page that holds a PaddleOCR-VL
  result
- **THEN** the menu opens holding "Open in new tab", "Download", a divider, then
  "Delete"
- **AND** it holds no "Rescan with Extra effort"
- **AND** the menu of a page of that book that holds no such result still holds
  it

#### Scenario: A user right clicks a dewarped page

- **WHEN** a user right clicks a page in the grid of a dewarped copy that has no
  copy of its own
- **THEN** a context menu opens at the pointer
- **AND** it holds "Open in new tab", "Download", a divider, "Show original", a
  divider, then "Delete"
- **AND** it holds no "Rescan with Extra effort"
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
- **AND** it holds no "Rescan with Extra effort"
- **AND** the menu of that page in the reader does hold it

#### Scenario: A user dismisses the menu

- **WHEN** the page menu is open and the user presses Escape or clicks away
- **THEN** the menu closes
- **AND** nothing is deleted, nothing is rolled back, nothing is cut and no
  rescan starts
