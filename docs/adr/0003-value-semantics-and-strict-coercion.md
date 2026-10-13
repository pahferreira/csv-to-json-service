# ADR-0003: Value semantics and strict Coercion

- **Status**: Accepted
- **Date**: 2026-10-13

## Context

The interaction between empty cells, Defaults, Required and Coercion is the detail most
likely to generate wrong output that nobody notices. Each case needs one explicit rule.

## Decision

**Empty and missing values:**

- A present-but-empty cell (`,,`) yields JSON `null` — not an omitted key, not the type's
  zero value.
- A **Default** fires when the cell is empty **or** the Column is absent.
- **Required** is evaluated **after** Defaults and Transforms have run. An empty or absent
  value at that point is a Row Failure.
- A Mapping naming a Column absent from the CSV Document appears in
  `fields_unmatched`; it is a Request Rejection only if that Field is also Required.

**Coercion is strict.** An unparseable value is a Row Failure. There is no lenient
fallback to string.

**Per type:**

- `date` — a Go layout string per Field, required when `type: date`, plus named aliases
  for common cases. Output always normalised to RFC3339.
- `int` / `float` — honour the Dialect's `decimal_separator` (default `.`) and
  `thousands_separator` (default `,`).
- `bool` — fixed accepted set, case-insensitive: `true`, `false`, `1`, `0`, `yes`, `no`,
  `y`, `n`. Not configurable.

## Consequences

- `null` for empty means consumers never have to distinguish "key absent" from "value
  empty", and a real `0` is never confused with a missing number. The ordering rule makes
  `required` + `default` coherent: the Default satisfies the requirement, which is the
  only reading under which both can be set without contradiction.
- Because `null` is the empty representation, an `empty_to_null` Transform would be
  redundant — it is deliberately **not** in the Transform set (ADR-0002).
- Strict Coercion means a type error is *visible* in `rows_failed` rather than producing
  mixed-type JSON fields that break consumers at some later, harder-to-trace point. This
  is the whole reason the failed-row count exists.
- Locale-aware separators cost almost nothing now and are miserable to retrofit once
  Clients have European files in production.

## Alternatives considered

- **Omit empty keys** — forces existence checks on every consumer.
- **Zero values for empty** — makes `0`, `false` and `""` ambiguous between real data and
  absence. The worst of the three options and the most common default.
- **Lenient Coercion with string fallback** — produces a field that is sometimes a number
  and sometimes a string, which is a silent data-quality bug.
- **Required evaluated before Defaults** — makes `required: true` with a `default` an
  unsatisfiable contradiction.
