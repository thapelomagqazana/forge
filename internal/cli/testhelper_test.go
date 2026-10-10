// Package cli contains white-box tests for the CLI package.
//
// This file defines the shared test helper used by every command test
// in the package. The helper exercises the CLI through the same
// boundary production code uses (executeWithOptions) with synthetic
// inputs, and captures the observable outputs (exit code, stdout,
// stderr).
//
// # Why a shared helper
//
// Before WBS 4.3.2, each test file defined its own helper or
// constructed a Cobra command directly. The two patterns produced
// subtly different tests: helpers that went through executeWithOptions
// exercised the full transformation (options → Dependencies → command
// tree); helpers that constructed a Cobra command exercised only the
// command. Both are valid, but having two patterns in the same package
// is confusing and produces tests that cannot be compared.
//
// The shared helper resolves the ambiguity: every command test in the
// package goes through executeWithOptions.
//
// # Why no subprocess
//
// The helper does not spawn a subprocess. Spawning is slow
// (milliseconds per case instead of microseconds), brittle (it depends
// on the test binary being built), and opaque (it cannot inspect
// internal state). The helper covers everything a subprocess-based
// test would cover, except process-level concerns like main.go's call
// to os.Exit. Those are covered by integration tests in
// cmd/forge/binary_integration_test.go, behind the `integration` build
// tag.
//
// # What the helper captures
//
// The helper returns a testRun value with three fields:
//
//   - exitCode — the value returned by executeWithOptions.
//   - stdout   — the content written to the injected stdout writer.
//   - stderr   — the content written to the injected stderr writer.
//
// The three fields are sufficient to assert on every user-observable
// behaviour of a command: what it prints, where it prints it, and
// whether it succeeded.
package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/thapelomagqazana/forge/internal/config"
	"github.com/thapelomagqazana/forge/internal/filesystem"
)

// testRun captures the observable result of a CLI invocation.
//
// It is the return type of the runCLI helper. Every field is
// deterministic: the same args and env produce the same testRun,
// byte for byte.
//
// # Why exported fields
//
// The fields are exported (capitalised) so that tests can construct a
// testRun literal for comparison. A common pattern is:
//
//	want := testRun{exitCode: ExitSuccess, stdout: "..."}
//	got := runCLI(t, []string{"version"}, nil)
//	if got != want { ... }
//
// The fields are not part of the package's public API; the struct is
// unexported, and no test outside `package cli` can name it.
type testRun struct {
	// exitCode is the value returned by executeWithOptions. It is
	// the same value main.go passes to os.Exit.
	exitCode int

	// stdout is the content written to the injected stdout writer.
	stdout string

	// stderr is the content written to the injected stderr writer.
	stderr string
}

// runCLI invokes the CLI with the given arguments and environment and
// returns the captured observable result.
//
// The helper:
//
//  1. Constructs an options value with synthetic inputs. stdout and
//     stderr are fresh bytes.Buffers; stdin is an empty reader; env
//     is a lookup function over the caller-supplied map.
//  2. Calls executeWithOptions with the options value.
//  3. Returns a testRun containing the exit code and the captured
//     writers.
//
// The helper does not spawn a subprocess, does not touch the real
// process environment, and does not read or write the filesystem
// except through the CLI's own code paths.
//
// # Argument shape
//
// args is the argument list as the user would type it, excluding the
// program name. The helper passes it to options.args unchanged. An
// empty or nil slice is treated by the CLI as "no subcommand"; a slice
// whose first element is a command name dispatches to that command.
//
// # Environment shape
//
// env is a map of environment variables visible to the CLI during the
// invocation. The helper converts it to a lookup function:
//
//	env: func(k string) string { return env[k] }
//
// A nil map is valid and produces a lookup that always returns the
// empty string. This matches the behaviour of a real process in which
// no environment variables are set.
//
// Tests that need to assert on the absence of a variable pass a map
// that does not contain the key. Tests that need to assert on the
// presence of a variable pass a map that contains it.
//
// # Concurrency
//
// runCLI is safe for concurrent use. Every call allocates its own
// buffers and its own options value. No package-level state is
// touched. Tests may call runCLI from parallel subtests without
// synchronisation.
//
// # Why the helper takes *testing.T
//
// The helper takes *testing.T so that it can call t.Helper(). The
// t.Helper() call marks the helper's frame as not-a-test, so that a
// failure inside the helper (for example, a panic from a nil writer)
// is reported with the caller's file and line number, not the
// helper's. This is the standard convention for test helpers and is
// what makes failures easy to locate.
func runCLI(t *testing.T, args []string, env map[string]string) testRun {
	t.Helper()

	var stdout, stderr bytes.Buffer

	opts := options{
		args:     args,
		stdin:    strings.NewReader(""),
		stdout:   &stdout,
		stderr:   &stderr,
		env:      func(k string) string { return env[k] },
		rootPath: "",
	}

	code := executeWithOptions(opts)

	return testRun{
		exitCode: code,
		stdout:   stdout.String(),
		stderr:   stderr.String(),
	}
}

// runCLIWithRoot is the same as runCLI but binds the CLI's filesystem
// abstraction to the given root directory.
//
// The helper exists for commands that resolve paths relative to a
// root. In Phase 2, no command does; the helper is provided so that
// future command tests have a single place to set the root, rather
// than constructing options values inline.
func runCLIWithRoot(t *testing.T, args []string, env map[string]string, rootPath string) testRun {
	t.Helper()

	var stdout, stderr bytes.Buffer

	opts := options{
		args:     args,
		stdin:    strings.NewReader(""),
		stdout:   &stdout,
		stderr:   &stderr,
		env:      func(k string) string { return env[k] },
		rootPath: rootPath,
	}

	code := executeWithOptions(opts)

	return testRun{
		exitCode: code,
		stdout:   stdout.String(),
		stderr:   stderr.String(),
	}
}

