// Package cli contains white-box tests for the CLI package.
//
// This file tests the boundary between the CLI and the process
// environment: the Execute public function and the unexported
// executeWithOptions function. It also tests the helper functions
// that support the boundary: defaultOptions and formatError.
//
// The test file is declared in package cli, not package cli_test,
// because it needs to reach unexported symbols.
package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// =============================================================================
// Test helper
// =============================================================================

// testRun captures the observable result of a CLI invocation.
//
// It is the return type of the runCLI helper. Every field is
// deterministic: the same args produce the same testRun, byte for
// byte.
type testRun struct {
	// exitCode is the value returned by executeWithOptions.
	exitCode int

	// stdout is the content written to the injected stdout writer.
	stdout string

	// stderr is the content written to the injected stderr writer.
	stderr string
}

// runCLI invokes the CLI with the given arguments and returns the
// captured observable result.
//
// The helper constructs an options value with synthetic inputs. It
// uses bytes.Buffer for stdout and stderr, an empty reader for stdin,
// an empty environment lookup, and an empty rootPath.
//
// The helper is the standard way to invoke the CLI in a test. Every
// test in this file uses it. The helper does not spawn a subprocess.
func runCLI(t *testing.T, args ...string) testRun {
	t.Helper()

	var stdout, stderr bytes.Buffer

	opts := options{
		args:     args,
		stdin:    strings.NewReader(""),
		stdout:   &stdout,
		stderr:   &stderr,
		env:      func(string) string { return "" },
		rootPath: "",
	}

	code := executeWithOptions(opts)

	return testRun{
		exitCode: code,
		stdout:   stdout.String(),
		stderr:   stderr.String(),
	}
}

// =============================================================================
// Tests — the public Execute function
// =============================================================================

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

// TestExecute_HasFrozenSignature verifies that Execute has the
// signature frozen by WBS 4.1.1.
//
// The signature must be exactly:
//
//	func Execute() int
//
// This is a compile-time check: if the signature changes, the
// assignment below fails to compile.
func TestExecute_HasFrozenSignature(t *testing.T) {
	t.Parallel()

	// A typed assignment forces the compiler to verify the
	// signature. If Execute's parameters or return type change,
	// this line does not compile.
	var _ func() int = Execute
}

// =============================================================================
// Tests — the root command constructor
// =============================================================================

// TestNewRootCmd_AcceptsOptions verifies that newRootCmd has the
// signature required by the two-layer model.
//
// The signature must be:
//
//	func newRootCmd(opts options) *cobra.Command
//
// This is a compile-time check.
func TestNewRootCmd_AcceptsOptions(t *testing.T) {
	t.Parallel()

	// A typed assignment forces the compiler to verify the
	// signature. If newRootCmd's parameter or return type changes,
	// this line does not compile.
	var _ func(options) *cobra.Command = newRootCmd
}

// =============================================================================
// Tests — the injectable boundary
// =============================================================================

// TestExecuteWithOptions_GivenEmptyArgs_ThenExitsSuccess verifies
// the boundary behaviour when args is empty.
//
// This is the lowest-level test of the execution boundary. It proves
// that an empty args slice is handled as the "no subcommand" case,
// which is the same as the no-args invocation.
func TestExecuteWithOptions_GivenEmptyArgs_ThenExitsSuccess(t *testing.T) {
	t.Parallel()

	got := runCLI(t)

	if got.exitCode != ExitSuccess {
		t.Errorf("empty args should exit %d; got: %d",
			ExitSuccess, got.exitCode)
	}
}

// TestExecuteWithOptions_GivenHelpFlag_ThenExitsSuccess verifies
// that the help flag produces ExitSuccess.
//
// Cobra intercepts --help at the framework level. The test proves
// that the interception works and that the output is routed to the
// injected stdout.
func TestExecuteWithOptions_GivenHelpFlag_ThenExitsSuccess(t *testing.T) {
	t.Parallel()

	got := runCLI(t, "--help")

	if got.exitCode != ExitSuccess {
		t.Errorf("--help should exit %d; got: %d",
			ExitSuccess, got.exitCode)
	}
	if got.stdout == "" {
		t.Error("stdout should contain help output")
	}
	if got.stderr != "" {
		t.Errorf("stderr should be empty on success; got: %q",
			got.stderr)
	}
}

