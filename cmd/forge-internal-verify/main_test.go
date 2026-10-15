// Package main contains white-box tests for the binary verification
// driver.
//
// The tests are declared in package main, not a separate _test
// package, because they exercise the driver's unexported helpers
// (checkSize, checkStartup, extractTarGz, extractZip, firstLine,
// archiveNameFor) and its unexported constants (maxBinarySize,
// maxStartupDuration).
//
// # What the tests cover
//
//   - Unit tests for the helpers the driver uses to inspect the
//     artifacts and to compute the pass/fail verdict.
//   - Boundary tests for the two threshold-based checks.
//   - A shape test for the archive-naming convention that the
//     driver shares with scripts/build.go.
//
// # What the tests do not cover
//
// The end-to-end acceptance tests for the driver live in
// test/verify/binary_smoke_test.go. They build the artifacts and run
// the driver against the resulting dist/ directory. The tests in
// this file exercise the driver's internals; the acceptance tests
// exercise its observable contract.
//
// # Why the tests are not parallel
//
// Two of the tests (TestCheckSize_OverBudget,
// TestCheckSize_WithinBudget) create temporary files whose size is
// near the size budget. The files are small sparse allocations; they
// are not a source of contention. The tests are marked parallel
// because they do not share state; a future test that mutated a
// package-level variable would need to opt out.
package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/thapelomagqazana/forge/internal/platform"
)

// =============================================================================
// platformForTest
// =============================================================================

// platformForTest constructs a platform.Platform value from a
// GOOS/GOARCH pair. The verifier reads the platform from
// internal/platform; this helper lets a unit test construct the same
// value without loading the full matrix.
func platformForTest(goos, goarch string) platform.Platform {
	return platform.Platform{
		Name:   goos + "-" + goarch,
		GOOS:   goos,
		GOARCH: goarch,
		Tier:   platform.Tier1,
	}
}

// =============================================================================
// archiveNameFor
// =============================================================================

// TestArchiveNameFor asserts the archive-naming convention the
// verifier shares with scripts/build.go.
//
// The convention is frozen by docs/guides/reproducible-builds.md §
// "Artifact naming": forge-<goos>-<goarch>.<ext>, where ext is "zip"
// for Windows and "tar.gz" for everything else. The verifier must
// agree with the builder; if the two diverge, the verifier looks
// for a filename the builder never produced and reports a spurious
// "artifact missing" failure.
//
// The test exercises the same six platforms the builder produces.
// A future platform added to docs/reference/platforms.yaml must be
// added here in the same commit; the mismatch would otherwise
// surface only at release time.
func TestArchiveNameFor(t *testing.T) {
	t.Parallel()

	cases := []struct {
		goos, goarch string
		want         string
	}{
		{"linux", "amd64", "forge-linux-amd64.tar.gz"},
		{"linux", "arm64", "forge-linux-arm64.tar.gz"},
		{"darwin", "amd64", "forge-darwin-amd64.tar.gz"},
		{"darwin", "arm64", "forge-darwin-arm64.tar.gz"},
		{"windows", "amd64", "forge-windows-amd64.zip"},
		{"windows", "arm64", "forge-windows-arm64.zip"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.goos+"/"+tc.goarch, func(t *testing.T) {
			t.Parallel()
			got := archiveNameFor(platformForTest(tc.goos, tc.goarch))
			if got != tc.want {
				t.Errorf("archiveNameFor(%s/%s) = %q, want %q",
					tc.goos, tc.goarch, got, tc.want)
			}
		})
	}
}

// TestArchiveNameFor_Deterministic asserts two calls with the same
// platform produce the same name. The function has no state and no
// randomness; the assertion is a regression guard for a future
// change that adds a per-call suffix (for example, a timestamp).
func TestArchiveNameFor_Deterministic(t *testing.T) {
	t.Parallel()
	p := platformForTest("linux", "amd64")
	first := archiveNameFor(p)
	for i := 0; i < 10; i++ {
		if got := archiveNameFor(p); got != first {
			t.Fatalf("archiveNameFor is not deterministic: %q vs %q",
				first, got)
		}
	}
}

