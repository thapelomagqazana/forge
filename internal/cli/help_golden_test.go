// Package cli contains white-box tests for the CLI package.
//
// This file pins the root help content contract (WBS 7.1.2). The
// root help output is compared byte-for-byte against a golden file,
// and its structural properties (no ANSI escapes, ends in newline,
// contains the identity constants, lists all global flags, orders
// commands alphabetically) are asserted by dedicated tests.
//
// # Why a separate file from help_matrix_test.go
//
// help_matrix_test.go asserts the *invocation* matrix: which ways of
// reaching help produce which exit codes and which streams. This
// file asserts the *content* contract: what the root help output
// contains, byte for byte.
//
// The two files are complementary. A change to the root command's
// configuration could break the content without changing the
// invocation behaviour, and vice versa. Keeping the assertions in
// separate files makes the failure mode of each clear.
package cli

import (
	"os"
	"sort"
	"strings"
	"testing"
)

// rootHelpGoldenPath is the path to the golden file for the root
// help output, relative to the test's working directory.
//
// The test runs with the package directory as its working directory,
// so the path is relative to internal/cli/.
const rootHelpGoldenPath = "testdata/help/root.golden.txt"

// readRootHelpGolden reads the golden file and returns its contents
// as a string. It fails the test if the file cannot be read.
//
// The helper is used by TestRootHelp_Golden and by the structural
// tests, which compare substrings against the golden content rather
// than against the raw output. Reading the golden file once and
// comparing against it is more reliable than re-running the CLI in
// each structural test.
func readRootHelpGolden(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(rootHelpGoldenPath)
	if err != nil {
		t.Fatalf("read golden %s: %v", rootHelpGoldenPath, err)
	}
	return string(b)
}

// =============================================================================
// Golden comparison
// =============================================================================

// TestRootHelp_Golden compares `forge --help` byte-for-byte against
// the golden file.
//
// This is the ATDD acceptance test for WBS 7.1.2. Every byte of the
// output is asserted; a change to the output fails the test and
// requires an ADR (per the WBS's "golden file updated only via
// ADR" KPI).
//
// # How to update the golden file
//
// Edit the file by hand, or regenerate it from the binary:
//
//	task build
//	./forge --help > internal/cli/testdata/help/root.golden.txt
//
// Then run this test. If it passes, the golden file records the
// current output. If it fails, the golden file was regenerated from
// a binary whose output differs from the one under test; investigate
// before committing.
//
// # Why regenerate from the binary
//
// The shell pipeline above is the safest way to produce a golden
// file that matches the binary's output. A file edited by hand can
// lose trailing whitespace, drop the final newline, or acquire
// carriage returns. Regenerating from the binary avoids all three.
func TestRootHelp_Golden(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"--help"}, nil)

	if got.exitCode != ExitSuccess {
		t.Fatalf("exit code: got %d, want %d\nstderr: %s",
			got.exitCode, ExitSuccess, got.stderr)
	}
	if got.stderr != "" {
		t.Fatalf("stderr should be empty on --help: %q", got.stderr)
	}

	want := readRootHelpGolden(t)

	if got.stdout != want {
		// Report the first line that differs, so the failure is
		// easy to diagnose without scrolling through the entire
		// output.
		gotLines := strings.Split(got.stdout, "\n")
		wantLines := strings.Split(want, "\n")
		for i := 0; i < len(gotLines) || i < len(wantLines); i++ {
			var g, w string
			if i < len(gotLines) {
				g = gotLines[i]
			}
			if i < len(wantLines) {
				w = wantLines[i]
			}
			if g != w {
				t.Errorf("first difference at line %d:\n"+
					"got:  %q\nwant: %q\n"+
					"if the change is intentional, update %s "+
					"and cite an ADR in the commit message",
					i+1, g, w, rootHelpGoldenPath)
				return
			}
		}
		// The loops above should have caught any difference, but
		// if the strings differ while every line matches, the
		// difference is in the newline handling. Report the raw
		// lengths as a fallback.
		t.Errorf("outputs differ in length but not in lines:\n"+
			"got length:  %d\nwant length: %d",
			len(got.stdout), len(want))
	}
}

// =============================================================================
// Structural assertions
// =============================================================================
//
// These tests assert the properties that WBS 7.1.2 lists as
// acceptance criteria. They read the golden file (or the raw output)
// and check a structural property, rather than comparing byte for
// byte. A byte-for-byte change to the output fails the golden test
// above; a structural change to the output fails one of these tests
// with a more specific message.

// TestRootHelp_HasNoAnsiEscapes verifies that the root help output
// contains no ANSI escape sequences.
//
// The escape character is 0x1b (ESC). Its presence in the output
// would mean Cobra or a future change introduced coloured output
// for help text. The contract requires plain text.
func TestRootHelp_HasNoAnsiEscapes(t *testing.T) {
	t.Parallel()

	want := readRootHelpGolden(t)

	if idx := strings.IndexByte(want, 0x1b); idx >= 0 {
		t.Errorf("golden file contains an ANSI escape at byte %d: %q",
			idx, want[idx:min(idx+10, len(want))])
	}
}

