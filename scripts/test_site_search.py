"""Tests of the search index and the llms files that every site build writes (#3633, task #3702)."""
import json
import re
import shutil
import sys
import unittest
from unittest import mock
from pathlib import Path
from urllib.parse import urljoin, urlsplit

sys.path.insert(0, str(Path(__file__).resolve().parent))
import build_site as bs  # noqa: E402
from test_build_site import make_repo  # noqa: E402
from test_site_i18n import LANGUAGES, build_all, i18n_repo, translation  # noqa: E402

BASE = "https://docs.example.test/"
INDEX_FILE, LLMS_FILE, FULL_FILE = "search-index.json", "llms.txt", "llms-full.txt"
FILES = (INDEX_FILE, LLMS_FILE, FULL_FILE)
HOME = {"slug": "index", "title": "Home", "source": "index.md", "description": "The home page of the test site."}
GUIDE = {"slug": "guide", "title": "Guide", "source": "guide.md", "group": "Reference", "description": "How to run the thing."}
SOURCES = {
    "index.md": "# Home\n\nWelcome to the zebra home page.\n\n## Getting started\n\nInstall it first.\n",
    "guide.md": "# Guide\n\nRun the quokka command.\n\n## Second step\n\nThen check the output.\n",
}


class SearchFilesCase(unittest.TestCase):
    def repo(self, files: dict[str, str], pages: list[dict], **extra) -> Path:
        root = make_repo(files, pages, **extra)
        self.addCleanup(shutil.rmtree, root, True)
        return root

    @staticmethod
    def build(root: Path, base: str = BASE) -> Path:
        out = root / "_out"
        errors = bs.build(root, out, base)
        if errors:
            raise AssertionError(errors)
        return out

    @staticmethod
    def read(out: Path, name: str) -> str:
        return (out / name).read_text(encoding="utf-8")

    def entries(self, out: Path) -> list[dict]:
        return json.loads(self.read(out, INDEX_FILE))

    def standard(self) -> Path:
        return self.repo(SOURCES, [HOME, GUIDE])

    def contents(self, out: Path) -> str:
        return "\n".join(self.read(out, name) for name in FILES)


class T1SearchIndex(SearchFilesCase):
    def test_t1_the_index_lists_both_pages_in_site_order_with_the_fields_the_script_reads(self):
        entries = self.entries(self.build(self.standard()))
        self.assertEqual([e["slug"] for e in entries], ["index", "guide"])
        for entry in entries:
            self.assertEqual(sorted(entry), ["description", "group", "headings", "slug", "text", "title"])
        home, guide = entries
        self.assertEqual((home["title"], home["group"], home["description"]), ("Home", "Guides", "The home page of the test site."))
        self.assertEqual((guide["title"], guide["group"]), ("Guide", "Reference"))

    def test_t1_the_headings_carry_the_text_and_the_anchor_of_the_page(self):
        out = self.build(self.standard())
        home = self.entries(out)[0]
        self.assertIn({"text": "Getting started", "anchor": "getting-started"}, home["headings"])
        self.assertIn('id="getting-started"', self.read(out, "index.html"))
        self.assertIn("zebra", home["text"])
        self.assertIn("Install it first.", home["text"])

    def test_t1_the_file_has_sorted_keys_and_newline_ends_and_is_identical_between_builds(self):
        root = self.standard()
        first = (self.build(root) / INDEX_FILE).read_bytes()
        second = (self.build(root) / INDEX_FILE).read_bytes()
        self.assertEqual(first, second)
        self.assertNotIn(b"\r", first)
        self.assertTrue(first.endswith(b"\n"))
        orders = []
        json.loads(first.decode("utf-8"), object_pairs_hook=lambda pairs: orders.append([k for k, _ in pairs]) or dict(pairs))
        self.assertTrue(orders)
        for keys in orders:
            self.assertEqual(keys, sorted(keys))


