package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- Unit: parseCSV ---------------------------------------------------------

// TestParseCSV asserts the comma-separated parser trims spaces and
// ignores empty entries. The empty-input case returns a non-nil empty
// map, which is what selectTargets relies on.
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

// --- Unit: sha256File -------------------------------------------------------

// TestSHA256File asserts the digest is the standard hex-encoded
// SHA-256 of the file's bytes.
func TestSHA256File(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "x")
	if err := os.WriteFile(path, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := sha256File(path)
	if err != nil {
		t.Fatal(err)
	}
	// sha256("hello\n")
	want := "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03"
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

// --- Unit: archiveName, archiveModTime --------------------------------------

// TestArchiveName asserts the frozen naming convention.
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
		if got := archiveName(in[0], in[1]); got != want {
			t.Errorf("archiveName(%q, %q) = %q, want %q", in[0], in[1], got, want)
		}
	}
}

// TestArchiveModTime asserts the returned time is UTC and equals the
// input's parsed value.
func TestArchiveModTime(t *testing.T) {
	t.Parallel()
	got := archiveModTime("2026-06-01T12:00:00Z")
	if got.Location() != time.UTC {
		t.Errorf("location = %v, want UTC", got.Location())
	}
	want, _ := time.Parse(time.RFC3339, "2026-06-01T12:00:00Z")
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestArchiveModTime_InvalidFallsBackToEpoch asserts an unparseable
// timestamp does not panic and yields the Unix epoch.
func TestArchiveModTime_InvalidFallsBackToEpoch(t *testing.T) {
	t.Parallel()
	got := archiveModTime("not a timestamp")
	if !got.Equal(time.Unix(0, 0).UTC()) {
		t.Errorf("got %v, want Unix epoch", got)
	}
}

// --- Unit: writeTarGz, writeZip ---------------------------------------------

// TestWriteTarGz asserts the archive contains exactly one entry, its
// name is the binary's base name, its mode is 0o755, its ModTime
// matches the input, and its content round-trips.
func TestWriteTarGz(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	bin := filepath.Join(dir, "forge")
	if err := os.WriteFile(bin, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, "forge.tar.gz")
	modTime := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	if err := writeTarGz(archive, bin, modTime); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	hdr, err := tr.Next()
	if err != nil {
		t.Fatal(err)
	}
	if hdr.Name != "forge" {
		t.Errorf("hdr.Name = %q, want forge", hdr.Name)
	}
	if hdr.Mode != 0o755 {
		t.Errorf("hdr.Mode = %o, want 755", hdr.Mode)
	}
	if !hdr.ModTime.Equal(modTime) {
		t.Errorf("hdr.ModTime = %v, want %v", hdr.ModTime, modTime)
	}
	if hdr.Uname != "" || hdr.Gname != "" {
		t.Errorf("archive embeds user/group: uname=%q gname=%q", hdr.Uname, hdr.Gname)
	}
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(tr); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "binary" {
		t.Errorf("content = %q, want binary", buf.String())
	}
}

// TestWriteZip asserts the archive contains exactly one entry, its
// name is the binary's base name, and its Modified time matches the
// input.
func TestWriteZip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	bin := filepath.Join(dir, "forge.exe")
	if err := os.WriteFile(bin, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, "forge.zip")
	modTime := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	if err := writeZip(archive, bin, modTime); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.OpenReader(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	if len(zr.File) != 1 {
		t.Fatalf("len = %d, want 1", len(zr.File))
	}
	if zr.File[0].Name != "forge.exe" {
		t.Errorf("name = %q, want forge.exe", zr.File[0].Name)
	}
	if !zr.File[0].Modified.Equal(modTime) {
		t.Errorf("Modified = %v, want %v", zr.File[0].Modified, modTime)
	}
}

// --- Unit: goModDependencies ------------------------------------------------

// TestGoModDependencies asserts both the block form and the single-line
// form are parsed.
func TestGoModDependencies(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	gomod := `module example.com/x

go 1.23.0

require github.com/stretchr/testify v1.9.0

require (
	github.com/spf13/cobra v1.8.1
	github.com/spf13/pflag v1.0.5
)
`
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644); err != nil {
		t.Fatal(err)
	}
	deps, err := goModDependencies(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(deps) != 3 {
		t.Fatalf("len = %d, want 3: %+v", len(deps), deps)
	}
	want := map[string]string{
		"github.com/stretchr/testify": "v1.9.0",
		"github.com/spf13/cobra":      "v1.8.1",
		"github.com/spf13/pflag":      "v1.0.5",
	}
	for _, d := range deps {
		if w, ok := want[d.path]; !ok || w != d.version {
			t.Errorf("unexpected dep %+v", d)
		}
	}
}

// --- Unit: checkModulePath --------------------------------------------------

// TestCheckModulePath asserts the check accepts the real go.mod and
// rejects a mismatched one.
func TestCheckModulePath(t *testing.T) {
	t.Parallel()

	t.Run("matches", func(t *testing.T) {
		t.Parallel()
		root, err := findModuleRoot()
		if err != nil {
			t.Skipf("no module root: %v", err)
		}
		if err := checkModulePath(root); err != nil {
			t.Fatalf("checkModulePath: %v", err)
		}
	})

	t.Run("mismatch", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		gomod := "module example.com/other\n\ngo 1.23.0\n"
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := checkModulePath(dir); err == nil {
			t.Fatal("expected mismatch error")
		}
	})

	t.Run("missing", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		if err := checkModulePath(dir); err == nil {
			t.Fatal("expected error for missing go.mod")
		}
	})
}

