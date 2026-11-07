# Development Workflow

## Branching

- `main` is protected and PR-only.
- Branch from up-to-date `main` for every task:
  - `feat/...` — user-visible capability
  - `fix/...` — defect
  - `chore/...` — tooling, dependencies, repository
  - `docs/...` — documentation only
  - `test/...` — tests only
  - `security/...` — security hardening or fix
- One bounded task per branch. If a task grows, split it.
- Rebase on `main` only your own branch; never force-push `main`.

## Pull requests

1. Link the requirement ID or issue.
2. Explain the scope and the security impact.
3. List exact tests run and their results.
4. Include screenshots for UI changes, with synthetic data only.
5. Update docs, traceability and changelog.
6. Write a handoff note.
7. Wait for required checks and review; squash-merge; delete the branch.

## Required checks

The exact check names live in the repository ruleset. They will include:

- Backend format, vet, lint, tests with race detector.
- Frontend lint, typecheck, tests, build.
- Secret scan.
- CodeQL / static analysis.
- Dependency review.
- E2E smoke once available.

Do not merge with a failing or skipped required check unless the check is proven flaky and the
failure is documented in the PR.

## Commits

Conventional Commits. Examples:

```text
feat(auth): add first-run setup
fix(csrf): reject missing origin on mutation
docs(adr): record egress policy
test(recovery): cover panic in background goroutine
chore(deps): update chi
```

No AI attribution trailers. Sign off with `git commit -s` (DCO).

## Review standards

- Correctness before style.
- Security-sensitive paths need explicit review (CODEOWNERS).
- No secret-like values, real hosts or personal data.
- No disabled security controls.
- No unbounded loops, output or memory growth.
- No swallowed errors.
- Tests must be deterministic and not depend on external services unless marked integration.

## Handoff notes

Every session that made progress writes `docs/handoffs/YYYY-MM-DD-short-topic.md` using the
template in `docs/handoffs/README.md`. The next agent reads the latest relevant note first.

## Requirement traceability

Update `docs/product/traceability.md` when a requirement moves state. A requirement is not complete
just because a route or screen exists; acceptance criteria and failure paths must be covered.

## Releases

- Semantic versioning.
- Update `CHANGELOG.md`.
- Tag from `main` after CI is green.
- Release notes include upgrade and security notes.
- Release artifacts are built in CI, not on a workstation.

## When to stop and ask

- A request conflicts with `AGENTS.md` or an ADR.
- A security control would need to be weakened.
- A change requires real credentials, production hosts or private data.
- Scope is ambiguous or growing beyond the linked requirement.
- A test cannot be run and its result is needed to claim completion.
