// Package main_test verifies the static and structural invariants of
// the forge entry point.
//
// These are not behavioural tests of the binary. They are structural
// tests of the source code, ensuring the entry point remains minimal
// and correctly wired as required by WBS 2.2.1.
//
// Behavioural tests of the binary live in binary_integration_test.go
// and are gated behind the `integration` build tag.
package main_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mainGoPath is the path to the entry point file, relative to this test.
const mainGoPath = "main.go"

// =============================================================================
// File layout
// =============================================================================

// TestMainGo_Exists verifies AC1: the file exists at exactly
// cmd/forge/main.go. The test's own location (cmd/forge/) enforces the
// directory; this test enforces the filename.
func TestMainGo_Exists(t *testing.T) {
	t.Parallel()

	if _, err := os.Stat(mainGoPath); err != nil {
		t.Fatalf("AC1 FAIL: %s not found: %v", mainGoPath, err)
	}
}

// TestMainGoPath verifies AC1 alongside the directory itself. It is
// redundant with the file's own location but exists for symmetry with
// the acceptance criteria table.
func TestMainGoPath(t *testing.T) {
	t.Parallel()

	abs, err := filepath.Abs(mainGoPath)
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	if !strings.HasSuffix(abs, filepath.Join("cmd", "forge", "main.go")) {
		t.Fatalf("main.go is not at cmd/forge/main.go: %s", abs)
	}
}

// TestDocGoExists verifies that doc.go exists alongside main.go. It is
// part of the invariant contract: package documentation lives in a
// dedicated file to keep main.go under the line budget.
func TestDocGoExists(t *testing.T) {
	t.Parallel()

	if _, err := os.Stat("doc.go"); err != nil {
		t.Fatalf("doc.go not found alongside main.go: %v", err)
	}
}

// TestNoOtherGoFilesInDir verifies that cmd/forge/ contains only the
// expected Go source files.
//
// Source files (non-test) are tightly constrained: cmd/forge/ must
// contain exactly two of them, main.go and doc.go. Any additional
// source file is a violation of the architectural invariant that all
// logic lives in internal/cli.
//
// Test files (suffix _test.go) are more permissive. New test files may
// be added whenever a new aspect of the package's behaviour needs to
// be exercised. Every test file must be added to the allowedTests map
// in the same change that introduces it.
func TestNoOtherGoFilesInDir(t *testing.T) {
	t.Parallel()

	// Non-test source files allowed in cmd/forge/, with the WBS item
	// that introduced each. Do not add entries here without an ADR.
	allowedSource := map[string]string{
		"main.go": "WBS 2.2.1 — process entry point",
		"doc.go":  "WBS 2.2.1 — package documentation",
	}

	// Test files allowed in cmd/forge/, with the WBS item that
	// introduced each. New test files are permitted provided they are
	// added to this map in the same change.
	allowedTests := map[string]string{
		"main_test.go":               "WBS 2.2.1 — structural invariant tests",
		"wiring_test.go":             "WBS 2.2.1 — negative wiring test",
		"binary_integration_test.go": "WBS 2.2.1 — process boundary integration tests",
	}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") {
			continue
		}

		if strings.HasSuffix(name, "_test.go") {
			if _, ok := allowedTests[name]; !ok {
				t.Errorf("unexpected test file in cmd/forge/: %s\n"+
					"Add this file to the allowedTests map in "+
					"TestNoOtherGoFilesInDir and cite the WBS item that "+
					"introduced it.", name)
			}
			continue
		}

		if _, ok := allowedSource[name]; !ok {
			t.Errorf("unexpected source file in cmd/forge/: %s\n"+
				"cmd/forge/ must contain exactly two non-test source "+
				"files: main.go and doc.go. All logic belongs in "+
				"internal/cli. Adding a new source file here requires "+
				"an ADR justifying the exception to the package's "+
				"entry-point-only invariant.", name)
		}
	}
}

