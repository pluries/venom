package skillcheck

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// US1-AS7, T009: a release signed by another certificate exits 12 and prints the found and the expected fingerprint.
func TestVerifySigner_US1AS7_OtherSignerExits12(t *testing.T) {
	requireVerifyTools(t)
	k := fixtureKeysFor(t)
	res := execVerify(t, k, verifyRun{dir: newRelease(t, k, k.two), pinned: k.one})
	assertVerdict(t, res, 12, "signer")
	out := normalizeFingerprints(res.stderr)
	require.Contains(t, out, k.two.fingerprint, "the signer found")
	require.Contains(t, out, k.one.fingerprint, "the signer expected")
}

var successLine = regexp.MustCompile(`^OK signed-by [0-9A-Fa-f:]+ \(published key\); timestamp checked$`)

// US1-AS1, T009: the success line says what was proven, "signed by the published key", and nothing stronger.
func TestVerifySigner_US1AS1_SuccessLineClaimsNothingStronger(t *testing.T) {
	requireVerifyTools(t)
	k := fixtureKeysFor(t)
	res := runVerify(t, nil, 0, "")
	line := strings.TrimSpace(res.stdout)
	require.Regexp(t, successLine, line)
	require.Contains(t, normalizeFingerprints(line), k.one.fingerprint)
}
