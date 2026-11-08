# 2026-11-08 Phase 2 SSH connections

## Task / Area
Implement saved SSH connections, encrypted credential storage, strict host-key trust, connection
tests, egress policy and capability detection.

## Current State
- Backend:
  - `internal/secrets`: AES-256-GCM encryption with associated data.
  - `internal/security/egress.go`: DNS resolution, IP validation, default blocking of loopback,
    link-local, multicast, unspecified and metadata, private-range control and operator CIDRs.
  - `internal/sshclient`: bounded dial/handshake, password and private-key auth, strict host-key
    callback, changed-key errors, bounded command output, capability detection and SFTP probe.
  - `internal/servers`: create/list/get/update/delete, credential encryption/decryption, host-key
    approval/replacement, connection tests and capability persistence.
  - Migration `00002_servers.sql`: `servers`, `known_hosts` and `server_capabilities`.
  - HTTP routes: `GET/POST /api/v1/servers`, `GET/PATCH/DELETE /api/v1/servers/{id}`,
    `POST /servers/{id}/test`, `POST /servers/{id}/trust-host-key`,
    `GET /servers/{id}/capabilities`.
- Frontend:
  - Servers page with saved-server cards, add/edit dialog, password/private-key auth, tags/notes,
    test-connection flow, host-key approval dialog, changed-key warning and delete confirmation.
  - Workspace command center now reflects saved servers and links to the server manager.
  - Topbar shows the saved-server count and the server nav is enabled.
- Tests:
  - `secrets` round-trip, AAD, tamper and version tests.
  - `security` egress tests, including explicit loopback override and deny-wins behavior.
  - `sshclient` integration tests against an in-process SSH/SFTP server: unknown key, password
    auth, private-key auth, changed key, command output and capabilities.
  - `servers` service tests: encryption at rest, validation, approval, wrong password, host change
    revoking trust and delete.
  - HTTP tests: CSRF rejection, create/list/delete, unknown host key details, trust, capability
    response and connection test.
- Deployment assets remain unchanged; the Phase 2 settings are documented in `.env.example`.

## Decisions
- Host-key trust is explicit TOFU: the 409 response exposes the presented key, and the operator
  must submit the same fingerprint/public key to approve. A changed key requires `replace: true`.
- Egress resolves once and dials the validated address; the original hostname is used for host-key
  identity.
- Credentials are never returned in API responses. AES-GCM associated data binds the server ID and
  secret field.
- Loopback is blocked by default; tests explicitly allow `127.0.0.1/32`. This prevents accidental
  SSRF in production while keeping local verification possible.

## Build / Test Status
- `make verify`: pass (Go format/vet/race tests/golangci-lint; frontend lint/typecheck/tests/build).
- `go test -race ./...`: pass, including the in-process SSH/SFTP integration tests.
- Browser review: add server, test connection → host-key approval → verified capabilities, with no
  console errors after the `Button asChild` fix.
- Not yet tested: real remote Linux hosts and the full session pool (Phase 3).

## Next Steps
- Phase 3: authenticated WebSocket terminal, PTY, resize, bounded session pool, cleanup and the
  xterm.js workspace panel.
- Add live connection-state transitions and resource-limit enforcement with the session pool.
- Add a Playwright flow for add server → trust key → test connection.

## Open Questions
- Provider-specific host-key algorithms (for example RSA-only hosts) need a compatibility pass.
- Whether to expose a raw known-hosts export in the admin UI; not required for MVP.
