# Public beta notes

**Release line:** `v0.1.0-beta.x`
**Scope:** self-hosted, single-operator MVP.

## What works

- First-run administrator setup, Argon2id login, server-side sessions and CSRF.
- Saved SSH servers with AES-256-GCM credentials and explicit host-key approval.
- Interactive SSH terminal over an authenticated WebSocket.
- SFTP browser, streamed transfers and a bounded text editor.
- Agentless CPU, load, memory, filesystem, network and uptime overview.
- Process list with SIGTERM/SIGKILL identity checks.
- systemd service discovery and enumerated actions with state re-reads.
- Bounded systemd journal viewer.
- Scoped audit history for sensitive actions.

## Known limitations

- One administrator; no user-management UI or RBAC.
- No historical metrics/time-series storage.
- No resumable transfers or background transfer jobs.
- No sudo/privilege escalation; remote permissions are authoritative.
- No unit-file editing, Docker management or file-based log tailing.
- CPU/memory process columns are `ps` lifetime averages, not sampled deltas.
- SQLite allows one writer; use local storage and stop Drovik for cold backups.
- Audit retention is not automatic and date/actor filtering is not yet exposed.
- Compatibility is validated by automated fixtures, not a full real-host matrix.
- Browser behavior is checked manually; no Playwright suite yet.

## Install and upgrade

Use the container image `ghcr.io/awbuilds/drovik:v0.1.0-beta.N` or the release archive. Keep
`DROVIK_ENCRYPTION_KEY` stable across upgrades. Back up the database and key before migrating.
Migrations are forward-only; see [backup and restore](../development/backup-restore.md).

## Security

Report vulnerabilities privately per [`SECURITY.md`](../../SECURITY.md). Do not post secrets,
private keys or production logs in issues. The beta has not yet completed an independent external
security assessment; treat it accordingly.

## Support

Use GitHub issues with the version, managed-host distribution, the stable error code and a minimal
reproduction. No telemetry is collected by Drovik.

## Audit retention

Audit events are retained indefinitely in the local SQLite database. An operator may archive or
delete old rows during a maintenance window:

```sql
DELETE FROM audit_events WHERE created_at < '2026-01-01T00:00:00Z';
VACUUM;
```

Treat audit rows as security metadata and include them in backups. Deleting them is a deliberate
administrative action; Drovik does not claim the local database is tamper-proof.
