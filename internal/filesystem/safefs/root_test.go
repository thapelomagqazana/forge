package safefs_test

import (
	"errors"
	"io/fs"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/thapelomagqazana/forge/internal/filesystem"
	"github.com/thapelomagqazana/forge/internal/filesystem/safefs"
)

// This file exercises the Root contract through its exported
// surface: New, Abs, Policy, Open, Stat, ReadFile, ReadDir, Walk.
//
// It uses an in-memory filesystem injected via safefs.WithFS for
// every case that does not require a real disk. The symlink suite
// lives in symlink_unix_test.go and symlink_windows_test.go
// because an in-memory filesystem does not represent symlinks as
// first-class entries.
//
// The test taxonomy this file follows:
//
//   - Positive:    the documented happy path succeeds.
//   - Negative:    the documented failure modes fail with the
//                  documented error.
//   - Edge:        inputs that are legal but unusual.
//   - Corner:      inputs that are legal but interact with
//                  another feature.
//   - Boundary:    values at the exact edge of a bound.
//   - Non-functional: concurrency, fuzz, race.

// -----------------------------------------------------------------------------
// New
// -----------------------------------------------------------------------------

// TestNew_AcceptsAbsolutePath is the canonical positive case for
// construction: an absolute path is the normal input from a caller
// that has already resolved the project root.
func TestNew_AcceptsAbsolutePath(t *testing.T) {
	root, err := safefs.New("/tmp/project",
		safefs.WithFS(filesystem.NewMemFS(nil)))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := root.Abs(); got == "" {
		t.Fatal("Abs() returned empty")
	}
}

// TestNew_ConvertsRelativePathToAbsolute verifies that New
// normalises a relative path rather than rejecting it. A caller
// that passes "." expects the root to be the process CWD.
func TestNew_ConvertsRelativePathToAbsolute(t *testing.T) {
	root, err := safefs.New(".",
		safefs.WithFS(filesystem.NewMemFS(nil)))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !strings.HasPrefix(root.Abs(), "/") &&
		!strings.Contains(root.Abs(), ":") {
		// On Unix the absolute path starts with "/"; on
		// Windows it starts with a drive letter and a colon.
		// Either form is acceptable.
		t.Fatalf("Abs() = %q, want an absolute path", root.Abs())
	}
}

// TestNew_RejectsEmptyPath is the negative counterpart: an empty
// string cannot be a root.
func TestNew_RejectsEmptyPath(t *testing.T) {
	_, err := safefs.New("")
	if err == nil {
		t.Fatal("New(\"\") succeeded, want error")
	}
	if !errors.Is(err, safefs.ErrNotLocal) {
		t.Fatalf("error = %v, want wrapping ErrNotLocal", err)
	}
}

// TestNew_DefaultPolicyIsNoFollow documents the default. A caller
// that does not configure a policy gets the safest one.
func TestNew_DefaultPolicyIsNoFollow(t *testing.T) {
	root, err := safefs.New("/tmp/project",
		safefs.WithFS(filesystem.NewMemFS(nil)))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := root.Policy(); got != safefs.NoFollow {
		t.Fatalf("Policy() = %v, want NoFollow", got)
	}
}

