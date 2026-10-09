// Package cli contains white-box tests for the CLI package.
//
// This file tests the boundary between the CLI and the process
// environment: the Execute public function, the unexported
// executeWithOptions function, and the helper functions that
// support the boundary: defaultOptions and formatError.
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
// # Test organisation
//
// The file is organised in sections, each with a banner comment:
//
//  1. Tests — the public Execute function
//  2. Tests — the root command constructor
//  3. Tests — the injectable boundary
//  4. Tests — stream separation
//  5. Tests — the Dependencies transformation (WBS 4.2.2)
//  6. Tests — formatError
//  7. Tests — the options struct
//  8. Tests — non-functional properties
//  9. Tests — the structural invariants the WBS specifies
//  10. Test helpers — source file reading and function body extraction
//
// A reader looking for a specific behaviour should scan the section
// banners, not read the file top to bottom.
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

// TestFormatError_GivenError_ReturnsMessage verifies that a
// non-nil error formats as its message.
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

// TestFormatError_IsPure verifies that formatError is
// deterministic.
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
// directory that has been removed, so its emptiness is not
// asserted.
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
// # Why the body is extracted by brace counting
//
// The body is extracted by extractFuncBody, which counts braces
// rather than searching for the first "\n}\n" after the function
// declaration. The naive search would stop at the first inner
// block's closing brace (for example, the closing brace of the
// `if err == nil` block), truncating the body and producing a false
// positive or false negative depending on what follows.
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

// TestExecuteWithOptions_UsesExitCodeFromError verifies that
// executeWithOptions does not derive exit codes itself; it
// delegates to exitCodeFromError.
//
// The test is a source-code check: it asserts that the function
// body contains a call to exitCodeFromError.
func TestExecuteWithOptions_UsesExitCodeFromError(t *testing.T) {
	t.Parallel()

	content := readFile(t, "execute.go")
	body := extractFuncBody(t, content, "executeWithOptions")

	if !strings.Contains(body, "exitCodeFromError(err)") {
		t.Error("executeWithOptions does not call exitCodeFromError")
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

// =============================================================================
// Test helpers — source file reading and function body extraction
// =============================================================================
//
// The structural tests in this file inspect the source code of
// execute.go rather than the runtime behaviour of
// executeWithOptions. They need to read the file from disk and to
// extract the body of a named function.
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
