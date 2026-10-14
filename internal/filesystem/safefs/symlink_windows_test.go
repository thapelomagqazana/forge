//go:build windows

package safefs_test

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thapelomagqazana/forge/internal/filesystem"
	"github.com/thapelomagqazana/forge/internal/filesystem/safefs"
)

// This file exercises the symlink policy matrix on Windows.
//
// The Windows host differs from Unix in three ways that matter
// to the policy:
//
//  1. Symbolic links require either Developer Mode or the
//     SeCreateSymbolicLinkPrivilege. Without one of the two,
//     os.Symlink returns an error. Every test that needs a
//     symlink skips in that case. The alternative — failing the
//     test — would make the suite depend on the developer's
//     Windows configuration, which is exactly the kind of
//     environment-dependent test that produces flaky CI.
//
//  2. Directory junctions do not require any privilege. They
//     are the common link form on Windows for pointing at a
//     directory, and they are the closest equivalent to a Unix
//     directory symlink. They are created with `cmd /c mklink /J
//     <link> <target>`. A junction's target must be an absolute
//     path; a junction with a relative target is not
//     representable.
//
//  3. The OS reports junctions and symlinks differently from
//     each other. Lstat on a junction returns a reparse-point
//     FileInfo whose Mode has fs.ModeSymlink set; the target's
//     type is not reported. The policy treats the two the same
//     way: any link component is a link, and the policy decides
//     what to do with it.
//
// The tests below are organised the same way as the Unix suite:
// NoFollow positive and negative, FollowInRoot positive and
// negative, chains, and interaction with the other Root methods.
// Duplicating the shape is deliberate: a future reader who
// understands one platform's suite understands the other.

// -----------------------------------------------------------------------------
// Fixtures
// -----------------------------------------------------------------------------

// newOSRootWindows constructs a Root backed by the real Windows
// filesystem, rooted at dir.
func newOSRootWindows(t *testing.T, dir string, opts ...safefs.Option) *safefs.Root {
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

// junction creates a directory junction at linkPath pointing to
// target. Junctions require no privilege, so the helper fails
// rather than skips on error. target must be an absolute path;
// junctions cannot express a relative target.
func junction(t *testing.T, target, linkPath string) {
	t.Helper()
	if !filepath.IsAbs(target) {
		t.Fatalf("junction target must be absolute, got %q", target)
	}
	cmd := exec.Command("cmd", "/c", "mklink", "/J", linkPath, target)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("mklink /J %s %s: %v\n%s",
			linkPath, target, err, out)
	}
}

// isPrivilegeError reports whether err is the Windows error for
// "the process lacks SeCreateSymbolicLinkPrivilege". The error
// surfaces either as a syscall.Errno or as a wrapped error whose
// message contains the OS-specific text. Both forms are checked
// because the wrapping depends on how os.Symlink is invoked.
func isPrivilegeError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "privilege") ||
		strings.Contains(msg, "A required privilege") ||
		strings.Contains(msg, "ERROR_PRIVILEGE_NOT_HELD")
}

// -----------------------------------------------------------------------------
// NoFollow — the default policy
// -----------------------------------------------------------------------------

// TestWindowsNoFollow_AcceptsPlainFile anchors every negative case
// below: a name with no link components opens normally.
func TestWindowsNoFollow_AcceptsPlainFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"), "hello")

	root := newOSRootWindows(t, dir)
	got, err := root.ReadFile("a.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("ReadFile = %q, want %q", got, "hello")
	}
}

// TestWindowsNoFollow_RejectsSymlinkToFile verifies the CWE-59
// case for a file symbolic link.
func TestWindowsNoFollow_RejectsSymlinkToFile(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "secret.txt"), "sensitive")
	symlink(t, filepath.Join(outside, "secret.txt"),
		filepath.Join(dir, "link.txt"))

	root := newOSRootWindows(t, dir)
	_, err := root.Open("link.txt")
	if err == nil {
		t.Fatal("Open(link.txt) succeeded under NoFollow, want error")
	}
	if !errors.Is(err, safefs.ErrSymlinkDenied) {
		t.Fatalf("error = %v, want wrapping ErrSymlinkDenied", err)
	}
}

