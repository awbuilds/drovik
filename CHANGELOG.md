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
- Project foundation: AGENTS.md contract, Hydra-style handoff notes, AGPL-3.0 licensing,
  contribution, security and governance documentation.
- Architecture decision records and threat model.
- React + Tailwind design system foundation.
- Go backend foundation with structured logging, recovery middleware, health checks and graceful
  shutdown.
- First-run setup, local authentication, sessions and CSRF protection.
- Docker Compose deployment with a separately mounted encryption secret.

### Security

- No telemetry or external error reporting by default.
- Secret scanning and dependency review in CI.

<!--
When releasing, rename [Unreleased] to [x.y.z] - YYYY-MM-DD and add a new Unreleased section.
-->
