# ADR 0012 — Deployment model

- **Status:** Accepted
- **Date:** 2026-11-07

## Context

Drovik is self-hosted. The initial audience runs a small number of servers and expects a simple,
secure installation. The deployment must protect stored credentials and the database, work behind
a reverse proxy, and avoid granting unnecessary host privileges.

## Decision

- **Primary path:** Docker Compose with a single application container serving the Go API and the
  built frontend.
- **Image:** multi-stage build, distroless or minimal runtime, non-root user, no shell required at
  runtime.
- **Persistence:** named volume or bind mount for SQLite and application data.
- **Secret:** encryption key mounted separately from the database volume, never baked into the
  image.
- **Network:** bind to localhost/private interface behind a reverse proxy by default; no privileged
  mode, no host network, no Docker socket mount.
- **Reverse proxy:** Caddy and Nginx examples with TLS, WebSocket upgrade, body-size and timeout
  guidance.
- **Health:** liveness and readiness endpoints; remote SSH availability is not part of readiness.
- **Backups:** documented database + key procedure, restore test, and explicit warning that losing
  the key makes credentials unrecoverable.
- **Native systemd deployment:** later, with a dedicated unprivileged service user.

## Consequences

- One container and one volume is easy to operate.
- Operators must handle TLS and the secret mount correctly; documentation and examples are part of
  the deliverable.
- SQLite limits horizontal scaling; this is acceptable for the target deployment.
- The image must not require cgo or a shell, which is why the pure-Go SQLite driver is used.

## Alternatives considered

- Kubernetes-first: too heavy for the target user.
- Bare-metal binary only: valid later, but Compose gives a reproducible first-run experience.
- Multiple services (separate frontend/backend containers): unnecessary complexity for a
  self-hosted app.

## References

PRD §8, §12; ADR 0003.
