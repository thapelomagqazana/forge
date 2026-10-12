// Package cli_test contains structural tests for the internal/cli
// package.
//
// These tests verify the *shape* of the package rather than its
// runtime behaviour. They are architectural tests: they enforce
// constraints that are invisible to the Go compiler but essential to
// the package's design.
//
// # What this file tests
//
//   - File layout: which files exist, and which must not exist.
//   - Import discipline: which packages may be imported where.
//   - Exported surface: how many functions and constants are exported.
//   - Required metadata: documentation comments and invariants.
//   - Package boundaries: no leakage into other packages.
//   - Test package declarations: the white-box / black-box split.
//
// # What this file does not test
//
//   - Runtime behaviour of any command.
//   - Command output formatting.
//   - Exit codes.
//   - Error formatting.
//
// Those are tested by the white-box tests in root_test.go,
// execute_test.go, version_test.go, deps_test.go, registry_test.go,
// contract_test.go, validate_test.go, flags_test.go,
// exitcodes_test.go, exitcodes_semantics_test.go,
// exitcodes_compat_test.go, and mapping_test.go, which live in
// package cli (not cli_test).
//
// # Why structural tests
//
// The CLI package's design depends on properties that the compiler
// does not enforce:
//
//   - Cobra is imported only under internal/cli. If a second package
//     imports it, the CLI framework leaks into a layer that should
//     not know about it.
//   - Only a small set of functions and constants may be exported
//     from the package. If a new one appears, downstream packages
//     can depend on internals, which locks the design in place.
//     The exit-code taxonomy (WBS 11.1.1) added eight functions
//     and twenty-five constants; the per-code enumeration and
//     severity classification (WBS 11.1.2) added eight more
//     constants (one new code and seven aliases) and four severity
//     classes; the error-to-exit-code mapping (WBS 11.2.1) added
//     one function (ExitCode). Each was reviewed and cited.
//   - The package must contain no unused files. Orphaned source files
//     accumulate over time and create confusion.
//
// Without structural tests, these constraints decay silently. With
// them, any violation fails CI immediately and forces a deliberate
// decision.
package cli_test

import (
	"bufio"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// =============================================================================
// Test helpers
// =============================================================================

// packageDir returns the absolute path to the internal/cli directory.
//
// It derives the path from the location of this test file, using
// runtime.Caller to find the source file path, then returning the
// directory that contains it.
//
// The helper is deliberately robust: it does not depend on the current
// working directory, which Go's testing framework sets to the package
// directory but which could in principle be changed by a test's setup
// code.
func packageDir(t *testing.T) string {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	return filepath.Dir(thisFile)
}

// listGoFiles returns the sorted list of non-test *.go files in the
// package directory.
//
// Test files are excluded because they may legitimately import Cobra
// for the purpose of testing it, and are not subject to the Cobra
// import-scope rule.
func listGoFiles(t *testing.T) []string {
	t.Helper()

	dir := packageDir(t)

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read package directory: %v", err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		files = append(files, name)
	}

	sort.Strings(files)
	return files
}

// readFile reads a file in the package directory and returns its
// contents as a string.
func readFile(t *testing.T, name string) string {
	t.Helper()

	path := filepath.Join(packageDir(t), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(data)
}

// findModuleRoot walks up from the package directory until it finds
// go.mod, and returns the directory containing it.
func findModuleRoot(t *testing.T) string {
	t.Helper()

	dir := packageDir(t)

	for {
		candidate := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(candidate); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("go.mod not found above %s", packageDir(t))
		}
		dir = parent
	}
}

// =============================================================================
// File layout
// =============================================================================

// expectedSourceFiles enumerates every non-test Go source file the
// package is permitted to contain, along with the WBS item that
// introduced it.
//
// The map is deliberately explicit rather than glob-based. When a new
// file is added in a future WBS, this map must be updated as part of
// that WBS. The update is a review checkpoint: it forces the author
// to justify the new file.
//
// This constant is used by both TestExpectedFilesExist and
// TestNoOrphanedFiles. Keeping the allowlist in one place ensures the
// two tests never disagree.
//
// Test files (*_test.go) are not listed here; listGoFiles excludes
// them. The test-file package declarations are checked separately by
// TestTestFilePackageDeclarations.
var expectedSourceFiles = map[string]string{
	"doc.go":       "WBS 2.4.2 — package documentation and public contract",
	"execute.go":   "WBS 2.4.2 — Execute and executeWithOptions",
	"exitcodes.go": "WBS 2.4.2 / WBS 11.1.1 / WBS 11.1.2 — exit code constants, range taxonomy, classifiers, and severity",
	"mapping.go":   "WBS 11.2.1 — ExitCode(err error) int, the error-to-exit-code mapping contract",
	"root.go":      "WBS 2.4.2 — root command constructor",
	"deps.go":      "WBS 4.2.2 — Dependencies injection struct and Logger",

	// WBS 4.3.1 adds the version handler. It is the reference
	// implementation of the handler / service boundary; the
	// application logic it delegates to lives in
	// internal/app/version.
	"version.go": "WBS 4.3.1 — thin handler for the version command",

	"registry.go": "WBS 4.4.1 — central command registry",
	"config.go":   "WBS 4.4.1 — hidden placeholder for `forge config`",

	// WBS 5.2.2 adds the pre-parse argument validator. It rejects
	// malformed invocations that Cobra would otherwise accept
	// silently. See docs/cli-ux-spec.md § 4.10 and § 4.11.
	"validate.go": "WBS 5.2.2 — pre-parse argument validator",

	// WBS 5.3.1 adds the global flag registration and the helpers
	// that read the parsed values. The three Phase 2 global flags
	// are documented in docs/cli-ux-spec.md § 4.12.
	"flags.go":            "WBS 5.3.1 — global flag registration and readers",
	"hooks.go":            "WBS 5.3.2 — PersistentPreRunE hook",
	"help_content.go":     "WBS 7.1.3 — per-command help content contract constants and helpers",
	"metadata.go":         "WBS 7.2.1 — command metadata contract rules and helpers",
	"errors.go":           "WBS 7.3.1 — invalid-command contract constants",
	"examples.go":         "WBS 7.2.2 — Examples convention rules and helpers",
	"stream_boundary.go":  "WBS 7.4.1 — output stream boundary rules and fixtures",
	"ascii_policy.go":     "WBS 7.4.2 — ASCII-only output policy predicate and constants",
	"flag_interaction.go": "WBS 7.4.3 — global flag interaction contract rules and fixtures",
	"flags_adapter.go":    "WBS 4.3.1 (follow-up) — adapter implementing config.FlagReader over a *cobra.Command",

	// WBS 11.3.2 adds the JSON error envelope and the
	// format-aware writers. The two files are a single unit: the
	// envelope's encoder produces the JSON document; the writers
	// dispatch on the OutputFormat and route the document to
	// stdout. Splitting them into one file would mix the
	// encoding concern (a pure data transformation) with the
	// dispatch concern (a decision that depends on the
	// invocation's flags).
	//
	// error_envelope.go defines the errorEnvelope and
	// errorEnvelopeError types and the encodeErrorEnvelope
	// function. The envelope's schema is published at
	// schemas/error-envelope.schema.json.
	//
	// output.go defines the OutputFormat type and its two values
	// (FormatText, FormatJSON), the writeFailure and
	// writeSuccessResult functions, and the successEnvelope type.
	"error_envelope.go": "WBS 11.3.2 — JSON error envelope encoder",
	"output.go":         "WBS 11.3.2 — format-aware stdout/stderr writers",
}

