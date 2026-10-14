package filesystem

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// OSFS is the production FS implementation, backed by the operating
// system's filesystem.
//
// # Contract
//
// OSFS is policy-free. It does not reject "..", it does not reject
// absolute paths, it does not follow or refuse to follow symlinks
// based on a caller-supplied policy, and it does not bound
// directory walks. Those are the responsibilities of
// internal/filesystem/safefs, which is the only package domain
// code is permitted to import (ADR-035).
//
// OSFS assumes every name passed to it is a cleaned, forward-slash,
// local path relative to its boundary. In production that is
// guaranteed by safefs.validateName. Tests that drive OSFS directly
// must uphold the same invariant.
//
// # The only *os.File holder
//
// OSFS is the single type in the module — apart from the process
// entrypoint in cmd/forge — that holds an *os.File or calls the os
// package for filesystem I/O. This is what makes filesystem policy
// auditable: there is exactly one place where the real disk is
// touched, and it contains no policy. scripts/deps/verify-fs-layering.sh
// enforces the confinement.
//
// # The abstraction boundary
//
// Every value OSFS returns to its caller is either a primitive
// ([]byte, fs.FileInfo, []fs.DirEntry, bool, error) or an
// unexported wrapper. No *os.File and no other OS-level type
// escapes the package. In particular, Open returns an
// *osFileHandle, not an *os.File, so a caller who receives an
// fs.File from a Root cannot type-assert it to reach the
// underlying descriptor. The abstraction is what makes the
// enforcement-by-construction model real (ADR-035).
//
// # Boundary semantics
//
// Boundary() returns the directory against which every path is
// resolved. OSFS treats it as opaque: it may be absolute, relative,
// cleaned, or uncleaned; OSFS joins it with the incoming path and
// hands the result to the os package. safefs.New is responsible for
// converting a user-supplied root into a validated absolute path
// before constructing an OSFS around it.
//
// # Concurrency
//
// OSFS is stateless apart from the immutable boundary field. It is
// therefore safe for concurrent use, but the FS interface does not
// require that and callers must not rely on it. The concurrency
// guarantee that callers actually use — that *safefs.Root is safe
// for concurrent use — is provided by safefs, not here.
type OSFS struct {
	boundary string
}

// NewOSFS returns an FS implementation bound to the given directory.
//
// The directory may or may not exist. Operations that create files
// will create parent directories as needed.
//
// The boundary is stored verbatim; no cleaning or validation is
// performed. Callers that need a validated root must construct it
// through safefs.New, which is the only supported entry point in
// domain code.
func NewOSFS(boundary string) *OSFS {
	return &OSFS{boundary: boundary}
}

// resolve joins the boundary and the path, and prepares the
// result for the operating system.
//
// # The three steps
//
//  1. filepath.Join combines the boundary and the name with the
//     host's separator. It also cleans redundant separators and
//     resolves "." components.
//
//  2. prepareForOS normalises the joined path for the host.
//     On Unix, it is the identity. On Windows, it applies the
//     \\?\ prefix when the path exceeds the long-path
//     threshold, and converts forward slashes to backslashes.
//
//  3. The result is handed to the os package. Every OSFS method
//     that touches the disk goes through this function, so the
//     preparation happens exactly once per operation.
//
// # Why the preparation is not inside filepath.Join
//
// filepath.Join is a pure function on paths; it does not know
// what will be done with its result. A caller might want to
// display the joined path, compare it to another path, or store
// it. Each of those uses has different requirements: display
// wants the human-readable form, comparison wants the canonical
// form, and storage wants the form that will survive a round
// trip. Only the OS call wants the long-path-prefixed form.
// prepareForOS is applied at the point where the OS is about to
// be called, and nowhere else.
//
// # What this function does not do
//
// It does not validate the name. Validation is safefs's job
// (ADR-035). It does not clean the name beyond what
// filepath.Join does, because the name is expected to be
// already clean. It does not check that the boundary is
// absolute, because safefs.New guarantees that. Each of those
// non-responsibilities is a place the abstraction is
// deliberately thin.
func (f *OSFS) resolve(path string) string {
	joined := filepath.Join(f.boundary, path)
	return prepareForOS(joined)
}

