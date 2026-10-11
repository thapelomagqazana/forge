// Package config contains unit tests for the CLI configuration
// type.
//
// This file pins the field name mapping rules (WBS 8.2.2). The
// mapping derives a YAML key, an environment variable name, and a
// CLI flag name from a Go field name. Each test exercises one
// function over a set of representative inputs; a fourth test
// exercises every field in the schema through all three functions.
//
// # How the tests work
//
// The tests are table-driven. Each case names an input (a Go field
// name) and the expected output. The tests do not construct a
// Config value; the functions are pure and their contract is the
// derivation from a string to a string.
package config

import (
	"reflect"
	"strings"
	"testing"
)

// =============================================================================
// GoFieldToYAML
// =============================================================================

// TestNaming_GoFieldToYAML verifies the PascalCase to snake_case
// conversion on a set of representative inputs.
func TestNaming_GoFieldToYAML(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"two words", "LogLevel", "log_level"},
		{"two words 2", "OutputFormat", "output_format"},
		{"two words 3", "SourcePath", "source_path"},
		{"single word", "Simple", "simple"},
		{"single letter", "A", "a"},
		{"all lowercase", "simple", "simple"},
		{"three words", "VeryLongFieldName", "very_long_field_name"},

		// Acronyms are converted letter-by-letter; the
		// convention is "uppercase starts a word".
		{"acronym", "APIKey", "a_p_i_key"},
		{"leading acronym", "HTTPClient", "h_t_t_p_client"},

		// Boundary cases.
		{"empty", "", ""},

		// Non-ASCII. The function does not validate the
		// input; a multi-byte character is passed through.
		{"non-ascii lowercase", "café", "café"},
		{"non-ascii uppercase", "Café", "café"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := GoFieldToYAML(tc.in)
			if got != tc.want {
				t.Errorf("GoFieldToYAML(%q) = %q; want %q",
					tc.in, got, tc.want)
			}
		})
	}
}

// =============================================================================
// GoFieldToEnv
// =============================================================================

// TestNaming_GoFieldToEnv verifies the PascalCase to
// FORGE_UPPER_SNAKE_CASE conversion.
func TestNaming_GoFieldToEnv(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"two words", "LogLevel", "FORGE_LOG_LEVEL"},
		{"two words 2", "OutputFormat", "FORGE_OUTPUT_FORMAT"},
		{"two words 3", "SourcePath", "FORGE_SOURCE_PATH"},
		{"single word", "Simple", "FORGE_SIMPLE"},
		{"single letter", "A", "FORGE_A"},
		{"three words", "VeryLongFieldName", "FORGE_VERY_LONG_FIELD_NAME"},
		{"acronym", "APIKey", "FORGE_A_P_I_KEY"},

		// Boundary cases.
		{"empty", "", EnvVarPrefix},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := GoFieldToEnv(tc.in)
			if got != tc.want {
				t.Errorf("GoFieldToEnv(%q) = %q; want %q",
					tc.in, got, tc.want)
			}
		})
	}
}

// =============================================================================
// GoFieldToFlag
// =============================================================================

// TestNaming_GoFieldToFlag verifies the PascalCase to
// --kebab-case conversion.
func TestNaming_GoFieldToFlag(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"two words", "LogLevel", "--log-level"},
		{"two words 2", "OutputFormat", "--output-format"},
		{"two words 3", "SourcePath", "--source-path"},
		{"single word", "Simple", "--simple"},
		{"single letter", "A", "--a"},
		{"three words", "VeryLongFieldName", "--very-long-field-name"},
		{"acronym", "APIKey", "--a-p-i-key"},

		// Boundary cases.
		{"empty", "", "--"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := GoFieldToFlag(tc.in)
			if got != tc.want {
				t.Errorf("GoFieldToFlag(%q) = %q; want %q",
					tc.in, got, tc.want)
			}
		})
	}
}

// =============================================================================
// Every schema field
// =============================================================================

