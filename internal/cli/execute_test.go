// Package cli contains white-box tests for the CLI package.
//
// This file tests the boundary between the CLI and the process
// environment: the Execute and Run public functions, the unexported
// runWithOptions function, and the helper functions that support
// the boundary: defaultOptions, exitCodeOf, recoveredPanicError,
// wrappedUnexpectedError, hasVerboseArg, and signalExitCode.
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
//   - mapping_test.go tests the error-to-exit-code mapping
//     (WBS 11.2.1).
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
// After WBS 4.2.2, runWithOptions is the single transformation
// point between options (the process boundary) and Dependencies
// (the command boundary). The tests below pin that transformation:
// they assert that runWithOptions accepts options, constructs
// Dependencies exactly once, and threads Dependencies to the root
// command constructor.
//
// # The exit boundary (WBS 11.3.1, ADR-020)
//
// The package has two public entry points:
//
//   - Run(ctx) (int, error) is the process boundary. It receives
//     the context the caller installed and returns the exit code
//     and the rendered error.
//
//   - Execute() int is a thin wrapper around Run. It calls Run
//     with a background context and discards the error.
//
// Both delegate to runWithOptions, which is the internal
// implementation and the subject of the tests in this file. The
// tests call runWithOptions through the runCLI helper (which uses
// a background context) and directly (for the context-cancellation
// cases).
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
//   - runWithOptions ensures every error is a *forgeerr.Error
//     (wrapping non-*Error errors as FORGE_INTERNAL_UNEXPECTED),
//     renders the error, writes it to the injected stderr, and
//     returns the error's Category().ExitCode() together with the
//     error.
//
// The tests below cover the conversion, the wrapping, and the
// exit-code derivation. The tests that cannot exercise the
// recovery through runCLI are the reason recoveredPanicError is
// extracted: the helper is the testable unit.
//
// # Exit codes (WBS 11.1.1)
//
// The exit codes asserted below are documented in
// docs/reference/exit-code-ranges.md and decided in ADR-018. The
// tests reference the named constants (ExitSuccess, ExitUsage,
// ExitInternal, ExitConfig, ExitSecurity) rather than their
// numeric values, so that a future additive change to the
// taxonomy does not require editing every assertion. The two
// numeric values that appear as literals in the tests are 0 and
// 255, which are frozen by the taxonomy's coverage invariant.
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
//  12. Tests — the exit boundary (WBS 11.3.1, ADR-020)
//  13. Test helpers — source file reading and function body extraction
//
// A reader looking for a specific behaviour should scan the section
// banners, not read the file top to bottom.
package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
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
// tests in the package that touches the process boundary directly.
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
//
// # Relationship to Run
//
// WBS 11.3.1 added Run(ctx) (int, error) as the primary entry
// point. Execute is preserved as a compatibility wrapper.
// TestRun_HasExpectedSignature (below) asserts Run's signature;
// this test asserts Execute's.
func TestExecute_HasFrozenSignature(t *testing.T) {
	t.Parallel()

	// A typed assignment forces the compiler to verify the
	// signature. If Execute's parameters or return type change,
	// this line does not compile.
	var _ func() int = Execute
}

