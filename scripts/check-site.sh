#!/usr/bin/env bash
# Checks the deployed site from outside (story #3623): home page and marker, certificate lifetime, http to https redirect, no cookie,
# Content-Security-Policy, the search and llms files, and three paths the server must refuse (POST /, /.git/config, /fonts/).
# Spec 013 (US5-AS1, FR-022, SC-006): for each language the home page advertises with a `<link rel="alternate" hreflang href>` (the
# prefix is the first folder of the address), the language home page (200, `<html lang>`, the same alternates, the policy), its search
# and llms files, `/<prefix>` redirecting to `/<prefix>/`; then `/xx/` answering 404 and `Accept-Language: de` getting the English home
# page with no redirect and no Vary on the language. CHECK_SITE_LANGUAGES (prefixes separated by spaces) replaces the advertised list, so
# a home page that stops advertising a language fails instead of dropping its probes. No language advertised: no language probe.
# Usage: check-site.sh [host]. The default host is the public site name. Prints `check-site: PASS|FAIL`.
set -euo pipefail

HOST="${1:-venom.pluries.dev}"
HTTPS_URL="${CHECK_SITE_HTTPS_URL:-https://$HOST}"
HTTP_URL="${CHECK_SITE_HTTP_URL:-http://$HOST}"
CERT_ENDPOINT="${CHECK_SITE_CERT_ENDPOINT:-$HOST:443}"
MARKER="${CHECK_SITE_MARKER:-Pluries Venom}"
MIN_DAYS="${CHECK_SITE_MIN_DAYS:-30}"
PATH_PROBE="${CHECK_SITE_PATH:-/quickstart/}"

for tool in curl openssl; do
  command -v "$tool" >/dev/null 2>&1 || { echo "check-site: no $tool on this machine" >&2; exit 2; }
done

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

FAILURES=0
ok() { echo "ok - $*"; }
bad() { echo "FAIL - $*"; FAILURES=$((FAILURES + 1)); }

fetch() {
  local label="$1" url="$2" rc=0
  shift 2
  STATUS="$(curl -sS -D "$TMP/$label.h" -o "$TMP/$label.b" -w '%{http_code}' "$@" "$url" 2>"$TMP/$label.e")" || rc=$?
  : >>"$TMP/$label.h"
  return "$rc"
}

probe_refused() {
  local label="$1" url="$2" rc=0
  shift 2
  fetch "$label" "$url" "$@" || rc=$?
  case "$rc:$STATUS" in
    0:2??) bad "$label: $url answered $STATUS, expected it refused" ;;
    0:*) ok "$label: $url refused with $STATUS" ;;
    *) bad "$label: request to $url failed (curl exit $rc)" ;;
  esac
}

header() {
  tr -d '\r' <"$TMP/$1.h" | grep -i "^$2:" | tail -n 1 | sed 's/^[^:]*:[[:space:]]*//; s/[[:space:]]*$//' || true
}

lang_fetch() {
  local label="$1" url="$2" rc=0
  shift 2
  fetch "$label" "$url" "$@" || rc=$?
  if [ "$rc" -ne 0 ]; then
    bad "$label: request to $url failed (curl exit $rc)"
    return 1
  fi
}

# The `hreflang href` pairs of the alternate links of the home page, which the language probes are built from.
home_alternates() {
  [ -f "$TMP/https.b" ] || return 0
  tr -d '\r' <"$TMP/https.b" | grep -oE '<link rel="alternate" hreflang="[^"]*" href="[^"]*"' |
    sed -E 's/.*hreflang="([^"]*)" href="([^"]*)"/\1 \2/' || true
}