class T2LlmsFiles(SearchFilesCase):
    def test_t2_llms_txt_lists_every_page_as_a_link_line_with_an_absolute_address_and_its_description(self):
        out = self.build(self.standard())
        text = self.read(out, LLMS_FILE)
        self.assertEqual(text.splitlines()[0], "# T")
        lines = [line for line in text.splitlines() if line.startswith("- ")]
        self.assertEqual(lines, [
            "- [Home](https://docs.example.test/index.html): The home page of the test site.",
            "- [Guide](https://docs.example.test/guide.html): How to run the thing.",
        ])
        self.assertNotIn("\r", text)

    def test_t2_every_page_address_in_llms_txt_is_absolute_and_inside_the_link(self):
        out = self.build(self.standard())
        found = re.findall(r"\]\((\S*?)\)", self.read(out, LLMS_FILE))
        self.assertEqual(found, ["https://docs.example.test/index.html", "https://docs.example.test/guide.html"])

    def test_t2_llms_full_holds_the_text_of_each_page_in_site_order(self):
        full = self.read(self.build(self.standard()), FULL_FILE)
        for needle in ("zebra home page", "Install it first.", "quokka command", "Then check the output."):
            self.assertIn(needle, full)
        self.assertLess(full.index("zebra"), full.index("quokka"))


class T3StaleEntries(SearchFilesCase):
    def edit_config(self, root: Path, change) -> None:
        path = root / "site" / "site.json"
        config = json.loads(path.read_text(encoding="utf-8"))
        change(config)
        path.write_text(json.dumps(config), encoding="utf-8")

    def test_t3_a_removed_page_leaves_no_entry_in_any_file(self):
        root = self.standard()
        self.assertIn("guide", self.contents(self.build(root)))
        self.edit_config(root, lambda c: c.update(pages=[p for p in c["pages"] if p["slug"] != "guide"]))
        text = self.contents(self.build(root))
        self.assertNotIn("guide", text)
        self.assertNotIn("quokka", text)
        self.assertIn("zebra", text)

    def test_t3_a_renamed_page_keeps_only_its_new_address(self):
        root = self.standard()
        self.build(root)

        def rename(config):
            for page in config["pages"]:
                if page["slug"] == "guide":
                    page["slug"] = "handbook"
        self.edit_config(root, rename)
        out = self.build(root)
        self.assertNotIn("guide.html", self.contents(out))
        self.assertIn("https://docs.example.test/handbook.html", self.read(out, LLMS_FILE))
        self.assertEqual([e["slug"] for e in self.entries(out)], ["index", "handbook"])


