# 2026-11-08 Phase 6 processes, services and logs

## Task / Area
Implement process discovery/signals, systemd unit management and bounded journal retrieval with the
matching UI pages.

## Current State
- Backend:
  - `internal/processes`: fixed `ps -eo` collector, bounded list, SIGTERM/SIGKILL only, start-time
    identity recheck, post-signal status, audit events and stable errors.
  - `internal/services`: `systemctl list-units` + `list-unit-files`, unit merge, enumerated actions
    (start/stop/restart/reload/enable/disable), unit regex validation, `systemctl show` re-read and
    audit events.
  - `internal/logs`: journal JSON retrieval, fixed time-range enum, validated unit, bounded lines and
    output, timestamp/priority/severity parsing and permission mapping.
  - Routes: `/servers/{id}/processes`, `/processes/{pid}/signal`, `/services`,
    `/services/{unit}/action`, `/logs`. Mutations require CSRF.
  - `internal/testenv` provides a shared authorized integration fixture; `testssh` has a command
    hook for realistic operation fixtures.
- Frontend:
  - Processes page: filter, sort, state indicators, auto/manual refresh, signal dialog with SIGKILL
    acknowledgement and post-action result toast.
  - Services page: search, active/sub/file-state display, action buttons with confirmation and
    state-known handling.
  - Logs page: unit/time/line controls, text filter, timestamp/severity display and text-only log
    rendering.
  - New Observe/Operate navigation entries; all three pages are lazy-loaded.
- Config/env: `DROVIK_OPS_COMMAND_TIMEOUT`, `DROVIK_PROCESS_MAX_ENTRIES`,
  `DROVIK_SERVICE_MAX_UNITS`, `DROVIK_LOG_MAX_LINES`, `DROVIK_LOG_MAX_OUTPUT_BYTES`.
- Tests:
  - Process parser, list/signal, reused PID, permission denied, validation.
  - Service parser/merge, list/action, permission, unavailable host, unit validation.
  - Journal JSON/byte-array parsing, range/unit/line validation, permission/unavailable, XSS text
    preservation.
  - HTTP end-to-end flow for all five endpoints including CSRF.
  - Frontend operation helper tests; 27 frontend tests total.
- Manual browser review: process list and signal success, service restart with confirmation and
  resulting state, journal window with a literal `<script>` payload rendered as text.

## Decisions
- Process identity uses the start time derived from `etimes` with a 3-second tolerance; a changed
  process returns `PROCESS_PID_REUSED` and is never signaled.
- PID 1 is rejected by the backend and disabled in the UI.
- Only SIGTERM and SIGKILL are exposed. SIGKILL requires a separate explicit acknowledgement.
- Service units must match a strict `.service` identifier pattern; actions are an enumeration, and
  Drovik never uses sudo or auto-escalates.
- Journal entries are parsed from `--output=json` and never persisted; the frontend renders every
  message as text (React escaping), confirmed with a script payload fixture.

## Build / Test Status
- `make verify` passes (Go race tests/lint; frontend lint/typecheck/27 tests/build).
- Manual browser flows for processes, services and logs passed with no page errors.
- Not run: a real systemd host matrix, polkit-denied action matrix and Playwright coverage.

## Next Steps
- Phase 7: audit history page/API, cross-module server-context review, accessibility and responsive
  pass, security test suite, beta release assets.
- Add integration tests against a real Linux VM for `ps`, systemctl and journal variants.
- Consider virtualization or pagination for very large process/service lists.

## Open Questions
- Whether to expose SIGINT/SIGHUP later; currently deliberately limited.
- Whether to allow journal priority filtering server-side; current filtering is client-side within
  the bounded window.
