// Package cli contains white-box tests for the CLI package.
//
// This file tests the boundary between the CLI and the process
// environment: the Execute public function, the unexported
// executeWithOptions function, and the helper functions that
// support the boundary: defaultOptions, exitCodeOf,
// recoveredPanicError, wrappedUnexpectedError, and hasVerboseArg.
//
// The test file is declared in package cli, not package cli_test,
// because it needs to reach unexported symbols.
//
// # Relationship to other test files
//
//   - deps_test.go tests the Dependencies struct itself: its
//     construction, its field contracts, and the Logger interface.
//     A reader looking for "how does the CLI get its collaborators"
//     should start there.
//
//   - root_test.go tests the root command's dispatcher behaviour
//     (no-args, --help, unknown-command, unknown-flag). A reader
//     looking for "what does the user see" should start there.
//
//   - version_test.go tests the version command.
//
//   - testhelper_test.go defines the runCLI helper used by every
//     white-box test in this package.
//
//   - structure_test.go (package cli_test) enforces the package's
//     shape: file layout, import discipline, exported surface.
//
//   - This file tests the process-boundary mechanics: how a process
//     invocation becomes an options value, how options becomes
//     Dependencies, and how a Dependencies value becomes observable
//     behaviour.
//
// # The two-boundary model
//
// After WBS 4.2.2, executeWithOptions is the single transformation
// point between options (the process boundary) and Dependencies
// (the command boundary). The tests below pin that transformation:
// they assert that executeWithOptions accepts options, constructs
// Dependencies exactly once, and threads Dependencies to the root
// command constructor.
//
// # The error boundary (WBS 10.5.1)
//
// The CLI's error handling is split across two functions:
//
//   - Execute recovers panics and converts them to
//     FORGE_INTERNAL_PANIC errors. The conversion is exercised
//     through recoveredPanicError; the placement of the deferred
//     recovery is a matter of code review, not of test.
//
//   - executeWithOptions ensures every error is a *forgeerr.Error
//     (wrapping non-*Error errors as FORGE_INTERNAL_UNEXPECTED),
//     renders the error, writes it to the injected stderr, and
//     returns the error's Category().ExitCode().
//
// The tests below cover the conversion, the wrapping, and the
// exit-code derivation. The tests that cannot exercise the
// recovery through runCLI are the reason recoveredPanicError is
// extracted: the helper is the testable unit.
//
// # Verbose mode (WBS 10.4.4)
//
// The CLI's error path reads the --verbose flag from the command
// tree and passes it through to forgeerr.Options.Verbose. The
// tests in the "verbose mode" section assert that the flag is
// honoured at the boundary.
//
// # Test organisation
//
// The file is organised in sections, each with a banner comment:
//
//  1. Tests — the public Execute function
//  2. Tests — the root command constructor
//  3. Tests — the injectable boundary
//  4. Tests — stream separation
//  5. Tests — the Dependencies transformation (WBS 4.2.2)
//  6. Tests — the panic recovery (WBS 10.5.1)
//  7. Tests — the non-*Error wrapping (WBS 10.5.1)
//  8. Tests — exit code derivation
//  9. Tests — the options struct
//  10. Tests — non-functional properties
//  11. Tests — the structural invariants the WBS specifies
//  12. Test helpers — source file reading and function body extraction
//
// A reader looking for a specific behaviour should scan the section
// banners, not read the file top to bottom.
package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/thapelomagqazana/forge/internal/forgeerr"
)

// =============================================================================
// Tests — the public Execute function
// =============================================================================

// TestExecute_IsCallable verifies that the exported Execute function
// can be invoked without panicking.
//
// This is the lightest possible test of the public surface. Its
// value is that it proves the symbol exists and its signature
// matches expectations.
//
// Note: this test calls the real Execute, which reads os.Args. The
// test binary's own arguments are treated by Cobra as an unknown
// command, so Execute returns a non-zero code. The test asserts the
// absence of a panic, not the exit code. The test is one of the
// three tests in the package that touches the process boundary
// directly; the other two are the signature check below and
// TestDefaultOptions_HasNonEmptyFields.
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
// The exported signature is exactly:
//
//	func Execute() int
//
// The named return in the implementation (the WBS 10.5.1 change)
// does not change the exported signature: callers see an int.
// This is a compile-time check; if the signature changes, the
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

