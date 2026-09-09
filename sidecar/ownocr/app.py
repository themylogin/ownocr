from fastapi import FastAPI, HTTPException

from ownocr import split

app = FastAPI(title="ownocr sidecar")


@app.get("/health")
def health() -> dict[str, str]:
    return {"status": "ok"}


@app.post("/split")
def split_pages(paths: list[str]) -> list[list[int] | None]:
    answer: list[list[int] | None] = []
    for path in paths:
        try:
            columns = split.cut_columns(path)
        except split.UnreadableImage as unreadable:
            raise HTTPException(status_code=400,
                                detail=f"cannot read {unreadable.path}") from unreadable
        answer.append(None if columns is None else [columns[0], columns[1]])
    return answer
