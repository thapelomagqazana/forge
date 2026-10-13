package streams

import (
	"errors"
	"io"
)

// IsBrokenPipe reports whether err indicates a broken pipe on the
// current platform.
//
// # What a broken pipe is
//
// A broken pipe occurs when the process writes to a stream whose
// read end has been closed. The common case is a shell pipeline
// where the downstream consumer exits early:
//
//	forge template list | head -1
//
// head reads one line and exits; the read end of the pipe closes;
// the next write from forge's stdout fails with a broken-pipe
// error.
//
// # Why it matters
//
// Without special handling, the write's error surfaces through
// the CLI's normal error path: the process prints an error block,
// maps the error to a non-zero exit code, and (depending on the
// error) may print a Go stack trace. None of that is appropriate
// for a broken pipe: the user asked for the first line of output
// and got it; the process should exit cleanly (code 141, per
// POSIX SIGPIPE semantics) with no diagnostic.
//
// # Platform support
//
// The function delegates to a platform-specific implementation:
//
//   - Unix: syscall.EPIPE and its wrapped forms.
//   - Windows: syscall.ERROR_BROKEN_PIPE and
//     syscall.ERROR_NO_DATA.
//   - Other: no platform-specific detection; the function returns
//     false.
//
// The delegation is by build tag; the package's doc.go lists the
// three files.
func IsBrokenPipe(err error) bool {
	if err == nil {
		return false
	}
	return isBrokenPipePlatform(err)
}

// HandleWriteError is the helper every write site should use when
// it writes to one of the streams.
//
// # What the helper does
//
//   - If err is nil, returns nil.
//   - If err is a broken pipe, returns ErrBrokenPipe.
//   - Otherwise, returns err unchanged.
//
// # Why a helper
//
// The classification of a broken pipe is platform-specific and
// involves unwrapping; a caller that re-implemented it would have
// to duplicate the platform logic. Centralizing it in one helper
// means every write site classifies its errors identically.
//
// # How callers use it
//
// A write site looks like:
//
//	if _, err := fmt.Fprintln(s.Out, line); err != nil {
//	    return streams.HandleWriteError(err)
//	}
//
// The caller returns ErrBrokenPipe to the exit boundary, which
// maps it to exit 141. The caller does not have to know whether
// the error is a broken pipe; the helper does.
func HandleWriteError(err error) error {
	if err == nil {
		return nil
	}
	if IsBrokenPipe(err) {
		return ErrBrokenPipe
	}
	return err
}

// ErrBrokenPipe is the canonical error a write site returns when
// the destination's read end has been closed.
//
// # Why a sentinel
//
// The exit boundary checks for ErrBrokenPipe with errors.Is.
// Using a sentinel rather than a string comparison lets the
// boundary distinguish a real broken pipe from an unrelated error
// whose message happens to contain "broken pipe".
//
// # Relationship to exit code 141
//
// The exit boundary maps ErrBrokenPipe to exit code 141
// (128 + SIGPIPE). The sentinel and the exit code are the two
// halves of the same contract; a change to one must be reflected
// in the other and in the reference documentation.
var ErrBrokenPipe = errors.New("streams: broken pipe")

// IsClosed reports whether err indicates that a stream has been
// closed by the writer's own process. It is distinct from a
// broken pipe: a caller that has already closed its own stream
// and then writes to it gets io.ErrClosedPipe (or a similar
// error), not a kernel-level EPIPE.
//
// The function exists so that a caller can distinguish the two
// cases if it needs to; the common path treats them the same
// (both indicate the stream is no longer usable).
func IsClosed(err error) bool {
	return errors.Is(err, io.ErrClosedPipe)
}
