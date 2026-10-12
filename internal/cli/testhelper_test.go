// Package cli contains white-box tests for the CLI package.
//
// This file defines the shared test helper used by every command test
// in the package. The helper exercises the CLI through the same
// boundary production code uses (executeWithOptions) with synthetic
// inputs, and captures the observable outputs (exit code, stdout,
// stderr).
//
// # Why a shared helper
//
// Before WBS 4.3.2, each test file defined its own helper or
// constructed a Cobra command directly. The two patterns produced
// subtly different tests: helpers that went through executeWithOptions
// exercised the full transformation (options → Dependencies → command
// tree); helpers that constructed a Cobra command exercised only the
// command. Both are valid, but having two patterns in the same package
// is confusing and produces tests that cannot be compared.
//
// The shared helper resolves the ambiguity: every command test in the
// package goes through executeWithOptions.
//
// # Why no subprocess
//
// The helper does not spawn a subprocess. Spawning is slow
// (milliseconds per case instead of microseconds), brittle (it depends
// on the test binary being built), and opaque (it cannot inspect
// internal state). The helper covers everything a subprocess-based
// test would cover, except process-level concerns like main.go's call
// to os.Exit. Those are covered by integration tests in
// cmd/forge/binary_integration_test.go, behind the `integration` build
// tag.
//
// # What the helper captures
//
// The helper returns a testRun value with three fields:
//
//   - exitCode — the value returned by executeWithOptions. The
//     value is derived from the error's category, via
//     Category.ExitCode (WBS 10.2.1) and the boundary's
//     exitCodeOf helper (WBS 10.5.1).
//   - stdout   — the content written to the injected stdout writer.
//   - stderr   — the content written to the injected stderr writer.
//
// The three fields are sufficient to assert on every user-observable
// behaviour of a command: what it prints, where it prints it, and
// whether it succeeded.
//
// # The error boundary (WBS 10.5.1)
//
// The helpers exercise the CLI through executeWithOptions, which
// renders errors via forgeerr.Format, wraps non-*forgeerr.Error
// errors via wrappedUnexpectedError, and derives exit codes via
// Category.ExitCode. The Execute function (the process's error
// boundary) is not exercised by the helpers; its panic recovery
// is tested directly through recoveredPanicError in execute_test.go.
//
// The only test-only hook in this file is
// testOnlyRegisterPlainErrorCommand, which is used by the
// non-*forgeerr.Error wrapping test.
package cli

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"github.com/thapelomagqazana/forge/internal/config"
	"github.com/thapelomagqazana/forge/internal/filesystem"
)

// testRun captures the observable result of a CLI invocation.
//
// It is the return type of the runCLI helper. Every field is
// deterministic: the same args and env produce the same testRun,
// byte for byte.
//
// # Why exported fields
//
// The fields are exported (capitalised) so that tests can construct a
// testRun literal for comparison. A common pattern is:
//
//	want := testRun{exitCode: ExitSuccess, stdout: "..."}
//	got := runCLI(t, []string{"version"}, nil)
//	if got != want { ... }
//
// The fields are not part of the package's public API; the struct is
// unexported, and no test outside `package cli` can name it.
type testRun struct {
	// exitCode is the value returned by executeWithOptions. It is
	// the same value main.go passes to os.Exit. The value is the
	// error's Category().ExitCode(), or ExitSuccess for a nil
	// error; see WBS 10.5.1.
	exitCode int

	// stdout is the content written to the injected stdout writer.
	stdout string

	// stderr is the content written to the injected stderr writer.
	stderr string
}

// updateGolden is the flag that turns on golden-file rewriting for
// the package's golden tests. It is declared once here so that
// every golden test file shares the same flag.
//
// # Usage
//
//	go test ./internal/cli -run TestConfigGolden -update
//
// The flag is intentionally not the default. A test run without
// -update compares against the committed golden files; a run with
// -update rewrites them. A contributor who runs -update without
// inspecting the diff has turned the golden files into a rubber
// stamp; the flag's docstring says "rewrite" because that is what
// it does — the review is the contributor's responsibility, not
// the test's.
var updateGolden = flag.Bool("update", false,
	"rewrite golden files with the current output")

