## MODIFIED Requirements

### Requirement: The reader marks the pages a rescan will read

The page menu SHALL hold one item for a page that offers the rescan, and that item
SHALL say what choosing it does. For a page that is not marked it SHALL be named
"Rescan", and choosing it SHALL mark that page. For a page that is marked it SHALL
be named "Do not rescan", and choosing it SHALL take the mark off that page.

Choosing it SHALL start nothing either way. The item names no engine, because the
dialog that starts the rescan asks for one.

One item and not two: a mark is on or off, and the menu of a marked page has no
use for an item that marks it again.

A marked page SHALL carry a border in the colour the system uses for success.
The border SHALL surround the read half of that page alone, not the pair. The
read half holds the text a rescan replaces, so the mark sits on what changes.
The border SHALL name the page as chosen, so the user sees every mark while
scrolling.

A mark says a user chose this page, which is why it wears the colour of a choice
made and not the colour of a fault found. A page whose reading disagrees wears
the warning colour, and the `page-reading-health` capability governs that mark.

A page that is marked and carries another reason SHALL carry a border for each, one
inside the other, and every one of them SHALL be visible. They say different
things: that a user chose the page, that the system doubts its reading, that the
page holds a reading from GLM-5.3 Flash. None may hide another, and the mark of the
user stands innermost.

Every border SHALL be drawn inside the box of the half it marks. The reader skips
the layout of a page off screen, which clips whatever is drawn outside that box,
so a border drawn outside it would show on some edges and not others and would
depend on the width of the window.

Marks SHALL accumulate. Choosing the item on a second page SHALL leave the first
mark standing.

A page that loses its mark SHALL lose its border with it, and SHALL count no
longer among the marked pages. Taking the mark off the last marked page SHALL
leave no marked page, and the control that starts the rescan SHALL go.

A mark SHALL belong to the reader it was made in. Leaving the reader SHALL drop
every mark.

#### Scenario: A user marks one page

- **WHEN** a user chooses "Rescan" on page 12
- **THEN** the read half of page 12 carries the success border
- **AND** the page half of page 12 carries none
- **AND** no request reaches the server
- **AND** the book stays in state `complete`

#### Scenario: A user marks three pages

- **WHEN** a user chooses the item on pages 12, 13 and 40
- **THEN** the read half of all three pages carries the success border
- **AND** no other page carries one

#### Scenario: A user marks a page a rescan has read

- **WHEN** a user chooses the item on a page that holds a PaddleOCR-VL result
- **THEN** the read half of that page carries the success border

#### Scenario: The menu of a marked page

- **WHEN** a user opens the menu of a page that is already marked
- **THEN** the item is named "Do not rescan"
- **AND** the menu holds no item named "Rescan"

#### Scenario: A user takes a mark off

- **WHEN** a user marks pages 12 and 13, then chooses "Do not rescan" on page 12
- **THEN** page 12 carries no border
- **AND** page 13 keeps its mark
- **AND** the control names one page

#### Scenario: A user takes off the last mark

- **WHEN** a user marks one page and then chooses "Do not rescan" on it
- **THEN** no page carries a border
- **AND** the top bar holds no rescan control

#### Scenario: A user marks a page that disagrees

- **WHEN** a user chooses the item on a page whose reading disagrees
- **THEN** the read half of that page carries both borders
- **AND** the success border and the warning border are both visible

#### Scenario: A border on every edge at every width

- **WHEN** a marked page stands in a window of any width
- **THEN** its border shows on all four edges of the read half

#### Scenario: A user leaves the reader

- **WHEN** a user marks two pages and then opens another book
- **THEN** the marks are gone
- **AND** returning to the first book shows no mark

### Requirement: A rescan runs the engines of its profile

A rescan SHALL run the engines its profile names. The profile names `v6`, `vl`
and `glm` are those the `book-ocr` capability defines, and a user picks
between them in the same words.

The profile `vl` SHALL start one endpoint job naming the PaddleOCR-VL engine
alone. The PP-OCRv6 result of a page it names is already stored, so the system
SHALL read that result from its own store to build the merged result, and SHALL
NOT ask the endpoint for it again. PP-OCRv6 reads a page the same way every
time, so the stored result is the result a second reading would give.

The profile `v6` SHALL start one endpoint job naming the PP-OCRv6 engine alone.

The profile `glm` SHALL start no endpoint job at all. It SHALL send the page
image to GLM-5.3 Flash through the OpenRouter API, from the worker itself, and
SHALL ask for the text of the page and nothing else. The worker SHALL run it wholly in the server process:
no serverless endpoint, no object storage, and no bucket prefix.

The PP-OCRv6 result of a page the `glm` profile names is already stored, so
the system SHALL read it from its own store to build the merged result, as the
`vl` profile does.

The system SHALL upload the original rendition of every named page, and no other
page, to the bucket the OCR endpoint reads.

#### Scenario: An extra rescan of three pages

- **WHEN** the worker starts a rescan of pages 12, 13 and 40 of a book with 200
  pages, with the profile `vl`
- **THEN** 3 objects exist under the input prefix
- **AND** the endpoint job names the PaddleOCR-VL engine alone

