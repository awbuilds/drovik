# 2026-11-08 Phase 9 advanced management

## Task / Area
Implement the read-first Phase 9 advanced-management scope: Docker, host diagnostics, snippets,
local users/server assignments and optional metric history.

## Current State
- Backend:
  - `internal/advanced`: fixed Docker container/image/log/inspect collectors and enumerated
    start/stop/restart/remove actions by validated container ID; read-only storage, network, package
    and scheduled-task collectors; snippet CRUD/preview/run; metric sample retention and in-app
    alert rules.
  - `internal/users`: local user CRUD, role/active changes, password reset/session revocation and
    last-active-admin protection.
  - `server_assignments` and effective access levels; owners/admins always write; read/write
    assignments enforced by `requireServerWrite` for terminal, test, signals, service actions, file
    mutations, Docker actions, alert mutations and snippet execution.
  - Migration `00003_advanced.sql`: assignments, snippets, metric_samples, alert_rules.
  - Metric snapshots from non-cached overview requests are retained for a configured window/sample
    cap; alert rules evaluate against the latest sample.
- Frontend:
  - Advanced page with Docker, Storage, Network, Packages and Schedules tabs; Docker actions and
    logs; root-equivalent warning.
  - Snippets page with variables, substituted preview, target/confirmation and output.
  - Users page with role/status/password/delete controls; server access dialog.
  - Monitoring history chart and in-app threshold rule controls.
- Docs: ADR 0013, OpenAPI paths, API README, traceability, README/ROADMAP/AGENTS/CHANGELOG.

## Decisions
- Destructive storage/network/package/cron operations remain unavailable; they require separate
  risk review. OIDC/MFA, external notifications and non-Linux target support are explicitly deferred.
- Docker is accessed through SSH, never through a mounted socket. The UI labels Docker access as
  potentially root-equivalent.
- Snippets are arbitrary command templates by design; values are shell-quoted, preview is mandatory,
  confirmation is required server-side and every execution is audited.
- Metrics history is local SQLite with a retention window and sample cap; it is not an enterprise
  monitoring system.

## Build / Test Status
- `go test -race ./...` and `golangci-lint run` pass; advanced/users/RBAC HTTP tests included.
- Frontend typecheck/lint/tests/build pass; manual browser checks covered Advanced tabs, Users and
  Snippets execution.
- Not run: real Docker daemon matrix, real macOS/BSD targets, OIDC/MFA.

## Next Steps
- Merge through a PR, run `make verify`, tag `v1.1.0` and verify release assets.
- Re-run the full security audit on the Phase 9 tree, including Docker/snippet/RBAC boundaries.
- Follow-up: shared SSH session accounting across advanced collectors, destructive host-operation
  designs, external notifications, OIDC/MFA and OS compatibility matrix.
