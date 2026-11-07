# ADR 0002 — Monorepo layout

- **Status:** Accepted
- **Date:** 2026-11-07

## Context

The frontend, backend, deployment assets, documentation and security material evolve together.
The project is small enough that a monorepo keeps changes atomic and reduces coordination cost,
while still needing clear boundaries between production code, deployment and docs.

## Decision

Use a monorepo:

```text
backend/      Go service (own go.mod)
frontend/     React application (own package.json)
deploy/       Docker, Compose, Caddy, Nginx, systemd
docs/         product, architecture, ADR, security, development, handoffs
scripts/      verification, hooks, handoff helpers
testdata/     sanitized fixtures only
.github/      CI, security workflows, templates
```

Each application keeps its own dependency manifest and lockfile. CI uses path filters where
useful, but the default verification runs both sides.

## Consequences

- One pull request can update API, UI, tests and docs together.
- Clear ownership and review paths via CODEOWNERS.
- CI must avoid rebuilding unrelated components; caching and path filters matter.
- No shared code package initially; the OpenAPI contract is the interface.

## Alternatives considered

- Separate repositories: more release overhead and cross-repo coordination for a small team.
- Single root package: blurs the Go/Node boundary and complicates tooling.

## References

PRD §12.
