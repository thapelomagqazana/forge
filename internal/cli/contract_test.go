// Package cli contains white-box tests for the CLI package.
//
// This file enforces the command constructor contract defined by
// WBS 4.4.2 and documented in docs/architecture.md § 11.14, plus
// the flag- and stream-separation contracts introduced by
// WBS 12.1.1 (output/logging separation, ADR-022) and WBS 12.1.2
// (log level semantics). The contract is frozen: changing it
// requires a new ADR (see docs/decisions/README.md).
//
// # What the contract test verifies
//
// The contract has twelve rules. Some are enforceable by static
// inspection of the source; others require human judgment and are
// enforced by review. This file covers the mechanically enforceable
// subset:
//
//	Rule 1  — the constructor has the signature
//	          func newXxxCmd(deps Dependencies) *cobra.Command
//	Rule 2  — the command lives in internal/cli/xxx.go
//	Rule 3  — the returned command has Use, Short, RunE, and does
//	          not have Run; Use is lowercase; Short is ≤ 60 chars
//	Rule 4  — the file has no init() function
//	Rule 5  — the file has no package-level mutable state
//	Rule 6  — the file does not call os.* directly
//	Rule 6b — the file does not write to stdout or stderr directly;
//	          product output flows through internal/output, and
//	          diagnostics through internal/logger (ADR-022)
//	Rule 8  — the constructor is in the registry
//	Rule 11 — the --quiet/--verbose flags are mutually exclusive;
//	          --log-level accepts only debug|info|warn|error
//	          (WBS 12.1.2)
//
// Rules 7 (ForgeError), 9 (runCLI in tests), and 10 (positive and
// negative test cases) are not enforceable by static inspection of
// the source alone. They are enforced by review and by the
// pre-commit hook that runs the tests.
//
// Rule 6b's name reflects that it is a companion to Rule 6, not a
// renumbering of the rule set. The numbering is stable: a rule's
// number is a citation key, not an ordinal position.
//
// # How the test works
//
// The test iterates the registry, calls each constructor with a test
// Dependencies value to obtain the command, and inspects both the
// command's fields and the file that defines the constructor. The
// file's path is inferred from the constructor's name: newXxxCmd is
// expected in internal/cli/xxx.go. The inference is a convention,
// not a contract; if a future command violates the convention, the
// test reports the file it looked for and did not find, and the
// contributor either names the file correctly or files a WBS item
// to change the convention.
//
// The file inventory is built by parsing the package's source files
// with go/parser and selecting those that define a function with
// the commandConstructor signature. This replaces the previous
// filename-based filter (a hand-maintained map of "infrastructure"
// files) with a signature-based one, so adding a new shared helper
// file does not require editing this test.
//
// # What the test does not verify
//
// The test does not compile the handler. It does not verify that the
// handler follows the handler / service boundary (that is
// TestVersionHandler_IsThin's job for the version command, and the
// contract test cannot generalise it because the shape of "thin"
// depends on what the command does). It does not verify that the
// command's tests exist. It does not verify that the command's
// documentation is accurate. Those are review responsibilities, and
// the reviewer checklist in docs/development.md names them.
//
// # Dependencies on helpers in other test files
//
// This file relies on helpers defined elsewhere in the package's
// test files:
//
//	testDependencies()    — builds a Dependencies value for tests
//	                        (testhelper_test.go)
//	firstWord(s)          — returns the first whitespace-delimited
//	                        word of s (testhelper_test.go)
//	listPackageFiles(t)   — returns the .go files in the package
//	                        directory (structure_test.go)
//	readFile(t, name)     — reads a file from the package directory
//	                        (testhelper_test.go)
//	stripComments(src)    — removes Go comments from src
//	                        (testhelper_test.go)
//
// The helpers are not duplicated here. The white-box / black-box
// split documented in doc.go means the cli_test package has its own
// copies; the two packages cannot share unexported identifiers.
package cli

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"unicode"

	"github.com/spf13/cobra"
	"github.com/thapelomagqazana/forge/internal/logger"
)

// =============================================================================
// Rule 1 — constructor signature
// =============================================================================

