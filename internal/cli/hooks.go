// Package cli implements the Forge command-line interface.
//
// This file defines the PersistentPreRunE hook that runs before
// every subcommand's RunE. The hook is where the CLI's
// configuration is loaded: the four precedence layers
// (defaults, file, environment, flags) are applied in order,
// and the resolved Config is stored for the dispatched
// command to read.
//
// # The configuration lifecycle
//
// The hook is the only place where the configuration is loaded
// (WBS 8.4.5). It runs exactly once per invocation, after Cobra
// parses the arguments and before the dispatched subcommand's
// RunE.
//
// The four layers, applied in ascending order of precedence:
//
//  4. Default              (config.Defaults)
//  3. Config file          (config.Discover + config.Load)
//  2. Environment variable (config.OverrideFromEnv)
//  1. CLI flag             (config.OverrideFromFlags)
//
// A layer's non-empty values override the layer below. The
// resolution is documented in docs/cli-ux-spec.md
// "Configuration Loading Lifecycle" and in the config
// package's docstrings.
//
// # Why PersistentPreRunE and not PreRunE
//
// Cobra has two pre-run hooks:
//
//   - PreRunE runs only for the command it is attached to. A
//     subcommand that wants the hook must define it, or inherit
//     it from an ancestor.
//
//   - PersistentPreRunE runs for the command it is attached to
//     and for every descendant. It is the correct hook for
//     concerns that must run for every command, regardless of
//     which command the user invoked.
//
// Configuration loading is a concern of the whole CLI, not of
// individual commands. The persistent hook is the only
// mechanism that runs for every command without requiring each
// command to opt in.
//
// # Why the hook is a closure over deps
//
// The hook needs three values from the resolved environment:
//
//   - The --config flag's value (read from the command).
//   - The environment lookup (deps.Env).
//   - The working directory for discovery (deps.RootPath).
//
// Cobra's PersistentPreRunE receives only the command and the
// args; it has no direct access to the Dependencies value the
// CLI constructed. The hook is therefore a constructor: it
// takes deps and returns the closure Cobra calls. The closure
// captures deps and reads its fields.
//
// newRootCmd calls initializeCommandEnvironment(deps) and
// assigns the returned closure to PersistentPreRunE. See
// root.go.
//
// # Where the resolved configuration is stored
//
// The hook stores the resolved Config in two places:
//
//   - The command's context, via context.WithValue. This is
//     the primary transport; a command's RunE reads it with
//     ConfigFromContext(cmd.Context()).
//
//   - The Dependencies value captured by the closure, via
//     deps.Config. This is a secondary access point for code
//     that already holds a Dependencies value.
//
// # The two transports' visibility
//
// The context transport is visible to the dispatched
// command's RunE: Cobra threads the command's context from
// PersistentPreRunE to RunE.
//
// The deps transport is visible only inside the closure:
// Dependencies is passed by value, so the hook's assignment to
// deps.Config mutates the closure's copy, not the caller's
// value. The caller does not see the update. The hook keeps
// both in sync for consistency; only the context is a
// programmatic transport.
//
// A future command that wants the resolved Config reads it
// from the context. The deps field is retained for the
// case where a future change passes a pointer to Dependencies
// through the hook (for example, if a future logger needs to
// be stored on deps.Logger and read by a caller).
//
// # Errors
//
// An error from any layer is returned from the hook. Cobra
// propagates it to executeWithOptions, which renders it with
// the frozen error format (WBS 7.3.2) and maps it to an exit
// code with exitCodeFromError. The config package's errors
// wrap ErrConfigInvalid; exitCodeFromError classifies them
// as ExitConfig (3).
//
// # No I/O of its own
//
// The hook reads no process globals directly. Every input —
// the --config flag's value, the environment lookup, the
// working directory — is read from the Dependencies value or
// from the command. The hook calls the config package's
// functions; those functions perform the file I/O.
//
// # Enforcement
//
// The rule that no subcommand defines its own
// PersistentPreRunE is enforced by
// TestHooks_NoSubcommandOverrides in hooks_test.go and by the
// Taskfile target verify:hooks. The two checks are
// complementary: the test runs in Go, the Taskfile target
// greps the source tree.
package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/thapelomagqazana/forge/internal/config"
)

// configContextKey is the context key under which the resolved
// configuration is stored.
//
// # Why a private key type
//
// The context key is a package-private type, not a string. A
// string key can collide with a key from another package; a
// private type cannot. Go's vet checks for this pattern and
// warns against string keys for exactly this reason.
type configContextKey struct{}

