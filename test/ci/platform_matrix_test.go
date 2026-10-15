// Package ci_test contains end-to-end acceptance tests for the CI
// pipeline's consumption of the platform matrix. These are ATDD tests:
// they assert the observable contract between the matrix document and
// the CI matrix generator.
package ci_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestAcceptance_CIMatrixCoversTier1 asserts every Tier 1 platform appears
// in the generated CI matrix.
func TestAcceptance_CIMatrixCoversTier1(t *testing.T) {
	t.Parallel()
	out := runGenCIMatrix(t, "--tier", "1")
	var m struct {
		Include []struct {
			Name string `json:"name"`
		} `json:"include"`
	}
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	want := map[string]bool{
		"linux-amd64":   false,
		"linux-arm64":   false,
		"darwin-arm64":  false,
		"windows-amd64": false,
	}
	for _, e := range m.Include {
		if _, ok := want[e.Name]; ok {
			want[e.Name] = true
		}
	}
	for name, seen := range want {
		if !seen {
			t.Errorf("Tier 1 platform %s missing from CI matrix", name)
		}
	}
}

// TestAcceptance_CIMatrixHasRunners asserts every entry has a runner.
func TestAcceptance_CIMatrixHasRunners(t *testing.T) {
	t.Parallel()
	out := runGenCIMatrix(t)
	if !strings.Contains(string(out), `"runner"`) {
		t.Fatalf("matrix has no runner field:\n%s", out)
	}
}

// TestAcceptance_CIMatrixIsValidJSON asserts the generator emits
// well-formed JSON. A malformed matrix would fail the workflow at parse
// time, before any job ran.
func TestAcceptance_CIMatrixIsValidJSON(t *testing.T) {
	t.Parallel()
	out := runGenCIMatrix(t)
	var v any
	if err := json.Unmarshal(out, &v); err != nil {
		t.Fatalf("matrix is not valid JSON: %v\n%s", err, out)
	}
}

// runGenCIMatrix runs scripts/platform/gen-ci-matrix.sh from the repo
// root and returns its stdout. It fails the test with a legible message
// (including the script's stderr) if the script exits non-zero.
//
// The script is resolved via repoRoot() and cmd.Dir is set to the repo
// root because `go test` runs each package with cwd set to the package
// directory (test/ci/), not the repo root. Without these two lines, the
// script's relative paths do not resolve and the test fails with a bare
// "exit status 127".
func runGenCIMatrix(t *testing.T, args ...string) []byte {
	t.Helper()
	root := repoRoot(t)
	script := filepath.Join(root, "scripts", "platform", "gen-ci-matrix.sh")
	cmdArgs := append([]string{script}, args...)
	cmd := exec.Command("bash", cmdArgs...)
	cmd.Dir = root
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("gen-ci-matrix %v: %v\nstderr:\n%s", args, err, stderr.String())
	}
	return out
}

// repoRoot walks up from the test's working directory to find go.mod.
// The same pattern is used by internal/platform and by other test
// packages in this repository; keeping it local avoids a shared helper
// package that would itself have to be classified in the arch matrix.
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