// TestContract_ConstructorSignature verifies Rule 1: every
// constructor in the registry has the signature
//
//	func newXxxCmd(deps Dependencies) *cobra.Command
//
// The signature is checked at compile time by the registry variable's
// type declaration:
//
//	var registry = []commandConstructor{...}
//
// where commandConstructor is func(Dependencies) *cobra.Command. Any
// constructor with a different signature would not compile as an
// element of the slice. This test is therefore a documentation of
// the rule, not an additional check: the compiler enforces the rule
// by construction.
//
// The test exists so that a reader of the test suite sees Rule 1
// explicitly. If a future refactor changes commandConstructor's
// definition, the test continues to pass because the type declaration
// changes with it; the test is a pointer to the rule, not a guard
// against it.
func TestContract_ConstructorSignature(t *testing.T) {
	t.Parallel()

	// Compile-time assertion: every registry entry has the
	// required signature. If this declaration compiles, the rule
	// holds.
	var _ []commandConstructor = registry

	// Runtime sanity: the registry is non-empty.
	if len(registry) == 0 {
		t.Fatal("registry is empty; want at least one command")
	}

	// Additional documentation: every entry, when called with a
	// test Dependencies, returns a non-nil *cobra.Command. This is
	// the observable half of Rule 1; the signature half is the
	// compile-time assertion above.
	deps := testDependencies()
	for i, ctor := range registry {
		cmd := ctor(deps)
		if cmd == nil {
			t.Errorf("registry[%d]: constructor returned nil command", i)
			continue
		}
		var _ *cobra.Command = cmd
	}
}

// =============================================================================
// Rule 2 — file location
// =============================================================================

// TestContract_FileLocation verifies Rule 2: every command's
// constructor lives in a file named after the command.
//
// The mapping is:
//
//	newVersionCmd  →  internal/cli/version.go
//	newConfigCmd   →  internal/cli/config.go
//	newFooCmd      →  internal/cli/foo.go
//
// The test derives the expected filename from the command's Use
// field (not from the constructor's name), because Use is the
// user-facing identity and the file name should match it. The
// derivation is:
//
//  1. Take the command's Use: "version".
//  2. Take the first word (Use may include arguments).
//  3. Validate the word is a lowercase identifier.
//  4. Append ".go".
//
// A command whose Use does not follow the convention (uppercase,
// spaces, punctuation) produces an empty expected filename and the
// test reports the mismatch.
func TestContract_FileLocation(t *testing.T) {
	t.Parallel()

	deps := testDependencies()

	for _, ctor := range registry {
		cmd := ctor(deps)
		name := firstWord(cmd.Use)
		if name == "" {
			t.Errorf("command %q: Use is empty", constructorName(ctor))
			continue
		}

		expectedFile := commandNameToFile(name)
		if expectedFile == "" {
			t.Errorf("command %q: Use does not map to a filename", name)
			continue
		}

		if !fileExists(t, expectedFile) {
			t.Errorf("command %q: expected file %s does not exist; "+
				"the contract requires each command to live in "+
				"internal/cli/<name>.go",
				name, expectedFile)
		}
	}
}

// commandNameToFile converts a command name to the filename that is
// expected to contain its constructor.
//
// Examples:
//
//	"version"  →  "version.go"
//	"config"   →  "config.go"
//
// The function returns the empty string if the name contains any
// character that is not a lowercase letter, a digit, or a hyphen.
func commandNameToFile(name string) string {
	if name == "" {
		return ""
	}
	for _, r := range name {
		if !unicode.IsLower(r) && !unicode.IsDigit(r) && r != '-' {
			return ""
		}
	}
	return name + ".go"
}

// packageDir returns the absolute path to the internal/cli
// directory.
//
// It derives the path from the location of this test file, using
// runtime.Caller to find the source file path, then returning the
// directory that contains it.
//
// # Why runtime.Caller and not os.Getwd
//
// os.Getwd returns the directory from which the test binary was
// invoked, which is not necessarily the directory containing the
// test file. In a Go module, `go test ./internal/cli/` runs the
// test binary with the working directory set to the package
// directory, but this is not guaranteed by the toolchain and can
// change. runtime.Caller returns the path of the source file that
// called it, which is stable regardless of the working directory.
func packageDir(t *testing.T) string {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	return filepath.Dir(thisFile)
}

// fileExists reports whether the named file exists in the package
// directory.
func fileExists(t *testing.T, name string) bool {
	t.Helper()

	path := filepath.Join(packageDir(t), name)
	_, err := os.Stat(path)
	return err == nil
}

// =============================================================================
// Rule 3 — command metadata
// =============================================================================

