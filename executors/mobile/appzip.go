package mobile

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
)

// zipBundle packs a .app folder into a temporary zip (created 0600 in os.TempDir) and returns its path and the
// function that removes it. The bundle's own folder name is the first level of every entry, as Appium expects.
func zipBundle(dir string) (zipped string, cleanup func(), err error) {
	if info, statErr := os.Lstat(dir); statErr == nil && info.Mode()&fs.ModeSymlink != 0 {
		return "", nil, usagef("mobile.app must not be a symlink")
	}
	f, err := os.CreateTemp("", "venom-mobile-*.zip")
	if err != nil {
		return "", nil, fmt.Errorf("cannot create the archive of the .app folder: %w", err)
	}
	cleanup = func() { _ = os.Remove(f.Name()) }
	if err := writeBundle(f, dir); err != nil {
		_ = f.Close()
		cleanup()
		return "", nil, err
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("cannot write the archive of the .app folder: %w", err)
	}
	return f.Name(), cleanup, nil
}

// writeBundle writes every entry of the folder to w as a zip. A symbolic link or any other special file is refused:
// a link could point outside the bundle and venom does not follow it.
func writeBundle(w io.Writer, root string) error {
	zw := zip.NewWriter(w)
	base := filepath.Base(root)
	walk := func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil || rel == "." {
			return err
		}
		return addEntry(zw, p, path.Join(base, filepath.ToSlash(rel)), rel, d)
	}
	if err := filepath.WalkDir(root, walk); err != nil {
		_ = zw.Close()
		return err
	}
	return zw.Close()
}

// addEntry adds one folder or regular file under name; rel is the path inside the bundle, for the messages.
func addEntry(zw *zip.Writer, file, name, rel string, d fs.DirEntry) error {
	switch {
	case d.IsDir():
		_, err := zw.Create(name + "/")
		return err
	case d.Type().IsRegular():
		return addFile(zw, file, name, d)
	case d.Type()&fs.ModeSymlink != 0:
		return usagef("mobile.app contains a symbolic link, which venom does not pack: %s", rel)
	default:
		return usagef("mobile.app contains a file that is not a regular file: %s", rel)
	}
}

// addFile copies a regular file, with its mode (the executable bit matters in a bundle).
func addFile(zw *zip.Writer, file, name string, d fs.DirEntry) error {
	info, err := d.Info()
	if err != nil {
		return err
	}
	hdr, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	hdr.Name, hdr.Method = name, zip.Deflate
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	src, err := os.Open(file)
	if err != nil {
		return err
	}
	defer src.Close()
	_, err = io.Copy(w, src)
	return err
}