// TestExpectedFilesExist verifies that every file this WBS expects is
// present in the package.
func TestExpectedFilesExist(t *testing.T) {
	t.Parallel()

	files := listGoFiles(t)

	for name, reason := range expectedSourceFiles {
		found := false
		for _, actual := range files {
			if actual == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected file missing: %s (required by %s)",
				name, reason)
		}
	}
}

// TestNoOrphanedFiles verifies that every Go file in the package is
// one of the files this WBS explicitly expects.
//
// The failure message tells the contributor exactly what to do:
// update the allowlist and cite the WBS item that introduced the new
// file.
func TestNoOrphanedFiles(t *testing.T) {
	t.Parallel()

	files := listGoFiles(t)

	for _, f := range files {
		if _, ok := expectedSourceFiles[f]; !ok {
			t.Errorf("unexpected Go file in package: %s\n"+
				"If this file was intentionally added, update the "+
				"expectedSourceFiles map in structure_test.go and "+
				"cite the WBS item that introduced it.", f)
		}
	}
}

// =============================================================================
// Import discipline
// =============================================================================

// cobraImportPath is the canonical import path for Cobra. Used by
// multiple tests below.
const cobraImportPath = `"github.com/spf13/cobra"`

// TestRootFileImportsCobra verifies that root.go imports Cobra. This
// is the positive half of the AC6 requirement from WBS 2.4.1: Cobra
// must be wired in.
//
// The test is kept after WBS 4.3.1 because root.go is still the file
// that constructs the root command. The version handler also imports
// Cobra (see version.go), which is expected; the test asserts only
// that root.go is among the importers, not that it is the only one.
func TestRootFileImportsCobra(t *testing.T) {
	t.Parallel()

	content := readFile(t, "root.go")

	if !strings.Contains(content, cobraImportPath) {
		t.Fatalf("root.go does not import Cobra; expected to find %s",
			cobraImportPath)
	}
}