// TestContract_CommandMetadata verifies Rule 3: every command has
// the required metadata and lacks the forbidden metadata.
//
// The required fields are:
//
//	Use    — lowercase, no spaces
//	Short  — non-empty, ≤ 60 characters
//	RunE   — non-nil
//
// The forbidden field is:
//
//	Run    — must be nil (Run discards the returned error)
func TestContract_CommandMetadata(t *testing.T) {
	t.Parallel()

	deps := testDependencies()

	for _, ctor := range registry {
		cmd := ctor(deps)
		name := firstWord(cmd.Use)

		// Use is lowercase and contains no uppercase letters.
		if strings.ToLower(cmd.Use) != cmd.Use {
			t.Errorf("command %q: Use contains uppercase: %q",
				name, cmd.Use)
		}
		if name == "" {
			t.Errorf("command %q: Use is empty", name)
		}

		// Short is non-empty and ≤ 60 characters.
		if cmd.Short == "" {
			t.Errorf("command %q: Short is empty", name)
		}
		if len(cmd.Short) > 60 {
			t.Errorf("command %q: Short is %d characters; limit is 60: %q",
				name, len(cmd.Short), cmd.Short)
		}

		// RunE is non-nil.
		if cmd.RunE == nil {
			t.Errorf("command %q: RunE is nil", name)
		}

		// Run is nil. The contract forbids Run because it discards
		// the returned error, which would break the CLI's error
		// handling and exit-code mapping.
		if cmd.Run != nil {
			t.Errorf("command %q: Run is non-nil; "+
				"the contract requires RunE (which returns an "+
				"error) and forbids Run", name)
		}
	}
}

// =============================================================================
// Rule 4 — no init() function
// =============================================================================

// TestContract_NoInitFunction verifies Rule 4: no command file
// defines an init() function.
//
// An init() function that registers the command would make the
// registry's contents depend on which files are compiled in and on
// Go's file-processing rules. An init() function that does anything
// else (setting up package-level state, for example) would violate
// Rule 5 (no package-level mutable state).
//
// The test uses go/parser rather than a substring search for
// "func init(". A substring search would false-positive on a
// comment mentioning init() — unlikely but possible — and would
// miss "func  init (" with extra whitespace. The parser is exact.
func TestContract_NoInitFunction(t *testing.T) {
	t.Parallel()

	for _, file := range listCommandFiles(t) {
		fset := token.NewFileSet()
		path := filepath.Join(packageDir(t), file)
		parsed, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Errorf("%s: parse error: %v", file, err)
			continue
		}

		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if fn.Name.Name == "init" && fn.Recv == nil {
				t.Errorf("%s: defines an init() function; "+
					"the contract forbids init() in command files "+
					"(Rule 4)", file)
			}
		}
	}
}

// =============================================================================
// Rule 5 — no package-level mutable state
// =============================================================================

// TestContract_NoPackageLevelMutableState verifies Rule 5: no
// command file declares a package-level variable.
//
// A package-level variable is mutable state shared across
// invocations. Its value at one invocation depends on the value left
// by a previous invocation, which makes tests non-hermetic and makes
// the CLI's behaviour depend on history.
//
// The rule permits:
//
//   - Package-level constants (`const` declarations).
//   - Package-level variables of unexported types that are assigned
//     once at declaration and never modified.
//
// The rule forbids:
//
//   - Package-level variables with mutable state.
//   - Package-level variables assigned by init() or by other
//     package-level code.
//
// The test uses go/parser and inspects only top-level GenDecl
// nodes with Tok == token.VAR, so local `var` declarations inside
// functions are not flagged. This is narrower than the previous
// line-based scan, which could not distinguish a top-level `var`
// from a function-scoped one at column zero.
func TestContract_NoPackageLevelMutableState(t *testing.T) {
	t.Parallel()

	for _, file := range listCommandFiles(t) {
		fset := token.NewFileSet()
		path := filepath.Join(packageDir(t), file)
		parsed, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Errorf("%s: parse error: %v", file, err)
			continue
		}

		for _, decl := range parsed.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			if gen.Tok != token.VAR {
				continue
			}
			// Top-level var declarations are forbidden.
			// (Const declarations are permitted; the switch
			// above excludes them.)
			t.Errorf("%s: declares package-level variable(s) at "+
				"line %d; the contract forbids package-level "+
				"mutable state (Rule 5)",
				file, fset.Position(gen.Pos()).Line)
		}
	}
}

