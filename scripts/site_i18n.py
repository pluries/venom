#!/usr/bin/env python3
"""Translations of the docs site (spec 013-site-languages, tasks T012 and later).

A translation is a file `site/i18n/<lang>/<slug>.md` (data-model.md): a header block of `key: value` lines between two `---`
lines (an HTML comment holds the same lines in a `home.html`), then the page. `Translations.page` turns one into a `Page`.
Standard library only.

    python3 scripts/site_i18n.py stamp <lang> <slug>
    python3 scripts/site_i18n.py review <lang> <slug> --by <name>
    python3 scripts/site_i18n.py status
"""
from __future__ import annotations

import argparse
import datetime
import functools
import hashlib
import html
import json
import re
import shutil
import sys
import unicodedata
from dataclasses import dataclass, field, replace
from html.parser import HTMLParser
from pathlib import Path
from typing import TYPE_CHECKING, Mapping

if TYPE_CHECKING:
    from build_site import Page

LANGUAGE_FOLDERS = ("fr", "de", "it", "es", "zh")
STATUSES = ("machine-drafted", "reviewed")
REVIEW_KEYS = ("reviewed_by", "reviewed_on", "reviewed_fingerprint")
REQUIRED_KEYS = ("translates", "title", "description")
UI_MAPS = ("groups", "group_names", "subgroups", "theme_names")
SCRIPT_KEYS = ("copy", "copied", "copy_failed", "copy_aria", "search_aria", "search_label", "search_placeholder", "search_empty",
               "search_none", "search_loading", "search_unavailable", "result_one", "result_other", "theme_aria", "theme_title", "theme_names")
_HEADER_LINE = re.compile(r"^([A-Za-z_][A-Za-z0-9_]*):[ \t]*(.*)$")
_DATE = re.compile(r"^\d{4}-\d{2}-\d{2}$")
# The words of the two findings that a dated override excuses (FR-025); `_is_staleness` tells them from every other finding.
_STALE = "stale: the translation records"
_REVIEW_STALE = "is not the English fingerprint of today"


class I18nError(Exception):
    """A translation file or the language configuration is not usable."""


@dataclass
class Translation:
    """A parsed translation file: its header keys and the page text that follows the header."""
    header: dict[str, str] = field(default_factory=dict)
    body: str = ""


def enabled_languages(config: dict) -> list[str]:
    """The codes of the translated languages the build writes: those of `config["languages"]` with `enabled` true, in file
    order, without `en` (the source, always built). No `languages` key: an empty list."""
    languages = config.get("languages") or {}
    return [code for code, entry in languages.items() if code != "en" and entry.get("enabled") is True]


def _header_block(lines: list[str]) -> tuple[list[str], list[str], str]:
    """The header lines, the body lines and the form of the header (`---` or an HTML comment)."""
    first = lines[0].strip() if lines else ""
    closing = {"---": "---", "<!--": "-->"}.get(first)
    if closing is None:
        raise I18nError("the file does not start with a header block (`---` or `<!--`)")
    for index in range(1, len(lines)):
        if lines[index].strip() == closing:
            return lines[1:index], lines[index + 1:], first
    raise I18nError(f"the header block is never closed (expected a line `{closing}`)")


def _header_values(lines: list[str]) -> dict[str, str]:
    header: dict[str, str] = {}
    for line in lines:
        if not line.strip():
            continue
        match = _HEADER_LINE.match(line.rstrip())
        if match is None:
            raise I18nError(f"header line is not `key: value`: {line[:60]!r}")
        header[match.group(1)] = match.group(2).strip()
    return header


def parse_translation(text: str) -> Translation:
    """Split a translation file into its header and its body. Raises I18nError when there is no header, when it is not
    closed, when a line of it is not `key: value`, when `translates`, `title` or `description` is missing, or when `status`
    is present and is neither `machine-drafted` nor `reviewed`."""
    lines = text.lstrip("﻿").replace("\r\n", "\n").replace("\r", "\n").split("\n")
    head, body, _ = _header_block(lines)
    header = _header_values(head)
    missing = [key for key in REQUIRED_KEYS if not header.get(key)]
    if missing:
        raise I18nError(f"the header misses {', '.join(missing)}")
    if "status" in header and header["status"] not in STATUSES:
        raise I18nError(f"status '{header['status']}' must be one of {', '.join(STATUSES)}")
    return Translation(header, "\n".join(body))


def _normal(text: str) -> str:
    return "\n".join(line.rstrip() for line in text.replace("\r\n", "\n").replace("\r", "\n").split("\n"))


def fingerprint(text: str, title: str, description: str, home_template: str | None = None) -> str:
    """The SHA-256 (64 hex characters) of an English page: its text with includes expanded, line endings as LF and trailing
    spaces removed, then its title and its description; for the home page the text of `site/home.html` as well."""
    parts = [_normal(text), title, description]
    if home_template is not None:
        parts.append(_normal(home_template))
    return hashlib.sha256("\x00".join(parts).encode("utf-8")).hexdigest()


def _serialise(translation: Translation, form: str) -> str:
    lines = "".join(f"{key}: {value}\n" for key, value in translation.header.items())
    opening, closing = ("---", "---") if form == "---" else ("<!--", "-->")
    return f"{opening}\n{lines}{closing}\n{translation.body}"


def _update_header(path: Path, values: dict[str, str], drop: tuple[str, ...] = ()) -> None:
    text = path.read_text(encoding="utf-8")
    form = text.lstrip("﻿").lstrip().split("\n", 1)[0].strip()
    parsed = parse_translation(text)
    parsed.header.update(values)
    for key in drop:
        parsed.header.pop(key, None)
    with open(path, "w", encoding="utf-8", newline="\n") as handle:
        handle.write(_serialise(parsed, form))


def _page_files(folder: Path, slug: str) -> list[Path]:
    """The files of one page: its Markdown and, for the home page, its template."""
    names = [f"{slug}.md"] + (["home.html"] if slug == "index" else [])
    return [folder / name for name in names if (folder / name).is_file()]


def _read(root: Path, lang: str, name: str) -> Translation:
    shown = f"site/i18n/{lang}/{name}"
    path = root / "site" / "i18n" / lang / name
    if not path.is_file():
        raise I18nError(f"{shown} does not exist")
    try:
        return parse_translation(path.read_text(encoding="utf-8"))
    except (I18nError, UnicodeDecodeError) as error:
        raise I18nError(f"{shown}: {error}") from error