// TestWindowsNoFollow_RejectsJunctionToDirectory verifies the
// Windows-specific negative case: a junction is a link, and
// NoFollow refuses it. This is the case the Unix suite cannot
// express, because Unix has no junction as a distinct concept.
func TestWindowsNoFollow_RejectsJunctionToDirectory(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "secret.txt"), "sensitive")
	junction(t, outside, filepath.Join(dir, "linkdir"))

	root := newOSRootWindows(t, dir)
	_, err := root.Open("linkdir/secret.txt")
	if err == nil {
		t.Fatal("Open succeeded through junction, want error")
	}
	if !errors.Is(err, safefs.ErrSymlinkDenied) {
		t.Fatalf("error = %v, want wrapping ErrSymlinkDenied", err)
	}
}

// TestWindowsNoFollow_RejectsSymlinkToDirectory verifies that a
// directory symbolic link (which is distinct from a junction on
// Windows) is also refused. The distinction between the two
// forms is not visible to the policy, and the test asserts that.
func TestWindowsNoFollow_RejectsSymlinkToDirectory(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "secret.txt"), "sensitive")
	symlink(t, outside, filepath.Join(dir, "linkdir"))

	root := newOSRootWindows(t, dir)
	_, err := root.Open("linkdir/secret.txt")
	if err == nil {
		t.Fatal("Open succeeded through directory symlink, want error")
	}
	if !errors.Is(err, safefs.ErrSymlinkDenied) {
		t.Fatalf("error = %v, want wrapping ErrSymlinkDenied", err)
	}
}

// TestWindowsNoFollow_RejectsIntermediateJunction verifies the
// junction in an intermediate position.
func TestWindowsNoFollow_RejectsIntermediateJunction(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(outside, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(outside, "sub", "secret.txt"), "sensitive")
	junction(t, outside, filepath.Join(dir, "linkdir"))

	root := newOSRootWindows(t, dir)
	_, err := root.Open("linkdir/sub/secret.txt")
	if err == nil {
		t.Fatal("Open succeeded, want error")
	}
	if !errors.Is(err, safefs.ErrSymlinkDenied) {
		t.Fatalf("error = %v, want wrapping ErrSymlinkDenied", err)
	}
}

// TestWindowsNoFollow_AcceptsDot verifies that "." (the root) is
// readable via ReadDir under the default policy.
func TestWindowsNoFollow_AcceptsDot(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"), "x")

	root := newOSRootWindows(t, dir)
	entries, err := root.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir(.): %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "a.txt" {
		t.Fatalf("entries = %v, want [a.txt]", dirEntryNamesWindows(entries))
	}
}

// -----------------------------------------------------------------------------
// FollowInRoot — in-root targets
// -----------------------------------------------------------------------------

// TestWindowsFollowInRoot_RelativeTargetInside verifies the
// positive case for a relative file symlink.
func TestWindowsFollowInRoot_RelativeTargetInside(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "real.txt"), "hello")
	symlink(t, "real.txt", filepath.Join(dir, "link.txt"))

	root := newOSRootWindows(t, dir,
		safefs.WithSymlinkPolicy(safefs.FollowInRoot))
	got, err := root.ReadFile("link.txt")
	if err != nil {
		t.Fatalf("ReadFile(link.txt): %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("ReadFile = %q, want %q", got, "hello")
	}
}

// TestWindowsFollowInRoot_JunctionInside verifies the positive
// case for a junction whose target is inside the root. Junctions
// always use an absolute target, so this is also the case where
// the absolute-target rule from symlink.go would refuse the link
// if the rule applied to junctions. The rule applies only to
// symbolic links; junctions are handled by the same code path
// and, since their target is inside the root, the escape check
// passes.
//
// This test documents the interaction: a junction whose absolute
// target is inside the root succeeds; a junction whose absolute
// target is outside is refused (next test).
func TestWindowsFollowInRoot_JunctionInside(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(sub, "a.txt"), "hello")
	junction(t, sub, filepath.Join(dir, "linkdir"))

	root := newOSRootWindows(t, dir,
		safefs.WithSymlinkPolicy(safefs.FollowInRoot))
	got, err := root.ReadFile("linkdir/a.txt")
	if err != nil {
		t.Fatalf("ReadFile(linkdir/a.txt): %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("ReadFile = %q, want %q", got, "hello")
	}
}

