# GitHub Actions

Use this when the project builds with GitHub Actions. The file below runs the suite with the pinned venom, fails the build when a test fails,
keeps the results as an artifact and gives credentials only from the system's secret variables.

- Before you add the file, check whether the project already has one. If it does, show the difference and get the developer's
  approval before you change it. Never overwrite a pipeline file silently.
- The file calls `tools/venom/install-venom.sh` (copied into the project, see `install.md`), which checks the download before it
  installs anything. If that step fails, the build fails before any suite runs.
- The version comes from `.venom-version`, the file that every machine and pipeline of the project reads.
- Credentials are secret variables of the system. Never write a value in the file. The workflow uses the `pull_request` trigger, which gives no secret to a pull request from a fork, and the credentialed step also checks that the pull request comes from the same repository.
- The results can hold response data (bodies and headers). Keep the artifact private to the project.
- Venom has no sandbox: a suite runs with the rights of the build. Do not run suites from untrusted pull requests or forks with
  secrets.

Save as `.github/workflows/venom.yml`:

```yaml
# Never run this file with secrets for pull requests from forks or other untrusted sources.
# The pull_request trigger gives a fork no secret; the suite step also checks that the pull request is from this repository.
name: venom
on:
  push:
    branches: [main]
  pull_request:
permissions:
  contents: read
jobs:
  venom:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # v4.2.2
      - name: Install venom
        run: |
          bash tools/venom/install-venom.sh --version "$(cat .venom-version)" --dest "$HOME/.local/bin"
          echo "$HOME/.local/bin" >> "$GITHUB_PATH"
      - name: Run the suite
        if: github.event_name == 'push' || github.event.pull_request.head.repo.full_name == github.repository
        env:
          VENOM_VAR_base_url: ${{ vars.VENOM_BASE_URL }}
          VENOM_VAR_token: ${{ secrets.VENOM_TOKEN }}
        run: venom run tests/venom/first.yml --format xml --output-dir tests/venom/results
      - name: Keep the results
        if: always()
        uses: actions/upload-artifact@ea165f8d65b6e75b540449e92b4886f43607fa02 # v4.6.2
        with:
          name: venom-results
          path: tests/venom/results
          if-no-files-found: ignore
```

Set `VENOM_BASE_URL` as a repository variable and `VENOM_TOKEN` as a repository secret, with the developer, in the repository's
settings. Do not set them yourself and do not ask for their values. The pins are 40-character commits of the two actions; to
move to another version, look up the commit of that tag and replace the pin.
