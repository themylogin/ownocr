## Purpose

Reads chosen pages of an already read book again, with the model the user picks.
A rescan runs on the book itself, changes the results of the pages it names, and
leaves the book whole when it cannot finish.

## Requirements

### Requirement: Every page of a book that has been read offers a rescan

The system SHALL offer the rescan of a page when the book is in the `ocr` stage
and the book is in state `complete`. It SHALL offer it for every page of such a
book, whatever results that page holds.

A reading is a choice, not a rank. A page a rescan read with the visual engine
can read worse than the page the line engine read, so the system SHALL let a
user read that page again with either engine. The offer therefore depends on the
book alone.

The client SHALL take the offer from the book, and SHALL NOT derive it from the
results of a page or from the profile of the book.

#### Scenario: A page of a book read with the normal profile

- **WHEN** a client reads a complete book whose profile is `v6`
- **THEN** every page offers the rescan

#### Scenario: A page of a book read with the extra profile

- **WHEN** a client reads a complete book whose profile is `vl`
- **THEN** every page offers the rescan

#### Scenario: A page a rescan has already read

- **WHEN** a rescan reads pages 12 and 13 of a book whose profile is `v6`
- **THEN** pages 12 and 13 still offer the rescan
- **AND** every other page of that book still offers it

#### Scenario: The book is not complete

- **WHEN** a client reads a book in the `ocr` stage in state `processing`
- **THEN** no page offers the rescan

#### Scenario: A book of another stage

- **WHEN** a client reads a book in the `uploaded`, `dewarped` or `split` stage
- **THEN** no page offers the rescan

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

### Requirement: The reader top bar starts the rescan of every marked page

The reader top bar SHALL hold a control while one page or more is marked. The
control SHALL carry the colour of the marks, which is the colour the system uses
for success, and SHALL name the number of marked pages. It SHALL name no engine.
It SHALL stand after every other control of the bar, which puts it last and after
Reject. It SHALL be absent while no page is marked.

Activating the control SHALL open a dialog before anything runs. The dialog SHALL
ask for the model to read the marked pages with, and SHALL say what a rescan does
and why a user wants one. The `book-library` capability governs what the dialog
holds. Cancelling SHALL run nothing and SHALL leave every mark standing.

Starting SHALL run the rescan with the chosen model and SHALL then leave the
reader for the list of the `ocr` stage. The book runs there, and its card reports
the phase as every running book does. Leaving drops every mark with it.

The control SHALL start no rescan while the book is not in state `complete`.

#### Scenario: No page is marked

- **WHEN** a user opens the reader of a complete book whose profile is `v6`
- **THEN** the top bar holds no rescan control

#### Scenario: Three pages are marked

- **WHEN** a user marks three pages
- **THEN** the top bar holds a control naming three pages
- **AND** it carries the colour of the marks, which is the colour of success
- **AND** it names no engine

#### Scenario: The control stands last in the bar

- **WHEN** a user marks a page of a book in the reader
- **THEN** the control appears to the right of Reject

#### Scenario: A user confirms

- **WHEN** a user activates the control, picks a model, and starts the dialog
- **THEN** the system starts one rescan of the marked pages with that model
- **AND** the reader closes and the list of the `ocr` stage appears
- **AND** the card of that book reports it running

#### Scenario: A user cancels

- **WHEN** a user activates the control and cancels the dialog
- **THEN** no rescan starts
- **AND** every mark stays

#### Scenario: The book already runs

- **WHEN** a user opens the reader of a book whose rescan runs
- **THEN** the control starts no rescan

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

### Requirement: A rescan changes no page of the book and no other book

The system SHALL run a rescan in a background worker, not in the request that
starts it. The system SHALL move the book through `processing` and then back to
`complete`.

A rescan SHALL change the stored results of the pages it names, and nothing
else. It SHALL NOT add, remove or reorder a page. It SHALL NOT change the page
count, the stage, the profile, the storage key, or the image of any page. It
SHALL NOT change a result of a page it does not name. It SHALL NOT change the
source book or any other book.

A rescan SHALL run on the queue the OCR action runs on, so a rescan and a whole
book reading never reach the endpoint at once.

#### Scenario: A rescan runs

