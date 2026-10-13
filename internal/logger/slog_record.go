package logger

import (
	"context"
	"log/slog"
)

// Reserved attribute keys used by the adapter to carry per-logger
// state through slog's per-call API.
//
// The keys are namespaced with a leading underscore to reduce the
// chance of collision with a user-supplied attr. The reserved-key
// check in record.go does not apply to these keys because they are
// never part of a Record's Attrs; the handler strips them before
// building the Record.
const (
	attrKeyComponent     = "_forge_component"
	attrKeyCorrelationID = "_forge_correlation_id"
)

// forgeHandler is a slog.Handler that translates slog.Record to
// the package's Record and writes the result to a Sink.
//
// # Why a handler
//
// slog's extension point is Handler. Implementing one lets the
// package reuse slog's attribute handling, group support, and
// level filtering while keeping the package's schema as the
// contract. The handler's only job is translation; it does no
// I/O of its own (the Sink does).
//
// # Attribute accumulation
//
// slog's Logger.With calls the handler's WithAttrs method to
// accumulate the caller's attributes. A handler that returns
// itself drops them. forgeHandler therefore stores them in the
// attrs field and prepends them on each Handle call.
//
// The stored attrs are the ones attached to a Logger via With;
// the per-call attrs (the ones passed to Logger.Info, etc.)
// arrive in the slog.Record and are appended after the stored
// ones. The concatenation order matches the caller's mental
// model: With-attrs are "always true for this logger", per-call
// attrs are "true for this call". A later attr with the same key
// shadows an earlier one, matching slog's own behavior.
//
// # Value unwrapping
//
// The adapter wraps each caller-supplied value in a rawValue
// (see slog_adapter.go). The handler unwraps it via
// unwrapRawValue, which recovers the original Go value and
// bypasses slog.Value's canonicalisation. A value the adapter
// did not wrap (for example, one set by a user calling
// slog.Logger directly) is passed through as its canonical form.
//
// # Concurrency
//
// forgeHandler is safe for concurrent use. The sink is
// thread-safe by contract; the level, format, groupPrefix, and
// attrs fields are read-only after construction. WithAttrs and
// WithGroup return copies; the copies share the sink and the
// stored attrs slice.
type forgeHandler struct {
	level  slog.Level
	format Format
	sink   Sink

	// groupPrefix is the current group path, used for nested
	// attributes. slog calls WithGroup to push a group; the
	// handler records the path so that a nested attr is
	// namespaced.
	groupPrefix string

	// attrs are the attributes attached to the handler by
	// slog.Logger.With. They are prepended to every record's
	// attrs, before the per-call attrs in the slog.Record.
	//
	// The slice is shared with the handler returned by
	// WithAttrs; a caller that mutated it would affect both
	// handlers. slog never mutates it after passing it, so the
	// sharing is safe in practice.
	attrs []slog.Attr
}

// newForgeHandler constructs a handler around the resolved options.
//
// # Why the resolved form and not the raw Options
//
// The resolve step (see options.go) fills defaults and unifies the
// two sink representations (Writer and Sink) into a single Sink. By
// the time newForgeHandler is called, every decision the handler
// needs has been made. Passing the raw Options in addition would let
// the handler read a field the resolved form already owns, which
// reopens the "who decides the sink?" question the resolve step
// exists to answer.
//
// # What the handler reads
//
//   - res.level → slog's level filter.
//   - res.format → stored for the handler's own consistency;
//     the Sink's renderer dispatches on the format.
//   - res.sink → the destination for translated records.
//
// The component is not read from the resolved options: it arrives
// per-record, attached by newSlogLogger via slog.Logger.With. The
// handler promotes it to the Record's top-level Component field.
func newForgeHandler(res resolvedOptions) *forgeHandler {
	return &forgeHandler{
		level:  levelToSlog(res.level),
		format: res.format,
		sink:   res.sink,
	}
}

