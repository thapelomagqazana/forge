package safefs_test

import (
	"errors"
	"io/fs"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/thapelomagqazana/forge/internal/filesystem"
	"github.com/thapelomagqazana/forge/internal/filesystem/safefs"
	"github.com/thapelomagqazana/forge/internal/forgeerr"
)

// This file exercises the sentinel errors and the wrapping helpers
// defined in errors.go, through the exported surface of safefs.
//
// The sentinels — ErrPathTraversal, ErrNotLocal, ErrSymlinkDenied,
// ErrDepthExceeded, ErrEntriesExceeded, ErrCaseCollision — are
// exported. The helpers that wrap them — pathError, wrapFS,
// wrapFSName, isNotExist, isPermission, isSymlinkDenied,
// isTraversal — are unexported and are tested indirectly.
//
// The package's internal test file (currently policy_test.go)
// does NOT exercise the unexported helpers. A future
// errors_internal_test.go in `package safefs` should add direct
// unit tests for them. This file documents that gap so it is not
// lost, and exercises what it can from outside.
//
// The test taxonomy:
//
//   - Positive:    the sentinel is the error that a failure
//                  produces, and errors.Is finds it through the
//                  returned value.
//   - Negative:    an unrelated sentinel does not match.
//   - Edge:        a wrapped sentinel still matches; a nil error
//                  does not match anything.
//   - Corner:      the error message contains the name that
//                  caused the failure.
//   - Boundary:    the sentinel's string form is stable.
//   - Non-functional: sentinel identity is process-wide (two
//                     references are the same value); the
//                     message is deterministic across calls.

// -----------------------------------------------------------------------------
// Sentinel identity
// -----------------------------------------------------------------------------

// TestSentinels_AreDistinct verifies that the exported sentinels
// are distinct values. A future refactor that collapses two
// sentinels into one would pass other tests but fail this one.
func TestSentinels_AreDistinct(t *testing.T) {
	all := []error{
		safefs.ErrPathTraversal,
		safefs.ErrNotLocal,
		safefs.ErrSymlinkDenied,
		safefs.ErrDepthExceeded,
		safefs.ErrEntriesExceeded,
		safefs.ErrCaseCollision,
	}
	for i := range all {
		for j := range all {
			if i == j {
				continue
			}
			if errors.Is(all[i], all[j]) {
				t.Fatalf("sentinel %d matches sentinel %d", i, j)
			}
		}
	}
}

// TestSentinels_NonNil verifies that every sentinel is a non-nil
// error. A sentinel that is nil would make errors.Is always
// return true, breaking every negative test elsewhere.
func TestSentinels_NonNil(t *testing.T) {
	all := map[string]error{
		"ErrPathTraversal":   safefs.ErrPathTraversal,
		"ErrNotLocal":        safefs.ErrNotLocal,
		"ErrSymlinkDenied":   safefs.ErrSymlinkDenied,
		"ErrDepthExceeded":   safefs.ErrDepthExceeded,
		"ErrEntriesExceeded": safefs.ErrEntriesExceeded,
		"ErrCaseCollision":   safefs.ErrCaseCollision,
	}
	for name, sentinel := range all {
		if sentinel == nil {
			t.Fatalf("%s is nil", name)
		}
	}
}

// TestSentinels_HaveStableMessages verifies that each sentinel
// produces a non-empty, deterministic message. The message is
// part of the diagnostic surface, and a future change to it is a
// deliberate breaking change that should touch a test.
func TestSentinels_HaveStableMessages(t *testing.T) {
	all := []error{
		safefs.ErrPathTraversal,
		safefs.ErrNotLocal,
		safefs.ErrSymlinkDenied,
		safefs.ErrDepthExceeded,
		safefs.ErrEntriesExceeded,
		safefs.ErrCaseCollision,
	}
	for _, sentinel := range all {
		first := sentinel.Error()
		if first == "" {
			t.Fatalf("sentinel with empty message: %v", sentinel)
		}
		for i := 0; i < 8; i++ {
			if got := sentinel.Error(); got != first {
				t.Fatalf("message changed: %q vs %q", first, got)
			}
		}
	}
}

