# ADR 0004 — Authentication and sessions

- **Status:** Accepted
- **Date:** 2026-11-07

## Context

The MVP has a single trusted local administrator, but the data model and authorization boundaries
must support multiple users later. The application is a gateway to remote servers, so session
security and CSRF protection are critical.

## Decision

- Local username/password authentication for the MVP; no external identity provider.
- Passwords hashed with Argon2id using reviewed parameters; never encrypted reversibly.
- Opaque, random session tokens (at least 256 bits). Store only a SHA-256 hash in the database.
- Session cookies: `HttpOnly`, `SameSite=Lax`, `Secure` when served over HTTPS, scoped path.
- Configurable idle and absolute lifetimes. Logout revokes the session server-side.
- CSRF: double-submit token plus `Origin` validation for state-changing cookie-authenticated
  requests.
- Login rate limiting per IP and username; constant-time password verification; generic error
  messages to avoid user enumeration.
- WebSocket upgrades authenticate the session and validate `Origin`.
- Authorization is enforced in backend middleware/handlers, never only in the UI.
- Sensitive account changes require recent authentication.

The schema includes `role` and ownership fields from the start, even though the MVP UI exposes a
single administrator.

## Consequences

- The server is the source of truth for sessions; restart preserves sessions in SQLite.
- A session token leak is bounded by expiry and revocation.
- CSRF tokens must be issued and rotated appropriately.
- Password reset/recovery requires an administrative recovery procedure, not email.

## Alternatives considered

- JWT sessions: harder to revoke and easier to misuse; not needed.
- Cookie-only with SameSite: insufficient as the sole CSRF defense.
- bcrypt: acceptable, but Argon2id is the PRD recommendation and memory-hard.

## References

PRD §4.2, §5.8, §6.3; AGENTS.md.
