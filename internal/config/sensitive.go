// Package config — the sensitive-field registry.
//
// This file declares which fields of the Config type are
// credential-like. The list is the config-layer's contribution to
// the four-layer redaction model (see
// docs/security/logging-redaction.md §3.3).
//
// # What "sensitive" means
//
// A field is sensitive if its disclosure would grant access to a
// resource, authenticate a caller, or reveal internal state that
// is not meant for public consumption. Examples: an API token, a
// password, a private key.
//
// A field is NOT sensitive if it is a public identifier: a URL, a
// version string, a path, a log level. Wrapping those in Secret
// would make logs useless and would not improve security.
//
// # The current sensitive fields (WBS 12.7.1)
//
// The frozen top-level schema (WBS 8.2.1) has three scalar
// fields — LogLevel, OutputFormat, SourcePath — none of which is
// credential-like.
//
// WBS 12.7.1 added two nested sections with three sensitive
// members:
//
//	Registry.Token      string  sensitive (registry auth token)
//	Registry.Password   string  sensitive (registry password)
//	Telemetry.APIKey    string  sensitive (telemetry API key)
//
// Each is declared `sensitive: true` in
// schemas/config.schema.json. The registry below mirrors those
// declarations; redact_test.go pins the two against each other.
//
// # Why a registry and not inline checks
//
// The alternative — calling RedactSecrets's logic inline at each
// call site — would spread the knowledge of which fields are
// sensitive across the codebase. The registry centralizes that
// knowledge in one file, so adding a sensitive field is a
// one-file change and the schema test can pin the registry
// against the schema.
//
// # The declaration's shape
//
// Each entry's key is a dotted path from the Config root to the
// field:
//
//	"Registry.Token"
//	"Telemetry.APIKey"
//
// The path is a Go field path, not a YAML key path. It names the
// field in the Config struct, not the key in the YAML document.
// The two are related but not identical: a YAML key can be
// renamed (`output_format` to `format`) without changing the Go
// field name, and the sensitive list must track the Go field
// name because that is what the loader walks.
//
// # The value's shape and the copy contract
//
// Each entry's value is a function that takes a *Config, copies
// the section struct the field lives on, moves the field's value
// into the corresponding Secret field, and installs the copy.
//
// The copy is mandatory. RedactSecrets does a shallow copy of
// the top-level Config (out := *cfg); the copy shares the
// section pointers with the input. A mover that wrote through
// the section pointer without copying would mutate the caller's
// Config, violating RedactSecrets's "the input is not mutated"
// contract. Every mover below copies.
//
// The nil-section check is likewise mandatory: a Config whose
// Registry is nil (the section was absent from the file) has no
// field to move.
package config

import "github.com/thapelomagqazana/forge/internal/logger"

// sensitiveFields is the set of Config field paths whose values
// are credential-like and must be wrapped in logger.Secret at
// load time.
//
// # Extending the set
//
// A future WBS that adds a sensitive field adds the field's path
// to this map, the corresponding `sensitive: true` declaration to
// schemas/config.schema.json, and the Secret companion field to
// the appropriate struct in config.go. The mechanism in
// redact.go and the tests in redact_test.go consume this map
// generically; no change to either is required when the map
// grows.
//
// # Order
//
// The map has no defined iteration order, and the movers do not
// depend on one: each mover reads c.Registry or c.Telemetry
// afresh, copies the current section, and installs the copy. Two
// movers on the same section therefore compose correctly in
// either order (the second mover copies the section the first
// installed).
//
// # Why a map of functions and not a map of field names
//
// A map of field names would require a reflection-based mover,
// which is slower and harder to make correct for nested structs.
// A map of functions is explicit: each entry names the field's
// path and provides the code that moves it. The code is
// trivially testable per field.
//
// # No package-level mutable state
//
// The CLI package's contract test forbids package-level mutable
// state. This map is effectively immutable: it is written once at
// package initialization, its entries are read-only, and nothing
// in the package writes to it after initialization. The movers
// it holds close over no mutable state.
var sensitiveFields = map[string]func(*Config){
	"Registry.Token":    moveRegistryToken,
	"Registry.Password": moveRegistryPassword,
	"Telemetry.APIKey":  moveTelemetryAPIKey,
}

// moveRegistryToken moves Config.Registry.Token into
// Config.Registry.TokenSecret and clears the plain field.
//
// The mover copies the section before writing, so the input
// Config's *RegistryConfig is not mutated. A nil Registry is left
// nil: the section was absent from the file, and there is nothing
// to move.
func moveRegistryToken(c *Config) {
	if c.Registry == nil {
		return
	}
	registry := *c.Registry
	registry.TokenSecret = logger.NewSecret(registry.Token)
	registry.Token = ""
	c.Registry = &registry
}

// moveRegistryPassword moves Config.Registry.Password into
// Config.Registry.PasswordSecret and clears the plain field.
//
// The mover copies the section before writing, so the input
// Config's *RegistryConfig is not mutated. A nil Registry is left
// nil: the section was absent from the file, and there is nothing
// to move.
//
// The mover runs independently of moveRegistryToken. When both
// run, the second one copies the section the first installed; the
// result carries both Secret fields.
func moveRegistryPassword(c *Config) {
	if c.Registry == nil {
		return
	}
	registry := *c.Registry
	registry.PasswordSecret = logger.NewSecret(registry.Password)
	registry.Password = ""
	c.Registry = &registry
}

// moveTelemetryAPIKey moves Config.Telemetry.APIKey into
// Config.Telemetry.APIKeySecret and clears the plain field.
//
// The mover copies the section before writing, so the input
// Config's *TelemetryConfig is not mutated. A nil Telemetry is
// left nil: the section was absent from the file, and there is
// nothing to move.
func moveTelemetryAPIKey(c *Config) {
	if c.Telemetry == nil {
		return
	}
	telemetry := *c.Telemetry
	telemetry.APIKeySecret = logger.NewSecret(telemetry.APIKey)
	telemetry.APIKey = ""
	c.Telemetry = &telemetry
}
