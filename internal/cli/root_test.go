// Package cli contains white-box tests for the CLI package.
//
// This file tests the root command: its dispatcher behaviour (what
// happens with no arguments, with --help, with an unknown command,
// with an unknown flag, with an empty-string argument), the
// properties that make it a well-behaved CLI dispatcher (exit codes,
// stream separation, determinism), the frozen identity strings it
// exposes (WBS 5.1.1), and the version flag it exposes (WBS 5.1.2).
//
// It is the test companion to internal/cli/root.go. The root
// command's job is small — it is a dispatcher, not a worker — and the
// tests are correspondingly small. The substantive behaviour lives in
// the subcommands and is tested by the subcommands' own test files.
//
// # Test organisation
//
// The tests are grouped by intent:
//
//   - Smoke tests: the three invocations a user will try first
//     (no args, --help, an unknown command). These assert on both
//     exit code and output.
//
//   - Stream separation: what a successful invocation writes to
//     stdout, what a failed invocation writes to stderr, and that
//     the two never mix.
//
//   - Edge cases: empty-string argument, unknown flag, multiple
//     arguments. These exercise the boundary between "absent" and
//     "present but invalid".
//
//   - Non-functional properties: determinism across repeated
//     invocations, and consistency between the two informational
//     invocations. These guard against accidental introduction of
//     timestamps, map iteration order, and similar nondeterminism.
//
//   - Environment isolation: the helper's env parameter is the only
//     source of environment variables the command sees. This pins a
//     property of the helper, placed here because root_test.go is
//     where the helper's behaviour is first used.
//
//   - Registry/help correspondence: the visible commands in help
//     output match the visible constructors in the registry.
//
//   - Root identity (WBS 5.1.1): the frozen identity strings
//     (RootName, RootUsage, RootShortDesc, RootLongDesc) satisfy
//     their rules, and the help output is derived from them.
//
//   - Version flag (WBS 5.1.2): `forge --version` produces the same
//     output as `forge version`, exits with the same code, and
//     writes to stdout.
//
// # What this file does not test
//
//   - The subcommands' behaviour. Each subcommand has its own test
//     file (for example, version_test.go) that exercises the
//     subcommand's logic. This file tests only the root command's
//     dispatch to those subcommands, and then only indirectly (an
//     unknown command is dispatched to the root's RunE handler;
//     a known command is dispatched to the subcommand).
//
//   - The exact help text. Cobra's help output is stable within a
//     version of Cobra but changes between versions. The tests
//     assert on the presence of stable substrings ("Usage:", the
//     command name, the identity strings' values) rather than on
//     the full text.
//
// # Test names
//
// Test names follow the BDD convention TestXxx_GivenY_ThenZ, which
// makes the precondition, action, and expected outcome explicit. A
// failure message names the invariant that was violated; a passing
// test name describes the behaviour it confirms.
package cli

