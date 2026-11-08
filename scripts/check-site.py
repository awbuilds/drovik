#!/usr/bin/env python3
"""Check the Drovik static site.

No dependencies beyond the Python standard library. The goal is not to be a
general HTML validator: it enforces the invariants this site relies on.

  * every local link, fragment and subresource resolves
  * every page has the expected metadata, one <h1>, and no inline JS/CSS
  * the sitemap covers every indexable page and 404 is excluded
  * the primary navigation, footer legal links and page chrome agree
  * the site's color/font tokens still mirror frontend/src/styles/tokens.css
  * documentation links point at paths that exist in the repository
  * no emails, non-documentation IPs or absolute user paths are published

Exit code is 0 only when every check passes.
"""

from __future__ import annotations

import re
import struct
import sys
import xml.etree.ElementTree as ET
from pathlib import Path
from urllib.parse import unquote, urlsplit

ROOT = Path(__file__).resolve().parent.parent
SITE = ROOT / "site"
TOKENS = ROOT / "frontend" / "src" / "styles" / "tokens.css"
BASE = "https://awbuilds.github.io/drovik/"
BASE_HOST = "https://awbuilds.github.io/drovik/"
GH = "https://github.com/awbuilds/drovik"

ALLOWED_IPS = {"127.0.0.1"}
ALLOWED_IP_PREFIXES = ("192.0.2.", "198.51.100.", "203.0.113.")
IP_RE = re.compile(r"(?<![\w.])(?:\d{1,3}\.){3}\d{1,3}(?![\w.])")
EMAIL_RE = re.compile(r"[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}")
URL_RE = re.compile(r"url\(\s*['\"]?([^'\")]+)['\"]?\s*\)", re.IGNORECASE)
TOKEN_RE = re.compile(r"(--[A-Za-z0-9-]+)\s*:\s*([^;]+);")
ASSET_RELS = {"stylesheet", "icon", "apple-touch-icon", "preload", "manifest"}


class Problem:
    def __init__(self) -> None:
        self.errors: list[str] = []

    def add(self, path: Path | str, line: int | None, message: str) -> None:
        location = str(path)
        if line:
            location += f":{line}"
        self.errors.append(f"{location}: {message}")


class PageParser:
    """Small, deliberately tolerant HTML inspector."""

    def __init__(self, path: Path) -> None:
        self.path = path
        self.ids: dict[str, int] = {}
        self.links: list[tuple[str, int]] = []
        self.assets: list[tuple[str, int, str]] = []
        self.inline_scripts: list[tuple[str, int]] = []
        self.style_attrs: list[int] = []
        self.style_tags: list[int] = []
        self.unsafe_blank: list[tuple[str, int]] = []
        self.metas: dict[str, str] = {}
        self.meta_lines: dict[str, int] = {}
        self.title = ""
        self.title_line = 0
        self.h1_count = 0
        self.main_count = 0
        self.lang = ""
        self.navs: list[dict] = []
        self._nav_stack: list[dict] = []
        self._in_title = False
        self._current_tag = ""
        self.headings: list[tuple[str, int]] = []

    def parse(self) -> None:
        from html.parser import HTMLParser

        parser = self

        class Handler(HTMLParser):
            def __init__(self) -> None:
                super().__init__(convert_charrefs=True)

            def handle_starttag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
                values = {key: (value or "") for key, value in attrs}
                line = self.getpos()[0]

                if "style" in values:
                    parser.style_attrs.append(line)

                if tag == "html":
                    parser.lang = values.get("lang", "")
                elif tag == "title":
                    parser.title_line = line
                    parser._in_title = True
                elif tag in {"h1", "h2", "h3"}:
                    parser.headings.append((tag, line))
                    if tag == "h1":
                        parser.h1_count += 1
                elif tag == "main":
                    parser.main_count += 1
                elif tag == "a":
                    href = values.get("href", "")
                    if href:
                        parser.links.append((href, line))
                        if parser._nav_stack:
                            parser._nav_stack[-1]["links"].append(href)
                    if values.get("target") == "_blank":
                        rel = values.get("rel", "").lower().split()
                        if "noopener" not in rel and "noreferrer" not in rel:
                            parser.unsafe_blank.append((href, line))
                elif tag == "link":
                    rel = values.get("rel", "").lower()
                    href = values.get("href", "")
                    if rel in ASSET_RELS and href:
                        parser.assets.append((href, line, f"link[{rel}]"))
                elif tag == "script":
                    src = values.get("src", "")
                    if src:
                        parser.assets.append((src, line, "script"))
                    else:
                        script_type = values.get("type", "").lower()
                        if script_type != "application/ld+json":
                            parser.inline_scripts.append((script_type or "classic", line))
                elif tag == "style":
                    parser.style_tags.append(line)
                elif tag in {"img", "source", "video", "audio"}:
                    src = values.get("src", "")
                    if src:
                        parser.assets.append((src, line, tag))
                elif tag == "nav":
                    nav = {"label": values.get("aria-label", ""), "links": []}
                    parser.navs.append(nav)
                    parser._nav_stack.append(nav)
                elif tag == "meta":
                    key = values.get("name") or values.get("property")
                    if key:
                        parser.metas[key] = values.get("content", "")
                        parser.meta_lines[key] = line

                if "id" in values:
                    ident = values["id"]
                    parser.ids[ident] = parser.ids.get(ident, 0) + 1

            def handle_startendtag(
                self, tag: str, attrs: list[tuple[str, str | None]]
            ) -> None:
                self.handle_starttag(tag, attrs)

            def handle_endtag(self, tag: str) -> None:
                if tag == "title":
                    parser._in_title = False
                elif tag == "nav" and parser._nav_stack:
                    parser._nav_stack.pop()

            def handle_data(self, data: str) -> None:
                if parser._in_title:
                    parser.title += data

        handler = Handler()
        handler.feed(self.path.read_text(encoding="utf-8"))
        handler.close()


