# Drovik

[![CI](https://github.com/awbuilds/drovik/actions/workflows/ci.yml/badge.svg)](https://github.com/awbuilds/drovik/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/awbuilds/drovik?sort=semver)](https://github.com/awbuilds/drovik/releases)
[![License: AGPL-3.0-only](https://img.shields.io/badge/license-AGPL--3.0--only-blue.svg)](LICENSE)
[![Website](https://img.shields.io/badge/website-awbuilds.github.io%2Fdrovik-2ea043)](https://awbuilds.github.io/drovik/)

**A self-hosted, agentless web workspace for your Linux servers.**

Drovik brings the everyday work of managing Linux machines into one deliberate interface:
saved SSH connections, an interactive terminal, SFTP file management, system health, processes,
systemd services, logs and audit history. It uses the SSH access you already have. It installs
nothing on the servers you manage.

> **Status:** stable (`v1.1.1`). Drovik provides authentication, strict SSH host-key trust, terminal,
> SFTP files, agentless monitoring, process/service/log operations, Docker and host diagnostics,
> snippets, local users/server assignments, history and audit. Drovik is a working name pending
> trademark clearance.

[Website](https://awbuilds.github.io/drovik/) · [Documentation](docs/) ·
[Install](deploy/README.md) · [Security](SECURITY.md) · [Contributing](CONTRIBUTING.md) ·
[Support](SUPPORT.md)

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

## What it does

| Area | Capability |
|---|---|
| Servers | Save and verify SSH hosts, password or key auth, strict host-key trust, per-server assignments |
| Workspace | One shell with persistent server context and multiple tool panels |
| Terminal | Full interactive SSH PTY with resize, copy/paste and clean lifecycle |
| Files | SFTP browsing, streamed transfers, create/rename/delete and a bounded text editor |
| Overview | CPU, memory, disk, uptime, load and network counters without an agent |
| Processes | Inspect and safely signal processes your SSH user owns, with PID identity checks |
| Services | Inspect and control systemd units where permitted, with post-action state |
| Logs | Bounded journal viewing with safe text rendering |
| Docker | Inspect containers and images; start, stop, restart or remove after confirmation |
| Advanced | Read-only storage, network, package and scheduled-task diagnostics |
| Snippets | Reusable command templates with substituted preview and explicit confirmation |
| Users | Local accounts, roles and per-server `read`/`write` assignments |
| History | Optional bounded metric retention, charts and in-app threshold alerts |
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

## Install

The supported deployment is a self-hosted container on Linux:

```bash
docker pull ghcr.io/awbuilds/drovik:v1.1.1
```

Use the [Compose example](deploy/compose/docker-compose.yml) with a persistent `/data` volume, a
separate encryption key secret, `DROVIK_PUBLIC_URL`, and a TLS reverse proxy. Release archives with
the frontend included are attached to each [GitHub release](https://github.com/awbuilds/drovik/releases).
Read the [compatibility matrix](docs/development/compatibility.md) and
[backup guide](docs/development/backup-restore.md) before production use.

> Drovik is stable, but it is a gateway to privileged systems. Start with a disposable host, verify
> host keys out of band, and keep the encryption key outside the database.

## Quick start (development)

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

## Website

The project website is a dependency-free static site in [`site/`](site/). Preview it with
`python3 -m http.server 4173 --directory site`, then open <http://127.0.0.1:4173/>. It makes no
third-party requests and has no build step. See [site/README.md](site/README.md) for structure,
checks and deployment.

## Project

- **Website:** <https://awbuilds.github.io/drovik/>
- **Engineering docs:** [docs/](docs/) including the [PRD](docs/product/PRD.md),
  [architecture](docs/architecture/overview.md) and [ADRs](docs/adr/)
- **Roadmap:** [ROADMAP.md](ROADMAP.md)
- **Changelog:** [CHANGELOG.md](CHANGELOG.md)
- **Governance:** [GOVERNANCE.md](GOVERNANCE.md)
- **Code of conduct:** [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)
- **Support:** [SUPPORT.md](SUPPORT.md)

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
