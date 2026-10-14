//go:build darwin

package safefs_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/thapelomagqazana/forge/internal/filesystem"
	"github.com/thapelomagqazana/forge/internal/filesystem/safefs"
)

// This file exercises the macOS-specific rows of the decision
// table in docs/reference/path-normalization.md. macOS is
// Unix-like but has two quirks that affect path handling:
// case-insensitive APFS by default, and NFD/NFC Unicode
// normalization.
//
// The file is guarded by a build tag so that the darwin-specific
// assertions do not run on Linux or Windows CI runners. On those
// platforms the file is not compiled, and the corresponding rows
// of the decision table are covered by normalize_unix_test.go
// or normalize_windows_test.go.

// TestDarwin_CaseInsensitiveByDefault asserts that two names
// differing only by case resolve to the same file on a default
// APFS volume.
//
// # Why the test probes before asserting
//
// APFS is case-insensitive by default, but a case-sensitive
// variant is available and is what some CI runners use. The
// test cannot know the volume's case-sensitivity without
// probing. It creates a file with a mixed-case name, then
// attempts to stat the lowercased form. If the stat succeeds,
// the volume is case-insensitive and the test proceeds. If it
// fails, the volume is case-sensitive, the assumption does not
// hold, and the test skips rather than fails: the behaviour
// under test is genuinely absent, not broken.
//
// The probe is performed with os.Stat, not with the Root, so
// the test does not depend on the abstraction to determine
// whether the abstraction should be tested.
func TestDarwin_CaseInsensitiveByDefault(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(dir, "Foo.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Probe for case-insensitivity.
	if _, err := os.Stat(filepath.Join(dir, "foo.txt")); err != nil {
		t.Skipf("volume appears case-sensitive; test does not apply: %v", err)
	}

	root, err := safefs.New(dir,
		safefs.WithFS(filesystem.NewOSFS(dir)))
	if err != nil {
		t.Fatalf("safefs.New: %v", err)
	}

	for _, name := range []string{"Foo.txt", "foo.txt", "FOO.TXT"} {
		t.Run(name, func(t *testing.T) {
			got, err := root.ReadFile(name)
			if err != nil {
				t.Fatalf("ReadFile(%q): %v", name, err)
			}
			if string(got) != "x" {
				t.Fatalf("ReadFile(%q) = %q, want %q",
					name, got, "x")
			}
		})
	}
}

// TestDarwin_UnicodeNormalization documents the NFD/NFC
// behaviour on APFS.
//
// # The behaviour under test
//
// HFS+ normalizes filenames to NFD (canonical decomposition)
// before storing them. APFS preserves the caller's form on
// creation but compares using NFC (canonical composition). The
// practical effect is that a file created with an NFC name and
// looked up with an NFD name (or vice versa) may or may not
// resolve, depending on the filesystem and the API.
//
// # Why the test does not assert a specific outcome
//
// The outcome depends on the volume's configuration, which the
// test cannot control. A test that asserted "NFD resolves" would
// fail on volumes where it does not; a test that asserted "NFD
// does not resolve" would fail on volumes where it does. The
// honest assertion is that the NFC form resolves (which is what
// the caller wrote and what APFS stores) and that the NFD form
// either resolves to the same content or does not resolve at
// all. Anything else would be asserting a property of the
// volume, not of safefs.
//
// # Why this test is in the safefs package's test suite
//
// safefs does not normalize Unicode; the OS does. The test
// documents the OS's behaviour so that a reader of the decision
// table's row 40 finds the corresponding exercise. It does not
// test safefs's code; it tests the environment safefs operates
// in.
func TestDarwin_UnicodeNormalization(t *testing.T) {
	dir := t.TempDir()
	nfc := "caf\u00e9"  // é as a single rune
	nfd := "cafe\u0301" // e + combining acute accent

	if err := os.WriteFile(
		filepath.Join(dir, nfc), []byte("nfc"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Reading the same name works. This is the primary
	// assertion: the caller's form is what the caller gets.
	got, err := os.ReadFile(filepath.Join(dir, nfc))
	if err != nil {
		t.Fatalf("ReadFile(nfc): %v", err)
	}
	if string(got) != "nfc" {
		t.Fatalf("ReadFile(nfc) = %q, want %q", got, "nfc")
	}

	// Reading the NFD form may or may not work. On APFS, the
	// file created with NFC is stored under NFD internally,
	// so the NFD form resolves. On a case-sensitive APFS
	// volume or a filesystem that preserves form, it may not.
	// Either outcome is acceptable; the test logs the answer.
	got2, err2 := os.ReadFile(filepath.Join(dir, nfd))
	if err2 != nil {
		t.Logf("NFD form does not resolve on this volume: %v", err2)
		return
	}
	// If it resolves, the content must be the same file.
	if string(got2) != "nfc" {
		t.Fatalf("ReadFile(nfd) = %q, want %q (same file as nfc)",
			got2, "nfc")
	}
}
