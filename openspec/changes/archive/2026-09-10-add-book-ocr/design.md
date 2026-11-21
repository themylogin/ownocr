## Context

See proposal.md - Why.

The dewarp path already does everything OCR needs, except the last step. It
uploads pages to a bucket, starts one RunPod job, polls it, downloads the
result, and builds the pages of a copy. `internal/dewarp` holds all of it: the
object keys in `args.go`, the RunPod HTTP client in `runpod.go`, the job in
`worker.go`.

The endpoint image exists too. `runpod/paddleocr-vl/handler.py` takes the same
job input the dewarp handler takes: `bucket`, `endpoint_url`, `region`, the two
credentials, `input_prefix`, and `output_prefix`. It ignores `jpeg_quality`. It
answers with `{"pages": [...], "failed": [...]}`, where a page carries
`input_key`, `output_key`, `chars`, and `seconds`. It writes one markdown object
per input, named after the input file with a `.md` suffix.

Two facts shape the design:

1. OCR returns text, not images. The copy therefore cannot build its page
   images out of the endpoint result, as dewarp does.
2. The goal now is to debug the real endpoint. The markdown must land somewhere
   an operator can read with `cat`, without a database column or a new view.

## Goals / Non-Goals

**Goals:**

- One OCR action that runs the real RunPod endpoint end to end.
- One RunPod job client, used by both workers.
- Markdown files on disk, in an obvious place, one per page.
- Logs detailed enough to debug a failing endpoint job: the job id, every status
  change, a heartbeat while it runs, and the reason of a failure.

**Non-Goals:**

- Markdown in the database, a text view, a search, or an export.
- Streaming per page progress out of the endpoint. The handler already sends
  `progress_update`, but reading it needs another poller field. The five
  existing phases are enough.
- Any change to `runpod/paddleocr-vl`.

## Decisions

### The copy carries the images of its source pages

The OCR worker copies every source page into the copy: a new storage key, and
the three renditions written from the original rendition of the source page,
with `ingest.WriteRenditions`. The copy therefore reads, browses, and rejects
like every other book, and the markdown of a page has a page to belong to.

Alternative, rejected: a copy with no page rows, and markdown alone under
`/data/ocr/<book key>/`. It costs no disk, but the Done tab then shows a book
with no cover and an empty grid, and the reject path would need a second place
to clean. The pipeline already rewrites every page at every stage, so one more
copy is consistent with what is there.

Alternative, rejected: page rows that point at the files of the source book. A
reject of the source book would then destroy the images of the copy.

### The markdown of a page sits beside its renditions

`<data>/pages/<book key>/<page key>.md`, written by a new
`filestore.PageMarkdownPath`. The rendition names are `<page key>.orig.jpg`,
`<page key>.1280.jpg` and `<page key>.480.jpg`, so `.md` cannot collide. The
page directory is already removed as a whole on a retry and on a reject, so the
markdown needs no separate cleanup.

The frontend serves `/files/<book key>/<name>` from the same directory. The
markdown is therefore reachable in a browser as soon as someone wants it, with
no server change.

### One RunPod client for both workers

Move `internal/dewarp/runpod.go` to `internal/runpod`. The client keeps `Run`
and `Wait`, and gains a name (`"dewarp"`, `"OCR"`) that it puts in its error
messages, which today are hard-coded to "the dewarp endpoint". `Run` takes an
`any` input and `Wait` returns the raw output, so each worker keeps its own
input and output types: the two endpoints share the transport, not the contract.

Alternative, rejected: a second copy of the poller in `internal/ocr`. It is 200
lines of retry and heartbeat logic that must not drift.

### The workers stay separate

`internal/ocr` gets its own worker, with its own upload, run, download, and
store steps. The two workers look alike, but they differ at every step that
matters: what they send (`jpeg_quality` or nothing), what they check, what they
download, and what they write. A shared "endpoint action" worker would need a
callback at each of those points, which is longer than the duplication it
removes.

### Object keys and the queue

- Prefix: `ocr/<copy key>/`, with `in/` and `out/` under it, so an OCR copy
  never touches the objects of a dewarp copy.
- Input name: `0001.jpg`, the padded page index, as dewarp does. The endpoint
  writes `0001.md` beside it, under the output prefix.
- Queue: `ocr`, `MaxWorkers: 1`, `MaxAttempts: 3`. One book pushes gigabytes to
  object storage, as dewarp does.
- Pairing: the result of the endpoint pairs by `input_key`, never by order.
  A missing pair fails the copy.

### The OCR job timeout

`workTimeout` is 6 hours, as dewarp uses. PaddleOCR-VL parses a page in seconds,
so a 500 page book fits with room to spare. The RunPod queue wait counts against
the same timeout, which is the honest way to bound the job.

### The stub endpoint answers both actions

`tools/fakerunpod` decides from the input prefix: an `ocr/` prefix makes it
write one markdown file per input object, holding the name of that object. A
`dewarp/` prefix keeps the half size image it writes today. The e2e suite then
covers the OCR path with no GPU, as it covers dewarp.

## Risks / Trade-offs

- **The copy doubles the images of a book on disk** → It is the price of a copy
  that browses like every other book. A book that reaches `done` is at the end
  of the pipeline, so the operator can reject the stages before it.
- **The endpoint contract is only proven against the stub** → The stub cannot
  prove that the real handler answers the same shape. The first real run is the
  test. The worker logs the job id, every status change, and the raw reason of a
  failure, so a mismatch is readable in the log instead of silent.
- **A slow endpoint holds the queue** → The `ocr` queue runs one job, so a long
  book blocks the next one. Same trade-off as dewarp, for the same reason.
- **`PADDLEOCR_VL_API_URL` is required to start** → An operator who upgrades
  without setting it gets a server that refuses to start. The message names the
  variable. The `.env` file in this project holds the name
  `PADDLEOCR_VL_AP_URL`, which no longer matches: it must be renamed by hand.

## Migration Plan

1. Rename `PADDLEOCR_VL_AP_URL` to `PADDLEOCR_VL_API_URL` in the local `.env`
   file, before the new backend starts.
2. Deploy. No database migration runs: the copy of an OCR action uses the
   columns every copy uses.
3. Rollback: an older backend refuses the `ocr` action, and leaves an OCR copy
   in state `pending` with no worker to take it. Reject that copy by hand.
