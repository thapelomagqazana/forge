// Package forgeerr — error code registry.
//
// This file is the single source of truth for the set of error
// codes that Forge emits. A code is a stable, machine-readable
// identifier of the form FORGE_<CATEGORY>_<SPECIFIC>, where
// <CATEGORY> matches one of the seven members of the taxonomy in
// category.go and <SPECIFIC> disambiguates the code within its
// category.
//
// Stability policy (WBS 10.3.1):
//
//   - A code, once shipped in a release, is frozen forever. Its
//     string value and its meaning never change.
//
//   - Codes are never renamed. If the meaning of a code must
//     change, a new code is added and the old code is marked
//     deprecated.
//
//   - Codes are never reused for different meanings.
//
//   - Deprecated codes remain in the registry, marked
//     deprecated, so that historical logs and scripts continue to
//     resolve.
//
//   - Adding a code requires a PR that touches this file. The
//     test suite in codes_test.go asserts that the registry, the
//     documentation, and the golden file agree.
//
// The registry is a closed set: the constructors in
// constructors.go reject any code not listed here. This is what
// makes the stability policy enforceable rather than aspirational.
//
// Only the Go standard library is used (see docs/dependency-policy.md).
package forgeerr

import "sort"

// Code is a stable, machine-readable error identifier.
//
// A Code is a member of the closed registry enumerated in codeSpecs.
// The zero value ("") is not a valid code and is rejected by the
// constructors.
//
// A Code carries no policy of its own: the category and exit code
// are derived from the code's prefix (see Category). The Code type
// exists so that call sites can pass codes as a distinct type
// rather than as bare strings, which makes accidental interchange
// with messages or context values a compile error rather than a
// runtime bug.
type Code string

// The frozen registry of error codes.
//
// Every constant below is a member of codeSpecs and is validated
// by the test suite. Adding a constant requires:
//
//  1. Adding a row to codeSpecs with the matching prefix,
//     introduced phase, and (optionally) deprecation flag.
//  2. Regenerating testdata/codes/codes.golden.txt and
//     docs/reference/error-codes.golden.txt (the exact command is
//     documented in the test file).
//  3. Adding a row to docs/reference/error-codes.md (the sync
//     test will fail otherwise).
//
// The names of the constants follow the pattern
// Code<Category><Specific>, with the category in CamelCase. The
// string values follow FORGE_<CATEGORY>_<SPECIFIC> in
// UPPER_SNAKE_CASE.

