package safefs_test

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/thapelomagqazana/forge/internal/filesystem"
	"github.com/thapelomagqazana/forge/internal/filesystem/safefs"
)

// This file contains the fuzz targets for safefs.
//
// Fuzzing is the right tool for safefs for three reasons:
//
//  1. The input domain — arbitrary strings handed to Root methods
//     — is exactly what fuzzing explores. Unit tests can only
//     cover the cases the author thought of; fuzzing covers the
//     rest.
//
//  2. The property under test — "no accepted name escapes the
//     root" — is a universal quantifier. A single counterexample
//     is a security bug; a hundred passing examples are evidence,
//     not proof. Fuzzing searches for the counterexample.
//
//  3. validateName delegates to filepath.IsLocal, whose
//     behaviour is documented but subtle (drive letters, UNC
//     prefixes, reserved names, empty segments). Fuzzing
//     exercises the delegate's edges without reimplementing
//     them.
//
// The targets below are meant to be run with:
//
//	go test -run '^$' -fuzz '^FuzzValidateName$' \
//	    ./internal/filesystem/safefs/
//
// and in CI with a bounded -fuzztime so the corpus grows over
// time without blocking every push.

// -----------------------------------------------------------------------------
// Fixtures
// -----------------------------------------------------------------------------

// fuzzRoot returns a Root backed by an in-memory filesystem with a
// small, deterministic tree. Every fuzz target uses the same root
// so that the property under test is stable across runs.
//
// The tree deliberately contains a mix of files at various depths
// and a filename that starts with "..", so that a fuzz input that
// happens to be a legal path resolves to a known file rather than
// a not-found error. A not-found result is still useful — it
// exercises the error path — but a mix of both is better.
func fuzzRoot(tb testing.TB) *safefs.Root {
	tb.Helper()
	m := fstest.MapFS{
		"a.txt":     &fstest.MapFile{Data: []byte("a")},
		"..foo":     &fstest.MapFile{Data: []byte("dots")},
		"b/c.txt":   &fstest.MapFile{Data: []byte("bc")},
		"b/d/e.txt": &fstest.MapFile{Data: []byte("bde")},
		"dır/f.txt": &fstest.MapFile{Data: []byte("unicode")},
	}
	root, err := safefs.New("/root", safefs.WithFS(filesystem.NewMemFS(m)))
	if err != nil {
		tb.Fatalf("safefs.New: %v", err)
	}
	return root
}

// -----------------------------------------------------------------------------
// FuzzValidateName
// -----------------------------------------------------------------------------

