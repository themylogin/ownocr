## Purpose

Splits the pages of a book into single pages. The result is a copy in the `split`
stage. The worker cuts every page that holds a spread into its two pages and
copies every other page whole, so a cover reaches the copy unchanged.

## Requirements

### Requirement: The Split action leads to the Split stage

The system SHALL offer the Split action for a complete book in the `uploaded` or
the `dewarped` stage that has no copy. The copy of a Split action SHALL carry
the stage `split`.

A book that already carries the stage `split` SHALL NOT be offered the Split
action again. It SHALL still be offered the OCR action, which comes after that
stage.

#### Scenario: A user splits an uploaded book

- **WHEN** a user runs the Split action on a complete book in the `uploaded`
  stage
- **THEN** the system creates a copy in the `split` stage
- **AND** the copy appears in the Split tab while it runs

#### Scenario: A user splits a dewarped book

- **WHEN** a user runs the Split action on a complete book in the `dewarped`
  stage
- **THEN** the system creates a copy in the `split` stage

#### Scenario: A book is already split

- **WHEN** a user runs the Split action on a book in the `split` stage
- **THEN** the system rejects the request and names the reason

#### Scenario: A book is already done

- **WHEN** a user runs the Split action on a book in the `ocr` stage
- **THEN** the system rejects the request and names the reason

### Requirement: Splitting runs as a background job

The system SHALL split a book in a background worker, not in the request that
starts it. The system SHALL move the copy through the states `pending`,
`processing`, and then `complete` or `failed`. The system SHALL run one split
job at a time, so a large book does not starve the other queues.

#### Scenario: The job starts

- **WHEN** the worker picks up a split job for a copy in state `pending`
- **THEN** the copy state becomes `processing`
- **AND** the system broadcasts a `book.updated` event

#### Scenario: The job finishes

- **WHEN** the worker stores every page of the copy
- **THEN** the copy state becomes `complete`
- **AND** the system broadcasts a `book.updated` event

#### Scenario: Several books are split at once

- **WHEN** a user starts split jobs for three books
- **THEN** the system runs them one after another
- **AND** the HTTP server and the other queues keep working meanwhile

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

### Requirement: Splitting copies the pages of the source book

The split worker SHALL give the copy one or more pages for every page of the
source book. The pages of the copy SHALL follow the order of the source book, and
the pages made from one source page SHALL sit next to each other in reading
order. Every page of the copy SHALL name the source page it was made from.

Every page of the copy SHALL have its own stored files, so removing the source
book SHALL NOT damage the copy. Two pages made from one source page SHALL hold
separate files, so removing one SHALL NOT damage the other. The system SHALL
record the dimensions of every page of the copy.

The system SHALL report the number of pages the copy holds, not the number the
source book holds.

#### Scenario: A book is split

- **WHEN** the worker splits a source book of 120 pages
- **THEN** every page of the source book has at least one page made from it in
  the copy
- **AND** every page of the copy names exactly one page of the source book
- **AND** the pages of the copy follow the order of the source book

#### Scenario: The copy reports its own page count

- **WHEN** a split job completes
- **THEN** the copy reports the number of pages it holds
- **AND** that count may differ from the count of the source book

#### Scenario: The pages are independent

- **WHEN** the pages of a copy are stored
- **THEN** no page file of the copy is a page file of the source book
- **AND** no two pages of the copy share a page file

#### Scenario: The source book is untouched

- **WHEN** a split job runs to its end, whatever the result
- **THEN** the source book keeps its state, its stage, and all of its pages

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

### Requirement: Splitting reports failure

When a split job cannot finish, the system SHALL set the copy state to `failed`
and SHALL store a readable reason. The system SHALL set `failed` only after the
job exhausts its retries.

#### Scenario: The job fails

- **WHEN** a split job fails on its last attempt
- **THEN** the copy state becomes `failed`
- **AND** the interface shows the copy and its reason

#### Scenario: Attempts remain

- **WHEN** a split attempt fails and further attempts remain
- **THEN** the copy state stays `processing`
- **AND** the system runs the job again

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
