//go:build integration

// Package main_test contains integration tests that exercise the
// compiled forge binary as a subprocess.
//
// # Why a separate file
//
// Structural tests of main.go live in main_test.go. They read source
// code as data and run in milliseconds.
//
// The tests in this file are different: they compile the binary and
// invoke it as a subprocess. They are slower (seconds per test) and
// require a working Go toolchain at test time. They are gated behind
// the `integration` build tag so that the default `go test ./...` run
// stays fast.
//
// # What these tests prove
//
// The in-process tests in internal/cli verify that the CLI behaves
// correctly when executed within the test process. These tests verify
// that the same behaviour holds when the CLI is executed as a real
// binary: from a clean process, with os.Args, os.Stdin, os.Stdout,
// and os.Stderr bound to the operating system.
//
// The two layers are complementary:
//
//   - In-process tests are fast and precise. They catch logic errors.
//   - Integration tests are slow and end-to-end. They catch wiring
//     errors: a mismatch between what cli.Run() returns and what
//     os.Exit() receives, a missing build tag, a stale binary, a
//     subprocess behaviour that differs from the in-process behaviour.
//
// # Running these tests
//
// Run them explicitly with:
//
//	go test -tags=integration ./cmd/forge/...
//
// Or through the Taskfile:
//
//	task test:integration
//
// # Test organisation
//
// Every test in this file follows the pattern:
//
//  1. Build the binary once per test binary invocation (via a cached
//     helper, see buildBinary below).
//  2. Invoke the binary with specific arguments.
//  3. Capture stdout, stderr, and exit code.
//  4. Assert on the captured values.
//
// The helper runBinary captures all three observable outputs in a
// single struct so that assertions read cleanly.
package main_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// =============================================================================
// Binary build and execution helpers
// =============================================================================

// binaryPathOnce ensures the forge binary is built exactly once per
// test binary invocation, regardless of how many integration tests
// call buildBinary.
//
// Building the binary is the most expensive part of an integration
// test (roughly one second). Caching the result across tests reduces
// the total suite runtime from N seconds to roughly one second plus
// the runtime of the individual invocations.
//
// The variables below are package-level because sync.Once must be
// shared across all tests in the binary. They are initialised lazily:
// a test that never calls buildBinary never triggers the build.
var (
	binaryPathOnce sync.Once
	binaryPathVal  string
	binaryPathErr  error
)

// buildBinary returns the path to a compiled forge binary.
//
// The binary is compiled once per test binary invocation. Subsequent
// calls return the same path. If the build fails, every subsequent
// call returns the same error.
//
// The build is performed with `go build`, using the current working
// directory as the package to build. The test assumes that the
// current working directory is cmd/forge/, which Go's test framework
// guarantees.
//
// The output is placed in the test binary's temporary directory. Go
// removes the directory when the test binary exits, so no cleanup is
// required.
func buildBinary(t *testing.T) string {
	t.Helper()

	binaryPathOnce.Do(func() {
		dir, err := os.MkdirTemp("", "forge-integration-")
		if err != nil {
			binaryPathErr = err
			return
		}

		// On Windows, the binary needs a .exe suffix to be
		// executable. On Unix, any name works.
		name := "forge"
		if filepath.Ext(os.Args[0]) == ".exe" {
			name = "forge.exe"
		}
		out := filepath.Join(dir, name)

		cmd := exec.Command("go", "build", "-o", out, ".")
		cmd.Dir = "."
		if buildOut, err := cmd.CombinedOutput(); err != nil {
			binaryPathErr = errors.New("build failed: " +
				err.Error() + "\n" + string(buildOut))
			return
		}

		binaryPathVal = out
	})

	if binaryPathErr != nil {
		t.Fatalf("build forge binary: %v", binaryPathErr)
	}
	return binaryPathVal
}

// binaryRun captures the observable result of a single binary
// invocation.
//
// Every field is deterministic for a given binary and given arguments.
// The same binary invoked twice with the same arguments produces the
// same binaryRun.
type binaryRun struct {
	// exitCode is the process exit code.
	// A value of 0 means the process exited normally with code 0.
	// A value of -1 means the process was terminated by a signal
	// (Unix only) or the exit code could not be determined.
	exitCode int

	// stdout is the content the process wrote to its stdout stream.
	stdout string

	// stderr is the content the process wrote to its stderr stream.
	stderr string
}

