## ADDED Requirements

### Requirement: The API answers the read lines of a whole book

The system SHALL answer the read lines of every page of a book, on request by
book. The answer SHALL hold one entry per page, in page order. Every entry SHALL
name its page index and SHALL carry the same content the answer for one page
carries: the model the lines come from, the pixel size the boxes count in, and
one entry per line with the box and the text of that line.

Every rule the answer for one page follows SHALL hold for every page of this
answer. The model chosen, the order of the lines, and the text of a line SHALL
NOT differ between the two answers. A client that reads one page from this answer
SHALL get what the answer for that page alone would give.

A page that holds no result SHALL appear in the answer with no lines. The whole
book is one request, so one unread page SHALL NOT fail it. A running OCR copy
holds unread pages, and a reader of that copy SHALL still get the pages it has
read.

The system SHALL refuse the request for a book that does not exist.

#### Scenario: A client asks for a whole book

- **WHEN** a client asks for the lines of a book of 12 read pages
- **THEN** the answer holds 12 entries
- **AND** they carry the page indexes 1 through 12 in that order
- **AND** each holds the model, the page size, and the lines of that page

#### Scenario: The whole book agrees with one page

- **WHEN** a client asks for the lines of a book, and then for the lines of page
  3 of that book
- **THEN** the model of page 3 is the same in both answers
- **AND** the lines of page 3 are the same, in the same order, character for
  character

#### Scenario: A copy is still running

- **WHEN** a client asks for the lines of a copy that has read 5 of its 12 pages
- **THEN** the answer holds an entry for all 12 pages
- **AND** the 5 read pages hold their lines
- **AND** the 7 unread pages hold no lines
- **AND** the request succeeds

#### Scenario: A page holds a merged result

- **WHEN** a client asks for the lines of a book of a copy with the profile `vl`
- **THEN** every read page names the model `merged`

#### Scenario: The book does not exist

- **WHEN** a client asks for the lines of a book id that does not exist
- **THEN** the system refuses the request
