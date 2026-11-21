## Purpose

Dewarps the pages of a book on a remote GPU endpoint. The result is a second
book in the next stage that keeps a reference to the book it came from.

## ADDED Requirements

### Requirement: A book at the dewarp stage offers a Dewarp action

The system SHALL offer a Dewarp action for a complete book whose stage is
`dewarp` and which is not processed. The book view SHALL show the action as a
button. A right click on a book card in a stage list SHALL open a context menu
that holds the same action. The system SHALL NOT offer the action for a
processed book, or for a book at another stage.

#### Scenario: A user opens a book at the dewarp stage

- **WHEN** a user opens a complete book whose stage is `dewarp`
- **THEN** the book view shows a button named "Dewarp"

#### Scenario: A user right clicks a book card

- **WHEN** a user right clicks a book card in the Dewarp tab
- **THEN** a context menu opens at the pointer
- **AND** the menu holds an item named "Dewarp"

#### Scenario: A user dismisses the context menu

- **WHEN** the context menu is open and the user presses Escape or clicks away
- **THEN** the menu closes
- **AND** no action runs

#### Scenario: The book is already processed

- **WHEN** a user opens a processed book, or right clicks its card
- **THEN** the system offers no Dewarp action

#### Scenario: The book is at another stage

- **WHEN** a user opens a complete book whose stage is `split`
- **THEN** the system offers no Dewarp action

### Requirement: The Dewarp action creates a copy at once

The Dewarp action SHALL create a second book immediately. The copy SHALL carry
the same name as the source book and a reference to it. The copy SHALL start in
state `pending` with no stage, and SHALL enter the stage `split` when its pages
are stored. The system SHALL create the copy and queue its dewarp job together,
so neither can exist without the other.

#### Scenario: A user runs the action

- **WHEN** a user activates the Dewarp action on a book named "Atlas"
- **THEN** a second book named "Atlas" exists in state `pending`
- **AND** that book references the source book
- **AND** the system broadcasts a `book.created` event for it

#### Scenario: The copy is visible while it runs

- **WHEN** the copy is in state `pending` or `processing`
- **THEN** it appears in the area for books that are not complete
- **AND** it appears in no stage list

#### Scenario: The copy finishes

- **WHEN** the dewarp job stores every page of the copy
- **THEN** the copy state becomes `complete`
- **AND** its stage is `split`
- **AND** it appears in the Split tab

#### Scenario: The action runs twice

- **WHEN** a user runs the action on a book that already has a copy that has not
  failed
- **THEN** the system rejects the request
- **AND** it creates no second copy

#### Scenario: The book cannot be dewarped

- **WHEN** a user runs the action on a book that is not complete, or whose stage
  is not `dewarp`
- **THEN** the system rejects the request and names the reason

#### Scenario: The book does not exist

- **WHEN** a user runs the action for an unknown book
- **THEN** the system responds `404 Not Found`

### Requirement: A reference marks the source book processed

A book SHALL be processed exactly while another book references it. The system
SHALL derive the processed mark from that reference alone, and SHALL NOT keep a
separate flag. The state of the referencing book SHALL NOT change the mark.

#### Scenario: The copy exists

- **WHEN** a book has a copy in any state
- **THEN** the source book is processed

#### Scenario: No copy exists

- **WHEN** no book references a book
- **THEN** that book is not processed
- **AND** it offers the Dewarp action again

#### Scenario: The copy fails

- **WHEN** the copy reaches state `failed`
- **THEN** the source book stays processed

#### Scenario: The copy is deleted

- **WHEN** the copy is removed from the system
- **THEN** the source book is no longer processed

### Requirement: A processed book shows a checkmark without a reload

The system SHALL mark the source book as processed in the interface as soon as
the action succeeds. The mark SHALL be a checkmark in the top right corner of
the book. The book SHALL fade. The system SHALL apply the same mark to every
connected client from a server event.

#### Scenario: The user runs the action in the book view

- **WHEN** a user activates the Dewarp button
- **THEN** the button gives way to a checkmark in the top right corner
- **AND** the book fades
- **AND** the view needs no reload

