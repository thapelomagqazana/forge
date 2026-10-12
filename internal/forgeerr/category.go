package forgeerr

// Category classifies a *Error for exit-code mapping and for the
// derivation of the Code prefix.
//
// The taxonomy is a closed set of seven categories. Adding a
// category requires an ADR (see WBS 10.2.1 and ADR-013). The set
// is enumerated in allCategories; callers use AllCategories() to
// obtain a defensive copy.
//
// Design notes:
//
//   - Category is string-backed, not int-backed. String categories
//     survive serialization (JSON error envelopes, log lines),
//     produce readable diffs, and do not need renumbering when a
//     member is inserted. The cost — a few bytes per error — is
//     irrelevant at the CLI boundary, where errors are constructed
//     once per process.
//
//   - Each category carries its Code prefix and its process exit
//     code as method behaviour, not as parallel maps. There is
//     exactly one place to change when a prefix or an exit code
//     changes, and that place is the corresponding switch.
//
//   - The empty string is NOT a valid category. It exists only as
//     the zero value and is rejected by validateCategory. This is
//     the same discipline as *Error's unexported code and message:
//     a value that could be a category but is not is a programming
//     error, not a runtime error.
//
// # The exit-code taxonomy (WBS 11.1.1)
//
// ExitCode returns the process exit code associated with the
// category. The values are grouped into documented ranges defined
// in internal/cli/exitcodes.go and published in
// docs/reference/exit-code-ranges.md. The two non-zero domains
// that matter for the categories below are:
//
//   - Forge findings (1–9): the command ran correctly; the subject
//     of the command is in a detectable state. CategoryValidation
//     lives here.
//
//   - Forge execution failures (10–19): the command could not run;
//     Forge itself encountered a problem. CategoryConfig,
//     CategoryTemplate, CategoryFilesystem, CategorySecurity, and
//     CategoryConflict live here.
//
// Two categories sit outside those ranges:
//
//   - CategoryCLI maps to the usage singleton (64, BSD EX_USAGE).
//   - CategoryInternal maps to the internal-error range (70–78;
//     the generic code is 70, BSD EX_SOFTWARE).
//
// The numeric values are a public contract. They are additive only
// and are never reassigned. See ADR-018 for the decision and
// internal/cli/exitcodes.go for the constants. Before the WBS
// 11.1.1 taxonomy, the values were flat (1–7); the change is the
// breaking change documented in ADR-018 § "Migration from WBS
// 4.1.2".
//
// # Why CategoryConflict is declared below but not in the set
//
// The allCategories slice enumerates the seven categories that the
// Phase 2 constructors accept. CategoryConflict is declared as a
// constant (see the constant block below) but is not a member of
// the set: it is reserved for a future phase, and its exit code
// (14) is documented here so that a future ADR that adds the
// category to the set has the value already assigned.
//
// The type is safe for concurrent use: it is a string, and all
// methods are pure.
type Category string

// The closed set of categories. Each constant is a member of
// allCategories; see that variable for the canonical enumeration.
//
// Adding a member here requires an ADR. The test suite in
// category_test.go asserts that AllCategories() and allCategories
// agree; a constant declared but missing from allCategories fails
// the build.
//
// # Exit codes and the WBS 11.1.1 taxonomy
//
// The "Exit code:" line in each docstring names the WBS 11.1.1
// value and the range it belongs to. The values are literals in
// the ExitCode switch below; the named constants live in
// internal/cli/exitcodes.go (which forgeerr cannot import without
// creating a cycle). TestAC4_ExitCodesMatchCLIConstants in
// category_test.go polices the agreement between the literals and
// the constants.
const (
	// CategoryCLI classifies errors from CLI usage: unknown flags,
	// missing arguments, mutually-exclusive options, unknown
	// subcommands.
	//
	// Code prefix: "FORGE_CLI_".
	// Exit code: 64 (ExitUsage; BSD EX_USAGE). The usage singleton.
	CategoryCLI Category = "CLI"

	// CategoryConfig classifies errors from configuration loading
	// and validation: malformed files, unknown fields, invalid
	// enums, unresolvable paths.
	//
	// Code prefix: "FORGE_CONFIG_".
	// Exit code: 10 (ExitConfig; Forge execution failures).
	CategoryConfig Category = "CONFIG"

	// CategoryFilesystem classifies errors from file I/O: reads,
	// writes, path resolution, permissions.
	//
	// Code prefix: "FORGE_FILESYSTEM_".
	// Exit code: 12 (ExitFilesystem; Forge execution failures).
	CategoryFilesystem Category = "FILESYSTEM"

	// CategoryValidation classifies errors from schema, blueprint,
	// and check validation.
	//
	// Code prefix: "FORGE_VALIDATION_".
	// Exit code: 2 (ExitValidation; Forge findings).
	//
	// The category moved from the execution-failure side of the
	// taxonomy to the findings side during WBS 11.1.1: a validation
	// warning that is promoted to an error by --strict is a
	// finding, not a tool malfunction. See ADR-018 § "Migration
	// from WBS 4.1.2".
	CategoryValidation Category = "VALIDATION"

	// CategoryTemplate classifies errors from template resolution
	// and rendering.
	//
	// Code prefix: "FORGE_TEMPLATE_".
	// Exit code: 11 (ExitTemplate; Forge execution failures).
	//
	// Before WBS 11.1.1, this category shared ExitFailure with
	// INTERNAL; the taxonomy assigned it a dedicated code so that
	// a template error is distinguishable from an internal bug.
	CategoryTemplate Category = "TEMPLATE"

	// CategorySecurity classifies errors from path traversal,
	// secret handling, and trust-boundary violations.
	//
	// Code prefix: "FORGE_SECURITY_".
	// Exit code: 13 (ExitSecurity; Forge execution failures).
	CategorySecurity Category = "SECURITY"

	// CategoryInternal classifies bugs and unexpected panics.
	// Every error that does not fit another category is INTERNAL;
	// the taxonomy deliberately has no "OTHER" member.
	//
	// Code prefix: "FORGE_INTERNAL_".
	// Exit code: 70 (ExitInternal; BSD EX_SOFTWARE, in the
	// internal-error range 70–78).
	//
	// Before WBS 11.1.1, this category returned 1 (ExitFailure).
	// The taxonomy moved it to the sysexits.h range so that
	// internal bugs are distinguishable from usage errors and
	// from findings. See ADR-018 § "Migration from WBS 4.1.2".
	CategoryInternal Category = "INTERNAL"

	// CategoryConflict classifies update conflicts: a change could
	// not be applied without overwriting developer modifications.
	//
	// Code prefix: "FORGE_CONFLICT_".
	// Exit code: 14 (ExitConflict; Forge execution failures).
	//
	// This category is NOT a member of allCategories in Phase 2.
	// It is declared here, and its exit code is assigned here, so
	// that the future ADR that adds it to the closed set has the
	// value already recorded. Until that ADR, the constructors
	// reject CategoryConflict and the category is not reachable
	// from any registered code.
	CategoryConflict Category = "CONFLICT"
)

