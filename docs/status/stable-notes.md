# Stable release notes

**Release line:** `v1.3.1`

Drovik 1.0 was the first stable MVP for self-hosted, agentless Linux server management. `v1.3.1`
completes the Phase 9 scope. The patch release adds teams, audit search/export, snippet
import/export, strict non-Linux read-only enforcement, and fixes found by live Lima testing. It includes:

- setup, Argon2id authentication, sessions and CSRF;
- encrypted SSH credentials with strict host-key approval;
- terminal, SFTP files and bounded editing;
- agentless monitoring and host overview;
- process signals, systemd actions and bounded journal logs;
- scoped audit history;
- Docker inspection and enumerated container actions, read-only storage/network/package/scheduled-task
  diagnostics, command snippets with preview and confirmation;
- local users with roles and per-server `read`/`write` assignments;
- optional bounded metric history, in-app threshold alerts and encrypted webhooks;
- package install/upgrade/remove and Drovik-managed cron tasks;
- local TOTP MFA with recovery codes and optional OIDC single sign-on with explicit account linking;
- optional HashiCorp Vault KV v2 SSH credentials;
- read-only macOS/BSD monitoring;
- feature-flagged destructive storage/network operations (disabled by default).

## Compatibility

See [compatibility](../development/compatibility.md). The beta and stable releases share the same
SQLite schema; upgrades are forward-only and require a database backup.

## Support

Only the latest stable minor release and the current beta line receive security fixes. See
[`SECURITY.md`](../../SECURITY.md) and [`SUPPORT.md`](../../SUPPORT.md).

## Security patches

`v1.3.0` completed Phase 9 and fixed six issues found by the final source-first audit, including
OIDC link step-up, MFA re-enrollment, OIDC browser binding and webhook URL logging.

`v1.1.1` fixed seven issues found by the second source-first audit of the Phase 9 tree and pins
container base images by digest. See [`docs/security/audit-2026-11-08.md`](../security/audit-2026-11-08.md).

`v1.0.1` fixed five issues found by the first source-first audit. The high-severity issue could
expose a stored private key when changing authentication method; the other fixes cover terminal
logout/expiry, IPv6 zone egress bypass, upload permission widening and journal entry-count
exhaustion. See [`docs/security/audit-2026-11-08.md`](../security/audit-2026-11-08.md).

## Known limitations

Local users and server assignments replace the original single-operator limitation. Destructive
storage/network/package/cron changes, OIDC/MFA, external notifications and non-Linux targets are
permanently out of scope for the 1.x line (ADR 0013), not pending work. Other remaining limitations:
no resumable transfers or privilege escalation, no unit editing, SQLite single writer and limited
real-host validation. The
Phase 9 threat model is recorded in ADR 0013 and the 2026-11-08 audit summary.

## Verification

```text
make verify
make security
go test -race ./...
pnpm lint && pnpm typecheck && pnpm test && pnpm build
release workflow on the tagged commit
```
