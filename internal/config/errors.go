// Package config defines the CLI configuration for Forge.
//
// This file holds the configuration error taxonomy (WBS 8.5.1).
// Every failure in the config package produces an error whose
// sentinel is one of the eight values declared here. The sentinel
// identifies the category; the constructor that produced the error
// adds the context (the file path, the offending value, the allowed
// values) and the suggestion.
//
// # The eight categories
//
//	E1  ErrConfigNotFound          --config points to a missing file
//	E2  ErrConfigPermission        file exists but cannot be read
//	E3  ErrConfigMalformed         invalid YAML syntax
//	E4  ErrConfigUnknownField      unknown YAML key
//	E5  ErrConfigInvalidType       type mismatch
//	E6  ErrConfigInvalid           value outside allowed enum
//	E7  ErrConfigTooLarge          file exceeds 1 MiB
//	E8  ErrConfigInvalidEncoding   BOM or non-UTF-8
//
// The codes are documented in docs/cli-ux-spec.md "Config Error
// Reference". Their exit code is ExitConfig (3) for all eight.
//
// # How the CLI classifies a config error
//
// The CLI's exitCodeFromError reads the error's Category() method.
// Every sentinel's Category() returns "config". The classifier maps
// that category to ExitConfig (3).
//
// The classification is done by category, not by code. The eight
// codes all map to ExitConfig (3); a future WBS that splits the
// exit codes (for example, a distinct code for a syntax error)
// changes the classifier's mapping, not the sentinels.
//
// # The error's structure
//
// Each error the package produces has:
//
//   - A sentinel (the category). Reachable via errors.Is.
//   - A message. The message names the file path and the
//     offending element (where applicable). It does not end with
//     a period; the CLI's formatter adds the "Error: " prefix.
//   - A suggestion. The suggestion is the actionable remediation.
//     It is read via the Suggestion() method, which the CLI's
//     formatter uses to render the "Suggestion:" block.
//
// The message and the suggestion are separate concerns: the
// message says what happened; the suggestion says what to do. A
// future renderer (a JSON emitter, a log formatter) reads them
// separately.
//
// # No package-level mutable state
//
// The sentinels declared here are package-level values. The config
// package's contract test does not forbid them: the sentinels are
// the package's public API, not accidental state. Each sentinel is
// a pointer to a configError value whose fields are set once at
// declaration and never mutated. The values are safe for
// concurrent reads.
//
// A future WBS that wants to make the sentinels values instead of
// pointers can do so; the errors.Is check works either way because
// the type's Is method does the comparison.
package config

import (
	"errors"
	"fmt"
	"strings"
)

// =============================================================================
// The error type
// =============================================================================

// configError is the type of every error the config package
// produces.
//
// # Why a type and not a plain errors.New
//
// The type carries four pieces of information:
//
//   - code: the E1..E8 identifier.
//   - msg: the human-readable message.
//   - suggestion: the actionable remediation.
//   - cause: the underlying error, if any (for example, the
//     os.ReadFile error that triggered E2).
//
// A plain errors.New carries only the message. The type is the
// minimum structure that supports errors.Is (via the Is method),
// the Category() classification, and the Suggestion() accessor.
//
// # Why the fields are unexported
//
// The error is a value object; its fields are read by the
// package's methods, not by callers. Callers use errors.Is,
// Category(), and Suggestion(). Exposing the fields would invite
// callers to depend on the internal representation.
//
// # Immutability
//
// A configError's fields are set once, by the constructor that
// produced it, and never mutated. A caller that holds a
// configError can read its fields through the methods; a caller
// that holds an error interface can check errors.Is and read
// Category() and Suggestion().
type configError struct {
	code       string
	msg        string
	suggestion string
	cause      error
}

// Error implements the error interface. The message does not end
// with a period; the CLI's formatter adds the "Error: " prefix.
func (e *configError) Error() string {
	return e.msg
}

// Unwrap returns the cause, or nil if there is none.
//
// The Unwrap method is what lets errors.Is and errors.As see
// through the error to its cause. A caller that wants to
// distinguish "permission denied" from "file not found" checks
// the cause's errors.Is against os.ErrPermission.
func (e *configError) Unwrap() error {
	return e.cause
}

