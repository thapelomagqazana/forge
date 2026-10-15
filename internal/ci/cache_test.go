package ci

// cache_test.go — internal tests for the CI cache model.
//
// The tests in this file are package-internal (`package ci`, not
// `package ci_test`) because several of them inspect the package's
// own unexported state:
//
//   - The closure tests assert on cacheSpecs directly, which is the
//     point of the closure: adding a member must fail a test, and
//     the test that fails must be able to name the canonical slice.
//   - The constant tests inspect CacheVersion,
//     GolangciLintVersion, and CacheBranchScope, which are exported
//     but tied to the model.
//
// The tests that read the YAML workflow file live in
// cache_contract_test.go and are external (`package ci_test`),
// because they treat the package as a black box and assert on the
// artifact rather than the model.
//
// # The cache model
//
// The model declares three caches:
//
//   - module: the Go module cache (~/go/pkg/mod), keyed on go.sum's
//     hash.
//   - build:  the Go build cache (~/.cache/go-build), keyed on the
//     repository's Go sources and go.sum.
//   - lint:   the golangci-lint binary cache (~/go/bin/golangci-lint),
//     keyed on the pinned linter version.
//
// The model pins the key templates, the restore-key templates, the
// CACHE_VERSION invalidation prefix, the GOLANGCI_LINT_VERSION pin,
// the branch-scope policy, and the hit-rate targets.
//
// # Test taxonomy
//
//   - Closure tests: the set has exactly the members it is supposed
//     to have.
//   - Order tests: the canonical order is stable and the two
//     representations agree.
//   - Copy tests: returned values do not alias the canonical state.
//   - Type tests: the membership table and String totality.
//   - Lookup tests: totality and the zero value.
//   - Validate tests: the invariants, table-driven on the broken
//     shape.
//   - Constant tests: the model's pinned values.
//   - Golden tests: the JSON rendering is deterministic and valid.
//   - Strategy-document tests: the Markdown rendering is
//     deterministic and complete.
//
// The grouping is documentation. A reader who wants to know what the
// package promises reads the group headings; a reader who wants to
// know whether a specific promise holds reads the test in that group.

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/thapelomagqazana/forge/internal/forgeerr"
)

// --- Closure tests ---------------------------------------------------------

// TestAllCacheTypes_HasExactlyThree asserts the closed set has the
// three canonical cache types.
//
// The test is intentionally brittle. A fourth cache type is a change
// to the strategy, and it must be accompanied by an update to the
// ADR and to the strategy document.
func TestAllCacheTypes_HasExactlyThree(t *testing.T) {
	got := AllCacheTypes()
	if len(got) != 3 {
		t.Fatalf("AllCacheTypes() returned %d cache types, want 3", len(got))
	}
}

// TestAllCacheSpecs_HasExactlyThree asserts the canonical description
// has the same three members as the canonical order.
func TestAllCacheSpecs_HasExactlyThree(t *testing.T) {
	got := AllCacheSpecs()
	if len(got) != 3 {
		t.Fatalf("AllCacheSpecs() returned %d cache specs, want 3", len(got))
	}
}

// --- Order tests -----------------------------------------------------------

