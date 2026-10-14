// Package forgeerr — error code registry tests.
//
// Every test is named after the acceptance criterion (AC1–AC8) it
// verifies. AC7 (codes are never renamed) and AC8 (deprecated
// codes are marked) are policies that cannot be tested in Go; they
// are documented in ADR-012 and enforced by code review. The tests
// here enforce everything that *can* be mechanised: shape,
// membership, prefix-category agreement, and documentation sync.
//
// # Shared test helpers
//
// The -update flag, the golden-file reader (goldenText), and the
// repoRoot walk-up helper are defined once per package:
//
//   - update and goldenText live in testhelpers_test.go (or, if
//     the package has no such file, in format_test.go). This file
//     consumes them; it does not define them. A duplicate
//     definition would fail to compile.
//   - repoRoot lives in format_test.go. The AC10 documentation
//     test uses it; this file does not need it.
//
// # Registry size
//
// The registry grows additively. WBS 13.1.1 added four codes to
// the FORGE_FILESYSTEM_ family so that internal/filesystem/safefs
// can report structural name problems, bounded-walk overruns, and
// case collisions through the taxonomy. The Phase 2 count moved
// from 19 to 23. The two "enumerate every code" tests below are
// the place that count is asserted; a future addition updates
// both tests and the golden file in the same commit.
//
// Test categories:
//
//   - ATDD:      AC-mapped tests
//   - Positive:  valid registry entries
//   - Negative:  unknown codes, malformed shapes
//   - Edge:      empty code, deprecated-code round-trip
//   - Boundary:  shortest and longest valid codes
//   - Non-func:  AllCodes returns a copy, sorted, allocation-free
//   - Sync:      codes.go, golden file, and reference doc agree
package forgeerr

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// --- AC1: registry has all Phase 2 codes ------------------------------

// TestAC1_RegistryHasAllPhase2Codes asserts that the registry
// contains exactly the 23 codes enumerated across WBS 10.3.1 and
// WBS 13.1.1. A mismatch in either direction — a missing code or
// an extra one — is a failure, because the registry is a frozen
// set.
//
// The expected list is written out by hand rather than derived
// from the registry, so that a change to codeSpecs that silently
// adds or removes an entry is caught here. This is the point of a
// frozen set: its contents are pinned, not inferred.
//
// The four FORGE_FILESYSTEM_* codes added in WBS 13.1.1 are:
//
//   - FORGE_FILESYSTEM_NOT_LOCAL         structural name rejection
//   - FORGE_FILESYSTEM_DEPTH_EXCEEDED    bounded walk overrun
//   - FORGE_FILESYSTEM_ENTRIES_EXCEEDED  bounded walk overrun
//   - FORGE_FILESYSTEM_CASE_COLLISION    case-insensitive collision
func TestAC1_RegistryHasAllPhase2Codes(t *testing.T) {
	t.Parallel()

	want := []Code{
		// CLI
		CodeCLIUnknownCommand,
		CodeCLIUnknownFlag,
		CodeCLIInvalidArg,
		CodeCLIUnknownFormat,

		// Config
		CodeConfigNotFound,
		CodeConfigPermission,
		CodeConfigMalformed,
		CodeConfigUnknownField,
		CodeConfigInvalidType,
		CodeConfigInvalidValue,
		CodeConfigTooLarge,
		CodeConfigInvalidEncoding,

		// Filesystem — Phase 2
		CodeFilesystemNotFound,
		CodeFilesystemPermission,
		CodeFilesystemEscape,

		// Filesystem — WBS 13.1.1
		CodeFilesystemNotLocal,
		CodeFilesystemDepthExceeded,
		CodeFilesystemEntriesExceeded,
		CodeFilesystemCaseCollision,

		// Filesystem — WBS 13.5.1
		CodeFilesystemExists,
		CodeFilesystemReadOnly,
		CodeFilesystemIsDirectory,
		CodeFilesystemNotDirectory,
		CodeFilesystemSymlinkLoop,
		CodeFilesystemReservedName,
		CodeFilesystemPathTooLong,

		// Security
		CodeSecurityPathTraversal,
		CodeSecuritySymlinkEscape,

		// Internal
		CodeInternalPanic,
		CodeInternalUnexpected,
	}

	got := AllCodes()

	if len(got) != len(want) {
		t.Fatalf("AllCodes() has %d entries, want %d", len(got), len(want))
	}

	// AllCodes is sorted; want is in declaration order. Sort a
	// copy of want for the comparison.
	wantSorted := append([]Code(nil), want...)
	sort.Slice(wantSorted, func(i, j int) bool { return wantSorted[i] < wantSorted[j] })

	for i := range wantSorted {
		if got[i] != wantSorted[i] {
			t.Fatalf("AllCodes()[%d] = %q, want %q", i, got[i], wantSorted[i])
		}
	}
}