// =============================================================================
// extractTarGz / extractZip
// =============================================================================

// TestExtractTarGz asserts the tar.gz extractor copies the archive's
// single file into the destination directory.
func TestExtractTarGz(t *testing.T) {
	t.Parallel()

	src := t.TempDir()
	archive := filepath.Join(src, "forge.tar.gz")
	if err := writeTarGzFixture(archive, "forge", []byte("binary")); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	dst := t.TempDir()
	if err := extractTarGz(archive, dst); err != nil {
		t.Fatalf("extractTarGz: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dst, "forge"))
	if err != nil {
		t.Fatalf("read extracted: %v", err)
	}
	if string(got) != "binary" {
		t.Errorf("extracted content = %q, want %q", got, "binary")
	}
}

// TestExtractTarGz_MissingArchive asserts the extractor returns an
// error rather than panicking when the archive does not exist.
func TestExtractTarGz_MissingArchive(t *testing.T) {
	t.Parallel()
	dst := t.TempDir()
	err := extractTarGz(filepath.Join(t.TempDir(), "nope.tar.gz"), dst)
	if err == nil {
		t.Fatal("expected error for missing archive")
	}
}

// TestExtractTarGz_CorruptArchive asserts the extractor returns an
// error for a file that is not a valid gzip stream.
func TestExtractTarGz_CorruptArchive(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	archive := filepath.Join(dir, "forge.tar.gz")
	if err := os.WriteFile(archive, []byte("not a gzip stream"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := extractTarGz(archive, dir)
	if err == nil {
		t.Fatal("expected error for corrupt archive")
	}
}

// TestExtractZip asserts the zip extractor copies the archive's
// single file into the destination directory.
func TestExtractZip(t *testing.T) {
	t.Parallel()

	src := t.TempDir()
	archive := filepath.Join(src, "forge.zip")
	if err := writeZipFixture(archive, "forge.exe", []byte("binary")); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	dst := t.TempDir()
	if err := extractZip(archive, dst); err != nil {
		t.Fatalf("extractZip: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dst, "forge.exe"))
	if err != nil {
		t.Fatalf("read extracted: %v", err)
	}
	if string(got) != "binary" {
		t.Errorf("extracted content = %q, want %q", got, "binary")
	}
}

// TestExtractZip_MissingArchive asserts the extractor returns an
// error rather than panicking when the archive does not exist.
func TestExtractZip_MissingArchive(t *testing.T) {
	t.Parallel()
	dst := t.TempDir()
	err := extractZip(filepath.Join(t.TempDir(), "nope.zip"), dst)
	if err == nil {
		t.Fatal("expected error for missing archive")
	}
}

// =============================================================================
// checkStartup
// =============================================================================

// TestCheckStartup_UnderBudget asserts a duration strictly under the
// budget passes.
func TestCheckStartup_UnderBudget(t *testing.T) {
	t.Parallel()
	c := checkStartup(maxStartupDuration / 2)
	if !c.Passed {
		t.Errorf("checkStartup(budget/2) failed: %s", c.Detail)
	}
}

// TestCheckStartup_AtBudget asserts a duration exactly equal to the
// budget passes. The comparison is `> budget`, so equality is a
// pass; the assertion pins the boundary.
func TestCheckStartup_AtBudget(t *testing.T) {
	t.Parallel()
	c := checkStartup(maxStartupDuration)
	if !c.Passed {
		t.Errorf("checkStartup(budget) failed: %s", c.Detail)
	}
}

// TestCheckStartup_OverBudget asserts a duration strictly over the
// budget fails.
func TestCheckStartup_OverBudget(t *testing.T) {
	t.Parallel()
	c := checkStartup(maxStartupDuration + time.Millisecond)
	if c.Passed {
		t.Errorf("checkStartup(budget+1ms) passed: %s", c.Detail)
	}
}

// TestCheckStartup_ReportsDuration asserts the result's Duration
// field is the value passed in, and the Detail names both the
// observed value and the budget.
func TestCheckStartup_ReportsDuration(t *testing.T) {
	t.Parallel()
	const d = 42 * time.Millisecond
	c := checkStartup(d)
	if c.Duration != d {
		t.Errorf("Duration = %v, want %v", c.Duration, d)
	}
	if !contains(c.Detail, "42ms") {
		t.Errorf("Detail = %q, want it to name the observed value", c.Detail)
	}
	if !contains(c.Detail, "100ms") {
		t.Errorf("Detail = %q, want it to name the budget", c.Detail)
	}
}

// =============================================================================
// checkSize
// =============================================================================

// TestCheckSize_WithinBudget asserts an artifact smaller than the
// budget passes.
func TestCheckSize_WithinBudget(t *testing.T) {
	t.Parallel()
	path := writeSparseFile(t, "small.tar.gz", 1024)
	c := checkSize(path)
	if !c.Passed {
		t.Errorf("checkSize(small) failed: %s", c.Detail)
	}
}

// TestCheckSize_AtBudget asserts an artifact exactly at the budget
// passes. The comparison is `> budget`, so equality is a pass.
func TestCheckSize_AtBudget(t *testing.T) {
	t.Parallel()
	path := writeSparseFile(t, "at-budget.tar.gz", int64(maxBinarySize))
	c := checkSize(path)
	if !c.Passed {
		t.Errorf("checkSize(budget) failed: %s", c.Detail)
	}
}

// TestCheckSize_OverBudget asserts an artifact one byte over the
// budget fails. The off-by-one is the boundary; a check that used
// `>= budget` would fail here.
func TestCheckSize_OverBudget(t *testing.T) {
	t.Parallel()
	path := writeSparseFile(t, "over-budget.tar.gz", int64(maxBinarySize)+1)
	c := checkSize(path)
	if c.Passed {
		t.Errorf("checkSize(budget+1) passed: %s", c.Detail)
	}
}

// TestCheckSize_MissingFile asserts the check reports failure when
// the artifact does not exist.
func TestCheckSize_MissingFile(t *testing.T) {
	t.Parallel()
	c := checkSize(filepath.Join(t.TempDir(), "nope.tar.gz"))
	if c.Passed {
		t.Error("checkSize(missing) passed; want failure")
	}
}

// =============================================================================
// firstLine
// =============================================================================

// TestFirstLine asserts the helper returns the first line of a
// string, or the whole string when there is no newline.
func TestFirstLine(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"line", "line"},
		{"line\n", "line"},
		{"line1\nline2", "line1"},
		{"\nline", ""},
		{"\n", ""},
		{"a\nb\nc", "a"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			if got := firstLine(tc.in); got != tc.want {
				t.Errorf("firstLine(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// =============================================================================
// Test fixtures
// =============================================================================

// writeSparseFile creates a file of the given size without writing
// the bytes. The truncate syscall allocates the file's metadata and
// no data blocks; the resulting file is small on disk and cheap to
// create even for a size near the size budget.
//
// The name is placed in a fresh temporary directory so tests do not
// collide.
func writeSparseFile(t *testing.T, name string, size int64) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	if err := f.Truncate(size); err != nil {
		f.Close()
		t.Fatalf("truncate %s: %v", path, err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close %s: %v", path, err)
	}
	return path
}

// writeTarGzFixture writes a single-file .tar.gz archive containing
// one regular file with the given name and content.
func writeTarGzFixture(path, name string, content []byte) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)

	hdr := &tar.Header{
		Name:    name,
		Mode:    0o755,
		Size:    int64(len(content)),
		ModTime: time.Unix(0, 0).UTC(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	if _, err := tw.Write(content); err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

// writeZipFixture writes a single-file .zip archive containing one
// regular file with the given name and content.
func writeZipFixture(path, name string, content []byte) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	if _, err := w.Write(content); err != nil {
		return err
	}
	return zw.Close()
}

// =============================================================================
// Small string helper
// =============================================================================

// contains reports whether s contains substr. It is a tiny wrapper
// over strings.Contains that keeps the test file's import list
// shorter; a future refactor may inline it.
func contains(s, substr string) bool {
	return len(substr) == 0 || len(s) >= len(substr) && indexOf(s, substr) >= 0
}

// indexOf returns the index of substr in s, or -1.
func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
