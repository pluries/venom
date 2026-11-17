#!/usr/bin/env python3
"""Tests of the language mechanism of the docs site (spec 013-site-languages): the English build pinned against a golden copy
(T004), the translation file and its fingerprint (T007), the stamp command (T008), a translated page of the README (T009) and
the assets prefix of a language page (T010), and the visitor's side of User Story 1 (T017 to T020, T023, T025): the language control,
the interface text, the head and the fallback page, no language choice, every address, the preview option. Each test names the
scenario it proves (US1-AS10, US2-AS1, ...). Offline,
standard library only. Run: python scripts/test_site_i18n.py
"""
import atexit
import contextlib
import functools
import hashlib
import html
import io
import json
import re
import shutil
import sys
import tempfile
import unittest
from html.parser import HTMLParser
from pathlib import Path
from urllib.parse import urljoin, urlsplit

sys.path.insert(0, str(Path(__file__).resolve().parent))
import build_site as bs  # noqa: E402
import site_i18n as i18n  # noqa: E402
from test_build_site import ROOT, make_repo, write_files  # noqa: E402

GOLDEN = ROOT / "docs" / "specs" / "013-site-languages" / "golden" / "english-build.json"
LANGUAGES = {"en": {"name": "English", "path": "", "lang": "en"},
             "fr": {"name": "Français", "path": "fr", "lang": "fr", "enabled": True}}
DISABLED_FR = {"en": LANGUAGES["en"], "fr": {**LANGUAGES["fr"], "enabled": False}}
LANGUAGE_FOLDERS = ("fr", "de", "it", "es", "zh")

README = """# Overview

Intro text of the overview.

## Setup steps

Run the setup. See [installing it](#installing-it).

# Installing

## Installing it

Install the thing.

# Skipped

Not a page.
"""
SOURCES = {
    "README.md": README,
    "site/pages/index.md": "# Home\n\nHome text.\n",
    "site/pages/guide.md": "# Guide\n\nGuide text. ![pic](pic.png)\n\n## Local part\n\nSee [the other page](other.md#remote-part).\n",
    "site/pages/other.md": "# Other\n\n## Remote part\n\nOther text.\n",
    "site/pages/pic.png": "not a real image, only a file that exists",
}
PAGES = [
    {"slug": "index", "title": "Home", "source": "site/pages/index.md", "template": "site/home.html", "description": "The home page."},
    {"generator": "readme-split", "source": "README.md", "skip": ["Skipped"], "sections": [
        {"heading": "Overview", "slug": "overview", "group": "Getting started", "description": "What it is."},
        {"heading": "Installing", "slug": "installing", "group": "Getting started", "description": "How to install it."}]},
    {"slug": "guide", "title": "Guide", "source": "site/pages/guide.md", "group": "Guides", "description": "A guide."},
    {"slug": "other", "title": "Other", "source": "site/pages/other.md", "group": "Guides", "description": "Another guide."},
]
GROUPS = {"Getting started": "Install it.", "Guides": "How to use it."}


def translation(slug: str, title: str, body: str, status: str = "machine-drafted") -> str:
    """A well-formed translation file: the header of data-model.md, then the page."""
    return (f"---\ntranslates: {slug}\ntitle: {title}\ndescription: Description de {title}.\nfingerprint: not-checked-by-the-build\n"
            f"drafted_by: claude\ndrafted_on: 2026-11-17\nstatus: {status}\n---\n{body}")


FR_OVERVIEW = translation("overview", "Apercu", "# Apercu {#overview}\n\nTexte introductif de apercu.\n\n## Etapes {#setup-steps}\n\n"
                          "Lancez. Voir [installer](#installing-it) et [les etapes](#setup-steps).\n")
FR_INSTALLING = translation("installing", "Installer", "# Installer {#installing}\n\n## Installer le produit {#installing-it}\n\nInstallez.\n")
FR_GUIDE = translation("guide", "Guide FR", "# Guide FR {#guide}\n\nTexte du guide. ![image](pic.png)\n\n## Partie locale {#local-part}\n\n"
                       "Voir [autre page](other.md#remote-part) et [la suite](#extra-details).\n\n## Details en plus {#extra-details}\n\nFin.\n")
FR_FILES = {"site/i18n/fr/overview.md": FR_OVERVIEW, "site/i18n/fr/installing.md": FR_INSTALLING, "site/i18n/fr/guide.md": FR_GUIDE}


def mark(value, code: str):
    """The same interface file in another language, made of the English one: every string gets a marker."""
    if isinstance(value, str):
        return f"[{code}] {value}"
    if isinstance(value, dict):
        return {key: mark(item, code) for key, item in value.items()}
    if isinstance(value, list):
        return [mark(item, code) for item in value]
    return value


def write_ui(root: Path, codes) -> None:
    """`site/i18n/<code>/ui.json` for each language, copied from the real English one once task T013 has written it (until then
    there is none to copy and the build under test does not exist either)."""
    source = ROOT / "site" / "i18n" / "en" / "ui.json"
    if not source.is_file():
        return
    ui = json.loads(source.read_text(encoding="utf-8"))
    for code in codes:
        target = root / "site" / "i18n" / code / "ui.json"
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(json.dumps(ui if code == "en" else mark(ui, code), ensure_ascii=False), encoding="utf-8")


def i18n_repo(languages=None, files=None, sources=None) -> Path:
    """A scratch repository with the real templates, stylesheet, scripts, fonts and icons, the pages above, the languages given
    (`LANGUAGES` by default) and the translation files given (`FR_FILES` by default)."""
    root = Path(tempfile.mkdtemp())
    atexit.register(shutil.rmtree, root, True)
    shutil.copytree(ROOT / "site", root / "site", ignore=shutil.ignore_patterns("pages", "examples", "_build", "site.json", "i18n"))
    languages = LANGUAGES if languages is None else languages
    config = {"title": "T", "repo": "https://example.test/r", "branch": "main", "baseUrl": "https://docs.example.test", "groups": GROUPS,
              "languages": languages, "i18n": {"complete": False}, "pages": PAGES}
    write_files(root, {"site/site.json": json.dumps(config), **{**SOURCES, **(sources or {})}, **(FR_FILES if files is None else files)})
    write_ui(root, languages)
    return root


def build_all(root: Path) -> tuple[list[str], Path]:
    """The whole build, English then every enabled language: (errors, output folder). A configuration error is an error line."""
    out = root / "_out"
    try:
        return bs.build_languages(root, out), out
    except bs.BuildError as error:
        return [str(error)], out


@functools.lru_cache(maxsize=None)
def standard_build() -> tuple[list[str], Path]:
    return build_all(i18n_repo())


def read(out: Path, rel: str) -> str:
    return (out / rel).read_text(encoding="utf-8")


# ---------- the golden copy of the English build (T003, T004) ----------

_MAIN = re.compile(r"<main\b[^>]*>(.*?)</main>", re.S)
_FOOTER = re.compile(r"<footer\b[^>]*>(.*?)</footer>", re.S)
_TAGS = re.compile(r"<[^>]+>")
_ANCHOR_LINK = re.compile(r'\s*<a class="anchor"[^>]*>#</a>')


def text_figure(text: str) -> dict:
    return {"words": len(text.split()), "sha256": hashlib.sha256(text.encode("utf-8")).hexdigest()}


def english_projection(out: Path) -> dict:
    """What the English build must keep whatever the language feature adds: the page names, the address of every page, the text of
    every page body (a hash and a word count), the search entries (their text likewise) and the footer sentence. Not bytes: every
    page gains the language control and the alternate links."""
    pages = sorted(path.name for path in out.glob("*.html"))
    bodies, footers = {}, set()
    for name in pages:
        page = read(out, name)
        bodies[name] = text_figure(bs.plain_text(_ANCHOR_LINK.sub("", _MAIN.search(page).group(1))))
        footers.add(" ".join(html.unescape(_TAGS.sub(" ", _FOOTER.search(page).group(1))).split()))
    search = [{**entry, "text": text_figure(entry["text"])} for entry in json.loads(read(out, "search-index.json"))]
    return {"pages": pages, "llms_txt": read(out, "llms.txt").splitlines(), "footers": sorted(footers), "bodies": bodies, "search": search}


@functools.lru_cache(maxsize=None)
def real_english_build() -> Path:
    out = Path(tempfile.mkdtemp())
    atexit.register(shutil.rmtree, out, True)
    errors = bs.build(ROOT, out)
    if errors:
        raise AssertionError(f"the real site does not build: {errors}")
    return out


@unittest.skipUnless((ROOT / "docs").is_dir(), "docs/ is private (absent from the public mirror), so is the golden copy")
class T004EnglishBuildKeepsItsGoldenCopy(unittest.TestCase):
    """US1-AS10, FR-006. Characterization: the English build of today, written down before the feature (golden/README.md)."""

    @classmethod
    def setUpClass(cls):
        cls.golden = json.loads(GOLDEN.read_text(encoding="utf-8"))
        cls.now = english_projection(real_english_build())

    def test_us1_as10_the_pages_and_their_addresses_are_the_same(self):
        self.assertEqual(self.now["pages"], self.golden["pages"])
        self.assertEqual(self.now["llms_txt"], self.golden["llms_txt"])

    def test_us1_as10_the_body_of_every_page_is_the_same(self):
        self.assertEqual(sorted(self.now["bodies"]), sorted(self.golden["bodies"]))
        for name, figure in self.golden["bodies"].items():
            with self.subTest(page=name):
                self.assertEqual(self.now["bodies"][name], figure)

    def test_us1_as10_the_search_entries_are_the_same(self):
        self.assertEqual(len(self.now["search"]), len(self.golden["search"]))
        for now, golden in zip(self.now["search"], self.golden["search"]):
            with self.subTest(page=golden["slug"]):
                self.assertEqual(now, golden)

    def test_us1_as10_the_footer_sentence_is_the_same(self):
        self.assertEqual(self.now["footers"], self.golden["footers"])
        self.assertIn("Pluries Venom is an independent fork of ovh/venom, published under the Apache-2.0 licence", self.golden["footers"][0])

    def test_fr_006_the_real_build_writes_a_language_folder_for_each_enabled_language_and_for_no_other(self):
        config = json.loads((ROOT / "site" / "site.json").read_text(encoding="utf-8"))["languages"]
        enabled = {entry["path"] for entry in config.values() if entry.get("path") and entry.get("enabled") is True}
        folders = {path.name for path in real_english_build().iterdir() if path.is_dir()}
        self.assertEqual(folders, {"fonts", "images", "img"} | enabled)


class T004NoLanguageEnabled(unittest.TestCase):
    """US1-AS10, FR-006, FR-024: a language that is not enabled is neither built nor listed."""

    def test_fr_006_enabled_languages_are_the_translated_ones_that_are_switched_on(self):
        config = {"languages": {"en": {"name": "English", "path": ""}, "fr": {"enabled": True}, "de": {"enabled": False}, "it": {}, "zh": {"enabled": True}}}
        self.assertEqual(i18n.enabled_languages(config), ["fr", "zh"])

    def test_fr_006_no_languages_key_means_english_only(self):
        self.assertEqual(i18n.enabled_languages({"title": "T"}), [])
        self.assertEqual(i18n.enabled_languages({"languages": {"en": {"name": "English", "path": ""}}}), [])

    def test_us1_as10_a_disabled_language_leaves_no_folder_and_no_mention(self):
        root = i18n_repo(languages=DISABLED_FR)
        errors, out = build_all(root)
        self.assertEqual(errors, [])
        self.assertFalse((out / "fr").exists())
        for name in ("search-index.json", "llms.txt", "guide.html"):
            self.assertNotIn("/fr/", read(out, name), name)
        self.assertTrue((out / "guide.html").is_file())


# ---------- the translation file (T007) ----------

HEADER = ("translates: guide\ntitle: Guide FR\ndescription: Un guide.\nfingerprint: abc123\ndrafted_by: claude\n"
          "drafted_on: 2026-11-17\nstatus: machine-drafted\n")


class T007ParseTranslation(unittest.TestCase):
    """FR-007, US2-AS1: the header records what the translation was drafted from."""

    def test_fr_007_the_header_keys_and_the_body_are_split(self):
        parsed = i18n.parse_translation(f"---\n{HEADER}---\n# Guide FR {{#guide}}\n\nTexte.\n")
        self.assertEqual(parsed.header, {
            "translates": "guide", "title": "Guide FR", "description": "Un guide.", "fingerprint": "abc123", "drafted_by": "claude",
            "drafted_on": "2026-11-17", "status": "machine-drafted"})
        self.assertEqual(parsed.body.strip(), "# Guide FR {#guide}\n\nTexte.")

    def test_fr_007_a_reviewed_file_keeps_its_three_reviewed_keys(self):
        reviewed = HEADER.replace("machine-drafted", "reviewed") + "reviewed_by: Maker\nreviewed_on: 2026-11-18\nreviewed_fingerprint: abc123\n"
        header = i18n.parse_translation(f"---\n{reviewed}---\nTexte.\n").header
        self.assertEqual((header["status"], header["reviewed_by"], header["reviewed_on"], header["reviewed_fingerprint"]),
                         ("reviewed", "Maker", "2026-11-18", "abc123"))

    def test_fr_007_a_value_may_hold_a_colon_and_any_script(self):
        text = "---\ntranslates: guide\ntitle: Installer: demarrage rapide\ndescription: 安装并运行：第一步\nstatus: machine-drafted\n---\nx\n"
        header = i18n.parse_translation(text).header
        self.assertEqual((header["title"], header["description"]), ("Installer: demarrage rapide", "安装并运行：第一步"))

    def test_fr_007_windows_line_endings_leave_no_carriage_return_in_a_value(self):
        parsed = i18n.parse_translation(f"---\n{HEADER}---\nTexte.\n".replace("\n", "\r\n"))
        self.assertEqual(parsed.header["title"], "Guide FR")
        self.assertEqual(parsed.header["status"], "machine-drafted")
        self.assertEqual(parsed.body.strip(), "Texte.")

    def test_fr_007_the_home_template_holds_the_same_header_in_an_html_comment(self):
        parsed = i18n.parse_translation(f"<!--\n{HEADER.replace('guide', 'index')}-->\n<!doctype html>\n<p>x</p>\n")
        self.assertEqual((parsed.header["translates"], parsed.header["status"]), ("index", "machine-drafted"))
        self.assertIn("<p>x</p>", parsed.body)
        for gone in ("translates", "-->", "<!--"):
            self.assertNotIn(gone, parsed.body)

    def test_fr_007_a_rule_line_inside_the_body_is_not_a_second_header_end(self):
        parsed = i18n.parse_translation(f"---\n{HEADER}---\nAvant.\n\n---\n\nApres.\n")
        self.assertIn("Avant.\n\n---\n\nApres.", parsed.body)

    def test_fr_007_a_draft_may_miss_what_stamp_writes(self):
        parsed = i18n.parse_translation("---\ntranslates: guide\ntitle: Guide FR\ndescription: Un guide.\n---\nTexte.\n")
        for key in ("fingerprint", "drafted_on", "status"):
            self.assertNotIn(key, parsed.header)

    def test_fr_007_the_two_status_values_are_accepted(self):
        for status in ("machine-drafted", "reviewed"):
            with self.subTest(status=status):
                self.assertEqual(i18n.parse_translation(f"---\n{HEADER.replace('machine-drafted', status)}---\nx\n").header["status"], status)

    def test_fr_007_another_status_is_refused(self):
        for status in ("published", "draft", "Reviewed", ""):
            with self.subTest(status=status), self.assertRaises(i18n.I18nError):
                i18n.parse_translation(f"---\n{HEADER.replace('machine-drafted', status)}---\nx\n")

    def test_fr_007_a_file_without_a_header_is_refused(self):
        with self.assertRaises(i18n.I18nError):
            i18n.parse_translation("# Guide FR\n\nTexte sans en-tete.\n")

    def test_fr_007_a_header_that_is_never_closed_is_refused(self):
        with self.assertRaises(i18n.I18nError):
            i18n.parse_translation(f"---\n{HEADER}# Guide FR\n\nTexte.\n")

    def test_fr_007_a_header_line_that_is_not_key_value_is_refused(self):
        with self.assertRaises(i18n.I18nError):
            i18n.parse_translation(f"---\n{HEADER}this line has no key\n---\nx\n")

    def test_fr_007_the_translated_page_needs_its_page_title_and_description(self):
        for key in ("translates", "title", "description"):
            header = "".join(line + "\n" for line in HEADER.splitlines() if not line.startswith(key + ":"))
            with self.subTest(missing=key), self.assertRaises(i18n.I18nError):
                i18n.parse_translation(f"---\n{header}---\nx\n")


class T007Fingerprint(unittest.TestCase):
    """US2-AS1, US2-AS10, FR-008: a stated set of inputs, after normalising line endings and trailing spaces (D-013)."""
    TEXT, TITLE, DESCRIPTION = "# Guide\n\nRun it.\n\n```yaml\nname: a\n  nested: b\n```\n", "Guide", "How to run it."

    def fp(self, text=None, title=None, description=None, home=None) -> str:
        return i18n.fingerprint(self.TEXT if text is None else text, title or self.TITLE, description or self.DESCRIPTION, home)

    def test_us2_as1_it_is_a_sha256_in_hex_and_stable(self):
        value = self.fp()
        self.assertRegex(value, r"^[0-9a-f]{64}$")
        self.assertEqual(value, self.fp())

    def test_us2_as10_windows_and_unix_line_endings_give_the_same_fingerprint(self):
        self.assertEqual(self.fp(text=self.TEXT.replace("\n", "\r\n")), self.fp())

    def test_us2_as10_trailing_spaces_are_ignored(self):
        self.assertEqual(self.fp(text=self.TEXT.replace("Run it.\n", "Run it.   \n").rstrip("\n") + "  \n"), self.fp())

    def test_us2_as10_indentation_and_inner_spaces_still_count(self):
        self.assertNotEqual(self.fp(text=self.TEXT.replace("  nested", "    nested")), self.fp())
        self.assertNotEqual(self.fp(text=self.TEXT.replace("Run it.", "Run  it.")), self.fp())

    def test_us2_as1_a_changed_text_title_or_description_changes_it(self):
        base = self.fp()
        self.assertNotEqual(self.fp(text=self.TEXT.replace("Run it.", "Run it twice.")), base)
        self.assertNotEqual(self.fp(title="Guide 2"), base)
        self.assertNotEqual(self.fp(description="How to run it twice."), base)

    def test_fr_008_the_inputs_do_not_slide_into_each_other(self):
        self.assertNotEqual(i18n.fingerprint("t", "ab", "c"), i18n.fingerprint("t", "a", "bc"))
        self.assertNotEqual(i18n.fingerprint("ab", "c", "d"), i18n.fingerprint("a", "bc", "d"))

    def test_fr_008_the_home_template_is_part_of_the_home_page_fingerprint(self):
        home = "<main>{{content}}</main>\n<p>Six cards.</p>\n"
        with_home = self.fp(home=home)
        self.assertNotEqual(with_home, self.fp())
        self.assertNotEqual(self.fp(home=home.replace("Six", "Seven")), with_home)
        self.assertEqual(self.fp(home=home.replace("\n", "\r\n")), with_home)


# ---------- the loader (T012 contract, used by T009) ----------

class T007Translations(unittest.TestCase):
    """FR-007: `Translations.page(lang, slug)` builds the translated Page out of the English one."""

    def setUp(self):
        self.root = Path(tempfile.mkdtemp())
        self.addCleanup(shutil.rmtree, self.root, True)
        self.english = {slug: bs.Page(slug, slug.title(), f"site/pages/{slug}.md", group="Guides", description=f"English {slug}.",
                                      subgroup="Sub", heading_index=[3, 4]) for slug in ("guide", "bad", "absent")}
        write_files(self.root, {"site/i18n/fr/guide.md": FR_GUIDE, "site/i18n/fr/bad.md": "# No header at all\n"})
        self.translations = i18n.Translations(self.root, self.english)

    def test_fr_007_the_page_takes_the_translated_title_description_and_text(self):
        page = self.translations.page("fr", "guide")
        self.assertEqual((page.slug, page.title, page.description), ("guide", "Guide FR", "Description de Guide FR."))
        self.assertTrue(page.text.strip().startswith("# Guide FR {#guide}"))
        self.assertNotIn("translates:", page.text)

    def test_fr_007_everything_else_stays_the_english_pages(self):
        page = self.translations.page("fr", "guide")
        self.assertEqual((page.source, page.group, page.subgroup, page.heading_index), ("site/pages/guide.md", "Guides", "Sub", [3, 4]))
        self.assertEqual((self.english["guide"].title, self.english["guide"].text), ("Guide", None))

    def test_fr_007_no_file_is_no_page(self):
        self.assertIsNone(self.translations.page("fr", "absent"))
        self.assertIsNone(self.translations.page("de", "guide"))

    def test_fr_007_a_file_that_cannot_be_parsed_is_an_error_not_a_missing_page(self):
        with self.assertRaises(i18n.I18nError):
            self.translations.page("fr", "bad")


