package version

import (
	"os"
	"strings"
	"testing"

	forgeversion "github.com/thapelomagqazana/forge/internal/version"
)

// =============================================================================
// Helpers
// =============================================================================

// snapshotVersionVars saves the four package variables in
// internal/version and registers a t.Cleanup that restores them.
//
// Tests that mutate the variables must call this first. Because the
// variables are package-level, tests that mutate them cannot run in
// parallel with each other; the helper makes the save/restore
// pattern uniform and hard to get wrong.
//
// The helper exists in this package rather than in internal/version
// because the tests here reach into internal/version's exported
// variables, and the restoration must happen on the same goroutine
// the mutation happened on. t.Cleanup guarantees that.
func snapshotVersionVars(t *testing.T) {
	t.Helper()

	origVersion := forgeversion.Version
	origCommit := forgeversion.Commit
	origBuildDate := forgeversion.BuildDate
	origDirty := forgeversion.Dirty

	t.Cleanup(func() {
		forgeversion.Version = origVersion
		forgeversion.Commit = origCommit
		forgeversion.BuildDate = origBuildDate
		forgeversion.Dirty = origDirty
	})
}

// =============================================================================
// TDD — the accessor contract
// =============================================================================

// TestBuildInfo_ReturnsTheFourVariables is the first failing test
// for the accessor.
//
// It sets the four package variables in internal/version to known
// values and asserts that buildInfo() returns them, in order.
//
// This test pins the ordering: (version, commit, buildDate, dirty).
// A future refactor that reorders the return values fails here.
func TestBuildInfo_ReturnsTheFourVariables(t *testing.T) {
	// Not parallel: mutates package state.
	snapshotVersionVars(t)

	forgeversion.Version = "1.2.3"
	forgeversion.Commit = "abc1234"
	forgeversion.BuildDate = "2026-10-10T12:00:00Z"
	forgeversion.Dirty = "false"

	version, commit, buildDate, dirty := buildInfo()

	if version != "1.2.3" {
		t.Errorf("version: got %q, want %q", version, "1.2.3")
	}
	if commit != "abc1234" {
		t.Errorf("commit: got %q, want %q", commit, "abc1234")
	}
	if buildDate != "2026-10-10T12:00:00Z" {
		t.Errorf("buildDate: got %q, want %q", buildDate, "2026-10-10T12:00:00Z")
	}
	if dirty != "false" {
		t.Errorf("dirty: got %q, want %q", dirty, "false")
	}
}

// TestBuildInfo_IsPure verifies that two consecutive calls with no
// intervening mutation return equal values. buildInfo is documented
// as pure; this is the executable form of that contract.
func TestBuildInfo_IsPure(t *testing.T) {
	t.Parallel()

	v1, c1, b1, d1 := buildInfo()
	v2, c2, b2, d2 := buildInfo()

	if v1 != v2 || c1 != c2 || b1 != b2 || d1 != d2 {
		t.Fatalf("buildInfo is not pure:\n"+
			"first:  (%q, %q, %q, %q)\n"+
			"second: (%q, %q, %q, %q)",
			v1, c1, b1, d1, v2, c2, b2, d2)
	}
}

// TestBuildInfo_HasNoSideEffects verifies that calling buildInfo
// does not mutate the package variables in internal/version.
func TestBuildInfo_HasNoSideEffects(t *testing.T) {
	// Not parallel: reads package state that other tests may mutate.
	snapshotVersionVars(t)

	forgeversion.Version = "1.0.0"
	forgeversion.Commit = "deadbee"
	forgeversion.BuildDate = "2026-10-10T12:00:00Z"
	forgeversion.Dirty = "true"

	before := [4]string{
		forgeversion.Version,
		forgeversion.Commit,
		forgeversion.BuildDate,
		forgeversion.Dirty,
	}

	_, _, _, _ = buildInfo()
	_, _, _, _ = buildInfo()
	_, _, _, _ = buildInfo()

	after := [4]string{
		forgeversion.Version,
		forgeversion.Commit,
		forgeversion.BuildDate,
		forgeversion.Dirty,
	}

	if before != after {
		t.Fatalf("buildInfo mutated package state:\nbefore=%v\nafter =%v", before, after)
	}
}

// =============================================================================
// ATDD — the service contract (Get)
// =============================================================================

