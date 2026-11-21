## Purpose

Joins the line boxes of one OCR engine with the text of another, so a page keeps
the boxes a renderer needs and the characters the better reader recognised. It
also decides when the two engines disagree too much to be joined at all.

## ADDED Requirements

### Requirement: A merge pairs the lines of a box engine with the blocks of a text engine

The merge SHALL take two readings of one page. The box engine reading SHALL be an
ordered list of lines, each carrying a box and text. The text engine reading SHALL
be a list of blocks, each carrying a box and text.

The merge SHALL answer one line for every line of the box engine, in the order the
box engine gave them, carrying the box the box engine gave. The merge SHALL NOT
add a line, remove a line, reorder the lines, or change a box.

The merge SHALL name the source of the text of every line it answers, so a caller
can tell a line filled from the text engine from a line that kept its own.

The merge SHALL depend on no property of any script, language or alphabet. It
SHALL hold no list of characters, and SHALL treat every character alike.

#### Scenario: The answer follows the box engine

- **WHEN** the merge runs on a page whose box engine reading holds 22 lines
- **THEN** it answers 22 lines
- **AND** line 5 of the answer carries the box of line 5 of the box engine
- **AND** the lines follow the order the box engine gave

#### Scenario: The text engine read nothing

- **WHEN** the text engine reading holds no block
- **THEN** every line of the answer holds the text of the box engine
- **AND** every line names the box engine as its source

### Requirement: A line takes its text from the block whose box holds it

The merge SHALL assign a line to the block whose box covers the largest part of
the box of that line. It SHALL assign no line to a block that covers less than
half the box of that line.

A line that no block covers SHALL keep the text of the box engine. Such a line is
text the text engine did not read at all, and the merge SHALL NOT fill it from a
block that lies elsewhere on the page.

Geometry SHALL decide the pairing, not order. The two engines can walk a page in
different orders, and a page with two columns or with footnotes gives each engine
a reason to disagree.

#### Scenario: A line inside one block

- **WHEN** a block covers the whole box of a line
- **THEN** that line takes its text from that block

#### Scenario: A line in the margin

- **WHEN** a line lies outside every block of the text engine
- **THEN** that line holds the text of the box engine
- **AND** it names the box engine as its source

#### Scenario: A line the text engine barely touches

- **WHEN** the best block covers a third of the box of a line
- **THEN** that line holds the text of the box engine

### Requirement: The merge reads the lines of a block in reading order

The merge SHALL sort the lines of one block into reading order before it cuts the
block. It SHALL group lines into rows by the top edge of the box, with a tolerance
taken from the median line height of the group, and SHALL order the lines of one
row by the left edge.

The merge SHALL NOT trust the order of the box engine for this. A box engine can
report its lines in detection order, which interleaves lines from different parts
of the page.

#### Scenario: The box engine reports lines out of order

- **WHEN** a block holds three lines the box engine reported bottom to top
- **THEN** the merge cuts the block top to bottom
- **AND** the topmost line takes the first part of the block text

#### Scenario: Two lines on one row

- **WHEN** a block holds two lines whose boxes share a row
- **THEN** the merge orders them left to right

### Requirement: The merge cuts the text of a block at the line boundaries

The merge SHALL align the text of the block against the text of its lines joined
in order. The comparison SHALL be fuzzy: it SHALL treat two characters as the same
when they match after Unicode decomposition, removal of combining marks and case
folding, and SHALL treat any whitespace as matching any whitespace.

The merge SHALL then cut the text of the block at the line boundaries the alignment
gives. The pieces SHALL partition the text of the block exactly: read in order,
they SHALL be the text of the block, character for character. No character SHALL
land in two pieces, and no character between two pieces SHALL be left out.

The merge SHALL keep the text of the text engine as the text engine wrote it,
markup included.

The merge SHALL collapse a run of whitespace inside a piece to one space, and SHALL
remove whitespace at both ends of a piece. A line is a line of a page, so its
leading and interior line breaks carry nothing.

#### Scenario: A block of one paragraph over five lines

- **WHEN** a block of one paragraph covers five lines of the box engine
- **THEN** the five answered lines read as the text of the block, in order
- **AND** no word of the block is missing and none appears twice

#### Scenario: A word the box engine broke across two lines

- **WHEN** the box engine ends a line with `(φiλoσo-` and starts the next with
  `φíα, philosophia)`
- **AND** the block reads `(φιλοσοφία, philosophia)`
- **THEN** the first answered line ends inside the word, as the printed line does
- **AND** the second answered line holds the rest of the word
- **AND** no hyphen remains, because the block holds none

#### Scenario: A quotation the box engine read wrong

- **WHEN** the box engine reads a Greek line as `ν τò σοφν μονον λγεσ9αι`
- **AND** the block reads it as `ἐν τὸ σοφὸν μοῦνον λέγεσθαι`
- **THEN** the answered line holds the text of the block
- **AND** it names the text engine as its source

#### Scenario: The text engine wrote markup

- **WHEN** a block reads `misma $ ^{1} $ está vinculada`
- **THEN** the answered line holds `$ ^{1} $` as the block wrote it

### Requirement: The merge refuses a line the two engines did not read alike

The merge SHALL measure, for every line, the share of the characters of the box
engine that the alignment paired with an equal character of the block. Two
characters count as equal by the same fuzzy rule the alignment uses.

