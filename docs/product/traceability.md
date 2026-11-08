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


## Phase 4 — SFTP file manager and editor

| ID | Requirement | Status | Implementation / evidence |
|---|---|---|---|
| FILE-001 | Directory listing | Tested | `files.Service.List`; HTTP/service tests |
| FILE-002 | Navigation | Implemented | Breadcrumbs and directory navigation |
| FILE-003 | Metadata | Implemented | Name/type/size/mode/mtime/UID/GID |
| FILE-004 | Sorting | Implemented | Frontend name/type/size/modified sorting |
| FILE-005 | Hidden files | Implemented | Client-side dotfile toggle |
| FILE-006 | Upload | Tested | Multipart streaming upload; HTTP test |
| FILE-007 | Download | Tested | Streamed download; HTTP test |
| FILE-008 | Upload progress/cancel | Implemented | XHR progress + abort signal |
| FILE-009 | Download progress | Partial | Browser download manager; no in-app cancel |
| FILE-010 | Create folder | Tested | `Mkdir`; HTTP test |
| FILE-011 | Create file | Tested | `Touch`; HTTP test |
| FILE-012 | Rename | Tested | `Rename`; service/HTTP tests |
| FILE-013 | Move | Implemented | SFTP rename semantics |
| FILE-014 | Delete / recursive guard | Tested | Recursive flag + bounded traversal; tests |
| FILE-015 | Permissions inspection | Partial | Mode shown; chmod deferred |
| FILE-016 | Text editor | Implemented | `FileEditorDialog` + bounded read/write |
| FILE-017 | Explicit save | Implemented | Save writes only on action |
| FILE-018 | Unsaved changes prompt | Implemented | Discard confirmation dialog |
| FILE-019 | Editor size limit | Tested | `ErrTooLarge`; service test |
| FILE-020 | Binary detection | Tested | NUL/UTF-8 check; service test |
| FILE-021 | Transfer collision | Implemented | `409 FILE_CONFLICT` + overwrite confirmation |
| FILE-022 | Path safety | Tested | Absolute-only normalization; invalid-path test |
| FILE-023 | Symlink handling | Tested | Lstat, no follow for editor/recursive; test |
| FILE-024 | Partial transfer | Implemented | Temp file + rename, cleanup on error |
| FILE-025 | Permission feedback | Implemented | `SFTP_PERMISSION_DENIED` mapping |
| FILE-026 | SFTP unavailable | Implemented | Typed `SFTP_UNAVAILABLE` error |
| FILE-027 | No external upload | Implemented | Content stays in memory/stream |
| FILE-028 | User-selected files | Implemented | Browser file input only |
| FILE-029 | Refresh | Implemented | Refresh button |
| FILE-030 | Stale editor detection | Tested | mtime + size check; `TestFilesHTTPFlow` stale case |

### Phase 4 notes

- All remote paths must be absolute; `path.Clean` normalizes them and the SFTP user's permissions
  remain the authority. No shell is used.
- Writes and uploads use a temporary remote file and rename to reduce partial-file exposure.
- Recursive delete never follows symlinks and is bounded by entry count and depth.
- Editor content is bounded by `DROVIK_SFTP_MAX_EDITOR_BYTES`; uploads by
  `DROVIK_SFTP_MAX_UPLOAD_BYTES`.
- Download progress is delegated to the browser's download manager; no in-app transfer job is
  implemented.


## Phase 5 — System overview and monitoring

