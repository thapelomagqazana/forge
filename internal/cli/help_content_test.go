// Package cli contains white-box tests for the CLI package.
//
// This file pins the help *output* contract (WBS 7.1.3): the
// global flags appear in every command's help output, and the
// output contains no ANSI escapes. The metadata rules (Short
// shape, Long shape, Use regex, Args presence, Example presence,
// Aliases) live in metadata_test.go; the split reflects two
// different sources of truth.
//
// # Why the tests iterate the registry
//
// Each test iterates allRegisteredCommands() (defined in
// help_content.go) and invokes the command through runCLI,
// capturing stdout. The iteration is the same pattern as
// metadata_test.go's; the difference is that these tests read the
// output, not the struct.
package cli

import (
	"strings"
	"testing"
)

// =============================================================================
// Global flags
// =============================================================================

// TestHelpContent_GlobalFlagsPresent verifies that every registered
// command's help output lists all three global flags.
//
// The test invokes `forge <command> --help` through the runCLI
// helper, captures the output, and asserts that each global flag
// appears. It does not assert the order of the flags; the order is
// Cobra's and is pinned by the golden files.
//
// # Why the check is on the rendered output
//
// The global flags are persistent on the root command. They are
// inherited by every subcommand, but they are not registered
// directly on each subcommand's flag set. Whether Cobra renders
// them in the "Global Flags:" section is a property of Cobra's
// rendering, not of the subcommand's flag set. The test asserts
// the rendered output.
//
// # What the test catches
//
// A future change that accidentally removes a global flag from the
// root command, or that registers a local flag with the same name
// on a subcommand (which shadows the inherited flag and removes it
// from the "Global Flags:" section). The failure names the missing
// flag and quotes the command's help output.
func TestHelpContent_GlobalFlagsPresent(t *testing.T) {
	t.Parallel()

	for _, cmd := range allRegisteredCommands() {
		cmd := cmd
		name := commandName(cmd.Use)
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := runCLI(t, []string{name, "--help"}, nil)
			if got.exitCode != ExitSuccess {
				t.Fatalf("exit code: got %d, want %d\nstderr: %s",
					got.exitCode, ExitSuccess, got.stderr)
			}
			for _, flag := range globalFlagNames() {
				if !strings.Contains(got.stdout, flag) {
					t.Errorf("help output does not list global flag %q\n"+
						"output:\n%s", flag, got.stdout)
				}
			}
		})
	}
}

// =============================================================================
// ANSI escapes
// =============================================================================

// TestHelpContent_NoAnsiEscapes verifies that no command's help
// output contains ANSI escape sequences.
//
// The check is on the rendered output, because the escape bytes are
// produced by Cobra's renderer, not by the command's fields. A
// future change to the root command's configuration (for example,
// enabling colour) would affect every command's output; the test
// catches the change.
//
// # What the test catches
//
// A future change that enables Cobra's coloured help output, or
// that adds an ANSI-coloured prefix or suffix to the help text.
// The failure names the byte offset and quotes the surrounding
// output, so the contributor can locate the source of the escape.
//
// # Why the check is on the byte, not on a string
//
// The ANSI escape character is 0x1b (ESC). A test that searched
// for the string "\x1b" or "\033" would find only the escaped
// representation, not the byte. The test uses
// strings.IndexByte(got.stdout, 0x1b) to find the actual byte.
//
// Non-ASCII characters other than ESC are permitted: the identity
// strings contain an em dash (U+2014) and a right arrow (U+2192),
// both of which are intentional. The test looks only for the ESC
// byte.
func TestHelpContent_NoAnsiEscapes(t *testing.T) {
	t.Parallel()

	for _, cmd := range allRegisteredCommands() {
		cmd := cmd
		name := commandName(cmd.Use)
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := runCLI(t, []string{name, "--help"}, nil)
			if got.exitCode != ExitSuccess {
				t.Fatalf("exit code: got %d, want %d\nstderr: %s",
					got.exitCode, ExitSuccess, got.stderr)
			}
			if idx := strings.IndexByte(got.stdout, 0x1b); idx >= 0 {
				end := idx + 10
				if end > len(got.stdout) {
					end = len(got.stdout)
				}
				t.Errorf("help output contains an ANSI escape at byte %d: %q",
					idx, got.stdout[idx:end])
			}
		})
	}
}
