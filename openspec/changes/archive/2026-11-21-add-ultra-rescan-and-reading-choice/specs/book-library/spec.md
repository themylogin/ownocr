## ADDED Requirements

### Requirement: The reader reports the pages that disagree

The reader of a book whose pages hold more than one reading SHALL name the number
of pages that disagree, in the same block as the breadcrumbs and immediately after
the last of them. It SHALL say it in words, as "16 pages disagree", and SHALL say
"1 page disagrees" for one.

It SHALL read as part of the trail and not as a control of its own, so it stands
beside the last crumb and SHALL NOT be pushed to the far side of the bar.

It SHALL say nothing at all when no page of the book disagrees, and nothing for a
book whose pages hold one reading each. A count of zero is not news.

The reader SHALL draw a border in the colour the system uses for a warning round
the read half of every disagreeing page. The read half holds the text that
disagrees, so the mark sits on what is wrong. The border SHALL surround that half
alone, not the pair, and SHALL move no page.

The border SHALL show on all four edges of that half, in a window of any width.
The reader skips the layout of a page off screen, which clips whatever is drawn
outside the box of that half, so the border SHALL be drawn inside the box.

The `page-reading-health` capability decides which pages disagree.

A page carrying more than one reason SHALL carry a border for each, one inside the
other, and every one of them SHALL be visible. The mark of a rescan wears the
colour of success, because a user chose that page; this border wears the colour of
a warning, because the system doubts that reading; and the border of a page holding
an Ultra reading wears the colour the system uses for information. Each says a
different thing, so none may replace another.

#### Scenario: A book with sixteen disagreeing pages

- **WHEN** a user opens the reader of a book whose pages hold two readings, and
  sixteen of them disagree
- **THEN** the bar says "16 pages disagree" in the same block as the breadcrumbs,
  right of the last crumb
- **AND** the read half of each of those sixteen pages carries the warning border

#### Scenario: A book with one disagreeing page

- **WHEN** one page of the book disagrees
- **THEN** the bar says "1 page disagrees"

#### Scenario: A book where nothing disagrees

- **WHEN** no page of the book disagrees
- **THEN** the bar says nothing about disagreement
- **AND** no page carries the border

#### Scenario: A book read with the normal profile

- **WHEN** a user opens the reader of a book whose every page holds one reading
- **THEN** the bar says nothing about disagreement

#### Scenario: A page that is marked and disagrees

- **WHEN** a user marks a disagreeing page for a rescan
- **THEN** the read half of that page carries the warning border and the success
  border, one inside the other
- **AND** both are visible

#### Scenario: A page carrying every reason at once

- **WHEN** a user marks a disagreeing page that holds an Ultra reading
- **THEN** the read half of that page carries three borders, one inside the other
- **AND** all three are visible, in three colours

#### Scenario: The border of a narrow window and a wide one

- **WHEN** a user reads a book holding a disagreeing page in a window 1280 pixels
  wide, and then in one 4000 pixels wide
- **THEN** the border shows on the left, the right, the top and the bottom of the
  read half in both

### Requirement: The reader shows which lines the merge filled from the line reading

The reader SHALL draw a line of a merged reading whose text came from the line
engine on a grey ground, where it draws the rest of the reading on white. Its text
SHALL stay the colour of every other line. The `book-ocr` capability says which
lines those are.

The text and not the ground is the page: such a line is as much the reading as any
other, so colouring its glyphs would read as a mark upon the text rather than as a
note about where it came from. The ground SHALL stay light enough to read black text
on.

A merged reading holds the text of its reading engine where the two engines read a
line alike, and the text of the line engine where they did not. The page looks
whole either way, so a reader who is not told reads the weaker transcription of
some lines without knowing it — on a page whose body came from the reading engine
and whose footnotes did not, the footnote Greek is the line engine's and wrong,
and nothing on the page says so.

The colour SHALL stay dark enough to read as body text. These lines are the page
and not an annotation of it, so the reader SHALL NOT mark them up, box them or
move them.

The reader SHALL draw every line of an engine result in black. A reading that is
no merge holds one engine's text throughout, and a page drawn wholly in brown
says nothing.

