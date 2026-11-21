## ADDED Requirements

### Requirement: A rescan reports its phase

The rescan worker SHALL report the phase of each part of its job, in this order:
"Uploading" while it sends the named pages to object storage, "Waiting for AI"
while the endpoint job waits in the endpoint queue, "Processing with AI" while
the endpoint job runs, "Downloading" while it fetches the results, and
"Converting" while it stores them. The rescan worker SHALL use the same phase
vocabulary as every other worker, with no phase of its own.

A rescan returns its book to state `complete` whatever the result, so the book
SHALL carry no phase when the rescan ends. This holds for a rescan that read no
page at all.

#### Scenario: The worker sends the pages

- **WHEN** the rescan worker uploads the named pages to object storage
- **THEN** the phase of the book is "Uploading"

#### Scenario: The endpoint job waits

- **WHEN** the endpoint reports that the rescan job is in its queue
- **THEN** the phase of the book is "Waiting for AI"

#### Scenario: The endpoint job runs

- **WHEN** the endpoint reports that the rescan job is running
- **THEN** the phase of the book is "Processing with AI"

#### Scenario: The worker fetches the results

- **WHEN** the rescan worker downloads the results from object storage
- **THEN** the phase of the book is "Downloading"

#### Scenario: The worker stores the results

- **WHEN** the rescan worker stores the results of the pages it read
- **THEN** the phase of the book is "Converting"

#### Scenario: A rescan ends

- **WHEN** a rescan returns its book to state `complete`
- **THEN** the book carries no phase

#### Scenario: A rescan reads no page

- **WHEN** every attempt of a rescan fails
- **THEN** the book carries no phase
- **AND** the book carries no failure reason