class T4Extraction(SearchFilesCase):
    """The text is the page's words verbatim: only tags, entities, includes, fences and the anchor link go."""
    SOURCE = ("# Home\n\nPlain **bold** and [a link](guide.md) and `inline` words.\n\n"
              "## Include\n\n{{include src/a.go go}}\n\n> A quote line.\n\n- first item\n- second item\n\n"
              "## Verbatim\n\nUse `{{ .venom.testsuite }}` in a step.\n\n"
              "Files match `./foo/b*/**/z*.yml` here.\n\n"
              "```yaml\nvars:\n  foo: bar\n  nested:\n    key: value\nif: a < b\n```\n\n"
              "| Name | Meaning |\n|---|---|\n| alpha | first |\n| beta | second |\n")
    GO = "package a\n\nfunc main() {\n    return\n}\n"
    FORBIDDEN = ("&#123;", "&lt;", "&gt;", "&amp;", "&quot;", "```", "{{include", "[a link]", "<p>", "</", "<li>", "<h2", "<pre",
                 "<code", "<table", "<td", "<th")

    def built(self) -> tuple[Path, str]:
        root = self.repo({"src/a.go": self.GO, "guide.md": SOURCES["guide.md"], "index.md": self.SOURCE}, [HOME, GUIDE])
        out = self.build(root)
        return out, self.entries(out)[0]["text"]

    def test_t4_the_files_hold_no_markup_entity_include_or_fence(self):
        out, _ = self.built()
        for name in (INDEX_FILE, FULL_FILE, LLMS_FILE):
            content = self.read(out, name)
            for forbidden in self.FORBIDDEN:
                self.assertNotIn(forbidden, content, f"{forbidden!r} in {name}")
        self.assertNotIn("Getting started#", self.contents(out))

    def test_t4_the_plain_words_survive(self):
        _, text = self.built()
        for kept in ("Plain bold and a link and inline words.", "A quote line.", "first item", "second item", "if: a < b"):
            self.assertIn(kept, text)

    def test_t4_interpolation_syntax_in_the_page_text_survives_literally(self):
        out, text = self.built()
        self.assertIn("Use {{ .venom.testsuite }} in a step.", text)
        self.assertIn("{{ .venom.testsuite }}", self.read(out, FULL_FILE))

    def test_t4_a_glob_with_double_stars_in_a_code_span_survives(self):
        out, text = self.built()
        self.assertIn("./foo/b*/**/z*.yml", text)
        self.assertIn("./foo/b*/**/z*.yml", self.read(out, FULL_FILE))

    def test_t4_a_pre_block_keeps_its_indentation_and_line_breaks(self):
        out, text = self.built()
        block = "vars:\n  foo: bar\n  nested:\n    key: value\nif: a < b"
        self.assertIn(block, text)
        self.assertIn(block, self.read(out, FULL_FILE))

    def test_t4_an_included_file_keeps_its_code_content_and_indentation(self):
        out, text = self.built()
        self.assertIn("package a\n\nfunc main() {\n    return\n}", text)
        self.assertIn("func main() {\n    return\n}", self.read(out, FULL_FILE))

    def test_t4_table_cells_are_separated(self):
        out, text = self.built()
        for content in (text, self.read(out, FULL_FILE)):
            for left, right in (("Name", "Meaning"), ("alpha", "first"), ("beta", "second")):
                self.assertRegex(content, rf"\b{left}\b\W+{right}\b")
            self.assertNotRegex(content, r"alphafirst|betasecond|NameMeaning|firstbeta")


class T5BuildErrors(SearchFilesCase):
    @staticmethod
    def run_build(root: Path, base: str) -> list[str]:
        try:
            return bs.SiteBuild(root, bs.load_config(root), base).run(root / "_out")
        except bs.BuildError as error:
            return [str(error)]

    def test_t5_an_empty_base_is_a_clear_error_and_no_llms_file_is_written(self):
        root = self.standard()
        errors = self.run_build(root, "")
        self.assertTrue(any("base" in e.lower() for e in errors), errors)
        self.assertFalse((root / "_out" / LLMS_FILE).exists())

    def test_t5_a_page_without_a_description_is_named_in_the_error(self):
        pages = [HOME, {"slug": "guide", "title": "Guide", "source": "guide.md", "group": "Reference", "description": ""}]
        root = self.repo(SOURCES, pages, default_description=False)
        errors = self.run_build(root, "https://docs.example.test")
        self.assertTrue(any("guide" in e and "description" in e.lower() for e in errors), errors)
        self.assertFalse((root / "_out" / LLMS_FILE).exists())


class T6Size(SearchFilesCase):
    def sizes(self, out: Path) -> dict[str, int]:
        return {name: (out / name).stat().st_size for name in (INDEX_FILE, FULL_FILE)}

    def test_t6_the_index_and_llms_full_stay_under_the_stated_limit(self):
        out = self.build(self.standard())
        self.assertIsInstance(bs.SEARCH_SIZE_LIMIT, int)
        self.assertGreater(bs.SEARCH_SIZE_LIMIT, 0)
        for name, size in self.sizes(out).items():
            self.assertGreater(size, 0, name)
            self.assertLessEqual(size, bs.SEARCH_SIZE_LIMIT, name)

    def test_t6_a_build_whose_files_are_exactly_at_the_limit_is_clean(self):
        root = self.standard()
        limit = max(self.sizes(self.build(root)).values())
        with mock.patch.object(bs, "SEARCH_SIZE_LIMIT", limit):
            self.assertEqual(T5BuildErrors.run_build(root, BASE), [])

    def test_t6_the_index_over_the_limit_is_an_error_naming_the_file_and_the_limit(self):
        root = self.standard()
        sizes = self.sizes(self.build(root))
        self.assertGreater(sizes[INDEX_FILE], sizes[FULL_FILE])
        limit = sizes[INDEX_FILE] - 1
        with mock.patch.object(bs, "SEARCH_SIZE_LIMIT", limit):
            errors = T5BuildErrors.run_build(root, BASE)
        self.assertTrue(any(INDEX_FILE in e and str(limit) in e for e in errors), errors)
        self.assertFalse(any(FULL_FILE in e for e in errors), errors)

    def test_t6_llms_full_over_the_limit_is_an_error_naming_the_file_and_the_limit(self):
        root = self.standard()
        limit = self.sizes(self.build(root))[FULL_FILE] - 1
        with mock.patch.object(bs, "SEARCH_SIZE_LIMIT", limit):
            errors = T5BuildErrors.run_build(root, BASE)
        self.assertTrue(any(FULL_FILE in e and str(limit) in e for e in errors), errors)


