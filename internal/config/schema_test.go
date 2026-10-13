// Package config contains unit tests for the CLI configuration
// type.
//
// This file pins the CLI configuration schema (WBS 8.2.1) and the
// field name mapping (WBS 8.2.2). The schema names the Config
// type's value fields and their representations; the mapping
// derives the non-Go representations from the Go field name.
//
// # Value fields, sections, and bookkeeping fields
//
// ADR-009 (extended for the schema contract) distinguishes value
// fields from bookkeeping fields; ADR-022 adds sections as a
// third category. Only scalar value fields and sections
// participate in the schema:
//
//   - Scalar value fields (exported, string) have YAML keys,
//     environment variables, and flag names. They are returned by
//     SchemaFields().
//
//   - Sections (exported, pointer to a struct defined in this
//     package) have a top-level YAML key. They are file-only: no
//     env var, no flag. They are returned by
//     SchemaSectionFields().
//
//   - Bookkeeping fields (unexported, such as `sources`) have no
//     external representation and are excluded from every schema
//     check.
//
// The tests iterate exported fields (the scalar value fields and
// sections) when checking schema properties; unexported fields
// are not schema participants.
//
// # How the tests work
//
// The tests read the Config type via the reflect package and the
// schema's mappings via SchemaFields and SchemaSectionFields. The
// type's fields, their tags, and their kinds are available at
// runtime; the tests compare them to the schema's expectations. A
// future contributor who changes the type without updating the
// schema fails the tests.
package config

import (
	"reflect"
	"strings"
	"testing"
)

// =============================================================================
// The struct tags
// =============================================================================

// TestSchema_ConfigFieldsHaveYAMLTags verifies that every exported
// Config field carries a `yaml` struct tag.
//
// # Why exported fields
//
// ADR-009 (extended for the schema contract) distinguishes value
// fields from bookkeeping fields; ADR-022 adds sections. Both
// scalar value fields and sections are exported and carry YAML
// tags. Bookkeeping fields (such as `sources`) are unexported and
// have no YAML representation. The test iterates exported fields,
// which are exactly the scalar value fields and the sections.
//
// The YAML tags are the schema's machine-readable form. A future
// loader uses them to map YAML keys to fields. A value field or
// section without a tag would be silently omitted from the
// mapping; the check ensures every exported field is mapped.
func TestSchema_ConfigFieldsHaveYAMLTags(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeOf(Config{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}
		if _, ok := field.Tag.Lookup("yaml"); !ok {
			t.Errorf("Config exported field %q has no yaml struct tag; "+
				"every value field and section must declare its "+
				"mapping (WBS 8.2.1)", field.Name)
		}
	}
}

// TestSchema_YAMLTagsAreSnakeCase verifies that every Config
// exported field's YAML tag is either "-" (excluded) or a
// snake_case key.
//
// # Why exported fields
//
// Bookkeeping fields (ADR-009) have no YAML tag; they are not
// part of the schema. The test iterates exported fields, which
// are the scalar value fields and the sections.
func TestSchema_YAMLTagsAreSnakeCase(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeOf(Config{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}
		tag := field.Tag.Get("yaml")

		if tag == "-" {
			continue
		}

		key, _, _ := strings.Cut(tag, ",")

		if !IsSnakeCase(key) {
			t.Errorf("Config exported field %q has YAML tag %q; "+
				"the key %q is not snake_case (WBS 8.2.1)",
				field.Name, tag, key)
		}
	}
}

// TestSchema_SectionYAMLTagsAreSnakeCase verifies that every
// section's YAML tag is a snake_case key (not "-"; a section with
// "-" would violate TestSchema_SectionMappingsAreFileOnly).
//
// The test mirrors TestSchema_YAMLTagsAreSnakeCase for the
// section category. It reads the section list from
// SchemaSectionFields, so it stays in sync with the schema
// without hard-coding the section names.
func TestSchema_SectionYAMLTagsAreSnakeCase(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeOf(Config{})
	for _, m := range SchemaSectionFields() {
		field, ok := typ.FieldByName(m.GoName)
		if !ok {
			// Missing-field is caught by
			// TestSchema_FieldNameMapping; skip here.
			continue
		}
		tag := field.Tag.Get("yaml")
		if tag == "-" {
			t.Errorf("section %q has yaml tag %q; sections must "+
				"have a snake_case key (ADR-022)", m.GoName, tag)
			continue
		}
		key, _, _ := strings.Cut(tag, ",")
		if !IsSnakeCase(key) {
			t.Errorf("section %q has YAML tag %q; the key %q is "+
				"not snake_case (ADR-022)", m.GoName, tag, key)
		}
	}
}