// testConfigPath is the resolved config file path used by the
// tests that assert on the `with-file` golden files.
//
// # Why a shared constant
//
// Two test files compare against the with-file golden files:
//
//   - config_golden_test.go: TestConfigGolden_TextWithFile and
//     TestConfigGolden_JSONWithFile
//   - config_interaction_test.go: the `--config <path>` row of
//     TestConfigInteraction_FullOutput
//
// The three tests inject a Config whose SourcePath is this
// constant; the golden files contain it verbatim. If two tests
// injected different paths and shared a golden file, one would
// fail. The constant makes the two tests agree by construction.
//
// The value is deliberately distinctive so that a mismatch
// produces an obvious diff. It is not a path the tests expect to
// exist on disk; the Config is injected, not loaded.
const testConfigPath = "/home/user/.config/forge/config.yaml"

// runCLI invokes the CLI with the given arguments and environment and
// returns the captured observable result.
//
// The helper:
//
//  1. Constructs an options value with synthetic inputs. stdout and
//     stderr are fresh bytes.Buffers; stdin is an empty reader; env
//     is a lookup function over the caller-supplied map.
//  2. Calls executeWithOptions with the options value.
//  3. Returns a testRun containing the exit code and the captured
//     writers.
//
// The helper does not spawn a subprocess, does not touch the real
// process environment, and does not read or write the filesystem
// except through the CLI's own code paths.
//
// # Argument shape
//
// args is the argument list as the user would type it, excluding the
// program name. The helper passes it to options.args unchanged. An
// empty or nil slice is treated by the CLI as "no subcommand"; a slice
// whose first element is a command name dispatches to that command.
//
// # Environment shape
//
// env is a map of environment variables visible to the CLI during the
// invocation. The helper converts it to a lookup function:
//
//	env: func(k string) string { return env[k] }
//
// A nil map is valid and produces a lookup that always returns the
// empty string. This matches the behaviour of a real process in which
// no environment variables are set.
//
// Tests that need to assert on the absence of a variable pass a map
// that does not contain the key. Tests that need to assert on the
// presence of a variable pass a map that contains it.
//
// # Concurrency
//
// runCLI is safe for concurrent use. Every call allocates its own
// buffers and its own options value. No package-level state is
// touched. Tests may call runCLI from parallel subtests without
// synchronisation.
//
// The one exception is the registry: a caller that installs a
// test-only command via testOnlyRegisterPlainErrorCommand mutates
// the registry for the duration of the test. A parallel test that
// reads the registry during that window would race. The hook is
// therefore documented as serial-only, and the tests that call it
// are not marked t.Parallel.
//
// # Why the helper takes *testing.T
//
// The helper takes *testing.T so that it can call t.Helper(). The
// t.Helper() call marks the helper's frame as not-a-test, so that a
// failure inside the helper (for example, a panic from a nil writer)
// is reported with the caller's file and line number, not the
// helper's. This is the standard convention for test helpers and is
// what makes failures easy to locate.
func runCLI(t *testing.T, args []string, env map[string]string) testRun {
	t.Helper()

	var stdout, stderr bytes.Buffer

	opts := options{
		args:     args,
		stdin:    strings.NewReader(""),
		stdout:   &stdout,
		stderr:   &stderr,
		env:      func(k string) string { return env[k] },
		rootPath: "",
	}

	code := executeWithOptions(opts)

	return testRun{
		exitCode: code,
		stdout:   stdout.String(),
		stderr:   stderr.String(),
	}
}

// runCLIWithRoot is the same as runCLI but binds the CLI's filesystem
// abstraction to the given root directory.
//
// The helper exists for commands that resolve paths relative to a
// root. In Phase 2, no command does; the helper is provided so that
// future command tests have a single place to set the root, rather
// than constructing options values inline.
func runCLIWithRoot(t *testing.T, args []string, env map[string]string, rootPath string) testRun {
	t.Helper()

	var stdout, stderr bytes.Buffer

	opts := options{
		args:     args,
		stdin:    strings.NewReader(""),
		stdout:   &stdout,
		stderr:   &stderr,
		env:      func(k string) string { return env[k] },
		rootPath: rootPath,
	}

	code := executeWithOptions(opts)

	return testRun{
		exitCode: code,
		stdout:   stdout.String(),
		stderr:   stderr.String(),
	}
}

