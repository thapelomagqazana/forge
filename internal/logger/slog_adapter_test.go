package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// The adapter is the default implementation
// =============================================================================

// TestSlogLogger_SatisfiesLogger pins the compile-time assertion
// that the concrete type implements the interface. The assertion
// itself lives in slog_adapter.go; this test fails to build if a
// future refactor removes it.
func TestSlogLogger_SatisfiesLogger(t *testing.T) {
	t.Parallel()

	var _ Logger = (*slogLogger)(nil)
}

// =============================================================================
// Level mapping (both directions)
// =============================================================================

// TestSlogToLevel_AllCanonicalLevels asserts the slog → package
// mapping for the four canonical slog levels.
func TestSlogToLevel_AllCanonicalLevels(t *testing.T) {
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
		if got := slogToLevel(tc.in); got != tc.want {
			t.Errorf("slogToLevel(%v) = %v; want %v", tc.in, got, tc.want)
		}
	}
}

// TestSlogToLevel_CustomLevels asserts the collision rule: slog
// supports custom levels between the canonical ones, and the
// mapping collapses them to the nearest canonical level.
func TestSlogToLevel_CustomLevels(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   slog.Level
		want Level
	}{
		// Below Info → Debug.
		{"below Info", slog.LevelInfo - 1, LevelDebug},
		{"well below Info", slog.LevelDebug - 4, LevelDebug},
		// [Info, Warn) → Info.
		{"just above Info", slog.LevelInfo + 1, LevelInfo},
		{"just below Warn", slog.LevelWarn - 1, LevelInfo},
		// [Warn, Error) → Warn.
		{"just above Warn", slog.LevelWarn + 1, LevelWarn},
		{"just below Error", slog.LevelError - 1, LevelWarn},
		// >= Error → Error.
		{"just above Error", slog.LevelError + 1, LevelError},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := slogToLevel(tc.in); got != tc.want {
				t.Errorf("slogToLevel(%v) = %v; want %v",
					tc.in, got, tc.want)
			}
		})
	}
}

// TestLevelToSlog_AllPackageLevels asserts the package → slog
// mapping for the four canonical levels.
func TestLevelToSlog_AllPackageLevels(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   Level
		want slog.Level
	}{
		{LevelDebug, slog.LevelDebug},
		{LevelInfo, slog.LevelInfo},
		{LevelWarn, slog.LevelWarn},
		{LevelError, slog.LevelError},
	}
	for _, tc := range cases {
		if got := levelToSlog(tc.in); got != tc.want {
			t.Errorf("levelToSlog(%v) = %v; want %v", tc.in, got, tc.want)
		}
	}
}

// TestLevelMapping_RoundTrip asserts that the two mappings are
// inverses for the four canonical levels. A round-trip through
// slog must not change the level.
func TestLevelMapping_RoundTrip(t *testing.T) {
	t.Parallel()

	for _, l := range []Level{LevelDebug, LevelInfo, LevelWarn, LevelError} {
		l := l
		t.Run(l.String(), func(t *testing.T) {
			t.Parallel()
			got := slogToLevel(levelToSlog(l))
			if got != l {
				t.Errorf("round trip: slogToLevel(levelToSlog(%v)) = %v",
					l, got)
			}
		})
	}
}

// =============================================================================
// Adapter emits through the Logger interface
// =============================================================================

// TestSlogLogger_LevelFilteringAtSlogLayer asserts that the adapter
// honours the configured threshold. The filtering happens inside
// slog (Enabled), not in the package's loop.
func TestSlogLogger_LevelFilteringAtSlogLayer(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	l := New(Options{
		Level:     LevelWarn,
		Format:    FormatJSON,
		Component: ComponentCLI,
		Writer:    &buf,
	})

	l.Debug("d")
	l.Info("i")
	l.Warn("w")
	l.Error("e")

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines; want 2: %q", len(lines), buf.String())
	}
	for _, line := range lines {
		var doc map[string]any
		if err := json.Unmarshal([]byte(line), &doc); err != nil {
			t.Fatalf("decode %q: %v", line, err)
		}
		lvl, _ := doc["level"].(string)
		if lvl != "warn" && lvl != "error" {
			t.Errorf("emitted level %q; want warn or error", lvl)
		}
	}
}

