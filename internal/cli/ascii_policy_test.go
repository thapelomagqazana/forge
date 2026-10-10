// Package cli contains white-box tests for the CLI package.
//
// This file pins the ASCII-only output policy (WBS 7.4.2). The
// policy freezes two Phase 2 decisions:
//
//   - No colour output. No ANSI escape sequences are emitted.
//   - No non-ASCII characters in output. Every byte is in the
//     ASCII printable range or is a permitted control byte.
//
// # How the tests invoke the CLI
//
// Each test invokes the CLI through runCLI and asserts the policy
// on the captured stdout and stderr. The invocations cover the
// representative configurations a user might run:
//
//   - Each command's help output.
//   - Each command's success output (where applicable).
//   - Each command's error output (where applicable).
//   - The root command's help output.
//   - The version command's text and JSON output.
//
// The tests do not enumerate every possible invocation. They cover
// the paths that produce output; a future path that produces new
// output is caught by the future test that exercises it.
//
// # What the tests do not do
//
// The tests do not enforce the future-proofing rules. The rules
// apply to Phase 6+; they are documented but not tested in Phase 2.
// Testing them would require a colour implementation that does not
// yet exist.
package cli

import (
	"testing"
)

// =============================================================================
// The policy predicate
// =============================================================================

// TestASCIIPolicy_PredicateOnPlainText verifies the predicate's
// behaviour on plain text.
//
// The predicate is the mechanical implementation of the ASCII-only
// rule. The test pins its behaviour on a small set of inputs so
// that a future change to the predicate is caught by a specific
// failure.
func TestASCIIPolicy_PredicateOnPlainText(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"empty", "", true},
		{"spaces", "   ", true},
		{"plain ASCII", "forge version", true},
		{"with newline", "line1\nline2", true},
		{"with tab", "col1\tcol2", true},
		{"with carriage return", "line1\r\nline2", false},
		{"with form feed", "line1\x0cline2", false},
		{"with ESC", "text\x1b[0m", false},
		{"with non-ASCII", "café", false},
		{"with em dash", "a—b", false},
		{"with right arrow", "CREATE → forge new", false},
		{"with emoji", "✓", false},
		{"boundary: 0x1f", "a\x1fb", false},
		{"boundary: 0x20", "a b", true},
		{"boundary: 0x7e", "a~b", true},
		{"boundary: 0x7f", "a\x7fb", false},
		{"boundary: 0x80", "a\x80b", false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := isASCIIOnly(tc.in)
			if got != tc.want {
				t.Errorf("isASCIIOnly(%q) = %v; want %v", tc.in, got, tc.want)
			}
		})
	}
}

// TestASCIIPolicy_FirstNonASCIIByte verifies the byte-index finder.
func TestASCIIPolicy_FirstNonASCIIByte(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		in      string
		wantIdx int
	}{
		{"empty", "", -1},
		{"plain", "abc", -1},
		{"with newline", "a\nb", -1},
		{"with tab", "a\tb", -1},
		{"with carriage return", "a\rb", 1},
		{"with ESC", "ab\x1bcd", 2},
		{"with non-ASCII", "abé", 2},
		{"first byte non-ASCII", "éabc", 0},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := firstNonASCIIByte(tc.in)
			if got != tc.wantIdx {
				t.Errorf("firstNonASCIIByte(%q) = %d; want %d",
					tc.in, got, tc.wantIdx)
			}
		})
	}
}

// TestASCIIPolicy_FirstAnsiEscape verifies the ANSI-escape finder.
func TestASCIIPolicy_FirstAnsiEscape(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		in      string
		wantIdx int
	}{
		{"empty", "", -1},
		{"plain", "abc", -1},
		{"with newline", "a\nb", -1},
		{"with tab", "a\tb", -1},
		{"with ESC", "ab\x1bcd", 2},
		{"first byte ESC", "\x1babc", 0},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := firstAnsiEscape(tc.in)
			if got != tc.wantIdx {
				t.Errorf("firstAnsiEscape(%q) = %d; want %d",
					tc.in, got, tc.wantIdx)
			}
		})
	}
}

// =============================================================================
// The policy applied to real output
// =============================================================================