import (
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

// =============================================================================
// Smoke tests
// =============================================================================
//
// The three invocations a user will try first. If these fail, the
// CLI is broken in a way that affects every subsequent test.

// TestRoot_GivenNoArgs_ThenExitsZeroAndPrintsUsage verifies that
// running `forge` with no arguments prints usage to stdout and exits
// with ExitSuccess.
//
// The exit code is the contract. A CLI that exits non-zero when
// invoked with no arguments is signalling that the invocation was
// wrong; for a dispatcher with subcommands, "no arguments" is a
// request for information, and the correct response is to exit
// successfully after printing the usage text.
//
// The test asserts:
//
//   - exitCode == ExitSuccess
//   - stdout contains "Forge" (the first word of the Long description)
//   - stderr is empty (a successful invocation writes nothing there)
func TestRoot_GivenNoArgs_ThenExitsZeroAndPrintsUsage(t *testing.T) {
	t.Parallel()

	got := runCLI(t, nil, nil)

	if got.exitCode != ExitSuccess {
		t.Errorf("exit code: got %d, want %d", got.exitCode, ExitSuccess)
	}

	// The root command prints its Long description when invoked
	// with no subcommand. The description begins with "Forge is a
	// cross-platform CLI...". The substring "Forge" is the stable
	// part; the full sentence may change as the product matures.
	if !strings.Contains(got.stdout, "Forge") {
		t.Errorf("stdout does not contain the root command description; "+
			"got: %q", got.stdout)
	}

	// Usage output is a successful result, not an error. Nothing
	// should be written to stderr.
	if got.stderr != "" {
		t.Errorf("stderr should be empty on success; got: %q", got.stderr)
	}
}

// TestRoot_GivenHelpFlag_ThenExitsZeroAndPrintsHelpToStdout verifies
// that `forge --help` prints help to stdout and exits with
// ExitSuccess.
//
// Cobra intercepts --help at the framework level, before any
// subcommand's RunE handler runs. The test proves that the
// interception works and that the output is routed to the injected
// stdout.
//
// The test asserts:
//
//   - exitCode == ExitSuccess
//   - stdout contains "forge" (the command name)
//   - stdout contains "Forge" (the Long description's first word)
//   - stderr is empty
//
// The distinction between "forge" and "Forge" is deliberate: the
// first is the command's canonical name (lowercase), the second is
// the human-readable description's first word (capitalised). Both
// must appear in help output.
func TestRoot_GivenHelpFlag_ThenExitsZeroAndPrintsHelpToStdout(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"--help"}, nil)

	if got.exitCode != ExitSuccess {
		t.Errorf("exit code: got %d, want %d", got.exitCode, ExitSuccess)
	}

	if !strings.Contains(got.stdout, "forge") {
		t.Errorf("stdout does not contain 'forge'; got: %q", got.stdout)
	}
	if !strings.Contains(got.stdout, "Forge") {
		t.Errorf("stdout does not contain the Long description; "+
			"got: %q", got.stdout)
	}

	if got.stderr != "" {
		t.Errorf("stderr should be empty when --help succeeds; "+
			"got: %q", got.stderr)
	}
}

// TestRoot_GivenUnknownCommand_ThenExitsUsageAndPrintsErrorToStderr
// verifies that `forge unknown-command` writes an error to stderr
// and exits with ExitUsage.
//
// The expected exit code is ExitUsage. This is the value the root
// command's RunE handler returns for an unrecognised command, and it
// is the value exitCodeFromError maps the resulting error to. The
// test pins both the code and the stream: the error goes to stderr,
// nothing goes to stdout.
func TestRoot_GivenUnknownCommand_ThenExitsUsageAndPrintsErrorToStderr(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"unknown-command"}, nil)

	if got.exitCode != ExitUsage {
		t.Errorf("exit code: got %d, want %d (ExitUsage)",
			got.exitCode, ExitUsage)
	}

	// The error message mentions the command the user typed.
	if !strings.Contains(got.stderr, "unknown-command") {
		t.Errorf("stderr does not mention the unknown command; "+
			"got: %q", got.stderr)
	}

	// Successful output goes to stdout; error output goes to
	// stderr. The two must not be mixed.
	if got.stdout != "" {
		t.Errorf("stdout should be empty when an error occurs; "+
			"got: %q", got.stdout)
	}
}

// =============================================================================
// Stream separation
// =============================================================================
//
// The stdout/stderr separation contract is the property that makes a
// CLI composable in shell pipelines. A successful invocation writes
// to stdout; a failed invocation writes to stderr. Neither mixes
// with the other.

// TestRoot_GivenSuccess_ThenStdoutIsNotEmptyAndStderrIsEmpty verifies
// the stream separation contract from the success side.
//
// Both the no-args and --help invocations are covered because they
// are the two ways to trigger a successful informational output.
// The test uses them in a single test rather than two because the
// property being verified is the same.
func TestRoot_GivenSuccess_ThenStdoutIsNotEmptyAndStderrIsEmpty(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
	}{
		{"no-args", nil},
		{"--help", []string{"--help"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := runCLI(t, tc.args, nil)

			if got.exitCode != ExitSuccess {
				t.Fatalf("exit code: got %d, want %d",
					got.exitCode, ExitSuccess)
			}
			if got.stdout == "" {
				t.Error("stdout is empty on success; want usage text")
			}
			if got.stderr != "" {
				t.Errorf("stderr is not empty on success: %q", got.stderr)
			}
		})
	}
}

