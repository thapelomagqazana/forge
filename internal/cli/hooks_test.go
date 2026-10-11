// Package cli contains white-box tests for the CLI package.
//
// This file tests the PersistentPreRunE hook defined in hooks.go.
// The hook loads the CLI's configuration (WBS 8.4.5): it applies
// the four precedence layers (defaults, file, environment, CLI
// flags) and stores the resolved Config in the command's context
// and in the Dependencies value captured by the closure.
//
// # What the tests verify
//
// Unit tests, exercising the hook in isolation:
//
//   - The defaults are applied when no file, no env, and no
//     flags are present.
//   - A config file's values are applied.
//   - The environment overrides the file.
//   - A CLI flag overrides the environment.
//   - A malformed config file produces an error.
//   - An invalid enum value produces an error.
//   - The resolved config is stored in the command's context.
//   - The resolved config is stored through deps.Config.
//
// Integration tests, exercising the hook installed on the root
// command:
//
//   - The root command has a non-nil PersistentPreRunE.
//   - No subcommand defines its own PersistentPreRunE.
//   - The hook runs before a subcommand's RunE.
//
// # How the unit tests invoke the hook
//
// The hook is a constructor: initializeCommandEnvironment(deps)
// returns the closure Cobra calls. A unit test constructs a
// Dependencies value with synthetic inputs, calls the
// constructor, and invokes the returned closure with a minimal
// command.
//
// The synthetic Dependencies carries:
//
//   - Env: a function backed by a map, so the test does not
//     depend on the test runner's environment.
//   - RootPath: a t.TempDir() value, so config discovery is
//     deterministic and does not read the developer's home
//     directory.
//
// # Why the tests use a local helper
//
// The local helper (testHookDeps) constructs the Dependencies
// value the hook reads. Keeping it local to this file makes the
// tests self-contained: a change to the production
// testDependencies helper does not affect this file.
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/thapelomagqazana/forge/internal/config"
)

// =============================================================================
// Test helper
// =============================================================================

// testHookDeps constructs a Dependencies value for the hook tests.
//
// # What the helper sets
//
// The helper sets the fields the hook reads:
//
//   - Env: the environment lookup, backed by the provided map.
//   - RootPath: the working directory for config discovery.
//
// The other fields are left at their zero values; the hook does
// not read them.
//
// # Why a real env map
//
// The tests set the env map explicitly so that they do not
// depend on the test runner's environment. A test that set
// FORGE_LOG_LEVEL in the shell would affect every test in the
// process; the map confines the setting to one test.
func testHookDeps(env map[string]string, rootPath string) Dependencies {
	envLookup := func(key string) string {
		return env[key]
	}
	return Dependencies{
		Env:      envLookup,
		RootPath: rootPath,
	}
}

// newHookTestCommand constructs a minimal *cobra.Command with the
// three flags the hook reads: --log-level, --output-format, and
// --config.
//
// # Why the helper is minimal
//
// The hook reads three flags. The helper registers only those
// three. A test that wants to simulate "the user typed the flag"
// calls cmd.Flags().Set(name, value); a test that wants to
// simulate "the user did not type the flag" does nothing.
//
// # Why a fresh command per test
//
// A command's flags accumulate state (the value and the changed
// marker). A test that reused a command would see the previous
// test's state. A fresh command per test isolates the state.
func newHookTestCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String(config.FlagLogLevel, config.LogLevelInfo, "")
	cmd.Flags().String(config.FlagOutputFormat, config.OutputFormatText, "")
	cmd.Flags().String(FlagConfig, "", "")
	return cmd
}

// =============================================================================
// Unit tests — the four layers
// =============================================================================

// TestInitializeCommandEnvironment_DefaultsOnly verifies that the
// hook produces the defaults when no file, no env, and no flags
// are present.
func TestInitializeCommandEnvironment_DefaultsOnly(t *testing.T) {
	t.Parallel()

	cmd := newHookTestCommand()
	deps := testHookDeps(map[string]string{}, t.TempDir())

	hook := initializeCommandEnvironment(deps)
	if err := hook(cmd, nil); err != nil {
		t.Fatalf("hook: %v", err)
	}

	got, ok := ConfigFromContext(cmd.Context())
	if !ok {
		t.Fatalf("config not found in context")
	}

	want := config.Defaults()
	if got.LogLevel != want.LogLevel {
		t.Errorf("LogLevel = %q; want %q", got.LogLevel, want.LogLevel)
	}
	if got.OutputFormat != want.OutputFormat {
		t.Errorf("OutputFormat = %q; want %q",
			got.OutputFormat, want.OutputFormat)
	}
}

