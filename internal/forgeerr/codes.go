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
// # The three-layer mapping
//
// This file sits between two taxonomies, and it is important to
// understand which layer owns which property:
//
//  1. The code registry (this file, WBS 10.3.1) owns the set of
//     codes and their string values. A code is registered or it
//     is not; the registry does not assign an exit code.
//
//  2. The category taxonomy (category.go, WBS 10.2.1) owns the
//     mapping from a code's prefix to a Category. The prefix is
//     the first segment of the code's string value: FORGE_CLI_*
//     is in CategoryCLI, FORGE_CONFIG_* is in CategoryConfig, and
//     so on.
//
//  3. The exit-code taxonomy (internal/cli/exitcodes.go, WBS
//     11.1.1) owns the mapping from a Category to a numeric exit
//     code, and groups those codes into documented ranges. The
//     mapping is Category.ExitCode(); the ranges are the
//     ExitCodeRange values published in
//     docs/reference/exit-code-ranges.md and decided in ADR-018.
//
// The three layers are strictly separated. A change to the
// exit-code ranges does not touch this file. A change to the
// category taxonomy does not touch this file either: it touches
// category.go and the prefix rules, and the registry's codes are
// re-interpreted by the new taxonomy because their prefixes are
// unchanged. Only a change to the set of codes touches this file.
//
// # Worked example
//
// The chain for CodeSecurityPathTraversal is:
//
//  1. CodeSecurityPathTraversal = "FORGE_SECURITY_PATH_TRAVERSAL"
//     (this file, WBS 10.3.1).
//  2. The prefix "FORGE_SECURITY_" selects CategorySecurity
//     (category.go, WBS 10.2.1).
//  3. CategorySecurity.ExitCode() returns ExitSecurity
//     (internal/cli/exitcodes.go, WBS 11.1.1). Under the WBS
//     11.1.1 taxonomy, ExitSecurity is 13, in the execution-
//     failure range (10–19). Before the taxonomy, the same
//     category returned 6, in the flat range that no longer
//     exists.
//
// The registry's code string did not change when the exit code
// moved from 6 to 13. This is the property the three-layer
// separation buys: a taxonomy change is absorbed at the layer
// that owns the affected property, and the layers above it are
// untouched.
//
// # Filesystem versus Security
//
// Three codes describe failures at the boundary between a Forge
// process and the filesystem. They are deliberately distinct:
//
//   - FORGE_FILESYSTEM_ESCAPE (CategoryFilesystem) — a path that
//     Forge computed escaped the expected root. This is a bug in
//     Forge, not an attack. The exit code is ExitFilesystem (12).
//
//   - FORGE_SECURITY_PATH_TRAVERSAL (CategorySecurity) — a
//     user-supplied path attempted to traverse outside the
//     project root. This is an attack. The exit code is
//     ExitSecurity (13).
//
//   - FORGE_SECURITY_SYMLINK_ESCAPE (CategorySecurity) — a
//     user-supplied path resolved through a symlink whose target
//     lies outside the project root. This is an attack. The exit
//     code is ExitSecurity (13).
//
// The distinction matters to scripts: a script that operates on
// untrusted input can abort on ExitSecurity without aborting on
// ExitFilesystem, because the two failures have different causes
// and different remedies. Merging the codes would erase that
// distinction and force every script to treat both as a single
// class.
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
//
// # Where the exit code lives
//
// A Code does not carry an exit code. The exit code is a property
// of the Category, not of the Code (see Category.ExitCode). Two
// codes in the same category share an exit code by construction;
// the design is deliberate, because the exit code is a
// caller-facing classification (what kind of failure was this?)
// while the code is a developer-facing identifier (which specific
// failure was it?). The two questions have different stability
// policies and different change cadences, and separating them is
// what lets the exit-code ranges change without touching the
// registry.
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
//
// # The category is derived from the prefix
//
// The string value's prefix determines the Category. The prefix
// is the FORGE_<CATEGORY>_ segment; the specific segment follows.
// A code is well-formed if and only if exactly one category's
// prefix matches the code's start, and the code is registered in
// codeSpecs. Both properties are checked by the test suite
// (TestAC3_PrefixesAreUnique, TestCodes_HaveValidCategory).
//
// # The exit code is not in this file
//
// No constant below carries an exit code, and no docstring below
// names one. The exit code is Category.ExitCode() (category.go),
// and the numeric values are grouped into ranges in
// internal/cli/exitcodes.go (WBS 11.1.1). A reader who wants to
// know the exit code for a code should follow the prefix to the
// category and the category to the exit code.
//
// # Codes added in WBS 13.1.1
//
// The four filesystem codes NOT_LOCAL, DEPTH_EXCEEDED,
// ENTRIES_EXCEEDED, and CASE_COLLISION were added to support
// internal/filesystem/safefs (WBS 13.1.1, ADR-035). They are
// grouped with the existing filesystem codes because their prefix
// is FORGE_FILESYSTEM_; the security counterparts
// (FORGE_SECURITY_PATH_TRAVERSAL and FORGE_SECURITY_SYMLINK_ESCAPE)
// already existed and are unchanged.
//
// # Codes added in WBS 13.5.1
//
// Seven filesystem codes were added to complete the OS-level
// error taxonomy defined by WBS 13.5.1:
//
//	FORGE_FILESYSTEM_EXISTS
//	FORGE_FILESYSTEM_READ_ONLY
//	FORGE_FILESYSTEM_IS_DIRECTORY
//	FORGE_FILESYSTEM_NOT_DIRECTORY
//	FORGE_FILESYSTEM_SYMLINK_LOOP
//	FORGE_FILESYSTEM_RESERVED_NAME
//	FORGE_FILESYSTEM_PATH_TOO_LONG
//
// They are produced by the classify function in
// internal/filesystem/safefs/errors.go when the underlying FS
// returns an OS-level error. See
// docs/reference/filesystem-errors.md for the full taxonomy and
// the cross-platform mapping table.