| ID | Requirement | Status | Implementation / evidence |
|---|---|---|---|
| MON-001 | Server overview | Tested | `/servers/{id}/overview`; HTTP test |
| MON-002 | Host identity | Tested | hostname, OS/distribution, kernel, architecture, uptime; parser tests |
| MON-003 | CPU | Tested | `/proc/stat` deltas; first-sample and reset tests |
| MON-004 | Memory | Tested | `/proc/meminfo`; total, available, used, free, buffers, cached, swap |
| MON-005 | Storage | Tested | `df -Pk` plus `/proc/mounts`; usage tests |
| MON-006 | Network | Tested | `/proc/net/dev` counters and rate deltas; reset test |
| MON-007 | Load | Tested | `/proc/loadavg`; parser tests |
| MON-008 | Refresh | Implemented | 5s UI auto-refresh and configurable minimum interval |
| MON-009 | Missing data | Tested | `available` flags; partial-metrics test |
| MON-010 | Polling limits | Tested | singleflight per host + cache; concurrency test |
| MON-011 | Timeout | Implemented | `DROVIK_MONITOR_TIMEOUT` and context deadline |
| MON-012 | Failure isolation | Tested | Missing sections are marked unavailable; partial-metrics test |
| MON-013 | No agent | Implemented | Short-lived SSH command over existing connection |
| MON-014 | Data freshness | Implemented | Collected timestamp, cached badge, stale warning |
| MON-015 | Capability detection | Tested | Missing `/proc` sections; unavailable-state tests |
| MON-016 | No invented precision | Implemented | CPU/rate values are `null` until measured; unavailable shown as such |

### Phase 5 notes

- One fixed, enumerated collector command reads `/proc` and `df`; no caller input is interpolated.
- CPU usage and network rates require two samples. The first snapshot reports `null`, never zero.
- Snapshots are cached for `DROVIK_MONITOR_REFRESH_INTERVAL`; concurrent requests are coalesced
  with a per-host singleflight group.
- A failed refresh inside `DROVIK_MONITOR_STALE_AFTER` returns the last successful snapshot marked
  `stale` with a warning; older failures surface as an error.
- Metrics are not persisted as historical time series.


## Phase 6 — Processes, services and logs

| ID | Requirement | Status | Implementation / evidence |
|---|---|---|---|
| PROC-001 | Process list | Tested | `processes.Service.List`; service/HTTP tests |
| PROC-002 | Identity fields | Tested | PID/PPID/user/state/CPU/memory/RSS/uptime/command; parser test |
| PROC-003 | Sorting and refresh | Implemented | Client sorting; 10s auto + manual refresh |
| PROC-004 | Filter/search | Implemented | Client filter over the bounded list |
| PROC-005 | Remote account context | Implemented | Account badge and page description |
| PROC-006 | SIGTERM | Tested | Enumerated signal; service/HTTP tests |
| PROC-007 | SIGKILL confirmation | Implemented | Separate signal + explicit checkbox |
| PROC-008 | Honest permissions | Tested | Permission mapping; denied-signal test |
| PROC-009 | Structured parameters | Tested | Validated PID and fixed kill command; no shell concatenation |
| PROC-010 | PID identity recheck | Tested | Start-time recheck; reused-PID test |
| PROC-011 | Data-loss warning | Implemented | Signal dialog warning |
| PROC-012 | Post-action check | Implemented | Running/state returned and list invalidated |
| PROC-013 | Failed/denied display | Implemented | Stable error codes and dialog errors |
| SVC-001 | systemd detection | Tested | `systemctl` list failure maps to unavailable; test |
| SVC-002 | Unit discovery/state | Tested | list-units + list-unit-files merge; parser test |
| SVC-003 | Distinct states | Implemented | Active/sub/file-state badges |
| SVC-004 | Service details | Implemented | Unit name, description, states |
| SVC-005 | Start/stop/restart | Tested | Enumerated actions; HTTP test |
| SVC-006 | Reload | Implemented | Enumerated action |
| SVC-007 | Enable/disable | Implemented | Enumerated actions with confirmation |
| SVC-008 | Confirmation | Implemented | Action dialog with impact copy |
| SVC-009 | Re-read after action | Tested | `systemctl show`; action test |
| SVC-010 | No escalation | Implemented | `--no-ask-password`, no sudo |
| SVC-011 | Unit validation | Tested | Regex validation before use; test |
| SVC-012 | Unsupported states | Implemented | Unavailable alert and no retry as another user |
| SVC-013 | Safe action errors | Implemented | Stable codes; generic client messages |
| SVC-014 | Unit editing | Deferred | Explicitly out of MVP |
| LOG-001 | Journal viewer | Tested | `logs.Service`; service/HTTP tests |
| LOG-002 | System/unit logs | Tested | Unit query parameter; tests |
| LOG-003 | Bounded lines/time | Implemented | `lines` limit and fixed time-range enum |
| LOG-004 | Timestamp/severity | Tested | Journal JSON parsing; parser test |
| LOG-005 | Text filtering | Implemented | Client filter over returned window |
| LOG-006 | Refresh | Implemented | Manual refresh |
| LOG-007 | Output/time limits | Implemented | Line/byte caps and command timeout |
| LOG-008 | Permission errors | Tested | `JOURNAL_PERMISSION_DENIED`; test |
| LOG-009 | No log retention | Implemented | Response-only; no persistence |
| LOG-010 | XSS-safe rendering | Tested | React text rendering; script payload test |
| LOG-011 | Unavailable state | Implemented | Unavailable alert with permission context |
| LOG-012 | File-based logs | Deferred | Post-MVP; must use bounded SFTP |

