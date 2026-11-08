# 2026-11-08 Security audit run 2 (Phase 9)

## Task / Area
Re-audit the Phase 9 tree (`v1.1.0`, `5771341`), fix confirmed findings, and remediate the GitHub
code-scanning/scorecard alerts.

## Current State
- Audit artifacts: `~/security-audit-skill/drovik/run-2/` (`REPORT.md`, `FINDINGS-DETAIL.md`,
  `NEEDS-VALIDATION.md`, `findings.json`, `coverage-ledger.json`, `evidence-run-2.md`).
- Confirmed and fixed in the `v1.1.1` release:
  - Snippet substitution could expand one request into gigabytes (bounded builder, 4 KiB value cap,
    declaration validation).
  - Concurrent administrator deactivation could leave zero active admins (statement-level guard,
    `ErrLastAdmin` mapping).
  - Collector section assembly was quadratic (linear builders, 4096-line section cap).
  - Failed post-signal state reads were reported as stopped with success audit (`outcome_unknown`,
    unknown audit outcome).
  - Delivered signals could lose their audit row on cancellation (detached bounded audit context).
  - Non-finite host metrics could be persisted and turn APIs into empty 200s (finiteness checks,
    marshal-before-status JSON).
  - Concurrent overview requests persisted duplicate samples (unique index + `ON CONFLICT DO
    NOTHING`, migration 00004).
- Additional hardening: Docker inspect labels removed, Docker image and alert-rule caps, alert-rule
  ID validation, `no-store` on advanced responses, read-only UI gating, startup retention prune.
- GitHub alerts: top-level workflow write permissions moved to the job, container base images pinned
  by digest, `golang.org/x/crypto` upgraded past GO-2026-5932.
- The five `v1.0.1` fixes were re-verified and remain intact.
- Eleven `needs_validation` leads remain, including a new one: removing an assignment does not close
  an established terminal.

## Decisions
- `v1.1.1` is a security patch release; no functional scope changes.
- Branch-protection scorecard alert 1 is accepted as a single-maintainer limitation (no second
  approver); ruleset still requires PR, checks, linear history and conversation resolution.
- Snippet variables are capped at 4 KiB each and the rendered command at 64 KiB.

## Build / Test Status
- `make verify` passes (Go race tests/vet/lint; frontend lint/typecheck/tests/build).
- New regression tests: snippet expansion and declaration, section line cap, Docker image cap,
  alert-rule ID/cap, process/service unknown outcomes and audit rows, non-finite load/uptime,
  non-finite persistence, sample deduplication, last-admin SQL guard, fail-visible JSON.
- Independent verifier confirmed all seven records against `5771341`.

## Next Steps
- Merge, tag `v1.1.1`, verify release assets/image; confirm Scorecard alerts 13/14–18/24/25 close.
- Work the remaining validation leads; prioritize assignment revocation for live terminals,
  terminal/SFTP deadlines, recursive-delete symlink safety and shared SSH accounting.
- Continue real-host Docker/PolicyKit/browser validation.

## Open Questions
- Whether to disable recursive delete until a no-follow primitive is available.
- Whether the frontend should force a central session-expired transition on any 401.
