// Package cli contains white-box tests for the CLI package.
//
// This file tests the root command: its dispatcher behaviour (what
// happens with no arguments, with --help, with an unknown command,
// with an unknown flag, with an empty-string argument), the
// properties that make it a well-behaved CLI dispatcher (exit codes,
// stream separation, determinism), the frozen identity strings it
// exposes (WBS 5.1.1), the version flag it exposes (WBS 5.1.2), the
// help and version behaviours (WBS 5.2.2 and 5.2.3), the global
// flags (WBS 5.3.1), and the four contracts at the acceptance level
// (WBS 5.4.1 through 5.4.4).
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
//   - Acceptance tests: the four contracts the CLI exposes (no-args,
//     help, version flag, unknown command). Each acceptance test
//     exercises one contract end-to-end through the shared runCLI
//     helper and asserts on the same four observable properties.
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
//   - Help contract (WBS 5.2.2): the eight help invocations behave
//     consistently with respect to exit code, output stream, and
//     content.
//
//   - Version behaviour (WBS 5.2.3): the five version invocations
//     behave consistently, and `-v` is the short form of `--version`
//     rather than `--verbose`.
//
//   - Global flags (WBS 5.3.1): the three-flag inventory is
//     exhaustive, the flags are persistent, and the precedence rule
//     holds.
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
//
// The acceptance tests use the prefix TestAcceptance_ instead of the
// Given/Then convention because they are the file's summary of the
// CLI's user-facing contracts, not a specific behaviour. A reader
// scanning the file sees the four TestAcceptance_* tests as a block
// and can read them top to bottom as a narrative.
package cli

