package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRun_GivenHomebrew_ThenWritesFormula asserts the driver writes
// the formula to the output directory.
func TestRun_GivenHomebrew_ThenWritesFormula(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	dist := writeFixtureDist(t, dir)
	out := filepath.Join(dir, "out")

	var stdout, stderr strings.Builder
	code := run([]string{
		"--channel=homebrew",
		"--version=v1.4.2",
		"--release-channel=stable",
		"--dist=" + dist,
		"--out=" + out,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d; stderr = %s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(out, "Formula", "forge.rb")); err != nil {
		t.Errorf("formula not written: %v", err)
	}
}

// TestRun_GivenNightlyToHomebrew_ThenExits3 asserts a nightly to a
// channel that does not accept a nightly exits 3.
func TestRun_GivenNightlyToHomebrew_ThenExits3(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	dist := writeFixtureDist(t, dir)
	var stdout, stderr strings.Builder
	code := run([]string{
		"--channel=homebrew",
		"--version=v1.4.2",
		"--release-channel=nightly",
		"--dist=" + dist,
		"--out=" + filepath.Join(dir, "out"),
	}, &stdout, &stderr)
	if code != 3 {
		t.Errorf("code = %d; want 3", code)
	}
}

// TestRun_GivenUnknownChannel_ThenExits2 asserts an unknown channel
// is a usage error.
func TestRun_GivenUnknownChannel_ThenExits2(t *testing.T) {
	t.Parallel()
	var stdout, stderr strings.Builder
	code := run([]string{"--channel=helm", "--version=v1.4.2"}, &stdout, &stderr)
	if code != 2 {
		t.Errorf("code = %d; want 2", code)
	}
}
