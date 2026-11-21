## Why

The stages that come next need numpy and OpenCV. Go has no equal for this work.
The backend image runs on Alpine, so it links against musl, and the OpenCV
wheels on PyPI need glibc. Adding Python to that image means building OpenCV
from source or moving the backend off Alpine.

A separate Python image solves the packaging problem once. This change solves it
alone, before any stage depends on the answer.

## What Changes

- A new `sidecar/` directory holds a Python image on `python:3.13-slim-trixie`,
  with numpy and OpenCV installed from wheels. A `requirements.txt` pins every
  direct dependency exactly.
- A new `sidecar` service joins all four compose stacks. It mounts the same
  `book_data` volume as the backend, and publishes no port.
- The service runs FastAPI on uvicorn, with synchronous handlers. It answers
  `GET /health` and nothing else yet.
- The service runs one worker and holds no state.

### Non-goals

- Any image processing. The service does no work yet. The first operation
  arrives with the stage that needs it.
- Any Go change. Nothing calls the sidecar, so the backend gains no client, no
  setting, and no dependency. Adding them now would be dead code, and a required
  setting for an unused service would only stop the server from starting.
- Any change to Split, Dewarp, or Ingest. No stage moves to Python here.
- GPU work. The sidecar is CPU only. Dewarp keeps its RunPod endpoint.

## Capabilities

### New Capabilities

None. The service does no work yet, so no behaviour is observable.

### Modified Capabilities

None. No stage changes, and nothing a user can see changes.

This change sets `skip_specs: true`. It delivers packaging and wiring alone.

## Impact

- A new `sidecar/` directory at the root, beside `runpod/`.
- `docker-compose.yml`, `docker-compose.dev.yml`, `docker-compose.dev-e2e.yml`,
  and `docker-compose.prod-e2e.yml` each gain the service.
- `README.md` gains the service, the layout entry, and a note that nothing calls
  it yet.
- No Go file changes. No database change. No frontend change.
