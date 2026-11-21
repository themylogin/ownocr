## ADDED Requirements

### Requirement: The rescan asks for a model before it runs

The interface SHALL ask for the model a rescan reads with, in the dialog that
starts the rescan. The rescan SHALL NOT start before the user answers the dialog.

The dialog SHALL keep its own title, which names the rescan and not the OCR
action. It SHALL say what a rescan does and why a user wants one. Its starting
control SHALL be named "Rescan".

The dialog SHALL offer the two choices the OCR dialog offers, in the same order
and the same words:

- "Normal", subtitled "PP-OCRv6", described as the frontier OCR model.
- "Extra", subtitled "PaddleOCR-VL", described as the visual model, for
  difficult cases like polytonic Greek.

The dialog SHALL preselect "Extra", where the OCR dialog preselects "Normal". A
user rescans a page because the normal model read it badly, so the extra model is
the common answer here.

Cancelling, pressing Escape, or clicking away SHALL close the dialog, start
nothing, and leave every mark standing.

Starting SHALL run the rescan of every marked page with the chosen profile. A
refusal from the server SHALL appear in the dialog, and the dialog SHALL stay
open with every mark standing.

#### Scenario: A user opens the dialog

- **WHEN** a user marks two pages and activates the rescan control
- **THEN** the dialog opens holding "Normal" and "Extra" in that order
- **AND** "Extra" is selected
- **AND** no rescan has started

#### Scenario: A user starts the extra profile

- **WHEN** a user opens the dialog and starts it without changing the choice
- **THEN** the system rescans the marked pages with the profile `vl`
- **AND** the dialog closes

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

## MODIFIED Requirements

### Requirement: The right half of the OCR reader offers the rescan alone

A right click on the right half of the OCR reader SHALL open a menu holding
"Rescan" and nothing else, when that page offers the rescan. It SHALL NOT open
the page menu, and it SHALL NOT open the menu of the browser.

The right half stacks an image, a white veil and drawn text, so a copy saved
from it would hold the page without one glyph of the reading. Every item that
reaches the image therefore stays on the left half. The rescan reaches neither
the image nor the text, which is why it alone belongs here: a user who finds a
badly read page reads it on this half.

Every page of a complete book in the `ocr` stage offers the rescan, whatever
results that page holds. A right click on the right half of a page of a book that
offers no rescan SHALL open no menu. An empty menu says less than no menu.

A right click on the left half SHALL open the page menu, as a right click on any
other page does. A right click on the gap between the two halves SHALL open no
menu.

The drawn text SHALL stay selectable, and a user SHALL still copy a selection by
keyboard.

#### Scenario: A user right clicks the read half

- **WHEN** a user right clicks the right half of a page in the reader of a
  complete book whose profile is `v6`
- **THEN** a menu opens at the pointer holding "Rescan"
- **AND** it holds no other item
- **AND** no browser menu opens

#### Scenario: A user right clicks the read half of an already rescanned page

- **WHEN** a user right clicks the right half of a page that holds a
  PaddleOCR-VL result
- **THEN** a menu opens at the pointer holding "Rescan"
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

The menu SHALL then hold "Rescan" when that page offers a rescan, then a divider.
The rescan stands in a group of its own, after the two items that reach the image
and before the items that change the page. It changes no page: it reads a page
the system already holds. The item SHALL name no engine, because the dialog it
opens asks for one.

The rescan is the one item the reader offers and the grid does not. A rescan is
marked, counted and started in the reader, because that is where a user finds a
page the engine misread. The grid shows a page too small to read. The menu of a
page in the grid therefore SHALL hold every other item and SHALL NOT hold the
rescan.

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
- **THEN** the menu opens holding "Open in new tab", "Download", a divider,
  "Rescan", a divider, then "Delete"
- **AND** it holds no "Edit" and no "Show original"

#### Scenario: A user right clicks an already rescanned page

- **WHEN** a user right clicks the left half of a page that holds a PaddleOCR-VL
  result
- **THEN** the menu opens holding "Open in new tab", "Download", a divider,
  "Rescan", a divider, then "Delete"

#### Scenario: A user right clicks a page of a book read with the extra profile

- **WHEN** a user right clicks the left half of a page in the reader of a
  complete book whose profile is `vl`
- **THEN** the menu holds "Rescan"

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
- **AND** the menu of that page in the reader does hold it

#### Scenario: A user dismisses the menu

- **WHEN** the page menu is open and the user presses Escape or clicks away
- **THEN** the menu closes
- **AND** nothing is deleted, nothing is rolled back, nothing is cut and no
  rescan starts
