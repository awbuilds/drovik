# 2026-11-08 Phase 9 expansion — packages and tasks

## Task / Area
Extend Phase 9 beyond read-only diagnostics with package lifecycle and managed scheduled tasks.

## Current State
- Package lifecycle: `POST /api/v1/servers/{id}/packages/action` with install/upgrade/remove for
  detected apt, dnf, apk or pacman. Package names are strictly validated, confirmation is required,
  commands are enumerated, no sudo is attempted, output is bounded and every action is audited.
- Scheduled tasks: `POST /servers/{id}/tasks` and `DELETE /servers/{id}/tasks/{taskID}?confirm=true`.
  Drovik writes a marked block in the SSH user's own crontab. Schedule and command are validated
  (five cron fields, no newlines/NUL/percent, 500-byte cap), the full crontab is staged over SFTP
  and installed with `crontab`, existing entries are preserved and changes are audited.
- UI: package install/upgrade/remove dialog with confirmation and remote output; managed-task list,
  add dialog and delete confirmation in the Advanced page.
- Tests: package manager mapping, command construction, confirmation, invalid names, failure
  output; task lifecycle, crontab preservation, validation and schedule parsing.
- Docs: ADR 0013, PRD status, traceability, CHANGELOG and OpenAPI updated.

## Decisions
- Package actions run as the connected SSH user; a non-root account sees the remote permission
  failure. No privilege escalation.
- Task editing is limited to a Drovik-marked block in the user's crontab. System crontabs and
  systemd unit files are not edited.

## Build / Test Status
- `make verify` passed for the increment (Go race/vet/lint; frontend lint/typecheck/tests/build).
- Not run: a real apt/dnf/apk/pacman host matrix and a real crontab round-trip on a host.

## Next Steps
- Notifications (SSRF-safe webhook, alert cooldown), local MFA (TOTP), OIDC/SSO.
- Remaining destructive storage/network actions and non-Linux targets need separate design.
