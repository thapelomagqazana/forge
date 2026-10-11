// Package cli contains white-box tests for the internal/cli
// package.
//
// This file tests the exit code constants and the mapping from
// errors to exit codes. See exitcodes.go for the implementation.
package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/thapelomagqazana/forge/internal/config"
)

// mockCategorizedError is a minimal implementation of the
// CategorizedError interface, used to exercise the mapping without
// depending on any specific error type from other packages.
type mockCategorizedError struct {
	msg      string
	category string
}

func (e *mockCategorizedError) Error() string    { return e.msg }
func (e *mockCategorizedError) Category() string { return e.category }

// TestExitCodeConstants_HaveExpectedValues verifies that the exit
// code constants have the integer values declared in the WBS.
//
// The test is deliberately brittle: it asserts on the exact values.
// If a value is changed, the test fails, and the change must be
// recorded in an ADR.
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

// TestExitCodeFromError_Nil verifies the nil-error case.
func TestExitCodeFromError_Nil(t *testing.T) {
	t.Parallel()

	if got := exitCodeFromError(nil); got != ExitSuccess {
		t.Errorf("nil error: got %d, want %d", got, ExitSuccess)
	}
}

// TestExitCodeFromError_Categorized verifies every category maps to
// the expected exit code.
func TestExitCodeFromError_Categorized(t *testing.T) {
	t.Parallel()

	tests := []struct {
		category string
		expected int
	}{
		{"config", ExitConfig},
		{"filesystem", ExitFilesystem},
		{"validation", ExitValidation},
		{"security", ExitSecurity},
		{"conflict", ExitConflict},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.category, func(t *testing.T) {
			t.Parallel()
			err := &mockCategorizedError{
				msg:      "test error",
				category: tc.category,
			}
			if got := exitCodeFromError(err); got != tc.expected {
				t.Errorf("category %q: got %d, want %d",
					tc.category, got, tc.expected)
			}
		})
	}
}

// TestExitCodeFromError_UnknownCategory verifies that a category the
// mapping does not recognise falls through to ExitFailure.
func TestExitCodeFromError_UnknownCategory(t *testing.T) {
	t.Parallel()

	err := &mockCategorizedError{
		msg:      "test error",
		category: "some-future-category",
	}

	if got := exitCodeFromError(err); got != ExitFailure {
		t.Errorf("unknown category: got %d, want %d", got, ExitFailure)
	}
}

// TestExitCodeFromError_Uncategorized verifies that an error which
// does not implement CategorizedError maps to ExitUsage.
//
// This is the case for Cobra's own errors: unknown command, unknown
// flag. They are, by definition, usage errors.
func TestExitCodeFromError_Uncategorized(t *testing.T) {
	t.Parallel()

	err := errors.New("some plain error")

	if got := exitCodeFromError(err); got != ExitUsage {
		t.Errorf("uncategorized error: got %d, want %d", got, ExitUsage)
	}
}

// TestExitCodeFromError_WrappedCategorized verifies that a
// CategorizedError wrapped inside another error is still detected.
//
// This is the common case when an error is wrapped with fmt.Errorf
// and the %w verb: the outer error does not implement
// CategorizedError, but errors.As unwraps until it finds the inner
// error that does.
func TestExitCodeFromError_WrappedCategorized(t *testing.T) {
	t.Parallel()

	inner := &mockCategorizedError{
		msg:      "inner error",
		category: "filesystem",
	}
	wrapped := fmt.Errorf("outer context: %w", inner)

	if got := exitCodeFromError(wrapped); got != ExitFilesystem {
		t.Errorf("wrapped categorized error: got %d, want %d",
			got, ExitFilesystem)
	}
}

// TestExitCodeFromError_Pure verifies that the mapping is
// deterministic. Calling it twice with the same error returns the
// same code.
func TestExitCodeFromError_Pure(t *testing.T) {
	t.Parallel()

	err := &mockCategorizedError{
		msg:      "test error",
		category: "security",
	}

	first := exitCodeFromError(err)
	for i := 0; i < 100; i++ {
		if got := exitCodeFromError(err); got != first {
			t.Fatalf("iteration %d: got %d, want %d", i, got, first)
		}
	}
}

