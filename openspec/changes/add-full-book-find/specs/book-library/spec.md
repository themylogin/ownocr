## ADDED Requirements

### Requirement: The OCR reader holds the text of every page

The reader SHALL ask for the lines of every page of the book in one request, when
it opens. It SHALL draw the text of every page it holds lines for, whether that
page is near the window or far from it. It SHALL remove the text of no page while
the reader stays open.

A page SHALL reserve the size it takes before it is drawn, so the reader holds
its full height from the start. No page SHALL move while a user scrolls, and a
jump to a page SHALL land on that page.

A page that holds no result SHALL show the veiled image with no text, and SHALL
raise no error to the user. A book whose lines do not arrive SHALL show every
page as a veiled image, and SHALL raise no error.

The reader SHALL skip the drawing of a page that lies far from the window, and
SHALL draw it when it comes near. Skipping the drawing SHALL NOT remove the text
of that page, and SHALL NOT change the place or the size of any page.

#### Scenario: A large OCR book opens

- **WHEN** a user opens the reader for an OCR book of 500 pages
- **THEN** the reader makes one request for the lines of the book
- **AND** it makes no further request while it stays open

#### Scenario: A user scrolls on

- **WHEN** a user scrolls past a page that carried drawn text
- **THEN** that page keeps its text
- **AND** the pages that come near the window show their text
- **AND** no page moves

#### Scenario: A user opens the reader at a page

- **WHEN** a user opens the reader at page 300 of a book of 500 pages
- **THEN** the reader lands on page 300
- **AND** the pages around it show their text

#### Scenario: A page holds no result

- **WHEN** the reader reaches a page of a running OCR copy that holds no result
- **THEN** the right half shows the veiled image with no text
- **AND** the reader shows no error

#### Scenario: The lines do not arrive

- **WHEN** the request for the lines of the book fails
- **THEN** every page shows its veiled image with no text
- **AND** the reader shows no error

### Requirement: Find-in-page reaches every page of the OCR reader

The browser find SHALL reach the drawn text of every page of the book, not the
pages near the window only. A match on a page far from the window SHALL scroll
that page into view.

The count the browser reports SHALL count the matches of the whole book. A reader
who searches a word SHALL learn how often the book holds it.

The find SHALL match inside one drawn line. A drawn line stands at the place its
box names, so two lines are two separate pieces of text, and a phrase that
crosses a line break SHALL NOT match.

#### Scenario: A user searches a word on a far page

- **WHEN** a user opens the reader at page 1 of a book of 500 pages, and searches
  a word that only page 400 holds
- **THEN** the browser reports the match
- **AND** it scrolls page 400 into view

#### Scenario: The count covers the book

- **WHEN** a user searches a word the book holds 30 times, across 20 pages
- **THEN** the browser reports 30 matches

#### Scenario: A phrase inside one line

- **WHEN** a user searches a phrase that one drawn line holds in full
- **THEN** the browser reports the match

#### Scenario: A phrase across two lines

- **WHEN** a user searches a phrase whose words end one drawn line and begin the
  next
- **THEN** the browser reports no match

## REMOVED Requirements

### Requirement: The OCR reader draws the text of the pages near the window only

**Reason**: The reader now holds the text of every page, so find-in-page reaches
the whole book. A spike measured the cost on a book of 1000 pages and found it
affordable.

**Migration**: The requirement "The OCR reader holds the text of every page"
replaces it. The rule for a page with no result carries over unchanged. The rule
that no page moves while a user scrolls carries over, and a page now reserves its
size instead of keeping a drawn image it already held.
