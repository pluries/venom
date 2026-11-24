#!/usr/bin/env bash
# Check a downloaded Venom Pluries release before anything of it is installed.
#
#   verify-release.sh --dir <dir> --asset <file name> [--release-env <file>]
#
# <dir> holds the asset and the three files of the release: SHA256SUMS, SHA256SUMS.p7s and SHA256SUMS.p7s.tsr.
# The checks run in this order, and the first one that fails stops the script with its own exit code:
#   15 a file is missing          14 a tool cannot be run        11 the CMS signature of SHA256SUMS
#   12 the signer is not the one  13 the timestamp               10 the checksum of the asset
# of the key published in release.env (SIGNER_SHA256).
# What a pass proves: SHA256SUMS was signed by the key whose SHA-256 is in release.env, a timestamp token covers the
# signature, and the asset matches SHA256SUMS. The chain to a certificate authority is NOT judged (-noverify), and the
# certificate may be expired: the output never says "trusted" or "valid".
# VENOM_CA_FILE names a CA bundle for the timestamp check; it can weaken only that check, never the signer comparison.
set -u

fail() { # fail <exit code> <check> <reason>
  printf 'FAIL %s: %s\n' "$2" "$3" >&2
  exit "$1"
}

usage() { printf 'usage: verify-release.sh --dir <dir> --asset <file name> [--release-env <file>]\n' >&2; exit 2; }

dir=""; asset=""; envfile=""
while [ $# -gt 0 ]; do
  case "$1" in
    --dir|--asset|--release-env) [ $# -ge 2 ] || usage ;;
  esac
  case "$1" in
    --dir) dir="${2-}"; shift 2 ;;
    --asset) asset="${2-}"; shift 2 ;;
    --release-env) envfile="${2-}"; shift 2 ;;
    *) usage ;;
  esac
done
[ -n "$dir" ] && [ -n "$asset" ] || usage

here="$(cd "$(dirname "$0")" && pwd)"
if [ -z "$envfile" ]; then
  for c in "$here/release.env" "$here/../release.env"; do [ -f "$c" ] && { envfile="$c"; break; }; done
fi
[ -f "${envfile:-}" ] || fail 15 files "release.env not found (give --release-env)"

# Read the pinned signer without sourcing the file.
expected="$(sed -n 's/^SIGNER_SHA256=//p' "$envfile" | head -n 1 | tr -d '\r' | tr 'A-F' 'a-f')"
[ -n "$expected" ] || fail 15 files "SIGNER_SHA256 is missing from $envfile"

# Read before the clean environment is built below.
ca_override="${VENOM_CA_FILE-}"

for f in SHA256SUMS SHA256SUMS.p7s SHA256SUMS.p7s.tsr "$asset"; do
  [ -f "$dir/$f" ] || fail 15 files "$f is missing from $dir"
done

# A real OpenSSL 3 or later is needed: the LibreSSL that ships with macOS cannot verify the timestamp token.
openssl_major() { "$1" version 2>/dev/null | tr -d '\r' | sed -n 's/^OpenSSL \([0-9][0-9]*\)\..*/\1/p' | head -n 1; }
ossl_bin=""
for c in openssl /opt/homebrew/bin/openssl /opt/homebrew/opt/openssl@3/bin/openssl /usr/local/opt/openssl@3/bin/openssl; do
  b="$(command -v "$c" 2>/dev/null)" || continue
  m="$(openssl_major "$b")"
  if [ -n "$m" ] && [ "$m" -ge 3 ]; then ossl_bin="$b"; break; fi
done
if [ -z "$ossl_bin" ]; then
  if command -v openssl >/dev/null 2>&1; then
    fail 14 tools "OpenSSL 3 or later is required, found: $(openssl version 2>/dev/null | head -n 1). The LibreSSL that ships with macOS cannot verify the timestamp: install OpenSSL 3 (for example with Homebrew)"
  fi
  fail 14 tools "openssl was not found: install OpenSSL 3 (on Windows, Git for Windows ships it)"
