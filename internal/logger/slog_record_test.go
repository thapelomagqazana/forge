// This file tests the forgeHandler in isolation.
//
// # Why these tests are separate from slog_adapter_test.go
//
// slog_adapter_test.go exercises the adapter end to end: it calls
// Logger.Info and asserts on what reaches the sink. Those tests
// verify the wiring (call → slog → handler → sink) but they
// obscure which layer produced a record's fields. A failure in
// the adapter's level filter, in the handler's translation, or in
// the sink's rendering all look the same from the outside.
//
// This file tests the handler directly: it constructs a
// forgeHandler, calls Handle with a hand-built slog.Record, and
// asserts on the package Record the handler produced. A failure
// here names the handler as the culprit; a failure in
// slog_adapter_test.go names the wiring.
//
// # What the handler owns
//
// The handler is responsible for four things, each with a
// dedicated section below:
//
//   - The field mapping (slog → package).
//   - The stripping and promotion of the two reserved keys
//     (attrKeyComponent, attrKeyCorrelationID).
//   - The group prefix applied to user attrs.
//   - The level filter (Enabled) that short-circuits Handle.
package logger

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

// =============================================================================
// Test helper
// =============================================================================

// newTestHandler returns a forgeHandler wired to the given sink at
// the given threshold. The format field is set to FormatJSON by
// default; the handler does not read it (the sink does), so the
// value is inert and does not affect any assertion in this file.
//
// The helper exists so a change to the handler's construction
// touches one line, not every test.
func newTestHandler(sink Sink, level Level) *forgeHandler {
	return newForgeHandler(resolvedOptions{
		level:     level,
		format:    FormatJSON,
		component: ComponentInvalid, // not read by the handler
		sink:      sink,
	})
}

// makeRecord constructs a slog.Record for the tests. The time is
// fixed so the assertion on Record.Ts is deterministic.
func makeRecord(level slog.Level, msg string) slog.Record {
	return slog.NewRecord(
		time.Date(2026, 10, 13, 10, 0, 0, 123456789, time.UTC),
		level,
		msg,
		0,
	)
}

// =============================================================================
// Field mapping (slog → package Record)
// =============================================================================

// TestForgeHandler_MapsCoreFields asserts the direct mapping:
// time, level, and message travel from the slog.Record to the
// package Record unchanged (modulo the level conversion).
func TestForgeHandler_MapsCoreFields(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	h := newTestHandler(sink, LevelDebug)

	r := makeRecord(slog.LevelWarn, "hello")
	if err := h.Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	got := sink.snapshot()
	if len(got) != 1 {
		t.Fatalf("got %d records; want 1", len(got))
	}
	rec := got[0]

	if !rec.Ts.Equal(r.Time) {
		t.Errorf("Ts = %v; want %v", rec.Ts, r.Time)
	}
	if rec.Ts.Location() != time.UTC {
		t.Errorf("Ts location = %v; want UTC", rec.Ts.Location())
	}
	if rec.Level != LevelWarn {
		t.Errorf("Level = %v; want Warn", rec.Level)
	}
	if rec.Message != "hello" {
		t.Errorf("Message = %q; want hello", rec.Message)
	}
}

// TestForgeHandler_LevelMapping asserts the slog.Level → package
// Level mapping through the handler. The direct test of
// slogToLevel lives in slog_adapter_test.go; this test verifies
// that the handler applies the mapping to the incoming record.
func TestForgeHandler_LevelMapping(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   slog.Level
		want Level
	}{
		{slog.LevelDebug, LevelDebug},
		{slog.LevelInfo, LevelInfo},
		{slog.LevelWarn, LevelWarn},
		{slog.LevelError, LevelError},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.in.String(), func(t *testing.T) {
			t.Parallel()

			sink := &captureSink{}
			h := newTestHandler(sink, LevelDebug)

			r := makeRecord(tc.in, "x")
			if err := h.Handle(context.Background(), r); err != nil {
				t.Fatalf("Handle: %v", err)
			}
			got := sink.snapshot()
			if len(got) != 1 {
				t.Fatalf("got %d records; want 1", len(got))
			}
			if got[0].Level != tc.want {
				t.Errorf("Level = %v; want %v", got[0].Level, tc.want)
			}
		})
	}
}

