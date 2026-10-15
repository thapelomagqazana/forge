// Package release_test contains end-to-end acceptance tests for the
// supply-chain integrity pipeline (WBS 17.5.2).
//
// The tests build the artifacts, run the three supply-chain drivers,
// and assert every expected file is present and well-formed. They
// exercise the drivers' observable contract.
package release_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestSupplyChain_PipelineProducesAllArtifacts is the ATDD acceptance
// test for WBS 17.5.2.
//
// The test:
//
//  1. Builds the artifacts.
//  2. Runs the checksums driver.
//  3. Runs the SBOM driver.
//  4. Runs the sign driver.
//  5. Asserts every expected file is present.
//
// The sign driver requires credentials the CI does not have in the
// test environment. The test does not assert that signatures exist;
// it asserts that the driver ran and reported blockers or success.
// A separate job in the release workflow asserts the signatures when
// the credentials are available.
func TestSupplyChain_PipelineProducesAllArtifacts(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping supply-chain tests in short mode")
	}
	t.Parallel()

	root := repoRoot(t)
	dist := t.TempDir()

	// 1. Build.
	runTool(t, root, "go", "run", "./scripts/build.go", "--all", "--dist="+dist)

	// 2. Checksums.
	runTool(t, root, "go", "run", "./cmd/forge-internal-checksums",
		"--dist="+dist, "--no-gpg")

	// 3. SBOM.
	runTool(t, root, "go", "run", "./cmd/forge-internal-sbom",
		"--dist="+dist, "--version=v0.0.0-test")

	// 4. Sign. The driver records blockers when credentials are
	// absent; the test does not require them.
	runTool(t, root, "go", "run", "./cmd/forge-internal-sign", "--dist="+dist)

	// 5. Assert artifacts.
	for _, name := range []string{
		"SHA256SUMS",
		"forge.sbom.json",
		"forge-linux-amd64.tar.gz.sha256",
		"forge-darwin-arm64.tar.gz.sha256",
		"forge-windows-amd64.zip.sha256",
		"forge-linux-amd64.tar.gz.cdx.json",
	} {
		path := filepath.Join(dist, name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("missing artifact: %s (%v)", name, err)
		}
	}
}

// TestSupplyChain_SHA256SUMSHasSixEntries asserts the combined
// checksum file lists every Tier 1/2 archive.
func TestSupplyChain_SHA256SUMSHasSixEntries(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping supply-chain tests in short mode")
	}
	t.Parallel()

	root := repoRoot(t)
	dist := t.TempDir()

	runTool(t, root, "go", "run", "./scripts/build.go", "--all", "--dist="+dist)
	runTool(t, root, "go", "run", "./cmd/forge-internal-checksums",
		"--dist="+dist, "--no-gpg")

	data, err := os.ReadFile(filepath.Join(dist, "SHA256SUMS"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 6 {
		t.Errorf("SHA256SUMS has %d lines, want 6\n%s", len(lines), data)
	}
}

// TestSupplyChain_SBOMsAreValidJSON asserts every SBOM parses as
// JSON and has the CycloneDX fields.
func TestSupplyChain_SBOMsAreValidJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping supply-chain tests in short mode")
	}
	t.Parallel()

	root := repoRoot(t)
	dist := t.TempDir()

	runTool(t, root, "go", "run", "./scripts/build.go", "--all", "--dist="+dist)
	runTool(t, root, "go", "run", "./cmd/forge-internal-sbom",
		"--dist="+dist, "--version=v0.0.0-test")

	entries, err := os.ReadDir(dist)
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".cdx.json") {
			continue
		}
		found++
		data, err := os.ReadFile(filepath.Join(dist, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		var doc map[string]any
		if err := json.Unmarshal(data, &doc); err != nil {
			t.Errorf("%s is not valid JSON: %v", e.Name(), err)
			continue
		}
		if doc["bomFormat"] != "CycloneDX" {
			t.Errorf("%s bomFormat = %v", e.Name(), doc["bomFormat"])
		}
	}
	if found != 6 {
		t.Errorf("found %d per-binary SBOMs, want 6", found)
	}
}

// runTool runs a command in dir and fails the test on a non-zero
// exit.
func runTool(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out.String())
	}
}

// repoRoot walks up from the test's cwd to find go.mod.
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
			t.Fatal("no go.mod found")
		}
		dir = parent
	}
}
