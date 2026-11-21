## 1. Configuration loader

- [x] 1.1 Add the `env` accumulator to `backend/internal/config/config.go` with
      `require`, `orElse`, `positiveOrElse` and `err`, exactly as design.md
      sketches them. Verify `go build ./...` passes with the type still unused.
- [x] 1.2 Rewrite `Load` as one `Config` struct literal that reads every
      variable through the accumulator. Return `Config{}` and the joined error
      when `err()` is not nil. Delete `loadDewarp`, `Dewarp.Configured`, the
      `envOr` helper, and the `sort` import. Verify `go vet ./internal/config`
      reports no unused import or function.
- [x] 1.3 Confirm `require` covers exactly `DATABASE_URL`,
      `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_BUCKET`,
      `DEWARP_API_URL` and `DEWARP_API_TOKEN`. Verify `AWS_ENDPOINT_URL` still
      uses `os.Getenv` and `AWS_REGION` still uses `orElse` with
      `defaultRegion`.
- [x] 1.4 Keep the trailing-slash trim on `DEWARP_API_URL` by wrapping the
      `require` call in `strings.TrimRight`. Verify the existing assertion in
      `TestDewarpComplete` on `https://api.runpod.ai/v2/abc` still passes.

## 2. Configuration tests

- [x] 2.1 Delete `TestDewarpIsOptional` and `TestDewarpEndpointStaysOptional`
      from `config_test.go`. Verify `go test ./internal/config` still compiles
      and that no test references `Configured`.
- [x] 2.2 Replace them with a test that clears `AWS_BUCKET` and
      `DEWARP_API_TOKEN` together, then asserts one error names both. This
      covers the accumulator, which is the reason for the change. Verify the
      test fails against the old first-error behavior.
- [x] 2.3 Add a test that an empty endpoint URL and a default region still load
      with the five required variables set. Verify it asserts `Region ==
      defaultRegion` and `Endpoint == ""`.
- [x] 2.4 Update `setDewarpEnv` and any test that expects `Load` to succeed with
      empty dewarp variables. Verify `go test ./...` passes for the whole
      backend.

## 3. Object store and its callers

- [x] 3.1 Delete `Settings.complete()` and the nil return in
      `backend/internal/objectstore/store.go`. `New` returns `*Store`
      unconditionally. Verify `go build ./...` names every caller that checks
      for nil.
- [x] 3.2 Remove the `objects == nil` warning and the `if objects != nil` guard
      around `AddWorker` in `backend/cmd/server/main.go`. The dewarp worker
      always registers. Verify the server starts under
      `docker-compose.dev-e2e.yml` and logs no dewarp warning.
- [x] 3.3 Remove the nil-means-unconfigured comment on `Deps.Objects` in
      `backend/internal/httpapi/router.go`. Verify no comment in the package
      still claims the field can be nil.
- [x] 3.4 Remove the `ActionDewarp && a.deps.Objects == nil` branch and its 503
      answer from `backend/internal/httpapi/actions.go`. Verify
      `go test ./internal/httpapi` passes and no test expects a 503 from the
      Dewarp action.
- [x] 3.5 Replace the `store == nil` check in `dewarpPrefix` in
      `backend/tools/fakerunpod/main.go` with a direct check on
      `input.AccessKeyID`, `input.SecretAccessKey` and `input.Bucket`. Keep the
      "the job carried no bucket credentials" error. Verify a job posted with an
      empty bucket still gets that error.
- [x] 3.6 Delete the `s.store == nil` checks in `controlObjects` and
      `controlPurge` in `backend/tools/fakerunpod/main.go`. They guard a store
      built from the environment, which is never nil now. Verify the e2e suite
      still drives both control endpoints.

## 4. Deployment and documentation

- [x] 4.1 Change the five required variables in `docker-compose.yml` and
      `docker-compose.dev.yml` from `${VAR:-}` to bare `${VAR}`. Leave
      `AWS_ENDPOINT_URL` and `AWS_REGION` as they are. Verify
      `docker compose -f docker-compose.yml config` renders them empty rather
      than failing.
- [x] 4.2 Rewrite the Dewarp comment block in `.env.example`. State that all
      five variables are required and that the server refuses to start without
      them. Remove the sentence about running with them empty. Verify no line
      exceeds the file's existing width.
- [x] 4.3 Mark the five variables required in the README variable table, and
      state that development needs real credentials or a local S3. Verify the
      table no longer shows `—` as their default.

## 5. Verification

- [x] 5.1 Start the backend with `AWS_BUCKET` and `DEWARP_API_TOKEN` empty.
      Verify it exits and that the single error names both variables on
      separate lines.
- [x] 5.2 Run the full e2e suite with `./run-e2e.sh`. The e2e composes already
      set every variable, so verify the suite passes unchanged.
- [x] 5.3 Run `openspec validate require-dewarp-config --strict` and verify it
      reports no problem with the delta spec.
