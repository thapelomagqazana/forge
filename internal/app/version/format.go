package version

import (
	"fmt"
	"io"
	"runtime"
)

// Format writes a human-readable rendering of info to w.
//
// # The frozen format
//
// The output is exactly six lines:
//
//	forge <version>
//	  commit:     <short-sha>
//	  built:      <rfc3339-utc>
//	  dirty:      <true|false>
//	  go version: <go-version>
//	  platform:   <os>/<arch>
//
// The format is frozen by WBS 6.4.1. Every byte of every line is
// part of the contract. A change to any byte requires a new WBS
// item.
//
// # The header line
//
// When Version is empty (an uninstrumented build), the header is
// exactly "forge" with no trailing space. When Version is
// non-empty, the header is "forge " followed by the version.
//
// The empty case is not special-cased in the format string; it is
// special-cased in a branch. The format string "forge %s\n" would
// produce "forge \n" (trailing space) for an empty version, which
// is not the frozen format.
//
// # The detail lines
//
// Every detail line has:
//
//   - two spaces of indent
//   - the key followed by a colon, padded to twelve characters
//   - a single space
//   - the value
//
// The padding to twelve characters is what makes the value column
// align across all five detail lines. Twelve is the length of the
// longest key ("go version:") plus one character of padding, and
// the single space after the padding makes the value start at
// column 15 (1-indexed).
//
// The alignment is asserted byte-for-byte by TestFormat_FullOutput
// and asserted structurally by TestFormat_ValueColumnIsAligned.
//
// # The Dirty line
//
// The `dirty` line reports whether the binary was built from a
// working tree with uncommitted changes. The value is parsed
// strictly: only the literal "true" renders as true; every other
// value — including "false", "1", "yes", the empty string, and any
// typo — renders as false.
//
// The strict rule matches the parse rule documented on
// internal/version.Dirty (WBS 6.1.1) and the injection rule in
// WBS 6.1.2. It ensures the output is stable regardless of how the
// build system represented the boolean.
//
// # The empty-value case
//
// An uninstrumented build has empty Version, Commit, and BuildDate.
// The formatter renders Commit and BuildDate as empty values (with
// the trailing padding spaces retained). It does not substitute
// sentinels such as "unknown" or "dev". The empty value is the
// honest signal, defined in WBS 6.1.1, that the binary was not
// built with linker injection.
//
// # Runtime reads
//
// The formatter reads runtime.Version() and runtime.GOOS /
// runtime.GOARCH to populate the "go version" and "platform" lines.
// These are properties of the running binary, not of the build
// metadata injected at link time, so they are not fields of Info.
// Reading them here keeps Info as the four-field model and puts
// the runtime reads in the single place that renders them.
//
// # Error behaviour
//
// Format returns an error if and only if the underlying Writer
// returns an error. It does not return an error for empty fields,
// for an unknown platform, or for any other condition. A caller
// that passes a nil Writer will panic when Format first writes;
// the panic is the correct symptom of a caller that failed to
// construct its dependencies.
//
// # Determinism
//
// The output is deterministic given the same Info and the same Go
// runtime. The Go version and platform do not change within a
// single process. Two calls with the same Info produce
// byte-identical output.
func Format(w io.Writer, info Info) error {
	// Header. When Version is empty, the line is exactly "forge"
	// with no trailing space. This is the frozen format for
	// uninstrumented builds; it is asserted by
	// TestFormat_EmptyVersionRendersHeaderWithoutTrailingSpace.
	header := "forge"
	if info.Version != "" {
		header = "forge " + info.Version
	}
	if _, err := fmt.Fprintf(w, "%s\n", header); err != nil {
		return fmt.Errorf("format version header: %w", err)
	}

	// Detail lines. The order is: build metadata (commit, built,
	// dirty), then runtime information (go version, platform). A
	// reader who wants to know how the binary was produced reads
	// the first three lines; a reader who wants to know what it
	// will run on reads the last two.
	//
	// The key-plus-colon is padded to twelve characters so that
	// every value starts at the same column. Twelve is the width
	// of the longest key ("go version:"), which is eleven
	// characters plus one of padding. A single space follows the
	// padding; the total prefix before the value is two spaces of
	// indent + twelve of field + one of separator = fifteen, so
	// every value starts at column 15 (1-indexed).
	//
	// Do not change the field width without also changing the
	// frozen example in this docstring and the assertions in
	// TestFormat_FullOutput.
	details := []struct {
		key   string
		value string
	}{
		{"commit", info.Commit},
		{"built", info.BuildDate},
		{"dirty", parseDirty(info.Dirty)},
		{"go version", runtime.Version()},
		{"platform", runtime.GOOS + "/" + runtime.GOARCH},
	}

	for _, d := range details {
		if _, err := fmt.Fprintf(w, "  %-12s %s\n", d.key+":", d.value); err != nil {
			return fmt.Errorf("format detail %q: %w", d.key, err)
		}
	}

	return nil
}

// parseDirty converts the raw Dirty string to a lowercase "true"
// or "false" for the frozen format.
//
// # The strict rule
//
// Only the literal "true" is truthy. Everything else — "false",
// "1", "yes", "TRUE", the empty string, or any typo — is falsey.
// This is the same rule documented on internal/version.Dirty
// (WBS 6.1.1) and enforced by TestDirty_ParsingRuleIsStrict in
// internal/version/version_test.go.
//
// # Why the formatter normalises
//
// The model does not validate Dirty; it stores whatever the build
// system injected. The formatter's job is to produce a stable
// output for downstream consumers. A consumer that reads
// "dirty: true" or "dirty: false" should never see "dirty: yes"
// or "dirty: " (empty) because a build system passed the wrong
// string. The normalisation here is the boundary where "any
// string from the build system" becomes "one of two values in the
// frozen format".
//
// # Why not strconv.ParseBool
//
// strconv.ParseBool accepts "1", "t", "T", "TRUE", "true",
// "True", and their falsey counterparts. Accepting "1" would make
// the output of `forge version` depend on which shell produced
// the build. The strict rule is auditable: a reviewer sees
// parseDirty("1") == "false" and can ask why the build system
// produced "1". See WBS 6.1.2 § "Rules".
func parseDirty(s string) string {
	if s == "true" {
		return "true"
	}
	return "false"
}
