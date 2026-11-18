#!/usr/bin/env python3
"""Tests of the Recipes page, its examples and the pack brief (spec 014, story 5: T047, T048; US5-AS1 to AS3, AS5, SC-005).
They read the REAL repository, so a page, an example or a brief that should exist and does not is an assertion failure
naming it. Standard library only. `go` (builds the venom binary into a temporary folder), Chromium (the web examples)
and `node` (the repository's secret scanner) are optional: the test that needs one is skipped when it is absent.
Run: python scripts/test_site_recipes.py
"""
import json
import os
import re
import shlex
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
PAGE = ROOT / "site" / "pages" / "recipes.md"
EXAMPLES = ROOT / "site" / "examples" / "recipes"
REQUIRED_EXAMPLES = ("web-page.yml", "web-execute.yml", "mobile-counter.yml")
BRIEF = ROOT / "docs" / "specs" / "014-web-mobile-recipes" / "pack-brief.md"
FIXTURE_PAGE = ROOT / "tests" / "recipes" / "fixtures" / "page" / "index.html"
SECRET_PATTERNS = ROOT / ".claude" / "skills" / "cst-g-commit" / "scripts" / "secret-patterns.js"
SECTIONS = (  # the exact `## ` headings of the page, in the order of US5-AS1
    "Folder layout",
    "Testcase names and the spec id",
    "Selectors",
    "Proof steps",
    "What the executors cannot do",
    "Credentials",
    "Results and artifacts",
    "Undefined variables",
)
NEEDS_TAG = re.compile(r"(?m)^#\s*needs:\s*(device|firefox|webkit)\b")
BUILD_TIMEOUT = 900
RUN_TIMEOUT = 300


# ---------- helpers ----------

def read(path: Path) -> str:
    return path.read_text(encoding="utf-8").replace("\r\n", "\n")


def sections(markdown: str) -> dict[str, str]:
    """`## heading` -> the text up to the next `## ` heading (headings inside code fences are not headings)."""
    result: dict[str, list[str]] = {}
    current, fenced = None, False
    for line in markdown.split("\n"):
        if line.lstrip().startswith(("```", "~~~")):
            fenced = not fenced
        match = None if fenced else re.match(r"##\s+(.+?)\s*$", line)
        if match:
            current = match.group(1)
            result[current] = []
        elif current is not None:
            result[current].append(line)
    return {name: "\n".join(lines) for name, lines in result.items()}


def yaml_subset_problems(text: str) -> list[str]:
    """What is wrong with a suite written in the YAML subset the recipes use: block mappings and lists, plain or quoted
    scalars, comments. Not a YAML parser (`venom recipe check` is the real one); it catches the breakage a hand edit makes."""
    problems = []
    if "\t" in text:
        problems.append("a tab character (YAML indents with spaces)")
    if not re.search(r"(?m)^name:\s*\S", text):
        problems.append("no top-level name:")
    if not re.search(r"(?m)^testcases:\s*$", text):
        problems.append("no top-level testcases:")
    for number, line in enumerate(text.split("\n"), 1):
        problems.extend(yaml_line_problems(number, line))
    return problems


def yaml_line_problems(number: int, line: str) -> list[str]:
    body = line.strip()
    if not body or body.startswith("#"):
        return []
    problems = []
    if (len(line) - len(line.lstrip(" "))) % 2:
        problems.append(f"line {number}: odd indentation")
    if not re.match(r"^(- )*([^\s:#][^:]*:(\s|$)|- \S|-$)", body) and not body.startswith(("- ", "|", ">")):
        problems.append(f"line {number}: neither a key, a list item nor a continuation: {body[:40]}")
    scalar = body.split(": ", 1)[-1] if ": " in body else body
    if scalar[:1] in "\"'" and scalar.count(scalar[0]) % 2 and not scalar.endswith(("|", ">")):
        problems.append(f"line {number}: unbalanced quote")
    return problems


def file_url(path: Path) -> str:
    return path.resolve().as_uri()


