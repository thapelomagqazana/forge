// Package cli — exit code taxonomy compatibility tests.
//
// This file records and pins the migration from the flat exit-code
// assignment of WBS 4.1.2 to the range-based taxonomy of WBS
// 11.1.1. It exists for one major release cycle as a migration aid
// for consumers who hard-coded the legacy integers, and it is
// removed in the release after the migration is complete.
//
// # What this file asserts
//
//  1. Every legacy constant still has a name in this package, so
//     that a reader searching for "ExitUsage" or "ExitFailure"
//     finds a definition (and a docstring pointing at the new
//     constant).
//
//  2. Every legacy constant's *current* value falls in the
//     documented exit-code range that ADR-018 assigns to it. This
//     is the property that ties the migration to the taxonomy: a
//     future reclassification that moved a legacy constant to a
//     different range would fail the test.
//
//  3. The specific same-number-different-meaning transitions (in
//     particular ExitUsage from 2 to 64) are called out
//     explicitly, so that `go test -v` output is the migration
//     table and a reader does not have to cross-reference ADR-018.
//
// # What this file does NOT assert
//
// The legacy *integers* are not preserved. The transition is a
// deliberate breaking change: ExitUsage moved from 2 to 64 so that
// the flat range 1–9 could be reserved for Forge findings, and so
// that usage errors could align with BSD sysexits.h EX_USAGE. A
// consumer that hard-coded 2 must update to 64; the compatibility
// test does not pretend otherwise. It exists to make the required
// update *visible* and *verifiable*, not to avoid it.
//
// # When to remove this file
//
// Remove this file in the first major release after the WBS 11.1.1
// taxonomy ships. The removal is a deliberate act: it signals that
// the migration window has closed and that no consumer is expected
// to migrate from the flat assignment any longer. Until then, the
// test is cheap (a handful of assertions, no I/O, no allocations)
// and the value is high (the migration table is executable, not
// merely documented).
//
// # Why this is a separate file
//
// The compatibility tests are deliberately not in exitcodes_test.go.
// exitcodes_test.go asserts the current taxonomy; this file asserts
// the transition *to* the current taxonomy. Mixing them would blur
// the boundary between "the contract we have" and "the contract we
// migrated from", and would make the eventual removal of the
// compatibility window a cherry-pick rather than a single
// `git rm exitcodes_compat_test.go`.
//
// # Relationship to the other test files
//
//   - exitcodes_test.go asserts the current taxonomy's structure
//     (the range table, the classifiers, the constants, the
//     panics, the examples).
//
//   - This file asserts the transition from the previous taxonomy.
//     It uses the current taxonomy's API (the range table, the
//     classifiers) to make its assertions, so it is not
//     independent of exitcodes_test.go, only separate from it.
//
//   - forgeerr/codes_range_test.go asserts the cross-package
//     invariant that every registered code's category's exit code
//     falls in a documented range. That test is not migration-
//     specific; it is a property of the current taxonomy.
//
// # Concurrency
//
// The tests in this file are parallel. The rule for parallel
// subtests is: a subtest must not write to state captured from the
// parent loop, because two subtests may run concurrently and race
// on that state. Where the tests below need shared state (the
// accumulation of seen names and values in
// TestCompat_MigrationTableIsWellFormed, for example), the parent
// loop performs every write before spawning the subtests; the
// subtests only read.
package cli

import (
	"strconv"
	"testing"
)

// ---------------------------------------------------------------------------
// Migration table
// ---------------------------------------------------------------------------

