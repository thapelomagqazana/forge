// Package cli implements the Forge command-line interface.
//
// This package is the sole owner of the Cobra command tree. Every other
// package in Forge is unaware that Cobra exists — they receive data
// structures and return results, and this package translates between
// Cobra's *cobra.Command abstraction and Forge's own domain model.
//
// # Public surface
//
// The package exports exactly one symbol: Execute. All other types and
// functions are unexported and are considered implementation details.
// This invariant is enforced by tests; see internal/cli/root_test.go.
//
// # Scope of this file
//
// root.go establishes the root command. It is intentionally minimal in
// WBS 2.4.1 (the current task): it exists to prove that Cobra is
// correctly wired into the module, and it gives WBS 4.x a place to
// grow the full command tree.
//
// Do not add business logic to this file. The pattern established here
// (unexported constructor, thin command tree) is the pattern every
// future command follows. Business logic belongs in application
// services under internal/<domain>/, not in this package.
//
// # Relation to other files
//
// The package will grow to include:
//
//   - root.go     — this file; the root command and Execute entry point.
//   - execute.go  — the injectable execution boundary (WBS 4.2.1).
//   - exitcodes.go — exit code constants and mapping (WBS 4.1.2).
//   - version.go  — the version command (WBS 6.3.1).
//   - config.go   — the config command (WBS 9.1.1).
//   - registry.go — the central command registry (WBS 4.4.1).
//
// Each of those files will be added in its own WBS task.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Execute runs the Forge CLI and returns a process exit code.
//
// It is the only exported symbol of this package. It exists so that
// cmd/forge/main.go can delegate the entire process lifecycle to a
// single, testable function.
//
// The signature is frozen. Adding parameters would break the contract
// with main.go. For testability, Execute delegates to an unexported
// function with injectable inputs (see WBS 4.2.1).
//
// In WBS 2.4.1, Execute is a stub: it constructs the root command,
// executes it with the process arguments, and returns 0 on success or
// 1 on failure. WBS 4.x replaces this with the full exit code mapping.
func Execute() int {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// newRootCmd constructs the root Cobra command.
//
// It is unexported because no package outside internal/cli is permitted
// to depend on the command tree. Tests within the package may call it
// directly.
//
// The root command's metadata (Use, Short, Long) is deliberately
// minimal in WBS 2.4.1. WBS 5.1.1 freezes the final values and adds
// acceptance tests for them.
//
// Cobra-specific configuration:
//
//   - SilenceUsage and SilenceErrors are set to true, so that Forge
//     controls error formatting and exit codes. Without these flags,
//     Cobra prints usage on every error and writes errors to stdout,
//     both of which break the CLI UX contract.
//
//   - TraverseChildren is left at its default (false). This will be
//     revisited in WBS 4.4 when the full command tree is added.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "forge",
		Short:         "Forge — Engineering Foundations as Code",
		Long:          `Forge is a cross-platform CLI for defining, generating, validating, and evolving software project foundations as code.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	return root
}