#### Scenario: A page whose footnotes came from the line engine

- **WHEN** a user reads a page whose merged reading took the body from
  PaddleOCR-VL and the footnotes from PP-OCRv6
- **THEN** the footnote lines stand on a grey ground
- **AND** the body lines stand on a white one
- **AND** every line of both is drawn in the same colour

#### Scenario: A page that answers its line reading

- **WHEN** a user chooses the line reading of a page
- **THEN** every line of that page stands on a white ground

### Requirement: The reader holds a strip naming the pages worth returning to

The reader SHALL hold a strip down the right edge of the window, inside it and so
left of the scrollbar of the browser, holding one mark per page that carries a
reason: a page that disagrees, a page marked for a rescan, or a page holding an
Ultra reading. A book runs to many pages and the reader scrolls through all of them,
so a reader who finds such a page means to come back to it.

A mark SHALL stand where the thumb of the scrollbar stands when its page is in the
window, so a reader scrolls to a mark by dragging the scrollbar to it.

That is not the share of the book the page sits at. The pages do not begin at the
top of the scrollable height — the bar and the controls stand above them — and the
thumb is as tall a share of its track as the window is of the document and travels
the track less its own height. A mark placed by the share of the page count alone
therefore sits above the thumb that reaches it, and a mark placed by the share of
the document agrees with the thumb in the middle of a book and misses it at both
ends by half a thumb.

A page carrying no reason SHALL get no mark. A mark for every page would be a second
scrollbar, and would say nothing.

A mark SHALL wear the colours its page wears, and for the same reasons: the colour of
success for a page a user chose, the colour of a warning for a reading the system
doubts, the colour of information for a page holding an Ultra reading.

A page carrying more than one reason SHALL show a band per reason within its mark,
sharing the mark between them in the order the borders of that page stand in. Every
band SHALL be visible, and a page carrying one reason SHALL wear that colour
throughout its mark.

Choosing a mark SHALL bring its page into view. The strip SHALL take no room from
the pages and SHALL move none of them.

The strip SHALL hold nothing at all while no page of the book carries a reason.

#### Scenario: A book holding doubted pages

- **WHEN** a user opens the reader of a book whose pages 6 and 60 disagree
- **THEN** the strip holds two marks in the colour of a warning
- **AND** the mark of page 60 stands below the mark of page 6

#### Scenario: A user marks a page

- **WHEN** a user marks a page for a rescan
- **THEN** the strip holds a mark for it in the colour of success

#### Scenario: A page that is marked and disagrees

- **WHEN** a user marks a page whose reading disagrees
- **THEN** its mark shows both colours, one inside the other

#### Scenario: A page holding an Ultra reading

- **WHEN** an Ultra rescan reads a page and the reader is opened again
- **THEN** the strip holds a mark for it in the colour of information

#### Scenario: A mark of a page carrying every reason

- **WHEN** a user marks a disagreeing page that holds an Ultra reading
- **THEN** its mark shows three bands, in three colours
- **AND** all three are visible

#### Scenario: A user chooses a mark

- **WHEN** a user chooses the mark of page 60
- **THEN** page 60 comes into view

#### Scenario: The marks of the first and the last page

- **WHEN** a book whose first and last page both disagree stands in the reader
- **THEN** the mark of the first page stands where the thumb stands at the top of
  the book, and the mark of the last where it stands at the foot
- **AND** neither sits against the edge of the strip

#### Scenario: A book with nothing to mark

- **WHEN** a user opens the reader of a book whose every page holds one reading
- **THEN** the strip holds no mark

### Requirement: The page menu says why a reading is doubted

The menu of a page whose reading disagrees SHALL say why, under the reading it
judges. A reader who sees the border asks what is wrong with the page, and the menu
is where the page answers.

It SHALL stand with the reading that answers and not at the foot of the menu, where
it would read as a note on the rescan below it. A page whose merged PaddleOCR-VL
reading answers therefore explains itself under "Extra" and above "Ultra".

It SHALL be short. Each reason SHALL name the fault and its number and nothing
else, because a menu is read at a glance: "119 unexpected Cyrillic characters",
"Contains only 5% of the text".

