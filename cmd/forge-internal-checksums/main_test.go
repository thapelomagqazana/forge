package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thapelomagqazana/forge/internal/platform"
)

// TestWritePerBinaryChecksum asserts the per-binary .sha256 file
// contains the digest and the archive's basename in `sha256sum`
// format.
//
// The file's content is "<hex digest>  <archive name>\n". The two
// spaces between the digest and the name are the canonical form;
// the test asserts the format, not the digest's value, because the
// digest depends on the archive's bytes and the test writes those
// bytes itself.
func TestWritePerBinaryChecksum(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	archive := filepath.Join(dir, "forge-linux-amd64.tar.gz")
	if err := os.WriteFile(archive, []byte("binary"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := platformForTest("linux", "amd64")
	if err := writePerBinaryChecksum(dir, p); err != nil {
		t.Fatalf("writePerBinaryChecksum: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "forge-linux-amd64.tar.gz.sha256"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, "  forge-linux-amd64.tar.gz\n") {
		t.Errorf("content does not have the sha256sum format: %q", got)
	}
}

// TestWriteCombinedChecksums_Sorted asserts the combined list is
// sorted by archive name. The order is deterministic so two runs of
// the driver produce byte-identical output.
func TestWriteCombinedChecksums_Sorted(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, name := range []string{
		"forge-linux-amd64.tar.gz",
		"forge-darwin-arm64.tar.gz",
		"forge-windows-amd64.zip",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	platforms := []platform.Platform{
		platformForTest("linux", "amd64"),
		platformForTest("darwin", "arm64"),
		platformForTest("windows", "amd64"),
	}
	if err := writeCombinedChecksums(dir, platforms); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "SHA256SUMS"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(lines))
	}
	// Sorted ascending.
	if !strings.Contains(lines[0], "forge-darwin-arm64.tar.gz") {
		t.Errorf("first line is not darwin: %q", lines[0])
	}
	if !strings.Contains(lines[1], "forge-linux-amd64.tar.gz") {
		t.Errorf("second line is not linux: %q", lines[1])
	}
	if !strings.Contains(lines[2], "forge-windows-amd64.zip") {
		t.Errorf("third line is not windows: %q", lines[2])
	}
}

// TestSignWithGPG_NoGPG asserts the blocker is returned when gpg is
// not on PATH.
//
// The test does not attempt to remove gpg from the environment; it
// skips when gpg is present. A future change that made the blocker
// depend on something else would need a different test.
//
// The context is context.Background(): the driver passes it to
// exec.CommandContext as the parent, and the test's command runs to
// its natural completion before the test returns. The test does not
// start a goroutine that would observe the test's cancellation, so
// the t.Context() convenience (Go 1.24+) is not needed.
func TestSignWithGPG_NoGPG(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("gpg"); err == nil {
		t.Skip("gpg is on PATH; the no-gpg path cannot be tested")
	}
	var out, errOut bytes.Buffer
	blocker := signWithGPG(context.Background(), t.TempDir(), &out, &errOut)
	if !strings.Contains(blocker, "gpg") {
		t.Errorf("blocker = %q, want it to name gpg", blocker)
	}
}

// TestSignWithGPG_NoFingerprint asserts the blocker is returned when
// the fingerprint is not set.
//
// The test unsets the environment variable and restores it on
// return. The restore is what makes the test safe to run in
// parallel with other tests that read the variable; a test that
// left the variable unset would affect a sibling.
//
// The context is context.Background(), for the same reason as
// TestSignWithGPG_NoGPG.
func TestSignWithGPG_NoFingerprint(t *testing.T) {
	t.Parallel()
	old, had := os.LookupEnv("FORGE_GPG_KEY_FINGERPRINT")
	defer func() {
		if had {
			os.Setenv("FORGE_GPG_KEY_FINGERPRINT", old)
		} else {
			os.Unsetenv("FORGE_GPG_KEY_FINGERPRINT")
		}
	}()
	os.Unsetenv("FORGE_GPG_KEY_FINGERPRINT")
	var out, errOut bytes.Buffer
	blocker := signWithGPG(context.Background(), t.TempDir(), &out, &errOut)
	if !strings.Contains(blocker, "FORGE_GPG_KEY_FINGERPRINT") {
		t.Errorf("blocker = %q, want it to name the env var", blocker)
	}
}