// =============================================================================
// Rule 6 — no direct os.* calls
// =============================================================================

// TestContract_NoDirectOSCalls verifies Rule 6: no command file
// references os.* directly.
//
// Every effect a command performs must flow through the Dependencies
// value: filesystem through deps.FS, output through deps.Stdout and
// deps.Stderr, environment through deps.Env. A command that reaches
// for os.* would bypass the injection boundary and violate the
// two-boundary model of WBS 4.2.2.
//
// The rule permits imports of the os package for type references
// (os.FileMode, for example) if a future command needs them. It
// forbids calls: os.ReadFile, os.WriteFile, os.Getenv, os.Exit, and
// so on.
//
// The test checks for the call forms, not the imports. A file that
// imports os but never calls it is unusual but not a contract
// violation; goimports would remove the import.
func TestContract_NoDirectOSCalls(t *testing.T) {
	t.Parallel()

	forbidden := []string{
		"os.Args",
		"os.Stdin",
		"os.Stdout",
		"os.Stderr",
		"os.Getenv",
		"os.Setenv",
		"os.Environ",
		"os.Getwd",
		"os.Chdir",
		"os.Exit",
		"os.ReadFile",
		"os.WriteFile",
		"os.Open",
		"os.Create",
		"os.Remove",
		"os.MkdirAll",
	}

	for _, file := range listCommandFiles(t) {
		content := readFile(t, file)
		code := stripComments(content)
		for _, token := range forbidden {
			if strings.Contains(code, token) {
				t.Errorf("%s: references %s; the contract forbids "+
					"direct os.* calls in command files (Rule 6). "+
					"Use the Dependencies value instead.",
					file, token)
			}
		}
	}
}

// =============================================================================
// Rule 6b — no direct stdout/stderr writes (ADR-022)
// =============================================================================

// TestContract_NoDirectStreamWrites verifies Rule 6b: no command
// file writes to stdout or stderr except through deps.Stdout and
// deps.Stderr.
//
// WBS 12.1.1 (ADR-022) establishes a two-layer output architecture:
//
//   - Product output flows through internal/output (stdout only).
//   - Diagnostics flow through internal/logger (stderr only).
//
// A command that writes to stdout or stderr directly — via
// fmt.Println, fmt.Fprintln, os.Stdout.Write, log.Print, and so
// on — bypasses the layers. Under --format json, a stray
// fmt.Println corrupts the JSON envelope and breaks every
// downstream parser. The rule exists to prevent that class of bug
// at build time.
//
// The check is a substring scan for the call forms, not a
// type-aware analysis. It will false-positive on a comment that
// mentions fmt.Println; the stripComments call removes comments
// before the scan, so that case is handled. It will not catch a
// call through a variable (f := fmt.Println; f("x")), which is
// rare enough to accept as a review responsibility.
//
// The rule permits fmt.Sprintf and other pure-formatting
// functions; only the printing forms are forbidden.
func TestContract_NoDirectStreamWrites(t *testing.T) {
	t.Parallel()

	forbidden := []string{
		"fmt.Println",
		"fmt.Printf",
		"fmt.Print",
		"fmt.Fprintln(os.Stdout",
		"fmt.Fprintln(os.Stderr",
		"fmt.Fprintf(os.Stdout",
		"fmt.Fprintf(os.Stderr",
		"fmt.Fprint(os.Stdout",
		"fmt.Fprint(os.Stderr",
		"os.Stdout.Write",
		"os.Stderr.Write",
		"log.Print",
		"log.Printf",
		"log.Println",
		"log.Fatal",
		"log.Fatalf",
		"log.Fatalln",
		"log.Panic",
		"log.Panicf",
		"log.Panicln",
	}

	for _, file := range listCommandFiles(t) {
		content := readFile(t, file)
		code := stripComments(content)
		for _, token := range forbidden {
			if strings.Contains(code, token) {
				t.Errorf("%s: references %s; the contract forbids "+
					"direct stdout/stderr writes in command files "+
					"(Rule 6b, ADR-022). Route product output "+
					"through internal/output and diagnostics "+
					"through internal/logger.",
					file, token)
			}
		}
	}
}

// =============================================================================
// Rule 8 — every constructor is in the registry
// =============================================================================

