// Package main_test contains a wiring test that proves main.go depends
// on internal/cli. The test is unusual: rather than asserting
// behaviour, it asserts a build-time relationship, by deliberately
// breaking the wiring in a temporary copy of the module and confirming
// that the build fails.
//
// # Why the test names the package, not a function
//
// The substantive property is that main.go imports internal/cli.
// The specific function main.go calls (Execute in WBS 2.2.1, Run in
// WBS 11.3.1) is an implementation detail that may change without
// affecting the property. Stating the property at the package level
// keeps the test resilient to renames inside the package.
package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestMainGoWiringIsReal verifies AC7: removing the dependency on
// internal/cli causes a compile error. This is a negative test: it
// asserts that a specific *failure* occurs.
//
// Implementation notes:
//
//  1. Copy the entire module to a temporary directory.
//  2. Positive control: build the unmodified copy. If the build
//     fails, the environment is broken (wrong GOOS, missing module
//     cache, network restrictions) and the negative build's failure
//     would be uninformative. The test stops here to avoid a
//     false-positive pass.
//  3. Rewrite the internal/cli import in the copy to point at a
//     non-existent package.
//  4. Build again. Assert that the build fails, and that the failure
//     names the broken import.
//
// Step 2 is the load-bearing addition relative to the original
// (WBS 2.2.1) version. Without it, a broken `go` command would cause
// both builds to fail, and the test would pass for the wrong reason.
// The control costs one extra `go build` invocation and removes the
// false-positive mode entirely.
//
// This is slower than a unit test (it invokes `go build` twice), but
// it is the only way to prove that the wiring is not decorative.
func TestMainGoWiringIsReal(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping wiring test in short mode")
	}

	// Locate the module root (four directories up from cmd/forge).
	moduleRoot := findModuleRoot(t)

	tmpDir := t.TempDir()
	copyTree(t, moduleRoot, tmpDir)

	// Step 2: positive control. Prove the unmodified copy builds.
	// If this fails, the environment is broken and the negative
	// build's failure would be uninformative.
	//
	// The control runs before the import is broken, so it builds
	// the same module the developer's checkout would build. The
	// build target is ./cmd/forge because that is the package the
	// test is about; building the whole module would be slower and
	// would make the test sensitive to unrelated packages' build
	// health.
	positiveCmd := exec.Command("go", "build", "./cmd/forge")
	positiveCmd.Dir = tmpDir
	if out, err := positiveCmd.CombinedOutput(); err != nil {
		t.Fatalf("positive control: unmodified module failed to build: %v\n%s",
			err, out)
	}

	// Step 3: rewrite main.go in the copy so that the internal/cli
	// import points at a package that does not exist. If main.go
	// were not genuinely wired to internal/cli, this rewrite would
	// not break anything.
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

	// Step 4: attempt to build the copied module again. Expect
	// failure.
	negativeCmd := exec.Command("go", "build", "./cmd/forge")
	negativeCmd.Dir = tmpDir
	out, err := negativeCmd.CombinedOutput()

	if err == nil {
		t.Fatalf("AC7 FAIL: build succeeded despite broken import; " +
			"wiring is not real")
	}

	// The build's failure must be attributable to the broken import,
	// not to something else. The Go toolchain's error message for a
	// missing package mentions the package path; the test checks for
	// the sentinel string we wrote into the import, and for the two
	// common phrasings the toolchain uses ("no required module",
	// "cannot find package").
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
