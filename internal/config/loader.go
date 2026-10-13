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
//	  -> Check encoding (BOM, UTF-8)
//	  -> Parse YAML (strict mode: unknown fields rejected)
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
// Every failure produces a *forgeerr.Error with category
// CategoryConfig (WBS 10.5.2). The taxonomy's eight codes
// classify the failure:
//
//	E1  CodeConfigNotFound          missing file
//	E2  CodeConfigPermission        read failure
//	E3  CodeConfigMalformed         invalid YAML syntax
//	E4  CodeConfigUnknownField      unknown YAML key
//	E5  CodeConfigInvalidType       type mismatch
//	E6  CodeConfigInvalidValue      invalid value (enum, trailing)
//	E7  CodeConfigTooLarge          file exceeds 1 MiB
//	E8  CodeConfigInvalidEncoding   BOM or non-UTF-8
//
// The loader uses the code that best describes each failure. The
// CLI's classifier reads the error's Category() method; every
// config error returns CategoryConfig and maps to ExitConfig (3).
//
// # Why E3 for syntax errors and E6 for value errors
//
// A YAML syntax error is a problem with the file's structure: the
// parser cannot produce a document. It is classified as E3
// (CodeConfigMalformed). The parser's error message names the
// line and column; the loader extracts them into the error's
// context.
//
// A value error is a problem with a field's content: the
// document parsed, but a value is not in the field's allowed set.
// It is classified as E6 (CodeConfigInvalidValue). The loader
// names the field, the offending value, and the allowed set in
// the error's context.
//
// The two categories are distinct because the user's remediation
// is different: a syntax error is fixed by editing the YAML; a
// value error is fixed by choosing an allowed value. Classifying
// both as E6 (as an earlier version did) would hide the
// distinction.
//
// # No package-level mutable state
//
// This file declares constants and functions. It declares one
// package-level variable, utf8BOM, which is read-only after
// initialization. The YAML library (yaml.v3) is imported as a
// direct dependency; its own package-level state is the
// library's concern.
//
// # DEBUG records (WBS 12.5.1)
//
// The loader emits four DEBUG records, all to the process logger
// (which writes to stderr):
//
//  1. "config loader: entered" — after Load/LoadContext is
//     called, with the input path (or "(none)"). This is the
//     entry record: it answers "did the loader run, and with
//     what input?".
//  2. "config loader: read file" — after the file is read
//     successfully, with the path, the byte count, and the BOM
//     check result.
//  3. "config loader: parsed YAML" — after the YAML document
//     parses into a Config, with the byte count and the two
//     field-set indicators.
//  4. "config loader: validated" — after the enum validation
//     passes, with the two effective field values.
//
// The four records trace the pipeline's three successful stages
// (read, parse, validate) plus the entry. They are the config
// subsystem's contribution to docs/reference/verbose-mode.md §3.
//
// The error branches do NOT emit DEBUG records. An error is a
// failure, not a decision; the boundary renders it with
// forgeerr.Format, and the user sees the failure there. Emitting
// a DEBUG record in addition to the rendered error would
// duplicate the diagnostic. The rule from the reference doc is
// "emit at decision points"; an error is not a decision.
//
// # Context
//
// The records carry the context passed to LoadContext. A caller
// that uses Load (the legacy entry point) gets records with a
// background context, which means they carry the zero
// correlation ID. A caller that wants the records to carry an
// invocation's correlation ID uses LoadContext.
package config

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/thapelomagqazana/forge/internal/logger"
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
// after initialization. If a future config structure test
// forbids package-level variables, this slice is the exception
// to document: the alternative — a function returning a fresh
// three-byte slice on each call — is a two-line change.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// Load reads, parses, and validates the config file at path.
//
// Load is the legacy entry point. It calls LoadContext with a
// background context, which means the DEBUG records it emits
// carry the zero correlation ID.
//
// New callers that have an invocation's context (the CLI's
// hooks, for example) should use LoadContext directly. The two
// functions share the same pipeline; Load is a one-line wrapper.
//
// The full docstring for the pipeline is on LoadContext.
func Load(path string) (Config, error) {
	return LoadContext(context.Background(), path)
}

