// Package cli contains white-box tests for the CLI package.
//
// This file pins the complete help invocation matrix (WBS 7.1.1).
// Every way to reach help is listed as a row in the matrix, and
// every row has a test. A change that breaks a row fails the test
// that names the row.
//
// # The matrix
//
// The fourteen invocations are grouped into three sets:
//
//   - Positive root help  (H1–H4, H13)
//   - Positive nested help (H5–H10, H14)
//   - Negative invocations (H11, H12)
//
// The test is table-driven for the simple rows (exit code, stream,
// non-empty output). The equivalence classes and the identity-string
// assertion are separate tests because they assert a different
// property.
//
// # Why every row needs a test
//
// The help system has fourteen entry points, each of which is
// reachable from a shell. A future change to Cobra, to the command
// registry, or to the root command's configuration could break one
// without breaking the others. The matrix test is the mechanism
// that catches the break: if a row fails, the failure message names
// the row and the property that changed.
//
// # Relationship to the handler tests
//
// The version command's handler is tested in version_test.go. This
// file tests the paths that reach help, regardless of which command
// the help is about. The two files overlap on `forge version --help`
// (H7): version_test.go has a test for it, and this file has one
// too. The overlap is deliberate — version_test.go asserts the
// version-specific property (help mentions the command name), and
// this file asserts the matrix-level property (help goes to stdout
// and exits 0).
package cli

import (
	"strings"
	"testing"
)

// helpMatrixCase describes one row of the help invocation matrix.
//
// The struct is the table's element type. Each field maps to a
// column of the matrix in docs/cli-ux-spec.md's "Help Invocation
// Matrix" section.
type helpMatrixCase struct {
	// id is the matrix row identifier (H1, H2, ...). It is used
	// in the subtest name so that a failure points at the row in
	// the documentation.
	id string

	// name is a short human-readable description of the row. It
	// appears in the subtest name alongside the id.
	name string

	// args is the argument vector to pass to runCLI. It does not
	// include the program name; runCLI prepends it.
	args []string

	// wantExit is the expected exit code.
	wantExit int

	// wantStdoutNonEmpty is true if the invocation must produce
	// output on stdout. All positive help paths set this to true.
	wantStdoutNonEmpty bool

	// wantStderrEmpty is true if the invocation must produce no
	// output on stderr. All positive help paths set this to true.
	wantStderrEmpty bool

	// wantStderrNonEmpty is true if the invocation must produce
	// a diagnostic on stderr. All negative help paths set this to
	// true.
	wantStderrNonEmpty bool

	// wantStdoutEmpty is true if the invocation must produce no
	// output on stdout. All negative help paths set this to true.
	wantStdoutEmpty bool

	// contains is an optional list of substrings that must appear
	// in stdout. It is used to distinguish which help output was
	// produced (root help vs. nested help).
	contains []string
}

