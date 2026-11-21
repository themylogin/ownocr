## RENAMED Requirements

- FROM: `### Requirement: The OCR action leads to the Done stage`
- TO: `### Requirement: The OCR action leads to the OCR stage`

## ADDED Requirements

### Requirement: The OCR action carries a model profile

The system SHALL define two OCR profiles: `v6` and `vl`. The profile `v6` SHALL
run the PP-OCRv6 engine alone. The profile `vl` SHALL run the PaddleOCR-VL
engine and the PP-OCRv6 engine together.

The system SHALL require a profile on every OCR action, and SHALL refuse an OCR
action that names no profile or an unknown one. The system SHALL record the
profile on the copy the action creates.

The profile SHALL say which engine holds the text of record for that copy. For
`v6` the PP-OCRv6 result answers. For `vl` the PaddleOCR-VL result answers, and
the PP-OCRv6 result carries the line boxes and the confidence PaddleOCR-VL does
not report.

#### Scenario: A user picks the normal profile

- **WHEN** a user runs the OCR action with the profile `v6`
- **THEN** the copy records the profile `v6`
- **AND** the endpoint job asks for the PP-OCRv6 engine alone

#### Scenario: A user picks the extra profile

- **WHEN** a user runs the OCR action with the profile `vl`
- **THEN** the copy records the profile `vl`
- **AND** the endpoint job asks for both engines

#### Scenario: The request names no profile

- **WHEN** a user runs the OCR action without a profile
- **THEN** the system rejects the request and names the reason
- **AND** no copy exists for that book

#### Scenario: The request names an unknown profile

- **WHEN** a user runs the OCR action with a profile that is not `v6` or `vl`
- **THEN** the system rejects the request and names the reason

#### Scenario: A retry keeps the profile

- **WHEN** a user retries a copy that failed with the profile `vl`
- **THEN** the retried job asks for both engines again
- **AND** the copy still records the profile `vl`

### Requirement: Every page of a copy holds one result per engine

The system SHALL store one result for every engine that read a page. A result
SHALL name its engine, SHALL hold the text that engine produced, and SHALL hold
the result that engine produced in full.

The text SHALL be what the engine wrote, unchanged. The system SHALL NOT rewrite
it, merge it with the text of another engine, or drop part of it. PP-OCRv6
writes its recognised lines in detection order. PaddleOCR-VL writes markdown.

The full result SHALL hold what the text does not. PaddleOCR-VL leaves footnote
and page number blocks out of its markdown, and PP-OCRv6 reports a box and a
confidence for every line. Both SHALL survive in the stored result.

A page SHALL hold at most one result per engine. A retry SHALL replace the
results of the copy, so no result of an earlier attempt remains.

#### Scenario: A page read with the normal profile

- **WHEN** the worker stores page 3 of a copy whose profile is `v6`
- **THEN** page 3 holds one result, naming the PP-OCRv6 engine
- **AND** the result holds the recognised lines and the full engine result

#### Scenario: A page read with the extra profile

- **WHEN** the worker stores page 3 of a copy whose profile is `vl`
- **THEN** page 3 holds two results, one per engine
- **AND** the PaddleOCR-VL result holds markdown
- **AND** the PP-OCRv6 result holds a box and a confidence for every line

#### Scenario: A page holds no text

- **WHEN** an engine returns an empty text for a page
- **THEN** the result of that engine for that page exists and its text is empty
- **AND** the copy still completes

#### Scenario: A retry replaces the results

- **WHEN** a retried job stores every page of a copy again
- **THEN** each page holds exactly one result per engine of its profile
- **AND** no result of the earlier attempt remains

#### Scenario: A page is deleted

- **WHEN** a page of a copy is removed
- **THEN** every result of that page is removed with it

## MODIFIED Requirements

### Requirement: The OCR action leads to the OCR stage

The system SHALL offer the OCR action for a complete book that has no copy, in
every stage before `ocr`. The copy of an OCR action SHALL carry the stage `ocr`.

The `ocr` stage names the end of the pipeline. A book that carries it SHALL be
offered no action.

#### Scenario: A user reads a split book

- **WHEN** a user runs the OCR action on a complete book in the `split` stage
- **THEN** the system creates a copy in the `ocr` stage
- **AND** the copy appears in the OCR tab while it runs

#### Scenario: A user reads an uploaded book

- **WHEN** a user runs the OCR action on a complete book in the `uploaded` stage
- **THEN** the system creates a copy in the `ocr` stage

