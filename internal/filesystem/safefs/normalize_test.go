package safefs_test

import (
	"errors"
	"runtime"
	"testing"

	"github.com/thapelomagqazana/forge/internal/filesystem"
	"github.com/thapelomagqazana/forge/internal/filesystem/safefs"
)

// This file is the executable form of the decision table in
// docs/reference/path-normalization.md. Every row of the table
// has a corresponding test case here; a change to the table
// without a change here (or vice versa) is a spec/code drift
// that the reviewer is expected to catch.
//
// # Structure
//
// The cases are grouped by layer:
//
//   - Layer 1: lexical normalization. Pure string work, no
//     filesystem. These cases run on every platform.
//   - Layer 2: boundary check. Folded into layer 1; tested
//     together.
//   - Layer 3: symlink resolution. Requires a real filesystem;
//     the cases live in symlink_unix_test.go and
//     symlink_windows_test.go and are referenced here by name.
//   - Layer 4: filesystem canonicalization. Not used by safefs;
//     the case is a documentation row, not a test.
//
// # Platform gating
//
// Cases whose decision depends on the platform are tagged with
// the platform they apply to. The test skips on the other
// platforms. A Windows-specific case that runs on Linux would
// test nothing (the input is legal on Linux) and would give a
// false sense of coverage.
//
// # What this file does not test
//
// It does not test the symlink resolution cases (rows 35–39 of
// the decision table). Those live in the platform-specific
// symlink test files. This file references them so that a
// reader tracing the table to the code finds them.

// decisionCase is one row of the decision table.
//
// The struct's fields mirror the columns: input, layer,
// platform, decision. The "wantAccept" field is the boolean
// form of the decision; the "wantErr" field names the sentinel
// the decision wraps when the decision is a rejection.
type decisionCase struct {
	name       string // descriptive name for the subtest
	input      string // the caller-supplied name
	platform   string // "", "windows", "darwin", or "linux"
	wantAccept bool   // true if the name is accepted
	wantErr    error  // the sentinel the rejection wraps, or nil
}

