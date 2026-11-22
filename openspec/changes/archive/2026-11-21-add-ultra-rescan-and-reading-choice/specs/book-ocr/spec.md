## MODIFIED Requirements

### Requirement: The OCR action carries a model profile

The system SHALL define three OCR profiles: `v6`, `vl` and `claude`. For the OCR
action the profile `v6` SHALL run the PP-OCRv6 engine alone, and the profile `vl`
SHALL run the PaddleOCR-VL engine and the PP-OCRv6 engine together.

The profile `claude` SHALL belong to the rescan alone. The OCR action SHALL
refuse it, and SHALL name the reason. A whole book is read by the engines the
endpoint runs; `claude` reads a page the endpoint already failed.

A rescan names a profile from all three, and a user picks between them in the
same words. The engines a rescan runs for a profile SHALL be those the
`page-rescan` capability names, which are not those the OCR action runs. The OCR
action reads a page the system holds no result for; a rescan reads a page the
system has read already.

The system SHALL require a profile on every OCR action, and SHALL refuse an OCR
action that names no profile or an unknown one. The system SHALL record the
profile on the copy the action creates.

The profile SHALL say which engine holds the text of record for the reading the
action performed. For `v6` the PP-OCRv6 result answers. For `vl` the
PaddleOCR-VL result answers, and the PP-OCRv6 result carries the line boxes and
the confidence PaddleOCR-VL does not report.

The profile SHALL describe the reading the action performed and SHALL NOT
describe a page a later rescan read again. The system SHALL NOT change the
profile of a copy when a rescan reads a page of it, whatever profile that rescan
names. A client that needs to know which engines read a page SHALL read the
results of that page, and SHALL NOT derive it from the profile.

#### Scenario: A user picks the normal profile

- **WHEN** a user runs the OCR action with the profile `v6`
- **THEN** the copy records the profile `v6`
- **AND** the endpoint job asks for the PP-OCRv6 engine alone

#### Scenario: A user picks the extra profile

- **WHEN** a user runs the OCR action with the profile `vl`
- **THEN** the copy records the profile `vl`
- **AND** the endpoint job asks for both engines

#### Scenario: The request names no profile

- **WHEN** a user runs the OCR action without a profile
- **THEN** the system rejects the request and names the reason
- **AND** no copy exists for that book

#### Scenario: The request names an unknown profile

- **WHEN** a user runs the OCR action with a profile that is not `v6`, `vl` or
  `claude`
- **THEN** the system rejects the request and names the reason

#### Scenario: The OCR action names the rescan profile

- **WHEN** a user runs the OCR action with the profile `claude`
- **THEN** the system rejects the request and names the reason
- **AND** no copy exists for that book

#### Scenario: A retry keeps the profile

- **WHEN** a user retries a copy that failed with the profile `vl`
- **THEN** the retried job asks for both engines again
- **AND** the copy still records the profile `vl`

#### Scenario: A rescan keeps the profile

- **WHEN** a rescan with the profile `vl` reads pages 12 and 13 of a copy whose
  profile is `v6`
- **THEN** the copy still records the profile `v6`
- **AND** pages 12 and 13 hold a PaddleOCR-VL result
- **AND** the profile of the copy says nothing about pages 12 and 13

#### Scenario: A normal rescan keeps the profile

- **WHEN** a rescan with the profile `v6` reads page 12 of a copy whose profile
  is `vl`
- **THEN** the copy still records the profile `vl`
- **AND** page 12 holds a PP-OCRv6 result alone

### Requirement: A page read by two engines holds a merged result

The system SHALL store a third result for every page that both the PP-OCRv6
engine and the PaddleOCR-VL engine have read. Its model SHALL be `merged`. The
system SHALL write it once it holds the results of both engines for that page.

The two engine results MAY reach the system in one job or in two. A copy whose
profile names two engines gains both in the job that builds it. A page of a copy
whose profile names one engine gains the second in a later rescan. The merged
result is the same either way.

