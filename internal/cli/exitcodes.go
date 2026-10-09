package cli

// Exit codes returned by Execute().
//
// The codes are stable across releases. Scripts and CI pipelines may
// depend on their values. Changing a code is a breaking change and
// requires an ADR.
//
// The codes follow the convention used by most UNIX tools:
//
//	0 — success
//	1 — general failure (the error was unexpected or uncategorised)
//	2 — usage error (bad arguments, unknown command, unknown flag)
//	>2 — category-specific failures
//
// Codes 3 through 6 are reserved for error categories that will be
// introduced in later phases (config, filesystem, validation, security).
// They are declared here so that the mapping is centralised from the
// start, even before the categories are fully populated.
const (
	// ExitSuccess indicates the command completed successfully.
	ExitSuccess = 0

	// ExitFailure indicates a general failure that does not fall into
	// a more specific category. It is the default for unclassified
	// errors.
	ExitFailure = 1

	// ExitUsage indicates that the command was invoked incorrectly:
	// an unknown command, an unknown flag, or a malformed argument.
	// This is the code that Cobra's own usage errors map to.
	ExitUsage = 2

	// ExitConfig indicates a configuration loading or validation
	// failure. Reserved for WBS 8.x.
	ExitConfig = 3

	// ExitFilesystem indicates a filesystem operation failure.
	// Reserved for WBS 13.x.
	ExitFilesystem = 4

	// ExitValidation indicates a validation failure from a check or
	// diff operation. Reserved for WBS 7.x and WBS 9.x.
	ExitValidation = 5

	// ExitSecurity indicates a security boundary violation (path
	// traversal, symlink escape, secret leak). Reserved for WBS 13.x.
	ExitSecurity = 6
)

// exitCodeFromError maps a Go error returned by the command tree to a
// process exit code.
//
// The mapping is deliberately simple in WBS 2.4.2. When the structured
// error model arrives in WBS 10.0, this function will be extended to
// inspect the error's category and return the corresponding code. For
// now, every error is treated as a usage error except when it is nil.
//
// The function is unexported and lives in this file so that the mapping
// is the single source of truth. No other file derives exit codes from
// errors.
func exitCodeFromError(err error) int {
	if err == nil {
		return ExitSuccess
	}
	// In WBS 2.4.2, all command errors are treated as usage errors.
	// Cobra returns an error for unknown commands and unknown flags,
	// both of which warrant ExitUsage. When the structured error
	// model lands in WBS 10.0, this function will be replaced by a
	// switch on the error's category.
	return ExitUsage
}