## Purpose

Dewarps the pages of a book on a remote GPU endpoint. The result is a second
book in the next stage that keeps a reference to the book it came from.

## Requirements

### Requirement: The Dewarp action leads to the Dewarped stage

The system SHALL offer the Dewarp action for a complete book in the `uploaded`
stage that has no copy. The copy of a Dewarp action SHALL carry the stage
`dewarped`.

#### Scenario: A user dewarps an uploaded book

- **WHEN** a user runs the Dewarp action on a complete book in the `uploaded`
  stage
- **THEN** the system creates a copy in the `dewarped` stage
- **AND** the copy appears in the Dewarped tab while it runs

#### Scenario: The copy finishes

- **WHEN** the dewarp job stores every page of the copy
- **THEN** the copy state becomes `complete`
- **AND** its stage is still `dewarped`
- **AND** it is ready for the Split action

#### Scenario: The book is already dewarped

- **WHEN** a user runs the Dewarp action on a book in the `dewarped` stage
- **THEN** the system rejects the request and names the reason

### Requirement: Dewarping runs as a background job

The system SHALL dewarp a book in a background worker, not in the request that
starts it. The system SHALL move the copy through the states `pending`,
`processing`, and then `complete` or `failed`. The copy SHALL keep the stage
`dewarped` through the whole job. The system SHALL NOT change the state, the
stage, or the pages of the source book.

#### Scenario: The job starts

- **WHEN** the worker picks up a dewarp job for a copy in state `pending`
- **THEN** the copy state becomes `processing`
- **AND** the copy keeps the stage `dewarped`
- **AND** the system broadcasts a `book.updated` event

#### Scenario: The source book is untouched

- **WHEN** a dewarp job runs to its end, whatever the result
- **THEN** the source book keeps its state, its stage, and all of its pages

#### Scenario: One job at a time

- **WHEN** a user starts dewarp jobs for three books
- **THEN** the system runs them one after another
- **AND** the HTTP server and the other queues keep working meanwhile

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
failure reason, its stage, and the reference to the source book. The system
SHALL set the copy to `failed` only after the job exhausts its retries.

#### Scenario: The job fails

- **WHEN** a dewarp job fails on its last attempt
- **THEN** the copy state becomes `failed`
- **AND** the copy keeps the stage `dewarped`
- **AND** the Dewarped tab shows the copy and its reason at the head of the list
- **AND** the source book carries a warning mark and offers no action

#### Scenario: Attempts remain

- **WHEN** a dewarp attempt fails and further attempts remain
- **THEN** the copy state stays `processing`
- **AND** the system runs the job again

#### Scenario: A user retries a failed copy

- **WHEN** a user activates the retry control on a failed copy
- **THEN** the same copy returns to state `pending`
- **AND** the system queues its dewarp job again
- **AND** no third book exists

#### Scenario: A retry succeeds

- **WHEN** a retried job stores every page
- **THEN** the copy holds exactly one set of pages
- **AND** no page index appears twice

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
