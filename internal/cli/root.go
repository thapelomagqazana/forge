package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newRootCmd constructs the root Cobra command.
//
// It is unexported because no package outside internal/cli is
// permitted to depend on the command tree. Tests within the package
// may call it directly.
//
// The root command's metadata is deliberately minimal in WBS 4.2.1.
// WBS 5.1.1 freezes the final values (Use, Short, Long) and adds
// acceptance tests for them.
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
// forwards it to the subcommand constructors. As of WBS 4.3.1 there
// is one such subcommand (version); the constructor receives deps and
// captures it in its handler closure.
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
// TraverseChildren is left at its default (false). This will be
// revisited in WBS 4.4.1 when the full command tree is added.
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
// The options struct (options.go) is the process boundary: it is the
// only place os.Args, os.Stdin, os.Stdout, os.Stderr, os.Getenv, and
// os.Getwd are read. The Dependencies struct is the command boundary:
// it is the resolved, injectable set of collaborators that commands
// consume. executeWithOptions is the single transformation point
// between the two. This function sees only Dependencies.
//
// # Subcommand registration
//
// Every subcommand constructor receives the same Dependencies value.
// The constructor captures the value in its handler closure and never
// reaches for a global. See docs/architecture.md § 11.12 for the
// handler / service boundary that every subcommand follows, and
// internal/cli/version.go for the reference implementation.
func newRootCmd(deps Dependencies) *cobra.Command {
	root := &cobra.Command{
		Use:           "forge",
		Short:         "Forge — Engineering Foundations as Code",
		Long:          `Forge is a cross-platform CLI for defining, generating, validating, and evolving software project foundations as code.`,
		SilenceUsage:  true,
		SilenceErrors: true,

		// The handler runs only when Cobra has not dispatched to a
		// subcommand. That happens in two cases:
		//
		//  1. The invocation has no arguments at all.
		//  2. The invocation's first argument does not match any
		//     registered subcommand.
		//
		// Case 1 is a request for information; case 2 is a usage
		// error. The handler distinguishes them by inspecting args.
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				// No arguments: the user wants help. Print to
				// stdout and return nil, which maps to
				// ExitSuccess.
				return cmd.Help()
			}

			// One or more unknown arguments. Return an error,
			// which maps to ExitUsage via the default case in
			// exitCodeFromError.
			return fmt.Errorf("unknown command %q for %q",
				args[0], cmd.Name())
		},

		// Args is set to ArbitraryArgs so that Cobra passes the
		// positional arguments to RunE instead of rejecting them
		// before RunE runs. RunE is responsible for validating them.
		Args: cobra.ArbitraryArgs,
	}

	// Register subcommands. Every constructor receives the same
	// Dependencies value, captures it in its handler closure, and
	// never reaches for a global.
	//
	// The order of registration is the order Cobra lists subcommands
	// in help output when no explicit ordering is applied. Cobra
	// sorts alphabetically by default; the registration order is
	// therefore not significant for user-visible output today. It is
	// kept in the order the commands were introduced, for
	// readability.
	//
	// See docs/architecture.md § 11.12 for the handler / service
	// boundary that newVersionCmd (and every future constructor)
	// follows.
	root.AddCommand(newVersionCmd(deps))

	return root
}
