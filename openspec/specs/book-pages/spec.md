## Purpose

Defines what a user can do to one page instead of one book: remove a page from
a book and every book derived from it, return one page of a copy to the page it
was made from, and cut one page of a copy out of its parent again. Defines the
page lineage every one of them follows.

## Requirements

### Requirement: A page of a copy names the page it came from

Every page a copy holds SHALL name the page of the source book that it was made
from. A page of a book that has no source book SHALL name no source page. The
system SHALL keep that reference for the life of the page, and SHALL NOT derive
the reference from the page index.

A page whose source page no longer exists SHALL name no source page. The page
itself SHALL survive, because a copy owns its own stored renditions.

#### Scenario: A dewarped book records its lineage

- **WHEN** the Dewarp action completes for a book of forty pages
- **THEN** each page of the copy names exactly one page of the source book
- **AND** no two pages of the copy name the same source page

#### Scenario: An uploaded book has no lineage

- **WHEN** a user uploads a book
- **THEN** no page of that book names a source page

#### Scenario: A retry records the lineage again

- **WHEN** a failed copy is retried and completes
- **THEN** every page of the copy names its source page

### Requirement: A page index never changes

The system SHALL give a page its index when it writes the page, and SHALL never
change that index afterwards. Removing a page SHALL leave the index of every
other page untouched, so the indexes of a book may hold gaps. The system SHALL
order the pages of a book by index, ascending.

The index SHALL NOT appear in the interface. It identifies a page inside its
book; it does not number the pages for a reader.

#### Scenario: A page is removed from the middle

- **WHEN** a user removes the page at index 2 of a book with indexes 1, 2, 3, 4
- **THEN** the book holds the pages at indexes 1, 3 and 4
- **AND** the system renumbers nothing

#### Scenario: The order survives a gap

- **WHEN** a client reads a book whose indexes hold a gap
- **THEN** the pages arrive in ascending index order
- **AND** the grid shows one cell per page that is left

### Requirement: Deleting a page removes it from the book and from every book derived from it

The system SHALL remove a named page from its book, and SHALL remove every page
made from it in every book derived from that book, however deep the chain runs.
One source page may have made several pages in one book. The system SHALL remove
all of them.

It SHALL remove the stored renditions of every page it removes. It SHALL reduce
the page count of every book it changed, and SHALL announce every changed book
to connected clients. It SHALL name each changed book once, however many pages
that book lost.

The system SHALL NOT remove the page of any book the named book was made from.
A delete travels down the chain alone.

The system SHALL remove the page row before its files. A file left behind is
waste; a row that names missing files is a broken page.

#### Scenario: A user deletes a page of a source book

- **WHEN** a user deletes a page of an uploaded book that a dewarped copy was
  made from
- **THEN** that page is gone from the uploaded book
- **AND** the page made from it is gone from the dewarped copy
- **AND** both books report one page fewer

#### Scenario: A user deletes a page of a copy

- **WHEN** a user deletes a page of a dewarped copy
- **THEN** that page is gone from the copy
- **AND** the source book still holds every page it had

#### Scenario: A chain of three books

- **WHEN** a user deletes a page of a book that a dewarped copy and a split copy
  were made from
- **THEN** the page is gone from all three books

#### Scenario: A copy lost several pages from one

- **WHEN** a user deletes a page of a book whose split copy made two pages from
  it
- **THEN** both pages are gone from the split copy
- **AND** the split copy reports two pages fewer
- **AND** the answer names the split copy once

#### Scenario: The stored files are gone

- **WHEN** a delete removes a page
- **THEN** the three renditions of that page are gone from the file store

#### Scenario: A user deletes the last page

- **WHEN** a user deletes the only page a book holds
- **THEN** the book still exists and reports no pages
- **AND** its card shows no cover image

#### Scenario: The page does not exist

- **WHEN** a user deletes a page index that the book does not hold
- **THEN** the system answers `404 Not Found`
- **AND** it removes nothing

### Requirement: The system answers what a page delete would remove

The system SHALL answer, before it removes anything, with every book that would
lose a page. The answer SHALL name each book and its stage, in chain order. It
SHALL name a book once, however many pages that book would lose. It SHALL also
report whether the delete is allowed.

A book that would lose several pages is still one book. Naming it once is what
lets the interface count the books it lists.

#### Scenario: A client asks what a delete would remove

- **WHEN** a client asks what deleting a page of a book with a dewarped copy
  would remove
- **THEN** the answer names both books, each with its stage

#### Scenario: A client asks about a book with no copy

- **WHEN** a client asks what deleting a page of a book with no copy would
  remove
- **THEN** the answer names that book alone

#### Scenario: A copy would lose several pages from one

