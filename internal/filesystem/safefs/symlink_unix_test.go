//go:build unix

package safefs_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	"github.com/thapelomagqazana/forge/internal/filesystem/safefs"
)

// This file exercises the symlink policy matrix on Unix.
//
// It is the only test file in the package that touches a real
// disk, because fstest.MapFS does not represent symlinks and the
// property under test — that the OS-level Lstat and Readlink
// behave as the policy assumes — can only be observed against a
// real filesystem.
//
// Every test skips if the host does not support symlinks. That is
// the case in some containerized CI environments, and a skip is
// the honest outcome: the code is unchanged, the environment
// cannot express the input.
//
// The test taxonomy:
//
//   - Positive:    a policy-legal link resolves.
//   - Negative:    a policy-illegal link is refused.
//   - Edge:        an empty target, a target of ".", a target of
//                  "..", a chain, a cycle, a relative target
//                  with "..".
//   - Corner:      policy interactions with Walk, ReadDir, Stat.
//   - Boundary:    MaxSymlinkChain exactly at the limit and one
//                  over.
//   - Non-functional: concurrent resolution of the same link.

// assertEntriesEqual asserts that entries contains exactly the
// names in want, ignoring order. It is the assertion helper for
// the ReadDir-based tests, where the fixture always includes the
// directory's own entries and enumerating them explicitly at the
// call site makes the fixture legible.
func assertEntriesEqual(t *testing.T, entries []fs.DirEntry, want ...string) {
	t.Helper()
	got := dirEntryNames(entries)
	sort.Strings(got)
	sort.Strings(want)
	if !equalStrings(got, want) {
		t.Fatalf("entries = %v, want %v", got, want)
	}
}

// -----------------------------------------------------------------------------
// NoFollow — the default policy
// -----------------------------------------------------------------------------

// TestNoFollow_AcceptsPlainFile verifies the positive case: a
// name with no link components opens normally under the default
// policy. This anchors every negative case below.
func TestNoFollow_AcceptsPlainFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"), "hello")

	root := newOSRoot(t, dir)
	got, err := root.ReadFile("a.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("ReadFile = %q, want %q", got, "hello")
	}
}

// TestNoFollow_RejectsFinalComponentLink verifies that a name
// whose final component is a symlink is refused. This is the
// CWE-59 case: the link itself is the escape vector, and the
// caller did not ask to follow it.
func TestNoFollow_RejectsFinalComponentLink(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "secret.txt"), "sensitive")
	symlink(t, filepath.Join(outside, "secret.txt"),
		filepath.Join(dir, "link.txt"))

	root := newOSRoot(t, dir)
	_, err := root.Open("link.txt")
	if err == nil {
		t.Fatal("Open(link.txt) succeeded under NoFollow, want error")
	}
	if !errors.Is(err, safefs.ErrSymlinkDenied) {
		t.Fatalf("error = %v, want wrapping ErrSymlinkDenied", err)
	}
}

// TestNoFollow_RejectsIntermediateComponentLink verifies that a
// link is refused even when it appears before the final
// component. This is the case a naive "check only the last
// component" implementation would miss.
func TestNoFollow_RejectsIntermediateComponentLink(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(outside, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(outside, "sub", "secret.txt"), "sensitive")
	symlink(t, filepath.Join(outside, "sub"),
		filepath.Join(dir, "linkdir"))

	root := newOSRoot(t, dir)
	_, err := root.Open("linkdir/secret.txt")
	if err == nil {
		t.Fatal("Open(linkdir/secret.txt) succeeded, want error")
	}
	if !errors.Is(err, safefs.ErrSymlinkDenied) {
		t.Fatalf("error = %v, want wrapping ErrSymlinkDenied", err)
	}
}

