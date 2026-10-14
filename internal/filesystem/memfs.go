package filesystem

import (
	"errors"
	"io/fs"
	"path"
	"sort"
	"strings"
	"testing/fstest"
)

// NewMemFS returns a filesystem.FS backed by an in-memory map.
//
// The returned FS implements the full filesystem.FS contract:
// read, write, list, stat, lstat, readlink, and the three
// atomic-write primitives (WriteFileExclusive, Rename,
// FsyncDir). It is the filesystem of choice for tests and for
// any tool that needs to exercise code that depends on
// filesystem.FS without touching the disk.
//
// # Why this exists
//
// fstest.MapFS implements io/fs.FS, io/fs.StatFS, and
// io/fs.ReadDirFS. It does not implement filesystem.FS, which
// declares a wider surface. This adapter is the single place
// where the two contracts are bridged.
//
// The adapter is deliberately in the production package, not in
// a test helper file, because the bridge is a property of the
// filesystem contract, not of any one test suite.
//
// # Symlink semantics
//
// fstest.MapFS does not represent symlinks. The adapter therefore
// reports "not a symlink" for every path: Lstat returns the same
// result as Stat, and Readlink returns fs.ErrInvalid. A test
// that needs real symlink behaviour must use OSFS against a real
// t.TempDir().
//
// # Atomic-write semantics
//
// The three atomic-write primitives are implemented in-memory:
//
//   - WriteFileExclusive stores the content under the name and
//     fails if the name is already present. There is no fsync;
//     an in-memory filesystem has no durability concept.
//
//   - Rename moves the entry from oldPath to newPath, replacing
//     the target if it exists.
//
//   - FsyncDir is a no-op; the durability it provides on a real
//     filesystem has no meaning in memory.
//
// # Concurrency
//
// The returned FS is not safe for concurrent use. The underlying
// fstest.MapFS is a plain map, and the adapter does not add a
// mutex. safefs.Root serialises calls to its FS, so this is not
// a problem for the Root contract; a caller that drives MemFS
// directly must synchronise.
type MemFS struct {
	m    fstest.MapFS
	root string
}

// NewMemFS returns an FS backed by m. The root string is
// informational; it is returned by Boundary and plays no role in
// path resolution, because fstest.MapFS has its own flat
// namespace.
func NewMemFS(m fstest.MapFS) *MemFS {
	return &MemFS{m: m, root: "/mem"}
}

// Boundary returns the informational root.
func (f *MemFS) Boundary() string { return f.root }

// Read returns the contents of the named file.
func (f *MemFS) Read(name string) ([]byte, error) {
	return fs.ReadFile(f.m, name)
}

// Open opens the named file for reading.
func (f *MemFS) Open(name string) (fs.File, error) {
	return f.m.Open(name)
}

// Stat returns metadata about name, following no symlinks (which
// fstest.MapFS does not represent).
func (f *MemFS) Stat(name string) (fs.FileInfo, error) {
	return fs.Stat(f.m, name)
}

// Lstat is identical to Stat for an in-memory FS: there are no
// symlinks to distinguish.
func (f *MemFS) Lstat(name string) (fs.FileInfo, error) {
	return fs.Stat(f.m, name)
}

// Readlink returns fs.ErrInvalid for every path, because
// fstest.MapFS does not represent symlinks.
func (f *MemFS) Readlink(name string) (string, error) {
	return "", &fs.PathError{Op: "readlink", Path: name, Err: fs.ErrInvalid}
}

// Exists reports whether name exists. It returns (false, nil) for
// a missing entry and propagates any other error from Stat.
func (f *MemFS) Exists(name string) (bool, error) {
	_, err := fs.Stat(f.m, name)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, err
}

// List returns the entries in the named directory, sorted by name.
func (f *MemFS) List(name string) ([]fs.DirEntry, error) {
	entries, err := fs.ReadDir(f.m, name)
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	return entries, nil
}

