# Contributing to Drovik

Thank you for considering a contribution. Drovik is a security-sensitive, self-hosted product, so
we value clear scope, honest testing and careful security work over speed.

## Before you start

1. Read [AGENTS.md](AGENTS.md) — it contains the hard rules, architecture and crash-safety contract.
2. Read the relevant requirement in [docs/product/PRD.md](docs/product/PRD.md).
3. Read the latest [handoff note](docs/handoffs/).
4. Check open issues and discussions; open an issue before large changes so scope can be agreed.
5. For security vulnerabilities, follow [SECURITY.md](SECURITY.md) instead of opening a public issue.

## Development workflow

1. Fork or branch from up-to-date `main`. Never commit to `main` directly.
2. Use a descriptive branch: `feat/...`, `fix/...`, `chore/...`, `docs/...`, `test/...`,
   `security/...`.
3. Keep one bounded task per pull request.
4. Implement backend validation and authorization before relying on frontend controls.
5. Add tests alongside the implementation.
6. Run the relevant checks before opening the PR.
7. Update documentation, requirement traceability and `CHANGELOG.md`.
8. Write a handoff note for any session that made progress.
9. Open a pull request using the template; link the requirement/issue.

## Commit messages

Use Conventional Commits:

```text
feat: add first-run setup endpoint
fix: reject expired sessions on WebSocket upgrade
docs: record SSH host-key trust decision
test: cover CSRF rejection
chore: pin GitHub Actions
```

Do not add `Co-Authored-By` or other attribution trailers naming anyone other than the repository
owner. Do not add AI attribution.

## Developer Certificate of Origin

By contributing, you certify that you have the right to submit the contribution under the project
license. Sign off your commits with `git commit -s` (DCO). The sign-off is a certification, not a
copyright assignment.

## Code expectations

- Go: idiomatic, context-aware, errors wrapped with `%w`, no swallowed errors, no unbounded work.
- TypeScript: strict types, no `any` without a documented reason, one typed API client.
- Security: no secrets, no command construction from untrusted input, no disabled host-key checks.
- UX: accessible, keyboard-friendly, explicit loading/error/empty states, no dark patterns.
- Tests: deterministic, disposable fixtures, no real credentials or production hosts.

## Review and merge

- Required CI checks must pass.
- Conversations must be resolved.
- The change must not introduce a known critical or high security issue.
- The maintainer squash-merges; the branch is deleted after merge.

## License

Contributions are accepted under the GNU Affero General Public License v3.0 only
(`AGPL-3.0-only`). See [LICENSE](LICENSE).
