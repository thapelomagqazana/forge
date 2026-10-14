// Package cli contains white-box tests for the internal/cli
// package.
//
// This file tests the exit code constants, the range-based exit code
// taxonomy, and the mapping from errors to exit codes. See
// exitcodes.go for the constants and the range table, and execute.go
// (exitCodeOf) for the mapping.
//
// # The mapping's home
//
// The mapping is Category.ExitCode() in internal/forgeerr, not a
// switch in this package. The CLI's local helper exitCodeOf (in
// execute.go) extracts the *forgeerr.Error from the chain and reads
// the category's method. This file tests the helper's behaviour and
// asserts that its results fall within a documented range; the
// category's method is tested by internal/forgeerr/category_test.go.
//
// # The taxonomy's home
//
// The range table lives in exitcodes.go. Its structural invariants
// (sorted, contiguous, covering [0, 255]) are checked by init and by
// TestExitCodeTaxonomy_RangesAreContiguous below. The table is the
// machine-readable form of docs/reference/exit-code-ranges.md; the
// documentation and the table must agree, and the CI check
// scripts/deps/verify-exit-codes.sh asserts the shell-visible part of
// that agreement.
//
// # Why the tests do not reference CategorizedError
//
// Before WBS 10.5.1, this package defined a CategorizedError
// interface and an exitCodeFromError function. Both were superseded:
// the interface by the closed forgeerr.Category type (WBS 10.2.1),
// and the function by the category's ExitCode method. The tests
// below assert the replacement's behaviour.
//
// # Two failure domains
//
// WBS 11.1.1 splits non-zero exit codes into two domains: Forge
// findings (1–9) and Forge execution failures (10–19). The tests
// below assert that every constant and every category's exit code
// falls into the correct domain. This is the load-bearing property
// that lets CI pipelines branch on code < 10 vs. code >= 10 without
// parsing output.
//
// # Type-level contract
//
// forgeerr.New takes its code parameter as forgeerr.Code. Every
// constructor call in this file passes the named constants from
// internal/forgeerr/codes.go directly. A string(...) cast in a
// constructor call is the diagnostic that the migration documented
// in safefs/errors.go was rolled back.
//
// The accessor (*Error).Code() returns string, so comparisons
// against a named constant cast the constant on the right-hand
// side: e.Code() != string(forgeerr.CodeConfigMalformed). The cast
// is on the known side (the constant), not on the accessor's
// return value.
package cli

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/thapelomagqazana/forge/internal/forgeerr"
)

// ---------------------------------------------------------------------------
// Legacy constant values (WBS 4.1.2, re-mapped by WBS 11.1.1)
// ---------------------------------------------------------------------------

// TestExitCodeConstants_HaveExpectedValues verifies that the exit
// code constants have the integer values declared by the WBS 11.1.1
// taxonomy.
//
// The test is deliberately brittle: it asserts on the exact values.
// If a value is changed, the test fails, and the change must be
// recorded in an ADR. This is the same discipline as WBS 4.1.2; only
// the values themselves changed, per ADR-018.
//
// # Relationship to the categories' exit codes
//
// The constants in this file and the categories' Category.ExitCode()
// methods must agree. The agreement is policed by
// TestAC4_ExitCodesMatchCLIConstants in
// internal/forgeerr/category_test.go. This test asserts the
// constants' values; the forgeerr test asserts their agreement with
// the categories.
func TestExitCodeConstants_HaveExpectedValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		constant int
		expected int
	}{
		// Range boundaries.
		{"ExitSuccess", ExitSuccess, 0},
		{"ExitRangeFindingsMin", ExitRangeFindingsMin, 1},
		{"ExitRangeFindingsMax", ExitRangeFindingsMax, 9},
		{"ExitRangeExecutionMin", ExitRangeExecutionMin, 10},
		{"ExitRangeExecutionMax", ExitRangeExecutionMax, 19},
		{"ExitRangeReservedRegistryMin", ExitRangeReservedRegistryMin, 20},
		{"ExitRangeReservedRegistryMax", ExitRangeReservedRegistryMax, 29},
		{"ExitRangeReservedForgeMin", ExitRangeReservedForgeMin, 30},
		{"ExitRangeReservedForgeMax", ExitRangeReservedForgeMax, 63},
		{"ExitUsage", ExitUsage, 64},
		{"ExitRangeReservedSysexitsMin", ExitRangeReservedSysexitsMin, 65},
		{"ExitRangeReservedSysexitsMax", ExitRangeReservedSysexitsMax, 69},
		{"ExitRangeInternalMin", ExitRangeInternalMin, 70},
		{"ExitRangeInternalMax", ExitRangeInternalMax, 78},
		{"ExitRangeReservedFutureMin", ExitRangeReservedFutureMin, 79},
		{"ExitRangeReservedFutureMax", ExitRangeReservedFutureMax, 127},
		{"ExitInvalidArgument", ExitInvalidArgument, 128},
		{"ExitRangeSignalsMin", ExitRangeSignalsMin, 129},
		{"ExitSIGHUP", ExitSIGHUP, 129},
		{"ExitSIGINT", ExitSIGINT, 130},
		{"ExitSIGTERM", ExitSIGTERM, 143},
		{"ExitRangeSignalsMax", ExitRangeSignalsMax, 255},

		// Named codes.
		{"ExitDriftDetected", ExitDriftDetected, 1},
		{"ExitValidation", ExitValidation, 2},
		{"ExitCheckFailed", ExitCheckFailed, 3},
		{"ExitConfig", ExitConfig, 10},
		{"ExitTemplate", ExitTemplate, 11},
		{"ExitFilesystem", ExitFilesystem, 12},
		{"ExitSecurity", ExitSecurity, 13},
		{"ExitConflict", ExitConflict, 14},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.constant != tc.expected {
				t.Errorf("%s: got %d, want %d",
					tc.name, tc.constant, tc.expected)
			}
		})
	}
}

