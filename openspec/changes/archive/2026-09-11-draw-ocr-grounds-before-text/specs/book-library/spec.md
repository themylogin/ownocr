## MODIFIED Requirements

### Requirement: The reader of an OCR book shows the text beside the page

The reader SHALL split in two for a book in the `ocr` stage. The left half SHALL
show the page image. The right half SHALL show the same image, covered by a white
veil of 50 percent opacity, with the read text drawn on top of the veil.

The reader SHALL draw one text box per read line, at the place the line box
names. Each drawn line SHALL stand on a white ground the size of its box, so the
print under a line reaches no glyph. The text SHALL be the text of that line,
unchanged. The drawn text SHALL be selectable, so a user can copy it.

The reader SHALL draw every white ground below every drawn line. No ground SHALL
cover the text of another line, whatever the boxes of two lines do to one
another. A glyph that reaches out of its own box SHALL stay readable.

The two halves SHALL stand side by side and SHALL scroll together, so a line of
the right half stays beside the same line of the left half. The reader SHALL keep
the page order and SHALL show no page number.

The reader of a book in every other stage SHALL NOT split, SHALL show no veil,
and SHALL draw no text.

#### Scenario: A user opens the reader of an OCR book

- **WHEN** a user opens the reader for a book in the `ocr` stage
- **THEN** every page appears twice, side by side
- **AND** the right image carries the white veil and the drawn text

#### Scenario: The drawn text matches the lines

- **WHEN** a page holds a result of 40 lines
- **THEN** the right half draws 40 text boxes
- **AND** each one holds the text of its line

#### Scenario: A ground covers no text

- **WHEN** a page holds two lines whose boxes touch or overlap
- **THEN** the text of the upper line is drawn over the ground of the lower line
- **AND** no part of a glyph is hidden by a ground

#### Scenario: A user copies the text

- **WHEN** a user selects the drawn text of a line and copies it
- **THEN** the clipboard holds that text
- **AND** the lines read in page order

#### Scenario: A book of another stage is unchanged

- **WHEN** a user opens the reader for a book in the `split` stage
- **THEN** the reader shows one image per page, in one column
- **AND** no veil and no text appear
