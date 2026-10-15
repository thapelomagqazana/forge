// Package scripts_test contains the acceptance tests for the scripts
// in the scripts/ directory.
//
// The tests are in the scripts package's test file rather than in the
// scripts themselves because the scripts are shell and PowerShell;
// the Go tests invoke them as subprocesses and assert their output.
//
// # The fixture
//
// The tests build a fixture release directory in a temp directory:
// one archive, its SHA-256, a SHA256SUMS file, a SHA256SUMS.asc, the
// archive's cosign signature and certificate, and the SLSA
// provenance. The fixture's signatures are produced with a
// temporary GPG key and a temporary cosign key so the tests do not
// need the release key.
//
// The fixture is deliberately small: one archive, one platform, one
// version. A test that needed a second platform's archive would
// extend the fixture; the current shape is the minimum the script
// reads.
//
// # What the tests assert
//
//   - The script exits 0 against a valid fixture.
//   - The script exits 1 against a tampered archive.
//   - The script exits 1 against a tampered SHA256SUMS.
//   - The script exits 2 with no arguments.
//   - The script skips the GPG and cosign checks when the
//     fingerprints are not set, and reports the skips.
//   - The script's stdout is machine-readable: one `key=value` per
//     line for the summary.
//
// The tests are the script's contract. A change to the script that
// breaks one of the assertions fails the test.
package main_test

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// scriptPath returns the path to the script for the host OS.
//
// The function is the one place the tests name the two scripts. A
// future third script (a different shell) would add a case.
func scriptPath(t *testing.T, root string) string {
	t.Helper()
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(root, "scripts", "verify-release.ps1")
	default:
		return filepath.Join(root, "scripts", "verify-release.sh")
	}
}

// runScript invokes the script with args and returns stdout, stderr,
// and the exit code.
//
// The function is the tests' one place that knows how to run a
// script. A shell script is run with `bash`; a PowerShell script is
// run with `pwsh`. The two commands are the scripts' interpreters.
func runScript(t *testing.T, root string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("pwsh", append([]string{
			"-NoProfile",
			"-File", scriptPath(t, root),
		}, args...)...)
	} else {
		cmd = exec.Command("bash", append([]string{
			scriptPath(t, root),
		}, args...)...)
	}
	cmd.Dir = root

	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return outBuf.String(), errBuf.String(), exitErr.ExitCode()
		}
		t.Fatalf("run script: %v", err)
	}
	return outBuf.String(), errBuf.String(), 0
}

// TestVerifyReleaseScript_NoArgs_Exits2 asserts the script's usage
// exit code.
func TestVerifyReleaseScript_NoArgs_Exits2(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping script acceptance test in short mode")
	}
	t.Parallel()
	root := repoRoot(t)
	_, stderr, code := runScript(t, root)
	if code != 2 {
		t.Errorf("exit code = %d; want 2\nstderr:\n%s", code, stderr)
	}
	if !strings.Contains(stderr, "Usage") {
		t.Errorf("stderr does not contain usage:\n%s", stderr)
	}
}

// TestVerifyReleaseScript_MissingTools_Exits1 asserts the script
// reports a missing tool before running any check.
//
// The test runs the script with a PATH that does not include the
// tools. On Unix, the script's `command -v` finds nothing; the
// script exits 1. On Windows, the same check runs through
// `Get-Command`.
//
// The test is Unix-only for the moment: the Windows path requires
// a modified PATH, which the Go test can set on the subprocess's
// environment but not on the host's. A future change that made
// the Windows test set the subprocess's PATH would remove the
// skip.
func TestVerifyReleaseScript_MissingTools_Exits1(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the Windows PATH modification is not implemented")
	}
	if testing.Short() {
		t.Skip("skipping script acceptance test in short mode")
	}
	t.Parallel()
	root := repoRoot(t)

	cmd := exec.Command("bash", scriptPath(t, root), "v1.4.2")
	cmd.Dir = root
	cmd.Env = []string{"PATH=/nonexistent", "HOME=" + t.TempDir()}

	var stderr strings.Builder
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		t.Fatalf("script exited 0; want a non-zero exit")
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("run: %v", err)
	}
	if exitErr.ExitCode() != 1 {
		t.Errorf("exit code = %d; want 1", exitErr.ExitCode())
	}
	if !strings.Contains(stderr.String(), "missing tool") {
		t.Errorf("stderr does not name a missing tool:\n%s", stderr.String())
	}
}

// TestVerifyReleaseScript_ValidFixture_Exits0 asserts the script
// passes against a fixture the test builds.
//
// The test builds the fixture in a temp directory; the script reads
// the fixture's assets from the directory it is given. The script's
// downloads are skipped because the assets exist; the script's
// checks run against them.
func TestVerifyReleaseScript_ValidFixture_Exits0(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping script acceptance test in short mode")
	}
	if runtime.GOOS == "windows" {
		t.Skip("the fixture's GPG and cosign setup is Unix-only for now")
	}
	t.Parallel()
	root := repoRoot(t)
	dir := t.TempDir()

	buildFixture(t, dir)

	// The script reads the fingerprint from the environment.
	// The fixture's GPG key is the fingerprint the test sets.
	stdout, stderr, code := runScriptWithEnv(t, root, []string{
		"FORGE_GPG_KEY_FINGERPRINT=" + fixtureFingerprint(t, dir),
	}, "v1.4.2", "forge-linux-amd64.tar.gz", dir)
	if code != 0 {
		t.Fatalf("exit code = %d; want 0\nstderr:\n%s\nstdout:\n%s", code, stderr, stdout)
	}
	for _, want := range []string{
		"version=v1.4.2",
		"archive=forge-linux-amd64.tar.gz",
		"checks=checksum,gpg,cosign,slsa",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout does not contain %q:\n%s", want, stdout)
		}
	}
}

// TestVerifyReleaseScript_TamperedArchive_Exits1 asserts the script
// fails when the archive's bytes do not match the checksum.
func TestVerifyReleaseScript_TamperedArchive_Exits1(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping script acceptance test in short mode")
	}
	if runtime.GOOS == "windows" {
		t.Skip("the fixture's GPG and cosign setup is Unix-only for now")
	}
	t.Parallel()
	root := repoRoot(t)
	dir := t.TempDir()

	buildFixture(t, dir)
	tamperFile(t, filepath.Join(dir, "forge-linux-amd64.tar.gz"))

	stdout, stderr, code := runScriptWithEnv(t, root, []string{
		"FORGE_GPG_KEY_FINGERPRINT=" + fixtureFingerprint(t, dir),
	}, "v1.4.2", "forge-linux-amd64.tar.gz", dir)
	if code != 1 {
		t.Fatalf("exit code = %d; want 1\nstderr:\n%s\nstdout:\n%s", code, stderr, stdout)
	}
	if !strings.Contains(stderr, "FAIL: checksum mismatch") {
		t.Errorf("stderr does not name the failure:\n%s", stderr)
	}
}
