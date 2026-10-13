// Package config defines the CLI configuration for Forge.
//
// This file holds the CLI configuration schema (WBS 8.2.1) and
// the field name mapping (WBS 8.2.2). The schema names the Config
// type's fields and their representations; the mapping derives the
// non-Go representations from the Go field name.
//
// # The schema
//
// Scalar value fields:
//
//	Field         Go name        YAML key        Env var              Flag
//	LogLevel      LogLevel       log_level       FORGE_LOG_LEVEL      --log-level
//	OutputFormat  OutputFormat   output_format   FORGE_OUTPUT_FORMAT  --output-format
//	SourcePath    SourcePath     (metadata)      (metadata)           (metadata)
//
// The first two scalar value fields are configuration values; the
// third is metadata. The metadata field has no YAML key, no
// environment variable, and no flag; the schema's FieldMapping for
// SourcePath leaves those three fields empty.
//
// Sections (WBS 12.7.1, recognized by ADR-022):
//
//	Field      Go name    YAML key   Env var     Flag
//	Registry   Registry   registry   (none)      (none)
//	Telemetry  Telemetry  telemetry  (none)      (none)
//
// A section is an exported Config field whose type is a pointer to
// a struct defined in this package. The section's own YAML key
// names the nested mapping; the members inside the section
// (Registry.URL, Registry.Token, ...) have their own YAML tags and
// are not declared here. Sections are file-only: neither the
// section itself nor its members has an environment variable or a
// flag, because the sensitive members are declared `sensitive:
// true` in the schema and are read from the file only.
//
// # Why the mapping is derived, not declared
//
// An earlier draft of the schema declared each representation as
// a string literal in the FieldMapping list. With the naming
// functions (WBS 8.2.2), the representations are derived from the
// Go field name. The derivation is the single source of truth for
// the conventions; a future change to a convention changes one
// function, not every mapping.
//
// # No package-level mutable state
//
// This file declares a type and functions. It declares no
// package-level variables.
package config

import "strings"

// FieldMapping describes the mapping from a Config field's Go name
// to its YAML key, environment variable name, and CLI flag name.
//
// # Why a struct
//
// The four names are related but distinct. A struct groups them
// so that a caller (the schema test, a future documentation
// generator, a future loader) can iterate the mappings as a
// unit. A map from Go name to a slice would work but would be
// less legible.
//
// # Why FlagName
//
// WBS 8.2.2 added the CLI flag name as the fourth representation.
// The field's value is the flag's long name, including the "--"
// prefix. The field's name (FlagName) matches the schema's
// vocabulary.
//
// # Sections
//
// A FieldMapping for a section (see SchemaSectionFields) uses the
// same shape. The section's YAMLKey is the top-level key of the
// nested mapping; the section's EnvVar and FlagName are empty
// because sections are file-only. The struct is shared between
// scalar value fields and sections so that a caller that walks
// either list uses one type.
type FieldMapping struct {
	// GoName is the field's name in the Go source. It uses
	// PascalCase.
	GoName string

	// YAMLKey is the field's key in the YAML config file. It
	// uses snake_case. An empty YAMLKey means the field is not
	// part of the YAML document.
	YAMLKey string

	// EnvVar is the field's environment variable name. It uses
	// UPPER_SNAKE_CASE with the FORGE_ prefix. An empty EnvVar
	// means the field has no environment variable.
	EnvVar string

	// FlagName is the field's CLI flag name. It uses kebab-case
	// with the "--" prefix. An empty FlagName means the field
	// has no flag.
	FlagName string
}

// SchemaFields returns the mappings for every scalar value field
// of the Config type, in the order the fields appear in the type.
// It does not return sections; those are returned by
// SchemaSectionFields.
//
// # What the function returns
//
// The function returns one FieldMapping per scalar value field.
// The order matches the type's declaration order; a caller that
// iterates the mappings gets a deterministic result.
//
// # How the mappings are derived
//
// For the two configuration fields (LogLevel, OutputFormat), the
// function derives the YAML key, the environment variable name,
// and the flag name from the Go field name using the three
// derivation functions (GoFieldToYAML, GoFieldToEnv,
// GoFieldToFlag). The metadata field (SourcePath) has empty
// values for the three non-Go representations.
//
// The derivation is deliberate: the schema's representations are
// a function of the Go field name. A future change to a naming
// convention (for example, a different env var prefix) changes
// one function; the schema follows.
//
// # Why a function, not a variable
//
// The CLI package's contract test forbids package-level
// variables. A function that returns a fresh slice on each call
// has no package-level state.
func SchemaFields() []FieldMapping {
	return []FieldMapping{
		{
			GoName:   "LogLevel",
			YAMLKey:  GoFieldToYAML("LogLevel"),
			EnvVar:   GoFieldToEnv("LogLevel"),
			FlagName: GoFieldToFlag("LogLevel"),
		},
		{
			GoName:   "OutputFormat",
			YAMLKey:  GoFieldToYAML("OutputFormat"),
			EnvVar:   GoFieldToEnv("OutputFormat"),
			FlagName: GoFieldToFlag("OutputFormat"),
		},
		{
			GoName: "SourcePath",
			// The metadata field has no YAML key, no
			// environment variable, and no flag. The three
			// empty strings declare that.
		},
	}
}

