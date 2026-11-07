# Drovik Deployment

Drovik is designed to run as one small self-hosted service. The primary supported
path is Docker Compose behind a TLS reverse proxy.

## Docker Compose

1. Install Docker Engine with the Compose plugin.
2. Create the compose environment file:

   ```bash
   cd deploy/compose
   cp .env.example .env
   # Edit .env: set DROVIK_PUBLIC_URL and DROVIK_TRUSTED_ORIGINS to your HTTPS URL.
   ```

3. Create the encryption secret. This key protects stored SSH credentials and must
   not live beside the database:

   ```bash
   mkdir -p secrets
   openssl rand -base64 32 > secrets/encryption.key
   chmod 600 secrets/encryption.key
   ```

4. Build and start:

   ```bash
   docker compose up -d --build
   docker compose ps
   ```

5. Put a reverse proxy in front of `127.0.0.1:8080`. Examples:
   - [`deploy/caddy/Caddyfile`](../caddy/Caddyfile)
   - [`deploy/nginx/drovik.conf`](../nginx/drovik.conf)

6. Open the public URL and complete first-run setup **before exposing the instance to untrusted
   users**. Setup is disabled after the first administrator and is rate-limited per IP, but an
   unconfigured installation should still be reached only over a trusted network or a private
   tunnel.

The container runs as non-root on a distroless image, with a read-only root
filesystem, no Linux capabilities, and `no-new-privileges`. The database and
uploads live on the `drovik-data` volume; the encryption key is a Compose secret.

## Backup

A complete backup needs both:

- the SQLite database on the `drovik-data` volume, and
- `secrets/encryption.key`.

Back up the database consistently (stop the container or use SQLite's backup API),
encrypt the backup, and store the key separately. **If the key is lost, stored SSH
credentials cannot be recovered.** See the PRD §8.6 for the full procedure.

```bash
docker compose stop drovik
docker run --rm -v drovik_drovik-data:/data -v "$PWD/backups:/backup" alpine \
  tar -czf /backup/drovik-data-$(date +%Y%m%d).tar.gz -C /data .
docker compose start drovik
```

Never commit `.env`, `secrets/` or backups.

## Upgrade

1. Back up the database and key.
2. Pull or build the new version.
3. Start it; migrations run before the service accepts traffic.
4. Verify `/readyz` and the version in Settings.

A failed migration must be treated as a failed upgrade. Restore the backup if the
documented rollback path is not available.

## Native systemd

A hardened unit template is at [`deploy/systemd/drovik.service`](../systemd/drovik.service).
Create a dedicated `drovik` user, install the binary at `/usr/local/bin/drovik`,
and put the environment file at `/etc/drovik/drovik.env` with restrictive
permissions. Native packaging is secondary to the container path for the MVP.