def chromium_exists() -> bool:
    for cache in (Path.home() / ".cache" / "ms-playwright", Path(os.environ.get("LOCALAPPDATA", "")) / "ms-playwright"):
        if cache.is_dir() and any(p.name.startswith("chromium") for p in cache.iterdir()):
            return True
    return False


def example_files() -> list[Path]:
    return sorted(EXAMPLES.glob("*.yml")) if EXAMPLES.is_dir() else []


def needs_a_browser_run(path: Path) -> bool:
    """A web example that is not tagged as needing another browser or a device runs against the static page."""
    text = read(path)
    return re.search(r"(?m)^\s*(- )?type:\s*web\s*$", text) is not None and not NEEDS_TAG.search(text)


class VenomBinary(unittest.TestCase):
    """Builds `venom` once into a temporary folder; every test of a subclass is skipped when `go` is missing."""
    binary: Path | None = None
    tmp: str | None = None

    @classmethod
    def setUpClass(cls):
        if shutil.which("go") is None:
            raise unittest.SkipTest("go is not installed: the venom binary cannot be built")
        cls.tmp = tempfile.mkdtemp(prefix="site-recipes-")
        cls.binary = Path(cls.tmp) / ("venom.exe" if sys.platform == "win32" else "venom")
        build = subprocess.run(["go", "build", "-o", str(cls.binary), "./cmd/venom"], cwd=ROOT, capture_output=True,
                               text=True, timeout=BUILD_TIMEOUT)
        if build.returncode != 0:
            shutil.rmtree(cls.tmp, ignore_errors=True)
            raise AssertionError(f"go build ./cmd/venom failed:\n{build.stdout}{build.stderr}")

    @classmethod
    def tearDownClass(cls):
        if cls.tmp:
            shutil.rmtree(cls.tmp, ignore_errors=True)

    def venom(self, *args: str, cwd: Path = ROOT) -> subprocess.CompletedProcess:
        return subprocess.run([str(self.binary), *args], cwd=cwd, capture_output=True, text=True, timeout=RUN_TIMEOUT)

    def check(self, *paths: Path) -> subprocess.CompletedProcess:
        return self.venom("recipe", "check", "--specs-dir", str(ROOT / "docs" / "specs"), *map(str, paths))

    def replay(self, recipe: Path, address: str) -> subprocess.CompletedProcess:
        """`venom run` of a recipe in a clean folder, the address given as the variable appUrl."""
        with tempfile.TemporaryDirectory(prefix="site-recipes-run-") as work:
            return self.venom("run", "--output-dir", work, "--var", f"appUrl={address}", str(recipe), cwd=Path(work))


# ---------- T047: the page ----------