def _reviewers(root: Path) -> list[str]:
    path = root / "site" / "i18n" / "reviewers.txt"
    if not path.is_file():
        return []
    return [line.strip() for line in path.read_text(encoding="utf-8-sig").splitlines() if line.strip()]


def _glossary(root: Path) -> dict:
    path = root / "site" / "i18n" / "glossary.json"
    return _read_ui(path) if path.is_file() else {}


def page_fingerprint(site, page: "Page") -> str:
    """The fingerprint of an English page of `site`, a `build_site.SiteBuild` of the English site."""
    home = site.read(page.template) if page.template else None
    return fingerprint(site.page_text(page), page.title, page.description, home)


class _Site:
    """The English site of a repository, read once for all the pages a command looks at."""

    def __init__(self, root: Path) -> None:
        from build_site import BuildError, SiteBuild, load_config  # build_site imports this module
        try:
            self.config = load_config(root)
            self.build = SiteBuild(root, self.config, "")
        except BuildError as error:
            raise I18nError(str(error)) from error
        self.root = root
        self._fingerprints: dict[str, str] = {}

    def page(self, slug: str) -> "Page":
        page = self.build.pages_by_slug.get(slug)
        if page is None:
            raise I18nError(f"the site has no page '{slug}'")
        return page

    def fingerprint(self, slug: str) -> str:
        if slug not in self._fingerprints:
            self._fingerprints[slug] = page_fingerprint(self.build, self.page(slug))
        return self._fingerprints[slug]

    def template(self, page: "Page") -> str | None:
        return self.build.read(page.template) if page.template else None

    def slugs(self) -> list[str]:
        return [page.slug for page in self.build.pages]

    def languages(self) -> list[str]:
        return [code for code in (self.config.get("languages") or {}) if code != "en"]

    def complete(self) -> bool:
        return (self.config.get("i18n") or {}).get("complete") is True

    def exists(self, lang: str, slug: str) -> bool:
        return (self.root / "site" / "i18n" / lang / f"{slug}.md").is_file()

    def missing(self, lang: str) -> list[str]:
        return [slug for slug in self.slugs() if not self.exists(lang, slug)]


def stamp(root: Path, lang: str, slug: str, today: str) -> str:
    """Write the fingerprint of the English page `slug`, `drafted_by: claude` and `drafted_on: <today>` (an ISO date) into
    `site/i18n/<lang>/<slug>.md` (and `home.html` for `index`), set `status: machine-drafted` and remove `reviewed_by`,
    `reviewed_on` and `reviewed_fingerprint`; every other key and the body stay as they are. Returns the fingerprint written.
    Raises I18nError for a slug the site does not have or a translation file that does not exist."""
    if not _DATE.match(today):
        raise I18nError(f"'{today}' is not an ISO date")
    folder = root / "site" / "i18n" / lang
    if not (folder / f"{slug}.md").is_file():
        raise I18nError(f"site/i18n/{lang}/{slug}.md does not exist")
    value = _Site(root).fingerprint(slug)
    values = {"fingerprint": value, "drafted_by": "claude", "drafted_on": today, "status": "machine-drafted"}
    for path in _page_files(folder, slug):
        _update_header(path, values, REVIEW_KEYS)
    return value


def check_fingerprint(label: str, recorded: str | None, current: str) -> list[str]:
    """US2-AS1: no finding when `recorded` (the `fingerprint` key of the file, None when it has none) equals `current` (the English
    fingerprint today); otherwise one finding that starts with `label` (`<lang>/<slug>`) and names both values."""
    if recorded == current:
        return []
    return [f"{label}: {_STALE} the English fingerprint {recorded or 'none'}, the English page is now {current}; "
            "re-draft it from the English page and run stamp"]


# The patterns below are those of the renderer in build_site.py: a heading, a fence or a list item it sees is one here.
_FENCE = re.compile(r"^(\s*)(```+|~~~+)\s*([\w+-]*)")
_HEADING = re.compile(r"^(#{1,6})\s+(.*?)(?:\s+#+)?\s*$")
_EXPLICIT_ID = re.compile(r"\s+\{#([\w-]+)\}$")
_LIST = re.compile(r"^(\s*)([-*+]|\d+[.)])\s+(.*)")
_TABLE_SEP = re.compile(r"^\s*\|?\s*:?-{2,}:?\s*(\|\s*:?-{2,}:?\s*)*\|?\s*$")
_RULE = re.compile(r"^\s*([-*_])(\s*\1){2,}\s*$")
_INLINE_CODE = re.compile(r"(`+)(.+?)\1", re.S)
_LINK = re.compile(r"(!?)\[((?:[^\[\]]|\[[^\]]*\])*)\]\(\s*<?([^)\s>]+)>?(?:\s+\"[^\"]*\")?\s*\)")
_RAW_IMAGE_SOURCE = re.compile(r"<img\b[^>]*?\bsrc\s*=\s*[\"']([^\"']+)[\"']", re.I)
_ANCHOR_ID = re.compile(r"\s*\{#[\w-]+\}")
_ADDRESS = re.compile(r"https?://[^\s)\]>\"'<`]+")


@dataclass
class _Scan:
    """A page split into its fenced code blocks (language tag, a newline, the code) and the lines outside them."""
    blocks: list[str] = field(default_factory=list)
    prose: list[str] = field(default_factory=list)


def _take_fence(lines: list[str], index: int, opener: re.Match, scan: _Scan) -> int:
    marker, indent = opener.group(2), len(opener.group(1))
    closing = re.compile(rf"^\s*{re.escape(marker[0])}{{{len(marker)},}}\s*$")
    code: list[str] = []
    index += 1
    while index < len(lines) and not closing.match(lines[index]):
        line = lines[index]
        code.append((line[indent:] if line[:indent].strip() == "" else line).rstrip())
        index += 1
    scan.blocks.append(opener.group(3) + "\n" + "\n".join(code))
    return index + 1


def _scan(text: str) -> _Scan:
    scan = _Scan()
    lines = text.replace("\r\n", "\n").replace("\r", "\n").split("\n")
    index = 0
    while index < len(lines):
        opener = _FENCE.match(lines[index])
        if opener is None:
            scan.prose.append(lines[index])
            index += 1
        else:
            index = _take_fence(lines, index, opener, scan)
    return scan


def _short(item: str) -> str:
    return item if len(item) <= 80 else item[:77] + "..."


def _list_finding(label: str, name: str, english: list[str], translated: list[str]) -> list[str]:
    if english == translated:
        return []
    gone = [_short(item) for item in english if item not in translated]
    new = [_short(item) for item in translated if item not in english]
    detail = f"only in the English page: {gone}; only in the translation: {new}" if gone or new else "are the same items in another order or number"
    return [f"{label}: {name} {detail}"]


