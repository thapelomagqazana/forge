package filesystem

import (
	"io/fs"
	"os"
	"path/filepath"
)

// OSFS is an FS implementation backed by the operating system's
// filesystem.
//
// # Boundary semantics in Phase 2
//
// In Phase 2, the boundary is informational. Paths are resolved
// against it, but boundary enforcement (rejecting paths that escape)
// is not yet implemented. WBS 13.0 completes the enforcement.
//
// The current behaviour is:
//
//   - Paths are joined with the boundary using filepath.Join.
//   - A path that begins with "/" or "\" is joined anyway, which
//     produces an absolute path below the boundary. This is
//     incorrect but harmless in Phase 2 because no Forge command
//     accepts user-controlled paths yet.
//
// When WBS 13.0 lands, the implementation will:
//
//   - Reject absolute paths.
//   - Reject paths containing "..".
//   - Reject symlinks that escape the boundary.
//
// The interface does not change; only the implementation does.
type OSFS struct {
	boundary string
}

// NewOSFS returns an FS implementation bound to the given directory.
//
// The directory may or may not exist. Operations that create files
// will create parent directories as needed.
func NewOSFS(boundary string) *OSFS {
	return &OSFS{boundary: boundary}
}

// resolve joins the boundary and the path.
//
// The helper is private. It is the single place where the boundary
// is combined with a caller-supplied path. WBS 13.0 will extend it
// to enforce boundary rules.
func (f *OSFS) resolve(path string) string {
	return filepath.Join(f.boundary, path)
}

// Read returns the content of the file at path.
func (f *OSFS) Read(path string) ([]byte, error) {
	return os.ReadFile(f.resolve(path))
}

// Write writes content to the file at path.
func (f *OSFS) Write(path string, content []byte, mode fs.FileMode) error {
	full := f.resolve(path)

	// Ensure the parent directory exists.
	if dir := filepath.Dir(full); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	return os.WriteFile(full, content, mode)
}

// Exists reports whether a file or directory exists at path.
func (f *OSFS) Exists(path string) (bool, error) {
	_, err := os.Stat(f.resolve(path))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// MkdirAll creates a directory at path, along with parents.
func (f *OSFS) MkdirAll(path string, mode fs.FileMode) error {
	return os.MkdirAll(f.resolve(path), mode)
}

// Remove deletes the file at path.
func (f *OSFS) Remove(path string) error {
	return os.Remove(f.resolve(path))
}

// List returns the entries in the directory at path.
func (f *OSFS) List(path string) ([]fs.DirEntry, error) {
	return os.ReadDir(f.resolve(path))
}

// Stat returns metadata about the file or directory at path.
func (f *OSFS) Stat(path string) (fs.FileInfo, error) {
	return os.Stat(f.resolve(path))
}

// Boundary returns the directory against which paths are resolved.
func (f *OSFS) Boundary() string {
	return f.boundary
}
