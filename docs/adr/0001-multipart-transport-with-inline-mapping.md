# ADR-0001: Multipart transport with an inline Mapping

- **Status**: Accepted
- **Date**: 2026-10-13

## Context

A Conversion needs two inputs in one request: a CSV Document (bytes, potentially
megabytes) and a Mapping (a structured document). HTTP gives us several ways to carry
both, and the choice constrains memory behaviour and the later asynchronous path.

## Decision

`POST /v1/convert` accepts `multipart/form-data` with two parts:

- `file` — the CSV Document
- `mapping` — a JSON document (see ADR-0002)

The Mapping is **inline on every request**. There is no server-side Mapping store and
no Mapping ID in the MVP. The Mapping is nonetheless modelled as a self-contained,
versioned value object (`version: 1`) parsed by its own package with no HTTP knowledge.

## Consequences

- Bytes stay out of a JSON parser; no base64 inflation (~33%).
- The service remains fully stateless, which keeps deployment and scaling trivial.
- Callers re-send the Mapping on every request. Acceptable with one known Client; it is
  the first thing that will hurt if Mappings grow large or Clients multiply.
- Because the Mapping is a standalone versioned value object, adding a Mapping store and
  a `mapping_id` reference later is **additive** — a new adapter plus a resolution step,
  with no change to the Converter.
- The `version` field exists from day one, so a breaking change to the Mapping language
  does not require a new endpoint.

## Alternatives considered

- **JSON body with base64 CSV** — inflates payloads, forces the whole document through
  a JSON parser, and makes streaming impossible if we ever want it.
- **Raw `text/csv` body with the Mapping in a header or query param** — headers are the
  wrong place for a nested document and impose length limits.
- **Two-step: upload, then convert by reference** — requires server-side state and two
  endpoints, contradicting the single-endpoint MVP. This is the shape to revisit together
  with ADR-0009.
