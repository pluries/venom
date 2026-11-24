# Azure Pipelines

Use this when the project builds with Azure Pipelines. The file below runs the suite with the pinned venom, fails the build when a test fails,
keeps the results as an artifact and gives credentials only from the system's secret variables.

- Before you add the file, check whether the project already has one. If it does, show the difference and get the developer's
  approval before you change it. Never overwrite a pipeline file silently.
- The file calls `tools/venom/install-venom.sh` (copied into the project, see `install.md`), which checks the download before it
  installs anything. If that step fails, the build fails before any suite runs.
- The version comes from `.venom-version`, the file that every machine and pipeline of the project reads.
- Credentials are secret variables of the system. Never write a value in the file. Pipeline secret variables are not given to builds of forks; never turn on the setting that makes secrets available to fork builds.
- The results can hold response data (bodies and headers). Keep the artifact private to the project.
- Venom has no sandbox: a suite runs with the rights of the build. Do not run suites from untrusted pull requests or forks with
  secrets.

Save as `azure-pipelines.yml` (or add the steps to the existing one, with the difference shown):

```yaml
# Never run this file with secrets for pull requests from forks or other untrusted sources.
# Secret variables are not given to builds of forks; never turn on the setting that gives secrets to fork builds.
trigger:
  branches:
    include: [main]
pr:
  branches:
    include: [main]
pool:
  vmImage: ubuntu-latest
steps:
  - script: bash tools/venom/install-venom.sh --version "$(cat .venom-version)" --dest "$HOME/.local/bin"
    displayName: Install venom
  - script: PATH="$HOME/.local/bin:$PATH" venom run tests/venom/first.yml --format xml --output-dir tests/venom/results
    displayName: Run the suite
    condition: and(succeeded(), ne(variables['Build.Reason'], 'PullRequest'))
    env:
      VENOM_VAR_base_url: $(VENOM_BASE_URL)
      VENOM_VAR_token: $(VENOM_TOKEN)
  - task: PublishTestResults@2
    condition: always()
    inputs:
      testResultsFormat: JUnit
      testResultsFiles: tests/venom/results/*.xml
      failTaskOnFailedTests: true
  - task: PublishPipelineArtifact@1
    condition: always()
    inputs:
      targetPath: tests/venom/results
      artifact: venom-results
```

Set `VENOM_BASE_URL` as a variable and `VENOM_TOKEN` as a secret variable of the pipeline, with the developer. Do not set them
yourself and do not ask for their values.
