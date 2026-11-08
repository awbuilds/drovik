# DROVIK — Product Requirements, Technical Specification & Phased Roadmap

**Document type:** Product Requirements Document (PRD) + Software Requirements Specification (SRS) + Technical Design Outline  
**Product name:** Drovik *(working name; brand, domain and trademark clearance are not yet complete)*  
**Document status:** Baseline specification for discovery and implementation  
**Initial product form:** Self-hosted web application  
**Primary environment:** Linux servers managed over SSH  
**Initial connection model:** Agentless; SSH is the only required remote access mechanism  
**Recommended initial license:** GNU Affero General Public License v3.0 (AGPL-3.0)  
**Intended audience:** Product owner, developers, contributors, testers, security reviewers and future maintainers

---

## 0. How to use this document

This document defines the product direction and the implementation requirements for Drovik. It is intended to be the central reference for building the product phase by phase.

- **MUST** means mandatory for the stated release or phase.
- **SHOULD** means strongly recommended; deviations require a documented reason.
- **MAY** means optional or suitable for a later release.
- A feature marked **Later** is deliberately outside the initial MVP unless the roadmap is changed.
- Requirements are written to be testable wherever practical. A feature is not complete merely because its screen or API exists; it must also handle errors, authorization, cleanup and relevant security cases.

This is a product baseline, not a claim that every feature belongs in the first release. The phased plan is the delivery contract. Do not pull later-phase features into the MVP without reviewing their impact on security, reliability and scope.

### 0.1 Decisions established by this specification

| Area | Baseline decision |
|---|---|
| Product concept | A visual web desktop/workspace for managing remote Linux servers |
| Target users | Developers, self-hosters, homelab users, hobbyists and system administrators |
| Primary platform | Web browser on desktop; responsive support for smaller screens where practical |
| Deployment | Self-hosted by the operator; no mandatory Drovik cloud account or hosted control plane |
| Remote connectivity | SSH, including SFTP where the server exposes the SFTP subsystem |
| Remote agent | Not required; Drovik must not install a daemon, agent or persistent helper on managed servers |
| Remote privilege | Connect as a normal SSH user; do not require root. Use only the privileges already available to that account, unless a later explicit and opt-in privilege mechanism is designed |
| Frontend | React + TypeScript + Vite; Tailwind CSS may be used for styling and layout |
| Backend | Go, with a clearly separated HTTP API, WebSocket handling, SSH services and persistence layer |
| SSH implementation | Go SSH library (`golang.org/x/crypto/ssh`) or a reviewed compatible implementation |
| SFTP implementation | Go SFTP client library such as `github.com/pkg/sftp` |
| Terminal | xterm.js in the browser, backed by an interactive SSH PTY channel |
| Database | SQLite for the initial self-hosted deployment; persistence layer should avoid unnecessary coupling so a server database can be added later |
| Realtime communication | WebSockets for terminal I/O and live session updates; normal HTTP APIs for standard operations |
| Initial managed OS | Linux with OpenSSH-compatible SSH access; capability detection is required because distributions and installed tools differ |
| Primary data handling | Store connection metadata, configuration and audit metadata; do not upload or retain remote files unless a user explicitly requests an operation that requires it |
| Network model | Browser communicates with Drovik backend; backend establishes SSH connections to managed hosts. The browser must never receive stored private keys or SSH passwords |
| License direction | AGPL-3.0 to support an open-source self-hosted product while requiring source availability for modified network-served versions |

### 0.2 Product name note

“Drovik” is a proposed working name, not a cleared brand. Before a public launch, complete domain registration checks, GitHub namespace checks, company-name checks and professional trademark clearance in relevant jurisdictions and classes. Do not buy branded assets or make a legal availability claim based only on search-engine results.

---

# 1. Product vision and definition

## 1.1 Vision

Drovik is an open-source, self-hosted web workspace that makes remote Linux server administration more approachable by bringing common server operations into one visual environment.

A user opens Drovik in a browser, adds one or more servers using SSH credentials, and works with those servers through a desktop-like interface. The workspace provides a file manager, terminal, system overview, process controls, service controls and log access without requiring a custom agent to be installed on each remote server.

The product should combine the convenience of a graphical desktop metaphor with the reliability and transparency of normal Linux administration tools. It must not hide the fact that operations are executed on a remote host, and it must not pretend an operation succeeded until the remote host confirms it.

## 1.2 Problem statement

Managing several Linux servers commonly requires switching between SSH clients, SFTP clients, monitoring dashboards, service commands and log viewers. This can be inconvenient for users who manage personal servers, homelabs, development machines or small production environments.

Existing server panels may require an agent, a particular distribution, a specific deployment model or a separate service to be installed on every managed host. Some are focused on hosting, containers or system configuration rather than a unified workspace.

Drovik addresses this by providing a self-hosted control interface that uses the SSH access the operator already has.

## 1.3 Product goals

1. Provide one place to connect to and work with multiple Linux servers.
2. Make common tasks discoverable through a visual desktop-like workspace.
3. Keep a full interactive terminal available for advanced tasks.
4. Support secure file browsing and file operations through SFTP.
5. Display useful system health information without requiring an agent.
6. Allow users to inspect and manage processes and system services when their SSH account has the necessary permissions.
7. Make the product easy to self-host and maintain.
8. Keep the core product open-source and avoid mandatory vendor cloud services.
9. Preserve the normal Linux permission model: Drovik must not silently bypass the privileges of the connected user.
10. Provide clear feedback when a remote system lacks a required command, capability or permission.

## 1.4 Non-goals

The following are explicitly not core product goals for the initial release:

- Replacing a full Linux desktop environment or remote graphical desktop protocol.
- Replacing SSH itself or hiding the terminal from experienced administrators.
- Providing a general-purpose cloud hosting provider, VPS marketplace or managed hosting service.
- Requiring a Drovik account hosted by the project maintainers.
- Installing a permanent agent or daemon on every managed server.
- Providing a root shell by default or automatically escalating privileges.
- Automatically changing firewall rules, network settings, kernel settings or boot configuration without an explicit user action.
- Acting as a full configuration-management system such as Ansible, Puppet or Chef.
- Acting as a full monitoring/alerting platform with long-term time-series storage in the MVP.
- Providing a full backup and disaster-recovery solution in the MVP.
- Providing Kubernetes management in the MVP.
- Guaranteeing support for every Unix-like OS or every Linux distribution in the first release.
- Running arbitrary commands without making the execution context and risk clear.

## 1.5 Product principles

### Agentless by default

Drovik must use SSH and SFTP capabilities already available on the target host. It must not install a remote service, background process, package or scheduled task as a prerequisite.

### Least privilege

Drovik must operate as the authenticated SSH user. It must not require root. Actions that the remote user is not allowed to perform must fail safely with an understandable permission message.

### Transparent operations

Users should be able to understand which server they are working on, which account is connected, what an action will do and whether it succeeded. Dangerous actions require clear warnings and confirmation.

### Terminal remains first-class

The graphical tools are helpers, not a replacement for SSH. An interactive terminal must be a core capability.

### No false success

A request being accepted by the browser or backend is not proof that the remote operation succeeded. Status must reflect the remote command or SFTP result.

### Local ownership of data

The self-hosting operator controls stored data, connection credentials, logs and backups. The project must not transmit secrets or operational data to a third-party service by default.

### Capability-aware behavior

Drovik must detect or handle missing tools and OS differences instead of assuming every machine has the same commands or service manager.

---

# 2. Target users, use cases and roles

## 2.1 Primary user groups

### Hobbyist / self-hoster

- Manages one or several VPS instances, home servers or small Linux machines.
- Wants a convenient way to inspect files, CPU/RAM/disk usage and running services.
- May not remember every Linux command.
- Needs a terminal for tasks that are not covered by the graphical tools.

### Developer

- Uses remote development, staging or test servers.
- Frequently edits configuration files, checks logs, restarts services and transfers artifacts.
- Wants fast navigation between hosts.

### System administrator

- Manages multiple Linux hosts.
- Needs reliable visibility, clear permissions and predictable behavior.
- Expects the interface to show errors rather than suppressing them.

### Homelab operator

- Manages devices with different distributions, hardware and available commands.
- May have limited privileges on some systems.
- Needs capability detection and compatibility fallbacks.

## 2.2 Personas and access levels

The first release can be deployed for a single trusted operator. However, the data model and authorization boundaries must be designed so multi-user support can be added without rewriting the system.

Recommended roles when multi-user mode is introduced:

| Role | Capabilities |
|---|---|
| Owner / Administrator | Configure Drovik, manage users, add/edit/delete server connections, inspect audit history, configure security and backups |
| Operator | Access assigned servers and perform allowed operations using the connected SSH identity |
| Read-only user | View server status, metrics, processes, services and logs where remote permissions allow; cannot mutate server state |

The application role must not be confused with the remote Linux identity. Drovik authorization decides who may request an operation; Linux permissions decide whether the SSH account can execute it.

## 2.3 Core user journeys

### Journey A — First-time setup

1. Operator deploys Drovik using Docker Compose or a supported native package.
2. Operator opens the application over HTTPS or a trusted local connection.
3. Operator completes first-run setup and creates the initial administrator account.
4. Operator is asked to configure a strong password and, where available, an encryption secret.
5. Operator lands on an empty workspace with guidance to add a server.
6. Operator adds an SSH host, port, username and authentication method.
7. Drovik connects and presents the host key fingerprint for first-time verification.
8. Operator verifies and approves the fingerprint.
9. Drovik saves the connection and reports the capabilities detected on the host.

### Journey B — Add and connect to a server

1. User chooses “Add server”.
2. User enters a display name, host or IP address, SSH port, username and authentication method.
3. User selects password, private key, or supported SSH agent integration if implemented.
4. Drovik validates fields locally and attempts a bounded SSH connection.
5. On an unknown host key, Drovik displays the fingerprint and requires explicit trust approval.
6. On a changed host key, Drovik blocks the connection and explains that the key changed; user must deliberately investigate and re-approve it.
7. Drovik validates authentication and SFTP availability separately.
8. Drovik displays connection and capability status.

### Journey C — Inspect and work with files

1. User selects a connected server.
2. User opens the file manager.
3. Drovik lists a directory through SFTP.
4. User navigates folders, searches within the current directory where supported, downloads or uploads files, renames or deletes an item, and opens a supported text file.
5. A text edit is saved only after the user chooses Save.
6. Errors caused by permissions, disk space, network failure or unsupported operations are shown without losing the current context where possible.

### Journey D — Use the terminal

1. User opens a terminal window for a selected server.
2. Drovik creates an SSH session and requests a PTY.
3. The browser displays terminal output through xterm.js.
4. User types commands and sees output incrementally.
5. Resize events are passed to the remote PTY.
6. User can close the terminal without terminating the whole server connection.
7. Drovik closes SSH channels and releases resources when the terminal is closed or the user logs out.

### Journey E — Inspect system health

1. User selects a server.
2. Drovik reads a bounded set of system information using non-persistent SSH commands and/or standard Linux interfaces.
3. Dashboard displays CPU, memory, storage, uptime, load and network information where available.
4. Metrics refresh at a controlled interval.
5. If a metric is unavailable, the UI marks it unavailable rather than displaying a fabricated zero.

### Journey F — Manage a service or process

1. User opens Services or Processes.
2. Drovik displays current remote state and the remote identity.
3. User selects an action such as restart a service or terminate a process.
4. Drovik validates the action, applies confirmation rules and sends a structured request.
5. Backend runs a fixed, parameterized operation rather than building an unsafe shell command from arbitrary input.
6. Drovik confirms the resulting state by re-reading remote status.
7. UI displays success only after verification; otherwise it displays the actual error or uncertain state.

---

# 3. Scope by release

## 3.1 MVP definition

The MVP is the smallest complete version that proves the agentless server-workspace concept. It is not just a landing page plus SSH connection.

The MVP MUST include:

- Secure first-run setup and authentication.
- A server connection manager supporting multiple SSH hosts.
- SSH host key verification and secure credential handling.
- A desktop/workspace shell with server context always visible.
- Interactive SSH terminal with PTY and resize support.
- SFTP-based file manager with essential operations.
- Basic system overview and live metrics.
- Process list and safe process termination.
- systemd service list and common controls where systemd exists.
- Basic log viewing for supported systemd/journal-based hosts.
- Clear capability detection and error handling.
- Activity/audit metadata for relevant actions.
- Deployment documentation, backup guidance and upgrade process.
- Automated tests for security-sensitive and core workflows.

## 3.2 Explicitly deferred from the MVP

