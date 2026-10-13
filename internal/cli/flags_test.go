// Package cli contains white-box tests for the CLI package.
//
// This file tests the helpers defined in flags.go. The helpers read
// the parsed values of the five global flags and resolve their
// combined effect. The tests exercise the helpers in isolation,
// without constructing the root command, by building a minimal
// *cobra.Command and registering the flags on it.
//
// # Relationship to root_test.go
//
// root_test.go contains the contract tests for the global flags'
// observable behaviour (persistence, precedence, inventory size).
// The tests in this file cover the helpers themselves: the readers
// (verboseRequested, quietRequested, logLevelRequested, configPath,
// formatValue, formatRequested) and the resolver (resolveLogLevel).
// A failure in this file localizes the defect to the helper; a
// failure in root_test.go might indicate a defect in the helper, in
// registerGlobalFlags, or in the root command's wiring.
//
// # The log-level contract (WBS 12.1.2)
//
// The log level is resolved by a six-source precedence chain
// (highest first):
//
//  1. --log-level=<level>   (explicit; wins over implicating flags)
//  2. --verbose             (implies DEBUG)
//  3. --quiet               (implies ERROR)
//  4. FORGE_LOG_LEVEL       (env var)
//  5. log.level             (config file)
//  6. default               (INFO)
//
// --quiet and --verbose are mutually exclusive: passing both is a
// usage error (ErrQuietVerboseConflict, exit 64), not a precedence
// tie. The tests below assert the conflict, the chain, and the
// mapping of each flag to its implied level.
//
// The reference is docs/reference/log-levels.md.
//
// # Test discipline: registering the global flags
//
// Every test that calls resolveLogLevel must pass a command built
// by newFlagTestCmd, which registers the global flags. A bare
// &cobra.Command{} lacks the flags and would trip the readers.
// The tolerant readers (quietRequested, verboseRequested,
// logLevelRequested) return their zero value when the flag is
// absent, but the cleaner discipline is to register the flags
// once and rely on the readers' normal path. The regression tests
// at the bottom of this file pin the tolerance.
//
// # t.Parallel discipline
//
// The reader tests and the precedence tests can run in parallel:
// each builds its own command, and there is no shared state. The
// env-var tests cannot: t.Setenv mutates process-global state and
// forbids parallel execution. The absence of t.Parallel on those
// tests is intentional and is commented at the call site.
package cli

import (
	"errors"
	"testing"

	"github.com/spf13/cobra"

	"github.com/thapelomagqazana/forge/internal/logger"
)

// newFlagTestCmd returns a *cobra.Command with the global flags
// registered and no subcommands. The command is used by the tests
// below to exercise the flag helpers in isolation.
//
// The helper is the canonical way to obtain a test command for this
// file. Every test that reaches resolveLogLevel must use it; a bare
// &cobra.Command{} lacks the flags and would trip the readers on
// their normal (panicking) path if any of them were to panic. See
// the package docstring for the discipline.
func newFlagTestCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	registerGlobalFlags(cmd)
	return cmd
}

// setFlags parses the given args on the command, so that the flag
// values are visible to the readers. The helper uses Cobra's own
// parser; the values the readers see are the values Cobra produced.
//
// Calling setFlags with no args is valid and leaves the flag values
// at their registered defaults. It is the explicit "parse nothing"
// call used by tests that want to make the intent visible.
func setFlags(t *testing.T, cmd *cobra.Command, args ...string) {
	t.Helper()
	if err := cmd.ParseFlags(args); err != nil {
		t.Fatalf("ParseFlags(%v): %v", args, err)
	}
}

// =============================================================================
// verboseRequested
// =============================================================================

// TestVerboseRequested verifies the reader for the --verbose flag.
func TestVerboseRequested(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"default", nil, false},
		{"no-flags", []string{}, false},
		{"--verbose", []string{"--verbose"}, true},
		{"--verbose=false", []string{"--verbose=false"}, false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cmd := newFlagTestCmd()
			setFlags(t, cmd, tc.args...)
			if got := verboseRequested(cmd); got != tc.want {
				t.Errorf("verboseRequested() = %v; want %v", got, tc.want)
			}
		})
	}
}

// =============================================================================
// quietRequested
// =============================================================================