// runBinary invokes the given binary with the given arguments and
// returns the captured observable result.
//
// The helper:
//
//   - Spawns a fresh subprocess for each invocation.
//   - Captures stdout and stderr into separate buffers.
//   - Captures the process exit code.
//   - Uses the current process's environment, so the subprocess sees
//     whatever environment the test binary was invoked with.
//
// The helper does not set stdin. The forge binary does not read stdin
// in WBS 2.4.2. If a future command reads stdin, this helper must be
// extended to accept an io.Reader.
func runBinary(t *testing.T, binary string, args ...string) binaryRun {
	t.Helper()

	var stdout, stderr bytes.Buffer

	cmd := exec.Command(binary, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			// The process could not be started at all (e.g., the
			// binary was deleted between build and run). This is a
			// test infrastructure failure, not a test failure.
			t.Fatalf("run binary %s: %v", binary, err)
		}
	}

	return binaryRun{
		exitCode: exitCode,
		stdout:   stdout.String(),
		stderr:   stderr.String(),
	}
}

// =============================================================================
// Smoke test 1 — forge (no arguments)
// =============================================================================

// TestBinary_GivenNoArgs_ThenExitsZeroAndPrintsUsage is the
// process-boundary equivalent of
// TestRoot_GivenNoArgs_ThenExitsZeroAndPrintsUsage in
// internal/cli/root_test.go.
//
// Scenario:
//
//	Given a compiled forge binary
//	When  the binary is invoked with no arguments
//	Then  the process exits with code 0
//	And   stdout contains the root command description
//	And   stderr is empty
func TestBinary_GivenNoArgs_ThenExitsZeroAndPrintsUsage(t *testing.T) {
	t.Parallel()

	binary := buildBinary(t)
	got := runBinary(t, binary)

	if got.exitCode != 0 {
		t.Errorf("exit code: got %d, want 0", got.exitCode)
	}
	if !strings.Contains(got.stdout, "Forge") {
		t.Errorf("stdout does not contain the root description; got: %q",
			got.stdout)
	}
	if got.stderr != "" {
		t.Errorf("stderr should be empty; got: %q", got.stderr)
	}
}

// =============================================================================
// Smoke test 2 — forge --help
// =============================================================================

// TestBinary_GivenHelpFlag_ThenExitsZeroAndPrintsHelp is the
// process-boundary equivalent of
// TestRoot_GivenHelpFlag_ThenExitsZeroAndPrintsHelpToStdout in
// internal/cli/root_test.go.
//
// Scenario:
//
//	Given a compiled forge binary
//	When  the binary is invoked with --help
//	Then  the process exits with code 0
//	And   stdout contains the help text
//	And   stderr is empty
func TestBinary_GivenHelpFlag_ThenExitsZeroAndPrintsHelp(t *testing.T) {
	t.Parallel()

	binary := buildBinary(t)
	got := runBinary(t, binary, "--help")

	if got.exitCode != 0 {
		t.Errorf("exit code: got %d, want 0", got.exitCode)
	}
	if !strings.Contains(got.stdout, "forge") {
		t.Errorf("stdout does not contain 'forge'; got: %q", got.stdout)
	}
	if !strings.Contains(got.stdout, "Forge") {
		t.Errorf("stdout does not contain the description; got: %q",
			got.stdout)
	}
	if got.stderr != "" {
		t.Errorf("stderr should be empty; got: %q", got.stderr)
	}
}

// =============================================================================
// Smoke test 3 — forge unknown-command
// =============================================================================

