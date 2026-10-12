// Package cli contains white-box tests for the internal/cli
// package.
//
// This file tests the exit code constants and the mapping from
// errors to exit codes. See exitcodes.go for the constants and
// execute.go (exitCodeOf) for the mapping.
//
// # The mapping's home
//
// The mapping is Category.ExitCode() in internal/forgeerr, not a
// switch in this package. The CLI's local helper exitCodeOf (in
// execute.go) extracts the *forgeerr.Error from the chain and
// reads the category's method. This file tests the helper's
// behaviour; the category's method is tested by
// internal/forgeerr/category_test.go.
//
// # Why the tests do not reference CategorizedError
//
// Before WBS 10.5.1, this package defined a CategorizedError
// interface and an exitCodeFromError function. Both were
// superseded: the interface by the closed forgeerr.Category type
// (WBS 10.2.1), and the function by the category's ExitCode
// method. The tests below assert the replacement's behaviour.
package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/thapelomagqazana/forge/internal/forgeerr"
)

// TestExitCodeConstants_HaveExpectedValues verifies that the exit
// code constants have the integer values declared in WBS 4.1.2.
//
// The test is deliberately brittle: it asserts on the exact values.
// If a value is changed, the test fails, and the change must be
// recorded in an ADR.
//
// # Relationship to the categories' exit codes
//
// The constants in this file and the categories'
// Category.ExitCode() methods must agree. The agreement is
// policed by TestAC4_ExitCodesMatchCLIConstants in
// internal/forgeerr/category_test.go. This test asserts the
// constants' values; the forgeerr test asserts their agreement
// with the categories.
func TestExitCodeConstants_HaveExpectedValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		constant int
		expected int
	}{
		{"ExitSuccess", ExitSuccess, 0},
		{"ExitFailure", ExitFailure, 1},
		{"ExitUsage", ExitUsage, 2},
		{"ExitConfig", ExitConfig, 3},
		{"ExitFilesystem", ExitFilesystem, 4},
		{"ExitValidation", ExitValidation, 5},
		{"ExitSecurity", ExitSecurity, 6},
		{"ExitConflict", ExitConflict, 7},
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

// TestExitCodeConstants_AreDistinct verifies that no two constants
// share a value. Duplicate values would make the mapping ambiguous.
func TestExitCodeConstants_AreDistinct(t *testing.T) {
	t.Parallel()

	constants := map[string]int{
		"ExitSuccess":    ExitSuccess,
		"ExitFailure":    ExitFailure,
		"ExitUsage":      ExitUsage,
		"ExitConfig":     ExitConfig,
		"ExitFilesystem": ExitFilesystem,
		"ExitValidation": ExitValidation,
		"ExitSecurity":   ExitSecurity,
		"ExitConflict":   ExitConflict,
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

// TestExitCodeOf_Nil verifies the nil-error case.
//
// The helper returns ExitSuccess for a nil error. This is the
// same convention as the removed exitCodeFromError and is what
// the boundary relies on: a nil error means the command
// succeeded.
func TestExitCodeOf_Nil(t *testing.T) {
	t.Parallel()

	if got := exitCodeOf(nil); got != ExitSuccess {
		t.Errorf("nil error: got %d, want %d", got, ExitSuccess)
	}
}

// TestExitCodeOf_ForgeError verifies that a *forgeerr.Error maps
// to its category's exit code.
//
// The test constructs one error per category and asserts that the
// helper returns Category.ExitCode(). The taxonomy is closed
// (WBS 10.2.1), so the set is enumerated here rather than
// discovered dynamically.
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

			e := forgeerr.New(tc.cat, string(tc.code), "message")
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
// This is the common case when an error is wrapped with
// fmt.Errorf and the %w verb: the outer error does not carry a
// category, but errors.As unwraps until it finds the inner error
// that does.
func TestExitCodeOf_WrappedForgeError(t *testing.T) {
	t.Parallel()

	inner := forgeerr.New(
		forgeerr.CategoryConfig,
		string(forgeerr.CodeConfigMalformed),
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
		string(forgeerr.CodeSecurityPathTraversal),
		"inner error",
	)
	wrapped := fmt.Errorf("a: %w", fmt.Errorf("b: %w", fmt.Errorf("c: %w", inner)))

	if got := exitCodeOf(wrapped); got != ExitSecurity {
		t.Errorf("deeply wrapped *forgeerr.Error: got %d, want %d",
			got, ExitSecurity)
	}
}

// TestExitCodeOf_PlainError verifies that an error which is
// neither a *forgeerr.Error nor wraps one maps to ExitFailure.
//
// This is the safety-net branch. The boundary (Execute) wraps
// such errors as FORGE_INTERNAL_UNEXPECTED before they reach the
// user, so the branch is reached only when executeWithOptions is
// called directly by a caller that constructs a non-*Error error.
// The branch's behaviour is ExitFailure (1), the INTERNAL
// category's code.
func TestExitCodeOf_PlainError(t *testing.T) {
	t.Parallel()

	err := errors.New("some plain error")

	if got := exitCodeOf(err); got != ExitFailure {
		t.Errorf("plain error: got %d, want %d", got, ExitFailure)
	}
}

// TestExitCodeOf_IsPure verifies that the helper is
// deterministic. Calling it twice with the same error returns the
// same code.
func TestExitCodeOf_IsPure(t *testing.T) {
	t.Parallel()

	err := forgeerr.New(
		forgeerr.CategorySecurity,
		string(forgeerr.CodeSecurityPathTraversal),
		"test error",
	)

	first := exitCodeOf(err)
	for i := 0; i < 100; i++ {
		if got := exitCodeOf(err); got != first {
			t.Fatalf("iteration %d: got %d, want %d", i, got, first)
		}
	}
}

// TestExitCodeOf_MatchesCategoryExitCode verifies that the
// helper's result for a *forgeerr.Error is exactly the category's
// method's result.
//
// The test iterates the closed taxonomy (WBS 10.2.1) and asserts
// the agreement for every category. It is the type-level
// companion to TestAC4_ExitCodesMatchCLIConstants in
// internal/forgeerr/category_test.go: that test asserts the
// categories' codes agree with the CLI constants; this test
// asserts the CLI's helper reads the categories' codes correctly.
func TestExitCodeOf_MatchesCategoryExitCode(t *testing.T) {
	t.Parallel()

	for _, cat := range forgeerr.AllCategories() {
		cat := cat
		t.Run(string(cat), func(t *testing.T) {
			t.Parallel()

			// Find a registered code for the category. If none
			// exists (the VALIDATION and TEMPLATE categories in
			// Phase 2), skip; the helper's behaviour is the same
			// regardless of the code, and the skip is honest
			// about the registry's current coverage.
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

			e := forgeerr.New(cat, string(code), "message")

			got := exitCodeOf(e)
			want := cat.ExitCode()
			if got != want {
				t.Errorf("exitCodeOf(%v) = %d; want %d",
					cat, got, want)
			}
		})
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
// property: a future change that removed the validation would
// allow an unknown category, and this test would fail to compile
// or fail at runtime depending on the shape of the change.
func TestExitCodeOf_UnknownCategoryCannotExist(t *testing.T) {
	t.Parallel()

	// The constructor panics on an invalid category. The test
	// asserts the panic to pin the property.
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("forgeerr.New did not panic on an invalid category; " +
				"the closed taxonomy is no longer enforced")
		}
	}()
	_ = forgeerr.New(
		forgeerr.Category("UNKNOWN"),
		string(forgeerr.CodeConfigMalformed),
		"message",
	)
}
