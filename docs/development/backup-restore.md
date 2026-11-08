# Backup and restore

Drovik stores all persistent state in one SQLite database and keeps the credential encryption key
outside it. A backup is only useful when both are preserved.

## What to back up

1. **Database**: `${DROVIK_DB_PATH}` (default `/data/drovik.db`) and any SQLite `-wal` / `-shm`
   files created beside it. The safest method is to stop Drovik first.
2. **Encryption key**: the value of `DROVIK_ENCRYPTION_KEY` or the file referenced by
   `DROVIK_ENCRYPTION_KEY_FILE`. Store it separately from the database.
3. **Configuration**: environment variables or your deployment's `.env` file, excluding any
   secrets that are already covered by the key backup.

The frontend static build and the Drovik binary are reproducible from a release tag and do not need
to be backed up.

## Cold backup (recommended)

```bash
# 1. Stop the service so SQLite has no open writers.
sudo systemctl stop drovik

# 2. Copy the database and its sidecars.
sudo install -d -m 700 /var/backups/drovik
sudo cp -a /data/drovik.db /data/drovik.db-wal /data/drovik.db-shm /var/backups/drovik/ 2>/dev/null || true

# 3. Back up the key through your secret store or a root-only file.
sudo install -m 600 /etc/drovik/encryption.key /var/backups/drovik/encryption.key

# 4. Restart.
sudo systemctl start drovik
```

Encrypt the backup at rest. A database copy plus its key is equivalent to full credential access.

## Restore

1. Install the same or a newer Drovik release.
2. Stop the service.
3. Restore `drovik.db` and the key file to their configured paths.
4. Start the service and confirm `/readyz` and the version endpoint.
5. Sign in and run a connection test against one server.

Never restore an older database over a newer one without a migration plan: migrations are
forward-only. Keep the pre-upgrade database until the new version is verified.

## Verify a backup

Restore into a disposable environment, start Drovik with a non-production port, sign in, and verify
that a saved server and its host-key trust entry are present. Do not test by connecting to a
production host unless that is acceptable.

## Key rotation

Key rotation is not automated in the MVP. Changing `DROVIK_ENCRYPTION_KEY` makes existing
credentials undecryptable. Keep the previous key available until credentials are re-entered, or
restore the matching backup.

## Known limitation

Backups are operator-managed. Drovik does not upload data, schedule backups or provide cloud
storage.