// TestCobraImportedOnlyInCliPackage verifies that Cobra is imported
// only by files under internal/cli, and never by files outside it.
//
// # Why the rule is about packages, not files
//
// Before WBS 4.3.1, the rule was "exactly one file imports Cobra".
// That was correct when internal/cli/root.go was the only file that
// constructed commands. WBS 4.3.1 introduces the handler / service
// pattern: each command's handler lives in its own file, and each
// handler constructs a *cobra.Command. The number of Cobra importers
// under internal/cli grows with the number of commands, which is
// expected.
//
// The rule that matters is the package boundary: Cobra is a
// CLI-layer concern. It must not leak into internal/app (the
// application layer), internal/domain (the domain layer), or
// internal/infra (the infrastructure layer). A file under
// internal/cli may import Cobra; a file outside internal/cli may
// not.
//
// # Why a violation matters
//
// If an application service imports Cobra, the boundary between the
// handler and the service collapses: the service becomes aware of
// the CLI framework, and a future replacement of Cobra would touch
// every service. The rule exists to keep the CLI framework at the
// CLI layer.
func TestCobraImportedOnlyInCliPackage(t *testing.T) {
	t.Parallel()

	moduleRoot := findModuleRoot(t)

	allowedPrefix := filepath.Join("internal", "cli") + string(filepath.Separator)

	var offenders []string

	err := filepath.Walk(moduleRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if name == ".git" || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, relErr := filepath.Rel(moduleRoot, path)
		if relErr != nil {
			return nil
		}
		if strings.HasPrefix(rel, allowedPrefix) {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		if strings.Contains(string(data), cobraImportPath) {
			offenders = append(offenders, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk module: %v", err)
	}

	if len(offenders) > 0 {
		t.Fatalf("Cobra is imported outside internal/cli: %v\n"+
			"Cobra is a CLI-layer concern. Move the affected code into "+
			"internal/cli, or add an ADR explaining why the dependency "+
			"must be imported elsewhere.", offenders)
	}
}

// TestStdlibOnlyForRootImports verifies that root.go imports only
// standard-library packages, Cobra, and packages under Forge's own
// module path.
//
// This is a defence against accidental dependency creep. If a new
// third-party import appears in root.go, this test fails and forces
// the author to update docs/dependency-policy.md and the ADR suite.
func TestStdlibOnlyForRootImports(t *testing.T) {
	t.Parallel()

	content := readFile(t, "root.go")
	imports := parseImportPaths(t, content)

	const forgeModulePrefix = "github.com/thapelomagqazana/forge/"

	for _, imp := range imports {
		// Cobra is explicitly permitted.
		if imp == "github.com/spf13/cobra" {
			continue
		}

		// Standard library: no dots in the first path segment.
		firstSegment := imp
		if idx := strings.Index(imp, "/"); idx > 0 {
			firstSegment = imp[:idx]
		}
		if !strings.Contains(firstSegment, ".") {
			continue
		}

		// Forge's own packages are permitted.
		if strings.HasPrefix(imp, forgeModulePrefix) {
			continue
		}

		t.Errorf("forbidden import in root.go: %s\n"+
			"root.go may only import the standard library, Cobra, and "+
			"Forge's own packages. Any other import requires updating "+
			"docs/dependency-policy.md and adding an ADR.", imp)
	}
}

// TestStdlibOnlyForExecuteImports verifies that execute.go imports
// only standard-library packages, Cobra, and packages under Forge's
// own module path.
//
// The same discipline that applies to root.go applies to every file
// in the package. execute.go is checked separately because it is the
// second file most likely to attract a new dependency.
func TestStdlibOnlyForExecuteImports(t *testing.T) {
	t.Parallel()

	content := readFile(t, "execute.go")
	imports := parseImportPaths(t, content)

	const forgeModulePrefix = "github.com/thapelomagqazana/forge/"

	for _, imp := range imports {
		// Standard library: no dots in the first path segment.
		firstSegment := imp
		if idx := strings.Index(imp, "/"); idx > 0 {
			firstSegment = imp[:idx]
		}
		if !strings.Contains(firstSegment, ".") {
			continue
		}

		// Forge's own packages are permitted.
		if strings.HasPrefix(imp, forgeModulePrefix) {
			continue
		}

		t.Errorf("forbidden import in execute.go: %s\n"+
			"execute.go may only import the standard library and "+
			"Forge's own packages. Any other import requires updating "+
			"docs/dependency-policy.md and adding an ADR.", imp)
	}
}

// TestStdlibOnlyForVersionImports verifies that version.go imports
// only standard-library packages, Cobra, and packages under Forge's
// own module path.
//
// version.go is the reference implementation of the handler / service
// boundary (WBS 4.3.1). Its import set is deliberately narrow: Cobra
// for the command definition, and internal/app/version for the
// service it delegates to. Any third-party import in version.go would
// be a violation of the boundary.
func TestStdlibOnlyForVersionImports(t *testing.T) {
	t.Parallel()

	content := readFile(t, "version.go")
	imports := parseImportPaths(t, content)

	const forgeModulePrefix = "github.com/thapelomagqazana/forge/"

	for _, imp := range imports {
		// Cobra is explicitly permitted: version.go constructs a
		// *cobra.Command.
		if imp == "github.com/spf13/cobra" {
			continue
		}

		// Standard library: no dots in the first path segment.
		firstSegment := imp
		if idx := strings.Index(imp, "/"); idx > 0 {
			firstSegment = imp[:idx]
		}
		if !strings.Contains(firstSegment, ".") {
			continue
		}

		// Forge's own packages are permitted.
		if strings.HasPrefix(imp, forgeModulePrefix) {
			continue
		}

		t.Errorf("forbidden import in version.go: %s\n"+
			"version.go may only import the standard library, Cobra, "+
			"and Forge's own packages. Any other import requires "+
			"updating docs/dependency-policy.md and adding an ADR.", imp)
	}
}

// =============================================================================
// Exported surface — functions
// =============================================================================

// expectedExportedFunctions enumerates every function the cli
// package is permitted to export, along with the WBS item or ADR
// that introduced it.
//
// # Why the map is explicit
//
// The CLI package's exported function surface is deliberately
// small. The map is the allowlist; adding a function requires
// updating the map, which forces the contributor to justify the
// addition in a citation. Removing a function is a breaking
// change for any importer and requires the same review.
//
// # The entries
//
//   - Execute is the CLI's process entry point. It is called by
//     cmd/forge/main.go. WBS 4.1.1 froze its signature.
//
//   - ConfigFromContext is the accessor a command handler uses to
//     read the resolved Config from the command's context. WBS
//     8.4.5 introduced it. The export is required because command
//     handlers (in internal/cli) and future service packages (in
//     internal/app/<name>) both need to read the resolved
//     configuration from the command's context.
//
//   - ExitCode is the error-to-exit-code mapping contract. WBS
//     11.2.1 introduced it. The export is required because
//     cmd/forge/main.go and downstream consumers (IDE tooling,
//     wrapper scripts, CI integrations) need the same mapping;
//     the mapping is a public contract, not an implementation
//     detail. It is documented in docs/reference/error-mapping.md.
//
//   - The eight taxonomy functions (ExitCodeRanges,
//     LookupExitCodeRange, VerifyExitCodeRange, IsFinding,
//     IsExecutionFailure, IsUsageError, IsInternalError, IsSignal)
//     are the introspection surface of the WBS 11.1.1 exit-code
//     taxonomy. They are exported because:
//
//   - Documentation generators and IDE tooling need to read
//     the range table (ExitCodeRanges) and find the range
//     containing a code (LookupExitCodeRange).
//
//   - CI checks and wrapper scripts need to validate that an
//     observed exit code is in the documented namespace
//     (VerifyExitCodeRange).
//
//   - Callers that branch on the two failure domains
//     (findings vs. execution failures) need the classifiers
//     (IsFinding, IsExecutionFailure, IsUsageError,
//     IsInternalError, IsSignal) rather than the numeric range
//     boundaries. The classifiers are the stable contract; the
//     boundaries are the machine-readable form of the same
//     information.
//
//     The eight functions are the only exported API the taxonomy
//     adds. The range table itself is returned by value from
//     ExitCodeRanges; there is no exported mutable table.
//
// # Extending the map
//
// A future WBS item that adds a function adds a row with its
// citation. The row's presence is the record of the addition; the
// test's iteration is what enforces the policy.
//
// # Why the map's values are citations
//
// The values are not consulted by the test; they are for a human
// reader who wants to know why a function is on the list. The
// citation names the WBS item or ADR; that document names the
// rationale.
var expectedExportedFunctions = map[string]string{
	"Execute":           "WBS 4.1.1 — the CLI's process entry point",
	"ConfigFromContext": "WBS 8.4.5 — accessor for the resolved configuration",

	// WBS 11.2.1 — the error-to-exit-code mapping contract.
	//
	// ExitCode is the mapping function the process boundary calls
	// to convert a Go error into a process exit code. It is
	// exported because cmd/forge/main.go and downstream consumers
	// (IDE tooling, wrapper scripts) need the same mapping; the
	// mapping is a public contract, not an implementation detail.
	//
	// The function is defined in internal/cli/mapping.go and
	// documented in docs/reference/error-mapping.md. It is
	// distinct from the boundary's unexported helper exitCodeOf
	// (in execute.go): exitCodeOf assumes a *forgeerr.Error and
	// reads its category's code; ExitCode handles every error
	// shape (nil, sentinels, wrapped chains, fallback).
	"ExitCode": "WBS 11.2.1 — the error-to-exit-code mapping contract",

	// WBS 11.1.1 — exit-code taxonomy introspection surface.
	"ExitCodeRanges":      "WBS 11.1.1, ADR-018 — the documented range table",
	"LookupExitCodeRange": "WBS 11.1.1, ADR-018 — find the range containing a code",
	"VerifyExitCodeRange": "WBS 11.1.1, ADR-018 — validate a code is in [0, 255]",

	// WBS 11.1.1 — the two-failure-domains classifiers.
	"IsFinding":          "WBS 11.1.1, ADR-018 — findings range classifier (1–9)",
	"IsExecutionFailure": "WBS 11.1.1, ADR-018 — execution-failure range classifier (10–19)",
	"IsUsageError":       "WBS 11.1.1, ADR-018 — usage-error singleton classifier (64)",
	"IsInternalError":    "WBS 11.1.1, ADR-018 — internal-error range classifier (70–78)",
	"IsSignal":           "WBS 11.1.1, ADR-018 — signal-termination classifier (129–255)",

	// WBS 11.3.1 — the process boundary.
	//
	// Run is the primary entry point introduced by WBS 11.3.1. It
	// receives the caller's context, parses the arguments,
	// dispatches to the matching command, renders any error, and
	// returns the process exit code and the error. It does not
	// call os.Exit; main does.
	//
	// Execute (above) is preserved as a thin wrapper around Run
	// for backward compatibility. Both functions are on the
	// allowlist; the pair is the entry-point contract.
	"Run": "WBS 11.3.1 — the process boundary entry point (Run(ctx) (int, error))",
}

// TestOnlyExpectedFunctionsAreExported verifies that the package
// exports exactly the functions the CLI contract requires, and
// nothing more.
//
// # What the test asserts
//
// The test iterates two sets:
//
//  1. The package's actual exported functions (collected from the
//     source by collectExportedFunctions). Every actual function
//     must be in expectedExportedFunctions.
//
//  2. The expected functions. Every entry in
//     expectedExportedFunctions must be present in the package.
//
// The two iterations together enforce a bijection: the actual set
// equals the expected set. A function in one but not the other
// fails the test.
//
// # Why the test is a bijection
//
// The bijection catches both directions of drift:
//
//   - A contributor adds a function without updating the map: the
//     "every actual must be expected" iteration fails.
//
//   - A contributor removes a function without removing its map
//     entry: the "every expected must be present" iteration
//     fails.
//
// The two failures have different messages, and both cite the
// map's role as the review checkpoint.
//
// # Relationship to the constants test
//
// Constants are tested separately by
// TestOnlyExpectedConstantsAreExported. They are a different
// category of public surface: an enumerated vocabulary that
// downstream code may reference by name. Functions are the
// package's behaviour; constants are its data.
//
// # The scan
//
// The test scans every non-test source file in the package and
// collects top-level function declarations whose identifier
// begins with an uppercase letter. Methods (functions with a
// receiver) are excluded: they are attached to their receiver's
// type, not to the package.
func TestOnlyExpectedFunctionsAreExported(t *testing.T) {
	t.Parallel()

	files := listGoFiles(t)
	actual := collectExportedFunctions(t, files)

	// Every actual function must be in the expected set.
	for _, name := range actual {
		if _, ok := expectedExportedFunctions[name]; !ok {
			t.Errorf("unexpected exported function: %s\n"+
				"If this function was intentionally added, update the "+
				"expectedExportedFunctions map in structure_test.go "+
				"and cite the WBS item or ADR that introduced it.", name)
		}
	}

	// Every expected function must be present.
	for name, reason := range expectedExportedFunctions {
		found := false
		for _, actualName := range actual {
			if actualName == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected exported function missing: %s (%s)",
				name, reason)
		}
	}
}

// collectExportedFunctions returns the sorted list of exported
// top-level function identifiers across the given files.
//
// Methods (functions with a receiver) are excluded: they are attached
// to their receiver's type, not to the package. A method is exported
// only if both the method name and the receiver type are exported,
// which is separately checked when the type is added.
func collectExportedFunctions(t *testing.T, files []string) []string {
	t.Helper()

	dir := packageDir(t)
	fset := token.NewFileSet()

	seen := make(map[string]bool)

	for _, file := range files {
		path := filepath.Join(dir, file)
		parsed, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}

		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if fn.Recv != nil {
				continue
			}
			if !fn.Name.IsExported() {
				continue
			}
			seen[fn.Name.Name] = true
		}
	}

	var out []string
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// =============================================================================
// Exported surface — constants
// =============================================================================

// expectedConstants enumerates every exported constant the package is
// permitted to expose, along with the WBS item or ADR that
// introduced it.
//
// # Five kinds of exported constant
//
// The map contains five categories of exported constant:
//
//  1. The exit-code constants (WBS 11.1.1). These form a stable
//     numeric vocabulary. They are exported because downstream
//     code, tests, and wrapper scripts may reference them by name
//     to interpret the CLI's exit codes. The set includes the
//     named codes (ExitDriftDetected, ExitConfig, …), the range
//     boundaries (ExitRangeFindingsMin, …), the POSIX-derived
//     singletons (ExitSuccess, ExitUsage, ExitInvalidArgument,
//     ExitSIGHUP, ExitSIGINT, ExitSIGTERM), and the three
//     Stability class constants (StabilityFrozen,
//     StabilityAdditive, StabilityReserved).
//
//  2. The exit-code aliases and the ambiguity code (WBS 11.1.2).
//     The aliases (ExitUpdateConflict, ExitInternalError,
//     ExitTemplateNotFound, ExitTemplateIncompatible,
//     ExitRenderingFailed, ExitPathSafetyViolation,
//     ExitSecurityPolicyViolation) give the WBS 11.1.2 vocabulary
//     a Go symbol; their values are identical to their canonical
//     constants by construction. ExitAmbiguity (4) is the one
//     genuinely new code the enumeration adds.
//
//  3. The severity classes (WBS 11.1.2). The four string-typed
//     constants of the Severity type (SeverityOK, SeverityWarn,
//     SeverityError, SeverityFatal). They are exported so that
//     callers compare a code's severity to a named value rather
//     than to a string literal, and so that the YAML's
//     `severity` field and the Go source agree on the vocabulary.
//
//  4. The root identity constants (RootName through RootLongDesc),
//     added by WBS 5.1.1. These form a stable string vocabulary.
//     They are exported because the tests in root_test.go assert
//     on them and the specification in docs/cli-ux-spec.md § 4.7
//     quotes them.
//
//  5. The global flag-name constants (FlagVerbose, FlagQuiet,
//     FlagConfig, FlagFormat, FlagFormatText, FlagFormatJSON),
//     added by WBS 5.3.1 and WBS 9.4.1. These form a stable
//     string vocabulary that the flag readers and the tests
//     reference by name. The names themselves are part of the
//     CLI's surface: scripts and documentation use them.
//
// # Why the map is explicit
//
// The map is deliberately explicit. Adding a constant requires
// updating this map, which forces the contributor to justify the
// addition. Removing a constant is a breaking change and requires
// an ADR, because downstream code may depend on the name.
//
// The update is a review checkpoint: a reviewer who sees the map
// change asks "why did the exported surface grow?" and the answer
// must be in the diff.
//
// # The WBS 4.1.2 → WBS 11.1.1 migration
//
// The WBS 4.1.2 exit-code constants (ExitSuccess, ExitFailure,
// ExitUsage, ExitConfig, ExitFilesystem, ExitValidation,
// ExitSecurity, ExitConflict) were replaced by the WBS 11.1.1
// taxonomy. The mapping is documented in ADR-018 § "Migration from
// WBS 4.1.2". In summary:
//
//   - ExitFailure (1) is removed. Its meaning split into
//     ExitDriftDetected (1, a finding) and ExitInternal (70, an
//     internal error).
//   - ExitUsage moved from 2 to 64 (BSD EX_USAGE).
//   - ExitConfig moved from 3 to 10.
//   - ExitFilesystem moved from 4 to 12.
//   - ExitValidation moved from 5 to 2 and reclassified from an
//     execution failure to a finding.
//   - ExitSecurity moved from 6 to 13.
//   - ExitConflict moved from 7 to 14.
//   - ExitSuccess (0) is unchanged.
//
// The constants' names are preserved where the meaning is
// preserved; the names are new where the meaning is new.
//
// # The WBS 11.1.1 → WBS 11.1.2 relationship
//
// WBS 11.1.2 is additive on top of WBS 11.1.1. It does not
// renumber, rename, or reclassify any WBS 11.1.1 code. It adds:
//
//   - One new code, ExitAmbiguity (4), in the findings range.
//
//   - Seven aliases (ExitUpdateConflict, ExitInternalError,
//     ExitTemplateNotFound, ExitTemplateIncompatible,
//     ExitRenderingFailed, ExitPathSafetyViolation,
//     ExitSecurityPolicyViolation) whose values equal their
//     canonical WBS 11.1.1 constants.
//
//   - Four severity classes (SeverityOK, SeverityWarn,
//     SeverityError, SeverityFatal) for the CI classification.
//
// A consumer that used only WBS 11.1.1 constants sees no change in
// the numbers. A consumer that adopts the WBS 11.1.2 vocabulary
// gains finer-grained names for the same codes. A consumer that
// branches on severity gains a machine-readable CI classification.
var expectedConstants = map[string]string{
	// --- WBS 11.1.1 — exit-code taxonomy ---------------------------

	// Range boundaries. Each boundary is a named constant so that
	// callers can write `code <= ExitRangeFindingsMax` rather than
	// a magic number.
	"ExitRangeFindingsMin":         "WBS 11.1.1, ADR-018 — findings range lower bound (1)",
	"ExitRangeFindingsMax":         "WBS 11.1.1, ADR-018 — findings range upper bound (9)",
	"ExitRangeExecutionMin":        "WBS 11.1.1, ADR-018 — execution-failure range lower bound (10)",
	"ExitRangeExecutionMax":        "WBS 11.1.1, ADR-018 — execution-failure range upper bound (19)",
	"ExitRangeReservedRegistryMin": "WBS 11.1.1, ADR-018 — reserved registry/network/enterprise range lower bound (20)",
	"ExitRangeReservedRegistryMax": "WBS 11.1.1, ADR-018 — reserved registry/network/enterprise range upper bound (29)",
	"ExitRangeReservedForgeMin":    "WBS 11.1.1, ADR-018 — reserved future Forge use range lower bound (30)",
	"ExitRangeReservedForgeMax":    "WBS 11.1.1, ADR-018 — reserved future Forge use range upper bound (63)",
	"ExitRangeReservedSysexitsMin": "WBS 11.1.1, ADR-018 — reserved sysexits sub-range lower bound (65)",
	"ExitRangeReservedSysexitsMax": "WBS 11.1.1, ADR-018 — reserved sysexits sub-range upper bound (69)",
	"ExitRangeInternalMin":         "WBS 11.1.1, ADR-018 — internal-error range lower bound (70)",
	"ExitRangeInternalMax":         "WBS 11.1.1, ADR-018 — internal-error range upper bound (78)",
	"ExitRangeReservedFutureMin":   "WBS 11.1.1, ADR-018 — reserved future use range lower bound (79)",
	"ExitRangeReservedFutureMax":   "WBS 11.1.1, ADR-018 — reserved future use range upper bound (127)",
	"ExitRangeSignalsMin":          "WBS 11.1.1, ADR-018 — signal-termination range lower bound (129)",
	"ExitRangeSignalsMax":          "WBS 11.1.1, ADR-018 — signal-termination range upper bound (255)",

	// Frozen singletons.
	"ExitSuccess":         "WBS 11.1.1, ADR-018 — success (0)",
	"ExitUsage":           "WBS 11.1.1, ADR-018 — usage error (64, BSD EX_USAGE)",
	"ExitInvalidArgument": "WBS 11.1.1, ADR-018 — invalid exit argument (128)",

	// Signal codes.
	"ExitSIGHUP":  "WBS 11.1.1, ADR-018 — SIGHUP termination (129)",
	"ExitSIGINT":  "WBS 11.1.1, ADR-018 — SIGINT termination (130)",
	"ExitSIGTERM": "WBS 11.1.1, ADR-018 — SIGTERM termination (143)",

	// Named codes in the findings range.
	"ExitDriftDetected": "WBS 11.1.1, ADR-018 — drift detected (1, finding)",
	"ExitValidation":    "WBS 11.1.1, ADR-018 — validation finding (2)",
	"ExitCheckFailed":   "WBS 11.1.1, ADR-018 — check assertion failed (3, finding)",

	// Named codes in the execution-failure range.
	"ExitConfig":     "WBS 11.1.1, ADR-018 — configuration error (10)",
	"ExitTemplate":   "WBS 11.1.1, ADR-018 — template error (11)",
	"ExitFilesystem": "WBS 11.1.1, ADR-018 — filesystem error (12)",
	"ExitSecurity":   "WBS 11.1.1, ADR-018 — security boundary violation (13)",
	"ExitConflict":   "WBS 11.1.1, ADR-018 — update conflict (14)",

	// Named code in the internal-error range.
	"ExitInternal": "WBS 11.1.1, ADR-018 — internal error (70, BSD EX_SOFTWARE)",

	// Stability classes. These are string-typed constants, not
	// integers. They are exported so that callers can compare a
	// range's Stability field to a named value rather than to a
	// string literal.
	"StabilityFrozen":   "WBS 11.1.1, ADR-018 — frozen range class",
	"StabilityAdditive": "WBS 11.1.1, ADR-018 — additive range class",
	"StabilityReserved": "WBS 11.1.1, ADR-018 — reserved range class",

	// --- WBS 11.1.2 — per-code enumeration and severity ------------
	//
	// The WBS 11.1.2 enumeration (ADR-019) fills in the triggers
	// that WBS 11.1.1 left unspecified and gives every code a
	// severity. Two kinds of constant are introduced:
	//
	//   1. Alias constants for the WBS 11.1.2 vocabulary. Where
	//      WBS 11.1.1 already names a trigger, the WBS 11.1.2 name
	//      is declared as `const X = Y` (an alias); where the
	//      WBS 11.1.2 name is finer-grained than the WBS 11.1.1
	//      name, the alias shares the WBS 11.1.1 code (the
	//      taxonomy does not have room for distinct values, and
	//      the stability policy forbids renumbering).
	//
	//   2. The Severity type's four constants. They are exported
	//      so that callers can compare a code's severity to a
	//      named value rather than to a string literal, and so
	//      that the YAML's `severity` field and the Go source
	//      agree on the vocabulary.
	//
	// ExitAmbiguity is the one genuinely new code: value 4, in
	// the findings range. The WBS 11.1.2 doc lists the ambiguity
	// trigger as a finding (the command ran correctly; the inputs
	// do not uniquely determine the foundation). The value 4 was
	// free in WBS 11.1.1.
	"ExitAmbiguity":               "WBS 11.1.2, ADR-019 — init ambiguity (4, finding)",
	"ExitUpdateConflict":          "WBS 11.1.2, ADR-019 — alias of ExitConflict (14)",
	"ExitInternalError":           "WBS 11.1.2, ADR-019 — alias of ExitInternal (70)",
	"ExitTemplateNotFound":        "WBS 11.1.2, ADR-019 — alias of ExitTemplate (11)",
	"ExitTemplateIncompatible":    "WBS 11.1.2, ADR-019 — alias of ExitTemplate (11)",
	"ExitRenderingFailed":         "WBS 11.1.2, ADR-019 — alias of ExitTemplate (11)",
	"ExitPathSafetyViolation":     "WBS 11.1.2, ADR-019 — alias of ExitFilesystem (12)",
	"ExitSecurityPolicyViolation": "WBS 11.1.2, ADR-019 — alias of ExitSecurity (13)",

	// Severity classes. String-typed constants; exported so that a
	// caller compares a code's severity to a named value.
	"SeverityOK":    "WBS 11.1.2, ADR-019 — success severity class",
	"SeverityWarn":  "WBS 11.1.2, ADR-019 — warning severity class (does not fail CI)",
	"SeverityError": "WBS 11.1.2, ADR-019 — error severity class (fails CI)",
	"SeverityFatal": "WBS 11.1.2, ADR-019 — fatal severity class (fails CI)",

	// --- WBS 5.1.1 — root identity strings ------------------------
	"RootName":      "WBS 5.1.1 — frozen CLI name (lowercase)",
	"RootUsage":     "WBS 5.1.1 — frozen usage line",
	"RootShortDesc": "WBS 5.1.1 — frozen short description (<= 80 chars)",
	"RootLongDesc":  "WBS 5.1.1 — frozen long description (raw string)",

	// --- WBS 5.3.1 — global flag names ----------------------------
	"FlagVerbose": "WBS 5.3.1 — name of the --verbose global flag",
	"FlagQuiet":   "WBS 5.3.1 — name of the --quiet global flag",
	"FlagConfig":  "WBS 5.3.1 — name of the --config global flag",

	// --- WBS 9.4.1 — --format flag --------------------------------
	"FlagFormat":     "WBS 9.4.1 — name of the --format global flag",
	"FlagFormatText": "WBS 9.4.1 — value \"text\" for the --format flag",
	"FlagFormatJSON": "WBS 9.4.1 — value \"json\" for the --format flag",

	// WBS 11.3.2 — output format values.
	//
	// The two values of the --format flag. They are exported so
	// that downstream consumers (and the tests in this package)
	// can reference the canonical values by name rather than by
	// string literal. The values are part of the CLI's public
	// surface: scripts and documentation use them.
	//
	// The constants were added alongside the JSON error
	// envelope (WBS 11.3.2). They are distinct from the
	// FlagFormat, FlagFormatText, and FlagFormatJSON names
	// already in the allowlist from WBS 9.4.1:
	//
	//   - FlagFormat / FlagFormatText / FlagFormatJSON are the
	//     *flag name* and the *string values* as the flag stores
	//     them. They are registered by registerGlobalFlags.
	//
	//   - FormatText / FormatJSON are the *typed* OutputFormat
	//     values used by the error path's dispatch. They are a
	//     distinct vocabulary; the two vocabularies coincide in
	//     value but not in type.
	"FormatText": "WBS 11.3.2 — typed text output format",
	"FormatJSON": "WBS 11.3.2 — typed JSON output format",
}

// TestOnlyExpectedConstantsAreExported verifies that the package
// exports exactly the constants that the CLI contract requires, and
// nothing more.
//
// The exported constants form a stable vocabulary that downstream
// code and tests may reference by name. The vocabulary has five
// parts: the exit-code taxonomy (WBS 11.1.1), the aliases and
// ambiguity code (WBS 11.1.2), the severity classes (WBS 11.1.2),
// the root identity strings, and the global flag names. All five
// are enumerated in the expectedConstants map above.
//
// Adding a new exported constant requires updating the map. The
// update is a review checkpoint: it forces the contributor to
// justify the addition.
func TestOnlyExpectedConstantsAreExported(t *testing.T) {
	t.Parallel()

	files := listGoFiles(t)
	actual := collectExportedConstants(t, files)

	// Every actual constant must be in the expected set.
	for _, name := range actual {
		if _, ok := expectedConstants[name]; !ok {
			t.Errorf("unexpected exported constant: %s\n"+
				"If this constant was intentionally added, update the "+
				"expectedConstants map in structure_test.go and cite "+
				"the WBS item or ADR that introduced it.", name)
		}
	}

	// Every expected constant must be present.
	for name, reason := range expectedConstants {
		found := false
		for _, actualName := range actual {
			if actualName == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected exported constant missing: %s (%s)",
				name, reason)
		}
	}
}

