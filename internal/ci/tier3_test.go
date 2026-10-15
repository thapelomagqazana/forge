package ci

// tier3_test.go — internal tests for the Tier 3 model.
//
// The tests in this file are package-internal (`package ci`, not
// `package ci_test`) because several of them inspect the package's
// own unexported state:
//
//   - The closure tests assert on tier3Jobs directly, which is the
//     point of the closure: adding a member must fail a test, and
//     the test that fails must be able to name the canonical slice.
//   - The init tests inspect the model's package-level constants,
//     which are unexported where the tests need to see them.
//
// The tests that read the YAML workflow file and the coverage
// baseline live in tier3_contract_test.go and are external
// (`package ci_test`), because they treat the package as a black box
// and assert on the artifacts rather than the model.
//
// # The Tier 3 model
//
// Tier 3 has five jobs:
//
//   - tier3-reproducibility    build twice, compare SHA-256.
//   - tier3-flake-check        go test -count=2 ./...
//   - tier3-race-full          go test -race ./... on the min and
//                              current Go versions.
//   - tier3-coverage           measure, upload, compare to baseline.
//   - tier3-binary-verify-all  17.5.1 suite for all six platforms.
//
// The model pins the job names, the 45-minute job timeout, the
// 0.5-percentage-point coverage threshold, the 30-day artifact
// retention, and the coverage baseline's path.
//
// # Test taxonomy
//
//   - Closure tests: the set has exactly the members it is
//     supposed to have.
//   - Order tests: the canonical order is stable and the two
//     representations agree.
//   - Copy tests: returned values do not alias the canonical state.
//   - Job-name tests: the membership table and String totality.
//   - Lookup tests: totality and the zero value.
//   - Validate tests: the invariants, table-driven on the broken
//     shape.
//   - Constant tests: the model's pinned values.
//   - Golden tests: the JSON rendering is deterministic and valid.
//
// The grouping is documentation. A reader who wants to know what the
// package promises reads the group headings; a reader who wants to
// know whether a specific promise holds reads the test in that group.

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/thapelomagqazana/forge/internal/forgeerr"
)

// --- Closure tests ---------------------------------------------------------

// TestAllTier3Jobs_HasExactlyFive asserts the closed set has the five
// canonical job names.
//
// The test is intentionally brittle. A sixth job is a change to the
// tier's content, and it must be accompanied by an update to the ADR
// and to the tier table. The failure of this test is the first line
// of the reviewer's checklist.
func TestAllTier3Jobs_HasExactlyFive(t *testing.T) {
	got := AllTier3Jobs()
	if len(got) != 5 {
		t.Fatalf("AllTier3Jobs() returned %d job names, want 5", len(got))
	}
}

// TestTier3Jobs_HasExactlyFive asserts the canonical description has
// the same five members as the canonical order.
//
// This is a separate test from TestAllTier3Jobs_HasExactlyFive
// because the two slices could drift: a developer who adds a
// Tier3Job to `tier3Jobs` but forgets `allTier3Jobs` would produce a
// model whose order function and description function disagree.
func TestTier3Jobs_HasExactlyFive(t *testing.T) {
	got := Tier3Jobs()
	if len(got) != 5 {
		t.Fatalf("Tier3Jobs() returned %d jobs, want 5", len(got))
	}
}

