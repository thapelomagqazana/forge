// Package cli defines the Forge command-line interface.
//
// This file holds the helpers that the help *output* contract
// (WBS 7.1.3) uses. The contract itself is exercised by the tests
// in help_content_test.go; the helpers here are the small
// functions those tests share.
//
// # What lives here
//
// This file holds:
//
//   - globalFlagNames: the set of persistent global flags that
//     every command inherits and that every command's help output
//     lists.
//   - commandName: a helper that extracts a command's name from its
//     Use field. Used by every test's subtest naming.
//   - allRegisteredCommands: a helper that resolves the registry to
//     a slice of constructed *cobra.Command values. Used by every
//     test that iterates the registry.
//   - commandSpecificFlagNames: a helper that returns the flags
//     registered on a command that are neither global flags nor
//     the auto-registered --help flag. Used by the metadata test
//     (via requiresExample) to decide whether a command requires
//     an Example.
//
// # What does not live here
//
// The metadata rules (Short shape, Long shape, Use regex, Args
// presence, Example presence, Aliases) live in metadata.go. The
// split reflects two different kinds of assertion: this file's
// helpers are used by tests that read the *rendered help output*;
// metadata.go's helpers are used by tests that read the
// *command's struct fields*.
//
// # No package-level mutable state
//
// The CLI package's contract test (Rule 5 in contract_test.go)
// forbids package-level mutable state. The helpers here are
// functions; the only package-level value is none.
package cli

import (
	"github.com/spf13/cobra"
)

// globalFlagNames returns the set of persistent global flags that
// every command inherits and must therefore list in its help
// output.
//
// The set is defined by WBS 5.3.1 and documented in
// docs/cli-ux-spec.md § 4.11. It is frozen for Phase 2; adding a
// fourth flag requires an ADR.
//
// # Why a function, not a variable
//
// The CLI package's contract test forbids package-level mutable
// state (Rule 5 in contract_test.go). A package-level
// `var globalFlagNames = []string{...}` is mutable: any code that
// imports the package could append to it or replace its elements.
// A function that returns a fresh slice on each call has no shared
// state; the returned slice is private to the caller.
//
// # Why a slice, not a map
//
// The slice preserves the order in which the flags appear in
// Cobra's "Global Flags:" section (alphabetical by long name). A
// test that asserts the presence of each flag does not care about
// order, but a test that reports a failure reads better when the
// flags are listed in the order the user sees them.
func globalFlagNames() []string {
	return []string{
		"--config",
		"--quiet",
		"--verbose",
	}
}

// allRegisteredCommands returns every command in the registry,
// including hidden ones.
//
// The registry is the source of truth for the command tree; the
// function resolves it to a slice of *cobra.Command values, one per
// entry. The commands are constructed with a minimal Dependencies
// value; the tests that use them read only the command's metadata
// (Short, Long, Use, Args, Flags, Hidden), not its behaviour.
//
// # Why hidden commands are included
//
// Hidden commands are still reachable by name (`forge config --help`
// works even when `config` is hidden). The contract applies to them
// the same way it applies to visible commands; excluding them would
// let a hidden command's help drift without a test noticing.
//
// # Why a minimal Dependencies
//
// The command constructors call cobra.Command methods and register
// flags. None of those methods read the Dependencies value's
// fields. The zero value is a valid input for construction; the
// tests that need a fully initialised Dependencies use the runCLI
// helper (testhelper_test.go), which builds one.
//
// # Why the function returns a fresh slice
//
// The function allocates a new slice on each call, so that the
// returned value cannot be mutated by one caller and observed by
// another. The allocation is trivial; the safety is not.
func allRegisteredCommands() []*cobra.Command {
	deps := Dependencies{}
	cmds := make([]*cobra.Command, 0, len(registry))
	for _, ctor := range registry {
		cmds = append(cmds, ctor(deps))
	}
	return cmds
}
