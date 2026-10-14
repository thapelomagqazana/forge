//go:build windows

package safefs

import (
	"errors"
	"strings"
	"syscall"
)

// This file provides the Windows-specific predicates that
// classify uses to distinguish OS-level error codes that the
// standard library's sentinels do not cover.
//
// # Why the constants are defined locally
//
// The standard library's syscall package does not export the
// Windows ERROR_* constants. It defines syscall.Errno as an
// integer type and populates it from the OS at runtime, but the
// symbolic names live in the Windows SDK's winerror.h and are
// not part of Go's public API.
//
// The alternative is golang.org/x/sys/windows, which exports
// every ERROR_* constant as a named value. This file uses local
// definitions instead, for two reasons:
//
//  1. The repository's dependency policy prefers zero
//     third-party dependencies for the safefs package. The
//     package's other Windows-specific file (symlink_windows.go)
//     already defines its two constants locally, and keeping the
//     two files consistent is the smaller maintenance burden.
//
//  2. golang.org/x/sys's go.mod declares a go directive that is
//     newer than the module's frozen minimum. Importing the
//     package forces the module's go directive up, which the
//     version-pinning checks refuse. The local constants avoid
//     the constraint.
//
// The values below are the DWORD codes that Windows API
// functions return on failure. Each is a stable part of the
// Windows ABI; a future Windows version will not change them.
// They are documented in winerror.h and in the Windows SDK;
// the Go source for the syscall package also references several
// of them.
//
// # What "error code" means on Windows
//
// Windows API functions return a DWORD error code on failure.
// The Go runtime wraps that code in a syscall.Errno, whose
// Error() method formats it as "The filename or extension is
// too long." or a similar message. The predicates below compare
// the error against the local constants, so a caller can write
//
//	if errors.Is(err, errnoFilenameExcedRange) {
//
// instead of comparing against the numeric value 206.

// Windows error codes used by the predicates below. Values from
// the Windows SDK's winerror.h.
const (
	// errnoInvalidName is ERROR_INVALID_NAME (123). Returned
	// when a path contains an invalid character, including a
	// reserved name.
	errnoInvalidName syscall.Errno = 123

	// errnoPathNotFound is ERROR_PATH_NOT_FOUND (3). Returned
	// when a path component does not exist. Also used by some
	// APIs for a path that exceeds the maximum length.
	errnoPathNotFound syscall.Errno = 3

	// errnoFilenameExcedRange is ERROR_FILENAME_EXCED_RANGE
	// (206). Returned when a filename or path exceeds the
	// maximum length the API accepts.
	errnoFilenameExcedRange syscall.Errno = 206

	// errnoCantResolveFilename is ERROR_CANT_RESOLVE_FILENAME
	// (1921). Returned when a symlink chain exceeds the OS's
	// own resolution limit.
	errnoCantResolveFilename syscall.Errno = 1921

	// errnoTooManyLinks is ERROR_TOO_MANY_LINKS (1142).
	// Returned when a symlink chain exceeds the process's
	// link-following limit.
	errnoTooManyLinks syscall.Errno = 1142

	// errnoWriteProtect is ERROR_WRITE_PROTECT (19). Returned
	// when the volume is mounted read-only or the file's
	// attributes forbid writes.
	errnoWriteProtect syscall.Errno = 19

	// errnoDirectory is ERROR_DIRECTORY (267). Returned when a
	// path names a file where a directory was expected.
	errnoDirectory syscall.Errno = 267

	// errnoAccessDenied is ERROR_ACCESS_DENIED (5). Returned
	// when the process lacks permission, and also (on Windows)
	// when an operation targets a directory where a file was
	// expected.
	errnoAccessDenied syscall.Errno = 5
)

// isReservedNameError reports whether err is
// ERROR_INVALID_NAME.
//
// Windows reports reserved names (CON, PRN, AUX, NUL, COM1–9,
// LPT1–9) as ERROR_INVALID_NAME during path parsing. The
// reserved set is enforced by the OS, not by the pipeline; a
// caller that constructs a Root on a Windows volume can trigger
// the error by naming one of the reserved names.
//
// The predicate fires only when the error's code matches. The
// caller is expected to have excluded permission errors, which
// also use ERROR_ACCESS_DENIED, before reaching this predicate.
func isReservedNameError(err error) bool {
	return errors.Is(err, errnoInvalidName)
}

// isPathTooLongError reports whether err is
// ERROR_FILENAME_EXCED_RANGE, or ERROR_PATH_NOT_FOUND with a
// "path too long" message.
//
// The long-path prefix (\\?\) raises the limit but does not
// eliminate it. A path that exceeds the raised limit produces
// ERROR_FILENAME_EXCED_RANGE. Some Win32 APIs, particularly
// older ones, produce ERROR_PATH_NOT_FOUND for the same
// condition; the message is the only signal that distinguishes
// the two uses of the code, so the predicate inspects it.
func isPathTooLongError(err error) bool {
	if errors.Is(err, errnoFilenameExcedRange) {
		return true
	}
	if errors.Is(err, errnoPathNotFound) {
		// Distinguish "not found because too long" from
		// "genuinely missing". The message is the only
		// signal available.
		return strings.Contains(err.Error(), "path too long")
	}
	return false
}

// isSymlinkLoopError reports whether err is
// ERROR_CANT_RESOLVE_FILENAME or ERROR_TOO_MANY_LINKS.
//
// Windows reports a symlink chain that exceeds its own bound
// as ERROR_CANT_RESOLVE_FILENAME. A chain that exceeds the
// process's link-following limit produces ERROR_TOO_MANY_LINKS.
// Both conditions correspond to the POSIX ELOOP sentinel.
func isSymlinkLoopError(err error) bool {
	return errors.Is(err, errnoCantResolveFilename) ||
		errors.Is(err, errnoTooManyLinks)
}

// isReadOnlyError reports whether err is ERROR_WRITE_PROTECT.
//
// The error indicates the volume is mounted read-only or the
// file's attributes forbid writes. It corresponds to the POSIX
// EROFS sentinel.
func isReadOnlyError(err error) bool {
	return errors.Is(err, errnoWriteProtect)
}

// isIsDirectoryError reports whether err is
// ERROR_ACCESS_DENIED.
//
// # Why ERROR_ACCESS_DENIED is overloaded
//
// Windows uses ERROR_ACCESS_DENIED for two distinct conditions:
//
//   - The process lacks permission (the POSIX EACCES case).
//   - The operation targeted a directory where a file was
//     expected (the POSIX EISDIR case).
//
// classify's switch checks the fs.ErrPermission branch before
// this predicate, so a genuine permission error is classified
// as ErrPermissionDenied first. The predicate therefore fires
// only for the directory-targeted case, which is the residual
// use of the code.
//
// The overloading is a limitation of the Windows error
// vocabulary; the taxonomy cannot distinguish the two cases
// without inspecting the operation's context. Callers that
// receive ErrIsDirectory on Windows should treat it as a hint
// rather than a certainty.
func isIsDirectoryError(err error) bool {
	return errors.Is(err, errnoAccessDenied)
}

// isNotDirectoryError reports whether err is ERROR_DIRECTORY.
//
// The error indicates the path names a file where a directory
// was expected. It corresponds to the POSIX ENOTDIR sentinel.
func isNotDirectoryError(err error) bool {
	return errors.Is(err, errnoDirectory)
}