class RecipesPage(unittest.TestCase):
    def test_the_page_source_exists(self):
        self.assertTrue(PAGE.is_file(), "site/pages/recipes.md does not exist")

    def test_the_site_lists_the_page_once_in_the_guides_group(self):
        config = json.loads(read(ROOT / "site" / "site.json"))
        entries = [p for p in config["pages"] if p.get("slug") == "recipes"]
        self.assertEqual(len(entries), 1, "site/site.json must hold exactly one page with the slug recipes")
        self.assertEqual(entries[0].get("group"), "Guides")
        self.assertEqual(entries[0].get("source"), "site/pages/recipes.md")

    def test_the_page_has_the_sections_of_us5_as1_in_order(self):
        self.assertTrue(PAGE.is_file(), "site/pages/recipes.md does not exist")
        found = [name for name in sections(read(PAGE)) if name in SECTIONS]
        self.assertEqual(found, list(SECTIONS), "the `## ` headings the page must have, in this order")

    def content(self, section: str) -> str:
        self.assertTrue(PAGE.is_file(), "site/pages/recipes.md does not exist")
        self.assertIn(section, sections(read(PAGE)), f"no `## {section}` section")
        return sections(read(PAGE))[section]

    def test_the_layout_section_names_the_recipe_folder(self):
        self.assertRegex(self.content("Folder layout"), r"tests/")

    def test_the_naming_section_gives_the_id_form_and_the_spec_variable(self):
        text = self.content("Testcase names and the spec id")
        self.assertRegex(text, r"US\d+-AS\d+|USn-ASm")
        self.assertRegex(text, r"\bspec\b")

    def test_the_selector_section_names_css_xpath_and_the_stable_test_attribute(self):
        text = self.content("Selectors")
        self.assertRegex(text, r"CSS")
        self.assertRegex(text, r"XPath")
        self.assertRegex(text, r"data-test")

    def test_the_proof_section_names_replay_negative_control_and_the_check_command(self):
        text = self.content("Proof steps")
        self.assertRegex(text, r"(?i)replay")
        self.assertRegex(text, r"(?i)negative control")
        self.assertIn("venom recipe check", text)

    def test_the_limits_section_names_the_execute_fallback(self):
        self.assertRegex(self.content("What the executors cannot do"), r"\bexecute\b")

    def test_the_credentials_section_says_they_come_from_variables(self):
        self.assertRegex(self.content("Credentials"), r"(?i)variable")

    def test_the_artifacts_section_keeps_results_out_of_git_and_ci_retention_short_and_restricted(self):
        text = self.content("Results and artifacts")
        self.assertRegex(text, r"\bgit\b")
        self.assertRegex(text, r"\bCI\b")
        self.assertRegex(text, r"(?i)\bshort\b")
        self.assertRegex(text, r"(?i)\brestricted\b")

    def test_the_undefined_variable_section_states_both_results(self):
        text = self.content("Undefined variables")
        self.assertIn("{{.password}}", text)
        self.assertIn("--strict-variables", text)
        self.assertRegex(text, r"(?i)\bliteral\b")
        self.assertRegex(text, r"(?i)exit(?: code| status)? 2\b")

    def test_every_example_file_is_included_by_the_page_and_the_required_ones_exist(self):
        for name in REQUIRED_EXAMPLES:
            self.assertTrue((EXAMPLES / name).is_file(), f"site/examples/recipes/{name} does not exist")
        self.assertTrue(PAGE.is_file(), "site/pages/recipes.md does not exist")
        included = set(re.findall(r"(?m)^\{\{include (site/examples/recipes/\S+\.yml) yaml\}\}\s*$", read(PAGE)))
        for path in example_files():
            self.assertIn(f"site/examples/recipes/{path.name}", included, f"{path.name} is not included by the page")
        for rel in included:
            self.assertTrue((ROOT / rel).is_file(), f"the page includes {rel}, which does not exist")


# ---------- T047: the examples ----------

class Examples(unittest.TestCase):
    def test_the_examples_exist(self):
        self.assertTrue(example_files(), "site/examples/recipes holds no .yml example")

    def test_every_example_is_in_the_yaml_subset_of_a_suite(self):
        self.assertTrue(example_files(), "site/examples/recipes holds no .yml example")
        for path in example_files():
            self.assertEqual(yaml_subset_problems(read(path)), [], path.name)

    def test_the_subset_check_finds_a_broken_example(self):
        broken = "name: x\ntestcases:\n\t- name: y\n"
        self.assertTrue(yaml_subset_problems(broken))
        self.assertTrue(yaml_subset_problems("name: x\ntestcases:\n - name: 'y\n"))
        self.assertTrue(yaml_subset_problems("testcases:\n- name: y\n"))

    def test_a_mobile_example_is_tagged_as_needing_a_device(self):
        mobile = [p for p in example_files() if re.search(r"(?m)^\s*(- )?type:\s*mobile\s*$", read(p))]
        self.assertTrue(mobile, "no example drives the mobile executor")
        for path in mobile:
            self.assertRegex(read(path), r"(?m)^#\s*needs:\s*device\b", f"{path.name} needs a device and is not tagged `# needs: device`")

    def test_the_untagged_web_examples_include_one_that_reads_the_scroll_width_through_execute(self):
        runnable = [p for p in example_files() if needs_a_browser_run(p)]
        self.assertGreaterEqual(len(runnable), 2, "at least web-page.yml and web-execute.yml run against the static page")
        execute = EXAMPLES / "web-execute.yml"
        self.assertTrue(execute.is_file(), "site/examples/recipes/web-execute.yml does not exist")
        text = read(execute)
        for word in ("execute", "scrollWidth", "clientWidth"):
            self.assertIn(word, text)


