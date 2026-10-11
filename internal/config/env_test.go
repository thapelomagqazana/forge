// Package config contains unit tests for the CLI configuration
// type.
//
// This file pins the environment-variable naming convention (WBS
// 8.2.2) and the environment-variable resolution (WBS 8.4.3). The
// convention derives a variable's name from a field's name; the
// resolution applies the variables' values to a Config.
//
// # How the tests work
//
// The convention tests are table-driven over inputs and expected
// names. The resolution tests construct a Config value, construct
// a synthetic env function, call OverrideFromEnv, and assert the
// returned Config and any error. The tests do not read the real
// environment; a test that used os.Getenv would depend on the
// process's state, which is not reproducible.
package config

import (
	"errors"
	"strings"
	"testing"
)

// =============================================================================
// The naming convention (WBS 8.2.2)
// =============================================================================

// TestEnv_EnvVarNameFor verifies that EnvVarNameFor produces the
// expected variable name for each field.
func TestEnv_EnvVarNameFor(t *testing.T) {
	t.Parallel()

	cases := []struct {
		field string
		want  string
	}{
		{"log_level", "FORGE_LOG_LEVEL"},
		{"output_format", "FORGE_OUTPUT_FORMAT"},
		{"config_path", "FORGE_CONFIG_PATH"},
		{"simple", "FORGE_SIMPLE"},
		{"a", "FORGE_A"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.field, func(t *testing.T) {
			t.Parallel()

			got := EnvVarNameFor(tc.field)
			if got != tc.want {
				t.Errorf("EnvVarNameFor(%q) = %q; want %q",
					tc.field, got, tc.want)
			}
		})
	}
}

// TestEnv_EnvVarNameIsDeterministic verifies that EnvVarNameFor
// is deterministic: two calls with the same input produce the
// same output.
func TestEnv_EnvVarNameIsDeterministic(t *testing.T) {
	t.Parallel()

	fields := []string{"log_level", "output_format", "config_path"}
	for _, field := range fields {
		first := EnvVarNameFor(field)
		second := EnvVarNameFor(field)
		if first != second {
			t.Errorf("EnvVarNameFor(%q) is not deterministic: %q vs %q",
				field, first, second)
		}
	}
}

// TestEnv_EnvVarNameForEmptyFieldName verifies the boundary case
// of an empty field name.
//
// The function returns the prefix alone ("FORGE_"). The result is
// not a valid variable name; the test pins the boundary's
// behaviour so that a future change to the function's handling of
// the empty case is caught.
func TestEnv_EnvVarNameForEmptyFieldName(t *testing.T) {
	t.Parallel()

	got := EnvVarNameFor("")
	if got != EnvVarPrefix {
		t.Errorf("EnvVarNameFor(\"\") = %q; want %q",
			got, EnvVarPrefix)
	}
}

// TestEnv_EnvVarNameIsUppercase verifies that every derived
// variable name is uppercase.
//
// The rule is "uppercase"; the test asserts the property
// independently of the specific field names. A future change to
// the derivation that produced a lowercase character would fail
// the test.
func TestEnv_EnvVarNameIsUppercase(t *testing.T) {
	t.Parallel()

	fields := []string{
		"log_level",
		"output_format",
		"config_path",
		"a",
		"a_b_c",
		"x1_y2",
	}
	for _, field := range fields {
		got := EnvVarNameFor(field)
		if got != strings.ToUpper(got) {
			t.Errorf("EnvVarNameFor(%q) = %q; not uppercase",
				field, got)
		}
	}
}

// TestEnv_EnvVarNameStartsWithPrefix verifies that every derived
// variable name starts with the prefix.
func TestEnv_EnvVarNameStartsWithPrefix(t *testing.T) {
	t.Parallel()

	fields := []string{"log_level", "output_format", "config_path", "a"}
	for _, field := range fields {
		got := EnvVarNameFor(field)
		if !strings.HasPrefix(got, EnvVarPrefix) {
			t.Errorf("EnvVarNameFor(%q) = %q; does not start with %q",
				field, got, EnvVarPrefix)
		}
	}
}