### Phase 6 notes

- Process discovery uses the fixed `ps -eo ...` format; invalid lines are skipped.
- Signals are limited to SIGTERM and SIGKILL. PID 1 is rejected by the backend and disabled in the UI.
- The identity check compares the start time derived from `etimes` against the value captured by
  the list within a 3-second tolerance. A mismatch returns `409 PROCESS_PID_REUSED` without
  signaling.
- Service actions accept only validated `.service` identifiers and enumerated verbs. Drovik never
  uses sudo or silently retries with elevated privileges.
- Journal output is parsed from JSON, bounded, and never stored. Log text is untrusted and rendered
  as text.


## Phase 7 — MVP integration, security hardening and public beta

| ID | Requirement | Status | Implementation / evidence |
|---|---|---|---|
| AUD-001 | Record relevant events | Tested | Setup/login, server, host-key, file, process and service events; HTTP test |
| AUD-002 | Event metadata | Partial | Timestamp, actor, server, action and outcome; correlation ID only on request-scoped events |
| AUD-003 | No secrets in audit | Implemented | Target summaries contain paths/PIDs/units only |
| AUD-004 | No file contents | Implemented | Audit records never include content |
| AUD-005 | Failed operations recorded | Implemented | Safe error codes on process/service/SSH failures |
| AUD-006 | Audit protected from user modification | Implemented | No mutation API; SQLite file permissions are the boundary |
| AUD-007 | Filter audit history | Partial | Server-side user/server scoping and limit; client text/outcome filters, no date/actor filter yet |
| AUD-008 | Retention documented | Implemented | Retention documented in backup/restore and beta notes |
| AUD-009 | No tamper-proof claim | Implemented | UI and docs describe local audit metadata only |
| INT-001 | Server-context consistency | Implemented | Route/query keys include server ID; panels reset on server switch |
| INT-002 | No stale cross-server data | Implemented | React Query keys and server-scoped fetches |
| INT-003 | Standard error states | Implemented | Stable code catalog in `docs/api/errors.md` |
| UX-001 | Accessibility | Partial | Labels, focus rings, skip link and reduced-motion CSS; no full screen-reader audit |
| UX-002 | Responsive behavior | Partial | Sidebar/drawer, responsive tables and grids; real-device pass pending |
| REL-001 | Beta release assets | Implemented | `.github/workflows/release.yml` builds archives/checksums/image on tags |
| SEC-001 | Security test pass | Implemented | `make verify`, CodeQL, dependency review, govulncheck, gitleaks; see security-testing |
| SEC-002 | Threat-model review | Deferred | Scheduled with independent security audit before stable |

### Phase 7 notes

- Audit events are scoped to the actor or servers owned by the current user; the API exposes no
  mutation path.
- The audit UI performs text/outcome filtering over the latest 100 events. Date/actor filtering and
  configurable retention remain future work.
- Release artifacts are built from a tag, carry version/commit/build-date metadata and include
  SHA-256 checksums.
- Beta limitations are listed in `docs/status/beta-notes.md`.


## Phase 8 — Stable release and operational maturity

