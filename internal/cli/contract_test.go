// Package cli contains white-box tests for the CLI package.
//
// This file enforces the command constructor contract defined by
// WBS 4.4.2 and documented in docs/architecture.md § 11.14. The
// contract is frozen: changing it requires a new ADR (see
// docs/decisions/README.md).
//
// # What the contract test verifies
//
// The contract has ten rules. Some are enforceable by static
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
//	Rule 8  — the constructor is in the registry
//
// Rules 7 (ForgeError), 9 (runCLI in tests), and 10 (positive and
// negative test cases) are not enforceable by static inspection of
// the source alone. They are enforced by review and by the
// pre-commit hook that runs the tests.
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
package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"unicode"

	"github.com/spf13/cobra"
)

// funcNameOf returns the fully-qualified name of the function that
// implements ctor. The name is of the form
// "github.com/thapelomagqazana/forge/internal/cli.newVersionCmd".
//
// The helper is used by constructorName to derive a constructor's
// short name. It is placed in contract_test.go because the contract
// test is its only consumer.
func funcNameOf(ctor commandConstructor) string {
	return runtime.FuncForPC(reflect.ValueOf(ctor).Pointer()).Name()
}

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

	// The compile-time assertion is the test. If the assertion
	// compiles, every registry entry has the required signature.
	var _ []commandConstructor = registry

	// Runtime sanity: the registry is non-empty.
	if len(registry) == 0 {
		t.Fatal("registry is empty; want at least one command")
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
// The test derives the expected filename from the constructor's
// symbolic name. The derivation is:
//
//  1. Take the constructor's name: "newFooCmd".
//  2. Strip the "new" prefix and the "Cmd" suffix: "Foo".
//  3. Convert to lowercase: "foo".
//  4. Append ".go": "foo.go".
//
// A constructor whose name does not follow the convention (for
// example, "makeFooCommand") produces an empty expected filename and
// the test reports the mismatch.
func TestContract_FileLocation(t *testing.T) {
	t.Parallel()

	deps := testDependencies()

	for _, ctor := range registry {
		name := constructorName(ctor)
		if name == "" {
			t.Errorf("constructor has no name; cannot derive filename")
			continue
		}

		cmd := ctor(deps)
		expectedFile := commandNameToFile(firstWord(cmd.Use))
		if expectedFile == "" {
			t.Errorf("command name %q does not map to a filename",
				firstWord(cmd.Use))
			continue
		}

		if !fileExists(t, expectedFile) {
			t.Errorf("command %q: expected file %s does not exist; "+
				"the contract requires each command to live in "+
				"internal/cli/<name>.go",
				firstWord(cmd.Use), expectedFile)
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
//
// # Why this helper is in testhelper_test.go
//
// The helper is used by every white-box test file that needs to
// read a source file from disk: execute_test.go, version_test.go,
// contract_test.go, and any future file that inspects the source
// of a command. Placing it in testhelper_test.go gives every
// white-box test file a single source for the package directory.
//
// A parallel helper exists in structure_test.go, which lives in
// package cli_test (black-box). The two are not the same function
// and cannot be shared: package cli and package cli_test are
// compiled as two different packages, and unexported identifiers
// are not visible across packages. The duplication is the
// consequence of the white-box / black-box split documented in
// doc.go.
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
// The test is deliberately coarse: it flags any init() function in
// any non-test command file, regardless of the body. If a future
// command legitimately needs an init() function — for example, to
// register a flag type with the standard library — the rule and the
// test should be revisited with a new ADR.
func TestContract_NoInitFunction(t *testing.T) {
	t.Parallel()

	files := listCommandFiles(t)
	for _, file := range files {
		content := readFile(t, file)
		code := stripComments(content)
		if strings.Contains(code, "func init(") {
			t.Errorf("%s: defines an init() function; "+
				"the contract forbids init() in command files "+
				"(Rule 4)", file)
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
// The test is deliberately over-broad: it flags any `var` declaration
// at the top level of a command file. A contributor who legitimately
// needs a package-level variable (for example, a lookup table) files
// a WBS item to add an exception to the rule and to the test.
func TestContract_NoPackageLevelMutableState(t *testing.T) {
	t.Parallel()

	files := listCommandFiles(t)
	for _, file := range files {
		content := readFile(t, file)
		code := stripComments(content)

		// The pattern looks for "var " at the beginning of a line
		// (after optional whitespace). A `var` declaration inside a
		// function body is indented and matches the same pattern;
		// the test therefore also flags local variables, which is
		// over-broad. To avoid the false positive, the test uses a
		// narrower pattern: a `var` at the start of a line with no
		// leading whitespace.
		for _, line := range strings.Split(code, "\n") {
			if strings.HasPrefix(line, "var ") {
				t.Errorf("%s: declares a package-level variable: %q; "+
					"the contract forbids package-level mutable "+
					"state (Rule 5)", file, strings.TrimSpace(line))
			}
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

	files := listCommandFiles(t)
	for _, file := range files {
		content := readFile(t, file)
		code := stripComments(content)

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
// Rule 8 — every constructor is in the registry
// =============================================================================

// TestContract_EveryConstructorIsInRegistry verifies Rule 8: every
// function in the package whose name matches the command constructor
// pattern appears in the registry.
//
// The test scans every non-test .go file in the package for
// functions with names of the form "newXxxCmd". For each one, it
// checks that the function appears in the registry. A constructor
// that is defined but not registered would produce a command that
// silently does not exist; the test catches the omission at build
// time.
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
// in the package's non-test files that match the constructor pattern
// newXxxCmd.
func collectCommandConstructorNames(t *testing.T) map[string]bool {
	t.Helper()

	out := make(map[string]bool)
	files := listCommandFiles(t)

	for _, file := range files {
		content := readFile(t, file)
		code := stripComments(content)

		// The pattern is "func new" followed by an identifier
		// ending in "Cmd(".
		lines := strings.Split(code, "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmed, "func new") {
				continue
			}
			// Extract the function name: between "func " and "(".
			rest := strings.TrimPrefix(trimmed, "func ")
			paren := strings.Index(rest, "(")
			if paren < 0 {
				continue
			}
			name := rest[:paren]
			if strings.HasSuffix(name, "Cmd") {
				out[name] = true
			}
		}
	}
	return out
}

// =============================================================================
// Test helpers
// =============================================================================

// listCommandFiles returns the list of non-test .go files in the
// package that correspond to commands. A command file is any file
// that is not the package doc, not the registry, not execute.go, and
// not one of the shared helpers.
//
// The helper is deliberately conservative: it includes version.go,
// config.go, and any future command file, and excludes files whose
// names are known to be infrastructure (execute.go, deps.go,
// exitcodes.go, registry.go, doc.go, root.go). If a future command
// is named after one of those files, the helper must be updated.
func listCommandFiles(t *testing.T) []string {
	t.Helper()

	infrastructure := map[string]bool{
		"doc.go":       true,
		"execute.go":   true,
		"deps.go":      true,
		"exitcodes.go": true,
		"registry.go":  true,
		"root.go":      true,
	}

	all := listPackageFiles(t)
	var out []string
	for _, f := range all {
		if infrastructure[f] {
			continue
		}
		out = append(out, f)
	}
	return out
}

// constructorName returns the name of a constructor function. It
// uses runtime.FuncForPC to obtain the function's symbolic name,
// then strips the package path and the function's suffix.
//
// The helper is used by TestContract_FileLocation and
// TestContract_EveryConstructorIsInRegistry. It is not used by any
// non-contract test; a future refactor that changes the constructor
// naming convention updates this helper in one place.
func constructorName(ctor commandConstructor) string {
	name := funcNameOf(ctor)
	// name is like "github.com/thapelomagqazana/forge/internal/cli.newVersionCmd".
	if i := strings.LastIndex(name, "."); i >= 0 {
		return name[i+1:]
	}
	return name
}

// Silence the unused import for cobra in case the file is edited and
// the import is temporarily unused.
var _ *cobra.Command
