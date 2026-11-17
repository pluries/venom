#!/usr/bin/env python3
"""The gate over the real translation files of the docs site (spec 013-site-languages, task T060): every file of every language
folder under `site/i18n/` is checked against the English page of today (fingerprint, code, addresses, anchors, figures, glossary,
review rules), the real site is built with every language that has a folder, and its size is held against SC-007. Nothing here
names a page or a language: a page translated tomorrow is covered by the same tests. The mechanism itself is proved on scratch
translations by `test_site_i18n.py`. Offline, standard library only. Run: python scripts/test_site_i18n_content.py
"""
import atexit
import functools
import json
import re
import shutil
import sys
import tempfile
import time
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import build_site as bs  # noqa: E402
import site_i18n as i18n  # noqa: E402
from test_build_site import ROOT  # noqa: E402
from test_site_i18n import assert_no_draft_banner, key_paths  # noqa: E402

I18N = ROOT / "site" / "i18n"
MAX_SITE_BYTES = 15 * 1024 * 1024  # SC-007
DATE = re.compile(r"^\d{4}-\d{2}-\d{2}$")
FINGERPRINT = re.compile(r"^[0-9a-f]{64}$")


def language_folders() -> list[str]:
    """The languages that have a folder, enabled or not (D-014): found on the disk, not in the module under test."""
    return sorted(path.name for path in I18N.iterdir() if path.is_dir() and path.name != "en")


def disk_files() -> list[tuple[str, str]]:
    return sorted((lang, path.stem) for lang in language_folders() for path in (I18N / lang).glob("*.md"))


def read_header(lang: str, name: str) -> dict[str, str]:
    return i18n.parse_translation((I18N / lang / name).read_text(encoding="utf-8")).header


class RealTranslationsPassTheGate(unittest.TestCase):
    """US2-AS1 to AS12, FR-008, FR-009, SC-002, SC-004: the real files, whatever their number."""

    def test_fr_008_the_gate_sees_every_translation_file_of_every_language_folder(self):
        files = disk_files()
        self.assertTrue(files, "no translation file under site/i18n")
        self.assertEqual(i18n.translation_files(ROOT), files)

    def test_us1_as4_every_language_folder_has_its_interface_file_and_its_home_page(self):
        for lang in language_folders():
            for name in ("ui.json", "index.md", "home.html"):
                with self.subTest(language=lang, file=name):
                    self.assertTrue((I18N / lang / name).is_file())

    def test_us2_as1_every_real_translation_passes_every_check(self):
        for lang, slug in disk_files():
            with self.subTest(page=f"{lang}/{slug}"):
                self.assertEqual(i18n.check_page(ROOT, lang, slug), [])

    def test_us2_as7_the_gate_of_the_whole_site_is_clean_and_follows_the_complete_switch_of_site_json(self):
        self.assertEqual(i18n.check_site(ROOT), [])

    def test_fr_007_every_file_records_its_english_page_fingerprint_drafter_date_and_status(self):
        names = [(lang, f"{slug}.md") for lang, slug in disk_files()] + [(lang, "home.html") for lang in language_folders()
                                                                          if (I18N / lang / "home.html").is_file()]
        for lang, name in names:
            with self.subTest(file=f"{lang}/{name}"):
                header = read_header(lang, name)
                self.assertEqual(header["translates"], name.rsplit(".", 1)[0] if name.endswith(".md") else "index")
                self.assertRegex(header.get("fingerprint", ""), FINGERPRINT)
                self.assertTrue(header.get("drafted_by"), "drafted_by: the drafter is recorded per file")
                self.assertRegex(header.get("drafted_on", ""), DATE)
                self.assertIn(header.get("status"), i18n.STATUSES)

    def test_fr_007_the_home_page_has_one_fingerprint_in_both_of_its_files(self):
        for lang in language_folders():
            with self.subTest(language=lang):
                self.assertEqual(read_header(lang, "index.md").get("fingerprint"), read_header(lang, "home.html").get("fingerprint"))
                self.assertEqual(read_header(lang, "index.md").get("status"), read_header(lang, "home.html").get("status"))

    def test_us4_as4_the_status_counts_every_english_page_once_and_no_real_page_is_stale(self):
        pages = bs.SiteBuild(ROOT, bs.load_config(ROOT), "").pages
        complete = bs.load_config(ROOT).get("i18n", {}).get("complete") is True
        counts = i18n.status(ROOT)
        self.assertEqual(sorted(counts), sorted(lang for lang in bs.load_config(ROOT)["languages"] if lang != "en"))
        for lang, row in counts.items():
            with self.subTest(language=lang):
                self.assertEqual(sum(row.values()), len(pages))
                self.assertEqual(row["stale"], 0)
                if complete:
                    self.assertEqual(row["missing"], 0)


