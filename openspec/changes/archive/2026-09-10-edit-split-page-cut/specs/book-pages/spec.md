## ADDED Requirements

### Requirement: A page cut out of a spread records its cut

A page that an action cut out of a spread SHALL record which side of the gutter
it holds and the column it was cut at. The column SHALL count in the columns of
the stored image of the source page.

The page on the left SHALL hold every column before the recorded column. The
page on the right SHALL hold the recorded column and every column after it. The
two pages of one spread SHALL record their own columns, so the columns may
overlap.

A page that reached its book whole SHALL record no cut. The system SHALL report
the recorded side with every page, and SHALL report no side for a page that
records no cut.

#### Scenario: A cut page records its side

- **WHEN** a split cuts a spread into two pages
- **THEN** the first page records the left side
- **AND** the second page records the right side

#### Scenario: A whole page records no cut

- **WHEN** a split copies a page whole because it holds no spread
- **THEN** that page records no cut
- **AND** the system reports no side for it

#### Scenario: The recorded column matches the stored image

- **WHEN** a page records the left side and the column 1500
- **THEN** the stored image of that page is 1500 columns wide

### Requirement: A page cut out of a spread can be cut again

The system SHALL let a user cut one page out of the page it was made from
again, at a column the user names. The system SHALL offer this only when all of
the following hold:

- the page records a cut
- the page names a source page that still exists
- the book has no copy of its own
- no book of the chain is in state `pending` or `processing`

The system SHALL take the column in the columns of the stored image of the
source page. It SHALL refuse a column that leaves the page with no column.

The system SHALL cut the named page out of the original bytes of the source
page, on the side that page records. It SHALL build the renditions of the page
again, and SHALL store them under a storage key of its own book. It SHALL
remove the renditions it replaced. It SHALL record the new column and the new
dimensions of the page.

The page SHALL keep its index, its source page reference and its side. The book
SHALL keep its page count. The system SHALL announce the book to connected
clients.

The system SHALL re-cut the named page alone. The other page of the spread
SHALL keep the cut it holds, so the two pages may overlap or may leave a strip
of the spread in neither page.

The step SHALL NOT be reversible. The system SHALL cut every attempt out of the
source page, never out of the page it replaces, so a run of edits loses no
column.

#### Scenario: A user moves the cut of a left page

- **WHEN** a user re-cuts a left page at a column 200 columns further right
- **THEN** the stored image of that page grows by 200 columns
- **AND** the page keeps its index
- **AND** the book reports the same page count

#### Scenario: A user moves the cut of a right page

- **WHEN** a user re-cuts a right page at a column 200 columns further left
- **THEN** that page holds 200 columns more of the spread on its inner edge
- **AND** the left page of that spread is unchanged

#### Scenario: The copy keeps its own files

- **WHEN** a re-cut completes
- **THEN** the new renditions sit under the storage key of the copy
- **AND** the source book keeps every one of its files
- **AND** the renditions the page held before are gone from the file store

#### Scenario: A user widens a page it narrowed before

- **WHEN** a user re-cuts a page, then re-cuts it again further out
- **THEN** the page holds every column the second cut names
- **AND** no column of the first cut is missing

#### Scenario: The page records no cut

- **WHEN** a user re-cuts a page that reached its book whole
- **THEN** the system rejects the request and names the reason

#### Scenario: The book already has a copy

- **WHEN** a user re-cuts a page of a book that a copy was made from
- **THEN** the system rejects the request and names the reason

#### Scenario: The source page is gone

- **WHEN** a user re-cuts a page whose source page was deleted
- **THEN** the system rejects the request and names the reason

#### Scenario: The column leaves no page

- **WHEN** a user re-cuts a left page at the column 0
- **THEN** the system rejects the request and names the reason
- **AND** the page is unchanged

#### Scenario: The column lies outside the image

- **WHEN** a user names a column past the width of the source page
- **THEN** the system rejects the request and names the reason

### Requirement: The system answers the parent image and the current cut

The system SHALL answer, for a page that offers a re-cut, with the page it was
cut out of and the cut it holds now. The answer SHALL carry the display
rendition of the source page, its stored width and height, the side the page
holds, and the current column.

The width SHALL be the width of the stored image the column counts in, so a
client can place the column on the display rendition itself.

#### Scenario: A client asks for the cut

- **WHEN** a client asks for the cut of a page of a split copy
- **THEN** the answer holds the display rendition of the source page
- **AND** it holds the width and the height of the source page
- **AND** it holds the side and the current column

#### Scenario: The page offers no re-cut

- **WHEN** a client asks for the cut of a page that offers none
- **THEN** the system rejects the request and names the reason

## MODIFIED Requirements

### Requirement: The system refuses a page operation while the chain holds work in flight

The system SHALL refuse to delete a page, SHALL refuse to roll a page back, and
SHALL refuse to cut a page again, while the book or any book derived from it is
in state `pending` or `processing`. It SHALL name the reason.

A running job reads the pages of a source book and writes the pages of a copy.
Changing either set under the job would make the copy disagree with its source.

A failed book SHALL NOT block any of the three operations. A retry rebuilds the
copy from the source book as it stands.

#### Scenario: A copy is running

- **WHEN** a user deletes a page of a book whose dewarp copy is in state
  `processing`
- **THEN** the system rejects the request and names the reason
- **AND** it removes nothing

#### Scenario: The book itself is running

- **WHEN** a user deletes a page of a book that is in state `processing`
- **THEN** the system rejects the request and names the reason

#### Scenario: A copy failed

- **WHEN** a user deletes a page of a book whose dewarp copy is in state
  `failed`
- **THEN** the system removes the page from both books

#### Scenario: The chain is complete

- **WHEN** every book of the chain is in state `complete`
- **THEN** the system carries out the request

#### Scenario: The book of the page is running

- **WHEN** a user re-cuts a page of a split copy that is in state `processing`
- **THEN** the system rejects the request and names the reason
- **AND** the page is unchanged