// TestExitCodeFromError_AllCategoriesAreMapped verifies that every
// category the repository knows about is handled by the mapping.
//
// The list is maintained manually: when a new category is
// introduced by a future WBS, it must be added here and to the
// switch in exitCodeFromError.
func TestExitCodeFromError_AllCategoriesAreMapped(t *testing.T) {
	t.Parallel()

	knownCategories := []string{
		"config",
		"filesystem",
		"validation",
		"security",
		"conflict",
	}

	for _, category := range knownCategories {
		category := category
		t.Run(category, func(t *testing.T) {
			t.Parallel()
			err := &mockCategorizedError{
				msg:      "test",
				category: category,
			}
			if got := exitCodeFromError(err); got == ExitFailure {
				// ExitFailure is the default for unrecognised
				// categories. Reaching it for a known category
				// means the category is missing from the switch.
				t.Errorf("category %q maps to ExitFailure; "+
					"it should map to a specific exit code",
					category)
			}
		})
	}
}

// TestExitCodeFromError_ConfigErrorsClassifyAsExitConfig verifies
// that every error the config package produces is classified as
// ExitConfig.
//
// # Why this test exists alongside the mock-based tests
//
// The other tests in this file exercise the classifier against a
// mock CategorizedError. The mock's category string is chosen
// by the test; it always matches the classifier's switch. The
// tests therefore verify the classifier's *interface* contract
// but cannot catch a drift between the classifier's string
// literals and a real package's categories.
//
// This test closes the gap. It constructs every category of
// error the config package produces and asserts that the
// classifier maps each to ExitConfig (3). If someone changes
// the config package's Category() method to return a different
// string, this test fails; the contributor sees the failure and
// updates the classifier's switch in the same commit.
//
// # Why the test uses the config package's constructors
//
// The config package's error type is unexported. The test cannot
// construct a *configError directly. It uses the package's
// exported constructors (NewConfigNotFound, NewConfigPermission,
// and so on) to obtain an error whose Category() method the
// classifier reads. The constructors' signatures are documented
// in the config package; the test's use is exactly the way a
// caller (the loader) uses them.
//
// # Why the test lists every category explicitly
//
// The list is deliberately explicit. A future WBS that adds a
// ninth category to the config package's taxonomy must add a
// row here. The explicit list is the checkpoint that forces the
// contributor to consider the classifier's handling of the new
// category.
func TestExitCodeFromError_ConfigErrorsClassifyAsExitConfig(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
	}{
		{"E1 not found", config.NewConfigNotFound("/p/forge.yaml")},
		{"E2 permission", config.NewConfigPermission("/p/forge.yaml", nil)},
		{"E3 malformed", config.NewConfigMalformed("/p/forge.yaml", 5, 3, nil)},
		{"E4 unknown field", config.NewConfigUnknownField("/p/forge.yaml", "log_levl", nil)},
		{"E5 invalid type", config.NewConfigInvalidType("/p/forge.yaml", "log_level", nil)},
		{"E6 invalid value", config.NewConfigInvalid("log_level", "verbose", []string{"debug", "info"})},
		{"E7 too large", config.NewConfigTooLarge("/p/forge.yaml")},
		{"E8 invalid encoding", config.NewConfigInvalidEncoding("/p/forge.yaml")},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := exitCodeFromError(tc.err)
			if got != ExitConfig {
				t.Errorf("exitCodeFromError = %d; want %d (ExitConfig)\n"+
					"the config package's Category() may have changed; "+
					"check the switch in exitcodes.go",
					got, ExitConfig)
			}
		})
	}
}

// TestExitCodeFromError_ConfigErrorsCategoryMatches verifies that
// the config package's Category() method returns the string the
// classifier recognises.
//
// # Why this test is separate from the classification test
//
// The classification test asserts the end-to-end property: a
// config error maps to ExitConfig. This test asserts the
// intermediate property: the config package's category string
// is exactly "config". A failure here is more specific than a
// failure in the classification test; it tells the contributor
// that the string changed, not just that the classification
// broke.
//
// # How the test reads the category
//
// The config package's error type is unexported; the test cannot
// assert on the concrete type. It uses the CategorizedError
// interface (defined in this package) to read the error's
// category string. The interface assertion works because the
// config error implements Category() string.
func TestExitCodeFromError_ConfigErrorsCategoryMatches(t *testing.T) {
	t.Parallel()

	// One representative config error is sufficient; the
	// classification test above covers every category. This
	// test asserts the string the classifier compares against.
	err := config.NewConfigNotFound("/p/forge.yaml")

	var cat CategorizedError
	if !errors.As(err, &cat) {
		t.Fatalf("config error does not implement CategorizedError")
	}
	if got := cat.Category(); got != "config" {
		t.Errorf("config error's Category() = %q; want %q\n"+
			"the classifier's switch compares against the literal %q; "+
			"if the config package changed its category string, "+
			"update the switch in exitcodes.go and the string in the "+
			"config package's errors.go",
			got, "config", "config")
	}
}
