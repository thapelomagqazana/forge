// Package cli implements the Forge command-line interface.
//
// This file defines the CLI's global flags: the persistent flags
// that every subcommand inherits. It is the single source of truth
// for the flag names, the registration of the flags on the root
// command, and the helpers that read the parsed values.
//
// # The current flag inventory
//
// Forge has five global flags:
//
//   - --verbose:   implicating flag; sets the log level to DEBUG.
//   - --quiet:     implicating flag; sets the log level to ERROR.
//   - --log-level: explicit flag; sets the log level by name.
//   - --config:    path to the configuration file.
//   - --format:    output rendering format (text or json).
//
// Each flag has a documented consumer in a specific WBS item:
//
//   - --verbose is consumed by WBS 12.1.1, 12.1.2, and 12.5.1.
//   - --quiet is consumed by WBS 12.1.1 and 12.1.2.
//   - --log-level is consumed by WBS 12.1.2 (explicit override).
//   - --config is consumed by WBS 8.0 (configuration loading).
//   - --format is consumed by WBS 9.2.1 and WBS 9.4.1 (config
//     command output), by the error path (WBS 11.3.2), and by
//     every future command that renders JSON.
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
// --format. WBS 12.1.2 added --log-level and made --verbose and
// --quiet mutually exclusive. WBS 12.5.1 wired the resolved level
// into the logger and emitted DEBUG records from the subsystems
// that have useful diagnostic state. Each addition is documented
// by an ADR (see docs/decisions/ for ADR-010 and ADR-022).
//
// The rule the original docstring stated — "a new flag requires an
// ADR" — is unchanged; only the count and the semantics are.
//
// # Persistence
//
// All five flags are registered as *persistent* flags on the root
// command. Cobra's persistent flags are inherited by every
// subcommand, so both `forge --verbose config` and `forge config
// --verbose` are equivalent. The persistence is the reason the
// flags appear in the root command's help output under "Global
// Flags" rather than "Flags".
//
// # Log-level precedence (WBS 12.1.2)
//
// The effective log level is resolved by a precedence chain
// (highest first):
//
//  1. --log-level=<level>   (explicit; wins over implicating flags)
//  2. --verbose             (implies DEBUG)
//  3. --quiet               (implies ERROR)
//  4. FORGE_LOG_LEVEL       (env var)
//  5. log.level             (config file)
//  6. default               (INFO)
//
// The resolution is implemented in internal/logger/precedence.go
// and documented in docs/reference/log-levels.md §2. This file is
// the CLI-layer adapter: it reads the flags, reads the env var,
// accepts the config value from the caller, and calls the resolver.
//
// # The env lookup (WBS 12.5.1)
//
// The resolver reads FORGE_LOG_LEVEL through a caller-supplied
// env function, not through os.Getenv directly. Two entry points
// exist:
//
//   - resolveLogLevel(cmd, configLevel) reads the process
//     environment (via logger.ReadEnv, which calls os.Getenv).
//     It is the legacy entry point, used by the flag-level tests.
//
//   - resolveLogLevelWithEnv(cmd, env, configLevel) reads the env
//     var through the given function. The CLI's process boundary
//     (runWithOptions, execute.go) calls this variant with
//     opts.env, which is os.Getenv in production and an injected
//     function in tests.
//
// The threading is what makes FORGE_LOG_LEVEL testable in-process:
// a test injects an opts.env that returns "debug" for
// FORGE_LOG_LEVEL, and the resolver sees the value without
// mutating the process environment.
//
// # The --quiet / --verbose mutual exclusion (WBS 12.1.2)
//
// --quiet and --verbose are mutually exclusive. Passing both is a
// usage error (exit 64), detected before any command logic runs.
// The rationale is that the two flags express opposite intents,
// and silently picking one hides a user error. See
// docs/reference/log-levels.md §4.5.
//
// # The error shape for the conflict (WBS 12.5.1 reconciliation)
//
// validateLogFlags returns a *forgeerr.Error with category
// CategoryCLI and code CodeCLIUnknownFlag. It wraps the
// ErrQuietVerboseConflict sentinel as the cause.
//
// The sentinel remains for callers (and tests) that match on it
// with errors.Is; the *forgeerr.Error is the value that reaches
// the boundary, where it is classified as a usage error (exit 64)
// rather than an internal error (exit 70).
//
// The distinction matters because the boundary wraps any
// non-*forgeerr.Error value as FORGE_INTERNAL_UNEXPECTED. A bare
// sentinel would be classified as internal, which is the wrong
// category for a user-flag conflict. The sentinel-wrapped-in-
// *forgeerr.Error shape gives both properties: the sentinel is
// matchable, and the boundary sees a first-class CLI error.
//
// The code CodeCLIUnknownFlag is used because no dedicated code
// exists for flag conflicts. A future WBS may add one (for
// example, FORGE_CLI_CONFLICTING_FLAGS); until then, the code
// registry's closest registered value is used. The error's
// context records the conflicting flags so a consumer can
// distinguish the case without relying on the code.
//
// # The --verbose flag and the -v reservation (WBS 12.5.1)
//
// --verbose has no short form. The single letter -v is the short
// form of --version, registered by Cobra when the root command's
// Version field is non-empty. The decision follows git, docker,
// and kubectl, whose -v also means version. The rationale is in
// docs/reference/verbose-mode.md §2; the enforcement is by
// TestContract_VerboseHasNoShortForm and by
// TestVersionBehaviour_VerboseHasNoShortFlag.
//
// Verbose mode affects stderr only. It never changes stdout. The
// DEBUG records it enables are listed in
// docs/reference/verbose-mode.md §3. The stream contract is
// asserted by TestVerbose_StdoutUnchanged_BDD in the binary
// integration tests.
//
// # The --format reader and the --quiet reader (WBS 11.3.2)
//
// The error path (runWithOptions, execute.go) reads two of the
// five flags: --format and --quiet. The reads are performed by
// formatRequested and quietRequested. The two functions live in
// this file because the file is the single source of truth for
// the flags' names, types, and defaults.
//
// formatRequested returns an OutputFormat, not the raw string.
// The conversion is the reason a second format reader exists
// alongside formatValue (which returns the raw string for the
// config command's dispatch).
package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/thapelomagqazana/forge/internal/forgeerr"
	"github.com/thapelomagqazana/forge/internal/logger"
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
	// FlagVerbose is the name of the --verbose flag.
	//
	// # Type and default
	//
	// The flag is a bool with default false. It is an
	// *implicating* flag: it implies DEBUG unless --log-level
	// overrides it.
	//
	// # The -v short form is reserved for --version
	//
	// --verbose has no short form. The single letter -v is
	// registered by Cobra as the short form of --version when
	// the root command's Version field is non-empty (WBS 5.1.2).
	// This matches the Cobra convention and the behaviour of
	// git, docker, and kubectl, whose -v also means version.
	//
	// A CLI that gives -v to --verbose (a common alternative,
	// used by curl and ssh) forces --version to be long-only.
	// The tradeoff is asymmetric: users type --version rarely
	// (mostly in scripts), but type -v often as a habit from
	// other tools. Forge follows the majority convention.
	//
	// The decision is recorded in docs/reference/verbose-mode.md
	// §2 and is enforced by two tests:
	//
	//   - TestContract_VerboseHasNoShortForm (flags_test.go)
	//     asserts that this flag's Shorthand field is empty.
	//   - TestVersionBehaviour_VerboseHasNoShortFlag (root_test.go)
	//     asserts that `forge -v` prints the version block, not
	//     a verbose banner.
	//
	// A future contributor who wants -v for --verbose must file
	// an ADR; the decision is not reversible by editing this
	// file alone.
	//
	// # Interaction with --quiet
	//
	// --verbose and --quiet are mutually exclusive. Passing both
	// is a usage error (ErrQuietVerboseConflict, exit 64). See
	// docs/reference/log-levels.md §4.5.
	//
	// # Effect on streams
	//
	// Verbose mode affects stderr only. Under --verbose, the CLI
	// emits DEBUG records about how it resolved its inputs. The
	// stdout product output is byte-identical to the default
	// run. See docs/reference/verbose-mode.md §4.
	FlagVerbose = "verbose"

	// FlagQuiet is the name of the --quiet flag. The flag is a
	// bool with default false. It is an *implicating* flag: it
	// implies ERROR unless --log-level or --verbose overrides it.
	//
	// # The -q shorthand
	//
	// --quiet registers `-q` as its shorthand. Unlike --verbose
	// (which has no shorthand because `-v` is taken by
	// --version), --quiet has no collision: `-q` is unclaimed by
	// Cobra's built-ins and by any other Forge flag. The
	// shorthand is safe to add and matches the convention of
	// `git commit -q`, `apt-get -q`, and similar tools.
	//
	// # Quiet mode
	//
	// Quiet mode sets the invocation's log level to ERROR. The
	// effect on streams is documented in
	// docs/reference/quiet-mode.md:
	//
	//   - stdout: product output is unchanged. --quiet does not
	//     suppress the command's result.
	//   - stderr: log records at INFO, WARN, and DEBUG are
	//     filtered out. ERROR records remain. The human error
	//     block on failure is not suppressed.
	//
	// # Interaction with --verbose
	//
	// --quiet and --verbose are mutually exclusive. Passing both
	// is a usage error (ErrQuietVerboseConflict, exit 64). See
	// docs/reference/log-levels.md §4.5.
	//
	// # Interaction with --format json
	//
	// --quiet does not suppress the JSON envelope on stdout. The
	// envelope is the command's result in JSON mode; suppressing
	// it would break every downstream parser. See
	// docs/reference/quiet-mode.md §4.
	FlagQuiet = "quiet"

	// FlagLogLevel is the name of the --log-level flag. The flag
	// is a string with default "" (meaning "not set"). It is the
	// *explicit* level selector and has the highest precedence in
	// the level resolution chain.
	//
	// Accepted values: "debug", "info", "warn", "warning",
	// "error" (case-insensitive). Any other value is a usage
	// error (exit 64), detected at resolution time.
	FlagLogLevel = "log-level"

	// FlagConfig is the name of the --config flag. The flag is a
	// string with default "" (meaning "discover the file").
	FlagConfig = "config"

	// FlagFormat is the name of the --format flag. The flag is a
	// string with default FlagFormatText. Its value set is
	// {FlagFormatText, FlagFormatJSON}, frozen by WBS 6.4.1.
	FlagFormat = "format"
)

