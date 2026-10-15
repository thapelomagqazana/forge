package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// writeFixtureDist writes a directory tree the distribution driver can
// parse: a dist/ directory with one archive per platform and a
// SHA256SUMS file, plus a go.mod next to dist/.
//
// The helper returns the dist/ directory's path. The driver's
// ParseRelease reads two paths:
//
//   - <dist>/SHA256SUMS, which names every archive and its digest.
//   - <dist>/../go.mod, which names the module path the goinstall
//     adapter verifies.
//
// The helper writes both. A caller that passed a temp directory
// without a go.mod would make ParseRelease fail with "read go.mod:
// no such file or directory"; the helper's job is to satisfy the
// driver's inputs, not to be minimal.
//
// The fixture is deliberately fixed: every test in this package
// asserts against the same version (v1.4.2), the same six archives,
// the same checksums, and the same module path. A change to the
// fixture is a change to every test that uses it.
//
// The archives' bytes are small ASCII strings. ParseRelease reads
// each archive's size from the filesystem and each archive's
// checksum from SHA256SUMS; it does not hash the archive's bytes.
// The helper computes SHA256SUMS from the bytes it writes, so the
// two agree: a future test that asserted the checksum equals the
// hash of the bytes would pass.
func writeFixtureDist(t *testing.T, parent string) string {
	t.Helper()

	// The driver's ParseRelease reads the module path from
	// <dist>/../go.mod. The fixture's dist/ is at
	// <parent>/dist, so the driver reads <parent>/go.mod. The
	// helper writes a minimal one so the read succeeds.
	//
	// The module path is the one the installation guide
	// documents and the one the goinstall adapter verifies; a
	// fixture with a different path would make the driver's
	// goinstall channel fail. The Go version in the file is
	// irrelevant to the driver's read; the helper writes a
	// recent one so the file is a valid go.mod.
	gomod := "module github.com/forge-cli/forge\n\ngo 1.22\n"
	if err := os.WriteFile(filepath.Join(parent, "go.mod"), []byte(gomod), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	dist := filepath.Join(parent, "dist")
	if err := os.MkdirAll(dist, 0o755); err != nil {
		t.Fatalf("mkdir dist: %v", err)
	}

	archives := []struct {
		name    string
		content string
	}{
		{"forge-darwin-amd64.tar.gz", "darwin-amd64"},
		{"forge-darwin-arm64.tar.gz", "darwin-arm64"},
		{"forge-linux-amd64.tar.gz", "linux-amd64"},
		{"forge-linux-arm64.tar.gz", "linux-arm64"},
		{"forge-windows-amd64.zip", "windows-amd64"},
		{"forge-windows-arm64.zip", "windows-arm64"},
	}

	lines := make([]string, 0, len(archives))
	for _, a := range archives {
		path := filepath.Join(dist, a.name)
		if err := os.WriteFile(path, []byte(a.content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
		sum := sha256.Sum256([]byte(a.content))
		lines = append(lines, fmt.Sprintf("%s  %s", hex.EncodeToString(sum[:]), a.name))
	}

	sums := ""
	for _, line := range lines {
		sums += line + "\n"
	}
	if err := os.WriteFile(filepath.Join(dist, "SHA256SUMS"), []byte(sums), 0o644); err != nil {
		t.Fatalf("write SHA256SUMS: %v", err)
	}

	return dist
}
