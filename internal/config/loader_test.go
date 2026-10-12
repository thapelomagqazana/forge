// Package config contains unit tests for the CLI configuration
// type.
//
// This file pins the config file loader (WBS 8.4.2). The loader
// reads, parses, and validates the config file at a given path.
// The tests exercise the loader's pipeline: the size limit, the
// BOM check, strict parsing, value validation, and the empty-path
// case.
//
// # Fixtures
//
// The tests use the YAML fixtures in testdata/. Each fixture is a
// file with a specific property (valid, invalid, edge case). The
// tests name the fixture's purpose in their subtests.
//
// # The temp-file pattern
//
// The tests that need a file of a specific size (for example, the
// size-limit test) write the file into a t.TempDir(). The
// testdata/ fixtures cover the parsing rules; the temp files
// cover the size and BOM rules.
//
// # The parse-error classification
//
// The loader classifies a YAML parse error by the shape of the
// parser's message. The classification is documented on
// classifyYAMLError in loader.go. The relevant sentinels are:
//
//	E3  ErrConfigMalformed         syntax error, duplicate key
//	E4  ErrConfigUnknownField      unknown YAML field
//	E5  ErrConfigInvalidType       type mismatch
//	E6  ErrConfigInvalid           invalid enum value, trailing
//	                               content after the first document
//
// A test that asserts on a parse error asserts the sentinel that
// matches the case's classification. A test that asserts the old
// catch-all (ErrConfigInvalid) for a case now classified more
// specifically would fail; the failure points at the classification
// change, and the fix is a one-line assertion update.
//
// # Rendering the error
//
// After WBS 10.5.2, the config package's errors are
// *forgeerr.Error values. The message is short and fixed; the
// varying data (paths, field names, line numbers, offending
// values) is in the error's context map. A test that wants to
// assert on the full diagnostic renders the error through
// forgeerr.Format, which produces the frozen user-facing format:
//
//	Error: <message>
//	Code:  <code>
//	       <context-key>: <value>
//
// The tests below that assert on the diagnostic use
// forgeerr.Format for that reason. The tests that assert only on
// the error's existence or its sentinel use errors.Is.
package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thapelomagqazana/forge/internal/forgeerr"
)

// =============================================================================
// Valid inputs
// =============================================================================

// TestLoad_ValidMinimal verifies that a minimal valid file
// produces a Config with the file's values.
func TestLoad_ValidMinimal(t *testing.T) {
	t.Parallel()

	got, err := Load("testdata/valid-minimal.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.LogLevel != LogLevelDebug {
		t.Errorf("LogLevel = %q; want %q", got.LogLevel, LogLevelDebug)
	}
	if got.OutputFormat != OutputFormatJSON {
		t.Errorf("OutputFormat = %q; want %q",
			got.OutputFormat, OutputFormatJSON)
	}
	if got.SourcePath != "testdata/valid-minimal.yaml" {
		t.Errorf("SourcePath = %q; want the path",
			got.SourcePath)
	}
}

// TestLoad_ValidFull verifies that a full valid file produces a
// Config with all the file's values.
func TestLoad_ValidFull(t *testing.T) {
	t.Parallel()

	got, err := Load("testdata/valid-full.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.LogLevel != LogLevelWarn {
		t.Errorf("LogLevel = %q; want %q", got.LogLevel, LogLevelWarn)
	}
	if got.OutputFormat != OutputFormatText {
		t.Errorf("OutputFormat = %q; want %q",
			got.OutputFormat, OutputFormatText)
	}
}

// TestLoad_EmptyFileProducesDefaults verifies that an empty file
// produces a Config with zero values.
//
// The loader returns a zero Config for an empty file; the caller
// (a future WBS item) applies the defaults. The test asserts the
// zero Config.
func TestLoad_EmptyFileProducesDefaults(t *testing.T) {
	t.Parallel()

	got, err := Load("testdata/valid-empty.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.LogLevel != "" {
		t.Errorf("LogLevel = %q; want empty", got.LogLevel)
	}
	if got.OutputFormat != "" {
		t.Errorf("OutputFormat = %q; want empty", got.OutputFormat)
	}
}