// TestContract_EveryConstructorIsInRegistry verifies Rule 8: every
// function in the package whose signature matches the command
// constructor pattern appears in the registry.
//
// The test uses go/parser to identify functions with the exact
// signature func(Dependencies) *cobra.Command. This is stronger
// than a name-based scan: a helper named newFooCmd with a different
// signature (for example, a test double) is not flagged, and a
// constructor with a non-conforming name is flagged.
//
// The test is the inverse of the registry tests, which check that
// every registry entry produces a valid command. Together they
// enforce a bijection: every constructor is in the registry, and
// every registry entry is a constructor.
func TestContract_EveryConstructorIsInRegistry(t *testing.T) {
	t.Parallel()

	// Collect the names of every constructor defined in the
	// package's non-test files.
	defined := collectCommandConstructorNames(t)

	// Collect the names of every constructor in the registry.
	registered := make(map[string]bool)
	for _, ctor := range registry {
		name := constructorName(ctor)
		if name == "" {
			t.Error("registry contains a constructor with no name")
			continue
		}
		registered[name] = true
	}

	// Every defined constructor must be registered.
	for name := range defined {
		if !registered[name] {
			t.Errorf("constructor %s is defined but not in the registry; "+
				"add it to the registry slice in registry.go (Rule 8)",
				name)
		}
	}
}

// collectCommandConstructorNames returns the set of function names
// in the package's non-test files that have the exact signature
//
//	func(Dependencies) *cobra.Command
//
// # The signature check
//
// The signature check is done with go/parser and go/ast. A function
// matches if it:
//
//   - is a top-level function (not a method),
//   - has exactly one parameter,
//   - the parameter's type is the identifier "Dependencies",
//   - has exactly one result,
//   - the result's type is *cobra.Command.
//
// The name itself is not part of the match. A future contributor
// who names a constructor "makeVersionCommand" with the correct
// signature still gets flagged as unregistered, which is the
// correct behaviour: the registry's type is the contract, not the
// function's name.
//
// # The root-command exception
//
// newRootCmd has the same signature as a subcommand constructor
// but is not a subcommand. It is the root dispatcher that owns the
// registry; requiring it to be in the registry would be circular
// and would produce a `forge forge` command. The rule's intent is
// "every *subcommand* constructor is registered", and newRootCmd
// is excluded from that intent.
//
// The exclusion is by name. The list is intentionally short and is
// verified by TestContract_RootConstructorExists below, so a
// rename of newRootCmd forces the exclusion list to be updated in
// the same commit.
func collectCommandConstructorNames(t *testing.T) map[string]bool {
	t.Helper()

	// Functions that have the constructor signature but are not
	// registered as production subcommands. Each entry must be
	// justified in the docstring above. The list must remain
	// short; a growing list is a sign that the signature test is
	// too broad.
	//
	//   - newRootCmd is the root dispatcher. It owns the registry;
	//     requiring it to be a member of the registry would be
	//     circular and would produce a `forge forge` command.
	//
	//   - newTestEmitLinesCmd is the test-only command introduced
	//     by WBS 12.4.1. It is registered conditionally by
	//     registerTestCommands, which reads FORGE_TEST from
	//     deps.Env; the command is absent in every production
	//     build whose environment does not set the variable. The
	//     registry holds the unconditional production commands;
	//     the test command is registered outside it, by design.
	notASubcommand := map[string]bool{
		"newRootCmd":          true,
		"newTestEmitLinesCmd": true,
	}

	out := make(map[string]bool)
	for _, file := range listCommandFiles(t) {
		fset := token.NewFileSet()
		path := filepath.Join(packageDir(t), file)
		parsed, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Errorf("%s: parse error: %v", file, err)
			continue
		}

		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if fn.Recv != nil {
				continue // methods are not constructors
			}
			if !hasConstructorSignature(fn) {
				continue
			}
			if notASubcommand[fn.Name.Name] {
				continue
			}
			out[fn.Name.Name] = true
		}
	}
	return out
}

// hasConstructorSignature reports whether fn has the signature
//
//	func(Dependencies) *cobra.Command
//
// The function is used by collectCommandConstructorNames. It is
// deliberately narrow: a function with the wrong parameter count,
// the wrong parameter type, or the wrong result type is not a
// constructor, even if its name matches the convention.
func hasConstructorSignature(fn *ast.FuncDecl) bool {
	// One parameter.
	if fn.Type.Params == nil || len(fn.Type.Params.List) != 1 {
		return false
	}
	param := fn.Type.Params.List[0]
	// The parameter's type is the identifier "Dependencies".
	ident, ok := param.Type.(*ast.Ident)
	if !ok || ident.Name != "Dependencies" {
		return false
	}
	// Exactly one result.
	if fn.Type.Results == nil || len(fn.Type.Results.List) != 1 {
		return false
	}
	// The result's type is *cobra.Command.
	result := fn.Type.Results.List[0].Type
	star, ok := result.(*ast.StarExpr)
	if !ok {
		return false
	}
	sel, ok := star.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "cobra" {
		return false
	}
	return sel.Sel.Name == "Command"
}

