# ADR-0005: Buffered Conversion and size caps

- **Status**: Accepted
- **Date**: 2026-10-13

## Context

Expected real workload is CSV Documents of **under ~1,000 Rows**. The response is a
single JSON document containing every Record, so we must choose whether to buffer the
result or stream it to the wire as Rows complete. Caps on input size interact with that
choice: buffering is only safe while the bound is small.

## Decision

**Buffer the response.** Build all Records in memory, then write one JSON document.

**Keep the pipeline row-at-a-time anyway.** `Converter.Convert` takes a `RecordSink`
parameter; the HTTP adapter supplies a slice-collecting implementation.

**Caps**, both environment-configurable:

| Cap | Default |
| --- | --- |
| `MAX_BYTES` | 2 MB |
| `MAX_ROWS` | 10,000 |

`MAX_BYTES` is enforced with `http.MaxBytesReader` **before** the multipart parse, so an
oversized body is rejected without being read into memory. Both return `413` stating the
limit. Emit `input_rows` and `input_bytes` as **histograms** plus a counter of `413`s.

## Consequences

- Error handling stays simple and status codes remain available until the last byte, since
  nothing is committed to the wire early. This also makes `on_error: "abort"` (ADR-0004)
  nearly free to add later.
- Memory per in-flight request is bounded by `MAX_BYTES` plus the mapped Records, which at
  these numbers is small. The in-flight semaphore (ADR-0007) bounds the multiple.
- **The 1,000-Row expectation is load-bearing** for this decision, for the caps, and for
  ADR-0009. It is an assumption, not a measurement, which is exactly why `input_rows` and
  `input_bytes` are histograms from day one.
- 10× headroom absorbs a bigger-than-expected file without letting one request allocate
  100× what any legitimate Client sends. The original 10 MB / 100k figures were ~100× the
  real workload and were revised down for that reason.
- Taking `RecordSink` now (~15 lines for the slice implementation) means switching to a
  streaming or asynchronous emitter is a new adapter, not a change to the domain signature.

## Alternatives considered

- **Streamed response** — necessary at 100k Rows, but forces a strict two-phase handler
  (all Request Rejections must be detected before the first byte), forces `summary` to be
  emitted after `data`, and makes error handling materially harder. Unjustified at 1k Rows.
- **Keeping 10 MB / 100k caps with buffering** — 10 MB × concurrency is how the service
  OOMs; a cap 100× above real traffic protects nothing.
- **Returning `[]Record` instead of taking a Sink** — simpler by a few lines, but makes the
  asynchronous path a change to the domain's public signature.
- **Per-Row goroutines** — at 1k Rows the coordination costs more than it saves and it
  makes Conversion Error ordering nondeterministic. Conversion is sequential.
