# Roadmap

The authoritative phased plan is in [docs/product/PRD.md](docs/product/PRD.md). This page is a
short, human-readable summary.

## Now — Phase 0: foundation

- Repository, branch protection and PR-only workflow.
- AGENTS.md, handoff notes, ADRs, threat model.
- Open-source governance, licensing and security policy.
- CI, linting, tests and security scanning.

## Next — Phase 1: application skeleton and authentication

- Go service, configuration, structured logging and graceful shutdown.
- SQLite schema and migrations.
- First-run setup, local login/logout, sessions and CSRF.
- React application shell and design system.
- Docker Compose deployment and end-to-end smoke test.

## Then

| Phase | Focus |
|---|---|
| 2 | SSH connection manager, encrypted credentials and host-key trust |
| 3 | Workspace and interactive terminal |
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
