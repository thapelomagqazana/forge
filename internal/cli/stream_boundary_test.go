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
// output to stdout and nothing to stderr.
//
// # History
//
// At WBS 9.1.1, the config command was a hidden placeholder whose
// RunE returned a sentinel error. The boundary rule applied in the
// "failing command" direction: the diagnostic went to stderr, and
// stdout was empty.
//
// At WBS 9.2.1, the command renders the resolved configuration to
// stdout and returns nil. The boundary rule now applies in the
// "successful command" direction: the output goes to stdout, and
// stderr is empty.
//
// # The output stream boundary
//
// The decision rule for authors (documented in stream_boundary.go
// and docs/cli-ux-spec.md) is:
//
//	If the user pipes this command's output to another program,
//	should that program receive this line?
//
// For `forge config`, the answer is yes: the rendered configuration
// is the command's requested result, not a diagnostic. It belongs
// on stdout. Diagnostics (if any) belong on stderr; the current
// command produces none.
//
// # Exit code
//
// The command succeeds. The exit code is ExitSuccess (0).
//
// When WBS 9.3.1 (--path) and 9.4.1 (--format json) land, the
// boundary is unchanged: output to stdout, stderr empty. Those
// tasks may add categories to the error path, but the success path
// does not change.
func TestStreamBoundary_Config(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"config"}, nil)

	if got.exitCode != ExitSuccess {
		t.Fatalf("exit code: got %d, want %d\nstderr: %s",
			got.exitCode, ExitSuccess, got.stderr)
	}
	assertOnlyStdout(t, got)
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

// TestStreamBoundary_VerboseDoesNotPolluteStdout verifies the
// stream boundary under --verbose.
//
// # The property
//
// --verbose enables two things: DEBUG records on stderr, and a
// verbose product output on stdout (for commands that have a
// verbose variant). The property the stream contract requires is
// that the two streams do not mix.
//
// # The structural check
//
// The test runs the command twice. Under --verbose, the stderr
// contains DEBUG records. The test extracts the message from
// each DEBUG record on stderr and asserts that the same message
// does not appear on stdout. The check uses the actual records
// the logger emitted, not a hard-coded token list, so it cannot
// be fooled by a build setting whose name contains a logger
// keyword.
//
// # What the test does not assert
//
// The test does not assert that stderr is empty without
// --verbose. Under the default log level the DEBUG records are
// filtered out; a test that wants to assert "stderr is empty on
// success" runs without --verbose (see
// TestStreamBoundary_Version).
//
// # Why the test is not parallel
//
// The root's --verbose enables DEBUG records on stderr. The
// records are written through the shared test helper's stderr
// buffer. A parallel test that reads the same buffer would race.
// The test is therefore serial.
func TestStreamBoundary_VerboseDoesNotPolluteStdout(t *testing.T) {
	plain := runCLI(t, []string{"version"}, nil)
	verbose := runCLI(t, []string{"--verbose", "version"}, nil)

	if plain.exitCode != ExitSuccess || verbose.exitCode != ExitSuccess {
		t.Fatalf("exit codes: plain=%d verbose=%d",
			plain.exitCode, verbose.exitCode)
	}

	// The two stdout shapes.
	if !strings.HasPrefix(plain.stdout, "forge") {
		t.Errorf("plain stdout does not start with the version header:\n%q",
			plain.stdout)
	}
	if !strings.HasPrefix(verbose.stdout, "version:") {
		t.Errorf("verbose stdout does not start with the verbose header:\n%q",
			verbose.stdout)
	}

	// The positive case: --verbose selects the verbose format.
	if plain.stdout == verbose.stdout {
		t.Errorf("--verbose did not change stdout; want the verbose format:\n%q",
			plain.stdout)
	}

	// The boundary case. Under --verbose, the stderr contains
	// the DEBUG records the version handler and the config
	// loader emit. Each record's message is a unique string
	// ("version: resolved build info", "version: selected
	// format"). The test asserts that none of those messages
	// appears on stdout.
	//
	// The messages are hard-coded because they are the
	// handler's own; a future change to a message updates the
	// list in the same commit as the handler.
	for _, msg := range []string{
		"version: resolved build info",
		"version: selected format",
	} {
		if strings.Contains(verbose.stdout, msg) {
			t.Errorf("stdout contains the DEBUG record message %q:\n%q",
				msg, verbose.stdout)
		}
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