// TestBinary_GivenUnknownCommand_ThenExitsUsage is the
// process-boundary equivalent of
// TestRoot_GivenUnknownCommand_ThenExitsUsageAndPrintsErrorToStderr
// in internal/cli/root_test.go.
//
// Scenario:
//
//	Given a compiled forge binary
//	When  the binary is invoked with an unknown command
//	Then  the process exits with ExitUsage (64, BSD EX_USAGE)
//	And   stderr contains the error message
//	And   stdout is empty
//
// # The exit code changed in WBS 11.1.1
//
// Before the WBS 11.1.1 taxonomy, an unknown command exited with 2
// (the flat assignment's CLI category code). The taxonomy moved the
// CLI category's code to 64 (BSD sysexits.h EX_USAGE) so that the
// flat range 1–9 could be reserved for Forge findings and so that
// usage errors could align with the sysexits convention. The
// assertion below uses the new value; the cross-layer test
// (TestBinary_GivenUnknownCommand_ExitCodeMatchesInProcessContract)
// checks that the binary and the in-process code agree.
func TestBinary_GivenUnknownCommand_ThenExitsUsage(t *testing.T) {
	t.Parallel()

	binary := buildBinary(t)
	got := runBinary(t, binary, "unknown-command")

	if got.exitCode != 64 {
		t.Errorf("exit code: got %d, want 64 (ExitUsage)", got.exitCode)
	}
	if !strings.Contains(got.stderr, "unknown-command") {
		t.Errorf("stderr does not mention the unknown command; got: %q",
			got.stderr)
	}
	if got.stdout != "" {
		t.Errorf("stdout should be empty; got: %q", got.stdout)
	}
}

// =============================================================================
// Consistency between the binary and the in-process tests
// =============================================================================

// TestBinary_GivenNoArgs_ExitCodeMatchesInProcessContract verifies
// that the binary's exit code for the no-args case matches the value
// the in-process tests assert.
//
// This is a cross-layer consistency check. The in-process tests in
// internal/cli/root_test.go assert exit code 0 for the no-args case.
// The binary must produce the same exit code when invoked from the
// operating system. If the two layers disagree, the in-process tests
// are proving a property that does not hold at the process boundary.
func TestBinary_GivenNoArgs_ExitCodeMatchesInProcessContract(t *testing.T) {
	t.Parallel()

	binary := buildBinary(t)
	got := runBinary(t, binary)

	// The in-process contract (internal/cli/root_test.go) is that
	// the no-args case exits with ExitSuccess (0).
	const inProcessExpected = 0

	if got.exitCode != inProcessExpected {
		t.Errorf("binary exit code %d does not match in-process "+
			"contract %d (ExitSuccess)", got.exitCode, inProcessExpected)
	}
}

// TestBinary_GivenUnknownCommand_ExitCodeMatchesInProcessContract
// verifies that the binary's exit code for the unknown-command case
// matches the value the in-process tests assert.
//
// This is the negative-side counterpart of the previous test.
//
// # The exit code changed in WBS 11.1.1
//
// The in-process contract (internal/cli/root_test.go) asserts
// ExitUsage for an unknown command. Under the WBS 11.1.1 taxonomy,
// ExitUsage is 64 (BSD EX_USAGE). The literal below mirrors the
// in-process constant; a future change to either requires a change
// to both in the same commit, and the test fails if they diverge.
func TestBinary_GivenUnknownCommand_ExitCodeMatchesInProcessContract(t *testing.T) {
	t.Parallel()

	binary := buildBinary(t)
	got := runBinary(t, binary, "unknown-command")

	// The in-process contract (internal/cli/root_test.go) is that
	// an unknown command exits with ExitUsage (64, BSD EX_USAGE).
	const inProcessExpected = 64

	if got.exitCode != inProcessExpected {
		t.Errorf("binary exit code %d does not match in-process "+
			"contract %d (ExitUsage)", got.exitCode, inProcessExpected)
	}
}

// =============================================================================
// Non-functional cases
// =============================================================================

