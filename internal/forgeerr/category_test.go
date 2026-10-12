// Package forgeerr — error category taxonomy tests.
//
// Every test is named after the acceptance criterion (AC1–AC6) it
// verifies. AC7 (documentation) and AC8 (ADR policy) are not
// testable in Go and are enforced by code review; they are noted
// here so the traceability matrix is complete.
//
// Test categories:
//
//   - ATDD:      AC-mapped tests
//   - Positive:  valid members of the closed set
//   - Negative:  invalid categories (zero value, unknown strings)
//   - Edge:      String() on invalid input, Prefix/ExitCode panics
//   - Boundary:  first and last members of the enumeration order
//   - Non-func:  AllCategories returns a copy, no allocation
//   - Fuzz:      valid() is a pure membership test
//
// Tests that construct a *Error use codes from the registry in
// codes.go (WBS 10.3.1). Tests that call Category methods
// directly do not consult the registry and use only the category
// constants.
package forgeerr

import (
	"fmt"
	"strings"
	"testing"
)

// --- AC1: all 7 categories defined -----------------------------------

// TestAC1_AllCategoriesDefined asserts that the closed set has
// exactly seven members and that every declared constant is a
// member of allCategories. A constant that is declared but not
// listed in allCategories is a bug: validateCategory would reject
// it, and the CLI would fall through to the INTERNAL exit code.
func TestAC1_AllCategoriesDefined(t *testing.T) {
	t.Parallel()

	declared := []Category{
		CategoryCLI,
		CategoryConfig,
		CategoryFilesystem,
		CategoryValidation,
		CategoryTemplate,
		CategorySecurity,
		CategoryInternal,
	}

	if got, want := len(declared), 7; got != want {
		t.Fatalf("declared constants = %d, want %d", got, want)
	}

	fromAPI := AllCategories()
	if got, want := len(fromAPI), len(declared); got != want {
		t.Fatalf("AllCategories() = %d members, want %d", got, want)
	}

	for _, c := range declared {
		if !c.valid() {
			t.Errorf("declared category %q is not in allCategories", c)
		}
	}
}