// TestLoad_EmptyMappingProducesDefaults verifies that a file
// containing only "{}" produces a Config with zero values.
func TestLoad_EmptyMappingProducesDefaults(t *testing.T) {
	t.Parallel()

	// Write a file with "{}".
	dir := t.TempDir()
	path := filepath.Join(dir, "empty-mapping.yaml")
	if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.LogLevel != "" {
		t.Errorf("LogLevel = %q; want empty", got.LogLevel)
	}
	if got.OutputFormat != "" {
		t.Errorf("OutputFormat = %q; want empty", got.OutputFormat)
	}
}

// TestLoad_NullValuesAreUnset verifies that a field set to null is
// treated as "not set" (the empty string).
func TestLoad_NullValuesAreUnset(t *testing.T) {
	t.Parallel()

	got, err := Load("testdata/valid-null-values.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.LogLevel != "" {
		t.Errorf("LogLevel = %q; want empty (null value)", got.LogLevel)
	}
	if got.OutputFormat != "" {
		t.Errorf("OutputFormat = %q; want empty (null value)",
			got.OutputFormat)
	}
}

// TestLoad_CaseInsensitiveEnumAccepted verifies that an enum
// value in a different case is accepted and normalised to
// lowercase.
func TestLoad_CaseInsensitiveEnumAccepted(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "case-insensitive.yaml")
	if err := os.WriteFile(path,
		[]byte("log_level: DEBUG\noutput_format: JSON\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.LogLevel != LogLevelDebug {
		t.Errorf("LogLevel = %q; want %q",
			got.LogLevel, LogLevelDebug)
	}
	if got.OutputFormat != OutputFormatJSON {
		t.Errorf("OutputFormat = %q; want %q",
			got.OutputFormat, OutputFormatJSON)
	}
}

// =============================================================================
// Invalid inputs
// =============================================================================

// TestLoad_UnknownFieldRejected verifies that a file with an
// unknown field produces ErrConfigUnknownField.
//
// The code is E4 (CodeConfigUnknownField). The loader's
// classifyYAMLError recognizes yaml.v3's unknown-field message
// ("field <name> not found in type") and constructs an E4 error.
//
// # Why not ErrConfigInvalid
//
// The pre-WBS-10.5.2 loader classified every parse error as
// ErrConfigInvalid (E6). The current loader distinguishes the
// three shapes: unknown field (E4), type mismatch (E5), and
// syntax error (E3). The distinction tells the user what kind
// of problem the file has; the exit code is ExitConfig (3) for
// all three.
func TestLoad_UnknownFieldRejected(t *testing.T) {
	t.Parallel()

	_, err := Load("testdata/invalid-unknown-field.yaml")
	if !errors.Is(err, ErrConfigUnknownField) {
		t.Fatalf("Load: err = %v; want ErrConfigUnknownField", err)
	}
}

// TestLoad_TypeMismatchRejected verifies that a file with a type
// mismatch produces ErrConfigInvalidType.
//
// The code is E5 (CodeConfigInvalidType). The loader's
// classifyYAMLError recognizes yaml.v3's type-mismatch message
// ("cannot unmarshal ... into ...") and constructs an E5 error.
//
// # Why not ErrConfigInvalid
//
// Same rationale as TestLoad_UnknownFieldRejected: the loader
// distinguishes the three parse-error shapes.
func TestLoad_TypeMismatchRejected(t *testing.T) {
	t.Parallel()

	_, err := Load("testdata/invalid-type.yaml")
	if !errors.Is(err, ErrConfigInvalidType) {
		t.Fatalf("Load: err = %v; want ErrConfigInvalidType", err)
	}
}

// TestLoad_InvalidEnumRejected verifies that a file with an
// invalid enum value produces ErrConfigInvalid.
//
// The code is E6 (CodeConfigInvalidValue). An enum-value error is
// a value error, not a parse error: the YAML document parsed
// successfully, but the field's value is outside the allowed set.
// The loader's validateConfig produces the error.
//
// # The sentinel is the E6 sentinel
//
// ErrConfigInvalid is the sentinel for E6. It is the catch-all
// for the value errors the validator produces. The name is
// historical: it was introduced by WBS 8.4.4 before the
// taxonomy's current shape.
func TestLoad_InvalidEnumRejected(t *testing.T) {
	t.Parallel()

	_, err := Load("testdata/invalid-enum.yaml")
	if !errors.Is(err, ErrConfigInvalid) {
		t.Fatalf("Load: err = %v; want ErrConfigInvalid", err)
	}
}