def _nth(items: list[str], number: int) -> str | None:
    return items[number - 1] if number <= len(items) else None


def _block_findings(label: str, english: list[str], translated: list[str]) -> list[str]:
    found = []
    for number in range(1, max(len(english), len(translated)) + 1):
        mine, theirs = _nth(english, number), _nth(translated, number)
        if mine == theirs:
            continue
        if theirs is None:
            found.append(f"{label}: code block {number} of the English page is missing")
        elif mine is None:
            found.append(f"{label}: code block {number} is not in the English page")
        else:
            found.append(f"{label}: code block {number} differs from the English one")
    return found


def _without_code(text: str) -> str:
    return _INLINE_CODE.sub(" ", text)


def _codes(text: str) -> list[str]:
    return [found.group(2).strip() for found in _INLINE_CODE.finditer(text)]


def _targets(text: str, image: bool) -> list[str]:
    plain = _without_code(text)
    found = [match.group(3) for match in _LINK.finditer(plain) if bool(match.group(1)) == image]
    return found + _RAW_IMAGE_SOURCE.findall(plain) if image else found


def _addresses(text: str) -> set[str]:
    return {found.rstrip(".,;:") for found in _ADDRESS.findall(text)}


def _new_addresses(label: str, english: str, translated: str, where: str = "") -> list[str]:
    return [f"{label}: {where}URL not in the English page: {url}" for url in sorted(_addresses(translated) - _addresses(english))]


def _inline_findings(label: str, english: str, translated: str) -> list[str]:
    found = _list_finding(label, "inline code", _codes(english), _codes(translated))
    found += _list_finding(label, "image", _targets(english, True), _targets(translated, True))
    found += _list_finding(label, "link", _targets(english, False), _targets(translated, False))
    return found + _new_addresses(label, english, translated)


def _headings(prose: list[str]) -> list[tuple[str, str | None]]:
    """The text and the explicit `{#id}` (None without one) of each heading."""
    found = []
    for line in prose:
        match = _HEADING.match(line)
        if match:
            marked = _EXPLICIT_ID.search(match.group(2))
            found.append((match.group(2)[:marked.start()], marked.group(1)) if marked else (match.group(2), None))
    return found


def _heading_findings(label: str, english: list[str], translated: list[str], english_anchors: list[str]) -> list[str]:
    mine, theirs = _headings(english), _headings(translated)
    found = []
    if len(mine) != len(theirs):
        found.append(f"{label}: heading count: English {len(mine)}, translation {len(theirs)}")
    missing = [text for text, anchor in theirs if anchor is None]
    found += [f"{label}: anchor: the heading '{text}' has no {{#english-anchor}} at its end" for text in missing]
    anchors = [anchor for _, anchor in theirs if anchor]
    if not missing and anchors != english_anchors:
        found.append(f"{label}: anchor list differs: English {english_anchors}, translation {anchors}")
    return found


def check_identity(label: str, english: str, translated: str, english_anchors: list[str]) -> list[str]:
    """US2-AS2, AS3, AS8, FR-009: what must be the English page's, in content, number and order, in the Markdown of a translation.
    Findings start with `label` and name the thing: `code block <n>` (fenced blocks, language tag included, numbered from 1),
    `inline code`, `image`, `link`, `URL` (any `http(s)://` address in the text, so one English lacks is named), `anchor` (the
    explicit `{#id}` of each heading, in order, must be `english_anchors`; a heading without one is named) and `heading` (their
    number). An image or link is compared by its target; its text and alt are the translator's."""
    mine, theirs = _scan(english), _scan(translated)
    found = _block_findings(label, mine.blocks, theirs.blocks)
    found += _inline_findings(label, "\n".join(mine.prose), "\n".join(theirs.prose))
    return found + _heading_findings(label, mine.prose, theirs.prose, english_anchors)


def _figures(text: str) -> set[str]:
    return set(re.findall(r"[0-9]+", unicodedata.normalize("NFKC", _ANCHOR_ID.sub("", text))))


def _number_findings(label: str, english: str, translated: str, where: str = "") -> list[str]:
    mine, theirs = _figures(english), _figures(translated)
    if mine == theirs:
        return []
    gone, new = sorted(mine - theirs, key=int), sorted(theirs - mine, key=int)
    return [f"{label}: {where}number: missing from the translation {gone}, not in the English page {new}"]


def _tables(prose: list[str]) -> int:
    return sum(1 for index in range(len(prose) - 1)
               if "|" in prose[index] and "|" in prose[index + 1] and _TABLE_SEP.match(prose[index + 1]))


def _quotes(prose: list[str]) -> int:
    quoted = [line.lstrip().startswith(">") for line in prose]
    return sum(1 for index, flag in enumerate(quoted) if flag and (index == 0 or not quoted[index - 1]))


def _structure(text: str) -> dict[str, int]:
    prose = _scan(text).prose
    return {"table": _tables(prose), "list item": sum(1 for line in prose if _LIST.match(line) and not _RULE.match(line)),
            "admonition": _quotes(prose)}


def check_numbers(label: str, english: str, translated: str) -> list[str]:
    """US2-AS12, FR-009: the numbers of the page as a set of digit strings (`1.5` is `1` and `5`) and the counts of `table`s,
    `list item`s and `admonition`s (a block quote) must equal the English ones; code blocks included. Findings start with `label`
    and name `number` (and the digit strings) or the structure counted (and both counts)."""
    mine, theirs = _structure(english), _structure(translated)
    found = _number_findings(label, english, translated)
    return found + [f"{label}: {name}: English {mine[name]}, translation {theirs[name]}" for name in mine if mine[name] != theirs[name]]


def check_glossary(label: str, lang: str, english: str, translated: str, glossary: dict) -> list[str]:
    """US2-AS6, FR-013: `glossary` is `site/i18n/glossary.json`. Each term of `verbatim` that `english` contains (a case-insensitive
    substring) must be in `translated`, and no rendering of `forbidden[lang]` (a term to a list of words) may be in it, whatever the
    English has. Findings start with `label` and name the term or the forbidden word."""
    wanted, text = english.lower(), translated.lower()
    found = [f"{label}: glossary: the term '{term}' stays as it is, and the translation does not have it"
             for term in glossary.get("verbatim", []) if term.lower() in wanted and term.lower() not in text]
    for term, words in (glossary.get("forbidden", {}).get(lang) or {}).items():
        found += [f"{label}: glossary: '{word}' is not an accepted rendering of '{term}'" for word in words if word.lower() in text]
    return found


