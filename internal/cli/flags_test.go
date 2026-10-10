// Package cli contains white-box tests for the CLI package.
//
// This file tests the helpers defined in flags.go. The helpers read
// the parsed values of the three global flags and resolve their
// combined effect. The tests exercise the helpers in isolation,
// without constructing the root command, by building a minimal
// *cobra.Command and registering the flags on it.
//
// # Relationship to root_test.go
//
// root_test.go contains the contract tests for the global flags'
// observable behaviour (persistence, precedence, inventory size).
// The tests in this file cover the helpers themselves: the readers
// (verboseRequested, quietRequested, configPath) and the resolver
// (resolveLogLevel). A failure in this file localizes the defect
// to the helper; a failure in root_test.go might indicate a defect
// in the helper, in registerGlobalFlags, or in the root command's
// wiring.
package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

// newFlagTestCmd returns a *cobra.Command with the global flags
// registered and no subcommands. The command is used by the tests
// below to exercise the flag helpers in isolation.
func newFlagTestCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	registerGlobalFlags(cmd)
	return cmd
}

// setFlags parses the given args on the command, so that the flag
// values are visible to the readers. The helper uses Cobra's own
// parser; the values the readers see are the values Cobra produced.
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
		{"--verbose", []string{"--verbose"}, true},
		{"no-flags", []string{}, false},
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
		{"--quiet", []string{"--quiet"}, true},
		{"no-flags", []string{}, false},
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
// resolveLogLevel
// =============================================================================

// TestResolveLogLevel verifies the precedence rule for the log
// level. The rule is documented in docs/cli-ux-spec.md § 4.12.
//
// The four combinations are enumerated: neither flag, only
// --verbose, only --quiet, and both. The last is the precedence
// case: --quiet wins.
func TestResolveLogLevel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		want string
	}{
		{"neither", nil, "info"},
		{"--verbose", []string{"--verbose"}, "debug"},
		{"--quiet", []string{"--quiet"}, "error"},
		{"both", []string{"--verbose", "--quiet"}, "error"},
		{"both-reversed", []string{"--quiet", "--verbose"}, "error"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cmd := newFlagTestCmd()
			setFlags(t, cmd, tc.args...)
			if got := resolveLogLevel(cmd); got != tc.want {
				t.Errorf("resolveLogLevel(%v) = %q; want %q",
					tc.args, got, tc.want)
			}
		})
	}
}