SITE_DIR = Path(__file__).resolve().parent.parent / "site"
SITE_JS = (SITE_DIR / "js" / "site.js").read_text(encoding="utf-8")
SITE_CSS = (SITE_DIR / "style.css").read_text(encoding="utf-8")


def search_buttons(markup: str) -> list[str]:
    return [m for m in re.findall(r"<button\b[^>]*>", markup) if re.search(r'class="[^"]*\bsearch-button\b', m)]


class SearchButtonCase(SearchFilesCase):
    def topbar(self) -> str:
        root = self.standard()
        site = bs.SiteBuild(root, bs.load_config(root), BASE)
        return bs.render_topbar(site.config, site.pages)


class T1SearchButtonInTopbar(SearchButtonCase):
    def test_t1_the_top_bar_holds_exactly_one_labelled_search_button_in_the_actions(self):
        topbar = self.topbar()
        buttons = search_buttons(topbar)
        self.assertEqual(len(buttons), 1, topbar)
        button = buttons[0]
        self.assertIn('type="button"', button)
        label = re.search(r'aria-label="([^"]+)"', button)
        self.assertIsNotNone(label, button)
        self.assertIn("search", label.group(1).lower())
        actions = topbar[topbar.index('class="top-actions"'):]
        self.assertIn("search-button", actions)

    def test_t1_the_button_carries_the_hidden_attribute(self):
        self.assertRegex(search_buttons(self.topbar())[0], r"\shidden(\s|=|>)")


class T2EveryPageHasTheButton(SearchButtonCase):
    def test_t2_every_generated_page_has_the_button_and_loads_the_search_script(self):
        template = "<html><head>{{scripts}}</head><body>{{topbar}}{{content}}</body></html>"
        scripts = {"site/js/theme.js": "// theme\n", "site/js/site.js": "// site\n"}
        out = self.build(self.repo({**SOURCES, "site/template.html": template, **scripts}, [HOME, GUIDE]))
        for name in ("index.html", "guide.html"):
            page = self.read(out, name)
            self.assertEqual(len(search_buttons(page)), 1, name)
            self.assertIn(bs.SCRIPT_TAGS, page, name)
            self.assertIn('<script src="site.js" defer></script>', page, name)

    def test_t2_the_script_fetches_only_the_relative_index(self):
        calls = re.findall(r"fetch\(([^)]*)\)", SITE_JS)
        self.assertEqual(calls, ['"search-index.json"'])


class T3NoExternalAddress(unittest.TestCase):
    def test_t3_the_stylesheet_and_the_script_name_no_third_party_host(self):
        for name, source in (("site/js/site.js", SITE_JS), ("site/style.css", SITE_CSS)):
            cleaned = source.replace("http://www.w3.org/2000/svg", "")
            self.assertNotRegex(cleaned, r"https?://", name)
            self.assertNotRegex(cleaned, r"(?i)googleapis|gstatic|cdn\.|unpkg|jsdelivr|cloudflare", name)


