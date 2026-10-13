// Package cli implements the Forge command-line interface.
//
// This file defines the PersistentPreRunE hook that runs before
// every subcommand's RunE. The hook is where the CLI's
// per-invocation state is prepared: the logger is built (bound to
// a fresh correlation ID) and the configuration is loaded. Both
// are stored on the command's context, where the dispatched
// subcommand reads them.
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
// # The logger and correlation-ID lifecycle (WBS 12.2.1, 12.3.1)
//
// The hook builds the invocation's Logger once, at the top of the
// hook, and stores it on the command's context via
// logger.IntoContext. The Logger carries:
//
//   - A ComponentCLI component tag (the hook is the CLI layer).
//   - A fresh correlation ID from logger.NewCorrelationID, so
//     every record the invocation emits shares the same ID.
//   - A Sink that writes to the process's stderr in the format
//     selected by FORGE_LOG_FORMAT.
//
// Downstream code reads the Logger with
// logger.FromContext(cmd.Context()); the Logger's four level
// methods emit records whose correlation_id field is the ID
// generated here. The Logger is the transport, not the ID: a
// caller who wants to log through the invocation's logger does
// not need to know about correlation IDs at all.
//
// # The correlation ID is generated once, before config loading
//
// The ID is bound into the Logger at construction. Any record
// emitted *during* configuration loading (for example, a debug
// record explaining which config file was discovered) carries the
// ID, because the Logger is on the context before the loader runs.
// This ordering is deliberate: a consumer who wants to trace an
// entire invocation, including its configuration resolution, sees
// one ID on every record from start to finish.
//
// Records emitted *before* the hook runs (for example, by Cobra's
// argument parsing) or *after* the dispatched subcommand returns
// carry ZeroCorrelationID, the sentinel for "not part of an
// invocation". See docs/reference/log-record-format.md
// §"Interaction with correlation IDs".
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
// Logger construction and configuration loading are concerns of
// the whole CLI, not of individual commands. The persistent hook
// is the only mechanism that runs for every command without
// requiring each command to opt in.
//
// # Why the hook is a closure over deps
//
// The hook needs four values from the resolved environment:
//
//   - The --config flag's value (read from the command).
//   - The environment lookup (deps.Env).
//   - The working directory for discovery (deps.RootPath).
//   - The stderr writer for the Logger (deps.Streams.Err).
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
// The hook stores the resolved Config in the command's context,
// via contextWithConfig. This is the sole transport; a command's
// RunE reads it with ConfigFromContext(cmd.Context()).
//
// An earlier version also wrote the resolved Config through the
// closure's captured deps.Config pointer. That write was removed:
// Dependencies is a value type, but its Config field is a
// *config.Config, and two invocations that share a Dependencies
// value would race on the pointee. The race detector caught the
// write in the parallel tests. The context is per-invocation —
// each Execute() call constructs its own — so it is the only
// transport that is safe under concurrency.
//
// # Where the Logger is stored
//
// The Logger is stored in the same command context, under a
// private key owned by internal/logger. The CLI does not read it
// back; downstream callers use logger.FromContext. The CLI's only
// role is to construct the Logger and call logger.IntoContext
// once.
//
// The correlation ID travels *inside* the Logger, not as a
// separate context value. This is the correct design for the
// Logger interface introduced by WBS 12.3.1: the interface's
// methods do not take a context, so the Logger must carry its own
// per-invocation state. Storing the ID and the Logger as two
// separate context values would leave the Logger unaware of the
// ID.
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
// The Logger is built unconditionally, on both paths. A test that
// injects a Config still gets a real Logger bound to a real
// correlation ID; there is no test-only bypass.
//
// # Log-level and log-format resolution
//
// The Logger's level is resolved in three tiers, from highest
// precedence to lowest:
//
//  1. The decision attached to the command's context by
//     runWithOptions (via logger.ContextWithDecision). This is
//     the production path: runWithOptions resolves the level
//     from the flags and the env var, detects the
//     --quiet/--verbose conflict, and attaches the resulting
//     Decision to the context before ExecuteContext runs.
//
//  2. The deps.LogLevel field, initialized by
//     buildDependencies to the default decision (Info,
//     Default). This tier exists so a caller that invokes the
//     hook outside runWithOptions (a unit test, a future
//     library consumer) gets a sensible default rather than
//     the zero value of logger.Level (which is Debug, and is
//     wrong for a default invocation).
//
//  3. The environment variable FORGE_LOG_LEVEL, parsed by
//     loggerLevelFromEnv. This tier is the last fallback: a
//     caller that has neither a context decision nor a
//     populated LogLevel field but does have an Env lookup
//     still gets the env-var's value.
//
// The three tiers are ordered by how directly they reflect the
// invocation's intent: the context decision is the resolver's
// answer, the field is a default the builder set, the env var is
// a hint from the caller's environment. The tiers do not
// contradict each other in practice (all three agree on the
// default), but the ordering makes the resolution deterministic
// when a caller supplies more than one.
//
// The Logger's format is read from FORGE_LOG_FORMAT via
// loggerFormatFromEnv. There is no per-invocation format
// decision: WBS 12.5.1 does not add a --log-format flag, so the
// env var is the only source.
//
// An unrecognized value falls back to the package default
// (Info, Human); the invocation is not rejected. A future WBS
// that wants a malformed value to be a usage error changes the
// helpers to return an error and the hook to propagate it.
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
// The logger construction cannot fail in the current design:
// logger.NewCorrelationID panics on a failure of the system's
// entropy source, and logger.New panics only on a missing
// Component (which the hook always supplies). The panics
// propagate through Cobra; the process exits non-zero with a
// stack trace. In practice, crypto/rand does not fail on the
// platforms Forge supports.
//
// # No I/O of its own
//
// The hook reads no process globals directly. Every input — the
// --config flag's value, the environment lookup, the working
// directory, the stderr writer — is read from the Dependencies
// value or from the command. The hook calls the config and
// logger packages' functions; those functions perform the
// process-global reads they need.
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
	"github.com/thapelomagqazana/forge/internal/logger"
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
//
// # Why the logger uses a different key
//
// The Logger is stored by the logger package, under the logger's
// own private key (logger.contextKey). The CLI does not read the
// Logger back through a direct key; downstream callers use
// logger.FromContext. This split keeps each package's context
// keys private to the package that owns them.
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
	if ctx == nil {
		return config.Config{}, false
	}
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
//
// # Composition with the logger
//
// contextWithConfig wraps an existing context. The hook applies
// it after logger.IntoContext has already wrapped the context,
// so the resulting context carries both the logger and the
// config. The two keys are independent; a future addition of a
// third per-invocation value composes the same way.
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
// The hook performs two independent preparations and stores their
// results on the command's context:
//
//  1. It builds the invocation's Logger (WBS 12.3.1) bound to a
//     fresh correlation ID (WBS 12.2.1), and attaches the Logger
//     via logger.IntoContext. The Logger is built
//     unconditionally, on both the injection and the production
//     path.
//
//  2. It resolves the configuration (WBS 8.4.5) and attaches it
//     via contextWithConfig. Two paths, distinguished by whether
//     the Config is already present on the closure's captured
//     deps:
//
//     - Injection path: deps.Config is non-nil and non-zero. The
//     hook stores the injected value in the command's context
//     and returns. The loader does not run.
//
//     - Production path: deps.Config is nil or the zero value.
//     The hook applies the four configuration layers in order
//     (defaults, file, env, flags) and stores the resolved
//     Config in the command's context.
//
// The two preparations are ordered: the Logger is attached
// first, so that any record emitted during configuration loading
// carries the invocation's correlation ID. See the package
// docstring §"The logger and correlation-ID lifecycle".
//
// # The Logger is the transport, not the ID
//
// Downstream code reads the Logger with
// logger.FromContext(cmd.Context()). The Logger's four level
// methods emit records whose correlation_id field is the ID
// generated here. A caller who wants to log through the
// invocation's logger does not need to know about correlation
// IDs: the Logger carries the ID internally.
//
// This is the correct design for the Logger interface introduced
// by WBS 12.3.1: the interface's methods do not take a context,
// so the Logger must carry its own per-invocation state. Storing
// the ID and the Logger as two separate context values would
// leave the Logger unaware of the ID.
//
// # The context is the sole transport
//
// Both preparations store their results in the command's context.
// The dispatched command reads the config with
// ConfigFromContext(cmd.Context()) and the logger with
// logger.FromContext(cmd.Context()).
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
//
// logger.NewCorrelationID and logger.New can panic. The panics
// are not converted to errors: a process that cannot build its
// logger cannot meaningfully report the failure through the CLI's
// error contract either. See the package docstring §"Errors".
func initializeCommandEnvironment(deps Dependencies) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		// Step 1 — build the invocation's Logger (WBS 12.3.1)
		// bound to a fresh correlation ID (WBS 12.2.1), and attach
		// it to the context.
		//
		// The Logger carries:
		//
		//   - ComponentCLI, the subsystem tag for the CLI layer.
		//   - A fresh v4 UUID from logger.NewCorrelationID. Every
		//     record the invocation emits carries this ID.
		//   - A Sink that writes to stderr, formatted according to
		//     FORGE_LOG_FORMAT (human by default).
		//
		// The order matters: the Logger is built and attached before
		// the configuration is loaded, so any record emitted during
		// loading (a debug trace, a warning about a deprecated field)
		// carries the invocation's ID.
		//
		// The construction is unconditional. A test that injects a
		// Config still gets a real Logger bound to a real ID; there
		// is no test-only bypass, because the bypass would hide a
		// regression in the construction's wiring.
		//
		// The Writer is deps.Streams.Err: the process's stderr
		// under the stream contract (ADR-022). Diagnostics go to
		// stderr; product output goes to deps.Streams.Out and is
		// the responsibility of the dispatched command.
		//
		// # The level's three-tier resolution
		//
		// The level is taken from the first of these tiers that
		// provides a value:
		//
		//  1. The Decision attached to the context by
		//     runWithOptions (via logger.ContextWithDecision).
		//     This is the production path: the resolver ran before
		//     ExecuteContext and its output is on the context.
		//
		//  2. The deps.LogLevel field. buildDependencies
		//     initializes it to (Info, Default); a caller that
		//     invokes the hook outside runWithOptions gets a
		//     sensible default rather than the zero value of
		//     logger.Level (Debug).
		//
		//  3. The FORGE_LOG_LEVEL env var, via loggerLevelFromEnv.
		//     This is the last fallback for a caller that has
		//     neither a context decision nor a populated LogLevel
		//     field.
		//
		// The three tiers agree on the default (Info), so the
		// ordering is only observable when a caller supplies more
		// than one.
		level := loggerLevelFromEnv(deps)
		if d, ok := logger.DecisionFromContext(ctx); ok {
			level = d.Level
		} else if deps.LogLevel.Level != 0 {
			level = deps.LogLevel.Level
		}

		lg := logger.New(logger.Options{
			Level:         level,
			Format:        loggerFormatFromEnv(deps),
			Component:     logger.ComponentCLI,
			CorrelationID: logger.NewCorrelationID(),
			Writer:        deps.Streams.Err,
		})
		ctx = logger.IntoContext(ctx, lg)

		// Step 2 — configuration (WBS 8.4.5).
		//
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

