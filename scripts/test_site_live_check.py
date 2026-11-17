#!/usr/bin/env python3
"""Tests of the live check of the deployed site (story #3623, task #3678).
LiveCheck covers AC-1, AC-2, FR-01, FR-02, FR-07, SC-01: scripts/check-site.sh checks the home page and its marker,
the certificate lifetime, the http to https redirect, the absence of cookies and the Content-Security-Policy header.
LanguageLiveCheck (spec 013, T120) covers US5-AS1, FR-022, SC-006: the probes per language.
A local http.server on 127.0.0.1 plays the site and a stub openssl plays the certificate. Offline, standard library
only. Run: python scripts/test_site_live_check.py
"""
import os
import re
import shutil
import subprocess
import sys
import tempfile
import threading
import unittest
from datetime import datetime, timedelta, timezone
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from test_build_site import ROOT  # noqa: E402

BASH = shutil.which("bash")
CURL = shutil.which("curl")
GNU_DATE = False
if BASH:
    probe = subprocess.run([BASH, "-c", "date -d '+1 day' +%s"], capture_output=True, text=True)
    GNU_DATE = probe.returncode == 0
SCRIPT = ROOT / "scripts" / "check-site.sh"
HOST = "venom.example.test"
MARKER = "Pluries Venom"
PATH_PROBE = "/quickstart/"
CSP = "default-src 'none'; script-src 'self'"
# Task #3704: the page fetches its own search index and the text files are read as utf-8; one check line per file.
FILES = ("search-index.json", "llms.txt", "llms-full.txt")
TEXT_FILES = ("llms.txt", "llms-full.txt")
BASE_CHECKS = 5
PROBES = ("post", "git", "dir")
HEALTHY_OKS = BASE_CHECKS + len(FILES) + len(PROBES)
PROBE_KEYS = {"/.git/config": "git_status", "/fonts/": "dir_status"}

OPENSSL_STUB = """\
#!/usr/bin/env bash
case "$1" in
  s_client) echo "stub handshake"; exit 0 ;;
  x509) echo "notAfter=$STUB_NOT_AFTER"; exit 0 ;;
esac
echo "openssl stub: unexpected arguments: $*" >&2
exit 1
"""


def not_after(days: int) -> str:
    moment = datetime.now(timezone.utc) + timedelta(days=days)
    return f"{moment:%b} {moment.day:2d} {moment:%H:%M:%S} {moment.year} GMT"


def default_files() -> dict:
    return {
        "/search-index.json": (200, "application/json", '[{"title": "Quickstart", "url": "/quickstart/"}]'),
        "/llms.txt": (200, "text/plain; charset=utf-8", "# Pluries Venom\n"),
        "/llms-full.txt": (200, "text/plain; charset=utf-8", "# Pluries Venom\n\nFull text.\n"),
    }


def default_config() -> dict:
    return {
        "files": default_files(),
        "home_status": 200,
        "home_body": f"<html><title>{MARKER}</title></html>",
        "home_headers": {"Content-Security-Policy": CSP},
        "redirect_status": 301,
        "redirect_headers": {"Location": f"https://{HOST}{PATH_PROBE}"},
        "post_status": 403,
        "git_status": 404,
        "dir_status": 403,
    }


def probe_response(status: int) -> tuple:
    return status, ("probe body" if status == 200 else ""), {}


def file_response(config: dict, path: str) -> tuple:
    status, content_type, body = config["files"][path]
    return status, body, ({"Content-Type": content_type} if content_type else {})


def stub_response(config: dict, path: str, accept_language: str = "") -> tuple:
    if accept_language and path in config.get("accept_routes", {}):
        return config["accept_routes"][path]
    if path in config.get("routes", {}):
        return config["routes"][path]
    if path == "/":
        return config["home_status"], config["home_body"], config["home_headers"]
    if path in config.get("files", {}):
        return file_response(config, path)
    if path in PROBE_KEYS:
        return probe_response(config[PROBE_KEYS[path]])
    return config["redirect_status"], "", config["redirect_headers"]


