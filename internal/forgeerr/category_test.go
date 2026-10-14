// Package forgeerr — error category taxonomy tests.
//
// Every test is named after the acceptance criterion (AC1–AC6) it
// verifies. AC7 (documentation) and AC8 (ADR policy) are not
// testable in Go and are enforced by code review; they are noted
// here so the traceability matrix is complete.
//
// # Type-level contract
//
// The constructors New and Wrap take their code parameter as
// forgeerr.Code. Every call site in this file passes the named
// constants from codes.go directly. A string(...) cast in a
// constructor call is the diagnostic that the constructor's
// parameter type has reverted to string and the migration is
// incomplete.
//
// The accessor (*Error).Code() returns string, so comparisons
// against a named constant cast the constant: e.Code() !=
// string(CodeConfigMalformed). The cast is on the known side
// (the constant), not on the value returned by the accessor.
//
// # Shared test helpers
//
// pendingCategories is defined in codes_test.go. This file
// references it; it does not define it. Two definitions in the
// same package would not compile. If a future refactor moves
// pendingCategories, this file's reference moves with it.
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
//
// # Exit codes and the WBS 11.1.1 taxonomy
//
// The exit codes asserted below are the WBS 11.1.1 values, not the
// WBS 4.1.2 flat values. The migration is documented in
// docs/decisions/ADR-018-exit-code-migration-and-stability-policy.md
// and in internal/cli/exitcodes.go; the numeric agreement between
// this package's Category.ExitCode() and the cli package's
// constants is policed by TestAC4_ExitCodesMatchCLIConstants below.
//
// The two non-zero domains named by the taxonomy are:
//
//   - Forge findings (1–9): the command ran; the subject is in a
//     detectable state. CategoryValidation lives here (code 2).
//   - Forge execution failures (10–19): the command could not run;
//     Forge itself hit a problem. CategoryConfig (10),
//     CategoryTemplate (11), CategoryFilesystem (12),
//     CategorySecurity (13), and CategoryConflict (14) live here.
//
// Two categories sit outside those ranges: CategoryCLI maps to the
// usage singleton (64, BSD EX_USAGE), and CategoryInternal maps to
// the internal-error range (70–78; the generic code is 70, BSD
// EX_SOFTWARE).
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
// the WBS 11.1.1 values. If a category is added without an entry
// in ExitCode, this test fails via the panic in the default branch.
//
// # The mapping and the ranges
//
//	CLI         → 64  (ExitUsage; usage singleton, BSD EX_USAGE)
//	CONFIG      → 10  (ExitConfig; execution failures)
//	FILESYSTEM  → 12  (ExitFilesystem; execution failures)
//	VALIDATION  → 2   (ExitValidation; findings)
//	TEMPLATE    → 11  (ExitTemplate; execution failures)
//	SECURITY    → 13  (ExitSecurity; execution failures)
//	INTERNAL    → 70  (ExitInternal; internal errors, BSD EX_SOFTWARE)
//
// The mapping is the same one asserted by
// TestAC4_ExitCodesMatchCLIConstants. The two tests are
// deliberately redundant: this one asserts the values directly,
// the other asserts them against the cli constants. A drift
// between the two surfaces in whichever fails first.
func TestAC2_ExitCodes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		c    Category
		want int
	}{
		{CategoryCLI, 64},
		{CategoryConfig, 10},
		{CategoryFilesystem, 12},
		{CategoryValidation, 2},
		{CategoryTemplate, 11},
		{CategorySecurity, 13},
		{CategoryInternal, 70},
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

// --- AC4: exit codes match the CLI constants --------------------------
//
// The constants ExitUsage, ExitConfig, ExitTemplate,
// ExitFilesystem, ExitSecurity, ExitValidation, and ExitInternal
// are frozen in WBS 11.1.1 and defined in
// internal/cli/exitcodes.go. This test asserts that
// Category.ExitCode returns the same numeric values as those
// constants.
//
// # Why the constants are duplicated in the test
//
// The test could import internal/cli and compare against the
// actual cli constants. It does not, because forgeerr is imported
// by cli: a non-test import of cli from forgeerr would create the
// cycle cli → forgeerr → cli. The test is in package forgeerr
// (internal test), so it cannot import cli either. The constants
// are therefore duplicated as local literals, and the test is the
// police for their agreement. If WBS 11.1.1 changes, this test
// fails, and the reviewer updates both the local literals and the
// switch in Category.ExitCode in the same commit.

func TestAC4_ExitCodesMatchCLIConstants(t *testing.T) {
	t.Parallel()

	const (
		wantUsage      = 64 // ExitUsage; usage singleton, BSD EX_USAGE
		wantConfig     = 10 // ExitConfig; execution failures
		wantTemplate   = 11 // ExitTemplate; execution failures
		wantFilesystem = 12 // ExitFilesystem; execution failures
		wantSecurity   = 13 // ExitSecurity; execution failures
		wantValidation = 2  // ExitValidation; findings
		wantInternal   = 70 // ExitInternal; internal errors, BSD EX_SOFTWARE
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
		{CategoryTemplate, wantTemplate, "ExitTemplate"},
		{CategorySecurity, wantSecurity, "ExitSecurity"},
		{CategoryInternal, wantInternal, "ExitInternal"},
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
//
// # The exit code
//
// The error's category is CategoryConfig, whose ExitCode() is
// ExitConfig (10) under the WBS 11.1.1 taxonomy. Before the
// taxonomy, the same category returned 3, in the flat range that
// no longer exists. The test asserts the current value; the
// migration is documented in ADR-018.
func TestBDD_CategoryDrivesExitCode(t *testing.T) {
	t.Parallel()

	// Given: an error constructed with a known category and a
	// registered code.
	e := New(CategoryConfig, CodeConfigMalformed, "invalid YAML in config.yaml")

	// When: the caller maps the error to an exit code.
	code := e.Category().ExitCode()

	// Then: the exit code matches the frozen value for CONFIG
	// in the execution-failure range.
	if code != 10 {
		t.Fatalf("ExitCode() = %d, want 10 (ExitConfig)", code)
	}
	if !(code >= 10 && code <= 19) {
		t.Fatalf("ExitCode() = %d is not in the execution-failure range (10–19)", code)
	}
}

// TestBDD_SecurityCategoryIsObservable asserts that the SECURITY
// category can be reached through the constructor API and carries
// its dedicated exit code. This guards against accidentally
// collapsing SECURITY into FILESYSTEM or INTERNAL during a
// refactor.
//
// The code is a member of the registry (WBS 10.3.1).
//
// # The exit code
//
// The category's ExitCode() is ExitSecurity (13) under the
// WBS 11.1.1 taxonomy, in the execution-failure range. Before the
// taxonomy, the same category returned 6.
func TestBDD_SecurityCategoryIsObservable(t *testing.T) {
	t.Parallel()

	e := New(
		CategorySecurity,
		CodeSecurityPathTraversal,
		"path escapes the project root",
	)

	if got := e.Code(); got != string(CodeSecurityPathTraversal) {
		t.Fatalf("Code() = %q, want %q", got, CodeSecurityPathTraversal)
	}
	if got := e.Category(); got != CategorySecurity {
		t.Fatalf("Category() = %v, want %v", got, CategorySecurity)
	}
	if got := e.Category().ExitCode(); got != 13 {
		t.Fatalf("ExitCode() = %d, want 13 (ExitSecurity)", got)
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

			e := New(cat, code, "x")
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

// TestAC4_DefaultExitCodesMatchExitCodes asserts that each
// category's DefaultExitCode equals its ExitCode. The two methods
// exist so that a mapping contract can refer to "the category's
// default" without implying a per-error override. In the current
// design the two are identical; the test pins the invariant so
// that a future change introducing a per-error override cannot
// silently change DefaultExitCode's meaning.
//
// # Why both methods exist
//
// ExitCode is the per-category mapping: given a category, which
// process exit code should the CLI use? DefaultExitCode is a
// naming accommodation: a future mapping contract may want to
// distinguish "the exit code this category always produces" from
// "the exit code this particular error would produce if the
// error had an override". Today the two are the same function;
// the pair of names exists so that the distinction can be drawn
// later without a rename.
func TestAC4_DefaultExitCodesMatchExitCodes(t *testing.T) {
	t.Parallel()

	for _, cat := range AllCategories() {
		cat := cat
		t.Run(string(cat), func(t *testing.T) {
			t.Parallel()

			def := cat.DefaultExitCode()
			eff := cat.ExitCode()
			if def != eff {
				t.Errorf("%v.DefaultExitCode() = %d; %v.ExitCode() = %d; must agree",
					cat, def, cat, eff)
			}
		})
	}
}
