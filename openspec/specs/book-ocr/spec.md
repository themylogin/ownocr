## Purpose

Reads the text off every page of a book on a remote endpoint, with the model the
user picks. The result is a second book in the `ocr` stage that keeps a
reference to the book it came from, and one stored result per engine per page.

## Requirements

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

### Requirement: The system needs the endpoint settings to start

The system SHALL read the endpoint address and the endpoint token from the
environment, as `PADDLEOCR_VL_API_URL` and `PADDLEOCR_VL_API_TOKEN`. The system
SHALL refuse to start when either is missing, and SHALL name the missing
setting.

#### Scenario: A setting is missing

- **WHEN** an operator starts the server without `PADDLEOCR_VL_API_TOKEN`
- **THEN** the server stops at once
- **AND** the message names `PADDLEOCR_VL_API_TOKEN`

#### Scenario: Both settings are present

- **WHEN** both settings hold a value
- **THEN** the server starts
- **AND** the OCR worker sends its jobs to that address

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

### Requirement: The system monitors the endpoint job to its end

The system SHALL poll the OCR endpoint until the job reaches a final result. A
job that ends in any way other than success SHALL fail the copy and SHALL store
the reason the endpoint reports.

#### Scenario: The job is still running

- **WHEN** the endpoint reports that the job is queued or in progress
- **THEN** the system waits and asks again
- **AND** the copy stays in state `processing`

#### Scenario: The job succeeds

- **WHEN** the endpoint reports success
- **THEN** the system downloads the output objects

#### Scenario: The job fails at the endpoint

- **WHEN** the endpoint reports a failure, a cancellation, or a timeout
- **THEN** the OCR job fails
- **AND** the stored reason repeats what the endpoint reported

#### Scenario: The endpoint cannot be reached

- **WHEN** the endpoint does not answer
- **THEN** the OCR job fails
- **AND** the stored reason names the endpoint

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

### Requirement: The OCR action carries a model profile

The system SHALL define two OCR profiles: `v6` and `vl`. For the OCR action the
profile `v6` SHALL run the PP-OCRv6 engine alone, and the profile `vl` SHALL run
the PaddleOCR-VL engine and the PP-OCRv6 engine together.

A rescan names a profile from the same two, and a user picks between them in the
same words. The engines a rescan runs for a profile SHALL be those the
`page-rescan` capability names, which are not those the OCR action runs. The OCR
action reads a page the system holds no result for; a rescan reads a page the
system has read already.

The system SHALL require a profile on every OCR action, and SHALL refuse an OCR
action that names no profile or an unknown one. The system SHALL record the
profile on the copy the action creates.

The profile SHALL say which engine holds the text of record for the reading the
action performed. For `v6` the PP-OCRv6 result answers. For `vl` the
PaddleOCR-VL result answers, and the PP-OCRv6 result carries the line boxes and
the confidence PaddleOCR-VL does not report.

The profile SHALL describe the reading the action performed and SHALL NOT
describe a page a later rescan read again. The system SHALL NOT change the
profile of a copy when a rescan reads a page of it, whatever profile that rescan
names. A client that needs to know which engines read a page SHALL read the
results of that page, and SHALL NOT derive it from the profile.

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

#### Scenario: A rescan keeps the profile

- **WHEN** a rescan with the profile `vl` reads pages 12 and 13 of a copy whose
  profile is `v6`
- **THEN** the copy still records the profile `v6`
- **AND** pages 12 and 13 hold a PaddleOCR-VL result
- **AND** the profile of the copy says nothing about pages 12 and 13

#### Scenario: A normal rescan keeps the profile

- **WHEN** a rescan with the profile `v6` reads page 12 of a copy whose profile
  is `vl`
- **THEN** the copy still records the profile `vl`
- **AND** page 12 holds a PP-OCRv6 result alone

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

A rescan of a page MAY replace the result of an engine that read that page
before, and MAY remove one. The `page-rescan` capability governs both. A page
therefore holds the results of the last reading of that page, not of the reading
that built the copy.

