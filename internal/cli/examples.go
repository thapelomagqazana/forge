// Package cli defines the Forge command-line interface.
//
// This file holds the constants and helpers that the Examples
// convention (WBS 7.2.2) enforces. The convention specifies when a
// command must declare an Example and what the Example's content
// must look like.
//
// # The convention
//
// A command requires an Example if any of the following holds:
//
//   - It takes positional arguments. The Use field contains a
//     bracketed placeholder ("[name]" or "<name>").
//   - It has command-specific flags. The flag set contains a flag
//     that is neither a global flag nor the auto-registered --help
//     flag.
//   - It has non-obvious behaviour. This condition is not
//     mechanically checkable; it is enforced by review. The two
//     existing commands with non-obvious behaviour (version with
//     its --format flag, config with its placeholder status) have
//     Examples.
//
// A command that has none of these conditions may omit the Example
// field; its invocation is self-explanatory from the Usage line.
//
// When present, the Example's content must satisfy:
//
//   - Each line starts with two spaces.
//   - Each line contains exactly one command.
//   - No line starts with the "$" prefix.
//   - Values are realistic, not placeholders like "foo" or "bar".
//   - The primary use case comes first, then common variations.
//   - The number of lines is between 2 and 5.
//
// # Where the checks live
//
// The rules split into two groups:
//
//   - Presence: requiresExample (in metadata.go) decides whether
//     the command must declare an Example. The rule is shared with
//     the metadata contract (WBS 7.2.1) and is not duplicated here.
//   - Format: the helpers in this file check the content of the
//     Example field when it is present.
//
// # No package-level mutable state
//
// The CLI package's contract test forbids package-level
// variables. This file declares only constants and functions.
package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

// minExampleLines is the minimum number of example lines a command
// with an Example field may declare.
//
// # Why 2
//
// An Example with a single line does not show a variation; it
// duplicates the command's primary invocation, which the Usage line
// already shows. Two lines are the minimum that shows the primary
// case plus one variation.
const minExampleLines = 2

// maxExampleLines is the maximum number of example lines a command
// with an Example field may declare.
//
// # Why 5
//
// An Example with more than five lines is a tutorial, not a help
// text. The Long field or the CLI UX spec is the place for a
// tutorial. Five lines are enough to show the primary case and up
// to four variations.
const maxExampleLines = 5

// exampleLines returns the non-empty lines of cmd's Example field,
// trimmed of leading whitespace.
//
// # Why trimming matters
//
// Cobra's help output renders the Example field verbatim. The
// convention requires two leading spaces on each line, but the
// trimming here is for the checks in this file, not for the
// rendered output. The rendered output preserves the original
// indentation; the checks operate on the trimmed content.
//
// An empty Example field returns an empty slice. The caller decides
// whether that is a violation (see requiresExample in metadata.go).
func exampleLines(cmd *cobra.Command) []string {
	if cmd.Example == "" {
		return nil
	}
	var lines []string
	for _, line := range strings.Split(cmd.Example, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

// exampleLineCount returns the number of non-empty lines in cmd's
// Example field.
//
// # Why non-empty lines
//
// The convention counts example lines, not text lines. A blank
// line between two example groups is not an example; it is
// whitespace. Counting only non-empty lines makes the 2-to-5 range
// meaningful.
func exampleLineCount(cmd *cobra.Command) int {
	return len(exampleLines(cmd))
}

// exampleLinesHaveTwoSpaceIndent reports whether every non-empty
// line in cmd's Example field starts with at least two spaces.
//
// # Why "at least two"
//
// The convention requires two spaces. A line with more than two
// spaces is unusual but not a violation; the extra spaces are
// preserved in the rendered output. The check requires at least
// two, not exactly two.
//
// # Why non-empty lines only
//
// A blank line has no indentation to check. The check skips blank
// lines; the check for blank-line presence is a separate concern.
func exampleLinesHaveTwoSpaceIndent(cmd *cobra.Command) bool {
	if cmd.Example == "" {
		return true
	}
	for _, line := range strings.Split(cmd.Example, "\n") {
		// Skip blank lines. A line that is all whitespace is
		// treated as blank.
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.HasPrefix(line, "  ") {
			return false
		}
	}
	return true
}

// exampleLinesHaveNoDollarPrefix reports whether no non-empty line
// in cmd's Example field starts with "$".
//
// # Why no "$"
//
// The "$" prefix is a convention for shell transcripts. The help
// output is not a transcript; the reader copies the commands
// directly. A "$" prefix makes the copy non-pasteable.
//
// The check looks for "$" after the leading whitespace, so a line
// like "  $ forge version" is flagged. A line like "  forge
// --format=$format" is not flagged; the "$" is not at the start of
// the command.
func exampleLinesHaveNoDollarPrefix(cmd *cobra.Command) bool {
	if cmd.Example == "" {
		return true
	}
	for _, line := range strings.Split(cmd.Example, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "$ ") || trimmed == "$" {
			return false
		}
	}
	return true
}

// exampleLinesHaveNoBlankLines reports whether cmd's Example field
// contains no blank lines between example lines.
//
// # Why no blank lines
//
// A blank line between examples visually separates them, but the
// convention counts example lines and expects a contiguous block.
// A blank line is treated as a formatting error.
//
// # Boundary: trailing newline
//
// A trailing newline at the end of the Example field is not a blank
// line; it is the terminator of the last line. The check tolerates
// one trailing newline. Two or more consecutive newlines produce a
// blank line, which the check flags.
func exampleLinesHaveNoBlankLines(cmd *cobra.Command) bool {
	if cmd.Example == "" {
		return true
	}
	// A trailing newline is normal; strip it before checking.
	example := strings.TrimRight(cmd.Example, "\n")
	// Look for two consecutive newlines.
	return !strings.Contains(example, "\n\n")
}

// exampleValuesAreRealistic is a placeholder for a future
// implementation.
//
// # Why this is a placeholder
//
// The rule "values are realistic, not foo/bar" is not mechanically
// checkable. A regular expression cannot distinguish "payments-api"
// from "my-project" without a list of accepted values, and no such
// list is stable across commands.
//
// The rule is enforced by review. A reviewer who sees an Example
// with "foo" or "bar" asks the contributor to use a plausible
// value. The check is documented here so that a future contributor
// who reads the rules sees the placeholder and knows where to add
// the check if a mechanical heuristic becomes practical.
//
// # When this might become checkable
//
// If the project adopts a convention for example values (for
// example, all example project names end in "-api" or "-cli"), the
// check could enforce the convention. Until then, the check is
// empty.
func exampleValuesAreRealistic(cmd *cobra.Command) bool {
	// No mechanical check is possible. Return true unconditionally.
	return true
}
