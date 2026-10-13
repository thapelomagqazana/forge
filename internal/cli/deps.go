// Package cli implements the Forge command-line interface.
//
// This file defines the Dependencies injection struct, the single
// boundary through which every command handler receives its
// collaborators. It is the mechanism that enforces the
// "no business logic in handlers" rule (WBS 4.3): handlers may not
// reach for globals, may not call os.* directly, and may not read
// os.Args or os.Environ.
//
// # Design rationale
//
// The struct exists to keep command constructors stable across
// phases. Every new subsystem (blueprint, template, renderer,
// policy, registry) is added as a field on Dependencies, not as a
// parameter on every constructor. This is the "parameter object"
// refactoring applied preemptively, before the parameter lists grow
// long enough to hurt.
//
// # What is NOT a field
//
// Two collaborators that existed as fields in earlier phases are
// no longer here, and their absence is deliberate:
//
//   - The Logger. The invocation's logger is bound to a fresh
//     correlation ID in the PersistentPreRunE hook (see
//     hooks.go) and stored on the command's context. Handlers
//     read it with logger.FromContext(ctx). Carrying a Logger
//     field would let a handler bypass the context and emit
//     records without the invocation's correlation ID, which
//     defeats the purpose of the ID. WBS 12.3.1 and WBS 12.3.2
//     established the context-based transport.
//
//   - The individual Stdout and Stderr writers. The three
//     standard streams are carried together by a
//     streams.Streams value (WBS 12.4.1). A single field means a
//     caller that wants to redirect the streams redirects all
//     three, and a command that receives the value cannot
//     accidentally read a different stream.
//
// # Import direction
//
// This file imports internal/config, internal/filesystem,
// internal/logger, and internal/streams. It must not import any
// command package. Command packages import cli (or, more
// precisely, receive a Dependencies value from cli). The
// dependency graph is acyclic and one-directional.
package cli

import (
	"github.com/thapelomagqazana/forge/internal/config"
	"github.com/thapelomagqazana/forge/internal/filesystem"
	"github.com/thapelomagqazana/forge/internal/logger"
	"github.com/thapelomagqazana/forge/internal/streams"
)