// TestAC1_EveryConstantIsRegistered asserts that every declared
// constant is a member of the registry. A constant that is
// declared but not registered would be unusable with the
// constructors, which is a silent contract violation.
//
// The list below must match the list in
// TestAC1_RegistryHasAllPhase2Codes. The two are intentionally
// duplicated rather than shared: a single shared list would mean
// a single edit to keep both tests in sync, which is the failure
// mode the duplication exists to prevent. Two independent lists
// that must agree catch a mistaken edit that updates one and
// forgets the other.
func TestAC1_EveryConstantIsRegistered(t *testing.T) {
	t.Parallel()

	declared := []Code{
		// CLI
		CodeCLIUnknownCommand,
		CodeCLIUnknownFlag,
		CodeCLIInvalidArg,
		CodeCLIUnknownFormat,

		// Config
		CodeConfigNotFound,
		CodeConfigPermission,
		CodeConfigMalformed,
		CodeConfigUnknownField,
		CodeConfigInvalidType,
		CodeConfigInvalidValue,
		CodeConfigTooLarge,
		CodeConfigInvalidEncoding,

		// Filesystem
		CodeFilesystemNotFound,
		CodeFilesystemPermission,
		CodeFilesystemEscape,
		CodeFilesystemNotLocal,
		CodeFilesystemDepthExceeded,
		CodeFilesystemEntriesExceeded,
		CodeFilesystemCaseCollision,

		// Security
		CodeSecurityPathTraversal,
		CodeSecuritySymlinkEscape,

		// Internal
		CodeInternalPanic,
		CodeInternalUnexpected,
	}

	for _, c := range declared {
		if !IsRegistered(c) {
			t.Errorf("declared code %q is not in the registry", c)
		}
	}
}

// --- AC2: every code matches the regex --------------------------------

// codeShapeRegex mirrors codePattern from forgeerr.go but is
// defined here explicitly so that this test does not depend on the
// production regex. If the production regex changes without
// updating the registry, this test fails, which is the intended
// signal: the registry and the regex must agree.
var codeShapeRegex = regexp.MustCompile(`^FORGE_[A-Z][A-Z0-9_]*$`)

// TestAC2_EveryCodeMatchesRegex asserts that every registered code
// satisfies the shape regex. The regex is applied to the code's
// string value, not to the Code type, because the regex is the
// external contract.
func TestAC2_EveryCodeMatchesRegex(t *testing.T) {
	t.Parallel()

	for _, c := range AllCodes() {
		c := c
		t.Run(string(c), func(t *testing.T) {
			t.Parallel()
			if !codeShapeRegex.MatchString(string(c)) {
				t.Fatalf("code %q does not match %s", c, codeShapeRegex)
			}
		})
	}
}