// collectExportedConstants returns the sorted list of exported
// top-level constant identifiers across the given files.
//
// Constants declared inside a `const (...)` block are visited
// individually. Constants declared as a single `const Name = value`
// line are also visited.
func collectExportedConstants(t *testing.T, files []string) []string {
	t.Helper()

	dir := packageDir(t)
	fset := token.NewFileSet()

	seen := make(map[string]bool)

	for _, file := range files {
		path := filepath.Join(dir, file)
		parsed, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}

		for _, decl := range parsed.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			if gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, name := range vs.Names {
					if name.IsExported() {
						seen[name.Name] = true
					}
				}
			}
		}
	}

	var out []string
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// =============================================================================
// Required content
// =============================================================================

// TestDocFileHasPackageDocumentation verifies that doc.go begins with
// a package comment.
//
// The package comment lives in doc.go, not in root.go. This is a Go
// convention: the package comment may live in any file, but placing
// it in a file named doc.go makes it discoverable.
func TestDocFileHasPackageDocumentation(t *testing.T) {
	t.Parallel()

	content := readFile(t, "doc.go")

	// The first non-empty, non-build-constraint line must be a comment
	// that begins with "Package cli".
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "// Package cli") {
			t.Fatalf("doc.go does not begin with a package comment; "+
				"first non-empty line was: %q", line)
		}
		return
	}
	t.Fatal("doc.go is empty")
}

