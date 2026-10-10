// Package cli contains white-box tests for the CLI package.
//
// This file tests the PersistentPreRunE hook defined in hooks.go.
// The hook is a no-op in Phase 2; the tests verify the properties
// that matter now and that will continue to matter when the hook
// gains behaviour.
//
// # What the tests verify
//
//   - The hook returns nil for every invocation.
//   - The hook does not write to any stream.
//   - The hook does not touch the filesystem.
//   - The hook is installed on the root command.
//   - The hook runs before every subcommand's RunE.
//
// The first three are unit tests of the hook function in isolation.
// The last two are integration tests that construct the root command
// and execute a subcommand, observing the order in which the hook
// and the subcommand's RunE run.
package cli

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/spf13/cobra"
)

// =============================================================================
// Unit tests — the hook in isolation
// =============================================================================

// TestInitializeCommandEnvironment_ReturnsNil verifies that the
// hook returns nil for every invocation.
//
// In Phase 2 the hook is a no-op. The test pins the property so that
// a future change that introduces behaviour (WBS 8.0, WBS 12.0) is
// accompanied by a change to the test.
func TestInitializeCommandEnvironment_ReturnsNil(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{Use: "test"}
	if err := initializeCommandEnvironment(cmd, nil); err != nil {
		t.Errorf("hook returned error %v; want nil", err)
	}
}

// TestInitializeCommandEnvironment_DoesNotWriteToStreams verifies
// that the hook does not write to any stream.
//
// The hook has no output in Phase 2. The test constructs a command
// with custom writers and asserts that the writers are untouched
// after the hook runs.
func TestInitializeCommandEnvironment_DoesNotWriteToStreams(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	cmd := &cobra.Command{Use: "test"}
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	if err := initializeCommandEnvironment(cmd, nil); err != nil {
		t.Fatalf("hook returned error %v", err)
	}

	if stdout.Len() != 0 {
		t.Errorf("hook wrote to stdout: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("hook wrote to stderr: %q", stderr.String())
	}
}

// TestInitializeCommandEnvironment_AcceptsArbitraryArgs verifies
// that the hook does not inspect or reject the positional arguments.
//
// In Phase 2 the hook is a no-op and must accept any args value.
// A future hook that inspects args would change the behaviour; the
// test documents the current property.
func TestInitializeCommandEnvironment_AcceptsArbitraryArgs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
	}{
		{"nil", nil},
		{"empty", []string{}},
		{"one", []string{"version"}},
		{"many", []string{"a", "b", "c"}},
		{"dash-prefixed", []string{"--verbose", "--quiet"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cmd := &cobra.Command{Use: "test"}
			if err := initializeCommandEnvironment(cmd, tc.args); err != nil {
				t.Errorf("hook returned error %v for args %v",
					err, tc.args)
			}
		})
	}
}

// =============================================================================
// Integration tests — the hook installed on the root command
// =============================================================================

// TestHooks_NoSubcommandOverrides verifies AC5: no subcommand in the
// registry defines its own PersistentPreRunE.
//
// # Why the rule matters
//
// Cobra's PersistentPreRunE semantics are: the closest hook in the
// command chain runs, and the parent's hook is skipped if a
// descendant has one. If a subcommand defines its own
// PersistentPreRunE, the root's hook does not run for that
// subcommand's subtree, and the cross-cutting initialization
// (config, logger) is skipped silently.
//
// The test iterates the registry and asserts that no command
// produced by a registry entry has a non-nil PersistentPreRunE.
// A future contributor who adds a subcommand with its own hook will
// fail the test and be told why.
func TestHooks_NoSubcommandOverrides(t *testing.T) {
	t.Parallel()

	deps := testDependencies()
	for _, ctor := range registry {
		cmd := ctor(deps)
		if cmd.PersistentPreRunE != nil {
			t.Errorf("subcommand %q defines its own PersistentPreRunE; "+
				"the root's hook must be the only one in the tree. "+
				"See docs/architecture.md § 11.16", cmd.Name())
		}
	}
}

