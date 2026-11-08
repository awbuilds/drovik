# 2026-11-08 Stable release v1.0.0

## Task / Area
Publish and verify the stable release after Phase 8 merged.

## Current State
- `v1.0.0` annotated tag pushed from `main` at `bcc762d`.
- Release workflow succeeded:
  - assets: `drovik-v1.0.0-linux-amd64.tar.gz`, `drovik-v1.0.0-linux-arm64.tar.gz`,
    `SHA256SUMS`;
  - GitHub release marked stable;
  - GHCR image `ghcr.io/awbuilds/drovik:v1.0.0` pushed;
  - `latest` also pushed (digest `sha256:1befbff…` in the workflow log).
- Beta `v0.1.0-beta.1` workflow also succeeded, including prerelease assets/image.

## Decisions
- Stable release is `v1.0.0`; beta remains `v0.1.0-beta.1`.
- Phase 9 is documented as deferred post-MVP work.
- A full security audit is in progress; findings and any follow-up fixes are recorded separately.

## Build / Test Status
- `make verify` and the release workflow passed on the tagged commit.
- Release assets and `latest` image verified from workflow logs.

## Next Steps
- Complete the security audit and address confirmed findings.
- Consider tag ruleset/environment protection for release tags in a follow-up.