# ---------- stamp (T008) ----------

class T008Stamp(unittest.TestCase):
    """US2-AS9: only `stamp` writes the fingerprint; a re-draft is a machine draft again, never `reviewed`."""
    ENGLISH = "# Alpha\n\nAlpha body text.\n\n{{include src/a.txt text}}\n"
    TODAY = "2026-11-17"

    def repo(self, translated: str) -> Path:
        root = make_repo({"alpha.md": self.ENGLISH, "src/a.txt": "included once\n", "index.md": "# Home\n",
                          "site/i18n/fr/alpha.md": translated},
                         [{"slug": "index", "title": "Home", "source": "index.md"}, {"slug": "alpha", "title": "Alpha", "source": "alpha.md"}],
                         languages={"fr": {"name": "Francais", "path": "fr", "enabled": False}})
        self.addCleanup(shutil.rmtree, root, True)
        return root

    @staticmethod
    def reviewed_file() -> str:
        return ("---\ntranslates: alpha\ntitle: Alfa\ndescription: Une page.\nfingerprint: old\ndrafted_by: claude\ndrafted_on: 2026-01-01\n"
                "status: reviewed\nreviewed_by: Maker\nreviewed_on: 2026-01-02\nreviewed_fingerprint: old\n---\n# Alfa {#alpha}\n\nCorps.\n")

    def stamped(self, root: Path) -> i18n.Translation:
        return i18n.parse_translation((root / "site" / "i18n" / "fr" / "alpha.md").read_text(encoding="utf-8"))

    def expected(self, root: Path) -> str:
        text = bs.expand_includes(self.ENGLISH, root, "alpha.md")
        return i18n.fingerprint(text, "Alpha", "A page of the test site.")

    def test_us2_as9_stamp_writes_the_fingerprint_of_the_english_page_and_the_date(self):
        root = self.repo(self.reviewed_file())
        written = i18n.stamp(root, "fr", "alpha", self.TODAY)
        header = self.stamped(root).header
        self.assertEqual(written, self.expected(root))
        self.assertEqual((header["fingerprint"], header["drafted_on"]), (written, self.TODAY))

    def test_us2_as9_a_reviewed_file_falls_back_to_machine_drafted_and_loses_its_review(self):
        root = self.repo(self.reviewed_file())
        i18n.stamp(root, "fr", "alpha", self.TODAY)
        header = self.stamped(root).header
        self.assertEqual(header["status"], "machine-drafted")
        for key in ("reviewed_by", "reviewed_on", "reviewed_fingerprint"):
            self.assertNotIn(key, header)

    def test_us2_as9_the_translation_itself_is_left_as_it_was(self):
        root = self.repo(self.reviewed_file())
        i18n.stamp(root, "fr", "alpha", self.TODAY)
        parsed = self.stamped(root)
        self.assertEqual({k: parsed.header[k] for k in ("translates", "title", "description", "drafted_by")},
                         {"translates": "alpha", "title": "Alfa", "description": "Une page.", "drafted_by": "claude"})
        self.assertEqual(parsed.body.strip(), "# Alfa {#alpha}\n\nCorps.")
        self.assertNotIn(b"\r", (root / "site" / "i18n" / "fr" / "alpha.md").read_bytes())

    def test_us2_as9_a_fresh_draft_gets_its_fingerprint_date_and_status(self):
        root = self.repo("---\ntranslates: alpha\ntitle: Alfa\ndescription: Une page.\ndrafted_by: claude\n---\n# Alfa {#alpha}\n")
        i18n.stamp(root, "fr", "alpha", self.TODAY)
        header = self.stamped(root).header
        self.assertEqual((header["fingerprint"], header["drafted_on"], header["status"]), (self.expected(root), self.TODAY, "machine-drafted"))

    def test_us2_as10_the_fingerprint_follows_an_included_file(self):
        root = self.repo(self.reviewed_file())
        first = i18n.stamp(root, "fr", "alpha", self.TODAY)
        (root / "src" / "a.txt").write_text("included, then changed\n", encoding="utf-8")
        self.assertNotEqual(i18n.stamp(root, "fr", "alpha", self.TODAY), first)

    def test_us2_as9_a_page_the_site_does_not_have_or_a_missing_file_is_refused(self):
        root = self.repo(self.reviewed_file())
        for slug in ("nowhere", "index"):
            with self.subTest(slug=slug), self.assertRaises(i18n.I18nError):
                i18n.stamp(root, "fr", slug, self.TODAY)


# ---------- a translated page of the README, and the assets of a language (T009, T010) ----------

class T009TranslatedReadmePages(unittest.TestCase):
    """Plan, 'Details the review found in the real code', items on the README pages and one SiteBuild per language."""

    def test_us2_as11_a_translated_readme_page_builds_with_the_english_anchor_map(self):
        errors, out = standard_build()
        self.assertEqual(errors, [])
        page = read(out, "fr/overview.html")
        self.assertIn("Texte introductif", page)
        self.assertIn('<h2 id="setup-steps">Etapes', page)
        self.assertNotIn("Intro text of the overview", page)
        self.assertIn("Intro text of the overview", read(out, "overview.html"))

    def test_us2_as11_a_link_to_an_english_anchor_resolves_on_the_page_and_to_another_page(self):
        _, out = standard_build()
        page = read(out, "fr/overview.html")
        self.assertIn('href="#setup-steps"', page)
        self.assertIn('href="installing.html#installing-it"', page)

    def test_us2_as11_the_link_to_the_anchor_a_french_heading_would_have_had_is_dead(self):
        files = {**FR_FILES, "site/i18n/fr/overview.md": FR_OVERVIEW.replace("[les etapes](#setup-steps)", "[les etapes](#etapes)")}
        errors, _ = build_all(i18n_repo(files=files))
        self.assertTrue(any("dead anchor" in error and "#etapes" in error for error in errors), errors)

    def test_us2_as11_the_explicit_ids_of_a_translated_page_are_its_anchors(self):
        errors, out = standard_build()
        self.assertEqual(errors, [])
        page = read(out, "fr/guide.html")
        self.assertIn('id="extra-details"', page)
        self.assertIn('href="#extra-details"', page)
        self.assertIn('href="other.html#remote-part"', page)

    def test_us2_as11_a_link_to_an_anchor_the_translation_does_not_have_is_dead(self):
        files = {**FR_FILES, "site/i18n/fr/guide.md": FR_GUIDE.replace("(#extra-details)", "(#nope)")}
        errors, _ = build_all(i18n_repo(files=files))
        self.assertTrue(any("dead anchor" in error and "#nope" in error for error in errors), errors)

    def test_us2_as11_a_translated_heading_without_its_english_anchor_fails_the_build_naming_page_and_heading(self):
        files = {**FR_FILES, "site/i18n/fr/guide.md": FR_GUIDE.replace("## Partie locale {#local-part}", "## Partie locale")}
        errors, _ = build_all(i18n_repo(files=files))
        self.assertTrue(any("guide" in error and "Partie locale" in error for error in errors), errors)

    def test_plan_fr_after_en_has_the_body_of_fr_and_the_body_of_en_stays(self):
        _, out = standard_build()
        self.assertIn("Texte du guide", read(out, "fr/guide.html"))
        self.assertNotIn("Guide text", read(out, "fr/guide.html"))
        self.assertIn("Guide text", read(out, "guide.html"))
        self.assertNotIn("Texte du guide", read(out, "guide.html"))

    def test_plan_a_language_build_writes_its_pages_and_search_files_and_no_asset(self):
        _, out = standard_build()
        for name in ("images", "style.css", "fonts", "img", "theme.js", "site.js"):
            self.assertFalse((out / "fr" / name).exists(), f"fr/{name}")
        for name in ("images/site/pages/pic.png", "style.css", "theme.js", "site.js"):
            self.assertTrue((out / name).exists(), name)
        for name in ("guide.html", "overview.html", "search-index.json", "llms.txt", "llms-full.txt"):
            self.assertTrue((out / "fr" / name).is_file(), f"fr/{name}")

    def test_plan_build_is_the_language_build(self):
        root = i18n_repo()
        errors, out = build_all(root)
        self.assertEqual(errors, [])
        other = root / "_other"
        self.assertEqual(bs.build(root, other), [])
        self.assertEqual(sorted(p.relative_to(out).as_posix() for p in out.rglob("*") if p.is_file()),
                         sorted(p.relative_to(other).as_posix() for p in other.rglob("*") if p.is_file()))
        self.assertEqual(read(other, "fr/guide.html"), read(out, "fr/guide.html"))


LOCAL_TAGS = re.compile(r"<(?:img|script|link)\b[^>]*>")
LOCAL_RELS = ("stylesheet", "icon", "apple-touch-icon")


def local_addresses(page: str) -> list[str]:
    """The relative address of every image, script, stylesheet and icon a page loads."""
    found = []
    for tag in LOCAL_TAGS.findall(page):
        address = re.search(r'\b(?:src|href)="([^"]*)"', tag)
        rel = re.search(r'\brel="([^"]*)"', tag)
        if address and not re.match(r"[a-z]+:", address.group(1)) and (not tag.startswith("<link") or (rel and rel.group(1) in LOCAL_RELS)):
            found.append(address.group(1))
    return found


class T010AssetsPrefix(unittest.TestCase):
    """US1-AS13, FR-023, D-010: shared assets stay at the root; a language page reaches them with a `../` prefix."""
    PREFIXED = ('href="../img/snake.svg"', 'href="../img/favicon.ico"', 'href="../img/snake-180.png"', 'src="../img/snake.svg"',
                'src="../theme.js"', 'src="../site.js"', 'href="../style.css"', 'src="../images/site/pages/pic.png"')

    def test_us1_as13_head_links_brand_scripts_stylesheet_and_image_carry_the_prefix(self):
        _, out = standard_build()
        page = read(out, "fr/guide.html")
        for needle in self.PREFIXED:
            self.assertIn(needle, page)

    def test_us1_as13_the_home_template_stylesheet_and_icons_carry_the_prefix(self):
        _, out = standard_build()
        page = read(out, "fr/index.html")
        for needle in self.PREFIXED[:7]:
            self.assertIn(needle, page)

    def test_us1_as13_every_asset_a_language_page_loads_starts_with_the_prefix(self):
        _, out = standard_build()
        for name in ("guide.html", "index.html", "overview.html"):
            with self.subTest(page=name):
                addresses = local_addresses(read(out, f"fr/{name}"))
                self.assertTrue(addresses)
                self.assertEqual([a for a in addresses if not a.startswith("../")], [])

    def test_us1_as13_the_links_inside_the_language_folder_get_no_prefix(self):
        _, out = standard_build()
        page = read(out, "fr/guide.html")
        self.assertIn('class="brand" href="index.html"', page)
        self.assertIn('<li><a href="other.html">', page)
        self.assertRegex(page, r'rel="(?:prev|next)" href="[a-z-]+\.html"')

    def test_fr_006_the_english_pages_keep_an_empty_prefix(self):
        _, out = standard_build()
        for name in ("guide.html", "index.html"):
            with self.subTest(page=name):
                addresses = local_addresses(read(out, name))
                self.assertIn("style.css", addresses)
                self.assertEqual([a for a in addresses if a.startswith("../")], [])

    def test_fr_023_a_build_removes_every_language_folder_before_it_writes(self):
        root = i18n_repo(languages=DISABLED_FR)
        out = root / "_out"
        for code in LANGUAGE_FOLDERS:
            write_files(out, {f"{code}/stale.html": "<p>left by an earlier build</p>"})
        errors, _ = build_all(root)
        self.assertEqual(errors, [])
        for code in LANGUAGE_FOLDERS:
            self.assertFalse((out / code).exists(), code)
        self.assertTrue((out / "guide.html").is_file())

    def test_fr_023_a_page_left_in_an_enabled_language_folder_is_removed_too(self):
        root = i18n_repo()
        write_files(root / "_out", {"fr/stale.html": "<p>left by an earlier build</p>"})
        errors, out = build_all(root)
        self.assertEqual(errors, [])
        self.assertFalse((out / "fr" / "stale.html").exists())
        self.assertTrue((out / "fr" / "guide.html").is_file())

    def test_us1_as13_a_page_slug_equal_to_a_language_prefix_is_refused(self):
        root = i18n_repo(sources={"site/pages/fr.md": "# Francais\n"})
        path = root / "site" / "site.json"
        config = json.loads(path.read_text(encoding="utf-8"))
        config["pages"] = [*config["pages"], {"slug": "fr", "title": "Fr", "source": "site/pages/fr.md", "group": "Guides", "description": "A page."}]
        path.write_text(json.dumps(config), encoding="utf-8")
        errors, _ = build_all(root)
        self.assertTrue(any("fr" in error and ("prefix" in error.lower() or "language" in error.lower()) for error in errors), errors)


# ---------- User Story 1: a visitor reads the site in their language (T017 to T020, T023, T025) ----------

SITE_BASE = "https://docs.example.test/"
CODES = ["en", "fr", "de", "it", "es", "zh"]
NAMES = ["English", "Français", "Deutsch", "Italiano", "Español", "中文"]
LANG_ATTRIBUTE = {"en": "en", "fr": "fr", "de": "de", "it": "it", "es": "es", "zh": "zh-Hans"}
PATH = {"en": "", "fr": "fr", "de": "de", "it": "it", "es": "es", "zh": "zh"}
SLUGS = ["index", "overview", "installing", "guide", "other"]
SITE_JS = (ROOT / "site" / "js" / "site.js").read_text(encoding="utf-8")
THEME_JS = (ROOT / "site" / "js" / "theme.js").read_text(encoding="utf-8")
SCRIPT_KEYS = ("copy", "copied", "copy_failed", "copy_aria", "search_aria", "search_label", "search_placeholder", "search_empty",
               "search_none", "search_loading", "search_unavailable", "result_one", "result_other", "theme_aria", "theme_title", "theme_names")
REAL_LANGUAGES = json.loads((ROOT / "site" / "site.json").read_text(encoding="utf-8"))["languages"]
SIX = {code: {**entry, **({"enabled": True} if code != "en" else {})} for code, entry in REAL_LANGUAGES.items()}
SIX_FILES = {**FR_FILES, **{f"site/i18n/{code}/guide.md": translation("guide", f"Guide {code}", f"# Guide {code} {{#guide}}\n\nTexte {code}.\n\n## Partie {code} {{#local-part}}\n\nFin.\n")
                            for code in ("de", "it", "es", "zh")}}
VOID_TAGS = {"meta", "link", "br", "img", "hr", "input", "source", "area", "base", "col", "embed", "wbr", "track", "param"}


def page_rel(code: str, slug: str) -> str:
    return (f"{PATH[code]}/" if PATH[code] else "") + f"{slug}.html"


def page_url(code: str, slug: str) -> str:
    return SITE_BASE + page_rel(code, slug)


def squash(text: str) -> str:
    return re.sub(r"\s+", " ", text).strip()


def attrs_of(tag_text: str) -> dict[str, str]:
    return {name: html.unescape(value) for name, value in re.findall(r'([\w:-]+)="([^"]*)"', tag_text)}


class Doc(HTMLParser):
    """A page as a flat list of elements: tag, attributes, the language in force on it (the nearest `lang` attribute up the tree),
    its text and the addresses of the links inside it."""

    def __init__(self, text: str) -> None:
        super().__init__(convert_charrefs=True)
        self.stack: list[dict] = []
        self.elements: list[dict] = []
        self.feed(text)
        self.close()

    def _open(self, tag: str, attrs: dict) -> dict:
        parent = self.stack[-1] if self.stack else None
        element = {"tag": tag, "attrs": attrs, "lang": attrs.get("lang", parent["lang"] if parent else ""), "text": [], "hrefs": []}
        self.elements.append(element)
        return element

    def handle_starttag(self, tag, attrs):
        attrs = dict(attrs)
        element = self._open(tag, attrs)
        if tag == "a" and "href" in attrs:
            for open_element in self.stack:
                open_element["hrefs"].append(attrs["href"])
        if tag not in VOID_TAGS:
            self.stack.append(element)

    def handle_startendtag(self, tag, attrs):
        self._open(tag, dict(attrs))

    def handle_endtag(self, tag):
        if tag in VOID_TAGS:
            return
        for index in range(len(self.stack) - 1, -1, -1):
            if self.stack[index]["tag"] == tag:
                del self.stack[index:]
                return

    def handle_data(self, data):
        for element in self.stack:
            element["text"].append(data)

    def find(self, tag: str | None = None, cls: str | None = None) -> list[dict]:
        return [e for e in self.elements if (tag is None or e["tag"] == tag) and (cls is None or cls in e["attrs"].get("class", "").split())]

    @staticmethod
    def words(element: dict) -> str:
        return squash("".join(element["text"]))


def doc_of(out: Path, rel: str) -> Doc:
    return Doc(read(out, rel))


def html_files(out: Path) -> list[str]:
    """Every page of the built site, English at the root and each language in its folder (not the HTML files of `images/`)."""
    return sorted(p.relative_to(out).as_posix() for p in out.rglob("*.html") if p.relative_to(out).parts[0] not in ("images", "fonts", "img"))


def code_of(rel: str) -> str:
    first = rel.split("/")[0]
    return first if first in LANGUAGE_FOLDERS else "en"


@functools.lru_cache(maxsize=None)
def standard_six() -> tuple[list[str], Path]:
    """All six languages enabled; `guide` is translated in the five, `overview` and `installing` in French, nothing else is."""
    root = i18n_repo(languages=SIX, files=SIX_FILES, sources={"site/pages/quickstart.md": "# Quickstart\n\nStart here.\n"})
    path = root / "site" / "site.json"  # the home template links `quickstart.html`
    config = json.loads(path.read_text(encoding="utf-8"))
    config["pages"] = [*config["pages"], {"slug": "quickstart", "title": "Quickstart", "source": "site/pages/quickstart.md", "group": "Getting started",
                                          "description": "Start."}]
    path.write_text(json.dumps(config), encoding="utf-8")
    return build_all(root)


def ui_of(out: Path, code: str) -> dict:
    return json.loads((out.parent / "site" / "i18n" / code / "ui.json").read_text(encoding="utf-8"))


def menu_links(out: Path, code: str, slug: str) -> list[tuple[dict, str]]:
    """The language control of a page: (attributes, text) of each link."""
    found = re.findall(r'<nav class="lang-menu"[^>]*>(.*?)</nav>', read(out, page_rel(code, slug)), re.S)
    if len(found) != 1:
        raise AssertionError(f"{page_rel(code, slug)} has {len(found)} language controls, expected 1")
    return [(attrs_of(m.group(1)), html.unescape(m.group(2))) for m in re.finditer(r"<a\b([^>]*)>(.*?)</a>", found[0], re.S)]


