// Package config contains unit tests for the CLI configuration
// type.
//
// This file pins the CLI configuration scope (WBS 8.1.1). The
// scope is deliberately small: three fields, no secrets, no
// repository paths, no project-level configuration. The tests in
// this file enforce the scope against the Config type and against
// the package's imports.
//
// # How the tests work
//
// The tests use the reflect package to enumerate the Config type's
// fields. Reflection is the only way to check the type's shape
// without hard-coding the field list; a hard-coded list would
// require an update every time a field is added, which defeats
// the purpose of the test.
//
// The tests do not construct a Config value or invoke any method.
// The Config type has no methods; the tests are about the type's
// shape, not its behaviour.
package config

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

// =============================================================================
// The field count
// =============================================================================

// TestScope_ConfigHasExactlyThreeFields verifies that the Config
// type has exactly three fields.
//
// The count is frozen by WBS 8.1.1. Adding a fourth field requires
// an ADR; the ADR must update this test and the constant
// configFieldCount.
func TestScope_ConfigHasExactlyThreeFields(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeOf(Config{})
	n := typ.NumField()

	if n != configFieldCount {
		t.Errorf("Config has %d fields; want %d\n"+
			"adding a field requires an ADR (WBS 8.1.1) and an "+
			"update to configFieldCount in scope.go",
			n, configFieldCount)
	}
}

// =============================================================================
// The field names
// =============================================================================

// TestScope_FieldNamesAreInScope verifies that every Config field
// is one of the names the scope contract permits.
//
// The permitted names are returned by InScopeFieldNames. A field
// whose name is not in the list fails the test. The failure names
// the field; the contributor either renames the field to a
// permitted name or opens an ADR to extend the list.
func TestScope_FieldNamesAreInScope(t *testing.T) {
	t.Parallel()

	permitted := make(map[string]bool)
	for _, name := range InScopeFieldNames() {
		permitted[name] = true
	}

	typ := reflect.TypeOf(Config{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			t.Errorf("Config has unexported field %q; "+
				"config fields must be exported so callers can read "+
				"and set them", field.Name)
			continue
		}
		if !permitted[field.Name] {
			t.Errorf("Config field %q is not in the scope contract's "+
				"list of permitted names\n"+
				"permitted: %v\n"+
				"adding a field requires an ADR (WBS 8.1.1)",
				field.Name, InScopeFieldNames())
		}
	}
}

// TestScope_NoForbiddenFieldNames verifies that no Config field's
// name contains a substring from the forbidden list.
//
// The forbidden substrings are the three categories the scope
// contract rejects: secrets and credentials, repository paths,
// and command-specific flags (the last is not a name match; it is
// checked by the field count and the permitted list).
//
// # What the test catches
//
// A future contributor who adds a field named "SecretValue",
// "AuthToken", "RepoPath", or similar. The failure names the
// field and the offending substring.
func TestScope_NoForbiddenFieldNames(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeOf(Config{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if IsForbiddenFieldName(field.Name) {
			t.Errorf("Config field %q is forbidden by the scope contract\n"+
				"secrets, credentials, and repository paths are "+
				"rejected from CLI config (WBS 8.1.1)",
				field.Name)
		}
	}
}

// =============================================================================
// The field types
// =============================================================================

// TestScope_AllFieldsAreStrings verifies that every Config field
// is a string.
//
// The scope contract's three fields are all strings. A future
// field with a different type (for example, a struct or a slice)
// would be a change to the contract's shape and would require an
// ADR. The test pins the shape.
//
// # Why string
//
// The three fields (log level, output format, config path) are
// all values that come from a text-based config file. A string is
// the natural type: the value is what the file contains, and the
// consumer interprets it. A more specific type (for example, an
// enum) would require a parse step at the file boundary; the
// parse is the loader's job, not the type's.
func TestScope_AllFieldsAreStrings(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeOf(Config{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.Type.Kind() != reflect.String {
			t.Errorf("Config field %q has type %s; want string\n"+
				"the scope contract's fields are all strings "+
				"(WBS 8.1.1)",
				field.Name, field.Type)
		}
	}
}

// =============================================================================
// The package boundary
// =============================================================================

// TestScope_DoesNotImportBlueprint verifies that the config
// package does not import the blueprint package.
//
// The two packages have separate responsibilities: config is the
// CLI's configuration (log level, output format); blueprint is
// the project's configuration (name, language, testing, CI). The
// boundary between them is the boundary between CLI-scoped and
// project-scoped data.
//
// # How the check works
//
// The test reads the package's source files and looks for the
// string "internal/blueprint". The check is textual, not
// AST-based; a package that mentions the blueprint package in a
// comment would fail the check, which is the desired
// over-strictness.
//
// # Why the check is textual
//
// A go/parser-based check would be more precise but would add a
// dependency on the parser package. The textual check is
// sufficient for the package's small size; a future contributor
// who adds a mention of the blueprint package sees the failure
// and either removes the mention or opens an ADR.
func TestScope_DoesNotImportBlueprint(t *testing.T) {
	t.Parallel()

	files := []string{
		"config.go",
		"scope.go",
		"doc.go",
	}
	for _, file := range files {
		src := readFile(t, file)
		if strings.Contains(src, "internal/blueprint") {
			t.Errorf("%s mentions internal/blueprint; "+
				"the config package must not depend on the blueprint "+
				"package (WBS 8.1.1)", file)
		}
	}
}

// TestScope_SourcePathIsMetadata verifies that the SourcePath
// field is documented as metadata.
//
// The field is not a config value: it is not read from the config
// file. It is a resolution result, written by the loader after it
// resolves the file's path. The distinction is documented in the
// SourcePath field's docstring and in the package docstring.
//
// # How the check works
//
// The test reads config.go and asserts that the file contains both
// the string "SourcePath" and the string "metadata" (case-
// insensitive). The two strings may appear anywhere in the file;
// the test does not require them to be adjacent. The check is
// deliberately loose: its purpose is to catch a future edit that
// removes the metadata distinction from the file entirely, not to
// pin the docstring's exact wording.
//
// # What the test does not check
//
// The test does not check that "metadata" appears in the
// SourcePath field's docstring specifically. A future contributor
// who moves the word to a different comment in the file would
// still pass the test. The looser check is the correct
// trade-off: a stricter check would require parsing the Go
// source, and the parsing would be more machinery than the
// distinction warrants.
func TestScope_SourcePathIsMetadata(t *testing.T) {
	t.Parallel()

	src := readFile(t, "config.go")
	lower := strings.ToLower(src)

	if !strings.Contains(lower, "sourcepath") {
		t.Fatal("config.go does not mention SourcePath")
	}
	if !strings.Contains(lower, "metadata") {
		t.Errorf("config.go does not mention that SourcePath is metadata\n" +
			"see WBS 8.1.1's scope contract for the distinction " +
			"between config values and resolution results")
	}
}

// =============================================================================
// Test helpers
// =============================================================================

// readFile reads a source file from the package directory and
// returns its contents. It fails the test if the read errors.
//
// The helper is used by the tests that inspect the package's own
// source. It relies on go test's behaviour of running with the
// package directory as the working directory.
func readFile(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(b)
}