// TestAC2_RegistryAgreesWithProductionRegex asserts that the
// test-side regex and the production codePattern are identical in
// behaviour on the registered codes. Divergence here would mean
// the registry and the constructors disagree on what a code looks
// like.
func TestAC2_RegistryAgreesWithProductionRegex(t *testing.T) {
	t.Parallel()

	for _, c := range AllCodes() {
		wantMatch := codeShapeRegex.MatchString(string(c))
		gotMatch := codePattern.MatchString(string(c))
		if gotMatch != wantMatch {
			t.Fatalf("code %q: codePattern=%v, codeShapeRegex=%v", c, gotMatch, wantMatch)
		}
	}
}

// --- AC3: every code's prefix matches a category ----------------------

// TestAC3_EveryCodeHasPrefixOfSomeCategory asserts that every
// registered code begins with exactly one category prefix. This is
// what makes the category derivable from the code.
func TestAC3_EveryCodeHasPrefixOfSomeCategory(t *testing.T) {
	t.Parallel()

	for _, c := range AllCodes() {
		c := c
		t.Run(string(c), func(t *testing.T) {
			t.Parallel()
			cat, ok := c.Category()
			if !ok {
				t.Fatalf("Category() = (_, false) for %q", c)
			}
			if cat == "" {
				t.Fatalf("Category() returned empty category for %q", c)
			}
			if !strings.HasPrefix(string(c), cat.Prefix()) {
				t.Fatalf("code %q does not begin with %q (category %s)",
					c, cat.Prefix(), cat)
			}
		})
	}
}

// TestAC3_CategoryUniqueness asserts that every code belongs to
// exactly one category. If a code began with two prefixes, the
// "category derivable from code" rule would be ambiguous.
func TestAC3_CategoryUniqueness(t *testing.T) {
	t.Parallel()

	for _, c := range AllCodes() {
		c := c
		t.Run(string(c), func(t *testing.T) {
			t.Parallel()
			matches := 0
			for _, cat := range AllCategories() {
				if strings.HasPrefix(string(c), cat.Prefix()) {
					matches++
				}
			}
			if matches != 1 {
				t.Fatalf("code %q matches %d category prefixes, want 1", c, matches)
			}
		})
	}
}

// pendingCategories enumerates the categories that are declared in
// the taxonomy but have no registered codes in the current phase.
//
// A category belongs here only when there is an explicit decision
// that its codes will arrive in a later phase. Adding a category
// to this list is a deliberate act: it means "this category
// exists but nothing emits it yet."
//
// Phase 2:
//   - VALIDATION: no validation subsystem yet.
//   - TEMPLATE:   templates are a Phase 4 concern.
//
// WBS 13.1.1 did not change this list. The four new codes are
// FORGE_FILESYSTEM_*, so the FILESYSTEM category was already
// covered by CodeFilesystemNotFound and remains covered.
//
// When the first code for a pending category is added, the entry
// must be removed from this list in the same commit; the second
// half of TestAC3_RegistryCoversEveryCategory enforces that.
var pendingCategories = []Category{
	CategoryValidation,
	CategoryTemplate,
}

// TestAC3_RegistryCoversEveryCategory asserts that every declared
// category is *either* covered by the registry *or* listed in
// pendingCategories. A category that is in neither is a signal
// that the taxonomy and the registry have drifted apart.
//
// The test also asserts the converse: a category that is in
// pendingCategories must *not* have any registered codes. That
// keeps the pending list honest as codes are added in later
// phases.
func TestAC3_RegistryCoversEveryCategory(t *testing.T) {
	t.Parallel()

	pending := make(map[Category]bool, len(pendingCategories))
	for _, c := range pendingCategories {
		pending[c] = true
	}

	counts := make(map[Category]int, len(allCategories))
	for _, c := range AllCodes() {
		cat, ok := c.Category()
		if !ok {
			t.Fatalf("code %q has no category", c)
		}
		counts[cat]++
	}

	for _, cat := range AllCategories() {
		switch {
		case counts[cat] > 0:
			// Covered. The positive assertion is exercised by the
			// constructor tests.
		case pending[cat]:
			// Explicitly pending. Acceptable for this phase.
		default:
			t.Errorf(
				"category %s has no registered codes and is not in pendingCategories; "+
					"add a code or add the category to pendingCategories",
				cat,
			)
		}
	}

	// A category that is both covered and pending is a bug: the
	// pending entry should have been removed when the first code
	// was added.
	for _, c := range pendingCategories {
		if counts[c] > 0 {
			t.Errorf(
				"category %s is in pendingCategories but has %d registered codes; "+
					"remove it from pendingCategories",
				c, counts[c],
			)
		}
	}
}

