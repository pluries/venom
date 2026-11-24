package recipecheck

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

// The release builds for 32-bit systems (linux-arm, linux-386). A constant that does not fit an int there broke the
// release build of 2026-11-24 (alias.go, sizeCap), and nothing on a 64-bit machine shows it.
func TestPackageTypeChecksOnThirtyTwoBitSystems(t *testing.T) {
	for _, arch := range []string{"386", "arm"} {
		t.Run(arch, func(t *testing.T) {
			cmd := exec.Command("go", "vet", ".")
			cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+arch, "CGO_ENABLED=0")
			out, err := cmd.CombinedOutput()
			require.NoError(t, err, string(out))
		})
	}
}
