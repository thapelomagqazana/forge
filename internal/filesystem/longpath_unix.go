//go:build !windows

package filesystem

// prepareForOSPlatform is the Unix implementation of
// prepareForOS. It is the identity function.
//
// # Why it is the identity
//
// Unix-like systems do not have a path-length limit that the
// abstraction needs to work around. Each syscall (open(2),
// stat(2), unlink(2), ...) enforces PATH_MAX (typically 4096 on
// Linux, 1024 on macOS) on the path argument it is given, and a
// path exceeding that limit returns ENAMETOOLONG. There is no
// prefix analogous to Windows's \\?\ that raises the limit. The
// caller's responsibility is to keep paths short enough; the
// abstraction does not intervene.
//
// The function is not inlined away by the compiler because it is
// called through the portable wrapper prepareForOS, but the
// wrapper is a single function call whose body is a single
// return statement. The overhead is negligible, and the
// readability gain — a reviewer of longpath.go sees the seam,
// and a reviewer of longpath_unix.go sees the Unix answer — is
// worth the indirection.
//
// # What would change this
//
// If a future platform gained a long-path prefix analogous to
// Windows's, this file would be replaced by a platform-specific
// counterpart. The portable interface in longpath.go would not
// change; the caller would not change; only the implementation
// would. That is the point of the seam.
func prepareForOSPlatform(joined string) string {
	return joined
}