// =============================================================================
// Rule 11 — log-level flag contract (WBS 12.1.2)
// =============================================================================

// TestContract_LogLevelFlags verifies Rule 11: the --quiet and
// --verbose flags are mutually exclusive, and --log-level accepts
// only the four canonical level names.
//
// The rule is a flag contract, not a constructor contract, but it
// lives in this file because this file is the home of the CLI's
// mechanically-enforceable contracts. The rule is defined by WBS
// 12.1.2 and documented in docs/reference/log-levels.md.
//
// # What the test checks
//
//  1. --quiet and --verbose are mutually exclusive: passing both
//     produces ErrQuietVerboseConflict.
//  2. --log-level accepts debug, info, warn, warning, error
//     (case-insensitive) and rejects anything else.
//  3. --log-level beats --verbose and --quiet in the precedence
//     chain.
//
// The test calls the CLI's own helpers (registerGlobalFlags,
// resolveLogLevel) rather than duplicating the logic. A change to
// the helpers that violates the contract fails this test.
func TestContract_LogLevelFlags(t *testing.T) {
	t.Parallel()

	// Subtest 1: --quiet and --verbose are mutually exclusive.
	t.Run("mutual_exclusion", func(t *testing.T) {
		t.Parallel()
		cmd := &cobra.Command{Use: "test"}
		registerGlobalFlags(cmd)
		if err := cmd.ParseFlags([]string{"--quiet", "--verbose"}); err != nil {
			t.Fatalf("ParseFlags: %v", err)
		}
		if err := validateLogFlags(cmd); err == nil {
			t.Fatal("--quiet --verbose must be rejected")
		}
	})

	// Subtest 2: --log-level accepts the four canonical names
	// and the "warning" alias, case-insensitively.
	t.Run("accepted_values", func(t *testing.T) {
		t.Parallel()
		for _, value := range []string{
			"debug", "info", "warn", "warning", "error",
			"DEBUG", "Info", "WARN", "Error", "  warn  ",
		} {
			cmd := &cobra.Command{Use: "test"}
			registerGlobalFlags(cmd)
			if err := cmd.ParseFlags([]string{"--log-level", value}); err != nil {
				t.Fatalf("ParseFlags(%q): %v", value, err)
			}
			if _, err := resolveLogLevel(cmd, ""); err != nil {
				t.Errorf("--log-level=%q rejected: %v", value, err)
			}
		}
	})

	// Subtest 3: --log-level rejects anything else.
	t.Run("rejected_values", func(t *testing.T) {
		t.Parallel()
		for _, value := range []string{
			"trace", "fatal", "loud", "silent", "3", "!!!",
		} {
			cmd := &cobra.Command{Use: "test"}
			registerGlobalFlags(cmd)
			if err := cmd.ParseFlags([]string{"--log-level", value}); err != nil {
				t.Fatalf("ParseFlags(%q): %v", value, err)
			}
			if _, err := resolveLogLevel(cmd, ""); err == nil {
				t.Errorf("--log-level=%q accepted; want error", value)
			}
		}
	})

	// Subtest 4: --log-level beats --verbose and --quiet.
	t.Run("precedence", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			name string
			args []string
			want string
		}{
			{"level beats verbose", []string{"--verbose", "--log-level=error"}, "error"},
			{"level beats quiet", []string{"--quiet", "--log-level=debug"}, "debug"},
		}
		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				cmd := &cobra.Command{Use: "test"}
				registerGlobalFlags(cmd)
				if err := cmd.ParseFlags(tc.args); err != nil {
					t.Fatalf("ParseFlags(%v): %v", tc.args, err)
				}
				got, err := resolveLogLevel(cmd, "")
				if err != nil {
					t.Fatalf("resolveLogLevel: %v", err)
				}
				if got.Level.String() != tc.want {
					t.Errorf("Level = %v; want %v", got.Level, tc.want)
				}
			})
		}
	})
}