// TestQuietRequested verifies the reader for the --quiet flag.
func TestQuietRequested(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"default", nil, false},
		{"no-flags", []string{}, false},
		{"--quiet", []string{"--quiet"}, true},
		{"--quiet=false", []string{"--quiet=false"}, false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cmd := newFlagTestCmd()
			setFlags(t, cmd, tc.args...)
			if got := quietRequested(cmd); got != tc.want {
				t.Errorf("quietRequested() = %v; want %v", got, tc.want)
			}
		})
	}
}

// =============================================================================
// logLevelRequested (WBS 12.1.2)
// =============================================================================

// TestLogLevelRequested verifies the reader for the --log-level flag.
func TestLogLevelRequested(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		want string
	}{
		{"default is empty", nil, ""},
		{"no-flags", []string{}, ""},
		{"--log-level warn", []string{"--log-level", "warn"}, "warn"},
		{"--log-level=debug", []string{"--log-level=debug"}, "debug"},
		{"empty value", []string{"--log-level", ""}, ""},
		{"unknown value preserved", []string{"--log-level", "trace"}, "trace"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cmd := newFlagTestCmd()
			setFlags(t, cmd, tc.args...)
			if got := logLevelRequested(cmd); got != tc.want {
				t.Errorf("logLevelRequested() = %q; want %q", got, tc.want)
			}
		})
	}
}

// =============================================================================
// configPath
// =============================================================================

// TestConfigPath verifies the reader for the --config flag.
func TestConfigPath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		want string
	}{
		{"default", nil, ""},
		{"--config path", []string{"--config", "/tmp/forge.yaml"}, "/tmp/forge.yaml"},
		{"--config=path", []string{"--config=/tmp/forge.yaml"}, "/tmp/forge.yaml"},
		{"empty-value", []string{"--config", ""}, ""},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cmd := newFlagTestCmd()
			setFlags(t, cmd, tc.args...)
			if got := configPath(cmd); got != tc.want {
				t.Errorf("configPath() = %q; want %q", got, tc.want)
			}
		})
	}
}

// =============================================================================
// formatValue and formatRequested
// =============================================================================

// TestFormatValue verifies the string reader for the --format flag.
func TestFormatValue(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		want string
	}{
		{"default", nil, FlagFormatText},
		{"explicit text", []string{"--format", "text"}, FlagFormatText},
		{"explicit json", []string{"--format", "json"}, FlagFormatJSON},
		{"empty value falls back to text", []string{"--format", ""}, FlagFormatText},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cmd := newFlagTestCmd()
			setFlags(t, cmd, tc.args...)
			if got := formatValue(cmd); got != tc.want {
				t.Errorf("formatValue() = %q; want %q", got, tc.want)
			}
		})
	}
}

// TestFormatRequested verifies the typed reader for the --format
// flag. The reader returns an OutputFormat (not a string), for the
// error path's dispatch.
func TestFormatRequested(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		want OutputFormat
	}{
		{"default", nil, FormatText},
		{"explicit text", []string{"--format", "text"}, FormatText},
		{"explicit json", []string{"--format", "json"}, FormatJSON},
		{"empty value falls back to text", []string{"--format", ""}, FormatText},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cmd := newFlagTestCmd()
			setFlags(t, cmd, tc.args...)
			if got := formatRequested(cmd); got != tc.want {
				t.Errorf("formatRequested() = %v; want %v", got, tc.want)
			}
		})
	}
}

// =============================================================================
// Mutual exclusion: --quiet vs --verbose (WBS 12.1.2)
// =============================================================================

// TestValidateLogFlags_QuietVerboseConflict asserts that passing both
// --quiet and --verbose is a usage error, regardless of order.
//
// Pre-WBS-12.1.2 the rule was "--quiet wins". WBS 12.1.2 makes the
// pair mutually exclusive: the two flags express opposite intents,
// and silently picking one hides a user error. See
// docs/reference/log-levels.md §4.5.
func TestValidateLogFlags_QuietVerboseConflict(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
	}{
		{"quiet then verbose", []string{"--quiet", "--verbose"}},
		{"verbose then quiet", []string{"--verbose", "--quiet"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cmd := newFlagTestCmd()
			setFlags(t, cmd, tc.args...)
			err := validateLogFlags(cmd)
			if !errors.Is(err, ErrQuietVerboseConflict) {
				t.Fatalf("validateLogFlags(%v) = %v; want ErrQuietVerboseConflict",
					tc.args, err)
			}
		})
	}
}