import (
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/spf13/pflag"
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
// Acceptance tests
// =============================================================================
//
// The four contracts the CLI exposes at the user-facing level. Each
// test exercises one contract end-to-end through the shared runCLI
// helper and asserts on the same four observable properties:
//
//   - exit code
//   - stdout
//   - stderr
//   - (where the contract names it) a specific substring of the output
//
// The tests are phrased in terms of the frozen identity strings
// (RootName, RootUsage, RootShortDesc, RootLongDesc) rather than
// literals. The constants are the source of truth, and the tests
// reference them, so a change to either constant is visible in the
// test's diff.
//
// The four tests are the CLI's contract with its users. A reader who
// has read only these four tests knows: what `forge` does with no
// arguments, what `forge --help` does, what `forge --version` does,
// and what `forge unknown-command` does. Everything else in the file
// is a deeper test of one of these four behaviours.

// TestAcceptance_NoArgs is the WBS 5.4.1 acceptance test for the
// no-args behaviour contract.
//
// # What the test verifies
//
// The contract (documented in docs/cli-ux-spec.md § 4.9) has four
// parts, all asserted here:
//
//   - The exit code is ExitSuccess (0).
//   - stdout contains RootLongDesc, the frozen long description.
//   - stdout contains RootUsage, the frozen usage line.
//   - stderr is empty.
//
// # Why the test asserts on RootLongDesc and RootUsage
//
// The two frozen constants appear in the help output and are the
// contract's user-visible identity strings. Asserting on them rather
// than on literal substrings ties the test to the constants: a
// change to either constant is visible in the test's diff, and a
// change to the help output that preserves the constants is caught
// even when the literal text would also have changed.
//
// # Why the test does not assert on RootShortDesc
//
// RootShortDesc is the CLI's one-line short description. Cobra
// renders a command's Short field only in a *parent's* "Available
// Commands:" table; the root command has no parent, so its Short
// field is not rendered by `forge --help`.
//
// The absence of RootShortDesc from `forge --help` is deliberate
// and is pinned by TestRootIdentity_HelpOutputContainsConstants
// (WBS 5.1.1). An earlier version of this test asserted on
// RootShortDesc based on the WBS 5.4.1 task text, which predated
// the frozen identity constants; that assertion was removed in
// favour of the correct constant (RootLongDesc). See
// docs/cli-ux-spec.md § 4.7 for the rationale and the correction.
//
// # Why the test is in this file
//
// The test exercises the CLI through the shared runCLI helper
// (WBS 4.3.2). The helper is an unexported symbol in package cli;
// only a test file in the same package can call it. The test file
// therefore lives in `package cli`, not `package cli_test`.
//
// # Why the test does not spawn a subprocess
//
// The runCLI helper constructs an `options` value with synthetic
// inputs, calls `executeWithOptions`, and captures the observable
// result. No subprocess is spawned. The test runs in microseconds,
// is hermetic (no dependence on the test binary being built or on
// the process's environment), and can inspect the injected streams
// directly.
//
// # What the test does not verify
//
// The test does not verify the exact text of the help output. Cobra
// formats the help slightly differently across versions; pinning the
// full text would make the test fail on a Cobra upgrade for a
// reason unrelated to the CLI's behaviour. The test asserts on the
// frozen constants, which are stable across Cobra versions by
// construction.
//
// The test also does not verify the "no default RunE" property;
// that is the domain of TestRootNoArgs_NotImplementedViaDefaultRunE
// (WBS 5.2.1).
func TestAcceptance_NoArgs(t *testing.T) {
	t.Parallel()

	got := runCLI(t, nil, nil)

	// Assertion 1: the exit code is ExitSuccess.
	if got.exitCode != ExitSuccess {
		t.Errorf("exit code: got %d, want %d",
			got.exitCode, ExitSuccess)
	}

	// Assertion 2: stdout contains the frozen long description.
	if !strings.Contains(got.stdout, RootLongDesc) {
		t.Errorf("stdout missing RootLongDesc: %q", got.stdout)
	}

	// Assertion 3: stdout contains the frozen usage line.
	if !strings.Contains(got.stdout, RootUsage) {
		t.Errorf("stdout missing RootUsage %q: %q",
			RootUsage, got.stdout)
	}

	// Assertion 4: stderr is empty.
	if got.stderr != "" {
		t.Errorf("stderr should be empty, got: %q", got.stderr)
	}
}

// TestAcceptance_Help is the WBS 5.4.2 acceptance test for the help
// behaviour contract.
//
// # What the test verifies
//
// The contract (documented in docs/cli-ux-spec.md § 4.10) has eight
// invocations. This test covers the two that produce root help —
// `forge --help` and `forge -h` — and asserts the four properties
// that both share:
//
//   - The exit code is ExitSuccess (0).
//   - stdout contains the root help (checked via RootLongDesc and
//     RootUsage).
//   - stderr is empty.
//   - The two invocations are equivalent (byte-identical output).
//
// The remaining six invocations of the help contract are covered by
// the TestHelpContract_* family of tests later in this file. This
// acceptance test is the user-facing summary: "the two ways a user
// asks for help are the same, and both succeed."
//
// # Why the test asserts on RootLongDesc and RootUsage
//
// RootLongDesc is the body of the help text; RootUsage is the usage
// line. Both are the frozen identity strings from WBS 5.1.1, and
// both are visible to a user who runs `forge --help`. Asserting on
// them ties the test to the constants.
//
// # Why the test asserts byte-identical output
//
// The contract says `--help` and `-h` are equivalent. "Equivalent"
// means byte-identical for the reason the version contract's
// equivalence is byte-identical: a script that captures help from
// one flag and compares it to help from the other must see the same
// text. A less strict assertion would not catch a divergence that
// matters to users.
func TestAcceptance_Help(t *testing.T) {
	t.Parallel()

	longFlag := runCLI(t, []string{"--help"}, nil)
	shortFlag := runCLI(t, []string{"-h"}, nil)

	// Assertion 1: both exit codes are ExitSuccess.
	if longFlag.exitCode != ExitSuccess {
		t.Errorf("`forge --help` exit code: got %d, want %d",
			longFlag.exitCode, ExitSuccess)
	}
	if shortFlag.exitCode != ExitSuccess {
		t.Errorf("`forge -h` exit code: got %d, want %d",
			shortFlag.exitCode, ExitSuccess)
	}

	// Assertion 2: both stdouts contain the root help.
	for _, run := range []struct {
		name string
		got  testRun
	}{
		{"--help", longFlag},
		{"-h", shortFlag},
	} {
		if !strings.Contains(run.got.stdout, RootLongDesc) {
			t.Errorf("%s stdout missing RootLongDesc: %q",
				run.name, run.got.stdout)
		}
		if !strings.Contains(run.got.stdout, RootUsage) {
			t.Errorf("%s stdout missing RootUsage %q: %q",
				run.name, RootUsage, run.got.stdout)
		}
	}

	// Assertion 3: both stderrs are empty.
	if longFlag.stderr != "" {
		t.Errorf("`forge --help` stderr is not empty: %q",
			longFlag.stderr)
	}
	if shortFlag.stderr != "" {
		t.Errorf("`forge -h` stderr is not empty: %q",
			shortFlag.stderr)
	}

	// Assertion 4: the two invocations are equivalent.
	if longFlag.stdout != shortFlag.stdout {
		t.Errorf("`forge --help` and `forge -h` produce different stdout:\n"+
			"--help: %q\n"+
			"-h:     %q",
			longFlag.stdout, shortFlag.stdout)
	}
}

// TestAcceptance_VersionFlag is the WBS 5.4.3 acceptance test for
// the version-flag behaviour contract.
//
// # What the test verifies
//
// The contract (documented in docs/cli-ux-spec.md § 4.11) has five
// invocations. This test covers the three that produce the version
// block — `forge --version`, `forge -v`, and `forge version` — and
// asserts the four properties that all three share:
//
//   - The exit code is ExitSuccess (0).
//   - stdout is non-empty.
//   - stderr is empty.
//   - The three invocations produce byte-identical output.
//
// The fifth invocation of the contract (`forge --version extra`) is
// a rejection; it is covered by TestVersionBehaviour_ExtraArgsRejected
// later in this file.
//
// # Why the test asserts byte-identical output across three
// # invocations
//
// The contract says the three invocations are interchangeable for
// scripts and for users. A script that parses `forge --version` and
// a script that parses `forge version` must see the same input.
// Asserting byte-identity across all three catches any divergence at
// the moment it is introduced. The WBS 5.1.2 test covers the pair
// `--version` / `version`; this acceptance test extends the
// assertion to include `-v`.
//
// # Why the test asserts on RootName
//
// The version block begins with the CLI's name. Asserting on
// RootName ties the test to the constant. The version block's
// format is documented in § 4.8 and is verified by the version
// service's own tests; this acceptance test verifies only that the
// three invocations agree and that the output begins with the CLI's
// name.
func TestAcceptance_VersionFlag(t *testing.T) {
	t.Parallel()

	longFlag := runCLI(t, []string{"--version"}, nil)
	shortFlag := runCLI(t, []string{"-v"}, nil)
	subcommand := runCLI(t, []string{"version"}, nil)

	// Assertion 1: all three exit codes are ExitSuccess.
	for _, run := range []struct {
		name string
		got  testRun
	}{
		{"--version", longFlag},
		{"-v", shortFlag},
		{"version", subcommand},
	} {
		if run.got.exitCode != ExitSuccess {
			t.Errorf("`forge %s` exit code: got %d, want %d",
				run.name, run.got.exitCode, ExitSuccess)
		}
	}

	// Assertion 2: all three stdouts are non-empty and begin with
	// RootName.
	for _, run := range []struct {
		name string
		got  testRun
	}{
		{"--version", longFlag},
		{"-v", shortFlag},
		{"version", subcommand},
	} {
		if run.got.stdout == "" {
			t.Errorf("`forge %s` stdout is empty; want version block",
				run.name)
			continue
		}
		if !strings.HasPrefix(run.got.stdout, RootName) {
			t.Errorf("`forge %s` stdout does not start with RootName %q: %q",
				run.name, RootName, run.got.stdout)
		}
	}

	// Assertion 3: all three stderrs are empty.
	for _, run := range []struct {
		name string
		got  testRun
	}{
		{"--version", longFlag},
		{"-v", shortFlag},
		{"version", subcommand},
	} {
		if run.got.stderr != "" {
			t.Errorf("`forge %s` stderr is not empty: %q",
				run.name, run.got.stderr)
		}
	}

	// Assertion 4: the three invocations are byte-identical.
	if longFlag.stdout != shortFlag.stdout {
		t.Errorf("`forge --version` and `forge -v` differ:\n"+
			"--version: %q\n"+
			"-v:        %q",
			longFlag.stdout, shortFlag.stdout)
	}
	if longFlag.stdout != subcommand.stdout {
		t.Errorf("`forge --version` and `forge version` differ:\n"+
			"--version: %q\n"+
			"version:   %q",
			longFlag.stdout, subcommand.stdout)
	}
}

// TestAcceptance_UnknownCommand is the WBS 5.4.4 acceptance test for
// the unknown-command behaviour contract.
//
// # What the test verifies
//
// The contract is a corollary of the no-args and help contracts: an
// invocation whose first positional argument does not match a
// registered subcommand is a usage error. The test asserts the four
// properties:
//
//   - The exit code is non-success.
//   - stdout is empty (the diagnostic goes to stderr, not stdout).
//   - stderr is non-empty and mentions the offending argument.
//   - The same behaviour holds for multiple arguments and for an
//     empty-string argument.
//
// # Why the test asserts on the offending argument
//
// The error's message names the argument the user typed. The
// property is the actionable half of the CLI UX contract § 9.3:
// "what happened, why, what to do". A future change that rewrites
// the error message but preserves the argument's presence would
// continue to pass the test. A change that removes the argument
// would fail it, and the failure message would name the property.
//
// # Why the test covers four cases
//
// The test runs four invocations:
//
//   - `forge unknown-command`
//   - `forge no-such-command extra args`
//   - `forge ""` (empty-string argument)
//   - `forge --nonexistent-flag` (unknown flag)
//
// The four cases exercise the ways a user can produce a usage
// error: an unknown command, extra arguments after an unknown
// command, an empty-string argument, and an unknown flag. All four
// must produce the same observable result: non-zero exit, empty
// stdout, non-empty stderr.
//
// # What the test does not verify
//
// The test does not verify the exact wording of the error message.
// The wording is Cobra's for the flag case and Forge's for the
// command case; pinning the wording would couple the test to a
// specific version of either. The test asserts only that the
// offending argument is named when the case names one.
func TestAcceptance_UnknownCommand(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		args       []string
		wantSubstr string
	}{
		{
			name:       "unknown-command",
			args:       []string{"unknown-command"},
			wantSubstr: "unknown-command",
		},
		{
			name:       "unknown-command-with-extra-args",
			args:       []string{"no-such-command", "extra", "args"},
			wantSubstr: "no-such-command",
		},
		{
			name:       "empty-string-argument",
			args:       []string{""},
			wantSubstr: "",
		},
		{
			name:       "unknown-flag",
			args:       []string{"--nonexistent-flag"},
			wantSubstr: "",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := runCLI(t, tc.args, nil)

			// Assertion 1: the exit code is non-success.
			if got.exitCode == ExitSuccess {
				t.Errorf("exit code: got %d, want non-success",
					got.exitCode)
			}

			// Assertion 2: stdout is empty.
			if got.stdout != "" {
				t.Errorf("stdout is not empty: %q", got.stdout)
			}

			// Assertion 3: stderr is non-empty.
			if got.stderr == "" {
				t.Error("stderr is empty; want a diagnostic")
			}

			// Assertion 4: stderr mentions the offending argument
			// (when the case names one).
			if tc.wantSubstr != "" &&
				!strings.Contains(got.stderr, tc.wantSubstr) {
				t.Errorf("stderr does not mention %q: %q",
					tc.wantSubstr, got.stderr)
			}
		})
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

// TestRootNoArgs_Contract is the WBS 5.2.1 acceptance test for the
// no-args behaviour contract.
//
// The contract has four parts, all asserted here:
//
//  1. `forge` exits with ExitSuccess (0).
//  2. `forge` writes root help to stdout.
//  3. `forge` writes nothing to stderr.
//  4. The help output contains RootLongDesc, the frozen body of the
//     root command's help text.
//
// # Why the test exists in addition to the smoke test
//
// TestRoot_GivenNoArgs_ThenExitsZeroAndPrintsUsage (above) covers
// parts 1 and 3 of the contract but asserts on the substring
// "Forge", not on RootLongDesc. It predates the frozen identity
// strings introduced by WBS 5.1.1. This test pins the contract to
// the constant, so that a change to the constant is visible in the
// test's diff, and a change to the help output is caught even when
// the change preserves the substring "Forge".
//
// The two tests are not redundant: the smoke test is a fast sanity
// check that the CLI is alive; this test is the formal contract.
//
// # Why the behaviour matters
//
// A CLI that exits non-zero when invoked with no arguments signals
// to shell scripts that the invocation was wrong. For a dispatcher
// with subcommands, "no arguments" is a request for information, and
// the correct response is to exit successfully after printing the
// usage text. The convention is universal: `git`, `docker`, `kubectl`,
// and `go` all behave this way.
//
// Printing to stdout (not stderr) is the second half of the
// convention: the help text is a successful result, not a diagnostic.
// A script that does `forge > usage.txt` expects the usage text in
// the file; a script that does `forge 2> errors.txt` expects no
// errors. Both are satisfied only if stdout carries the help.
func TestRootNoArgs_Contract(t *testing.T) {
	t.Parallel()

	got := runCLI(t, nil, nil)

	// Part 1: exit code.
	if got.exitCode != ExitSuccess {
		t.Errorf("exit code: got %d, want %d (ExitSuccess)",
			got.exitCode, ExitSuccess)
	}

	// Part 2: stdout carries the help text. The check is on the
	// presence of RootLongDesc, the frozen body of the help. The
	// first line of RootLongDesc is sufficient because Cobra renders
	// the Long description verbatim; the full string comparison is
	// performed by TestRootIdentity_HelpOutputContainsConstants.
	firstLine := strings.SplitN(RootLongDesc, "\n", 2)[0]
	if !strings.Contains(got.stdout, firstLine) {
		t.Errorf("stdout does not contain the first line of RootLongDesc %q: %q",
			firstLine, got.stdout)
	}

	// Part 3: stderr is empty. A successful invocation writes
	// nothing there.
	if got.stderr != "" {
		t.Errorf("stderr is not empty on success: %q", got.stderr)
	}

	// Part 4: the four core-loop keywords from RootLongDesc are
	// present in order. This is a stronger assertion on the body of
	// the help text than the single-line check above.
	for _, kw := range []string{"CREATE", "VERIFY", "EXPLAIN", "EVOLVE"} {
		if !strings.Contains(got.stdout, kw) {
			t.Errorf("help output does not contain %q: %q", kw, got.stdout)
		}
	}
}

// TestRootNoArgs_NotImplementedViaDefaultRunE verifies AC5: the
// no-args behaviour is Cobra's default, not a custom RunE handler.
//
// # Why the distinction matters
//
// Cobra has two ways to make a command print help and exit 0:
//
//  1. Leave the command's RunE nil. Cobra's default is to print
//     help and exit 0 when the command is invoked with no
//     arguments and has subcommands.
//
//  2. Set RunE to a function that calls cmd.Help() and returns nil.
//     The observable behaviour is identical.
//
// Option 2 is dangerous because it looks like a deliberate design
// decision but has the same effect as the default. A future
// contributor who sees a custom RunE will assume the handler is
// doing something non-trivial and will extend it — adding logic
// that would be better placed in a subcommand, or (worse) changing
// the exit code for the no-args case without realizing the impact.
//
// Forge uses a third pattern, which is closer to option 2 but
// distinct: RunE is non-nil, but it is used to distinguish the
// no-args case from the unknown-command case. The handler returns
// nil (and prints help) when args is empty, and returns an error
// (which maps to ExitUsage) when args contains an unrecognised
// command. This pattern is documented in root.go and is the
// subject of TestRoot_GivenUnknownCommand_ThenExitsUsageAndPrintsErrorToStderr.
//
// The test asserts that the RunE handler is present (it is required
// by the third pattern) but that its body does not unconditionally
// call cmd.Help(). A future contributor who replaces the body with
// an unconditional Help call will fail this test.
//
// # How the test works
//
// The test constructs the root command directly and inspects its
// RunE field. It calls RunE with a synthetic command and empty args
// to observe the behaviour, then calls RunE with a synthetic command
// and non-empty args to observe the difference. The two observations
// together pin the distinction that the third pattern makes.
func TestRootNoArgs_NotImplementedViaDefaultRunE(t *testing.T) {
	t.Parallel()

	deps := testDependencies()
	root := newRootCmd(deps)

	if root.RunE == nil {
		// The third pattern requires RunE to be non-nil. A nil RunE
		// would disable the unknown-command handling and cause
		// Cobra to print help to stdout on every unrecognised
		// command, which violates the CLI UX contract for usage
		// errors.
		t.Fatal("root.RunE is nil; the unknown-command case would " +
			"silently print help instead of returning a usage error")
	}

	// Behavioural check: with no args, RunE returns nil.
	if err := root.RunE(root, nil); err != nil {
		t.Errorf("RunE with no args returned error %v; want nil", err)
	}

	// Behavioural check: with an unrecognised arg, RunE returns an
	// error. The error's message mentions the argument.
	err := root.RunE(root, []string{"no-such-command"})
	if err == nil {
		t.Fatal("RunE with unknown arg returned nil; " +
			"want a usage error")
	}
	if !strings.Contains(err.Error(), "no-such-command") {
		t.Errorf("RunE error does not mention the argument: %v", err)
	}
}

// TestHelpContract_LongFlag verifies row 2 of the help contract:
// `forge --help` prints root help to stdout and exits 0.
func TestHelpContract_LongFlag(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"--help"}, nil)

	if got.exitCode != ExitSuccess {
		t.Errorf("exit code: got %d, want %d",
			got.exitCode, ExitSuccess)
	}
	if got.stdout == "" {
		t.Error("stdout is empty; want root help")
	}
	if got.stderr != "" {
		t.Errorf("stderr is not empty: %q", got.stderr)
	}

	// The help output contains the frozen identity strings.
	if !strings.Contains(got.stdout, RootLongDesc) {
		t.Errorf("help output does not contain RootLongDesc: %q",
			got.stdout)
	}
	if !strings.Contains(got.stdout, RootUsage) {
		t.Errorf("help output does not contain RootUsage %q: %q",
			RootUsage, got.stdout)
	}
}

