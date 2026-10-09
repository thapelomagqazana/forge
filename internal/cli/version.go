package cli

import (
	"github.com/spf13/cobra"

	"github.com/thapelomagqazana/forge/internal/app/version"
)

// newVersionCmd constructs the `forge version` command.
//
// # The handler / service pattern
//
// This file is the reference implementation of the pattern defined
// by WBS 4.3.1 and documented in docs/architecture.md § 11.12.
//
// The handler:
//
//   - Parses flags (there are none in Phase 2).
//   - Collects arguments (there are none in Phase 2).
//   - Constructs the application service.
//   - Calls the service.
//   - Formats the result by delegating to the service's formatter.
//   - Returns an error on failure; never calls os.Exit.
//
// The handler does not:
//
//   - Read or write files.
//   - Access the network.
//   - Contain business rules.
//   - Contain business logic.
//   - Write to os.Stdout or os.Stderr directly.
//   - Print formatted output with fmt.Fprintln.
//
// The handler is deliberately short. The target is under 20 lines of
// handler body (AC3). If a future change pushes it over 20 lines,
// the change is almost certainly business logic that belongs in the
// service package.
//
// # Why deps is a parameter
//
// The command receives a Dependencies value from newRootCmd. In Phase
// 2, the version command uses only deps.Stdout. A future version of
// the command may read a --format flag and write JSON to the same
// writer; the Dependencies value already carries everything needed.
//
// # Error path
//
// The only error the handler can return is a write error from
// Format. Cobra captures the error and returns it from Execute;
// executeWithOptions formats it and maps it to an exit code. The
// handler does not touch os.Exit, and the handler does not touch
// os.Stderr. Both are handled one layer up.
func newVersionCmd(deps Dependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print Forge version information",
		Long: `Print the version, commit, build date, Go version, and
platform of the running Forge binary.

The output is a stable, line-oriented format suitable for humans.
Machine-readable output (--format json) is planned for a future
phase.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return version.Format(deps.Stdout, version.Get())
		},
	}
}