// TestNewRootCmd_AcceptsDependencies verifies that newRootCmd has
// the signature required by the two-boundary model introduced in
// WBS 4.2.2.
//
// The signature must be:
//
//	func newRootCmd(deps Dependencies) *cobra.Command
//
// Before WBS 4.2.2, the signature was func(options) *cobra.Command.
// The refactor changed it to accept the command-boundary type so
// that subcommand constructors added in later WBS items receive
// resolved collaborators rather than raw process inputs.
//
// This is a compile-time check. If newRootCmd's parameter or return
// type changes, this line does not compile.
//
// # Why the old signature is not also asserted
//
// Some refactors preserve backward compatibility by adding a
// wrapper with the old signature. WBS 4.2.2 does not:
// executeWithOptions is the single transformation point from
// options to Dependencies, and newRootCmd is the single consumer of
// the result. Adding a func(options) wrapper would create a second
// construction path and violate AC4 (Dependencies constructed
// exactly once). The old signature is therefore deliberately gone,
// and the test asserts only the new one.
func TestNewRootCmd_AcceptsDependencies(t *testing.T) {
	t.Parallel()

	// A typed assignment forces the compiler to verify the
	// signature. If newRootCmd's parameter or return type changes,
	// this line does not compile.
	var _ func(Dependencies) *cobra.Command = newRootCmd
}

// TestNewRootCmd_ConstructsFromDependencies is the runtime companion
// to TestNewRootCmd_AcceptsDependencies. It verifies that a
// Dependencies value built from options produces a working root
// command.
//
// The test constructs Dependencies the same way
// executeWithOptions does, calls newRootCmd, and asserts that the
// returned command has the expected name. It does not execute the
// command; that is the responsibility of the tests below.
func TestNewRootCmd_ConstructsFromDependencies(t *testing.T) {
	t.Parallel()

	deps := buildDependencies(options{
		stdout:   &bytes.Buffer{},
		stderr:   &bytes.Buffer{},
		env:      func(string) string { return "" },
		rootPath: t.TempDir(),
	})

	root := newRootCmd(deps)
	if root == nil {
		t.Fatal("newRootCmd returned nil")
	}
	if root.Name() != "forge" {
		t.Errorf("root.Name() = %q; want %q", root.Name(), "forge")
	}
}

// =============================================================================
// Tests — the injectable boundary
// =============================================================================

// TestExecuteWithOptions_GivenEmptyArgs_ThenExitsSuccess verifies
// the boundary behaviour when args is empty.
//
// This is the lowest-level test of the execution boundary. It
// proves that an empty args slice is handled as the "no subcommand"
// case, which is the same as the no-args invocation.
//
// The helper is called with nil args and nil env. The helper
// converts the nil slice to a nil options.args, and the nil map to
// an env lookup that always returns the empty string.
func TestExecuteWithOptions_GivenEmptyArgs_ThenExitsSuccess(t *testing.T) {
	t.Parallel()

	got := runCLI(t, nil, nil)

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

	got := runCLI(t, []string{"--help"}, nil)

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
//
// The unknown-command error is produced by the root command's RunE
// handler, which constructs a *forgeerr.Error with category
// CategoryCLI (WBS 10.4.2). The exit code is therefore the CLI
// category's code (2).
func TestExecuteWithOptions_GivenUnknownCommand_ThenExitsUsage(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"unknown-command"}, nil)

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

	got := runCLI(t, []string{""}, nil)

	if got.exitCode == ExitSuccess {
		t.Errorf("empty-string argument should not exit %d; got: %d",
			ExitSuccess, got.exitCode)
	}
}

// TestExecuteWithOptions_GivenUnknownFlag_ThenExitsUsage verifies
// that an unknown flag produces ExitUsage.
func TestExecuteWithOptions_GivenUnknownFlag_ThenExitsUsage(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"--nonexistent-flag"}, nil)

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

// TestExecuteWithOptions_WritesNothingToStderrOnSuccess verifies
// that a successful invocation writes nothing to stderr.
//
// This is a boundary condition for the stdout/stderr separation
// contract. Even a stray newline on stderr would violate the
// contract and break shell scripts that redirect stderr to a log.
func TestExecuteWithOptions_WritesNothingToStderrOnSuccess(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"--help"}, nil)

	if got.stderr != "" {
		t.Errorf("stderr should be empty on success; got: %q",
			got.stderr)
	}
}