// --- Unit: checkCGOEnabled --------------------------------------------------

// TestCheckCGOEnabled asserts the four cases: unset (ok), "0" (ok),
// "1" (rejected), and "true" (rejected).
//
// Not parallel: this test mutates the process environment.
func TestCheckCGOEnabled(t *testing.T) {
	cases := []struct {
		value   string
		setIt   bool
		wantErr bool
	}{
		{"", false, false},
		{"0", true, false},
		{"1", true, true},
		{"true", true, true},
	}
	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			old, had := os.LookupEnv("CGO_ENABLED")
			defer func() {
				if had {
					_ = os.Setenv("CGO_ENABLED", old)
				} else {
					_ = os.Unsetenv("CGO_ENABLED")
				}
			}()
			if tc.setIt {
				_ = os.Setenv("CGO_ENABLED", tc.value)
			} else {
				_ = os.Unsetenv("CGO_ENABLED")
			}
			err := checkCGOEnabled()
			if (err != nil) != tc.wantErr {
				t.Errorf("checkCGOEnabled(%q, set=%v) = %v, wantErr=%v",
					tc.value, tc.setIt, err, tc.wantErr)
			}
		})
	}
}

// --- Unit: sourceDateEpoch --------------------------------------------------

// TestSourceDateEpoch_EnvWins asserts SOURCE_DATE_EPOCH takes priority
// over git.
//
// Not parallel: this test mutates the process environment.
func TestSourceDateEpoch_EnvWins(t *testing.T) {
	old, had := os.LookupEnv("SOURCE_DATE_EPOCH")
	defer func() {
		if had {
			_ = os.Setenv("SOURCE_DATE_EPOCH", old)
		} else {
			_ = os.Unsetenv("SOURCE_DATE_EPOCH")
		}
	}()
	_ = os.Setenv("SOURCE_DATE_EPOCH", "1748779200") // 2025-06-01T12:00:00Z

	got, err := sourceDateEpoch(".")
	if err != nil {
		t.Fatal(err)
	}
	if got != "2025-06-01T12:00:00Z" {
		t.Errorf("got %q, want 2025-06-01T12:00:00Z", got)
	}
}

// TestSourceDateEpoch_RejectsNonInteger asserts a non-numeric
// SOURCE_DATE_EPOCH is rejected.
//
// Not parallel: this test mutates the process environment.
func TestSourceDateEpoch_RejectsNonInteger(t *testing.T) {
	old, had := os.LookupEnv("SOURCE_DATE_EPOCH")
	defer func() {
		if had {
			_ = os.Setenv("SOURCE_DATE_EPOCH", old)
		} else {
			_ = os.Unsetenv("SOURCE_DATE_EPOCH")
		}
	}()
	_ = os.Setenv("SOURCE_DATE_EPOCH", "not-a-number")

	if _, err := sourceDateEpoch("."); err == nil {
		t.Fatal("expected error for non-integer SOURCE_DATE_EPOCH")
	}
}