def _review_problems(header: dict[str, str], current: str, reviewers: list[str]) -> list[str]:
    found = []
    by, on, recorded = header.get("reviewed_by"), header.get("reviewed_on"), header.get("reviewed_fingerprint")
    if not by:
        found.append("reviewed_by is missing")
    elif by not in reviewers:
        found.append(f"reviewed_by '{by}' is not on site/i18n/reviewers.txt")
    if not on:
        found.append("reviewed_on is missing")
    elif not _DATE.match(on):
        found.append(f"reviewed_on '{on}' is not an ISO date")
    if not recorded:
        found.append("reviewed_fingerprint is missing")
    elif recorded != current:
        found.append(f"reviewed_fingerprint {recorded} {_REVIEW_STALE} {current}")
    return found


def check_review(label: str, header: dict[str, str], current: str, reviewers: list[str]) -> list[str]:
    """US2-AS5, AS9, FR-012: `header` is the parsed header of a file. `reviewed` needs `reviewed_by` on `reviewers`, `reviewed_on` as
    an ISO date and `reviewed_fingerprint` equal to `current`; `machine-drafted` (or no status) must carry none of the three keys.
    Findings start with `label` and name the key at fault (and the reviewer, or both fingerprints)."""
    if header.get("status") == "reviewed":
        problems = _review_problems(header, current, reviewers)
    else:
        problems = [f"{key} must not be on a machine-drafted file" for key in REVIEW_KEYS if key in header]
    return [f"{label}: {problem}" for problem in problems]


REVIEWS_FILE = "site/i18n/reviews.json"


def _reviews(root: Path) -> tuple[list[dict], list[str]]:
    """The records of `site/i18n/reviews.json` (a missing file holds none) and the findings on the file: a file that is not one
    JSON list of objects gives no usable record."""
    path = root / "site" / "i18n" / "reviews.json"
    if not path.is_file():
        return [], []
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, ValueError) as error:
        return [], [f"{REVIEWS_FILE}: {error}"]
    if not isinstance(value, list) or not all(isinstance(entry, dict) for entry in value):
        return [], [f"{REVIEWS_FILE}: the file must hold one JSON list of objects"]
    return value, []


def translation_fingerprint(root: Path, lang: str, slug: str) -> str:
    """D-024 amended: the SHA-256 of the body of the page's translation files (the Markdown, then `home.html` for `index`), header
    excluded. An empty string when a file cannot be read: it matches no record."""
    try:
        bodies = [_normal(parse_translation(path.read_text(encoding="utf-8")).body)
                  for path in _page_files(root / "site" / "i18n" / lang, slug)]
    except (I18nError, UnicodeDecodeError, OSError):
        return ""
    return hashlib.sha256("\x00".join(bodies).encode("utf-8")).hexdigest() if bodies else ""


def _recorded(records: list[dict], lang: str, slug: str, header: dict[str, str], own: str) -> bool:
    by, recorded = header.get("reviewed_by"), header.get("reviewed_fingerprint")
    return bool(by and recorded and own) and any(
        entry.get("language") == lang and entry.get("slug") == slug and entry.get("reviewer") == by
        and entry.get("fingerprint") == recorded and entry.get("translation_fingerprint") == own for entry in records)


def check_review_record(label: str, header: dict[str, str], records: list[dict], own: str) -> list[str]:
    """D-024: a `reviewed` file needs a record of `reviews.json` for its language and slug (from `label`) with its `reviewed_by`,
    `reviewed_fingerprint` and `own`, the fingerprint of the translation's text today; the file alone is no review. A file that is
    not `reviewed` needs none."""
    lang, _, slug = label.partition("/")
    if header.get("status") != "reviewed" or _recorded(records, lang, slug, header, own):
        return []
    return [f"{label}: reviewed but no record in reviews.json"]


class _TemplateFacts(HTMLParser):
    """What `check_template` compares: the text of each `<pre>` and `<code>`, every `href` and `src`, the visible text, the scripts."""

    def __init__(self) -> None:
        super().__init__(convert_charrefs=True)
        self.pre: list[str] = []
        self.code: list[str] = []
        self.refs: list[str] = []
        self.text: list[str] = []
        self.scripts = 0
        self._open: list[tuple[str, list[str]]] = []
        self._hidden = 0

    def handle_starttag(self, tag: str, attrs: list) -> None:
        self.refs += [value for name, value in attrs if name in ("href", "src") and value]
        self.scripts += tag == "script"
        self._hidden += tag in ("script", "style")
        if tag in ("pre", "code"):
            self._open.append((tag, []))

    def handle_endtag(self, tag: str) -> None:
        if tag in ("script", "style") and self._hidden:
            self._hidden -= 1
        if self._open and self._open[-1][0] == tag:
            name, parts = self._open.pop()
            getattr(self, name).append("".join(parts))

    def handle_data(self, data: str) -> None:
        if self._hidden:
            return
        self.text.append(data)
        for _, parts in self._open:
            parts.append(data)


def _facts(markup: str) -> _TemplateFacts:
    facts = _TemplateFacts()
    facts.feed(markup)
    facts.close()
    return facts


def check_template(label: str, english_html: str, translated_html: str) -> list[str]:
    """US3-AS1 for the home page, FR-009, FR-018: `home.html` of a language against `site/home.html`. The text of each `<pre>`, the
    text of each `<code>`, every `href` and `src`, and the set of digit strings of the visible text equal the English ones; a
    `<script>` or an address in the text that English lacks is a finding. Findings start with `label` and name `home.html`."""
    mine, theirs = _facts(english_html), _facts(translated_html)
    found = _list_finding(label, "home.html <pre> text", mine.pre, theirs.pre)
    found += _list_finding(label, "home.html <code> text", mine.code, theirs.code)
    found += _list_finding(label, "home.html href and src", mine.refs, theirs.refs)
    english_text, translated_text = " ".join(mine.text), " ".join(theirs.text)
    found += _number_findings(label, english_text, translated_text, "home.html ")
    found += _new_addresses(label, english_text, translated_text, "home.html ")
    if theirs.scripts > mine.scripts:
        found.append(f"{label}: home.html has a <script> that the English page lacks")
    return found


def translation_files(root: Path) -> list[tuple[str, str]]:
    """Every `(language, slug)` that has a `site/i18n/<language>/<slug>.md` for a language folder that exists, whether the language is
    enabled or not (D-014), sorted. `ui.json` and `home.html` are not pages (the home page is `index.md`)."""
    base = root / "site" / "i18n"
    folders = [folder for folder in base.iterdir() if folder.is_dir() and folder.name != "en"] if base.is_dir() else []
    return sorted((folder.name, path.stem) for folder in folders for path in folder.glob("*.md"))


