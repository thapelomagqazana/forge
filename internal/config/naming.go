// Package config defines the CLI configuration for Forge.
//
// This file holds the field name mapping rules (WBS 8.2.2). A
// configuration field has four representations:
//
//	Go identifier      PascalCase     LogLevel
//	YAML key           snake_case     log_level
//	Environment var    UPPER_SNAKE    FORGE_LOG_LEVEL
//	CLI flag           kebab-case     --log-level
//
// The three functions in this file derive the non-Go
// representations from the Go field name. The derivation is the
// single source of truth for the naming conventions; a future
// consumer that needs a representation (a YAML loader, an env var
// reader, a flag binder) calls the corresponding function rather
// than embedding the convention in its own code.
//
// # Why the derivation is one-way
//
// The Go field name is the source of truth. The other three
// representations are derived from it; each is a pure function of
// the Go name. A future contributor who renames a field updates
// the field's declaration; the three representations follow. The
// direction is deliberate: a bidirectional mapping (for example, a
// function that derives the Go field name from a YAML key) would
// require handling collisions (a hypothetical "log_level" and
// "logLevel" both mapping to "LogLevel") and would be ambiguous.
//
// # Why the functions do not validate
//
// The functions are pure derivations: given a string, return a
// string. They do not check whether the input is a valid Go field
// name (PascalCase, ASCII). A future consumer that reads a field
// name from an external source is responsible for validating it;
// the derivation functions assume a valid input.
//
// # No package-level mutable state
//
// This file declares functions and constants. It declares no
// package-level variables.
package config

import (
	"strings"
	"unicode"
)

// fieldFlagPrefix is the prefix on every CLI flag name.
//
// # Why the prefix is a constant
//
// The prefix appears on every flag name the derivation function
// returns. Its spelling is part of the convention; a future change
// to the prefix (for example, to a single dash) is a change to the
// convention and would break every consumer. The constant makes
// the prefix's spelling visible and changeable in one place.
//
// # Why the prefix is "--"
//
// The double dash is the POSIX convention for long-form flags.
// The CLI uses long-form flags exclusively in Phase 2; a future
// single-dash short form is a separate decision.
const fieldFlagPrefix = "--"

// GoFieldToYAML converts a Go field name (PascalCase) to a YAML
// key (snake_case).
//
// # The rule
//
// The function inserts an underscore before each uppercase letter
// (except the first) and lowercases the result. For example:
//
//	GoFieldToYAML("LogLevel")     == "log_level"
//	GoFieldToYAML("OutputFormat") == "output_format"
//	GoFieldToYAML("SourcePath")   == "source_path"
//	GoFieldToYAML("Simple")       == "simple"
//	GoFieldToYAML("A")            == "a"
//
// # The acronym case
//
// A field name with an acronym (for example, "APIKey") is
// converted as if each capital letter begins a word: the result is
// "a_p_i_key". The convention the function implements is the
// simple one (uppercase starts a word); a more sophisticated
// convention that keeps acronyms together ("api_key") would
// require a list of known acronyms. The CLI config's field names
// do not contain acronyms in Phase 2; the simpler rule is the one
// the schema uses.
//
// # The empty input
//
// An empty input produces the empty string. A caller that passes
// an empty field name gets an empty key; the caller decides
// whether that is a bug.
//
// # Why the function does not validate
//
// The function does not check whether the input is PascalCase. A
// lowercased input ("loglevel") produces "loglevel" (no
// underscores, since there are no uppercase letters to split on).
// The function's contract is "given a PascalCase field name,
// return the snake_case key"; a caller that passes a non-
// PascalCase name gets a derived result that may not be the
// caller's intent.
func GoFieldToYAML(field string) string {
	if field == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(field) + 4) // a small over-allocation for the underscores
	for i, r := range field {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// GoFieldToEnv converts a Go field name (PascalCase) to an
// environment variable name (FORGE_ + UPPER_SNAKE_CASE).
//
// # The rule
//
// The function applies GoFieldToYAML to the field name and then
// uppercases the result and prepends the prefix. For example:
//
//	GoFieldToEnv("LogLevel")     == "FORGE_LOG_LEVEL"
//	GoFieldToEnv("OutputFormat") == "FORGE_OUTPUT_FORMAT"
//	GoFieldToEnv("SourcePath")   == "FORGE_SOURCE_PATH"
//
// # Why the function reuses GoFieldToYAML
//
// The snake_case key and the UPPER_SNAKE_CASE variable name share
// the same structure: underscore-separated lowercase words. The
// variable name is the key uppercased, with the prefix prepended.
// Reusing GoFieldToYAML keeps the two representations in sync; a
// change to the snake_case rule changes both.
//
// # The empty input
//
// An empty input produces the prefix alone ("FORGE_"). The result
// is not a valid variable name; a caller that passes an empty
// field name gets a value it can detect.
func GoFieldToEnv(field string) string {
	return EnvVarPrefix + strings.ToUpper(GoFieldToYAML(field))
}

// GoFieldToFlag converts a Go field name (PascalCase) to a CLI
// flag name (kebab-case with a "--" prefix).
//
// # The rule
//
// The function applies GoFieldToYAML to the field name, replaces
// each underscore with a hyphen, and prepends the flag prefix.
// For example:
//
//	GoFieldToFlag("LogLevel")     == "--log-level"
//	GoFieldToFlag("OutputFormat") == "--output-format"
//	GoFieldToFlag("SourcePath")   == "--source-path"
//
// # Why the function reuses GoFieldToYAML
//
// The snake_case key and the kebab-case flag name share the same
// structure: separator-delimited lowercase words. The flag name is
// the key with underscores replaced by hyphens, with the flag
// prefix prepended. Reusing GoFieldToYAML keeps the two
// representations in sync; a change to the snake_case rule changes
// both.
//
// # Why the flag prefix is "--"
//
// The CLI uses long-form flags exclusively in Phase 2. The double
// dash is the POSIX convention for long-form flags; a single-dash
// short form is a separate decision.
//
// # The empty input
//
// An empty input produces the flag prefix alone ("--"). The
// result is not a valid flag name; a caller that passes an empty
// field name gets a value it can detect.
func GoFieldToFlag(field string) string {
	snake := GoFieldToYAML(field)
	if snake == "" {
		return fieldFlagPrefix
	}
	return fieldFlagPrefix + strings.ReplaceAll(snake, "_", "-")
}