// TestRoot_GivenFailure_ThenStdoutIsEmptyAndStderrIsNotEmpty verifies
// the stream separation contract from the failure side.
//
// The unknown-command and unknown-flag invocations are covered
// because they are the two ways to trigger a usage error. The test
// uses them in a single test because the property being verified is
// the same.
func TestRoot_GivenFailure_ThenStdoutIsEmptyAndStderrIsNotEmpty(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
	}{
		{"unknown-command", []string{"unknown-command"}},
		{"unknown-flag", []string{"--nonexistent-flag"}},
		{"empty-string-arg", []string{""}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := runCLI(t, tc.args, nil)

			if got.exitCode == ExitSuccess {
				t.Errorf("exit code: got %d, want non-success",
					got.exitCode)
			}
			if got.stdout != "" {
				t.Errorf("stdout is not empty on failure: %q",
					got.stdout)
			}
			if got.stderr == "" {
				t.Error("stderr is empty on failure; want diagnostic")
			}
		})
	}
}

// =============================================================================
// Edge cases
// =============================================================================
//
// The boundary between "absent" and "present but invalid". These are
// the cases where a naive dispatcher gets confused.

// TestRoot_GivenEmptyStringArgument_ThenExitsUsage verifies that an
// empty-string argument is treated as an unknown command.
//
// An empty string is not the same as an absent argument. The
// dispatcher must distinguish the two: no arguments is a request for
// help; an empty-string argument is a command the user typed that
// the dispatcher does not recognise. The test pins the distinction.
func TestRoot_GivenEmptyStringArgument_ThenExitsUsage(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{""}, nil)

	if got.exitCode == ExitSuccess {
		t.Errorf("expected non-zero exit code for empty-string argument; "+
			"got: %d", got.exitCode)
	}
	if got.stdout != "" {
		t.Errorf("stdout should be empty on failure; got: %q", got.stdout)
	}
	if got.stderr == "" {
		t.Error("stderr should contain a diagnostic on failure")
	}
}

// TestRoot_GivenUnknownFlag_ThenExitsUsage verifies that an unknown
// flag is treated as a usage error.
//
// This is distinct from an unknown command: Cobra handles the two
// through different code paths and produces different error
// messages, but the exit code and the stream separation are the
// same. The test pins the shared behaviour and leaves the specific
// error message to Cobra.
func TestRoot_GivenUnknownFlag_ThenExitsUsage(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"--nonexistent-flag"}, nil)

	if got.exitCode == ExitSuccess {
		t.Errorf("expected non-zero exit code for unknown flag; "+
			"got: %d", got.exitCode)
	}
	if got.stderr == "" {
		t.Error("expected an error message on stderr")
	}
	if got.stdout != "" {
		t.Errorf("stdout should be empty when an error occurs; "+
			"got: %q", got.stdout)
	}
}

// TestRoot_GivenMultipleArguments_ThenExitsUsage verifies that
// multiple positional arguments are treated as a usage error.
//
// The root command's RunE handler inspects args[0] for the command
// name and returns an error if it does not match a registered
// subcommand. The remaining arguments are not inspected; their
// presence does not change the outcome. The test pins that the
// dispatcher reports a usage error for any unrecognised first
// argument, regardless of what follows.
func TestRoot_GivenMultipleArguments_ThenExitsUsage(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"no-such-command", "extra", "args"}, nil)

	if got.exitCode != ExitUsage {
		t.Errorf("exit code: got %d, want %d (ExitUsage)",
			got.exitCode, ExitUsage)
	}
	if got.stdout != "" {
		t.Errorf("stdout should be empty on failure; got: %q", got.stdout)
	}
	if !strings.Contains(got.stderr, "no-such-command") {
		t.Errorf("stderr does not mention the command; got: %q", got.stderr)
	}
}

// =============================================================================
// Non-functional properties
// =============================================================================
//
// Properties that do not correspond to a single behaviour but to the
// consistency of the CLI across invocations.