// TestLoad_DuplicateKeyRejected verifies that a file with a
// duplicate key produces ErrConfigMalformed.
//
// The code is E3 (CodeConfigMalformed). yaml.v3 reports a
// duplicate key as a syntax error (the YAML spec requires keys
// to be unique). The loader's classifyYAMLError falls through to
// E3 for this shape.
//
// # Why E3 and not a dedicated E-code
//
// The taxonomy (WBS 8.5.1) has eight codes. Duplicate keys are
// one of the E3 cases: the YAML document's structure is invalid.
// A future WBS item that wants to distinguish "duplicate key"
// from "syntax error" reads the parser's message and adds a
// code. Until then, the E3 classification is correct: the user
// sees a diagnostic that names the line, and the exit code is
// ExitConfig (3).
func TestLoad_DuplicateKeyRejected(t *testing.T) {
	t.Parallel()

	_, err := Load("testdata/invalid-duplicate-key.yaml")
	if !errors.Is(err, ErrConfigMalformed) {
		t.Fatalf("Load: err = %v; want ErrConfigMalformed", err)
	}
}

// TestLoad_MalformedYAMLRejected verifies that a malformed YAML
// file produces ErrConfigMalformed.
//
// The code is E3 (CodeConfigMalformed). The loader's
// classifyYAMLError falls through to E3 for syntax errors; the
// error's context includes the line and column when the parser
// names them.
//
// # Why E3 and not ErrConfigInvalid
//
// The pre-WBS-10.5.2 loader classified every parse error as
// ErrConfigInvalid (E6). The current loader distinguishes the
// three shapes and classifies a syntax error as E3. The
// distinction is user-visible: the error message names the
// offending line, and the remediation suggests fixing the YAML
// syntax at that line.
func TestLoad_MalformedYAMLRejected(t *testing.T) {
	t.Parallel()

	_, err := Load("testdata/invalid-malformed.yaml")
	if !errors.Is(err, ErrConfigMalformed) {
		t.Fatalf("Load: err = %v; want ErrConfigMalformed", err)
	}
}

// TestLoad_MalformedYAMLIncludesLineNumber verifies that a
// malformed YAML file's error carries a line number in its
// context.
//
// The check renders the error through forgeerr.Format and
// asserts that the rendered string contains a "line:" context
// line. The structured error keeps the message short and puts
// the line number in the context; the user-facing format renders
// both.
//
// # Why the test renders instead of reading err.Error()
//
// err.Error() returns only the short fixed message
// ("<code>: <message>"). The line number is in the error's
// context map, which err.Error() does not include. Rendering the
// error through forgeerr.Format is the way to see the full
// diagnostic.
func TestLoad_MalformedYAMLIncludesLineNumber(t *testing.T) {
	t.Parallel()

	_, err := Load("testdata/invalid-malformed.yaml")
	if err == nil {
		t.Fatal("Load: err = nil; want an error")
	}

	// Render the error and assert that the rendered diagnostic
	// names the line. The "line:" prefix is what the frozen
	// format uses for context keys.
	rendered := forgeerr.Format(err, forgeerr.Options{})
	if !strings.Contains(rendered, "line:") {
		t.Errorf("rendered error does not mention a line:\n%s", rendered)
	}
}

// TestLoad_TooLargeRejected verifies that a file larger than the
// size limit produces ErrConfigTooLarge.
func TestLoad_TooLargeRejected(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "too-large.yaml")
	// Write a file with maxConfigFileSize+1 bytes of content.
	content := strings.Repeat("#", maxConfigFileSize+1)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := Load(path)
	if !errors.Is(err, ErrConfigTooLarge) {
		t.Fatalf("Load: err = %v; want ErrConfigTooLarge", err)
	}
}

// TestLoad_BOMRejected verifies that a file starting with a BOM
// produces an E8 error (ErrConfigInvalidEncoding).
//
// # Why E8 and not a BOM-specific sentinel
//
// The config error taxonomy (WBS 8.5.1) has eight codes. The BOM
// condition is one of the E8 cases: the file's encoding is not
// what the loader expects. The sentinel for E8 is
// ErrConfigInvalidEncoding; there is no separate ErrConfigBOM.
//
// A caller that wants to distinguish "the file has a BOM" from
// "the file is not UTF-8" reads the error's message. The
// taxonomy's codes are coarser than the messages; the CLI's exit
// code is ExitConfig (3) for both.
func TestLoad_BOMRejected(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "bom.yaml")
	// Write the BOM followed by a valid config.
	content := append([]byte{}, utf8BOM...)
	content = append(content, []byte("log_level: debug\n")...)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := Load(path)
	if !errors.Is(err, ErrConfigInvalidEncoding) {
		t.Fatalf("Load: err = %v; want ErrConfigInvalidEncoding", err)
	}
}

