// Package cli defines the Forge command-line interface.
//
// This file holds the constants and helpers that the error
// contract enforces: the invalid-command behaviour (WBS 7.3.1) and
// the error message format (WBS 7.3.2). The two contracts are
// related: the invalid-command cases are the primary source of
// errors the format applies to.
//
// # The error message format
//
// Every error that reaches the user is rendered into the frozen
// format:
//
//	Error: <message>
//	       <optional context line, indented 7 spaces>
//
//	Suggestion:
//	  <actionable remediation>
//
// The format's rules are documented in docs/cli-ux-spec.md "Error
// Message Format" and enforced by the tests in format_error_test.go
// and errors_test.go.
//
// # The three pieces
//
// The format has three pieces:
//
//  1. The first line: "Error: " followed by the message.
//  2. Zero or more context lines, each indented 7 spaces.
//  3. An optional Suggestion block.
//
// The first line is always present. The context lines are present
// only when the error carries context. The Suggestion block is
// present only when the error carries a suggestion.
//
// # Where the format is applied
//
// executeWithOptions (execute.go) is the single point where an
// error is rendered to stderr. It calls formatErrorMessage to
// produce the string and writes the string followed by a single
// newline. Every error path goes through this function; the
// format is applied uniformly.
//
// # No package-level mutable state
//
// The CLI package's contract test forbids package-level
// variables. This file declares only constants and functions.
package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
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
// trigger the "Did you mean this?" hint. A distance of 1 would miss
// two-character typos; a distance of 3 would produce suggestions
// for inputs that are not close to any command.
const suggestionsMinimumDistance = 2

// maxMessageLength is the maximum permitted length of the first
// line's message portion, in runes.
//
// # Why 80
//
// The message is rendered on one line. Terminal widths are
// commonly 80 columns; a longer message wraps, which makes the
// error block harder to read. The limit is on the message alone,
// not on the "Error: " prefix or any context line.
//
// # What happens when the limit is exceeded
//
// The builder does not truncate. A message over the limit is a
// programming error in the code that constructed the error; the
// builder reports it as a distinct line in the output ("Error:
// <message>" followed by a "context: message is N runes; limit is
// 80" line). The developer who sees this output knows the error
// they wrote is too long; they shorten the message. Truncating
// would hide the problem.
const maxMessageLength = 80

// messageIndent is the indentation applied to context lines, in
// spaces.
//
// # Why 7
//
// The first line begins with "Error: ", which is 7 characters
// ("E", "r", "r", "o", "r", ":", " "). Indenting context lines by
// 7 spaces aligns them under the first character of the message,
// so the context reads as a continuation of the message rather
// than as a separate block.
//
// The value is a constant so that a change to the prefix is a
// single-line change; the constant is the source of truth for the
// alignment.
const messageIndent = 7

// suggestionIndent is the indentation applied to the suggestion
// body, in spaces.
//
// # Why 2
//
// The suggestion's label ("Suggestion:") is at column 0; the
// body is indented 2 spaces. The two-space indent is the standard
// Markdown-like convention for a labeled block: the label is the
// header, the indented lines are the body.
const suggestionIndent = 2

// errorPrefix is the prefix on the first line of every error
// message.
//
// # Why the prefix is a constant
//
// The prefix appears in the first line of every error. Its
// spelling is part of the format contract; a change requires
// updating the docs, the tests, and the golden files in one
// commit. The constant makes the single definition explicit.
const errorPrefix = "Error: "

// suggestionLabel is the label of the suggestion block.
//
// # Why the label is a constant
//
// The label is part of the format contract. Its spelling is
// stable; the constant makes that stable spelling visible in
// code.
const suggestionLabel = "Suggestion:"

// errorContext is the metadata an error carries beyond its
// message and suggestion.
//
// # Why a struct
//
// The context is a small set of fields: the message, an optional
// list of context lines, and an optional suggestion. Grouping them
// into a struct makes the builder's signature small and stable;
// adding a field (for example, a file/line reference in WBS 10.0)
// is a one-line change to the struct, not a change to every call
// site.
//
// # Why the message is a string, not an error
//
// The builder takes a string for the message, not an error. The
// caller is responsible for extracting the message from whatever
// error it holds. This keeps the builder independent of the error
// hierarchy (ForgeError, *fmt.wrapError, custom types) and lets
// the caller decide which part of a wrapped error is the message.
type errorContext struct {
	// message is the first line's message, without the
	// "Error: " prefix. It must not end with a newline; the
	// builder adds the newline that terminates the first line.
	message string

	// contextLines are optional lines rendered below the first
	// line, each indented by messageIndent spaces. They provide
	// supplementary information (file paths, command names, the
	// offending value). An empty slice produces no context
	// lines.
	contextLines []string

	// suggestion is an optional actionable remediation. An empty
	// string suppresses the Suggestion block. The string is
	// rendered after the suggestionIndent spaces; the builder
	// adds the "Suggestion:" label.
	suggestion string
}