class T017LanguageControl(unittest.TestCase):
    """US1-AS1, AS2, AS3, FR-001, FR-002, SC-005: the language control of every page, and the notice of a page not translated."""

    @classmethod
    def setUpClass(cls):
        cls.errors, cls.out = standard_six()

    def test_us1_as1_the_six_languages_build_without_an_error(self):
        self.assertEqual(self.errors, [])

    def test_fr_001_the_real_site_json_names_the_six_languages_each_in_its_own_script(self):
        self.assertEqual(list(REAL_LANGUAGES), CODES)
        self.assertEqual([entry["name"] for entry in REAL_LANGUAGES.values()], NAMES)
        self.assertEqual({code: entry["path"] for code, entry in REAL_LANGUAGES.items()}, PATH)
        self.assertEqual({code: entry["lang"] for code, entry in REAL_LANGUAGES.items()}, LANG_ATTRIBUTE)

    def test_us1_as1_every_page_lists_the_six_languages_by_their_own_names(self):
        for code in CODES:
            for slug in SLUGS:
                with self.subTest(page=page_rel(code, slug)):
                    self.assertEqual([text for _, text in menu_links(self.out, code, slug)], NAMES)

    def test_us1_as1_the_current_language_is_marked_and_only_it(self):
        for code in CODES:
            for slug in ("guide", "other", "index"):
                with self.subTest(page=page_rel(code, slug)):
                    marked = [text for attrs, text in menu_links(self.out, code, slug) if "aria-current" in attrs]
                    self.assertEqual(marked, [NAMES[CODES.index(code)]])

    def test_us1_as1_each_link_carries_the_language_and_hreflang_of_its_target(self):
        for code in CODES:
            with self.subTest(page=page_rel(code, "guide")):
                links = menu_links(self.out, code, "guide")
                self.assertEqual([(a.get("lang"), a.get("hreflang")) for a, _ in links], [(LANG_ATTRIBUTE[c], LANG_ATTRIBUTE[c]) for c in CODES])

    def test_us1_as2_every_link_leads_to_the_same_page_in_the_chosen_language(self):
        for code in CODES:
            for slug in SLUGS:
                with self.subTest(page=page_rel(code, slug)):
                    own = page_url(code, slug)
                    self.assertEqual([urljoin(own, attrs["href"]) for attrs, _ in menu_links(self.out, code, slug)],
                                     [page_url(target, slug) for target in CODES])

    def test_us1_as2_the_script_carries_the_anchor_of_the_address_to_the_language_links(self):
        self.assertIn("lang-menu", SITE_JS)
        self.assertRegex(SITE_JS, r"location\.hash", "without a script the control is a plain link; with one, it keeps the section")

    def test_sc_005_every_link_of_every_control_leads_to_a_page_that_exists(self):
        for rel in html_files(self.out):
            code, slug = code_of(rel), rel.rsplit("/", 1)[-1][:-5]
            for attrs, text in menu_links(self.out, code, slug):
                with self.subTest(page=rel, link=text):
                    target = urlsplit(urljoin(SITE_BASE + rel, attrs["href"])).path.lstrip("/")
                    self.assertTrue((self.out / target).is_file(), target)

    def test_us1_as3_a_page_not_translated_is_served_under_the_prefix_with_a_notice_in_the_language_and_a_link_to_english(self):
        for code in CODES[1:]:
            for slug in ("other", "index"):
                rel = page_rel(code, slug)
                with self.subTest(page=rel):
                    page = doc_of(self.out, rel)
                    notices = page.find(cls="fallback-notice")
                    self.assertEqual(len(notices), 1, "one element of class fallback-notice")
                    self.assertIn(html.escape(ui_of(self.out, code)["fallback_notice"], quote=False).replace("&amp;", "&"), page.words(notices[0]))
                    self.assertEqual([urljoin(SITE_BASE + rel, href) for href in notices[0]["hrefs"]], [page_url("en", slug)])

    def test_us1_as3_a_translated_page_and_an_english_page_carry_no_such_notice(self):
        for code in CODES:
            with self.subTest(page=page_rel(code, "guide")):
                self.assertEqual(doc_of(self.out, page_rel(code, "guide")).find(cls="fallback-notice"), [])
        self.assertEqual(doc_of(self.out, page_rel("en", "other")).find(cls="fallback-notice"), [])


def top_bar(out: Path, rel: str) -> str:
    found = re.search(r'<header class="top">.*?</header>', read(out, rel), re.S)
    if not found:
        raise AssertionError(f"{rel} has no top bar")
    return found.group(0)


def menu_html(out: Path, rel: str) -> str:
    return re.search(r'<nav class="lang-menu"[^>]*>(.*?)</nav>', read(out, rel), re.S).group(1)


class T017LanguageDropdown(unittest.TestCase):
    """US1-AS1, FR-002, D-027: the language control is a dropdown in the top bar, the first of its actions; the links are the same."""

    @classmethod
    def setUpClass(cls):
        cls.errors, cls.out = standard_six()

    def test_us1_as1_the_control_is_the_first_child_of_the_actions_of_the_top_bar_on_every_page(self):
        for rel in html_files(self.out):
            bar = top_bar(self.out, rel)
            with self.subTest(page=rel):
                self.assertEqual(bar.count('class="lang-menu"'), 1)
                self.assertRegex(bar, r'<div class="top-actions">\s*<nav class="lang-menu"')

    def test_us1_as1_the_control_comes_before_the_first_icon_button(self):
        for rel in html_files(self.out):
            bar = top_bar(self.out, rel)
            with self.subTest(page=rel):
                self.assertIn('class="lang-menu"', bar, "the control is not in the top bar")
                self.assertIn('class="icon-btn"', bar)
                self.assertLess(bar.index('class="lang-menu"'), bar.index('class="icon-btn"'))

    def test_us1_as1_the_old_row_after_the_top_bar_is_gone_on_every_page_and_home_page(self):
        for rel in html_files(self.out):
            text = read(self.out, rel)
            with self.subTest(page=rel):
                self.assertEqual(text.count('<nav class="lang-menu"'), 1)
                self.assertNotIn("lang-menu", text.split("</header>", 1)[1])

    def test_us1_as1_no_template_keeps_the_language_menu_placeholder_of_the_old_row(self):
        paths = [ROOT / "site" / "template.html", ROOT / "site" / "home.html", *sorted((ROOT / "site" / "i18n").glob("*/home.html"))]
        self.assertGreater(len(paths), 2)
        for path in paths:
            with self.subTest(template=path.relative_to(ROOT).as_posix()):
                self.assertNotIn("{{language_menu}}", path.read_text(encoding="utf-8"))

    def test_us1_as1_the_nav_is_labelled_and_marked_with_the_language_of_the_page(self):
        for code in CODES:
            for slug in ("guide", "index", "other"):
                with self.subTest(page=page_rel(code, slug)):
                    nav = doc_of(self.out, page_rel(code, slug)).find("nav", "lang-menu")
                    self.assertEqual(len(nav), 1)
                    self.assertEqual(nav[0]["attrs"].get("aria-label"), ui_of(self.out, code)["language"])
                    self.assertEqual(nav[0]["attrs"].get("lang"), LANG_ATTRIBUTE[code])

    def test_us1_as1_one_details_holds_a_summary_naming_the_current_language_and_all_the_links(self):
        for code in CODES:
            for slug in ("guide", "index"):
                rel = page_rel(code, slug)
                menu = menu_html(self.out, rel)
                with self.subTest(page=rel):
                    details = re.findall(r"<details\b[^>]*>", menu)
                    self.assertEqual(len(details), 1)
                    self.assertIn("lang-dropdown", attrs_of(details[0]).get("class", "").split())
                    self.assertNotRegex(details[0], r"\sopen\b", "closed until the visitor opens it")
                    summaries = re.findall(r"<summary\b[^>]*>(.*?)</summary>", menu, re.S)
                    self.assertEqual([squash(html.unescape(re.sub(r"<[^>]+>", "", s))) for s in summaries], [NAMES[CODES.index(code)]])
                    inside = re.search(r"<details\b.*</details>", menu, re.S).group(0)
                    self.assertEqual(len(re.findall(r"<a\b", inside)), len(CODES))
                    panel = re.search(r'<div\b[^>]*\bclass="lang-panel"[^>]*>(.*?)</div>', inside, re.S)
                    self.assertIsNotNone(panel, "the links sit inside div.lang-panel")
                    self.assertEqual(len(re.findall(r"<a\b", panel.group(1))), len(CODES))
                    self.assertNotIn("<a", inside.split("</summary>")[0], "no link in the summary")
                    current = doc_of(self.out, rel).find(cls="lang-current")
                    self.assertEqual([Doc.words(e) for e in current], [NAMES[CODES.index(code)]])

    def test_us1_as1_a_site_with_one_language_renders_no_control_and_the_actions_start_with_the_repository_icon(self):
        root = us2_repo(home=True, languages=DISABLED_FR)
        self.assertEqual(build_errors(root), [])
        for rel in ("index.html", "guide.html"):
            bar = top_bar(root / "_out", rel)
            with self.subTest(page=rel):
                self.assertNotIn("lang-", bar)
                self.assertRegex(bar, r'<div class="top-actions">\s*<a class="icon-btn"')

    def test_us1_as1_the_script_closes_an_open_dropdown_on_escape_and_on_a_click_outside(self):
        self.assertIn(".lang-dropdown", SITE_JS)
        self.assertRegex(SITE_JS, r"\.open\s*=\s*false|removeAttribute\(\s*[\"']open[\"']\s*\)")
        self.assertGreaterEqual(len(re.findall(r"[\"']Escape[\"']", SITE_JS)), 2, "the search overlay and the dropdown both close on Escape")
        self.assertRegex(SITE_JS, r"addEventListener\(\s*[\"']click[\"']")
        self.assertRegex(SITE_JS, r"\.contains\(\s*event\.target\s*\)", "a click outside is told from a click inside")

    def test_us1_as1_escape_inside_the_open_dropdown_returns_the_focus_to_its_summary_and_a_click_outside_does_not_move_it(self):
        """WCAG 2.4.3: closing the panel from the keyboard must not leave the focus on a hidden link."""
        body = SITE_JS[SITE_JS.index("const setupLanguageMenu"):].split("\n  setupTheme();", 1)[0]
        on_key, on_click = body.split('addEventListener("keydown"', 1)[1].split('addEventListener("click"', 1)
        self.assertIn("closeDropdowns(null)", on_key, "the Escape handler still closes the dropdowns")
        self.assertIn("contains(document.activeElement)", on_key, "the focus moves only when it was inside the dropdown")
        self.assertIn("summary", on_key)
        self.assertIn(".focus()", on_key)
        self.assertLess(on_key.index("closeDropdowns(null)"), on_key.index(".focus()"), "the focus returns after the panel is closed")
        self.assertNotIn(".focus(", on_click, "a click outside leaves the focus where the visitor put it")

    def test_us1_as1_the_language_links_still_keep_the_section_of_the_address(self):
        self.assertRegex(SITE_JS, r"link\.hash\s*=\s*globalThis\.location\.hash|link\.hash\s*=\s*(?:window\.)?location\.hash")


class T018InterfaceText(unittest.TestCase):
    """US1-AS4, FR-003, FR-020: the interface text, the `lang` attribute and `data-ui` come from the language's `ui.json`."""

    @classmethod
    def setUpClass(cls):
        cls.errors, cls.out = standard_six()

    def test_us1_as4_the_lang_attribute_of_a_translated_page_is_its_language_zh_hans_for_chinese(self):
        for code in CODES:
            with self.subTest(code=code):
                self.assertEqual(doc_of(self.out, page_rel(code, "guide")).find("html")[0]["attrs"]["lang"], LANG_ATTRIBUTE[code])

    def test_us1_as4_the_menu_search_skip_theme_pager_toc_and_footer_text_come_from_the_ui_file(self):
        for code in CODES:
            ui = ui_of(self.out, code)
            page = doc_of(self.out, page_rel(code, "guide"))
            theme = ui["theme_aria"].replace("{name}", ui["theme_names"]["auto"])
            with self.subTest(code=code):
                self.assertEqual(page.words(page.find("a", "skip")[0]), ui["skip"])
                self.assertEqual(page.find("button", "search-button")[0]["attrs"]["aria-label"], ui["search_aria"])
                self.assertEqual(page.find("button", "theme-button")[0]["attrs"]["aria-label"], theme)
                menu = page.find("button", "menu-toggle")[0]
                self.assertEqual((menu["attrs"]["aria-label"], page.words(menu)), (ui["menu_aria"], ui["menu"]))
                self.assertEqual(page.words(page.find("p", "side-title")[0]), ui["on_this_page"])
                self.assertEqual(page.find("nav", "pager")[0]["attrs"]["aria-label"], ui["pager_aria"])
                self.assertEqual(page.find("nav", "lang-menu")[0]["attrs"]["aria-label"], ui["language"])
                self.assertEqual([page.words(e) for e in page.find("span") if e["text"] and page.words(e) in (ui["previous"], ui["next"])],
                                 [ui["previous"], ui["next"]])
                footer = page.words(page.find("footer")[0])
                for key in ("footer_notice", "footer_upstream", "footer_licence"):
                    self.assertIn(ui[key], footer)
                sidebar = Doc(re.sub(r'<nav class="lang-menu".*?</nav>', "", read(self.out, page_rel(code, "guide")), flags=re.S))
                self.assertEqual([sidebar.words(e) for e in sidebar.find("summary")], [ui["group_names"]["Getting started"], ui["group_names"]["Guides"]])

    def test_us1_as4_the_top_navigation_names_come_from_the_ui_file(self):
        for code in CODES:
            ui = ui_of(self.out, code)
            top = re.search(r'<nav class="top-nav"[^>]*>(.*?)</nav>', read(self.out, page_rel(code, "guide")), re.S).group(1)
            with self.subTest(code=code):
                self.assertEqual([html.unescape(t) for t in re.findall(r"<a\b[^>]*>(.*?)</a>", top)], [ui["docs"], ui["reference"]])

    def test_fr_003_every_page_carries_the_interface_text_of_the_script_in_data_ui(self):
        for rel in html_files(self.out):
            ui = ui_of(self.out, code_of(rel))
            with self.subTest(page=rel):
                raw = doc_of(self.out, rel).find("body")[0]["attrs"].get("data-ui")
                self.assertIsNotNone(raw, "<body> has no data-ui attribute")
                data = json.loads(raw)
                self.assertEqual({key: data.get(key) for key in SCRIPT_KEYS}, {key: ui[key] for key in SCRIPT_KEYS})

    def test_fr_003_the_data_ui_of_a_home_page_is_there_too_not_only_on_the_documentation_template(self):
        for code in CODES:
            with self.subTest(code=code):
                body = doc_of(self.out, page_rel(code, "index")).find("body")[0]
                self.assertIn("home", body["attrs"].get("class", "").split())
                self.assertIn("data-ui", body["attrs"])

    def test_fr_003_the_script_reads_its_strings_from_data_ui_and_every_key_it_needs_is_one_of_them(self):
        self.assertRegex(SITE_JS, r"dataset\.ui\b|getAttribute\(\s*[\"']data-ui[\"']\s*\)")
        self.assertIn("JSON.parse", SITE_JS)
        for key in SCRIPT_KEYS:
            if key not in ("copy", "copied"):
                with self.subTest(key=key):
                    self.assertRegex(SITE_JS, rf"\b{key}\b")

    def test_fr_003_no_text_of_the_page_outside_its_content_is_an_english_literal_of_the_code_or_the_templates(self):
        """Every ui string of the fixture carries the marker `[<code>] `: what is left without it, outside the page's own content, is a
        literal of a render function or a template that has no key."""
        allowed = {"PLURIES VENOM", "github.com/pluries/venom", "·"}
        strip = (r"<head\b.*?</head>", r"<main\b.*?</main>", r'<nav class="lang-menu".*?</nav>', r"<svg\b.*?</svg>", r"<li\b[^>]*>.*?</li>",
                 r"<strong>.*?</strong>")
        for code, slug in (("fr", "guide"), ("fr", "index"), ("zh", "guide"), ("de", "other")):
            rest = read(self.out, page_rel(code, slug))
            for pattern in strip:
                rest = re.sub(pattern, "", rest, flags=re.S)
            found = [html.unescape(t).strip() for t in re.findall(r">([^<>]+)<", rest)]
            found += [html.unescape(v) for v in re.findall(r'(?:aria-label|title|placeholder|alt)="([^"]*)"', rest)]
            with self.subTest(page=page_rel(code, slug)):
                self.assertEqual([t for t in found if t and not t.startswith(f"[{code}]") and t not in allowed], [])


def key_paths(value, prefix: str = "") -> set[str]:
    """Every key of an interface file, nested ones as `a.b`."""
    if not isinstance(value, dict):
        return set()
    return {path for key, item in value.items() for path in ({f"{prefix}{key}"} | key_paths(item, f"{prefix}{key}."))}


def placeholders_of(value, prefix: str = "") -> dict[str, list[str]]:
    """The `{name}` placeholders of every string of an interface file, by key path."""
    if isinstance(value, dict):
        return {path: found for key, item in value.items() for path, found in placeholders_of(item, f"{prefix}{key}.").items()}
    return {prefix.rstrip("."): sorted(re.findall(r"\{\w+\}", value))} if isinstance(value, str) else {}


_PLACEHOLDER = re.compile(r"\{\{\s*([\w.]+)\s*\}\}")
_TAG = re.compile(r"<(/?)([a-zA-Z][\w-]*)([^>]*)>")


def template_shape(text: str) -> tuple[list[str], list[tuple[str, str, str]]]:
    """The placeholders of a template in order, and its tags in order with their class and local link: not its words."""
    tags = []
    for closing, name, rest in _TAG.findall(text):
        cls, href = re.search(r'class="([^"]*)"', rest), re.search(r'href="([^"]*)"', rest)
        tags.append((closing + name, cls.group(1) if cls else "", href.group(1) if href and not re.match(r"[a-z]+:", href.group(1)) else ""))
    return _PLACEHOLDER.findall(text), tags


def template_problems(english: str, other: str) -> list[str]:
    """Where a translated home template differs from the English one in placeholders or structure (empty when the same)."""
    wanted, got = template_shape(english), template_shape(other)
    problems = []
    if wanted[0] != got[0]:
        problems.append(f"placeholders {got[0]} instead of {wanted[0]}")
    if len(wanted[1]) != len(got[1]):
        problems.append(f"{len(got[1])} tags instead of {wanted[1].__len__()}")
    problems += [f"tag {i + 1}: {b} instead of {a}" for i, (a, b) in enumerate(zip(wanted[1], got[1])) if a != b][:3]
    return problems


class T018InterfaceFiles(unittest.TestCase):
    """US1-AS4, FR-003, FR-020: the interface files and the home templates are the same in every language."""
    ENGLISH = json.loads((ROOT / "site" / "i18n" / "en" / "ui.json").read_text(encoding="utf-8"))

    def test_fr_003_the_key_sets_of_every_ui_file_that_exists_equal_the_english_one(self):
        for code in LANGUAGE_FOLDERS:
            path = ROOT / "site" / "i18n" / code / "ui.json"
            if path.is_file():
                with self.subTest(code=code):
                    other = json.loads(path.read_text(encoding="utf-8"))
                    self.assertEqual((key_paths(other) - key_paths(self.ENGLISH), key_paths(self.ENGLISH) - key_paths(other)), (set(), set()))

    def test_d_028_no_ui_file_has_a_draft_banner_key_and_the_other_two_notices_keep_theirs(self):
        for code in ["en", *LANGUAGE_FOLDERS]:
            path = ROOT / "site" / "i18n" / code / "ui.json"
            if path.is_file():
                with self.subTest(code=code):
                    keys = key_paths(json.loads(path.read_text(encoding="utf-8")))
                    self.assertNotIn("draft_banner", keys)
                    self.assertLessEqual({"fallback_notice", "override_notice"}, keys)

    def test_fr_003_a_placeholder_of_an_english_string_is_in_the_same_string_of_every_language(self):
        for code in LANGUAGE_FOLDERS:
            path = ROOT / "site" / "i18n" / code / "ui.json"
            if path.is_file():
                with self.subTest(code=code):
                    self.assertEqual(placeholders_of(json.loads(path.read_text(encoding="utf-8"))), placeholders_of(self.ENGLISH))

    def test_fr_003_the_key_set_checker_reports_a_missing_nested_key_and_an_extra_key(self):
        reference = {"a": "x", "b": {"c": "y", "d": "z"}}
        self.assertEqual(key_paths(reference), {"a", "b", "b.c", "b.d"})
        self.assertEqual(key_paths({"a": "x", "b": {"c": "y"}}) ^ key_paths(reference), {"b.d"})
        self.assertEqual(key_paths({**reference, "e": "w"}) ^ key_paths(reference), {"e"})
        self.assertNotEqual(placeholders_of({"p": "{name} package"}), placeholders_of({"p": "package"}))

    def test_fr_003_the_english_ui_groups_are_the_groups_of_site_json(self):
        config = json.loads((ROOT / "site" / "site.json").read_text(encoding="utf-8"))
        self.assertEqual(self.ENGLISH["groups"], config["groups"])
        self.assertEqual(set(self.ENGLISH["group_names"]), set(bs.GROUPS))

    def test_fr_003_subgroups_has_a_key_for_every_subgroup_name_of_the_pages(self):
        site = bs.SiteBuild(ROOT, bs.load_config(ROOT), "")
        names = {page.subgroup for page in site.pages if page.subgroup}
        self.assertEqual(names - set(self.ENGLISH["subgroups"]), set())

    def test_fr_020_both_templates_carry_the_language_placeholders(self):
        for name in ("template.html", "home.html"):
            text = (ROOT / "site" / name).read_text(encoding="utf-8")
            with self.subTest(template=name):
                for placeholder in ("lang", "assets", "alternates", "topbar", "ui_skip", "ui_footer_notice"):
                    self.assertIn("{{" + placeholder + "}}", text)

    def test_fr_020_the_home_template_of_every_language_that_exists_has_the_placeholders_and_the_structure_of_the_english_one(self):
        english = (ROOT / "site" / "home.html").read_text(encoding="utf-8")
        for code in LANGUAGE_FOLDERS:
            path = ROOT / "site" / "i18n" / code / "home.html"
            if path.is_file():
                with self.subTest(code=code):
                    self.assertEqual(template_problems(english, i18n.parse_translation(path.read_text(encoding="utf-8")).body), [])

    def test_fr_020_the_template_checker_accepts_other_words_and_reports_any_other_structure(self):
        english = '<h1 class="a">Hello <b>you</b></h1>{{content}}<a href="quickstart.html">Go</a><a href="https://x.test/">Out</a>'
        self.assertEqual(template_problems(english, '<h1 class="a">Bonjour <b>toi</b></h1>{{content}}<a href="quickstart.html">Aller</a><a href="https://y.test/">Dehors</a>'), [])
        for broken in ('<h1 class="b">Hello <b>you</b></h1>{{content}}<a href="quickstart.html">Go</a><a href="https://x.test/">Out</a>',
                       '<h1 class="a">Hello <b>you</b></h1><a href="quickstart.html">Go</a><a href="https://x.test/">Out</a>',
                       '<h1 class="a">Hello <b>you</b></h1>{{content}}<a href="other.html">Go</a><a href="https://x.test/">Out</a>',
                       '<h1 class="a">Hello <b>you</b></h1>{{content}}<a href="quickstart.html">Go</a>'):
            with self.subTest(broken=broken):
                self.assertNotEqual(template_problems(english, broken), [])