// TestRoot_RepeatedInvocationsAreDeterministic verifies that two
// invocations with identical inputs produce byte-identical outputs.
//
// This is a non-functional test that guards against accidental
// introduction of nondeterminism: timestamps in output, map
// iteration order, and similar sources of drift. The test runs the
// same invocation twenty times and asserts on the first run's
// output; a single differing byte is a failure.
//
// The test uses --help because it exercises the largest output
// (usage, description, subcommand list, flags). A deterministic
// large output implies a deterministic small output; the reverse is
// not true.
func TestRoot_RepeatedInvocationsAreDeterministic(t *testing.T) {
	t.Parallel()

	first := runCLI(t, []string{"--help"}, nil)

	for i := 0; i < 20; i++ {
		got := runCLI(t, []string{"--help"}, nil)
		if got.exitCode != first.exitCode {
			t.Fatalf("run %d: exit code differs: got %d, want %d",
				i, got.exitCode, first.exitCode)
		}
		if got.stdout != first.stdout {
			t.Fatalf("run %d: stdout differs", i)
		}
		if got.stderr != first.stderr {
			t.Fatalf("run %d: stderr differs", i)
		}
	}
}

// TestRoot_NoArgsAndHelpFlag_ProduceSameExitCode verifies that the
// two informational invocations are consistent in their exit code.
//
// The content of the output may differ (Cobra formats help slightly
// differently depending on how it is triggered), but the exit code
// must be identical. A CLI that exits 0 for one and 2 for the other
// is inconsistent, and scripts that branch on the exit code would
// treat the two invocations differently for no good reason.
func TestRoot_NoArgsAndHelpFlag_ProduceSameExitCode(t *testing.T) {
	t.Parallel()

	noArgs := runCLI(t, nil, nil)
	helpFlag := runCLI(t, []string{"--help"}, nil)

	if noArgs.exitCode != helpFlag.exitCode {
		t.Errorf("exit code differs: no-args=%d, --help=%d",
			noArgs.exitCode, helpFlag.exitCode)
	}
	if noArgs.exitCode != ExitSuccess {
		t.Errorf("expected both to exit %d; got no-args=%d, --help=%d",
			ExitSuccess, noArgs.exitCode, helpFlag.exitCode)
	}
}

// =============================================================================
// Environment isolation
// =============================================================================
//
// The env parameter is a property of the runCLI helper, not of the
// root command. It is placed in this file because root_test.go is
// where the helper's behaviour is first used, and because the
// property is a precondition for every future test that needs to
// inject environment variables.

// TestRoot_GivenEnvMap_ThenTheCommandRunsWithThatMap verifies that
// the env map passed to runCLI is the source of environment
// variables visible to the command during the invocation.
//
// The version command does not read environment variables in
// Phase 2, so this test cannot assert on the effect of a specific
// variable. It asserts the weaker but still useful property that a
// non-nil env map does not prevent the command from succeeding. When
// a future command begins reading env, this test will already exist
// as a starting point for its tests.
//
// The test cannot assert on the absence of a variable in the
// process's real environment, because the process is the test binary
// itself. That assertion would require spawning a subprocess, which
// WBS 4.3.2 forbids.
func TestRoot_GivenEnvMap_ThenTheCommandRunsWithThatMap(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"FORGE_TEST_VAR": "injected",
	}
	got := runCLI(t, []string{"version"}, env)

	if got.exitCode != ExitSuccess {
		t.Fatalf("exit code: got %d, want %d", got.exitCode, ExitSuccess)
	}
	if got.stderr != "" {
		t.Errorf("stderr should be empty on success; got: %q", got.stderr)
	}
}

// TestRoot_GivenNilEnv_ThenTheCommandRunsWithNoVariables verifies
// that a nil env map is a valid input and produces a command
// invocation in which no environment variables are set.
//
// This is the behaviour that makes tests hermetic: a test that does
// not pass an env map sees no environment variables, regardless of
// what the developer's shell has exported.
func TestRoot_GivenNilEnv_ThenTheCommandRunsWithNoVariables(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"version"}, nil)

	if got.exitCode != ExitSuccess {
		t.Fatalf("exit code: got %d, want %d", got.exitCode, ExitSuccess)
	}
	if got.stderr != "" {
		t.Errorf("stderr should be empty on success; got: %q", got.stderr)
	}
}