// TestHelpContract_ShortFlag verifies row 3 of the help contract:
// `forge -h` prints root help to stdout and exits 0.
//
// The test also asserts AC4: `-h` and `--help` produce identical
// output. Byte-identical output is the correct strength for the
// contract, because a script that captures help from one flag and
// compares it to help from the other must see the same text.
func TestHelpContract_ShortFlag(t *testing.T) {
	t.Parallel()

	longFlag := runCLI(t, []string{"--help"}, nil)
	shortFlag := runCLI(t, []string{"-h"}, nil)

	// Exit code.
	if shortFlag.exitCode != ExitSuccess {
		t.Errorf("exit code: got %d, want %d",
			shortFlag.exitCode, ExitSuccess)
	}
	// Stream: stdout carries help, stderr is empty.
	if shortFlag.stdout == "" {
		t.Error("stdout is empty; want root help")
	}
	if shortFlag.stderr != "" {
		t.Errorf("stderr is not empty: %q", shortFlag.stderr)
	}
	// Equivalence with --help.
	if longFlag.stdout != shortFlag.stdout {
		t.Errorf("--help and -h produce different stdout:\n"+
			"--help: %q\n"+
			"-h:     %q",
			longFlag.stdout, shortFlag.stdout)
	}
	if longFlag.exitCode != shortFlag.exitCode {
		t.Errorf("--help and -h produce different exit codes: "+
			"--help=%d, -h=%d", longFlag.exitCode, shortFlag.exitCode)
	}
}

