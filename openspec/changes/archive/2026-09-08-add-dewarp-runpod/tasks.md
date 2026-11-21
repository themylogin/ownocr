## 1. Data model and stages

- [x] 1.1 Add `backend/internal/db/migrations/0003_dewarp.sql` with the
      `source_book_id` column and its index; verify the server starts and
      `\d book` shows the column and the foreign key
- [x] 1.2 Add `{ID: "split", Title: "Split"}` to `books.Stages`; verify
      `GET /api/stages` returns both stages in order
- [x] 1.3 Extend `books.Book` with `SourceBookID` and `Processed`, put both last
      in `bookColumns` and in the struct, and derive `Processed` with the
      `EXISTS` sub-select in every list and get query; verify
      `go test ./internal/books/...` passes
- [x] 1.4 Add `CreateDerivedTx` (name, storage key, source book id) and
      `RequeueDewarp` (a failed copy back to `pending`) to `books.Store`; verify
      new store tests cover both

## 2. Configuration and object storage

- [x] 2.1 Add the seven dewarp variables to `config.Config` with the
      all-or-none rule and the `AWS_REGION` default; verify new cases in
      `config_test.go` cover absent, complete, and partial sets
- [x] 2.2 Add `aws-sdk-go-v2` and `service/s3` to `go.mod`; verify
      `go mod tidy` and `go build ./...` succeed
- [x] 2.3 Add `internal/objectstore` with `Put`, `Get`, `List`, `DeletePrefix`,
      path-style addressing, and a `New` that returns `nil` when unconfigured;
      verify a unit test covers key building and the nil case
- [x] 2.4 Wire the object store and the dewarp settings into `cmd/server/main.go`
      and `httpapi.Deps`; verify the server starts both with and without the
      variables

## 3. Dewarp API

- [x] 3.1 Add `internal/dewarp/args.go` with `Args`, `Kind`, and
      `QueueName = "dewarp"`, and register the queue in `queue.New` with
      `MaxWorkers: 1`; verify `go build ./...` succeeds
- [x] 3.2 Add `POST /api/books/{id}/dewarp`: guard the source book, lock its
      row, create the copy and insert the job in one transaction, and re-queue a
      failed copy instead of creating a second one; verify each row of the
      decision table in design.md with a request
- [x] 3.3 Publish `book.created` for the copy and `book.updated` for the source
      book from the handler; verify a WebSocket client receives both
- [x] 3.4 Add `processed` and `sourceBookId` to `bookSummary` and to the event
      payload in `ingest.bookEvent`; verify `GET /api/books` and a WebSocket
      event both carry the two fields

## 4. Dewarp worker

- [x] 4.1 Add `internal/dewarp/runpod.go`: start a job, poll the status, map a
      failed, cancelled, or timed out job to an error that repeats the
      endpoint's reason; verify unit tests drive it against an `httptest` server
- [x] 4.2 Add `internal/dewarp/worker.go` steps 1 to 4 from design.md: load the
      copy and its source, set `processing`, clear page rows, the page directory
      and the S3 prefix, then upload every `orig` rendition under
      `dewarp/<key>/in/`; verify a test with a fake object store sees one padded
      object per page
- [x] 4.3 Add steps 5 to 7: run the endpoint job, download every output, store
      it through `ingest.WriteRenditions`, insert the page rows in input order,
      complete the copy into the `split` stage, then delete the prefix; verify a
      worker test produces the page rows and leaves no object
- [x] 4.4 Fail the copy when `failed` is non-empty, an output is missing, or the
      page count differs from the input count, naming the pages at fault; verify
      a test asserts the reason text
- [x] 4.5 Reuse the `reportFailure` rule so only the last attempt sets `failed`,
      and register the worker in `main.go`; verify a test with attempts
      remaining leaves the copy in `processing`

## 5. RunPod worker image

- [x] 5.1 Split `build_pipeline` and `dewarp_files` out of `docpp.py` and make
      `main` call them; verify the CLI still runs against a local directory
