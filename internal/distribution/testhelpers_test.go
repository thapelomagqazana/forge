package distribution_test

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thapelomagqazana/forge/internal/distribution"
)

// fixtureRelease returns a Release that every adapter's test uses. The
// fixture is deliberately fixed: every adapter test asserts against
// the same version, the same archives, and the same checksums. A
// change to the fixture changes every adapter's golden; the change is
// reviewed once.
//
// The releaseChannel argument selects the release's channel. The
// default fixture is a stable release; an adapter's
// TestXxx_RejectsNightly passes "nightly" and expects
// ErrNotApplicable.
//
// The fixture is built in memory; it does not read a dist/ directory.
// ParseRelease is tested separately against a real directory
// (TestParseRelease_*), so the adapters' tests do not need one.
//
// The fixture's fields are what every adapter reads:
//
//   - Version: the adapters' manifests name it.
//   - Channel: the adapters' ErrNotApplicable check reads it.
//   - PublishedAt: the apt adapter's Release file records it.
//   - Archives: the adapters' platformURLs helper reads it.
//   - Checksums: the adapters' checksum fields read it.
//   - GPGFingerprint: the apt adapter embeds it.
//   - ModulePath: the goinstall adapter verifies it.
//
// A future field an adapter reads would be added here. A field that
// is set but not read by any adapter is either a fixture for a
// future test or a leftover; the fixture's doc names the readers.
func fixtureRelease(releaseChannel string) distribution.Release {
	const version = "v1.4.2"

	archives := map[string]distribution.Archive{
		"darwin/amd64": {
			Name:      "forge-darwin-amd64.tar.gz",
			GOOS:      "darwin",
			GOARCH:    "amd64",
			SHA256:    "aaaa000000000000000000000000000000000000000000000000000000000001",
			SizeBytes: 12_000_000,
		},
		"darwin/arm64": {
			Name:      "forge-darwin-arm64.tar.gz",
			GOOS:      "darwin",
			GOARCH:    "arm64",
			SHA256:    "aaaa000000000000000000000000000000000000000000000000000000000002",
			SizeBytes: 12_100_000,
		},
		"linux/amd64": {
			Name:      "forge-linux-amd64.tar.gz",
			GOOS:      "linux",
			GOARCH:    "amd64",
			SHA256:    "aaaa000000000000000000000000000000000000000000000000000000000003",
			SizeBytes: 12_200_000,
		},
		"linux/arm64": {
			Name:      "forge-linux-arm64.tar.gz",
			GOOS:      "linux",
			GOARCH:    "arm64",
			SHA256:    "aaaa000000000000000000000000000000000000000000000000000000000004",
			SizeBytes: 12_300_000,
		},
		"windows/amd64": {
			Name:      "forge-windows-amd64.zip",
			GOOS:      "windows",
			GOARCH:    "amd64",
			SHA256:    "aaaa000000000000000000000000000000000000000000000000000000000005",
			SizeBytes: 12_400_000,
		},
		"windows/arm64": {
			Name:      "forge-windows-arm64.zip",
			GOOS:      "windows",
			GOARCH:    "arm64",
			SHA256:    "aaaa000000000000000000000000000000000000000000000000000000000006",
			SizeBytes: 12_500_000,
		},
	}

	checksums := map[string]string{}
	for _, a := range archives {
		checksums[a.Name] = a.SHA256
	}

	return distribution.Release{
		Version:        version,
		Channel:        releaseChannel,
		PublishedAt:    "2026-04-15T12:00:00Z",
		Archives:       archives,
		Checksums:      checksums,
		GPGFingerprint: "DEADBEEFDEADBEEFDEADBEEFDEADBEEFDEADBEEF",
		ModulePath:     "github.com/forge-cli/forge",
	}
}

// assertGolden compares got against the golden file at
// testdata/<name>. The test writes the golden when the test is run
// with -update.
//
// The helper is the package's golden-file convention: every adapter
// test that has a golden calls it, and every golden lives in
// testdata/. The -update flag is the Go standard's; the helper
// implements it the way the standard library's own golden tests do.
//
// The helper is not exported: it is a test helper in the
// distribution_test package, alongside the tests that use it.
//
// A golden test's value is its diff. A template change makes the
// golden's bytes differ from the rendered output; the test reports
// the whole file. A reviewer who reads the diff sees exactly what
// changed in the adapter's output.
func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir testdata: %v", err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", path, err)
	}
	if string(want) != string(got) {
		t.Errorf("golden %s does not match\n--- want ---\n%s\n--- got ---\n%s", name, want, got)
	}
}

// updateGolden is the -update flag the golden helper reads. It is
// declared once, here, so every golden test shares it.
//
// The flag's name matches the convention the repository already
// uses: the arch tests' -update flag, the version tests' -update
// flag. A user who runs `go test ./... -update` regenerates every
// golden in one command.
var updateGolden = flag.Bool("update", false, "rewrite the golden files")