// TestValidateLogFlags_EachFlagAloneIsOK asserts that neither flag
// alone triggers the conflict.
func TestValidateLogFlags_EachFlagAloneIsOK(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
	}{
		{"neither", nil},
		{"no-flags", []string{}},
		{"only quiet", []string{"--quiet"}},
		{"only verbose", []string{"--verbose"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cmd := newFlagTestCmd()
			setFlags(t, cmd, tc.args...)
			if err := validateLogFlags(cmd); err != nil {
				t.Fatalf("validateLogFlags(%v) = %v; want nil", tc.args, err)
			}
		})
	}
}

// =============================================================================
// resolveLogLevel: flag-to-level mapping
// =============================================================================

// TestResolveLogLevel verifies the flag-to-level mapping of the
// resolver. The chain (flag > env > config > default) is tested
// separately; this test isolates the flag contributions.
//
// The mutual-exclusion case is tested by
// TestResolveLogLevel_ConflictIsUsageError, not here.
func TestResolveLogLevel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		args    []string
		wantLvl logger.Level
		wantSrc logger.Source
	}{
		{
			name:    "neither flag -> INFO from default",
			args:    nil,
			wantLvl: logger.LevelInfo,
			wantSrc: logger.SourceDefault,
		},
		{
			name:    "--verbose -> DEBUG",
			args:    []string{"--verbose"},
			wantLvl: logger.LevelDebug,
			wantSrc: logger.SourceFlagVerbose,
		},
		{
			name:    "--quiet -> ERROR",
			args:    []string{"--quiet"},
			wantLvl: logger.LevelError,
			wantSrc: logger.SourceFlagQuiet,
		},
		{
			name:    "--log-level=warn -> WARN",
			args:    []string{"--log-level", "warn"},
			wantLvl: logger.LevelWarn,
			wantSrc: logger.SourceFlagExplicit,
		},
		{
			name:    "--log-level=debug -> DEBUG",
			args:    []string{"--log-level", "debug"},
			wantLvl: logger.LevelDebug,
			wantSrc: logger.SourceFlagExplicit,
		},
		{
			name:    "--log-level=error -> ERROR",
			args:    []string{"--log-level", "error"},
			wantLvl: logger.LevelError,
			wantSrc: logger.SourceFlagExplicit,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cmd := newFlagTestCmd()
			setFlags(t, cmd, tc.args...)

			got, err := resolveLogLevel(cmd, "")
			if err != nil {
				t.Fatalf("resolveLogLevel: %v", err)
			}
			if got.Level != tc.wantLvl {
				t.Errorf("Level = %v; want %v", got.Level, tc.wantLvl)
			}
			if got.Source != tc.wantSrc {
				t.Errorf("Source = %v; want %v", got.Source, tc.wantSrc)
			}
		})
	}
}

// TestResolveLogLevel_ConflictIsUsageError asserts that the resolver
// surfaces ErrQuietVerboseConflict. It runs before any precedence
// resolution, so even a valid --log-level does not rescue the call.
func TestResolveLogLevel_ConflictIsUsageError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
	}{
		{"both, no level", []string{"--quiet", "--verbose"}},
		{"both, plus level", []string{"--quiet", "--verbose", "--log-level=warn"}},
		{"both, reversed, plus level", []string{"--log-level=debug", "--verbose", "--quiet"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cmd := newFlagTestCmd()
			setFlags(t, cmd, tc.args...)

			_, err := resolveLogLevel(cmd, "")
			if !errors.Is(err, ErrQuietVerboseConflict) {
				t.Fatalf("resolveLogLevel(%v) error = %v; want ErrQuietVerboseConflict",
					tc.args, err)
			}
		})
	}
}

// =============================================================================
// resolveLogLevel: precedence chain (flag > config > default)
// =============================================================================