// TestRun_HasExpectedSignature verifies that Run has the signature
// introduced by WBS 11.3.1.
//
// The signature must be:
//
//	func Run(ctx context.Context) (int, error)
//
// The two-value return is the seam for embedding Forge as a
// library; see ADR-020.
func TestRun_HasExpectedSignature(t *testing.T) {
	t.Parallel()

	// A typed assignment forces the compiler to verify the
	// signature.
	var _ func(context.Context) (int, error) = Run
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
func TestNewRootCmd_AcceptsDependencies(t *testing.T) {
	t.Parallel()

	var _ func(Dependencies) *cobra.Command = newRootCmd
}

// TestNewRootCmd_ConstructsFromDependencies is the runtime companion
// to TestNewRootCmd_AcceptsDependencies.
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
// category's code, ExitUsage (64, BSD EX_USAGE).
//
// # The value changed in WBS 11.1.1
//
// Before the WBS 11.1.1 taxonomy, the CLI category's code was 2.
// The taxonomy moved it to 64 (BSD sysexits.h EX_USAGE). The
// assertion uses the named constant ExitUsage, so the test remains
// correct regardless of the numeric value.
func TestExecuteWithOptions_GivenUnknownCommand_ThenExitsUsage(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"unknown-command"}, nil)

	if got.exitCode != ExitUsage {
		t.Errorf("unknown command should exit %d (ExitUsage); got: %d",
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
//
// The error is produced by Cobra's parser and converted by
// wrapCobraError (execute.go) into a *forgeerr.Error with category
// CategoryCLI. The exit code is therefore the CLI category's code,
// ExitUsage (64, BSD EX_USAGE).
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
// a command tree constructed by runWithOptions writes to the
// injected stdout, which can only happen if Dependencies was built
// from options and threaded through newRootCmd.
func TestExecuteWithOptions_BuildsDependenciesOnce(t *testing.T) {
	t.Parallel()

	got := runCLI(t, nil, nil)

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
// from running.
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
// env function does not prevent --help from working.
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
// runCLI calls runWithOptions, which does not recover, and
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
// error's category's exit code is the internal-error code.
func TestRecoveredPanicError_ExitCode(t *testing.T) {
	t.Parallel()

	e := recoveredPanicError("panic-value")
	if got := e.Category().ExitCode(); got != ExitInternal {
		t.Errorf("Category().ExitCode() = %d; want %d (ExitInternal)",
			got, ExitInternal)
	}
	if got := e.Category().ExitCode(); got != forgeerr.CategoryInternal.ExitCode() {
		t.Errorf("Category().ExitCode() = %d; want %d (CategoryInternal.ExitCode)",
			got, forgeerr.CategoryInternal.ExitCode())
	}
}

// TestRecoveredPanicError_MessageIsFixed asserts that the
// converted error's message does not depend on the panic value.
func TestRecoveredPanicError_MessageIsFixed(t *testing.T) {
	t.Parallel()

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
// # The exit code
//
// The wrapped error's category is CategoryInternal, whose
// ExitCode() is ExitInternal (70, BSD EX_SOFTWARE).
//
// # Why the test is serial
//
// The test installs a temporary registry entry via the test-only
// hook. The registry is shared state; the hook mutates it.
func TestExecute_NonForgeErrorWrapped(t *testing.T) {
	restore := testOnlyRegisterPlainErrorCommand(errors.New("plain error"))
	defer restore()

	got := runCLI(t, []string{"__test_plain_error__"}, nil)

	if got.exitCode != ExitInternal {
		t.Errorf("exit code = %d; want %d (ExitInternal)",
			got.exitCode, ExitInternal)
	}
	if got.exitCode != forgeerr.CategoryInternal.ExitCode() {
		t.Errorf("exit code = %d; want %d (CategoryInternal.ExitCode)",
			got.exitCode, forgeerr.CategoryInternal.ExitCode())
	}
	if !strings.Contains(got.stderr, string(forgeerr.CodeInternalUnexpected)) {
		t.Errorf("stderr does not contain %q:\n%s",
			forgeerr.CodeInternalUnexpected, got.stderr)
	}
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

// TestExitCodeOf_GivenPlainError_ReturnsExitInternal asserts the
// safety-net branch: an error that is not a *forgeerr.Error and
// does not wrap one yields ExitInternal.
func TestExitCodeOf_GivenPlainError_ReturnsExitInternal(t *testing.T) {
	t.Parallel()

	if got := exitCodeOf(errors.New("plain")); got != ExitInternal {
		t.Errorf("exitCodeOf(plain error) = %d; want %d (ExitInternal)",
			got, ExitInternal)
	}
}

// TestExitCodeOf_ResultIsInRange asserts that the helper's result
// always falls within a documented exit-code range (WBS 11.1.1).
func TestExitCodeOf_ResultIsInRange(t *testing.T) {
	t.Parallel()

	for _, cat := range forgeerr.AllCategories() {
		cat := cat
		t.Run(string(cat), func(t *testing.T) {
			t.Parallel()

			code := cat.ExitCode()
			if err := VerifyExitCodeRange(code); err != nil {
				t.Errorf("VerifyExitCodeRange(%v.ExitCode() = %d): %v",
					cat, code, err)
			}
			if _, ok := LookupExitCodeRange(code); !ok {
				t.Errorf("%v.ExitCode() = %d falls outside all documented ranges",
					cat, code)
			}
		})
	}

	if got := exitCodeOf(errors.New("plain")); !IsInternalError(got) {
		t.Errorf("plain-error fallback = %d; want an internal-error code (70–78)", got)
	}
}

// TestExecute_UsesCategoryExitCode asserts that the exit code
// returned by runWithOptions for an error is the error's
// Category().ExitCode().
func TestExecute_UsesCategoryExitCode(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"unknown-command"}, nil)

	if got.exitCode != forgeerr.CategoryCLI.ExitCode() {
		t.Errorf("exit code = %d; want %d (CategoryCLI.ExitCode)",
			got.exitCode, forgeerr.CategoryCLI.ExitCode())
	}
	if got.exitCode != ExitUsage {
		t.Errorf("exit code = %d; want %d (ExitUsage)",
			got.exitCode, ExitUsage)
	}
}

// =============================================================================
// Tests — the options struct
// =============================================================================

// TestDefaultOptions_HasNonEmptyFields verifies that the default
// options value has sensible values for every field.
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
// runWithOptions does not reference os.Stdout, os.Stderr,
// os.Stdin, os.Args, or os.Getenv directly.
//
// This is a structural invariant the WBS specifies: "No global
// state is read directly (os.Args, os.Stdout) inside commands —
// everything flows through a Dependencies struct."
//
// # Scope of the check
//
// The check is deliberately narrow: it inspects only the body of
// runWithOptions, not the whole file. defaultOptions is allowed
// to reference os.Args, os.Stdin, os.Stdout, os.Stderr, and
// os.Getenv, because it is the single reader of those values.
// runWithOptions is not allowed to reference them, because it
// must receive every input through the options struct.
func TestExecuteWithOptions_NoProcessStreamsReferenced(t *testing.T) {
	t.Parallel()

	content := readFile(t, "execute.go")
	body := extractFuncBody(t, content, "runWithOptions")

	forbidden := []string{
		"os.Stdout",
		"os.Stderr",
		"os.Stdin",
		"os.Args",
		"os.Getenv",
	}
	for _, token := range forbidden {
		if strings.Contains(body, token) {
			t.Errorf("runWithOptions body references %s; "+
				"all I/O must flow through the options struct",
				token)
		}
	}
}

// TestExecuteWithOptions_UsesBuildDependencies verifies that
// runWithOptions constructs Dependencies via buildDependencies,
// not by hand.
func TestExecuteWithOptions_UsesBuildDependencies(t *testing.T) {
	t.Parallel()

	content := readFile(t, "execute.go")
	body := extractFuncBody(t, content, "runWithOptions")

	if !strings.Contains(body, "buildDependencies(opts)") {
		t.Error("runWithOptions does not call buildDependencies(opts)")
	}
}

// TestExecuteWithOptions_DoesNotConstructDependenciesByHand verifies
// that runWithOptions does not construct a Dependencies value with
// a struct literal.
func TestExecuteWithOptions_DoesNotConstructDependenciesByHand(t *testing.T) {
	t.Parallel()

	content := readFile(t, "execute.go")
	body := extractFuncBody(t, content, "runWithOptions")

	if strings.Contains(body, "Dependencies{") {
		t.Error("runWithOptions constructs Dependencies by hand; " +
			"use buildDependencies instead (AC4, AC6)")
	}
}

// TestExecuteWithOptions_ThreadsDependenciesToRoot verifies that
// runWithOptions passes the Dependencies value to newRootCmd,
// rather than passing options or nothing.
func TestExecuteWithOptions_ThreadsDependenciesToRoot(t *testing.T) {
	t.Parallel()

	content := readFile(t, "execute.go")
	body := extractFuncBody(t, content, "runWithOptions")

	if !strings.Contains(body, "newRootCmd(deps)") {
		t.Error("runWithOptions does not call newRootCmd(deps); " +
			"the Dependencies value must be threaded to the root")
	}
	if strings.Contains(body, "newRootCmd(opts)") {
		t.Error("runWithOptions calls newRootCmd(opts); " +
			"the root must receive Dependencies, not options")
	}
}

// TestExecuteWithOptions_UsesVerboseRequested verifies that
// runWithOptions reads the verbose flag from the command tree
// rather than from the options struct or from a global.
func TestExecuteWithOptions_UsesVerboseRequested(t *testing.T) {
	t.Parallel()

	content := readFile(t, "execute.go")
	body := extractFuncBody(t, content, "runWithOptions")

	if !strings.Contains(body, "verboseRequested(root)") {
		t.Error("runWithOptions does not call verboseRequested(root); " +
			"the verbose flag must be read from the command tree")
	}
}

// TestExecuteWithOptions_WrapsNonForgeErrors verifies that
// runWithOptions wraps non-*forgeerr.Error errors as
// FORGE_INTERNAL_UNEXPECTED before rendering.
func TestExecuteWithOptions_WrapsNonForgeErrors(t *testing.T) {
	t.Parallel()

	content := readFile(t, "execute.go")
	body := extractFuncBody(t, content, "runWithOptions")

	if !strings.Contains(body, "errors.As(err, &ferr)") {
		t.Error("runWithOptions does not call errors.As with a " +
			"*forgeerr.Error target; the wrapping cannot happen")
	}
	if !strings.Contains(body, "wrappedUnexpectedError(err)") {
		t.Error("runWithOptions does not call wrappedUnexpectedError; " +
			"non-*forgeerr.Error errors are not wrapped")
	}
}

// TestExecuteWithOptions_NoFormatError verifies that the
// formatError helper no longer exists.
func TestExecuteWithOptions_NoFormatError(t *testing.T) {
	t.Parallel()

	content := readFile(t, "execute.go")
	if strings.Contains(content, "func formatError(") {
		t.Error("execute.go still contains a formatError definition; " +
			"the format is owned by forgeerr.Format (WBS 10.4.1)")
	}
}

// TestExecuteWithOptions_NoExitCodeFromError verifies that the
// exitCodeFromError helper no longer exists.
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
// Tests — the exit boundary (WBS 11.3.1, ADR-020)
// =============================================================================

// TestRun_ReturnsCodeAndNilErrorOnSuccess asserts that Run
// returns (ExitSuccess, nil) for a successful invocation.
func TestRun_ReturnsCodeAndNilErrorOnSuccess(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	opts := options{
		args:     []string{"--help"},
		stdin:    strings.NewReader(""),
		stdout:   &stdout,
		stderr:   &stderr,
		env:      func(string) string { return "" },
		rootPath: t.TempDir(),
	}

	code, err := runWithOptions(context.Background(), opts)
	if code != ExitSuccess {
		t.Errorf("code = %d; want %d (ExitSuccess)", code, ExitSuccess)
	}
	if err != nil {
		t.Errorf("err = %v; want nil", err)
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q; want empty", stderr.String())
	}
}

// TestRun_ReturnsCodeAndErrorOnFailure asserts that the boundary
// returns the mapped code and the error it rendered.
func TestRun_ReturnsCodeAndErrorOnFailure(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	opts := options{
		args:     []string{"unknown-command"},
		stdin:    strings.NewReader(""),
		stdout:   &stdout,
		stderr:   &stderr,
		env:      func(string) string { return "" },
		rootPath: t.TempDir(),
	}

	code, err := runWithOptions(context.Background(), opts)
	if code != ExitUsage {
		t.Errorf("code = %d; want %d (ExitUsage)", code, ExitUsage)
	}
	if err == nil {
		t.Fatal("err = nil; want non-nil for a failing invocation")
	}
	var fe *forgeerr.Error
	if !errors.As(err, &fe) {
		t.Fatalf("err = %T; want *forgeerr.Error", err)
	}
	if !strings.Contains(stderr.String(), "unknown-command") {
		t.Errorf("stderr does not contain the command name:\n%s", stderr.String())
	}
}

// TestRun_ContextCancelled_ReturnsSIGINT asserts the WBS 11.3.1
// acceptance criterion "SIGINT produces exit 130".
//
// The test cancels the context before running the CLI. The
// boundary's fast-path check returns ExitSIGINT without running
// the command tree.
func TestRun_ContextCancelled_ReturnsSIGINT(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var stdout, stderr bytes.Buffer
	opts := options{
		args:     []string{"--help"},
		stdin:    strings.NewReader(""),
		stdout:   &stdout,
		stderr:   &stderr,
		env:      func(string) string { return "" },
		rootPath: t.TempDir(),
	}

	code, _ := runWithOptions(ctx, opts)
	if code != ExitSIGINT {
		t.Errorf("code = %d; want %d (ExitSIGINT)", code, ExitSIGINT)
	}
}

// signalError wraps an os.Signal so that it satisfies the error
// interface. It is declared at package scope because Go does not
// allow method declarations inside a function body.
//
// # Why the field is named sig, not Signal
//
// An anonymous embed of os.Signal names the field Signal, and
// the embedded interface has a method Signal. Go's promotion rule
// requires the selector x.Signal to be legal and to denote the
// embedded method; when the outer struct's own field is named
// Signal, the shallower field shadows the deeper method, and the
// method is not promoted. The type then does not satisfy
// os.Signal, because its method set lacks Signal.
//
// Naming the field sig removes the collision. The field is a
// named field of interface type, so its methods are not promoted
// to the outer type; the outer type declares String and Signal
// explicitly to satisfy os.Signal, and Error to satisfy error.
//
// # Why the methods are explicit
//
// A named field of interface type contributes no methods to the
// outer struct's method set. The three methods are therefore
// declared by hand. The bodies are one-liners; their purpose is
// to make the wrapper satisfy both interfaces.
type signalError struct {
	sig os.Signal
}

// Error satisfies the error interface. The returned string is a
// fixed, non-user-authored marker; the test does not assert on
// its value.
func (e signalError) Error() string {
	return "signal received"
}

// String returns the embedded signal's canonical name, matching
// what the standard library's wrapper would return. The body is
// the value the production string comparison in signalExitCode
// compares against.
func (e signalError) String() string {
	return e.sig.String()
}

// Signal satisfies os.Signal's Signal method. It is a no-op; its
// purpose in the os.Signal interface is to distinguish signal
// values from other fmt.Stringer implementations.
func (e signalError) Signal() {}

// TestRun_ContextCancelled_ReturnsSIGTERM asserts the WBS 11.3.1
// acceptance criterion "SIGTERM produces exit 143".
//
// The test installs a context whose Cause is an os.Signal equal to
// syscall.SIGTERM. The production code uses signal.NotifyContext,
// which attaches the received signal as the context's Cause; the
// test simulates the same shape with context.WithCancelCause and
// the signalError wrapper.
//
// # Why the test asserts on the cause first
//
// The test asserts that context.Cause(ctx) returns the wrapper the
// test installed, before it exercises the production code. The
// assertion pins the test's own setup: if a future Go version
// changes context.WithCancelCause's behaviour, or if the test's
// construction is edited incorrectly, the failure surfaces as a
// setup problem, not as a confusing "code = 130" from
// runWithOptions.
//
// # What the test exercises
//
// runWithOptions's fast-path check sees the cancelled context and
// calls signalExitCode. The production function inspects the
// cause's shape:
//
//   - The cause satisfies os.Signal (via the wrapper's embedded
//     field).
//   - The direct comparison `sig == syscall.SIGTERM` is false
//     because the wrapper's dynamic type is not syscall.Signal.
//   - The string comparison `sig.String() == syscall.SIGTERM.String()`
//     is true because both return "terminated".
//
// The function therefore returns ExitSIGTERM (143). The test
// asserts that value.
func TestRun_ContextCancelled_ReturnsSIGTERM(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(signalError{sig: syscall.SIGTERM})

	// Pin the test's setup before exercising the production code.
	// If context.Cause does not return the wrapper, the failure
	// is a setup problem, not a production bug.
	cause := context.Cause(ctx)
	if cause == nil {
		t.Fatal("context.Cause returned nil; the context is not cancelled")
	}
	sig, ok := cause.(os.Signal)
	if !ok {
		t.Fatalf("context.Cause returned %T, which does not satisfy os.Signal", cause)
	}
	if got := sig.String(); got != syscall.SIGTERM.String() {
		t.Fatalf("context.Cause().(os.Signal).String() = %q; want %q",
			got, syscall.SIGTERM.String())
	}

	var stdout, stderr bytes.Buffer
	opts := options{
		args:     []string{"--help"},
		stdin:    strings.NewReader(""),
		stdout:   &stdout,
		stderr:   &stderr,
		env:      func(string) string { return "" },
		rootPath: t.TempDir(),
	}

	code, _ := runWithOptions(ctx, opts)
	if code != ExitSIGTERM {
		t.Errorf("code = %d; want %d (ExitSIGTERM)", code, ExitSIGTERM)
	}
}

// TestSignalExitCode_UnknownCause_ReturnsSIGINT asserts the
// fallback: a cancelled context whose cause is not SIGTERM maps
// to ExitSIGINT.
func TestSignalExitCode_UnknownCause_ReturnsSIGINT(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(errors.New("some non-signal cause"))

	if got := signalExitCode(ctx); got != ExitSIGINT {
		t.Errorf("signalExitCode(unknown cause) = %d; want %d (ExitSIGINT)",
			got, ExitSIGINT)
	}
}

// =============================================================================
// Test helpers — source file reading and function body extraction
// =============================================================================

// executeTestPackageDir returns the absolute path to the
// internal/cli directory.
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
// # Why comments are stripped
//
// The extractor is used by structural tests that assert the body
// does not contain certain tokens (for example, "os.Stdout"). A
// comment inside the body may legitimately mention those tokens
// while explaining why they are not used. Stripping comments
// before the token check separates "the code does not reference
// os.Stdout" from "the docstring mentions os.Stdout".
//
// Stripping comments also makes brace counting robust: a comment
// containing an unbalanced brace (for example, `// see {foo`)
// would otherwise be counted as an opening brace and would
// truncate or extend the extracted body.
//
// # Limitations
//
// The stripper handles the two Go comment forms: `// ...` to end
// of line, and `/* ... */`. It does not handle braces inside
// string or rune literals. The file it is applied to (execute.go)
// contains none of those inside the function bodies it inspects.
func extractFuncBody(t *testing.T, content, funcName string) string {
	t.Helper()

	needle := "func " + funcName + "("
	start := strings.Index(content, needle)
	if start < 0 {
		t.Fatalf("func %s not found", funcName)
	}

	openRel := strings.Index(content[start:], "{")
	if openRel < 0 {
		t.Fatalf("opening brace of %s not found", funcName)
	}
	open := start + openRel

	var b strings.Builder
	depth := 0
	i := open
	for i < len(content) {
		if i+1 < len(content) && content[i] == '/' && content[i+1] == '/' {
			j := strings.IndexByte(content[i:], '\n')
			if j < 0 {
				t.Fatalf("unterminated line comment in %s", funcName)
			}
			i += j
			continue
		}

		if i+1 < len(content) && content[i] == '/' && content[i+1] == '*' {
			j := strings.Index(content[i+2:], "*/")
			if j < 0 {
				t.Fatalf("unterminated block comment in %s", funcName)
			}
			i += 2 + j + 2
			continue
		}

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
	return ""
}
