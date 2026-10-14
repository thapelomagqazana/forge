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
// This package is NOT Forge's filesystem security boundary. It
// contains no security policy: no symlink policy, no path-traversal
// rejection, no root-confinement invariant. Those live one layer up,
// in internal/filesystem/safefs, which is the only package domain
// code is permitted to import.
//
// The layering is:
//
//	domain packages  →  safefs  →  filesystem  →  os / io/fs
//
// Import direction is one-way. This package MUST NOT import safefs
// or any domain package. The rule is enforced by
// scripts/deps/verify-fs-layering.sh (ADR-035).
//
// # The only package that holds *os.File
//
// OSFS, defined in osfs.go, is the sole type in the module — apart
// from the process entrypoint in cmd/forge — that holds an
// *os.File. Every other package reaches the disk through this
// interface. This is what makes filesystem policy auditable: there
// is exactly one place to enumerate it, and it is not here.
//
// # Interface stability
//
// The FS interface is deliberately narrow. The methods are:
//
//   - Read, Write, Exists, MkdirAll, Remove, List, Stat, Boundary
//     (Phase 2)
//   - Open, Lstat, Readlink (added by WBS 13.1.1 to support the
//     safefs symlink policy)
//   - WriteFileExclusive, Rename, FsyncDir (added by WBS 13.5.2
//     to support safefs's atomic-write primitive)
//
// This set is stable. Later work may add methods, but it will not
// remove or rename the existing ones. Adding a method is a
// breaking change to every implementation; the interface therefore
// grows only when a command genuinely needs a new operation, not
// speculatively.
//
// # Boundary
//
// Every FS implementation is bound to a "target directory". The
// Boundary method returns that directory. Every path passed to a
// method is resolved relative to the boundary.
//
// Layer 1 does NOT enforce the boundary. The boundary is the
// anchor that safefs.New consumes: safefs converts it to an
// absolute, cleaned path, validates it once, and from then on
// rejects any name that would escape it. Boundary() itself is a
// pure accessor and performs no I/O.
//
// # Errors
//
// Implementations return errors that preserve the underlying
// operating-system error. Callers may inspect the error with
// errors.Is against standard sentinels (fs.ErrNotExist,
// fs.ErrPermission) or unwrap it to the os package's error types.
//
// Implementations MUST wrap the underlying error with fmt.Errorf's
// %w verb (or an equivalent that supports errors.Is / errors.As).
// Returning a bare error string breaks the error taxonomy:
// safefs maps fs.ErrNotExist, fs.ErrPermission, and friends onto
// forgeerr categories (ADR-011, ADR-013), and it can only do so if
// the sentinel survives wrapping.
//
// # Concurrency
//
// The interface does not require implementations to be safe for
// concurrent use. Callers that need concurrency must synchronise
// their own access.
//
// In practice, Forge commands are sequential. The concurrency
// guarantee that callers rely on — that a *safefs.Root is safe for
// concurrent use — is provided by safefs itself, which serialises
// access to its underlying FS. Layer 1 makes no such promise, and
// a future implementation is free to assume single-threaded use.
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
// # Policy
//
// An FS implementation applies no security policy. It does not
// reject "..", it does not follow or refuse to follow symlinks
// based on a caller-supplied policy, and it does not bound
// directory walks. Those are responsibilities of
// internal/filesystem/safefs, which is the only package domain
// code is permitted to import (ADR-035).
//
// A path passed to an FS method is trusted by construction: it has
// already been validated by safefs, or it was constructed by the
// implementation itself. Calling FS directly from a domain package
// is forbidden by the layering lint.
//
// # Concurrency
//
// The interface does not require implementations to be safe for
// concurrent use. Callers that need concurrency must synchronise
// their own access. In practice, Forge commands are sequential; the
// interface is designed for that case. safefs.Root, which wraps
// this interface, does provide a concurrency guarantee — it
// achieves this by serialising calls, not by requiring it here.
//
// # Errors
//
// Implementations return errors that preserve the underlying
// operating-system error. Callers may inspect the error with
// errors.Is against standard sentinels (fs.ErrNotExist,
// fs.ErrPermission) or unwrap it to the os package's error types.
//
// Implementations MUST wrap the underlying error so that
// errors.Is / errors.As continue to work through the chain. Use
// fmt.Errorf("...: %w", err) or equivalent.
type FS interface {
	// Read returns the content of the file at path.
	//
	// The path is resolved relative to the implementation's
	// boundary. If the file does not exist, the error wraps
	// fs.ErrNotExist.
	//
	// Read is not required to be atomic with respect to
	// concurrent writers. Callers that need a consistent
	// snapshot must coordinate externally.
	Read(path string) ([]byte, error)

	// Write writes content to the file at path, creating it if it
	// does not exist.
	//
	// The mode is applied when the file is created. If the file
	// already exists, the mode is not changed.
	//
	// The path is resolved relative to the implementation's
	// boundary.
	//
	// Write is not atomic: a partial write leaves a partially
	// written file. Callers that need atomic replacement use
	// safefs.Root.WriteAtomic, which composes WriteFileExclusive,
	// Rename, and FsyncDir into an atomic write.
	Write(path string, content []byte, mode fs.FileMode) error

	// Open opens the file at path for reading.
	//
	// The path is resolved relative to the implementation's
	// boundary. If the file does not exist, the error wraps
	// fs.ErrNotExist.
	//
	// Open returns an io/fs.File, not an *os.File. Callers cannot
	// type-assert to reach the underlying descriptor; this is a
	// property of the abstraction, not an accident. The caller
	// must Close the returned file.
	//
	// Open follows symlinks per the implementation's platform
	// semantics. Policy-aware symlink handling lives in safefs,
	// which calls Open only after it has resolved the name to a
	// path it has decided is safe.
	//
	// Introduced by WBS 13.1.1 for safefs.Root.Open.
	Open(path string) (fs.File, error)

	// Exists reports whether a file or directory exists at path.
	//
	// The path is resolved relative to the implementation's
	// boundary. A permission error is returned as an error, not
	// as "false": the caller must be able to distinguish "does
	// not exist" from "cannot tell".
	Exists(path string) (bool, error)

	// MkdirAll creates a directory at path, along with any missing
	// parent directories.
	//
	// The path is resolved relative to the implementation's
	// boundary. If path already exists as a directory, MkdirAll
	// returns nil. If path exists as a non-directory, MkdirAll
	// returns an error wrapping fs.ErrExist.
	MkdirAll(path string, mode fs.FileMode) error

	// Remove deletes the file at path.
	//
	// The path is resolved relative to the implementation's
	// boundary. If the file does not exist, the error wraps
	// fs.ErrNotExist.
	//
	// Remove operates on files only. Removing a non-empty
	// directory is an error; callers that need recursive removal
	// must do so explicitly. This is deliberate: recursive
	// removal is a destructive operation that safefs will gate
	// behind an explicit policy in WBS 13.2.
	Remove(path string) error

	// List returns the entries in the directory at path.
	//
	// The path is resolved relative to the implementation's
	// boundary. Entries are returned in the order the operating
	// system provides them; callers that need a sorted order must
	// sort the result.
	//
	// List does not recurse. Callers that need a recursive walk
	// must use safefs.Root.Walk, which enforces depth and entry
	// bounds (WBS 13.0, Phase 7, Phase 14).
	List(path string) ([]fs.DirEntry, error)

	// Stat returns metadata about the file or directory at path.
	//
	// The path is resolved relative to the implementation's
	// boundary.
	//
	// Stat follows symlinks per the implementation's platform
	// semantics. Callers that need policy-aware symlink handling
	// must use safefs, which is the only package permitted to
	// make that decision (ADR-035).
	Stat(path string) (fs.FileInfo, error)

	// Lstat returns metadata about the file or directory at path
	// without following a terminal symbolic link.
	//
	// The path is resolved relative to the implementation's
	// boundary. When the final component is a symlink, the
	// returned FileInfo has Mode()&fs.ModeSymlink set.
	//
	// Lstat is the primitive safefs uses to detect symlinks
	// during resolution. It is not for general use: a domain
	// package that calls Lstat is making a policy decision that
	// belongs in safefs (ADR-035).
	//
	// Introduced by WBS 13.1.1 for safefs.resolveSymlinks.
	Lstat(path string) (fs.FileInfo, error)

	// Readlink returns the target of the symbolic link at path.
	//
	// The path is resolved relative to the implementation's
	// boundary. The returned string is the link's stored target,
	// not its resolved path: it may be relative, absolute, or
	// contain ".." segments. It is the caller's responsibility
	// to interpret the target in context.
	//
	// If path is not a symlink, the error wraps an
	// implementation-specific sentinel (on POSIX systems,
	// syscall.EINVAL). Callers that need to distinguish "not a
	// link" from other errors should call Lstat first.
	//
	// Readlink is the primitive safefs uses to resolve symlinks
	// under FollowInRoot. Like Lstat, it is not for general use.
	//
	// Introduced by WBS 13.1.1 for safefs.resolveSymlinks.
	Readlink(path string) (string, error)

	// WriteFileExclusive creates the file at path, writes content
	// to it, and fsyncs it before returning.
	//
	// # Semantics
	//
	// The file is created with exclusive semantics: the call
	// fails if the file already exists. The failure wraps
	// fs.ErrExist.
	//
	// The file is created with symlink-safe semantics: the call
	// fails if the final component of path is a symlink. On Unix
	// this is achieved with O_NOFOLLOW; on Windows with
	// FILE_FLAG_OPEN_REPARSE_POINT. The failure wraps fs.ErrExist
	// on Unix (the kernel reports ELOOP, which the implementation
	// normalises) and a platform-specific error on Windows.
	//
	// The fsync is performed before the file is closed. Without
	// it, the write is atomic but not durable: a power failure
	// between the write and the next directory flush could lose
	// the data. The fsync is what makes the write survive the
	// failure.
	//
	// # Why the interface exposes it
	//
	// safefs's atomic-write primitive needs an exclusive,
	// symlink-safe create. The standard os.OpenFile does not
	// expose the platform-specific flags (O_NOFOLLOW on Unix,
	// FILE_FLAG_OPEN_REPARSE_POINT on Windows) through a
	// portable API. Exposing the primitive through the FS
	// interface lets OSFS be the only place that knows the
	// flags, and lets safefs stay free of os.
	//
	// Introduced by WBS 13.5.2 for safefs.Root.WriteAtomic.
	WriteFileExclusive(path string, content []byte, mode fs.FileMode) error

	// Rename renames the file at oldPath to newPath.
	//
	// # Semantics
	//
	// On Unix, the rename is atomic (rename(2)). On Windows, it
	// is implemented with MoveFileEx. The target is replaced if
	// it exists; the caller enforces the fail-closed policy by
	// checking for the target's existence before the rename.
	//
	// # Errors
	//
	//   - The source does not exist: error wraps fs.ErrNotExist.
	//
	//   - The target is on a different filesystem: EXDEV on
	//     Unix. The primitive does not attempt a cross-device
	//     copy; the caller is responsible for keeping the temp
	//     file and the target on the same filesystem.
	//
	//   - Permission: error wraps fs.ErrPermission.
	//
	// # Why the interface exposes it
	//
	// safefs's atomic-write primitive uses the rename as its
	// atomic step. The primitive is a single syscall on every
	// supported platform, but the syscall is different on Unix
	// and Windows. Exposing it through the FS interface hides
	// the difference.
	//
	// Introduced by WBS 13.5.2 for safefs.Root.WriteAtomic.
	Rename(oldPath, newPath string) error

	// FsyncDir fsyncs the directory containing name.
	//
	// # Semantics
	//
	// On Unix, the directory is opened and fsynced. The fsync
	// makes the directory entry change (a rename, an unlink, a
	// create) durable across a power failure.
	//
	// On Windows, the operation is a no-op. Windows does not
	// have a separate directory fsync; the durability of a
	// rename is provided by MoveFileEx's MOVEFILE_WRITE_THROUGH
	// flag, which OSFS passes when it can.
	//
	// # Why the interface exposes it
	//
	// safefs's atomic-write primitive needs a directory fsync
	// after the rename to make the directory entry change
	// durable. Exposing it through the interface keeps the
	// operation portable: on Unix it does work, on Windows it
	// is a no-op.
	//
	// Introduced by WBS 13.5.2 for safefs.Root.WriteAtomic.
	FsyncDir(name string) error

	// Boundary returns the directory against which all paths are
	// resolved. The value is informational at this layer: callers
	// may display it, and tests may assert on it.
	//
	// Boundary performs no I/O and never returns an error. It is
	// the anchor that safefs.New consumes when constructing a
	// policy-enforcing Root.
	Boundary() string
}

// Compile-time conformance assertion. Every FS implementation in
// the module must satisfy the interface. The blank assignment fails
// the build if a future change to FS silently breaks a
// implementation that the compiler would otherwise only catch at a
// distant call site.
//
// OSFS is declared in osfs.go; the assertion lives here so that
// reviewers see the contract and its implementation side by side.
var _ FS = (*OSFS)(nil)