// loggerLevelFromEnv resolves the log level from the Dependencies
// environment.
//
// # Contract
//
// FORGE_LOG_LEVEL is parsed by logger.ParseLevel, the same
// function the CLI's --log-level flag uses. The accepted values,
// the case-insensitivity, the whitespace trimming, and the
// "warning" alias for "warn" are therefore identical between the
// env var and the flag. A future change to the accepted values
// updates logger.ParseLevel in one place and both call sites
// inherit it.
//
// # Role in the hook's resolution
//
// The helper is the third-tier fallback: the hook uses it when
// neither the context carries a decision nor the Dependencies
// LogLevel field is populated. See the hook's docstring
// §"The level's three-tier resolution".
//
// The helper is exported (as an unexported package-level
// function) so that a future WBS can replace it with a variant
// that returns an error for a malformed value, without changing
// the hook's structure.
//
// # Fallback on error
//
// An unrecognized value falls back to LevelInfo with no error
// returned. The hook does not reject the invocation for a
// malformed FORGE_LOG_LEVEL: a process that cannot parse its
// logging configuration can still run, and the default level is
// the safe choice. A future WBS that wants a malformed
// FORGE_LOG_LEVEL to be a usage error (exit 64) changes this
// function to return an error and the hook to propagate it.
//
// # Why a helper
//
// The helper is package-level rather than inline so a test can
// substitute its behavior. It reads only deps.Env; it does not
// read the process environment, so a test that injects an env
// map gets exactly the value the map contains.
func loggerLevelFromEnv(deps Dependencies) logger.Level {
	if deps.Env == nil {
		return logger.LevelInfo
	}
	lvl, err := logger.ParseLevel(deps.Env(logger.EnvVar))
	if err != nil {
		return logger.LevelInfo
	}
	return lvl
}