// TestResolveLogLevel_Precedence verifies the flag-vs-flag and
// flag-vs-config precedence rules. The env tier is tested in the
// env-specific tests below, which cannot run in parallel.
//
// The cases are named after the two sources involved, in the
// direction of the rule being tested (e.g. "flag beats config"
// means the flag is higher precedence).
func TestResolveLogLevel_Precedence(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        []string
		configLevel string
		wantLvl     logger.Level
		wantSrc     logger.Source
	}{
		{
			name:        "explicit flag beats verbose",
			args:        []string{"--verbose", "--log-level=error"},
			configLevel: "",
			wantLvl:     logger.LevelError,
			wantSrc:     logger.SourceFlagExplicit,
		},
		{
			name:        "explicit flag beats quiet",
			args:        []string{"--quiet", "--log-level=debug"},
			configLevel: "",
			wantLvl:     logger.LevelDebug,
			wantSrc:     logger.SourceFlagExplicit,
		},
		{
			name:        "verbose beats config",
			args:        []string{"--verbose"},
			configLevel: "error",
			wantLvl:     logger.LevelDebug,
			wantSrc:     logger.SourceFlagVerbose,
		},
		{
			name:        "quiet beats config",
			args:        []string{"--quiet"},
			configLevel: "debug",
			wantLvl:     logger.LevelError,
			wantSrc:     logger.SourceFlagQuiet,
		},
		{
			name:        "config beats default",
			args:        nil,
			configLevel: "debug",
			wantLvl:     logger.LevelDebug,
			wantSrc:     logger.SourceConfig,
		},
		{
			name:        "config warning alias",
			args:        nil,
			configLevel: "warning",
			wantLvl:     logger.LevelWarn,
			wantSrc:     logger.SourceConfig,
		},
		{
			name:        "config uppercase",
			args:        nil,
			configLevel: "ERROR",
			wantLvl:     logger.LevelError,
			wantSrc:     logger.SourceConfig,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cmd := newFlagTestCmd()
			setFlags(t, cmd, tc.args...)

			got, err := resolveLogLevel(cmd, tc.configLevel)
			if err != nil {
				t.Fatalf("resolveLogLevel: %v", err)
			}
			if got.Level != tc.wantLvl {
				t.Errorf("Level = %v; want %v", got.Level, tc.wantLvl)
			}
			if got.Source != tc.wantSrc {
				t.Errorf("Source = %v; want %v", got.Source, tc.wantSrc)
			}
		})
	}
}

// =============================================================================
// resolveLogLevel: env var precedence (WBS 12.1.2)
// =============================================================================
//
// The tests in this section use t.Setenv, which mutates
// process-global state and forbids t.Parallel. The absence of
// t.Parallel here is intentional, not an oversight. See the package
// docstring for the t.Parallel discipline.

// TestResolveLogLevel_EnvBeatsConfig asserts FORGE_LOG_LEVEL wins
// over the config file. This is the Twelve-Factor §III rule and the
// headline precedence case in docs/reference/log-levels.md §2.3.
func TestResolveLogLevel_EnvBeatsConfig(t *testing.T) {
	t.Setenv(logger.EnvVar, "warn")

	cmd := newFlagTestCmd()
	setFlags(t, cmd) // no flags; the flags are registered by the helper

	got, err := resolveLogLevel(cmd, "debug")
	if err != nil {
		t.Fatalf("resolveLogLevel: %v", err)
	}
	if got.Level != logger.LevelWarn {
		t.Errorf("Level = %v; want WARN", got.Level)
	}
	if got.Source != logger.SourceEnv {
		t.Errorf("Source = %v; want env", got.Source)
	}
}

// TestResolveLogLevel_FlagBeatsEnv asserts --log-level wins over
// FORGE_LOG_LEVEL.
func TestResolveLogLevel_FlagBeatsEnv(t *testing.T) {
	t.Setenv(logger.EnvVar, "debug")

	cmd := newFlagTestCmd()
	setFlags(t, cmd, "--log-level=error")

	got, err := resolveLogLevel(cmd, "")
	if err != nil {
		t.Fatalf("resolveLogLevel: %v", err)
	}
	if got.Level != logger.LevelError {
		t.Errorf("Level = %v; want ERROR", got.Level)
	}
	if got.Source != logger.SourceFlagExplicit {
		t.Errorf("Source = %v; want flag(--log-level)", got.Source)
	}
}