// TestExecuteHasDocComment verifies that the Execute function is
// documented.
//
// Execute is the only exported function in the package. It defines
// the package's contract with cmd/forge/main.go. Its documentation is
// what future maintainers read to understand why the signature is
// frozen.
//
// Execute lives in execute.go, not root.go.
func TestExecuteHasDocComment(t *testing.T) {
	t.Parallel()

	content := readFile(t, "execute.go")

	// Find the line containing "func Execute" and verify that the
	// preceding non-empty line is a comment.
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if !strings.Contains(line, "func Execute(") {
			continue
		}
		// Walk backwards to find the nearest non-blank line.
		for j := i - 1; j >= 0; j-- {
			prev := strings.TrimSpace(lines[j])
			if prev == "" {
				continue
			}
			if !strings.HasPrefix(prev, "//") {
				t.Fatalf("Execute has no doc comment; "+
					"the line before 'func Execute' was: %q", prev)
			}
			return
		}
		t.Fatal("Execute has no preceding line")
	}
	t.Fatal("func Execute not found in execute.go")
}

// TestExitCodesHaveDocComment verifies that the exit code constants
// block is documented.
//
// The constants are part of the CLI's public vocabulary. Their
// documentation explains what each code means and why the values are
// stable. Its absence signals that the constants were added without
// consideration for their public role.
func TestExitCodesHaveDocComment(t *testing.T) {
	t.Parallel()

	content := readFile(t, "exitcodes.go")

	// The file must contain a comment block before the `const (` line.
	constDecl := "const ("
	idx := strings.Index(content, constDecl)
	if idx < 0 {
		t.Fatal("exitcodes.go does not contain a const declaration")
	}

	// Walk backwards from the const declaration to find the nearest
	// non-blank line. It must be a comment.
	lines := strings.Split(content[:idx], "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "//") {
			t.Fatalf("exit code constants have no preceding doc comment; "+
				"the line before 'const (' was: %q", line)
		}
		return
	}
	t.Fatal("exitcodes.go begins with the const declaration; no doc comment")
}