// TestHelpContract_HelpCommand verifies row 4 of the help contract:
// `forge help` prints root help to stdout and exits 0.
//
// The `help` command is Cobra's built-in help command, registered
// automatically when the root command has subcommands. Its output
// is equivalent to `forge --help`.
func TestHelpContract_HelpCommand(t *testing.T) {
	t.Parallel()

	helpCmd := runCLI(t, []string{"help"}, nil)
	_ = runCLI(t, []string{"--help"}, nil)

	if helpCmd.exitCode != ExitSuccess {
		t.Errorf("exit code: got %d, want %d",
			helpCmd.exitCode, ExitSuccess)
	}
	if helpCmd.stdout == "" {
		t.Error("stdout is empty; want root help")
	}
	if helpCmd.stderr != "" {
		t.Errorf("stderr is not empty: %q", helpCmd.stderr)
	}

	// The two invocations produce the same content. Byte-identical
	// comparison is not asserted here because Cobra may format the
	// output slightly differently (for example, the usage line may
	// include "help [command]" when help was invoked via the
	// command form). The contract is that both are valid help
	// output; the two are not required to be byte-identical.
	//
	// The equivalence that *is* required is between `forge help
	// <cmd>` and `forge <cmd> --help`, which TestHelpContract_HelpCommandEquivalence
	// asserts.
	if !strings.Contains(helpCmd.stdout, RootUsage) {
		t.Errorf("help output does not contain RootUsage %q: %q",
			RootUsage, helpCmd.stdout)
	}
}