// TestNoFollow_RejectsFirstComponentLink verifies the first
// component case, which is the least obvious: the link is the
// first thing in the path, and every subsequent component sits
// beneath it.
func TestNoFollow_RejectsFirstComponentLink(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "secret.txt"), "sensitive")
	symlink(t, outside, filepath.Join(dir, "linkdir"))

	root := newOSRoot(t, dir)
	_, err := root.Open("linkdir/secret.txt")
	if err == nil {
		t.Fatal("Open succeeded, want error")
	}
	if !errors.Is(err, safefs.ErrSymlinkDenied) {
		t.Fatalf("error = %v, want wrapping ErrSymlinkDenied", err)
	}
}

// TestNoFollow_AcceptsDotDotInFilename verifies that "..foo" is
// not mistaken for a traversal segment. This is a corner case
// shared with validateName; the test exists here because a
// symlink-aware implementation might otherwise misclassify it.
func TestNoFollow_AcceptsDotDotInFilename(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "..foo"), "x")

	root := newOSRoot(t, dir)
	if _, err := root.ReadFile("..foo"); err != nil {
		t.Fatalf("ReadFile(..foo): %v", err)
	}
}

// TestNoFollow_AcceptsDot verifies that "." (the root) is
// readable via ReadDir under the default policy. The fixture
// creates the file first and the assertion enumerates both the
// file and any other entries the fixture placed at the root.
func TestNoFollow_AcceptsDot(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"), "x")

	root := newOSRoot(t, dir)
	entries, err := root.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir(.): %v", err)
	}
	assertEntriesEqual(t, entries, "a.txt")
}

// -----------------------------------------------------------------------------
// FollowInRoot — in-root targets
// -----------------------------------------------------------------------------

// TestFollowInRoot_RelativeTargetInside verifies the positive
// case: a link whose target is inside the root resolves, and the
// operation succeeds.
func TestFollowInRoot_RelativeTargetInside(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "real.txt"), "hello")
	symlink(t, "real.txt", filepath.Join(dir, "link.txt"))

	root := newOSRoot(t, dir,
		safefs.WithSymlinkPolicy(safefs.FollowInRoot))
	got, err := root.ReadFile("link.txt")
	if err != nil {
		t.Fatalf("ReadFile(link.txt): %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("ReadFile = %q, want %q", got, "hello")
	}
}

// TestFollowInRoot_AbsoluteTargetInside verifies that an absolute
// target that happens to be inside the root is refused. The
// rationale is in symlink.go: an absolute target cannot be
// confined by relative-path reasoning, so it is refused without
// inspection of whether it happens to resolve inside.
func TestFollowInRoot_AbsoluteTargetInside(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "real.txt"), "hello")
	symlink(t, filepath.Join(dir, "real.txt"),
		filepath.Join(dir, "link.txt"))

	root := newOSRoot(t, dir,
		safefs.WithSymlinkPolicy(safefs.FollowInRoot))
	_, err := root.Open("link.txt")
	if err == nil {
		t.Fatal("Open succeeded for absolute target, want error")
	}
	if !errors.Is(err, safefs.ErrSymlinkDenied) {
		t.Fatalf("error = %v, want wrapping ErrSymlinkDenied", err)
	}
}

// TestFollowInRoot_IntermediateTargetInside verifies that a link
// appearing before the final component resolves when its target
// is inside the root.
func TestFollowInRoot_IntermediateTargetInside(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "sub", "a.txt"), "hello")
	symlink(t, "sub", filepath.Join(dir, "linkdir"))

	root := newOSRoot(t, dir,
		safefs.WithSymlinkPolicy(safefs.FollowInRoot))
	got, err := root.ReadFile("linkdir/a.txt")
	if err != nil {
		t.Fatalf("ReadFile(linkdir/a.txt): %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("ReadFile = %q, want %q", got, "hello")
	}
}