// TestForgeHandler_UserAttrsAreCopied asserts that a user-supplied
// attribute is copied to the Record's Attrs under its own key.
// The handler does not rename user keys; only the reserved keys
// are transformed.
func TestForgeHandler_UserAttrsAreCopied(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	h := newTestHandler(sink, LevelDebug)

	r := makeRecord(slog.LevelInfo, "x")
	r.AddAttrs(
		slog.String("path", "/tmp/out.go"),
		slog.Int("line", 42),
		slog.Bool("dirty", true),
	)
	if err := h.Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	got := sink.snapshot()
	if len(got) != 1 {
		t.Fatalf("got %d records; want 1", len(got))
	}
	attrs := got[0].Attrs
	if attrs["path"] != "/tmp/out.go" {
		t.Errorf("attr path = %v; want /tmp/out.go", attrs["path"])
	}
	if attrs["line"] != int64(42) {
		t.Errorf("attr line = %v (%T); want int64(42)", attrs["line"], attrs["line"])
	}
	if attrs["dirty"] != true {
		t.Errorf("attr dirty = %v; want true", attrs["dirty"])
	}
}

// TestForgeHandler_NoUserAttrsProducesNilMap asserts that a record
// with no user attrs has a nil Attrs field, not an empty map. The
// distinction matters for the JSON renderer, which omits the
// field entirely when Attrs is nil.
func TestForgeHandler_NoUserAttrsProducesNilMap(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	h := newTestHandler(sink, LevelDebug)

	r := makeRecord(slog.LevelInfo, "x")
	if err := h.Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	got := sink.snapshot()
	if len(got) != 1 {
		t.Fatalf("got %d records; want 1", len(got))
	}
	if got[0].Attrs != nil {
		t.Fatalf("Attrs = %v; want nil", got[0].Attrs)
	}
}

// =============================================================================
// Reserved keys: component and correlation ID
// =============================================================================

// TestForgeHandler_PromotesComponent asserts that a reserved
// component attr is promoted to the Record's top-level Component
// field and is not copied into Attrs.
func TestForgeHandler_PromotesComponent(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	h := newTestHandler(sink, LevelDebug)

	r := makeRecord(slog.LevelInfo, "x")
	r.AddAttrs(slog.String(attrKeyComponent, "filesystem"))
	if err := h.Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	got := sink.snapshot()
	if len(got) != 1 {
		t.Fatalf("got %d records; want 1", len(got))
	}
	if got[0].Component != ComponentFilesystem {
		t.Errorf("Component = %v; want Filesystem", got[0].Component)
	}
	if _, leaked := got[0].Attrs[attrKeyComponent]; leaked {
		t.Errorf("reserved key leaked into Attrs: %v", got[0].Attrs)
	}
}

// TestForgeHandler_PromotesCorrelationID asserts that a reserved
// correlation-ID attr is promoted to the Record's top-level field
// and is not copied into Attrs.
func TestForgeHandler_PromotesCorrelationID(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	h := newTestHandler(sink, LevelDebug)

	id := NewCorrelationID()
	r := makeRecord(slog.LevelInfo, "x")
	r.AddAttrs(slog.String(attrKeyCorrelationID, id))
	if err := h.Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	got := sink.snapshot()
	if len(got) != 1 {
		t.Fatalf("got %d records; want 1", len(got))
	}
	if got[0].CorrelationID != id {
		t.Errorf("CorrelationID = %q; want %q", got[0].CorrelationID, id)
	}
	if _, leaked := got[0].Attrs[attrKeyCorrelationID]; leaked {
		t.Errorf("reserved key leaked into Attrs: %v", got[0].Attrs)
	}
}

// TestForgeHandler_BothReservedKeysPromoted asserts that the two
// reserved attrs are promoted independently, in either order.
func TestForgeHandler_BothReservedKeysPromoted(t *testing.T) {
	t.Parallel()

	orders := []struct {
		name string
		keys []slog.Attr
	}{
		{
			name: "component first",
			keys: []slog.Attr{
				slog.String(attrKeyComponent, "policy"),
				slog.String(attrKeyCorrelationID, "id-xyz"),
			},
		},
		{
			name: "correlation first",
			keys: []slog.Attr{
				slog.String(attrKeyCorrelationID, "id-xyz"),
				slog.String(attrKeyComponent, "policy"),
			},
		},
	}
	for _, tc := range orders {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sink := &captureSink{}
			h := newTestHandler(sink, LevelDebug)

			r := makeRecord(slog.LevelInfo, "x")
			r.AddAttrs(tc.keys...)
			if err := h.Handle(context.Background(), r); err != nil {
				t.Fatalf("Handle: %v", err)
			}

			got := sink.snapshot()
			if len(got) != 1 {
				t.Fatalf("got %d records; want 1", len(got))
			}
			if got[0].Component != ComponentPolicy {
				t.Errorf("Component = %v; want Policy", got[0].Component)
			}
			if got[0].CorrelationID != "id-xyz" {
				t.Errorf("CorrelationID = %q; want id-xyz", got[0].CorrelationID)
			}
		})
	}
}

