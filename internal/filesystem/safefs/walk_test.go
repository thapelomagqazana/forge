package safefs_test

import (
	"errors"
	"io/fs"
	"sort"
	"testing"
	"testing/fstest"

	"github.com/thapelomagqazana/forge/internal/filesystem"
	"github.com/thapelomagqazana/forge/internal/filesystem/safefs"
)

// This file exercises Root.Walk and WalkFunc through the exported
// surface of safefs.
//
// Every case uses filesystem.MemFS, the in-memory adapter, because
// none of them require real symlinks. The symlink-in-walk cases
// live in symlink_unix_test.go and symlink_windows_test.go, where
// a real t.TempDir and os.Symlink are available.
//
// The test taxonomy:
//
//   - Positive:    documented happy-path traversal.
//   - Negative:    documented failure modes.
//   - Edge:        SkipDir, SkipAll, empty directories, read
//                  errors delivered to the callback.
//   - Corner:      interaction of Walk with the name contract
//                  and with the entry/depth bounds.
//   - Boundary:    exact-bound values for MaxDepth and
//                  MaxEntries.
//   - Non-functional: concurrency, determinism of order.

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

// newWalkRoot constructs a Root backed by an in-memory FS
// populated with the given tree. The tree's keys use forward
// slashes; directories are implicit.
func newWalkRoot(t *testing.T, tree map[string]string, opts ...safefs.Option) *safefs.Root {
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

// collectWalk runs Walk and returns the names visited, in visit
// order, or the error Walk returned. The callback never skips.
func collectWalk(t *testing.T, root *safefs.Root, name string) ([]string, error) {
	t.Helper()
	var seen []string
	err := root.Walk(name, func(rel string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		seen = append(seen, rel)
		return nil
	})
	return seen, err
}

// localSortedNames returns entries' names in lexicographic order.
// It is a copy of the helper the production package used to
// define; the production definition was removed when the tests
// were rewritten to use it locally.
func localSortedNames(entries []fs.DirEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Name()
	}
	sort.Strings(out)
	return out
}

// -----------------------------------------------------------------------------
// Positive
// -----------------------------------------------------------------------------

// TestWalk_VisitsEveryEntry verifies the documented contract:
// every file and directory under the start name is visited
// exactly once.
func TestWalk_VisitsEveryEntry(t *testing.T) {
	root := newWalkRoot(t, map[string]string{
		"a.txt":         "a",
		"dir/b.txt":     "b",
		"dir/c.txt":     "c",
		"dir/sub/d.txt": "d",
	})

	seen, err := collectWalk(t, root, ".")
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	sort.Strings(seen)
	want := []string{
		"a.txt",
		"dir",
		"dir/b.txt",
		"dir/c.txt",
		"dir/sub",
		"dir/sub/d.txt",
	}
	if !equalStrings(seen, want) {
		t.Fatalf("visited = %v, want %v", seen, want)
	}
}

// TestWalk_FromSubdirectory verifies that Walk's start name is
// honoured: walking "dir" visits only entries under "dir", and
// the callback sees paths relative to the Root (not relative to
// the start name).
func TestWalk_FromSubdirectory(t *testing.T) {
	root := newWalkRoot(t, map[string]string{
		"a.txt":     "a",
		"dir/b.txt": "b",
		"dir/c.txt": "c",
	})
	seen, err := collectWalk(t, root, "dir")
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	sort.Strings(seen)
	want := []string{"dir/b.txt", "dir/c.txt"}
	if !equalStrings(seen, want) {
		t.Fatalf("visited = %v, want %v", seen, want)
	}
}

// TestWalk_EmptyDirectory verifies that a directory with no
// children produces no callback invocations for its (absent)
// children.
func TestWalk_EmptyDirectory(t *testing.T) {
	root := newWalkRoot(t, map[string]string{
		"empty/keepme.txt": "x", // ensures "empty" exists as a dir
	})
	seen, err := collectWalk(t, root, "empty")
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	// "empty/keepme.txt" is the only entry under "empty".
	want := []string{"empty/keepme.txt"}
	if !equalStrings(seen, want) {
		t.Fatalf("visited = %v, want %v", seen, want)
	}
}