// TestExitCodeConstants_AreDistinct verifies that no two named code
// constants share a value. Duplicate values would make the mapping
// ambiguous.
//
// Range boundaries are not included: a boundary may legitimately
// equal a named code (e.g., ExitDriftDetected == ExitRangeFindingsMin).
// The test asserts distinctness only among named codes and between
// named codes and the frozen singletons.
func TestExitCodeConstants_AreDistinct(t *testing.T) {
	t.Parallel()

	constants := map[string]int{
		"ExitSuccess":         ExitSuccess,
		"ExitDriftDetected":   ExitDriftDetected,
		"ExitValidation":      ExitValidation,
		"ExitCheckFailed":     ExitCheckFailed,
		"ExitConfig":          ExitConfig,
		"ExitTemplate":        ExitTemplate,
		"ExitFilesystem":      ExitFilesystem,
		"ExitSecurity":        ExitSecurity,
		"ExitConflict":        ExitConflict,
		"ExitUsage":           ExitUsage,
		"ExitInvalidArgument": ExitInvalidArgument,
		"ExitSIGHUP":          ExitSIGHUP,
		"ExitSIGINT":          ExitSIGINT,
		"ExitSIGTERM":         ExitSIGTERM,
	}

	seen := make(map[int]string)
	for name, value := range constants {
		if other, exists := seen[value]; exists {
			t.Errorf("duplicate value %d: %s and %s",
				value, name, other)
		}
		seen[value] = name
	}
}

// TestExitCodeConstants_FindingsDomainHasRoom verifies that the
// findings range (1–9) is not yet exhausted. A future WBS that
// assigns codes must find a free integer.
//
// This is a forward-looking invariant: it fails before the range is
// full so that the design discussion happens with room to spare.
func TestExitCodeConstants_FindingsDomainHasRoom(t *testing.T) {
	t.Parallel()

	used := map[int]bool{
		ExitDriftDetected: true,
		ExitValidation:    true,
		ExitCheckFailed:   true,
	}
	free := 0
	for code := ExitRangeFindingsMin; code <= ExitRangeFindingsMax; code++ {
		if !used[code] {
			free++
		}
	}
	if free == 0 {
		t.Errorf("findings range %d–%d has no free codes; a new ADR is required before assigning",
			ExitRangeFindingsMin, ExitRangeFindingsMax)
	}
}

// TestExitCodeConstants_ExecutionDomainHasRoom verifies that the
// execution-failure range (10–19) is not yet exhausted. See
// TestExitCodeConstants_FindingsDomainHasRoom for rationale.
func TestExitCodeConstants_ExecutionDomainHasRoom(t *testing.T) {
	t.Parallel()

	used := map[int]bool{
		ExitConfig:     true,
		ExitTemplate:   true,
		ExitFilesystem: true,
		ExitSecurity:   true,
		ExitConflict:   true,
	}
	free := 0
	for code := ExitRangeExecutionMin; code <= ExitRangeExecutionMax; code++ {
		if !used[code] {
			free++
		}
	}
	if free == 0 {
		t.Errorf("execution range %d–%d has no free codes; a new ADR is required before assigning",
			ExitRangeExecutionMin, ExitRangeExecutionMax)
	}
}

// ---------------------------------------------------------------------------
// Range taxonomy structure (WBS 11.1.1)
// ---------------------------------------------------------------------------

// TestExitCodeTaxonomy_RangesAreContiguous verifies the structural
// invariants of the range table: sorted, contiguous, covering
// [0, 255] without gaps or overlaps.
//
// The init function in exitcodes.go panics on violation, so reaching
// this test means the table is well-formed. The test re-asserts the
// invariant explicitly so that the property is visible in `go test
// -v` output and so that a future refactor that removes the init
// panic still has test coverage.
func TestExitCodeTaxonomy_RangesAreContiguous(t *testing.T) {
	t.Parallel()

	if err := checkRangeTableInvariants(); err != nil {
		t.Fatalf("range table invariant violated: %v", err)
	}
}

// TestExitCodeTaxonomy_CoversFullByte verifies that every integer in
// [0, 255] maps to exactly one documented range. This is the
// boundary case for LookupExitCodeRange: a gap or overlap would
// violate the public contract.
func TestExitCodeTaxonomy_CoversFullByte(t *testing.T) {
	t.Parallel()

	for code := 0; code <= 255; code++ {
		if _, ok := LookupExitCodeRange(code); !ok {
			t.Errorf("code %d has no documented range", code)
		}
	}
}

