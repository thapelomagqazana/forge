//go:build unix

package streams

import (
	"errors"
	"syscall"
)

// isBrokenPipePlatform reports whether err (or any error it wraps)
// is a Unix broken-pipe error.
//
// # What the function matches
//
//   - syscall.EPIPE — the canonical "broken pipe" errno. Returned
//     by write(2) when the read end of the pipe has closed.
//
// The function does NOT match syscall.EINTR (interrupted system
// call): an EINTR is not a broken pipe and should be retried by
// the caller.
func isBrokenPipePlatform(err error) bool {
	return errors.Is(err, syscall.EPIPE)
}