def make_handler(config: dict):
    class Handler(BaseHTTPRequestHandler):
        def answer(self, status: int, body: str, headers: dict):
            if status == 0:
                self.close_connection = True
                return
            payload = body.encode("utf-8")
            self.send_response(status)
            for name, value in headers.items():
                self.send_header(name, value)
            self.send_header("Content-Length", str(len(payload)))
            self.end_headers()
            self.wfile.write(payload)

        def do_GET(self):
            self.answer(*stub_response(config, self.path, self.headers.get("Accept-Language", "")))

        def do_POST(self):
            self.rfile.read(int(self.headers.get("Content-Length") or 0))
            self.answer(*probe_response(config["post_status"] if self.path == "/" else 404))

        def log_message(self, *args):
            pass

    return Handler


def write_stub(directory: Path, name: str, text: str) -> None:
    path = directory / name
    path.write_text(text, encoding="utf-8", newline="\n")
    path.chmod(0o755)


class Result:
    def __init__(self, completed: subprocess.CompletedProcess):
        self.returncode = completed.returncode
        self.stdout = completed.stdout
        self.stderr = completed.stderr
        self.output = completed.stdout + completed.stderr
        self.lines = self.output.splitlines()
        self.ok_lines = [line for line in self.lines if line.startswith("ok - ")]
        self.fail_lines = [line for line in self.lines if line.startswith("FAIL - ")]


def run_check(config: dict, days: int = 60, extra_env: dict | None = None) -> Result:
    server = ThreadingHTTPServer(("127.0.0.1", 0), make_handler(config))
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        base = f"http://127.0.0.1:{server.server_address[1]}"
        with tempfile.TemporaryDirectory() as tmp:
            stubs = Path(tmp) / "stubs"
            stubs.mkdir()
            write_stub(stubs, "openssl", OPENSSL_STUB)
            env = dict(os.environ)
            env["PATH"] = stubs.as_posix() + os.pathsep + env.get("PATH", "")
            env["STUB_NOT_AFTER"] = not_after(days)
            env["CHECK_SITE_HTTPS_URL"] = base
            env["CHECK_SITE_HTTP_URL"] = base
            env["CHECK_SITE_CERT_ENDPOINT"] = "127.0.0.1:443"
            env.update(extra_env or {})
            completed = subprocess.run([BASH, SCRIPT.as_posix(), HOST], cwd=ROOT, env=env, capture_output=True,
                                       text=True, encoding="utf-8", timeout=60)
            return Result(completed)
    finally:
        server.shutdown()
        server.server_close()