// =============================================================================
// Test file package declarations
// =============================================================================

// TestTestFilePackageDeclarations verifies that the package's test
// files are split between `package cli` (white-box) and
// `package cli_test` (black-box) according to the design.
//
// The split is deliberate:
//
//   - White-box tests (package cli) reach unexported symbols and are
//     used for behaviour and boundary testing.
//
//   - Black-box tests (package cli_test) read source files as data
//     and are used for structural invariant testing.
//
// The split is enforced so that a contributor who moves a test file
// to the wrong package is told immediately why the move is wrong.
func TestTestFilePackageDeclarations(t *testing.T) {
	t.Parallel()

	expected := map[string]string{
		// White-box: need unexported symbols.
		"root_test.go":             "package cli",
		"execute_test.go":          "package cli",
		"version_test.go":          "package cli",
		"deps_test.go":             "package cli",
		"registry_test.go":         "package cli",
		"contract_test.go":         "package cli",
		"validate_test.go":         "package cli",
		"flags_test.go":            "package cli",
		"exitcodes_test.go":        "package cli",
		"exitcodes_compat_test.go": "package cli",
		"mapping_test.go":          "package cli",

		// Black-box: read source files as data.
		"structure_test.go": "package cli_test",
	}

	for file, wantPkg := range expected {
		file := file
		wantPkg := wantPkg
		t.Run(file, func(t *testing.T) {
			t.Parallel()
			content := readFile(t, file)
			gotPkg := extractPackageDeclaration(content)
			if gotPkg != wantPkg {
				t.Errorf("package declaration mismatch\n"+
					"  file: %s\n"+
					"  want: %s\n"+
					"  got:  %s",
					file, wantPkg, gotPkg)
			}
		})
	}
}