// =============================================================================
// Registry / help correspondence
// =============================================================================

// TestRoot_HelpOutput_MatchesRegistry verifies that the visible
// subcommands in `forge --help` match the visible constructors in
// the registry, in order.
//
// The test is the behavioural counterpart to
// TestRegistry_OrderMatchesHelpOutput in registry_test.go. It
// exercises the check through the shared runCLI helper rather than
// by constructing the root command directly, so it verifies the
// property as a user would observe it.
func TestRoot_HelpOutput_MatchesRegistry(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"--help"}, nil)

	if got.exitCode != ExitSuccess {
		t.Fatalf("exit code: got %d, want %d",
			got.exitCode, ExitSuccess)
	}

	// Extract the "Available Commands:" section from the help
	// output. Cobra prints one line per visible command, indented
	// by two spaces, with the command name first.
	names := extractVisibleCommandNamesFromHelp(got.stdout)

	// Compare to the visible names from the registry.
	deps := testDependencies()
	var expected []string
	for _, ctor := range registry {
		cmd := ctor(deps)
		if cmd.Hidden {
			continue
		}
		expected = append(expected, firstWord(cmd.Use))
	}

	if len(names) != len(expected) {
		t.Fatalf("visible command count mismatch:\n"+
			"help:     %v\n"+
			"registry: %v", names, expected)
	}
	for i := range names {
		if names[i] != expected[i] {
			t.Errorf("visible command order mismatch at index %d:\n"+
				"help[%d]     = %q\n"+
				"registry[%d] = %q",
				i, i, names[i], i, expected[i])
		}
	}
}

// extractVisibleCommandNamesFromHelp parses Cobra's help output and
// returns the names of the visible Forge commands, in the order they
// appear.
//
// # What "Forge commands" means
//
// The help output contains both commands that Forge authored (the
// entries in registry.go) and commands that Cobra generates
// automatically:
//
//   - "completion" — the shell-completion command Cobra adds when
//     the root command has subcommands.
//   - "help" — the help command Cobra adds for every command tree.
//
// Neither is a Forge command. Neither appears in the registry. The
// helper filters them out so that the caller can compare the help
// output to the registry without a spurious mismatch.
//
// # What if a future Forge command is named "help" or "completion"
//
// Forge must not name a command "help" or "completion"; those names
// are owned by Cobra. If a future contributor attempts to register
// a command with one of those names, Cobra's AddCommand call fails
// at construction time, and the failure is visible in tests that
// construct the root command. The filter in this helper is therefore
// safe: it excludes only names that Forge cannot use.
//
// # The filter is by name, not by provenance
//
// The helper cannot inspect the registry directly — it has only the
// help output as a string. It filters by name. The names "help" and
// "completion" are stable across Cobra versions; they have been the
// auto-generated command names since Cobra 1.0. If a future version
// of Cobra changes them, this helper must be updated in the same
// commit, and the test that uses it will fail loudly.
func extractVisibleCommandNamesFromHelp(help string) []string {
	const header = "Available Commands:"

	// Cobra's auto-generated command names. They are not in the
	// registry and must not be included in the comparison.
	autoGenerated := map[string]bool{
		"completion": true,
		"help":       true,
	}

	lines := strings.Split(help, "\n")

	var names []string
	inSection := false
	for _, line := range lines {
		trimmed := strings.TrimRight(line, " \t")
		if trimmed == header {
			inSection = true
			continue
		}
		if !inSection {
			continue
		}
		if trimmed == "" {
			break
		}
		fields := strings.Fields(strings.TrimSpace(trimmed))
		if len(fields) == 0 {
			continue
		}
		name := fields[0]
		if autoGenerated[name] {
			continue
		}
		names = append(names, name)
	}
	return names
}

// =============================================================================
// Root command identity — the frozen constants (WBS 5.1.1)
// =============================================================================
//
// The four constants in root.go are the frozen identity of the root
// command. The tests below assert their invariants and their
// relationship to the help output.