def normalize_css(value: str) -> str:
    value = value.strip().lower().replace('"', "'")
    value = re.sub(r"\s+", " ", value)
    value = value.replace(" ,", ",").replace(", ", ",")
    return value.strip()


def css_block(text: str, selector: str, problem: Problem, source: Path) -> str:
    index = text.find(selector)
    if index < 0:
        problem.add(source, None, f"missing CSS block {selector!r}")
        return ""
    start = text.find("{", index)
    if start < 0:
        problem.add(source, None, f"unclosed CSS block {selector!r}")
        return ""
    depth = 0
    for position in range(start, len(text)):
        char = text[position]
        if char == "{":
            depth += 1
        elif char == "}":
            depth -= 1
            if depth == 0:
                return text[start + 1 : position]
    problem.add(source, None, f"unclosed CSS block {selector!r}")
    return ""


def css_tokens(block: str) -> dict[str, str]:
    tokens: dict[str, str] = {}
    for match in TOKEN_RE.finditer(block):
        tokens[match.group(1)] = normalize_css(match.group(2))
    return tokens


def local_target(page: Path, href: str, problem: Problem, line: int) -> tuple[Path, str] | None:
    split = urlsplit(href)
    if split.scheme in {"http", "https", "mailto", "tel", "data", "javascript"}:
        if split.scheme == "http":
            problem.add(page, line, f"insecure external link {href!r}")
        elif split.scheme in {"mailto", "tel"}:
            problem.add(page, line, f"contact link {href!r} is not allowed; use GitHub")
        elif split.scheme == "javascript":
            problem.add(page, line, f"javascript: link {href!r}")
        return None
    if href.startswith("//"):
        problem.add(page, line, f"protocol-relative link {href!r}")
        return None
    if not split.path:
        target = page
    else:
        target = (page.parent / unquote(split.path)).resolve()
    try:
        target.relative_to(SITE.resolve())
    except ValueError:
        problem.add(page, line, f"link escapes the site root: {href!r}")
        return None
    if target.is_dir():
        target = target / "index.html"
    return target, split.fragment


def check_file(path: Path, line: int, problem: Problem, kind: str) -> None:
    if not path.is_file():
        problem.add(path, line, f"missing {kind} target")
    else:
        try:
            path.resolve().relative_to(SITE.resolve())
        except ValueError:
            problem.add(path, line, f"{kind} target escapes the site root")


