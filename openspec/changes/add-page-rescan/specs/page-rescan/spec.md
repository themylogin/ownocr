## Purpose

Reads chosen pages of an already read book again, with the engine that reads
difficult pages. A rescan runs on the book itself, adds results to the pages it
names, and leaves the book whole when it cannot finish.

## ADDED Requirements

### Requirement: A page that the visual engine has not read offers a rescan

The system SHALL offer the rescan of a page when the book is in the `ocr` stage,
the book is in state `complete`, and the page holds no PaddleOCR-VL result.

A page that holds a PaddleOCR-VL result SHALL NOT offer it. The visual engine
has already read that page, so a second reading adds nothing.

The system SHALL report per page whether the page holds a PaddleOCR-VL result.
The client SHALL read that report and SHALL NOT derive the offer from the
profile of the book. After a rescan a book holds pages of both kinds.

#### Scenario: A page of a book read with the normal profile

- **WHEN** a client reads a complete book whose profile is `v6`
- **THEN** every page reports that it holds no PaddleOCR-VL result
- **AND** every page offers the rescan

#### Scenario: A page of a book read with the extra profile

- **WHEN** a client reads a complete book whose profile is `vl`
- **THEN** every page reports that it holds a PaddleOCR-VL result
- **AND** no page offers the rescan

#### Scenario: A book holds pages of both kinds

- **WHEN** a rescan reads pages 12 and 13 of a book whose profile is `v6`
- **THEN** pages 12 and 13 offer the rescan no more
- **AND** every other page of that book still offers it

#### Scenario: The book is not complete

- **WHEN** a client reads a book in the `ocr` stage in state `processing`
- **THEN** no page offers the rescan

#### Scenario: A book of another stage

- **WHEN** a client reads a book in the `uploaded`, `dewarped` or `split` stage
- **THEN** no page offers the rescan

### Requirement: The reader marks the pages a rescan will read

The page menu SHALL hold an item named "Rescan with Extra effort" for a page
that offers the rescan. Choosing it SHALL mark that page and SHALL start
nothing.

A marked page SHALL carry a border in the colour the system uses for a warning.
The border SHALL surround the read half of that page alone, not the pair. The
read half holds the text a rescan replaces, so the mark sits on what changes.
The border SHALL name the page as chosen, so the user sees every mark while
scrolling.

Marks SHALL accumulate. Choosing the item on a second page SHALL leave the first
mark standing. Choosing it again on a page that is already marked SHALL leave
that page marked.

A mark SHALL belong to the reader it was made in. Leaving the reader SHALL drop
every mark.

#### Scenario: A user marks one page

- **WHEN** a user chooses "Rescan with Extra effort" on page 12
- **THEN** the read half of page 12 carries the warning border
- **AND** the page half of page 12 carries none
- **AND** no request reaches the server
- **AND** the book stays in state `complete`

#### Scenario: A user marks three pages

- **WHEN** a user chooses the item on pages 12, 13 and 40
- **THEN** the read half of all three pages carries the warning border
- **AND** no other page carries one

#### Scenario: A user leaves the reader

- **WHEN** a user marks two pages and then opens another book
- **THEN** the marks are gone
- **AND** returning to the first book shows no mark

### Requirement: The reader top bar starts the rescan of every marked page

The reader top bar SHALL hold a control while one page or more is marked. The
control SHALL carry the colour of the marks and SHALL name the number of marked
pages. It SHALL stand after every other control of the bar, which puts it last
and after Reject. It SHALL be absent while no page is marked.

Activating the control SHALL ask for confirmation before anything runs. The
question SHALL say what a rescan does and why a user wants one. Cancelling SHALL
run nothing and SHALL leave every mark standing.

Confirming SHALL start the rescan and SHALL then leave the reader for the list
of the `ocr` stage. The book runs there, and its card reports the phase as every
running book does. Leaving drops every mark with it.

The control SHALL start no rescan while the book is not in state `complete`.

#### Scenario: No page is marked

- **WHEN** a user opens the reader of a complete book whose profile is `v6`
- **THEN** the top bar holds no rescan control

#### Scenario: Three pages are marked

- **WHEN** a user marks three pages
- **THEN** the top bar holds a control naming three pages
- **AND** it carries the colour of the marks

#### Scenario: The control stands last in the bar

- **WHEN** a user marks a page of a book in the reader
- **THEN** the control appears to the right of Reject

#### Scenario: A user confirms

- **WHEN** a user activates the control and confirms
- **THEN** the system starts one rescan of the marked pages
- **AND** the reader closes and the list of the `ocr` stage appears
- **AND** the card of that book reports it running

#### Scenario: A user cancels