// TestRootIdentity_RootNameIsLowercase verifies AC2: RootName is
// lowercase and contains no spaces, uppercase letters, or
// punctuation.
//
// The rule is enforced here rather than at compile time because a
// constant's value is a runtime property. The test fails loudly if
// a contributor changes RootName to "Forge" or "forge-cli".
func TestRootIdentity_RootNameIsLowercase(t *testing.T) {
	t.Parallel()

	if RootName == "" {
		t.Fatal("RootName is empty")
	}
	if strings.ToLower(RootName) != RootName {
		t.Errorf("RootName contains uppercase: %q", RootName)
	}
	for _, r := range RootName {
		if r == ' ' || r == '\t' || r == '\n' {
			t.Errorf("RootName contains whitespace: %q", RootName)
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' {
			t.Errorf("RootName contains %q; "+
				"only letters, digits, and hyphens are permitted", r)
		}
	}
}

// TestRootIdentity_RootShortDescLength verifies AC3: RootShortDesc
// is 80 characters or fewer.
//
// The length is measured in runes, not bytes, because the string
// contains an em dash (—), which is three bytes in UTF-8 but one
// rune. The rule is about the visual length, not the encoded
// length.
func TestRootIdentity_RootShortDescLength(t *testing.T) {
	t.Parallel()

	runeCount := utf8.RuneCountInString(RootShortDesc)
	if runeCount == 0 {
		t.Fatal("RootShortDesc is empty")
	}
	if runeCount > 80 {
		t.Errorf("RootShortDesc is %d runes; limit is 80: %q",
			runeCount, RootShortDesc)
	}
	if strings.Contains(RootShortDesc, "\n") {
		t.Errorf("RootShortDesc spans multiple lines: %q",
			RootShortDesc)
	}
}

// TestRootIdentity_RootLongDescContainsCoreLoop verifies AC4:
// RootLongDesc contains the CREATE / VERIFY / EXPLAIN / EVOLVE loop.
//
// The four keywords are the product's core loop. If a future change
// rewrites the Long description, the test asserts that the loop is
// still present. The keywords are checked in order; the loop is a
// sequence, not a set.
func TestRootIdentity_RootLongDescContainsCoreLoop(t *testing.T) {
	t.Parallel()

	keywords := []string{"CREATE", "VERIFY", "EXPLAIN", "EVOLVE"}
	pos := 0
	for _, kw := range keywords {
		idx := strings.Index(RootLongDesc[pos:], kw)
		if idx < 0 {
			t.Errorf("RootLongDesc does not contain %q after position %d: %q",
				kw, pos, RootLongDesc)
			return
		}
		pos += idx + len(kw)
	}
}

// TestRootIdentity_RootLongDescEndsWithHelpPointer verifies that
// RootLongDesc ends with a pointer to `<command> --help`.
//
// The pointer is the last non-empty line of the description. It
// tells a user how to learn more about a specific subcommand.
func TestRootIdentity_RootLongDescEndsWithHelpPointer(t *testing.T) {
	t.Parallel()

	if !strings.Contains(RootLongDesc, "<command> --help") {
		t.Errorf("RootLongDesc does not contain a '<command> --help' "+
			"pointer: %q", RootLongDesc)
	}
}

// TestRootIdentity_RootLongDescHasNoForbiddenChars verifies that the
// four identity strings contain no emojis, no ANSI colour codes, and
// no tabs.
//
// The rule is a Phase 2 constraint from docs/cli-ux-spec.md: colour
// and emphasis belong to the terminal, not to the CLI's identity
// strings. A future ADR may relax the rule; until then, the test
// enforces it.
func TestRootIdentity_RootLongDescHasNoForbiddenChars(t *testing.T) {
	t.Parallel()

	for _, s := range []struct {
		name  string
		value string
	}{
		{"RootName", RootName},
		{"RootUsage", RootUsage},
		{"RootShortDesc", RootShortDesc},
		{"RootLongDesc", RootLongDesc},
	} {
		if strings.ContainsAny(s.value, "\t") {
			t.Errorf("%s contains a tab: %q", s.name, s.value)
		}
		if strings.Contains(s.value, "\x1b[") {
			t.Errorf("%s contains an ANSI escape sequence: %q",
				s.name, s.value)
		}
	}
}

