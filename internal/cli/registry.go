// Package cli implements the Forge command-line interface.
//
// This file defines the command registry: the single, deterministic
// list of subcommand constructors that newRootCmd iterates to build
// the command tree.
//
// # How to add a command
//
// To add a subcommand to Forge:
//
//  1. Create internal/cli/<name>.go. The file must contain one
//     exported-style (but unexported) constructor with the signature
//
//     func new<Name>Cmd(deps Dependencies) *cobra.Command
//
//     The constructor must not perform I/O, must not read process
//     globals, and must follow the handler / service boundary
//     documented in docs/architecture.md § 11.12.
//
//  2. Add the constructor to the registry slice below. The position
//     in the slice determines the order in which the command appears
//     in `forge --help`.
//
//  3. Add the file to the allowlist in structure_test.go
//     (expectedSourceFiles) and to the test-file package-declaration
//     list (TestTestFilePackageDeclarations) if you added a test file.
//
//  4. Add tests. The reference test file is internal/cli/version_test.go.
//
// That is the entire procedure. If adding a command requires editing
// any file other than <name>.go, <name>_test.go, registry.go, and the
// allowlist in structure_test.go, the abstraction has leaked and the
// contributor should file a WBS item rather than work around it.
//
// # Why a slice and not a map
//
// The registry is an ordered slice because the order of commands in
// `forge --help` is user-visible and must be deterministic. A map
// would produce a different order on every invocation, and the order
// would not be reviewable in a diff. A slice makes the order an
// explicit, greppable, reviewable property of the code.
//
// # Why a named function type and not an interface
//
// commandConstructor is a function type, not an interface. A
// constructor is a pure function: given a Dependencies value, it
// returns a *cobra.Command. There is no state to carry, no
// polymorphic method to dispatch, and no reason to force every
// command to define a struct. A function type is the minimal
// abstraction that captures the contract.
//
// # Why no init() registration
//
// A contributor could register a command by writing
//
//	func init() {
//	    registry = append(registry, newFooCmd)
//	}
//
// in the command's own file. This pattern is forbidden because it
// makes the registry's contents depend on which files happen to be
// compiled in, on package initialization order, and on Go's
// file-processing rules. A reviewer reading registry.go would see an
// incomplete list, and the effective list would only be discoverable
// by running the binary. The registry is a static, reviewable slice
// for exactly this reason.
//
// The test TestRegistry_NoInitRegistration in registry_test.go
// enforces this rule by scanning the package's source for init()
// functions that reference the registry.
package cli

import (
	"github.com/spf13/cobra"
)

// commandConstructor is the function type every Forge command
// implements.
//
// A commandConstructor is a pure function: given a Dependencies
// value, it returns a fully constructed *cobra.Command. The
// constructor does not perform I/O, does not read process globals,
// and does not have side effects beyond allocating the command and
// its subcommands.
//
// # Why the parameter is Dependencies
//
// The constructor receives a Dependencies value rather than an
// options value. Dependencies is the command boundary defined by WBS
// 4.2.2: it holds resolved collaborators (config, logger, filesystem,
// stdout, stderr, env) that the command may use. A constructor that
// accepted options would be free to read raw os.* values, which
// violates the two-boundary model. The Dependencies parameter is the
// mechanism by which the invariant is enforced.
//
// # Why the return type is *cobra.Command
//
// The constructor returns a *cobra.Command because that is what
// Cobra's AddCommand method accepts. Every Forge command is a Cobra
// command; the registry does not abstract over CLI frameworks. If a
// future version of Forge replaced Cobra, the commandConstructor type
// would change, but the surrounding structure (an ordered slice
// iterated by newRootCmd) would survive.
type commandConstructor func(Dependencies) *cobra.Command

// registry is the ordered list of command constructors.
//
// # Ordering
//
// The slice's order determines the order in which commands appear in
// `forge --help`. The order is user-visible and must be deliberate.
// The conventions are:
//
//   - User-facing commands come before administrative commands.
//   - Within each group, commands are alphabetical by name.
//   - Hidden commands (commands with `Hidden: true` in their Cobra
//     definition) may appear anywhere; their position does not affect
//     help output.
//
// When adding a command, place it in the correct position for the
// ordering convention. A reviewer will check.
//
// # Why the registry is unexported
//
// The registry is an implementation detail of newRootCmd. No package
// outside internal/cli needs to iterate it. Exporting it would invite
// downstream code to depend on the list of commands, which would
// freeze the command set in place and make adding a command a
// breaking change. The registry is private for the same reason
// newRootCmd is private.
//
// # Adding a command
//
// See the package docstring at the top of this file for the four-step
// procedure. In short: create <name>.go, append new<Name>Cmd to the
// slice below, update the allowlist in structure_test.go, and add
// tests.
//
// # Test coverage
//
// registry_test.go verifies:
//
//   - No duplicate command names appear in the registry.
//   - No init() function in the package appends to the registry.
//   - The registry's order matches the order Cobra reports in help
//     output, for the visible commands.
//   - Every constructor accepts a Dependencies value and returns a
//     non-nil *cobra.Command.
//   - The registry contains at least one command (a sanity check that
//     the slice has not been accidentally emptied).
//
// These invariants are what make the registry reviewable. Without
// them, a contributor could add a command in the wrong order, or
// register the same command twice under different names, and the
// mistake would only be visible in the help output.
var registry = []commandConstructor{
	// ─────────────────────────────────────────────────────────────────
	// User-facing commands.
	//
	// These commands implement the primary use cases of Forge. They
	// appear in `forge --help` in the order listed below. The
	// convention within this group is alphabetical by command name.
	// ─────────────────────────────────────────────────────────────────

	// config — hidden placeholder for the configuration command.
	// The real implementation is WBS 8.x. The placeholder exists so
	// that the registry has two entries and the ordering convention
	// is visible in code.
	newConfigCmd,

	// version — the reference implementation of the handler / service
	// boundary introduced by WBS 4.3.1. See version.go.
	newVersionCmd,

	// ─────────────────────────────────────────────────────────────────
	// WBS 5.x will add: newNewCmd, newInitCmd, newValidateCmd
	// WBS 6.x will add: newCheckCmd, newDiffCmd
	// WBS 7.x will add: newUpdateCmd, newExplainCmd
	//
	// When adding a command, place it in the correct alphabetical
	// position within the user-facing group. Do not append to the
	// end unless the command belongs at the end.
	// ─────────────────────────────────────────────────────────────────
}