class T4OverlayAccessibility(unittest.TestCase):
    def test_t4_the_overlay_keeps_its_dialog_role_and_label(self):
        self.assertIn('setAttribute("role", "dialog")', SITE_JS)
        self.assertIn('box.setAttribute("aria-label"', SITE_JS)

    def test_t4_the_input_has_an_accessible_label(self):
        """The label is the page's own wording (ui.search_label); a literal in the script would be overwritten or win over it."""
        self.assertIn('input.setAttribute("aria-label", ui.search_label);', SITE_JS)
        self.assertNotRegex(SITE_JS, r'input\.setAttribute\("aria-label", "')
        self.assertRegex(SITE_JS, r'const DEFAULT_UI = \{[^}]*?search_label: "Search",')


class T4InputMethodComposition(unittest.TestCase):
    """US1-AS8: Enter that confirms a Pinyin candidate must not navigate. No test runs the script, so the guard is pinned by source."""

    def test_us1_as8_the_key_handler_returns_early_while_an_ime_is_composing(self):
        """US1-AS8."""
        match = re.search(r"const onInputKey = \(event\) => \{(.*?)\n  \};", SITE_JS, re.S)
        self.assertIsNotNone(match, "onInputKey not found")
        body = match.group(1)
        guard = re.match(r"\s*if \(event\.isComposing \|\| event\.keyCode === 229\) \{\s*return;\s*\}", body)
        self.assertIsNotNone(guard, "onInputKey must start with: if (event.isComposing || event.keyCode === 229) { return; }")

    def test_t4_a_live_region_carries_the_result_count_and_the_nothing_found_message(self):
        self.assertRegex(SITE_JS, r'status\.setAttribute\("role", "status"\)|aria-live')
        self.assertIn("Nothing matches that search.", SITE_JS)
        self.assertRegex(SITE_JS, r"search\.status\.textContent = message")
        self.assertRegex(SITE_JS, r"search\.status\.textContent = `\$\{found\.length\}")


class T6ResultsAreOnlyText(unittest.TestCase):
    """US4-AS8, characterization: already satisfied by site/js/site.js."""

    def test_us4_as8_no_api_that_parses_markup_is_used(self):
        """US4-AS8."""
        for name in ("innerHTML", "outerHTML", "insertAdjacentHTML", "document.write", "DOMParser",
                     "createContextualFragment", "setHTML", "srcdoc", ".writeln"):
            with self.subTest(api=name):
                self.assertNotIn(name, SITE_JS)

    def test_us4_as8_the_result_title_and_place_are_set_as_text(self):
        """US4-AS8."""
        self.assertIn("title.textContent = entry.title", SITE_JS)
        self.assertIn("place.textContent = section + (heading ? heading.text : entry.description);", SITE_JS)


class T7ChineseSearchSource(unittest.TestCase):
    """US1-AS8, FR-015, D-016. Characterization, green before and after: no test runs the script (no node in the approved stack),
    so the two lines that decide how a Chinese query matches are pinned by their source. The browser proof is the venom web suite
    `tests/site/site-languages.yml`, run by the maker. In the script `\\s` matches U+3000, so a full-width space separates two terms; a
    term is matched as a substring of the page text, so a one-character term and a term inside a run of characters both match."""

    def test_us1_as8_the_query_is_split_on_whitespace_only(self):
        self.assertIn("const terms = (query) => query.toLowerCase().split(/\\s+/).filter((word) => word.length > 0);", SITE_JS)

    def test_us1_as8_every_term_is_matched_as_a_substring_of_the_text(self):
        self.assertIn("const has = (text, words) => {\n    const lower = text.toLowerCase();\n"
                      "    return words.every((word) => lower.includes(word));\n  };", SITE_JS)

    def test_us1_as8_the_text_is_not_segmented_by_word_boundaries(self):
        for api in ("Intl.Segmenter", "\\b", "normalize("):
            with self.subTest(api=api):
                self.assertNotIn(api, SITE_JS)