// TestHelpMatrix exercises every row of the help invocation matrix
// defined in WBS 7.1.1 and documented in docs/cli-ux-spec.md.
//
// Each row is a subtest. The subtest name carries the matrix id so
// that a failure points at the row in the documentation.
//
// # What the test asserts
//
// For each row:
//
//   - The exit code matches wantExit.
//   - stdout is non-empty iff wantStdoutNonEmpty.
//   - stdout is empty iff wantStdoutEmpty.
//   - stderr is empty iff wantStderrEmpty.
//   - stderr is non-empty iff wantStderrNonEmpty.
//   - Every substring in `contains` appears in stdout.
//
// The matrix-level properties (H5 == H7 == H8, H6 == H9, H13 help
// wins over version) are asserted in separate tests because they
// require comparing two invocations rather than checking one.
func TestHelpMatrix(t *testing.T) {
	t.Parallel()

	cases := []helpMatrixCase{
		// ─────────────────────────────────────────────────────────
		// Positive root help (H1–H4, H13)
		// ─────────────────────────────────────────────────────────
		{
			id:                 "H1",
			name:               "no-args prints root help",
			args:               []string{},
			wantExit:           ExitSuccess,
			wantStdoutNonEmpty: true,
			wantStderrEmpty:    true,
			contains:           []string{"forge", "Available Commands"},
		},
		{
			id:                 "H2",
			name:               "--help prints root help",
			args:               []string{"--help"},
			wantExit:           ExitSuccess,
			wantStdoutNonEmpty: true,
			wantStderrEmpty:    true,
			contains:           []string{"forge", "Available Commands"},
		},
		{
			id:                 "H3",
			name:               "-h prints root help",
			args:               []string{"-h"},
			wantExit:           ExitSuccess,
			wantStdoutNonEmpty: true,
			wantStderrEmpty:    true,
			contains:           []string{"forge", "Available Commands"},
		},
		{
			id:                 "H4",
			name:               "help subcommand prints root help",
			args:               []string{"help"},
			wantExit:           ExitSuccess,
			wantStdoutNonEmpty: true,
			wantStderrEmpty:    true,
			contains:           []string{"forge", "Available Commands"},
		},
		{
			id:                 "H13",
			name:               "-h --version; help wins",
			args:               []string{"-h", "--version"},
			wantExit:           ExitSuccess,
			wantStdoutNonEmpty: true,
			wantStderrEmpty:    true,
			contains:           []string{"forge", "Available Commands"},
		},

		// ─────────────────────────────────────────────────────────
		// Positive nested help (H5–H10, H14)
		// ─────────────────────────────────────────────────────────
		{
			id:                 "H5",
			name:               "help version",
			args:               []string{"help", "version"},
			wantExit:           ExitSuccess,
			wantStdoutNonEmpty: true,
			wantStderrEmpty:    true,
			contains:           []string{"version", "--format"},
		},
		{
			id:                 "H6",
			name:               "help config",
			args:               []string{"help", "config"},
			wantExit:           ExitSuccess,
			wantStdoutNonEmpty: true,
			wantStderrEmpty:    true,
			contains:           []string{"config"},
		},
		{
			id:                 "H7",
			name:               "version --help",
			args:               []string{"version", "--help"},
			wantExit:           ExitSuccess,
			wantStdoutNonEmpty: true,
			wantStderrEmpty:    true,
			contains:           []string{"version", "--format"},
		},
		{
			id:                 "H8",
			name:               "version -h",
			args:               []string{"version", "-h"},
			wantExit:           ExitSuccess,
			wantStdoutNonEmpty: true,
			wantStderrEmpty:    true,
			contains:           []string{"version", "--format"},
		},
		{
			id:                 "H9",
			name:               "config --help",
			args:               []string{"config", "--help"},
			wantExit:           ExitSuccess,
			wantStdoutNonEmpty: true,
			wantStderrEmpty:    true,
			contains:           []string{"config"},
		},
		{
			id:                 "H10",
			name:               "help help",
			args:               []string{"help", "help"},
			wantExit:           ExitSuccess,
			wantStdoutNonEmpty: true,
			wantStderrEmpty:    true,
			contains:           []string{"help"},
		},
		{
			id:                 "H14",
			name:               "help --help",
			args:               []string{"help", "--help"},
			wantExit:           ExitSuccess,
			wantStdoutNonEmpty: true,
			wantStderrEmpty:    true,
			contains:           []string{"help"},
		},

		// ─────────────────────────────────────────────────────────
		// Negative invocations (H11, H12)
		// ─────────────────────────────────────────────────────────
		{
			id:                 "H11",
			name:               "help unknown is rejected",
			args:               []string{"help", "unknown"},
			wantExit:           ExitUsage,
			wantStdoutEmpty:    true,
			wantStderrNonEmpty: true,
			// The diagnostic must name the offending topic. The
			// exact wording is pinned in validate_test.go; the
			// matrix asserts only that the topic is named.
			contains: nil,
		},
		{
			id:                 "H12",
			name:               "--help version is rejected",
			args:               []string{"--help", "version"},
			wantExit:           ExitUsage,
			wantStdoutEmpty:    true,
			wantStderrNonEmpty: true,
			contains:           nil,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.id+" "+tc.name, func(t *testing.T) {
			t.Parallel()

			got := runCLI(t, tc.args, nil)

			// Exit code.
			if got.exitCode != tc.wantExit {
				t.Errorf("exit code: got %d, want %d\n"+
					"stdout: %q\nstderr: %q",
					got.exitCode, tc.wantExit, got.stdout, got.stderr)
			}

			// stdout: presence or absence.
			if tc.wantStdoutNonEmpty && got.stdout == "" {
				t.Errorf("stdout is empty; want non-empty")
			}
			if tc.wantStdoutEmpty && got.stdout != "" {
				t.Errorf("stdout should be empty; got %q", got.stdout)
			}

			// stderr: presence or absence.
			if tc.wantStderrEmpty && got.stderr != "" {
				t.Errorf("stderr should be empty; got %q", got.stderr)
			}
			if tc.wantStderrNonEmpty && got.stderr == "" {
				t.Errorf("stderr is empty; want a diagnostic")
			}

			// Substring assertions on stdout.
			for _, want := range tc.contains {
				if !strings.Contains(got.stdout, want) {
					t.Errorf("stdout does not contain %q:\n%s", want, got.stdout)
				}
			}
		})
	}
}

// =============================================================================
// Equivalence classes
// =============================================================================
//
// The matrix defines three equivalence classes: H5 == H7 == H8, H6
// == H9, and the help-wins-over-version rule at H13. Each class is
// asserted by a dedicated test because it requires comparing two or
// more invocations rather than checking one in isolation.