@unittest.skipUnless(BASH and CURL and GNU_DATE, "bash, curl or GNU date is not available")
class LiveCheck(unittest.TestCase):
    """T1 to T4 of task #3678. Covers AC-1, AC-2, FR-01, FR-02, FR-07, SC-01."""

    def assert_single_failure(self, result: Result, pattern: str):
        self.assertEqual(result.returncode, 1, result.output)
        self.assertIn("check-site: FAIL", result.lines)
        self.assertNotIn("check-site: PASS", result.lines)
        self.assertEqual(len(result.fail_lines), 1, result.output)
        self.assertRegex(result.fail_lines[0], re.compile(pattern, re.IGNORECASE))
        self.assertEqual(len(result.ok_lines), HEALTHY_OKS - 1, result.output)

    def test_T1_a_healthy_site_passes(self):
        result = run_check(default_config())
        self.assertEqual(result.returncode, 0, result.output)
        self.assertEqual(result.fail_lines, [], result.output)
        self.assertEqual(len(result.ok_lines), HEALTHY_OKS, result.output)
        self.assertEqual(result.lines[-1], "check-site: PASS")

    def test_T2_a_302_instead_of_301_fails(self):
        config = default_config()
        config["redirect_status"] = 302
        self.assert_single_failure(run_check(config), r"301|redirect")

    def test_T2_a_location_that_drops_the_path_fails(self):
        config = default_config()
        config["redirect_headers"] = {"Location": f"https://{HOST}/"}
        self.assert_single_failure(run_check(config), r"location")

    def test_T2_a_location_to_another_host_fails(self):
        config = default_config()
        config["redirect_headers"] = {"Location": f"https://other.example.test{PATH_PROBE}"}
        self.assert_single_failure(run_check(config), r"location")

    def test_T3_a_certificate_20_days_away_fails(self):
        self.assert_single_failure(run_check(default_config(), days=20), r"certificate|expire|days")

    def test_T3_a_set_cookie_on_the_home_page_fails(self):
        config = default_config()
        config["home_headers"] = {"Content-Security-Policy": CSP, "Set-Cookie": "sid=1"}
        self.assert_single_failure(run_check(config), r"cookie")

    def test_T3_a_set_cookie_on_the_redirect_fails(self):
        config = default_config()
        config["redirect_headers"] = {"Location": f"https://{HOST}{PATH_PROBE}", "Set-Cookie": "sid=1"}
        self.assert_single_failure(run_check(config), r"cookie")

    def test_T3_a_home_page_without_the_marker_fails(self):
        config = default_config()
        config["home_body"] = "<html><title>Something else</title></html>"
        self.assert_single_failure(run_check(config), r"marker|home")

    def test_a_missing_content_security_policy_fails(self):
        config = default_config()
        config["home_headers"] = {}
        self.assert_single_failure(run_check(config), r"content-security-policy|csp")

    def test_all_checks_are_reported_after_the_first_failure(self):
        config = default_config()
        config["home_status"] = 500
        config["home_body"] = "broken"
        config["redirect_status"] = 302
        result = run_check(config, days=20)
        self.assertEqual(result.returncode, 1, result.output)
        self.assertGreaterEqual(len(result.fail_lines), 4, result.output)
        self.assertEqual(len(result.ok_lines) + len(result.fail_lines), HEALTHY_OKS, result.output)
        self.assertEqual(result.lines[-1], "check-site: FAIL")

    def lines_naming(self, result: Result, name: str) -> list[str]:
        return [line for line in result.lines if line.startswith(("ok - ", "FAIL - ")) and name in line]

    def test_3704_T2_a_stub_serving_the_three_files_passes_with_one_ok_line_per_file(self):
        result = run_check(default_config())
        self.assertEqual(result.returncode, 0, result.output)
        for name in FILES:
            with self.subTest(file=name):
                lines = self.lines_naming(result, name)
                self.assertEqual(len(lines), 1, result.output)
                self.assertTrue(lines[0].startswith("ok - "), lines[0])

    def test_3704_T2_a_missing_file_fails_and_names_it(self):
        for name in FILES:
            with self.subTest(file=name):
                config = default_config()
                del config["files"]["/" + name]
                result = run_check(config)
                self.assert_single_failure(result, re.escape(name))
                self.assertEqual(len(self.lines_naming(result, name)), 1, result.output)

    def test_3704_T2_a_wrong_status_fails_and_names_the_file(self):
        for name in FILES:
            with self.subTest(file=name):
                config = default_config()
                status, content_type, body = config["files"]["/" + name]
                config["files"]["/" + name] = (500, content_type, body)
                self.assert_single_failure(run_check(config), re.escape(name) + r".*500|500.*" + re.escape(name))

    def test_3704_T3_a_text_file_served_without_utf_8_fails(self):
        for name in TEXT_FILES:
            with self.subTest(file=name):
                config = default_config()
                config["files"]["/" + name] = (200, "text/plain", "# Pluries Venom\n")
                self.assert_single_failure(run_check(config), re.escape(name) + r".*utf-8|utf-8.*" + re.escape(name))

    def test_3704_T3_a_text_file_served_with_another_charset_fails(self):
        config = default_config()
        config["files"]["/llms.txt"] = (200, "text/plain; charset=iso-8859-1", "# Pluries Venom\n")
        self.assert_single_failure(run_check(config), r"llms\.txt")

    PROBE_CASES = (
        ("post", "post_status", "probe-post", "/"),
        ("git", "git_status", "probe-git", "/.git/config"),
        ("dir", "dir_status", "probe-dir", "/fonts/"),
    )

    def test_us3_as9_a_healthy_host_yields_one_ok_line_per_probe(self):
        """US3-AS9."""
        result = run_check(default_config())
        self.assertEqual(result.returncode, 0, result.output)
        self.assertEqual(len(result.ok_lines), HEALTHY_OKS, result.output)
        for _, _, label, _ in self.PROBE_CASES:
            with self.subTest(probe=label):
                lines = self.lines_naming(result, label)
                self.assertEqual(len(lines), 1, result.output)
                self.assertTrue(lines[0].startswith("ok - "), lines[0])

    def test_us3_as9_a_probe_answered_200_fails_and_names_the_path_and_the_status(self):
        """US3-AS9."""
        for _, key, label, path in self.PROBE_CASES:
            with self.subTest(probe=label):
                config = default_config()
                config[key] = 200
                result = run_check(config)
                self.assert_single_failure(result, re.escape(label) + r".*\b200\b")
                if path != "/":
                    self.assertIn(path, result.fail_lines[0])

    def test_us3_as9_any_non_2xx_status_passes(self):
        """US3-AS9."""
        for _, key, label, _ in self.PROBE_CASES:
            for status in (301, 403, 404, 405, 500):
                with self.subTest(probe=label, status=status):
                    config = default_config()
                    config[key] = status
                    result = run_check(config)
                    self.assertEqual(result.returncode, 0, result.output)
                    self.assertEqual(len(result.ok_lines), HEALTHY_OKS, result.output)

    def test_us3_as9_a_probe_that_cannot_complete_fails_like_the_other_checks(self):
        """US3-AS9."""
        for _, key, label, _ in self.PROBE_CASES:
            with self.subTest(probe=label):
                config = default_config()
                config[key] = 0
                self.assert_single_failure(run_check(config), re.escape(label))

    def test_T4_a_host_that_does_not_resolve_exits_two_and_claims_nothing(self):
        with tempfile.TemporaryDirectory() as tmp:
            stubs = Path(tmp) / "stubs"
            stubs.mkdir()
            write_stub(stubs, "openssl", OPENSSL_STUB)
            env = {k: v for k, v in os.environ.items() if not k.startswith("CHECK_SITE_")}
            env["PATH"] = stubs.as_posix() + os.pathsep + env.get("PATH", "")
            env["STUB_NOT_AFTER"] = not_after(60)
            completed = subprocess.run([BASH, SCRIPT.as_posix(), "venom-unresolvable.invalid"], cwd=ROOT, env=env,
                                       capture_output=True, text=True, encoding="utf-8", timeout=120)
        result = Result(completed)
        self.assertEqual(result.returncode, 2, result.output)
        self.assertIn("check-site: cannot resolve venom-unresolvable.invalid", result.lines)
        self.assertNotIn("check-site: PASS", result.lines)
        self.assertNotIn("check-site: FAIL", result.lines)
        self.assertEqual(result.ok_lines, [], result.output)