// Read returns the content of the file at path.
//
// Errors from os.ReadFile are returned unchanged. They already wrap
// fs.ErrNotExist, fs.ErrPermission, and friends, so callers can
// inspect them with errors.Is. safefs relies on that to map them
// onto forgeerr categories without string matching (ADR-013).
func (f *OSFS) Read(path string) ([]byte, error) {
	return os.ReadFile(f.resolve(path))
}

// Write writes content to the file at path, creating it if it does
// not exist.
//
// The parent directory is created with mode 0o755 if it is missing.
// The file itself is created with the caller-supplied mode; if the
// file already exists, its mode is left unchanged, matching the
// semantics documented on FS.Write.
//
// The parent-directory mode is fixed rather than derived from the
// file mode because directories require the execute bit to be
// traversable; propagating a file's 0o644 to a directory would
// produce an unreadable directory. This is a deliberate separation
// of concerns, not an oversight.
func (f *OSFS) Write(path string, content []byte, mode fs.FileMode) error {
	full := f.resolve(path)

	if dir := filepath.Dir(full); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	return os.WriteFile(full, content, mode)
}

// Open opens the file at path for reading.
//
// The returned value is an io/fs.File whose concrete type is
// unexported and does not expose the underlying *os.File. A
// caller cannot type-assert to reach the descriptor, cannot call
// methods the io/fs.File interface does not declare, and cannot
// use the returned value with any API that requires an *os.File.
// This is what makes the abstraction meaningful: every read
// through a Root goes through a type that the policy layer
// controls.
//
// Open follows symlinks per the operating system's semantics.
// Policy-aware resolution is the caller's responsibility: safefs
// calls Open only with a path it has already decided is safe.
//
// Errors from os.Open are returned unchanged. They wrap
// fs.ErrNotExist, fs.ErrPermission, and friends, so callers can
// inspect them with errors.Is.
//
// The caller must Close the returned file.
func (f *OSFS) Open(path string) (fs.File, error) {
	raw, err := os.Open(f.resolve(path))
	if err != nil {
		return nil, err
	}
	return &osFileHandle{f: raw}, nil
}

// Exists reports whether a file or directory exists at path.
//
// A permission error is returned as an error, not as "false": the
// caller must be able to distinguish "does not exist" from "cannot
// tell". The not-exist predicate uses errors.Is against
// fs.ErrNotExist rather than os.IsNotExist so that it composes with
// wrapped errors produced by higher layers.
func (f *OSFS) Exists(path string) (bool, error) {
	_, err := os.Stat(f.resolve(path))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, err
}

// MkdirAll creates a directory at path, along with any missing
// parents.
//
// The caller-supplied mode is applied to every directory that
// MkdirAll creates. If path already exists as a directory, MkdirAll
// returns nil. If path exists as a non-directory, the error wraps
// fs.ErrExist, matching the interface contract.
//
// # Why this is not a straight passthrough
//
// os.MkdirAll returns the raw errno from the underlying mkdir(2)
// call when the target exists as a non-directory. On Linux and
// most Unixes that is ENOTDIR ("not a directory"), which does not
// satisfy errors.Is(err, fs.ErrExist). The interface contract for
// FS.MkdirAll promises fs.ErrExist for that case, so the errno is
// normalised here.
//
// The check is performed only when os.MkdirAll has already
// returned an error, so the common case (a successful creation)
// is unaffected.
func (f *OSFS) MkdirAll(path string, mode fs.FileMode) error {
	err := os.MkdirAll(f.resolve(path), mode)
	if err == nil {
		return nil
	}
	// The interface contract says a non-directory at path is
	// reported as fs.ErrExist. Detect that case explicitly.
	//
	// The check is: the path exists, and it is not a directory.
	// If both are true, the failure is "the path is occupied by
	// something that is not a directory," which is what
	// fs.ErrExist means in this context. The exact errno the OS
	// produced is not inspected, because it varies by platform.
	//
	// Lstat is used rather than Stat so that a symlink to a
	// directory is treated as a non-directory. Creating a
	// directory where a symlink is should fail; silently
	// succeeding through the link would make MkdirAll behave
	// differently depending on what the link points at.
	if info, statErr := os.Lstat(f.resolve(path)); statErr == nil && !info.IsDir() {
		return &fs.PathError{
			Op:   "mkdir",
			Path: f.resolve(path),
			Err:  fs.ErrExist,
		}
	}
	return err
}

