#!/usr/bin/env python3
"""Tests of the look of the docs site: template, style, theme switch and fork notice (story #3621, task #3637).
They build a scratch repository holding a copy of the REAL `site/` assets, so a missing asset is an assertion failure.
Offline, standard library only. Run: python scripts/test_site_look.py
"""
import base64
import functools
import json
import re
import shutil
import sys
import tempfile
import unittest
from html.parser import HTMLParser
from pathlib import Path
from urllib.parse import unquote

sys.path.insert(0, str(Path(__file__).resolve().parent))
import build_site as bs  # noqa: E402
from test_build_site import ROOT, write_files  # noqa: E402
from test_site_i18n import translation, write_ui  # noqa: E402

BASE_URL = "https://venom.pluries.dev"
REPO_URL = "https://github.com/pluries/venom"
UPSTREAM_URL = "https://github.com/ovh/venom"
REQUIRED_ASSETS = ("template.html", "home.html", "style.css", "js/site.js", "js/theme.js", "fonts/LICENSES.md")
FORBIDDEN_WORDS = ("craticula", "ag grid", "odata", "nuget")
VOID = {"meta", "link", "br", "img", "hr", "input", "source", "area", "base", "col", "embed", "wbr", "track", "param"}
LONG_TITLE = 'Long <title> & "quoted" ' + "x" * 270


def missing_assets() -> list[str]:
    """What the real `site/` folder lacks of the files the look needs (empty when complete)."""
    site = ROOT / "site"
    missing = [f"site/{name}" for name in REQUIRED_ASSETS if not (site / name).is_file()]
    if not list((site / "fonts").glob("*.woff2")):
        missing.append("site/fonts/*.woff2")
    return missing


def scratch_site(pages: list[dict], sources: dict[str, str], extra_groups: dict[str, str] | None = None,
                 languages: dict | None = None) -> tuple[Path, Path]:
    """A scratch repository: a copy of the real site assets, a site.json written here, the page sources; built into a temp folder.
    With `languages`, those languages are built as well; their interface text is the English one with a marker in front of each string."""
    root = Path(tempfile.mkdtemp())
    shutil.copytree(ROOT / "site", root / "site", ignore=shutil.ignore_patterns("pages", "examples", "_build", "site.json", "fr", "de", "it", "es", "zh"))
    groups = {"Guides": "How-to guides.", "Concepts": "How venom works.", **(extra_groups or {})}
    config = {"title": "Pluries Venom", "repo": REPO_URL, "branch": "main", "baseUrl": BASE_URL, "groups": groups, "pages": pages}
    if languages:
        config["languages"] = languages
        write_ui(root, languages)
    write_files(root, {"site/site.json": json.dumps(config), **sources})
    out = root / "_out"
    errors = bs.build(root, out)
    if errors:
        raise AssertionError(f"the scratch build failed: {errors}")
    return root, out


def page(slug: str, title: str, group: str | None = None, **extra) -> dict:
    entry = {"slug": slug, "title": title, "source": f"{slug}.md", "description": f"About {title}.", **extra}
    if group:
        entry["group"] = group
    return entry


STANDARD_PAGES = [page("index", "Home", template="site/home.html"), page("alpha", "Alpha", "Guides"), page("beta", "Beta", "Concepts")]
STANDARD_SOURCES = {"index.md": "# Home\n\nHome text.\n", "alpha.md": "# Alpha\n\nAlpha body text.\n", "beta.md": "# Beta\n\nBeta body text.\n"}


FR_LANGUAGES = {"en": {"name": "English", "path": "", "lang": "en"}, "fr": {"name": "Français", "path": "fr", "lang": "fr", "enabled": True}}
FR_SOURCES = {"site/i18n/fr/alpha.md": translation("alpha", "Alpha FR", "# Alpha FR {#alpha}\n\nCorps de la page alpha.\n")}


@functools.lru_cache(maxsize=None)
def _standard_cached() -> tuple[Path, Path]:
    """The English site and, next to it, French with one translated page (alpha) and two pages served as English (index, beta)."""
    return scratch_site(STANDARD_PAGES, {**STANDARD_SOURCES, **FR_SOURCES}, languages=FR_LANGUAGES)


