package config

import "strings"

// This file holds the config scope contract (WBS 8.1.1, amended by
// ADR-009 and ADR-022). The contract names the config's fields and
// the rules that constrain them, and it distinguishes three
// categories of field: scalar value fields, sections, and
// bookkeeping fields.
//
// # Scalar value fields
//
// The scalar value fields are the fields that carry scalar
// configuration values. There are exactly three:
//
//	LogLevel       the log level
//	OutputFormat   the output format
//	SourcePath     the resolved config file path (metadata)
//
// Every scalar value field is exported and has type string. The
// count, exportedness, and type are frozen. Adding, removing, or
// retyping a scalar value field requires an ADR.
//
// # Sections
//
// A section is an exported field whose type is a pointer to a
// struct defined in this package. A section carries a named group
// of related configuration values (for example, Registry.URL and
// Registry.Token). Sections are part of the configuration surface,
// but they are a distinct category from scalar value fields: they
// are not counted by configFieldCount, and they are not required
// to have type string.
//
// The current sections are listed by InScopeSectionNames. The
// first sections were added by WBS 12.7.1 to hold the credential
// fields declared `sensitive: true` in the schema. Adding a
// section requires an ADR: it widens the configuration surface,
// even though the surface is nested.
//
// # Bookkeeping fields
//
// A bookkeeping field is an unexported field that carries internal
// state about the value fields. It is not part of the
// configuration surface; a caller who reads the value fields
// never sees it. The contract permits bookkeeping fields so that
// the loader can attach metadata (such as source attribution, per
// WBS 9.2.2) to the Config value without widening the
// configuration surface.
//
// The current bookkeeping fields are listed by
// InScopeBookkeepingFieldNames. The naming convention is: a
// bookkeeping field's name is descriptive of the value fields it
// describes (for example, "sources" describes the source of each
// value).
//
// Adding a bookkeeping field requires an amendment to the scope
// contract, but not a full ADR: the field does not change the
// configuration surface, which is what the ADR process is
// protecting. ADR-009 is the first such amendment.

// The valid values for Config.LogLevel.
//
// # Why constants
//
// The values are referenced by the logging subsystem (WBS 12.0)
// and by `forge config`. A typo in one place is a bug; the
// constants make the values' spelling the single source of truth.
//
// The values are lowercase, ASCII-only, and stable. A future
// addition to the set is a change to the logging contract and
// requires an ADR.
const (
	// LogLevelError suppresses everything except errors.
	LogLevelError = "error"

	// LogLevelWarn suppresses INFO and DEBUG, keeps WARN and ERROR.
	LogLevelWarn = "warn"

	// LogLevelInfo is the default; keeps INFO, WARN, and ERROR.
	LogLevelInfo = "info"

	// LogLevelDebug keeps everything.
	LogLevelDebug = "debug"
)

// The valid values for Config.OutputFormat.
//
// # Why "text" and "human" are both present
//
// The two values are aliases with a subtle difference:
//
//   - "text" is the machine-friendly line-oriented format used by
//     `forge version` (frozen by WBS 6.4.1).
//   - "human" is the prose-oriented format used by the creation
//     and inspection commands (Phase 5).
//
// The two values are distinct because the two formats are
// distinct. A command that supports both declares both in its
// per-command --format value set (see docs/cli-ux-spec.md § 4.12).
//
// A future change to add "json-pretty" or "sarif" is a change to a
// command's value set, not to this list. The list here is the set
// of values the CLI config can hold as a default.
const (
	// OutputFormatText is the machine-friendly text format.
	OutputFormatText = "text"

	// OutputFormatJSON is the machine-readable JSON format.
	OutputFormatJSON = "json"
)

