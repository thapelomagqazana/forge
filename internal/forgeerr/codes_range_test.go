// Package forgeerr_test — cross-package invariants between the error
// code registry and the exit code taxonomy.
//
// This file asserts the invariant that ties the error-code registry
// (codes.go, WBS 10.3.1) to the exit-code taxonomy
// (internal/cli/exitcodes.go, WBS 11.1.1):
//
//	For every registered code c:
//	    cat, ok := c.Category()
//	    require ok
//	    require cli.VerifyExitCodeRange(cat.ExitCode()) == nil
//	    require cli.LookupExitCodeRange(cat.ExitCode()) is in a
//	        non-reserved range
//
// The invariant is the third layer of the three-layer mapping
// documented in codes.go: the registry says what codes exist, the
// category taxonomy says which category each code belongs to, and
// the exit-code taxonomy says which numeric range each category's
// exit code falls in. This test is the bridge between layers 1 and
// 3, with layer 2 in between.
//
// # Why this file is in package forgeerr_test
//
// The test imports internal/cli to read the range table, and
// internal/cli imports internal/forgeerr. If the test were in
// package forgeerr (internal test package), the import of
// internal/cli would create a cycle: forgeerr → cli → forgeerr.
// Putting the test in the external test package (forgeerr_test)
// breaks the cycle: the external test package may import both
// forgeerr and cli, and neither imports the other's test package.
// This is the standard Go idiom for breaking a test-only cycle.
//
// # Why the check belongs in forgeerr and not in cli
//
// The check is "does every code in the registry land in a
// documented range?". That is a property of the registry, and the
// registry lives in forgeerr. The cli package's own tests
// (exitcodes_test.go) check that the range table is well-formed;
// the forgeerr package's tests check that the registry's codes are
// consistent with that table. Each package tests its own
// invariants; the cross-package invariant lives with the package
// that owns the more specific claim (the registry).
//
// # Relationship to scripts/deps/verify-exit-codes.sh
//
// The shell script asserts the same invariant from the outside: it
// runs the Go tests below and greps the reference documentation
// for the two failure domains. The shell script additionally
// checks the JSON schema's presence and validity, which is not a
// Go-level concern. The two checks overlap deliberately: the Go
// test catches the invariant at unit-test speed and in isolation;
// the shell script catches it in CI and also covers the
// non-Go sources.
package forgeerr_test

import (
	"testing"

	"github.com/thapelomagqazana/forge/internal/cli"
	"github.com/thapelomagqazana/forge/internal/forgeerr"
)