// TestNew_NilOptionIsSkipped verifies that a nil Option in the
// list does not panic. This is an edge case: callers that build
// an option slice conditionally can produce nil entries.
func TestNew_NilOptionIsSkipped(t *testing.T) {
	root, err := safefs.New("/tmp/project",
		safefs.WithFS(filesystem.NewMemFS(nil)),
		nil,
		safefs.WithMaxDepth(1),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if root == nil {
		t.Fatal("New returned nil Root without error")
	}
}

// TestNew_RejectsInvalidOptions verifies that New surfaces option
// validation errors at construction rather than at first use.
func TestNew_RejectsInvalidOptions(t *testing.T) {
	cases := []struct {
		name string
		opt  safefs.Option
	}{
		{"negative max depth", safefs.WithMaxDepth(-1)},
		{"negative max entries", safefs.WithMaxEntries(-1)},
		{"negative max symlink chain", safefs.WithMaxSymlinkChain(-1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := safefs.New("/tmp/project",
				safefs.WithFS(filesystem.NewMemFS(nil)),
				tc.opt,
			)
			if err == nil {
				t.Fatal("New succeeded, want error")
			}
			if !errors.Is(err, safefs.ErrInvalidOption) {
				t.Fatalf("error = %v, want wrapping ErrInvalidOption", err)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Open / ReadFile / Stat — positive
// -----------------------------------------------------------------------------

// TestOpen_ReadsFile is the canonical positive case for Open.
func TestOpen_ReadsFile(t *testing.T) {
	root := newMemRoot(t, map[string]string{
		"a.txt": "hello",
	})
	f, err := root.Open("a.txt")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Name() != "a.txt" {
		t.Fatalf("Name() = %q, want %q", info.Name(), "a.txt")
	}
}

// TestReadFile_ReturnsContents verifies the whole-file convenience.
func TestReadFile_ReturnsContents(t *testing.T) {
	root := newMemRoot(t, map[string]string{
		"a.txt": "hello",
	})
	got, err := root.ReadFile("a.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("ReadFile = %q, want %q", got, "hello")
	}
}

// TestStat_ReturnsMetadata verifies Stat returns the FileInfo
// produced by the underlying filesystem.
func TestStat_ReturnsMetadata(t *testing.T) {
	root := newMemRoot(t, map[string]string{
		"a.txt": "hello",
	})
	info, err := root.Stat("a.txt")
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Size() != 5 {
		t.Fatalf("Size = %d, want 5", info.Size())
	}
}

// TestReadDir_ReturnsSortedEntries verifies the sorted-order
// contract inherited from io/fs.ReadDir.
func TestReadDir_ReturnsSortedEntries(t *testing.T) {
	root := newMemRoot(t, map[string]string{
		"dir/c.txt": "c",
		"dir/a.txt": "a",
		"dir/b.txt": "b",
	})
	entries, err := root.ReadDir("dir")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	got := entryNames(entries)
	want := []string{"a.txt", "b.txt", "c.txt"}
	if !equalStrings(got, want) {
		t.Fatalf("entries = %v, want %v", got, want)
	}
}

// -----------------------------------------------------------------------------
// validateName — negative and edge
// -----------------------------------------------------------------------------

// TestOpen_RejectsEmpty is the empty-string corner case. The
// diagnostic is ErrNotLocal, not ErrPathTraversal, because an
// empty name is malformed rather than an escape attempt.
func TestOpen_RejectsEmpty(t *testing.T) {
	root := newMemRoot(t, nil)
	_, err := root.Open("")
	if err == nil {
		t.Fatal("Open(\"\") succeeded, want error")
	}
	if !errors.Is(err, safefs.ErrNotLocal) {
		t.Fatalf("error = %v, want wrapping ErrNotLocal", err)
	}
}

// TestOpen_RejectsParentTraversal is the canonical negative case
// for CWE-22. Each name in the list cleans to a path that begins
// with ".." and therefore escapes the root.
//
// Names that mention ".." but resolve back inside the root (such
// as "a/..") are legal and are tested in
// TestOpen_AcceptsDotDotThatStaysInside. The policy is defined on
// the *cleaned* form of the name, not on the raw input.
func TestOpen_RejectsParentTraversal(t *testing.T) {
	root := newMemRoot(t, nil)
	cases := []string{
		"..",
		"../etc/passwd",
		"a/../../b",
	}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := root.Open(name)
			if err == nil {
				t.Fatalf("Open(%q) succeeded, want error", name)
			}
			if !errors.Is(err, safefs.ErrPathTraversal) {
				t.Fatalf("error = %v, want wrapping ErrPathTraversal", err)
			}
		})
	}
}

// TestOpen_AcceptsDotDotThatStaysInside verifies that a name
// which mentions ".." but resolves to a path inside the root is
// accepted. The policy is "the cleaned name must be local," not
// "the raw name must not contain a parent segment."
//
// "a/.." cleans to ".", which is the root. It is the canonical
// example of a name that mentions ".." and is still safe.
func TestOpen_AcceptsDotDotThatStaysInside(t *testing.T) {
	root := newMemRoot(t, map[string]string{
		"a/b.txt": "x",
	})

	// "a/.." is equivalent to ".". A ReadDir on "." must
	// succeed and list the root's entries.
	entries, err := root.ReadDir("a/..")
	if err != nil {
		t.Fatalf("ReadDir(a/..): %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "a" {
		t.Fatalf("entries = %v, want [a]", entryNames(entries))
	}
}

// TestOpen_RejectsAbsolutePath is the CWE-22 counterpart for
// absolute names that are absolute on every platform.
//
// The Windows drive-letter form is tested separately, because
// filepath.IsLocal on Unix treats "C:/Windows/System32" as a
// legal relative path with a literal colon in the first segment.
// See TestOpen_RejectsWindowsDriveLetter for the
// platform-specific assertion.
func TestOpen_RejectsAbsolutePath(t *testing.T) {
	root := newMemRoot(t, nil)
	cases := []string{
		"/etc/passwd",
		`\\server\share`,
	}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := root.Open(name)
			if err == nil {
				t.Fatalf("Open(%q) succeeded, want error", name)
			}
			// The distinction between ErrNotLocal and
			// ErrPathTraversal is documented: absolute
			// paths are malformed, not traversal.
			if !errors.Is(err, safefs.ErrNotLocal) &&
				!errors.Is(err, safefs.ErrPathTraversal) {
				t.Fatalf("error = %v, want ErrNotLocal or ErrPathTraversal", err)
			}
		})
	}
}

// TestOpen_RejectsWindowsDriveLetter verifies the drive-letter
// case on Windows, where filepath.IsLocal recognises it as
// absolute. On other platforms the same input normalises to a
// legal relative path, so the test asserts the platform-specific
// behaviour.
//
// A separate test exists for this case because the input's
// meaning genuinely differs by platform, and folding it into
// TestOpen_RejectsAbsolutePath would encode a Windows assumption
// into a cross-platform test.
func TestOpen_RejectsWindowsDriveLetter(t *testing.T) {
	root := newMemRoot(t, nil)
	name := `C:\Windows\System32`

	_, err := root.Open(name)

	if runtime.GOOS == "windows" {
		if err == nil {
			t.Fatalf("Open(%q) succeeded on Windows, want error", name)
		}
		if !errors.Is(err, safefs.ErrNotLocal) &&
			!errors.Is(err, safefs.ErrPathTraversal) {
			t.Fatalf("error = %v, want ErrNotLocal or ErrPathTraversal", err)
		}
		return
	}

	// Non-Windows: the input is a legal relative path. It may
	// fail with fs.ErrNotExist because the in-memory tree is
	// empty, but it must not fail with ErrNotLocal or
	// ErrPathTraversal.
	if errors.Is(err, safefs.ErrNotLocal) || errors.Is(err, safefs.ErrPathTraversal) {
		t.Fatalf("Open(%q) produced a name-validation error on %s, "+
			"where the input is a legal relative path",
			name, runtime.GOOS)
	}
}

// TestOpen_NormalisesBackslashes verifies that a Windows-style
// name reaches the in-memory FS with forward slashes. On Unix
// this is a no-op for well-formed names; the test asserts the
// behaviour on the platform it runs on.
func TestOpen_NormalisesBackslashes(t *testing.T) {
	root := newMemRoot(t, map[string]string{
		"a/b.txt": "x",
	})
	f, err := root.Open(`a\b.txt`)
	if err != nil {
		t.Fatalf("Open(`a\\b.txt`): %v", err)
	}
	f.Close()
}

// TestOpen_DotMeansRoot verifies that "." is accepted and refers
// to the root itself. It is the only name with a non-slash
// meaning after cleaning.
func TestOpen_DotMeansRoot(t *testing.T) {
	root := newMemRoot(t, map[string]string{"a.txt": "x"})
	entries, err := root.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir(\".\"): %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("ReadDir(\".\") returned %d entries, want 1", len(entries))
	}
}

// TestOpen_CleansRedundantSeparators verifies that "a//b.txt" is
// equivalent to "a/b.txt".
func TestOpen_CleansRedundantSeparators(t *testing.T) {
	root := newMemRoot(t, map[string]string{
		"a/b.txt": "x",
	})
	f, err := root.Open("a//b.txt")
	if err != nil {
		t.Fatalf("Open(\"a//b.txt\"): %v", err)
	}
	f.Close()
}

// TestOpen_RejectsNameWithOnlyDots is the corner case where a
// name cleans to "..". path.Clean("..") is "..", which fails
// IsLocal and, via containsTraversal, is classified as traversal.
func TestOpen_RejectsNameWithOnlyDots(t *testing.T) {
	root := newMemRoot(t, nil)
	_, err := root.Open("..")
	if err == nil {
		t.Fatal("Open(\"..\") succeeded, want error")
	}
	if !errors.Is(err, safefs.ErrPathTraversal) {
		t.Fatalf("error = %v, want wrapping ErrPathTraversal", err)
	}
}

// TestOpen_AcceptsNameWithDotsInFilename verifies that "..foo" is
// a legal filename, not a traversal segment. This is the case
// strings.Contains("..foo", "..") would get wrong.
func TestOpen_AcceptsNameWithDotsInFilename(t *testing.T) {
	root := newMemRoot(t, map[string]string{
		"..foo": "x",
	})
	f, err := root.Open("..foo")
	if err != nil {
		t.Fatalf("Open(\"..foo\"): %v", err)
	}
	f.Close()
}

// -----------------------------------------------------------------------------
// Error propagation from the underlying FS
// -----------------------------------------------------------------------------

// TestReadFile_MissingWrapsErrNotExist verifies the errors.Is
// contract: the sentinel fs.ErrNotExist must survive the
// abstraction.
func TestReadFile_MissingWrapsErrNotExist(t *testing.T) {
	root := newMemRoot(t, nil)
	_, err := root.ReadFile("missing.txt")
	if err == nil {
		t.Fatal("ReadFile succeeded, want error")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("error = %v, want wrapping fs.ErrNotExist", err)
	}
}

// TestStat_MissingWrapsErrNotExist is the Stat counterpart.
func TestStat_MissingWrapsErrNotExist(t *testing.T) {
	root := newMemRoot(t, nil)
	_, err := root.Stat("missing.txt")
	if err == nil {
		t.Fatal("Stat succeeded, want error")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("error = %v, want wrapping fs.ErrNotExist", err)
	}
}

// TestReadDir_MissingWrapsErrNotExist is the ReadDir counterpart.
func TestReadDir_MissingWrapsErrNotExist(t *testing.T) {
	root := newMemRoot(t, nil)
	_, err := root.ReadDir("missing")
	if err == nil {
		t.Fatal("ReadDir succeeded, want error")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("error = %v, want wrapping fs.ErrNotExist", err)
	}
}

// -----------------------------------------------------------------------------
// ReadDir — case collisions
// -----------------------------------------------------------------------------

// TestReadDir_DetectsCaseCollisions verifies that when the check
// is enabled, two names that fold to the same key produce
// ErrCaseCollision.
//
// The in-memory FS is case-sensitive on every platform, so a
// directory containing "README.md" and "readme.md" is legal at
// the FS layer. The detection happens in safefs.
func TestReadDir_DetectsCaseCollisions(t *testing.T) {
	root := newMemRoot(t,
		map[string]string{
			"d/README.md": "a",
			"d/readme.md": "b",
		},
		safefs.WithCaseCollisionDetection(true),
	)
	_, err := root.ReadDir("d")
	if err == nil {
		t.Fatal("ReadDir succeeded, want ErrCaseCollision")
	}
	if !errors.Is(err, safefs.ErrCaseCollision) {
		t.Fatalf("error = %v, want wrapping ErrCaseCollision", err)
	}
}

// TestReadDir_NoCollisionWhenDisabled verifies that the check is
// opt-in. The same tree that produces an error above succeeds
// with the default configuration.
func TestReadDir_NoCollisionWhenDisabled(t *testing.T) {
	root := newMemRoot(t, map[string]string{
		"d/README.md": "a",
		"d/readme.md": "b",
	})
	entries, err := root.ReadDir("d")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(entries))
	}
}

