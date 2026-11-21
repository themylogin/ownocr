## Purpose

Turns an uploaded source into an ordered set of stored page images with
thumbnails, then marks the book complete. The book carries the first stage from
the moment it is created.

## Requirements

### Requirement: Ingest runs as a background job

The system SHALL process an uploaded book in a background worker, not in the
upload request. The system SHALL move the book through the states `pending`,
`processing`, and then `complete` or `failed`. The book SHALL keep the stage
`uploaded` through the whole job.

#### Scenario: A queued book starts

- **WHEN** the worker picks up an ingest job for a book in state `pending`
- **THEN** the book state becomes `processing`
- **AND** the book keeps the stage `uploaded`
- **AND** the system broadcasts a `book.updated` event

#### Scenario: Ingest finishes

- **WHEN** the worker stores every page of a book
- **THEN** the book state becomes `complete`
- **AND** the book keeps the stage `uploaded`
- **AND** the system broadcasts a `book.updated` event

### Requirement: Ingest splits a PDF into ordered page images

The system SHALL render every page of an uploaded PDF to a raster image. The
page order SHALL match the page order in the PDF.

#### Scenario: A ten page PDF

- **WHEN** the worker ingests a PDF with ten pages
- **THEN** the book has ten pages
- **AND** page index 1 holds the first PDF page and page index 10 holds the last

#### Scenario: A PDF that cannot be read

- **WHEN** the worker cannot open an uploaded PDF
- **THEN** the book state becomes `failed`
- **AND** the book carries the failure reason

### Requirement: Ingest keeps the upload order of an image set

The system SHALL order the pages of an uploaded image set by the order in which
the files arrive in the upload. The system SHALL NOT reorder them by file name,
by size, or by any other property.

#### Scenario: Files arrive out of name order

- **WHEN** the worker ingests files uploaded as `b.jpg`, then `a.jpg`, then `c.jpg`
- **THEN** the page order is `b.jpg`, `a.jpg`, `c.jpg`

#### Scenario: File names carry no numbering

- **WHEN** the worker ingests files whose names contain no page numbers
- **THEN** the page order still matches the upload order

#### Scenario: Two uploaded files share a name

- **WHEN** an upload contains two files both named `scan.jpg`
- **THEN** both become pages
- **AND** their page indexes follow their upload order

### Requirement: Every page gets a unique file name

The system SHALL assign each page a unique identifier and SHALL derive every
stored file name from that identifier. The system SHALL NOT reuse the uploaded
file name for stored page files.

#### Scenario: Two books contain a file with the same name

- **WHEN** two books each contain a page uploaded as `scan.jpg`
- **THEN** the two pages have different stored file names
- **AND** neither page overwrites the other

### Requirement: Ingest stores three renditions of every page

For every page the system SHALL store an original rendition, a 1280 px display
rendition, and a 480 px thumbnail rendition. A thumbnail SHALL preserve the
aspect ratio of its page. Its longest side SHALL equal the target size.

#### Scenario: A landscape page

- **WHEN** the worker stores a page of 3000 x 2000 pixels
- **THEN** the display rendition is 1280 x 853 pixels
- **AND** the thumbnail rendition is 480 x 320 pixels

#### Scenario: A portrait page

- **WHEN** the worker stores a page of 2000 x 3000 pixels
- **THEN** the display rendition is 853 x 1280 pixels
- **AND** the thumbnail rendition is 320 x 480 pixels

#### Scenario: The original is preserved

- **WHEN** a user uploads a JPEG page
- **THEN** the stored original holds the uploaded bytes unchanged

### Requirement: Ingest never upscales a rendition

The system SHALL NOT enlarge a page to reach a target size. When a page is
smaller than a target size, that rendition SHALL keep the page dimensions.

#### Scenario: A page smaller than the display size

- **WHEN** the worker stores a page of 900 x 600 pixels
- **THEN** the display rendition is 900 x 600 pixels
- **AND** the thumbnail rendition is 480 x 320 pixels

#### Scenario: A page smaller than the thumbnail size

- **WHEN** the worker stores a page of 300 x 200 pixels
- **THEN** all three renditions are 300 x 200 pixels

### Requirement: Ingest records each page with its index and dimensions

The system SHALL store one row per page. The row SHALL carry the book, a
one-based page index, the stored file identifier, and the pixel width and height
of the page.

#### Scenario: Page rows after ingest

- **WHEN** the worker finishes a book of five pages
- **THEN** five page rows exist for that book
- **AND** their indexes are 1 through 5 with no gaps and no duplicates
- **AND** each row carries the width and height of its page

### Requirement: The uploaded source is kept

The system SHALL keep the uploaded source files after ingest. This allows a
later re-render at a different resolution.

#### Scenario: After a PDF ingest

- **WHEN** the worker finishes ingesting a PDF
- **THEN** the uploaded PDF is still readable from storage

### Requirement: An uploaded book starts in the first stage

The system SHALL give an uploaded book the first stage when it creates the book
row, before ingest runs. The first stage SHALL be `uploaded`.

#### Scenario: A user uploads a book

- **WHEN** the system accepts an upload
- **THEN** the new book carries the stage `uploaded` in state `pending`
- **AND** the Uploaded tab shows it at once

#### Scenario: A book is still processing

- **WHEN** a book is in state `pending` or `processing`
- **THEN** its stage is `uploaded`
- **AND** it appears in the Uploaded tab and in no other tab

#### Scenario: Ingest fails

- **WHEN** a book reaches state `failed`
- **THEN** its stage is still `uploaded`
- **AND** the Uploaded tab shows it at the head of its list

### Requirement: Ingest reports failure

When ingest cannot finish, the system SHALL set the book state to `failed` and
SHALL store a human readable reason. The system SHALL set `failed` only after
the job exhausts its retries.

#### Scenario: A page image is corrupt

- **WHEN** the worker cannot decode one uploaded image
- **THEN** the book state becomes `failed` after the final retry
- **AND** the stored reason names the file that failed

#### Scenario: Retries remain

- **WHEN** an ingest attempt fails and further retries remain
- **THEN** the book state stays `processing`
- **AND** the system retries the job

### Requirement: Ingest is repeatable for one book

A retried ingest job SHALL leave the book with exactly one set of pages. The
system SHALL discard the results of an interrupted attempt before it retries.

#### Scenario: The worker dies mid book

- **WHEN** the worker stores three of ten pages and then restarts
- **THEN** the retry produces exactly ten page rows
- **AND** no page index appears twice

### Requirement: Stored page files are readable over HTTP

The system SHALL serve every stored rendition over HTTP under a stable path. A
page record SHALL expose the URL of each of its renditions.

#### Scenario: A browser loads a thumbnail

- **WHEN** a browser requests the thumbnail URL of a stored page
- **THEN** the system responds `200 OK` with the image bytes
- **AND** the response carries an image content type

#### Scenario: A request for an unknown file

- **WHEN** a browser requests a file path that does not exist
- **THEN** the system responds `404 Not Found`

#### Scenario: A request escapes the storage root

- **WHEN** a request contains a path that traverses outside the storage root
- **THEN** the system responds `404 Not Found`
- **AND** the system serves no file from outside the root
