# AGENTS.md — Drovik

This file is auto-loaded by coding agents at session start. It is the onboarding contract for
every human or AI contributor. Read it fully, then read the latest handoff note in
`docs/handoffs/` before touching code. If a request conflicts with a hard rule below, stop and ask
instead of silently bypassing it.

## Project at a glance

- **Product:** Drovik is an open-source, self-hosted, agentless web workspace for managing Linux
  servers over SSH: servers, terminal, SFTP files, system overview, processes, systemd services,
  logs and audit history.
- **Status:** Phase 0 (foundation) and Phase 1 (application skeleton + authentication) are the
  current delivery. Phases 2–8 remain per `docs/product/PRD.md`.
- **Architecture:** browser → Go backend → SSH/SFTP → managed Linux host. The browser never
  connects directly to remote hosts and never receives stored SSH secrets.
- **Stack:** Go backend; React 19 + TypeScript + Vite + Tailwind CSS 4 frontend; SQLite via
  `modernc.org/sqlite`; `golang.org/x/crypto/ssh`; `github.com/pkg/sftp`; xterm.js for terminals.
- **License:** GNU Affero General Public License v3.0 only (`AGPL-3.0-only`).
- **Copyright:** Abhijith Warrier.
- **Repository:** <https://github.com/awbuilds/drovik>.

## Non-negotiable rules

These rules override convenience, demo deadlines and generic habits.

### 1. Branch and pull-request workflow

- **Never commit or push directly to `main`.** `main` is protected and PR-only.
- Start every task on a short-lived branch from up-to-date `main`:
  `feat/...`, `fix/...`, `chore/...`, `docs/...`, `test/...`, `security/...`.
- One bounded task per branch/PR. Keep PRs reviewable.
- Merge only through a pull request after required checks pass and conversations are resolved.
- Do not force-push `main`, delete `main`, or bypass the ruleset. Do not use admin bypass.
- Conventional Commits: `feat:`, `fix:`, `chore:`, `docs:`, `test:`, `refactor:`, `perf:`,
  `security:`, `build:`, `ci:`.
- No `Co-Authored-By`, `Generated-by` or other AI attribution trailers. Commit as the repository
  owner only.

### 2. Crash safety

- **No application panic may escape a boundary.** Recover at every HTTP, WebSocket, goroutine,
  background-job, CLI and startup boundary.
- **Never swallow an error.** Return it, wrap it with context, map it to a stable code, and log it
  once at the boundary. `errcheck` is enforced.
- **Every caught error or panic is logged** with a stable code, correlation ID and safe context.
  Panics log a stack trace. Secrets and sensitive content are redacted.
- **Frontend:** root and per-panel error boundaries; explicit `try/catch` around every async call,
  event handler, WebSocket callback, storage/parse/clipboard access; global `error` and
  `unhandledrejection` handlers.
- **Fail closed** for security checks. **Degrade gracefully** for optional features. Never fake
  success, and never retry forever.
- A green build is not enough. Changes touching an entry point require a crash-safety test.

### 3. Secrets, privacy and open-source hygiene

- **Never commit secrets or private data.** No real hostnames, IP addresses, usernames, emails,
  private keys, passwords, tokens, cookies, customer names, screenshots with personal data,
  absolute user paths or internal company references.
- Use `example.com`, RFC 5737 IP ranges (`192.0.2.0/24`, `198.51.100.0/24`, `2001:db8::/32`), and
  runtime-generated test keys. Test fixtures must never contain real credentials.
- `.env` files, databases, keys, logs, build output and local tool state are gitignored. Commit
  `.env.example` only.
- **Never log or return** secrets, credentials, tokens, private keys, passphrases, file contents,
  terminal input/output, log contents, or `Authorization`/cookie headers.
- No telemetry, analytics or external crash reporting by default. Any future telemetry must be
  opt-in, documented and free of secrets.
- Run `gitleaks` and the repository secret scan before pushing.

### 4. Security invariants

