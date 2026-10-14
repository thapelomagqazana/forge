// Package forgeerr — user-facing format tests.
//
// Every test is named after the acceptance criterion (AC1–AC10)
// it verifies. The byte-level rules F1–F11 are each asserted by a
// dedicated test. AC10 (documentation) is checked by a separate
// documentation test.
//
// Test categories:
//
//   - ATDD:      AC-mapped tests
//   - Golden:    byte-exact comparison against committed files
//   - Positive:  valid inputs
//   - Negative:  invalid inputs
//   - Edge:      nil error, non-*Error error, wrapped *Error
//   - Corner:    many context keys, one key, empty values
//   - BND:       line lengths, trailing whitespace
//   - NF:        determinism, concurrency
//   - Fuzz:      invariant preservation
//
// # Shared test helpers
//
// goldenText, the -update flag, and the package-level fixture
// functions live in codes_flags_test.go. This file consumes them;
// it does not define them. A duplicate definition of goldenText
// in this file would fail to compile when both files are built
// together, and moving the shared helpers into a single file is
// what keeps the -update flag's registration point unambiguous.
//
// # Type-level contract
//
// Every call to forgeerr.New in this file passes the code as
// forgeerr.Code, not as string. The constructors' signature is
// New(category, code Code, message). The Code parameter is what
// makes the registry a closed set at compile time: a bare string
// literal in the code position does not convert implicitly, so a
// caller who invents a new code cannot compile the call.
package forgeerr

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// --- Golden file tests (AC1) ------------------------------------------

// TestAC1_GoldenFiles compares Format's output for four canonical
// errors against committed golden files. The comparison is exact:
// the golden file must match the output byte for byte, including
// the trailing newline.
//
// The four golden files cover the four cases from WBS 10.4.1:
//
//   - minimal.golden.txt           no context, no remediation
//   - with-context.golden.txt      context, no remediation
//   - with-remediation.golden.txt  no context, remediation
//   - full.golden.txt              context and remediation
//
// Regenerate the golden files with:
//
//	go test -run TestAC1_GoldenFiles -update
//
// from inside internal/forgeerr/. The -update flag is registered
// once for the package in codes_flags_test.go.
func TestAC1_GoldenFiles(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  *Error
		file string
	}{
		{
			name: "minimal",
			err:  New(CategoryCLI, CodeCLIUnknownCommand, `unknown command "foobar"`),
			file: "minimal.golden.txt",
		},
		{
			name: "with-context",
			err: New(CategoryConfig, CodeConfigMalformed, "invalid YAML in config.yaml").
				WithContext("file", "config.yaml").
				WithContext("line", "12"),
			file: "with-context.golden.txt",
		},
		{
			name: "with-remediation",
			err: New(CategoryCLI, CodeCLIUnknownCommand, `unknown command "foobar"`).
				WithRemediation("Run 'forge --help' for usage."),
			file: "with-remediation.golden.txt",
		},
		{
			name: "full",
			err: New(CategoryConfig, CodeConfigMalformed, "invalid YAML in config.yaml").
				WithContext("file", "config.yaml").
				WithContext("line", "12").
				WithRemediation("Fix the YAML syntax at line 12."),
			file: "full.golden.txt",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := Format(tc.err, Options{})
			path := filepath.Join("testdata", "format", tc.file)
			want := goldenText(t, path, got)

			if got != want {
				t.Fatalf(
					"Format output does not match %s.\n"+
						"Run `go test -run TestAC1_GoldenFiles -update` from internal/forgeerr/ to regenerate.\n"+
						"--- got ---\n%s\n--- want ---\n%s",
					tc.file, got, want,
				)
			}
		})
	}
}

// --- AC2: Error: prefix on first line ---------------------------------

