package safefs

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"github.com/thapelomagqazana/forge/internal/forgeerr"
)

// This file defines the sentinel errors that safefs returns and the
// small helpers that attach context to them.
//
// # Design
//
// safefs errors are sentinel values created with forgeerr.New. They
// carry a stable forgeerr code so the CLI can map them to exit
// codes without string matching (ADR-011, ADR-012). The sentinels
// are exported so domain packages can use errors.Is to branch on
// them without importing forgeerr directly.
//
// Every error that safefs returns is a wrapping of one of the
// sentinels below, augmented with four fields that the caller can
// rely on:
//
//   - operation — the name of the Root or Handle method that
//     produced the error. Attached as context under the key
//     "operation" by classify.
//
//   - path — the cleaned relative name the operation acted on.
//     Attached as context under the key "path" by classify.
//
//   - cause — the underlying error, if any. Attached via the %w
//     verb so errors.Is and errors.As find it.
//
//   - remediation — a single-line hint the CLI can display to
//     help the user fix the problem. Attached by classify.
//
// The four fields are the diagnostic contract. See
// docs/reference/filesystem-errors.md.
//
// # The sentinels
//
// The sentinels divide into two families:
//
//   - Boundary sentinels — ErrNotLocal, ErrPathTraversal,
//     ErrSymlinkDenied, ErrDepthExceeded, ErrEntriesExceeded,
//     ErrCaseCollision, ErrSubBoundary, ErrDenyList,
//     ErrNotAllowed, ErrInvalidPattern. These are produced by the
//     pipeline (pipeline.go) before the operation delegates to
//     the underlying FS.
//
//   - OS sentinels — ErrNotFound, ErrExists, ErrPermissionDenied,
//     ErrReadOnly, ErrIsDirectory, ErrNotDirectory,
//     ErrSymlinkLoop, ErrReservedName, ErrPathTooLong,
//     ErrNameCollision. These are produced by classify when the
//     underlying FS returns an OS-level error.
//
// The two families have the same shape (sentinel + four fields)
// and the same stability guarantee. The split is documentation:
// a reader who wants to know "what can the pipeline produce"
// reads the first list; a reader who wants to know "what can the
// OS produce" reads the second.
//
// # Code mapping
//
// The mapping from a safefs sentinel to a forgeerr code is chosen
// so that the category, and therefore the exit code, matches the
// caller's mental model:
//
//   - Structural name problems (empty, absolute, non-local) and
//     operational bounds (depth, entries, case) map to the
//     FORGE_FILESYSTEM_* family. Exit code 12.
//
//   - Deliberate boundary violations (a name that attempts to
//     traverse outside the root, or a symlink whose target lies
//     outside the root) map to the FORGE_SECURITY_* family. Exit
//     code 13.
//
// The two families are deliberately distinct. See
// docs/architecture/filesystem-layering.md, section "Filesystem
// versus Security".
//
// # Signature
//
// Every sentinel below is constructed with forgeerr.New, whose
// signature is New(category, code, message). The category is
// passed explicitly rather than derived from the code so that a
// mis-categorised code fails at package initialisation time — the
// sentinel panics on load — rather than at the first call.

// -----------------------------------------------------------------------------
// Boundary sentinels
// -----------------------------------------------------------------------------

// ErrNotLocal is returned when a name is not a local, forward-slash,
// cleaned relative path: it is empty, absolute, contains a Windows
// drive letter, contains a UNC prefix, or is otherwise rejected by
// filepath.IsLocal.
//
// ErrNotLocal describes a *structural* problem with the name. When
// the name is well-formed but attempts to traverse outside the
// root, the more specific ErrPathTraversal is returned instead.
//
// Mapping: forgeerr.CodeFilesystemNotLocal.
// Category: CategoryFilesystem. Exit code: ExitFilesystem (12).
var ErrNotLocal = forgeerr.New(
	forgeerr.CategoryFilesystem,
	forgeerr.CodeFilesystemNotLocal,
	"name is not a local relative path",
)