// TestFollowInRoot_TargetIsDot verifies the corner case where a
// link's target is "." — the directory that contains the link.
// The link resolves to that directory, and ReadDir on the link
// lists the directory's entries.
//
// The target directory contains both the file the fixture
// created and the link itself. The assertion enumerates both.
func TestFollowInRoot_TargetIsDot(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"), "hello")
	symlink(t, ".", filepath.Join(dir, "link"))

	root := newOSRoot(t, dir,
		safefs.WithSymlinkPolicy(safefs.FollowInRoot))

	entries, err := root.ReadDir("link")
	if err != nil {
		t.Fatalf("ReadDir(link): %v", err)
	}
	assertEntriesEqual(t, entries, "a.txt", "link")
}

// TestFollowInRoot_TargetContainsDotDot verifies the corner case
// where a link's relative target contains ".." but stays inside
// the root. The cleaning step in followInRoot must resolve the
// ".." before the escape check.
//
// The link is at "a/b/link.txt" and its target is "../real.txt".
// The target is interpreted relative to the link's parent
// ("a/b"), so the resolved path is "a/b/../real.txt", which
// cleans to "a/real.txt". That file exists inside the root.
func TestFollowInRoot_TargetContainsDotDot(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "a", "real.txt"), "hello")
	symlink(t, "../real.txt", filepath.Join(dir, "a", "b", "link.txt"))

	root := newOSRoot(t, dir,
		safefs.WithSymlinkPolicy(safefs.FollowInRoot))
	got, err := root.ReadFile("a/b/link.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("ReadFile = %q, want %q", got, "hello")
	}
}

// -----------------------------------------------------------------------------
// FollowInRoot — escaping targets
// -----------------------------------------------------------------------------

// TestFollowInRoot_RelativeTargetOutside verifies the canonical
// negative case: a relative target that escapes the root is
// refused. This is CWE-59 and CWE-61.
//
// The link is at the root and its target is "../<basename>/secret.txt",
// where <basename> is the base name of the outside directory.
// The target is interpreted relative to the link's parent (the
// root), so the resolved path is the outside directory.
func TestFollowInRoot_RelativeTargetOutside(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "secret.txt"), "sensitive")
	symlink(t, filepath.Join("..", filepath.Base(outside), "secret.txt"),
		filepath.Join(dir, "leak.txt"))

	root := newOSRoot(t, dir,
		safefs.WithSymlinkPolicy(safefs.FollowInRoot))
	_, err := root.Open("leak.txt")
	if err == nil {
		t.Fatal("Open(leak.txt) succeeded, want error")
	}
	if !errors.Is(err, safefs.ErrSymlinkDenied) {
		t.Fatalf("error = %v, want wrapping ErrSymlinkDenied", err)
	}
}

// TestFollowInRoot_AbsoluteTargetOutside verifies the absolute
// escape case. Unlike the in-root absolute case above, the reason
// for refusal here is obvious; the test asserts the same sentinel
// is returned.
func TestFollowInRoot_AbsoluteTargetOutside(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "secret.txt"), "sensitive")
	symlink(t, filepath.Join(outside, "secret.txt"),
		filepath.Join(dir, "leak.txt"))

	root := newOSRoot(t, dir,
		safefs.WithSymlinkPolicy(safefs.FollowInRoot))
	_, err := root.Open("leak.txt")
	if err == nil {
		t.Fatal("Open(leak.txt) succeeded, want error")
	}
	if !errors.Is(err, safefs.ErrSymlinkDenied) {
		t.Fatalf("error = %v, want wrapping ErrSymlinkDenied", err)
	}
}

