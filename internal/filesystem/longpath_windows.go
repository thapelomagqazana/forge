//go:build windows

package filesystem

import (
	"path/filepath"
	"strings"
)

// Constants controlling the Windows long-path prefix.
//
// # Why the threshold is 240 and not 260
//
// MAX_PATH is 260 characters, but the operating system's
// path-length check is applied to the *final* path after any
// per-API normalisation. Some Win32 APIs append suffixes (for
// example, a backup API appends a timestamp), and some add a
// trailing separator. 240 leaves a 20-character margin for those
// suffixes, which is enough for the suffixes the Go standard
// library's os package applies and for the common antivirus and
// backup tools' suffixes that intercept file I/O on Windows.
//
// 240 is not a hard boundary; it is a safety margin. A path of
// 250 characters that happens to be short enough for the current
// API will still be prefixed if the total length exceeds 240,
// which is a small cost (a 4-character prefix) for a large gain
// (the absence of a class of intermittent failures).
const (
	// longPathThreshold is the total joined-path length above
	// which the \\?\ prefix is applied.
	longPathThreshold = 240

	// longPathPrefix is the prefix that disables the Windows
	// path-length check for a local absolute path.
	longPathPrefix = `\\?\`

	// longPathUNCPrefix is the prefix that disables the check
	// for a UNC path. Note that the UNC form does not repeat
	// the leading "\\"; it replaces it.
	longPathUNCPrefix = `\\?\UNC\`
)

// prepareForOSPlatform is the Windows implementation of
// prepareForOS. It prefixes the joined path with \\?\ (or
// \\?\UNC\ for a UNC path) when the total length exceeds
// longPathThreshold.
//
// # The three input shapes
//
// A Windows path handed to this function has one of three shapes,
// and each is handled differently:
//
//  1. A local absolute path: "C:\Users\...". The prefix
//     \\?\ is prepended, producing \\?\C:\Users\...
//
//  2. A UNC path: \\server\share\path. The prefix \\?\UNC\ is
//     prepended and the leading \\ is stripped, producing
//     \\?\UNC\server\share\path. Note that \\?\UNC\ replaces
//     the leading \\, it does not precede it.
//
//  3. A relative path. \\?\ is not defined for relative paths;
//     the prefix must be applied only to absolute paths. The
//     function returns the relative path unchanged. In practice
//     the joined path produced by OSFS.resolve is always
//     absolute (the boundary is absolute after safefs.New), so
//     this case is defensive.
//
// # The forward-slash problem
//
// The \\?\ prefix disables *all* path normalisation in the
// Win32 API, including the translation of forward slashes to
// backslashes. A path that is prefixed with \\?\ and contains
// forward slashes will be passed to the kernel verbatim, and the
// kernel does not treat "/" as a separator when the prefix is
// present. The function must therefore convert forward slashes
// to backslashes before applying the prefix.
//
// The FS interface documents forward-slash names. The conversion
// happens here, at the last possible moment before the os call.
// Everywhere above this function, names are forward-slash; below
// it, they are the host's separator.
func prepareForOSPlatform(joined string) string {
	// Length is measured before any normalisation, because the
	// limit the operating system enforces is on the path as the
	// caller presents it.
	if len(joined) <= longPathThreshold {
		return joined
	}

	// The path is long enough to need the prefix. Determine
	// which prefix and convert separators.
	switch {
	case strings.HasPrefix(joined, `\\`):
		// UNC path: \\server\share\... → \\?\UNC\server\share\...
		rest := joined[2:] // strip the leading \\
		rest = toBackslash(rest)
		return longPathUNCPrefix + rest

	case filepath.IsAbs(joined):
		// Local absolute path: C:\... → \\?\C:\...
		return longPathPrefix + toBackslash(joined)

	default:
		// Relative path. \\?\ is undefined for relative
		// paths. Return unchanged.
		//
		// This branch is defensive: safefs.New resolves the
		// boundary to an absolute path, so OSFS.resolve
		// always produces an absolute joined path. A caller
		// that constructs an OSFS with a relative boundary
		// and hands it a long relative name will reach this
		// branch, and the OS will apply its usual path-length
		// check. The result is a clear ENAMETOOLONG rather
		// than an ambiguous failure, which is the correct
		// outcome: the caller violated the abstraction's
		// precondition (boundary must be absolute after
		// safefs.New), and the error names the cause.
		return joined
	}
}

// toBackslash converts forward slashes to backslashes. It is a
// deliberate replacement, not a path translation: the Windows
// kernel treats "/" as a separator only when the path is not
// prefixed with \\?\, and the caller's name is guaranteed by
// the FS interface to use forward slashes. Any forward slash in
// the joined path is therefore a separator that must become a
// backslash before the prefix is applied.
//
// The function does not touch other characters. A forward slash
// inside a filename (which is impossible on Windows but possible
// in a name that reached this function through a bug) would be
// converted to a backslash, which would change the file being
// addressed. The conversion is correct under the abstraction's
// invariant that the name is a well-formed path; a name that
// violates the invariant is a bug in the caller.
func toBackslash(s string) string {
	if !strings.ContainsRune(s, '/') {
		return s
	}
	return strings.ReplaceAll(s, "/", `\`)
}
