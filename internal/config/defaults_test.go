// Package config contains unit tests for the CLI configuration
// type.
//
// This file pins the built-in default values (WBS 8.3.1). The
// defaults are the lowest-precedence layer of the configuration
// chain; the tests assert their exact values, their purity, and
// their relationship to the schema's constants.
//
// # How the tests work
//
// The tests call Defaults() and compare the result to the frozen
// values. The tests do not construct a Config value or read a
// source; the function is pure and its contract is the return
// value.
package config

import (
	"strings"
	"testing"
)

// =============================================================================
// The frozen values
// =============================================================================

// TestDefaults_ReturnsLogLevelInfo verifies that the LogLevel
// default is "info".
//
// The value is the balance between "error" (too quiet) and
// "debug" (too noisy). The WBS 8.3.1 table freezes it as "info".
func TestDefaults_ReturnsLogLevelInfo(t *testing.T) {
	t.Parallel()

	got := Defaults()
	if got.LogLevel != LogLevelInfo {
		t.Errorf("Defaults().LogLevel = %q; want %q",
			got.LogLevel, LogLevelInfo)
	}
}

// TestDefaults_ReturnsOutputFormatText verifies that the
// OutputFormat default is "text".
//
// The value is the human-readable format, which is readable in
// any terminal. The WBS 8.3.1 table freezes it as "text".
func TestDefaults_ReturnsOutputFormatText(t *testing.T) {
	t.Parallel()

	got := Defaults()
	if got.OutputFormat != OutputFormatText {
		t.Errorf("Defaults().OutputFormat = %q; want %q",
			got.OutputFormat, OutputFormatText)
	}
}

// TestDefaults_SourcePathIsEmpty verifies that the metadata field
// (SourcePath) has no default.
//
// The field is not a configuration value; its zero value (the
// empty string) means "not loaded from a file". The default does
// not set the field.
func TestDefaults_SourcePathIsEmpty(t *testing.T) {
	t.Parallel()

	got := Defaults()
	if got.SourcePath != "" {
		t.Errorf("Defaults().SourcePath = %q; want empty",
			got.SourcePath)
	}
}

// =============================================================================
// Purity
// =============================================================================

// TestDefaults_IsPure verifies that Defaults() returns the same
// value on repeated calls.
//
// The property is a consequence of the function being pure: it
// reads no global state and writes nothing. The test pins the
// property in case a future change introduces a global.
func TestDefaults_IsPure(t *testing.T) {
	t.Parallel()

	first := Defaults()
	second := Defaults()
	if first != second {
		t.Errorf("Defaults is not pure:\nfirst:  %+v\nsecond: %+v",
			first, second)
	}
}

// TestDefaults_IsDeterministic verifies that Defaults() returns
// the same value across a longer sequence of calls.
//
// The test is a stronger version of TestDefaults_IsPure: it
// exercises the function enough times to catch a hypothetical
// future change that mutates state on every Nth call.
func TestDefaults_IsDeterministic(t *testing.T) {
	t.Parallel()

	want := Defaults()
	for i := 0; i < 10; i++ {
		got := Defaults()
		if got != want {
			t.Errorf("call %d returned %+v; want %+v", i, got, want)
		}
	}
}

// =============================================================================
// The zero value is distinguishable
// =============================================================================

// TestDefaults_ZeroValueWouldBeRejected verifies that the defaults
// are not the zero value.
//
// A zero-value Config has all fields empty. The defaults set
// LogLevel and OutputFormat to non-empty values. The test pins
// the distinction: a future change that set the defaults to the
// zero value would fail the test, and the contributor would
// consider whether "use the default" and "use the zero value" are
// the same concept (they are not: the zero value means "no
// preference", the default means "info" and "text").
func TestDefaults_ZeroValueWouldBeRejected(t *testing.T) {
	t.Parallel()

	defaults := Defaults()
	var zero Config
	if defaults == zero {
		t.Error("Defaults() returned the zero value; " +
			"the defaults must set LogLevel and OutputFormat")
	}
}

// =============================================================================
// Constants are used
// =============================================================================

// TestDefaults_ConstantsAreUsed verifies that the Defaults
// function body references the schema's constants rather than the
// string literals.
//
// # How the check works
//
// The test reads defaults.go and asserts that the source contains
// "LogLevelInfo" and "OutputFormatText" (the constants' names),
// and that the source does not contain the string literals
// `"info"` and `"text"` outside comments.
//
// The check is deliberately textual: a future contributor who
// writes `LogLevel: "info"` (a literal) instead of
// `LogLevel: LogLevelInfo` (the constant) sees the failure and
// either uses the constant or opens an ADR to remove the
// constant.
//
// # What the check does not catch
//
// The check does not catch a literal used in a context where a
// constant would be inappropriate (for example, a string literal
// in a test's error message). The check's scope is the defaults
// file's source; a future file with a different purpose is not
// checked.
func TestDefaults_ConstantsAreUsed(t *testing.T) {
	t.Parallel()

	src := readFile(t, "defaults.go")

	// The function must reference the constants.
	if !strings.Contains(src, "LogLevelInfo") {
		t.Error("defaults.go does not reference LogLevelInfo; " +
			"the default must use the constant, not a string literal")
	}
	if !strings.Contains(src, "OutputFormatText") {
		t.Error("defaults.go does not reference OutputFormatText; " +
			"the default must use the constant, not a string literal")
	}

	// The function must not use the string literals as the
	// default's values. The check strips comments so that a
	// docstring mentioning the literal value is not a violation.
	code := stripComments(src)
	for _, literal := range []string{`"info"`, `"text"`} {
		if strings.Contains(code, literal) {
			t.Errorf("defaults.go contains the string literal %s "+
				"in code; the default must use the constant "+
				"(WBS 8.3.1)", literal)
		}
	}
}

// stripComments removes Go line comments (// ...) and block
// comments (/* ... */) from src, preserving newlines so that line
// numbers in subsequent error messages remain approximately
// correct.
//
// The helper is a copy of the one in internal/cli's tests. The
// two packages are separate; a shared test helper would require a
// third package or a build tag. The duplication is small and
// deliberate.
func stripComments(src string) string {
	var b strings.Builder
	b.Grow(len(src))

	i := 0
	for i < len(src) {
		// Line comment.
		if i+1 < len(src) && src[i] == '/' && src[i+1] == '/' {
			j := i + 2
			for j < len(src) && src[j] != '\n' {
				j++
			}
			if j < len(src) {
				b.WriteByte('\n')
				i = j + 1
			} else {
				i = j
			}
			continue
		}

		// Block comment.
		if i+1 < len(src) && src[i] == '/' && src[i+1] == '*' {
			j := i + 2
			for j+1 < len(src) && !(src[j] == '*' && src[j+1] == '/') {
				if src[j] == '\n' {
					b.WriteByte('\n')
				}
				j++
			}
			if j+1 < len(src) {
				i = j + 2
			} else {
				i = len(src)
			}
			continue
		}

		b.WriteByte(src[i])
		i++
	}

	return b.String()
}
