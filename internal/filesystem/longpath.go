package filesystem

import "path/filepath"

// prepareForOS takes a name that has already been validated as a
// cleaned, forward-slash, local relative path — the shape
// produced by safefs.validateName — and returns a string that is
// safe to hand to the os package on the current platform.
//
// # What "safe" means on each platform
//
// On Unix, safe means "join with the boundary and pass through."
// There is no path-length limit on Linux or macOS that the
// abstraction needs to work around; the operating system enforces
// PATH_MAX on individual syscalls, and the caller's responsibility
// is to keep names short enough. prepareForOS on Unix is
// therefore the identity function.
//
// On Windows, safe means "join with the boundary, and if the
// result exceeds MAX_PATH (260 characters, or 240 to leave a
// safety margin for APIs that append suffixes), prefix the result
// with \\?\ (or \\?\UNC\ for UNC paths)." The prefix disables the
// operating system's path-length checks and lets the file be
// opened by its full name. See longpath_windows.go for the
// implementation.
//
// # Why prepareForOS is applied after joining
//
// The Windows path-length limit applies to the fully resolved
// path, not to the relative name alone. A short name joined to a
// deep boundary can still exceed the limit; conversely, a long
// name joined to a shallow boundary may not. prepareForOS
// therefore operates on the joined path, not on either component.
//
// # Why the function is not exported
//
// prepareForOS is an implementation detail. A caller that needs
// to know the OS path it is about to hand to os can call
// Boundary() and join the name itself. The abstraction's job is
// to make that caller's job unnecessary; exposing the function
// would invite callers to depend on the prefix appearing (or not
// appearing) in their own code, which is exactly the coupling the
// abstraction exists to prevent.
//
// # Contract for platform files
//
// Every implementation of prepareForOS must:
//
//   - Return the same string for the same input on repeated calls.
//   - Not perform I/O.
//   - Not return an error. Path preparation is pure string work;
//     a name that cannot be prepared is a name that the caller
//     should not have produced, and the failure mode is a
//     compile-time or test-time property, not a runtime one.
//   - Preserve the invariant that the return value, when handed
//     to os, addresses the same file the caller named.
func prepareForOS(joined string) string {
	return prepareForOSPlatform(joined)
}

// separator is the host platform's path separator, exposed so
// that tests can assert on it without importing os directly.
//
// The FS interface documents that callers pass forward-slash
// names. The separator is what the host uses when it joins them
// with the boundary and hands them to the kernel. On Unix the two
// are the same character; on Windows they are not, and the
// abstraction is what hides the difference.
const separator = filepath.Separator
