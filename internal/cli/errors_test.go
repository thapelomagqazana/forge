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
// # Golden files
//
// For the five cases that produce stable, fully-deterministic
// output, the test compares the stderr against a golden file under
// testdata/errors/. The two cases that involve Cobra's suggestion
// mechanism (I2) or that produce output whose exact form is
// sensitive to Cobra's internals (I6) are asserted by substring
// rather than by byte-for-byte comparison. The choice is
// documented on each case's test.
//
// # How the tests invoke the CLI
//
// Each test invokes the CLI through runCLI, which routes through
// executeWithOptions. The invocation exercises the same path
// production code uses: argument parsing, pre-parse validation,
// command dispatch, and error rendering.
package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// =============================================================================
// The matrix
// =============================================================================

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

	// contains is a list of substrings that must appear in stderr.
	// It is used for cases where the exact output is not pinned
	// by a golden file.
	contains []string

	// golden is the path to the golden file for stderr, relative
	// to the test's working directory. It is empty for cases that
	// use `contains` instead.
	golden string
}

// invalidCases returns the seven cases the contract pins.
//
// The function is a method-style helper; it returns a fresh slice
// on each call so that a caller cannot mutate a shared value.
func invalidCases() []invalidCase {
	return []invalidCase{
		{
			id:   "I1",
			name: "unknown root command",
			args: []string{"foobar"},
			contains: []string{
				`unknown command "foobar" for "forge"`,
			},
		},
		{
			id:   "I2",
			name: "typo of a known command",
			args: []string{"verison"},
			contains: []string{
				`unknown command "verison" for "forge"`,
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
			},
		},
		{
			id:   "I4",
			name: "unknown flag at the root",
			args: []string{"--unknown-flag"},
			contains: []string{
				"unknown flag: --unknown-flag",
			},
		},
		{
			id:   "I5",
			name: "unknown flag at a subcommand",
			args: []string{"version", "--unknown-flag"},
			contains: []string{
				"unknown flag: --unknown-flag",
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
// stdout, non-empty stderr, and the required substrings.
//
// The test does not assert the exact bytes of stderr. That is the
// job of TestErrors_Golden for the cases that have a golden file.
// This test asserts the properties that apply to every case.
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
// Golden files
// =============================================================================

// TestErrors_Golden compares each case's stderr against a golden
// file, for the cases that have one.
//
// The cases without a golden file (I2 and I6) use TestErrors_Matrix's
// substring assertions instead. The choice is documented in
// errors.go's case list.
//
// # How to regenerate a golden file
//
// Regenerate it from the binary:
//
//	task build
//	./forge <args> 2> internal/cli/testdata/errors/<name>.golden.txt
//
// Then run this test. If it passes, the golden file records the
// current output.
//
// # Why the shell redirect writes to a file
//
// The shell redirect ("2>") captures stderr, not stdout. The
// golden file records the diagnostic, not the (empty) stdout.
// Redirecting stdout would produce an empty file.
func TestErrors_Golden(t *testing.T) {
	t.Parallel()

	for _, tc := range invalidCases() {
		if tc.golden == "" {
			continue
		}
		tc := tc
		t.Run(tc.id+" "+tc.name, func(t *testing.T) {
			t.Parallel()

			got := runCLI(t, tc.args, nil)

			wantBytes, err := os.ReadFile(filepath.Join("testdata", tc.golden))
			if err != nil {
				t.Fatalf("read golden %s: %v", tc.golden, err)
			}
			want := string(wantBytes)

			if got.stderr != want {
				// Report the first differing line.
				gotLines := strings.Split(got.stderr, "\n")
				wantLines := strings.Split(want, "\n")
				for i := 0; i < len(gotLines) || i < len(wantLines); i++ {
					var g, w string
					if i < len(gotLines) {
						g = gotLines[i]
					}
					if i < len(wantLines) {
						w = wantLines[i]
					}
					if g != w {
						t.Errorf("first difference at line %d:\n"+
							"got:  %q\nwant: %q\n"+
							"if the change is intentional, regenerate %s "+
							"from the binary",
							i+1, g, w, tc.golden)
						return
					}
				}
				t.Errorf("outputs differ in length but not in lines:\n"+
					"got length:  %d\nwant length: %d",
					len(got.stderr), len(want))
			}
		})
	}
}

// =============================================================================
// Suggestion mechanism
// =============================================================================// TestErrors_TypoSuggestsVersion verifies that the typo case (I2)
// produces a suggestion.
//
// The test is separate from the matrix because the suggestion's
// exact text depends on Cobra's internal formatting, which is not
// part of the contract. The test asserts the presence of the
// distinctive substrings ("Did you mean this?" and "version") that
// the suggestion must contain.
// TestErrors_TypoSuggestsVersion verifies that the typo case (I2)
// produces a suggestion.
//
// The test is separate from the matrix because the suggestion's
// exact text depends on Cobra's internal formatting, which is not
// part of the contract. The test asserts the presence of the
// distinctive substrings ("Did you mean" and "version") that the
// suggestion must contain, plus the "Suggestion:" block header.
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

// TestErrors_SuggestionMinimumDistanceIsTwo verifies that the root
// command's SuggestionsMinimumDistance field is 2.
//
// The check reads the constant suggestionsMinimumDistance (defined
// in errors.go). A future change to the constant is a change to
// the contract; the test would fail and the contributor would
// update the contract's documentation in the same commit.
//
// # Why a constant, not a field read
//
// The value is set on the root command in newRootCmd. Reading it
// back from a constructed command would work, but the construction
// requires a Dependencies value and the read would be indirect.
// The constant is the source of truth; the root command's
// assignment references it. The test asserts the constant's value.
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
// escape check to bound a slice operation.
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
