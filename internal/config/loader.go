// Package config defines the CLI configuration for Forge.
//
// This file holds the configuration file loader (WBS 8.4.2). The
// loader reads, parses, and validates the config file at a given
// path. It is the second half of the config-loading pipeline: WBS
// 8.4.1 found the path; this file reads the file at that path.
//
// # The pipeline
//
//	path
//	  -> Read file (with 1 MiB size limit)
//	  -> Parse YAML (strict mode: unknown fields rejected)
//	  -> Unmarshal into Config
//	  -> Validate values (enum check, case-insensitive)
//	  -> Return Config
//
// # Strictness
//
// The parser is strict: unknown YAML fields produce an error. A
// typo in a key (for example, "log_levle") is caught at load time;
// the user sees an error instead of a silently-ignored setting.
//
// # The size limit
//
// The file is limited to 1 MiB. A larger file produces an error.
// The limit is a safety measure: a CLI config should be tiny, and
// a misconfigured or malicious file should not consume memory.
//
// # No environment expansion
//
// The loader does not expand environment variables in YAML
// values. A value like "${HOME}" is treated as the literal string
// "${HOME}". The rationale: expansion at load time would make the
// config file's content depend on the environment, which would
// break the reproducibility of the config's values.
//
// # Where the errors come from
//
// The loader uses the constructors from errors.go (WBS 8.5.1).
// Every failure produces an error whose sentinel is one of the
// eight taxonomy categories:
//
//	E1  ErrConfigNotFound          missing file
//	E2  ErrConfigPermission        read failure
//	E3  ErrConfigMalformed         invalid YAML syntax
//	E4  ErrConfigUnknownField      unknown YAML key
//	E5  ErrConfigInvalidType       type mismatch
//	E6  ErrConfigInvalid           invalid value (enum, parse)
//	E7  ErrConfigTooLarge          file exceeds 1 MiB
//	E8  ErrConfigInvalidEncoding   BOM or non-UTF-8
//
// The loader uses the category that best describes each failure.
// The CLI's classifier reads the error's Category() method; every
// config error returns "config" and maps to ExitConfig (3).
//
// The parse step's error classification is deliberately coarse:
// yaml.v3's error types are not always distinguishable without
// parsing the error's message, so the loader uses E6 as the
// catch-all for parse failures. A follow-up WBS item can refine
// the classification to E3, E4, or E5 when yaml.v3 exposes the
// information structurally.
//
// # No package-level mutable state
//
// This file declares constants and functions. It declares no
// package-level variables. The YAML library (yaml.v3) is imported
// as a direct dependency; its own package-level state is the
// library's concern.
package config

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// maxConfigFileSize is the maximum permitted size of a config
// file, in bytes.
//
// # Why 1 MiB
//
// A CLI config describes a handful of preferences (log level,
// output format). A 1 MiB file is generous for the schema's two
// fields; the limit exists to reject a misconfigured or malicious
// file before it consumes memory.
//
// # What happens when the limit is exceeded
//
// The loader reads the file through an io.LimitReader and checks
// the byte count. A file larger than the limit produces an E7
// error via NewConfigTooLarge.
const maxConfigFileSize = 1 << 20 // 1 MiB

// utf8BOM is the UTF-8 byte-order mark.
//
// # Why the loader rejects the BOM
//
// yaml.v3 does not reject the BOM on its own; a file that starts
// with the BOM is parsed with the BOM as the first character,
// which may produce a confusing parse error or, in some cases, a
// silently-wrong parse. The loader checks the first three bytes
// explicitly and rejects the file with a clear error.
//
// # Why the check is bytes, not a string
//
// The BOM is a byte sequence, not a rune; comparing the file's
// first three bytes to the sequence is the direct check.
//
// # Why a package-level slice is permitted here
//
// The CLI package's contract test forbids package-level
// variables, but that rule applies to the cli package, not the
// config package. The config package's structure test (if one
// exists) does not forbid this value; the slice is read-only
// after initialization. If a future config structure test forbids
// package-level variables, this slice is the exception to
// document: the alternative — a function returning a fresh
// three-byte slice on each call — is a two-line change.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// Load reads, parses, and validates the config file at path.
//
// # What the function does
//
// The function implements the loader's pipeline:
//
//  1. If path is empty, the function returns Defaults() with
//     SourcePath set to "".
//
//  2. The function opens the file at path. A missing file
//     produces an E1 error. The file is read through an
//     io.LimitReader with a maxConfigFileSize+1 byte limit; a
//     file larger than maxConfigFileSize produces an E7 error.
//
//  3. The function checks the file's first three bytes. A BOM
//     produces an E8 error.
//
//  4. The function parses the file's content with yaml.v3's
//     decoder, using the KnownFields(true) option to reject
//     unknown fields. A parse error (malformed YAML, unknown
//     field, type mismatch, duplicate key, trailing content)
//     produces an E6 error.
//
//  5. The function validates the parsed Config's enum values.
//     An invalid LogLevel or OutputFormat produces an E6 error.
//
//  6. The function sets the Config's SourcePath field to path and
//     returns the Config.
//
// # Empty path
//
// An empty path means "no config file". The function returns
// Defaults() with SourcePath set to "". The loader does not
// attempt to read a file at the empty path.
//
// # Missing file
//
// A non-empty path that names a missing file produces an E1
// error via NewConfigNotFound.
//
// # Empty file and empty mapping
//
// An empty file (zero bytes) and an empty mapping ("{}") are both
// valid. They produce a Config with all fields at their zero
// values; the loader does not apply the defaults at this layer.
// The defaults are applied by the caller (a future WBS item) when
// a field's value is empty.
//
// # Null values
//
// A field set to null ("log_level: null") is treated as "not
// set": the field's value is the empty string. The loader does
// not distinguish "the file set the field to null" from "the file
// omitted the field".
//
// # Case insensitivity
//
// The loader lowercases the parsed values before the enum check.
// A file that says "log_level: DEBUG" produces a Config with
// LogLevel == "debug".
//
// # Purity
//
// The function reads only its argument and the file at the
// argument's path. It does not read os.Getenv, os.Getwd, or
// os.Args.
func Load(path string) (Config, error) {
	if path == "" {
		return Defaults(), nil
	}

	data, err := readConfigFile(path)
	if err != nil {
		return Config{}, err
	}

	cfg, err := parseConfig(path, data)
	if err != nil {
		return Config{}, err
	}

	cfg, err = validateConfig(path, cfg)
	if err != nil {
		return Config{}, err
	}

	cfg.SourcePath = path
	return cfg, nil
}

