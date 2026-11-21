## ADDED Requirements

### Requirement: The interim algorithm makes two pages from one

Until the real algorithm arrives, the split worker SHALL write two pages for
every page of the source book. The two pages SHALL show the same image as the
source page, and SHALL sit next to each other in the order of the copy.

This algorithm stands in for cutting a spread. It exercises the one-to-many path
end to end, so the page indexes, the page count and the delete chain are all
proved before the real algorithm lands. Replacing it changes no requirement other
than this one.

#### Scenario: The interim algorithm doubles the pages

- **WHEN** the worker splits a source book of 60 pages
- **THEN** the copy holds 120 pages
- **AND** the card of the copy reports 120 pages

#### Scenario: The two pages sit together

- **WHEN** the worker writes the two pages made from one source page
- **THEN** the two pages follow one another in the order of the copy
- **AND** both show the image of that source page

## MODIFIED Requirements

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
