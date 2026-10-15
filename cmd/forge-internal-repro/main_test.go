package main

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/thapelomagqazana/forge/internal/platform"
)

// loadPlatform loads the canonical platform matrix. It is a thin
// wrapper used by the ATDD test; the production code path calls
// platform.LoadDefault directly.
func loadPlatform(_ string) (*platform.Matrix, error) {
	return platform.LoadDefault()
}

// --- Unit: parseCSV ---------------------------------------------------------

// TestParseCSV asserts the comma-separated parser trims spaces and
// ignores empty entries.
func TestParseCSV(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"linux", []string{"linux"}},
		{"linux,darwin", []string{"darwin", "linux"}},
		{" linux , darwin ", []string{"darwin", "linux"}},
		{"linux,,darwin", []string{"darwin", "linux"}},
	}
	for _, tc := range cases {
		got := parseCSV(tc.in)
		if len(got) != len(tc.want) {
			t.Errorf("parseCSV(%q) len = %d, want %d", tc.in, len(got), len(tc.want))
			continue
		}
		for _, k := range tc.want {
			if _, ok := got[k]; !ok {
				t.Errorf("parseCSV(%q) missing %q", tc.in, k)
			}
		}
	}
}

// --- Unit: archiveName ------------------------------------------------------

// TestArchiveName asserts the naming convention matches the driver's.
// If the two diverge, the verifier will not find the archive it built
// and will report a misleading failure.
func TestArchiveName(t *testing.T) {
	t.Parallel()
	cases := map[[2]string]string{
		{"linux", "amd64"}:   "forge-linux-amd64.tar.gz",
		{"linux", "arm64"}:   "forge-linux-arm64.tar.gz",
		{"darwin", "amd64"}:  "forge-darwin-amd64.tar.gz",
		{"darwin", "arm64"}:  "forge-darwin-arm64.tar.gz",
		{"windows", "amd64"}: "forge-windows-amd64.zip",
		{"windows", "arm64"}: "forge-windows-arm64.zip",
	}
	for in, want := range cases {
		if got := archiveNameFor(in[0], in[1]); got != want {
			t.Errorf("archiveName(%q, %q) = %q, want %q", in[0], in[1], got, want)
		}
	}
}

// --- Unit: sha256Of ---------------------------------------------------------

// TestSHA256Of asserts the helper computes the standard SHA-256 of a
// file's bytes.
func TestSHA256Of(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "x")
	if err := os.WriteFile(path, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := sha256Of(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03"
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

// --- Unit: resolveSourceDateEpoch -------------------------------------------

// TestResolveSourceDateEpoch_EnvWins asserts SOURCE_DATE_EPOCH takes
// priority over git.
//
// Not parallel: this test mutates the process environment.
func TestResolveSourceDateEpoch_EnvWins(t *testing.T) {
	old, had := os.LookupEnv("SOURCE_DATE_EPOCH")
	defer func() {
		if had {
			_ = os.Setenv("SOURCE_DATE_EPOCH", old)
		} else {
			_ = os.Unsetenv("SOURCE_DATE_EPOCH")
		}
	}()
	const want = int64(1748779200) // 2025-06-01T12:00:00Z
	_ = os.Setenv("SOURCE_DATE_EPOCH", strconv.FormatInt(want, 10))

	got, err := resolveSourceDateEpoch(".")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("got %d, want %d", got, want)
	}
}

// TestResolveSourceDateEpoch_RejectsNonInteger asserts a non-numeric
// SOURCE_DATE_EPOCH is rejected rather than silently truncated.
//
// Not parallel: this test mutates the process environment.
func TestResolveSourceDateEpoch_RejectsNonInteger(t *testing.T) {
	old, had := os.LookupEnv("SOURCE_DATE_EPOCH")
	defer func() {
		if had {
			_ = os.Setenv("SOURCE_DATE_EPOCH", old)
		} else {
			_ = os.Unsetenv("SOURCE_DATE_EPOCH")
		}
	}()
	_ = os.Setenv("SOURCE_DATE_EPOCH", "1700000000garbage")

	if _, err := resolveSourceDateEpoch("."); err == nil {
		t.Fatal("expected error for non-integer SOURCE_DATE_EPOCH")
	}
}

// --- ATDD ------------------------------------------------------------------

// TestVerifyOne_TwoBuildsMatch is the acceptance test for the
// verifier's core claim: two builds of the same commit produce
// byte-identical archives. It builds linux/amd64 twice and compares
// SHA-256.
//
// Skipped in short mode. In CI, the reproducible workflow runs the
// verifier for all six platforms; this test exercises the same code
// path for one platform, fast enough for a normal `go test` run.
func TestVerifyOne_TwoBuildsMatch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	root, err := findModuleRoot()
	if err != nil {
		t.Skipf("no module root: %v", err)
	}

	m, err := loadPlatform(root)
	if err != nil {
		t.Fatal(err)
	}
	p, ok := m.ByGOOSGOARCH("linux", "amd64")
	if !ok {
		t.Fatal("linux/amd64 not in platform matrix")
	}

	sde := time.Now().Unix() // any fixed value works; both builds see it
	opts := options{root: root}
	if err := verifyOne(opts, *p, sde, os.Stdout, os.Stderr); err != nil {
		t.Fatalf("verifyOne: %v", err)
	}
}
