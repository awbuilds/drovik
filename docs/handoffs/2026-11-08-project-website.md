# 2026-11-08 Project website

## Task / Area

Build and verify a complete static website for Drovik, matching the application's control-room
design and covering the product, security, install, docs, FAQ, changelog, privacy, terms,
accessibility and 404 pages.

## Current State

- `site/` is a dependency-free static site: 11 pages, one layered stylesheet, two small JS files,
  self-hosted Latin variable fonts (OFL), favicon/app icons and a 1200×630 social card.
- Content targets **v1.1.1**, the stable line in the current README. v1.1.1 is not tagged yet; do
  not publish before the tag exists, or revert the version references to the last published tag.
- `scripts/check-site.py` (stdlib only) checks links/fragments/assets, metadata, CSP-safe markup,
  sitemap coverage, nav parity, design-token parity, GitHub docs paths, data hygiene and PNG
  dimensions. It runs in `make verify` and the new `Site` CI job.
- Added `site/README.md`, ADR 0014 and the ADR index entry. Updated root README, CHANGELOG,
  Makefile, `scripts/verify.sh`, `.github/workflows/ci.yml`, `.dockerignore` and `.gitignore`.
- The site work is **uncommitted on `main`**. No application code changed. Concurrent commits
  landed while this work was in progress; one (`0be160d`) swept the CHANGELOG site bullet into an
  unrelated build commit. The site, checker, ADR, handoff and wiring files remain uncommitted.
  Do not commit to `main`; move the paths to `feat/project-website` and open a PR.
- Canonical URLs use `https://awbuilds.github.io/drovik/`; a custom domain is a documented
  search-and-replace. `_headers` carries the strict same-origin CSP for hosts that apply it;
  GitHub Pages ignores it.

## Decisions

- Static HTML/CSS/JS, no build step and no runtime service. The app remains the only service.
- Tokens mirror `frontend/src/styles/tokens.css`; the checker fails on drift. Dark is primary; the
  light theme uses the app's warm-paper values. No third-party requests, cookies or analytics;
  only a `localStorage` theme preference.
- Chrome is duplicated per page for static hosting/no-JS resilience; the checker enforces nav
  parity. Engineering docs stay in the repository and are linked, not duplicated.
- Legal pages use India/Kerala governing law with a non-waivable-rights carve-out. They are a
  good-faith policy, not legal advice; review before publishing. Contact is via GitHub only.
- No commit, push or PR was created. The current branch is `main`.

## Files Touched

- `site/` — pages, CSS, JS, assets, fonts/licenses, headers/discovery files, README.
- `scripts/check-site.py`; `Makefile`, `scripts/verify.sh`, `.github/workflows/ci.yml`.
- `README.md`, `CHANGELOG.md`; `docs/adr/0014-project-website.md`, `docs/adr/README.md`.
- `.dockerignore`, `.gitignore`; `docs/handoffs/2026-11-08-project-website.md`.

## Build / Test Status

- `make verify` — exit 0: gofmt, vet, `go test -race ./...`, golangci-lint (0 issues), pnpm frozen
  install, frontend lint/typecheck (27 tests), production build, site check.
- `python3 scripts/check-site.py` — passed for all 11 pages; 41 dark and 31 light application
  tokens match; sitemap, nav parity, GitHub docs paths and data hygiene pass.
- Playwright + Chrome — screenshots at 1440×1000 and 390×844 in both themes; no console/page
  errors; no horizontal overflow at 320/360/390/768/1024/1440; no external requests and no cookies.
- axe-core — 0 violations on all 11 pages in dark and light themes.
- Interaction checks — theme persistence, mobile menu/Escape/focus return, keyboard tabs, copy
  success/failure, no-JS content/nav, anchor offset and reduced motion all pass.
- Production CSP simulation — no violations; theme JS, tabs and JSON-LD work.
- `git diff --check` clean. Legacy `tidy` only reports HTML5/SVG element non-recognition.

## Next Steps

1. Review pages/screenshots, especially v1.1.1 audit copy and legal text.
2. Confirm governing law/contact and the final URL with the maintainer.
3. Tag v1.1.1 (or adjust site version) before deploying.
4. Create `feat/project-website`, stage only these paths, run CI, open a PR.
5. Configure Pages/hosting, apply `_headers` at the proxy if needed, and consider making the
   `Site` CI job required.

## Open Questions

- GitHub Pages project base or a custom domain?
- Keep India/Kerala as the governing jurisdiction?
- Does v1.1.1 ship before the site is published?
