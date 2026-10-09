package version

import (
	"fmt"
	"io"
	"runtime"
)

// Format writes a human-readable rendering of info to w.
//
// It returns an error if the write fails. A short write (fewer bytes
// written than supplied) is treated as an error; a caller that
// receives a nil error can assume the full output reached w.
//
// # What the format looks like
//
// The output is a stable, line-oriented format:
//
//	forge 0.1.0
//	  commit:     a1b2c3d
//	  built:      2026-10-09T12:00:00Z
//	  dirty:      false
//	  go version: go1.23.0
//	  platform:   linux/amd64
//
// The first line is the version number, prefixed with "forge ". A
// reader who wants to know "what version is this?" reads one line.
// The remaining lines are indented key-value pairs, in a fixed order,
// with the keys left-aligned.
//
// # The Dirty line
//
// The `dirty` line reports whether the binary was built from a
// working tree with uncommitted changes. The value is "true" or
// "false" when the binary was built with ldflags, and the empty
// string otherwise. The formatter does not interpret the value; it
// writes whatever string the Info value carries. An empty value
// renders as an empty value, which is a visible signal that the
// binary was built without build metadata.
//
// # Determinism
//
// The output is deterministic given the same Info and the same Go
// runtime. The Go version and platform are read from the runtime
// package, which does not change within a single process.
//
// # Why the formatter writes rather than returns a string
//
// A formatter that returns a string forces the caller to allocate,
// then write. A formatter that writes directly to an io.Writer avoids
// the intermediate allocation and lets the caller stream. For a
// command as small as `forge version`, the difference is negligible,
// but the pattern matters: the handler should not know how the output
// is produced, only where it goes.
//
// # Why the formatter is in the service package
//
// The formatter lives here rather than in internal/cli because the
// formatting rules are part of the application service's contract.
// If the CLI package owned the formatter, every change to the format
// would touch the CLI package. Keeping the formatter here means the
// CLI handler is a one-line call to Format, and the format itself can
// be unit-tested without constructing a Cobra command.
//
// # Error handling
//
// The function wraps write errors with a short prefix so that a
// caller can distinguish "the write failed" from "the data was
// malformed". In Phase 2, no data can be malformed; the prefix is
// defensive for future formats that may validate their input.
func Format(w io.Writer, info Info) error {
	// The header line. Rendered first so that a partial write (for
	// example, a broken pipe) still delivers the most important
	// piece of information: the version number.
	if _, err := fmt.Fprintf(w, "forge %s\n", info.Version); err != nil {
		return fmt.Errorf("format version header: %w", err)
	}

	// The detail lines. Rendered in a fixed order, so that a
	// diff between two invocations is limited to the values that
	// actually changed.
	//
	// The order is: build metadata (commit, built, dirty), then
	// runtime information (go version, platform). A reader who
	// wants to know how the binary was produced reads the first
	// three lines; a reader who wants to know what it will run on
	// reads the last two.
	details := []struct {
		key   string
		value string
	}{
		{"commit", info.Commit},
		{"built", info.BuildDate},
		{"dirty", info.Dirty},
		{"go version", runtime.Version()},
		{"platform", runtime.GOOS + "/" + runtime.GOARCH},
	}

	for _, d := range details {
		if _, err := fmt.Fprintf(w, "  %-10s  %s\n", d.key+":", d.value); err != nil {
			return fmt.Errorf("format detail %q: %w", d.key, err)
		}
	}

	return nil
}