// TestGet_ComposesInfoFromBuildInfo verifies that Get reads the
// four values through buildInfo and returns them in an Info value.
//
// The test sets the package variables to known values and asserts
// that Get reflects them. This is the ATDD acceptance test for the
// "Info is composed from the model" contract.
func TestGet_ComposesInfoFromBuildInfo(t *testing.T) {
	// Not parallel: mutates package state.
	snapshotVersionVars(t)

	forgeversion.Version = "1.2.3"
	forgeversion.Commit = "abc1234"
	forgeversion.BuildDate = "2026-10-10T12:00:00Z"
	forgeversion.Dirty = "false"

	got := Get()

	if got.Version != "1.2.3" {
		t.Errorf("Version: got %q, want %q", got.Version, "1.2.3")
	}
	if got.Commit != "abc1234" {
		t.Errorf("Commit: got %q, want %q", got.Commit, "abc1234")
	}
	if got.BuildDate != "2026-10-10T12:00:00Z" {
		t.Errorf("BuildDate: got %q, want %q", got.BuildDate, "2026-10-10T12:00:00Z")
	}
	if got.Dirty != "false" {
		t.Errorf("Dirty: got %q, want %q", got.Dirty, "false")
	}
}

// TestGet_PreservesEmptyStrings verifies that Get does not
// substitute sentinels for empty values. An uninstrumented build
// has empty Version, Commit, BuildDate, and Dirty; Get must return
// those empties unchanged.
//
// This is the ATDD acceptance test for the "no substitution"
// policy documented in internal/version (WBS 6.1.1).
func TestGet_PreservesEmptyStrings(t *testing.T) {
	// Not parallel: mutates package state.
	snapshotVersionVars(t)

	forgeversion.Version = ""
	forgeversion.Commit = ""
	forgeversion.BuildDate = ""
	forgeversion.Dirty = ""

	got := Get()

	if got.Version != "" {
		t.Errorf("Version: got %q, want empty", got.Version)
	}
	if got.Commit != "" {
		t.Errorf("Commit: got %q, want empty", got.Commit)
	}
	if got.BuildDate != "" {
		t.Errorf("BuildDate: got %q, want empty", got.BuildDate)
	}
	if got.Dirty != "" {
		t.Errorf("Dirty: got %q, want empty", got.Dirty)
	}
}

// TestGet_PreservesDirtyAsString verifies that Get does not parse
// or normalise the Dirty value. The model stores Dirty as a string;
// Get passes it through unchanged. Parsing is the formatter's job.
func TestGet_PreservesDirtyAsString(t *testing.T) {
	// Not parallel: mutates package state.
	snapshotVersionVars(t)

	cases := []string{
		"true",
		"false",
		"1",
		"0",
		"yes",
		"no",
		"TRUE",
		"",
	}

	for _, raw := range cases {
		raw := raw
		t.Run("raw="+raw, func(t *testing.T) {
			// Not parallel: mutates package state.
			forgeversion.Dirty = raw
			got := Get().Dirty
			if got != raw {
				t.Errorf("Dirty=%q round-tripped as %q; Get must not normalise",
					raw, got)
			}
		})
	}
}

// =============================================================================
// BDD — behaviour specifications
// =============================================================================

// TestBehaviour_UninstrumentedBuildReportsEmptyInfo encodes the BDD
// scenario:
//
//	Given a binary built with `go build` and no ldflags
//	When Get() is called
//	Then every string field of Info is the empty string.
//
// This is the "fresh checkout" scenario. It is the most common
// scenario during development, and it must be predictable.
func TestBehaviour_UninstrumentedBuildReportsEmptyInfo(t *testing.T) {
	// Not parallel: mutates package state.
	snapshotVersionVars(t)

	forgeversion.Version = ""
	forgeversion.Commit = ""
	forgeversion.BuildDate = ""
	forgeversion.Dirty = ""

	got := Get()

	if got.Version != "" || got.Commit != "" || got.BuildDate != "" || got.Dirty != "" {
		t.Fatalf("uninstrumented build reported non-empty Info: %+v", got)
	}
}

