// Package config defines the CLI configuration for Forge.
//
// This file holds the configuration error taxonomy (WBS 8.5.1).
// Every failure in the config package produces a *forgeerr.Error
// with category CategoryConfig. The code (E1..E8 in the
// taxonomy's numbering, FORGE_CONFIG_* in the registry) identifies
// the specific failure; the constructor that produced the error
// adds the context (the file path, the offending value, the
// allowed values) and the remediation.
//
// # The eight categories
//
//	E1  CodeConfigNotFound        --config points to a missing file
//	E2  CodeConfigPermission      file exists but cannot be read
//	E3  CodeConfigMalformed       invalid YAML syntax
//	E4  CodeConfigUnknownField    unknown YAML key
//	E5  CodeConfigInvalidType     type mismatch
//	E6  CodeConfigInvalidValue    value outside allowed enum
//	E7  CodeConfigTooLarge        file exceeds 1 MiB
//	E8  CodeConfigInvalidEncoding BOM or non-UTF-8
//
// The codes are documented in docs/cli-ux-spec.md "Config Error
// Reference" and in docs/reference/error-codes.md. Their exit
// code is ExitConfig (3) for all eight.
//
// # How the CLI classifies a config error
//
// The CLI's exitCodeOf reads the error's Category() method.
// Every config error's Category() returns CategoryConfig. The
// helper maps that category to CategoryConfig.ExitCode(), which
// is ExitConfig (3).
//
// The classification is done by category, not by code. The eight
// codes all map to ExitConfig (3); a future WBS that splits the
// exit codes (for example, a distinct code for a syntax error)
// changes the categories' ExitCode method, not the constructors.
//
// # The error's structure
//
// Each error the package produces is a *forgeerr.Error with:
//
//   - Category: CategoryConfig.
//   - Code: one of the eight FORGE_CONFIG_* codes.
//   - Message: a short fixed string naming the failure kind.
//   - Context: the varying data (path, field name, line number,
//     offending value, allowed values).
//   - Remediation: the actionable suggestion, ending with a
//     period (the frozen format's rule F6).
//   - Cause: the underlying error, if any, attached via WithCause.
//
// The split between the message and the context is deliberate.
// The message is short and stable; the context carries the
// varying data. The frozen user-facing format renders both:
//
//	Error: config file not found
//	Code:  FORGE_CONFIG_NOT_FOUND
//	       path: /etc/forge/config.yaml
//
// # Migration note (WBS 10.5.2)
//
// Before WBS 10.5.2, the constructors returned plain *configError
// values whose Category() method returned the string "config".
// The plain errors were the source of an incorrect exit code at
// the CLI boundary: the boundary's non-*forgeerr.Error wrapping
// path classified them as FORGE_INTERNAL_UNEXPECTED, whose
// category is CategoryInternal and whose exit code is
// ExitFailure (1), not ExitConfig (3).
//
// The migration replaces the plain errors with *forgeerr.Error
// values. The configError type is deleted; the Category() string
// method is no longer needed because the category is a
// forgeerr.Category and the *forgeerr.Error carries it.
//
// # What about the sentinels
//
// The eight sentinels (ErrConfigNotFound, ErrConfigPermission,
// ...) remain exported. Their type changes from *configError to
// *forgeerr.Error, but their role is unchanged: they are values
// that callers can match with errors.Is. The *forgeerr.Error's
// Is method compares codes, so errors.Is(err, ErrConfigNotFound)
// matches any error with code CodeConfigNotFound, regardless of
// the error's context or cause.
//
// # What about the Category() and Suggestion() methods
//
// The old *configError type had a Category() string method and a
// Suggestion() string method. Both are superseded:
//
//   - Category() string is replaced by Category() forgeerr.Category
//     on the *forgeerr.Error. The type is different; a caller that
//     asserted on the old string method must use errors.As with a
//     *forgeerr.Error target and read the category via the new
//     method.
//
//   - Suggestion() string is replaced by Remediation() string on
//     the *forgeerr.Error. The name changed because the frozen
//     format calls the field "remediation"; the concept is the
//     same. A caller that read the old method must read the new
//     one.
package config

