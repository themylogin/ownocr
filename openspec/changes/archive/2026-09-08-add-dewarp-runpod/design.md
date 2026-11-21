## Context

See proposal.md - Why.

The pieces that shape this design already exist:

- A book is a row in `book` plus rows in `book_page`. The state machine is
  `pending` -> `processing` -> `complete` or `failed`. A complete book carries a
  stage. Nothing else may carry one.
- `internal/ingest` already turns a local image file into three renditions with
  `WriteRenditions`, and it already models a rebuild-on-retry worker.
- River runs jobs in the same Postgres database. Ingest owns a queue with one
  worker so a large book cannot starve the rest.
- The e2e suite runs the whole stack in Docker and covers every scenario in
  `openspec/specs/`. This machine has no GPU, and CI has none either.
- `runpod/dewarp/` builds a CUDA image around PaddleX. `docpp.py` is a command
  line tool that reads a directory and writes a directory.

## Goals / Non-Goals

**Goals:**

- One action, two entry points, immediate feedback, no reload.
- The copy carries all job state. The source book is never written during a
  dewarp.
- The whole path is testable without a GPU and without a RunPod account.
- The GPU worker stays useful as a command line tool.

**Non-Goals:**

- Splitting a page in two. The `split` stage only receives books here.
- Choosing dewarp parameters per book. The endpoint gets one fixed setup.
- Deleting a book, a copy, or a stored file. No delete endpoint exists yet.
- Streaming per page progress from the GPU into the interface. Page counts
  update while pages are stored, and that is enough.

## Decisions

### The reference is a column, and `processed` is derived

Migration `0003_dewarp.sql`:

```sql
ALTER TABLE book ADD COLUMN source_book_id BIGINT
    REFERENCES book (id) ON DELETE SET NULL;

CREATE INDEX book_source_idx ON book (source_book_id);
```

`processed` never becomes a column. Every read derives it:

```sql
EXISTS (SELECT 1 FROM book copy WHERE copy.source_book_id = book.id)
```

The spec says the reference alone decides. A second column would be a second
truth that can disagree with the first. `ON DELETE SET NULL` keeps a copy alive
if its source ever goes away; the copy then simply stops marking anything.

`bookColumns` gains `source_book_id`, and the `Book` struct gains
`SourceBookID *int64`. The list and get queries gain the derived `Processed
bool`. `pgx.RowToStructByPos` maps by position, so the two fields go last in
both the column list and the struct, in the same order.

Alternative rejected: a `processed_at` timestamp on the source book. It answers
"when" but needs a second write that can fail after the copy exists.

### One endpoint serves start and retry

`POST /api/books/{id}/dewarp`, where `{id}` is the **source** book:

| Situation                            | Result                                  |
| ------------------------------------ | --------------------------------------- |
| No copy                              | `201` with the new copy summary         |
| Copy in state `failed`               | `200` with the same copy, job re-queued |
| Copy in any other state              | `409`                                   |
| Book missing                         | `404`                                   |
| Book not complete, or not at `dewarp`| `422`                                   |
| Dewarp not configured                | `503`                                   |

One endpoint keeps the client simple: the retry control on a failed copy posts
to the copy's `sourceBookId`. The handler takes `SELECT ... FOR UPDATE` on the
source row, so two clicks cannot make two copies.

The row and the job commit together, the way `insertBook` already does it. The
queue is `dewarp` with `MaxAttempts: 3`.

Alternative rejected: a separate `POST /api/books/{id}/retry`. It duplicates
every guard for one caller.

### The copy exists before the job runs

`POST` returns after the copy row exists. The interface therefore marks the
source processed at once, which is what the spec requires. The copy appears in
the "Processing" area through the existing `book.created` event, and the handler
also publishes `book.updated` for the source book so other clients fade it.

The copy starts `pending` with `stage = NULL`. It gets `stage = 'split'` only in
`SetComplete`, exactly like an ingested book. This keeps the existing invariant
that only a complete book has a stage.

### The worker mirrors the ingest worker

`internal/dewarp` holds `Args{BookID}` for the copy, `QueueName = "dewarp"`, and
a `Worker` with the same shape as `ingest.Worker`, including `reportFailure` so
only the last attempt marks the book failed.