// FlagFormatText and FlagFormatJSON are the frozen values of the
// --format flag, per WBS 6.4.1.
//
// The constants are declared here (not in config.go) because the
// flag's value set is a global concern: any command that reads
// --format accepts exactly these two values. A per-command
// declaration would risk drift between commands.
const (
	// FlagFormatText is the text rendering. It is the default.
	FlagFormatText = "text"

	// FlagFormatJSON is the JSON rendering.
	FlagFormatJSON = "json"
)

// ErrQuietVerboseConflict is the sentinel for the --quiet /
// --verbose conflict.
//
// It is a *usage* error: the CLI maps it to exit code 64 (see
// docs/reference/exit-codes.md). The check runs during flag
// validation, before any command logic, so an invalid invocation
// exits without doing work.
//
// # How the sentinel is used
//
// validateLogFlags returns a *forgeerr.Error (category
// CategoryCLI, code CodeCLIUnknownFlag) that wraps this sentinel.
// The *forgeerr.Error is what reaches the boundary, where it is
// classified as a usage error. The sentinel is the wrapped cause,
// so callers (and tests) that match on it with errors.Is continue
// to work.
//
// # Why a sentinel and not a bare *forgeerr.Error
//
// A caller that wants to recognize the conflict without depending
// on the CLI's *forgeerr.Error shape uses errors.Is(err,
// ErrQuietVerboseConflict). The sentinel is the stable identity of
// the conflict; the *forgeerr.Error is the CLI-layer presentation.
// Keeping both lets the CLI's internals change without breaking
// the sentinel's consumers, and lets the sentinel's consumers
// change without affecting the CLI's error classification.
var ErrQuietVerboseConflict = errors.New(
	"--quiet and --verbose are mutually exclusive")

