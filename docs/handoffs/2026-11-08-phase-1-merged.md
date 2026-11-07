# 2026-11-08 Phase 1 merged

## Task / Area
Mark Phase 0/1 complete, record the deployed architecture and tee up Phase 2.

## Current State
- `main` contains:
  - `6029223` — repository foundation and governance (#1);
  - `af94b39` — Phase 1 application foundation and authentication (#3).
- `main` ruleset `main-protection` is active with PR-only merges, linear history, no force-push,
  no deletion, no bypass actors and required checks: Backend, Frontend, Secret scan, CodeQL,
  Go vulnerability check, Dependency review, Docs.
- Phase 1 backend: config, structured logging/redaction, panic recovery, SQLite migrations,
  first-run setup, Argon2id, opaque sessions, session-bound CSRF, rate limits, audit events,
  health/readiness, graceful shutdown and a healthcheck subcommand.
- Phase 1 frontend: React 19 + TypeScript + Vite + Tailwind, control-room design system,
  dark/light themes, setup/login/workspace/settings, typed API client, error boundaries and
  sanitized global error handling.
- Deployment assets: distroless Dockerfile, Compose with a separate secret, Caddy/Nginx examples,
  systemd template, OpenAPI 3.1 contract.
- Remote SSH, SFTP, terminal, monitoring, process, service and log features remain unimplemented.

## Decisions
- CSRF uses an HMAC-derived session-bound synchronizer token; there is no readable CSRF cookie.
- Static SPA serving uses `http.FileServer` with explicit extension routing instead of tainted
  filesystem paths.
- Setup is rate-limited per IP (5 attempts / 10 minutes) and must be completed before exposure.
- Required status checks are enforced for administrators.

## Files Touched
- Status/docs: `README.md`, `ROADMAP.md`, `AGENTS.md`, `docs/status/GOAL.md`, this handoff.
- The implementation itself is in the merged PR #3.

## Build / Test Status
- `make verify` on `main`: pass (Go format, vet, race tests, golangci-lint; frontend install,
  lint, typecheck, 11 tests, production build).
- `govulncheck ./...`: no called vulnerabilities.
- `gitleaks detect`: no leaks.
- CI on PR #3: Backend, Frontend, Secret scan, CodeQL, Go vulnerability check, Dependency review
  and Docs all pass.
- Manual browser review: setup, login, workspace, settings, light/dark, logout and login again —
  no console or page errors.
- Not run: Docker/Compose smoke test in this environment (Colima installation did not complete
  before the session ended); Playwright E2E is planned.

## Next Steps
- Start Phase 2 on a new branch: encrypted SSH credential storage, host-key trust, server CRUD,
  connection tests, egress policy and capability detection.
- Add Playwright E2E for setup/login/logout and the first server workflow.
- Verify the Docker image and Compose stack on a machine with a working container runtime.
- Consider a one-time setup token for instances exposed before first-run setup.

## Open Questions
- Add a second maintainer before requiring one approving review.
- Decide when to publish the first tagged release (`v0.1.0` candidate).
