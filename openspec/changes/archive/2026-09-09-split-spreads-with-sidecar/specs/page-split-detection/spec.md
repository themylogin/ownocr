## Purpose

Finds the gutter of a photographed book spread and reports where to cut it into
two single pages. It also decides whether an image is a spread at all, so a
cover or a single page passes through whole.

## ADDED Requirements

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

The sidecar SHALL report null for an image that holds one page. A photographed
cover, a single page and an image with no gutter SHALL all report null.

The decision SHALL rest on whether the gutter runs the whole height of the
image. The strength of the gutter alone SHALL NOT decide it.

#### Scenario: A photographed cover

- **WHEN** the caller passes a photograph of a book cover
- **THEN** the entry is null

#### Scenario: A faint gutter on a flat book

- **WHEN** the caller passes a spread of a book that lies almost flat, whose
  gutter is a pale ridge rather than a dark shadow
- **THEN** the entry is a pair of whole numbers

#### Scenario: A strong mark that is not a gutter

- **WHEN** the caller passes a single page carrying a strong dark band that
  covers part of its height
- **THEN** the entry is null

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