def head_alternates(page: str) -> tuple[list[str], dict[str, str]]:
    """The canonical addresses and the `hreflang` -> address map of a page's head."""
    head = re.search(r"<head\b.*?</head>", page, re.S).group(0)
    tags = [attrs_of(tag) for tag in re.findall(r"<link\b[^>]*>", head)]
    return ([t["href"] for t in tags if t.get("rel") == "canonical"],
            {t["hreflang"]: t["href"] for t in tags if t.get("rel") == "alternate" and "hreflang" in t})


def index_slugs(out: Path, code: str) -> list[str]:
    folder = f"{PATH[code]}/" if PATH[code] else ""
    return [entry["slug"] for entry in json.loads(read(out, f"{folder}search-index.json"))]


class T019HeadAndFallback(unittest.TestCase):
    """US1-AS5, US1-AS11, FR-004, FR-026: the head of every page, and the markup of an English page served under a language prefix."""

    @classmethod
    def setUpClass(cls):
        cls.errors, cls.out = standard_six()

    def test_us1_as5_a_page_translated_in_every_language_names_them_all_plus_x_default_and_its_own_canonical(self):
        for code in CODES:
            with self.subTest(page=page_rel(code, "guide")):
                canonical, alternates = head_alternates(read(self.out, page_rel(code, "guide")))
                self.assertEqual(canonical, [page_url(code, "guide")])
                self.assertEqual(alternates, {**{LANG_ATTRIBUTE[c]: page_url(c, "guide") for c in CODES}, "x-default": page_url("en", "guide")})

    def test_us1_as5_a_page_names_only_the_versions_that_exist(self):
        for code in ("en", "fr"):
            with self.subTest(page=page_rel(code, "overview")):
                canonical, alternates = head_alternates(read(self.out, page_rel(code, "overview")))
                self.assertEqual(canonical, [page_url(code, "overview")])
                self.assertEqual(alternates, {"en": page_url("en", "overview"), "fr": page_url("fr", "overview"), "x-default": page_url("en", "overview")})

    def test_us1_as5_every_page_of_every_language_has_one_canonical_address_of_its_own_site(self):
        for rel in html_files(self.out):
            with self.subTest(page=rel):
                canonical, _ = head_alternates(read(self.out, rel))
                self.assertEqual(len(canonical), 1)
                self.assertTrue(canonical[0].startswith(SITE_BASE), canonical)

    def test_us1_as11_the_content_of_a_page_not_translated_is_english_inside_an_interface_in_the_language(self):
        for code in ("de", "zh"):
            for slug in ("other", "index"):
                rel = page_rel(code, slug)
                with self.subTest(page=rel):
                    page = doc_of(self.out, rel)
                    self.assertEqual(page.find("main")[0]["lang"], "en")
                    self.assertEqual(page.find("a", "skip")[0]["lang"], LANG_ATTRIBUTE[code])
                    self.assertEqual(page.find(cls="fallback-notice")[0]["lang"], LANG_ATTRIBUTE[code])
                    self.assertEqual(page.find("nav", "lang-menu")[0]["lang"], LANG_ATTRIBUTE[code])

    def test_us1_as11_the_canonical_is_the_english_address_and_the_page_claims_no_hreflang_of_its_language(self):
        for code in ("de", "zh"):
            for slug in ("other", "index"):
                rel = page_rel(code, slug)
                with self.subTest(page=rel):
                    canonical, alternates = head_alternates(read(self.out, rel))
                    self.assertEqual(canonical, [page_url("en", slug)])
                    self.assertNotIn(LANG_ATTRIBUTE[code], alternates)

    def test_us1_as11_a_page_not_translated_is_in_neither_the_index_nor_the_llms_files_of_its_language(self):
        for code in CODES[1:]:
            folder = PATH[code]
            for name in ("llms.txt", "llms-full.txt"):
                text = read(self.out, f"{folder}/{name}")
                with self.subTest(file=f"{folder}/{name}"):
                    self.assertNotIn(f"/{folder}/other.html", text)
                    self.assertNotIn("# Other", text)
            self.assertNotIn("other", index_slugs(self.out, code))
            self.assertNotIn("index", index_slugs(self.out, code))
            self.assertIn("guide", index_slugs(self.out, code))
            self.assertIn(f"{SITE_BASE}{folder}/guide.html", read(self.out, f"{folder}/llms.txt"))

    def test_us1_as11_no_page_appears_twice_in_the_index_or_the_llms_file_of_a_language(self):
        for code in CODES:
            folder = f"{PATH[code]}/" if PATH[code] else ""
            slugs = index_slugs(self.out, code)
            addresses = re.findall(r"\]\((\S+?)\)", read(self.out, f"{folder}llms.txt"))
            with self.subTest(code=code):
                self.assertEqual(len(slugs), len(set(slugs)))
                self.assertEqual(len(addresses), len(set(addresses)))


class T020NoLanguageChoice(unittest.TestCase):
    """US1-AS6, AS12, FR-005: the site never chooses a language for the visitor; the control is plain links."""
    SCRIPTS = {"site.js": SITE_JS, "theme.js": THEME_JS}

    @classmethod
    def setUpClass(cls):
        cls.errors, cls.out = standard_six()

    def test_us1_as6_no_script_reads_a_cookie_or_the_language_of_the_browser(self):
        for name, source in self.SCRIPTS.items():
            for needle in ("document.cookie", "navigator.language", "navigator.languages", "userLanguage", "Accept-Language"):
                with self.subTest(script=name, needle=needle):
                    self.assertNotIn(needle, source)

    def test_fr_005_the_only_stored_value_of_a_script_is_the_theme(self):
        for name, source in self.SCRIPTS.items():
            keys = set(re.findall(r"(?:get|set|remove)Item\(\s*([^,)\s]+)", source))
            with self.subTest(script=name):
                self.assertLessEqual(keys, {"THEME_KEY", '"venom-theme"'})
                self.assertNotIn("sessionStorage", source)
                self.assertNotRegex(source, r"localStorage\s*(?:\[|\.(?!(?:get|set|remove)Item))")
        self.assertIn('THEME_KEY = "venom-theme"', SITE_JS)

    def test_us1_as6_no_script_redirects_the_visitor_on_language(self):
        for name, source in self.SCRIPTS.items():
            with self.subTest(script=name):
                self.assertNotRegex(source, r"location\.(?:replace|assign)\(")
        self.assertEqual(re.findall(r"location\.href\s*=\s*([^;]+);", SITE_JS), ["search.hits[search.selected].href"])

    def test_us1_as6_no_page_redirects_or_sets_a_cookie_by_markup(self):
        for rel in html_files(self.out):
            page = read(self.out, rel)
            with self.subTest(page=rel):
                self.assertNotRegex(page, r"(?i)http-equiv\s*=\s*[\"']?(?:refresh|set-cookie)")
                self.assertNotIn("<form", page)
                self.assertNotIn("<select", page)

    def test_us1_as12_the_control_is_made_of_plain_links_with_a_real_address(self):
        for rel in html_files(self.out):
            control = re.search(r'<nav class="lang-menu"[^>]*>(.*?)</nav>', read(self.out, rel), re.S).group(1)
            with self.subTest(page=rel):
                self.assertEqual(re.findall(r"<(?!/?(?:a|details|summary|span|div)\b)([a-z]+)", control), [], "no form control, button or script")
                self.assertNotRegex(control, r"(?i)\bon\w+\s*=|javascript:")
                hrefs = [attrs_of(tag)["href"] for tag in re.findall(r"<a\b[^>]*>", control)]
                self.assertEqual(len(hrefs), len(CODES))
                self.assertTrue(all(h.endswith(".html") and not h.startswith(("#", "javascript:")) for h in hrefs), hrefs)

    def test_us1_as6_the_server_config_chooses_nothing_from_the_headers_and_redirects_only_a_language_folder_to_its_slash(self):
        conf = (ROOT / "deploy" / "site" / "nginx.conf").read_text(encoding="utf-8")
        for needle in ("accept_language", "$cookie_", "set-cookie", "vary"):
            with self.subTest(needle=needle):
                self.assertNotIn(needle, conf.lower())
        self.assertNotRegex(conf, r"(?m)^\s*map\s")
        self.assertLessEqual(set(re.findall(r"return\s+30\d\s+(\S+?);", conf)), {"/$1/"})


def resolve_in_site(rel: str, value: str):
    """(path inside the site, fragment) an address of the page `rel` leads to, or None for another host."""
    parts = urlsplit(urljoin(SITE_BASE + rel, value))
    return None if parts.netloc != urlsplit(SITE_BASE).netloc else (parts.path.lstrip("/"), parts.fragment)


def _reference_value(element: dict) -> str | None:
    """The address an element of a page loads or links to, or None when it is not one of the references the check follows."""
    attrs, tag = element["attrs"], element["tag"]
    value = attrs.get("src") if tag in ("img", "script") else attrs.get("href") if tag in ("a", "link") else None
    if tag == "link" and attrs.get("rel") not in ("stylesheet", "icon", "apple-touch-icon", "canonical", "alternate"):
        return None
    return value


def _stylesheet_problems(out: Path, path: str) -> list[str]:
    """The `url()` of a stylesheet that leads to no file of the built site."""
    problems = []
    for address in re.findall(r"url\(\s*['\"]?([^)'\"]+)", read(out, path)):
        inner = None if address.startswith("data:") else resolve_in_site(path, address)
        if inner and not (out / inner[0]).is_file():
            problems.append(f"{path}: url({address}) leads to {inner[0]}, which does not exist")
    return problems


def _element_problems(out: Path, rel: str, element: dict, ids: dict[str, set[str]]) -> list[str]:
    value = _reference_value(element)
    target = None if value is None else resolve_in_site(rel, value)
    if target is None:
        return []
    path, fragment = target[0] or rel, target[1]
    if not (out / path).is_file():
        return [f"{rel}: {value} leads to {path}, which does not exist"]
    problems = []
    if fragment and path.endswith(".html"):
        ids.setdefault(path, set(re.findall(r'\bid="([^"]+)"', read(out, path))))
        if fragment not in ids[path]:
            problems.append(f"{rel}: {value} names no id of {path}")
    if element["tag"] == "link" and element["attrs"].get("rel") == "stylesheet":
        problems += _stylesheet_problems(out, path)
    return problems


def broken_references(out: Path) -> list[str]:
    """Every link, image, stylesheet, script, icon and font of every page that does not lead to a file of the built site from the
    page's own address, and every fragment that names no id of its target."""
    problems: list[str] = []
    ids: dict[str, set[str]] = {}
    for rel in html_files(out):
        for element in doc_of(out, rel).elements:
            problems += _element_problems(out, rel, element, ids)
        if not (out / (urlsplit(urljoin(SITE_BASE + rel, "search-index.json")).path.lstrip("/"))).is_file():
            problems.append(f"{rel}: the search index it loads does not exist")
    return problems


class T023EveryAddressResolves(unittest.TestCase):
    """US1-AS13, FR-023: from its own address, every page of every language loads the same assets as an English page."""

    def test_us1_as13_every_reference_of_every_page_of_the_six_languages_resolves(self):
        errors, out = standard_six()
        self.assertEqual(errors, [])
        self.assertEqual(broken_references(out), [])
        self.assertIn("fr/guide.html", html_files(out))

    def test_us1_as13_the_reference_checker_reports_a_missing_file_a_dead_fragment_and_a_missing_stylesheet_font(self):
        _, built = standard_six()
        out = Path(tempfile.mkdtemp())
        self.addCleanup(shutil.rmtree, out, True)
        shutil.copytree(built, out, dirs_exist_ok=True)
        (out / "fr" / "guide.html").write_text(read(out, "fr/guide.html").replace('href="../style.css"', 'href="style.css"').replace("#extra-details", "#nope", 1),
                                               encoding="utf-8")
        (out / "theme.js").unlink()
        problems = "\n".join(broken_references(out))
        for needle in ("fr/guide.html: style.css leads to fr/style.css", "fr/guide.html: #nope names no id", "theme.js, which does not exist"):
            self.assertIn(needle, problems)

    def test_us1_as13_a_page_slug_equal_to_the_declared_path_of_a_language_is_refused(self):
        languages = {"en": LANGUAGES["en"], "fr": {**LANGUAGES["fr"], "path": "manual"}}
        root = i18n_repo(languages=languages, sources={"site/pages/manual.md": "# Manual\n"})
        path = root / "site" / "site.json"
        config = json.loads(path.read_text(encoding="utf-8"))
        config["pages"] = [*config["pages"], {"slug": "manual", "title": "Manual", "source": "site/pages/manual.md", "group": "Guides", "description": "A page."}]
        path.write_text(json.dumps(config), encoding="utf-8")
        errors, _ = build_all(root)
        self.assertTrue(any("manual" in error for error in errors), errors)

    def test_us1_as13_every_language_folder_name_is_refused_as_a_slug(self):
        for code in LANGUAGE_FOLDERS:
            with self.subTest(slug=code):
                root = i18n_repo(languages=DISABLED_FR, sources={f"site/pages/{code}.md": "# Page\n"})
                path = root / "site" / "site.json"
                config = json.loads(path.read_text(encoding="utf-8"))
                config["pages"] = [*config["pages"], {"slug": code, "title": "P", "source": f"site/pages/{code}.md", "group": "Guides", "description": "A page."}]
                path.write_text(json.dumps(config), encoding="utf-8")
                errors, _ = build_all(root)
                self.assertTrue(any(code in error for error in errors), errors)


def run_main(root: Path, out: Path, *extra: str) -> tuple[int, str]:
    stdout, stderr = io.StringIO(), io.StringIO()
    with contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
        code = bs.main(["--root", str(root), "--out", str(out), *extra])
    return code, stdout.getvalue() + stderr.getvalue()


def make_targets(text: str) -> dict[str, str]:
    """Each target of a Makefile with its header line and its recipe."""
    blocks: dict[str, list[str]] = {}
    current = None
    for line in text.splitlines():
        header = re.match(r"^([A-Za-z0-9_.-]+):(?!=)", line)
        if header:
            current = header.group(1)
            blocks[current] = [line]
        elif current and line.startswith("\t"):
            blocks[current].append(line)
        elif line.strip():
            current = None
    return {name: "\n".join(lines) for name, lines in blocks.items()}


class T025PreviewLanguages(unittest.TestCase):
    """D-017: a disabled language is built only when it is named for a preview, never for the published site or the image."""

    def test_d_017_a_build_that_names_a_disabled_language_builds_it_and_a_normal_build_does_not(self):
        root = i18n_repo(languages=DISABLED_FR)
        errors = bs.build_languages(root, root / "_preview", preview_languages=("fr",))
        self.assertEqual(errors, [])
        self.assertTrue((root / "_preview" / "fr" / "guide.html").is_file())
        self.assertIn("/fr/guide.html", read(root / "_preview", "fr/llms.txt"))
        self.assertEqual(bs.build_languages(root, root / "_normal"), [])
        self.assertFalse((root / "_normal" / "fr").exists())

    def test_d_017_the_command_line_option_builds_the_named_language_and_the_plain_command_does_not(self):
        root = i18n_repo(languages=DISABLED_FR)
        code, text = run_main(root, root / "_with", "--preview-languages", "fr")
        self.assertEqual(code, 0, text)
        self.assertTrue((root / "_with" / "fr" / "guide.html").is_file())
        code, text = run_main(root, root / "_without")
        self.assertEqual(code, 0, text)
        self.assertFalse((root / "_without" / "fr").exists())

    def test_d_017_an_option_naming_a_language_the_site_does_not_have_is_refused(self):
        root = i18n_repo(languages=DISABLED_FR)
        code, text = run_main(root, root / "_out", "--preview-languages", "xx")
        self.assertEqual(code, 2, text)
        self.assertIn("xx", text)
        self.assertFalse((root / "_out" / "xx").exists())

    def test_d_017_the_preview_page_count_adds_the_language_pages(self):
        root = i18n_repo(languages=DISABLED_FR)
        _, plain = run_main(root, root / "_a")
        _, preview = run_main(root, root / "_b", "--preview-languages", "fr")
        self.assertIn(f"built {len(SLUGS)} page(s)", plain)
        self.assertIn(f"built {2 * len(SLUGS)} page(s)", preview)

    def test_d_017_the_publish_and_image_builds_never_pass_the_option(self):
        files = [ROOT / "Dockerfile", ROOT / "Jenkinsfile", *(ROOT / "deploy").rglob("*"), *(ROOT / "jenkins").rglob("*"), *(ROOT / "scripts").glob("*.sh")]
        for path in files:
            if path.is_file():
                text = path.read_text(encoding="utf-8", errors="replace")
                with self.subTest(file=path.relative_to(ROOT).as_posix()):
                    self.assertNotIn("preview-languages", text)
                    self.assertNotIn("preview_languages", text)
        for name, recipe in make_targets((ROOT / "Makefile").read_text(encoding="utf-8")).items():
            if name != "site-preview":
                with self.subTest(target=name):
                    self.assertNotIn("preview-languages", recipe)

    def test_d_017_make_site_preview_builds_every_language_with_the_option(self):
        recipe = make_targets((ROOT / "Makefile").read_text(encoding="utf-8")).get("site-preview", "")
        self.assertIn("build_site.py", recipe, "no target site-preview in the Makefile")
        self.assertRegex(recipe, r"--preview-languages[ =]\S*\bfr\b\S*\bde\b\S*\bit\b\S*\bes\b\S*\bzh\b")


# ---------- User Story 2: a translation cannot silently go wrong or go stale (T047 to T053) ----------
# Each test builds a scratch repository whose French translations are right (the clean set, stamped with the fingerprint of the
# English pages of the scratch), breaks ONE rule, and runs the gate `site_i18n.check_site` (or the build, the commands).

TODAY = "2026-11-18"
REVIEWER = "Test Reviewer"
GLOSSARY = {"verbatim": ["venom", "executor", "assertion"], "forbidden": {"fr": {"executor": ["executeur"]}, "de": {"executor": ["Ausfuehrer"]}}}
RICH_EN = """# Guide

The executor runs 3 steps of venom 1.5. Use `venom run` first, then read [the repo](https://example.test/repo) or [the other page](other.md#remote-part).

## Setup

```yaml
name: demo
testcases:
  - name: one
```

```sh
venom run demo.yml
```

![diagram](pic.png)

- first item
- second item
- third item

| name | value |
|------|-------|
| a    | 1     |

> **Note**: keep the file name.

## Results

Done in 2 minutes.
"""
FR_RICH = """# Guide FR {#guide}

L'executor lance 3 etapes de venom 1.5. Utilisez `venom run` d'abord, puis lisez [le depot](https://example.test/repo) ou [l'autre page](other.md#remote-part).

## Installation {#setup}

```yaml
name: demo
testcases:
  - name: one
```

```sh
venom run demo.yml
```

![schema](pic.png)

- premier element
- deuxieme element
- troisieme element

| nom | valeur |
|-----|--------|
| a   | 1      |

> **Note** : gardez le nom du fichier.

## Resultats {#results}

Termine en 2 minutes.
"""
FR_US2_OVERVIEW = translation("overview", "Apercu", "# Apercu {#overview}\n\nTexte introductif.\n\n## Etapes {#setup-steps}\n\n"
                              "Lancez la configuration. Voir [installer](#installing-it).\n")
FR_US2_INSTALLING = translation("installing", "Installer", "# Installer {#installing}\n\n## Installer le produit {#installing-it}\n\nInstallez le produit.\n")
FR_US2_OTHER = translation("other", "Autre", "# Autre {#other}\n\n## Partie distante {#remote-part}\n\nAutre texte.\n")
FR_US2_QUICKSTART = translation("quickstart", "Demarrage", "# Demarrage {#quickstart}\n\nCommencez ici.\n")
DE_US2_OVERVIEW = translation("overview", "Uebersicht", "# Uebersicht {#overview}\n\nEinleitung.\n\n## Schritte {#setup-steps}\n\nSiehe [installieren](#installing-it).\n")
ENGLISH_HOME = (ROOT / "site" / "home.html").read_text(encoding="utf-8")
HOME_HEADER = ("<!--\ntranslates: index\ntitle: Accueil\ndescription: Page d'accueil.\nfingerprint: not-checked-by-the-build\ndrafted_by: claude\n"
               "drafted_on: 2026-11-17\nstatus: machine-drafted\n-->\n")