// TestSchema_SourcePathIsNotReadFromYAML verifies that the
// SourcePath field carries the `yaml:"-"` tag.
//
// SourcePath is a scalar value field (it is exported and carries
// configuration-related metadata), so it participates in the
// schema. Its tag declares that it is not read from the YAML
// document; the field is a resolution result, not a config value.
//
// The test does not need to skip bookkeeping fields: it looks up
// a specific field by name, and that field is a scalar value
// field.
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

// TestSchema_FieldNameMapping verifies that each Config scalar
// value field's YAML key in the type's tag matches the schema's
// derivation, that each section's YAML tag matches its section
// mapping, and that the schema declares no bookkeeping fields.
//
// # Why value fields and sections, not bookkeeping fields
//
// ADR-009 (extended) excludes bookkeeping fields from the
// schema; ADR-022 adds sections as a separate category.
// SchemaFields() describes the scalar value fields;
// SchemaSectionFields() describes the sections. The test
// iterates only exported fields when cross-checking tags, and
// asserts that no unexported field appears in either mapping
// list.
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

	valueMappings := SchemaFields()
	byValueGoName := make(map[string]FieldMapping, len(valueMappings))
	for _, m := range valueMappings {
		byValueGoName[m.GoName] = m
	}
	sectionMappings := SchemaSectionFields()
	bySectionGoName := make(map[string]FieldMapping, len(sectionMappings))
	for _, m := range sectionMappings {
		bySectionGoName[m.GoName] = m
	}

	typ := reflect.TypeOf(Config{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}

		// A field is either a scalar value field (in
		// SchemaFields) or a section (in
		// SchemaSectionFields). Exactly one must claim it.
		m, inValues := byValueGoName[field.Name]
		sm, inSections := bySectionGoName[field.Name]
		switch {
		case inValues && inSections:
			t.Errorf("Config field %q is declared in both "+
				"SchemaFields() and SchemaSectionFields()", field.Name)
			continue
		case !inValues && !inSections:
			t.Errorf("Config exported field %q is in neither "+
				"SchemaFields() nor SchemaSectionFields(); "+
				"add the field's mapping and update the schema test",
				field.Name)
			continue
		case inSections:
			m = sm
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
				"the schema says %q",
				field.Name, tagKey, m.YAMLKey)
		}
	}

	// Every declared scalar-value-field mapping must name an
	// exported field.
	for _, m := range valueMappings {
		field, ok := typ.FieldByName(m.GoName)
		if !ok {
			t.Errorf("SchemaFields() declares a mapping for %q, "+
				"but Config has no such field", m.GoName)
			continue
		}
		if !field.IsExported() {
			t.Errorf("SchemaFields() declares a mapping for %q, "+
				"but the field is unexported (a bookkeeping "+
				"field); the schema describes only value fields "+
				"(ADR-009)", m.GoName)
		}
	}

	// Every declared section mapping must name an exported
	// field.
	for _, m := range sectionMappings {
		field, ok := typ.FieldByName(m.GoName)
		if !ok {
			t.Errorf("SchemaSectionFields() declares a mapping for "+
				"%q, but Config has no such field", m.GoName)
			continue
		}
		if !field.IsExported() {
			t.Errorf("SchemaSectionFields() declares a mapping for "+
				"%q, but the field is unexported", m.GoName)
		}
	}
}

