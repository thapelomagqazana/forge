// Package config contains unit tests for the CLI configuration
// type.
//
// This file tests the Config type as a value: its zero value's
// validity, its comparability, and its field semantics. The
// schema contract is tested in schema_test.go; the scope
// contract is tested in scope_test.go. The three files are
// complementary.
package config

import (
	"strings"
	"testing"
)

// TestConfig_ZeroValueIsValid verifies that a zero-value Config is
// a valid configuration.
//
// The zero value has all three fields empty. An empty LogLevel
// means "use the default level"; an empty OutputFormat means "use
// the command's default format"; an empty SourcePath means "the
// config was not loaded from a file".
//
// # Why the test exists
//
// The scope contract (WBS 8.1.1) says the Config type is a value
// type. A value type's zero value must be usable; if it were not,
// callers would have to construct a Config with non-empty fields
// even when they had no configuration to provide. The zero value's
// validity is what makes the type's use as Dependencies.Config
// possible.
//
// The test also pins the field names: a future rename of a field
// (as happened when ConfigPath became SourcePath in WBS 8.2.1)
// requires updating this test. The compiler catches a field's
// rename at every call site; the test catches it as executable
// documentation.
func TestConfig_ZeroValueIsValid(t *testing.T) {
	t.Parallel()

	var c Config

	// The zero value's fields are all empty. The test asserts
	// that explicitly so that a future change to a field's
	// default is caught.
	if c.LogLevel != "" {
		t.Errorf("zero-value LogLevel = %q; want empty", c.LogLevel)
	}
	if c.OutputFormat != "" {
		t.Errorf("zero-value OutputFormat = %q; want empty", c.OutputFormat)
	}
	if c.SourcePath != "" {
		t.Errorf("zero-value SourcePath = %q; want empty", c.SourcePath)
	}
}

// TestConfig_IsComparable verifies that two Config values can be
// compared with ==.
//
// The type has three string fields and no slices, maps, or
// functions. It is comparable. The property is used by tests that
// assert two Config values are equal; a future change to a field's
// type (for example, to a slice) would break the property and
// would be caught by this test.
//
// # Why the property matters
//
// The Dependencies struct (internal/cli/deps.go) holds a Config
// value. A test that constructs a Dependencies value can compare
// it against another with ==. If Config were not comparable, the
// comparison would be a compile error, and the test would have to
// use reflect.DeepEqual instead. The comparable property keeps
// the tests simple.
func TestConfig_IsComparable(t *testing.T) {
	t.Parallel()

	a := Config{LogLevel: LogLevelInfo, OutputFormat: OutputFormatJSON}
	b := Config{LogLevel: LogLevelInfo, OutputFormat: OutputFormatJSON}
	c := Config{LogLevel: LogLevelDebug, OutputFormat: OutputFormatJSON}

	if a != b {
		t.Error("equal Config values compared unequal")
	}
	if a == c {
		t.Error("unequal Config values compared equal")
	}
}

// TestConfig_FieldSemantics verifies that each field holds the
// value it is set to.
//
// The test is trivial in isolation; its value is as documentation.
// A reader who wants to know how to set a field reads this test.
func TestConfig_FieldSemantics(t *testing.T) {
	t.Parallel()

	c := Config{
		LogLevel:     LogLevelDebug,
		OutputFormat: OutputFormatJSON,
		SourcePath:   "/home/user/.forge.yaml",
	}

	if c.LogLevel != "debug" {
		t.Errorf("LogLevel = %q; want %q", c.LogLevel, "debug")
	}
	if c.OutputFormat != "json" {
		t.Errorf("OutputFormat = %q; want %q", c.OutputFormat, "json")
	}
	if c.SourcePath != "/home/user/.forge.yaml" {
		t.Errorf("SourcePath = %q; want %q",
			c.SourcePath, "/home/user/.forge.yaml")
	}
}

// TestConfig_ValidLogLevels verifies that the four log-level
// constants have the expected values.
//
// The constants are the vocabulary the logging subsystem
// understands. A future change to a constant's value is a change
// to the logging contract; the test would fail and the
// contributor would update the contract.
func TestConfig_ValidLogLevels(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		got  string
		want string
	}{
		{"LogLevelError", LogLevelError, "error"},
		{"LogLevelWarn", LogLevelWarn, "warn"},
		{"LogLevelInfo", LogLevelInfo, "info"},
		{"LogLevelDebug", LogLevelDebug, "debug"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.got != tc.want {
				t.Errorf("%s = %q; want %q", tc.name, tc.got, tc.want)
			}
		})
	}
}

// TestConfig_ValidOutputFormats verifies that the two
// output-format constants have the expected values.
//
// # Why two, not three
//
// WBS 8.1.1 declared three output formats (text, human, json).
// WBS 8.2.1 removed "human" from the schema. The schema's allowed
// set is {text, json}; a future phase that needs "human" adds it
// via an ADR and updates this test.
func TestConfig_ValidOutputFormats(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		got  string
		want string
	}{
		{"OutputFormatText", OutputFormatText, "text"},
		{"OutputFormatJSON", OutputFormatJSON, "json"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.got != tc.want {
				t.Errorf("%s = %q; want %q", tc.name, tc.got, tc.want)
			}
		})
	}
}

// TestConfig_NoOutputFormatHuman verifies that the "human"
// constant is not declared.
//
// WBS 8.2.1 removed the value. The test pins the removal at the
// source level: the string "human" does not appear as a declared
// output-format value in the package's source files.
//
// # How the check works
//
// The test reads the package's source files and looks for the
// string `"human"`. The list of files is the set that could
// declare the constant: config.go (where the other
// OutputFormat* constants are declared) and scope.go (where an
// earlier WBS item declared the constant before WBS 8.2.1 removed
// it).
//
// The check is deliberately textual: a future contributor who
// adds the constant back sees the failure and either removes the
// addition or opens an ADR to re-add the value.
//
// # What the check does not catch
//
// The check does not catch a "human" value declared in a comment
// (for example, "the human format is...") because comments are
// part of the source. The check's purpose is to catch the
// constant's redeclaration, not its mention in prose.
func TestConfig_NoOutputFormatHuman(t *testing.T) {
	t.Parallel()

	files := []string{"config.go", "scope.go"}
	for _, file := range files {
		src := readFile(t, file)
		if containsOutputFormatHuman(src) {
			t.Errorf("%s contains an output-format value \"human\"; "+
				"the schema has two values (text, json), not three "+
				"(WBS 8.2.1)", file)
		}
	}
}

// containsOutputFormatHuman reports whether src contains the
// string `"human"` outside a comment.
//
// # Why the helper exists
//
// The naive check (`strings.Contains(src, "\"human\"")`) matches
// the string in a comment or a docstring. The helper strips
// comment lines before the check, so a mention in prose is not a
// violation. The helper is deliberately simple: it strips lines
// that begin with `//` (after optional whitespace) and lines that
// begin with `*` (the middle of a block comment). It does not
// parse Go's string literals; a future test that needs precise
// parsing should use go/parser.
func containsOutputFormatHuman(src string) bool {
	for _, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		if strings.HasPrefix(trimmed, "*") {
			continue
		}
		if strings.Contains(trimmed, `"human"`) {
			return true
		}
	}
	return false
}
