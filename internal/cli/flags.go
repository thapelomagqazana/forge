// Package cli implements the Forge command-line interface.
//
// This file defines the CLI's global flags: the persistent flags
// that every subcommand inherits. It is the single source of truth
// for the flag names, the registration of the flags on the root
// command, and the helpers that read the parsed values.
//
// # The current flag inventory
//
// Forge has four global flags:
//
//   - --verbose: set the log level to DEBUG.
//   - --quiet:   set the log level to ERROR (errors only).
//   - --config:  path to the configuration file, overriding
//     discovery.
//   - --format:  output format for commands that render output
//     (text or json; default text).
//
// Each flag has a documented consumer in a specific WBS item:
//
//   - --verbose is consumed by WBS 12.0 (logging).
//   - --quiet is consumed by WBS 12.0 (logging).
//   - --config is consumed by WBS 8.0 (configuration loading).
//   - --format is consumed by WBS 9.2.1 and WBS 9.4.1 (config
//     command output), and by every future command that renders
//     JSON.
//
// No flag is added speculatively. A future contributor who wants a
// new global flag must first write an ADR explaining which consumer
// needs it and what its semantics are. This rule is enforced by
// TestGlobalFlags_InventoryIsExhaustive and by the Taskfile target
// verify:global-flags.
//
// # History
//
// Phase 2 registered exactly three flags (--verbose, --quiet,
// --config). WBS 9.4.1 (JSON output for `forge config`) added
// --format as the fourth. The addition is documented by an ADR
// (docs/decisions/ADR-010-add-format-global-flag.md), which
// supersedes the "exactly three" clause of the Phase 2 inventory.
// The rule the original docstring stated — "a new flag requires an
// ADR" — is unchanged; only the count is.
//
// # Persistence
//
// All four flags are registered as *persistent* flags on the root
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
//
// The --format flag has no precedence interaction with the other
// three: it selects a rendering, not a level. A user who sets
// --quiet and --format json gets a JSON rendering with the
// logger's verbosity lowered; the two flags compose without
// conflict.
package cli

import (
	"fmt"

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
//
// # Inventory
//
// The inventory is four flags. Adding a fifth requires an ADR; see
// the package docstring.
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

	// FlagFormat is the name of the --format flag. The flag is a
	// string with default FlagFormatText. Its value set is
	// {FlagFormatText, FlagFormatJSON}, frozen by WBS 6.4.1 and
	// consumed by every command that renders output (starting
	// with `forge config`, WBS 9.2.1 and WBS 9.4.1).
	//
	// # Why a global flag
	//
	// The format applies to any command that produces output. It
	// is not a config-command-specific concern. Registering it
	// once on the root, as a persistent flag, means every command
	// that wants to render JSON reads the same flag with the
	// same semantics. A per-command flag would produce a
	// proliferation of --format declarations, each with its own
	// default and value set, and the read sites would diverge.
	//
	// # Why the default is FlagFormatText
	//
	// "text" is the human-readable default. "json" is opt-in. A
	// user who pipes output to a JSON consumer passes --format
	// json; a user reading the output in a terminal does nothing
	// and sees text. The default preserves the pre-WBS-9.4.1
	// behaviour for every existing invocation.
	FlagFormat = "format"
)

// FlagFormatText and FlagFormatJSON are the frozen values of the
// --format flag, per WBS 6.4.1.
//
// The constants are declared here (not in config.go) because the
// flag's value set is a global concern: any command that reads
// --format accepts exactly these two values. A per-command
// declaration would risk drift between commands.
//
// # Why these are string constants and not an enum type
//
// The values are strings because Cobra's String flag stores
// strings. An enum type would require conversion at every read
// site. The constants give the strings names, which is the
// benefit an enum would provide, without the conversion cost.
//
// # Where the values are validated
//
// This file defines the values; it does not validate them. Each
// command that reads --format performs its own validation and
// produces its own error for an unsupported value. The config
// command's validation lives in config.go (see writeConfigPath and
// the --path mode's dispatch in runConfig). A future command that
// reads --format does the same.
//
// A shared validator was considered and rejected: it would couple
// every command to a single error message, which would make
// command-specific diagnostics (the config command's, for
// instance) impossible to vary. The cost of duplicating the
// validation is one switch statement per command; the benefit is
// each command's ability to phrase its own error.
const (
	// FlagFormatText is the text rendering. It is the default.
	FlagFormatText = "text"

	// FlagFormatJSON is the JSON rendering.
	FlagFormatJSON = "json"
)