// ErrPathTraversal is returned when a name attempts to traverse
// outside the Root, or otherwise resolves outside it.
//
// ErrPathTraversal is returned by Root.validateName before any I/O
// is attempted. It is therefore the earliest possible detection
// point and the cheapest to test.
//
// Mapping: forgeerr.CodeSecurityPathTraversal.
// Category: CategorySecurity. Exit code: ExitSecurity (13).
var ErrPathTraversal = forgeerr.New(
	forgeerr.CategorySecurity,
	forgeerr.CodeSecurityPathTraversal,
	"path escapes the safefs root",
)

// ErrSymlinkDenied is returned when the symlink policy forbids
// following a link encountered during resolution.
//
// Under NoFollow, ErrSymlinkDenied is returned for any symlink.
// Under FollowInRoot, it is returned when a symlink's resolved
// target escapes the root. In both cases the failure is a
// deliberate boundary violation: the caller asked for a target the
// policy forbids.
//
// Mapping: forgeerr.CodeSecuritySymlinkEscape.
// Category: CategorySecurity. Exit code: ExitSecurity (13).
var ErrSymlinkDenied = forgeerr.New(
	forgeerr.CategorySecurity,
	forgeerr.CodeSecuritySymlinkEscape,
	"symlink denied by policy",
)

// ErrDepthExceeded is returned by Root.Walk when the depth of the
// traversal exceeds Root's MaxDepth. It is a security control, not
// a performance optimisation: a hostile deep tree must not OOM the
// process.
//
// Mapping: forgeerr.CodeFilesystemDepthExceeded.
// Category: CategoryFilesystem. Exit code: ExitFilesystem (12).
var ErrDepthExceeded = forgeerr.New(
	forgeerr.CategoryFilesystem,
	forgeerr.CodeFilesystemDepthExceeded,
	"walk depth limit exceeded",
)

// ErrEntriesExceeded is returned by Root.Walk when the number of
// entries visited exceeds Root's MaxEntries. Like
// ErrDepthExceeded, it is a security control.
//
// Mapping: forgeerr.CodeFilesystemEntriesExceeded.
// Category: CategoryFilesystem. Exit code: ExitFilesystem (12).
var ErrEntriesExceeded = forgeerr.New(
	forgeerr.CategoryFilesystem,
	forgeerr.CodeFilesystemEntriesExceeded,
	"walk entry limit exceeded",
)

// ErrCaseCollision is returned when case-collision detection is
// enabled and two names in the same directory collide under Unicode
// case folding. It is only reachable on case-insensitive
// filesystems; on case-sensitive filesystems the check is a no-op.
//
// Mapping: forgeerr.CodeFilesystemCaseCollision.
// Category: CategoryFilesystem. Exit code: ExitFilesystem (12).
var ErrCaseCollision = forgeerr.New(
	forgeerr.CategoryFilesystem,
	forgeerr.CodeFilesystemCaseCollision,
	"case-insensitive name collision",
)

// ErrSubBoundary is returned when a Handle operation names a
// path that resolves outside the handle's sub-boundary.
//
// # Why a separate sentinel
//
// A Handle's sub-boundary is a stricter constraint than the
// root boundary. A name that escapes the sub-boundary is
// rejected even if it would be legal under the parent Root, so
// the failure is not a root-boundary violation. It is a
// Handle-specific failure and deserves its own sentinel for two
// reasons:
//
//  1. A caller that branches on errors.Is(err, ErrPathTraversal)
//     would not match a sub-boundary violation, because the
//     name itself is legal.
//
//  2. A log line that says "sub-boundary violation" is more
//     actionable than one that says "path traversal", because
//     the fix is different.
//
// Mapping: forgeerr.CodeSecurityPathTraversal.
// Category: CategorySecurity. Exit code: ExitSecurity (13).
var ErrSubBoundary = forgeerr.New(
	forgeerr.CategorySecurity,
	forgeerr.CodeSecurityPathTraversal,
	"path escapes the handle's sub-boundary",
)