QUICKSTART_SOURCE = {"site/pages/quickstart.md": "# Quickstart\n\nStart here.\n"}
DRAFTED_PAGES = ("overview", "installing", "guide")
DRAFT_TEXTS = (  # the wording of the banner D-028 removed, in each language: no page carries any of them
    "This page was translated by a machine and no person has reviewed it yet. The English page is the reference.",
    "Cette page a été traduite par une machine et personne ne l'a encore relue. La page en anglais fait foi.",
    "Diese Seite wurde maschinell übersetzt und noch von keiner Person geprüft. Maßgeblich ist die englische Seite.",
    "Questa pagina è stata tradotta automaticamente e nessuna persona l'ha ancora rivista. Fa fede la pagina in inglese.",
    "Esta página ha sido traducida por una máquina y todavía nadie la ha revisado. La página en inglés es la referencia.",
    "本页由机器翻译，尚未经过人工审校。请以英文页面为准。",
)


def assert_no_draft_banner(test: unittest.TestCase, text: str) -> None:
    """D-028, FR-011: neither the element, nor its class, nor the text of the draft notice is on the page."""
    page = Doc(text)
    test.assertEqual(page.find(cls="draft-banner"), [], "an element of class draft-banner")
    test.assertNotIn("draft-banner", text)
    words = squash(html.unescape(re.sub(r"<[^>]+>", " ", text)))
    for wording in DRAFT_TEXTS:
        test.assertNotIn(wording, words)


def broke(text: str, old: str, new: str) -> str:
    """`text` with its first `old` replaced; a fixture that drifted away from its own text is an error, not a passing test."""
    if old not in text:
        raise AssertionError(f"fixture drift: {old!r} is not in the text")
    return text.replace(old, new, 1)


def edit(path: Path, old: str, new: str, newline: str | None = None) -> None:
    text = broke(path.read_text(encoding="utf-8"), old, new)
    with open(path, "w", encoding="utf-8", newline=newline) as handle:
        handle.write(text)


def us2_files(guide: str = FR_RICH, home: bool = False) -> dict[str, str]:
    """The clean French set of the scratch site: three pages (a fourth and the home page with `home`), the glossary and the reviewers."""
    files = {"site/i18n/fr/overview.md": FR_US2_OVERVIEW, "site/i18n/fr/installing.md": FR_US2_INSTALLING,
             "site/i18n/fr/guide.md": translation("guide", "Guide FR", guide),
             "site/i18n/glossary.json": json.dumps(GLOSSARY), "site/i18n/reviewers.txt": f"{REVIEWER}\n"}
    if home:
        files.update({"site/i18n/fr/other.md": FR_US2_OTHER, "site/i18n/fr/quickstart.md": FR_US2_QUICKSTART,
                      "site/i18n/fr/index.md": translation("index", "Accueil", "# Accueil {#home}\n\nTexte d'accueil.\n"),
                      "site/i18n/fr/home.html": HOME_HEADER + ENGLISH_HOME})
    return files


def add_quickstart(root: Path) -> None:
    """The real home template links `quickstart.html`: the scratch site gets that page, as `standard_six` does."""
    write_files(root, QUICKSTART_SOURCE)
    path = root / "site" / "site.json"
    config = json.loads(path.read_text(encoding="utf-8"))
    config["pages"] = [*config["pages"], {"slug": "quickstart", "title": "Quickstart", "source": "site/pages/quickstart.md",
                                          "group": "Getting started", "description": "Start."}]
    path.write_text(json.dumps(config), encoding="utf-8")


def us2_repo(guide: str = FR_RICH, files: dict | None = None, home: bool = False, languages=None, stamped: bool = True) -> Path:
    """A scratch repository with the clean set; every French file is stamped with the fingerprint of today's English page."""
    root = i18n_repo(languages=languages, files={**us2_files(guide, home), **(files or {})}, sources={"site/pages/guide.md": RICH_EN})
    if home:
        add_quickstart(root)
    if stamped:
        for path in sorted((root / "site" / "i18n" / "fr").glob("*.md")):
            i18n.stamp(root, "fr", path.stem, TODAY)
    return root


def set_complete(root: Path, value: bool = True) -> None:
    path = root / "site" / "site.json"
    config = json.loads(path.read_text(encoding="utf-8"))
    config["i18n"] = {"complete": value}
    path.write_text(json.dumps(config), encoding="utf-8")


def i18n_file(root: Path, rel: str, lang: str = "fr") -> Path:
    return root / "site" / "i18n" / lang / rel


def header_of(root: Path, rel: str, lang: str = "fr") -> dict[str, str]:
    return i18n.parse_translation(i18n_file(root, rel, lang).read_text(encoding="utf-8")).header


def reviews_file(root: Path) -> Path:
    return root / "site" / "i18n" / "reviews.json"


def write_reviews(root: Path, *records: dict) -> None:
    """The review record (D-024) as the `review` command writes it: a JSON list; the tests that hand-write it need it in their fixture."""
    reviews_file(root).write_text(json.dumps(list(records)), encoding="utf-8")


def add_review_record(root: Path, slug: str, by: str, on: str) -> None:
    """What the `review` command would have recorded for the page: its language, slug, the English fingerprint it was drafted from and
    the fingerprint of the translation's own text (D-024, amended)."""
    path = reviews_file(root)
    records = json.loads(path.read_text(encoding="utf-8")) if path.is_file() else []
    fingerprint = header_of(root, f"{slug}.md")["fingerprint"]
    write_reviews(root, *records, {"language": "fr", "slug": slug, "fingerprint": fingerprint, "translation_fingerprint": translation_fingerprint_of(root, slug),
                                   "reviewer": by, "date": on})


def translation_fingerprint_of(root: Path, slug: str, lang: str = "fr") -> str:
    """The SHA-256 of the body of a one-file page's translation (header excluded), line endings as LF and trailing spaces removed:
    computed here, independently of the code under test. The home page (two files) is not hand-recorded."""
    body = i18n.parse_translation(i18n_file(root, f"{slug}.md", lang).read_text(encoding="utf-8")).body
    return hashlib.sha256(i18n._normal(body).encode("utf-8")).hexdigest()


def hand_review(root: Path, slug: str = "guide", by: str | None = REVIEWER, on: str | None = TODAY, fingerprint: str | None = "today",
                record: bool = True) -> None:
    """What a person writing the reviewed keys by hand would write (the `review` command is not used): None leaves a key out. The review
    record of `reviews.json` is added as the command would write it unless `record` is False (a forged review: header only)."""
    path = i18n_file(root, f"{slug}.md")
    value = header_of(root, f"{slug}.md")["fingerprint"] if fingerprint == "today" else fingerprint
    keys = [("reviewed_by", by), ("reviewed_on", on), ("reviewed_fingerprint", value)]
    lines = "".join(f"{key}: {item}\n" for key, item in keys if item is not None)
    if record:
        add_review_record(root, slug, by or REVIEWER, on or TODAY)
    edit(path, "status: machine-drafted\n", "status: reviewed\n" + lines)


def append_body(path: Path, text: str) -> None:
    """Edit a translation after its review, the way a person fixing a sentence would: the header is left alone."""
    with open(path, "a", encoding="utf-8", newline="\n") as handle:
        handle.write(text)


def current_fingerprint(text: str, title: str, description: str) -> str:
    return i18n.fingerprint(text, title, description)


def only_page(test: unittest.TestCase, found: list[str], page: str = "fr/guide") -> None:
    """The gate failed, and only on that page: every finding starts with `<language>/<slug>`."""
    test.assertTrue(found, "the gate found nothing")
    test.assertTrue(all(finding.startswith(page) for finding in found), found)


def mentions(test: unittest.TestCase, found: list[str], *parts: str) -> None:
    test.assertTrue(any(all(part in finding for part in parts) for finding in found), f"no finding with {parts}: {found}")


class T047StaleFingerprint(unittest.TestCase):
    """US2-AS1, US2-AS10, SC-004, FR-008: a translation whose recorded English fingerprint is not today's fails and says so."""

    def test_us2_as1_a_set_drafted_from_todays_english_pages_has_no_finding(self):
        self.assertEqual(i18n.check_site(us2_repo()), [])

    def test_us2_as1_a_changed_english_text_fails_naming_language_page_and_both_fingerprints(self):
        root = us2_repo()
        recorded = header_of(root, "guide.md")["fingerprint"]
        changed = broke(RICH_EN, "Done in 2 minutes.", "Done in 2 minutes, or 3.")
        (root / "site" / "pages" / "guide.md").write_text(changed, encoding="utf-8")
        found = i18n.check_site(root)
        today = current_fingerprint(changed, "Guide", "A guide.")
        self.assertNotEqual(recorded, today)
        mentions(self, found, "fr/guide", recorded, today)
        self.assertEqual([finding for finding in found if not finding.startswith("fr/guide")], [])

    def test_us2_as1_a_changed_title_or_description_of_the_english_page_fails_too(self):
        for key, value in (("title", "Guide, renamed"), ("description", "A guide, reworded.")):
            root = us2_repo()
            path = root / "site" / "site.json"
            config = json.loads(path.read_text(encoding="utf-8"))
            for entry in config["pages"]:
                if entry.get("slug") == "guide":
                    entry[key] = value
            path.write_text(json.dumps(config), encoding="utf-8")
            with self.subTest(changed=key):
                mentions(self, i18n.check_site(root), "fr/guide")

    def test_sc_004_one_english_change_names_every_language_and_page_to_update_and_only_those(self):
        languages = {**LANGUAGES, "de": {"name": "Deutsch", "path": "de", "lang": "de", "enabled": True}}
        root = us2_repo(languages=languages, files={"site/i18n/de/overview.md": DE_US2_OVERVIEW})
        i18n.stamp(root, "de", "overview", TODAY)
        self.assertEqual(i18n.check_site(root), [])
        edit(root / "README.md", "Intro text of the overview.", "Intro text, changed.")
        found = i18n.check_site(root)
        mentions(self, found, "fr/overview")
        mentions(self, found, "de/overview")
        self.assertEqual([finding for finding in found if not finding.startswith(("fr/overview", "de/overview"))], [], "installing and guide are not stale")

    def test_us2_as1_a_file_that_records_no_fingerprint_is_stale(self):
        root = us2_repo()
        edit(i18n_file(root, "guide.md"), f"fingerprint: {header_of(root, 'guide.md')['fingerprint']}\n", "")
        mentions(self, i18n.check_site(root), "fr/guide")

    def test_us2_as10_the_same_english_page_with_windows_line_endings_is_not_stale(self):
        root = us2_repo()
        (root / "site" / "pages" / "guide.md").write_bytes(RICH_EN.replace("\n", "\r\n").encode("utf-8"))
        self.assertEqual(i18n.check_site(root), [])

    def test_us2_as10_trailing_spaces_in_the_english_page_do_not_make_it_stale(self):
        root = us2_repo()
        (root / "site" / "pages" / "guide.md").write_text(RICH_EN.replace("Done in 2 minutes.", "Done in 2 minutes.   "), encoding="utf-8")
        self.assertEqual(i18n.check_site(root), [])

    def test_us2_as1_the_unit_names_both_values_and_accepts_equal_ones(self):
        self.assertEqual(i18n.check_fingerprint("fr/guide", "abc", "abc"), [])
        found = i18n.check_fingerprint("fr/guide", "abc", "def")
        self.assertEqual(len(found), 1)
        self.assertTrue(found[0].startswith("fr/guide") and "abc" in found[0] and "def" in found[0], found)
        self.assertEqual(len(i18n.check_fingerprint("fr/guide", None, "def")), 1)


YAML_BLOCK = "```yaml\nname: demo\ntestcases:\n  - name: one\n```\n"
SH_BLOCK = "```sh\nvenom run demo.yml\n```\n"


class T048Identity(unittest.TestCase):
    """US2-AS2, AS3, AS8, FR-009, SC-002: code blocks, inline code, URLs, images, link targets and anchors are the English ones, in
    number and order; a link or an address that English lacks fails. One rule broken per case, and only that page fails."""
    CASES = [
        ("one character of a code block", lambda t: broke(t, "venom run demo.yml", "venom run demo.yaml"), ["code block 2"]),
        ("one character of the yaml block", lambda t: broke(t, "  - name: one", "  - name: two"), ["code block 1"]),
        ("the language tag of a block", lambda t: broke(t, "```sh", "```bash"), ["code block 2"]),
        ("a block removed", lambda t: broke(t, "```sh\nvenom run demo.yml\n```\n\n", ""), ["code block"]),
        ("a block added", lambda t: t + "\n```sh\nrm -rf /\n```\n", ["code block"]),
        ("two blocks swapped", lambda t: broke(broke(broke(t, YAML_BLOCK, "@@"), SH_BLOCK, YAML_BLOCK), "@@", SH_BLOCK), ["code block 1", "code block 2"]),
        ("an inline command changed", lambda t: broke(t, "`venom run` d'abord", "`venom  run` d'abord"), ["inline code"]),
        ("an inline command lost its backticks", lambda t: broke(t, "`venom run`", "venom run"), ["inline code"]),
        ("a link address changed", lambda t: broke(t, "https://example.test/repo", "https://evil.example/repo"), ["evil.example"]),
        ("a link English lacks", lambda t: t + "\nVoir [plus](https://evil.example/more).\n", ["evil.example"]),
        ("a bare address English lacks", lambda t: t + "\nSource: https://evil.example/x\n", ["evil.example"]),
        ("an image target changed", lambda t: broke(t, "![schema](pic.png)", "![schema](other.png)"), ["image"]),
        ("an internal link target changed", lambda t: broke(t, "other.md#remote-part", "other.md#remote"), ["link"]),
        ("a link removed", lambda t: broke(t, "[le depot](https://example.test/repo)", "le depot"), ["link"]),
        ("two links swapped", lambda t: broke(broke(broke(t, "[le depot](https://example.test/repo)", "@@"), "[l'autre page](other.md#remote-part)",
                                                    "[le depot](https://example.test/repo)"), "@@", "[l'autre page](other.md#remote-part)"), ["link"]),
        ("an anchor changed", lambda t: broke(t, "{#setup}", "{#config}"), ["anchor"]),
        ("two anchors swapped", lambda t: broke(broke(broke(t, "{#setup}", "{#@@}"), "{#results}", "{#setup}"), "{#@@}", "{#results}"), ["anchor"]),
        ("a heading without its English anchor", lambda t: broke(t, "## Installation {#setup}", "## Installation"), ["anchor"]),
        ("a heading removed", lambda t: broke(t, "## Resultats {#results}\n\nTermine en 2 minutes.\n", "Termine en 2 minutes.\n"), ["heading"]),
    ]

    def test_us2_as2_each_case_fails_that_page_only_and_names_the_thing(self):
        for name, change, parts in self.CASES:
            with self.subTest(case=name):
                found = i18n.check_site(us2_repo(guide=change(FR_RICH)))
                only_page(self, found)
                for part in parts:
                    mentions(self, found, part)

    def test_us2_as2_a_translation_with_windows_line_endings_is_still_identical(self):
        root = us2_repo()
        path = i18n_file(root, "guide.md")
        path.write_bytes(path.read_text(encoding="utf-8").replace("\n", "\r\n").encode("utf-8"))
        self.assertEqual(i18n.check_site(root), [])

    def test_us2_as3_the_text_of_links_headings_and_alt_is_the_translators(self):
        guide = broke(broke(FR_RICH, "[le depot]", "[the repository, in any words]"), "![schema]", "![a different description]")
        self.assertEqual(i18n.check_site(us2_repo(guide=guide)), [])

    def test_us2_as3_the_unit_reports_a_different_anchor_list(self):
        english = "# A\n\n## B\n"
        self.assertEqual(i18n.check_identity("fr/a", english, "# A {#a}\n\n## B {#b}\n", ["a", "b"]), [])
        found = i18n.check_identity("fr/a", english, "# A {#a}\n\n## B {#c}\n", ["a", "b"])
        mentions(self, found, "fr/a", "anchor")

    def test_us2_as2_the_unit_numbers_the_blocks_from_one_and_ignores_prose(self):
        english = "Text.\n\n```sh\none\n```\n\nMore.\n\n```sh\ntwo\n```\n"
        self.assertEqual(i18n.check_identity("x/y", english, english.replace("Text.", "Texte.").replace("More.", "Plus."), []), [])
        mentions(self, i18n.check_identity("x/y", english, english.replace("two", "deux"), []), "x/y", "code block 2")


class T049NumbersAndStructure(unittest.TestCase):
    """US2-AS12, FR-009: the numbers of the page as a set of digit strings, and the counts of tables, list items and admonitions."""
    CASES = [
        ("a number changed", lambda t: broke(t, "3 etapes", "4 etapes"), ["number"]),
        ("a version changed", lambda t: broke(t, "venom 1.5", "venom 1.6"), ["number"]),
        ("a number added", lambda t: broke(t, "Termine en 2 minutes.", "Termine en 2 minutes (20 secondes)."), ["number", "20"]),
        ("a list item removed", lambda t: broke(t, "- troisieme element\n", ""), ["list item"]),
        ("a list item added", lambda t: broke(t, "- troisieme element\n", "- troisieme element\n- quatrieme element\n"), ["list item"]),
        ("the table removed", lambda t: broke(t, "| nom | valeur |\n|-----|--------|\n| a   | 1      |\n\n", ""), ["table"]),
        ("the admonition removed", lambda t: broke(t, "> **Note** : gardez le nom du fichier.\n\n", ""), ["admonition"]),
        ("an admonition added", lambda t: t + "\n> **Attention** : une autre note.\n", ["admonition"]),
    ]

    def test_us2_as12_each_case_fails_that_page_only_and_names_the_thing(self):
        for name, change, parts in self.CASES:
            with self.subTest(case=name):
                found = i18n.check_site(us2_repo(guide=change(FR_RICH)))
                only_page(self, found)
                for part in parts:
                    mentions(self, found, part)

    def test_us2_as12_the_numbers_are_a_set_not_a_sequence(self):
        guide = broke(FR_RICH, "Termine en 2 minutes.", "Termine en 2 minutes, voir 1 et 3, puis 5 et 2 encore.")
        self.assertEqual(i18n.check_site(us2_repo(guide=guide)), [])

    def test_us2_as12_the_explicit_anchors_of_a_translation_are_not_numbers(self):
        english = "# Step\n\n## Install 2\n"
        self.assertEqual(i18n.check_numbers("fr/step", english, "# Etape {#step}\n\n## Installer 2 {#install-2}\n"), [])

    def test_us2_as12_the_unit_names_both_counts_and_accepts_equal_ones(self):
        english = "- a\n- b\n\n| x | y |\n|---|---|\n| 1 | 2 |\n\n> note\n"
        self.assertEqual(i18n.check_numbers("fr/p", english, "- a\n- b\n\n| u | v |\n|---|---|\n| 1 | 2 |\n\n> remarque\n"), [])
        found = i18n.check_numbers("fr/p", english, english.replace("- b\n", "- b\n- c\n"))
        mentions(self, found, "fr/p", "list item", "2", "3")


class T050Glossary(unittest.TestCase):
    """US2-AS6, FR-013: the terms that stay English are still there where the English page has them; forbidden renderings are absent."""

    def found(self, guide: str) -> list[str]:
        return i18n.check_site(us2_repo(guide=guide))

    def test_us2_as6_a_page_that_keeps_its_terms_has_no_finding_and_a_term_english_lacks_is_not_required(self):
        self.assertIn("assertion", GLOSSARY["verbatim"])
        self.assertNotIn("assertion", RICH_EN.lower())
        self.assertEqual(self.found(FR_RICH), [])

    def test_us2_as6_a_verbatim_term_translated_away_fails_naming_the_term(self):
        found = self.found(broke(FR_RICH, "L'executor lance", "L'agent lance"))
        only_page(self, found)
        mentions(self, found, "executor")

    def test_us2_as6_a_forbidden_rendering_fails_even_beside_the_verbatim_term(self):
        found = self.found(broke(FR_RICH, "L'executor lance", "L'executor (executeur) lance"))
        only_page(self, found)
        mentions(self, found, "executeur")

    def test_us2_as6_the_forbidden_list_of_another_language_does_not_apply(self):
        self.assertEqual(self.found(broke(FR_RICH, "L'executor lance", "L'executor, Ausfuehrer en allemand, lance")), [])

    def test_us2_as6_the_term_is_found_whatever_its_case(self):
        self.assertEqual(self.found(broke(FR_RICH, "L'executor lance", "L'Executor lance")), [])

    def test_us2_as6_the_unit_checks_the_verbatim_terms_and_the_forbidden_words_of_the_language(self):
        self.assertEqual(i18n.check_glossary("fr/x", "fr", "an executor runs", "un executor lance", GLOSSARY), [])
        mentions(self, i18n.check_glossary("fr/x", "fr", "an executor runs", "un agent lance", GLOSSARY), "fr/x", "executor")
        mentions(self, i18n.check_glossary("fr/x", "fr", "it runs", "un executeur lance", GLOSSARY), "fr/x", "executeur")
        self.assertEqual(i18n.check_glossary("it/x", "it", "it runs", "un executeur lance", GLOSSARY), [])


