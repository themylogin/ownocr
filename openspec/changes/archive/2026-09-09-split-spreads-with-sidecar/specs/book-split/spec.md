## REMOVED Requirements

### Requirement: The interim algorithm makes two pages from one

**Reason**: The real algorithm has arrived. The worker cuts a spread into its two
pages and copies every other page whole, so a source page no longer makes a fixed
number of copy pages.

**Migration**: None for stored data. A copy made by the interim algorithm keeps
its duplicated pages, because no action rebuilds a copy on its own. Running Split
again on the source book replaces the copy with real single pages, and the
existing rebuild clears the copy's pages and files first.

## ADDED Requirements

### Requirement: Splitting cuts a spread into two pages

The split worker SHALL cut every page of the source book that holds a spread into
two pages, the left page before the right page. It SHALL copy every other page
whole.

A page the worker cuts SHALL keep every column of its own page, so a leaning
gutter SHALL NOT clip either page. A page the worker copies whole SHALL keep the
stored bytes of its source page.

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

### Requirement: Splitting asks the sidecar where to cut

The split worker SHALL ask the sidecar where to cut, one batch of pages at a
time, and SHALL NOT decide it itself. The worker SHALL send the paths of the
stored pages, never their bytes.

A page the sidecar reports as no spread SHALL reach the copy whole. A page the
sidecar reports as a spread SHALL reach the copy as two pages.

#### Scenario: A book runs in batches

- **WHEN** the worker splits a source book of 100 pages
- **THEN** the worker asks the sidecar several times
- **AND** the reported page count of the copy grows while the job runs

#### Scenario: The sidecar refuses a batch

- **WHEN** the sidecar refuses a batch because it cannot read a page
- **THEN** the split job fails
- **AND** the copy holds no page of that attempt after the last attempt

## MODIFIED Requirements

### Requirement: Splitting needs no external configuration

The Split action SHALL work with no object storage and no remote endpoint. It
SHALL need the sidecar service alone, which runs beside the backend and needs no
account and no graphics card.

The system SHALL offer the action whenever the stage allows it.

#### Scenario: The dewarp settings are absent

- **WHEN** the object storage and endpoint settings are absent
- **THEN** the Split action still works
- **AND** a book still reaches the `split` stage

#### Scenario: The sidecar is unreachable

- **WHEN** the sidecar does not answer while a split job runs
- **THEN** the job fails
- **AND** the copy carries a reason that names the sidecar
