// Package config contains unit tests for the CLI configuration
// type.
//
// This file pins the CLI flag resolution (WBS 8.4.4). The
// resolution applies explicitly-set CLI flags to a Config.
//
// # The port / adapter split
//
// The production code (flags.go) reads flag values through the
// FlagReader interface. The interface is the port; the CLI
// package's flagReader adapter implements it over a
// *cobra.Command. This file tests the resolution against a
// fake FlagReader (fakeFlagReader, defined below). The fake is
// a small map-backed implementation; it has no dependency on
// Cobra.
//
// The adapter itself is tested in
// internal/cli/flags_adapter_test.go, where Cobra is a
// legitimate dependency.
//
// # Why the tests use a fake
//
// A fake reader has two properties the real command does not:
//
//   - It is trivial to construct. A test names the flags that
//     are "changed" and the values they carry; nothing else.
//
//   - It is deterministic. A real command's flags accumulate
//     state (the value and the changed marker); a test that
//     reused a command would see the previous test's state. A
//     fresh fake per test isolates the state, and the fake's
//     values are whatever the test set them to.
//
// The fake also keeps the config package's test suite free of
// Cobra. The package's production code does not import Cobra;
// its tests should not either. A future change to the CLI
// framework touches the adapter and its test, not the
// resolution and its tests.
package config

import (
	"errors"
	"strings"
	"testing"
)

// =============================================================================
// Test helper
// =============================================================================

// fakeFlagReader is an in-memory implementation of the
// FlagReader interface.
//
// # What the fake stores
//
// Two maps: one for the flags that were explicitly set (the
// "changed" state), one for the flags' values. A test
// constructs a fake with the states and values it wants; the
// resolution reads them.
//
// # How a test uses the fake
//
// The idiomatic construction is a single composite literal:
//
//	reader := &fakeFlagReader{
//	    changed: map[string]bool{
//	        FlagLogLevel: true,
//	    },
//	    values: map[string]string{
//	        FlagLogLevel: "debug",
//	    },
//	}
//
// The test then calls OverrideFromFlags(cfg, reader). A flag
// that is absent from the maps is treated as "not set"; the
// resolution leaves the corresponding Config field unchanged.
//
// # Why the fake is exported as lowercase
//
// The fake is a test double for a test in the same package; it
// is unexported because no external test needs it. If a future
// package needs a similar fake, the right move is to define a
// second fake in that package, not to export this one. The
// duplication is cheaper than a shared test-only package.
type fakeFlagReader struct {
	changed map[string]bool
	values  map[string]string
}

// Changed implements FlagReader.
//
// The method returns the map's value for the named flag, or
// false if the flag is absent. An absent flag is "not set".
func (f *fakeFlagReader) Changed(name string) bool {
	return f.changed[name]
}

// String implements FlagReader.
//
// The method returns the map's value for the named flag, or the
// empty string if the flag is absent. The resolution only calls
// String on flags for which Changed returned true; a test that
// sets a value without setting the changed state produces the
// empty string, which the resolution treats as "not set".
func (f *fakeFlagReader) String(name string) string {
	return f.values[name]
}

// =============================================================================
// No flags set
// =============================================================================

// TestOverrideFromFlags_NoFlags verifies that a Config is
// returned unchanged when no flags were explicitly set.
func TestOverrideFromFlags_NoFlags(t *testing.T) {
	t.Parallel()

	cfg := Config{LogLevel: LogLevelWarn, OutputFormat: OutputFormatJSON}
	reader := &fakeFlagReader{}

	got, err := OverrideFromFlags(cfg, reader)
	if err != nil {
		t.Fatalf("OverrideFromFlags: %v", err)
	}
	if got != cfg {
		t.Errorf("Config changed: got %+v; want %+v", got, cfg)
	}
}

// TestOverrideFromFlags_NilReader verifies that a nil reader
// produces a Config unchanged.
func TestOverrideFromFlags_NilReader(t *testing.T) {
	t.Parallel()

	cfg := Config{LogLevel: LogLevelWarn, OutputFormat: OutputFormatJSON}
	got, err := OverrideFromFlags(cfg, nil)
	if err != nil {
		t.Fatalf("OverrideFromFlags: %v", err)
	}
	if got != cfg {
		t.Errorf("Config changed: got %+v; want %+v", got, cfg)
	}
}

// =============================================================================
// Explicit overrides
// =============================================================================

