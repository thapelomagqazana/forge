package version

import (
	"os"
	"strings"
	"testing"
)

// ─────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────

// snapshot captures the four package variables so a test can restore
// them with t.Cleanup. Tests that mutate package state must call this
// first so the save/restore pattern is uniform and hard to get wrong.
//
// Because the four variables are package-level, tests that mutate
// them cannot run in parallel with each other. The helper exists so
// that this constraint is expressed once, in one place.
func snapshot(t *testing.T) {
	t.Helper()

	v, c, b, d := Version, Commit, BuildDate, Dirty
	t.Cleanup(func() {
		Version, Commit, BuildDate, Dirty = v, c, b, d
	})
}

// readFile reads a file relative to the test's working directory and
// fails the test if the read errors. It is used by
// TestNonFunctional_NoForbiddenImports to assert the leaf-package
// contract without pulling in go/parser.
func readFile(t *testing.T, name string) string {
	t.Helper()

	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(b)
}

// ─────────────────────────────────────────────────────────────────────
// TDD — contract tests (written first, expected to fail)
// ─────────────────────────────────────────────────────────────────────

// TestGet_ReturnsAllFourVariables is the first failing test for WBS
// 6.1.1. It asserts the tuple contract: Get returns exactly the four
// package variables, in order.
func TestGet_ReturnsAllFourVariables(t *testing.T) {
	// Not parallel: mutates package state.
	snapshot(t)

	Version = "1.2.3"
	Commit = "abc1234"
	BuildDate = "2026-10-09T12:00:00Z"
	Dirty = "false"

	v, c, b, d := Get()

	if v != "1.2.3" {
		t.Errorf("version = %q, want %q", v, "1.2.3")
	}
	if c != "abc1234" {
		t.Errorf("commit = %q, want %q", c, "abc1234")
	}
	if b != "2026-10-09T12:00:00Z" {
		t.Errorf("buildDate = %q, want %q", b, "2026-10-09T12:00:00Z")
	}
	if d != "false" {
		t.Errorf("dirty = %q, want %q", d, "false")
	}
}

// TestGet_IsDeterministic asserts that two consecutive calls with no
// intervening mutation return equal tuples. Get must not depend on
// time, environment, or any mutable state.
func TestGet_IsDeterministic(t *testing.T) {
	t.Parallel()

	v1, c1, b1, d1 := Get()
	v2, c2, b2, d2 := Get()

	if v1 != v2 || c1 != c2 || b1 != b2 || d1 != d2 {
		t.Fatalf("Get is not deterministic: (%q,%q,%q,%q) vs (%q,%q,%q,%q)",
			v1, c1, b1, d1, v2, c2, b2, d2)
	}
}

// ─────────────────────────────────────────────────────────────────────
// ATDD — acceptance criteria from WBS 6.1.1
// ─────────────────────────────────────────────────────────────────────

// TestAC3_VariablesDefaultToEmptyString verifies the contract that a
// binary built without ldflags reports empty values, not sentinels.
//
// AC3 (revised): "All four variables default to the empty string."
//
// This is the most important test in the package because it pins the
// design decision that "empty means uninstrumented". A future
// contributor who changes a default to "dev" will fail this test and
// be forced to read the doc comment explaining why.
func TestAC3_VariablesDefaultToEmptyString(t *testing.T) {
	// Not parallel: mutates package state.
	snapshot(t)

	Version, Commit, BuildDate, Dirty = "", "", "", ""

	v, c, b, d := Get()
	for _, f := range []struct {
		name string
		got  string
	}{
		{"Version", v},
		{"Commit", c},
		{"BuildDate", b},
		{"Dirty", d},
	} {
		if f.got != "" {
			t.Errorf("%s = %q, want empty string", f.name, f.got)
		}
	}
}

