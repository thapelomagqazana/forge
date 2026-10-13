//go:build !unix && !windows

package streams

// isBrokenPipePlatform reports whether err (or any error it wraps)
// is a broken-pipe error on the current platform.
//
// # Why a fallback
//
// The package targets Unix and Windows explicitly; other
// platforms (for example, js/wasm, plan9) do not have a standard
// broken-pipe errno. On those platforms, the function returns
// false: the caller's write error surfaces through the CLI's
// normal error path, which is the correct behavior for a
// platform without the POSIX SIGPIPE convention.
//
// # Adding a platform
//
// A new platform with a broken-pipe errno adds a file with the
// appropriate build tag and a real implementation. The build-tag
// structure ensures that exactly one implementation is compiled
// per build.
func isBrokenPipePlatform(err error) bool {
	return false
}