class T051DraftStatusAndReview(unittest.TestCase):
    """US2-AS4, AS5, AS9, FR-011, FR-012: no page carries a draft banner (D-028), the rules of a `reviewed` file, and the `review` command."""

    def page_text(self, root: Path, rel: str) -> str:
        errors, out = build_all(root)
        self.assertEqual(errors, [])
        return read(out, rel)

    def test_us2_as4_a_machine_drafted_page_carries_no_banner_and_no_draft_text(self):
        errors, out = build_all(us2_repo())
        self.assertEqual(errors, [])
        for slug in DRAFTED_PAGES:
            rel = f"fr/{slug}.html"
            with self.subTest(page=rel):
                assert_no_draft_banner(self, read(out, rel))

    def test_us2_as4_no_page_of_the_site_carries_a_draft_banner_whatever_its_language(self):
        errors, out = standard_six()
        self.assertEqual(errors, [])
        for rel in html_files(out):
            with self.subTest(page=rel):
                assert_no_draft_banner(self, read(out, rel))

    def test_us2_as4_the_english_page_and_a_page_not_translated_carry_no_draft_banner_and_the_latter_keeps_its_fallback_notice(self):
        errors, out = build_all(us2_repo())
        self.assertEqual(errors, [])
        for rel in ("guide.html", "other.html", "fr/other.html"):
            with self.subTest(page=rel):
                assert_no_draft_banner(self, read(out, rel))
        notices = doc_of(out, "fr/other.html").find(cls="fallback-notice")
        self.assertEqual(len(notices), 1, "one element of class fallback-notice")
        self.assertEqual([urljoin(SITE_BASE + "fr/other.html", href) for href in notices[0]["hrefs"]], [page_url("en", "other")])

    def test_us2_as4_the_translated_home_page_carries_no_banner(self):
        root = us2_repo(home=True)
        assert_no_draft_banner(self, self.page_text(root, "fr/index.html"))
        assert_no_draft_banner(self, self.page_text(root, "index.html"))

    def test_us2_as5_a_reviewed_page_and_a_drafted_page_look_alike_as_to_the_banner(self):
        root = us2_repo()
        hand_review(root, "guide")
        assert_no_draft_banner(self, self.page_text(root, "fr/guide.html"))
        assert_no_draft_banner(self, self.page_text(root, "fr/overview.html"))

    def test_us2_as4_a_stale_page_carries_no_banner(self):
        root = us2_repo()
        (root / "site" / "pages" / "guide.md").write_text(broke(RICH_EN, "Done in 2 minutes.", "Done in 2 minutes, or 3."), encoding="utf-8")
        _, out = build_all(root)
        assert_no_draft_banner(self, read(out, "fr/guide.html"))

    def test_us2_as5_a_reviewed_file_with_its_reviewer_date_and_fingerprint_has_no_finding(self):
        root = us2_repo()
        hand_review(root)
        self.assertEqual(i18n.check_site(root), [])

    def test_us2_as5_a_reviewed_file_that_misses_or_gets_a_key_wrong_fails(self):
        cases = [("no reviewer", {"by": None}, "reviewed_by"), ("a reviewer who is not on the list", {"by": "Somebody Else"}, "Somebody Else"),
                 ("no date", {"on": None}, "reviewed_on"), ("a date that is not ISO", {"on": "18/11/2026"}, "reviewed_on"),
                 ("no reviewed fingerprint", {"fingerprint": None}, "reviewed_fingerprint"),
                 ("a reviewed fingerprint that is not today's", {"fingerprint": "0" * 64}, "reviewed_fingerprint")]
        for name, arguments, word in cases:
            root = us2_repo()
            hand_review(root, **arguments)
            with self.subTest(case=name):
                found = i18n.check_site(root)
                only_page(self, found)
                mentions(self, found, word)

    def test_us2_as5_an_empty_list_of_reviewers_accepts_nobody(self):
        root = us2_repo()
        hand_review(root)
        i18n_file(root, "reviewers.txt", "").write_text("\n", encoding="utf-8")
        mentions(self, i18n.check_site(root), "fr/guide", REVIEWER)

    def test_us2_as5_a_reviewed_file_goes_stale_when_the_english_text_changes_and_names_both_fingerprints(self):
        root = us2_repo()
        hand_review(root)
        recorded = header_of(root, "guide.md")["reviewed_fingerprint"]
        changed = broke(RICH_EN, "Done in 2 minutes.", "Done in 2 minutes, or 3.")
        (root / "site" / "pages" / "guide.md").write_text(changed, encoding="utf-8")
        found = i18n.check_site(root)
        only_page(self, found)
        mentions(self, found, "reviewed_fingerprint", recorded, current_fingerprint(changed, "Guide", "A guide."))

    def test_us2_as9_a_machine_drafted_file_carries_no_reviewed_key(self):
        for key, value in (("reviewed_by", REVIEWER), ("reviewed_on", TODAY), ("reviewed_fingerprint", "0" * 64)):
            root = us2_repo()
            edit(i18n_file(root, "guide.md"), "status: machine-drafted\n", f"status: machine-drafted\n{key}: {value}\n")
            with self.subTest(key=key):
                found = i18n.check_site(root)
                only_page(self, found)
                mentions(self, found, key)

    def test_us2_as5_the_unit_accepts_the_three_keys_and_names_the_one_at_fault(self):
        good = {"status": "reviewed", "reviewed_by": REVIEWER, "reviewed_on": TODAY, "reviewed_fingerprint": "f" * 64}
        self.assertEqual(i18n.check_review("fr/x", good, "f" * 64, [REVIEWER]), [])
        self.assertEqual(i18n.check_review("fr/x", {"status": "machine-drafted"}, "f" * 64, [REVIEWER]), [])
        mentions(self, i18n.check_review("fr/x", good, "e" * 64, [REVIEWER]), "fr/x", "reviewed_fingerprint")
        mentions(self, i18n.check_review("fr/x", good, "f" * 64, ["Another"]), "fr/x", REVIEWER)


class T051ReviewCommand(unittest.TestCase):
    """FR-012, US2-AS5, AS9: the `review` command is the only writer of the reviewed keys, and a re-draft takes them away."""

    def test_us2_as5_review_writes_the_reviewer_the_date_and_the_english_fingerprint_of_today(self):
        root = us2_repo()
        drafted = header_of(root, "guide.md")
        written = i18n.review(root, "fr", "guide", REVIEWER, TODAY)
        header = header_of(root, "guide.md")
        self.assertEqual((header["status"], header["reviewed_by"], header["reviewed_on"]), ("reviewed", REVIEWER, TODAY))
        self.assertEqual((header["reviewed_fingerprint"], written), (drafted["fingerprint"], drafted["fingerprint"]))
        self.assertEqual(i18n.check_site(root), [])

    def test_us2_as5_review_changes_nothing_else_of_the_file(self):
        root = us2_repo()
        before = i18n.parse_translation(i18n_file(root, "guide.md").read_text(encoding="utf-8"))
        i18n.review(root, "fr", "guide", REVIEWER, TODAY)
        after = i18n.parse_translation(i18n_file(root, "guide.md").read_text(encoding="utf-8"))
        self.assertEqual(after.body, before.body)
        for key, value in before.header.items():
            if key != "status":
                self.assertEqual(after.header[key], value, key)
        self.assertNotIn(b"\r", i18n_file(root, "guide.md").read_bytes())

    def test_us2_as5_a_reviewer_who_is_not_on_the_list_is_refused_and_nothing_is_written(self):
        root = us2_repo()
        before = i18n_file(root, "guide.md").read_bytes()
        with self.assertRaises(i18n.I18nError) as raised:
            i18n.review(root, "fr", "guide", "Somebody Else", TODAY)
        self.assertIn("Somebody Else", str(raised.exception))
        self.assertEqual(i18n_file(root, "guide.md").read_bytes(), before)

    def test_us2_as5_a_draft_made_from_an_older_english_text_cannot_be_reviewed(self):
        root = us2_repo()
        before = i18n_file(root, "guide.md").read_bytes()
        (root / "site" / "pages" / "guide.md").write_text(broke(RICH_EN, "Done in 2 minutes.", "Done in 2 minutes, or 3."), encoding="utf-8")
        with self.assertRaises(i18n.I18nError):
            i18n.review(root, "fr", "guide", REVIEWER, TODAY)
        self.assertEqual(i18n_file(root, "guide.md").read_bytes(), before)

    def test_us2_as5_a_file_or_a_page_that_is_not_there_is_refused(self):
        root = us2_repo()
        for slug in ("other", "nowhere"):
            with self.subTest(slug=slug), self.assertRaises(i18n.I18nError):
                i18n.review(root, "fr", slug, REVIEWER, TODAY)

    def test_us2_as5_the_home_page_is_reviewed_in_both_of_its_files(self):
        root = us2_repo(home=True)
        i18n.review(root, "fr", "index", REVIEWER, TODAY)
        for name in ("index.md", "home.html"):
            with self.subTest(file=name):
                self.assertEqual((header_of(root, name)["status"], header_of(root, name)["reviewed_by"]), ("reviewed", REVIEWER))
        self.assertEqual(i18n.check_site(root), [])

    def test_us2_as9_a_re_draft_of_a_reviewed_file_falls_back_to_machine_drafted_and_the_page_has_no_banner_before_or_after(self):
        root = us2_repo()
        i18n.review(root, "fr", "guide", REVIEWER, TODAY)
        errors, out = build_all(root)
        self.assertEqual(errors, [])
        assert_no_draft_banner(self, read(out, "fr/guide.html"))
        i18n.stamp(root, "fr", "guide", "2026-11-19")
        header = header_of(root, "guide.md")
        self.assertEqual(header["status"], "machine-drafted")
        for key in i18n.REVIEW_KEYS:
            self.assertNotIn(key, header)
        errors, out = build_all(root)
        self.assertEqual(errors, [])
        assert_no_draft_banner(self, read(out, "fr/guide.html"))

    def test_us2_as9_the_command_line_reviews_and_refuses_in_the_same_way(self):
        root = us2_repo()
        before = i18n_file(root, "guide.md").read_bytes()
        stdout, stderr = io.StringIO(), io.StringIO()
        with contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
            refused = i18n.main(["review", "fr", "guide", "--by", "Nobody Here"], root=root)
        self.assertEqual(refused, 2, stderr.getvalue())
        self.assertIn("Nobody Here", stderr.getvalue())
        self.assertEqual(i18n_file(root, "guide.md").read_bytes(), before)
        with contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
            done = i18n.main(["review", "fr", "guide", "--by", REVIEWER], root=root)
        self.assertEqual(done, 0, stderr.getvalue())
        header = header_of(root, "guide.md")
        self.assertEqual((header["status"], header["reviewed_by"]), ("reviewed", REVIEWER))
        self.assertRegex(header["reviewed_on"], r"^\d{4}-\d{2}-\d{2}$")

    def test_us2_as9_the_stamp_command_never_writes_a_reviewed_key(self):
        root = us2_repo()
        stdout, stderr = io.StringIO(), io.StringIO()
        with contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
            self.assertEqual(i18n.main(["stamp", "fr", "guide"], root=root), 0, stderr.getvalue())
        for key in i18n.REVIEW_KEYS:
            self.assertNotIn(key, header_of(root, "guide.md"))


NO_RECORD = "reviewed but no record in reviews.json"
MALFORMED_REVIEWS = (("not JSON", "{not json"), ("an object, not a list", '{"language": "fr"}'), ("a list of non-objects", "[1, 2]"))


def built_page(root: Path, rel: str = "fr/guide.html") -> str:
    _, out = build_all(root)
    return read(out, rel)


def recorded(root: Path) -> list[dict]:
    if not reviews_file(root).is_file():
        raise AssertionError("site/i18n/reviews.json was not written")
    return json.loads(reviews_file(root).read_text(encoding="utf-8"))


class T135ReviewRecord(unittest.TestCase):
    """D-024, FR-012, FR-024, T-54: `site/i18n/reviews.json`, a JSON list of `{language, slug, fingerprint, reviewer, date}`, is written
    only by the `review` command. A page is reviewed only when its file says `reviewed` AND a record matches it (same language, slug,
    reviewer as `reviewed_by`, fingerprint as `reviewed_fingerprint`); the file alone, or a record that does not match, is a finding,
    does not count for the gate or the status (and no page has a draft banner, D-028)."""

    # -- the `review` command writes the record
    def test_d_024_review_writes_one_record_with_the_six_keys_and_a_trailing_newline(self):
        # changed from "the five keys": the record now also holds the translation's own fingerprint (D-024 amended)
        root = us2_repo()
        fingerprint = i18n.review(root, "fr", "guide", REVIEWER, TODAY)
        self.assertEqual(recorded(root), [{"language": "fr", "slug": "guide", "fingerprint": fingerprint,
                                           "translation_fingerprint": translation_fingerprint_of(root, "guide"), "reviewer": REVIEWER, "date": TODAY}])
        self.assertTrue(reviews_file(root).read_text(encoding="utf-8").endswith("\n"))
        self.assertNotIn(b"\r", reviews_file(root).read_bytes())

    def test_d_024_reviewing_again_replaces_the_record_of_that_page_and_keeps_one(self):
        root = us2_repo()
        i18n.review(root, "fr", "guide", REVIEWER, TODAY)
        i18n.review(root, "fr", "guide", REVIEWER, "2026-11-19")
        self.assertEqual([(item["slug"], item["date"]) for item in recorded(root)], [("guide", "2026-11-19")])

    def test_d_024_the_records_are_kept_sorted_by_language_then_slug_whatever_the_order_of_the_reviews(self):
        root = us2_repo()
        for slug in ("installing", "guide", "overview"):
            i18n.review(root, "fr", slug, REVIEWER, TODAY)
        self.assertEqual([item["slug"] for item in recorded(root)], ["guide", "installing", "overview"])
        before = reviews_file(root).read_bytes()
        i18n.review(root, "fr", "guide", REVIEWER, TODAY)
        self.assertEqual(reviews_file(root).read_bytes(), before, "the same review again changes no byte")

    def test_d_024_the_home_page_has_one_record_for_its_two_files(self):
        root = us2_repo(home=True)
        i18n.review(root, "fr", "index", REVIEWER, TODAY)
        self.assertEqual([(item["language"], item["slug"]) for item in recorded(root)], [("fr", "index")])

    def test_d_024_a_refused_review_writes_no_record(self):
        root = us2_repo()
        with self.assertRaises(i18n.I18nError):
            i18n.review(root, "fr", "guide", "Somebody Else", TODAY)
        (root / "site" / "pages" / "guide.md").write_text(broke(RICH_EN, "Done in 2 minutes.", "Done in 2 minutes, or 3."), encoding="utf-8")
        with self.assertRaises(i18n.I18nError):
            i18n.review(root, "fr", "guide", REVIEWER, TODAY)
        self.assertFalse(reviews_file(root).exists())

    def test_d_024_a_review_keeps_the_records_of_the_other_pages_and_the_header_update_as_before(self):
        root = us2_repo()
        i18n.review(root, "fr", "overview", REVIEWER, TODAY)
        i18n.review(root, "fr", "guide", REVIEWER, "2026-11-19")
        self.assertEqual(sorted((item["slug"], item["date"]) for item in recorded(root)), [("guide", "2026-11-19"), ("overview", TODAY)])
        self.assertEqual((header_of(root, "guide.md")["status"], header_of(root, "guide.md")["reviewed_on"]), ("reviewed", "2026-11-19"))
        self.assertEqual(i18n.check_site(root), [])

    # -- a `reviewed` status with no matching record is not a review
    def test_d_024_a_reviewed_file_with_no_reviews_json_is_a_finding_that_names_the_page_and_the_missing_record(self):
        root = us2_repo()
        hand_review(root, record=False)
        self.assertFalse(reviews_file(root).exists())
        found = i18n.check_site(root)
        only_page(self, found)
        mentions(self, found, "fr/guide", NO_RECORD)

    def test_d_024_a_record_that_does_not_match_the_page_is_no_record(self):
        other = {"language": "fr", "slug": "guide", "fingerprint": "today", "translation_fingerprint": "own", "reviewer": REVIEWER, "date": TODAY}
        cases = (("another page", {"slug": "overview"}), ("another language", {"language": "de"}),
                 ("another fingerprint", {"fingerprint": "0" * 64}), ("another reviewer", {"reviewer": "Second Reviewer"}),
                 ("no reviewer key", {"reviewer": None}), ("no fingerprint key", {"fingerprint": None}),
                 ("another translation fingerprint", {"translation_fingerprint": "0" * 64}),
                 ("no translation fingerprint key", {"translation_fingerprint": None}))
        for name, change in cases:
            root = us2_repo()
            i18n_file(root, "reviewers.txt", "").write_text(f"{REVIEWER}\nSecond Reviewer\n", encoding="utf-8")
            hand_review(root, record=False)
            entry = {**other, "fingerprint": header_of(root, "guide.md")["fingerprint"],
                     "translation_fingerprint": translation_fingerprint_of(root, "guide"), **change}
            write_reviews(root, {key: value for key, value in entry.items() if value is not None})
            with self.subTest(case=name):
                found = i18n.check_site(root)
                mentions(self, found, "fr/guide", NO_RECORD)

    def test_d_024_a_record_for_one_page_does_not_vouch_for_a_forged_review_of_another(self):
        root = us2_repo()
        i18n.review(root, "fr", "overview", REVIEWER, TODAY)
        hand_review(root, "guide", record=False)
        found = i18n.check_site(root)
        only_page(self, found)
        mentions(self, found, "fr/guide", NO_RECORD)

    def test_d_024_a_reviewed_file_with_its_matching_record_has_no_finding(self):
        root = us2_repo()
        hand_review(root)
        self.assertEqual(i18n.check_site(root), [])

    # -- the translation's own text is part of the record (D-024 amended): a body edited after `review` is no longer reviewed
    def test_d_024_review_records_the_sha256_of_the_translation_body_and_not_the_header(self):
        root = us2_repo()
        i18n.review(root, "fr", "guide", REVIEWER, TODAY)
        value = recorded(root)[0]["translation_fingerprint"]
        self.assertRegex(value, r"^[0-9a-f]{64}$")
        self.assertEqual(value, translation_fingerprint_of(root, "guide"))
        self.assertNotEqual(value, recorded(root)[0]["fingerprint"], "the English fingerprint and the translation's are different things")

    def test_d_024_the_home_page_record_covers_its_second_file(self):
        root = us2_repo(home=True)
        i18n.review(root, "fr", "index", REVIEWER, TODAY)
        before = recorded(root)[0]["translation_fingerprint"]
        self.assertRegex(before, r"^[0-9a-f]{64}$")
        append_body(i18n_file(root, "home.html"), "<!-- une note -->\n")
        i18n.review(root, "fr", "index", REVIEWER, TODAY)
        self.assertNotEqual(recorded(root)[0]["translation_fingerprint"], before, "home.html is part of the page's translation text")
        append_body(i18n_file(root, "index.md"), "\nUne phrase de plus.\n")
        i18n.review(root, "fr", "index", REVIEWER, TODAY)
        self.assertNotEqual(recorded(root)[0]["translation_fingerprint"], before)

    def test_d_024_a_body_edited_after_review_is_a_finding_that_names_the_page_and_the_missing_record(self):
        root = us2_repo()
        i18n.review(root, "fr", "guide", REVIEWER, TODAY)
        self.assertEqual(i18n.check_site(root), [])
        append_body(i18n_file(root, "guide.md"), "\nUne phrase ajoutee apres la relecture.\n")
        mentions(self, i18n.check_site(root), "fr/guide", NO_RECORD)

    def test_d_024_a_body_edited_after_review_is_not_counted_as_reviewed_by_the_status(self):
        root = us2_repo()
        i18n.review(root, "fr", "guide", REVIEWER, TODAY)
        append_body(i18n_file(root, "guide.md"), "\nUne phrase ajoutee apres la relecture.\n")
        counts = i18n.status(root)["fr"]
        self.assertEqual((counts["reviewed"], counts["machine-drafted"], counts["stale"]), (0, 3, 0))

    def test_d_024_a_core_page_edited_after_review_is_not_reviewed_for_the_gate(self):
        root = gate_repo()
        self.assertEqual(i18n.check_enable_gate(root, "fr"), [])
        append_body(i18n_file(root, "installing.md"), "\nUne phrase ajoutee apres la relecture.\n")
        found = i18n.check_enable_gate(root, "fr")
        mentions(self, found, "fr/installing", "core page not reviewed")

    def test_d_024_a_home_template_edited_after_review_is_a_finding_too(self):
        root = us2_repo(home=True)
        i18n.review(root, "fr", "index", REVIEWER, TODAY)
        append_body(i18n_file(root, "home.html"), "<!-- ajoute apres la relecture -->\n")
        mentions(self, i18n.check_site(root), "fr/index", NO_RECORD)

    def test_d_024_reviewing_again_after_the_edit_makes_the_page_reviewed_again(self):
        root = us2_repo()
        i18n.review(root, "fr", "guide", REVIEWER, TODAY)
        append_body(i18n_file(root, "guide.md"), "\nUne phrase ajoutee apres la relecture.\n")
        old = recorded(root)[0]["translation_fingerprint"]
        i18n.review(root, "fr", "guide", REVIEWER, "2026-11-19")
        self.assertNotEqual(recorded(root)[0]["translation_fingerprint"], old)
        self.assertEqual([finding for finding in i18n.check_site(root) if NO_RECORD in finding], [])
        counts = i18n.status(root)["fr"]
        self.assertEqual((counts["reviewed"], counts["machine-drafted"]), (1, 2))

    def test_d_024_a_record_written_before_the_key_existed_does_not_match(self):
        root = us2_repo()
        i18n.review(root, "fr", "guide", REVIEWER, TODAY)
        legacy = [{key: value for key, value in item.items() if key != "translation_fingerprint"} for item in recorded(root)]
        write_reviews(root, *legacy)
        mentions(self, i18n.check_site(root), "fr/guide", NO_RECORD)
        self.assertEqual(i18n.status(root)["fr"]["reviewed"], 0)

    def test_d_024_a_machine_drafted_file_needs_no_record_and_no_finding_names_reviews_json(self):
        self.assertEqual(i18n.check_site(us2_repo()), [])

    # -- a malformed reviews.json is a finding, never a crash, and excuses nothing
    def test_d_024_a_malformed_reviews_json_is_a_finding_that_names_the_file(self):
        for name, text in MALFORMED_REVIEWS:
            root = us2_repo()
            reviews_file(root).write_text(text, encoding="utf-8")
            with self.subTest(case=name):
                mentions(self, i18n.check_site(root), "reviews.json")

    def test_d_024_a_malformed_reviews_json_excuses_no_forged_review(self):
        for name, text in MALFORMED_REVIEWS:
            root = us2_repo()
            hand_review(root, record=False)
            reviews_file(root).write_text(text, encoding="utf-8")
            with self.subTest(case=name):
                found = i18n.check_site(root)
                mentions(self, found, "reviews.json")
                mentions(self, found, "fr/guide", NO_RECORD)

    # -- no draft banner on a page, forged review or not (D-028)
    def test_d_028_a_forged_review_is_a_finding_and_its_page_carries_no_banner(self):
        root = us2_repo()
        hand_review(root, record=False)
        mentions(self, i18n.check_site(root), "fr/guide", NO_RECORD)
        assert_no_draft_banner(self, built_page(root))

    def test_d_028_a_malformed_reviews_json_leaves_a_forged_review_a_finding_and_the_page_without_a_banner(self):
        for name, text in MALFORMED_REVIEWS:
            root = us2_repo()
            hand_review(root, record=False)
            reviews_file(root).write_text(text, encoding="utf-8")
            with self.subTest(case=name):
                mentions(self, i18n.check_site(root), "fr/guide", NO_RECORD)
                assert_no_draft_banner(self, built_page(root))

    def test_d_028_a_page_reviewed_by_the_command_and_a_drafted_page_carry_no_banner(self):
        root = us2_repo()
        i18n.review(root, "fr", "guide", REVIEWER, TODAY)
        assert_no_draft_banner(self, built_page(root))
        assert_no_draft_banner(self, built_page(root, "fr/overview.html"))

    # -- the status counts
    def test_d_024_the_status_does_not_count_a_reviewed_file_with_no_record_as_reviewed(self):
        root = us2_repo()
        hand_review(root, record=False)
        counts = i18n.status(root)["fr"]
        self.assertEqual((counts["reviewed"], counts["machine-drafted"], counts["stale"]), (0, 3, 0))

    def test_d_024_the_status_counts_a_reviewed_file_with_its_record(self):
        root = us2_repo()
        i18n.review(root, "fr", "guide", REVIEWER, TODAY)
        counts = i18n.status(root)["fr"]
        self.assertEqual((counts["reviewed"], counts["machine-drafted"], counts["stale"]), (1, 2, 0))

    # -- the enable gate
    def test_d_024_a_core_page_with_the_reviewed_status_but_no_record_is_not_reviewed_for_the_gate(self):
        root = gate_repo()
        kept = recorded(root) if reviews_file(root).is_file() else []
        write_reviews(root, *[item for item in kept if item["slug"] != "quickstart"])
        found = i18n.check_enable_gate(root, "fr")
        only_page(self, found, "fr/quickstart")
        mentions(self, found, "fr/quickstart", "core page not reviewed")
        self.assertEqual(len(found), 1, found)

    def test_d_024_with_no_reviews_json_every_core_page_is_not_reviewed_and_the_build_refuses(self):
        root = gate_repo()
        reviews_file(root).unlink(missing_ok=True)
        found = i18n.check_enable_gate(root, "fr")
        self.assertEqual(len(found), len(CORE_IN_SCRATCH), found)
        code, text = run_main(root, root / "_cli")
        self.assertNotEqual(code, 0, text)
        self.assertIn("fr/installing: core page not reviewed", text)

    def test_d_024_a_malformed_reviews_json_makes_no_core_page_reviewed_and_does_not_crash_the_gate(self):
        for name, text in MALFORMED_REVIEWS:
            root = gate_repo()
            reviews_file(root).write_text(text, encoding="utf-8")
            with self.subTest(case=name):
                self.assertEqual(len(i18n.check_enable_gate(root, "fr")), len(CORE_IN_SCRATCH))

    # -- guards: what already held still holds (green today)
    def test_guard_a_stale_reviewed_page_is_still_stale_and_names_both_fingerprints(self):
        root = us2_repo()
        i18n.review(root, "fr", "guide", REVIEWER, TODAY)
        recorded_fingerprint = header_of(root, "guide.md")["reviewed_fingerprint"]
        changed = broke(RICH_EN, "Done in 2 minutes.", "Done in 2 minutes, or 3.")
        (root / "site" / "pages" / "guide.md").write_text(changed, encoding="utf-8")
        found = i18n.check_site(root)
        only_page(self, found)
        mentions(self, found, "reviewed_fingerprint", recorded_fingerprint, current_fingerprint(changed, "Guide", "A guide."))
        self.assertEqual(i18n.status(root)["fr"]["stale"], 1)

    def test_guard_a_reviewer_who_is_not_on_the_list_is_still_a_finding_and_no_review_for_the_gate(self):
        root = gate_repo()
        (root / "site" / "i18n" / "reviewers.txt").write_text("Somebody Else\n", encoding="utf-8")
        mentions(self, i18n.check_site(root), "fr/installing", REVIEWER)
        self.assertEqual(len(i18n.check_enable_gate(root, "fr")), len(CORE_IN_SCRATCH))

    def test_guard_the_14_day_override_still_excuses_a_reviewed_core_page_whose_english_text_changed(self):
        root = gate_repo()
        edit(root / "README.md", "Install the thing.", "Install the thing now.")
        write_overrides(root, override(slug="installing"))
        self.assertEqual(i18n.check_enable_gate(root, "fr", today=DURING), [])
        self.assertEqual(i18n.check_site(root, today=DURING), [])

    def test_guard_an_override_of_more_than_14_days_excuses_nothing_at_the_gate(self):
        root = gate_repo()
        edit(root / "README.md", "Install the thing.", "Install the thing now.")
        write_overrides(root, override(slug="installing", expires="2026-12-01"))
        mentions(self, i18n.check_enable_gate(root, "fr", today=DURING), "fr/installing", "not reviewed")

    def test_d_024_the_override_does_not_excuse_a_forged_review(self):
        root = stale_guide_repo(override())
        hand_review(root, record=False)
        mentions(self, i18n.check_site(root, today=DURING), "fr/guide", NO_RECORD)


