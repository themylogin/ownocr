## MODIFIED Requirements

### Requirement: The OCR action carries a model profile

The system SHALL define two OCR profiles: `v6` and `vl`. For the OCR action the
profile `v6` SHALL run the PP-OCRv6 engine alone, and the profile `vl` SHALL run
the PaddleOCR-VL engine and the PP-OCRv6 engine together.

A rescan names a profile from the same two, and a user picks between them in the
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

- **WHEN** a user runs the OCR action with a profile that is not `v6` or `vl`
- **THEN** the system rejects the request and names the reason

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

### Requirement: Every page of a copy holds one result per engine

The system SHALL store one result for every engine that read a page. A result
SHALL name its engine, SHALL hold the text that engine produced, and SHALL hold
the result that engine produced in full.

The text SHALL be what the engine wrote, unchanged. The system SHALL NOT rewrite
it, merge it with the text of another engine, or drop part of it. PP-OCRv6
writes its recognised lines in detection order. PaddleOCR-VL writes markdown.

The full result SHALL hold what the text does not. PaddleOCR-VL leaves footnote
and page number blocks out of its markdown, and PP-OCRv6 reports a box and a
confidence for every line. Both SHALL survive in the stored result.

A page SHALL hold at most one result per engine. A retry SHALL replace the
results of the copy, so no result of an earlier attempt remains.

A rescan of a page MAY replace the result of an engine that read that page
before, and MAY remove one. The `page-rescan` capability governs both. A page
therefore holds the results of the last reading of that page, not of the reading
that built the copy.

A page MAY hold one further result whose model names no engine. Such a result is
derived from the engine results of that page. It SHALL NOT change them, and the
rules above SHALL hold for the engine results whatever else a page holds.

#### Scenario: A page read with the normal profile

- **WHEN** the worker stores page 3 of a copy whose profile is `v6`
- **THEN** page 3 holds one result, naming the PP-OCRv6 engine
- **AND** the result holds the recognised lines and the full engine result

#### Scenario: A page read with the extra profile

- **WHEN** the worker stores page 3 of a copy whose profile is `vl`
- **THEN** page 3 holds one result per engine, one naming PaddleOCR-VL and one
  naming PP-OCRv6
- **AND** the PaddleOCR-VL result holds markdown
- **AND** the PP-OCRv6 result holds a box and a confidence for every line

#### Scenario: A page holds no text

- **WHEN** an engine returns an empty text for a page
- **THEN** the result of that engine for that page exists and its text is empty
- **AND** the copy still completes

#### Scenario: A retry replaces the results

- **WHEN** a retried job stores every page of a copy again
- **THEN** each page holds exactly one result per engine of its profile
- **AND** no result of the earlier attempt remains

#### Scenario: A rescan replaces the result of an engine

- **WHEN** a rescan reads page 3 with an engine that has read page 3 before
- **THEN** page 3 holds one result of that engine
- **AND** it is the result of the rescan

#### Scenario: A page is deleted

- **WHEN** a page of a copy is removed
- **THEN** every result of that page is removed with it

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

A page that one engine alone has read SHALL hold no merged result. One engine
leaves nothing to join. A copy whose profile names one engine therefore holds no
merged result when its job ends.

A page that loses one of its two engine results SHALL lose its merged result in
the same step. The merged result derives from both, so it outlives neither.

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

#### Scenario: A page loses its visual reading

- **WHEN** a rescan with the profile `v6` reads page 3 of a copy, and page 3 held
  a `v6`, a `vl` and a `merged` result
- **THEN** page 3 holds one result, named `v6`
- **AND** page 3 holds no merged result

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
