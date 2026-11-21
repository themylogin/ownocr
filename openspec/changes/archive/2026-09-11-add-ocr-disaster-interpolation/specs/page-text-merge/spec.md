## ADDED Requirements

### Requirement: The merge replaces the boxes of a region it cannot order

The merge SHALL decide, for every block, whether the boxes of the box engine can
say where the printed rows of that block are. It SHALL call the block a disaster
when they cannot, and SHALL then answer the text of the block in boxes of its own
instead of the boxes the box engine reported.

Two tests SHALL both hold before the merge calls a block a disaster.

The first test SHALL find every pair of boxes of the block that stand beside each
other in the column, and SHALL measure how far apart the two top edges of such a
pair stand. A rectangle drawn round a sloping line reaches the highest point of
that line, so its top edge stands above the ink by however far the line rises
from that point to its own end. Two boxes covering different stretches of the
column therefore carry different errors, and the difference is the slope of the
block times the distance between the points at which each box reaches highest.

The merge SHALL call the boxes unusable when that difference, for any such pair,
reaches the distance from one printed row to the next.

At that difference the two boxes stand a whole row apart in a grouping that
should have put them in one row. Rows are grouped by the top edge, so no order
derived from the rectangles can be right. The threshold is a property of the page
and not a measured preference.

Only boxes that stand beside each other SHALL be compared, because only they can
be two halves of one printed row. That is what tells a broken row from a
paragraph that ends in a short line. Both are a short box in a block of long
ones. The short box of a broken row shares no column with its own row mate, and
the last line of a paragraph shares its whole column with the line above it.

The distance from one printed row to the next SHALL come from the whole page and
not from the rows of the block under test. A block of two rows spans one step,
and one step is no measurement.

The second test SHALL compare the characters other than whitespace of the block
against those of the lines of the box engine assigned to it. The merge SHALL
require the block to hold between 0.7 and 1.4 times what those lines hold.

The second test says which reading is broken. Two engines that read the same
pixels and report the same amount of text disagree only about where that text
goes, and the geometry says the box engine is the one that cannot place it. A
text engine that collapsed reports far too little text or repeats itself without
end, and the merge SHALL leave such a block to the requirement that refuses a
line the two engines did not read alike.

For a disaster block the merge SHALL derive the number of printed rows two ways,
and SHALL interpolate nothing unless both ways give the same number.

- From the text: the characters of the block against the characters a printed row
  of the page holds.
- From the geometry: the height of the block, less the slope times its width,
  against the distance from one printed row to the next.

Both the characters a printed row holds and the distance from one row to the next
SHALL come from the blocks of the same page that the merge did not call a
disaster. A page that holds no such block SHALL get no interpolation, and its
disaster blocks SHALL keep the boxes and the text the box engine reported.

The merge SHALL split the text of the block into that number of lines without
breaking a word, SHALL give every line the width of the block, and SHALL space
the lines evenly from the top of the block to its bottom. Every line it answers
this way SHALL name the text engine as the source of its text, SHALL carry no
score, and SHALL be marked as a line no engine read.

The merge SHALL discard the lines of the box engine that the block owns, and
SHALL report how many it discarded.

A disaster block the merge cannot interpolate SHALL keep every box and every
character the box engine gave it, and its own text SHALL reach no line. The merge
SHALL report that text as dropped, with a reason of its own that names an order it
could not derive. Cutting the block in an order the merge has already judged wrong
would write the text of one printed row into the box of another on purpose.

#### Scenario: A block whose boxes name the wrong rows

- **WHEN** a block holds a box reaching 276 pixels beside a box reaching 1769
- **AND** the lines of the block slope 0.086
- **AND** the printed rows of the page stand 81 pixels apart
- **AND** the block holds 1.02 times the characters its lines hold
- **THEN** the two top edges stand 1.66 printed rows apart
- **AND** the merge calls the block a disaster
- **AND** the answer holds no box the box engine reported for that block
- **AND** the merge reports the count of boxes it discarded

#### Scenario: A paragraph that ends in a short line

- **WHEN** a sloping block of long lines ends in a line a fifth of their width
- **AND** that short line shares its whole column with the line above it
- **THEN** the merge does not call the block a disaster
- **AND** every line of that block keeps the box the box engine reported

#### Scenario: A block of two rows

- **WHEN** a block holds two printed rows, so its own rows span one step
- **THEN** the merge measures the row distance on the rest of the page
- **AND** the one step of that block decides nothing

#### Scenario: The interpolated lines carry the text of the block in order

- **WHEN** the merge interpolates a disaster block of 453 characters
- **AND** a printed row of that page holds 56 characters
- **AND** the block is 765 pixels tall and its rows stand 81 pixels apart
- **THEN** the merge answers 8 lines
- **AND** read in order they hold the text of the block, character for character
- **AND** every one spans the width of the block
- **AND** every one names the text engine and carries no score
- **AND** every one is marked as a line no engine read

#### Scenario: A sloping block whose every printed row is one box

- **WHEN** a block of three whole rows slopes 0.066 across 1540 pixels
- **AND** its printed rows stand 76 pixels apart
- **THEN** the merge does not call the block a disaster, because no two of its
  boxes stand beside each other
- **AND** every line of that block keeps the box the box engine reported

Every box of such a block carries the same error, so their order holds however
far the block slopes. This block slopes 1.34 times the row distance across its
own width and reads correctly.

#### Scenario: The text engine collapsed into repetition

- **WHEN** a block holds 1.75 times the characters of the lines assigned to it,
  because the text engine repeated one phrase to the end of the block
- **THEN** the merge does not call the block a disaster
- **AND** every line of that block keeps the box the box engine reported