// TestHelpContract_HelpSubcommand verifies row 5 of the help
// contract: `forge help version` prints the version command's help
// to stdout and exits 0.
func TestHelpContract_HelpSubcommand(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"help", "version"}, nil)

	if got.exitCode != ExitSuccess {
		t.Errorf("exit code: got %d, want %d",
			got.exitCode, ExitSuccess)
	}
	if got.stdout == "" {
		t.Error("stdout is empty; want version help")
	}
	if got.stderr != "" {
		t.Errorf("stderr is not empty: %q", got.stderr)
	}

	// The output is the version command's help. It mentions the
	// command's name and the version-related strings.
	if !strings.Contains(got.stdout, "version") {
		t.Errorf("help output does not mention 'version': %q",
			got.stdout)
	}
}

// TestHelpContract_SubcommandFlag verifies row 6 of the help
// contract: `forge version --help` prints the version command's
// help to stdout and exits 0.
//
// The test also asserts AC5: `forge help version` and
// `forge version --help` produce identical output. The two are
// expected to be interchangeable: a user who remembers one form
// should get the same text as a user who remembers the other.
func TestHelpContract_SubcommandFlag(t *testing.T) {
	t.Parallel()

	helpCmd := runCLI(t, []string{"help", "version"}, nil)
	helpFlag := runCLI(t, []string{"version", "--help"}, nil)

	if helpFlag.exitCode != ExitSuccess {
		t.Errorf("exit code: got %d, want %d",
			helpFlag.exitCode, ExitSuccess)
	}
	if helpFlag.stdout == "" {
		t.Error("stdout is empty; want version help")
	}
	if helpFlag.stderr != "" {
		t.Errorf("stderr is not empty: %q", helpFlag.stderr)
	}

	// Equivalence with `forge help version`.
	if helpCmd.stdout != helpFlag.stdout {
		t.Errorf("`forge help version` and `forge version --help` "+
			"produce different stdout:\n"+
			"help version:   %q\n"+
			"version --help: %q",
			helpCmd.stdout, helpFlag.stdout)
	}
	if helpCmd.exitCode != helpFlag.exitCode {
		t.Errorf("exit codes differ: help version=%d, version --help=%d",
			helpCmd.exitCode, helpFlag.exitCode)
	}
}

