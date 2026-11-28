package skillcheck

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// installCommand is the install step every CI body runs, word for word (US3-AS6).
const installCommand = `bash tools/venom/install-venom.sh --version "$(cat .venom-version)" --dest "$HOME/.local/bin"`

var fencedBlock = regexp.MustCompile("(?s)```([A-Za-z0-9_-]*)[ \t]*\r?\n(.*?)\r?\n```")

// ciBody extracts the one fenced file body of a CI reference: the block whose language tag is the file kind.
func ciBody(t *testing.T, r ciRef) string {
	t.Helper()
	var bodies []string
	for _, m := range fencedBlock.FindAllStringSubmatch(skillText(t, r.file), -1) {
		if m[1] == r.lang {
			bodies = append(bodies, m[2])
		}
	}
	require.Len(t, bodies, 1, "%s must hold exactly one fenced %s file body", r.file, r.lang)
	return bodies[0]
}

// forEachCIBody runs fn as a subtest per CI system with its extracted body.
func forEachCIBody(t *testing.T, fn func(t *testing.T, r ciRef, body string)) {
	t.Helper()
	for _, r := range ciRefs {
		t.Run(r.system, func(t *testing.T) { fn(t, r, ciBody(t, r)) })
	}
}

// US3-AS6, US1-AS5: the install step is the script of the project's copy, fed by the pin file.
func TestCI_US3AS6_InstallStepIsTheScript(t *testing.T) {
	forEachCIBody(t, func(t *testing.T, _ ciRef, body string) {
		require.Contains(t, body, installCommand)
	})
}

var pipedToShell = regexp.MustCompile(`(?i)(curl|wget|iwr|Invoke-WebRequest)\b[^\n]*\|\s*(sudo\s+)?\w*(sh|iex)\b`)

// US3-AS7: no download piped into a shell, in any body.
func TestCI_US3AS7_NoDownloadPipedIntoAShell(t *testing.T) {
	forEachCIBody(t, func(t *testing.T, _ ciRef, body string) {
		require.NotRegexp(t, pipedToShell, body)
	})
}

var (
	usesLine   = regexp.MustCompile(`(?m)^\s*-?\s*uses:\s*([^\s#]+)`)
	pinnedUses = regexp.MustCompile(`^[^@]+@[0-9a-f]{40}$`)
)

// US3-AS7: every action of the GitHub body that does not come from the actions organisation is pinned by a
// 40-character commit; local actions need no pin.
func TestCI_US3AS7_GitHubThirdPartyActionsPinnedByCommit(t *testing.T) {
	body := ciBody(t, ciRefs[0])
	for _, m := range usesLine.FindAllStringSubmatch(body, -1) {
		ref := m[1]
		if strings.HasPrefix(ref, "./") || strings.HasPrefix(ref, "actions/") {
			continue
		}
		require.Regexp(t, pinnedUses, ref, "third-party action %s must be pinned by a 40-character commit", ref)
	}
}

// US3-AS7: no body uses the trigger that hands secrets to fork pull requests; GitHub runs on pull_request.
func TestCI_US3AS7_NoForkTriggerWithSecrets(t *testing.T) {
	forEachCIBody(t, func(t *testing.T, _ ciRef, body string) {
		require.NotContains(t, body, "pull_request_target")
	})
	require.Regexp(t, regexp.MustCompile(`(?m)^\s*pull_request\s*:`), ciBody(t, ciRefs[0]))
}

var literalCredential = regexp.MustCompile(`(?i)\b(password|passwd|secret|token|api[_-]?key)\w*\s*[:=]\s*['"]?([^\s'"]{8,})`)

