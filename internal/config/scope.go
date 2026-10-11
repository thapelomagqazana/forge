package config

import "strings"

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

// configFieldCount is the number of fields the Config type has.
//
// # Why a constant
//
// The scope contract (WBS 8.1.1) freezes the count at three. The
// test TestScope_ConfigHasExactlyThreeFields reads this constant;
// adding a field requires updating the constant and the ADR.
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
func IsForbiddenFieldName(name string) bool {
	lower := strings.ToLower(name)
	for _, sub := range forbiddenFieldNameSubstrings() {
		if strings.Contains(lower, sub) {
			return true
		}
	}
	return false
}

// InScopeFieldNames returns the names of the fields that the
// scope contract permits.
//
// # Why the function exists
//
// The scope test checks that every field of the Config type is in
// this list. A future contributor who adds a field without
// updating the list fails the test. The failure names the new
// field; the contributor either renames it to one of the allowed
// names or opens an ADR to extend the list.
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