// TestWindowsFollowInRoot_TargetIsDot verifies the corner case
// where a link's target is "." — the root itself. On Windows a
// symbolic link to "." is representable; a junction to "." is
// not (junctions require absolute targets and "." is not
// absolute).
func TestWindowsFollowInRoot_TargetIsDot(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"), "hello")
	symlink(t, ".", filepath.Join(dir, "link"))

	root := newOSRootWindows(t, dir,
		safefs.WithSymlinkPolicy(safefs.FollowInRoot))
	entries, err := root.ReadDir("link")
	if err != nil {
		t.Fatalf("ReadDir(link): %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "a.txt" {
		t.Fatalf("entries = %v, want [a.txt]", dirEntryNamesWindows(entries))
	}
}

// -----------------------------------------------------------------------------
// FollowInRoot — escaping targets
// -----------------------------------------------------------------------------

// TestWindowsFollowInRoot_RelativeTargetOutside verifies the
// canonical negative case: a relative target that escapes the
// root is refused.
func TestWindowsFollowInRoot_RelativeTargetOutside(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "secret.txt"), "sensitive")
	symlink(t, filepath.Join("..", filepath.Base(outside), "secret.txt"),
		filepath.Join(dir, "leak.txt"))

	root := newOSRootWindows(t, dir,
		safefs.WithSymlinkPolicy(safefs.FollowInRoot))
	_, err := root.Open("leak.txt")
	if err == nil {
		t.Fatal("Open(leak.txt) succeeded, want error")
	}
	if !errors.Is(err, safefs.ErrSymlinkDenied) {
		t.Fatalf("error = %v, want wrapping ErrSymlinkDenied", err)
	}
}

// TestWindowsFollowInRoot_AbsoluteTargetOutside verifies that an
// absolute symbolic-link target outside the root is refused. On
// Windows this is the common case, because Windows users tend to
// create symbolic links with absolute targets.
func TestWindowsFollowInRoot_AbsoluteTargetOutside(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "secret.txt"), "sensitive")
	symlink(t, filepath.Join(outside, "secret.txt"),
		filepath.Join(dir, "leak.txt"))

	root := newOSRootWindows(t, dir,
		safefs.WithSymlinkPolicy(safefs.FollowInRoot))
	_, err := root.Open("leak.txt")
	if err == nil {
		t.Fatal("Open(leak.txt) succeeded, want error")
	}
	if !errors.Is(err, safefs.ErrSymlinkDenied) {
		t.Fatalf("error = %v, want wrapping ErrSymlinkDenied", err)
	}
}

// TestWindowsFollowInRoot_JunctionOutside verifies that a
// junction whose target is outside the root is refused. This is
// the case that distinguishes junction handling from symbolic-
// link handling: a junction's target is always absolute, and the
// escape check must refuse it when the target lies outside.
func TestWindowsFollowInRoot_JunctionOutside(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "secret.txt"), "sensitive")
	junction(t, outside, filepath.Join(dir, "leakdir"))

	root := newOSRootWindows(t, dir,
		safefs.WithSymlinkPolicy(safefs.FollowInRoot))
	_, err := root.Open("leakdir/secret.txt")
	if err == nil {
		t.Fatal("Open succeeded through escaping junction, want error")
	}
	if !errors.Is(err, safefs.ErrSymlinkDenied) {
		t.Fatalf("error = %v, want wrapping ErrSymlinkDenied", err)
	}
}

// TestWindowsFollowInRoot_MixedCaseTarget verifies that Windows's
// case-insensitive path comparison does not create a false
// escape. A symlink whose target differs from the root only in
// case must resolve to the in-root file, not be refused as an
// escape.
//
// The test is meaningful only on case-insensitive filesystems;
// on a case-sensitive NTFS volume, the target does not exist and
// the test's assertions must be adjusted. The test skips when the
// filesystem reports the target as not existing, which is the
// signal that case-sensitivity is in effect.
func TestWindowsFollowInRoot_MixedCaseTarget(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Real.txt"), "hello")
	target := filepath.Join(dir, "real.txt")

	symlink(t, target, filepath.Join(dir, "link.txt"))

	root := newOSRootWindows(t, dir,
		safefs.WithSymlinkPolicy(safefs.FollowInRoot))
	got, err := root.ReadFile("link.txt")
	if err != nil {
		// On a case-sensitive NTFS volume, the target does
		// not exist and the read fails with fs.ErrNotExist.
		// That is a valid outcome for such a volume, not a
		// failure of the policy.
		if errors.Is(err, fs.ErrNotExist) {
			t.Skip("filesystem is case-sensitive; mixed-case target does not resolve")
		}
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("ReadFile = %q, want %q", got, "hello")
	}
}