It SHALL wear the colour the system uses for a warning, which is the colour of the
border it explains, so the words and the mark on the page read as one thing. It
SHALL carry no heading: a reader who opened the menu of a bordered page needs no
sentence telling them the reading is doubted.

The text SHALL carry the left padding of an item of the menu, so it lines up with
the labels around it and not with the edge.

The explanation SHALL be read and not chosen. It SHALL answer no selection and
SHALL close nothing.

The menu of a page whose reading agrees SHALL say nothing about it. The
`page-reading-health` capability decides which pages disagree and holds the words.

#### Scenario: The menu of a page that turned Cyrillic

- **WHEN** a user opens the menu of a page whose reading holds 119 Cyrillic
  characters its line reading holds none of
- **THEN** the menu says "119 unexpected Cyrillic characters"
- **AND** it says it in the colour of a warning, under the reading that answers

#### Scenario: The menu of a page whose reading stopped early

- **WHEN** a user opens the menu of a page whose reading holds 5% of the text of
  its line reading
- **THEN** the menu says "Contains only 5% of the text"

#### Scenario: The explanation stands with the reading it judges

- **WHEN** a user opens the menu of a doubted page whose merged PaddleOCR-VL
  reading answers, and which also holds an Ultra reading
- **THEN** the explanation stands below "Extra (PaddleOCR-VL)" and above
  "Ultra (Claude Opus 5)"

#### Scenario: The menu of a page that agrees

- **WHEN** a user opens the menu of a page whose reading agrees
- **THEN** the menu says nothing about a doubt

#### Scenario: The explanation answers no choice

- **WHEN** a user clicks the explanation
- **THEN** nothing runs and the menu stays open

### Requirement: The reader shows which pages hold an Ultra reading

The reader SHALL draw a border in the colour the system uses for information round
the read half of every page that holds a reading from Claude Opus 5. A reading on a
metered API is paid for once, so a reader is told which pages already hold one and
can choose it without paying again.

It SHALL mark a page that holds such a reading, whether or not that reading is the
one the page answers. A reader who chose another reading of that page has not lost
the Ultra one.

It SHALL NOT mark a page that merely could be read by Claude Opus 5. Every page of
a complete book could be, so such a border would mark every page and say nothing.

The border SHALL be drawn inside the box of the read half, as every other border of
a page is, and SHALL move no page. Where a page carries another reason as well, the
borders stand one inside the other and each stays visible.

#### Scenario: A page read by Claude Opus 5

- **WHEN** an Ultra rescan reads page 12 and the reader is opened again
- **THEN** the read half of page 12 carries the border of information
- **AND** no other page carries it

#### Scenario: A page that answers another of its readings

- **WHEN** a reader chooses the line reading of a page that holds an Ultra reading
- **THEN** that page still carries the border of information

#### Scenario: A page no Ultra rescan has read

- **WHEN** a user opens the reader of a book whose pages hold the line reading and
  the merged PaddleOCR-VL reading
- **THEN** no page carries the border of information

### Requirement: The page grid marks the pages that disagree

The grid of pages of a book SHALL name the number of pages that disagree, in the
same block as its breadcrumbs and immediately after the last of them, in the same
words the reader uses. The grid is where a reader arrives before the reader, so it
says the same thing in the same place.

The grid of pages of a book SHALL draw a border in the colour the system uses for
a warning round the thumbnail of every page whose reading disagrees. A reader
opens the grid to find a page, so the grid says which pages are worth opening.

It SHALL mark nothing for a book whose pages hold one reading each, and nothing
for a book that has not been read.

The border SHALL move no thumbnail and SHALL NOT change the size of the cell it
sits in. The `page-reading-health` capability decides which pages disagree, and it
decides the same pages here as in the reader.

#### Scenario: A grid of a book holding disagreeing pages

- **WHEN** a user opens the grid of a book whose pages 6 and 18 disagree
- **THEN** the bar says "2 pages disagree" in the same block as the breadcrumbs
- **AND** the thumbnails of pages 6 and 18 carry the warning border
- **AND** no other thumbnail carries one
- **AND** every cell keeps the size it had

#### Scenario: A grid of a book read with the normal profile