// TestForgeHandler_DefaultsForMissingReservedKeys asserts the two
// defaults: a record with no component attr carries
// ComponentInvalid; a record with no correlation-ID attr carries
// ZeroCorrelationID. The defaults are the sentinels that tell a
// consumer "this field was not provided".
func TestForgeHandler_DefaultsForMissingReservedKeys(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	h := newTestHandler(sink, LevelDebug)

	r := makeRecord(slog.LevelInfo, "x")
	if err := h.Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	got := sink.snapshot()
	if len(got) != 1 {
		t.Fatalf("got %d records; want 1", len(got))
	}
	if got[0].Component != ComponentInvalid {
		t.Errorf("Component = %v; want Invalid", got[0].Component)
	}
	if got[0].CorrelationID != ZeroCorrelationID {
		t.Errorf("CorrelationID = %q; want zero", got[0].CorrelationID)
	}
}

// --- Negative: invalid component value is ignored ---

// TestForgeHandler_InvalidComponentIsIgnored asserts that an
// unknown component string does not corrupt the Record. The
// handler parses the value; a parse failure leaves the
// ComponentInvalid default in place. The alternative — panicking
// or assigning the raw string — would corrupt a schema-validating
// consumer.
func TestForgeHandler_InvalidComponentIsIgnored(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	h := newTestHandler(sink, LevelDebug)

	r := makeRecord(slog.LevelInfo, "x")
	r.AddAttrs(slog.String(attrKeyComponent, "not-a-real-component"))
	if err := h.Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	got := sink.snapshot()
	if len(got) != 1 {
		t.Fatalf("got %d records; want 1", len(got))
	}
	if got[0].Component != ComponentInvalid {
		t.Errorf("Component = %v; want Invalid (parse failed)",
			got[0].Component)
	}
}

// =============================================================================
// Groups
// =============================================================================

// TestForgeHandler_WithGroupPrefixesUserAttrs asserts that a group
// name is prefixed to every user attr key. The dotted form is the
// convention the human renderer uses; the JSON renderer sees the
// same key in Attrs.
func TestForgeHandler_WithGroupPrefixesUserAttrs(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	h := newTestHandler(sink, LevelDebug)
	grouped := h.WithGroup("db")

	r := makeRecord(slog.LevelInfo, "x")
	r.AddAttrs(
		slog.String("query", "SELECT 1"),
		slog.Int("rows", 10),
	)
	if err := grouped.Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	got := sink.snapshot()
	if len(got) != 1 {
		t.Fatalf("got %d records; want 1", len(got))
	}
	if got[0].Attrs["db.query"] != "SELECT 1" {
		t.Errorf("attr db.query = %v; want SELECT 1", got[0].Attrs)
	}
	if got[0].Attrs["db.rows"] != int64(10) {
		t.Errorf("attr db.rows = %v; want int64(10)", got[0].Attrs["db.rows"])
	}
}

// TestForgeHandler_WithGroupDoesNotPrefixReservedKeys asserts that
// the group prefix applies only to user attrs. The reserved keys
// are extracted before the prefix is applied, so a group named
// "db" does not turn `_forge_component` into `db._forge_component`.
func TestForgeHandler_WithGroupDoesNotPrefixReservedKeys(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	h := newTestHandler(sink, LevelDebug)
	grouped := h.WithGroup("db")

	r := makeRecord(slog.LevelInfo, "x")
	r.AddAttrs(
		slog.String(attrKeyComponent, "policy"),
		slog.String(attrKeyCorrelationID, "id-abc"),
	)
	if err := grouped.Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	got := sink.snapshot()
	if len(got) != 1 {
		t.Fatalf("got %d records; want 1", len(got))
	}
	if got[0].Component != ComponentPolicy {
		t.Errorf("Component = %v; want Policy", got[0].Component)
	}
	if got[0].CorrelationID != "id-abc" {
		t.Errorf("CorrelationID = %q; want id-abc", got[0].CorrelationID)
	}
	if len(got[0].Attrs) != 0 {
		t.Errorf("Attrs = %v; want empty (reserved keys stripped)",
			got[0].Attrs)
	}
}

// TestForgeHandler_NestedGroups asserts that nested WithGroup calls
// concatenate the prefixes with dots. A group "db" followed by a
// group "tx" produces the prefix "db.tx".
func TestForgeHandler_NestedGroups(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	h := newTestHandler(sink, LevelDebug)
	grouped := h.WithGroup("db").WithGroup("tx")

	r := makeRecord(slog.LevelInfo, "x")
	r.AddAttrs(slog.String("id", "tx-1"))
	if err := grouped.Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	got := sink.snapshot()
	if len(got) != 1 {
		t.Fatalf("got %d records; want 1", len(got))
	}
	if got[0].Attrs["db.tx.id"] != "tx-1" {
		t.Errorf("attr db.tx.id = %v; want tx-1", got[0].Attrs)
	}
}