// TestAC2_ErrorPrefix asserts that the first line of the output is
// "Error: " followed by the message.
func TestAC2_ErrorPrefix(t *testing.T) {
	t.Parallel()

	e := New(CategoryCLI, CodeCLIUnknownCommand, "unknown command")
	got := Format(e, Options{})

	firstLine, _, _ := strings.Cut(got, "\n")
	if !strings.HasPrefix(firstLine, "Error: ") {
		t.Fatalf("first line = %q, want prefix %q", firstLine, "Error: ")
	}
	if firstLine != "Error: unknown command" {
		t.Fatalf("first line = %q, want %q", firstLine, "Error: unknown command")
	}
}

// --- AC3: Code: line with two-space indent ----------------------------

// TestAC3_CodeLine asserts that the second line is "Code:  " (two
// spaces) followed by the code.
func TestAC3_CodeLine(t *testing.T) {
	t.Parallel()

	e := New(CategoryCLI, CodeCLIUnknownCommand, "unknown command")
	got := Format(e, Options{})

	lines := strings.Split(got, "\n")
	if len(lines) < 2 {
		t.Fatalf("output has %d lines, want at least 2", len(lines))
	}
	second := lines[1]
	if !strings.HasPrefix(second, "Code:  ") {
		t.Fatalf("second line = %q, want prefix %q", second, "Code:  ")
	}
	if !strings.HasSuffix(second, string(CodeCLIUnknownCommand)) {
		t.Fatalf("second line = %q, want suffix %q", second, CodeCLIUnknownCommand)
	}
}

// --- AC4: context lines indented 7 spaces -----------------------------

// TestAC4_ContextIndent asserts that every context line begins
// with exactly seven spaces.
func TestAC4_ContextIndent(t *testing.T) {
	t.Parallel()

	e := New(CategoryConfig, CodeConfigMalformed, "invalid YAML").
		WithContext("file", "config.yaml").
		WithContext("line", "12")
	got := Format(e, Options{})

	lines := strings.Split(got, "\n")
	for i := 2; i < len(lines); i++ {
		line := lines[i]
		if line == "" {
			break
		}
		if !strings.HasPrefix(line, "       ") {
			t.Fatalf("line %d = %q, want 7-space indent", i, line)
		}
		if strings.HasPrefix(line, "        ") {
			t.Fatalf("line %d = %q, want exactly 7 spaces", i, line)
		}
	}
}

// --- AC5: keys sorted alphabetically ----------------------------------

// TestAC5_KeysSortedAlphabetically asserts that context lines
// appear in lexicographic order by key, regardless of the order
// in which the keys were added.
func TestAC5_KeysSortedAlphabetically(t *testing.T) {
	t.Parallel()

	// Insert keys in reverse-alphabetical order.
	e := New(CategoryConfig, CodeConfigMalformed, "x").
		WithContext("zeta", "1").
		WithContext("yotta", "2").
		WithContext("alpha", "3")

	got := Format(e, Options{})

	alphaIdx := strings.Index(got, "       alpha: 3")
	yottaIdx := strings.Index(got, "       yotta: 2")
	zetaIdx := strings.Index(got, "       zeta: 1")

	if alphaIdx == -1 || yottaIdx == -1 || zetaIdx == -1 {
		t.Fatalf("missing context lines in output:\n%s", got)
	}
	if !(alphaIdx < yottaIdx && yottaIdx < zetaIdx) {
		t.Fatalf(
			"context lines not in alphabetical order: "+
				"alpha at %d, yotta at %d, zeta at %d",
			alphaIdx, yottaIdx, zetaIdx,
		)
	}
}

// --- AC6: blank line before Suggestion --------------------------------

// TestAC6_BlankLineBeforeSuggestion asserts that the line before
// "Suggestion:" is empty.
func TestAC6_BlankLineBeforeSuggestion(t *testing.T) {
	t.Parallel()

	e := New(CategoryCLI, CodeCLIUnknownCommand, "unknown command").
		WithRemediation("Run 'forge --help' for usage.")
	got := Format(e, Options{})

	lines := strings.Split(got, "\n")
	for i, line := range lines {
		if line == "Suggestion:" {
			if i == 0 {
				t.Fatal("Suggestion: is the first line")
			}
			if lines[i-1] != "" {
				t.Fatalf("line before Suggestion: = %q, want empty", lines[i-1])
			}
			return
		}
	}
	t.Fatalf("output does not contain Suggestion: line:\n%s", got)
}