@functools.lru_cache(maxsize=None)
def real_build() -> tuple[list[str], Path, float]:
    """The real site with every language that has a folder, previewed (a language still being drafted is disabled in site.json)."""
    out = Path(tempfile.mkdtemp())
    atexit.register(shutil.rmtree, out, True)
    started = time.monotonic()
    errors = bs.build_languages(ROOT, out, preview_languages=tuple(language_folders()))
    return errors, out, time.monotonic() - started


class RealSiteBuild(unittest.TestCase):
    """US2-AS4, AS5, FR-011, SC-007: the built site in every language."""

    @classmethod
    def setUpClass(cls):
        cls.errors, cls.out, cls.seconds = real_build()
        cls.paths = {code: i18n.language_path(bs.load_config(ROOT), code) for code in language_folders()}

    def test_us2_as8_the_real_site_builds_in_every_language_without_an_error(self):
        self.assertEqual(self.errors, [])

    def test_us2_as4_no_real_page_of_any_language_has_a_draft_banner_whatever_its_status(self):
        for lang, slug in disk_files():
            rel = f"{self.paths[lang]}/{slug}.html"
            status = read_header(lang, f"{slug}.md").get("status")
            with self.subTest(page=rel, status=status):
                assert_no_draft_banner(self, (self.out / rel).read_text(encoding="utf-8"))

    def test_sc_007_the_built_site_is_at_most_fifteen_megabytes_and_the_time_is_reported(self):
        files = [path for path in self.out.rglob("*") if path.is_file()]
        size = sum(path.stat().st_size for path in files)
        sys.stderr.write(f"\nsite_i18n_content: {len(files)} files, {size} bytes, built in {self.seconds:.1f} s (time is evidence, not a gate)\n")
        self.assertLessEqual(size, MAX_SITE_BYTES)


class InterfaceFilesHaveTheSameKeys(unittest.TestCase):
    """FR-003: every language's `ui.json` has exactly the keys of the English one, at every depth."""

    def test_us1_as4_the_key_sets_of_all_ui_files_equal_the_english_one(self):
        english = key_paths(json.loads((I18N / "en" / "ui.json").read_text(encoding="utf-8")))
        for lang in language_folders():
            other = key_paths(json.loads((I18N / lang / "ui.json").read_text(encoding="utf-8")))
            with self.subTest(language=lang):
                self.assertEqual((sorted(english - other), sorted(other - english)), ([], []))


class InstallingPageKeepsItsCommands(unittest.TestCase):
    """US3-AS4, FR-009: the translated Installing page keeps the English page's fenced commands and file names byte for byte, and
    still links to the `pluries/venom` releases."""

    RELEASES = "https://github.com/pluries/venom/releases"

    @classmethod
    def setUpClass(cls):
        site = i18n._Site(ROOT)
        cls.english = i18n._scan(site.build.page_text(site.page("installing"))).blocks

    def test_us3_as4_the_verify_commands_and_file_names_are_unchanged_and_the_releases_link_stays(self):
        self.assertTrue(self.english, "the English Installing page has fenced commands")
        for lang in language_folders():
            path = I18N / lang / "installing.md"
            if not path.is_file():
                continue
            text = i18n.parse_translation(path.read_text(encoding="utf-8")).body
            with self.subTest(language=lang):
                self.assertEqual(i18n._scan(text).blocks, self.english)
                self.assertIn(self.RELEASES, text)


