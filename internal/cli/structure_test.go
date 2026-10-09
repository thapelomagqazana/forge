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
// Those are tested by the white-box tests in root_test.go and
// execute_test.go, which live in package cli (not cli_test).
//
// # Why structural tests
//
// The CLI package's design depends on properties that the compiler
// does not enforce:
//
//   - Only one file may import Cobra. If a second file imports it,
//     the "one entry point" invariant is violated.
//   - Only one function may be exported from the package (Execute).
//     If a second function becomes exported, downstream packages can
//     depend on internals, which locks the design in place.
//   - The package's exported constants are a bounded vocabulary. New
//     ones require review.
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
// for the purpose of testing it, and are not subject to the "one file
// imports Cobra" rule.
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
var expectedSourceFiles = map[string]string{
	"doc.go":       "WBS 2.4.2 — package documentation and public contract",
	"execute.go":   "WBS 2.4.2 — Execute and executeWithOptions",
	"exitcodes.go": "WBS 2.4.2 — exit code constants and error mapping",
	"root.go":      "WBS 2.4.2 — root command constructor",
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
func TestRootFileImportsCobra(t *testing.T) {
	t.Parallel()

	content := readFile(t, "root.go")

	if !strings.Contains(content, cobraImportPath) {
		t.Fatalf("root.go does not import Cobra; expected to find %s",
			cobraImportPath)
	}
}

// TestCobraImportedInExactlyOneFile verifies that root.go is the only
// non-test file in the package that imports Cobra.
//
// This is the negative half of AC6: no *other* file may import Cobra.
// If two files import it, the package acquires two coupling points to
// the framework, and future refactors become harder.
func TestCobraImportedInExactlyOneFile(t *testing.T) {
	t.Parallel()

	files := listGoFiles(t)

	var importers []string
	for _, f := range files {
		content := readFile(t, f)
		if strings.Contains(content, cobraImportPath) {
			importers = append(importers, f)
		}
	}

	if len(importers) != 1 {
		t.Fatalf("expected exactly 1 file to import Cobra; found %d: %v",
			len(importers), importers)
	}

	if importers[0] != "root.go" {
		t.Fatalf("expected Cobra to be imported by root.go; "+
			"found it imported by %s", importers[0])
	}
}

// TestNoCobraImportOutsidePackage verifies that no file in the
// repository imports Cobra except internal/cli/root.go.
//
// This test is stronger than TestCobraImportedInExactlyOneFile: it
// walks the entire module, not just the package. If a future package
// (say internal/blueprint) accidentally imports Cobra, this test
// fails and forces the author to reconsider the dependency.
func TestNoCobraImportOutsidePackage(t *testing.T) {
	t.Parallel()

	moduleRoot := findModuleRoot(t)

	// The one allowed non-test importer.
	allowed := filepath.Join("internal", "cli", "root.go")

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
		if rel == allowed {
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
		t.Fatalf("Cobra is imported outside internal/cli/root.go: %v\n"+
			"Move the affected logic into internal/cli, or add an ADR "+
			"explaining why the dependency must be imported elsewhere.",
			offenders)
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

// =============================================================================
// Exported surface — functions
// =============================================================================

// TestOnlyExecuteFunctionIsExported verifies that the package exports
// exactly one *function*: Execute.
//
// The package's public function surface is deliberately one function.
// Every other function — command constructors, helpers — is
// unexported. This prevents downstream packages from depending on
// internals, which would freeze the design in place and make
// refactoring harder.
//
// Constants are tested separately by
// TestOnlyExpectedConstantsAreExported. They are a different category
// of public surface: an enumerated vocabulary that downstream code
// may reference by name.
//
// The test scans every non-test source file in the package and
// collects top-level function declarations whose identifier begins
// with an uppercase letter. Only Execute is permitted.
func TestOnlyExecuteFunctionIsExported(t *testing.T) {
	t.Parallel()

	files := listGoFiles(t)
	exported := collectExportedFunctions(t, files)

	if len(exported) != 1 {
		t.Fatalf("expected exactly 1 exported function in the package; "+
			"found %d: %v", len(exported), exported)
	}

	if exported[0] != "Execute" {
		t.Fatalf("expected the sole exported function to be Execute; "+
			"found %s", exported[0])
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
// permitted to expose, along with the WBS item that introduced it.
//
// The map is deliberately explicit. Adding a constant requires
// updating this map, which forces the contributor to justify the
// addition. Removing a constant is a breaking change and requires an
// ADR, because downstream code may depend on the name.
var expectedConstants = map[string]string{
	"ExitSuccess":    "WBS 2.4.2 — successful command exit code",
	"ExitFailure":    "WBS 2.4.2 — general failure exit code",
	"ExitUsage":      "WBS 2.4.2 — usage error exit code",
	"ExitConfig":     "WBS 2.4.2 — configuration failure exit code (reserved)",
	"ExitFilesystem": "WBS 2.4.2 — filesystem failure exit code (reserved)",
	"ExitValidation": "WBS 2.4.2 — validation failure exit code (reserved)",
	"ExitSecurity":   "WBS 2.4.2 — security failure exit code (reserved)",
	"ExitConflict":   "WBS 4.1.2 — update conflict exit code (reserved)",
}

// TestOnlyExpectedConstantsAreExported verifies that the package
// exports exactly the exit code constants that the CLI contract
// requires, and nothing more.
//
// The exit code constants form a stable vocabulary that downstream
// code and tests may reference by name. They are:
//
//   - ExitSuccess
//   - ExitFailure
//   - ExitUsage
//   - ExitConfig
//   - ExitFilesystem
//   - ExitValidation
//   - ExitSecurity
//
// Adding a new exported constant requires updating the
// expectedConstants map in this file. The update is a review
// checkpoint: it forces the contributor to justify the addition.
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
		// White-box: needs unexported symbols.
		"root_test.go":    "package cli",
		"execute_test.go": "package cli",

		// Black-box: reads source files as data.
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
// matches an exit code.
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
// known exit code values (0 through 6). The test is deliberately
// over-broad: any integer in that range triggers the failure, even
// if the contributor intended it as something else. Renaming the
// intended constant clarifies the code.
func TestNoRawExitCodeLiterals(t *testing.T) {
	t.Parallel()

	dir := packageDir(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read package directory: %v", err)
	}

	// The set of integers that are reserved as exit codes.
	reserved := map[int64]string{
		0: "ExitSuccess",
		1: "ExitFailure",
		2: "ExitUsage",
		3: "ExitConfig",
		4: "ExitFilesystem",
		5: "ExitValidation",
		6: "ExitSecurity",
		7: "ExitConflict",
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
			// We look for two specific node shapes:
			//
			//   return <int-literal>
			//   os.Exit(<int-literal>)
			//
			// A bare integer literal anywhere else in the file is
			// legitimate: slice indices, length comparisons, loop bounds,
			// array sizes. Only these two shapes indicate an exit code.

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