#### Scenario: Another user runs the action

- **WHEN** a second user dewarps a book that the first user has in view
- **THEN** the first user's card for that book shows the checkmark and fades
- **AND** the first user reloads nothing

#### Scenario: A processed book after a reload

- **WHEN** a user reloads the Dewarp tab
- **THEN** every processed book still shows the checkmark and stays faded

### Requirement: Dewarping runs as a background job

The system SHALL dewarp a book in a background worker, not in the request that
starts it. The system SHALL move the copy through the states `pending`,
`processing`, and then `complete` or `failed`. The system SHALL NOT change the
state, the stage, or the pages of the source book.

#### Scenario: The job starts

- **WHEN** the worker picks up a dewarp job for a copy in state `pending`
- **THEN** the copy state becomes `processing`
- **AND** the system broadcasts a `book.updated` event

#### Scenario: The source book is untouched

- **WHEN** a dewarp job runs to its end, whatever the result
- **THEN** the source book keeps its state, its stage, and all of its pages

#### Scenario: One job at a time

- **WHEN** a user starts dewarp jobs for three books
- **THEN** the system runs them one after another
- **AND** the HTTP server and the ingest queue keep working meanwhile

### Requirement: The system exchanges pages with the endpoint through object storage

The system SHALL upload the original rendition of every source page to a bucket
in object storage, under a prefix unique to the copy. The system SHALL then
start one job on the dewarp endpoint and tell it that bucket and those prefixes.
The system SHALL NOT put page bytes in the request body. The uploaded object
names SHALL carry the page order.

#### Scenario: A book goes to dewarp

- **WHEN** the worker starts a job for a copy of a book with 12 pages
- **THEN** 12 objects exist under the input prefix of that copy
- **AND** each object holds the original rendition bytes of its page
- **AND** the object names sort into page order

#### Scenario: The endpoint job starts

- **WHEN** every page is uploaded
- **THEN** the system starts one job on the dewarp endpoint
- **AND** the request carries the bucket, the input prefix, and the output prefix
- **AND** the request carries the credentials the endpoint needs to reach them

#### Scenario: Two copies run

- **WHEN** two copies of different books are dewarped
- **THEN** their prefixes differ
- **AND** neither reads or overwrites the objects of the other

### Requirement: The system monitors the endpoint job to its end

The system SHALL poll the dewarp endpoint until the job reaches a final result.
A job that ends in any way other than success SHALL fail the copy and SHALL
store the reason the endpoint reports.

#### Scenario: The job is still running

- **WHEN** the endpoint reports that the job is queued or in progress
- **THEN** the system waits and asks again
- **AND** the copy stays in state `processing`

#### Scenario: The job succeeds

- **WHEN** the endpoint reports success
- **THEN** the system downloads the output objects

#### Scenario: The job fails at the endpoint

- **WHEN** the endpoint reports a failure, a cancellation, or a timeout
- **THEN** the dewarp job fails
- **AND** the stored reason repeats what the endpoint reported

#### Scenario: The endpoint cannot be reached

- **WHEN** the endpoint does not answer
- **THEN** the dewarp job fails
- **AND** the stored reason names the endpoint

### Requirement: The dewarped pages become the pages of the copy

The system SHALL store one page of the copy for every page of the source book.
The page order of the copy SHALL match the page order of the source book. Every
page SHALL get the same three renditions as an ingested page: an original, a
1280 px display rendition, and a 480 px thumbnail. The recorded width and height
SHALL be those of the dewarped image, not those of the source page.

#### Scenario: A twelve page book

- **WHEN** the worker finishes a copy of a book with 12 pages
- **THEN** the copy has 12 pages with the indexes 1 through 12
- **AND** page 1 of the copy is the dewarped page 1 of the source book

#### Scenario: The renditions of a dewarped page

- **WHEN** the worker stores a dewarped page
- **THEN** the original, the display, and the thumbnail rendition all exist
- **AND** a browser can load each of them

#### Scenario: Dewarping changes the page size

- **WHEN** a dewarped page has other dimensions than its source page
- **THEN** the page row of the copy carries the dewarped dimensions
- **AND** the grid cell of the copy follows from those dimensions

