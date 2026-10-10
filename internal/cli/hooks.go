// Package cli implements the Forge command-line interface.
//
// This file defines the PersistentPreRunE hook that runs before every
// subcommand's RunE. The hook is reserved for cross-cutting concerns
// that must initialize uniformly across every command:
//
//   - Configuration loading (WBS 8.0).
//   - Logger initialization (WBS 12.0).
//
// In Phase 2 the hook is a no-op stub. Its existence is the point:
// the hook point is claimed before any subcommand might be tempted to
// use it for its own purposes. A subcommand that installs its own
// PersistentPreRunE would override the root's hook for that
// subcommand's subtree, and the cross-cutting initialization would
// not run. The rule that no subcommand defines its own
// PersistentPreRunE is enforced by a structural test in
// root_test.go and by the Taskfile target verify:hooks.
//
// # Why PersistentPreRunE and not PreRunE
//
// Cobra has two pre-run hooks:
//
//   - PreRunE runs only for the command it is attached to. A
//     subcommand that wants the hook must define it, or inherit it
//     from an ancestor.
//
//   - PersistentPreRunE runs for the command it is attached to and
//     for every descendant. It is the correct hook for concerns
//     that must run for every command, regardless of which command
//     the user invoked.
//
// Configuration loading and logger initialization are concerns of
// the whole CLI, not of individual commands. The persistent hook is
// the only mechanism that runs for every command without requiring
// each command to opt in.
//
// # What the hook will do
//
// When WBS 8.0 lands, the hook will:
//
//  1. Read the --config flag (or its default) to determine the
//     configuration file path.
//  2. Load the configuration file, if it exists.
//  3. Replace deps.Config with the loaded configuration.
//
// When WBS 12.0 lands, the hook will:
//
//  1. Read the --verbose and --quiet flags.
//  2. Resolve the effective log level.
//  3. Construct a Logger at the resolved level.
//  4. Replace deps.Logger with the constructed logger.
//
// The two updates are independent: one can land before the other.
// The hook's structure accommodates both.
//
// # What the hook does not do
//
// The hook does not:
//
//   - Parse flags. Cobra parses them before the hook runs.
//   - Resolve commands. Cobra resolves the command before the hook
//     runs.
//   - Perform I/O. In Phase 2 the hook is a no-op. When WBS 8.0 and
//     WBS 12.0 land, the I/O they perform flows through deps.FS and
//     the loader's interfaces, not through the hook directly.
//   - Write to any stream. The hook has no output.
//
// # Signature
//
// The hook's signature is the one Cobra requires for
// PersistentPreRunE:
//
//	func(cmd *cobra.Command, args []string) error
//
// The signature is fixed by Cobra. The hook returns an error when
// initialization fails; the error propagates through Cobra's normal
// error path, is formatted by formatError, and is mapped to an exit
// code by exitCodeFromError. In Phase 2 the hook never returns a
// non-nil error.
package cli

import (
	"github.com/spf13/cobra"
)

// initializeCommandEnvironment is the root command's
// PersistentPreRunE hook.
//
// It runs before every subcommand's RunE. The hook is reserved for
// the cross-cutting concerns that must initialize uniformly across
// every command: configuration loading and logger initialization.
//
// # Phase 2 behaviour
//
// The hook is a no-op. It returns nil for every invocation.
//
// # Future behaviour
//
// WBS 8.0 will load the configuration file here, using the path
// from the --config flag or the discovery mechanism. The loaded
// configuration will replace deps.Config.
//
// WBS 12.0 will initialize the logger here, using the level resolved
// from the --verbose and --quiet flags. The constructed logger will
// replace deps.Logger.
//
// The two updates are independent and can land in either order. Both
// modify the deps value the hook receives; the hook's structure
// accommodates both without changing its signature.
//
// # How the hook accesses deps
//
// In Phase 2 the hook does not access deps. When WBS 8.0 and WBS
// 12.0 land, the hook will need to read from and write to deps. The
// current design intends for the hook to close over the deps value
// that newRootCmd received. This is an implementation detail that
// the WBS 8.0 and WBS 12.0 changes will settle; the hook's signature
// does not change either way.
//
// # Why the hook is a stub in Phase 2
//
// The hook exists in Phase 2 even though it does nothing, because
// the reservation is the task. Without the hook, a subcommand added
// in WBS 5.x could define its own PersistentPreRunE for a local
// purpose, and the root's hook point would be unavailable when WBS
// 8.0 needed it. Installing the hook now, while the code is small
// and the cost is trivial, prevents that.
//
// The pattern is the same one WBS 5.1.1 applied to the root command's
// identity strings: claim the structure before the code that depends
// on it exists.
//
// # Enforcement
//
// The rule that no subcommand defines its own PersistentPreRunE is
// enforced by TestHooks_NoSubcommandOverrides in root_test.go and by
// the Taskfile target verify:hooks. The two checks are complementary:
// the test runs in Go, the Taskfile target greps the source tree.
func initializeCommandEnvironment(cmd *cobra.Command, args []string) error {
	// Phase 2: no-op stub.
	//
	// WBS 8.0 will load the configuration file here, using the path
	// from the --config flag or the discovery mechanism. The loaded
	// configuration will replace deps.Config.
	//
	// WBS 12.0 will initialize the logger here, using the level
	// resolved from the --verbose and --quiet flags. The constructed
	// logger will replace deps.Logger.
	//
	// The two updates are independent. Neither changes this
	// function's signature.

	_ = cmd
	_ = args
	return nil
}