// TestExecuteWithOptions_WritesNothingToStdoutOnFailure verifies
// that a failing invocation writes nothing to stdout.
//
// The symmetric boundary condition: when the CLI fails, the
// diagnostic goes to stderr, and stdout remains empty.
func TestExecuteWithOptions_WritesNothingToStdoutOnFailure(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"unknown-command"}, nil)

	if got.stdout != "" {
		t.Errorf("stdout should be empty on failure; got: %q",
			got.stdout)
	}
}

// =============================================================================
// Tests — the Dependencies transformation (WBS 4.2.2)
// =============================================================================

// TestExecuteWithOptions_BuildsDependenciesOnce is a structural test
// for AC4. It asserts the observable consequence of the invariant:
// a command tree constructed by executeWithOptions writes to the
// injected stdout, which can only happen if Dependencies was built
// from options and threaded through newRootCmd.
//
// The test does not count call sites of buildDependencies (that
// requires source analysis, and is performed by the Taskfile target
// `verify:two-boundary:single-construction`). It asserts the runtime
// behaviour that depends on the invariant being true.
func TestExecuteWithOptions_BuildsDependenciesOnce(t *testing.T) {
	t.Parallel()

	got := runCLI(t, nil, nil) // no args: root prints help to stdout

	if got.exitCode != ExitSuccess {
		t.Errorf("exit code = %d; want %d (ExitSuccess)",
			got.exitCode, ExitSuccess)
	}
	if got.stdout == "" {
		t.Error("stdout is empty; " +
			"want help text (proves Dependencies was threaded)")
	}
	if got.stderr != "" {
		t.Errorf("stderr = %q; want empty", got.stderr)
	}
}

// TestExecuteWithOptions_GivenBadRootPath_StillRuns verifies that a
// rootPath which does not exist on disk does not prevent the CLI
// from running. The filesystem abstraction is bound to the path at
// construction time, but the path is not touched until a command
// actually performs I/O.
//
// This is an edge case for WBS 4.2.2: buildDependencies constructs
// an OSFS bound to rootPath, and OSFS.NewOSFS does not stat the
// directory. A test that passes a non-existent path therefore
// exercises the "boundary is informational" behaviour documented in
// osfs.go.
//
// The helper runCLIWithRoot is used instead of runCLI because the
// latter always uses an empty rootPath. runCLIWithRoot accepts the
// rootPath as its fourth parameter.
func TestExecuteWithOptions_GivenBadRootPath_StillRuns(t *testing.T) {
	t.Parallel()

	got := runCLIWithRoot(t, []string{"--help"}, nil,
		"/this/path/does/not/exist")

	if got.exitCode != ExitSuccess {
		t.Errorf("exit code = %d; want %d (ExitSuccess)",
			got.exitCode, ExitSuccess)
	}
	if got.stdout == "" {
		t.Error("stdout should contain help output")
	}
}

// TestExecuteWithOptions_GivenNilEnv_StillRuns verifies that a nil
// env function does not prevent --help from working. The env
// function is stored on Dependencies but is not called by the root
// command's help path. A nil env would panic only if a command
// actually reads an environment variable.
//
// This is a corner case that documents the laziness of the env
// dependency: it is injected eagerly but consumed lazily.
//
// The test uses runCLI with a nil env map, which the helper
// converts to a lookup function that always returns the empty
// string. This is not the same as a nil lookup function; the
// latter is exercised by the helper's implementation. The test
// confirms that a command which does not read env runs regardless
// of the env map's contents.
func TestExecuteWithOptions_GivenNilEnv_StillRuns(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"--help"}, nil)

	if got.exitCode != ExitSuccess {
		t.Errorf("exit code = %d; want %d (ExitSuccess)",
			got.exitCode, ExitSuccess)
	}
	if got.stdout == "" {
		t.Error("stdout should contain help output")
	}
}

// =============================================================================
// Tests — the panic recovery (WBS 10.5.1)
// =============================================================================
//
// The panic recovery lives in Execute, at the outermost point
// before os.Exit. It cannot be exercised through the test helpers:
// runCLI calls executeWithOptions, which does not recover, and
// calling Execute directly would require redirecting the process's
// streams (os.Args and os.Stderr).
//
// The conversion — the part of the recovery with logic — is
// extracted into recoveredPanicError and tested directly. The
// placement of the deferred recovery is a matter of code review.