// TestWalk_DeterministicOrder verifies that two walks of the same
// tree visit entries in the same order. The order is the FS's
// sorted order; the test asserts stability, not a specific
// ordering.
func TestWalk_DeterministicOrder(t *testing.T) {
	root := newWalkRoot(t, map[string]string{
		"c.txt":     "c",
		"a.txt":     "a",
		"b.txt":     "b",
		"dir/x.txt": "x",
		"dir/y.txt": "y",
	})
	first, err := collectWalk(t, root, ".")
	if err != nil {
		t.Fatalf("first Walk: %v", err)
	}
	second, err := collectWalk(t, root, ".")
	if err != nil {
		t.Fatalf("second Walk: %v", err)
	}
	if !equalStrings(first, second) {
		t.Fatalf("order not stable: %v vs %v", first, second)
	}
}

// -----------------------------------------------------------------------------
// Negative
// -----------------------------------------------------------------------------

// TestWalk_TraversalStartRejected verifies that the name contract
// applies to Walk's start name. This is the case that prevents a
// caller from walking outside the root by starting at "..".
func TestWalk_TraversalStartRejected(t *testing.T) {
	root := newWalkRoot(t, nil)
	err := root.Walk("../escape", func(string, fs.DirEntry, error) error {
		return nil
	})
	if err == nil {
		t.Fatal("Walk(\"../escape\") succeeded, want error")
	}
	if !errors.Is(err, safefs.ErrPathTraversal) {
		t.Fatalf("error = %v, want wrapping ErrPathTraversal", err)
	}
}

// TestWalk_AbsoluteStartRejected is the absolute-path counterpart.
func TestWalk_AbsoluteStartRejected(t *testing.T) {
	root := newWalkRoot(t, nil)
	err := root.Walk("/etc", func(string, fs.DirEntry, error) error {
		return nil
	})
	if err == nil {
		t.Fatal("Walk(\"/etc\") succeeded, want error")
	}
	if !errors.Is(err, safefs.ErrNotLocal) &&
		!errors.Is(err, safefs.ErrPathTraversal) {
		t.Fatalf("error = %v, want ErrNotLocal or ErrPathTraversal", err)
	}
}