- Remote Docker/container management.
- Kubernetes management.
- File synchronization between servers.
- Scheduled tasks and cron editor.
- Full network configuration editor.
- Firewall rule editor.
- User/group management.
- Package manager / software installation UI.
- One-click server provisioning.
- Remote desktop streaming.
- Persistent shell session manager/tmux-like orchestration.
- Long-term metrics retention, dashboards and alerting.
- Automated backup/restore orchestration.
- Multiple identity providers and enterprise SSO.
- Complex multi-tenant SaaS hosting.

---

# 4. Functional requirements

The requirements below are grouped by capability. Each requirement has an identifier for traceability in issues, tests and release notes.

## 4.1 First-run setup and application settings

### Requirements

- **SET-001 — First-run detection:** The application MUST detect whether initial setup has been completed.
- **SET-002 — Initial administrator:** The first-run wizard MUST allow creation of the initial administrator account.
- **SET-003 — One-time setup:** Once an administrator has been created, the public setup route MUST be disabled. Resetting setup must require an explicit administrative recovery procedure.
- **SET-004 — Secure password storage:** Passwords MUST be stored using a modern password hashing function such as Argon2id or bcrypt with appropriate parameters; never as plaintext or reversible encryption.
- **SET-005 — Session policy:** The application MUST support configurable session expiry and logout.
- **SET-006 — Application identity:** The settings page MUST display the Drovik version, build identifier and database schema version.
- **SET-007 — Configuration validation:** Invalid or missing mandatory configuration MUST produce a clear startup error. Secrets MUST not be printed in logs.
- **SET-008 — Time settings:** The application MUST use UTC for stored timestamps and display a user-appropriate local time.
- **SET-009 — Backup configuration:** The operator MUST be given documented instructions for backing up the database and encryption key material together.
- **SET-010 — Safe defaults:** New installations MUST begin with conservative security settings and no remote servers configured.

### First-run acceptance criteria

- A clean installation presents setup instead of a login screen.
- A second attempt to use first-run setup is rejected after initialization.
- The initial password is not recoverable from the database.
- Restarting the service preserves the account and settings.
- Missing encryption configuration is handled according to the documented deployment mode and never causes silent plaintext storage.

## 4.2 Authentication, sessions and authorization

### Requirements

- **AUTH-001:** Unauthenticated users MUST NOT access server data or operation endpoints.
- **AUTH-002:** Login attempts MUST be rate-limited and protected against brute-force attempts.
- **AUTH-003:** Password verification MUST use constant-time comparison through the chosen password-hashing implementation.
- **AUTH-004:** Session cookies MUST be HttpOnly and SameSite; Secure MUST be enabled when served over HTTPS.
- **AUTH-005:** State-changing HTTP requests MUST have CSRF protection when cookie-based authentication is used.
- **AUTH-006:** Logout MUST invalidate the active session on the server.
- **AUTH-007:** Expired and revoked sessions MUST be rejected by both HTTP and WebSocket endpoints.
- **AUTH-008:** WebSocket upgrades MUST verify the authenticated session and allowed Origin.
- **AUTH-009:** Authorization MUST be enforced in the backend, not only by hiding frontend controls.
- **AUTH-010:** A user MUST only access servers assigned to that user's account when multi-user access control is enabled.
- **AUTH-011:** Read-only users MUST be blocked from mutating endpoints even if they manually call the API.
- **AUTH-012:** Sensitive account changes MUST require re-authentication or an equivalent recent-authentication check.
- **AUTH-013:** Password change MUST invalidate other active sessions, unless a documented policy explicitly allows selected sessions to remain.
- **AUTH-014:** The system MUST avoid user enumeration through login error messages.

### Authentication scope

For the initial single-operator MVP, local username/password authentication is sufficient. The codebase must still define clear authorization middleware and ownership boundaries so multi-user support does not depend on frontend-only assumptions.

### Later authentication options

- OIDC/OAuth 2.0 login.
- LDAP integration.
- Passkeys/WebAuthn.
- TOTP-based multi-factor authentication.
- Reverse-proxy authentication integration, only with carefully validated trusted-proxy configuration.

## 4.3 SSH server connection manager

### Requirements

- **SSH-001 — Multiple servers:** Users MUST be able to save and manage more than one SSH server connection.
- **SSH-002 — Connection fields:** A connection MUST support a display name, hostname/IP, port, username, authentication method and optional notes/tags.
- **SSH-003 — Default port:** The default SSH port MUST be 22, while allowing a custom port.
- **SSH-004 — Authentication methods:** Initial support SHOULD include password authentication and private-key authentication. Encrypted private keys MUST be supported with a passphrase prompt or securely configured passphrase handling.
- **SSH-005 — Key formats:** The implementation MUST document supported private-key formats and return a useful error for unsupported formats.
- **SSH-006 — Host key verification:** The client MUST verify the SSH host key. It MUST NOT disable host-key checking for convenience.
- **SSH-007 — First connection:** Unknown host keys MUST be shown to the user with the fingerprint and a clear trust decision.
- **SSH-008 — Changed host key:** A changed host key MUST block connection by default and require deliberate re-verification. The UI MUST warn that the change may indicate a legitimate server rebuild or a man-in-the-middle attack.
- **SSH-009 — Known-host store:** Approved host keys MUST be saved in a persistent known-hosts store associated with the application or connection.
- **SSH-010 — Connection test:** Users MUST be able to test a connection before saving or from the connection details page.
- **SSH-011 — Timeouts:** TCP dial, SSH handshake, authentication and command operations MUST have bounded timeouts.
- **SSH-012 — Resource limits:** The backend MUST limit concurrent SSH sessions per user and globally.
- **SSH-013 — Session cleanup:** SSH clients, channels, PTYs, SFTP clients and goroutines MUST be closed when no longer needed.
- **SSH-014 — Reconnect:** A failed connection MUST be marked disconnected. Reconnection MUST be explicit or follow a documented bounded retry policy.
- **SSH-015 — Connection status:** The interface MUST distinguish saved, connecting, connected, degraded and disconnected states.
- **SSH-016 — No silent credential fallback:** If an authentication method fails, Drovik MUST NOT silently try unrelated stored credentials.
- **SSH-017 — No agent forwarding:** SSH agent forwarding MUST be disabled by default and not exposed in the MVP.
- **SSH-018 — Port forwarding:** Local, remote and dynamic port forwarding MUST be disabled in the MVP unless separately threat-modeled and intentionally added.
- **SSH-019 — Identity display:** The UI MUST show the remote host and username associated with an active terminal or operation.
- **SSH-020 — Delete connection:** Removing a saved connection MUST remove or invalidate its stored credentials and associated metadata according to the retention policy. It MUST NOT modify the remote server.
- **SSH-021 — Edit connection:** Editing host, port or username MUST trigger a new validation and host-key check as appropriate.
- **SSH-022 — Tags/groups:** The application SHOULD support server tags or groups for organization.
- **SSH-023 — Import/export:** Later versions MAY support encrypted connection configuration import/export. Plaintext credential export MUST NOT be offered.
- **SSH-024 — Network policy:** The backend MUST validate destination addresses and apply configurable egress policy to prevent unintended access to local services, cloud metadata endpoints or arbitrary internal networks. Private/LAN addresses may be legitimate targets in a self-hosted installation, so restrictions must be configurable by the operator rather than blindly blocking all private addresses.

### Authentication and credential storage requirements

- Passwords and private keys MUST be encrypted at rest using authenticated encryption such as AES-256-GCM or ChaCha20-Poly1305.
- The encryption key MUST be supplied through a protected environment secret, mounted secret file or supported OS secret store. It MUST NOT be stored beside the ciphertext in the SQLite database.
- The operator MUST be warned that losing the encryption key can make saved credentials unrecoverable.
- A deployment backup MUST include the database and encryption key through separate, protected handling.
- Secrets MUST be redacted from logs, errors, traces and audit events.
- Private keys MUST never be returned to the frontend after initial submission.
- Credentials SHOULD be decrypted only for the shortest period required to establish a session.
- If a credential is cached in memory for a live session, it MUST be cleared when that session is closed where practical.
- The product MUST document the risks of storing SSH credentials on a machine reachable over a network.

### Connection acceptance criteria

- A valid password-authenticated host can be added and connected.
- A valid key-authenticated host can be added and connected.
- An unknown host key requires user approval.
- A changed host key blocks access until re-approved.
- Wrong credentials produce a useful error without leaking secret values.
- An unreachable host times out and does not leave an orphaned session.
- Restarting Drovik preserves approved host keys and saved connection data.
- Deleting a saved connection removes its stored credential material.

## 4.4 Workspace and desktop shell

### Requirements

- **DESK-001:** The application MUST provide a consistent workspace for navigating server tools.
- **DESK-002:** The current server context MUST remain visible while the user works in a server-specific tool.
- **DESK-003:** The user MUST be able to switch between saved servers without re-entering connection details.
- **DESK-004:** The workspace SHOULD support multiple open tool windows or panels where this does not compromise usability.
- **DESK-005:** The interface MUST clearly distinguish a local application action from an action executed on a remote server.
- **DESK-006:** Every server-specific window MUST display the target server identity.
- **DESK-007:** The UI MUST make connection state visible and handle disconnection without pretending the tool remains live.
- **DESK-008:** Open terminal and SFTP sessions MUST be closed or safely detached according to a defined policy when the user logs out or the browser session expires.
- **DESK-009:** The interface MUST show loading, empty, success, warning, permission-denied, timeout and error states.
- **DESK-010:** A failed tool window MUST not crash the entire application workspace.
- **DESK-011:** The application SHOULD remember non-sensitive workspace preferences such as panel arrangement and selected server.
- **DESK-012:** Sensitive state such as passwords, private keys and passphrases MUST NOT be saved in browser local storage.

### Window behavior

If floating windows are implemented:

- Windows MUST have a clear title and server identity.
- Windows MUST be closable and brought to the foreground when selected.
- A window MUST not display data from a previous server after the user switches its context.
- Long-running sessions such as a terminal MUST have explicit close and disconnect behavior.
- Minimum window dimensions and viewport handling MUST prevent core controls becoming inaccessible.

A simpler tab/panel workspace is acceptable for the MVP. A complex operating-system simulation is not a prerequisite for delivering the core product.

## 4.5 File manager (SFTP)

### Requirements

- **FILE-001 — Directory listing:** Drovik MUST list remote directories through SFTP.
- **FILE-002 — Navigation:** Users MUST be able to navigate directories, move to the parent directory and return to a known location.
- **FILE-003 — Metadata:** Listings SHOULD show name, type, size, permissions, owner/group where available and last-modified timestamp.
- **FILE-004 — Sorting:** Users SHOULD be able to sort the current listing by name, type, size and modified time.
- **FILE-005 — Hidden files:** Users MUST be able to show or hide dotfiles.
- **FILE-006 — Upload:** Users MUST be able to upload one or more files to a selected remote directory.
- **FILE-007 — Download:** Users MUST be able to download a remote file.
- **FILE-008 — Upload progress:** Large transfers MUST display progress and allow cancellation where the transport supports it.
- **FILE-009 — Download progress:** Downloads MUST display progress where file size is known and cancellation MUST close the active transfer cleanly.
- **FILE-010 — Create folder:** Users MUST be able to create a directory when remote permissions allow.
- **FILE-011 — Create file:** Users SHOULD be able to create an empty file when remote permissions allow.
- **FILE-012 — Rename:** Users MUST be able to rename files and folders.
- **FILE-013 — Move:** Users SHOULD be able to move files and folders within the same server using SFTP rename semantics where supported.
- **FILE-014 — Delete:** Users MUST be able to delete files and empty directories. Recursive deletion MUST require a more explicit confirmation and must never be accidental.
- **FILE-015 — Permissions:** Users SHOULD be able to inspect permissions and, in a later iteration, change mode bits when permitted. Ownership changes are not required for MVP.
- **FILE-016 — Text editor:** Users MUST be able to open and edit a bounded set of text files.
- **FILE-017 — Save behavior:** An edited file MUST only be written to the remote server after an explicit Save action.
- **FILE-018 — Unsaved changes:** Closing an editor with unsaved changes MUST prompt the user.
- **FILE-019 — File size limits:** The application MUST impose configurable limits for in-browser editing to avoid loading huge files into memory.
- **FILE-020 — Binary detection:** Binary or unsupported files MUST not be opened as ordinary text without a warning.
- **FILE-021 — Transfer collision:** Uploading or renaming onto an existing path MUST use a clear overwrite/rename/cancel choice.
- **FILE-022 — Path safety:** Backend path operations MUST prevent path traversal and must operate within the requested SFTP path context.
- **FILE-023 — Symlinks:** Symlinks MUST be identified and handled safely. The file manager MUST not silently follow a symlink into an unintended location when doing recursive operations.
- **FILE-024 — Partial transfer:** Interrupted transfers MUST be reported as incomplete. Partial local or remote files SHOULD be cleaned up or clearly marked according to the operation.
- **FILE-025 — Permission feedback:** Permission-denied errors MUST identify the failed action and path without exposing credentials.
- **FILE-026 — SFTP capability:** If SFTP is unavailable, Drovik MUST explain that the host's SFTP subsystem is unavailable; it MUST NOT silently switch to unsafe shell-based file operations.
- **FILE-027 — No automatic upload:** Opening a file in the editor MUST NOT upload it to an external service.
- **FILE-028 — Local file selection:** Upload selection MUST be restricted to files deliberately selected by the user through the browser.
- **FILE-029 — Refresh:** Users MUST be able to refresh the current directory listing.
- **FILE-030 — Stale editor detection:** Before saving an edited file, Drovik SHOULD detect if the remote file changed since it was opened and warn before overwriting newer content.