// TestRecoveredPanicError asserts that a recovered panic value is
// converted into a *forgeerr.Error with category
// CategoryInternal, code CodeInternalPanic, and the panic's
// string representation in the context map.
//
// The test exercises the conversion with several panic-value
// shapes: a string, an integer, an error, and nil. The
// conversion's contract is that fmt.Sprint handles every shape
// and produces a non-empty string (nil renders as "<nil>").
func TestRecoveredPanicError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		panicValue any
		wantCtx    string
	}{
		{"string", "panic-value", "panic-value"},
		{"int", 42, "42"},
		{"error", errors.New("boom"), "boom"},
		{"nil", nil, "<nil>"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			e := recoveredPanicError(tc.panicValue)

			if got := e.Category(); got != forgeerr.CategoryInternal {
				t.Errorf("Category() = %v; want %v",
					got, forgeerr.CategoryInternal)
			}
			if got := e.Code(); got != string(forgeerr.CodeInternalPanic) {
				t.Errorf("Code() = %q; want %q",
					got, forgeerr.CodeInternalPanic)
			}
			if got := e.Context()["panic"]; got != tc.wantCtx {
				t.Errorf("Context()[panic] = %q; want %q",
					got, tc.wantCtx)
			}
		})
	}
}

// TestRecoveredPanicError_ExitCode asserts that the converted
// error's category's exit code is the INTERNAL category's code.
//
// The check is separate from the conversion test so that a
// failure in the exit-code derivation is distinguishable from a
// failure in the conversion.
func TestRecoveredPanicError_ExitCode(t *testing.T) {
	t.Parallel()

	e := recoveredPanicError("panic-value")
	if got := e.Category().ExitCode(); got != forgeerr.CategoryInternal.ExitCode() {
		t.Errorf("Category().ExitCode() = %d; want %d (INTERNAL)",
			got, forgeerr.CategoryInternal.ExitCode())
	}
}

// TestRecoveredPanicError_MessageIsFixed asserts that the
// converted error's message does not depend on the panic value.
//
// The message is a fixed string ("internal error occurred") so
// that the message invariants (single line, no trailing period,
// <= 80 bytes) hold regardless of the panic value's content. A
// panic value that happens to contain newlines or a trailing
// period would otherwise produce a malformed *forgeerr.Error if
// the message included it.
func TestRecoveredPanicError_MessageIsFixed(t *testing.T) {
	t.Parallel()

	// A panic value with a newline and a trailing period. If the
	// message included the value, the constructor would panic.
	cases := []any{
		"line one\nline two",
		"ends with a period.",
		strings.Repeat("a", 200),
	}
	for _, v := range cases {
		v := v
		t.Run(fmt.Sprintf("%v", v)[:minInt(len(fmt.Sprintf("%v", v)), 20)], func(t *testing.T) {
			t.Parallel()

			e := recoveredPanicError(v)
			if got := e.Message(); got != "internal error occurred" {
				t.Errorf("Message() = %q; want %q",
					got, "internal error occurred")
			}
		})
	}
}

// =============================================================================
// Tests — the non-*Error wrapping (WBS 10.5.1)
// =============================================================================

// TestWrappedUnexpectedError asserts that a plain error is
// wrapped in a *forgeerr.Error with category CategoryInternal,
// code CodeInternalUnexpected, and the original error's message
// preserved in the context and attached as the cause.
func TestWrappedUnexpectedError(t *testing.T) {
	t.Parallel()

	original := errors.New("plain error")
	e := wrappedUnexpectedError(original)

	if got := e.Category(); got != forgeerr.CategoryInternal {
		t.Errorf("Category() = %v; want %v",
			got, forgeerr.CategoryInternal)
	}
	if got := e.Code(); got != string(forgeerr.CodeInternalUnexpected) {
		t.Errorf("Code() = %q; want %q",
			got, forgeerr.CodeInternalUnexpected)
	}
	if got := e.Context()["cause"]; got != "plain error" {
		t.Errorf("Context()[cause] = %q; want %q", got, "plain error")
	}
	if !errors.Is(e, original) {
		t.Error("errors.Is(e, original) = false; the cause is not reachable")
	}
}

// TestWrappedUnexpectedError_ReachableThroughChain asserts that
// the original error is reachable through errors.As and errors.Is
// when the wrapper is in a chain.
func TestWrappedUnexpectedError_ReachableThroughChain(t *testing.T) {
	t.Parallel()

	original := errors.New("plain error")
	e := wrappedUnexpectedError(original)
	wrapped := fmt.Errorf("outer: %w", e)

	if !errors.Is(wrapped, original) {
		t.Error("errors.Is did not reach the original error through the wrapper")
	}
}