// TestOverrideFromFlags_LogLevelOverride verifies that an
// explicitly-set --log-level overrides the Config's LogLevel.
func TestOverrideFromFlags_LogLevelOverride(t *testing.T) {
	t.Parallel()

	cfg := Config{LogLevel: LogLevelInfo, OutputFormat: OutputFormatText}
	reader := &fakeFlagReader{
		changed: map[string]bool{FlagLogLevel: true},
		values:  map[string]string{FlagLogLevel: "debug"},
	}

	got, err := OverrideFromFlags(cfg, reader)
	if err != nil {
		t.Fatalf("OverrideFromFlags: %v", err)
	}
	if got.LogLevel != LogLevelDebug {
		t.Errorf("LogLevel = %q; want %q",
			got.LogLevel, LogLevelDebug)
	}
	if got.OutputFormat != OutputFormatText {
		t.Errorf("OutputFormat = %q; want unchanged %q",
			got.OutputFormat, OutputFormatText)
	}
}

// TestOverrideFromFlags_OutputFormatOverride verifies that an
// explicitly-set --output-format overrides the Config's
// OutputFormat.
func TestOverrideFromFlags_OutputFormatOverride(t *testing.T) {
	t.Parallel()

	cfg := Config{LogLevel: LogLevelInfo, OutputFormat: OutputFormatText}
	reader := &fakeFlagReader{
		changed: map[string]bool{FlagOutputFormat: true},
		values:  map[string]string{FlagOutputFormat: "json"},
	}

	got, err := OverrideFromFlags(cfg, reader)
	if err != nil {
		t.Fatalf("OverrideFromFlags: %v", err)
	}
	if got.OutputFormat != OutputFormatJSON {
		t.Errorf("OutputFormat = %q; want %q",
			got.OutputFormat, OutputFormatJSON)
	}
	if got.LogLevel != LogLevelInfo {
		t.Errorf("LogLevel = %q; want unchanged %q",
			got.LogLevel, LogLevelInfo)
	}
}

// TestOverrideFromFlags_BothOverridden verifies that both flags
// override their fields.
func TestOverrideFromFlags_BothOverridden(t *testing.T) {
	t.Parallel()

	cfg := Config{LogLevel: LogLevelInfo, OutputFormat: OutputFormatText}
	reader := &fakeFlagReader{
		changed: map[string]bool{
			FlagLogLevel:     true,
			FlagOutputFormat: true,
		},
		values: map[string]string{
			FlagLogLevel:     "debug",
			FlagOutputFormat: "json",
		},
	}

	got, err := OverrideFromFlags(cfg, reader)
	if err != nil {
		t.Fatalf("OverrideFromFlags: %v", err)
	}
	if got.LogLevel != LogLevelDebug {
		t.Errorf("LogLevel = %q; want %q",
			got.LogLevel, LogLevelDebug)
	}
	if got.OutputFormat != OutputFormatJSON {
		t.Errorf("OutputFormat = %q; want %q",
			got.OutputFormat, OutputFormatJSON)
	}
}

// TestOverrideFromFlags_UnsetFlagsDoNotOverride verifies that a
// flag whose value equals the schema's default but was not
// explicitly set does not override the Config.
//
// The test is the negative of the "explicitly set" rule: a flag
// at its default must not be treated as a user-supplied value.
//
// # The fake's role in this test
//
// The fake has no entries. Changed returns false for both
// flags; the resolution skips both. The test asserts the
// Config is unchanged.
func TestOverrideFromFlags_UnsetFlagsDoNotOverride(t *testing.T) {
	t.Parallel()

	// The Config's fields are set to values that differ from
	// the flags' defaults. The flags are unset. The Config's
	// values must not change.
	cfg := Config{LogLevel: LogLevelWarn, OutputFormat: OutputFormatJSON}
	reader := &fakeFlagReader{}

	got, err := OverrideFromFlags(cfg, reader)
	if err != nil {
		t.Fatalf("OverrideFromFlags: %v", err)
	}
	if got != cfg {
		t.Errorf("unset flags overrode Config: got %+v; want %+v",
			got, cfg)
	}
}

