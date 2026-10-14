package safefs_test

import (
	"errors"
	"io/fs"
	"sort"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/thapelomagqazana/forge/internal/filesystem"
	"github.com/thapelomagqazana/forge/internal/filesystem/safefs"
)

// newHandleRoot builds a Root backed by an in-memory filesystem
// and returns it. The tree is chosen to exercise sub-boundary
// checks without needing real disk files.
func newHandleRoot(t *testing.T, tree map[string]string) *safefs.Root {
	t.Helper()
	m := make(fstest.MapFS, len(tree))
	for k, v := range tree {
		m[k] = &fstest.MapFile{Data: []byte(v)}
	}
	root, err := safefs.New("/root", safefs.WithFS(filesystem.NewMemFS(m)))
	if err != nil {
		t.Fatalf("safefs.New: %v", err)
	}
	return root
}

// -----------------------------------------------------------------------------
// Positive — a handle's operations are equivalent to the parent's
// with the sub-path prepended.
// -----------------------------------------------------------------------------

// TestHandle_ReadFile asserts that a handle reads the same file
// the parent Root reads through the sub-path.
func TestHandle_ReadFile(t *testing.T) {
	root := newHandleRoot(t, map[string]string{
		"src/main.go": "package main",
	})
	h, err := root.Rel("src")
	if err != nil {
		t.Fatalf("Rel: %v", err)
	}
	got, err := h.ReadFile("main.go")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "package main" {
		t.Fatalf("ReadFile = %q, want %q", got, "package main")
	}
}

// TestHandle_ReadDir asserts that a handle lists the entries of
// its sub-directory.
func TestHandle_ReadDir(t *testing.T) {
	root := newHandleRoot(t, map[string]string{
		"src/a.go": "a",
		"src/b.go": "b",
	})
	h, _ := root.Rel("src")
	entries, err := h.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	got := names(entries)
	sort.Strings(got)
	want := []string{"a.go", "b.go"}
	if !equal(got, want) {
		t.Fatalf("entries = %v, want %v", got, want)
	}
}