class LookCase(unittest.TestCase):
    def standard(self) -> Path:
        """The output folder of the standard scratch build; fails (not errors) when the real assets are missing."""
        missing = missing_assets()
        if missing:
            self.fail("; ".join(f"{name} does not exist" for name in missing))
        return _standard_cached()[1]

    def html_pages(self) -> dict[str, str]:
        out = self.standard()
        pages = {p.name: p.read_text(encoding="utf-8") for p in sorted(out.glob("*.html"))}
        self.assertEqual(set(pages), {"index.html", "alpha.html", "beta.html"})
        return pages

    def language_pages(self) -> dict[str, str]:
        """The pages of the French folder of the standard build, by their path under the output folder."""
        out = self.standard()
        pages = {f"fr/{p.name}": p.read_text(encoding="utf-8") for p in sorted((out / "fr").glob("*.html"))}
        self.assertEqual(set(pages), {"fr/index.html", "fr/alpha.html", "fr/beta.html"})
        return pages

    def french_ui(self) -> dict:
        return json.loads((self.standard().parent / "site" / "i18n" / "fr" / "ui.json").read_text(encoding="utf-8"))


# ---------- text helpers ----------

class Anchors(HTMLParser):
    """The links inside the first <footer> and the plain text of it."""

    def __init__(self) -> None:
        super().__init__()
        self.in_footer, self.hrefs, self.text = False, [], []

    def handle_starttag(self, tag, attrs):
        if tag == "footer":
            self.in_footer = True
        if self.in_footer and tag == "a":
            self.hrefs.append(dict(attrs).get("href", ""))

    def handle_endtag(self, tag):
        if tag == "footer":
            self.in_footer = False

    def handle_data(self, data):
        if self.in_footer:
            self.text.append(data)


def footer_of(page_html: str) -> tuple[list[str], str]:
    parser = Anchors()
    parser.feed(page_html)
    return parser.hrefs, re.sub(r"\s+", " ", "".join(parser.text))


class Footer(LookCase):
    def test_every_page_carries_the_fork_notice(self):
        for name, text in self.html_pages().items():
            _, words = footer_of(text)
            with self.subTest(page=name):
                self.assertIn("Pluries Venom is an independent fork of ovh/venom, published under the Apache-2.0 licence", words)
                self.assertIn("Apache-2.0", words)

    def test_every_page_links_upstream_the_licence_and_the_fork(self):
        for name, text in self.html_pages().items():
            hrefs, _ = footer_of(text)
            with self.subTest(page=name):
                self.assertIn(UPSTREAM_URL, hrefs)
                self.assertTrue(any(h.endswith("/blob/main/LICENSE") or h.endswith("/LICENSE") for h in hrefs), hrefs)
                self.assertIn(REPO_URL, hrefs)

    def test_no_page_outside_its_footer_names_the_project_it_was_copied_from(self):
        for name, text in {**self.html_pages(), **self.language_pages()}.items():
            body = re.sub(r"<footer\b.*?</footer>", "", text, flags=re.S | re.I).lower()
            for word in FORBIDDEN_WORDS:
                with self.subTest(page=name, word=word):
                    self.assertNotIn(word, body)

    def test_us1_as4_every_page_of_a_language_carries_the_fork_notice_in_that_language(self):
        notice = self.french_ui()["footer_notice"]
        self.assertNotEqual(notice, "Pluries Venom is an independent fork of ovh/venom, published under the Apache-2.0 licence.")
        for name, text in self.language_pages().items():
            _, words = footer_of(text)
            with self.subTest(page=name):
                self.assertIn(notice, words)
                self.assertIn("Apache-2.0", words)

    def test_us1_as4_english_pages_keep_the_english_sentence_next_to_a_translated_site(self):
        self.assertEqual(self.french_ui()["footer_notice"], "[fr] Pluries Venom is an independent fork of ovh/venom, published under the Apache-2.0 licence.")
        for name, text in self.html_pages().items():
            with self.subTest(page=name):
                self.assertIn("Pluries Venom is an independent fork of ovh/venom, published under the Apache-2.0 licence.", footer_of(text)[1])
                self.assertNotIn("[fr]", footer_of(text)[1])

    def test_us1_as4_every_page_of_a_language_links_upstream_the_licence_and_the_fork(self):
        for name, text in self.language_pages().items():
            hrefs, _ = footer_of(text)
            with self.subTest(page=name):
                self.assertIn(UPSTREAM_URL, hrefs)
                self.assertTrue(any(h.endswith("/LICENSE") for h in hrefs), hrefs)
                self.assertIn(REPO_URL, hrefs)


# ---------- T3: placeholders ----------

class Placeholders(LookCase):
    def test_no_page_holds_an_unresolved_placeholder(self):
        for name, text in {**self.html_pages(), **self.language_pages()}.items():
            with self.subTest(page=name):
                self.assertEqual(re.findall(r"\{\{\s*[\w.]+\s*\}\}", text), [])