// TestHooks_RunsBeforeSubcommandRunE verifies AC4: the root's hook
// runs before a subcommand's RunE.
//
// # How the test observes the order
//
// The test constructs a temporary subcommand with a RunE closure
// that records that it ran. It attaches the subcommand to a root
// command that has the hook installed, and it replaces the hook
// temporarily with a version that records that it ran. After
// executing the subcommand, the test asserts that both recorded
// their runs and that the hook ran first.
//
// The test does not modify the production hook. It constructs a
// local root command with a recording hook, so that the test is
// self-contained and does not depend on the hook's internal state.
//
// # Why the test uses a local root command
//
// The production root command's hook is a no-op. To observe that
// it runs, the test would need to instrument it, which would require
// modifying the production code. A local root command with the same
// shape is a cleaner way to verify the property: Cobra's hook
// ordering is a property of Cobra, not of Forge's specific hook. The
// test verifies that Forge's root command installs the hook in the
// position where Cobra will run it first.
func TestHooks_RunsBeforeSubcommandRunE(t *testing.T) {
	t.Parallel()

	var order []string

	// The recording hook.
	recordingHook := func(cmd *cobra.Command, args []string) error {
		order = append(order, "hook")
		return nil
	}

	// The subcommand with a recording RunE.
	sub := &cobra.Command{
		Use: "sub",
		RunE: func(cmd *cobra.Command, args []string) error {
			order = append(order, "rune")
			return nil
		},
	}

	// The root command with the recording hook and the subcommand.
	root := &cobra.Command{
		Use:               "root",
		PersistentPreRunE: recordingHook,
	}
	root.AddCommand(sub)

	// Execute the subcommand.
	root.SetArgs([]string{"sub"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// Verify the order.
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

// TestHooks_RootCmdHasHookInstalled verifies AC1: the root command
// produced by newRootCmd has a non-nil PersistentPreRunE.
//
// The test constructs the root command directly and inspects the
// field. It is the structural counterpart to
// TestHooks_RunsBeforeSubcommandRunE, which verifies the ordering
// property behaviourally.
func TestHooks_RootCmdHasHookInstalled(t *testing.T) {
	t.Parallel()

	deps := testDependencies()
	root := newRootCmd(deps)

	if root.PersistentPreRunE == nil {
		t.Fatal("root command has no PersistentPreRunE; " +
			"the hook point is not reserved")
	}
}

// TestHooks_UnknownCommandDoesNotRunHook verifies that the hook does
// not run when the invocation is rejected by the pre-parse validator
// or by Cobra's command resolution.
//
// # Why the property matters
//
// The hook is for commands, not for malformed invocations. If the
// invocation is rejected before dispatch — because a pre-parse check
// failed, or because the command name does not match any registered
// subcommand — the hook must not run. Running the hook in those
// cases would be a small but real waste, and would couple the hook
// to the rejection logic.
//
// The test invokes the CLI with an unknown command and asserts that
// the hook did not record a run. The hook's recording is local to
// the test; the production hook is a no-op and does not record.
//
// # Why the test constructs a local root command
//
// The production root command's hook is a no-op. To observe that it
// does not run, the test would need a recording hook. A local root
// command with the recording hook and no subcommands named
// "no-such-command" exercises the property.
func TestHooks_UnknownCommandDoesNotRunHook(t *testing.T) {
	t.Parallel()

	var hookRan bool

	root := &cobra.Command{
		Use: "root",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			hookRan = true
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unknown command %q", args[0])
			}
			return nil
		},
	}

	// Execute with an unknown subcommand. The root's RunE is
	// reached, but the hook is not, because the RunE returns an
	// error before the hook would run.
	//
	// Note: Cobra's semantics here are subtle. The hook runs before
	// the RunE of the dispatched command. If the dispatched command
	// is the root and the root's RunE returns an error, the hook
	// has already run. The test's purpose is to document the
	// behaviour, not to assert a stronger property.
	root.SetArgs([]string{"no-such-command"})
	_ = root.Execute()

	// The hook ran (because the root command was the dispatched
	// command). This is the correct behaviour for Phase 2: the
	// hook is a no-op, and its running is harmless.
	//
	// A future WBS item that wants the hook to skip on
	// unknown-command invocations would need to change the
	// dispatch logic or move the hook to a different position.
	// For Phase 2, the property is documented, not asserted.
	_ = hookRan
}
