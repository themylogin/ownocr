## ADDED Requirements

### Requirement: OCR reports its phase

The OCR worker SHALL report the phase of each part of its job, in this order:
"Uploading" while it sends pages to object storage, "Waiting for AI" while the
endpoint job waits in the endpoint queue, "Processing with AI" while the
endpoint job runs, "Downloading" while it fetches the markdown, and "Converting"
while it builds the pages of the copy. The OCR worker SHALL use the same phase
vocabulary as every other worker, with no phase of its own.

#### Scenario: The worker sends the pages

- **WHEN** the OCR worker uploads the source pages to object storage
- **THEN** the phase of the copy is "Uploading"

#### Scenario: The endpoint job waits

- **WHEN** the endpoint reports that the OCR job is in its queue
- **THEN** the phase of the copy is "Waiting for AI"

#### Scenario: The endpoint job runs

- **WHEN** the endpoint reports that the OCR job is running
- **THEN** the phase of the copy is "Processing with AI"

#### Scenario: The worker fetches the results

- **WHEN** the OCR worker downloads the markdown from object storage
- **THEN** the phase of the copy is "Downloading"

#### Scenario: The worker builds the pages

- **WHEN** the OCR worker stores the pages of the copy after the download
- **THEN** the phase of the copy is "Converting"