class LicencePageKeepsTheEnglishLegalText(unittest.TestCase):
    """US4-AS3: the licence page of a language has a translated heading and a note, and the legal text is the English original."""

    @classmethod
    def setUpClass(cls):
        site = i18n._Site(ROOT)
        cls.english = site.build.page_text(site.page("license")).strip().splitlines()

    def test_us4_as3_the_heading_and_note_are_translated_and_the_legal_text_is_verbatim_english(self):
        heading = next(line for line in self.english if line.startswith("#"))
        legal = [line for line in self.english if line.strip() and not line.startswith("#")]
        checked = 0
        for lang in language_folders():
            path = I18N / lang / "license.md"
            if not path.is_file():
                continue
            checked += 1
            lines = i18n.parse_translation(path.read_text(encoding="utf-8")).body.strip().splitlines()
            translated = [line for line in lines if line.strip()]
            with self.subTest(language=lang):
                self.assertTrue(translated[0].startswith("#"))
                self.assertNotEqual(re.sub(r"\s*\{#.*\}$", "", translated[0]), re.sub(r"\s*\{#.*\}$", "", heading))
                self.assertFalse(translated[1].startswith("#") or translated[1] in legal, "a note in the language, before the legal text")
                self.assertEqual(translated[2:], legal)
        self.assertTrue(checked, "no licence page translated")


EXECUTOR_SLUGS = ("amqp", "couchbase", "dbfixtures", "exec", "grpc", "http", "kafka", "mongo", "mqtt", "ovhapi", "rabbitmq", "readfile",
                  "redis", "smtp", "sql", "ssh", "imap", "mcp", "web", "mobile")
_SPAN = re.compile(r"`([^`\n]+)`")


def _spans(prose: list[str]) -> list[str]:
    return [span.strip() for line in prose for span in _SPAN.findall(line)]


def _in_order(wanted: list[str], found: list[str]) -> list[str]:
    """The items of `wanted` that are not met, in order, in `found`: each one is looked for after the previous one."""
    position, missing = 0, []
    for item in wanted:
        try:
            position = found.index(item, position) + 1
        except ValueError:
            missing.append(item)
    return missing


class ExecutorPagesKeepTheirTechnicalNames(unittest.TestCase):
    """US4-AS2, FR-009: in a translated executor page the field names, step types, assertion names, variable names and output examples
    are the English ones: every fenced block is equal and every inline-code span of the English page is met, in order (characterization of
    the real files: the identity check of the gate already holds them, this pins the claim itself)."""

    @classmethod
    def setUpClass(cls):
        cls.site = i18n._Site(ROOT)

    def translated(self, lang: str, slug: str) -> tuple[i18n._Scan, i18n._Scan]:
        page = self.site.page(slug)
        english = i18n._scan(self.site.build.page_text(page))
        body = i18n.parse_translation((I18N / lang / f"{slug}.md").read_text(encoding="utf-8")).body
        return english, i18n._scan(bs.expand_includes(body, ROOT, page.source))

    def test_us4_as2_every_translated_executor_page_has_the_english_code_blocks_and_inline_code(self):
        checked = 0
        for lang in language_folders():
            for slug in EXECUTOR_SLUGS:
                if not (I18N / lang / f"{slug}.md").is_file():
                    continue
                checked += 1
                english, theirs = self.translated(lang, slug)
                with self.subTest(page=f"{lang}/{slug}"):
                    self.assertEqual(theirs.blocks, english.blocks)
                    self.assertEqual(_in_order(_spans(english.prose), _spans(theirs.prose)), [])
        self.assertTrue(checked, "no translated executor page")

    def test_us4_as2_the_comparison_sees_a_renamed_field_a_changed_block_and_a_reordered_span(self):
        self.assertEqual(_in_order(["a", "b"], ["a", "x", "b"]), [])
        self.assertEqual(_in_order(["a", "b"], ["b", "a"]), ["b"])
        self.assertEqual(_in_order(["header"], ["entete"]), ["header"])
        self.assertEqual(_spans(["Use `type: http` and `{{.x}}`.", "Plain."]), ["type: http", "{{.x}}"])


if __name__ == "__main__":
    unittest.main()