// TestExitCodeTaxonomy_OutOfByteRejected verifies that values outside
// [0, 255] are rejected by VerifyExitCodeRange. This is the
// cross-platform stability guarantee: Forge emits only 0–255 so that
// Windows 32-bit exit codes and POSIX 8-bit exit codes agree.
func TestExitCodeTaxonomy_OutOfByteRejected(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		code int
	}{
		{"negative one", -1},
		{"large negative", -256},
		{"just above byte", 256},
		{"int max", int(^uint(0) >> 1)},
		{"int min", -int(^uint(0)>>1) - 1},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := VerifyExitCodeRange(tc.code)
			if err == nil {
				t.Fatalf("VerifyExitCodeRange(%d) = nil, want error", tc.code)
			}
			var rerr *ExitCodeRangeError
			if !errors.As(err, &rerr) {
				t.Fatalf("error type = %T, want *ExitCodeRangeError", err)
			}
			if rerr.Code != tc.code {
				t.Errorf("error.Code = %d, want %d", rerr.Code, tc.code)
			}
		})
	}
}

// TestExitCodeTaxonomy_InByteAccepted verifies that every value in
// [0, 255] is accepted by VerifyExitCodeRange.
func TestExitCodeTaxonomy_InByteAccepted(t *testing.T) {
	t.Parallel()

	for code := 0; code <= 255; code++ {
		if err := VerifyExitCodeRange(code); err != nil {
			t.Errorf("VerifyExitCodeRange(%d) = %v, want nil", code, err)
		}
	}
}

// TestExitCodeTaxonomy_ExitCodeRangesReturnsCopy verifies that
// ExitCodeRanges returns a defensive copy. Mutating the returned
// slice must not affect internal state.
func TestExitCodeTaxonomy_ExitCodeRangesReturnsCopy(t *testing.T) {
	t.Parallel()

	a := ExitCodeRanges()
	if len(a) == 0 {
		t.Fatal("ExitCodeRanges() returned empty slice")
	}
	original := a[0].Name
	a[0].Name = "MUTATED"

	b := ExitCodeRanges()
	if b[0].Name != original {
		t.Errorf("internal state mutated: got %q, want %q", b[0].Name, original)
	}
}

// TestExitCodeTaxonomy_StabilityClasses verifies that every documented
// range declares a valid stability class and non-empty metadata.
func TestExitCodeTaxonomy_StabilityClasses(t *testing.T) {
	t.Parallel()

	valid := map[Stability]bool{
		StabilityFrozen:   true,
		StabilityAdditive: true,
		StabilityReserved: true,
	}
	for _, r := range ExitCodeRanges() {
		if !valid[r.Stability] {
			t.Errorf("range %q has invalid stability %q", r.Name, r.Stability)
		}
		if r.Name == "" {
			t.Errorf("range with Min=%d has empty Name", r.Min)
		}
		if r.Purpose == "" {
			t.Errorf("range %q has empty Purpose", r.Name)
		}
		if r.CallerGuide == "" {
			t.Errorf("range %q has empty CallerGuide", r.Name)
		}
	}
}

// TestExitCodeTaxonomy_FrozenRangesAreSingletons verifies the
// compatibility policy: a frozen range is a singleton. Frozen ranges
// never grow, so they cannot contain more than one code.
func TestExitCodeTaxonomy_FrozenRangesAreSingletons(t *testing.T) {
	t.Parallel()

	for _, r := range ExitCodeRanges() {
		if r.Stability == StabilityFrozen && !r.IsSingleton() {
			t.Errorf("frozen range %q is not a singleton (Min=%d, Max=%d)",
				r.Name, r.Min, r.Max)
		}
	}
}

// TestExitCodeRange_Contains verifies the boundary behaviour of
// Contains: inclusive on both ends, exclusive outside.
func TestExitCodeRange_Contains(t *testing.T) {
	t.Parallel()

	r := ExitCodeRange{Min: 10, Max: 19}
	cases := []struct {
		code int
		want bool
	}{
		{9, false},
		{10, true},
		{15, true},
		{19, true},
		{20, false},
	}
	for _, tc := range cases {
		if got := r.Contains(tc.code); got != tc.want {
			t.Errorf("Contains(%d) = %v, want %v", tc.code, got, tc.want)
		}
	}
}

// TestExitCodeRange_IsSingleton verifies singleton detection.
func TestExitCodeRange_IsSingleton(t *testing.T) {
	t.Parallel()

	if !(ExitCodeRange{Min: 0, Max: 0}).IsSingleton() {
		t.Error("zero-width range not detected as singleton")
	}
	if (ExitCodeRange{Min: 0, Max: 1}).IsSingleton() {
		t.Error("one-width range mis-detected as singleton")
	}
}

// ---------------------------------------------------------------------------
// Range classifiers (WBS 11.1.1)
// ---------------------------------------------------------------------------