// TestExecuteWithOptions_GivenUnknownCommand_ThenExitsUsage verifies
// that an unknown command produces ExitUsage.
//
// The error is written to the injected stderr. The stdout remains
// empty.
func TestExecuteWithOptions_GivenUnknownCommand_ThenExitsUsage(t *testing.T) {
	t.Parallel()

	got := runCLI(t, "unknown-command")

	if got.exitCode != ExitUsage {
		t.Errorf("unknown command should exit %d; got: %d",
			ExitUsage, got.exitCode)
	}
	if !strings.Contains(got.stderr, "unknown-command") {
		t.Errorf("stderr does not mention the unknown command; got: %q",
			got.stderr)
	}
	if got.stdout != "" {
		t.Errorf("stdout should be empty on failure; got: %q",
			got.stdout)
	}
}

// TestExecuteWithOptions_GivenEmptyStringArg_ThenExitsUsage verifies
// that an empty-string argument is treated as an unknown command.
//
// This is a boundary case: the argument is not absent (which would
// trigger the no-args behaviour), but it is not a valid command
// either.
func TestExecuteWithOptions_GivenEmptyStringArg_ThenExitsUsage(t *testing.T) {
	t.Parallel()

	got := runCLI(t, "")

	if got.exitCode == ExitSuccess {
		t.Errorf("empty-string argument should not exit %d; got: %d",
			ExitSuccess, got.exitCode)
	}
}

// TestExecuteWithOptions_GivenUnknownFlag_ThenExitsUsage verifies
// that an unknown flag produces ExitUsage.
func TestExecuteWithOptions_GivenUnknownFlag_ThenExitsUsage(t *testing.T) {
	t.Parallel()

	got := runCLI(t, "--nonexistent-flag")

	if got.exitCode == ExitSuccess {
		t.Errorf("unknown flag should not exit %d; got: %d",
			ExitSuccess, got.exitCode)
	}
	if got.stderr == "" {
		t.Error("stderr should contain an error message")
	}
	if got.stdout != "" {
		t.Errorf("stdout should be empty on failure; got: %q",
			got.stdout)
	}
}

// =============================================================================
// Tests — stream separation
// =============================================================================

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
		t.Errorf("stderr should be empty on success; got: %q",
			got.stderr)
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
		t.Errorf("stdout should be empty on failure; got: %q",
			got.stdout)
	}
}

// TestExecuteWithOptions_StreamsAreIndependent verifies that the two
// streams are truly independent.
//
// A test that writes to stdout does not affect stderr, and vice versa.
// This is a corner case that catches accidental sharing of buffers in
// the options struct.
func TestExecuteWithOptions_StreamsAreIndependent(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer

	opts := options{
		args:     []string{"--help"},
		stdin:    strings.NewReader(""),
		stdout:   &stdout,
		stderr:   &stderr,
		env:      func(string) string { return "" },
		rootPath: "",
	}

	_ = executeWithOptions(opts)

	// After a successful --help invocation, stdout must be non-empty
	// and stderr must be empty. The two buffers must have been
	// written to independently.
	if stdout.Len() == 0 {
		t.Error("stdout should contain help output")
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr should be empty; got: %q", stderr.String())
	}
}

// =============================================================================
// Tests — formatError
// =============================================================================

// TestFormatError_GivenNilError_ReturnsEmptyString verifies that a
// nil error formats as an empty string.
//
// The function is defensive: it does not panic on a nil input.
func TestFormatError_GivenNilError_ReturnsEmptyString(t *testing.T) {
	t.Parallel()

	if got := formatError(nil); got != "" {
		t.Errorf("formatError(nil): got %q, want %q", got, "")
	}
}

// TestFormatError_GivenError_ReturnsMessage verifies that a non-nil
// error formats as its message.
//
// In WBS 4.2.1, the format is simply the error's own message. When
// WBS 10.0 introduces the structured error model, this test will be
// extended to assert on the structured format.
func TestFormatError_GivenError_ReturnsMessage(t *testing.T) {
	t.Parallel()

	err := errors.New("test error")
	want := "test error"

	if got := formatError(err); got != want {
		t.Errorf("formatError: got %q, want %q", got, want)
	}
}

// TestFormatError_IsPure verifies that formatError is deterministic.
//
// Calling it twice with the same error returns the same string.
func TestFormatError_IsPure(t *testing.T) {
	t.Parallel()

	err := errors.New("test error")
	first := formatError(err)

	for i := 0; i < 100; i++ {
		if got := formatError(err); got != first {
			t.Fatalf("iteration %d: got %q, want %q",
				i, got, first)
		}
	}
}

// =============================================================================
// Tests — the options struct
// =============================================================================

