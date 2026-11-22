# page-reading-choice Specification

## Purpose
A page holds every reading that ever read it, and one of them answers. This says
which one that is, when a new reading takes over, and how a reader chooses
another.

## Requirements

### Requirement: A page names the reading that answers for it

A page SHALL name one of its stored results as the reading that answers. The
system SHALL answer that reading wherever it answers the lines of the page.

A page MAY name none. A page that names none SHALL answer the first reading of
the order the system prefers, which is the order it answered by before a page
could name one. Every page read before this rule existed therefore reads as it
read.

A page SHALL NOT name a reading of another page, and SHALL NOT name a reading it
no longer holds. Removing the named reading SHALL leave the page naming none,
and the page SHALL answer by the preferred order again.

#### Scenario: A page that names a reading

- **WHEN** a page names its Ultra reading
- **THEN** the lines of that page come from the Ultra reading

#### Scenario: A page that names none

- **WHEN** a page names no reading and holds a line reading and a merged reading
- **THEN** the lines of that page come from the merged reading, as they did
  before

#### Scenario: The named reading is removed

- **WHEN** a normal rescan removes the reading a page named
- **THEN** the page names none
- **AND** it answers the first reading of the preferred order

#### Scenario: A page is deleted

- **WHEN** a page is removed
- **THEN** the name it held is removed with it

### Requirement: A better reading clears the choice of a reader

The system SHALL rank the readings a page can answer. Ultra outranks the merged
reading of PaddleOCR-VL, which outranks the line reading.

When the system stores a reading that outranks the one a page names, it SHALL leave
that page naming nothing. The page then answers the best reading it holds, as though
nothing had ever been chosen for it.

A reader chose a reading because it was the best the page held. A better one arriving
answers the same want, and leaving the older choice standing would hide the very
reading the reader would have picked.

A reading that does not outrank the one a page names SHALL leave that name alone. So
SHALL the same reading stored again, which is no better than itself.

A page that names nothing SHALL be left alone. It answers by rank already, so a
better reading arriving answers for it with nothing written.

A page SHALL name a reading because a reader chose it and for no other reason. The
system SHALL NOT name one of its own accord, so a page that names a reading is a page
a reader chose for.

#### Scenario: An Ultra rescan of a page answering the merged reading

- **WHEN** a page answers its merged reading, naming nothing, and an Ultra rescan
  stores a better ranked reading
- **THEN** the page answers the Ultra reading
- **AND** it still names nothing

#### Scenario: A normal rescan of a page answering Ultra

- **WHEN** a page answers its Ultra reading and a normal rescan stores a line
  reading
- **THEN** the page still answers the Ultra reading

#### Scenario: A reader chose the line reading

- **WHEN** a reader chooses the line reading of a page
- **AND** a later Ultra rescan stores a reading of that page
- **THEN** the page names nothing
- **AND** it answers the Ultra reading

#### Scenario: A reader chose the best reading of a page

- **WHEN** a reader chooses the Ultra reading of a page and a later rescan stores a
  merged PaddleOCR-VL reading
- **THEN** the page still names the Ultra reading and answers it

#### Scenario: A page that has never been chosen for

- **WHEN** a rescan stores the first merged reading of a page that answers its
  line reading
- **THEN** the page answers the merged reading
- **AND** it names nothing

### Requirement: A reader chooses which reading of a page answers

The system SHALL accept a request naming a page and one of the readings it
holds, and SHALL make that page answer that reading.

It SHALL refuse a request naming a reading the page does not hold, and one
naming a reading a renderer cannot draw. It SHALL name the reason in both cases.

A reading a renderer cannot draw SHALL NOT be offered and SHALL NOT be chosen.
The PaddleOCR-VL result and the Ultra result hold reflowed text and no lines, so
only the line reading and the merged readings answer.

Choosing SHALL be the only way a page comes to name a reading.

#### Scenario: A reader chooses the line reading

- **WHEN** a client asks that a page answer its line reading
- **THEN** the page answers the line reading
- **AND** it names that reading until a better one arrives

#### Scenario: A reader chooses a reading the page does not hold

- **WHEN** a client asks that a page answer a reading it holds no result for
- **THEN** the system refuses the request and names the reason
- **AND** the page answers what it answered

#### Scenario: A reader chooses a reading with no lines

- **WHEN** a client asks that a page answer the raw PaddleOCR-VL result
- **THEN** the system refuses the request and names the reason

### Requirement: The system reports the readings a page can answer

The answer that holds the lines of a page SHALL report every reading that page
can answer, and SHALL say which one answers now.

Every reported reading SHALL name the profile that made it, so a reader sees
"Normal", "Extra" or "Ultra" and not the name of a stored model. The line
reading is Normal, the merged PaddleOCR-VL reading is Extra, and the merged
Ultra reading is Ultra.

The answer SHALL report a reading a renderer cannot draw not at all.

A page holding one reading SHALL report that one. A reader can choose it, and
choosing it changes nothing but the mark.

#### Scenario: A page read three times

- **WHEN** a client asks for the lines of a page holding the line reading, the
  merged PaddleOCR-VL reading and the merged Ultra reading
- **THEN** the answer reports three readings, named Normal, Extra and Ultra
- **AND** it says which of them answers

#### Scenario: A page read once

- **WHEN** a client asks for the lines of a page holding the line reading alone
- **THEN** the answer reports one reading, named Normal
- **AND** it says that reading answers

#### Scenario: A reader looks through the readings of a doubted page

- **WHEN** a reader chooses the line reading of a doubted page and then its merged
  reading again
- **THEN** the page still says it disagrees, and still says why
- **AND** the reader is told the same thing it was told before

#### Scenario: The raw readings are not reported

- **WHEN** a page holds a raw PaddleOCR-VL result and a raw Ultra result beside
  its merged readings
- **THEN** the answer reports neither raw result
