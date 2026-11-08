# 2026-11-08 Phase 5 host monitoring

## Task / Area
Implement the agentless system overview and metrics: bounded `/proc` collection, CPU/load/memory/
filesystem/network/uptime snapshots, per-host caching and coalescing, stale fallback and the
monitoring UI.

## Current State
- Backend:
  - `internal/monitoring`: fixed `MetricsCommand`, section parser, CPU delta/rate computation,
    availability flags, per-host cache, singleflight coalescing and stale fallback.
  - `GET /api/v1/servers/{id}/overview` with `Cache-Control: no-store` and stable error mapping.
  - Partial metrics are supported: missing `/proc/stat`, `/proc/meminfo`, `/proc/loadavg`,
    `/proc/uptime`, `df` or `/proc/net/dev` marks only that section unavailable.
  - CPU usage and network rates need two samples; the first snapshot reports `null`.
  - A refresh failure inside `DROVIK_MONITOR_STALE_AFTER` returns the last snapshot marked stale
    with a warning; older failures return an error.
  - Recursive/counter-reset handling: lower counters produce unavailable usage/rates, never a
    negative or fabricated value.
  - Filesystems filter pseudo mounts (proc/sysfs/tmpfs/squashfs etc.) and truncate by config.
- Config/env: `DROVIK_MONITOR_REFRESH_INTERVAL`, `DROVIK_MONITOR_STALE_AFTER`,
  `DROVIK_MONITOR_TIMEOUT`, `DROVIK_MONITOR_MAX_OUTPUT_BYTES`,
  `DROVIK_MONITOR_MAX_FILESYSTEMS`, `DROVIK_MONITOR_MAX_INTERFACES`.
- Frontend:
  - `/monitoring` and `/monitoring/:serverId` with server selector, live 5s refresh, host identity,
    CPU/per-core, load, memory/swap, filesystem bars, network rates and uptime.
  - Explicit unavailable states per section, cached/stale badges, last-collected and boot time.
  - Monitoring route is lazy-loaded; nav item added under Observe.
- Tests:
  - Parser fixtures, OS variants, CPU delta math, first-sample nulls, counter resets, missing and
    partial `/proc`, filesystem read-only/percent behavior.
  - Service cache, coalescing, stale fallback, unavailable and authorization states, truncation.
  - HTTP overview flow: host-key conflict, trust, snapshot, cached second request.
  - Frontend metric formatting tests; 24 tests total.
- Manual browser verification against the in-process SSH fixture: host identity, memory, filesystem
  and network sections render with no console errors; first-sample CPU/rates show baseline states.

## Decisions
- One fixed collector command reads `/proc` and `df`; no caller input is interpolated and no shell
  command is built per request.
- Snapshots are not persisted. The cache exists only to bound remote polling and is per process.
- Missing values use explicit availability flags and `null` rates, never zero.
- Total collection failure with no previous sample returns `CAPABILITY_UNAVAILABLE`; a recent
  previous sample degrades to `stale`.

## Build / Test Status
- `make verify`: pass (Go race tests/lint; frontend lint/typecheck/24 tests/build).
- Manual overview review passed against the fixture; CPU deltas were covered by unit tests because
  the fixture counters are static.
- Not run: a matrix of real Linux distributions and extremely large `/proc/mounts`/interface sets.

## Next Steps
- Phase 6: process discovery/signals, systemd units/actions and bounded journal logs.
- Add a compact multi-server overview once process/service pages exist.
- Add Playwright coverage for the monitoring refresh and unavailable states.

## Open Questions
- Whether to expose a configurable polling pause when the tab is hidden (currently React Query
  keeps the 5s interval while mounted).
- Whether a future release should persist short rolling history; deliberately deferred.