# ---------- T4: hosts ----------

URL_TEXT = re.compile(r"(?:https?:)?//[^\s\"'<>)\\]+", re.I)
CSS_URL = re.compile(r"url\(\s*(['\"]?)(.*?)\1\s*\)", re.I | re.S)
CSS_IMPORT = re.compile(r"@import\s+(?:url\(\s*)?['\"]?([^'\")\s;]+)", re.I)


def is_remote(value: str) -> bool:
    value = value.strip()
    return bool(re.match(r"^(?:[a-zA-Z][a-zA-Z0-9+.-]*:|//)", value)) and not value.lower().startswith("mailto:")


DATA_IMAGE = re.compile(r"^data:image/[a-z0-9.+-]+\s*(;[^,]*)?,", re.I)
SVG_PAYLOAD_FORBIDDEN = re.compile(r"<\s*script|<\s*foreignObject|xlink:href|\bhref\s*=|\bon\w+\s*=|javascript:|(?:https?:)?//", re.I)


def css_address_is_remote(value: str) -> bool:
    """A CSS url() address that is not the site's own: an inline image data URI makes no request, unless its payload holds a script or a link."""
    value = value.strip()
    if not DATA_IMAGE.match(value):
        return is_remote(value) or value.startswith("//")
    head, _, payload = value.partition(",")
    payload = base64.b64decode(payload + "=" * (-len(payload) % 4), validate=False).decode("utf-8", "replace") \
        if ";base64" in head.lower() else unquote(payload)
    payload = re.sub(r"""xmlns(?::\w+)?\s*=\s*(["'])[^"']*\1""", "", payload)
    return bool(SVG_PAYLOAD_FORBIDDEN.search(payload))


def allowed_link(value: str) -> bool:
    """A remote address a link may point to: the fork, its upstream, or the site itself."""
    return any(value == base or value.startswith(base + "/") or value.startswith(base + "#") or value.startswith(base + "?")
               for base in (REPO_URL, UPSTREAM_URL, BASE_URL))


class References(HTMLParser):
    def __init__(self) -> None:
        super().__init__()
        self.found: list[tuple[str, str, str]] = []  # (tag, attribute, value)

    def handle_starttag(self, tag, attrs):
        values = dict(attrs)
        for attribute in ("href", "src", "action", "data", "poster", "formaction"):
            if values.get(attribute) is not None:
                self.found.append((tag, attribute, values[attribute]))
        for candidate in (values.get("srcset") or "").split(","):
            if candidate.strip():
                self.found.append((tag, "srcset", candidate.split()[0]))
        if tag == "meta" and (values.get("http-equiv") or "").lower() == "refresh":
            self.found.append((tag, "refresh", re.sub(r"^\s*\d+\s*;\s*(?:url=)?", "", values.get("content") or "", flags=re.I)))
        if tag == "iframe":
            self.found.append((tag, "iframe", values.get("src", "")))
        if values.get("style"):
            self.found.extend((tag, "style", m.group(2)) for m in CSS_URL.finditer(values["style"]))


class Hosts(LookCase):
    def output_files(self) -> dict[str, str]:
        out = self.standard()
        return {str(p.relative_to(out)): p.read_text(encoding="utf-8") for p in sorted(out.rglob("*"))
                if p.is_file() and p.suffix in (".html", ".css", ".js", ".svg")}

    def test_every_address_in_every_text_file_is_the_fork_its_upstream_or_the_site(self):
        for name, text in self.output_files().items():
            text = re.sub(r"""xmlns(?::\w+)?\s*=\s*(["'])[^"']*\1""", "", text)
            for url in URL_TEXT.findall(text):
                with self.subTest(file=name, url=url):
                    self.assertTrue(allowed_link(url.rstrip(".,;")), "address on another host")

    def test_only_links_point_to_a_remote_address_and_never_to_another_host(self):
        for name, text in self.html_pages().items():
            parser = References()
            parser.feed(text)
            for tag, attribute, value in parser.found:
                if not is_remote(value):
                    continue
                with self.subTest(page=name, tag=tag, attribute=attribute, value=value):
                    self.assertNotIn(tag, ("iframe", "form", "object", "embed"))
                    self.assertTrue(allowed_link(value), "remote address on another host")
                    if allowed_link(value) and not value.startswith(BASE_URL):
                        self.assertEqual((tag, attribute), ("a", "href"), "the GitHub addresses are links, never loaded resources")

    def test_the_stylesheet_loads_nothing_remote_and_its_fonts_are_the_sites_own_files(self):
        out = self.standard()
        css = (out / "style.css").read_text(encoding="utf-8")
        addresses = [m.group(2) for m in CSS_URL.finditer(css)] + CSS_IMPORT.findall(css)
        self.assertTrue(any(a.endswith(".woff2") for a in addresses), "the stylesheet declares self-hosted fonts")
        for address in addresses:
            with self.subTest(address=address):
                self.assertFalse(css_address_is_remote(address))
                if not address.startswith("data:"):
                    self.assertTrue((out / address.partition("?")[0].partition("#")[0]).is_file(), "no such file in the output")

    def test_the_font_files_and_their_licences_are_published(self):
        out = self.standard()
        self.assertTrue(list((out / "fonts").glob("*.woff2")))
        self.assertTrue((out / "fonts" / "LICENSES.md").is_file())