The merged result SHALL hold the line boxes of the PP-OCRv6 result filled with the
text of the PaddleOCR-VL result. Its full result SHALL hold, for every line, the
box, the text and the name of the engine that text came from. It SHALL also hold
what the merge dropped and why, and the counts the merge reports. Its text SHALL be
the text of the merged lines, one line per line, in page order.

The merged result SHALL hold one line for every line of the PP-OCRv6 result,
except where the merge adds or removes one. The merge adds a line for a printed
row PP-OCRv6 missed, adds one for ink PP-OCRv6 left uncovered between two boxes of
one printed row, and removes the lines of a region whose boxes cannot say where
the printed rows are. The `page-text-merge` capability governs all three. The
counts of the merged result SHALL report how many lines the merge placed itself
and how many boxes it discarded.

A page SHALL hold one merged result per text engine that read it. The merged
result of PaddleOCR-VL and the merged result of Claude Opus 5 SHALL be stored
apart, so a page read by both holds both and a reader can answer either.

A text engine that reports no block SHALL be merged through one block covering
the whole page. Claude Opus 5 answers the text of a page and no geometry, so the
system SHALL build that block from the stored size of the page and the text it
answered. The merge then reads the printed rows from the PP-OCRv6 boxes alone,
which is what it does for any block.

A page that one engine alone has read SHALL hold no merged result. One engine
leaves nothing to join. A copy whose profile names one engine therefore holds no
merged result when its job ends.

A page that loses one of its two engine results SHALL lose its merged result in
the same step. The merged result derives from both, so it outlives neither.

A reading read on a metered API SHALL survive a reading it did not come from. A
rescan that replaces the PP-OCRv6 result of a page SHALL keep the Claude Opus 5
text of that page and SHALL write its merged result again from the new line boxes.
That text is a reading of the page image and derives from no other result, so
nothing about new line boxes makes it wrong — and re-reading it costs money where
rebuilding the merge costs nothing.

The merged result SHALL be derived. The system SHALL NOT change either engine
result to write it, and SHALL be able to write it again from the two engine
results alone. The system MAY read a stored engine result to write it, rather
than asking the endpoint for that engine again.

#### Scenario: A page read with the extra profile

- **WHEN** the worker stores page 3 of a copy whose profile is `vl`
- **THEN** page 3 holds three results, named `v6`, `vl` and `merged`
- **AND** every line of the merged result that came from a `v6` line carries the
  box of that line

#### Scenario: A page whose boxes the merge replaced

- **WHEN** the worker stores a page whose top region PP-OCRv6 read as 14 sloping
  boxes across 8 printed rows
- **THEN** the merged result holds 8 lines for that region, none of them a `v6`
  box
- **AND** its counts report 14 discarded boxes
- **AND** the `v6` result still holds the 14 boxes PP-OCRv6 wrote

#### Scenario: A page whose rows hold ink no box covered

- **WHEN** the worker stores a page where two printed rows each hold ink between
  two boxes that neither box covers
- **THEN** the merged result holds a line for each of those two stretches
- **AND** its counts report 2 lines the merge placed
- **AND** the `v6` result still holds only the boxes PP-OCRv6 wrote

#### Scenario: A page read with the normal profile

- **WHEN** the worker stores page 3 of a copy whose profile is `v6`
- **THEN** page 3 holds one result, named `v6`
- **AND** page 3 holds no merged result

#### Scenario: A page of a normal copy is read again by the visual engine

- **WHEN** a rescan with the profile `vl` reads page 3 of a copy whose profile is
  `v6`
- **THEN** page 3 holds three results, named `v6`, `vl` and `merged`
- **AND** every line of the merged result that came from a `v6` line carries the
  box of that line
- **AND** page 4 of that copy, which the rescan did not name, still holds one
  result and no merged result

