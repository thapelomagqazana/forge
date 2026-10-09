//go:build integration

// Package main_test contains integration tests that exercise the
// compiled forge binary as a subprocess. These tests are gated behind
// the "integration" build tag because they require:
//
//   1. A successful compilation of the binary.
//   2. Spawning a subprocess per test case.
//
// Run them explicitly with:
//
//     go test -tags=integration ./cmd/forge/...
//
// The tests use BDD-style naming (Given/When/Then) to make the
// behavioural contract explicit. Each test exercises a scenario from
// the process boundary contract.
package main_test

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildBinary compiles cmd/forge into a temporary path and returns it.
// The binary is cached across tests via sync.Once in a real project; for
// clarity this example builds it once per test binary invocation.
func buildBinary(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	binary := filepath.Join(dir, "forge")

	cmd := exec.Command("go", "build", "-o", binary, ".")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build forge: %v\n%s", err, out)
	}
	return binary
}

// runBinary executes the binary with the given args and returns
// stdout, stderr, and the process exit code.
func runBinary(t *testing.T, binary string, args ...string) (stdout, stderr string, code int) {
	t.Helper()

	var outBuf, errBuf bytes.Buffer
	cmd := exec.Command(binary, args...)
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	code = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("run binary: %v", err)
		}
	}
	return outBuf.String(), errBuf.String(), code
}

// Scenario: Process boundary contract — success path.
//
//   Given a compiled forge binary
//   When  the binary is invoked with --help
//   Then  the process exits with code 0
//   And   stdout contains the root help
//   And   stderr is empty
func TestBinary_GivenHelp_ThenExitsZeroAndPrintsToStdout(t *testing.T) {
	binary := buildBinary(t)

	stdout, stderr, code := runBinary(t, binary, "--help")

	if code != 0 {
		t.Errorf("exit code: got %d, want 0", code)
	}
	if !strings.Contains(stdout, "forge") {
		t.Errorf("stdout does not contain 'forge': %q", stdout)
	}
	if stderr != "" {
		t.Errorf("stderr should be empty: %q", stderr)
	}
}

// Scenario: Process boundary contract — failure path.
//
//   Given a compiled forge binary
//   When  the binary is invoked with an unknown command
//   Then  the process exits with a non-zero code
//   And   stderr contains the error
//   And   stdout is empty
func TestBinary_GivenUnknownCommand_ThenExitsNonZeroAndPrintsToStderr(t *testing.T) {
	binary := buildBinary(t)

	stdout, stderr, code := runBinary(t, binary, "unknown-command")

	if code == 0 {
		t.Errorf("exit code: got 0, want non-zero")
	}
	if stdout != "" {
		t.Errorf("stdout should be empty on error: %q", stdout)
	}
	if stderr == "" {
		t.Errorf("stderr should contain the error message")
	}
}

// Scenario: Process boundary contract — version path.
//
//   Given a compiled forge binary
//   When  the binary is invoked with --version
//   Then  the process exits with code 0
//   And   stdout contains a version string
func TestBinary_GivenVersionFlag_ThenExitsZeroAndPrintsVersion(t *testing.T) {
	binary := buildBinary(t)

	stdout, stderr, code := runBinary(t, binary, "--version")

	if code != 0 {
		t.Errorf("exit code: got %d, want 0", code)
	}
	if !strings.Contains(stdout, "forge") {
		t.Errorf("stdout does not contain 'forge': %q", stdout)
	}
	if stderr != "" {
		t.Errorf("stderr should be empty: %q", stderr)
	}
}

// Scenario: Process boundary contract — no arguments.
//
//   Given a compiled forge binary
//   When  the binary is invoked with no arguments
//   Then  the process exits with code 0
//   And   stdout contains help text
func TestBinary_GivenNoArgs_ThenExitsZeroAndPrintsHelp(t *testing.T) {
	binary := buildBinary(t)

	stdout, stderr, code := runBinary(t, binary)

	if code != 0 {
		t.Errorf("exit code: got %d, want 0", code)
	}
	if !strings.Contains(stdout, "forge") {
		t.Errorf("stdout does not contain 'forge': %q", stdout)
	}
	if stderr != "" {
		t.Errorf("stderr should be empty: %q", stderr)
	}
}