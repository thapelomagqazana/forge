package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
)

// =============================================================================
// Test doubles
// =============================================================================

// captureSink records emitted records for assertions.
//
// # Concurrency
//
// captureSink MUST be safe for concurrent use, because the default
// Logger calls Sink.Emit without holding its own lock. (See the
// Sink interface doc in options.go.) A mutex guards the slice so
// concurrent Emit calls do not race on the slice header or lose
// records during append.
//
// Tests must never read c.records directly. Use snapshot() or
// len().
type captureSink struct {
	mu      sync.Mutex
	records []Record
}

// Emit implements Sink. Safe for concurrent use.
func (c *captureSink) Emit(_ context.Context, r Record) {
	c.mu.Lock()
	c.records = append(c.records, r)
	c.mu.Unlock()
}

// snapshot returns a copy of the recorded records.
//
// A copy is returned so the caller can iterate without holding the
// lock and without racing with concurrent Emit calls.
func (c *captureSink) snapshot() []Record {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Record, len(c.records))
	copy(out, c.records)
	return out
}

// len returns the number of recorded records.
func (c *captureSink) len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.records)
}

// newCaptureLogger returns a Logger that writes to sink at the given
// threshold and component. It is the canonical constructor for the
// tests in this file.
//
// The helper exists so a change to Options' shape touches one line,
// not every test. The tests care about the logger's behaviour, not
// about the constructor's field order.
func newCaptureLogger(t *testing.T, sink Sink, level Level, component Component) Logger {
	t.Helper()
	return New(Options{
		Level:     level,
		Format:    FormatHuman,
		Component: component,
		Sink:      sink,
	})
}

// =============================================================================
// TDD: the four level methods
// =============================================================================

func TestLogger_AllFourLevelsEmit(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	l := newCaptureLogger(t, sink, LevelDebug, ComponentCLI)

	l.Debug("d")
	l.Info("i")
	l.Warn("w")
	l.Error("e")

	got := sink.snapshot()
	if len(got) != 4 {
		t.Fatalf("got %d records; want 4", len(got))
	}
	want := []Level{LevelDebug, LevelInfo, LevelWarn, LevelError}
	for i, w := range want {
		if got[i].Level != w {
			t.Errorf("record %d level = %v; want %v", i, got[i].Level, w)
		}
	}
}

// =============================================================================
// TDD: level filtering at every threshold
// =============================================================================

// TestLogger_LevelFiltering exhaustively checks the four thresholds
// against the four levels. A threshold enables its own level and
// every more severe level.
func TestLogger_LevelFiltering(t *testing.T) {
	t.Parallel()

	type call struct {
		level Level
		emit  func(Logger)
	}
	calls := []call{
		{LevelDebug, func(l Logger) { l.Debug("d") }},
		{LevelInfo, func(l Logger) { l.Info("i") }},
		{LevelWarn, func(l Logger) { l.Warn("w") }},
		{LevelError, func(l Logger) { l.Error("e") }},
	}

	cases := []struct {
		threshold Level
		want      []Level
	}{
		{LevelDebug, []Level{LevelDebug, LevelInfo, LevelWarn, LevelError}},
		{LevelInfo, []Level{LevelInfo, LevelWarn, LevelError}},
		{LevelWarn, []Level{LevelWarn, LevelError}},
		{LevelError, []Level{LevelError}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.threshold.String(), func(t *testing.T) {
			t.Parallel()

			sink := &captureSink{}
			l := newCaptureLogger(t, sink, tc.threshold, ComponentCLI)
			for _, c := range calls {
				c.emit(l)
			}

			got := sink.snapshot()
			if len(got) != len(tc.want) {
				t.Fatalf("threshold %v: got %d records; want %d",
					tc.threshold, len(got), len(tc.want))
			}
			for i, w := range tc.want {
				if got[i].Level != w {
					t.Errorf("threshold %v: record %d level = %v; want %v",
						tc.threshold, i, got[i].Level, w)
				}
			}
		})
	}
}

