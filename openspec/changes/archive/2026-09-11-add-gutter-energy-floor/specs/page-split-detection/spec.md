## MODIFIED Requirements

### Requirement: The sidecar tells a spread from a single page

The sidecar SHALL report null for an image that holds one page and is not
landscape. A portrait cover, a portrait single page and a square image with no
gutter SHALL all report null.

A gutter SHALL run the whole height of the image, and it SHALL carry a minimum
strength. Both SHALL hold. Neither one alone SHALL decide.

The sidecar SHALL reject a candidate below the strength floor and report no
gutter, whatever height it covers. The residue of a block of text covers the
height of that block, so coverage alone lets a paragraph pass as a gutter.

The floor SHALL be absolute. It SHALL NOT compare the candidate against the
strongest column of the image, because a dark scanner border is often that
column. It SHALL NOT change with the width or the height of the image.

An image with no gutter that is landscape SHALL NOT report null. The sidecar
answers its middle column instead, because its shape says it holds two pages.

#### Scenario: A photographed cover

- **WHEN** the caller passes a portrait photograph of a book cover
- **THEN** the entry is null

#### Scenario: A faint gutter on a flat book

- **WHEN** the caller passes a spread of a book that lies almost flat, whose
  gutter is a pale ridge rather than a dark shadow
- **THEN** the entry is a pair of whole numbers

#### Scenario: A strong mark that is not a gutter

- **WHEN** the caller passes a portrait single page carrying a strong dark band
  that covers part of its height
- **THEN** the entry is null

#### Scenario: A square image with no gutter

- **WHEN** the caller passes a square image with no gutter
- **THEN** the entry is null

#### Scenario: A spread whose facing page is blank

- **WHEN** the caller passes a landscape scan of a spread that shows no gutter,
  whose left page is blank and whose right page holds a block of text
- **THEN** the entry is a pair of whole numbers
- **AND** both numbers are the middle column of the image
- **AND** neither number falls inside the block of text

#### Scenario: A candidate as weak as the residue of a text block

- **WHEN** the strongest candidate of an image is as weak as the residue a block
  of text leaves behind
- **THEN** the sidecar finds no gutter
- **AND** the shape of the image alone decides the answer

#### Scenario: A dark border does not hide a real gutter

- **WHEN** the caller passes a spread with a real gutter and a dark border strip
  many times stronger than that gutter
- **THEN** the entry names the columns of the gutter
- **AND** it does not name the middle column
