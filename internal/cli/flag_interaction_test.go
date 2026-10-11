// Package cli contains white-box tests for the CLI package.
//
// This file pins the global flag interaction contract (WBS 7.4.3).
// The contract specifies how --quiet, --verbose, and --config
// interact with help output, version output, and error output.
// Each test asserts one row of the matrix.
//
// # How the tests invoke the CLI
//
// Each test invokes the CLI through runCLI, which routes through
// executeWithOptions. The invocation exercises the same path
// production code uses: argument parsing, pre-parse validation,
// command dispatch, flag reading, and error rendering.
//
// # What the tests assert
//
// The tests assert the stream boundary for each interaction. A
// result (help, version) is asserted on stdout; a diagnostic
// (error) is asserted on stderr. The flags do not change the
// boundary; the tests verify that.
//
// # The --config tests and WBS 8.4.5
//
// Before WBS 8.4.5 the PersistentPreRunE hook was a no-op, and
// --config had no effect on any command. After WBS 8.4.5 the
// hook loads the config, and --config with a non-existent path
// produces a config error (E1). The two tests that exercise
// --config with a non-existent path assert the new behaviour:
// the invocation fails with ExitConfig (3), stdout is empty,
// and stderr contains the diagnostic.
//
// The behaviour change is documented in the CLI UX spec
// "Configuration Loading Lifecycle" and in the discovery
// algorithm's docstring (WBS 8.4.1).
package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// =============================================================================
// --quiet and help
// =============================================================================

// TestFlagInteraction_QuietHelpPrintsHelp verifies that
// `forge --quiet --help` prints the help to stdout.
//
// Help is a result; --quiet suppresses diagnostics, not results.
// The help text is unchanged by --quiet.
func TestFlagInteraction_QuietHelpPrintsHelp(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{quietFlagName, "--help"}, nil)

	if got.exitCode != ExitSuccess {
		t.Fatalf("exit code: got %d, want %d\nstderr: %s",
			got.exitCode, ExitSuccess, got.stderr)
	}
	if got.stdout == "" {
		t.Errorf("stdout is empty; want help output\n"+
			"stderr: %q", got.stderr)
	}
	if got.stderr != "" {
		t.Errorf("stderr is non-empty; want nothing\nstderr: %q",
			got.stderr)
	}
}

// TestFlagInteraction_VerboseHelpPrintsHelp verifies that
// `forge --verbose --help` prints the help to stdout.
//
// Help is a result; --verbose adds diagnostics to stderr, not
// content to stdout. The help text is unchanged by --verbose.
func TestFlagInteraction_VerboseHelpPrintsHelp(t *testing.T) {
	t.Parallel()

	plain := runCLI(t, []string{"--help"}, nil)
	verbose := runCLI(t, []string{verboseFlagName, "--help"}, nil)

	if plain.exitCode != ExitSuccess || verbose.exitCode != ExitSuccess {
		t.Fatalf("exit codes: plain=%d verbose=%d",
			plain.exitCode, verbose.exitCode)
	}
	if plain.stdout != verbose.stdout {
		t.Errorf("help output differs with --verbose:\n"+
			"plain:   %q\nverbose: %q",
			plain.stdout, verbose.stdout)
	}
}

// =============================================================================
// --quiet and version
// =============================================================================

// TestFlagInteraction_QuietVersionPrintsVersion verifies that
// `forge --quiet version` prints the version to stdout.
//
// Version is a result; --quiet suppresses diagnostics, not results.
func TestFlagInteraction_QuietVersionPrintsVersion(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{quietFlagName, "version"}, nil)

	if got.exitCode != ExitSuccess {
		t.Fatalf("exit code: got %d, want %d\nstderr: %s",
			got.exitCode, ExitSuccess, got.stderr)
	}
	if got.stdout == "" {
		t.Errorf("stdout is empty; want version output\n"+
			"stderr: %q", got.stderr)
	}
	if got.stderr != "" {
		t.Errorf("stderr is non-empty; want nothing\nstderr: %q",
			got.stderr)
	}
}