// registerGlobalFlags registers the four global flags on the given
// command as persistent flags. Persistent flags are inherited by
// every subcommand.
//
// # What the function registers
//
//   - --verbose: a bool flag, default false.
//   - --quiet:   a bool flag, default false.
//   - --config:  a string flag, default "".
//   - --format:  a string flag, default FlagFormatText.
//
// The short forms are deliberately not registered. --verbose and
// --quiet have no short forms because the single-letter aliases
// are reserved for other purposes (in particular, -v is reserved
// for --version, see WBS 5.2.3). --config and --format have no
// short forms because none of the conventional letters (c, f) is
// unambiguously associated with either flag in the presence of the
// other.
//
// # Why a function and not inline calls in newRootCmd
//
// The flags are a single unit: they are registered together, they
// are documented together, and they are tested together. Keeping
// them in one function makes the unit visible: a reader of
// newRootCmd sees a single call to registerGlobalFlags and knows
// that "all global flags are registered here". A future
// contributor who wants to add a flag edits this file, not
// newRootCmd.
//
// # Why the function takes *cobra.Command
//
// The function registers the flags on a *cobra.Command. It does
// not construct the command. Passing the command rather than
// returning one keeps the function usable with the root command
// and with any future command that needs the same flags (for
// example, a future test that constructs a subcommand in
// isolation).
func registerGlobalFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().Bool(FlagVerbose, false,
		"enable verbose output (log level DEBUG)")

	cmd.PersistentFlags().Bool(FlagQuiet, false,
		"suppress all output except errors (log level ERROR)")

	cmd.PersistentFlags().String(FlagConfig, "",
		"path to the configuration file (overrides discovery)")

	// WBS 9.4.1: the --format flag selects the output rendering.
	// The value set is {text, json}; the default is text.
	cmd.PersistentFlags().String(FlagFormat, FlagFormatText,
		"output format: text|json")
}

// verboseRequested reports whether the --verbose flag was set on
// the invocation.
//
// The flag is a persistent flag registered by registerGlobalFlags
// (WBS 5.3.1). It is inherited by every subcommand, so reading it
// from the root command returns the value the user passed
// regardless of where the flag appeared on the command line.
//
// # Why the helper exists
//
// The flag's value is read from a *cobra.Command, not from the
// options struct or from a global. The helper centralises the
// read so that a future change to the flag's name or type is a
// change in one place.
//
// # The nil-command case
//
// A nil cmd is a programming error. The helper panics rather than
// returning a default, because a caller that passes nil has a
// bug that a default would hide. The signature is unchanged from
// the pre-WBS-10.4.4 helper.
func verboseRequested(cmd *cobra.Command) bool {
	if cmd == nil {
		panic("cli: verboseRequested called with nil command")
	}
	v, err := cmd.Flags().GetBool(FlagVerbose)
	if err != nil {
		// The flag was not registered. This is a programming
		// error: registerGlobalFlags is called from newRootCmd,
		// and every command in the tree inherits the flag.
		panic(fmt.Sprintf("cli: --%s flag not registered: %v", FlagVerbose, err))
	}
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

// formatValue returns the value of --format, or FlagFormatText if
// the flag was not set or its value is empty.
//
// # Why the empty-string case is handled
//
// The flag's default is FlagFormatText, so a command on which
// registerGlobalFlags has run always returns a non-empty value.
// The empty case arises only when the flag is not present on the
// command's flag set at all — for example, when a command is
// constructed in isolation, without its parent, and the test
// expects the reader to be robust to that situation.
//
// The function returns FlagFormatText in the empty case, matching
// the flag's declared default. A caller that reads the value and
// dispatches on it produces the same behaviour as if the flag had
// been explicitly set to text.
//
// # Error handling
//
// See verboseRequested for the rationale of the ignored error. The
// flag is guaranteed to exist when the registration has run; the
// reader tolerates its absence for the isolated-command case.
func formatValue(cmd *cobra.Command) string {
	s, err := cmd.Flags().GetString(FlagFormat)
	if err != nil || s == "" {
		return FlagFormatText
	}
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
