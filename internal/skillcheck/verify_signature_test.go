package skillcheck

import "testing"

// US1-AS2, T007: a changed byte in SHA256SUMS or in its signature exits 11 and names the signature;
// the signature is checked before the checksum, so a changed executable beside it does not change the verdict.
func TestVerifySignature_US1AS2(t *testing.T) {
	cases := []struct {
		name   string
		mutate mutation
	}{
		{"changed SHA256SUMS", flipByte(sumsName, 0)},
		{"changed SHA256SUMS.p7s", flipByte(p7sName, -1)},
		{"changed signature and executable, signature first", allOf(flipByte(p7sName, -1), flipByte(releaseAsset, -1))},
		{"changed SHA256SUMS and executable, signature first", allOf(flipByte(sumsName, 0), flipByte(releaseAsset, -1))},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			runVerify(t, c.mutate, 11, "signature")
		})
	}
}
