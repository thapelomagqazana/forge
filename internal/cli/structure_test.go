// Package cli_test contains structural tests for the internal/cli package.
//
// These tests verify the *shape* of the package rather than its runtime
// behaviour. They are architectural tests: they enforce constraints that
// are invisible to the Go compiler but essential to the package's design.
//
// # What this file tests
//
//   - File layout: which files exist, and which must not exist.
//   - Import discipline: which packages may be imported where.
//   - Exported surface: how many symbols are exported.
//   - Required metadata: documentation comments and invariants.
//   - Package boundaries: no leakage into other packages.
//
// # What this file does not test
//
//   - Runtime behaviour of any command.
//   - Command output formatting.
//   - Exit codes.
//   - Error formatting.
//
// Those are tested by runtime test files, which are added in WBS 4.x
// onwards. See the package documentation for the full test strategy.
//
// # Why structural tests
//
// The CLI package's design depends on properties that the compiler does
// not enforce:
//
//   - Only one file may import Cobra. If a second file imports it, the
//     "one entry point" invariant is violated.
//   - Only one function may be exported from the package (Execute).
//     If a second symbol becomes exported, downstream packages can depend
//     on internals, which locks the design in place.
//   - The package must contain no unused files. Orphaned source files
//     accumulate over time and create confusion.
//
// Without structural tests, these constraints decay silently. With them,
// any violation fails CI immediately and forces a deliberate decision.
package cli_test

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// =============================================================================
// Test helpers
// =============================================================================

// packageDir returns the absolute path to the internal/cli directory.
//
// It is derived from the location of this test file, using runtime.Caller
// to find the source file path, then walking up to the directory
// containing it.
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

// listGoFiles returns the sorted list of *.go files in the package
// directory, excluding test files and the file that contains this
// helper.
//
// The exclusion of *_test.go files is deliberate: test files may
// legitimately import Cobra for the purpose of testing it, and are not
// subject to the "one file imports Cobra" rule.
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

// =============================================================================
// File layout
// =============================================================================

// TestRootFileExists verifies that root.go exists in the package.
//
// root.go is the anchor file: it contains the Execute entry point and
// the root command constructor. Without it, the package cannot be
// imported.
func TestRootFileExists(t *testing.T) {
	t.Parallel()

	files := listGoFiles(t)

	found := false
	for _, f := range files {
		if f == "root.go" {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("root.go not found in package; files present: %v", files)
	}
}

// TestNoOrphanedFiles verifies that every Go file in the package is
// one of the files this WBS explicitly expects.
//
// The list is deliberately explicit rather than glob-based. When a new
// file is added in a future WBS, this test must be updated as part of
// that WBS. The update is a review checkpoint: it forces the author to
// justify the new file.
func TestNoOrphanedFiles(t *testing.T) {
	t.Parallel()

	// Files expected in the package after WBS 2.4.1.
	//
	// This list grows as subsequent WBS items are completed. Every
	// addition requires a corresponding WBS reference in the comment.
	expected := map[string]string{
		"root.go": "WBS 2.4.1 — root command and Execute entry point",
	}

	files := listGoFiles(t)

	for _, f := range files {
		if _, ok := expected[f]; !ok {
			t.Errorf("unexpected Go file in package: %s\n"+
				"If this file was intentionally added, update the "+
				"expected list in TestNoOrphanedFiles and cite the WBS "+
				"item that introduced it.", f)
		}
	}

	for f, reason := range expected {
		found := false
		for _, actual := range files {
			if actual == f {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected file missing: %s (required by %s)", f, reason)
		}
	}
}

// =============================================================================
// Import discipline
// =============================================================================

// cobraImportPath is the canonical import path for Cobra. Used by
// multiple tests below.
const cobraImportPath = `"github.com/spf13/cobra"`

// TestRootFileImportsCobra verifies that root.go imports Cobra. This is
// the positive half of the AC6 requirement from WBS 2.4.1: Cobra must
// be wired in.
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

// =============================================================================
// Exported surface
// =============================================================================

// TestOnlyExecuteIsExported verifies that root.go exports exactly one
// symbol: the Execute function.
//
// The package's public surface is deliberately one function. Every
// other symbol — command constructors, helpers, types — is unexported.
// This prevents downstream packages from depending on internals, which
// would freeze the design in place and make refactoring harder.
//
// The test scans the file for top-level declarations whose identifier
// begins with an uppercase letter. The scan is line-based rather than
// AST-based: for the small, well-formed source files in this package,
// a regex-based scan is sufficient and much simpler to read.
func TestOnlyExecuteIsExported(t *testing.T) {
	t.Parallel()

	content := readFile(t, "root.go")

	// Match top-level declarations of the form:
	//
	//   func Name(...)         — function
	//   func (recv Type) Name  — method (receiver's methods are attached
	//                            to types, not to the package)
	//   var Name ...           — variable
	//   const Name ...         — constant
	//   type Name ...          — type
	//
	// The regex anchors on the start of a line (no leading whitespace),
	// which is where top-level declarations occur in gofmt-formatted
	// source.
	declPattern := regexp.MustCompile(
		`^(?:func|var|const|type)\s+(?:\([^)]*\)\s+)?([A-Z][A-Za-z0-9_]*)`,
	)

	var exported []string
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		matches := declPattern.FindStringSubmatch(line)
		if len(matches) >= 2 {
			exported = append(exported, matches[1])
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan root.go: %v", err)
	}

	// The package may contain additional files in future WBS items;
	// this test is scoped to root.go specifically. When a new file is
	// added, a similar test should be added for it if the file is
	// expected to have a bounded public surface. Most files will
	// export nothing at all.
	if len(exported) != 1 {
		t.Fatalf("expected exactly 1 exported symbol in root.go; found %d: %v",
			len(exported), exported)
	}

	if exported[0] != "Execute" {
		t.Fatalf("expected the sole exported symbol to be Execute; found %s",
			exported[0])
	}
}

// =============================================================================
// Required content
// =============================================================================

// TestRootFileHasPackageDocumentation verifies that root.go begins
// with a package comment.
//
// The package comment is the first thing a reader sees when opening
// the file. It documents the package's public contract and the
// invariants that contributors must preserve. Its absence signals that
// the package's design was not thought through.
func TestRootFileHasPackageDocumentation(t *testing.T) {
	t.Parallel()

	content := readFile(t, "root.go")

	// The first non-empty, non-build-constraint line must be a comment
	// that begins with "Package cli".
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "// Package cli") {
			t.Fatalf("root.go does not begin with a package comment; "+
				"first non-empty line was: %q", line)
		}
		break
	}
}

// TestExecuteHasDocComment verifies that the Execute function is
// documented.
//
// Execute is the only exported symbol in the package. It defines the
// package's contract with cmd/forge/main.go. Its documentation is what
// future maintainers read to understand why the signature is frozen.
func TestExecuteHasDocComment(t *testing.T) {
	t.Parallel()

	content := readFile(t, "root.go")

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
	t.Fatal("func Execute not found in root.go")
}

// =============================================================================
// Module integration
// =============================================================================

// findModuleRoot walks up from the package directory until it finds
// go.mod, and returns the directory containing it.
//
// The helper fails the test if no go.mod is found, which would mean
// the test is running outside the Forge module.
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
// Import parsing helper
// =============================================================================

// importBlockPattern matches a Go import block:
//
//	import (
//	    "fmt"
//	    "os"
//	    "github.com/spf13/cobra"
//	)
//
// The capture group includes everything between the parentheses.
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