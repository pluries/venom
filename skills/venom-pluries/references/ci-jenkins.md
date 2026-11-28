# Jenkins

Use this when the project builds with Jenkins. The file below runs the suite with the pinned venom, fails the build when a test fails,
keeps the results as an artifact and gives credentials only from the system's secret variables.

- Before you add the file, check whether the project already has one. If it does, do not change it in the same turn that proposes the
  change: put the difference in your message, end the turn with the question, and edit the file only after the developer has
  approved it by answering yes. Never overwrite a pipeline file silently.
- The file calls `tools/venom/install-venom.sh` (copied into the project, see `install.md`), which checks the download before it
  installs anything. If that step fails, the build fails before any suite runs.
- The version comes from `.venom-version`, the file that every machine and pipeline of the project reads.
- The agent: ask the developer for the label of an agent that has bash, curl and OpenSSL 3 (and what the suite and the service
  under test need), and write it in `agent { label '...' }`. Do not leave `agent any`: on a Jenkins whose agents are reserved by
  label, such a build waits in the queue for ever and nothing says why.
- Credentials are secret variables of the system. Never write a value in the file. The credentialed stage runs only for the trusted branch `main`; other branches and pull requests get no credential.
- A build of any other branch therefore runs no suite. The file marks that build `UNSTABLE` with `unstable(...)` and says so, because a
  green build would read as "tested". Tell the developer: only `main` is tested here; to test a branch, they decide how it may get a
  credential. Where the suite does run, a missing report fails the build (`allowEmptyResults: false`).
- The results can hold response data (bodies and headers). Keep the artifact private to the project.
- Venom has no sandbox: a suite runs with the rights of the build. Do not run suites from untrusted pull requests or forks with
  secrets.

Save as `Jenkinsfile` (or add the stages to the existing one, with the difference shown):

```groovy
// Never run this file with secrets for pull requests from forks or other untrusted sources.
// The credential is bound only for the trusted branch main; other branches and pull requests get no secret.
pipeline {
  // The label of an agent that has bash, curl and OpenSSL 3. Ask the developer; a build with no label can wait in the queue for ever.
  agent { label '<label of an agent with bash, curl and OpenSSL 3>' }
  stages {
    stage('Install venom') {
      steps {
        sh 'bash tools/venom/install-venom.sh --version "$(cat .venom-version)" --dest "$HOME/.local/bin"'
      }
    }
    stage('Venom') {
      when { branch 'main' }
      steps {
        withCredentials([string(credentialsId: 'venom-token', variable: 'VENOM_VAR_token')]) {
          sh 'PATH="$HOME/.local/bin:$PATH" venom run tests/venom/first.yml --format xml --output-dir tests/venom/results'
        }
      }
      post {
        always {
          junit allowEmptyResults: false, testResults: 'tests/venom/results/*.xml'
          archiveArtifacts artifacts: 'tests/venom/results/**', allowEmptyArchive: false
        }
      }
    }
    stage('Venom not run') {
      when { not { branch 'main' } }
      steps {
        unstable('The suite did not run: only main gets the credential. A green build of this branch would mean nothing was tested.')
      }
    }
  }
}
```

Create the credential `venom-token` (secret text) in Jenkins with the developer. Do not create it yourself and do not ask for its
value. The address of the service under test is given by the job's environment (`VENOM_VAR_base_url`), set by the developer.
