# Troubleshooting

Start with `GET /healthz` and `GET /readyz`, then the structured logs. Drovik never includes
credentials, terminal contents or log contents in logs.

## Startup

**`DROVIK_ENCRYPTION_KEY is required` / invalid key**
Generate a 32-byte base64 key and keep it outside the database:

```bash
openssl rand -base64 32
```

Set `DROVIK_ENCRYPTION_KEY` or point `DROVIK_ENCRYPTION_KEY_FILE` at a root-readable file. Changing
the key makes existing credentials undecryptable.

**`bind: address already in use`**
Another process is using `DROVIK_HTTP_ADDR`. Stop it or choose a different port.

**Service exits at startup without a stack trace**
Startup configuration errors are fatal by design. Read the last structured log line; it names the
invalid variable.

## Connections

**`SSH_HOST_KEY_UNKNOWN`**
Expected on first use. Verify the fingerprint out of band and approve it on the Servers page.

**`SSH_HOST_KEY_CHANGED`**
Drovik blocks the connection until an operator deliberately re-verifies and replaces the key.
Investigate a rebuild, key rotation or man-in-the-middle before replacing.

**`SSH_AUTH_FAILED`**
The username or credential is wrong, or the account is locked. Keys must be OpenSSH or PEM format;
encrypted keys need their passphrase.

**`SSH_OPERATION_TIMEOUT` / `SSH_CONNECTION_FAILED`**
Check network reachability, `DROVIK_SSH_*_TIMEOUT` values and the egress policy. Loopback,
link-local, multicast and metadata destinations are blocked by default. For a local test server,
add an explicit narrow allowlist such as `DROVIK_EGRESS_ALLOWED_CIDRS=127.0.0.1/32`; do not widen
this in production.

## Features

**`SFTP_UNAVAILABLE`**
The host does not expose the SFTP subsystem. Enable it in `sshd_config` or use the terminal.

**`SFTP_PERMISSION_DENIED`**
The connected SSH user lacks permission for that path or action. Drovik does not retry with sudo.

**`PROCESS_UNAVAILABLE`**
The remote `ps` does not support the required `-eo` format. This is common with minimal BusyBox
images. Install procps or use the terminal.

**`PROCESS_PID_REUSED`**
The process start time changed between listing and signaling. Refresh the list and verify the new
process before acting.

**`SERVICE_MANAGER_UNAVAILABLE`**
The host is not running systemd (common in containers) or the account cannot reach the system bus.
Drovik does not escalate privileges.

**`SERVICE_PERMISSION_DENIED`**
PolicyKit or systemd denied the action for this SSH user. Adjust policy on the host or use an
account with the intended permission.

**`JOURNAL_PERMISSION_DENIED`**
The SSH user is not in `systemd-journal` (or equivalent) and cannot read the requested journal.
Grant read access on the host; Drovik will not retry as root.

**Terminal WebSocket returns 403**
The browser `Origin` must match `DROVIK_TRUSTED_ORIGINS`. Add the exact scheme, host and port used
to reach Drovik.

## Data and updates

**`database is locked`**
SQLite allows one writer. Ensure only one Drovik process uses the database and that the data
directory is on a local filesystem, not a network share. Stop the service before copying the
database.

**Upgrade fails during migration**
Migrations are forward-only. Restore the pre-upgrade database backup and run the matching release.
See [backup and restore](backup-restore.md).

**Encryption key lost**
There is no recovery path for encrypted credentials. Restore the matching key or delete and
re-enter server credentials.

## Reporting a problem

Include the Drovik version, managed-host distribution, the stable error code, the correlation ID
from the response, and whether the action worked through plain `ssh`. Do not include secrets,
private keys, hostnames or log contents you consider sensitive.