import (
	"fmt"
	"strings"

	"github.com/thapelomagqazana/forge/internal/forgeerr"
)

// =============================================================================
// The eight sentinels
// =============================================================================
//
// Each sentinel is a *forgeerr.Error with the code set and the
// message and remediation filled in. The sentinel is used as a
// target for errors.Is; the constructors produce errors with the
// same code but with context and cause attached.
//
// The docstring on each sentinel records the code's meaning, the
// trigger, the message shape, and the remediation. The four
// together are the code's contract.

// ErrConfigNotFound is the sentinel for E1: the file the user
// asked for does not exist.
//
// # Trigger
//
// The --config flag or the FORGE_CONFIG environment variable
// points to a file that does not exist. The trigger is an
// explicit path that was given by the user; a path that
// config.Discover found on its own is not an E1 error (a missing
// discovered file is not an error; discovery's contract is to
// return "" when no file exists).
//
// # Message shape
//
// The constructor's message is "config file not found". The path
// is in the context under the key "path".
//
// # Remediation
//
// "Check the path passed to --config." (ends with a period, per
// rule F6 of the frozen format).
//
// # Exit code
//
// ExitConfig (3), via CategoryConfig.ExitCode().
var ErrConfigNotFound = forgeerr.New(
	forgeerr.CategoryConfig,
	string(forgeerr.CodeConfigNotFound),
	"config file not found",
)

// ErrConfigPermission is the sentinel for E2: the file exists but
// cannot be read.
//
// # Trigger
//
// The file exists (os.Stat succeeds) but os.ReadFile returns a
// permission error. The cause is the underlying *os.PathError;
// a caller can check errors.Is(err, os.ErrPermission) to
// distinguish a permission failure from a generic read failure.
//
// # Message shape
//
// The constructor's message is "cannot read config file". The
// path is in the context under the key "path".
//
// # Remediation
//
// "Check the file's permissions." (ends with a period).
//
// # Exit code
//
// ExitConfig (3).
var ErrConfigPermission = forgeerr.New(
	forgeerr.CategoryConfig,
	string(forgeerr.CodeConfigPermission),
	"cannot read config file",
)

// ErrConfigMalformed is the sentinel for E3: the file is not
// valid YAML.
//
// # Trigger
//
// The YAML parser returns a syntax error. The cause is the
// parser's error; the constructor extracts the line and column
// from it if the parser provides them.
//
// # Message shape
//
// The constructor's message is "invalid YAML". The path, line,
// and column are in the context under the keys "path", "line",
// and "column" (the latter two as decimal strings).
//
// # Remediation
//
// "Fix the YAML syntax at line <N>." when the line is known, or
// "Fix the YAML syntax." when it is not.
//
// # Exit code
//
// ExitConfig (3).
var ErrConfigMalformed = forgeerr.New(
	forgeerr.CategoryConfig,
	string(forgeerr.CodeConfigMalformed),
	"invalid YAML",
)

// ErrConfigUnknownField is the sentinel for E4: the YAML
// contains a key the schema does not declare.
//
// # Trigger
//
// The YAML decoder, configured with strict field matching,
// returns an "unknown field" error. The cause is the decoder's
// error; the constructor extracts the field name from it.
//
// # Message shape
//
// The constructor's message is "unknown field". The path and
// field name are in the context under the keys "path" and
// "field".
//
// # Remediation
//
// "Remove the field or check for typos." (ends with a period).
//
// # Exit code
//
// ExitConfig (3).
var ErrConfigUnknownField = forgeerr.New(
	forgeerr.CategoryConfig,
	string(forgeerr.CodeConfigUnknownField),
	"unknown field",
)

// ErrConfigInvalidType is the sentinel for E5: a field has a
// value of the wrong type.
//
// # Trigger
//
// The YAML decoder returns a type-mismatch error (for example, a
// string where an integer was expected). The cause is the
// decoder's error.
//
// # Message shape
//
// The constructor's message is "invalid type". The path and
// field name are in the context under the keys "path" and
// "field".
//
// # Remediation
//
// "Check the field's type." (ends with a period).
//
// # Exit code
//
// ExitConfig (3).
var ErrConfigInvalidType = forgeerr.New(
	forgeerr.CategoryConfig,
	string(forgeerr.CodeConfigInvalidType),
	"invalid type",
)