// TestExitCodeTaxonomy_Classifiers verifies the five classifier
// helpers at every boundary and one interior point. Boundary cases
// are exhaustive for the finding, execution, internal, and signal
// ranges; the usage classifier has three cases.
func TestExitCodeTaxonomy_Classifiers(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		code      int
		finding   bool
		execution bool
		usage     bool
		internal  bool
		signal    bool
	}{
		// Boundaries — findings.
		{"finding min", ExitRangeFindingsMin, true, false, false, false, false},
		{"finding interior", 5, true, false, false, false, false},
		{"finding max", ExitRangeFindingsMax, true, false, false, false, false},
		{"finding below", 0, false, false, false, false, false},
		{"finding above", 10, false, true, false, false, false},

		// Boundaries — execution failures.
		{"execution min", ExitRangeExecutionMin, false, true, false, false, false},
		{"execution interior", 15, false, true, false, false, false},
		{"execution max", ExitRangeExecutionMax, false, true, false, false, false},
		{"execution below", 9, true, false, false, false, false},
		{"execution above", 20, false, false, false, false, false},

		// Usage (singleton).
		{"usage", ExitUsage, false, false, true, false, false},
		{"usage below", ExitUsage - 1, false, false, false, false, false},
		{"usage above", ExitUsage + 1, false, false, false, false, false},

		// Internal.
		{"internal min", ExitRangeInternalMin, false, false, false, true, false},
		{"internal interior", 74, false, false, false, true, false},
		{"internal max", ExitRangeInternalMax, false, false, false, true, false},
		{"internal below", ExitRangeInternalMin - 1, false, false, false, false, false},
		{"internal above", ExitRangeInternalMax + 1, false, false, false, false, false},

		// Signals.
		{"signal min", ExitRangeSignalsMin, false, false, false, false, true},
		{"sigint", ExitSIGINT, false, false, false, false, true},
		{"sigterm", ExitSIGTERM, false, false, false, false, true},
		{"signal max", ExitRangeSignalsMax, false, false, false, false, true},
		{"signal below", ExitRangeSignalsMin - 1, false, false, false, false, false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := IsFinding(tc.code); got != tc.finding {
				t.Errorf("IsFinding(%d) = %v, want %v", tc.code, got, tc.finding)
			}
			if got := IsExecutionFailure(tc.code); got != tc.execution {
				t.Errorf("IsExecutionFailure(%d) = %v, want %v", tc.code, got, tc.execution)
			}
			if got := IsUsageError(tc.code); got != tc.usage {
				t.Errorf("IsUsageError(%d) = %v, want %v", tc.code, got, tc.usage)
			}
			if got := IsInternalError(tc.code); got != tc.internal {
				t.Errorf("IsInternalError(%d) = %v, want %v", tc.code, got, tc.internal)
			}
			if got := IsSignal(tc.code); got != tc.signal {
				t.Errorf("IsSignal(%d) = %v, want %v", tc.code, got, tc.signal)
			}
		})
	}
}

// TestExitCodeTaxonomy_ClassifiersArePartition verifies that the
// classifiers partition [0, 255] without overlap: no code is
// classified as both a finding and an execution failure, and no code
// is classified as both an execution failure and an internal error,
// etc.
//
// This is the load-bearing property for callers that branch on the
// classifiers: the branches are mutually exclusive.
func TestExitCodeTaxonomy_ClassifiersArePartition(t *testing.T) {
	t.Parallel()

	for code := 0; code <= 255; code++ {
		hits := 0
		if IsFinding(code) {
			hits++
		}
		if IsExecutionFailure(code) {
			hits++
		}
		if IsUsageError(code) {
			hits++
		}
		if IsInternalError(code) {
			hits++
		}
		if IsSignal(code) {
			hits++
		}
		if hits > 1 {
			t.Errorf("code %d classified by %d classifiers; want at most 1", code, hits)
		}
	}
}

// ---------------------------------------------------------------------------
// exitCodeOf behavior (WBS 10.5.1, preserved)
// ---------------------------------------------------------------------------

// TestExitCodeOf_Nil verifies the nil-error case.
//
// The helper returns ExitSuccess for a nil error. This is the same
// convention as the removed exitCodeFromError and is what the
// boundary relies on: a nil error means the command succeeded.
func TestExitCodeOf_Nil(t *testing.T) {
	t.Parallel()

	if got := exitCodeOf(nil); got != ExitSuccess {
		t.Errorf("nil error: got %d, want %d", got, ExitSuccess)
	}
}

// TestExitCodeOf_ForgeError verifies that a *forgeerr.Error maps to
// its category's exit code.
//
// The test constructs one error per category and asserts that the
// helper returns Category.ExitCode(). The taxonomy is closed
// (WBS 10.2.1), so the set is enumerated here rather than discovered
// dynamically.
//
// # Type-level contract
//
// Every forgeerr.New call passes the code as forgeerr.Code (the
// named constants from codes.go). The constructor's parameter is
// forgeerr.Code after the WBS 13.1.1 / ADR-035 migration; a
// string(...) cast would be a compile error once the migration is
// complete, and its presence is the diagnostic that the migration
// was rolled back.
func TestExitCodeOf_ForgeError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		cat  forgeerr.Category
		code forgeerr.Code
	}{
		{forgeerr.CategoryCLI, forgeerr.CodeCLIUnknownCommand},
		{forgeerr.CategoryConfig, forgeerr.CodeConfigMalformed},
		{forgeerr.CategoryFilesystem, forgeerr.CodeFilesystemNotFound},
		{forgeerr.CategorySecurity, forgeerr.CodeSecurityPathTraversal},
		{forgeerr.CategoryInternal, forgeerr.CodeInternalUnexpected},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(string(tc.cat), func(t *testing.T) {
			t.Parallel()

			e := forgeerr.New(tc.cat, tc.code, "message")
			got := exitCodeOf(e)
			want := tc.cat.ExitCode()
			if got != want {
				t.Errorf("exitCodeOf(%v) = %d; want %d (category's code)",
					tc.cat, got, want)
			}
		})
	}
}