class T052MissingAndUnsafeTranslations(unittest.TestCase):
    """US2-AS7, AS8, FR-014, FR-018: missing pages are counted and fail only in complete mode; a translation cannot bring markup,
    a script, a remote address or a private path into the site."""

    def test_us2_as7_missing_pages_are_listed_counted_and_pass_by_default(self):
        root = us2_repo()
        self.assertEqual(i18n.missing_pages(root, "fr"), ["index", "other"])
        self.assertEqual(i18n.status(root)["fr"]["missing"], 2)
        self.assertEqual(i18n.check_site(root), [])

    def test_us2_as7_in_complete_mode_each_missing_page_fails_by_name(self):
        root = us2_repo()
        set_complete(root)
        found = i18n.check_site(root)
        mentions(self, found, "fr/index", "missing")
        mentions(self, found, "fr/other", "missing")
        self.assertEqual([finding for finding in found if not finding.startswith(("fr/index", "fr/other"))], [])

    def test_us2_as7_in_complete_mode_a_language_with_every_page_passes(self):
        root = us2_repo(home=True)
        set_complete(root)
        self.assertEqual(i18n.missing_pages(root, "fr"), [])
        self.assertEqual(i18n.check_site(root), [])

    def test_us2_as7_a_language_with_no_file_at_all_has_every_page_missing(self):
        languages = {**LANGUAGES, "de": {"name": "Deutsch", "path": "de", "lang": "de", "enabled": False}}
        root = us2_repo(languages=languages)
        self.assertEqual(i18n.missing_pages(root, "de"), SLUGS)
        self.assertEqual(i18n.status(root)["de"], {"machine-drafted": 0, "reviewed": 0, "stale": 0, "missing": len(SLUGS)})
        self.assertEqual(i18n.check_site(root), [])

    def test_us2_as8_raw_html_a_script_and_a_remote_image_are_escaped_in_the_page_and_the_build_stays_clean(self):
        hostile = ("\n<script>alert(1)</script>\n\n<img src=\"https://evil.example/p.png\">\n\n"
                   "<iframe src=\"https://evil.example/f\"></iframe>\n")
        errors, out = build_all(us2_repo(guide=FR_RICH + hostile))
        page = read(out, "fr/guide.html")
        for raw in ("<script>alert", "<img src=\"https://evil", "<iframe"):
            self.assertNotIn(raw, page)
        self.assertIn("&lt;script&gt;", page)
        self.assertEqual(errors, [])

    def test_us2_as8_the_same_hostile_text_fails_the_gate_for_what_english_lacks(self):
        hostile = "\n<script>alert(1)</script>\n\n<img src=\"https://evil.example/p.png\">\n"
        found = i18n.check_site(us2_repo(guide=FR_RICH + hostile))
        only_page(self, found)
        mentions(self, found, "evil.example")

    def test_us2_as8_a_link_to_a_private_path_is_refused_by_the_build_and_by_the_gate(self):
        guide = FR_RICH + "\nVoir [la note](docs/secret.md).\n"
        errors, _ = build_all(us2_repo(guide=guide))
        self.assertTrue(any("docs/secret.md" in error for error in errors), errors)
        mentions(self, i18n.check_site(us2_repo(guide=guide)), "fr/guide", "link")

    def test_us2_as8_a_remote_address_in_a_plain_text_line_of_the_home_page_fails_the_gate(self):
        found = self.home_found(lambda text: broke(text, "<p class=\"lead\">", "<p class=\"lead\">https://evil.example/home "))
        mentions(self, found, "fr/index", "evil.example")

    @staticmethod
    def home_found(change) -> list[str]:
        files = us2_files(home=True)
        files["site/i18n/fr/home.html"] = HOME_HEADER + change(ENGLISH_HOME)
        return i18n.check_site(us2_repo(files=files, home=True))


class T052HomeTemplate(unittest.TestCase):
    """US3-AS1, FR-009, FR-018, FR-020: `home.html` of a language has the English page's code, addresses and figures."""
    CASES = [
        ("a link address changed", lambda t: broke(t, 'href="quickstart.html"', 'href="quickstart-fr.html"'), ["home.html"]),
        ("a link English lacks", lambda t: broke(t, "<h2>About this fork</h2>", '<h2>About <a href="https://evil.example/">this</a> fork</h2>'), ["evil.example"]),
        ("a character of the demo block", lambda t: broke(t, 'echo "hello venom"', 'echo "bonjour venom"'), ["home.html"]),
        ("a command of the build block", lambda t: broke(t, "go build -o venom", "go build -o venin"), ["home.html"]),
        ("an inline command", lambda t: broke(t, "<code>--parallel</code>", "<code>--parallele</code>"), ["home.html"]),
        ("a figure", lambda t: broke(t, "213 commits", "214 commits"), ["number"]),
        ("a figure dropped", lambda t: broke(t, "13&times; faster", "faster"), ["number"]),
        ("a script", lambda t: broke(t, "</body>", "<script>alert(1)</script></body>"), ["script"]),
    ]

    def test_us3_as1_each_case_fails_the_home_page_only_and_names_home_html(self):
        for name, change, parts in self.CASES:
            files = us2_files(home=True)
            files["site/i18n/fr/home.html"] = HOME_HEADER + change(ENGLISH_HOME)
            with self.subTest(case=name):
                found = i18n.check_site(us2_repo(files=files, home=True))
                only_page(self, found, "fr/index")
                for part in parts:
                    mentions(self, found, part)

    def test_us3_as1_a_home_page_with_the_english_figures_and_code_and_other_words_has_no_finding(self):
        words = broke(broke(ENGLISH_HOME, "About this fork", "A propos de ce fork"), "Someone is on it", "Quelqu'un s'en occupe")
        files = {**us2_files(home=True), "site/i18n/fr/home.html": HOME_HEADER + words}
        self.assertEqual(i18n.check_site(us2_repo(files=files, home=True)), [])

    def test_us3_as1_the_unit_compares_the_template_with_the_english_one(self):
        self.assertEqual(i18n.check_template("fr/index", ENGLISH_HOME, ENGLISH_HOME), [])
        mentions(self, i18n.check_template("fr/index", ENGLISH_HOME, broke(ENGLISH_HOME, "213 commits", "214 commits")), "fr/index", "home.html")


class T053StatusCommand(unittest.TestCase):
    """US4-AS4, FR-019: one command prints, per language, the pages machine-drafted, reviewed, stale and missing."""

    def mixed(self) -> Path:
        """fr: overview drafted, guide reviewed, installing stale (its English text changed), index and other missing."""
        root = us2_repo()
        hand_review(root, "guide")
        edit(root / "README.md", "Install the thing.", "Install the thing now.")
        return root

    def test_us4_as4_the_counts_of_a_language_are_the_four_figures_and_each_page_counts_once(self):
        counts = i18n.status(self.mixed())
        self.assertEqual(list(counts), ["fr"])
        self.assertEqual(counts["fr"], {"machine-drafted": 1, "reviewed": 1, "stale": 1, "missing": 2})
        self.assertEqual(sum(counts["fr"].values()), len(SLUGS))

    def test_us4_as4_a_reviewed_page_whose_english_text_changed_counts_as_stale_not_reviewed(self):
        root = us2_repo()
        hand_review(root, "guide")
        (root / "site" / "pages" / "guide.md").write_text(broke(RICH_EN, "Done in 2 minutes.", "Done in 2 minutes, or 3."), encoding="utf-8")
        counts = i18n.status(root)["fr"]
        self.assertEqual((counts["reviewed"], counts["stale"], counts["machine-drafted"]), (0, 1, 2))

    def test_us4_as4_a_language_with_no_folder_is_all_missing_and_every_declared_language_is_listed(self):
        languages = {**LANGUAGES, "de": {"name": "Deutsch", "path": "de", "lang": "de", "enabled": False}}
        counts = i18n.status(us2_repo(languages=languages))
        self.assertEqual(list(counts), ["fr", "de"])
        self.assertEqual(counts["de"], {"machine-drafted": 0, "reviewed": 0, "stale": 0, "missing": len(SLUGS)})

    @staticmethod
    def figures(text: str, lang: str) -> dict[str, int]:
        lines = [line for line in text.splitlines() if line.split()[:1] == [lang]]
        if len(lines) != 1:
            raise AssertionError(f"expected one line for {lang}: {text!r}")
        return {word: int(number) for word, number in re.findall(r"(machine-drafted|reviewed|stale|missing)\D+?(\d+)", lines[0])}

    def test_us4_as4_the_printed_report_has_one_line_per_language_with_the_four_words_and_figures(self):
        counts = {"fr": {"machine-drafted": 3, "reviewed": 2, "stale": 1, "missing": 37}, "zh": {"machine-drafted": 0, "reviewed": 0, "stale": 0, "missing": 43}}
        text = i18n.render_status(counts)
        self.assertEqual(self.figures(text, "fr"), counts["fr"])
        self.assertEqual(self.figures(text, "zh"), counts["zh"])
        self.assertEqual(len(text.strip().splitlines()), 2)

    def test_us4_as4_the_command_prints_the_report_of_the_repository_and_exits_zero(self):
        root = self.mixed()
        stdout, stderr = io.StringIO(), io.StringIO()
        with contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
            code = i18n.main(["status"], root=root)
        self.assertEqual(code, 0, stderr.getvalue())
        self.assertEqual(self.figures(stdout.getvalue(), "fr"), {"machine-drafted": 1, "reviewed": 1, "stale": 1, "missing": 2})

    def test_us4_as4_make_has_a_target_that_runs_the_status_command(self):
        recipe = make_targets((ROOT / "Makefile").read_text(encoding="utf-8")).get("site-i18n-status", "")
        self.assertRegex(recipe, r"site_i18n\.py\s+status", "no target site-i18n-status in the Makefile")


# ---------- User Story 5: the enable gate, the dated override and the llms files (T122 to T124) ----------
# The scratch site has four of the seven core pages (index, quickstart, overview, installing); the gate judges the core pages the site has.

CORE_IN_SCRATCH = ("index", "quickstart", "overview", "installing")
WITH_GERMAN = {**LANGUAGES, "de": {"name": "Deutsch", "path": "de", "lang": "de", "enabled": True}}


def gate_repo(unreviewed: tuple = (), languages=None) -> Path:
    """French enabled, its core pages of the scratch site drafted and then reviewed, except the ones named."""
    root = us2_repo(home=True, languages=languages)
    for slug in CORE_IN_SCRATCH:
        if slug not in unreviewed:
            i18n.review(root, "fr", slug, REVIEWER, TODAY)
    return root


def build_errors(root: Path, **options) -> list[str]:
    try:
        return bs.build_languages(root, root / "_out", **options)
    except bs.BuildError as error:
        return [str(error)]