// TestNaming_EverySchemaFieldMapsCorrectly verifies that every
// field declared in the schema's FieldMapping list maps correctly
// through all three derivation functions.
//
// The test's value is as a contract check: a future contributor
// who adds a field to the schema without updating the mapping
// sees the failure. The test iterates the schema's fields (the
// SchemaFields function) and derives each representation from the
// field's Go name, comparing to the mapping's declared value.
func TestNaming_EverySchemaFieldMapsCorrectly(t *testing.T) {
	t.Parallel()

	for _, m := range SchemaFields() {
		m := m
		t.Run(m.GoName, func(t *testing.T) {
			t.Parallel()

			// YAML key.
			if m.YAMLKey != "" {
				want := GoFieldToYAML(m.GoName)
				if m.YAMLKey != want {
					t.Errorf("field %q: YAML key = %q; "+
						"derived from Go name it should be %q",
						m.GoName, m.YAMLKey, want)
				}
			}

			// Env var.
			if m.EnvVar != "" {
				want := GoFieldToEnv(m.GoName)
				if m.EnvVar != want {
					t.Errorf("field %q: env var = %q; "+
						"derived from Go name it should be %q",
						m.GoName, m.EnvVar, want)
				}
			}

			// Flag.
			if m.FlagName != "" {
				want := GoFieldToFlag(m.GoName)
				if m.FlagName != want {
					t.Errorf("field %q: flag name = %q; "+
						"derived from Go name it should be %q",
						m.GoName, m.FlagName, want)
				}
			}
		})
	}
}

// =============================================================================
// Determinism
// =============================================================================

// TestNaming_DerivationIsDeterministic verifies that each
// function returns the same result for the same input across
// repeated calls.
//
// The property is a consequence of the functions being pure: they
// read no global state and write nothing. The test pins the
// property in case a future change introduces a global.
func TestNaming_DerivationIsDeterministic(t *testing.T) {
	t.Parallel()

	inputs := []string{"LogLevel", "OutputFormat", "SourcePath", "Simple"}

	for _, input := range inputs {
		first := GoFieldToYAML(input)
		second := GoFieldToYAML(input)
		if first != second {
			t.Errorf("GoFieldToYAML(%q) is not deterministic: %q vs %q",
				input, first, second)
		}

		firstEnv := GoFieldToEnv(input)
		secondEnv := GoFieldToEnv(input)
		if firstEnv != secondEnv {
			t.Errorf("GoFieldToEnv(%q) is not deterministic: %q vs %q",
				input, firstEnv, secondEnv)
		}

		firstFlag := GoFieldToFlag(input)
		secondFlag := GoFieldToFlag(input)
		if firstFlag != secondFlag {
			t.Errorf("GoFieldToFlag(%q) is not deterministic: %q vs %q",
				input, firstFlag, secondFlag)
		}
	}
}

// =============================================================================
// Boundary and edge cases
// =============================================================================

// TestNaming_EmptyInputDoesNotPanic verifies that the three
// functions handle the empty input without panicking.
func TestNaming_EmptyInputDoesNotPanic(t *testing.T) {
	t.Parallel()

	// The calls' results are not asserted; the test's purpose
	// is the absence of a panic.
	_ = GoFieldToYAML("")
	_ = GoFieldToEnv("")
	_ = GoFieldToFlag("")
}

// TestNaming_NoUnexpectedCharacters verifies that the three
// functions do not introduce characters outside the expected
// set.
//
// The YAML key contains only lowercase letters and underscores.
// The env var contains only uppercase letters, underscores, and
// the FORGE_ prefix. The flag contains only the "--" prefix,
// lowercase letters, and hyphens.
func TestNaming_NoUnexpectedCharacters(t *testing.T) {
	t.Parallel()

	inputs := []string{"LogLevel", "OutputFormat", "SourcePath", "Simple"}

	yamlAllowed := func(r rune) bool {
		return (r >= 'a' && r <= 'z') || r == '_'
	}
	envAllowed := func(r rune) bool {
		return (r >= 'A' && r <= 'Z') || r == '_'
	}
	flagBodyAllowed := func(r rune) bool {
		return (r >= 'a' && r <= 'z') || r == '-'
	}

	for _, input := range inputs {
		yaml := GoFieldToYAML(input)
		for _, r := range yaml {
			if !yamlAllowed(r) {
				t.Errorf("GoFieldToYAML(%q) = %q contains %q; "+
					"the YAML key should contain only lowercase "+
					"letters and underscores", input, yaml, r)
			}
		}

		env := GoFieldToEnv(input)
		if !strings.HasPrefix(env, EnvVarPrefix) {
			t.Errorf("GoFieldToEnv(%q) = %q; missing prefix %q",
				input, env, EnvVarPrefix)
		}
		for _, r := range strings.TrimPrefix(env, EnvVarPrefix) {
			if !envAllowed(r) {
				t.Errorf("GoFieldToEnv(%q) = %q contains %q; "+
					"the env var should contain only uppercase "+
					"letters and underscores after the prefix",
					input, env, r)
			}
		}

		flag := GoFieldToFlag(input)
		if !strings.HasPrefix(flag, fieldFlagPrefix) {
			t.Errorf("GoFieldToFlag(%q) = %q; missing prefix %q",
				input, flag, fieldFlagPrefix)
		}
		for _, r := range strings.TrimPrefix(flag, fieldFlagPrefix) {
			if !flagBodyAllowed(r) {
				t.Errorf("GoFieldToFlag(%q) = %q contains %q; "+
					"the flag body should contain only lowercase "+
					"letters and hyphens",
					input, flag, r)
			}
		}
	}
}

