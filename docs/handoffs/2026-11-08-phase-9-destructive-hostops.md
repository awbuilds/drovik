# 2026-11-08 Phase 9 destructive host operations

## Task / Area
Add feature-flagged destructive storage and network operations with disposable-container testing.

## Current State
- Config: `DROVIK_ENABLE_DESTRUCTIVE_HOST_OPS` (default false).
- Storage: mount/unmount, format and GPT partition endpoints. Admin + server-write + CSRF; explicit
  confirm; format/partition require typing the exact device; `/`, `/boot`, `/proc`, `/sys`, `/dev`,
  `/etc`, `/usr`, `/var`, `/run` mountpoints are refused; mounted devices are refused for
  format/partition; wildcard devices and shell metacharacters are rejected.
- Network: link up/down, address add/del, route add/del and ufw/firewalld port allow/deny. Interface,
  CIDR, gateway and port are validated; commands are enumerated; actions are audited.
- UI: destructive-operation panels in the Advanced Storage and Network tabs, disabled unless the
  server reports the feature flag; typed device confirmation for format/partition.
- Tests: unit command/validation tests plus a disposable Colima/Ubuntu run that created a loop disk,
  formatted and mounted it, created a GPT partition table, and exercised dummy interface, address,
  route and ufw commands. The container was removed afterwards.

## Decisions
- Disabled by default. Enabling is an explicit operator action.
- Even with the flag, every operation is administrator-only and audited; there is no scheduling or
  unattended destructive operation.
- These commands can still destroy data or disconnect the host; tests must use disposable targets.

## Build / Test Status
- `go test -race ./internal/advanced` destructive tests pass.
- Colima disposable test output: `colima-destructive-test: PASS`, `ufw-test: PASS`.
- `make verify` is run before merge.

## Next Steps
- OIDC/SSO and non-Linux read-only support remain.
- Final full audit of the expanded tree.
