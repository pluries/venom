# GitLab CI

Use this when the project builds with GitLab CI. The file below runs the suite with the pinned venom, fails the build when a test fails,
keeps the results as an artifact and gives credentials only from the system's secret variables.

- Before you add the file, check whether the project already has one. If it does, do not change it in the same turn that proposes the
  change: put the difference in your message, end the turn with the question, and edit the file only after the developer has
  approved it by answering yes. Never overwrite a pipeline file silently.
- The file calls `tools/venom/install-venom.sh` (copied into the project, see `install.md`), which checks the download before it
  installs anything. If that step fails, the build fails before any suite runs.
- The version comes from `.venom-version`, the file that every machine and pipeline of the project reads.
- This file has not been run on a real GitLab server: the maintainers of this skill had no GitLab, so it was checked by tests
  and by reading it, while the GitHub, Jenkins and Azure files were run. Tell the developer so, and that the first run is theirs:
  they should check that a failing test fails the job and that a merge request from a fork gets no secret.
- Credentials are secret variables of the system. Never write a value in the file. The job runs only on protected branches and tags, and the variables are protected and masked, so a merge request from a fork never gets them.
- The results can hold response data (bodies and headers). Keep the artifact private to the project.
- Venom has no sandbox: a suite runs with the rights of the build. Do not run suites from untrusted pull requests or forks with
  secrets.

Save as `.gitlab-ci.yml` (or add the job to the existing one, with the difference shown):

```yaml
# Never run this file with secrets for pull requests from forks or other untrusted sources.
# The job runs on protected refs only, and the variables are protected, so a fork's merge request gets no secret.
venom:
  image: debian:stable-slim
  rules:
    - if: $CI_COMMIT_REF_PROTECTED == "true"
  before_script:
    - apt-get update -qq && apt-get install -y -qq curl openssl ca-certificates
    - bash tools/venom/install-venom.sh --version "$(cat .venom-version)" --dest "$HOME/.local/bin"
    - export PATH="$HOME/.local/bin:$PATH"
  script:
    - VENOM_VAR_base_url="$VENOM_BASE_URL" VENOM_VAR_token="$VENOM_TOKEN" venom run tests/venom/first.yml --format xml --output-dir tests/venom/results
  artifacts:
    when: always
    paths:
      - tests/venom/results
    reports:
      junit: tests/venom/results/*.xml
```

Set `VENOM_BASE_URL` and `VENOM_TOKEN` as CI/CD variables of the project, with the developer, and mark both Protected and the token
Masked. Do not set them yourself and do not ask for their values.