// =============================================================================
// Component and correlation ID travel as reserved attributes
// =============================================================================

// TestSlogLogger_ComponentIsCarriedThroughSlog asserts that the
// adapter attaches the component as a reserved attribute and the
// handler promotes it to the Record's top-level Component field.
func TestSlogLogger_ComponentIsCarriedThroughSlog(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	l := New(Options{
		Level:     LevelInfo,
		Format:    FormatJSON,
		Component: ComponentBlueprint,
		Writer:    &buf,
	})
	l.Info("x")

	var doc map[string]any
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := doc["component"]; got != "blueprint" {
		t.Fatalf("component = %v; want blueprint", got)
	}
}

// TestSlogLogger_CorrelationIDIsCarriedThroughSlog asserts that the
// adapter attaches the correlation ID as a reserved attribute and
// the handler promotes it to the Record's top-level field.
func TestSlogLogger_CorrelationIDIsCarriedThroughSlog(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	base := newSlogLogger(Options{
		Level:     LevelInfo,
		Format:    FormatJSON,
		Component: ComponentCLI,
		Writer:    &buf,
	})

	id := NewCorrelationID()
	l := base.withCorrelationID(id)
	l.Info("x")

	var doc map[string]any
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := doc["correlation_id"]; got != id {
		t.Fatalf("correlation_id = %v; want %q", got, id)
	}
}

// TestSlogLogger_ComponentPreservedThroughWith asserts that a child
// logger created by With carries the parent's component. The
// component is a property of the logger, not of its attributes, so
// it must survive the With chain.
func TestSlogLogger_ComponentPreservedThroughWith(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	l := New(Options{
		Level:     LevelInfo,
		Format:    FormatJSON,
		Component: ComponentFilesystem,
		Writer:    &buf,
	}).With("k", "v")

	l.Info("x")

	var doc map[string]any
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := doc["component"]; got != "filesystem" {
		t.Fatalf("component = %v; want filesystem", got)
	}
	attrs, _ := doc["attrs"].(map[string]any)
	if attrs == nil || attrs["k"] != "v" {
		t.Fatalf("attrs = %v; want k=v", doc["attrs"])
	}
}

// =============================================================================
// With semantics through the adapter
// =============================================================================

// TestSlogLogger_WithReturnsNewLogger asserts that With returns a
// distinct logger value. The interface permits returning the
// receiver for an empty args, but a non-empty args must produce a
// distinct value.
func TestSlogLogger_WithReturnsNewLogger(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	base := New(Options{
		Level:     LevelInfo,
		Component: ComponentCLI,
		Sink:      sink,
	})
	child := base.With("k", "v")
	if child == base {
		t.Fatal("With(non-empty) returned the receiver")
	}

	child.Info("hello")
	got := sink.snapshot()
	if len(got) != 1 {
		t.Fatalf("got %d records; want 1", len(got))
	}
	if got[0].Attrs["k"] != "v" {
		t.Fatalf("attr k = %v; want v", got[0].Attrs["k"])
	}
}

// =============================================================================
// The handler's translation (slog.Record → package Record)
// =============================================================================

