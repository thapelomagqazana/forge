// Package cli defines the Forge command-line interface.
//
// This file holds the global flag interaction contract (WBS 7.4.3).
// The contract specifies how --quiet, --verbose, and --config
// interact with help output, version output, and error output. It
// is documented in docs/cli-ux-spec.md "Global Flags Interaction
// Matrix" and enforced by the tests in flag_interaction_test.go.
//
// # The interaction matrix
//
//	Invocation                              Behaviour
//	forge --quiet --help                    Help on stdout
//	forge --verbose --help                  Help on stdout; verbose adds nothing
//	forge --quiet version                   Version on stdout
//	forge --quiet unknown-cmd               Error on stderr
//	forge --verbose unknown-cmd             Error on stderr (stack trace in Phase 6+)
//	forge --quiet --verbose version         Quiet wins; version on stdout
//	forge --config nonexistent.yml version  Version on stdout (config is a warning)
//	forge --config nonexistent.yml check    Config is an error in future phases
//
// # The rules
//
//   - --quiet suppresses warnings and info logs; it never suppresses
//     errors.
//   - --quiet never suppresses stdout results (help, version, JSON).
//   - --verbose never adds to stdout. Verbose output goes to stderr
//     only.
//   - Errors are always printed to stderr, regardless of --quiet or
//     --verbose.
//   - --quiet wins over --verbose when both are set.
//   - Help and version are always printed to stdout, regardless of
//     --quiet.
//   - A config load failure in Phase 2 is a warning, not a fatal
//     error, since config has no behavioural impact yet.
//
// # Why the rules are what they are
//
// The two streams are the boundary between results and diagnostics
// (WBS 7.4.1). A result is a piece of information the user asked
// for; it goes to stdout. A diagnostic is a message about how the
// CLI is running or about a problem it encountered; it goes to
// stderr.
//
// --quiet and --verbose affect diagnostics, not results. --quiet
// suppresses the diagnostics the user does not want; --verbose
// adds diagnostics the user does want. Neither flag affects the
// result stream.
//
// The rule that --quiet never suppresses errors follows from the
// boundary: an error is a diagnostic, and a user who asked for
// quiet asked for less non-error output, not for errors to be
// hidden. Hiding errors would make the CLI's behaviour opaque;
// the user would see a failure without a reason.
//
// The rule that --quiet wins over --verbose when both are set is a
// convenience: the two flags have opposite effects, and the user
// who sets both has asked for both. The CLI honours the stricter
// one. --quiet is stricter because it commits to less output;
// --verbose commits to more. Honouring the smaller output surface
// is the safer default.
//
// # Phase 2 scope
//
// Phase 2 implements the log-level effects of --quiet and
// --verbose (WBS 12.0). The flags' interaction with help and
// version output is a consequence of the stream boundary: help
// and version are results, and the flags do not affect results.
//
// Phase 2 does not implement a stack-trace mode for --verbose.
// The "--verbose unknown-cmd adds a stack trace" row is
// aspirational; a future WBS item adds the mechanism and the test
// together.
//
// # No package-level mutable state
//
// The CLI package's contract test forbids package-level
// variables. This file declares only constants.
package cli

// quietFlagName is the long name of the --quiet flag.
//
// # Why a constant
//
// The tests reference the flag by name in several places. A
// future rename is a single-line change here; the tests read the
// constant. The constant is not exported because no code outside
// the package needs it.
const quietFlagName = "--quiet"

// verboseFlagName is the long name of the --verbose flag.
//
// # Why a constant
//
// Same rationale as quietFlagName.
const verboseFlagName = "--verbose"

// configFlagName is the long name of the --config flag.
//
// # Why a constant
//
// Same rationale as quietFlagName.
const configFlagName = "--config"

// nonexistentConfigPath is the path to a configuration file that
// does not exist.
//
// # Why a constant
//
// The tests that exercise the --config flag need a path that is
// guaranteed to be missing. The constant makes that choice
// explicit and shared across tests.
//
// # Why this path
//
// The path is relative and clearly artificial. A test that runs in
// the package directory will not find a file with this name. A
// future test that runs in a different directory can use the same
// constant; the path is equally absent everywhere.
const nonexistentConfigPath = "nonexistent-forge-config-do-not-create.yml"