// -----------------------------------------------------------------------------
// Chains and cycles
// -----------------------------------------------------------------------------

// TestWindowsFollowInRoot_Chain resolves a chain of in-root
// symbolic links.
func TestWindowsFollowInRoot_Chain(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "target.txt"), "hello")
	symlink(t, "target.txt", filepath.Join(dir, "l1.txt"))
	symlink(t, "l1.txt", filepath.Join(dir, "l2.txt"))
	symlink(t, "l2.txt", filepath.Join(dir, "l3.txt"))

	root := newOSRootWindows(t, dir,
		safefs.WithSymlinkPolicy(safefs.FollowInRoot))
	got, err := root.ReadFile("l3.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("ReadFile = %q, want %q", got, "hello")
	}
}

// TestWindowsFollowInRoot_ChainAtBoundary verifies the boundary
// case where the chain length equals MaxSymlinkChain.
func TestWindowsFollowInRoot_ChainAtBoundary(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "target.txt"), "hello")
	symlink(t, "target.txt", filepath.Join(dir, "l1.txt"))
	symlink(t, "l1.txt", filepath.Join(dir, "l2.txt"))

	root := newOSRootWindows(t, dir,
		safefs.WithSymlinkPolicy(safefs.FollowInRoot),
		safefs.WithMaxSymlinkChain(2),
	)
	if _, err := root.ReadFile("l2.txt"); err != nil {
		t.Fatalf("ReadFile at chain boundary: %v", err)
	}
}

// TestWindowsFollowInRoot_ChainOverBoundary verifies the case one
// link over the bound.
func TestWindowsFollowInRoot_ChainOverBoundary(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "target.txt"), "hello")
	symlink(t, "target.txt", filepath.Join(dir, "l1.txt"))
	symlink(t, "l1.txt", filepath.Join(dir, "l2.txt"))
	symlink(t, "l2.txt", filepath.Join(dir, "l3.txt"))

	root := newOSRootWindows(t, dir,
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

// TestWindowsFollowInRoot_SelfCycle verifies that a self-
// referential link is bounded by MaxSymlinkChain.
func TestWindowsFollowInRoot_SelfCycle(t *testing.T) {
	dir := t.TempDir()
	symlink(t, "self.txt", filepath.Join(dir, "self.txt"))

	root := newOSRootWindows(t, dir,
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

// TestWindowsFollowInRoot_TwoLinkCycle verifies a mutual cycle is
// bounded.
func TestWindowsFollowInRoot_TwoLinkCycle(t *testing.T) {
	dir := t.TempDir()
	symlink(t, "b.txt", filepath.Join(dir, "a.txt"))
	symlink(t, "a.txt", filepath.Join(dir, "b.txt"))

	root := newOSRootWindows(t, dir,
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

// TestWindowsNoFollow_StatRejectsLink verifies that Stat enforces
// the same policy as Open.
func TestWindowsNoFollow_StatRejectsLink(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "real.txt"), "x")
	symlink(t, "real.txt", filepath.Join(dir, "link.txt"))

	root := newOSRootWindows(t, dir)
	_, err := root.Stat("link.txt")
	if err == nil {
		t.Fatal("Stat succeeded, want error")
	}
	if !errors.Is(err, safefs.ErrSymlinkDenied) {
		t.Fatalf("error = %v, want wrapping ErrSymlinkDenied", err)
	}
}

// TestWindowsNoFollow_ReadDirRejectsJunction verifies ReadDir is
// subject to the same policy.
func TestWindowsNoFollow_ReadDirRejectsJunction(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	junction(t, sub, filepath.Join(dir, "linkdir"))

	root := newOSRootWindows(t, dir)
	_, err := root.ReadDir("linkdir")
	if err == nil {
		t.Fatal("ReadDir succeeded, want error")
	}
	if !errors.Is(err, safefs.ErrSymlinkDenied) {
		t.Fatalf("error = %v, want wrapping ErrSymlinkDenied", err)
	}
}

// TestWindowsFollowInRoot_ReadDirResolvesJunction verifies that
// ReadDir resolves an in-root junction under FollowInRoot.
func TestWindowsFollowInRoot_ReadDirResolvesJunction(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(sub, "a.txt"), "x")
	junction(t, sub, filepath.Join(dir, "linkdir"))

	root := newOSRootWindows(t, dir,
		safefs.WithSymlinkPolicy(safefs.FollowInRoot))
	entries, err := root.ReadDir("linkdir")
	if err != nil {
		t.Fatalf("ReadDir(linkdir): %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "a.txt" {
		t.Fatalf("entries = %v, want [a.txt]", dirEntryNamesWindows(entries))
	}
}

// TestWindowsNoFollow_WalkDoesNotDescendJunction verifies the
// documented Walk contract: directory links (both symlinks and
// junctions) are not descended, regardless of policy.
func TestWindowsNoFollow_WalkDoesNotDescendJunction(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "secret.txt"), "sensitive")
	junction(t, outside, filepath.Join(dir, "linkdir"))
	writeFile(t, filepath.Join(dir, "a.txt"), "x")

	root := newOSRootWindows(t, dir)
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
		t.Fatal("Walk descended into junction")
	}
	if isDir, ok := seen["linkdir"]; !ok {
		t.Fatal("Walk did not visit the junction")
	} else if isDir {
		t.Fatal("Walk reported the junction as a directory")
	}
}