// TestFlagInteraction_VerboseVersionPrintsVersion verifies that
// `forge --verbose version` prints the version to stdout.
//
// Version is a result; --verbose adds diagnostics to stderr, not
// content to stdout. The version output is unchanged by --verbose.
func TestFlagInteraction_VerboseVersionPrintsVersion(t *testing.T) {
	t.Parallel()

	plain := runCLI(t, []string{"version"}, nil)
	verbose := runCLI(t, []string{verboseFlagName, "version"}, nil)

	if plain.exitCode != ExitSuccess || verbose.exitCode != ExitSuccess {
		t.Fatalf("exit codes: plain=%d verbose=%d",
			plain.exitCode, verbose.exitCode)
	}
	if plain.stdout != verbose.stdout {
		t.Errorf("version output differs with --verbose:\n"+
			"plain:   %q\nverbose: %q",
			plain.stdout, verbose.stdout)
	}
}

// =============================================================================
// --quiet and errors
// =============================================================================

// TestFlagInteraction_QuietUnknownCommandPrintsError verifies that
// `forge --quiet unknown-cmd` prints the error to stderr.
//
// Errors are never suppressed by --quiet. The error output is
// identical to the output without --quiet.
func TestFlagInteraction_QuietUnknownCommandPrintsError(t *testing.T) {
	t.Parallel()

	plain := runCLI(t, []string{boundaryFixtureUnknownCommand}, nil)
	quiet := runCLI(t, []string{quietFlagName, boundaryFixtureUnknownCommand}, nil)

	if plain.exitCode != ExitUsage || quiet.exitCode != ExitUsage {
		t.Fatalf("exit codes: plain=%d quiet=%d",
			plain.exitCode, quiet.exitCode)
	}
	if quiet.stderr == "" {
		t.Errorf("stderr is empty; want the error\n"+
			"stdout: %q", quiet.stdout)
	}
	if quiet.stdout != "" {
		t.Errorf("stdout is non-empty; want nothing\n"+
			"stdout: %q", quiet.stdout)
	}
	if plain.stderr != quiet.stderr {
		t.Errorf("error output differs with --quiet:\n"+
			"plain: %q\nquiet: %q",
			plain.stderr, quiet.stderr)
	}
}

// =============================================================================
// --verbose and errors
// =============================================================================

// TestFlagInteraction_VerboseUnknownCommandPrintsError verifies
// that `forge --verbose unknown-cmd` prints the error to stderr.
//
// The WBS 7.4.3 matrix says --verbose "adds a stack trace" to
// errors. Phase 2 does not implement a stack-trace mode; the
// behaviour is deferred to a future WBS item. This test asserts
// the boundary and the error message; the stack-trace assertion
// is added when the mechanism is implemented.
//
// # What the test does not assert
//
// The test does not assert that --verbose adds content to stderr
// in Phase 2. The Phase 2 behaviour is that --verbose does not
// change the error output; a future WBS item adds the
// stack-trace mode and amends this test.
func TestFlagInteraction_VerboseUnknownCommandPrintsError(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{verboseFlagName, boundaryFixtureUnknownCommand}, nil)

	if got.exitCode != ExitUsage {
		t.Fatalf("exit code: got %d, want %d\nstderr: %s",
			got.exitCode, ExitUsage, got.stderr)
	}
	if got.stderr == "" {
		t.Errorf("stderr is empty; want the error\nstdout: %q", got.stdout)
	}
	if got.stdout != "" {
		t.Errorf("stdout is non-empty; want nothing\nstdout: %q", got.stdout)
	}
}

// =============================================================================
// --quiet wins over --verbose
// =============================================================================

// TestFlagInteraction_QuietWinsOverVerbose verifies that when both
// --quiet and --verbose are set, --quiet wins.
//
// The rule is documented in the CLI UX spec § 4.11. The test
// asserts that a successful command produces the same output as
// without either flag; the two flags cancel each other out, and
// the effective behaviour is "quiet".
func TestFlagInteraction_QuietWinsOverVerbose(t *testing.T) {
	t.Parallel()

	plain := runCLI(t, []string{"version"}, nil)
	both := runCLI(t, []string{quietFlagName, verboseFlagName, "version"}, nil)

	if plain.exitCode != ExitSuccess || both.exitCode != ExitSuccess {
		t.Fatalf("exit codes: plain=%d both=%d",
			plain.exitCode, both.exitCode)
	}
	if plain.stdout != both.stdout {
		t.Errorf("version output differs with both flags:\n"+
			"plain: %q\nboth:  %q",
			plain.stdout, both.stdout)
	}
}