// TestExecute_NonForgeErrorWrapped asserts that a non-*forgeerr.Error
// error returned from a command is wrapped in a
// FORGE_INTERNAL_UNEXPECTED error at the boundary.
//
// The test installs a temporary command that returns a plain
// error, runs the CLI with that command, and asserts on the
// output and exit code.
//
// # Why the test is serial
//
// The test installs a temporary registry entry via the test-only
// hook. The registry is shared state; the hook mutates it. A
// parallel test would race with other tests that read the
// registry. The test is therefore serial.
func TestExecute_NonForgeErrorWrapped(t *testing.T) {
	restore := testOnlyRegisterPlainErrorCommand(errors.New("plain error"))
	defer restore()

	got := runCLI(t, []string{"__test_plain_error__"}, nil)

	// The exit code is the INTERNAL category's code (1). The
	// wrapper has category CategoryInternal.
	if got.exitCode != forgeerr.CategoryInternal.ExitCode() {
		t.Errorf("exit code = %d; want %d (INTERNAL category)",
			got.exitCode, forgeerr.CategoryInternal.ExitCode())
	}

	// The stderr contains the FORGE_INTERNAL_UNEXPECTED code,
	// which proves the plain error was wrapped rather than
	// rendered by the fallback path.
	if !strings.Contains(got.stderr, string(forgeerr.CodeInternalUnexpected)) {
		t.Errorf("stderr does not contain %q:\n%s",
			forgeerr.CodeInternalUnexpected, got.stderr)
	}

	// The stdout is empty on failure.
	if got.stdout != "" {
		t.Errorf("stdout = %q; want empty", got.stdout)
	}
}

// =============================================================================
// Tests — exit code derivation
// =============================================================================

// TestExitCodeOf_GivenNilError_ReturnsExitSuccess asserts the
// nil case of the exit-code helper.
func TestExitCodeOf_GivenNilError_ReturnsExitSuccess(t *testing.T) {
	t.Parallel()

	if got := exitCodeOf(nil); got != ExitSuccess {
		t.Errorf("exitCodeOf(nil) = %d; want %d (ExitSuccess)",
			got, ExitSuccess)
	}
}

// TestExitCodeOf_GivenForgeError_ReturnsCategoryCode asserts that
// the helper reads the error's category's code.
//
// The test constructs a *forgeerr.Error with each category and
// asserts that the helper returns the category's code. The
// category-code agreement is defined by the taxonomy (WBS
// 10.2.1); the helper does not maintain a separate mapping.
func TestExitCodeOf_GivenForgeError_ReturnsCategoryCode(t *testing.T) {
	t.Parallel()

	cases := []struct {
		cat  forgeerr.Category
		code forgeerr.Code
	}{
		{forgeerr.CategoryCLI, forgeerr.CodeCLIUnknownCommand},
		{forgeerr.CategoryConfig, forgeerr.CodeConfigMalformed},
		{forgeerr.CategoryFilesystem, forgeerr.CodeFilesystemNotFound},
		{forgeerr.CategorySecurity, forgeerr.CodeSecurityPathTraversal},
		{forgeerr.CategoryInternal, forgeerr.CodeInternalUnexpected},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(string(tc.cat), func(t *testing.T) {
			t.Parallel()

			e := forgeerr.New(tc.cat, string(tc.code), "message")
			got := exitCodeOf(e)
			want := tc.cat.ExitCode()
			if got != want {
				t.Errorf("exitCodeOf(%v) = %d; want %d (category's code)",
					tc.cat, got, want)
			}
		})
	}
}

// TestExitCodeOf_GivenWrappedForgeError_ReturnsCategoryCode
// asserts that the helper extracts the *forgeerr.Error from a
// wrapping chain.
func TestExitCodeOf_GivenWrappedForgeError_ReturnsCategoryCode(t *testing.T) {
	t.Parallel()

	e := forgeerr.New(
		forgeerr.CategoryConfig,
		string(forgeerr.CodeConfigMalformed),
		"invalid YAML",
	)
	wrapped := fmt.Errorf("outer context: %w", e)

	if got := exitCodeOf(wrapped); got != forgeerr.CategoryConfig.ExitCode() {
		t.Errorf("exitCodeOf(wrapped) = %d; want %d",
			got, forgeerr.CategoryConfig.ExitCode())
	}
}

