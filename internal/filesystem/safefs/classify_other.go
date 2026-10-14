//go:build !windows

package safefs

import (
	"errors"
	"syscall"
)

// This file provides the Unix-specific predicates that classify
// uses to distinguish OS-level error codes that the standard
// library's sentinels do not cover.

// isReservedNameError is always false on POSIX: the Windows
// reserved-name set does not exist.
func isReservedNameError(err error) bool {
	return false
}

// isPathTooLongError reports whether err is ENAMETOOLONG.
func isPathTooLongError(err error) bool {
	return errors.Is(err, syscall.ENAMETOOLONG)
}

// isSymlinkLoopError reports whether err is ELOOP.
func isSymlinkLoopError(err error) bool {
	return errors.Is(err, syscall.ELOOP)
}

// isReadOnlyError reports whether err is EROFS.
func isReadOnlyError(err error) bool {
	return errors.Is(err, syscall.EROFS)
}

// isIsDirectoryError reports whether err is EISDIR.
func isIsDirectoryError(err error) bool {
	return errors.Is(err, syscall.EISDIR)
}

// isNotDirectoryError reports whether err is ENOTDIR.
func isNotDirectoryError(err error) bool {
	return errors.Is(err, syscall.ENOTDIR)
}
