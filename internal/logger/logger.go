// Package logger implements the operational surface of the forge CLI.
//
// # Purpose
//
// The logger carries *diagnostics* — information about how a command
// ran, not what it produced. Logs, traces, timing, debug context.
// They are written to stderr only, are level-controlled, and are
// never injected into stdout.
//
// # Layer contract
//
//	Output  → stdout  (product result)
//	Logger  → stderr  (operational diagnostics)
//	Errors  → stderr  (rendered by internal/forgeerr)
//
// This package MUST NEVER write to stdout. Product output belongs to
// internal/output. See ADR-022 and
// docs/architecture/output-vs-logging.md.
//
// # The interface
//
// A Logger is the programming interface for emitting diagnostics.
// The interface has four level methods and one combinator (With):
//
//	Debug(msg string, args ...any)
//	Info(msg string, args ...any)
//	Warn(msg string, args ...any)
//	Error(msg string, args ...any)
//	With(args ...any) Logger
//
// The default implementation (see slog_adapter.go) is backed by
// log/slog. The no-op implementation (see nop.go) is the correct
// value for tests and for code paths where logging is disabled.
// A caller who wants a different implementation (OTLP, syslog, a
// test spy) implements the interface; the package does not expose
// slog.
//
// # The record
//
// Every call produces one Record (see record.go). The record carries
// a timestamp, a level, a component tag, a message, a
// per-invocation correlation ID, and free-form structured attrs.
// Two renderings are produced by the default implementation:
//
//	human  [LEVEL] [component] message key=value key=value ...
//	json   {"ts":...,"level":...,"component":...,"msg":...,
//	        "correlation_id":...,"attrs":{...}}
//
// The human format is the default and is NOT stable; consumers must
// not parse it. The JSON format is documented by
// schemas/log-record.schema.json and is stable within a major
// version. See docs/reference/log-record-format.md.
//
// # Component and correlation ID
//
// A Logger carries a fixed Component (set at construction). The
// invocation's correlation ID travels with the Logger, not with the
// call:
//
//	l := logger.FromContext(ctx)
//	l.Info("loaded configuration", "path", p)
//
// FromContext reads the ID from the context and returns a Logger
// bound to it. The ID is generated once per invocation in
// internal/cli/hooks.go and attached to the command's context.
// Records emitted outside an invocation carry ZeroCorrelationID.
//
// # Construction
//
// A Logger is constructed with New(Options{...}):
//
//	l := logger.New(logger.Options{
//	    Level:     logger.LevelInfo,
//	    Format:    logger.FormatJSON,
//	    Component: logger.ComponentCLI,
//	    Writer:    os.Stderr,
//	})
//
// The zero Options produces a logger at LevelInfo in FormatHuman
// writing to os.Stderr. Production code must set Component; New
// panics otherwise (see New for the rationale and the debug-level
// exception).
//
// # The no-op logger
//
// Nop() returns a Logger whose methods do nothing. It is the
// correct value for tests that do not assert on logs, for optional
// logger fields, and for code paths where logging is disabled.
// Nop is allocation-free and panic-free by construction.
//
// # Anti-patterns
//
// Do NOT use fmt.Println or os.Stdout.Write in this package.
// Do NOT log the product result — that is output.
// Do NOT log secrets; redaction is handled by internal/forgeerr.
//
// # Stability
//
// The interface's method set is a public contract. Adding a method
// requires an ADR. The With combinator's semantics ("a child shares
// the parent's level, format, component, and sink; it prepends its
// args to every record") are likewise a contract. The concrete
// implementation (slogLogger) is not part of the contract; a caller
// who depends on it is depending on an implementation detail.
package logger

import (
	"context"
	"io"
	"sync"
)

// =============================================================================
// Logger interface
// =============================================================================