LANGUAGE_BASE = "https://docs.example.test"
ENGLISH_SOURCE_LINE = re.compile(r"(?im)^(?!- \[).*\bEnglish\b.*https://docs\.example\.test/llms(?:-full)?\.txt")
ZH = {"en": LANGUAGES["en"], "zh": {"name": "中文", "path": "zh", "lang": "zh-Hans", "enabled": True}}
ZH_GUIDE = translation("guide", "指南", "# 指南 {#guide}\n\n运行测试，检查输出。\n\n## 第二步 {#second-step}\n\n然后检查结果。\n")


class T022LanguageSearchFiles(unittest.TestCase):
    """US1-AS9, FR-015, FR-016: each language folder has its own index and llms files; English keeps its own, unchanged."""

    @classmethod
    def setUpClass(cls):
        cls.errors, cls.out = build_all(i18n_repo())

    def read(self, rel: str) -> str:
        return (self.out / rel).read_text(encoding="utf-8")

    def entries(self, rel: str) -> list[dict]:
        return json.loads(self.read(rel))

    def test_us1_as9_the_build_is_clean_and_every_language_folder_has_the_three_files(self):
        self.assertEqual(self.errors, [])
        for name in FILES:
            for folder in ("", "fr/"):
                with self.subTest(file=folder + name):
                    self.assertTrue((self.out / (folder + name)).is_file())

    def test_us1_as9_the_french_index_holds_the_french_pages_only_in_site_order(self):
        entries = self.entries("fr/search-index.json")
        self.assertEqual([e["slug"] for e in entries], ["overview", "installing", "guide"])
        self.assertEqual([e["title"] for e in entries], ["Apercu", "Installer", "Guide FR"])
        self.assertIn("Texte du guide", entries[2]["text"])
        self.assertNotIn("Guide text", entries[2]["text"])
        for entry in entries:
            self.assertEqual(sorted(entry), ["description", "group", "headings", "slug", "text", "title"])

    def test_us1_as9_the_english_index_holds_no_french_text(self):
        english = self.read("search-index.json")
        self.assertNotIn("Texte du guide", english)
        self.assertEqual([e["slug"] for e in self.entries("search-index.json")], ["index", "overview", "installing", "guide", "other"])

    def test_us1_as9_the_headings_of_a_french_entry_carry_the_english_anchors(self):
        guide = self.entries("fr/search-index.json")[2]
        self.assertEqual([h["anchor"] for h in guide["headings"]], ["local-part", "extra-details"])
        self.assertEqual([h["text"] for h in guide["headings"]], ["Partie locale", "Details en plus"])

    def test_us1_as9_the_page_of_a_language_loads_the_index_of_its_own_folder(self):
        self.assertIn('fetch("search-index.json")', SITE_JS)
        for rel in ("fr/guide.html", "fr/index.html", "guide.html"):
            with self.subTest(page=rel):
                target = urlsplit(urljoin(f"{LANGUAGE_BASE}/{rel}", "search-index.json")).path.lstrip("/")
                self.assertEqual(target, rel.rsplit("/", 1)[0] + "/search-index.json" if "/" in rel else "search-index.json")
                self.assertTrue((self.out / target).is_file())

    def test_us1_as9_a_result_links_to_a_page_of_its_own_folder_by_a_relative_address(self):
        self.assertIn("link.href = `${entry.slug}.html${fragment}`;", SITE_JS)

    def test_fr_016_the_llms_txt_of_a_language_links_its_pages_with_the_addresses_of_its_folder(self):
        text = self.read("fr/llms.txt")
        self.assertEqual([line for line in text.splitlines() if line.startswith("- ")], [
            f"- [Apercu]({LANGUAGE_BASE}/fr/overview.html): Description de Apercu.",
            f"- [Installer]({LANGUAGE_BASE}/fr/installing.html): Description de Installer.",
            f"- [Guide FR]({LANGUAGE_BASE}/fr/guide.html): Description de Guide FR.",
        ])
        self.assertEqual(text.splitlines()[0], "# T")

    def test_fr_016_the_llms_files_of_a_language_name_the_english_source(self):
        for name in ("fr/llms.txt", "fr/llms-full.txt"):
            with self.subTest(file=name):
                self.assertRegex(self.read(name), ENGLISH_SOURCE_LINE)

    def test_fr_016_the_english_llms_files_are_unchanged_and_name_no_other_source(self):
        for name in ("llms.txt", "llms-full.txt"):
            with self.subTest(file=name):
                self.assertNotRegex(self.read(name), ENGLISH_SOURCE_LINE)
                self.assertNotIn("/fr/", self.read(name))
        self.assertEqual([line for line in self.read("llms.txt").splitlines() if line.startswith("- ")][:2], [
            f"- [Home]({LANGUAGE_BASE}/index.html): The home page.", f"- [Overview]({LANGUAGE_BASE}/overview.html): What it is."])

    def test_fr_016_llms_full_of_a_language_holds_the_text_of_its_pages_and_none_of_the_english_ones(self):
        full = self.read("fr/llms-full.txt")
        for needle in ("Texte introductif", "Installez.", "Texte du guide"):
            self.assertIn(needle, full)
        for english in ("Intro text of the overview", "Install the thing.", "Guide text"):
            self.assertNotIn(english, full)

    def test_fr_015_the_files_of_every_language_stay_under_the_stated_limit(self):
        for rel in ("fr/search-index.json", "fr/llms-full.txt"):
            with self.subTest(file=rel):
                self.assertLessEqual((self.out / rel).stat().st_size, bs.SEARCH_SIZE_LIMIT)

    def test_fr_015_a_language_file_over_the_limit_is_an_error_naming_the_file(self):
        root = i18n_repo()
        limit = (self.out / "fr" / "search-index.json").stat().st_size - 1
        with mock.patch.object(bs, "SEARCH_SIZE_LIMIT", limit):
            errors, _ = build_all(root)
        self.assertTrue(any("search-index.json" in e and str(limit) in e for e in errors), errors)


