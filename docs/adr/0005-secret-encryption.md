# ADR 0005 — Secret encryption at rest

- **Status:** Accepted
- **Date:** 2026-11-07

## Context

Drovik stores SSH passwords and private keys so it can connect to managed hosts. A database leak
must not expose those credentials. The PRD requires authenticated encryption and a key stored
outside the SQLite database.

## Decision

- Encrypt credential material at the application layer with AES-256-GCM (or an equivalent reviewed
  AEAD).
- Use a random nonce per encryption; store the nonce and key version alongside the ciphertext.
- Supply the 32-byte key through an environment secret or a mounted secret file. Never store it in
  the database or repository.
- Include associated data binding the ciphertext to its server/credential record.
- Decrypt only for the shortest period required to establish a connection; do not return secrets to
  the browser.
- Redact secrets from logs, errors, audit events and traces.
- Document that losing the key makes stored credentials unrecoverable, and that backups must include
  the database and key separately and securely.
- Plan key rotation as a versioned envelope: a key-encryption key wraps data-encryption keys.

## Consequences

- Operators must manage one additional secret.
- Credential records are not portable without the key.
- Key rotation is a documented, testable procedure.
- Encryption failures must fail closed and never silently store plaintext.

## Alternatives considered

- SQLCipher: encrypts the whole database but still requires key management and couples encryption
  to the storage engine.
- Plaintext storage: unacceptable.
- OS keychain only: not portable to containers and headless servers.

## References

PRD §4.3 credential storage, §6.3, §8.6; AGENTS.md.