// --- AC7: Suggestion: line mandatory when remediation exists ---------

// TestAC7_SuggestionPresent asserts that "Suggestion:" appears
// when and only when a remediation is set.
func TestAC7_SuggestionPresent(t *testing.T) {
	t.Parallel()

	withRem := New(CategoryCLI, CodeCLIUnknownCommand, "x").
		WithRemediation("Run 'forge --help'.")
	if !strings.Contains(Format(withRem, Options{}), "Suggestion:") {
		t.Fatal("Suggestion: missing when remediation is set")
	}

	withoutRem := New(CategoryCLI, CodeCLIUnknownCommand, "x")
	if strings.Contains(Format(withoutRem, Options{}), "Suggestion:") {
		t.Fatal("Suggestion: present when remediation is not set")
	}
}

// --- AC8: no trailing whitespace --------------------------------------

// TestAC8_NoTrailingWhitespace asserts that no line in the output
// ends with a space, tab, or carriage return.
func TestAC8_NoTrailingWhitespace(t *testing.T) {
	t.Parallel()

	cases := []*Error{
		New(CategoryCLI, CodeCLIUnknownCommand, "x"),
		New(CategoryConfig, CodeConfigMalformed, "x").
			WithContext("k", "v"),
		New(CategoryCLI, CodeCLIUnknownCommand, "x").
			WithRemediation("Fix it."),
		New(CategoryConfig, CodeConfigMalformed, "x").
			WithContext("a", "1").
			WithContext("b", "2").
			WithRemediation("Fix it."),
	}
	for i, e := range cases {
		got := Format(e, Options{})
		for j, line := range strings.Split(got, "\n") {
			trimmed := strings.TrimRight(line, " \t\r")
			if trimmed != line {
				t.Errorf("case %d line %d has trailing whitespace: %q", i, j, line)
			}
		}
	}
}

// --- AC9: no ANSI escapes ---------------------------------------------

// TestAC9_NoANSI asserts that the output contains no ANSI escape
// sequences (no "\x1b" anywhere).
func TestAC9_NoANSI(t *testing.T) {
	t.Parallel()

	e := New(CategoryConfig, CodeConfigMalformed, "x").
		WithContext("k", "v").
		WithRemediation("Fix it.")
	got := Format(e, Options{})

	if strings.Contains(got, "\x1b") {
		t.Fatalf("output contains an escape character: %q", got)
	}
}

// --- AC10: format documented ------------------------------------------

// TestAC10_DocumentationExists asserts that the CLI UX spec
// contains a section describing the user-facing error format.
func TestAC10_DocumentationExists(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	path := filepath.Join(root, "docs", "cli-ux-spec.md")

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	const marker = "#### 4.10g Error Message Format"
	if !strings.Contains(string(raw), marker) {
		t.Fatalf("%s does not contain the section %q", path, marker)
	}
}

// --- Byte-level rules (F7, F9, F10) -----------------------------------

// TestRuleF7_NoTrailingWhitespace is the same as AC8 but is named
// for the byte-level rule; it exists so that `go test -run F7`
// selects the rule's check.
func TestRuleF7_NoTrailingWhitespace(t *testing.T) {
	t.Parallel()

	e := New(CategoryConfig, CodeConfigMalformed, "x").
		WithContext("k", "v").
		WithRemediation("Fix it.")
	got := Format(e, Options{})

	for i, line := range strings.Split(got, "\n") {
		if strings.TrimRight(line, " \t") != line {
			t.Fatalf("line %d has trailing whitespace: %q", i, line)
		}
	}
}

