# page-reading-health Specification

## Purpose
Says whether the reading a page answers disagrees with the line reading of that
page. The line engine cannot loop, cannot invent a script, and cannot abandon a
page, so where the two readings part company the visual reading is the suspect
one.

## Requirements

### Requirement: The system judges a reading against the line reading of its page

The system SHALL compare the reading a page answers with the PP-OCRv6 result of
that page, and SHALL answer whether they disagree.

It SHALL judge that reading by the text the engine behind it wrote. A merged
reading carries the text of its reading engine only where that text aligned with
a line box, and PP-OCRv6 text everywhere else, so judging the merged text against
the PP-OCRv6 text would compare PP-OCRv6 with itself. A reading engine that
abandoned the page would pass, because the merge already replaced what it
abandoned. The system SHALL therefore judge the raw text of the reading engine,
and a page answering its line reading holds no such text and SHALL NOT disagree.

That raw text SHALL be everything the reading engine read of the page. The
markdown of PaddleOCR-VL is not everything: it leaves out the footnote and the
page number blocks, which its full result holds. Judging the markdown would call
every page carrying a footnote truncated, and each of them is a page that engine
read whole.

The judge SHALL read the prose of both readings alone. A reading holds markup
the engine wrote and no reader speaks: image placeholders for a plate, HTML for
a table, TeX for a footnote mark. None of it is reading, and all of it repeats,
so the system SHALL remove it before it measures anything.

A page SHALL disagree when any of these holds:

- The reading keeps less than 60% of the words of four letters or more that the
  line reading holds.
- The reading holds less than 60% of the characters the line reading holds.
- The reading repeats more than 25% more of itself than the line reading of that
  page repeats, counting the share of its 15 character runs that it holds more
  than once.
- The reading holds a script the page is not written in, by the rule below.

The excess and not the share SHALL decide the repetition. A page can repeat
itself: front matter naming three texts published by the same press in three
years repeats a third of itself, and so does the line reading of it, because the
repetition is printed on the page. A reading that degenerated repeats what its
line reading does not.

A page whose line reading holds fewer than 400 letters SHALL be judged by the
repetition and the script alone. A plate, a half title or a blank leaf holds too
little text for a share of it to mean anything.

A page that holds no line reading SHALL NOT disagree. There is nothing to judge
it against.

#### Scenario: A reading that abandoned the page

- **WHEN** the line reading of a page holds 2148 characters and the reading it
  answers holds 110
- **THEN** the page disagrees

#### Scenario: A reading that lost the wording

- **WHEN** a reading keeps 45% of the words the line reading holds
- **THEN** the page disagrees

#### Scenario: A reading that repeats itself

- **WHEN** a reading fills half its length with one phrase repeated
- **AND** the line reading of that page repeats none of itself
- **THEN** the page disagrees

#### Scenario: A page that repeats itself

- **WHEN** a page of front matter names three texts published by the same press
  in three years, so a third of its reading repeats
- **AND** the line reading of that page repeats as much
- **THEN** the page does not disagree on the repetition

#### Scenario: A page whose footnotes the markdown leaves out

- **WHEN** a page holds a reading whose markdown drops the footnotes it read and
  whose full result holds them
- **THEN** the judge reads the footnotes with the body
- **AND** the page does not disagree for the length the markdown lost

#### Scenario: A reading that agrees

- **WHEN** a reading keeps 85% of the words of the line reading and holds a
  similar number of characters
- **THEN** the page does not disagree

#### Scenario: A plate

- **WHEN** the line reading of a page holds 13 letters and the reading it
  answers holds 182
- **THEN** the share of the wording and the share of the length judge nothing
- **AND** the page disagrees only if its reading repeats itself or holds a script
  the page is not written in

#### Scenario: A merged reading whose engine abandoned the page

- **WHEN** a page answers a merged reading whose reading engine wrote 110
  characters for a page the line engine read 2148 characters of
- **THEN** the page disagrees
- **AND** the length of the merged reading, which the merge filled out with the
  line text, does not decide it

#### Scenario: The markup of a cover page

- **WHEN** a reading of a cover holds image placeholders whose markup repeats
- **THEN** the system removes the markup before it measures the repetition
- **AND** the page does not disagree on that markup alone

### Requirement: The script a page is not written in is decided by that page

The system SHALL NOT hold a list of forbidden scripts. A book written in a script
is not a book failing to be written in another.

A script SHALL count as one the page is not written in when both of these hold:

- The reading the page answers holds characters of that script.
- The line reading of that page holds none.

The two readings of the one page SHALL decide it. What the other pages of the
book hold SHALL NOT enter the test.

