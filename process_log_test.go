//go:build unix

package venom

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOpenLogFileIsOwnerOnly pins #2300: venom.log is created readable by its owner only, whatever the umask. The mode
// only applies to a file that does not exist yet: a pre-existing file keeps the mode it has.
func TestOpenLogFileIsOwnerOnly(t *testing.T) {
	for _, mask := range []int{0o022, 0o000} {
		old := syscall.Umask(mask)
		file := filepath.Join(t.TempDir(), "venom.log")
		f, err := openLogFile(file)
		syscall.Umask(old)
		require.NoError(t, err)
		require.NoError(t, f.Close())

		info, err := os.Stat(file)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "umask %04o", mask)
	}
}