// TestAC1_EnumerationOrderIsStable pins the order of
// AllCategories. The order is part of the contract: documentation
// generators and golden files depend on it.
func TestAC1_EnumerationOrderIsStable(t *testing.T) {
	t.Parallel()

	want := []Category{
		CategoryCLI,
		CategoryConfig,
		CategoryFilesystem,
		CategoryValidation,
		CategoryTemplate,
		CategorySecurity,
		CategoryInternal,
	}
	got := AllCategories()

	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("AllCategories()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// --- AC2: each category has a fixed exit code -------------------------

// TestAC2_ExitCodes pins the exit code for every category against
// the frozen values from WBS 4.1.2. If a category is added without
// an entry in ExitCode, this test fails via the panic in the
// default branch.
func TestAC2_ExitCodes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		c    Category
		want int
	}{
		{CategoryCLI, 2},
		{CategoryConfig, 3},
		{CategoryFilesystem, 4},
		{CategoryValidation, 5},
		{CategoryTemplate, 1},
		{CategorySecurity, 6},
		{CategoryInternal, 1},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.c.String(), func(t *testing.T) {
			t.Parallel()
			if got := tc.c.ExitCode(); got != tc.want {
				t.Fatalf("ExitCode() = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestAC2_ExitCodePanicsOnInvalid asserts that ExitCode rejects a
// non-member. This is the constructor-side discipline: an invalid
// category at the process boundary is a programming error.
func TestAC2_ExitCodePanicsOnInvalid(t *testing.T) {
	t.Parallel()

	cases := []Category{"", "UNKNOWN", "cli", "CLI "}
	for _, c := range cases {
		c := c
		t.Run(fmt.Sprintf("%q", c), func(t *testing.T) {
			t.Parallel()
			defer func() {
				if r := recover(); r == nil {
					t.Fatalf("ExitCode(%q) did not panic", c)
				}
			}()
			_ = c.ExitCode()
		})
	}
}

// --- AC3: each category has a fixed prefix ----------------------------

// TestAC3_Prefixes pins the Code prefix for every category against
// the frozen values from WBS 10.2.1.
func TestAC3_Prefixes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		c    Category
		want string
	}{
		{CategoryCLI, "FORGE_CLI_"},
		{CategoryConfig, "FORGE_CONFIG_"},
		{CategoryFilesystem, "FORGE_FILESYSTEM_"},
		{CategoryValidation, "FORGE_VALIDATION_"},
		{CategoryTemplate, "FORGE_TEMPLATE_"},
		{CategorySecurity, "FORGE_SECURITY_"},
		{CategoryInternal, "FORGE_INTERNAL_"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.c.String(), func(t *testing.T) {
			t.Parallel()
			if got := tc.c.Prefix(); got != tc.want {
				t.Fatalf("Prefix() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestAC3_PrefixPanicsOnInvalid asserts that Prefix rejects a
// non-member.
func TestAC3_PrefixPanicsOnInvalid(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Prefix() did not panic on invalid category")
		}
	}()
	_ = Category("UNKNOWN").Prefix()
}

// TestAC3_PrefixesAreUnique asserts that no two categories share a
// prefix. If they did, a Code could satisfy two categories, and
// the "category derivable from prefix" rule would break.
func TestAC3_PrefixesAreUnique(t *testing.T) {
	t.Parallel()

	seen := make(map[string]Category, len(allCategories))
	for _, c := range allCategories {
		p := c.Prefix()
		if other, dup := seen[p]; dup {
			t.Fatalf("prefix %q shared by %s and %s", p, other, c)
		}
		seen[p] = c
	}
}

// TestAC3_PrefixSatisfiesCodePattern asserts that a Code formed by
// appending a valid tail to a prefix satisfies the Code regex
// (codePattern). This is the bridge between the taxonomy and the
// constructor's invariant I3: if a prefix did not satisfy the
// pattern, no Code could ever carry it.
func TestAC3_PrefixSatisfiesCodePattern(t *testing.T) {
	t.Parallel()

	for _, c := range allCategories {
		c := c
		t.Run(c.String(), func(t *testing.T) {
			t.Parallel()
			// A minimal Code: prefix + "A". E.g. "FORGE_CLI_A".
			// The regex is satisfied by shape, not by registry
			// membership; the registry check is a separate
			// concern tested in codes_test.go and
			// constructors_test.go.
			code := c.Prefix() + "A"
			if !codePattern.MatchString(code) {
				t.Fatalf("code %q (prefix %q + A) does not match codePattern",
					code, c.Prefix())
			}
		})
	}
}

// --- AC4: exit codes match WBS 4.1.2 constants ------------------------
//
// The constants ExitUsage, ExitConfig, ExitFilesystem,
// ExitValidation, ExitFailure, and ExitSecurity are frozen in
// WBS 4.1.2 and defined in internal/cli/exitcodes.go. This test
// asserts that Category.ExitCode returns the same numeric values
// as those constants.
//
// The constants are duplicated here on purpose: the test asserts
// that Category.ExitCode returns the same values. If WBS 4.1.2
// changes, this test fails and the reviewer updates both the
// constants and the switch in Category.ExitCode.

func TestAC4_ExitCodesMatchCLIConstants(t *testing.T) {
	t.Parallel()

	const (
		wantUsage      = 2
		wantConfig     = 3
		wantFilesystem = 4
		wantValidation = 5
		wantFailure    = 1
		wantSecurity   = 6
	)

	cases := []struct {
		c    Category
		want int
		note string
	}{
		{CategoryCLI, wantUsage, "ExitUsage"},
		{CategoryConfig, wantConfig, "ExitConfig"},
		{CategoryFilesystem, wantFilesystem, "ExitFilesystem"},
		{CategoryValidation, wantValidation, "ExitValidation"},
		{CategoryTemplate, wantFailure, "ExitFailure"},
		{CategorySecurity, wantSecurity, "ExitSecurity"},
		{CategoryInternal, wantFailure, "ExitFailure"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.c.String(), func(t *testing.T) {
			t.Parallel()
			if got := tc.c.ExitCode(); got != tc.want {
				t.Fatalf("ExitCode() = %d, want %d (%s)", got, tc.want, tc.note)
			}
		})
	}
}

// --- AC5: AllCategories() returns all 7 -------------------------------

func TestAC5_AllCategoriesReturnsSeven(t *testing.T) {
	t.Parallel()

	if got := len(AllCategories()); got != 7 {
		t.Fatalf("AllCategories() has %d members, want 7", got)
	}
}

// TestAC5_AllCategoriesReturnsACopy asserts that mutating the slice
// returned by AllCategories does not affect the package's
// enumeration. This is the same discipline as Context(): the API
// never hands out a reference to internal state.
func TestAC5_AllCategoriesReturnsACopy(t *testing.T) {
	t.Parallel()

	first := AllCategories()
	if len(first) == 0 {
		t.Fatal("AllCategories returned an empty slice")
	}
	first[0] = Category("MUTATED")

	second := AllCategories()
	if second[0] == Category("MUTATED") {
		t.Fatal("AllCategories returned an aliased slice")
	}
}

// --- AC6: category validation rejects unknown values ------------------

// TestAC6_ValidAcceptsClosedSet asserts that every member of the
// closed set is accepted by valid().
func TestAC6_ValidAcceptsClosedSet(t *testing.T) {
	t.Parallel()

	for _, c := range AllCategories() {
		if !c.valid() {
			t.Errorf("valid() = false for %q", c)
		}
	}
}

// TestAC6_ValidRejectsUnknown asserts that valid() rejects values
// that look like categories but are not members. The list covers
// the interesting failure modes: empty, wrong case, trailing
// whitespace, a typo, and a value that is a valid *prefix* but not
// a category.
func TestAC6_ValidRejectsUnknown(t *testing.T) {
	t.Parallel()

	invalid := []Category{
		"",           // zero value
		"cli",        // wrong case
		"CLI ",       // trailing space
		" CLI",       // leading space
		"CONFIGX",    // typo
		"FORGE_CLI_", // a prefix, not a category
		"UNKNOWN",    // the canonical "not a category" value
		"OTHER",      // explicitly forbidden by WBS 10.2.1
		"INTERNALX",  // prefix of CategoryInternal but not equal
		"CONFIG\n",   // trailing newline
		"\x00",       // NUL byte
		"CONFIG\x00", // embedded NUL
	}
	for _, c := range invalid {
		c := c
		t.Run(fmt.Sprintf("%q", c), func(t *testing.T) {
			t.Parallel()
			if c.valid() {
				t.Fatalf("valid() = true for %q", c)
			}
		})
	}
}

// TestAC6_ValidateCategoryPanicsOnUnknown asserts that the
// constructor-side helper validateCategory panics on every invalid
// value.
func TestAC6_ValidateCategoryPanicsOnUnknown(t *testing.T) {
	t.Parallel()

	for _, c := range []Category{"", "cli", "UNKNOWN", "OTHER"} {
		c := c
		t.Run(fmt.Sprintf("%q", c), func(t *testing.T) {
			t.Parallel()
			defer func() {
				r := recover()
				if r == nil {
					t.Fatalf("validateCategory(%q) did not panic", c)
				}
				if !strings.Contains(fmt.Sprint(r), "closed set") {
					t.Fatalf("panic %q does not mention the closed set", r)
				}
			}()
			validateCategory(c)
		})
	}
}

// --- String() ---------------------------------------------------------

// TestString_ValidCategories asserts that String returns the
// canonical name for every member. The canonical name is the
// underlying string value; the method exists so that String is
// total (never panics) and so that %v formats categories correctly.
func TestString_ValidCategories(t *testing.T) {
	t.Parallel()

	cases := []struct {
		c    Category
		want string
	}{
		{CategoryCLI, "CLI"},
		{CategoryConfig, "CONFIG"},
		{CategoryFilesystem, "FILESYSTEM"},
		{CategoryValidation, "VALIDATION"},
		{CategoryTemplate, "TEMPLATE"},
		{CategorySecurity, "SECURITY"},
		{CategoryInternal, "INTERNAL"},
	}
	for _, tc := range cases {
		if got := tc.c.String(); got != tc.want {
			t.Errorf("String() = %q, want %q", got, tc.want)
		}
	}
}

// TestString_InvalidReturnsUnknown asserts that String does not
// panic on an invalid category. Diagnostics may call String on a
// zero-value Category; a panic there would mask the real error.
func TestString_InvalidReturnsUnknown(t *testing.T) {
	t.Parallel()

	for _, c := range []Category{"", "cli", "UNKNOWN"} {
		if got := c.String(); got != "unknown" {
			t.Errorf("String() = %q for %q, want %q", got, c, "unknown")
		}
	}
}

// --- BDD: end-to-end category selection -------------------------------

// TestBDD_CategoryDrivesExitCode walks through the scenario a
// caller uses to map an error to a process exit code. It is the
// user-observable acceptance test for the taxonomy: given an
// error, the caller reads its Category and derives the exit code.
//
// The error is constructed with a registered code from the
// registry in codes.go.
func TestBDD_CategoryDrivesExitCode(t *testing.T) {
	t.Parallel()

	// Given: an error constructed with a known category and a
	// registered code.
	e := New(CategoryConfig, string(CodeConfigMalformed), "invalid YAML in config.yaml")

	// When: the caller maps the error to an exit code.
	code := e.Category().ExitCode()

	// Then: the exit code matches the frozen value for CONFIG.
	if code != 3 {
		t.Fatalf("ExitCode() = %d, want 3", code)
	}
}

// TestBDD_SecurityCategoryIsObservable asserts that the SECURITY
// category can be reached through the constructor API and carries
// its dedicated exit code. This guards against accidentally
// collapsing SECURITY into FILESYSTEM or INTERNAL during a
// refactor.
//
// The code is a member of the registry (WBS 10.3.1); the previous
// placeholder FORGE_SECURITY_TRAVERSAL is not.
func TestBDD_SecurityCategoryIsObservable(t *testing.T) {
	t.Parallel()

	e := New(
		CategorySecurity,
		string(CodeSecurityPathTraversal),
		"path escapes the project root",
	)

	if got := e.Code(); got != string(CodeSecurityPathTraversal) {
		t.Fatalf("Code() = %q, want %q", got, CodeSecurityPathTraversal)
	}
	if got := e.Category(); got != CategorySecurity {
		t.Fatalf("Category() = %v, want %v", got, CategorySecurity)
	}
	if got := e.Category().ExitCode(); got != 6 {
		t.Fatalf("ExitCode() = %d, want 6", got)
	}
	if got := e.Category().Prefix(); got != "FORGE_SECURITY_" {
		t.Fatalf("Prefix() = %q", got)
	}
}

// TestBDD_CategoryMatchesCodePrefix walks the taxonomy and asserts
// that a registered code exists for every category that is not
// explicitly pending, and that the code's prefix matches the
// category's prefix. This is the end-to-end version of the prefix
// rule: it exercises the constructor, the registry, and the
// taxonomy together.
func TestBDD_CategoryMatchesCodePrefix(t *testing.T) {
	t.Parallel()

	pending := make(map[Category]bool, len(pendingCategories))
	for _, c := range pendingCategories {
		pending[c] = true
	}

	for _, cat := range AllCategories() {
		cat := cat
		t.Run(cat.String(), func(t *testing.T) {
			t.Parallel()

			// Find a registered code in this category, if any.
			var code Code
			for _, k := range AllCodes() {
				if c, ok := k.Category(); ok && c == cat {
					code = k
					break
				}
			}

			if code == "" {
				if pending[cat] {
					t.Skipf("category %s is pending; no code to check", cat)
				}
				t.Fatalf("category %s has no registered code and is not pending", cat)
			}

			if !strings.HasPrefix(string(code), cat.Prefix()) {
				t.Fatalf("code %q does not begin with %q", code, cat.Prefix())
			}

			e := New(cat, string(code), "x")
			if e.Category() != cat {
				t.Fatalf("Category() = %v, want %v", e.Category(), cat)
			}
			if e.Code() != string(code) {
				t.Fatalf("Code() = %q, want %q", e.Code(), code)
			}
		})
	}
}

// --- Non-functional: allocation discipline ----------------------------

// BenchmarkAllCategories measures the cost of the defensive copy
// that AllCategories performs. It is O(len(allCategories)) and is
// expected to be called rarely (documentation generation, tests).
func BenchmarkAllCategories(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = AllCategories()
	}
}

// BenchmarkCategoryValid measures the cost of the membership scan.
// Seven comparisons; no allocation. This is on the constructor's
// hot path, so it should remain cheap even if the set grows a
// little.
func BenchmarkCategoryValid(b *testing.B) {
	c := CategoryConfig
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.valid()
	}
}

// --- Fuzz: membership is a pure predicate -----------------------------

// FuzzCategoryValid asserts that valid() is a pure predicate: it
// must agree exactly with membership in AllCategories. The fuzzer
// constructs candidate strings by prepending, appending, and
// mutating known-valid values, so it explores the boundary of the
// set rather than wandering the entire string space.
func FuzzCategoryValid(f *testing.F) {
	for _, c := range allCategories {
		f.Add(string(c))
	}
	f.Add("")
	f.Add("cli")
	f.Add("CLI ")
	f.Add(" CLI")
	f.Add("UNKNOWN")
	f.Add("OTHER")
	f.Add("CLI\x00")
	f.Add("\n")

	f.Fuzz(func(t *testing.T, s string) {
		c := Category(s)
		wantValid := false
		for _, k := range allCategories {
			if c == k {
				wantValid = true
				break
			}
		}
		if got := c.valid(); got != wantValid {
			t.Fatalf("valid(%q) = %v, want %v", s, got, wantValid)
		}
	})
}