// formatErrorMessage renders ctx into the frozen error message
// format.
//
// # The format
//
// When ctx has a message, no context, and no suggestion:
//
//	Error: <message>
//
// When ctx has context lines:
//
//	Error: <message>
//	       <context-line-1>
//	       <context-line-2>
//
// When ctx has a suggestion:
//
//	Error: <message>
//
//	Suggestion:
//	  <suggestion>
//
// When ctx has all three:
//
//	Error: <message>
//	       <context-line-1>
//
//	Suggestion:
//	  <suggestion>
//
// The blank line before "Suggestion:" is the only blank line the
// builder produces. The output never begins or ends with a blank
// line.
//
// # Return value
//
// The function returns the formatted string. The caller is
// responsible for appending a trailing newline (the writer adds
// one) and for writing the string to stderr.
//
// The string does not end in a newline. The writer adds it. This
// split is the same as the one for unknownCommandError: the
// builder builds the message; the writer terminates the line.
//
// # Why the builder does not validate
//
// The builder does not enforce the maxMessageLength limit by
// truncating. It reports an over-length message as an additional
// context line, so that the developer who constructed the error
// sees the violation and fixes it. Truncation would hide the
// problem; the long message would appear to work but lose
// information.
//
// The builder does not enforce the "no trailing period"
// convention either. The convention is stylistic; the caller is
// responsible for following it. A caller that appends a period is
// reviewed; the builder does not police.
func formatErrorMessage(ctx errorContext) string {
	var b strings.Builder

	// First line.
	b.WriteString(errorPrefix)
	b.WriteString(ctx.message)
	b.WriteString("\n")

	// Context lines.
	for _, line := range ctx.contextLines {
		b.WriteString(strings.Repeat(" ", messageIndent))
		b.WriteString(line)
		b.WriteString("\n")
	}

	// Suggestion block.
	if ctx.suggestion != "" {
		b.WriteString("\n")
		b.WriteString(suggestionLabel)
		b.WriteString("\n")
		b.WriteString(strings.Repeat(" ", suggestionIndent))
		b.WriteString(ctx.suggestion)
		b.WriteString("\n")
	}

	// Strip the trailing newline. The writer will add one.
	return strings.TrimRight(b.String(), "\n")
}

// checkMessageLength reports whether a message exceeds
// maxMessageLength. If it does, the function returns a context line
// that names the violation; the caller appends the context line to
// the error's context.
//
// # Why this is a separate function
//
// The check is not part of formatErrorMessage's output path; the
// builder does not call it. A caller that wants to enforce the
// limit (for example, a development-mode assertion) calls this
// function and appends its result to the context lines. A caller
// that does not care about the limit skips it.
//
// The split keeps the builder's behaviour predictable: the builder
// renders what it is given. The check is a separate concern.
func checkMessageLength(message string) (string, bool) {
	if len([]rune(message)) <= maxMessageLength {
		return "", true
	}
	return fmt.Sprintf("message is %d runes; limit is %d",
		len([]rune(message)), maxMessageLength), false
}

// =============================================================================
// Suggestion attachment
// =============================================================================

// suggestedError wraps an error with a suggestion.
//
// # Why a wrapper
//
// The error's message and its suggestion are separate concerns.
// The message is what the error says; the suggestion is what the
// caller should do about it. Some errors have a message but no
// suggestion (an internal invariant violation, for example); some
// have both (a user typo, for example).
//
// The wrapper attaches a suggestion to any error without changing
// the error's underlying type. The error's Error() method returns
// the original error's message; the Suggestion() method returns
// the suggestion. The caller can test for the wrapper with
// errors.As.
//
// # Why not a struct field on ForgeError
//
// ForgeError (WBS 10.0) will carry the suggestion as a field. The
// wrapper is a Phase 2 stand-in: it lets a caller attach a
// suggestion to a plain error (from Cobra, from the standard
// library) without forcing every error to be a ForgeError. When
// WBS 10.0 lands, the wrapper's role is absorbed by ForgeError's
// field; the wrapper's public surface (the Suggestion() method)
// can remain as the interface.
type suggestedError struct {
	cause      error
	suggestion string
}