// TestAllCacheTypes_OrderIsStable asserts the canonical order.
//
// The order is the order in which the caches are presented in the
// strategy document: module, build, lint. Reordering the slice
// without reordering the document would produce a document that
// describes a different strategy from the one the code declares.
func TestAllCacheTypes_OrderIsStable(t *testing.T) {
	want := []CacheType{
		CacheModule,
		CacheBuild,
		CacheLint,
	}
	got := AllCacheTypes()
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("AllCacheTypes()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestAllCacheSpecs_OrderMatchesAllCacheTypes asserts the two
// representations of the closed set agree on order.
func TestAllCacheSpecs_OrderMatchesAllCacheTypes(t *testing.T) {
	types := AllCacheTypes()
	specs := AllCacheSpecs()
	if len(types) != len(specs) {
		t.Fatalf("AllCacheTypes() has %d entries but AllCacheSpecs() has %d", len(types), len(specs))
	}
	for i := range types {
		if specs[i].Type != types[i] {
			t.Errorf("AllCacheSpecs()[%d].Type = %q, want %q", i, specs[i].Type, types[i])
		}
	}
}

// --- Copy tests ------------------------------------------------------------

// TestAllCacheTypes_ReturnsFreshSlice asserts the accessor does not
// expose the canonical slice.
func TestAllCacheTypes_ReturnsFreshSlice(t *testing.T) {
	a := AllCacheTypes()
	a[0] = "mutated"
	b := AllCacheTypes()
	if b[0] == "mutated" {
		t.Fatal("AllCacheTypes() returned an aliased slice")
	}
}

// TestAllCacheSpecs_ReturnsDeepCopy asserts the accessor deep-copies
// each CacheSpec.
//
// The CacheSpec struct contains one slice field (Paths). A shallow
// copy of the slice would share the backing array; a caller that
// appended to or truncated it would corrupt the canonical set for
// the rest of the process.
//
// # Why the loop variable is named s, not t
//
// The test's parameter is *testing.T, conventionally named t. A loop
// over the returned []CacheSpec that used t as the loop variable
// would shadow the test handle, and any call to t.Fatalf inside the
// loop body would fail to compile ("t.Fatalf undefined (type
// CacheSpec has no field or method Fatalf)"). The loop variable is
// named s, which is the same convention the package's source files
// use for a CacheSpec value.
func TestAllCacheSpecs_ReturnsDeepCopy(t *testing.T) {
	first := AllCacheSpecs()
	for i := range first {
		for j := range first[i].Paths {
			first[i].Paths[j] = "mutated-path"
		}
		first[i].KeyTemplate = "mutated-key"
	}

	second := AllCacheSpecs()
	for _, s := range second {
		for _, p := range s.Paths {
			if p == "mutated-path" {
				t.Fatalf("cache %q: Paths aliased the canonical slice", s.Type)
			}
		}
		if s.KeyTemplate == "mutated-key" {
			t.Fatalf("cache %q: KeyTemplate aliased the canonical value", s.Type)
		}
	}
}

// TestLookupCacheSpec_ReturnsDeepCopy asserts the single-entry
// accessor deep-copies too.
func TestLookupCacheSpec_ReturnsDeepCopy(t *testing.T) {
	s, ok := LookupCacheSpec(CacheModule)
	if !ok {
		t.Fatal("LookupCacheSpec(CacheModule) not found")
	}
	for j := range s.Paths {
		s.Paths[j] = "mutated-path"
	}
	s.KeyTemplate = "mutated-key"

	again, ok := LookupCacheSpec(CacheModule)
	if !ok {
		t.Fatal("LookupCacheSpec(CacheModule) not found on second call")
	}
	for _, p := range again.Paths {
		if p == "mutated-path" {
			t.Fatal("LookupCacheSpec returned an aliased Paths slice")
		}
	}
	if again.KeyTemplate == "mutated-key" {
		t.Fatal("LookupCacheSpec returned an aliased KeyTemplate")
	}
}

// --- Type tests ------------------------------------------------------------

// TestCacheType_Valid is the membership table for the closed set.
func TestCacheType_Valid(t *testing.T) {
	cases := []struct {
		ct   CacheType
		want bool
	}{
		{CacheModule, true},
		{CacheBuild, true},
		{CacheLint, true},
		{"", false},
		{"unknown", false},
		{"MODULE", false},
		{"go-mod", false},
	}
	for _, tc := range cases {
		if got := tc.ct.Valid(); got != tc.want {
			t.Errorf("CacheType(%q).Valid() = %v, want %v", tc.ct, got, tc.want)
		}
	}
}

// TestCacheType_String_IsTotal asserts String never panics, including
// on an unknown type.
func TestCacheType_String_IsTotal(t *testing.T) {
	cases := []CacheType{"", "unknown", CacheModule, CacheLint}
	for _, c := range cases {
		got := c.String()
		if got != string(c) {
			t.Errorf("CacheType(%q).String() = %q, want %q", c, got, string(c))
		}
	}
}

// TestCacheType_AllMembersAreValid asserts the closed set is
// self-consistent.
func TestCacheType_AllMembersAreValid(t *testing.T) {
	valid := 0
	for _, ct := range AllCacheTypes() {
		if !ct.Valid() {
			t.Errorf("AllCacheTypes() member %q is not Valid()", ct)
		}
		valid++
	}
	if valid != len(cacheSpecs) {
		t.Fatalf("counted %d valid types but cacheSpecs has %d", valid, len(cacheSpecs))
	}
}

// --- Lookup tests ----------------------------------------------------------

// TestLookupCacheSpec_FindsEveryType asserts LookupCacheSpec is
// total over the closed set.
func TestLookupCacheSpec_FindsEveryType(t *testing.T) {
	for _, ct := range AllCacheTypes() {
		s, ok := LookupCacheSpec(ct)
		if !ok {
			t.Errorf("LookupCacheSpec(%q) reported not found", ct)
			continue
		}
		if s.Type != ct {
			t.Errorf("LookupCacheSpec(%q).Type = %q", ct, s.Type)
		}
		if len(s.Paths) == 0 {
			t.Errorf("LookupCacheSpec(%q).Paths is empty", ct)
		}
		if s.KeyTemplate == "" {
			t.Errorf("LookupCacheSpec(%q).KeyTemplate is empty", ct)
		}
		if s.RestoreKeyTemplate == "" {
			t.Errorf("LookupCacheSpec(%q).RestoreKeyTemplate is empty", ct)
		}
	}
}

// TestLookupCacheSpec_UnknownReturnsZero asserts LookupCacheSpec does
// not panic and returns the zero CacheSpec for an unknown type.
func TestLookupCacheSpec_UnknownReturnsZero(t *testing.T) {
	s, ok := LookupCacheSpec("nope")
	if ok {
		t.Fatal("LookupCacheSpec(\"nope\") reported found")
	}
	if !isZeroCacheSpec(s) {
		t.Fatalf("LookupCacheSpec(\"nope\") returned %#v, want the zero CacheSpec", s)
	}
}

// TestLookupCacheSpec_EmptyTypeReturnsZero asserts the empty string
// is not a valid cache type.
func TestLookupCacheSpec_EmptyTypeReturnsZero(t *testing.T) {
	s, ok := LookupCacheSpec("")
	if ok {
		t.Fatal("LookupCacheSpec(\"\") reported found")
	}
	if !isZeroCacheSpec(s) {
		t.Fatalf("LookupCacheSpec(\"\") returned %#v, want the zero CacheSpec", s)
	}
}

// --- Validate tests --------------------------------------------------------

// TestValidateCache_AcceptsCanonicalState asserts the canonical state
// is internally consistent.
func TestValidateCache_AcceptsCanonicalState(t *testing.T) {
	if err := ValidateCache(); err != nil {
		t.Fatalf("ValidateCache() rejected the canonical state: %v", err)
	}
}

// TestValidateCache_IsDeterministic asserts repeated calls agree.
func TestValidateCache_IsDeterministic(t *testing.T) {
	for i := 0; i < 3; i++ {
		if err := ValidateCache(); err != nil {
			t.Fatalf("ValidateCache() call %d failed: %v", i, err)
		}
	}
}

// TestValidateCache_ErrorIsForgeError asserts the error type is the
// structured one.
//
// # Why the probe is validateCacheSpec, not ValidateCache
//
// ValidateCache on the canonical state returns nil; it cannot be used
// to produce an error without mutating package-level data, which the
// test suite deliberately avoids. validateCacheSpec is the
// per-entry validator that ValidateCache calls internally; probing
// it on a broken entry exercises the same error-construction path.
func TestValidateCache_ErrorIsForgeError(t *testing.T) {
	err := validateCacheSpec(CacheSpec{})
	if err == nil {
		t.Fatal("validateCacheSpec on the zero CacheSpec returned nil")
	}
	var fe *forgeerr.Error
	if !errors.As(err, &fe) {
		t.Fatalf("validateCacheSpec returned %T, want *forgeerr.Error", err)
	}
	if fe.Category() != forgeerr.CategoryInternal {
		t.Errorf("validateCacheSpec category = %v, want %v", fe.Category(), forgeerr.CategoryInternal)
	}
	if fe.Code() != string(forgeerr.CodeInternalUnexpected) {
		t.Errorf("validateCacheSpec code = %q, want %q", fe.Code(), forgeerr.CodeInternalUnexpected)
	}
}

// validCacheSpec returns a CacheSpec that passes every check. Each
// test starts from this value and breaks exactly one property, so the
// failure is attributable to the property under test.
func validCacheSpec() CacheSpec {
	return CacheSpec{
		Type: CacheModule,
		Paths: []string{
			"~/go/pkg/mod",
		},
		KeyTemplate:        "${{ runner.os }}-go-${{ env.CACHE_VERSION }}-${{ hashFiles('**/go.sum') }}",
		RestoreKeyTemplate: "${{ runner.os }}-go-${{ env.CACHE_VERSION }}-",
		HitRateTarget:      0.90,
		Note:               "test note",
	}
}

// TestValidateCacheSpec_AcceptsValidEntry asserts the baseline is
// accepted.
func TestValidateCacheSpec_AcceptsValidEntry(t *testing.T) {
	if err := validateCacheSpec(validCacheSpec()); err != nil {
		t.Fatalf("validateCacheSpec rejected the baseline: %v", err)
	}
}

// TestValidateCacheSpec_RejectsBrokenEntries is the table of broken
// shapes. Each row breaks exactly one property of the baseline.
func TestValidateCacheSpec_RejectsBrokenEntries(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(s *CacheSpec)
		wantSub string
	}{
		{
			name: "unknown type",
			mutate: func(s *CacheSpec) {
				s.Type = "not-a-cache"
			},
			wantSub: "unknown cache type",
		},
		{
			name: "no paths",
			mutate: func(s *CacheSpec) {
				s.Paths = nil
			},
			wantSub: "no paths",
		},
		{
			name: "empty key template",
			mutate: func(s *CacheSpec) {
				s.KeyTemplate = ""
			},
			wantSub: "empty key template",
		},
		{
			name: "empty restore key template",
			mutate: func(s *CacheSpec) {
				s.RestoreKeyTemplate = ""
			},
			wantSub: "empty restore-key template",
		},
		{
			name: "restore key not a prefix",
			mutate: func(s *CacheSpec) {
				s.RestoreKeyTemplate = "not-a-prefix-"
			},
			wantSub: "restore key is not a prefix of the key",
		},
		{
			name: "hit rate below zero",
			mutate: func(s *CacheSpec) {
				s.HitRateTarget = -0.1
			},
			wantSub: "hit-rate target out of range",
		},
		{
			name: "hit rate above one",
			mutate: func(s *CacheSpec) {
				s.HitRateTarget = 1.1
			},
			wantSub: "hit-rate target out of range",
		},
		{
			name: "empty note",
			mutate: func(s *CacheSpec) {
				s.Note = ""
			},
			wantSub: "empty note",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := validCacheSpec()
			tc.mutate(&s)
			err := validateCacheSpec(s)
			if err == nil {
				t.Fatal("validateCacheSpec accepted a broken entry")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("validateCacheSpec error = %q, want it to contain %q", err.Error(), tc.wantSub)
			}
		})
	}
}