// TestRootHelp_EndsInNewline verifies that the root help output
// ends in exactly one newline.
//
// The output is line-oriented; the last line is terminated by a
// newline. The test asserts the final byte is 0x0a and that the
// preceding byte is not another 0x0a (which would mean the file ends
// in a blank line).
func TestRootHelp_EndsInNewline(t *testing.T) {
	t.Parallel()

	want := readRootHelpGolden(t)

	if len(want) == 0 {
		t.Fatal("golden file is empty")
	}
	if want[len(want)-1] != '\n' {
		t.Errorf("golden file does not end in a newline; "+
			"last byte: 0x%02x", want[len(want)-1])
	}
	if len(want) >= 2 && want[len(want)-2] == '\n' {
		t.Errorf("golden file ends in a blank line; " +
			"the frozen format has no trailing blank line")
	}
}

// TestRootHelp_ContainsRootLongDesc verifies that the RootLongDesc
// constant appears verbatim in the root help output.
//
// The constant is the body of the help text. If it is not present,
// the root command's Long field is not set to the constant, or
// Cobra is rendering a different field.
func TestRootHelp_ContainsRootLongDesc(t *testing.T) {
	t.Parallel()

	want := readRootHelpGolden(t)

	if !strings.Contains(want, RootLongDesc) {
		t.Errorf("golden file does not contain RootLongDesc:\n"+
			"RootLongDesc:\n%s", RootLongDesc)
	}
}

// TestRootHelp_ContainsRootUsage verifies that the RootUsage
// constant appears verbatim in the root help output.
//
// The usage line is rendered by Cobra under the "Usage:" heading.
// The constant is the composed string "forge [command]".
func TestRootHelp_ContainsRootUsage(t *testing.T) {
	t.Parallel()

	want := readRootHelpGolden(t)

	if !strings.Contains(want, RootUsage) {
		t.Errorf("golden file does not contain RootUsage %q:\n%s",
			RootUsage, want)
	}
}

// TestRootHelp_ListsAllGlobalFlags verifies that every flag in the
// global flag inventory (WBS 5.3.1) appears in the root help output.
//
// The inventory is: --config, --quiet, --verbose. The root command
// also has --help and --version, which Cobra adds automatically.
// The test asserts the three persistent flags plus the two Cobra
// flags.
//
// The test does not assert the order of the flags. The order is
// Cobra's (alphabetical by long name) and is pinned by the golden
// file. Asserting order here would duplicate the golden file's job.
func TestRootHelp_ListsAllGlobalFlags(t *testing.T) {
	t.Parallel()

	want := readRootHelpGolden(t)

	flags := []string{
		"--config",
		"--help",
		"--quiet",
		"--verbose",
		"--version",
	}
	for _, flag := range flags {
		if !strings.Contains(want, flag) {
			t.Errorf("golden file does not list flag %q", flag)
		}
	}
}

// TestRootHelp_CommandsAreAlphabetical verifies that the visible
// commands in the "Available Commands:" section appear in
// alphabetical order by name.
//
// The commands are the visible entries in the registry
// (internal/cli/registry.go) plus Cobra's auto-generated `completion`
// and `help` commands. Hidden commands (for example, `config` in
// Phase 2) do not appear in the help output and are not checked.
//
// # How the check works
//
// The test extracts the command names from the "Available Commands:"
// section of the golden file, then verifies that the list is sorted.
// The extraction relies on the format Cobra produces: each command
// line begins with two spaces followed by the command name and
// whitespace.
func TestRootHelp_CommandsAreAlphabetical(t *testing.T) {
	t.Parallel()

	want := readRootHelpGolden(t)
	commands := extractCommandNames(t, want)

	if len(commands) < 2 {
		t.Fatalf("expected at least 2 visible commands, found %d: %v",
			len(commands), commands)
	}

	if !sort.StringsAreSorted(commands) {
		sorted := make([]string, len(commands))
		copy(sorted, commands)
		sort.Strings(sorted)
		t.Errorf("commands are not alphabetically ordered:\n"+
			"got:  %v\nwant: %v", commands, sorted)
	}
}

// extractCommandNames parses the "Available Commands:" section of
// the help output and returns the command names in the order they
// appear.
//
// # Format assumption
//
// The function relies on the format Cobra produces: an "Available
// Commands:" heading, followed by one or more lines that begin with
// two spaces and a command name, followed by whitespace and the
// command's short description. The section ends at the first blank
// line or the next heading.
//
// If a future Cobra version changes the format, this function must
// be updated. The golden test above will fail first, which will
// prompt the update.
func extractCommandNames(t *testing.T, help string) []string {
	t.Helper()

	const heading = "Available Commands:"
	idx := strings.Index(help, heading)
	if idx < 0 {
		t.Fatalf("help output does not contain %q:\n%s", heading, help)
	}

	rest := help[idx+len(heading):]
	var names []string
	for _, line := range strings.Split(rest, "\n") {
		// Skip the blank line immediately after the heading.
		if strings.TrimSpace(line) == "" {
			continue
		}
		// A command line begins with exactly two spaces.
		if !strings.HasPrefix(line, "  ") {
			// A line that does not begin with two spaces ends
			// the commands section (it is a blank line or the
			// next heading).
			break
		}
		// Extract the command name: everything after the two
		// leading spaces, up to the next whitespace.
		body := line[2:]
		name, _, found := strings.Cut(body, " ")
		if !found {
			// A line with no space is malformed; skip it.
			continue
		}
		names = append(names, name)
	}
	return names
}

// min returns the smaller of two ints. It is used by the ANSI escape
// test to bound a slice operation.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
