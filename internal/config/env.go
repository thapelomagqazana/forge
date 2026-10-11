// Package config defines the CLI configuration for Forge.
//
// This file holds the environment-variable convention (WBS 8.2.2)
// and the environment-variable resolution (WBS 8.4.3). The
// convention derives an environment variable's name from a
// configuration field's name. The resolution applies the
// variables' values to a Config.
//
// # The convention
//
// A field name is a lowercase snake_case string ("log_level"). An
// environment variable name is the field name with a prefix
// ("FORGE_") and all-uppercase letters ("FORGE_LOG_LEVEL").
//
// The rules:
//
//   - Prefix: FORGE_ (frozen).
//   - Case: uppercase (frozen).
//   - Separator: underscore (frozen).
//   - No nested namespaces in Phase 2 (for example, no
//     FORGE_LOG_LEVEL_DEBUG).
//
// # The reserved variable
//
// FORGE_CONFIG is reserved for the config file path. It is the
// environment equivalent of --config; a user who sets
// FORGE_CONFIG=/path/to/config.yaml gets the same behaviour as
// a user who passes --config /path/to/config.yaml.
//
// The reserved variable is not derived from a field name; it is
// a special case. The function EnvVarNameFor derives a variable
// name from a field name; the reserved variable is documented in
// ReservedEnvVars.
//
// # The resolution
//
// The OverrideFromEnv function applies the environment variables'
// values to a Config value. It is the middle layer of the
// precedence chain (WBS 8.1.2): the config file's values are
// read first; the environment variables override them; the CLI
// flags override the environment variables in turn.
//
// # No package-level mutable state
//
// This file declares constants and functions. It declares no
// package-level variables.
package config

import (
	"fmt"
	"strings"
)

// EnvVarPrefix is the prefix on every Forge environment variable.
//
// # Why the prefix is a constant
//
// The prefix appears on every variable the CLI reads. Its
// spelling is part of the convention; a future change to the
// prefix is a change to the convention and would break every
// user's environment. The constant makes the prefix's spelling
// visible and changeable in one place.
//
// # Why the prefix is uppercase
//
// Environment variable names are conventionally uppercase on
// POSIX systems. The uppercase prefix matches the convention.
// A lowercase prefix would work on most systems but would be
// unusual; the uppercase form is the safe choice.
const EnvVarPrefix = "FORGE_"

// EnvVarConfigPath is the reserved environment variable that
// holds the config file path.
//
// # Why the variable is reserved
//
// The variable is the environment equivalent of --config. A
// user who sets it gets the same behaviour as a user who passes
// the flag. The variable is not derived from a field name
// because the field it corresponds to (SourcePath) is metadata,
// not a config value (see WBS 8.1.1).
//
// # Why the constant is separate from EnvVarPrefix
//
// The prefix is a rule; the reserved variable is an instance.
// A future addition to the reserved variables (for example, a
// FORGE_HOME for the user's config directory) adds a new
// constant, not a new rule.
const EnvVarConfigPath = EnvVarPrefix + "CONFIG"

// EnvVarNameFor returns the environment variable name for a
// configuration field.
//
// # The rule
//
// The function uppercases the field name and prepends the
// prefix. For example:
//
//	EnvVarNameFor("log_level")       == "FORGE_LOG_LEVEL"
//	EnvVarNameFor("output_format")   == "FORGE_OUTPUT_FORMAT"
//
// The field name is expected to be lowercase snake_case. The
// function does not validate the field name's case; it uppercases
// whatever it is given. A field name that is already uppercase
// produces the same result as its lowercase form.
//
// # The empty field name
//
// An empty field name produces the prefix alone ("FORGE_"). The
// result is not a valid environment variable; a caller that
// passes an empty field name has a bug. The function returns the
// prefix rather than panicking so that the caller can decide how
// to handle the situation.
//
// # Why the function does not validate
//
// The function is a pure mapping from field name to variable
// name. Validation is a separate concern: a future loader
// validates the field names it reads; the function does not.
// The separation keeps the function simple and testable.
func EnvVarNameFor(fieldName string) string {
	return EnvVarPrefix + strings.ToUpper(fieldName)
}

// ReservedEnvVars returns the environment variables that the CLI
// reads but that are not derived from a configuration field.
//
// # What the function returns
//
// The function returns the reserved variables in a fixed order.
// The current reserved variable is EnvVarConfigPath; a future
// phase may add others. The order is not significant; a caller
// that iterates the variables gets a deterministic result.
//
// # Why the function exists
//
// A test that wants to check "the reserved variables are what we
// expect" calls the function. A future loader that wants to look
// up the reserved variables calls the function and iterates. The
// function is the single source of truth for the reserved set.
//
// # Why a function, not a variable
//
// The CLI package's contract test forbids package-level
// variables. A function that returns a fresh slice on each call
// has no package-level state.
func ReservedEnvVars() []string {
	return []string{
		EnvVarConfigPath,
	}
}

// IsReservedEnvVar reports whether name is one of the reserved
// environment variables.
//
// # What the function does
//
// The function compares name against each reserved variable. The
// comparison is case-sensitive: an environment variable is
// uppercase, and a name that differs only by case is not the
// same variable on POSIX systems.
//
// # Why the function exists
//
// A future loader that reads environment variables checks
// whether a variable is reserved before reading it. The
// function is the check's implementation; the reserved set is
// documented in ReservedEnvVars.
func IsReservedEnvVar(name string) bool {
	for _, reserved := range ReservedEnvVars() {
		if name == reserved {
			return true
		}
	}
	return false
}

