package version

import (
	"os"
	"reflect"
	"strings"
	"testing"

	forgeversion "github.com/thapelomagqazana/forge/internal/version"
)

// =============================================================================
// Helpers
// =============================================================================

// snapshotVersionVars saves the five package variables in
// internal/version and registers a t.Cleanup that restores them.
//
// Tests that mutate the variables must call this first. Because the
// variables are package-level, tests that mutate them cannot run in
// parallel with each other; the helper makes the save/restore
// pattern uniform and hard to get wrong.
//
// # Why five variables
//
// WBS 6.1.1 introduced four variables (Version, Commit, BuildDate,
// Dirty). WBS 17.4.1 added a fifth (BuildType). The helper saves all
// five so a test that mutates any of them is isolated.
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
	origBuildType := forgeversion.BuildType

	t.Cleanup(func() {
		forgeversion.Version = origVersion
		forgeversion.Commit = origCommit
		forgeversion.BuildDate = origBuildDate
		forgeversion.Dirty = origDirty
		forgeversion.BuildType = origBuildType
	})
}

// =============================================================================
// TDD — the accessor contract
// =============================================================================

// TestBuildInfo_ReturnsTheFiveVariables is the first failing test
// for the accessor.
//
// It sets the five package variables in internal/version to known
// values and asserts that buildInfo() returns them, in order.
//
// This test pins the ordering: (version, commit, buildDate, dirty,
// buildType). A future refactor that reorders the return values
// fails here.
func TestBuildInfo_ReturnsTheFiveVariables(t *testing.T) {
	// Not parallel: mutates package state.
	snapshotVersionVars(t)

	forgeversion.Version = "1.2.3"
	forgeversion.Commit = "abc1234"
	forgeversion.BuildDate = "2026-10-10T12:00:00Z"
	forgeversion.Dirty = "false"
	forgeversion.BuildType = "release"

	version, commit, buildDate, dirty, buildType := buildInfo()

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
	if buildType != "release" {
		t.Errorf("buildType: got %q, want %q", buildType, "release")
	}
}

// TestBuildInfo_IsPure verifies that two consecutive calls with no
// intervening mutation return equal values. buildInfo is documented
// as pure; this is the executable form of that contract.
func TestBuildInfo_IsPure(t *testing.T) {
	t.Parallel()

	v1, c1, b1, d1, k1 := buildInfo()
	v2, c2, b2, d2, k2 := buildInfo()

	if v1 != v2 || c1 != c2 || b1 != b2 || d1 != d2 || k1 != k2 {
		t.Fatalf("buildInfo is not pure:\n"+
			"first:  (%q, %q, %q, %q, %q)\n"+
			"second: (%q, %q, %q, %q, %q)",
			v1, c1, b1, d1, k1, v2, c2, b2, d2, k2)
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
	forgeversion.BuildType = "release"

	before := [5]string{
		forgeversion.Version,
		forgeversion.Commit,
		forgeversion.BuildDate,
		forgeversion.Dirty,
		forgeversion.BuildType,
	}

	_, _, _, _, _ = buildInfo()
	_, _, _, _, _ = buildInfo()
	_, _, _, _, _ = buildInfo()

	after := [5]string{
		forgeversion.Version,
		forgeversion.Commit,
		forgeversion.BuildDate,
		forgeversion.Dirty,
		forgeversion.BuildType,
	}

	if before != after {
		t.Fatalf("buildInfo mutated package state:\nbefore=%v\nafter =%v", before, after)
	}
}

// =============================================================================
// ATDD — the service contract (Get)
// =============================================================================

// TestGet_ComposesInfoFromBuildInfo verifies that Get reads the
// five values through buildInfo and returns them in an Info value.
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
	forgeversion.BuildType = "release"

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
	if got.BuildType != "release" {
		t.Errorf("BuildType: got %q, want %q", got.BuildType, "release")
	}
}