// ErrDenyList is returned when a name matches the Root's
// deny-list. The deny-list is a set of path patterns that reject
// names inside the boundary; a name that matches is refused
// before any filesystem access.
//
// ErrDenyList takes precedence over ErrNotAllowed.
//
// Mapping: forgeerr.CodeSecurityPathTraversal.
// Category: CategorySecurity. Exit code: ExitSecurity (13).
var ErrDenyList = forgeerr.New(
	forgeerr.CategorySecurity,
	forgeerr.CodeSecurityPathTraversal,
	"path matches the root's deny-list",
)

// ErrNotAllowed is returned when a name does not match the
// Root's allow-list. The allow-list is a set of path patterns
// that constrain the Root to a specific set of paths.
//
// Mapping: forgeerr.CodeSecurityPathTraversal.
// Category: CategorySecurity. Exit code: ExitSecurity (13).
var ErrNotAllowed = forgeerr.New(
	forgeerr.CategorySecurity,
	forgeerr.CodeSecurityPathTraversal,
	"path is not on the root's allow-list",
)

// ErrInvalidPattern is returned by New when a deny-list or
// allow-list pattern fails to parse.
//
// Mapping: forgeerr.CodeInternalUnexpected.
// Category: CategoryInternal. Exit code: ExitInternal (70).
var ErrInvalidPattern = forgeerr.New(
	forgeerr.CategoryInternal,
	forgeerr.CodeInternalUnexpected,
	"invalid deny-list or allow-list pattern",
)

// -----------------------------------------------------------------------------
// OS sentinels
// -----------------------------------------------------------------------------
//
// The following sentinels are produced by classify() when the
// underlying FS returns an OS-level error. Each sentinel maps to
// a forgeerr code; classify preserves the underlying error so
// errors.Is against fs.ErrNotExist, fs.ErrPermission, and friends
// continues to work.
//
// See docs/reference/filesystem-errors.md for the cross-platform
// mapping table (POSIX errno → Windows ERROR_* → sentinel).

// ErrNotFound is returned when the named file or directory does
// not exist.
//
// The error wraps fs.ErrNotExist, so a caller that branches on
// the stdlib sentinel continues to work.
//
// Remediation: "Check the path and try again."
//
// Mapping: forgeerr.CodeFilesystemNotFound.
// Category: CategoryFilesystem. Exit code: ExitFilesystem (12).
var ErrNotFound = forgeerr.New(
	forgeerr.CategoryFilesystem,
	forgeerr.CodeFilesystemNotFound,
	"file or directory not found",
)

// ErrExists is returned when the target exists and the operation
// does not permit overwriting.
//
// # Fail-closed default
//
// Every operation that could overwrite an existing file is
// fail-closed by default. A caller that wants to overwrite uses
// a distinct method (WriteOverwrite, RenameOverwrite). No
// operation silently overwrites.
//
// See docs/reference/filesystem-errors.md, "The Existing-File
// Policy Matrix", for the full policy.
//
// Remediation: "Choose a different path, or use the overwrite
// variant of the operation."
//
// Mapping: forgeerr.CodeFilesystemExists.
// Category: CategoryFilesystem. Exit code: ExitFilesystem (12).
var ErrExists = forgeerr.New(
	forgeerr.CategoryFilesystem,
	forgeerr.CodeFilesystemExists,
	"target already exists",
)

// ErrPermissionDenied is returned when the process lacks
// permission to perform the operation.
//
// On POSIX systems, this corresponds to EACCES or EPERM. On
// Windows, it corresponds to ERROR_ACCESS_DENIED.
//
// The error wraps fs.ErrPermission.
//
// Remediation: "Check the file's permissions, or run with
// sufficient privileges."
//
// Mapping: forgeerr.CodeFilesystemPermission.
// Category: CategoryFilesystem. Exit code: ExitFilesystem (12).
var ErrPermissionDenied = forgeerr.New(
	forgeerr.CategoryFilesystem,
	forgeerr.CodeFilesystemPermission,
	"permission denied",
)