### File editor baseline

The MVP editor should support plain text and common configuration/log formats. Syntax highlighting is useful but secondary to correctness. It should support line numbers, basic search, unsaved-change indication and explicit save/cancel behavior.

Initial editable formats may include:

- `.txt`, `.md`, `.json`, `.yaml`, `.yml`, `.toml`, `.ini`, `.conf`, `.env` and common script/configuration extensions.
- Unknown extensions may be opened as plain text only when file-size and binary checks pass.
- Sensitive-looking files such as private keys, credential files and environment files should show a caution before editing or copying content. This is a warning, not an attempt to block legitimate administration.

### File manager acceptance criteria

- User can browse from the SFTP home directory to permitted subdirectories.
- Dotfiles can be displayed.
- Upload/download works for ordinary files and reports progress.
- A permission-denied upload or save is reported without claiming success.
- Deleting a directory with contents requires explicit confirmation and a clear recursive-delete warning.
- An interrupted transfer does not appear as a successful complete transfer.
- An editor warns about unsaved changes.
- A remote file changed by another process is not silently overwritten where change detection is supported.

## 4.6 Interactive terminal

### Requirements

- **TERM-001 — PTY:** Drovik MUST establish an interactive SSH channel with a PTY for terminal sessions.
- **TERM-002 — Browser terminal:** The frontend MUST use a terminal emulator such as xterm.js rather than a plain textarea.
- **TERM-003 — Bidirectional I/O:** Keyboard input and remote output MUST stream in both directions.
- **TERM-004 — Resize:** Browser terminal size changes MUST be propagated to the SSH PTY.
- **TERM-005 — Multiple sessions:** Users SHOULD be able to open more than one terminal session, subject to configured limits.
- **TERM-006 — Server identity:** Every terminal MUST display the target host and remote username.
- **TERM-007 — Session lifecycle:** Closing a terminal MUST close its SSH channel and release associated resources.
- **TERM-008 — Disconnect handling:** Network loss MUST be shown clearly. The terminal MUST not silently send buffered commands after an uncertain reconnect.
- **TERM-009 — Output encoding:** Terminal output MUST be handled safely, including control sequences and resize events.
- **TERM-010 — Authentication:** A terminal WebSocket MUST require an authenticated, authorized application session.
- **TERM-011 — Origin validation:** WebSocket Origin MUST be validated against configured allowed origins.
- **TERM-012 — Input limits:** WebSocket message size and rate MUST be bounded to protect the backend.
- **TERM-013 — No default transcript storage:** Terminal input/output MUST NOT be persistently recorded by default.
- **TERM-014 — Clear session:** The user MUST be able to close a terminal session explicitly.
- **TERM-015 — Paste safety:** The interface SHOULD warn on large or multi-line paste operations because pasted commands may execute immediately in the remote shell.
- **TERM-016 — Clipboard permissions:** Clipboard read/write MUST follow browser permission and user interaction rules.
- **TERM-017 — No command rewriting:** Drovik MUST NOT alter commands typed into the terminal.
- **TERM-018 — Environment:** The application SHOULD request a sensible terminal type such as `xterm-256color` when supported.
- **TERM-019 — Shell exit:** Remote shell exit status and close reason SHOULD be reported.
- **TERM-020 — Resource cleanup:** Browser disconnect, user logout, idle timeout and backend shutdown MUST clean up terminal resources according to a documented policy.

### Terminal acceptance criteria

- An interactive shell behaves like a normal SSH terminal for common command-line applications.
- `vim`, `top` or comparable interactive terminal programs work to the extent supported by the remote terminal and terminal emulator.
- Terminal resize updates the remote PTY dimensions.
- Closing the browser or terminal does not leave an unmanaged backend channel indefinitely.
- A user cannot open a terminal to a server they are not authorized to access.

## 4.7 System overview and monitoring

### Purpose

Provide a lightweight, current snapshot of the managed host without installing a permanent monitoring agent. Metrics are fetched through bounded SSH operations and standard Linux interfaces when possible.

### Requirements

- **MON-001 — Overview:** Each server MUST have a system overview.
- **MON-002 — Host identity:** Display hostname, operating system/distribution where detectable, kernel version, architecture and uptime.
- **MON-003 — CPU:** Display CPU utilization or a clearly labeled load/usage approximation, with the measurement method documented.
- **MON-004 — Memory:** Display total, used, available and percentage where the source provides reliable values.
- **MON-005 — Storage:** Display mounted filesystem capacity and usage.
- **MON-006 — Network:** Display available interface names and traffic counters where accessible; rates may be calculated from counter differences.
- **MON-007 — Load:** Display load average where supported.
- **MON-008 — Refresh:** Metrics MUST refresh at a controlled interval with a configurable minimum.
- **MON-009 — Missing data:** Unavailable metrics MUST be labeled unavailable; they MUST NOT be represented as zero.
- **MON-010 — Polling limits:** The backend MUST prevent excessive concurrent or duplicate metric polling for the same host.
- **MON-011 — Timeout:** Metric collection MUST have a bounded timeout.
- **MON-012 — Failure isolation:** A failed metric query MUST not prevent other metrics from rendering.
- **MON-013 — No agent:** Monitoring MUST NOT require a persistent daemon on the remote server.
- **MON-014 — Data freshness:** The UI MUST display when the metric snapshot was last updated.
- **MON-015 — Capability detection:** Metrics MUST adapt to missing commands and limited `/proc` availability.
- **MON-016 — No invented precision:** Estimated values MUST be labeled as estimates and not shown with misleading precision.

### Suggested metric sources

Use standard interfaces and commands conservatively:

- `/proc/stat` for CPU counters.
- `/proc/meminfo` for memory values.
- `/proc/loadavg` for load averages.
- `/proc/uptime` for uptime.
- `/proc/net/dev` for interface counters.
- `df` for mounted filesystem usage.
- `uname` and `/etc/os-release` for operating system identification.

The backend should prefer a small number of bounded, predictable queries rather than launching many commands per refresh. The implementation must not assume every file or utility exists.

### Metric interpretation rules

- CPU percentage requires at least two counter samples; the first sample alone cannot produce a valid utilization percentage.
- Memory calculations should use `MemAvailable` when present rather than treating all cache as unavailable memory.
- Filesystem usage should identify the mount point and filesystem and should handle pseudo-filesystems appropriately.
- Network throughput must be calculated from counter deltas over a known interval; a single byte counter is not a rate.
- Counter resets, interface recreation and host reboot must reset rate calculation state.
- When metrics are stale because a host is disconnected, the UI must show the last successful timestamp and stale status.

## 4.8 Process manager

### Requirements

- **PROC-001:** Display a process list where remote permissions permit.
- **PROC-002:** Show PID, parent PID where available, owner, CPU, memory, state and command summary where supported.
- **PROC-003:** Support sorting and refresh.
- **PROC-004:** Support filtering/search within the loaded process list.
- **PROC-005:** Display the remote account and permission context.
- **PROC-006:** Allow sending SIGTERM to a selected process where permitted.
- **PROC-007:** SIGKILL MUST require an additional explicit confirmation and should be a later MVP follow-up if time is constrained.
- **PROC-008:** Drovik MUST NOT imply that it can terminate processes owned by another user when the SSH account lacks permission.
- **PROC-009:** Process actions MUST use structured parameters and validated PIDs; do not concatenate arbitrary UI input into shell commands.
- **PROC-010:** PID values MUST be validated as positive integers and rechecked to reduce the chance of acting on a reused PID.
- **PROC-011:** The UI MUST warn that terminating a process can cause data loss or service interruption.
- **PROC-012:** A process termination request MUST be followed by a refresh or status check.
- **PROC-013:** Failed or denied operations MUST be displayed accurately.

### Process manager safety

- Do not provide “kill all” in the MVP.
- Do not allow process termination through an unvalidated command string.
- Confirm the process identity at the time of action as far as practical.
- Make the distinction between SIGTERM and SIGKILL explicit.
- Do not automatically use sudo.

## 4.9 Service manager

### Requirements

- **SVC-001:** Detect whether systemd is available on the host.
- **SVC-002:** If systemd is available, display service/unit names and current state.
- **SVC-003:** Show active, inactive, failed and other reported states distinctly.
- **SVC-004:** Allow viewing service details where supported.
- **SVC-005:** Allow start, stop and restart actions where permitted.
- **SVC-006:** Reload MAY be supported for units that expose the relevant operation.
- **SVC-007:** Enable/disable at boot SHOULD be included after start/stop/restart are stable.
- **SVC-008:** Destructive or service-impacting actions MUST require confirmation.
- **SVC-009:** Drovik MUST re-read status after an operation and report the resulting state.
- **SVC-010:** The service manager MUST NOT silently escalate privileges.
- **SVC-011:** Service names MUST be validated against the discovered list or an explicitly allowed unit identifier pattern.
- **SVC-012:** A service manager interface MUST clearly show when the host does not use systemd or the current user lacks access.
- **SVC-013:** A service action failure MUST show the remote error in a safe, understandable form.
- **SVC-014:** Unit-file editing and arbitrary unit creation are outside the MVP.

### Service manager acceptance criteria

- systemd hosts show available services and their status.
- A user can restart a service they are permitted to manage.
- A user without permission sees a permission error, not a false success.
- A non-systemd host displays an unsupported/capability message instead of an empty list that could be mistaken for no services.

## 4.10 Logs viewer

### Requirements

- **LOG-001:** Provide a basic log viewer for hosts using systemd journal where `journalctl` is available.
- **LOG-002:** Support viewing recent system logs and logs for a selected systemd unit.
- **LOG-003:** Support a bounded line count and time range.
- **LOG-004:** Display timestamps and priority/severity where provided.
- **LOG-005:** Provide text search or filtering within the returned log window.
- **LOG-006:** Support refresh.
- **LOG-007:** Limit output size and command duration.
- **LOG-008:** Handle journal permission errors clearly.
- **LOG-009:** Do not retain full remote log contents on the Drovik server by default.
- **LOG-010:** Escape or safely render log text; log content is untrusted input and must not be interpreted as HTML or script.
- **LOG-011:** When journal access is not available, clearly state that the host does not expose the requested logs to the connected user.
- **LOG-012:** Later file-based log browsing MUST use SFTP or another carefully bounded mechanism and must preserve path-safety requirements.

### Log viewer boundaries

The initial viewer is for inspection, not a full SIEM. It does not provide guaranteed long-term retention, alerting, log indexing or cross-host correlation.

## 4.11 Activity history and audit metadata

### Requirements

- **AUD-001:** Record relevant application events such as login, logout, connection created/edited/deleted, host key approved/rejected, file mutation request, service action and process action.
- **AUD-002:** Record event timestamp, actor, target server reference, action type, outcome and request correlation ID where applicable.
- **AUD-003:** Do not record passwords, private keys, passphrases, session cookies or full terminal input/output.
- **AUD-004:** Avoid storing file contents in audit events.
- **AUD-005:** Failed operations SHOULD be recorded with a safe error category.
- **AUD-006:** Audit records MUST be protected from ordinary user modification.
- **AUD-007:** Admin users SHOULD be able to filter audit events by date, server, actor and action type.
- **AUD-008:** Retention MUST be configurable or documented.
- **AUD-009:** The application MUST not describe its own local audit database as tamper-proof or equivalent to a remote immutable audit service.

### Important distinction

Audit history should describe what Drovik was asked to do and the outcome it observed. It is not a replacement for the target server's own audit logs or shell history.

## 4.12 Notifications and user feedback

### Requirements

