# Architecture Decision Records

This directory records durable architecture decisions. Each ADR states the context, the decision,
the consequences and alternatives. ADRs are immutable once accepted; a later ADR supersedes an
earlier one explicitly.

| ADR | Title | Status |
|---|---|---|
| [0001](0001-language-and-stack.md) | Backend and frontend language stack | Accepted |
| [0002](0002-monorepo-layout.md) | Monorepo layout | Accepted |
| [0003](0003-sqlite-and-migrations.md) | SQLite and migrations | Accepted |
| [0004](0004-authentication-and-sessions.md) | Authentication and sessions | Accepted |
| [0005](0005-secret-encryption.md) | Secret encryption at rest | Accepted |
| [0006](0006-ssh-host-key-trust.md) | SSH host-key trust | Accepted |
| [0007](0007-structured-remote-operations.md) | Structured remote operations | Accepted |
| [0008](0008-api-and-versioning.md) | API and versioning | Accepted |
| [0009](0009-frontend-design-system.md) | Frontend design system | Accepted |
| [0010](0010-licensing.md) | Licensing | Accepted |
| [0011](0011-egress-policy.md) | Egress policy defaults | Accepted |
| [0012](0012-deployment-model.md) | Deployment model | Accepted |

## Template

```markdown
# ADR NNNN — Title

- **Status:** Proposed | Accepted | Superseded by ADR-XXXX
- **Date:** YYYY-MM-DD
- **Decision owners:** ...

## Context
What problem or force requires a decision?

## Decision
What was decided, precisely.

## Consequences
What becomes easier, harder, required or forbidden.

## Alternatives considered
What else was considered and why it was rejected.

## References
Requirement IDs, docs, links.
```