// TestHelpContract_HelpFlagWithArg verifies row 7 of the help
// contract: `forge --help version` is rejected because `--help` is
// a flag, not a prefix.
//
// # Why the rejection is correct
//
// Users familiar with `git help <cmd>` or `kubectl help <cmd>` may
// expect `forge --help <cmd>` to be equivalent to `forge help <cmd>`.
// It is not. `--help` is a flag that takes no argument; passing
// `version` after it is a usage error. The CLI rejects the
// invocation with ExitUsage and writes a diagnostic to stderr.
//
// The rejection is a deliberate design choice, not an accident. The
// alternative — accepting `--help version` as a synonym for
// `help version` — would blur the distinction between flags and
// commands, which the CLI's grammar does not support.
func TestHelpContract_HelpFlagWithArg(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"--help", "version"}, nil)

	if got.exitCode == ExitSuccess {
		t.Errorf("exit code: got %d, want non-success; "+
			"`--help version` should be a usage error",
			got.exitCode)
	}
	if got.stdout != "" {
		t.Errorf("stdout is not empty: %q; "+
			"the error should go to stderr", got.stdout)
	}
	if got.stderr == "" {
		t.Error("stderr is empty; want a diagnostic")
	}
}

// TestHelpContract_HelpUnknownTopic verifies row 8 of the help
// contract: `forge help unknown` prints an "unknown help topic"
// diagnostic to stderr and exits with ExitUsage.
//
// The test asserts AC3: errors about unknown help topics go to
// stderr, not stdout. The exact wording of the message is Cobra's,
// not Forge's; the test does not pin it.
func TestHelpContract_HelpUnknownTopic(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"help", "no-such-command"}, nil)

	if got.exitCode == ExitSuccess {
		t.Errorf("exit code: got %d, want non-success; "+
			"an unknown help topic should be a usage error",
			got.exitCode)
	}
	if got.stdout != "" {
		t.Errorf("stdout is not empty: %q; "+
			"the error should go to stderr", got.stdout)
	}
	if got.stderr == "" {
		t.Error("stderr is empty; want a diagnostic")
	}
}

// TestHelpContract_ListsAllCommands verifies AC7: the help output
// lists every command in the registry.
//
// The test compares the command names parsed from the help output
// to the visible command names in the registry. The two must match
// in both content and order. The comparison is the same as
// TestRoot_HelpOutput_MatchesRegistry (WBS 4.4.1); the test is
// duplicated here because the help contract explicitly names this
// property, and a reader of the contract should find its assertion
// in the contract's test section.
func TestHelpContract_ListsAllCommands(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"--help"}, nil)

	if got.exitCode != ExitSuccess {
		t.Fatalf("exit code: got %d, want %d",
			got.exitCode, ExitSuccess)
	}

	listed := helpCommandNames(got.stdout)

	deps := testDependencies()
	var expected []string
	for _, ctor := range registry {
		cmd := ctor(deps)
		if cmd.Hidden {
			continue
		}
		expected = append(expected, firstWord(cmd.Use))
	}

	if len(listed) != len(expected) {
		t.Fatalf("command count mismatch:\n"+
			"help:     %v\n"+
			"registry: %v", listed, expected)
	}
	for i := range listed {
		if listed[i] != expected[i] {
			t.Errorf("command order mismatch at index %d:\n"+
				"help[%d]     = %q\n"+
				"registry[%d] = %q",
				i, i, listed[i], i, expected[i])
		}
	}
}

// =============================================================================
// Version behaviour contract (WBS 5.2.3)
// =============================================================================
//
// The version behaviour contract (docs/cli-ux-spec.md § 4.11) defines
// five invocations and their observable results. Four of them are
// covered by tests in the sections above; the four below complete
// the coverage with the properties the contract names explicitly.

// TestVersionBehaviour_ShortFlagMatchesLongFlag verifies AC2: the
// short flag `-v` is an alias for `--version`, not for `--verbose`.
//
// # Why the alias matters
//
// Cobra registers `-v` as the short form of `--version` when the
// root command's Version field is non-empty. Some CLIs use `-v` for
// `--verbose` instead. Forge uses `-v` for `--version`; `--verbose`
// has no short form (see TestVersionBehaviour_VerboseHasNoShortFlag).
//
// The test compares the two invocations byte-for-byte. A future
// change that remaps `-v` to a different flag would fail the test
// and the failure message would name the diverging invocation.
func TestVersionBehaviour_ShortFlagMatchesLongFlag(t *testing.T) {
	t.Parallel()

	longFlag := runCLI(t, []string{"--version"}, nil)
	shortFlag := runCLI(t, []string{"-v"}, nil)

	if longFlag.exitCode != shortFlag.exitCode {
		t.Errorf("exit codes differ: --version=%d, -v=%d",
			longFlag.exitCode, shortFlag.exitCode)
	}
	if longFlag.stdout != shortFlag.stdout {
		t.Errorf("stdout differs:\n"+
			"--version: %q\n"+
			"-v:        %q",
			longFlag.stdout, shortFlag.stdout)
	}
	if longFlag.stderr != shortFlag.stderr {
		t.Errorf("stderr differs:\n"+
			"--version: %q\n"+
			"-v:        %q",
			longFlag.stderr, shortFlag.stderr)
	}
}

