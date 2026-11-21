## Purpose

Defines what a user can do to one page instead of one book: remove a page from
a book and every book derived from it, and return one page of a copy to the
page it was made from. Defines the page lineage both operations follow.

## ADDED Requirements

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

The system SHALL remove a named page from its book, and SHALL remove the page
made from it in every book derived from that book, however deep the chain runs.
It SHALL remove the stored renditions of every page it removes. It SHALL reduce
the page count of every book it changed, and SHALL announce every changed book
to connected clients.

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
SHALL also report whether the delete is allowed.

#### Scenario: A client asks what a delete would remove

- **WHEN** a client asks what deleting a page of a book with a dewarped copy
  would remove
- **THEN** the answer names both books, each with its stage

#### Scenario: A client asks about a book with no copy

- **WHEN** a client asks what deleting a page of a book with no copy would
  remove
- **THEN** the answer names that book alone

### Requirement: The system refuses a page operation while the chain holds work in flight

The system SHALL refuse to delete a page, and SHALL refuse to roll a page back,
while the book or any book derived from it is in state `pending` or
`processing`. It SHALL name the reason.

A running job reads the pages of a source book and writes the pages of a copy.
Changing either set under the job would make the copy disagree with its source.

A failed book SHALL NOT block either operation. A retry rebuilds the copy from
the source book as it stands.

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
