## Purpose

Finds the strip of a split page that holds the ink of the page beside it. A
spread is cut at a column and its gutter is a curve, so each half carries a
wedge of its neighbour. This reads the lines of one page and answers the column
that cuts that wedge off, or nothing when the page holds no such strip.

## ADDED Requirements

### Requirement: The trim reads one page and nothing else

The trim SHALL take the lines of one page, the width of that page, and the cut
side of that page. Every line SHALL carry a box of four numbers: left, top,
right and bottom, counting in the pixels of that page.

The trim SHALL answer one column, or nothing. The column SHALL count as a
distance from the gutter edge, so the answer does not depend on the side.

The trim SHALL read no file, SHALL call no engine, and SHALL know no alphabet.
It SHALL decide from geometry alone. The same lines, width and side SHALL always
give the same answer.

#### Scenario: The answer is a distance from the gutter edge

- **WHEN** a caller passes the lines of a page 2083 pixels wide whose cut side is
  `right`
- **THEN** the answer is a distance measured from the left edge of that page

#### Scenario: A page with no lines

- **WHEN** a caller passes an empty list of lines
- **THEN** the answer is nothing

### Requirement: The cut side names the gutter edge

The cut side of a page names which half of a spread that page is. The gutter edge
SHALL be the opposite edge, because the fold stands between the two halves. A
page cut as the `right` half SHALL be measured from its left edge, and a page cut
as the `left` half from its right edge.

The trim SHALL name the gutter edge in its answer, not the cut side, so a
renderer knows which edge to cover without knowing how the page was cut.

The near inset of a line SHALL be the distance from the gutter edge to the
nearer side of its box. The far inset SHALL be the distance from that edge to the
farther side of its box. Every rule below SHALL be written in insets, so one rule
serves both sides.

A page that carries no cut side is not a half of a spread. The trim SHALL answer
nothing for it.

#### Scenario: A right half measures from its left edge

- **WHEN** a page 2083 pixels wide was cut as the `right` half
- **AND** a line has the box 0, 273, 55, 331
- **THEN** the near inset of that line is 0 and its far inset is 55
- **AND** a trim of that page lies against its left edge

#### Scenario: A left half measures from its right edge

- **WHEN** a page 2067 pixels wide was cut as the `left` half
- **AND** a line has the box 2003, 2667, 2064, 2728
- **THEN** the near inset of that line is 3 and its far inset is 64
- **AND** a trim of that page lies against its right edge

#### Scenario: A page that was never cut out of a spread

- **WHEN** a caller passes a page that carries no cut side
- **THEN** the answer is nothing

### Requirement: A fragment touches the gutter edge and is short

A fragment SHALL be a line whose near inset is at most 0.005 of the page width
and whose far inset is at most 0.10 of the page width. A line that fails either
test SHALL NOT be a fragment.

The cut truncates a word of the neighbouring page in the middle of its strokes,
so the box the detector draws around what is left SHALL reach the gutter edge. A
line of this page begins at the margin of this page, which is far from that edge.
Touching the edge is therefore what tells the two apart, and shortness alone is
not: a page photographed at an angle makes the detector break its own printed
rows in half, and the left part of a broken row is short and stands in a column
against the inner edge without ever reaching it.

#### Scenario: A truncated word of the neighbouring page

- **WHEN** a line of a page 2083 pixels wide has the near inset 0 and the far
  inset 55, and holds the text `ar-`
- **THEN** that line is a fragment

#### Scenario: The left part of a broken row

- **WHEN** a page 2107 pixels wide is photographed at an angle, so the detector
  breaks seven printed rows in half
- **AND** the left part of each broken row has a near inset between 65 and 102
- **THEN** none of those lines is a fragment, because none reaches the gutter edge
- **AND** the answer is nothing

#### Scenario: A line that reaches the edge but is not short

- **WHEN** a line of a page 1988 pixels wide has the near inset 4 and the far
  inset 410, which is 0.21 of that width
- **THEN** that line is not a fragment

#### Scenario: A full line that reaches the edge

- **WHEN** a line of a page 1425 pixels wide has the near inset 0 and the far
  inset 1342, which is 0.94 of that width
- **THEN** that line is not a fragment

### Requirement: The body of the page bounds the trim

A body line SHALL be a line whose box is wider than 0.50 of the page width. The
body edge SHALL be the smallest near inset of the body lines. The trim SHALL
never reach the body edge.

A body line SHALL be far wider than a fragment can be, so no fragment can ever
set the body edge. That is what the width test is for. A running head and a folio
stand away from the margin, so neither lowers the smallest near inset even where
one is wide enough to pass the test.

A page that holds no body line SHALL get no trim. Such a page carries no evidence
of where its own text begins, and a trim placed without that evidence could cover
the page itself.

#### Scenario: The body edge comes from the full lines

- **WHEN** a page 2083 pixels wide holds 33 body lines whose near insets run from
  182 to 257, a running head at the near inset 784, and a folio at 1738
- **THEN** the body edge is 182

#### Scenario: A page that holds no body line

- **WHEN** a page 1595 pixels wide holds three fragments and no line wider than
  0.50 of its width
- **THEN** the answer is nothing

### Requirement: A trim needs a corridor between the fragments and the body

The corridor SHALL be the distance from the largest far inset of the fragments to
the body edge. The trim SHALL be refused when the corridor is below 0.03 of the
page width.

A corridor that narrow means the fragments and the body are not separated, so
they may not be two things at all. Refusing is what keeps a trim from cutting the
page it was meant to clean.

#### Scenario: A measured corridor

- **WHEN** a page 2083 pixels wide holds fragments whose largest far inset is 61
- **AND** its body edge is 182
- **THEN** the corridor is 121, which is 0.06 of that width, and the trim holds

#### Scenario: The fragments reach the body

- **WHEN** the corridor of a page is below 0.03 of its width
- **THEN** the answer is nothing

### Requirement: One mark is not a strip

The trim SHALL be refused when the page holds fewer than two fragments. A strip
of the neighbouring page carries the ends of the lines of that page, so it stands
in a column. A single mark against the edge is a speck, a fold or a shadow, and
one of those is not evidence that a whole strip is there.

#### Scenario: A single fragment

- **WHEN** a page 2067 pixels wide cut as the `left` half holds one fragment
  with the far inset 64, and no other fragment
- **THEN** the answer is nothing

#### Scenario: Two fragments are a column

- **WHEN** a page 2033 pixels wide holds two fragments whose largest far inset is
  28, and its body edge is 152
- **THEN** the trim holds

### Requirement: The trim column lies in the middle of the corridor

The trim column SHALL be the middle of the corridor: the point halfway between
the largest far inset of the fragments and the body edge, rounded to a whole
number.

The detector does not read every fragment of a strip. It found 2 of them on one
measured page and 23 on another, while the ink of both strips runs the height of
the page. The trim is therefore placed by the corridor and not by the fragments
it happened to find, so it covers the ink the detector missed as well.

#### Scenario: A strip the detector read well

- **WHEN** a page 2083 pixels wide holds 18 fragments whose largest far inset is
  61, and its body edge is 182
- **THEN** the trim column is 122

#### Scenario: A strip the detector barely read

- **WHEN** a page 2033 pixels wide holds 2 fragments whose largest far inset is
  28, and its body edge is 152
- **THEN** the trim column is 90, which covers the ink of the strip the detector
  did not read
