## Purpose

Writes the single pages of a batch of photographed book pages. It finds the
gutter of a spread, straightens it, and cuts it in two. It also decides whether
an image is a spread at all, so a cover or a single page passes through whole.

## Requirements

### Requirement: The sidecar reports where to cut a list of images

The sidecar SHALL offer a method that takes an ordered list of image paths and
returns an ordered list of the same length. Every entry SHALL correspond to the
image at the same position.

An entry SHALL be null when the image is not a spread. An entry SHALL be a pair
of whole numbers when the image is a spread. The first number SHALL be the
column after the last column of the left page. The second number SHALL be the
column of the first column of the right page.

The method SHALL take no other input. It SHALL offer no threshold, no margin and
no mode.

#### Scenario: A list of one spread and one single page

- **WHEN** a caller passes a single page followed by a spread
- **THEN** the answer holds two entries
- **AND** the first entry is null
- **AND** the second entry is a pair of whole numbers

#### Scenario: The caller passes an empty list

- **WHEN** a caller passes an empty list
- **THEN** the answer is an empty list

#### Scenario: The order is kept

- **WHEN** a caller passes twenty images
- **THEN** entry `n` of the answer describes image `n` of the request

### Requirement: The two cut columns overlap

For a spread, the first number SHALL be greater than or equal to the second
number. Cutting an image at these two columns SHALL therefore lose no column of
either page.

The numbers SHALL lie inside the image. The first number SHALL NOT exceed the
width of the image, and the second number SHALL NOT be below zero.

#### Scenario: An upright spread

- **WHEN** the gutter of a spread stands upright
- **THEN** the two numbers are close together
- **AND** the first is greater than or equal to the second

#### Scenario: A leaning gutter

- **WHEN** the gutter of a spread leans by several degrees
- **THEN** the gap between the two numbers grows with the lean
- **AND** the left page keeps every one of its columns
- **AND** the right page keeps every one of its columns

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

### Requirement: The sidecar refuses a request it cannot read

The sidecar SHALL refuse the whole request when it cannot read any image in the
list. It SHALL name the image it could not read. It SHALL NOT return a partial
answer.

#### Scenario: One image is missing

- **WHEN** one path of the list names no file
- **THEN** the sidecar refuses the request
- **AND** the answer names that path

#### Scenario: One image is corrupt

- **WHEN** one path of the list names a file that is not a readable image
- **THEN** the sidecar refuses the request
- **AND** the answer names that path

### Requirement: The sidecar reads pages from the shared data volume

The sidecar SHALL read the images from the file store it shares with the
backend. Image bytes SHALL NOT travel in the request or in the answer.

#### Scenario: The caller passes paths

- **WHEN** the caller passes the path of a stored page
- **THEN** the sidecar reads that file itself
- **AND** the request carries no image bytes

### Requirement: A landscape image with no gutter is cut at its middle

The sidecar SHALL report the middle column for a landscape image it finds no
gutter in. It SHALL call an image landscape when its width is at least 1.15 times
its height.

The two numbers SHALL both be the middle column, so the left page holds every
column before the middle and the right page holds the middle and every column
after it. No column SHALL lie in both pages, and no column SHALL be lost.

The shape of the image is the evidence. A spread is wider than it is tall, and a
page that is not is not.

A gutter the sidecar finds SHALL decide the cut. The middle SHALL answer only
when the sidecar finds none.

#### Scenario: A landscape image holds no gutter

- **WHEN** the caller passes a landscape photograph the sidecar finds no gutter
  in
- **THEN** the entry is a pair of whole numbers
- **AND** both numbers are the middle column of the image

#### Scenario: The two pages meet at the middle

- **WHEN** the sidecar answers the middle column for an image 1600 columns wide
- **THEN** the entry is the pair 800 and 800
- **AND** cutting there gives two pages of 800 columns each

#### Scenario: A found gutter beats the middle

- **WHEN** the caller passes a landscape spread whose gutter sits away from the
  middle
- **THEN** the entry names the columns of that gutter
- **AND** it does not name the middle column

#### Scenario: An image just wide enough

- **WHEN** the caller passes an image 1.15 times wider than it is tall, with no
  gutter
- **THEN** the entry is a pair of whole numbers

#### Scenario: An image not quite wide enough

- **WHEN** the caller passes an image 1.1 times wider than it is tall, with no
  gutter
- **THEN** the entry is null
