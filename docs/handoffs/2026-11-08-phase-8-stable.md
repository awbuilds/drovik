# 2026-11-08 Phase 8 stable release

## Task / Area
Finalize release operations, support policy, performance documentation and the stable release.

## Current State
- Release:
  - `.github/workflows/release.yml` builds linux amd64/arm64 archives, frontend assets, SHA-256
    checksums, the GHCR image and a GitHub release from `v*` tags.
  - Beta `v0.1.0-beta.1` built successfully; assets and prerelease flag verified.
  - Stable tag `v1.0.0` publishes `latest` in addition to the version tag.
- Documentation:
  - `docs/development/release.md` covers versioning, checklist, artifact verification, migration,
    rollback and cadence.
  - `docs/development/performance.md` records parser benchmarks and real-host load guidance.
  - `docs/status/stable-notes.md`, `SECURITY.md` supported-version table and `SUPPORT.md` updated.
  - Traceability records Phase 8 and explicitly defers Phase 9 post-MVP work.
- Benchmarks:
  - `make backend-bench` covers monitoring sample, process line, journal entry and service unit
    parsing.
  - Reference results on Apple M3 Pro: ~5.2 µs/sample, ~170 ns/process, ~0.99 µs/journal entry,
    ~2.4 µs/unit list.
- Support/ownership:
  - CODEOWNERS exists for security-sensitive paths and workflows.
  - Stable support policy: latest stable plus current beta security fixes.

## Decisions
- Artifacts are checksummed but not cryptographically signed in the MVP; the GitHub release and
  GHCR package are the trusted distribution points.
- Phase 9 remains deferred by product decision; each advanced feature needs its own design and
  threat model.
- Performance validation is benchmark-based plus documented real-host guidance; there is no
  synthetic load harness for remote SSH latency.

## Build / Test Status
- `make verify` passes.
- Beta release workflow completed successfully:
  `https://github.com/awbuilds/drovik/releases/tag/v0.1.0-beta.1`.
- Stable tag workflow must be run after this change merges.

## Next Steps
- Merge, tag `v1.0.0`, verify stable assets and the `latest` image tag.
- Run the full `security-audit` workflow and address confirmed findings.
- Continue real-host compatibility validation as operational work.

## Open Questions
- Whether stable artifacts should use Sigstore/cosign or GitHub attestations in a follow-up.
- Whether Phase 9 should start with RBAC or historical metrics; both need separate threat models.