### Requirement: A page that cannot be dewarped fails the whole copy

The system SHALL fail the copy when the endpoint cannot dewarp every page. The
system SHALL NOT store a copy with fewer pages than the source book. The stored
reason SHALL name the pages that failed.

#### Scenario: One page fails at the endpoint

- **WHEN** the endpoint dewarps 11 of 12 pages and reports the twelfth as failed
- **THEN** the copy state becomes `failed`
- **AND** the reason names the failed page

#### Scenario: An output object is missing

- **WHEN** the endpoint reports success but an output object is absent
- **THEN** the copy state becomes `failed`
- **AND** the reason names the missing page

### Requirement: The system removes the exchange objects after a success

The system SHALL delete every input and output object of a copy when the copy is
complete. The system SHALL clear the prefix of a copy before it uploads pages
again, so a retry cannot read an object from an earlier attempt.

#### Scenario: A job finishes

- **WHEN** the copy reaches state `complete`
- **THEN** no object remains under the prefix of that copy

#### Scenario: A job is retried

- **WHEN** the worker starts an attempt for a copy after a failed attempt
- **THEN** it removes every object left under the prefix of that copy first

### Requirement: A failed dewarp keeps the copy and allows a retry

The system SHALL keep the copy when a dewarp job fails. The copy SHALL hold the
failure reason and the reference to the source book. The system SHALL offer a
retry for a failed copy. A retry SHALL reuse the same copy and SHALL NOT create
a third book. The system SHALL set the copy to `failed` only after the job
exhausts its retries.

#### Scenario: The job fails

- **WHEN** a dewarp job fails on its last attempt
- **THEN** the copy state becomes `failed`
- **AND** the interface shows the copy and its reason
- **AND** the source book stays processed and shows no Dewarp action

#### Scenario: Attempts remain

- **WHEN** a dewarp attempt fails and further attempts remain
- **THEN** the copy state stays `processing`
- **AND** the system runs the job again

#### Scenario: A user retries a failed copy

- **WHEN** a user activates the retry action on a failed copy
- **THEN** the same copy returns to state `pending`
- **AND** the system queues its dewarp job again
- **AND** no third book exists

#### Scenario: A retry succeeds

- **WHEN** a retried job stores every page
- **THEN** the copy holds exactly one set of pages
- **AND** no page index appears twice

### Requirement: Dewarp is optional configuration

The system SHALL start and serve every other feature when the object storage or
the dewarp endpoint is not configured. The Dewarp action SHALL then report that
dewarp is not configured, and SHALL create no copy.

#### Scenario: The server starts without the settings

- **WHEN** the object storage and endpoint settings are absent
- **THEN** the server starts
- **AND** upload, ingest, and browsing all work

#### Scenario: A user runs the action without the settings

- **WHEN** a user activates the Dewarp action and dewarp is not configured
- **THEN** the system rejects the request with a message that says so
- **AND** it creates no copy

### Requirement: The dewarp endpoint runs the image as a serverless worker

The dewarp endpoint SHALL accept a job that names a bucket, an input prefix, and
an output prefix. It SHALL read every input object, dewarp it, and write one
output image per input object under the output prefix. The output name SHALL
derive from the input name, so the caller can pair them. The result SHALL report
each page that failed. The worker SHALL load its models once and SHALL reuse
them for later jobs. The image SHALL keep its command line tool.

#### Scenario: A job dewarps a prefix

- **WHEN** the endpoint runs a job for a prefix that holds 12 images
- **THEN** 12 output objects exist under the output prefix
- **AND** each output name derives from the name of its input

#### Scenario: One input cannot be read

- **WHEN** one input object holds bytes that are not a readable image
- **THEN** the result names that object as failed
- **AND** the result still lists the pages that succeeded

#### Scenario: A second job arrives

- **WHEN** the same worker process runs a second job
- **THEN** it does not load the models again

#### Scenario: The image runs from the command line

- **WHEN** an operator runs the image against a local directory of images
- **THEN** it writes the dewarped images to the output directory as before