class ExamplesCheck(VenomBinary):
    def test_every_example_passes_the_recipe_check(self):
        self.assertTrue(example_files(), "site/examples/recipes holds no .yml example")
        result = self.check(EXAMPLES)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_a_stale_example_fails_the_check(self):
        with tempfile.TemporaryDirectory() as tmp:
            stale = Path(tmp) / "stale.yml"
            stale.write_text("name: stale\nvars:\n  spec: 014-web-mobile-recipes\ntestcases:\n- name: US1-AS1 no assertion\n"
                             "  steps:\n  - type: web\n    action:\n      find: h1\n", encoding="utf-8")
            result = self.check(stale)
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        self.assertIn("RC05", result.stdout)

    def test_a_copy_that_does_not_parse_exits_2(self):
        with tempfile.TemporaryDirectory() as tmp:
            broken = Path(tmp) / "broken.yml"
            broken.write_text("name: [unclosed\n", encoding="utf-8")
            self.assertEqual(self.check(broken).returncode, 2)


@unittest.skipUnless(chromium_exists(), "Chromium is not installed (Playwright cache absent): the web examples are not run")
@unittest.skipUnless(FIXTURE_PAGE.is_file(), "tests/recipes/fixtures/page/index.html is absent")
class WebExamplesReplay(VenomBinary):
    def test_every_untagged_web_example_passes_against_the_static_page(self):
        runnable = [p for p in example_files() if needs_a_browser_run(p)]
        self.assertGreaterEqual(len(runnable), 2, "at least web-page.yml and web-execute.yml run against the static page")
        for path in runnable:
            result = self.replay(path, file_url(FIXTURE_PAGE))
            self.assertEqual(result.returncode, 0, f"{path.name}:\n{result.stdout}{result.stderr}")

    def test_an_example_that_stops_passing_fails_the_run(self):
        runnable = [p for p in example_files() if needs_a_browser_run(p)]
        self.assertTrue(runnable, "no untagged web example")
        with tempfile.TemporaryDirectory() as tmp:
            missing = Path(tmp) / "no-such-page.html"  # the page the example reads is gone: the example is stale
            for path in runnable:
                result = self.replay(path, file_url(missing))
                self.assertNotEqual(result.returncode, 0, f"{path.name} still passes with no page to read")


# ---------- T048: the pack brief ----------

COMMAND = re.compile(r"^(?:\$\s+)?((?:venom recipe check|go build)\b.*)$")


def brief_commands(text: str) -> list[str]:
    """The `venom recipe check` and `go build` lines of the fenced blocks of the brief."""
    commands, fenced = [], False
    for line in text.split("\n"):
        if line.lstrip().startswith(("```", "~~~")):
            fenced = not fenced
            continue
        match = COMMAND.match(line.strip()) if fenced else None
        if match:
            commands.append(match.group(1))
    return commands