class T122EnableGate(unittest.TestCase):
    """US5-AS4, FR-024, D-008: a language is built only when the maker enabled it, and an enabled language needs its core pages
    reviewed. `check_enable_gate` is the unit; the build and the command line refuse through it."""

    def test_us5_as4_the_core_pages_are_the_seven_of_the_spec(self):
        self.assertEqual(i18n.CORE_PAGES, ("index", "quickstart", "overview", "installing", "updating", "docker", "first-test"))

    def test_us5_as4_a_language_with_every_core_page_reviewed_has_no_finding_and_is_built(self):
        root = gate_repo()
        self.assertEqual(i18n.check_enable_gate(root, "fr"), [])
        self.assertEqual(build_errors(root), [])
        self.assertTrue((root / "_out" / "fr" / "guide.html").is_file())

    def test_us5_as4_a_missing_core_page_is_one_finding_that_names_it(self):
        for slug in CORE_IN_SCRATCH:
            root = gate_repo()
            i18n_file(root, f"{slug}.md").unlink()
            with self.subTest(page=slug):
                found = i18n.check_enable_gate(root, "fr")
                only_page(self, found, f"fr/{slug}")
                mentions(self, found, f"fr/{slug}", "missing")
                self.assertEqual(len(found), 1, found)

    def test_us5_as4_a_core_page_that_is_not_reviewed_is_one_finding_that_names_it(self):
        for slug in CORE_IN_SCRATCH:
            root = gate_repo(unreviewed=(slug,))
            with self.subTest(page=slug):
                found = i18n.check_enable_gate(root, "fr")
                only_page(self, found, f"fr/{slug}")
                mentions(self, found, f"fr/{slug}", "not reviewed")
                self.assertEqual(len(found), 1, found)

    def test_us5_as4_every_page_at_fault_is_named_not_only_the_first(self):
        found = i18n.check_enable_gate(gate_repo(unreviewed=("overview", "installing")), "fr")
        mentions(self, found, "fr/overview")
        mentions(self, found, "fr/installing")
        self.assertEqual(len(found), 2, found)

    def test_us5_as4_a_reviewed_page_whose_english_text_changed_is_not_reviewed(self):
        root = gate_repo()
        edit(root / "README.md", "Install the thing.", "Install the thing now.")
        found = i18n.check_enable_gate(root, "fr")
        only_page(self, found, "fr/installing")
        mentions(self, found, "fr/installing", "not reviewed")

    def test_us5_as4_a_reviewer_who_is_not_on_the_list_does_not_count_as_a_review(self):
        root = gate_repo()
        (root / "site" / "i18n" / "reviewers.txt").write_text("Somebody Else\n", encoding="utf-8")
        found = i18n.check_enable_gate(root, "fr")
        self.assertEqual(len(found), len(CORE_IN_SCRATCH), found)

    def test_us5_as4_pages_outside_the_core_never_stop_a_language(self):
        root = gate_repo()
        i18n_file(root, "other.md").unlink()
        self.assertEqual(i18n.check_enable_gate(root, "fr"), [])
        self.assertEqual(header_of(root, "guide.md")["status"], "machine-drafted")

    # The gate is carried by the command line (`main`), the build the publish job and `make site` run (FR-024, D-017): the library
    # `build_languages` builds a language that is still in draft, so these two tests drive `bs.main`.
    def test_us5_as4_the_build_refuses_an_enabled_language_with_a_missing_core_page_and_names_it(self):
        for slug in CORE_IN_SCRATCH:
            root = gate_repo()
            i18n_file(root, f"{slug}.md").unlink()
            with self.subTest(page=slug):
                code, text = run_main(root, root / "_cli")
                self.assertNotEqual(code, 0, text)
                self.assertIn(f"fr/{slug}: core page missing", text)

    def test_us5_as4_the_build_refuses_an_enabled_language_with_an_unreviewed_core_page_and_names_it(self):
        for slug in CORE_IN_SCRATCH:
            root = gate_repo(unreviewed=(slug,))
            with self.subTest(page=slug):
                code, text = run_main(root, root / "_cli")
                self.assertNotEqual(code, 0, text)
                self.assertIn(f"fr/{slug}: core page not reviewed", text)

    def test_us5_as4_the_command_line_exits_with_an_error_and_names_the_page(self):
        root = gate_repo(unreviewed=("quickstart",))
        code, text = run_main(root, root / "_cli")
        self.assertNotEqual(code, 0, text)
        self.assertIn("quickstart", text)

    def test_us5_as4_a_disabled_language_is_neither_built_nor_listed_whatever_its_core_pages(self):
        """Characterization: D-014, the gate judges enabled languages only."""
        root = us2_repo(home=True, languages=DISABLED_FR)
        self.assertEqual(build_errors(root), [])
        out = root / "_out"
        self.assertFalse((out / "fr").exists())
        for rel in ("index.html", "guide.html", "llms.txt"):
            with self.subTest(file=rel):
                text = read(out, rel)
                self.assertNotIn("/fr/", text)
                self.assertNotIn('hreflang="fr"', text)
                self.assertNotIn("lang-menu", text)
                self.assertNotIn("lang-dropdown", text)

    def test_us5_as4_a_disabled_language_leaves_no_folder_in_the_output(self):
        """Characterization: a folder left by an earlier build is removed."""
        root = us2_repo(home=True, languages=DISABLED_FR)
        for rel in ("fr/old.html", "zh/old.html"):
            (root / "_out" / rel).parent.mkdir(parents=True, exist_ok=True)
            (root / "_out" / rel).write_text("old", encoding="utf-8")
        self.assertEqual(build_errors(root), [])
        self.assertFalse((root / "_out" / "fr").exists())
        self.assertFalse((root / "_out" / "zh").exists())

    def test_us5_as4_a_preview_builds_a_disabled_language_whatever_its_core_pages(self):
        """Characterization: D-017, the gate does not apply to a preview."""
        root = us2_repo(home=True, languages=DISABLED_FR)
        self.assertEqual(build_errors(root, preview_languages=("fr",)), [])
        self.assertTrue((root / "_out" / "fr" / "guide.html").is_file())


CREATED, DURING, EXPIRES, AFTER = "2026-11-16", "2026-11-20", "2026-11-30", "2026-12-01"
STALE_GUIDE = broke(RICH_EN, "Done in 2 minutes.", "Done in 2 minutes, really.")


def override(language: str = "fr", slug: str = "guide", reason: str = "A wrong install command is fixed in English first.",
             created: str = CREATED, expires: str = EXPIRES) -> dict:
    return {"language": language, "slug": slug, "reason": reason, "created": created, "expires": expires}


def write_overrides(root: Path, *entries: dict) -> None:
    (root / "site" / "i18n").mkdir(parents=True, exist_ok=True)
    (root / "site" / "i18n" / "overrides.json").write_text(json.dumps(list(entries)), encoding="utf-8")


def stale_guide_repo(*entries: dict, languages=None) -> Path:
    """The clean French set whose English `guide` page then changed (prose only: no digit, no code), with the overrides given."""
    root = us2_repo(languages=languages)
    (root / "site" / "pages" / "guide.md").write_text(STALE_GUIDE, encoding="utf-8")
    write_overrides(root, *entries)
    return root


def override_notices(root: Path, rel: str, today: str) -> list[dict]:
    errors = build_errors(root, today=today)
    if errors:
        raise AssertionError(errors)
    return doc_of(root / "_out", rel).find(cls="override-notice")


class T123Overrides(unittest.TestCase):
    """US5-AS5, FR-025, D-008: a dated override of at most 14 days lets a stale page pass, with a notice on the page, and the tests fail
    again after its expiry. `today` is a parameter; every test passes a fixed date."""

    def test_us5_as5_a_stale_page_with_no_override_fails(self):
        """Characterization: the stale check of US2 is unchanged by the `today` parameter."""
        found = i18n.check_site(stale_guide_repo(), today=DURING)
        only_page(self, found)
        mentions(self, found, "fr/guide")

    def test_us5_as5_an_unexpired_override_lets_the_stale_page_pass_on_every_day_up_to_its_expiry(self):
        root = stale_guide_repo(override())
        for today in (CREATED, DURING, EXPIRES):
            with self.subTest(today=today):
                self.assertEqual(i18n.check_site(root, today=today), [])

    def test_us5_as5_after_its_expiry_the_page_fails_again_and_the_finding_says_the_override_expired(self):
        found = i18n.check_site(stale_guide_repo(override()), today=AFTER)
        only_page(self, found)
        mentions(self, found, "fr/guide", "override", "expired")

    def test_us5_as5_an_override_excuses_the_page_and_the_language_it_names_and_no_other(self):
        root = stale_guide_repo(override())
        edit(root / "README.md", "Install the thing.", "Install the thing now.")
        found = i18n.check_site(root, today=DURING)
        only_page(self, found, "fr/installing")

    def test_us5_as5_an_override_for_another_language_does_not_excuse_the_page(self):
        found = i18n.check_site(stale_guide_repo(override(language="de"), languages=WITH_GERMAN), today=DURING)
        only_page(self, found)
        mentions(self, found, "fr/guide")

    def test_us5_as5_an_override_excuses_the_staleness_and_nothing_else_of_the_page(self):
        root = stale_guide_repo(override())
        edit(i18n_file(root, "guide.md"), "venom run demo.yml", "venom run demo.yaml")
        found = i18n.check_site(root, today=DURING)
        mentions(self, found, "fr/guide", "code block")
        self.assertEqual([finding for finding in found if "stale" in finding], [], "the staleness is excused")

    def test_us5_as5_a_reviewed_page_whose_english_text_changed_passes_under_an_override(self):
        root = us2_repo()
        i18n.review(root, "fr", "guide", REVIEWER, TODAY)
        (root / "site" / "pages" / "guide.md").write_text(STALE_GUIDE, encoding="utf-8")
        self.assertNotEqual(i18n.check_site(root, today=DURING), [])
        write_overrides(root, override())
        self.assertEqual(i18n.check_site(root, today=DURING), [])

    def test_us5_as5_an_override_of_more_than_14_days_is_refused_and_excuses_nothing(self):
        root = stale_guide_repo(override(expires="2026-12-01"))
        found = i18n.check_site(root, today=DURING)
        mentions(self, found, "fr/guide", "14")
        mentions(self, found, "fr/guide", "stale")

    def test_us5_as5_an_override_of_exactly_14_days_is_accepted(self):
        root = stale_guide_repo(override(created="2026-11-16", expires="2026-11-30"))
        self.assertEqual(i18n.check_overrides(root, DURING), [])

    def test_us5_as5_an_override_that_is_not_valid_is_refused_and_names_its_page_and_the_key_at_fault(self):
        incomplete = {key: value for key, value in override().items() if key != "reason"}
        cases = (("over 14 days", override(expires="2026-12-01"), "14"), ("no reason", incomplete, "reason"),
                 ("a blank reason", override(reason="   "), "reason"),
                 ("expires before it was created", override(created="2026-11-20", expires="2026-11-16"), "expires"),
                 ("created is not an ISO date", override(created="16/11/2026"), "created"),
                 ("expires is not an ISO date", override(expires="soon"), "expires"),
                 ("a page the site does not have", override(slug="nope"), "nope"),
                 ("a language the site does not have", override(language="xx"), "xx"))
        for name, entry, word in cases:
            root = us2_repo()
            write_overrides(root, entry)
            with self.subTest(case=name):
                found = i18n.check_overrides(root, DURING)
                only_page(self, found, f"{entry['language']}/{entry['slug']}")
                mentions(self, found, word)

    def test_us5_as5_no_file_an_empty_list_and_a_valid_entry_have_no_finding(self):
        root = us2_repo()
        self.assertEqual(i18n.check_overrides(root, DURING), [])
        write_overrides(root)
        self.assertEqual(i18n.check_overrides(root, DURING), [])
        write_overrides(root, override())
        self.assertEqual(i18n.check_overrides(root, DURING), [])

    def test_us5_as5_a_file_that_is_not_a_list_is_a_finding(self):
        root = us2_repo()
        (root / "site" / "i18n" / "overrides.json").parent.mkdir(parents=True, exist_ok=True)
        (root / "site" / "i18n" / "overrides.json").write_text('{"language": "fr"}', encoding="utf-8")
        mentions(self, i18n.check_overrides(root, DURING), "overrides.json")

    def test_us5_as5_an_entry_past_its_expiry_is_reported_as_expired(self):
        root = us2_repo()
        write_overrides(root, override())
        found = i18n.check_overrides(root, AFTER)
        mentions(self, found, "fr/guide", "expired")

    def test_us5_as5_the_page_shows_the_override_notice_in_its_language_while_the_override_lasts(self):
        root = stale_guide_repo(override())
        notices = override_notices(root, "fr/guide.html", DURING)
        self.assertEqual(len(notices), 1, "one element of class override-notice")
        self.assertIn(ui_of(root / "_out", "fr")["override_notice"], Doc.words(notices[0]))

    def test_us5_as5_only_the_page_under_override_shows_the_notice(self):
        root = stale_guide_repo(override())
        self.assertEqual(len(override_notices(root, "fr/guide.html", DURING)), 1)
        for rel in ("guide.html", "fr/overview.html", "fr/installing.html", "fr/other.html"):
            with self.subTest(page=rel):
                self.assertEqual(doc_of(root / "_out", rel).find(cls="override-notice"), [])

    def test_us5_as5_after_the_expiry_and_for_a_refused_override_the_page_shows_no_notice(self):
        """Characterization today (no page shows one); it must stay true once the notice exists."""
        self.assertEqual(override_notices(stale_guide_repo(override()), "fr/guide.html", AFTER), [])
        self.assertEqual(override_notices(stale_guide_repo(override(expires="2026-12-01")), "fr/guide.html", DURING), [])


TRANSLATED_IN_SIX = {"fr": ("overview", "installing", "guide"), "de": ("guide",), "it": ("guide",), "es": ("guide",), "zh": ("guide",)}
TITLES = {"index": "Home", "overview": "Overview", "installing": "Installing", "guide": "Guide", "other": "Other", "quickstart": "Quickstart"}
_LLMS_LINK = re.compile(r"^- \[([^\]]*)\]\((\S+?)\): ", re.M)


class T124LlmsPerLanguage(unittest.TestCase):
    """US5-AS3, FR-016, FR-026: the `llms.txt` of a language lists the pages it has translated, at its own addresses, and names the
    English source. Characterization: the build already does this (task T032); these tests keep it so."""

    @classmethod
    def setUpClass(cls):
        cls.errors, cls.out = standard_six()

    def test_us5_as3_each_language_lists_its_translated_pages_at_its_own_addresses_and_no_other(self):
        self.assertEqual(self.errors, [])
        for code, slugs in TRANSLATED_IN_SIX.items():
            with self.subTest(language=code):
                listed = [address for _, address in _LLMS_LINK.findall(read(self.out, f"{PATH[code]}/llms.txt"))]
                self.assertEqual(sorted(listed), sorted(page_url(code, slug) for slug in slugs))

    def test_us5_as3_the_listed_title_is_the_translated_one(self):
        for code, slugs in TRANSLATED_IN_SIX.items():
            with self.subTest(language=code):
                found = dict((address, title) for title, address in _LLMS_LINK.findall(read(self.out, f"{PATH[code]}/llms.txt")))
                for slug in slugs:
                    self.assertNotEqual(found[page_url(code, slug)], TITLES[slug])

    def test_us5_as3_each_language_names_the_english_source_before_its_list(self):
        for code in TRANSLATED_IN_SIX:
            with self.subTest(language=code):
                text = read(self.out, f"{PATH[code]}/llms.txt")
                source = text.find(SITE_BASE + "llms.txt")
                self.assertGreaterEqual(source, 0, "no address of the English llms.txt")
                self.assertLess(source, text.find("\n- ["))

    def test_us5_as3_the_full_text_of_a_language_holds_its_translated_pages_only_and_names_the_english_source(self):
        for code, slugs in TRANSLATED_IN_SIX.items():
            with self.subTest(language=code):
                text = read(self.out, f"{PATH[code]}/llms-full.txt")
                self.assertIn(SITE_BASE + "llms-full.txt", text)
                for slug in TITLES:
                    self.assertIsNone(re.search(rf"^# {TITLES[slug]}$", text, re.M), f"the English page {slug} is in the full text of {code}")
                self.assertEqual(len(re.findall(r"^# ", text, re.M)), len(slugs))

    def test_us5_as3_the_english_files_list_every_page_at_the_english_addresses_and_name_no_source(self):
        text = read(self.out, "llms.txt")
        self.assertEqual(sorted(address for _, address in _LLMS_LINK.findall(text)), sorted(page_url("en", slug) for slug in TITLES))
        self.assertNotIn("English version", text)
        self.assertNotIn("English version", read(self.out, "llms-full.txt"))


# ---------- review findings: a language folder is one safe name, the gate expands includes ----------

UNSAFE_NAMES = ["..", ".", "", "../x", "..\\..", "a\\b", "x/..", "/abs", "C:", "fr ", "FR", "f", "french", "fr-", "fr-toolongsuffix", "images", "img", "fonts"]
SAFE_NAMES = ["fr", "de", "zh", "pt-BR", "zh-Hant", "fil"]


def one_language(code: str, **entry) -> dict:
    return {"languages": {"en": LANGUAGES["en"], code: {"name": "X", "enabled": True, **entry}}}


def outside_output(root: Path) -> tuple[Path, Path, Path]:
    """An output folder two levels inside a private wrapper (so even `..\\..` escapes only into the wrapper, never the system temp
    folder), next to a sentinel file and a sentinel folder `keep`: what must survive any build."""
    wrapper = Path(tempfile.mkdtemp())
    atexit.register(shutil.rmtree, wrapper, True)
    box = wrapper / "box"
    box.mkdir()
    (box / "sentinel.txt").write_text("keep me", encoding="utf-8")
    (box / "keep").mkdir()
    (box / "keep" / "inside.txt").write_text("keep me too", encoding="utf-8")
    return box / "out", box / "sentinel.txt", box / "keep"


class FolderNameIsOneSafeSegment(unittest.TestCase):
    """The path of a language (and its code, which is the default path) is a single folder name the build may clear and fill."""

    def test_a_path_that_is_not_one_safe_name_is_a_finding_that_names_it(self):
        for name in UNSAFE_NAMES:
            with self.subTest(path=name):
                problem = i18n.config_problem(one_language("fr", path=name), ["index"])
                self.assertIsNotNone(problem)
                self.assertIn("fr", problem)

    def test_a_language_code_that_is_not_one_safe_name_is_a_finding_whatever_its_path(self):
        for code in ["..", ".", "../x", "..\\..", "a/b", "images", "img", "fonts", "toolong"]:
            with self.subTest(code=code):
                self.assertIsNotNone(i18n.config_problem(one_language(code), ["index"]))
                self.assertIsNotNone(i18n.config_problem(one_language(code, path="fr"), ["index"]))

    def test_a_language_code_with_no_path_is_checked_as_the_path(self):
        problem = i18n.config_problem(one_language("../x"), ["index"])
        self.assertIsNotNone(problem)
        self.assertIn("../x", problem)

    def test_a_safe_name_is_accepted_as_path_and_as_code(self):
        for name in SAFE_NAMES:
            with self.subTest(name=name):
                self.assertIsNone(i18n.config_problem(one_language(name), ["index"]))
                self.assertIsNone(i18n.config_problem(one_language("fr", path=name), ["index"]))

    def test_a_build_with_an_unsafe_path_reports_an_error_and_touches_nothing_outside_the_output(self):
        for name in ["..", "", "../keep", "..\\keep", "..\\.."]:
            with self.subTest(path=name):
                root = i18n_repo(languages={"en": LANGUAGES["en"], "fr": {**LANGUAGES["fr"], "path": name}})
                out, sentinel, keep = outside_output(root)
                try:
                    errors = bs.build_languages(root, out)
                except (bs.BuildError, OSError) as error:
                    errors = [str(error)]
                self.assertTrue(errors, "an unsafe path built without an error")
                self.assertTrue(sentinel.is_file(), "the sentinel file outside the output was deleted")
                self.assertEqual(sorted(path.name for path in keep.iterdir()) if keep.is_dir() else None, ["inside.txt"])

    def test_a_build_with_an_unsafe_code_and_no_path_reports_an_error_and_touches_nothing_outside_the_output(self):
        root = i18n_repo(languages={"en": LANGUAGES["en"], "../keep": {"name": "X", "enabled": True}})
        out, sentinel, keep = outside_output(root)
        try:
            errors = bs.build_languages(root, out)
        except (bs.BuildError, OSError) as error:
            errors = [str(error)]
        self.assertTrue(errors)
        self.assertTrue(sentinel.is_file(), "the sentinel file outside the output was deleted")
        self.assertEqual(sorted(path.name for path in keep.iterdir()) if keep.is_dir() else None, ["inside.txt"])

    def test_a_language_cannot_take_the_name_of_a_folder_the_build_already_writes(self):
        for name in ("images", "img", "fonts"):
            with self.subTest(path=name):
                root = i18n_repo(languages={"en": LANGUAGES["en"], "fr": {**LANGUAGES["fr"], "path": name}})
                errors, _ = build_all(root)
                self.assertTrue(any(name in error for error in errors), errors)


INCLUDE_EN = "# Guide\n\nGuide text.\n\n{{include src/demo.yml yaml}}\n"
INCLUDE_FR = "# Guide FR {#guide}\n\nTexte du guide.\n\n{{include src/demo.yml yaml}}\n"


class GateExpandsIncludes(unittest.TestCase):
    """A translation keeps the `{{include}}` line of the English page: the gate compares it as the build does, expanded."""

    @staticmethod
    def repo(translated: str = INCLUDE_FR) -> Path:
        root = i18n_repo(files={"site/i18n/fr/guide.md": translation("guide", "Guide FR", translated)},
                         sources={"site/pages/guide.md": INCLUDE_EN, "src/demo.yml": "name: demo\n"})
        i18n.stamp(root, "fr", "guide", TODAY)
        return root

    def test_a_translation_that_keeps_the_include_line_has_no_finding(self):
        self.assertEqual(i18n.check_page(self.repo(), "fr", "guide"), [])

    def test_a_translation_that_drops_the_included_code_still_fails_on_the_code_block(self):
        found = i18n.check_page(self.repo("# Guide FR {#guide}\n\nTexte du guide.\n"), "fr", "guide")
        mentions(self, found, "fr/guide", "code block")

    def test_a_translation_that_writes_the_included_file_by_hand_with_other_text_fails(self):
        found = i18n.check_page(self.repo("# Guide FR {#guide}\n\nTexte du guide.\n\n```yaml\nname: autre\n```\n"), "fr", "guide")
        mentions(self, found, "fr/guide", "code block")


if __name__ == "__main__":
    unittest.main()