- **WHEN** a user opens the grid of a book whose every page holds one reading
- **THEN** no thumbnail carries the border
- **AND** the bar says nothing about disagreement

#### Scenario: A grid of a book that has not been read

- **WHEN** a user opens the grid of a book in the `split` stage
- **THEN** no thumbnail carries the border

## MODIFIED Requirements

### Requirement: The right half of the OCR reader offers the reading of the page

A right click on the right half of the OCR reader SHALL open a menu about the
reading of that page and nothing else. It SHALL NOT open the page menu, and it
SHALL NOT open the menu of the browser.

The right half stacks an image, a white veil and drawn text, so a copy saved from
it would hold the page without one glyph of the reading. Every item that reaches
the image therefore stays on the left half, and everything about the reading
belongs here: a user who finds a badly read page is reading it on this half.

The menu SHALL hold, in this order:

- The readings that page can answer, one item each, named by the profile that made
  them and the engine behind them, ordered "Normal", "Extra", "Ultra". The
  `page-reading-choice` capability says which readings those are and what choosing
  one does.
- Why the reading of that page is doubted, when it is doubted, under the reading it
  judges rather than under all of them.
- The rescan item, named "Rescan" for a page that is not marked and "Do not
  rescan" for one that is. The `page-rescan` capability governs what choosing it
  does.

The readings SHALL be offered best last and not best first. A reader reads the
list as the dialog that starts a rescan reads it, and that dialog offers Normal,
then Extra, then Ultra.

A page of a book that has not been read holds no reading and is doubted for none,
so its menu SHALL hold the rescan item alone. Every page of a complete book in the
`ocr` stage offers the rescan, whatever results that page holds. A right click on
the right half of a page of a book that offers no rescan SHALL open no menu. An
empty menu says less than no menu.

A right click on the left half SHALL open the page menu, as a right click on any
other page does. A right click on the gap between the two halves SHALL open no
menu.

The drawn text SHALL stay selectable, and a user SHALL still copy a selection by
keyboard.

#### Scenario: A user right clicks the read half

- **WHEN** a user right clicks the right half of a page in the reader of a
  complete book whose profile is `v6`
- **THEN** a menu opens at the pointer holding "Normal (PP-OCRv6)" and "Rescan"
- **AND** no browser menu opens

#### Scenario: A user right clicks the read half of a page read three times

- **WHEN** a user right clicks the right half of a page holding the line reading,
  the merged PaddleOCR-VL reading and the merged Ultra reading
- **THEN** the menu holds "Normal (PP-OCRv6)", then "Extra (PaddleOCR-VL)", then
  "Ultra (Claude Opus 5)", in that order
- **AND** the rescan item stands after all three

#### Scenario: A user right clicks the read half of a doubted page

- **WHEN** a user right clicks the right half of a page whose reading is doubted
- **THEN** the menu says why, under the reading that answers

#### Scenario: A user right clicks the read half of a marked page

- **WHEN** a user right clicks the right half of a page already marked for a
  rescan
- **THEN** the menu holds "Do not rescan"
- **AND** it holds no item named "Rescan"

#### Scenario: A user right clicks the read half of an already rescanned page

- **WHEN** a user right clicks the right half of a page that holds a
  PaddleOCR-VL result
- **THEN** a menu opens at the pointer holding the rescan item
- **AND** no browser menu opens

#### Scenario: A user right clicks the read half of a running book

- **WHEN** a user right clicks the right half of a page of a book in the `ocr`
  stage that is not complete
- **THEN** no menu opens
- **AND** no browser menu opens

#### Scenario: A user right clicks the page half

- **WHEN** a user right clicks the left half of a page in the reader of a book in
  the `ocr` stage
- **THEN** the page menu opens at the pointer
- **AND** it holds no reading, no rescan item and no explanation of a doubt

#### Scenario: A user copies the text of a read page

- **WHEN** a user selects the drawn text of a line and copies it by keyboard
- **THEN** the clipboard holds that text

#### Scenario: The reader of another stage keeps its menu

- **WHEN** a user right clicks a page in the reader of a book in the `split`
  stage
- **THEN** the page menu opens at the pointer

