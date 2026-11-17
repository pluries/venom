#!/usr/bin/env python3
"""Reviewed exceptions of the secret detector (spec 013, D-022 amended): the translated grpc, ovhapi and sql pages repeat the
English pages' placeholder fences, so three file names per language carry one reviewed kind of finding each. The exception is
`isAllowedFinding(file, kind)` of secret-patterns.js, fed by docs/compliance/publish-scan-allow.txt (the file
scripts/publish-github.sh reads), NOT a path exemption of `isFixturePath`, which would skip every kind and the public scan too.
Drives node through subprocesses; needs node (and git for the commit-scan class). Offline, standard library only.
The placeholder lines are read from the real French pages; planted values are assembled at run time.
Run: python scripts/test_site_secret_patterns.py
"""
import json
import os
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from test_build_site import ROOT  # noqa: E402

NODE = shutil.which("node")
GIT = shutil.which("git")
PATTERNS = ROOT / ".claude" / "skills" / "cst-g-commit" / "scripts" / "secret-patterns.js"
GUARD = ROOT / ".claude" / "hooks" / "secrets-guard.js"
SCAN = ROOT / ".claude" / "skills" / "cst-g-commit" / "scripts" / "scan-secrets.js"
ALLOW_FILE = ROOT / "docs" / "compliance" / "publish-scan-allow.txt"
LANGUAGES = ("fr", "de", "it", "es", "zh")
KINDS = {"grpc": "private key block", "ovhapi": "generic secret assignment", "sql": "URL with embedded credentials"}
PAIRS = [(f"site/i18n/{lang}/{name}.md", kind) for lang in LANGUAGES for name, kind in KINDS.items()]


def node_json(script: str, cwd: Path = ROOT):
    """Evaluate `script` in node with `P` = secret-patterns.js; it prints one JSON value."""
    code = f"const P=require({json.dumps(str(PATTERNS))});const out=({script});console.log(JSON.stringify(out));"
    done = subprocess.run([NODE, "-e", code], cwd=cwd, capture_output=True, text=True, encoding="utf-8", timeout=60)
    if done.returncode != 0:
        raise AssertionError(done.stderr)
    return json.loads(done.stdout)


def allowed(file: str, kind: str, cwd: Path = ROOT):
    return node_json(f"P.isAllowedFinding({json.dumps(file)},{json.dumps(kind)})", cwd)


def run_guard(file_path: str, content: str, cwd: Path = ROOT) -> subprocess.CompletedProcess:
    payload = json.dumps({"tool_name": "Write", "tool_input": {"file_path": file_path, "content": content}})
    env = {key: value for key, value in os.environ.items() if key != "IM_SECRETS_GUARD"}
    return subprocess.run([NODE, str(GUARD)], input=payload, cwd=cwd, env=env, capture_output=True, text=True, encoding="utf-8", timeout=60)


def aws_key() -> str:
    return "AK" + "IA" + "QZ7XW3NB5LM2HRTY"


def real_lines_flagged(path: str) -> list[tuple[str, str]]:
    """What findSecret reports on the real page: (kind, line) pairs, so the tests meet the real content."""
    script = f"require('fs').readFileSync({json.dumps(str(ROOT / path))},'utf8').split(/\\r?\\n/).map(l=>[P.findSecret(l),l]).filter(x=>x[0]).map(x=>[x[0].kind,x[1]])"
    return [tuple(item) for item in node_json(script)]


def placeholder(name: str) -> str:
    """The first line of the French `<name>` page the detector flags: the deliberate placeholder of that page."""
    return real_lines_flagged(f"site/i18n/fr/{name}.md")[0][1]


@unittest.skipUnless(NODE, "needs node on PATH")
class IsFixturePathNoLongerExemptsTheSitePages(unittest.TestCase):
    def test_the_translated_pages_are_not_fixture_paths(self):
        for path, _ in PAIRS:
            with self.subTest(path=path):
                self.assertFalse(node_json(f"P.isFixturePath({json.dumps(path)})"))
                self.assertFalse(node_json(f"P.isFixturePath({json.dumps(str(ROOT / path))})"))

    def test_the_other_fixture_paths_are_untouched(self):
        for path in (".claude/skills/x/SKILL.md", "evals/fixtures/a.txt", "config.example"):
            with self.subTest(path=path):
                self.assertTrue(node_json(f"P.isFixturePath({json.dumps(path)})"))


