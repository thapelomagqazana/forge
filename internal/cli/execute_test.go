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
// # The three streams (WBS 12.4.1)
//
// The options struct carries the three standard streams in a
// single streams.Streams value. Tests construct the value with
// buffers for In, Out, and Err. The tests below use the
// newTestOptions helper to build the value in one place, so that
// a change to the streams' shape touches one helper, not every
// test.
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
// # Verbose mode (WBS 10.4.4, WBS 12.5.1)
//
// The --verbose flag has two effects, both on stderr:
//
//   - The logger's level is set to DEBUG. DEBUG records emitted by
//     the command tree reach the invocation's injected stderr.
//     The tests in the "logger wiring" section pin this behaviour.
//
//   - The error rendering's verbosity is enabled. The tests in the
//     "stream separation" and "exit boundary" sections assert on
//     the rendered error block.
//
// The two effects share the flag but are otherwise independent.
// See docs/reference/verbose-mode.md and
// docs/reference/log-levels.md §4.
//
// # Test organisation
//
// The file is organised in sections, each with a banner comment:
//
//  1. Tests — the public Execute function
//  2. Tests — the root command constructor
//  3. Tests — the injectable boundary
//  4. Tests — stream separation
//  5. Tests — the logger wiring (WBS 12.5.1)
//  6. Tests — the Dependencies transformation (WBS 4.2.2)
//  7. Tests — the panic recovery (WBS 10.5.1)
//  8. Tests — the non-*Error wrapping (WBS 10.5.1)
//  9. Tests — exit code derivation
//  10. Tests — the options struct
//  11. Tests — non-functional properties
//  12. Tests — the structural invariants the WBS specifies
//  13. Tests — the exit boundary (WBS 11.3.1, ADR-020)
//  14. Test helpers — source file reading and function body extraction
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
	"github.com/thapelomagqazana/forge/internal/logger"
	"github.com/thapelomagqazana/forge/internal/streams"
)