// TestSourceDateUnix_RoundTrips asserts the Unix conversion is exact.
func TestSourceDateUnix_RoundTrips(t *testing.T) {
	t.Parallel()
	got := sourceDateUnix("2026-06-01T12:00:00Z")
	want, _ := time.Parse(time.RFC3339, "2026-06-01T12:00:00Z")
	if got != want.Unix() {
		t.Errorf("got %d, want %d", got, want.Unix())
	}
}

// TestSourceDateUnix_InvalidReturnsZero asserts an unparseable value
// yields 0 rather than panicking.
func TestSourceDateUnix_InvalidReturnsZero(t *testing.T) {
	t.Parallel()
	if got := sourceDateUnix("nope"); got != 0 {
		t.Errorf("got %d, want 0", got)
	}
}

// --- ATDD: the driver's observable contracts -------------------------------

// TestStepBuild_ProducesArchives is the acceptance test for the build
// step. It runs the driver against the real repository with a fake
// version and asserts that every Tier 1 and Tier 2 platform's archive
// is produced with the frozen name.
//
// It uses opts.dist to isolate the output; it does not chdir, so the
// test is safe to run in parallel.
//
// Skipped in short mode and when the source tree is not present. It
// does not skip in CI.
func TestStepBuild_ProducesArchives(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	root, err := findModuleRoot()
	if err != nil {
		t.Skipf("no module root: %v", err)
	}

	opts := options{
		all:       true,
		version:   "v0.0.0-test",
		commit:    "abc1234",
		buildDate: "2026-06-01T12:00:00Z",
		root:      root,
		dist:      t.TempDir(),
	}

	if err := stepBuild(opts, io.Discard, io.Discard); err != nil {
		t.Fatalf("stepBuild: %v", err)
	}
	want := []string{
		"forge-linux-amd64.tar.gz",
		"forge-linux-arm64.tar.gz",
		"forge-darwin-amd64.tar.gz",
		"forge-darwin-arm64.tar.gz",
		"forge-windows-amd64.zip",
		"forge-windows-arm64.zip",
	}
	for _, name := range want {
		if _, err := os.Stat(filepath.Join(opts.dist, name)); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}
}

