## MODIFIED Requirements

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
