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

// This file exercises the Unix-specific rows of the decision
// table in docs/reference/path-normalization.md. The cross-
// platform rows are in normalize_test.go.

// TestUnix_ByteSensitiveNames asserts that two names that differ
// only by case are distinct files on Linux, and that a name with
// a trailing dot is a distinct file from the same name without
// the dot.
//
// The test documents the Unix behaviour: case is significant,
// trailing dots are significant, and a name that is reserved on
// Windows is a legal filename on Unix.
func TestUnix_ByteSensitiveNames(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(dir, "Foo"), []byte("upper"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(dir, "foo"), []byte("lower"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(dir, "bar."), []byte("dotted"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(dir, "CON"), []byte("device"), 0o600); err != nil {
		t.Fatal(err)
	}

	root := newOSRoot(t, dir)

	cases := []struct {
		name string
		want string
	}{
		{"Foo", "upper"},
		{"foo", "lower"},
		{"bar.", "dotted"},
		{"CON", "device"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := root.ReadFile(tc.name)
			if err != nil {
				t.Fatalf("ReadFile(%q): %v", tc.name, err)
			}
			if string(got) != tc.want {
				t.Fatalf("ReadFile(%q) = %q, want %q",
					tc.name, got, tc.want)
			}
		})
	}
}

// TestUnix_UnicodeNamesAreByteDistinct asserts that two names
// that are canonically equivalent but byte-distinct are distinct
// files on Unix.
//
// The test is the Unix counterpart to the macOS NFD/NFC case.
// On Linux, "café" (NFC) and "café" (NFD) are two different
// files; the operating system does not normalize.
func TestUnix_UnicodeNamesAreByteDistinct(t *testing.T) {
	dir := t.TempDir()
	nfc := "caf\u00e9"  // é as a single rune
	nfd := "cafe\u0301" // e + combining acute accent

	if err := os.WriteFile(
		filepath.Join(dir, nfc), []byte("nfc"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(dir, nfd), []byte("nfd"), 0o600); err != nil {
		t.Fatal(err)
	}

	root := newOSRoot(t, dir)

	got, err := root.ReadFile(nfc)
	if err != nil {
		t.Fatalf("ReadFile(nfc): %v", err)
	}
	if string(got) != "nfc" {
		t.Fatalf("ReadFile(nfc) = %q, want %q", got, "nfc")
	}

	got, err = root.ReadFile(nfd)
	if err != nil {
		t.Fatalf("ReadFile(nfd): %v", err)
	}
	if string(got) != "nfd" {
		t.Fatalf("ReadFile(nfd) = %q, want %q", got, "nfd")
	}
}

// Silence the unused-import warning for errors if the file
// above does not otherwise use it.
var _ = errors.Is

// Silence the unused-import warning for filesystem if the file
// above does not otherwise use it.
var _ = filesystem.NewOSFS

// Silence the unused-import warning for safefs if the file
// above does not otherwise use it.
var _ = safefs.NoFollow