// repoRoot walks up from the test's working directory to find the
// repository root (the directory containing go.mod). It fails the
// test if no go.mod is found.
//
// The helper is the package's way of locating the repository. A test
// that reads the repository's go.mod (writeFixtureDist does) needs
// the path; the helper supplies it.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod found in any parent directory")
		}
		dir = parent
	}
}

// fixtureArchive is one archive the fixture dist/ directory contains.
type fixtureArchive struct {
	// Name is the archive's filename, e.g.
	// "forge-linux-amd64.tar.gz". It is the name the driver's
	// ParseRelease reads from SHA256SUMS.
	Name string

	// Content is the archive's bytes. The fixture does not produce
	// a real tar.gz or zip; it produces a few bytes whose SHA-256
	// is what the fixture's SHA256SUMS names. The adapters read
	// the archive's checksum from SHA256SUMS, not from the
	// archive's bytes; the fixture's content only needs to exist
	// so ParseRelease's fileSize call succeeds.
	Content []byte
}

// writeFixtureDist writes a directory tree the distribution driver
// can parse: a dist/ directory with one archive per platform and a
// SHA256SUMS file, plus a go.mod next to dist/.
//
// The helper returns the dist/ directory's path. The driver's
// ParseRelease reads two paths:
//
//   - <dist>/SHA256SUMS, which names every archive and its digest.
//   - <dist>/../go.mod, which names the module path the goinstall
//     adapter verifies.
//
// The helper writes both. The go.mod's module path is read from the
// repository's own go.mod, so a future module-path change is
// reflected in the fixture without a second edit.
//
// The root argument is the repository root; the helper's callers
// pass the value repoRoot returns. The helper fails the test if
// <root>/go.mod is missing.
//
// The fixture is deliberately fixed: every acceptance test asserts
// against the same version (v1.4.2), the same six archives, the
// same checksums, and the same module path. A change to the fixture
// is a change to every test that uses it.
//
// The directory's shape matches what the release pipeline's build
// job produces: one archive per platform, plus a SHA256SUMS file
// naming each archive's digest. The pipeline's build job does not
// produce a checksum for each archive separately; the per-archive
// files are a convenience the release job adds. The fixture writes
// only SHA256SUMS, because that is what ParseRelease reads.
func writeFixtureDist(t *testing.T, parent, root string) string {
	t.Helper()

	// The driver's ParseRelease reads the module path from
	// <dist>/../go.mod. The fixture's dist/ is at <parent>/dist,
	// so the driver reads <parent>/go.mod. The helper writes one,
	// with the module path the repository's own go.mod declares.
	modulePath, err := readModulePath(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatalf("writeFixtureDist: %v", err)
	}
	gomod := "module " + modulePath + "\n\ngo 1.22\n"
	if err := os.WriteFile(filepath.Join(parent, "go.mod"), []byte(gomod), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	dist := filepath.Join(parent, "dist")
	if err := os.MkdirAll(dist, 0o755); err != nil {
		t.Fatalf("mkdir dist: %v", err)
	}

	archives := []fixtureArchive{
		{Name: "forge-darwin-amd64.tar.gz", Content: []byte("darwin-amd64")},
		{Name: "forge-darwin-arm64.tar.gz", Content: []byte("darwin-arm64")},
		{Name: "forge-linux-amd64.tar.gz", Content: []byte("linux-amd64")},
		{Name: "forge-linux-arm64.tar.gz", Content: []byte("linux-arm64")},
		{Name: "forge-windows-amd64.zip", Content: []byte("windows-amd64")},
		{Name: "forge-windows-arm64.zip", Content: []byte("windows-arm64")},
	}

	// Write each archive's bytes and remember its SHA-256 for
	// SHA256SUMS.
	lines := make([]string, 0, len(archives))
	for _, a := range archives {
		path := filepath.Join(dist, a.Name)
		if err := os.WriteFile(path, a.Content, 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
		sum := sha256.Sum256(a.Content)
		lines = append(lines, fmt.Sprintf("%s  %s", hex.EncodeToString(sum[:]), a.Name))
	}

	// SHA256SUMS is in the standard `sha256sum` format: one line
	// per archive, "<hex>  <name>". Two spaces between the digest
	// and the name is the canonical form; the driver's parser
	// accepts one or more whitespace.
	sums := ""
	for _, line := range lines {
		sums += line + "\n"
	}
	if err := os.WriteFile(filepath.Join(dist, "SHA256SUMS"), []byte(sums), 0o644); err != nil {
		t.Fatalf("write SHA256SUMS: %v", err)
	}

	return dist
}

// readModulePath reads the module path from a go.mod file. It
// returns the path or an error if the file has no module directive.
//
// The helper is deliberately minimal: it reads the first line that
// begins with "module " and returns the rest of the line. A full
// go.mod parser would handle comments, quoted paths, and the
// directive's other forms; the fixture's go.mod is a two-line file
// the helper itself writes, and the repository's go.mod's module
// line is the file's first non-comment line. The minimal parser is
// sufficient for both.
func readModulePath(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	return "", fmt.Errorf("%s has no module directive", path)
}