- **WHEN** a client asks what deleting a page of a book with a split copy would
  remove
- **THEN** the answer names two books
- **AND** the interface says the delete removes the page from two books

### Requirement: The system refuses a page operation while the chain holds work in flight

The system SHALL refuse to delete a page, SHALL refuse to roll a page back,
SHALL refuse to cut a page again, and SHALL refuse to rescan a page, while the
book or any book derived from it is in state `pending` or `processing`. It SHALL
name the reason.

A running job reads the pages of a source book and writes the pages of a copy.
Changing either set under the job would make the copy disagree with its source.

A rescan adds stored results to pages of the book it runs on. It SHALL also
refuse while its own book is in any state other than `complete`, so two rescans
of one book never run at once and a rescan never runs against a book a job is
still building.

A failed book SHALL NOT block the delete, the rollback or the re-cut. A retry
rebuilds the copy from the source book as it stands. A failed book SHALL block
the rescan of its own pages, because a rescan adds to a reading that is not
finished.

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

#### Scenario: A rescan already runs

- **WHEN** a user asks to rescan a page of a book whose rescan is in state
  `processing`
- **THEN** the system rejects the request and names the reason
- **AND** the running rescan is unaffected

#### Scenario: A page operation during a rescan

- **WHEN** a user deletes a page of a book whose rescan is in state `processing`
- **THEN** the system rejects the request and names the reason
- **AND** it removes nothing

#### Scenario: The reading of the book is not finished

- **WHEN** a user asks to rescan a page of a book in state `failed`
- **THEN** the system rejects the request and names the reason

### Requirement: A page of a copy can return to the page it came from

The system SHALL let a user replace one page of a copy with the page it was made
from. The system SHALL offer this only when all of the following hold:

- the book names the action that made it
- that action makes exactly one copy page from one source page
- the page names a source page that still exists
- the page does not already show that source page
- the book has no copy of its own
- no book of the chain is in state `pending` or `processing`

The system SHALL build the renditions of the page again from the original bytes
of the source page, and SHALL store them under a storage key of its own book. It
SHALL remove the renditions it replaced. The page SHALL keep its index and its
source page reference. The book SHALL keep its page count.

The system SHALL record that the page returned to its source, and SHALL report
that with the page. A page that carries the mark SHALL offer the rollback no
more: it already shows the page it would install. A worker that rebuilds a copy
writes new pages, which carry no mark.

The step SHALL NOT be reversible. Getting the replaced page back means running
the action for the whole book again.

#### Scenario: A user rolls a dewarped page back

- **WHEN** a user rolls back a page of a dewarped copy
- **THEN** that page shows the bytes of the uploaded page it was made from
- **AND** the page keeps its index
- **AND** the book reports the same page count

#### Scenario: The copy keeps its own files

- **WHEN** a rollback completes
- **THEN** the new renditions sit under the storage key of the copy
- **AND** the source book keeps every one of its files
- **AND** rejecting the source book afterwards leaves the page readable

#### Scenario: The replaced files are gone

- **WHEN** a rollback completes
- **THEN** the three renditions the page held before are gone from the file
  store

#### Scenario: The book already has a copy

- **WHEN** a user rolls back a page of a book that a split copy was made from
- **THEN** the system rejects the request and names the reason

#### Scenario: The book has no source

- **WHEN** a user rolls back a page of an uploaded book
- **THEN** the system rejects the request and names the reason

#### Scenario: The action makes two pages from one

- **WHEN** a user rolls back a page of a book that the Split action made
- **THEN** the system rejects the request and names the reason

#### Scenario: The source page is gone

- **WHEN** a user rolls back a page that names no source page
- **THEN** the system rejects the request and names the reason

#### Scenario: The page already shows the original

- **WHEN** a user rolls back a page that was rolled back before
- **THEN** the system rejects the request and names the reason
- **AND** the page reports that it returned to its source

#### Scenario: A retry clears the mark

- **WHEN** a worker rebuilds a copy whose page was rolled back
- **THEN** no page of the copy reports that it returned to its source
- **AND** every page offers the rollback again

### Requirement: The system answers the two pages a rollback compares

The system SHALL answer with the page a rollback would install and the page it
would replace. Each SHALL carry its display rendition, its stored width and
height, and the title of the stage its book belongs to.

#### Scenario: A client asks for the comparison

- **WHEN** a client asks what rolling back a page of a dewarped copy would
  compare
- **THEN** the answer holds the uploaded page and the dewarped page
- **AND** each carries a display rendition, a width, a height and a stage title

#### Scenario: The page offers no rollback

- **WHEN** a client asks for the comparison of a page that offers no rollback
- **THEN** the system rejects the request and names the reason

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