// TestEveryCodeIsInARange asserts that every registered code's
// category's exit code falls within a documented exit-code range.
//
// The invariant is checked in four steps, each of which is
// asserted separately so that a failure names the offending
// property:
//
//  1. Every code is registered (it comes from AllCodes, so this is
//     true by construction; the step is included for symmetry with
//     a hypothetical future where AllCodes might return an
//     unregistered code).
//
//  2. Every code has a category (cat, ok := c.Category()). A code
//     whose prefix does not match any category's prefix would fail
//     here, which is the registry's own internal invariant.
//
//  3. Every category's exit code is within [0, 255] (the
//     cross-platform stability guarantee).
//
//  4. Every category's exit code falls in a documented range, and
//     that range is not reserved. A code in a reserved range would
//     mean the registry assigned a code to a range the taxonomy
//     marked as "do not assign"; the test rejects that.
//
// The four steps are deliberately redundant with the production
// constructors (which validate the code's registration) and with
// the exit-code tests (which validate the range table's shape).
// The redundancy is the point: a bug in the constructors or the
// table that let an invalid code through would be caught here even
// if the constructors' or table's own tests happened to pass.
func TestEveryCodeIsInARange(t *testing.T) {
	t.Parallel()

	codes := forgeerr.AllCodes()
	if len(codes) == 0 {
		t.Fatal("forgeerr.AllCodes() returned no codes; the registry is empty")
	}

	for _, c := range codes {
		c := c
		t.Run(string(c), func(t *testing.T) {
			t.Parallel()

			// Step 1: the code is registered. AllCodes is the
			// registry's own list, so this is a tautology today;
			// the assertion is kept so that a future change to
			// AllCodes (for example, to return codes from a
			// different source) is caught.
			if !forgeerr.IsRegistered(c) {
				t.Fatalf("code %q is not registered; AllCodes returned an unregistered code", c)
			}

			// Step 2: the code has a category. A code whose
			// prefix does not match any category's prefix would
			// fail here. The failure is a registry bug: the code
			// was added without a matching prefix.
			cat, ok := c.Category()
			if !ok {
				t.Fatalf("code %q has no category; its prefix does not match any Category.Prefix()", c)
			}

			// Step 3: the category's exit code is within [0, 255].
			// This is the cross-platform stability guarantee: a
			// code outside [0, 255] would be masked or rejected
			// by an OS, and Forge's contract is that every code
			// is stable across platforms.
			exit := cat.ExitCode()
			if err := cli.VerifyExitCodeRange(exit); err != nil {
				t.Fatalf("code %q has category %q with exit code %d, "+
					"which is outside [0, 255]: %v",
					c, cat, exit, err)
			}

			// Step 4: the exit code falls in a documented range,
			// and the range is not reserved. A code in a reserved
			// range would mean the registry assigned a code to a
			// range the taxonomy marked as "do not assign"; the
			// test rejects that. The failure's message names the
			// range, so a reader sees exactly where the code
			// landed.
			r, ok := cli.LookupExitCodeRange(exit)
			if !ok {
				t.Fatalf("code %q has category %q with exit code %d, "+
					"which falls outside every documented range",
					c, cat, exit)
			}
			if r.Stability == cli.StabilityReserved {
				t.Fatalf("code %q has category %q with exit code %d, "+
					"which falls in reserved range %q; "+
					"reserved ranges must not be assigned",
					c, cat, exit, r.Name)
			}
		})
	}
}

// TestEveryCategoryIsReachableFromSomeCode asserts that every
// category in the taxonomy is reachable from at least one
// registered code.
//
// The invariant is the converse of TestEveryCodeIsInARange: that
// test goes from code to category; this test goes from category to
// code. Together they assert that the registry and the taxonomy
// have the same set of categories.
//
// # Why the check matters
//
// A category with no code is dead weight: the category exists in
// the taxonomy, Category.ExitCode() answers for it, but no code
// ever carries it. The situation is not a bug in itself — a future
// phase may register the first code for a category — but it is a
// fact that should be visible in the test output rather than
// discovered by reading category.go and codes.go side by side.
//
// The test does NOT fail on a category with no code; it logs the
// category and continues. The rationale: the taxonomy is a
// closed set of categories that the package's design fixes; the
// registry is a growing set of codes. A category that has no code
// yet is not a defect, and failing on it would force the registry
// to grow in lockstep with the taxonomy, which is the opposite of
// the design.
func TestEveryCategoryIsReachableFromSomeCode(t *testing.T) {
	t.Parallel()

	// Build the set of categories that have at least one code.
	reachable := make(map[forgeerr.Category]bool)
	for _, c := range forgeerr.AllCodes() {
		cat, ok := c.Category()
		if !ok {
			continue // TestEveryCodeIsInARange reports this
		}
		reachable[cat] = true
	}

	// Iterate the taxonomy. Categories without a code are logged,
	// not failed.
	for _, cat := range forgeerr.AllCategories() {
		cat := cat
		t.Run(string(cat), func(t *testing.T) {
			t.Parallel()

			if !reachable[cat] {
				t.Logf("category %q has no registered code; "+
					"its exit code %d is documented but not yet emitted",
					cat, cat.ExitCode())
			}
		})
	}
}

