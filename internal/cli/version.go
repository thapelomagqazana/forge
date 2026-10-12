// Package cli — the `forge version` command.
//
// This file is the reference implementation of the handler /
// service pattern defined by WBS 4.3.1 and documented in
// docs/architecture.md "Handler / Service Boundary".
//
// The handler:
//
//   - Parses flags.
//   - Collects arguments (there are none; the command is arg-free).
//   - Calls the application service.
//   - Writes the result to deps.Stdout, never to a process-global.
//   - Returns an error on failure; never calls os.Exit.
//
// The handler does not:
//
//   - Read or write files.
//   - Access the network.
//   - Contain business rules or formatting logic.
//   - Write to os.Stdout or os.Stderr directly.
//   - Print formatted output with fmt.Fprintln or its variants.
//
// The handler body is under ten lines of executable code. The
// `--format` flag is parsed into a local variable; the dispatch
// happens inside `appversion.FormatAs`.
//
// # The --format flag
//
// The flag has two supported values, "text" and "json". The
// default is "text", which produces the human-readable output
// frozen by WBS 6.4.1. The "json" value produces the machine-
// readable output frozen by WBS 6.4.2.
//
// An unknown value produces a *forgeerr.Error with category
// CategoryCLI and code CodeCLIUnknownFormat. The handler
// constructs the error; executeWithOptions renders it and maps
// its category to the exit code (2, ExitUsage).
//
// # Why the error is a *forgeerr.Error
//
// The WBS 10.5.1 invariant is that every error reaching the CLI
// boundary is a *forgeerr.Error. Before the migration, the
// handler returned a plain error wrapped around the service's
// sentinel; the boundary classified the plain error as
// FORGE_INTERNAL_UNEXPECTED, and the exit code was
// ExitFailure (1) instead of ExitUsage (2).
//
// The handler now constructs a *forgeerr.Error directly. The
// category is CategoryCLI, because the failure is a usage error
// (the user passed a value the command does not accept). The
// code is CodeCLIUnknownFormat, which is a member of the
// registry (WBS 10.3.1). The offending value is in the error's
// context under the key "format", and the remediation names the
// accepted values.
//
// # What about the service's sentinel
//
// The service still exports appversion.ErrUnknownFormat. The
// handler does not use it in the new implementation: the
// *forgeerr.Error carries the same information in a structured
// form (the code), and the CLI's consumer can match on the code
// via errors.As + Code() (the pattern documented in ADR-015).
// The sentinel remains exported for callers outside the CLI that
// need it; a future WBS item may deprecate it.
//
// # Error path
//
// Two error paths exist:
//
//  1. Unknown format — the handler constructs a *forgeerr.Error
//     with code CodeCLIUnknownFormat. Exit code ExitUsage (2).
//  2. Write error — the service returns the writer's error,
//     wrapped by the service. Exit code depends on the service's
//     category; for a write failure it is ExitFailure (1).
//
// Both are returned by the handler; neither is written to stderr
// by the handler. The error-to-message and error-to-exit-code
// mapping is done one layer up, in executeWithOptions.
//
// # The Example field (WBS 7.1.3)
//
// The per-command help contract (WBS 7.1.3) requires an Examples
// block for any command that takes arguments, has command-specific
// flags, or has subcommands. The version command has a
// command-specific flag (`--format`), so it must declare an
// Example.
//
// Cobra renders the Example field under an "Examples:" heading,
// between the Usage section and the Flags section. The content is
// rendered verbatim; the leading two spaces on each line are part
// of the field's value, not added by Cobra.
//
// The two examples are the command's two primary invocations: the
// default text output and the JSON output. They are deliberately
// minimal; a user who needs more detail reads the Long
// description or the CLI UX spec.
//
// The golden file internal/cli/testdata/help/version.golden.txt
// records the rendered output, including this Examples block. A
// change to the field's content changes the rendered output and
// fails TestHelpGolden_Subcommands/version until the golden file
// is regenerated.
package cli

import (
	"github.com/spf13/cobra"

	appversion "github.com/thapelomagqazana/forge/internal/app/version"
	"github.com/thapelomagqazana/forge/internal/forgeerr"
)

// newVersionCmd constructs the `forge version` command.
//
// The constructor's signature is fixed by the command registry
// (registry.go): every constructor takes a Dependencies value and
// returns a *cobra.Command. The handler body is under ten lines
// of executable code; the flags, arguments, and formatting are
// delegated.
func newVersionCmd(deps Dependencies) *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print Forge version information",
		Long: `Print the version, commit, build date, Go version, and
platform of the running Forge binary.

The default output is a stable, line-oriented format suitable for
humans. The --format flag selects an alternative representation:

  --format text   the human-readable format (default)
  --format json   a single-line JSON object suitable for machines

See the CLI UX specification for the frozen format of each.`,
		Example: `  forge version
  forge version --format json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// The dispatch is delegated to the application
			// service. The service returns a plain error when
			// the writer fails; the handler wraps it as a
			// *forgeerr.Error if it is not one.
			//
			// The unknown-format error is constructed below,
			// before the call, because the service's sentinel
			// (ErrUnknownFormat) is not a *forgeerr.Error.
			// Pre-checking the format keeps the handler's
			// error path uniform: every error the handler
			// returns is a *forgeerr.Error.
			switch format {
			case appversion.FormatText, appversion.FormatJSON:
				// The value is supported; proceed.
			default:
				return forgeerr.New(
					forgeerr.CategoryCLI,
					string(forgeerr.CodeCLIUnknownFormat),
					"unknown output format",
				).
					WithContext("format", format).
					WithRemediation("Use '--format text' or '--format json'.")
			}

			if err := appversion.FormatAs(deps.Stdout, appversion.Get(), format); err != nil {
				// The service returned a writer error. Wrap
				// it as an INTERNAL error, so the invariant
				// "every error reaching the boundary is a
				// *forgeerr.Error" holds without the
				// boundary's safety-net wrapping firing.
				//
				// The wrapper is the FORGE_INTERNAL_UNEXPECTED
				// shape: the failure is a write to the
				// injected writer, which is an internal
				// concern (the caller supplied the writer,
				// and its failure is not a user input
				// problem).
				return forgeerr.Wrap(
					forgeerr.CategoryInternal,
					string(forgeerr.CodeInternalUnexpected),
					"failed to write version output",
					err,
				)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&format, "format", appversion.FormatText,
		"Output format: text|json")

	return cmd
}