@unittest.skipUnless(NODE, "needs node on PATH")
class ReviewedExceptionsFile(unittest.TestCase):
    def test_the_file_lists_the_fifteen_pages_with_their_kind(self):
        lines = {line.strip() for line in ALLOW_FILE.read_text(encoding="utf-8").splitlines()}
        for path, kind in PAIRS:
            with self.subTest(path=path):
                self.assertIn(f"{path}|{kind}", lines)

    def test_each_listed_kind_is_what_the_detector_reports_on_the_real_page_and_nothing_else(self):
        for path, kind in PAIRS:
            with self.subTest(path=path):
                self.assertEqual({found for found, _ in real_lines_flagged(path)}, {kind})


@unittest.skipUnless(NODE, "needs node on PATH")
class IsAllowedFinding(unittest.TestCase):
    def test_true_for_each_page_and_its_own_kind(self):
        for path, kind in PAIRS:
            with self.subTest(path=path):
                self.assertTrue(allowed(path, kind))

    def test_false_for_another_kind_in_the_same_file(self):
        for path, kind in PAIRS:
            with self.subTest(path=path):
                self.assertFalse(allowed(path, "AWS access key id"))
                self.assertFalse(allowed(path, next(other for other in KINDS.values() if other != kind)))

    def test_false_for_another_file(self):
        for path in ("site/i18n/fr/http.md", "site/i18n/fr/sql.md.bak", "site/i18n/xx/sql.md", "site/pages/sql.md", "x/site/i18n/fr/sql.md"):
            with self.subTest(path=path):
                self.assertFalse(allowed(path, KINDS["sql"]))

    def test_false_for_a_path_with_dot_dot_even_under_an_allowed_prefix(self):
        for path in ("site/i18n/fr/../fr/sql.md", "vendor/../secret.txt", "vendor/a/../../x.txt", "../site/i18n/fr/sql.md"):
            with self.subTest(path=path):
                self.assertFalse(allowed(path, "*" if path.startswith("vendor") else KINDS["sql"]))
                self.assertFalse(allowed(path, KINDS["sql"]))

    def test_the_semantics_of_publish_github_sh_a_trailing_slash_is_a_prefix_and_a_star_allows_every_kind(self):
        self.assertTrue(allowed("vendor/github.com/a/b.go", "JWT"))
        self.assertTrue(allowed("executors/grpc/README.md", "private key block"))
        self.assertFalse(allowed("executors/grpc/README.md", "JWT"))
        self.assertFalse(allowed("executors/grpc/README.md.old", "private key block"))

    def test_an_absolute_path_inside_the_repository_is_read_relative_to_it(self):
        """The hook receives absolute paths."""
        self.assertTrue(allowed(str(ROOT / "site" / "i18n" / "fr" / "sql.md"), KINDS["sql"]))
        self.assertFalse(allowed(str(ROOT / "site" / "i18n" / "fr" / "sql.md"), "AWS access key id"))
        self.assertFalse(allowed(str(ROOT.parent / "elsewhere" / "site" / "i18n" / "fr" / "sql.md"), KINDS["sql"]))

    def test_the_exceptions_file_is_found_from_the_script_when_the_working_directory_has_none(self):
        with tempfile.TemporaryDirectory() as scratch:
            self.assertTrue(allowed("site/i18n/fr/sql.md", KINDS["sql"], Path(scratch)))

    def test_the_working_directory_file_wins_when_it_exists(self):
        with tempfile.TemporaryDirectory() as scratch:
            target = Path(scratch) / "docs" / "compliance"
            target.mkdir(parents=True)
            (target / "publish-scan-allow.txt").write_text("# nothing reviewed here\n", encoding="utf-8")
            self.assertFalse(allowed("site/i18n/fr/sql.md", KINDS["sql"], Path(scratch)))
            (target / "publish-scan-allow.txt").write_text("a/b.txt|JWT\n", encoding="utf-8")
            self.assertTrue(allowed("a/b.txt", "JWT", Path(scratch)))