// Write stores content at name. It creates parent directories as
// needed, matching the OSFS behaviour.
func (f *MemFS) Write(name string, content []byte, mode fs.FileMode) error {
	if dir := path.Dir(name); dir != "." && dir != "/" {
		if err := f.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	f.m[name] = &fstest.MapFile{Data: content, Mode: mode}
	return nil
}

// WriteFileExclusive stores content at name, failing if name
// already exists.
//
// # Semantics
//
// The name must not already be present in the underlying map. If
// it is, the operation returns an error wrapping fs.ErrExist.
//
// Parent directories are created as needed, matching Write. The
// parent creation is not part of the exclusivity check: only the
// final name is checked.
//
// # What is not modelled
//
// There is no fsync; an in-memory filesystem has no durability
// concept. The FS interface's WriteFileExclusive promises an
// fsync on a real filesystem; the in-memory adapter is honest
// about the difference in the doc comment, and callers that
// exercise the durability path must use OSFS.
//
// There is no symlink-safe open; fstest.MapFS does not represent
// symlinks. A name whose final component would be a symlink on
// a real filesystem is a regular entry in the map.
func (f *MemFS) WriteFileExclusive(name string, content []byte, mode fs.FileMode) error {
	if _, exists := f.m[name]; exists {
		return &fs.PathError{Op: "open", Path: name, Err: fs.ErrExist}
	}
	if dir := path.Dir(name); dir != "." && dir != "/" {
		if err := f.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	f.m[name] = &fstest.MapFile{Data: content, Mode: mode}
	return nil
}

// Rename moves the entry from oldPath to newPath.
//
// # Semantics
//
// The source must exist. The target is replaced if it exists,
// matching POSIX rename(2) and Windows MoveFileEx.
//
// The rename is atomic within the map: a caller that reads
// newPath after Rename returns sees the source's content. The
// map is modified under a single assignment.
//
// # Errors
//
//   - The source does not exist: error wraps fs.ErrNotExist.
//
//   - The source and target are the same path: no-op.
//
// # What is not modelled
//
// The cross-filesystem (EXDEV) error is not modelled: an
// in-memory filesystem has a single namespace.
func (f *MemFS) Rename(oldPath, newPath string) error {
	if oldPath == newPath {
		return nil
	}
	src, ok := f.m[oldPath]
	if !ok {
		return &fs.PathError{Op: "rename", Path: oldPath, Err: fs.ErrNotExist}
	}
	f.m[newPath] = src
	delete(f.m, oldPath)
	return nil
}

// FsyncDir is a no-op on an in-memory filesystem.
//
// # Why a no-op
//
// The FS interface's FsyncDir makes the directory entry change
// (a rename, an unlink, a create) durable across a power
// failure. An in-memory filesystem is lost entirely across a
// power failure, so there is no "directory entry change" to
// make durable. The operation is a no-op.
//
// The method exists so that MemFS satisfies the FS interface
// and can be passed to safefs.WithFS. A test that exercises
// the durability path must use OSFS.
func (f *MemFS) FsyncDir(name string) error {
	return nil
}

// MkdirAll stores an explicit directory entry for name.
// fstest.MapFS treats directories as implicit: a file at "a/b.txt"
// implies "a" exists as a directory. Storing an explicit entry
// lets an empty directory exist.
func (f *MemFS) MkdirAll(name string, mode fs.FileMode) error {
	if name == "" || name == "." || name == "/" {
		return nil
	}
	if _, ok := f.m[name]; ok {
		return nil
	}
	f.m[name] = &fstest.MapFile{Mode: mode | fs.ModeDir}
	return nil
}

// Remove deletes the entry at name. Removing a directory that has
// children is an error, matching os.Remove.
func (f *MemFS) Remove(name string) error {
	if _, ok := f.m[name]; !ok {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrNotExist}
	}
	prefix := strings.TrimSuffix(name, "/") + "/"
	for k := range f.m {
		if strings.HasPrefix(k, prefix) {
			return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrInvalid}
		}
	}
	delete(f.m, name)
	return nil
}

// Compile-time conformance assertion. MemFS must satisfy the FS
// interface; the assertion fails the build if a future change
// to the interface is not reflected here.
var _ FS = (*MemFS)(nil)
