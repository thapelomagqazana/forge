package cli

import "errors"

// Exit codes returned by Execute().
//
// The codes are the CLI's contract with the outside world. Scripts,
// CI pipelines, and shell operators depend on them. Changing a code
// is a breaking change: it must be recorded in an ADR and referenced
// from docs/cli-ux-spec.md.
//
// The codes follow the convention used by most UNIX tools:
//
//	0 — success
//	1 — general failure (the error was unexpected or uncategorised)
//	2 — usage error (bad arguments, unknown command, unknown flag)
//	>2 — category-specific failures
//
// The specific mapping from errors to codes is defined in
// exitCodeFromError, below.
//
// # Stability
//
// A constant, once shipped in a release, is never renumbered. If a
// new failure category is introduced, a new constant is added at
// the next available integer. Existing codes retain their meaning.
//
// This is the same stability rule that applies to error codes in
// internal/forgeerr (WBS 10.0). The two rules are consistent: exit
// codes and error codes both change by addition, never by
// renumbering.
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
	// When this code is returned, the error message should be
	// informative enough for the user to determine the cause
	// without further diagnostics.
	ExitFailure = 1

	// ExitUsage indicates that the command was invoked incorrectly:
	// an unknown command, an unknown flag, or a malformed argument.
	//
	// This is the code that Cobra's own usage errors map to. It is
	// also the code returned for any error the CLI cannot classify
	// into a more specific category at the point where the error
	// is raised.
	ExitUsage = 2

	// ExitConfig indicates a configuration loading or validation
	// failure. Reserved for WBS 8.x, when the configuration
	// subsystem is introduced.
	//
	// The error categories that map to this code are defined by
	// the configuration subsystem.
	ExitConfig = 3

	// ExitFilesystem indicates a filesystem operation failure.
	// Reserved for WBS 13.x, when the filesystem abstraction is
	// introduced.
	//
	// The error categories that map to this code are defined by
	// the filesystem subsystem.
	ExitFilesystem = 4

	// ExitValidation indicates a validation failure from a check or
	// diff operation. Reserved for WBS 7.x and WBS 9.x.
	//
	// The error categories that map to this code are defined by
	// the validation and drift subsystems.
	ExitValidation = 5

	// ExitSecurity indicates a security boundary violation: a path
	// traversal attempt, a symlink escape, or a secret leak.
	// Reserved for WBS 13.x.
	//
	// The error categories that map to this code are defined by
	// the security subsystem.
	ExitSecurity = 6

	// ExitConflict indicates that an update could not be applied
	// without overwriting developer changes. Reserved for WBS 14.x,
	// when the update engine is introduced.
	//
	// The error categories that map to this code are defined by
	// the update subsystem. The code exists now, before the
	// subsystem is implemented, so that the update engine can be
	// written against the constant from the start.
	ExitConflict = 7
)

// CategorizedError is implemented by any error that carries an
// exit-code category.
//
// The interface is deliberately minimal. It has one method, and that
// method returns a string. The exit code layer does not need to know
// anything else about the error.
//
// # Why an interface
//
// WBS 4.1.2 exists before the structured error model in WBS 10.0.
// Rather than couple the exit code layer to a specific error type
// that does not yet exist, the layer uses an interface. Any error
// that implements CategorizedError participates in the mapping.
//
// The structured error model in WBS 10.0 will implement this
// interface. So will any custom error types that future phases
// introduce. The exit code layer does not need to change when a new
// error type is added.
//
// # Category values
//
// The Category method returns a string. The set of valid strings is
// open: new categories may be introduced by any package. The exit
// code layer recognises the categories it knows about and treats
// every other category as ExitFailure.
//
// The recognised categories are documented in exitCodeFromError.
type CategorizedError interface {
	error

	// Category returns a short, stable identifier for the error's
	// category. The identifier is used by exitCodeFromError to
	// select an exit code.
	//
	// The value must be lowercase, must not contain whitespace, and
	// must be stable across releases.
	//
	// Examples: "config", "filesystem", "validation", "security".
	Category() string
}

// errorsAs is a small wrapper around errors.As for the
// CategorizedError interface.
//
// It exists because errors.As requires a pointer to the target
// type, and the target type is an interface. The wrapper encapsulates
// the pointer indirection, so exitCodeFromError reads cleanly.
func errorsAs(err error, target *CategorizedError) bool {
	return errors.As(err, target)
}

// exitCodeFromError maps a Go error to a process exit code.
//
// It is the only function in the package that derives an exit code
// from an error. Every other file refers to the constants by name;
// none of them performs the mapping.
//
// The mapping is:
//
//	nil error                 → ExitSuccess
//	category "config"         → ExitConfig
//	category "filesystem"     → ExitFilesystem
//	category "validation"     → ExitValidation
//	category "security"       → ExitSecurity
//	any other error           → ExitUsage
//
// # Why "any other error" maps to ExitUsage
//
// Cobra returns errors for unknown commands and unknown flags. Those
// errors do not implement CategorizedError. They are, by definition,
// usage errors. Mapping unclassified errors to ExitUsage ensures
// that Cobra's errors reach the user with the correct exit code
// without special-casing them in the exit code layer.
//
// # Why an empty interface is not used
//
// The mapping could have been implemented with a type switch on
// concrete error types. That approach would require editing
// exitcodes.go every time a new error type is introduced. The
// interface approach decouples the two: a new error type is added
// by defining its Category method, and the exit code layer is not
// touched.
//
// # The function is pure
//
// exitCodeFromError does not log, does not read environment
// variables, and does not perform I/O. Given the same error, it
// always returns the same code. This property is tested in
// exitcodes_test.go.
func exitCodeFromError(err error) int {
	if err == nil {
		return ExitSuccess
	}

	var cat CategorizedError
	if !errors.As(err, &cat) {
		return ExitUsage
	}

	switch cat.Category() {
	case "config":
		return ExitConfig
	case "filesystem":
		return ExitFilesystem
	case "validation":
		return ExitValidation
	case "security":
		return ExitSecurity
	case "conflict":
		return ExitConflict
	default:
		return ExitFailure
	}
}
