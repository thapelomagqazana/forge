//go:build !windows

package filesystem

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// createExclusive creates the file at path with
// O_WRONLY | O_CREATE | O_EXCL | O_NOFOLLOW, writes content, and
// fsyncs. The ELOOP that the kernel returns for a symlink at the
// final component is normalised to fs.ErrExist, matching the
// interface's contract.
func (f *OSFS) createExclusive(path string, content []byte, mode fs.FileMode) error {
	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL | syscall.O_NOFOLLOW
	fh, err := os.OpenFile(path, flags, mode)
	if err != nil {
		if errors.Is(err, syscall.ELOOP) {
			return &fs.PathError{
				Op:   "open",
				Path: path,
				Err:  fs.ErrExist,
			}
		}
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

// fsyncDirImpl fsyncs the directory containing path.
func (f *OSFS) fsyncDirImpl(path string) error {
	dir := filepath.Dir(path)
	if dir == "" || dir == "." {
		return nil
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
