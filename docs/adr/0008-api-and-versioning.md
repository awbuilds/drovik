# ADR 0008 — API and versioning

- **Status:** Accepted
- **Date:** 2026-11-07

## Context

The frontend and backend need a stable contract that can evolve without breaking clients. The PRD
requires versioning from the beginning, defined schemas, stable error codes, bounded payloads and
WebSocket support for terminals.

## Decision

- REST JSON API under `/api/v1`.
- A spec-first OpenAPI 3.1 document is the source of truth for HTTP endpoints.
- Generate Go server types and the TypeScript client from the specification; do not hand-maintain
  duplicate schemas.
- Stable machine-readable error codes with a consistent envelope:
  `{ "error": { "code", "message", "correlation_id", "details" } }`.
- No stack traces or secrets in responses.
- Cursor or page-based pagination for potentially large results.
- Request and response size limits; bounded timeouts; cancellation support.
- Terminal and future streaming features use `/api/v1/servers/{id}/terminal` over WebSocket with
  typed message envelopes.
- Breaking changes require a new API version; additive changes are allowed within `v1`.

## Consequences

- The OpenAPI document and generated code must stay in sync; CI verifies generation is clean.
- Error handling is consistent across the UI and tests.
- API documentation is publishable from the repository.

## Alternatives considered

- Code-first with generated OpenAPI: convenient but allows drift and weaker contract review.
- GraphQL: unnecessary complexity for this product.
- Unversioned API: rejected; breaks self-hosted upgrades.

## References

PRD §5.7, §5.8; AGENTS.md.
