//go:build windows

package filesystem

import (
	"io/fs"
	"os"
)

// createExclusive creates the file at path with CREATE_NEW
// semantics, writes content, and flushes.
func (f *OSFS) createExclusive(path string, content []byte, mode fs.FileMode) error {
	fh, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if _, err := fh.Write(content); err != nil {
		fh.Close()
		return err
	}
	if err := fh.Sync(); err != nil {
		fh.Close()
		return err
	}
	return fh.Close()
}

// fsyncDirImpl is a no-op on Windows.
func (f *OSFS) fsyncDirImpl(path string) error {
	return nil
}