@unittest.skipUnless(NODE, "needs node on PATH")
class SecretsGuardHook(unittest.TestCase):
    def test_the_dsn_placeholder_in_a_translated_sql_page_is_let_through(self):
        for path in ("site/i18n/fr/sql.md", str(ROOT / "site" / "i18n" / "zh" / "sql.md")):
            with self.subTest(path=path):
                done = run_guard(path, placeholder("sql") + "\n")
                self.assertEqual(done.returncode, 0, done.stderr)

    def test_each_translated_page_lets_its_own_placeholder_through(self):
        for name in KINDS:
            with self.subTest(page=name):
                self.assertEqual(run_guard(f"site/i18n/de/{name}.md", placeholder(name) + "\n").returncode, 0)

    def test_the_same_content_in_another_page_is_blocked(self):
        for path in ("site/i18n/fr/http.md", "site/pages/sql.md", "site/i18n/fr/sql.txt"):
            with self.subTest(path=path):
                done = run_guard(path, placeholder("sql") + "\n")
                self.assertEqual(done.returncode, 2, done.stderr)

    def test_a_real_looking_key_in_the_translated_sql_page_is_still_blocked_because_its_kind_differs(self):
        for path in ("site/i18n/fr/sql.md", str(ROOT / "site" / "i18n" / "fr" / "sql.md")):
            with self.subTest(path=path):
                done = run_guard(path, f"aws_key: {aws_key()}\n")
                self.assertEqual(done.returncode, 2, done.stderr)
                self.assertIn("AWS access key id", done.stderr)

    def test_the_allowed_kind_does_not_hide_another_kind_on_other_lines_of_the_same_write(self):
        done = run_guard("site/i18n/fr/sql.md", placeholder("sql") + f"\nkey {aws_key()}\n")
        self.assertEqual(done.returncode, 2, done.stderr)
        self.assertNotIn("oracle://", done.stderr, "the reviewed line is not reported")


@unittest.skipUnless(NODE and GIT, "needs node and git on PATH")
class CommitScan(unittest.TestCase):
    """scan-secrets.js on a scratch repository: it carries a copy of the exceptions file, as the real tree does."""

    def scratch(self, path: str, content: str) -> Path:
        root = Path(tempfile.mkdtemp(prefix="scan-allow-"))
        self.addCleanup(shutil.rmtree, root, True)
        env = {**os.environ, "GIT_AUTHOR_NAME": "fixture", "GIT_AUTHOR_EMAIL": "f@example.invalid",
               "GIT_COMMITTER_NAME": "fixture", "GIT_COMMITTER_EMAIL": "f@example.invalid"}

        def git(*args: str) -> None:
            subprocess.run([GIT, *args], cwd=root, env=env, check=True, capture_output=True, timeout=60)

        git("init", "-q")
        (root / "README.md").write_text("scratch\n", encoding="utf-8")
        (root / "docs" / "compliance").mkdir(parents=True)
        shutil.copy(ALLOW_FILE, root / "docs" / "compliance" / "publish-scan-allow.txt")
        git("add", "-A")
        git("-c", "commit.gpgsign=false", "commit", "-q", "-m", "base")
        (root / path).parent.mkdir(parents=True, exist_ok=True)
        (root / path).write_text(content, encoding="utf-8", newline="\n")
        git("add", path)
        return root

    def scan(self, root: Path) -> subprocess.CompletedProcess:
        return subprocess.run([NODE, str(SCAN), "--staged"], cwd=root, capture_output=True, text=True, encoding="utf-8", timeout=60)

    def test_the_dsn_placeholder_staged_in_a_translated_sql_page_passes(self):
        done = self.scan(self.scratch("site/i18n/fr/sql.md", placeholder("sql") + "\n"))
        self.assertEqual(done.returncode, 0, done.stderr)

    def test_the_same_line_staged_in_another_page_blocks(self):
        done = self.scan(self.scratch("site/i18n/fr/http.md", placeholder("sql") + "\n"))
        self.assertEqual(done.returncode, 1, done.stderr)
        self.assertIn("site/i18n/fr/http.md", done.stderr)

    def test_a_real_looking_key_staged_in_the_translated_sql_page_blocks(self):
        done = self.scan(self.scratch("site/i18n/fr/sql.md", f"key {aws_key()}\n"))
        self.assertEqual(done.returncode, 1, done.stderr)
        self.assertIn("AWS access key id", done.stderr)


if __name__ == "__main__":
    unittest.main()