A page MAY hold one further result whose model names no engine. Such a result is
derived from the engine results of that page. It SHALL NOT change them, and the
rules above SHALL hold for the engine results whatever else a page holds.

#### Scenario: A page read with the normal profile

- **WHEN** the worker stores page 3 of a copy whose profile is `v6`
- **THEN** page 3 holds one result, naming the PP-OCRv6 engine
- **AND** the result holds the recognised lines and the full engine result

#### Scenario: A page read with the extra profile

- **WHEN** the worker stores page 3 of a copy whose profile is `vl`
- **THEN** page 3 holds one result per engine, one naming PaddleOCR-VL and one
  naming PP-OCRv6
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

#### Scenario: A rescan replaces the result of an engine

- **WHEN** a rescan reads page 3 with an engine that has read page 3 before
- **THEN** page 3 holds one result of that engine
- **AND** it is the result of the rescan

#### Scenario: A page is deleted

- **WHEN** a page of a copy is removed
- **THEN** every result of that page is removed with it

### Requirement: A page read by two engines holds a merged result

The system SHALL store a third result for every page that both the PP-OCRv6
engine and the PaddleOCR-VL engine have read. Its model SHALL be `merged`. The
system SHALL write it once it holds the results of both engines for that page.

The two engine results MAY reach the system in one job or in two. A copy whose
profile names two engines gains both in the job that builds it. A page of a copy
whose profile names one engine gains the second in a later rescan. The merged
result is the same either way.

The merged result SHALL hold the line boxes of the PP-OCRv6 result filled with the
text of the PaddleOCR-VL result. Its full result SHALL hold, for every line, the
box, the text and the name of the engine that text came from. It SHALL also hold
what the merge dropped and why, and the counts the merge reports. Its text SHALL be
the text of the merged lines, one line per line, in page order.

The merged result SHALL hold one line for every line of the PP-OCRv6 result,
except where the merge adds or removes one. The merge adds a line for a printed
row PP-OCRv6 missed, adds one for ink PP-OCRv6 left uncovered between two boxes of
one printed row, and removes the lines of a region whose boxes cannot say where
the printed rows are. The `page-text-merge` capability governs all three. The
counts of the merged result SHALL report how many lines the merge placed itself
and how many boxes it discarded.

A page that one engine alone has read SHALL hold no merged result. One engine
leaves nothing to join. A copy whose profile names one engine therefore holds no
merged result when its job ends.

A page that loses one of its two engine results SHALL lose its merged result in
the same step. The merged result derives from both, so it outlives neither.

The merged result SHALL be derived. The system SHALL NOT change either engine
result to write it, and SHALL be able to write it again from the two engine
results alone. The system MAY read a stored engine result to write it, rather
than asking the endpoint for that engine again.

#### Scenario: A page read with the extra profile

- **WHEN** the worker stores page 3 of a copy whose profile is `vl`
- **THEN** page 3 holds three results, named `v6`, `vl` and `merged`
- **AND** every line of the merged result that came from a `v6` line carries the
  box of that line

#### Scenario: A page whose boxes the merge replaced

- **WHEN** the worker stores a page whose top region PP-OCRv6 read as 14 sloping
  boxes across 8 printed rows
- **THEN** the merged result holds 8 lines for that region, none of them a `v6`
  box
- **AND** its counts report 14 discarded boxes
- **AND** the `v6` result still holds the 14 boxes PP-OCRv6 wrote

#### Scenario: A page whose rows hold ink no box covered

- **WHEN** the worker stores a page where two printed rows each hold ink between
  two boxes that neither box covers
- **THEN** the merged result holds a line for each of those two stretches
- **AND** its counts report 2 lines the merge placed
- **AND** the `v6` result still holds only the boxes PP-OCRv6 wrote

#### Scenario: A page read with the normal profile

- **WHEN** the worker stores page 3 of a copy whose profile is `v6`
- **THEN** page 3 holds one result, named `v6`
- **AND** page 3 holds no merged result

