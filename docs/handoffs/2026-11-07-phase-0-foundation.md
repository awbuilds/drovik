# 2026-11-07 Phase 0 foundation

## Task / Area
Bootstrap the Drovik repository, governance, AGENTS.md contract, handoff system, ADRs, threat
model and protected `main` ruleset.

## Current State
- Local repository initialized from `https://github.com/awbuilds/drovik.git`; working branch
  `chore/repo-foundation`.
- `main` ruleset `main-protection` is active: PR required, conversation resolution, linear
  history, no force-push, no deletion, no bypass actors. Required status checks will be added once
  CI exists.
- Repository settings: squash-only merges, delete branch on merge, discussions enabled, secret
  scanning and push protection enabled.
- Foundation files created: AGENTS.md, README, AGPL-3.0 LICENSE, CONTRIBUTING, CODE_OF_CONDUCT,
  SECURITY, SUPPORT, GOVERNANCE, CHANGELOG, ROADMAP, issue/PR templates, CODEOWNERS, Dependabot.
- Docs created: PRD moved to `docs/product/PRD.md`, traceability skeleton, status goal, ADRs
  0001–0012, threat model, security architecture, architecture overview, development setup and
  workflow, handoff README.
- Scripts and hooks created: verification, branch guard, handoff generator, pre-commit hook.
- No backend or frontend application code exists yet.

## Decisions
- React + Tailwind + Radix with a Rune/OpenCode-inspired design language; the Rune Rust framework
  is not used (ADR 0001, 0009).
- AGPL-3.0-only; copyright Abhijith Warrier.
- Go latest stable; SQLite via `modernc.org/sqlite`; goose migrations; Argon2id; opaque sessions.
- Egress default: allow public + RFC1918; block loopback/link-local/metadata.
- PR-only `main` with 0 required approvals (solo maintainer), required checks added with CI.

## Files Touched
- `AGENTS.md`, `README.md`, `LICENSE`, governance files, `.github/`, `scripts/`, `.githooks/`.
- `docs/` (product, adr, security, architecture, development, handoffs, status).
- `docs/product/PRD.md` moved from the repository root.

## Build / Test Status
- Not yet run: no backend/frontend code exists. `make verify` will report the missing components
  and pass only after Phase 1 code lands.
- Git ruleset and repository settings verified through the GitHub API.

## Next Steps
- Commit and push `chore/repo-foundation`; open a PR against `main`.
- Add CI and required status checks.
- Implement the Phase 1 backend foundation and authentication.
- Implement the Phase 1 frontend foundation and design system.
- Add Docker Compose, reverse-proxy examples and E2E smoke test.

## Open Questions
- Add a second maintainer later if 1-approval reviews are desired.
- Release signing/SBOM timing is deferred to Phase 8.
