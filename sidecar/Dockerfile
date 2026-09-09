FROM python:3.13-slim-trixie AS base

ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1

WORKDIR /app

COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt

COPY pyproject.toml ./
COPY ownocr ./ownocr

RUN pip install --no-cache-dir --no-deps -e .

RUN groupadd --gid 10001 ownocr \
    && useradd --uid 10001 --gid 10001 --no-create-home --shell /usr/sbin/nologin ownocr

# Docker seeds a new named volume's ownership from the image directory it
# covers. Without this, a fresh data volume belongs to root whenever this
# service mounts it before the backend does, and then neither of them can write.
RUN mkdir -p /data && chown ownocr:ownocr /data

EXPOSE 8000

FROM base AS dev

COPY requirements-dev.txt ./
RUN pip install --no-cache-dir -r requirements-dev.txt

# Development mounts the source over /app and runs the backend as root, so this
# service matches it. A file either of them writes stays writable by the other.
USER 0

CMD ["uvicorn", "ownocr.app:app", "--host", "0.0.0.0", "--port", "8000", "--reload"]

FROM base AS prod

USER ownocr

CMD ["uvicorn", "ownocr.app:app", "--host", "0.0.0.0", "--port", "8000", "--workers", "1"]