// TestSchema_NoUnexpectedFields verifies that Config has no
// exported field that is missing from both SchemaFields() and
// SchemaSectionFields().
//
// # Why both lists
//
// Under ADR-022, an exported Config field is either a scalar
// value field (declared in SchemaFields) or a section (declared
// in SchemaSectionFields). A bookkeeping field is not expected to
// be in either list; its existence is recorded by
// InScopeBookkeepingFieldNames in scope.go. The test iterates
// only exported fields.
func TestSchema_NoUnexpectedFields(t *testing.T) {
	t.Parallel()

	expected := make(map[string]bool)
	for _, m := range SchemaFields() {
		expected[m.GoName] = true
	}
	for _, m := range SchemaSectionFields() {
		expected[m.GoName] = true
	}

	typ := reflect.TypeOf(Config{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}
		if !expected[field.Name] {
			t.Errorf("Config exported field %q is not declared in "+
				"either SchemaFields() or SchemaSectionFields()\n"+
				"adding an exported field requires an ADR (WBS 8.2.1)",
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
// The test iterates SchemaFields() and SchemaSectionFields(); the
// former contains only scalar value fields, the latter only
// sections. No bookkeeping field appears in either, so no skip is
// needed here.
//
// For sections, only the YAMLKey is derived; EnvVar and FlagName
// must be empty, and their emptiness is asserted by
// TestSchema_SectionMappingsAreFileOnly rather than by derivation.
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

	for _, m := range SchemaSectionFields() {
		m := m
		t.Run("section/"+m.GoName, func(t *testing.T) {
			t.Parallel()

			if m.YAMLKey != "" && m.YAMLKey != GoFieldToYAML(m.GoName) {
				t.Errorf("section %q: YAMLKey = %q; derived = %q",
					m.GoName, m.YAMLKey, GoFieldToYAML(m.GoName))
			}
			// Sections are file-only; EnvVar and FlagName
			// must be empty. Their emptiness is asserted
			// by TestSchema_SectionMappingsAreFileOnly,
			// not by derivation.
		})
	}
}

// TestSchema_MetadataFieldHasNoRepresentations verifies that the
// metadata field (SourcePath) has empty values for its three
// non-Go representations.
//
// The field is not part of the YAML document, has no environment
// variable, and has no flag. The test pins the emptiness.
//
// The test looks up SourcePath explicitly; it does not iterate
// all fields. It is unaffected by ADR-009 and ADR-022.
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

// TestSchema_SectionMappingsAreFileOnly verifies that every
// section mapping has a non-empty YAMLKey and empty EnvVar and
// FlagName.
//
// # Why sections are file-only
//
// A section (ADR-022) is an exported Config field whose type is a
// pointer to a struct defined in this package. Its sensitive
// members are declared `sensitive: true` in the schema and are
// read from the file only. Exposing a section via an environment
// variable or a flag would put the sensitive member values into
// the process environment or the command line, where they are
// visible to other processes (the environment) or to any process
// on the machine (the command line via ps). The schema's contract
// is that sections are file-only.
//
// A section mapping with an empty YAMLKey is also a violation:
// the section's members could not be loaded from a file if the
// section itself had no key.
func TestSchema_SectionMappingsAreFileOnly(t *testing.T) {
	t.Parallel()

	for _, m := range SchemaSectionFields() {
		if m.YAMLKey == "" {
			t.Errorf("section %q has empty YAMLKey; sections must "+
				"have a top-level YAML key (ADR-022)", m.GoName)
		}
		if m.EnvVar != "" {
			t.Errorf("section %q has EnvVar %q; sections are "+
				"file-only (ADR-022)", m.GoName, m.EnvVar)
		}
		if m.FlagName != "" {
			t.Errorf("section %q has FlagName %q; sections are "+
				"file-only (ADR-022)", m.GoName, m.FlagName)
		}
	}
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
		{"registry", true},
		{"telemetry", true},
		{"api_key", true},
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
//
// The test is not affected by ADR-009 or ADR-022: SchemaFields()
// describes only scalar value fields, and SchemaFieldCount()
// returns its length. Sections are counted separately (there is
// no SchemaSectionFieldCount function; a caller reads
// len(SchemaSectionFields()) directly).
func TestSchema_SchemaFieldCountMatches(t *testing.T) {
	t.Parallel()

	if got, want := SchemaFieldCount(), len(SchemaFields()); got != want {
		t.Errorf("SchemaFieldCount() = %d; len(SchemaFields()) = %d",
			got, want)
	}
}