#### Scenario: A page of a normal copy is read again by the visual engine

- **WHEN** a rescan with the profile `vl` reads page 3 of a copy whose profile is
  `v6`
- **THEN** page 3 holds three results, named `v6`, `vl` and `merged`
- **AND** every line of the merged result that came from a `v6` line carries the
  box of that line
- **AND** page 4 of that copy, which the rescan did not name, still holds one
  result and no merged result

#### Scenario: A page loses its visual reading

- **WHEN** a rescan with the profile `v6` reads page 3 of a copy, and page 3 held
  a `v6`, a `vl` and a `merged` result
- **THEN** page 3 holds one result, named `v6`
- **AND** page 3 holds no merged result

#### Scenario: The engine results are untouched

- **WHEN** the worker writes the merged result of a page
- **THEN** the `v6` result holds the text and the result PP-OCRv6 wrote
- **AND** the `vl` result holds the text and the result PaddleOCR-VL wrote

#### Scenario: A retry replaces the merged result

- **WHEN** a retried job stores every page of a copy whose profile is `vl` again
- **THEN** each page holds exactly one result named `merged`
- **AND** no merged result of the earlier attempt remains

#### Scenario: One engine read a page as empty

- **WHEN** PaddleOCR-VL returns no block for a page PP-OCRv6 read as 12 lines
- **THEN** the merged result of that page holds those 12 lines with their `v6` text
- **AND** the copy still completes
### Requirement: The API answers the read lines of one page

The system SHALL answer the read lines of one page of a book, on request by book
and page index. The answer SHALL name the model it comes from, SHALL report the
pixel size the boxes count in, and SHALL hold one entry per line. Every entry
SHALL hold the box of that line and the text of that line.

A box SHALL be four numbers: left, top, right, and bottom. They SHALL count in
the pixels of the stored page image, which are the width and the height the book
detail reports for that page.

The system SHALL answer the merged result of the page when the page holds one,
and the PP-OCRv6 result when it does not. It SHALL NOT answer the PaddleOCR-VL
result, which reports blocks of reflowed text and no lines. It SHALL NOT rewrite
the text of a line or reorder the lines. The order SHALL be the order the stored
result holds.

The answer SHALL report the trim of the page when the page holds one. A trim
SHALL name the edge it lies against and the column it ends at, counting as a
distance from that edge. The answer SHALL report no trim when the page holds
none.

The system SHALL drop from the answer every line whose box lies wholly inside the
trim. Those lines hold the ink of the page beside this one, which this page never
held. A client therefore needs no rule of its own, and a reader and an export
SHALL drop the same lines.

The answer for a whole book SHALL report the trim of every page and drop the same
lines, by the same rule. The reader asks for a whole book in one request, so the
two answers SHALL never differ about what a page says.

The system SHALL NOT drop a line that lies outside the trim, whatever its text.

A trim SHALL cover pixels and SHALL move none. The reported width and height
SHALL be the stored size of the page whether the page holds a trim or not, and
the box of a line the answer keeps SHALL be the box the stored result holds.

The cut side of a page SHALL come from the nearest page in the lineage of that
page that carries a cut side. A copy holds the pages of its source book, so a
page of an OCR copy carries no cut side of its own and takes the side of the page
it was read from. A page with no such ancestor SHALL get no trim.

The system SHALL refuse the request for a page that holds neither result, and
SHALL name the reason. It SHALL refuse the request for a book or a page that
does not exist.

#### Scenario: A page holds a merged result

- **WHEN** a client asks for the lines of a page of a copy with the profile `vl`
- **THEN** the answer names the model `merged`
- **AND** it holds one entry per merged line, with the box and the text of that
  line

#### Scenario: A page holds one engine result

- **WHEN** a client asks for the lines of a page of a copy with the profile `v6`
- **THEN** the answer names the model `v6`
- **AND** it holds one entry per PP-OCRv6 line, with the box and the text of that
  line

#### Scenario: The boxes count in the stored page size