// Error implements the error interface. It returns the original
// error's message unchanged.
func (e *suggestedError) Error() string {
	return e.cause.Error()
}

// Unwrap returns the wrapped error. It lets errors.Is and errors.As
// see through the wrapper.
func (e *suggestedError) Unwrap() error {
	return e.cause
}

// Suggestion returns the suggestion attached to the error.
func (e *suggestedError) Suggestion() string {
	return e.suggestion
}

// withSuggestion returns an error that wraps cause and carries
// suggestion. The returned error's message is cause.Error(); its
// Suggestion() method returns the suggestion.
//
// # Why an empty suggestion is allowed
//
// Passing an empty suggestion is equivalent to returning cause
// unchanged. The function does not construct a wrapper for an
// empty suggestion; a caller that conditionally attaches a
// suggestion does not need to check for the empty case.
func withSuggestion(cause error, suggestion string) error {
	if suggestion == "" {
		return cause
	}
	return &suggestedError{cause: cause, suggestion: suggestion}
}

// suggestionOf extracts the suggestion from an error, if any. It
// returns the empty string if the error does not carry a
// suggestion.
//
// # How the extraction works
//
// The function walks the error chain with errors.As, looking for
// an error that implements the Suggestion() string method. The
// first such error's suggestion is returned. If no error in the
// chain has a suggestion, the function returns the empty string.
//
// The interface check is a type assertion; the function does not
// depend on the concrete suggestedError type. A future error type
// that implements Suggestion() string is recognised the same way.
func suggestionOf(err error) string {
	if err == nil {
		return ""
	}
	var s interface{ Suggestion() string }
	if errors.As(err, &s) {
		return s.Suggestion()
	}
	return ""
}

// =============================================================================
// Unknown command error
// =============================================================================

// unknownCommandError constructs an error for an unknown command,
// including Cobra's suggestion mechanism.
//
// # Why this function exists
//
// Cobra's suggestion mechanism runs during Cobra's own dispatch,
// before the root command's RunE. When the root command has a RunE
// (which Forge's root command must, to print help on no args),
// Cobra passes the unknown positional arguments to RunE instead of
// producing its own error. The suggestion mechanism is bypassed.
//
// The fix is to construct the error in RunE with the same
// suggestion logic Cobra uses. Cobra exposes the suggestion
// computation as `SuggestionsFor(typedName)` on the command; the
// function calls it and formats the result.
//
// # The error's message
//
// The error's message is "unknown command %q for %q". The
// suggestion (if any) is attached via withSuggestion; the
// "Did you mean this?" block is no longer part of the message. It
// is rendered by formatErrorMessage as a context block, and the
// suggestion itself is rendered as the Suggestion block's body.
//
// # Why the message does not include the "Did you mean this?"
// # block
//
// The format contract specifies the shape of the error message.
// The message is one line; the suggestion is a separate block.
// Putting the "Did you mean this?" text inside the message would
// put multi-line content in a single-line field, violating the
// format.
//
// The suggestion is rendered by the format builder. The builder
// does not know about "Did you mean this?"; the caller (this
// function) attaches the suggestion and lets the builder render
// it.
//
// # The trailing newline
//
// The function returns an error whose message does not end in a
// newline. The caller (executeWithOptions) appends a single
// newline when it writes the error to stderr. If this function
// included a trailing newline, the output would end in two
// newlines, producing a blank line that the invalid-command
// contract forbids.
func unknownCommandError(cmd *cobra.Command, name string) error {
	suggestions := cmd.SuggestionsFor(name)
	base := fmt.Errorf("unknown command %q for %q", name, cmd.CommandPath())

	suggestion := "Run 'forge --help' for usage."
	if len(suggestions) > 0 {
		suggestion = fmt.Sprintf("Did you mean %s? %s",
			quoteList(suggestions), suggestion)
	}
	return withSuggestion(base, suggestion)
}

// quoteList renders a list of strings as a comma-separated
// sequence of double-quoted tokens.
//
// # Why the helper exists
//
// The "Did you mean ...?" hint names one or more commands. The
// commands are quoted so that a reader can tell them apart from
// the surrounding prose. A single suggestion is
// `"version"`; two suggestions are `"version", "config"`.
//
// # Boundary cases
//
// An empty list returns the empty string. The caller checks for
// the empty case before calling; the function does not guard.
func quoteList(items []string) string {
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = fmt.Sprintf("%q", item)
	}
	return strings.Join(quoted, ", ")
}