// Is reports whether target is the same sentinel as e's code.
//
// # How the comparison works
//
// errors.Is calls the receiver's Is method (this method) with
// each error in the target's chain. The method compares the
// target's code to e's code. Two errors with the same code are
// considered the same error for the purposes of errors.Is, even
// if their messages or suggestions differ.
//
// # Why the comparison is on the code
//
// The code is the identity of the category. Two E2 errors, one
// for /home/user/forge.yaml and one for /etc/forge.yaml, are
// both E2 errors; a caller that writes
// errors.Is(err, ErrConfigPermission) wants to match both. The
// code is the right comparison key.
//
// # The target's type
//
// The target is expected to be a *configError. A target of a
// different type is not matched by this method; errors.Is
// continues to the next error in the chain or returns false.
func (e *configError) Is(target error) bool {
	t, ok := target.(*configError)
	if !ok {
		return false
	}
	return t.code == e.code
}

// Category returns the error's category. Every config error
// returns "config".
//
// # Why the method exists
//
// The CLI's exitCodeFromError classifies errors by category. A
// single category for every config error keeps the classifier
// simple: one case handles all eight codes. A future WBS that
// splits the exit codes (for example, a distinct code for a
// syntax error) changes this method to return different strings
// per sentinel.
//
// # Why the value is a constant-like string
//
// The string "config" is the category name used by the CLI UX
// spec's exit-code table (docs/cli-ux-spec.md § 7.2). Changing
// it is a change to that table; the two are updated in the same
// commit.
func (e *configError) Category() string {
	return "config"
}

// Suggestion returns the error's suggestion.
//
// # What the suggestion is
//
// The suggestion is the actionable remediation: what the user
// should do to fix the error. It is read by the CLI's error
// formatter (WBS 7.3.2), which renders it in the "Suggestion:"
// block.
//
// # Why the suggestion is not part of the message
//
// The message says what happened; the suggestion says what to
// do. A renderer that wants to show only the message (for
// example, a log line) reads Error(). A renderer that wants to
// show the suggestion (for example, the CLI's error formatter)
// reads Suggestion(). The two are separate; the separation
// lets each renderer choose.
func (e *configError) Suggestion() string {
	return e.suggestion
}

// Code returns the error's E1..E8 code. It is used by tests and
// by a future structured-error model (WBS 10.0).
func (e *configError) Code() string {
	return e.code
}

// =============================================================================
// The eight sentinels
// =============================================================================
//
// Each sentinel is a *configError with the code set and the
// message and suggestion empty. The sentinel is used only as a
// target for errors.Is; the message and suggestion are added by
// the constructors.
//
// The docstring on each sentinel records the category's trigger,
// the message format, and the suggestion. The three together are
// the category's contract.

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
// # Message format
//
//	config file not found: <path>
//
// # Suggestion
//
//	Check the path passed to --config.
//
// # Exit code
//
// ExitConfig (3).
var ErrConfigNotFound = &configError{
	code: "E1",
	msg:  "config file not found",
}

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
// # Message format
//
//	cannot read config file: <path>
//
// # Suggestion
//
//	Check file permissions: chmod 644 <path>
//
// # Exit code
//
// ExitConfig (3).
var ErrConfigPermission = &configError{
	code: "E2",
	msg:  "cannot read config file",
}

// ErrConfigMalformed is the sentinel for E3: the file is not
// valid YAML.
//
// # Trigger
//
// The YAML parser returns a syntax error. The cause is the
// parser's error; the constructor extracts the line and column
// from it if the parser provides them.
//
// # Message format
//
//	invalid YAML in <path>: <line>:<col>
//
// # Suggestion
//
//	Fix the YAML syntax at line <N>.
//
// # Exit code
//
// ExitConfig (3).
var ErrConfigMalformed = &configError{
	code: "E3",
	msg:  "invalid YAML",
}