// TestAC3_PendingCategoriesAreValid asserts that every entry in
// pendingCategories is a member of the closed taxonomy. A typo in
// the pending list would silently mask a real gap.
func TestAC3_PendingCategoriesAreValid(t *testing.T) {
	t.Parallel()

	for _, c := range pendingCategories {
		if !c.valid() {
			t.Errorf("pendingCategories contains %q, which is not a valid category", c)
		}
	}
}

// --- AC4: documentation is generated from codes.go --------------------
//
// The documentation at docs/reference/error-codes.md is kept in
// sync by a golden file. The golden file is generated from the
// registry by renderCodesGolden, so a registry change that is not
// accompanied by a regenerated golden file fails CI.
//
// The workflow:
//
//  1. Edit codes.go.
//  2. Run `go test -run TestAC4 -update ./internal/forgeerr/`
//     to regenerate the golden file.
//  3. Copy the generated table into docs/reference/error-codes.md
//     (the file itself contains a comment with the command).
//  4. Commit all four files together: codes.go, codes_test.go,
//     testdata/codes/codes.golden.txt, docs/reference/error-codes.md.
//
// # Shared with the user-facing format goldens
//
// The -update flag and the golden-file reader (goldenText) are the
// same helpers used by format_test.go's golden tests. They are
// defined once in testhelpers_test.go (or, if the package has no
// such file, in format_test.go). This file consumes them; a
// duplicate definition would fail to compile.

// TestAC4_DocsSyncWithRegistry asserts that the golden file
// testdata/codes/codes.golden.txt matches the registry.
//
// The golden file is the machine-readable rendering of the
// registry that the docs generation pipeline consumes. If it
// drifts, CI fails; the fix is to regenerate it, not to edit it
// by hand.
//
// # Why this test uses goldenText and not a local reader
//
// A local reader (the previous version's readGolden) would have
// to duplicate the -update branch and the mkdir-or-fail logic
// that goldenText already implements. Two readers, one per golden
// family, drift the moment one of them is updated. A single
// reader guarantees that a change to the flag's semantics — for
// example, adding a -check mode — applies to every golden in the
// package.
func TestAC4_DocsSyncWithRegistry(t *testing.T) {
	t.Parallel()

	want := renderCodesGolden()
	path := filepath.Join("testdata", "codes", "codes.golden.txt")
	got := goldenText(t, path, want)

	if got != want {
		t.Fatalf(
			"testdata/codes/codes.golden.txt is out of date.\n"+
				"Run `go test -run TestAC4 -update ./internal/forgeerr/` to regenerate.\n"+
				"--- got ---\n%s\n--- want ---\n%s",
			got, want,
		)
	}
}

// --- Positive / Negative / Boundary: Lookup and IsRegistered ----------

// TestLookup_KnownCodes asserts that every registered code has
// non-empty metadata. As of Phase 2, no code is deprecated; when a
// code is deprecated in the future, this test should be updated to
// assert the specific deprecated code and leave the others as
// false.
func TestLookup_KnownCodes(t *testing.T) {
	t.Parallel()

	for _, c := range AllCodes() {
		introduced, deprecated, ok := Lookup(c)
		if !ok {
			t.Errorf("Lookup(%q) = (_, _, false)", c)
			continue
		}
		if introduced == "" {
			t.Errorf("Lookup(%q).introduced is empty", c)
		}
		if deprecated {
			t.Errorf("Lookup(%q).deprecated = true, want false", c)
		}
	}
}