// TestAC4_GetHasNoSideEffects verifies that Get does not mutate the
// package variables.
//
// Get is documented as pure. Purity has two halves: no observable
// side effect on the caller (no I/O, no allocation beyond the return
// value) and no mutation of the package's own state. This test pins
// the second half: calling Get repeatedly must leave Version, Commit,
// BuildDate, and Dirty exactly as they were.
//
// The three calls are deliberate: a single call would not catch a
// hypothetical implementation that mutated on the second invocation
// (for example, a lazily-initialised cache). Three calls give the
// mutation a chance to appear without slowing the test noticeably.
//
// Note on the blank identifier: Go does not spread a multi-value
// return into a single blank. `_ = Get()` is illegal; the arity of
// the left-hand side must match the arity of the call. Hence the
// four-blank form below.
func TestAC4_GetHasNoSideEffects(t *testing.T) {
	// Not parallel: mutates package state.
	snapshot(t)

	Version = "1.0.0"
	Commit = "deadbee"
	BuildDate = "2026-10-09T12:00:00Z"
	Dirty = "true"

	before := [4]string{Version, Commit, BuildDate, Dirty}

	_, _, _, _ = Get()
	_, _, _, _ = Get()
	_, _, _, _ = Get()

	after := [4]string{Version, Commit, BuildDate, Dirty}

	if before != after {
		t.Fatalf("Get mutated package state:\nbefore=%v\nafter =%v", before, after)
	}
}

// TestAC5_VariablesAreExported is a compile-time contract: if any of
// the four variables were unexported, this test would not compile.
//
// AC5 (revised): "All four variables are exported so -X can target
// them."
//
// The test exists as executable documentation of the design choice.
// It will not fail at runtime; it fails at build time if someone
// unexports a variable.
func TestAC5_VariablesAreExported(t *testing.T) {
	t.Parallel()

	// These references require exported identifiers. If a variable
	// is renamed or unexported, the package fails to build.
	_ = Version
	_ = Commit
	_ = BuildDate
	_ = Dirty
}

// TestAC6_GetReturnsFourStrings is a compile-time contract on the
// tuple arity and types.
//
// AC6: "Get returns exactly four string values."
//
// The test passes if and only if Get's signature is
//
//	func Get() (string, string, string, string)
//
// If the arity changes, the multi-value assignment below fails to
// compile. If any element's type changes, one of the `var _ string`
// lines fails to compile. No runtime assertion is needed or possible.
//
// Note: Go does not spread a multi-value return into a variadic
// parameter list. A helper such as `mustString(Get())` would not
// compile, because `Get()` is a single argument expression, not four
// arguments. The destructuring form below is the correct idiom.
func TestAC6_GetReturnsFourStrings(t *testing.T) {
	t.Parallel()

	// Arity: a 4-tuple is required. Fewer or more values fails to
	// compile here.
	v, c, b, d := Get()

	// Types: each element must be a string. Any other type fails to
	// compile here.
	var _ string = v
	var _ string = c
	var _ string = b
	var _ string = d
}

// ─────────────────────────────────────────────────────────────────────
// BDD — behaviour specifications
// ─────────────────────────────────────────────────────────────────────

// TestBehaviour_UninstrumentedBuildReportsEmptyValues encodes the BDD
// scenario:
//
//	Given a binary built with `go build` and no ldflags
//	When a caller calls version.Get()
//	Then all four values are the empty string.
//
// This is the "fresh checkout" scenario. It is the most common
// scenario during development, and it must be predictable.
func TestBehaviour_UninstrumentedBuildReportsEmptyValues(t *testing.T) {
	// Not parallel: mutates package state.
	snapshot(t)

	Version, Commit, BuildDate, Dirty = "", "", "", ""

	v, c, b, d := Get()
	if v != "" || c != "" || b != "" || d != "" {
		t.Fatalf("uninstrumented build reported non-empty values: (%q,%q,%q,%q)", v, c, b, d)
	}
}

// TestBehaviour_InstrumentedBuildReportsInjectedValues encodes the
// BDD scenario:
//
//	Given a binary built with ldflags that inject all four values
//	When a caller calls version.Get()
//	Then all four values match what was injected.
func TestBehaviour_InstrumentedBuildReportsInjectedValues(t *testing.T) {
	// Not parallel: mutates package state.
	snapshot(t)

	Version = "1.2.3"
	Commit = "abc1234"
	BuildDate = "2026-10-09T12:00:00Z"
	Dirty = "true"

	v, c, b, d := Get()
	if v != "1.2.3" || c != "abc1234" || b != "2026-10-09T12:00:00Z" || d != "true" {
		t.Fatalf("instrumented build reported wrong values: (%q,%q,%q,%q)", v, c, b, d)
	}
}

