## Why

A book stops at the `dewarp` stage. The tab lists it and nothing moves it on.
Photographed pages stay curved, and curved pages read badly.

A GPU dewarps a page well and a CPU does not. `runpod/dewarp/` already holds a
CUDA image that runs the PaddleX orientation and UVDoc unwarping models, but it
is a command line tool with no way to call it from the server. This change makes
that image a RunPod serverless worker and gives the app a Dewarp action that
uses it.

## What Changes

- Add a second stage, `split`. The stage list becomes `dewarp`, then `split`.
- Add a Dewarp action to a book at the `dewarp` stage. The book view shows a
  button. A right click on a book card opens a context menu with the same
  action.
- The action creates a copy of the book in the `split` stage at once. The copy
  keeps the source book's name and holds a reference to it.
- A book that is the target of such a reference is `processed`. A processed book
  keeps a checkmark in its top right corner, fades, and offers no Dewarp action.
- Add `POST /api/books/{id}/dewarp`. It creates the copy and queues one dewarp
  job in one transaction. It re-queues the job when a previous attempt failed.
- Add a dewarp worker. It uploads the source book's original page images to S3,
  starts a RunPod job, polls it to the end, downloads the dewarped pages into
  the copy, then deletes the S3 prefix.
- Add S3 and RunPod configuration: `AWS_ENDPOINT_URL`, `AWS_ACCESS_KEY_ID`,
  `AWS_SECRET_ACCESS_KEY`, `AWS_BUCKET`, `AWS_REGION`, `DEWARP_API_URL`, and
  `DEWARP_API_TOKEN`. All are optional. Without them the server starts and the
  Dewarp action reports that dewarp is not configured.
- Turn `runpod/dewarp/` into a RunPod serverless worker. Add `handler.py`, which
  reads its input pages from S3 and writes its output pages back to S3. Split
  the pipeline out of `docpp.py` so the handler holds one warm pipeline across
  jobs. Keep the command line tool working.
- Add a build and push script for the worker image.
- Add a fake RunPod endpoint and a MinIO service to the e2e stack, so the whole
  path runs in tests without a GPU.

## Capabilities

### New Capabilities

- `book-dewarp`: Dewarping a book on a remote GPU endpoint. Covers the action
  and its two entry points, the copy in the `split` stage, the reference that
  marks the source book processed, the S3 exchange, the RunPod job lifecycle,
  and failure and retry.

### Modified Capabilities

- `book-library`: The stage tab row now holds two tabs, `Dewarp` and `Split`. A
  processed book card shows a checkmark and fades.

## Impact

**New backend packages**: `internal/dewarp` (worker, RunPod client, job args),
`internal/objectstore` (S3 client wrapper).

**Changed backend**: `internal/books/stages.go` (the `split` stage),
`internal/books/store.go` (the `source_book_id` column, the derived `processed`
flag, creating a derived book), `internal/httpapi/` (the new route and handler,
two new summary fields), `internal/config/config.go` (seven new variables),
`internal/queue/queue.go` (a `dewarp` queue), `cmd/server/main.go` (wiring),
`internal/db/migrations/0003_dewarp.sql`.

**New dependencies**: `aws-sdk-go-v2` and its `service/s3` module in the
backend. `boto3` and `runpod` in the worker image. No new services in
production.

**Changed frontend**: `views/BookView.vue` (the Dewarp button and the
checkmark), `views/StageView.vue` (the context menu, the faded card, a retry
control on a failed copy), a new `components/ContextMenu.vue`,
`api/client.ts` (the new call and fields).

**Changed worker image**: `runpod/dewarp/Dockerfile` (`boto3`, `runpod`, the
handler as the entry point), `runpod/dewarp/docpp.py` (a reusable pipeline
function), new `runpod/dewarp/handler.py`, `runpod/dewarp/build.sh` (tag and
push). `runpod/dewarp/example/` stays as the reference it is.

**Changed infrastructure**: `docker-compose.yml` and `docker-compose.prod.yml`
pass the S3 and RunPod variables to the backend. `docker-compose.e2e.yml` gains
MinIO and a fake RunPod endpoint. `.env.example` documents the new variables.

**New tool**: `backend/tools/fakerunpod`, a RunPod compatible stub that copies
the input objects to the output prefix. Tests only.

**Risk**: A 500 page book at 300 DPI moves roughly 3 GB to S3 and 3 GB back.
The upload runs before the GPU job starts, so a slow link delays every book
behind it. The dewarp queue therefore runs one job at a time, and the whole book
fails if any page fails.