// TestFollowInRoot_RelativeTargetWithParentSegment verifies the
// case where a relative target escapes via ".." segments that are
// not obvious until cleaning. This is the case a naive
// implementation that checks only the raw target string would
// miss.
//
// The link is at "a/b/c/link.txt" and its target is four ".."
// segments followed by the outside directory's base name. From
// the link's parent ("a/b/c"), four ".." segments reach the
// parent of "dir" — that is, the common parent of "dir" and
// "outside". The link resolves to outside/secret.txt, which
// escapes the root.
func TestFollowInRoot_RelativeTargetWithParentSegment(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "a", "b", "c"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(outside, "secret.txt"), "sensitive")
	relTarget := filepath.Join("..", "..", "..", "..",
		filepath.Base(outside), "secret.txt")
	symlink(t, relTarget, filepath.Join(dir, "a", "b", "c", "link.txt"))

	root := newOSRoot(t, dir,
		safefs.WithSymlinkPolicy(safefs.FollowInRoot))
	_, err := root.Open("a/b/c/link.txt")
	if err == nil {
		t.Fatal("Open succeeded for escaping target, want error")
	}
	if !errors.Is(err, safefs.ErrSymlinkDenied) {
		t.Fatalf("error = %v, want wrapping ErrSymlinkDenied", err)
	}
}

// -----------------------------------------------------------------------------
// Chains and cycles
// -----------------------------------------------------------------------------

// TestFollowInRoot_Chain resolves a chain of in-root links and
// asserts the terminal target's content.
func TestFollowInRoot_Chain(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "target.txt"), "hello")
	symlink(t, "target.txt", filepath.Join(dir, "l1.txt"))
	symlink(t, "l1.txt", filepath.Join(dir, "l2.txt"))
	symlink(t, "l2.txt", filepath.Join(dir, "l3.txt"))

	root := newOSRoot(t, dir,
		safefs.WithSymlinkPolicy(safefs.FollowInRoot))
	got, err := root.ReadFile("l3.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("ReadFile = %q, want %q", got, "hello")
	}
}

// TestFollowInRoot_ChainAtBoundary verifies the case where a
// chain's length is exactly MaxSymlinkChain: the resolution must
// succeed. The comparison in followInRoot is strict (>), so a
// chain of N links with a bound of N succeeds.
func TestFollowInRoot_ChainAtBoundary(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "target.txt"), "hello")
	symlink(t, "target.txt", filepath.Join(dir, "l1.txt"))
	symlink(t, "l1.txt", filepath.Join(dir, "l2.txt"))

	root := newOSRoot(t, dir,
		safefs.WithSymlinkPolicy(safefs.FollowInRoot),
		safefs.WithMaxSymlinkChain(2),
	)
	if _, err := root.ReadFile("l2.txt"); err != nil {
		t.Fatalf("ReadFile at chain boundary: %v", err)
	}
}

// TestFollowInRoot_ChainOverBoundary verifies the case one link
// over the bound: the resolution must be refused.
func TestFollowInRoot_ChainOverBoundary(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "target.txt"), "hello")
	symlink(t, "target.txt", filepath.Join(dir, "l1.txt"))
	symlink(t, "l1.txt", filepath.Join(dir, "l2.txt"))
	symlink(t, "l2.txt", filepath.Join(dir, "l3.txt"))

	root := newOSRoot(t, dir,
		safefs.WithSymlinkPolicy(safefs.FollowInRoot),
		safefs.WithMaxSymlinkChain(2),
	)
	_, err := root.ReadFile("l3.txt")
	if err == nil {
		t.Fatal("ReadFile succeeded, want ErrSymlinkDenied")
	}
	if !errors.Is(err, safefs.ErrSymlinkDenied) {
		t.Fatalf("error = %v, want wrapping ErrSymlinkDenied", err)
	}
}

// TestFollowInRoot_ZeroChainIsUnbounded verifies that zero
// disables the chain bound. A chain longer than the default is
// resolved.
func TestFollowInRoot_ZeroChainIsUnbounded(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "target.txt"), "hello")

	prev := "target.txt"
	for i := 0; i < 100; i++ {
		name := "l" + itoa(i) + ".txt"
		symlink(t, prev, filepath.Join(dir, name))
		prev = name
	}

	root := newOSRoot(t, dir,
		safefs.WithSymlinkPolicy(safefs.FollowInRoot),
		safefs.WithMaxSymlinkChain(0),
	)
	if _, err := root.ReadFile(prev); err != nil {
		t.Fatalf("ReadFile with unbounded chain: %v", err)
	}
}

