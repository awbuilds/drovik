# Governance

Drovik is currently maintained by Abhijith Warrier as the project owner.

## Decision making

- Day-to-day technical decisions are made by the maintainer with input from contributors.
- Durable architecture and security decisions are recorded in `docs/adr/` and `AGENTS.md`.
- Product scope follows `docs/product/PRD.md`; changes to the MVP boundary require a documented
  decision and a roadmap update.
- Security fixes may be prioritised over feature work.

## Roles

| Role | Responsibilities |
|---|---|
| Maintainer | Roadmap, reviews, releases, security response, repository settings |
| Contributor | Issues, pull requests, tests, documentation |
| Security reporter | Private vulnerability reports under `SECURITY.md` |

Additional maintainers may be added as the project grows. A new maintainer must have a consistent
contribution history, agree to the security and review rules, and be added by the current owner.

## Pull requests

- `main` is protected and PR-only.
- Required checks and conversation resolution must pass.
- The maintainer performs the final review and squash-merge.
- No direct pushes, force-pushes or admin bypass.

## Releases

- Semantic versioning.
- `CHANGELOG.md` is updated for user-visible changes.
- Release artifacts are built from tagged commits in CI.
- Security fixes may ship as patch releases outside the normal cadence.

## Changes to this document

Open a pull request with the proposed change and rationale. The maintainer approves governance
changes.