// configFieldCount is the number of scalar value fields the Config
// type has.
//
// # Why a constant
//
// The scope contract (WBS 8.1.1, amended by ADR-009) freezes the
// scalar value-field count at three. The test
// TestScope_ConfigHasExactlyThreeValueFields reads this constant;
// adding a scalar value field requires updating the constant and
// the ADR.
//
// # Why "scalar value fields" and not "fields"
//
// ADR-009 distinguishes value fields (exported, carrying scalar
// configuration values) from bookkeeping fields (unexported,
// carrying internal state). WBS 12.7.1 added a third category,
// sections (exported, pointer-to-struct, carrying a named group
// of related values), recognized by ADR-022. The constant counts
// scalar value fields only. Neither the bookkeeping fields' count
// nor the sections' count is frozen by this constant.
//
// The constant is not exported. A future caller that wants the
// count can compute it with reflection; the constant is for the
// package's own use.
const configFieldCount = 3

// forbiddenFieldNameSubstrings is the set of substrings whose
// presence in a Config field name indicates a scope violation.
//
// # Why a list
//
// The scope contract forbids three categories of fields: secrets,
// credentials, and repository paths. The list below names the
// substrings that identify each category. A field whose name
// contains any of the substrings fails the scope test.
//
// # Why the check is name-based, not type-based
//
// A field named "SecretValue" of type string is a violation
// regardless of what the string holds. The name is the signal: a
// future contributor who adds a field with that name is thinking
// about secrets, and the test forces them to consider the scope
// contract. A type-based check (for example, "no field of type
// []byte") would be easy to circumvent with a string field; a
// name-based check is harder to circumvent accidentally.
//
// # Why the list is over-broad
//
// The list includes substrings that a legitimate field might
// contain. For example, a field named "PathPrefix" would match
// "Path" and would be rejected even if the field held a harmless
// value. The over-broadness is deliberate: a contributor whose
// field's name coincidentally matches opens an ADR, renames the
// field, and updates the list. The friction is small; the benefit
// is that no secret-bearing field can slip through.
//
// # Why a function, not a variable
//
// The CLI package's contract test forbids package-level
// variables. A function that returns a fresh slice on each call
// has no package-level state.
func forbiddenFieldNameSubstrings() []string {
	return []string{
		// Secrets and credentials.
		"secret",
		"token",
		"password",
		"passwd",
		"credential",
		"apikey",
		"api_key",
		"privatekey",
		"private_key",
		"access_key",
		"accesskey",

		// Repository paths.
		"repository",
		"repo",
		"workdir",
		"workingdir",
		"working_dir",
	}
}

// IsForbiddenFieldName reports whether name contains a substring
// from forbiddenFieldNameSubstrings.
//
// The comparison is case-insensitive: "SecretValue" and
// "secretvalue" both match "secret". A field name that contains a
// forbidden substring is a scope violation regardless of the
// field's type.
//
// # Why case-insensitive
//
// Go's exported field names are CamelCase; an unexported field
// would be camelCase. The forbidden substrings are lowercase, so
// a case-sensitive check would match "SecretValue" only after
// lowercasing. The function lowercases the input before
// comparing, so both forms are caught.
//
// # Applies to both value and bookkeeping fields
//
// The check applies to every field of the Config type, whether
// it is a scalar value field (exported), a section (exported), or
// a bookkeeping field (unexported). A bookkeeping field named
// "secretCache" would be rejected by the check even though the
// field is not part of the configuration surface. The reason: a
// bookkeeping field that holds secrets would be an implicit
// widening of the surface, and the scope contract's intent is to
// prevent that.
//
// # Scope of the current application
//
// The scope tests apply the check to Config's own fields only, not
// to section members. Sensitive fields inside a section
// (Registry.Token, Registry.Password, Telemetry.APIKey) are
// intentionally on the surface and would match the substrings
// "token", "password", and "api_key". Whether the check should
// extend into sections with an explicit sensitive-field carve-out
// is left as future work; see ADR-022.
func IsForbiddenFieldName(name string) bool {
	lower := strings.ToLower(name)
	for _, sub := range forbiddenFieldNameSubstrings() {
		if strings.Contains(lower, sub) {
			return true
		}
	}
	return false
}

