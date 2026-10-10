// Package cli implements the Forge command-line interface.
//
// This file defines the CLI's global flags: the three persistent
// flags that every subcommand inherits. It is the single source of
// truth for the flag names, the registration of the flags on the
// root command, and the helpers that read the parsed values.
//
// # The Phase 2 flag inventory
//
// Forge has exactly three global flags in Phase 2:
//
//   - --verbose: set the log level to DEBUG.
//   - --quiet:   set the log level to ERROR (errors only).
//   - --config:  path to the configuration file, overriding
//     discovery.
//
// Each flag has a documented consumer in a later WBS item:
//
//   - --verbose is consumed by WBS 12.0 (logging).
//   - --quiet is consumed by WBS 12.0 (logging).
//   - --config is consumed by WBS 8.0 (configuration loading).
//
// No flag is added speculatively. A future contributor who wants a
// new global flag must first write an ADR explaining which consumer
// needs it and what its semantics are. This rule is enforced by
// TestGlobalFlags_InventoryIsExhaustive and by the Taskfile target
// verify:global-flags.
//
// # Persistence
//
// All three flags are registered as *persistent* flags on the root
// command. Cobra's persistent flags are inherited by every
// subcommand, so both `forge --verbose config` and `forge config
// --verbose` are equivalent. The persistence is the reason the
// flags appear in the root command's help output under "Global
// Flags" rather than "Flags".
//
// # Precedence
//
// When both --verbose and --quiet are set, --quiet wins. The
// rationale is that quiet is the stricter contract: the user who
// asked for quiet asked for a smaller output surface, and honoring
// a smaller surface when a larger one is also requested is the
// correct default. The precedence is implemented in
// resolveLogLevel and is documented in docs/cli-ux-spec.md § 4.12.
package cli

import (
	"github.com/spf13/cobra"
)

// Flag-name constants.
//
// The constants are used everywhere a flag name is needed: in the
// registration call below, in the helpers that read the parsed
// values, in the tests that assert on the flags' presence and
// behaviour. Using constants rather than string literals ensures
// that a rename is a single-place edit and that a typo is a
// compile error rather than a silent no-op.
const (
	// FlagVerbose is the name of the --verbose flag. The flag is
	// a bool with default false.
	FlagVerbose = "verbose"

	// FlagQuiet is the name of the --quiet flag. The flag is a
	// bool with default false.
	FlagQuiet = "quiet"

	// FlagConfig is the name of the --config flag. The flag is a
	// string with default "" (meaning "discover the file").
	FlagConfig = "config"
)

// registerGlobalFlags registers the three global flags on the given
// command as persistent flags. Persistent flags are inherited by
// every subcommand.
//
// # What the function registers
//
//   - --verbose: a bool flag, default false.
//   - --quiet:   a bool flag, default false.
//   - --config:  a string flag, default "".
//
// The short forms are deliberately not registered. --verbose and
// --quiet have no short forms because the single-letter aliases are
// reserved for other purposes (in particular, -v is reserved for
// --version, see WBS 5.2.3). --config has no short form because
// none of the conventional letters (c, f) is unambiguously
// associated with "config" in the presence of the other global
// flags.
//
// # Why a function and not inline calls in newRootCmd
//
// The three flags are a single unit: they are registered together,
// they are documented together, and they are tested together.
// Keeping them in one function makes the unit visible: a reader of
// newRootCmd sees a single call to registerGlobalFlags and knows
// that "all global flags are registered here". A future contributor
// who wants to add a flag edits this file, not newRootCmd.
//
// # Why the function takes *cobra.Command
//
// The function registers the flags on a *cobra.Command. It does not
// construct the command. Passing the command rather than returning
// one keeps the function usable with the root command and with any
// future command that needs the same flags (for example, a future
// test that constructs a subcommand in isolation).
func registerGlobalFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().Bool(FlagVerbose, false,
		"enable verbose output (log level DEBUG)")

	cmd.PersistentFlags().Bool(FlagQuiet, false,
		"suppress all output except errors (log level ERROR)")

	cmd.PersistentFlags().String(FlagConfig, "",
		"path to the configuration file (overrides discovery)")
}

// verboseRequested reports whether --verbose was set on the command
// or on any of its ancestors.
//
// # Reading the flag
//
// The function reads the flag from the command's persistent flags.
// A persistent flag set on the root is visible to every subcommand;
// a persistent flag set on a subcommand is visible to that
// subcommand and to its descendants. The function reads from the
// command it receives, which is the command the invocation is
// running.
//
// # Error handling
//
// The function ignores the error returned by GetBool. The flag is
// guaranteed to exist because registerGlobalFlags registered it,
// and the call sites are always commands on which the registration
// has run. A future refactor that removes the registration would
// break the call sites at compile time only if they were rewritten
// to depend on the flag's presence; the current form would silently
// return false. The trade-off is accepted because the registration
// is centralized and reviewed.
func verboseRequested(cmd *cobra.Command) bool {
	v, _ := cmd.Flags().GetBool(FlagVerbose)
	return v
}

// quietRequested reports whether --quiet was set on the command or
// on any of its ancestors.
//
// See verboseRequested for the rationale of the ignored error.
func quietRequested(cmd *cobra.Command) bool {
	v, _ := cmd.Flags().GetBool(FlagQuiet)
	return v
}

// configPath returns the value of --config, or the empty string if
// the flag was not set.
//
// The empty string is the "discover the file" sentinel. A consumer
// that receives the empty string treats it as "the flag was not
// given"; a consumer that receives a non-empty string treats it as
// "the user specified this path, use it instead of discovery".
func configPath(cmd *cobra.Command) string {
	s, _ := cmd.Flags().GetString(FlagConfig)
	return s
}

// resolveLogLevel returns the effective log level implied by the
// --verbose and --quiet flags on the given command.
//
// # The precedence rule
//
// When both flags are set, --quiet wins. The rationale is that
// quiet is the stricter contract: the user who asked for quiet
// asked for a smaller output surface, and honoring a smaller
// surface when a larger one is also requested is the correct
// default.
//
// # The return value
//
// The function returns one of three string values:
//
//   - "debug" when --verbose is set and --quiet is not.
//   - "error" when --quiet is set (regardless of --verbose).
//   - "info" when neither flag is set.
//
// The values are the log levels defined by WBS 12.0. The consumer
// (the logger constructor) maps them to a slog.Level. The mapping
// is kept out of this function so that the flag semantics can be
// tested without depending on the logging package.
//
// # Why the function is on the command and not on the flags alone
//
// The resolution needs to see the effective values of the flags
// after parsing, including flags set on parent commands. Reading
// from the *cobra.Command is the way to see them. A function that
// took two bools would not know which command the flags came from
// and would have to be called with the results of the two readers,
// which is the same information with less context.
func resolveLogLevel(cmd *cobra.Command) string {
	if quietRequested(cmd) {
		return "error"
	}
	if verboseRequested(cmd) {
		return "debug"
	}
	return "info"
}