// TestHandle_Walk asserts that Walk on a handle visits the
// subtree under the handle's sub-path.
func TestHandle_Walk(t *testing.T) {
	root := newHandleRoot(t, map[string]string{
		"src/a.go":     "a",
		"src/b.go":     "b",
		"src/sub/c.go": "c",
		"other/d.go":   "d",
	})
	h, _ := root.Rel("src")
	var seen []string
	err := h.Walk(".", func(rel string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		seen = append(seen, rel)
		return nil
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	sort.Strings(seen)
	// The handle's walk is rooted at "src", so the entries
	// it sees are relative to the handle.
	want := []string{"a.go", "b.go", "sub", "sub/c.go"}
	if !equal(seen, want) {
		t.Fatalf("visited = %v, want %v", seen, want)
	}
}

// -----------------------------------------------------------------------------
// Negative — sub-boundary violations are rejected.
// -----------------------------------------------------------------------------

// TestHandle_RejectsParentTraversal asserts that a name that
// escapes the sub-boundary is rejected even though it would be
// legal under the parent root.
func TestHandle_RejectsParentTraversal(t *testing.T) {
	root := newHandleRoot(t, map[string]string{
		"src/main.go":   "package main",
		"other/main.go": "other",
	})
	h, _ := root.Rel("src")

	// "../other/main.go" is lexically legal under the root:
	// after cleaning it is "../other/main.go", which fails
	// IsLocal and is rejected by the pipeline as a traversal.
	// The rejection is therefore ErrPathTraversal, not
	// ErrSubBoundary.
	_, err := h.ReadFile("../other/main.go")
	if err == nil {
		t.Fatal("ReadFile succeeded, want error")
	}
	if !errors.Is(err, safefs.ErrPathTraversal) {
		t.Fatalf("error = %v, want wrapping ErrPathTraversal", err)
	}
}

// TestHandle_RejectsSiblingViaCleaning asserts that a name that
// lexically stays inside the sub-path after cleaning but whose
// pre-clean form contained a sibling reference is either
// accepted (if it cleans to a name inside the sub-path) or
// rejected as traversal (if it does not).
//
// The test's point is that the sub-boundary check is applied to
// the *resolved* name, not to the raw input, so a name like
// "a/../b" is equivalent to "b" and is accepted if "b" is
// inside the sub-path.
func TestHandle_RejectsSiblingViaCleaning(t *testing.T) {
	root := newHandleRoot(t, map[string]string{
		"src/a.go": "a",
		"src/b.go": "b",
	})
	h, _ := root.Rel("src")

	// "a/../b" cleans to "b", which is inside the handle.
	// It must be accepted.
	got, err := h.ReadFile("a/../b.go")
	if err != nil {
		t.Fatalf("ReadFile(a/../b.go): %v", err)
	}
	if string(got) != "b" {
		t.Fatalf("ReadFile = %q, want %q", got, "b")
	}

	// "a/../../b" cleans to "../b", which escapes the root.
	// It must be rejected as traversal.
	_, err = h.ReadFile("a/../../b.go")
	if err == nil {
		t.Fatal("ReadFile(a/../../b.go) succeeded, want error")
	}
	if !errors.Is(err, safefs.ErrPathTraversal) {
		t.Fatalf("error = %v, want wrapping ErrPathTraversal", err)
	}
}

// TestHandle_RejectsAbsolutePath asserts that an absolute name
// is rejected at the root's pipeline, before the sub-boundary
// check runs.
func TestHandle_RejectsAbsolutePath(t *testing.T) {
	root := newHandleRoot(t, nil)
	h, _ := root.Rel("src")
	_, err := h.ReadFile("/etc/passwd")
	if err == nil {
		t.Fatal("ReadFile succeeded, want error")
	}
	if !errors.Is(err, safefs.ErrNotLocal) {
		t.Fatalf("error = %v, want wrapping ErrNotLocal", err)
	}
}

// TestHandle_RejectsNameAtSubBoundaryAboveRoot verifies that a
// name that would be legal under the root but resolves to a
// sibling of the handle's sub-path is rejected with
// ErrSubBoundary.
//
// The test uses a symlink on the parent path to force the
// resolution; without a symlink, the name would be rejected
// lexically.
func TestHandle_RejectsNameAtSubBoundaryAboveRoot(t *testing.T) {
	// The in-memory FS does not model symlinks, so this test
	// cannot construct the scenario it wants. It is retained
	// as a placeholder for the real-disk test in
	// handle_unix_test.go, which exercises the same property
	// with a real symlink.
	t.Skip("symlink resolution requires a real filesystem; see handle_unix_test.go")
}

// -----------------------------------------------------------------------------
// Nested handles
// -----------------------------------------------------------------------------

// TestNestedHandle asserts that a handle-of-a-handle enforces
// both boundaries.
func TestNestedHandle(t *testing.T) {
	root := newHandleRoot(t, map[string]string{
		"src/internal/a.go":     "a",
		"src/internal/sub/b.go": "b",
		"src/other.go":          "other",
	})
	src, _ := root.Rel("src")
	internal, err := src.Rel("internal")
	if err != nil {
		t.Fatalf("nested Rel: %v", err)
	}
	if got, err := internal.ReadFile("a.go"); err != nil {
		t.Fatalf("ReadFile(a.go): %v", err)
	} else if string(got) != "a" {
		t.Fatalf("ReadFile = %q, want %q", got, "a")
	}

	// A name that would be legal under "src" but not under
	// "src/internal" is rejected.
	_, err = internal.ReadFile("../other.go")
	if err == nil {
		t.Fatal("ReadFile(../other.go) succeeded, want error")
	}
	if !errors.Is(err, safefs.ErrPathTraversal) {
		t.Fatalf("error = %v, want wrapping ErrPathTraversal", err)
	}
}

// TestNestedHandle_EmptySub asserts that Rel(".") returns the
// same handle, and that the handle's operations are unchanged.
func TestNestedHandle_EmptySub(t *testing.T) {
	root := newHandleRoot(t, map[string]string{"a.go": "a"})
	h, _ := root.Rel(".")
	got, err := h.ReadFile("a.go")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "a" {
		t.Fatalf("ReadFile = %q, want %q", got, "a")
	}
}

// -----------------------------------------------------------------------------
// Path()
// -----------------------------------------------------------------------------

// TestRoot_Path_IsInformational asserts that Root.Path returns
// the canonical root path and that a handle's Path joins the
// sub-path.
func TestRoot_Path_IsInformational(t *testing.T) {
	root := newHandleRoot(t, nil)
	if root.Path() == "" {
		t.Fatal("Root.Path() is empty")
	}
	h, _ := root.Rel("src")
	wantSuffix := "src"
	if got := h.Path(); !strings.HasSuffix(got, wantSuffix) {
		t.Fatalf("Handle.Path() = %q, want suffix %q", got, wantSuffix)
	}
}

// TestRoot_Abs_Deprecated asserts that Root.Abs is an alias for
// Root.Path. The deprecation is documented; the test pins the
// alias so that a future removal is a deliberate change.
func TestRoot_Abs_Deprecated(t *testing.T) {
	root := newHandleRoot(t, nil)
	if root.Abs() != root.Path() {
		t.Fatalf("Abs() = %q, Path() = %q; want equal",
			root.Abs(), root.Path())
	}
}
