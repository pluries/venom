#!/usr/bin/env python3
"""Tests of the site image checks (story #3622, task #3662).
Findings start with a kind prefix: leak: diff: user: history: license: language:
T6 and T7 (spec 013, task T121, US5-AS2, FR-022) cover the language folders of the image.
Offline, standard library only. Run: python scripts/test_site_image_check.py
"""
import argparse
import contextlib
import io
import json
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock

sys.path.insert(0, str(Path(__file__).resolve().parent))
import site_image_check as sc  # noqa: E402

HOST = "docs.example.test"
PRIVATE_PATHS = ("docs/compliance",)
NAME = "example-private.test"
CLEAN_PAGE = '<!doctype html><html><head><title>Venom</title></head><body><a href="/guide/">Guide</a></body></html>'
CLEAN_CONF = "worker_processes 1;\nevents {}\nhttp { server { listen 8080; root /usr/share/nginx/html; } }\n"


def write(root: Path, rel: str, text: str) -> Path:
    path = root / rel
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding="utf-8")
    return path


def make_export(root: Path) -> None:
    write(root, sc.HTML_DIR + "/index.html", CLEAN_PAGE)
    write(root, sc.NGINX_CONF, CLEAN_CONF)
    write(root, sc.LICENSE_FILE, "Apache License 2.0\n")