def _english_anchors(text: str) -> list[str]:
    from build_site import render_markdown  # build_site imports this module
    return [anchor for _, _, anchor in render_markdown(text).headings]


class _Checker:
    """The checks of the translation files of one repository against the English pages of today."""

    def __init__(self, root: Path) -> None:
        self.root, self.site = root, _Site(root)
        self.glossary, self.reviewers = _glossary(root), _reviewers(root)
        self.records, self.review_findings = _reviews(root)

    def _header(self, label: str, slug: str, header: dict[str, str], current: str) -> list[str]:
        found = check_fingerprint(label, header.get("fingerprint"), current)
        if header["translates"] != slug:
            found.append(f"{label}: translates says '{header['translates']}', the file is the page '{slug}'")
        found += check_review(label, header, current, self.reviewers)
        lang, _, _ = label.partition("/")
        return found + check_review_record(label, header, self.records, translation_fingerprint(self.root, lang, slug))

    def _home(self, lang: str, label: str, template: str, current: str) -> tuple[list[str], str]:
        """The findings on `home.html` of a language and its body."""
        try:
            home = _read(self.root, lang, "home.html")
        except I18nError as error:
            return [f"{label}: {error}"], ""
        found = check_fingerprint(label, home.header.get("fingerprint"), current)
        found += check_review(label, home.header, current, self.reviewers)
        found += check_review_record(label, home.header, self.records, translation_fingerprint(self.root, lang, "index"))
        found = [f"{item} (home.html)" for item in found]
        return found + check_template(label, template, home.body), home.body

    def page(self, lang: str, slug: str) -> list[str]:
        label = f"{lang}/{slug}"
        page = self.site.page(slug)
        parsed = _read(self.root, lang, f"{slug}.md")
        from build_site import expand_includes  # build_site imports this module
        english, current = self.site.build.page_text(page), self.site.fingerprint(slug)
        body = expand_includes(parsed.body, self.root, page.source)
        found = self._header(label, slug, parsed.header, current)
        found += check_identity(label, english, body, _english_anchors(english))
        found += check_numbers(label, english, body)
        template, translated = self.site.template(page), body
        if template is not None:
            home, body = self._home(lang, label, template, current)
            found, english, translated = found + home, english + "\n" + template, translated + "\n" + body
        return found + check_glossary(label, lang, english, translated, self.glossary)


def check_page(root: Path, lang: str, slug: str) -> list[str]:
    """Every check of one translation file against the English page of today: fingerprint, identity (English anchors from the
    rendered English page), numbers and counts, glossary, review rules (`site/i18n/reviewers.txt`) and, for `index`, `home.html`
    against `site/home.html`. The glossary of the home page reads `index.md` and `home.html` together (the English words of the
    page are in both). Findings all start with `<lang>/<slug>`. Raises I18nError for a file or slug that is not there."""
    return _Checker(root).page(lang, slug)


def missing_pages(root: Path, lang: str) -> list[str]:
    """The slugs of the English pages that have no translation file in `lang`, in page order."""
    return _Site(root).missing(lang)


CORE_PAGES = ("index", "quickstart", "overview", "installing", "updating", "docker", "first-test")
OVERRIDE_MAX_DAYS = 14


def check_enable_gate(root: Path, lang: str, today: str | None = None) -> list[str]:
    """FR-024, D-008, US5-AS4: the findings that stop `lang` from being built when the maker has enabled it. One finding per page of
    `CORE_PAGES` that the site has and that `lang` has no file for (`<lang>/<slug>: core page missing`) or has but not `reviewed`
    (`<lang>/<slug>: core page not reviewed`); a `reviewed` file that `check_review` rejects (a stale `reviewed_fingerprint`, a
    reviewer who is not on the list) is not reviewed. Pages outside `CORE_PAGES` are never a finding here. A page under a valid
    override (`today`, the system date when None) is not reviewed only if its review is wrong in another way than a stale
    fingerprint: the override excuses that staleness here as it does in `check_site`."""
    site, reviewers, records = _Site(root), _reviewers(root), _reviews(root)[0]
    excused = {slug for code, slug in active_overrides(root, today) if code == lang}
    found = [_core_finding(site, lang, slug, (reviewers, records), slug in excused) for slug in CORE_PAGES
             if slug in site.build.pages_by_slug]
    return [finding for finding in found if finding]


def _core_finding(site: _Site, lang: str, slug: str, review: tuple[list[str], list[dict]], excused: bool) -> str | None:
    label = f"{lang}/{slug}"
    if not site.exists(lang, slug):
        return f"{label}: core page missing"
    try:
        header = _read(site.root, lang, f"{slug}.md").header
    except I18nError as error:
        return f"{label}: core page not reviewed: {error}"
    if header.get("status") != "reviewed":
        return f"{label}: core page not reviewed: the status is machine-drafted"
    reviewers, records = review
    problems = [problem for problem in check_review(label, header, site.fingerprint(slug), reviewers)
                if not (excused and _is_staleness(problem))]
    problems += check_review_record(label, header, records, translation_fingerprint(site.root, lang, slug))
    return f"{label}: core page not reviewed: {problems[0]}" if problems else None


def _is_staleness(finding: str) -> bool:
    return _STALE in finding or _REVIEW_STALE in finding


def _iso_date(value: object) -> datetime.date | None:
    if not isinstance(value, str) or not _DATE.match(value):
        return None
    try:
        return datetime.date.fromisoformat(value)
    except ValueError:
        return None


def _day(today: str | None) -> datetime.date:
    if today is None:
        return datetime.date.today()
    day = _iso_date(today)
    if day is None:
        raise I18nError(f"'{today}' is not an ISO date")
    return day


def _read_overrides(root: Path) -> list:
    path = root / "site" / "i18n" / "overrides.json"
    if not path.is_file():
        return []
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, ValueError) as error:
        raise I18nError(f"site/i18n/overrides.json: {error}") from error
    if not isinstance(value, list):
        raise I18nError("site/i18n/overrides.json: the file must hold a list of overrides")
    return value


def _span_problems(entry: dict) -> list[str]:
    created, expires = _iso_date(entry.get("created")), _iso_date(entry.get("expires"))
    problems = [f"{key} is not an ISO date" for key, day in (("created", created), ("expires", expires)) if day is None]
    if created is None or expires is None:
        return problems
    days = (expires - created).days
    if days < 0:
        problems.append("expires is before created")
    elif days > OVERRIDE_MAX_DAYS:
        problems.append(f"expires is {days} days after created, more than the {OVERRIDE_MAX_DAYS} allowed")
    return problems


