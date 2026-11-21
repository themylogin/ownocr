## ADDED Requirements

### Requirement: The reader of an OCR book shows the text beside the page

The reader SHALL split in two for a book in the `ocr` stage. The left half SHALL
show the page image. The right half SHALL show the same image, covered by a white
veil of 50 percent opacity, with the read text drawn on top of the veil.

The reader SHALL draw one text box per read line, at the place the line box
names. Each drawn line SHALL stand on a white ground the size of its box, so the
print under a line reaches no glyph. The text SHALL be the text of that line,
unchanged. The drawn text SHALL
be selectable, so a user can copy it.

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

#### Scenario: A user copies the text

- **WHEN** a user selects the drawn text of a line and copies it
- **THEN** the clipboard holds that text

#### Scenario: A book of another stage is unchanged

- **WHEN** a user opens the reader for a book in the `split` stage
- **THEN** the reader shows one image per page, in one column
- **AND** no veil and no text appear

### Requirement: The OCR reader fits both halves in the window

The OCR reader SHALL use the whole window width and SHALL NOT stay inside the
72 rem content column. The two halves of one page, and the gap between them,
SHALL fit in the window width. The reader SHALL scale both halves down together
when they do not fit, so the two images always keep the same size.

The reader SHALL NOT enlarge a page beyond its stored size. The reader SHALL use
the 1280 px display rendition for both halves.

The line boxes SHALL follow the same scale as the image they lie on. The font
size of a drawn line SHALL follow the height of its box, and the text SHALL fill
the width of its box. A change of the window width SHALL move and resize the
boxes with the image.

#### Scenario: Two halves do not fit

- **WHEN** the two halves of a page are wider than the window
- **THEN** both halves are scaled down by the same factor
- **AND** together with the gap they fit in the window width
- **AND** neither half is cropped

#### Scenario: The page is smaller than half the window

- **WHEN** half the window is wider than the stored page
- **THEN** the page is shown at its stored size
- **AND** it is not enlarged

#### Scenario: A box follows its image

- **WHEN** the reader scales a page down by half
- **THEN** every box of that page is half as wide and half as high
- **AND** the drawn text is half as large

#### Scenario: A user resizes the window

- **WHEN** a user makes the window narrower
- **THEN** both halves and every box of the visible pages take the new scale

### Requirement: The OCR reader draws the text of the pages near the window only

The reader SHALL ask for the lines of a page when that page comes near the
window, and SHALL NOT ask for the lines of every page of the book at once. It
SHALL draw the boxes of a page it holds the lines of, and SHALL remove the boxes
of a page that is far from the window.

Removing the boxes of a page SHALL NOT change the place of any page. A page whose
lines have not arrived SHALL keep its veiled image and its size, so no page moves
while a user scrolls.

A page that holds no result SHALL show the veiled image with no text, and SHALL
raise no error to the user.

#### Scenario: A large OCR book opens

- **WHEN** a user opens the reader for an OCR book of 500 pages
- **THEN** the reader asks for the lines of the pages near the window only
- **AND** the drawn boxes cover those pages only

#### Scenario: A user scrolls on

- **WHEN** a user scrolls past a page that carried drawn boxes
- **THEN** the boxes of the pages that come near the window appear
- **AND** the boxes of the pages far behind are removed
- **AND** no page moves

#### Scenario: A page holds no result

- **WHEN** the reader reaches a page of a running OCR copy that holds no result
- **THEN** the right half shows the veiled image with no text
- **AND** the reader shows no error

## MODIFIED Requirements

### Requirement: The reader fits a page to the content column

The reader SHALL show every page of the book stacked vertically, in page order,
each fitted to the width of the content column. The reader SHALL NOT enlarge a
page beyond its stored size. The reader SHALL use the 1280 px display rendition.
The reader SHALL show no page number.

This requirement covers a book that is not in the `ocr` stage. A book in the
`ocr` stage follows the OCR reader requirements, which use the whole window
width.

#### Scenario: A user scrolls the reader

- **WHEN** a user scrolls down in the reader
- **THEN** the next page follows directly below the current page
- **AND** pages stay in page order

#### Scenario: A page is wider than the column

- **WHEN** a page is wider than the content column
- **THEN** the page is scaled down to the column width
- **AND** the page is not cropped

#### Scenario: A page number is never shown

- **WHEN** a user reads any page
- **THEN** no page number appears with it

#### Scenario: A large book opens

- **WHEN** a user opens the reader for a book of 500 pages
- **THEN** the view appears without loading all 500 images at once
- **AND** images load as they approach the viewport

#### Scenario: An OCR book uses the whole window

- **WHEN** a user opens the reader for a book in the `ocr` stage
- **THEN** the pages are not fitted to the content column