// runCLIWithConfig is a variant of runCLI that injects a pre-built
// Config into the dispatcher. It exists so that tests can exercise
// the config command with a Config whose fields have been set to
// specific values.
//
// # How it works
//
// The function constructs an options value with the injected
// Config, then calls executeWithOptions. The mechanics mirror
// runCLI; the only difference is the Config field.
//
// # Threading the Config
//
// The options struct must carry a Config field (or a similar
// mechanism) that buildDependencies threads into the Dependencies
// value it constructs. If the options struct does not have such a
// field, one must be added, and buildDependencies must be updated
// to use it when non-nil. Without this plumbing, the injected
// Config is discarded and the command runs with the default
// (zero-value) Config.
//
// # Why not extend runCLI
//
// runCLI's signature (args, env) is used by dozens of tests.
// Adding a config parameter would require touching every call
// site. A sibling function with a distinct name is the idiomatic
// Go choice: the common case stays terse, and the new case is
// explicit at the call site.
//
// # Concurrency
//
// runCLIWithConfig is safe for concurrent use. Every call
// allocates its own buffers and its own options value. No
// package-level state is touched.
func runCLIWithConfig(
	t *testing.T,
	args []string,
	env map[string]string,
	cfg *config.Config,
) testRun {
	t.Helper()

	var stdout, stderr bytes.Buffer

	opts := options{
		args:     args,
		stdin:    strings.NewReader(""),
		stdout:   &stdout,
		stderr:   &stderr,
		env:      func(k string) string { return env[k] },
		rootPath: "",
		config:   cfg,
	}

	code := executeWithOptions(opts)

	return testRun{
		exitCode: code,
		stdout:   stdout.String(),
		stderr:   stderr.String(),
	}
}

// runCLIWithFiles invokes the CLI with a synthetic configuration
// derived from a fixtures map (file path → content) and an env map.
//
// It is the helper the acceptance tests use to express a scenario
// as inputs (fixtures, env) rather than as a pre-built Config. The
// helper writes the fixtures to a temporary directory, loads the
// configuration with config.Load, applies the env layer with
// config.OverrideFromEnv, and delegates to runCLIWithConfig.
//
// # Why the loader is used rather than a synthesized Config
//
// The config package deliberately hides its source-tracking writer
// (setSource) so that callers cannot construct a Config whose
// sources disagree with its fields. The exported read path is
// SourceFor; the exported write path is the loader. The helper
// uses the loader — the same entry point production uses — so that
// the Config the command sees is one the loader produced, with the
// correct source attribution for every field.
//
// # Fixture format
//
// Each fixture is a config file's content, in the schema's YAML
// subset (`<key>: <value>` per line). The fixture named
// "config.yaml" is treated as the resolved configuration file; its
// content is written to a temp file whose path becomes the
// Config's SourcePath.
//
// A fixture with any other name is currently ignored; the loader
// does not discover arbitrary files, so the helper does not either.
//
// # Env format
//
// The env map's keys are the environment variable names from
// internal/config's schema: FORGE_LOG_LEVEL and
// FORGE_OUTPUT_FORMAT. The helper reads those keys (via
// config.EnvVarNameFor) and applies their values via
// config.OverrideFromEnv.
//
// # Precedence
//
// The helper applies layers in the schema's order: file first,
// then env. CLI flags are not represented by this helper; flag
// interaction is exercised separately by the interaction tests
// (WBS 9.4.2).
//
// # Concurrency
//
// runCLIWithFiles is safe for concurrent use. Every call writes to
// its own t.TempDir() and constructs its own Config.
func runCLIWithFiles(
	t *testing.T,
	args []string,
	env map[string]string,
	fixtures map[string]string,
) testRun {
	t.Helper()

	cfg := loadFixtureConfig(t, fixtures)

	// Env layer: apply the environment variables the schema
	// declares. OverrideFromEnv is the exported function the
	// loader calls; it records SourceEnv on the fields it sets.
	lookup := func(k string) string {
		if env == nil {
			return ""
		}
		return env[k]
	}
	var err error
	cfg, err = config.OverrideFromEnv(cfg, lookup)
	require.NoError(t, err, "OverrideFromEnv: %v", err)

	return runCLIWithConfig(t, args, env, &cfg)
}