class CssAddressIsRemote(unittest.TestCase):
    SVG = "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='8' height='8'%3E%3Cpath d='M0 0h8v8z'/%3E%3C/svg%3E"

    def test_an_inline_svg_image_is_not_a_request(self):
        self.assertFalse(css_address_is_remote(self.SVG))

    def test_an_inline_base64_svg_image_is_not_a_request(self):
        payload = base64.b64encode(b"<svg xmlns='http://www.w3.org/2000/svg'><path d='M0 0'/></svg>").decode()
        self.assertFalse(css_address_is_remote("data:image/svg+xml;base64," + payload))

    def test_a_site_file_is_not_remote(self):
        self.assertFalse(css_address_is_remote("fonts/a.woff2"))

    def test_every_network_or_other_scheme_is_remote(self):
        for address in ("https://example.org/a.png", "http://example.org/a.png", "//example.org/a.png",
                        "ftp://example.org/a.png", "javascript:alert(1)", "blob:abc", "file:///etc/x"):
            with self.subTest(address=address):
                self.assertTrue(css_address_is_remote(address))

    def test_a_data_address_of_another_type_is_remote(self):
        for address in ("data:text/html,%3Ch1%3Ex%3C/h1%3E", "data:application/javascript,alert(1)", "data:,plain"):
            with self.subTest(address=address):
                self.assertTrue(css_address_is_remote(address))

    def test_an_svg_payload_with_a_script_or_a_link_is_remote(self):
        for payload in ("%3Csvg%3E%3Cscript%3Ealert(1)%3C/script%3E%3C/svg%3E",
                        "%3Csvg%3E%3Ca href='https://example.org'/%3E%3C/svg%3E",
                        "%3Csvg%3E%3Cuse xlink:href='x.svg%23a'/%3E%3C/svg%3E",
                        "%3Csvg onload='x()'%3E%3C/svg%3E"):
            with self.subTest(payload=payload):
                self.assertTrue(css_address_is_remote("data:image/svg+xml," + payload))

    def test_a_base64_svg_payload_with_a_script_is_remote(self):
        payload = base64.b64encode(b"<svg><script>alert(1)</script></svg>").decode()
        self.assertTrue(css_address_is_remote("data:image/svg+xml;base64," + payload))


# ---------- theme switch and scripts ----------

class ThemeSwitch(LookCase):
    def test_scripts_are_referenced_by_relative_urls_and_the_theme_script_is_published(self):
        out = self.standard()
        for name, text in self.html_pages().items():
            sources = re.findall(r"<script\b[^>]*\bsrc=[\"']([^\"']+)[\"']", text)
            with self.subTest(page=name):
                self.assertIn("theme.js", sources)
                self.assertTrue(all(not is_remote(s) for s in sources), sources)
        self.assertTrue((out / "theme.js").is_file())
        self.assertTrue((out / "site.js").is_file())

    def test_the_theme_script_applies_the_stored_choice_to_data_theme(self):
        script = (self.standard() / "theme.js").read_text(encoding="utf-8")
        self.assertIn("localStorage", script)
        self.assertTrue("dataset.theme" in script or "data-theme" in script)

    def test_the_full_content_is_in_the_html_without_any_script(self):
        pages = self.html_pages()
        self.assertIn("Alpha body text.", pages["alpha.html"])
        self.assertIn("Beta body text.", pages["beta.html"])
        self.assertIn("Home text.", pages["index.html"])
        self.assertIn('<nav aria-label="Pages">', pages["alpha.html"])
        self.assertIn('class="side-group"', pages["alpha.html"])