// =============================================================================
// Environment variable resolution (WBS 8.4.3)
// =============================================================================

// OverrideFromEnv applies environment variables to a Config.
//
// # What the function does
//
// For each configuration field in the schema (LogLevel,
// OutputFormat), the function:
//
//  1. Computes the field's environment variable name via
//     EnvVarNameFor.
//
//  2. Reads the variable via the env function.
//
//  3. Trims leading and trailing whitespace from the value.
//
//  4. If the trimmed value is empty, leaves the Config's field
//     unchanged (the empty value means "not set").
//
//  5. If the trimmed value is non-empty, validates it against
//     the field's allowed values. A valid value overrides the
//     Config's field. An invalid value produces an error
//     wrapping ErrConfigInvalid.
//
// The function returns the (possibly modified) Config.
//
// # Precedence
//
// The function applies the environment layer of the precedence
// chain (WBS 8.1.2). The chain, from lowest to highest:
//
//  4. Default (applied by Defaults)
//  3. Config file (applied by Load)
//  2. Environment variable (this function)
//  1. CLI flag (applied by a future WBS item)
//
// A caller applies the layers in ascending order: Defaults, then
// Load, then OverrideFromEnv, then the flag resolution. Each
// layer's non-empty values override the layer below.
//
// # Empty values are unset
//
// A variable set to the empty string ("FORGE_LOG_LEVEL=")
// leaves the Config's field unchanged. The empty value means
// "no preference"; the next layer's value stays in place. The
// rationale: a user who unsets a variable or leaves it empty
// wants the next source to apply.
//
// # Invalid values error
//
// A variable set to an invalid enum value (for example,
// "FORGE_LOG_LEVEL=verbose") produces an error wrapping
// ErrConfigInvalid. The error's message names the variable and
// the invalid value. The caller maps the error to ExitUsage (2).
//
// # Case insensitivity
//
// The function lowercases the value before the enum check. A
// variable set to "FORGE_LOG_LEVEL=DEBUG" is accepted and the
// Config's field is normalised to "debug". Environment
// variables are conventionally uppercase; the case-insensitive
// match accommodates the convention.
//
// # Unknown variables
//
// A variable with the FORGE_ prefix that is not in the schema
// (for example, "FORGE_TEST_VAR") is ignored. The function
// reads only the variables the schema declares; an unknown
// variable is not a config value and is not an error. The
// behaviour allows unrelated tooling to set FORGE_* variables.
//
// # The reserved variable
//
// The function does not read FORGE_CONFIG. The variable is
// reserved (WBS 8.1.2); its consumer is Discover (WBS 8.4.1).
// The function's field list is the schema's configuration
// fields; the reserved variable is not one of them.
//
// # Purity
//
// The function reads only its two arguments (cfg and env). It
// does not read os.Getenv, os.Getwd, or os.Args. It does not
// mutate cfg; it returns a new Config value. Two calls with the
// same arguments produce equal results.
func OverrideFromEnv(cfg Config, env func(string) string) (Config, error) {
	if env == nil {
		return cfg, nil
	}

	// LogLevel.
	if err := overrideLogLevel(&cfg, env); err != nil {
		return Config{}, err
	}

	// OutputFormat.
	if err := overrideOutputFormat(&cfg, env); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// overrideLogLevel applies the FORGE_LOG_LEVEL variable to cfg's
// LogLevel field.
//
// # What the function does
//
// The function reads EnvVarNameFor("log_level") via env, trims
// whitespace, and applies the value if non-empty. A valid value
// is normalised (lowercased) and written to cfg. An invalid
// value produces an error wrapping ErrConfigInvalid.
//
// # Why a helper
//
// The logic for one field is small but not trivial: the trim,
// the empty check, the case-normalisation, the enum check, and
// the error's wording. Extracting the logic into a helper keeps
// OverrideFromEnv readable and makes the field-specific error
// messages consistent.
func overrideLogLevel(cfg *Config, env func(string) string) error {
	name := EnvVarNameFor("log_level")
	raw := env(name)
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}
	normalised := strings.ToLower(value)
	if !isValidLogLevel(normalised) {
		return fmt.Errorf("%w: %s=%q is not a valid log level (allowed: %s)",
			ErrConfigInvalid, name, raw, allowedLogLevels())
	}
	cfg.LogLevel = normalised
	return nil
}

// overrideOutputFormat applies the FORGE_OUTPUT_FORMAT variable
// to cfg's OutputFormat field.
//
// # What the function does
//
// The function reads EnvVarNameFor("output_format") via env,
// trims whitespace, and applies the value if non-empty. A valid
// value is normalised (lowercased) and written to cfg. An
// invalid value produces an error wrapping ErrConfigInvalid.
//
// # The reserved variable
//
// The function does not read FORGE_CONFIG. The reserved variable
// is not the field's variable; the field's variable is
// FORGE_OUTPUT_FORMAT. The distinction is documented on
// OverrideFromEnv.
func overrideOutputFormat(cfg *Config, env func(string) string) error {
	name := EnvVarNameFor("output_format")
	raw := env(name)
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}
	normalised := strings.ToLower(value)
	if !isValidOutputFormat(normalised) {
		return fmt.Errorf("%w: %s=%q is not a valid output format (allowed: %s)",
			ErrConfigInvalid, name, raw, allowedOutputFormats())
	}
	cfg.OutputFormat = normalised
	return nil
}