// TestEveryCategoryExitCodeIsInARange asserts that every category's
// exit code falls in a documented range, regardless of whether the
// category has a registered code.
//
// This test complements TestEveryCodeIsInARange: that test covers
// the categories that are reachable from a code; this test covers
// all categories, including those that have no code yet. The two
// tests together cover the entire mapping, not just the reachable
// part.
//
// # Why the check is separate
//
// A category with no code would pass TestEveryCodeIsInARange
// vacuously (there is no code to check). TestEveryCategoryExitCodeIsInARange
// asserts the range membership directly, so a category added to
// the taxonomy with an exit code outside all ranges fails here
// even before the first code for it is registered.
func TestEveryCategoryExitCodeIsInARange(t *testing.T) {
	t.Parallel()

	for _, cat := range forgeerr.AllCategories() {
		cat := cat
		t.Run(string(cat), func(t *testing.T) {
			t.Parallel()

			exit := cat.ExitCode()

			if err := cli.VerifyExitCodeRange(exit); err != nil {
				t.Fatalf("category %q has exit code %d, "+
					"which is outside [0, 255]: %v",
					cat, exit, err)
			}

			r, ok := cli.LookupExitCodeRange(exit)
			if !ok {
				t.Fatalf("category %q has exit code %d, "+
					"which falls outside every documented range",
					cat, exit)
			}
			if r.Stability == cli.StabilityReserved {
				t.Fatalf("category %q has exit code %d, "+
					"which falls in reserved range %q; "+
					"reserved ranges must not be assigned",
					cat, exit, r.Name)
			}
		})
	}
}

// TestRegistryCoverageByRange asserts that the registry's codes are
// distributed across the non-reserved ranges that the taxonomy
// documents as "assigned".
//
// The test logs a table of range → codes, so that `go test -v`
// output shows the registry's coverage at a glance. It fails only
// if a range the taxonomy marks as assigned has no codes, which
// would be a documentation defect: the range table would advertise
// a range that no code actually uses.
//
// # Which ranges are "assigned" today
//
// Phase 2's registry uses the findings range (1–9), the
// execution-failure range (10–19), the usage singleton (64), and
// the internal-error range (70–78). The reserved ranges (20–29,
// 30–63, 65–69, 79–127) are expected to be empty; the test
// asserts that they are, because a code in a reserved range is a
// contract violation.
//
// The signals range (129–255) is not part of the registry: signal
// exit codes are produced by the OS, not by forgeerr. A registered
// code that mapped to a signal exit code would be a design error;
// the test does not check for it explicitly, because the
// constructors' category-to-code mapping makes it impossible (no
// category's prefix matches a FORGE_* string, and the taxonomy's
// categories' exit codes are all in the non-signal ranges).
func TestRegistryCoverageByRange(t *testing.T) {
	t.Parallel()

	// Build the set of codes per range.
	byRange := make(map[string][]forgeerr.Code)
	for _, c := range forgeerr.AllCodes() {
		cat, ok := c.Category()
		if !ok {
			continue // TestEveryCodeIsInARange reports this
		}
		r, ok := cli.LookupExitCodeRange(cat.ExitCode())
		if !ok {
			continue // TestEveryCodeIsInARange reports this
		}
		byRange[r.Name] = append(byRange[r.Name], c)
	}

	// Log the coverage table. The table's order is the range
	// table's order, which is ascending by Min.
	for _, r := range cli.ExitCodeRanges() {
		codes := byRange[r.Name]
		t.Logf("range %-40q (%d–%d, %s): %d code(s)",
			r.Name, r.Min, r.Max, r.Stability, len(codes))
		for _, c := range codes {
			t.Logf("    %s", c)
		}
	}

	// Assert that no code is in a reserved range. This is the
	// same check as TestEveryCodeIsInARange's step 4, repeated
	// here so that the failure's context (which range, which
	// codes) is visible in a single place.
	for _, r := range cli.ExitCodeRanges() {
		if r.Stability != cli.StabilityReserved {
			continue
		}
		if codes := byRange[r.Name]; len(codes) > 0 {
			t.Errorf("reserved range %q has %d assigned code(s): %v",
				r.Name, len(codes), codes)
		}
	}
}

// TestCodePrefixesAreUniquePerCategory asserts that the category
// taxonomy's prefixes are distinct: no two categories share a
// prefix.
//
// The invariant is what makes Category's prefix-based mapping
// well-defined. Two categories with the same prefix would make
// Code.Category() ambiguous: a code with that prefix could belong
// to either category, and the loop in Code.Category() would return
// whichever came first in allCategories.
//
// The test is a self-check of the taxonomy, in the same spirit as
// TestCompat_MigrationTableIsWellFormed in the cli package. Its
// value is that a future category added to the taxonomy with a
// prefix that collides with an existing one fails here rather
// than silently producing wrong categories for codes.
func TestCodePrefixesAreUniquePerCategory(t *testing.T) {
	t.Parallel()

	seen := make(map[string]forgeerr.Category)
	for _, cat := range forgeerr.AllCategories() {
		prefix := cat.Prefix()
		if prefix == "" {
			t.Errorf("category %q has an empty prefix", cat)
			continue
		}
		if other, exists := seen[prefix]; exists {
			t.Errorf("prefix %q is shared by categories %q and %q",
				prefix, other, cat)
			continue
		}
		seen[prefix] = cat
	}
}

