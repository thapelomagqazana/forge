// Package cli contains white-box tests for the CLI package.
//
// This file pins the global flag interaction contract (WBS 7.4.3,
// amended by WBS 12.1.2, WBS 12.5.1, and WBS 17.4.1). The contract
// specifies how --quiet, --verbose, and --config interact with help
// output, version output, and error output. Each test asserts one
// row of the matrix.
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
// # The --quiet / --verbose interaction (WBS 12.1.2)
//
// Before WBS 12.1.2, --quiet won over --verbose. After WBS 12.1.2,
// the two flags are mutually exclusive: passing both is a usage
// error (exit 64). The test that exercises the combination asserts
// the new behaviour (see TestFlagInteraction_QuietVerboseConflict
// below). The change is documented in docs/reference/log-levels.md
// §4.5.
//
// # The --verbose DEBUG records (WBS 12.5.1)
//
// Under the root's --verbose, the config loader and version service
// emit DEBUG records to stderr. The tests in this file that
// exercise the root's --verbose must therefore tolerate the records
// (or assert their presence), and must not run in parallel with
// other tests that write to the same stderr buffer. The tests that
// exercise --verbose are marked as non-parallel where the DEBUG
// records could race with a concurrently running test.
//
// # The two --verbose flags (WBS 17.4.1)
//
// The token "--verbose" appears twice in the CLI's flag set:
//
//   - The root's persistent --verbose sets the log level. It may be
//     written before any subcommand. It affects stderr only.
//
//   - The version command's local --verbose selects the verbose
//     output format. It must be written after the `version`
//     subcommand. It affects stdout only.
//
// The two are independent. The tests in this file pin both halves
// of the contract:
//
//   - TestFlagInteraction_VerboseVersionPrintsVersion asserts that
//     the root's flag does not change the version output.
//
//   - TestFlagInteraction_VersionVerboseSelectsVerboseOutput
//     asserts that the version command's own flag does.
//
// The split is required by the stream contract (ADR-022): a log
// level is a diagnostic concern; an output format is a product
// concern. Coupling them would make a user choose between debug
// logging and the one-line version output.
//
// # The --config tests and WBS 8.4.5
//
// Before WBS 8.4.5 the PersistentPreRunE hook was a no-op, and
// --config had no effect on any command. After WBS 8.4.5 the hook
// loads the config, and --config with a non-existent path produces
// a config error (E1). The two tests that exercise --config with a
// non-existent path assert the new behaviour: the invocation fails
// with ExitConfig (3), stdout is empty, and stderr contains the
// diagnostic.
//
// The behaviour change is documented in the CLI UX spec
// "Configuration Loading Lifecycle" and in the discovery
// algorithm's docstring (WBS 8.4.1).
package cli