// TestForgeHandler_TranslatesSlogRecord exercises the translation
// directly. The handler is the seam between slog and the package's
// schema; testing it in isolation localizes a failure to the
// translation rather than to the wiring.
func TestForgeHandler_TranslatesSlogRecord(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	res := resolvedOptions{
		level:     LevelDebug,
		format:    FormatJSON,
		component: ComponentCLI,
		sink:      sink,
	}
	h := newForgeHandler(res)

	r := slog.NewRecord(
		time.Date(2026, 10, 13, 10, 0, 0, 123456789, time.UTC),
		slog.LevelWarn,
		"hello",
		0,
	)
	r.AddAttrs(slog.String("k", "v"))

	if err := h.Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	got := sink.snapshot()
	if len(got) != 1 {
		t.Fatalf("got %d records; want 1", len(got))
	}
	if got[0].Level != LevelWarn {
		t.Errorf("Level = %v; want Warn", got[0].Level)
	}
	if got[0].Message != "hello" {
		t.Errorf("Message = %q; want hello", got[0].Message)
	}
	if got[0].Attrs["k"] != "v" {
		t.Errorf("attr k = %v; want v", got[0].Attrs["k"])
	}
	if got[0].Ts.Location() != time.UTC {
		t.Errorf("Ts location = %v; want UTC", got[0].Ts.Location())
	}
}

// TestForgeHandler_PromotesReservedAttrs asserts that the handler
// strips the reserved component/correlation-ID attributes and
// promotes them to the Record's top-level fields. Without this
// behaviour, the JSON rendering would carry an `_forge_component`
// attr alongside a `component` field, which is redundant and
// leaks the adapter's implementation.
func TestForgeHandler_PromotesReservedAttrs(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	res := resolvedOptions{
		level:  LevelDebug,
		format: FormatJSON,
		sink:   sink,
	}
	h := newForgeHandler(res)

	r := slog.NewRecord(time.Time{}, slog.LevelInfo, "x", 0)
	r.AddAttrs(
		slog.String(attrKeyComponent, "filesystem"),
		slog.String(attrKeyCorrelationID, "id-123"),
		slog.String("user", "value"),
	)
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
	if got[0].CorrelationID != "id-123" {
		t.Errorf("CorrelationID = %q; want id-123", got[0].CorrelationID)
	}
	if got[0].Attrs["user"] != "value" {
		t.Errorf("attr user = %v; want value", got[0].Attrs["user"])
	}
	if _, collision := got[0].Attrs[attrKeyComponent]; collision {
		t.Errorf("reserved attr key leaked into Attrs: %v", got[0].Attrs)
	}
	if _, collision := got[0].Attrs[attrKeyCorrelationID]; collision {
		t.Errorf("reserved attr key leaked into Attrs: %v", got[0].Attrs)
	}
}

// TestForgeHandler_WithGroup asserts that a group name prefixes
// every subsequent attribute key. The dotted form is the
// convention used by the human renderer; the JSON renderer
// receives the same key.
func TestForgeHandler_WithGroup(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	res := resolvedOptions{
		level: LevelDebug,
		sink:  sink,
	}
	h := newForgeHandler(res)
	grouped := h.WithGroup("db")

	r := slog.NewRecord(time.Time{}, slog.LevelInfo, "x", 0)
	r.AddAttrs(slog.String("query", "SELECT 1"))
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
}

// =============================================================================
// End-to-end: adapter produces schema-valid JSON
// =============================================================================

// TestSlogLogger_ProducesSchemaValidJSON asserts that the adapter's
// output has every field the schema requires, with the right
// types. This is the ATDD test for the adapter's integration with
// the record schema from WBS 12.2.1.
func TestSlogLogger_ProducesSchemaValidJSON(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	l := New(Options{
		Level:     LevelInfo,
		Format:    FormatJSON,
		Component: ComponentCLI,
		Writer:    &buf,
	})
	l.Info("hello", "k", "v")

	var doc map[string]any
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("decode: %v", err)
	}

	for _, key := range []string{"ts", "level", "component", "msg", "correlation_id"} {
		if _, ok := doc[key]; !ok {
			t.Errorf("missing required field %q in %v", key, doc)
		}
	}
	if doc["level"] != "info" {
		t.Errorf("level = %v; want info", doc["level"])
	}
	if doc["component"] != "cli" {
		t.Errorf("component = %v; want cli", doc["component"])
	}
	if doc["msg"] != "hello" {
		t.Errorf("msg = %v; want hello", doc["msg"])
	}
	if doc["correlation_id"] != ZeroCorrelationID {
		t.Errorf("correlation_id = %v; want zero",
			doc["correlation_id"])
	}
}