// ErrConfigInvalid is the sentinel for E6: a field's value is
// outside the allowed set (typically an enum).
//
// # Trigger
//
// The value is syntactically valid but semantically rejected.
// For example, log_level: verbose where the allowed values are
// debug|info|warn|error. The constructor names the field, the
// invalid value, and the allowed values.
//
// # Message shape
//
// The constructor's message is "invalid value". The field, the
// offending value, and the allowed values are in the context
// under the keys "field", "value", and "allowed". The path, if
// known, is in the context under the key "path".
//
// # Remediation
//
// "Allowed values: <list>." (ends with a period).
//
// # Exit code
//
// ExitConfig (3).
//
// # Why the sentinel is named ErrConfigInvalid and not
// # ErrConfigInvalidValue
//
// The sentinel predates WBS 8.5.1: it was introduced by WBS
// 8.4.4 for the flag-resolution errors. WBS 8.5.1 adopts it as
// the E6 sentinel without renaming. The code it carries is
// CodeConfigInvalidValue, which is the registry's name for E6.
var ErrConfigInvalid = forgeerr.New(
	forgeerr.CategoryConfig,
	string(forgeerr.CodeConfigInvalidValue),
	"invalid value",
)

// ErrConfigTooLarge is the sentinel for E7: the file exceeds the
// maximum size.
//
// # Trigger
//
// The file's size (os.Stat's Size) exceeds the maximum. The
// check is done before os.ReadFile to avoid reading a large
// file into memory.
//
// # Message shape
//
// The constructor's message is "config file too large". The path
// is in the context under the key "path".
//
// # Remediation
//
// "Config files must be under 1 MiB." (ends with a period).
//
// # Exit code
//
// ExitConfig (3).
var ErrConfigTooLarge = forgeerr.New(
	forgeerr.CategoryConfig,
	string(forgeerr.CodeConfigTooLarge),
	"config file too large",
)

// ErrConfigInvalidEncoding is the sentinel for E8: the file is
// not valid UTF-8, or it starts with a UTF-8 BOM.
//
// # Trigger
//
// The file's first three bytes are the UTF-8 BOM (0xEF, 0xBB,
// 0xBF), or the file contains a byte sequence that is not valid
// UTF-8. The check is done on the raw bytes.
//
// # Message shape
//
// The constructor's message is "invalid encoding". The path is
// in the context under the key "path".
//
// # Remediation
//
// "Ensure the file is UTF-8 encoded without BOM." (ends with a
// period).
//
// # Exit code
//
// ExitConfig (3).
var ErrConfigInvalidEncoding = forgeerr.New(
	forgeerr.CategoryConfig,
	string(forgeerr.CodeConfigInvalidEncoding),
	"invalid encoding",
)

// =============================================================================
// The constructors
// =============================================================================
//
// Each constructor returns a *forgeerr.Error with the code's
// sentinel's code, a short fixed message, the varying data in
// the context, and the code's remediation. Constructors that
// receive a cause attach it via WithCause.
//
// # Why the constructors are not variadic for the cause
//
// The old constructors accepted a cause argument for some codes
// and not others. The new constructors follow the same shape:
// where a cause is meaningful (a permission error, a YAML parse
// error, a strict-decode error), the constructor takes it;
// where it is not (not-found, too-large, invalid-encoding), the
// constructor does not. The asymmetry matches the reality: some
// failures have an underlying cause and some do not.

// NewConfigNotFound constructs an E1 error for the given path.
//
// The path is the file the user asked for. The constructor does
// not check whether the file exists; the caller (config.Load or
// config.Discover) performs the check and calls the constructor
// when the file is missing.
func NewConfigNotFound(path string) error {
	// ErrConfigNotFound is the sentinel; the returned error
	// shares its code but adds the path to the context.
	//
	// The context is added via WithContext, which returns a
	// copy of the sentinel with the new key set. The sentinel
	// itself is not mutated; a caller that holds
	// ErrConfigNotFound sees the unmodified sentinel.
	return ErrConfigNotFound.
		WithContext("path", path)
}