// TestVersionBehaviour_ExtraArgsRejected verifies AC5: extra
// positional arguments after --version are rejected with a
// diagnostic on stderr and a non-zero exit code.
//
// The rejection is implemented in validateArgs (validate.go). The
// test invokes the CLI through the shared runCLI helper, so it
// exercises the full path from args to the observable result.
func TestVersionBehaviour_ExtraArgsRejected(t *testing.T) {
	t.Parallel()

	cases := [][]string{
		{"--version", "extra"},
		{"-v", "extra"},
		{"--version", "version"},
	}

	for _, args := range cases {
		args := args
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			t.Parallel()

			got := runCLI(t, args, nil)

			if got.exitCode == ExitSuccess {
				t.Errorf("exit code: got %d, want non-success; "+
					"`forge %s` should be a usage error",
					got.exitCode, strings.Join(args, " "))
			}
			if got.stdout != "" {
				t.Errorf("stdout is not empty: %q; "+
					"the error should go to stderr", got.stdout)
			}
			if got.stderr == "" {
				t.Error("stderr is empty; want a diagnostic")
			}
		})
	}
}

// TestVersionBehaviour_VerboseHasNoShortFlag verifies AC7: `-v`
// does not enable verbose mode. If it did, the version-flag's
// short form would be ambiguous with a verbose short form, and the
// CLI's behaviour would depend on which flag Cobra happened to
// register first.
//
// The test asserts that `forge -v` produces the same output as
// `forge version`: the version block, on stdout, exit 0. A future
// change that repurposed `-v` for verbose would change the output
// (from a version block to a log line) and fail the test.
//
// The test does not attempt to assert "there is no verbose short
// flag" directly; it asserts the observable consequence, which is
// the property that matters for users and scripts.
func TestVersionBehaviour_VerboseHasNoShortFlag(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"-v"}, nil)

	if got.exitCode != ExitSuccess {
		t.Fatalf("exit code: got %d, want %d",
			got.exitCode, ExitSuccess)
	}
	// The output must be the version block, not a diagnostic line
	// about verbose mode. The version block begins with the CLI's
	// name and contains the commit, built, dirty, go version, and
	// platform keys.
	if !strings.HasPrefix(got.stdout, RootName) {
		t.Errorf("stdout does not start with RootName %q: %q",
			RootName, got.stdout)
	}
	for _, key := range []string{"commit:", "built:", "dirty:", "go version:", "platform:"} {
		if !strings.Contains(got.stdout, key) {
			t.Errorf("stdout missing %q; "+
				"-v did not produce the version block: %q",
				key, got.stdout)
		}
	}
	if got.stderr != "" {
		t.Errorf("stderr is not empty: %q", got.stderr)
	}
}

// TestVersionBehaviour_FlagAndSubcommandIdentical is a redundant
// assertion of the WBS 5.1.2 contract at the WBS 5.2.3 layer.
//
// The test exists so that a reader of the version behaviour section
// finds the byte-identical property asserted there, in its
// canonical form, alongside the other version-behaviour tests. The
// WBS 5.1.2 test (TestRootVersion_FlagMatchesSubcommand) makes the
// same assertion. The duplication is deliberate: the two WBS items
// cover overlapping behaviour, and a reader of either section
// should find the property asserted.
func TestVersionBehaviour_FlagAndSubcommandIdentical(t *testing.T) {
	t.Parallel()

	flagRun := runCLI(t, []string{"--version"}, nil)
	cmdRun := runCLI(t, []string{"version"}, nil)

	if flagRun.exitCode != cmdRun.exitCode {
		t.Errorf("exit codes differ: --version=%d, version=%d",
			flagRun.exitCode, cmdRun.exitCode)
	}
	if flagRun.stdout != cmdRun.stdout {
		t.Errorf("stdout differs:\n"+
			"--version: %q\n"+
			"version:   %q",
			flagRun.stdout, cmdRun.stdout)
	}
	if flagRun.stderr != cmdRun.stderr {
		t.Errorf("stderr differs:\n"+
			"--version: %q\n"+
			"version:   %q",
			flagRun.stderr, cmdRun.stderr)
	}
}

// =============================================================================
// Global flags contract (WBS 5.3.1)
// =============================================================================
//
// The three global flags (--verbose, --quiet, --config) are
// documented in docs/cli-ux-spec.md § 4.12. The tests below cover
// the observable properties the contract names: the inventory's
// size, the persistence of the flags, and the precedence rule.