The merge SHALL refuse the piece when that share is below 0.30, unless the line of
the box engine holds fewer than 20 characters. A short line carries too little
evidence to judge, so the merge SHALL accept it.

The merge SHALL also refuse a piece longer than four times its line plus 20
characters, whatever that share is. A piece that long cannot be one reading of one
line. This second test catches the line where the text engine began correctly and
then ran on: its opening agrees, so the share alone accepts it and the whole run
lands in one line.

The two tests SHALL be independent. Either one refuses the piece on its own, and
the length test SHALL hold for a short line as well, because its allowance is
measured in characters and not in shares.

A refused line SHALL keep the text of the box engine and SHALL name the box engine
as its source. The merge SHALL record the refused piece.

Folding is what keeps the measure safe. A box engine that read the base letters of
a word and missed its diacritics still agrees with the block after folding, so a
line the text engine merely improved SHALL NOT be refused. A line the two engines
read as different words agrees on almost nothing, and SHALL be refused. Without
folding the merge would refuse the very lines it exists to fix.

#### Scenario: The text engine repeated itself

- **WHEN** a block holds one phrase repeated to fill a whole page region
- **THEN** every line of that region longer than 20 characters keeps its box
  engine text
- **AND** the merge records the refused pieces

#### Scenario: The text engine answered too little

- **WHEN** a block holds 411 characters for a region whose box engine lines hold
  2044
- **THEN** the lines the block never reached keep their box engine text
- **AND** the lines the block did reach take their text from it

#### Scenario: The text engine ran on after a correct start

- **WHEN** a 64 character line agrees with its piece on 0.53 of its characters
- **AND** that piece holds 699 characters, because the text engine began the line
  correctly and then repeated itself for the rest of the block
- **THEN** the merge refuses the piece
- **AND** that line keeps its box engine text
- **AND** the merge records the refused piece

#### Scenario: A piece longer than its line but not too long

- **WHEN** the text engine writes a Greek word as a formula, so a 56 character
  line takes a piece of 134 characters
- **THEN** the merge accepts the piece

#### Scenario: A short line with a longer piece

- **WHEN** a 2 character line takes a piece of 12 characters
- **THEN** the merge accepts the piece, because the allowance in characters covers
  a short line whatever its share

#### Scenario: A short line the two engines read differently

- **WHEN** the box engine reads a 9 character line as `δoxovτα.`
- **AND** the block reads it as `δοχοῦντα.`
- **THEN** the answered line holds the text of the block

#### Scenario: A long line of a script the box engine failed at

- **WHEN** a line of 55 Greek characters agrees with its block piece on more than
  0.30 of its characters after folding
- **THEN** the answered line holds the text of the block

### Requirement: The merge never empties a line

The merge SHALL keep the text of the box engine whenever it has nothing better. A
line whose piece holds no character other than whitespace SHALL keep its box
engine text. Every line of a block whose text holds no character other than
whitespace SHALL keep its box engine text.

The merge SHALL NOT answer an empty line for a line the box engine read.

#### Scenario: A block over a figure

- **WHEN** a block whose text is empty covers two lines of the box engine
- **THEN** both answered lines hold their box engine text

#### Scenario: A line the block skipped

- **WHEN** the alignment leaves a line with an empty piece
- **THEN** that line holds its box engine text

### Requirement: The merge accounts for every character of the text engine

The merge SHALL report the text of every block it did not render into a line, with
the reason. The reasons SHALL be a refused line, a block no line was assigned to,
and a block the merge would not align.

For every page, the characters other than whitespace of every block SHALL equal
the characters other than whitespace of the answered lines that name the text
engine, plus the characters other than whitespace the merge reported as dropped.
The merge SHALL lose no character in silence.

The merge SHALL report the count of lines it filled from the text engine, the
count of lines that kept their box engine text, and the count of characters it
dropped.

#### Scenario: A clean page

- **WHEN** every block of a page merges
- **THEN** the merge reports no dropped text
- **AND** the answered lines hold every character of every block

#### Scenario: A block no line was assigned to

- **WHEN** a block holding `2023` lies where the box engine found no line
- **THEN** the merge reports `2023` as dropped
- **AND** no answered line holds it

#### Scenario: The accounting of a whole book

- **WHEN** the merge runs on all 124 pages of a book
- **THEN** the rendered characters plus the dropped characters equal the
  characters of every block of every page

### Requirement: The merge refuses a block it cannot align at a bounded cost

The merge SHALL refuse a block whose text and lines are too far apart in length to
be one reading of one region, and SHALL refuse a block whose alignment would cost
more work than a page is worth. A refused block SHALL leave every one of its lines
with the box engine text, and its text SHALL be reported as dropped.

Alignment costs the product of the two lengths. A text engine that repeats itself
without end would otherwise stall a whole book on one page.

#### Scenario: A block far longer than its lines

- **WHEN** a block holds more than eight times the characters of its lines
- **THEN** the merge refuses the block
- **AND** every line of that block keeps its box engine text
- **AND** the merge reports the text of the block as dropped

#### Scenario: A block far shorter than its lines

- **WHEN** the lines of a block hold more than eight times the characters of the
  block
- **THEN** the merge refuses the block