// TestLookup_UnknownCodes asserts that Lookup and IsRegistered
// agree on values that look like codes but are not registered.
// Each vector is a plausible typo: a missing suffix, a truncated
// word, a trailing underscore.
//
// The vectors include near-misses for both the original codes and
// the four added in WBS 13.1.1. A typo of a new code is as
// plausible as a typo of an old one.
func TestLookup_UnknownCodes(t *testing.T) {
	t.Parallel()

	unknown := []Code{
		"",
		"UNKNOWN",
		"FORGE_CLI_UNKNOWN",           // missing suffix
		"FORGE_CONFIG_MALFORM",        // missing trailing D
		"FORGE_SECURITY_TRAVERSAL_",   // trailing underscore
		"FORGE_INTERNAL_UNEXPECTED_X", // extra suffix
		"FORGE_CONFIG_X",              // well-formed, unregistered

		// Near-misses for the WBS 13.1.1 codes.
		"FORGE_FILESYSTEM_NOT_LOCA",       // truncated
		"FORGE_FILESYSTEM_DEPTH_EXCEED",   // truncated
		"FORGE_FILESYSTEM_ENTRIES_EXCEED", // truncated
		"FORGE_FILESYSTEM_CASE_COLLISON",  // misspelled
		"FORGE_FILESYSTEM_NOT_LOCAL_X",    // extra suffix
	}
	for _, c := range unknown {
		c := c
		t.Run(string(c), func(t *testing.T) {
			t.Parallel()
			_, _, ok := Lookup(c)
			if ok {
				t.Fatalf("Lookup(%q) = (_, _, true), want false", c)
			}
			if IsRegistered(c) {
				t.Fatalf("IsRegistered(%q) = true, want false", c)
			}
		})
	}
}

// TestCategory_UnknownCodes asserts that a Code's Category method
// returns false for unregistered codes. The category derivation
// is only meaningful for registered codes: an unregistered string
// has no category to derive from.
func TestCategory_UnknownCodes(t *testing.T) {
	t.Parallel()

	unknown := []Code{
		"",
		"UNKNOWN",
		"FORGE_CLI_UNKNOWN",
		"FORGE_CONFIG_X",
	}
	for _, c := range unknown {
		c := c
		t.Run(string(c), func(t *testing.T) {
			t.Parallel()
			cat, ok := c.Category()
			if ok {
				t.Fatalf("Category() = (%q, true), want (_, false)", cat)
			}
			if cat != "" {
				t.Fatalf("Category() = %q, want empty on failure", cat)
			}
		})
	}
}

// TestBoundary_ShortestAndLongestCodes asserts that the shortest
// and longest registered codes are handled correctly by the
// category derivation. This is a sanity check on the length
// arithmetic in hasPrefix.
//
// The introduction of the four FORGE_FILESYSTEM_* codes in WBS
// 13.1.1 likely changes which code is the longest; the test does
// not name the specific code, so a length change does not require
// an edit here.
func TestBoundary_ShortestAndLongestCodes(t *testing.T) {
	t.Parallel()

	codes := AllCodes()
	if len(codes) == 0 {
		t.Fatal("registry is empty")
	}
	shortest, longest := codes[0], codes[0]
	for _, c := range codes[1:] {
		if len(c) < len(shortest) {
			shortest = c
		}
		if len(c) > len(longest) {
			longest = c
		}
	}

	for _, c := range []Code{shortest, longest} {
		if _, ok := c.Category(); !ok {
			t.Fatalf("Category() failed for boundary code %q", c)
		}
	}
}

// --- BDD: an end-to-end construction and lookup -----------------------

