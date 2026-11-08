# 2026-11-08 Phase 9 released v1.1.0

## Task / Area
Complete Phase 9, publish `v1.1.0`, and prepare the second security-audit run on the new tree.

## Current State
- Phase 9 merged through PR #15 as `5771341` and released as `v1.1.0`.
- Release assets: `drovik-v1.1.0-linux-amd64.tar.gz`, `drovik-v1.1.0-linux-arm64.tar.gz`,
  `SHA256SUMS`; GHCR image `ghcr.io/awbuilds/drovik:v1.1.0` plus `latest`
  (`sha256:5e7cb1a2…`).
- Scope delivered: Docker inspection/enumerated actions, read-only storage/network/package/
  scheduled-task diagnostics, snippets, local users/roles/server assignments, optional metric
  retention/history and in-app threshold alerts.
- RBAC is backend-enforced: read-only role and `read` assignment block terminal, mutations, Docker
  actions, alert changes and snippet execution.
- Phase 9 security audit run (`~/security-audit-skill/drovik/run-2/`) is in progress on `5771341`.

## Decisions
- Phase 9 is read-first by design; destructive storage/network/package/cron changes remain
  unavailable pending separate risk review.
- Docker is reached only over SSH; no socket mount.
- Snippets are arbitrary templates by design with shell-quoted substitution, preview, server-side
  confirmation and audit.
- History is local and bounded; external notifications and OIDC/MFA remain deferred.

## Build / Test Status
- `make verify` passes (Go race tests, vet, golangci-lint; frontend lint/typecheck/tests/build).
- PR #15 checks all pass (Backend, Frontend, CodeQL, Go vulnerability, Secret scan, Dependency
  review, Docs).
- Manual browser checks: Advanced Docker/Storage/Network/Packages, Users creation, Snippets
  preview/run, Monitoring history.

## Next Steps
- Complete audit run-2, remediate confirmed findings, publish a patch release if needed.
- Re-check the 12 carried validation leads and update their status.
- Continue real-host Docker/systemd/browser validation.

## Open Questions
- Whether snippet command templates should forbid quotes around placeholders as defense-in-depth
  (currently self-inflicted only).
- Whether to close terminals immediately on password reset in addition to the 30s revalidation.