// -----------------------------------------------------------------------------
// ErrNotLocal — reached through Root
// -----------------------------------------------------------------------------

// TestErrNotLocal_EmptyName verifies that an empty name produces
// an error wrapping ErrNotLocal.
func TestErrNotLocal_EmptyName(t *testing.T) {
	root := newErrorsRoot(t, nil)
	_, err := root.Open("")
	if err == nil {
		t.Fatal("Open(\"\") succeeded, want error")
	}
	if !errors.Is(err, safefs.ErrNotLocal) {
		t.Fatalf("error = %v, want wrapping ErrNotLocal", err)
	}
}

// TestErrNotLocal_AbsoluteName verifies that a Unix-style absolute
// path produces an error wrapping ErrNotLocal. The distinction
// from ErrPathTraversal is documented: absolute paths are
// malformed, not traversal attempts.
//
// The Windows drive-letter form is tested separately, because
// filepath.IsLocal on Unix treats "C:/Windows" as a legal
// relative path with a literal colon in the first segment. See
// TestErrNotLocal_WindowsDriveLetter for the platform-specific
// assertion.
func TestErrNotLocal_AbsoluteName(t *testing.T) {
	root := newErrorsRoot(t, nil)
	for _, name := range []string{"/etc/passwd", "/"} {
		t.Run(name, func(t *testing.T) {
			_, err := root.Open(name)
			if err == nil {
				t.Fatalf("Open(%q) succeeded, want error", name)
			}
			if !errors.Is(err, safefs.ErrNotLocal) {
				t.Fatalf("error = %v, want wrapping ErrNotLocal", err)
			}
		})
	}
}

// TestErrNotLocal_WindowsDriveLetter verifies the drive-letter
// case on Windows, where filepath.IsLocal recognises it as
// absolute. On other platforms the same input is a legal
// relative path, so the test asserts the platform-specific
// behaviour.
//
// A separate test exists for this case because the input's
// meaning genuinely differs by platform, and folding it into
// TestErrNotLocal_AbsoluteName would encode a Windows
// assumption into a cross-platform test.
func TestErrNotLocal_WindowsDriveLetter(t *testing.T) {
	root := newErrorsRoot(t, nil)
	name := `C:\Windows`

	_, err := root.Open(name)

	if runtime.GOOS == "windows" {
		if err == nil {
			t.Fatalf("Open(%q) succeeded on Windows, want error", name)
		}
		if !errors.Is(err, safefs.ErrNotLocal) {
			t.Fatalf("error = %v, want wrapping ErrNotLocal", err)
		}
		return
	}

	// On non-Windows platforms the input normalises to
	// "C:/Windows", which is a legal relative path. The
	// operation may fail for an unrelated reason (the path
	// does not exist in the in-memory tree), but it must not
	// fail with ErrNotLocal.
	if errors.Is(err, safefs.ErrNotLocal) {
		t.Fatalf("Open(%q) produced ErrNotLocal on %s, "+
			"where the input is a legal relative path",
			name, runtime.GOOS)
	}
}

// TestErrNotLocal_DoesNotMatchTraversal verifies the negative
// direction: an ErrNotLocal error does not also match
// ErrPathTraversal. The two are separate branches of validateName
// and must remain distinguishable to callers.
func TestErrNotLocal_DoesNotMatchTraversal(t *testing.T) {
	root := newErrorsRoot(t, nil)
	_, err := root.Open("")
	if err == nil {
		t.Fatal("Open(\"\") succeeded, want error")
	}
	if errors.Is(err, safefs.ErrPathTraversal) {
		t.Fatal("ErrNotLocal error also matches ErrPathTraversal")
	}
}