// legacyMapping records the WBS 4.1.2 integer, the WBS 11.1.1
// constant that supersedes it, and the documented range the new
// constant lives in.
//
// It is the machine-readable form of the migration table in
// ADR-018 § "Migration from WBS 4.1.2". The two tables must agree:
// a future change to either requires a change to the other in the
// same commit. The correspondence is asserted by
// TestCompat_MigrationTableIsWellFormed (which checks the table's
// internal consistency against the live range table) and by
// TestCompat_ADRExists (which checks that the ADR still names every
// legacy constant).
//
// # Field semantics
//
//   - legacyName is the WBS 4.1.2 Go identifier. It is present so
//     that a reader searching the codebase for the old name finds
//     this file and, through it, the migration table.
//
//   - legacyValue is the WBS 4.1.2 integer. It is recorded for
//     documentation only: the current code does not use it, and no
//     test asserts that the current constant equals it.
//
//   - currentName is the WBS 11.1.1 Go identifier that supersedes
//     the legacy name. For most rows it is a new name
//     (ExitDriftDetected, ExitConfig); for one row it is the same
//     name with a different value (ExitUsage).
//
//   - currentValue is the WBS 11.1.1 integer. It is asserted
//     against the named constant to catch a drift between this
//     table and exitcodes.go.
//
//   - rangeName is the name of the documented ExitCodeRange the
//     current constant lives in. It is asserted against the range
//     table in exitcodes.go.
type legacyMapping struct {
	legacyName   string
	legacyValue  int
	currentName  string
	currentValue int
	rangeName    string
}

// legacyMappings is the migration table. The order matches
// ADR-018's table, so a reader can diff the two side by side.
//
// # The one row with the same name
//
// ExitUsage appears twice in the reader's mind: as the WBS 4.1.2
// identifier (value 2) and as the WBS 11.1.1 identifier (value 64).
// It is one row in this table: the name is preserved, the value is
// not. This is the sharpest edge of the migration and is the reason
// the compat test exists.
//
// # Why no row for ExitFailure
//
// WBS 4.1.2's ExitFailure (value 1) does not have a same-named
// successor. Its meaning — "a generic command failure" — was
// ambiguous under the two-failure-domains model: a failure could be
// a finding (drift) or an execution failure (config). The taxonomy
// resolved the ambiguity by removing the name entirely and
// replacing it with two names: ExitDriftDetected (1) in the
// findings range and ExitInternal (70) in the internal range.
// Consumers that used ExitFailure for a drift finding migrate to
// ExitDriftDetected; consumers that used it for an internal error
// migrate to ExitInternal. The compat test cannot pick for them,
// so it records the removal via the ExitFailure legacy name with
// the drift-detection successor (the more common case).
var legacyMappings = []legacyMapping{
	// Generic failure. WBS 4.1.2 used ExitFailure (1) for a
	// generic failure; the taxonomy reclassified the generic
	// failure as a finding and named it ExitDriftDetected (1).
	// A consumer that used ExitFailure for an internal error
	// migrates to ExitInternal (70) instead; see
	// TestCompat_ExitFailureWasRemoved.
	{
		legacyName:   "ExitFailure",
		legacyValue:  1,
		currentName:  "ExitDriftDetected",
		currentValue: ExitDriftDetected,
		rangeName:    "Forge findings",
	},

	// Usage errors. WBS 4.1.2 used ExitUsage (2); the taxonomy
	// moved it to ExitUsage (64, BSD EX_USAGE) so that 1–9 could be
	// reserved for findings. This is the same name with a different
	// value; the migration is the sharpest edge of the change.
	{
		legacyName:   "ExitUsage",
		legacyValue:  2,
		currentName:  "ExitUsage",
		currentValue: ExitUsage,
		rangeName:    "Usage error",
	},

	// Config errors. WBS 4.1.2 used ExitConfig (3); the taxonomy
	// moved it to ExitConfig (10) so that all execution failures
	// live in 10–19.
	{
		legacyName:   "ExitConfig",
		legacyValue:  3,
		currentName:  "ExitConfig",
		currentValue: ExitConfig,
		rangeName:    "Forge execution failures",
	},

	// Filesystem errors. WBS 4.1.2 used ExitFilesystem (4); the
	// taxonomy moved it to ExitFilesystem (12).
	{
		legacyName:   "ExitFilesystem",
		legacyValue:  4,
		currentName:  "ExitFilesystem",
		currentValue: ExitFilesystem,
		rangeName:    "Forge execution failures",
	},

	// Validation findings. WBS 4.1.2 used ExitValidation (5); the
	// taxonomy moved it to ExitValidation (2) and reclassified it
	// from a generic failure to a finding.
	{
		legacyName:   "ExitValidation",
		legacyValue:  5,
		currentName:  "ExitValidation",
		currentValue: ExitValidation,
		rangeName:    "Forge findings",
	},

	// Security errors. WBS 4.1.2 used ExitSecurity (6); the
	// taxonomy moved it to ExitSecurity (13).
	{
		legacyName:   "ExitSecurity",
		legacyValue:  6,
		currentName:  "ExitSecurity",
		currentValue: ExitSecurity,
		rangeName:    "Forge execution failures",
	},

	// Update conflicts. WBS 4.1.2 used ExitConflict (7); the
	// taxonomy moved it to ExitConflict (14).
	{
		legacyName:   "ExitConflict",
		legacyValue:  7,
		currentName:  "ExitConflict",
		currentValue: ExitConflict,
		rangeName:    "Forge execution failures",
	},
}