// Dependencies is the set of collaborators a command may use.
//
// It is the injection boundary between the CLI layer and the command
// handlers. Every command receives a Dependencies value at
// construction time and uses it for every side effect: reading
// configuration, accessing the filesystem, reading the three
// standard streams, reading the environment, and (through the
// command's context) logging.
//
// A command must never reach for a global. It must not read
// os.Args, must not call os.Getenv, must not write to os.Stdout or
// os.Stderr, and must not call os.ReadFile or os.WriteFile. Every
// effect flows through this struct or through the command's
// context.
//
// # Why a struct and not individual parameters
//
// A command's constructor signature could take each dependency as a
// separate parameter. That works while there are few dependencies,
// but it does not scale: every new collaborator added in a later
// phase would require editing the signature of every command
// constructor. Threading a single struct through the command tree
// means the constructor signature is stable, and new collaborators
// appear as new fields.
//
// # Why the fields are interfaces or value types
//
// Config is a pointer to a value object: commands read its fields
// and never call methods on it. Streams is a value type carrying
// three io.Reader/io.Writer fields; it is passed by value and
// copied on read. FS is an interface so tests can substitute a
// fixture filesystem. Env is a function so tests can substitute an
// environment map.
//
// The Logger is not a field. See the package docstring
// §"What is NOT a field".
//
// # Mutability
//
// Commands must not mutate the Dependencies value. The struct is
// passed by value, so mutation of its fields would not affect the
// caller; but the fields it contains (Config, Streams, FS, Env)
// are shared. A command that mutates the pointee of Config would
// be observing a change that other commands do not see. Treat
// every field as read-only.
//
// # Zero value
//
// The zero value of Dependencies is valid but not useful. It has a
// nil Config (which reads return the zero Config), nil Streams
// (which reads return nil writers, and writes panic), a nil FS
// (which panics on use), and a nil Env (which panics on call).
// Tests should construct Dependencies via buildDependencies or
// testDependencies, not via the zero value.
//
// # Extending this struct
//
// Adding a field to Dependencies is the intended mechanism for
// introducing a new collaborator. The field is added, the builder
// in buildDependencies is updated to construct it, and every
// command that wants to use the new collaborator reads it from
// the struct. No command constructor signature changes.
//
// Because the struct is small, the addition is a two-line change.
// Because commands receive the struct by value, the addition does
// not break any existing caller.
//
// The Logger is the deliberate exception: it is not a field, and a
// future collaborator that is per-invocation (like the logger)
// should also travel on the context rather than on this struct.
// The rule of thumb: a collaborator that is the same for every
// command in a process lives here; a collaborator that is bound to
// one invocation (a correlation ID, a trace span, a cancellation
// signal) travels on the context.
type Dependencies struct {
	// Config is the resolved configuration for this invocation.
	//
	// The value is never nil. It is resolved by the CLI's
	// PersistentPreRunE hook (see hooks.go), which applies the
	// four precedence layers (defaults, file, environment, flags)
	// and stores the result on the command's context. A command's
	// RunE handler reads it with ConfigFromContext(ctx).
	//
	// The struct field is retained for the CLI's own internal use
	// (the boundary's error rendering, the hook's idempotence
	// check). Handlers should read the resolved Config from the
	// context, not from this field, so that a future change to
	// the resolution's timing does not surprise them.
	//
	// # Test injection
	//
	// buildDependencies copies opts.config into this field when
	// the caller set it. The hook sees a non-zero Config and
	// treats it as pre-resolved, skipping the loader. This is how
	// a test injects a specific Config.
	Config *config.Config

	// FS is the filesystem abstraction.
	//
	// The value is never nil. It is an OS-backed implementation
	// bound to the rootPath from the options struct. WBS 13.0
	// completes the abstraction's boundary enforcement without
	// changing the interface.
	//
	// Commands read and write files through this field. They must
	// not call os.ReadFile, os.WriteFile, os.Open, or any other
	// os.* filesystem function directly. The Rule 6 contract test
	// enforces the restriction.
	FS filesystem.FS

	// Streams carries the process's three standard streams:
	// stdin, stdout, and stderr.
	//
	// The value is copied from opts.streams by
	// buildDependencies, unchanged. In production it is
	// streams.Default(), the three os.File globals. In tests it
	// is a Streams whose Out and Err are bytes.Buffers and whose
	// In is a strings.Reader.
	//
	// # Why one field instead of three
	//
	// The three streams are used together: a command that reads
	// from stdin and writes to stdout wants both from the same
	// source. Carrying them as one value means an injection
	// replaces all three at once, and a command cannot
	// accidentally read a different stream than the one the test
	// provided. See WBS 12.4.1 for the streams package's design.
	//
	// # Stream contract
	//
	// Product output goes to Streams.Out. Diagnostics go to
	// Streams.Err. The Logger (bound to the context in the hook)
	// writes to Streams.Err as well; the two share the diagnostic
	// stream but not the logger's per-invocation state.
	Streams streams.Streams

	// Env reads environment variables by name.
	//
	// The value is never nil. It is the lookup function from the
	// options struct. Commands that need to read an environment
	// variable call this function rather than calling os.Getenv
	// directly.
	Env func(string) string

	// RootPath is the directory against which relative paths are
	// resolved.
	//
	// The value is normally the process's current working
	// directory. Tests set it to a temporary directory so that
	// path resolution is deterministic.
	//
	// The field is a collaborator, not a derived value: it is
	// read from the process boundary (options.rootPath) by
	// buildDependencies and passed through to commands unchanged.
	// The config loader reads it to discover the configuration
	// file when the --config flag is not set.
	//
	// The empty string means "use the process's current working
	// directory". A caller that wants the default behavior sets
	// the field to the empty string.
	RootPath string

	// LogLevel is the log-level decision resolved by the CLI's
	// boundary (resolveLogLevelWithEnv in execute.go) before the
	// command tree runs.
	//
	// # What the field carries
	//
	// The value is a logger.Decision: the resolved Level and the
	// Source that produced it. The Source is diagnostic (a record
	// that reports "log level: debug (source: --verbose)"), and
	// the Level is what the hook needs to construct the
	// invocation's logger.
	//
	// # Who consumes the field
	//
	// The PersistentPreRunE hook (initializeCommandEnvironment in
	// hooks.go) reads this field to construct the invocation's
	// logger at the resolved level. The logger is stored on the
	// command's context, not on the Dependencies struct; see the
	// package docstring §"What is NOT a field".
	//
	// # Why the field is on Dependencies and not on options
	//
	// The resolution runs after buildDependencies (it needs the
	// constructed command tree to read the flags). Threading the
	// decision through options would require either re-running
	// the resolution or storing it on a struct that is passed by
	// value and already gone by the time the decision exists.
	// The Dependencies struct is captured by the
	// PersistentPreRunE closure; storing the decision there is the
	// smallest transport that survives until the hook runs.
	//
	// # Zero value
	//
	// The zero value is Decision{Level: LevelDebug, Source:
	// SourceDefault}. LevelDebug is the zero value of
	// logger.Level (the constant that starts the iota). A hook
	// that receives a zero-valued LogLevel constructs the
	// logger at DEBUG, which is wrong for a default invocation.
	//
	// To avoid the zero-value trap, buildDependencies normalizes
	// a zero Level (which is indistinguishable from an
	// uninitialized field) to a nonzero "unset" sentinel. See
	// buildDependencies' docstring §"LogLevel field resolution".
	LogLevel logger.Decision
}

