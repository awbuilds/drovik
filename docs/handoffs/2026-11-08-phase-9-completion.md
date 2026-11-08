# 2026-11-08 Phase 9 completion — teams, audit export, snippets

## Task / Area
Close the remaining non-destructive Phase 9 gaps on an unmerged branch.

## Current State
- Teams/groups: migration `00009_teams.sql`; admin-managed teams, members and team-server
  `read`/`write` assignments. Effective access is additive with direct assignments and owners.
  Storage, service and HTTP tests cover membership, access resolution and admin-only management.
- Audit: server-side text/event/outcome/time filters, accessible-server scoping including
  assignments and teams, and bounded CSV/JSON export. CSV formula characters are prefixed.
- Snippets: versioned JSON export and bounded transactional import (1–100, duplicate names
  rejected, full validation before insert).
- Non-Linux: once capabilities report a non-Linux OS, every `requireServerWrite` route returns
  `CAPABILITY_UNAVAILABLE`; read-only monitoring still works. macOS collector validated against the
  live local macOS host; FreeBSD remains fixture-based.
- UI: Teams page/nav, audit search/export controls, snippet import/export controls.

## Decisions
- Teams are administrative groups, not principals: no team login, no team-owned servers, no team
  role. Team access is additive and the strongest level wins.
- Audit exports are bounded to 5,000 rows and audited.
- Snippet imports are all-or-nothing and never overwrite an existing name.
- Systemd unit/timer editing is deliberately excluded as a root-level persistent host mutation.

## Build / Test Status
- `go test -race ./...` passes, including new teams, audit-export, snippet-import/export and
  non-Linux write-gate tests.
- Live macOS collector test passes locally.
- `make verify` is run before handoff.
- Not run: live FreeBSD/OpenBSD/NetBSD, a real team deployment with multiple administrators.

## Next Steps
- Review on the branch; do not merge or tag until the owner approves the release.
