## ADDED Requirements

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
the text of a line, reorder the lines, or drop a line. The order SHALL be the
order the stored result holds.

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

#### Scenario: A page was not read

- **WHEN** a client asks for the lines of a page that holds no result
- **THEN** the system refuses the request and names the reason

#### Scenario: The page does not exist

- **WHEN** a client asks for the lines of a page index the book does not hold
- **THEN** the system refuses the request