// TestBinary_IsDeterministicAcrossInvocations verifies that two
// invocations with identical arguments produce byte-identical
// observable output.
//
// The binary is stateless in WBS 2.4.2. Two invocations with the same
// arguments must therefore produce the same stdout, stderr, and exit
// code. This test guards against accidentally introducing
// nondeterminism: timestamps in output, environment-dependent
// formatting, or other sources of drift.
func TestBinary_IsDeterministicAcrossInvocations(t *testing.T) {
	t.Parallel()

	binary := buildBinary(t)
	first := runBinary(t, binary, "--help")

	for i := 0; i < 10; i++ {
		got := runBinary(t, binary, "--help")

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

// TestBinary_StreamSeparation verifies that a single invocation
// writes to at most one of stdout or stderr.
//
// The stream separation contract (WBS 7.4) is: successful output goes
// to stdout; errors and diagnostics go to stderr. A single invocation
// must not write to both. This is the boundary condition for shell
// pipelines that redirect one stream and consume the other.
//
// The test uses two representative invocations: one that succeeds and
// one that fails. For the success case, stderr must be empty. For the
// failure case, stdout must be empty.
func TestBinary_StreamSeparation(t *testing.T) {
	t.Parallel()

	binary := buildBinary(t)

	t.Run("success writes only to stdout", func(t *testing.T) {
		t.Parallel()
		got := runBinary(t, binary, "--help")
		if got.stdout == "" {
			t.Error("stdout is empty on success")
		}
		if got.stderr != "" {
			t.Errorf("stderr should be empty on success; got: %q",
				got.stderr)
		}
	})

	t.Run("failure writes only to stderr", func(t *testing.T) {
		t.Parallel()
		got := runBinary(t, binary, "unknown-command")
		if got.stdout != "" {
			t.Errorf("stdout should be empty on failure; got: %q",
				got.stdout)
		}
		if got.stderr == "" {
			t.Error("stderr is empty on failure")
		}
	})
}

// =============================================================================
// Edge and corner cases
// =============================================================================

// TestBinary_GivenEmptyStringArgument verifies that an empty-string
// argument is treated as an unknown command, not as the absence of
// arguments.
//
// This is a corner case at the boundary between the OS layer and the
// CLI layer. The OS passes an empty string as a positional argument;
// the CLI must treat it as an invalid command name, not as "no
// command".
func TestBinary_GivenEmptyStringArgument(t *testing.T) {
	t.Parallel()

	binary := buildBinary(t)
	got := runBinary(t, binary, "")

	if got.exitCode == 0 {
		t.Errorf("expected non-zero exit code for empty-string argument; "+
			"got: %d", got.exitCode)
	}
}

// TestBinary_GivenUnknownFlag verifies that an unknown flag is
// treated as a usage error at the process boundary.
//
// This complements the unknown-command case: both are usage errors,
// but they arrive at Cobra through different channels.
func TestBinary_GivenUnknownFlag(t *testing.T) {
	t.Parallel()

	binary := buildBinary(t)
	got := runBinary(t, binary, "--nonexistent-flag")

	if got.exitCode == 0 {
		t.Errorf("expected non-zero exit code for unknown flag; got: %d",
			got.exitCode)
	}
	if got.stderr == "" {
		t.Error("expected an error message on stderr")
	}
	if got.stdout != "" {
		t.Errorf("stdout should be empty on failure; got: %q", got.stdout)
	}
}

// TestBinary_GivenMultipleUnknownArguments verifies that the CLI
// handles more than one unrecognised argument without crashing or
// truncating the error message.
//
// This is a boundary case: the CLI must produce a single error that
// mentions at least one of the unknown arguments, not a panic, not a
// stack trace, not silent success.
func TestBinary_GivenMultipleUnknownArguments(t *testing.T) {
	t.Parallel()

	binary := buildBinary(t)
	got := runBinary(t, binary, "unknown-one", "unknown-two")

	if got.exitCode == 0 {
		t.Errorf("expected non-zero exit code; got: %d", got.exitCode)
	}
	if got.stdout != "" {
		t.Errorf("stdout should be empty; got: %q", got.stdout)
	}
	// The error message must mention at least one of the unknown
	// tokens. Which one is an implementation detail of Cobra; we do
	// not over-specify.
	if !strings.Contains(got.stderr, "unknown-one") &&
		!strings.Contains(got.stderr, "unknown-two") {
		t.Errorf("stderr does not mention either unknown token; got: %q",
			got.stderr)
	}
}

// =============================================================================
// Documentation and help consistency
// =============================================================================

// TestBinary_HelpContainsRootDescription verifies that the help
// output contains the root command's Long description, verbatim.
//
// This is a content check on the binary's help output. It ensures
// that the description is not lost in the process boundary — for
// example, by being written to a stream that is discarded.
func TestBinary_HelpContainsRootDescription(t *testing.T) {
	t.Parallel()

	binary := buildBinary(t)
	got := runBinary(t, binary, "--help")

	// The Long description in internal/cli/root.go begins with
	// "Forge is a cross-platform CLI". Checking for this substring
	// proves the description reached stdout.
	const expectedSubstring = "Forge is a cross-platform CLI"

	if !strings.Contains(got.stdout, expectedSubstring) {
		t.Errorf("help output does not contain the root description; "+
			"expected substring: %q", expectedSubstring)
	}
}

// =============================================================================
// Exit boundary — signal handling (WBS 11.3.1, ADR-020)
// =============================================================================

// TestBinary_GivenSIGINT_ThenExitsSIGINT is the process-boundary
// test for the WBS 11.3.1 signal-handling contract: when the binary
// receives SIGINT (Ctrl-C), it exits with ExitSIGINT (130, the
// 128 + 2 convention).
//
// # How the test works
//
// The test spawns the binary with a long-running argument that
// blocks (for example, a hypothetical future long-running command,
// or — in the current Phase 2 tree — the help output which is too
// fast to catch). Since Phase 2 has no long-running command, the
// test cannot reliably deliver a signal before the process exits.
//
// The test is therefore skipped unless a future WBS adds a command
// that blocks. The skip is documented here so that a reader sees
// the test is intentional, not forgotten. When a blocking command
// exists, this test's body is the reference implementation for
// delivering SIGINT and asserting the exit code.
//
// # Cross-platform note
//
// The test uses syscall.SIGINT, which is available on Unix. On
// Windows, the signal model differs; the test skips on Windows.
func TestBinary_GivenSIGINT_ThenExitsSIGINT(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("signal delivery semantics differ on Windows")
	}

	// Phase 2 has no long-running command, so a SIGINT cannot be
	// delivered before the process exits. The test skips.
	//
	// A future WBS that adds a blocking command will remove the
	// skip and use the following pattern:
	//
	//	cmd := exec.Command(binary, "some-blocking-command")
	//	var stdout, stderr bytes.Buffer
	//	cmd.Stdout = &stdout
	//	cmd.Stderr = &stderr
	//	if err := cmd.Start(); err != nil {
	//	    t.Fatalf("start: %v", err)
	//	}
	//	// Give the process time to install its signal handler.
	//	time.Sleep(100 * time.Millisecond)
	//	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
	//	    t.Fatalf("signal: %v", err)
	//	}
	//	err := cmd.Wait()
	//	var exitErr *exec.ExitError
	//	if !errors.As(err, &exitErr) {
	//	    t.Fatalf("expected exit error, got: %v", err)
	//	}
	//	if got := exitErr.ExitCode(); got != 130 {
	//	    t.Errorf("exit code: got %d, want 130 (ExitSIGINT)", got)
	//	}
	t.Skip("Phase 2 has no long-running command to signal")
}

// =============================================================================
// Short mode
// =============================================================================

// TestIntegration_ShortMode is a placeholder that documents the
// interaction between the `integration` build tag and the `-short`
// flag.
//
// This file is gated behind the `integration` build tag. It only runs
// when the tag is set. In addition, when `go test -short` is used,
// tests in this file that perform expensive operations may be
// skipped. In WBS 2.4.2, the only expensive operation is the binary
// build, which is cached. No test in this file currently checks
// testing.Short(). This test exists to remind future maintainers of
// the convention.
//
// If a future integration test becomes materially slower than the
// others (for example, if it spawns a long-running process), it
// should begin with:
//
//	if testing.Short() {
//		t.Skip("skipping slow integration test in short mode")
//	}
//
// This convention keeps `go test -short -tags=integration` usable as
// a fast smoke check even when the full integration suite is slow.
func TestIntegration_ShortMode(t *testing.T) {
	t.Parallel()

	// This test performs no assertions. Its presence is
	// documentation. It is skipped automatically if the
	// `integration` build tag is not set, because it lives in a
	// file that requires the tag.
	//
	// The test is intentionally trivial so that it never fails for
	// any reason other than a change to the convention it documents.
	_ = testing.Short()
}

// =============================================================================
// Quiet mode (WBS 12.6.1)
// =============================================================================

// TestQuiet_PreservesExitCode asserts that every documented exit
// code is identical with and without --quiet.
//
// The test runs each scenario twice (with and without --quiet)
// and asserts the exit codes match. The scenarios cover every
// exit-code category the CLI can produce: success, usage, config.
// The list is exhaustive over the codes that a reasonable
// invocation can reach; a future code added to the taxonomy
// should be added here.
//
// # Why the binary path is fetched inside the loop
//
// buildBinary returns a cached path: the first call builds the
// binary, subsequent calls return the same path. Fetching it
// inside the loop keeps the test's dependency on the binary
// explicit and avoids a package-level variable that would be
// visible to every test in this file. The cost is a single
// sync.Once call per iteration; the benefit is that the test's
// data flow is visible from its body alone.
func TestQuiet_PreservesExitCode(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
	}{
		{"success", []string{"version"}},
		{"usage error", []string{"unknown-command"}},
		{"config error", []string{"--config", "/nonexistent-for-testing/x.yaml", "version"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			binary := buildBinary(t)

			plain := runBinary(t, binary, tc.args...)
			quiet := runBinary(t, binary,
				append([]string{"--quiet"}, tc.args...)...)

			if plain.exitCode != quiet.exitCode {
				t.Fatalf("exit codes differ: plain=%d quiet=%d",
					plain.exitCode, quiet.exitCode)
			}
		})
	}
}