// loadFixtureConfig writes the fixture named "config.yaml" to a
// temp file and loads it with config.Load. If no such fixture is
// present, it returns config.Defaults().
//
// The temp file lives under t.TempDir(), which Go removes when the
// test ends. The path passed to config.Load is the temp file's
// absolute path, so the resulting Config's SourcePath is that path.
//
// # Why config.Defaults
//
// When there is no fixture, the loader's semantics are "no file
// found; use the defaults." config.Defaults() produces exactly
// that: a Config with the schema's default values. The helper
// calls it rather than constructing the Config by hand, so that
// the defaults come from the schema's single source of truth.
func loadFixtureConfig(t *testing.T, fixtures map[string]string) config.Config {
	t.Helper()

	if fixtures == nil {
		return config.Defaults()
	}
	content, ok := fixtures["config.yaml"]
	if !ok {
		return config.Defaults()
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write fixture %s: %v", path, err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("load fixture %s: %v", path, err)
	}
	return cfg
}

// testDependencies returns a Dependencies value with non-nil writers,
// suitable for tests that construct commands but do not execute them.
//
// # What the helper provides
//
//   - Stdout and Stderr are fresh bytes.Buffer values. Tests that
//     want to assert on output should call runCLI instead, which
//     captures the writers it allocates.
//
//   - Config is a fresh &config.Config{}. In Phase 2 the struct is
//     empty; the pointer is non-nil so that commands may read its
//     fields without a nil check.
//
//   - Logger is a no-op logger. Tests that want to assert on log
//     output construct their own logger and pass it in a Dependencies
//     value they build themselves.
//
//   - FS is an OS-backed filesystem bound to an empty root. Tests
//     that need a specific root call runCLIWithRoot instead.
//
//   - Env is a lookup function that always returns the empty string.
//     Tests that want to inject environment variables call runCLI
//     with an env map.
//
// # When to use this helper
//
// Use testDependencies for tests that construct a command and
// inspect its metadata (Use, Short, Long, Hidden, Args) but do not
// execute it. For tests that execute a command, use runCLI, which
// captures the command's observable output.
//
// # Why the helper is not named newTestDependencies
//
// The name follows the convention used by the package's other test
// helpers (testRun, testHelper, etc.): a noun phrase beginning with
// "test" describes a value or a fixture; a verb phrase beginning
// with "new" describes a constructor that produces a distinct value
// on every call. testDependencies produces a distinct value on every
// call, but its name is read as "the test dependencies", which is a
// stable, reusable fixture. The two conventions are documented in
// the package docstring of testhelper_test.go.
//
// # Concurrency
//
// testDependencies is safe for concurrent use. Every call allocates
// its own buffers and its own Dependencies value. No package-level
// state is touched.
func testDependencies() Dependencies {
	return Dependencies{
		Config: &config.Config{},
		Logger: newNoopLogger(),
		FS:     filesystem.NewOSFS(""),
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Env:    func(string) string { return "" },
	}
}

// helpCommandNames returns the names of the visible commands listed
// in any Cobra help output, in the order they appear.
//
// # What "any help output" means
//
// Cobra's help output has the same structure regardless of whether
// it was produced by `forge --help`, `forge help`, or
// `forge version --help`: it contains a "Usage:" section, an
// "Available Commands:" section (when the command has subcommands),
// a "Flags:" section, and a closing line that points at
// `--help` for more information.
//
// The helper extracts the command names from the "Available
// Commands:" section. For a leaf command (like `version`), the
// section is absent and the helper returns nil.
//
// # Why the helper is general
//
// The same parsing logic is used by:
//
//   - TestRoot_HelpOutput_MatchesRegistry (checks that the visible
//     commands match the registry)
//   - The WBS 5.2.2 help contract tests (check that the help output
//     lists every command regardless of which invocation produced
//     it)
//
// Sharing the logic ensures the two sets of tests agree on what
// "the help output" contains. If a future change to Cobra's help
// format requires updating the parser, both tests see the update.
func helpCommandNames(help string) []string {
	return extractVisibleCommandNamesFromHelp(help)
}

// assertOnlyStdout verifies that got has output on stdout and none
// on stderr.
//
// # Why the helper exists
//
// The output stream boundary (WBS 7.4.1) requires each piece of
// output to go to exactly one stream. A test that asserts the
// boundary for a successful command uses this helper: it asserts
// that stdout is non-empty and stderr is empty. The helper
// produces a specific failure message if either assertion fails.
//
// # What the helper does not assert
//
// The helper does not assert the content of stdout. Content is the
// job of the command's own tests. The helper asserts only the
// stream: output on stdout, nothing on stderr.
//
// # Exit code
//
// The helper does not assert the exit code. A caller that wants to
// assert the exit code does so separately. The helper is called
// from tests that have already asserted the exit code (usually
// ExitSuccess).
func assertOnlyStdout(t *testing.T, got testRun) {
	t.Helper()
	if got.stdout == "" {
		t.Errorf("stdout is empty; want output on stdout\n"+
			"stderr: %q", got.stderr)
	}
	if got.stderr != "" {
		t.Errorf("stderr is non-empty; want nothing on stderr\n"+
			"stderr: %q", got.stderr)
	}
}

