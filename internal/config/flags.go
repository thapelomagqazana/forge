// Package config defines the CLI configuration for Forge.
//
// This file holds the CLI flag resolution (WBS 8.4.4). The
// resolution applies explicitly-set CLI flags to a Config value.
// It is the highest-precedence layer of the configuration chain
// (WBS 8.1.2): the flags win over the environment, the config
// file, and the defaults.
//
// # The precedence chain
//
// The chain, from lowest to highest:
//
//  4. Default              (applied by Defaults)
//  3. Config file          (applied by Load)
//  2. Environment variable (applied by OverrideFromEnv)
//  1. CLI flag             (this file)
//
// A caller applies the layers in ascending order: Defaults, then
// Load, then OverrideFromEnv, then OverrideFromFlags. Each
// layer's non-empty values override the layer below.
//
// # Why this file does not import Cobra
//
// The resolution reads flag values from an interface
// (FlagReader) rather than from a *cobra.Command. The interface
// has one method for each thing the resolution needs to know:
// whether a flag was explicitly set, and what its value is.
//
// The concrete implementation of FlagReader lives in
// internal/cli (WBS 4.3.1 and its follow-ups); it wraps a
// *cobra.Command. The config package defines the interface;
// the CLI package implements it. This is the ports-and-adapters
// arrangement: the config package owns the port, the CLI package
// owns the adapter.
//
// The split keeps the config package free of Cobra. A future
// change to the CLI framework touches the adapter, not the
// resolution logic.
//
// # No package-level mutable state
//
// This file declares functions, constants, and an interface. It
// declares no package-level variables.
package config

import (
	"fmt"
	"strings"
)

// FlagReader is the interface the config package uses to read
// explicitly-set CLI flag values.
//
// # Why an interface
//
// The resolution needs two things per flag: whether the user
// explicitly set it, and what its value is. The interface
// expresses both requirements without naming Cobra. The CLI
// package's implementation wraps a *cobra.Command and provides
// the two answers.
//
// # Contract
//
// Changed(name) returns true if and only if the user explicitly
// set the named flag. A flag left at its default value returns
// false. A flag whose value equals its default because the user
// typed the default explicitly returns true.
//
// String(name) returns the flag's current value as a string. For
// a bool flag, the value is "true" or "false". The resolution
// only calls String on flags that Changed returned true for.
//
// # Nil safety
//
// A nil FlagReader is invalid; callers must supply a non-nil
// implementation. The OverrideFromFlags function checks for nil
// and returns the config unchanged if the reader is nil, but a
// caller that passes nil is using the API in a way the
// documentation does not recommend.
type FlagReader interface {
	// Changed reports whether the named flag was explicitly set
	// by the user.
	Changed(name string) bool

	// String returns the named flag's value as a string.
	String(name string) string
}

// FlagLogLevel is the CLI flag name for the LogLevel field.
//
// # Why a constant
//
// The flag name appears in the resolution and in the tests.
// A future rename is a single-line change here; the callers
// read the constant. The constant's value matches the schema's
// kebab-case flag name (WBS 8.2.2).
//
// The constant is exported because a future caller (the CLI's
// flag registration in a future WBS item) reads it to register
// the flag with the same name.
const FlagLogLevel = "log-level"

// FlagOutputFormat is the CLI flag name for the OutputFormat
// field.
//
// # Why a constant
//
// Same rationale as FlagLogLevel. The value matches the schema's
// kebab-case flag name (WBS 8.2.2).
const FlagOutputFormat = "output-format"

// OverrideFromFlags applies explicitly-set CLI flags to a Config.
//
// # What the function does
//
// For each configuration field in the schema (LogLevel,
// OutputFormat), the function:
//
//  1. Checks whether the field's flag was explicitly set by the
//     user via reader.Changed(name). A flag that was not set is
//     skipped.
//
//  2. Reads the flag's value via reader.String(name).
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
// The function applies the CLI flag layer of the precedence
// chain (WBS 8.1.2). The layer is the highest; its overrides
// are the final word. A caller applies the layers in ascending
// order:
//
//	cfg := Defaults()
//	cfg, err := Load(cfg.SourcePath)
//	cfg, err = OverrideFromEnv(cfg, os.Getenv)
//	cfg, err = OverrideFromFlags(cfg, reader)
//
// The reader is a FlagReader; see internal/cli/flags_adapter.go
// for the adapter that wraps a *cobra.Command.
//
// # Empty values are unset
//
// A flag set to the empty string (--log-level="") is treated as
// "not set": the Config's field stays at whatever the lower
// layers set. The empty value means "no preference"; the
// rationale matches the environment layer's empty-value rule
// (WBS 8.4.3).
//
// # Invalid values error
//
// A flag set to an invalid enum value (for example,
// --log-level=verbose) produces an error wrapping
// ErrConfigInvalid. The error's message names the flag and the
// invalid value. The caller maps the error to ExitUsage (2).
//
// # Case insensitivity
//
// The function lowercases the value before the enum check. A
// flag set to --log-level=DEBUG is accepted and the Config's
// field is normalised to "debug".
//
// # Nil reader
//
// A nil reader is a no-op: the function returns the Config
// unchanged and a nil error. The behaviour lets a caller pass
// a nil reader when it has none (for example, a test that
// wants to check the function's behaviour without a reader).
//
// # Purity
//
// The function reads only its two arguments (cfg and reader).
// It does not read os.Args, os.Getenv, or os.Getwd. It does
// not mutate cfg; it returns a new Config value. Two calls
// with the same arguments produce equal results, provided the
// reader is deterministic.
func OverrideFromFlags(cfg Config, reader FlagReader) (Config, error) {
	if reader == nil {
		return cfg, nil
	}

	if err := overrideLogLevelFromFlags(&cfg, reader); err != nil {
		return Config{}, err
	}

	if err := overrideOutputFormatFromFlags(&cfg, reader); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// overrideLogLevelFromFlags applies the --log-level flag to
// cfg's LogLevel field.
//
// See OverrideFromFlags for the resolution's full contract.
func overrideLogLevelFromFlags(cfg *Config, reader FlagReader) error {
	if !reader.Changed(FlagLogLevel) {
		return nil
	}

	raw := reader.String(FlagLogLevel)
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}

	normalised := strings.ToLower(value)
	if !isValidLogLevel(normalised) {
		return fmt.Errorf("%w: --%s=%q is not a valid log level (allowed: %s)",
			ErrConfigInvalid, FlagLogLevel, raw, allowedLogLevels())
	}

	cfg.LogLevel = normalised
	return nil
}

// overrideOutputFormatFromFlags applies the --output-format flag
// to cfg's OutputFormat field.
//
// See OverrideFromFlags for the resolution's full contract.
func overrideOutputFormatFromFlags(cfg *Config, reader FlagReader) error {
	if !reader.Changed(FlagOutputFormat) {
		return nil
	}

	raw := reader.String(FlagOutputFormat)
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}

	normalised := strings.ToLower(value)
	if !isValidOutputFormat(normalised) {
		return fmt.Errorf("%w: --%s=%q is not a valid output format (allowed: %s)",
			ErrConfigInvalid, FlagOutputFormat, raw, allowedOutputFormats())
	}

	cfg.OutputFormat = normalised
	return nil
}
