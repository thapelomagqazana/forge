// Package cli contains white-box tests for the CLI package.
//
// These tests exercise the CLI through executeWithOptions with
// injectable inputs and assert on the captured output and exit code.
// They do not spawn a subprocess.
//
// The test file is declared in package cli, not package cli_test,
// because it needs to reach unexported symbols (executeWithOptions,
// options). This is the idiomatic Go pattern for testing a CLI
// package's behaviour. Structural tests (file layout, import
// discipline, exported surface) live in structure_test.go and are
// declared in package cli_test because they do not need internals.
//
// Test organisation:
//
//   - The runCLI helper at the top of this file is the standard way
//     to invoke the CLI in a test. It is used by every test in the
//     file.
//
//   - Test names follow the BDD convention
//     TestXxx_GivenY_ThenZ, which makes the precondition, action,
//     and expected outcome explicit.
package cli

import (
	"bytes"
	"strings"
	"testing"
)

// cliRun captures the observable result of a single CLI invocation.
//
// It is the return type of the runCLI helper. Every field is
// deterministic: the same args and env produce the same cliRun,
// byte for byte.
type cliRun struct {
	// exitCode is the value returned by the CLI.
	exitCode int

	// stdout is the content written to the injected stdout writer.
	stdout string

	// stderr is the content written to the injected stderr writer.
	stderr string
}

// runCLI invokes the CLI with the given arguments and returns the
// captured observable result.
//
// The helper:
//
//   - Constructs an injectable environment with the given args.
//   - Uses empty readers and buffers for stdin, stdout, and stderr.
//   - Returns the exit code and the contents of stdout and stderr.
//
// Environment variable lookups are not supported by this helper in
// WBS 2.4.2. When WBS 8.x introduces configuration, the helper will
// be extended to accept an env map.
//
// The helper is deliberately unexported and unparameterised beyond
// args. Tests that need to inject additional state should construct
// the options struct directly; this is rarely necessary in Phase 2.
func runCLI(t *testing.T, args ...string) cliRun {
	t.Helper()

	var stdout, stderr bytes.Buffer

	opts := options{
		args:   args,
		stdin:  strings.NewReader(""),
		stdout: &stdout,
		stderr: &stderr,
		env:    func(string) string { return "" },
	}

	code := executeWithOptions(opts)

	return cliRun{
		exitCode: code,
		stdout:   stdout.String(),
		stderr:   stderr.String(),
	}
}

// =============================================================================
// Smoke test 1 — forge (no arguments)
// =============================================================================

// TestRoot_GivenNoArgs_ThenExitsZeroAndPrintsUsage verifies AC1:
// running forge with no arguments prints usage to stdout and exits
// with a defined code.
//
// The "defined code" is ExitSuccess (0). This is the contract every
// well-behaved CLI follows: running the command with no arguments is
// not an error; it is a request for information.
func TestRoot_GivenNoArgs_ThenExitsZeroAndPrintsUsage(t *testing.T) {
	t.Parallel()

	got := runCLI(t)

	if got.exitCode != ExitSuccess {
		t.Errorf("exit code: got %d, want %d", got.exitCode, ExitSuccess)
	}

	// The root command prints its Long description when invoked with
	// no subcommand. The description must appear on stdout.
	if !strings.Contains(got.stdout, "Forge") {
		t.Errorf("stdout does not contain the root command description; "+
			"got: %q", got.stdout)
	}

	// Usage output is a successful result, not an error. Nothing
	// should be written to stderr.
	if got.stderr != "" {
		t.Errorf("stderr should be empty on success; got: %q", got.stderr)
	}
}

// =============================================================================
// Smoke test 2 — forge --help
// =============================================================================

// TestRoot_GivenHelpFlag_ThenExitsZeroAndPrintsHelpToStdout verifies
// AC2: running forge --help prints help to stdout and exits 0.
//
// Cobra intercepts --help at the framework level. The test proves
// that the interception works and that the output is routed to the
// injected stdout.
func TestRoot_GivenHelpFlag_ThenExitsZeroAndPrintsHelpToStdout(t *testing.T) {
	t.Parallel()

	got := runCLI(t, "--help")

	if got.exitCode != ExitSuccess {
		t.Errorf("exit code: got %d, want %d", got.exitCode, ExitSuccess)
	}

	// Cobra's help output begins with the command's usage line and
	// includes the Long description.
	if !strings.Contains(got.stdout, "forge") {
		t.Errorf("stdout does not contain 'forge'; got: %q", got.stdout)
	}
	if !strings.Contains(got.stdout, "Forge") {
		t.Errorf("stdout does not contain the Long description; "+
			"got: %q", got.stdout)
	}

	// Help output is a successful result. Nothing should be written
	// to stderr.
	if got.stderr != "" {
		t.Errorf("stderr should be empty when --help succeeds; "+
			"got: %q", got.stderr)
	}
}

