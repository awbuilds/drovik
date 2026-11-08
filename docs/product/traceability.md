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


## Phase 2 — SSH connections and host-key trust

| ID | Requirement | Status | Implementation / evidence |
|---|---|---|---|
| SSH-001 | Multiple servers | Tested | `servers.Service`, `ServerRepository`; HTTP and service tests |
| SSH-002 | Connection fields | Tested | `servers.CreateInput`, `servers.Server`; form and API |
| SSH-003 | Default port 22 | Implemented | `validateCreate` defaults port |
| SSH-004 | Password and private-key auth | Tested | `sshclient.authMethods`; password and private-key tests |
| SSH-005 | Key formats and useful errors | Implemented | OpenSSH/PEM via `ssh.ParsePrivateKey`; generic safe error |
| SSH-006 | Strict host-key verification | Tested | `sshclient.Dialer` callback; `TestDialPasswordAuthenticatedHost` |
| SSH-007 | Unknown host key flow | Tested | `HostKeyUnknownError`; HTTP 409 + approval test |
| SSH-008 | Changed host key blocks | Tested | `HostKeyChangedError`; `TestDialChangedHostKey` |
| SSH-009 | Persistent known-hosts store | Tested | `known_hosts` migration/repository; service test |
| SSH-010 | Connection test | Tested | `/servers/{id}/test`; HTTP and service tests |
| SSH-011 | Bounded timeouts | Implemented | dial/handshake/operation config; connection deadline |
| SSH-012 | Session limits | Partial | config limits; enforced with the Phase 3 session pool |
| SSH-013 | Resource cleanup | Tested | clients/sessions closed in tests and defer paths |
| SSH-014 | Reconnect policy | Deferred | Phase 3 terminal/session lifecycle |
| SSH-015 | Connection states | Partial | saved/untested/verified in UI; live states Phase 3 |
| SSH-016 | No silent credential fallback | Implemented | single selected auth method; no retry alternatives |
| SSH-017 | No agent forwarding | Implemented | not requested or exposed |
| SSH-018 | No port forwarding | Implemented | not exposed |
| SSH-019 | Remote identity display | Tested | UI shows `user@host:port`; manual browser review |
| SSH-020 | Delete removes credentials | Tested | cascade delete; `TestDeleteServer` |
| SSH-021 | Edit revalidates trust | Tested | host/port change revokes known host; `TestUpdateHostRevokesTrust` |
| SSH-022 | Tags/groups | Implemented | tags stored and displayed; groups deferred |
| SSH-023 | Encrypted import/export | Deferred | post-MVP |
| SSH-024 | Egress policy | Tested | `security.EgressPolicy`; unit and HTTP tests |
| MON-015 | Capability detection | Tested | `DetectCapabilities`; SSH and HTTP tests |
| DESK-003 | Switch saved servers | Implemented | server list and workspace preview |
| DESK-006 | Every server window shows identity | Implemented | server cards and test dialog |

### Phase 2 notes

- Host-key approval is explicit TOFU: the untrusted key details are returned in the 409 response,
  and the operator must submit the same fingerprint to approve it.
- A changed key is never accepted automatically; `replace: true` is required after re-verification.
- Credentials are encrypted with AES-256-GCM and associated data binding the server ID and field.
- The egress policy resolves once, blocks loopback/link-local/metadata by default, and allows
  explicit operator overrides. Loopback is allowed in tests only via an explicit test configuration.
- Live connection-state transitions, reconnect policy and the bounded session pool land with the
  Phase 3 terminal work.


## Phase 3 — Workspace and interactive terminal

| ID | Requirement | Status | Implementation / evidence |
|---|---|---|---|
| TERM-001 | Interactive SSH PTY | Tested | `terminal.Manager` requests a PTY; `TestTerminalWebSocket` |
| TERM-002 | xterm.js terminal | Implemented | `TerminalPanel` with `@xterm/xterm`; manual review |
| TERM-003 | Bidirectional I/O | Tested | WebSocket input/output test |
| TERM-004 | Resize propagation | Implemented | resize envelope + `ssh.Session.WindowChange`; test sends resize |
| TERM-005 | Multiple sessions | Tested | per-user/global manager limits; `TestAcquireLimits` |
| TERM-006 | Server identity | Implemented | Terminal header shows `user@host:port` |
| TERM-007 | Session lifecycle | Tested | close/EOF cleanup and manager close |
| TERM-008 | Disconnect handling | Implemented | UI reconnect state; no buffered replay |
| TERM-009 | Safe output handling | Implemented | output written only to xterm; JSON text envelopes |
| TERM-010 | WebSocket authentication | Tested | `TestTerminalRequiresAuthentication` |
| TERM-011 | Origin validation | Tested | `TestTerminalRejectsUntrustedOrigin` |
| TERM-012 | Message size/rate limits | Implemented | 64 KiB read limit; 200 msg/s input cap |
| TERM-013 | No transcript storage | Implemented | no terminal content persisted |
| TERM-014 | Explicit close | Implemented | close envelope and UI button |
| TERM-015 | Paste safety warning | Deferred | SHOULD; follow-up |
| TERM-016 | Clipboard permissions | Implemented | browser/xterm behavior |
| TERM-017 | No command rewriting | Implemented | input written verbatim |
| TERM-018 | TERM environment | Implemented | `xterm-256color` |
| TERM-019 | Shell exit reporting | Partial | closed status; exit code not surfaced |
| TERM-020 | Cleanup on disconnect/shutdown | Implemented | manager close, PTY/channel/client cleanup |

### Phase 3 notes

- The terminal WebSocket requires both the session cookie and a trusted `Origin`; the Origin check
  is performed before the upgrade.
- Input is bounded by a 64 KiB read limit and a per-second message cap. Output is chunked.
- The session pool is bounded per user and globally; shutdown closes registered sessions.
- Terminal content is never written to the database by default.

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