// buildDependencies constructs a Dependencies value from the
// injectable options.
//
// It is called exactly once per invocation, from runWithOptions.
// Every field of the returned value is non-nil.
//
// # The construction order
//
// The order in which the fields are constructed matters in one
// case:
//
//   - The filesystem needs the rootPath. The path must be
//     resolved before the filesystem is constructed.
//
// The order below reflects that dependency. Every field is
// constructed from either the options value or from a value
// constructed earlier in the function.
//
// # Why not construct Dependencies in newRootCmd
//
// The construction could live in newRootCmd instead of here. It is
// placed here because runWithOptions owns the options value, and
// the construction is a pure transformation of that value.
// Keeping the transformation next to the value it transforms
// makes the boundary explicit.
//
// # Config field resolution
//
// The Config field has two sources, decided here:
//
//   - If opts.config is non-nil, it is used directly. This is
//     the test-injection path: a test sets opts.config to a
//     pre-built *config.Config, and the hook sees the value and
//     skips the loader.
//
//   - If opts.config is nil, the zero value of config.Config is
//     used. This is the production placeholder: the hook will
//     replace it with the loader's output when the loader runs.
//
// The nil-check is the only place in buildDependencies where a
// field's value depends on whether the caller set it. Every other
// field either has a default that is unconditionally used (FS
// from opts.rootPath) or is passed through unchanged (Streams,
// Env, RootPath).
//
// # Streams field resolution
//
// The Streams field is copied from opts.streams, unchanged. The
// caller (defaultOptions or its test) supplies the streams it
// wants: streams.Default() for the process's standard streams, or
// a Streams value with buffers for a test. buildDependencies does
// not call streams.Default() on the caller's behalf; the choice
// belongs to the caller, and the field is a faithful copy.
//
// # LogLevel field resolution
//
// The LogLevel field is initialized to a well-defined sentinel
// that the hook can recognize as "not yet resolved". The
// resolution runs after buildDependencies returns; the hook
// replaces the sentinel with the resolved Decision before
// constructing the logger.
//
// The sentinel is Decision{Level: LevelInfo, Source:
// SourceDefault} — the same values the resolver would produce
// for an invocation with no flags, no env var, and no config
// file. It is a valid "default level" answer, not a "not set"
// marker. The hook does not distinguish the two: a
// buildDependencies that runs in a code path that never calls
// the resolver (a future test helper, for example) still gets a
// logger at INFO, which is the correct default for the CLI.
//
// A caller that wants a different level attaches the resolved
// decision to the context via logger.ContextWithDecision; the
// hook reads the context first and falls back to the
// LogLevel field when the context is absent. See hooks.go.
//
// # Phase 2 placeholders
//
// FS is an OS-backed implementation bound to rootPath; WBS 13.0
// completes boundary enforcement. Neither change alters the
// interface commands use.
func buildDependencies(opts options) Dependencies {
	// Config: prefer the injected value; fall back to the zero
	// value. See the docstring's "Config field resolution"
	// section.
	//
	// The local variable is used rather than an inline
	// conditional so that the fallback is a single, reviewable
	// statement. A reader who wants to know where Config comes
	// from reads these four lines.
	cfg := opts.config
	if cfg == nil {
		cfg = &config.Config{}
	}

	return Dependencies{
		Config: cfg,

		// FS is bound to the rootPath from the options. In Phase
		// 2, boundary enforcement is not yet implemented; WBS
		// 13.0 completes it.
		FS: filesystem.NewOSFS(opts.rootPath),

		// Streams is copied from the options unchanged. The
		// options value already carries the three streams in a
		// streams.Streams value; buildDependencies does not
		// reassemble them from individual fields (there are
		// none). The copy is a value copy: the three
		// io.Reader/io.Writer fields are shared with the
		// options value, which is the intended behaviour (a
		// test that injects a buffer expects the buffer to
		// receive the CLI's output).
		Streams: opts.streams,

		// The environment lookup is passed through unchanged.
		Env: opts.env,

		// RootPath is passed through unchanged.
		RootPath: opts.rootPath,

		// LogLevel is initialized to the default-level sentinel.
		// The resolver runs after this function returns; the
		// hook replaces the sentinel with the resolved Decision
		// when it constructs the invocation's logger. See the
		// docstring's "LogLevel field resolution" section.
		LogLevel: logger.Decision{
			Level:  logger.LevelInfo,
			Source: logger.SourceDefault,
		},
	}
}