// TestFollowInRoot_SelfCycle verifies a self-referential link is
// bounded by MaxSymlinkChain rather than looping forever.
func TestFollowInRoot_SelfCycle(t *testing.T) {
	dir := t.TempDir()
	symlink(t, "self.txt", filepath.Join(dir, "self.txt"))

	root := newOSRoot(t, dir,
		safefs.WithSymlinkPolicy(safefs.FollowInRoot),
		safefs.WithMaxSymlinkChain(4),
	)
	_, err := root.ReadFile("self.txt")
	if err == nil {
		t.Fatal("ReadFile succeeded, want error")
	}
	if !errors.Is(err, safefs.ErrSymlinkDenied) {
		t.Fatalf("error = %v, want wrapping ErrSymlinkDenied", err)
	}
}

// TestFollowInRoot_TwoLinkCycle verifies a mutual cycle is
// bounded.
func TestFollowInRoot_TwoLinkCycle(t *testing.T) {
	dir := t.TempDir()
	symlink(t, "b.txt", filepath.Join(dir, "a.txt"))
	symlink(t, "a.txt", filepath.Join(dir, "b.txt"))

	root := newOSRoot(t, dir,
		safefs.WithSymlinkPolicy(safefs.FollowInRoot),
		safefs.WithMaxSymlinkChain(4),
	)
	_, err := root.ReadFile("a.txt")
	if err == nil {
		t.Fatal("ReadFile succeeded, want error")
	}
	if !errors.Is(err, safefs.ErrSymlinkDenied) {
		t.Fatalf("error = %v, want wrapping ErrSymlinkDenied", err)
	}
}

// -----------------------------------------------------------------------------
// Interaction with the other Root methods
// -----------------------------------------------------------------------------

// TestNoFollow_StatRejectsLink verifies that Stat enforces the
// same policy as Open. Every read method goes through
// resolveSymlinks; Stat is no exception.
func TestNoFollow_StatRejectsLink(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "real.txt"), "x")
	symlink(t, "real.txt", filepath.Join(dir, "link.txt"))

	root := newOSRoot(t, dir)
	_, err := root.Stat("link.txt")
	if err == nil {
		t.Fatal("Stat succeeded, want error")
	}
	if !errors.Is(err, safefs.ErrSymlinkDenied) {
		t.Fatalf("error = %v, want wrapping ErrSymlinkDenied", err)
	}
}

// TestNoFollow_ReadDirRejectsLinkDirectory verifies ReadDir is
// subject to the same policy.
func TestNoFollow_ReadDirRejectsLinkDirectory(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	symlink(t, "sub", filepath.Join(dir, "linkdir"))

	root := newOSRoot(t, dir)
	_, err := root.ReadDir("linkdir")
	if err == nil {
		t.Fatal("ReadDir succeeded, want error")
	}
	if !errors.Is(err, safefs.ErrSymlinkDenied) {
		t.Fatalf("error = %v, want wrapping ErrSymlinkDenied", err)
	}
}

// TestFollowInRoot_ReadDirResolvesLinkDirectory verifies ReadDir
// resolves in-root link directories under FollowInRoot.
func TestFollowInRoot_ReadDirResolvesLinkDirectory(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(sub, "a.txt"), "x")
	symlink(t, "sub", filepath.Join(dir, "linkdir"))

	root := newOSRoot(t, dir,
		safefs.WithSymlinkPolicy(safefs.FollowInRoot))
	entries, err := root.ReadDir("linkdir")
	if err != nil {
		t.Fatalf("ReadDir(linkdir): %v", err)
	}
	assertEntriesEqual(t, entries, "a.txt")
}