# `prefix:hreflang` for each alternate that is neither English nor x-default and lives in a folder of its own.
advertised_languages() {
  local attr href path
  while read -r attr href; do
    case "$attr" in '' | en | x-default) continue ;; esac
    path="$(printf '%s' "$href" | sed -E 's#^[A-Za-z]+://[^/]*/##')"
    case "$path" in */*) printf '%s:%s\n' "${path%%/*}" "$attr" ;; esac
  done <<<"$1"
}

missing_alternate() {
  local value
  for value in $ALT_VALUES; do
    grep -qF "hreflang=\"$value\"" "$TMP/$1.b" || { echo "$value"; return 0; }
  done
}

probe_language_home() {
  local prefix="$1" attr="$2" label="lang-$1-home" url="$HTTPS_URL/$1/" missing
  lang_fetch "$label" "$url" || return 0
  if [ "$STATUS" != "200" ]; then
    bad "language $prefix: $url answered $STATUS, expected 200"
  elif ! grep -Eqi "<html[^>]* lang=\"$attr\"" "$TMP/$label.b"; then
    bad "language $prefix: $url does not set <html lang=\"$attr\">"
  elif missing="$(missing_alternate "$label")" && [ -n "$missing" ]; then
    bad "language $prefix: $url has no hreflang alternate link for $missing"
  elif ! header "$label" content-security-policy | grep -qF "default-src 'none'"; then
    bad "language $prefix: $url is served without Content-Security-Policy default-src 'none'"
  else
    ok "language $prefix: $url answers 200 with lang=\"$attr\", the hreflang alternates and the policy"
  fi
}

probe_language_file() {
  local prefix="$1" file="$2" label="lang-$1-file-$2" url="$HTTPS_URL/$1/$2"
  lang_fetch "$label" "$url" || return 0
  if [ "$STATUS" != "200" ]; then
    bad "language $prefix: $url answered $STATUS, expected 200"
  elif [ "$file" != "search-index.json" ] && ! header "$label" content-type | grep -qi 'charset=utf-8'; then
    bad "language $prefix: $url Content-Type without charset=utf-8"
  else
    ok "language $prefix: $url answers 200"
  fi
}

probe_language_slash() {
  local prefix="$1" label="lang-$1-slash" url="$HTTPS_URL/$1" location
  lang_fetch "$label" "$url" || return 0
  if [ "$STATUS" != "301" ]; then
    bad "language $prefix: $url answered $STATUS, expected a 301 to the address with a trailing slash"
    return 0
  fi
  location="$(header "$label" location)"
  case "$location" in
    "/$prefix/" | "$HTTPS_URL/$prefix/" | "https://$HOST/$prefix/" | "http://$HOST/$prefix/")
      ok "language $prefix: $url redirects with 301 to the address with a trailing slash" ;;
    *) bad "language $prefix: $url redirects to '$location', expected the address with a trailing slash" ;;
  esac
}

probe_unknown_language() {
  local url="$HTTPS_URL/xx/"
  lang_fetch lang-unknown "$url" || return 0
  if [ "$STATUS" = "404" ]; then
    ok "language: $url answers 404, no folder for a language that does not exist"
  else
    bad "language: $url answered $STATUS, expected 404 for a language that does not exist"
  fi
}

# No choice of language from the reader's headers: the cookie is judged with the others, on every response kept as lang-*.h.
probe_accept_language() {
  local url="$HTTPS_URL/"
  lang_fetch lang-accept "$url" -H 'Accept-Language: de' || return 0
  if [ "$STATUS" != "200" ]; then
    bad "an Accept-Language: de request to $url answered $STATUS, expected the English page with 200 and no redirect"
  elif ! grep -Eqi '<html[^>]* lang="en"' "$TMP/lang-accept.b"; then
    bad "an Accept-Language: de request to $url did not get the English page (<html lang=\"en\">)"
  elif header lang-accept vary | grep -qi 'accept-language'; then
    bad "an Accept-Language: de request to $url got a Vary on the language, the page must be the same for every reader"
  else
    ok "an Accept-Language: de request gets the English home page, no redirect and no Vary on the language"
  fi
}

# Sets PROBE_PAIRS to the `prefix:hreflang` pairs to probe: the advertised ones, or those of CHECK_SITE_LANGUAGES, where a prefix that
# the home page does not advertise fails. Not a command substitution: `bad` must count in this shell.
choose_languages() {
  local advertised="$1" prefix pair
  PROBE_PAIRS=""
  if [ -z "${CHECK_SITE_LANGUAGES:-}" ]; then
    PROBE_PAIRS="$advertised"
    return 0
  fi
  for prefix in $CHECK_SITE_LANGUAGES; do
    pair="$(printf '%s
' "$advertised" | grep "^$prefix:" | head -n 1 || true)"
    if [ -n "$pair" ]; then
      PROBE_PAIRS="$PROBE_PAIRS$pair"$'
'
    else
      bad "language $prefix: the home page advertises no hreflang alternate for it"
    fi
  done
}

probe_languages() {
  local alternates advertised pair prefix file
  alternates="$(home_alternates)"
  ALT_VALUES="$(printf '%s
' "$alternates" | awk 'NF { print $1 }')"
  advertised="$(advertised_languages "$alternates")"
  choose_languages "$advertised"
  [ -n "${PROBE_PAIRS//[$'
']/}" ] || return 0
  while read -r pair; do
    [ -n "$pair" ] || continue
    prefix="${pair%%:*}"
    probe_language_home "$prefix" "${pair#*:}"
    for file in search-index.json llms.txt llms-full.txt; do
      probe_language_file "$prefix" "$file"
    done
    probe_language_slash "$prefix"
  done <<<"$PROBE_PAIRS"
  probe_unknown_language
  probe_accept_language
}

rc=0
fetch https "$HTTPS_URL/" || rc=$?
HOME_STATUS="$STATUS"
if [ "$rc" -eq 6 ]; then
  echo "check-site: cannot resolve $HOST" >&2
  exit 2
fi
if [ "$rc" -ne 0 ]; then
  bad "home page: request to $HTTPS_URL/ failed (curl exit $rc)"
elif [ "$HOME_STATUS" != "200" ]; then
  bad "home page: $HTTPS_URL/ answered $HOME_STATUS, expected 200"
elif ! grep -qF -- "$MARKER" "$TMP/https.b"; then
  bad "home page: marker '$MARKER' not found in the body"
else
  ok "home page answers 200 and carries the marker '$MARKER'"
fi

probe_refused probe-post "$HTTPS_URL/" --data ''
probe_refused probe-git "$HTTPS_URL/.git/config"
probe_refused probe-dir "$HTTPS_URL/fonts/"

not_after="$(openssl s_client -connect "$CERT_ENDPOINT" -servername "$HOST" </dev/null 2>/dev/null | openssl x509 -noout -enddate 2>/dev/null || true)"
if [ "${not_after#notAfter=}" = "$not_after" ]; then
  bad "certificate: no expiry date read from $CERT_ENDPOINT"
elif ! end_epoch="$(date -d "${not_after#notAfter=}" +%s 2>/dev/null)"; then
  bad "certificate: expiry date '${not_after#notAfter=}' not understood"
else
  days=$(((end_epoch - $(date +%s)) / 86400))
  if [ "$days" -gt "$MIN_DAYS" ]; then
    ok "certificate expires in $days days (more than $MIN_DAYS)"
  else
    bad "certificate expires in $days days, expected more than $MIN_DAYS"
  fi
fi

rc=0
fetch http "$HTTP_URL$PATH_PROBE" || rc=$?
REDIRECT_STATUS="$STATUS"
if [ "$rc" -ne 0 ]; then
  bad "redirect: request to $HTTP_URL$PATH_PROBE failed (curl exit $rc)"
elif [ "$REDIRECT_STATUS" != "301" ]; then
  bad "redirect: $HTTP_URL$PATH_PROBE answered $REDIRECT_STATUS, expected 301"
else
  location="$(header http location)"
  if [ "$location" = "https://$HOST$PATH_PROBE" ]; then
    ok "http redirects with 301 to https://$HOST$PATH_PROBE"
  else
    bad "redirect: Location is '$location', expected https://$HOST$PATH_PROBE"
  fi
fi

probe_languages

if grep -qis '^set-cookie:' "$TMP/https.h" "$TMP/http.h" "$TMP"/lang-*.h; then
  bad "cookie: a Set-Cookie header was sent"
else
  ok "no Set-Cookie header on any response"
fi

if [ "$HOME_STATUS" != "200" ]; then
  bad "Content-Security-Policy: no successful https response to read it from"
elif header https content-security-policy | grep -qF "default-src 'none'"; then
  ok "Content-Security-Policy carries default-src 'none'"
else
  bad "Content-Security-Policy header missing or without default-src 'none'"
fi

for file in search-index.json llms.txt llms-full.txt; do
  rc=0
  fetch "file-$file" "$HTTPS_URL/$file" || rc=$?
  if [ "$rc" -ne 0 ]; then
    bad "$file: request to $HTTPS_URL/$file failed (curl exit $rc)"
  elif [ "$STATUS" != "200" ]; then
    bad "$file: $HTTPS_URL/$file answered $STATUS, expected 200"
  elif [ "$file" != "search-index.json" ] && ! header "file-$file" content-type | grep -qi 'charset=utf-8'; then
    bad "$file: Content-Type without charset=utf-8"
  else
    ok "$file answers 200"
  fi
done

if [ "$FAILURES" -eq 0 ]; then
  echo "check-site: PASS"
else
  echo "check-site: FAIL"
  exit 1
fi