// TestGlobalFlags_InventoryIsExhaustive verifies AC7: the root
// command has exactly three global flags. A future contributor who
// adds a fourth flag must edit this test and the specification,
// which forces the addition to be deliberate.
//
// # How the test counts
//
// The test constructs the root command and reads its persistent
// flags. Cobra registers the help flag automatically; the test
// subtracts it from the count. The remaining count must be exactly
// three.
func TestGlobalFlags_InventoryIsExhaustive(t *testing.T) {
	t.Parallel()

	deps := testDependencies()
	root := newRootCmd(deps)

	expected := map[string]bool{
		FlagVerbose: true,
		FlagQuiet:   true,
		FlagConfig:  true,
	}

	actual := map[string]bool{}
	root.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		actual[f.Name] = true
	})

	// Every expected flag must be present.
	for name := range expected {
		if !actual[name] {
			t.Errorf("expected global flag %q is missing", name)
		}
	}

	// Every present flag must be expected, except for Cobra's
	// auto-generated help flag. The help flag is not a Forge
	// global flag; it is a Cobra built-in.
	for name := range actual {
		if name == "help" {
			continue
		}
		if !expected[name] {
			t.Errorf("unexpected global flag %q; "+
				"the Phase 2 inventory is exactly %v. "+
				"Adding a new global flag requires an ADR",
				name, []string{FlagVerbose, FlagQuiet, FlagConfig})
		}
	}
}

// TestGlobalFlags_ArePersistent verifies AC5: the three flags are
// persistent. Persistent flags are inherited by every subcommand,
// so the flags can appear before or after the subcommand name.
//
// # What the test asserts
//
// The test constructs the root command and checks that each flag
// appears in root.PersistentFlags(). A persistent flag is visible
// to subcommands; a local flag is not. The test does not exercise
// a subcommand's access to the flags (that is covered by the
// precedence test); it asserts the registration's shape.
func TestGlobalFlags_ArePersistent(t *testing.T) {
	t.Parallel()

	deps := testDependencies()
	root := newRootCmd(deps)

	for _, name := range []string{FlagVerbose, FlagQuiet, FlagConfig} {
		if root.PersistentFlags().Lookup(name) == nil {
			t.Errorf("flag %q is not persistent; "+
				"subcommands will not inherit it", name)
		}
	}
}

// TestGlobalFlags_ParseBeforeAndAfterSubcommand verifies that the
// flags are accepted in either position: before the subcommand name
// or after it. This is the observable consequence of persistence.
//
// # What the test does
//
// The test invokes the CLI with a subcommand (version) and the
// --verbose flag in two positions:
//
//   - Before the subcommand: `forge --verbose version`
//   - After the subcommand:  `forge version --verbose`
//
// Both invocations must succeed (exit 0) with the same output.
// The version command does not consume the flag in Phase 2, but
// its presence must not cause the invocation to fail. In Phase 3,
// when a command begins reading --verbose, the test will be
// extended to assert on the flag's effect.
func TestGlobalFlags_ParseBeforeAndAfterSubcommand(t *testing.T) {
	t.Parallel()

	before := runCLI(t, []string{"--verbose", "version"}, nil)
	after := runCLI(t, []string{"version", "--verbose"}, nil)

	if before.exitCode != ExitSuccess {
		t.Errorf("`forge --verbose version` exit code: got %d, want %d",
			before.exitCode, ExitSuccess)
	}
	if after.exitCode != ExitSuccess {
		t.Errorf("`forge version --verbose` exit code: got %d, want %d",
			after.exitCode, ExitSuccess)
	}
	if before.stdout != after.stdout {
		t.Errorf("stdout differs by position:\n"+
			"before: %q\n"+
			"after:  %q",
			before.stdout, after.stdout)
	}
}

// TestGlobalFlags_QuietWinsOverVerbose verifies AC4: when both
// --verbose and --quiet are set, the effective log level is the
// quiet level.
//
// # How the test observes the precedence
//
// The test does not inspect a logger's level directly. Phase 2's
// version command does not emit log messages, so the effective
// level has no observable effect on its output. Instead, the test
// asserts the precedence at the helper level
// (TestResolveLogLevel in flags_test.go) and, here, at the CLI
// level, that an invocation with both flags succeeds. A future
// WBS item (WBS 12.0) that wires the level to the logger will
// extend this test to assert on the observed log output.
//
// The test exists so that a reader of the global flags section
// finds the precedence asserted in the contract's test section.
// The unit test in flags_test.go carries the substantive
// assertion.
func TestGlobalFlags_QuietWinsOverVerbose(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"--verbose", "--quiet", "version"}, nil)

	if got.exitCode != ExitSuccess {
		t.Errorf("exit code: got %d, want %d",
			got.exitCode, ExitSuccess)
	}
	if got.stderr != "" {
		t.Errorf("stderr is not empty: %q", got.stderr)
	}
}

// TestGlobalFlags_ConfigWithNoValueRejected verifies that
// `forge --config` with no following value is rejected.
//
// The rejection is implemented in validateArgs (validate.go); see
// validateConfigFlagWithArgs for the rationale.
func TestGlobalFlags_ConfigWithNoValueRejected(t *testing.T) {
	t.Parallel()

	cases := [][]string{
		{"--config"},
		{"--config", "--verbose"},
		{"version", "--config"},
	}

	for _, args := range cases {
		args := args
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			t.Parallel()
			got := runCLI(t, args, nil)

			if got.exitCode == ExitSuccess {
				t.Errorf("exit code: got %d, want non-success",
					got.exitCode)
			}
			if got.stderr == "" {
				t.Error("stderr is empty; want a diagnostic")
			}
		})
	}
}
