# Drovik — Current Goal

**Milestone:** Phase 0 foundation + Phase 1 application skeleton and authentication
**Status:** implemented on `feat/phase-1-app`, awaiting review and deployment verification
**Updated:** 2026-11-07

## Goal

Ship Drovik's Phase 0 and Phase 1 as reviewable pull requests on a protected `main`:
repository and governance foundation, AGENTS.md and Hydra-style handoffs, AGPL-3.0 licensing,
ADRs and threat model, CI/security gates, a buildable Go backend with crash-safe request handling
and authentication, and a deliberate React frontend with setup/login flows.

## Evidence of completion

- `main` has an active ruleset: PR required, conversation resolution, linear history, no force-push,
  no deletion, enforced for administrators. Required checks are added once CI names are registered.
- The foundation and feature PRs are merged only through pull requests.
- `make verify` passes: Go format, vet, race tests, golangci-lint; frontend install, lint,
  typecheck, tests and production build.
- Secret scan is clean; no private data in the repository or history.
- First-run setup, login, logout, restart persistence and session revocation are covered by tests.
- A deliberate panic in a request is recovered, logged and does not terminate the process.
- Docker Compose, Caddy and Nginx examples exist; a container smoke test is the remaining
  environment-dependent verification.
- The latest handoff note records exact commands run and any gaps.

## Scope

- In: Phase 0 and Phase 1 of `docs/product/PRD.md`.
- Out: SSH connections, SFTP, terminal, monitoring, processes, services, logs, multi-user UI.