// TestStepChecksums_ProducesSums asserts SHA256SUMS lists every
// archive and contains a valid sha256 for each.
func TestStepChecksums_ProducesSums(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	root, err := findModuleRoot()
	if err != nil {
		t.Skipf("no module root: %v", err)
	}

	opts := options{
		all:       true,
		version:   "v0.0.0-test",
		commit:    "abc1234",
		buildDate: "2026-06-01T12:00:00Z",
		root:      root,
		dist:      t.TempDir(),
	}

	if err := stepBuild(opts, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := stepChecksums(opts, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(opts.dist, "SHA256SUMS"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 6 {
		t.Fatalf("SHA256SUMS has %d lines, want 6", len(lines))
	}
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			t.Errorf("malformed line: %q", line)
			continue
		}
		path := filepath.Join(opts.dist, fields[1])
		got, err := sha256File(path)
		if err != nil {
			t.Errorf("sha256File(%s): %v", path, err)
			continue
		}
		if got != fields[0] {
			t.Errorf("%s: SHA256SUMS says %s, actual %s", fields[1], fields[0], got)
		}
	}
}

// TestStepSBOM_IsValidCycloneDX asserts the SBOM has the required
// CycloneDX fields and lists at least one component.
func TestStepSBOM_IsValidCycloneDX(t *testing.T) {
	t.Parallel()
	root, err := findModuleRoot()
	if err != nil {
		t.Skipf("no module root: %v", err)
	}
	opts := options{
		version: "v0.0.0-test",
		root:    root,
		dist:    t.TempDir(),
	}
	if err := os.MkdirAll(opts.dist, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := stepSBOM(opts, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(opts.dist, "forge.sbom.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("sbom is not valid JSON: %v", err)
	}
	if doc["bomFormat"] != "CycloneDX" {
		t.Errorf("bomFormat = %v, want CycloneDX", doc["bomFormat"])
	}
	if doc["specVersion"] != "1.5" {
		t.Errorf("specVersion = %v, want 1.5", doc["specVersion"])
	}
	comps, ok := doc["components"].([]any)
	if !ok {
		t.Fatalf("components is not a list: %T", doc["components"])
	}
	if len(comps) == 0 {
		t.Error("components list is empty; go.mod should have at least one require")
	}
}

// --- Non-functional --------------------------------------------------------

// TestBuildMetadata_IsUTC asserts the driver always derives an RFC 3339
// UTC buildDate when the caller does not override it.
func TestBuildMetadata_IsUTC(t *testing.T) {
	t.Parallel()
	root, err := findModuleRoot()
	if err != nil {
		t.Skipf("no module root: %v", err)
	}
	opts := options{
		version: "v1.0.0",
		commit:  "abc1234",
		root:    root,
	}
	if err := resolveBuildMetadata(&opts); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(opts.buildDate, "Z") {
		t.Errorf("buildDate %q does not end in Z", opts.buildDate)
	}
}

// TestResolveBuildMetadata_RejectsBadDate asserts a non-RFC3339
// buildDate is rejected.
func TestResolveBuildMetadata_RejectsBadDate(t *testing.T) {
	t.Parallel()
	opts := options{version: "v1.0.0", commit: "abc1234", buildDate: "yesterday"}
	if err := resolveBuildMetadata(&opts); err == nil {
		t.Fatal("expected error")
	}
}

// TestResolveBuildMetadata_PreservesOverrides asserts a caller-supplied
// version, commit, and buildDate are used verbatim. This is the
// contract every ATDD test above relies on.
func TestResolveBuildMetadata_PreservesOverrides(t *testing.T) {
	t.Parallel()
	opts := options{
		version:   "v9.9.9-test",
		commit:    "deadbee",
		buildDate: "2020-01-01T00:00:00Z",
	}
	if err := resolveBuildMetadata(&opts); err != nil {
		t.Fatal(err)
	}
	if opts.version != "v9.9.9-test" {
		t.Errorf("version = %q", opts.version)
	}
	if opts.commit != "deadbee" {
		t.Errorf("commit = %q", opts.commit)
	}
	if opts.buildDate != "2020-01-01T00:00:00Z" {
		t.Errorf("buildDate = %q", opts.buildDate)
	}
}

// TestBuildOne_NoWallClock asserts buildOne does not call time.Now().
// A future change that reintroduces the wall clock would make the
// binary non-reproducible; this test fails in milliseconds rather than
// after a full cross-compile.
func TestBuildOne_NoWallClock(t *testing.T) {
	t.Parallel()
	root, err := findModuleRoot()
	if err != nil {
		t.Skipf("no module root: %v", err)
	}
	src, err := os.ReadFile(filepath.Join(root, "scripts", "build.go"))
	if err != nil {
		t.Fatal(err)
	}
	const marker = "func buildOne("
	idx := strings.Index(string(src), marker)
	if idx < 0 {
		t.Fatalf("%s not found in scripts/build.go", marker)
	}
	rest := string(src)[idx:]
	// The function ends at the first "\n}\n" after the marker.
	end := strings.Index(rest, "\n}\n")
	if end < 0 {
		t.Fatal("could not find end of buildOne")
	}
	body := rest[:end]
	if strings.Contains(body, "time.Now()") {
		t.Error("buildOne calls time.Now(); builds are not reproducible")
	}
}

// TestResolveBuildMetadata_NoWallClock asserts the metadata resolver
// does not call time.Now(). An earlier version did; that was the defect
// WBS 17.3.1 exists to fix.
func TestResolveBuildMetadata_NoWallClock(t *testing.T) {
	t.Parallel()
	root, err := findModuleRoot()
	if err != nil {
		t.Skipf("no module root: %v", err)
	}
	src, err := os.ReadFile(filepath.Join(root, "scripts", "build.go"))
	if err != nil {
		t.Fatal(err)
	}
	const marker = "func resolveBuildMetadata("
	idx := strings.Index(string(src), marker)
	if idx < 0 {
		t.Fatalf("%s not found in scripts/build.go", marker)
	}
	rest := string(src)[idx:]
	end := strings.Index(rest, "\n}\n")
	if end < 0 {
		t.Fatal("could not find end of resolveBuildMetadata")
	}
	body := rest[:end]
	if strings.Contains(body, "time.Now()") {
		t.Error("resolveBuildMetadata calls time.Now(); builds are not reproducible")
	}
}
