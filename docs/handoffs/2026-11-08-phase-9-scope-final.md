# 2026-11-08 Phase 9 scope finalized

## Task / Area
Record the user-approved decision that the read-first Phase 9 scope is permanent for the 1.x line.

## Current State
- ADR 0013 is now "Accepted — final Phase 9 scope (approved 2026-11-08)".
- Permanently out of scope: mount/format/partition, network/firewall changes, package
  install/upgrade/remove, cron/timer editing, OIDC/MFA/external secrets, external notifications and
  non-Linux managed hosts.
- Traceability rows for those items changed from `Deferred` to `Out of scope` with the ADR
  reference; PRD Phase 9 carries the same implementation-status note.
- ROADMAP, AGENTS.md, stable notes and the handoff are aligned.
- No code changed in this decision; the final code tree remains `v1.1.1` (`7d7ed48`) with the
  Corepack build follow-up (`0be160d`).

## Decisions
- The read-first boundary is final. The excluded items are not pending deliverables and must not be
  used as completion criteria for Phase 9.
- Any future work on an excluded item requires a new ADR and threat model, plus its own validation.

## Build / Test Status
- Documentation-only change; `make verify` is run before merge.
- The final audit run (run-3) is executed after this documentation merge on the resulting tree.

## Next Steps
- Merge this decision, then run the full `security-audit` workflow on the final tree, verifying the
  seven `v1.1.1` remediations and re-checking the carried validation leads.
- Continue owner-observed real-host Docker/PolicyKit/browser validation as operational work.
