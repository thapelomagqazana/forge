package filesystem

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPrepareForOS_ShortPathUnchanged verifies that the
// preparation step is a no-op for short paths on every platform.
//
// A short path on Unix is returned unchanged by definition. A
// short path on Windows is also returned unchanged: the \\?\
// prefix is applied only when the path exceeds the threshold, and
// applying it unconditionally would change the semantics for
// relative paths and for paths that are already short enough.
func TestPrepareForOS_ShortPathUnchanged(t *testing.T) {
	cases := []string{
		"/a/b/c",
		"a/b/c",
		"C:/Windows/System32",
		`C:\Windows\System32`,
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			// The call goes through the portable wrapper,
			// which dispatches to the platform file.
			got := prepareForOS(in)
			if got != in {
				t.Fatalf("prepareForOS(%q) = %q, want %q", in, got, in)
			}
		})
	}
}

// TestSeparator_MatchesHost asserts that the exported separator
// constant matches the host's filepath.Separator. The test
// documents the value the abstraction guarantees.
func TestSeparator_MatchesHost(t *testing.T) {
	if separator != filepath.Separator {
		t.Fatalf("separator = %q, want %q",
			separator, filepath.Separator)
	}
}

// TestResolve_RoundTripsThroughOS verifies the invariant that
// the prepared path addresses the same file the caller named.
// The test creates a file with a short name and reads it back
// through the abstraction.
func TestResolve_RoundTripsThroughOS(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(dir, "a.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	f := NewOSFS(dir)
	got, err := f.Read("a.txt")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(got) != "x" {
		t.Fatalf("Read = %q, want %q", got, "x")
	}
}

// TestResolve_LongNameOnAllPlatforms verifies that a long
// relative name — longer than the Windows threshold but legal on
// Unix — works on both platforms.
//
// On Unix, the name is passed through unchanged and the
// operating system accepts it. On Windows, the name is prefixed
// with \\?\ and the operating system accepts it. Either way the
// file is created and read back.
//
// The name is 250 characters, which is below PATH_MAX on Unix
// but above the Windows threshold.
func TestResolve_LongNameOnAllPlatforms(t *testing.T) {
	dir := t.TempDir()

	// Build a name that is long enough to trip the Windows
	// threshold but still well under PATH_MAX on Unix. The
	// directory components are small; the file name carries
	// the length.
	name := strings.Repeat("a", 240) + ".txt"

	// Deeply nested directories would exceed NAME_MAX (255
	// per component on most systems) and the boundary + name
	// would exceed the threshold. A single-level name at the
	// root is enough to exercise the preparation step.
	if err := os.WriteFile(
		filepath.Join(dir, name), []byte("long"), 0o600); err != nil {
		t.Skipf("cannot create long name on this host: %v", err)
	}

	f := NewOSFS(dir)
	got, err := f.Read(name)
	if err != nil {
		t.Fatalf("Read(long name): %v", err)
	}
	if string(got) != "long" {
		t.Fatalf("Read = %q, want %q", got, "long")
	}
}