def check_github_repo_link(href: str, page: Path, line: int, problem: Problem) -> None:
    prefix = f"{GH}/"
    if not href.startswith(prefix):
        return
    rest = href[len(prefix) :].split("#", 1)[0].split("?", 1)[0]
    if not (rest.startswith("blob/main/") or rest.startswith("tree/main/")):
        return
    kind, _, raw_path = rest.partition("/main/")
    path = ROOT / unquote(raw_path.rstrip("/"))
    if kind == "blob":
        if not path.is_file():
            problem.add(page, line, f"repository file does not exist: {raw_path}")
    else:
        if not path.is_dir():
            problem.add(page, line, f"repository directory does not exist: {raw_path}")


def check_private_data(path: Path, text: str, problem: Problem) -> None:
    if "/Users/" in text:
        problem.add(path, None, "absolute user path published")
    for match in EMAIL_RE.finditer(text):
        problem.add(path, None, f"email address published: {match.group(0)}")
    for match in IP_RE.finditer(text):
        candidate = match.group(0)
        if candidate in ALLOWED_IPS:
            continue
        if any(candidate.startswith(prefix) for prefix in ALLOWED_IP_PREFIXES):
            continue
        problem.add(path, None, f"non-documentation IP address published: {candidate}")


def check_site() -> Problem:
    problem = Problem()
    pages: dict[Path, PageParser] = {}

    html_files = sorted(SITE.glob("*.html"))
    if not html_files:
        problem.add(SITE, None, "no HTML pages found")
        return problem

    for path in html_files:
        parser = PageParser(path)
        parser.parse()
        pages[path] = parser

        if not parser.lang:
            problem.add(path, None, "missing html lang attribute")
        if not parser.title.strip():
            problem.add(path, parser.title_line or None, "missing title")
        if parser.h1_count != 1:
            problem.add(path, None, f"expected exactly one h1, found {parser.h1_count}")
        if parser.main_count < 1:
            problem.add(path, None, "missing main landmark")
        if not parser.metas.get("description"):
            problem.add(path, parser.meta_lines.get("description"), "missing meta description")
        if not parser.metas.get("viewport"):
            problem.add(path, parser.meta_lines.get("viewport"), "missing viewport meta")
        if not parser.metas.get("og:title") or not parser.metas.get("og:description"):
            problem.add(path, None, "missing Open Graph title or description")
        if not parser.metas.get("og:url") or not parser.metas.get("og:image"):
            problem.add(path, None, "missing Open Graph URL or image")
        if parser.metas.get("twitter:card") != "summary_large_image":
            problem.add(path, None, "twitter:card should be summary_large_image")
        if not parser.metas.get("theme-color") or not parser.metas.get("color-scheme"):
            problem.add(path, None, "missing theme-color or color-scheme meta")

        for ident, count in parser.ids.items():
            if count > 1:
                problem.add(path, None, f"duplicate id {ident!r}")

        for line in parser.style_attrs:
            problem.add(path, line, "inline style attribute (blocked by the site CSP)")
        for line in parser.style_tags:
            problem.add(path, line, "inline <style> block (blocked by the site CSP)")
        for script_type, line in parser.inline_scripts:
            problem.add(path, line, f"inline <script> ({script_type}) is not allowed")

        for href, line in parser.links:
            if href.startswith(f"{GH}/"):
                check_github_repo_link(href, path, line, problem)
            target = local_target(path, href, problem, line)
            if target is None:
                continue
            target_path, fragment = target
            check_file(target_path, line, problem, "link")
            if fragment:
                target_page = pages.get(target_path)
                if target_page is None and target_path.is_file():
                    target_page = PageParser(target_path)
                    target_page.parse()
                    pages[target_path] = target_page
                if target_page is not None and fragment not in target_page.ids:
                    problem.add(path, line, f"missing fragment #{fragment} in {target_path.name}")

        for src, line, kind in parser.assets:
            split = urlsplit(src)
            if split.scheme or src.startswith("//"):
                problem.add(path, line, f"external subresource is not allowed: {kind} {src!r}")
                continue
            target = (path.parent / unquote(split.path)).resolve()
            check_file(target, line, problem, kind)

        for href, line in parser.unsafe_blank:
            problem.add(path, line, f"target=_blank without noopener: {href!r}")

        check_private_data(path, path.read_text(encoding="utf-8"), problem)

    # Titles and descriptions should identify a page, not repeat another.
    seen_titles: dict[str, Path] = {}
    seen_descriptions: dict[str, Path] = {}
    for path, parser in pages.items():
        title = parser.title.strip()
        description = parser.metas.get("description", "").strip()
        if title:
            if title in seen_titles:
                problem.add(path, None, f"duplicate title (also {seen_titles[title].name})")
            seen_titles[title] = path
        if description:
            if description in seen_descriptions:
                problem.add(path, None, f"duplicate description (also {seen_descriptions[description].name})")
            seen_descriptions[description] = path

    # Canonicals and the sitemap.
    indexable = [path for path in pages if path.name != "404.html"]
    for path, parser in pages.items():
        # The parser records link rel=canonical as an asset only when rel is in
        # ASSET_RELS; canonical is not. Read it from the raw head instead.
        match = re.search(
            r'<link\s+rel="canonical"\s+href="([^"]+)"', path.read_text(encoding="utf-8")
        )
        canonical = match.group(1) if match else ""
        if path.name == "404.html":
            if canonical:
                problem.add(path, None, "404 page must not declare a canonical")
            if "noindex" not in parser.metas.get("robots", ""):
                problem.add(path, None, "404 page must be noindex")
            continue
        expected = BASE if path.name == "index.html" else BASE + path.name
        if canonical != expected:
            problem.add(path, None, f"canonical should be {expected!r}, found {canonical!r}")
        og_url = parser.metas.get("og:url", "")
        if og_url != expected:
            problem.add(path, None, f"og:url should be {expected!r}, found {og_url!r}")

    sitemap_path = SITE / "sitemap.xml"
    if not sitemap_path.is_file():
        problem.add(sitemap_path, None, "missing sitemap.xml")
    else:
        try:
            tree = ET.parse(sitemap_path)
            ns = {"sm": "http://www.sitemaps.org/schemas/sitemap/0.9"}
            locs = [node.text.strip() for node in tree.getroot().findall(".//sm:loc", ns) if node.text]
        except ET.ParseError as error:
            problem.add(sitemap_path, None, f"invalid XML: {error}")
            locs = []
        listed = set()
        for loc in locs:
            if not loc.startswith(BASE_HOST):
                problem.add(sitemap_path, None, f"URL outside the canonical base: {loc}")
                continue
            rel = loc[len(BASE_HOST) :]
            target = SITE / ("index.html" if rel in {"", "index.html"} else rel)
            if not target.is_file():
                problem.add(sitemap_path, None, f"sitemap URL has no page: {loc}")
            listed.add(target.resolve())
        for path in indexable:
            if path.resolve() not in listed:
                problem.add(path, None, "indexable page is missing from sitemap.xml")
        if (SITE / "404.html").resolve() in listed:
            problem.add(sitemap_path, None, "404 page must not be in the sitemap")

    # Shared chrome.
    index = pages[SITE / "index.html"]
    primary = next((nav for nav in index.navs if nav["label"] == "Primary"), None)
    legal = next((nav for nav in index.navs if nav["label"] == "Legal"), None)
    if primary is None:
        problem.add(SITE / "index.html", None, "missing primary navigation")
    if legal is None:
        problem.add(SITE / "index.html", None, "missing footer legal navigation")
    for path, parser in pages.items():
        page_primary = next((nav for nav in parser.navs if nav["label"] == "Primary"), None)
        if primary and page_primary and page_primary["links"] != primary["links"]:
            problem.add(path, None, "primary navigation differs from index.html")
        page_legal = next((nav for nav in parser.navs if nav["label"] == "Legal"), None)
        if legal and page_legal and page_legal["links"] != legal["links"]:
            problem.add(path, None, "footer legal navigation differs from index.html")

    # Design-token parity with the application.
    if not TOKENS.is_file():
        problem.add(TOKENS, None, "application token file is missing")
    else:
        app_css = TOKENS.read_text(encoding="utf-8")
        site_css = (SITE / "css" / "style.css").read_text(encoding="utf-8")
        app_dark = css_tokens(css_block(app_css, "@theme", problem, TOKENS))
        app_light = css_tokens(css_block(app_css, "[data-theme='light']", problem, TOKENS))
        site_dark = css_tokens(css_block(site_css, ":root", problem, SITE / "css" / "style.css"))
        site_light = css_tokens(
            css_block(site_css, '[data-theme="light"]', problem, SITE / "css" / "style.css")
        )
        for name, value in sorted(app_dark.items()):
            if name not in site_dark:
                problem.add(SITE / "css" / "style.css", None, f"design token {name} is missing")
            elif site_dark[name] != value:
                problem.add(
                    SITE / "css" / "style.css",
                    None,
                    f"design token {name} drifted: {site_dark[name]!r} != {value!r}",
                )
        for name, value in sorted(app_light.items()):
            if name not in site_light:
                problem.add(SITE / "css" / "style.css", None, f"light token {name} is missing")
            elif site_light[name] != value:
                problem.add(
                    SITE / "css" / "style.css",
                    None,
                    f"light token {name} drifted: {site_light[name]!r} != {value!r}",
                )

        for match in URL_RE.finditer(site_css):
            url = match.group(1).strip()
            if url.startswith(("data:", "#")):
                continue
            if url.startswith(("http://", "https://", "//")):
                problem.add(SITE / "css" / "style.css", None, f"external CSS resource: {url}")
                continue
            target = (SITE / "css" / url).resolve()
            if not target.is_file():
                problem.add(SITE / "css" / "style.css", None, f"missing CSS resource: {url}")

    # Required static-host and discovery files.
    required = [
        SITE / "robots.txt",
        SITE / "sitemap.xml",
        SITE / "llms.txt",
        SITE / ".well-known" / "security.txt",
        SITE / "_headers",
        SITE / ".nojekyll",
        SITE / "assets" / "favicon.svg",
        SITE / "assets" / "og.png",
        SITE / "assets" / "icon-512.png",
        SITE / "assets" / "apple-touch-icon.png",
    ]
    for path in required:
        if not path.is_file():
            problem.add(path, None, "required site file is missing")

    headers_path = SITE / "_headers"
    if headers_path.is_file():
        headers = headers_path.read_text(encoding="utf-8")
        for needle in (
            "Content-Security-Policy",
            "default-src 'none'",
            "script-src 'self'",
            "style-src 'self'",
            "frame-ancestors 'none'",
            "Strict-Transport-Security",
        ):
            if needle not in headers:
                problem.add(headers_path, None, f"header policy missing {needle!r}")

    robots = SITE / "robots.txt"
    if robots.is_file() and f"Sitemap: {BASE}sitemap.xml" not in robots.read_text(encoding="utf-8"):
        problem.add(robots, None, "robots.txt does not point at the canonical sitemap")

    security = SITE / ".well-known" / "security.txt"
    if security.is_file():
        text = security.read_text(encoding="utf-8")
        if "Contact: https://github.com/awbuilds/drovik/security/advisories/new" not in text:
            problem.add(security, None, "security.txt does not use the private advisory contact")
        if "Expires:" not in text:
            problem.add(security, None, "security.txt is missing Expires")

    # PNG dimensions are part of the social/asset contract.
    expected_png = {
        SITE / "assets" / "og.png": (1200, 630),
        SITE / "assets" / "icon-512.png": (512, 512),
        SITE / "assets" / "apple-touch-icon.png": (180, 180),
    }
    for path, expected in expected_png.items():
        if not path.is_file():
            continue
        header = path.read_bytes()[:24]
        if len(header) < 24 or header[:8] != b"\x89PNG\r\n\x1a\n":
            problem.add(path, None, "not a PNG")
            continue
        width, height = struct.unpack(">II", header[16:24])
        if (width, height) != expected:
            problem.add(path, None, f"expected {expected[0]}x{expected[1]}, found {width}x{height}")

    # A compact sanity check for the docs index and legal surface.
    for path in (SITE / "docs.html", SITE / "privacy.html", SITE / "terms.html"):
        if path.is_file() and len(path.read_text(encoding="utf-8")) < 4000:
            problem.add(path, None, "page looks unexpectedly small")

    return problem


def main() -> int:
    problem = check_site()
    if problem.errors:
        print(f"site check failed with {len(problem.errors)} problem(s):\n", file=sys.stderr)
        for error in problem.errors:
            print(f"  - {error}", file=sys.stderr)
        return 1
    print("site check passed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
