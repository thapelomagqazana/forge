package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// =============================================================================
// Root command identity strings
// =============================================================================
//
// The four constants below are the frozen identity of the root
// command. They appear in `forge --help`, in documentation, and in
// user-facing messages. They are authored once, committed, and
// treated as a contract: changing any of them is a change to the
// CLI's identity, and the tests in root_test.go fail until the
// change is deliberate.
//
// # Why constants
//
// The strings are constants rather than inline literals in
// newRootCmd for three reasons:
//
//  1. They are referenced by more than one consumer. The root
//     command uses them; tests use them; the documentation quotes
//     them. A single definition is the only way to keep the three
//     in sync.
//
//  2. They are part of the CLI's identity contract. A user who
//     reads `forge --help` sees RootLongDesc; a user who pipes
//     `forge --help` into a script sees the same string; a
//     contributor reading the WBS item sees the same string. The
//     constant is the canonical form.
//
//  3. They can be asserted on. A test can compare the help output
//     to the constant, which is a stronger assertion than comparing
//     to a literal. If the literal and the constant diverge, the
//     test fails; if the constant and the help output diverge, the
//     test also fails.
//
// # Rules the constants satisfy
//
//   - RootName is lowercase. It is never "Forge" in the CLI's name
//     field. The lowercase form is the command a user types.
//
//   - RootUsage is a single line ending with "[command]". It
//     describes the root command as a dispatcher.
//
//   - RootShortDesc is a single line of 80 characters or fewer. It
//     is the text that appears in a parent's help output and in
//     documentation's one-line summary.
//
//   - RootLongDesc is a raw string literal. It preserves its line
//     breaks and appears verbatim in `forge --help`. It ends with
//     a pointer to `<command> --help`.
//
//   - None of the four contains emojis, colour codes, or tabs.
//     Colour and emphasis belong to the terminal, not to the CLI's
//     identity strings.
//
// # Where each string is used
//
//	RootName        the first word of cobra.Command.Use
//	RootUsage       the "Usage:" line in help output
//	RootShortDesc   parent help and documentation summaries
//	RootLongDesc    the body of `forge --help`
//
// # Changing a string
//
// Changing any of these strings requires updating the test that
// asserts on it and the section of docs/cli-ux-spec.md that quotes
// it. The two updates are in the same commit as the change. A
// reviewer who sees a constant change without the corresponding
// test and documentation changes rejects the PR.
const (
	// RootName is the CLI's name as a user types it. It is
	// lowercase and contains no spaces, uppercase letters, or
	// punctuation. It is the first word of cobra.Command.Use and
	// the string a user types to invoke the CLI.
	RootName = "forge"

	// RootUsage is the usage line that appears in help output. It
	// describes the root command as a dispatcher with subcommands.
	// The "[command]" suffix is the conventional syntax for an
	// optional positional argument.
	//
	// The full Use field on the cobra.Command is composed from
	// RootName and RootUsage; see newRootCmd for the composition.
	RootUsage = "forge [command]"

	// RootShortDesc is the one-line description of the CLI. It
	// appears in a parent's help output (if Forge were ever a
	// subcommand of another tool) and in documentation summaries.
	// It is 80 characters or fewer.
	//
	// The em dash (—) is the only non-ASCII character permitted in
	// the identity strings. It is used as a separator between the
	// product name and the tagline, and it is intentional.
	RootShortDesc = "Forge — Engineering Foundations as Code"

	// RootLongDesc is the extended description of the CLI. It
	// appears as the body of `forge --help`, above the "Usage:"
	// line.
	//
	// The string is a raw literal (backticks), so its line breaks
	// and indentation are preserved exactly. The indentation of
	// the "CREATE → ..." block is deliberate: it is a code block
	// in the output.
	//
	// The string ends with a pointer to `<command> --help`. The
	// pointer tells a user how to learn more about a specific
	// subcommand. Every version of this string must end with some
	// equivalent pointer; the test in root_test.go asserts the
	// presence of the substring "<command> --help".
	RootLongDesc = `Forge is a cross-platform CLI for defining, generating,
validating, and evolving software project foundations as code.

The core loop:

    CREATE  →  forge new
    VERIFY  →  forge check
    EXPLAIN →  forge explain
    EVOLVE  →  forge update

Run 'forge <command> --help' for details on any command.`
)

