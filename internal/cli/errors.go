// Package cli defines the Forge command-line interface.
//
// This file holds the constants and helpers that the invalid-
// command contract (WBS 7.3.1) enforces.
//
// The user-facing error format itself lives in
// internal/forgeerr/format.go (WBS 10.4.1). The CLI does not
// implement a formatter of its own: every error that reaches the
// user is a *forgeerr.Error, and every rendering is performed by
// forgeerr.Format. This file's job is to construct the
// *forgeerr.Error for the one case the CLI is responsible for —
// an unknown command typed at the root — and to expose the
// suggestion-distance constant that governs Cobra's suggestion
// mechanism.
//
// # No package-level mutable state
//
// The CLI package's contract test forbids package-level
// variables. This file declares only constants and functions.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/thapelomagqazana/forge/internal/forgeerr"
)

// suggestionsMinimumDistance is the maximum edit distance at which
// Cobra's suggestion mechanism fires.
//
// # Why 2
//
// Cobra's default is 2. An explicit setting here makes the value
// visible in the source and pins it against a future Cobra default
// change. The contract requires that a single-character typo (edit
// distance 1) and a two-character typo (edit distance 2) both
// trigger a suggestion. A distance of 1 would miss two-character
// typos; a distance of 3 would produce suggestions for inputs that
// are not close to any command.
const suggestionsMinimumDistance = 2

// unknownCommandError constructs the error returned when the
// user types a command that the root command does not recognise.
//
// The error is a *forgeerr.Error with code
// FORGE_CLI_UNKNOWN_COMMAND. Its message names the command and
// the parent, matching Cobra's standard phrasing. Its
// remediation names the closest match, when Cobra's suggestion
// mechanism (SuggestionsMinimumDistance, configured in root.go)
// produces one.
//
// # Why the suggestion is a remediation and not part of the message
//
// The frozen user format (WBS 10.4.1) has separate slots for the
// message and the remediation. The message describes what went
// wrong; the remediation describes what to do about it. A
// suggestion ("Did you mean 'version'.") is a remediation, not a
// description, so it belongs in the remediation field.
//
// # Why the remediation ends with a period
//
// Rule F6 of the frozen format requires the remediation to end
// with a period. The period follows the quoted suggestion; the
// resulting string is grammatically unusual but satisfies the
// invariant. If the team wants a question mark to satisfy the
// rule, that requires an ADR amending WBS 10.4.1.
//
// # Why no "Run 'forge --help' for usage." line
//
// The frozen format has no slot for a usage line. The remediation
// is a specific, actionable suggestion or absent. A general "how
// to get help" pointer is not a remediation for a specific error,
// and adding it would require a format change (an ADR). When no
// suggestion is available, the error carries no remediation and
// the frozen format omits the Suggestion block entirely.
func unknownCommandError(cmd *cobra.Command, name string) error {
	base := fmt.Sprintf("unknown command %q for %q", name, cmd.CommandPath())

	e := forgeerr.New(
		forgeerr.CategoryCLI,
		string(forgeerr.CodeCLIUnknownCommand),
		base,
	)

	suggestions := cmd.SuggestionsFor(name)
	if len(suggestions) == 0 {
		return e
	}

	rem := fmt.Sprintf("Did you mean %q.", suggestions[0])
	return e.WithRemediation(rem)
}