- **WHEN** a client asks for the lines of a page 3000 pixels wide
- **THEN** the answer reports the width 3000 and the height of that page
- **AND** every box lies in that space

#### Scenario: The text reaches the client unchanged

- **WHEN** a stored line holds the text the engine wrote
- **THEN** the answer holds that text, character for character

#### Scenario: A page that carries a strip of the page beside it

- **WHEN** a client asks for the lines of a page 2083 pixels wide that was cut as
  the right half of a spread
- **AND** the stored result holds 53 lines, 18 of which are fragments of the page
  beside it
- **THEN** the answer reports a trim of 122 pixels against the left edge
- **AND** it holds the 35 lines that lie outside that trim
- **AND** it reports the width 2083

#### Scenario: The book answer trims as the page answer does

- **WHEN** a client asks for the lines of a whole book of two pages, one carrying
  a strip and one not
- **THEN** each page of the answer reports the trim the answer for that page
  alone reports
- **AND** each holds the lines that answer holds

#### Scenario: A page that carries no strip

- **WHEN** a client asks for the lines of a page that holds no trim
- **THEN** the answer reports no trim
- **AND** it holds every line of the stored result

#### Scenario: The cut side comes from the page the copy was read from

- **WHEN** a client asks for the lines of a page of an OCR copy
- **AND** that page carries no cut side, but the split page it was read from
  carries the cut side `right`
- **THEN** the trim of the answer lies against the left edge

#### Scenario: A page that is not a half of a spread

- **WHEN** a client asks for the lines of a page whose lineage holds no cut side
- **THEN** the answer reports no trim
- **AND** it holds every line of the stored result

#### Scenario: A short line outside the trim survives

- **WHEN** a page holds a folio, a running head and a one word last line of a
  paragraph, and all three lie outside the trim
- **THEN** the answer holds all three

#### Scenario: A page was not read

- **WHEN** a client asks for the lines of a page that holds no result
- **THEN** the system refuses the request and names the reason

#### Scenario: The page does not exist

- **WHEN** a client asks for the lines of a page index the book does not hold
- **THEN** the system refuses the request

### Requirement: The API answers the read lines of a whole book

The system SHALL answer the read lines of every page of a book, on request by
book. The answer SHALL hold one entry per page, in page order. Every entry SHALL
name its page index and SHALL carry the same content the answer for one page
carries: the model the lines come from, the pixel size the boxes count in, and
one entry per line with the box and the text of that line.

Every rule the answer for one page follows SHALL hold for every page of this
answer. The model chosen, the order of the lines, and the text of a line SHALL
NOT differ between the two answers. A client that reads one page from this answer
SHALL get what the answer for that page alone would give.

A page that holds no result SHALL appear in the answer with no lines. The whole
book is one request, so one unread page SHALL NOT fail it. A running OCR copy
holds unread pages, and a reader of that copy SHALL still get the pages it has
read.

The system SHALL refuse the request for a book that does not exist.

#### Scenario: A client asks for a whole book

- **WHEN** a client asks for the lines of a book of 12 read pages
- **THEN** the answer holds 12 entries
- **AND** they carry the page indexes 1 through 12 in that order
- **AND** each holds the model, the page size, and the lines of that page

#### Scenario: The whole book agrees with one page

- **WHEN** a client asks for the lines of a book, and then for the lines of page
  3 of that book
- **THEN** the model of page 3 is the same in both answers
- **AND** the lines of page 3 are the same, in the same order, character for
  character

#### Scenario: A copy is still running

- **WHEN** a client asks for the lines of a copy that has read 5 of its 12 pages
- **THEN** the answer holds an entry for all 12 pages
- **AND** the 5 read pages hold their lines
- **AND** the 7 unread pages hold no lines
- **AND** the request succeeds

#### Scenario: A page holds a merged result

- **WHEN** a client asks for the lines of a book of a copy with the profile `vl`
- **THEN** every read page names the model `merged`

#### Scenario: The book does not exist

- **WHEN** a client asks for the lines of a book id that does not exist
- **THEN** the system refuses the request
