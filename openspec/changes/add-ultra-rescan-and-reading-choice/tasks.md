## 1. Prove the page-wide block before building on it

- [x] 1.1 Write a throwaway Go test that builds one page-wide block from a stored
  Claude transcription and merges it with the stored `v6` result of the same
  page, and verify it runs for all 11 pages that already hold both
- [x] 1.2 Read off, per page, the merge counts: lines from text, lines from
  boxes, lines the merge placed itself, and boxes discarded — and verify that
  discarded boxes stay near zero, which is what says the disaster path did not
  fire
- [x] 1.3 Verify on a two column page and on a page with footnotes that the
  merged lines carry `v6` boxes rather than boxes the merge placed
- [x] 1.4 If discarded boxes are high, switch to one block per paragraph of the
  transcription and repeat 1.2; record which shape the measurement chose, and
  stop to agree the fallback if neither holds

## 2. The stored choice

- [x] 2.1 Add migration `0010_page_reading_choice.sql` adding
  `book_page.ocr_id` (`REFERENCES page_ocr(id) ON DELETE SET NULL`) and
  `book_page.ocr_chosen_by_reader`, and verify the dev schema loads and every
  existing page reads unchanged
- [x] 2.2 Extend `ocr.RenderModels` to `[claude_merged, merged, v6]` and add
  `ocr.RankOf(model)`, and verify a unit test that the rank orders the three and
  answers false for a model that cannot be rendered
- [x] 2.3 Change `books.PageOCRIn` and `books.BookOCRIn` to answer the reading
  `ocr_id` names before the preference order, and verify store tests that a page
  naming its `v6` result answers it while a page naming none still answers the
  merged result
- [x] 2.4 Clear `ocr_id` on store: after a result is written, drop the name when the
  new reading outranks the one the page names — in the same transaction — and verify
  store tests that a better reading clears the choice, that a worse one and the same
  one again leave it, and that a page naming nothing is left alone
- [x] 2.6 Drop `ocr_chosen_by_reader` in migration `0011`: a page names a reading
  because a reader chose it and for no other reason, so the flag held the same fact
  as `ocr_id IS NOT NULL`
- [x] 2.5 Verify a store test that deleting the named result leaves `ocr_id` null
  and the page answering by preference again

## 3. The disagreement rule

- [x] 3.1 Add `internal/health` with a function answering, for one book, which
  pages disagree: markup stripped first, then wording share, length share,
  repeated 15 character runs, and the foreign script test
- [x] 3.2 Verify unit tests for each fault on its own: a truncated reading, a
  reading that lost the wording, a looping reading, and a clean reading
- [x] 3.3 Verify a unit test that a page whose `v6` reading holds under 400
  letters is judged by repetition and script alone
- [x] 3.4 Implement the script test page-wise: foreign when the reading holds a
  script the `v6` reading of that same page does not, reading no other page
- [x] 3.5 Verify unit tests that a page whose `v6` reading lacks the Cyrillic
  disagrees, that a page whose `v6` reading holds it does not, and that a single
  substituted letter still disagrees
- [x] 3.6 Verify the rule against the live data: it flags every page of books 17,
  18 and 24 that was confirmed by hand, and every other page it flags is a real
  fault read off by eye

- [x] 3.7 Judge the full text a reading engine read and not its markdown, which
  leaves the footnote and page number blocks out, and verify the pages carrying a
  footnote are no longer called truncated
- [x] 3.8 Answer why a page disagrees, in words short enough to read in a menu and
  with the number that decided each fault, and verify a plate names no share of the
  wording
- [x] 3.9 Require 10 characters or more of scripts the page is not written in,
  counting every such script together, and verify one substituted letter no longer
  disagrees while a passage does
## 4. The Ultra rescan worker

- [x] 4.1 Add `internal/anthropic` with a `Client` over `net/http` carrying a
  name, base URL and token, and verify tests against an `httptest` server for a
  success, an API error, and a refusal
- [x] 4.2 Add required `config.Claude` reading `CLAUDE_API_KEY` through
  `e.require`, and verify the server refuses to start without it
- [x] 4.3 Add the `claude` profile to `ocr`: rescan engines, rejected by the OCR
  action, and verify unit tests for both
- [x] 4.4 Add `ocr.MergeOfText(boxData, text, width, height)` building the block
  shape task 1 chose, and verify it stores a `claude_merged` result whose lines
  carry `v6` boxes
- [x] 4.5 Add the `claude` branch to the rescan worker: no bucket, no endpoint,
  one API call per page, storing `claude` and `claude_merged`, and verify worker
  tests that no object is uploaded and no endpoint job starts
- [x] 4.6 Verify a worker test that a page the API could not read keeps every
  result it held, and that the other named pages still gain theirs
- [x] 4.7 Verify a test that the key reaches the Anthropic request alone: it
  appears in no stored result, no log line and no uploaded object
- [x] 4.8 Verify the existing rescan tests still pass: the book returns to
  `complete`, the page count survives, and the book is never failed

## 5. The API

- [x] 5.1 Report the readings of a page in the page lines answer — profile name,
  model, and which answers — omitting those a renderer cannot draw, and verify a
  handler test for a page holding one, two and three readings
