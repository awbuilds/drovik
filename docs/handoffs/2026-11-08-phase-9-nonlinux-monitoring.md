# 2026-11-08 Phase 9 non-Linux read-only monitoring

## Task / Area
Add read-only macOS/BSD host monitoring without weakening the Linux path.

## Current State
- `MetricsCommand` now also emits portable sections: `sysctl` boot time/load/CPU count/memory/page
  size, `vm_stat`, `sw_vers`, `top` (macOS CPU sample), `mount` and `netstat -ibn`.
- `internal/monitoring/posix.go` detects Darwin/BSD from `uname` and parses those sections:
  host identity, load, uptime, memory, filesystems (including APFS read-only mounts), network
  counters and a point-in-time macOS CPU usage. Unsupported sources stay individually unavailable.
- Linux parsing is unchanged; the POSIX sections are ignored while `/proc` is present.
- Capability detection reports `macOS <version>` for Darwin.
- The Monitoring UI shows a read-only macOS/BSD notice and states that systemd, journal and
  Linux-only actions are unavailable.
- Tests: macOS fixture (host, load, memory, CPU, APFS read-only, netstat) and FreeBSD fixture
  (host, load, uptime, UFS, FreeBSD netstat column layout).

## Decisions
- Only read-only monitoring is claimed for macOS/BSD. Terminal/SFTP use standard SSH but are outside
  the validated matrix; systemd, journal, Docker and Linux advanced actions are not claimed.
- macOS CPU usage is a `top -l 1` point-in-time sample, not a delta; per-core counters are absent.
- No new dependency: parsers are stdlib-only and every optional source degrades individually.

## Build / Test Status
- `go test ./internal/monitoring` passes, including both new fixtures.
- `make verify` is run before merge.
- Not run: a live macOS or FreeBSD SSH target. The fixtures cover the documented command output.

## Next Steps
- Final full audit of the expanded tree, then a release/site refresh.
