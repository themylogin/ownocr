## Why

Dewarp is optional configuration today. The server starts with no bucket and no
endpoint, and the Dewarp action answers 503. That branch runs through six files:
the config loader, the object store, `main`, the router, the action handler, and
the spec.

The branch buys a half-working server. Nobody wants to run one. Every deployment
sets the dewarp variables, and the e2e composes already set all of them.

## What Changes

- **BREAKING** Require `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`,
  `AWS_BUCKET`, `DEWARP_API_URL` and `DEWARP_API_TOKEN`. The server refuses to
  start when one of them is empty.
- **BREAKING** Development needs real credentials or a local S3. A copy of
  `.env.example` with empty dewarp variables no longer starts the server.
- Remove `Dewarp.Configured()` and the all-or-nothing rule in `loadDewarp`.
- Remove the nil return from `objectstore.New`. The store is never nil, so the
  router and the action handler drop their nil checks. `fakerunpod` gains an
  explicit check on the job payload, which is the one place the nil did work.
- Always register the dewarp worker. Remove the startup warning.
- Remove the 503 "dewarp is not configured" answer from the action handler.
- Replace the six per-variable checks in `config.Load` with one accumulator. It
  names every missing or invalid variable in a single error.

`AWS_ENDPOINT_URL` stays optional. An empty value means real AWS, which is a
value and not an omission. `AWS_REGION` keeps its `us-east-1` default.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `book-dewarp`: Remove "Dewarp is optional configuration" and its three
  scenarios. The server needs the settings to start, so no request can find them
  missing. Nothing replaces the requirement: a server that refuses to start is
  not behavior the dewarp capability promises.

## Impact

- Backend: `internal/config/config.go` loses `loadDewarp`, `Configured()` and the
  `sort` import, and gains the accumulator. `internal/objectstore/store.go`
  loses `Settings.complete()` and the nil return. `cmd/server/main.go` loses the
  warning and the `if objects != nil` guard. `internal/httpapi/router.go` and
  `internal/httpapi/actions.go` lose their nil checks. `tools/fakerunpod`
  validates the credentials in the job body instead of reading a nil store.
- Tests: `internal/config/config_test.go` drops `TestDewarpIsOptional` and
  `TestDewarpEndpointStaysOptional`. It gains a test that one error names two
  missing variables.
- Deployment: `docker-compose.yml` and `docker-compose.dev.yml` drop the `:-`
  empty defaults on the five required variables.
- Docs: `.env.example` and the README variable table mark the five as required.
- API: no route changes. The Dewarp action loses one failure answer.
- Frontend: none. No client reads the 503.
