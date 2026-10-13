# ADR-0006: Idiomatic hexagonal package structure

- **Status**: Accepted
- **Date**: 2026-10-13

## Context

The service must keep the Conversion logic independent of HTTP so that (a) it is testable
without a server, and (b) a future asynchronous path (ADR-0009) is an added adapter rather
than a rewrite. This repo is also a portfolio artifact, so structure will be read and
judged by Go engineers.

## Decision

Dependencies point inward. The domain imports nothing from adapters.

```
cmd/server/main.go               wiring only
internal/csvjson/                the domain; no HTTP, no chi, no multipart, no telemetry
  mapping.go                     Mapping, Field, Dialect; Parse + Validate
  convert.go                     Converter.Convert(ctx, RowSource, Mapping, RecordSink) (Summary, error)
  transform.go                   named Transform registry
  coerce.go                      typed Coercion
  diagnostics.go                 headers seen / mapped / dropped / unmatched
  errors.go                      error codes
internal/adapter/httpapi/        chi router, middleware, multipart decode, response DTOs, SliceSink
internal/adapter/csvsource/      encoding/csv -> RowSource
internal/platform/config/        typed env config, validated at startup
internal/platform/obs/           slog, OTel metrics and traces, usage events
```

**Exactly two ports**, declared in `internal/csvjson` where they are consumed:

- `RowSource` — `Next() ([]string, error)`
- `RecordSink` — `Write(Record) error`

There is no `ports/` directory and no `services/` directory.

`chi` is the only non-stdlib runtime dependency for the MVP; stdlib covers `encoding/csv`,
`encoding/json`, `log/slog`, and routing-adjacent concerns. OTel SDK and `x/time/rate`
arrive with ADR-0007 and ADR-0008.

## Consequences

- The Converter is unit-testable with a fake `RowSource` and a fake `RecordSink`, needing
  no HTTP server and no files.
- The asynchronous path replaces the **source** (object storage or queue message) and the
  **sink** (write results to storage) and touches the Converter not at all. That is the
  concrete, checkable claim this structure exists to make true.
- Interfaces declared at the point of consumption is idiomatic Go; a `ports/` package of
  interface declarations reads as Java transplanted into Go and would count against the
  repo with a Go reviewer.
- `chi` being `http.Handler`-compatible means none of the above is weakened by using it.

## Alternatives considered

- **Full ports-and-adapters ceremony** (`internal/core/{domain,ports,services}` +
  `internal/adapters/*`) — same dependency direction, more directories, less idiomatic.
- **Flat** (`internal/convert` + `internal/api`) — adequate for a small service, but
  provides nothing to point at when claiming the queue migration is additive.
- **Framework (gin/echo)** — hides the stdlib `http.Handler` contract that makes the
  adapter boundary clean, and adds surface we do not need.
