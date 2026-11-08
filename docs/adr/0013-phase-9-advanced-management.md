# 0013 — Phase 9 advanced management boundaries

## Status

Accepted — final Phase 9 scope (approved 2026-11-08). The read-first scope below is the permanent
1.x boundary. The excluded items are not pending work; revisiting any of them requires a new ADR
and its own threat model.

## Context

Phase 9 adds higher-risk surfaces: Docker daemon access, host diagnostics, user-defined command
snippets, local users/server assignments and retained metrics. These must not turn an authenticated
web session into unbounded root-equivalent authority by accident.

## Decision

- **Docker** is accessed only through the existing SSH connection. Drovik never mounts a Docker
  socket into its own container. Only list/image/log/inspect and start/stop/restart/remove by
  validated container ID are exposed. Every action is confirmed, audited, blocked for read-only
  assignments and labelled as potentially root-equivalent.
- **Storage, network, packages and scheduled tasks** are read-only. Destructive disk, firewall,
  interface, package and cron mutations are intentionally absent until separately designed and
  reviewed. Commands are fixed; package search filters a bounded local list rather than querying
  repositories.
- **Snippets** store a named command template and optional non-secret variables. Substitution
  shell-quotes every value and the client must render a substituted preview and confirm a target
  server. Snippets execute as the SSH account, never with hidden elevation, and are audited. The UI
  warns against storing secrets.
- **Users and assignments:** administrators manage local users and per-server `read`/`write`
  assignments. Owners and administrators always have write access. The account role `read_only` and
  a `read` assignment both block terminal, process/service actions, file mutations, Docker actions,
  snippet execution and alert mutations. At least one active administrator must remain; a user
  cannot delete themselves.
- **History:** optional local SQLite samples are retained for a configured window and sample cap.
  Existing snapshots are recorded on non-cached overview requests. Threshold rules are evaluated
  in-app; external notification channels are not implemented. The UI states that this is not an
  enterprise monitoring system.
- **Additional operating systems:** Linux remains the only supported target. macOS/BSD are detected
  as unsupported for Linux-specific collectors; no unverified shell compatibility is claimed.

## Permanently out of scope for the 1.x line

The following Phase 9 candidates were reviewed and are not planned for the 1.x series:

- Mount, unmount, format, partition or other destructive storage operations.
- Firewall, interface, route or other network changes.
- Package install, upgrade, removal or repository configuration changes.
- Cron, systemd timer or other scheduled-task editing.
- OIDC/SSO, MFA, external secret managers or other federated identity.
- External notification channels (email, webhook, chat, paging).
- macOS, BSD or Windows managed-host support.

Rationale: each changes the product's threat model or support matrix, and several can destroy data
or lock an operator out of a host. The current read-first surface preserves the project's rule that
destructive host changes need independent risk review and explicit design. Adding any of them later
is a new feature programme, not a completion criterion for Phase 9.

## Consequences

- Docker remains a high-risk capability; operators must treat the SSH account as privileged.
- Read-only users can inspect assigned servers but cannot perform any action that changes remote
  state or opens a PTY.
- Metrics storage grows with retention; the cap and retention are operator-controlled, and the
  database remains part of the backup boundary.
- OIDC/SSO, MFA, external secret stores, external alert notifications and destructive host changes
  remain future work and require their own threat models.