// assertOnlyStderr verifies that got has output on stderr and none
// on stdout.
//
// # Why the helper exists
//
// The output stream boundary (WBS 7.4.1) requires errors and
// diagnostics to go to stderr. A test that asserts the boundary
// for a failing command uses this helper: it asserts that stderr
// is non-empty and stdout is empty. The helper produces a
// specific failure message if either assertion fails.
//
// # What the helper does not assert
//
// The helper does not assert the content of stderr. Content is the
// job of the command's own tests (or the errors tests). The helper
// asserts only the stream: output on stderr, nothing on stdout.
//
// # Exit code
//
// The helper does not assert the exit code. A caller that wants to
// assert the exit code does so separately. The helper is called
// from tests that have already asserted the exit code (usually
// ExitUsage).
func assertOnlyStderr(t *testing.T, got testRun) {
	t.Helper()
	if got.stderr == "" {
		t.Errorf("stderr is empty; want a diagnostic on stderr\n"+
			"stdout: %q", got.stdout)
	}
	if got.stdout != "" {
		t.Errorf("stdout is non-empty; want nothing on stdout\n"+
			"stdout: %q", got.stdout)
	}
}

// =============================================================================
// Test-only registry hooks
// =============================================================================
//
// The hook below installs a temporary subcommand into the registry
// for the duration of a test. It exists so that the boundary's
// non-*forgeerr.Error wrapping path can be exercised in-process,
// without spawning a subprocess.
//
// # Why the hook is in testhelper_test.go
//
// The hook is a test-only fixture. The _test.go suffix ensures it
// is compiled only into the test binary; the production binary
// does not contain it. Placing it in the shared helper file
// (rather than in the execute_test.go file that calls it) makes it
// available to any future test file that needs the same seam.
//
// # Why the hook is serial-only
//
// The registry is a package-level slice. The hook appends to it
// and its restore function truncates it. A parallel test that
// reads the registry during the hook's lifetime would race with
// the mutation. The hook is therefore documented as serial-only,
// and the tests that call it are not marked t.Parallel.
//
// # Why the hook does not take a mutex
//
// The registry is not protected by a mutex in production. Adding
// one would impose a cost on every command construction for the
// benefit of a test-only concern. The serial-only convention is
// the simpler alternative: the tests that mutate the registry
// run in sequence, and the tests that read it run in parallel
// only when the hook is not active.
//
// # The panicking hook was removed in WBS 10.5.1
//
// A sibling hook, testOnlyRegisterPanickingCommand, existed to
// exercise the boundary's panic recovery. It was removed when the
// recovery's test was rewritten to test recoveredPanicError
// directly. The recovery lives in Execute, which cannot be
// exercised through runCLI; the conversion is the testable unit,
// and it does not need a registry hook.

// testOnlyRegisterPlainErrorCommand appends a subcommand that
// returns a plain error to the registry for the duration of one
// test.
//
// The function returns a restore function that removes the
// subcommand. The test must defer the restore so that a failed
// assertion does not leave the registry in a mutated state.
//
// The subcommand is used by TestExecute_NonForgeErrorWrapped to
// verify that the boundary wraps a non-*forgeerr.Error error as
// FORGE_INTERNAL_UNEXPECTED.
//
// The subcommand is not added to the visible list in the help
// output: it has no Short field, and its Use field is prefixed
// with "__test_" so that it is recognisable as a test fixture if
// it leaks into any golden file. The tests that use the hook do
// not run the help path.
func testOnlyRegisterPlainErrorCommand(err error) func() {
	registry = append(registry, func(Dependencies) *cobra.Command {
		return &cobra.Command{
			Use: "__test_plain_error__",
			RunE: func(*cobra.Command, []string) error {
				return err
			},
		}
	})
	return func() {
		registry = registry[:len(registry)-1]
	}
}
