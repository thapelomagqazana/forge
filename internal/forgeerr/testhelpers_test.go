// Package forgeerr — shared test helpers.
//
// This file contains the helpers that more than one test file in
// the package uses: the -update flag and the goldenText reader.
// It exists so that format_test.go, log_test.go, and
// codes_test.go can each consume them without one defining and
// the others duplicating.
//
// The file is not a test file: it declares no TestXxx functions.
// It is named *_test.go so that Go's build system includes it in
// the package's test binary and makes its symbols visible to the
// package's other test files.
package forgeerr

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// update is the flag consulted by goldenText. It is registered at
// package initialisation, which means the testing package parses
// it before any Test function runs.
//
// The flag is named "update" and is intended to be used as:
//
//	go test -run TestGolden -update ./internal/forgeerr/
//
// The flag is package-private. It is defined once, here, so that
// every golden test in the package shares the same registration
// point and the same semantics: when set, goldenText writes the
// current output to the file instead of reading it.
//
// # Why a single flag for the package
//
// Go's testing framework parses a single set of command-line
// flags per test binary. A second `flag.Bool("update", ...)` call
// in the same package panics at init with "flag redefined:
// update". Defining the flag once here is therefore not a
// stylistic choice; it is a requirement.
var update = flag.Bool("update", false, "update golden files")

// goldenText compares got against the file at path. If the
// -update flag is set, it writes got to the file and returns it
// unchanged, so that the caller's comparison passes. Otherwise,
// it reads the file and returns its contents, failing the test
// with a clear regeneration message if the file is missing.
//
// # Why the helper is byte-exact
//
// goldenText does not normalise whitespace, it does not tolerate
// trailing-newline differences, and it does not fall back to a
// substring match. A golden file is a byte-exact specification,
// and the moment a helper starts tolerating differences, the
// specification becomes fuzzy. The helper's only affordance is
// the -update flag, which is an explicit, opt-in rewrite.
//
// # Why the parent directory is created on demand
//
// The first run of a golden test in a fresh checkout will find
// the testdata directory missing, if the golden file has not yet
// been committed. Creating the directory in -update mode means
// the caller does not need to pre-create testdata/format,
// testdata/log, or testdata/codes; the regeneration command is
// self-sufficient.
//
// # Why the failure message names the regeneration command
//
// A contributor whose golden test fails has two possible
// reactions: inspect the diff and decide the new output is
// correct (regenerate), or inspect the diff and decide the code
// has a bug (fix the code). The message must make both reactions
// available. Naming the exact `-run` pattern and the exact working
// directory is what distinguishes "run `go test -update ./...` and
// hope" from "regenerate exactly this one golden file."
func goldenText(t *testing.T, path, got string) string {
	t.Helper()

	if update != nil && *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
		return got
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf(
			"read %s: %v\n"+
				"Run `go test -run %s -update` from internal/forgeerr/ to generate it.",
			path, err, t.Name(),
		)
	}
	return string(want)
}

// repoRoot walks up from the test's working directory until it
// finds a directory containing go.mod, and returns that
// directory. It is used by tests that read files outside the
// package's own testdata tree — specifically, the AC10
// documentation test in format_test.go, which reads
// docs/cli-ux-spec.md from the repository root.
//
// # Why the walk-up is bounded
//
// The loop stops when it reaches the filesystem root, which is
// the same directory as its own parent. A walk-up that never
// finds go.mod is a sign that the test is running outside the
// module — a misconfigured CI job, or a binary built without the
// module's context. In that case the test must fail with a
// message that names the starting directory, not hang or walk
// the entire filesystem. The unbounded loop below is safe
// because filepath.Dir("/") == "/"; the loop breaks on that
// condition and the test fails on the next line.
//
// # Why not runtime.Caller
//
// runtime.Caller(0) returns the source file path of the caller,
// which is exact at the moment the file is compiled but is
// rewritten by build cache relocation and by Go 1.20+'s
// -trimpath. It is a common choice for this helper and a common
// source of CI-only failures. Walking up from os.Getwd does not
// depend on how the binary was built, only on where it is run;
// go test always runs a package's test binary with the package
// directory as the working directory, which is the invariant
// this helper relies on.
func repoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("go.mod not found above %s", dir)
		}
		dir = parent
	}
}
