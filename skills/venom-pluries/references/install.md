# Install Venom Pluries

Read this before you install venom in a project. Everything here runs only with the developer's approval of each command,
and nothing is installed that has not been checked.

## What you need on the machine

- `bash` (on Windows: Git Bash, which ships with Git for Windows), `curl`, `openssl` 3 or later, and `sha256sum` or `shasum`.
  On macOS the built-in `openssl` is LibreSSL, which cannot verify the timestamp: install OpenSSL 3 (`brew install openssl`); the
  script finds the Homebrew copy by itself.
- If one is missing, the install script stops and names it. Tell the developer what to install; do not install it for them
  and do not skip a check.

## Step 1: copy the two scripts into the project

The CI system of the project has only the project's files, so the scripts live in the project, in `tools/venom/`:

1. Copy `scripts/install-venom.sh`, `scripts/verify-release.sh` and `release.env` from this skill into `tools/venom/`.
2. If `tools/venom/` already exists, show the difference and get approval before you overwrite anything.

## Step 2: pick the version and install

1. The version is the one in `release.env` (`VENOM_VERSION`). Write it, alone on one line, to `.venom-version` at the project
   root. The developer may choose another file; say so. Every pipeline and the developer's machine read this one file, so
   they all use the same version.
2. Show the developer this one command and let Claude Code ask for approval. It downloads the release into a temporary
   folder, checks it, and only then copies the executable:

   ```bash
   bash tools/venom/install-venom.sh --version "$(cat .venom-version)" --dest tools/venom/bin
   ```

   On a developer machine the install is project-local: `tools/venom/bin` inside the project, so nothing is written outside it.
   Add the line `tools/venom/bin/` to the project's `.gitignore` (show the change and get the developer's approval, as for
   any file). Install into the home folder (`$HOME/.local/bin`) only if the developer asks for it. A build system installs
   into its own work folder; the CI references give that command.
3. The script prints one line when every check passed: `OK signed-by <fingerprint> (published key); timestamp checked`.
   Report exactly that. It proves: the list of checksums was signed by the key whose fingerprint is in `release.env`, a
   timestamp covers the signature, and the executable matches the list. It does not prove that a certificate authority
   trusts the key, and it does not prove the certificate is valid today. Never write "trusted" or "verified by Certum".
4. Run `venom version` and show the result.

## The exit codes

| Code | Meaning | What to say |
|---|---|---|
| 10 | The checksum of the executable does not match `SHA256SUMS` | The file is not the released one. Stop. |
| 11 | The signature of `SHA256SUMS` does not verify | Stop. Do not offer to skip the check. |
| 12 | `SHA256SUMS` is signed by another certificate | Show the found and the expected fingerprint. It may be a renewed certificate (then this skill needs an update) or a swapped release. Stop. |
| 13 | The timestamp does not verify, or no CA bundle was found | Say which. Set `VENOM_CA_FILE` to a bundle only if the developer provides one. |
| 14 | A tool is missing, or `openssl` is not OpenSSL 3 or later (LibreSSL, OpenSSL 1.x) | Name it and what to install. |
| 15 | A file is missing | Name it. |
| 20 | No executable exists for this system | Name the supported systems: Linux (amd64, arm64, arm, 386, ppc64le), macOS (amd64, arm64), Windows amd64 in Git Bash, FreeBSD and OpenBSD amd64. Do not build from source unless the developer asks. |
| 21 | The download failed | Offer the files saved by hand (below). |
| 22 | A venom already exists in the destination folder | Say what it is and its version. Change nothing unless the developer approves `--replace`. |

## A venom that is already there

A venom that is already on the PATH is reported by the install script (its path and its version) and is not changed: the
install goes to the destination folder and leaves it alone. Tell the developer which venom is which, and that the project's
copy is called by its path (`tools/venom/bin/venom`, `venom.exe` on Windows). If it is not the pinned Venom Pluries version
(it may be the upstream project or an older fork), say so. Only a venom that already sits in the destination folder stops the
install (exit 22); replace it only with the developer's approval.

## No internet, or behind a proxy

The developer saves these four files by hand from the release page of the repository named in `release.env`:
`SHA256SUMS`, `SHA256SUMS.p7s`, `SHA256SUMS.p7s.tsr` and the executable of the system. Then run the same script with the
folder, so the same checks run:

```bash
bash tools/venom/install-venom.sh --version "$(cat .venom-version)" --dest tools/venom/bin --from-dir <folder>
```

## The image instead of the executable

The image is `ghcr.io/pluries/venom`. It is built for linux/amd64 only; on another system say so and offer the executable.

1. Resolve the digest of the pinned tag, with approval of the command:

   ```bash
   docker buildx imagetools inspect ghcr.io/pluries/venom:v1.5.0-pluries.1
   ```

2. Write the full reference with the digest, `ghcr.io/pluries/venom@sha256:<digest>`, alone on one line to
   `.venom-version.image`. Always use the digest form for the image; a tag can be moved, a digest cannot.
3. Run the first suite with the tests folder and the results folder mounted, using that reference. The image has no
   signature that this skill checks; say so to the developer.

## What is out of the skill's hands

A release that was withdrawn after this skill was written cannot be known offline. If the developer is told that a release was
withdrawn, choose another pinned version with them; never move silently to the newest.