// ErrConfigUnknownField is the sentinel for E4: the YAML
// contains a key the schema does not declare.
//
// # Trigger
//
// The YAML decoder, configured with strict field matching,
// returns an "unknown field" error. The cause is the decoder's
// error; the constructor extracts the field name from it.
//
// # Message format
//
//	unknown field <name> in <path>
//
// # Suggestion
//
//	Remove field '<name>' or check for typos.
//
// # Exit code
//
// ExitConfig (3).
var ErrConfigUnknownField = &configError{
	code: "E4",
	msg:  "unknown field",
}

// ErrConfigInvalidType is the sentinel for E5: a field has a
// value of the wrong type.
//
// # Trigger
//
// The YAML decoder returns a type-mismatch error (for example,
// a string where an integer was expected). The cause is the
// decoder's error.
//
// # Message format
//
//	invalid type for <field> in <path>
//
// # Suggestion
//
//	Check the type of '<field>' in <path>.
//
// # Exit code
//
// ExitConfig (3).
var ErrConfigInvalidType = &configError{
	code: "E5",
	msg:  "invalid type",
}

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
// # Message format
//
//	invalid value for <field>: <value>
//
// The constructor that produced the error may append the
// allowed values to the message; the CLI UX spec's example
// message format omits them and puts them in the suggestion.
//
// # Suggestion
//
//	Allowed values: <list>.
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
// the E6 sentinel without renaming. A future WBS that wants a
// distinct name can rename it; the rename is a coordinated
// change across the config package and the CLI's classifier.
var ErrConfigInvalid = &configError{
	code: "E6",
	msg:  "invalid value",
}

// ErrConfigTooLarge is the sentinel for E7: the file exceeds the
// maximum size.
//
// # Trigger
//
// The file's size (os.Stat's Size) exceeds the maximum. The
// check is done before os.ReadFile to avoid reading a large
// file into memory.
//
// # Message format
//
//	config file too large: <path>
//
// # Suggestion
//
//	Config files must be under 1 MiB.
//
// # Exit code
//
// ExitConfig (3).
var ErrConfigTooLarge = &configError{
	code: "E7",
	msg:  "config file too large",
}

// ErrConfigInvalidEncoding is the sentinel for E8: the file is
// not valid UTF-8, or it starts with a UTF-8 BOM.
//
// # Trigger
//
// The file's first three bytes are the UTF-8 BOM (0xEF, 0xBB,
// 0xBF), or the file contains a byte sequence that is not valid
// UTF-8. The check is done on the raw bytes.
//
// # Message format
//
//	invalid encoding in <path>
//
// # Suggestion
//
//	Ensure the file is UTF-8 encoded without BOM.
//
// # Exit code
//
// ExitConfig (3).
var ErrConfigInvalidEncoding = &configError{
	code: "E8",
	msg:  "invalid encoding",
}

// =============================================================================
// The constructors
// =============================================================================
//
// Each constructor returns an error whose sentinel is the
// category's sentinel, whose message is the formatted message,
// and whose suggestion is the category's suggestion. The
// constructors wrap their errors with fmt.Errorf and %w so that
// errors.Is reaches the sentinel.

// NewConfigNotFound constructs an E1 error for the given path.
//
// The path is the file the user asked for. The constructor does
// not check whether the file exists; the caller (config.Load or
// config.Discover) performs the check and calls the constructor
// when the file is missing.
func NewConfigNotFound(path string) error {
	return &configError{
		code:       ErrConfigNotFound.code,
		msg:        fmt.Sprintf("config: %s: not found", path),
		suggestion: "Check the path passed to --config.",
	}
}

// NewConfigPermission constructs an E2 error for the given path
// and cause.
//
// The cause is the underlying *os.PathError from os.ReadFile.
// The constructor wraps the cause so that errors.Is(err,
// os.ErrPermission) works.
func NewConfigPermission(path string, cause error) error {
	return &configError{
		code:       ErrConfigPermission.code,
		msg:        fmt.Sprintf("%s: %s", ErrConfigPermission.msg, path),
		suggestion: fmt.Sprintf("Check file permissions: chmod 644 %s", path),
		cause:      cause,
	}
}

