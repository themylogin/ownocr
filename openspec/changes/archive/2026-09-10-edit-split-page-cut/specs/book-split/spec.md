## MODIFIED Requirements

### Requirement: Splitting cuts a spread into two pages

The split worker SHALL cut every page of the source book that holds a spread into
two pages, the left page before the right page. It SHALL copy every other page
whole.

A page the worker cuts SHALL keep every column of its own page, so a leaning
gutter SHALL NOT clip either page. A page the worker copies whole SHALL keep the
stored bytes of its source page.

The worker SHALL record, for every page it cuts, the side of the gutter that
page holds and the column it was cut at. A page it copies whole SHALL record no
cut. The record is what lets a user move that cut afterwards.

#### Scenario: A book of spreads is split

- **WHEN** the worker splits a source book of 120 pages that are all spreads
- **THEN** the copy holds 240 pages
- **AND** page 1 of the copy shows the left page of source page 1
- **AND** page 2 of the copy shows the right page of source page 1

#### Scenario: A book mixes spreads and single pages

- **WHEN** the worker splits a source book whose first page is a cover and whose
  other 67 pages are spreads
- **THEN** the copy holds 135 pages
- **AND** page 1 of the copy shows the cover unchanged

#### Scenario: A cut page keeps every column of its own page

- **WHEN** the worker cuts a spread whose gutter leans
- **THEN** the left page of the copy holds every column of the left page
- **AND** the right page of the copy holds every column of the right page

#### Scenario: A page that is not a spread keeps its bytes

- **WHEN** the worker copies a page whole
- **THEN** the stored image of that page of the copy is the stored image of its
  source page

#### Scenario: The two pages of a spread record their cuts

- **WHEN** the worker cuts a spread
- **THEN** the left page records the left side and the column it ends at
- **AND** the right page records the right side and the column it starts at

#### Scenario: A whole page records no cut

- **WHEN** the worker copies a cover whole
- **THEN** that page records no cut

### Requirement: Splitting is repeatable for one book

A retried split job SHALL leave the copy with exactly one set of pages. The
system SHALL discard whatever an interrupted attempt left behind before it
builds the pages again.

A retry SHALL write new pages, which carry the cuts the sidecar reports. A cut
a user moved by hand SHALL therefore not survive a retry.

#### Scenario: A job is retried

- **WHEN** a split job runs again after an interrupted attempt
- **THEN** the copy holds exactly one set of pages
- **AND** no page index appears twice

#### Scenario: A retry discards an edited cut

- **WHEN** a user moves the cut of a page and the split job then runs again
- **THEN** the pages of the copy carry the cuts the sidecar reports
- **AND** the moved cut is gone