// TestRuleF9_EveryLineEndsWithNewline asserts that every line of
// the output — including the last — is terminated by "\n".
func TestRuleF9_EveryLineEndsWithNewline(t *testing.T) {
	t.Parallel()

	e := New(CategoryConfig, CodeConfigMalformed, "x").
		WithContext("k", "v").
		WithRemediation("Fix it.")
	got := Format(e, Options{})

	if !strings.HasSuffix(got, "\n") {
		t.Fatalf("output does not end with newline: %q", got)
	}
	lines := strings.Split(got, "\n")
	if strings.Join(lines, "\n") != got {
		t.Fatal("output is not line-separated by \\n")
	}
}

// TestRuleF10_ExactlyOneTrailingNewline asserts that the output
// ends with exactly one "\n" and no blank line after it.
func TestRuleF10_ExactlyOneTrailingNewline(t *testing.T) {
	t.Parallel()

	e := New(CategoryCLI, CodeCLIUnknownCommand, "x")
	got := Format(e, Options{})

	if !strings.HasSuffix(got, "\n") {
		t.Fatalf("output does not end with newline: %q", got)
	}
	if strings.HasSuffix(got, "\n\n") {
		t.Fatalf("output ends with more than one newline: %q", got)
	}
}

// --- Positive / Negative / Edge cases ---------------------------------

// TestFormat_NilError asserts that Format(nil, ...) returns "".
func TestFormat_NilError(t *testing.T) {
	t.Parallel()

	if got := Format(nil, Options{}); got != "" {
		t.Fatalf("Format(nil) = %q, want %q", got, "")
	}
}

// TestFormat_NonError asserts that Format on a plain error (not a
// *Error and not wrapping one) falls back to a single-line
// rendering.
func TestFormat_NonError(t *testing.T) {
	t.Parallel()

	plain := errors.New("something went wrong")
	got := Format(plain, Options{})

	if got != "Error: something went wrong\n" {
		t.Fatalf("Format(plain) = %q, want %q", got, "Error: something went wrong\n")
	}
}

// TestFormat_WrappedError asserts that Format extracts the inner
// *Error from a fmt.Errorf %w wrapper and renders it in full.
func TestFormat_WrappedError(t *testing.T) {
	t.Parallel()

	inner := New(CategoryConfig, CodeConfigMalformed, "invalid YAML").
		WithContext("file", "config.yaml").
		WithRemediation("Fix the YAML syntax.")
	wrapped := fmt.Errorf("outer context: %w", inner)

	got := Format(wrapped, Options{})
	want := Format(inner, Options{})

	if got != want {
		t.Fatalf(
			"Format(wrapped) = %q, want %q (inner)",
			got, want,
		)
	}
}

// TestFormat_DeeplyWrappedError asserts that Format finds the
// inner *Error through multiple layers of fmt.Errorf wrapping.
func TestFormat_DeeplyWrappedError(t *testing.T) {
	t.Parallel()

	inner := New(CategoryConfig, CodeConfigMalformed, "invalid YAML")
	wrapped := fmt.Errorf("a: %w", fmt.Errorf("b: %w", fmt.Errorf("c: %w", inner)))

	got := Format(wrapped, Options{})
	want := Format(inner, Options{})

	if got != want {
		t.Fatalf("Format(deeply wrapped) = %q, want %q", got, want)
	}
}

// --- Corner cases -----------------------------------------------------

// TestFormat_EmptyContextValue asserts that a context key with an
// empty value renders without a trailing space, satisfying F7.
//
// The constructor permits an empty value for a context key; the
// formatter must render it without violating F7 (no trailing
// whitespace). The implementation writes "<key>: <value>" with
// the value empty, which produces "<key>: " with a trailing
// space. This test pins the correct behaviour: no trailing space.
//
// The fix, if the implementation does not satisfy this, is to
// emit "<key>:" (no space) when the value is empty.
func TestFormat_EmptyContextValue(t *testing.T) {
	t.Parallel()

	e := New(CategoryConfig, CodeConfigMalformed, "x").
		WithContext("k", "")
	got := Format(e, Options{})

	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(line, "       k:") && strings.HasSuffix(line, " ") {
			t.Fatalf("line for empty value has trailing space: %q", line)
		}
	}
}