// ErrReadOnly is returned when the filesystem is mounted
// read-only, or when the file's attributes forbid writes.
//
// On POSIX systems, this corresponds to EROFS. On Windows, it
// corresponds to ERROR_WRITE_PROTECT.
//
// Remediation: "The filesystem is read-only. Check the mount
// options or the file's attributes."
//
// Mapping: forgeerr.CodeFilesystemReadOnly.
// Category: CategoryFilesystem. Exit code: ExitFilesystem (12).
var ErrReadOnly = forgeerr.New(
	forgeerr.CategoryFilesystem,
	forgeerr.CodeFilesystemReadOnly,
	"filesystem is read-only",
)

// ErrIsDirectory is returned when an operation that expects a
// file finds a directory.
//
// On POSIX systems, this corresponds to EISDIR.
//
// Remediation: "The path names a directory. Use a file path."
//
// Mapping: forgeerr.CodeFilesystemIsDirectory.
// Category: CategoryFilesystem. Exit code: ExitFilesystem (12).
var ErrIsDirectory = forgeerr.New(
	forgeerr.CategoryFilesystem,
	forgeerr.CodeFilesystemIsDirectory,
	"path names a directory",
)

// ErrNotDirectory is returned when an operation that expects a
// directory finds a file.
//
// On POSIX systems, this corresponds to ENOTDIR. On Windows, it
// corresponds to ERROR_DIRECTORY.
//
// Remediation: "The path names a file where a directory was
// expected."
//
// Mapping: forgeerr.CodeFilesystemNotDirectory.
// Category: CategoryFilesystem. Exit code: ExitFilesystem (12).
var ErrNotDirectory = forgeerr.New(
	forgeerr.CategoryFilesystem,
	forgeerr.CodeFilesystemNotDirectory,
	"path names a file where a directory was expected",
)

// ErrSymlinkLoop is returned when symlink resolution exceeds the
// Root's MaxSymlinkChain bound.
//
// On POSIX systems, the operating system returns ELOOP for a
// symlink chain that exceeds its own bound. On Windows, the
// equivalent is ERROR_CANT_RESOLVE_FILENAME.
//
// Remediation: "The symlink chain is too deep. Check for a
// cycle or increase MaxSymlinkChain."
//
// Mapping: forgeerr.CodeFilesystemSymlinkLoop.
// Category: CategoryFilesystem. Exit code: ExitFilesystem (12).
var ErrSymlinkLoop = forgeerr.New(
	forgeerr.CategoryFilesystem,
	forgeerr.CodeFilesystemSymlinkLoop,
	"symlink chain exceeds the maximum depth",
)

// ErrReservedName is returned when a name is reserved by the
// operating system. On Windows, the reserved set is:
//
//	CON, PRN, AUX, NUL,
//	COM1 ... COM9,
//	LPT1 ... LPT9
//
// The reserved set does not exist on POSIX systems, so this
// sentinel is unreachable there.
//
// # Why this sentinel exists separately from ErrNotLocal
//
// filepath.IsLocal rejects reserved names with a false return,
// which the pipeline classifies as ErrNotLocal. The two are
// distinct conditions and the diagnostic should distinguish
// them so a caller can provide a targeted remediation.
//
// Remediation: "The name is reserved by the operating system.
// Choose a different name."
//
// Mapping: forgeerr.CodeFilesystemReservedName.
// Category: CategoryFilesystem. Exit code: ExitFilesystem (12).
var ErrReservedName = forgeerr.New(
	forgeerr.CategoryFilesystem,
	forgeerr.CodeFilesystemReservedName,
	"name is reserved by the operating system",
)