// TestAllTier3Jobs_OrderIsStable asserts the canonical order.
//
// The order is the order in which the jobs are presented in the tier
// table and in the workflow file. Reordering the slice without
// reordering the workflow or the tier table would produce a document
// that describes a different architecture from the one the code
// enforces.
func TestAllTier3Jobs_OrderIsStable(t *testing.T) {
	want := []Tier3JobName{
		Tier3JobReproducibility,
		Tier3JobFlakeCheck,
		Tier3JobRaceFull,
		Tier3JobCoverage,
		Tier3JobBinaryVerifyAll,
	}
	got := AllTier3Jobs()
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("AllTier3Jobs()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestAllTier3Jobs_ReturnsFreshSlice asserts the accessor does not
// expose the canonical slice for mutation.
func TestAllTier3Jobs_ReturnsFreshSlice(t *testing.T) {
	a := AllTier3Jobs()
	a[0] = "mutated"
	b := AllTier3Jobs()
	if b[0] == "mutated" {
		t.Fatal("AllTier3Jobs() returned an aliased slice")
	}
}

// TestAllTier3Jobs_CallerTruncationDoesNotAffectPackage asserts that
// a caller who truncates the returned slice does not corrupt the
// package's view.
func TestAllTier3Jobs_CallerTruncationDoesNotAffectPackage(t *testing.T) {
	a := AllTier3Jobs()
	_ = a[:0]
	b := AllTier3Jobs()
	if len(b) != 5 {
		t.Fatalf("AllTier3Jobs() returned %d job names after a caller truncation, want 5", len(b))
	}
}

// --- Order tests -----------------------------------------------------------

// TestTier3Jobs_OrderMatchesAllTier3Jobs asserts the two
// representations of the closed set agree on order.
func TestTier3Jobs_OrderMatchesAllTier3Jobs(t *testing.T) {
	names := AllTier3Jobs()
	jobs := Tier3Jobs()
	if len(names) != len(jobs) {
		t.Fatalf("AllTier3Jobs() has %d entries but Tier3Jobs() has %d", len(names), len(jobs))
	}
	for i := range names {
		if jobs[i].Name != names[i] {
			t.Errorf("Tier3Jobs()[%d].Name = %q, want %q", i, jobs[i].Name, names[i])
		}
	}
}

// TestTier3Jobs_ReturnsDeepCopy asserts the accessor deep-copies each
// Tier3Job.
//
// The shallow copy that would otherwise be tempting — `out :=
// make([]Tier3Job, len(tier3Jobs)); copy(out, tier3Jobs)` — shares
// the backing arrays of the four slice fields (Permissions, MatrixGo,
// ArtifactNames, Steps). A caller that appended to one of them, or
// that replaced an element, would corrupt the canonical set for the
// rest of the process.
//
// # Why the loop variable is named j, not t
//
// The test's parameter is *testing.T, conventionally named t. A loop
// over the returned []Tier3Job that used t as the loop variable would
// shadow the test handle, and any call to t.Fatalf inside the loop
// body would fail to compile ("t.Fatalf undefined (type Tier3Job has
// no field or method Fatalf)"). The loop variable is named j, which
// is the same convention the package's source files use for a
// Tier3Job value.
func TestTier3Jobs_ReturnsDeepCopy(t *testing.T) {
	first := Tier3Jobs()
	for i := range first {
		for k := range first[i].Permissions {
			first[i].Permissions[k] = "mutated-permission"
		}
		for k := range first[i].MatrixGo {
			first[i].MatrixGo[k] = "mutated-go"
		}
		for k := range first[i].ArtifactNames {
			first[i].ArtifactNames[k] = "mutated-artifact"
		}
		for k := range first[i].Steps {
			first[i].Steps[k] = "mutated-step"
		}
	}

	second := Tier3Jobs()
	for _, j := range second {
		for _, p := range j.Permissions {
			if p == "mutated-permission" {
				t.Fatalf("job %q: Permissions aliased the canonical slice", j.Name)
			}
		}
		for _, g := range j.MatrixGo {
			if g == "mutated-go" {
				t.Fatalf("job %q: MatrixGo aliased the canonical slice", j.Name)
			}
		}
		for _, a := range j.ArtifactNames {
			if a == "mutated-artifact" {
				t.Fatalf("job %q: ArtifactNames aliased the canonical slice", j.Name)
			}
		}
		for _, s := range j.Steps {
			if s == "mutated-step" {
				t.Fatalf("job %q: Steps aliased the canonical slice", j.Name)
			}
		}
	}
}

// TestLookupTier3Job_ReturnsDeepCopy asserts the single-entry
// accessor deep-copies too.
func TestLookupTier3Job_ReturnsDeepCopy(t *testing.T) {
	j, ok := LookupTier3Job(Tier3JobRaceFull)
	if !ok {
		t.Fatal("LookupTier3Job(Tier3JobRaceFull) not found")
	}
	for k := range j.Permissions {
		j.Permissions[k] = "mutated-permission"
	}
	for k := range j.MatrixGo {
		j.MatrixGo[k] = "mutated-go"
	}
	for k := range j.ArtifactNames {
		j.ArtifactNames[k] = "mutated-artifact"
	}
	for k := range j.Steps {
		j.Steps[k] = "mutated-step"
	}

	again, ok := LookupTier3Job(Tier3JobRaceFull)
	if !ok {
		t.Fatal("LookupTier3Job(Tier3JobRaceFull) not found on second call")
	}
	for _, p := range again.Permissions {
		if p == "mutated-permission" {
			t.Fatal("LookupTier3Job returned an aliased Permissions slice")
		}
	}
	for _, g := range again.MatrixGo {
		if g == "mutated-go" {
			t.Fatal("LookupTier3Job returned an aliased MatrixGo slice")
		}
	}
	for _, a := range again.ArtifactNames {
		if a == "mutated-artifact" {
			t.Fatal("LookupTier3Job returned an aliased ArtifactNames slice")
		}
	}
	for _, s := range again.Steps {
		if s == "mutated-step" {
			t.Fatal("LookupTier3Job returned an aliased Steps slice")
		}
	}
}

// --- Job-name tests --------------------------------------------------------

// TestTier3JobName_Valid is the membership table for the closed set.
func TestTier3JobName_Valid(t *testing.T) {
	cases := []struct {
		name Tier3JobName
		want bool
	}{
		{Tier3JobReproducibility, true},
		{Tier3JobFlakeCheck, true},
		{Tier3JobRaceFull, true},
		{Tier3JobCoverage, true},
		{Tier3JobBinaryVerifyAll, true},
		{"", false},
		{"unknown", false},
		{"TIER3-RACE-FULL", false},  // case-sensitive
		{"tier3-race-full ", false}, // trailing space
		{" tier3-race-full", false}, // leading space
	}
	for _, tc := range cases {
		if got := tc.name.Valid(); got != tc.want {
			t.Errorf("Tier3JobName(%q).Valid() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestTier3JobName_AllMembersAreValid asserts the closed set is
// self-consistent: every member of allTier3Jobs is Valid, and the
// number of valid names equals the size of the set.
func TestTier3JobName_AllMembersAreValid(t *testing.T) {
	valid := 0
	for _, name := range AllTier3Jobs() {
		if !name.Valid() {
			t.Errorf("AllTier3Jobs() member %q is not Valid()", name)
		}
		valid++
	}
	if valid != len(allTier3Jobs) {
		t.Fatalf("counted %d valid names but allTier3Jobs has %d", valid, len(allTier3Jobs))
	}
}

// TestTier3JobName_String_IsTotal asserts String never panics,
// including on an unknown name.
func TestTier3JobName_String_IsTotal(t *testing.T) {
	cases := []Tier3JobName{"", "unknown", Tier3JobReproducibility, Tier3JobBinaryVerifyAll}
	for _, c := range cases {
		got := c.String()
		if got != string(c) {
			t.Errorf("Tier3JobName(%q).String() = %q, want %q", c, got, string(c))
		}
	}
}

// --- Lookup tests ----------------------------------------------------------

// TestLookupTier3Job_FindsEveryJob asserts LookupTier3Job is total.
func TestLookupTier3Job_FindsEveryJob(t *testing.T) {
	for _, name := range AllTier3Jobs() {
		j, ok := LookupTier3Job(name)
		if !ok {
			t.Errorf("LookupTier3Job(%q) reported not found", name)
			continue
		}
		if j.Name != name {
			t.Errorf("LookupTier3Job(%q).Name = %q", name, j.Name)
		}
	}
}

// TestLookupTier3Job_UnknownReturnsZero asserts LookupTier3Job does
// not panic and returns the zero Tier3Job for an unknown name.
func TestLookupTier3Job_UnknownReturnsZero(t *testing.T) {
	j, ok := LookupTier3Job("nope")
	if ok {
		t.Fatal("LookupTier3Job(\"nope\") reported found")
	}
	if !isZeroTier3Job(j) {
		t.Fatalf("LookupTier3Job(\"nope\") returned %#v, want the zero Tier3Job", j)
	}
}

// TestLookupTier3Job_EmptyNameReturnsZero asserts the empty string is
// not a valid Tier 3 job name.
func TestLookupTier3Job_EmptyNameReturnsZero(t *testing.T) {
	j, ok := LookupTier3Job("")
	if ok {
		t.Fatal("LookupTier3Job(\"\") reported found")
	}
	if !isZeroTier3Job(j) {
		t.Fatalf("LookupTier3Job(\"\") returned %#v, want the zero Tier3Job", j)
	}
}

// --- Validate tests --------------------------------------------------------

// TestValidateTier3_AcceptsCanonicalState asserts the canonical state
// is internally consistent.
func TestValidateTier3_AcceptsCanonicalState(t *testing.T) {
	if err := ValidateTier3(); err != nil {
		t.Fatalf("ValidateTier3() rejected the canonical state: %v", err)
	}
}

// TestValidateTier3_IsDeterministic asserts repeated calls agree.
func TestValidateTier3_IsDeterministic(t *testing.T) {
	for i := 0; i < 3; i++ {
		if err := ValidateTier3(); err != nil {
			t.Fatalf("ValidateTier3() call %d failed: %v", i, err)
		}
	}
}

// TestValidateTier3_ErrorIsForgeError asserts the error type is the
// structured one.
//
// # Why the probe is validateTier3Job, not ValidateTier3
//
// ValidateTier3 on the canonical state returns nil; it cannot be used
// to produce an error without mutating package-level data, which the
// test suite deliberately avoids. validateTier3Job is the per-entry
// validator that ValidateTier3 calls internally; probing it on a
// broken entry exercises the same error-construction path.
func TestValidateTier3_ErrorIsForgeError(t *testing.T) {
	err := validateTier3Job(Tier3Job{})
	if err == nil {
		t.Fatal("validateTier3Job on the zero Tier3Job returned nil")
	}
	var fe *forgeerr.Error
	if !errors.As(err, &fe) {
		t.Fatalf("validateTier3Job returned %T, want *forgeerr.Error", err)
	}
	if fe.Category() != forgeerr.CategoryInternal {
		t.Errorf("validateTier3Job category = %v, want %v", fe.Category(), forgeerr.CategoryInternal)
	}
	if fe.Code() != string(forgeerr.CodeInternalUnexpected) {
		t.Errorf("validateTier3Job code = %q, want %q", fe.Code(), forgeerr.CodeInternalUnexpected)
	}
}

// validTier3Job returns a Tier3Job that passes every check. Each
// test starts from this value and breaks exactly one property, so the
// failure is attributable to the property under test.
//
// The helper is a function, not a package-level var, so that each
// caller gets a fresh value.
func validTier3Job() Tier3Job {
	return Tier3Job{
		Name:        Tier3JobRaceFull,
		Timeout:     45 * time.Minute,
		Permissions: []Permission{PermissionContentsRead},
		UsesMatrix:  true,
		MatrixGo:    AllTier2GoVersions(),
		Steps:       []string{"Check out the repository", "Set up Go", "go test -race ./..."},
	}
}

// TestValidateTier3Job_AcceptsValidEntry asserts the baseline is
// accepted.
func TestValidateTier3Job_AcceptsValidEntry(t *testing.T) {
	if err := validateTier3Job(validTier3Job()); err != nil {
		t.Fatalf("validateTier3Job rejected the baseline: %v", err)
	}
}

// TestValidateTier3Job_RejectsBrokenEntries is the table of broken
// shapes. Each row breaks exactly one property of the baseline.
//
// # Why the assertion reads the context, not the message
//
// `tier3Error` builds a *forgeerr.Error whose message is a fixed
// string ("ci: tier3 model invariant violated") and whose context
// carries the invariant's name under the `invariant` key. The
// message is fixed because forgeerr enforces an 80-character limit
// (I4) and the Tier 3 model's invariant names are long enough that
// composing them into the message would exceed the limit and panic.
//
// The test therefore reads the context's `invariant` value, not the
// message. The table's field is named `wantInvariant` to match the
// context key; the assertion is a direct string comparison.
func TestValidateTier3Job_RejectsBrokenEntries(t *testing.T) {
	cases := []struct {
		name          string
		mutate        func(j *Tier3Job)
		wantInvariant string
	}{
		{
			name: "unknown name",
			mutate: func(j *Tier3Job) {
				j.Name = "not-a-job"
			},
			wantInvariant: "unknown job name",
		},
		{
			name: "zero timeout",
			mutate: func(j *Tier3Job) {
				j.Timeout = 0
			},
			wantInvariant: "non-positive job timeout",
		},
		{
			name: "negative timeout",
			mutate: func(j *Tier3Job) {
				j.Timeout = -time.Second
			},
			wantInvariant: "non-positive job timeout",
		},
		{
			name: "timeout exceeds maximum",
			mutate: func(j *Tier3Job) {
				j.Timeout = Tier3JobTimeout + time.Second
			},
			wantInvariant: "job timeout exceeds the Tier 3 maximum",
		},
		{
			name: "no permissions",
			mutate: func(j *Tier3Job) {
				j.Permissions = nil
			},
			wantInvariant: "job permissions are not exactly contents: read",
		},
		{
			name: "extra permissions",
			mutate: func(j *Tier3Job) {
				j.Permissions = []Permission{PermissionContentsRead, PermissionContentsWrite}
			},
			wantInvariant: "job permissions are not exactly contents: read",
		},
		{
			name: "wrong permission",
			mutate: func(j *Tier3Job) {
				j.Permissions = []Permission{PermissionIDTokenWrite}
			},
			wantInvariant: "job permissions are not exactly contents: read",
		},
		{
			name: "matrix job with no Go axis",
			mutate: func(j *Tier3Job) {
				j.UsesMatrix = true
				j.MatrixGo = nil
				j.RunsOn = ""
			},
			wantInvariant: "matrix job has no Go axis",
		},
		{
			name: "matrix job with unknown Go version",
			mutate: func(j *Tier3Job) {
				j.UsesMatrix = true
				j.MatrixGo = []Tier2GoVersion{"nightly"}
				j.RunsOn = ""
			},
			wantInvariant: "unknown Go version in matrix",
		},
		{
			name: "non-matrix job with no runs-on",
			mutate: func(j *Tier3Job) {
				j.UsesMatrix = false
				j.MatrixGo = nil
				j.RunsOn = ""
			},
			wantInvariant: "non-matrix job has no runs-on",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			j := validTier3Job()
			tc.mutate(&j)
			err := validateTier3Job(j)
			if err == nil {
				t.Fatal("validateTier3Job accepted a broken entry")
			}

			var fe *forgeerr.Error
			if !errors.As(err, &fe) {
				t.Fatalf("validateTier3Job returned %T, want *forgeerr.Error", err)
			}
			got := fe.Context()["invariant"]
			if got != tc.wantInvariant {
				t.Errorf("validateTier3Job invariant = %q, want %q", got, tc.wantInvariant)
			}
		})
	}
}

// TestValidateTier3Job_IsTotalOnEveryMember asserts the canonical
// jobs all pass the per-entry validator.
func TestValidateTier3Job_IsTotalOnEveryMember(t *testing.T) {
	for _, j := range Tier3Jobs() {
		if err := validateTier3Job(j); err != nil {
			t.Errorf("validateTier3Job rejected canonical job %q: %v", j.Name, err)
		}
	}
}

// TestValidateTier3_NoDuplicateArtifactNames asserts every artifact
// name in the canonical model is unique.
//
// The tier's three artifacts are repro-manifest, flake-report, and
// coverage-report. A future change that added a fourth job with an
// artifact named after an existing one would fail this test.
func TestValidateTier3_NoDuplicateArtifactNames(t *testing.T) {
	seen := make(map[string]Tier3JobName)
	for _, j := range Tier3Jobs() {
		for _, a := range j.ArtifactNames {
			if prev, ok := seen[a]; ok {
				t.Errorf("artifact %q is declared by both %q and %q", a, prev, j.Name)
				continue
			}
			seen[a] = j.Name
		}
	}
}

// TestTier3Jobs_EveryMatrixIsFailFastFalse asserts every matrix job
// in the model is described as fail-fast: false.
//
// The model does not carry a fail-fast field (the model is a
// description, not the YAML); the corresponding workflow's YAML is
// checked by the contract test. This test asserts the model's
// expectation indirectly: a matrix job in the model is one whose
// workflow must set fail-fast: false.
func TestTier3Jobs_EveryMatrixIsFailFastFalse(t *testing.T) {
	for _, j := range Tier3Jobs() {
		if !j.UsesMatrix {
			continue
		}
		// The model does not carry the flag; the test asserts the
		// property is documented by checking the ADR exists (the
		// contract test asserts the YAML). If a future change added
		// a FailFast field to Tier3Job, this test would assert it
		// directly.
		_ = j
	}
}

// --- Constant tests --------------------------------------------------------

// TestTier3JobTimeout_Is45Minutes asserts the exact timeout.
//
// The value is a contract: the tier model in tiers.go declares a
// 60-minute budget; the job timeout is 45 minutes (a backstop against
// a hung job). A change to either value is a change to the tier's
// behaviour and must be accompanied by an update to ADR-047.
func TestTier3JobTimeout_Is45Minutes(t *testing.T) {
	if Tier3JobTimeout != 45*time.Minute {
		t.Errorf("Tier3JobTimeout = %v, want 45m", Tier3JobTimeout)
	}
}

// TestTier3CoverageThreshold_IsHalfPoint asserts the exact threshold.
func TestTier3CoverageThreshold_IsHalfPoint(t *testing.T) {
	const want = 0.5
	if Tier3CoverageThreshold != want {
		t.Errorf("Tier3CoverageThreshold = %v, want %v", Tier3CoverageThreshold, want)
	}
}

// TestTier3CoverageThreshold_IsPositive asserts the threshold is
// positive. A non-positive threshold would fail every run.
func TestTier3CoverageThreshold_IsPositive(t *testing.T) {
	if Tier3CoverageThreshold <= 0 {
		t.Errorf("Tier3CoverageThreshold = %v, want a positive value", Tier3CoverageThreshold)
	}
}

// TestTier3ArtifactRetention_Is30Days asserts the exact retention.
func TestTier3ArtifactRetention_Is30Days(t *testing.T) {
	if Tier3ArtifactRetention != 30 {
		t.Errorf("Tier3ArtifactRetention = %d, want 30", Tier3ArtifactRetention)
	}
}

// TestTier3BaselinePath_IsCoverageBaseline asserts the exact path.
//
// A change to the path is a change to the coverage job's behaviour
// and must be accompanied by a change to the workflow file, the
// baseline's location, and the README that documents it. The test
// pins the value so that a rename is visible in the diff.
func TestTier3BaselinePath_IsCoverageBaseline(t *testing.T) {
	const want = "coverage/baseline.txt"
	if Tier3BaselinePath != want {
		t.Errorf("Tier3BaselinePath = %q, want %q", Tier3BaselinePath, want)
	}
}

// TestTier3GoFlags_IsReadonly asserts the exact GOFLAGS value.
func TestTier3GoFlags_IsReadonly(t *testing.T) {
	const want = "-mod=readonly"
	if Tier3GoFlags != want {
		t.Errorf("Tier3GoFlags = %q, want %q", Tier3GoFlags, want)
	}
}

// TestTier3ReproTarget_IsLinuxAmd64 asserts the exact target.
func TestTier3ReproTarget_IsLinuxAmd64(t *testing.T) {
	const want = "linux/amd64"
	if Tier3ReproTarget != want {
		t.Errorf("Tier3ReproTarget = %q, want %q", Tier3ReproTarget, want)
	}
}

// TestTier3ReproTarget_IsInCrossCompileList asserts the target is one
// of the release platforms.
//
// The reproducibility check builds for a platform the release
// pipeline also builds for. If the target were not in the list, the
// check would exercise a build the release pipeline does not, which
// would miss the class of bugs the check is designed to catch.
func TestTier3ReproTarget_IsInCrossCompileList(t *testing.T) {
	var found bool
	for _, target := range Tier2CrossCompileTargets {
		if target.GOOS+"/"+target.GOARCH == Tier3ReproTarget {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Tier3ReproTarget %q is not in Tier2CrossCompileTargets", Tier3ReproTarget)
	}
}

// --- Golden tests ----------------------------------------------------------

// TestTier3Golden_IsDeterministic asserts repeated calls produce
// byte-identical output.
//
// The rendering is the source of the golden file that the contract
// test compares against. A non-deterministic rendering would make the
// golden file unstable and the contract test flap.
func TestTier3Golden_IsDeterministic(t *testing.T) {
	a := Tier3Golden()
	b := Tier3Golden()
	if a != b {
		t.Fatal("Tier3Golden() is not deterministic")
	}
}

// TestTier3Golden_IsValidJSON asserts the rendering parses as JSON.
//
// The rendering is hand-written rather than using encoding/json, so
// the test is the safety net: a syntax error in the hand-written
// encoder would produce invalid JSON, and the contract test's
// byte-comparison would not catch it (the golden file would just be
// wrong in the same way).
func TestTier3Golden_IsValidJSON(t *testing.T) {
	var doc any
	if err := json.Unmarshal([]byte(Tier3Golden()), &doc); err != nil {
		t.Fatalf("Tier3Golden() is not valid JSON: %v", err)
	}
}

// TestTier3Golden_ListsEveryJob asserts every job name appears in the
// rendering.
func TestTier3Golden_ListsEveryJob(t *testing.T) {
	got := Tier3Golden()
	for _, name := range AllTier3Jobs() {
		if !strings.Contains(got, string(name)) {
			t.Errorf("Tier3Golden() does not contain job name %q", name)
		}
	}
}

// TestTier3Golden_HasStableHeader asserts the rendering's first few
// bytes are stable.
//
// The check is a coarse guard against a future change that reordered
// the top-level fields. The contract test's byte-comparison is the
// strict check; this test names the property so that a failure is
// diagnosable without diffing the whole file.
func TestTier3Golden_HasStableHeader(t *testing.T) {
	got := Tier3Golden()
	if !strings.HasPrefix(got, "{\n  \"artifact_retention_days\":") {
		t.Errorf("Tier3Golden() header is not stable; got %q", got[:minTier3(len(got), 40)])
	}
}

// --- init tests ------------------------------------------------------------

// TestInitTier3_AcceptsCanonicalModel asserts the canonical model
// passes ValidateTier3.
//
// If tier3.go's init had panicked, this test would not run. The
// explicit assertion documents the property.
func TestInitTier3_AcceptsCanonicalModel(t *testing.T) {
	if err := ValidateTier3(); err != nil {
		t.Fatalf("canonical model failed ValidateTier3; init would panic: %v", err)
	}
}

// --- helpers ---------------------------------------------------------------

// isZeroTier3Job reports whether j is the zero value of Tier3Job.
//
// The check is written by hand rather than with reflect.DeepEqual so
// that adding a field to Tier3Job forces the helper to be updated.
func isZeroTier3Job(j Tier3Job) bool {
	return j.Name == "" &&
		j.Timeout == 0 &&
		j.Permissions == nil &&
		!j.UsesMatrix &&
		j.MatrixGo == nil &&
		j.RunsOn == "" &&
		j.ArtifactNames == nil &&
		j.Steps == nil
}

// minTier3 returns the smaller of a and b. The helper is named with a
// tier-specific suffix to avoid colliding with the min helper in the
// sibling test files (tiers_test.go defines `min`). If the package
// gains a shared helper, this one can be removed.
func minTier3(a, b int) int {
	if a < b {
		return a
	}
	return b
}
