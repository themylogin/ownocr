## Purpose

Lets a user browse books by processing stage, see one book's pages as a regular
grid, and read those pages full width in order.

## ADDED Requirements

### Requirement: Stages appear as tabs

The system SHALL show the processing stages as an ordered row of tabs at the top
of the screen. Selecting a tab SHALL show the books pending that stage. The
system SHALL define the stage list on the server, so a new stage needs no
client change.

#### Scenario: The app opens

- **WHEN** a user opens the app
- **THEN** the tab row shows one tab named "Dewarp"
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
thumbnail of the book's first page.

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

### Requirement: Books that are not complete appear apart from the stage list

The system SHALL show books in state `pending`, `processing`, or `failed` in a
separate area above the stage list. A `failed` book SHALL show its reason.

#### Scenario: A book is ingesting

- **WHEN** a user uploads a book and stays on the tab
- **THEN** the book appears in the separate area with its state
- **AND** it does not appear in the stage list

#### Scenario: Ingest completes while the user watches

- **WHEN** the book reaches state `complete`
- **THEN** it leaves the separate area
- **AND** it appears in the Dewarp list without a page reload

#### Scenario: Ingest fails

- **WHEN** a book reaches state `failed`
- **THEN** it stays in the separate area
- **AND** the interface shows its failure reason

### Requirement: Opening a book shows its pages in a regular grid

Selecting a book SHALL show every page of that book as a thumbnail in a grid.
Every grid cell SHALL have the same size.

#### Scenario: A user opens a book

- **WHEN** a user selects a book with 120 pages
- **THEN** the system shows 120 thumbnails in page order
- **AND** every cell has identical width and height

#### Scenario: A page is smaller than its cell

- **WHEN** a page's thumbnail is smaller than the cell
- **THEN** the thumbnail is centered in the cell
- **AND** the thumbnail is not cropped and not enlarged

### Requirement: The grid cell size derives from the page dimensions

The system SHALL compute one cell size for the whole book from the dimensions of
all its pages. The longest cell side SHALL be 480 px. The system SHALL use the
median page aspect ratio, where aspect ratio is width divided by height.

#### Scenario: Every page is landscape

- **WHEN** every page of a book is wider than it is tall with median aspect 1.5
- **THEN** the cell is 480 x 320 px

#### Scenario: Every page is portrait

- **WHEN** every page of a book is taller than it is wide with median aspect 0.667
- **THEN** the cell is 320 x 480 px

#### Scenario: Pages mix orientations

- **WHEN** a book holds both landscape and portrait pages
- **THEN** the cell is 480 x 480 px

#### Scenario: A page is square

- **WHEN** any page of a book is exactly square
- **THEN** the cell is 480 x 480 px

#### Scenario: Pages are extremely wide

- **WHEN** every page is landscape with median aspect 6.0
- **THEN** the cell short side is clamped to 240 px
- **AND** the cell is 480 x 240 px

#### Scenario: The book has one page

- **WHEN** a book holds a single page of 2000 x 3000 pixels
- **THEN** the cell is 320 x 480 px

### Requirement: Selecting a page opens the reader at that page

Selecting a thumbnail SHALL open the reader view scrolled to that page.

#### Scenario: A user selects page 42

- **WHEN** a user selects the thumbnail of page 42
- **THEN** the reader opens
- **AND** page 42 is in view

#### Scenario: The reader URL is shared

- **WHEN** a user opens a reader URL for a specific page directly
- **THEN** the reader opens scrolled to that page

### Requirement: The reader shows pages full width in order

The reader SHALL show every page of the book stacked vertically, in page order,
each spanning the full width of the screen. The reader SHALL use the 1280 px
display rendition.

#### Scenario: A user scrolls the reader

- **WHEN** a user scrolls down in the reader
- **THEN** the next page follows directly below the current page
- **AND** pages stay in page order

#### Scenario: A large book opens

- **WHEN** a user opens the reader for a book of 500 pages
- **THEN** the view appears without loading all 500 images at once
- **AND** images load as they approach the viewport

#### Scenario: A user leaves the reader

- **WHEN** a user activates the back control in the reader
- **THEN** the system returns to the page grid of the same book

### Requirement: Book lists update live

The system SHALL update the visible book lists from server events without a page
reload.

#### Scenario: Another user uploads a book

- **WHEN** a second user uploads a book and its ingest completes
- **THEN** the first user's Dewarp tab shows the new book without a reload

#### Scenario: The connection drops

- **WHEN** the event connection closes
- **THEN** the interface shows the connection state
- **AND** it reconnects and refreshes the visible list
