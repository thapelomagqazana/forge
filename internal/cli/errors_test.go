// Package cli contains white-box tests for the CLI package.
//
// This file pins the invalid-command contract (WBS 7.3.1). The
// contract specifies what happens when a user types an invalid
// invocation. Each test iterates a set of cases and asserts one
// property.
//
// # The cases
//
// The seven cases are listed in errors.go. Each case is a
// table-driven subtest; the subtest name carries the case ID so
// that a failure points at the case in the documentation.
//
// # The frozen error format
//
// The user-facing error format is frozen in WBS 10.4.1 and lives
// in internal/forgeerr/format.go. The CLI produces
// *forgeerr.Error values and lets forgeerr.Format render them. The
// assertions below therefore expect the frozen format:
//
//	Error: <message>
//	Code:  <code>
//	       <context-key>: <value>
//
//	Suggestion:
//	  <remediation>
//
// The format's byte-exactness is tested in
// internal/forgeerr/format_test.go against the golden files in
// internal/forgeerr/testdata/format/. This file does not
// duplicate that coverage; it asserts the properties the CLI is
// responsible for: exit code, empty stdout, the required
// substrings, the absence of ANSI escapes, and the first-line and
// trailing-newline rules.
//
// # How the tests invoke the CLI
//
// Each test invokes the CLI through runCLI, which routes through
// executeWithOptions. The invocation exercises the same path
// production code uses: argument parsing, pre-parse validation,
// command dispatch, and error rendering.
//
// # The Cobra-error wrapping (WBS 10.5.1)
//
// Cobra's parser and dispatcher produce plain errors for the
// failures they detect: unknown flags, unknown commands. The CLI
// wraps each as a *forgeerr.Error (via wrapCobraError in
// execute.go), so the frozen format applies uniformly. The
// wrapped error puts the varying data (the flag name, the command
// name) in the context map, not in the message. The I4 and I5
// assertions below reflect that shape.
package cli

import (
	"strings"
	"testing"
)

// invalidCase describes one invalid invocation.
type invalidCase struct {
	// id is the case identifier (I1, I2, ...). It appears in the
	// subtest name so that a failure points at the row in the
	// documentation.
	id string

	// name is a short description of the case. It appears in the
	// subtest name alongside the id.
	name string

	// args is the argument vector to pass to runCLI. It does not
	// include the program name; runCLI prepends it.
	args []string

	// contains is a list of substrings that must appear in
	// stderr.
	contains []string
}

// invalidCases returns the seven cases the contract pins.
//
// The function is a method-style helper; it returns a fresh slice
// on each call so that a caller cannot mutate a shared value.
//
// # The substrings
//
// The substrings are chosen to match the frozen format's
// structure. Two properties are asserted for each case: the code
// (the stable identifier) and the varying data (the flag name,
// the command name, the unknown topic). A third property — the
// message — is asserted for cases whose message is stable and
// distinctive.
//
// The substrings do not assert the exact indentation of context
// lines. The indentation is the frozen format's concern (rule F3
// of WBS 10.4.1); the CLI's tests assert on the semantic content.
func invalidCases() []invalidCase {
	return []invalidCase{
		{
			id:   "I1",
			name: "unknown root command",
			args: []string{"foobar"},
			contains: []string{
				"unknown command",
				"foobar",
				"FORGE_CLI_UNKNOWN_COMMAND",
			},
		},
		{
			id:   "I2",
			name: "typo of a known command",
			args: []string{"verison"},
			contains: []string{
				"unknown command",
				"verison",
				"FORGE_CLI_UNKNOWN_COMMAND",
				"Did you mean",
				"version",
			},
		},
		{
			id:   "I3",
			name: "extra argument to a no-args command",
			args: []string{"version", "extra"},
			contains: []string{
				"unknown command",
				"extra",
				"FORGE_CLI_UNKNOWN_COMMAND",
			},
		},
		{
			id:   "I4",
			name: "unknown flag at the root",
			args: []string{"--unknown-flag"},
			contains: []string{
				"unknown flag",
				"FORGE_CLI_UNKNOWN_FLAG",
				"--unknown-flag",
			},
		},
		{
			id:   "I5",
			name: "unknown flag at a subcommand",
			args: []string{"version", "--unknown-flag"},
			contains: []string{
				"unknown flag",
				"FORGE_CLI_UNKNOWN_FLAG",
				"--unknown-flag",
			},
		},
		{
			id:   "I6",
			name: "unknown help topic",
			args: []string{"help", "unknown"},
			contains: []string{
				"unknown",
			},
		},
		{
			id:   "I7",
			name: "args to the help flag",
			args: []string{"--help", "unknown"},
			contains: []string{
				"help",
			},
		},
	}
}

