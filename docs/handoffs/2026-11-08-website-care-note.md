# 2026-11-08 Website use-with-care note

## Task / Area

Add a plain-language notice to the website that Drovik is published in good faith and without
warranty, and that operators remain responsible for any data loss.

## Current State

- The home page has a full-width warning note after the install section: use the application with
  care, keep tested backups, rehearse changes on a host you can afford to lose, and review
  destructive actions. It links to the terms.
- The terms page has the same warning as a visible note after the short version, a summary bullet,
  and a "Data and recovery" bullet under operator responsibilities. The note keeps the
  "to the extent permitted by law" qualifier.
- No version change; the site remains on the v1.3.0 line. No application code changed.
- The edits are uncommitted alongside unrelated in-progress backend/frontend and
  changelog work on `feat/phase-9-completion`; `CHANGELOG.md` was deliberately left to that work.
  No commit or push was performed.

## Decisions

- Plain language on the home page; the legal qualifier lives in the terms. Both make the
  data-loss risk explicit without implying the project can recover data it cannot access.
- Reused the existing `.note--warn` and prose styles; no new CSS or JavaScript.

## Files Touched

- `site/index.html` — home warning section.
- `site/terms.html` — summary bullet, visible note and operator-responsibility bullet.
- `docs/handoffs/2026-11-08-website-care-note.md` — this note.

## Build / Test Status

- `python3 scripts/check-site.py` — passed.
- `make site-check` — passed.
- Playwright/Chrome — the notes render at 1440px and 390px in dark and light; no console errors;
  no horizontal overflow at 320/390/768/1440.
- axe-core — 0 violations on the home and terms pages in both themes.
- `make verify` was not run: the branch has unrelated in-progress backend/frontend changes and
  this is a website copy-only change.

## Next Steps

- Review the wording and placement; adjust the terms effective date if the note is published on a
  different day.
- Include these files in the website PR when the site work is separated; do not commit to `main`.