// =============================================================================
// --quiet does not suppress stdout
// =============================================================================

// TestFlagInteraction_QuietDoesNotSuppressStdout verifies that
// --quiet does not suppress any command's stdout.
//
// The test iterates the registry and, for each command, invokes it
// with --quiet and asserts that stdout is non-empty (for commands
// that produce stdout on success).
//
// # What the test does not assert
//
// The test does not assert stdout's content. Content is the job
// of each command's own tests. This test asserts only that stdout
// is not suppressed.
func TestFlagInteraction_QuietDoesNotSuppressStdout(t *testing.T) {
	t.Parallel()

	for _, cmd := range allRegisteredCommands() {
		cmd := cmd
		name := commandName(cmd.Use)
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := runCLI(t, []string{quietFlagName, name}, nil)

			// The command may fail (for example, the config
			// placeholder returns an error). The check is on
			// stdout: if the command succeeded, stdout must
			// not be suppressed by --quiet.
			if got.exitCode == ExitSuccess && got.stdout == "" {
				t.Errorf("--quiet suppressed stdout on success\n"+
					"stderr: %q", got.stderr)
			}
		})
	}
}

// =============================================================================
// Errors always go to stderr
// =============================================================================

// TestFlagInteraction_ErrorsAlwaysGoToStderr verifies that error
// output goes to stderr regardless of the flags.
//
// The test iterates the flag combinations that could plausibly
// affect the boundary: no flag, --quiet, --verbose, both. For
// each combination, an unknown command's error is asserted on
// stderr, with stdout empty.
func TestFlagInteraction_ErrorsAlwaysGoToStderr(t *testing.T) {
	t.Parallel()

	combinations := []struct {
		name string
		args []string
	}{
		{"no flags", []string{}},
		{"--quiet", []string{quietFlagName}},
		{"--verbose", []string{verboseFlagName}},
		{"--quiet --verbose", []string{quietFlagName, verboseFlagName}},
	}

	for _, combo := range combinations {
		combo := combo
		t.Run(combo.name, func(t *testing.T) {
			t.Parallel()

			args := append([]string{}, combo.args...)
			args = append(args, boundaryFixtureUnknownCommand)

			got := runCLI(t, args, nil)

			if got.exitCode != ExitUsage {
				t.Fatalf("exit code: got %d, want %d\nstderr: %s",
					got.exitCode, ExitUsage, got.stderr)
			}
			if got.stderr == "" {
				t.Errorf("stderr is empty; want the error\nstdout: %q",
					got.stdout)
			}
			if got.stdout != "" {
				t.Errorf("stdout is non-empty; want nothing\nstdout: %q",
					got.stdout)
			}
			if !strings.HasPrefix(got.stderr, "Error: ") {
				t.Errorf("stderr does not start with %q:\n%s",
					"Error: ", got.stderr)
			}
		})
	}
}

// =============================================================================
// --config and the missing path
// =============================================================================
//
// The two tests below replace earlier tests that asserted "--config
// has no effect". After WBS 8.4.5 the hook loads the config, and
// an explicit path that names a missing file is a hard error (E1).
// The tests assert the new behaviour.