// =============================================================================
// Smoke test 3 — forge unknown-cmd
// =============================================================================

// TestRoot_GivenUnknownCommand_ThenExitsUsageAndPrintsErrorToStderr
// verifies AC3: running forge with an unknown command prints an error
// to stderr and exits with a non-zero code.
//
// The expected exit code is ExitUsage (2). This is the value Cobra
// returns for its built-in usage errors. exitCodeFromError maps all
// errors to ExitUsage in WBS 2.4.2.
func TestRoot_GivenUnknownCommand_ThenExitsUsageAndPrintsErrorToStderr(t *testing.T) {
	t.Parallel()

	got := runCLI(t, "unknown-command")

	if got.exitCode != ExitUsage {
		t.Errorf("exit code: got %d, want %d (ExitUsage)",
			got.exitCode, ExitUsage)
	}

	// Cobra writes the error message to stderr. The message mentions
	// the unknown command.
	if !strings.Contains(got.stderr, "unknown-command") {
		t.Errorf("stderr does not mention the unknown command; "+
			"got: %q", got.stderr)
	}

	// Successful output goes to stdout; error output goes to stderr.
	// The two must not be mixed.
	if got.stdout != "" {
		t.Errorf("stdout should be empty when an error occurs; "+
			"got: %q", got.stdout)
	}
}

// =============================================================================
// Non-functional cases
// =============================================================================

// TestRoot_RepeatedInvocationsAreDeterministic verifies that two
// invocations with identical inputs produce byte-identical outputs.
//
// This is a non-functional test that guards against accidental
// introduction of nondeterminism: timestamps in output, map iteration
// order, and similar sources of drift.
func TestRoot_RepeatedInvocationsAreDeterministic(t *testing.T) {
	t.Parallel()

	first := runCLI(t, "--help")

	for i := 0; i < 20; i++ {
		got := runCLI(t, "--help")
		if got.exitCode != first.exitCode {
			t.Fatalf("run %d: exit code differs: got %d, want %d",
				i, got.exitCode, first.exitCode)
		}
		if got.stdout != first.stdout {
			t.Fatalf("run %d: stdout differs", i)
		}
		if got.stderr != first.stderr {
			t.Fatalf("run %d: stderr differs", i)
		}
	}
}

// TestRoot_NoArgs_And_HelpFlag_ProduceSameExitCode verifies that the
// two informational invocations — no args and --help — are consistent
// in their exit code.
//
// The content of the output may differ (Cobra formats help slightly
// differently depending on how it is triggered), but the exit code
// must be identical. A CLI that exits 0 for one and 2 for the other
// is inconsistent.
func TestRoot_NoArgs_And_HelpFlag_ProduceSameExitCode(t *testing.T) {
	t.Parallel()

	noArgs := runCLI(t)
	helpFlag := runCLI(t, "--help")

	if noArgs.exitCode != helpFlag.exitCode {
		t.Errorf("exit code differs: no-args=%d, --help=%d",
			noArgs.exitCode, helpFlag.exitCode)
	}
	if noArgs.exitCode != ExitSuccess {
		t.Errorf("expected both to exit %d; got no-args=%d, --help=%d",
			ExitSuccess, noArgs.exitCode, helpFlag.exitCode)
	}
}

// =============================================================================
// Edge cases
// =============================================================================

// TestRoot_GivenEmptyStringArgument_ThenExitsUsage verifies that an
// empty-string argument is treated as an unknown command.
//
// This is a boundary case: the argument is not absent (which would
// trigger the no-args behaviour), but it is not a valid command
// either.
func TestRoot_GivenEmptyStringArgument_ThenExitsUsage(t *testing.T) {
	t.Parallel()

	got := runCLI(t, "")

	if got.exitCode == ExitSuccess {
		t.Errorf("expected non-zero exit code for empty-string argument; "+
			"got: %d", got.exitCode)
	}
}

// TestRoot_GivenUnknownFlag_ThenExitsUsage verifies that an unknown
// flag is treated as a usage error.
//
// This is distinct from an unknown command. Cobra handles both, but
// the error messages differ. The exit code is the same.
func TestRoot_GivenUnknownFlag_ThenExitsUsage(t *testing.T) {
	t.Parallel()

	got := runCLI(t, "--nonexistent-flag")

	if got.exitCode == ExitSuccess {
		t.Errorf("expected non-zero exit code for unknown flag; "+
			"got: %d", got.exitCode)
	}
	if got.stderr == "" {
		t.Errorf("expected an error message on stderr")
	}
	if got.stdout != "" {
		t.Errorf("stdout should be empty when an error occurs; "+
			"got: %q", got.stdout)
	}
}