# ---------- T6: robustness ----------

class Balance(HTMLParser):
    def __init__(self) -> None:
        super().__init__()
        self.stack: list[str] = []
        self.problems: list[str] = []
        self.titles: list[str] = []
        self.in_title = False

    def handle_starttag(self, tag, attrs):
        if tag == "title":
            self.in_title = True
        if tag not in VOID:
            self.stack.append(tag)

    def handle_startendtag(self, tag, attrs):
        pass

    def handle_endtag(self, tag):
        if tag == "title":
            self.in_title = False
        if tag in VOID:
            return
        if not self.stack or self.stack[-1] != tag:
            self.problems.append(f"</{tag}> closes {self.stack[-1] if self.stack else 'nothing'}")
            if tag in self.stack:
                while self.stack.pop() != tag:
                    pass
            return
        self.stack.pop()

    def handle_data(self, data):
        if self.in_title:
            self.titles.append(data)


class Robustness(LookCase):
    def build_stress(self) -> Path:
        missing = missing_assets()
        if missing:
            self.fail("; ".join(f"{name} does not exist" for name in missing))
        pages = [page("index", "Home", template="site/home.html"), page("long", LONG_TITLE, "Guides")]
        pages += [page(f"entry-{n:02d}", f"Entry number {n}", "Guides") for n in range(40)]
        pages += [page("beta", "Beta", "Concepts")]
        sources = {"index.md": "# Home\n\nHome text.\n", "long.md": "# Long\n\nBody.\n", "beta.md": "# Beta\n\nBody.\n",
                   **{f"entry-{n:02d}.md": f"# Entry {n}\n\nBody.\n" for n in range(40)}}
        return scratch_site(pages, sources)[1]

    def test_every_page_has_balanced_tags_with_a_long_title_many_entries_and_a_group_without_pages(self):
        out = self.build_stress()
        built = sorted(out.glob("*.html"))
        self.assertEqual(len(built), 43)
        for path in built:
            parser = Balance()
            parser.feed(path.read_text(encoding="utf-8"))
            parser.close()
            with self.subTest(page=path.name):
                self.assertEqual(parser.problems, [])
                self.assertEqual(parser.stack, [], "tags left open")

    def test_the_long_title_is_escaped_and_whole(self):
        text = (self.build_stress() / "long.html").read_text(encoding="utf-8")
        self.assertIn("Long &lt;title&gt; &amp; &quot;quoted&quot; " + "x" * 270, text.replace("&#x27;", "'"))
        self.assertNotIn("Long <title>", text)
        parser = Balance()
        parser.feed(text)
        self.assertIn(LONG_TITLE, "".join(parser.titles))

    def test_a_group_without_pages_leaves_no_sidebar_block_or_card(self):
        out = self.build_stress()
        text = (out / "long.html").read_text(encoding="utf-8")
        self.assertNotIn("<summary>Reference</summary>", text)
        self.assertNotIn("<h3>Reference</h3>", (out / "index.html").read_text(encoding="utf-8"))
        self.assertEqual(text.count('class="side-group"'), 2)


class LanguageFolders(LookCase):
    """US1-AS4, AS13, FR-023: the tests above walk the language folders too; these are the checks that only apply to a language page."""

    def test_the_output_scan_reaches_the_files_of_the_language_folder(self):
        out = self.standard()
        found = {p.relative_to(out).as_posix() for p in out.rglob("*") if p.is_file() and p.suffix in (".html", ".css", ".js", ".svg")}
        self.assertTrue({"fr/alpha.html", "fr/index.html", "fr/beta.html", "style.css", "site.js"} <= found, sorted(found))

    def test_a_language_page_loads_the_scripts_of_the_root_by_relative_urls(self):
        for name, text in self.language_pages().items():
            sources = re.findall(r"<script\b[^>]*\bsrc=[\"']([^\"']+)[\"']", text)
            with self.subTest(page=name):
                self.assertEqual(sources, ["../theme.js", "../site.js"])
                self.assertTrue(all(not is_remote(s) for s in sources), sources)

    def test_every_remote_reference_of_a_language_page_is_a_link_to_the_fork_its_upstream_or_the_site(self):
        for name, text in self.language_pages().items():
            parser = References()
            parser.feed(text)
            for tag, attribute, value in parser.found:
                if is_remote(value):
                    with self.subTest(page=name, tag=tag, value=value):
                        self.assertTrue(allowed_link(value), "remote address on another host")
                        self.assertTrue(tag in ("a", "link") and attribute == "href", "a remote address is a link, never a loaded resource")

    def test_a_language_page_has_balanced_tags(self):
        for name, text in self.language_pages().items():
            parser = Balance()
            parser.feed(text)
            parser.close()
            with self.subTest(page=name):
                self.assertEqual((parser.problems, parser.stack), ([], []))

    def test_the_text_files_of_the_language_folder_name_no_other_host(self):
        out = self.standard()
        for path in sorted((out / "fr").glob("*")):
            text = re.sub(r"""xmlns(?::\w+)?\s*=\s*(["'])[^"']*\1""", "", path.read_text(encoding="utf-8"))
            for url in URL_TEXT.findall(text):
                with self.subTest(file=path.name, url=url):
                    self.assertTrue(allowed_link(url.rstrip(".,;")), "address on another host")