// =============================================================================
// TDD: attrs
// =============================================================================

func TestLogger_AttrsAreStructured(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	l := newCaptureLogger(t, sink, LevelDebug, ComponentCLI)
	l.Info("hello", "key", "value", "n", 42)

	got := sink.snapshot()
	if len(got) != 1 {
		t.Fatalf("got %d records; want 1", len(got))
	}
	if got[0].Attrs["key"] != "value" {
		t.Fatalf("attr key = %v", got[0].Attrs["key"])
	}
	if got[0].Attrs["n"] != 42 {
		t.Fatalf("attr n = %v", got[0].Attrs["n"])
	}
}

func TestLogger_OddAttrsTolerated(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	l := newCaptureLogger(t, sink, LevelDebug, ComponentCLI)
	l.Info("msg", "lonely")

	got := sink.snapshot()
	if len(got) != 1 {
		t.Fatalf("got %d records; want 1", len(got))
	}
	if v, ok := got[0].Attrs["lonely"]; !ok || v != nil {
		t.Fatalf("expected lonely=nil, got %v", got[0].Attrs)
	}
}

// --- Boundary: no attrs produces a nil map ---

func TestLogger_NoAttrs(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	l := newCaptureLogger(t, sink, LevelDebug, ComponentCLI)
	l.Info("plain")

	got := sink.snapshot()
	if len(got) != 1 {
		t.Fatalf("got %d records; want 1", len(got))
	}
	if got[0].Attrs != nil {
		t.Fatalf("expected nil attrs; got %v", got[0].Attrs)
	}
	if got[0].Message != "plain" {
		t.Fatalf("msg = %q; want %q", got[0].Message, "plain")
	}
}

// --- Negative: reserved attr key is namespaced, not lost ---

func TestLogger_ReservedAttrKeyIsNamespaced(t *testing.T) {
	t.Parallel()

	// A caller who passes a reserved key to Info (not WithAttr)
	// cannot receive a panic on the hot path: kvToMap renames the
	// key to "attr_" + key and preserves the value.
	sink := &captureSink{}
	l := newCaptureLogger(t, sink, LevelDebug, ComponentCLI)
	l.Info("msg", "level", "custom-value")

	got := sink.snapshot()
	if len(got) != 1 {
		t.Fatalf("got %d records; want 1", len(got))
	}
	if _, collision := got[0].Attrs["level"]; collision {
		t.Fatalf("reserved key collided: %v", got[0].Attrs)
	}
	if got[0].Attrs["attr_level"] != "custom-value" {
		t.Fatalf("namespaced key missing or wrong: %v", got[0].Attrs)
	}
}

// =============================================================================
// With: child logger
// =============================================================================

func TestLogger_WithPrependsAttrs(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	base := newCaptureLogger(t, sink, LevelDebug, ComponentCLI)
	child := base.With("k", "v")

	child.Info("hello")

	got := sink.snapshot()
	if len(got) != 1 {
		t.Fatalf("got %d records; want 1", len(got))
	}
	if got[0].Attrs["k"] != "v" {
		t.Fatalf("attr k = %v; want v", got[0].Attrs["k"])
	}
}

func TestLogger_WithMergesAttrs(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	base := newCaptureLogger(t, sink, LevelDebug, ComponentCLI)
	child := base.With("a", 1).With("b", 2)
	child.Info("x", "c", 3)

	got := sink.snapshot()
	if len(got) != 1 {
		t.Fatalf("got %d records; want 1", len(got))
	}
	if got[0].Attrs["a"] != 1 {
		t.Fatalf("attr a = %v; want 1", got[0].Attrs["a"])
	}
	if got[0].Attrs["b"] != 2 {
		t.Fatalf("attr b = %v; want 2", got[0].Attrs["b"])
	}
	if got[0].Attrs["c"] != 3 {
		t.Fatalf("attr c = %v; want 3", got[0].Attrs["c"])
	}
}