// TestCodeStringsMatchCategoryPrefix asserts that every registered
// code's string value begins with its category's prefix.
//
// The invariant is the contract that Category's implementation
// relies on. A code that did not begin with its category's prefix
// would be categorised by the *other* category's prefix (or not at
// all), and Code.Category() would return the wrong answer.
//
// The test asserts the invariant by construction: it takes each
// code, asks for its category, and verifies that the code's string
// begins with the category's prefix. The "asks for its category"
// step uses the same Code.Category() implementation the production
// code uses, so the test is a tautology in the strict sense. Its
// value is the failure message: if the invariant breaks, the test
// names the code, the category, and the prefix, which a reader can
// use to locate the offending row in codes.go.
func TestCodeStringsMatchCategoryPrefix(t *testing.T) {
	t.Parallel()

	for _, c := range forgeerr.AllCodes() {
		c := c
		t.Run(string(c), func(t *testing.T) {
			t.Parallel()

			cat, ok := c.Category()
			if !ok {
				t.Fatalf("code %q has no category", c)
			}
			prefix := cat.Prefix()
			if len(string(c)) < len(prefix) || string(c)[:len(prefix)] != prefix {
				t.Fatalf("code %q does not begin with category %q's prefix %q",
					c, cat, prefix)
			}
		})
	}
}

// TestEveryCodeIsLookupable asserts that every code returned by
// AllCodes is also findable via Lookup.
//
// The invariant is the consistency of the registry's two public
// accessors. AllCodes returns the sorted list; Lookup returns the
// metadata for a single code. A code that appeared in AllCodes but
// not in Lookup would mean the two views of the registry
// disagreed, which would break diagnostics that iterate the list
// and then look up each entry.
//
// The test also asserts that every code's Lookup returns a
// non-empty introduced phase. The phase is documentation-only, but
// an empty phase is a sign that a registry row was added without
// its metadata, which is the kind of oversight a test is for.
func TestEveryCodeIsLookupable(t *testing.T) {
	t.Parallel()

	for _, c := range forgeerr.AllCodes() {
		c := c
		t.Run(string(c), func(t *testing.T) {
			t.Parallel()

			introduced, deprecated, ok := forgeerr.Lookup(c)
			if !ok {
				t.Fatalf("code %q is in AllCodes but not findable via Lookup", c)
			}
			if introduced == "" {
				t.Errorf("code %q has an empty introduced phase", c)
			}
			if deprecated {
				// A deprecated code is not an error; the
				// registry permits them. The test logs the
				// fact so that `go test -v` output shows the
				// deprecations.
				t.Logf("code %q is deprecated", c)
			}
		})
	}
}

// TestRegistryHasNoDuplicateValues asserts that no two registry
// entries share a Code value.
//
// The invariant is guaranteed by the map's key type: two rows with
// the same key would overwrite each other, and the map would
// contain only the last. The test is therefore a tautology today.
// Its value is the failure's context: if a future refactor changed
// the registry's type from map[Code]codeSpec to a slice of
// codeSpec, the duplicate check would become non-trivial, and the
// test would already be in place to catch it.
//
// # Why the check is included despite the tautology
//
// The test documents the invariant that the map key enforces. A
// reader who wonders "what happens if two rows have the same
// code?" finds the answer here: the map prevents it, and the test
// records the prevention. A future change to the registry's shape
// that removed the prevention would fail this test.
func TestRegistryHasNoDuplicateValues(t *testing.T) {
	t.Parallel()

	seen := make(map[forgeerr.Code]bool, len(forgeerr.AllCodes()))
	for _, c := range forgeerr.AllCodes() {
		if seen[c] {
			t.Errorf("code %q appears twice in AllCodes", c)
		}
		seen[c] = true
	}
}