// InScopeFieldNames returns the names of the Config type's scalar
// value fields, in the order they appear in the struct.
//
// # Why the function exists
//
// The scope test checks that every exported field of the Config
// type is either a scalar value field (in this list) or a section
// (in InScopeSectionNames). A future contributor who adds a field
// without updating one of the two lists fails the test. The
// failure names the new field; the contributor either renames it
// to one of the allowed names or opens an ADR to extend the
// surface.
//
// # Why the list is authoritative
//
// The Config type's fields are the source of truth for the type's
// shape; the list here is the source of truth for the scope
// contract. The two are kept in sync by the test. A change to
// either without the other fails the test.
//
// # The SourcePath rename
//
// WBS 8.2.1 renamed the third field from ConfigPath to SourcePath.
// The new name is more general: a future loader might read from a
// URL or an environment variable, and SourcePath remains accurate
// where ConfigPath would not. The list below uses the new name.
//
// # Why a function, not a variable
//
// The CLI package's contract test forbids package-level
// variables. A function that returns a fresh slice on each call
// has no package-level state.
func InScopeFieldNames() []string {
	return []string{
		"LogLevel",
		"OutputFormat",
		"SourcePath",
	}
}

// InScopeSectionNames returns the names of the Config type's
// sections, in the order they appear in the struct.
//
// # What a section is
//
// A section is an exported field whose type is a pointer to a
// struct defined in this package. A section carries a named group
// of related configuration values; the field's name is the
// section's name, and the group's members are the pointer's
// target's fields (for example, Registry.URL and Registry.Token).
//
// Sections are part of the configuration surface. They are
// distinct from scalar value fields for two reasons:
//
//   - Their type is a pointer to a struct, not a string.
//   - Their members can carry per-field schema metadata
//     (sensitive: true), which the scalar value fields cannot.
//
// Both reasons are why configFieldCount does not count them.
// Adding a section still requires an ADR: it widens the
// configuration surface.
//
// # Why the function exists
//
// The scope test checks that every exported field of the Config
// type is either a scalar value field or a section. A future
// contributor who adds a section without updating this list fails
// the test. The failure names the new field; the contributor
// opens an ADR to add the name.
//
// # The WBS 12.7.1 sections
//
// Registry and Telemetry were added by WBS 12.7.1 and recognized
// as a category by ADR-022. Registry holds the registry endpoint
// (Registry.URL) and two sensitive members (Registry.Token,
// Registry.Password); Telemetry holds a telemetry endpoint
// (Telemetry.Endpoint) and one sensitive member
// (Telemetry.APIKey). A nil section pointer means the section was
// absent from the config file.
//
// # Order
//
// The names are returned in the order the fields appear in the
// struct. The order is not part of the contract; the test
// compares membership, not sequence.
//
// # Why a function, not a variable
//
// The CLI package's contract test forbids package-level
// variables. A function that returns a fresh slice on each call
// has no package-level state.
func InScopeSectionNames() []string {
	return []string{
		"Registry",
		"Telemetry",
	}
}

// InScopeBookkeepingFieldNames returns the names of the Config
// type's bookkeeping fields, in the order they appear in the
// struct.
//
// # What a bookkeeping field is
//
// A bookkeeping field is an unexported field that carries internal
// state about the value fields. It is not part of the
// configuration surface; a caller who reads the value fields
// never sees it. The distinction is documented in this file's
// package docstring and in ADR-009.
//
// # Why the function exists
//
// The scope test checks that every unexported field of the Config
// type is in this list. A future contributor who adds a
// bookkeeping field without updating the list fails the test. The
// failure names the new field; the contributor amends the scope
// contract (a small change, not a full ADR) to add the name.
//
// # Why not enforce the naming convention here
//
// A bookkeeping field's name is by convention descriptive of the
// value fields it describes ("sources" describes the source of
// each value). The convention is useful but not required: a
// future bookkeeping field with a different shape (for example,
// "file" for a field that records where a value was read from)
// would be permitted if the contract is amended to include it.
// The function returns the names verbatim; the test does not
// impose the convention.
//
// # Why a function, not a variable
//
// The CLI package's contract test forbids package-level
// variables. A function that returns a fresh slice on each call
// has no package-level state.
func InScopeBookkeepingFieldNames() []string {
	return []string{
		"sources",
	}
}
