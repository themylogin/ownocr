## ADDED Requirements

### Requirement: A page read by two engines holds a merged result

The system SHALL store a third result for every page of a copy whose profile names
two engines. Its model SHALL be `merged`. The system SHALL write it after it
collects the results of both engines for that page.

The merged result SHALL hold the line boxes of the PP-OCRv6 result filled with the
text of the PaddleOCR-VL result. Its full result SHALL hold, for every line, the
box, the text and the name of the engine that text came from. It SHALL also hold
what the merge dropped and why, and the counts the merge reports. Its text SHALL be
the text of the merged lines, one line per line, in page order.

A copy whose profile names one engine SHALL hold no merged result. One engine
leaves nothing to join.

The merged result SHALL be derived. The system SHALL NOT change either engine
result to write it, and SHALL be able to write it again from the two engine
results alone.

#### Scenario: A page read with the extra profile

- **WHEN** the worker stores page 3 of a copy whose profile is `vl`
- **THEN** page 3 holds three results, named `v6`, `vl` and `merged`
- **AND** the merged result holds one line for every line of the `v6` result
- **AND** every line of the merged result carries the box of its `v6` line

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

## MODIFIED Requirements

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

#### Scenario: A page is deleted

- **WHEN** a page of a copy is removed
- **THEN** every result of that page is removed with it