// allCategories is the canonical enumeration of the closed set.
//
// It is unexported so that callers cannot mutate it; callers use
// AllCategories, which returns a copy.
//
// Ordering is deliberate and stable: the slice is used by
// documentation generators and by tests that walk every category.
// Reordering would break golden files, so treat the order as part
// of the contract. TestAC1_EnumerationOrderIsStable pins it.
//
// CategoryConflict is deliberately absent: it is declared above as
// a reserved constant, but the Phase 2 constructors do not accept
// it. The absence is the mechanism that keeps the category
// unreachable until a future ADR adds it.
var allCategories = []Category{
	CategoryCLI,
	CategoryConfig,
	CategoryFilesystem,
	CategoryValidation,
	CategoryTemplate,
	CategorySecurity,
	CategoryInternal,
}

// AllCategories returns a copy of the closed set of categories.
//
// The copy protects the internal enumeration from mutation: a
// caller that reorders or truncates the returned slice does not
// affect the package's definition of the taxonomy.
//
// The order is stable: CLI, CONFIG, FILESYSTEM, VALIDATION,
// TEMPLATE, SECURITY, INTERNAL.
//
// AllCategories is intended for documentation generators, tests,
// and any code that must iterate over every category. It is not on
// the hot path of error construction; the constructors consult
// valid() directly.
func AllCategories() []Category {
	out := make([]Category, len(allCategories))
	copy(out, allCategories)
	return out
}

// valid reports whether c is a member of the closed set.
//
// The check is a linear scan over allCategories. The set has seven
// members, so a scan is faster than a map lookup and has zero
// allocation. If the set ever grows beyond ~20 members, switch to
// a package-level map; the call sites do not change.
//
// valid is unexported: callers outside the package should not need
// to ask "is this a category?" — they should be constructing
// errors through New or Wrap, which validate. Internal callers
// (validateCategory, the constructors, tests) use valid directly.
func (c Category) valid() bool {
	for _, k := range allCategories {
		if c == k {
			return true
		}
	}
	return false
}

// String returns the canonical name of the category. It is used in
// logs and in the JSON error envelope (WBS 10.4.1).
//
// The name is the same as the underlying string value, so this
// method exists for two reasons:
//
//   - It satisfies fmt.Stringer, so %v and %s format categories
//     correctly in logs and error messages.
//
//   - It gives the "unknown" case a defined home. A zero-value
//     Category is not valid, but String must not panic; returning
//     "unknown" lets diagnostics proceed without a nil check.
//
// The asymmetry with Prefix and ExitCode (which do panic on
// invalid input) is deliberate: String is for humans and must be
// total; Prefix and ExitCode are for the program and are called
// only after validation, so a panic there signals a bug.
func (c Category) String() string {
	if !c.valid() {
		return "unknown"
	}
	return string(c)
}

