# 2026-11-08 Security audit fixes

## Task / Area
Remediate confirmed findings from the first source-first security audit and publish a patch release.

## Current State
- `v1.0.0` audit completed with 5 confirmed findings, 12 needs-validation leads and 1 rejected
  candidate. Full report/ledger/evidence are retained outside the repository.
- Fixes:
  - `servers.Update` requires a credential of the new type when `auth_method` changes and never
    decrypts a stored secret under a different method.
  - Terminal sessions carry the application session ID, are closed on logout, and are revalidated
    every 30 seconds; stdin close/write is synchronized.
  - Egress rejects IPv6 zone identifiers and explicitly blocks loopback/link-local/multicast/
    unspecified classes.
  - Upload/editor staging uses a private temp file, applies the final target mode before rename and
    aborts on chmod failure.
  - Journal parsing stops at the requested line count.
- Regression tests added for all five fixes.
- Public summary: `docs/security/audit-2026-11-08.md`. Changelog and stable notes updated.

## Decisions
- Confirmed issues are fixed in a patch release (`v1.0.1`) rather than rewriting `v1.0.0`.
- The 12 validation leads remain documented as leads; several are beta limitations or require a
  real host/browser observation. They do not block the patch.
- The full audit artifacts stay outside the repository because they contain detailed attack paths;
  the repository carries a concise fixed-issue summary.

## Build / Test Status
- `go test -race ./...` and `golangci-lint run` pass with the new regression tests.
- `make verify` must be run before merge.
- Release workflow for `v1.0.1` must be verified after tagging.

## Next Steps
- Run `make verify`, merge through a PR, tag `v1.0.1`, verify release assets/image.
- Work the 12 validation leads as follow-up hardening; prioritize shared SSH accounting, process
  outcome reporting and terminal/SFTP operation deadlines.
- Add real-host and browser validation for the needs-validation records.

## Open Questions
- Whether to make `OutcomeUnknown` a first-class API/UI state in the next minor release.
- Whether recursive delete should be disabled until a no-follow primitive is available.
