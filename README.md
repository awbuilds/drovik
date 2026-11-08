# Drovik

**A self-hosted, agentless web workspace for your Linux servers.**

Drovik brings the everyday work of managing Linux machines into one deliberate interface:
saved SSH connections, an interactive terminal, SFTP file management, system health, processes,
systemd services, logs and audit history. It uses the SSH access you already have. It installs
nothing on the servers you manage.

> **Status:** early development. Phases 0–3 are complete: repository foundation, authentication,
> SSH connections with strict host-key trust, and the interactive terminal. Phase 4 (SFTP file
> manager) is next. File, monitoring and operations features are not implemented yet. Drovik is a
> working name pending trademark clearance.

## Why Drovik

Managing a handful of servers usually means juggling an SSH client, an SFTP client, a monitoring
dashboard and a pile of remembered commands. Drovik gives you one workspace that is honest about
what it is doing:

- **Agentless.** No daemon, package or persistent helper on managed hosts.
- **Least privilege.** Drovik acts as your normal SSH user and never silently escalates.
- **No false success.** A remote operation is only reported as successful after it is verified.
- **Terminal first.** The graphical tools are helpers; the terminal remains a first-class tool.
- **Local ownership.** Self-hosted, no mandatory cloud account, no default telemetry.
- **Capability-aware.** Missing tools and distribution differences are shown, not hidden.

## What it will do

| Area | Capability |
|---|---|
| Servers | Save and verify multiple SSH hosts, password or key auth, strict host-key trust |
| Workspace | One shell with persistent server context and multiple tool panels |
| Terminal | Full interactive SSH PTY with resize, copy/paste and clean lifecycle |
| Files | SFTP browsing, upload/download, create/rename/delete, text editing |
| Overview | CPU, memory, disk, uptime, load and network counters without an agent |
| Processes | Inspect and safely signal processes your SSH user owns |
| Services | Inspect and control systemd units where permitted |
| Logs | Bounded journal viewing with safe text rendering |
| Audit | Local metadata for logins, connections and sensitive actions |

See the full [product requirements and roadmap](docs/product/PRD.md) for scope, non-goals and
acceptance criteria.

## Architecture

```text
Browser (React)
   │  HTTPS / WebSocket
   ▼
Drovik backend (Go)
   │  SSH / SFTP
   ▼
Managed Linux host (OpenSSH, no Drovik agent)
```

The browser never connects directly to managed hosts and never receives stored SSH credentials.
See [the architecture overview](docs/architecture/overview.md) and the
[threat model](docs/security/threat-model.md).

## Quick start

> The application is under active construction. These commands describe the intended local
> development setup and will be kept current as each phase lands.

```bash
git clone https://github.com/awbuilds/drovik.git
cd drovik

# Configuration and development encryption key
cp .env.example backend/.env
mkdir -p backend/secrets
openssl rand -base64 32 > backend/secrets/encryption.key
chmod 600 backend/secrets/encryption.key

# Backend
cd backend
go run ./cmd/drovik

# Frontend (in another terminal)
cd frontend
pnpm install --frozen-lockfile
pnpm dev
```

The frontend dev server proxies `/api` to `http://127.0.0.1:8080` by default. Set
`DROVIK_API_PROXY` if the backend runs elsewhere. For production deployment, see
[deploy/README.md](deploy/README.md) and
[docs/development/setup.md](docs/development/setup.md).

## Security

Drovik becomes a gateway to systems that may contain sensitive data. Security is a release
requirement, not a feature:

- Strict SSH host-key verification, including explicit first-use and changed-key flows.
- SSH credentials encrypted at rest with the encryption key outside the database.
- Backend authorization on every server-scoped resource.
- CSRF, WebSocket origin checks, rate limits and bounded output.
- Structured remote operations instead of shell-string construction.
- No telemetry and no secret logging by default.

Please report vulnerabilities privately through GitHub Security Advisories as described in
[SECURITY.md](SECURITY.md). Do not open a public issue for a security problem.

## Contributing

Contributions are welcome. Read [CONTRIBUTING.md](CONTRIBUTING.md) and
[AGENTS.md](AGENTS.md) before starting. All work goes through a branch and a pull request; `main`
is protected.

## License

Drovik is licensed under the [GNU Affero General Public License v3.0 only](LICENSE)
(`AGPL-3.0-only`). If you run a modified version as a network service, you must offer its source.

Copyright © 2026 Abhijith Warrier.
