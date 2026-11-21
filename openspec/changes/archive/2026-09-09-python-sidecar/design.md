## Context

See proposal.md for the motivation. Two facts of the current system shape the
decisions below.

The backend image is `golang:1.27-alpine`, so it links against musl. The OpenCV
wheels on PyPI are manylinux, which needs glibc. numpy ships musllinux wheels;
OpenCV does not. Alpine carries `py3-opencv`, but the version lags and the build
is not the one the wheels give.

The `book_data` volume already mounts in more than one container. The backend
writes it and the frontend reads it. A third container can mount it too.

This change ships a service that nothing calls. The decisions below therefore
split in two: what the service is today, and the shape the first operation must
follow. The second half is written down now because the reasoning came from the
sizing of the real work, and it would be expensive to rediscover.

## Goals / Non-Goals

**Goals:**

- Keep numpy and OpenCV out of the Go image.
- Prove the image builds, the service starts, and both containers see the same
  volume.
- Record the contract shape so the first operation does not reopen it.

**Non-Goals:**

- Any processing operation, and therefore any request but `/health`.
- Any Go code. The client, its timeout, and the setting arrive with the first
  caller.
- Authentication. The service has no published port and serves one backend on an
  internal network.
- A stable contract. Both sides will ship together, so the request shape can
  change in one commit.

## Decisions

### A service, not a subprocess

The Go workers could call `python -m something` and read the result off stdout.
That needs no port, no server, and no health check, and River already turns a
nonzero exit into a retry. It is the simpler design on every axis but one:
Python then lives in the backend image, which is the musl problem above.

A separate image is therefore the point of the sidecar. Once a separate image
exists, HTTP costs almost nothing over a pipe, and it keeps the containers
independent: the sidecar restarts without the backend.

### Debian 13 trixie, from the official Python image

The base is `python:3.13-slim-trixie`. Trixie is the current Debian stable, and
the tag names it rather than tracking whatever `slim` points at, so the next
Debian release does not arrive in an unrelated rebuild.

The official Python image is the base, not `debian:trixie-slim` with the system
Python. `pip` then installs without a virtual environment, because these images
are not marked externally managed. The venv in `runpod/dewarp/Dockerfile` exists
only because its base is a CUDA image with a system Python; nothing here needs
it.

### Dependencies in a requirements file, pinned exactly

`sidecar/requirements.txt` lists every direct dependency with an `==` pin. The
Dockerfile copies it and installs before it copies the code, so editing the
server does not reinstall OpenCV.

`runpod/dewarp/Dockerfile` does it differently. It pins some direct
dependencies as build args and installs them inline, and leaves
`opencv-python-headless` and `boto3` unpinned. That suits an image built by hand
and pushed once to a registry tag.

The sidecar is rebuilt by `docker compose up --build` and by CI, in all four
stacks. A floating dependency there breaks a build during unrelated work. The
requirements file is the smaller half of the fix; the pins are the rest.

Transitive dependencies stay unpinned. `pip-compile` or `uv lock` would pin the
whole closure with hashes, which is the only thing that makes a build truly
reproducible. Four direct dependencies do not earn that tooling yet, and the
requirements file is what those tools take as input, so nothing is wasted if it
changes later.

### The sidecar runs as the backend's user in production

Production runs the backend as uid 10001 and would run the sidecar as root. Both
write the data volume, and every worker deletes a page directory before each
attempt. A root owned file refuses that delete, so the first operation would
fail on a retry, in production alone. Development hides it, because both
containers run as root there.

The image declares the user, as the backend image does, rather than a compose
`user:` line. The Dockerfile has a `dev` target and a `prod` target on a shared
base: `prod` ends with `USER ownocr`, and `dev` returns to root, which matches
the development backend. The `dev` target also installs `requirements-dev.txt`,
so a reload watcher never reaches production.

The image also creates `/data` and gives it to 10001, as the backend image
already does. Docker takes the ownership of a new named volume from the image
directory it covers, so without this the first service to mount an empty volume
decides. The sidecar owning no `/data` would leave the volume to root, and then
neither service could write it.

### One package, one subpackage per action

The code is a package, `ownocr`, not a loose module. Every action that moves to
Python becomes a subpackage of it, `ownocr/<action>/`, which mirrors the Go side,
where `internal/dewarp` and `internal/split` are siblings.

`pip install --no-deps -e .` installs it. Editable, so the development bind mount
serves the same package the image installed, and `--no-deps` so the pinned
install from `requirements.txt` stays on its own cached layer. `pyproject.toml`
takes its dependency list from that file, so the versions live in one place.