- **MSG-001:** All remote operations MUST have a visible result state.
- **MSG-002:** Errors MUST provide the action, target and safe reason where available.
- **MSG-003:** Success messages MUST be based on confirmed results.
- **MSG-004:** Long-running operations MUST show progress or a running state.
- **MSG-005:** The UI MUST distinguish warning, unsupported, permission denied, timeout, disconnected and failed states.
- **MSG-006:** Notifications MUST not expose secrets or full sensitive command output.
- **MSG-007:** An operation that times out after being sent MUST be marked as outcome unknown when the backend cannot determine whether it completed. It must not be labeled failed or successful without evidence.

## 4.13 Search, navigation and organization

### MVP

- Search/filter the saved server list by name or host.
- Search/filter the current directory listing.
- Search/filter the current process and service list.
- Keep current server context visible.

### Later

- Global command palette.
- Saved server groups and nested folders.
- Cross-server search.
- Favorites and recently used tools.
- Saved command snippets.
- User-defined dashboard layouts.

---

# 5. Technical architecture

## 5.1 High-level architecture

Drovik MUST use a browser-to-backend-to-SSH architecture:

```text
+---------------------------+
|        Web Browser        |
| React + TypeScript        |
| Workspace / xterm.js      |
+-------------+-------------+
              |
       HTTPS / WebSocket
              |
+-------------v-------------+
|       Drovik Backend      |
| Go HTTP API               |
| Authentication / RBAC     |
| SSH connection manager    |
| SFTP service              |
| Terminal session manager  |
| Monitoring collectors     |
| Process / service actions |
| Audit service             |
+------+--------------+-----+
       |              |
       |              +----------------+
       |                               |
+------v------+                 +------v------+
| SQLite DB   |                 | Secret/key  |
| Metadata    |                 | encryption  |
| Sessions    |                 | material    |
| Audit       |                 | (external)  |
+-------------+                 +-------------+
       |
       | SSH / SFTP
       |
+------v-----------------------------------+
|              Remote Linux Host           |
| OpenSSH server / SFTP subsystem           |
| Standard OS interfaces and commands      |
| No permanent Drovik agent installed       |
+------------------------------------------+
```

### Core rule

The browser MUST NOT connect directly to remote SSH hosts. All SSH credential handling and remote operation authorization MUST occur in the backend.

## 5.2 Frontend architecture

### Technology baseline

- React.
- TypeScript with strict compiler settings.
- Vite for development and production bundling.
- Tailwind CSS or an equivalent utility/style system.
- xterm.js for terminal rendering.
- A query/state management library MAY be used for API caching and server state.
- A small, consistent component system should be established rather than implementing every screen independently.

### Frontend module boundaries

Suggested modules:

```text
frontend/
  src/
    app/                 # app bootstrap, routes, providers
    api/                 # typed API client and request helpers
    auth/                # login, setup, session state
    workspace/           # desktop shell, panels, window/tab state
    servers/             # server list, forms, connection status
    terminal/            # xterm.js component and websocket lifecycle
    files/               # file browser, transfer UI, text editor
    monitoring/          # system overview and charts
    processes/            # process listing and actions
    services/             # service listing and actions
    logs/                 # log viewer
    activity/             # audit history
    settings/             # application configuration
    components/           # shared UI components
    hooks/                # shared React hooks
    types/                # shared frontend types
    utils/                # formatting, validation and safe helpers
```

### Frontend requirements

- API calls MUST go through a single typed client layer.
- Authentication state MUST be centralized.
- Server-specific requests MUST include an explicit server identifier.
- The selected server MUST not be inferred from stale component state.
- Every async operation MUST handle loading, error, cancellation and stale-response conditions.
- WebSocket connections MUST close on unmount, logout, server switch or session expiry.
- User-provided values MUST be rendered as text unless a safe, explicit rich-text renderer is required.
- File and log content MUST be treated as untrusted data.
- Sensitive values MUST not be logged to the browser console.
- Frontend route guards are usability controls only; backend authorization remains authoritative.

## 5.3 Backend architecture

### Technology baseline

- Go.
- HTTP router/framework selected for maintainability and middleware support.
- `golang.org/x/crypto/ssh` or a reviewed equivalent for SSH.
- `github.com/pkg/sftp` or a reviewed equivalent for SFTP.
- SQLite driver with migrations.
- WebSocket library with explicit origin, authentication and size limits.
- Structured logging with secret redaction.

### Suggested package layout

```text
backend/
  cmd/
    drovik/                 # application entry point
  internal/
    app/                    # dependency wiring and lifecycle
    config/                 # environment/config validation
    httpapi/                # routes, handlers and middleware
    websocket/              # websocket upgrade and message handling
    auth/                   # password hashing, sessions, authorization
    users/                  # user/account service
    servers/                # connection metadata and server ownership
    secrets/                # encryption/decryption and secret handling
    hostkeys/               # known-host storage and verification
    sshclient/              # SSH dialing, session pool and lifecycle
    terminal/               # PTY sessions and terminal streaming
    sftp/                   # remote file operations and transfer jobs
    monitoring/             # host metrics and capability collection
    processes/              # structured process operations
    services/               # systemd discovery and controls
    logs/                   # journal retrieval and filtering
    audit/                  # activity/audit event service
    storage/                # repositories and database access
    migrations/             # schema migrations
    jobs/                   # bounded background operations
    security/               # origin checks, SSRF controls, validation
  migrations/
  tests/
```

The exact package names can change, but the separation of concerns must remain. HTTP handlers should not contain raw SSH command construction, database encryption logic or permission rules.

### Backend service boundaries

- **HTTP/API layer:** request validation, authentication, authorization, response mapping.
- **Domain/services layer:** business rules and operation orchestration.
- **SSH layer:** connection creation, host key checks, session lifecycle and timeouts.
- **SFTP layer:** safe file operations and transfer lifecycle.
- **Persistence layer:** database access, migrations and transactions.
- **Secrets layer:** encryption/decryption using externally supplied key material.
- **Audit layer:** safe event recording.
- **Job layer:** bounded concurrent tasks and cancellation.

## 5.4 SSH connection lifecycle

A connection should be created on demand or from a bounded managed session pool. The implementation must avoid unbounded long-lived SSH connections.

Recommended lifecycle:

1. Validate the saved connection belongs to the authenticated user.
2. Load encrypted credentials and decrypt only as needed.
3. Resolve and validate the target address under configured egress policy.
4. Establish TCP connection with a timeout.
5. Verify server host key against the approved known-host record.
6. Authenticate using the selected method.
7. Create an SSH client/session or SFTP client for the requested operation.
8. Apply operation-specific timeout and output limits.
9. Return structured result data.
10. Close channels and release resources when complete, except for explicitly managed terminal sessions.

### Connection pool considerations

- Pools MUST be bounded per host and globally.
- Idle sessions MUST expire after a configurable period.
- Credentials MUST not be included in pool keys or debug strings.
- A server credential change MUST invalidate related cached sessions.
- Host key mismatch MUST immediately invalidate the attempted connection.
- A backend restart MUST close all in-memory sessions cleanly.
- Long-running terminal channels must be tracked separately from short command sessions.

## 5.5 Remote command execution policy

This is one of the most important architectural controls.

### Structured operations

System monitoring, process management, services and logs MUST use predefined operation handlers with validated parameters. Do not build shell commands by concatenating arbitrary user-controlled strings.

Examples:

- Service action accepts a validated unit name from a discovered list and an enumerated action (`start`, `stop`, `restart`, `reload`, `enable`, `disable`).
- Process action accepts a validated numeric PID and an enumerated signal.
- Log request accepts a unit identifier, bounded line count, time range and filter value that is safely passed/escaped.
- Monitoring uses fixed command templates or standard interfaces.

### Terminal exception

The interactive terminal intentionally allows the authenticated user to type arbitrary commands. That is a core feature and must be treated as a privileged capability. Drovik must make the target host and remote username visible and must not rewrite the command.

### Command execution controls

- Use fixed command templates and safe argument handling.
- Avoid shell interpretation for values when an exec API or direct file interface can be used.
- If shell execution is unavoidable, use rigorously validated parameters and safe quoting; do not trust a generic quote helper as a substitute for design.
- Bound output size, execution time and concurrency.
- Support cancellation when the SSH channel supports it.
- Do not run operations with sudo automatically.
- Do not rely on command output alone where a direct status recheck is possible.
- Do not expose arbitrary command execution through a “system operation” API route.

## 5.6 Data model

The initial database may use SQLite. The schema should be normalized enough to support future multi-user access and a later migration to PostgreSQL if needed.

Suggested logical entities:

### `users`

- `id`
- `username` or `email` (depending on login policy)
- `password_hash`
- `role`
- `is_active`
- `created_at`
- `updated_at`
- `last_login_at`

### `sessions`

- `id` or token hash
- `user_id`
- `created_at`
- `expires_at`
- `revoked_at`
- `last_seen_at`
- safe client metadata if required by policy

Store only a hash of the session token where practical.

### `servers`

- `id`
- `owner_user_id` or tenant reference
- `display_name`
- `host`
- `port`
- `username`
- `auth_method`
- `encrypted_credential` or reference to encrypted secret material
- `credential_nonce` / encryption metadata as required by the chosen AEAD scheme
- `tags` or normalized tag relationships
- `notes`
- `created_at`
- `updated_at`
- `last_connected_at`
- `is_active`

Do not store an SSH private key or password in plaintext.

### `known_hosts`

- `id`
- `server_id` or host/port identity
- `key_type`
- `public_key_fingerprint`
- `public_key_material` as required for verification
- `approved_at`
- `approved_by`
- `revoked_at`

### `audit_events`

- `id`
- `actor_user_id`
- `server_id` nullable
- `event_type`
- `target_summary`
- `outcome`
- `safe_error_code` nullable
- `correlation_id`
- `created_at`

### `app_settings`

- setting key
- setting value or encrypted value as appropriate
- update timestamp

### Future entities

- `server_groups`
- `server_tags`
- `user_server_permissions`
- `saved_commands`
- `saved_scripts`
- `transfer_jobs`
- `metric_snapshots` if historical metrics are introduced
- `notifications`
- `api_tokens` only if a secure API-token feature is justified

### Data retention rules

- Do not retain remote file contents by default.
- Do not retain terminal input/output by default.
- Do not retain full log content by default.
- Retain only the metadata necessary for the feature and audit requirements.
- Provide deletion and retention policies for user accounts, server connections, credentials and audit events.
- Backups must be treated as sensitive because they may contain encrypted credentials and account metadata.

## 5.7 API design

The API should be versioned from the beginning, for example under `/api/v1`.

Illustrative endpoint groups (final names can differ):

| Method | Endpoint | Purpose |
|---|---|---|
| `GET` | `/api/v1/health` | Health/readiness status; must not expose secrets |
| `POST` | `/api/v1/auth/login` | Authenticate |
| `POST` | `/api/v1/auth/logout` | Invalidate session |
| `GET` | `/api/v1/auth/me` | Return current user and role |
| `POST` | `/api/v1/setup` | One-time first-run setup |
| `GET` | `/api/v1/servers` | List authorized servers |
| `POST` | `/api/v1/servers` | Create server connection |
| `GET` | `/api/v1/servers/{id}` | Server detail |
| `PATCH` | `/api/v1/servers/{id}` | Update server metadata/credentials |
| `DELETE` | `/api/v1/servers/{id}` | Delete saved connection |
| `POST` | `/api/v1/servers/{id}/test` | Test connection |
| `POST` | `/api/v1/servers/{id}/trust-host-key` | Approve a presented host key after explicit confirmation |
| `GET` | `/api/v1/servers/{id}/capabilities` | Return detected capabilities |
| `GET` | `/api/v1/servers/{id}/system` | Current system overview |
| `GET` | `/api/v1/servers/{id}/processes` | Process listing |
| `POST` | `/api/v1/servers/{id}/processes/{pid}/signal` | Send an allowed signal |
| `GET` | `/api/v1/servers/{id}/services` | Service listing |
| `POST` | `/api/v1/servers/{id}/services/{unit}/action` | Perform an enumerated service action |
| `GET` | `/api/v1/servers/{id}/logs` | Retrieve bounded log data |
| `GET` | `/api/v1/servers/{id}/files` | List a remote directory |
| `POST` | `/api/v1/servers/{id}/files/upload` | Upload a file |
| `GET` | `/api/v1/servers/{id}/files/download` | Download a file |
| `POST` | `/api/v1/servers/{id}/files/mkdir` | Create a directory |
| `PATCH` | `/api/v1/servers/{id}/files/rename` | Rename/move an item |
| `DELETE` | `/api/v1/servers/{id}/files` | Delete an item subject to confirmation rules |
| `GET` | `/api/v1/audit` | Query authorized audit events |