// TestEnv_ReservedEnvVarsAreKnown verifies that the reserved
// variables list contains the expected variable.
func TestEnv_ReservedEnvVarsAreKnown(t *testing.T) {
	t.Parallel()

	reserved := ReservedEnvVars()
	if len(reserved) == 0 {
		t.Fatal("ReservedEnvVars returned no variables")
	}

	found := false
	for _, name := range reserved {
		if name == EnvVarConfigPath {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ReservedEnvVars does not include %q",
			EnvVarConfigPath)
	}
}

// TestEnv_EnvVarConfigPathIsCorrect verifies that the reserved
// variable's name matches the convention.
func TestEnv_EnvVarConfigPathIsCorrect(t *testing.T) {
	t.Parallel()

	want := EnvVarPrefix + "CONFIG"
	if EnvVarConfigPath != want {
		t.Errorf("EnvVarConfigPath = %q; want %q",
			EnvVarConfigPath, want)
	}
}

// TestEnv_IsReservedEnvVar verifies that IsReservedEnvVar
// correctly identifies the reserved variables.
func TestEnv_IsReservedEnvVar(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		want bool
	}{
		{EnvVarConfigPath, true},
		{"FORGE_CONFIG", true},
		{EnvVarNameFor("log_level"), false},
		{EnvVarNameFor("output_format"), false},
		{"forger_config", false}, // typo, not the reserved name
		{"", false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := IsReservedEnvVar(tc.name); got != tc.want {
				t.Errorf("IsReservedEnvVar(%q) = %v; want %v",
					tc.name, got, tc.want)
			}
		})
	}
}

// TestEnv_ConfigPathEnvVarIsReserved verifies that the config
// path's environment variable is not the same as the one derived
// from a "config_path" field name.
//
// The reserved variable (FORGE_CONFIG) and the derived variable
// (FORGE_CONFIG_PATH) are distinct. The distinction is
// deliberate: the config path is metadata (WBS 8.1.1), not a
// config value; its environment variable has a shorter name that
// matches the --config flag.
func TestEnv_ConfigPathEnvVarIsReserved(t *testing.T) {
	t.Parallel()

	derived := EnvVarNameFor("config_path")
	if derived == EnvVarConfigPath {
		t.Errorf("the derived variable for config_path (%q) is the "+
			"same as the reserved variable (%q); "+
			"the two are distinct by design",
			derived, EnvVarConfigPath)
	}
}

// TestOverrideFromEnv_NoVariables verifies that a Config is
// returned unchanged when no FORGE_* variables are set.
func TestOverrideFromEnv_NoVariables(t *testing.T) {
	t.Parallel()

	cfg := Config{LogLevel: LogLevelInfo, OutputFormat: OutputFormatText}
	got, err := OverrideFromEnv(cfg, envFunc(nil))
	if err != nil {
		t.Fatalf("OverrideFromEnv: %v", err)
	}
	if got != cfg {
		t.Errorf("Config changed: got %+v; want %+v", got, cfg)
	}
}

// TestOverrideFromEnv_NilEnv verifies that a nil env function
// produces a Config unchanged.
//
// The function's nil-handling is a convenience: a caller that has
// no environment to inspect passes nil rather than an empty
// closure. The test pins the behaviour.
func TestOverrideFromEnv_NilEnv(t *testing.T) {
	t.Parallel()

	cfg := Config{LogLevel: LogLevelInfo, OutputFormat: OutputFormatText}
	got, err := OverrideFromEnv(cfg, nil)
	if err != nil {
		t.Fatalf("OverrideFromEnv: %v", err)
	}
	if got != cfg {
		t.Errorf("Config changed: got %+v; want %+v", got, cfg)
	}
}