// US3-AS3: no literal credential in any body: a value assigned to a credential-like name must be a reference
// to the system's secret syntax, never a literal.
func TestCI_US3AS3_NoLiteralCredential(t *testing.T) {
	forEachCIBody(t, func(t *testing.T, _ ciRef, body string) {
		for _, line := range splitLines(body) {
			for _, m := range literalCredential.FindAllStringSubmatch(line, -1) {
				v := m[2]
				ok := strings.HasPrefix(v, "$") || strings.HasPrefix(v, "<") || strings.HasPrefix(v, "credentials(")
				require.True(t, ok, "literal value on a credential-like line: %q", line)
			}
		}
	})
}

// US3-AS3, US3-AS7: credentials come from the system's secret syntax, and each system's rule for forks holds.
func TestCI_US3AS3_AS7_SecretSyntaxPerSystem(t *testing.T) {
	forEachCIBody(t, func(t *testing.T, r ciRef, body string) {
		switch r.system {
		case "GitHub Actions":
			require.Regexp(t, regexp.MustCompile(`\$\{\{\s*secrets\.[A-Za-z0-9_]+\s*\}\}`), body)
		case "GitLab CI":
			require.Regexp(t, regexp.MustCompile(`(?i)protected`), skillText(t, r.file), "GitLab variables are protected, not available to unprotected branches")
		case "Jenkins":
			requireJenkinsCredentialsGuarded(t, body)
		case "Azure Pipelines":
			require.Regexp(t, regexp.MustCompile(`\$\([A-Za-z0-9_.]+\)`), body)
			require.Regexp(t, regexp.MustCompile(`(?i)fork`), skillText(t, r.file), "Azure secret variables are not exposed to forks")
		}
	})
}

// requireJenkinsCredentialsGuarded checks that credentials are bound only after a trusted-branch condition.
func requireJenkinsCredentialsGuarded(t *testing.T, body string) {
	t.Helper()
	bind := regexp.MustCompile(`withCredentials|credentials\(`).FindStringIndex(body)
	require.NotNil(t, bind, "the Jenkins body binds its credentials")
	cond := regexp.MustCompile(`branch\s+['"]|BRANCH_NAME|changeRequest\(\)|CHANGE_ID`).FindStringIndex(body)
	require.NotNil(t, cond, "the Jenkins body has a trusted-branch condition")
	require.Less(t, cond[0], bind[0], "the trusted-branch condition comes before the credentials")
}

// US3 (0.1.7, decision D-021 of spec 016): the maintainers had no GitLab, so the GitLab file was read, never run. The reference says
// so, and tells the skill to pass that on to the developer, who is the first to run it.
func TestCI_GitLab_SaysItWasNotRun(t *testing.T) {
	r := ciRefs[1]
	require.Equal(t, "GitLab CI", r.system)
	text := skillText(t, r.file)
	requireParagraph(t, text, `(?i)\bnot\b`, `(?i)\b(run|executed|tested)\b`, `(?i)maintainers`, `(?i)\btell\b`)
}

// US3-AS1 (0.1.6, evaluation of 0.1.5 on a real Jenkins): `agent any` waits for ever on a Jenkins whose agents are reserved by
// label, so the reference asks the developer for the label of an agent that has the tools, and the file never says `agent any`.
func TestCI_Jenkins_AgentLabelIsAskedNotAny(t *testing.T) {
	r := ciRefs[2]
	require.Equal(t, "Jenkins", r.system)
	body := ciBody(t, r)
	require.NotRegexp(t, regexp.MustCompile(`agent\s+any`), body)
	require.Regexp(t, regexp.MustCompile(`agent\s*\{\s*label\s+'`), body)
	requireParagraph(t, skillText(t, r.file), `\bagent\b`, `\blabel\b`, `\bask`)
}

// US3-AS2 (0.1.6): a branch that gets no credential runs no suite; its build must not be green, which would read as "tested".
func TestCI_Jenkins_SkippedSuiteIsNotGreen(t *testing.T) {
	body := ciBody(t, ciRefs[2])
	require.Regexp(t, regexp.MustCompile(`not\s*\{\s*branch\s+'main'\s*\}`), body)
	require.Contains(t, body, "unstable(")
	requireParagraph(t, skillText(t, ciRefs[2].file), `unstable`, `(not tested|nothing was tested)`)
}