### Requirement: The rescan asks for a model before it runs

The interface SHALL ask for the model a rescan reads with, in the dialog that
starts the rescan. The rescan SHALL NOT start before the user answers the dialog.

The dialog SHALL keep its own title, which names the rescan and not the OCR
action. It SHALL say what a rescan does and why a user wants one. Its starting
control SHALL be named "Rescan".

The dialog SHALL offer the two choices the OCR dialog offers, in the same order
and the same words, and a third the OCR dialog does not offer:

- "Normal", subtitled "PP-OCRv6", described as the frontier OCR model.
- "Extra", subtitled "PaddleOCR-VL", described as the visual model, for
  difficult cases like polytonic Greek.
- "Ultra", subtitled "Claude Opus 5", described as the frontier visual model, to
  use when even PaddleOCR-VL fails.

The dialog SHALL preselect the model after the one the book was read with, where
the OCR dialog preselects "Normal". A user rescans a page because the model it was
read with read that page badly, so the next model up is the common answer.

A book read with PP-OCRv6 SHALL therefore preselect "Extra", and a book read with
PaddleOCR-VL SHALL preselect "Ultra", which is the only model left to try.

Cancelling, pressing Escape, or clicking away SHALL close the dialog, start
nothing, and leave every mark standing.

Starting SHALL run the rescan of every marked page with the chosen profile. A
refusal from the server SHALL appear in the dialog, and the dialog SHALL stay
open with every mark standing.

#### Scenario: A user opens the dialog

- **WHEN** a user marks two pages of a book whose profile is `v6` and activates the
  rescan control
- **THEN** the dialog opens holding "Normal", "Extra" and "Ultra" in that order
- **AND** "Extra" is selected
- **AND** no rescan has started

#### Scenario: A user opens the dialog on a book read with PaddleOCR-VL

- **WHEN** a user marks a page of a book whose profile is `vl` and activates the
  rescan control
- **THEN** "Ultra" is selected
- **AND** no rescan has started

#### Scenario: A user starts the extra profile

- **WHEN** a user opens the dialog of a book whose profile is `v6` and starts it
  without changing the choice
- **THEN** the system rescans the marked pages with the profile `vl`
- **AND** the dialog closes

#### Scenario: A user starts the Ultra profile

- **WHEN** a user picks "Ultra" and starts the dialog
- **THEN** the system rescans the marked pages with the profile `claude`

#### Scenario: The OCR dialog offers no Ultra

- **WHEN** a user opens the dialog of the OCR action
- **THEN** it offers "Normal" and "Extra" and no third choice

#### Scenario: A user starts the normal profile

- **WHEN** a user picks "Normal" and starts the dialog
- **THEN** the system rescans the marked pages with the profile `v6`

#### Scenario: A user cancels the dialog

- **WHEN** a user cancels the dialog, presses Escape, or clicks away
- **THEN** the dialog closes
- **AND** no rescan runs
- **AND** every marked page keeps its mark

#### Scenario: The server refuses the rescan

- **WHEN** the server refuses the rescan the dialog started
- **THEN** the dialog shows the reason
- **AND** the reader stays open with every mark standing

### Requirement: A right click on a page offers the page menu

A right click on a page SHALL open a context menu at the pointer. The page grid
and the reader SHALL offer the same menu.

This menu SHALL reach the image of the page and change the page. It SHALL hold
nothing about the reading of that page: no reading to choose, no rescan and no
explanation of a doubt. Those belong to the read half of the reader, which the
requirement above governs, because that half is where the reading is.

The menu SHALL open with "Open in new tab" and "Download", then a divider. Both
items SHALL appear for every page, so a page always opens a menu holding at least
those two and "Delete".

The menu SHALL then hold "Edit" when that page offers a re-cut, then "Show
original" when that page offers a rollback, then a divider, then a "Delete"
item. No page offers both, because an action makes either one page from one page
or two. The menu SHALL decide per page, not per book: a page that already shows
its original, that names no source page, or that records no cut, SHALL NOT hold
its item while its neighbours do. The system SHALL show "Delete" in the colour it
uses for danger. A page that offers none of the three SHALL open a menu holding
"Open in new tab", "Download", a divider and "Delete". Pressing Escape or
clicking away SHALL close the menu and run nothing.

