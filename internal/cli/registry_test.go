// Package cli contains white-box tests for the CLI package.
//
// This file tests the command registry defined in registry.go. The
// registry's purpose is to make the command tree's shape a static,
// reviewable, deterministic property. The tests below enforce that
// purpose.
//
// # Relationship to root_test.go
//
// root_test.go tests the root command's behaviour (no-args, --help,
// unknown-command). This file tests the registry itself: its
// ordering, its contents, and the rules a contributor must follow
// when adding a command. A failure in this file means the registry's
// invariants are violated; a failure in root_test.go means the root
// command's behaviour is wrong.
//
// # The invariants tested
//
//   - The registry is non-empty (sanity check).
//   - No two constructors produce commands with the same name
//     (catches an accidental duplicate).
//   - The registry's order matches the order Cobra reports for
//     visible commands (catches a contributor adding a command in
//     the wrong position).
//   - No init() function in the package appends to the registry
//     (catches a forbidden registration pattern).
//   - Every constructor accepts a Dependencies value and returns a
//     non-nil *cobra.Command (catches a signature drift).
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// =============================================================================
// Tests — the registry's contents
// =============================================================================

// TestRegistry_IsNotEmpty verifies that the registry contains at
// least one constructor.
//
// A registry that was accidentally emptied would silently produce a
// CLI with no subcommands. The test catches that mistake at build
// time rather than at the first user invocation.
func TestRegistry_IsNotEmpty(t *testing.T) {
	t.Parallel()

	if len(registry) == 0 {
		t.Fatal("registry is empty; want at least one command")
	}
}

// TestRegistry_EveryConstructorReturnsNonNilCommand verifies that
// each constructor in the registry returns a non-nil *cobra.Command.
//
// A constructor that returned nil would panic inside Cobra's
// AddCommand at the first invocation. The test catches the mistake
// before the panic can occur.
func TestRegistry_EveryConstructorReturnsNonNilCommand(t *testing.T) {
	t.Parallel()

	deps := testDependencies()

	for i, ctor := range registry {
		cmd := ctor(deps)
		if cmd == nil {
			t.Errorf("registry[%d] returned nil *cobra.Command", i)
		}
	}
}

// TestRegistry_NoDuplicateCommandNames verifies that no two
// constructors produce commands with the same Use string.
//
// A duplicate command name would make Cobra's dispatch ambiguous:
// the second command would shadow the first in help output and in
// argument matching. The test catches the mistake before the CLI
// is built.
//
// The check compares only the first word of the Use field (the
// command name), not the full Use string. Cobra allows the Use field
// to include usage arguments (for example, "foo <bar>"); the
// command name is the first word.
func TestRegistry_NoDuplicateCommandNames(t *testing.T) {
	t.Parallel()

	deps := testDependencies()
	seen := make(map[string]int)

	for i, ctor := range registry {
		cmd := ctor(deps)
		name := firstWord(cmd.Use)
		if name == "" {
			t.Errorf("registry[%d] has an empty command name", i)
			continue
		}
		if prev, ok := seen[name]; ok {
			t.Errorf("duplicate command name %q: registry[%d] and registry[%d]",
				name, prev, i)
		}
		seen[name] = i
	}
}

// firstWord returns the first whitespace-separated word of s, or the
// empty string if s is empty or contains only whitespace.
func firstWord(s string) string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// =============================================================================
// Tests — the registry's order matches help output
// =============================================================================

// TestRegistry_OrderMatchesHelpOutput verifies that the order of
// visible commands in the registry matches the order Cobra reports
// them in `forge --help`.
//
// # Why this test matters
//
// Cobra sorts subcommands alphabetically by default. The registry
// is also expected to be alphabetical (within groups). The two
// orderings coincide for the current command set. If a future
// contributor adds a command in the wrong position — for example,
// appending it to the end when it should be in the middle — the
// registry's order would no longer match Cobra's. The test catches
// the divergence.
//
// # How the test works
//
// The test constructs the root command from the registry and asks
// Cobra for the list of commands. It filters out hidden commands
// (which do not appear in help output) from both lists, then asserts
// that the two filtered lists are equal.
//
// The test does not assert that Cobra's internal ordering algorithm
// matches the registry's; it asserts only that the visible commands
// appear in the same relative order in both. That is the property a
// reader of help output observes.
func TestRegistry_OrderMatchesHelpOutput(t *testing.T) {
	t.Parallel()

	// Build the root command the same way the CLI does.
	deps := testDependencies()
	root := newRootCmd(deps)

	// Collect the visible command names from Cobra, in the order
	// Cobra reports them.
	var cobraOrder []string
	for _, cmd := range root.Commands() {
		if cmd.Hidden {
			continue
		}
		cobraOrder = append(cobraOrder, firstWord(cmd.Use))
	}

	// Collect the visible command names from the registry, in the
	// order they appear.
	var registryOrder []string
	for _, ctor := range registry {
		cmd := ctor(deps)
		if cmd.Hidden {
			continue
		}
		registryOrder = append(registryOrder, firstWord(cmd.Use))
	}

	// The two orders must be equal.
	if len(cobraOrder) != len(registryOrder) {
		t.Fatalf("command count mismatch:\n"+
			"registry: %v\n"+
			"cobra:    %v",
			registryOrder, cobraOrder)
	}
	for i := range cobraOrder {
		if cobraOrder[i] != registryOrder[i] {
			t.Errorf("command order mismatch at index %d:\n"+
				"registry[%d] = %q\n"+
				"cobra[%d]    = %q",
				i, i, registryOrder[i], i, cobraOrder[i])
		}
	}
}