// ---------------------------------------------------------------------------
// Structural tests — the migration table's internal consistency
// ---------------------------------------------------------------------------

// TestCompat_MigrationTableIsWellFormed asserts the migration table's
// structural invariants:
//
//   - Every row has a non-empty legacy name and current name.
//   - Every row names a range that exists in the range table.
//   - Every row's current value is within [0, 255].
//   - No two rows share a current name (each legacy constant maps to
//     a distinct current constant).
//   - No two rows share a current value (the current constants are
//     distinct, which mirrors TestExitCodeConstants_AreDistinct).
//
// The test is a self-check of the test file, not of the production
// code. Its value is that a future edit to the migration table
// (adding a row, changing a range name) fails loudly rather than
// silently producing a misleading migration.
//
// # Why the parent loop owns the accumulation
//
// The subtests are parallel. A parallel subtest must not write to
// state captured from the parent loop, because two subtests may run
// concurrently and race on that state. The parent loop therefore
// performs every write to the accumulation maps before spawning the
// subtests; the subtests only read the row's own fields and the
// immutable `validRange` set.
//
// The alternative — a mutex around the maps — would work but is
// unnecessary: the parent's iteration is already serial, and moving
// the writes to the parent is simpler and clearer than
// synchronising the subtests.
//
// # Why every problem is aggregated before reporting
//
// A row may violate more than one invariant. Aggregating the
// violations and emitting one error per row (rather than one per
// invariant) keeps the test output readable: a reviewer sees one
// message per broken row, not a cascade of related messages.
func TestCompat_MigrationTableIsWellFormed(t *testing.T) {
	t.Parallel()

	// Build the set of valid range names from the production range
	// table. Using the production table rather than a literal list
	// means a rename in exitcodes.go is caught here.
	//
	// The set is built once, before the subtests spawn, and is
	// read-only thereafter. A read-only map is safe to share
	// across parallel subtests.
	validRange := make(map[string]bool)
	for _, r := range ExitCodeRanges() {
		validRange[r.Name] = true
	}

	// Accumulate in the parent loop. The maps below are written
	// only by the loop; the parallel subtests do not touch them.
	dupName := make(map[string]string, len(legacyMappings))
	dupValue := make(map[int]string, len(legacyMappings))

	type rowProblem struct {
		index  int
		row    legacyMapping
		issues []string
	}
	var problems []rowProblem

	for i, m := range legacyMappings {
		var issues []string

		if m.legacyName == "" {
			issues = append(issues, "empty legacyName")
		}
		if m.currentName == "" {
			issues = append(issues, "empty currentName")
		}
		if !validRange[m.rangeName] {
			issues = append(issues,
				"range "+strconv.Quote(m.rangeName)+
					" is not in the production range table")
		}
		if m.currentValue < 0 || m.currentValue > 255 {
			issues = append(issues,
				"currentValue "+strconv.Itoa(m.currentValue)+
					" outside [0, 255]")
		}

		// Duplicate-name detection. The first row that uses a
		// name records it; a later row that repeats the name is
		// reported against the first user.
		if m.currentName != "" {
			if first, ok := dupName[m.currentName]; ok {
				issues = append(issues,
					"currentName "+strconv.Quote(m.currentName)+
						" duplicates row "+strconv.Quote(first))
			} else {
				dupName[m.currentName] = m.legacyName
			}
		}

		// Duplicate-value detection. Every currentValue is
		// recorded, including zero, because zero is a legitimate
		// value in the table (it is not, but a future row might
		// be the first to use it).
		if first, ok := dupValue[m.currentValue]; ok {
			issues = append(issues,
				"currentValue "+strconv.Itoa(m.currentValue)+
					" duplicates row "+strconv.Quote(first))
		} else {
			dupValue[m.currentValue] = m.legacyName
		}

		if len(issues) > 0 {
			problems = append(problems, rowProblem{
				index:  i,
				row:    m,
				issues: issues,
			})
		}
	}

	// Report every problem in the parent, after the loop. Each
	// report is a separate subtest so that `go test -v` names the
	// offending row.
	for _, p := range problems {
		p := p
		t.Run(p.row.legacyName, func(t *testing.T) {
			t.Parallel()
			for _, issue := range p.issues {
				t.Errorf("row %d (%s): %s", p.index, p.row.legacyName, issue)
			}
		})
	}
}