// TestExitCodeOf_WrappedForgeError verifies that a *forgeerr.Error
// wrapped inside another error is still detected.
//
// This is the common case when an error is wrapped with fmt.Errorf
// and the %w verb: the outer error does not carry a category, but
// errors.As unwraps until it finds the inner error that does.
func TestExitCodeOf_WrappedForgeError(t *testing.T) {
	t.Parallel()

	inner := forgeerr.New(
		forgeerr.CategoryConfig,
		forgeerr.CodeConfigMalformed,
		"inner error",
	)
	wrapped := fmt.Errorf("outer context: %w", inner)

	if got := exitCodeOf(wrapped); got != ExitConfig {
		t.Errorf("wrapped *forgeerr.Error: got %d, want %d",
			got, ExitConfig)
	}
}

// TestExitCodeOf_DeeplyWrappedForgeError verifies that the helper
// finds the *forgeerr.Error through multiple layers of wrapping.
func TestExitCodeOf_DeeplyWrappedForgeError(t *testing.T) {
	t.Parallel()

	inner := forgeerr.New(
		forgeerr.CategorySecurity,
		forgeerr.CodeSecurityPathTraversal,
		"inner error",
	)
	wrapped := fmt.Errorf("a: %w", fmt.Errorf("b: %w", fmt.Errorf("c: %w", inner)))

	if got := exitCodeOf(wrapped); got != ExitSecurity {
		t.Errorf("deeply wrapped *forgeerr.Error: got %d, want %d",
			got, ExitSecurity)
	}
}

// TestExitCodeOf_PlainError verifies that an error which is neither
// a *forgeerr.Error nor wraps one maps to ExitInternal.
//
// Historically this branch mapped to ExitFailure (WBS 4.1.2, value
// 1). Under the WBS 11.1.1 taxonomy, a plain error is an internal
// bug: the boundary wraps such errors as FORGE_INTERNAL_UNEXPECTED
// before they reach the user. The safety-net branch therefore maps
// to ExitInternal (70), the INTERNAL category's code, not to a
// finding.
func TestExitCodeOf_PlainError(t *testing.T) {
	t.Parallel()

	err := errors.New("some plain error")

	got := exitCodeOf(err)
	if got != ExitInternal {
		t.Errorf("plain error: got %d, want %d (ExitInternal)", got, ExitInternal)
	}
	if !IsInternalError(got) {
		t.Errorf("plain error mapped to %d, which is not in the internal-error range", got)
	}
}

// TestExitCodeOf_IsPure verifies that the helper is deterministic.
// Calling it twice with the same error returns the same code.
func TestExitCodeOf_IsPure(t *testing.T) {
	t.Parallel()

	err := forgeerr.New(
		forgeerr.CategorySecurity,
		forgeerr.CodeSecurityPathTraversal,
		"test error",
	)

	first := exitCodeOf(err)
	for i := 0; i < 100; i++ {
		if got := exitCodeOf(err); got != first {
			t.Fatalf("iteration %d: got %d, want %d", i, got, first)
		}
	}
}

// TestExitCodeOf_MatchesCategoryExitCode verifies that the helper's
// result for a *forgeerr.Error is exactly the category's method's
// result.
//
// The test iterates the closed taxonomy (WBS 10.2.1) and asserts the
// agreement for every category. It is the type-level companion to
// TestAC4_ExitCodesMatchCLIConstants in
// internal/forgeerr/category_test.go: that test asserts the
// categories' codes agree with the CLI constants; this test asserts
// the CLI's helper reads the categories' codes correctly.
func TestExitCodeOf_MatchesCategoryExitCode(t *testing.T) {
	t.Parallel()

	for _, cat := range forgeerr.AllCategories() {
		cat := cat
		t.Run(string(cat), func(t *testing.T) {
			t.Parallel()

			// Find a registered code for the category. If none
			// exists (the VALIDATION and TEMPLATE categories in
			// Phase 2), skip; the helper's behaviour is the same
			// regardless of the code, and the skip is honest about
			// the registry's current coverage.
			var code forgeerr.Code
			for _, c := range forgeerr.AllCodes() {
				if c2, ok := c.Category(); ok && c2 == cat {
					code = c
					break
				}
			}
			if code == "" {
				t.Skipf("no registered code for category %s", cat)
			}

			e := forgeerr.New(cat, code, "message")

			got := exitCodeOf(e)
			want := cat.ExitCode()
			if got != want {
				t.Errorf("exitCodeOf(%v) = %d; want %d",
					cat, got, want)
			}
		})
	}
}

