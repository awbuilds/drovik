# ADR 0007 — Structured remote operations

- **Status:** Accepted
- **Date:** 2026-11-07

## Context

Drovik executes operations on remote Linux hosts: metrics, process signals, service actions and
journal retrieval. Building shell commands from arbitrary user input would create a command
injection vulnerability. The interactive terminal is intentionally arbitrary and is treated as a
separate, privileged capability.

## Decision

- Use predefined operation handlers with validated, typed parameters.
- Enumerated actions only: service `start|stop|restart|reload|enable|disable`; signals from an
  allowlist; bounded log line counts and time ranges.
- Validate service unit names against the discovered unit list or a strict identifier pattern.
- Validate PIDs as positive integers and recheck identity before signaling where practical.
- Prefer direct `/proc`, SFTP or exec APIs over shell interpretation. Where a shell is unavoidable,
  use fixed templates and rigorously validated arguments; do not rely on a generic quote helper as
  a substitute for design.
- Bound output size, execution time and concurrency. Support cancellation.
- Never run `sudo` automatically.
- Re-read remote state after a mutating action and report `OUTCOME_UNKNOWN` when it cannot be
  verified.
- Do not expose a generic “run arbitrary command” API. The terminal WebSocket is the only arbitrary
  command path and requires an authenticated, authorized session.

## Consequences

- Adding a new operation requires a deliberate handler, validation and tests.
- Metrics and service parsing must handle distribution differences.
- The UI cannot send ad-hoc commands outside the terminal, by design.
- Audit metadata can describe operations precisely.

## Alternatives considered

- Shell command builder with quoting: rejected; too easy to get wrong.
- Generic SSH exec endpoint: rejected; violates the product security boundary.

## References

PRD §5.5, PROC-009, SVC-011, LOG-007; AGENTS.md.