// TestBDD_ConfigMalformedRoundTrip walks through the exact
// scenario a config loader follows: construct an error with a
// registered code, then recover the code's metadata from the
// registry.
func TestBDD_ConfigMalformedRoundTrip(t *testing.T) {
	t.Parallel()

	e := New(CategoryConfig, CodeConfigMalformed, "invalid YAML in config.yaml")

	code := Code(e.Code())

	cat, ok := code.Category()
	if !ok {
		t.Fatalf("Category() = (_, false) for %q", code)
	}
	if cat != CategoryConfig {
		t.Fatalf("Category() = %v, want %v", cat, CategoryConfig)
	}

	introduced, deprecated, ok := Lookup(code)
	if !ok {
		t.Fatalf("Lookup(%q) = (_, _, false)", code)
	}
	if introduced == "" {
		t.Fatalf("Lookup(%q).introduced is empty", code)
	}
	if deprecated {
		t.Fatalf("Lookup(%q).deprecated = true, want false", code)
	}
}

// TestBDD_FilesystemNotLocalRoundTrip walks the same scenario
// through one of the four codes added in WBS 13.1.1. It exists so
// that the round-trip is exercised for the new codes at least
// once through the constructor, not only through the registry
// directly.
func TestBDD_FilesystemNotLocalRoundTrip(t *testing.T) {
	t.Parallel()

	e := New(CategoryFilesystem, CodeFilesystemNotLocal, "name is not local")

	code := Code(e.Code())
	cat, ok := code.Category()
	if !ok {
		t.Fatalf("Category() = (_, false) for %q", code)
	}
	if cat != CategoryFilesystem {
		t.Fatalf("Category() = %v, want %v", cat, CategoryFilesystem)
	}

	if _, deprecated, ok := Lookup(code); !ok || deprecated {
		t.Fatalf("Lookup(%q) = (_, %v, %v), want (_, false, true)", code, deprecated, ok)
	}
}

// --- Non-functional: AllCodes is a sorted copy ------------------------

// TestAllCodes_ReturnsCopy asserts that mutating the slice
// returned by AllCodes does not affect the registry. This is the
// same defensive-copy discipline as AllCategories and
// *Error.Context.
func TestAllCodes_ReturnsCopy(t *testing.T) {
	t.Parallel()

	first := AllCodes()
	if len(first) == 0 {
		t.Fatal("AllCodes returned an empty slice")
	}
	first[0] = "MUTATED"

	second := AllCodes()
	if second[0] == "MUTATED" {
		t.Fatal("AllCodes returned an aliased slice")
	}
}

// TestAllCodes_IsSorted asserts that AllCodes returns the codes in
// lexicographic order. The order is part of the contract: the
// golden file and the reference doc both depend on it.
func TestAllCodes_IsSorted(t *testing.T) {
	t.Parallel()

	codes := AllCodes()
	for i := 1; i < len(codes); i++ {
		if codes[i-1] >= codes[i] {
			t.Fatalf("AllCodes not sorted at index %d: %q >= %q", i, codes[i-1], codes[i])
		}
	}
}

// BenchmarkAllCodes measures the cost of the defensive copy that
// AllCodes performs. It is O(len(codeSpecs) log len(codeSpecs))
// because of the sort, and is expected to be called rarely
// (documentation generation, tests).
func BenchmarkAllCodes(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = AllCodes()
	}
}

// BenchmarkIsRegistered measures the cost of a registry lookup. It
// is a single map access, zero allocation, and is on the
// constructor's hot path.
func BenchmarkIsRegistered(b *testing.B) {
	c := CodeConfigMalformed
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = IsRegistered(c)
	}
}

// --- Fuzz: registry membership is a pure predicate --------------------

