# 2026-11-07 Phase 1 implementation

## Task / Area
Implement the Phase 1 application skeleton and authentication: Go backend, SQLite schema,
first-run setup, login/logout/session/CSRF, React design system and auth/workspace screens,
Docker/Compose deployment assets and CI.

## Current State
- Backend implemented under `backend/`:
  - config validation with a required 32-byte encryption key;
  - structured logging with secret redaction;
  - chi HTTP server with request IDs, panic recovery, access logs, security headers, CORS,
    body limits, Origin checks, login rate limiting and graceful shutdown;
  - SQLite via `modernc.org/sqlite`, WAL, foreign keys, embedded goose migrations;
  - Argon2id passwords, opaque sessions with SHA-256 token hashes, idle/absolute expiry;
  - first-run setup, login, logout, `/auth/me`, `/version`, `/healthz`, `/readyz`;
  - CSRF double-submit + Origin checks, audit events, session janitor.
- Frontend implemented under `frontend/`:
  - React 19 + TypeScript 7 strict + Vite 8 + Tailwind CSS 4;
  - deliberate dark/light “control room” design tokens, self-hosted fonts, Radix primitives;
  - setup wizard, login, session restore, app shell, command center, settings, 404;
  - typed API client, auth provider, error boundaries, global error handlers, toasts;
  - Vitest/Testing Library tests.
- Deployment:
  - distroless non-root Dockerfile, healthcheck subcommand;
  - Compose with a separate secret, read-only rootfs, no capabilities;
  - Caddy/Nginx examples, systemd unit template, backup/upgrade guidance.
- CI/security workflows pinned to commit SHAs; gitleaks CLI instead of the org-licensed action.

## Decisions
- Rune/OpenCode design language in React; Rune framework not used (ADR 0001/0009).
- `modernc.org/sqlite` pure Go driver; goose migrations.
- Argon2id with 64 MiB / 3 iterations / 2 lanes; opaque sessions; hashed token storage.
- CSRF double-submit cookie plus Origin/Sec-Fetch-Site checks.
- Egress defaults and SSH trust are documented but not implemented until Phase 2.

## Files Touched
- `backend/` — application, packages, migrations, tests, lint config, Go module.
- `frontend/` — application, design system, tests, build config.
- `deploy/` — Dockerfile, Compose, Caddy, Nginx, systemd, deployment README.
- `.github/workflows/` — CI and security workflows.
- `docs/product/traceability.md`, `docs/status/GOAL.md`, `.env.example`, `README.md`.

## Build / Test Status
- `make verify` passes:
  - `gofmt`, `go vet`, `go test -race ./...`, `golangci-lint run` (0 issues);
  - `pnpm install --frozen-lockfile`, `pnpm lint`, `pnpm typecheck`, `pnpm test` (11 tests),
    `pnpm build`.
- `govulncheck ./...`: no called vulnerabilities.
- `gitleaks detect`: no leaks across the local history.
- Not run: Docker image build/Compose smoke test (Docker is not installed in this environment);
  browser E2E suite; SSH/SFTP integration tests (later phases).
- UI manually reviewed with the browse tool at desktop width; no console or page errors.

## Next Steps
1. Merge the foundation PR, then rebase this branch onto updated `main`.
2. Add required status checks to the `main` ruleset once CI names are registered.
3. Run the Docker/Compose smoke test on a machine with Docker.
4. Start Phase 2: encrypted SSH credentials, host-key trust and server CRUD.

## Open Questions
- Add a second maintainer before requiring 1 approving review.
- Add a one-time setup token for instances exposed before first-run setup is complete.
