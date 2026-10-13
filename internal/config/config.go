package config

import (
	"fmt"

	"github.com/thapelomagqazana/forge/internal/logger"
)

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
// # The frozen schema (WBS 8.2.1, WBS 12.7.1)
//
// The type has three top-level fields: two configuration values and
// one piece of metadata. Two nested sections — Registry and
// Telemetry — were added by WBS 12.7.1 for credential fields that
// are declared `sensitive: true` in the schema and are wrapped by
// the loader's redaction layer.
//
//	Field                Type             Allowed values            Default  YAML key        Env var
//	LogLevel             string           debug, info, warn, error  "info"   log_level       FORGE_LOG_LEVEL
//	OutputFormat         string           text, json                "text"   output_format   FORGE_OUTPUT_FORMAT
//	SourcePath           string           (metadata)                ""       (not in file)   (not in env)
//	Registry.URL         string           (free-form)               ""       registry.url    (not in env)
//	Registry.Token       string(sensitive) (free-form)              ""       registry.token  (not in env)
//	Registry.Password    string(sensitive) (free-form)              ""       registry.password (not in env)
//	Telemetry.APIKey     string(sensitive) (free-form)              ""       telemetry.api_key (not in env)
//
// The top-level schema is frozen. Adding a top-level field requires
// an ADR. See the schema test (schema_test.go) for the mechanical
// assertions.
//
// # Allowed values
//
// Both top-level configuration fields hold strings from enumerated
// sets. The allowed values are declared as constants in this
// package. The loader validates the file's values against the
// constants; an unknown value produces a validation error.
//
// The enumerations are case-insensitive on input, normalised to
// lowercase on output. The loader is responsible for the
// normalisation; the type holds whatever the loader gives it.
//
// # Sensitive fields (WBS 12.7.1)
//
// The Registry.Token, Registry.Password, and Telemetry.APIKey
// fields are declared `sensitive: true` in the schema. The loader
// (see internal/config/redact.go) wraps each value in a
// logger.Secret and clears the plain-string field. A caller that
// reads the plain field after loading sees the empty string; the
// value is accessible only via the corresponding Secret.Reveal.
//
// This is layer 3 of the four-layer redaction model documented in
// docs/security/logging-redaction.md §3. The type-level layer
// (internal/logger/secret.go) and the pattern-based layer
// (internal/logger/redact.go) are the other two; the test suite
// (test/security/redaction_test.go) is the fourth.
//
// # Why a value type
//
// The type has string fields and no identity. Two Config values
// with the same field values are interchangeable. The type is used
// as a value in `Dependencies.Config` (see internal/cli/deps.go); a
// value type makes the type comparable with ==, which tests use,
// and avoids the nil-pointer question that a pointer type would
// introduce.
//
// # Why struct tags
//
// The configuration fields carry `yaml:` struct tags. The tags are
// the schema's machine-readable form: the loader uses them to map
// YAML keys to fields. The mapping is part of the schema; a change
// to a tag is a schema change and requires an ADR.
//
// The tags use snake_case, matching the schema's YAML key
// convention. SourcePath's tag is `yaml:"-"`, which declares that
// the field is not read from or written to the YAML document. The
// Secret fields carry `yaml:"-"` for the same reason: they are
// populated by the loader, not by the file.
//
// # Why no json tags
//
// The type is not directly serialized to JSON. The `forge config
// --format json` command serializes a command-specific view of the
// config, not the Config type itself. The view is a separate type
// in the command's package; it carries its own tags. Keeping the
// Config type free of JSON tags means the type's shape is not
// coupled to any command's output format.
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
	// The value is not validated by this package. The loader
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
	// The value is not validated by this package. The loader
	// validates it against the constants; a consumer that reads
	// the field treats an unrecognised value as the command's
	// default.
	OutputFormat string `yaml:"output_format"`

	// Registry is the registry section of the config file.
	//
	// The section was added by WBS 12.7.1. It holds the registry
	// endpoint (a public identifier) and two sensitive fields
	// (Token and Password). A nil *RegistryConfig means the
	// section was absent from the file.
	Registry *RegistryConfig `yaml:"registry,omitempty"`

	// Telemetry is the telemetry section of the config file.
	//
	// The section was added by WBS 12.7.1. It holds a telemetry
	// API key (a sensitive field). A nil *TelemetryConfig means
	// the section was absent from the file.
	Telemetry *TelemetryConfig `yaml:"telemetry,omitempty"`

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

	// sources records the precedence layer that last set each
	// field, keyed by the field's Go name ("LogLevel",
	// "OutputFormat", "Registry.Token", "Telemetry.APIKey").
	//
	// # Why the map is unexported
	//
	// The map is an implementation detail of source tracking.
	// Exposing it would let a caller create a Config whose sources
	// disagree with its fields, or read a stale entry that no
	// longer matches the field's value. Callers read via SourceFor,
	// which enforces the zero-value semantics and centralizes the
	// map's shape.
	//
	// # Why the map is nil in the zero value
	//
	// A zero-value Config has no sources; every field is
	// SourceDefault. Making the map lazily initialized (see
	// setSource) means the zero value is usable without an
	// explicit make(), and SourceFor handles the nil case
	// uniformly with the "no entry" case.
	//
	// # Field naming
	//
	// Keys are dotted Go field names ("Registry.Token"), not YAML
	// keys ("registry.token"). The YAML key is a property of the
	// file (WBS 8.2.1); the Go field name is a property of this
	// struct. Using YAML keys would couple the source map to the
	// YAML schema, and a future schema change would silently break
	// source lookups.
	sources map[string]Source
}

