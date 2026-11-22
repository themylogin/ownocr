## MODIFIED Requirements

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
  "Ultra (GLM-5.3 Flash)"

#### Scenario: The menu of a page that agrees

- **WHEN** a user opens the menu of a page whose reading agrees
- **THEN** the menu says nothing about a doubt

#### Scenario: The explanation answers no choice

- **WHEN** a user clicks the explanation
- **THEN** nothing runs and the menu stays open

### Requirement: The reader shows which pages hold an Ultra reading

The reader SHALL draw a border in the colour the system uses for information round
the read half of every page that holds a reading from GLM-5.3 Flash. A reading on a
metered API is paid for once, so a reader is told which pages already hold one and
can choose it without paying again.

It SHALL mark a page that holds such a reading, whether or not that reading is the
one the page answers. A reader who chose another reading of that page has not lost
the Ultra one.

It SHALL NOT mark a page that merely could be read by GLM-5.3 Flash. Every page of
a complete book could be, so such a border would mark every page and say nothing.

The border SHALL be drawn inside the box of the read half, as every other border of
a page is, and SHALL move no page. Where a page carries another reason as well, the
borders stand one inside the other and each stays visible.

#### Scenario: A page an Ultra rescan read

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
  "Ultra (GLM-5.3 Flash)", in that order
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
- "Ultra", subtitled "GLM-5.3 Flash", described as the frontier visual model, to
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
- **THEN** the system rescans the marked pages with the profile `glm`

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
