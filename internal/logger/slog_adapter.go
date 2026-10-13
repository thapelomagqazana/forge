// This file implements the default Logger: an adapter over
// log/slog.
//
// # Direction of the bridge
//
// The package *uses* slog; it does not adapt to it. A caller
// writes against the package's Logger interface; the
// implementation forwards to a slog.Logger whose Handler is a
// forgeHandler. The handler translates slog.Record to the
// package's Record and writes it to a Sink.
//
//	caller
//	  │  Logger.Info(msg, args...)
//	  ▼
//	slogLogger
//	  │  argsToSlogAttrs(args) -> []slog.Attr
//	  │  sl.LogAttrs(ctx, level, msg, attrs...)
//	  ▼
//	slog.Logger   (constructed with the reserved attrs already attached)
//	  │  handler.Handle(ctx, slog.Record)
//	  ▼
//	forgeHandler
//	  │  Sink.Emit(ctx, Record)
//	  ▼
//	StderrSink  →  writeHuman / writeJSON  →  stderr
//
// # Why the adapter parses the caller's args itself
//
// slog.Logger.Log takes a variadic []any and applies slog's own
// attribute-parsing rules. Those rules differ from the package's:
//
//   - slog treats an unpaired trailing key as a bare string with a
//     "!BADKEY" sentinel; the package treats it as a key with a
//     nil value.
//   - slog's handling of non-string keys and mixed-type values
//     differs from the package's.
//
// The adapter therefore parses args with the package's rules
// (argsToSlogAttrs) and passes the result to LogAttrs, which takes
// pre-parsed []slog.Attr. This decouples the package's attribute
// contract from slog's parsing.
//
// # Why values are wrapped in rawValue
//
// slog.Any canonicalises the value it carries: an int is stored as
// int64, a uint as uint64, and slog.Value.Any() returns the
// canonical type, not the original. The package's contract is that
// the caller's value reaches the Record.Attrs map unchanged. The
// adapter wraps each value in a rawValue, which slog carries
// opaquely; the handler unwraps it via unwrapRawValue. See
// rawValue for the full rationale.
//
// # Why the reserved attrs are attached at construction
//
// The component and correlation ID are per-logger state. Attaching
// them on every log call (by appending to the caller's args) would
// require the adapter to inject them into the same slice slog
// parses, which produces misalignment when the caller's args have
// an unexpected shape. Attaching them once, at construction, via
// slog.Logger.With, lets slog's own attribute-accumulation
// machinery carry them without interfering with the caller's args.
//
// # What this file does not do
//
// This file does not import internal/cli or any domain package.
// The package boundary (see the package docstring) forbids it.
// The Logger interface is the extension point; the CLI wires an
// implementation into the command's context (see
// internal/cli/hooks.go).
package logger

import (
	"context"
	"fmt"
	"log/slog"
)

// =============================================================================
// slogLogger: the default implementation
// =============================================================================

// slogLogger is the default Logger implementation, backed by
// log/slog.
//
// # Construction
//
// The type is constructed by newSlogLogger, which is called only
// from New (see logger.go). The constructor's signature is
// unexported: callers use New(Options{...}).
//
// # Reserved attributes
//
// The underlying slog.Logger is constructed with the component
// and correlation ID already attached via With. Every record it
// emits carries them; the adapter does not inject them on each
// call.
//
// # Concurrency
//
// slogLogger is safe for concurrent use. Its only state is an
// immutable *slog.Logger and two scalars (component,
// correlationID). The underlying slog.Logger is safe for
// concurrent use by the slog package's contract; the scalars are
// read-only after construction.
//
// # With semantics
//
// A call to With returns a copy of the receiver with a new
// *slog.Logger (produced by slog's With). The copy shares the
// original slog.Logger's handler chain; the component and
// correlation ID are carried through unchanged, because they are
// properties of the logger, not of its per-call attributes.
type slogLogger struct {
	// sl is the underlying slog.Logger. Its Handler is a
	// forgeHandler (see handler.go), which writes records to a
	// Sink. The logger was constructed with the reserved
	// component and correlation-ID attrs already attached.
	sl *slog.Logger

	// component is the subsystem tag carried on every record
	// this logger emits. It is set at construction and read-only
	// for the logger's lifetime.
	component Component

	// correlationID is the invocation's correlation ID. It is
	// set at construction (from Options.CorrelationID) and
	// read-only for the logger's lifetime.
	correlationID string
}