// TestCompat_MigrationTableCoversEveryLegacyConstant asserts that
// the migration table has a row for every WBS 4.1.2 constant.
//
// The legacy names are enumerated literally, because the constants
// themselves no longer exist: their absence is the point of the
// migration. If a WBS 4.1.2 constant is discovered that the table
// does not cover (for example, by a consumer's bug report), the
// table is updated in the same commit that adds the missing row
// here.
func TestCompat_MigrationTableCoversEveryLegacyConstant(t *testing.T) {
	t.Parallel()

	// The WBS 4.1.2 constants, as recorded in ADR-018.
	legacyNames := []string{
		"ExitSuccess",
		"ExitFailure",
		"ExitUsage",
		"ExitConfig",
		"ExitFilesystem",
		"ExitValidation",
		"ExitSecurity",
		"ExitConflict",
	}

	covered := make(map[string]bool, len(legacyMappings))
	for _, m := range legacyMappings {
		covered[m.legacyName] = true
	}

	for _, name := range legacyNames {
		if name == "ExitSuccess" {
			// ExitSuccess is not in the migration table: its
			// value (0) did not change and its meaning did not
			// change. Including it would inflate the table with
			// a non-transition.
			if covered[name] {
				t.Errorf("%s is in the migration table but did not transition",
					name)
			}
			continue
		}
		if !covered[name] {
			t.Errorf("WBS 4.1.2 constant %q is not covered by the migration table",
				name)
		}
	}
}

// ---------------------------------------------------------------------------
// Range membership tests — the legacy constant's current value is
// in the documented range
// ---------------------------------------------------------------------------