// TestFormat_ManyContextKeys asserts that a *Error with many
// context keys renders them all, sorted.
func TestFormat_ManyContextKeys(t *testing.T) {
	t.Parallel()

	e := New(CategoryConfig, CodeConfigMalformed, "x")
	keys := []string{"z", "y", "x", "w", "v", "u", "t", "s", "r", "q", "p", "o", "n", "m", "l", "k"}
	for _, k := range keys {
		e = e.WithContext(k, "v-"+k)
	}
	got := Format(e, Options{})

	for _, k := range keys {
		if !strings.Contains(got, "       "+k+": v-"+k) {
			t.Fatalf("key %q missing from output", k)
		}
	}

	sortedKeys := []string{"k", "l", "m", "n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z"}
	prev := -1
	for _, k := range sortedKeys {
		idx := strings.Index(got, "       "+k+": ")
		if idx == -1 {
			t.Fatalf("sorted key %q missing from output", k)
		}
		if idx <= prev {
			t.Fatalf("key %q at index %d, not after previous %d", k, idx, prev)
		}
		prev = idx
	}
}

// TestFormat_NoContext asserts that the output has no context
// lines when the context is nil.
func TestFormat_NoContext(t *testing.T) {
	t.Parallel()

	e := New(CategoryCLI, CodeCLIUnknownCommand, "x")
	got := Format(e, Options{})

	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(line, "       ") {
			t.Fatalf("unexpected context line: %q", line)
		}
	}
}

// --- Determinism and concurrency --------------------------------------

// TestFormat_Deterministic asserts that Format returns the same
// string on repeated calls with the same input.
func TestFormat_Deterministic(t *testing.T) {
	t.Parallel()

	e := New(CategoryConfig, CodeConfigMalformed, "x").
		WithContext("a", "1").
		WithContext("b", "2").
		WithContext("c", "3").
		WithRemediation("Fix it.")

	first := Format(e, Options{})
	for i := 0; i < 100; i++ {
		if got := Format(e, Options{}); got != first {
			t.Fatalf("Format changed on call %d: %q -> %q", i, first, got)
		}
	}
}

// TestFormat_Concurrent asserts that Format is safe for
// concurrent use. The *Error is immutable, so Format must be a
// pure function of its inputs.
func TestFormat_Concurrent(t *testing.T) {
	e := New(CategoryConfig, CodeConfigMalformed, "x").
		WithContext("a", "1").
		WithContext("b", "2").
		WithRemediation("Fix it.")

	want := Format(e, Options{})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got := Format(e, Options{}); got != want {
				t.Errorf("concurrent Format mismatch:\ngot:  %q\nwant: %q", got, want)
			}
		}()
	}
	wg.Wait()
}

// --- Fuzz: the format's invariants ------------------------------------

