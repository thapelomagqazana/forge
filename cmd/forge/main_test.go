// Package main_test verifies the static and structural invariants of
// the forge entry point.
//
// These are not behavioural tests of the binary. They are structural
// tests of the source code, ensuring the entry point remains minimal
// and correctly wired as required by WBS 2.2.1 and extended by WBS
// 11.3.1.
//
// Behavioural tests of the binary live in binary_integration_test.go
// and are gated behind the `integration` build tag.
//
// # The two WBS items
//
//   - WBS 2.2.1 froze the entry point's minimality: one function,
//     one os.Exit, two imports, no logic.
//
//   - WBS 11.3.1 (ADR-020) added signal handling. The entry point
//     installs a signal-aware context and passes it to cli.Run.
//     The addition is documented in the file's own docstring and
//     in docs/architecture/exit-boundary.md.
//
// The tests below enforce both invariants. The three tests that
// needed updating for WBS 11.3.1 are:
//
//   - TestMainGo_ImportsAtMostTwoPackages, which now permits four
//     imports (os, os/signal, syscall, internal/cli).
//
//   - TestMainGoNoForbiddenPatterns, whose "defer statement" check
//     is relaxed to permit the one deferred stop() call that
//     unregisters the signal handler.
//
//   - TestMainGo_ContainsExactlyOneDeferredCall, which is a new
//     test that pins the deferred call's shape so that a future
//     change cannot add a second defer without an explicit edit to
//     this test.
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

		// WBS 12.4.1 — platform-specific signal initialization.
		//
		// The two files provide the same function
		// (installPlatformSignalHandlers) with different build
		// tags. The function installs the process's platform-
		// specific signal dispositions; on Unix it installs
		// SIG_IGN for SIGPIPE, and on every other platform it is
		// a no-op.
		//
		// The files exist because the direct call to
		// signal.Ignore(syscall.SIGPIPE) in main.go would fail
		// to compile on platforms where syscall.SIGPIPE is not
		// declared (for example, js/wasm). The split confines
		// the platform-specific symbol to the file that is
		// compiled on the platforms where it exists.
		//
		// The files are the minimum shape of the platform-tagged
		// code pattern: one file per platform class, each a few
		// lines, each providing a function the platform-agnostic
		// caller invokes. They do not contain logic in the
		// architectural sense — no parsing, no I/O, no formatting
		// — and they do not violate the entry-point minimality
		// invariant's intent: main.go remains the only file with
		// the process's logic, and the signal-init files
		// install a disposition whose lifetime must match the
		// process's.
		"signal_init_unix.go":  "WBS 12.4.1 — Unix SIGPIPE disposition",
		"signal_init_other.go": "WBS 12.4.1 — non-Unix signal disposition (no-op)",
	}

	// Test files allowed in cmd/forge/, with the WBS item that
	// introduced each. New test files are permitted provided they are
	// added to this map in the same change.
	allowedTests := map[string]string{
		"main_test.go":               "WBS 2.2.1 — structural invariant tests",
		"wiring_test.go":             "WBS 2.2.1 — negative wiring test",
		"binary_integration_test.go": "WBS 2.2.1 — process boundary integration tests",

		// WBS 12.4.1 — the broken-pipe tests. The three files
		// are split by role and by build tag:
		//
		//   - brokenpipe_testhelpers_test.go holds the two
		//     helpers (buildForge, processExitCode) that both
		//     platform tests use. The file has no build tag, so
		//     both platform tests see the same helpers.
		//   - brokenpipe_unix_test.go exercises the contract on
		//     Unix.
		//   - brokenpipe_windows_test.go exercises the contract
		//     on Windows.
		//
		// The three files are one logical test suite; the split
		// is forced by Go's build-tag mechanism, not by a desire
		// for separate concerns.
		"brokenpipe_testhelpers_test.go": "WBS 12.4.1 — shared broken-pipe test helpers",
		"brokenpipe_unix_test.go":        "WBS 12.4.1 — Unix broken-pipe integration test",
		"brokenpipe_windows_test.go":     "WBS 12.4.1 — Windows broken-pipe integration test",
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

// TestMainGo_ImportsTheApprovedPackages verifies AC4 (as extended by
// WBS 11.3.1): the file imports exactly the four packages the entry
// point's contract permits.
//
// # The approved imports
//
//   - "context":            for context.Background(), the parent of
//     the signal-aware context.
//   - "os":                 for os.Exit and os.Interrupt.
//   - "os/signal":          for signal.NotifyContext.
//   - "syscall":            for syscall.SIGTERM.
//   - internal/cli:         for cli.Run.
//
// That is five entries (four standard-library packages plus the
// internal one), but the test's name reads "ApprovedPackages"
// rather than "AtMostN" because the number grew: WBS 2.2.1's
// "at most two" is superseded by WBS 11.3.1's set of five. The
// test asserts set membership, not count; a future WBS that adds
// a sixth package updates this test in the same commit.
//
// # Why a set, not a count
//
// The count-based assertion ("at most two") was a useful proxy for
// the invariant when the set was small and the invariant was
// "minimality". The set-based assertion ("exactly these") is a
// stronger statement: it names the packages the entry point may
// import, so a future contributor who adds `fmt` is rejected at
// the membership check, not just at the count. The set-based form
// is the one the invariant deserves.
func TestMainGo_ImportsTheApprovedPackages(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, mainGoPath, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse %s: %v", mainGoPath, err)
	}

	allowed := map[string]string{
		`"context"`:   "WBS 11.3.1 — context.Background for the signal-aware context",
		`"os"`:        "WBS 2.2.1 — os.Exit; WBS 11.3.1 — os.Interrupt",
		`"os/signal"`: "WBS 11.3.1 — signal.NotifyContext for SIGINT/SIGTERM",
		`"syscall"`:   "WBS 11.3.1 — syscall.SIGTERM",
		`"github.com/thapelomagqazana/forge/internal/cli"`: "WBS 2.2.1 — cli.Run",
	}

	// Every import must be in the allowlist.
	for _, imp := range file.Imports {
		path := imp.Path.Value
		if _, ok := allowed[path]; !ok {
			t.Errorf("forbidden import %s\n"+
				"The entry point may import only the packages named in "+
				"the allowed map. Adding a new import requires an ADR "+
				"and an update to this test.", path)
		}
	}

	// Every allowed import must be present. A missing import means
	// the entry point no longer performs the action the import
	// enables; the test reports the missing package so the
	// contributor can decide whether the action should be removed.
	present := make(map[string]bool, len(file.Imports))
	for _, imp := range file.Imports {
		present[imp.Path.Value] = true
	}
	for path, reason := range allowed {
		if !present[path] {
			t.Errorf("expected import %s is missing (%s)", path, reason)
		}
	}
}

