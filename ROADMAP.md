# Roadmap

The authoritative phased plan is in [docs/product/PRD.md](docs/product/PRD.md). This page is a
short, human-readable summary.

## Complete — Phase 0: foundation

- Repository, branch protection and PR-only workflow.
- AGENTS.md, handoff notes, ADRs, threat model.
- Open-source governance, licensing and security policy.
- CI, linting, tests and security scanning.

## Complete — Phase 1: application skeleton and authentication

- Go service, configuration, structured logging and graceful shutdown.
- SQLite schema and migrations.
- First-run setup, local login/logout, sessions and CSRF.
- React application shell and design system.
- Docker Compose deployment assets and OpenAPI contract.

## Complete — Phase 2: SSH connections

- Encrypted credentials and strict host-key trust.
- Saved server CRUD, connection tests and capability detection.
- Egress policy, password/private-key authentication and audit events.

## Complete — Phase 3: workspace and interactive terminal

- Authenticated WebSocket terminal with PTY and resize.
- Bounded session pool and cleanup.
- Terminal UI with server context and reconnect states.

## Complete — Phase 4: SFTP file manager and editor

- Safe directory listing, uploads, downloads and transfers.
- Create/rename/delete with traversal and symlink protections.
- Bounded text editor with unsaved-change and stale-file handling.

## Complete — Phase 5: system overview and metrics

- Bounded `/proc` and standard-command collectors.
- CPU deltas, memory, disk, uptime, load and network rates.
- Honest unavailable/stale states and per-host coalescing.

## Complete — Phase 6: processes, systemd services and logs

- Bounded process discovery and enumerated signal actions.
- systemd unit discovery, actions and status re-reads.
- Bounded journal retrieval with safe filtering.

## Complete — Phase 7: MVP integration, security hardening and public beta

- Cross-module server-context and stale-data review.
- Audit history, accessibility and responsive polish.
- Security test pass, dependency/static analysis and beta release assets.

## Complete — Phase 8: stable release and operational maturity

- Versioned release pipeline, checksums and container publishing.
- Migration, backup/restore and compatibility guidance.
- Parser benchmarks, stable support policy and release runbook.

## Complete — Phase 9: advanced server management (read-first)

- Docker and host diagnostics with explicit actions and read-only blocking.
- Snippets with substitution preview and confirmation.
- Local users, roles and per-server read/write assignments.
- Local metric retention, history charts and in-app threshold alerts.

Destructive storage/network/package/cron changes, OIDC/MFA, external notifications and non-Linux
target support are permanently out of scope for the 1.x line (ADR 0013). Revisiting them requires a
new ADR and threat model; they are not pending Phase 9 work.
| 4 | SFTP file manager and text editor |
| 5 | System overview and metrics |
| 6 | Processes, systemd services and logs |
| 7 | Integration, security hardening and public beta |
| 8 | Stable release and operational maturity |
| 9 | Post-MVP: containers, storage, network, packages, multi-user, metrics history |

## Explicitly not in the MVP

Docker/container management, Kubernetes, file synchronization, cron editor, firewall/network
editors, user management UI, package installation, provisioning, remote desktop, persistent
session orchestration, long-term metrics, alerting, SSO/MFA and multi-tenant SaaS.
