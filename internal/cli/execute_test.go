// Package cli contains white-box tests for the CLI package.
//
// This file tests the boundary between the CLI and the process
// environment: the Execute public function and the unexported
// executeWithOptions function.
//
// The test file is declared in package cli, not package cli_test,
// because it needs to reach unexported symbols.
package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestExecute_IsCallable verifies that the exported Execute function
// can be invoked without panicking.
//
// This is the lightest possible test of the public surface. Its value
// is that it proves the symbol exists and its signature matches
// expectations.
//
// Note: this test calls the real Execute, which reads os.Args. The
// test binary's own arguments are treated by Cobra as an unknown
// command, so Execute returns a non-zero code. The test asserts the
// absence of a panic, not the exit code.
func TestExecute_IsCallable(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Execute panicked: %v", r)
		}
	}()

	_ = Execute()
}

// =============================================================================
// Boundary tests for executeWithOptions
// =============================================================================

// TestExecuteWithOptions_GivenEmptyArgs_ThenExitsZero verifies the
// boundary behaviour when args is empty.
//
// This is the lowest-level test of the execution boundary. It proves
// that an empty args slice is handled as the "no subcommand" case,
// which is the same as the no-args invocation.
func TestExecuteWithOptions_GivenEmptyArgs_ThenExitsZero(t *testing.T) {
	t.Parallel()

	got := runCLI(t)

	if got.exitCode != ExitSuccess {
		t.Errorf("empty args should exit %d; got: %d",
			ExitSuccess, got.exitCode)
	}
}

// TestExecuteWithOptions_WritesNothingToStderrOnSuccess verifies that
// a successful invocation writes nothing to stderr.
//
// This is a boundary condition for the stdout/stderr separation
// contract. Even a stray newline on stderr would violate the contract
// and break shell scripts that redirect stderr to a log.
func TestExecuteWithOptions_WritesNothingToStderrOnSuccess(t *testing.T) {
	t.Parallel()

	got := runCLI(t, "--help")

	if got.stderr != "" {
		t.Errorf("stderr should be empty on success; got: %q", got.stderr)
	}
}

// TestExecuteWithOptions_WritesNothingToStdoutOnFailure verifies that
// a failing invocation writes nothing to stdout.
//
// The symmetric boundary condition: when the CLI fails, the diagnostic
// goes to stderr, and stdout remains empty.
func TestExecuteWithOptions_WritesNothingToStdoutOnFailure(t *testing.T) {
	t.Parallel()

	got := runCLI(t, "unknown-command")

	if got.stdout != "" {
		t.Errorf("stdout should be empty on failure; got: %q", got.stdout)
	}
}

// TestExecuteWithOptions_StderrIsIndependentOfStdout verifies that the
// two streams are truly independent.
//
// A test that writes to stdout does not affect stderr, and vice versa.
// This is a corner case that catches accidental sharing of buffers in
// the options struct.
func TestExecuteWithOptions_StderrIsIndependentOfStdout(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer

	opts := options{
		args:   []string{"--help"},
		stdin:  strings.NewReader(""),
		stdout: &stdout,
		stderr: &stderr,
		env:    func(string) string { return "" },
	}

	_ = executeWithOptions(opts)

	// After a successful --help invocation, stdout must be non-empty
	// and stderr must be empty. The two buffers must have been written
	// to independently.
	if stdout.Len() == 0 {
		t.Error("stdout should contain help output")
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr should be empty; got: %q", stderr.String())
	}
}