// LoadContext reads, parses, and validates the config file at path.
//
// # What the function does
//
// The function implements the loader's pipeline:
//
//  1. If path is empty, the function returns Defaults() with
//     SourcePath set to "". An empty path means "no config file".
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
//     unknown fields. A syntax error produces an E3 error with
//     the line and column (when available) in the context.
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
// not distinguish "the file set the field to null" from "the
// file omitted the field".
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
// os.Args. The context is used solely for the DEBUG records; the
// pipeline's behavior does not depend on it.
//
// # Nil context
//
// A nil context is replaced by context.Background(). The
// substitution keeps the DEBUG records' emission unconditional
// without requiring every caller to pass a non-nil context.
// Callers that pass a real context (the CLI's hooks) get records
// that carry the invocation's correlation ID.
func LoadContext(ctx context.Context, path string) (Config, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	// WBS 12.5.1: the entry record.
	//
	// # What the record reports
	//
	// path is the input path, or "(none)" when the empty-path
	// shortcut applies. The record tells a reader whether the
	// loader was even asked to find a file, which distinguishes
	// "discovery found nothing" (path == "") from "discovery
	// found a file that failed to load" (path != "").
	//
	// The record is emitted before any work, so a reader sees
	// it even when the pipeline fails at the first stage.
	{
		displayPath := path
		if displayPath == "" {
			displayPath = "(none)"
		}
		logger.Debug(ctx, "config loader: entered",
			"path", displayPath,
		)
	}

	if path == "" {
		// The empty-path shortcut. The summary record is not
		// emitted for this branch because no work was done;
		// the entry record's "(none)" path is sufficient.
		return Defaults(), nil
	}

	data, err := readConfigFile(path)
	if err != nil {
		return Config{}, err
	}

	// WBS 12.5.1: the file-read record.
	//
	// # Why "bytes" and "bom"
	//
	// bytes names the file's size, which is diagnostic for a
	// user who is looking at a large file that just fit under
	// the limit. bom reports whether the file started with the
	// UTF-8 BOM; the loader rejected files that did, so the
	// record's value is always false at this point. Including
	// the field documents the check that the reader is about to
	// see the result of; a future change that loosens the BOM
	// policy would make the field meaningful.
	logger.Debug(ctx, "config loader: read file",
		"path", path,
		"bytes", len(data),
		"bom", bytes.HasPrefix(data, utf8BOM),
	)

	cfg, err := parseConfig(path, data)
	if err != nil {
		return Config{}, err
	}

	// WBS 12.5.1: the parse record.
	//
	// # Why these fields
	//
	// bytes repeats the size (from the file-read record); the
	// two records appear back-to-back in the log, and the
	// repetition makes each record self-contained for a reader
	// who filters by message. log_level_set and
	// output_format_set report which fields the file actually
	// specified, before the defaults are applied by the caller.
	// A file with no keys ("{}") produces two false values; a
	// file with both keys produces two true values.
	logger.Debug(ctx, "config loader: parsed YAML",
		"path", path,
		"bytes", len(data),
		"log_level_set", cfg.LogLevel != "",
		"output_format_set", cfg.OutputFormat != "",
	)

	cfg, err = validateConfig(path, cfg)
	if err != nil {
		return Config{}, err
	}

	// WBS 12.5.1: the validation record.
	//
	// # Why the values and not just the set indicators
	//
	// After validation, the values are the effective values the
	// caller will see (with case normalization applied). The
	// record reports them so a reader can correlate the
	// loader's output with the resolver's input without reading
	// the file.
	logger.Debug(ctx, "config loader: validated",
		"path", path,
		"log_level", cfg.LogLevel,
		"output_format", cfg.OutputFormat,
	)

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
//
// # Why no DEBUG records
//
// The file-read record is emitted by LoadContext, not by this
// function. Emitting it here would require threading a context
// through a function whose signature is stable and whose
// behavior is pure; the caller already has the context and the
// values (path, data) the record needs.
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
// value.
//
// Error classification:
//
//   - A syntax error (malformed YAML, duplicate key) is E3
//     (CodeConfigMalformed). The parser's error message names
//     the line and column; the loader extracts them into the
//     error's context under the keys "line" and "column".
//
//   - An unknown field is E4 (CodeConfigUnknownField). The
//     strict decoder rejects the field before unmarshalling;
//     the field's name is in the parser's error message and is
//     extracted into the error's context under the key "field".
//
//   - A type mismatch is E5 (CodeConfigInvalidType). The
//     decoder's error names the field and the expected type;
//     the field's name is extracted into the error's context.
//
//   - Trailing content after the first document is E6
//     (CodeConfigInvalidValue). The content is not part of the
//     schema, and the error names the fact that the file has
//     more than one document.
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
// Config. The caller (LoadContext) applies the defaults'
// semantics at a higher layer.
//
// # Error classification stability
//
// The classification of yaml.v3's errors is textual (the
// library does not export a structured error type for parse
// failures). The recognized prefixes are documented on
// classifyYAMLError; a yaml.v3 upgrade that changes a message
// fails the test TestLoad_MalformedYAMLIncludesLineNumber, which
// asserts that a syntax error is classified as E3.
//
// # The path parameter
//
// The path is passed to the constructors so that the errors'
// messages name the file. The function does not read the file
// itself; the caller (LoadContext) has already read it.
//
// # Why no DEBUG records
//
// The parse record is emitted by LoadContext after this function
// returns successfully. The parse-failure branches do not emit
// records; the error itself is the diagnostic, and the boundary
// renders it.
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
		return Config{}, classifyYAMLError(path, err)
	}

	// Check for trailing content. A second Decode call that
	// returns a non-EOF result means the file has a second
	// YAML document (or trailing garbage), which the loader
	// rejects.
	//
	// The trailing-content error is E6 because the file's
	// structure is valid; the problem is that the file has
	// more than the schema expects. This is closer to a
	// "value" problem (the file's content is not what the
	// schema permits) than a syntax problem.
	var trailing any
	if err := dec.Decode(&trailing); err == nil {
		return Config{}, NewConfigInvalid(
			"yaml", "trailing content after YAML document", nil, path)
	} else if !errors.Is(err, io.EOF) {
		// The trailing decode failed with a syntax error;
		// classify it as such.
		return Config{}, classifyYAMLError(path, err)
	}

	return cfg, nil
}