func TestLogger_WithSharesComponent(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	base := newCaptureLogger(t, sink, LevelDebug, ComponentFilesystem)
	child := base.With("k", "v")
	child.Info("x")

	got := sink.snapshot()
	if len(got) != 1 {
		t.Fatalf("got %d records; want 1", len(got))
	}
	if got[0].Component != ComponentFilesystem {
		t.Fatalf("Component = %v; want %v",
			got[0].Component, ComponentFilesystem)
	}
}

func TestLogger_WithEmptyReturnsEquivalentLogger(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	base := newCaptureLogger(t, sink, LevelDebug, ComponentCLI)
	got := base.With()
	if got == nil {
		t.Fatal("With() returned nil")
	}
	got.Info("x")
	if sink.len() != 1 {
		t.Fatalf("got %d records; want 1", sink.len())
	}
}

// =============================================================================
// WBS 12.2.1: Component
// =============================================================================

func TestLogger_ComponentIsAttached(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	l := New(Options{
		Level:     LevelDebug,
		Component: ComponentFilesystem,
		Sink:      sink,
	})
	l.Info("x")

	got := sink.snapshot()
	if len(got) != 1 {
		t.Fatalf("got %d records; want 1", len(got))
	}
	if got[0].Component != ComponentFilesystem {
		t.Fatalf("Component = %v; want %v",
			got[0].Component, ComponentFilesystem)
	}
}

// --- Edge: zero Component is carried through at debug level ---

func TestLogger_ZeroComponentIsCarried(t *testing.T) {
	t.Parallel()

	// A zero Component is rejected by New at non-debug levels. At
	// debug level, New permits it so tests can exercise the zero
	// value. The record carries ComponentInvalid, whose String()
	// is "invalid".
	sink := &captureSink{}
	l := New(Options{
		Level: LevelDebug, // exception: see New
		Sink:  sink,
		// Component deliberately left zero
	})
	l.Info("x")

	got := sink.snapshot()
	if len(got) != 1 {
		t.Fatalf("got %d records; want 1", len(got))
	}
	if got[0].Component != ComponentInvalid {
		t.Fatalf("Component = %v; want Invalid", got[0].Component)
	}
}

// =============================================================================
// WBS 12.2.1: CorrelationID
// =============================================================================

// newLoggerBoundToID is a test helper that returns a Logger bound to
// the correlation ID carried by ctx. Production code obtains an
// equivalent logger through logger.FromContext; the test uses the
// unexported binding directly because FromContext returns the Logger
// interface, whose correlation ID is not observable in the tests'
// assertions.
//
// This is the correct place to reach for the unexported seam: the
// tests live in the same package and exercise the binding that
// FromContext performs.
func newLoggerBoundToID(t *testing.T, ctx context.Context, sink Sink, component Component) Logger {
	t.Helper()
	base := newSlogLogger(Options{
		Level:     LevelDebug,
		Format:    FormatHuman,
		Component: component,
		Sink:      sink,
	})
	id := CorrelationIDFromContext(ctx)
	return base.withCorrelationID(id)
}

func TestLogger_CorrelationIDFromContext(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	id := NewCorrelationID()
	ctx := WithCorrelationID(context.Background(), id)

	l := newLoggerBoundToID(t, ctx, sink, ComponentCLI)
	l.Info("x")

	got := sink.snapshot()
	if len(got) != 1 {
		t.Fatalf("got %d records; want 1", len(got))
	}
	if got[0].CorrelationID != id {
		t.Fatalf("CorrelationID = %q; want %q",
			got[0].CorrelationID, id)
	}
}

// --- Edge: no ID in context yields the zero sentinel ---