// NewConfigMalformed constructs an E3 error for the given path,
// line, and column.
//
// The line and column are the location of the syntax error, as
// reported by the YAML parser. A parser that does not report a
// line uses 0; the constructor then omits the line and column
// from the message.
func NewConfigMalformed(path string, line, col int, cause error) error {
	var location string
	switch {
	case line > 0 && col > 0:
		location = fmt.Sprintf(":%d:%d", line, col)
	case line > 0:
		location = fmt.Sprintf(":%d", line)
	}
	var suggestion string
	if line > 0 {
		suggestion = fmt.Sprintf("Fix the YAML syntax at line %d.", line)
	} else {
		suggestion = "Fix the YAML syntax."
	}
	return &configError{
		code:       ErrConfigMalformed.code,
		msg:        fmt.Sprintf("%s in %s%s", ErrConfigMalformed.msg, path, location),
		suggestion: suggestion,
		cause:      cause,
	}
}

// NewConfigUnknownField constructs an E4 error for the given
// path and field name.
func NewConfigUnknownField(path, field string, cause error) error {
	return &configError{
		code:       ErrConfigUnknownField.code,
		msg:        fmt.Sprintf("%s %s in %s", ErrConfigUnknownField.msg, field, path),
		suggestion: fmt.Sprintf("Remove field '%s' or check for typos.", field),
		cause:      cause,
	}
}

// NewConfigInvalidType constructs an E5 error for the given
// path and field name.
func NewConfigInvalidType(path, field string, cause error) error {
	return &configError{
		code:       ErrConfigInvalidType.code,
		msg:        fmt.Sprintf("%s for %s in %s", ErrConfigInvalidType.msg, field, path),
		suggestion: fmt.Sprintf("Check the type of '%s' in %s.", field, path),
		cause:      cause,
	}
}

// NewConfigInvalid constructs an E6 error for the given field,
// invalid value, and allowed values.
//
// # Why the path is optional
//
// E6 errors arise from two sources: a value in the config file,
// and a value in a CLI flag or environment variable. The first
// has a path; the second does not. The constructor takes an
// optional path; when the path is empty, the message omits it.
//
// The signature uses a variadic path for compatibility with the
// existing callers from WBS 8.4.4; the callers pass one path or
// none.
func NewConfigInvalid(field, value string, allowed []string, path ...string) error {
	var location string
	if len(path) > 0 && path[0] != "" {
		location = fmt.Sprintf(" in %s", path[0])
	}
	return &configError{
		code:       ErrConfigInvalid.code,
		msg:        fmt.Sprintf("%s for %s: %s%s", ErrConfigInvalid.msg, field, value, location),
		suggestion: fmt.Sprintf("Allowed values: %s.", strings.Join(allowed, ", ")),
	}
}

// NewConfigTooLarge constructs an E7 error for the given path.
func NewConfigTooLarge(path string) error {
	return &configError{
		code:       ErrConfigTooLarge.code,
		msg:        fmt.Sprintf("%s: %s", ErrConfigTooLarge.msg, path),
		suggestion: "Config files must be under 1 MiB.",
	}
}

// NewConfigInvalidEncoding constructs an E8 error for the given
// path.
func NewConfigInvalidEncoding(path string) error {
	return &configError{
		code:       ErrConfigInvalidEncoding.code,
		msg:        fmt.Sprintf("%s in %s", ErrConfigInvalidEncoding.msg, path),
		suggestion: "Ensure the file is UTF-8 encoded without BOM.",
	}
}

// =============================================================================
// Sentinel accessor for tests and for a future structured-error model
// =============================================================================

// ConfigErrorCategory is the category string returned by every
// config error's Category() method.
//
// The constant is exported for tests and for the CLI's
// classifier; a future WBS that splits the exit codes reads it
// to know which string to compare against.
const ConfigErrorCategory = "config"

// asConfigError returns err as *configError, or nil if err is not
// one. The helper is used by tests and by a future structured-
// error model (WBS 10.0).
//
// # Why the function is unexported
//
// The function is used only by the package's own tests and by
// future code in the package. A caller outside the package uses
// the exported methods (Category, Suggestion, Code) through an
// interface assertion.
func asConfigError(err error) *configError {
	var ce *configError
	if errors.As(err, &ce) {
		return ce
	}
	return nil
}