// withCorrelationID returns a copy of s with the given
// correlation ID.
//
// The method is used by the correlation-aware constructors (see
// logger.go and the tests) to bind an ID after construction. The
// implementation builds a new slog.Logger with the new ID
// attached, replacing the old one. Because slog accumulates
// attributes with last-write-wins semantics, the new ID shadows
// the old one in the handler's stored attrs.
//
// A copy is returned rather than mutating in place because the
// logger may be shared across goroutines; mutating the
// correlationID field would race with concurrent reads.
func (s *slogLogger) withCorrelationID(id string) *slogLogger {
	cp := *s
	cp.correlationID = id
	cp.sl = s.sl.With(slog.String(attrKeyCorrelationID, id))
	return &cp
}

// Debug implements Logger.
func (s *slogLogger) Debug(msg string, args ...any) {
	s.log(context.Background(), slog.LevelDebug, msg, args)
}

// Info implements Logger.
func (s *slogLogger) Info(msg string, args ...any) {
	s.log(context.Background(), slog.LevelInfo, msg, args)
}

// Warn implements Logger.
func (s *slogLogger) Warn(msg string, args ...any) {
	s.log(context.Background(), slog.LevelWarn, msg, args)
}

// Error implements Logger.
func (s *slogLogger) Error(msg string, args ...any) {
	s.log(context.Background(), slog.LevelError, msg, args)
}

// With implements Logger.
//
// The returned logger shares the receiver's slog.Logger with a
// child created by slog's With. The pre-computed component and
// correlation ID are carried through unchanged: they are
// properties of the logger, not of its attributes.
//
// A With call with no args returns the receiver, matching the
// interface's "encouraged" behavior. The check avoids the cost of
// slog's With (which allocates a child logger) when there is
// nothing to attach.
//
// # Caller args are parsed with the package's rules
//
// slog's With takes []any and applies its own parsing. That is
// not what the package's With promises; the package promises the
// same key-value semantics as the level methods. The adapter
// therefore converts the caller's args with argsToSlogAttrs and
// passes the resulting []slog.Attr (as []any) to slog's With,
// which recognizes slog.Attr values.
func (s *slogLogger) With(args ...any) Logger {
	if len(args) == 0 {
		return s
	}
	attrs := argsToSlogAttrs(args)
	if len(attrs) == 0 {
		return s
	}
	// slog.Logger.With accepts ...any and recognizes slog.Attr
	// elements. The conversion from []slog.Attr to []any is
	// required because Go has no direct way to spread a typed
	// slice into an untyped variadic.
	anyAttrs := make([]any, len(attrs))
	for i, a := range attrs {
		anyAttrs[i] = a
	}
	cp := *s
	cp.sl = s.sl.With(anyAttrs...)
	return &cp
}

// log is the common emission path.
//
// The function parses the caller's args with the package's rules
// and passes the result to slog.Logger.LogAttrs, which takes
// pre-parsed attributes. The adapter does not rely on slog's own
// args parsing; the two contracts differ.
//
// # Why the enabled check is done here
//
// slog.Logger.LogAttrs already checks Enabled before calling
// Handler.Handle. The explicit check in s.log is redundant for
// correctness but avoids the cost of building the attrs slice
// when the record will be filtered. The cost is one method call;
// the benefit is not allocating a slice for a filtered record,
// which matters on hot paths.
func (s *slogLogger) log(ctx context.Context, level slog.Level, msg string, args []any) {
	if !s.sl.Enabled(ctx, level) {
		return
	}
	attrs := argsToSlogAttrs(args)
	s.sl.LogAttrs(ctx, level, msg, attrs...)
}

// =============================================================================
// argsToSlogAttrs: the package's attribute contract
// =============================================================================

// argsToSlogAttrs converts the package's variadic key-value pairs
// into slog attributes under the package's rules.
//
// # Rules
//
//   - Pairs are (key, value). A non-string key is stringified
//     with fmt.Sprint.
//   - An odd trailing key maps to a nil value. The nil is
//     carried as a rawValue whose v field is nil; the handler's
//     Attrs map receives nil for that key.
//   - A key that collides with a reserved envelope field (see
//     ReservedAttrKeys in record.go) is namespaced with an
//     "attr_" prefix. The namespacing matches the behavior of
//     the old kvToMap helper; it prevents a user attr from
//     shadowing the record's own field of the same name in the
//     JSON rendering.
//   - The empty input returns nil, so the record's Attrs field
//     is nil (not an empty map) and the JSON rendering omits it.
//
// # Why values are wrapped in rawValue
//
// slog.Any canonicalises the value it carries: an int is stored
// as int64, a uint as uint64, and slog.Value.Any() returns the
// canonical type, not the original. The package's contract is
// that the caller's value reaches the Record.Attrs map unchanged.
// Wrapping in rawValue preserves the original; the handler
// unwraps it via unwrapRawValue. See rawValue.
//
// # Why not slog's own parsing
//
// slog.Logger.Log parses args with its own rules: unpaired keys
// become bare strings with a "!BADKEY" sentinel; non-string
// values in some positions are handled differently from the
// package's rules. The package promises a specific contract to
// its callers; the adapter must honor it. Parsing here and
// passing to LogAttrs is the mechanism.
func argsToSlogAttrs(args []any) []slog.Attr {
	if len(args) == 0 {
		return nil
	}
	out := make([]slog.Attr, 0, (len(args)+1)/2)
	for i := 0; i < len(args); i += 2 {
		key := fmt.Sprint(args[i])
		if ReservedAttrKeys[key] {
			key = "attr_" + key
		}
		var val any
		if i+1 < len(args) {
			val = args[i+1]
		}
		out = append(out, slog.Any(key, rawValue{v: val}))
	}
	return out
}