// classifyYAMLError classifies a yaml.v3 decode error as one of
// the config taxonomy's codes.
//
// # The classification
//
// The function recognizes three shapes of yaml.v3 errors:
//
//  1. Unknown field: the error message contains "field <name>
//     not found in type". The function extracts the field name
//     and constructs an E4 error.
//
//  2. Type mismatch: the error message contains "cannot
//     unmarshal" or "cannot decode". The function extracts the
//     field name (when present) and constructs an E5 error.
//
//  3. Syntax error: everything else. The function extracts the
//     line and column (when present in the message) and
//     constructs an E3 error.
//
// # Why textual classification
//
// yaml.v3 does not export structured error types for the three
// shapes. The shapes are distinguished by the error message's
// content. The classification is therefore textual and is
// documented on each recognized prefix.
//
// # Stability
//
// A yaml.v3 upgrade that changes a message shape fails the tests
// that exercise the affected classification. The failure points
// at the classification function, and the fix is localized to
// the prefix list below.
func classifyYAMLError(path string, err error) error {
	msg := err.Error()

	// Unknown field: yaml.v3's message is
	// "line N: field <name> not found in type config.Config".
	if strings.Contains(msg, "not found in type") {
		field := extractQuotedOrWord(msg, "field ", " not found")
		return NewConfigUnknownField(path, field, err)
	}

	// Type mismatch: yaml.v3's message is
	// "line N: cannot unmarshal !!str `...` into int".
	if strings.Contains(msg, "cannot unmarshal") ||
		strings.Contains(msg, "cannot decode") {
		field := extractYAMLFieldName(msg)
		return NewConfigInvalidType(path, field, err)
	}

	// Syntax error: everything else. Extract the line and
	// column when the message names them.
	line, col := parseYAMLErrorLocation(err)
	return NewConfigMalformed(path, line, col, err)
}