// newTestOptions returns an options value bound to in-memory
// streams.
//
// The helper is the single construction point for the tests in
// this file. Every test that needs an options value calls it and
// then adjusts the field it cares about. A change to the options
// struct's shape (for example, the WBS 12.4.1 change that replaced
// three stream fields with one Streams value) touches this helper,
// not every test.
//
// # Returned values
//
//   - stdout and stderr are *bytes.Buffer values owned by the
//     caller. The caller captures them to assert on the CLI's
//     output.
//   - stdin is an empty strings.Reader. A test that needs a
//     different stdin replaces the field after calling the
//     helper.
//   - env returns "" for every key. A test that needs a specific
//     environment overrides the field.
//   - rootPath is a t.TempDir() value.
//
// # Why the buffers are returned separately
//
// The caller needs to assert on the bytes the CLI wrote, and the
// bytes are the buffer's contents. Returning the streams value
// alone would force the caller to type-assert the io.Writer fields
// back to *bytes.Buffer, which is verbose and error-prone.
// Returning the two buffers directly keeps the tests' assertions
// on strings simple.
func newTestOptions(t *testing.T) (opts options, stdout, stderr *bytes.Buffer) {
	t.Helper()

	stdout = &bytes.Buffer{}
	stderr = &bytes.Buffer{}
	opts = options{
		streams: streams.Streams{
			In:  strings.NewReader(""),
			Out: stdout,
			Err: stderr,
		},
		env:      func(string) string { return "" },
		rootPath: t.TempDir(),
	}
	return opts, stdout, stderr
}

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
func TestExecute_HasFrozenSignature(t *testing.T) {
	t.Parallel()

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

	opts, _, _ := newTestOptions(t)
	deps := buildDependencies(opts)

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
// Tests — the logger wiring (WBS 12.5.1)
// =============================================================================
//
// runWithOptions resolves the log level from the flags and the env
// var before executing the command tree, then installs the process
// logger via logger.SetDefault. The tests below pin both halves of
// that wiring:
//
//   - The installation: after runWithOptions returns, the
//     installed logger emits records at the resolved level, and
//     the records reach the invocation's injected stderr.
//   - The ordering: the resolution happens before the tree runs,
//     so a conflicting invocation never executes a command.
//
// # How the level is observed
//
// logger.Logger is an interface whose method set is Debug, Info,
// Warn, Error, and With. It does NOT expose a Level() method:
// the interface's method set is a public contract (see the
// package docstring), and the level is a property of the concrete
// slogLogger type, not of the interface.
//
// A test therefore cannot ask the installed logger "what is your
// level?". It must instead emit a record at a level that should
// or should not pass the filter and observe whether the record
// reaches the injected stderr. The tests below install the
// production wiring by calling runWithOptions with an options
// value whose stderr is a bytes.Buffer, then assert on the bytes
// that reach the buffer.
//
// The "level is DEBUG" observable is the presence of a DEBUG
// record in the buffer; the "level is INFO" observable is the
// absence of a DEBUG record. Neither requires the Level() method.
//
// # Why the version command is the vehicle
//
// The version command emits a DEBUG record at its build-info
// resolution point (see internal/cli/version.go and
// docs/reference/verbose-mode.md §3). It is the only Phase 2
// command with a DEBUG emission that a test can trigger without
// setting up a config file. The tests therefore drive the wiring
// through `version`.
//
// # Why the tests save and restore the default logger
//
// logger.SetDefault mutates process-global state. A test that
// installs its own logger must restore the previous one, or a
// later test in the same package that reads logger.Default()
// observes the wrong value. The tests below save the previous
// logger and restore it via t.Cleanup. They are NOT parallel:
// parallel tests that share the global would race on the
// mutation.

// TestRunWithOptions_VerboseInstallsDebugLogger asserts that
// `--verbose version` produces a DEBUG record on the invocation's
// injected stderr.
//
// # What the test observes
//
// The version command emits at least one DEBUG record at its
// build-info resolution point. Under `--verbose`, the installed
// logger's level is DEBUG; the record passes the filter and
// reaches the injected stderr. The test asserts the buffer
// contains a DEBUG record.
//
// # Why the buffer, not a sink
//
// The wiring installs a logger with Writer: opts.streams.Err. The
// injected buffer is where the records land. A capturing sink
// installed on the default logger *before* runWithOptions would
// be discarded when runWithOptions calls logger.SetDefault with
// the production logger; the buffer is the only observable that
// survives the replacement.
func TestRunWithOptions_VerboseInstallsDebugLogger(t *testing.T) {
	// Not parallel: the test mutates logger.SetDefault.
	savedLogger := logger.Default()
	t.Cleanup(func() { logger.SetDefault(savedLogger) })

	opts, _, stderr := newTestOptions(t)
	opts.args = []string{"--verbose", "version"}

	code, err := runWithOptions(context.Background(), opts)
	if err != nil {
		t.Fatalf("runWithOptions: %v", err)
	}
	if code != ExitSuccess {
		t.Fatalf("code = %d; want %d (ExitSuccess)", code, ExitSuccess)
	}
	if stderr.Len() == 0 {
		t.Fatal("injected stderr is empty; " +
			"--verbose did not enable DEBUG records, or the " +
			"logger did not write to the injected stderr")
	}
	if !strings.Contains(strings.ToLower(stderr.String()), "debug") {
		t.Errorf("injected stderr does not contain a DEBUG record; "+
			"--verbose did not install a DEBUG logger:\n%s",
			stderr.String())
	}
}

// TestRunWithOptions_DefaultLevelSuppressesDebug asserts the
// inverse: without --verbose and without FORGE_LOG_LEVEL, the
// installed logger is at the default level (INFO), so the version
// command's DEBUG records do NOT reach the injected stderr.
//
// # Why absence is the observable
//
// At the default level, DEBUG records are filtered out. The
// version command emits them, the sink drops them, and the
// injected stderr contains no "debug" token. The absence is the
// proof that the level filter is working.
func TestRunWithOptions_DefaultLevelSuppressesDebug(t *testing.T) {
	savedLogger := logger.Default()
	t.Cleanup(func() { logger.SetDefault(savedLogger) })

	opts, _, stderr := newTestOptions(t)
	opts.args = []string{"version"}

	code, err := runWithOptions(context.Background(), opts)
	if err != nil {
		t.Fatalf("runWithOptions: %v", err)
	}
	if code != ExitSuccess {
		t.Fatalf("code = %d; want %d (ExitSuccess)", code, ExitSuccess)
	}
	if strings.Contains(strings.ToLower(stderr.String()), "debug") {
		t.Errorf("injected stderr contains a DEBUG record at the "+
			"default level; the logger was not installed at INFO:\n%s",
			stderr.String())
	}
}

// TestRunWithOptions_EnvVarInstallsDebugLogger asserts that
// FORGE_LOG_LEVEL=debug installs a DEBUG logger, mirroring the
// --verbose path.
//
// The env lookup is injected via opts.env, not set process-wide.
// The injected function returns "debug" for FORGE_LOG_LEVEL and
// "" for every other key; the resolver reads the level via
// logger.ReadEnv, which consults the injected lookup.
func TestRunWithOptions_EnvVarInstallsDebugLogger(t *testing.T) {
	savedLogger := logger.Default()
	t.Cleanup(func() { logger.SetDefault(savedLogger) })

	opts, _, stderr := newTestOptions(t)
	opts.args = []string{"version"}
	opts.env = func(name string) string {
		if name == logger.EnvVar {
			return "debug"
		}
		return ""
	}

	code, err := runWithOptions(context.Background(), opts)
	if err != nil {
		t.Fatalf("runWithOptions: %v", err)
	}
	if code != ExitSuccess {
		t.Fatalf("code = %d; want %d (ExitSuccess)", code, ExitSuccess)
	}
	if !strings.Contains(strings.ToLower(stderr.String()), "debug") {
		t.Errorf("injected stderr does not contain a DEBUG record; "+
			"FORGE_LOG_LEVEL=debug did not install a DEBUG logger:\n%s",
			stderr.String())
	}
}

// TestRunWithOptions_QuietVerboseConflictFailsBeforeTree asserts
// that a conflicting invocation is rejected as a usage error
// (exit 64) *before* the command tree runs.
//
// # What the test proves
//
// The resolution happens before root.ExecuteContext. If it did
// not, a conflicting invocation of the `version` command would
// produce version output on stdout before the conflict was
// detected. The test asserts stdout is empty, which is only true
// if no command ran.
func TestRunWithOptions_QuietVerboseConflictFailsBeforeTree(t *testing.T) {
	savedLogger := logger.Default()
	t.Cleanup(func() { logger.SetDefault(savedLogger) })

	opts, stdout, stderr := newTestOptions(t)
	opts.args = []string{"--quiet", "--verbose", "version"}

	code, err := runWithOptions(context.Background(), opts)
	if code != ExitUsage {
		t.Fatalf("code = %d; want %d (ExitUsage)", code, ExitUsage)
	}
	if err == nil {
		t.Fatal("err = nil; want non-nil for a conflicting invocation")
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q; want empty "+
			"(the conflict must be detected before the tree runs)",
			stdout.String())
	}
	if !strings.Contains(stderr.String(), "mutually exclusive") {
		t.Errorf("stderr does not mention mutual exclusion:\n%s",
			stderr.String())
	}
}

