# ADR-0010: Dialect handling and rejected inputs

- **Status**: Accepted
- **Date**: 2026-10-13

## Context

"CSV" is a family of formats, not a format. Files arrive from Excel in various locales and
encodings. We must decide how much variation to absorb and, more importantly, which inputs
to **reject** rather than guess at.

## Decision

**Configurable Dialect**, supplied with the Mapping:

| Setting | Default |
| --- | --- |
| `delimiter` | `,` |
| `has_header` | `true` (`false` is a Request Rejection — ADR-0002) |
| `decimal_separator` | `.` |
| `thousands_separator` | `,` |

**Accepted and normalised:**

- A leading UTF-8 BOM is stripped.
- CRLF and LF line endings both.
- Quoted fields containing delimiters and newlines (Line Number tracks physical lines).

**Rejected as Request Rejections (4xx), never guessed at:**

- **Duplicate Column names.** A file with two Columns named `email` makes the Mapping
  ambiguous and only the Client can say which was meant. Code: `duplicate_header`.
- **Non-UTF-8 encoding.** Rejected with a clear code rather than mangled into replacement
  characters.
- **Missing Header Row** (empty file, or `has_header: false`).

**Ragged Rows** — a Row with a different field count than the Header Row — are a **Row
Failure**, not a Request Rejection. One malformed line should not discard a good file.

**Encoding sniffing is not performed.** The Dialect is declared, never inferred.

**Formula injection is out of scope.** A cell such as `=cmd|'/c calc'!A1` is passed
through as a string. Output is JSON, where it is inert; the risk exists only if a
downstream consumer re-exports to CSV or Excel. This is documented in the README as the
consumer's responsibility. A `neutralize_formula` Transform is a possible opt-in follow-up.

## Consequences

- Rejecting duplicate headers is detectable while reading the Header Row, before any
  Records exist, so it is cheap. The alternative — first-wins or last-wins — is a silent
  ambiguity that surfaces months later as "the wrong email address was used".
- Declared-not-sniffed means a Client with an unusual file gets a clear error and a knob,
  rather than output that is subtly wrong in a way nobody notices.
- Locale separators in the Dialect (rather than per Field) match how real files vary: a
  file is European or it is not; it is not European in column 3 only.
- Scoping formula injection out **consciously and in writing** is materially different
  from not having considered it. Always-neutralising would corrupt legitimate data —
  plenty of real cells begin with `-` or `+`.
- The `testdata/` corpus must contain one file per rejected and normalised case above.

## Alternatives considered

- **Strict single dialect** (UTF-8, comma, quote, header) — simpler, but fails on the
  first semicolon-delimited export from a European Excel.
- **Automatic dialect sniffing** — guesses wrong on exactly the files you would least like
  it to, and the wrong guess is silent.
- **First-wins / last-wins on duplicate headers** — silent ambiguity in a data pipeline.
- **Transcoding non-UTF-8 input** — requires guessing the source encoding; a clear
  rejection is more honest.
- **Always neutralise formulas** — silently corrupts valid data.
- **Ragged Rows as a Request Rejection** — one bad line would discard an otherwise good
  file, contradicting the isolation principle in ADR-0004.