// TestNoFollow_WalkDoesNotDescendLinkDirectory verifies the
// documented Walk contract: directory symlinks are not descended,
// regardless of policy. The link is delivered to the callback as
// a non-directory entry.
func TestNoFollow_WalkDoesNotDescendLinkDirectory(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "secret.txt"), "sensitive")
	symlink(t, outside, filepath.Join(dir, "linkdir"))
	writeFile(t, filepath.Join(dir, "a.txt"), "x")

	root := newOSRoot(t, dir)
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
	if _, ok := seen["linkdir/secret.txt"]; ok {
		t.Fatal("Walk descended into linkdir")
	}
	if isDir, ok := seen["linkdir"]; !ok {
		t.Fatal("Walk did not visit linkdir")
	} else if isDir {
		t.Fatal("Walk reported linkdir as a directory")
	}
}

// TestFollowInRoot_WalkStillDoesNotDescendLinkDirectory verifies
// that even under FollowInRoot, Walk does not descend into a
// directory symlink. The rationale is in walk.go: descent through
// links turns the traversal into a graph and defeats the bounds.
func TestFollowInRoot_WalkStillDoesNotDescendLinkDirectory(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(sub, "a.txt"), "x")
	symlink(t, "sub", filepath.Join(dir, "linkdir"))

	root := newOSRoot(t, dir,
		safefs.WithSymlinkPolicy(safefs.FollowInRoot))
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
	if _, ok := seen["linkdir/a.txt"]; ok {
		t.Fatal("Walk descended into linkdir under FollowInRoot")
	}
	if isDir, ok := seen["linkdir"]; !ok {
		t.Fatal("Walk did not visit linkdir")
	} else if isDir {
		t.Fatal("Walk reported linkdir as a directory")
	}
}

// TestNoFollow_WalkDeliversSymlinkToFile verifies that a symlink
// to a file is delivered to the callback as a non-directory
// entry, and that Walk continues past it.
func TestNoFollow_WalkDeliversSymlinkToFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "real.txt"), "x")
	symlink(t, "real.txt", filepath.Join(dir, "link.txt"))

	root := newOSRoot(t, dir)
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
	if _, ok := seen["link.txt"]; !ok {
		t.Fatal("Walk did not visit link.txt")
	}
	if isDir := seen["link.txt"]; isDir {
		t.Fatal("Walk reported link.txt as a directory")
	}
}

// -----------------------------------------------------------------------------
// Non-functional
// -----------------------------------------------------------------------------

// TestFollowInRoot_ConcurrentResolution verifies that the symlink
// cache is safe under concurrent use, and that two goroutines
// resolving the same link see the same target.
//
// Run with -race.
func TestFollowInRoot_ConcurrentResolution(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "target.txt"), "hello")
	symlink(t, "target.txt", filepath.Join(dir, "link.txt"))

	root := newOSRoot(t, dir,
		safefs.WithSymlinkPolicy(safefs.FollowInRoot))

	const goroutines = 32
	errs := make(chan error, goroutines)
	done := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			got, err := root.ReadFile("link.txt")
			if err != nil {
				errs <- err
				return
			}
			if string(got) != "hello" {
				errs <- errors.New("wrong content")
				return
			}
		}()
	}
	for i := 0; i < goroutines; i++ {
		<-done
	}
	close(errs)
	for err := range errs {
		t.Errorf("goroutine error: %v", err)
	}
}

// -----------------------------------------------------------------------------
// Helpers local to this file
// -----------------------------------------------------------------------------

// dirEntryNames returns the names of entries. It is local to this
// file to avoid a cross-file dependency on the helper in
// root_test.go; the shared helper file described in the package
// notes would remove the duplication.
func dirEntryNames(entries []fs.DirEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Name()
	}
	return out
}

// Silence an unused-import warning if runtime is not used in this
// file. It is imported to document the build tag's intent and will
// be used the moment a platform-specific behaviour is added.
var _ = runtime.GOOS
