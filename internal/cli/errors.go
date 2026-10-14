// Package cli defines the Forge command-line interface.
//
// This file holds the constants and helpers that the invalid-
// command contract (WBS 7.3.1) enforces, plus the package-local
// predicates that the boundary uses to classify errors without
// importing the libraries that produce them.
//
// The user-facing error format itself lives in
// internal/forgeerr/format.go (WBS 10.4.1). The CLI does not
// implement a formatter of its own: every error that reaches the
// user is a *forgeerr.Error, and every rendering is performed by
// forgeerr.Format. This file's job is to construct the
// *forgeerr.Error for the one case the CLI is responsible for —
// an unknown command typed at the root — to expose the
// suggestion-distance constant that governs Cobra's suggestion
// mechanism, and to declare the predicates the boundary uses.
//
// # Type-level contract
//
// forgeerr.New takes its code parameter as forgeerr.Code. The
// constructor below passes the named constant from the registry
// directly. A string(...) cast in a constructor call is the
// diagnostic that the migration documented in safefs/errors.go
// was rolled back.
//
// # No package-level mutable state
//
// The CLI package's contract test forbids package-level
// variables. The file declares only constants and functions. A
// predicate (a function that inspects an error's message) is
// preferred over a sentinel `var` for the reason documented on
// isPflagHelpRequested: errors.Is compares by identity, and two
// errors.New calls with the same message are distinct values, so
// a sentinel comparison across packages silently fails.
//
// # Why predicates live in errors.go
//
// The boundary (execute.go) classifies errors. Two categories
// exist:
//
//   - Errors that originate in the CLI's own code or in a Forge
//     package. These are already *forgeerr.Error values, or they
//     are wrapped by wrapCobraError.
//   - Errors that originate in a third-party package and that the
//     boundary must recognize by value or message. These are
//     recognized by a predicate function that does not import the
//     third-party package.
//
// isPflagHelpRequested is the second kind. Its docstring records
// the coupling and the rationale.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/thapelomagqazana/forge/internal/forgeerr"
)

// errPflagHelpRequestedMsg is the message pflag uses for its
// ErrHelp sentinel.
//
// # What pflag.ErrHelp is
//
// pflag's FlagSet.Parse returns a sentinel error (pflag.ErrHelp,
// whose message is "pflag: help requested") when the --help flag
// is set. The error is not a failure: it is pflag's signal that
// the caller should print help and exit. Cobra intercepts the
// signal in its own ExecuteContext and produces the help output.
//
// # Why a message constant and not a sentinel error
//
// execute.go is subject to a stdlib-only import policy (enforced
// by TestStdlibOnlyForExecuteImports). Importing
// github.com/spf13/pflag to reference pflag.ErrHelp directly
// would violate the policy. The natural alternative — declaring a
// package-local sentinel via
//
//	var errPflagHelpRequested = errors.New("pflag: help requested")
//
// and comparing with errors.Is — does not work: errors.Is
// compares errors by identity (the == operator on the error
// values) or by an explicit Is method on either error. It does
// not compare messages. Two calls to errors.New with the same
// string produce two distinct *errors.errorString values, and
// errors.Is on the two returns false.
//
// The correct check is therefore a string comparison of the
// messages. The constant holds pflag's message; isPflagHelpRequested
// performs the comparison.
//
// # Coupling to pflag's message
//
// The constant's value is pflag's, verbatim. If a future pflag
// version changes the message, isPflagHelpRequested will return
// false and the early parse will classify the help signal as an
// error. The failure is loud (a --help invocation would exit 70
// rather than 0), and the fix is a one-line change to the
// constant. The coupling is documented here so the fix is not a
// mystery to a future contributor.
const errPflagHelpRequestedMsg = "pflag: help requested"

// isPflagHelpRequested reports whether err is pflag's ErrHelp
// signal, recognized by its message.
//
// # What the function does
//
// The function returns true when err is non-nil and its message
// equals errPflagHelpRequestedMsg. It is the boundary's only
// consumer of the message constant.
//
// # Why a message comparison and not errors.Is
//
// errors.Is compares errors by identity. Two errors created by
// separate errors.New calls with the same message are distinct
// values; errors.Is(err, localSentinel) returns false even when
// err.Error() == localSentinel.Error(). The check the boundary
// needs is "does this error carry pflag's help message?", and the
// correct implementation is a string comparison.
//
// The alternative — importing pflag and comparing with
// errors.Is(err, pflag.ErrHelp) — would succeed (the comparison
// would match the same pointer), but violates the stdlib-only
// import policy on execute.go.
//
// # Why the helper exists
//
// The check is used by execute.go's early flag parse. Extracting
// it into a named function makes the boundary's call site read as
// a question ("is this pflag's help signal?") rather than as a
// string comparison whose intent is less obvious. The helper is
// also the single place the message constant is referenced, so a
// future pflag message change is one edit.
//
// # Why not a typed error with an Is method
//
// A custom error type implementing Is(error) bool would let the
// boundary keep using errors.Is. But the type would have to be
// constructed somewhere, and the error it compares against is
// pflag's, not the CLI's. The type would need a wrapper, or
// errors.As, or an Unwrap; each of those is more code than the
// string comparison for the same result.
func isPflagHelpRequested(err error) bool {
	return err != nil && err.Error() == errPflagHelpRequestedMsg
}

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
//
// # Why a constant and not a field on the root command
//
// The value is set on the root command (root.go) and read by
// Cobra's suggestion machinery. It is declared here so that the
// contract tests can reference it without constructing the root
// command, and so that a reader of this file sees the constant
// next to the constructor that produces the suggestion text.
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
//
// # Why the code is passed as Code, not string
//
// forgeerr.New's second parameter is forgeerr.Code. The registry
// constant CodeCLIUnknownCommand satisfies it directly. A
// string(...) cast would yield a string and would not satisfy the
// parameter; if such a cast is present, the constructor's
// parameter has reverted to string and the migration is
// incomplete.
func unknownCommandError(cmd *cobra.Command, name string) error {
	base := fmt.Sprintf("unknown command %q for %q", name, cmd.CommandPath())

	e := forgeerr.New(
		forgeerr.CategoryCLI,
		forgeerr.CodeCLIUnknownCommand,
		base,
	)

	suggestions := cmd.SuggestionsFor(name)
	if len(suggestions) == 0 {
		return e
	}

	rem := fmt.Sprintf("Did you mean %q.", suggestions[0])
	return e.WithRemediation(rem)
}