// extractQuotedOrWord extracts the word between the given prefix
// and suffix in msg.
//
// The helper is used by classifyYAMLError to pull a field name
// out of a yaml.v3 message. The extraction is textual; it is not
// intended to be a general-purpose parser. When the prefix or
// suffix is not present, the function returns the empty string.
func extractQuotedOrWord(msg, prefix, suffix string) string {
	i := strings.Index(msg, prefix)
	if i < 0 {
		return ""
	}
	rest := msg[i+len(prefix):]
	j := strings.Index(rest, suffix)
	if j < 0 {
		return ""
	}
	return strings.TrimSpace(rest[:j])
}

// extractYAMLFieldName extracts a field name from a yaml.v3 type
// mismatch message.
//
// yaml.v3's type mismatch messages do not always name the
// offending field. When the field name is present (as a quoted
// string or as a struct field name), the helper returns it. When
// it is not, the helper returns "yaml" as a placeholder; the
// error's message still names the type mismatch, and the CLI's
// exit code is unaffected by the placeholder.
func extractYAMLFieldName(msg string) string {
	// The message often contains "into <type>" at the end; the
	// field name, when present, precedes it. The heuristic
	// looks for "field " followed by a name.
	if name := extractQuotedOrWord(msg, "field ", " "); name != "" {
		return name
	}
	return "yaml"
}

// parseYAMLErrorLocation extracts the line and column from a
// yaml.v3 parse error's message.
//
// yaml.v3's error messages have one of two forms:
//
//	yaml: line N: <description>
//	yaml: line N, column M: <description>
//
// The function returns (0, 0) when the message does not match
// either form. A zero line or column is treated by
// NewConfigMalformed as "unknown"; the constructor then omits
// the corresponding context key.
//
// # Why the extraction is textual
//
// yaml.v3 does not export a structured error type for parse
// failures; the error is a *yaml.TypeError or a plain
// errors.errorString depending on the failure. The line and
// column are embedded in the message. The textual extraction
// is the only way to recover them without vendoring the
// library's internal types.
//
// # Stability
//
// The two message forms are stable across yaml.v3's minor
// releases. A major release that changes them would fail the
// test TestLoad_MalformedYAMLIncludesLineNumber, which asserts
// that a syntax error's context includes a "line" key.
func parseYAMLErrorLocation(err error) (line, col int) {
	msg := err.Error()

	// Find "line N" in the message.
	const linePrefix = "line "
	i := strings.Index(msg, linePrefix)
	if i < 0 {
		return 0, 0
	}
	rest := msg[i+len(linePrefix):]

	// Read the digits.
	j := 0
	for j < len(rest) && rest[j] >= '0' && rest[j] <= '9' {
		j++
	}
	if j == 0 {
		return 0, 0
	}
	line, _ = strconv.Atoi(rest[:j])

	// Look for ", column M" immediately after the line number.
	rest = rest[j:]
	const colSep = ", column "
	if !strings.HasPrefix(rest, colSep) {
		return line, 0
	}
	rest = rest[len(colSep):]
	j = 0
	for j < len(rest) && rest[j] >= '0' && rest[j] <= '9' {
		j++
	}
	if j == 0 {
		return line, 0
	}
	col, _ = strconv.Atoi(rest[:j])
	return line, col
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
//
// # Why no DEBUG records
//
// The validation record is emitted by LoadContext after this
// function returns successfully. The validation-failure branches
// do not emit records; the error itself is the diagnostic, and
// the boundary renders it.
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