// ErrPathTooLong is returned when a path exceeds the operating
// system's maximum length.
//
// On POSIX systems, this corresponds to ENAMETOOLONG. On
// Windows, the long-path prefix (\\?\) raises the limit but does
// not eliminate it.
//
// Remediation: "The path is too long for this filesystem.
// Shorten it or restructure the tree."
//
// Mapping: forgeerr.CodeFilesystemPathTooLong.
// Category: CategoryFilesystem. Exit code: ExitFilesystem (12).
var ErrPathTooLong = forgeerr.New(
	forgeerr.CategoryFilesystem,
	forgeerr.CodeFilesystemPathTooLong,
	"path exceeds the operating system's maximum length",
)

// ErrNameCollision is returned when two names in the same
// directory collide under Unicode case folding, and the Root's
// case-collision detection is enabled.
//
// The collision is a property of the directory's contents, not
// of the name the caller passed. The error is returned by
// ReadDir after the listing, not by the pipeline.
//
// Remediation: "Two names differ only by case. Rename one of
// them."
//
// Mapping: forgeerr.CodeFilesystemCaseCollision.
// Category: CategoryFilesystem. Exit code: ExitFilesystem (12).
var ErrNameCollision = forgeerr.New(
	forgeerr.CategoryFilesystem,
	forgeerr.CodeFilesystemCaseCollision,
	"two names differ only by case",
)

// -----------------------------------------------------------------------------
// Wrapping helpers
// -----------------------------------------------------------------------------

// pathError wraps a sentinel with the offending name, preserving
// errors.Is identity.
//
// The name is attached as a quoted literal in the message. It is
// safe to log: validateName only passes names that are local
// relative paths, and Root's methods always call validateName
// before pathError, so the name cannot contain an absolute path, a
// home directory, or a drive letter.
//
// Callers must not pass a name that has not been through
// validateName. The contract is checked in tests but not in
// production; a violation would only affect log output, not
// security.
func pathError(sentinel error, name string) error {
	return fmt.Errorf("%w: %q", sentinel, name)
}

// wrapFS attaches an underlying filesystem error to a safefs
// sentinel. Both errors remain discoverable via errors.Is: the
// caller's errors.Is(err, ErrSymlinkDenied) and the caller's
// errors.Is(err, fs.ErrNotExist) both succeed.
//
// wrapFS is used when safefs re-categorises a low-level error
// produced by the FS implementation. For example, a symlink
// resolution that fails because the target does not exist should
// surface as ErrSymlinkDenied *and* preserve fs.ErrNotExist so the
// caller can decide which to trust.
//
// If underlying is nil, wrapFS returns sentinel unchanged. This
// keeps callers free of nil checks at the call site.
func wrapFS(sentinel, underlying error) error {
	if underlying == nil {
		return sentinel
	}
	return fmt.Errorf("%w: %w", sentinel, underlying)
}

// wrapFSName is wrapFS with a name attached. It is the composition
// of pathError and wrapFS and exists so call sites can express the
// common case in one call:
//
//	return wrapFSName(ErrSymlinkDenied, err, name)
//
// produces a message like:
//
//	symlink denied by policy: "link.txt": open: no such file or directory
//
// and preserves errors.Is for both ErrSymlinkDenied and fs.ErrNotExist.
func wrapFSName(sentinel, underlying error, name string) error {
	if underlying == nil {
		return pathError(sentinel, name)
	}
	return fmt.Errorf("%w: %q: %w", sentinel, name, underlying)
}

// -----------------------------------------------------------------------------
// OS-error bridge
// -----------------------------------------------------------------------------