// testDependencies returns a Dependencies value with non-nil writers,
// suitable for tests that construct commands but do not execute them.
//
// # What the helper provides
//
//   - Stdout and Stderr are fresh bytes.Buffer values. Tests that
//     want to assert on output should call runCLI instead, which
//     captures the writers it allocates.
//
//   - Config is a fresh &config.Config{}. In Phase 2 the struct is
//     empty; the pointer is non-nil so that commands may read its
//     fields without a nil check.
//
//   - Logger is a no-op logger. Tests that want to assert on log
//     output construct their own logger and pass it in a Dependencies
//     value they build themselves.
//
//   - FS is an OS-backed filesystem bound to an empty root. Tests
//     that need a specific root call runCLIWithRoot instead.
//
//   - Env is a lookup function that always returns the empty string.
//     Tests that want to inject environment variables call runCLI
//     with an env map.
//
// # When to use this helper
//
// Use testDependencies for tests that construct a command and
// inspect its metadata (Use, Short, Long, Hidden, Args) but do not
// execute it. For tests that execute a command, use runCLI, which
// captures the command's observable output.
//
// # Why the helper is not named newTestDependencies
//
// The name follows the convention used by the package's other test
// helpers (testRun, testHelper, etc.): a noun phrase beginning with
// "test" describes a value or a fixture; a verb phrase beginning
// with "new" describes a constructor that produces a distinct value
// on every call. testDependencies produces a distinct value on every
// call, but its name is read as "the test dependencies", which is a
// stable, reusable fixture. The two conventions are documented in
// the package docstring of testhelper_test.go.
//
// # Concurrency
//
// testDependencies is safe for concurrent use. Every call allocates
// its own buffers and its own Dependencies value. No package-level
// state is touched.
func testDependencies() Dependencies {
	return Dependencies{
		Config: &config.Config{},
		Logger: newNoopLogger(),
		FS:     filesystem.NewOSFS(""),
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Env:    func(string) string { return "" },
	}
}

// helpCommandNames returns the names of the visible commands listed
// in any Cobra help output, in the order they appear.
//
// # What "any help output" means
//
// Cobra's help output has the same structure regardless of whether
// it was produced by `forge --help`, `forge help`, or
// `forge version --help`: it contains a "Usage:" section, an
// "Available Commands:" section (when the command has subcommands),
// a "Flags:" section, and a closing line that points at
// `--help` for more information.
//
// The helper extracts the command names from the "Available
// Commands:" section. For a leaf command (like `version`), the
// section is absent and the helper returns nil.
//
// # Why the helper is general
//
// The same parsing logic is used by:
//
//   - TestRoot_HelpOutput_MatchesRegistry (checks that the visible
//     commands match the registry)
//   - The WBS 5.2.2 help contract tests (check that the help output
//     lists every command regardless of which invocation produced
//     it)
//
// Sharing the logic ensures the two sets of tests agree on what
// "the help output" contains. If a future change to Cobra's help
// format requires updating the parser, both tests see the update.
func helpCommandNames(help string) []string {
	return extractVisibleCommandNamesFromHelp(help)
}

// assertOnlyStdout verifies that got has output on stdout and none
// on stderr.
//
// # Why the helper exists
//
// The output stream boundary (WBS 7.4.1) requires each piece of
// output to go to exactly one stream. A test that asserts the
// boundary for a successful command uses this helper: it asserts
// that stdout is non-empty and stderr is empty. The helper
// produces a specific failure message if either assertion fails.
//
// # What the helper does not assert
//
// The helper does not assert the content of stdout. Content is the
// job of the command's own tests. The helper asserts only the
// stream: output on stdout, nothing on stderr.
//
// # Exit code
//
// The helper does not assert the exit code. A caller that wants to
// assert the exit code does so separately. The helper is called
// from tests that have already asserted the exit code (usually
// ExitSuccess).
func assertOnlyStdout(t *testing.T, got testRun) {
	t.Helper()
	if got.stdout == "" {
		t.Errorf("stdout is empty; want output on stdout\n"+
			"stderr: %q", got.stderr)
	}
	if got.stderr != "" {
		t.Errorf("stderr is non-empty; want nothing on stderr\n"+
			"stderr: %q", got.stderr)
	}
}

// assertOnlyStderr verifies that got has output on stderr and none
// on stdout.
//
// # Why the helper exists
//
// The output stream boundary (WBS 7.4.1) requires errors and
// diagnostics to go to stderr. A test that asserts the boundary
// for a failing command uses this helper: it asserts that stderr
// is non-empty and stdout is empty. The helper produces a
// specific failure message if either assertion fails.
//
// # What the helper does not assert
//
// The helper does not assert the content of stderr. Content is the
// job of the command's own tests (or the errors tests). The helper
// asserts only the stream: output on stderr, nothing on stdout.
//
// # Exit code
//
// The helper does not assert the exit code. A caller that wants to
// assert the exit code does so separately. The helper is called
// from tests that have already asserted the exit code (usually
// ExitUsage).
func assertOnlyStderr(t *testing.T, got testRun) {
	t.Helper()
	if got.stderr == "" {
		t.Errorf("stderr is empty; want a diagnostic on stderr\n"+
			"stdout: %q", got.stdout)
	}
	if got.stdout != "" {
		t.Errorf("stdout is non-empty; want nothing on stdout\n"+
			"stdout: %q", got.stdout)
	}
}