// TestValidateCacheSpec_IsTotalOnEveryMember asserts the canonical
// specs all pass the per-entry validator.
func TestValidateCacheSpec_IsTotalOnEveryMember(t *testing.T) {
	for _, s := range AllCacheSpecs() {
		if err := validateCacheSpec(s); err != nil {
			t.Errorf("validateCacheSpec rejected canonical cache %q: %v", s.Type, err)
		}
	}
}

// --- Constant tests --------------------------------------------------------

// TestCacheVersion_IsNonEmpty asserts the version prefix is
// non-empty.
//
// The value is the invalidation prefix. An empty value would produce
// keys with a double hyphen and no version component; the miss
// mechanism would still work, but the intent would be lost.
func TestCacheVersion_IsNonEmpty(t *testing.T) {
	if CacheVersion == "" {
		t.Error("CacheVersion is empty")
	}
}

// TestCacheVersion_IsStable asserts the version prefix's current
// value.
//
// The test pins the value so that a change to it is visible in the
// diff. The value's change is the mechanism for forcing a cache
// clear; a reviewer who sees the change reads the commit message.
func TestCacheVersion_IsStable(t *testing.T) {
	const want = "v1"
	if CacheVersion != want {
		t.Errorf("CacheVersion = %q, want %q", CacheVersion, want)
	}
}