// classify turns an OS-level error into a *forgeerr.Error with the
// appropriate sentinel and the four fields (operation, path,
// cause, remediation).
//
// # Inputs
//
//   - err: the error returned by the underlying FS. It may be a
//     *fs.PathError, a *fs.LinkError, a syscall.Errno, or any
//     error that wraps one of those.
//
//   - op: the name of the Root or Handle method that produced the
//     error. Used as the "operation" context field.
//
//   - path: the cleaned relative name the operation acted on.
//     Used as the "path" context field.
//
// # Output
//
// A *forgeerr.Error wrapping one of the OS sentinels. The
// underlying error is preserved in the chain, so callers can
// still branch on fs.ErrNotExist and friends.
//
// # Why a single function
//
// Every safefs operation that delegates to the underlying FS uses
// classify to convert the result. The function is the single
// point at which the OS's error vocabulary is translated to
// Forge's.
//
// # Order of checks
//
// The checks are ordered from the most specific to the least:
//
//  1. Context cancellation — the error is returned unchanged.
//
//  2. Boundary sentinels — an error that already wraps one of
//     the safefs boundary sentinels is returned unchanged.
//
//  3. The standard sentinels (fs.ErrNotExist, fs.ErrExist,
//     fs.ErrPermission).
//
//  4. The platform-specific errors (reserved names, path too
//     long, symlink loop, read-only, is-directory,
//     not-directory).
//
//  5. Anything else is wrapped as ErrInternal with the underlying
//     cause preserved.
//
// # Concurrency
//
// classify is a pure function; it is safe for concurrent use.
func classify(err error, op, path string) error {
	if err == nil {
		return nil
	}

	// 1. Context cancellation is not a filesystem error.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}

	// 2. Boundary sentinels are passed through.
	for _, s := range []error{
		ErrNotLocal,
		ErrPathTraversal,
		ErrSymlinkDenied,
		ErrDepthExceeded,
		ErrEntriesExceeded,
		ErrCaseCollision,
		ErrSubBoundary,
		ErrDenyList,
		ErrNotAllowed,
		ErrInvalidPattern,
	} {
		if errors.Is(err, s) {
			return err
		}
	}

	// 3–4. Map the standard and platform-specific sentinels.
	var sentinel error
	switch {
	case errors.Is(err, fs.ErrNotExist):
		sentinel = ErrNotFound
	case errors.Is(err, fs.ErrExist):
		sentinel = ErrExists
	case errors.Is(err, fs.ErrPermission):
		sentinel = ErrPermissionDenied
	case isReservedNameError(err):
		sentinel = ErrReservedName
	case isPathTooLongError(err):
		sentinel = ErrPathTooLong
	case isSymlinkLoopError(err):
		sentinel = ErrSymlinkLoop
	case isReadOnlyError(err):
		sentinel = ErrReadOnly
	case isIsDirectoryError(err):
		sentinel = ErrIsDirectory
	case isNotDirectoryError(err):
		sentinel = ErrNotDirectory
	}

	// 5. Fall back to an internal error if no sentinel matched.
	if sentinel == nil {
		return forgeerr.Wrap(
			forgeerr.CategoryInternal,
			forgeerr.CodeInternalUnexpected,
			"filesystem operation failed",
			err,
		).WithContext("operation", op).
			WithContext("path", path)
	}

	// 6. Attach the four fields and return.
	return forgeerr.Wrap(
		sentinelCategory(sentinel),
		sentinelCode(sentinel),
		sentinel.Error(),
		err,
	).WithContext("operation", op).
		WithContext("path", path).
		WithRemediation(remediationOf(sentinel))
}

// sentinelCategory returns the category of an OS sentinel.
//
// The function exists so that classify can construct a
// *forgeerr.Error without hard-coding the category for each
// sentinel. The category is a property of the sentinel and is
// stable; a future addition to the OS sentinel list adds a case
// here and nowhere else.
//
// The default returns CategoryInternal, which is correct for a
// sentinel that classify does not recognise: an unrecognised
// sentinel is a programming error, and the internal category
// names it as such.
func sentinelCategory(s error) forgeerr.Category {
	switch s {
	case ErrNotFound, ErrExists, ErrPermissionDenied, ErrReadOnly,
		ErrIsDirectory, ErrNotDirectory, ErrSymlinkLoop,
		ErrReservedName, ErrPathTooLong, ErrNameCollision:
		return forgeerr.CategoryFilesystem
	default:
		return forgeerr.CategoryInternal
	}
}