class BriefText(unittest.TestCase):
    def test_the_brief_exists(self):
        self.assertTrue(BRIEF.is_file(), "docs/specs/014-web-mobile-recipes/pack-brief.md does not exist")

    def test_the_brief_states_the_duties_the_limits_and_the_commands(self):
        self.assertTrue(BRIEF.is_file(), "pack-brief.md does not exist")
        text = read(BRIEF).lower()
        for word in ("explore", "write", "replay", "negative control", "check", "report", "assume", "static"):
            self.assertIn(word, text, f"the brief never says `{word}`")
        self.assertRegex(text, r"cannot prove|can not prove|does not prove")
        self.assertIn("--strict-variables", text, "the brief must state the result of an undefined variable")

    def test_the_command_extractor_reads_fenced_lines_only(self):
        text = "venom recipe check x\n```sh\nvenom recipe check a b\n$ go build ./cmd/venom\necho hi\n```\n"
        self.assertEqual(brief_commands(text), ["venom recipe check a b", "go build ./cmd/venom"])

    def test_the_brief_holds_a_runnable_check_command_and_a_build_command(self):
        self.assertTrue(BRIEF.is_file(), "pack-brief.md does not exist")
        commands = brief_commands(read(BRIEF))
        runnable = [c for c in commands if "<" not in c]
        self.assertTrue([c for c in runnable if c.startswith("venom recipe check")], "no runnable `venom recipe check` line (no <placeholder>)")
        self.assertTrue([c for c in commands if c.startswith("go build")], "no `go build` line")


class BriefCommands(VenomBinary):
    def test_the_commands_that_need_no_device_succeed(self):
        self.assertTrue(BRIEF.is_file(), "pack-brief.md does not exist")
        commands = [c for c in brief_commands(read(BRIEF)) if "<" not in c]
        self.assertTrue(commands, "the brief holds no runnable command")
        for command in commands:
            args = shlex.split(command)
            if args[0] == "go":
                if "-o" not in args:
                    args[2:2] = ["-o", str(Path(self.tmp) / "brief-build")]
                result = subprocess.run(args, cwd=ROOT, capture_output=True, text=True, timeout=BUILD_TIMEOUT)
            else:
                result = self.venom(*args[1:])
            self.assertEqual(result.returncode, 0, f"`{command}`:\n{result.stdout}{result.stderr}")


# ---------- T048: the secret scanner ----------

SCAN = """const fs=require('fs');const {findSecret}=require(process.argv[1]);const out=[];
for(const f of process.argv.slice(2)){fs.readFileSync(f,'utf8').split(/\\r?\\n/).forEach((l,i)=>{const s=findSecret(l);if(s)out.push(f+':'+(i+1)+': '+s.kind);});}
console.log(JSON.stringify(out));"""


def scan_secrets(files: list[Path]) -> list[str]:
    result = subprocess.run(["node", "-e", SCAN, str(SECRET_PATTERNS), *map(str, files)], capture_output=True, text=True, timeout=120)
    if result.returncode != 0:
        raise AssertionError(f"the scan did not run: {result.stderr}")
    return json.loads(result.stdout)


def files_under(folder: Path) -> list[Path]:
    """The files of a folder outside `fixtures/secrets/` (the scanner exempts that folder by design)."""
    return [p for p in sorted(folder.rglob("*")) if p.is_file() and "fixtures/secrets" not in p.as_posix()]


@unittest.skipUnless(shutil.which("node"), "node is not installed: the repository's secret scanner is not run")
@unittest.skipUnless(SECRET_PATTERNS.is_file(), "the secret scanner (.claude/skills/cst-g-commit/scripts/secret-patterns.js) is absent")
class SecretScan(unittest.TestCase):
    def test_the_scan_finds_a_planted_credential(self):
        with tempfile.TemporaryDirectory() as tmp:
            planted = Path(tmp) / "planted.yml"
            planted.write_text("key: " + "AKIA" + "ABCDEFGHIJKLMNOP" + "\n", encoding="utf-8")
            self.assertEqual(len(scan_secrets([planted])), 1)

    def test_the_recipes_the_page_the_examples_and_the_brief_hold_no_credential(self):
        files = files_under(ROOT / "tests" / "recipes") + files_under(EXAMPLES) + [PAGE, BRIEF]
        missing = [str(p.relative_to(ROOT)) for p in (PAGE, BRIEF) if not p.is_file()]
        self.assertEqual(missing, [], "files the scan must cover do not exist")
        self.assertTrue(example_files(), "site/examples/recipes holds no example to scan")
        self.assertEqual(scan_secrets(files), [])


if __name__ == "__main__":
    unittest.main()
