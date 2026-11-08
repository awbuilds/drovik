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
- **Destructive storage and network** are implemented behind the disabled-by-default
  `DROVIK_ENABLE_DESTRUCTIVE_HOST_OPS` flag. Mount/unmount, format, partition, link state,
  address/route changes and ufw/firewalld rules are administrator-only, require CSRF and explicit
  confirmation, and format/partition require typing the exact device. Mountpoints `/`, `/boot`,
  `/proc`, `/sys`, `/dev`, `/etc`, `/usr`, `/var` and `/run` are refused; format/partition refuse a
  mounted device; all actions are audited. These commands can still destroy data or disconnect the
  host, so tests are run only against disposable Colima containers/loop devices.
- **Package lifecycle** exposes install, upgrade and remove for the detected package manager
  (apt/dnf/apk/pacman). Package names are strictly validated, confirmation is required, commands
  are enumerated, no sudo/escalation is attempted and every action is audited. Failures return the
  remote output without claiming success.
- **Scheduled tasks** support creating and deleting Drovik-marked entries in the SSH user's own
  crontab. The schedule and command are validated, the full crontab is staged over SFTP and
  installed with `crontab`, existing entries are preserved, and changes are audited. Commands are fixed; package search filters a bounded local list rather than querying
  repositories.
- **Snippets** store a named command template and optional non-secret variables. Substitution
  shell-quotes every value and the client must render a substituted preview and confirm a target
  server. Snippets execute as the SSH account, never with hidden elevation, and are audited. The UI
  warns against storing secrets.
- **Local MFA:** users can enroll a TOTP authenticator with encrypted-at-rest secrets and eight
  single-use recovery codes. Enrollment and replacement require the local password; replacement
  also requires a current authenticator/recovery code. Login requires the second factor when
  enabled; recovery codes are hashed and consumed once.
- **External secret managers:** optional HashiCorp Vault KV v2 support. A server records
  `secret_provider=vault` and a validated path; the password, private key or passphrase is fetched
  at connect time with a bounded timeout, no redirects and the shared egress policy. Credentials
  are never written to the Drovik database, and Vault failures fail closed with
  `SECRET_UNAVAILABLE`. Only the path is returned to the UI.
- **OIDC/SSO:** optional authorization-code flow with PKCE, discovery (issuer checked), no
  redirects, bounded responses and single-use five-minute state. Accounts are never auto-provisioned
  or matched by username: an authenticated user links their existing local account from Settings,
  and login only succeeds for a stored `(issuer, subject)` pair. Link start requires the local
  password and, when MFA is enabled, a current authenticator or recovery code; the callback must
  arrive on the same session that started it, and state is bound to a short-lived HttpOnly cookie.
  Unlink also requires the password. The provider's policy is the SSO authentication boundary; local
  TOTP is not also required for an SSO sign-in. State is in-process, which matches the
  single-instance SQLite architecture.
- **Users and assignments:** administrators manage local users and per-server `read`/`write`
  assignments. Owners and administrators always have write access. The account role `read_only` and
  a `read` assignment both block terminal, process/service actions, file mutations, Docker actions,
  snippet execution and alert mutations. At least one active administrator must remain; a user
  cannot delete themselves.
- **Alert webhooks:** each threshold rule may carry an HTTPS/HTTP webhook. URLs are encrypted at
  rest, resolved once through the egress policy (blocking loopback/link-local/metadata by default),
  delivered with a bounded timeout, no redirects and a 15-minute per-rule cooldown. Delivery
  failures are logged and retried after the cooldown. Email/paging integrations remain future work.
- **History:** optional local SQLite samples are retained for a configured window and sample cap.
  Existing snapshots are recorded on non-cached overview requests. Threshold rules are evaluated
  in-app and may deliver encrypted webhooks. The UI states that this is not an enterprise
  monitoring system.
- **Additional operating systems:** Linux remains the full-feature target. macOS (Darwin) and BSD
  hosts get read-only monitoring from portable collectors (`uname`, `sysctl`, `vm_stat`, `df`,
  `mount`, `netstat`, `top`): host identity, load, memory, filesystems, network counters and uptime.
  CPU usage on macOS is a point-in-time `top` sample; per-core counters, systemd, journal, Docker
  and the Linux-only advanced actions remain unavailable and are labelled as such. Linux-specific
  collectors are not claimed to work on other targets.

## Permanently out of scope for the 1.x line

The following Phase 9 candidates were reviewed and are not planned for the 1.x series:

- Unattended or scheduled destructive operations (the flag remains manual and disabled by default).
- Repository configuration changes and automatic update scheduling.
- systemd timer or unit-file editing.
- Additional secret-manager backends beyond Vault KV v2 and additional federated identity
  providers.
- Email, chat or paging notification channels (webhooks are implemented).
- Windows managed-host support.
- Write operations on macOS/BSD hosts beyond read-only monitoring.

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
