// Package cli — process exit codes.
//
// This file defines the process exit codes returned by the CLI.
// The values are frozen in WBS 4.1.2 and documented in
// docs/cli-ux-spec.md § 7.
//
// # The mapping
//
// The exit code for an error is the error's Category().ExitCode().
// The method is defined in internal/forgeerr/category.go; it is
// the single source of truth. This file does not implement the
// mapping; it declares the constants that the categories' exit
// codes correspond to.
//
// The two definitions must agree. The test
// TestAC4_ExitCodesMatchCLIConstants in
// internal/forgeerr/category_test.go polices the numeric
// agreement. If WBS 4.1.2 changes, the test fails and both the
// constants and the switch in Category.ExitCode are updated in
// the same commit.
//
// # Why the constants are duplicated
//
// The constants live in internal/cli (not in internal/forgeerr)
// so that internal/forgeerr does not depend on internal/cli. The
// dependency goes one way: cli imports forgeerr.
//
// # Stability
//
// A constant, once shipped in a release, is never renumbered. If
// a new failure category is introduced, a new constant is added
// at the next available integer. Existing codes retain their
// meaning.
//
// This is the same stability rule that applies to error codes in
// internal/forgeerr (WBS 10.3.1). The two rules are consistent:
// exit codes and error codes both change by addition, never by
// renumbering.
//
// # The removed mapping
//
// Before WBS 10.5.1, this file also defined a CategorizedError
// interface, an errorsAs wrapper, and an exitCodeFromError
// function that mapped a category string to an exit code. All
// three are superseded:
//
//   - CategorizedError was replaced by forgeerr.Category, the
//     closed taxonomy type (WBS 10.2.1).
//   - errorsAs was replaced by direct use of errors.As on
//     *forgeerr.Error.
//   - exitCodeFromError was replaced by Category.ExitCode(), the
//     category's method (WBS 10.2.1).
//
// The boundary's local helper exitCodeOf (in execute.go) performs
// the errors.As extraction and reads the category's method.
package cli

// Exit codes. Values are frozen in WBS 4.1.2.
const (
	// ExitSuccess indicates the command completed successfully.
	//
	// This is the only non-error exit code. Every other code
	// indicates some form of failure.
	ExitSuccess = 0

	// ExitFailure indicates a general failure that does not fall
	// into a more specific category. It is the default for
	// unclassified errors.
	//
	// The TEMPLATE and INTERNAL categories use it. When this code
	// is returned, the error message should be informative enough
	// for the user to determine the cause without further
	// diagnostics.
	ExitFailure = 1

	// ExitUsage indicates that the command was invoked
	// incorrectly: an unknown command, an unknown flag, a
	// malformed argument, or a malformed invocation rejected by
	// validateArgs.
	//
	// The CLI category uses it.
	ExitUsage = 2

	// ExitConfig indicates a configuration loading or validation
	// failure: file not found, file unreadable, malformed YAML,
	// unknown field, invalid value.
	//
	// The CONFIG category uses it.
	ExitConfig = 3

	// ExitFilesystem indicates a filesystem operation failure:
	// permission denied, path not found, write failure.
	//
	// The FILESYSTEM category uses it.
	ExitFilesystem = 4

	// ExitValidation indicates a validation failure from a check
	// or diff operation.
	//
	// The VALIDATION category uses it.
	ExitValidation = 5

	// ExitSecurity indicates a security boundary violation: a
	// path traversal attempt, a symlink escape, or a secret leak.
	//
	// The SECURITY category uses it.
	ExitSecurity = 6

	// ExitConflict indicates that an update could not be applied
	// without overwriting developer changes.
	//
	// The CONFLICT category is reserved for a future phase. The
	// constant exists now, before the subsystem is implemented, so
	// that the update engine can be written against it from the
	// start.
	ExitConflict = 7
)