- [x] 5.2 Add `runpod/dewarp/handler.py`: build the pipeline once at import,
      list the input prefix, download, dewarp, upload to the output prefix, and
      return `pages` and `failed`; verify the file imports with a stubbed
      `paddlex`
- [x] 5.3 Add `boto3` and `runpod` to the image, copy `handler.py`, and set the
      handler as the default command with an empty entry point; verify the
      Dockerfile keeps the documented `--entrypoint python` CLI invocation
- [x] 5.4 Extend `runpod/dewarp/build.sh` with an `IMAGE` tag and a `--push`
      flag; verify `IMAGE=... ./build.sh --help` prints its usage without
      building
- [x] 5.5 Add `runpod/dewarp/README.md` with the build, push, endpoint creation,
      and job input contract; verify it names every field design.md lists

## 6. Interface

- [x] 6.1 Add `processed` and `sourceBookId` to the `Book` type and
      `dewarpBook(id)` to `api/client.ts`; verify `npm run typecheck` passes
- [x] 6.2 Add the Dewarp button and the corner checkmark to `BookView.vue`, and
      set `processed` locally on success; verify the button gives way to the
      checkmark with no reload
- [x] 6.3 Add `components/ContextMenu.vue` that opens at the pointer and closes
      on Escape, outside click, and scroll; verify all three close paths by hand
- [x] 6.4 Open the menu from a card in `StageView.vue`, fade a processed card,
      draw its corner checkmark, and hide the action for a processed book;
      verify a processed card renders faded with the checkmark
- [x] 6.5 Add the Retry control to a failed book that carries a `sourceBookId`;
      verify it posts to that source book and the copy returns to `pending`

## 7. Test and run infrastructure

- [x] 7.1 Add `backend/tools/fakerunpod` with `/run`, `/status/{id}`, and
      `/control/fail`, copying input objects to the output prefix; verify
      `go run ./tools/fakerunpod` answers a hand-made job
- [x] 7.2 Add MinIO, a bucket creation step, and the fake endpoint to
      `docker-compose.e2e.yml`, and point the backend's seven variables at them;
      verify `./run-e2e.sh` brings the stack up with the new services healthy
- [x] 7.3 Pass the seven variables through `docker-compose.yml` and
      `docker-compose.prod.yml` from `.env`; verify `docker compose config`
      shows them on the backend service
- [x] 7.4 Document the seven variables in `.env.example`, including the warning
      that the endpoint receives the bucket credentials; verify the file lists
      all seven

## 8. End-to-end coverage

- [x] 8.1 Add `e2e/tests/dewarp.spec.ts` helpers: reset MinIO and the fake
      endpoint between tests; verify the suite still passes with the helper in
      place
- [x] 8.2 Cover the action and its two entry points; verify the scenarios of
      "A book at the dewarp stage offers a Dewarp action" pass
- [x] 8.3 Cover the copy, the checkmark, the fade, and the live update to a
      second client; verify the scenarios of "The Dewarp action creates a copy
      at once" and "A processed book shows a checkmark without a reload" pass
- [x] 8.4 Cover the finished copy: page count, page order, renditions, the Split
      tab, and an empty prefix; verify the scenarios of "The dewarped pages
      become the pages of the copy" pass
- [x] 8.5 Cover failure and retry through `/control/fail`; verify the scenarios
      of "A failed dewarp keeps the copy and allows a retry" pass
- [x] 8.6 Cover the two library scenarios that changed: the tab row and the
      faded processed entry; verify `library.spec.ts` passes with its updated
      expectations
- [x] 8.7 Run the whole suite against the production stack; verify
      `./run-e2e.sh` and the prod overlay both finish green

## 9. Documentation

- [x] 9.1 Update `README.md`: the two stages, the dewarp flow, the new endpoint,
      the seven variables, and the RunPod worker; verify no sentence in the
      added text passes 25 words