// TestResolveLogLevel_QuietBeatsEnv asserts --quiet (an implicating
// flag) wins over FORGE_LOG_LEVEL (an env var). The flag tier is
// above the env tier in the chain, so any flag wins over the env.
func TestResolveLogLevel_QuietBeatsEnv(t *testing.T) {
	t.Setenv(logger.EnvVar, "debug")

	cmd := newFlagTestCmd()
	setFlags(t, cmd, "--quiet")

	got, err := resolveLogLevel(cmd, "")
	if err != nil {
		t.Fatalf("resolveLogLevel: %v", err)
	}
	if got.Level != logger.LevelError {
		t.Errorf("Level = %v; want ERROR", got.Level)
	}
	if got.Source != logger.SourceFlagQuiet {
		t.Errorf("Source = %v; want flag(--quiet)", got.Source)
	}
}

// TestResolveLogLevel_VerboseBeatsEnv asserts --verbose wins over
// FORGE_LOG_LEVEL, symmetrically with the quiet case.
func TestResolveLogLevel_VerboseBeatsEnv(t *testing.T) {
	t.Setenv(logger.EnvVar, "error")

	cmd := newFlagTestCmd()
	setFlags(t, cmd, "--verbose")

	got, err := resolveLogLevel(cmd, "")
	if err != nil {
		t.Fatalf("resolveLogLevel: %v", err)
	}
	if got.Level != logger.LevelDebug {
		t.Errorf("Level = %v; want DEBUG", got.Level)
	}
	if got.Source != logger.SourceFlagVerbose {
		t.Errorf("Source = %v; want flag(--verbose)", got.Source)
	}
}

// TestResolveLogLevel_EmptyEnvFallsThrough asserts an empty
// FORGE_LOG_LEVEL is treated as "unset" and the chain falls through
// to the config file.
func TestResolveLogLevel_EmptyEnvFallsThrough(t *testing.T) {
	t.Setenv(logger.EnvVar, "")

	cmd := newFlagTestCmd()
	setFlags(t, cmd)

	got, err := resolveLogLevel(cmd, "debug")
	if err != nil {
		t.Fatalf("resolveLogLevel: %v", err)
	}
	if got.Level != logger.LevelDebug {
		t.Errorf("Level = %v; want DEBUG", got.Level)
	}
	if got.Source != logger.SourceConfig {
		t.Errorf("Source = %v; want config", got.Source)
	}
}

// TestResolveLogLevel_InvalidEnvValue asserts an invalid
// FORGE_LOG_LEVEL surfaces as an error and does not fall through.
func TestResolveLogLevel_InvalidEnvValue(t *testing.T) {
	t.Setenv(logger.EnvVar, "loud")

	cmd := newFlagTestCmd()
	setFlags(t, cmd)

	if _, err := resolveLogLevel(cmd, "debug"); err == nil {
		t.Fatal("expected error for invalid FORGE_LOG_LEVEL")
	}
}

// =============================================================================
// resolveLogLevel: negative — invalid values
// =============================================================================

// TestResolveLogLevel_InvalidFlagValue asserts an invalid
// --log-level surfaces as an error.
func TestResolveLogLevel_InvalidFlagValue(t *testing.T) {
	t.Parallel()

	cmd := newFlagTestCmd()
	setFlags(t, cmd, "--log-level=trace")

	if _, err := resolveLogLevel(cmd, ""); err == nil {
		t.Fatal("expected error for invalid --log-level")
	}
}

// TestResolveLogLevel_InvalidFlagDoesNotFallThrough asserts that an
// invalid higher-precedence source does not silently fall through to
// a valid lower one. This is the "no silent fallback" rule from
// docs/reference/log-levels.md §2.
func TestResolveLogLevel_InvalidFlagDoesNotFallThrough(t *testing.T) {
	t.Parallel()

	cmd := newFlagTestCmd()
	setFlags(t, cmd, "--log-level=trace")

	if _, err := resolveLogLevel(cmd, "debug"); err == nil {
		t.Fatal("invalid flag must error even when config is valid")
	}
}

// TestResolveLogLevel_InvalidConfigValue asserts an invalid config
// value surfaces as an error when it is the winning source.
//
// The test uses newFlagTestCmd, which registers the global flags.
// A bare &cobra.Command{} lacks them, and verboseRequested panics on
// an unregistered flag. See the package docstring for the test
// discipline.
func TestResolveLogLevel_InvalidConfigValue(t *testing.T) {
	t.Parallel()

	cmd := newFlagTestCmd()
	if _, err := resolveLogLevel(cmd, "silent"); err == nil {
		t.Fatal("expected error for invalid config log.level")
	}
}

