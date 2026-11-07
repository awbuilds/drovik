# ADR 0003 — SQLite and migrations

- **Status:** Accepted
- **Date:** 2026-11-07

## Context

Drovik's initial deployment is a single self-hosted instance managing roughly 5–20 servers. The
data is metadata, sessions, settings, audit events and encrypted credentials. The PRD recommends
SQLite for the initial deployment while keeping the persistence layer replaceable.

## Decision

- Use SQLite in WAL mode with foreign keys enabled and a bounded busy timeout.
- Use the pure-Go `modernc.org/sqlite` driver to avoid cgo and simplify cross-compilation and
  distroless images.
- Use embedded, versioned migrations executed at startup before serving traffic.
- Keep SQL behind repository interfaces; handlers never issue SQL directly.
- Store timestamps in UTC; display local time in the UI.
- Use parameterized queries only.

## Consequences

- One file to back up, simple local deployment, no external database service.
- Writes are serialized; long transactions must be avoided. Audit and session writes must be
  short.
- A future PostgreSQL migration is possible but requires SQL dialect review and a migration plan.
- The encryption key is not stored in the database (see ADR 0005).

## Alternatives considered

- PostgreSQL: operationally heavier for the target audience and unnecessary for the MVP.
- `mattn/go-sqlite3`: requires cgo, complicates static builds.
- No migrations framework: rejected; schema drift is unacceptable.

## References

PRD §5.6, §8.7.