// layer1Cases are the lexical-normalization rows of the decision
// table (rows 1–18, 21–34). The platform column gates the rows
// that depend on the platform.
var layer1Cases = []decisionCase{
	// Row 1
	{name: "empty", input: "", wantAccept: false, wantErr: safefs.ErrNotLocal},
	// Row 2
	{name: "dot", input: ".", wantAccept: true},
	// Row 3
	{name: "simple", input: "a", wantAccept: true},
	// Row 4
	{name: "nested", input: "a/b", wantAccept: true},
	// Row 5
	{name: "repeated separator", input: "a//b", wantAccept: true},
	// Row 6
	{name: "dot component", input: "a/./b", wantAccept: true},
	// Row 7
	{name: "parent resolves inside", input: "a/../b", wantAccept: true},
	// Row 8
	{name: "trailing parent resolves inside", input: "a/b/..", wantAccept: true},
	// Row 9
	{name: "trailing parent resolves to root", input: "a/..", wantAccept: true},
	// Row 10
	{name: "leading parent", input: "..", wantAccept: false, wantErr: safefs.ErrPathTraversal},
	// Row 11
	{name: "leading parent with child", input: "../a", wantAccept: false, wantErr: safefs.ErrPathTraversal},
	// Row 12
	{name: "parent escapes", input: "a/../../b", wantAccept: false, wantErr: safefs.ErrPathTraversal},
	// Row 13
	{name: "unix absolute", input: "/a", wantAccept: false, wantErr: safefs.ErrNotLocal},
	// Row 14
	{name: "unix root", input: "/", wantAccept: false, wantErr: safefs.ErrNotLocal},
	// Row 15
	{name: "leading dot slash", input: "./a", wantAccept: true},
	// Row 16
	{name: "leading dot slash parent", input: "./../a", wantAccept: false, wantErr: safefs.ErrPathTraversal},
	// Row 17
	{name: "double-dot prefix filename", input: "..foo", wantAccept: true},
	// Row 18
	{name: "double-dot suffix filename", input: "foo..", wantAccept: true},
	// Row 21
	{name: "backslash separator", input: `a\b`, wantAccept: true},
	// Row 22
	{name: "doubled backslash separator", input: `a\\b`, wantAccept: true},

	// Rows 19, 20: trailing dots and spaces. These are accepted
	// on every platform; the OS's behaviour (stripping on
	// Windows, preserving elsewhere) is documented but not
	// asserted here.
	{name: "trailing dot", input: "foo.", wantAccept: true},
	{name: "trailing space", input: "foo ", wantAccept: true},

	// Rows 23–26: Windows absolute forms.
	{name: "windows drive absolute", input: `C:\foo`, platform: "windows", wantAccept: false, wantErr: safefs.ErrNotLocal},
	{name: "windows drive relative", input: `C:foo`, platform: "windows", wantAccept: false, wantErr: safefs.ErrNotLocal},
	{name: "windows drive root", input: `C:\`, platform: "windows", wantAccept: false, wantErr: safefs.ErrNotLocal},
	{name: "windows UNC", input: `\\server\share\a`, platform: "windows", wantAccept: false, wantErr: safefs.ErrNotLocal},

	// Rows 27–34: Windows reserved names.
	{name: "windows reserved CON", input: "CON", platform: "windows", wantAccept: false, wantErr: safefs.ErrNotLocal},
	{name: "windows reserved con.txt", input: "con.txt", platform: "windows", wantAccept: false, wantErr: safefs.ErrNotLocal},
	{name: "windows reserved NUL", input: "NUL", platform: "windows", wantAccept: false, wantErr: safefs.ErrNotLocal},
	{name: "windows reserved AUX", input: "AUX", platform: "windows", wantAccept: false, wantErr: safefs.ErrNotLocal},
	{name: "windows reserved COM1", input: "COM1", platform: "windows", wantAccept: false, wantErr: safefs.ErrNotLocal},
	{name: "windows reserved COM9", input: "COM9", platform: "windows", wantAccept: false, wantErr: safefs.ErrNotLocal},
	{name: "windows reserved LPT1", input: "LPT1", platform: "windows", wantAccept: false, wantErr: safefs.ErrNotLocal},
	{name: "windows non-reserved COM10", input: "COM10", platform: "windows", wantAccept: true},
}

// TestDecisionTable_Layer1 runs the lexical-normalization cases
// against every platform. A case whose platform does not match
// the current host is skipped.
//
// The test constructs a Root over an in-memory FS with a tree
// containing the files the accept cases refer to. The rejection
// cases do not reach the FS; the accept cases do, so the tree
// must contain the accepted paths.
func TestDecisionTable_Layer1(t *testing.T) {
	// The tree contains one file for each accept case whose
	// name is a single component. Multi-component accept cases
	// (a/b, a//b, a/./b, a/../b) also resolve to files; the
	// tree contains the resolved forms.
	tree := map[string]string{
		"a":     "x",
		"a/b":   "x",
		"b":     "x",
		"foo.":  "x",
		"foo ":  "x",
		"..foo": "x",
		"foo..": "x",
		"COM10": "x",
	}
	root := newMemRoot(t, tree)

	for _, tc := range layer1Cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if tc.platform != "" && tc.platform != runtime.GOOS {
				t.Skipf("case is %s-specific; skipping on %s",
					tc.platform, runtime.GOOS)
			}

			_, err := root.Open(tc.input)

			if tc.wantAccept {
				if err != nil {
					// An accept case may fail if the file
					// does not exist in the tree. That is
					// not a rejection by the pipeline; it
					// is a not-found from the FS. The
					// distinction is the point of the
					// test: the pipeline does not reject
					// the name.
					if errors.Is(err, safefs.ErrNotLocal) ||
						errors.Is(err, safefs.ErrPathTraversal) ||
						errors.Is(err, safefs.ErrSymlinkDenied) {
						t.Fatalf("input %q rejected by pipeline: %v",
							tc.input, err)
					}
					// Not-found and other FS errors are
					// acceptable for accept cases.
				}
				return
			}

			if err == nil {
				t.Fatalf("input %q accepted, want rejection %v",
					tc.input, tc.wantErr)
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("input %q error = %v, want wrapping %v",
					tc.input, err, tc.wantErr)
			}
		})
	}
}

// TestDecisionTable_Layer3_Reference is a documentation test.
// It exists so that a reader tracing a decision-table row to the
// code finds the reference. The actual layer-3 cases live in the
// platform-specific symlink test files; this test does not
// exercise them.
func TestDecisionTable_Layer3_Reference(t *testing.T) {
	t.Skip("layer-3 cases live in symlink_unix_test.go and symlink_windows_test.go")
}

// TestDecisionTable_Layer4_Reference is a documentation test.
// Layer 4 is not used by safefs; the row exists in the spec to
// say so. This test does not exercise anything.
func TestDecisionTable_Layer4_Reference(t *testing.T) {
	t.Skip("layer 4 (filesystem canonicalization) is not used by safefs")
}

// Ensure the test file references the option constructors, so
// that a future refactor that moves them does not silently
// change the decision table's applicability. The constructors
// are used by the platform-specific symlink tests; the
// reference here is for documentation.
var (
	_ = safefs.WithSymlinkPolicy
	_ = filesystem.NewMemFS
)
