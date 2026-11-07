# Architecture Overview

**Status:** Phase 0/1
**Last updated:** 2026-11-07

## 1. System context

```text
+-----------------------------+
| Browser                     |
| React + TypeScript          |
| Workspace, xterm.js, forms  |
+--------------+--------------+
               | HTTPS / WebSocket
+--------------v--------------+
| Drovik backend (Go)         |
| HTTP API, auth, WebSocket   |
| SSH/SFTP, monitoring, audit |
+------+--------------+-------+
       |              |
       |              +------------------+
       |                                 |
+------v------+                   +------v------+
| SQLite      |                   | Secret/key  |
| metadata,   |                   | material    |
| sessions,   |                   | (external)  |
| audit       |                   +-------------+
+-------------+
       |
       | SSH / SFTP
+------v----------------------------------+
| Managed Linux host                      |
| OpenSSH + SFTP subsystem                |
| No Drovik agent                         |
+-----------------------------------------+
```

The browser never connects directly to managed hosts. All SSH credentials and remote operations
stay in the backend.

## 2. Request lifecycle

1. Reverse proxy terminates TLS (production) and forwards to the backend.
2. Middleware assigns a request/correlation ID, recovers panics and sets security headers.
3. Body size and content type are validated.
4. Authentication resolves the session cookie to a server-side session.
5. CSRF and Origin checks run for state-changing requests.
6. Authorization checks the resource and server ownership.
7. The handler validates input and calls a domain service.
8. The service performs bounded work with context cancellation.
9. The result or a typed error is mapped to a stable API response.
10. The access log records safe metadata; secrets are redacted.

## 3. Backend components

| Component | Responsibility |
|---|---|
| `httpapi` | Routes, middleware, handlers, error mapping |
| `auth` | Password hashing, sessions, CSRF, recent-auth checks |
| `users` | Accounts, roles, lifecycle |
| `servers` | Saved connection metadata and ownership |
| `secrets` | AEAD encryption/decryption and key loading |
| `hostkeys` | Known-host storage and verification |
| `sshclient` | Dialing, timeouts, session pool, cleanup |
| `terminal` | PTY sessions and WebSocket streaming |
| `sftp` | Remote file operations and transfers |
| `monitoring` | Bounded metric collection and capability detection |
| `processes` | Structured process inspection and signals |
| `services` | systemd discovery and enumerated actions |
| `logs` | Bounded journal retrieval |
| `audit` | Safe audit events |
| `storage` | Repositories, transactions, migrations |
| `security` | Origin, egress, validation, redaction |
| `observability` | Structured logging, request IDs |
| `app` | Wiring and lifecycle |

Handlers do not construct shell commands, perform encryption or embed permission rules.

## 4. Frontend components

| Area | Responsibility |
|---|---|
| `app` | Bootstrap, routes, providers, error boundaries |
| `api` | Typed client, CSRF, error mapping, cancellation |
| `auth` | Setup, login, session restore, logout |
| `workspace` | Shell, navigation, server context, panels |
| `components` | Design-system primitives |
| Feature folders | Servers, terminal, files, monitoring, processes, services, logs, activity, settings |

The selected server is explicit in every server-scoped request. Stale responses are discarded when
the context changes.

## 5. Data model (Phase 1)

- `users` — id, username, password_hash, role, is_active, timestamps, last_login_at.
- `sessions` — id, token_hash, user_id, created_at, expires_at, revoked_at, last_seen_at.
- `app_settings` — key, value, updated_at.
- `audit_events` — id, actor_user_id, server_id, event_type, target_summary, outcome,
  safe_error_code, correlation_id, created_at.
- `schema_migrations` — migration version bookkeeping.

Later phases add `servers`, `known_hosts` and related entities.

## 6. Security boundaries

- Browser input is untrusted.
- The database is trusted storage but may be stolen; credentials are ciphertext.
- The encryption key is outside the database.
- Managed hosts are untrusted.
- Egress is policy-controlled.
- The interactive terminal is a privileged capability and is clearly scoped to an authenticated,
  authorized server.

## 7. Deployment

- Single Go binary serving the built frontend.
- SQLite on a persistent volume.
- Encryption key mounted separately.
- Reverse proxy for TLS and WebSocket upgrade.
- Distroless/minimal container, non-root, no privileged mode.

See [ADR 0012](../adr/0012-deployment-model.md).

## 8. Phase status

| Phase | Status |
|---|---|
| 0 — foundation | in progress |
| 1 — skeleton + auth | in progress |
| 2–8 | not started |
