package skillcheck

import "testing"

// US1-AS2, T006: the signature is good, so the checksum is the one check that fails.
func TestVerifyChecksum_US1AS2_ChangedExecutableExits10(t *testing.T) {
	runVerify(t, flipByte(releaseAsset, -1), 10, "checksum")
}

// US1-AS2, T006: a release nothing touched passes with the single success line (control for the case above).
func TestVerifyChecksum_US1AS2_UntouchedReleasePasses(t *testing.T) {
	runVerify(t, nil, 0, "")
}
