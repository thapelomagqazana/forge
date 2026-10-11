// Package config contains unit tests for the CLI configuration
// type.
//
// This file pins the CLI configuration schema (WBS 8.2.1) and the
// field name mapping (WBS 8.2.2). The schema names the Config
// type's fields and their representations; the mapping derives the
// non-Go representations from the Go field name.
//
// # How the tests work
//
// The tests read the Config type via the reflect package and the
// schema's mappings via SchemaFields. The type's fields, their
// tags, and their kinds are available at runtime; the tests
// compare them to the schema's expectations. A future contributor
// who changes the type without updating the schema fails the
// tests.
package config

import (
	"reflect"
	"strings"
	"testing"
)

// =============================================================================
// The struct tags
// =============================================================================

// TestSchema_ConfigFieldsHaveYAMLTags verifies that every Config
// field carries a `yaml` struct tag.
//
// The YAML tags are the schema's machine-readable form. A future
// loader uses them to map YAML keys to fields. A field without a
// tag would be silently omitted from the mapping; the check
// ensures every field is mapped.
func TestSchema_ConfigFieldsHaveYAMLTags(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeOf(Config{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if _, ok := field.Tag.Lookup("yaml"); !ok {
			t.Errorf("Config field %q has no yaml struct tag; "+
				"every field must declare its mapping (WBS 8.2.1)",
				field.Name)
		}
	}
}

// TestSchema_YAMLTagsAreSnakeCase verifies that every Config
// field's YAML tag is either "-" (excluded) or a snake_case key.
func TestSchema_YAMLTagsAreSnakeCase(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeOf(Config{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		tag := field.Tag.Get("yaml")

		if tag == "-" {
			continue
		}

		key, _, _ := strings.Cut(tag, ",")

		if !IsSnakeCase(key) {
			t.Errorf("Config field %q has YAML tag %q; "+
				"the key %q is not snake_case (WBS 8.2.1)",
				field.Name, tag, key)
		}
	}
}

// TestSchema_SourcePathIsNotReadFromYAML verifies that the
// SourcePath field carries the `yaml:"-"` tag.
func TestSchema_SourcePathIsNotReadFromYAML(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeOf(Config{})
	field, ok := typ.FieldByName("SourcePath")
	if !ok {
		t.Fatal("Config has no field named SourcePath")
	}
	tag := field.Tag.Get("yaml")
	if tag != "-" {
		t.Errorf("SourcePath's yaml tag = %q; want %q\n"+
			"SourcePath is metadata, not a config value; the tag "+
			"declares that it is not part of the YAML document",
			tag, "-")
	}
}

// =============================================================================
// The field-name mapping
// =============================================================================

// TestSchema_FieldNameMapping verifies that each Config field's
// YAML key in the type's tag matches the schema's derivation.
//
// The test uses SchemaFields() as the source of truth and
// cross-checks it against the type's tags. A future contributor
// who renames a field or changes a tag without updating
// SchemaFields() fails the test.
//
// # The derivation
//
// WBS 8.2.2 introduced the three derivation functions
// (GoFieldToYAML, GoFieldToEnv, GoFieldToFlag). The schema's
// mappings are derived from the Go field names; the test asserts
// the derivation is what the type's tags declare.
//
// # The metadata field's tag
//
// A field that is not part of the YAML document has the struct
// tag `yaml:"-"`. The schema's mapping for such a field has an
// empty YAMLKey. The two representations are equivalent: both
// declare that the field has no YAML key. The test normalises
// the tag before comparing: a tag of "-" is treated as "".
func TestSchema_FieldNameMapping(t *testing.T) {
	t.Parallel()

	mappings := SchemaFields()
	byGoName := make(map[string]FieldMapping, len(mappings))
	for _, m := range mappings {
		byGoName[m.GoName] = m
	}

	typ := reflect.TypeOf(Config{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		m, ok := byGoName[field.Name]
		if !ok {
			t.Errorf("Config field %q is not in SchemaFields(); "+
				"add the field's mapping and update the schema test",
				field.Name)
			continue
		}

		// The type's YAML tag must match the mapping's
		// YAMLKey. A tag of "-" and an empty key are
		// equivalent: both declare that the field is not part
		// of the YAML document.
		tagKey, _, _ := strings.Cut(field.Tag.Get("yaml"), ",")
		if tagKey == "-" {
			tagKey = ""
		}
		if tagKey != m.YAMLKey {
			t.Errorf("Config field %q has yaml tag %q; "+
				"SchemaFields() says %q",
				field.Name, tagKey, m.YAMLKey)
		}
	}

	for _, m := range mappings {
		if _, ok := typ.FieldByName(m.GoName); !ok {
			t.Errorf("SchemaFields() declares a mapping for %q, "+
				"but Config has no such field", m.GoName)
		}
	}
}

// TestSchema_NoUnexpectedFields verifies that Config has no field
// that is not in SchemaFields().
func TestSchema_NoUnexpectedFields(t *testing.T) {
	t.Parallel()

	mappings := SchemaFields()
	expected := make(map[string]bool, len(mappings))
	for _, m := range mappings {
		expected[m.GoName] = true
	}

	typ := reflect.TypeOf(Config{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !expected[field.Name] {
			t.Errorf("Config field %q is not declared in "+
				"SchemaFields()\n"+
				"adding a field requires an ADR (WBS 8.2.1)",
				field.Name)
		}
	}
}

// =============================================================================
// The derived representations
// =============================================================================

// TestSchema_RepresentationsAreDerived verifies that each mapping's
// YAML key, env var name, and flag name are derived from the Go
// field name by the corresponding function.
//
// The test is the schema's contract for the derivation: a future
// contributor who hand-codes a representation (for example, sets
// YAMLKey to a string literal that does not match the derivation)
// sees the failure.
func TestSchema_RepresentationsAreDerived(t *testing.T) {
	t.Parallel()

	for _, m := range SchemaFields() {
		m := m
		t.Run(m.GoName, func(t *testing.T) {
			t.Parallel()

			if m.YAMLKey != "" && m.YAMLKey != GoFieldToYAML(m.GoName) {
				t.Errorf("field %q: YAMLKey = %q; derived = %q",
					m.GoName, m.YAMLKey, GoFieldToYAML(m.GoName))
			}
			if m.EnvVar != "" && m.EnvVar != GoFieldToEnv(m.GoName) {
				t.Errorf("field %q: EnvVar = %q; derived = %q",
					m.GoName, m.EnvVar, GoFieldToEnv(m.GoName))
			}
			if m.FlagName != "" && m.FlagName != GoFieldToFlag(m.GoName) {
				t.Errorf("field %q: FlagName = %q; derived = %q",
					m.GoName, m.FlagName, GoFieldToFlag(m.GoName))
			}
		})
	}
}

// TestSchema_MetadataFieldHasNoRepresentations verifies that the
// metadata field (SourcePath) has empty values for its three
// non-Go representations.
//
// The field is not part of the YAML document, has no environment
// variable, and has no flag. The test pins the emptiness.
func TestSchema_MetadataFieldHasNoRepresentations(t *testing.T) {
	t.Parallel()

	for _, m := range SchemaFields() {
		if m.GoName != "SourcePath" {
			continue
		}
		if m.YAMLKey != "" {
			t.Errorf("SourcePath's YAMLKey = %q; want empty",
				m.YAMLKey)
		}
		if m.EnvVar != "" {
			t.Errorf("SourcePath's EnvVar = %q; want empty",
				m.EnvVar)
		}
		if m.FlagName != "" {
			t.Errorf("SourcePath's FlagName = %q; want empty",
				m.FlagName)
		}
		return
	}
	t.Fatal("SchemaFields does not declare SourcePath")
}

// =============================================================================
// The allowed values
// =============================================================================

// TestSchema_AllowedLogLevelValues verifies that the LogLevel
// constants are the four values the schema declares.
func TestSchema_AllowedLogLevelValues(t *testing.T) {
	t.Parallel()

	values := []string{
		LogLevelDebug,
		LogLevelInfo,
		LogLevelWarn,
		LogLevelError,
	}
	want := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}

	if len(values) != len(want) {
		t.Fatalf("LogLevel has %d values; want %d",
			len(values), len(want))
	}
	for _, v := range values {
		if !want[v] {
			t.Errorf("LogLevel value %q is not in the schema's "+
				"allowed set", v)
		}
	}
}

// TestSchema_AllowedOutputFormatValues verifies that the
// OutputFormat constants are the two values the schema declares.
func TestSchema_AllowedOutputFormatValues(t *testing.T) {
	t.Parallel()

	values := []string{
		OutputFormatText,
		OutputFormatJSON,
	}
	want := map[string]bool{
		"text": true,
		"json": true,
	}

	if len(values) != len(want) {
		t.Fatalf("OutputFormat has %d values; want %d",
			len(values), len(want))
	}
	for _, v := range values {
		if !want[v] {
			t.Errorf("OutputFormat value %q is not in the schema's "+
				"allowed set", v)
		}
	}
}

// =============================================================================
// The snake_case predicate
// =============================================================================

// TestSchema_IsSnakeCase verifies the snake_case predicate on a
// set of representative inputs.
func TestSchema_IsSnakeCase(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want bool
	}{
		{"log_level", true},
		{"output_format", true},
		{"a", true},
		{"a_b_c", true},
		{"x1_y2", true},
		{"log_level1", true},

		{"LogLevel", false},
		{"logLevel", false},
		{"log-level", false},
		{"log level", false},
		{"log__level", false},
		{"_log_level", false},
		{"log_level_", false},
		{"", false},
		{"1log_level", false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()

			if got := IsSnakeCase(tc.in); got != tc.want {
				t.Errorf("IsSnakeCase(%q) = %v; want %v",
					tc.in, got, tc.want)
			}
		})
	}
}

// =============================================================================
// The schema field count
// =============================================================================

// TestSchema_SchemaFieldCountMatches verifies that
// SchemaFieldCount() returns the number of mappings
// SchemaFields() produces.
func TestSchema_SchemaFieldCountMatches(t *testing.T) {
	t.Parallel()

	if got, want := SchemaFieldCount(), len(SchemaFields()); got != want {
		t.Errorf("SchemaFieldCount() = %d; len(SchemaFields()) = %d",
			got, want)
	}
}
