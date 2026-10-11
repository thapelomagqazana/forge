// Package cli implements the Forge command-line interface.
//
// This file holds the adapter that implements config.FlagReader
// on top of a *cobra.Command. The adapter is the CLI package's
// side of the port / adapter pair: the config package defines
// the FlagReader interface; this file supplies the concrete
// implementation that reads a Cobra command's flags.
//
// # Why the adapter lives here and not in internal/config
//
// The config package must not import Cobra (WBS 2.4.1's rule,
// restated in internal/cli/structure_test.go). The adapter's
// implementation reads a *cobra.Command; it therefore belongs
// in the package that owns the Cobra dependency. The config
// package owns the interface; the CLI package owns the
// implementation.
//
// # Usage
//
// The adapter is constructed from a *cobra.Command at the point
// where the configuration is resolved. The PersistentPreRunE
// hook (hooks.go) constructs the adapter and passes it to
// config.OverrideFromFlags:
//
//	cfg, err = config.OverrideFromFlags(cfg, newFlagReader(cmd))
//
// The adapter is a thin wrapper; it holds the command and
// forwards the two method calls to the command's flag set.
//
// # No package-level mutable state
//
// This file declares a type and its methods. It declares no
// package-level variables.
package cli

import (
	"github.com/spf13/cobra"
)

// flagReader adapts a *cobra.Command to the config.FlagReader
// interface.
//
// # What the type does
//
// The type holds the command. Its two methods forward to the
// command's flag set:
//
//   - Changed(name) → cmd.Flags().Changed(name)
//   - String(name)  → cmd.Flags().GetString(name), with the
//     error discarded in favour of an empty
//     string
//
// The forward is a one-line call in each method. The type
// exists to satisfy the interface; it does not add behaviour.
//
// # Why the error is discarded
//
// The config package's FlagReader interface does not have an
// error return on String. The reason is that the resolution
// only calls String on flags for which Changed returned true;
// a flag that Changed returned true for is guaranteed to
// exist. The GetString error would only fire for a missing
// flag, which the Changed check rules out. Discarding the
// error in the adapter is safe under that precondition.
//
// # Nil safety
//
// A nil *cobra.Command in the wrapped field is a programming
// error; the adapter's methods would panic on the first call.
// The constructor (newFlagReader) does not check for nil; the
// caller is responsible for supplying a non-nil command.
type flagReader struct {
	cmd *cobra.Command
}

// newFlagReader constructs a flagReader from cmd.
//
// The constructor is the only supported way to build a
// flagReader. It exists so that a future change to the type's
// field set is a single-line edit here; callers use the
// constructor.
func newFlagReader(cmd *cobra.Command) *flagReader {
	return &flagReader{cmd: cmd}
}

// Changed reports whether the named flag was explicitly set by
// the user.
//
// The method forwards to cmd.Flags().Changed(name). A flag
// inherited from a parent command is visible through
// cmd.Flags(); Cobra merges the flag sets. The method therefore
// reports "changed" for a flag set on the root command when
// called on a subcommand, which is the correct behaviour for
// the resolution.
func (r *flagReader) Changed(name string) bool {
	return r.cmd.Flags().Changed(name)
}

// String returns the named flag's value as a string.
//
// The method forwards to cmd.Flags().GetString(name). The
// error is discarded; see the type's docstring for the
// rationale.
//
// # Boundary: bool flags
//
// A bool flag's GetString returns "true" or "false". The
// config package's resolution only calls String on flags for
// which Changed returned true, and only for the two string
// flags the schema declares (--log-level and --output-format).
// A future bool flag that needed resolution would add a
// reader method; the interface would grow by one method.
func (r *flagReader) String(name string) string {
	v, _ := r.cmd.Flags().GetString(name)
	return v
}