#### Scenario: The copy finishes

- **WHEN** the OCR job stores every page of the copy
- **THEN** the copy state becomes `complete`
- **AND** its stage is still `ocr`
- **AND** it offers no further action

#### Scenario: The book is already read

- **WHEN** a user runs the OCR action on a book in the `ocr` stage
- **THEN** the system rejects the request and names the reason

### Requirement: OCR runs as a background job

The system SHALL run OCR in a background worker, not in the request that starts
it. The system SHALL move the copy through the states `pending`, `processing`,
and then `complete` or `failed`. The copy SHALL keep the stage `ocr` through the
whole job. The system SHALL NOT change the state, the stage, or the pages of the
source book.

#### Scenario: The job starts

- **WHEN** the worker picks up an OCR job for a copy in state `pending`
- **THEN** the copy state becomes `processing`
- **AND** the copy keeps the stage `ocr`
- **AND** the system broadcasts a `book.updated` event

#### Scenario: The source book is untouched

- **WHEN** an OCR job runs to its end, whatever the result
- **THEN** the source book keeps its state, its stage, and all of its pages

#### Scenario: One job at a time

- **WHEN** a user starts OCR jobs for three books
- **THEN** the system runs them one after another
- **AND** the HTTP server and the other queues keep working meanwhile

### Requirement: A failed OCR keeps the copy and allows a retry

The system SHALL keep the copy when an OCR job fails. The copy SHALL hold the
failure reason, its stage, its profile, and the reference to the source book.
The system SHALL set the copy to `failed` only after the job exhausts its
retries.

#### Scenario: The job fails

- **WHEN** an OCR job fails on its last attempt
- **THEN** the copy state becomes `failed`
- **AND** the copy keeps the stage `ocr`
- **AND** the OCR tab shows the copy and its reason at the head of the list
- **AND** the source book carries a warning mark and offers no action

#### Scenario: Attempts remain

- **WHEN** an OCR attempt fails and further attempts remain
- **THEN** the copy state stays `processing`
- **AND** the system runs the job again

#### Scenario: A user retries a failed copy

- **WHEN** a user activates the retry control on a failed copy
- **THEN** the same copy returns to state `pending`
- **AND** the system queues its OCR job again with the profile the copy holds
- **AND** no third book exists

### Requirement: The copy holds the pages of its source book

The system SHALL store one page of the copy for every page of the source book.
The page order of the copy SHALL match the page order of the source book.

A page of an OCR copy SHALL show the image of its source page, and SHALL NOT
hold an image of its own. OCR reads a page, it does not change it, so the copy
builds no renditions and writes no image file. The original, the display and the
thumbnail rendition of a page of the copy SHALL be those of its source page.

The system SHALL keep this safe by two rules it already holds. A book that has a
copy offers no page rollback and no re-cut, so a source image cannot change
under the copy. Rejecting a book destroys every copy made from it in the same
request, so no page of a copy can outlive the image it shows.

#### Scenario: A twelve page book

- **WHEN** the worker finishes a copy of a book with 12 pages
- **THEN** the copy has 12 pages with the indexes 1 through 12
- **AND** page 1 of the copy shows page 1 of the source book

#### Scenario: The renditions of a page of the copy

- **WHEN** a client reads a page of the copy
- **THEN** the original, the display and the thumbnail rendition all load
- **AND** each is the rendition of the source page

#### Scenario: The copy writes no image

- **WHEN** the worker finishes a copy of a book with 12 pages
- **THEN** the copy holds no image file of its own
- **AND** the source book holds exactly the images it held before

#### Scenario: The cover of a copy

- **WHEN** a client lists the books of the `ocr` stage
- **THEN** the cover of an OCR copy loads
- **AND** it is the thumbnail of the first page of its source book

#### Scenario: The source book is deleted later

- **WHEN** a user rejects the source book of a complete OCR copy
- **THEN** the copy is destroyed with it, as every copy is
- **AND** no page of the copy remains pointing at a removed image

### Requirement: The system exchanges pages with the endpoint through object storage

The system SHALL upload the original rendition of every source page to a bucket
in object storage, under a prefix unique to the copy. The system SHALL then
start one job on the OCR endpoint and tell it that bucket and those prefixes.
The system SHALL also tell it which engines to run, taken from the profile of
the copy. The system SHALL NOT put page bytes in the request body. The uploaded
object names SHALL carry the page order. The prefix of an OCR copy SHALL differ
from the prefix of a dewarp copy of the same book.

