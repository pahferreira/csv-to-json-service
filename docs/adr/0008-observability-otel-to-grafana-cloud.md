# ADR-0008: Observability — OpenTelemetry to Grafana Cloud, with PostHog usage events

- **Status**: Accepted
- **Date**: 2026-10-13

## Context

Observability is not decoration here: it is the mechanism by which the
synchronous-vs-queue decision (ADR-0009) gets made from data rather than from intuition.
Deployment target is **Cloud Run**, which scales to zero — so there is no stable instance
for a Prometheus scraper to reach, and metrics from an instance that has already shut down
are simply lost.

## Decision

**Instrument with the OpenTelemetry SDK and push over OTLP.** Grafana Cloud is the single
destination for **logs, metrics and traces**.

**Logs are written twice, deliberately:**

- `slog` JSON to **stdout** — crash-safe, zero configuration, captured free by Cloud Run.
- `otelslog` handler to **Grafana Cloud** — carries `trace_id`, so a slow span links
  directly to that request's logs.

Grafana is the pane actually used; stdout is a safety net for the lines an OTLP buffer
would lose on `SIGKILL`, and for startup errors raised before the exporter exists.

**Traces**: one server span per request with `parse` and `convert` children, accepting
W3C `traceparent` from the Node Client.

**Metrics** (all labels bounded):

| Metric | Type | Purpose |
| --- | --- | --- |
| `request_duration_seconds{outcome}` | histogram | latency, p99 |
| `input_rows` | histogram | **ADR-0009 trigger** |
| `input_bytes` | histogram | **ADR-0009 trigger** |
| `rows_processed_total` / `rows_failed_total` | counter | throughput, data quality |
| `errors_total{code}` | counter | which Mapping/data errors dominate |
| `requests_rejected_total{reason}` | counter | 401 / 413 / 429 / 503 |
| `conversions_in_flight` | gauge | semaphore saturation |

**PostHog** receives one `conversion_completed` usage event per Conversion, carrying
Conversion Stats. It ships **after** the OTel work, via a single
`obs.RecordConversion(ctx, ConversionStats)` call site in the HTTP adapter that fans out
to both sinks. The PostHog send is **fire-and-forget with a bounded buffer**.

**Hard rules:**

- **Metadata only** leaves the process. Row counts, byte counts, durations, error codes,
  Client name, Mapping Field *count*. Never Column names, never cell values, never keys.
- **No unbounded metric labels.** `client`, `code`, `outcome`, `reason` are bounded sets.
  Never label by request ID, Line Number or filename. High-cardinality detail belongs in
  traces and logs.

## Consequences

- Push-based export is the only correct choice for scale-to-zero; a pull-based
  `/metrics` endpoint would silently lose most data.
- Because instrumentation is OTel, the backend is an endpoint and a token in config.
  Changing vendor is an environment-variable change, not a code change — this decision is
  cheaply reversible.
- Grafana Cloud's free tier is bounded by **active metric series**, which is why the
  cardinality rule above is a hard rule and not a style preference.
- PromQL and Grafana dashboards transfer to any Prometheus deployment; a proprietary query
  language would not. A dashboard screenshot is also the single best observability artifact
  for the README.
- Traces are near-worthless today with one hop and one Client — they are built now because
  they are exactly the instrumentation needed on day one of a queue migration, when a
  request stops being one hop.
- PostHog answers "what are Clients actually doing" with ad-hoc queries, which is a
  different question from "is it healthy". Fire-and-forget means a PostHog outage can never
  add latency to or fail a Conversion.
- Writing logs twice is a deliberate, near-free redundancy; it is not an oversight.

## Alternatives considered

- **Google Managed Prometheus with a sidecar collector** — works, but a sidecar for one
  service is operational overhead for no gain.
- **Log-based metrics via Cloud Logging** — cheap, but a real p99 cannot be computed from
  log-derived counters.
- **`/metrics` plus `min-instances=1`** — defeats the reason for choosing Cloud Run.
- **New Relic** — the most generous free tier by raw volume and lower effort to first
  dashboard; rejected because NRQL is proprietary knowledge that does not transfer.
- **Honeycomb** — the best tracing UX of the candidates; rejected because the ADR-0009
  decision is fundamentally about *distributions of metrics*, which is Prometheus-shaped.
- **Datadog** — no meaningful free tier.
- **Google Cloud Monitoring only** — convenient, but custom metric ingest is billed, the
  dashboarding is the weakest of the candidates, and "I used the platform default" is the
  least interesting answer for a project whose stated purpose includes demonstrating
  observability thinking.
- **PostHog as the only backend** — it can compute `quantile(0.95)(rows)` over usage
  events and would genuinely answer ADR-0009, but it has no traces, no latency-SLO
  alerting, and no operational layer (cold starts, restarts, memory).