#### Scenario: The text engine answered far too little

- **WHEN** a block holds 0.08 of the characters of the lines assigned to it
- **THEN** the merge does not call the block a disaster
- **AND** every line of that block keeps the box and the text the box engine gave

#### Scenario: The two line counts disagree

- **WHEN** the characters of a disaster block say 8 rows
- **AND** its height and the row distance say 11 rows
- **THEN** the merge interpolates nothing
- **AND** every line of that block keeps the box and the text the box engine gave
- **AND** the merge reports the text of the block as dropped, for an order it
  could not derive

#### Scenario: Every block of a page is a disaster

- **WHEN** the only text block of a page is a disaster
- **THEN** the merge can measure neither the row distance nor the characters a
  row holds
- **AND** it interpolates nothing
- **AND** every line of that page keeps the box and the text the box engine gave
- **AND** the merge reports the text of the block as dropped, for an order it
  could not derive

## MODIFIED Requirements

### Requirement: A merge pairs the lines of a box engine with the blocks of a text engine

The merge SHALL take two readings of one page. The box engine reading SHALL be an
ordered list of lines, each carrying a box, the corners of the line as that engine
found them, and text. The text engine reading SHALL be a list of blocks, each
carrying a box and text.

The corners are what report the slope of a line. A box is a rectangle, so a line
that slopes across the page cannot say so through its box alone.

The merge SHALL answer one line for every line of the box engine, in the order the
box engine gave them, carrying the box the box engine gave. The merge SHALL NOT
reorder the lines or change a box.

The merge SHALL add and remove a line in two cases only, and SHALL report the
count in both.

- It SHALL add a line for a printed row the box engine missed, placed from the
  spacing of the rows around it. Such a line stands between the lines it belongs
  between, and nothing else moves.
- It SHALL remove every line of a block whose boxes cannot say where the printed
  rows are, and answer lines of its own in their place. The requirement that
  replaces the boxes of a region it cannot order governs that case.

The merge SHALL name the source of the text of every line it answers, so a caller
can tell a line filled from the text engine from a line that kept its own. It
SHALL mark a line it placed itself, so a caller can tell it from a line an engine
read.

The merge SHALL depend on no property of any script, language or alphabet. It
SHALL hold no list of characters, and SHALL treat every character alike.

#### Scenario: The answer follows the box engine

- **WHEN** the merge runs on a page whose box engine reading holds 22 lines
- **AND** it misses no printed row and calls no block a disaster
- **THEN** it answers 22 lines
- **AND** line 5 of the answer carries the box of line 5 of the box engine
- **AND** the lines follow the order the box engine gave

#### Scenario: The text engine read nothing

- **WHEN** the text engine reading holds no block
- **THEN** every line of the answer holds the text of the box engine
- **AND** every line names the box engine as its source

#### Scenario: A page the merge added and removed lines on

- **WHEN** the merge adds 2 lines for printed rows the box engine missed
- **AND** it discards 14 boxes of one disaster block
- **THEN** it reports both counts
- **AND** the lines the box engine read elsewhere keep their boxes and their order

### Requirement: The merge reads the lines of a block in reading order

The merge SHALL sort the lines of one block into reading order before it cuts the
block. It SHALL group lines into rows by the top edge of the box, with a tolerance
taken from the median line height of the group, and SHALL order the lines of one
row by the left edge.

The merge SHALL NOT trust the order of the box engine for this. A box engine can
report its lines in detection order, which interleaves lines from different parts
of the page.

The top edge of a box says which row a line belongs to only while the lines of the
block lie level enough. The merge SHALL NOT derive a reading order for a block
whose lines slope too far for their top edges to name their rows. It SHALL treat
such a block under the requirement that replaces the boxes of a region it cannot
order.

#### Scenario: The box engine reports lines out of order

- **WHEN** a block holds three lines the box engine reported bottom to top
- **THEN** the merge cuts the block top to bottom
- **AND** the topmost line takes the first part of the block text

#### Scenario: Two lines on one row

- **WHEN** a block holds two lines whose boxes share a row
- **THEN** the merge orders them left to right

#### Scenario: A block whose lines slope past its row spacing

- **WHEN** the lines of a block slope far enough that the top edge of a full-width
  box stands a whole printed row from the row its line belongs to
- **THEN** the merge derives no reading order for that block
- **AND** it cuts the text of that block at no line boundary

### Requirement: The merge accounts for every character of the text engine

The merge SHALL report the text of every block it did not render into a line, with
the reason. The reasons SHALL be a refused line, a block no line was assigned to,
a block the merge would not align, and a block whose printed rows the merge could
neither order nor count.

For every page, the characters other than whitespace of every block SHALL equal
the characters other than whitespace of the answered lines that name the text
engine, plus the characters other than whitespace the merge reported as dropped.
The merge SHALL lose no character in silence.

That equality SHALL hold for a disaster block as well. The lines the merge
interpolates hold the text of the block character for character, so a block the
merge takes the boxes away from loses no character of the text engine.

The merge SHALL report the count of lines it filled from the text engine, the
count of lines that kept their box engine text, the count of lines it placed
itself, the count of boxes it discarded, and the count of characters it dropped.

The count of discarded boxes is what keeps the loss visible. A disaster block
throws away everything the box engine read of a region, and a page SHALL NOT lose
a box in silence.

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

#### Scenario: The accounting of a page holding a disaster block

- **WHEN** the merge interpolates a disaster block of 453 characters
- **THEN** the interpolated lines hold those 453 characters, whitespace aside
- **AND** the merge reports no character of that block as dropped
- **AND** it reports the count of boxes it discarded