// TestReadDir_NoCollisionWhenNamesDiffer verifies that a
// directory without collisions passes the check. The check must
// not produce false positives.
func TestReadDir_NoCollisionWhenNamesDiffer(t *testing.T) {
	root := newMemRoot(t,
		map[string]string{
			"d/a.txt": "a",
			"d/b.txt": "b",
		},
		safefs.WithCaseCollisionDetection(true),
	)
	if _, err := root.ReadDir("d"); err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
}

// TestReadDir_SingleEntryNoCollision verifies the no-op path for
// fewer than two entries.
func TestReadDir_SingleEntryNoCollision(t *testing.T) {
	root := newMemRoot(t,
		map[string]string{"d/a.txt": "a"},
		safefs.WithCaseCollisionDetection(true),
	)
	if _, err := root.ReadDir("d"); err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
}

// -----------------------------------------------------------------------------
// Walk
// -----------------------------------------------------------------------------

// TestWalk_VisitsAllEntries verifies that Walk visits every file
// and directory exactly once.
func TestWalk_VisitsAllEntries(t *testing.T) {
	root := newMemRoot(t, map[string]string{
		"a.txt":     "a",
		"dir/b.txt": "b",
		"dir/c.txt": "c",
	})
	var seen []string
	err := root.Walk(".", func(rel string, d fs.DirEntry, err error) error {
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
	want := []string{"a.txt", "dir", "dir/b.txt", "dir/c.txt"}
	if !equalStrings(seen, want) {
		t.Fatalf("visited = %v, want %v", seen, want)
	}
}

// TestWalk_SkipDir verifies the fs.SkipDir control flow.
//
// The callback records every visited path before deciding
// whether to skip, so that the assertion can distinguish "was
// visited" from "was descended into." A directory that triggers
// SkipDir is recorded once and its children are not visited.
func TestWalk_SkipDir(t *testing.T) {
	root := newMemRoot(t, map[string]string{
		"a.txt":     "a",
		"dir/b.txt": "b",
	})
	var seen []string
	err := root.Walk(".", func(rel string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		seen = append(seen, rel)
		if d.IsDir() {
			return fs.SkipDir
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	sort.Strings(seen)
	want := []string{"a.txt", "dir"}
	if !equalStrings(seen, want) {
		t.Fatalf("visited = %v, want %v", seen, want)
	}
}

// TestWalk_SkipAll verifies that SkipAll stops the walk and
// returns nil.
func TestWalk_SkipAll(t *testing.T) {
	root := newMemRoot(t, map[string]string{
		"a.txt": "a",
		"b.txt": "b",
	})
	var count int
	err := root.Walk(".", func(rel string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		count++
		return fs.SkipAll
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if count == 0 {
		t.Fatal("Walk visited no entries before SkipAll")
	}
}

// TestWalk_NilFuncRejected verifies that a nil callback is
// rejected with an internal error rather than a panic.
func TestWalk_NilFuncRejected(t *testing.T) {
	root := newMemRoot(t, nil)
	err := root.Walk(".", nil)
	if err == nil {
		t.Fatal("Walk(nil) succeeded, want error")
	}
}

// TestWalk_MaxDepth verifies that the depth bound fires one level
// below the documented value.
func TestWalk_MaxDepth(t *testing.T) {
	root := newMemRoot(t,
		map[string]string{"a/b/c/d.txt": "x"},
		safefs.WithMaxDepth(1),
	)
	err := root.Walk(".", func(rel string, d fs.DirEntry, err error) error {
		return err
	})
	if err == nil {
		t.Fatal("Walk succeeded, want ErrDepthExceeded")
	}
	if !errors.Is(err, safefs.ErrDepthExceeded) {
		t.Fatalf("error = %v, want wrapping ErrDepthExceeded", err)
	}
}

// TestWalk_MaxDepthZeroIsUnbounded verifies that zero disables
// the bound.
func TestWalk_MaxDepthZeroIsUnbounded(t *testing.T) {
	root := newMemRoot(t,
		map[string]string{"a/b/c/d/e/f.txt": "x"},
		safefs.WithMaxDepth(0),
	)
	err := root.Walk(".", func(rel string, d fs.DirEntry, err error) error {
		return err
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
}

// TestWalk_MaxEntries verifies that the entry bound fires on the
// (n+1)th entry.
func TestWalk_MaxEntries(t *testing.T) {
	root := newMemRoot(t,
		map[string]string{
			"a.txt": "a",
			"b.txt": "b",
			"c.txt": "c",
		},
		safefs.WithMaxEntries(2),
	)
	err := root.Walk(".", func(rel string, d fs.DirEntry, err error) error {
		return err
	})
	if err == nil {
		t.Fatal("Walk succeeded, want ErrEntriesExceeded")
	}
	if !errors.Is(err, safefs.ErrEntriesExceeded) {
		t.Fatalf("error = %v, want wrapping ErrEntriesExceeded", err)
	}
}

// TestWalk_MaxEntriesZeroIsUnbounded verifies that zero disables
// the bound.
func TestWalk_MaxEntriesZeroIsUnbounded(t *testing.T) {
	root := newMemRoot(t,
		map[string]string{
			"a.txt": "a",
			"b.txt": "b",
			"c.txt": "c",
		},
		safefs.WithMaxEntries(0),
	)
	err := root.Walk(".", func(rel string, d fs.DirEntry, err error) error {
		return err
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
}

// TestWalk_TraversalNameRejected verifies that the name contract
// applies to Walk's starting point as well as to every other
// method.
func TestWalk_TraversalNameRejected(t *testing.T) {
	root := newMemRoot(t, nil)
	err := root.Walk("../escape", func(rel string, d fs.DirEntry, err error) error {
		return err
	})
	if err == nil {
		t.Fatal("Walk succeeded, want error")
	}
	if !errors.Is(err, safefs.ErrPathTraversal) {
		t.Fatalf("error = %v, want wrapping ErrPathTraversal", err)
	}
}

// -----------------------------------------------------------------------------
// Non-functional
// -----------------------------------------------------------------------------

// TestRoot_ConcurrentReads verifies the concurrency contract: a
// single Root is safe for concurrent use by multiple goroutines.
// Run with -race.
func TestRoot_ConcurrentReads(t *testing.T) {
	root := newMemRoot(t, map[string]string{
		"a.txt": "aaa",
		"b.txt": "bbb",
		"c.txt": "ccc",
	})
	var wg sync.WaitGroup
	errs := make(chan error, 64)
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := []string{"a.txt", "b.txt", "c.txt"}[i%3]
			if _, err := root.ReadFile(name); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("goroutine error: %v", err)
	}
}

// FuzzReadFile asserts that ReadFile never panics on an arbitrary
// input. The property is deliberately loose: the fuzz target is a
// smoke test that the API is robust to any string, not a
// verification of the confinement invariant. A stronger target
// lives in fuzz_test.go.
func FuzzReadFile(f *testing.F) {
	f.Add("a.txt")
	f.Add("../etc/passwd")
	f.Add("")
	f.Add("..foo")
	f.Add("a/b/c.txt")
	f.Add(`C:\Windows`)
	f.Add("a\x00b")

	m := fstest.MapFS{
		"a.txt":     &fstest.MapFile{Data: []byte("x")},
		"a/b/c.txt": &fstest.MapFile{Data: []byte("y")},
	}
	root, err := safefs.New("/root",
		safefs.WithFS(filesystem.NewMemFS(m)))
	if err != nil {
		f.Fatalf("safefs.New: %v", err)
	}

	f.Fuzz(func(t *testing.T, name string) {
		// No panic is the only hard requirement.
		_, _ = root.ReadFile(name)
	})
}

// -----------------------------------------------------------------------------
// Small helpers, local to this file
// -----------------------------------------------------------------------------

func entryNames(entries []fs.DirEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Name()
	}
	return out
}

func equalStrings(a, b []string) bool {
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