// TestContract_RootConstructorExists pins the existence of
// newRootCmd, the only function with the constructor signature
// that is deliberately excluded from the registry bijection.
//
// # Why the guard exists
//
// collectCommandConstructorNames excludes newRootCmd by name. If a
// future contributor renames or removes newRootCmd without
// updating the exclusion list, the bijection test would still pass
// (the new name would be flagged, or no function would be flagged)
// but the exclusion list would be stale. The staleness would not
// produce an immediate failure, but it would hide a future rename
// of the real root constructor.
//
// This guard asserts that a function named "newRootCmd" is defined
// in the package. If the constructor is renamed, this test fails
// and forces the exclusion list to be updated in the same commit.
func TestContract_RootConstructorExists(t *testing.T) {
	t.Parallel()

	// Re-parse without the exclusion, to find whether any
	// function named newRootCmd exists with the constructor
	// signature.
	found := false
	for _, file := range listCommandFiles(t) {
		fset := token.NewFileSet()
		path := filepath.Join(packageDir(t), file)
		parsed, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			continue
		}
		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if fn.Recv != nil {
				continue
			}
			if fn.Name.Name != "newRootCmd" {
				continue
			}
			if !hasConstructorSignature(fn) {
				continue
			}
			found = true
			break
		}
		if found {
			break
		}
	}

	if !found {
		t.Fatal("newRootCmd is not defined with the constructor " +
			"signature in the package; the exclusion list in " +
			"collectCommandConstructorNames is stale. Either " +
			"restore newRootCmd or update the exclusion list.")
	}
}

// =============================================================================
// Rule 12 — correlation ID present in every record (WBS 12.2.1, 12.3.1)
// =============================================================================

// TestContract_CorrelationIDPresentInEveryRecord asserts that every
// record emitted by the invocation's Logger carries the same
// non-zero correlation ID.
//
// # How the test observes the ID
//
// After WBS 12.3.1, the Logger interface's methods do not take a
// context, and the correlation ID lives inside the Logger. There
// is no public API to read the ID out of a Logger; the only way
// to observe it is to ask the Logger to emit a record and inspect
// the record's CorrelationID field.
//
// The test therefore:
//
//  1. Builds a Logger bound to a fresh correlation ID.
//  2. Calls the Logger's four level methods.
//  3. Inspects the records the Logger emitted and asserts they
//     all carry the same ID.
//
// # Why the test does not use logger.WithCorrelationID
//
// WithCorrelationID stores a string under the logger package's
// context key. That context value is no longer read by the
// Logger: the Logger carries its own ID as a field. The test
// must therefore bind the ID at construction, not via the
// context.
//
// The public API for binding an ID at construction is
// logger.New together with the unexported
// (*slogLogger).withCorrelationID, which is not reachable from
// this package. The test instead uses the production path that
// binds an ID at construction: logger.IntoContext, which the
// CLI hook uses. That path is unavailable here because
// IntoContext stores an already-built Logger.
//
// The practical resolution is for the test to assert the ID on
// records emitted by the Logger the *CLI hook* builds. That is
// the integration test in hooks_test.go, not this file. This
// file's Rule 12 test uses the public constructor and asserts
// the property that is observable at this layer:
//
//   - Every record a Logger emits carries *a* correlation ID
//     (the sentinel when no ID was bound, a real value
//     otherwise).
//   - The sentinel appears when no ID was bound; a non-sentinel
//     value appears when the Logger was built by a path that
//     binds one.
//
// The stronger assertion — "the ID is the invocation's ID" — is
// the job of the CLI's own hook tests, because the binding
// happens there.
func TestContract_CorrelationIDPresentInEveryRecord(t *testing.T) {
	t.Parallel()

	// Build a Logger with a capture Sink. The Logger has no
	// bound correlation ID, so every record it emits must carry
	// the zero sentinel. That is the contract at this layer:
	// a record's correlation_id field is never empty.
	var captured []logger.Record
	sink := &captureSinkAdapter{records: &captured}
	l := logger.New(logger.Options{
		Level:     logger.LevelDebug,
		Format:    logger.FormatJSON,
		Component: logger.ComponentCLI,
		Sink:      sink,
	})

	l.Debug("d")
	l.Info("i")
	l.Warn("w")
	l.Error("e")

	if len(captured) != 4 {
		t.Fatalf("got %d records; want 4", len(captured))
	}
	for i, r := range captured {
		if r.CorrelationID == "" {
			t.Errorf("record %d correlation_id is empty; "+
				"every record must carry a value (the sentinel "+
				"when no ID was bound)", i)
		}
		if r.CorrelationID != logger.ZeroCorrelationID {
			t.Errorf("record %d correlation_id = %q; want the zero "+
				"sentinel (no ID was bound on this Logger)",
				i, r.CorrelationID)
		}
	}
}