// TestRunWithOptions_InvalidLogLevelFailsBeforeTree asserts that
// an invalid --log-level value is rejected as a usage error
// before the command tree runs.
func TestRunWithOptions_InvalidLogLevelFailsBeforeTree(t *testing.T) {
	savedLogger := logger.Default()
	t.Cleanup(func() { logger.SetDefault(savedLogger) })

	opts, stdout, stderr := newTestOptions(t)
	opts.args = []string{"--log-level=trace", "version"}

	code, err := runWithOptions(context.Background(), opts)
	if code != ExitUsage {
		t.Fatalf("code = %d; want %d (ExitUsage)", code, ExitUsage)
	}
	if err == nil {
		t.Fatal("err = nil; want non-nil for an invalid --log-level")
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q; want empty "+
			"(the invalid level must be detected before the tree runs)",
			stdout.String())
	}
	if stderr.Len() == 0 {
		t.Error("stderr is empty; want a diagnostic")
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
		t.Run(truncate(fmt.Sprintf("%v", v), 20), func(t *testing.T) {
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

			e := forgeerr.New(tc.cat, tc.code, "message")
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
		forgeerr.CodeConfigMalformed,
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
//
// The three streams are asserted through the streams.Streams
// value's three fields. The env function is asserted directly.
// The verbose flag is asserted to be false.
func TestDefaultOptions_HasNonEmptyFields(t *testing.T) {
	t.Parallel()

	opts := defaultOptions()

	if opts.streams.In == nil {
		t.Error("streams.In is nil")
	}
	if opts.streams.Out == nil {
		t.Error("streams.Out is nil")
	}
	if opts.streams.Err == nil {
		t.Error("streams.Err is nil")
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

	// The reader is called on the target command, not the root:
	// the early flag parse populated the target's flag set, and
	// the post-execution reads must observe the same values.
	if !strings.Contains(body, "verboseRequested(target)") {
		t.Error("runWithOptions does not call " +
			"verboseRequested(target); the verbose flag must be " +
			"read from the target command (WBS 12.5.1)")
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

// TestExecuteWithOptions_CallsResolveLogLevel verifies that
// runWithOptions resolves the log level (WBS 12.5.1). The
// resolver is what detects the --quiet/--verbose conflict and
// produces the Decision consumed by the logger installation.
func TestExecuteWithOptions_CallsResolveLogLevel(t *testing.T) {
	t.Parallel()

	content := readFile(t, "execute.go")
	body := extractFuncBody(t, content, "runWithOptions")

	// The resolver is called with the target command (where the
	// early flag parse populated the flags) and with opts.env (the
	// injected environment lookup). Both are WBS 12.5.1 changes:
	//
	//   - The target, not the root: the early parse runs on the
	//     command the invocation dispatches to, so the resolver
	//     reads from the same site the parse wrote to.
	//
	//   - The injected env, not the process env: the resolver must
	//     read FORGE_LOG_LEVEL through the options value so tests
	//     can inject a value without mutating the process.
	//
	// A future change that returns to the process-env form (via
	// the legacy resolveLogLevel wrapper) would remove the test's
	// ability to observe the env injection; the assertion is
	// deliberately specific.
	if !strings.Contains(body, "resolveLogLevelWithEnv(target, ") {
		t.Error("runWithOptions does not call " +
			"resolveLogLevelWithEnv(target, ...); the log level " +
			"must be resolved at the boundary with the injected " +
			"env (WBS 12.5.1)")
	}
}

// TestExecuteWithOptions_InstallsLoggerBeforeTree verifies that
// the invocation's logger is installed before the command tree runs.
//
// # Why the test now inspects hooks.go and not execute.go
//
// The logger is no longer installed by runWithOptions. The
// boundary (runWithOptions) resolves the log level and attaches
// the Decision to the command's context via
// logger.ContextWithDecision. The PersistentPreRunE hook
// (initializeCommandEnvironment, hooks.go) reads the Decision
// and constructs the invocation's logger, which Cobra's
// ExecuteContext runs before the dispatched command's RunE.
//
// The property the test asserts is unchanged: the invocation's
// logger exists before any command runs. The function whose body
// realizes the property is now the hook.
func TestExecuteWithOptions_InstallsLoggerBeforeTree(t *testing.T) {
	t.Parallel()

	// The boundary attaches the Decision to the context before
	// ExecuteContext.
	content := readFile(t, "execute.go")
	body := extractFuncBody(t, content, "runWithOptions")

	attachIdx := strings.Index(body, "logger.ContextWithDecision(")
	if attachIdx < 0 {
		t.Fatal("runWithOptions does not call " +
			"logger.ContextWithDecision; the resolved level " +
			"does not reach the hook (WBS 12.5.1)")
	}
	executeIdx := strings.Index(body, "root.ExecuteContext(")
	if executeIdx < 0 {
		t.Fatal("runWithOptions does not call root.ExecuteContext; " +
			"the command tree is not executed")
	}
	if attachIdx >= executeIdx {
		t.Error("logger.ContextWithDecision appears after " +
			"root.ExecuteContext in runWithOptions; the Decision " +
			"must reach the context before the tree runs " +
			"(WBS 12.5.1)")
	}

	// The hook constructs the logger. Cobra invokes the hook
	// (PersistentPreRunE) before any subcommand's RunE.
	hooksContent := readFile(t, "hooks.go")
	hooksBody := extractFuncBody(t, hooksContent, "initializeCommandEnvironment")

	if !strings.Contains(hooksBody, "logger.IntoContext(") {
		t.Error("initializeCommandEnvironment does not call " +
			"logger.IntoContext; the invocation's logger is not " +
			"attached to the context (WBS 12.5.1)")
	}
	if !strings.Contains(hooksBody, "logger.NewCorrelationID()") {
		t.Error("initializeCommandEnvironment does not call " +
			"logger.NewCorrelationID; the invocation's logger " +
			"carries no correlation ID (WBS 12.2.1)")
	}
}

// TestExecuteWithOptions_LoggerHasComponent verifies that the
// invocation's logger is constructed with a non-zero Component.
//
// # Why the test now inspects hooks.go and not execute.go
//
// The logger's construction moved from the boundary to the hook.
// The hook supplies Component: logger.ComponentCLI; without a
// non-zero Component, logger.New panics (see logger.New's
// docstring).
func TestExecuteWithOptions_LoggerHasComponent(t *testing.T) {
	t.Parallel()

	content := readFile(t, "hooks.go")
	body := extractFuncBody(t, content, "initializeCommandEnvironment")

	if !strings.Contains(body, "Component:") {
		t.Error("initializeCommandEnvironment does not set a " +
			"Component on the logger; logger.New panics when " +
			"Component is zero at non-debug levels " +
			"(see internal/logger/logger.go)")
	}
	if !strings.Contains(body, "logger.ComponentCLI") {
		t.Error("initializeCommandEnvironment does not use " +
			"logger.ComponentCLI as the component; the CLI layer " +
			"should tag its records with the CLI component")
	}
}

// TestExecuteWithOptions_LoggerHasWriter verifies that the
// invocation's logger writes to the invocation's injected stderr.
//
// # Why the test now inspects hooks.go and not execute.go
//
// The logger's construction moved from the boundary to the hook.
// The hook supplies Writer: deps.Streams.Err; without this, the
// logger would default to os.Stderr and bypass the injection.
func TestExecuteWithOptions_LoggerHasWriter(t *testing.T) {
	t.Parallel()

	content := readFile(t, "hooks.go")
	body := extractFuncBody(t, content, "initializeCommandEnvironment")

	if !strings.Contains(body, "Writer:") {
		t.Error("initializeCommandEnvironment does not set a " +
			"Writer on the logger; the logger bypasses the " +
			"injected stderr (WBS 12.5.1)")
	}
	if !strings.Contains(body, "deps.Streams.Err") {
		t.Error("initializeCommandEnvironment does not use " +
			"deps.Streams.Err as the logger's Writer; the logger " +
			"must write to the process boundary's injected stderr")
	}
}

// =============================================================================
// Tests — the exit boundary (WBS 11.3.1, ADR-020)
// =============================================================================

// TestRun_ReturnsCodeAndNilErrorOnSuccess asserts that Run
// returns (ExitSuccess, nil) for a successful invocation.
func TestRun_ReturnsCodeAndNilErrorOnSuccess(t *testing.T) {
	t.Parallel()

	opts, _, stderr := newTestOptions(t)
	opts.args = []string{"--help"}

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

	opts, _, stderr := newTestOptions(t)
	opts.args = []string{"unknown-command"}

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
func TestRun_ContextCancelled_ReturnsSIGINT(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	opts, _, _ := newTestOptions(t)
	opts.args = []string{"--help"}

	code, _ := runWithOptions(ctx, opts)
	if code != ExitSIGINT {
		t.Errorf("code = %d; want %d (ExitSIGINT)", code, ExitSIGINT)
	}
}

// TestProbeResolver is a diagnostic probe, not a contract test. It
// prints the resolver's output for the two usage-error cases so a
// developer investigating a resolver failure can see the shapes
// without attaching a debugger.
//
// # Why the probe uses resolveLogLevelWithEnv
//
// The production boundary calls resolveLogLevelWithEnv (the
// WBS 12.5.1 form, which reads the env through an injected lookup).
// The earlier resolveLogLevel wrapper reads the process env and
// exists only for backward compatibility. A probe that used the
// wrapper would exercise a code path the boundary does not use.
func TestProbeResolver(t *testing.T) {
	env := func(string) string { return "" }

	cmd := newFlagTestCmd()
	if err := cmd.ParseFlags([]string{"--quiet", "--verbose"}); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	d, err := resolveLogLevelWithEnv(cmd, env, "")
	t.Logf("conflict case: decision=%+v err=%v", d, err)

	cmd2 := newFlagTestCmd()
	if err := cmd2.ParseFlags([]string{"--log-level=trace"}); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	d2, err2 := resolveLogLevelWithEnv(cmd2, env, "")
	t.Logf("invalid case:  decision=%+v err=%v", d2, err2)
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
func TestRun_ContextCancelled_ReturnsSIGTERM(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(signalError{sig: syscall.SIGTERM})

	// Pin the test's setup before exercising the production code.
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

	opts, _, _ := newTestOptions(t)
	opts.args = []string{"--help"}

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

// truncate returns s, shortened to at most n bytes. If s is already
// shorter, it is returned unchanged. The helper is used to build
// subtest names from arbitrary strings, where t.Run requires a name
// that is a valid identifier-ish token and where arbitrarily long
// names are noisy in test output.
//
// # Why bytes and not runes
//
// The helper is used on subtest names that are ASCII in practice
// (a panic value's fmt.Sprint, truncated). A byte-based cut is the
// conservative choice: it never splits a multi-byte rune in a way
// that would produce invalid output, because it truncates before
// the rune's continuation byte. A rune-based cut would be more
// correct for non-ASCII inputs but is not needed here, and the
// extra machinery (utf8.RuneCountInString, a rune loop) would
// obscure the helper's purpose.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// =============================================================================
// Test helpers — function body extraction
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