const (
	// --- CLI errors ----------------------------------------------------
	//
	// Invocation-level failures: the user asked for something that
	// does not exist, or supplied a value in the wrong shape. These
	// are the errors a help system is designed to prevent.

	// CodeCLIUnknownCommand indicates an unknown subcommand.
	CodeCLIUnknownCommand Code = "FORGE_CLI_UNKNOWN_COMMAND"

	// CodeCLIUnknownFlag indicates an unrecognised flag.
	CodeCLIUnknownFlag Code = "FORGE_CLI_UNKNOWN_FLAG"

	// CodeCLIInvalidArg indicates a positional argument that does
	// not satisfy the command's contract.
	CodeCLIInvalidArg Code = "FORGE_CLI_INVALID_ARGUMENT"

	// CodeCLIUnknownFormat indicates a --format value that is not
	// one of the supported output formats.
	CodeCLIUnknownFormat Code = "FORGE_CLI_UNKNOWN_FORMAT"

	// --- Config errors -------------------------------------------------
	//
	// Failures loading or validating a configuration file. The
	// distinction between NOT_FOUND, PERMISSION, and MALFORMED is
	// intentional: it lets the CLI suggest the right remediation
	// (create the file, fix permissions, fix the syntax).

	// CodeConfigNotFound indicates the config file does not exist
	// at the expected path.
	CodeConfigNotFound Code = "FORGE_CONFIG_NOT_FOUND"

	// CodeConfigPermission indicates the config file exists but
	// cannot be read or written due to permissions.
	CodeConfigPermission Code = "FORGE_CONFIG_PERMISSION"

	// CodeConfigMalformed indicates the config file is not valid
	// YAML.
	CodeConfigMalformed Code = "FORGE_CONFIG_MALFORMED"

	// CodeConfigUnknownField indicates the config file contains a
	// field that is not part of the schema.
	CodeConfigUnknownField Code = "FORGE_CONFIG_UNKNOWN_FIELD"

	// CodeConfigInvalidType indicates a field has the wrong YAML
	// type (e.g. a string where a list was expected).
	CodeConfigInvalidType Code = "FORGE_CONFIG_INVALID_TYPE"

	// CodeConfigInvalidValue indicates a field has the right type
	// but a value outside the permitted set.
	CodeConfigInvalidValue Code = "FORGE_CONFIG_INVALID_VALUE"

	// CodeConfigTooLarge indicates the config file exceeds the
	// maximum permitted size.
	CodeConfigTooLarge Code = "FORGE_CONFIG_TOO_LARGE"

	// CodeConfigInvalidEncoding indicates the config file is not
	// valid UTF-8.
	CodeConfigInvalidEncoding Code = "FORGE_CONFIG_INVALID_ENCODING"

	// --- Filesystem errors ---------------------------------------------
	//
	// Failures reading or writing files that are not the config
	// file: blueprints, templates, output paths.

	// CodeFilesystemNotFound indicates a referenced path does not
	// exist.
	CodeFilesystemNotFound Code = "FORGE_FILESYSTEM_NOT_FOUND"

	// CodeFilesystemPermission indicates a referenced path exists
	// but cannot be read or written due to permissions.
	CodeFilesystemPermission Code = "FORGE_FILESYSTEM_PERMISSION"

	// CodeFilesystemEscape indicates a path resolves outside the
	// expected root. This is a filesystem error, not a security
	// error: the path was computed, not supplied by an attacker.
	// See CategorySecurity for the attacker-supplied case.
	CodeFilesystemEscape Code = "FORGE_FILESYSTEM_ESCAPE"

	// --- Security errors -----------------------------------------------
	//
	// Deliberate boundary violations. Kept distinct from
	// FILESYSTEM so that scripts can abort on security violations
	// (ExitSecurity = 6) without aborting on ordinary I/O errors.

	// CodeSecurityPathTraversal indicates a user-supplied path
	// attempts to traverse outside the project root.
	CodeSecurityPathTraversal Code = "FORGE_SECURITY_PATH_TRAVERSAL"

	// CodeSecuritySymlinkEscape indicates a symlink resolves to a
	// target outside the project root.
	CodeSecuritySymlinkEscape Code = "FORGE_SECURITY_SYMLINK_ESCAPE"

	// --- Internal errors -----------------------------------------------
	//
	// Bugs in Forge itself. Every error that does not fit another
	// category is INTERNAL; the taxonomy deliberately has no
	// "OTHER" member.

	// CodeInternalPanic indicates a recovered panic at the process
	// boundary.
	CodeInternalPanic Code = "FORGE_INTERNAL_PANIC"

	// CodeInternalUnexpected indicates an invariant that was
	// expected to hold did not. This is the fallback for errors
	// that cannot be classified more specifically.
	CodeInternalUnexpected Code = "FORGE_INTERNAL_UNEXPECTED"
)

// codeSpec is the registry entry for a code. It carries the
// metadata that is not encoded in the Code's string value: the
// phase in which the code was introduced, and whether the code has
// been deprecated.
//
// The category and exit code are derived from the code's prefix
// (see Category), so they are not duplicated here. This is the
// same single-source-of-truth discipline as Category.Prefix and
// Category.ExitCode.
type codeSpec struct {
	// code is the string value. It is redundant with the key in
	// codeSpecs, but keeping it on the struct makes it trivial to
	// iterate and to render the golden file without a second
	// lookup.
	code Code

	// introduced is the WBS phase in which the code was first
	// shipped. It is documentation-only: no behaviour depends on
	// it. It is used by the golden file and by the reference doc.
	introduced string

	// deprecated marks a code that has been superseded. A
	// deprecated code remains in the registry and remains valid
	// for construction; the deprecation is advisory. A future
	// release may emit a warning when a deprecated code is
	// constructed (see WBS 10.5.x), but that is out of scope here.
	deprecated bool
}

