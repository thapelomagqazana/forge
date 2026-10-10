// Package cli contains white-box tests for the CLI package.
//
// This file pins the error message format (WBS 7.3.2). The format
// is a small set of rules: a first line, optional context lines,
// an optional suggestion block. The tests in this file exercise
// the builder in isolation; the errors_test.go matrix exercises
// the format against real invocations.
//
// # Why unit tests for the builder
//
// The builder is the single point where the format is applied. A
// future change to the format is a change to the builder; the
// tests here pin the builder's behaviour directly, with small
// inputs and precise assertions. The matrix test in errors_test.go
// pins the format's application to the seven invalid-command
// cases; the two layers are complementary.
package cli

import (
	"strings"
	"testing"
)

// =============================================================================
// The minimal case
// =============================================================================

// TestFormatErrorMessage_MinimalError verifies that a context with
// only a message produces a single line.
//
// The output is "Error: <message>" with no trailing newline. The
// caller (executeWithOptions) adds the trailing newline when it
// writes to stderr.
func TestFormatErrorMessage_MinimalError(t *testing.T) {
	t.Parallel()

	ctx := errorContext{
		message: "something went wrong",
	}
	got := formatErrorMessage(ctx)
	want := "Error: something went wrong"

	if got != want {
		t.Errorf("output mismatch:\ngot:  %q\nwant: %q", got, want)
	}
}

// =============================================================================
// The suggestion case
// =============================================================================

// TestFormatErrorMessage_WithSuggestion verifies that a context
// with a suggestion produces both blocks, separated by a blank
// line, with the suggestion's body indented two spaces.
func TestFormatErrorMessage_WithSuggestion(t *testing.T) {
	t.Parallel()

	ctx := errorContext{
		message:    "unknown command \"foobar\" for \"forge\"",
		suggestion: "Run 'forge --help' for usage.",
	}
	got := formatErrorMessage(ctx)
	want := "Error: unknown command \"foobar\" for \"forge\"\n" +
		"\n" +
		"Suggestion:\n" +
		"  Run 'forge --help' for usage."

	if got != want {
		t.Errorf("output mismatch:\ngot:  %q\nwant: %q", got, want)
	}
}

// =============================================================================
// The context case
// =============================================================================

// TestFormatErrorMessage_WithContext verifies that context lines
// are indented by messageIndent spaces.
func TestFormatErrorMessage_WithContext(t *testing.T) {
	t.Parallel()

	ctx := errorContext{
		message: "unknown command",
		contextLines: []string{
			"command: verison",
			"closest:  version",
		},
	}
	got := formatErrorMessage(ctx)
	want := "Error: unknown command\n" +
		"       command: verison\n" +
		"       closest:  version"

	if got != want {
		t.Errorf("output mismatch:\ngot:  %q\nwant: %q", got, want)
	}
}

// =============================================================================
// All three pieces
// =============================================================================

// TestFormatErrorMessage_AllThree verifies the full format: message,
// context lines, and a suggestion block.
func TestFormatErrorMessage_AllThree(t *testing.T) {
	t.Parallel()

	ctx := errorContext{
		message: "unknown command \"verison\" for \"forge\"",
		contextLines: []string{
			"did you mean:",
			"  version",
		},
		suggestion: "Run 'forge --help' for usage.",
	}
	got := formatErrorMessage(ctx)
	want := "Error: unknown command \"verison\" for \"forge\"\n" +
		"       did you mean:\n" +
		"         version\n" +
		"\n" +
		"Suggestion:\n" +
		"  Run 'forge --help' for usage."

	if got != want {
		t.Errorf("output mismatch:\ngot:  %q\nwant: %q", got, want)
	}
}

// =============================================================================
// Edge cases
// =============================================================================

// TestFormatErrorMessage_NoTrailingNewline verifies that the
// builder's output does not end with a newline.
//
// The output's last byte is the last character of the last line.
// The caller adds the trailing newline; the builder does not.
func TestFormatErrorMessage_NoTrailingNewline(t *testing.T) {
	t.Parallel()

	ctx := errorContext{
		message:    "test",
		suggestion: "test",
	}
	got := formatErrorMessage(ctx)

	if strings.HasSuffix(got, "\n") {
		t.Errorf("output ends with a newline: %q", got)
	}
}