// =============================================================================
// rawValue: preserve the caller's value through slog
// =============================================================================

// rawValue carries a caller-supplied value through slog's attribute
// machinery without canonicalisation.
//
// # Why this type exists
//
// slog.Any(key, v) wraps v in a slog.Value. The wrapping is not
// identity: an int is stored as int64, a uint as uint64, and
// slog.Value.Any() returns the canonical type, not the original.
// The package's contract is that the caller's value reaches the
// Record.Attrs map unchanged. This type bridges the two: the
// adapter wraps each value in a rawValue, slog carries it as an
// opaque slog.Any, and the handler unwraps it to recover the
// original.
//
// # The envelope is unforgeable
//
// rawValueEnvelope is unexported, so a user cannot construct a
// value of the type. The handler's unwrap is a type assertion; a
// user-supplied value that happens to be a map or a string cannot
// be misidentified as an envelope.
//
// # Why a LogValuer
//
// slog.LogValuer is slog's extension point for values that want
// to control their own representation. By implementing
// LogValue, rawValue returns a slog.Value that wraps the
// original any; slog carries the wrapper opaquely, and the
// handler recovers the original.
type rawValue struct {
	v any
}

// rawValueEnvelope is the private struct rawValue carries through
// slog. It is unexported; the handler's type assertion on it is
// unforgeable from outside the package.
type rawValueEnvelope struct {
	v any
}

// LogValue implements slog.LogValuer.
//
// slog calls LogValue to obtain the slog.Value it will store on
// the Attr. The implementation wraps the envelope in a
// slog.Value via slog.AnyValue; the handler's unwrapRawValue
// extracts the envelope's v field.
//
// # Why AnyValue and not Any
//
// slog.Any(key, v) returns a slog.Attr (a key-value pair), not a
// slog.Value. LogValue's contract is to return a slog.Value, so
// the helper must be slog.AnyValue(v), which returns a Value
// directly. The key the adapter cares about lives on the Attr
// the caller supplied to slog.Any in argsToSlogAttrs, not here.
//
// # Why the Value's Kind is KindAny
//
// slog.AnyValue wraps any Go value that does not match a more
// specific slog.Kind (string, int, bool, time.Time, etc.) in a
// KindAny value. The envelope struct is not a recognised kind,
// so it is stored opaquely. The handler's type assertion on
// rawValueEnvelope recovers the original.
func (r rawValue) LogValue() slog.Value {
	return slog.AnyValue(rawValueEnvelope{v: r.v})
}

// newSlogLogger constructs a slogLogger from opts. It resolves
// the options once and delegates to newSlogLoggerFromResolved.
//
// The wrapper exists for the package's own tests, which construct
// a logger with a zero Component at debug level without going
// through New's panic check. Production code uses New.
func newSlogLogger(opts Options) *slogLogger {
	return newSlogLoggerFromResolved(resolve(opts))
}

// newSlogLoggerFromResolved constructs a slogLogger from the
// already-resolved options. It is used by New (which resolves
// once, checks the Component/Level invariant, and delegates) and
// by newSlogLogger (which is the test-facing wrapper).
func newSlogLoggerFromResolved(res resolvedOptions) *slogLogger {
	handler := newForgeHandler(res)
	base := slog.New(handler)
	withReserved := base.With(
		slog.String(attrKeyComponent, res.component.String()),
		slog.String(attrKeyCorrelationID, res.correlationID),
	)
	return &slogLogger{
		sl:            withReserved,
		component:     res.component,
		correlationID: res.correlationID,
	}
}

// =============================================================================
// Compile-time interface satisfaction
// =============================================================================

// slogLogger implements Logger. The assertion is here, next to
// the type, rather than in logger.go, so that a change to the
// interface's method set fails in the file that owns the
// implementation.
var _ Logger = (*slogLogger)(nil)