- Verify SSH host keys strictly. Never disable host-key verification for a demo.
- Encrypt SSH credentials at rest with authenticated encryption; keep the key outside the database.
- Enforce authorization in the backend for every server-scoped resource.
- Use parameterized SQL only; never concatenate SQL.
- Use structured, enumerated remote operations. Never build shell commands from arbitrary input.
- Validate remote paths; prevent traversal and symlink escapes.
- Validate WebSocket authentication and `Origin`; enforce message size/rate limits.
- Apply bounded timeouts, output sizes, concurrency and request sizes everywhere.
- Apply the egress policy: block loopback, link-local, multicast, unspecified and cloud metadata
  endpoints by default; allow public and RFC1918 ranges by default for self-hosted use, with
  operator configuration.
- CSRF protection is required for cookie-authenticated mutations.

### 5. Dependencies and supply chain

- Keep dependencies minimal, maintained and pinned.
- Pin GitHub Actions to full commit SHAs.
- Review dependency changes for security and license compatibility.
- Do not add a dependency to work around a design problem.

### 6. Decisions, handoffs and documentation

- Read this file, then the latest relevant `docs/handoffs/` note, then the relevant ADR/doc.
- Durable decisions go into `AGENTS.md` and/or `docs/adr/`. Session state goes into a handoff.
- Update requirement traceability when a requirement is implemented or deferred.
- Update documentation and `CHANGELOG.md` for user-visible changes.

### 7. Definition of done

A change is complete only when:

- Requirement/issue linked; scope clear and limited.
- Backend authorization and input bounds implemented where relevant.
- Failure, cancellation and cleanup paths handled; crash-safety guards added at entry points.
- Tests added/updated and actually run; report exactly what ran and what did not.
- Secrets redacted; no private data added.
- Docs, ADRs and traceability updated.
- A handoff note is written.
- CI is green; no known critical/high security issue is introduced.

## Branch protection and repository settings

`main` must have an active ruleset with:

- Pull request required before merge.
- Required status checks (strict, up to date).
- Conversation resolution required.
- Linear history required.
- Force pushes blocked.
- Branch deletion blocked.
- Rules enforced for administrators; no bypass actors.
- `v*` release tags protected.

Repository settings:

- Secret scanning and push protection enabled.
- Dependabot alerts and security updates enabled.
- Squash merge only; delete branch on merge.
- GitHub Actions pinned to full SHAs.

If you cannot verify these settings, stop and report it.

## Build and verify

Run from the repository root unless noted.

```bash
# Backend
cd backend
go test -race ./...
go vet ./...
golangci-lint run
govulncheck ./...

# Frontend
cd frontend
pnpm install --frozen-lockfile
pnpm lint
pnpm typecheck
pnpm test
pnpm build

# Everything
make verify
make security
```

The exact commands may evolve; update this section when they do.

## Repository layout

```text
backend/                 Go service, packages under internal/
frontend/                React application
deploy/                  Docker, Compose, Caddy and Nginx examples
docs/
  adr/                   Architecture decision records
  architecture/          Architecture overview and diagrams
  development/           Local setup and workflow
  handoffs/              Append-only session handoff notes
  product/               PRD and requirement traceability
  security/              Threat model and security architecture
scripts/                 Verification, hooks and handoff helpers
testdata/                Sanitized test fixtures only
.github/                 CI, security workflows and templates
```

Production code, deployment assets, documentation and test fixtures stay separated.

## Architecture map

### Backend boundaries

- `cmd/drovik` — process entry point, wiring, lifecycle.
- `internal/config` — environment/config parsing and validation.
- `internal/app` — dependency wiring, startup and graceful shutdown.
- `internal/httpapi` — routes, handlers, middleware, error mapping.
- `internal/auth` — password hashing, sessions, CSRF, authorization.
- `internal/users` — accounts and roles.
- `internal/servers` — saved server metadata and ownership.
- `internal/secrets` — authenticated encryption and secret handling.
- `internal/hostkeys` — known-host storage and verification.
- `internal/sshclient` — SSH dialing, timeouts, session pool and lifecycle.
- `internal/terminal` — PTY sessions and WebSocket streaming.
- `internal/sftp` — remote file operations and transfer jobs.
- `internal/monitoring` — bounded host metrics and capability detection.
- `internal/processes` — structured process operations.
- `internal/services` — systemd discovery and actions.
- `internal/logs` — bounded journal retrieval.
- `internal/audit` — safe audit events.
- `internal/storage` — repositories, transactions, migrations.
- `internal/security` — origin, egress, validation and redaction helpers.
- `internal/observability` — structured logging, request IDs and redaction.
- `internal/version` — version, commit and build metadata.

