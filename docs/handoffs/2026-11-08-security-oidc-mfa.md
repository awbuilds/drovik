# 2026-11-08 Security fixes — OIDC link, MFA setup, webhook logging

## Task / Area
Remediate confirmed findings from the final run-4 audit before the v1.3.0 release.

## Current State
- OIDC link start now requires the local password and, when MFA is enabled, a current TOTP/recovery
  code. The pending state records the initiating session ID; the callback must present the state
  cookie and the same live session. A cross-browser callback is rejected before the state is
  consumed.
- OIDC start/callback use a short-lived HttpOnly SameSite state cookie and `no-store`.
- MFA setup requires the password; replacing an enabled factor also requires a current code. A
  rejected replacement keeps the existing factor enabled.
- Webhook delivery errors are sanitized and never include the URL. `MarkNotified` is a conditional
  claim so concurrent overview requests cannot double-deliver an alert.
- Destructive host operations record `unknown`/`OUTCOME_UNKNOWN` on cancellation or timeout.
- `/api/v1/version` now assigns `destructive_host_ops` and `vault_enabled`.
- Tests cover the step-up requirements, cross-browser link rejection, webhook URL redaction,
  notification claim atomicity, destructive cancellation audit and version defaults.

## Decisions
- Link and re-enrollment are treated as sensitive account changes requiring fresh local proof.
- The OIDC browser binding is an HttpOnly SameSite=Lax cookie scoped to the callback path; state
  remains single-use and five-minute bounded.
- A successful re-enrollment still disables the old factor until the new one is confirmed; the
  step-up requirement is the security boundary, not a pending-secret schema change.

## Build / Test Status
- `go test -race ./internal/oidc ./internal/httpapi ./internal/advanced` passes.
- `make verify` passes.
- Final run-4 verification is being re-run on this tree; no live IdP, TOTP device or Vault was used.

## Next Steps
- Merge, re-verify the final tree, finish the run-4 report and tag v1.3.0.