#### Scenario: A book goes to OCR

- **WHEN** the worker starts a job for a copy of a book with 12 pages
- **THEN** 12 objects exist under the input prefix of that copy
- **AND** each object holds the original rendition bytes of its page
- **AND** the object names sort into page order

#### Scenario: The endpoint job starts

- **WHEN** every page is uploaded
- **THEN** the system starts one job on the OCR endpoint
- **AND** the request carries the bucket, the input prefix, and the output prefix
- **AND** the request carries the credentials the endpoint needs to reach them
- **AND** the request names the engines of the profile

#### Scenario: The extra profile asks for both engines

- **WHEN** the worker starts a job for a copy whose profile is `vl`
- **THEN** the request names both engines

#### Scenario: Two actions use one book

- **WHEN** a book is dewarped and another book is read by OCR
- **THEN** their prefixes differ
- **AND** neither reads or overwrites the objects of the other

### Requirement: A page that cannot be read fails the whole copy

The system SHALL fail the copy when an engine of the profile cannot read every
page. The system SHALL NOT store a copy with fewer results than its profile
requires. The stored reason SHALL name the pages that failed.

The endpoint answers one result per engine per page, so the system SHALL count
the results of each engine on its own. A copy whose profile names two engines
SHALL be complete only when both engines returned every page.

#### Scenario: One page fails at the endpoint

- **WHEN** the endpoint reads 11 of 12 pages and reports the twelfth as failed
- **THEN** the copy state becomes `failed`
- **AND** the reason names the failed page

#### Scenario: One engine of two misses a page

- **WHEN** a copy has the profile `vl`, and one engine returns 12 pages while the
  other returns 11
- **THEN** the copy state becomes `failed`
- **AND** the reason names the engine and the missing page

#### Scenario: Both engines return every page

- **WHEN** a copy has the profile `vl`, and both engines return all 12 pages
- **THEN** the copy completes
- **AND** each page holds two results

#### Scenario: An output object is missing

- **WHEN** the endpoint reports success but an output object is absent
- **THEN** the copy state becomes `failed`
- **AND** the reason names the missing page

### Requirement: The OCR endpoint runs the image as a serverless worker

The OCR endpoint SHALL accept a job that names a bucket, an input prefix, an
output prefix, and the engines to run. It SHALL refuse a job that names no
engine, and SHALL refuse a job that names an engine it does not have.

It SHALL read every input object, parse it with every engine the job names, and
write the output of each engine under the output prefix. The output names SHALL
derive from the input name and SHALL differ per engine, so the caller can pair a
result with both its page and its engine. The result SHALL report one entry per
engine per page, and SHALL report each page that failed. The worker SHALL load
the models of an engine once and SHALL reuse them for later jobs. The image
SHALL keep its command line tool.

#### Scenario: A job reads a prefix

- **WHEN** the endpoint runs a job for a prefix that holds 12 images, naming one
  engine
- **THEN** the result holds 12 entries
- **AND** every entry names the objects that engine wrote
- **AND** each output name derives from the name of its input

#### Scenario: A job names two engines

- **WHEN** the endpoint runs a job for a prefix that holds 12 images, naming two
  engines
- **THEN** the result holds 24 entries
- **AND** the objects of one engine do not overwrite the objects of the other

#### Scenario: A job names no engine

- **WHEN** a job carries an empty engine list
- **THEN** the endpoint refuses the job and names the reason

#### Scenario: One input cannot be read

- **WHEN** one input object holds bytes that are not a readable image
- **THEN** the result names that object as failed
- **AND** the result still lists the pages that succeeded

#### Scenario: A second job arrives

- **WHEN** the same worker process runs a second job
- **THEN** it does not load the models again

#### Scenario: The image runs from the command line

- **WHEN** an operator runs the image against a local directory of images
- **THEN** it writes the files of every named engine to the output directory

## REMOVED Requirements

### Requirement: Every page of the copy carries its markdown

**Reason**: The endpoint now runs two engines, and a single markdown file per
page cannot hold two results. A file also cannot hold the layout blocks, the
line boxes and the confidences the engines report, which the markdown drops. The
results move to one stored record per engine per page, which the requirement
"Every page of a copy holds one result per engine" defines.

**Migration**: No client reads these files. An existing complete copy keeps its
markdown files on disk and holds no stored results, until the book is rejected
or the OCR action runs again. Running OCR again on that book produces the stored
results.
