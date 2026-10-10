package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	appversion "github.com/thapelomagqazana/forge/internal/app/version"
)

// newVersionCmd constructs the `forge version` command.
//
// # The handler / service pattern
//
// This file is the reference implementation of the pattern defined
// by WBS 4.3.1 and documented in docs/architecture.md "Handler /
// Service Boundary".
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
// An unknown value produces an error from the service
// (`appversion.ErrUnknownFormat`). The handler wraps the error
// with a diagnostic that names the flag and the offending value,
// and returns it. `executeWithOptions` maps the error to the
// `ExitUsage` exit code (2), matching the "usage error" category
// for invalid flag values.
//
// # Why the error is wrapped, not replaced
//
// The sentinel `appversion.ErrUnknownFormat` is preserved by the
// wrap, so callers can test with errors.Is. The wrapping message
// is the user-facing text; the sentinel is the machine-facing
// signal. Both are useful.
//
// # Error path
//
// Two error paths exist:
//
//  1. Unknown format — the service returns ErrUnknownFormat,
//     wrapped by the handler. Exit code ExitUsage (2).
//  2. Write error — the service returns the writer's error,
//     wrapped by the service. Exit code ExitError (1).
//
// Both are returned by the handler; neither is written to stderr
// by the handler. The error-to-message and error-to-exit-code
// mapping is done one layer up, in executeWithOptions.
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
  --format json   a single-line JSON object suitable for machines`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := appversion.FormatAs(deps.Stdout, appversion.Get(), format); err != nil {
				if errors.Is(err, appversion.ErrUnknownFormat) {
					return fmt.Errorf("--format: %w", err)
				}
				return err
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&format, "format", appversion.FormatText,
		"Output format: text|json")

	return cmd
}