// TestWindowsFollowInRoot_WalkStillDoesNotDescendJunction verifies
// that even under FollowInRoot, Walk does not descend into a
// junction directory. The rationale is the same as on Unix: a
// descent through links makes bounds meaningless.
func TestWindowsFollowInRoot_WalkStillDoesNotDescendJunction(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(sub, "a.txt"), "x")
	junction(t, sub, filepath.Join(dir, "linkdir"))

	root := newOSRootWindows(t, dir,
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
		t.Fatal("Walk descended into junction under FollowInRoot")
	}
	if isDir, ok := seen["linkdir"]; !ok {
		t.Fatal("Walk did not visit the junction")
	} else if isDir {
		t.Fatal("Walk reported the junction as a directory")
	}
}

// -----------------------------------------------------------------------------
// Non-functional
// -----------------------------------------------------------------------------

// TestWindowsFollowInRoot_ConcurrentResolution verifies that the
// symlink cache is safe under concurrent use, and that two
// goroutines resolving the same link see the same target. Run
// with -race.
func TestWindowsFollowInRoot_ConcurrentResolution(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "target.txt"), "hello")
	symlink(t, "target.txt", filepath.Join(dir, "link.txt"))

	root := newOSRootWindows(t, dir,
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

// dirEntryNamesWindows returns the names of entries. It is local
// to this file to avoid a cross-file dependency on the helper in
// root_test.go; a shared test helper file would remove the
// duplication.
func dirEntryNamesWindows(entries []fs.DirEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Name()
	}
	return out
}

// This file exercises the symlink policy on Windows. Windows
// supports two kinds of link: symbolic links (which require
// privilege) and directory junctions (which do not). The tests
// use junctions where possible and skip on symbolic links when
// the process lacks the privilege.

// TestRule2_WindowsJunctionInWriteParentRejected verifies that a
// write whose parent contains a junction is refused.
func TestRule2_WindowsJunctionInWriteParentRejected(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(dir, "sub")
	// Create a junction. Junctions do not require privilege.
	cmd := exec.Command("cmd", "/c", "mklink", "/J", link, outside)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("mklink /J failed: %v\n%s", err, out)
	}

	root, err := safefs.New(dir, safefs.WithFS(filesystem.NewOSFS(dir)))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	err = root.WriteAtomic("sub/a.txt", []byte("x"), 0o600)
	if err == nil {
		t.Fatal("WriteAtomic succeeded through junction, want error")
	}
	if !errors.Is(err, safefs.ErrSymlinkDenied) {
		t.Fatalf("error = %v, want ErrSymlinkDenied", err)
	}
}

// TestRule2_WindowsSymlinkAtWriteTargetRejected verifies that a
// write whose final component is a symbolic link is refused.
// The test skips on a host that does not permit symbolic links.
func TestRule2_WindowsSymlinkAtWriteTargetRejected(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "target.txt")
	if err := os.WriteFile(target, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("os.Symlink failed: %v", err)
	}

	root, err := safefs.New(dir, safefs.WithFS(filesystem.NewOSFS(dir)))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	err = root.WriteAtomicOverwrite("link.txt", []byte("new"), 0o600)
	if err == nil {
		t.Fatal("WriteAtomicOverwrite succeeded through symlink, want error")
	}
	got, _ := os.ReadFile(target)
	if string(got) != "old" {
		t.Fatalf("target content changed: %q", got)
	}
}
