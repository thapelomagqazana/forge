// Package main contains the integration tests for the forge
// binary. This file holds the helpers shared by the platform-
// specific broken-pipe tests.
//
// # Why a shared file
//
// The broken-pipe tests are split across two files:
//
//   - brokenpipe_unix_test.go   (build tag: unix)
//   - brokenpipe_windows_test.go (build tag: windows)
//
// Each file exercises a platform-specific shape of the same
// contract: when the read end of stdout closes, forge exits 141
// without a stack trace. The two files' bodies differ — the Unix
// file reads one line before closing; the Windows file reads a
// 1 KiB chunk — but the setup and the assertion helpers are the
// same on both platforms.
//
// The helpers are defined once in this file, which carries no
// build tag, so both platform files can call them. A copy in each
// platform file would compile, but the two copies would drift as
// the helpers changed; a single definition is the only way to
// keep the behavior identical.
//
// # The file name's _test.go suffix
//
// The file is a test file (the suffix is required) and a helper
// file (the "testhelpers" fragment makes the intent visible). The
// package is `main`, matching the package of the tests it serves.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// buildForge builds the forge binary once per test run and returns
// its absolute path.
//
// # What the helper does
//
// The helper runs `go build -o <tempdir>/forge .` in the package
// directory. The resulting binary is the same one the Taskfile's
// build task produces, except that it is placed under a per-test
// temporary directory so that parallel tests do not race on the
// output file.
//
// # Why the helper is in this file and not in each platform test
//
// The two platform test files call the helper; a copy in each
// would compile, but the two copies would drift. A single
// definition in a file with no build tag means both platform
// tests see the same behavior.
//
// # Why the binary is rebuilt per test
//
// The helper is called once per test. The build is cached by the
// Go toolchain, so a second call within the same `go test` run is
// fast; a `go test` run that has not built the package incurs one
// build, which is a few hundred milliseconds. The alternative —
// building the binary once in TestMain — would require moving
// TestMain into a shared file and threading the binary path
// through a package-level variable. The per-test build is simpler
// and fast enough.
func buildForge(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	bin := filepath.Join(dir, "forge")

	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("build forge: %v", err)
	}
	return bin
}

// processExitCode extracts the process's exit code from the error
// returned by cmd.Wait.
//
// # The three cases
//
//   - err == nil: the process exited successfully. The helper
//     returns 0.
//   - err is *exec.ExitError: the process exited non-zero. The
//     helper returns the code via the error's ExitCode method.
//   - err is some other error: the process did not start or did
//     not exit for a reason the helper cannot map. The helper
//     returns -1, which no valid process exit code equals, so a
//     test that asserts on the code sees the failure.
//
// # Why the helper is in this file and not in each platform test
//
// Same reason as buildForge. The two platform tests call it; a
// single definition keeps the behavior identical.
func processExitCode(err error) int {
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return -1
}