def _entry_problems(entry: dict, today: datetime.date) -> list[str]:
    reason = entry.get("reason")
    problems = [] if isinstance(reason, str) and reason.strip() else ["reason is missing or blank"]
    problems += _span_problems(entry)
    expires = _iso_date(entry.get("expires"))
    if expires is not None and expires < today:
        problems.append(f"the override expired on {expires}")
    return problems


def _site_problems(entry: dict, site: _Site) -> list[str]:
    problems = []
    if entry.get("language") not in site.languages():
        problems.append(f"language '{entry.get('language')}' is not a language of site/site.json")
    if entry.get("slug") not in site.slugs():
        problems.append(f"the site has no page '{entry.get('slug')}'")
    return problems


def active_overrides(root: Path, today: str | None = None) -> set[tuple[str, str]]:
    """The `(language, slug)` of each override of `site/i18n/overrides.json` that is valid and not expired on `today` (the system
    date when None). An unreadable file excuses nothing: `check_overrides` reports it."""
    day = _day(today)
    try:
        entries = _read_overrides(root)
    except I18nError:
        return set()
    return {(entry["language"], entry["slug"]) for entry in entries if isinstance(entry, dict) and not _entry_problems(entry, day)
            and isinstance(entry.get("language"), str) and isinstance(entry.get("slug"), str)}


def check_overrides(root: Path, today: str) -> list[str]:
    """FR-025, D-008, US5-AS5: the findings on `site/i18n/overrides.json` (a list of `{language, slug, reason, created, expires}`; an
    absent file is no finding). Each finding starts with `<language>/<slug>` and names the key at fault: a blank or missing `reason`,
    a `created` or `expires` that is not an ISO date, an `expires` before `created` or more than `OVERRIDE_MAX_DAYS` days after it, a
    language or a page the site does not have. An entry whose `expires` is before `today` is reported as expired."""
    day = _day(today)
    try:
        entries = _read_overrides(root)
    except I18nError as error:
        return [str(error)]
    if not entries:
        return []
    site = _Site(root)
    return [finding for number, entry in enumerate(entries, 1) for finding in _entry_findings(entry, number, site, day)]


def _entry_findings(entry: object, number: int, site: _Site, day: datetime.date) -> list[str]:
    if not isinstance(entry, dict):
        return [f"site/i18n/overrides.json: entry {number} is not an object"]
    label = f"{entry.get('language', '?')}/{entry.get('slug', '?')}"
    return [f"{label}: override: {problem}" for problem in _entry_problems(entry, day) + _site_problems(entry, site)]


def check_site(root: Path, today: str | None = None) -> list[str]:
    """The findings of `check_page` for every file of `translation_files`, and, when `i18n.complete` of `site/site.json` is true, one
    finding `<lang>/<slug>: missing` for each missing page of each language declared in `site/site.json`. Missing pages are no
    finding otherwise (US2-AS7). `today` (an ISO date, the system date when None) is the day the overrides of
    `site/i18n/overrides.json` are judged on (FR-025); the tests pass a fixed one."""
    checker = _Checker(root)
    excused = active_overrides(root, today)
    found: list[str] = []
    for lang, slug in translation_files(root):
        found += _page_findings(checker, lang, slug, (lang, slug) in excused)
    if checker.site.complete():
        found += [f"{lang}/{slug}: missing" for lang in checker.site.languages() for slug in checker.site.missing(lang)]
    return found + checker.review_findings + check_overrides(root, today)


def _page_findings(checker: _Checker, lang: str, slug: str, excused: bool) -> list[str]:
    try:
        found = checker.page(lang, slug)
    except I18nError as error:
        return [f"{lang}/{slug}: {error}"]
    return [finding for finding in found if not _is_staleness(finding)] if excused else found


def _page_state(site: _Site, lang: str, slug: str, records: list[dict]) -> str:
    if not site.exists(lang, slug):
        return "missing"
    try:
        header = _read(site.root, lang, f"{slug}.md").header
    except I18nError:
        return "stale"
    if _is_stale(header, site.fingerprint(slug)):
        return "stale"
    if header.get("status") == "reviewed" and _recorded(records, lang, slug, header, translation_fingerprint(site.root, lang, slug)):
        return "reviewed"
    return "machine-drafted"


def _is_stale(header: dict[str, str], current: str) -> bool:
    """Whether the English fingerprint a file records (the reviewed one for a `reviewed` file) is not `current`."""
    return header.get("reviewed_fingerprint" if header.get("status") == "reviewed" else "fingerprint") != current


def status(root: Path) -> dict[str, dict[str, int]]:
    """FR-019: for each language declared in `site/site.json` (not `en`), `{"machine-drafted": n, "reviewed": n, "stale": n,
    "missing": n}`. A page is counted once: `missing` without a file, `stale` when its `fingerprint` (or, for a `reviewed` file,
    its `reviewed_fingerprint`) is not the English one of today, else its status."""
    site = _Site(root)
    counts = {lang: dict.fromkeys((*STATUSES, "stale", "missing"), 0) for lang in site.languages()}
    records = _reviews(root)[0]
    for lang, row in counts.items():
        for slug in site.slugs():
            row[_page_state(site, lang, slug, records)] += 1
    return counts


def render_status(counts: dict[str, dict[str, int]]) -> str:
    """One line per language: its code, then each of the four words followed by its number, e.g.
    `fr  machine-drafted 3  reviewed 2  stale 1  missing 37`."""
    words = (*STATUSES, "stale", "missing")
    return "".join(f"{lang:<4}" + "  ".join(f"{word} {row[word]}" for word in words) + "\n" for lang, row in counts.items())


