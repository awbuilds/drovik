# ADR 0014 — Project website

- **Status:** Accepted
- **Date:** 2026-11-08

## Context

Drovik needs a public face that explains the product, its security boundary,
its install path, its limits and its legal position. The application's design
contract is a specific "control room" language; a generic dashboard template
would misrepresent the product. The repository already has a detailed docs
tree, a release pipeline and a strict no-telemetry rule. The site must not add
a build system, a runtime service, a third-party font request, analytics or a
second source of truth for the design.

## Decision

- The website is a static site under `site/`, written as HTML, CSS and
  dependency-free JavaScript. There is no build step and no runtime service.
- Visual tokens are copied from `frontend/src/styles/tokens.css`; the site is
  dark-first with the application's fully supported light theme. The same
  self-hosted variable fonts, wordmark and semantic colors are used.
- The site makes no third-party requests: no analytics, advertising, cookies,
  CDNs, embedded frames or remote fonts. External links are ordinary anchors.
  The only browser storage is a theme preference in `localStorage`.
- The site owns the public pages the application does not: product tour,
  security posture, install/deploy, a docs index, FAQ, changelog, privacy,
  terms, accessibility and 404. The repository's `docs/` tree remains the
  source of truth for engineering documentation; the site links to it.
- Page chrome is duplicated for static hosting and no-JavaScript resilience.
  `scripts/check-site.py` enforces navigation parity, metadata, internal links,
  CSP-safe markup, sitemap coverage, data hygiene and design-token parity.
  The checker runs in `make verify` and in a dedicated CI job.
- The site is deployment-agnostic. Canonical URLs currently use the GitHub
  Pages project base; moving to a custom domain is a documented search-and-
  replace. `_headers` carries a strict same-origin CSP for hosts that apply it.
- The published site describes the current stable release line. It must not
  advertise a tag that does not exist; the release checklist records this.

## Consequences

- Beautiful, fast pages with no supply-chain surface and no privacy cost.
- Chrome, navigation and version references are duplicated; the checker
  catches drift, and changes touching the shell must update every page.
- The token block is duplicated by necessity; the checker fails if the app and
  site tokens diverge, and both must change together.
- The site cannot use server-side includes or runtime personalization. That is
  acceptable for a project marketing and trust site.
- Legal pages are a project policy, not legal advice, and must be reviewed by
  the maintainer for the deployment's jurisdiction.

## Alternatives considered

- **React/Vite or Next.js site:** adds a build, dependencies and a second
  runtime for a handful of static pages; rejected.
- **Docs generator (MkDocs, Docusaurus):** good for engineering docs, but a
  poor fit for the product/trust pages and adds a toolchain; the existing docs
  tree stays on GitHub.
- **Copying the AuraSpend site:** its structure (static pages, sitemap, legal
  surface) was a useful reference, but its visual language is for a different
  product; rejected.
- **Server-rendered marketing pages inside the Go service:** couples the
  public site to the application process, image and release cadence; rejected.

## References

`docs/adr/0009-frontend-design-system.md`, `site/README.md`,
`scripts/check-site.py`, `AGENTS.md` frontend design contract, `LICENSE`.