- **WHEN** a user activates the control and cancels
- **THEN** no rescan starts
- **AND** every mark stays

#### Scenario: The book already runs

- **WHEN** a user opens the reader of a book whose rescan runs
- **THEN** the control starts no rescan

### Requirement: One request starts a rescan of the named pages

The system SHALL accept one request naming a book and the page indexes to read
again. It SHALL check the whole request before it changes anything, and SHALL
refuse the whole request when any check fails.

The system SHALL refuse a request that names no page. It SHALL refuse a request
naming a page index the book does not hold. It SHALL refuse a request naming a
page that already holds a PaddleOCR-VL result. It SHALL refuse a request for a
book that is not in the `ocr` stage, or that is not in state `complete`. It
SHALL name the reason in every case.

A page index named twice SHALL be read once. The book therefore bounds the
request: a rescan reads at most every page of its book.

The system SHALL set the book to state `processing` and SHALL queue one job, in
one transaction. It SHALL broadcast a `book.updated` event.

#### Scenario: A request names three pages

- **WHEN** a client asks to rescan pages 12, 13 and 40 of a complete book whose
  profile is `v6`
- **THEN** the book state becomes `processing`
- **AND** one job is queued
- **AND** every connected client hears a `book.updated` event

#### Scenario: A request names no page

- **WHEN** a client asks to rescan a book and names no page
- **THEN** the system refuses the request and names the reason
- **AND** the book stays in state `complete`

#### Scenario: A request names a page the book does not hold

- **WHEN** a client asks to rescan pages 12 and 900 of a book with 200 pages
- **THEN** the system refuses the whole request and names the reason
- **AND** page 12 is not queued

#### Scenario: A request names an already read page

- **WHEN** a client asks to rescan a page that holds a PaddleOCR-VL result
- **THEN** the system refuses the whole request and names the reason

#### Scenario: A request names a page twice

- **WHEN** a client asks to rescan pages 12, 12 and 13
- **THEN** the rescan reads page 12 once and page 13 once

#### Scenario: The book is not read yet

- **WHEN** a client asks to rescan a page of a book in the `split` stage
- **THEN** the system refuses the request and names the reason

#### Scenario: The book still runs

- **WHEN** a client asks to rescan a page of a book in state `processing`
- **THEN** the system refuses the request and names the reason

### Requirement: A rescan changes no page of the book and no other book

The system SHALL run a rescan in a background worker, not in the request that
starts it. The system SHALL move the book through `processing` and then back to
`complete`.

A rescan SHALL add stored results to the pages it names. It SHALL NOT add,
remove or reorder a page. It SHALL NOT change the page count, the stage, the
storage key, or the image of any page. It SHALL NOT change a page it does not
name. It SHALL NOT change the source book or any other book.

A rescan SHALL run on the queue the OCR action runs on, so a rescan and a whole
book reading never reach the endpoint at once.

#### Scenario: A rescan runs

- **WHEN** the worker picks up a rescan of pages 12, 13 and 40
- **THEN** the book state is `processing`
- **AND** the book keeps the stage `ocr`
- **AND** the book keeps its page count

#### Scenario: The other pages are untouched

- **WHEN** a rescan of pages 12, 13 and 40 ends, whatever the result
- **THEN** every other page holds the results it held before
- **AND** every page shows the image it showed before

#### Scenario: The source book is untouched

- **WHEN** a rescan ends, whatever the result
- **THEN** the source book keeps its state, its stage, and all of its pages

#### Scenario: The page count survives

- **WHEN** a rescan of 3 pages of a book with 200 pages completes
- **THEN** the book reports 200 pages throughout
- **AND** it reports 200 pages when it completes

### Requirement: A rescan runs the visual engine alone and merges with the stored line result

The system SHALL upload the original rendition of every named page, and no other
page, to the bucket the OCR endpoint reads. It SHALL start one endpoint job
naming the PaddleOCR-VL engine alone.

The PP-OCRv6 result of every named page is already stored, so the system SHALL
read it from its own store to build the merged result. It SHALL NOT ask the
endpoint for it again.

For every page it reads, the system SHALL store the PaddleOCR-VL result and the
merged result. It SHALL NOT change the stored PP-OCRv6 result of that page.

#### Scenario: A rescan of three pages

- **WHEN** the worker starts a rescan of pages 12, 13 and 40 of a book with 200
  pages
- **THEN** 3 objects exist under the input prefix
- **AND** the endpoint job names the PaddleOCR-VL engine alone

#### Scenario: A page gains its results

- **WHEN** the rescan stores page 12
- **THEN** page 12 holds a PaddleOCR-VL result and a merged result
- **AND** the merged result holds one line for every line of the PP-OCRv6 result
  of that page

