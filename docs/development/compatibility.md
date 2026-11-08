# Compatibility and supported platforms

Drovik is agentless: it uses the SSH server and standard Linux interfaces that already exist on
the managed host. The beta targets servers, not desktops.

## Controller (Drovik itself)

- Linux amd64 and arm64 containers are the supported deployment targets.
- Go builds are statically linked (`CGO_ENABLED=0`); the release archive includes the frontend.
- SQLite is embedded; no external database is required.
- Modern browsers with WebSocket, `ResizeObserver` and ES2022 support (current Chrome, Firefox,
  Safari and Edge).

## Managed hosts

| Area | Requirement | Notes |
|---|---|---|
| SSH | OpenSSH 7.4+ or compatible | Password or private-key auth; strict host-key verification |
| OS | Linux with `/proc` | Full feature set: terminal, SFTP, monitoring, processes, systemd, journal, Docker and advanced actions |
| OS | macOS (Darwin) or BSD | Read-only monitoring: host, load, memory, filesystems, network counters and uptime. Linux-only panels report `CAPABILITY_UNAVAILABLE` |
| Architecture | x86_64, arm64, armv7 | Depends on the remote tools, not the Drovik binary |
| Process tools | `ps` with `-eo` (procps-ng preferred) | BusyBox `ps` may lack the required format; the UI reports `PROCESS_UNAVAILABLE` |
| Files | SFTP subsystem | Required for the Files page; terminal and monitoring still work without it |
| Credentials | Optional HashiCorp Vault KV v2 | Store only the path in Drovik; the secret is fetched at connect time |
| Services | systemd 230+ with `systemctl` | Non-systemd hosts show `SERVICE_MANAGER_UNAVAILABLE` |
| Logs | `journalctl` with JSON output | Journal permissions are enforced by the remote system |
| Monitoring (Linux) | `/proc/stat`, `/proc/meminfo`, `/proc/loadavg`, `/proc/uptime`, `/proc/net/dev`, `df` | Missing sections are individually marked unavailable |
| Monitoring (macOS/BSD) | `uname`, `sysctl`, `vm_stat`, `df`, `mount`, `netstat`, `top` (macOS CPU) | Optional sources degrade individually; CPU usage is point-in-time on macOS |

## Distribution matrix

Beta validation targets:

- Ubuntu Server 22.04 LTS and 24.04 LTS
- Debian 12
- Rocky Linux 9 / AlmaLinux 9
- Fedora Server 40+

Other current Linux distributions with the tools above are expected to work but are not part of the
beta test matrix. Containers without systemd, minimal appliances and network devices are supported
only for features whose tools are present.

## Not supported

- Windows hosts.
- A validated write feature set on macOS/BSD hosts. Read-only monitoring is supported; the
  terminal and SFTP use standard SSH but are outside the validated matrix. Process, systemd,
  journal, Docker and Linux advanced actions are unavailable.
- Editing systemd unit files or creating arbitrary units.
- File-based log tailing outside the journal.
- Destructive Docker/Podman changes beyond enumerated container start/stop/restart/remove.
- Unattended destructive storage/network operations. Feature-flagged manual actions require
  `DROVIK_ENABLE_DESTRUCTIVE_HOST_OPS=true`, administrator access and explicit confirmation.

Report compatibility gaps with the managed host distribution, tool versions and the stable Drovik
error code.