// newRootCmd constructs the root Cobra command.
//
// It is unexported because no package outside internal/cli is
// permitted to depend on the command tree. Tests within the package
// may call it directly.
//
// The root command's identity strings are frozen as package-level
// constants (see the block above). newRootCmd references those
// constants rather than embedding the strings inline, so that the
// strings have exactly one definition and can be asserted on by
// tests.
//
// # Why the constructor accepts Dependencies
//
// Subcommands added in later WBS items need access to the injectable
// collaborators: the resolved config, the logger, the filesystem
// abstraction, the output streams, and the environment lookup. The
// cleanest way to provide that access is to pass a single
// Dependencies value down the command tree at construction time.
//
// Before WBS 4.2.2, this constructor accepted the raw options struct.
// That worked while there were no subcommands, but it coupled every
// future subcommand constructor to the process-boundary type. The
// Dependencies struct decouples them: subcommands receive resolved
// collaborators, not raw os.* values, and the constructor signature
// is stable across phases.
//
// The root command itself does not consume the value. It only
// forwards it to the subcommand constructors, which receive it
// through the registry (see the "Subcommand registration" section
// below). Each subcommand's constructor captures deps in its
// handler closure and never reaches for a global.
//
// # Why the root command has a RunE handler
//
// Cobra's default behaviour when a command has subcommands but no
// RunE handler is to print the command's help to stdout and exit 0,
// even when the invocation contains unrecognised arguments. That
// behaviour is wrong for Forge, because it silently swallows
// unknown commands and prints the help text to stdout instead of an
// error to stderr.
//
// The RunE handler distinguishes the two cases the root command must
// handle:
//
//   - No positional arguments: the user requested information.
//     Print help to stdout and return nil, which maps to ExitSuccess.
//
//   - One or more positional arguments that do not match a registered
//     subcommand: the user typed a command that does not exist.
//     Return an error. executeWithOptions prints the error to stderr,
//     and exitCodeFromError maps it to ExitUsage.
//
// This pattern is the standard way to build a Cobra command tree
// where the root is a dispatcher rather than a runnable command
// itself.
//
// # Cobra configuration
//
// SilenceUsage and SilenceErrors are set to true, so that Forge
// controls error formatting and exit codes. Without these flags,
// Cobra prints usage on every error and writes errors to stdout, both
// of which break the CLI UX contract.
//
// TraverseChildren is left at its default (false). If the command
// tree ever requires traversal before dispatch, this is the field
// to revisit; the default is correct for a tree whose subcommands
// do not use persistent flags that apply to their own invocation.
//
// # No I/O
//
// The function performs no I/O of its own. It does not read os.Args,
// does not write to any stream, and does not touch the filesystem.
// The handler is delegated to Cobra, which uses the streams bound by
// executeWithOptions from the Dependencies value.
//
// # Relationship to options
//
// The options struct (execute.go) is the process boundary: it is the
// only place os.Args, os.Stdin, os.Stdout, os.Stderr, os.Getenv, and
// os.Getwd are read. The Dependencies struct is the command boundary:
// it is the resolved, injectable set of collaborators that commands
// consume. executeWithOptions is the single transformation point
// between the two. This function sees only Dependencies.
//
// # Subcommand registration
//
// Subcommands are not registered explicitly in this function. They
// are registered by iterating the central registry defined in
// registry.go:
//
//	for _, ctor := range registry {
//	    root.AddCommand(ctor(deps))
//	}
//
// The registry is the single place where the command tree's shape is
// declared. Adding a command means appending to the registry slice,
// not editing this function. See registry.go for the four-step
// procedure a contributor follows, and docs/architecture.md § 11.13
// for the rules the registry enforces.
//
// Every constructor in the registry receives the same Dependencies
// value. The constructor captures it in its handler closure and
// never reaches for a global. See docs/architecture.md § 11.12 for
// the handler / service boundary that every subcommand follows, and
// internal/cli/version.go for the reference implementation.
func newRootCmd(deps Dependencies) *cobra.Command {
	root := &cobra.Command{
		// Use is composed from RootName and RootUsage. The two
		// constants are the source of truth; the composition is
		// the only place they are joined.
		Use: RootName + " [command]",

		Short: RootShortDesc,
		Long:  RootLongDesc,

		SilenceUsage:  true,
		SilenceErrors: true,

		// The handler runs only when Cobra has not dispatched to a
		// subcommand. See the docstring above for the two cases.
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return fmt.Errorf("unknown command %q for %q",
				args[0], cmd.Name())
		},

		// Args is set to ArbitraryArgs so that Cobra passes the
		// positional arguments to RunE instead of rejecting them
		// before RunE runs. RunE is responsible for validating them.
		Args: cobra.ArbitraryArgs,
	}

	// Register every subcommand in the central registry. See the
	// docstring above and docs/architecture.md § 11.13.
	for _, ctor := range registry {
		root.AddCommand(ctor(deps))
	}

	return root
}