#### Scenario: The line result is untouched

- **WHEN** the rescan stores page 12
- **THEN** the PP-OCRv6 result of page 12 holds the text and the result it held
  before the rescan

#### Scenario: A rescan of an already rescanned page

- **WHEN** a page holds a PaddleOCR-VL result
- **THEN** no rescan reads it again

### Requirement: A rescan stores every page it reads and abandons the rest

The system SHALL store the results of a page as soon as it holds them. One page
the endpoint could not read SHALL NOT cost a page the endpoint did read.

A whole book reading refuses a partial result, because a copy holding fewer
pages than its source is worse than no copy. That reason does not carry to a
rescan. A rescan only adds results, so a page stored is a page improved.

The system SHALL record the reason it could not read a page, where an operator
reads it. It SHALL NOT show that reason on the book.

#### Scenario: The endpoint reads two pages of three

- **WHEN** a rescan names pages 12, 13 and 40, and the endpoint reports page 40
  as failed
- **THEN** pages 12 and 13 hold a PaddleOCR-VL result and a merged result
- **AND** page 40 holds the one result it held before
- **AND** the book completes

#### Scenario: The endpoint reads no page

- **WHEN** a rescan names three pages and the endpoint fails whole
- **THEN** no page gains a result
- **AND** the book completes
- **AND** the book holds every result it held before

#### Scenario: An output object is missing

- **WHEN** the endpoint reports success for page 12 but an output object is
  absent
- **THEN** page 12 gains no result
- **AND** the other named pages still gain theirs

### Requirement: A rescan never fails the book

The system SHALL return the book to state `complete` when a rescan ends,
whatever the result. It SHALL NOT set the book to state `failed`. It SHALL NOT
store a failure reason on the book. It SHALL NOT offer a retry control for the
book.

The book is whole before a rescan and whole after one that fails. It holds every
result it held before. A failure state would describe work the book does not
depend on.

A retry of a whole book reading deletes every page of that book, which removes
every stored result with it. A rescan that set the book to `failed` would offer
that retry and would destroy every earlier rescan of the book. The system SHALL
close that path by never setting the state.

#### Scenario: A rescan fails whole

- **WHEN** every attempt of a rescan fails
- **THEN** the book state is `complete`
- **AND** the book carries no failure reason
- **AND** the book carries no phase

#### Scenario: No retry is offered

- **WHEN** a rescan of a book has failed
- **THEN** the card of that book offers no retry control
- **AND** the card of its source book carries no warning mark

#### Scenario: Earlier rescans survive

- **WHEN** a rescan of pages 12 and 13 succeeds, and a later rescan of page 40
  fails
- **THEN** pages 12 and 13 still hold their PaddleOCR-VL and merged results

#### Scenario: Attempts remain

- **WHEN** a rescan attempt fails and further attempts remain
- **THEN** the book state stays `processing`
- **AND** the system runs the job again

### Requirement: The reader shows the new text when it is opened again

A user who opens the reader of a book after a rescan SHALL see the text every
rescanned page now holds. The reader reads the text of a page when it opens, so
a rescan that ended before the reader opened is already in what it draws.

A rescan leaves the reader for the stage list, so no reader is open while one
runs. The reader therefore needs no way to replace the text of a page under the
user, and holds none.

A page the rescan did not reach SHALL draw the text it drew before, and SHALL
offer the rescan again. The system reports the offer per page, so a page that
did not take is offered again and a page that took is not.

#### Scenario: A rescanned page

- **WHEN** a user opens the reader of a book after a rescan of pages 12 and 13
- **THEN** page 12 draws the merged text
- **AND** page 13 draws the merged text

#### Scenario: A page the rescan did not name

- **WHEN** a user opens the reader after a rescan of pages 12 and 13
- **THEN** page 40 draws the text it drew before
- **AND** page 40 offers the rescan

#### Scenario: A page the rescan did not reach

- **WHEN** a rescan names pages 12 and 40, and the endpoint refuses page 40
- **THEN** page 40 draws the text it drew before
- **AND** page 40 offers the rescan again
- **AND** page 12 offers it no more

### Requirement: A rescan removes its exchange objects

The system SHALL delete every input and output object of a rescan when the
rescan ends. It SHALL clear the prefix before it uploads pages, so an object of
an earlier attempt cannot be read as a result of this one.

#### Scenario: A rescan ends

- **WHEN** a rescan returns the book to state `complete`
- **THEN** no object of that rescan remains in the bucket

#### Scenario: An attempt runs again

- **WHEN** the worker starts an attempt after a failed attempt
- **THEN** it removes every object left under the prefix first