// TestQuiet_FormatJSON_ProducesValidJSON asserts that
// --quiet --format json produces a valid JSON document.
//
// The JSON error envelope (ADR-021) is the command's result in
// JSON mode; --quiet must not suppress it. The test asserts the
// document is present, decodes as a single JSON value, and has
// no trailing data.
func TestQuiet_FormatJSON_ProducesValidJSON(t *testing.T) {
	t.Parallel()

	binary := buildBinary(t)
	got := runBinary(t, binary,
		"--quiet", "--format", "json", "version")

	if got.exitCode != 0 {
		t.Fatalf("exit code: got %d, want 0\nstderr: %q",
			got.exitCode, got.stderr)
	}
	dec := json.NewDecoder(strings.NewReader(got.stdout))
	var v any
	if err := dec.Decode(&v); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %q",
			err, got.stdout)
	}
	if dec.More() {
		t.Fatalf("stdout contains trailing data: %q", got.stdout)
	}
}

// TestQuiet_ErrorStillOnStderr asserts that an error's diagnostic
// reaches stderr under --quiet.
func TestQuiet_ErrorStillOnStderr(t *testing.T) {
	t.Parallel()

	binary := buildBinary(t)
	got := runBinary(t, binary, "--quiet", "unknown-command")

	if got.exitCode == 0 {
		t.Fatalf("exit code: got 0, want non-success")
	}
	if got.stderr == "" {
		t.Fatal("stderr is empty; --quiet must not suppress the error block")
	}
}