// FuzzRegistryMembership asserts that IsRegistered and Lookup
// agree on every input: a code is either registered and has a
// non-empty introduced field, or it is not registered and Lookup
// returns false. Divergence would mean two different answers to
// the same question.
//
// The seeds include every registered code, every shape of
// malformed input, and the near-misses for the WBS 13.1.1 codes.
// The fuzzer's job is to explore the space around these seeds;
// the seeds themselves are chosen so that a failing input is
// likely to be a plausible typo rather than a random byte string.
func FuzzRegistryMembership(f *testing.F) {
	for _, c := range AllCodes() {
		f.Add(string(c))
	}
	f.Add("")
	f.Add("UNKNOWN")
	f.Add("FORGE_CLI_UNKNOWN_COMMAND_X")
	f.Add("FORGE_CONFIG_MALFORM")
	f.Add("FORGE_CONFIG_X")
	f.Add("FORGE_")
	f.Add("FORGE_FILESYSTEM_NOT_LOCA")
	f.Add("FORGE_FILESYSTEM_NOT_LOCAL_X")

	f.Fuzz(func(t *testing.T, s string) {
		c := Code(s)
		registered := IsRegistered(c)
		_, _, ok := Lookup(c)
		if registered != ok {
			t.Fatalf("IsRegistered(%q)=%v, Lookup ok=%v", s, registered, ok)
		}
		if registered {
			introduced, _, _ := Lookup(c)
			if introduced == "" {
				t.Fatalf("registered code %q has empty introduced field", s)
			}
		}
	})
}

// --- Golden file and table renderers ----------------------------------
//
// These renderers are the machine-generated portion of the
// documentation pipeline. They are called by the sync tests above
// and by the -update flag.

// renderCodesGolden produces the canonical rendering of the
// registry used in testdata/codes/codes.golden.txt.
//
// The format is deliberately minimal and stable:
//
//	<code>\t<category>\t<exit>\t<introduced>\t<deprecated>
//
// One row per code, sorted by code. Rows are terminated by a
// single newline. The file is easy to diff, easy to generate, and
// easy to consume from a documentation generator.
//
// # Why the exit code is in the golden file
//
// The exit code is not a property of the registry — it is a
// property of the category, and Category.ExitCode is the single
// source of truth for it. The golden file includes the exit code
// anyway because the reference documentation at
// docs/reference/error-codes.md documents it, and a change to the
// exit-code mapping should be visible in the golden diff. The
// value is derived here, not stored; if Category.ExitCode changes,
// the golden file is regenerated in the same commit.
func renderCodesGolden() string {
	var b strings.Builder
	for _, c := range AllCodes() {
		cat, _ := c.Category()
		introduced, deprecated, _ := Lookup(c)
		b.WriteString(string(c))
		b.WriteByte('\t')
		b.WriteString(string(cat))
		b.WriteByte('\t')
		b.WriteString(intString(cat.ExitCode()))
		b.WriteByte('\t')
		b.WriteString(introduced)
		b.WriteByte('\t')
		if deprecated {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// renderCodesTable produces the markdown table that is embedded in
// docs/reference/error-codes.md.
//
// The table is the machine-generated portion of the reference
// doc; the surrounding prose is hand-written. The test
// TestAC5_DocsReferenceSyncWithRegistry asserts that the file
// contains this table verbatim.
func renderCodesTable() string {
	var b strings.Builder
	b.WriteString("| Code | Category | Exit | Introduced | Deprecated |\n")
	b.WriteString("|---|---|---|---|---|\n")
	for _, c := range AllCodes() {
		cat, _ := c.Category()
		introduced, deprecated, _ := Lookup(c)
		dep := "no"
		if deprecated {
			dep = "yes"
		}
		b.WriteString("| `")
		b.WriteString(string(c))
		b.WriteString("` | ")
		b.WriteString(string(cat))
		b.WriteString(" | ")
		b.WriteString(intString(cat.ExitCode()))
		b.WriteString(" | ")
		b.WriteString(introduced)
		b.WriteString(" | ")
		b.WriteString(dep)
		b.WriteString(" |\n")
	}
	return b.String()
}

// intString is a tiny helper that avoids importing strconv into
// this file. The only integers we render are small exit codes.
func intString(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