// TestGet_PreservesEmptyStrings verifies that Get does not
// substitute sentinels for empty values. An uninstrumented build
// has empty Version, Commit, BuildDate, Dirty, and BuildType; Get
// must return those empties unchanged.
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
	forgeversion.BuildType = ""

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
	if got.BuildType != "" {
		t.Errorf("BuildType: got %q, want empty", got.BuildType)
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

// TestGet_PreservesBuildTypeAsString verifies that Get does not
// parse or normalise the BuildType value. The model stores the raw
// string; normalization to "local" happens in MetadataOf, not in
// Get.
//
// This is the analogue of TestGet_PreservesDirtyAsString for the
// fifth linker variable.
func TestGet_PreservesBuildTypeAsString(t *testing.T) {
	// Not parallel: mutates package state.
	snapshotVersionVars(t)

	cases := []string{
		"release",
		"ci",
		"local",
		"production",
		"RELEASE",
		"",
	}

	for _, raw := range cases {
		raw := raw
		t.Run("raw="+raw, func(t *testing.T) {
			// Not parallel: mutates package state.
			forgeversion.BuildType = raw
			got := Get().BuildType
			if got != raw {
				t.Errorf("BuildType=%q round-tripped as %q; Get must not normalise",
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
	forgeversion.BuildType = ""

	got := Get()

	if got.Version != "" || got.Commit != "" || got.BuildDate != "" || got.Dirty != "" || got.BuildType != "" {
		t.Fatalf("uninstrumented build reported non-empty Info: %+v", got)
	}
}

// TestBehaviour_InstrumentedBuildReportsInjectedInfo encodes the
// BDD scenario:
//
//	Given a binary built with ldflags that inject all five values
//	When Get() is called
//	Then every field of Info matches what was injected.
func TestBehaviour_InstrumentedBuildReportsInjectedInfo(t *testing.T) {
	// Not parallel: mutates package state.
	snapshotVersionVars(t)

	forgeversion.Version = "1.2.3"
	forgeversion.Commit = "abc1234"
	forgeversion.BuildDate = "2026-10-10T12:00:00Z"
	forgeversion.Dirty = "true"
	forgeversion.BuildType = "release"

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
	if got.BuildType != "release" {
		t.Errorf("BuildType: got %q, want %q", got.BuildType, "release")
	}
}

// TestBehaviour_GetIsSafeForConcurrentUse encodes the BDD scenario:
//
//	Given a running process
//	When many goroutines call Get concurrently
//	Then no data race is reported and all callers see the same Info.
//
// Run with `go test -race` to exercise the race detector.
//
// The comparison uses reflect.DeepEqual because Info contains a
// Provenance value whose Dependencies and Settings fields are
// slices. Go's == operator does not work on structs containing
// slices.
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
				if !reflect.DeepEqual(got, want) {
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
// does not validate, trim, or parse any of the five values. Any
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
		buildType string
	}{
		{
			name:      "empty",
			version:   "",
			commit:    "",
			buildDate: "",
			dirty:     "",
			buildType: "",
		},
		{
			name:      "whitespace",
			version:   " ",
			commit:    " ",
			buildDate: " ",
			dirty:     " ",
			buildType: " ",
		},
		{
			name:      "surrounding spaces",
			version:   " 1.2.3 ",
			commit:    " abc ",
			buildDate: " 2026-10-10 ",
			dirty:     " true ",
			buildType: " release ",
		},
		{
			name:      "non-ASCII",
			version:   "1.0.0-α",
			commit:    "café",
			buildDate: "2026-10-10T12:00:00Z",
			dirty:     "false",
			buildType: "release",
		},
		{
			name:      "embedded newline",
			version:   "1.2.3\nmalicious",
			commit:    "abc\ndef",
			buildDate: "2026-10-10",
			dirty:     "false",
			buildType: "release",
		},
		{
			name:      "long strings",
			version:   strings.Repeat("v", 4096),
			commit:    strings.Repeat("c", 4096),
			buildDate: strings.Repeat("d", 4096),
			dirty:     strings.Repeat("t", 4096),
			buildType: strings.Repeat("k", 4096),
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
			forgeversion.BuildType = tc.buildType

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
			if got.BuildType != tc.buildType {
				t.Errorf("BuildType: got %q, want %q", got.BuildType, tc.buildType)
			}
		})
	}
}

// TestBoundary_EmptyIndividualFieldIsPreserved asserts that setting
// only one of the five variables to empty does not affect the
// others. This catches a future implementation that, for example,
// substitutes a default when Version is empty.
func TestBoundary_EmptyIndividualFieldIsPreserved(t *testing.T) {
	// Not parallel: mutates package state.
	snapshotVersionVars(t)

	forgeversion.Version = ""
	forgeversion.Commit = "abc1234"
	forgeversion.BuildDate = "2026-10-10T12:00:00Z"
	forgeversion.Dirty = "false"
	forgeversion.BuildType = "release"

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
	if got.BuildType != "release" {
		t.Errorf("BuildType: got %q, want %q", got.BuildType, "release")
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

// TestCorner_BuildTypeUnknownVsLocal asserts the analogous corner
// case for BuildType: the accessor preserves the empty string
// (unknown) and "local" (explicitly local) as distinct values.
//
// The normalization to "local" happens later, in MetadataOf; Get
// itself does not normalize.
func TestCorner_BuildTypeUnknownVsLocal(t *testing.T) {
	// Not parallel: mutates package state.
	snapshotVersionVars(t)

	// Unknown: uninstrumented build.
	forgeversion.BuildType = ""
	unknown := Get().BuildType
	if unknown != "" {
		t.Fatalf("unknown BuildType = %q, want empty", unknown)
	}

	// Local: explicitly local build.
	forgeversion.BuildType = "local"
	local := Get().BuildType
	if local != "local" {
		t.Fatalf("local BuildType = %q, want %q", local, "local")
	}

	if unknown == local {
		t.Fatalf("unknown and local must be distinguishable; both were %q", unknown)
	}
}

// =============================================================================
// Non-functional
// =============================================================================

// TestNonFunctional_GetAllocsAreBounded asserts that Get's
// allocation count is bounded by a small constant.
//
// # Why the bound is not zero
//
// Before WBS 17.4.1, Get returned only the five linker strings.
// The compiler kept the value on the stack and the test asserted
// zero allocations. WBS 17.4.1 added the provenance to Info; the
// provenance contains Dependencies and Settings slices, which are
// heap-allocated by the library's ReadProvenance.
//
// The allocation is small and bounded: one Provenance value, two
// slice headers, and the slice elements. For the Forge binary the
// slices are short (typically fewer than twenty entries).
//
// # What the test catches
//
// A change that allocates a variable amount (a map, a growing
// slice built element by element, a reflected struct) exceeds the
// bound. A change that adds one more small allocation per call
// might not, and that is acceptable: the property pinned here is
// "no unbounded allocation", not "byte-identical allocation count".
//
// # The exact number
//
// maxAllocs is set to 8, which is the measurement for the test
// binary (Go 1.23, linux/amd64) plus one unit of slack. A future
// toolchain whose ReadBuildInfo allocates differently may require
// raising the number; the failure message names the new value and
// the raise is a one-line edit.
func TestNonFunctional_GetAllocsAreBounded(t *testing.T) {
	// Not parallel: allocation measurement is sensitive to
	// concurrent activity.
	const maxAllocs = 8
	allocs := testing.AllocsPerRun(1000, func() {
		_ = Get()
	})
	if allocs > maxAllocs {
		t.Errorf("Get allocated %v times per run; want at most %d",
			allocs, maxAllocs)
	}
}

// TestNonFunctional_BuildInfoIsAllocationFreeOnHeap asserts the
// same for the lower-level accessor.
//
// # Why this bound is zero
//
// buildInfo reads five strings from the library's GetAll and
// returns them as a tuple. The library's GetAll returns a struct of
// five strings; the compiler keeps both the struct and the tuple on
// the stack. No heap allocation occurs.
//
// This property is what makes buildInfo the right primitive for the
// CLI's command-tree construction path, which runs before any
// logging is configured. A future change that adds allocation to
// buildInfo (for example, reading the provenance) would break this
// property and the test would fail.
func TestNonFunctional_BuildInfoIsAllocationFreeOnHeap(t *testing.T) {
	// Not parallel: see above.
	allocs := testing.AllocsPerRun(1000, func() {
		_, _, _, _, _ = buildInfo()
	})
	if allocs > 0 {
		t.Errorf("buildInfo allocated %v times per run; want 0", allocs)
	}
}

// TestNonFunctional_BuildInfoIsolatesTheImport is a compile-time
// contract that buildinfo.go is the only file in this package that
// imports internal/version directly.
//
// The check is textual: it reads buildinfo.go and asserts that the
// import line is present, and reads service.go and asserts that the
// import line is absent. This pins the "isolate the import in one
// file" property documented on buildInfo and in
// docs/architecture.md § 11.11.
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