// RegistryConfig is the registry section of the config file.
//
// # Sensitive fields
//
// Token and Password are declared `sensitive: true` in the schema.
// The loader wraps each value in a logger.Secret and clears the
// plain-string field. The plain fields are retained in the struct
// definition so the loader can read them from YAML; callers must
// read the Secret companions instead.
//
// # Why the plain fields are not removed
//
// The YAML loader (yaml.v3) populates fields by reflection. A
// field that does not exist cannot be populated. The plain fields
// are the loader's input; the Secret fields are the loader's
// output. After RedactSecrets runs, the plain fields are empty.
type RegistryConfig struct {
	// URL is the registry endpoint. It is not sensitive.
	URL string `yaml:"url"`

	// Token is the registry authentication token. It is
	// declared `sensitive: true` in the schema.
	//
	// # Why the field is cleared after loading
	//
	// The loader (RedactSecrets) moves this field's value into
	// TokenSecret and clears this field. A caller that reads
	// cfg.Registry.Token after loading sees the empty string; the
	// value is accessible only via TokenSecret.Reveal. The move is
	// deliberate: it makes the credential value inaccessible
	// through the plain field, forcing every caller that wants the
	// value to go through Reveal (a deliberate, grep-able action).
	Token string `yaml:"token"`

	// TokenSecret is the redacted form of Token. It is set by the
	// loader and is the field callers should use when they need to
	// log or serialize the token.
	//
	// The YAML tag `-` declares that the field is not part of the
	// YAML document; it is populated by the loader, not by the
	// file.
	TokenSecret logger.Secret `yaml:"-"`

	// Password is the registry password. It is declared
	// `sensitive: true` in the schema.
	Password string `yaml:"password"`

	// PasswordSecret is the redacted form of Password.
	PasswordSecret logger.Secret `yaml:"-"`
}

// String implements fmt.Stringer for RegistryConfig.
//
// # Why the method exists
//
// A RegistryConfig carries two sensitive fields (Token, Password)
// and their Secret companions. Without this method, fmt.Sprintf on
// a RegistryConfig recurses into the struct's fields and renders
// each one with the requested verb. The Secret fields render as
// the marker (their String method returns it), but the plain
// fields render as whatever string they hold. Once RedactSecrets
// has run the plain fields are empty and the recursion is
// harmless; a Config that has not been redacted — a caller that
// constructed one directly, or a test that asserts on the
// pre-redaction value — would leak through %s.
//
// The method renders the non-sensitive URL and the two Secret
// companions. The plain fields are not consulted, so the rendering
// is safe for a RegistryConfig in any state.
//
// # Why the method is on the value receiver
//
// The method matches the Secret type's own method set (see
// internal/logger/secret.go), which is on the value receiver.
// Placing it on the value receiver means a *RegistryConfig field
// picks it up through auto-dereference, so %s on a Config whose
// Registry is non-nil calls this method.
func (r RegistryConfig) String() string {
	return fmt.Sprintf("RegistryConfig{URL:%q Token:%s Password:%s}",
		r.URL, r.TokenSecret, r.PasswordSecret)
}