# ---------- Spec 013, task T120: the live check proves every language (US5-AS1, FR-022, SC-006) ----------
# The languages come from the served site: the home page names each of them in `<link rel="alternate" hreflang href>` and in its
# language control; the address prefix is the first folder of the address. `CHECK_SITE_LANGUAGES` (address prefixes separated by
# spaces) forces the list, so a home page that stops advertising a language fails the check instead of dropping its probes.
LANGUAGE_PATHS = {"fr": "fr", "zh": "zh-Hans"}
UNKNOWN = "/xx/"
CONTENT_SECURITY = {"Content-Security-Policy": CSP}


def alternate_links(languages: dict) -> str:
    links = [f'<link rel="alternate" hreflang="x-default" href="https://{HOST}/index.html">',
             f'<link rel="alternate" hreflang="en" href="https://{HOST}/index.html">']
    links += [f'<link rel="alternate" hreflang="{attr}" href="https://{HOST}/{prefix}/index.html">' for prefix, attr in languages.items()]
    return "".join(links)


def language_control(languages: dict) -> str:
    items = [f'<a href="{prefix}/index.html" lang="{attr}" hreflang="{attr}">{prefix}</a>' for prefix, attr in languages.items()]
    return ('<nav class="lang-menu" lang="en"><details class="lang-dropdown"><summary><span class="lang-current">English</span></summary>'
            '<a href="index.html" lang="en" hreflang="en" aria-current="true">English</a>' + "".join(items) + "</details></nav>")