class TempCase(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.root = Path(tmp.name)


class T1LeakTests(TempCase):
    def scan(self, names=()):
        return sc.leak_scan(self.root, HOST, PRIVATE_PATHS, names)

    def setUp(self):
        super().setUp()
        make_export(self.root)

    def test_T1_clean_export_has_no_finding(self):
        self.assertEqual(self.scan((NAME,)), [])

    def assert_found(self, rel: str, findings: list[str]):
        self.assertTrue(findings, "no finding")
        self.assertTrue(all(f.startswith("leak:") for f in findings), findings)
        self.assertTrue(any(rel in f for f in findings), findings)

    def plant(self, rel: str, text: str, names=()):
        write(self.root, rel, text)
        return self.scan(names)

    def test_T1_private_path_in_page(self):
        rel = sc.HTML_DIR + "/index.html"
        self.assert_found(rel, self.plant(rel, CLEAN_PAGE.replace("Guide", "see docs/compliance/plan.md")))

    def test_T1_private_ip_in_page(self):
        rel = sc.HTML_DIR + "/index.html"
        self.assert_found(rel, self.plant(rel, CLEAN_PAGE.replace("Guide", "host 10.1.2.3")))

    def test_T1_dev_azure_in_page(self):
        rel = sc.HTML_DIR + "/index.html"
        self.assert_found(rel, self.plant(rel, CLEAN_PAGE.replace("Guide", "https://dev.azure.com/org")))

    def test_T1_private_name_in_page(self):
        rel = sc.HTML_DIR + "/index.html"
        self.assert_found(rel, self.plant(rel, CLEAN_PAGE.replace("Guide", NAME), (NAME,)))

    def test_T1_private_path_in_nginx_conf(self):
        self.assert_found(sc.NGINX_CONF, self.plant(sc.NGINX_CONF, CLEAN_CONF + "# docs/compliance/x\n"))

    def test_T1_private_ip_in_nginx_conf(self):
        self.assert_found(sc.NGINX_CONF, self.plant(sc.NGINX_CONF, CLEAN_CONF + "# upstream 192.168.1.5\n"))

    def test_T1_dev_azure_in_nginx_conf(self):
        self.assert_found(sc.NGINX_CONF, self.plant(sc.NGINX_CONF, CLEAN_CONF + "# dev.azure.com/org\n"))

    def test_T1_private_name_in_nginx_conf(self):
        self.assert_found(sc.NGINX_CONF, self.plant(sc.NGINX_CONF, CLEAN_CONF + f"# {NAME}\n", (NAME,)))

    def test_T1_private_name_ignored_when_not_given(self):
        self.assertEqual(self.plant(sc.NGINX_CONF, CLEAN_CONF + f"# {NAME}\n"), [])


class T2CompareTests(TempCase):
    def setUp(self):
        super().setUp()
        self.left = self.root / "left"
        self.right = self.root / "right"
        for side in (self.left, self.right):
            write(side, "index.html", "a")
            write(side, "guide/page.html", "b")

    def test_T2_equal_trees_have_no_difference(self):
        self.assertEqual(sc.compare_trees(self.left, self.right), [])

    def test_T2_changed_file(self):
        write(self.right, "guide/page.html", "changed")
        found = sc.compare_trees(self.left, self.right)
        self.assertEqual(len(found), 1)
        self.assertTrue(found[0].startswith("diff:") and "guide/page.html" in found[0], found)

    def test_T2_one_byte_difference(self):
        (self.right / "index.html").write_bytes(b"b")
        self.assertEqual(len(sc.compare_trees(self.left, self.right)), 1)

    def test_T2_extra_file(self):
        write(self.right, "new/extra.html", "x")
        found = sc.compare_trees(self.left, self.right)
        self.assertEqual(len(found), 1)
        self.assertTrue(found[0].startswith("diff:") and "new/extra.html" in found[0], found)

    def test_T2_missing_file(self):
        (self.right / "index.html").unlink()
        found = sc.compare_trees(self.left, self.right)
        self.assertEqual(len(found), 1)
        self.assertTrue(found[0].startswith("diff:") and "index.html" in found[0], found)

    def test_T2_one_finding_each_kind(self):
        write(self.right, "guide/page.html", "changed")
        write(self.right, "extra.html", "x")
        (self.right / "index.html").unlink()
        self.assertEqual(len(sc.compare_trees(self.left, self.right)), 3)


def inspect(user) -> str:
    return json.dumps([{"Config": {"User": user}}])


class T3UserTests(unittest.TestCase):
    def test_T3_uid_and_gid_ok(self):
        self.assertEqual(sc.check_user(inspect("101:101")), [])

    def test_T3_uid_only_ok(self):
        self.assertEqual(sc.check_user(inspect("101")), [])

    def test_T3_parsed_list_and_object_accepted(self):
        self.assertEqual(sc.check_user([{"Config": {"User": "101"}}]), [])
        self.assertEqual(sc.check_user({"Config": {"User": "101"}}), [])

    def test_T3_bad_users_fail(self):
        for user in ("", "0", "0:0", "root", "nginx", "101:0"):
            with self.subTest(user=user):
                found = sc.check_user(inspect(user))
                self.assertTrue(found and all(f.startswith("user:") for f in found), found)

    def test_T3_missing_user_key_fails(self):
        found = sc.check_user(json.dumps([{"Config": {}}]))
        self.assertTrue(found and found[0].startswith("user:"), found)


class T4HistoryTests(unittest.TestCase):
    CLEAN = (
        "IMAGE CREATED CREATED BY SIZE\n"
        "abc 2 days ago USER 101:101 0B\n"
        "def 2 days ago COPY /out/ /usr/share/nginx/html/ # buildkit 2MB\n"
        "ghi 2 days ago EXPOSE 8080 0B\n"
    )

    def test_T4_clean_history_has_no_finding(self):
        self.assertEqual(sc.scan_history(self.CLEAN, (NAME,)), [])

    def test_T4_findings(self):
        cases = {
            "private ip": "RUN curl http://10.0.0.7/pkg",
            "private 192": "ENV PROXY=192.168.1.9",
            "azure": "RUN git clone https://dev.azure.com/org/p/_git/r",
            "password": "ENV password=hunter2",
            "token": "RUN login --token abcdef0123456789abcdef0123456789",
        }
        for label, line in cases.items():
            with self.subTest(label=label):
                found = sc.scan_history(self.CLEAN + line + "\n")
                self.assertTrue(found and all(f.startswith("history:") for f in found), found)

    def test_T4_private_name(self):
        found = sc.scan_history(self.CLEAN + f"RUN ping {NAME}\n", (NAME,))
        self.assertTrue(found and found[0].startswith("history:"), found)

    def test_T4_private_name_ignored_when_not_given(self):
        self.assertEqual(sc.scan_history(self.CLEAN + f"RUN ping {NAME}\n"), [])


class T5LicenseTests(TempCase):
    def test_T5_present_is_ok(self):
        write(self.root, sc.LICENSE_FILE, "Apache License 2.0\n")
        self.assertEqual(sc.check_license(self.root), [])

    def test_T5_missing_fails(self):
        found = sc.check_license(self.root)
        self.assertTrue(found and found[0].startswith("license:"), found)

    def test_T5_empty_fails(self):
        write(self.root, sc.LICENSE_FILE, "")
        found = sc.check_license(self.root)
        self.assertTrue(found and found[0].startswith("license:"), found)


LANGUAGE_FOLDERS = ("fr", "de", "zh")
LANGUAGES = {"en": {"name": "English", "path": "", "lang": "en"},
             "fr": {"name": "French", "path": "fr", "lang": "fr", "enabled": True},
             "de": {"name": "German", "path": "de", "lang": "de", "enabled": False},
             "zh": {"name": "Chinese", "path": "zh", "lang": "zh-Hans", "enabled": True},
             "it": {"name": "Italian", "path": "it", "lang": "it"}}


def make_language_export(root: Path, folders=LANGUAGE_FOLDERS) -> None:
    """An exported image whose pages sit at the root (English) and in one folder per language."""
    make_export(root)
    for folder in folders:
        write(root, f"{sc.HTML_DIR}/{folder}/index.html", CLEAN_PAGE)
        write(root, f"{sc.HTML_DIR}/{folder}/guide.html", CLEAN_PAGE)
        write(root, f"{sc.HTML_DIR}/{folder}/llms.txt", "# Venom\n")


class T6LanguageFolderTests(TempCase):
    """Characterization (the checks already walk every folder): the leak scan and the tree comparison cover the language folders."""

    def setUp(self):
        super().setUp()
        make_language_export(self.root)

    def test_T6_clean_language_folders_have_no_leak_finding(self):
        self.assertEqual(sc.leak_scan(self.root, HOST, PRIVATE_PATHS, (NAME,)), [])

    def test_T6_a_leak_in_a_language_page_is_found_and_names_the_page(self):
        cases = (("fr/guide.html", "<p>see docs/compliance/pipelines.md</p>"), ("zh/index.html", "<p>host 10.1.2.3</p>"),
                 ("de/llms.txt", f"# {NAME}\n"))
        for rel, text in cases:
            write(self.root, f"{sc.HTML_DIR}/{rel}", text)
            with self.subTest(page=rel):
                found = sc.leak_scan(self.root, HOST, PRIVATE_PATHS, (NAME,))
                self.assertTrue(found and all(f.startswith("leak:") for f in found), found)
                self.assertTrue(any(rel in f for f in found), found)
            write(self.root, f"{sc.HTML_DIR}/{rel}", CLEAN_PAGE)

    def test_T6_the_page_set_of_the_image_must_equal_the_build_output_language_folders_included(self):
        built = self.root / "built"
        for rel in ("index.html", *(f"{folder}/{name}" for folder in LANGUAGE_FOLDERS for name in ("index.html", "guide.html", "llms.txt"))):
            write(built, rel, CLEAN_PAGE if rel.endswith(".html") else "# Venom\n")
        html = self.root / sc.HTML_DIR
        self.assertEqual(sc.compare_trees(built, html), [])
        (html / "fr" / "guide.html").unlink()
        write(html, "es/index.html", CLEAN_PAGE)
        write(html, "de/index.html", CLEAN_PAGE + "<!-- changed -->")
        self.assertEqual(sc.compare_trees(built, html), ["diff: missing fr/guide.html", "diff: extra es/index.html", "diff: changed de/index.html"])


class T7LanguageGapTests(TempCase):
    """US5-AS2, FR-022: without a reference build to compare with, the image must still hold the folder of every enabled language and
    none of a language that is not enabled (a folder left by a preview, or by an earlier build, is a finding)."""

    def check(self, folders=("fr", "zh"), languages=None) -> list[str]:
        make_language_export(self.root, folders)
        return sc.check_languages(self.root, {"languages": LANGUAGES if languages is None else languages})

    def test_T7_the_folders_of_the_enabled_languages_and_nothing_else_have_no_finding(self):
        self.assertEqual(self.check(), [])

    def test_T7_an_enabled_language_without_its_folder_is_a_finding_that_names_it(self):
        found = self.check(folders=("fr",))
        self.assertEqual(len(found), 1, found)
        self.assertTrue(found[0].startswith("language:") and "zh" in found[0], found)

    def test_T7_an_enabled_language_whose_folder_has_no_home_page_is_a_finding(self):
        make_language_export(self.root, ("fr", "zh"))
        (self.root / sc.HTML_DIR / "zh" / "index.html").unlink()
        found = sc.check_languages(self.root, {"languages": LANGUAGES})
        self.assertEqual(len(found), 1, found)
        self.assertTrue(found[0].startswith("language:") and "zh" in found[0] and "index.html" in found[0], found)

    def assert_unwanted(self, folder: str):
        found = self.check(folders=("fr", "zh", folder))
        self.assertEqual(len(found), 1, found)
        self.assertTrue(found[0].startswith("language:") and folder in found[0], found)

    def test_T7_a_language_that_is_disabled_must_have_no_folder(self):
        self.assert_unwanted("de")

    def test_T7_a_language_with_no_enabled_key_is_not_enabled_and_must_have_no_folder(self):
        self.assert_unwanted("it")

    def test_T7_english_and_a_site_without_languages_have_no_finding(self):
        make_export(self.root)
        self.assertEqual(sc.check_languages(self.root, {"languages": {"en": LANGUAGES["en"]}}), [])
        self.assertEqual(sc.check_languages(self.root, {}), [])

    def test_T7_the_run_reports_the_language_findings_without_a_reference_build(self):
        make_language_export(self.root, ("fr", "zh", "de"))
        files = {}
        for name, text in (("inspect.json", inspect("101:101")), ("history.txt", "COPY /out/ /usr/share/nginx/html/\n")):
            files[name] = write(self.root, "work/" + name, text)
        args = argparse.Namespace(export=self.root, inspect=files["inspect.json"], history=files["history.txt"], compare_with=None)
        config = {"baseUrl": f"https://{HOST}", "languages": LANGUAGES}
        with mock.patch.object(sc.build_site, "load_config", return_value=config), mock.patch.object(sc.build_site, "private_names", return_value=()):
            found = sc.run_checks(args, self.root)
        self.assertTrue(any(f.startswith("language:") and "de" in f for f in found), found)
        self.assertEqual([f for f in found if not f.startswith("language:")], [])


if __name__ == "__main__":
    unittest.main()