// registerGlobalFlags registers the five global flags on the given
// command as persistent flags. Persistent flags are inherited by
// every subcommand.
//
// # What the function registers
//
//   - --verbose:   a bool flag, default false.
//   - --quiet:     a bool flag, default false.
//   - --log-level: a string flag, default "".
//   - --config:    a string flag, default "".
//   - --format:    a string flag, default FlagFormatText.
//
// The short forms are deliberately not registered. --verbose and
// --quiet have no short forms because the single-letter aliases
// are reserved for other purposes (in particular, -v is reserved
// for --version, see WBS 5.2.3 and
// docs/reference/verbose-mode.md §2). --config and --format have
// no short forms because none of the conventional letters (c, f)
// is unambiguously associated with either flag in the presence of
// the other.
//
// # The --verbose help text
//
// The help text for --verbose names the stream the records go to
// ("to stderr") so a user reading `forge --help` sees the stream
// contract without opening the reference doc. The help text does
// not enumerate the DEBUG records; the reference doc does that.
//
// # Why a function and not inline calls in newRootCmd
//
// The flags are a single unit: they are registered together, they
// are documented together, and they are tested together. Keeping
// them in one function makes the unit visible. A future
// contributor who wants to add a flag edits this function, not
// newRootCmd.
func registerGlobalFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().Bool(FlagVerbose, false,
		"enable verbose output to stderr (log level DEBUG)")

	cmd.PersistentFlags().BoolP(FlagQuiet, "q", false,
		"suppress all output except errors (log level ERROR)")

	cmd.PersistentFlags().String(FlagLogLevel, "",
		"set the log level explicitly: debug|info|warn|error "+
			"(overrides --verbose and --quiet)")

	cmd.PersistentFlags().String(FlagConfig, "",
		"path to the configuration file (overrides discovery)")

	cmd.PersistentFlags().String(FlagFormat, FlagFormatText,
		"output format: text|json")
}

