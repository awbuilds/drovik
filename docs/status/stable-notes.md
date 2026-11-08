# Stable release notes

**Release line:** `v1.0.0`

Drovik 1.0 is the first stable MVP for self-hosted, agentless Linux server management. It includes:

- setup, Argon2id authentication, sessions and CSRF;
- encrypted SSH credentials with strict host-key approval;
- terminal, SFTP files and bounded editing;
- agentless monitoring and host overview;
- process signals, systemd actions and bounded journal logs;
- scoped audit history.

## Compatibility

See [compatibility](../development/compatibility.md). The beta and stable releases share the same
SQLite schema; upgrades are forward-only and require a database backup.

## Support

Only the latest stable minor release and the current beta line receive security fixes. See
[`SECURITY.md`](../../SECURITY.md) and [`SUPPORT.md`](../../SUPPORT.md).

## Security patch

`v1.0.1` fixes five issues found by the first source-first audit. The high-severity issue could
expose a stored private key when changing authentication method; the other fixes cover terminal
logout/expiry, IPv6 zone egress bypass, upload permission widening and journal entry-count
exhaustion. See [`docs/security/audit-2026-11-08.md`](../security/audit-2026-11-08.md).

## Known limitations

The beta limitations remain accurate unless noted: single operator, no historical metrics, no
resumable transfers, no privilege escalation, no unit editing, SQLite single-writer, and limited
real-host validation. Phase 9 features are explicitly post-MVP and require separate threat models.

## Verification

```text
make verify
make security
go test -race ./...
pnpm lint && pnpm typecheck && pnpm test && pnpm build
release workflow on the tagged commit
```
