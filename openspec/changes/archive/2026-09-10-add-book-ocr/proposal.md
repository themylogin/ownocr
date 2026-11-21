## Why

The pipeline stops at `split`. No action reads the text off a page, so the
`done` stage stays empty. The image `runpod/paddleocr-vl` already parses a
prefix of pages into markdown, but nothing calls it. This change connects it,
so an operator can run the OCR endpoint on a real book and debug it.

## What Changes

- Add the OCR action. It leads to the `done` stage, which no action reached
  before.
- Add the `ocr` queue and its worker. The worker uploads the pages of the source
  book to the bucket, starts one job on the PaddleOCR-VL endpoint, waits for it,
  and downloads one markdown file per page.
- Store the markdown of a page next to the renditions of that page, as
  `<pageKey>.md` in the page directory of the copy.
- Copy the pages of the source book into the OCR copy, with their own storage
  keys and their own three renditions. The copy reads and browses like every
  other book.
- Read `PADDLEOCR_VL_API_URL` and `PADDLEOCR_VL_API_TOKEN` from the environment.
  Both are required, like the dewarp settings. The name in the local `.env` file
  today is `PADDLEOCR_VL_AP_URL`. The operator renames it.
- Pass the endpoint settings through `docker-compose.yml`,
  `docker-compose.dev.yml`, the two e2e compose files, and `.env.example`.
- Extract the RunPod job client out of the dewarp package, so OCR and dewarp
  share one poller instead of two copies of it.

Non-goals of this change: no markdown in the database, no text view in the
interface, and no editing of a parsed page. The markdown files on disk are the
result an operator inspects while debugging the endpoint.

## Capabilities

### New Capabilities
- `book-ocr`: reads the text off every page of a book on the PaddleOCR-VL
  endpoint, and stores one markdown file per page of the copy.

### Modified Capabilities
- `book-stages`: the action list gains `ocr`, which leads to `done`. The `done`
  stage is no longer unreachable, and the stages before it offer one more
  action. OCR declares that it maps one page to one page.
- `book-progress`: the OCR worker reports its phase, from the same vocabulary
  the other workers use.

## Impact

- `backend/internal/config`: two new required settings.
- `backend/internal/books/stages.go`: the `Actions` list and the `ActionOCR`
  constant.
- `backend/internal/httpapi/actions.go`: the OCR branch of `queueAction`.
- `backend/internal/ocr` (new): args, worker, and the endpoint contract.
- `backend/internal/runpod` (new): the job client that dewarp uses today.
- `backend/internal/dewarp`: keeps its own input and output types, and calls the
  shared client.
- `backend/cmd/server/main.go`: registers the OCR worker.
- `runpod/paddleocr-vl`: no code change. The handler already accepts the job
  input this worker sends.
- `backend/tools/fakerunpod` and `e2e`: the stub endpoint answers OCR jobs, so
  the OCR path runs in the tests without a GPU.
- Compose files, `.env.example`, and `README.md`.
- The frontend needs no change. It takes the stages and their actions from the
  server.
