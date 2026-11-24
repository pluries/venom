# Jenkins

Use this when the project builds with Jenkins. The file below runs the suite with the pinned venom, fails the build when a test fails,
keeps the results as an artifact and gives credentials only from the system's secret variables.

- Before you add the file, check whether the project already has one. If it does, show the difference and get the developer's
  approval before you change it. Never overwrite a pipeline file silently.
- The file calls `tools/venom/install-venom.sh` (copied into the project, see `install.md`), which checks the download before it
  installs anything. If that step fails, the build fails before any suite runs.
- The version comes from `.venom-version`, the file that every machine and pipeline of the project reads.
- Credentials are secret variables of the system. Never write a value in the file. The credentialed stage runs only for the trusted branch `main`; other branches and pull requests get no credential.
- The results can hold response data (bodies and headers). Keep the artifact private to the project.
- Venom has no sandbox: a suite runs with the rights of the build. Do not run suites from untrusted pull requests or forks with
  secrets.

Save as `Jenkinsfile` (or add the stages to the existing one, with the difference shown):

```groovy
// Never run this file with secrets for pull requests from forks or other untrusted sources.
// The credential is bound only for the trusted branch main; other branches and pull requests get no secret.
pipeline {
  agent any
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
    }
  }
  post {
    always {
      junit allowEmptyResults: true, testResults: 'tests/venom/results/*.xml'
      archiveArtifacts artifacts: 'tests/venom/results/**', allowEmptyArchive: true
    }
  }
}
```

Create the credential `venom-token` (secret text) in Jenkins with the developer. Do not create it yourself and do not ask for its
value. The address of the service under test is given by the job's environment (`VENOM_VAR_base_url`), set by the developer.
