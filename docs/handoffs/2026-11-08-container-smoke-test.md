# 2026-11-08 Container smoke test

## Task / Area
Verify the Phase 1 container image and runtime security constraints with a real Docker daemon.

## Current State
- `docker build -f deploy/docker/Dockerfile -t drovik:test .` succeeds from the repository root.
- The final image is distroless, runs as `nonroot:nonroot`, and starts with:
  - read-only root filesystem,
  - `--tmpfs /tmp`,
  - `--security-opt no-new-privileges`,
  - `--cap-drop ALL`,
  - a named `/data` volume and a read-only secret mount.
- `/drovik healthcheck` exits 0; `/healthz`, `/api/v1/setup/status` and the static root return 200.
- First-run setup, `/auth/me`, logout without CSRF (403), logout with CSRF (204), login (200) all
  work inside the container.
- After `docker restart`, setup remains complete and login still succeeds (SQLite volume
  persistence).
- Container logs are JSON and contain no credentials or tokens.

## Decisions
- The smoke test used development HTTP settings inside the container (`DROVIK_ENV=development`,
  `DROVIK_COOKIE_SECURE=false`, `DROVIK_PUBLIC_URL=http://localhost:8099`) so the HTTP-only test
  client could exercise cookies. Production Compose uses HTTPS, `DROVIK_ENV=production` and
  `DROVIK_COOKIE_SECURE=true`.
- A bind-mounted secret file had to be readable by the distroless user for this manual test;
  Compose manages the secret mount, so the documented path remains the production route.

## Files Touched
- No code changes; this note records verification evidence.
- Deployment assets: `deploy/docker/Dockerfile`, `deploy/compose/docker-compose.yml`.

## Build / Test Status
- Docker image build: pass.
- Container healthcheck: pass.
- Setup → session → CSRF logout → login: pass.
- Restart persistence: pass.
- Cleanup: container and test volume removed; secret permissions restored to `600`.
- Not tested: TLS termination via Caddy/Nginx and a full Compose `up` with the production HTTPS
  configuration (requires a domain/certificate).

## Next Steps
- Start Phase 2 (SSH connections, host-key trust, encrypted credentials).
- Add Playwright E2E and a Compose-based CI smoke test once the test SSH target exists.

## Open Questions
- Choose the first tagged release (`v0.1.0` is a reasonable candidate now that Phase 1 is merged).