The terminal should use a separate WebSocket route, for example:

`/api/v1/servers/{id}/terminal`

### API requirements

- All request and response payloads MUST have defined schemas.
- Validate path, query and body values on the backend.
- Use stable error codes in addition to human-readable messages.
- Do not return stack traces or secret material to the browser.
- Apply authorization checks at the resource level.
- Use pagination for potentially large result sets.
- Apply request-size and response-size limits.
- Define cancellation behavior for long-running operations.
- Use consistent timestamps and status semantics.
- Keep API documentation in the repository and generate an OpenAPI specification where practical.

## 5.8 WebSocket design

WebSockets are required for the terminal and may later be used for progress/status streams.

Requirements:

- Authenticate the connection during upgrade.
- Validate Origin against configured allowed origins.
- Authorize access to the referenced server.
- Do not put passwords, private keys or reusable long-lived bearer credentials in the WebSocket URL.
- Enforce maximum message size and rate limits.
- Use typed message envelopes for resize, input, output, status and close events.
- Ensure one session's output cannot be routed to another user's socket.
- Stop and clean up sessions after logout, timeout, explicit close or backend shutdown.
- Handle ping/pong and dead connections.
- Protect against a client sending malformed resize or control messages.

## 5.9 Capability detection and compatibility layer

Drovik must not assume every target host has the same tools.

At connection time or on first use, detect capabilities such as:

- Operating system and distribution information.
- Kernel and architecture.
- SFTP availability.
- systemd availability.
- `journalctl` availability and access.
- `/proc` availability.
- Common process-listing tools.
- Available shell and command environment.

Store or cache capability results with a timestamp and refresh them periodically or after an explicit request. A failed feature should not invalidate unrelated features.

### Compatibility policy

- Linux is the supported primary target.
- Prefer POSIX-compatible tools where realistic.
- Use `/proc` when available, with documented fallback behavior.
- Detect systemd rather than assuming it.
- Avoid relying on `sudo` being installed or configured.
- Do not assume GNU-specific command options if a portable interface is available.
- Provide a compatibility matrix for tested distributions and versions.
- Support additional Unix-like systems only after dedicated tests.

---

# 6. Security, privacy and threat model

Security is a product feature and a release requirement. Drovik becomes a gateway to systems that may contain sensitive files, credentials and production workloads. A flaw in the web application can become a route to remote server compromise.

## 6.1 Assets to protect

- SSH passwords.
- SSH private keys and passphrases.
- SSH host keys and trust decisions.
- Drovik administrator credentials.
- Session tokens and cookies.
- Server addresses and usernames.
- Remote file contents handled temporarily by the application.
- Terminal input/output while in memory.
- Audit metadata.
- Encryption keys and backups.
- Remote system availability and integrity.

## 6.2 Threat actors and risks

- An unauthenticated internet attacker probing the web application.
- An authenticated low-privilege user attempting to access another user's server.
- A malicious website attempting CSRF or WebSocket abuse against an authenticated browser.
- A compromised browser or workstation.
- A malicious or compromised remote SSH server.
- A man-in-the-middle attacker attempting SSH host-key substitution.
- A malicious file or log containing HTML, script or terminal control sequences.
- A user entering a malicious hostname or address to reach internal services through the Drovik backend.
- A compromised application host exposing stored credentials.
- A dependency or build pipeline compromise.

## 6.3 Mandatory controls

### Network and transport

- Production deployments MUST use HTTPS.
- The backend MUST bind to a private interface or localhost behind a reverse proxy unless direct exposure is explicitly configured.
- The deployment guide MUST explain firewall rules and reverse-proxy TLS.
- SSH host keys MUST be verified.
- SSH algorithms MUST use supported modern defaults; obsolete algorithms should be disabled unless an explicit compatibility exception is configured.
- Network destinations MUST be validated against configurable egress rules.
- DNS resolution and connection establishment must be designed to reduce DNS rebinding and time-of-check/time-of-use risks.
- Cloud metadata endpoints and loopback/local administrative endpoints should be blocked by default unless explicitly authorized by the operator for a legitimate managed-host use case.

### Application security

- Use secure password hashing.
- Use authenticated encryption for stored SSH credentials.
- Use secure, revocable sessions.
- Implement CSRF protection for cookie-authenticated mutations.
- Validate WebSocket origin and authentication.
- Enforce backend authorization for every server resource.
- Validate all input and bound all output.
- Rate-limit login and sensitive operations.
- Use secure HTTP headers, including a restrictive Content Security Policy where practical.
- Prevent XSS by treating logs, filenames, hostnames and remote command output as untrusted text.
- Prevent SQL injection through parameterized queries or a safe query layer.
- Prevent path traversal and symlink escapes in file operations.
- Prevent command injection by using structured operations and fixed command templates.
- Avoid leaking internal error traces to clients.

### Remote operation safety

- No automatic sudo.
- No privilege escalation hidden from the user.
- Destructive operations require confirmation.
- Operations use bounded timeouts and concurrency.
- The backend records safe audit metadata for important actions.
- The system must distinguish “request sent” from “operation verified”.

### Secrets and logging

- No passwords, passphrases, private keys, session tokens or encryption keys in logs.
- Redact secret-like values from errors where practical.
- Do not send secrets to analytics, telemetry or crash-reporting systems.
- Do not write SSH private keys to temporary files unless unavoidable and protected; prefer in-memory processing.
- If temporary files are necessary, use restrictive permissions, predictable cleanup and secure storage paths.

## 6.4 SSRF and network reachability controls

Because Drovik intentionally connects to user-specified servers, it is a network-capable backend. This creates an important boundary: the operator must be able to manage legitimate private-network hosts, but an attacker must not be able to use Drovik as an unrestricted scanner or proxy.

Requirements:

1. Only authenticated and authorized users may create or test server connections.
2. The operator MUST be able to define allowed destination ranges or network rules.
3. A deployment with multiple users SHOULD default to an administrator-managed host allowlist or approval flow.
4. Block cloud metadata IP ranges and local control endpoints by default.
5. Treat loopback, link-local, multicast, unspecified and reserved IP ranges specially.
6. Resolve DNS once for validation and ensure the connection uses a validated resolved address, while preserving correct SSH host-key identity handling.
7. Revalidate after redirects are not relevant to SSH, but revalidate every new connection attempt.
8. Apply outbound connection rate limits and maximum concurrent destination count.
9. Log connection attempts as metadata without logging credentials.
10. Do not expose a generic backend “connect to any host and return data” endpoint.

## 6.5 File and terminal content safety

- Remote filenames and logs are untrusted data.
- Render text through safe text nodes; do not inject raw HTML.
- Terminal escape sequences should be handled by the terminal emulator, not interpreted by the surrounding web page.
- File preview must apply size limits and binary detection.
- Downloads should use safe content-disposition headers and non-executable preview behavior where possible.
- The backend must not execute a file merely because it was uploaded or opened.
- Do not automatically render remote HTML or SVG as trusted application content.

## 6.6 Privacy defaults

- No mandatory cloud telemetry.
- No remote file-content retention by default.
- No terminal session recording by default.
- No log-content retention by default.
- No user tracking across unrelated installations.
- Any optional telemetry must be opt-in, documented, minimal and free of secrets or remote server content.
- Clearly document which metadata is stored locally.

## 6.7 Security testing requirements

Before public release, test at least:

- Authentication bypass attempts.
- Session fixation and invalidation.
- CSRF against state-changing endpoints.
- WebSocket origin bypass.
- Cross-user server access.
- Host-key mismatch behavior.
- Credential encryption/decryption and key rotation/recovery behavior.
- SQL injection and unsafe deserialization risks.
- Command injection in server, service, process and log parameters.
- Path traversal and symlink traversal.
- Large file and output denial-of-service cases.
- SSRF to loopback, private, link-local and metadata endpoints under configured policy.
- Resource exhaustion through many terminal sessions or metric refreshes.
- XSS through filenames, hostnames, process names and log messages.
- Secret leakage in logs, traces, error responses and database backups.

---

# 7. Performance, reliability and quality requirements

Targets below are initial engineering objectives and should be validated on a documented reference environment. They are not guarantees for slow networks or overloaded remote servers.

## 7.1 Performance objectives

- **NFR-PERF-001:** The application shell SHOULD become usable within 3 seconds on a normal desktop browser and local/self-hosted network, excluding first-time large asset downloads.
- **NFR-PERF-002:** Normal terminal keystrokes SHOULD feel interactive, with no avoidable buffering in the frontend/backend path.
- **NFR-PERF-003:** A typical metric refresh SHOULD complete within 2–5 seconds on a healthy host and network.
- **NFR-PERF-004:** Metric polling MUST be configurable and must not create overlapping refreshes for the same host by default.
- **NFR-PERF-005:** Directory browsing SHOULD load common folders quickly; large directories must use pagination or bounded listing behavior where SFTP/server support allows.
- **NFR-PERF-006:** File transfers MUST stream rather than loading the entire file into backend memory.
- **NFR-PERF-007:** The backend MUST enforce configurable limits on sessions, transfer concurrency, output size and request duration.

## 7.2 Reliability objectives

- **NFR-REL-001:** A failure on one managed server MUST not crash Drovik or interrupt operations for unrelated servers.
- **NFR-REL-002:** A remote disconnection MUST be represented explicitly.
- **NFR-REL-003:** Failed background jobs MUST release resources.
- **NFR-REL-004:** Database schema changes MUST use versioned migrations.
- **NFR-REL-005:** An upgrade MUST not silently discard connection records or user data.
- **NFR-REL-006:** The application MUST provide a documented backup and restore procedure.
- **NFR-REL-007:** Shutdown MUST stop accepting new work and close active connections gracefully within a bounded period.

## 7.3 Resource objectives

Initial target for a small installation:

- One Drovik instance manages approximately 5–20 saved servers for an individual operator, depending on concurrent activity and network conditions.
- The backend uses bounded concurrency and streams file content.
- Idle operation should remain lightweight; actual RAM/CPU requirements must be measured and documented during testing.
- Large file transfer capacity is bounded by disk/network and must not require permanent staging of the entire file.

Do not promise a fixed maximum number of servers until load testing has verified it.

## 7.4 Accessibility and usability

- Support keyboard navigation for core workflows.
- Provide visible focus states.
- Use meaningful labels and accessible names for controls.
- Maintain adequate contrast.
- Do not rely only on color to convey status.
- Support screen-reader-friendly labels for key forms, navigation and status messages.
- Ensure dialogs can be operated and dismissed accessibly.
- Show units and timestamps clearly.
- Explain dangerous operations in plain language.
- Make the remote host identity persistent and easy to notice.

Target WCAG 2.2 AA for core application flows where practical.

## 7.5 Browser support

Initial support should target current stable desktop versions of:

- Google Chrome / Chromium.
- Microsoft Edge.
- Mozilla Firefox.
- Safari on macOS, subject to terminal and WebSocket compatibility testing.

Mobile browsers may support basic navigation and monitoring, but complex terminal and file-management workflows can be optimized for desktop first. The application must remain usable at narrower widths and must not silently discard user actions on smaller displays.

## 7.6 Compatibility matrix

Maintain a tested matrix covering a representative set of distributions, for example:

- Ubuntu LTS.
- Debian stable.
- Fedora Server.
- Rocky Linux / AlmaLinux.
- Arch Linux (best effort).
- Alpine Linux (best effort; BusyBox differences).

For each, test OpenSSH authentication, SFTP, metrics, process listing, systemd service operations and journal access. A distribution is not “fully supported” merely because an SSH connection succeeds.

---

# 8. Deployment and operations

## 8.1 Deployment models

### Docker Compose (recommended initial path)

- One application container for the Go backend and built frontend, or a small number of clearly documented services.
- Persistent volume for SQLite and application data.
- Secret mounted separately from the database volume.
- Reverse proxy for HTTPS termination where appropriate.
- No privileged container mode.
- No Docker socket mount for the core product.
- No host network mode by default.
- Explicit outbound network access to intended SSH destinations.

### Native Linux deployment

Later or parallel support may provide:

- Linux binary.
- systemd unit for the Drovik application itself.
- Configuration file or environment variables.
- Dedicated unprivileged service user.
- Data directory with restrictive permissions.
- Reverse proxy configuration examples.

The Drovik service may run as a systemd service on the machine hosting Drovik; this does not violate the agentless promise because it is not installed on the remote servers being managed.

## 8.2 Configuration

All runtime configuration must be documented. Suggested categories:

- Listen address and port.
- Public/base URL and trusted origins.
- Database path.
- Data directory.
- Encryption key or secret-file path.
- Session lifetime.
- Login rate-limit settings.
- SSH dial/handshake/operation timeouts.
- Maximum SSH sessions globally and per user.
- Maximum SFTP transfer concurrency.
- Maximum file-editor size.
- Metric refresh limits.
- Egress/allowed-host policy.
- Logging level and format.
- Optional telemetry setting (off by default).

Do not place actual secrets in example files committed to the repository.

## 8.3 Reverse proxy and HTTPS

Documentation must include an example for a mainstream reverse proxy such as Nginx or Caddy, covering:

- TLS certificates.
- HTTP-to-HTTPS redirect.
- WebSocket upgrade headers.
- Request/body size configuration appropriate for file transfers.
- Proxy timeouts for long-lived terminal sessions.
- Forwarded headers and trusted proxy configuration.
- Security headers.
- Restricting direct access to the backend port.

## 8.4 Health checks

Provide separate liveness and readiness checks where appropriate:

- Liveness: process is running and event loop/server can respond.
- Readiness: configuration is valid and database is accessible.
- Remote SSH availability MUST NOT be part of the overall application readiness check because managed hosts may legitimately be offline.

Health endpoints must not disclose secrets, host lists, filesystem paths or detailed internal configuration to unauthenticated clients.

## 8.5 Logging and observability

- Use structured logs.
- Include request/correlation IDs.
- Redact credentials and tokens.
- Avoid logging terminal content or remote file content.
- Separate application errors from remote-operation errors.
- Provide log levels that are safe by default.
- Make debug logging opt-in and warn that it may contain sensitive operational metadata.
- Do not introduce an external telemetry dependency in the default deployment.

## 8.6 Backup and restore

Document:

- Which files contain the database.
- Which separate secret/key material is required to decrypt saved credentials.
- How to stop or quiesce writes before a consistent backup if needed.
- How to encrypt and store backups.
- How to restore on a new host.
- How to validate that restored credentials can be decrypted.
- How to rotate the encryption key in a future supported procedure.

A backup that contains the database but not the encryption key may preserve non-secret metadata while losing access to stored SSH credentials. The documentation must make this explicit.

## 8.7 Upgrade and migration

- Every database change MUST have a migration.
- Migrations MUST be tested against a copy of the previous supported schema.
- The release process MUST recommend a backup before upgrading.
- Startup MUST not proceed with an unknown or incompatible schema state.
- Rollback behavior must be documented. Not every migration can be safely reversed; in those cases, restore from backup may be the rollback path.
- Container images should use explicit version tags; avoid relying solely on `latest` in production.

## 8.8 Updates and supply chain

- Pin or lock frontend and backend dependencies.
- Run dependency vulnerability scanning.
- Produce reproducible or verifiable build artifacts where practical.
- Sign release artifacts when the project has a stable release process.
- Publish checksums.
- Protect the default branch and release process.
- Do not include private test credentials or production server details in example configurations.

---

# 9. Testing and quality strategy

## 9.1 Test layers

### Unit tests

Cover:

- Input validation.
- Password hashing and session handling.
- Secret encryption/decryption.
- Host-key fingerprint calculation and trust decisions.
- Capability parsing.
- CPU/memory/disk/network calculations.
- Safe process PID and signal validation.
- Service action validation.
- Path normalization and file-operation policy.
- Audit event redaction.

### Integration tests

Use disposable test SSH servers or containers configured with OpenSSH/SFTP to test:

- Password authentication.
- Key authentication.
- Wrong credentials.
- Host-key first trust and host-key change.
- SFTP listing, upload, download, rename and delete.
- Terminal PTY behavior and resize.
- Command timeout and disconnect.
- systemd capability handling where a test environment can provide it.
- Process-list parsing.
- Journal/log retrieval where supported.

### API tests

Test:

- Authentication required.
- Authorization per server.
- Role enforcement.
- Invalid input and oversized requests.
- Rate limits.
- Session expiry.
- CSRF enforcement.
- Error response consistency.

### Frontend tests

Test:

- First-run and login flows.
- Server creation and connection states.
- Host-key approval dialog.
- Workspace server context.
- Terminal lifecycle.
- File operations and unsaved editor warnings.
- Metrics loading/stale/unavailable states.
- Confirmation flows.
- Accessibility basics.

### End-to-end tests

At least one complete test should cover:

1. Fresh Drovik setup.
2. Login.
3. Add a test server.
4. Approve host key.
5. Open terminal and run a safe command.
6. Browse a test directory over SFTP.
7. Upload, download and remove a test file.
8. Inspect system metrics.
9. Inspect processes/services/logs where supported.
10. Logout and verify session invalidation.

## 9.2 Test fixtures and safety

- Never use a real production server for destructive automated tests.
- Create disposable test directories and service units.
- Use test-only SSH keys and passwords.
- Clean up test resources.
- Keep fixtures free of real secrets.
- Run security tests against a local test deployment.

## 9.3 Definition of done

A feature is complete only when:

- Requirements and acceptance criteria are met.
- Backend authorization is enforced.
- Inputs and outputs are bounded and validated.
- Failure states are handled.
- Resource cleanup is implemented.
- Unit/integration/frontend tests are added as appropriate.
- Logs do not expose secrets.
- Documentation is updated.
- No known critical/high security issue is introduced.
- The feature works on at least one documented reference Linux host and does not break other supported hosts.

---

# 10. Phased implementation roadmap

The following roadmap is intentionally sequential. Each phase has a goal, scope, implementation order, acceptance criteria and exit gate. The phase boundaries may be adjusted, but security foundations must not be postponed until after the feature set is complete.

## Phase 0 — Product discovery, architecture and project foundation

**Goal:** Remove major implementation uncertainties and establish a maintainable repository before building user-facing features.

### Scope

1. Confirm product name remains provisional pending clearance.
2. Finalize MVP scope and explicitly defer non-MVP modules.
3. Write architecture decision records for frontend, backend, database, SSH, SFTP and authentication.
4. Define threat model and security boundaries.
5. Define the initial supported Linux compatibility matrix.
6. Define environment configuration and secret handling.
7. Create repository structure and contribution guidelines.
8. Define coding standards, linting and test approach.
9. Decide versioning, migrations and API versioning.
10. Define licensing and third-party dependency policy.

### Deliverables

- Repository initialized.
- `README.md` with product overview and development setup.
- `AGENTS.md` or equivalent contributor/AI coding-agent instructions.
- `CONTRIBUTING.md`.
- `SECURITY.md` with vulnerability reporting guidance.
- `LICENSE` containing the selected open-source license after final legal review.
- Architecture decision records under `docs/adr/`.
- Threat model under `docs/security/`.
- Product requirements document (this file).
- Initial issue backlog mapped to requirement IDs.

### Exit criteria

- The team can explain the browser/backend/SSH trust boundary.
- There is agreement on what the MVP includes and excludes.
- Secrets handling and host-key verification have a reviewed design.
- A clean clone can be built using documented instructions.

## Phase 1 — Application skeleton, deployment and authentication

**Goal:** Produce a deployable application with a secure foundation and no remote server functionality yet.

### Backend work

1. Create Go application entry point and configuration loader.
2. Add structured logging and secret redaction.
3. Add SQLite connection and migration framework.
4. Create initial schema for users, sessions, settings and audit events.
5. Implement first-run setup.
6. Implement password hashing.
7. Implement login, logout and current-user endpoint.
8. Implement session cookies and expiry.
9. Implement authorization middleware.
10. Implement CSRF protection.
11. Add health and readiness endpoints.
12. Add graceful shutdown.

### Frontend work

1. Create React + TypeScript + Vite application.
2. Configure strict TypeScript and linting.
3. Create routing and application shell.
4. Implement first-run setup page.
5. Implement login page.
6. Implement session restoration and logout.
7. Add global loading, error and notification handling.
8. Add a typed API client.
9. Add a basic settings/about view.

### Deployment work

1. Create Dockerfile using a multi-stage build.
2. Create Docker Compose configuration.
3. Persist SQLite and application data.
4. Mount encryption secret separately, even before SSH credential storage is implemented.
5. Run the application as a non-root container user.
6. Add reverse-proxy example and HTTPS documentation.
7. Add environment example without real secrets.

### Tests

- First-run can only complete once.
- Passwords are hashed.
- Session expires and logout invalidates it.
- CSRF protection blocks invalid state-changing requests.
- Readiness fails when the database is unavailable.
- Health endpoints do not leak sensitive information.

### Exit criteria

A new operator can deploy Drovik, complete first-run setup, log in, log out, restart the service and log in again without data loss.

## Phase 2 — SSH connection manager and trust establishment

**Goal:** Connect to Linux servers safely and persist connection definitions.

### Backend work

1. Implement encrypted secret storage.
2. Implement key loading and password authentication.
3. Implement host/port validation.
4. Implement DNS resolution and egress policy checks.
5. Implement SSH dialing with timeouts.
6. Implement strict host-key checking.
7. Implement unknown-host-key presentation flow.
8. Implement changed-host-key blocking and re-approval flow.
9. Implement connection test endpoint.
10. Implement saved server CRUD endpoints.
11. Implement credential replacement and deletion.
12. Implement bounded session management.
13. Implement capability discovery for OS, SFTP and common tools.
14. Record safe audit events.

### Frontend work

1. Create server list.
2. Create add/edit server form.
3. Add password and private-key authentication choices.
4. Add passphrase handling without browser persistence.
5. Implement connection test flow.
6. Implement host-key fingerprint approval dialog.
7. Implement changed-host-key warning flow.
8. Display connection state and capability summary.
9. Add server search/filter and basic tags if time permits.

### Tests

- Password authentication.
- Key authentication.
- Encrypted private-key passphrase handling.
- Wrong password/key.
- Unknown host key.
- Changed host key.
- DNS and egress policy behavior.
- Timeout and disconnect cleanup.
- Credential encryption and deletion.

### Exit criteria

A user can securely add and test at least two Linux SSH servers, trust their host keys explicitly, reconnect after an application restart and remove a connection without changing the remote host.

## Phase 3 — Workspace and interactive terminal

**Goal:** Deliver the first genuinely useful remote workspace and a reliable terminal.

### Backend work

1. Implement authenticated WebSocket upgrade.
2. Validate WebSocket Origin.
3. Implement terminal session creation.
4. Request PTY with terminal type and dimensions.
5. Stream stdin/stdout safely.
6. Support resize messages.
7. Add ping/pong and dead-connection detection.
8. Add message size/rate limits.
9. Track terminal sessions by user and server.
10. Implement cleanup on close, logout, timeout and shutdown.
11. Add terminal session concurrency limits.

### Frontend work

1. Implement workspace shell.
2. Implement persistent server context indicator.
3. Integrate xterm.js.
4. Connect terminal to authenticated WebSocket.
5. Support keyboard input and output.
6. Support terminal resize.
7. Add copy/paste behavior and multi-line paste warning.
8. Add terminal tabs or multiple terminal windows if stable.
9. Add explicit close/reconnect states.
10. Ensure switching servers cannot mix terminal output.

### Tests

- Interactive shell.
- Terminal resize.
- Concurrent terminals.
- Network disconnect.
- Browser close cleanup.
- Logout invalidation.
- Unauthorized server terminal access.
- WebSocket Origin and message limits.

### Exit criteria

A user can connect to a remote Linux host and use a normal interactive shell through the browser. Session cleanup and authorization tests pass.

## Phase 4 — SFTP file manager and text editor

**Goal:** Provide secure, practical remote file operations.

### Backend work

1. Implement SFTP client creation and cleanup.
2. Implement bounded directory listing.
3. Normalize and validate remote paths.
4. Implement metadata retrieval.
5. Implement upload and download streaming.
6. Implement transfer progress and cancellation.
7. Implement directory creation.
8. Implement file creation.
9. Implement rename/move.
10. Implement delete with safe recursive policy.
11. Implement symlink inspection and handling.
12. Implement file-size limits for preview/editor operations.
13. Implement stale-file detection where practical.
14. Record file mutation metadata without file contents.

### Frontend work

1. Build directory navigation and breadcrumb path.
2. Display files/folders and metadata.
3. Add hidden-file toggle.
4. Add sorting and current-directory filtering.
5. Add upload and download controls.
6. Add transfer progress and cancellation.
7. Add create/rename/delete operations.
8. Add overwrite confirmation.
9. Add text editor with unsaved-change warning.
10. Add file-size and binary-file warnings.
11. Add explicit Save and Cancel behavior.
12. Add permission and transfer error states.

### Tests

- Directory listing and navigation.
- Dotfiles.
- Upload/download streaming.
- Cancelled transfer.
- Permission denied.
- Existing file collision.
- Path traversal and symlink tests.
- Recursive delete confirmation.
- Unsaved editor changes.
- Remote stale-file conflict.