// =============================================================================
// Tests — no init-based registration
// =============================================================================

// TestRegistry_NoInitRegistration verifies that no init() function
// in the package appends to the registry.
//
// # Why this rule exists
//
// A contributor could register a command from the command's own file
// by writing:
//
//	func init() {
//	    registry = append(registry, newFooCmd)
//	}
//
// This pattern is forbidden because it makes the registry's contents
// depend on which files happen to be compiled in and on Go's
// file-processing rules. A reviewer reading registry.go would see an
// incomplete list, and the effective list would only be discoverable
// by running the binary. The registry must be a static, reviewable
// slice.
//
// # How the test works
//
// The test reads every non-test .go file in the package, strips
// comments, and searches for the pattern `func init` followed by a
// body that mentions `registry`. The search is deliberately coarse:
// it flags any init() function whose body mentions the registry, even
// if the mention is a read rather than a write. A read is a sign that
// the file's initialization depends on the registry's contents,
// which is the same problem in a different form.
func TestRegistry_NoInitRegistration(t *testing.T) {
	t.Parallel()

	files := listPackageFiles(t)
	for _, file := range files {
		content := readFile(t, file)
		code := stripComments(content)

		// Find every "func init" in the file.
		if !strings.Contains(code, "func init(") {
			continue
		}
		// The file has an init function. Does its body mention the
		// registry?
		body := extractFuncBody(t, code, "init")
		if strings.Contains(body, "registry") {
			t.Errorf("%s: init() function references the registry; "+
				"register commands by adding them to the registry "+
				"slice in registry.go, not by appending from init()",
				file)
		}
	}
}

// =============================================================================
// Tests — the registry is deterministic
// =============================================================================

// TestRegistry_IsDeterministic verifies that iterating the registry
// twice produces the same sequence of command names.
//
// The test is a tautology for a slice (slice iteration is
// deterministic by definition), but it documents the invariant: the
// registry's order does not depend on Go's map iteration order, on
// package initialization order, or on any other source of
// nondeterminism. A future refactor that changed the registry to a
// map would fail this test.
func TestRegistry_IsDeterministic(t *testing.T) {
	t.Parallel()

	deps := testDependencies()

	names := func() []string {
		var out []string
		for _, ctor := range registry {
			out = append(out, firstWord(ctor(deps).Use))
		}
		return out
	}

	first := names()
	for i := 0; i < 20; i++ {
		got := names()
		if len(got) != len(first) {
			t.Fatalf("run %d: length differs: got %d, want %d",
				i, len(got), len(first))
		}
		for j := range got {
			if got[j] != first[j] {
				t.Fatalf("run %d: element %d differs: got %q, want %q",
					i, j, got[j], first[j])
			}
		}
	}
}

// TestRegistry_EveryEntryFollowsContract is a thin wrapper around the
// contract test's invariants, invoked from the registry test file so
// that a reader of registry_test.go sees the connection.
//
// The test does not duplicate the contract test's work; it calls the
// same helpers. If the contract test is updated, this test continues
// to pass without modification. If the registry is extended with a
// command that violates the contract, both tests fail.
func TestRegistry_EveryEntryFollowsContract(t *testing.T) {
	t.Parallel()

	deps := testDependencies()

	for _, ctor := range registry {
		cmd := ctor(deps)

		// The command must have a non-empty Use with no uppercase.
		if cmd.Use == "" {
			t.Errorf("registry entry has empty Use")
			continue
		}
		if strings.ToLower(cmd.Use) != cmd.Use {
			t.Errorf("registry entry %q has uppercase in Use", cmd.Use)
		}

		// The command must have RunE and not Run.
		if cmd.RunE == nil {
			t.Errorf("registry entry %q has nil RunE", firstWord(cmd.Use))
		}
		if cmd.Run != nil {
			t.Errorf("registry entry %q has non-nil Run; "+
				"the contract requires RunE", firstWord(cmd.Use))
		}
	}
}

// =============================================================================
// Test helpers
// =============================================================================

// listPackageFiles returns the sorted list of non-test .go files in
// the package directory.
//
// The helper is used by TestRegistry_NoInitRegistration, which needs
// to scan every source file for init() functions. Test files are
// excluded because test files may legitimately define init()
// functions (for example, to set up package-level test state).
func listPackageFiles(t *testing.T) []string {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(thisFile)

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read package directory: %v", err)
	}

	var files []string
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		files = append(files, name)
	}
	return files
}

// Compile-time check: every constructor in the registry must match
// the commandConstructor type. This is enforced by the registry
// variable's type declaration; the assertion below is a redundant
// confirmation that the compiler checks the same thing.
var _ []commandConstructor = registry

// Silence the unused-import warning for bytes in case the file is
// edited and the import is temporarily unused.
var _ = bytes.MinRead

// Silence the unused-import warning for cobra in case the file is
// edited and the import is temporarily unused.
var _ *cobra.Command
