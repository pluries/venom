package mobile

import (
	"fmt"
	"os"
	"path/filepath"
)

// writeOwnerOnly writes data to path so that only the owner can read it, even when a file of that name already exists
// with wider rights: os.WriteFile keeps the mode of an existing file, so the data goes to a new file (mode 0600) that
// then replaces the old one. A failure leaves no temporary file behind.
func writeOwnerOnly(path string, data []byte) (err error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".mobile-*.tmp")
	if err != nil {
		return fmt.Errorf("cannot write %q: %w", path, err)
	}
	defer func() {
		if err != nil {
			_ = os.Remove(tmp.Name())
		}
	}()
	if err = tmp.Chmod(0o600); err == nil {
		_, err = tmp.Write(data)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		return fmt.Errorf("cannot write %q: %w", path, err)
	}
	return nil
}