// TestForgeHandler_WithGroupReturnsNewHandler asserts that
// WithGroup returns a distinct handler and does not mutate the
// receiver. A caller that reuses the parent handler must see the
// parent's behavior, unaffected by the child's group.
func TestForgeHandler_WithGroupReturnsNewHandler(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	parent := newTestHandler(sink, LevelDebug)
	child := parent.WithGroup("db")

	if child == slog.Handler(parent) {
		t.Fatal("WithGroup returned the receiver")
	}

	// The parent is unaffected.
	r := makeRecord(slog.LevelInfo, "x")
	r.AddAttrs(slog.String("k", "v"))
	if err := parent.Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle on parent: %v", err)
	}
	got := sink.snapshot()
	if _, prefixed := got[0].Attrs["db.k"]; prefixed {
		t.Errorf("parent handler gained the child's group: %v", got[0].Attrs)
	}
	if got[0].Attrs["k"] != "v" {
		t.Errorf("parent attr k = %v; want v", got[0].Attrs["k"])
	}
}

// =============================================================================
// Level filter
// =============================================================================

// TestForgeHandler_Enabled asserts the level filter. The handler's
// Enabled method is what slog calls before Handle; the test
// verifies that the filter correctly compares against the
// configured threshold.
func TestForgeHandler_Enabled(t *testing.T) {
	t.Parallel()

	cases := []struct {
		threshold slog.Level
		probe     slog.Level
		want      bool
	}{
		{slog.LevelDebug, slog.LevelDebug, true},
		{slog.LevelDebug, slog.LevelError, true},
		{slog.LevelInfo, slog.LevelDebug, false},
		{slog.LevelInfo, slog.LevelInfo, true},
		{slog.LevelWarn, slog.LevelWarn, true},
		{slog.LevelWarn, slog.LevelInfo, false},
		{slog.LevelError, slog.LevelError, true},
		{slog.LevelError, slog.LevelWarn, false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.threshold.String()+"/"+tc.probe.String(), func(t *testing.T) {
			t.Parallel()
			sink := &captureSink{}
			h := newTestHandler(sink, slogToLevel(tc.threshold))
			if got := h.Enabled(context.Background(), tc.probe); got != tc.want {
				t.Errorf("Enabled(%v) at threshold %v = %v; want %v",
					tc.probe, tc.threshold, got, tc.want)
			}
		})
	}
}

// TestForgeHandler_EnabledIgnoresContext asserts that Enabled does
// not consult the context. The behavior is required by slog's
// contract (a context may be nil or carry values the handler must
// not interpret), and the test pins it so a future change does not
// start reading the context in Enabled.
func TestForgeHandler_EnabledIgnoresContext(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	h := newTestHandler(sink, LevelInfo)

	// nil context
	if !h.Enabled(nil, slog.LevelInfo) {
		t.Error("Enabled(nil, Info) = false; want true")
	}
	// a context with an unrelated value
	type ctxKey struct{}
	ctx := context.WithValue(context.Background(), ctxKey{}, "value")
	if !h.Enabled(ctx, slog.LevelInfo) {
		t.Error("Enabled(ctx-with-value, Info) = false; want true")
	}
}

// =============================================================================
// Handle: error return
// =============================================================================

// TestForgeHandler_HandleAlwaysReturnsNil asserts the documented
// contract: Handle never returns an error. The Sink absorbs write
// failures, and the handler has nothing else to report. A future
// change that returned an error would require changing the
// adapter's log method, which currently discards Handle's return.
func TestForgeHandler_HandleAlwaysReturnsNil(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	h := newTestHandler(sink, LevelDebug)

	r := makeRecord(slog.LevelInfo, "x")
	if err := h.Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle returned %v; want nil", err)
	}
}

// =============================================================================
// Nil-context handling
// =============================================================================

// TestForgeHandler_NilContext asserts that Handle and Enabled
// accept a nil context without panicking. The handler does not
// consult the context (the correlation ID arrives as an attr), so
// nil is safe.
func TestForgeHandler_NilContext(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	h := newTestHandler(sink, LevelDebug)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Handle(nil, ...) panicked: %v", r)
		}
	}()
	r := makeRecord(slog.LevelInfo, "x")
	if err := h.Handle(nil, r); err != nil {
		t.Fatalf("Handle returned %v; want nil", err)
	}
	if sink.len() != 1 {
		t.Fatalf("sink received %d records; want 1", sink.len())
	}
}

// =============================================================================
// Compile-time interface satisfaction
// =============================================================================

// TestForgeHandler_ImplementsSlogHandler pins the compile-time
// assertion that forgeHandler satisfies slog.Handler. The
// assertion is at the point of use; the test makes it visible.
func TestForgeHandler_ImplementsSlogHandler(t *testing.T) {
	t.Parallel()

	var _ slog.Handler = (*forgeHandler)(nil)
}