| Area | Status | Evidence |
|---|---|---|
| Stable versioning and cadence | Implemented | `docs/development/release.md`, SemVer tags |
| Compatibility matrix | Implemented | `docs/development/compatibility.md` |
| Migration/rollback guidance | Implemented | `docs/development/backup-restore.md`, release runbook |
| Automated release pipeline | Implemented | `.github/workflows/release.yml`; beta tag run |
| Checksums and container publishing | Implemented | SHA-256 archives + GHCR image; no signing in MVP |
| Security policy and disclosure | Implemented | `SECURITY.md`, supported-version table |
| Code ownership | Implemented | `.github/CODEOWNERS` |
| Performance/load validation | Partial | Parser benchmarks in `docs/development/performance.md`; no real-host load harness |
| Upgrade/backup procedures | Implemented | Backup/restore and release docs |
| Translation framework | Deferred | No community demand yet |

### Phase 8 notes

- Release artifacts carry version, commit and build-date metadata; stable tags also update the
  `latest` container tag.
- Checksums are published, but artifacts are not cryptographically signed yet.
- Real-host load and distribution matrix validation remain ongoing operational work.

## Phase 9 — Advanced server management (post-MVP)

Deferred as a product decision. Planned candidates: Docker/Podman management, historical metrics,
multi-user RBAC, file-based log browsing, resumable transfers and unit-file editing. Each requires a
separate design and threat model before implementation.


## Phase 9 — Advanced management (read-first scope)

| Area | Status | Implementation / evidence |
|---|---|---|
| 9.1 Docker detection | Tested | Fixed `docker ps/images` collector; `internal/advanced` tests |
| 9.1 Container status/ports | Tested | Container JSON parsing, ports/state/status |
| 9.1 Start/stop/restart/remove | Tested | Enumerated actions by validated ID, confirmation, audit, read-only block |
| 9.1 Logs/inspect | Tested | Bounded `docker logs`, JSON inspect |
| 9.1 Compose/socket warning | Implemented | No socket mount; UI warns root-equivalent access; Compose deferred |
| 9.2 Block devices/filesystems/inodes | Tested | `lsblk -J`, `df -Pk`, `df -Pki`, `/proc/mounts` parser tests |
| 9.2 Mount/format/partition | Deferred | Read-only inspection only, per risk review requirement |
| 9.3 Interfaces/routes/listeners | Tested | `ip -j addr/route`, `ss -tulpn` parser tests |
| 9.3 Network changes | Deferred | Read-only only; disconnect safeguards required |
| 9.4 Package manager detection/list | Tested | dpkg/rpm/apk/pacman detection and bounded list |
| 9.4 Install/update/remove | Deferred | Read-only list/search only |
| 9.5 Snippets CRUD/variables | Tested | Storage + service tests; variables and shell quoting |
| 9.5 Preview/confirm/execute | Tested | Preview endpoint, confirm required, audit, output bounds |
| 9.6 Cron/timer inspection | Tested | Fixed cron/timer collector parser tests |
| 9.6 Task editing | Deferred | Read-only only; backup/validation design required |
| 9.7 Local users/roles | Tested | User service, last-admin guard, HTTP read-only test |
| 9.7 Server assignment/read-only | Tested | `server_assignments`, effective access checks, route guard |
| 9.7 OIDC/MFA/external secrets | Deferred | Requires separate identity threat model |
| 9.8 Metric retention/history | Implemented | SQLite samples, retention/cap, history API and chart |
| 9.8 Threshold alerts | Implemented | In-app threshold rules; no external notifications |
| 9.8 Notifications | Deferred | No external channels in this scope |
| 9.9 Additional OS | Deferred | Linux-only support matrix; macOS/BSD explicitly unsupported |

### Phase 9 notes

- Advanced host operations reuse the existing SSH connection, host verification, authorization
  checks and audit pipeline from prior phases.
- Docker access is explicitly labelled root-equivalent. The Docker socket is never mounted into the
  Drovik container.
- Storage, network, package and scheduled-task surfaces are read-only; destructive changes remain
  intentionally absent.
- Read-only accounts and read-only server assignments are enforced by backend middleware, not only
  UI controls.
- History is local, bounded and explicitly not a substitute for enterprise monitoring. External
  notifications and OIDC/MFA remain post-Phase-9 hardening items.

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
