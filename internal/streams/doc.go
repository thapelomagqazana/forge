// Package streams centralizes access to the process's three
// standard streams: stdin, stdout, and stderr.
//
// # Why a package
//
// The CLI's stream contract (see docs/reference/output-streams.md
// and ADR-022) is: product output goes to stdout, diagnostics go
// to stderr, errors go to stderr. Every command must read and
// write through the same three streams. If each command read
// os.Stdout directly, tests would have to replace the process's
// globals or fork a subprocess to observe behavior. Centralizing
// the streams behind an injectable value lets tests provide a
// buffer and assert on the bytes.
//
// The package has three responsibilities:
//
//   - Provide the default Streams bound to the process's standard
//     streams (os.Stdin, os.Stdout, os.Stderr).
//   - Let the CLI inject an alternate Streams at the process
//     boundary (main), so tests and embedding callers can supply
//     their own.
//   - Detect broken-pipe errors and classify them for the exit
//     boundary, which maps them to exit 141 (128 + SIGPIPE).
//
// # The stream contract
//
// Output → stdout. Logger → stderr. Errors → stderr. The
// classification is enforced at the type level: an output writer
// is an io.Writer, a logger writer is an io.Writer, and the two
// are always distinct values. A caller that writes product output
// to the logger's writer is writing to the wrong stream, and the
// streams type has no API to do so by accident.
//
// # Broken pipe
//
// The process exits cleanly (code 141) when stdout is closed by a
// downstream consumer, as in:
//
//	forge template list | head -1
//
// head exits after one line, the pipe's read end closes, and the
// next write to stdout fails with a broken-pipe error. The
// process must not panic and must not print a Go stack trace.
// The broken-pipe classification and handler live in this package
// and in the CLI's exit boundary; see IsBrokenPipe and
// HandleWriteError.
//
// # Platform support
//
// The classification of a broken-pipe error is platform-specific:
//
//   - Unix: syscall.EPIPE (or the wrapped form).
//   - Windows: syscall.ERROR_BROKEN_PIPE and
//     syscall.ERROR_NO_DATA.
//   - Other: no platform-specific detection; the generic error
//     path applies.
//
// The detection lives in brokenpipe_unix.go,
// brokenpipe_windows.go, and brokenpipe_other.go, selected by
// build tags.
package streams