HTTP handlers contain no raw SSH command construction, database encryption or permission rules.

### Frontend boundaries

```text
src/
  app/          bootstrap, routes, providers
  api/          typed API client and error mapping
  auth/         setup, login, session state
  workspace/    shell, navigation, panels
  servers/      server list and forms
  terminal/     xterm.js lifecycle
  files/        file browser and editor
  monitoring/   system overview
  processes/    process manager
  services/     service manager
  logs/         log viewer
  activity/     audit history
  settings/     application settings
  components/   shared design-system components
  hooks/        shared hooks
  lib/          formatting, validation, safe helpers
  types/        shared types
```

- One typed API client; no ad-hoc `fetch` in components.
- Centralized auth state.
- Explicit server IDs in every server-scoped request.
- Every async operation handles loading, error, cancellation and stale responses.
- WebSockets close on unmount, logout, server switch and session expiry.
- User/remote content is rendered as text, never raw HTML.
- No secrets in browser storage or console logs.

## Crash-safety contract

### Go backend

- Use `errors.Is`/`errors.As`; wrap with `%w`; map to a stable API error code.
- `panic` is allowed only for truly impossible programmer states, and must be recovered at a
  boundary. Prefer returning an error.
- Entry points that must recover:
  - HTTP middleware (`Recoverer`).
  - WebSocket read/write loops.
  - Every goroutine via `safe.Go` (recover, log, optionally restart with backoff).
  - Background jobs and schedulers.
  - CLI subcommands.
  - `main` startup.
- A recovered panic returns `INTERNAL_ERROR`, logs the stack and correlation ID, and does not
  terminate the process.
- Expected errors (4xx) log at `warn`; unexpected/5xx/panic log at `error`.
- Never `os.Exit` inside a handler or goroutine. Fatal exit is reserved for invalid startup
  configuration, after a safe log line.
- Graceful shutdown: stop accepting work, cancel contexts, close SSH/SFTP/WebSocket resources,
  wait with a bounded timeout.

### React frontend

- A root `ErrorBoundary` and per-panel boundaries render a recoverable fallback, never a white
  screen.
- Wrap every async/event/WebSocket/storage/parse/clipboard call in `try/catch`.
- Global `window.onerror` and `unhandledrejection` handlers log sanitized context.
- Never leave a promise unhandled; `void` is not a substitute for error handling.
- Show explicit loading, empty, success, warning, permission-denied, timeout, disconnected and
  unknown states.
- No secrets in `localStorage`, `sessionStorage`, URLs or console logs.
- Respect `prefers-reduced-motion`.

### Honest limits

`recover` cannot catch Go runtime fatals (OOM, stack overflow, `runtime.throw`, SIGKILL), and React
cannot catch a browser-tab crash or network loss. We guarantee that no *application* panic escapes a
boundary, that the process stays alive for request/goroutine panics, and that failures degrade
visibly and are logged — not literal immortality.

### Logging contract

Structured JSON in production, human-readable text in development. Every entry includes:
timestamp, level, event, stable error code (when relevant), correlation ID, and safe operation
context. Redact secret-like keys and values. Never log request bodies for auth, credentials, file
contents, terminal I/O or logs.

## Frontend design contract

The UI must be deliberate, not a generic dashboard template.

- **Direction:** “control room” — deep ink surfaces, warm signal accent, precise typography,
  restrained motion, high information density without clutter.
- **Tokens:** semantic colors, spacing, radii, elevation, typography and motion live in one token
  layer. No raw hex values in components.
- **Components:** Radix primitives for accessibility, custom styling on top. Shared Button, Input,
  Field, Card, Badge, StatusDot, Dialog, Toast, Tooltip, Tabs, Separator, Skeleton, EmptyState and
  ErrorState.
- **Type:** self-hosted variable fonts; monospace for identifiers, metrics and terminal-adjacent
  labels.