// FuzzFormat asserts that Format's output satisfies the frozen
// invariants for every valid *Error. The fuzzer constructs
// *Error values from arbitrary strings and asserts the
// byte-level rules on the output.
//
// The fuzzer does not check the golden-file content, because
// that requires a fixed input. It checks the *structural*
// invariants: F7 (no trailing whitespace), F8 (no ANSI), F9
// (every line ends with \n), F10 (exactly one trailing \n).
func FuzzFormat(f *testing.F) {
	f.Add("x", "FORGE_CLI_UNKNOWN_COMMAND", "CLI", "", "")
	f.Add("a longer message", "FORGE_CONFIG_MALFORMED", "CONFIG", "k", "v")
	f.Add("x", "FORGE_CLI_UNKNOWN_COMMAND", "CLI", "a", "1")
	f.Add("y", "FORGE_CONFIG_MALFORMED", "CONFIG", "z", "2")

	f.Fuzz(func(t *testing.T, msg, code, catStr, key, value string) {
		cat := Category(catStr)
		if !cat.valid() {
			t.Skip("invalid category")
		}
		if !IsRegistered(Code(code)) {
			t.Skip("unregistered code")
		}
		actualCat, ok := Code(code).Category()
		if !ok || actualCat != cat {
			t.Skip("code does not match category")
		}
		if msg == "" || len(msg) > 80 || strings.HasSuffix(msg, ".") || strings.ContainsAny(msg, "\n\r") {
			t.Skip("invalid message")
		}
		if key != "" && !isSnakeCase(key) {
			t.Skip("invalid context key")
		}

		var e *Error
		if key == "" {
			e = New(cat, Code(code), msg)
		} else {
			e = New(cat, Code(code), msg).WithContext(key, value)
		}

		out := Format(e, Options{})

		// F7: no trailing whitespace on any line.
		for i, line := range strings.Split(out, "\n") {
			if strings.TrimRight(line, " \t\r") != line {
				t.Fatalf("F7 violated on line %d: %q", i, line)
			}
		}

		// F8: no ANSI escapes.
		if strings.Contains(out, "\x1b") {
			t.Fatalf("F8 violated: %q", out)
		}

		// F9: every line ends with \n.
		if !strings.HasSuffix(out, "\n") {
			t.Fatalf("F9 violated: %q", out)
		}

		// F10: exactly one trailing \n.
		if strings.HasSuffix(out, "\n\n") {
			t.Fatalf("F10 violated: %q", out)
		}
	})
}

// TestFormat_RedactsSecretContextValue asserts that a context
// entry whose key is a secret is rendered as "<redacted>".
//
// The rule is absolute and applies in every mode: a secret is
// never shown, even when the key is not recognised as a secret
// by name. The redaction is keyed on the *value*, not the key.
func TestFormat_RedactsSecretContextValue(t *testing.T) {
	t.Parallel()

	e := New(CategoryConfig, CodeConfigMalformed, "x").
		WithContext("api_token", "ghp_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")

	out := Format(e, Options{})

	if !strings.Contains(out, "api_token: <redacted>") {
		t.Errorf("output does not contain redacted token:\n%s", out)
	}
	if strings.Contains(out, "ghp_") {
		t.Errorf("output contains the raw token:\n%s", out)
	}
}

// TestFormat_RedactsHomeDirectoryPath asserts that a context
// value that is a path under the home directory is rendered with
// the "~" prefix.
//
// The test uses the actual home directory, obtained from
// os.UserHomeDir, rather than a synthetic path. A synthetic path
// would not exercise the redaction rule's dependence on the
// running user's home, which is the part that can break across
// platforms or CI environments.
func TestFormat_RedactsHomeDirectoryPath(t *testing.T) {
	t.Parallel()

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("no home directory available")
	}

	in := filepath.Join(home, "config.yaml")
	e := New(CategoryConfig, CodeConfigMalformed, "x").
		WithContext("file", in)

	out := Format(e, Options{})
	if !strings.Contains(out, "file: ~/config.yaml") {
		t.Errorf("output does not contain redacted path:\n%s", out)
	}
	if strings.Contains(out, home) {
		t.Errorf("output contains the raw home directory:\n%s", out)
	}
}

// TestFormat_CauseNeverShown asserts that the user-facing format
// does not render the cause, regardless of whether it is set.
// The rule is frozen in WBS 10.4.3: the cause is a programmer
// concern.
func TestFormat_CauseNeverShown(t *testing.T) {
	t.Parallel()

	e := New(CategoryConfig, CodeConfigMalformed, "x").
		WithCause(errors.New("internal detail: /etc/forge/secret.yaml"))

	out := Format(e, Options{})
	if strings.Contains(out, "internal detail") {
		t.Errorf("output contains the cause:\n%s", out)
	}
	if strings.Contains(out, "/etc/forge/secret.yaml") {
		t.Errorf("output contains the cause's path:\n%s", out)
	}
}

