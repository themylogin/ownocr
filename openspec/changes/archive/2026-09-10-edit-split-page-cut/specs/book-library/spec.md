## ADDED Requirements

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

## MODIFIED Requirements

### Requirement: A right click on a page offers the page menu

A right click on a page SHALL open a context menu at the pointer. The page grid
and the reader SHALL offer the same menu.

The menu SHALL hold "Edit" when that page offers a re-cut, then "Show original"
when that page offers a rollback, then a divider, then a "Delete" item. No page
offers both, because an action makes either one page from one page or two. The
menu SHALL decide per page, not per book: a page that already shows its
original, that names no source page, or that records no cut, SHALL NOT hold its
item while its neighbours do. The system SHALL show "Delete" in the colour it
uses for danger. A page that offers neither SHALL open a menu holding "Delete"
alone, so every page can be deleted. Pressing Escape or clicking away SHALL
close the menu and run nothing.

The system SHALL take the offer of a rollback and the offer of a re-cut from the
server, so a new action needs no client change.

#### Scenario: A user right clicks a dewarped page

- **WHEN** a user right clicks a page in the grid of a dewarped copy that has no
  copy of its own
- **THEN** a context menu opens at the pointer
- **AND** it holds "Show original", a divider, then "Delete"
- **AND** "Delete" is not the colour of the other items

#### Scenario: A user right clicks a page cut out of a spread

- **WHEN** a user right clicks a page of a split copy that was cut out of a
  spread, and that copy has no copy of its own
- **THEN** the menu opens holding "Edit", a divider, then "Delete"
- **AND** "Edit" is the first item

#### Scenario: A user right clicks a page the split copied whole

- **WHEN** a user right clicks a page of a split copy that records no cut
- **THEN** the menu opens holding "Delete" alone
- **AND** the menu of a page of that book that was cut still holds "Edit"

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
- **AND** nothing is deleted, nothing is rolled back and nothing is cut
