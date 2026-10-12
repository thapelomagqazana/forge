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
const (
	// CategoryCLI classifies errors from CLI usage: unknown flags,
	// missing arguments, mutually-exclusive options, unknown
	// subcommands.
	//
	// Code prefix: "FORGE_CLI_".
	// Exit code: 2 (ExitUsage).
	CategoryCLI Category = "CLI"

	// CategoryConfig classifies errors from configuration loading
	// and validation: malformed files, unknown fields, invalid
	// enums, unresolvable paths.
	//
	// Code prefix: "FORGE_CONFIG_".
	// Exit code: 3 (ExitConfig).
	CategoryConfig Category = "CONFIG"

	// CategoryFilesystem classifies errors from file I/O: reads,
	// writes, path resolution, permissions.
	//
	// Code prefix: "FORGE_FILESYSTEM_".
	// Exit code: 4 (ExitFilesystem).
	CategoryFilesystem Category = "FILESYSTEM"

	// CategoryValidation classifies errors from schema, blueprint,
	// and check validation.
	//
	// Code prefix: "FORGE_VALIDATION_".
	// Exit code: 5 (ExitValidation).
	CategoryValidation Category = "VALIDATION"

	// CategoryTemplate classifies errors from template resolution
	// and rendering. A dedicated exit code may be assigned in a
	// future ADR; for now it shares ExitFailure with INTERNAL.
	//
	// Code prefix: "FORGE_TEMPLATE_".
	// Exit code: 1 (ExitFailure).
	CategoryTemplate Category = "TEMPLATE"

	// CategorySecurity classifies errors from path traversal,
	// secret handling, and trust-boundary violations.
	//
	// Code prefix: "FORGE_SECURITY_".
	// Exit code: 6 (ExitSecurity).
	CategorySecurity Category = "SECURITY"

	// CategoryInternal classifies bugs and unexpected panics.
	// Every error that does not fit another category is INTERNAL;
	// the taxonomy deliberately has no "OTHER" member.
	//
	// Code prefix: "FORGE_INTERNAL_".
	// Exit code: 1 (ExitFailure).
	CategoryInternal Category = "INTERNAL"
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
// The constants ExitUsage, ExitConfig, ExitFilesystem,
// ExitValidation, ExitFailure, and ExitSecurity are frozen in
// WBS 4.1.2 and defined in internal/cli/exitcodes.go. They are
// referenced here by their numeric values to keep the
// internal/forgeerr package free of a dependency on internal/cli.
// The two definitions must agree; the test
// TestAC4_ExitCodesMatchCLIConstants in category_test.go polices
// the numeric agreement.
//
// ExitCode panics if c is not a member of the closed set. The
// rationale is the same as for Prefix: ExitCode is called at the
// process boundary, and an invalid category at that point means
// the constructor's validation was bypassed.
func (c Category) ExitCode() int {
	switch c {
	case CategoryCLI:
		return 2 // ExitUsage
	case CategoryConfig:
		return 3 // ExitConfig
	case CategoryFilesystem:
		return 4 // ExitFilesystem
	case CategoryValidation:
		return 5 // ExitValidation
	case CategoryTemplate:
		return 1 // ExitFailure
	case CategorySecurity:
		return 6 // ExitSecurity
	case CategoryInternal:
		return 1 // ExitFailure
	default:
		panic("forgeerr: Category.ExitCode called on invalid category " + string(c))
	}
}
