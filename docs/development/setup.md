# Local Development Setup

## Prerequisites

| Tool | Version | Purpose |
|---|---|---|
| Go | latest stable (1.27+ tested) | Backend |
| Node.js | 22+ (26 tested) | Frontend build |
| pnpm | 10+ (12 tested) | Frontend package manager |
| Git | 2.40+ | Version control |
| Docker + Compose | optional | Integration tests and deployment |
| golangci-lint | latest | Go linting |
| govulncheck | latest | Go vulnerability scanning |
| gitleaks | latest | Secret scanning |

On macOS:

```bash
brew install go node pnpm golangci-lint gitleaks
go install golang.org/x/vuln/cmd/govulncheck@latest
```

Docker is optional for Phase 1. Use Docker Desktop, Colima or another compatible runtime when
integration tests need disposable SSH containers.

## Clone and branch

```bash
git clone https://github.com/awbuilds/drovik.git
cd drovik
git switch -c feat/your-task
```

Never work on `main`.

## Configuration

```bash
cp .env.example .env
# Generate a development encryption key:
mkdir -p secrets
openssl rand -base64 32 > secrets/encryption.key
chmod 600 secrets/encryption.key
```

`.env` and `secrets/` are gitignored. Never commit them.

The backend reads configuration from environment variables. For local development it also loads
`.env` if present. Secrets are never printed in logs.

## Backend

```bash
cd backend
go mod download
go run ./cmd/drovik
```

The API listens on `DROVIK_HTTP_ADDR` (default `127.0.0.1:8080`).

Health endpoints:

- `GET /healthz` — liveness.
- `GET /readyz` — readiness (configuration + database).
- `GET /api/v1/version` — build/version metadata (no secrets).

## Frontend

```bash
cd frontend
pnpm install --frozen-lockfile
pnpm dev
```

Vite serves on `http://localhost:5173` and proxies `/api` to the backend. Set
`DROVIK_TRUSTED_ORIGINS` to include the Vite origin.

## Tests and checks

```bash
# Everything available
make verify

# Backend
cd backend
go test -race ./...
go vet ./...
golangci-lint run

# Frontend
cd frontend
pnpm lint
pnpm typecheck
pnpm test -- --run
pnpm build
```

## Git hooks

```bash
make hooks
```

The pre-commit hook refuses commits on `main`, scans staged changes for secrets when `gitleaks` is
installed, checks Go formatting and rejects staged secret-like files.

## Database

SQLite is created under `DROVIK_DATA_DIR` (default `./data`). Migrations run automatically on
startup. To reset local data:

```bash
rm -rf data/drovik.db data/drovik.db-shm data/drovik.db-wal
```

Resetting deletes the local administrator and sessions. It is a development action only.

## Troubleshooting

- **Port already in use:** change `DROVIK_HTTP_ADDR` or stop the other process.
- **Readiness fails:** check the database path, permissions and migrations.
- **Setup route disabled:** an administrator already exists. Use the documented recovery procedure
  rather than deleting production data.
- **Frontend cannot reach API:** check the Vite proxy and `DROVIK_TRUSTED_ORIGINS`.
- **Cookie not set:** in local HTTP development, `DROVIK_COOKIE_SECURE` must be `false`.

## Security notes for development

- Use synthetic hostnames and test keys only.
- Never point automated tests at a production server.
- Do not disable host-key verification, CSRF or rate limits to make a demo work.
- Do not commit screenshots containing real hosts, usernames or data.
