# 2026-11-08 Phase 3 interactive terminal

## Task / Area
Implement the authenticated WebSocket SSH terminal, PTY streaming, resize, bounded sessions and
the xterm.js workspace panel.

## Current State
- Backend:
  - `internal/terminal`: session manager with per-user/global limits, PTY request, stdin/stdout/
    stderr streaming, resize, close handling, input rate limits, shutdown cleanup.
  - `httpapi.handleTerminal`: authenticated upgrade with trusted-Origin check before accepting.
  - `statusRecorder` now supports HTTP hijacking for WebSocket upgrades.
  - `servers.Service.Connect` opens and audits long-lived managed SSH clients.
  - `Server.Close` closes terminal sessions on shutdown.
  - Terminal protocol documented in `docs/api/terminal-protocol.md` and OpenAPI.
- Frontend:
  - `TerminalPanel`: xterm.js + fit addon, WebSocket lifecycle, resize observer, reconnect and
    explicit close, theme refresh, sanitized error reporting.
  - `TerminalPage`: server selector, server identity, status states and security note.
  - Terminal nav enabled; server cards link directly to `/terminal/{id}`.
  - Terminal route is lazy-loaded, keeping xterm out of the main bundle.
  - Vite proxies WebSocket upgrades for `/api`.
- Tests:
  - WebSocket end-to-end test against the in-process SSH server: connect, prompt, resize, input
    and echo.
  - Authentication rejection test (401) and untrusted-Origin rejection test (403).
  - Terminal manager per-user/global limit and closed-manager tests.
  - Frontend terminal message parsing and URL tests; 16 frontend tests total.
- Manual browser review: add server → trust key → open terminal → prompt → type, with no page
  errors.

## Decisions
- The WebSocket upgrade requires the session cookie and a trusted `Origin`; the Origin check runs
  before `websocket.Accept`.
- Input is bounded at 64 KiB per message and 200 messages/second; output is chunked at 16 KiB.
- Sessions are bounded per user/global and registered for shutdown cleanup.
- Terminal input/output is not stored anywhere by default.
- The terminal page is lazy-loaded so xterm does not inflate the initial bundle.

## Files Touched
- Backend: `internal/terminal/`, `internal/httpapi/terminal.go`, `internal/httpapi/server.go`,
  `internal/httpapi/middleware.go`, `internal/servers/service.go`, `internal/httpapi/terminal_test.go`.
- Frontend: `src/components/terminal/`, `src/pages/TerminalPage.tsx`, `src/lib/terminal.ts`,
  `src/app/App.tsx`, `src/layout/AppShell.tsx`, `src/pages/ServersPage.tsx`,
  `src/components/servers/ServerCard.tsx`, `vite.config.ts`.
- Docs: OpenAPI, terminal protocol, traceability, README/ROADMAP/AGENTS/CHANGELOG.

## Build / Test Status
- `make verify` passes after the Phase 3 changes (Go race tests/lint; frontend lint/typecheck/
  tests/build).
- `go test -race ./...`: terminal and HTTP WebSocket tests pass.
- Manual terminal review: connected, prompt visible, typed input echoed.
- Not run: real remote host matrix; xterm rendering across every browser.

## Next Steps
- Phase 4: SFTP file manager, streaming transfers, path/symlink safety and a bounded text editor.
- Add Playwright coverage for terminal open/close/reconnect.
- Surface remote shell exit status (TERM-019) and add a multi-line paste warning (TERM-015).

## Open Questions
- Whether to persist per-user terminal preferences (font size) locally or in app settings.
- Whether to support multiple concurrent terminals in tabs; the backend already supports it.