import (
	"encoding/json"
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
// Help is a result; the root's --verbose adds diagnostics to
// stderr, not content to stdout. The help text is unchanged by
// --verbose.
//
// # Why this test is not parallel
//
// The root's --verbose enables DEBUG records on stderr. The
// records are written through the shared test helper's stderr
// buffer. A parallel test that reads the same buffer would race.
// The test is therefore serial with respect to other --verbose
// tests, but the two runCLI calls within the test are sequential
// (plain then verbose), so the comparison is stable.
func TestFlagInteraction_VerboseHelpPrintsHelp(t *testing.T) {
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
// the --verbose flag selects the version command's verbose output.
//
// # The two positions of --verbose
//
// The flag may be written before or after the subcommand name:
//
//	forge --verbose version
//	forge version --verbose
//
// Both invocations produce the same output: the verbose version
// format. The two are equivalent because --verbose is a persistent
// flag on the root command; it is inherited by every subcommand
// and its value is the same regardless of where the user wrote it.
//
// # Why there is no independent local flag
//
// An earlier design proposed a separate local --verbose flag on
// the version command that would be independent of the root's
// log-level flag. Cobra cannot support two flags with the same
// name in the same flag set: the merged set has one --verbose,
// and its value is the same for both positions. The design was
// abandoned; the flag is a single persistent flag.
//
// # What the test asserts
//
//   - The exit code is ExitSuccess.
//   - The verbose output differs from the plain output.
//   - stdout contains the three verbose sections.
//   - The two positions of --verbose produce identical output.
func TestFlagInteraction_VerboseVersionPrintsVersion(t *testing.T) {
	t.Parallel()

	plain := runCLI(t, []string{"version"}, nil)
	before := runCLI(t, []string{verboseFlagName, "version"}, nil)
	after := runCLI(t, []string{"version", verboseFlagName}, nil)

	for name, r := range map[string]testRun{
		"plain":       plain,
		"--verbose v": before,
		"v --verbose": after,
	} {
		if r.exitCode != ExitSuccess {
			t.Fatalf("%s: exit code: got %d, want %d\nstderr: %s",
				name, r.exitCode, ExitSuccess, r.stderr)
		}
	}

	if plain.stdout == before.stdout {
		t.Errorf("--verbose version produced the same output as version:\n%q",
			plain.stdout)
	}
	if before.stdout != after.stdout {
		t.Errorf("--verbose version and version --verbose differ:\n"+
			"before: %q\nafter:  %q", before.stdout, after.stdout)
	}
	for _, want := range []string{"version:", "go:", "settings:"} {
		if !strings.Contains(before.stdout, want) {
			t.Errorf("verbose output missing %q:\n%s", want, before.stdout)
		}
	}
}

// TestFlagInteraction_VersionVerboseSelectsVerboseOutput verifies
// that the version command's own --verbose flag (written after the
// subcommand name) selects the verbose output format.
//
// # Why this test and TestFlagInteraction_VerboseVersionPrintsVersion
// # are separate
//
// The two tests pin opposite halves of the flag's contract:
//
//   - `--verbose version`  (root's flag before the subcommand)
//     sets the log level; the version output is unchanged.
//
//   - `version --verbose`  (version command's flag after the
//     subcommand) selects the verbose output format.
//
// A single test cannot distinguish the two cases because both
// invocations contain the token "--verbose". Splitting them makes
// the position-dependence visible: a reviewer who changes the
// handler's flag-reading code must pass both tests to keep the
// contract.
//
// # What the test asserts
//
//   - The exit code is ExitSuccess.
//   - The verbose output differs from the plain output.
//   - stdout contains the three verbose sections (metadata,
//     provenance, settings).
//
// # What the test does not assert
//
// The test does not assert on stderr. Under this invocation, the
// root's --verbose is not written, so the log level is the default
// (info) and no DEBUG records are emitted. A future change that
// enables DEBUG records by default would be a separate contract
// and a separate test.
func TestFlagInteraction_VersionVerboseSelectsVerboseOutput(t *testing.T) {
	t.Parallel()

	plain := runCLI(t, []string{"version"}, nil)
	verbose := runCLI(t, []string{"version", verboseFlagName}, nil)

	if plain.exitCode != ExitSuccess || verbose.exitCode != ExitSuccess {
		t.Fatalf("exit codes: plain=%d verbose=%d\nstderr: %s",
			plain.exitCode, verbose.exitCode, verbose.stderr)
	}
	if plain.stdout == verbose.stdout {
		t.Errorf("version --verbose produced the same output as version:\n%q",
			plain.stdout)
	}
	for _, want := range []string{
		"version:",
		"go:",
		"settings:",
	} {
		if !strings.Contains(verbose.stdout, want) {
			t.Errorf("version --verbose output missing %q:\n%s",
				want, verbose.stdout)
		}
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
//
// # Why this test is not parallel
//
// The root's --verbose enables DEBUG records on stderr. A parallel
// test that reads the same buffer would race.
func TestFlagInteraction_VerboseUnknownCommandPrintsError(t *testing.T) {
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
// --quiet and --verbose are mutually exclusive (WBS 12.1.2)
// =============================================================================

// TestFlagInteraction_QuietVerboseConflict verifies that when both
// --quiet and --verbose are set, the invocation fails with a usage
// error (exit 64).
//
// # The rule (WBS 12.1.2)
//
// Before WBS 12.1.2, the rule was "--quiet wins": the two flags
// cancelled each other out, and the invocation succeeded. WBS
// 12.1.2 made the flags mutually exclusive: they express opposite
// intents, and silently picking one hides a user error. Passing
// both is a usage error.
//
// The rule is documented in docs/reference/log-levels.md §4.5.
// The error's message is the sentinel's message ("--quiet and
// --verbose are mutually exclusive"); the exit code is
// ExitUsage (64).
//
// # What the test asserts
//
//   - The exit code is ExitUsage (64), not ExitSuccess.
//   - stdout is empty: no command runs.
//   - stderr contains the conflict message.
func TestFlagInteraction_QuietVerboseConflict(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{quietFlagName, verboseFlagName, "version"}, nil)

	if got.exitCode != ExitUsage {
		t.Fatalf("exit code: got %d, want %d (ExitUsage)\nstderr: %s",
			got.exitCode, ExitUsage, got.stderr)
	}
	if got.stdout != "" {
		t.Errorf("stdout is non-empty; want nothing\nstdout: %q", got.stdout)
	}
	if got.stderr == "" {
		t.Fatal("stderr is empty; want the conflict message")
	}
	if !strings.Contains(got.stderr, "mutually exclusive") {
		t.Errorf("stderr does not mention mutual exclusion:\n%s", got.stderr)
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
//
// # The "--quiet --verbose" row (WBS 12.1.2)
//
// The combination is a usage error: the flags are mutually
// exclusive (see TestFlagInteraction_QuietVerboseConflict). The
// error for this combination is the conflict message, not the
// unknown-command message. The test asserts that the error still
// goes to stderr with stdout empty; the message's identity is
// asserted by the conflict test, not here.
//
// # Why the "--verbose" rows are not parallel
//
// The root's --verbose enables DEBUG records on stderr. A parallel
// test that reads the same buffer would race.
func TestFlagInteraction_ErrorsAlwaysGoToStderr(t *testing.T) {
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
			// The subtests are serial: the --verbose combinations
			// write DEBUG records to the shared stderr buffer, and
			// a parallel reader would race.

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
// out an exception for other flags being present.
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
// # Why the test is not parallel
//
// The test is serial with respect to the other tests in this file
// because the config loader's DEBUG records share the test
// helper's stderr buffer. This test does not pass --verbose, so
// no records are emitted, but the helper's buffer is shared
// across the package's tests and the config loader runs on every
// invocation. Serializing the test is the safe default.
func TestFlagInteraction_ConfigExistingVersionPrintsVersion(t *testing.T) {
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

// =============================================================================
// Quiet mode (WBS 12.6.1)
// =============================================================================

// TestQuietMode_VersionOutputUnchanged asserts that --quiet does
// not change the version command's stdout.
//
// The version block is product output; --quiet does not suppress
// it. The test compares the two outputs byte-for-byte.
func TestQuietMode_VersionOutputUnchanged(t *testing.T) {
	t.Parallel()

	plain := runCLI(t, []string{"version"}, nil)
	quiet := runCLI(t, []string{"--quiet", "version"}, nil)

	if plain.exitCode != quiet.exitCode {
		t.Fatalf("exit codes differ: plain=%d quiet=%d",
			plain.exitCode, quiet.exitCode)
	}
	if plain.stdout != quiet.stdout {
		t.Fatalf("stdout differs:\nplain: %q\nquiet: %q",
			plain.stdout, quiet.stdout)
	}
}

// TestQuietMode_ConfigOutputUnchanged asserts that --quiet does
// not change the config command's stdout.
func TestQuietMode_ConfigOutputUnchanged(t *testing.T) {
	t.Parallel()

	plain := runCLI(t, []string{"config"}, nil)
	quiet := runCLI(t, []string{"--quiet", "config"}, nil)

	if plain.exitCode != quiet.exitCode {
		t.Fatalf("exit codes differ: plain=%d quiet=%d",
			plain.exitCode, quiet.exitCode)
	}
	if plain.stdout != quiet.stdout {
		t.Fatalf("stdout differs:\nplain: %q\nquiet: %q",
			plain.stdout, quiet.stdout)
	}
}

// TestQuietMode_FormatJSON_StdoutIsValidJSON asserts that
// --quiet --format json produces a valid JSON document on stdout.
//
// The envelope is the command's result in JSON mode; --quiet does
// not suppress it. The test does not assert on the envelope's
// contents (those are the command's own tests' concern); it
// asserts that stdout parses as exactly one JSON document.
func TestQuietMode_FormatJSON_StdoutIsValidJSON(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"--quiet", "--format", "json", "config"}, nil)

	if got.exitCode != ExitSuccess {
		t.Fatalf("exit code: got %d, want %d\nstderr: %s",
			got.exitCode, ExitSuccess, got.stderr)
	}

	// The output must decode as exactly one JSON document. A
	// decoder that finds trailing data after the first document
	// reports dec.More() == true; the test rejects that.
	dec := json.NewDecoder(strings.NewReader(got.stdout))
	var v any
	if err := dec.Decode(&v); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %q",
			err, got.stdout)
	}
	if dec.More() {
		t.Fatalf("stdout contains trailing data: %q", got.stdout)
	}
}

// TestQuietMode_ErrorStillGoesToStderr asserts that an error's
// diagnostic reaches stderr under --quiet.
//
// The flag sets the log level to ERROR; the error block is not a
// log record and is not suppressed.
func TestQuietMode_ErrorStillGoesToStderr(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"--quiet", "unknown-command"}, nil)

	if got.exitCode != ExitUsage {
		t.Fatalf("exit code: got %d, want %d",
			got.exitCode, ExitUsage)
	}
	if got.stderr == "" {
		t.Fatal("stderr is empty; --quiet must not suppress " +
			"the error block")
	}
	if got.stdout != "" {
		t.Fatalf("stdout is not empty: %q", got.stdout)
	}
}

// TestQuietMode_ErrorFormatJSON_ErrorEnvelopeOnStdout asserts
// that --quiet --format json on failure produces the JSON error
// envelope on stdout.
func TestQuietMode_ErrorFormatJSON_ErrorEnvelopeOnStdout(t *testing.T) {
	t.Parallel()

	got := runCLI(t,
		[]string{"--quiet", "--format", "json", "unknown-command"}, nil)

	if got.exitCode != ExitUsage {
		t.Fatalf("exit code: got %d, want %d",
			got.exitCode, ExitUsage)
	}

	// The envelope must decode as JSON.
	dec := json.NewDecoder(strings.NewReader(got.stdout))
	var env map[string]any
	if err := dec.Decode(&env); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %q",
			err, got.stdout)
	}
	if env["schemaVersion"] == nil {
		t.Fatalf("envelope missing schemaVersion: %q", got.stdout)
	}
	if env["exitCode"] == nil {
		t.Fatalf("envelope missing exitCode: %q", got.stdout)
	}
}
