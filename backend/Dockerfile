FROM golang:1.27-alpine AS dev

RUN apk add --no-cache poppler-utils

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

EXPOSE 8080
CMD ["go", "run", "./cmd/server"]

FROM dev AS build

RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ownocr ./cmd/server
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ocrmerge ./tools/ocrmerge

FROM alpine:3.21 AS prod

RUN apk add --no-cache ca-certificates poppler-utils \
    && adduser -D -u 10001 ownocr

COPY --from=build /out/ownocr /usr/local/bin/ownocr

# ocrmerge rewrites stored results and never serves anything, so it runs as a
# one-off container and not as the entrypoint. It ships here because it needs the
# database of the deployment, which nothing outside this network can reach.
COPY --from=build /out/ocrmerge /usr/local/bin/ocrmerge

# Docker seeds a new named volume's ownership from the image directory it
# covers. Without this, /data belongs to root and the unprivileged server
# cannot create its file store.
RUN mkdir -p /data && chown ownocr:ownocr /data

USER ownocr
ENV ADDR=":8080" DATA_DIR="/data"
EXPOSE 8080
VOLUME ["/data"]

ENTRYPOINT ["ownocr"]