// SchemaSectionFields returns the mappings for every section of
// the Config type, in the order the fields appear in the type.
//
// # What the function returns
//
// The function returns one FieldMapping per section. A section is
// an exported Config field whose type is a pointer to a struct
// defined in this package (see scope.go's InScopeSectionNames).
// The mapping's YAMLKey is the section's top-level YAML key
// (derived from the Go field name by GoFieldToYAML). The mapping's
// EnvVar and FlagName are empty: sections are file-only, because
// their sensitive members are declared `sensitive: true` in the
// schema and are read from the file only.
//
// # What the function does not return
//
// The function does not return mappings for the members inside a
// section (Registry.Token, Telemetry.APIKey, ...). Those members
// have their own YAML tags on the member fields of the section
// structs (RegistryConfig, TelemetryConfig); the loader reads
// them by reflection. Declaring them here would duplicate the
// member tags and couple this list to the section structs' shapes.
//
// # Why a function, not a variable
//
// The CLI package's contract test forbids package-level
// variables. A function that returns a fresh slice on each call
// has no package-level state.
func SchemaSectionFields() []FieldMapping {
	return []FieldMapping{
		{
			GoName:  "Registry",
			YAMLKey: GoFieldToYAML("Registry"),
			// Sections are file-only: no env var, no flag.
		},
		{
			GoName:  "Telemetry",
			YAMLKey: GoFieldToYAML("Telemetry"),
			// Sections are file-only: no env var, no flag.
		},
	}
}

// SchemaFieldCount returns the number of scalar value fields the
// schema declares.
//
// # Why a function
//
// The function's value is the length of SchemaFields(). A caller
// that wants the count without the mappings reads the function.
// A caller that wants the mappings reads SchemaFields.
//
// The count does not include sections; see SchemaSectionFields
// for those. The count is used by the schema test; a future
// change to the schema's scalar value-field set updates the
// test's expectation.
func SchemaFieldCount() int {
	return len(SchemaFields())
}

// IsSnakeCase reports whether s is a valid snake_case key.
//
// # The rule
//
// A snake_case key consists of lowercase ASCII letters, digits,
// and underscores. It starts with a lowercase letter and does not
// end with an underscore. Examples: "log_level", "output_format",
// "a_b_c", "x1_y2", "log_level1". Counterexamples: "LogLevel",
// "log-level", "log__level", "_log_level", "log_level_",
// "1log_level".
//
// # Why digits are allowed
//
// A snake_case key may contain digits after the first character.
// A field name like "Level1" maps to "level1"; rejecting the
// digit would make the schema fail on a legitimate field name. The
// first character must be a letter: a key like "1log" is not
// valid snake_case.
//
// # Why the function exists
//
// The schema test checks that every YAML key is snake_case. The
// function is the check's implementation. A future contributor
// who wants to add a key with a different case sees the failure
// and either renames the key or opens an ADR.
//
// # The empty string
//
// The function returns false for the empty string. A caller that
// wants to allow an empty key (for a field that is not part of
// the YAML document) checks the empty case separately. The
// schema's SourcePath field has an empty YAML key and is handled
// by the test's explicit check.
//
// # Why the function does not use a regexp
//
// A regular expression would be shorter to write but would
// require a package-level compiled value (the CLI package's
// contract test forbids package-level variables) or a per-call
// compilation. The character-by-character loop is faster and
// has no package-level state.
func IsSnakeCase(s string) bool {
	if s == "" {
		return false
	}
	if s[0] < 'a' || s[0] > 'z' {
		return false
	}
	if s[len(s)-1] == '_' {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z':
			// ok
		case c >= '0' && c <= '9':
			// ok
		case c == '_':
			// ok
		default:
			return false
		}
	}
	// Reject two consecutive underscores.
	if strings.Contains(s, "__") {
		return false
	}
	return true
}
