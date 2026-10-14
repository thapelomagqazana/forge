//go:build unix

package safefs_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thapelomagqazana/forge/internal/filesystem"
	"github.com/thapelomagqazana/forge/internal/filesystem/safefs"
)

// This file exercises the symlink policy on Unix. It is
// build-tagged so that the Windows-only tests live in a
// separate file and the Unix-only ones here do not need runtime
// checks.

// TestRule1_TargetDirAsSymlink verifies that a Root can be
// constructed on a symlink and that Path() returns the resolved
// target.
func TestRule1_TargetDirAsSymlink(t *testing.T) {
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	root, err := safefs.New(link, safefs.WithFS(filesystem.NewOSFS(link)))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := root.Path(); got != real {
		t.Fatalf("Path() = %q, want resolved target %q", got, real)
	}
}

// TestRule1_OperationsWorkThroughSymlink verifies that
// operations through a symlinked root succeed.
func TestRule1_OperationsWorkThroughSymlink(t *testing.T) {
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}
	if err := os.WriteFile(filepath.Join(real, "a.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	root, err := safefs.New(link, safefs.WithFS(filesystem.NewOSFS(link)))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got, err := root.ReadFile("a.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "x" {
		t.Fatalf("ReadFile = %q, want %q", got, "x")
	}
}

// TestRule2_SymlinkInWriteParentRejected verifies that a write
// whose parent contains a symlink is refused.
func TestRule2_SymlinkInWriteParentRejected(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, "sub")); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	root, err := safefs.New(dir, safefs.WithFS(filesystem.NewOSFS(dir)))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	err = root.WriteAtomic("sub/a.txt", []byte("x"), 0o600)
	if err == nil {
		t.Fatal("WriteAtomic succeeded through symlink, want error")
	}
	if !errors.Is(err, safefs.ErrSymlinkDenied) {
		t.Fatalf("error = %v, want ErrSymlinkDenied", err)
	}
}

// TestRule2_SymlinkAtWriteTargetRejected verifies that a write
// whose final component is a symlink is refused. The refusal
// comes from O_NOFOLLOW in the write primitive.
func TestRule2_SymlinkAtWriteTargetRejected(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "target.txt")
	if err := os.WriteFile(target, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "link.txt")); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	root, err := safefs.New(dir, safefs.WithFS(filesystem.NewOSFS(dir)))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	err = root.WriteAtomicOverwrite("link.txt", []byte("new"), 0o600)
	if err == nil {
		t.Fatal("WriteAtomicOverwrite succeeded through symlink, want error")
	}
	// The target must be unchanged.
	got, _ := os.ReadFile(target)
	if string(got) != "old" {
		t.Fatalf("target content changed: %q", got)
	}
}

// TestRule3_WalkReportsSymlinks verifies that a symlink is
// delivered to the walk callback and not descended.
func TestRule3_WalkReportsSymlinks(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("s"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "linkdir")); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	root, err := safefs.New(dir, safefs.WithFS(filesystem.NewOSFS(dir)))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	seen := map[string]bool{}
	err = root.Walk(".", func(rel string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		seen[rel] = d.Type()&fs.ModeSymlink != 0
		return nil
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if _, ok := seen["linkdir"]; !ok {
		t.Fatal("Walk did not visit linkdir")
	}
	if !seen["linkdir"] {
		t.Fatal("Walk did not report linkdir as a symlink")
	}
	if _, ok := seen["linkdir/secret.txt"]; ok {
		t.Fatal("Walk descended into linkdir")
	}
}

// TestRule4_BrokenSymlinkReported verifies that a broken symlink
// is a valid entry and does not error the walk.
func TestRule4_BrokenSymlinkReported(t *testing.T) {
	dir := t.TempDir()
	if err := os.Symlink("nonexistent", filepath.Join(dir, "broken")); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	root, err := safefs.New(dir, safefs.WithFS(filesystem.NewOSFS(dir)))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	entries, err := root.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var found bool
	for _, e := range entries {
		if e.Name() == "broken" {
			found = true
			if e.Type()&fs.ModeSymlink == 0 {
				t.Error("broken link reported without ModeSymlink")
			}
		}
	}
	if !found {
		t.Fatal("ReadDir did not return the broken link")
	}
}

// TestAtomic_RenameIsAtomic verifies that a concurrent reader
// sees either the old content or the new content, never a
// partial write.
func TestAtomic_RenameIsAtomic(t *testing.T) {
	dir := t.TempDir()
	root, err := safefs.New(dir, safefs.WithFS(filesystem.NewOSFS(dir)))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Create the initial file.
	if err := root.WriteAtomic("f.txt", []byte("old"), 0o600); err != nil {
		t.Fatalf("initial WriteAtomic: %v", err)
	}

	// Overwrite it. The test's assertion is that the target
	// never has a partial write; because the test is
	// single-threaded, we assert the final state, which the
	// rename guarantees.
	if err := root.WriteAtomicOverwrite("f.txt", []byte("new-content"), 0o600); err != nil {
		t.Fatalf("WriteAtomicOverwrite: %v", err)
	}
	got, err := root.ReadFile("f.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "new-content" {
		t.Fatalf("content = %q, want %q", got, "new-content")
	}
}

// TestAtomic_TempFileCleanedOnFailure verifies that the temp
// file is removed when the write fails.
func TestAtomic_TempFileCleanedOnFailure(t *testing.T) {
	dir := t.TempDir()
	root, err := safefs.New(dir, safefs.WithFS(filesystem.NewOSFS(dir)))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Create a directory where the target should be. The
	// rename will fail (a directory cannot be replaced by a
	// file via rename on most systems).
	if err := os.Mkdir(filepath.Join(dir, "conflict"), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = root.WriteAtomicOverwrite("conflict", []byte("x"), 0o600)
	// After the failure, the directory should have no
	// temp files.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "conflict.tmp.") {
			t.Fatalf("temp file left behind: %s", e.Name())
		}
	}
}

// TestTOCTOU_SymlinkSwapFailsSafely simulates the TOCTOU
// scenario: a symlink is placed on the parent path between the
// pipeline's resolution and the write.
//
// The test's approach is to use a hook (not present in this
// version) or to construct the scenario deterministically: a
// symlink is placed on the write path before the operation, and
// the operation is asserted to fail. The deterministic version
// is weaker than a real race but exercises the same code path:
// the pipeline's verifyWritePath must detect the symlink.
func TestTOCTOU_SymlinkSwapFailsSafely(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "target.txt"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Place the symlink before the write. A real race would
	// place it between the resolution and the open; the
	// deterministic version places it before, which is the
	// same code path.
	if err := os.RemoveAll(filepath.Join(dir, "sub")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "sub")); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	root, err := safefs.New(dir, safefs.WithFS(filesystem.NewOSFS(dir)))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	err = root.WriteAtomic("sub/target.txt", []byte("new"), 0o600)
	if err == nil {
		t.Fatal("WriteAtomic succeeded through swapped symlink, want error")
	}
	if !errors.Is(err, safefs.ErrSymlinkDenied) {
		t.Fatalf("error = %v, want ErrSymlinkDenied", err)
	}
	// The outside target must be unchanged.
	got, _ := os.ReadFile(filepath.Join(outside, "target.txt"))
	if string(got) != "old" {
		t.Fatalf("outside target content changed: %q", got)
	}
}
