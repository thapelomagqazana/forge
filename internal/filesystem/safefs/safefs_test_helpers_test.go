package safefs_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"testing/fstest"

	"github.com/thapelomagqazana/forge/internal/filesystem"
	"github.com/thapelomagqazana/forge/internal/filesystem/safefs"
)

// This file contains the helpers that more than one test file in
// the safefs package's external test suite uses. It is named
// *_test.go so the Go build system includes it in the test binary
// and makes its symbols visible to the other test files in
// package safefs_test.
//
// The file exists because three files — root_test.go, walk_test.go,
// and handle_test.go — each need a small set of helpers that
// operate on fs.DirEntry slices and on the safefs.Root
// construction pattern. Duplicating the helpers per file was the
// alternative; consolidating them here is cheaper, and the
// package's test-suite lint rule discourages copy-paste helpers.
//
// # What belongs here
//
// A helper belongs here if and only if more than one test file
// uses it. A helper used by a single file lives in that file. The
// rule keeps this file small and its purpose clear.
//
// # What does NOT belong here
//
// Test functions (TestXxx, BenchmarkXxx, FuzzXxx) do not belong
// here; they live in the file they test. This file is helpers
// only.

// names returns the base names of entries, in order.
//
// It is the idiomatic way for a test to inspect a
// []fs.DirEntry without importing io/fs at the assertion site.
// The helper does not sort; callers that need a sorted list sort
// the result themselves, which makes the sort visible at the
// call site.
func names(entries []fs.DirEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Name()
	}
	return out
}

// equal reports whether two string slices are equal
// element-by-element.
//
// It is a simple length-and-membership check; it does not attempt
// to be generic over element types. The tests that need it compare
// []string values, and a generic version would add a type
// parameter for no gain.
func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// newMemRoot constructs a Root backed by an in-memory filesystem
// populated with the given tree. Keys use forward slashes.
// Directories are implicit: an entry "a/b.txt" creates "a" as a
// directory automatically.
//
// The in-memory filesystem is filesystem.MemFS, the adapter that
// bridges fstest.MapFS to the filesystem.FS interface. The root
// path "/root" is informational; MemFS resolves every name in
// its own flat namespace and ignores the prefix.
//
// The helper is used by root_test.go, walk_test.go, and
// handle_test.go. The three files' fixtures differ but the
// construction is identical, which is why the helper lives here.
func newMemRoot(t *testing.T, tree map[string]string, opts ...safefs.Option) *safefs.Root {
	t.Helper()
	m := make(fstest.MapFS, len(tree))
	for k, v := range tree {
		m[k] = &fstest.MapFile{Data: []byte(v)}
	}
	all := append([]safefs.Option{
		safefs.WithFS(filesystem.NewMemFS(m)),
	}, opts...)
	root, err := safefs.New("/root", all...)
	if err != nil {
		t.Fatalf("safefs.New: %v", err)
	}
	return root
}

// newOSRoot constructs a Root backed by a real filesystem rooted
// at dir. It is the counterpart to newMemRoot, and it is the only
// way to test symlink behaviour, because the in-memory filesystem
// does not represent symlinks.
//
// The helper lives here for the same reason as newMemRoot: the
// construction is identical across the files that need it, and
// the difference between the two constructors is the FS the Root
// is backed by.
func newOSRoot(t *testing.T, dir string, opts ...safefs.Option) *safefs.Root {
	t.Helper()
	all := append([]safefs.Option{
		safefs.WithFS(filesystem.NewOSFS(dir)),
	}, opts...)
	root, err := safefs.New(dir, all...)
	if err != nil {
		t.Fatalf("safefs.New: %v", err)
	}
	return root
}

// symlink creates a symlink from linkPath to target and skips the
// test if the host does not support symlinks. The skip is
// deliberate: a skipped test is a statement that the host cannot
// express the input, not a silent pass.
//
// The helper lives here because three files — symlink_unix_test.go,
// symlink_windows_test.go, and handle_unix_test.go — all need to
// create a symlink with the same skip behaviour.
func symlink(t *testing.T, target, linkPath string) {
	t.Helper()
	if err := os.Symlink(target, linkPath); err != nil {
		t.Skipf("symlinks not supported on this host: %v", err)
	}
}

// writeFile creates a regular file at path with the given content.
//
// The helper lives here because several files create fixture
// files and the four lines of error handling are identical each
// time.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// itoa is a minimal int-to-string used by the symlink chain tests
// to build link names. It exists so those tests do not import
// strconv for a helper whose only job is to produce "l0", "l1",
// "l2", ...
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// containsBackslash reports whether s contains a backslash.
//
// The helper is used by the tests that assert on the forward-slash
// contract of Walk's relPath and of the FS interface's names.
func containsBackslash(s string) bool {
	for _, r := range s {
		if r == '\\' {
			return true
		}
	}
	return false
}

// fsOnlyFS is a minimal adapter that exposes a single io/fs.File
// as an fs.FS rooted at a fake name.
//
// It is used by the filesystem package's tests, not by the safefs
// tests. It is defined here as a placeholder so that a future
// refactor that consolidates helpers can find it; if no safefs
// test uses it, remove it.
//
// The type is unused today. Delete this block before merge if the
// package's test suite does not need it.
type fsOnlyFS struct {
	fh fs.File
}

func (f fsOnlyFS) Open(string) (fs.File, error) { return f.fh, nil }

// Ensure the helpers compile against the types they take. A
// future change to io/fs or to safefs that breaks a helper's
// signature fails here rather than at a distant call site.
var (
	_ func([]fs.DirEntry) []string                                       = names
	_ func([]string, []string) bool                                      = equal
	_ func(*testing.T, map[string]string, ...safefs.Option) *safefs.Root = newMemRoot
	_ func(*testing.T, string, ...safefs.Option) *safefs.Root            = newOSRoot
)

// Silence unused-import warnings for imports that are used only
// by a subset of the helpers above. If every import is used by at
// least one helper, this block can be removed.
var (
	_ = sort.Strings
	_ = filepath.Join
)