// Logger is the operational surface's programming interface.
//
// # Method set
//
// A Logger has four level methods and one combinator:
//
//	Debug(msg string, args ...any)
//	Info(msg string, args ...any)
//	Warn(msg string, args ...any)
//	Error(msg string, args ...any)
//	With(args ...any) Logger
//
// The four level methods emit one record at the named level. The
// msg is the record's Message; the args are variadic key-value
// pairs that populate the record's Attrs. An odd-length args is
// tolerated: the trailing key maps to nil.
//
// The With method returns a child logger that prepends its args to
// every record. A child shares the parent's level, format,
// component, and sink; only its Attrs differ. A child is safe for
// concurrent use.
//
// # Concurrency
//
// Every method must be safe for concurrent use by multiple
// goroutines. The interface does not expose a SetLevel method; a
// caller that needs to change the level after construction does so
// through the concrete type, not through the interface.
//
// # Error handling
//
// The methods do not return an error. A write to stderr that fails
// is not actionable: the process cannot report a failure to report
// a failure. The default implementation drops the error; see
// StderrSink (in sink.go) for the rationale. An implementation that
// wants to observe errors wraps its own writer.
type Logger interface {
	// Debug logs at debug level.
	Debug(msg string, args ...any)

	// Info logs at info level.
	Info(msg string, args ...any)

	// Warn logs at warn level.
	Warn(msg string, args ...any)

	// Error logs at error level.
	//
	// Note: the error *value* should flow through
	// internal/forgeerr; this method records the operational
	// event.
	Error(msg string, args ...any)

	// With returns a child logger that prepends args to every
	// record it emits.
	//
	// The child shares the parent's level, format, component,
	// and sink. The args are parsed with the same key-value
	// convention as the level methods: an odd-length args is
	// tolerated.
	//
	// A With call with no args returns a logger equivalent to
	// the receiver. Implementations are encouraged (but not
	// required) to return the receiver itself in that case.
	With(args ...any) Logger
}

// =============================================================================
// Construction
// =============================================================================

// Options configures a Logger created by New.
//
// # Zero-value semantics
//
// A zero Options produces a Logger that:
//
//   - uses LevelInfo (the default level),
//   - uses FormatHuman (the default rendering),
//   - emits with ComponentInvalid (which New rejects at non-debug
//     levels; see New),
//   - writes to os.Stderr,
//   - emits records with CorrelationID = ZeroCorrelationID.
//
// Production code should always set Level, Format, and Component.
// Code that runs inside a command invocation should also set
// CorrelationID (see below).
//
// # Writer vs. Sink
//
// Exactly one of Writer and Sink is used:
//
//   - If Sink is non-nil, it is used directly. This is the seam
//     for tests that want to observe records as structured data
//     rather than as bytes.
//
//   - Otherwise, Writer is used. If Writer is also nil, os.Stderr
//     is used.
//
// A caller should not set both. If both are set, Sink wins; the
// Writer field is ignored.
//
// # CorrelationID
//
// CorrelationID is the invocation's correlation ID, as produced by
// NewCorrelationID. Every record the Logger emits carries this
// value in its CorrelationID field.
//
// An empty CorrelationID is replaced by ZeroCorrelationID: the
// sentinel for "not part of an invocation". A Logger constructed
// outside an invocation (for example, at package initialization)
// therefore emits records with the sentinel, which a consumer can
// filter out.
//
// The CLI's PersistentPreRunE constructs the invocation's Logger
// with a fresh CorrelationID (see internal/cli/hooks.go). A test
// that wants to assert on the ID binds its own value; a test that
// does not care leaves the field zero and observes the sentinel.
type Options struct {
	// Level is the minimum level to emit. Zero value is
	// LevelInfo.
	Level Level

	// Format is the rendering. Zero value is FormatHuman.
	Format Format

	// Component is the subsystem tag attached to every record.
	// Zero value is ComponentInvalid; production code must set
	// a canonical component. New panics on a zero Component at
	// non-debug levels.
	Component Component

	// CorrelationID is the invocation's correlation ID. Empty is
	// normalized to ZeroCorrelationID; a non-empty value is
	// carried on every record the Logger emits.
	CorrelationID string

	// Writer is the destination for rendered records. If nil,
	// os.Stderr is used.
	Writer io.Writer

	// Sink, if non-nil, overrides Writer. It is the injection
	// seam for tests that want to observe records as structured
	// data. Production code leaves it nil and sets Writer.
	Sink Sink
}

