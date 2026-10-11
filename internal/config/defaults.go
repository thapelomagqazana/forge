// Package config defines the CLI configuration for Forge.
//
// This file holds the built-in default values (WBS 8.3.1). The
// defaults are the lowest-precedence layer of the configuration
// chain (WBS 8.1.2): when no flag, no environment variable, and no
// config file provide a value, the loader uses the value from
// Defaults().
//
// # The frozen defaults
//
//	Field          Default  Rationale
//	LogLevel       "info"   Balanced output; not silent, not noisy
//	OutputFormat   "text"   Human-readable is the assumed default
//
// The metadata field (SourcePath) has no default; its zero value
// (the empty string) means "not loaded from a file".
//
// # The safety criteria
//
// The WBS item's term "safe" is defined by six criteria. Every
// default satisfies all six:
//
//   - Non-destructive. No default causes file writes, network
//     access, or process execution. A default only sets a value
//     that the consumer reads; the consumer's behaviour is
//     unaffected by whether the value came from a default or from
//     another source.
//
//   - Non-silent. The LogLevel default ("info") produces enough
//     output to be useful. A default of "error" would hide
//     warnings; a default of "debug" would spam.
//
//   - Non-noisy. The LogLevel default ("info") does not produce
//     debug-level spam. "info" is the balance between "error"
//     (too quiet) and "debug" (too loud).
//
//   - Interoperable. The OutputFormat default ("text") is
//     readable in any terminal. "json" would be the default for a
//     tool whose primary consumer is a script; Forge's primary
//     consumer is a human.
//
//   - Stable. The defaults do not change without an ADR. A change
//     to a default affects every user who has not configured the
//     value; the change is a user-visible behaviour change.
//
//   - Testable. Every default has a test asserting its exact
//     value. The tests are in defaults_test.go.
//
// # Why the criteria matter
//
// A default that violated one of the criteria would be a bug. A
// "destructive" default would perform I/O on its own (for example,
// by triggering a file write); the default is a value, not an
// action, so it cannot. A "silent" default would hide useful
// information; the LogLevel default is "info" specifically to
// avoid that. A "noisy" default would spam; the LogLevel default
// is "info" specifically to avoid that. A non-interoperable
// default would break in some terminals; "text" is universal. An
// unstable default would surprise users on upgrade; the ADR
// requirement prevents the surprise. An untestable default would
// drift; the tests pin the values.
//
// # No package-level mutable state
//
// This file declares constants and functions. It declares no
// package-level variables.
package config

// Defaults returns the built-in configuration defaults.
//
// # What the function returns
//
// The function returns a Config value with the two configuration
// fields set to their defaults:
//
//	LogLevel     = LogLevelInfo   ("info")
//	OutputFormat = OutputFormatText ("text")
//
// The metadata field (SourcePath) is left at its zero value (the
// empty string), meaning "not loaded from a file".
//
// # Purity
//
// The function is pure: it reads no environment, touches no
// filesystem, accesses no package-level state, and returns the
// same value on every call for a given Forge version. The
// determinism is asserted by TestDefaults_IsPure.
//
// A future change to a default is a change to this function's
// return value; it requires an ADR (per the "stable" criterion).
//
// # Why the function references constants
//
// The function body uses the constants LogLevelInfo and
// OutputFormatText rather than the string literals "info" and
// "text". The constants are the schema's vocabulary (WBS 8.2.1);
// the default is a use of the vocabulary. Referencing the
// constants keeps the default consistent with the schema; a
// future change to a constant's value propagates through the
// default without a second edit.
//
// # Why the defaults are the lowest-precedence layer
//
// WBS 8.1.2's precedence order is:
//
//  1. CLI flag          (highest)
//  2. Environment var
//  3. Config file
//  4. Default           (lowest)
//
// The default applies only when no higher-precedence source
// provides a value. The loader (WBS 8.4.1) reads the four sources
// in descending precedence and stops at the first non-empty
// value; the default is the fallback.
//
// # Why a function, not a variable
//
// A package-level `var Defaults = Config{...}` would be mutable
// state; the CLI package's contract test forbids package-level
// variables. A function that returns a fresh value on each call
// has no package-level state. The Config type is a value type
// (three string fields), so returning it by value is cheap and
// safe.
func Defaults() Config {
	return Config{
		LogLevel:     LogLevelInfo,
		OutputFormat: OutputFormatText,
	}
}
