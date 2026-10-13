// Package config — config-aware redaction.
//
// This file implements layer 3 of the four-layer redaction model.
// See docs/security/logging-redaction.md §3.3 for the full model.
//
// # What the layer does
//
// When the config loader parses forge.yaml, fields whose schema
// declares `sensitive: true` are wrapped in logger.Secret before
// they are stored in the Config. The wrapping happens once, at
// load time; every subsequent log of the field sees the Secret's
// [REDACTED] rendering, and every JSON serialization of the
// field sees the marker.
//
// # Why the layer is separate from the pattern layer
//
// The pattern layer (internal/logger/redact.go) redacts text by
// its shape. The config layer redacts values by their *declared
// sensitivity*. A config field marked `sensitive: true` is
// redacted even if its value does not match any pattern; a field
// not so marked is redacted only if its value matches a pattern.
// The two layers are complementary: the config layer is precise
// (it knows which fields are sensitive); the pattern layer is a
// safety net (it catches values that leak without going through
// the config).
//
// # The mechanism is generic over the schema
//
// The set of sensitive fields is declared in sensitive.go. The
// mechanism in this file iterates the set and applies each
// field's mover function. Adding a sensitive field to the schema
// adds one entry to the set; this file does not change.
//
// # The current schema has no sensitive fields
//
// The frozen schema (WBS 8.2.1) has three fields: LogLevel,
// OutputFormat, SourcePath. None is credential-like. The set in
// sensitive.go is therefore empty, and RedactSecrets is a
// no-op. This is the correct behavior for the current schema.
// The mechanism is in place so that a future WBS that adds a
// credential field does not have to design the mechanism and the
// field at the same time.
package config

// RedactSecrets returns a copy of cfg with every field declared
// sensitive in sensitive.go replaced by its Secret counterpart.
//
// # What the function preserves
//
//   - The Config's non-sensitive fields are copied unchanged.
//   - The Config's sensitive fields are moved: the plain field is
//     cleared and the corresponding Secret field is set. The
//     wrapped string is preserved (accessible via
//     Secret.Reveal), but every serialization renders
//     [REDACTED].
//
// # The copy
//
// The function returns a new Config; the input is not mutated.
// This is the correct behavior for a loader: a caller that holds
// the pre-redaction Config (a test, for example) is not
// surprised by a mutation.
//
// # Nil safety
//
// A nil *Config returns a zero Config. A nil embedded struct (a
// future nil *RegistryConfig, for example) is left as nil; the
// corresponding mover function checks for nil before
// dereferencing.
//
// # Errors
//
// The function cannot fail. Its inputs are values; its output is
// a value. No I/O, no allocation beyond the copy.
//
// # Concurrency
//
// The function reads a package-level, immutable map and applies
// its functions to a local copy. It is safe for concurrent use.
func RedactSecrets(cfg *Config) Config {
	if cfg == nil {
		return Config{}
	}
	// Start from a shallow copy of the input. The copy shares
	// the underlying maps and pointers with the input; each
	// mover function is responsible for copying the nested
	// struct it modifies before writing, so the input is not
	// mutated.
	out := *cfg
	for _, move := range sensitiveFields {
		move(&out)
	}
	return out
}