// New returns a Logger configured by opts.
//
// The returned Logger is the default, slog-backed implementation
// (see slog_adapter.go). It is safe for concurrent use.
//
// # Panics
//
// New panics if the resolved Component is ComponentInvalid and
// the resolved Level is not LevelDebug. The check runs against
// the resolved values, not the raw Options fields, so a zero
// Options — which resolves to LevelInfo with
// ComponentInvalid — panics as the Options docstring promises.
//
// The debug-level exception allows tests that deliberately
// exercise the zero component to construct a logger at debug
// level. Such a test must pass Level explicitly (because
// LevelDebug is the zero value and is otherwise indistinguishable
// from unset); see resolve's docstring §"The limitation this
// creates".
//
// A caller that genuinely wants a zero-component logger at a
// non-debug level uses Nop() or constructs the concrete type
// directly. The panic is a deliberate design choice: silently
// emitting records whose component is "invalid" would corrupt
// downstream consumers that validate against the schema.
func New(opts Options) Logger {
	res := resolve(opts)
	if res.component == ComponentInvalid && res.level != LevelDebug {
		panic("logger: New requires a non-zero Component " +
			"(see internal/logger/component.go); " +
			"use Nop() for a no-op logger")
	}
	return newSlogLoggerFromResolved(res)
}

// =============================================================================
// Package-level default logger
// =============================================================================
//
// The default logger is the fallback used by packages that have not
// been wired with an explicit Logger. It is intended for library
// code that wants to log without taking a Logger parameter; the
// CLI wires a per-invocation logger into the command's context
// and downstream code uses FromContext.
//
// The default is a Nop logger, not a live one. This is deliberate:
// a package that logs through the default without the CLI having
// wired a real logger should not write to stderr by accident. A
// caller that wants the default to be live calls SetDefault once
// at process start.
//
// defaultLogger is guarded by defaultMu. The guard is needed
// because SetDefault may race with a concurrent call through the
// package-level functions. The race is rare in practice (SetDefault
// is called at process start), but the guard makes the package
// race-free under any caller.
var (
	defaultMu     sync.RWMutex
	defaultLogger Logger = Nop()
)

// SetDefault replaces the package-level logger.
//
// Call SetDefault once at process start, before any goroutine may
// call the package-level Debug/Info/Warn/Error functions. It is
// safe to call concurrently with those functions, but a call that
// races with SetDefault may observe either the old or the new
// logger.
func SetDefault(l Logger) {
	if l == nil {
		l = Nop()
	}
	defaultMu.Lock()
	defaultLogger = l
	defaultMu.Unlock()
}

// Default returns the package-level logger.
//
// The returned value is never nil. A caller that wants a
// guaranteed Nop logger uses Nop() directly; Default returns
// whatever SetDefault last installed, or Nop() if none.
func Default() Logger {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	return defaultLogger
}

// Debug logs at debug level using the logger bound to ctx.
//
// The lookup order is:
//
//  1. The logger stored in ctx via IntoContext (the
//     per-invocation logger).
//  2. The package-level default installed by SetDefault.
//  3. Nop.
//
// FromContext returns the first of these that is available; the
// fallback chain is FromContext's contract (see its docstring).
//
// # Why the change
//
// Before this change, the package-level functions called
// Default() directly. After the CLI stopped installing a
// process-global logger (WBS 12.5.1), Default() is Nop()
// throughout a normal invocation, and every record emitted by a
// library function that calls logger.Debug(ctx, ...) was
// silently dropped.
//
// Reading the context first routes the record to the
// invocation's logger, which is what the caller intended by
// passing ctx. A caller that passes context.Background() (or a
// context with no logger) still gets the package-level default,
// which is the correct behavior for code emitted outside an
// invocation.
func Debug(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).Debug(msg, args...)
}

// Info logs at info level using the logger bound to ctx.
//
// See Debug for the lookup order and rationale.
func Info(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).Info(msg, args...)
}

// Warn logs at warn level using the logger bound to ctx.
//
// See Debug for the lookup order and rationale.
func Warn(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).Warn(msg, args...)
}

// Error logs at error level using the logger bound to ctx.
//
// See Debug for the lookup order and rationale.
func Error(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).Error(msg, args...)
}
