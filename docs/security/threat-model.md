# Drovik Threat Model

**Status:** living document — reviewed at each phase gate
**Last updated:** 2026-11-07
**Owners:** maintainer

## 1. Purpose and scope

This document describes the assets Drovik protects, the actors and trust boundaries involved, the
threats that matter, and the controls that mitigate them. It covers the web application, backend,
stored data and the SSH/SFTP boundary to managed hosts. It does not replace the target server's own
security model.

## 2. Assets

| Asset | Why it matters |
|---|---|
| SSH passwords and private keys | Direct access to managed servers |
| SSH host keys and trust decisions | MITM detection |
| Drovik administrator credentials | Control of the gateway |
| Session tokens and CSRF tokens | Authenticated access |
| Server addresses, usernames and metadata | Reconnaissance and privacy |
| Remote file contents handled in memory | Confidentiality |
| Terminal input/output in memory | Confidentiality; may contain secrets |
| Audit metadata | Accountability and investigation |
| Encryption key and backups | Recovery and credential confidentiality |
| Managed host availability and integrity | Operations impact |

## 3. Actors

- Unauthenticated internet attacker.
- Authenticated low-privilege user attempting cross-user access.
- Malicious website attempting CSRF or WebSocket abuse.
- Compromised browser or workstation.
- Malicious or compromised remote SSH server.
- Man-in-the-middle attacker.
- Malicious file, filename, log line or terminal escape sequence.
- User entering a malicious hostname to reach internal services (SSRF).
- Attacker with access to the Drovik host, database or backups.
- Compromised dependency or build pipeline.

## 4. Trust boundaries

```text
[ Browser ] --HTTPS/WSS--> [ Drovik backend ] --SSH/SFTP--> [ Managed Linux host ]
     ^                          |    |
     |                          |    +--> [ SQLite database ]
     |                          +-------> [ Encryption key / secret file ]
     +--- untrusted input, hostile content, stolen cookies
```

1. **Browser ↔ backend:** all browser input is untrusted. Sessions and CSRF are enforced here.
2. **Backend ↔ database:** the database is trusted storage but may be stolen; secrets are
   ciphertext.
3. **Backend ↔ encryption key:** key is outside the database; compromise of both is required to
   recover credentials.
4. **Backend ↔ managed host:** the remote host is untrusted and may be hostile or compromised.
5. **Backend ↔ internet:** egress can reach unintended internal services; policy applies.
6. **Build/release ↔ repository:** supply-chain compromise can ship malicious code.

## 5. Threats and controls

### 5.1 Authentication and session threats

| Threat | Controls |
|---|---|
| Credential stuffing / brute force | Login rate limiting, generic errors, constant-time verification, Argon2id |
| User enumeration | Uniform error messages and timing where practical |
| Session theft via XSS | HttpOnly cookies, CSP, no token in URL, safe rendering |
| Session fixation | New random token on login; server-side sessions |
| CSRF on mutations | Double-submit token + Origin/Sec-Fetch-Site checks + SameSite |
| Session replay after logout | Server-side revocation; expiry checks on HTTP and WebSocket |
| Setup takeover | Setup only when no administrator exists; rate limited; setup disabled after completion |

### 5.2 SSH and credential threats

| Threat | Controls |
|---|---|
| MITM via host-key substitution | Strict verification, explicit fingerprint approval, changed-key block |
| Credential theft from database | AEAD encryption, key outside DB, least-privilege DB access |
| Credential leakage in logs/errors | Redaction, typed errors, no request-body logging |
| Silent credential fallback | No automatic retry with unrelated stored credentials |
| Unbounded connections | Per-user/global limits, timeouts, cleanup |
| Agent/port forwarding abuse | Disabled by default, not exposed in MVP |

### 5.3 Remote operation threats