// TestOverrideFromFlags_FlagEqualToDefaultButChanged verifies
// that a flag whose value equals the schema's default but WAS
// explicitly set by the user overrides the Config.
//
// The test pins the distinction between "the flag's value equals
// the default" and "the flag was explicitly set". Cobra's
// Changed method distinguishes the two; the resolution uses the
// method through the FlagReader interface. The fake's changed
// map is the fake's analogue of the method.
func TestOverrideFromFlags_FlagEqualToDefaultButChanged(t *testing.T) {
	t.Parallel()

	// The Config's LogLevel is debug; the flag's default is
	// info. The user explicitly sets the flag to info (the
	// default value). The resolution must override the
	// Config's debug with the flag's info.
	cfg := Config{LogLevel: LogLevelDebug, OutputFormat: OutputFormatText}
	reader := &fakeFlagReader{
		changed: map[string]bool{FlagLogLevel: true},
		values:  map[string]string{FlagLogLevel: LogLevelInfo},
	}

	got, err := OverrideFromFlags(cfg, reader)
	if err != nil {
		t.Fatalf("OverrideFromFlags: %v", err)
	}
	if got.LogLevel != LogLevelInfo {
		t.Errorf("LogLevel = %q; want %q (explicitly set to the "+
			"flag's default)", got.LogLevel, LogLevelInfo)
	}
}

// =============================================================================
// Empty and whitespace values
// =============================================================================

// TestOverrideFromFlags_EmptyValueIsUnset verifies that an
// explicitly-set flag with an empty value leaves the Config's
// field unchanged.
func TestOverrideFromFlags_EmptyValueIsUnset(t *testing.T) {
	t.Parallel()

	cfg := Config{LogLevel: LogLevelWarn, OutputFormat: OutputFormatJSON}
	reader := &fakeFlagReader{
		changed: map[string]bool{
			FlagLogLevel:     true,
			FlagOutputFormat: true,
		},
		values: map[string]string{
			FlagLogLevel:     "",
			FlagOutputFormat: "",
		},
	}

	got, err := OverrideFromFlags(cfg, reader)
	if err != nil {
		t.Fatalf("OverrideFromFlags: %v", err)
	}
	if got != cfg {
		t.Errorf("Config changed: got %+v; want %+v", got, cfg)
	}
}

// TestOverrideFromFlags_WhitespaceTrimmed verifies that leading
// and trailing whitespace in the flag value is trimmed.
func TestOverrideFromFlags_WhitespaceTrimmed(t *testing.T) {
	t.Parallel()

	cfg := Config{}
	reader := &fakeFlagReader{
		changed: map[string]bool{
			FlagLogLevel:     true,
			FlagOutputFormat: true,
		},
		values: map[string]string{
			FlagLogLevel:     "  debug  ",
			FlagOutputFormat: "\tjson\n",
		},
	}

	got, err := OverrideFromFlags(cfg, reader)
	if err != nil {
		t.Fatalf("OverrideFromFlags: %v", err)
	}
	if got.LogLevel != LogLevelDebug {
		t.Errorf("LogLevel = %q; want %q",
			got.LogLevel, LogLevelDebug)
	}
	if got.OutputFormat != OutputFormatJSON {
		t.Errorf("OutputFormat = %q; want %q",
			got.OutputFormat, OutputFormatJSON)
	}
}

// =============================================================================
// Case insensitivity
// =============================================================================

// TestOverrideFromFlags_CaseInsensitive verifies that the enum
// match is case-insensitive and the value is normalised to
// lowercase.
func TestOverrideFromFlags_CaseInsensitive(t *testing.T) {
	t.Parallel()

	cfg := Config{}
	reader := &fakeFlagReader{
		changed: map[string]bool{
			FlagLogLevel:     true,
			FlagOutputFormat: true,
		},
		values: map[string]string{
			FlagLogLevel:     "DEBUG",
			FlagOutputFormat: "JSON",
		},
	}

	got, err := OverrideFromFlags(cfg, reader)
	if err != nil {
		t.Fatalf("OverrideFromFlags: %v", err)
	}
	if got.LogLevel != LogLevelDebug {
		t.Errorf("LogLevel = %q; want %q",
			got.LogLevel, LogLevelDebug)
	}
	if got.OutputFormat != OutputFormatJSON {
		t.Errorf("OutputFormat = %q; want %q",
			got.OutputFormat, OutputFormatJSON)
	}
}

// =============================================================================
// Invalid values
// =============================================================================

// TestOverrideFromFlags_InvalidLogLevelErrors verifies that an
// invalid --log-level value produces an error wrapping
// ErrConfigInvalid.
func TestOverrideFromFlags_InvalidLogLevelErrors(t *testing.T) {
	t.Parallel()

	cfg := Config{}
	reader := &fakeFlagReader{
		changed: map[string]bool{FlagLogLevel: true},
		values:  map[string]string{FlagLogLevel: "verbose"},
	}

	_, err := OverrideFromFlags(cfg, reader)
	if !errors.Is(err, ErrConfigInvalid) {
		t.Fatalf("err = %v; want ErrConfigInvalid", err)
	}
	if !strings.Contains(err.Error(), FlagLogLevel) {
		t.Errorf("error does not name the flag: %v", err)
	}
	if !strings.Contains(err.Error(), "verbose") {
		t.Errorf("error does not name the invalid value: %v", err)
	}
}

