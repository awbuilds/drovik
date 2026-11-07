# ADR 0001 — Backend and frontend language stack

- **Status:** Accepted
- **Date:** 2026-11-07

## Context

Drovik must manage SSH and SFTP connections, stream terminal I/O, serve a web UI and run as a
small self-hosted service. The PRD baseline specifies Go for the backend and React + TypeScript
for the frontend. The project needs a stack that is maintainable, security-reviewable, easy to
deploy and suitable for AI-assisted development with strong tooling.

## Decision

- **Backend:** Go (latest stable), with a clear separation between HTTP, domain services, SSH,
  SFTP, persistence, secrets and audit.
- **Frontend:** React 19 + TypeScript (strict) + Vite + Tailwind CSS 4, with Radix primitives for
  accessible components.
- **API:** versioned JSON over HTTPS, plus WebSocket for terminal streaming.
- **Database:** SQLite for the initial self-hosted deployment, behind repository interfaces.

The Rune Rust UI framework was evaluated and is **not** used. Drovik is a browser-based,
self-hosted web application; Rune is a pre-release native/canvas UI framework whose web target and
accessibility story do not match the PRD's browser and DOM requirements. Drovik adopts a
Rune/OpenCode-inspired visual language instead.

## Consequences

- Go's error-return model replaces exceptions; crash safety is implemented with boundary recovery
  and explicit error handling (see AGENTS.md).
- A single Go binary plus static frontend assets is straightforward to deploy.
- The frontend remains accessible, indexable and testable in normal browsers.
- The team must maintain two toolchains (Go and Node/pnpm).

## Alternatives considered

- **Rust + Rune:** rejected for the MVP because it is pre-release, canvas-based, changes the
  deployment and accessibility model, and contradicts the PRD's React/web baseline.
- **Node/TypeScript backend:** rejected because Go's SSH/SFTP ecosystem, concurrency model,
  static binary and operational simplicity fit the product better.
- **SolidJS + `@opencode/ui`:** rejected because it replaces the PRD's React baseline for visual
  familiarity only; the design language can be reproduced in React.

## References

PRD §0.1, §5.1–5.3; AGENTS.md.