// Remove deletes the file at path.
//
// Removing a non-empty directory is an error, matching os.Remove.
// Callers that need recursive removal must do so explicitly;
// recursive removal is a destructive operation that safefs gates
// behind an explicit policy (WBS 13.2).
func (f *OSFS) Remove(path string) error {
	return os.Remove(f.resolve(path))
}

// List returns the entries in the directory at path.
//
// Entries are returned in the order the operating system provides
// them; os.ReadDir is documented to sort by filename, and OSFS
// inherits that order. Callers must not rely on any other ordering.
//
// List does not recurse. Callers that need a recursive walk must
// use safefs.Root.Walk, which enforces depth and entry bounds.
func (f *OSFS) List(path string) ([]fs.DirEntry, error) {
	return os.ReadDir(f.resolve(path))
}

// Stat returns metadata about the file or directory at path.
//
// Stat follows symlinks per the host platform's semantics. Policy-
// aware symlink handling lives in safefs; OSFS does not inspect the
// target's location relative to the boundary.
//
// Stat is the primitive that answers "what is at this path after
// resolving links." Callers that need to know whether the final
// component is itself a link must use Lstat.
func (f *OSFS) Stat(path string) (fs.FileInfo, error) {
	return os.Stat(f.resolve(path))
}

// Lstat returns metadata about the file or directory at path
// without following a terminal symbolic link.
//
// The distinction from Stat is confined to the *final* component
// of the path: if the last element is a symlink, Lstat returns
// metadata about the link itself (Mode()&fs.ModeSymlink is set),
// while Stat returns metadata about the link's target. Intermediate
// components are followed by both, per the operating system's
// semantics.
//
// safefs uses Lstat to detect symlinks during resolution. It calls
// Lstat on each prefix of a name, one component at a time, so that
// intermediate links are detected even though Lstat itself would
// resolve them: by the time Lstat("a/b") is called, "a" has already
// been confirmed to be a non-symlink by Lstat("a"), and the only
// remaining question is whether "b" is a link.
//
// Lstat is not for general use. A domain package that calls Lstat
// is making a policy decision that belongs in safefs (ADR-035).
//
// Errors from os.Lstat are returned unchanged, so callers can use
// errors.Is against fs.ErrNotExist and friends.
func (f *OSFS) Lstat(path string) (fs.FileInfo, error) {
	return os.Lstat(f.resolve(path))
}

// Readlink returns the target of the symbolic link at path.
//
// The returned string is the link's stored target, not its
// resolved path. It may be relative, absolute, or contain ".."
// segments; the caller interprets it in context. safefs interprets
// relative targets against the link's parent directory and rejects
// absolute targets outright, because an absolute target cannot be
// inside a root that is itself a subtree (see symlink.go).
//
// If path is not a symlink, the returned error is the one the
// operating system produces (on POSIX, this is typically
// syscall.EINVAL). safefs calls Readlink only after Lstat has
// confirmed the path is a link, so this branch is unreachable in
// practice; it exists for callers that reach Readlink by other
// means and for the error to be honest about what happened.
//
// Readlink is not for general use. Like Lstat, it is the primitive
// safefs uses to resolve links under FollowInRoot (ADR-035).
func (f *OSFS) Readlink(path string) (string, error) {
	return os.Readlink(f.resolve(path))
}

// WriteFileExclusive creates the file at path, writes content, and
// fsyncs.
//
// The platform-specific flags and the fsync are provided by
// createExclusive, which is build-tagged. This method is a thin
// wrapper that joins the path with the boundary and delegates.
//
// # Errors
//
//   - The file exists: error wraps fs.ErrExist.
//
//   - The final component is a symlink: on Unix, ELOOP is
//     normalised to fs.ErrExist; on Windows, a platform-specific
//     error is returned.
//
//   - The parent directory does not exist: error wraps
//     fs.ErrNotExist.
//
// Introduced by WBS 13.5.2 for safefs.Root.WriteAtomic.
func (f *OSFS) WriteFileExclusive(path string, content []byte, mode fs.FileMode) error {
	return f.createExclusive(f.resolve(path), content, mode)
}

