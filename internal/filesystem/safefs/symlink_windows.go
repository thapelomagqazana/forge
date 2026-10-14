//go:build windows

package safefs

import (
	"errors"
	"syscall"
)

// This file provides the Windows implementation of
// isSymlinkLoop.
//
// # Why a build-tagged file
//
// The error code for a symlink loop is platform-specific. On
// POSIX systems the code is ELOOP; on Windows it is one of two
// ERROR_* codes. The predicate isolates the difference so that
// resolveRootSymlink can call a single function.
//
// # Windows's two codes
//
// Windows reports a symlink chain that exceeds its own bound as
// ERROR_CANT_RESOLVE_FILENAME. A chain that exceeds the
// process's link-following limit produces ERROR_TOO_MANY_LINKS.
// Both conditions correspond to the POSIX ELOOP sentinel; the
// predicate treats them the same.
//
// # Why the numeric values are defined locally
//
// The standard library's syscall package does not export the
// Windows ERROR_* constants. The values come from the Windows
// SDK's winerror.h and are stable across Windows versions:
//
//	ERROR_TOO_MANY_LINKS        = 1142
//	ERROR_CANT_RESOLVE_FILENAME = 1921
//
// The alternative is golang.org/x/sys/windows, which exports
// them as named constants. This file uses the local definitions
// because the repository's dependency policy prefers zero
// third-party dependencies for the safefs package. A future
// ADR that permits golang.org/x/sys would replace the two
// constants with imports; the predicate's body would be
// unchanged.
func isSymlinkLoop(err error) bool {
	const (
		errTooManyLinks        = syscall.Errno(1142)
		errCantResolveFilename = syscall.Errno(1921)
	)
	return errors.Is(err, errTooManyLinks) ||
		errors.Is(err, errCantResolveFilename)
}