#### Scenario: A normal rescan of three pages

- **WHEN** the worker starts a rescan of pages 12, 13 and 40, with the profile
  `v6`
- **THEN** 3 objects exist under the input prefix
- **AND** the endpoint job names the PP-OCRv6 engine alone

#### Scenario: An extra rescan reads the stored line result

- **WHEN** the worker runs a rescan with the profile `vl`
- **THEN** the endpoint job asks for no PP-OCRv6 result
- **AND** the merge of every page it stores uses the stored PP-OCRv6 result of
  that page

#### Scenario: An Ultra rescan of three pages

- **WHEN** the worker starts a rescan of pages 12, 13 and 40 with the profile
  `glm`
- **THEN** no object is uploaded to the bucket
- **AND** no endpoint job starts
- **AND** the worker sends three requests to the OpenRouter API

#### Scenario: An Ultra rescan stores its results

- **WHEN** a rescan with the profile `glm` reads page 12
- **THEN** page 12 holds a GLM-5.3 Flash result and a merged result of it
- **AND** the PP-OCRv6 result of page 12 holds the text it held before

#### Scenario: A page gains its results

- **WHEN** a rescan with the profile `vl` stores page 12
- **THEN** page 12 holds a PaddleOCR-VL result and a merged result
- **AND** the merged result holds one line for every line of the PP-OCRv6 result
  of that page

#### Scenario: The line result of an extra rescan is untouched

- **WHEN** a rescan with the profile `vl` stores page 12
- **THEN** the PP-OCRv6 result of page 12 holds the text and the result it held
  before the rescan

#### Scenario: An extra rescan of a page that holds a visual result

- **WHEN** a rescan with the profile `vl` reads a page that already holds a
  PaddleOCR-VL result
- **THEN** the page holds the PaddleOCR-VL result of this rescan
- **AND** it holds a merged result built from that result
- **AND** it holds no result of the earlier visual reading

### Requirement: One request starts a rescan of the named pages

The system SHALL accept one request naming a book, the page indexes to read
again, and the profile to read them with. It SHALL check the whole request
before it changes anything, and SHALL refuse the whole request when any check
fails.

The system SHALL refuse a request that names no page. It SHALL refuse a request
naming a page index the book does not hold. It SHALL refuse a request that names
no profile, and one that names a profile the system does not know. The rescan
SHALL accept every profile the system defines, including `glm`. It SHALL
refuse a request for a book that is not in the `ocr` stage, or that is not in
state `complete`. It SHALL name the reason in every case.

The system SHALL NOT refuse a page for the results it holds. Every page of a
book it accepts can be read again, with either profile.

A page index named twice SHALL be read once. The book therefore bounds the
request: a rescan reads at most every page of its book.

The system SHALL set the book to state `processing` and SHALL queue one job, in
one transaction. The job SHALL carry the profile of the request. It SHALL
broadcast a `book.updated` event.

The profile of the request SHALL describe this rescan alone. The system SHALL
NOT change the profile the book records.

#### Scenario: A request names three pages

- **WHEN** a client asks to rescan pages 12, 13 and 40 of a complete book whose
  profile is `v6`, with the profile `vl`
- **THEN** the book state becomes `processing`
- **AND** one job is queued, carrying the profile `vl`
- **AND** every connected client hears a `book.updated` event
- **AND** the book still records the profile `v6`

#### Scenario: A request names the Ultra profile

- **WHEN** a client asks to rescan page 12 with the profile `glm`
- **THEN** one job is queued, carrying the profile `glm`

#### Scenario: A request names the normal profile

- **WHEN** a client asks to rescan page 12 with the profile `v6`
- **THEN** one job is queued, carrying the profile `v6`

#### Scenario: A request names no page

- **WHEN** a client asks to rescan a book and names no page
- **THEN** the system refuses the request and names the reason
- **AND** the book stays in state `complete`

#### Scenario: A request names no profile

- **WHEN** a client asks to rescan page 12 and names no profile
- **THEN** the system refuses the request and names the reason
- **AND** the book stays in state `complete`

#### Scenario: A request names an unknown profile

- **WHEN** a client asks to rescan page 12 with a profile that is not `v6` or
  `vl`
- **THEN** the system refuses the request and names the reason

#### Scenario: A request names a page the book does not hold

- **WHEN** a client asks to rescan pages 12 and 900 of a book with 200 pages
- **THEN** the system refuses the whole request and names the reason
- **AND** page 12 is not queued

#### Scenario: A request names an already read page

- **WHEN** a client asks to rescan a page that holds a PaddleOCR-VL result
- **THEN** the system accepts the request
- **AND** one job is queued

#### Scenario: A request names a page twice

- **WHEN** a client asks to rescan pages 12, 12 and 13
- **THEN** the rescan reads page 12 once and page 13 once

#### Scenario: The book is not read yet

- **WHEN** a client asks to rescan a page of a book in the `split` stage
- **THEN** the system refuses the request and names the reason

#### Scenario: The book still runs

- **WHEN** a client asks to rescan a page of a book in state `processing`
- **THEN** the system refuses the request and names the reason