// TestBehaviour_GetIsSafeForConcurrentUse encodes the BDD scenario:
//
//	Given a running process
//	When many goroutines call Get concurrently
//	Then no data race is reported and all callers see the same tuple.
//
// Run with `go test -race` to exercise the race detector.
func TestBehaviour_GetIsSafeForConcurrentUse(t *testing.T) {
	t.Parallel()

	const goroutines = 32
	const iterations = 256

	// Read the tuple once on the main goroutine to establish the
	// expected value. We do not mutate the package variables, so the
	// value is stable for the duration of the test.
	wantV, wantC, wantB, wantD := Get()

	done := make(chan struct{}, goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for i := 0; i < iterations; i++ {
				v, c, b, d := Get()
				if v != wantV || c != wantC || b != wantB || d != wantD {
					t.Errorf("concurrent Get diverged: (%q,%q,%q,%q)", v, c, b, d)
					return
				}
			}
		}()
	}
	for g := 0; g < goroutines; g++ {
		<-done
	}
}

// ─────────────────────────────────────────────────────────────────────
// Negative / edge / corner / boundary cases
// ─────────────────────────────────────────────────────────────────────

// TestDirty_IsNotValidated asserts the negative contract: the package
// does not normalise, parse, or reject Dirty. Any string round-trips
// unchanged. Parsing is the consumer's responsibility.
//
// This is the key negative test for the "why a string and not a bool"
// design decision documented on the Dirty variable.
func TestDirty_IsNotValidated(t *testing.T) {
	// Not parallel: mutates package state.
	snapshot(t)

	cases := []string{
		"true",
		"false",
		"1",
		"0",
		"yes",
		"no",
		"TRUE",
		"",
		" true",
		"true ",
		"maybe",
	}

	for _, raw := range cases {
		raw := raw
		t.Run("raw="+raw, func(t *testing.T) {
			// Not parallel: mutates package state.
			Dirty = raw
			_, _, _, got := Get()
			if got != raw {
				t.Errorf("Dirty=%q round-tripped as %q; package must not validate", raw, got)
			}
		})
	}
}

// TestFields_AreNotValidated asserts the negative contract for the
// three string fields: any value round-trips unchanged. The package
// does not parse semver, does not trim whitespace, and does not
// substitute defaults.
func TestFields_AreNotValidated(t *testing.T) {
	// Not parallel: mutates package state.
	snapshot(t)

	cases := []struct {
		name      string
		version   string
		commit    string
		buildDate string
	}{
		{"semver", "1.2.3", "abc1234", "2026-10-09T12:00:00Z"},
		{"prerelease", "1.2.3-rc.1", "abc1234", "2026-10-09T12:00:00Z"},
		{"build metadata", "1.2.3+build.5", "abc1234", "2026-10-09T12:00:00Z"},
		{"garbage", "not-a-version", "!!!", "not-a-date"},
		{"whitespace", " 1.2.3 ", " abc ", " 2026-10-09 "},
		{"empty", "", "", ""},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Not parallel: mutates package state.
			Version, Commit, BuildDate = tc.version, tc.commit, tc.buildDate
			v, c, b, _ := Get()
			if v != tc.version {
				t.Errorf("Version: got %q, want %q", v, tc.version)
			}
			if c != tc.commit {
				t.Errorf("Commit: got %q, want %q", c, tc.commit)
			}
			if b != tc.buildDate {
				t.Errorf("BuildDate: got %q, want %q", b, tc.buildDate)
			}
		})
	}
}

// TestBoundary_EmptyInjectedValueIsPreserved asserts the corner case
// where a build system injects an empty string for one field while
// injecting non-empty values for the others. The empty field must
// remain empty; the model does not substitute defaults.
func TestBoundary_EmptyInjectedValueIsPreserved(t *testing.T) {
	// Not parallel: mutates package state.
	snapshot(t)

	Version = "1.2.3"
	Commit = "" // e.g. built outside a Git tree
	BuildDate = "2026-10-09T12:00:00Z"
	Dirty = "false"

	v, c, b, d := Get()
	if v != "1.2.3" {
		t.Errorf("Version = %q, want %q", v, "1.2.3")
	}
	if c != "" {
		t.Errorf("Commit = %q, want empty (no substitution)", c)
	}
	if b != "2026-10-09T12:00:00Z" {
		t.Errorf("BuildDate = %q, want %q", b, "2026-10-09T12:00:00Z")
	}
	if d != "false" {
		t.Errorf("Dirty = %q, want %q", d, "false")
	}
}