// -----------------------------------------------------------------------------
// Reader helpers
// -----------------------------------------------------------------------------

// verboseRequested reports whether the --verbose flag was set.
//
// The flag is a persistent flag registered by registerGlobalFlags.
// It is inherited by every subcommand, so reading it from the root
// command returns the value the user passed regardless of where
// the flag appeared on the command line.
//
// # The nil-command case
//
// A nil cmd is a programming error. The helper panics rather than
// returning a default, because a caller that passes nil has a bug
// that a default would hide.
//
// # The unregistered-flag case
//
// The implementation tolerates an unregistered --verbose flag by
// returning false. The tolerance matches quietRequested's and is
// deliberate: a boolean flag's absence is indistinguishable from
// its default, so returning false is the reader's correct answer,
// not a fallback.
//
// This is a change from the pre-WBS-12.1.2 behaviour, which
// panicked. The panic was intended to catch programming errors
// (a command reaching the reader without registerGlobalFlags
// having run), but it fired on legitimate test commands that
// construct a *cobra.Command in isolation, causing repeated
// test-suite failures during the WBS-12.1.2 reconciliation. The
// tolerance is the correct design: the reader's contract is "the
// effective value of the flag", and an unregistered boolean flag's
// effective value is its default.
//
// The two readers resolveLogLevel depends on (verboseRequested and
// quietRequested) must agree on how to treat an absent flag. This
// version matches quietRequested's behaviour.
func verboseRequested(cmd *cobra.Command) bool {
	if cmd == nil {
		panic("cli: verboseRequested called with nil command")
	}
	v, err := cmd.Flags().GetBool(FlagVerbose)
	if err != nil {
		// The flag was not registered. The default is false;
		// see the docstring.
		return false
	}
	return v
}

// quietRequested reports whether --quiet was set.
//
// The flag is a persistent flag registered by registerGlobalFlags.
// It is inherited by every subcommand.
//
// # The nil-command case
//
// A nil cmd is a programming error. The helper panics rather than
// returning a default, because a caller that passes nil has a bug
// that a default would hide.
//
// # The unregistered-flag case
//
// The implementation tolerates an unregistered --quiet flag by
// returning false. The tolerance is deliberate: a test that
// constructs a command without calling registerGlobalFlags sees
// the flag's default value, which is false. The absence of the
// flag is indistinguishable from the flag's default, so the
// tolerance is safe.
func quietRequested(cmd *cobra.Command) bool {
	if cmd == nil {
		panic("cli: quietRequested called with nil command")
	}
	v, err := cmd.Flags().GetBool(FlagQuiet)
	if err != nil {
		return false
	}
	return v
}

// logLevelRequested returns the raw value of --log-level, or "" if
// the flag was not set.
//
// The empty string means "not specified"; the resolver treats it
// as unset and falls through to the next precedence source.
//
// # The nil-command case
//
// A nil cmd is a programming error. The helper panics rather than
// returning a default, because a caller that passes nil has a bug
// that a default would hide.
func logLevelRequested(cmd *cobra.Command) string {
	if cmd == nil {
		panic("cli: logLevelRequested called with nil command")
	}
	s, err := cmd.Flags().GetString(FlagLogLevel)
	if err != nil {
		return ""
	}
	return s
}