| Threat | Controls |
|---|---|
| Command injection | Structured operations, enumerated parameters, fixed templates |
| Acting on wrong process | PID validation and identity recheck |
| False success | Re-read remote state; `OUTCOME_UNKNOWN` when unverifiable |
| Privilege escalation | No automatic sudo; show remote identity and permission errors |
| Output exhaustion | Bounded output, timeouts, cancellation, concurrency limits |

### 5.4 Browser content threats

| Threat | Controls |
|---|---|
| XSS via filenames/logs/process names | Render as text; no raw HTML; safe CSP |
| Terminal escape injection into the page | xterm.js owns escape sequences; surrounding page does not interpret them |
| Malicious file preview | Size limits, binary detection, no execution, safe content-disposition |
| Clickjacking | Frame-ancestors CSP / `X-Frame-Options` |
| Open redirect | No user-controlled redirects |

### 5.5 SSRF and network threats

| Threat | Controls |
|---|---|
| Access to loopback/admin services | Block by default; explicit operator override |
| Cloud metadata access | Block metadata ranges by default |
| DNS rebinding / TOCTOU | Resolve once, validate, connect to validated address, revalidate each attempt |
| Network scanning | Authenticated only; rate limits; destination concurrency cap; audit |
| Private-range abuse in multi-user mode | Admin allowlist/approval flow recommended |

### 5.6 Data and operational threats

| Threat | Controls |
|---|---|
| Database theft | Encrypted credentials; no plaintext secrets; file permissions |
| Backup theft | Documented encryption; separate key handling; restrict access |
| Key loss | Explicit warnings; backup guidance; future rotation procedure |
| Log leakage | Structured logging with redaction; no terminal/file/log content |
| Disk exhaustion | Transfer/output limits; retention policy; monitoring guidance |
| Dependency compromise | Pinned dependencies, Dependabot, CodeQL, gitleaks, review |
| Build compromise | Protected main, PR review, pinned Actions, release from tags |

### 5.7 Availability threats

| Threat | Controls |
|---|---|
| One bad host affects others | Failure isolation, per-host limits, timeouts |
| Resource exhaustion via sessions/transfers | Global/per-user limits, streaming, cancellation |
| Unbounded metric polling | Per-host coalescing and minimum interval |
| Panic kills the process | Boundary recovery, goroutine guards, crash-safety tests |
| Disk full / DB locked | Bounded writes, WAL, busy timeout, readiness checks |

## 6. Phase 1 focus

Phase 1 implements the application skeleton and local authentication. The controls that must be
proven in Phase 1:

- First-run setup can complete only once.
- Passwords are Argon2id-hashed; no plaintext storage.
- Session tokens are hashed at rest; expiry and logout revoke access.
- CSRF and Origin checks reject invalid mutations.
- Login rate limiting works.
- Secrets are redacted from logs and API errors.
- A deliberate panic is recovered and does not terminate the process.
- Security headers and bounded request bodies are enforced.

SSH, SFTP, terminal, monitoring and file threats are designed but not implemented until later
phases.

## 7. Residual risks

- A compromised administrator account or browser can control Drovik and reach all assigned servers.
- A compromised Drovik host with both the database and encryption key exposes stored credentials.
- A malicious remote host can return hostile content; rendering and output limits reduce but do not
  eliminate the risk.
- Self-hosted operators may misconfigure TLS, firewall rules, backups or egress policy.
- No system can guarantee availability against OOM, kernel failure or deliberate resource
  exhaustion beyond configured limits.

These risks must be documented for users; they are not hidden behind product claims.

## 8. Review triggers

Review this model when:

- A new remote operation or data type is added.
- Authentication, authorization or session behavior changes.
- A new network destination class is allowed.
- File upload/download or terminal behavior changes.
- A dependency with network/filesystem access is added.
- A security report is received.
- A new deployment model is supported.

## 9. References

- PRD §6 (security, privacy and threat model), §13 (workflow).
- `docs/security/security-architecture.md`
- ADR 0004, 0005, 0006, 0007, 0011.