// TelemetryConfig is the telemetry section of the config file.
//
// # Sensitive fields
//
// APIKey is declared `sensitive: true` in the schema. The loader
// wraps its value in a logger.Secret and clears the plain field.
type TelemetryConfig struct {
	// Endpoint is the telemetry endpoint. It is not sensitive.
	Endpoint string `yaml:"endpoint"`

	// APIKey is the telemetry API key. It is declared
	// `sensitive: true` in the schema.
	APIKey string `yaml:"api_key"`

	// APIKeySecret is the redacted form of APIKey. It is set by
	// the loader and is the field callers should use when they
	// need to log or serialize the key.
	APIKeySecret logger.Secret `yaml:"-"`
}

// String implements fmt.Stringer for TelemetryConfig.
//
// See RegistryConfig.String for the rationale. The method renders
// the non-sensitive Endpoint and the Secret companion; the plain
// APIKey field is not consulted, so the rendering is safe for a
// TelemetryConfig in any state.
func (t TelemetryConfig) String() string {
	return fmt.Sprintf("TelemetryConfig{Endpoint:%q APIKey:%s}",
		t.Endpoint, t.APIKeySecret)
}

// SourceFor returns the source of the named field.
//
// # Semantics
//
// If the field was set by a loader layer, SourceFor returns that
// layer's source. If the field was never set, if the Config is a
// zero value, or if the field name is unknown, SourceFor returns
// SourceDefault.
//
// # Field names
//
// The argument is the dotted Go field name ("LogLevel",
// "Registry.Token", "Telemetry.APIKey"), not the YAML key
// ("log_level", "registry.token", "telemetry.api_key"). The loader
// records sources under Go names; SourceFor reads them under Go
// names. See the sources field's docstring for the rationale.
//
// # Unknown field names
//
// An unknown field name returns SourceDefault rather than an
// error. The frozen output format (WBS 9.2.1) has no other
// sensible value for an unset field, and returning an error would
// force every caller to handle a case that cannot occur for the
// fields the CLI renders. A typo in a field name produces a silent
// default, which is a caller bug, not a config-package error.
//
// # Concurrency
//
// SourceFor is safe for concurrent use. It reads from an immutable
// map; the loader that populated it has returned. Config values
// are not mutated after construction (see the type's docstring).
func (c Config) SourceFor(field string) Source {
	if c.sources == nil {
		return SourceDefault
	}
	if s, ok := c.sources[field]; ok {
		return s
	}
	return SourceDefault
}

// setSource records src as the source of field. It is the only
// writer of the sources map.
//
// # Precedence is the caller's responsibility
//
// The method does not check whether src outranks the field's
// current source. The loader calls setSource in ascending
// precedence order (default, then file, then env, then flag), so
// the last call for a given field is the highest-priority one.
// Moving the check into setSource would let the loader call the
// layers in any order, which would hide a loader bug rather than
// surface it.
//
// # Lazy map initialization
//
// The first call allocates the map. Subsequent calls reuse it.
// This is what makes the zero-value Config usable: a caller that
// constructs `Config{}` and then runs the loader does not need to
// pre-allocate the map.
//
// # Nil receiver
//
// The method dereferences c to write to c.sources. Calling
// setSource on a nil *Config panics, as any pointer-method call on
// nil would. The loader is expected to construct a Config (value)
// and pass its address; a nil receiver is a caller bug.
func (c *Config) setSource(field string, src Source) {
	if c.sources == nil {
		c.sources = make(map[string]Source, 8)
	}
	c.sources[field] = src
}
