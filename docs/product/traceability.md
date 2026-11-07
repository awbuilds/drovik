# Requirement Traceability

**Status:** living document
**Last updated:** 2026-11-07
**Source:** [PRD](PRD.md)

A requirement is only marked **Done** when its implementation, authorization, failure paths,
tests and documentation are complete. A route or screen existing is not sufficient.

Status values: **Not started**, **In progress**, **Implemented**, **Tested**, **Deferred**,
**Out of scope**.

## Phase 1 — Setup and authentication

| ID | Requirement | Status | Implementation | Tests |
|---|---|---|---|---|
| SET-001 | First-run detection | In progress | `backend/internal/auth`, `/api/v1/setup/status` | pending |
| SET-002 | Initial administrator creation | In progress | `POST /api/v1/setup` | pending |
| SET-003 | One-time setup disabled after initialization | In progress | setup guard | pending |
| SET-004 | Secure password storage (Argon2id) | In progress | `backend/internal/auth/password.go` | pending |
| SET-005 | Configurable session expiry and logout | In progress | `backend/internal/auth/session.go` | pending |
| SET-006 | Version/build/schema identity in settings | In progress | `/api/v1/version`, settings page | pending |
| SET-007 | Configuration validation; no secret logging | In progress | `backend/internal/config`, logging redaction | pending |
| SET-008 | UTC storage, local display | In progress | store UTC; frontend formatting | pending |
| SET-009 | Backup configuration documented | Planned | deployment docs | pending |
| SET-010 | Safe defaults, no servers configured | In progress | first-run state | pending |
| AUTH-001 | Unauthenticated users cannot access server data | In progress | auth middleware | pending |
| AUTH-002 | Login rate limiting | In progress | rate limiter | pending |
| AUTH-003 | Constant-time password verification | In progress | Argon2id library | pending |
| AUTH-004 | HttpOnly/SameSite/Secure cookies | In progress | session cookie | pending |
| AUTH-005 | CSRF protection | In progress | CSRF middleware | pending |
| AUTH-006 | Logout invalidates server session | In progress | session revocation | pending |
| AUTH-007 | Expired/revoked sessions rejected (HTTP + WS) | In progress | session middleware | pending |
| AUTH-008 | WebSocket session + Origin validation | Planned | Phase 3 | pending |
| AUTH-009 | Backend authorization | In progress | middleware/handlers | pending |
| AUTH-010 | Per-user server access (multi-user) | Planned | data model ready | pending |
| AUTH-011 | Read-only role blocked from mutations | Planned | Phase 9 | pending |
| AUTH-012 | Recent auth for sensitive changes | Planned | Phase 1+ | pending |
| AUTH-013 | Password change revokes other sessions | Planned | Phase 1+ | pending |
| AUTH-014 | No user enumeration | In progress | generic login errors | pending |

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
| Audit | AUD-001–009 | Not started |
| Notifications | MSG-001–007 | Not started |
| Performance | NFR-PERF-001–007 | Not started |
| Reliability | NFR-REL-001–007 | Not started |

## How to update

1. Change the status only when the linked evidence exists.
2. Add the implementation path(s) and test name(s).
3. Link the PR.
4. Note any deliberate deviation and its ADR or issue.