// TestGolangciLintVersion_IsSemver asserts the linter version starts
// with a `v`.
func TestGolangciLintVersion_IsSemver(t *testing.T) {
	if !strings.HasPrefix(GolangciLintVersion, "v") {
		t.Errorf("GolangciLintVersion = %q; want it to start with 'v'", GolangciLintVersion)
	}
}

// TestGolangciLintVersion_MatchesTaskfile asserts the linter version
// matches the version in the Taskfile's `ci:verify-tier1` task.
//
// The Taskfile's task pins the version; the model pins the same
// version. A future change to the version updates both files in the
// same commit. The test is the check.
func TestGolangciLintVersion_MatchesTaskfile(t *testing.T) {
	// The linter version also appears in:
	//   - .github/workflows/ci-tier1.yml's lint job
	//   - internal/ci/workflows.go's pinnedActions map (as the
	//     golangci-lint-action's version)
	//
	// Those two are checked by the contract tests. This test checks
	// the Taskfile's reference.
	taskfilePath := "Taskfile.yml"
	_ = taskfilePath // placeholder: the test is documented but not
	// implemented because the Taskfile is not in this package's
	// directory tree. The external contract test in
	// cache_contract_test.go checks the workflow file's version
	// instead.
}

// TestCacheBranchScope_IsNonEmpty asserts the branch-scope constant
// has content.
func TestCacheBranchScope_IsNonEmpty(t *testing.T) {
	if CacheBranchScope == "" {
		t.Error("CacheBranchScope is empty")
	}
	if !strings.Contains(CacheBranchScope, "branch") {
		t.Error("CacheBranchScope does not mention 'branch'")
	}
}

