# Requirement Traceability

**Status:** living document
**Last updated:** 2026-11-07
**Source:** [PRD](PRD.md)

A requirement is only marked **Done** when its implementation, authorization, failure paths,
tests and documentation are complete. A route or screen existing is not sufficient.

Status values: **Not started**, **In progress**, **Implemented**, **Tested**, **Deferred**,
**Out of scope**.

## Phase 1 — Setup and authentication

| ID | Requirement | Status | Implementation | Evidence |
|---|---|---|---|---|
| SET-001 | First-run detection | Tested | `/api/v1/setup/status` | `TestSetupLoginLogoutFlow` |
| SET-002 | Initial administrator creation | Tested | `POST /api/v1/setup` | `TestSetupLoginLogoutFlow` |
| SET-003 | One-time setup disabled after initialization | Tested | `CreateFirstAdmin` transaction + setup guard | `TestSetupLoginLogoutFlow` |
| SET-004 | Secure password storage (Argon2id) | Tested | `internal/auth/password.go` | `TestPasswordHashAndVerify`, `TestPasswordHashesAreSalted` |
| SET-005 | Configurable session expiry and logout | Tested | `internal/auth/session.go` | `TestSessionIdleExpiry`, `TestSessionRevoke` |
| SET-006 | Version/build/schema identity | Tested | `/api/v1/version`, Settings page | `TestVersionAndSecurityHeaders` |
| SET-007 | Configuration validation; no secret logging | Tested | `internal/config`, redacting logger | `config_test.go`, `TestRedaction` |
| SET-008 | UTC storage, local display | Implemented | UTC timestamps; frontend formatters | `formatDateTime`/`formatRelativeTime` |
| SET-009 | Backup configuration documented | Implemented | `deploy/README.md` | Manual review |
| SET-010 | Safe defaults, no servers configured | Implemented | Empty server catalog; first-run workspace | Workspace empty state |
| AUTH-001 | Unauthenticated users cannot access protected data | Tested | `requireAuth` middleware | `TestSetupLoginLogoutFlow` |
| AUTH-002 | Login rate limiting | Tested | `internal/security.LoginLimiter` | `TestLoginAndRateLimit` |
| AUTH-003 | Constant-time password verification | Tested | Argon2id + `subtle.ConstantTimeCompare` | `TestPasswordHashAndVerify` |
| AUTH-004 | HttpOnly/SameSite/Secure cookies | Tested | `SessionManager.SetSessionCookie` | `TestSessionCookie` |
| AUTH-005 | CSRF protection | Tested | Session-bound synchronizer token + Origin checks | `TestSetupLoginLogoutFlow`, `TestCSRFToken`, `TestOriginAndContentTypeValidation` |
| AUTH-006 | Logout invalidates server session | Tested | `RevokeByToken` + cookie clearing | `TestSetupLoginLogoutFlow` |
| AUTH-007 | Expired/revoked sessions rejected | Tested (HTTP) | Session validation | `TestSessionIdleExpiry`, `TestSessionRevoke` |
| AUTH-008 | WebSocket session + Origin validation | Deferred | Phase 3 WebSocket work | — |
| AUTH-009 | Backend authorization | Implemented | Auth middleware; server routes arrive in Phase 2 | `TestSetupLoginLogoutFlow` |
| AUTH-010 | Per-user server access | Deferred | Phase 2+ multi-user data model | — |
| AUTH-011 | Read-only role blocked from mutations | Deferred | Phase 9 | — |
| AUTH-012 | Recent auth for sensitive changes | Deferred | Phase 1+ follow-up | — |
| AUTH-013 | Password change revokes other sessions | Deferred | Phase 1+ follow-up | — |
| AUTH-014 | No user enumeration | Tested | Generic errors + dummy verification | `TestLoginAndRateLimit` (generic 401) |

## Later phases

The remaining requirement groups are tracked in the PRD and will be added here when their phase
starts. Summary:

| Group | IDs | Status |
|---|---|---|
| Desktop/workspace | DESK-001–012 | Not started |
| SSH connections | SSH-001–024 | Not started |
| SFTP files | FILE-001–030 | Not started |
| Terminal | TERM-001–020 | Not started |
| Monitoring | MON-001–016 | Not started |
| Processes | PROC-001–013 | Not started |
| Services | SVC-001–014 | Not started |
| Logs | LOG-001–012 | Not started |
| Audit | AUD-001–009 | Partially: login/setup/logout events |
| Notifications | MSG-001–007 | Not started |
| Performance | NFR-PERF-001–007 | Not started |
| Reliability | NFR-REL-001–007 | Not started |

## How to update

1. Change the status only when the linked evidence exists.
2. Add the implementation path(s) and test name(s).
3. Link the PR.
4. Note any deliberate deviation and its ADR or issue.