const (
	// --- CLI errors ----------------------------------------------------
	//
	// Invocation-level failures: the user asked for something that
	// does not exist, or supplied a value in the wrong shape. These
	// are the errors a help system is designed to prevent.
	//
	// Category: CategoryCLI.
	// Exit code: ExitUsage (64, BSD EX_USAGE), per the WBS 11.1.1
	// taxonomy. Usage errors are a singleton range; the CLI
	// category maps to that singleton. Before the taxonomy, the
	// same category returned 2, in the flat range that no longer
	// exists.

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
	//
	// Category: CategoryConfig.
	// Exit code: ExitConfig (10), in the execution-failure range
	// (10–19), per the WBS 11.1.1 taxonomy. A config error means
	// the command could not run; the caller should fix the config
	// and retry. Before the taxonomy, the same category returned
	// 3, in the flat range that no longer exists.

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
	//
	// Category: CategoryFilesystem.
	// Exit code: ExitFilesystem (12), in the execution-failure
	// range (10–19), per the WBS 11.1.1 taxonomy. A filesystem
	// error means the command could not read or write a path it
	// needed; the caller should fix the path or the permissions
	// and retry. Before the taxonomy, the same category returned
	// 4, in the flat range that no longer exists.
	//
	// # Filesystem versus Security
	//
	// Note the deliberate distinction between this category and
	// CategorySecurity. A filesystem error is a computed path that
	// happened to be unreachable, or a name that a caller
	// constructed incorrectly. A security error is a name supplied
	// by an attacker that was refused. The two categories have
	// different exit codes (ExitFilesystem vs ExitSecurity) so
	// that a script can abort on a security violation without
	// aborting on an ordinary I/O error or a programmer mistake.
	//
	// The three boundary-related codes across the two categories
	// form a deliberate triad:
	//
	//   - FORGE_FILESYSTEM_ESCAPE — a computed path escaped
	//     (bug in Forge).
	//   - FORGE_SECURITY_PATH_TRAVERSAL — a user-supplied path
	//     attempted traversal (attack).
	//   - FORGE_SECURITY_SYMLINK_ESCAPE — a user-supplied path
	//     resolved through an escaping symlink (attack).
	//
	// Do not merge them. See the package docstring, section
	// "Filesystem versus Security".

	// CodeFilesystemNotFound indicates a referenced path does not
	// exist.
	CodeFilesystemNotFound Code = "FORGE_FILESYSTEM_NOT_FOUND"

	// CodeFilesystemPermission indicates a referenced path exists
	// but cannot be read or written due to permissions.
	CodeFilesystemPermission Code = "FORGE_FILESYSTEM_PERMISSION"

	// CodeFilesystemEscape indicates a path Forge computed
	// resolves outside the expected root. This is a filesystem
	// error, not a security error: the path was computed, not
	// supplied by an attacker. See the two FORGE_SECURITY_* codes
	// below for the attacker-supplied case.
	CodeFilesystemEscape Code = "FORGE_FILESYSTEM_ESCAPE"

	// CodeFilesystemNotLocal indicates that a name supplied to a
	// safefs.Root method is not a local, forward-slash, cleaned
	// relative path: it is empty, absolute, contains a Windows
	// drive letter, contains a UNC prefix, or is otherwise
	// rejected by filepath.IsLocal.
	//
	// This code is returned by internal/filesystem/safefs when the
	// *structural* form of the name is wrong. When the name is
	// well-formed but attempts to traverse outside the root, the
	// more specific FORGE_SECURITY_PATH_TRAVERSAL is returned
	// instead.
	//
	// Introduced: WBS 13.1.1.
	CodeFilesystemNotLocal Code = "FORGE_FILESYSTEM_NOT_LOCAL"

	// CodeFilesystemDepthExceeded indicates that a recursive walk
	// has exceeded the MaxDepth configured on the Root.
	//
	// This is a security control, not a performance optimisation:
	// a hostile or accidentally deep tree must not exhaust the
	// process. The limit is opt-in per Root via
	// safefs.WithMaxDepth; zero disables the bound.
	//
	// Introduced: WBS 13.1.1.
	CodeFilesystemDepthExceeded Code = "FORGE_FILESYSTEM_DEPTH_EXCEEDED"

	// CodeFilesystemEntriesExceeded indicates that a recursive
	// walk has exceeded the MaxEntries configured on the Root.
	//
	// Like CodeFilesystemDepthExceeded, this is a security
	// control. The limit is opt-in per Root via
	// safefs.WithMaxEntries; zero disables the bound.
	//
	// Introduced: WBS 13.1.1.
	CodeFilesystemEntriesExceeded Code = "FORGE_FILESYSTEM_ENTRIES_EXCEEDED"

	// CodeFilesystemCaseCollision indicates that two names in the
	// same directory collide under Unicode case folding, on a
	// filesystem that is case-insensitive.
	//
	// This check is opt-in per Root via
	// safefs.WithCaseCollisionDetection. It is a no-op on
	// case-sensitive filesystems.
	//
	// Introduced: WBS 13.1.1.
	CodeFilesystemCaseCollision Code = "FORGE_FILESYSTEM_CASE_COLLISION"

	// --- Filesystem errors added by WBS 13.5.1 -------------------------
	//
	// The following seven codes complete the OS-level filesystem
	// error taxonomy. They are produced by the classify function
	// in internal/filesystem/safefs/errors.go when the underlying
	// FS returns an OS-level error, and they are consumed by the
	// existing-file policy that WBS 13.5.1 pins.
	//
	// Category: CategoryFilesystem.
	// Exit code: ExitFilesystem (12).
	//
	// See docs/reference/filesystem-errors.md for the full
	// taxonomy and the cross-platform mapping table
	// (POSIX errno → Windows ERROR_* → code).

	// CodeFilesystemExists indicates the target of an operation
	// exists and the operation is fail-closed: it refuses to
	// overwrite. The caller either chooses a different target or
	// uses the explicit overwrite variant of the operation.
	//
	// This is the code behind safefs.ErrExists. It is the default
	// outcome for Root.Write and Root.Rename when the target
	// exists; see the existing-file policy matrix in
	// docs/reference/filesystem-errors.md.
	//
	// Introduced: WBS 13.5.1.
	CodeFilesystemExists Code = "FORGE_FILESYSTEM_EXISTS"

	// CodeFilesystemReadOnly indicates the filesystem is mounted
	// read-only, or the file's attributes forbid writes.
	//
	// On POSIX systems, this corresponds to EROFS. On Windows, it
	// corresponds to ERROR_WRITE_PROTECT.
	//
	// This is the code behind safefs.ErrReadOnly.
	//
	// Introduced: WBS 13.5.1.
	CodeFilesystemReadOnly Code = "FORGE_FILESYSTEM_READ_ONLY"

	// CodeFilesystemIsDirectory indicates an operation that
	// expected a file found a directory.
	//
	// On POSIX systems, this corresponds to EISDIR. Windows does
	// not have a distinct code for this condition; the Windows
	// classifier maps ERROR_ACCESS_DENIED to this code when the
	// operation's context indicates a directory was targeted.
	//
	// This is the code behind safefs.ErrIsDirectory.
	//
	// Introduced: WBS 13.5.1.
	CodeFilesystemIsDirectory Code = "FORGE_FILESYSTEM_IS_DIRECTORY"

	// CodeFilesystemNotDirectory indicates an operation that
	// expected a directory found a file.
	//
	// On POSIX systems, this corresponds to ENOTDIR. On Windows,
	// it corresponds to ERROR_DIRECTORY.
	//
	// This is the code behind safefs.ErrNotDirectory.
	//
	// Introduced: WBS 13.5.1.
	CodeFilesystemNotDirectory Code = "FORGE_FILESYSTEM_NOT_DIRECTORY"

	// CodeFilesystemSymlinkLoop indicates symlink resolution
	// exceeded the Root's MaxSymlinkChain bound, or the operating
	// system's own limit.
	//
	// On POSIX systems, the OS limit corresponds to ELOOP. On
	// Windows, it corresponds to ERROR_CANT_RESOLVE_FILENAME or
	// ERROR_TOO_MANY_LINKS.
	//
	// This is the code behind safefs.ErrSymlinkLoop.
	//
	// Introduced: WBS 13.5.1.
	CodeFilesystemSymlinkLoop Code = "FORGE_FILESYSTEM_SYMLINK_LOOP"

	// CodeFilesystemReservedName indicates a name is reserved by
	// the operating system.
	//
	// The reserved set exists on Windows only: CON, PRN, AUX,
	// NUL, COM1–COM9, LPT1–LPT9. The set is case-insensitive and
	// applies with or without an extension. On POSIX systems the
	// condition does not arise and the code is unreachable.
	//
	// This is the code behind safefs.ErrReservedName.
	//
	// Introduced: WBS 13.5.1.
	CodeFilesystemReservedName Code = "FORGE_FILESYSTEM_RESERVED_NAME"

	// CodeFilesystemPathTooLong indicates a path exceeds the
	// operating system's maximum length.
	//
	// On POSIX systems, this corresponds to ENAMETOOLONG. On
	// Windows, the \\?\ prefix raises the limit but does not
	// eliminate it; the exceeded-limit error is
	// ERROR_FILENAME_EXCED_RANGE.
	//
	// This is the code behind safefs.ErrPathTooLong.
	//
	// Introduced: WBS 13.5.1.
	CodeFilesystemPathTooLong Code = "FORGE_FILESYSTEM_PATH_TOO_LONG"

	// --- Security errors -----------------------------------------------
	//
	// Deliberate boundary violations. Kept distinct from
	// FILESYSTEM so that scripts can abort on security violations
	// (ExitSecurity, 13) without aborting on ordinary I/O errors
	// (ExitFilesystem, 12).
	//
	// Category: CategorySecurity.
	// Exit code: ExitSecurity (13), in the execution-failure range
	// (10–19), per the WBS 11.1.1 taxonomy. Before the taxonomy,
	// the same category returned 6, in the flat range that no
	// longer exists.
	//
	// # Who returns these codes
	//
	// Every code in this block is returned by
	// internal/filesystem/safefs, and only by safefs. Domain
	// packages never construct one directly: they receive an error
	// wrapping one of these codes from a Root method and propagate
	// it. The CLI maps the error's Category to the exit code
	// without inspecting the specific code, so a new security code
	// added here automatically inherits the security exit code.
	//
	// CodeFilesystemEscape, above, is NOT returned by safefs. It
	// is reserved for internal computations — for example, a
	// path built by joining blueprint fragments — that are
	// expected to remain inside the root by construction and did
	// not. If safefs ever observes such a path it is a bug in the
	// caller, and the escape is reported as an INTERNAL error, not
	// as a security error, because the path was not user-supplied.

	// CodeSecurityPathTraversal indicates a user-supplied path
	// attempts to traverse outside the project root.
	//
	// Introduced: WBS 10.3.1.
	CodeSecurityPathTraversal Code = "FORGE_SECURITY_PATH_TRAVERSAL"

	// CodeSecuritySymlinkEscape indicates a symlink resolves to a
	// target outside the project root.
	//
	// Introduced: WBS 10.3.1.
	CodeSecuritySymlinkEscape Code = "FORGE_SECURITY_SYMLINK_ESCAPE"

	// --- Internal errors -----------------------------------------------
	//
	// Bugs in Forge itself. Every error that does not fit another
	// category is INTERNAL; the taxonomy deliberately has no
	// "OTHER" member.
	//
	// Category: CategoryInternal.
	// Exit code: ExitInternal (70, BSD EX_SOFTWARE), in the
	// internal-error range (70–78), per the WBS 11.1.1 taxonomy.
	// An internal error means Forge has a bug; the caller should
	// report it with --verbose output. Before the taxonomy, the
	// same category returned 1, in the flat range that no longer
	// exists.

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
//
// # Why introduced and deprecated are the only metadata
//
// The two fields are the metadata that is genuinely per-code and
// cannot be derived. Everything else — the category, the exit
// code, the range the exit code falls in — is a property of the
// code's prefix, and is therefore owned by the category taxonomy
// and the exit-code taxonomy. Adding a third per-code field is a
// signal that a property which should be per-category is being
// pushed down to the code, and should be questioned.
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
//
// # Why the registry does not carry exit codes
//
// The exit code is a property of the category (Category.ExitCode),
// not of the code. Storing it on codeSpec would duplicate the
// category's mapping and create a drift hazard: a change to the
// exit-code taxonomy would require editing every codeSpec row, and
// a missed row would produce an inconsistency that the tests would
// have to catch. The three-layer separation (registry → category
// → exit-code range) avoids the duplication: the registry says
// what the code is, the category says what kind of failure it is,
// and the exit-code taxonomy says how the failure is communicated
// to the shell.
//
// # Why the registry is keyed by Code and not by a struct
//
// A map keyed by the code makes IsRegistered O(1) and gives the
// Go compiler a single expression (codeSpecs[c]) for every lookup.
// An alternative design — a slice of codeSpec with a linear scan —
// would be simpler to read but would make the constructor's
// validation O(n) and would risk accidental duplicates (two rows
// with the same code), which the map prevents by construction. The
// registry is small (30 codes after WBS 13.5.1) and the map is the
// right data structure for the access pattern.
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

	// Filesystem — Phase 2 codes
	CodeFilesystemNotFound:   {code: CodeFilesystemNotFound, introduced: "Phase 2"},
	CodeFilesystemPermission: {code: CodeFilesystemPermission, introduced: "Phase 2"},
	CodeFilesystemEscape:     {code: CodeFilesystemEscape, introduced: "Phase 2"},

	// Filesystem — WBS 13.1.1 codes
	CodeFilesystemNotLocal:        {code: CodeFilesystemNotLocal, introduced: "WBS 13.1.1"},
	CodeFilesystemDepthExceeded:   {code: CodeFilesystemDepthExceeded, introduced: "WBS 13.1.1"},
	CodeFilesystemEntriesExceeded: {code: CodeFilesystemEntriesExceeded, introduced: "WBS 13.1.1"},
	CodeFilesystemCaseCollision:   {code: CodeFilesystemCaseCollision, introduced: "WBS 13.1.1"},

	// Filesystem — WBS 13.5.1 codes
	CodeFilesystemExists:       {code: CodeFilesystemExists, introduced: "WBS 13.5.1"},
	CodeFilesystemReadOnly:     {code: CodeFilesystemReadOnly, introduced: "WBS 13.5.1"},
	CodeFilesystemIsDirectory:  {code: CodeFilesystemIsDirectory, introduced: "WBS 13.5.1"},
	CodeFilesystemNotDirectory: {code: CodeFilesystemNotDirectory, introduced: "WBS 13.5.1"},
	CodeFilesystemSymlinkLoop:  {code: CodeFilesystemSymlinkLoop, introduced: "WBS 13.5.1"},
	CodeFilesystemReservedName: {code: CodeFilesystemReservedName, introduced: "WBS 13.5.1"},
	CodeFilesystemPathTooLong:  {code: CodeFilesystemPathTooLong, introduced: "WBS 13.5.1"},

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
//
// # The sort order is lexicographic, not categorical
//
// Lexicographic order groups the codes by category by accident:
// every FORGE_CLI_* sorts together, every FORGE_CONFIG_* sorts
// together, and so on. The grouping is a consequence of the string
// format, not a separate sort key. A reader who wants the codes
// grouped by category should iterate the categories (AllCategories)
// and filter (CategoryOf). A reader who wants the registry's
// canonical order should call AllCodes.
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
//
// # Why Lookup returns three values and not a codeSpec
//
// codeSpec is unexported. Returning it would expose an internal
// type through the public API, which would couple callers to the
// registry's internal shape and would make a future change to the
// struct a breaking change. The three return values are the
// registry's public surface: the phase and the deprecation flag.
// A future change that adds a fourth field to codeSpec would
// require a fourth return value and would be a deliberate breaking
// change, not an accidental one.
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
//
// # Why the implementation does not call Lookup
//
// The implementation reads codeSpecs directly rather than calling
// Lookup. The two are equivalent in behaviour, but the direct read
// is a single map lookup rather than a function call that performs
// a map lookup. The compiler may inline both; the direct form
// removes the question. The docstring above says "thin wrapper";
// the reader should not assume a function call.
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
//
// # The mapping is the registry's category layer
//
// This method is the middle layer of the three-layer mapping
// documented in the package docstring: the registry (this file)
// supplies the codes; CategoryOf supplies the category; and the
// exit-code taxonomy (internal/cli/exitcodes.go) supplies the
// numeric code. CategoryOf does not look at the exit-code
// taxonomy, and does not need to: the mapping from a code to its
// category is a property of the code's string, not of the numeric
// value that the category happens to produce.
//
// # Why the method is on Code and not on Category
//
// The question "what is the category of this code?" is naturally
// asked of the code. The question "what is the exit code of this
// category?" is naturally asked of the category
// (Category.ExitCode). Both methods exist; neither is derived from
// the other. A reader who wants the full chain follows the two
// calls:
//
//	cat, ok := code.Category()
//	if !ok { /* not registered */ }
//	exit := cat.ExitCode()
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
//
// # Why a local alias and not strings.HasPrefix
//
// The alias is one line. Importing strings would add an import to
// this file that is not otherwise needed, and would obscure the
// fact that the file's only string operation is a prefix check.
// The alias is not a micro-optimisation: the compiler emits the
// same code either way. It is a readability choice: the call site
// reads hasPrefix(code, cat.Prefix()), which names the operation.
func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
