// Package cli contains white-box tests for the CLI package.
//
// This file pins the output stream boundary (WBS 7.4.1). The
// boundary specifies which streams receive which categories of
// output. Each test invokes a command through runCLI, captures the
// output, and asserts the boundary using assertOnlyStdout or
// assertOnlyStderr.
//
// # The boundary
//
// The boundary is documented in stream_boundary.go and in
// docs/cli-ux-spec.md "Output Stream Boundary". The decision rule
// for authors is:
//
//	If the user pipes this command's output to another program,
//	should that program receive this line?
//
// Yes → stdout. No → stderr.
//
// # What the tests assert
//
// Each test asserts one of three properties:
//
//   - assertOnlyStdout: output on stdout, nothing on stderr. Used
//     for successful commands whose output is a requested result.
//   - assertOnlyStderr: output on stderr, nothing on stdout. Used
//     for failing commands whose output is a diagnostic.
//   - A specific stream's content: used for commands whose output
//     is large enough to warrant a golden file.
//
// The tests do not assert the content of stdout or stderr for the
// boundary tests alone. Content is the job of each command's own
// tests. This file asserts only the stream.
package cli

import (
	"strings"
	"testing"
)

// =============================================================================
// Successful commands write to stdout only
// =============================================================================

// TestStreamBoundary_Version verifies that `forge version` writes
// its output to stdout and nothing to stderr.
//
// The version command's output is a requested result. A script
// that pipes it to another program expects the version block on
// stdout.
func TestStreamBoundary_Version(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"version"}, nil)

	if got.exitCode != ExitSuccess {
		t.Fatalf("exit code: got %d, want %d\nstderr: %s",
			got.exitCode, ExitSuccess, got.stderr)
	}
	assertOnlyStdout(t, got)
}

// TestStreamBoundary_VersionJSON verifies that
// `forge version --format json` writes its output to stdout and
// nothing to stderr.
//
// The JSON output is machine-readable; a script that pipes it to
// jq expects it on stdout.
func TestStreamBoundary_VersionJSON(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"version", "--format", "json"}, nil)

	if got.exitCode != ExitSuccess {
		t.Fatalf("exit code: got %d, want %d\nstderr: %s",
			got.exitCode, ExitSuccess, got.stderr)
	}
	assertOnlyStdout(t, got)

	// Sanity: the JSON output is a single line.
	if n := strings.Count(got.stdout, "\n"); n != 1 {
		t.Errorf("JSON output has %d newlines; want exactly 1\n%q",
			n, got.stdout)
	}
}

// TestStreamBoundary_RootHelp verifies that `forge --help` writes
// its output to stdout and nothing to stderr.
//
// Help is a requested result. A script that captures it expects it
// on stdout.
func TestStreamBoundary_RootHelp(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"--help"}, nil)

	if got.exitCode != ExitSuccess {
		t.Fatalf("exit code: got %d, want %d\nstderr: %s",
			got.exitCode, ExitSuccess, got.stderr)
	}
	assertOnlyStdout(t, got)
}

// TestStreamBoundary_HelpSubcommand verifies that `forge help`
// writes its output to stdout and nothing to stderr.
//
// The `help` subcommand is an alternative way to request help; the
// stream boundary is the same as for `forge --help`.
func TestStreamBoundary_HelpSubcommand(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"help"}, nil)

	if got.exitCode != ExitSuccess {
		t.Fatalf("exit code: got %d, want %d\nstderr: %s",
			got.exitCode, ExitSuccess, got.stderr)
	}
	assertOnlyStdout(t, got)
}

// TestStreamBoundary_VersionHelp verifies that
// `forge version --help` writes its output to stdout and nothing to
// stderr.
//
// The command's help is a requested result; it goes to stdout.
func TestStreamBoundary_VersionHelp(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"version", "--help"}, nil)

	if got.exitCode != ExitSuccess {
		t.Fatalf("exit code: got %d, want %d\nstderr: %s",
			got.exitCode, ExitSuccess, got.stderr)
	}
	assertOnlyStdout(t, got)
}

// TestStreamBoundary_Config verifies that `forge config` writes its
// diagnostic to stderr and nothing to stdout.
//
// The config command is a hidden placeholder. Its RunE handler
// returns an error rather than producing output, so the boundary
// rule applies in the "failing command" direction: the diagnostic
// goes to stderr, and stdout is empty.
//
// # Why the command is in the registry
//
// The config command is registered (registry.go) so that the
// command tree's shape is visible and so that future WBS items
// have a placeholder to replace. Its presence in the registry is
// what makes this test necessary: the self-referential check
// requires every registered command to have a boundary test.
//
// # What the test asserts
//
// The command's RunE returns a plain error. Cobra dispatches to
// the root's error path; executeWithOptions writes the rendered
// error to stderr. The test asserts the boundary using
// assertOnlyStderr, which checks that stderr is non-empty and
// stdout is empty.
//
// # The exit code
//
// The error is a plain error with no category. exitCodeFromError
// maps uncategorised errors to ExitUsage (2), the same code the
// pre-parse validator's errors and Cobra's own dispatch errors
// receive. The test asserts that exit code.
//
// # Why ExitUsage, not ExitFailure
//
// The exit codes and their meanings are defined in
// docs/cli-ux-spec.md § 7. ExitUsage (2) is the code for any
// error that does not carry a category: the user's invocation
// could not be interpreted, or the command that would interpret
// it is not available. The config command's "not yet implemented"
// error fits the second case.
//
// ExitFailure (1) is the code for a general failure that does not
// fit a more specific category. It is the fallback for a future
// error category the mapping does not yet recognise. The config
// placeholder does not use it.
//
// When WBS 8.x implements the config command, its errors will
// carry categories (for example, "config" for a malformed
// forge.yaml) and map to ExitConfig (3). This test's assertion
// is updated at that point; the boundary (stderr only, stdout
// empty) is unchanged.
func TestStreamBoundary_Config(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"config"}, nil)

	if got.exitCode != ExitUsage {
		t.Fatalf("exit code: got %d, want %d\nstderr: %s",
			got.exitCode, ExitUsage, got.stderr)
	}
	assertOnlyStderr(t, got)
}

