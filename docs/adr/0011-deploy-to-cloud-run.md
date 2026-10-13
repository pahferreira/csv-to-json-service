# ADR-0011: Deploy to Cloud Run

- **Status**: Accepted
- **Date**: 2026-10-13

## Context

The service must be publicly reachable over HTTPS (ADR-0007), deployed as part of the MVP
rather than afterwards, cheap to run while traffic is negligible, and credible as a
portfolio artifact. It is a single stateless container with no persistent storage.

## Decision

**Google Cloud Run**, from a multi-stage distroless container image.

- Scale to zero; `max-instances` set deliberately low so the per-instance rate limiter in
  ADR-0007 has a knowable aggregate bound.
- TLS terminated at the platform edge.
- `PORT` read from the environment, as Cloud Run requires.
- API keys injected from **Secret Manager** as environment variables at deploy time,
  fetched once at startup. No periodic refresh.
- Container concurrency set as a second enforcement point for the in-flight bound.

**Operational endpoints and server hardening**, all required by this target:

- `GET /healthz` — liveness, returns 200 unconditionally.
- `GET /readyz` — readiness, checks config loaded.
- Both unauthenticated, excluded from rate limiting, and **excluded from request logging**.
- Explicit `http.Server` timeouts: `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`,
  `IdleTimeout`.
- Graceful shutdown on `SIGTERM`, draining in-flight Conversions with a **5-second**
  deadline.

Config is a typed struct populated from environment variables and validated at startup,
exiting non-zero with a clear message on anything missing or nonsensical.

## Consequences

- Cloud Run sends `SIGTERM` then `SIGKILL`s after a grace period that **defaults to 10
  seconds**. The 5-second drain deadline sits comfortably inside it; a longer deadline
  would make the drain code decorative.
- Scale to zero means an idle portfolio service costs approximately nothing — but it is
  also why metrics must be **pushed**, not scraped (ADR-0008).
- `ReadHeaderTimeout` is absent by default in Go and its absence is a genuine slowloris
  vulnerability on a public endpoint. It is set explicitly for that reason, not for tidiness.
- Health endpoints would otherwise be the large majority of log volume, hence the logging
  exclusion.
- Fetching secrets once at startup means key rotation requires a redeploy. Acceptable: a
  redeploy here takes well under two minutes, and periodic refresh is for certificates.
- A service refusing to boot with no API keys configured is strictly better than one that
  boots and silently accepts nothing.
- Nothing in the application code is Cloud Run-specific beyond reading `PORT`, so the
  platform is replaceable.

## Alternatives considered

- **Fly.io** — comparable fit, private networking via 6PN; Cloud Run chosen for
  scale-to-zero economics and tighter Secret Manager integration.
- **Render / Railway** — simplest deploy UX, less control over concurrency and instance
  bounds, which ADR-0007 relies on.
- **ECS Fargate** — the most "enterprise" line on a CV and the most YAML per unit of
  value; no scale-to-zero.
- **VPS with Docker Compose + Caddy** — cheapest, but hand-rolled TLS, deploys and
  restarts for no architectural gain.
