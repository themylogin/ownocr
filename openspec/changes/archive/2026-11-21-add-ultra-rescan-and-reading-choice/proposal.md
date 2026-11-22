## Why

A page can now be read twice, and the second reading is not always the better
one. PaddleOCR-VL sometimes substitutes another script for the Greek, loops a
phrase to the foot of the page, or abandons the page after two sentences. The
line engine cannot do any of those, so the two readings of a bad page disagree
loudly — but nothing in the system says so. A reader finds such a page by eye,
if at all.

Measured on the two Greek books the library holds: 17 of 287 read pages carry
one of those faults. Two are the script collapse that is easy to see. The other
fifteen are quieter and worse: one page keeps 5% of the words the line engine
read, another 8%. Their text reads fluently, so nothing about them looks wrong.

A rescan can cure some of them and cannot cure the rest. Eleven of the flagged
pages were read again at eight sizes each: four improve at some size, four are
unmoved at every size, and each of the four that improve wants a different size.
The same eleven pages read by Claude Opus 5 keep 66% to 97% of the line engine's
wording, beating the best rescan on every page, including the four that no size
could reach.

The system therefore needs three things it does not have: a rule that says which
pages disagree, a reading from a model that can read them, and a way to say which
reading of a page answers.

## What Changes

- The system reports, per page, whether the reading it answers disagrees with the
  line reading of that page. A page disagrees when it loses more than 40% of the
  line engine's wording, when it holds less than 60% of its text, when it
  degenerates into a repeated phrase, or when it holds a script that the line
  reading of that same page does not hold.
- The reader names the number of disagreeing pages beside the last breadcrumb, in
  the same block, and draws a warning border round the read half of every one of
  them. The page grid marks them too, so a reader finds them without opening each
  page.
- The mark of a rescan turns the colour of success, where the disagreement border
  keeps the colour of a warning. One says a user chose the page and the other that
  the system doubts its reading, so a page carrying both reasons draws both
  borders.
- The rescan dialog offers a third model: "Ultra", subtitled "Claude Opus 5",
  described as the frontier visual model for pages even PaddleOCR-VL cannot read.
  The OCR action of a whole book offers it not.
- A rescan with the Ultra profile runs in the Go worker alone. It sends the page
  image to the Anthropic API and asks for the text of the page. No serverless
  endpoint and no object storage stand in the way.
- The system merges the Ultra text with the line boxes of the page, as it merges
  the PaddleOCR-VL text, and the reader draws it like any other reading.
- A page holds every reading that ever read it, and one of them answers. The page
  menu lists them by the profile that made them, marks the one that answers, and
  changes it when the reader picks another.
- A reader's choice yields to a better reading. When a reading that outranks the
  chosen one arrives, the page stops naming anything and answers the best it holds,
  as though nothing had been chosen.
- **BREAKING**: `book_page` gains an `ocr_id` column naming the reading a reader
  chose. A page that names none answers by the order the system prefers, so every
  page read before this change reads as it did.

## Capabilities

### New Capabilities

- `page-reading-health`: the rule that decides whether the reading of a page
  disagrees with the line reading of that page, and how the system reports it.
- `page-reading-choice`: a page holds several readings and one of them answers.
  Which one a new reading takes over from, which one a reader can choose, and
  what a page that has never been chosen for answers.

### Modified Capabilities

- `book-ocr`: a third model reads a page, its text merges with the line boxes
  through a block that covers the page, and the API answers the reading the page
  names rather than the first of a fixed order.
- `page-rescan`: the rescan carries the `claude` profile, which runs in the Go
  worker against the Anthropic API instead of the OCR endpoint.
- `book-library`: the rescan dialog holds a third choice, the reader reports the
  disagreeing pages and marks them, and the page menu lists the readings of a
  page and switches between them.

## Impact

Database: one migration adds `book_page.ocr_id` referencing `page_ocr(id)`.

Backend:

- `internal/config`: `CLAUDE_API_KEY` joins the settings the server needs.
- `internal/ocr`: the `claude` profile and its engine names, an Anthropic client
  beside the RunPod one, a rescan worker path that calls it, and a merge that
  builds one page-wide block from plain text.
- `internal/books/store.go`: read and write `ocr_id`; select the reading a page
  names before the preference order.
- `internal/httpapi`: the page and book line answers honour `ocr_id`; the book
  answer reports the readings of every page and which disagree; one endpoint sets
  the reading of a page.
- `internal/health` (new): the disagreement rule.

Frontend: `RescanDialog.vue` (third choice), `ReaderView.vue` (count and border),
`PageMenu.vue` (reading list with a mark), `api/client.ts` (types and the set-
reading call).

Cost: the Ultra rescan bills the Anthropic API per page, about $0.06 at the page
sizes this library holds.