def page_of(lang: str, links: str, body: str = "") -> str:
    return f'<!doctype html><html lang="{lang}"><head><title>{MARKER}</title>{links}</head><body>{body}</body></html>'


def language_config(languages: dict | None = None) -> dict:
    """A healthy site that serves English and the given languages (prefix -> `lang` attribute), 404 for `/xx/`, English to a reader
    who sends `Accept-Language: de`, and a 301 from `/fr` to `/fr/`."""
    languages = LANGUAGE_PATHS if languages is None else languages
    config = default_config()
    links = alternate_links(languages)
    config["home_body"] = page_of("en", links, language_control(languages))
    config["accept_routes"] = {"/": (200, config["home_body"], {})}
    routes = {UNKNOWN: (404, "", {})}
    for prefix, attr in languages.items():
        routes[f"/{prefix}"] = (301, "", {"Location": f"/{prefix}/"})
        routes[f"/{prefix}/"] = (200, page_of(attr, links), dict(CONTENT_SECURITY))
        for name, (status, content_type, body) in default_files().items():
            routes[f"/{prefix}{name}"] = (status, body, {"Content-Type": content_type})
    config["routes"] = routes
    return config


def named(path: str) -> str:
    """A pattern for a line that names `path` as an address, not as the start of a longer one (`/fr` is not `/fr/` or `/fr/llms.txt`)."""
    return re.escape(path) + r"(?![\w./-])"


def probes_of(prefix: str) -> list[str]:
    return [f"/{prefix}/", f"/{prefix}/search-index.json", f"/{prefix}/llms.txt", f"/{prefix}/llms-full.txt", f"/{prefix}"]


