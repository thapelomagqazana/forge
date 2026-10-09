// Package main_test contains a wiring test that proves main.go depends
// on internal/cli.Execute(). The test is unusual: rather than asserting
// behaviour, it asserts a build-time relationship, by deliberately
// breaking the wiring in a temporary copy of the module and confirming
// that the build fails.
package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestMainGoWiringIsReal verifies AC7: removing the dependency on
// internal/cli.Execute() causes a compile error. This is a negative
// test: it asserts that a specific *failure* occurs.
//
// Implementation notes:
//
//   - We copy the entire module to a temporary directory.
//   - We rewrite the import in the copy to point at a non-existent
//     package.
//   - We attempt to build, and assert that the build fails with an
//     error mentioning the missing package.
//
// This is slower than a unit test (it invokes `go build`), but it is
// the only way to prove that the wiring is not decorative.
func TestMainGoWiringIsReal(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping wiring test in short mode")
	}

	// Locate the module root (four directories up from cmd/forge).
	moduleRoot := findModuleRoot(t)

	tmpDir := t.TempDir()
	copyTree(t, moduleRoot, tmpDir)

	// Rewrite main.go in the copy so that the internal/cli import points
	// at a package that does not exist. If main.go were not genuinely
	// wired to internal/cli, this rewrite would not break anything.
	mainCopy := filepath.Join(tmpDir, "cmd", "forge", "main.go")
	data, err := os.ReadFile(mainCopy)
	if err != nil {
		t.Fatalf("read %s: %v", mainCopy, err)
	}

	const realImport = `"github.com/thapelomagqazana/forge/internal/cli"`
	const brokenImport = `"github.com/thapelomagqazana/forge/internal/__nonexistent__"`

	content := string(data)
	if !strings.Contains(content, realImport) {
		t.Fatalf("expected main.go to import %s", realImport)
	}
	content = strings.ReplaceAll(content, realImport, brokenImport)

	if err := os.WriteFile(mainCopy, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", mainCopy, err)
	}

	// Attempt to build the copied module. Expect failure.
	cmd := exec.Command("go", "build", "./cmd/forge")
	cmd.Dir = tmpDir
	out, err := cmd.CombinedOutput()

	if err == nil {
		t.Fatalf("AC7 FAIL: build succeeded despite broken import; wiring is not real")
	}

	if !strings.Contains(string(out), "__nonexistent__") &&
		!strings.Contains(string(out), "no required module") &&
		!strings.Contains(string(out), "cannot find package") {
		t.Logf("build output:\n%s", out)
		t.Fatalf("AC7 FAIL: build failed but not due to the broken import")
	}
}

// findModuleRoot walks up from the current directory until it finds go.mod.
func findModuleRoot(t *testing.T) string {
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
			t.Fatalf("could not find module root from %s", dir)
		}
		dir = parent
	}
}

// copyTree recursively copies a directory tree into dst, excluding:
//
//   - the .git directory (at any depth)
//   - the built binary named "forge" at the source root
//   - the built binary named "forge.exe" at the source root
//
// The skip logic is expressed in two distinct ways:
//
//  1. Directories are skipped by base name (.git may appear at any
//     depth; the check is name-based).
//
//  2. Files are skipped by full relative path (only the root-level
//     binary is skipped; a source file at cmd/forge/main.go must be
//     copied).
//
// This distinction is the fix for a subtle bug: skipping directories
// by base name is safe for .git, but skipping files by base name
// would incorrectly skip cmd/forge/main.go (whose directory is named
// "forge") if the file skip were also name-based.
//
// The copy is deliberately minimal: no symlink following, no
// permission preservation beyond 0o644/0o755.
func copyTree(t *testing.T, src, dst string) {
	t.Helper()

	// Directories to skip, by base name. Only .git qualifies.
	skipDirNames := map[string]bool{
		".git": true,
	}

	// Files to skip, by full path relative to src. Only the built
	// binaries at the source root qualify.
	skipFilePaths := map[string]bool{
		"forge":     true,
		"forge.exe": true,
	}

	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// A read error in a subtree is not fatal; skip it and
			// continue. This is defensive: the walk is over a
			// clean checkout, so read errors should not occur.
			return nil
		}

		rel, relErr := filepath.Rel(src, path)
		if relErr != nil {
			return relErr
		}

		// The root of the walk: create dst and continue descending.
		if rel == "." {
			return nil
		}

		// Directories: skip by name, otherwise create.
		if info.IsDir() {
			if skipDirNames[info.Name()] {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}

		// Regular files: skip by relative path.
		if skipFilePaths[rel] {
			return nil
		}

		// Copy the file. Preserve the file's mode but use a safe
		// default for directories.
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}

		target := filepath.Join(dst, rel)
		if mkErr := os.MkdirAll(filepath.Dir(target), 0o755); mkErr != nil {
			return mkErr
		}
		return os.WriteFile(target, data, info.Mode())
	})
	if err != nil {
		t.Fatalf("copy tree: %v", err)
	}
}