// extractPackageDeclaration returns the package clause of a Go source
// file as a single-line string, e.g. "package cli".
func extractPackageDeclaration(content string) string {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if strings.HasPrefix(line, "package ") {
			return line
		}
	}
	return ""
}

// =============================================================================
// Import parsing helper
// =============================================================================

// importBlockPattern matches a Go import block:
//
//	import (
//	    "fmt"
//	    "os"
//	    "github.com/spf13/cobra"
//	)
var importBlockPattern = regexp.MustCompile(`(?s)import\s*\(([^)]*)\)`)

// singleImportPattern matches a single-line import:
//
//	import "fmt"
var singleImportPattern = regexp.MustCompile(`(?m)^import\s+"([^"]+)"`)

// parseImportPaths extracts the import paths from a Go source file's
// content.
//
// Both block imports (`import (...)`) and single-line imports
// (`import "path"`) are handled. The function returns the raw import
// paths without quotes.
func parseImportPaths(t *testing.T, content string) []string {
	t.Helper()

	var paths []string

	// Block imports.
	for _, match := range importBlockPattern.FindAllStringSubmatch(content, -1) {
		block := match[1]
		for _, line := range strings.Split(block, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "//") {
				continue
			}
			// A line may be: `"path"`, `alias "path"`, or `_ "path"`.
			// Extract the quoted path.
			start := strings.Index(line, `"`)
			end := strings.LastIndex(line, `"`)
			if start >= 0 && end > start {
				paths = append(paths, line[start+1:end])
			}
		}
	}

	// Single-line imports.
	for _, match := range singleImportPattern.FindAllStringSubmatch(content, -1) {
		if len(match) >= 2 {
			paths = append(paths, match[1])
		}
	}

	return paths
}