@unittest.skipUnless(BASH and CURL and GNU_DATE, "bash, curl or GNU date is not available")
class LanguageLiveCheck(unittest.TestCase):
    """T120. Each probe prints one `ok - ` or `FAIL - ` line that names the address it asked for, as the existing probes do."""

    def assert_one_failure(self, result: Result, *patterns: str):
        self.assertEqual(result.returncode, 1, result.output)
        self.assertEqual(len(result.fail_lines), 1, result.output)
        for pattern in patterns:
            self.assertRegex(result.fail_lines[0], re.compile(pattern, re.IGNORECASE))
        self.assertEqual(result.lines[-1], "check-site: FAIL")

    def oks_naming(self, result: Result, path: str) -> list[str]:
        return [line for line in result.ok_lines if re.search(named(path), line)]

    def changed(self, edit, languages: dict | None = None, **options) -> Result:
        config = language_config(languages)
        edit(config)
        return run_check(config, **options)

    def test_us5_as1_a_healthy_site_passes_and_every_probe_of_every_language_is_reported(self):
        result = run_check(language_config())
        self.assertEqual(result.returncode, 0, result.output)
        self.assertEqual(result.fail_lines, [], result.output)
        self.assertEqual(result.lines[-1], "check-site: PASS")
        for prefix in LANGUAGE_PATHS:
            for path in probes_of(prefix):
                with self.subTest(path=path):
                    self.assertEqual(len(self.oks_naming(result, path)), 1, result.output)
        self.assertEqual(len(self.oks_naming(result, UNKNOWN)), 1, result.output)
        self.assertEqual(len([line for line in result.ok_lines if "accept-language" in line.lower()]), 1, result.output)
        self.assertGreaterEqual(len(result.ok_lines), HEALTHY_OKS + 5 * len(LANGUAGE_PATHS) + 2, result.output)

    def test_us5_as1_a_language_home_page_that_does_not_answer_200_fails(self):
        for status in (404, 500):
            with self.subTest(status=status):
                result = self.changed(lambda c: c["routes"].update({"/fr/": (status, "", {})}))
                self.assert_one_failure(result, named("/fr/"), rf"\b{status}\b")

    def test_us5_as1_a_language_home_page_with_the_wrong_lang_attribute_fails(self):
        for prefix, wrong in (("fr", "en"), ("zh", "zh")):
            with self.subTest(language=prefix):
                body = page_of(wrong, alternate_links(LANGUAGE_PATHS))
                result = self.changed(lambda c: c["routes"].update({f"/{prefix}/": (200, body, dict(CONTENT_SECURITY))}))
                self.assert_one_failure(result, named(f"/{prefix}/"), r"lang")

    def test_us5_as1_a_language_home_page_without_its_alternate_links_fails(self):
        full = alternate_links(LANGUAGE_PATHS)
        cases = (("no link at all", "", "hreflang"),
                 ("no x-default", full.replace('hreflang="x-default"', 'hreflang="en"'), "x-default"),
                 ("no link to the other language", full.replace('hreflang="zh-Hans"', 'hreflang="ja"'), "zh-Hans"))
        for name, links, word in cases:
            with self.subTest(case=name):
                body = page_of("fr", links)
                result = self.changed(lambda c: c["routes"].update({"/fr/": (200, body, dict(CONTENT_SECURITY))}))
                self.assert_one_failure(result, named("/fr/"), word)

    def test_us5_as1_a_language_file_that_does_not_answer_200_fails_and_is_named(self):
        for prefix in LANGUAGE_PATHS:
            for name in FILES:
                with self.subTest(language=prefix, file=name):
                    path = f"/{prefix}/{name}"
                    result = self.changed(lambda c: c["routes"].update({path: (500, "", {"Content-Type": "text/plain; charset=utf-8"})}))
                    self.assert_one_failure(result, named(path), r"\b500\b")

    def test_us5_as1_a_language_text_file_without_utf_8_fails(self):
        for name in TEXT_FILES:
            with self.subTest(file=name):
                path = f"/zh/{name}"
                result = self.changed(lambda c: c["routes"].update({path: (200, "# Pluries Venom\n", {"Content-Type": "text/plain"})}))
                self.assert_one_failure(result, named(path), r"utf-8")

    def test_us5_as1_a_language_address_without_its_slash_must_redirect_to_the_slash(self):
        cases = (("no redirect, 200 served in place", (200, page_of("fr", alternate_links(LANGUAGE_PATHS)), {})),
                 ("404", (404, "", {})), ("302", (302, "", {"Location": "/fr/"})),
                 ("another folder", (301, "", {"Location": "/de/"})),
                 ("another host", (301, "", {"Location": "https://other.example.test/fr/"})))
        for name, answer in cases:
            with self.subTest(case=name):
                self.assert_one_failure(self.changed(lambda c: c["routes"].update({"/fr": answer})), named("/fr"))

    def test_us5_as1_a_redirect_to_the_absolute_address_of_the_same_host_is_accepted(self):
        result = self.changed(lambda c: c["routes"].update({"/fr": (301, "", {"Location": f"https://{HOST}/fr/"})}))
        self.assertEqual(result.returncode, 0, result.output)
        self.assertEqual(len(self.oks_naming(result, "/fr")), 1, result.output)

    def test_us5_as1_a_path_under_a_language_that_does_not_exist_must_answer_404(self):
        for status in (200, 403, 301):
            with self.subTest(status=status):
                result = self.changed(lambda c: c["routes"].update({UNKNOWN: (status, "", {})}))
                self.assert_one_failure(result, named(UNKNOWN), rf"\b{status}\b")

    def german_reader(self, answer: tuple) -> Result:
        return self.changed(lambda c: c["accept_routes"].update({"/": answer}))

    def test_us5_as1_a_reader_who_asks_for_german_gets_the_english_page_unchanged(self):
        result = self.german_reader((200, page_of("en", alternate_links(LANGUAGE_PATHS)), {"Vary": "Accept-Encoding"}))
        self.assertEqual(result.returncode, 0, result.output)
        self.assertEqual(len([line for line in result.ok_lines if "accept-language" in line.lower()]), 1, result.output)

    def test_us5_as1_the_site_must_not_pick_a_language_from_the_accept_language_header(self):
        english = page_of("en", alternate_links(LANGUAGE_PATHS))
        cases = (("a redirect to the German folder", (302, "", {"Location": "/de/"}), r"accept-language|redirect"),
                 ("the French page", (200, page_of("fr", alternate_links(LANGUAGE_PATHS)), {}), r"accept-language|lang"),
                 ("a cookie", (200, english, {"Set-Cookie": "lang=de"}), r"cookie"),
                 ("a Vary on the language", (200, english, {"Vary": "Accept-Language"}), r"vary"),
                 ("a Vary on the language next to another", (200, english, {"Vary": "Accept-Encoding, Accept-Language"}), r"vary"),
                 ("an error", (500, "", {}), r"accept-language|\b500\b"))
        for name, answer, pattern in cases:
            with self.subTest(case=name):
                self.assert_one_failure(self.german_reader(answer), pattern)

    def test_us5_as1_a_language_not_advertised_by_the_home_page_is_not_probed(self):
        result = run_check(language_config({"fr": "fr"}))
        self.assertEqual(result.returncode, 0, result.output)
        self.assertEqual(len(self.oks_naming(result, "/fr/")), 1, result.output)
        self.assertEqual(self.oks_naming(result, "/zh/"), [], result.output)

    def test_us5_as1_a_home_page_that_advertises_no_language_has_no_language_probe(self):
        """Characterization: the healthy default site keeps its count of ok lines."""
        result = run_check(default_config())
        self.assertEqual(result.returncode, 0, result.output)
        self.assertEqual(len(result.ok_lines), HEALTHY_OKS, result.output)
        self.assertEqual(self.oks_naming(result, UNKNOWN), [], result.output)

    def test_us5_as1_a_forced_list_is_probed_and_a_language_the_home_page_does_not_advertise_fails(self):
        result = run_check(language_config(), extra_env={"CHECK_SITE_LANGUAGES": "fr zh"})
        self.assertEqual(result.returncode, 0, result.output)
        self.assertEqual(len(self.oks_naming(result, "/zh/llms.txt")), 1, result.output)
        for forced, config in (("fr zh de", language_config()), ("fr", default_config())):
            result = run_check(config, extra_env={"CHECK_SITE_LANGUAGES": forced})
            with self.subTest(forced=forced):
                self.assertEqual(result.returncode, 1, result.output)
                missing = forced.split()[-1]
                self.assertTrue(any(re.search(rf"\b{missing}\b", line) for line in result.fail_lines), result.output)

    def test_us5_as1_the_existing_checks_still_fail_on_a_site_with_languages(self):
        """Characterization: the Content-Security-Policy and the redirect checks of the English site stay."""
        self.assert_one_failure(self.changed(lambda c: c.update({"home_headers": {}})), r"content-security-policy|csp")
        self.assert_one_failure(self.changed(lambda c: c.update({"redirect_status": 302})), r"301|redirect")

    def test_sc_006_a_set_cookie_on_any_language_response_fails(self):
        for path in ("/fr/", "/zh/llms.txt", "/fr/search-index.json", "/fr"):
            with self.subTest(path=path):
                def edit(config):
                    status, body, headers = config["routes"][path]
                    config["routes"][path] = (status, body, {**headers, "Set-Cookie": "lang=fr"})
                self.assert_one_failure(self.changed(edit), r"cookie")


if __name__ == "__main__":
    unittest.main()