def review(root: Path, lang: str, slug: str, by: str, today: str) -> str:
    """The only writer of the reviewed keys (FR-012): set `status: reviewed`, `reviewed_by: <by>`, `reviewed_on: <today>` and
    `reviewed_fingerprint` = the English fingerprint of today in `site/i18n/<lang>/<slug>.md` (and `home.html` for `index`), nothing
    else changed; returns that fingerprint. Raises I18nError, writing nothing, for a reviewer who is not on `site/i18n/reviewers.txt`,
    a file or slug that is not there, or a file whose own `fingerprint` is not today's (the maker would be vouching for text the
    translation was not drafted from: re-draft and `stamp` first)."""
    if not _DATE.match(today):
        raise I18nError(f"'{today}' is not an ISO date")
    if by not in _reviewers(root):
        raise I18nError(f"'{by}' is not on site/i18n/reviewers.txt")
    current = _Site(root).fingerprint(slug)
    _read(root, lang, f"{slug}.md")
    paths = _page_files(root / "site" / "i18n" / lang, slug)
    stale = [path.name for path in paths if parse_translation(path.read_text(encoding="utf-8")).header.get("fingerprint") != current]
    if stale:
        raise I18nError(f"site/i18n/{lang}/{', '.join(stale)} was not drafted from the English page of today: re-draft it and run stamp first")
    records, findings = _reviews(root)
    if findings:
        raise I18nError(findings[0])
    values = {"status": "reviewed", "reviewed_by": by, "reviewed_on": today, "reviewed_fingerprint": current}
    for path in paths:
        _update_header(path, values)
    _write_reviews(root, records, {"language": lang, "slug": slug, "fingerprint": current,
                                   "translation_fingerprint": translation_fingerprint(root, lang, slug), "reviewer": by, "date": today})
    return current


def _write_reviews(root: Path, records: list[dict], new: dict) -> None:
    kept = [entry for entry in records if (entry.get("language"), entry.get("slug")) != (new["language"], new["slug"])]
    kept = sorted([*kept, new], key=lambda entry: (str(entry.get("language")), str(entry.get("slug"))))
    with open(root / "site" / "i18n" / "reviews.json", "w", encoding="utf-8", newline="\n") as handle:
        handle.write(json.dumps(kept, indent=2, ensure_ascii=False) + "\n")


class Translations:
    """The translation files of a repository, read on demand: `english` maps each slug to the English `Page`."""

    def __init__(self, root: Path, english: Mapping[str, "Page"]) -> None:
        self.root, self.english = root, english

    def _file(self, lang: str, name: str) -> Path:
        return self.root / "site" / "i18n" / lang / name

    def exists(self, lang: str, slug: str) -> bool:
        return self._file(lang, f"{slug}.md").is_file()

    def page(self, lang: str, slug: str) -> "Page | None":
        """A copy of the English page `slug` carrying the translated title, description and text (and nothing else changed:
        source, group, subgroup, heading_index stay English's), or None when the language has no file for the slug.
        Raises I18nError when the file exists and cannot be parsed."""
        english = self.english.get(slug)
        path = self._file(lang, f"{slug}.md")
        if english is None or not path.is_file():
            return None
        shown = f"site/i18n/{lang}/{slug}.md"
        try:
            parsed = parse_translation(path.read_text(encoding="utf-8"))
        except (I18nError, UnicodeDecodeError) as error:
            raise I18nError(f"{shown}: {error}") from error
        if parsed.header["translates"] != slug:
            raise I18nError(f"{shown}: `translates` says '{parsed.header['translates']}', the file is the page '{slug}'")
        return replace(english, title=parsed.header["title"], description=parsed.header["description"], text=parsed.body, rendered=None)

    def is_stale(self, lang: str, slug: str, current: str) -> bool:
        """Whether the file's recorded English fingerprint is not `current`; a file that cannot be read is stale."""
        try:
            return _is_stale(_read(self.root, lang, f"{slug}.md").header, current)
        except (I18nError, OSError):
            return True

    def template(self, lang: str, rel: str) -> str | None:
        """The page template of a language (`site/i18n/<lang>/home.html` for `site/home.html`) without its header, or None."""
        path = self._file(lang, Path(rel).name)
        if not path.is_file():
            return None
        try:
            return parse_translation(path.read_text(encoding="utf-8")).body
        except (I18nError, UnicodeDecodeError) as error:
            raise I18nError(f"site/i18n/{lang}/{path.name}: {error}") from error


def ui_path(root: Path, lang: str) -> Path:
    return root / "site" / "i18n" / lang / "ui.json"


def _read_ui(path: Path) -> dict:
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, ValueError) as error:
        raise I18nError(f"cannot read {path.as_posix()}: {error}") from error
    if not isinstance(value, dict):
        raise I18nError(f"{path.as_posix()}: the file must hold one JSON object")
    return value


@functools.lru_cache(maxsize=None)
def default_ui() -> dict:
    """The English interface text of this checkout: what a render function uses when it is not given one."""
    return _read_ui(ui_path(Path(__file__).resolve().parent.parent, "en"))


def load_ui(root: Path, lang: str, reference: dict | None = None) -> dict:
    """The interface text of a language; with `reference` (the English one), a key it misses is an error."""
    ui = _read_ui(ui_path(root, lang))
    if reference is not None:
        missing = [key for key in reference if key not in ui]
        if missing:
            raise I18nError(f"site/i18n/{lang}/ui.json misses the key(s): {', '.join(missing)}")
        wrong = [key for key in UI_MAPS if key in reference and not isinstance(ui[key], dict)]
        if wrong:
            raise I18nError(f"site/i18n/{lang}/ui.json: {', '.join(wrong)} must be an object")
    return ui


def script_ui(ui: dict) -> str:
    """The strings `site.js` shows, as the JSON of the `data-ui` attribute of `<body>`; no extra request, no script of its own."""
    return json.dumps({key: ui[key] for key in SCRIPT_KEYS if key in ui}, ensure_ascii=False, separators=(",", ":"))


SAFE_NAME = re.compile(r"[a-z]{2,3}(-[A-Za-z0-9]{2,8})?")
RESERVED_NAMES = frozenset({"images", "img", "fonts"})


def _language_problem(code: str, entry: object) -> str | None:
    if not isinstance(entry, dict):
        return f"language '{code}' must be an object"
    for key in ("name", "path", "lang"):
        if key in entry and not isinstance(entry[key], str):
            return f"language '{code}': '{key}' must be a string"
    if "enabled" in entry and not isinstance(entry["enabled"], bool):
        return f"language '{code}': 'enabled' must be true or false"
    path = entry.get("path", code)
    if code != "en" and (not SAFE_NAME.fullmatch(code) or not SAFE_NAME.fullmatch(path) or path in RESERVED_NAMES or code in RESERVED_NAMES):
        return f"language '{code}': the code and 'path' ('{path}') must each be one short folder name such as 'fr' or 'zh-Hans', not a name the build uses"
    return None


def language_prefixes(config: dict) -> set[str]:
    """Every folder name a language may take: the five known ones and the paths the configuration declares."""
    declared = {entry.get("path", code) for code, entry in (config.get("languages") or {}).items() if code != "en"}
    return set(LANGUAGE_FOLDERS) | declared