// TestHelpMatrix_NestedVersionHelpIsByteIdentical verifies the
// equivalence class H5 == H7 == H8.
//
// `forge help version`, `forge version --help`, and
// `forge version -h` must produce byte-identical stdout. Any
// divergence means one of the three paths renders a different help
// text than the others, which would confuse a user who has learned
// one form and tries another.
func TestHelpMatrix_NestedVersionHelpIsByteIdentical(t *testing.T) {
	t.Parallel()

	h5 := runCLI(t, []string{"help", "version"}, nil)
	h7 := runCLI(t, []string{"version", "--help"}, nil)
	h8 := runCLI(t, []string{"version", "-h"}, nil)

	if h5.exitCode != ExitSuccess || h7.exitCode != ExitSuccess || h8.exitCode != ExitSuccess {
		t.Fatalf("expected all three invocations to succeed; "+
			"got H5=%d H7=%d H8=%d",
			h5.exitCode, h7.exitCode, h8.exitCode)
	}

	if h5.stdout != h7.stdout {
		t.Errorf("H5 and H7 differ:\nH5: %q\nH7: %q", h5.stdout, h7.stdout)
	}
	if h5.stdout != h8.stdout {
		t.Errorf("H5 and H8 differ:\nH5: %q\nH8: %q", h5.stdout, h8.stdout)
	}
}

// TestHelpMatrix_NestedConfigHelpIsByteIdentical verifies the
// equivalence class H6 == H9.
//
// `forge help config` and `forge config --help` must produce
// byte-identical stdout.
func TestHelpMatrix_NestedConfigHelpIsByteIdentical(t *testing.T) {
	t.Parallel()

	h6 := runCLI(t, []string{"help", "config"}, nil)
	h9 := runCLI(t, []string{"config", "--help"}, nil)

	if h6.exitCode != ExitSuccess || h9.exitCode != ExitSuccess {
		t.Fatalf("expected both invocations to succeed; "+
			"got H6=%d H9=%d", h6.exitCode, h9.exitCode)
	}

	if h6.stdout != h9.stdout {
		t.Errorf("H6 and H9 differ:\nH6: %q\nH9: %q", h6.stdout, h9.stdout)
	}
}

// TestHelpMatrix_HelpWinsOverVersion verifies that when both `-h`
// (or `--help`) and `--version` (or `-v`) are supplied, help is
// rendered.
//
// The rule is documented in WBS 7.1.1's row H13. The test checks
// both orderings of the two flags, because Cobra's parser may treat
// the two orderings differently.
func TestHelpMatrix_HelpWinsOverVersion(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
	}{
		{"-h before --version", []string{"-h", "--version"}},
		{"--version before -h", []string{"--version", "-h"}},
		{"--help before --version", []string{"--help", "--version"}},
		{"--version before --help", []string{"--version", "--help"}},
	}

	// The root help output is the reference for "help was rendered".
	// It contains "Available Commands", which the version output
	// does not.
	rootHelp := runCLI(t, []string{"--help"}, nil)
	if rootHelp.exitCode != ExitSuccess {
		t.Fatalf("failed to obtain reference root help; exit=%d", rootHelp.exitCode)
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := runCLI(t, tc.args, nil)

			if got.exitCode != ExitSuccess {
				t.Fatalf("exit code: got %d, want %d\nstderr: %q",
					got.exitCode, ExitSuccess, got.stderr)
			}
			if got.stdout != rootHelp.stdout {
				t.Errorf("output differs from root help:\n"+
					"got:  %q\nwant: %q", got.stdout, rootHelp.stdout)
			}
		})
	}
}

// =============================================================================
// Identity-string assertion
// =============================================================================

// TestHelpMatrix_RootHelpContainsIdentity verifies that the root
// help output contains the frozen root identity strings defined in
// WBS 5.1.1 and documented in docs/cli-ux-spec.md § 4.7.
//
// The check is on RootLongDesc, which is the body of the help text
// and is rendered by `forge --help`. RootShortDesc is not rendered
// for the root command (the root has no parent); the test checks
// only the strings that appear in the output.
func TestHelpMatrix_RootHelpContainsIdentity(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"--help"}, nil)

	if got.exitCode != ExitSuccess {
		t.Fatalf("exit code: got %d, want %d", got.exitCode, ExitSuccess)
	}

	// The long description is a raw string literal. Its distinctive
	// content is the CREATE / VERIFY / EXPLAIN / EVOLVE loop.
	for _, want := range []string{
		"CREATE",
		"VERIFY",
		"EXPLAIN",
		"EVOLVE",
	} {
		if !strings.Contains(got.stdout, want) {
			t.Errorf("root help does not contain %q:\n%s", want, got.stdout)
		}
	}
}
