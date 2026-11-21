## MODIFIED Requirements

### Requirement: A merge pairs the lines of a box engine with the blocks of a text engine

The merge SHALL take two readings of one page. The box engine reading SHALL be an
ordered list of lines, each carrying a box, the corners of the line as that engine
found them, and text. The text engine reading SHALL be a list of blocks, each
carrying a box and text.

The corners are what report the slope of a line. A box is a rectangle, so a line
that slopes across the page cannot say so through its box alone.

The merge SHALL answer one line for every line of the box engine, in the order the
box engine gave them. The merge SHALL NOT reorder the lines.

The merge SHALL change a box in one case only: it SHALL level the box of a printed
row onto the row grid of its page. The requirement that levels a row onto the row
grid governs that case. A line the merge does not level SHALL carry the box the
box engine gave.

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
- **AND** line 5 of the answer carries the text of line 5 of the box engine
- **AND** the lines follow the order the box engine gave

#### Scenario: A flat page keeps every box

- **WHEN** the merge runs on a page whose lines report no slope
- **THEN** every line of the answer carries the box the box engine gave

#### Scenario: The text engine read nothing

- **WHEN** the text engine reading holds no block
- **THEN** every line of the answer holds the text of the box engine
- **AND** every line names the box engine as its source

#### Scenario: A page the merge added and removed lines on

- **WHEN** the merge adds 2 lines for printed rows the box engine missed
- **AND** it discards 14 boxes of one disaster block
- **THEN** it reports both counts
- **AND** the lines the box engine read elsewhere keep their order

## ADDED Requirements

### Requirement: The merge levels a printed row onto the row grid of its page

The merge SHALL answer the lines of one printed row on one baseline and at one
height. It SHALL group the lines of a printed row by the same rule the reading
order rests on, so the two cannot disagree about what a row is.

Every line of a row SHALL keep the left edge and the right edge the box engine
gave it. Only the top edge and the bottom edge SHALL move. A levelled line
therefore covers the same column of the page it always did, and no line covers ink
that is not its own.

The merge SHALL read the baseline of a row from the corners of every line of that
row, at the horizontal middle of the row. A horizontal box over a sloping row
stands off the ink at both ends; reading at the middle SHALL make that error
alike at each end instead of laying it all on one.

The height of a row SHALL be the height of its box less the slope of the row times
the width of that box. A rectangle drawn round sloping ink stands taller than the
ink by exactly that much.

The height SHALL NOT exceed the distance from one printed row to the next. Two
printed rows do not overlap, so no row is taller than the distance to the next.
This is a bound and not a size: a row measuring under it SHALL keep what it
measured.

The merge SHALL report how many rows it levelled.

#### Scenario: A row the box engine read in pieces

- **WHEN** a printed row is read as `viaje;` in the box 112, 375, 261, 461 and
  `etcétera. Incluso su figura` in the box 231, 336, 1735, 459
- **THEN** both lines of the answer carry one top edge and one bottom edge
- **AND** the first still begins at 112 and ends at 261
- **AND** the second still begins at 231 and ends at 1735

#### Scenario: A sloping row stands no taller than the row spacing

- **WHEN** a page whose printed rows stand 80 apart holds a row 124 tall that
  slopes by 0.014 over a width of 1641
- **THEN** the height of that row in the answer is 80
- **AND** it is not 124, and not the 102 the slope alone accounts for

#### Scenario: A row that measures under the bound keeps what it measured

- **WHEN** a page whose printed rows stand 53 apart holds a row 44 tall that does
  not slope
- **THEN** the height of that row in the answer is 44

#### Scenario: A flat page is left alone

- **WHEN** every row of a page measures under the distance to the next row
- **THEN** the answer carries the boxes the box engine gave
- **AND** the merge reports that it levelled no row

#### Scenario: A page whose row spacing cannot be measured

- **WHEN** the merge cannot measure the distance from one printed row to the next
- **THEN** every line keeps the box the box engine gave

#### Scenario: A line that reports no corners

- **WHEN** a line carries no corners, as a result stored before the merge read
  them does
- **THEN** that line keeps the box the box engine gave

#### Scenario: The merge reports what it levelled

- **WHEN** the merge levels 34 rows of a page
- **THEN** it reports the count 34