// -----------------------------------------------------------------------------
// ErrPathTraversal — reached through Root
// -----------------------------------------------------------------------------

// TestErrPathTraversal_DotDot verifies that a name that *escapes*
// the root produces an error wrapping ErrPathTraversal.
//
// The inputs here all clean to a path that is not local — that
// is, a path that begins with ".." after path.Clean. Names that
// merely contain ".." as an intermediate segment but resolve back
// into the root (such as "a/..") are legal and are tested in
// TestValidateName_DotDotThatStaysInside.
func TestErrPathTraversal_DotDot(t *testing.T) {
	root := newErrorsRoot(t, nil)
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

// TestErrPathTraversal_MessageContainsName verifies the documented
// property that pathError attaches the offending name to the
// message. The name is safe to log because it has already been
// validated as a well-formed input that was refused.
func TestErrPathTraversal_MessageContainsName(t *testing.T) {
	root := newErrorsRoot(t, nil)
	_, err := root.Open("../etc/passwd")
	if err == nil {
		t.Fatal("Open succeeded, want error")
	}
	if !strings.Contains(err.Error(), "../etc/passwd") {
		t.Fatalf("error = %q, want to contain the offending name", err)
	}
}

// TestErrPathTraversal_DoesNotMatchNotLocal verifies the negative
// direction for the other branch.
func TestErrPathTraversal_DoesNotMatchNotLocal(t *testing.T) {
	root := newErrorsRoot(t, nil)
	_, err := root.Open("../etc/passwd")
	if err == nil {
		t.Fatal("Open succeeded, want error")
	}
	if errors.Is(err, safefs.ErrNotLocal) {
		t.Fatal("ErrPathTraversal error also matches ErrNotLocal")
	}
}

// -----------------------------------------------------------------------------
// Positive: names that mention ".." but stay inside
// -----------------------------------------------------------------------------

// TestValidateName_DotDotThatStaysInside verifies that a name
// which contains ".." but resolves to a path inside the root is
// accepted. The policy is "the cleaned name must be local," not
// "the raw name must not contain a parent segment."
//
// "a/.." cleans to ".", which is the root. It is the canonical
// example of a name that mentions ".." and is still safe.
func TestValidateName_DotDotThatStaysInside(t *testing.T) {
	root := newErrorsRoot(t, map[string]string{
		"a/b.txt": "x",
	})

	// "a/.." is equivalent to ".". A ReadDir on "." must
	// succeed and list the root's entries.
	entries, err := root.ReadDir("a/..")
	if err != nil {
		t.Fatalf("ReadDir(a/..): %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "a" {
		t.Fatalf("entries = %v, want [a]", entryNamesErrors(entries))
	}
}

// TestValidateName_DotDotInsideSubtree verifies a related case:
// a name that mentions ".." but stays within the root's subtree.
func TestValidateName_DotDotInsideSubtree(t *testing.T) {
	root := newErrorsRoot(t, map[string]string{
		"a/b.txt": "x",
	})

	// "a/b.txt/../b.txt" cleans to "a/b.txt", which exists.
	// The traversal segment is resolved away by path.Clean and
	// the result is a legal local path.
	got, err := root.ReadFile("a/b.txt/../b.txt")
	if err != nil {
		t.Fatalf("ReadFile(a/b.txt/../b.txt): %v", err)
	}
	if string(got) != "x" {
		t.Fatalf("ReadFile = %q, want %q", got, "x")
	}
}

// -----------------------------------------------------------------------------
// ErrDepthExceeded — reached through Walk
// -----------------------------------------------------------------------------

// TestErrDepthExceeded_ReachedByWalk verifies that a Walk over a
// tree deeper than MaxDepth produces an error wrapping
// ErrDepthExceeded.
func TestErrDepthExceeded_ReachedByWalk(t *testing.T) {
	root := newErrorsRoot(t,
		map[string]string{"a/b/c/d.txt": "x"},
		safefs.WithMaxDepth(1),
	)
	err := root.Walk(".", func(string, fs.DirEntry, error) error { return nil })
	if err == nil {
		t.Fatal("Walk succeeded, want error")
	}
	if !errors.Is(err, safefs.ErrDepthExceeded) {
		t.Fatalf("error = %v, want wrapping ErrDepthExceeded", err)
	}
}

// TestErrDepthExceeded_MessageContainsName verifies that the
// offending name is attached. The name is the directory from
// which the descent was refused.
func TestErrDepthExceeded_MessageContainsName(t *testing.T) {
	root := newErrorsRoot(t,
		map[string]string{"deep/very/deep/file.txt": "x"},
		safefs.WithMaxDepth(1),
	)
	err := root.Walk(".", func(string, fs.DirEntry, error) error { return nil })
	if err == nil {
		t.Fatal("Walk succeeded, want error")
	}
	// The name attached is the directory at the boundary, not
	// the file that would have been visited. The exact name is
	// implementation detail; the test asserts that *some* name
	// is present, which is the documented contract.
	if !strings.Contains(err.Error(), "deep") {
		t.Fatalf("error = %q, want to contain a path component", err)
	}
}

// -----------------------------------------------------------------------------
// ErrEntriesExceeded — reached through Walk
// -----------------------------------------------------------------------------

// TestErrEntriesExceeded_ReachedByWalk verifies that a Walk over a
// tree with more entries than MaxEntries produces an error
// wrapping ErrEntriesExceeded.
func TestErrEntriesExceeded_ReachedByWalk(t *testing.T) {
	root := newErrorsRoot(t,
		map[string]string{
			"a.txt": "a",
			"b.txt": "b",
			"c.txt": "c",
		},
		safefs.WithMaxEntries(2),
	)
	err := root.Walk(".", func(string, fs.DirEntry, error) error { return nil })
	if err == nil {
		t.Fatal("Walk succeeded, want error")
	}
	if !errors.Is(err, safefs.ErrEntriesExceeded) {
		t.Fatalf("error = %v, want wrapping ErrEntriesExceeded", err)
	}
}

// -----------------------------------------------------------------------------
// ErrCaseCollision — reached through ReadDir
// -----------------------------------------------------------------------------

// TestErrCaseCollision_ReachedByReadDir verifies that a directory
// with two names that fold to the same key produces an error
// wrapping ErrCaseCollision when the check is enabled.
func TestErrCaseCollision_ReachedByReadDir(t *testing.T) {
	root := newErrorsRoot(t,
		map[string]string{
			"d/README.md": "a",
			"d/readme.md": "b",
		},
		safefs.WithCaseCollisionDetection(true),
	)
	_, err := root.ReadDir("d")
	if err == nil {
		t.Fatal("ReadDir succeeded, want error")
	}
	if !errors.Is(err, safefs.ErrCaseCollision) {
		t.Fatalf("error = %v, want wrapping ErrCaseCollision", err)
	}
}

// TestErrCaseCollision_MessageContainsBothNames verifies that the
// message names both colliding entries. This is the property that
// makes the diagnostic actionable.
func TestErrCaseCollision_MessageContainsBothNames(t *testing.T) {
	root := newErrorsRoot(t,
		map[string]string{
			"d/README.md": "a",
			"d/readme.md": "b",
		},
		safefs.WithCaseCollisionDetection(true),
	)
	_, err := root.ReadDir("d")
	if err == nil {
		t.Fatal("ReadDir succeeded, want error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "README.md") || !strings.Contains(msg, "readme.md") {
		t.Fatalf("error = %q, want to name both colliding entries", msg)
	}
}

// -----------------------------------------------------------------------------
// Error chain composition
// -----------------------------------------------------------------------------

// TestErrorChain_PreservesFSErrNotExist verifies the property that
// safefs's error wrapping does not swallow the underlying
// filesystem sentinel. A caller can branch on both the safefs
// sentinel and the stdlib sentinel from the same error value.
//
// This is the property the ADR-013 wrapping contract requires,
// and it is what makes safefs usable from callers that need to
// distinguish "the file does not exist" from "the name was
// refused."
func TestErrorChain_PreservesFSErrNotExist(t *testing.T) {
	root := newErrorsRoot(t, nil)
	_, err := root.ReadFile("missing.txt")
	if err == nil {
		t.Fatal("ReadFile succeeded, want error")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("error = %v, want wrapping fs.ErrNotExist", err)
	}
}

// TestErrorChain_SafefsSentinelDoesNotImplyFSSentinel verifies the
// negative direction for the traversal sentinel: a name refused by
// validateName never reaches the filesystem, so the error must not
// claim to wrap fs.ErrNotExist.
func TestErrorChain_SafefsSentinelDoesNotImplyFSSentinel(t *testing.T) {
	root := newErrorsRoot(t, nil)
	_, err := root.Open("../escape")
	if err == nil {
		t.Fatal("Open succeeded, want error")
	}
	if errors.Is(err, fs.ErrNotExist) {
		t.Fatal("traversal error also matches fs.ErrNotExist")
	}
}

// -----------------------------------------------------------------------------
// Nil error handling
// -----------------------------------------------------------------------------

// TestErrorIs_NilDoesNotMatch verifies that a nil error does not
// satisfy any sentinel check. This is the safety property that
// lets callers write errors.Is without an explicit nil guard.
func TestErrorIs_NilDoesNotMatch(t *testing.T) {
	var err error
	for _, sentinel := range []error{
		safefs.ErrPathTraversal,
		safefs.ErrNotLocal,
		safefs.ErrSymlinkDenied,
		safefs.ErrDepthExceeded,
		safefs.ErrEntriesExceeded,
		safefs.ErrCaseCollision,
	} {
		if errors.Is(err, sentinel) {
			t.Fatalf("nil error matches %v", sentinel)
		}
	}
}

// -----------------------------------------------------------------------------
// forgeerr integration
// -----------------------------------------------------------------------------

// TestSentinels_AreForgeerrErrors verifies that every sentinel
// carries a forgeerr category. The category is what the CLI uses
// to map an error to an exit code without string matching, so a
// sentinel that lost its category would silently break the CLI's
// error handling.
//
// The check is done with errors.As against *forgeerr.Error.
func TestSentinels_AreForgeerrErrors(t *testing.T) {
	all := map[string]error{
		"ErrPathTraversal":   safefs.ErrPathTraversal,
		"ErrNotLocal":        safefs.ErrNotLocal,
		"ErrSymlinkDenied":   safefs.ErrSymlinkDenied,
		"ErrDepthExceeded":   safefs.ErrDepthExceeded,
		"ErrEntriesExceeded": safefs.ErrEntriesExceeded,
		"ErrCaseCollision":   safefs.ErrCaseCollision,
	}
	for name, sentinel := range all {
		t.Run(name, func(t *testing.T) {
			var fe *forgeerr.Error
			if !errors.As(sentinel, &fe) {
				t.Fatalf("%s is not a *forgeerr.Error", name)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

// newErrorsRoot constructs a Root backed by an in-memory FS
// populated with the given tree. It mirrors the helper in
// root_test.go; the two files do not share helpers because the
// external test package would require a shared helper file to do
// so, and that adds a file to the directory for no coverage gain.
// If a third test file needs the same helper, promote it to
// safefs_test_helpers_test.go.
func newErrorsRoot(t *testing.T, tree map[string]string, opts ...safefs.Option) *safefs.Root {
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

// entryNamesErrors returns the names of entries, in the order they
// appear. It is local to this file; a shared helper file would
// remove the duplication if a third file needs it.
func entryNamesErrors(entries []fs.DirEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Name()
	}
	return out
}
