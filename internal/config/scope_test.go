// Package config contains unit tests for the CLI configuration
// type.
//
// This file pins the CLI configuration scope (WBS 8.1.1, amended
// by ADR-009 and ADR-022). The scope distinguishes three kinds of
// Config field:
//
//   - Scalar value fields (exported, three of them, all strings)
//     carry the scalar configuration values the CLI exposes.
//
//   - Sections (exported, pointer to a struct defined in this
//     package) carry named groups of related configuration values.
//     The first sections were added by WBS 12.7.1 and recognized
//     as a category by ADR-022.
//
//   - Bookkeeping fields (unexported) carry internal state about
//     the value fields; they are not part of the configuration
//     surface.
//
// The tests enforce all three categories against the Config type
// and the package's imports.
//
// # How the tests work
//
// The tests use the reflect package to enumerate the Config type's
// fields. Reflection is the only way to check the type's shape
// without hard-coding the field list; a hard-coded list would
// require an update every time a field is added, which defeats
// the purpose of the test.
//
// The tests do not construct a Config value or invoke any method
// except where the test's purpose requires it (SourceFor, in
// TestScope_SourceForIsBookkeeping).
//
// # Deferred: section-member forbidden-substring check
//
// The forbidden-substring check (TestScope_NoForbiddenFieldNames)
// applies to Config's own fields only. It does not descend into
// section members. Section members include the sensitive fields
// Registry.Token, Registry.Password, and Telemetry.APIKey, whose
// names match the forbidden substrings "token", "password", and
// "api_key" but are intentionally on the surface. Extending the
// check into sections requires an explicit carve-out for those
// fields; that decision is deferred (see ADR-022).
package config

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

// =============================================================================
// The scalar-value-field count
// =============================================================================

// TestScope_ConfigHasExactlyThreeValueFields verifies that the
// Config type has exactly three scalar value (exported, string)
// fields.
//
// # Why the test counts scalar value fields, not all fields
//
// ADR-009 distinguishes scalar value fields from bookkeeping
// fields; ADR-022 adds sections as a third category. The
// scalar-value-field count is frozen at three; the total field
// count and the section count are not. The test counts exported
// fields whose type is string, which are exactly the scalar value
// fields.
//
// A future contributor who adds a fourth scalar value field sees
// the failure. A contributor who adds a section (exported, pointer
// to a struct) or a bookkeeping field (unexported) does not,
// because neither is counted. The distinction is the amendment
// ADR-009 makes, extended by ADR-022.
func TestScope_ConfigHasExactlyThreeValueFields(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeOf(Config{})
	valueCount := 0
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}
		// Sections are exported, but they are not scalar value
		// fields; configFieldCount counts only the latter.
		if field.Type.Kind() != reflect.String {
			continue
		}
		valueCount++
	}

	if valueCount != configFieldCount {
		t.Errorf("Config has %d scalar value fields; want %d\n"+
			"adding a scalar value field requires an ADR (WBS 8.1.1, "+
			"amended by ADR-009) and an update to configFieldCount "+
			"in scope.go",
			valueCount, configFieldCount)
	}
}

// =============================================================================
// The field names
// =============================================================================

