# Drovik — Current Goal

**Milestone:** Phase 0 foundation + Phase 1 application skeleton and authentication
**Status:** in progress
**Updated:** 2026-11-07

## Goal

Ship Drovik's Phase 0 and Phase 1 as reviewable pull requests on a protected `main`:
repository and governance foundation, AGENTS.md and Hydra-style handoffs, AGPL-3.0 licensing,
ADRs and threat model, CI/security gates, a buildable Go backend with crash-safe request handling
and authentication, and a deliberate React frontend with setup/login flows.

## Evidence of completion

- `main` has an active ruleset: PR required, required checks, conversation resolution, linear
  history, no force-push, no deletion, enforced for administrators.
- The foundation and feature PRs are merged only through pull requests.
- `make verify` passes: backend tests with race detector, vet, lint, frontend lint/typecheck/test/
  build.
- Secret scan is clean; no private data in the repository or history.
- A clean deployment can complete first-run setup, log in, log out, restart and log in again
  without data loss.
- A deliberate panic in a request is recovered, logged and does not terminate the process.
- The latest handoff note records exact commands run and any gaps.

## Scope

- In: Phase 0 and Phase 1 of `docs/product/PRD.md`.
- Out: SSH connections, SFTP, terminal, monitoring, processes, services, logs, multi-user UI.
