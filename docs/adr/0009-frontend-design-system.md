# ADR 0009 — Frontend design system

- **Status:** Accepted
- **Date:** 2026-11-07

## Context

Drovik should feel like a deliberate, world-class server workspace, not a generic admin dashboard.
The UI is dense, operational and used for long sessions. It must be accessible, keyboard-friendly
and visually distinctive without distracting from server identity and dangerous actions.

## Decision

- React 19 + TypeScript strict + Vite + Tailwind CSS 4.
- Radix primitives for accessible behavior; custom visual styling on top.
- A single semantic token layer: color, spacing, radius, elevation, typography, motion.
- Visual direction: “control room” — deep ink surfaces, warm signal accent, precise typography,
  restrained motion, clear hierarchy, monospace for identifiers and metrics.
- Self-hosted variable fonts (no third-party font requests).
- Dark-first with a fully supported light theme.
- WCAG 2.2 AA for core flows: keyboard navigation, visible focus, contrast, reduced motion.
- Every screen defines loading, empty, success, warning, permission-denied, timeout, disconnected
  and error states.
- No component fetches data directly; use the typed API client and query layer.

The visual language is inspired by the Rune/OpenCode design work, but the Rune framework is not
used. No code or assets are copied without license review.

## Consequences

- Design tokens must be maintained centrally; raw values in components are rejected in review.
- Storybook or a component gallery is desirable once components stabilize.
- Accessibility testing is part of the definition of done.
- The UI will intentionally look different from generic Tailwind templates.

## Alternatives considered

- Material UI/Ant Design: fast, but visually generic and heavy for a bespoke operational UI.
- Rune (Rust/WASM): rejected; see ADR 0001.
- `@opencode/ui` (SolidJS): rejected; different framework and design ownership.

## References

PRD §5.2, §7.4, §7.5; AGENTS.md frontend design contract.