### Exit criteria

A user can browse a remote server, transfer ordinary files, create/rename/delete permitted items and edit a small text configuration file safely over SFTP.

## Phase 5 — System overview and monitoring

**Goal:** Make Drovik useful as a server overview without requiring a monitoring agent.

### Backend work

1. Implement bounded system-information collection.
2. Implement operating-system and kernel detection.
3. Implement uptime/load collection.
4. Implement CPU counter parsing and percentage calculation.
5. Implement memory parsing.
6. Implement filesystem usage collection.
7. Implement network counter collection and rate calculation.
8. Implement polling scheduler or request coalescing.
9. Implement per-host metric cache with freshness timestamp.
10. Handle counter reset, reboot and unavailable values.
11. Add command timeouts and output limits.

### Frontend work

1. Create server overview page.
2. Show host identity and OS details.
3. Show CPU and load.
4. Show memory.
5. Show disk/filesystem usage.
6. Show network interfaces and traffic rates.
7. Show uptime.
8. Add refresh and last-updated indicator.
9. Add unavailable/stale states.
10. Add a compact multi-server overview if useful.

### Tests

- Metric parser fixtures from different Linux distributions.
- CPU first-sample and subsequent-sample logic.
- Counter reset/reboot handling.
- Missing `/proc` or commands.
- Stale metrics.
- Polling concurrency and cancellation.
- Large number of mounted filesystems/interfaces.

### Exit criteria

A user can open a server overview and receive honest, timestamped CPU, memory, disk, uptime and available network metrics without installing an agent.

## Phase 6 — Process manager, service manager and logs

**Goal:** Cover common operational actions while respecting remote permissions.

### Backend work

1. Implement process discovery/parsing.
2. Implement structured process signal operation.
3. Re-check process identity before signaling where practical.
4. Implement systemd detection.
5. Implement systemd unit discovery and state retrieval.
6. Implement enumerated service actions.
7. Re-read service status after actions.
8. Implement bounded journal retrieval.
9. Implement unit-specific log retrieval.
10. Implement safe filtering and output limits.
11. Add comprehensive permission/error mapping.
12. Add audit events for process/service actions.

### Frontend work

1. Process list with PID, owner, CPU, memory and state.
2. Process search/sort/refresh.
3. Process termination confirmation.
4. Service list with state indicators.
5. Service details and action controls.
6. Confirmation for stop/restart/enable/disable.
7. Log viewer for system and selected service.
8. Time/line filters and text search.
9. Clear unsupported/permission states.
10. Result status based on re-read state.

### Tests

- Process-list parsing.
- Invalid PID and PID reuse edge cases.
- SIGTERM and permission denied.
- systemd unavailable.
- Service action success and failure.
- Journal unavailable or unauthorized.
- Log output XSS handling.
- Output limits and timeout behavior.

### Exit criteria

A user can inspect processes, terminate a permitted process, inspect systemd services and perform allowed actions, and view bounded journal logs. The product must not claim success when the remote operation was denied or failed.

## Phase 7 — MVP integration, security hardening and public beta

**Goal:** Turn the modules into one dependable product and prepare for real users.

### Integration work

1. Review all server-context handling across windows and panels.
2. Verify no stale data crosses server boundaries.
3. Verify connection state is consistent across modules.
4. Standardize operation result states and error codes.
5. Complete audit history for sensitive operations.
6. Add empty states and first-use guidance.
7. Review keyboard navigation and accessibility.
8. Verify desktop and narrow viewport behavior.
9. Complete supported distribution matrix.
10. Finalize documentation and troubleshooting guides.

### Security work

1. Run threat-model review.
2. Run dependency and static analysis scans.
3. Run authentication/authorization tests.
4. Run WebSocket security tests.
5. Run command-injection tests.
6. Run SSRF/egress-policy tests.
7. Run SFTP path and symlink tests.
8. Test secret redaction.
9. Test backup and restore.
10. Review container permissions and network exposure.
11. Conduct an independent security review before a stable public release where feasible.

### Release work

1. Tag a beta release.
2. Publish Docker image with version tag.
3. Publish checksums.
4. Publish upgrade notes.
5. Publish known limitations.
6. Collect bug reports through a public issue template.
7. Track crash/error reports without default external telemetry.
8. Define support policy for the beta.

### Exit criteria

All MVP requirements are met or explicitly deferred with documented reasons. No known critical or high-severity security issue remains open. A new operator can install, secure, use, back up and upgrade Drovik using only the documentation.

## Phase 8 — Stable release and operational maturity

**Goal:** Make the MVP safe and maintainable for regular self-hosted use.

### Scope

- Stable versioning and release cadence.
- Formal compatibility matrix.
- Database migration and rollback guidance.
- Automated release pipeline.
- Signed release artifacts where feasible.
- Security policy and responsible disclosure process.
- Contributor guidelines and code ownership.
- Performance/load tests.
- Documentation website or structured docs.
- Translation framework if community demand exists.
- Stable upgrade and backup/restore procedures.

### Exit criteria

The team can produce a release from a tagged commit, verify its artifacts, migrate supported installations safely and respond to security reports using a published process.

## Phase 9 — Advanced server management (post-MVP)

> **Implementation status:** Docker inspection/enumerated actions, read-only storage/network
> diagnostics, package lifecycle, managed scheduled tasks, snippets, local users/server assignments,
> bounded metric history and in-app alerts are implemented. OIDC/MFA, notifications and destructive
> storage/network changes are being delivered as separate increments; non-Linux targets remain out
> of scope pending a new support matrix.

Features in this phase should be individually designed and threat-modeled.

### 9.1 Docker/container management

- Detect Docker Engine or compatible runtime.
- List containers and images.
- Display container status, ports and resource usage where available.
- Start, stop, restart and remove containers with confirmation.
- View container logs.
- Inspect container details.
- Support compose projects only after a secure design is defined.
- Never mount the Docker socket into Drovik's own container merely to simplify remote management; remote access should use a deliberate, documented mechanism.
- Clearly warn that access to a Docker daemon can effectively grant root-equivalent control on many hosts.

### 9.2 Storage tools

- View block devices, mount points and filesystem information.
- Display disk usage and inode usage.
- Show mount status.
- Provide safe read-only inspection first.
- Any mount/unmount/format/partition operation requires an independent risk review and explicit confirmation; destructive disk operations should not be added casually.

### 9.3 Network tools

- View interfaces, addresses, routes and listening ports.
- Provide read-only diagnostics first.
- Firewall and network configuration changes require safeguards against disconnecting the active SSH session.
- Never automatically apply a network configuration change without a rollback plan or explicit user acknowledgment.

### 9.4 Package management

- Detect package manager.
- Show installed package metadata.
- Read-only package search and update availability first.
- Install/remove/update operations require confirmation and clear privilege messaging.
- Avoid automated unattended system upgrades in the initial design.

### 9.5 Saved commands and snippets

- User-defined command snippets with names, descriptions and optional variables.
- Preview fully substituted command before execution.
- Explicit target server confirmation.
- No hidden privilege escalation.
- Store snippets without secrets by default.
- Provide import/export only with safe handling.

### 9.6 Scheduled tasks

- View cron jobs and systemd timers.
- Edit tasks only after backup, validation and confirmation safeguards.
- Make the execution user and schedule visible.
- Avoid silently creating persistent scheduled jobs.

### 9.7 Multi-user and team features

- User management.
- Server assignment and granular authorization.
- Team/group concepts.
- Read-only role.
- OIDC/SSO.
- MFA.
- Audit search and export.
- Optional external secrets integration.

### 9.8 Historical metrics and alerts

- Optional metrics retention in a local database.
- Per-server charts over time.
- Threshold configuration.
- Notifications through configurable channels.
- Retention and storage controls.
- Clear warning that this is not a substitute for enterprise monitoring without appropriate reliability guarantees.

### 9.9 Additional operating systems

- macOS over SSH.
- BSD variants.
- Appliance/homelab distributions.

Each target requires a tested capability matrix and explicit supported-feature list.

---

# 11. Release gates and MVP acceptance checklist

The MVP should not be declared complete until the following checklist is satisfied.

## Product and setup

- [ ] Product scope and limitations are documented.
- [ ] First-run setup works on a clean installation.
- [ ] Initial administrator account is secure.
- [ ] Application restarts without losing state.
- [ ] Deployment guide is complete.

## Authentication and security

- [ ] Passwords are securely hashed.
- [ ] Sessions expire and logout invalidates them.
- [ ] Cookie, CSRF and WebSocket protections are implemented.
- [ ] Credentials are encrypted at rest.
- [ ] Encryption key is stored separately from database ciphertext.
- [ ] Secrets are redacted from logs and API errors.
- [ ] Authorization is enforced in backend handlers.
- [ ] Egress policy protects against unintended backend access.

## SSH

- [ ] Multiple servers can be saved.
- [ ] Password authentication works.
- [ ] Private-key authentication works.
- [ ] Host keys are verified.
- [ ] Unknown and changed host keys have safe flows.
- [ ] Connection timeouts and cleanup work.
- [ ] Saved connection deletion handles credentials correctly.

## Terminal

- [ ] Interactive PTY works.
- [ ] Input and output stream correctly.
- [ ] Resize works.
- [ ] WebSocket authorization and Origin validation work.
- [ ] Sessions close on logout and timeout.
- [ ] Terminal content is not stored by default.

## Files

- [ ] SFTP directory listing works.
- [ ] Hidden files can be shown.
- [ ] Upload and download work.
- [ ] Transfer progress and cancellation work.
- [ ] Create, rename and delete work within permissions.
- [ ] Recursive delete is guarded.
- [ ] Editor enforces size and binary limits.
- [ ] Unsaved changes are protected.
- [ ] Path traversal and symlink tests pass.

## Monitoring and management

- [ ] Host information is displayed.
- [ ] CPU and load are displayed honestly.
- [ ] Memory values are calculated correctly.
- [ ] Filesystem usage is displayed.
- [ ] Network counters/rates are displayed where available.
- [ ] Stale and unavailable metrics are clear.
- [ ] Process list works.
- [ ] Process termination is validated and confirmed.
- [ ] systemd service list and actions work where supported.
- [ ] Journal logs work where permitted.
- [ ] Unsupported features are reported clearly.

## Quality and operations

- [ ] Unit tests pass.
- [ ] Integration tests pass.
- [ ] API authorization tests pass.
- [ ] End-to-end core workflow passes.
- [ ] No critical/high security issue is knowingly outstanding.
- [ ] Backup and restore are tested.
- [ ] Migration path is tested.
- [ ] Health checks and graceful shutdown work.
- [ ] Supported Linux compatibility matrix is published.
- [ ] User documentation and troubleshooting guides are published.

---

# 12. Suggested repository layout

A monorepo is recommended for the initial product so frontend, backend, deployment and docs can evolve together.

```text
drovik/
├── README.md
├── LICENSE
├── CONTRIBUTING.md
├── SECURITY.md
├── CODE_OF_CONDUCT.md
├── AGENTS.md
├── .gitignore
├── .editorconfig
├── .github/
│   ├── ISSUE_TEMPLATE/
│   ├── PULL_REQUEST_TEMPLATE.md
│   └── workflows/
│       ├── ci.yml
│       ├── security.yml
│       └── release.yml
├── frontend/
│   ├── package.json
│   ├── tsconfig.json
│   ├── vite.config.ts
│   └── src/
├── backend/
│   ├── go.mod
│   ├── cmd/
│   ├── internal/
│   ├── migrations/
│   └── tests/
├── deploy/
│   ├── docker/
│   ├── compose/
│   ├── nginx/
│   └── systemd/
├── docs/
│   ├── product/
│   ├── architecture/
│   ├── adr/
│   ├── security/
│   ├── deployment/
│   ├── user-guide/
│   ├── development/
│   └── compatibility/
├── scripts/
└── testdata/
```

Names may change, but keep production code, deployment assets, documentation and test fixtures clearly separated.

---

# 13. Engineering workflow and contribution rules

## 13.1 Development workflow

1. Every feature starts with a requirement ID or a documented product decision.
2. Break work into backend, frontend, database, security, tests and documentation tasks as relevant.
3. Define acceptance criteria before implementation.
4. Implement backend validation and authorization before relying on frontend controls.
5. Add automated tests alongside the implementation.
6. Run formatters, linters and test suites before opening a pull request.
7. Review dependency changes and security implications.
8. Update documentation and changelog.
9. Merge only after CI passes and required review is complete.

## 13.2 Coding rules