// Rename renames oldPath to newPath.
//
// On Unix this is rename(2). On Windows it is MoveFileEx, which
// os.Rename wraps. The target is replaced if it exists.
//
// Introduced by WBS 13.5.2 for safefs.Root.WriteAtomic.
func (f *OSFS) Rename(oldPath, newPath string) error {
	return os.Rename(f.resolve(oldPath), f.resolve(newPath))
}

// FsyncDir fsyncs the directory containing name.
//
// On Unix this opens the directory and calls Sync. On Windows it
// is a no-op. The platform-specific behaviour is provided by
// fsyncDirImpl, which is build-tagged.
//
// Introduced by WBS 13.5.2 for safefs.Root.WriteAtomic.
func (f *OSFS) FsyncDir(name string) error {
	return f.fsyncDirImpl(f.resolve(name))
}

// Boundary returns the directory against which paths are resolved.
//
// The value is the exact string passed to NewOSFS. It performs no
// I/O and never returns an error. safefs.New consumes it when
// constructing a policy-enforcing Root.
func (f *OSFS) Boundary() string {
	return f.boundary
}

// -----------------------------------------------------------------------------
// osFileHandle
// -----------------------------------------------------------------------------

// osFileHandle is the unexported wrapper that OSFS.Open returns.
//
// # Why a wrapper
//
// os.Open returns an *os.File. Returning that value directly
// through the fs.File interface would satisfy the signature but
// not the abstraction: a caller could type-assert the returned
// value to *os.File and reach the underlying descriptor, bypassing
// any policy layer that had trusted the fs.File interface.
//
// osFileHandle wraps the *os.File and exposes only the methods of
// io/fs.File. The concrete type is unexported, the field is
// unexported, and no method on osFileHandle returns the *os.File.
// A caller who receives an osFileHandle cannot reach the
// descriptor it wraps, except by reflection, which is out of
// scope for the abstraction's threat model.
//
// # What osFileHandle deliberately does not do
//
// It does not implement io.ReaderAt, io.Seeker, io.WriterTo, or
// any of the optional interfaces that *os.File implements. Those
// are not part of io/fs.File, and exposing them would give callers
// capabilities the policy layer did not intend. If a future
// operation needs one of them, it is added to fs.File (which is
// a significant change) or to the filesystem.FS interface (which
// is the right layer for it), not exposed on the wrapper.
//
// # Threading
//
// osFileHandle delegates every method to the underlying *os.File.
// Whether the wrapper is safe for concurrent use is therefore
// determined by the *os.File, whose behaviour is documented by
// the os package. safefs does not rely on concurrent use of a
// single file handle; callers that need it must obtain that
// guarantee from os.
type osFileHandle struct {
	f *os.File
}

// Stat returns metadata about the file the handle refers to.
//
// The returned FileInfo is the one the operating system produces.
// It describes the file the caller opened, which under safefs is
// a file the policy layer already approved.
func (h *osFileHandle) Stat() (fs.FileInfo, error) {
	return h.f.Stat()
}

// Read reads up to len(p) bytes from the file into p.
//
// The semantics are those of os.File.Read, which are those of
// io.Reader.
func (h *osFileHandle) Read(p []byte) (int, error) {
	return h.f.Read(p)
}

// Close releases the underlying file descriptor.
//
// Close is not idempotent at the wrapper level: calling it twice
// returns the error the underlying *os.File would return on the
// second call, which is a platform-specific "file already closed"
// error. This matches the io/fs.File contract, which does not
// promise idempotence.
func (h *osFileHandle) Close() error {
	return h.f.Close()
}

// Compile-time assertions.
//
// OSFS must satisfy FS, and osFileHandle must satisfy io/fs.File.
// The first is the package's contract; the second is the
// abstraction boundary that Open promises to uphold.
var (
	_ fs.File = (*osFileHandle)(nil)
)