- **WHEN** the worker picks up a rescan of pages 12, 13 and 40
- **THEN** the book state is `processing`
- **AND** the book keeps the stage `ocr`
- **AND** the book keeps its page count
- **AND** the book keeps the profile it records

#### Scenario: The other pages are untouched

- **WHEN** a rescan of pages 12, 13 and 40 ends, whatever the result and whatever
  the profile
- **THEN** every other page holds the results it held before
- **AND** every page shows the image it showed before

#### Scenario: The source book is untouched

- **WHEN** a rescan ends, whatever the result
- **THEN** the source book keeps its state, its stage, and all of its pages

#### Scenario: The page count survives

- **WHEN** a rescan of 3 pages of a book with 200 pages completes
- **THEN** the book reports 200 pages throughout
- **AND** it reports 200 pages when it completes

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

### Requirement: A normal rescan replaces the reading of a page

A rescan with the profile `v6` SHALL store the PP-OCRv6 result of every page it
reads, in place of the PP-OCRv6 result that page held.

It SHALL remove the PaddleOCR-VL result and the merged result of that page in
the same step. A user who rescans a page with the normal profile asks for the
line engine to answer for that page. The merged result derives from the boxes of
a PP-OCRv6 result this rescan has replaced, so it no longer describes the page.

The system SHALL store the new result and remove the two old ones together, or
do neither. A page SHALL NOT hold a PaddleOCR-VL result or a merged result built
from a PP-OCRv6 result it no longer holds.

A page that holds no PaddleOCR-VL result and no merged result SHALL gain its new
PP-OCRv6 result and lose nothing.

The reader answers the merged result of a page when the page holds one, and the
PP-OCRv6 result when it does not. A page a normal rescan read therefore reads as
PP-OCRv6 read it, with no further rule.

#### Scenario: A normal rescan of a page read by both engines

- **WHEN** a rescan with the profile `v6` stores a page holding a PP-OCRv6, a
  PaddleOCR-VL and a merged result
- **THEN** the page holds one result, naming PP-OCRv6
- **AND** that result is the result this rescan read
- **AND** the page holds no PaddleOCR-VL result and no merged result

#### Scenario: A normal rescan of a page read by one engine

- **WHEN** a rescan with the profile `v6` stores a page holding a PP-OCRv6
  result alone
- **THEN** the page holds one result, naming PP-OCRv6
- **AND** that result is the result this rescan read

#### Scenario: The reader after a normal rescan

- **WHEN** a client asks for the lines of a page a normal rescan read
- **THEN** the answer names the model `v6`
- **AND** it holds one entry per line of the result that rescan stored

#### Scenario: A normal rescan the endpoint could not read

- **WHEN** a rescan with the profile `v6` names a page and the endpoint reports
  that page as failed
- **THEN** the page holds every result it held before
- **AND** it keeps its PaddleOCR-VL result and its merged result

#### Scenario: A normal rescan reads one page of two

- **WHEN** a rescan with the profile `v6` names pages 12 and 13, and the
  endpoint reads page 12 alone
- **THEN** page 12 holds the new PP-OCRv6 result and no other result
- **AND** page 13 holds every result it held before

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

A page the rescan did not reach SHALL draw the text it drew before. Every page
of the book SHALL offer the rescan again, whatever the rescan reached.

#### Scenario: A rescanned page

- **WHEN** a user opens the reader of a book after a rescan of pages 12 and 13
  with the profile `vl`
- **THEN** page 12 draws the merged text
- **AND** page 13 draws the merged text

#### Scenario: A page a normal rescan read

- **WHEN** a user opens the reader of a book after a rescan of page 12 with the
  profile `v6`
- **THEN** page 12 draws the PP-OCRv6 text that rescan stored

#### Scenario: A page the rescan did not name

- **WHEN** a user opens the reader after a rescan of pages 12 and 13
- **THEN** page 40 draws the text it drew before
- **AND** page 40 offers the rescan

#### Scenario: A page the rescan did not reach

- **WHEN** a rescan names pages 12 and 40, and the endpoint refuses page 40
- **THEN** page 40 draws the text it drew before
- **AND** page 40 offers the rescan again
- **AND** page 12 offers it again as well

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