// TestOverrideFromFlags_InvalidOutputFormatErrors verifies that
// an invalid --output-format value produces an error wrapping
// ErrConfigInvalid.
func TestOverrideFromFlags_InvalidOutputFormatErrors(t *testing.T) {
	t.Parallel()

	cfg := Config{}
	reader := &fakeFlagReader{
		changed: map[string]bool{FlagOutputFormat: true},
		values:  map[string]string{FlagOutputFormat: "xml"},
	}

	_, err := OverrideFromFlags(cfg, reader)
	if !errors.Is(err, ErrConfigInvalid) {
		t.Fatalf("err = %v; want ErrConfigInvalid", err)
	}
	if !strings.Contains(err.Error(), FlagOutputFormat) {
		t.Errorf("error does not name the flag: %v", err)
	}
	if !strings.Contains(err.Error(), "xml") {
		t.Errorf("error does not name the invalid value: %v", err)
	}
}

// =============================================================================
// Purity
// =============================================================================

// TestOverrideFromFlags_DoesNotMutateInput verifies that the
// function does not mutate the Config value passed to it.
func TestOverrideFromFlags_DoesNotMutateInput(t *testing.T) {
	t.Parallel()

	cfg := Config{LogLevel: LogLevelInfo, OutputFormat: OutputFormatText}
	original := cfg

	reader := &fakeFlagReader{
		changed: map[string]bool{
			FlagLogLevel:     true,
			FlagOutputFormat: true,
		},
		values: map[string]string{
			FlagLogLevel:     "debug",
			FlagOutputFormat: "json",
		},
	}

	_, err := OverrideFromFlags(cfg, reader)
	if err != nil {
		t.Fatalf("OverrideFromFlags: %v", err)
	}
	if cfg != original {
		t.Errorf("input Config was mutated: got %+v; want %+v",
			cfg, original)
	}
}

// TestOverrideFromFlags_IsDeterministic verifies that two calls
// with the same arguments produce equal results.
func TestOverrideFromFlags_IsDeterministic(t *testing.T) {
	t.Parallel()

	cfg := Config{}

	reader1 := &fakeFlagReader{
		changed: map[string]bool{
			FlagLogLevel:     true,
			FlagOutputFormat: true,
		},
		values: map[string]string{
			FlagLogLevel:     "debug",
			FlagOutputFormat: "json",
		},
	}
	reader2 := &fakeFlagReader{
		changed: map[string]bool{
			FlagLogLevel:     true,
			FlagOutputFormat: true,
		},
		values: map[string]string{
			FlagLogLevel:     "debug",
			FlagOutputFormat: "json",
		},
	}

	first, err1 := OverrideFromFlags(cfg, reader1)
	second, err2 := OverrideFromFlags(cfg, reader2)

	if err1 != nil || err2 != nil {
		t.Fatalf("errors: first=%v second=%v", err1, err2)
	}
	if first != second {
		t.Errorf("not deterministic: first=%+v second=%+v",
			first, second)
	}
}

// =============================================================================
// Precedence
// =============================================================================

// TestOverrideFromFlags_PrefersFlagOverEnv verifies the
// precedence chain's top-layer property: a CLI flag overrides an
// environment variable's value for the same field.
//
// The test constructs a Config that represents the result of
// applying Load and OverrideFromEnv (LogLevel: debug, from the
// env), applies the flag override (--log-level=warn), and
// asserts the result is the flag's value (warn).
func TestOverrideFromFlags_PrefersFlagOverEnv(t *testing.T) {
	t.Parallel()

	// The Config represents the result of applying the env
	// layer: FORGE_LOG_LEVEL=debug.
	cfg := Config{LogLevel: LogLevelDebug, OutputFormat: OutputFormatText}

	// The CLI flag sets log-level=warn. The flag's value wins
	// over the env's value.
	reader := &fakeFlagReader{
		changed: map[string]bool{FlagLogLevel: true},
		values:  map[string]string{FlagLogLevel: "warn"},
	}

	got, err := OverrideFromFlags(cfg, reader)
	if err != nil {
		t.Fatalf("OverrideFromFlags: %v", err)
	}
	if got.LogLevel != LogLevelWarn {
		t.Errorf("LogLevel = %q; want %q (flag should override env)",
			got.LogLevel, LogLevelWarn)
	}
}

