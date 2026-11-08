# Changelog

All notable changes to Drovik are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Phase 2 SSH connection manager: encrypted credentials (AES-256-GCM), strict host-key
  verification with first-use approval and changed-key blocking, server CRUD, connection tests,
  egress policy, password/private-key authentication and capability detection.
- Phase 3 interactive terminal: authenticated WebSocket PTY, resize, bounded per-user/global
  sessions, rate limits, session cleanup and the xterm.js terminal panel.
- Phase 4 SFTP file manager: safe directory listing, streamed uploads/downloads, create/rename/
  delete, recursive-delete guards, symlink handling, stale-file detection and a bounded text
  editor.
- Phase 5 host monitoring: agentless CPU, load, memory, filesystem, network and uptime snapshots
  with two-sample rate calculation, per-host caching/coalescing, stale fallback and a live
  monitoring page.
- Phase 6 operations: bounded process list with SIGTERM/SIGKILL and PID identity checks, systemd
  service discovery/actions with post-action state, and a bounded systemd journal viewer.
- Phase 7 beta: scoped audit history with filters and a UI page, accessible skip navigation,
  stable error-code catalog, compatibility/troubleshooting/backup/security-testing guides and a
  tag-driven release workflow with checksum generation.
- Phase 8 stable: release runbook, versioned support policy, stable release notes and parser
  benchmarks for monitoring, processes, services and journal logs.
- Project foundation: AGENTS.md contract, Hydra-style handoff notes, AGPL-3.0 licensing,
  contribution, security and governance documentation.
- Architecture decision records and threat model.
- React + Tailwind design system foundation.
- Go backend foundation with structured logging, recovery middleware, health checks and graceful
  shutdown.
- First-run setup, local authentication, sessions and CSRF protection.
- Docker Compose deployment with a separately mounted encryption secret.

### Security

- Fixed a high-severity credential-exposure bug in `PATCH /api/v1/servers/{id}`: changing
  `auth_method` without a new credential could reinterpret a stored private key as a password.
- Fixed terminal sessions surviving logout/expiry, IPv6 zone identifier egress bypass, upload
  permission widening and journal entry-count exhaustion. See
  `docs/security/audit-2026-11-08.md`.
- No telemetry or external error reporting by default.
- Secret scanning and dependency review in CI.

<!--
When releasing, rename [Unreleased] to [x.y.z] - YYYY-MM-DD and add a new Unreleased section.
-->