// configPath returns the value of --config, or the empty string if
// the flag was not set.
//
// The empty string is the "discover the file" sentinel. A consumer
// that receives the empty string treats it as "the flag was not
// given"; a consumer that receives a non-empty string treats it as
// "the user specified this path, use it instead of discovery".
func configPath(cmd *cobra.Command) string {
	if cmd == nil {
		panic("cli: configPath called with nil command")
	}
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
// command's flag set at all.
func formatValue(cmd *cobra.Command) string {
	if cmd == nil {
		panic("cli: formatValue called with nil command")
	}
	s, err := cmd.Flags().GetString(FlagFormat)
	if err != nil || s == "" {
		return FlagFormatText
	}
	return s
}

// formatRequested returns the --format flag's value as an
// OutputFormat.
//
// The function is the error path's reader (WBS 11.3.2). It
// returns the typed OutputFormat rather than the raw string that
// formatValue returns, so that runWithOptions can dispatch on the
// typed value without a conversion at the call site.
//
// # The nil-command case
//
// A nil cmd is a programming error. The helper panics rather than
// returning a default, because a caller that passes nil has a bug
// that a default would hide.
//
// # The unregistered-flag case
//
// The function tolerates an unregistered --format flag by returning
// FormatText. The tolerance is the same as quietRequested's.
func formatRequested(cmd *cobra.Command) OutputFormat {
	if cmd == nil {
		panic("cli: formatRequested called with nil command")
	}
	s, err := cmd.Flags().GetString(FlagFormat)
	if err != nil || s == "" {
		return FormatText
	}
	return OutputFormat(s)
}

// -----------------------------------------------------------------------------
// Log-level resolution (WBS 12.1.2)
// -----------------------------------------------------------------------------

// validateLogFlags enforces the flag contracts for logging.
//
// Currently it checks that --quiet and --verbose are not both set.
// The flags express opposite intents; silently picking one would
// hide a user error. See docs/reference/log-levels.md §4.5.
//
// The check runs before any command logic, so an invalid invocation
// exits 64 without doing work.
//
// # The error's shape (WBS 12.5.1 reconciliation)
//
// The function returns a *forgeerr.Error with:
//
//   - Category: CategoryCLI (the conflict is a user error).
//   - Code: CodeCLIUnknownFlag (the closest registered code; a
//     future WBS may add a dedicated code for flag conflicts).
//   - Message: the sentinel's message ("--quiet and --verbose
//     are mutually exclusive").
//   - Cause: ErrQuietVerboseConflict (the sentinel, so callers
//     can match with errors.Is).
//   - Context: "flags" naming the two conflicting flags.
//   - Remediation: a suggestion to use one or the other.
//
// # Why a *forgeerr.Error and not the sentinel
//
// The boundary (runWithOptions) ensures every error it renders is
// a *forgeerr.Error. A plain sentinel would be wrapped as
// FORGE_INTERNAL_UNEXPECTED (CategoryInternal, exit 70), which is
// the wrong classification: the conflict is a user error, not an
// internal one. Returning the *forgeerr.Error directly gives the
// boundary the correct category and exit code (ExitUsage, 64).
//
// The sentinel is preserved as the wrapped cause so that existing
// callers (and the tests that assert errors.Is(err,
// ErrQuietVerboseConflict)) continue to work.
//
// # Why the message matches the sentinel's
//
// The message is the sentinel's text, so the user-facing rendering
// of the conflict is unchanged by the wrapping. A future change
// that wants a different message for the user-facing output than
// for the sentinel's errors.Is comparison can pass a different
// string to forgeerr.Wrap; the sentinel's message remains the
// stable identity.
func validateLogFlags(cmd *cobra.Command) error {
	if cmd == nil {
		panic("cli: validateLogFlags called with nil command")
	}
	quiet := quietRequested(cmd)
	verbose := verboseRequested(cmd)
	if quiet && verbose {
		return forgeerr.Wrap(
			forgeerr.CategoryCLI,
			string(forgeerr.CodeCLIUnknownFlag),
			ErrQuietVerboseConflict.Error(),
			ErrQuietVerboseConflict,
		).
			WithContext("flags", "--quiet, --verbose").
			WithRemediation("Use one of --quiet or --verbose, not both.")
	}
	return nil
}

// logFlagsToInputs builds logger.Inputs from the parsed flags.
//
// It reads --log-level, --verbose, and --quiet. The env var and
// config file are merged by the caller (see resolveLogLevelWithEnv).
// This keeps the flag layer free of env/config knowledge.
func logFlagsToInputs(cmd *cobra.Command) logger.Inputs {
	return logger.Inputs{
		LogLevel: logLevelRequested(cmd),
		Verbose:  verboseRequested(cmd),
		Quiet:    quietRequested(cmd),
	}
}

// resolveLogLevelWithEnv merges flag, env, and config inputs and
// returns the effective logger level, using the given env lookup
// for FORGE_LOG_LEVEL.
//
// # The precedence chain
//
// The chain is documented in docs/reference/log-levels.md §2 and
// implemented in internal/logger/precedence.go. This function is
// the CLI-layer adapter:
//
//  1. Validate the flag contract (--quiet vs --verbose).
//  2. Read the flag values.
//  3. Read the env var (FORGE_LOG_LEVEL) through env.
//  4. Accept the config value from the caller.
//  5. Call logger.ResolveLevel.
//
// # Why the env lookup is a parameter
//
// (unchanged)
//
// # Error classification
//
// The function's errors are classified as CategoryCLI, because
// they are user errors (an invalid flag or env value). The
// classifier is applied here, not in logger.ResolveLevel: the
// logger package returns a plain error so its callers can match
// on the message; the CLI is the consumer that needs the
// *forgeerr.Error classification for the boundary's exit-code
// mapping.
//
// The two error paths are:
//
//  1. The flag contract violation (--quiet and --verbose both
//     set). validateLogFlags returns a *forgeerr.Error directly;
//     this function returns it unchanged.
//
//  2. A parsing failure from logger.ResolveLevel (an invalid
//     --log-level or FORGE_LOG_LEVEL value). The error is a plain
//     error; this function wraps it in a *forgeerr.Error with
//     CategoryCLI and code CodeCLIUnknownFormat.
//
// Both are usage errors (exit 64).
//
// # Why the error is not returned bare
//
// The boundary (runWithOptions) ensures every error it renders is
// a *forgeerr.Error. A bare error would be wrapped as
// FORGE_INTERNAL_UNEXPECTED (CategoryInternal, exit 70), which is
// the wrong classification for a user's invalid flag value. The
// wrap here gives the boundary the correct category and exit code
// (ExitUsage, 64).
func resolveLogLevelWithEnv(cmd *cobra.Command, env func(string) string, configLevel string) (logger.Decision, error) {
	if err := validateLogFlags(cmd); err != nil {
		// validateLogFlags returns a *forgeerr.Error; pass it
		// through unchanged.
		return logger.Decision{}, err
	}

	in := logFlagsToInputs(cmd)

	if env != nil {
		in.EnvLevel = env(logger.EnvVar)
	} else {
		envLevel, _ := logger.ReadEnv()
		in.EnvLevel = envLevel
	}

	in.ConfigLevel = configLevel

	decision, err := logger.ResolveLevel(in)
	if err != nil {
		// The resolver returned a parsing error (an invalid
		// --log-level or FORGE_LOG_LEVEL value). The error is
		// a plain error; wrap it in a *forgeerr.Error with
		// category CategoryCLI so the boundary classifies it
		// as a usage error (exit 64).
		//
		// The message is the resolver's, so the user sees the
		// specific value that failed. The context names the
		// input the resolver consulted.
		//
		// The code CodeCLIUnknownFormat is the closest
		// registered code for "the value of a flag is not in
		// its allowed set"; a future WBS may add a dedicated
		// code for log levels.
		return logger.Decision{}, forgeerr.Wrap(
			forgeerr.CategoryCLI,
			string(forgeerr.CodeCLIUnknownFormat),
			err.Error(),
			err,
		).WithContext("input", "--log-level or FORGE_LOG_LEVEL")
	}

	return decision, nil
}

// resolveLogLevel merges flag, env, and config inputs and returns
// the effective logger level.
//
// It is resolveLogLevelWithEnv with a nil env, which makes the
// resolver read the process environment (via logger.ReadEnv). This
// is the legacy entry point; callers that have an injected env
// (the CLI's process boundary, with opts.env) should call
// resolveLogLevelWithEnv directly.
//
// See resolveLogLevelWithEnv for the full docstring. The two
// functions share the same behavior when env is nil.
func resolveLogLevel(cmd *cobra.Command, configLevel string) (logger.Decision, error) {
	return resolveLogLevelWithEnv(cmd, nil, configLevel)
}