// TestOverrideFromFlags_YieldsToLowerLayerWhenUnset verifies the
// precedence chain's top-layer property from the other
// direction: an unset flag leaves the lower layer's value in
// place.
func TestOverrideFromFlags_YieldsToLowerLayerWhenUnset(t *testing.T) {
	t.Parallel()

	// The Config represents the result of applying the env
	// layer: FORGE_LOG_LEVEL=debug.
	cfg := Config{LogLevel: LogLevelDebug, OutputFormat: OutputFormatText}

	// The CLI flag is unset. The env's value stays.
	reader := &fakeFlagReader{}

	got, err := OverrideFromFlags(cfg, reader)
	if err != nil {
		t.Fatalf("OverrideFromFlags: %v", err)
	}
	if got.LogLevel != LogLevelDebug {
		t.Errorf("LogLevel = %q; want %q (flag is unset; env's "+
			"value stays)", got.LogLevel, LogLevelDebug)
	}
}

// =============================================================================
// Interaction with other layers
// =============================================================================

// TestOverrideFromFlags_FullChain verifies the full chain's
// composition: Defaults, then the file (represented by a Config
// with a file-set value), then the env (applied by
// OverrideFromEnv), then the flag (applied by
// OverrideFromFlags).
//
// The test simulates the layers by constructing the
// intermediate Config values directly; it does not call Load
// (which would require a fixture file) or OverrideFromEnv
// (which is tested in env_test.go).
func TestOverrideFromFlags_FullChain(t *testing.T) {
	t.Parallel()

	// Layer 4: the default.
	cfg := Defaults()
	if cfg.LogLevel != LogLevelInfo {
		t.Fatalf("Defaults: LogLevel = %q; want %q",
			cfg.LogLevel, LogLevelInfo)
	}

	// Layer 3: the file's value. Simulated by setting the
	// field directly.
	cfg.LogLevel = LogLevelWarn

	// Layer 2: the env's value. Simulated by applying
	// OverrideFromEnv with a synthetic env function.
	env := envFunc(map[string]string{
		"FORGE_LOG_LEVEL": "debug",
	})
	cfg, err := OverrideFromEnv(cfg, env)
	if err != nil {
		t.Fatalf("OverrideFromEnv: %v", err)
	}
	if cfg.LogLevel != LogLevelDebug {
		t.Fatalf("after env: LogLevel = %q; want %q",
			cfg.LogLevel, LogLevelDebug)
	}

	// Layer 1: the flag's value. Simulated by applying
	// OverrideFromFlags with a fake reader whose --log-level
	// flag is set.
	reader := &fakeFlagReader{
		changed: map[string]bool{FlagLogLevel: true},
		values:  map[string]string{FlagLogLevel: "error"},
	}
	cfg, err = OverrideFromFlags(cfg, reader)
	if err != nil {
		t.Fatalf("OverrideFromFlags: %v", err)
	}
	if cfg.LogLevel != LogLevelError {
		t.Errorf("after flag: LogLevel = %q; want %q (flag wins)",
			cfg.LogLevel, LogLevelError)
	}
}

// TestOverrideFromFlags_EnvStaysWhenFlagUnset verifies the
// chain's middle-layer property: an unset flag leaves the env's
// value in place.
func TestOverrideFromFlags_EnvStaysWhenFlagUnset(t *testing.T) {
	t.Parallel()

	// Layer 4: the default.
	cfg := Defaults()

	// Layer 3: the file's value. Simulated.
	cfg.LogLevel = LogLevelWarn

	// Layer 2: the env's value.
	env := envFunc(map[string]string{
		"FORGE_LOG_LEVEL": "debug",
	})
	var err error
	cfg, err = OverrideFromEnv(cfg, env)
	if err != nil {
		t.Fatalf("OverrideFromEnv: %v", err)
	}

	// Layer 1: the flag is unset.
	reader := &fakeFlagReader{}
	cfg, err = OverrideFromFlags(cfg, reader)
	if err != nil {
		t.Fatalf("OverrideFromFlags: %v", err)
	}
	if cfg.LogLevel != LogLevelDebug {
		t.Errorf("LogLevel = %q; want %q (env's value stays)",
			cfg.LogLevel, LogLevelDebug)
	}
}
