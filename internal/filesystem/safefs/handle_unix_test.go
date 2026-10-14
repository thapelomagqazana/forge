//go:build unix

package safefs_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/thapelomagqazana/forge/internal/filesystem"
	"github.com/thapelomagqazana/forge/internal/filesystem/safefs"
)

// TestHandle_SymlinkParentSwapRejected constructs the TOCTOU
// scenario the pipeline defends against: a directory named
// "sub" is replaced, between the lexical check and the syscall,
// by a symlink to a directory outside the handle's boundary.
//
// The test creates the directory, replaces it with a symlink
// before the operation, and asserts that the operation fails
// with ErrSymlinkDenied. The replacement is performed by the
// test's setup, not concurrently; the property under test is
// that the pipeline resolves the parent and refuses to proceed
// when the resolution escapes.
func TestHandle_SymlinkParentSwapRejected(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()

	if err := os.WriteFile(
		filepath.Join(outside, "secret.txt"),
		[]byte("sensitive"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Create the intended sub-directory and then replace it
	// with a symlink to the outside directory.
	sub := filepath.Join(dir, "src")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(sub); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, sub); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	root, err := safefs.New(dir,
		safefs.WithFS(filesystem.NewOSFS(dir)),
		safefs.WithSymlinkPolicy(safefs.NoFollow))
	if err != nil {
		t.Fatalf("safefs.New: %v", err)
	}

	// A handle at "src" now has a parent that resolves to
	// "outside". Constructing the handle must reject the
	// configuration.
	_, err = root.Rel("src")
	if err == nil {
		t.Fatal("Rel(src) succeeded despite symlinked parent, want error")
	}
	if !errors.Is(err, safefs.ErrSymlinkDenied) {
		t.Fatalf("error = %v, want wrapping ErrSymlinkDenied", err)
	}
}

// TestHandle_SymlinkParentUnderFollowInRoot asserts that under
// FollowInRoot, a symlink whose target stays inside the root is
// accepted by the handle construction, and a symlink whose
// target escapes is refused.
func TestHandle_SymlinkParentUnderFollowInRoot(t *testing.T) {
	dir := t.TempDir()
	inside := filepath.Join(dir, "real")
	outside := t.TempDir()

	if err := os.Mkdir(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(inside, "a.txt"),
		[]byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}

	linkInside := filepath.Join(dir, "link-inside")
	if err := os.Symlink("real", linkInside); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	linkOutside := filepath.Join(dir, "link-outside")
	if err := os.Symlink(outside, linkOutside); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	root, err := safefs.New(dir,
		safefs.WithFS(filesystem.NewOSFS(dir)),
		safefs.WithSymlinkPolicy(safefs.FollowInRoot))
	if err != nil {
		t.Fatalf("safefs.New: %v", err)
	}

	// In-root symlink: Rel must succeed.
	h, err := root.Rel("link-inside")
	if err != nil {
		t.Fatalf("Rel(link-inside): %v", err)
	}
	got, err := h.ReadFile("a.txt")
	if err != nil {
		t.Fatalf("ReadFile(a.txt): %v", err)
	}
	if string(got) != "a" {
		t.Fatalf("ReadFile = %q, want %q", got, "a")
	}

	// Escaping symlink: Rel must fail.
	_, err = root.Rel("link-outside")
	if err == nil {
		t.Fatal("Rel(link-outside) succeeded, want error")
	}
	if !errors.Is(err, safefs.ErrSymlinkDenied) {
		t.Fatalf("error = %v, want wrapping ErrSymlinkDenied", err)
	}
}