func TestLogger_ZeroCorrelationIDWhenContextHasNone(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	l := newCaptureLogger(t, sink, LevelDebug, ComponentCLI)
	l.Info("x")

	got := sink.snapshot()
	if len(got) != 1 {
		t.Fatalf("got %d records; want 1", len(got))
	}
	if got[0].CorrelationID != ZeroCorrelationID {
		t.Fatalf("CorrelationID = %q; want zero %q",
			got[0].CorrelationID, ZeroCorrelationID)
	}
}

// --- ATDD: every record of one invocation shares one ID ---

func TestLogger_AllRecordsShareTheCorrelationID(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	id := NewCorrelationID()
	ctx := WithCorrelationID(context.Background(), id)
	l := newLoggerBoundToID(t, ctx, sink, ComponentCLI)

	l.Debug("d")
	l.Info("i")
	l.Warn("w")
	l.Error("e")

	got := sink.snapshot()
	if len(got) != 4 {
		t.Fatalf("got %d records; want 4", len(got))
	}
	for i, r := range got {
		if r.CorrelationID != id {
			t.Errorf("record %d CorrelationID = %q; want %q",
				i, r.CorrelationID, id)
		}
	}
}

// =============================================================================
// WBS 12.2.1: Record shape
// =============================================================================

func TestLogger_RecordTimestampIsUTC(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	l := newCaptureLogger(t, sink, LevelDebug, ComponentCLI)

	before := time.Now().UTC()
	l.Info("x")
	after := time.Now().UTC()

	got := sink.snapshot()
	if len(got) != 1 {
		t.Fatalf("got %d records; want 1", len(got))
	}
	ts := got[0].Ts
	if ts.Location() != time.UTC {
		t.Fatalf("Ts location = %v; want UTC", ts.Location())
	}
	if ts.Before(before) || ts.After(after) {
		t.Fatalf("Ts = %v; want in [%v, %v]", ts, before, after)
	}
}

func TestLogger_RecordMessageIsVerbatim(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	l := newCaptureLogger(t, sink, LevelDebug, ComponentCLI)
	l.Info("hello, world!")

	got := sink.snapshot()
	if got[0].Message != "hello, world!" {
		t.Fatalf("Message = %q; want %q",
			got[0].Message, "hello, world!")
	}
}

// =============================================================================
// TDD: StderrSink JSON format
// =============================================================================

func TestStderrSink_JSONFormat(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	s := NewSink(&buf, FormatJSON)
	s.Emit(context.Background(), Record{
		Ts:            time.Date(2026, 10, 13, 10, 0, 0, 0, time.UTC),
		Level:         LevelInfo,
		Component:     ComponentCLI,
		Message:       "hi",
		CorrelationID: "00000000-0000-4000-8000-000000000000",
	})

	// Exactly one JSON object on one line.
	dec := json.NewDecoder(&buf)
	var r map[string]any
	if err := dec.Decode(&r); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if dec.More() {
		t.Fatalf("more than one record: %q", buf.String())
	}
	if r["msg"] != "hi" {
		t.Fatalf("msg = %v; want hi", r["msg"])
	}
	if r["component"] != "cli" {
		t.Fatalf("component = %v; want cli", r["component"])
	}
}

// =============================================================================
// TDD: StderrSink human format
// =============================================================================

func TestStderrSink_HumanFormat(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	s := NewSink(&buf, FormatHuman)
	s.Emit(context.Background(), Record{
		Level:     LevelInfo,
		Component: ComponentCLI,
		Message:   "hi",
		Attrs:     map[string]any{"k": "v"},
	})
	out := buf.String()
	if !strings.HasPrefix(out, "INFO  [cli] hi") {
		t.Fatalf("prefix mismatch: %q", out)
	}
	if !strings.Contains(out, "k=v") {
		t.Fatalf("attr missing: %q", out)
	}
	if !strings.HasSuffix(out, "\n") {
		t.Fatalf("missing newline: %q", out)
	}
}