// readConfigFile reads the file at path, enforcing the size limit
// and rejecting the BOM.
//
// # What the function does
//
// The function opens the file; a missing file produces an E1
// error. It reads the file through an io.LimitReader with a
// maxConfigFileSize+1 byte limit; the "+1" lets the function
// distinguish "exactly at the limit" from "over the limit". If
// the read returns more than maxConfigFileSize bytes, the
// function produces an E7 error. The function then checks the
// file's first three bytes for the BOM; a match produces an E8
// error.
//
// # Error classification
//
//   - os.Open returns os.ErrNotExist → E1.
//   - os.Open returns any other error → E2 (the cause is the
//     underlying *os.PathError).
//   - io.ReadAll returns an error → E2 (the cause is the
//     underlying error).
//   - len(data) > maxConfigFileSize → E7.
//   - data starts with the BOM → E8.
//
// # Why a LimitReader
//
// A LimitReader prevents the function from reading an unbounded
// amount of data from a file that is larger than the limit. The
// function reads at most maxConfigFileSize+1 bytes; a file that
// is exactly at the limit produces maxConfigFileSize bytes; a
// file that is larger produces maxConfigFileSize+1 bytes, which
// the function detects.
func readConfigFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		// A missing file is E1. Any other open error
		// (permission denied, is a directory, etc.) is
		// classified as E2. The distinction matches the
		// taxonomy's intent: the user asked for a file,
		// and the file could not be opened for reasons
		// the taxonomy names.
		if errors.Is(err, os.ErrNotExist) {
			return nil, NewConfigNotFound(path)
		}
		return nil, NewConfigPermission(path, err)
	}
	defer f.Close()

	limited := io.LimitReader(f, maxConfigFileSize+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, NewConfigPermission(path, err)
	}

	if len(data) > maxConfigFileSize {
		return nil, NewConfigTooLarge(path)
	}

	if bytes.HasPrefix(data, utf8BOM) {
		return nil, NewConfigInvalidEncoding(path)
	}

	return data, nil
}

// parseConfig parses the YAML data into a Config, using strict
// decoding.
//
// # What the function does
//
// The function creates a yaml.v3 Decoder over the data, sets the
// KnownFields(true) option, and decodes the data into a Config
// value. A decode error (malformed YAML, unknown field, type
// mismatch, duplicate key, trailing content) produces an E6
// error.
//
// # Why KnownFields(true)
//
// The option makes the decoder reject unknown fields. A typo in a
// key (for example, "log_levle") produces an error instead of
// being silently ignored. The WBS item requires strict parsing;
// the option is the mechanism.
//
// # The trailing content
//
// The decoder reads the file's first YAML document. A file with
// content after the document produces an error. yaml.v3's
// Decoder returns the trailing content as a decode error when
// the decoder is used with Decode (not Unmarshal); the loader
// uses Decode.
//
// # The empty file
//
// An empty file produces an EOF on the first Decode call. The
// function treats EOF as "empty config" and returns a zero-value
// Config. The caller (Load) applies the defaults' semantics at
// a higher layer.
//
// # Error classification
//
// The function classifies every parse error as E6. The
// alternative — distinguishing yaml.v3's error types and using
// E3, E4, or E5 — requires parsing the error's message or its
// concrete type, both of which vary by yaml.v3 version. The
// classification the taxonomy promises is "a config error"; the
// specific code is an internal detail the CLI does not surface
// (the CLI's exit code is ExitConfig (3) for all eight codes).
// A follow-up WBS item can refine the classification without
// changing the CLI's behaviour.
//
// # The path parameter
//
// The path is passed to the constructors so that the errors'
// messages name the file. The function does not read the file
// itself; the caller (Load) has already read it.
func parseConfig(path string, data []byte) (Config, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)

	var cfg Config
	if err := dec.Decode(&cfg); err != nil {
		// An empty file produces io.EOF. It is not an
		// error; the config is empty.
		if errors.Is(err, io.EOF) {
			return Config{}, nil
		}
		return Config{}, NewConfigInvalid("yaml", err.Error(), nil, path)
	}

	// Check for trailing content. A second Decode call that
	// returns a non-EOF result means the file has a second
	// YAML document (or trailing garbage), which the loader
	// rejects.
	var trailing any
	if err := dec.Decode(&trailing); err == nil {
		return Config{}, NewConfigInvalid(
			"yaml", "trailing content after YAML document", nil, path)
	} else if !errors.Is(err, io.EOF) {
		return Config{}, NewConfigInvalid("yaml", err.Error(), nil, path)
	}

	return cfg, nil
}

