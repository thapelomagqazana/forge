package filesystem

// This file is an index of the FS interface, not a definition.
// Every method named below is declared on the interface in
// filesystem.go and implemented by OSFS in osfs.go. This file
// exists so that a reader can audit the entire Layer 1 surface
// — every operation, its purpose, its consumer, and its boundary
// behaviour — without reading four files.
//
// # Purpose
//
// The WBS 13.1.2 acceptance criterion "no operation exists just
// in case" is enforced by this file: every method below names a
// WBS item that consumes it, and a method whose row here lacks a
// consumer is a method that should be removed.
//
// # What this file does NOT do
//
//   - It does not declare the methods. The interface declaration
//     in filesystem.go is authoritative; this file is a summary.
//   - It does not change the interface. A future addition to FS
//     adds a row here in the same commit as the method and its
//     implementation, and the row must name a consumer.
//   - It does not enforce policy. Layer 1 is policy-free
//     (ADR-035); enforcement is a Layer 2 concern.

// FS — Layer 1 operations.
//
// The FS interface declares the following operations. The
// "Consumer" annotation on each is a WBS item; a reader auditing
// the surface can grep this file for the WBS item and find the
// call site it motivates.
//
// Read(path) ([]byte, error)
//   Reads a whole file into memory.
//   Boundary: path is joined with Boundary() and handed to os.
//   Missing: error wraps fs.ErrNotExist.
//   Consumer: WBS 13.2.1 (blueprint loader).
//
// Open(path) (fs.File, error)
//   Opens a file for streaming reads. The returned value is an
//   *osFileHandle, not an *os.File; a caller cannot type-assert
//   to reach the descriptor (see osfs.go).
//   Boundary: same as Read.
//   Missing: error wraps fs.ErrNotExist.
//   Consumer: WBS 13.2.1 (template loader), WBS 13.5.1 (scanner).
//
// Write(path, data, mode) error
//   Writes data to path, creating parents as needed.
//   Boundary: same as Read. Parent mode is 0o755 regardless of
//   the file mode.
//   Exists as file: mode is preserved; content is overwritten.
//   Exists as directory: error wraps fs.ErrExist.
//   Consumer: WBS 13.2.3 (renderer output).
//
// Exists(path) (bool, error)
//   Reports whether path exists.
//   Boundary: same as Read.
//   Missing: returns (false, nil). A permission error is an
//   error, not a "false".
//   Consumer: WBS 13.6.1 (drift detection).
//
// MkdirAll(path, mode) error
//   Creates path and any missing parents.
//   Boundary: same as Read.
//   Exists as directory: returns nil (idempotent).
//   Exists as non-directory: error wraps fs.ErrExist.
//   Consumer: WBS 13.2.3 (renderer scaffold).
//
// Remove(path) error
//   Removes a file or an empty directory.
//   Boundary: same as Read.
//   Missing: error wraps fs.ErrNotExist.
//   Non-empty directory: error (ENOTEMPTY on POSIX).
//   Consumer: WBS 13.2.3 (cleanup), WBS 13.5.2 (atomic-write
//   temp-file cleanup).
//
// List(path) ([]fs.DirEntry, error)
//   Lists the immediate children of path, sorted by name.
//   Boundary: same as Read.
//   Missing: error wraps fs.ErrNotExist.
//   Does not recurse; use safefs.Root.Walk for recursion.
//   Consumer: WBS 13.5.1 (scan), WBS 13.6.1 (drift).
//
// Stat(path) (fs.FileInfo, error)
//   Returns metadata, following a terminal symlink.
//   Boundary: same as Read.
//   Missing: error wraps fs.ErrNotExist.
//   Consumer: WBS 13.6.1 (drift mtime compare).
//
// Lstat(path) (fs.FileInfo, error)
//   Like Stat, but does not follow a terminal symlink.
//   Consumer: safefs.resolveSymlinks (WBS 13.2.2).
//
// Readlink(path) (string, error)
//   Returns a symlink's stored target.
//   Consumer: safefs.followInRoot (WBS 13.2.2).
//
// WriteFileExclusive(path, content, mode) error
//   Creates path, writes content, and fsyncs. The creation is
//   exclusive (fails if path exists) and symlink-safe (fails if
//   the final component is a symlink). The symlink-safety is
//   provided by O_NOFOLLOW on Unix and
//   FILE_FLAG_OPEN_REPARSE_POINT on Windows.
//   Boundary: same as Read. The parent directory must exist;
//   the primitive does not create it.
//   Exists: error wraps fs.ErrExist.
//   Final component is a symlink: error wraps fs.ErrExist on
//   Unix (ELOOP is normalised), a platform-specific error on
//   Windows.
//   Consumer: safefs.Root.writeAtomicImpl (WBS 13.5.2).
//
// Rename(oldPath, newPath) error
//   Renames oldPath to newPath, replacing the target if it
//   exists. On Unix this is rename(2); on Windows, MoveFileEx.
//   Boundary: both paths are resolved against the boundary. The
//   rename is atomic within a single filesystem; a cross-device
//   rename returns EXDEV.
//   Missing: error wraps fs.ErrNotExist for a missing source.
//   Consumer: safefs.Root.writeAtomicImpl (WBS 13.5.2).
//
// FsyncDir(name) error
//   Fsyncs the directory containing name. On Unix, this makes
//   the directory entry change (a rename, an unlink, a create)
//   durable across a power failure. On Windows, the operation
//   is a no-op: Windows's durability for a rename is provided
//   by MoveFileEx's MOVEFILE_WRITE_THROUGH flag.
//   Boundary: same as Read.
//   Consumer: safefs.Root.writeAtomicImpl (WBS 13.5.2).
//
// Boundary() string
//   Returns the directory against which paths are resolved.
//   Pure accessor; no I/O.
//   Consumer: safefs.New (WBS 13.1.1).
