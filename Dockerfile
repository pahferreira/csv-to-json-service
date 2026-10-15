# syntax=docker/dockerfile:1

FROM golang:1.25 AS build

WORKDIR /src

# Module files first: dependency download then caches independently of source,
# so editing a .go file does not re-fetch the module graph.
COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ ./cmd/
COPY internal/ ./internal/

ARG VERSION=dev

# CGO_ENABLED=0 is required, not tidiness: distroless static carries no libc,
# and net would otherwise link the cgo resolver and fail to start.
# -s -w drop the symbol table and DWARF; -X stamps the version. A wrong import
# path in -X fails silently, so `make docker` is paired with a version check.
RUN CGO_ENABLED=0 GOOS=linux go build \
      -trimpath \
      -ldflags "-s -w -X main.version=${VERSION}" \
      -o /out/server ./cmd/server

# static carries CA certificates, which ADR-0008's OTLP exporter needs for TLS.
FROM gcr.io/distroless/static:nonroot

COPY --from=build /out/server /server

USER nonroot:nonroot

# Documentation only. PORT is required with no default (ADR-0011) and Cloud Run
# injects 8080; the server binds whatever PORT says.
EXPOSE 8080

ENTRYPOINT ["/server"]
