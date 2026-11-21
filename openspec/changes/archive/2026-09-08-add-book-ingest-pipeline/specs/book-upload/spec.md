## Purpose

Accepts a book into the system as a named PDF or a named set of page images, and
queues that book for ingest. This is the only entry point for new books.

## ADDED Requirements

### Requirement: Book upload accepts a PDF or an image set

The system SHALL accept a book upload that contains either exactly one PDF file
or one or more JPEG and PNG files. The system SHALL reject any other combination.

#### Scenario: Upload of a single PDF

- **WHEN** a user uploads one file with content type `application/pdf`
- **THEN** the system accepts the upload and responds `201 Created`
- **AND** the response body contains the new book with state `pending`

#### Scenario: Upload of an image set

- **WHEN** a user uploads three files with content type `image/jpeg` or `image/png`
- **THEN** the system accepts the upload and responds `201 Created`

#### Scenario: Upload mixes a PDF with images

- **WHEN** a user uploads one PDF file and one JPEG file together
- **THEN** the system responds `422 Unprocessable Entity`
- **AND** the response states that a book is either one PDF or a set of images

#### Scenario: Upload contains two PDF files

- **WHEN** a user uploads two files with content type `application/pdf`
- **THEN** the system responds `422 Unprocessable Entity`

#### Scenario: Upload contains an unsupported file type

- **WHEN** a user uploads a file that is not a PDF, a JPEG, or a PNG
- **THEN** the system responds `422 Unprocessable Entity`
- **AND** the response names the rejected file

#### Scenario: Upload contains no files

- **WHEN** a user submits an upload with a name but no files
- **THEN** the system responds `422 Unprocessable Entity`

### Requirement: A book name is required

Every book SHALL have a non-empty name. The system SHALL reject an upload whose
name is missing or contains only whitespace.

#### Scenario: Name is missing

- **WHEN** a user submits an upload without a name field
- **THEN** the system responds `422 Unprocessable Entity`
- **AND** the response states that the name is required

#### Scenario: Name is only whitespace

- **WHEN** a user submits an upload with the name `"   "`
- **THEN** the system responds `422 Unprocessable Entity`

#### Scenario: Name has surrounding whitespace

- **WHEN** a user submits an upload with the name `"  Moby Dick  "`
- **THEN** the system stores the name as `"Moby Dick"`

#### Scenario: Two books share a name

- **WHEN** a user uploads a book whose name matches an existing book
- **THEN** the system accepts the upload
- **AND** both books exist independently

### Requirement: The upload form prefills the name from a PDF file name

The upload interface SHALL prefill the name field with the PDF file name without
its extension. The user SHALL be able to replace the prefilled name.

#### Scenario: User selects a PDF

- **WHEN** the user selects the file `Moby Dick (1851).pdf`
- **THEN** the name field shows `Moby Dick (1851)`

#### Scenario: User selects images

- **WHEN** the user selects a set of image files
- **THEN** the name field stays empty
- **AND** the submit control stays disabled until the user types a name

#### Scenario: User edits the prefilled name

- **WHEN** the user selects a PDF and then edits the name field
- **THEN** the system stores the edited name

### Requirement: Upload is reachable from every stage tab

The system SHALL show an upload control on every stage tab.

#### Scenario: User opens the upload dialog

- **WHEN** the user activates the upload control on any stage tab
- **THEN** the system shows a form with a name field and a file selector

### Requirement: The upload preserves the file order

The system SHALL record the order in which the files arrive and SHALL carry that
order through to the page order. The upload interface SHALL send the files in
the order the user selected them.

#### Scenario: The user selects files in a picker

- **WHEN** the user selects several images and submits the form
- **THEN** the request carries the files in the order the picker listed them

#### Scenario: The interface shows what it will send

- **WHEN** the user has selected files but has not submitted
- **THEN** the interface lists the file names in the order it will send them

### Requirement: The book row and the ingest job commit together

The system SHALL create the book row and enqueue its ingest job in one database
transaction. A failure SHALL leave neither the row nor the job.

#### Scenario: The transaction fails

- **WHEN** the database rejects the commit of a book upload
- **THEN** no book row exists
- **AND** no ingest job is queued
- **AND** the system responds `500 Internal Server Error`

#### Scenario: The transaction succeeds

- **WHEN** the system commits a book upload
- **THEN** exactly one book row exists with state `pending`
- **AND** exactly one ingest job is queued for that book

### Requirement: Uploaded bytes reach durable storage before the response

The system SHALL write the uploaded files to durable storage before it responds
`201 Created`. The system SHALL NOT buffer a whole upload in memory.

#### Scenario: The client disconnects mid upload

- **WHEN** a client disconnects before the upload finishes
- **THEN** the system creates no book row
- **AND** the system removes any partial files it wrote

#### Scenario: Upload of a large book

- **WHEN** a user uploads a book of 800 MB
- **THEN** the system accepts it
- **AND** backend memory use stays independent of the upload size

### Requirement: Upload size has a configured limit

The system SHALL enforce a maximum total upload size. The limit SHALL be
configurable and SHALL default to 2 GiB.

#### Scenario: Upload exceeds the limit

- **WHEN** a user uploads files whose total size exceeds the configured limit
- **THEN** the system responds `413 Content Too Large`
- **AND** the system stores no book

### Requirement: A new book is announced to connected clients

The system SHALL broadcast a `book.created` event to every connected WebSocket
client when it accepts an upload.

#### Scenario: A second browser is open

- **WHEN** one user uploads a book
- **THEN** every connected client receives a `book.created` event
- **AND** the event payload contains the book id, name, and state
