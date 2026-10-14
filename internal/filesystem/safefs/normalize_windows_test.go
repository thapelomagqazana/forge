//go:build windows

package safefs_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thapelomagqazana/forge/internal/filesystem"
	"github.com/thapelomagqazana/forge/internal/filesystem/safefs"
)

// This file exercises the Windows-specific rows of the decision
// table. The cross-platform rows are in normalize_test.go; the
// platform-specific symlink rows are in
// symlink_windows_test.go.

// TestWindows_ReservedNames asserts that every reserved name is
// rejected at the pipeline, before any syscall.
//
// The list is the full Windows reserved set: the three device
// names and the numbered COM and LPT ports. The names are
// reserved in every directory, with or without an extension.
func TestWindows_ReservedNames(t *testing.T) {
	reserved := []string{
		"CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5",
		"COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5",
		"LPT6", "LPT7", "LPT8", "LPT9",
	}
	dir := t.TempDir()
	root := newOSRoot(t, dir)

	for _, name := range reserved {
		for _, variant := range []string{name, strings.ToLower(name), name + ".txt"} {
			t.Run(variant, func(t *testing.T) {
				_, err := root.Open(variant)
				if err == nil {
					t.Fatalf("Open(%q) succeeded, want error", variant)
				}
				if !errors.Is(err, safefs.ErrNotLocal) {
					t.Fatalf("error = %v, want wrapping ErrNotLocal", err)
				}
			})
		}
	}
}

// TestWindows_CaseInsensitiveResolution asserts that two names
// that differ only by case resolve to the same file on Windows.
//
// The test does not test safefs's behavior; it documents the
// behavior of the underlying OS and asserts that the abstraction
// inherits it.
func TestWindows_CaseInsensitiveResolution(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(dir, "Foo.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	root := newOSRoot(t, dir)

	for _, name := range []string{"Foo.txt", "foo.txt", "FOO.TXT", "fOo.TxT"} {
		t.Run(name, func(t *testing.T) {
			got, err := root.ReadFile(name)
			if err != nil {
				t.Fatalf("ReadFile(%q): %v", name, err)
			}
			if string(got) != "x" {
				t.Fatalf("ReadFile(%q) = %q, want %q", name, got, "x")
			}
		})
	}
}

// TestWindows_DriveLettersAndUNC asserts that the four Windows
// absolute forms are rejected.
func TestWindows_DriveLettersAndUNC(t *testing.T) {
	dir := t.TempDir()
	root := newOSRoot(t, dir)

	cases := []string{
		`C:\foo`,
		`C:foo`,
		`C:\`,
		`\\server\share\a`,
		`\\?\C:\foo`,
	}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := root.Open(name)
			if err == nil {
				t.Fatalf("Open(%q) succeeded, want error", name)
			}
			if !errors.Is(err, safefs.ErrNotLocal) &&
				!errors.Is(err, safefs.ErrPathTraversal) {
				t.Fatalf("error = %v, want ErrNotLocal or ErrPathTraversal", err)
			}
		})
	}
}

// TestWindows_TrailingDotsAndSpaces asserts the documented
// behaviour: the OS strips trailing dots and spaces, so a name
// that ends with them may refer to a different file than the
// caller wrote.
//
// The test is a documentation test, not an assertion about
// safefs. It pins the behaviour so a reader of the decision
// table sees it exercised.
func TestWindows_TrailingDotsAndSpaces(t *testing.T) {
	dir := t.TempDir()

	// Create a file with a trailing dot. The OS stores it
	// without the dot.
	if err := os.WriteFile(
		filepath.Join(dir, "foo."), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	// The file is now accessible as "foo" on Windows.
	if _, err := os.Stat(filepath.Join(dir, "foo")); err != nil {
		t.Fatalf("after creating foo., foo does not exist: %v", err)
	}
}

// Silence the unused-import warnings for the imports the file
// may not use in every configuration.
var (
	_ = errors.Is
	_ = filesystem.NewOSFS
	_ = safefs.NoFollow
)
