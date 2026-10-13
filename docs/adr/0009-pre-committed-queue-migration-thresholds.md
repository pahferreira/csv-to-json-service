# ADR-0009: Pre-committed thresholds for moving to queues

- **Status**: Accepted
- **Date**: 2026-10-13

## Context

The service may eventually need asynchronous processing — accept a CSV Document, return a
job ID, process off a queue, deliver results elsewhere. Whether it _does_ is meant to be a
data-driven decision, which is the stated reason for investing in observability.

The failure mode this ADR exists to prevent: "data-driven" quietly degrading into
"someone looked at a dashboard and felt uneasy". A threshold chosen _after_ seeing the
data is a rationalisation, not a decision.

## Decision

**Write the thresholds down now, before the data exists.** Move to an asynchronous
queue-backed design when **any one** of these trips, sustained over a 7-day window:

| Signal                                  | Threshold                            | Why this one                                                                      |
| --------------------------------------- | ------------------------------------ | --------------------------------------------------------------------------------- |
| `input_rows` p95                        | **> 5,000 Rows**                     | Half of `MAX_ROWS`; the buffering assumption in ADR-0005 is no longer comfortable |
| `requests_rejected_total{reason="413"}` | **> 1%** of authenticated requests   | Clients are actively being turned away by the caps                                |
| `request_duration_seconds` p99          | **> 10 seconds**                     | A synchronous HTTP call this slow is a poor contract for the caller regardless of their timeout |
| `requests_rejected_total{reason="503"}` | **> 0.1%** of authenticated requests | The in-flight semaphore is saturating; sync capacity is the bottleneck            |

The p99 threshold is **absolute**, not a fraction of the Node Client's HTTP timeout. If
that timeout is ever found to be at or below 20s, this threshold gives the caller no
headroom and must be lowered to sit under it.

If a threshold trips, the migration shape is already determined by ADR-0006: replace the
`RowSource` with one reading from object storage, replace the `RecordSink` with one
writing results to storage, and add a queue-consumer adapter. The Converter does not change.

Raising a cap is an acceptable _interim_ response to the 413 threshold, but raising it
above 5 MB without moving to queues reopens ADR-0005's buffering decision.

## Consequences

- The decision becomes falsifiable: either a threshold trips or it does not. The
  thresholds can be revised, but revising them is itself a visible, dated act.
- `input_rows` and `input_bytes` histograms are therefore **required from day one**, not
  a later nicety. They are what make this ADR executable rather than aspirational.
- `errors_total{code}` is deliberately **not** a trigger. It informs the Mapping-language
  roadmap (computed Fields, index addressing, abort mode) — it says nothing about whether
  synchronous processing is adequate.
- Three of the four triggers can only fire if the service has real traffic. Until it does,
  the correct conclusion is "no evidence", not "queues are needed".

## Alternatives considered

- **Collect first, decide later** — the default path, and the one that reliably produces a
  decision made on vibes months later.
- **Build queues now** — unjustified at ~1,000 Rows per Conversion; it would add a broker,
  a result store, a job-status endpoint and polling semantics to serve one Client.
- **Trigger on a single composite SLO** — harder to reason about and hides which of four
  distinct pressures actually caused the change.
