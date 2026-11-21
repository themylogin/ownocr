## MODIFIED Requirements

### Requirement: A page read by two engines holds a merged result

The system SHALL store a third result for every page of a copy whose profile names
two engines. Its model SHALL be `merged`. The system SHALL write it after it
collects the results of both engines for that page.

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

A copy whose profile names one engine SHALL hold no merged result. One engine
leaves nothing to join.

The merged result SHALL be derived. The system SHALL NOT change either engine
result to write it, and SHALL be able to write it again from the two engine
results alone.

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