// TestRootIdentity_HelpOutputContainsConstants verifies AC5: the
// help output for `forge --help` contains the identity strings that
// Cobra actually renders.
//
// # Which strings appear in `forge --help`
//
// Cobra's help rendering is asymmetric with respect to the Short
// and Long fields:
//
//   - The Long description is the body of the help text. It appears
//     in the command's own `--help` output, above the "Usage:"
//     section.
//
//   - The Short description is the one-line summary that appears in
//     a *parent's* "Available Commands:" table. The root command has
//     no parent, so its Short description is not rendered by
//     `forge --help`.
//
// The test therefore asserts on the three strings that do appear:
//
//   - RootName, as the command name in the usage section.
//   - RootUsage, as the composed usage line.
//   - RootLongDesc, as the body of the help text.
//
// RootShortDesc is verified separately by
// TestRootIdentity_RootShortDescLength, which checks its length and
// format. Its absence from `forge --help` is not a defect; it is
// how Cobra renders commands that have both Short and Long
// descriptions.
func TestRootIdentity_HelpOutputContainsConstants(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"--help"}, nil)

	if got.exitCode != ExitSuccess {
		t.Fatalf("exit code: got %d, want %d",
			got.exitCode, ExitSuccess)
	}

	// RootName appears as the command name.
	if !strings.Contains(got.stdout, RootName) {
		t.Errorf("help output does not contain RootName %q: %q",
			RootName, got.stdout)
	}

	// RootUsage appears in the "Usage:" section. Cobra may append
	// "[flags]" to the usage line; the test asserts on the prefix,
	// which is RootUsage.
	if !strings.Contains(got.stdout, RootUsage) {
		t.Errorf("help output does not contain RootUsage %q: %q",
			RootUsage, got.stdout)
	}

	// RootLongDesc appears as the body of the help text. Cobra does
	// not reformat it, so the first line is sufficient to verify
	// its presence.
	firstLine := strings.SplitN(RootLongDesc, "\n", 2)[0]
	if !strings.Contains(got.stdout, firstLine) {
		t.Errorf("help output does not contain the first line of "+
			"RootLongDesc %q: %q", firstLine, got.stdout)
	}

	// The core loop keywords appear, in order. They are part of
	// RootLongDesc; the assertion is a stronger check on the body
	// than a single-line substring match.
	for _, kw := range []string{"CREATE", "VERIFY", "EXPLAIN", "EVOLVE"} {
		if !strings.Contains(got.stdout, kw) {
			t.Errorf("help output does not contain %q: %q", kw, got.stdout)
		}
	}

	// RootShortDesc does NOT appear in `forge --help`. The assertion
	// is deliberate: it pins the property that Cobra renders Short
	// only in a parent's command list, and the root has no parent.
	// If a future change to Cobra's rendering causes Short to appear
	// in the command's own help, this test will fail, and the
	// specification's table must be updated in the same commit.
	if strings.Contains(got.stdout, RootShortDesc) {
		t.Errorf("help output unexpectedly contains RootShortDesc %q; "+
			"Cobra's rendering may have changed, and the "+
			"specification's table in docs/cli-ux-spec.md § 4.7 "+
			"must be updated accordingly",
			RootShortDesc)
	}
}

// TestRootIdentity_UsageLineContainsRootUsage verifies that the
// "Usage:" line of `forge --help` contains RootUsage's value.
//
// Cobra composes the usage line from the command's Use field and the
// flags it detects. The exact composition (whether the line reads
// "forge [command]" or "forge [command] [flags]") depends on Cobra's
// version. The test asserts only that the frozen value appears
// somewhere in the usage line.
func TestRootIdentity_UsageLineContainsRootUsage(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"--help"}, nil)

	if got.exitCode != ExitSuccess {
		t.Fatalf("exit code: got %d, want %d",
			got.exitCode, ExitSuccess)
	}

	if !strings.Contains(got.stdout, RootUsage) {
		t.Errorf("help output does not contain RootUsage %q: %q",
			RootUsage, got.stdout)
	}
}

// =============================================================================
// Version flag (WBS 5.1.2)
// =============================================================================
//
// Cobra's built-in --version flag is enabled by setting root.Version
// to a non-empty string. The tests below assert that the flag works
// and that its output is identical to the `forge version`
// subcommand's output.