// =============================================================================
// Corner: alias and case for --log-level
// =============================================================================

// TestResolveLogLevel_LogLevelAliasAndCase asserts that --log-level
// accepts the canonical names case-insensitively and the "warning"
// alias, matching ParseLevel's contract.
func TestResolveLogLevel_LogLevelAliasAndCase(t *testing.T) {
	t.Parallel()

	cases := []struct {
		value string
		want  logger.Level
	}{
		{"debug", logger.LevelDebug},
		{"DEBUG", logger.LevelDebug},
		{"info", logger.LevelInfo},
		{"Info", logger.LevelInfo},
		{"warn", logger.LevelWarn},
		{"WARN", logger.LevelWarn},
		{"warning", logger.LevelWarn},
		{"WARNING", logger.LevelWarn},
		{"error", logger.LevelError},
		{"Error", logger.LevelError},
		{"  warn  ", logger.LevelWarn},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.value, func(t *testing.T) {
			t.Parallel()
			cmd := newFlagTestCmd()
			setFlags(t, cmd, "--log-level", tc.value)

			got, err := resolveLogLevel(cmd, "")
			if err != nil {
				t.Fatalf("resolveLogLevel(--log-level=%q): %v", tc.value, err)
			}
			if got.Level != tc.want {
				t.Errorf("Level = %v; want %v", got.Level, tc.want)
			}
		})
	}
}

// =============================================================================
// Regression tests: tolerant readers (WBS 12.1.2)
// =============================================================================

// TestVerboseRequested_UnregisteredFlagReturnsFalse pins the
// tolerant behaviour: a command that has not had its global flags
// registered reads --verbose as false rather than panicking.
//
// Pre-WBS-12.1.2, verboseRequested panicked on an unregistered
// flag, which made it impossible to test the resolver with an
// inline command. The tolerance matches quietRequested's and is
// the correct default for a boolean flag. See the package
// docstring for the test discipline this tolerance supports.
func TestVerboseRequested_UnregisteredFlagReturnsFalse(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{Use: "test"} // no registerGlobalFlags
	if got := verboseRequested(cmd); got != false {
		t.Fatalf("verboseRequested() = %v; want false", got)
	}
}

// TestQuietRequested_UnregisteredFlagReturnsFalse pins the
// tolerant behaviour for --quiet. It is the same contract as
// verboseRequested's, and is included so the two readers cannot
// drift apart.
func TestQuietRequested_UnregisteredFlagReturnsFalse(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{Use: "test"} // no registerGlobalFlags
	if got := quietRequested(cmd); got != false {
		t.Fatalf("quietRequested() = %v; want false", got)
	}
}

// TestLogLevelRequested_UnregisteredFlagReturnsEmpty pins the
// tolerant behaviour for --log-level.
func TestLogLevelRequested_UnregisteredFlagReturnsEmpty(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{Use: "test"} // no registerGlobalFlags
	if got := logLevelRequested(cmd); got != "" {
		t.Fatalf("logLevelRequested() = %q; want empty", got)
	}
}

// TestValidateLogFlags_UnregisteredFlagsNoConflict pins that the
// validator tolerates an unregistered command: no conflict because
// neither flag is set.
func TestValidateLogFlags_UnregisteredFlagsNoConflict(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{Use: "test"} // no registerGlobalFlags
	if err := validateLogFlags(cmd); err != nil {
		t.Fatalf("validateLogFlags() = %v; want nil", err)
	}
}

// =============================================================================
// Non-functional: validation is allocation-light
// =============================================================================

