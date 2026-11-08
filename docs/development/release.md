# Release runbook

## Versioning

Drovik uses Semantic Versioning. `v0.x` is beta; `v1.0.0` is the first stable MVP. Prerelease tags
contain a hyphen (`v0.1.0-beta.1`) and do not update the `latest` container tag.

## Checklist

1. `main` is green and all required checks pass.
2. `make verify` and `make security` pass locally.
3. `docs/product/traceability.md`, `CHANGELOG.md` and `docs/status/beta-notes.md` reflect the
   release.
4. Back up a production-like database and restore it into a disposable environment.
5. Create an annotated tag:

   ```bash
   git tag -a v1.0.0 -m "Drovik v1.0.0"
   git push origin v1.0.0
   ```

6. The `Release` workflow builds linux amd64/arm64 archives, SHA-256 checksums and the GHCR image,
   then creates the GitHub release.
7. Verify the release page has both archives, checksums and notes.
8. Pull the published image and run the health/readiness check plus one login.
9. Announce with upgrade notes and known limitations.

## Artifact verification

```bash
sha256sum -c SHA256SUMS
docker pull ghcr.io/awbuilds/drovik:v1.0.0
docker run --rm ghcr.io/awbuilds/drovik:v1.0.0 version
```

Artifacts are checksummed but not cryptographically signed in the MVP. Treat the GitHub release and
GHCR package as the trusted distribution points.

## Migration and rollback

Migrations are forward-only. Before upgrading, stop the service, copy the database plus the key,
then start the new version. If migration fails, restore the pre-upgrade database and run the matching
release. There is no automatic downgrade.

## Release cadence

Security fixes are released as needed. Feature releases are cut from `main` after the full verify
and security workflows pass. Patch releases cherry-pick the smallest fix and repeat the checklist.

## Post-release

- Confirm the image runs as non-root with a read-only root filesystem where possible.
- Monitor the issue tracker for upgrade reports.
- Update the supported-version table in `SECURITY.md`.