The system SHALL take the offer of a rollback, the offer of a re-cut and the
offer of a rescan from the server, so a new action needs no client change. The
server reports the offer of a rescan for the book, so every page of a book that
offers it holds the item.

#### Scenario: A user right clicks a page of a book read with the normal profile

- **WHEN** a user right clicks the left half of a page of a complete book whose
  profile is `v6`
- **THEN** the menu opens holding "Open in new tab", "Download", a divider, then
  "Delete"
- **AND** it holds no reading, no "Rescan" and no explanation of a doubt
- **AND** it holds no "Edit" and no "Show original"

#### Scenario: A user right clicks the page half of a page read three times

- **WHEN** a user right clicks the left half of a page holding the line reading,
  the merged PaddleOCR-VL reading and the merged Ultra reading
- **THEN** the menu holds no reading item
- **AND** the read half of that page offers all three

#### Scenario: A page of an unread book

- **WHEN** a user right clicks a page of a book in the `split` stage
- **THEN** the menu opens with "Open in new tab"
- **AND** it holds no reading item

#### Scenario: A user right clicks an already rescanned page

- **WHEN** a user right clicks the left half of a page that holds a PaddleOCR-VL
  result
- **THEN** the menu opens holding "Open in new tab", "Download", a divider, then
  "Delete"

#### Scenario: A user right clicks a page of a book read with the extra profile

- **WHEN** a user right clicks the left half of a page in the reader of a
  complete book whose profile is `vl`
- **THEN** the menu holds no "Rescan"
- **AND** the read half of that page holds it

#### Scenario: A user right clicks a dewarped page

- **WHEN** a user right clicks a page in the grid of a dewarped copy that has no
  copy of its own
- **THEN** a context menu opens at the pointer
- **AND** it holds "Open in new tab", "Download", a divider, "Show original", a
  divider, then "Delete"
- **AND** it holds no "Rescan"
- **AND** "Delete" is not the colour of the other items

#### Scenario: A user right clicks a page cut out of a spread

- **WHEN** a user right clicks a page of a split copy that was cut out of a
  spread, and that copy has no copy of its own
- **THEN** the menu opens holding "Open in new tab", "Download", a divider,
  "Edit", a divider, then "Delete"
- **AND** "Open in new tab" is the first item

#### Scenario: A user right clicks a page the split copied whole

- **WHEN** a user right clicks a page of a split copy that records no cut
- **THEN** the menu opens holding "Open in new tab", "Download", a divider, then
  "Delete"
- **AND** it holds no "Edit"
- **AND** the menu of a page of that book that was cut still holds "Edit"

#### Scenario: A user right clicks an uploaded page

- **WHEN** a user right clicks a page of an uploaded book
- **THEN** the menu opens holding "Open in new tab", "Download", a divider, then
  "Delete"
- **AND** it holds no "Edit" and no "Show original"

#### Scenario: A user right clicks a page that was rolled back

- **WHEN** a user right clicks a page that already shows its original
- **THEN** the menu opens holding "Open in new tab", "Download", a divider, then
  "Delete"
- **AND** the menu of every other page of that book still holds "Show original"

#### Scenario: The reader offers the same menu

- **WHEN** a user right clicks a page in the reader
- **THEN** the same menu opens at the pointer

#### Scenario: The grid offers no rescan

- **WHEN** a user right clicks a page in the grid of a complete book whose
  profile is `v6`
- **THEN** the menu opens holding "Open in new tab", "Download", a divider, then
  "Delete"
- **AND** it holds no "Rescan"

#### Scenario: A user dismisses the menu

- **WHEN** the page menu is open and the user presses Escape or clicks away
- **THEN** the menu closes
- **AND** nothing is deleted, nothing is rolled back, nothing is cut and no
  rescan starts

## RENAMED Requirements

### Requirement: The right half of the OCR reader offers the reading of the page

FROM: `### Requirement: The right half of the OCR reader offers the rescan alone`
TO: `### Requirement: The right half of the OCR reader offers the reading of the page`