// TestOverrideFromEnv_LogLevelOverride verifies that
// FORGE_LOG_LEVEL overrides the Config's LogLevel.
//
// The OutputFormat's variable is not set; the field's value is
// unchanged. The test pins the per-field independence.
func TestOverrideFromEnv_LogLevelOverride(t *testing.T) {
	t.Parallel()

	cfg := Config{LogLevel: LogLevelInfo, OutputFormat: OutputFormatText}
	env := envFunc(map[string]string{
		"FORGE_LOG_LEVEL": "debug",
	})

	got, err := OverrideFromEnv(cfg, env)
	if err != nil {
		t.Fatalf("OverrideFromEnv: %v", err)
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

// TestOverrideFromEnv_OutputFormatOverride verifies that
// FORGE_OUTPUT_FORMAT overrides the Config's OutputFormat.
func TestOverrideFromEnv_OutputFormatOverride(t *testing.T) {
	t.Parallel()

	cfg := Config{LogLevel: LogLevelInfo, OutputFormat: OutputFormatText}
	env := envFunc(map[string]string{
		"FORGE_OUTPUT_FORMAT": "json",
	})

	got, err := OverrideFromEnv(cfg, env)
	if err != nil {
		t.Fatalf("OverrideFromEnv: %v", err)
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

// TestOverrideFromEnv_BothOverridden verifies that both
// variables override their fields.
func TestOverrideFromEnv_BothOverridden(t *testing.T) {
	t.Parallel()

	cfg := Config{LogLevel: LogLevelInfo, OutputFormat: OutputFormatText}
	env := envFunc(map[string]string{
		"FORGE_LOG_LEVEL":     "debug",
		"FORGE_OUTPUT_FORMAT": "json",
	})

	got, err := OverrideFromEnv(cfg, env)
	if err != nil {
		t.Fatalf("OverrideFromEnv: %v", err)
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

// TestOverrideFromEnv_EmptyValueIsUnset verifies that an empty
// variable value leaves the Config's field unchanged.
//
// The rule is "empty means not set"; the test pins the rule for
// both fields.
func TestOverrideFromEnv_EmptyValueIsUnset(t *testing.T) {
	t.Parallel()

	cfg := Config{LogLevel: LogLevelInfo, OutputFormat: OutputFormatText}
	env := envFunc(map[string]string{
		"FORGE_LOG_LEVEL":     "",
		"FORGE_OUTPUT_FORMAT": "",
	})

	got, err := OverrideFromEnv(cfg, env)
	if err != nil {
		t.Fatalf("OverrideFromEnv: %v", err)
	}
	if got != cfg {
		t.Errorf("Config changed: got %+v; want %+v", got, cfg)
	}
}

// TestOverrideFromEnv_WhitespaceTrimmed verifies that leading and
// trailing whitespace in the variable value is trimmed.
//
// The rule is "values are trimmed"; the test pins the rule for
// the whitespace characters that a shell or a Makefile might
// introduce (spaces, tabs, newlines).
func TestOverrideFromEnv_WhitespaceTrimmed(t *testing.T) {
	t.Parallel()

	cfg := Config{}
	env := envFunc(map[string]string{
		"FORGE_LOG_LEVEL":     "  debug\n",
		"FORGE_OUTPUT_FORMAT": "\tjson ",
	})

	got, err := OverrideFromEnv(cfg, env)
	if err != nil {
		t.Fatalf("OverrideFromEnv: %v", err)
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

// TestOverrideFromEnv_CaseInsensitive verifies that the enum
// match is case-insensitive and the value is normalised to
// lowercase.
//
// Environment variables are conventionally uppercase; the test
// uses uppercase values and asserts the lowercase result.
func TestOverrideFromEnv_CaseInsensitive(t *testing.T) {
	t.Parallel()

	cfg := Config{}
	env := envFunc(map[string]string{
		"FORGE_LOG_LEVEL":     "DEBUG",
		"FORGE_OUTPUT_FORMAT": "JSON",
	})

	got, err := OverrideFromEnv(cfg, env)
	if err != nil {
		t.Fatalf("OverrideFromEnv: %v", err)
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

// TestOverrideFromEnv_InvalidLogLevelErrors verifies that an
// invalid FORGE_LOG_LEVEL value produces an error wrapping
// ErrConfigInvalid.
//
// The test asserts the error's sentinel and its message's content
// (the variable's name and the invalid value).
func TestOverrideFromEnv_InvalidLogLevelErrors(t *testing.T) {
	t.Parallel()

	cfg := Config{}
	env := envFunc(map[string]string{
		"FORGE_LOG_LEVEL": "verbose",
	})

	_, err := OverrideFromEnv(cfg, env)
	if !errors.Is(err, ErrConfigInvalid) {
		t.Fatalf("err = %v; want ErrConfigInvalid", err)
	}
	if !strings.Contains(err.Error(), "FORGE_LOG_LEVEL") {
		t.Errorf("error does not name the variable: %v", err)
	}
	if !strings.Contains(err.Error(), "verbose") {
		t.Errorf("error does not name the invalid value: %v", err)
	}
}

// TestOverrideFromEnv_InvalidOutputFormatErrors verifies that an
// invalid FORGE_OUTPUT_FORMAT value produces an error wrapping
// ErrConfigInvalid.
func TestOverrideFromEnv_InvalidOutputFormatErrors(t *testing.T) {
	t.Parallel()

	cfg := Config{}
	env := envFunc(map[string]string{
		"FORGE_OUTPUT_FORMAT": "xml",
	})

	_, err := OverrideFromEnv(cfg, env)
	if !errors.Is(err, ErrConfigInvalid) {
		t.Fatalf("err = %v; want ErrConfigInvalid", err)
	}
	if !strings.Contains(err.Error(), "FORGE_OUTPUT_FORMAT") {
		t.Errorf("error does not name the variable: %v", err)
	}
	if !strings.Contains(err.Error(), "xml") {
		t.Errorf("error does not name the invalid value: %v", err)
	}
}

// TestOverrideFromEnv_UnknownForgeVarsIgnored verifies that an
// unknown FORGE_* variable is ignored.
//
// The test sets FORGE_UNKNOWN=foo; the function returns the
// Config unchanged. The env function's call count for the unknown
// variable is asserted to be zero.
func TestOverrideFromEnv_UnknownForgeVarsIgnored(t *testing.T) {
	t.Parallel()

	cfg := Config{LogLevel: LogLevelInfo, OutputFormat: OutputFormatText}

	calls := make(map[string]int)
	env := func(name string) string {
		calls[name]++
		switch name {
		case "FORGE_UNKNOWN":
			return "foo"
		default:
			return ""
		}
	}

	got, err := OverrideFromEnv(cfg, env)
	if err != nil {
		t.Fatalf("OverrideFromEnv: %v", err)
	}
	if got != cfg {
		t.Errorf("Config changed: got %+v; want %+v", got, cfg)
	}
	if calls["FORGE_UNKNOWN"] != 0 {
		t.Errorf("env was called for FORGE_UNKNOWN %d times; want 0",
			calls["FORGE_UNKNOWN"])
	}
}

// TestOverrideFromEnv_ConfigVarIsReserved verifies that
// FORGE_CONFIG is not treated as a config value.
//
// The variable is reserved for the config file path; its
// consumer is Discover (WBS 8.4.1). The function does not read
// it, and the SourcePath field is not set from its value.
func TestOverrideFromEnv_ConfigVarIsReserved(t *testing.T) {
	t.Parallel()

	cfg := Config{LogLevel: LogLevelInfo, OutputFormat: OutputFormatText}
	env := envFunc(map[string]string{
		EnvVarConfigPath: "/some/path/config.yaml",
	})

	got, err := OverrideFromEnv(cfg, env)
	if err != nil {
		t.Fatalf("OverrideFromEnv: %v", err)
	}
	if got != cfg {
		t.Errorf("Config changed by FORGE_CONFIG: got %+v; want %+v",
			got, cfg)
	}
	if got.SourcePath != "" {
		t.Errorf("SourcePath = %q; want empty (FORGE_CONFIG does "+
			"not set the field)", got.SourcePath)
	}
}

// TestOverrideFromEnv_DoesNotMutateInput verifies that the
// function does not mutate the Config value passed to it.
//
// The rule is the value-type contract: the function reads cfg
// and returns a new value. The test pins the contract.
func TestOverrideFromEnv_DoesNotMutateInput(t *testing.T) {
	t.Parallel()

	cfg := Config{LogLevel: LogLevelInfo, OutputFormat: OutputFormatText}
	original := cfg

	env := envFunc(map[string]string{
		"FORGE_LOG_LEVEL":     "debug",
		"FORGE_OUTPUT_FORMAT": "json",
	})

	_, err := OverrideFromEnv(cfg, env)
	if err != nil {
		t.Fatalf("OverrideFromEnv: %v", err)
	}
	if cfg != original {
		t.Errorf("input Config was mutated: got %+v; want %+v",
			cfg, original)
	}
}

// TestOverrideFromEnv_EnvFuncIsNotCalledForUnknownFields
// verifies that the env function is called only for the schema's
// fields.
//
// The test wraps the env function with a counter; after the
// call, the counter's keys are the schema's fields' variables.
// The check pins the call pattern: one call per schema field,
// none for anything else.
func TestOverrideFromEnv_EnvFuncIsNotCalledForUnknownFields(t *testing.T) {
	t.Parallel()

	cfg := Config{}

	calls := make(map[string]int)
	env := func(name string) string {
		calls[name]++
		return ""
	}

	if _, err := OverrideFromEnv(cfg, env); err != nil {
		t.Fatalf("OverrideFromEnv: %v", err)
	}

	// The expected calls are FORGE_LOG_LEVEL and
	// FORGE_OUTPUT_FORMAT. The function must not call env for
	// any other name.
	expected := map[string]bool{
		"FORGE_LOG_LEVEL":     true,
		"FORGE_OUTPUT_FORMAT": true,
	}
	for name := range calls {
		if !expected[name] {
			t.Errorf("env was called for unexpected name %q", name)
		}
	}
	for name := range expected {
		if calls[name] != 1 {
			t.Errorf("env was called for %q %d times; want 1",
				name, calls[name])
		}
	}
}

// TestOverrideFromEnv_IsDeterministic verifies that two calls
// with the same arguments produce equal results.
func TestOverrideFromEnv_IsDeterministic(t *testing.T) {
	t.Parallel()

	cfg := Config{}
	env := envFunc(map[string]string{
		"FORGE_LOG_LEVEL":     "debug",
		"FORGE_OUTPUT_FORMAT": "json",
	})

	first, err1 := OverrideFromEnv(cfg, env)
	second, err2 := OverrideFromEnv(cfg, env)

	if err1 != nil || err2 != nil {
		t.Fatalf("errors: first=%v second=%v", err1, err2)
	}
	if first != second {
		t.Errorf("not deterministic: first=%+v second=%+v",
			first, second)
	}
}

// TestOverrideFromEnv_PrefersEarlierLayerOverConfigFile
// verifies the precedence chain's middle-layer property: an
// environment variable overrides a config file's value for the
// same field.
//
// The test constructs a Config that represents a file's load
// (LogLevel: warn), applies the env override (FORGE_LOG_LEVEL=
// debug), and asserts the result is the env's value (debug).
func TestOverrideFromEnv_PrefersEarlierLayerOverConfigFile(t *testing.T) {
	t.Parallel()

	// The Config represents a loaded config file with
	// log_level: warn.
	cfg := Config{LogLevel: LogLevelWarn, OutputFormat: OutputFormatText}

	// The env sets log_level: debug. The env's value wins.
	env := envFunc(map[string]string{
		"FORGE_LOG_LEVEL": "debug",
	})

	got, err := OverrideFromEnv(cfg, env)
	if err != nil {
		t.Fatalf("OverrideFromEnv: %v", err)
	}
	if got.LogLevel != LogLevelDebug {
		t.Errorf("LogLevel = %q; want %q (env should override file)",
			got.LogLevel, LogLevelDebug)
	}
}

// TestOverrideFromEnv_YieldsToLaterLayer verifies the precedence
// chain's middle-layer property from the other direction: an
// empty or unset environment variable leaves the config file's
// value in place, so a later layer (a future flag resolution) can
// override it.
//
// The test is the mirror of
// TestOverrideFromEnv_PrefersEarlierLayerOverConfigFile.
func TestOverrideFromEnv_YieldsToLaterLayer(t *testing.T) {
	t.Parallel()

	// The Config represents a loaded config file with
	// log_level: warn.
	cfg := Config{LogLevel: LogLevelWarn, OutputFormat: OutputFormatText}

	// The env sets nothing. The config file's value stays in
	// place, ready for a later layer to override.
	env := envFunc(nil)

	got, err := OverrideFromEnv(cfg, env)
	if err != nil {
		t.Fatalf("OverrideFromEnv: %v", err)
	}
	if got.LogLevel != LogLevelWarn {
		t.Errorf("LogLevel = %q; want %q (env is unset; file's "+
			"value stays)", got.LogLevel, LogLevelWarn)
	}
}