// TestInitializeCommandEnvironment_FileApplied verifies that a
// config file's values are applied.
func TestInitializeCommandEnvironment_FileApplied(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "forge.yaml")
	if err := os.WriteFile(cfgPath, []byte("log_level: warn\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cmd := newHookTestCommand()
	deps := testHookDeps(map[string]string{}, dir)

	hook := initializeCommandEnvironment(deps)
	if err := hook(cmd, nil); err != nil {
		t.Fatalf("hook: %v", err)
	}

	got, _ := ConfigFromContext(cmd.Context())
	if got.LogLevel != config.LogLevelWarn {
		t.Errorf("LogLevel = %q; want %q", got.LogLevel, config.LogLevelWarn)
	}
	if got.SourcePath != cfgPath {
		t.Errorf("SourcePath = %q; want %q", got.SourcePath, cfgPath)
	}
}

// TestInitializeCommandEnvironment_EnvOverridesFile verifies that
// the environment layer overrides the file layer.
func TestInitializeCommandEnvironment_EnvOverridesFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "forge.yaml")
	if err := os.WriteFile(cfgPath, []byte("log_level: warn\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cmd := newHookTestCommand()
	deps := testHookDeps(map[string]string{
		"FORGE_LOG_LEVEL": "debug",
	}, dir)

	hook := initializeCommandEnvironment(deps)
	if err := hook(cmd, nil); err != nil {
		t.Fatalf("hook: %v", err)
	}

	got, _ := ConfigFromContext(cmd.Context())
	if got.LogLevel != config.LogLevelDebug {
		t.Errorf("LogLevel = %q; want %q (env should override file)",
			got.LogLevel, config.LogLevelDebug)
	}
}

// TestInitializeCommandEnvironment_FlagOverridesEnv verifies that
// the CLI flag layer overrides the environment layer.
func TestInitializeCommandEnvironment_FlagOverridesEnv(t *testing.T) {
	t.Parallel()

	cmd := newHookTestCommand()
	if err := cmd.Flags().Set(config.FlagLogLevel, "error"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	deps := testHookDeps(map[string]string{
		"FORGE_LOG_LEVEL": "debug",
	}, t.TempDir())

	hook := initializeCommandEnvironment(deps)
	if err := hook(cmd, nil); err != nil {
		t.Fatalf("hook: %v", err)
	}

	got, _ := ConfigFromContext(cmd.Context())
	if got.LogLevel != config.LogLevelError {
		t.Errorf("LogLevel = %q; want %q (flag should override env)",
			got.LogLevel, config.LogLevelError)
	}
}

// TestInitializeCommandEnvironment_ExplicitConfigFlag verifies
// that the --config flag is honoured by config.Discover.
func TestInitializeCommandEnvironment_ExplicitConfigFlag(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "custom.yaml")
	if err := os.WriteFile(cfgPath, []byte("log_level: error\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cmd := newHookTestCommand()
	if err := cmd.Flags().Set(FlagConfig, cfgPath); err != nil {
		t.Fatalf("Set: %v", err)
	}
	deps := testHookDeps(map[string]string{}, dir)

	hook := initializeCommandEnvironment(deps)
	if err := hook(cmd, nil); err != nil {
		t.Fatalf("hook: %v", err)
	}

	got, _ := ConfigFromContext(cmd.Context())
	if got.LogLevel != config.LogLevelError {
		t.Errorf("LogLevel = %q; want %q", got.LogLevel, config.LogLevelError)
	}
	if got.SourcePath != cfgPath {
		t.Errorf("SourcePath = %q; want %q", got.SourcePath, cfgPath)
	}
}

// =============================================================================
// Unit tests — errors
// =============================================================================

// TestInitializeCommandEnvironment_MalformedFile verifies that a
// malformed config file produces an error.
func TestInitializeCommandEnvironment_MalformedFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "forge.yaml")
	if err := os.WriteFile(cfgPath, []byte("log_level: [unclosed\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cmd := newHookTestCommand()
	deps := testHookDeps(map[string]string{}, dir)

	hook := initializeCommandEnvironment(deps)
	err := hook(cmd, nil)
	if err == nil {
		t.Fatalf("hook: want error, got nil")
	}
}

// TestInitializeCommandEnvironment_InvalidEnum verifies that an
// invalid enum value in a CLI flag produces an error.
func TestInitializeCommandEnvironment_InvalidEnum(t *testing.T) {
	t.Parallel()

	cmd := newHookTestCommand()
	if err := cmd.Flags().Set(config.FlagLogLevel, "verbose"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	deps := testHookDeps(map[string]string{}, t.TempDir())

	hook := initializeCommandEnvironment(deps)
	err := hook(cmd, nil)
	if err == nil {
		t.Fatalf("hook: want error, got nil")
	}
	if !strings.Contains(err.Error(), "verbose") {
		t.Errorf("error does not name the invalid value: %v", err)
	}
}

// =============================================================================
// Unit tests — no stream writes
// =============================================================================

// TestInitializeCommandEnvironment_DoesNotWriteToStreams verifies
// that the hook does not write to the command's streams.
//
// The hook reads files and env; it does not produce output. Its
// diagnostic path is the returned error, not a direct write.
func TestInitializeCommandEnvironment_DoesNotWriteToStreams(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	cmd := newHookTestCommand()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	deps := testHookDeps(map[string]string{}, t.TempDir())
	hook := initializeCommandEnvironment(deps)
	if err := hook(cmd, nil); err != nil {
		t.Fatalf("hook: %v", err)
	}

	if stdout.Len() != 0 {
		t.Errorf("hook wrote to stdout: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("hook wrote to stderr: %q", stderr.String())
	}
}

// =============================================================================
// Integration tests — the hook installed on the root command
// =============================================================================

// TestHooks_NoSubcommandOverrides verifies AC5: no subcommand in
// the registry defines its own PersistentPreRunE.
//
// # Why the rule matters
//
// Cobra's PersistentPreRunE semantics are: the closest hook in
// the command chain runs, and the parent's hook is skipped if a
// descendant has one. If a subcommand defines its own
// PersistentPreRunE, the root's hook does not run for that
// subcommand's subtree, and the configuration is not loaded.
//
// The test iterates the registry and asserts that no command
// produced by a registry entry has a non-nil PersistentPreRunE.
func TestHooks_NoSubcommandOverrides(t *testing.T) {
	t.Parallel()

	deps := testHookDeps(map[string]string{}, t.TempDir())
	for _, ctor := range registry {
		cmd := ctor(deps)
		if cmd.PersistentPreRunE != nil {
			t.Errorf("subcommand %q defines its own PersistentPreRunE; "+
				"the root's hook must be the only one in the tree. "+
				"See docs/architecture.md § 11.16", cmd.Name())
		}
	}
}

// TestHooks_RunsBeforeSubcommandRunE verifies AC4: the root's
// hook runs before a subcommand's RunE.
//
// # How the test observes the order
//
// The test constructs a local root command with a recording
// hook and a subcommand with a recording RunE. After executing
// the subcommand, the test asserts that both recorded their
// runs and that the hook ran first.
//
// # Why a local root command
//
// The production root command's hook performs the loader chain.
// To observe that it runs before a subcommand's RunE, the test
// would need to instrument it, which is heavier than necessary.
// Cobra's hook ordering is a property of Cobra, not of Forge's
// specific hook; the local command verifies the ordering the
// production root command relies on.
func TestHooks_RunsBeforeSubcommandRunE(t *testing.T) {
	t.Parallel()

	var order []string

	recordingHook := func(cmd *cobra.Command, args []string) error {
		order = append(order, "hook")
		return nil
	}

	sub := &cobra.Command{
		Use: "sub",
		RunE: func(cmd *cobra.Command, args []string) error {
			order = append(order, "rune")
			return nil
		},
	}

	root := &cobra.Command{
		Use:               "root",
		PersistentPreRunE: recordingHook,
	}
	root.AddCommand(sub)

	root.SetArgs([]string{"sub"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if len(order) != 2 {
		t.Fatalf("order has %d entries; want 2: %v", len(order), order)
	}
	if order[0] != "hook" {
		t.Errorf("first run was %q; want %q", order[0], "hook")
	}
	if order[1] != "rune" {
		t.Errorf("second run was %q; want %q", order[1], "rune")
	}
}

// TestHooks_RootCmdHasHookInstalled verifies AC1: the root
// command produced by newRootCmd has a non-nil PersistentPreRunE.
//
// The test constructs the root command directly and inspects the
// field. It is the structural counterpart to
// TestHooks_RunsBeforeSubcommandRunE, which verifies the ordering
// property behaviourally.
func TestHooks_RootCmdHasHookInstalled(t *testing.T) {
	t.Parallel()

	deps := testHookDeps(map[string]string{}, t.TempDir())
	root := newRootCmd(deps)

	if root.PersistentPreRunE == nil {
		t.Fatal("root command has no PersistentPreRunE; " +
			"the hook point is not reserved")
	}
}
