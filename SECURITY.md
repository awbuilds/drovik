# Security Policy

Drovik is a gateway to remote systems. We take security reports seriously and ask that you do not
open a public issue for a vulnerability.

## Reporting a vulnerability

Use GitHub's private vulnerability reporting:

1. Open the repository's **Security** tab.
2. Choose **Advisories** → **Report a vulnerability**.
3. Include affected version/commit, impact, reproduction steps and any suggested fix.

If private reporting is unavailable, open a minimal issue asking for a private channel **without
including exploit details**, and the maintainer will follow up.

Do not include real credentials, private keys, customer data or production hostnames in a report.
Use synthetic examples.

## What to expect

- Acknowledgement within 7 days.
- An initial assessment and severity classification within 14 days where feasible.
- Coordinated disclosure after a fix is available and users have had a reasonable upgrade window.
- Credit in the advisory if you want it.

This is a volunteer-maintained project; these are targets, not contractual SLAs.

## In scope

- Authentication, session and CSRF weaknesses.
- Authorization bypass between users/servers.
- SSH host-key verification bypass or credential exposure.
- Command injection in server, process, service, file or log operations.
- Path traversal and symlink escape in file operations.
- SSRF or egress-policy bypass.
- WebSocket origin/authentication bypass.
- XSS through filenames, hostnames, process names, logs or terminal output.
- Secret leakage in logs, errors, traces, API responses or backups.
- Resource exhaustion that affects other users or the host.

## Out of scope

- Vulnerabilities requiring a compromised administrator account or physical access.
- Issues in unsupported browsers or operating systems.
- Social engineering.
- Denial of service from a single authenticated user against their own installation, unless it
  bypasses a documented limit.
- Findings from automated scanners without a demonstrated impact.

## Supported versions

| Version | Supported |
|---|---|
| Latest stable `v1.x` | Yes |
| Current beta `v0.1.x-beta` | Security fixes until the next stable release |
| Older tags/commits | No |

Security fixes are released as soon as practical. Upgrade using the documented backup and restore
procedure in [docs/development/backup-restore.md](docs/development/backup-restore.md).

## Security design

See [docs/security/threat-model.md](docs/security/threat-model.md) and
[docs/security/security-architecture.md](docs/security/security-architecture.md).