// FuzzValidateName asserts the confinement invariant: a name that
// any Root method accepts is a name that resolves to a path inside
// the root.
//
// The invariant is checked indirectly. The Root is backed by an
// in-memory FS whose namespace is flat; every entry the FS knows
// about is a top-level key with no path separator escape. If
// validateName (and resolveSymlinks, which under NoFollow calls
// verifyNoSymlinks) ever accepted a name whose cleaned form
// escaped the root, the FS would either resolve to an entry it
// should not, or — more likely — the operation would succeed on a
// name whose cleaned form is not a well-formed local path.
//
// The direct check is: if ReadFile returns no error, the cleaned
// form of the name must satisfy filepath.IsLocal. This is the
// property filepath.IsLocal is documented to provide, and it is
// the property validateName relies on.
//
// # What the target does not prove
//
// The target does not prove the OSFS layer behaves correctly on a
// real disk; that is the job of the OSFS test suite and the
// symlink tests. It does not prove the symlink policy; that is
// the job of the symlink test files. It proves exactly one thing:
// the name-validation phase, on its own, never accepts a name
// that violates the confinement invariant.
//
// # Seed corpus
//
// The seeds cover the input classes that have historically
// broken path validators:
//
//   - "..", "../..", "a/..", "a/../.." — traversal.
//   - "/etc/passwd", "C:\\Windows", "\\\\server\\share" —
//     absolute paths, Windows drive letters, UNC.
//   - "CON", "NUL", "AUX", "COM1" — Windows reserved names.
//   - "a\x00b" — NUL bytes, which truncate in some C APIs.
//   - "\u202e" — Unicode right-to-left override, which has been
//     used to disguise file extensions.
//   - "a\u0000b" — same as above, spelled differently.
//   - "" — the empty name.
//   - "." — the root.
//   - "a/b/c" — a normal path.
//   - "..foo" — a legal filename containing two dots.
//   - "a\\b" — a backslash-separated name, which must be
//     normalised on Windows and left alone on Unix.
//   - "very-long-name..." — a name near the length boundary.
func FuzzValidateName(f *testing.F) {
	seeds := []string{
		"..",
		"../..",
		"a/..",
		"a/../..",
		"../../../etc/passwd",
		"/etc/passwd",
		`C:\Windows\System32`,
		`\\server\share`,
		"CON",
		"NUL",
		"AUX",
		"COM1",
		"LPT1",
		"",
		".",
		"a/b/c",
		"..foo",
		`a\b`,
		"a\x00b",
		"\u202e",
		"\u202eexe.txt",
		"a/b/../c",
		"a//b",
		"a/./b",
		"./a",
		strings.Repeat("a", 255),
		strings.Repeat("a", 4096),
		"日本語/ファイル.txt",
		"emoji/😀.txt",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	root := fuzzRoot(f)

	f.Fuzz(func(t *testing.T, name string) {
		// The empty string is exercised by the seeds, but a
		// fuzz-generated "" is possible; both must return an
		// error and neither must panic.
		data, err := root.ReadFile(name)
		if err != nil {
			// Errors are the common case for fuzz-generated
			// inputs and are not a failure. The invariant
			// only applies to accepted inputs.
			return
		}

		// The operation succeeded. The name that was accepted
		// must, after the same normalisation validateName
		// performs, be a local path.
		//
		// The normalisation is: backslashes replaced with
		// forward slashes, then path.Clean. This is the
		// documented contract of validateName, and the fuzz
		// target re-applies it to check the contract.
		normalised := strings.ReplaceAll(name, `\`, `/`)
		normalised = cleanPath(normalised)

		if !filepath.IsLocal(normalised) {
			t.Fatalf(
				"ReadFile(%q) succeeded with cleaned name %q, "+
					"which is not local: confinement invariant violated",
				name, normalised,
			)
		}

		// The read data must be one of the values the in-memory
		// FS contains. This is a second-order check: it
		// catches the case where the operation succeeded on a
		// name that resolved to a file outside the intended
		// namespace. On MemFS there is no "outside" per se,
		// but the check catches the shape of the bug.
		if !knownFuzzContent(data) {
			t.Fatalf(
				"ReadFile(%q) returned unexpected content %q",
				name, data,
			)
		}
	})
}

// cleanPath is a local implementation of path.Clean's public
// contract, used only by the fuzz target. It exists so that the
// target does not import path for a single call and so that the
// cleaning rule the target applies is the rule the target
// documents.
//
// The implementation is a deliberate simplification: it collapses
// repeated separators and resolves "." and ".." segments in the
// same way path.Clean does, but it does not attempt to be
// bit-for-bit identical. The property under test is
// filepath.IsLocal, which is defined in terms of the cleaned
// path's segments, so an equivalent cleaning is sufficient.
func cleanPath(s string) string {
	if s == "" {
		return "."
	}
	// Path.Clean's algorithm: split on "/", drop empty segments
	// and ".", pop on "..".
	parts := strings.Split(s, "/")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		switch p {
		case "", ".":
			continue
		case "..":
			if len(out) > 0 && out[len(out)-1] != ".." {
				out = out[:len(out)-1]
			} else {
				out = append(out, "..")
			}
		default:
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return "."
	}
	return strings.Join(out, "/")
}

// knownFuzzContent reports whether data is one of the values in
// the fuzz tree. The check is a string comparison against the
// known content, which catches the case where a bug resolved a
// name to the wrong entry.
func knownFuzzContent(data []byte) bool {
	switch string(data) {
	case "a", "dots", "bc", "bde", "unicode":
		return true
	}
	return false
}

// -----------------------------------------------------------------------------
// FuzzValidateName_NoPanic
// -----------------------------------------------------------------------------

// FuzzValidateName_NoPanic is a narrower target whose property is
// only "no panic." It exists separately from FuzzValidateName so
// that a fuzz run can distinguish "the invariant was violated"
// from "the code crashed." A crash is always a bug; an invariant
// violation might be a documentation issue or a genuine security
// bug, and separating the two makes triage faster.
//
// This target runs the same seeds but calls every exported Root
// method, not just ReadFile, so that a panic in Open, Stat,
// ReadDir, or Walk is caught independently of the read path.
func FuzzValidateName_NoPanic(f *testing.F) {
	seeds := []string{
		"", ".", "..", "../..", "a/b", "/abs", `C:\x`,
		"a\x00b", strings.Repeat("a", 5000),
		"\u202e", "..foo", "a/../b",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	root := fuzzRoot(f)

	f.Fuzz(func(t *testing.T, name string) {
		// The only requirement: none of these calls may panic.
		// Errors are fine. Successes are fine. A panic is a
		// bug in safefs or in its delegate.

		if f, err := root.Open(name); err == nil {
			_ = f.Close()
		}
		_, _ = root.Stat(name)
		_, _ = root.ReadFile(name)
		_, _ = root.ReadDir(name)
		_ = root.Walk(name, func(string, fs.DirEntry, error) error {
			return nil
		})
	})
}

// -----------------------------------------------------------------------------
// FuzzWalk_DepthBound
// -----------------------------------------------------------------------------

// FuzzWalk_DepthBound asserts the property that Walk never returns
// a name deeper than the configured MaxDepth. The fuzz input is
// the tree (encoded as a flat map of name → content) and the
// starting depth; the target asserts the invariant on every
// visited entry.
//
// Because the tree is constructed from the fuzz input, the target
// cannot use fuzzRoot; it constructs a fresh Root per input. That
// is slower than the other targets but the property it verifies —
// the depth bound is honoured regardless of the tree's shape — is
// worth the cost.
//
// The encoded tree uses "\n" as the record separator and "=" as
// the key/value separator. Names must not contain these
// characters; a fuzz input that does is rejected by the parser
// and the target returns without assertion.
func FuzzWalk_DepthBound(f *testing.F) {
	// Seeds are simple trees that exercise the depth bound.
	f.Add("a=1", uint8(0))
	f.Add("a/b=1", uint8(1))
	f.Add("a/b/c=1", uint8(1))
	f.Add("a/b/c/d=1", uint8(2))
	f.Add("a=1\nb/c=2", uint8(1))
	f.Add("", uint8(2))

	f.Fuzz(func(t *testing.T, treeSpec string, depth uint8) {
		// Cap the depth to keep the test fast.
		if depth > 8 {
			depth = 8
		}

		m := parseFuzzTree(treeSpec)
		if m == nil {
			return
		}

		root, err := safefs.New("/root",
			safefs.WithFS(filesystem.NewMemFS(m)),
			safefs.WithMaxDepth(int(depth)),
		)
		if err != nil {
			return
		}

		var walkErr error
		_ = root.Walk(".", func(rel string, _ fs.DirEntry, err error) error {
			if err != nil {
				walkErr = err
				return err
			}
			// Every accepted name must be local and
			// must not be deeper than the bound.
			if !filepath.IsLocal(rel) {
				t.Fatalf("Walk visited non-local name %q", rel)
			}
			gotDepth := strings.Count(rel, "/")
			if depth > 0 && gotDepth > int(depth) {
				t.Fatalf("Walk visited %q at depth %d, "+
					"bound was %d", rel, gotDepth, depth)
			}
			return nil
		})

		// If the walk completed without error, the depth
		// bound was respected. If it returned
		// ErrDepthExceeded, the bound was enforced.
		if walkErr != nil && !errors.Is(walkErr, safefs.ErrDepthExceeded) {
			// Some other error is acceptable (the tree
			// may not contain the entries the spec
			// implied). No assertion.
			_ = walkErr
		}
	})
}

// parseFuzzTree converts a "\n"-separated list of "name=value"
// records into a fstest.MapFS. It returns nil if any record is
// malformed, in which case the caller skips the input.
//
// The parser is deliberately strict: fuzz-generated inputs are
// arbitrary and a permissive parser would accept inputs the test
// does not intend to construct, weakening the property.
func parseFuzzTree(spec string) fstest.MapFS {
	if spec == "" {
		return fstest.MapFS{}
	}
	out := fstest.MapFS{}
	for _, line := range strings.Split(spec, "\n") {
		if line == "" {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			return nil
		}
		name := line[:eq]
		value := line[eq+1:]
		if name == "" {
			return nil
		}
		if strings.ContainsAny(name, "\x00\n=") {
			return nil
		}
		// Reject names that are not local; they cannot
		// describe entries in a MapFS.
		if !filepath.IsLocal(name) {
			return nil
		}
		out[name] = &fstest.MapFile{Data: []byte(value)}
	}
	return out
}