// TestValidateLogFlags_AllocsBound asserts that validateLogFlags
// does not allocate per-input. The function runs on every
// invocation; a future refactor that turned it into an allocating
// operation on the hot path would be caught here.
//
// # Not parallel
//
// testing.AllocsPerRun observes process-global allocation counters
// and panics if called from a parallel test. The counters cannot
// be measured safely while other tests are allocating concurrently.
// The absence of t.Parallel here is required by the testing package,
// not an oversight. Do not add it back.
//
// # Flag registration
//
// The command is built by newFlagTestCmd, which calls
// registerGlobalFlags. validateLogFlags reads --verbose and
// --quiet; a command without those flags registered panics in
// verboseRequested. The helper is the canonical way to obtain a
// test command in this package; see the package docstring.
//
// # The bound
//
// The bound is a canary, not a zero-allocation guarantee. The
// function runs once per process invocation, so a small constant
// number of allocations is acceptable. The bound guards against
// a regression that makes the function allocate proportionally to
// input size, iteration count, or map growth.
//
// The measured count is dominated by pflag's GetBool path
// (flag lookup, value stringification, and bool parsing), which
// allocates a small fixed amount per call. The bound is set well
// above the measured count so that a legitimate pflag version bump
// that changes the constant does not fail the test; a refactor
// that introduces unbounded allocation will still exceed it.
func TestValidateLogFlags_AllocsBound(t *testing.T) {
	cmd := newFlagTestCmd()
	allocs := testing.AllocsPerRun(1000, func() {
		_ = validateLogFlags(cmd)
	})
	if allocs > 10 {
		t.Fatalf("validateLogFlags allocated %.1f times; want <= 10",
			allocs)
	}
}

// =============================================================================
// The -v reservation (WBS 12.5.1)
// =============================================================================

// TestContract_VerboseHasNoShortForm pins the -v reservation
// decision: --verbose is long-only, because -v is the short form
// of --version (Cobra convention; see
// docs/reference/verbose-mode.md §2).
//
// # Why the test exists
//
// --verbose is the only log-level flag that a user is likely to
// want a short form for (--quiet has no short form either, but
// that is less commonly missed). A future contributor adding -v
// to --verbose would silently break the -v = --version
// convention that git, docker, and kubectl share. This test fails
// on that change, and the failure message names the decision.
//
// # How the test reads the flag
//
// The test inspects the flag's Shorthand field on the *flag
// definition*, not on a parsed command. The definition is the
// source of truth: if the Shorthand is empty, no parser version
// can produce a non-empty shorthand for the flag.
//
// # The companion test
//
// TestContract_VersionHasShortFormV (below) asserts the other
// half of the decision: --version's short form is -v. The two
// tests together pin the reservation: -v is not verbose, and -v
// is version.
func TestContract_VerboseHasNoShortForm(t *testing.T) {
	t.Parallel()

	cmd := newFlagTestCmd()
	f := cmd.PersistentFlags().Lookup(FlagVerbose)
	if f == nil {
		t.Fatalf("--%s is not registered", FlagVerbose)
	}
	if f.Shorthand != "" {
		t.Errorf("--%s has shorthand %q; want none. "+
			"-v is reserved for --version; see "+
			"docs/reference/verbose-mode.md §2",
			FlagVerbose, f.Shorthand)
	}
}

// TestContract_VersionHasShortFormV pins the other half of the
// -v reservation: --version's short form is -v.
//
// # Why the test constructs the root command directly
//
// --version is registered by Cobra automatically when the root
// command's Version field is non-empty (WBS 5.1.2). It is not
// part of registerGlobalFlags; a command built by newFlagTestCmd
// does not have it. The test therefore constructs the root
// command via newRootCmd, which is the same constructor the CLI
// uses at runtime.
//
// # The value of this test
//
// A future change to the root command that cleared root.Version
// would silently disable the --version flag and, with it, the -v
// shorthand. This test fails on that change, and the failure
// names the flag that went missing. Without this test, the loss
// would show up only as a user complaint that `forge -v` no
// longer works.
func TestContract_VersionHasShortFormV(t *testing.T) {
	t.Parallel()

	root := newRootCmd(testDependencies())

	// Cobra registers --version on the root command's *local*
	// flags (not PersistentFlags) when root.Version is
	// non-empty. Look it up on the local flag set.
	f := root.Flags().Lookup("version")
	if f == nil {
		t.Fatal("--version is not registered on the root command; " +
			"expected Cobra to add it because root.Version is " +
			"non-empty (WBS 5.1.2). " +
			"See docs/reference/verbose-mode.md §2.")
	}
	if f.Shorthand != "v" {
		t.Errorf("--version has shorthand %q; want %q. "+
			"The -v reservation requires -v to be --version's "+
			"short form; see docs/reference/verbose-mode.md §2.",
			f.Shorthand, "v")
	}
}

