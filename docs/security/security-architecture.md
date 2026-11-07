# Security Architecture

**Status:** living document
**Last updated:** 2026-11-07

This document describes how Drovik implements its security controls. The threat model explains
what is being defended; this document explains how.

## 1. Security principles

1. **Least privilege.** Act as the configured SSH user. Never auto-escalate.
2. **Fail closed.** Authentication, authorization, host-key, path and egress checks fail closed.
3. **No false success.** Mutations are verified by re-reading remote state where possible.
4. **Defense in depth.** Backend authorization is authoritative; UI controls are convenience.
5. **Minimal data.** Do not retain remote files, terminal I/O or logs by default.
6. **No secrets in logs.** Redact at the logging boundary.
7. **Bounded work.** Timeouts, size limits, concurrency limits and cancellation everywhere.
8. **Open source, no telemetry.** Self-hosted operators own their data.

## 2. Request security pipeline

Every HTTP request passes through:

1. TLS termination at the reverse proxy (production) or explicit local development.
2. Request ID and correlation ID.
3. Panic recovery.
4. Security headers (CSP, frame-ancestors, nosniff, referrer policy, permissions policy).
5. Body size limit.
6. Content-type validation for JSON.
7. Authentication (session cookie or explicit setup route).
8. CSRF validation for state-changing methods.
9. Authorization for the resource.
10. Input validation and normalization.
11. Handler execution with a bounded context.
12. Error mapping and safe response.
13. Structured access log with no secrets.

## 3. Authentication

- Local accounts only in the MVP.
- Argon2id with per-password salt, reviewed memory/time/parallelism parameters, and a versioned
  hash encoding so parameters can be upgraded.
- Password verification uses the hash library's constant-time comparison.
- Login responses are generic; failures are rate-limited by IP and username.
- Successful login creates an opaque 256-bit session token. Only its SHA-256 hash is stored.
- Cookies: `HttpOnly`, `SameSite=Lax`, `Secure` when HTTPS, `Path=/`, no domain unless required.
- Idle and absolute expiry are enforced server-side. Logout revokes the session.
- Password changes revoke other sessions.

## 4. CSRF

Cookie-authenticated state-changing requests require:

- A CSRF token in a non-HttpOnly cookie, matched against an `X-CSRF-Token` header.
- `Origin`/`Sec-Fetch-Site` validation against trusted origins.
- `SameSite=Lax` as an additional layer.

The token is rotated on login and logout. GET/HEAD/OPTIONS requests do not require a CSRF token but
must not mutate state.

## 5. Secrets

- SSH credentials are encrypted with AES-256-GCM using a random nonce and associated data binding
  the record identity.
- The 32-byte key comes from `DROVIK_ENCRYPTION_KEY` or `DROVIK_ENCRYPTION_KEY_FILE`; it is never
  stored in SQLite, the repository or the image.
- Decryption happens only immediately before an SSH connection.
- Secrets are never returned to the browser after submission.
- Logging and error mapping redact secret-like fields.
- Backups must include the database and key separately; losing the key is unrecoverable.

## 6. SSH and SFTP

- Strict host-key verification via a custom callback; no insecure fallback.
- Unknown keys require explicit approval; changed keys block the connection.
- Host keys are stored per server identity with approval metadata.
- Dial, handshake, auth and operation timeouts are bounded.
- Sessions and channels are tracked and closed; pools are bounded.
- Agent forwarding and port forwarding are disabled in the MVP.
- Structured operations only; the terminal is the sole arbitrary command path.
- SFTP paths are normalized and validated against traversal and symlink escapes.

## 7. WebSockets

- Authenticate during upgrade; reject missing/expired sessions.
- Validate `Origin` against the trusted origin list.
- Authorize the specific server/resource.
- Enforce message size and rate limits.
- Use typed envelopes for input, resize, output, status and close.
- Isolate sessions so output cannot cross users or servers.
- Clean up on close, logout, timeout, disconnect and shutdown.
- Never put long-lived credentials in the WebSocket URL.

## 8. Browser security

- Treat filenames, hostnames, process names, logs and terminal output as untrusted text.
- No `dangerouslySetInnerHTML` for remote content.
- CSP restricts scripts, frames and connections; no inline scripts in production.
- `X-Content-Type-Options: nosniff`, `Referrer-Policy`, `Permissions-Policy`.
- Downloads use safe `Content-Disposition`; previews are bounded and non-executable.
- Sensitive values are never stored in `localStorage`/`sessionStorage`.

## 9. Egress policy

- Default allow: public and RFC1918.
- Default deny: loopback, link-local, multicast, unspecified, reserved and cloud metadata.
- Resolve once, validate, connect to the validated address, revalidate every attempt.
- Rate-limit outbound attempts and cap concurrent destinations.
- Log attempts as metadata without credentials.
- Operator-configurable allow/deny rules for multi-user or hardened deployments.

## 10. Logging and observability

- Structured JSON in production; text in development.
- Fields: timestamp, level, event, code, correlation ID, safe operation context.
- Redact authorization headers, cookies, passwords, keys, tokens, passphrases, file contents,
  terminal I/O and log contents.
- Expected client errors at `warn`; unexpected/5xx/panic at `error` with stack.
- No external telemetry, analytics or crash reporting by default.
- Debug logging is opt-in and may still never include secrets.

## 11. Database

- Parameterized queries only.
- WAL mode, foreign keys, busy timeout.
- Least-privilege file permissions on the data directory and database.
- Migrations are versioned and applied before serving.
- Backups are treated as sensitive.

## 12. Supply chain

- Dependencies pinned; lockfiles committed.
- GitHub Actions pinned to full commit SHAs.
- Dependabot, CodeQL, gitleaks, govulncheck and dependency review in CI.
- Releases built from protected tags in CI.
- Checksums and SBOMs planned before stable release.

## 13. Deployment

- Docker: distroless/minimal, non-root, no privileged mode, no host network, no Docker socket.
- Secret mounted separately from the database volume.
- Bind to localhost/private interface behind a reverse proxy by default.
- HTTPS required in production.
- Health endpoints do not disclose secrets, host lists or internal paths.
- Graceful shutdown closes resources in a bounded window.

## 14. Security testing

- Unit tests for hashing, sessions, CSRF, redaction and validation.
- API tests for authentication, authorization, rate limits and error envelopes.
- Crash-safety tests for panic recovery.
- Integration tests with disposable SSH/SFTP containers in later phases.
- Targeted security tests per the PRD §6.7 before beta.
