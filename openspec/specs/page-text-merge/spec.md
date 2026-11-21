## Purpose

Joins the line boxes of one OCR engine with the text of another, so a page keeps
the boxes a renderer needs and the characters the better reader recognised. It
also decides when the two engines disagree too much to be joined at all.

## Requirements

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

#### Scenario: A page the merge levels no row of keeps every box

- **WHEN** the merge runs on a page and levels no row of it
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

### Requirement: The merge cuts the text of a block at the line boundaries

The merge SHALL align the text of the block against the text of its lines joined
in order. The comparison SHALL be fuzzy: it SHALL treat two characters as the same
when they match after Unicode decomposition, removal of combining marks and case
folding, and SHALL treat any whitespace as matching any whitespace.

The merge SHALL then cut the text of the block at the line boundaries the alignment
gives. The pieces SHALL partition the text of the block exactly: read in order,
they SHALL be the text of the block, character for character. No character SHALL
land in two pieces, and no character between two pieces SHALL be left out.

Where two boxes of one printed row leave ink uncovered between them, the merge
SHALL divide the piece of the box before the gap between that box and a box it
draws over the gap. The two together SHALL hold that piece character for
character, so the pieces still partition the block.

Three tests SHALL all hold before the merge divides a piece that way.

- The gap SHALL have room for at least 2 characters at the density of its block.
  A narrower gap is the space between two words, which every printed row has.
- The gap SHALL be no wider than the box beside it.
- The piece SHALL hold at least 1.5 times what that box was drawn around.

The merge divides the piece by width, which holds only where the ink runs at one
density across the box and the gap. A gap wider than the box it is measured from
carries no such measurement: the gaps of a contents page are rows of leader dots,
sparse ink over as much as 1011 pixels beside a box of 189, and dividing that by
width moves letters of a title into the dots.

The third test is what says the gap holds ink at all. A piece that fits the box it
was cut for says the gap held nothing worth a box.

The box the merge draws over a gap SHALL span the gap exactly, from the right edge
of one box to the left edge of the next. It SHALL stand where the printed row
stands there, read off the two boxes beside it. It SHALL name the text engine,
SHALL carry no score, and SHALL be marked as a line no engine read.

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

#### Scenario: Ink the box engine left uncovered inside a row

- **WHEN** a printed row reads `otro—, tiene deseos costosos` over a box 163
  pixels wide holding `otro—,` and a box beginning 118 pixels further along
- **AND** the piece of the first box holds `otro—, tien`, which is 11 characters
  in room for 6
- **THEN** the merge draws a box over those 118 pixels
- **AND** the first box holds `otro—,` and the new box holds `tien`
- **AND** the new box names the text engine, carries no score, and is marked as a
  line no engine read
- **AND** the three boxes read in order hold the row character for character

#### Scenario: A gap that is the space between two words

- **WHEN** two boxes of one printed row leave a gap with room for one character
- **THEN** the merge draws no box over it

#### Scenario: A gap wider than the box beside it

- **WHEN** a box 189 pixels wide stands beside a gap of 1011 pixels
- **AND** its piece holds 1.71 times what the box was drawn around
- **THEN** the merge draws no box over the gap, because the width of the gap says
  nothing about how much of the piece stands in it

#### Scenario: A gap beside a box whose piece fits it

- **WHEN** two boxes of one printed row leave a gap with room for 5 characters
- **AND** the piece of the first box holds what that box was drawn around
- **THEN** the merge draws no box over the gap

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

The merge SHALL leave a row alone when it holds one line and that line stands no
taller than the distance to the next printed row. Such a row already draws as one
straight line of a size the page allows. Moving it by the pixel or two its own
slight slope accounts for would change nothing a reader can see, and every stored
result would have to be written again for it.

A row read in pieces SHALL be levelled whether or not it stands under that bound,
because pieces that keep their own geometry do not line up however short they are.

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

#### Scenario: A page that needs no levelling keeps every box

- **WHEN** every printed row of a page is read as one line
- **AND** every one of those lines stands no taller than the distance to the next
  row
- **THEN** the answer carries the boxes the box engine gave
- **AND** the merge reports that it levelled no row

#### Scenario: A page that does not slope but holds a row read in pieces

- **WHEN** a page whose rows stand 53 apart holds one printed row read as two
  boxes, and every other row is read as one line under that bound
- **THEN** the merge levels that row and no other
- **AND** it reports that it levelled one row

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