// --- Verbose mode tests (WBS 10.4.4) ----------------------------------

// TestVerbose_CauseBlockRendered asserts that verbose mode renders
// the cause chain as a "Cause:" block, and that non-verbose mode
// omits it.
func TestVerbose_CauseBlockRendered(t *testing.T) {
	t.Parallel()

	inner := errors.New("yaml: line 12: did not find expected key")
	e := New(CategoryConfig, CodeConfigMalformed, "invalid YAML").
		WithCause(inner)

	if out := Format(e, Options{}); strings.Contains(out, "Cause:") {
		t.Errorf("non-verbose output contains a Cause block:\n%s", out)
	}

	out := Format(e, Options{Verbose: true})
	if !strings.Contains(out, "Cause:\n") {
		t.Errorf("verbose output does not contain a Cause block:\n%s", out)
	}
	if !strings.Contains(out, "  "+inner.Error()) {
		t.Errorf("verbose output does not contain the cause:\n%s", out)
	}
}

// TestVerbose_CauseChainRendered asserts that verbose mode
// renders the full cause chain, not just the immediate cause.
//
// The fixture is constructed with two levels of wrapping around a
// base *Error, which is the shape a real call chain produces:
// a config loader returns a *Error, an outer layer wraps it with
// context, and the CLI attaches a final wrapper. The verbose
// formatter walks the chain via errors.Unwrap and renders each
// level.
func TestVerbose_CauseChainRendered(t *testing.T) {
	t.Parallel()

	inner := errors.New("inner")
	middle := errors.New("middle")

	// Build: outer(*Error) → middle → inner.
	// The *Error has a cause; the cause has a cause.
	outer := configErr(t, "x").WithCause(fmt.Errorf("%w: %w", middle, inner))

	out := Format(outer, Options{Verbose: true})
	for _, want := range []string{"Cause:", "inner"} {
		if !strings.Contains(out, want) {
			t.Errorf("verbose output does not contain %q:\n%s", want, out)
		}
	}
}

// TestVerbose_StackBlockRendered asserts that verbose mode renders
// the stack trace when Options.Stack is non-empty, and that it
// omits the block otherwise.
func TestVerbose_StackBlockRendered(t *testing.T) {
	t.Parallel()

	e := New(CategoryConfig, CodeConfigMalformed, "invalid YAML")

	out := Format(e, Options{Verbose: true})
	if strings.Contains(out, "Stack:") {
		t.Errorf("verbose output contains a Stack block when no stack was provided:\n%s", out)
	}

	stack := []string{
		"main.loadConfig (internal/config/loader.go:42)",
		"main.runConfig (internal/cli/config.go:31)",
	}
	out = Format(e, Options{Verbose: true, Stack: stack})
	if !strings.Contains(out, "Stack:\n") {
		t.Errorf("verbose output does not contain a Stack block:\n%s", out)
	}
	for _, frame := range stack {
		if !strings.Contains(out, "  "+frame) {
			t.Errorf("verbose output does not contain frame %q:\n%s", frame, out)
		}
	}
}

// TestVerbose_InternalContextRendered asserts that verbose mode
// renders the internal_-prefixed context keys, and that
// non-verbose mode filters them out.
func TestVerbose_InternalContextRendered(t *testing.T) {
	t.Parallel()

	e := New(CategoryConfig, CodeConfigMalformed, "invalid YAML").
		WithContext("file", "config.yaml").
		WithContext("internal_trace_id", "abc-123").
		WithContext("internal_req", "42")

	out := Format(e, Options{})
	if strings.Contains(out, "internal_") {
		t.Errorf("non-verbose output contains an internal_ key:\n%s", out)
	}
	if !strings.Contains(out, "file: config.yaml") {
		t.Errorf("non-verbose output does not contain the public key:\n%s", out)
	}

	out = Format(e, Options{Verbose: true})
	for _, want := range []string{"internal_trace_id: abc-123", "internal_req: 42"} {
		if !strings.Contains(out, want) {
			t.Errorf("verbose output does not contain %q:\n%s", want, out)
		}
	}
}