`Work` does:

1. Load the copy, then its source book and the source pages.
2. `SetProcessing(copy)`, publish.
3. Delete the copy's page rows and page directory, then clear the S3 prefix.
   This is the ingest rebuild rule applied to remote state: an attempt starts
   from nothing.
4. Upload each source page's `orig` rendition to
   `dewarp/<copy storage key>/in/<index>.<ext>`, with the index zero padded to
   four digits.
5. `POST {DEWARP_API_URL}/run`, then poll `{DEWARP_API_URL}/status/{id}`.
6. Download each output object, run it through `ingest.WriteRenditions` into the
   copy's page directory, and insert the page row. Order comes from the input
   index in the object name, not from the order the endpoint reports.
7. `SetComplete(copy, "split", n)`, publish, then delete the prefix.

The queue is `dewarp` with `MaxWorkers: 1`, for the reason ingest has one: a
single book can push gigabytes, and two at once helps nobody. `Timeout` is 6
hours, as for ingest.

`WriteRenditions` is exported already and takes a path, so the worker writes the
downloaded object to a temp file under `DATA_DIR/tmp` and hands over the path.
No ingest code changes.

### The endpoint contract

The RunPod serverless HTTP API is fixed: `POST /run` returns a job id, and
`GET /status/{id}` returns a status with the handler's return value in `output`.
`DEWARP_API_URL` is the endpoint base, for example
`https://api.runpod.ai/v2/<endpoint-id>`. `DEWARP_API_TOKEN` goes in
`Authorization: Bearer`.

Request:

```json
{
  "input": {
    "bucket": "ownocr",
    "endpoint_url": "https://...",
    "region": "us-east-1",
    "aws_access_key_id": "...",
    "aws_secret_access_key": "...",
    "input_prefix": "dewarp/<key>/in/",
    "output_prefix": "dewarp/<key>/out/",
    "jpeg_quality": 95
  }
}
```

Result in `output`:

```json
{
  "pages": [{"input_key": "...", "output_key": "...", "width": 2480, "height": 3508}],
  "failed": [{"input_key": "...", "error": "could not decode"}]
}
```

A non-empty `failed`, a missing output object, or a page count other than the
input count fails the copy. The worker names the pages at fault in the reason.

Polling is every 5 seconds, with a 6 hour ceiling that the River timeout already
enforces. RunPod also offers `/runsync`, but it returns early on a long job and
then needs polling anyway.

The credentials travel in the job input because that is how the example worker
receives them, and RunPod has no other per-job secret channel. They are the same
bucket credentials the backend holds.

### The handler keeps the pipeline warm

`docpp.py` gains two functions and keeps its `main`:

```python
def build_pipeline(device=None): ...        # create_pipeline, then a warm-up pass
def dewarp_files(pipeline, paths, out_dir, ...) -> list[dict]: ...
```

`handler.py` imports them, builds the pipeline once at module level, and reuses
it for every job. Model load is tens of seconds; a RunPod worker serves many
jobs. The example worker shells out to a CLI, which would pay that cost per job.

Per job the handler lists the input prefix, downloads to a temp directory,
dewarps, uploads each result to the output prefix with the input's base name and
a `.jpg` suffix, and returns the two lists. It reports progress with
`runpod.serverless.progress_update`. It deletes nothing: the backend owns the
lifetime of the objects.

The Dockerfile installs `boto3` and `runpod`, copies `handler.py`, and sets
`CMD ["python", "-u", "/opt/handler.py"]` with an empty `ENTRYPOINT`. The CLI
stays reachable:

```sh
docker run --rm --gpus all -v "$PWD:/data" --entrypoint python IMAGE \
    /opt/docpp.py -i /data/in -o /data/out
```

`build.sh` takes an image tag from `IMAGE` and pushes when called with `--push`.
It keeps the `hf_token` build secret.

### The S3 wrapper is thin

`internal/objectstore` wraps `aws-sdk-go-v2` with `Put`, `Get`, `List`, and
`DeletePrefix`. Static credentials, `BaseEndpoint` from `AWS_ENDPOINT_URL`, and
`UsePathStyle: true` so MinIO and other S3 compatible stores work. The SDK is
the only sane choice: SigV4 by hand is a security bug waiting to happen.

