package skillcheck

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	latestTag   = regexp.MustCompile(`(?i)(:latest\b|/latest\b|--version\s+latest\b|\blatest\s+(tag|image|release))`)
	imageDigest = regexp.MustCompile(`ghcr\.io/pluries/venom@sha256:`)
	imageByTag  = regexp.MustCompile(`ghcr\.io/pluries/venom:[A-Za-z0-9_.<-]+`)
)

func installText(t *testing.T) string {
	t.Helper()
	return readFile(t, skillFile(t, "references/install.md"))
}

// US1-AS5, T052: install.md names the two pin files.
func TestInstallRules_US1AS5_NamesThePinFiles(t *testing.T) {
	text := installText(t)
	require.Contains(t, text, ".venom-version.image")
	require.Regexp(t, regexp.MustCompile(`\.venom-version[^.a-z]`), text, "the executable pin file is named on its own, not only as the prefix of the image file")
}

// US1-AS3, T052: no `latest` for the executable or the image; the image is written as @sha256:.
func TestInstallRules_US1AS3_NoLatestAndImageByDigest(t *testing.T) {
	text := installText(t)
	require.NotRegexp(t, latestTag, text, "install.md must not tell anyone to use latest")
	require.Regexp(t, imageDigest, text, "the image is written as ghcr.io/pluries/venom@sha256:<digest>")
}

// US1-AS3, T052: a tag of the image appears only with the digest beside it, on the same line; the one exception is the
// `imagetools inspect` command, which is how the plan says to resolve that digest from the tag.
func TestInstallRules_US1AS3_ImageTagAlwaysHasItsDigest(t *testing.T) {
	for _, line := range splitLines(installText(t)) {
		if imageByTag.MatchString(line) && !strings.Contains(line, "imagetools inspect") {
			require.Contains(t, line, "@sha256:", "an image tag needs its digest on the same line: %q", line)
		}
	}
}

func splitLines(s string) []string {
	return regexp.MustCompile(`\r?\n`).Split(s, -1)
}

// US1-AS5 (maker's decision for skill 0.1.2): on a developer machine the install is project-local by default, so the
// first install command of install.md uses --dest tools/venom/bin.
func TestInstallRules_US1AS5_DeveloperMachineInstallsProjectLocal(t *testing.T) {
	for _, line := range splitLines(installText(t)) {
		if strings.Contains(line, "install-venom.sh --version") {
			require.Contains(t, line, "--dest tools/venom/bin", "the first install command is the developer-machine one")
			return
		}
	}
	t.Fatal("install.md holds no install command")
}

// US1-AS5: install.md says tools/venom/bin/ goes into .gitignore, with the change shown and approved.
func TestInstallRules_US1AS5_ProjectLocalFolderIsGitignored(t *testing.T) {
	requireParagraph(t, installText(t), `tools/venom/bin`, `\.gitignore`, `approv`, `(show|difference)`)
}

// US1-AS5: the home folder is used only when the developer asks.
func TestInstallRules_US1AS5_HomeFolderOnlyOnRequest(t *testing.T) {
	requireParagraph(t, installText(t), `(\$HOME/\.local/bin|home folder)`, `\bonly\b`, `\bask`)
}

// US1-AS4: the exit-22 row says a venom already exists in the destination, not "another venom is installed".
func TestInstallRules_US1AS4_Exit22MeansAVenomInTheDestination(t *testing.T) {
	var row string
	for _, line := range splitLines(installText(t)) {
		if strings.HasPrefix(strings.TrimSpace(line), "| 22 |") {
			row = line
		}
	}
	require.NotEmpty(t, row, "the exit-code table has a row for 22")
	require.Regexp(t, regexp.MustCompile(`(?i)destination|--dest`), row)
	require.Regexp(t, regexp.MustCompile(`(?i)(exists|already)`), row)
	require.NotRegexp(t, regexp.MustCompile(`(?i)another .?venom.? is installed`), row)
}

// US1-AS4, US1-AS10: a venom on the PATH is reported and left alone; it does not stop the install.
func TestInstallRules_US1AS4_VenomOnPathIsReportedNotBlocking(t *testing.T) {
	requireParagraph(t, installText(t), `\bPATH\b`, `(report|say)`, `(does not change|not change|left alone|unchanged)`)
}