// codeSpecs is the canonical registry. The map key is the Code; the
// value carries the metadata.
//
// Ordering is not meaningful here: AllCodes sorts the keys before
// returning them, so the iteration order is deterministic. The map
// is unexported so that callers cannot mutate the registry; they
// use AllCodes for the sorted list and Lookup for a single entry.
var codeSpecs = map[Code]codeSpec{
	// CLI
	CodeCLIUnknownCommand: {code: CodeCLIUnknownCommand, introduced: "Phase 2"},
	CodeCLIUnknownFlag:    {code: CodeCLIUnknownFlag, introduced: "Phase 2"},
	CodeCLIInvalidArg:     {code: CodeCLIInvalidArg, introduced: "Phase 2"},
	CodeCLIUnknownFormat:  {code: CodeCLIUnknownFormat, introduced: "Phase 2"},

	// Config
	CodeConfigNotFound:        {code: CodeConfigNotFound, introduced: "Phase 2"},
	CodeConfigPermission:      {code: CodeConfigPermission, introduced: "Phase 2"},
	CodeConfigMalformed:       {code: CodeConfigMalformed, introduced: "Phase 2"},
	CodeConfigUnknownField:    {code: CodeConfigUnknownField, introduced: "Phase 2"},
	CodeConfigInvalidType:     {code: CodeConfigInvalidType, introduced: "Phase 2"},
	CodeConfigInvalidValue:    {code: CodeConfigInvalidValue, introduced: "Phase 2"},
	CodeConfigTooLarge:        {code: CodeConfigTooLarge, introduced: "Phase 2"},
	CodeConfigInvalidEncoding: {code: CodeConfigInvalidEncoding, introduced: "Phase 2"},

	// Filesystem
	CodeFilesystemNotFound:   {code: CodeFilesystemNotFound, introduced: "Phase 2"},
	CodeFilesystemPermission: {code: CodeFilesystemPermission, introduced: "Phase 2"},
	CodeFilesystemEscape:     {code: CodeFilesystemEscape, introduced: "Phase 2"},

	// Security
	CodeSecurityPathTraversal: {code: CodeSecurityPathTraversal, introduced: "Phase 2"},
	CodeSecuritySymlinkEscape: {code: CodeSecuritySymlinkEscape, introduced: "Phase 2"},

	// Internal
	CodeInternalPanic:      {code: CodeInternalPanic, introduced: "Phase 2"},
	CodeInternalUnexpected: {code: CodeInternalUnexpected, introduced: "Phase 2"},
}

// AllCodes returns every code in the registry, sorted
// lexicographically.
//
// The returned slice is a copy: mutating it does not affect the
// registry. This is the same defensive-copy discipline as
// AllCategories and *Error.Context.
//
// Sorted order is part of the contract. It is used by the golden
// file and by the reference documentation; a change to the sort
// order is a change to the docs, and must be accompanied by a
// regenerated golden file in the same commit.
func AllCodes() []Code {
	out := make([]Code, 0, len(codeSpecs))
	for c := range codeSpecs {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Lookup returns the registry entry for c.
//
// The second return value is false if c is not a member of the
// registry. Callers that need only the boolean should use
// IsRegistered; callers that need the metadata should use Lookup.
//
// Lookup does not panic on an unregistered code: it is used by
// diagnostics that must describe an unknown code without failing.
// The constructors use IsRegistered, which also does not panic,
// and then panic on their own terms with a specific message.
func Lookup(c Code) (introduced string, deprecated bool, ok bool) {
	spec, ok := codeSpecs[c]
	if !ok {
		return "", false, false
	}
	return spec.introduced, spec.deprecated, true
}

// IsRegistered reports whether c is a member of the registry.
//
// It is a thin wrapper around Lookup and exists for two reasons:
// call sites that only need the boolean read more clearly, and the
// constructors want a single, well-named check to call. The
// implementation does not allocate.
func IsRegistered(c Code) bool {
	_, ok := codeSpecs[c]
	return ok
}

// CategoryOf returns the Category that a registered code belongs
// to.
//
// The mapping is derived from the code's prefix: a code is in
// category X if and only if it begins with X.Prefix(). The method
// is defined on Code rather than as a free function so that call
// sites read naturally:
//
//	cat, ok := code.Category()
//
// The second return value is false if the code is not registered.
// CategoryOf does not panic: it is used by diagnostics, and a
// diagnostic that panics on an unknown code is worse than a
// diagnostic that reports the unknown code as such.
func (c Code) Category() (Category, bool) {
	if !IsRegistered(c) {
		return "", false
	}
	for _, cat := range allCategories {
		if hasPrefix(string(c), cat.Prefix()) {
			return cat, true
		}
	}
	// Unreachable if the registry is well-formed: every registered
	// code begins with exactly one category prefix, and every
	// category prefix is unique (TestAC3_PrefixesAreUnique). The
	// branch exists so that a future registry edit that violates
	// the invariant produces a clean diagnostic rather than a
	// silent wrong answer.
	return "", false
}

// hasPrefix is a local alias for strings.HasPrefix, kept private to
// avoid importing strings into this file. The package already
// imports strings in forgeerr.go and constructors.go, but a small
// duplicate here keeps codes.go's dependency surface minimal and
// makes the intent obvious at the call site.
func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