`New` returns `nil` when the settings are absent. `Deps.ObjectStore == nil` is
what "dewarp is not configured" means, and the handler answers `503`.

### Configuration is optional and validated together

Seven variables: `AWS_ENDPOINT_URL`, `AWS_ACCESS_KEY_ID`,
`AWS_SECRET_ACCESS_KEY`, `AWS_BUCKET`, `AWS_REGION` (default `us-east-1`),
`DEWARP_API_URL`, `DEWARP_API_TOKEN`.

`config.Load` accepts all seven absent. It rejects a partial set: if any of
`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_BUCKET`, `DEWARP_API_URL`, or
`DEWARP_API_TOKEN` is set, all five must be. `AWS_ENDPOINT_URL` stays optional,
because real AWS needs none. A half configured server that fails at job time is
worse than one that refuses to start.

### The interface

`api/client.ts` gains `processed: boolean` and `sourceBookId: number | null` on
`Book`, and `dewarpBook(id)`.

`BookView.vue` shows a Dewarp button when `stage === 'dewarp' && !processed`,
and a checkmark badge in the top right corner when `processed`. It sets
`processed` locally on success, so the checkmark does not wait for the event.

`StageView.vue` gets `ContextMenu.vue`, a small component that takes a position
and a list of items. `@contextmenu.prevent` on a card opens it. The menu closes
on Escape, on an outside click, and on scroll. A processed card gets a
`processed` class: `opacity: 0.55` and a checkmark in the corner. A failed book
that carries a `sourceBookId` gets a Retry control next to its reason.

The tab row needs no change. It already renders whatever `/api/stages` returns,
and `split` is appended to `books.Stages`.

### Tests run against a fake endpoint

`backend/tools/fakerunpod` is a small Go program that speaks the two RunPod
routes against the same `objectstore` package. `POST /run` starts a goroutine
that copies every input object to the output prefix and records the result.
`GET /status/{id}` reports `IN_PROGRESS`, then `COMPLETED`. `POST /control/fail`
makes the next job fail, which is how the failure and retry scenarios run.

Copying, not dewarping, is the point: the test proves the app's half of the
contract. `docker-compose.e2e.yml` gains MinIO, a one-shot `mc` service that
creates the bucket, and the fake, and it points the backend's seven variables at
them.

Alternative rejected: a stub mode inside the backend. It would test a code path
that production never runs.

## Risks / Trade-offs

- **A 500 page book moves ~3 GB up and ~3 GB back** -> The dewarp queue runs one
  job at a time, so an upload never competes with another. The 6 hour timeout
  covers a slow link. Sending the 1280 px rendition instead was rejected: the
  copy would be the only surviving version.
- **Credentials travel in the job input** -> Use a bucket and a key pair that
  reach nothing else. Document that in `.env.example`.
- **A leaked prefix costs storage** -> A failed job leaves its objects until the
  next attempt clears the prefix. A copy that is never retried keeps them.
  Accepted: the prefix is derivable from the copy's storage key, so a cleanup
  task later is easy.
- **The endpoint may return pages out of order** -> Order comes from the input
  index in the object name. The endpoint's own order is never trusted.
- **A cold RunPod worker adds a minute** -> Accepted. The copy sits in
  `processing` and the user sees it.
- **`pgx.RowToStructByPos` maps by position** -> Two new columns go last in both
  the SQL and the struct. The existing store tests catch a mismatch.
- **MinIO in the e2e stack slows the suite** -> One extra container and a bucket
  creation. Small next to the Playwright image.

## Migration Plan

1. Apply `0003_dewarp.sql`. It only adds a nullable column and an index, so it
   runs on a live database and needs no backfill.
2. Deploy the backend. Without the seven variables, everything behaves as
   before, except the Split tab, which is empty.
3. Build and push the worker image, then create the RunPod endpoint from it.
4. Put the seven variables in `.env` and restart the backend.

Rollback: remove the variables and restart. The action then reports that dewarp
is not configured. The column and any existing copies stay; they are ordinary
books.
