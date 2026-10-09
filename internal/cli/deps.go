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
// # Import direction
//
// This file imports internal/config and internal/filesystem. It
// must not import any command package. Command packages import cli
// (or, more precisely, receive a Dependencies value from cli). The
// dependency graph is acyclic and one-directional.
package cli

import (
	"io"
	"log/slog"

	"github.com/thapelomagqazana/forge/internal/config"
	"github.com/thapelomagqazana/forge/internal/filesystem"
)

// Dependencies is the set of collaborators a command may use.
//
// It is the injection boundary between the CLI layer and the command
// handlers. Every command receives a Dependencies value at
// construction time and uses it for every side effect: reading
// configuration, writing output, accessing the filesystem, logging.
//
// A command must never reach for a global. It must not read
// os.Args, must not call os.Getenv, must not write to os.Stdout or
// os.Stderr, and must not call os.ReadFile or os.WriteFile. Every
// effect flows through this struct.
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
// # Why the fields are interfaces
//
// Config is a concrete type because it is a value object with no
// behaviour. The other fields are interfaces: Logger, FS, io.Writer,
// and func(string) string. Interfaces allow tests to substitute
// lightweight implementations without touching the real subsystems.
//
// # Mutability
//
// Commands must not mutate the Dependencies value. The struct is
// passed by value, so mutation would not affect the caller; but the
// fields it contains (Config, Logger, FS) are shared. A command that
// mutates its Config would be observing a change that other commands
// do not see. Treat every field as read-only.
//
// # Zero value
//
// The zero value of Dependencies is valid but not useful. It has a
// nil Config (which is an empty struct, so reads are safe), a nil
// Logger (which would panic on use), a nil FS (which would panic on
// use), nil writers (which would panic on use), and a nil Env (which
// would panic on call). Tests should construct Dependencies via
// buildDependencies or testDependencies, not via the zero value.
//
// # Extending this struct
//
// Adding a field to Dependencies is the intended mechanism for
// introducing a new collaborator. The field is added, the builder
// in buildDependencies is updated to construct it, and every command
// that wants to use the new collaborator reads it from the struct.
// No command constructor signature changes.
//
// Because the struct is small, the addition is a two-line change.
// Because commands receive the struct by value, the addition does
// not break any existing caller.
type Dependencies struct {
	// Config is the resolved configuration for this invocation.
	//
	// The value is never nil. In Phase 2 it is the zero value of
	// config.Config, which is an empty struct. WBS 8.2.1 populates
	// the struct with fields; commands that read those fields will
	// see the resolved values.
	Config *config.Config

	// Logger is the diagnostic logger.
	//
	// The value is never nil. In Phase 2 it is a logger backed by
	// log/slog writing to the injected stderr. WBS 12.0 refines
	// the logger's behaviour (levels, structured fields, redaction)
	// without changing the interface commands use.
	Logger Logger

	// FS is the filesystem abstraction.
	//
	// The value is never nil. In Phase 2 it is an OS-backed
	// implementation bound to the rootPath from the options struct.
	// WBS 13.0 completes the abstraction's boundary enforcement
	// without changing the interface.
	FS filesystem.FS

	// Stdout is the destination for successful command output.
	//
	// The value is never nil. In Phase 2 it is the writer from the
	// options struct. Commands write their primary output here.
	Stdout io.Writer

	// Stderr is the destination for diagnostics and errors.
	//
	// The value is never nil. In Phase 2 it is the writer from the
	// options struct. Commands write warnings, progress, and error
	// messages here.
	Stderr io.Writer

	// Env reads environment variables by name.
	//
	// The value is never nil. In Phase 2 it is the lookup function
	// from the options struct. Commands that need to read an
	// environment variable call this function rather than calling
	// os.Getenv directly.
	Env func(string) string
}

// Logger is the diagnostic logging interface used by commands.
//
// The interface is deliberately minimal. It has five methods, each
// corresponding to a log level or a scope extension. Commands do not
// need more.
//
// # Why not use log/slog directly
//
// Commands could receive a *slog.Logger and call its methods
// directly. That works today, but it couples every command to the
// slog package. If WBS 12.0 replaces slog with a different logging
// library, every command that calls slog directly would need to be
// updated. The Logger interface decouples commands from the
// implementation.
//
// # Why these five methods
//
// The four level methods (Debug, Info, Warn, Error) correspond to
// the four levels the CLI UX specification defines. The With method
// returns a logger with additional structured fields, so that a
// command can attach context (for example, a project name) once and
// have it appear on every subsequent log message.
//
// # Structured fields
//
// The level methods take a variadic list of key-value pairs rather
// than a formatted string. This is deliberate: structured logging is
// the CLI UX specification's policy, and the interface enforces it.
type Logger interface {
	// Debug logs a message at the debug level.
	//
	// Debug messages are emitted only when the user has requested
	// verbose output (--verbose or --debug).
	Debug(msg string, args ...any)

	// Info logs a message at the info level.
	//
	// Info messages are the default level. They are suppressed by
	// --quiet.
	Info(msg string, args ...any)

	// Warn logs a message at the warning level.
	//
	// Warn messages indicate potential issues. They are suppressed
	// by --quiet.
	Warn(msg string, args ...any)

	// Error logs a message at the error level.
	//
	// Error messages indicate failures. They are never suppressed.
	Error(msg string, args ...any)

	// With returns a logger that includes the given key-value pairs
	// on every subsequent message.
	//
	// The returned logger is independent of the receiver; adding
	// fields to one does not affect the other.
	With(args ...any) Logger
}