// NewConfigPermission constructs an E2 error for the given path
// and cause.
//
// The cause is the underlying *os.PathError from os.ReadFile.
// The constructor attaches the cause so that
// errors.Is(err, os.ErrPermission) works.
func NewConfigPermission(path string, cause error) error {
	e := ErrConfigPermission.WithContext("path", path)
	if cause != nil {
		e = e.WithCause(cause)
	}
	return e
}

// NewConfigMalformed constructs an E3 error for the given path,
// line, and column.
//
// The line and column are the location of the syntax error, as
// reported by the YAML parser. A parser that does not report a
// line uses 0; the constructor then omits the line and column
// from the context.
func NewConfigMalformed(path string, line, col int, cause error) error {
	e := ErrConfigMalformed.WithContext("path", path)
	if line > 0 {
		e = e.WithContext("line", fmt.Sprintf("%d", line))
	}
	if col > 0 {
		e = e.WithContext("column", fmt.Sprintf("%d", col))
	}

	// The remediation names the line when it is known; the
	// generic form is used otherwise.
	var remediation string
	if line > 0 {
		remediation = fmt.Sprintf("Fix the YAML syntax at line %d.", line)
	} else {
		remediation = "Fix the YAML syntax."
	}
	e = e.WithRemediation(remediation)

	if cause != nil {
		e = e.WithCause(cause)
	}
	return e
}

// NewConfigUnknownField constructs an E4 error for the given
// path and field name.
func NewConfigUnknownField(path, field string, cause error) error {
	e := ErrConfigUnknownField.
		WithContext("path", path).
		WithContext("field", field).
		WithRemediation("Remove the field or check for typos.")
	if cause != nil {
		e = e.WithCause(cause)
	}
	return e
}

// NewConfigInvalidType constructs an E5 error for the given
// path and field name.
func NewConfigInvalidType(path, field string, cause error) error {
	e := ErrConfigInvalidType.
		WithContext("path", path).
		WithContext("field", field).
		WithRemediation("Check the field's type.")
	if cause != nil {
		e = e.WithCause(cause)
	}
	return e
}

// NewConfigInvalid constructs an E6 error for the given field,
// invalid value, and allowed values.
//
// # Why the path is optional
//
// E6 errors arise from two sources: a value in the config file,
// and a value in a CLI flag or environment variable. The first
// has a path; the second does not. The constructor takes an
// optional path; when the path is empty or omitted, the context
// does not include a "path" key.
//
// The variadic signature preserves the call shape the existing
// callers from WBS 8.4.4 use; the callers pass one path or none.
func NewConfigInvalid(field, value string, allowed []string, path ...string) error {
	e := ErrConfigInvalid.
		WithContext("field", field).
		WithContext("value", value).
		WithContext("allowed", fmt.Sprintf("%v", allowed)).
		WithRemediation(fmt.Sprintf("Allowed values: %s.", joinAllowed(allowed)))
	if len(path) > 0 && path[0] != "" {
		e = e.WithContext("path", path[0])
	}
	return e
}

// NewConfigTooLarge constructs an E7 error for the given path.
func NewConfigTooLarge(path string) error {
	return ErrConfigTooLarge.
		WithContext("path", path).
		WithRemediation("Config files must be under 1 MiB.")
}

// NewConfigInvalidEncoding constructs an E8 error for the given
// path.
func NewConfigInvalidEncoding(path string) error {
	return ErrConfigInvalidEncoding.
		WithContext("path", path).
		WithRemediation("Ensure the file is UTF-8 encoded without BOM.")
}

// =============================================================================
// Helpers
// =============================================================================

// joinAllowed renders a slice of allowed values as a
// comma-separated string for the E6 remediation.
//
// # Why a helper and not strings.Join directly
//
// The remediation is a single sentence ending with a period
// (rule F6 of the frozen format). The list of allowed values is
// rendered inside the sentence. strings.Join's output does not
// include the list's delimiters; a caller that wants a
// human-friendly rendering (for example, "debug, info, warn, or
// error") uses this helper. The current implementation uses a
// simple comma-separated list; a future change that adds an
// "or" before the last item is localized to this function.
func joinAllowed(allowed []string) string {
	return strings.Join(allowed, ", ")
}