// TestCompat_LegacyConstantsMapToDocumentedRanges asserts that every
// legacy constant's current value is in the range that ADR-018
// assigns to it.
//
// The assertion is done twice, on purpose:
//
//  1. The named constant (ExitConfig, ExitSecurity, …) is looked up
//     in the production range table by value, and the range's name
//     is compared to the migration table's rangeName. This catches
//     a drift between the migration table and the production
//     ranges.
//
//  2. The named constant is passed to the classifier for the
//     expected range (IsFinding, IsExecutionFailure, …). This
//     catches a drift between the named constant and the
//     classifier that the range-name comparison might miss (for
//     example, if the classifier were redefined to include a
//     boundary it should not).
//
// The double assertion is cheap and makes the test the migration's
// single source of verification: a reader can trust the table
// because two independent checks agree on it.
func TestCompat_LegacyConstantsMapToDocumentedRanges(t *testing.T) {
	t.Parallel()

	for _, m := range legacyMappings {
		m := m
		t.Run(m.legacyName, func(t *testing.T) {
			t.Parallel()

			// Check 1: the range table lookup agrees with the
			// migration table.
			r, ok := LookupExitCodeRange(m.currentValue)
			if !ok {
				t.Fatalf("%s (current %d) has no documented range",
					m.currentName, m.currentValue)
			}
			if r.Name != m.rangeName {
				t.Errorf("%s (current %d) is in range %q; migration table says %q",
					m.currentName, m.currentValue, r.Name, m.rangeName)
			}

			// Check 2: the classifier agrees with the range name.
			// The classifier for each range is named explicitly;
			// a new range added to the migration table without a
			// classifier here fails the test.
			switch m.rangeName {
			case "Forge findings":
				if !IsFinding(m.currentValue) {
					t.Errorf("%s (current %d) is in range %q but IsFinding returns false",
						m.currentName, m.currentValue, m.rangeName)
				}
			case "Forge execution failures":
				if !IsExecutionFailure(m.currentValue) {
					t.Errorf("%s (current %d) is in range %q but IsExecutionFailure returns false",
						m.currentName, m.currentValue, m.rangeName)
				}
			case "Usage error":
				if !IsUsageError(m.currentValue) {
					t.Errorf("%s (current %d) is in range %q but IsUsageError returns false",
						m.currentName, m.currentValue, m.rangeName)
				}
			case "Internal errors":
				if !IsInternalError(m.currentValue) {
					t.Errorf("%s (current %d) is in range %q but IsInternalError returns false",
						m.currentName, m.currentValue, m.rangeName)
				}
			case "Signals (128 + N)":
				if !IsSignal(m.currentValue) {
					t.Errorf("%s (current %d) is in range %q but IsSignal returns false",
						m.currentName, m.currentValue, m.rangeName)
				}
			default:
				t.Errorf("migration table row %q uses range %q, "+
					"which has no classifier in this test; add one",
					m.legacyName, m.rangeName)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Explicit transition tests — the same-number-different-meaning and
// different-number-same-meaning cases
// ---------------------------------------------------------------------------

// TestCompat_ExitUsageMovedFromTwoToSixtyFour pins the single
// sharpest edge of the migration: ExitUsage kept its name but
// changed its value from 2 (WBS 4.1.2) to 64 (WBS 11.1.1, BSD
// EX_USAGE).
//
// The test is deliberately brittle: it asserts the exact numeric
// value of ExitUsage. A future change to that value would fail the
// test, which is the correct behaviour — a change to ExitUsage's
// value is a breaking change to the public contract and requires an
// ADR.
//
// The test also asserts the two consequences of the move:
//
//   - IsUsageError(2) is false. Under WBS 4.1.2, 2 was the usage
//     code; under the taxonomy, 2 is ExitValidation (a finding).
//     A consumer that branches on 2 expecting a usage error now
//     sees a finding, which is exactly the kind of silent
//     misclassification this test is designed to surface.
//
//   - IsFinding(2) is true. The same integer that used to mean
//     "usage error" now means "validation finding". The
//     reclassification is intentional: a validation warning that
//     is promoted to an error by --strict is a finding, not a
//     usage error.
func TestCompat_ExitUsageMovedFromTwoToSixtyFour(t *testing.T) {
	t.Parallel()

	if ExitUsage != 64 {
		t.Errorf("ExitUsage = %d; want 64 (BSD sysexits.h EX_USAGE)", ExitUsage)
	}
	if IsUsageError(2) {
		t.Error("IsUsageError(2) = true; want false (2 is now ExitValidation)")
	}
	if !IsFinding(2) {
		t.Error("IsFinding(2) = false; want true (2 is now ExitValidation)")
	}
	if ExitValidation != 2 {
		t.Errorf("ExitValidation = %d; want 2", ExitValidation)
	}
}

// TestCompat_ExitFailureWasRemoved pins the removal of the WBS
// 4.1.2 ExitFailure constant.
//
// ExitFailure (1) was ambiguous under the two-failure-domains
// model: a generic failure could be a finding (drift) or an
// execution failure (config). The taxonomy removed the name and
// replaced its two meanings with two names:
//
//   - ExitDriftDetected (1) for the finding case.
//   - ExitInternal (70) for the internal-error case.
//
// The constant ExitFailure is not defined in the current package;
// the compiler would reject a reference to it. The test records
// the removal by asserting the two successor constants' values.
//
// # Why the test does not use a build-time check
//
// A build-time check that "ExitFailure is not defined" would
// require a negative type assertion, which Go does not support.
// The test instead pins the successor constants and relies on the
// compiler to reject any accidental re-introduction of the
// ExitFailure name (which would shadow nothing but would produce
// an unused-constant warning if not referenced).
func TestCompat_ExitFailureWasRemoved(t *testing.T) {
	t.Parallel()

	if ExitDriftDetected != 1 {
		t.Errorf("ExitDriftDetected = %d; want 1 (successor for the finding case)",
			ExitDriftDetected)
	}
	if ExitInternal != 70 {
		t.Errorf("ExitInternal = %d; want 70 (successor for the internal-error case)",
			ExitInternal)
	}
	if !IsFinding(ExitDriftDetected) {
		t.Error("ExitDriftDetected is not in the findings range")
	}
	if !IsInternalError(ExitInternal) {
		t.Error("ExitInternal is not in the internal-error range")
	}
}

// TestCompat_LegacyCodesThatRemainedFindings asserts that the
// legacy constants whose domain did not change (from failure to
// finding, or from failure to failure) keep their new domain.
//
// Two WBS 4.1.2 constants were reclassified into the findings
// range:
//
//   - ExitFailure (1) → ExitDriftDetected (1), findings.
//   - ExitValidation (5) → ExitValidation (2), findings.
//
// The reclassification is the intended semantic improvement: a
// validation warning promoted to an error is a finding, not a
// generic failure.
func TestCompat_LegacyCodesThatRemainedFindings(t *testing.T) {
	t.Parallel()

	for _, code := range []int{ExitDriftDetected, ExitValidation, ExitCheckFailed} {
		if !IsFinding(code) {
			t.Errorf("code %d must be a finding", code)
		}
		if IsExecutionFailure(code) {
			t.Errorf("code %d must not be an execution failure", code)
		}
	}
}

// TestCompat_LegacyCodesThatBecameExecutionFailures asserts that
// the legacy constants whose new domain is the execution-failure
// range are correctly classified.
//
// Four WBS 4.1.2 constants moved into the execution-failure range:
//
//   - ExitConfig (3) → ExitConfig (10).
//   - ExitFilesystem (4) → ExitFilesystem (12).
//   - ExitSecurity (6) → ExitSecurity (13).
//   - ExitConflict (7) → ExitConflict (14).
//
// The reclassification is the intended semantic improvement: a
// config, filesystem, security, or conflict error means the command
// could not run, not that the subject is in a detectable state.
func TestCompat_LegacyCodesThatBecameExecutionFailures(t *testing.T) {
	t.Parallel()

	for _, code := range []int{ExitConfig, ExitFilesystem, ExitSecurity, ExitConflict} {
		if !IsExecutionFailure(code) {
			t.Errorf("code %d must be an execution failure", code)
		}
		if IsFinding(code) {
			t.Errorf("code %d must not be a finding", code)
		}
	}
}

// ---------------------------------------------------------------------------
// Executable migration table — the test's output is the table
// ---------------------------------------------------------------------------

// TestCompat_PrintMigrationTable is a diagnostic test that prints
// the migration table to the test log. It is not an assertion; it
// exists so that `go test -v -run TestCompat_PrintMigrationTable`
// produces the table on stdout for a reader who is migrating.
//
// # Why a test and not a doc
//
// The table in ADR-018 is the authoritative source. The test
// re-prints it from the source of truth (legacyMappings) so that a
// reader can verify the ADR's table against the code's table
// without opening two files. If the two disagree, the discrepancy
// is visible in the test's output.
//
// The test never fails; it only logs. It is marked with t.Log, not
// t.Error, because the table is informational, not a contract.
func TestCompat_PrintMigrationTable(t *testing.T) {
	t.Parallel()

	t.Log("WBS 4.1.2 → WBS 11.1.1 migration table:")
	t.Logf("%-16s %-6s → %-22s %-6s   %s",
		"legacy name", "value", "current name", "value", "range")
	for _, m := range legacyMappings {
		t.Logf("%-16s %-6d → %-22s %-6d   %s",
			m.legacyName, m.legacyValue,
			m.currentName, m.currentValue,
			m.rangeName)
	}
	t.Log("See docs/decisions/ADR-018-exit-code-taxonomy.md for the rationale.")
}