// Enabled implements slog.Handler.
//
// The handler's level is the resolved level; slog calls Enabled
// before Handle, so a filtered record never reaches Handle.
func (h *forgeHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

// Handle implements slog.Handler.
//
// It converts the slog.Record to the package's Record, stripping
// the reserved attributes and promoting them to top-level fields.
// The result is emitted to the Sink.
//
// # Attribute order
//
// The handler's stored attrs (from WithAttrs) are processed
// first, then the record's own attrs. A later attr with the same
// key shadows an earlier one. The correlation ID and component
// are extracted from whichever attr carries the reserved key;
// because the adapter attaches them on construction, they are in
// the stored attrs, not in the record's own attrs.
//
// # Why Handle does not return an error
//
// Handle is called from slogLogger.log, which has no way to act
// on an error. The Sink absorbs write failures (see StderrSink).
// Handle returns nil always.
func (h *forgeHandler) Handle(ctx context.Context, r slog.Record) error {
	rec := Record{
		Ts:            r.Time.UTC(),
		Level:         slogToLevel(r.Level),
		Component:     ComponentInvalid,
		Message:       r.Message,
		CorrelationID: ZeroCorrelationID,
	}

	attrs := make(map[string]any, len(h.attrs)+r.NumAttrs())

	// Stored attrs (from WithAttrs) first.
	for _, a := range h.attrs {
		h.applyAttr(a, &rec, attrs)
	}

	// Per-call attrs (from the slog.Record) next.
	r.Attrs(func(a slog.Attr) bool {
		h.applyAttr(a, &rec, attrs)
		return true
	})

	if len(attrs) > 0 {
		rec.Attrs = attrs
	}

	// Layer 2 of the four-layer redaction model (see
	// internal/logger/redact.go). Every emitted record passes
	// through the pattern middleware before reaching the sink.
	// The call is load-bearing: removing it disables the
	// middleware for every diagnostic the logger emits, and
	// test/security/redaction_test.go's TestRedaction_PatternLayer
	// is the test that catches the removal.
	rec = RedactRecord(rec)

	h.sink.Emit(ctx, rec)
	return nil
}

// applyAttr merges one slog.Attr into the record under construction.
//
// # Why a helper
//
// The stored attrs (from WithAttrs) and the per-call attrs (from
// the slog.Record) are processed with the same rules: reserved
// keys are promoted, other keys are namespaced by the group
// prefix and copied into the attrs map. A single helper ensures
// the two paths cannot drift.
//
// # Value unwrapping
//
// User-supplied values are unwrapped via unwrapRawValue, which
// recovers the original Go value the adapter wrapped in
// rawValue. A value that was not wrapped (a user calling slog
// directly, a future non-adapter caller) is passed through as
// slog.Value.Any().
//
// # Shadowing
//
// If two attrs have the same key, the later one wins because the
// map assignment overwrites. This matches slog's own behavior.
func (h *forgeHandler) applyAttr(a slog.Attr, rec *Record, attrs map[string]any) {
	switch a.Key {
	case attrKeyComponent:
		if c, err := ParseComponent(a.Value.String()); err == nil {
			rec.Component = c
		}
	case attrKeyCorrelationID:
		rec.CorrelationID = a.Value.String()
	default:
		key := a.Key
		if h.groupPrefix != "" {
			key = h.groupPrefix + "." + key
		}
		attrs[key] = unwrapRawValue(a.Value)
	}
}

// WithAttrs implements slog.Handler.
//
// The handler returns a copy with the given attributes appended
// to its stored attrs. Every subsequent Handle call prepends the
// stored attrs to the record's attrs.
//
// # Why a copy and not a mutation
//
// slog's contract is that a Handler is immutable: WithAttrs
// returns a new handler and leaves the receiver unchanged. A
// handler that mutated itself would break slog's group and
// attribute nesting, which relies on the handler chain being a
// persistent data structure.
//
// # Why the empty case returns the receiver
//
// An empty attrs slice carries no information; returning the
// receiver avoids an allocation. slog may call WithAttrs with an
// empty slice in some edge cases (for example, when a Logger is
// cloned without new attributes); the check keeps the common
// path allocation-free.
func (h *forgeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	cp := *h
	combined := make([]slog.Attr, 0, len(h.attrs)+len(attrs))
	combined = append(combined, h.attrs...)
	combined = append(combined, attrs...)
	cp.attrs = combined
	return &cp
}

// WithGroup implements slog.Handler.
//
// The group's name is prefixed to every future attribute key. The
// handler returns a copy with the extended prefix; the copy
// shares the receiver's stored attrs and sink.
//
// # Empty name
//
// An empty name is a no-op in slog's semantics; the handler
// returns the receiver to avoid an allocation. slog never calls
// WithGroup with an empty name in practice, but the guard is
// cheap and matches the "no allocation for no-op" principle the
// package follows elsewhere.
func (h *forgeHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	cp := *h
	if cp.groupPrefix == "" {
		cp.groupPrefix = name
	} else {
		cp.groupPrefix += "." + name
	}
	return &cp
}

// levelToSlog maps a package Level to a slog.Level.
func levelToSlog(l Level) slog.Level {
	switch l {
	case LevelDebug:
		return slog.LevelDebug
	case LevelInfo:
		return slog.LevelInfo
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// slogToLevel maps a slog.Level to a package Level.
func slogToLevel(l slog.Level) Level {
	switch {
	case l < slog.LevelInfo:
		return LevelDebug
	case l < slog.LevelWarn:
		return LevelInfo
	case l < slog.LevelError:
		return LevelWarn
	default:
		return LevelError
	}
}

// unwrapRawValue returns the original Go value the adapter wrapped
// via rawValue, or the slog.Value's own Any() if the value was not
// wrapped.
//
// # Why two type assertions
//
// slog's handling of LogValuer at Any() time varies across Go
// versions. The probe in the test suite (and the Go source at the
// version Forge targets) shows that Value.Any() returns the
// LogValuer itself for a KindLogValuer value, rather than calling
// LogValue() on it. Two shapes are therefore possible when the
// value reaches this helper:
//
//   - slog.Value.Any() returns the rawValue wrapper (this Go
//     version). The helper recovers the original from the
//     wrapper's v field.
//   - slog.Value.Any() returns the resolved rawValueEnvelope
//     (a future Go version that resolves LogValuer at Any()
//     time). The helper recovers the original from the
//     envelope's v field.
//
// Both shapes are handled; the type switch makes the two cases
// explicit and lets a future version with a third shape be added
// without silently leaking the wrapper.
//
// # The fallback path
//
// A value that is neither wrapper is returned as slog.Value.Any().
// This covers a user who constructs slog.Attr values directly, and
// any future non-adapter caller.
func unwrapRawValue(v slog.Value) any {
	switch x := v.Any().(type) {
	case rawValueEnvelope:
		return x.v
	case rawValue:
		return x.v
	default:
		return x
	}
}
