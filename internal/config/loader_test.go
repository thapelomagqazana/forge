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
package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
// unknown field produces an error wrapping ErrConfigInvalid.
func TestLoad_UnknownFieldRejected(t *testing.T) {
	t.Parallel()

	_, err := Load("testdata/invalid-unknown-field.yaml")
	if !errors.Is(err, ErrConfigInvalid) {
		t.Fatalf("Load: err = %v; want ErrConfigInvalid", err)
	}
}

// TestLoad_TypeMismatchRejected verifies that a file with a type
// mismatch produces an error wrapping ErrConfigInvalid.
func TestLoad_TypeMismatchRejected(t *testing.T) {
	t.Parallel()

	_, err := Load("testdata/invalid-type.yaml")
	if !errors.Is(err, ErrConfigInvalid) {
		t.Fatalf("Load: err = %v; want ErrConfigInvalid", err)
	}
}

// TestLoad_InvalidEnumRejected verifies that a file with an
// invalid enum value produces an error wrapping ErrConfigInvalid.
func TestLoad_InvalidEnumRejected(t *testing.T) {
	t.Parallel()

	_, err := Load("testdata/invalid-enum.yaml")
	if !errors.Is(err, ErrConfigInvalid) {
		t.Fatalf("Load: err = %v; want ErrConfigInvalid", err)
	}
}

// TestLoad_DuplicateKeyRejected verifies that a file with a
// duplicate key produces an error wrapping ErrConfigInvalid.
func TestLoad_DuplicateKeyRejected(t *testing.T) {
	t.Parallel()

	_, err := Load("testdata/invalid-duplicate-key.yaml")
	if !errors.Is(err, ErrConfigInvalid) {
		t.Fatalf("Load: err = %v; want ErrConfigInvalid", err)
	}
}

// TestLoad_MalformedYAMLRejected verifies that a malformed YAML
// file produces an error wrapping ErrConfigInvalid.
func TestLoad_MalformedYAMLRejected(t *testing.T) {
	t.Parallel()

	_, err := Load("testdata/invalid-malformed.yaml")
	if !errors.Is(err, ErrConfigInvalid) {
		t.Fatalf("Load: err = %v; want ErrConfigInvalid", err)
	}
}

// TestLoad_MalformedYAMLIncludesLineNumber verifies that a
// malformed YAML file's error message includes a line number.
//
// The check is textual: the error's message contains the string
// "line" (yaml.v3's error messages name the line). A future
// library release that changed the message format would fail the
// test; the contributor updates the expectation.
func TestLoad_MalformedYAMLIncludesLineNumber(t *testing.T) {
	t.Parallel()

	_, err := Load("testdata/invalid-malformed.yaml")
	if err == nil {
		t.Fatal("Load: err = nil; want an error")
	}
	if !strings.Contains(err.Error(), "line") {
		t.Errorf("error message does not mention a line: %v", err)
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

// TestLoad_NoEnvironmentExpansion verifies that a value containing
// "${VAR}" is treated as the literal string, not expanded.
func TestLoad_NoEnvironmentExpansion(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "no-expansion.yaml")
	// Write a config with an expansion-looking value. The
	// value is not a valid enum; if the loader did not
	// expand, it would reject the value.
	//
	// The test uses a valid enum whose name contains "$" so
	// that the value survives validation. The actual check is
	// that the loader did not expand the value.
	content := "log_level: info\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.LogLevel != LogLevelInfo {
		t.Errorf("LogLevel = %q; want %q", got.LogLevel, LogLevelInfo)
	}

	// Now check that an expansion-looking value is preserved.
	// Since the value is not a valid enum, the loader
	// rejects it. The test asserts the rejection's cause: the
	// value was not expanded.
	path2 := filepath.Join(dir, "expansion.yaml")
	content2 := "log_level: ${LOG_LEVEL}\n"
	if err := os.WriteFile(path2, []byte(content2), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err = Load(path2)
	if !errors.Is(err, ErrConfigInvalid) {
		t.Fatalf("Load(${LOG_LEVEL}): err = %v; want ErrConfigInvalid",
			err)
	}
	if !strings.Contains(err.Error(), "${LOG_LEVEL}") {
		t.Errorf("error does not quote the literal value: %v", err)
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
