## MODIFIED Requirements

### Requirement: The merge reads the lines of a block in reading order

The merge SHALL sort the lines of one block into reading order before it cuts the
block. It SHALL group lines into rows by the top edge of the box, with a tolerance
taken from the median line height of the group, and SHALL order the lines of one
row by the left edge.

Two boxes SHALL be read as two halves of one printed row when they share at most
0.40 of the narrower box's column, and stand at each other's height. Two boxes
that share more of the column than that SHALL be read as two rows.

The share is what the measure of a row rests on, and a detector draws the boxes of
one row overlapping. Over the measured books the share is bimodal: two halves of
one printed row share up to 0.37 of the narrower column, two different rows share
all of it, and nothing falls between. A tolerance below 0.37 splits a row in two,
and then the box above it takes the head of that row on top of its own text and
the page carries the same words twice.

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

#### Scenario: Three boxes of one row, drawn overlapping

- **WHEN** a printed row reads `pero no en el sentido de que la actuación de
  Alcibíades` over three boxes
- **AND** the box holding `en` shares 0.37 of its column with the box holding
  `el sentido`
- **THEN** the merge reads all three as one row
- **AND** each of the three holds the text printed in it and nothing more
- **AND** no line of the page holds `pero no` a second time

#### Scenario: Two boxes of two rows that share a column

- **WHEN** two boxes share the whole column of the narrower
- **THEN** the merge reads them as two rows, whatever their height says

#### Scenario: A block whose lines slope past its row spacing

- **WHEN** the lines of a block slope far enough that the top edge of a full-width
  box stands a whole printed row from the row its line belongs to
- **THEN** the merge derives no reading order for that block
- **AND** it cuts the text of that block at no line boundary
