## 1. Shared RunPod client

- [x] 1.1 Move `internal/dewarp/runpod.go` and `runpod_test.go` to a new
  `internal/runpod` package. `NewClient` takes the endpoint name, `Run` takes
  an `any` input, and `Wait` returns the raw JSON output. Verify with
  `go test ./internal/runpod/...`.
- [x] 1.2 Point `internal/dewarp` at `runpod.NewClient("dewarp", ...)`, keep
  `JobInput`, `Output`, `OutputPage` and `OutputFailure` in the dewarp package,
  and decode the raw output there. Verify with `go test ./internal/dewarp/...`.

## 2. Configuration

- [x] 2.1 Add `PaddleOCRVL{APIURL, APIToken}` to `config.Config`, required like
  the dewarp settings, read from `PADDLEOCR_VL_API_URL` and
  `PADDLEOCR_VL_API_TOKEN`, with the URL trimmed of a trailing slash. Verify
  with a `config_test.go` case that a missing variable names itself.
- [x] 2.2 Add both variables to `docker-compose.yml` and
  `docker-compose.dev.yml`, and document them in `.env.example` beside the
  dewarp block. Verify with `docker compose config` on both files.
- [x] 2.3 Rename `PADDLEOCR_VL_AP_URL` to `PADDLEOCR_VL_API_URL` in the local
  `.env` file. Verify that the dev backend starts and logs "listening".

## 3. Action and queue

- [x] 3.1 Add the `ocr` action to `books.Actions`, titled "OCR", with stage
  `done` and `OnePageEach: true`, plus the `ActionOCR` constant. Verify with
  `go test ./internal/books/...`, extended for the actions each stage offers.
- [x] 3.2 Add `OCRQueue` with `MaxWorkers: 1` to `queue.New`, and the `ocr`
  branch to `httpapi.queueAction`. Verify with a `httpapi` test that a POST to
  `/api/books/actions/ocr` on a complete book creates a copy in `done` and
  queues one job.

## 4. Markdown in the file store

- [x] 4.1 Add `filestore.PageMarkdownPath(bookKey, pageKey)`, returning
  `<pages>/<book key>/<page key>.md`, and a `PageMarkdownURL` beside
  `PageURL`. Verify with a `store_test.go` case that the name cannot collide
  with a rendition name.

## 5. OCR worker

- [x] 5.1 Create `internal/ocr/args.go`: `QueueName`, `MaxAttempts`, `Args`,
  and the key helpers `Prefix`, `InputPrefix`, `OutputPrefix`, `InputKey`,
  using the `ocr/<book key>/` prefix. Verify with a test that an OCR prefix and
  a dewarp prefix of one storage key differ.
- [x] 5.2 Create `internal/ocr/endpoint.go` with the job input the handler
  reads (`bucket`, `endpoint_url`, `region`, both credentials, `input_prefix`,
  `output_prefix`) and the output it answers (`pages` with `input_key`,
  `output_key`, `chars`, `seconds`, and `failed` with `input_key`, `error`).
  Verify with a test that decodes a recorded answer of the handler.
- [x] 5.3 Create `internal/ocr/worker.go`: load the copy and its source, delete
  the pages, the page directory and the bucket prefix, upload every source
  page, run the endpoint job, and report the phases "Uploading", "Waiting for
  AI", "Processing with AI", "Downloading" and "Converting". Verify with a
  worker test that uses a fake object store and a fake endpoint, as
  `dewarp/worker_test.go` does.
- [x] 5.4 Store the result: for every source page write a new storage key, the
  three renditions from the original rendition of the source page, the page row
  with the source page reference, and the markdown file. Verify with a worker
  test that a 3 page book yields 3 pages, 9 rendition files and 3 markdown
  files that hold the endpoint text.
- [x] 5.5 Fail the copy when the endpoint reports a failed page, when a page has
  no result, or when the count does not match, and name the pages in the
  reason. Verify with worker tests for each of the three cases.
- [x] 5.6 Delete the bucket prefix after a success, then set the copy complete
  and publish the copy and its source book. Verify with a worker test that the
  fake object store is empty at the end.
- [x] 5.7 Register the OCR worker in `cmd/server/main.go` with the config
  settings. Verify with `go build ./...` and a dev start that logs the `ocr`
  queue.

## 6. Test endpoint and e2e

- [x] 6.1 Teach `tools/fakerunpod` to answer an OCR job: an input prefix that
  starts with `ocr/` makes it write one `.md` object per input object, holding
  the input name. Verify by running the tool and posting a job by hand.
- [x] 6.2 Point `PADDLEOCR_VL_API_URL` and `PADDLEOCR_VL_API_TOKEN` at
  `fakerunpod` in `docker-compose.dev-e2e.yml` and
  `docker-compose.prod-e2e.yml`. Verify with `docker compose config` on both.
- [x] 6.3 Add `e2e/tests/ocr.spec.ts`: run the OCR action on a complete book,
  wait for the copy to complete in the Done tab, and read one markdown file
  from `/data`. Verify with `./run-e2e.sh`.

## 7. Documentation and the real endpoint

- [x] 7.1 Update `README.md`: the stage and action list, the OCR section beside
  the dewarp one, the two environment variables, and the source tree entries
  for `internal/ocr` and `internal/runpod`. Verify by reading the file against
  the code.
- [x] 7.2 Build and deploy the `runpod/paddleocr-vl` endpoint, following
  `runpod/paddleocr-vl/README.md`, and put its URL and token in `.env`. Verify
  that `POST <url>/run` with a hand written job input answers a job id.
- [x] 7.3 Run the OCR action against the real endpoint on a small book. Verify
  that the copy completes, that the markdown files hold the text of the pages,
  and that the bucket prefix of the copy is empty afterwards.
  Two one page books ran on the real endpoint. Both copies completed in the
  `done` stage, with the three renditions and the markdown of every page, and
  an empty prefix afterwards. A page carrying text answered 123 characters of
  markdown; a page carrying none answered an empty file, which is the empty
  page the spec allows. A cold worker takes about four minutes to start and
  about two more to load the model, and the first job holds `IN_QUEUE` for that
  whole time. An endpoint whose max workers is zero refuses the job with 409,
  and the copy then carries that answer as its reason.
