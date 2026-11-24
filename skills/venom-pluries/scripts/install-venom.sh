#!/usr/bin/env bash
# Install a released Venom Pluries executable, after checking it. One command, so the developer approves download and
# check together.
#
#   install-venom.sh --version <tag> --dest <dir> [--from-dir <dir>] [--base-url <url>] [--release-env <file>] [--replace]
#
# The four release files go to a new temporary folder, never to <dest>; verify-release.sh checks them; only then is the
# asset copied to <dest>/venom (venom.exe for Windows) and `venom version` run. Nothing else is written.
#   20 no executable for this system   21 the download failed   22 a venom already exists in <dest> (use --replace)
# Other codes are those of verify-release.sh (10 to 15). VENOM_UNAME_S and VENOM_UNAME_M override uname (for tests).
set -u

die() { printf '%s\n' "$2" >&2; exit "$1"; }
usage() { die 2 "usage: install-venom.sh --version <tag> --dest <dir> [--from-dir <dir>] [--base-url <url>] [--release-env <file>] [--replace]"; }

version=""; dest=""; from_dir=""; base_url=""; envfile=""; replace=0
while [ $# -gt 0 ]; do
  case "$1" in
    --version|--dest|--from-dir|--base-url|--release-env) [ $# -ge 2 ] || usage ;;
  esac
  case "$1" in
    --version) version="${2-}"; shift 2 ;;
    --dest) dest="${2-}"; shift 2 ;;
    --from-dir) from_dir="${2-}"; shift 2 ;;
    --base-url) base_url="${2-}"; shift 2 ;;
    --release-env) envfile="${2-}"; shift 2 ;;
    --replace) replace=1; shift ;;
    *) usage ;;
  esac
done
[ -n "$version" ] && [ -n "$dest" ] || usage

here="$(cd "$(dirname "$0")" && pwd)"
if [ -z "$envfile" ]; then
  for c in "$here/release.env" "$here/../release.env"; do [ -f "$c" ] && { envfile="$c"; break; }; done
fi
[ -f "${envfile:-}" ] || die 15 "release.env was not found (give --release-env)"
repository="$(sed -n 's/^REPOSITORY=//p' "$envfile" | head -n 1 | tr -d '\r')"

# System and architecture to the asset of the release.
sys="${VENOM_UNAME_S:-$(uname -s)}"; arch="${VENOM_UNAME_M:-$(uname -m)}"
asset=""; target="venom"
case "$sys" in
  Linux)
    case "$arch" in
      x86_64|amd64) asset="venom.linux-amd64" ;;
      aarch64|arm64) asset="venom.linux-arm64" ;;
      armv7l|armv6l|armhf|arm) asset="venom.linux-arm" ;;
      i386|i686) asset="venom.linux-386" ;;
      ppc64le) asset="venom.linux-ppc64le" ;;
    esac ;;
  Darwin)
    case "$arch" in
      x86_64|amd64) asset="venom.darwin-amd64" ;;
      arm64|aarch64) asset="venom.darwin-arm64" ;;
    esac ;;
  MINGW*|MSYS*|CYGWIN*)
    case "$arch" in x86_64|amd64) asset="venom.windows-amd64.exe"; target="venom.exe" ;; esac ;;
  FreeBSD) case "$arch" in amd64|x86_64) asset="venom.freebsd-amd64" ;; esac ;;
  OpenBSD) case "$arch" in amd64|x86_64) asset="venom.openbsd-amd64" ;; esac ;;
esac
[ -n "$asset" ] || die 20 "no Venom Pluries executable exists for $sys $arch. Supported: Linux (amd64, arm64, arm, 386, ppc64le), Darwin (amd64, arm64), Windows (amd64, in Git Bash), FreeBSD (amd64), OpenBSD (amd64). Build from source only if you ask for it."

# Another venom on the PATH: say what it is. This install never changes it.
if existing="$(command -v venom 2>/dev/null)" && [ -n "$existing" ]; then
  found="$(venom version 2>/dev/null | sed -n 's/^Version venom: *//p' | head -n 1 | tr -d '\r')"
  if [ "$found" = "$version" ]; then
    printf 'A venom is on the PATH at %s: version %s, which is the pinned version. This install does not change it.\n' "$existing" "${found:-unknown}"
  else
    printf 'Another venom is on the PATH at %s: version %s, not Venom Pluries %s. This install does not change it.\n' "$existing" "${found:-unknown}" "$version"
  fi
fi

# A venom already in the destination counts too, whether or not it is on the PATH.
if [ -e "$dest/$target" ] && [ "$replace" -ne 1 ]; then
  found="$("$dest/$target" version 2>/dev/null | sed -n 's/^Version venom: *//p' | head -n 1 | tr -d '\r')"
  die 22 "$dest/$target already exists (version ${found:-unknown}). Nothing was changed; run again with --replace to install over it."
fi

work="$(mktemp -d)" || die 21 "cannot make a temporary folder"
trap 'rm -rf "$work"' EXIT

files="SHA256SUMS SHA256SUMS.p7s SHA256SUMS.p7s.tsr $asset"
if [ -n "$from_dir" ]; then
  for f in $files; do
    [ -f "$from_dir/$f" ] || die 15 "FAIL files: $f is missing from $from_dir"
    cp "$from_dir/$f" "$work/$f" || die 21 "cannot copy $f"
  done
else
  [ -n "$base_url" ] || base_url="https://github.com/$repository"
  proto=(--proto '=https' --proto-redir '=https')
  case "$base_url" in
    https://*) ;;
    http://127.0.0.1|http://127.0.0.1:*|http://127.0.0.1/*|http://localhost|http://localhost:*|http://localhost/*) proto=() ;;
    *) die 21 "the base URL must be https (or http to 127.0.0.1 or localhost for a test): $base_url" ;;
  esac
  command -v curl >/dev/null 2>&1 || die 14 "FAIL tools: curl was not found: install curl"
  for f in $files; do
    curl ${proto[@]+"${proto[@]}"} -L --fail --silent --show-error -o "$work/$f" "$base_url/releases/download/$version/$f" \
      || die 21 "the download of $f from $base_url failed"
  done
fi

verify="$here/verify-release.sh"
bash "$verify" --dir "$work" --asset "$asset" --release-env "$envfile" || exit $?

mkdir -p "$dest" || die 21 "cannot make $dest"
cp "$work/$asset" "$dest/$target" || die 21 "cannot write $dest/$target"
chmod +x "$dest/$target"
"$dest/$target" version