// Prefix returns the Code prefix required for this category.
//
// Example: CategoryConfig.Prefix() == "FORGE_CONFIG_".
//
// Prefix panics if c is not a member of the closed set. This is
// deliberate: Prefix is called only from the constructors, which
// have already validated the category. A panic here means the
// constructor's validation was bypassed, which is a programming
// error.
//
// The empty string is not returned for any valid category; the
// constructors rely on this to enforce the code-prefix rule (AC7).
func (c Category) Prefix() string {
	switch c {
	case CategoryCLI:
		return "FORGE_CLI_"
	case CategoryConfig:
		return "FORGE_CONFIG_"
	case CategoryFilesystem:
		return "FORGE_FILESYSTEM_"
	case CategoryValidation:
		return "FORGE_VALIDATION_"
	case CategoryTemplate:
		return "FORGE_TEMPLATE_"
	case CategorySecurity:
		return "FORGE_SECURITY_"
	case CategoryInternal:
		return "FORGE_INTERNAL_"
	default:
		panic("forgeerr: Category.Prefix called on invalid category " + string(c))
	}
}

// ExitCode returns the process exit code for this category.
//
// The mapping is the WBS 11.1.1 taxonomy (docs/reference/exit-code-ranges.md,
// ADR-018). The values are frozen: once shipped, a category's exit
// code is never reassigned. A new category is added at the next
// available integer within its range.
//
// # The mapping
//
//	CategoryCLI         → 64  (ExitUsage; BSD EX_USAGE)
//	CategoryConfig      → 10  (ExitConfig; execution failure)
//	CategoryTemplate    → 11  (ExitTemplate; execution failure)
//	CategoryFilesystem  → 12  (ExitFilesystem; execution failure)
//	CategorySecurity    → 13  (ExitSecurity; execution failure)
//	CategoryConflict    → 14  (ExitConflict; execution failure)
//	CategoryValidation  → 2   (ExitValidation; finding)
//	CategoryInternal    → 70  (ExitInternal; BSD EX_SOFTWARE)
//
// # Why the values are literals
//
// The constants (ExitUsage, ExitConfig, …) live in
// internal/cli/exitcodes.go so that internal/forgeerr does not
// depend on internal/cli. The dependency goes one way: cli
// imports forgeerr. This method therefore returns numeric
// literals, and TestAC4_ExitCodesMatchCLIConstants in
// category_test.go asserts that the literals agree with the CLI
// constants. If the two definitions drift, the test fails and
// both are updated in the same commit.
//
// # The two failure domains
//
// The values fall into two non-zero domains — findings (1–9) and
// execution failures (10–19) — plus two singletons outside those
// ranges (64 for usage, 70 for internal). The split lets a caller
// branch on code < 10 vs. code >= 10 without parsing output; see
// ADR-018 for the decision.
//
// # Why CategoryConflict is handled here
//
// CategoryConflict is not a member of allCategories in Phase 2,
// but it is a declared constant with an assigned exit code (14,
// ExitConflict). The switch below includes it so that a future
// ADR that adds the category to the set does not need to revisit
// this method: the value is already correct.
//
// # The unknown-category case
//
// ExitCode panics if c is not a member of the closed set. The
// rationale is the same as for Prefix: ExitCode is called at the
// process boundary, and an invalid category at that point means
// the constructor's validation was bypassed. The panic is
// deliberately not a fallback value: a silent fallback would
// classify an unknown error as internal, which would hide the
// bug. A panic surfaces it in the test suite.
func (c Category) ExitCode() int {
	switch c {
	case CategoryCLI:
		return 64 // ExitUsage; usage singleton
	case CategoryConfig:
		return 10 // ExitConfig; execution failures
	case CategoryTemplate:
		return 11 // ExitTemplate; execution failures
	case CategoryFilesystem:
		return 12 // ExitFilesystem; execution failures
	case CategorySecurity:
		return 13 // ExitSecurity; execution failures
	case CategoryConflict:
		return 14 // ExitConflict; execution failures
	case CategoryValidation:
		return 2 // ExitValidation; findings
	case CategoryInternal:
		return 70 // ExitInternal; internal errors
	default:
		panic("forgeerr: Category.ExitCode called on invalid category " + string(c))
	}
}

// DefaultExitCode returns the exit code that applies to this
// category when no more specific code is set on an error.
//
// In the current design, the exit code is a property of the
// category and there is no per-error override: DefaultExitCode is
// therefore identical to ExitCode. The two names exist so that the
// mapping contract (WBS 11.2.1) can refer to "the category's default
// exit code" without implying that a per-error override exists or
// ever will.
//
// A future ADR that introduces per-error exit codes (for example, a
// ForgeError whose Code field is an exit code distinct from its
// category's) would change ExitCode to consult the per-error value
// first and fall back to DefaultExitCode. The two-method split is
// the seam for that change: callers that want "the category's
// default, regardless of any per-error override" call
// DefaultExitCode; callers that want "the effective exit code for
// this error" call ExitCode.
//
// # Stability
//
// The value is the same as ExitCode's and is subject to the same
// stability policy: once shipped, a category's default exit code is
// never reassigned. See ADR-018 and ADR-019.
func (c Category) DefaultExitCode() int {
	return c.ExitCode()
}
