//go:build windows

package streams

import (
	"errors"
	"syscall"
)

// isBrokenPipePlatform reports whether err (or any error it wraps)
// is a Windows broken-pipe error.
//
// # What the function matches
//
//   - syscall.ERROR_BROKEN_PIPE — the pipe has been closed by the
//     other end. Returned by WriteFile when the read handle of an
//     anonymous pipe has closed. This is the code the Windows
//     kernel returns for the scenario Forge encounters: a shell
//     pipeline whose downstream consumer exited early.
//
// # What the function does not match
//
// The Windows API also defines ERROR_NO_DATA (232), returned when
// a writer on a *named* pipe sees the reader close the pipe while
// the writer is still flushing. Forge does not create named pipes;
// the pipe between Forge and its downstream consumer is anonymous,
// and the kernel reports its closure as ERROR_BROKEN_PIPE. The
// check therefore omits ERROR_NO_DATA.
//
// The constant is not available in the standard library's syscall
// package (only golang.org/x/sys/windows declares it), so adding
// the check would require a direct module dependency. The trade
// is not worth it for a case Forge cannot encounter. If a future
// scenario introduces a named pipe, the check is added here as:
//
//	const errorNoData = syscall.Errno(232)
//	return errors.Is(err, syscall.ERROR_BROKEN_PIPE) ||
//	       errors.Is(err, errorNoData)
//
// with a test that pins the constant to its Windows API value
// (232) so a change to the number is caught.
func isBrokenPipePlatform(err error) bool {
	return errors.Is(err, syscall.ERROR_BROKEN_PIPE)
}