// TestFlagInteraction_ConfigMissingIsConfigError verifies that
// `forge --config nonexistent.yml version` fails with the
// configuration exit code.
//
// # Why the behaviour is "fail with ExitConfig"
//
// The discovery algorithm (WBS 8.4.1) treats an explicit path
// that names a non-existent file as an error. The path is the
// user's intent: they asked for the file at that path to be
// loaded. If the file does not exist, the invocation fails with
// a config error (E1), and the CLI maps the error to
// ExitConfig (3).
//
// # Why the earlier test expected success
//
// An earlier draft of the CLI had a no-op PersistentPreRunE
// hook; the config was not loaded. In that draft, --config had
// no effect, and the version command succeeded regardless of the
// path. The current tree wires the loader into the hook (WBS
// 8.4.5), and the missing path is now a hard error. This test
// replaces the earlier one, which asserted the draft's
// behaviour.
//
// # What the test asserts
//
//   - The exit code is ExitConfig (3), not ExitUsage (2) and not
//     ExitSuccess (0).
//   - stdout is empty: the version command's RunE is not
//     reached.
//   - stderr contains the diagnostic, and the diagnostic names
//     the missing path.
func TestFlagInteraction_ConfigMissingIsConfigError(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{
		configFlagName,
		nonexistentConfigPath,
		"version",
	}, nil)

	if got.exitCode != ExitConfig {
		t.Fatalf("exit code: got %d, want %d (ExitConfig)\nstderr: %s",
			got.exitCode, ExitConfig, got.stderr)
	}
	if got.stdout != "" {
		t.Errorf("stdout should be empty on error: %q", got.stdout)
	}
	if got.stderr == "" {
		t.Fatalf("stderr should contain a diagnostic")
	}
	if !strings.Contains(got.stderr, nonexistentConfigPath) {
		t.Errorf("stderr does not name the missing path:\n%s", got.stderr)
	}
}

// TestFlagInteraction_ConfigMissingHelpPrintsHelp verifies that
// `forge --config nonexistent.yml --help` prints the help to
// stdout and exits with success.
//
// # Why the missing --config path does not fail the invocation
//
// Cobra's --help flag is a pre-run interception flag: when Cobra
// sees it, it short-circuits the hook chain and prints the help
// text directly. The PersistentPreRunE hook (WBS 8.4.5) is not
// invoked, and the --config flag's value is never read. The
// missing path is therefore not an error for this invocation.
//
// This is the correct behaviour: the user asked for help, and
// help is what they receive. The --config flag is present but is
// not consumed; the missing path is irrelevant to the request.
//
// The CLI's help contract (docs/cli-ux-spec.md § 4.9) says all
// help goes to stdout and exits 0. The contract does not carve
// out an exception for other flags being present. The rule is
// uniform: any invocation that produces help succeeds.
//
// # What the test asserts
//
//   - The exit code is ExitSuccess.
//   - stdout is non-empty: the help text was printed.
//   - stderr is empty: no diagnostic.
//
// # What the test does not assert
//
// The test does not assert the help text's content. Content is
// the job of the help tests. This test asserts only the stream
// boundary and the exit code for this flag combination.
func TestFlagInteraction_ConfigMissingHelpPrintsHelp(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{
		configFlagName,
		nonexistentConfigPath,
		"--help",
	}, nil)

	if got.exitCode != ExitSuccess {
		t.Fatalf("exit code: got %d, want %d\nstderr: %s",
			got.exitCode, ExitSuccess, got.stderr)
	}
	if got.stdout == "" {
		t.Errorf("stdout is empty; want help output\nstderr: %q",
			got.stderr)
	}
	if got.stderr != "" {
		t.Errorf("stderr is non-empty; want nothing\nstderr: %q",
			got.stderr)
	}
}

// TestFlagInteraction_ConfigExistingVersionPrintsVersion verifies
// that `forge --config <existing> version` succeeds and prints the
// version.
//
// The test complements the two missing-path tests: with a valid
// config file, the invocation succeeds. The version command's
// output is unaffected by the config, but the hook must load the
// file without error.
//
// # What the test asserts
//
//   - The exit code is ExitSuccess.
//   - stdout is non-empty: the version command produced its
//     output.
//   - stderr is empty: the hook succeeded silently.
func TestFlagInteraction_ConfigExistingVersionPrintsVersion(t *testing.T) {
	t.Parallel()

	// Write a minimal valid config file into a temp directory.
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "forge.yaml")
	if err := os.WriteFile(cfgPath, []byte("log_level: warn\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	got := runCLI(t, []string{
		configFlagName,
		cfgPath,
		"version",
	}, nil)

	if got.exitCode != ExitSuccess {
		t.Fatalf("exit code: got %d, want %d\nstderr: %s",
			got.exitCode, ExitSuccess, got.stderr)
	}
	if got.stdout == "" {
		t.Errorf("stdout is empty; want version output\nstderr: %q",
			got.stderr)
	}
	if got.stderr != "" {
		t.Errorf("stderr should be empty on success: %q", got.stderr)
	}
}