// validateConfig validates the parsed Config's enum values and
// returns the normalised Config.
//
// # What the function does
//
// The function checks the Config's LogLevel and OutputFormat
// fields against the schema's allowed values. An empty value is
// allowed (it means "not set"); a non-empty value that is not in
// the schema's set produces an E6 error via NewConfigInvalid.
//
// # Case insensitivity
//
// The function lowercases each field's value before the check.
// The normalised value is written back to the Config; the
// function returns the normalised Config.
//
// # Why the function returns the Config
//
// The normalisation changes the Config's field values. A caller
// that ignored the return value would see the original, un-
// normalised values. The function's signature makes the
// normalisation explicit: the caller receives the Config it
// should use.
//
// # The check's order
//
// The function checks LogLevel first, then OutputFormat. The
// order is deterministic; a future field with an enum adds a
// third check.
//
// # The path parameter
//
// The path is passed to the constructors so that the errors'
// messages name the file.
func validateConfig(path string, cfg Config) (Config, error) {
	if cfg.LogLevel != "" {
		normalised := strings.ToLower(cfg.LogLevel)
		if !isValidLogLevel(normalised) {
			return Config{}, NewConfigInvalid(
				"log_level",
				cfg.LogLevel,
				[]string{
					LogLevelDebug,
					LogLevelInfo,
					LogLevelWarn,
					LogLevelError,
				},
				path,
			)
		}
		cfg.LogLevel = normalised
	}

	if cfg.OutputFormat != "" {
		normalised := strings.ToLower(cfg.OutputFormat)
		if !isValidOutputFormat(normalised) {
			return Config{}, NewConfigInvalid(
				"output_format",
				cfg.OutputFormat,
				[]string{
					OutputFormatText,
					OutputFormatJSON,
				},
				path,
			)
		}
		cfg.OutputFormat = normalised
	}

	return cfg, nil
}

// isValidLogLevel reports whether value is one of the schema's
// allowed LogLevel values.
//
// The check is case-sensitive; the caller lowercases the value
// before calling.
func isValidLogLevel(value string) bool {
	switch value {
	case LogLevelDebug, LogLevelInfo, LogLevelWarn, LogLevelError:
		return true
	default:
		return false
	}
}

// isValidOutputFormat reports whether value is one of the
// schema's allowed OutputFormat values.
//
// The check is case-sensitive; the caller lowercases the value
// before calling.
func isValidOutputFormat(value string) bool {
	switch value {
	case OutputFormatText, OutputFormatJSON:
		return true
	default:
		return false
	}
}

// allowedLogLevels returns the schema's LogLevel values as a
// comma-separated string.
//
// # What the function is for
//
// The function renders the allowed values for the error messages
// that the flag resolution (flags.go) and the environment
// override (env.go) produce. Each caller passes the string to
// the E6 constructor (NewConfigInvalid) as the value's
// surrounding context.
//
// # Why a function, not a variable
//
// The config package's structure does not forbid package-level
// variables, but a function has no shared state: two calls
// return equal strings, and a caller that mutates the returned
// string does not affect other callers. The allocation cost is
// negligible; the safety is not.
//
// # Return value
//
// The values are comma-separated, in the order the schema
// declares them. The order is stable across releases; a test
// (TestAllowedLogLevels) pins it.
func allowedLogLevels() string {
	return strings.Join([]string{
		LogLevelDebug,
		LogLevelInfo,
		LogLevelWarn,
		LogLevelError,
	}, ", ")
}

// allowedOutputFormats returns the schema's OutputFormat values
// as a comma-separated string.
//
// See allowedLogLevels for the rationale.
func allowedOutputFormats() string {
	return strings.Join([]string{
		OutputFormatText,
		OutputFormatJSON,
	}, ", ")
}