// TestMainGo_LineBudget verifies AC5: the total line count of main.go
// is within budget. The budget is 20 lines of *code*; comments and
// blank lines are excluded from the count.
//
// # The budget grew in WBS 11.3.1
//
// The WBS 2.2.1 budget was 20 lines, chosen to leave headroom over
// the canonical implementation's six lines. WBS 11.3.1 added the
// signal-aware context installation, which costs about ten lines
// of code (the signal.NotifyContext call and the os.Exit call).
// The new canonical implementation is around 12 lines; the budget
// stays at 20 to preserve the invariant's intent: the entry point
// must remain small enough that a reader can hold it in their
// head.
//
// # Why the budget did not grow
//
// A budget that grows whenever the code grows is not a budget. The
// WBS 11.3.1 addition fits inside the existing budget; a future
// WBS that does not fit must either simplify the entry point or
// argue for a larger budget in an ADR. The default is to keep the
// budget fixed.
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

// TestMainGo_ContainsExactlyOneDeferredCall verifies the WBS 11.3.1
// invariant that the entry point contains exactly one deferred
// call: the `defer stop()` that unregisters the signal handler.
//
// # Why the deferred call is permitted
//
// WBS 2.2.1's structural tests forbade defer statements. The
// prohibition was correct when the entry point did nothing but
// call cli.Execute and os.Exit: a defer in that code would be
// either dead or a sign that logic had leaked into main. WBS
// 11.3.1 adds the signal.NotifyContext install, whose stop
// function must be deferred so that the handler is unregistered
// before the process exits. The defer is not logic; it is the
// resource-cleanup half of the signal registration.
//
// # Why exactly one
//
// A second defer would be a sign that a second resource is being
// acquired. The entry point's approved resources are the signal
// handler (whose release is deferred) and the CLI's own state
// (released inside cli.Run, not here). A future WBS that adds a
// second resource requiring deferred cleanup must add a test for
// it in the same commit; this test's strictness is what forces
// the addition to be deliberate.
func TestMainGo_ContainsExactlyOneDeferredCall(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, mainGoPath, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", mainGoPath, err)
	}

	var deferCount int
	ast.Inspect(file, func(n ast.Node) bool {
		if _, ok := n.(*ast.DeferStmt); ok {
			deferCount++
		}
		return true
	})

	if deferCount != 1 {
		t.Fatalf("expected exactly 1 deferred call, found %d", deferCount)
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
//
// # The defer prohibition was relaxed in WBS 11.3.1
//
// The list no longer includes "defer statement" as a forbidden
// pattern. WBS 11.3.1 added the `defer stop()` call that
// unregisters the signal handler; the defer is the correct
// mechanism for that cleanup. The dedicated test
// TestMainGo_ContainsExactlyOneDeferredCall pins the count at one;
// this test no longer needs to forbid the construct entirely.
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
//
// # The phrases checked
//
// Three phrases from the file's docstring are load-bearing:
//
//   - "Command forge is the Forge CLI" — the package's identity.
//   - "This file must remain minimal" — the invariant's headline.
//   - "All logic belongs in internal/cli" — the invariant's substance.
//
// WBS 11.3.1 added a fourth phrase, "The exit boundary (WBS 11.3.1,
// ADR-020)", which this test checks as well. A future change that
// removes the section would fail the test and force the removal to
// be deliberate.
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
		// WBS 11.3.1 added the exit-boundary section. The section
		// documents the single-os.Exit rule and the signal
		// handling; its presence is part of the file's contract.
		"The exit boundary (WBS 11.3.1, ADR-020)",
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
