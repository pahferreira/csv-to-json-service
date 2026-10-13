# ADR-0002: Mapping language scope

- **Status**: Accepted
- **Date**: 2026-10-13

## Context

The Mapping language is the service's real interface — far more than the HTTP envelope.
Every capability added to it is permanent, because removing one breaks Clients. The MVP
needs to be expressive enough to be useful and small enough to specify exactly.

## Decision

A Mapping contains a `version`, a Dialect, global Transform defaults, and a list of Fields.

**In scope:**

| Capability | Form |
| --- | --- |
| Column addressing | by **header name** only |
| Rename | source Column → Output Path |
| Nesting | dot-separated Output Path (`customer.address.city`) |
| Types | `string`, `int`, `float`, `bool`, `date` |
| Required | per Field |
| Default | per Field |
| Transforms | ordered list, global default + per-Field override |
| Unmapped Columns | **dropped** |

**Transform set** (fixed, named, pure): `trim`, `collapse_spaces`, `lower`, `upper`,
`strip_non_digits`. Implemented as a registry so adding one is a single entry.

**Out of scope, deliberately:**

- **Index-based addressing** and headerless CSVs. `has_header` exists in the Dialect
  struct but `false` is a Request Rejection.
- **Computed Fields** — concatenating two Columns, splitting one into many. This is the
  boundary where a configuration language becomes an expression language.

## Consequences

- One addressing mode means one set of validation rules, one error taxonomy, and
  `fields_unmatched` in Diagnostics stays meaningful (it cannot be meaningful for index
  addressing, where every index "matches").
- Dropping unmapped Columns makes output shape fully determined by the Mapping, which is
  what a downstream consumer needs. The silent-drop risk is mitigated by Diagnostics
  (ADR-0004), not by passthrough.
- Clients needing derived values must do it themselves, before or after the Conversion.
  `errors_total{code}` (ADR-0008) will tell us whether this is actually painful before we
  guess at an expression language.
- `has_header: false` is a one-line change plus an addressing mode when a real Client
  needs it.

## Alternatives considered

- **Both addressing modes now** — doubles the validation surface and the error taxonomy
  for a case no known Client has.
- **Passthrough of unmapped Columns** — makes output shape depend on input shape, so a
  new Column in a source file silently changes every Record.
- **Configurable boolean literals / arbitrary user-supplied transforms** — a knob nobody
  asked for, and user-supplied transforms would mean shipping an expression evaluator.