// TestContract_QuietHasShortFormQ pins the -q shorthand on the
// --quiet flag.
//
// # Why the shorthand is registered
//
// -q is unclaimed: --version owns -v, --help owns -h, and no
// other global flag claims -q. The shorthand matches the
// convention of git commit -q, apt-get -q, and similar tools. A
// future WBS that wants -q for a different flag must file an ADR;
// the test fails until the decision is deliberate.
//
// # How the test reads the flag
//
// The test inspects the flag's Shorthand field on the flag
// definition. The definition is the source of truth: if the
// Shorthand is "-q", no parser version can produce a different
// shorthand for the flag.
func TestContract_QuietHasShortFormQ(t *testing.T) {
	t.Parallel()

	cmd := newFlagTestCmd()
	f := cmd.PersistentFlags().Lookup(FlagQuiet)
	if f == nil {
		t.Fatalf("--%s is not registered", FlagQuiet)
	}
	if f.Shorthand != "q" {
		t.Errorf("--%s has shorthand %q; want %q. "+
			"See docs/reference/quiet-mode.md §1",
			FlagQuiet, f.Shorthand, "q")
	}
}

// TestQuietRequested_ShortFormQ asserts the shorthand parses to the
// same value as the long form.
func TestQuietRequested_ShortFormQ(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"long form", []string{"--quiet"}, true},
		{"short form", []string{"-q"}, true},
		{"both forms", []string{"--quiet", "-q"}, true},
		{"neither", nil, false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cmd := newFlagTestCmd()
			setFlags(t, cmd, tc.args...)
			if got := quietRequested(cmd); got != tc.want {
				t.Errorf("quietRequested(%v) = %v; want %v",
					tc.args, got, tc.want)
			}
		})
	}
}

// TestQuietMode_ResolvesToErrorLevel asserts that --quiet
// resolves to the ERROR level via the precedence chain.
//
// The test reads the resolved Decision, not a raw flag value. The
// Decision is the resolver's output; it is what the logger
// installation consumes.
func TestQuietMode_ResolvesToErrorLevel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
	}{
		{"long form", []string{"--quiet"}},
		{"short form", []string{"-q"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cmd := newFlagTestCmd()
			setFlags(t, cmd, tc.args...)

			d, err := resolveLogLevel(cmd, "")
			if err != nil {
				t.Fatalf("resolveLogLevel: %v", err)
			}
			if d.Level != logger.LevelError {
				t.Errorf("Level = %v; want %v",
					d.Level, logger.LevelError)
			}
			if d.Source != logger.SourceFlagQuiet {
				t.Errorf("Source = %v; want %v",
					d.Source, logger.SourceFlagQuiet)
			}
		})
	}
}

// TestQuietMode_LogLevelOverridesQuiet asserts the precedence rule:
// --log-level wins over --quiet.
func TestQuietMode_LogLevelOverridesQuiet(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		level string
		want  logger.Level
	}{
		{"debug", "debug", logger.LevelDebug},
		{"info", "info", logger.LevelInfo},
		{"warn", "warn", logger.LevelWarn},
		{"error", "error", logger.LevelError},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cmd := newFlagTestCmd()
			setFlags(t, cmd, "--quiet", "--log-level="+tc.level)

			d, err := resolveLogLevel(cmd, "")
			if err != nil {
				t.Fatalf("resolveLogLevel: %v", err)
			}
			if d.Level != tc.want {
				t.Errorf("Level = %v; want %v", d.Level, tc.want)
			}
			if d.Source != logger.SourceFlagExplicit {
				t.Errorf("Source = %v; want %v",
					d.Source, logger.SourceFlagExplicit)
			}
		})
	}
}

// TestQuietVerboseConflict_ShortForms asserts that -q and
// --verbose (and their combinations) are rejected as a usage error.
func TestQuietVerboseConflict_ShortForms(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
	}{
		{"-q --verbose", []string{"-q", "--verbose"}},
		{"--quiet --verbose", []string{"--quiet", "--verbose"}},
		{"--verbose -q", []string{"--verbose", "-q"}},
		{"--verbose --quiet", []string{"--verbose", "--quiet"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cmd := newFlagTestCmd()
			setFlags(t, cmd, tc.args...)

			_, err := resolveLogLevel(cmd, "")
			if !errors.Is(err, ErrQuietVerboseConflict) {
				t.Fatalf("resolveLogLevel(%v) error = %v; "+
					"want ErrQuietVerboseConflict",
					tc.args, err)
			}
		})
	}
}