// =============================================================================
// The matrix test
// =============================================================================

// TestErrors_Matrix exercises every case of the invalid-command
// contract and asserts the shared properties: exit code 2, empty
// stdout, non-empty stderr, the required substrings, and the
// first-line and trailing-newline rules of the frozen format.
//
// The test does not assert the exact bytes of stderr. That
// coverage lives in internal/forgeerr/format_test.go, which tests
// the frozen format's golden files directly.
func TestErrors_Matrix(t *testing.T) {
	t.Parallel()

	for _, tc := range invalidCases() {
		tc := tc
		t.Run(tc.id+" "+tc.name, func(t *testing.T) {
			t.Parallel()

			got := runCLI(t, tc.args, nil)

			// Exit code 2.
			if got.exitCode != ExitUsage {
				t.Errorf("exit code: got %d, want %d (ExitUsage)\n"+
					"stderr: %q",
					got.exitCode, ExitUsage, got.stderr)
			}

			// stdout is empty.
			if got.stdout != "" {
				t.Errorf("stdout should be empty; got %q", got.stdout)
			}

			// stderr is non-empty.
			if got.stderr == "" {
				t.Errorf("stderr should contain a diagnostic")
			}

			// Required substrings.
			for _, want := range tc.contains {
				if !strings.Contains(got.stderr, want) {
					t.Errorf("stderr does not contain %q:\n%s",
						want, got.stderr)
				}
			}

			// No ANSI escapes.
			if idx := strings.IndexByte(got.stderr, 0x1b); idx >= 0 {
				t.Errorf("stderr contains an ANSI escape at byte %d: %q",
					idx, got.stderr[idx:minInt(idx+10, len(got.stderr))])
			}

			// First line starts with "Error: ".
			if !strings.HasPrefix(got.stderr, "Error: ") {
				t.Errorf("stderr does not start with %q:\n%s",
					"Error: ", got.stderr)
			}

			// Exactly one trailing newline.
			if !strings.HasSuffix(got.stderr, "\n") {
				t.Errorf("stderr does not end in a newline: %q", got.stderr)
			}
			if strings.HasSuffix(got.stderr, "\n\n") {
				t.Errorf("stderr ends in more than one newline: %q", got.stderr)
			}
		})
	}
}

// =============================================================================
// Suggestion mechanism
// =============================================================================

// TestErrors_TypoSuggestsVersion verifies that the typo case (I2)
// produces a suggestion.
//
// The suggestion is carried as the *forgeerr.Error's remediation
// and rendered by forgeerr.Format as the Suggestion block. The
// test asserts the distinctive substrings the block must contain.
//
// The test is separate from the matrix because the exact
// rendering of the suggestion is a property of the frozen format
// (tested in internal/forgeerr/format_test.go), not of the CLI.
// This test asserts that the CLI attaches a suggestion to the
// error in the first place.
func TestErrors_TypoSuggestsVersion(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"verison"}, nil)

	if got.exitCode != ExitUsage {
		t.Fatalf("exit code: got %d, want %d", got.exitCode, ExitUsage)
	}

	if !strings.HasPrefix(got.stderr, "Error: ") {
		t.Errorf("stderr does not start with %q:\n%s",
			"Error: ", got.stderr)
	}
	if !strings.Contains(got.stderr, "Code:  FORGE_CLI_UNKNOWN_COMMAND") {
		t.Errorf("stderr does not contain the code line:\n%s", got.stderr)
	}
	if !strings.Contains(got.stderr, "Suggestion:") {
		t.Errorf("stderr does not contain a Suggestion block:\n%s",
			got.stderr)
	}
	if !strings.Contains(got.stderr, "Did you mean") {
		t.Errorf("stderr does not contain the hint:\n%s", got.stderr)
	}
	if !strings.Contains(got.stderr, "version") {
		t.Errorf("stderr does not suggest the version command:\n%s",
			got.stderr)
	}
}

// TestErrors_SuggestionMinimumDistanceIsTwo verifies that the
// suggestion distance is 2.
//
// The check reads the constant suggestionsMinimumDistance
// (defined in errors.go). A future change to the constant is a
// change to the contract; the test would fail and the
// contributor would update the contract's documentation in the
// same commit.
func TestErrors_SuggestionMinimumDistanceIsTwo(t *testing.T) {
	t.Parallel()

	if suggestionsMinimumDistance != 2 {
		t.Errorf("suggestionsMinimumDistance = %d; want 2",
			suggestionsMinimumDistance)
	}
}

// =============================================================================
// Helpers
// =============================================================================

// minInt returns the smaller of two ints. It is used by the ANSI
// escape check to bound a slice operation. The helper exists
// because the project's go.mod may declare a Go version older
// than 1.21, where the built-in min was introduced.
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