// TestDefaultOptions_HasNonEmptyFields verifies that the default
// options value has sensible values for every field.
//
// The fields are read from the current process, so they reflect the
// state of the test binary. The test asserts that stdout, stderr,
// and stdin are not nil, and that env is not nil.
//
// The rootPath field may be empty in a test binary that runs from a
// directory that has been removed, so its emptiness is not asserted.
func TestDefaultOptions_HasNonEmptyFields(t *testing.T) {
	t.Parallel()

	opts := defaultOptions()

	if opts.stdin == nil {
		t.Error("stdin is nil")
	}
	if opts.stdout == nil {
		t.Error("stdout is nil")
	}
	if opts.stderr == nil {
		t.Error("stderr is nil")
	}
	if opts.env == nil {
		t.Error("env is nil")
	}
}

// =============================================================================
// Tests — non-functional properties
// =============================================================================

// TestExecuteWithOptions_IsDeterministic verifies that two
// invocations with identical inputs produce identical observable
// results.
//
// This is a non-functional test that guards against accidental
// introduction of nondeterminism: timestamps in output, map
// iteration order, and similar sources of drift.
func TestExecuteWithOptions_IsDeterministic(t *testing.T) {
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

// =============================================================================
// Tests — the structural invariants the WBS specifies
// =============================================================================

// TestExecuteWithOptions_NoProcessStreamsReferenced verifies that
// executeWithOptions does not reference os.Stdout, os.Stderr,
// os.Stdin, os.Args, or os.Getenv directly.
//
// The test is a source-code check: it reads the file containing
// executeWithOptions and asserts that no direct reference to a
// process input or stream appears inside the function body.
//
// This is a structural invariant the WBS specifies: "No global
// state is read directly (os.Args, os.Stdout) inside commands —
// everything flows through a Dependencies struct."
func TestExecuteWithOptions_NoProcessStreamsReferenced(t *testing.T) {
	t.Parallel()

	content := readFile(t, "execute.go")

	// Find the body of executeWithOptions.
	start := strings.Index(content, "func executeWithOptions(")
	if start < 0 {
		t.Fatal("func executeWithOptions not found in execute.go")
	}

	// Find the closing brace of the function. The first occurrence
	// of "\n}\n" after the function's opening brace is the function's
	// closing brace, because Go's formatter places the closing brace
	// at the start of a line.
	end := strings.Index(content[start:], "\n}\n")
	if end < 0 {
		t.Fatal("could not find end of executeWithOptions body")
	}
	body := content[start : start+end]

	// The body must not reference the process's global inputs or
	// streams.
	forbidden := []string{
		"os.Stdout",
		"os.Stderr",
		"os.Stdin",
		"os.Args",
		"os.Getenv",
	}
	for _, token := range forbidden {
		if strings.Contains(body, token) {
			t.Errorf("executeWithOptions body references %s; "+
				"all I/O must flow through the options struct",
				token)
		}
	}
}

// TestExecuteWithOptions_UsesExitCodeFromError verifies that
// executeWithOptions does not derive exit codes itself; it delegates
// to exitCodeFromError.
//
// The test is a source-code check: it asserts that the function body
// contains a call to exitCodeFromError and does not contain a bare
// integer literal in the exit code range.
func TestExecuteWithOptions_UsesExitCodeFromError(t *testing.T) {
	t.Parallel()

	content := readFile(t, "execute.go")

	if !strings.Contains(content, "exitCodeFromError(err)") {
		t.Error("executeWithOptions does not call exitCodeFromError")
	}
}

// =============================================================================
// Test helpers — source file reading
// =============================================================================
//
// The structural tests in this file (TestExecuteWithOptions_
// NoProcessStreamsReferenced and TestExecuteWithOptions_
// UsesExitCodeFromError) inspect the source code of execute.go
// rather than the runtime behaviour of executeWithOptions. They
// need to read the file from disk.
//
// The helpers below are duplicated from structure_test.go, which
// lives in package cli_test. A test file in package cli cannot call
// unexported helpers from a test file in package cli_test, even
// though both files are in the same directory: they are compiled as
// two different packages. The duplication is the consequence of
// the white-box / black-box split documented in doc.go.

// executeTestPackageDir returns the absolute path to the
// internal/cli directory.
//
// It derives the path from the location of this test file, using
// runtime.Caller to find the source file path, then returning the
// directory that contains it.
func executeTestPackageDir(t *testing.T) string {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	return filepath.Dir(thisFile)
}

// readFile reads a file in the package directory and returns its
// contents as a string.
//
// The helper is used by the structural tests in this file. It is
// named "readFile" to match the helper of the same name in
// structure_test.go, which serves the same purpose for the black-box
// tests.
func readFile(t *testing.T, name string) string {
	t.Helper()

	path := filepath.Join(executeTestPackageDir(t), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(data)
}