// loggerFormatFromEnv resolves the log format from the
// Dependencies environment.
//
// # Contract
//
// FORGE_LOG_FORMAT is parsed by logger.ParseFormat, the same
// function the logger package uses internally. The accepted
// values are "human" and "json", case-insensitively; empty is
// treated as "human". A future change to the accepted values
// updates logger.ParseFormat in one place and both call sites
// inherit it.
//
// # Role in the hook's resolution
//
// There is no per-invocation format decision. WBS 12.5.1 does
// not add a --log-format flag, so the env var is the only
// source. When a future WBS adds the flag, the hook will read
// the flag's value (from the command) with higher precedence,
// mirroring the level's three-tier resolution. Until then, this
// helper is the sole source.
//
// # Fallback on error
//
// An unrecognized value falls back to FormatHuman with no error
// returned. The hook does not reject the invocation for a
// malformed FORGE_LOG_FORMAT: a process that cannot parse its
// logging configuration can still run, and the human format is
// the safe choice. It is human-readable and cannot corrupt a
// downstream parser that is not expecting JSON. A future WBS
// that wants a malformed FORGE_LOG_FORMAT to be a usage error
// (exit 64) changes this function to return an error and the
// hook to propagate it.
//
// # Why a helper
//
// The helper is package-level rather than inline so a test can
// substitute its behavior. It reads only deps.Env; it does not
// read the process environment, so a test that injects an env
// map gets exactly the value the map contains.
func loggerFormatFromEnv(deps Dependencies) logger.Format {
	if deps.Env == nil {
		return logger.FormatHuman
	}
	f, err := logger.ParseFormat(deps.Env(logger.EnvVarFormat))
	if err != nil {
		return logger.FormatHuman
	}
	return f
}