// TestExitCodeOf_GivenPlainError_ReturnsExitFailure asserts the
// safety-net branch: an error that is not a *forgeerr.Error and
// does not wrap one yields ExitFailure.
//
// The branch is reached only when executeWithOptions is called
// directly by a caller that constructed a non-*Error error
// without the wrapping. The wrapping in executeWithOptions
// normally prevents this; the branch exists as a safety net.
func TestExitCodeOf_GivenPlainError_ReturnsExitFailure(t *testing.T) {
	t.Parallel()

	if got := exitCodeOf(errors.New("plain")); got != ExitFailure {
		t.Errorf("exitCodeOf(plain error) = %d; want %d (ExitFailure)",
			got, ExitFailure)
	}
}

// TestExecute_UsesCategoryExitCode asserts that the exit code
// returned by executeWithOptions for an error is the error's
// Category().ExitCode().
//
// The test is indirect: it runs the unknown-command path, which
// produces a *forgeerr.Error with category CategoryCLI, and
// asserts that the exit code is that category's code.
func TestExecute_UsesCategoryExitCode(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"unknown-command"}, nil)

	if got.exitCode != forgeerr.CategoryCLI.ExitCode() {
		t.Errorf("exit code = %d; want %d (CategoryCLI.ExitCode)",
			got.exitCode, forgeerr.CategoryCLI.ExitCode())
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
// directory that has been removed, so its emptiness is not
// asserted.
//
// The verbose field is not set by defaultOptions: it is read from
// the command tree after Cobra parses the arguments. The test
// asserts that the zero value is false.
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
	if opts.verbose {
		t.Error("verbose is true; defaultOptions must leave it unset")
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

	first := runCLI(t, []string{"--help"}, nil)

	for i := 0; i < 20; i++ {
		got := runCLI(t, []string{"--help"}, nil)
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
//
// # Scope of the check
//
// The check is deliberately narrow: it inspects only the body of
// executeWithOptions, not the whole file. defaultOptions is allowed
// to reference os.Args, os.Stdin, os.Stdout, os.Stderr, and
// os.Getenv, because it is the single reader of those values.
// executeWithOptions is not allowed to reference them, because it
// must receive every input through the options struct.
//
// The panic path in Execute references os.Stderr directly; the
// exception is documented in Execute's docstring. The check is
// scoped to executeWithOptions and does not flag the panic path.
func TestExecuteWithOptions_NoProcessStreamsReferenced(t *testing.T) {
	t.Parallel()

	content := readFile(t, "execute.go")
	body := extractFuncBody(t, content, "executeWithOptions")

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

// TestExecuteWithOptions_UsesBuildDependencies verifies that
// executeWithOptions constructs Dependencies via buildDependencies,
// not by hand. This is the structural companion to
// TestExecuteWithOptions_BuildsDependenciesOnce.
//
// The test is a source-code check: it asserts that the function
// body contains a call to buildDependencies. It is the single call
// site required by AC4.
func TestExecuteWithOptions_UsesBuildDependencies(t *testing.T) {
	t.Parallel()

	content := readFile(t, "execute.go")
	body := extractFuncBody(t, content, "executeWithOptions")

	if !strings.Contains(body, "buildDependencies(opts)") {
		t.Error("executeWithOptions does not call buildDependencies(opts)")
	}
}

// TestExecuteWithOptions_DoesNotConstructDependenciesByHand verifies
// that executeWithOptions does not construct a Dependencies value
// with a struct literal. A struct literal would bypass
// buildDependencies and duplicate the construction logic, violating
// AC4 and AC6.
//
// The test is a source-code check: it asserts that the function
// body does not contain the substring "Dependencies{".
func TestExecuteWithOptions_DoesNotConstructDependenciesByHand(t *testing.T) {
	t.Parallel()

	content := readFile(t, "execute.go")
	body := extractFuncBody(t, content, "executeWithOptions")

	if strings.Contains(body, "Dependencies{") {
		t.Error("executeWithOptions constructs Dependencies by hand; " +
			"use buildDependencies instead (AC4, AC6)")
	}
}

// TestExecuteWithOptions_ThreadsDependenciesToRoot verifies that
// executeWithOptions passes the Dependencies value to newRootCmd,
// rather than passing options or nothing.
//
// The test is a source-code check: it asserts that the function
// body contains the substring "newRootCmd(deps)". This is the
// contract that decouples subcommand constructors from the
// process-boundary type (AC2, AC6).
func TestExecuteWithOptions_ThreadsDependenciesToRoot(t *testing.T) {
	t.Parallel()

	content := readFile(t, "execute.go")
	body := extractFuncBody(t, content, "executeWithOptions")

	if !strings.Contains(body, "newRootCmd(deps)") {
		t.Error("executeWithOptions does not call newRootCmd(deps); " +
			"the Dependencies value must be threaded to the root")
	}
	if strings.Contains(body, "newRootCmd(opts)") {
		t.Error("executeWithOptions calls newRootCmd(opts); " +
			"the root must receive Dependencies, not options")
	}
}

// TestExecuteWithOptions_UsesVerboseRequested verifies that
// executeWithOptions reads the verbose flag from the command tree
// rather than from the options struct or from a global.
//
// The test is a source-code check: it asserts that the function
// body contains a call to verboseRequested. This is the WBS
// 10.4.4 contract: the flag is read after Cobra parses the
// arguments, from the command that carries it.
func TestExecuteWithOptions_UsesVerboseRequested(t *testing.T) {
	t.Parallel()

	content := readFile(t, "execute.go")
	body := extractFuncBody(t, content, "executeWithOptions")

	if !strings.Contains(body, "verboseRequested(root)") {
		t.Error("executeWithOptions does not call verboseRequested(root); " +
			"the verbose flag must be read from the command tree")
	}
}

// TestExecuteWithOptions_WrapsNonForgeErrors verifies that
// executeWithOptions wraps non-*forgeerr.Error errors as
// FORGE_INTERNAL_UNEXPECTED before rendering.
//
// The test is a source-code check: it asserts that the function
// body contains a call to errors.As with a *forgeerr.Error target
// and a call to wrappedUnexpectedError. The combination is the
// wrapping's fingerprint.
//
// # Why the check is source-code
//
// The runtime test (TestExecute_NonForgeErrorWrapped) exercises
// the wrapping through the CLI's error path. This source-code
// check is a belt-and-braces assertion: it catches a regression
// in which the wrapping is removed or bypassed, even if the
// runtime test happens to pass for a different reason.
func TestExecuteWithOptions_WrapsNonForgeErrors(t *testing.T) {
	t.Parallel()

	content := readFile(t, "execute.go")
	body := extractFuncBody(t, content, "executeWithOptions")

	if !strings.Contains(body, "errors.As(err, &ferr)") {
		t.Error("executeWithOptions does not call errors.As with a " +
			"*forgeerr.Error target; the wrapping cannot happen")
	}
	if !strings.Contains(body, "wrappedUnexpectedError(err)") {
		t.Error("executeWithOptions does not call wrappedUnexpectedError; " +
			"non-*forgeerr.Error errors are not wrapped")
	}
}

// TestExecuteWithOptions_NoFormatError verifies that the
// formatError helper no longer exists. The check is a
// source-code scan: it reads the file and asserts that
// "func formatError(" is not present.
//
// # Why the check is a source-code scan
//
// The compiler would catch a direct reference to a removed
// symbol. The source-code scan catches the case where the symbol
// is re-introduced but not used, which a compiler cannot detect.
// A re-introduction is a regression: the format is owned by
// forgeerr.Format (WBS 10.4.1), and a duplicate would drift.
func TestExecuteWithOptions_NoFormatError(t *testing.T) {
	t.Parallel()

	content := readFile(t, "execute.go")
	if strings.Contains(content, "func formatError(") {
		t.Error("execute.go still contains a formatError definition; " +
			"the format is owned by forgeerr.Format (WBS 10.4.1)")
	}
}

// TestExecuteWithOptions_NoExitCodeFromError verifies that the
// exitCodeFromError helper no longer exists. The check is a
// source-code scan over both execute.go and exitcodes.go.
//
// # Why the check is a source-code scan
//
// Same rationale as TestExecuteWithOptions_NoFormatError: the
// compiler catches direct references, but a re-introduction that
// is not referenced would slip through. The check is a
// regression guard against the helper's return.
func TestExecuteWithOptions_NoExitCodeFromError(t *testing.T) {
	t.Parallel()

	for _, file := range []string{"execute.go", "exitcodes.go"} {
		content := readFile(t, file)
		if strings.Contains(content, "func exitCodeFromError(") {
			t.Errorf("%s still contains an exitCodeFromError definition; "+
				"the exit code mapping is owned by Category.ExitCode (WBS 10.2.1)",
				file)
		}
	}
}

// =============================================================================
// Test helpers — source file reading and function body extraction
// =============================================================================

// executeTestPackageDir returns the absolute path to the
// internal/cli directory.
//
// It derives the path from the location of this test file, using
// runtime.Caller to find the source file path, then returning the
// directory that contains it.
//
// # Why runtime.Caller and not os.Getwd
//
// os.Getwd returns the directory from which the test binary was
// invoked, which is not necessarily the directory containing the
// test file. In a Go module, `go test ./internal/cli/` runs the
// test binary with the working directory set to the package
// directory, but this is not guaranteed by the toolchain and can
// change. runtime.Caller returns the path of the source file that
// called it, which is stable regardless of the working directory.
//
// The helper is called from readFile, which is called from the
// structural tests. The stack depth is fixed (runtime.Caller(0) is
// this function itself), so the file path returned is always this
// test file's path.
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
//
// # Why the helper does not cache
//
// The helper reads the file on every call. Caching would make the
// tests non-hermetic: a test that modifies a source file (which no
// test does today) would see stale contents. Reading on every call
// is cheap for a package with a handful of files, and it keeps the
// helper simple.
func readFile(t *testing.T, name string) string {
	t.Helper()

	path := filepath.Join(executeTestPackageDir(t), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(data)
}

// extractFuncBody returns the source text of the named function's
// body, including the opening and closing braces, with comments
// stripped.
//
// The function is located by searching for "func <name>(" in the
// content. The body is extracted by counting braces from the
// function's opening brace to its matching closing brace.
//
// # Why comments are stripped
//
// The extractor is used by structural tests that assert the body
// does not contain certain tokens (for example, "os.Stdout"). A
// comment inside the body may legitimately mention those tokens
// while explaining why they are not used. Stripping comments before
// the token check separates "the code does not reference os.Stdout"
// from "the docstring mentions os.Stdout".
//
// Stripping comments also makes brace counting robust: a comment
// containing an unbalanced brace (for example, `// see {foo`) would
// otherwise be counted as an opening brace and would truncate or
// extend the extracted body.
//
// # Limitations
//
// The stripper handles the two Go comment forms: `// ...` to end of
// line, and `/* ... */`. It does not handle braces inside string or
// rune literals. The file it is applied to (execute.go) contains
// none of those inside the function bodies it inspects. A future
// structural test that inspects such a function should use
// go/parser instead.
func extractFuncBody(t *testing.T, content, funcName string) string {
	t.Helper()

	needle := "func " + funcName + "("
	start := strings.Index(content, needle)
	if start < 0 {
		t.Fatalf("func %s not found", funcName)
	}

	// Find the opening brace of the function signature.
	openRel := strings.Index(content[start:], "{")
	if openRel < 0 {
		t.Fatalf("opening brace of %s not found", funcName)
	}
	open := start + openRel

	// Walk forward from the opening brace, skipping comments and
	// counting braces. When the depth returns to zero, we have
	// found the function's closing brace.
	//
	// The loop is written as an explicit index walk rather than a
	// range over runes because Go source is ASCII for the purposes
	// of brace counting, and byte indexing is clearer than rune
	// indexing for a scanner.
	var b strings.Builder
	depth := 0
	i := open
	for i < len(content) {
		// Line comment: skip to end of line. The newline is kept
		// so that the extracted body's line structure is
		// preserved for diagnostics.
		if i+1 < len(content) && content[i] == '/' && content[i+1] == '/' {
			j := strings.IndexByte(content[i:], '\n')
			if j < 0 {
				// Comment runs to end of file; the function
				// cannot be properly closed.
				t.Fatalf("unterminated line comment in %s", funcName)
			}
			i += j // leave the newline for the next iteration
			continue
		}

		// Block comment: skip to the closing "*/".
		if i+1 < len(content) && content[i] == '/' && content[i+1] == '*' {
			j := strings.Index(content[i+2:], "*/")
			if j < 0 {
				t.Fatalf("unterminated block comment in %s", funcName)
			}
			i += 2 + j + 2 // skip "/*", the comment body, and "*/"
			continue
		}

		// Ordinary character: append it, and update the depth on
		// braces.
		switch content[i] {
		case '{':
			depth++
		case '}':
			depth--
		}
		b.WriteByte(content[i])
		i++

		if depth == 0 {
			return b.String()
		}
	}

	t.Fatalf("closing brace of %s not found", funcName)
	return "" // unreachable; t.Fatalf does not return
}