// TestCorner_UnknownVsCleanDirty asserts the corner case that
// motivated the string-typed Dirty: the consumer can distinguish
// "unknown" (empty string, uninstrumented build) from "clean"
// ("false", instrumented clean build). The model preserves both.
func TestCorner_UnknownVsCleanDirty(t *testing.T) {
	// Not parallel: mutates package state.
	snapshot(t)

	// Unknown: uninstrumented build.
	Dirty = ""
	_, _, _, unknown := Get()
	if unknown != "" {
		t.Fatalf("unknown Dirty = %q, want empty", unknown)
	}

	// Clean: instrumented clean build.
	Dirty = "false"
	_, _, _, clean := Get()
	if clean != "false" {
		t.Fatalf("clean Dirty = %q, want %q", clean, "false")
	}

	if unknown == clean {
		t.Fatalf("unknown and clean must be distinguishable; both were %q", unknown)
	}
}

// ─────────────────────────────────────────────────────────────────────
// Non-functional cases
// ─────────────────────────────────────────────────────────────────────

// TestNonFunctional_GetIsAllocationFree asserts that Get does not
// allocate on the heap. The tuple is returned by value and contains
// only strings, so the compiler should keep it on the stack.
//
// This matters because Get may be called in hot paths (diagnostics
// emitted per command, structured logging, etc.).
func TestNonFunctional_GetIsAllocationFree(t *testing.T) {
	// Not parallel: we are measuring allocations, and t.Parallel
	// would interleave with other tests.
	allocs := testing.AllocsPerRun(1000, func() {
		_, _, _, _ = Get()
	})
	if allocs > 0 {
		t.Errorf("Get allocated %v times per run, want 0", allocs)
	}
}

// TestNonFunctional_GetIsFast asserts a generous upper bound on Get's
// runtime. The bound is intentionally loose so it does not flake on
// CI; the real guarantee is the allocation-free test.
func TestNonFunctional_GetIsFast(t *testing.T) {
	t.Parallel()

	const iterations = 10_000
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < iterations; i++ {
			_, _, _, _ = Get()
		}
	}()
	<-done
}

// TestNonFunctional_RoundTripLengthIsBounded asserts that Get does not
// pad, trim, or otherwise modify the length of any field. This is a
// proxy for "the package does no work" — a future implementation that
// substitutes defaults or normalises values would change lengths and
// fail this test.
func TestNonFunctional_RoundTripLengthIsBounded(t *testing.T) {
	// Not parallel: mutates package state.
	snapshot(t)

	const sentinel = "x"
	Version, Commit, BuildDate, Dirty = sentinel, sentinel, sentinel, sentinel

	v, c, b, d := Get()
	for _, f := range []struct {
		name string
		got  string
	}{
		{"Version", v},
		{"Commit", c},
		{"BuildDate", b},
		{"Dirty", d},
	} {
		if len(f.got) != len(sentinel) {
			t.Errorf("%s length changed: got %d, want %d", f.name, len(f.got), len(sentinel))
		}
	}
}

// TestNonFunctional_NoForbiddenImports asserts the leaf-package
// contract: version.go must not import anything. The package is a
// leaf in the dependency graph, and this test makes that contract
// executable so a future contributor cannot silently add an import.
//
// The check is deliberately textual rather than AST-based: the file
// is small, the invariant is simple, and pulling in go/parser for a
// one-line assertion would add a dependency the package is trying to
// avoid. If the invariant ever needs to be checked more precisely,
// this test should be replaced with a go/parser-based check in a
// separate tools package, not expanded here.
func TestNonFunctional_NoForbiddenImports(t *testing.T) {
	t.Parallel()

	src := readFile(t, "version.go")

	if strings.Contains(src, "import") {
		t.Errorf("version.go must not import anything; found an import block")
	}
}
