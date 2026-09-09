from fastapi import FastAPI

app = FastAPI(title="ownocr sidecar")


# Every handler is `def`, never `async def`. Starlette runs a `def` handler in a
# thread pool, so a handler that spends a minute inside OpenCV leaves the event
# loop free. An `async def` handler doing the same work blocks the whole server.
@app.get("/health")
def health() -> dict[str, str]:
    return {"status": "ok"}