A reading SHALL make its page disagree when it holds 10 characters or more of such
scripts, counting every one of them together.

Below that it SHALL NOT. A letter or two of another alphabet is a letter the reading
engine mistook, not a reading that turned: the merge drops such a line for
disagreeing with the line reading, so the page reads correctly and calling it
doubted is noise. A reading that turned to another script turns a passage. Over the
read pages of books 17, 18 and 24 the stray readings hold 1 to 8 such characters and
the collapsed ones hold 26, 30, 94 and 119.

The counts of every foreign script SHALL be read together and not one at a time. A
reading that turned a few characters of each of two scripts is as broken as one that
turned wholly into one.

#### Scenario: A Greek book whose page turns Cyrillic

- **WHEN** a page of a book written in Spanish and Greek answers a reading
  holding 119 Cyrillic characters
- **AND** the line reading of that page holds none
- **THEN** the page disagrees

#### Scenario: A page written in Cyrillic

- **WHEN** the line reading of a page holds Cyrillic
- **THEN** that page does not disagree on the script, whatever the rest of the
  book holds
- **AND** the system judges it by the wording, the length and the repetition as
  it judges any other page

#### Scenario: One substituted letter

- **WHEN** a reading holds one character of a script the page is not written in
- **THEN** the page does not disagree on the script

#### Scenario: A passage in another script

- **WHEN** a reading holds a passage of 23 characters of a script the line reading
  of that page holds none of
- **THEN** the page disagrees

#### Scenario: A reading that turned into two scripts

- **WHEN** a reading holds 6 characters of one foreign script and 5 of another
- **THEN** the page disagrees
- **AND** the system names both scripts

#### Scenario: A script the line engine cannot read

- **WHEN** a book is written in a script the line engine reads as nothing, so no
  line reading of that book holds it
- **AND** the line readings still hold the page numbers
- **THEN** every such page holding 10 characters or more of it disagrees on the
  script
- **AND** the system reports them, which the rule accepts: one page cannot tell a
  line engine that reads nothing from a reading that turned to another script

### Requirement: The system says why a page disagrees

The system SHALL answer, for every page that disagrees, why it does, in words a
reader reads. It SHALL say what was measured and not which test fired.

Every reason SHALL carry the number that decided it: the share of the wording or of
the text the reading keeps, the share of the reading that repeats, and the count of
the characters of a script beside the name of that script.

A reason SHALL be short enough to read at a glance, because a reader reads it in a
menu. It SHALL name the fault and its number and nothing else.

A page that holds a script the line reading holds none of SHALL answer one reason
per such script.

A page judged by the repetition and the script alone SHALL answer no share of the
wording and no share of the text, because neither was measured.

A page that does not disagree SHALL answer no reason.

#### Scenario: A page that turned Cyrillic

- **WHEN** a page answers a reading holding 119 Cyrillic characters its line
  reading holds none of
- **THEN** the reason reads "119 unexpected Cyrillic characters"

#### Scenario: A page holding two foreign scripts

- **WHEN** a reading holds characters of two scripts its line reading holds none of
- **THEN** the system answers one reason for each

#### Scenario: A page that stopped early

- **WHEN** a reading holds 5% of the text of its line reading
- **THEN** one reason reads "Contains only 5% of the text"

#### Scenario: A plate

- **WHEN** a page whose line reading holds 13 letters disagrees on its script
- **THEN** no reason names a share of the wording or of the text

#### Scenario: A page that agrees

- **WHEN** a reading agrees with the line reading of its page
- **THEN** the system answers no reason

### Requirement: The book answer reports which pages disagree

The answer that holds the read lines of a whole book SHALL report, for every
page, whether that page disagrees and why. It SHALL report the number of
disagreeing pages of the book.

The system SHALL judge the reading each page answers, not a reading of a fixed
model. A page whose answering reading changes SHALL be judged again by the next
answer.

A book whose pages hold one reading each SHALL report no disagreement. One
engine cannot disagree with itself.

#### Scenario: A book of 124 pages

- **WHEN** a client asks for the lines of a book whose pages hold a line reading
  and a visual reading
- **THEN** the answer reports a count of the disagreeing pages
- **AND** every page of the answer says whether it disagrees

#### Scenario: A book read with the normal profile

- **WHEN** a client asks for the lines of a book whose every page holds the line
  reading alone
- **THEN** the answer reports a count of zero
- **AND** no page says it disagrees

#### Scenario: The reading of a page changes

- **WHEN** a user chooses another reading of a page that disagreed
- **AND** a client asks for the lines of that book again
- **THEN** the answer judges the reading the page now answers
