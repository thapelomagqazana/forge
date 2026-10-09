package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

// newConfigCmd constructs the `forge config` command.
//
// # Status: placeholder
//
// This command is a placeholder. The real implementation is WBS 8.x.
// In the current state, the command is hidden (it does not appear in
// `forge --help`) and its RunE handler returns a "not yet implemented"
// error. It exists so that the registry has two entries and the
// ordering convention in registry.go is visible in code.
//
// When WBS 8.x lands, this file is replaced with the real
// implementation. The only change required in registry.go is nothing:
// the entry is already in the correct alphabetical position, and the
// constructor's signature already matches commandConstructor.
//
// # Why a hidden placeholder is preferable to an absent entry
//
// The alternative is to omit newConfigCmd from the registry until
// WBS 8.x. That is simpler in the short term but leaves the registry
// with a single entry, which makes the ordering convention
// unobservable. A registry with one entry cannot demonstrate that
// "commands are ordered alphabetically within groups"; a registry
// with two entries can. The placeholder is the cheapest way to
// demonstrate the convention.
//
// The placeholder is hidden so that users of the current binary do
// not see a `config` command that does nothing. `forge --help` lists
// only `version`. The registry test skips hidden commands when
// checking the order in help output; it still checks that the
// placeholder is present in the registry.
//
// # Following the handler / service boundary
//
// Even though the placeholder does nothing, it follows the
// handler / service boundary established by WBS 4.3.1:
//
//   - The handler is a thin RunE closure.
//   - It returns an error; it does not call os.Exit.
//   - It does not write to stdout or stderr directly.
//
// The placeholder's handler is the smallest possible conforming
// handler: it returns an error and does nothing else. When WBS 8.x
// replaces the placeholder, the new handler will follow the same
// pattern as version.go.
func newConfigCmd(deps Dependencies) *cobra.Command {
	return &cobra.Command{
		Use:    "config",
		Short:  "Manage Forge configuration (not yet implemented)",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("forge config: not yet implemented")
		},
	}
}
