// Package cli contains white-box tests for the CLI package.
//
// This file pins the Examples convention (WBS 7.2.2). The
// convention specifies when a command requires an Example and what
// the Example's content must look like. Each test iterates the
// registry and asserts one rule.
//
// # Relationship to metadata_test.go
//
// The metadata contract (WBS 7.2.1) has a test that asserts the
// presence of an Example for qualifying commands. That rule is
// shared with this file; it lives in metadata_test.go because it is
// part of the metadata contract. This file adds the format rules:
// indentation, no "$" prefix, line count, no blank lines.
//
// The two files iterate the same registry. A command that violates
// the presence rule fails in metadata_test.go; a command that
// violates a format rule fails here.
//
// # How the tests iterate
//
// Each test iterates allRegisteredCommands() (defined in
// help_content.go). The iteration includes hidden commands; the
// convention applies to them the same way it applies to visible
// ones.
package cli

import (
	"strings"
	"testing"
)

// =============================================================================
// Indentation
// =============================================================================

// TestExamples_TwoSpaceIndent verifies that every non-empty line in
// every command's Example field starts with at least two spaces.
//
// # What the test catches
//
// A future command whose Example field omits the two-space
// indentation. The failure quotes the offending line.
func TestExamples_TwoSpaceIndent(t *testing.T) {
	t.Parallel()

	for _, cmd := range allRegisteredCommands() {
		cmd := cmd
		name := commandName(cmd.Use)
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if cmd.Example == "" {
				return
			}
			if !exampleLinesHaveTwoSpaceIndent(cmd) {
				t.Errorf("Example line lacks two-space indent:\n%q", cmd.Example)
			}
		})
	}
}

// =============================================================================
// No "$" prefix
// =============================================================================

// TestExamples_NoDollarPrefix verifies that no line in any command's
// Example field starts with the "$" prefix.
//
// # What the test catches
//
// A future command whose Example uses shell-transcript style ("$
// forge version"). The failure quotes the offending line.
func TestExamples_NoDollarPrefix(t *testing.T) {
	t.Parallel()

	for _, cmd := range allRegisteredCommands() {
		cmd := cmd
		name := commandName(cmd.Use)
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if cmd.Example == "" {
				return
			}
			if !exampleLinesHaveNoDollarPrefix(cmd) {
				t.Errorf("Example line starts with \"$\":\n%q", cmd.Example)
			}
		})
	}
}

// =============================================================================
// Line count
// =============================================================================

// TestExamples_LineCount verifies that every command's Example field
// declares between minExampleLines and maxExampleLines non-empty
// lines, when the field is present.
//
// # What the test catches
//
// A future command whose Example has one line (too few to show a
// variation) or six or more lines (too many for help text). The
// failure quotes the count and the lines.
func TestExamples_LineCount(t *testing.T) {
	t.Parallel()

	for _, cmd := range allRegisteredCommands() {
		cmd := cmd
		name := commandName(cmd.Use)
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if cmd.Example == "" {
				return
			}
			n := exampleLineCount(cmd)
			if n < minExampleLines {
				t.Errorf("Example has %d lines; minimum is %d:\n%q",
					n, minExampleLines, cmd.Example)
			}
			if n > maxExampleLines {
				t.Errorf("Example has %d lines; maximum is %d:\n%q",
					n, maxExampleLines, cmd.Example)
			}
		})
	}
}

// =============================================================================
// No blank lines
// =============================================================================

// TestExamples_NoBlankLines verifies that no command's Example field
// contains a blank line between examples.
//
// # What the test catches
//
// A future command whose Example uses blank lines to separate
// groups. The failure quotes the field.
//
// # Boundary: trailing newline
//
// A single trailing newline at the end of the field is not a
// blank line. The check strips trailing newlines before looking
// for interior blank lines. Two consecutive newlines in the
// interior produce a blank line, which the check flags.
func TestExamples_NoBlankLines(t *testing.T) {
	t.Parallel()

	for _, cmd := range allRegisteredCommands() {
		cmd := cmd
		name := commandName(cmd.Use)
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if cmd.Example == "" {
				return
			}
			if !exampleLinesHaveNoBlankLines(cmd) {
				t.Errorf("Example contains a blank line:\n%q", cmd.Example)
			}
		})
	}
}

// =============================================================================
// Content — structural check for the "two examples per command"
// convention
// =============================================================================

// TestExamples_AtLeastTwoCommands verifies that every command's
// Example field contains at least two non-empty lines.
//
// This test overlaps with TestExamples_LineCount; it is kept
// because it produces a more specific failure when the Example
// has exactly one line. The line-count test's failure message
// quotes both the minimum and maximum; this test's message
// states the rule directly.
//
// # What the test catches
//
// A future command whose Example declares only the primary
// invocation. The failure names the command.
func TestExamples_AtLeastTwoCommands(t *testing.T) {
	t.Parallel()

	for _, cmd := range allRegisteredCommands() {
		cmd := cmd
		name := commandName(cmd.Use)
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if cmd.Example == "" {
				return
			}
			if exampleLineCount(cmd) < 2 {
				t.Errorf("Example has fewer than 2 lines:\n%q", cmd.Example)
			}
		})
	}
}

// =============================================================================
// Content — no placeholder values
// =============================================================================

// TestExamples_NoObviousPlaceholders verifies that no command's
// Example field contains an obvious placeholder value.
//
// # What the test catches
//
// A future command whose Example uses "foo", "bar", "baz",
// "example", "xxx", or similar placeholder values. The failure
// quotes the offending line.
//
// # Why the check is narrow
//
// The rule "values are realistic" is not mechanically checkable in
// general. This test checks a small set of obvious placeholders
// ("foo", "bar", "baz") that a reviewer would flag. A more
// thorough check would require a list of accepted values, which
// the project does not maintain.
//
// The check uses a simple case-insensitive substring match. A
// value that contains "foo" as part of a legitimate word (for
// example, a project name "foo-bar-baz-api") is flagged; the
// false positive is acceptable because the check is a forcing
// function, not a policy enforcer.
func TestExamples_NoObviousPlaceholders(t *testing.T) {
	t.Parallel()

	placeholders := []string{"foo", "bar", "baz"}

	for _, cmd := range allRegisteredCommands() {
		cmd := cmd
		name := commandName(cmd.Use)
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if cmd.Example == "" {
				return
			}
			lower := strings.ToLower(cmd.Example)
			for _, placeholder := range placeholders {
				if strings.Contains(lower, placeholder) {
					t.Errorf("Example contains placeholder value %q:\n%q",
						placeholder, cmd.Example)
				}
			}
		})
	}
}
