// Package filesystem defines the Forge filesystem abstraction.
//
// # What this package is
//
// This package defines the FS interface — the contract between the
// application layer and the operating system's filesystem. Every
// filesystem operation in Forge flows through this interface, which
// allows tests to substitute an in-memory implementation and allows
// the security model to enforce boundaries in one place.
//
// # What this package is not
//
// This package is not the complete filesystem abstraction. The
// complete abstraction — including path normalization, symlink
// resolution, atomic writes, and boundary enforcement — arrives in
// WBS 13.0. This package provides the minimum interface that the
// Dependencies struct requires.
//
// # Interface stability
//
// The FS interface is deliberately narrow. The methods are:
//
//   - Read, Write, Exists, MkdirAll, Remove, List, Stat, Boundary.
//
// This set is stable. WBS 13.0 may add methods (e.g., for atomic
// writes or symlink resolution), but it will not remove or rename
// the existing ones. Adding a method is a breaking change to every
// implementation; the interface is therefore designed to grow only
// when a command genuinely needs a new operation.
//
// # Boundary enforcement
//
// Every FS implementation is bound to a "target directory". The
// Boundary method returns that directory. Every path passed to a
// method is resolved relative to the boundary; a path that escapes
// the boundary produces an error rather than reaching the operating
// system.
//
// In Phase 2, the boundary is informational. It is recorded so that
// callers can display it and so that tests can assert on it. Full
// enforcement arrives in WBS 13.0.
package filesystem

import (
	"io/fs"
)

// FS is the filesystem abstraction used by Forge.
//
// Every method takes a path relative to the implementation's
// boundary. Implementations resolve the path against the boundary
// before performing the operation.
//
// # Concurrency
//
// The interface does not require implementations to be safe for
// concurrent use. Callers that need concurrency must synchronise
// their own access. In practice, Forge commands are sequential; the
// interface is designed for that case.
//
// # Errors
//
// Implementations return errors that preserve the underlying
// operating-system error. Callers may inspect the error with
// errors.Is against standard sentinels (fs.ErrNotExist,
// fs.ErrPermission) or unwrap it to the os package's error types.
type FS interface {
	// Read returns the content of the file at path.
	//
	// The path is resolved relative to the implementation's
	// boundary. If the file does not exist, the error wraps
	// fs.ErrNotExist.
	Read(path string) ([]byte, error)

	// Write writes content to the file at path, creating it if it
	// does not exist.
	//
	// The mode is applied when the file is created. If the file
	// already exists, the mode is not changed.
	//
	// The path is resolved relative to the implementation's
	// boundary.
	Write(path string, content []byte, mode fs.FileMode) error

	// Exists reports whether a file or directory exists at path.
	//
	// The path is resolved relative to the implementation's
	// boundary. A permission error is returned as an error, not
	// as "false".
	Exists(path string) (bool, error)

	// MkdirAll creates a directory at path, along with any missing
	// parent directories.
	//
	// The path is resolved relative to the implementation's
	// boundary.
	MkdirAll(path string, mode fs.FileMode) error

	// Remove deletes the file at path.
	//
	// The path is resolved relative to the implementation's
	// boundary. If the file does not exist, the error wraps
	// fs.ErrNotExist.
	Remove(path string) error

	// List returns the entries in the directory at path.
	//
	// The path is resolved relative to the implementation's
	// boundary. Entries are returned in the order the operating
	// system provides them; callers that need a sorted order must
	// sort the result.
	List(path string) ([]fs.DirEntry, error)

	// Stat returns metadata about the file or directory at path.
	//
	// The path is resolved relative to the implementation's
	// boundary.
	Stat(path string) (fs.FileInfo, error)

	// Boundary returns the directory against which all paths are
	// resolved. The value is informational: callers may display it
	// and tests may assert on it.
	Boundary() string
}