// =============================================================================
// Failing commands write to stderr only
// =============================================================================

// TestStreamBoundary_UnknownCommand verifies that an unknown
// command writes its diagnostic to stderr and nothing to stdout.
//
// The diagnostic is a result of a failed invocation, not a
// successful result. A script that pipes the command's output
// expects nothing on stdout when the command fails.
func TestStreamBoundary_UnknownCommand(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{boundaryFixtureUnknownCommand}, nil)

	if got.exitCode != ExitUsage {
		t.Fatalf("exit code: got %d, want %d\nstderr: %s",
			got.exitCode, ExitUsage, got.stderr)
	}
	assertOnlyStderr(t, got)
}

// TestStreamBoundary_UnknownFormat verifies that an unknown --format
// value writes its diagnostic to stderr and nothing to stdout.
//
// The diagnostic is a result of a failed invocation. The command's
// stdout must remain empty so that a caller that captures stdout
// sees no output when the invocation fails.
func TestStreamBoundary_UnknownFormat(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{
		boundaryFixtureVersion,
		"--format",
		boundaryFixtureUnknownFormat,
	}, nil)

	if got.exitCode != ExitUsage {
		t.Fatalf("exit code: got %d, want %d\nstderr: %s",
			got.exitCode, ExitUsage, got.stderr)
	}
	assertOnlyStderr(t, got)
}

// TestStreamBoundary_ExtraArguments verifies that extra arguments
// to the version command write their diagnostic to stderr and
// nothing to stdout.
func TestStreamBoundary_ExtraArguments(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{boundaryFixtureVersion, "extra"}, nil)

	if got.exitCode != ExitUsage {
		t.Fatalf("exit code: got %d, want %d\nstderr: %s",
			got.exitCode, ExitUsage, got.stderr)
	}
	assertOnlyStderr(t, got)
}

// =============================================================================
// Global flags do not cross the boundary
// =============================================================================

// TestStreamBoundary_QuietDoesNotSuppressStdout verifies that
// `--quiet` does not suppress the version command's stdout.
//
// The --quiet flag affects the log level, not the command's own
// output. A successful command that writes to stdout continues to
// write to stdout even when --quiet is set.
func TestStreamBoundary_QuietDoesNotSuppressStdout(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"--quiet", "version"}, nil)

	if got.exitCode != ExitSuccess {
		t.Fatalf("exit code: got %d, want %d\nstderr: %s",
			got.exitCode, ExitSuccess, got.stderr)
	}
	if got.stdout == "" {
		t.Errorf("--quiet suppressed stdout; want stdout unchanged\n"+
			"stderr: %q", got.stderr)
	}
}

// TestStreamBoundary_VerboseDoesNotPolluteStdout verifies that
// `--verbose` adds no bytes to stdout.
//
// The --verbose flag raises the log level, which affects stderr.
// The command's stdout is unchanged by --verbose.
func TestStreamBoundary_VerboseDoesNotPolluteStdout(t *testing.T) {
	t.Parallel()

	plain := runCLI(t, []string{"version"}, nil)
	verbose := runCLI(t, []string{"--verbose", "version"}, nil)

	if plain.exitCode != ExitSuccess || verbose.exitCode != ExitSuccess {
		t.Fatalf("exit codes: plain=%d verbose=%d",
			plain.exitCode, verbose.exitCode)
	}

	if plain.stdout != verbose.stdout {
		t.Errorf("stdout differs with --verbose:\n"+
			"plain:   %q\nverbose: %q",
			plain.stdout, verbose.stdout)
	}
}

// =============================================================================
// The boundary applies to every command in the registry
// =============================================================================

// TestStreamBoundary_EveryCommandHasBoundaryTest verifies that
// every command in the registry has at least one test in this file
// that asserts its boundary.
//
// The check is structural: it reads this file's source and looks
// for a test name that mentions each command. The check is
// deliberately conservative; a test whose name mentions the
// command counts as a boundary test for it.
//
// # Why the check exists
//
// WBS 7.4.1's AC5 requires that every command has at least one
// test asserting the boundary. The check enforces the requirement
// mechanically. A future command added to the registry without a
// boundary test fails this check.
//
// # How to satisfy the check
//
// Add a test to this file whose name contains the command's name.
// The test's body should invoke the command and assert the
// boundary using assertOnlyStdout or assertOnlyStderr.
func TestStreamBoundary_EveryCommandHasBoundaryTest(t *testing.T) {
	t.Parallel()

	// Read this file's source. The test file is in the same
	// package as the production code; the helper readFile
	// (defined in version_test.go) reads a file by name.
	src := readFile(t, "stream_boundary_test.go")

	for _, cmd := range allRegisteredCommands() {
		name := commandName(cmd.Use)
		// Capitalise the first letter for the test-name match.
		// A test for the "version" command is named
		// TestStreamBoundary_Version; the name portion is
		// "Version".
		titleName := strings.Title(name) //nolint:staticcheck // simple ASCII title-case

		// Look for the substring "TestStreamBoundary_" + title
		// in the source.
		want := "TestStreamBoundary_" + titleName
		if !strings.Contains(src, want) {
			t.Errorf("command %q has no boundary test in this file; "+
				"add a test named %q and cite WBS 7.4.1",
				name, want)
		}
	}
}