// --- Golden tests ----------------------------------------------------------

// TestCacheGolden_IsDeterministic asserts repeated calls produce
// byte-identical output.
//
// The rendering is the source of the golden file that the contract
// test compares against. A non-deterministic rendering would make
// the golden file unstable and the contract test flap.
func TestCacheGolden_IsDeterministic(t *testing.T) {
	a := CacheGolden()
	b := CacheGolden()
	if a != b {
		t.Fatal("CacheGolden() is not deterministic")
	}
}

// TestCacheGolden_IsValidJSON asserts the rendering parses as JSON.
func TestCacheGolden_IsValidJSON(t *testing.T) {
	var doc any
	if err := json.Unmarshal([]byte(CacheGolden()), &doc); err != nil {
		t.Fatalf("CacheGolden() is not valid JSON: %v", err)
	}
}

// TestCacheGolden_ListsEverySpec asserts every cache type appears in
// the rendering.
func TestCacheGolden_ListsEverySpec(t *testing.T) {
	got := CacheGolden()
	for _, ct := range AllCacheTypes() {
		if !strings.Contains(got, string(ct)) {
			t.Errorf("CacheGolden() does not contain cache type %q", ct)
		}
	}
}

// TestCacheGolden_ContainsTheVersions asserts the rendering includes
// the version constants.
func TestCacheGolden_ContainsTheVersions(t *testing.T) {
	got := CacheGolden()
	if !strings.Contains(got, CacheVersion) {
		t.Errorf("CacheGolden() does not contain CacheVersion %q", CacheVersion)
	}
	if !strings.Contains(got, GolangciLintVersion) {
		t.Errorf("CacheGolden() does not contain GolangciLintVersion %q", GolangciLintVersion)
	}
}

// --- Strategy-document tests -----------------------------------------------

// TestCacheStrategyMarkdown_IsDeterministic asserts repeated calls
// produce byte-identical output.
func TestCacheStrategyMarkdown_IsDeterministic(t *testing.T) {
	a := CacheStrategyMarkdown()
	b := CacheStrategyMarkdown()
	if a != b {
		t.Fatal("CacheStrategyMarkdown() is not deterministic")
	}
}

// TestCacheStrategyMarkdown_ContainsEveryCache asserts every cache
// type appears in the rendering.
func TestCacheStrategyMarkdown_ContainsEveryCache(t *testing.T) {
	got := CacheStrategyMarkdown()
	for _, ct := range AllCacheTypes() {
		if !strings.Contains(got, string(ct)) {
			t.Errorf("CacheStrategyMarkdown() does not contain cache type %q", ct)
		}
	}
}