// TestBehaviour_InstrumentedBuildReportsInjectedInfo encodes the
// BDD scenario:
//
//	Given a binary built with ldflags that inject all four values
//	When Get() is called
//	Then every field of Info matches what was injected.
func TestBehaviour_InstrumentedBuildReportsInjectedInfo(t *testing.T) {
	// Not parallel: mutates package state.
	snapshotVersionVars(t)

	forgeversion.Version = "1.2.3"
	forgeversion.Commit = "abc1234"
	forgeversion.BuildDate = "2026-10-10T12:00:00Z"
	forgeversion.Dirty = "true"

	got := Get()

	if got.Version != "1.2.3" {
		t.Errorf("Version: got %q, want %q", got.Version, "1.2.3")
	}
	if got.Commit != "abc1234" {
		t.Errorf("Commit: got %q, want %q", got.Commit, "abc1234")
	}
	if got.BuildDate != "2026-10-10T12:00:00Z" {
		t.Errorf("BuildDate: got %q, want %q", got.BuildDate, "2026-10-10T12:00:00Z")
	}
	if got.Dirty != "true" {
		t.Errorf("Dirty: got %q, want %q", got.Dirty, "true")
	}
}

// TestBehaviour_GetIsSafeForConcurrentUse encodes the BDD scenario:
//
//	Given a running process
//	When many goroutines call Get concurrently
//	Then no data race is reported and all callers see the same Info.
//
// Run with `go test -race` to exercise the race detector.
func TestBehaviour_GetIsSafeForConcurrentUse(t *testing.T) {
	t.Parallel()

	const goroutines = 32
	const iterations = 256

	want := Get()

	done := make(chan struct{}, goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for i := 0; i < iterations; i++ {
				got := Get()
				if got != want {
					t.Errorf("concurrent Get diverged:\nwant: %+v\ngot:  %+v",
						want, got)
					return
				}
			}
		}()
	}
	for g := 0; g < goroutines; g++ {
		<-done
	}
}

// =============================================================================
// Negative / edge / corner / boundary cases
// =============================================================================

// TestEdge_ArbitraryStringsArePreserved verifies that the accessor
// does not validate, trim, or parse any of the four values. Any
// string round-trips unchanged. This is the negative contract for
// the "no validation" policy documented in internal/version.
func TestEdge_ArbitraryStringsArePreserved(t *testing.T) {
	// Not parallel: mutates package state.
	snapshotVersionVars(t)

	cases := []struct {
		name      string
		version   string
		commit    string
		buildDate string
		dirty     string
	}{
		{
			name:      "empty",
			version:   "",
			commit:    "",
			buildDate: "",
			dirty:     "",
		},
		{
			name:      "whitespace",
			version:   " ",
			commit:    " ",
			buildDate: " ",
			dirty:     " ",
		},
		{
			name:      "surrounding spaces",
			version:   " 1.2.3 ",
			commit:    " abc ",
			buildDate: " 2026-10-10 ",
			dirty:     " true ",
		},
		{
			name:      "non-ASCII",
			version:   "1.0.0-α",
			commit:    "café",
			buildDate: "2026-10-10T12:00:00Z",
			dirty:     "false",
		},
		{
			name:      "embedded newline",
			version:   "1.2.3\nmalicious",
			commit:    "abc\ndef",
			buildDate: "2026-10-10",
			dirty:     "false",
		},
		{
			name:      "long strings",
			version:   strings.Repeat("v", 4096),
			commit:    strings.Repeat("c", 4096),
			buildDate: strings.Repeat("d", 4096),
			dirty:     strings.Repeat("t", 4096),
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Not parallel: mutates package state.
			forgeversion.Version = tc.version
			forgeversion.Commit = tc.commit
			forgeversion.BuildDate = tc.buildDate
			forgeversion.Dirty = tc.dirty

			got := Get()

			if got.Version != tc.version {
				t.Errorf("Version: got %q, want %q", got.Version, tc.version)
			}
			if got.Commit != tc.commit {
				t.Errorf("Commit: got %q, want %q", got.Commit, tc.commit)
			}
			if got.BuildDate != tc.buildDate {
				t.Errorf("BuildDate: got %q, want %q", got.BuildDate, tc.buildDate)
			}
			if got.Dirty != tc.dirty {
				t.Errorf("Dirty: got %q, want %q", got.Dirty, tc.dirty)
			}
		})
	}
}