// TestLoad_NoEnvironmentExpansion verifies that a value
// containing "${VAR}" is treated as the literal string, not
// expanded.
//
// # What the test asserts
//
//   - A literal value that is a valid enum ("info") is loaded
//     and normalised.
//   - A value that looks like an environment expansion
//     ("${LOG_LEVEL}") is rejected, because it is not a valid
//     enum. The rejection's context contains the literal value.
//
// The check renders the rejected error through forgeerr.Format
// and asserts that the rendered string contains the literal
// value. If the loader had expanded the value, the error's
// context would contain the expansion's result (the empty
// string, or an unrelated value), not the literal string.
func TestLoad_NoEnvironmentExpansion(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Case 1: a literal value that is a valid enum is loaded.
	path := filepath.Join(dir, "literal-valid.yaml")
	content := "log_level: info\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load(%q): %v", path, err)
	}
	if got.LogLevel != LogLevelInfo {
		t.Errorf("LogLevel = %q; want %q", got.LogLevel, LogLevelInfo)
	}

	// Case 2: a value that looks like an environment expansion
	// is rejected; the error's context contains the literal
	// value.
	path2 := filepath.Join(dir, "expansion.yaml")
	content2 := "log_level: ${LOG_LEVEL}\n"
	if err := os.WriteFile(path2, []byte(content2), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err = Load(path2)
	if err == nil {
		t.Fatalf("Load(${LOG_LEVEL}): err = nil; want an error")
	}
	if !errors.Is(err, ErrConfigInvalid) {
		t.Fatalf("Load(${LOG_LEVEL}): err = %v; want ErrConfigInvalid", err)
	}

	// The rendered error contains the literal value in its
	// context. If the loader had expanded the value, the context
	// would contain the expansion's result, not the literal.
	rendered := forgeerr.Format(err, forgeerr.Options{})
	if !strings.Contains(rendered, "${LOG_LEVEL}") {
		t.Errorf("rendered error does not quote the literal value:\n%s",
			rendered)
	}
}

// =============================================================================
// Missing file and empty path
// =============================================================================

// TestLoad_MissingFileErrors verifies that a non-empty path that
// names a missing file produces ErrConfigNotFound.
func TestLoad_MissingFileErrors(t *testing.T) {
	t.Parallel()

	_, err := Load("testdata/this-file-does-not-exist.yaml")
	if !errors.Is(err, ErrConfigNotFound) {
		t.Fatalf("Load: err = %v; want ErrConfigNotFound", err)
	}
}

// TestLoad_EmptyPathReturnsDefaults verifies that an empty path
// returns the defaults with SourcePath set to "".
//
// # Why not ==
//
// Config contains an unexported sources map (WBS 9.2.2), which
// makes the struct non-comparable. The comparison is therefore
// field-by-field. The fields are strings, so each comparison is
// valid.
func TestLoad_EmptyPathReturnsDefaults(t *testing.T) {
	t.Parallel()

	got, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	want := Defaults()
	if got.LogLevel != want.LogLevel {
		t.Errorf("LogLevel = %q; want %q", got.LogLevel, want.LogLevel)
	}
	if got.OutputFormat != want.OutputFormat {
		t.Errorf("OutputFormat = %q; want %q",
			got.OutputFormat, want.OutputFormat)
	}
	if got.SourcePath != want.SourcePath {
		t.Errorf("SourcePath = %q; want %q",
			got.SourcePath, want.SourcePath)
	}
}

// TestLoad_SourcePathIsSet verifies that a successful load sets
// the SourcePath field to the path argument.
func TestLoad_SourcePathIsSet(t *testing.T) {
	t.Parallel()

	const path = "testdata/valid-minimal.yaml"
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.SourcePath != path {
		t.Errorf("SourcePath = %q; want %q", got.SourcePath, path)
	}
}
