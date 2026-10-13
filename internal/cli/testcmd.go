// Package cli — the test-only `__test_emit_lines__` subcommand.
//
// This file defines a subcommand that exists only for the
// broken-pipe integration tests in cmd/forge/. It is registered
// when the FORGE_TEST environment variable is set to a non-empty
// value, and it is not registered otherwise.
//
// # Why the subcommand exists
//
// The broken-pipe contract (WBS 12.4.1) requires the process to
// exit 141 when a write to stdout fails because the read end of
// the pipe has closed. To exercise the contract, a test must run
// a command that writes many lines to stdout, and it must close
// the pipe's read end after the first line. No production
// command in Phase 2 writes enough output to trigger the
// condition; the test command exists to fill that gap.
//
// # Why the command is gated by FORGE_TEST
//
// The command is not part of the CLI's public surface. It must
// not appear in `forge --help`, must not be invocable by a
// production user, and must not be reachable in a release build.
// The env-var gate achieves all three: the command is registered
// only when the test that needs it sets FORGE_TEST=1.
//
// The gate is not a security boundary: a user who sets
// FORGE_TEST=1 could invoke the command. The command has no
// side effects beyond writing to stdout; the concern is surface
// area, not security. A production user who sets FORGE_TEST=1
// gets a command that writes lines to stdout, which is a
// harmless thing for a CLI to do.
//
// # Why the environment is read through deps.Env
//
// The registration reads FORGE_TEST from deps.Env, not from
// os.Getenv. Rule 6 of the CLI contract forbids command files
// from reading process globals directly; every environment
// lookup must flow through the Dependencies value. The
// Dependencies value is available at construction time, which
// is when the registration runs, so the rule is satisfiable
// without an exception.
//
// A caller that constructs the CLI with a synthetic Env (for
// example, the contract tests) sees the FORGE_TEST gate
// behave according to the synthetic environment. A caller that
// constructs the CLI with os.Getenv (production) sees it behave
// according to the process environment. Both are correct; the
// source of the value is the caller's choice.
//
// # The command's contract
//
// The command takes one positional argument: the number of lines
// to write. It writes exactly that many lines to
// deps.Streams.Out, one line per iteration, and returns the
// writer's error if any. It does not sleep between writes; the
// test controls the timing by how quickly it closes the read
// end.
//
// # The command's name
//
// The command's Use field is "__test_emit_lines__". The double
// underscore prefix and suffix mark the name as a test fixture
// so that a leak into any help output is obvious. The
// broken-pipe tests in cmd/forge/ invoke the command by this
// name.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// testEmitLinesEnvVar is the environment variable that enables the
// test-only subcommands. Any non-empty value enables them.
//
// The variable is read from deps.Env at command-construction
// time. A production process whose deps.Env resolves to os.Getenv
// and whose FORGE_TEST is unset never registers the test
// commands.
const testEmitLinesEnvVar = "FORGE_TEST"

// testEmitLinesUse is the command's Use field. The double
// underscore prefix and suffix mark the name as a test fixture.
//
// The constant is exported to the package's test files so that
// the broken-pipe tests and any future test that needs to invoke
// the command refer to the same string. A test that hardcoded
// the name would drift from the command's actual Use if the name
// changed.
const testEmitLinesUse = "__test_emit_lines__"

// registerTestCommands registers the test-only subcommands on the
// root command when the test environment variable is set.
//
// The function is called from newRootCmd, after the production
// commands have been registered. It is a no-op when the variable
// is unset: the function returns immediately and the production
// command tree is unchanged.
//
// # Why the function reads deps.Env
//
// Rule 6 of the CLI contract forbids command files from reading
// process globals directly. The Dependencies value carries an Env
// lookup that the caller supplies; the function uses that lookup
// rather than os.Getenv. The rule is satisfied by construction,
// and a caller that supplies a synthetic Env sees the gate
// behave according to the synthetic environment.
func registerTestCommands(root *cobra.Command, deps Dependencies) {
	if deps.Env == nil {
		return
	}
	if deps.Env(testEmitLinesEnvVar) == "" {
		return
	}
	root.AddCommand(newTestEmitLinesCmd(deps))
}

// newTestEmitLinesCmd constructs the test-only command that writes
// N lines to stdout.
//
// The command's Use field is testEmitLinesUse. The broken-pipe
// tests invoke it by name; the double underscore marks it as a
// test fixture in any accidental leak.
func newTestEmitLinesCmd(deps Dependencies) *cobra.Command {
	return &cobra.Command{
		Use:    testEmitLinesUse,
		Short:  "test-only: emit N lines to stdout",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var n int
			if _, err := fmt.Sscanf(args[0], "%d", &n); err != nil {
				return fmt.Errorf("__test_emit_lines__: parse count: %w", err)
			}
			for i := 0; i < n; i++ {
				if _, err := fmt.Fprintf(deps.Streams.Out, "line %d\n", i); err != nil {
					// The write failed. Return the error so
					// the boundary can classify it. The
					// broken-pipe classification turns
					// EPIPE into ExitBrokenPipe (141).
					return err
				}
			}
			return nil
		},
	}
}