// TestFormatErrorMessage_NoLeadingBlankLine verifies that the
// builder's output does not begin with a blank line.
func TestFormatErrorMessage_NoLeadingBlankLine(t *testing.T) {
	t.Parallel()

	ctx := errorContext{message: "test"}
	got := formatErrorMessage(ctx)

	if strings.HasPrefix(got, "\n") {
		t.Errorf("output begins with a newline: %q", got)
	}
}

// TestFormatErrorMessage_NoAnsiEscapes verifies that the builder's
// output contains no ANSI escape bytes.
func TestFormatErrorMessage_NoAnsiEscapes(t *testing.T) {
	t.Parallel()

	ctx := errorContext{
		message:      "test",
		contextLines: []string{"context"},
		suggestion:   "suggestion",
	}
	got := formatErrorMessage(ctx)

	if idx := strings.IndexByte(got, 0x1b); idx >= 0 {
		t.Errorf("output contains an ANSI escape at byte %d: %q",
			idx, got[idx:])
	}
}

// TestFormatErrorMessage_EmptySuggestion verifies that an empty
// suggestion suppresses the Suggestion block.
func TestFormatErrorMessage_EmptySuggestion(t *testing.T) {
	t.Parallel()

	ctx := errorContext{
		message:    "test",
		suggestion: "",
	}
	got := formatErrorMessage(ctx)

	if strings.Contains(got, "Suggestion:") {
		t.Errorf("output contains a Suggestion block: %q", got)
	}
}

// TestFormatErrorMessage_EmptyContext verifies that an empty context
// slice produces no context lines.
func TestFormatErrorMessage_EmptyContext(t *testing.T) {
	t.Parallel()

	ctx := errorContext{
		message:      "test",
		contextLines: nil,
	}
	got := formatErrorMessage(ctx)
	want := "Error: test"

	if got != want {
		t.Errorf("output mismatch:\ngot:  %q\nwant: %q", got, want)
	}
}

// TestFormatErrorMessage_SuggestionIsIndented verifies the
// suggestion's body is indented by suggestionIndent spaces.
func TestFormatErrorMessage_SuggestionIsIndented(t *testing.T) {
	t.Parallel()

	ctx := errorContext{
		message:    "test",
		suggestion: "run the command",
	}
	got := formatErrorMessage(ctx)
	want := "Error: test\n\nSuggestion:\n  run the command"

	if got != want {
		t.Errorf("output mismatch:\ngot:  %q\nwant: %q", got, want)
	}
}

// =============================================================================
// The checkMessageLength helper
// =============================================================================

// TestCheckMessageLength_WithinLimit verifies that a message
// within the limit is reported as passing.
func TestCheckMessageLength_WithinLimit(t *testing.T) {
	t.Parallel()

	message := strings.Repeat("a", maxMessageLength)
	_, ok := checkMessageLength(message)
	if !ok {
		t.Errorf("message of %d runes should pass the check",
			len(message))
	}
}

// TestCheckMessageLength_AtLimit verifies that a message exactly at
// the limit passes.
func TestCheckMessageLength_AtLimit(t *testing.T) {
	t.Parallel()

	message := strings.Repeat("a", maxMessageLength)
	_, ok := checkMessageLength(message)
	if !ok {
		t.Errorf("message of exactly %d runes should pass the check",
			maxMessageLength)
	}
}

// TestCheckMessageLength_OverLimit verifies that a message over the
// limit fails and produces a context line.
func TestCheckMessageLength_OverLimit(t *testing.T) {
	t.Parallel()

	message := strings.Repeat("a", maxMessageLength+1)
	line, ok := checkMessageLength(message)
	if ok {
		t.Errorf("message of %d runes should fail the check",
			len(message))
	}
	if line == "" {
		t.Errorf("over-limit message should produce a context line")
	}
}

// TestCheckMessageLength_CountsRunesNotBytes verifies that the
// length is measured in runes, not bytes.
func TestCheckMessageLength_CountsRunesNotBytes(t *testing.T) {
	t.Parallel()

	// "α" is two bytes in UTF-8 but one rune. A message of 80
	// alpha characters is 80 runes and 160 bytes. The check must
	// pass it.
	message := strings.Repeat("α", maxMessageLength)
	_, ok := checkMessageLength(message)
	if !ok {
		t.Errorf("message of %d runes should pass the check "+
			"(byte length is %d)",
			len([]rune(message)), len(message))
	}
}