# ---------- Chinese typography and narrow screens (US1-AS7, AS14, FR-017, FR-021) ----------

CSS_TEXT = (ROOT / "site" / "style.css").read_text(encoding="utf-8")
CJK_FONTS = ("PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", "Noto Sans CJK SC", "Noto Sans SC", "Source Han Sans SC", "WenQuanYi Micro Hei",
             "Heiti SC", "SimHei")
GENERIC_FAMILIES = ("sans-serif", "system-ui", "monospace", "serif")
SITE_FONTS = ("Archivo", "Public Sans", "IBM Plex Mono")
FONT_LIMIT = 100 * 1024


def css_rules(css: str = CSS_TEXT) -> list[tuple[list[str], str]]:
    """(selectors, declarations) of every rule, comments removed; a rule inside @media is a rule."""
    css = re.sub(r"/\*.*?\*/", "", css, flags=re.S)
    return [([part.strip() for part in selector.split(",")], body) for selector, body in re.findall(r"([^{}@]+)\{([^{}]*)\}", css)]


def family_list(body: str) -> list[str]:
    found = re.search(r"font-family:\s*([^;]+)", body)
    return [name.strip().strip("\"'") for name in found.group(1).split(",")] if found else []


class ChineseTypography(unittest.TestCase):
    """US1-AS7, FR-017: the Chinese text is set in a stack of system fonts; nothing is downloaded for it."""

    def zh_rules(self, region: str) -> list[tuple[list[str], str]]:
        return [(selectors, body) for selectors, body in css_rules()
                if any(re.search(r":lang\(\s*zh-Hans\s*\)", s) and re.search(region, s) for s in selectors)]

    def test_us1_as7_the_body_the_headings_and_the_code_of_a_chinese_page_have_a_stack_of_chinese_system_fonts(self):
        for name, region in (("body", r"(?:^|\s)body\b|^html:lang"), ("headings", r"\bh[1-6]\b"), ("code", r"\b(?:pre|code)\b")):
            rules = self.zh_rules(region)
            with self.subTest(region=name):
                self.assertTrue(rules, f"no rule with :lang(zh-Hans) for {name}")
                stack = family_list(rules[-1][1])
                self.assertGreaterEqual(len({f for f in stack if f in CJK_FONTS}), 2, stack)
                self.assertIn(stack[-1], GENERIC_FAMILIES, stack)

    def test_us1_as7_the_stack_covers_windows_macos_android_and_linux(self):
        stack = family_list(self.zh_rules(r"(?:^|\s)body\b|^html:lang")[-1][1]) if self.zh_rules(r"(?:^|\s)body\b|^html:lang") else []
        for system, names in (("Windows", ("Microsoft YaHei",)), ("macOS and iOS", ("PingFang SC",)), ("Android and Linux", ("Noto Sans CJK SC", "Noto Sans SC"))):
            with self.subTest(system=system):
                self.assertTrue(any(name in stack for name in names), stack)

    def test_us1_as7_the_sites_own_fonts_stay_first_so_that_latin_text_keeps_its_look(self):
        stack = family_list(self.zh_rules(r"(?:^|\s)body\b|^html:lang")[-1][1]) if self.zh_rules(r"(?:^|\s)body\b|^html:lang") else []
        self.assertTrue(stack and stack[0] in SITE_FONTS, stack)

    def test_us1_as7_the_stylesheet_has_no_remote_font_address_and_no_import(self):
        blocks = re.findall(r"@font-face\s*\{[^}]*\}", CSS_TEXT)
        self.assertTrue(blocks)
        for block in blocks:
            with self.subTest(block=block[:60]):
                self.assertIn(family_list(block)[0], SITE_FONTS)
                for address in re.findall(r"url\(\s*['\"]?([^)'\"]+)", block):
                    self.assertTrue(address.startswith("fonts/") and address.endswith(".woff2"), address)
                self.assertNotRegex(block, r"(?i)unicode-range:[^;]*(?:4E00|9FFF|3000|FF00)")
        self.assertNotRegex(CSS_TEXT, r"(?i)@import")
        self.assertNotRegex(CSS_TEXT.replace("http://www.w3.org/2000/svg", ""), r"https?://|(?<![\w:])//[\w.-]+\.[a-z]")

    def test_us1_as7_no_font_file_over_100_kb_and_none_for_chinese_is_published(self):
        fonts = [p for p in (ROOT / "site" / "fonts").iterdir() if p.is_file() and p.suffix != ".md"]
        self.assertTrue(fonts)
        for path in fonts:
            with self.subTest(font=path.name):
                self.assertLessEqual(path.stat().st_size, FONT_LIMIT)
                self.assertNotRegex(path.name.lower(), r"cjk|hans|han-sans|-sc-|noto-sans-sc|source-han|chinese")

    def test_us1_as7_a_built_site_publishes_the_same_small_fonts(self):
        out = _standard_cached()[1]
        for path in (out / "fonts").glob("*.woff2"):
            with self.subTest(font=path.name):
                self.assertLessEqual(path.stat().st_size, FONT_LIMIT)
        self.assertFalse(list((out / "fr").glob("*.woff2")))