// TestBoundary_EmptyIndividualFieldIsPreserved asserts that setting
// only one of the four variables to empty does not affect the
// others. This catches a future implementation that, for example,
// substitutes a default when Version is empty.
func TestBoundary_EmptyIndividualFieldIsPreserved(t *testing.T) {
	// Not parallel: mutates package state.
	snapshotVersionVars(t)

	forgeversion.Version = ""
	forgeversion.Commit = "abc1234"
	forgeversion.BuildDate = "2026-10-10T12:00:00Z"
	forgeversion.Dirty = "false"

	got := Get()

	if got.Version != "" {
		t.Errorf("Version: got %q, want empty (no substitution)", got.Version)
	}
	if got.Commit != "abc1234" {
		t.Errorf("Commit: got %q, want %q", got.Commit, "abc1234")
	}
	if got.BuildDate != "2026-10-10T12:00:00Z" {
		t.Errorf("BuildDate: got %q, want %q", got.BuildDate, "2026-10-10T12:00:00Z")
	}
	if got.Dirty != "false" {
		t.Errorf("Dirty: got %q, want %q", got.Dirty, "false")
	}
}

// TestCorner_DirtyUnknownVsClean asserts the corner case that the
// string-typed Dirty preserves: the consumer can distinguish
// "unknown" (empty string, uninstrumented build) from "clean"
// ("false", instrumented clean build). The accessor preserves both.
func TestCorner_DirtyUnknownVsClean(t *testing.T) {
	// Not parallel: mutates package state.
	snapshotVersionVars(t)

	// Unknown: uninstrumented build.
	forgeversion.Dirty = ""
	unknown := Get().Dirty
	if unknown != "" {
		t.Fatalf("unknown Dirty = %q, want empty", unknown)
	}

	// Clean: instrumented clean build.
	forgeversion.Dirty = "false"
	clean := Get().Dirty
	if clean != "false" {
		t.Fatalf("clean Dirty = %q, want %q", clean, "false")
	}

	if unknown == clean {
		t.Fatalf("unknown and clean must be distinguishable; both were %q", unknown)
	}
}

// =============================================================================
// Non-functional
// =============================================================================

// TestNonFunctional_GetIsAllocationFreeOnHeap asserts that Get does
// not allocate on the heap. Info contains only strings, so the
// compiler should keep the value on the stack.
func TestNonFunctional_GetIsAllocationFreeOnHeap(t *testing.T) {
	// Not parallel: allocation measurement is sensitive to
	// concurrent activity.
	allocs := testing.AllocsPerRun(1000, func() {
		_ = Get()
	})
	if allocs > 0 {
		t.Errorf("Get allocated %v times per run; want 0", allocs)
	}
}

// TestNonFunctional_BuildInfoIsAllocationFreeOnHeap asserts the
// same for the lower-level accessor.
func TestNonFunctional_BuildInfoIsAllocationFreeOnHeap(t *testing.T) {
	// Not parallel: see above.
	allocs := testing.AllocsPerRun(1000, func() {
		_, _, _, _ = buildInfo()
	})
	if allocs > 0 {
		t.Errorf("buildInfo allocated %v times per run; want 0", allocs)
	}
}

// TestNonFunctional_BuildInfoDoesNotImportRuntimeDocumented is a
// compile-time contract that buildInfo is the only function in this
// package that imports internal/version directly.
//
// The check is textual: it reads buildinfo.go and asserts that the
// import line is present, and reads service.go and asserts that the
// import line is absent. This pins the "isolate the import in one
// file" property documented on buildInfo.
func TestNonFunctional_BuildInfoIsolatesTheImport(t *testing.T) {
	t.Parallel()

	buildinfoSrc := readPackageFile(t, "buildinfo.go")
	serviceSrc := readPackageFile(t, "service.go")

	const importPath = `"github.com/thapelomagqazana/forge/internal/version"`

	if !strings.Contains(buildinfoSrc, importPath) {
		t.Errorf("buildinfo.go does not import %s; "+
			"it is documented as the single point of import", importPath)
	}
	if strings.Contains(serviceSrc, importPath) {
		t.Errorf("service.go imports %s directly; "+
			"the import must be isolated to buildinfo.go", importPath)
	}
}

// readPackageFile reads a source file from this package's directory
// and returns its contents. It fails the test if the file cannot be
// read.
//
// The helper is used by structural tests that assert on the shape
// of the package's source. It relies on go test's behaviour of
// running with the package directory as the working directory.
func readPackageFile(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(b)
}