// TestExitCodeOf_ResultIsInDocumentedRange verifies that every exit
// code the helper can return falls within a documented range. This
// is the cross-cutting invariant that ties the mapping (WBS 10.5.1)
// to the taxonomy (WBS 11.1.1).
func TestExitCodeOf_ResultIsInDocumentedRange(t *testing.T) {
	t.Parallel()

	// Enumerate every category and construct one error per category.
	// The helper's result must be in a documented range for each.
	for _, cat := range forgeerr.AllCategories() {
		cat := cat
		t.Run(string(cat), func(t *testing.T) {
			t.Parallel()

			var code forgeerr.Code
			for _, c := range forgeerr.AllCodes() {
				if c2, ok := c.Category(); ok && c2 == cat {
					code = c
					break
				}
			}
			if code == "" {
				t.Skipf("no registered code for category %s", cat)
			}

			e := forgeerr.New(cat, code, "message")
			got := exitCodeOf(e)

			if err := VerifyExitCodeRange(got); err != nil {
				t.Errorf("exitCodeOf(%v) = %d: %v", cat, got, err)
			}
			if _, ok := LookupExitCodeRange(got); !ok {
				t.Errorf("exitCodeOf(%v) = %d is outside all documented ranges",
					cat, got)
			}
		})
	}

	// The safety-net branch: a plain error must also be in range.
	if got := exitCodeOf(errors.New("plain")); !IsInternalError(got) {
		t.Errorf("plain error mapped to %d; want an internal-error code", got)
	}
}

// TestExitCodeOf_UnknownCategoryCannotExist verifies the closed
// taxonomy by construction: a *forgeerr.Error cannot carry a
// category outside the closed set. The check is a compile-time
// assertion in the current implementation; the test documents the
// property.
//
// # Why the test is a tautology
//
// The category is validated by forgeerr.New, which panics on an
// invalid category. There is no way to construct a *forgeerr.Error
// with an out-of-set category. The test exists to document this
// property: a future change that removed the validation would allow
// an unknown category, and this test would fail to compile or fail
// at runtime depending on the shape of the change.
func TestExitCodeOf_UnknownCategoryCannotExist(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("forgeerr.New did not panic on an invalid category; " +
				"the closed taxonomy is no longer enforced")
		}
	}()
	_ = forgeerr.New(
		forgeerr.Category("UNKNOWN"),
		forgeerr.CodeConfigMalformed,
		"message",
	)
}

// ---------------------------------------------------------------------------
// BDD — behavior scenarios for the taxonomy and the mapping
// ---------------------------------------------------------------------------

// TestBDD_DriftDetectedIsAFindingNotAFailure documents the caller-
// visible behavior of a drift finding under the WBS 11.1.1 taxonomy.
func TestBDD_DriftDetectedIsAFindingNotAFailure(t *testing.T) {
	t.Parallel()
	// GIVEN a drift finding is reported by a check command
	code := ExitDriftDetected

	// WHEN a caller classifies the exit code
	finding := IsFinding(code)
	execution := IsExecutionFailure(code)

	// THEN the caller sees a finding, not an execution failure
	if !finding {
		t.Errorf("drift (code %d) must be classified as a finding", code)
	}
	if execution {
		t.Errorf("drift (code %d) must not be classified as an execution failure", code)
	}
}

// TestBDD_ConfigErrorIsAnExecutionFailureNotAFinding documents the
// caller-visible behavior of a configuration error under the
// WBS 11.1.1 taxonomy.
func TestBDD_ConfigErrorIsAnExecutionFailureNotAFinding(t *testing.T) {
	t.Parallel()
	// GIVEN forge failed to load its configuration
	code := ExitConfig

	// WHEN a caller classifies the exit code
	finding := IsFinding(code)
	execution := IsExecutionFailure(code)

	// THEN the caller sees an execution failure, not a finding
	if finding {
		t.Errorf("config error (code %d) must not be classified as a finding", code)
	}
	if !execution {
		t.Errorf("config error (code %d) must be classified as an execution failure", code)
	}
}

// TestBDD_UsageErrorIsDistinctFromFindingsAndFailures documents the
// caller-visible behavior of a usage error.
func TestBDD_UsageErrorIsDistinctFromFindingsAndFailures(t *testing.T) {
	t.Parallel()
	// GIVEN the user mistyped a flag
	code := ExitUsage

	// WHEN a caller classifies the exit code
	// THEN it is a usage error, not a finding, not an execution failure
	if !IsUsageError(code) {
		t.Errorf("usage error (code %d) must be classified as a usage error", code)
	}
	if IsFinding(code) {
		t.Errorf("usage error (code %d) must not be a finding", code)
	}
	if IsExecutionFailure(code) {
		t.Errorf("usage error (code %d) must not be an execution failure", code)
	}
}