class NarrowWidthWrapping(unittest.TestCase):
    """US1-AS14, FR-021: long German words and unspaced Chinese lines wrap instead of widening the page at 320 px. The browser
    measurement is the venom web suite `tests/site/site-languages.yml` run by the maker; this guards the rules it depends on."""
    WRAP = re.compile(r"overflow-wrap:\s*(?:anywhere|break-word)|word-break:\s*(?:break-word|break-all)")
    WIDE = ("html", "body", "*", ":root")
    NEEDS_WRAP = (".prose", ".side li a", ".toc a", ".card", ".fact", ".band h2", ".hero h1")
    NEVER_NOWRAP = (".prose", ".side", ".toc", ".lang-menu", ".lang-dropdown", ".card", ".pager", ".fact", ".top-nav", ".fallback-notice")

    def wrapping(self) -> set[str]:
        return {s for selectors, body in css_rules() if self.WRAP.search(body) for s in selectors}

    def test_d_028_no_rule_of_the_stylesheet_styles_a_draft_banner_and_the_other_two_notices_keep_theirs(self):
        selectors = {s for rule_selectors, _ in css_rules() for s in rule_selectors}
        self.assertEqual([s for s in selectors if "draft-banner" in s], [])
        self.assertNotIn("draft-banner", CSS_TEXT)
        for notice in (".fallback-notice", ".override-notice"):
            with self.subTest(notice=notice):
                self.assertTrue(any(notice in s for s in selectors), f"no rule for {notice}")

    def test_us1_as14_every_block_of_text_wraps_a_long_word_itself_or_through_the_page(self):
        wrapping = self.wrapping()
        for fragment in self.NEEDS_WRAP:
            with self.subTest(selector=fragment):
                self.assertTrue(any(fragment in s for s in wrapping) or any(s in self.WIDE for s in wrapping),
                                f"no rule lets {fragment} break a long word")

    @staticmethod
    def language_rules() -> list[tuple[list[str], str]]:
        return [(selectors, body) for selectors, body in css_rules() if any(re.search(r"\.lang-(?:menu|dropdown)\b", s) for s in selectors)]

    def test_us1_as14_the_language_panel_wraps_long_names_and_never_exceeds_the_viewport(self):
        panels = [body for _, body in self.language_rules() if re.search(r"position:\s*absolute", body)]
        self.assertTrue(panels, "no absolutely positioned rule for the language dropdown")
        self.assertTrue(any(self.WRAP.search(body) and re.search(r"max-width:\s*calc\(100vw\s*-\s*32px\)", body) for body in panels), panels)

    def test_us1_as1_the_panel_opens_under_the_summary_right_aligned_in_the_colour_tokens(self):
        rules = self.language_rules()
        self.assertTrue(any(re.search(r"position:\s*relative", body) and any(re.search(r"\.lang-(?:menu|dropdown)$", s) for s in selectors)
                            for selectors, body in rules), "the panel needs a positioned parent")
        panels = [body for _, body in rules if re.search(r"position:\s*absolute", body)]
        self.assertTrue(any(re.search(r"(?<![\w-])right:\s*0", body) and "var(--" in body for body in panels), panels)

    def test_us1_as1_the_menu_is_no_longer_a_full_width_row_under_the_top_bar(self):
        own = [body for selectors, body in css_rules() if any(re.fullmatch(r"\.lang-menu", s) for s in selectors)]
        for body in own:
            with self.subTest(rule=body):
                self.assertNotRegex(body, r"border-bottom")
                self.assertNotIn("max-width: 1320px", body)

    def test_us1_as1_the_current_language_stays_marked_in_the_panel(self):
        self.assertTrue(any("[aria-current" in s for selectors, _ in self.language_rules() for s in selectors))

    def test_us1_as14_no_text_block_is_kept_on_one_line(self):
        for selectors, body in css_rules():
            if re.search(r"white-space:\s*nowrap", body) and any(
                    s.startswith(fragment) and not re.search(r"\b(?:th|td|table|pre|code)\b", s) for s in selectors for fragment in self.NEVER_NOWRAP):
                self.fail(f"{selectors} sets white-space: nowrap")

    def test_us1_as14_wide_content_scrolls_inside_its_own_box_and_the_column_can_shrink(self):
        pre = [body for selectors, body in css_rules() if "pre" in selectors]
        self.assertTrue(any("overflow-x: auto" in body for body in pre), pre)
        self.assertTrue(any("overflow-x: auto" in body for selectors, body in css_rules() if "table" in selectors))
        self.assertTrue(any("min-width: 0" in body for selectors, body in css_rules() if ".prose" in selectors))

    def test_us1_as14_the_page_scales_to_the_width_of_the_screen(self):
        for name in ("template.html", "home.html"):
            self.assertIn('width=device-width, initial-scale=1', (ROOT / "site" / name).read_text(encoding="utf-8"))