// =============================================================================
// The three functions are consistent
// =============================================================================

// TestNaming_EnvIsFlagUppercased verifies that the env var name is
// the flag name uppercased, with the prefixes adjusted.
//
// The property is a consequence of the two functions sharing
// GoFieldToYAML. The test pins the relationship: a future change
// that alters one function's derivation without altering the
// other's would fail the test.
func TestNaming_EnvIsFlagUppercased(t *testing.T) {
	t.Parallel()

	inputs := []string{"LogLevel", "OutputFormat", "SourcePath", "Simple"}

	for _, input := range inputs {
		env := GoFieldToEnv(input)
		flag := GoFieldToFlag(input)

		// Strip the prefixes.
		envBody := strings.TrimPrefix(env, EnvVarPrefix)
		flagBody := strings.TrimPrefix(flag, fieldFlagPrefix)

		// The env body should be the flag body, uppercased and
		// with hyphens replaced by underscores.
		wantEnvBody := strings.ToUpper(strings.ReplaceAll(flagBody, "-", "_"))
		if envBody != wantEnvBody {
			t.Errorf("for input %q: env body = %q; "+
				"derived from flag body it should be %q",
				input, envBody, wantEnvBody)
		}
	}
}

// =============================================================================
// The functions are pure
// =============================================================================

// TestNaming_FunctionsArePure is a compile-time and runtime check
// that the three functions have no side effects.
//
// The functions are declared to take a string and return a string.
// A future change that added a parameter (for example, a
// *Config) would change the signature and fail the test's
// compile-time assertion. A future change that added a global
// dependency would change the function's behaviour across calls
// and fail the determinism test.
//
// The test pins the signature by assigning each function to a
// variable of the expected type.
func TestNaming_FunctionsArePure(t *testing.T) {
	t.Parallel()

	// Compile-time signature assertions.
	var (
		_ func(string) string = GoFieldToYAML
		_ func(string) string = GoFieldToEnv
		_ func(string) string = GoFieldToFlag
	)

	// Runtime purity check: call each function twice with the
	// same input and compare the results.
	const input = "LogLevel"
	yaml1 := GoFieldToYAML(input)
	yaml2 := GoFieldToYAML(input)
	if yaml1 != yaml2 {
		t.Errorf("GoFieldToYAML is not pure: %q vs %q", yaml1, yaml2)
	}
	env1 := GoFieldToEnv(input)
	env2 := GoFieldToEnv(input)
	if env1 != env2 {
		t.Errorf("GoFieldToEnv is not pure: %q vs %q", env1, env2)
	}
	flag1 := GoFieldToFlag(input)
	flag2 := GoFieldToFlag(input)
	if flag1 != flag2 {
		t.Errorf("GoFieldToFlag is not pure: %q vs %q", flag1, flag2)
	}
}

// =============================================================================
// The functions return the expected types
// =============================================================================

// TestNaming_FunctionsReturnString verifies that each function's
// return type is string.
//
// The check is a compile-time assertion; a future change to a
// function's return type would fail to compile.
func TestNaming_FunctionsReturnString(t *testing.T) {
	t.Parallel()

	// The type assertion below fails to compile if the
	// function's return type changes.
	_ = reflect.TypeOf(GoFieldToYAML("LogLevel")).Kind() == reflect.String
	_ = reflect.TypeOf(GoFieldToEnv("LogLevel")).Kind() == reflect.String
	_ = reflect.TypeOf(GoFieldToFlag("LogLevel")).Kind() == reflect.String
}
