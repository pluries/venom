package skillcheck

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// US1-AS2, T008: a changed timestamp token exits 13 (the local timestamp root is given by VENOM_CA_FILE).
func TestVerifyTimestamp_US1AS2_ChangedTokenExits13(t *testing.T) {
	runVerify(t, flipByte(tsrName, -1), 13, "timestamp")
}

// US1-AS2, T008: a timestamp root that did not issue the token exits 13.
func TestVerifyTimestamp_US1AS2_WrongRootExits13(t *testing.T) {
	requireVerifyTools(t)
	k := fixtureKeysFor(t)
	res := execVerify(t, k, verifyRun{dir: newRelease(t, k, k.one), pinned: k.one, caFile: k.two.certPath})
	assertVerdict(t, res, 13, "timestamp")
}

// US1-AS2, T008: a missing file exits 15, names the check and the file.
func TestVerifyFiles_US1AS2_MissingFileExits15(t *testing.T) {
	for _, name := range []string{releaseAsset, sumsName, p7sName, tsrName} {
		t.Run(name, func(t *testing.T) {
			res := runVerify(t, removeFile(name), 15, "files")
			require.Contains(t, res.stderr, name)
		})
	}
}