- [x] 5.2 Add the endpoint that sets the reading of a page, refusing a reading
  the page does not hold and one that cannot be drawn, and verify handler tests
  for success and both refusals
- [x] 5.3 Report per page whether it disagrees, and the count for the book, in
  the book lines answer, and verify a handler test that a book of one reading per
  page reports zero
- [x] 5.4 Verify a handler test that the one page answer and the whole book answer
  name the same model for the same page, including a page that names a reading

## 6. The interface

- [x] 6.1 Add the "Ultra" choice to `RescanDialog.vue` — subtitle "Claude Opus 5"
  and the frontier-visual-model note — and verify the OCR dialog still offers two
  choices
- [x] 6.17 Preselect the model after the one the book was read with, so a book read
  with PaddleOCR-VL opens the rescan dialog on Ultra
- [x] 6.2 Send the `claude` profile from the reader, and add the readings and
  set-reading call to `api/client.ts`, and verify `npm run typecheck` passes
- [x] 6.3 Show "16 pages disagree" in the same block as the breadcrumbs in
  `ReaderView.vue`, right of the last crumb, singular for one, absent for zero,
  and verify the count and its place against book 24
- [x] 6.4 Draw the warning border inside the read half of every disagreeing page,
  turn the rescan mark and its control the colour of success, and verify a page
  that is both marked and disagreeing draws both borders
- [x] 6.5 Add the readings group to the top of `PageMenu.vue` with a success
  coloured mark on the one that answers, and a divider under the group, and
  verify a page of an unread book opens the menu with no such group
- [x] 6.6 Choosing another reading sets it, redraws the page from it, and moves
  the mark, and verify the choice survives reopening the reader
- [x] 6.7 Mark the disagreeing pages of the grid in `BookView.vue` with a border
  drawn inside the thumbnail, name the count beside its breadcrumbs, and verify no
  cell changes size
- [x] 6.8 Measure the row the spreads stand in rather than deriving it from the
  window, so no border is clipped by a scrollbar the first render did not know
  about, and verify the halves fill the row exactly
- [x] 6.9 Name the rescan item "Do not rescan" on a marked page and take the mark
  off when it is chosen, and verify the control goes with the last mark
- [x] 6.10 Report per line whether the merge took its text from the line engine,
  draw those lines dark brown in the reader, and verify a line reading draws
  nothing brown
- [x] 6.11 Add the strip down the right edge of the reader holding one mark per
  page that carries a reason, and verify a mark brings its page into view and a book
  with nothing to mark holds no strip
- [x] 6.18 Border the read half of a page holding an Ultra reading in the colour of
  information, band its minimap mark with the same colour, and verify a page
  carrying three reasons shows three rings and three bands with none hidden
- [x] 6.12 Say why the reading of a page is doubted, between the readings and the
  rescan of the read half menu, padded to the labels around it, and verify it
  answers no selection
- [x] 6.13 Move the readings, the rescan and the doubt to the read half menu, out
  of the page half, order the readings Normal, Extra, Ultra, and verify the page
  half holds none of them
- [x] 6.14 Place a minimap mark where the thumb of the scrollbar stands when its
  page is in the window, so the strip agrees with the scrollbar at both ends of a
  book and not only in the middle
- [x] 6.15 Keep the doubt of a page when a reader looks through its readings, and
  verify it survives Extra to Normal and back
- [x] 6.16 Stand a line the merge filled from the line engine on a grey ground and
  leave its text the colour of every other line, and verify the two grounds differ

## 7. End to end

- [x] 7.1 Add an e2e test that the page menu of a read page lists its readings and
  marks one
- [x] 7.2 Add an e2e test that choosing another reading changes the drawn text and
  survives a reload
- [x] 7.3 Add an e2e test that a book with a disagreeing page shows the count and
  the border, using a page seeded to disagree, and verify the border shows on every
  edge at 1280, 1920 and 4000 pixels wide, that both borders show on a page marked
  and disagreeing, and that the grid marks the same pages
- [x] 7.4 Add an e2e test that an Ultra rescan of a page, against a fake
  Anthropic API, stores the reading and adds Ultra to the page menu marked as the
  one that answers
- [x] 7.5 Run the whole Go suite on a scratch database and verify it passes
- [x] 7.6 Run `./run-e2e.sh` for the affected specs and verify they pass
- [ ] 7.7 Read one real page with the Ultra rescan against the live API and verify
  by eye in the reader that the merged text draws on the `v6` boxes
  — BLOCKED on this machine: no page of any read book holds its original image
  here. The development database came from a production dump and the page files
  under `/data` never came with it, so all 361 read pages fail at the file and
  reach no API. Attempted on book 24 page 18: the worker retried three times,
  abandoned the rescan, returned the book to `complete` and changed no result,
  which is the failure path the spec asks for. Run it on a machine that holds the
  images, or after `./pull-prod-db.sh` brings the files too.
- [x] 7.8 Run `openspec validate --strict add-ultra-rescan-and-reading-choice`
  and verify it passes

Note: `go test ./...` truncates `book CASCADE` on the development database. Follow
the scratch-database recipe before running the backend suite.
