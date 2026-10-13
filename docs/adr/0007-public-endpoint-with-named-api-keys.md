# ADR-0007: Public endpoint with named API keys and abuse controls

- **Status**: Accepted
- **Date**: 2026-10-13

## Context

The one known Client is a Node API the owner also operates, inside a VPC the owner
controls. The service stores nothing, which genuinely limits breach impact. But the
service is also a portfolio artifact, and a documented, reachable, authenticated API
demonstrates more than a VPC-internal one.

Two risks survive "nothing is stored": CSV Documents **transit** the service and are
overwhelmingly personal data; and an endpoint that does CPU and memory work on an
uploaded file is a **compute amplifier** for anyone who finds the URL.

## Decision

**Public endpoint, authentication mandatory.** Being public makes the following
non-optional rather than nice-to-have:

- **Named API keys.** Config carries a list of `name:key` pairs. Comparison is
  constant-time (`crypto/subtle`). `401` on failure, with no hint as to which part failed.
- **Per-key token bucket** (`golang.org/x/time/rate`), one limiter per key, built into a
  `map[string]*rate.Limiter` at startup from the configured keys. `Allow()`, never
  `Wait()`. `429` with `Retry-After`.
- **Global in-flight semaphore**, bounding concurrent Conversions. `503` when saturated.
- **Size caps enforced before the body is read** (ADR-0005).
- **TLS at the platform edge** (ADR-0011).
- **Cell values, Column names and API keys never appear in logs** — not even truncated.

Rejections are counted in `requests_rejected_total{reason}`.

## Consequences

- Rate limiting and the in-flight semaphore do different jobs and we need both: the
  limiter caps *arrival rate*, the semaphore caps *concurrent expensive work*. A Client at
  3 req/s of 2 MB files passes the limiter comfortably while potentially holding 30
  Conversions in flight. The semaphore is what turns that from an OOM into a `503`.
- The in-process limiter is **per-instance**: N instances means N× the configured rate.
  The mitigation is a low `max-instances` so the bound is knowable, not a distributed
  limiter — Redis for one Client is unjustifiable.
- Named keys cost nothing extra (the limiter map wants a key anyway) and make rotation
  possible with no downtime: add the new key, cut the Client over, remove the old one.
  Names also give logs, metrics and usage events a `client` label that is not the secret.
- `429`s must be counted, or we will never know whether our own Node API is being
  throttled in production.
- Being public means Conversion Error payloads (which carry `raw` Rows, per ADR-0004)
  cross the internet. TLS is therefore mandatory, not optional.

## Alternatives considered

- **VPC-internal only** — strictly safer and architecturally simpler, rejected because a
  reachable, documented API is a materially better portfolio artifact.
- **Public and unauthenticated** — "no sensitive data stored" is no defence against a
  compute-exhaustion bill or an availability incident.
- **HMAC request signing** — protects against replay and body tampering, neither of which
  is the threat model over TLS with one trusted Client.
- **OAuth2 client-credentials / JWT** — an authorization-server dependency for a single
  machine Client.
- **Platform-level rate limiting (Cloudflare, Cloud Armor)** — the right answer if real
  abuse ever appears; premature now.
- **Embedding the Converter as a Go library in the Node product** — impossible (the Client
  is Node), and it would eliminate the observability and queue-migration story entirely.