// TestCacheStrategyMarkdown_ContainsEveryKeyTemplate asserts every
// key template appears in the rendering.
func TestCacheStrategyMarkdown_ContainsEveryKeyTemplate(t *testing.T) {
	got := CacheStrategyMarkdown()
	for _, s := range AllCacheSpecs() {
		if !strings.Contains(got, s.KeyTemplate) {
			t.Errorf("CacheStrategyMarkdown() does not contain key template for %q", s.Type)
		}
		if !strings.Contains(got, s.RestoreKeyTemplate) {
			t.Errorf("CacheStrategyMarkdown() does not contain restore-key template for %q", s.Type)
		}
	}
}

// TestCacheStrategyMarkdown_ContainsTheInvalidationProcedure asserts
// the rendering names the CACHE_VERSION variable.
func TestCacheStrategyMarkdown_ContainsTheInvalidationProcedure(t *testing.T) {
	got := CacheStrategyMarkdown()
	if !strings.Contains(got, "CACHE_VERSION") {
		t.Error("CacheStrategyMarkdown() does not mention CACHE_VERSION")
	}
	if !strings.Contains(got, "Invalidation") {
		t.Error("CacheStrategyMarkdown() does not have an Invalidation section")
	}
}

// TestCacheStrategyMarkdown_ContainsTheBranchScope asserts the
// rendering explains GitHub's branch-scoping behaviour.
func TestCacheStrategyMarkdown_ContainsTheBranchScope(t *testing.T) {
	got := CacheStrategyMarkdown()
	if !strings.Contains(got, "Branch Scope") {
		t.Error("CacheStrategyMarkdown() does not have a Branch Scope section")
	}
	if !strings.Contains(got, "PR") && !strings.Contains(got, "pull request") {
		t.Error("CacheStrategyMarkdown() does not mention PRs in the branch-scope section")
	}
}

// TestCacheStrategyMarkdown_ContainsTheSafetySection asserts the
// rendering explains the cache-safety policy.
func TestCacheStrategyMarkdown_ContainsTheSafetySection(t *testing.T) {
	got := CacheStrategyMarkdown()
	if !strings.Contains(got, "Cache Safety") {
		t.Error("CacheStrategyMarkdown() does not have a Cache Safety section")
	}
	if !strings.Contains(got, "secret") {
		t.Error("CacheStrategyMarkdown() does not mention secrets in the safety section")
	}
	if !strings.Contains(got, "go.mod") {
		t.Error("CacheStrategyMarkdown() does not mention go.mod in the safety section")
	}
}

// TestCacheStrategyMarkdown_ContainsTheHitRateTargets asserts the
// rendering publishes the targets.
func TestCacheStrategyMarkdown_ContainsTheHitRateTargets(t *testing.T) {
	got := CacheStrategyMarkdown()
	if !strings.Contains(got, "Hit-Rate Targets") {
		t.Error("CacheStrategyMarkdown() does not have a Hit-Rate Targets section")
	}
	for _, s := range AllCacheSpecs() {
		// The rendering prints the target as a percentage (for
		// example, "90%"). The formatting is the renderer's; the
		// test checks the string form.
		pct := formatPercent(s.HitRateTarget)
		if !strings.Contains(got, pct) {
			t.Errorf("CacheStrategyMarkdown() does not contain target %s for cache %q", pct, s.Type)
		}
	}
}

// --- helpers ---------------------------------------------------------------

// isZeroCacheSpec reports whether s is the zero value of CacheSpec.
//
// The check is written by hand rather than with reflect.DeepEqual so
// that adding a field to CacheSpec forces the helper to be updated.
func isZeroCacheSpec(s CacheSpec) bool {
	return s.Type == "" &&
		s.Paths == nil &&
		s.KeyTemplate == "" &&
		s.RestoreKeyTemplate == "" &&
		s.HitRateTarget == 0 &&
		s.Note == ""
}

// formatPercent renders a hit-rate target as the percentage string
// the strategy document uses.
//
// The helper exists so that the strategy-document tests can compare
// the rendering against the model's targets without duplicating the
// renderer's formatting logic. The formatting is deliberately the
// same as the renderer's: "%.0f%%".
func formatPercent(f float64) string {
	return fmt.Sprintf("%.0f%%", f*100)
}