- **Accessibility:** WCAG 2.2 AA, visible focus, keyboard navigation, reduced motion, contrast.
- **States:** every screen has loading, empty, success, warning, permission-denied, timeout,
  disconnected and error states.
- **No generic AI aesthetics:** no purple gradient hero, no glassmorphism everywhere, no giant
  rounded cards with meaningless icons.

## Error codes

Use stable machine-readable codes from the PRD appendix, for example: `AUTH_REQUIRED`,
`AUTH_INVALID_CREDENTIALS`, `AUTH_SESSION_EXPIRED`, `AUTH_FORBIDDEN`, `VALIDATION_ERROR`,
`SERVER_NOT_FOUND`, `SSH_HOST_KEY_UNKNOWN`, `SSH_HOST_KEY_CHANGED`, `SSH_AUTH_FAILED`,
`SFTP_PERMISSION_DENIED`, `CAPABILITY_UNAVAILABLE`, `RATE_LIMITED`, `OUTCOME_UNKNOWN`,
`INTERNAL_ERROR`. Never expose stack traces or secrets.

## Handoff protocol

Read the newest relevant note in `docs/handoffs/` before starting. Write a new note before ending
any session that made progress.

- Location: `docs/handoffs/YYYY-MM-DD-short-topic.md`.
- Append-only. Never edit or delete an existing note; write a new dated note when state changes.
- Newest wins. The latest note for an area is authoritative over older ones.
- Aim for under 60 lines. It is a resume point, not a spec.
- Template is in `docs/handoffs/README.md`.
- Never include secrets or private data.
- If a task is finished and merged, say so, so the next agent does not redo it.

## Documentation map

- `docs/product/PRD.md` — product requirements and phased roadmap (source of truth).
- `docs/product/traceability.md` — requirement ID → implementation/test/PR status.
- `docs/adr/` — durable architecture decisions.
- `docs/security/threat-model.md` — assets, actors, threats and controls.
- `docs/security/security-architecture.md` — security design and invariants.
- `docs/development/setup.md` — local development.
- `docs/development/workflow.md` — branch, PR and review workflow.
- `docs/handoffs/` — session continuity.
- `docs/api/openapi.yaml` — versioned HTTP contract (OpenAPI 3.1).
- `CHANGELOG.md` — user-visible changes.
- `ROADMAP.md` — product roadmap summary.

## Requirement traceability

Every feature must reference a PRD requirement ID (for example `AUTH-004`, `FILE-022`) in its issue,
PR and tests. Update `docs/product/traceability.md` when status changes. Do not mark a requirement
complete just because a screen or endpoint exists; the acceptance criteria and failure paths must be
covered.

## Current status

- Phase 0: complete. The repository foundation, governance, ADRs, threat model, AGENTS.md,
  handoffs and CI/security gates are merged.
- Phase 1: complete and merged. It adds the Go service foundation, SQLite migrations,
  first-run setup, Argon2id authentication, server-side sessions, session-bound CSRF, rate
  limiting, the React design system/workspace shell, deployment assets and tests.
- Phase 2: complete. Encrypted SSH credentials, strict host-key trust, server CRUD, connection
  tests, egress policy and capability detection are implemented and tested.
- Phase 3: complete. Authenticated WebSocket PTY terminal, bounded session pool, resize, cleanup
  and the xterm.js workspace panel are implemented and tested.
- Phase 4: next. SFTP file manager, transfers and text editor.
- SFTP, monitoring, process, service and log features are not implemented yet.
- The UI is React + Tailwind, inspired by the Rune/OpenCode design language; the Rune Rust
  framework itself is not used.
- Read the latest handoff in `docs/handoffs/` before starting Phase 4.

## Decision inheritance

Before making a decision that affects architecture, security, data handling, UX or dependencies:

1. Check this file and the relevant ADR.
2. If the decision is already covered, follow it.
3. If it conflicts, stop and ask.
4. If it is new and durable, record it in an ADR and update this file where appropriate.
5. Record session-specific state in a handoff note.

The goal is that any agent, on any day, can make the same decision the previous agent would have
made — and that a human reviewer can see why.