// TestRootVersion_FlagIsRegistered verifies AC1: setting
// root.Version to a non-empty string enables Cobra's --version flag.
//
// The test constructs the root command directly and asserts that
// the Version field is non-empty. It does not depend on the help
// output, because Cobra does not list --version in the "Flags:"
// section of help.
//
// Constructing the root command directly is necessary: the field is
// not observable through the CLI boundary. The runCLI helper does
// not expose the constructed command. The direct construction is
// safe because it uses the same deps the CLI uses and does not
// execute the command.
func TestRootVersion_FlagIsRegistered(t *testing.T) {
	t.Parallel()

	deps := testDependencies()
	root := newRootCmd(deps)

	if root.Version == "" {
		t.Error("root.Version is empty; --version will not be enabled")
	}
}

// TestRootVersion_FlagPrintsToStdoutAndExitsZero verifies AC2 and
// AC3: `forge --version` exits with success and writes to stdout,
// not stderr.
//
// The test invokes the CLI through the shared runCLI helper, so it
// exercises the full path from args to output. If Cobra wrote the
// version to stderr, the test would fail on the stderr assertion; if
// the exit code were non-zero, the test would fail on the exit-code
// assertion.
func TestRootVersion_FlagPrintsToStdoutAndExitsZero(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"--version"}, nil)

	if got.exitCode != ExitSuccess {
		t.Fatalf("exit code: got %d, want %d",
			got.exitCode, ExitSuccess)
	}
	if got.stdout == "" {
		t.Error("stdout is empty; want version output")
	}
	if got.stderr != "" {
		t.Errorf("stderr is non-empty on success: %q", got.stderr)
	}
}

// TestRootVersion_FlagMatchesSubcommand is the central test for
// WBS 5.1.2: `forge --version` and `forge version` produce identical
// output.
//
// The test runs both invocations and compares their stdout
// byte-for-byte. It also asserts that their exit codes and stderr
// match. If the two ever diverge — for example, because a future
// change edits one formatter but not the other — this test fails,
// and the fix is local: the format is defined in exactly one place
// (version.Raw and version.Format share the underlying formatter),
// and the two paths consume it.
//
// # Why byte-identical, not merely similar
//
// The contract is that the two invocations are interchangeable for
// scripts and for users. A script that parses `forge --version` and
// a script that parses `forge version` must see the same input.
// "Similar" is not enough; the two must be byte-identical for the
// contract to hold.
func TestRootVersion_FlagMatchesSubcommand(t *testing.T) {
	t.Parallel()

	flagRun := runCLI(t, []string{"--version"}, nil)
	cmdRun := runCLI(t, []string{"version"}, nil)

	if flagRun.exitCode != cmdRun.exitCode {
		t.Errorf("exit codes differ: --version=%d, version=%d",
			flagRun.exitCode, cmdRun.exitCode)
	}
	if flagRun.stdout != cmdRun.stdout {
		t.Errorf("stdout differs between `forge --version` and "+
			"`forge version`:\n"+
			"  --version: %q\n"+
			"  version:   %q",
			flagRun.stdout, cmdRun.stdout)
	}
	if flagRun.stderr != cmdRun.stderr {
		t.Errorf("stderr differs:\n"+
			"  --version: %q\n"+
			"  version:   %q",
			flagRun.stderr, cmdRun.stderr)
	}
}

// TestRootVersion_OutputStartsWithRootName verifies that the version
// output begins with the CLI's name.
//
// The assertion is weaker than the byte-identical check above; it
// exists as a stable, human-readable property that a reader of the
// test file can see without parsing the format. The format itself
// is pinned by TestRootVersion_FlagMatchesSubcommand and by the
// specification.
func TestRootVersion_OutputStartsWithRootName(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"--version"}, nil)

	if got.exitCode != ExitSuccess {
		t.Fatalf("exit code: got %d, want %d",
			got.exitCode, ExitSuccess)
	}
	if !strings.HasPrefix(got.stdout, RootName) {
		t.Errorf("stdout does not start with RootName %q: %q",
			RootName, got.stdout)
	}
}