// assertASCIIPolicy asserts that a testRun's stdout and stderr
// satisfy the ASCII-only policy.
//
// The helper is used by every test below. It reports the offending
// byte's stream, index, and value if the policy is violated.
func assertASCIIPolicy(t *testing.T, got testRun) {
	t.Helper()

	for _, stream := range []struct {
		name string
		data string
	}{
		{"stdout", got.stdout},
		{"stderr", got.stderr},
	} {
		if idx := firstAnsiEscape(stream.data); idx >= 0 {
			end := idx + 10
			if end > len(stream.data) {
				end = len(stream.data)
			}
			t.Errorf("%s contains an ANSI escape at byte %d: %q",
				stream.name, idx, stream.data[idx:end])
		}
		if idx := firstNonASCIIByte(stream.data); idx >= 0 {
			end := idx + 10
			if end > len(stream.data) {
				end = len(stream.data)
			}
			t.Errorf("%s contains a non-ASCII byte at byte %d (0x%02x): %q",
				stream.name, idx, stream.data[idx], stream.data[idx:end])
		}
	}
}

// TestASCIIPolicy_RootHelp verifies that the root command's help
// output satisfies the policy.
func TestASCIIPolicy_RootHelp(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"--help"}, nil)
	assertASCIIPolicy(t, got)
}

// TestASCIIPolicy_CommandHelp verifies that every registered
// command's help output satisfies the policy.
//
// The test iterates the registry. A future command whose help
// output contains a non-ASCII character fails the test with the
// offending byte.
func TestASCIIPolicy_CommandHelp(t *testing.T) {
	t.Parallel()

	for _, cmd := range allRegisteredCommands() {
		cmd := cmd
		name := commandName(cmd.Use)
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := runCLI(t, []string{name, "--help"}, nil)
			assertASCIIPolicy(t, got)
		})
	}
}

// TestASCIIPolicy_VersionText verifies that the version command's
// text output satisfies the policy.
func TestASCIIPolicy_VersionText(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"version"}, nil)
	assertASCIIPolicy(t, got)
}

// TestASCIIPolicy_VersionJSON verifies that the version command's
// JSON output satisfies the policy.
//
// The JSON encoder may emit non-ASCII bytes if the input contains
// them. The version command's input is a set of linker-injected
// strings whose charset is constrained by WBS 6.1.2. The test
// verifies the constraint holds at the output layer.
func TestASCIIPolicy_VersionJSON(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"version", "--format", "json"}, nil)
	assertASCIIPolicy(t, got)
}

// TestASCIIPolicy_UnknownCommand verifies that an unknown command's
// error output satisfies the policy.
func TestASCIIPolicy_UnknownCommand(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{boundaryFixtureUnknownCommand}, nil)
	assertASCIIPolicy(t, got)
}

// TestASCIIPolicy_UnknownFormat verifies that an unknown format
// value's error output satisfies the policy.
func TestASCIIPolicy_UnknownFormat(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{
		boundaryFixtureVersion,
		"--format",
		boundaryFixtureUnknownFormat,
	}, nil)
	assertASCIIPolicy(t, got)
}

// TestASCIIPolicy_ExtraArguments verifies that an extra-argument
// error's output satisfies the policy.
func TestASCIIPolicy_ExtraArguments(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{boundaryFixtureVersion, "extra"}, nil)
	assertASCIIPolicy(t, got)
}

// =============================================================================
// Root identity strings
// =============================================================================

// TestASCIIPolicy_RootIdentityStrings verifies that the four root
// identity constants are ASCII-only.
//
// The constants are the CLI's identity; they appear in every help
// invocation. Their ASCII compliance is a precondition for the
// help output's policy compliance.
//
// # Boundary: the em dash and the right arrow
//
// The current values of RootShortDesc and RootLongDesc contain an
// em dash (—, U+2014) and a right arrow (→, U+2192). Both are
// non-ASCII characters. The policy forbids them. The test fails,
// and the constants must be updated to use ASCII equivalents
// (for example, "--" for the em dash and "->" for the arrow).
//
// The WBS item notes that "no emoji or Unicode symbols in Phase 2
// output" and "all output uses ASCII printable characters only".
// The em dash and the arrow are Unicode symbols; the policy
// requires their removal.
func TestASCIIPolicy_RootIdentityStrings(t *testing.T) {
	t.Parallel()

	constants := []struct {
		name  string
		value string
	}{
		{"RootName", RootName},
		{"RootUsage", RootUsage},
		{"RootShortDesc", RootShortDesc},
		{"RootLongDesc", RootLongDesc},
	}

	for _, c := range constants {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			if !isASCIIOnly(c.value) {
				idx := firstNonASCIIByte(c.value)
				end := idx + 10
				if end > len(c.value) {
					end = len(c.value)
				}
				t.Errorf("%s contains a non-ASCII byte at byte %d "+
					"(0x%02x): %q\n"+
					"the ASCII-only policy (WBS 7.4.2) requires "+
					"an ASCII equivalent",
					c.name, idx, c.value[idx], c.value[idx:end])
			}
		})
	}
}