class ContentImages(unittest.TestCase):
    """The demo animation of the overview page was wider than the column and ran off the page (2026-11-16)."""

    def test_an_image_in_the_content_never_exceeds_its_column(self):
        css = (ROOT / "site" / "style.css").read_text(encoding="utf-8")
        rules = re.findall(r"([^{}]+)\{([^{}]*)\}", css)
        hits = [(sel, body) for sel, body in rules
                if re.search(r"(^|[\s,])(\.prose\s+)?img", sel) and "max-width: 100%" in body.replace("max-width:100%", "max-width: 100%")
                and "height: auto" in body.replace("height:auto", "height: auto")]
        self.assertTrue(hits, "no rule gives content images max-width: 100% and height: auto")


class HomeSellingPoints(unittest.TestCase):
    """The first page sells what this fork adds to the original (change request of 2026-11-16, spec 010 D-020)."""

    @classmethod
    def setUpClass(cls):
        cls.home = (ROOT / "site" / "home.html").read_text(encoding="utf-8")
        match = re.search(r'<section class="band wrap-wide why">(.*?)</section>', cls.home, flags=re.S)
        cls.why = match.group(1) if match else ""

    def test_the_home_page_has_a_why_this_fork_section_before_the_navigation_cards(self):
        self.assertTrue(self.why, "no <section class=\"band wrap-wide why\"> on the home page")
        self.assertLess(self.home.index('class="band wrap-wide why"'), self.home.index("{{group_cards}}"))

    def test_the_section_names_the_five_gains(self):
        text = re.sub(r"<[^>]+>", " ", self.why).lower()
        for needle in ("maintained", "upgraded", "new executors", "faster", "signed"):
            with self.subTest(needle=needle):
                self.assertIn(needle, text)

    def test_only_measured_figures_are_claimed(self):
        text = re.sub(r"<[^>]+>", " ", self.why)
        figures = set(re.findall(r"\d+(?:\.\d+)?\s?[x×]", text))
        self.assertTrue(figures <= {"13 x", "13x", "13×", "3.4×", "3.4x", "5.6×", "5.6x"}, figures)

    def test_the_new_executors_named_exist_in_the_repository(self):
        for name in ("mcp", "mobile"):
            with self.subTest(name=name):
                self.assertIn(name, re.sub(r"<[^>]+>", " ", self.why).lower())
                self.assertTrue((ROOT / "executors" / name).is_dir())

    def test_the_old_facts_strip_is_replaced_by_the_section_not_duplicated(self):
        self.assertEqual(self.home.count('class="facts"'), 1)


if __name__ == "__main__":
    unittest.main()