- Keep HTTP handlers small.
- Keep SSH and SFTP code behind service interfaces.
- Avoid global mutable connection state.
- Use context cancellation and timeouts throughout backend operations.
- Use typed errors and stable error codes.
- Never log secrets.
- Avoid shell command construction from untrusted input.
- Use database transactions for related persistence changes.
- Add migrations for schema changes.
- Keep frontend components focused and avoid duplicate API logic.
- Use strict TypeScript types; avoid `any` unless justified and contained.
- Keep dependencies minimal and maintained.
- Document any security exception.

## 13.3 Pull request checklist

- [ ] Requirement or issue is linked.
- [ ] Scope is clear and limited.
- [ ] Backend authorization is implemented where needed.
- [ ] Input validation and bounds are implemented.
- [ ] Failure and cancellation paths are handled.
- [ ] Resources are cleaned up.
- [ ] Secrets are not logged or returned.
- [ ] Tests are added or updated.
- [ ] Documentation is updated.
- [ ] No unrelated formatting or refactoring obscures the change.

## 13.4 AI coding-agent instructions

If AI coding agents are used:

- They MUST read `AGENTS.md` before modifying code.
- They MUST inspect existing code before proposing a rewrite.
- They MUST implement one bounded task at a time.
- They MUST not invent successful test results.
- They MUST run relevant tests and report exactly which tests ran and which did not.
- They MUST not introduce secrets, real hostnames, credentials or private keys into fixtures.
- They MUST not bypass host-key verification or weaken security controls to make a demo work.
- They MUST update requirement traceability and documentation for changes.
- Each completed task SHOULD leave a handoff note containing changed files, design decisions, tests run, known issues and the next logical task.

---

# 14. Documentation that must ship with the product

## User documentation

- What Drovik is and is not.
- Installation using Docker Compose.
- Installation using native Linux deployment, when supported.
- First-run setup.
- Adding an SSH server.
- Understanding SSH host-key fingerprints.
- Password and private-key authentication.
- Using the terminal.
- Using the file manager.
- Uploading/downloading and editing files.
- Understanding monitoring metrics.
- Managing processes.
- Managing systemd services.
- Viewing logs.
- Understanding permissions and why an action may be denied.
- Removing a server connection.
- Changing credentials.
- Backing up and restoring Drovik.
- Upgrading Drovik.
- Troubleshooting connection and SFTP problems.

## Administrator documentation

- Configuration reference.
- Encryption key handling and recovery limitations.
- HTTPS/reverse-proxy setup.
- Firewall and network requirements.
- Egress policy configuration.
- Data directory and database maintenance.
- Backup/restore procedure.
- Session and resource limits.
- Log levels and redaction.
- Security hardening checklist.

## Developer documentation

- Architecture overview.
- Repository layout.
- Local development setup.
- API reference/OpenAPI specification.
- Database schema and migrations.
- SSH connection lifecycle.
- WebSocket protocol.
- SFTP operation behavior.
- Capability detection rules.
- Test environment setup.
- Release and migration process.
- Contribution guidelines.

---

# 15. Open decisions to resolve before or during implementation

These are the decisions that should be recorded in architecture decision records rather than left implicit.

| Decision | Recommended starting point | When to finalize |
|---|---|---|
| Product name | Drovik as a working name only | Before public branding/domain purchase |
| License | AGPL-3.0 | Before first public source release; review legal implications |
| Initial user model | One trusted local administrator; data model ready for multiple users | Phase 1 |
| Database | SQLite | Phase 1 |
| Database encryption | Encrypt SSH secrets at application layer; protect key outside DB | Before Phase 2 credentials are stored |
| Password hashing | Argon2id with reviewed parameters | Phase 1 |
| Session storage | Server-side opaque sessions; hash tokens in DB where practical | Phase 1 |
| SSH authentication | Password + private key first | Phase 2 |
| SSH agent integration | Defer unless a strong use case exists | Post-MVP |
| Terminal persistence | No session recording by default | Phase 3 |
| File editor max size | Set a conservative configurable initial limit and validate memory use | Phase 4 |
| Metric refresh interval | Start around 5 seconds with a configurable minimum and per-host coalescing | Phase 5 |
| Supported distributions | Ubuntu and Debian first, then broaden based on tests | Phases 0–8 |
| Process termination | SIGTERM first; SIGKILL only with stronger confirmation | Phase 6 |
| Service manager | systemd first; explicit unsupported state otherwise | Phase 6 |
| Logs | journalctl first; file-based logs later | Phase 6 / Phase 9 |
| Multi-user/RBAC | Design boundaries early; deliver after core single-operator MVP if needed | Phase 9 or earlier if deployment requires it |
| Docker management | Defer until core product is stable | Phase 9 |
| Historical metrics | Defer; avoid unnecessary time-series storage in MVP | Phase 9 |
| Telemetry | Off by default; opt-in only | Before beta |

---

# 16. Risks and mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| SSH credential compromise | Attacker may access managed servers | Strong app authentication, encryption at rest, separate key management, least privilege, HTTPS and security review |
| Host-key verification bypass | Man-in-the-middle attack | Strict host-key checking and explicit trust/re-trust flow |
| Arbitrary command injection | Remote command execution as connected user | Structured operation APIs, fixed command templates, validated parameters and security tests |
| SSRF/backend network abuse | Access to unintended internal services | Egress policy, host allowlists where appropriate, DNS/IP validation, rate limits and authorization |
| WebSocket hijacking | Unauthorized terminal access | Session authentication, Origin checks, CSRF/session protections and per-server authorization |
| Large file exhaustion | Backend memory/disk pressure | Streaming transfers, size limits, cancellation and concurrency controls |
| Distribution differences | Feature failures on some Linux hosts | Capability detection, compatibility matrix and graceful unsupported states |
| Privilege misunderstanding | User expects Drovik to perform root-only tasks | Show remote identity, explain Linux permissions and never escalate silently |
| Scope creep | Delayed MVP and fragile implementation | Phase gates and explicit deferrals |
| SQLite scaling/locking | Concurrent write limitations | Keep DB access isolated; test expected workload; migrate only when real requirements justify it |
| Complex desktop metaphor | Usability and development cost | Start with simple workspace/panels; add window-management complexity only if it helps workflows |
| Secret/key loss | Saved credentials become unrecoverable | Clear warnings, documented protected backup, recovery testing and future key-rotation process |
| False success from asynchronous operations | User believes a service/file/process action succeeded when it did not | Verify resulting remote state and support an “outcome unknown” state |
| Third-party dependency vulnerability | Application compromise | Dependency scanning, minimal dependencies, pinned versions and timely updates |

---

# 17. Success metrics

Measure product quality without collecting sensitive operational content.

## Adoption and onboarding

- Time for a new operator to complete first-run setup.
- Percentage of first-run users who successfully add a server.
- Percentage of connection tests completed successfully.
- Number of supported SSH authentication methods used.

## Core usage

- Successful terminal session starts.
- Successful SFTP transfers.
- Successful file operations.
- Successful metric refreshes.
- Successful service/process actions.

## Reliability and quality

- SSH connection error rate by safe error category.
- Terminal disconnect/cleanup failures.
- Failed or cancelled transfer rate.
- Metric polling timeout rate.
- Backend resource usage under representative load.
- Open security issues by severity.
- Upgrade and migration success in test environments.

Do not collect hostnames, usernames, file paths, command content, credentials, terminal output or log content for product analytics by default.

---

# 18. Final product definition

Drovik is an open-source, self-hosted, agentless web workspace for managing Linux servers over SSH. It provides a unified environment for server connections, interactive terminals, SFTP file management, system monitoring, process inspection, systemd service controls and basic log viewing.

The first release must prioritize secure SSH trust, credential protection, authorization, transparent remote-operation results and reliable session cleanup over a large number of management modules. It should be useful without root access, honest about remote permissions and compatible with ordinary Linux systems using standard SSH/SFTP capabilities.

The guiding implementation order is:

1. Secure application foundation.
2. SSH connections and host-key trust.
3. Workspace and terminal.
4. SFTP file manager.
5. System overview and metrics.
6. Process, service and log management.
7. Integration, security hardening and public beta.
8. Stable release and operational maturity.
9. Advanced management features only after the core is dependable.

**The MVP is successful when a user can deploy Drovik, securely connect to several Linux servers, work in an interactive terminal, manage files through SFTP, understand the basic health of each host, perform permitted process/service operations, inspect logs and confidently manage the lifecycle of the application—without installing a Drovik agent on any managed server.**

---

## Appendix A — Initial MVP user stories

1. As an operator, I can create the first administrator account so that I can secure my installation.
2. As an operator, I can log in and out so that access to the workspace is protected.
3. As an operator, I can add multiple SSH servers so that I can manage all my machines from one place.
4. As an operator, I can verify a host-key fingerprint so that I can detect an unexpected host identity.
5. As an operator, I can authenticate with a password or SSH private key so that I can use my existing access method.
6. As an operator, I can open a terminal for a selected server so that I can run normal Linux commands.
7. As an operator, I can resize the terminal so that interactive terminal programs remain usable.
8. As an operator, I can browse remote directories so that I can locate server files.
9. As an operator, I can upload and download files so that I can transfer configuration and application artifacts.
10. As an operator, I can edit a small text file and explicitly save it so that I can modify configuration files from the workspace.
11. As an operator, I can inspect CPU, memory, storage, uptime and network information so that I can understand server health.
12. As an operator, I can list processes and terminate a permitted process so that I can handle basic process issues.
13. As an operator, I can inspect systemd services and restart a permitted service so that I can perform common operations.
14. As an operator, I can view recent logs so that I can investigate a problem.
15. As an operator, I can see whether a feature is unavailable or denied by the remote host so that I do not mistake a failure for success.
16. As an operator, I can remove a saved server connection so that obsolete connection data is deleted from Drovik.
17. As an operator, I can back up Drovik's database and encryption key safely so that I can recover my installation.

## Appendix B — Suggested error categories

Use stable machine-readable codes and user-readable messages. The following categories are a starting point:

- `AUTH_REQUIRED`
- `AUTH_INVALID_CREDENTIALS`
- `AUTH_SESSION_EXPIRED`
- `AUTH_FORBIDDEN`
- `VALIDATION_ERROR`
- `SERVER_NOT_FOUND`
- `SERVER_NOT_AUTHORIZED`
- `SSH_DNS_FAILURE`
- `SSH_CONNECT_TIMEOUT`
- `SSH_CONNECTION_REFUSED`
- `SSH_HOST_KEY_UNKNOWN`
- `SSH_HOST_KEY_CHANGED`
- `SSH_AUTH_FAILED`
- `SSH_CHANNEL_FAILURE`
- `SSH_OPERATION_TIMEOUT`
- `SSH_DISCONNECTED`
- `SFTP_UNAVAILABLE`
- `SFTP_PERMISSION_DENIED`
- `SFTP_PATH_INVALID`
- `SFTP_FILE_TOO_LARGE`
- `SFTP_TRANSFER_CANCELLED`
- `SFTP_TRANSFER_INCOMPLETE`
- `CAPABILITY_UNAVAILABLE`
- `PROCESS_NOT_FOUND`
- `PROCESS_PERMISSION_DENIED`
- `SERVICE_NOT_FOUND`
- `SERVICE_PERMISSION_DENIED`
- `JOURNAL_UNAVAILABLE`
- `JOURNAL_PERMISSION_DENIED`
- `RATE_LIMITED`
- `RESOURCE_LIMIT_REACHED`
- `OUTCOME_UNKNOWN`
- `INTERNAL_ERROR`

Do not expose internal stack traces or secret values in the user-facing message associated with any error code.

## Appendix C — Initial technical validation checklist

Before committing to all implementation choices, build small proofs of concept for the following:

1. Go SSH connection with strict host-key checking and both password and private-key authentication.
2. Encrypted private key handling without writing the key to a world-readable temporary file.
3. SFTP directory listing, upload and download with streaming and cancellation.
4. xterm.js connected to an SSH PTY through an authenticated WebSocket.
5. PTY resize propagation and interactive programs.
6. CPU usage calculation using `/proc/stat` counter deltas.
7. Memory calculation using `/proc/meminfo`.
8. Disk usage parsing across representative distributions.
9. Process listing and safe signal behavior.
10. systemd service discovery and action behavior under a non-root account.
11. journal access under a non-root account.
12. Docker Compose deployment with persistent database and separately mounted encryption secret.
13. Backup and restore of SQLite plus secret material.
14. Egress policy behavior for public, private, loopback, link-local and metadata destinations.

Any proof of concept that bypasses host-key verification, disables security checks, runs as root unnecessarily or stores credentials in plaintext must be rejected rather than used as the foundation for production code.