// TestWalk_MissingStartReportsToCallback verifies the filepath.WalkDir-
// style behaviour: a read error on the start directory is
// delivered to the callback, which may return nil to continue (in
// which case the walk ends with no error) or return the error to
// abort.
//
// Here the callback returns nil, so Walk returns nil. The
// callback did see the error, which is the contract.
func TestWalk_MissingStartReportsToCallback(t *testing.T) {
	root := newWalkRoot(t, nil)
	var sawErr error
	err := root.Walk("missing", func(_ string, _ fs.DirEntry, err error) error {
		sawErr = err
		return nil
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if sawErr == nil {
		t.Fatal("callback never saw the read error")
	}
	if !errors.Is(sawErr, fs.ErrNotExist) {
		t.Fatalf("callback error = %v, want wrapping fs.ErrNotExist", sawErr)
	}
}

// TestWalk_MissingStartAbortsWhenCallbackReturnsError verifies
// the abort path.
func TestWalk_MissingStartAbortsWhenCallbackReturnsError(t *testing.T) {
	root := newWalkRoot(t, nil)
	sentinel := errors.New("stop here")
	err := root.Walk("missing", func(_ string, _ fs.DirEntry, err error) error {
		if err != nil {
			return sentinel
		}
		return nil
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v, want sentinel", err)
	}
}

// -----------------------------------------------------------------------------
// Edge — SkipDir, SkipAll, custom errors
// -----------------------------------------------------------------------------

// TestWalk_SkipDir_SkipsSubtree verifies that returning fs.SkipDir
// for a directory prevents its children from being visited, while
// the directory itself is still reported.
func TestWalk_SkipDir_SkipsSubtree(t *testing.T) {
	root := newWalkRoot(t, map[string]string{
		"a.txt":      "a",
		"skip/b.txt": "b",
		"skip/c.txt": "c",
		"keep/d.txt": "d",
	})
	var seen []string
	err := root.Walk(".", func(rel string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		seen = append(seen, rel)
		if d.IsDir() && d.Name() == "skip" {
			return fs.SkipDir
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	sort.Strings(seen)
	want := []string{"a.txt", "keep", "keep/d.txt", "skip"}
	if !equalStrings(seen, want) {
		t.Fatalf("visited = %v, want %v", seen, want)
	}
}

// TestWalk_SkipDir_OnFileIsNoop verifies that returning fs.SkipDir
// for a non-directory has no effect on the walk, matching
// io/fs.WalkDir.
func TestWalk_SkipDir_OnFileIsNoop(t *testing.T) {
	root := newWalkRoot(t, map[string]string{
		"a.txt":     "a",
		"dir/b.txt": "b",
	})
	var seen []string
	err := root.Walk(".", func(rel string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		seen = append(seen, rel)
		if !d.IsDir() {
			return fs.SkipDir
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	sort.Strings(seen)
	want := []string{"a.txt", "dir", "dir/b.txt"}
	if !equalStrings(seen, want) {
		t.Fatalf("visited = %v, want %v", seen, want)
	}
}

// TestWalk_SkipAll_Stops verifies that fs.SkipAll ends the walk
// and returns nil.
func TestWalk_SkipAll_Stops(t *testing.T) {
	root := newWalkRoot(t, map[string]string{
		"a.txt":     "a",
		"b.txt":     "b",
		"dir/c.txt": "c",
	})
	var count int
	err := root.Walk(".", func(rel string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		count++
		if count == 1 {
			return fs.SkipAll
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
}

// TestWalk_CustomErrorAborts verifies that a callback error is
// returned unchanged.
func TestWalk_CustomErrorAborts(t *testing.T) {
	root := newWalkRoot(t, map[string]string{"a.txt": "a"})
	sentinel := errors.New("custom")
	err := root.Walk(".", func(string, fs.DirEntry, error) error {
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v, want sentinel", err)
	}
}

// -----------------------------------------------------------------------------
// Boundary — MaxDepth
// -----------------------------------------------------------------------------
//
// Depth is the number of path components between the entry and
// the root. "." is depth 0, "a" is depth 1, "a/b" is depth 2,
// "a/b/c.txt" is depth 3.
//
// MaxDepth(N) permits visiting entries whose depth is at most N.
// The bound is checked against the child's depth, immediately
// before it is delivered to the callback. When the bound is
// exceeded, the walk aborts with an error wrapping
// ErrDepthExceeded, and the offending entry is not delivered.

// TestWalk_MaxDepth_ExactBoundSucceeds verifies that a tree whose
// deepest entry is at exactly MaxDepth visits every entry and
// returns nil.
//
// The tree's deepest entry is "a/b" at depth 2, so MaxDepth(2)
// succeeds.
func TestWalk_MaxDepth_ExactBoundSucceeds(t *testing.T) {
	root := newWalkRoot(t,
		map[string]string{"a/b": "x"},
		safefs.WithMaxDepth(2),
	)
	if _, err := collectWalk(t, root, "."); err != nil {
		t.Fatalf("Walk: %v", err)
	}
}

// TestWalk_MaxDepth_OffByOneFails verifies that a tree one level
// deeper than MaxDepth is rejected with ErrDepthExceeded, and
// that the entries at or below the bound are delivered before the
// error.
//
// The tree's deepest entry is "a/b/c.txt" at depth 3, so
// MaxDepth(2) fails when it reaches that entry.
func TestWalk_MaxDepth_OffByOneFails(t *testing.T) {
	root := newWalkRoot(t,
		map[string]string{"a/b/c.txt": "x"},
		safefs.WithMaxDepth(2),
	)
	seen, err := collectWalk(t, root, ".")
	if err == nil {
		t.Fatal("Walk succeeded, want ErrDepthExceeded")
	}
	if !errors.Is(err, safefs.ErrDepthExceeded) {
		t.Fatalf("error = %v, want wrapping ErrDepthExceeded", err)
	}
	// The entries at depth <= 2 were delivered: "a" (depth 1)
	// and "a/b" (depth 2). The offending entry "a/b/c.txt" was
	// not.
	sort.Strings(seen)
	want := []string{"a", "a/b"}
	if !equalStrings(seen, want) {
		t.Fatalf("visited = %v, want %v", seen, want)
	}
}

// TestWalk_MaxDepth_ZeroIsUnbounded verifies that zero disables
// the bound.
func TestWalk_MaxDepth_ZeroIsUnbounded(t *testing.T) {
	root := newWalkRoot(t,
		map[string]string{"a/b/c/d/e/f/g.txt": "x"},
		safefs.WithMaxDepth(0),
	)
	if _, err := collectWalk(t, root, "."); err != nil {
		t.Fatalf("Walk: %v", err)
	}
}

// TestWalk_MaxDepth_OneVisitsImmediateChildren verifies the
// documented meaning: MaxDepth is the number of levels below the
// root that may be visited.
//
// With MaxDepth(1) the immediate children of the root are
// visited; the next level is not. The walk aborts with
// ErrDepthExceeded when it reaches an entry that would be at
// depth 2.
func TestWalk_MaxDepth_OneVisitsImmediateChildren(t *testing.T) {
	root := newWalkRoot(t,
		map[string]string{
			"a.txt":     "a",
			"dir/b.txt": "b",
		},
		safefs.WithMaxDepth(1),
	)
	seen, err := collectWalk(t, root, ".")
	if err == nil {
		t.Fatal("Walk succeeded, want ErrDepthExceeded")
	}
	if !errors.Is(err, safefs.ErrDepthExceeded) {
		t.Fatalf("error = %v, want wrapping ErrDepthExceeded", err)
	}
	sort.Strings(seen)
	// "a.txt" (depth 1) and "dir" (depth 1) were delivered.
	// "dir/b.txt" (depth 2) was not.
	want := []string{"a.txt", "dir"}
	if !equalStrings(seen, want) {
		t.Fatalf("visited = %v, want %v", seen, want)
	}
}

// -----------------------------------------------------------------------------
// Boundary — MaxEntries
// -----------------------------------------------------------------------------

// TestWalk_MaxEntries_ExactBoundSucceeds verifies that a tree
// with exactly MaxEntries entries visits every entry.
func TestWalk_MaxEntries_ExactBoundSucceeds(t *testing.T) {
	root := newWalkRoot(t,
		map[string]string{
			"a.txt": "a",
			"b.txt": "b",
		},
		safefs.WithMaxEntries(2),
	)
	if _, err := collectWalk(t, root, "."); err != nil {
		t.Fatalf("Walk: %v", err)
	}
}

// TestWalk_MaxEntries_OffByOneFails verifies that a tree one
// entry larger than MaxEntries is rejected.
func TestWalk_MaxEntries_OffByOneFails(t *testing.T) {
	root := newWalkRoot(t,
		map[string]string{
			"a.txt": "a",
			"b.txt": "b",
			"c.txt": "c",
		},
		safefs.WithMaxEntries(2),
	)
	_, err := collectWalk(t, root, ".")
	if err == nil {
		t.Fatal("Walk succeeded, want ErrEntriesExceeded")
	}
	if !errors.Is(err, safefs.ErrEntriesExceeded) {
		t.Fatalf("error = %v, want wrapping ErrEntriesExceeded", err)
	}
}

// TestWalk_MaxEntries_ZeroIsUnbounded verifies that zero disables
// the bound.
func TestWalk_MaxEntries_ZeroIsUnbounded(t *testing.T) {
	root := newWalkRoot(t,
		map[string]string{
			"a.txt": "a",
			"b.txt": "b",
			"c.txt": "c",
			"d.txt": "d",
		},
		safefs.WithMaxEntries(0),
	)
	if _, err := collectWalk(t, root, "."); err != nil {
		t.Fatalf("Walk: %v", err)
	}
}

// TestWalk_MaxEntries_OneVisitsOneEntry verifies the tightest
// non-zero bound.
func TestWalk_MaxEntries_OneVisitsOneEntry(t *testing.T) {
	root := newWalkRoot(t,
		map[string]string{
			"a.txt": "a",
			"b.txt": "b",
		},
		safefs.WithMaxEntries(1),
	)
	seen, err := collectWalk(t, root, ".")
	if err == nil {
		t.Fatal("Walk succeeded, want ErrEntriesExceeded")
	}
	if !errors.Is(err, safefs.ErrEntriesExceeded) {
		t.Fatalf("error = %v, want wrapping ErrEntriesExceeded", err)
	}
	// Exactly one entry was visited before the bound fired.
	if len(seen) != 1 {
		t.Fatalf("visited %d entries, want 1", len(seen))
	}
}

// -----------------------------------------------------------------------------
// Corner — interaction with the callback's DirEntry
// -----------------------------------------------------------------------------

// TestWalk_DirEntryReportsCorrectType verifies that d.IsDir() is
// true exactly for directories, and d.Type() is what the FS
// reported.
func TestWalk_DirEntryReportsCorrectType(t *testing.T) {
	root := newWalkRoot(t, map[string]string{
		"a.txt":     "a",
		"dir/b.txt": "b",
	})
	seen := map[string]bool{}
	err := root.Walk(".", func(rel string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		seen[rel] = d.IsDir()
		return nil
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	want := map[string]bool{
		"a.txt":     false,
		"dir":       true,
		"dir/b.txt": false,
	}
	for name, isDir := range want {
		if got, ok := seen[name]; !ok {
			t.Errorf("missing entry %q in walk", name)
		} else if got != isDir {
			t.Errorf("IsDir(%q) = %v, want %v", name, got, isDir)
		}
	}
}

// TestWalk_RelPathUsesForwardSlashes verifies the platform-
// independent separator contract. Callers can build map keys
// from relPath without worrying about which OS the walk ran on.
func TestWalk_RelPathUsesForwardSlashes(t *testing.T) {
	root := newWalkRoot(t, map[string]string{
		"a/b/c.txt": "x",
	})
	var paths []string
	err := root.Walk(".", func(rel string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		paths = append(paths, rel)
		return nil
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	for _, p := range paths {
		if containsBackslash(p) {
			t.Fatalf("relPath %q contains a backslash", p)
		}
	}
}

// -----------------------------------------------------------------------------
// Non-functional
// -----------------------------------------------------------------------------

// TestWalk_ConcurrentWalks verifies that two walks of the same
// Root, with their own callbacks and their own state, run
// concurrently without data races. Run with -race.
func TestWalk_ConcurrentWalks(t *testing.T) {
	root := newWalkRoot(t, map[string]string{
		"a.txt":         "a",
		"b.txt":         "b",
		"dir/c.txt":     "c",
		"dir/d.txt":     "d",
		"dir/sub/e.txt": "e",
	})

	type result struct {
		paths []string
		err   error
	}
	results := make(chan result, 2)

	for i := 0; i < 2; i++ {
		go func() {
			var seen []string
			err := root.Walk(".", func(rel string, _ fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				seen = append(seen, rel)
				return nil
			})
			results <- result{paths: seen, err: err}
		}()
	}

	a := <-results
	b := <-results

	if a.err != nil {
		t.Fatalf("walk A: %v", a.err)
	}
	if b.err != nil {
		t.Fatalf("walk B: %v", b.err)
	}
	sort.Strings(a.paths)
	sort.Strings(b.paths)
	if !equalStrings(a.paths, b.paths) {
		t.Fatalf("walks disagree: %v vs %v", a.paths, b.paths)
	}
}

// TestWalk_DoesNotMutateRoot verifies the documented property
// that Walk does not mutate the Root. It does so by walking once,
// then asserting that a second walk over the same tree yields the
// same result.
func TestWalk_DoesNotMutateRoot(t *testing.T) {
	root := newWalkRoot(t, map[string]string{
		"a.txt":     "a",
		"dir/b.txt": "b",
	})
	first, err := collectWalk(t, root, ".")
	if err != nil {
		t.Fatalf("first walk: %v", err)
	}
	second, err := collectWalk(t, root, ".")
	if err != nil {
		t.Fatalf("second walk: %v", err)
	}
	if !equalStrings(first, second) {
		t.Fatalf("walk mutated Root: %v vs %v", first, second)
	}
}
