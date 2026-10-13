# ADR-0004: Failure semantics and response contract

- **Status**: Accepted
- **Date**: 2026-10-13

## Context

A CSV Document is partially-trusted bulk data: some Rows will be bad. The contract must
distinguish "your request was wrong" from "some of your data was wrong", and must report
enough detail to be actionable without letting a garbage file produce a garbage-sized
response.

## Decision

**Row Failures are isolated.** One bad cell fails only its Row; every other Row still
produces a Record. Bad data **never** produces a 4xx.

**Success response** — `200`:

```json
{
  "data": [ { } ],
  "diagnostics": {
    "headers_seen": [], "columns_mapped": [],
    "columns_dropped": [], "fields_unmatched": []
  },
  "summary": { "rows_total": 0, "rows_succeeded": 0, "rows_failed": 0 },
  "errors": [ { "line": 0, "column": "", "code": "", "message": "", "raw": [] } ],
  "errors_truncated": false
}
```

`errors` is capped at the first **100** Conversion Errors, with `errors_truncated`
flagging the cap. Every Conversion Error carries a machine-readable `code`.

**Diagnostics are always on**, and are sized by columns rather than rows.
`raw` (the original Row) is attached to Conversion Errors only — never to successful Records.

**Request Rejections** — 4xx, in a custom envelope consistent with the row-level shape:

```json
{ "error": { "code": "", "message": "", "details": {} } }
```

Every `code` at both levels is documented in the OpenAPI spec.

**Out of scope:** `on_error: "abort"` and preview/dry-run mode. The field is **not
shipped at all** rather than shipped accepting one value; error handling sits behind an
internal policy interface so adding it touches one package.

## Consequences

- Clients get one error vocabulary and one error-handling path for both levels — the
  reason for a custom envelope rather than RFC 9457.
- The 100-error cap bounds response size for an adversarial file while keeping the common
  case (a handful of bad rows) fully actionable.
- Diagnostics answer "what was parsed and what was dropped" at O(columns) instead of the
  O(rows) cost of echoing every Row, and `fields_unmatched` specifically catches a
  mistyped Column name — the failure mode that dropping unmapped Columns would otherwise
  hide completely.
- `raw` on errors means Conversion Errors carry cell values, so error payloads are as
  sensitive as the input. They must never be logged (ADR-0008).
- A Client wanting all-or-nothing must currently check `rows_failed` itself and discard.

## Alternatives considered

- **Raw echo of every Row** (the original proposal) — doubles payload and allocation on
  every request, duplicates data the Client just sent, and puts a full copy of possibly
  personal data into every response and anything that logs one.
- **Count only, no error detail** — near-useless to a Client trying to fix their file.
- **Unbounded error list** — a 10k-row garbage file produces a 10k-entry array.
- **RFC 9457 `application/problem+json`** — the better choice for a public API with
  unknown third-party consumers, rejected here because consistency with the row-level
  error shape matters more for a single known Client, and `type` as a URI reference would
  be invented ceremony.
- **4xx for a high Row Failure rate** — conflates the two failure classes, which is
  exactly what this ADR exists to prevent.
