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
// # Test injection
//
// When deps.Config already carries a non-zero value, the hook
// treats it as pre-resolved and skips the loader. This is how
// tests inject a specific Config:
//
//  1. The test calls runCLIWithConfig(cfg), which sets
//     options.config = cfg.
//  2. buildDependencies copies options.config to deps.Config.
//  3. newRootCmd calls initializeCommandEnvironment(deps),
//     capturing the injected deps.Config.
//  4. Cobra calls the hook. The hook sees a non-zero
//     deps.Config and skips the loader.
//  5. RunE reads the injected Config from the command's
//     context and renders it.
//
// The check is the hook's idempotence condition: if the value
// the hook produces is already present, the hook does nothing.
// Without the check, the hook would overwrite the injected
// value with the loader's output, and no test could inject a
// Config.
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

// configIsZero reports whether every exported field of c is the
// zero value for its type.
//
// The check is used by initializeCommandEnvironment to
// distinguish an injected Config (some field is non-empty) from
// the placeholder zero value that buildDependencies supplies in
// production.
//
// # Why a helper
//
// The concept "is this Config the placeholder value?" is used in
// one place today. Naming it documents the intent and centralizes
// the definition so that a future addition to Config (a fourth
// field) updates this function, not the call site.
//
// # What "zero" means
//
// Config has three exported string fields (LogLevel,
// OutputFormat, SourcePath). A zero Config has all three as
// empty strings. The check is a conjunction of == against "".
//
// # Why not a nil check on deps.Config
//
// buildDependencies always supplies a non-nil pointer (it falls
// back to &config.Config{} when options.config is nil). A nil
// check on deps.Config would always be false and would not
// distinguish injection from production. The field-by-field
// check is the correct test.
//
// # Why the check is here and not in config.go
//
// The concept is about the hook's contract, not about the
// Config type. The Config type has no notion of "is this the
// placeholder"; that notion is specific to how the hook uses
// the value. Placing the helper next to the hook keeps the two
// together.
func configIsZero(c config.Config) bool {
	return c.LogLevel == "" &&
		c.OutputFormat == "" &&
		c.SourcePath == ""
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
// The hook has two paths, distinguished by whether the Config is
// already present on the closure's captured deps:
//
//   - Injection path: deps.Config is non-nil and non-zero. The
//     hook stores the injected value in the command's context
//     and returns. The loader does not run.
//
//   - Production path: deps.Config is nil or the zero value. The
//     hook applies the four configuration layers in order
//     (defaults, file, env, flags) and stores the resolved Config
//     in the command's context.
//
// # The context is the sole transport
//
// Both paths store the resolved (or injected) Config in the
// command's context via contextWithConfig. The dispatched
// command reads it with ConfigFromContext(cmd.Context()).
//
// The hook does *not* write through deps.Config. An earlier
// version assigned the resolved cfg to *deps.Config, keeping the
// closure's deps pointer in sync with the context. That write is
// unsafe when two invocations share a Dependencies value: the
// parallel tests, for example, reuse the same *config.Config
// across subtests, and the write-through races on it. The race
// detector catches the write; the fix is to remove it. The
// context is per-invocation (each Execute() call constructs its
// own), so it is the only transport that is safe under
// concurrency.
//
// A future caller that wants to observe the resolved Config reads
// it from the command's context, not from a deps pointer it
// supplied. This is the documented contract.
//
// # Signature
//
// The hook's signature is fixed by Cobra. The args parameter is
// unused.
//
// # Error behaviour
//
// In the production path, the hook returns the first error from
// any layer. In the injection path, no error can occur: the value
// is already resolved.
func initializeCommandEnvironment(deps Dependencies) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		// Injection path: if deps.Config carries a non-zero
		// value, honor it. Skip the loader and store the
		// injected value in the command's context.
		if deps.Config != nil && !configIsZero(*deps.Config) {
			cmd.SetContext(contextWithConfig(ctx, *deps.Config))
			return nil
		}

		// Production path: apply the four layers.

		cfg := config.Defaults()

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

		cfg, err = config.OverrideFromEnv(cfg, deps.Env)
		if err != nil {
			return err
		}

		cfg, err = config.OverrideFromFlags(cfg, newFlagReader(cmd))
		if err != nil {
			return err
		}

		cmd.SetContext(contextWithConfig(ctx, cfg))
		return nil
	}
}
