# Security testing

This page records the repeatable security checks for the MVP and what they cover. It is not a
claim that the system is free of vulnerabilities.

## Automated checks

```bash
# Backend race tests, including auth, WebSocket, SFTP, egress and operations.
cd backend && go test -race ./...

# Static analysis and vulnerabilities.
cd backend && go vet ./... && golangci-lint run && govulncheck ./...

# Frontend unit tests and production build.
cd frontend && pnpm lint && pnpm typecheck && pnpm test && pnpm build

# Secret scanning and dependency audit.
make security
gitleaks detect --source . --redact -v
```

CI runs the same backend/frontend checks plus CodeQL, dependency review, the Go vulnerability check,
secret scanning and Scorecard. GitHub Actions are pinned to commit SHAs.

## Test coverage map

| Control | Evidence |
|---|---|
| Setup/login rate limits and Argon2id | `internal/auth`, `internal/security` tests |
| Session cookie and CSRF | `internal/httpapi` tests |
| Session expiry/revocation | `internal/storage`, `internal/auth` tests |
| Host-key TOFU and changed-key blocking | `internal/sshclient`, `internal/servers` tests |
| Credential encryption and AAD binding | `internal/secrets` tests; ciphertext-at-rest assertions |
| Egress policy and metadata blocking | `internal/security/egress_test.go` |
| Loopback override only with explicit CIDR | egress and HTTP tests |
| Terminal authentication | `TestTerminalRequiresAuthentication` |
| Terminal Origin validation | `TestTerminalRejectsUntrustedOrigin` |
| Terminal message size/rate limits | terminal manager tests and constants |
| SFTP path traversal and symlink handling | `internal/files` tests |
| File size/binary/stale guards | `internal/files` and HTTP tests |
| Process PID validation and reuse check | `internal/processes` tests |
| No arbitrary shell input for units/PIDs | service/process validation tests |
| Journal permission and bounds | `internal/logs` tests |
| Log content XSS safety | frontend operations tests and manual script payload |
| Audit scoping by user/server | `TestAuditListScopesToUser` |
| Secret redaction | `internal/observability` tests |

## Manual verification still required

- Real host matrix for OpenSSH versions, `ps` variants, systemd/PolicyKit and journal permissions.
- Container permissions and read-only root filesystem checks on the target platform.
- Backup/restore drill using the documented procedure.
- Browser keyboard/screen-reader pass on every page.
- Reverse-proxy TLS and WebSocket upgrade behavior (Caddy/Nginx examples).

## Reporting

Use the process in [`SECURITY.md`](../../SECURITY.md). Do not open a public issue for a suspected
vulnerability. Include the affected version, the trust boundary you believe is crossed and a minimal
reproduction that does not contain real credentials or third-party data.