// ConfigFromContext returns the resolved Config stored in ctx,
// or the zero Config value and false if no Config is present.
//
// # How a command reads the configuration
//
// A command's RunE handler reads the configuration with:
//
//	cfg, ok := ConfigFromContext(cmd.Context())
//	if !ok {
//	    // The hook did not run; this is a programming error.
//	    // The hook is installed on the root command and runs
//	    // before every subcommand's RunE.
//	    return fmt.Errorf("config not initialized")
//	}
//
// # Why the boolean
//
// The two-result form distinguishes "the config is the zero
// value" from "the config is absent". A caller that does not
// distinguish the two would silently use the zero value when
// the hook failed; the boolean forces the caller to handle the
// absent case.
//
// # Purity
//
// The function reads only its argument. It does not mutate
// ctx. Two calls with the same ctx return the same values.
func ConfigFromContext(ctx context.Context) (config.Config, bool) {
	v := ctx.Value(configContextKey{})
	if v == nil {
		return config.Config{}, false
	}
	cfg, ok := v.(config.Config)
	return cfg, ok
}

// contextWithConfig returns a new context with cfg stored under
// the config key.
//
// # Why the helper is unexported
//
// The helper is used by the hook to store the config; no other
// caller needs it. Keeping it unexported makes the write path
// (the hook) and the read path (ConfigFromContext) asymmetric:
// any code can read the config, but only the hook can write
// it. The asymmetry is deliberate: a command that could write
// the config could confuse the resolution.
func contextWithConfig(ctx context.Context, cfg config.Config) context.Context {
	return context.WithValue(ctx, configContextKey{}, cfg)
}

// initializeCommandEnvironment returns the root command's
// PersistentPreRunE hook, with deps captured in the closure.
//
// It is called exactly once, from newRootCmd. The returned
// closure has the signature Cobra requires for
// PersistentPreRunE:
//
//	func(cmd *cobra.Command, args []string) error
//
// # What the hook does
//
// The hook applies the four configuration layers in order:
//
//  1. Defaults. config.Defaults produces a Config with the
//     schema's default values.
//
//  2. Config file. config.Discover finds the file (using the
//     --config flag's value, deps.Env, and deps.RootPath);
//     config.Load reads it. If no file is found, the defaults
//     are used.
//
//  3. Environment variables. config.OverrideFromEnv applies
//     the variables the schema declares.
//
//  4. CLI flags. config.OverrideFromFlags applies the flags
//     the schema declares.
//
// The resolved Config is stored in the command's context and
// in the closure's captured deps value.
//
// # Signature
//
// The hook's signature is fixed by Cobra. The args parameter is
// unused; the hook's behaviour does not depend on the
// positional arguments.
//
// # Error behaviour
//
// The hook returns the first error from any layer. Errors from
// config.Discover, config.Load, config.OverrideFromEnv, and
// config.OverrideFromFlags are returned unchanged. The CLI's
// error path renders them and maps them to exit codes; the
// config package's errors map to ExitConfig (3).
//
// # The captured Dependencies value
//
// The deps argument is captured by the returned closure. The
// capture is what lets the hook read deps.Env and deps.RootPath
// and write deps.Config for code inside the closure.
//
// The Dependencies struct is passed by value; the closure's
// copy is independent of the caller's. A caller that wants to
// see the resolved Config reads it from the command's context,
// not from its own deps value.
func initializeCommandEnvironment(deps Dependencies) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		// Layer 4: defaults.
		cfg := config.Defaults()

		// Layer 3: the config file.
		explicitPath := configPath(cmd)
		cfgPath, err := config.Discover(explicitPath, deps.Env, deps.RootPath)
		if err != nil {
			return err
		}
		if cfgPath != "" {
			cfg, err = config.Load(cfgPath)
			if err != nil {
				return err
			}
			cfg.SourcePath = cfgPath
		}

		// Layer 2: environment variables.
		cfg, err = config.OverrideFromEnv(cfg, deps.Env)
		if err != nil {
			return err
		}

		// Layer 1: CLI flags. The reader adapts the command to
		// the config package's FlagReader interface; see
		// flags_adapter.go.
		cfg, err = config.OverrideFromFlags(cfg, newFlagReader(cmd))
		if err != nil {
			return err
		}

		// Store the resolved config in the command's context
		// and in the closure's captured deps value. The two are
		// kept in sync; a command reads whichever is convenient
		// for its call site.
		//
		// The context store is the programmatic transport; the
		// deps store is visible only inside the closure (see the
		// function's docstring).
		cmd.SetContext(contextWithConfig(ctx, cfg))
		if deps.Config != nil {
			*deps.Config = cfg
		}

		return nil
	}
}