// =============================================================================
// Structural invariants
// =============================================================================

// TestMainGo_ContainsExactlyOneFunction verifies AC2: the file contains
// exactly one function declaration named "main".
func TestMainGo_ContainsExactlyOneFunction(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, mainGoPath, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse %s: %v", mainGoPath, err)
	}

	var funcs []string
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			funcs = append(funcs, fn.Name.Name)
		}
	}

	if len(funcs) != 1 {
		t.Fatalf("AC2 FAIL: expected exactly 1 function, found %d: %v",
			len(funcs), funcs)
	}
	if funcs[0] != "main" {
		t.Fatalf("AC2 FAIL: expected function 'main', found %q", funcs[0])
	}
}

// TestMainGo_ContainsExactlyOneOsExit verifies AC3: the body of main
// contains exactly one call to os.Exit.
func TestMainGo_ContainsExactlyOneOsExit(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, mainGoPath, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", mainGoPath, err)
	}

	var osExitCalls int
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if sel.Sel.Name != "Exit" {
			return true
		}
		ident, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		if ident.Name == "os" {
			osExitCalls++
		}
		return true
	})

	if osExitCalls != 1 {
		t.Fatalf("AC3 FAIL: expected exactly 1 call to os.Exit, found %d",
			osExitCalls)
	}
}

// TestMainGo_ImportsAtMostTwoPackages verifies AC4: the file imports
// exactly "os" and the internal/cli package, and nothing else.
func TestMainGo_ImportsAtMostTwoPackages(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, mainGoPath, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse %s: %v", mainGoPath, err)
	}

	const (
		expectedOs  = `"os"`
		expectedCli = `"github.com/thapelomagqazana/forge/internal/cli"`
	)

	allowed := map[string]bool{
		expectedOs:  true,
		expectedCli: true,
	}

	for _, imp := range file.Imports {
		path := imp.Path.Value
		if !allowed[path] {
			t.Errorf("AC4 FAIL: forbidden import %s", path)
		}
	}

	if len(file.Imports) > 2 {
		t.Fatalf("AC4 FAIL: expected at most 2 imports, found %d",
			len(file.Imports))
	}
}

// TestMainGo_LineBudget verifies AC5: the total line count of main.go
// is within budget. The budget is 20 lines of *code*; comments and
// blank lines are excluded from the count.
func TestMainGo_LineBudget(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(mainGoPath)
	if err != nil {
		t.Fatalf("read %s: %v", mainGoPath, err)
	}

	lines := strings.Split(string(data), "\n")
	var codeLines int
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		codeLines++
	}

	// The canonical implementation has around 6 lines of code:
	//   1: package main
	//   2: import (
	//   3: "os"
	//   4: internal/cli import
	//   5: )
	//   6: func main() { os.Exit(cli.Execute()) }
	//
	// A budget of 20 gives headroom without permitting logic.
	const budget = 20
	if codeLines > budget {
		t.Fatalf("AC5 FAIL: %d lines of code exceed budget %d",
			codeLines, budget)
	}
}

// TestMainGo_NoLogicOutsideMain verifies a structural invariant: no
// variables, constants, types, or additional functions exist at
// package level beyond the permitted minimum.
func TestMainGo_NoLogicOutsideMain(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, mainGoPath, nil,
		parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", mainGoPath, err)
	}

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Name.Name != "main" {
				t.Errorf("found unexpected function %q", d.Name.Name)
			}
		case *ast.GenDecl:
			// Allow only imports. GenDecl covers var, const, type,
			// import.
			if d.Tok != token.IMPORT {
				t.Errorf("found unexpected %s declaration at package level",
					d.Tok)
			}
		}
	}
}

// TestMainGoSyntaxIsValid is a defensive check: even if all other
// tests pass, main.go must parse as valid Go.
func TestMainGoSyntaxIsValid(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	_, err := parser.ParseFile(fset, mainGoPath, nil, parser.AllErrors)
	if err != nil {
		t.Fatalf("main.go is not valid Go: %v", err)
	}
}

