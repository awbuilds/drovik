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

### Added

- Destructive host operations behind `DROVIK_ENABLE_DESTRUCTIVE_HOST_OPS` (default false):
  mount/unmount, format, partition, link state, address/route changes and ufw/firewalld rules, with
  admin-only access, typed device confirmation, mount/root guards and audit.
- Alert webhooks: encrypted URL storage, egress-checked delivery, bounded timeout and a 15-minute
  per-rule cooldown, configured from the monitoring history panel.
- Local TOTP MFA with encrypted secrets, eight single-use recovery codes and a login challenge;
  logout now clears the query cache and reports failure instead of claiming a revoked session.
- Phase 9 expansion: enumerated package install/upgrade/remove for apt, dnf, apk and pacman with
  confirmation and audit; Drovik-managed crontab task creation/deletion with validation, SFTP
  staging and audit.
- Phase 9 advanced management: Docker inspection and enumerated actions, read-only storage/network/
  package/scheduled-task diagnostics, command snippets, local users with roles and per-server
  assignments, retained metric history/charts and in-app threshold alerts.

### Documentation

- Added the static project website (`site/`) with product, security, install, documentation, FAQ,
  changelog, privacy, terms, accessibility and 404 pages. It self-hosts the application's fonts,
  makes no third-party requests, and is checked by `scripts/check-site.py` for links, metadata,
  design-token parity and data hygiene.

### Security

- Second security audit (Phase 9 tree): bounded snippet variable substitution, linear and
  line-capped collector assembly, truthful process/service outcomes with detached audit writes,
  non-finite metric rejection with fail-visible JSON encoding, deduplicated metric samples, an
  atomic last-active-administrator guard, sanitized Docker inspect output and bounded alert-rule
  cardinality/identifiers.
- Supply chain: pinned container base images by digest, Corepack-based pnpm provisioning,
  least-privilege workflow token permissions and upgraded `golang.org/x/crypto` past GO-2026-5932.
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