// TestQuiet_VerboseConflict asserts that --quiet --verbose is a
// usage error with a clear message.
func TestQuiet_VerboseConflict(t *testing.T) {
	t.Parallel()

	binary := buildBinary(t)
	got := runBinary(t, binary,
		"--quiet", "--verbose", "version")

	if got.exitCode != 64 {
		t.Fatalf("exit code: got %d, want 64 (usage error)\nstderr: %q",
			got.exitCode, got.stderr)
	}
	if !strings.Contains(got.stderr, "mutually exclusive") {
		t.Fatalf("stderr does not mention mutual exclusion: %q",
			got.stderr)
	}
	if got.stdout != "" {
		t.Fatalf("stdout is not empty on usage error: %q", got.stdout)
	}
}

// TestQuiet_ShortFormMatchesLongForm asserts that -q and --quiet
// are equivalent at the CLI boundary.
func TestQuiet_ShortFormMatchesLongForm(t *testing.T) {
	t.Parallel()

	binary := buildBinary(t)
	long := runBinary(t, binary, "--quiet", "version")
	short := runBinary(t, binary, "-q", "version")

	if long.exitCode != short.exitCode {
		t.Fatalf("exit codes differ: --quiet=%d -q=%d",
			long.exitCode, short.exitCode)
	}
	if long.stdout != short.stdout {
		t.Fatalf("stdout differs:\n--quiet: %q\n-q:      %q",
			long.stdout, short.stdout)
	}
	if long.stderr != short.stderr {
		t.Fatalf("stderr differs:\n--quiet: %q\n-q:      %q",
			long.stderr, short.stderr)
	}
}