// =============================================================================
// Forbidden constructs (AST-based)
// =============================================================================

// TestMainGoNoForbiddenPatterns is a defence-in-depth check. It parses
// main.go with go/ast and asserts that specific syntactic constructs do
// not appear in the executable code.
//
// The AST-based approach is used in preference to regex matching
// because regex matching produces false positives on comments and
// string literals. The package documentation of main.go contains the
// word "defer" in prose; a regex would flag it, but an AST walk
// correctly ignores it.
//
// Each check below looks for a specific syntax tree node type. Adding a
// new forbidden pattern is a matter of adding one entry to the checks
// slice.
func TestMainGoNoForbiddenPatterns(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, mainGoPath, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse %s: %v", mainGoPath, err)
	}

	// astCheck describes a forbidden construct.
	//
	// The name is used in error messages. The match function receives
	// a syntax tree node and returns true if the node constitutes a
	// violation.
	type astCheck struct {
		name  string
		match func(ast.Node) bool
	}

	checks := []astCheck{
		{
			name: "defer statement",
			match: func(n ast.Node) bool {
				_, ok := n.(*ast.DeferStmt)
				return ok
			},
		},
		{
			name: "go statement (goroutine)",
			match: func(n ast.Node) bool {
				_, ok := n.(*ast.GoStmt)
				return ok
			},
		},
		{
			name: "call to fmt.Print, fmt.Println, or fmt.Printf",
			match: func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return false
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return false
				}
				ident, ok := sel.X.(*ast.Ident)
				if !ok {
					return false
				}
				if ident.Name != "fmt" {
					return false
				}
				switch sel.Sel.Name {
				case "Print", "Println", "Printf":
					return true
				}
				return false
			},
		},
		{
			name: "call to os.Getenv, os.Setenv, os.LookupEnv, or os.Environ",
			match: func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return false
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return false
				}
				ident, ok := sel.X.(*ast.Ident)
				if !ok {
					return false
				}
				if ident.Name != "os" {
					return false
				}
				switch sel.Sel.Name {
				case "Getenv", "Setenv", "LookupEnv", "Environ":
					return true
				}
				return false
			},
		},
		{
			name: "call to recover()",
			match: func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return false
				}
				ident, ok := call.Fun.(*ast.Ident)
				if !ok {
					return false
				}
				return ident.Name == "recover"
			},
		},
	}

	for _, check := range checks {
		check := check // capture loop variable
		found := false

		ast.Inspect(file, func(n ast.Node) bool {
			if n == nil || found {
				return false
			}
			if check.match(n) {
				found = true
				pos := fset.Position(n.Pos())
				t.Errorf("forbidden construct %q detected in %s at %s",
					check.name, mainGoPath, pos)
				return false
			}
			return true
		})
	}
}

// =============================================================================
// Documentation invariants
// =============================================================================

// TestMainGo_DocCommentExists verifies that main.go includes the package
// documentation comment describing the invariant.
func TestMainGo_DocCommentExists(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(mainGoPath)
	if err != nil {
		t.Fatalf("read %s: %v", mainGoPath, err)
	}

	content := string(data)
	requiredPhrases := []string{
		"Command forge is the Forge CLI",
		"This file must remain minimal",
		"All logic belongs in internal/cli",
	}

	for _, phrase := range requiredPhrases {
		if !strings.Contains(content, phrase) {
			t.Errorf("doc comment missing required phrase: %q", phrase)
		}
	}
}

// TestReadmeExists verifies the invariant documentation file exists.
// The README is what makes the invariant discoverable to contributors
// who have never read the WBS.
func TestReadmeExists(t *testing.T) {
	t.Parallel()

	if _, err := os.Stat("README.md"); err != nil {
		t.Fatalf("README.md not found in cmd/forge/: %v", err)
	}
}