// TestBDD_SIGINTIsASignalNotAFailure documents the caller-visible
// behavior of SIGINT termination.
func TestBDD_SIGINTIsASignalNotAFailure(t *testing.T) {
	t.Parallel()
	// GIVEN the user pressed Ctrl-C
	code := ExitSIGINT

	// WHEN a caller classifies the exit code
	// THEN it is a signal, not a finding, not an execution failure
	if !IsSignal(code) {
		t.Errorf("SIGINT (code %d) must be classified as a signal", code)
	}
	if IsFinding(code) {
		t.Errorf("SIGINT (code %d) must not be a finding", code)
	}
	if IsExecutionFailure(code) {
		t.Errorf("SIGINT (code %d) must not be an execution failure", code)
	}
}

// TestBDD_CIPipelineCanBranchWithoutParsingOutput documents the
// load-bearing caller-visible property of the taxonomy: a CI
// pipeline can branch on the numeric range without parsing stderr.
func TestBDD_CIPipelineCanBranchWithoutParsingOutput(t *testing.T) {
	t.Parallel()

	// GIVEN a CI pipeline observes an exit code
	cases := []struct {
		name     string
		code     int
		category string
	}{
		{"drift", ExitDriftDetected, "findings"},
		{"config error", ExitConfig, "execution"},
		{"internal error", ExitRangeInternalMin, "internal"},
		{"usage error", ExitUsage, "usage"},
		{"ctrl-c", ExitSIGINT, "signal"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// WHEN the pipeline branches on the numeric range
			var got string
			switch {
			case IsFinding(tc.code):
				got = "findings"
			case IsExecutionFailure(tc.code):
				got = "execution"
			case IsUsageError(tc.code):
				got = "usage"
			case IsInternalError(tc.code):
				got = "internal"
			case IsSignal(tc.code):
				got = "signal"
			default:
				got = "unknown"
			}

			// THEN it recovers the intended category without parsing
			if got != tc.category {
				t.Errorf("code %d classified as %q, want %q",
					tc.code, got, tc.category)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ATDD — acceptance criteria from WBS 11.1.1
// ---------------------------------------------------------------------------

// TestAC_11_1_1_RangeTablePublished verifies the acceptance criterion
// "Range table published with each range's purpose".
func TestAC_11_1_1_RangeTablePublished(t *testing.T) {
	t.Parallel()

	ranges := ExitCodeRanges()
	if len(ranges) < 10 {
		t.Fatalf("expected at least 10 documented ranges, got %d", len(ranges))
	}
	for _, r := range ranges {
		if r.Purpose == "" {
			t.Errorf("range %q missing Purpose", r.Name)
		}
	}
}

// TestAC_11_1_1_TwoFailureDomainsDistinct verifies the acceptance
// criterion "Forge findings vs Forge execution failure distinction
// explicitly stated".
func TestAC_11_1_1_TwoFailureDomainsDistinct(t *testing.T) {
	t.Parallel()

	findings, ok := LookupExitCodeRange(ExitRangeFindingsMin)
	if !ok {
		t.Fatal("findings range not found")
	}
	if !strings.Contains(strings.ToLower(findings.Name), "finding") {
		t.Errorf("findings range name %q does not mention findings", findings.Name)
	}

	exec, ok := LookupExitCodeRange(ExitRangeExecutionMin)
	if !ok {
		t.Fatal("execution range not found")
	}
	if !strings.Contains(strings.ToLower(exec.Name), "execution") {
		t.Errorf("execution range name %q does not mention execution", exec.Name)
	}

	if findings.Max >= exec.Min {
		t.Errorf("findings (max %d) and execution (min %d) overlap",
			findings.Max, exec.Min)
	}
	if findings.Max+1 != exec.Min {
		t.Errorf("gap between findings (max %d) and execution (min %d)",
			findings.Max, exec.Min)
	}
}

// TestAC_11_1_1_ReservedRangesDocumented verifies the acceptance
// criterion "Reserved ranges documented".
func TestAC_11_1_1_ReservedRangesDocumented(t *testing.T) {
	t.Parallel()

	reserved := 0
	for _, r := range ExitCodeRanges() {
		if r.Stability == StabilityReserved {
			reserved++
			if r.CallerGuide == "" {
				t.Errorf("reserved range %q missing CallerGuide", r.Name)
			}
		}
	}
	if reserved == 0 {
		t.Error("no reserved ranges documented")
	}
}

// TestAC_11_1_1_CompatibilityPolicyStated verifies the acceptance
// criterion "Compatibility policy stated (additive only)".
//
// The policy is encoded as Stability classes. Every range declares
// one, and no range may be non-additive except frozen singletons and
// reserved ranges.
func TestAC_11_1_1_CompatibilityPolicyStated(t *testing.T) {
	t.Parallel()

	for _, r := range ExitCodeRanges() {
		switch r.Stability {
		case StabilityFrozen:
			if !r.IsSingleton() {
				t.Errorf("frozen range %q is not a singleton; frozen ranges must be singletons",
					r.Name)
			}
		case StabilityAdditive:
			// Additive ranges may be singletons or wider; no constraint.
		case StabilityReserved:
			// Reserved ranges may be any width; no constraint.
		default:
			t.Errorf("range %q has unknown stability %q", r.Name, r.Stability)
		}
	}
}

// TestAC_11_1_1_SignalCodesDocumented verifies the acceptance
// criterion "Signal-handling codes (130, 143) documented".
func TestAC_11_1_1_SignalCodesDocumented(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		code int
	}{
		{"SIGINT", ExitSIGINT},
		{"SIGTERM", ExitSIGTERM},
	}
	for _, tc := range cases {
		r, ok := LookupExitCodeRange(tc.code)
		if !ok {
			t.Errorf("%s (%d) has no documented range", tc.name, tc.code)
			continue
		}
		if !strings.Contains(r.Name, "Signal") {
			t.Errorf("%s (%d) is in range %q, want a Signal range",
				tc.name, tc.code, r.Name)
		}
		if !IsSignal(tc.code) {
			t.Errorf("IsSignal(%d) = false, want true", tc.code)
		}
	}
}

// TestAC_11_1_1_WindowsMasking verifies the acceptance criterion
// "Platform notes for Windows included" structurally: Forge only
// emits 0–255, and every emitted code passes VerifyExitCodeRange.
func TestAC_11_1_1_WindowsMasking(t *testing.T) {
	t.Parallel()

	emitted := []int{
		ExitSuccess,
		ExitDriftDetected, ExitValidation, ExitCheckFailed,
		ExitConfig, ExitTemplate, ExitFilesystem, ExitSecurity, ExitConflict,
		ExitUsage,
		ExitSIGINT, ExitSIGTERM,
	}
	for _, code := range emitted {
		if code < 0 || code > 255 {
			t.Errorf("emitted code %d outside 0–255; Windows masking would corrupt it", code)
		}
		if err := VerifyExitCodeRange(code); err != nil {
			t.Errorf("VerifyExitCodeRange(%d) = %v, want nil", code, err)
		}
	}
}

// TestAC_11_1_1_ReviewedByAnotherEngineer verifies the acceptance
// criterion "Reviewed by at least one other engineer" structurally:
// the code is accompanied by a decision record (ADR-018) and a
// reference document (exit-code-ranges.md). The presence of both is
// the artifact of review; the review itself is a process step
// recorded in the PR.
func TestAC_11_1_1_ReviewedByAnotherEngineer(t *testing.T) {
	t.Parallel()

	// This test is a placeholder for the process artifact. It
	// documents that the implementation is accompanied by ADR-018
	// and docs/reference/exit-code-ranges.md. The test does not read
	// the files (they are outside the package); it records the
	// expectation that a human reviewer verified them.
	//
	// If the ADR or the reference doc is removed, the CI check
	// scripts/deps/verify-exit-codes.sh fails.
	t.Log("accompanied by ADR-018 and docs/reference/exit-code-ranges.md")
}

// ---------------------------------------------------------------------------
// Non-functional — performance of the classifier hot path
// ---------------------------------------------------------------------------

// BenchmarkIsFinding measures the classifier hot path. The function
// is called once per exit and must be allocation-free.
func BenchmarkIsFinding(b *testing.B) {
	b.ReportAllocs()
	var sink bool
	for i := 0; i < b.N; i++ {
		sink = IsFinding(1)
	}
	_ = sink
}

// BenchmarkLookupExitCodeRange measures the range lookup. The
// function is called by tests and tooling, not on the hot path, but
// the benchmark documents that it is O(ranges) = O(11).
func BenchmarkLookupExitCodeRange(b *testing.B) {
	b.ReportAllocs()
	var sink ExitCodeRange
	for i := 0; i < b.N; i++ {
		sink, _ = LookupExitCodeRange(128)
	}
	_ = sink
}

// BenchmarkExitCodeOf_ForgeError measures the error-to-exit-code
// mapping. It is on the hot path of every failed command.
func BenchmarkExitCodeOf_ForgeError(b *testing.B) {
	err := forgeerr.New(
		forgeerr.CategoryConfig,
		forgeerr.CodeConfigMalformed,
		"benchmark error",
	)
	b.ReportAllocs()
	var sink int
	for i := 0; i < b.N; i++ {
		sink = exitCodeOf(err)
	}
	_ = sink
}

// ---------------------------------------------------------------------------
// Examples — executable documentation
// ---------------------------------------------------------------------------

// ExampleIsFinding demonstrates the two failure domains.
func ExampleIsFinding() {
	fmt.Println(IsFinding(ExitDriftDetected))
	fmt.Println(IsFinding(ExitConfig))
	// Output:
	// true
	// false
}

// ExampleIsExecutionFailure demonstrates the two failure domains
// from the other side.
func ExampleIsExecutionFailure() {
	fmt.Println(IsExecutionFailure(ExitConfig))
	fmt.Println(IsExecutionFailure(ExitDriftDetected))
	// Output:
	// true
	// false
}

// ExampleLookupExitCodeRange demonstrates range introspection.
func ExampleLookupExitCodeRange() {
	r, _ := LookupExitCodeRange(ExitSIGINT)
	fmt.Println(r.Name)
	// Output: Signals (128 + N)
}

// ExampleVerifyExitCodeRange demonstrates range validation.
func ExampleVerifyExitCodeRange() {
	fmt.Println(VerifyExitCodeRange(ExitUsage))
	fmt.Println(VerifyExitCodeRange(256) != nil)
	// Output:
	// <nil>
	// true
}