// US3-AS2 (0.1.6): where the suite ran, a missing report or artifact fails the build instead of passing silently.
func TestCI_Jenkins_EmptyResultsAreNotAccepted(t *testing.T) {
	body := ciBody(t, ciRefs[2])
	require.NotRegexp(t, regexp.MustCompile(`allowEmpty(Results|Archive)\s*:\s*true`), body)
	require.Contains(t, body, "junit")
}

// US3-AS3: the file says that suites from untrusted pull requests must not run with those secrets.
func TestCI_US3AS3_FileWarnsAboutUntrustedPullRequests(t *testing.T) {
	forEachCIBody(t, func(t *testing.T, _ ciRef, body string) {
		require.Regexp(t, regexp.MustCompile(`(?is)(untrusted|fork).{0,240}secret|secret.{0,240}(untrusted|fork)`), body)
	})
}

var reportKept = map[string]string{
	"GitHub Actions":  `actions/upload-artifact`,
	"GitLab CI":       `artifacts:`,
	"Jenkins":         `archiveArtifacts`,
	"Azure Pipelines": `PublishPipelineArtifact|PublishBuildArtifacts`,
}

// US3-AS1: the body runs the suite, writes a report in the xml (JUnit) format into a folder, and keeps it as an artifact.
func TestCI_US3AS1_ReportWrittenAndKept(t *testing.T) {
	forEachCIBody(t, func(t *testing.T, r ciRef, body string) {
		require.Contains(t, body, "venom run")
		require.Regexp(t, regexp.MustCompile(`--format[ =]xml`), body)
		require.Contains(t, body, "--output-dir")
		require.Regexp(t, regexp.MustCompile(reportKept[r.system]), body)
	})
}

var failureSwallowed = regexp.MustCompile(`(?i)(\|\|\s*true|\|\|\s*exit\s+0|continue-on-error:\s*true|allow_failure:\s*true|continueOnError:\s*true|set \+e|catchError|returnStatus:\s*true)`)

// US3-AS2: the failing test fails the build: nothing in the body swallows the exit code of venom.
func TestCI_US3AS2_FailingTestFailsTheBuild(t *testing.T) {
	forEachCIBody(t, func(t *testing.T, _ ciRef, body string) {
		require.NotRegexp(t, failureSwallowed, body)
	})
}

// US1-AS3, FR-002: no body uses latest for the executable or the image, and an image is pinned by digest.
func TestCI_US1AS3_NoLatestAndImageByDigest(t *testing.T) {
	forEachCIBody(t, func(t *testing.T, _ ciRef, body string) {
		require.NotRegexp(t, regexp.MustCompile(`(?i)(:latest\b|--version\s+latest\b)`), body)
		if strings.Contains(body, "ghcr.io/pluries/venom") {
			require.Regexp(t, regexp.MustCompile(`@sha256:|\.venom-version\.image`), body)
		}
	})
}

// US3-AS5 (0.1.8, evaluation of 0.1.7: the existing pipeline was edited in the run, four systems of four): the file is not changed in
// the turn that proposes the change. The reference says the difference goes into the message, the turn ends with the question, and the
// edit follows only the developer's answer.
func TestCI_US3AS5_ExistingPipelineIsNotEditedInTheProposingTurn(t *testing.T) {
	for _, r := range ciRefs {
		t.Run(r.system, func(t *testing.T) {
			requireParagraph(t, skillText(t, r.file), `\balready\b`, `\bdifference\b`, `\b(same|proposing) turn\b`, `\b(end|stop)s?\b[^.]*\bquestion\b`, `\bonly after\b`)
		})
	}
	requireParagraph(t, skillText(t, "SKILL.md"), `\bdifference\b`, `\bapproval\b`, `\b(same|proposing) turn\b`)
}
