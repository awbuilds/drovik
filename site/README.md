# Drovik project website

A dependency-free static site for the Drovik project. It lives beside the
application but is not part of the application image.

- **No build step.** Edit the HTML/CSS/JS and open or serve it.
- **No third-party requests.** Fonts, scripts, styles, icons and images are
  served from this directory. There are no analytics, cookies, embeds or CDNs.
- **Two themes.** Dark is the primary design; light uses the application's
  warm-paper tokens. The choice is stored in `localStorage`, never sent.
- **Matches the product.** Colors, fonts, radii, shadows and motion mirror
  `frontend/src/styles/tokens.css`; `scripts/check-site.py` fails if they drift.

## Preview

```bash
# from the repository root
python3 -m http.server 4173 --directory site
```

Open <http://127.0.0.1:4173/>. Opening `index.html` directly also works; the
copy buttons hide themselves when the clipboard API is unavailable.

## Check

```bash
python3 scripts/check-site.py
```

The checker requires no packages. It verifies:

- local links, fragments, stylesheets, scripts, fonts and icons resolve;
- every page has one `<h1>`, a main landmark, title, description, canonical,
  Open Graph/Twitter metadata and a viewport;
- no inline scripts/styles or external subresources (the CSP assumes none);
- the sitemap covers every indexable page and excludes `404.html`;
- primary and footer-legal navigation match across pages;
- color/font/radius/shadow tokens still match the application token file;
- GitHub documentation links point at files/directories that exist;
- no published emails, non-documentation IPs or absolute user paths;
- the social card and icons have the expected dimensions.

The script also runs in CI (the `Site` job) and from `make site-check`.

## Structure

```text
site/
  index.html  features.html  security.html  install.html  docs.html
  faq.html  changelog.html  privacy.html  terms.html  accessibility.html  404.html
  css/style.css                 one layered stylesheet; tokens at the top
  js/theme-init.js              pre-paint theme and js-class bootstrap
  js/site.js                    theme, menu, tabs, copy; no dependencies
  assets/                       favicon, social card, app icons
  fonts/                        Latin variable subsets + OFL licenses
  _headers                      CSP and security headers for capable hosts
  .well-known/security.txt      private reporting contact
  robots.txt  sitemap.xml  llms.txt  .nojekyll
```

The header and footer are intentionally duplicated in each page so the site
works with JavaScript disabled and on any static host. The checker enforces
that the shared navigation does not drift. When changing a page, keep the
header/footer blocks and version references consistent.

## Editing rules

- **Do not add a third-party script, font, image or embed.** The site's privacy
  claim and the CSP depend on it.
- **Do not add inline `style` attributes or inline `<script>` blocks.** The
  CSP in `_headers` is `default-src 'none'` with self-only assets. JSON-LD on
  the home page is the one allowed non-executable data block.
- **Use the tokens.** No raw hex values in component CSS; colors, fonts, radii
  and shadows come from the token block. The token block is checked against
  the application.
- **Respect progressive enhancement.** Content and navigation must work with
  JavaScript disabled; enhancement controls are hidden when they cannot work.
- **Use synthetic data.** Examples use `example.com`, RFC 5737 addresses and
  generic names such as `prod-web-01` and `deploy`. Never add a real host,
  credential, email or screenshot from a live machine.
- **Keep the legal copy current.** Privacy and terms describe the published
  software version and website behavior; update the effective date when the
  handling changes.

## Release checklist

The site currently targets **v1.2.0** (the stable line declared in the
repository's README). Do not publish it before that tag exists, or revert the
version to the last published tag first.

When a release changes the site's content:

1. Update the version references in `index.html`, `features.html`,
   `security.html`, `docs.html`, `faq.html`, `changelog.html`, `privacy.html`,
   `terms.html`, the home JSON-LD, the footer and `llms.txt`.
2. Update the changelog entry and the security audit summary if the release
   includes fixes.
3. Regenerate `assets/og.png` with the new version (see below).
4. Run `python3 scripts/check-site.py`.
5. Re-check the changed pages at 1440px and 390px in both themes, with a
   keyboard and with JavaScript disabled.
6. Update the handoff note.

## Deployment

Publish the **contents** of this directory at the web root of any static host
(GitHub Pages, Cloudflare Pages, Netlify, S3, nginx). All internal links are
relative, so a subpath works.

- **Canonical base:** all canonical, Open Graph, sitemap, `robots.txt` and
  `security.txt` URLs currently use `https://awbuilds.github.io/drovik/`. If
  the site moves to a custom domain, update that base everywhere it appears
  (search the directory for `awbuilds.github.io/drovik`).
- **GitHub Pages:** the repository's `docs/` folder is project documentation,
  not this site. Publish `site/` through a Pages workflow or copy its contents
  to the Pages branch. `404.html` is picked up automatically.
- **404:** for nginx, use `error_page 404 /404.html;`; for Caddy, use
  `handle_errors`. The page uses relative links so it works from a subpath.
- **Headers:** `_headers` is honored by Netlify and Cloudflare Pages. GitHub
  Pages ignores it; configure the equivalent CSP and security headers at the
  proxy or hosting layer. The policy allows only same-origin assets and no
  forms, frames or connections.
- No server, database, analytics or external service is required.

## Assets

- `assets/favicon.svg` is copied from `frontend/public/favicon.svg`.
- `assets/icon-512.png` and `assets/apple-touch-icon.png` are rasterized from
  the same mark. The 180px icon is a Lanczos resize of the 512px source.
- `assets/og.png` is a 1200×630 social card rendered from the brand tokens
  with the self-hosted fonts. Regenerate it whenever the version or headline
  changes; keep it under 1 MB.
- Fonts are the Latin variable subsets from the application's
  `@fontsource-variable` packages. Their OFL license files are in
  `fonts/`; do not remove them.

## Accessibility and browser support

The target is WCAG 2.2 AA for the site. The static pages were checked with
axe-core, a keyboard pass, reduced-motion emulation and a no-JavaScript pass.
Wide tables and code blocks scroll in labelled regions; focus is visible on
both themes. The site is tested at 320–1440px. See `accessibility.html` for
the public statement and known limits.

## License

The site is part of the Drovik repository and is licensed under
AGPL-3.0-only. Fonts are under the SIL Open Font License; see `fonts/`.