#### Scenario: A page read by both text engines

- **WHEN** an Ultra rescan reads a page that already holds a merged PaddleOCR-VL
  result
- **THEN** the page holds both merged results
- **AND** neither is built from the other

#### Scenario: A page merged from text alone

- **WHEN** the worker merges the text Claude Opus 5 answered for a page
- **THEN** it builds one block covering the stored size of that page
- **AND** every line of the merged result carries the box of a PP-OCRv6 line,
  except the lines the merge places itself

#### Scenario: A page loses its visual reading

- **WHEN** a rescan with the profile `v6` reads page 3 of a copy, and page 3 held
  a `v6`, a `vl` and a `merged` result
- **THEN** page 3 holds one result, named `v6`
- **AND** page 3 holds no merged result

#### Scenario: A normal rescan of a page read by Claude Opus 5

- **WHEN** a rescan with the profile `v6` reads a page holding a `v6`, a `vl`, a
  `merged`, a `claude` and a `claude_merged` result
- **THEN** the page holds the new `v6` result, the `claude` result it held, and a
  `claude_merged` result built from that text and the new line boxes
- **AND** it holds no `vl` result and no `merged` result
- **AND** no request reaches the metered API

#### Scenario: The engine results are untouched

- **WHEN** the worker writes the merged result of a page
- **THEN** the `v6` result holds the text and the result PP-OCRv6 wrote
- **AND** the `vl` result holds the text and the result PaddleOCR-VL wrote

#### Scenario: A retry replaces the merged result

- **WHEN** a retried job stores every page of a copy whose profile is `vl` again
- **THEN** each page holds exactly one result named `merged`
- **AND** no merged result of the earlier attempt remains

#### Scenario: One engine read a page as empty

- **WHEN** PaddleOCR-VL returns no block for a page PP-OCRv6 read as 12 lines
- **THEN** the merged result of that page holds those 12 lines with their `v6` text
- **AND** the copy still completes

### Requirement: The API answers the read lines of one page

The system SHALL answer the read lines of one page of a book, on request by book
and page index. The answer SHALL name the model it comes from, SHALL report the
pixel size the boxes count in, and SHALL hold one entry per line. Every entry
SHALL hold the box of that line and the text of that line.

Every entry SHALL say whether its text came from the line engine rather than from
the engine the reading is named for. A merged reading takes the text of a line
from its reading engine where the two engines read that line alike, and from the
line engine where they did not, so a reader cannot otherwise tell which of the two
is on the page. An entry of an engine result SHALL say nothing: every line of it is
that engine's own.

A box SHALL be four numbers: left, top, right, and bottom. They SHALL count in
the pixels of the stored page image, which are the width and the height the book
detail reports for that page.

The system SHALL answer the reading the page names, when it names one. The
`page-reading-choice` capability governs which reading a page names.

A page that names none SHALL answer the merged result when the page holds one,
and the PP-OCRv6 result when it does not. Where a page holds more than one merged
result, the best ranked SHALL answer.

It SHALL NOT answer the PaddleOCR-VL result or the Claude Opus 5 result, which
report blocks of reflowed text and no lines. It SHALL NOT rewrite
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

#### Scenario: A page names a reading

- **WHEN** a client asks for the lines of a page that names its PP-OCRv6 result,
  and that page also holds a merged result
- **THEN** the answer names the model of the PP-OCRv6 result
- **AND** it holds one entry per PP-OCRv6 line

#### Scenario: A merged reading whose lines came from both engines

- **WHEN** a client asks for the lines of a page whose merged reading took the
  body from PaddleOCR-VL and the footnotes from PP-OCRv6
- **THEN** every footnote entry says its text came from the line engine
- **AND** no entry of the body says so

#### Scenario: An engine result names no line as substituted

- **WHEN** a client asks for the lines of a page that answers its PP-OCRv6 result
- **THEN** no entry says its text came from the line engine

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