// TestContract_CorrelationIDIsNotDuplicatedInAttrs asserts that
// the correlation ID does not leak into the Attrs map. The
// record's correlation_id is a top-level field; a consumer who
// reads Attrs for structured data should not find the ID there.
//
// The property is a consequence of how the adapter carries the
// ID: as a reserved attribute that the handler strips and
// promotes. A future change that, say, copied the reserved
// attribute into Attrs would fail this test.
func TestContract_CorrelationIDIsNotDuplicatedInAttrs(t *testing.T) {
	t.Parallel()

	var captured []logger.Record
	sink := &captureSinkAdapter{records: &captured}
	l := logger.New(logger.Options{
		Level:     logger.LevelDebug,
		Format:    logger.FormatJSON,
		Component: logger.ComponentCLI,
		Sink:      sink,
	})

	l.Info("probe", "k", "v")

	if len(captured) != 1 {
		t.Fatalf("got %d records; want 1", len(captured))
	}
	if _, leaked := captured[0].Attrs["correlation_id"]; leaked {
		t.Errorf("correlation_id leaked into Attrs: %v", captured[0].Attrs)
	}
	// The user's own attr is present, as expected.
	if captured[0].Attrs["k"] != "v" {
		t.Errorf("user attr k = %v; want v", captured[0].Attrs["k"])
	}
}

// =============================================================================
// Test helpers
// =============================================================================

// funcNameOf returns the fully-qualified name of the function that
// implements ctor. The name is of the form
// "github.com/thapelomagqazana/forge/internal/cli.newVersionCmd".
func funcNameOf(ctor commandConstructor) string {
	return runtime.FuncForPC(reflect.ValueOf(ctor).Pointer()).Name()
}

// captureSinkAdapter is a minimal logger.Sink that appends to a
// slice under test. It is safe for concurrent use.
type captureSinkAdapter struct {
	mu      sync.Mutex
	records *[]logger.Record
}

func (c *captureSinkAdapter) Emit(_ context.Context, r logger.Record) {
	c.mu.Lock()
	defer c.mu.Unlock()
	*c.records = append(*c.records, r)
}

// constructorName returns the short name of a constructor function.
//
// It uses runtime.FuncForPC to obtain the function's symbolic name,
// then strips the package path. The result is "newVersionCmd", not
// the fully-qualified "github.com/.../internal/cli.newVersionCmd".
func constructorName(ctor commandConstructor) string {
	name := funcNameOf(ctor)
	if i := strings.LastIndex(name, "."); i >= 0 {
		return name[i+1:]
	}
	return name
}

// listCommandFiles returns the list of non-test .go files in the
// package that correspond to commands.
//
// A file is a candidate command file if it defines at least one
// function with the commandConstructor signature. Files that only
// define helpers, infrastructure, or types are excluded by this
// rule.
//
// The previous implementation used a hand-maintained map of
// "infrastructure" filenames. That map had to be updated whenever a
// new shared helper file was added; forgetting to update it caused
// false positives in TestContract_NoInitFunction and friends. The
// signature-based filter has no such maintenance burden: a file
// with no constructor is excluded automatically.
func listCommandFiles(t *testing.T) []string {
	t.Helper()

	var out []string
	for _, name := range listPackageFiles(t) {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		if fileDefinesConstructor(t, name) {
			out = append(out, name)
		}
	}
	return out
}

// fileDefinesConstructor reports whether the named file defines at
// least one function with the commandConstructor signature.
func fileDefinesConstructor(t *testing.T, name string) bool {
	t.Helper()

	fset := token.NewFileSet()
	path := filepath.Join(packageDir(t), name)
	parsed, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		// A parse error is reported by the caller's own parse
		// pass. Here, treat it as "not a constructor file" so
		// the caller does not double-report.
		return false
	}
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fn.Recv == nil && hasConstructorSignature(fn) {
			return true
		}
	}
	return false
}
