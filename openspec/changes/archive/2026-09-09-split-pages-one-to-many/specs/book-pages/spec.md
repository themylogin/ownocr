## MODIFIED Requirements

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