// slogLogger adapts a *slog.Logger to the Logger interface.
//
// The adapter is unexported because no package outside internal/cli
// needs to construct it. Tests that want a Logger use newNoopLogger
// or the adapter constructed by buildDependencies.
type slogLogger struct {
	inner *slog.Logger
}

// Debug logs a message at the debug level.
func (l *slogLogger) Debug(msg string, args ...any) {
	l.inner.Debug(msg, args...)
}

// Info logs a message at the info level.
func (l *slogLogger) Info(msg string, args ...any) {
	l.inner.Info(msg, args...)
}

// Warn logs a message at the warning level.
func (l *slogLogger) Warn(msg string, args ...any) {
	l.inner.Warn(msg, args...)
}

// Error logs a message at the error level.
func (l *slogLogger) Error(msg string, args ...any) {
	l.inner.Error(msg, args...)
}

// With returns a logger that includes the given fields.
func (l *slogLogger) With(args ...any) Logger {
	return &slogLogger{inner: l.inner.With(args...)}
}

// noopLogger is a Logger implementation that discards every message.
//
// It is used in tests that do not care about log output, and it is
// the default logger for a Dependencies value that has not been
// configured with a real logger.
//
// The implementation is deliberately trivial: every method is a
// no-op, and With returns the receiver.
type noopLogger struct{}

// Debug discards the message.
func (noopLogger) Debug(string, ...any) {}

// Info discards the message.
func (noopLogger) Info(string, ...any) {}

// Warn discards the message.
func (noopLogger) Warn(string, ...any) {}

// Error discards the message.
func (noopLogger) Error(string, ...any) {}

// With returns the receiver.
func (l noopLogger) With(...any) Logger { return l }

// newNoopLogger returns a Logger that discards every message.
//
// It is exported for use by tests in other packages that need a
// Logger without constructing a real one. Production code should
// use the Logger from buildDependencies.
func newNoopLogger() Logger { return noopLogger{} }

// newLogger returns a Logger writing to the given writer.
//
// The returned logger is a slogLogger wrapped around a slog.Logger
// configured with a text handler. The text handler writes structured
// key-value pairs, matching the CLI UX specification's policy for
// human-readable log output.
//
// In Phase 2, the logger's level is fixed at Debug. WBS 12.0 wires
// the level to the --verbose and --quiet flags.
func newLogger(w io.Writer) Logger {
	handler := slog.NewTextHandler(w, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	return &slogLogger{inner: slog.New(handler)}
}

// buildDependencies constructs a Dependencies value from the
// injectable options.
//
// It is called exactly once per invocation, from executeWithOptions.
// Every field of the returned value is non-nil.
//
// # The construction order
//
// The order in which the fields are constructed matters in two
// cases:
//
//  1. The logger needs the stderr writer. The writer must be
//     captured before the logger is constructed.
//
//  2. The filesystem needs the rootPath. The path must be resolved
//     before the filesystem is constructed.
//
// The order below reflects these dependencies. Every field is
// constructed from either the options value or from a value
// constructed earlier in the function.
//
// # Why not construct Dependencies in newRootCmd
//
// The construction could live in newRootCmd instead of here. It is
// placed here because executeWithOptions owns the options value, and
// the construction is a pure transformation of that value. Keeping
// the transformation next to the value it transforms makes the
// boundary explicit.
//
// # Phase 2 placeholders
//
// Config is the zero value of config.Config. WBS 8.4 replaces this
// with a call to the configuration loader. FS is an OS-backed
// implementation bound to rootPath; WBS 13.0 completes boundary
// enforcement. Logger is a slog-backed implementation; WBS 12.0
// wires levels to flags. None of these changes alter the interface.
func buildDependencies(opts options) Dependencies {
	return Dependencies{
		// Config is the zero value in Phase 2. WBS 8.4 replaces
		// this with a call to the configuration loader.
		Config: &config.Config{},

		// Logger writes to stderr, so that stdout remains clean
		// for command output.
		Logger: newLogger(opts.stderr),

		// FS is bound to the rootPath from the options. In Phase
		// 2, boundary enforcement is not yet implemented; WBS
		// 13.0 completes it.
		FS: filesystem.NewOSFS(opts.rootPath),

		// The output streams are passed through unchanged.
		Stdout: opts.stdout,
		Stderr: opts.stderr,

		// The environment lookup is passed through unchanged.
		Env: opts.env,
	}
}
