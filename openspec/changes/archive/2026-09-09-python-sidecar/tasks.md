## 1. The image

- [x] 1.1 Create `sidecar/requirements.txt` pinning numpy,
      `opencv-python-headless`, `fastapi`, and `uvicorn` with `==`, and verify
      `pip install -r` resolves with no conflict
- [x] 1.2 Create `sidecar/Dockerfile` on `python:3.13-slim-trixie` that copies
      the requirements file and installs it before it copies the code, and
      verify `docker build` succeeds
- [x] 1.3 Verify the wheels are the PyPI ones, not a source build: run
      `python -c "import cv2, numpy; print(cv2.__version__, numpy.__version__)"`
      in the built image
- [x] 1.4 Verify the install layer caches: edit the server file, rebuild, and
      confirm the build does not reinstall the dependencies

## 2. The server

- [x] 2.1 Write the FastAPI app with `def` handlers and no `async def`, run it
      under uvicorn, and verify `GET /health` answers 200
- [x] 2.2 Verify no handler is `async def`, by grepping the server file
- [x] 2.3 Run uvicorn with one worker and confirm no request timeout is
      configured, by checking the start command carries no timeout flag
- [x] 2.4 Verify nothing cuts a long request short: add a temporary handler that
      sleeps 120 seconds, call it, and confirm the client receives the answer
- [x] 2.5 Remove the temporary handler and verify only `/health` remains

## 3. Compose wiring

- [x] 3.1 Add the `sidecar` service to `docker-compose.yml` with the `book_data`
      volume and no published port, and verify `docker compose config` is valid
- [x] 3.2 Add it to `docker-compose.dev.yml` with the source mounted for reload,
      and verify an edit to the server takes effect without a rebuild
- [x] 3.3 Add it to `docker-compose.dev-e2e.yml` and
      `docker-compose.prod-e2e.yml`, and verify `./run-e2e.sh` still passes
- [x] 3.4 Add a compose healthcheck on `/health`, and verify the service reports
      healthy with `docker compose ps`

## 4. Verify the volume is shared

- [x] 4.1 Write a file into `DATA_DIR` from the backend container and verify the
      sidecar container reads it at the same path
- [x] 4.2 Verify the sidecar can write under `DATA_DIR`, so a later operation is
      not blocked by ownership or permissions

## 5. Documentation

- [x] 5.1 Update `README.md`: the service, the layout entry, and a note that
      nothing calls it yet
- [x] 5.2 Run `openspec validate python-sidecar --strict` and verify it passes