// TestVerbose_HomePathShown asserts that verbose mode shows the
// full home-directory path, and that non-verbose mode redacts it.
//
// The distinction is deliberate: verbose output is a programmer-
// facing diagnostic, and the home directory is not a secret to
// the programmer running the command. The non-verbose output is
// user-facing and may be pasted into a bug report, so the home
// directory is redacted.
func TestVerbose_HomePathShown(t *testing.T) {
	t.Parallel()

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("no home directory available")
	}

	in := filepath.Join(home, "config.yaml")
	e := New(CategoryConfig, CodeConfigMalformed, "invalid YAML").
		WithContext("file", in)

	out := Format(e, Options{})
	if !strings.Contains(out, "file: ~/config.yaml") {
		t.Errorf("non-verbose output does not contain redacted path:\n%s", out)
	}
	if strings.Contains(out, home) {
		t.Errorf("non-verbose output contains the raw home directory:\n%s", out)
	}

	out = Format(e, Options{Verbose: true})
	if !strings.Contains(out, "file: "+in) {
		t.Errorf("verbose output does not contain the full path:\n%s", out)
	}
}

// TestVerbose_SecretsStillRedacted asserts that verbose mode does
// not weaken secret redaction. The rule "Secrets are never shown"
// (WBS 10.4.3) is absolute.
//
// The test covers both a key whose name suggests a secret
// (api_token) and a key whose name does not (internal_token). The
// redaction is keyed on the value, not on the key name, so both
// values must be redacted. The test exists because the verbose
// mode's "show everything" disposition is exactly the kind of
// rule that would erode secret redaction by accident.
func TestVerbose_SecretsStillRedacted(t *testing.T) {
	t.Parallel()

	const secret = "ghp_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	e := New(CategoryConfig, CodeConfigMalformed, "x").
		WithContext("api_token", secret).
		WithContext("internal_token", secret)

	out := Format(e, Options{Verbose: true})

	if strings.Contains(out, secret) {
		t.Errorf("verbose output contains the raw token:\n%s", out)
	}
	for _, key := range []string{"api_token", "internal_token"} {
		if !strings.Contains(out, key+": <redacted>") {
			t.Errorf("verbose output does not redact %q:\n%s", key, out)
		}
	}
}

// TestGolden_VerboseFormat pins the verbose rendering of a
// canonical error. The fixture exercises every verbose-only
// element: cause, stack, and internal context.
//
// Regenerate with:
//
//	go test -run TestGolden_VerboseFormat -update
//
// from inside internal/forgeerr/.
func TestGolden_VerboseFormat(t *testing.T) {
	t.Parallel()

	e := New(CategoryConfig, CodeConfigMalformed, "invalid YAML").
		WithCause(errors.New("yaml: line 12: did not find expected key")).
		WithContext("file", "config.yaml").
		WithContext("line", "12").
		WithContext("internal_trace_id", "abc-123").
		WithRemediation("Fix the YAML syntax at line 12.")

	opts := Options{
		Verbose: true,
		Stack: []string{
			"main.loadConfig (internal/config/loader.go:42)",
			"main.runConfig (internal/cli/config.go:31)",
		},
	}

	got := Format(e, opts)
	path := filepath.Join("testdata", "verbose.golden.txt")
	want := goldenText(t, path, got)

	if got != want {
		t.Fatalf(
			"verbose output does not match %s.\n"+
				"Run `go test -run TestGolden_VerboseFormat -update` from internal/forgeerr/ to regenerate.\n"+
				"--- got ---\n%s\n--- want ---\n%s",
			path, got, want,
		)
	}
}
