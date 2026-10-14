//go:build !windows

package safefs

import (
	"errors"
	"syscall"
)

// This file provides the Unix implementation of isSymlinkLoop,
// the predicate that resolveRootSymlink uses to distinguish a
// symlink loop (a hard error at Root construction) from a target
// that does not exist (which is not an error).
//
// # Why a build-tagged file
//
// The error code for a symlink loop is platform-specific. On
// POSIX systems the code is ELOOP; on Windows it is one of two
// ERROR_* codes. The predicate isolates the difference so that
// resolveRootSymlink can call a single function.
//
// # Where the predicate is used
//
// resolveRootSymlink (symlink.go) calls it after
// filepath.EvalSymlinks fails. If the failure is ELOOP, the
// target is misconfigured and New returns an error wrapping
// ErrSymlinkLoop. If the failure is fs.ErrNotExist, the target
// may simply not have been created yet, and New falls back to
// the unresolved absolute path.
//
// # Where the predicate is NOT used
//
// The pipeline's symlink resolution (followInRoot) has its own
// loop bound (MaxSymlinkChain) and does not depend on the OS's
// ELOOP detection. The two mechanisms are complementary: the
// pipeline's bound catches cycles in the link graph, and the
// OS's ELOOP catches the case where the kernel gives up before
// the pipeline's bound is reached.
func isSymlinkLoop(err error) bool {
	return errors.Is(err, syscall.ELOOP)
}