class T022ChineseSearchFiles(unittest.TestCase):
    """US1-AS8, FR-015: the Chinese index keeps the characters as they are, so the script finds them as substrings of the text."""

    @classmethod
    def setUpClass(cls):
        cls.errors, cls.out = build_all(i18n_repo(languages=ZH, files={"site/i18n/zh/guide.md": ZH_GUIDE}))

    def test_us1_as8_the_chinese_index_holds_the_text_in_chinese_characters_not_escapes(self):
        self.assertEqual(self.errors, [])
        raw = (self.out / "zh" / "search-index.json").read_text(encoding="utf-8")
        self.assertIn("运行测试，检查输出", raw)
        self.assertNotIn("\\u", raw)
        entry = json.loads(raw)[0]
        self.assertEqual((entry["slug"], entry["title"]), ("guide", "指南"))
        self.assertIn("运行测试", entry["text"])
        self.assertEqual([h["text"] for h in entry["headings"]], ["第二步"])

    def test_fr_016_the_chinese_llms_files_use_the_zh_addresses_and_name_the_english_source(self):
        text = (self.out / "zh" / "llms.txt").read_text(encoding="utf-8")
        self.assertIn(f"- [指南]({LANGUAGE_BASE}/zh/guide.html): Description de 指南.", text)
        for name in ("zh/llms.txt", "zh/llms-full.txt"):
            self.assertRegex((self.out / name).read_text(encoding="utf-8"), ENGLISH_SOURCE_LINE)


class T5ButtonRevealedByScript(unittest.TestCase):
    def test_t5_the_script_reveals_every_search_button(self):
        loop = re.search(r'for \(const (\w+) of document\.querySelectorAll\("\.search-button"\)\) \{(.*?)\n    \}', SITE_JS, re.S)
        self.assertIsNotNone(loop)
        name, body = loop.groups()
        self.assertRegex(body, rf"{name}\.hidden = false|{name}\.removeAttribute\(\"hidden\"\)")

    def test_t5_the_stylesheet_keeps_a_hidden_button_hidden_over_icon_btn(self):
        rule = re.search(r"\.search-button\[hidden\]\s*\{([^}]*)\}", SITE_CSS)
        self.assertIsNotNone(rule)
        self.assertRegex(rule.group(1), r"display:\s*none")


if __name__ == "__main__":
    unittest.main()
