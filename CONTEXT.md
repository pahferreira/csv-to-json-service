# CONTEXT

Glossary and domain model for `csv-to-json-service`.

This service takes a CSV document and a caller-supplied mapping configuration, and
returns JSON records plus a report of what failed. It stores nothing.

Decisions behind this model are recorded in `docs/adr/`.

---

## The conversion model

A single **Conversion** is the unit of work behind one request:

```
            Dialect                    Mapping
               │                           │
               ▼                           ▼
 CSV bytes ─► RowSource ─► Row ─► Transform ─► Coercion ─► Record ─► RecordSink
                                     │            │
                                     └────────────┴──► Row Failure (no Record)
```

Every Row produces **either** a Record **or** a Row Failure, never both and never
neither. That invariant is what makes `rows_total = rows_succeeded + rows_failed`
hold, and it is the property the test suite asserts hardest.

A Conversion yields four things: the **Records**, the **Diagnostics**, the
**Summary**, and the **Conversion Errors**.

---

## Glossary

Use these terms verbatim in code, tests, issue titles, metric names and error codes.

### Input

**CSV Document** — the uploaded file, as bytes. Never persisted.

**Dialect** — how bytes become Rows: delimiter, whether a Header Row is present,
decimal separator, thousands separator. Supplied by the caller alongside the Mapping.
Concerns *reading*, never *meaning*.

**Row** — one raw CSV record as `[]string`, carrying its 1-based **Line Number** (the
physical line in the file, so it survives quoted fields containing newlines).

**Header Row** — the first Row when the Dialect says one is present. It names the
Columns. Required in the MVP (ADR-0002).

**Column** — a named source field in the CSV Document. Addressed by name only;
duplicate names are a Request Rejection (ADR-0010).

### Configuration

**Mapping** — the caller-supplied configuration document describing how to turn Rows
into Records. Versioned (`version: 1`), self-contained, and inline in the request.
It is a value object: parsing and validating it involves no I/O and no HTTP.

**Field** — one entry in a Mapping: which Column to read, the **Output Path** to write,
the target **Type**, whether it is **Required**, its **Default**, and its **Transforms**.

**Output Path** — dot-separated location of a value inside a Record
(`customer.address.city`). Creates intermediate objects as needed.

**Transform** — a named, pure, ordered `string → string` operation
(`trim`, `collapse_spaces`, `lower`, `upper`, `strip_non_digits`). Declared globally as
Mapping defaults and overridable per Field. Transforms run *before* Coercion.

**Coercion** — parsing a transformed string into a typed JSON value (`string`, `int`,
`float`, `bool`, `date`). Strict: an unparseable value is a Row Failure, never a
silent fallback to string (ADR-0003).

### Output

**Record** — one successfully converted Row: a nested JSON object shaped by the Output
Paths. Columns absent from the Mapping are dropped.

**Row Failure** — a Row that produced no Record. Causes: malformed CSV row, failed
Coercion, or a Required Field that was empty or absent after Defaults and Transforms.

**Conversion Error** — the reported form of a Row Failure: `line`, `column`, `code`,
`message`, and the `raw` Row. Emitted into a capped list, never as an HTTP error.

**Diagnostics** — the configuration-level report, sized by *columns* not rows:
`headers_seen`, `columns_mapped`, `columns_dropped`, `fields_unmatched`.
`fields_unmatched` is the one that catches a mistyped Column name, which would
otherwise silently produce a missing key in every Record.

**Summary** — `rows_total`, `rows_succeeded`, `rows_failed`.

**Request Rejection** — a failure of the request *itself* rather than of its data:
unparseable Mapping, unreadable CSV Document, missing Header Row, duplicate Column
names, non-UTF-8 encoding, oversized body, bad credentials, rate limit. Always a 4xx,
always distinct from a Row Failure (ADR-0004).

### Structure

**RowSource** — port: yields Rows one at a time. Implemented over `encoding/csv` today;
over object storage or a queue message if the service moves asynchronous (ADR-0009).

**RecordSink** — port: receives Records one at a time. Implemented as a slice collector
for the HTTP response today; as a streaming writer later. Taken as a parameter now so
the asynchronous path adds an adapter rather than changing the domain (ADR-0005, ADR-0006).

**Converter** — the domain service owning the Conversion. Knows about RowSource,
RecordSink, Mapping and Dialect. Knows nothing about HTTP, multipart, chi or telemetry.

### Operations

**Client** — an authenticated caller, identified by a *named* API key. The name appears
in logs, metrics and usage events; the key never does (ADR-0007).

**Conversion Stats** — the metadata emitted per Conversion for telemetry: row counts,
byte count, duration, failure count, error codes, Client name. Metadata only — Column
names and cell values never leave the process (ADR-0008).

---

## Vocabulary to avoid

| Don't say | Say | Why |
| --- | --- | --- |
| "parse" (unqualified) | **read** (bytes → Row) or **coerce** (string → typed value) | Two distinct operations with different failure modes |
| "validate" (for data) | **Coercion** / **Row Failure** | Reserve *validate* for checking a Mapping |
| "sanitize" | **Transform** | Sanitize implies a security guarantee this service does not make |
| "record" (for input) | **Row** | Row is input, Record is output |
| "line" (for a Record) | **Row** or **Line Number** | Keeps the input/output split clean |
| "job" | **Conversion** | Reserve *job* for the asynchronous path, if it ever exists |

---

## Explicit non-goals (MVP)

Each is a deliberate scope decision, not an oversight:

- **Headerless CSVs** and index-based Column addressing (ADR-0002)
- **Computed Fields** — concatenating or splitting Columns (ADR-0002)
- **Stored Mappings** addressed by ID (ADR-0001)
- **Abort-on-error mode** — `rows_failed` is always a partial success (ADR-0004)
- **Preview / dry-run mode** (ADR-0004)
- **Formula-injection neutralisation** — out of scope by decision, documented (ADR-0010)
- **Asynchronous processing via queues** — gated on measured thresholds (ADR-0009)
