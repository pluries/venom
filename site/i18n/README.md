# Translating the documentation site

| | |
|---|---|
| Classification | Restricted |
| Status | Current |
| Owner | Christian Stephan |
| Work items | Spec 013-site-languages; Feature #3618 |
| Controlled copy | n/a |
| Last updated | 2026-11-16 |

The site at venom.pluries.dev is built from English sources. Its translations (French, German, Italian, Spanish, Simplified Chinese) are files in this folder, one per page, drafted by an AI and reviewed by a person. The rules for a drafter are in `docs/specs/013-site-languages/translation-brief.md`. This note is not a page of the site: it lives here so that a change to it does not make any translation stale.

- Change an English page, and its translations go stale: `python scripts/test_site_i18n_content.py` fails and names every language and page to update.
- `python scripts/site_i18n.py stamp <code> <slug>` records the fingerprint of a new or updated draft and marks it `machine-drafted`.
- `python scripts/site_i18n.py review <code> <slug> --by "<name>"` marks a page reviewed; the name must be in `reviewers.txt`. It also writes a record to `reviews.json` (language, slug, fingerprint, reviewer, date). A page whose header says `reviewed` without a matching record is not reviewed: the check reports it and the enable gate refuses it. No page shows a draft banner (D-028), so reviewing is what lets a language be enabled, not what the reader sees. Never edit `reviews.json` by hand.
- `python scripts/site_i18n.py status` (or `make site-i18n-status`) counts the pages per language.
- Code, commands, YAML, URLs and numbers stay identical to English; the glossary is `glossary.json`.
- A language appears on the site only when it is enabled in `site/site.json`; `make site-preview` builds the others for review.