### FastAPI on uvicorn

The contract ahead is a JSON list of paths. Pydantic validates it at the edge
for free, which is worth the larger dependency tree over a bare WSGI stack.
`waitress` with `flask` was the alternative: a smaller tree, and no async
machinery at all, but no validation.

Gunicorn is the choice to avoid. Its `--timeout` is a worker liveness watchdog,
not a request timeout, and its default of 30 seconds kills a worker that spends
longer inside OpenCV, because a worker busy in C code looks hung. That produces
the worst failure mode: small inputs pass and large inputs fail. Uvicorn has no
such default. If gunicorn ever appears, `--timeout 0` is required.

The server must never cut its own work short. Bounding a request is the caller's
job.

### One worker, synchronous handlers, no thread pool

The queues that will call this service run one River worker each, so the sidecar
sees one request at a time.

Page level parallelism, when it is wanted, belongs inside OpenCV and BLAS, which
thread internally and release the GIL. One image across all cores beats several
images each on one core: the same throughput, lower latency, and one large array
alive instead of several.

A Python thread pool over numpy would work. numpy releases the GIL on arrays
above a few hundred elements, and OpenCV releases it almost everywhere. It is
still the wrong tool here. It multiplies peak memory, it oversubscribes the
cores OpenCV already uses, and a segmentation fault in a C library takes the
whole process down instead of one request.

Handlers are synchronous. Asyncio has nothing to overlap in CPU work, and an
`async def` handler running numpy blocks the event loop outright. Write every
handler as `def`, never `async def`: Starlette then runs it in a thread pool and
the event loop stays free. This is the one rule of the framework choice that
matters, and the easy one to get wrong.

When two queues call the sidecar later, their requests will serialize on one
worker. That is acceptable for CPU work, and must be a deliberate choice then,
not a surprise.

### The shape the first operation must follow

Nothing here is built in this change. It is recorded so the next one does not
reopen it.

**Paths, not bytes.** Both containers mount `DATA_DIR`. A request holds paths
under it and an answer holds paths under it. No image travels over HTTP.
Multipart or base64 bodies would move gigabytes through the loopback for no gain
and buffer a whole book in memory.

**One request per book, not per image.** A book of 500 pages at 0.1 seconds a
page is about 100 seconds, and a request of that length is ordinary. Per book
wins because the algorithms are per book: a later one reads every page to pick a
single parameter for the whole book, which a per-image contract cannot express.
The cost is that a copy reports one phase for the whole call, and that a failure
names one page but restarts the book.

**A directory per request.** A synchronous Python server does not observe a
closed connection, so a request the caller abandoned runs to its end. The
workers empty the page directory of a copy at the start of every attempt, so an
abandoned request could write into a directory the next attempt just created.
The sidecar must create its own output directory per request and return the
paths it wrote. The caller must read only those paths.

**The timeout budget.** Go's `http.Client` has no timeout by default, so a
wedged sidecar would hold a worker forever.

| Limit                   | Value      | Reason                                   |
| ----------------------- | ---------- | ---------------------------------------- |
| `http.Client.Timeout`   | 30 minutes | Far above a normal book. Catches a hang. |
| Sidecar request timeout | none       | The server must not cut its own work.    |
| River `Timeout()`       | 6 hours    | Unchanged. Covers every attempt.         |

**Startup.** The first caller adds its setting to the required set, as the
dewarp variables already are. It must not check that the sidecar answers at
startup: a sidecar slow to boot must not stop the backend. A job that runs too
early fails and retries, which River already gives.

## Risks / Trade-offs

- **A service nothing calls can rot.** → The e2e stacks start it, so a broken
  image fails CI rather than the change that first needs it.
- **A second image to build and ship.** → It is CPU only and small. The e2e
  suite runs the real service, so it needs no stub, unlike the RunPod endpoint.
- **The recorded contract may not survive contact with the first operation.** →
  It is written as guidance, not as a spec, and no code depends on it yet.
- **`book_data` becomes a shared write surface.** → The service writes nothing
  today. The first operation must write only into its own output directory, and
  never into `pages/`.

## Migration Plan

No data migration, no schema change, no Go change. Deploy adds one container.
Rollback removes it. Nothing depends on it, so either direction is safe in any
order.

## Open Questions

- Whether the transitive closure ever needs pinning with `pip-compile` or
  `uv lock`. Four direct dependencies do not earn it today. A build that breaks
  on a dependency nobody named is the signal to revisit.