func TestStderrSink_TextFormat_NoHTMLInjection(t *testing.T) {
	t.Parallel()

	// Boundary: user data containing HTML-like characters must
	// not be escaped in the human format.
	var buf bytes.Buffer
	s := NewSink(&buf, FormatHuman)
	s.Emit(context.Background(), Record{
		Level:     LevelInfo,
		Component: ComponentCLI,
		Message:   "a < b",
	})
	if !strings.Contains(buf.String(), "a < b") {
		t.Fatalf("message mangled: %q", buf.String())
	}
}

func TestStderrSink_TextFormat_IncludesAttrs(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	s := NewSink(&buf, FormatHuman)
	s.Emit(context.Background(), Record{
		Level:     LevelInfo,
		Component: ComponentCLI,
		Message:   "hello",
		Attrs:     map[string]any{"k": "v", "n": 42},
	})
	out := buf.String()
	if !strings.Contains(out, "k=v") {
		t.Fatalf("missing attr k=v: %q", out)
	}
	if !strings.Contains(out, "n=42") {
		t.Fatalf("missing attr n=42: %q", out)
	}
}

// =============================================================================
// ATDD: logger never writes to stdout
// =============================================================================

func TestLogger_NeverWritesToStdout(t *testing.T) {
	t.Parallel()

	// The logger has no stdout handle by construction. The test
	// asserts the sink destination is the injected buffer, not
	// os.Stdout.
	var buf bytes.Buffer
	s := NewSink(&buf, FormatJSON)
	l := newCaptureLogger(t, s, LevelDebug, ComponentCLI)
	l.Info("x")
	if buf.Len() == 0 {
		t.Fatal("expected bytes in injected sink")
	}
	if s.w != &buf {
		t.Fatal("sink destination changed")
	}
}

// =============================================================================
// Negative: cancelled context
// =============================================================================

func TestStderrSink_CancelledContext_NoOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	s := NewSink(&buf, FormatJSON)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s.Emit(ctx, Record{
		Level:     LevelInfo,
		Component: ComponentCLI,
		Message:   "x",
	})
	if buf.Len() != 0 {
		t.Fatalf("expected no output on cancel; got %q", buf.String())
	}
}

// =============================================================================
// Non-functional: concurrent safety
// =============================================================================

// TestLogger_ConcurrentSafe exercises concurrent Info calls and
// asserts every record is delivered. Run with -race.
//
// Note: captureSink is thread-safe by construction, so this test is
// a real test of the Logger's concurrency contract, not of the test
// helper.
func TestLogger_ConcurrentSafe(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	l := newCaptureLogger(t, sink, LevelDebug, ComponentCLI)

	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			l.Info("x")
		}()
	}
	wg.Wait()

	if got := sink.len(); got != n {
		t.Fatalf("got %d records; want %d", got, n)
	}
}

// TestLogger_ConcurrentSafe_NoLostRecords stresses the sink under
// contention. Run with -race.
func TestLogger_ConcurrentSafe_NoLostRecords(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	l := newCaptureLogger(t, sink, LevelDebug, ComponentCLI)

	const (
		goroutines = 32
		perG       = 100
	)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < perG; i++ {
				l.Info("x", "g", id, "i", i)
			}
		}(g)
	}
	wg.Wait()

	want := goroutines * perG
	if got := sink.len(); got != want {
		t.Fatalf("got %d records; want %d (lost %d)",
			got, want, want-got)
	}
}

// TestStderrSink_ConcurrentSafe verifies that the production sink is
// safe for concurrent use. Run with -race.
func TestStderrSink_ConcurrentSafe(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	s := NewSink(&buf, FormatJSON)
	l := newCaptureLogger(t, s, LevelDebug, ComponentCLI)

	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			l.Info("x")
		}()
	}
	wg.Wait()

	// Each record is one line of JSON. StderrSink writes one
	// line per record; if the sink lost records or interleaved
	// writes, the count would be wrong.
	lines := strings.Count(buf.String(), "\n")
	if lines != n {
		t.Fatalf("got %d JSON lines; want %d (sink lost records)",
			lines, n)
	}
}
