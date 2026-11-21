## ADDED Requirements

### Requirement: A landscape image with no gutter is cut at its middle

The sidecar SHALL report the middle column for a landscape image it finds no
gutter in. It SHALL call an image landscape when its width is at least 1.2 times
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

- **WHEN** the caller passes an image 1.2 times wider than it is tall, with no
  gutter
- **THEN** the entry is a pair of whole numbers

#### Scenario: An image not quite wide enough

- **WHEN** the caller passes an image 1.1 times wider than it is tall, with no
  gutter
- **THEN** the entry is null

## MODIFIED Requirements

### Requirement: The sidecar tells a spread from a single page

The sidecar SHALL report null for an image that holds one page and is not
landscape. A portrait cover, a portrait single page and a square image with no
gutter SHALL all report null.

The decision SHALL rest on whether the gutter runs the whole height of the
image. The strength of the gutter alone SHALL NOT decide it.

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
