## MODIFIED Requirements

### Requirement: The API answers the read lines of one page

The system SHALL answer the read lines of one page of a book, on request by book
and page index. The answer SHALL name the model it comes from, SHALL report the
pixel size the boxes count in, and SHALL hold one entry per line. Every entry
SHALL hold the box of that line and the text of that line.

A box SHALL be four numbers: left, top, right, and bottom. They SHALL count in
the pixels of the stored page image, which are the width and the height the book
detail reports for that page.

The system SHALL answer the merged result of the page when the page holds one,
and the PP-OCRv6 result when it does not. It SHALL NOT answer the PaddleOCR-VL
result, which reports blocks of reflowed text and no lines. It SHALL NOT rewrite
the text of a line or reorder the lines. The order SHALL be the order the stored
result holds.

The answer SHALL report the trim of the page when the page holds one. A trim
SHALL name the edge it lies against and the column it ends at, counting as a
distance from that edge. The answer SHALL report no trim when the page holds
none.

The system SHALL drop from the answer every line whose box lies wholly inside the
trim. Those lines hold the ink of the page beside this one, which this page never
held. A client therefore needs no rule of its own, and a reader and an export
SHALL drop the same lines.

The answer for a whole book SHALL report the trim of every page and drop the same
lines, by the same rule. The reader asks for a whole book in one request, so the
two answers SHALL never differ about what a page says.

The system SHALL NOT drop a line that lies outside the trim, whatever its text.

A trim SHALL cover pixels and SHALL move none. The reported width and height
SHALL be the stored size of the page whether the page holds a trim or not, and
the box of a line the answer keeps SHALL be the box the stored result holds.

The cut side of a page SHALL come from the nearest page in the lineage of that
page that carries a cut side. A copy holds the pages of its source book, so a
page of an OCR copy carries no cut side of its own and takes the side of the page
it was read from. A page with no such ancestor SHALL get no trim.

The system SHALL refuse the request for a page that holds neither result, and
SHALL name the reason. It SHALL refuse the request for a book or a page that
does not exist.

#### Scenario: A page holds a merged result

- **WHEN** a client asks for the lines of a page of a copy with the profile `vl`
- **THEN** the answer names the model `merged`
- **AND** it holds one entry per merged line, with the box and the text of that
  line

#### Scenario: A page holds one engine result

- **WHEN** a client asks for the lines of a page of a copy with the profile `v6`
- **THEN** the answer names the model `v6`
- **AND** it holds one entry per PP-OCRv6 line, with the box and the text of that
  line

#### Scenario: The boxes count in the stored page size

- **WHEN** a client asks for the lines of a page 3000 pixels wide
- **THEN** the answer reports the width 3000 and the height of that page
- **AND** every box lies in that space

#### Scenario: The text reaches the client unchanged

- **WHEN** a stored line holds the text the engine wrote
- **THEN** the answer holds that text, character for character

#### Scenario: A page that carries a strip of the page beside it

- **WHEN** a client asks for the lines of a page 2083 pixels wide that was cut as
  the right half of a spread
- **AND** the stored result holds 53 lines, 18 of which are fragments of the page
  beside it
- **THEN** the answer reports a trim of 122 pixels against the left edge
- **AND** it holds the 35 lines that lie outside that trim
- **AND** it reports the width 2083

#### Scenario: The book answer trims as the page answer does

- **WHEN** a client asks for the lines of a whole book of two pages, one carrying
  a strip and one not
- **THEN** each page of the answer reports the trim the answer for that page
  alone reports
- **AND** each holds the lines that answer holds

#### Scenario: A page that carries no strip

- **WHEN** a client asks for the lines of a page that holds no trim
- **THEN** the answer reports no trim
- **AND** it holds every line of the stored result

#### Scenario: The cut side comes from the page the copy was read from

- **WHEN** a client asks for the lines of a page of an OCR copy
- **AND** that page carries no cut side, but the split page it was read from
  carries the cut side `right`
- **THEN** the trim of the answer lies against the left edge

#### Scenario: A page that is not a half of a spread

- **WHEN** a client asks for the lines of a page whose lineage holds no cut side
- **THEN** the answer reports no trim
- **AND** it holds every line of the stored result

#### Scenario: A short line outside the trim survives

- **WHEN** a page holds a folio, a running head and a one word last line of a
  paragraph, and all three lie outside the trim
- **THEN** the answer holds all three

#### Scenario: A page was not read

- **WHEN** a client asks for the lines of a page that holds no result
- **THEN** the system refuses the request and names the reason

#### Scenario: The page does not exist

- **WHEN** a client asks for the lines of a page index the book does not hold
- **THEN** the system refuses the request