// sentinelCode returns the forgeerr code of an OS sentinel.
//
// Like sentinelCategory, this function centralises the mapping so
// that a future addition touches one place.
func sentinelCode(s error) forgeerr.Code {
	switch s {
	case ErrNotFound:
		return forgeerr.CodeFilesystemNotFound
	case ErrExists:
		return forgeerr.CodeFilesystemExists
	case ErrPermissionDenied:
		return forgeerr.CodeFilesystemPermission
	case ErrReadOnly:
		return forgeerr.CodeFilesystemReadOnly
	case ErrIsDirectory:
		return forgeerr.CodeFilesystemIsDirectory
	case ErrNotDirectory:
		return forgeerr.CodeFilesystemNotDirectory
	case ErrSymlinkLoop:
		return forgeerr.CodeFilesystemSymlinkLoop
	case ErrReservedName:
		return forgeerr.CodeFilesystemReservedName
	case ErrPathTooLong:
		return forgeerr.CodeFilesystemPathTooLong
	case ErrNameCollision:
		return forgeerr.CodeFilesystemCaseCollision
	default:
		return forgeerr.CodeInternalUnexpected
	}
}

// remediationOf returns the remediation hint for an OS sentinel.
//
// The hint is a single line, per the forgeerr convention. It is
// displayed by the CLI's error formatter and by --verbose output.
func remediationOf(s error) string {
	switch s {
	case ErrNotFound:
		return "Check the path and try again."
	case ErrExists:
		return "Choose a different path, or use the overwrite variant of the operation."
	case ErrPermissionDenied:
		return "Check the file's permissions, or run with sufficient privileges."
	case ErrReadOnly:
		return "The filesystem is read-only. Check the mount options or the file's attributes."
	case ErrIsDirectory:
		return "The path names a directory. Use a file path."
	case ErrNotDirectory:
		return "The path names a file where a directory was expected."
	case ErrSymlinkLoop:
		return "The symlink chain is too deep. Check for a cycle or increase MaxSymlinkChain."
	case ErrReservedName:
		return "The name is reserved by the operating system. Choose a different name."
	case ErrPathTooLong:
		return "The path is too long for this filesystem. Shorten it or restructure the tree."
	case ErrNameCollision:
		return "Two names differ only by case. Rename one of them."
	default:
		return ""
	}
}

// -----------------------------------------------------------------------------
// Predicates
// -----------------------------------------------------------------------------

// isNotExist reports whether err, or any error it wraps, is
// fs.ErrNotExist.
//
// It exists so that callers inside safefs can branch on "file not
// found" without importing io/fs directly. The layering rule
// permits io/fs — it is a stdlib interface package, not os — but
// keeping the predicate here means a future change to the sentinel
// used for not-found requires editing exactly one file.
func isNotExist(err error) bool {
	return errors.Is(err, fs.ErrNotExist)
}

// isPermission reports whether err, or any error it wraps, is
// fs.ErrPermission. It is the permission counterpart to
// isNotExist.
func isPermission(err error) bool {
	return errors.Is(err, fs.ErrPermission)
}

// isSymlinkDenied reports whether err, or any error it wraps, is
// ErrSymlinkDenied. It is a convenience for internal callers; the
// exported sentinel is available to domain packages directly.
func isSymlinkDenied(err error) bool {
	return errors.Is(err, ErrSymlinkDenied)
}

// isTraversal reports whether err, or any error it wraps, is
// ErrPathTraversal or ErrNotLocal. The two are treated as one
// class by callers that only care "the name was refused," which is
// the common case.
//
// The predicate is named for the security-relevant member of the
// class. A caller that needs to distinguish "malformed name" from
// "traversal attempt" checks the sentinels directly.
func isTraversal(err error) bool {
	return errors.Is(err, ErrPathTraversal) || errors.Is(err, ErrNotLocal)
}