// =============================================================================
// Exit code invariants
// =============================================================================

// TestNoRawExitCodeLiterals verifies that no file in the package
// (other than exitcodes.go) contains a bare integer literal that
// matches an exit code in a `return` statement or an `os.Exit` call.
//
// The invariant is: exit codes are defined in exactly one file, and
// every other file refers to them by name. This makes the mapping
// auditable: a reader who wants to know what exit code a command
// returns can search for the constant's name rather than for the
// integer value.
//
// The test uses go/ast to inspect integer literals in the syntax
// tree. It ignores:
//
//   - exitcodes.go, where the constants are defined.
//   - test files, which may legitimately reference exit codes by
//     integer in table-driven tests.
//
// A literal is considered an exit code if its value is one of the
// known exit code values. The set is the WBS 11.1.1 + WBS 11.1.2
// taxonomy; see the `reserved` map below. The test is deliberately
// over-broad within the set: any integer in the set triggers the
// failure, even if the contributor intended it as something else.
// Renaming the intended constant clarifies the code.
//
// # Why only `return` and `os.Exit`
//
// A bare integer literal is legitimate in many contexts: slice
// indices, loop bounds, array sizes, arithmetic. The test therefore
// looks for two specific node shapes where an integer is almost
// certainly an exit code:
//
//	return <int-literal>
//	os.Exit(<int-literal>)
//
// A literal elsewhere is not flagged. This keeps the test's false-
// positive rate low without weakening its coverage of the intended
// shapes.
//
// # Why exitcodes.go is excluded
//
// The exit codes are defined in exitcodes.go. That file's constants
// are the source of truth; the test does not need to check that the
// source of truth agrees with itself. The test's purpose is to catch
// raw literals in *other* files, where the code should use a named
// constant.
//
// # The mapping's literals
//
// mapping.go (WBS 11.2.1) returns named constants (ExitSuccess,
// ExitSIGINT, ExitInternal, ExitUsage), not raw integers. The test
// therefore does not flag it. If a future change introduced a raw
// literal in mapping.go, the test would catch it; that is the
// intent.
func TestNoRawExitCodeLiterals(t *testing.T) {
	t.Parallel()

	dir := packageDir(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read package directory: %v", err)
	}

	// The set of integers that are reserved as exit codes.
	//
	// The values are the WBS 11.1.1 + WBS 11.1.2 codes, not the
	// WBS 4.1.2 flat values. A raw integer in a return statement
	// or an os.Exit call that matches one of these is a bug: the
	// code should reference the named constant.
	//
	// The map is deliberately over-broad within each range: it
	// covers every assigned code, not just the ones currently
	// referenced. A new code added to the taxonomy should be added
	// here in the same commit, so that the test's coverage grows
	// with the registry.
	//
	// ExitAmbiguity (4) was added by WBS 11.1.2. The other codes
	// were introduced by WBS 11.1.1; the WBS 11.1.2 aliases share
	// their canonical constants' values, so no additional entries
	// are needed for the aliases.
	reserved := map[int64]string{
		// Success.
		0: "ExitSuccess",

		// Findings range.
		1: "ExitDriftDetected",
		2: "ExitValidation",
		3: "ExitCheckFailed",
		4: "ExitAmbiguity",

		// Execution-failure range.
		10: "ExitConfig",
		11: "ExitTemplate",
		12: "ExitFilesystem",
		13: "ExitSecurity",
		14: "ExitConflict",

		// Usage singleton.
		64: "ExitUsage",

		// Internal-error range.
		70: "ExitInternal",

		// Signal singletons.
		129: "ExitSIGHUP",
		130: "ExitSIGINT",
		143: "ExitSIGTERM",
	}

	fset := token.NewFileSet()

	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		if name == "exitcodes.go" {
			continue
		}

		path := filepath.Join(dir, name)
		parsed, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", name, parseErr)
		}

		ast.Inspect(parsed, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.ReturnStmt:
				// A return statement with exactly one result that is an
				// integer literal.
				if len(node.Results) != 1 {
					return true
				}
				lit, ok := node.Results[0].(*ast.BasicLit)
				if !ok || lit.Kind != token.INT {
					return true
				}
				value, convErr := strconv.ParseInt(lit.Value, 0, 64)
				if convErr != nil {
					return true
				}
				if constName, ok := reserved[value]; ok {
					pos := fset.Position(lit.Pos())
					t.Errorf("%s:%d:%d: raw exit code literal in return "+
						"statement: %d\n"+
						"  Use the named constant %s instead.\n"+
						"  Exit codes are defined only in exitcodes.go.",
						name, pos.Line, pos.Column, value, constName)
				}

			case *ast.CallExpr:
				// A call to os.Exit with an integer literal argument.
				sel, ok := node.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "Exit" {
					return true
				}
				ident, ok := sel.X.(*ast.Ident)
				if !ok || ident.Name != "os" {
					return true
				}
				if len(node.Args) != 1 {
					return true
				}
				lit, ok := node.Args[0].(*ast.BasicLit)
				if !ok || lit.Kind != token.INT {
					return true
				}
				value, convErr := strconv.ParseInt(lit.Value, 0, 64)
				if convErr != nil {
					return true
				}
				if constName, ok := reserved[value]; ok {
					pos := fset.Position(lit.Pos())
					t.Errorf("%s:%d:%d: raw exit code literal in os.Exit "+
						"call: %d\n"+
						"  Use the named constant %s instead.\n"+
						"  Exit codes are defined only in exitcodes.go.",
						name, pos.Line, pos.Column, value, constName)
				}
			}

			return true
		})
	}
}