// TestScope_FieldNamesAreInScope verifies that every Config field
// is a permitted scalar value field, a permitted section, or a
// permitted bookkeeping field.
//
// # The three lists
//
// The scalar value fields are the exported string fields; their
// names are returned by InScopeFieldNames.
//
// The sections are the exported pointer-to-struct fields; their
// names are returned by InScopeSectionNames.
//
// The bookkeeping fields are the unexported fields; their names
// are returned by InScopeBookkeepingFieldNames.
//
// A field that is not in any of the three lists fails the test.
// The failure names the field and the appropriate list; the
// contributor either renames the field, opens an ADR (for a
// scalar value field or a section), or amends the scope contract
// (for a bookkeeping field).
func TestScope_FieldNamesAreInScope(t *testing.T) {
	t.Parallel()

	permittedValues := make(map[string]bool)
	for _, name := range InScopeFieldNames() {
		permittedValues[name] = true
	}
	permittedSections := make(map[string]bool)
	for _, name := range InScopeSectionNames() {
		permittedSections[name] = true
	}
	permittedBookkeeping := make(map[string]bool)
	for _, name := range InScopeBookkeepingFieldNames() {
		permittedBookkeeping[name] = true
	}

	typ := reflect.TypeOf(Config{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.IsExported() {
			if permittedValues[field.Name] || permittedSections[field.Name] {
				continue
			}
			t.Errorf("Config exported field %q is neither a permitted "+
				"scalar value field nor a permitted section\n"+
				"permitted scalar value fields: %v\n"+
				"permitted sections: %v\n"+
				"adding an exported field requires an ADR (WBS 8.1.1)",
				field.Name, InScopeFieldNames(), InScopeSectionNames())
			continue
		}
		// Unexported: must be a permitted bookkeeping field.
		if !permittedBookkeeping[field.Name] {
			t.Errorf("Config bookkeeping field %q is not in the scope "+
				"contract's list of permitted bookkeeping names\n"+
				"permitted: %v\n"+
				"adding a bookkeeping field requires an amendment "+
				"to the scope contract (ADR-009)",
				field.Name, InScopeBookkeepingFieldNames())
		}
	}
}

// TestScope_NoForbiddenFieldNames verifies that no Config field's
// name contains a substring from the forbidden list.
//
// # Applies to all three kinds of field
//
// The forbidden-substring check applies to scalar value fields,
// sections, and bookkeeping fields alike. A bookkeeping field
// that holds secrets would be an implicit widening of the
// configuration surface, which is what the scope contract
// prevents.
//
// # What the test catches
//
// A future contributor who adds a field named "SecretValue",
// "AuthToken", "RepoPath", or similar. The failure names the
// field and the offending substring.
//
// # Scope of the current application
//
// The test walks Config's own fields. It does not descend into
// section members; see the package docstring's note on the
// deferred section-member check.
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
// The scalar-value-field types
// =============================================================================

// TestScope_ScalarValueFieldsAreStrings verifies that every scalar
// value field of the Config type is a string.
//
// # Why only scalar value fields
//
// The scope contract's scalar value fields are all strings.
// Sections (per WBS 12.7.1, recognized by ADR-022) are
// pointer-to-struct fields and are checked by
// TestScope_SectionsArePointersToStructs instead. Bookkeeping
// fields (per ADR-009) may have other types; the sources field is
// a map. The test checks only the exported fields that are not
// sections.
//
// A future contributor who gives a scalar value field a
// non-string type sees the failure. A contributor who adds a
// section with a pointer-to-struct type, or a bookkeeping field
// with a map or slice type, does not.
//
// # Why string
//
// The three scalar value fields (log level, output format, config
// path) are all values that come from a text-based config file. A
// string is the natural type: the value is what the file
// contains, and the consumer interprets it. A more specific type
// (for example, an enum) would require a parse step at the file
// boundary; the parse is the loader's job, not the type's.
func TestScope_ScalarValueFieldsAreStrings(t *testing.T) {
	t.Parallel()

	sections := make(map[string]bool)
	for _, name := range InScopeSectionNames() {
		sections[name] = true
	}

	typ := reflect.TypeOf(Config{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}
		if sections[field.Name] {
			continue
		}
		if field.Type.Kind() != reflect.String {
			t.Errorf("Config scalar value field %q has type %s; want string\n"+
				"the scope contract's scalar value fields are all strings "+
				"(WBS 8.1.1)", field.Name, field.Type)
		}
	}
}

// =============================================================================
// Sections
// =============================================================================

// TestScope_SectionsArePointersToStructs verifies that every
// section named by InScopeSectionNames is (a) present on the
// Config type, (b) exported, and (c) a pointer to a struct type
// defined in this package.
//
// # Why the check matters
//
// A section's whole purpose is to carry a named group of related
// configuration values. A section field that is not a pointer to
// a struct cannot do that; a section field that is a pointer to a
// struct defined elsewhere couples the config package to another
// package's shape, which the package boundary forbids (see
// TestScope_DoesNotImportBlueprint for the analogous rule).
//
// The test reads the section list from InScopeSectionNames, so it
// stays in sync with the contract without hard-coding the section
// names.
func TestScope_SectionsArePointersToStructs(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeOf(Config{})
	for _, name := range InScopeSectionNames() {
		field, ok := typ.FieldByName(name)
		if !ok {
			t.Errorf("Config has no field named %q, but %q is "+
				"in InScopeSectionNames\n"+
				"either remove the name from the contract or add the "+
				"field", name, name)
			continue
		}
		if !field.IsExported() {
			t.Errorf("section field %q is unexported; sections must "+
				"be exported to be part of the configuration surface",
				name)
			continue
		}
		if field.Type.Kind() != reflect.Ptr {
			t.Errorf("section field %q has type %s; want a pointer "+
				"to a struct", name, field.Type)
			continue
		}
		elem := field.Type.Elem()
		if elem.Kind() != reflect.Struct {
			t.Errorf("section field %q is a pointer to %s; want a "+
				"pointer to a struct", name, elem.Kind())
			continue
		}
		if elem.PkgPath() != typ.PkgPath() {
			t.Errorf("section field %q points to %s, defined in "+
				"package %q; sections must point to structs defined "+
				"in this package", name, elem, elem.PkgPath())
		}
	}
}

// =============================================================================
// Bookkeeping fields
// =============================================================================

// TestScope_SourceForIsBookkeeping verifies that the bookkeeping
// field `sources` is not part of the configuration surface: it is
// unexported, and its purpose is source attribution (WBS 9.2.2).
//
// # What the test asserts
//
//   - `sources` is unexported (already covered by
//     TestScope_FieldNamesAreInScope; this test asserts the
//     narrower claim that the field's existence does not leak
//     into the value surface).
//
//   - SourceFor on a zero Config returns SourceDefault for the
//     scalar-value-field names. This is the observable behavior
//     the bookkeeping field supports, and it is the reason the
//     field exists.
//
// The test goes through the public API (SourceFor) rather than
// reflecting on the field directly, so it pins the behavior the
// bookkeeping field is there to provide.
func TestScope_SourceForIsBookkeeping(t *testing.T) {
	t.Parallel()

	// The zero Config's bookkeeping field is nil. SourceFor must
	// handle the nil case by returning the default for every
	// field, including unknown ones.
	var c Config
	for _, field := range InScopeFieldNames() {
		if got := c.SourceFor(field); got != SourceDefault {
			t.Errorf("zero Config SourceFor(%q) = %v; want %v",
				field, got, SourceDefault)
		}
	}
	if got := c.SourceFor("UnknownField"); got != SourceDefault {
		t.Errorf("zero Config SourceFor(UnknownField) = %v; want %v",
			got, SourceDefault)
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
