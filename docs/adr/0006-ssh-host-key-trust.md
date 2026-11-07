# ADR 0006 — SSH host-key trust

- **Status:** Accepted
- **Date:** 2026-11-07

## Context

SSH host-key verification is the primary defense against man-in-the-middle attacks. Drovik is a
gateway to servers that may hold sensitive data, so convenience shortcuts such as disabling
host-key checking are unacceptable.

## Decision

- Always verify the SSH host key using `golang.org/x/crypto/ssh` host-key callbacks.
- First connection to an unknown host: present the key type and SHA-256 fingerprint to the user and
  require explicit trust before authentication.
- Changed host key: block the connection by default, show a prominent warning that the change may
  indicate a rebuild or a MITM attack, and require deliberate re-verification.
- Store approved keys in a persistent known-hosts store keyed by server identity (host, port, key
  type), with approval metadata.
- Never fall back to `InsecureIgnoreHostKey`, even in development or tests against real hosts.
- Never reuse an approved key for a different host or port.
- Include the fingerprint in audit metadata, but never the private key material.
- Test first-use, changed-key, revoked-key and mismatch paths with disposable SSH containers.

## Consequences

- Users must make an explicit trust decision on first connection and after a legitimate rebuild.
- Automated tests need a controlled test SSH server with generated keys.
- A compromised known-hosts record is a serious event; revocation and re-approval must be possible.

## Alternatives considered

- Trust-on-first-use without a prompt: rejected; the user must see and approve the fingerprint.
- Global `known_hosts` file only: insufficient for per-connection ownership and audit.
- Disabling verification for localhost/dev: rejected.

## References

PRD SSH-006–SSH-009, §6.3; AGENTS.md.