fi
command -v env >/dev/null 2>&1 || fail 14 tools "env was not found"
sha_tool=""
if command -v sha256sum >/dev/null 2>&1; then sha_tool="sha256sum"
elif command -v shasum >/dev/null 2>&1; then sha_tool="shasum -a 256"
else fail 14 tools "no sha256 tool was found: install sha256sum (coreutils) or shasum"; fi

native() { if command -v cygpath >/dev/null 2>&1; then cygpath -m "$1"; else printf '%s' "$1"; fi; }

# openssl sees nothing of the caller's environment but what it needs to run.
ossl() {
  env -i PATH="$PATH" HOME="${HOME:-/}" ${SYSTEMROOT:+SYSTEMROOT="$SYSTEMROOT"} ${TEMP:+TEMP="$TEMP"} ${TMP:+TMP="$TMP"} "$ossl_bin" "$@"
}

work="$(mktemp -d)" || fail 14 tools "cannot make a temporary folder"
trap 'rm -rf "$work"' EXIT

sums="$(native "$dir/SHA256SUMS")"; p7s="$(native "$dir/SHA256SUMS.p7s")"; tsr="$(native "$dir/SHA256SUMS.p7s.tsr")"
signer="$(native "$work/signer.pem")"

ossl cms -verify -inform DER -in "$p7s" -content "$sums" -binary -noverify -signer "$signer" -out /dev/null </dev/null >/dev/null 2>&1 \
  || fail 11 signature "SHA256SUMS does not verify against SHA256SUMS.p7s"

found="$(ossl x509 -in "$signer" -noout -fingerprint -sha256 </dev/null 2>/dev/null | sed -e 's/^.*=//' -e 's/://g' | tr 'A-F' 'a-f')"
[ -n "$found" ] || fail 12 signer "the signer certificate could not be read"
[ "$found" = "$expected" ] || fail 12 signer "SHA256SUMS is signed by $found, not by the published key $expected"

# The timestamp is checked against each CA bundle found, and passes when one of them verifies the token: the bundle of
# the system may lack the timestamp authority's root (macOS's own does), while another (Homebrew's) has it.
bundles=""
if [ -n "$ca_override" ]; then
  bundles="$ca_override"
else
  for c in /etc/ssl/certs/ca-certificates.crt /etc/pki/tls/certs/ca-bundle.crt /etc/ssl/cert.pem /opt/homebrew/etc/ca-certificates/cert.pem /opt/homebrew/etc/openssl@3/cert.pem /usr/local/etc/openssl@3/cert.pem /mingw64/etc/ssl/certs/ca-bundle.crt /usr/ssl/certs/ca-bundle.crt; do
    [ -f "$c" ] && bundles="$bundles $c"
  done
fi
[ -n "$bundles" ] || fail 13 timestamp "no CA bundle was found for the timestamp check: set VENOM_CA_FILE to one"
ts_ok=0
for ca in $bundles; do
  if ossl ts -verify -data "$p7s" -in "$tsr" -CAfile "$(native "$ca")" </dev/null >/dev/null 2>&1; then ts_ok=1; break; fi
done
[ "$ts_ok" -eq 1 ] || fail 13 timestamp "the timestamp token does not verify for SHA256SUMS.p7s (CA bundles tried:${bundles})"

want="$(awk -v a="$asset" '{ n=$2; sub(/^\*/, "", n); if (n == a) { print $1; exit } }' "$dir/SHA256SUMS" | tr 'A-F' 'a-f')"
[ -n "$want" ] || fail 10 checksum "$asset is not listed in SHA256SUMS"
have="$($sha_tool "$dir/$asset" | awk '{print $1}' | tr 'A-F' 'a-f')"
[ "$have" = "$want" ] || fail 10 checksum "$asset has the SHA-256 $have, SHA256SUMS lists $want"

printf 'OK signed-by %s (published key); timestamp checked\n' "$found"
