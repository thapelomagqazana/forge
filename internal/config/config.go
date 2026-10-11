package config

// Config is the CLI configuration for Forge.
//
// # Scope
//
// The type is deliberately minimal. CLI configuration describes how
// the CLI behaves; it does not describe the project the CLI operates
// on. Project configuration belongs to the project configuration
// subsystem (Blueprint, Phase 3). See the package docstring for the
// full scope contract.
//
// # The frozen schema (WBS 8.2.1)
//
// The type has three fields: two configuration values and one piece
// of metadata.
//
//	Field           Type    Allowed values            Default  YAML key        Env var
//	LogLevel        string  debug, info, warn, error  "info"   log_level       FORGE_LOG_LEVEL
//	OutputFormat    string  text, json                "text"   output_format   FORGE_OUTPUT_FORMAT
//	SourcePath      string  (metadata)                ""       (not in file)   (not in env)
//
// The schema is frozen. Adding a field requires an ADR. See the
// schema test (schema_test.go) for the mechanical assertions.
//
// # Allowed values
//
// Both configuration fields hold strings from enumerated sets. The
// allowed values are declared as constants in this package. A future
// loader validates the file's values against the constants; an
// unknown value produces a validation error.
//
// The enumerations are case-insensitive on input, normalised to
// lowercase on output. The loader is responsible for the
// normalisation; the type holds whatever the loader gives it.
//
// # The metadata field
//
// SourcePath is not a configuration value. It is a resolution
// result: the loader writes it after resolving the file's path.
// The field's YAML tag is `yaml:"-"` to declare that it is not read
// from the file.
//
// An empty SourcePath means the config was not loaded from a file;
// the other fields hold their default values.
//
// # Why a value type
//
// The type has three string fields and no identity. Two Config
// values with the same field values are interchangeable. The type
// is used as a value in `Dependencies.Config` (see
// internal/cli/deps.go); a value type makes the type comparable
// with ==, which tests use, and avoids the nil-pointer question
// that a pointer type would introduce.
//
// # Why struct tags
//
// The two configuration fields carry `yaml:` struct tags. The tags
// are the schema's machine-readable form: a future loader uses
// them to map YAML keys to fields. The mapping is part of the
// schema; a change to a tag is a schema change and requires an
// ADR.
//
// The tags use snake_case, matching the schema's YAML key
// convention. SourcePath's tag is `yaml:"-"`, which declares that
// the field is not read from or written to the YAML document.
//
// # Why no json tags
//
// The type is not directly serialized to JSON. A future
// `forge config --format json` command serializes a
// command-specific view of the config, not the Config type itself.
// The view is a separate type in the command's package; it carries
// its own tags. Keeping the Config type free of JSON tags means the
// type's shape is not coupled to any command's output format.
type Config struct {
	// LogLevel controls the verbosity of diagnostic output.
	//
	// Allowed values are the LogLevel* constants:
	//
	//	"debug"  keep everything
	//	"info"   keep INFO, WARN, ERROR (the default)
	//	"warn"   keep WARN, ERROR
	//	"error"  keep ERROR only
	//
	// The zero value (empty string) means "use the default level",
	// which is LogLevelInfo.
	//
	// The value is not validated by this package. A future loader
	// validates it against the constants; a consumer that reads
	// the field treats an unrecognised value as the default.
	LogLevel string `yaml:"log_level"`

	// OutputFormat is the default format for command output.
	//
	// Allowed values are the OutputFormat* constants:
	//
	//	"text"  the machine-friendly line-oriented format
	//	"json"  the machine-readable JSON format
	//
	// The zero value (empty string) means "use the command's
	// default format".
	//
	// The value is not validated by this package. A future loader
	// validates it against the constants; a consumer that reads
	// the field treats an unrecognised value as the command's
	// default.
	OutputFormat string `yaml:"output_format"`

	// SourcePath is the path the configuration was loaded from.
	//
	// The field is metadata, not a configuration value. It is not
	// read from the config file; the loader writes it after
	// resolving the file's path. The YAML tag `-` declares that
	// the field is not part of the YAML document.
	//
	// An empty SourcePath means the config was not loaded from a
	// file; the other fields hold their default values.
	SourcePath string `yaml:"-"`
}