def config_problem(config: dict, slugs: list[str]) -> str | None:
    """Why the `languages` and `i18n` keys of site.json cannot be used (or a page slug takes a language folder), or None."""
    languages = config.get("languages", {})
    if not isinstance(languages, dict):
        return "'languages' must map a language code to an object"
    for code, entry in languages.items():
        problem = _language_problem(code, entry)
        if problem:
            return problem
    options = config.get("i18n", {})
    if not isinstance(options, dict) or not isinstance(options.get("complete", False), bool):
        return "'i18n' must be an object whose 'complete' is true or false"
    clash = sorted(set(slugs) & language_prefixes(config))
    if clash:
        return f"page slug '{clash[0]}' is also the prefix of a language folder; a page cannot take a language's name"
    return None


def language_path(config: dict, code: str) -> str:
    if code == "en":
        return ""
    return (config.get("languages") or {}).get(code, {}).get("path", code)


def language_attribute(config: dict, code: str) -> str:
    return (config.get("languages") or {}).get(code, {}).get("lang", code)


def languages_to_build(config: dict, preview: tuple[str, ...] = ()) -> list[str]:
    """The enabled languages, plus the disabled ones named for a preview (D-017), in file order. An unknown one is an error."""
    declared = [code for code in (config.get("languages") or {}) if code != "en"]
    unknown = [code for code in preview if code not in declared]
    if unknown:
        raise I18nError(f"--preview-languages: {', '.join(unknown)} is not a language of site/site.json")
    wanted = set(enabled_languages(config)) | set(preview)
    return [code for code in declared if code in wanted]


def clear_language_folders(out_dir: Path, config: dict) -> None:
    """A language that is disabled, or a page that is removed, must leave nothing in the output."""
    for name in language_prefixes(config):
        if not SAFE_NAME.fullmatch(name):
            continue
        shutil.rmtree(out_dir / name, ignore_errors=True)


def address(base: str, path: str, slug: str) -> str:
    return f"{base.rstrip('/')}/" + (f"{path}/" if path else "") + f"{slug}.html"


@dataclass(frozen=True)
class Language:
    code: str
    name: str
    path: str
    lang: str


@dataclass
class LanguageSet:
    """The languages one build writes (English first) and, for each, the slugs that have a translation file."""
    languages: list[Language] = field(default_factory=list)
    translated: dict[str, set[str]] = field(default_factory=dict)
    base: str = ""

    def prefix(self, code: str) -> str:
        path = next((entry.path for entry in self.languages if entry.code == code), "")
        return "../" * len([part for part in path.split("/") if part])

    def _is_translated(self, code: str, slug: str) -> bool:
        return code == "en" or slug in self.translated.get(code, set())

    def alternates(self, code: str, slug: str, fallback: bool) -> str:
        """The canonical link and the `hreflang` alternates of a page; nothing while only English is built or without a base
        address. A fallback page is the English page: its canonical is the English address and it lists no alternate."""
        if len(self.languages) < 2 or not self.base:
            return ""
        english = next(entry for entry in self.languages if entry.code == "en")
        own = english if fallback else next(entry for entry in self.languages if entry.code == code)
        links = [f'<link rel="canonical" href="{html.escape(address(self.base, own.path, slug))}">']
        if not fallback:
            links += [f'<link rel="alternate" hreflang="{html.escape(entry.lang)}" href="{html.escape(address(self.base, entry.path, slug))}">'
                      for entry in self.languages if self._is_translated(entry.code, slug)]
            links.append(f'<link rel="alternate" hreflang="x-default" href="{html.escape(address(self.base, english.path, slug))}">')
        return "".join(links)

    def _item(self, current: Language, entry: Language, slug: str) -> str:
        if entry.code == current.code:
            href = f"{slug}.html"
        else:
            href = self.prefix(current.code) + (f"{entry.path}/" if entry.path else "") + f"{slug}.html"
        mark = ' aria-current="true"' if entry.code == current.code else ""
        return f'<a href="{html.escape(href)}" lang="{html.escape(entry.lang)}" hreflang="{html.escape(entry.lang)}"{mark}>{html.escape(entry.name)}</a>'

    def menu(self, code: str, slug: str, label: str) -> str:
        """The language control: a dropdown of plain links to the same page in every language built, the current one marked."""
        if len(self.languages) < 2:
            return ""
        current = next(entry for entry in self.languages if entry.code == code)
        items = "".join(self._item(current, entry, slug) for entry in self.languages)
        return (f'<nav class="lang-menu" aria-label="{html.escape(label)}" lang="{html.escape(current.lang)}"><details class="lang-dropdown">'
                f'<summary><span class="lang-current">{html.escape(current.name)}</span></summary><div class="lang-panel">{items}</div></details></nav>')


def language_set(config: dict, base: str, codes: list[str], translations: Translations) -> LanguageSet:
    """English plus `codes`, with the slugs each of them has translated."""
    entries = [Language(code, (config.get("languages") or {}).get(code, {}).get("name", code), language_path(config, code),
                        language_attribute(config, code)) for code in ["en", *codes]]
    translated = {code: {slug for slug in translations.english if translations.exists(code, slug)} for code in codes}
    return LanguageSet(entries, translated, base)


def main(argv: list[str] | None = None, root: Path | None = None) -> int:
    parser = argparse.ArgumentParser(description="Commands on the translations of the docs site.")
    commands = parser.add_subparsers(dest="command", required=True)
    stamped = commands.add_parser("stamp", help="write the fingerprint and the date of a draft")
    stamped.add_argument("language")
    stamped.add_argument("slug")
    reviewed = commands.add_parser("review", help="record that the maker reviewed a translation")
    reviewed.add_argument("language")
    reviewed.add_argument("slug")
    reviewed.add_argument("--by", required=True)
    commands.add_parser("status", help="count the pages machine-drafted, reviewed, stale and missing per language")
    args = parser.parse_args(argv)
    root = root or Path(__file__).resolve().parent.parent
    today = datetime.date.today().isoformat()
    try:
        if args.command == "status":
            sys.stdout.write(render_status(status(root)))
            return 0
        if args.command == "review":
            value = review(root, args.language, args.slug, args.by, today)
            sys.stdout.write(f"site_i18n: {args.language}/{args.slug} reviewed by {args.by}, English {value}\n")
            return 0
        value = stamp(root, args.language, args.slug, today)
    except (I18nError, OSError, UnicodeDecodeError) as error:
        sys.stderr.write(f"site_i18n: {error}\n")
        return 2
    sys.stdout.write(f"site_i18n: {args.language}/{args.slug} stamped {value}\n")
    return 0


if __name__ == "__main__":
    sys.exit(main())
