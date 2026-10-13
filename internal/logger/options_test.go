package logger

import (
	"bytes"
	"context"
	"os"
	"testing"
)

// =============================================================================
// Sink interface satisfaction
// =============================================================================

// TestStderrSink_ImplementsSink pins the compile-time assertion that
// the concrete sink satisfies the Sink interface. The assertion is
// here, in a test, rather than in options.go, so a future refactor
// that removes a method fails in the file whose test names the
// missing capability.
func TestStderrSink_ImplementsSink(t *testing.T) {
	t.Parallel()

	var _ Sink = (*StderrSink)(nil)
}

// =============================================================================
// NewSink: the constructor bound to a writer
// =============================================================================

// TestNewSink_ValidWriter asserts the basic contract: a Sink bound
// to a valid writer writes to that writer.
func TestNewSink_ValidWriter(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	s := NewSink(&buf, FormatHuman)

	s.Emit(context.Background(), Record{
		Level:     LevelInfo,
		Component: ComponentCLI,
		Message:   "hello",
	})

	if buf.Len() == 0 {
		t.Fatal("sink wrote nothing to the injected writer")
	}
}

// TestNewSink_NilWriterFallsBackToStderr asserts that a nil writer
// does not panic. The docstring makes this promise so that a test
// that forgot to pass a buffer fails gracefully rather than with a
// nil-pointer dereference in the middle of a log call.
//
// The test cannot easily observe that the fallback destination is
// os.Stderr without capturing the process's stderr; it asserts the
// observable property: Emit does not panic and returns normally.
func TestNewSink_NilWriterFallsBackToStderr(t *testing.T) {
	t.Parallel()

	s := NewSink(nil, FormatHuman)
	if s == nil {
		t.Fatal("NewSink(nil, ...) returned nil")
	}
	// The fallback is os.Stderr, which the test cannot capture
	// safely. The property under test is "does not panic".
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("NewSink(nil, ...).Emit panicked: %v", r)
		}
	}()
	s.Emit(context.Background(), Record{
		Level:     LevelInfo,
		Component: ComponentCLI,
		Message:   "will go to stderr",
	})
}

// TestNewStderrSink_BoundToStderr asserts that the production
// constructor uses os.Stderr. The test asserts by identity: the
// internal writer must be exactly os.Stderr, not a wrapper.
func TestNewStderrSink_BoundToStderr(t *testing.T) {
	t.Parallel()

	s := NewStderrSink(FormatHuman)
	if s.w != os.Stderr {
		t.Fatalf("NewStderrSink bound to %p; want os.Stderr (%p)",
			s.w, os.Stderr)
	}
}

// =============================================================================
// StderrSink: format dispatch
// =============================================================================

// TestStderrSink_DispatchesOnFormat asserts that the sink's format
// field selects the renderer. FormatJSON produces a JSON object;
// FormatHuman produces a `[component]`-prefixed line.
func TestStderrSink_DispatchesOnFormat(t *testing.T) {
	t.Parallel()

	rec := Record{
		Level:         LevelInfo,
		Component:     ComponentCLI,
		Message:       "hello",
		CorrelationID: ZeroCorrelationID,
	}

	t.Run("human", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		NewSink(&buf, FormatHuman).Emit(context.Background(), rec)
		if !bytes.Contains(buf.Bytes(), []byte("[cli]")) {
			t.Fatalf("human rendering lacks [cli]: %q", buf.String())
		}
	})

	t.Run("json", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		NewSink(&buf, FormatJSON).Emit(context.Background(), rec)
		if !bytes.HasPrefix(buf.Bytes(), []byte("{")) {
			t.Fatalf("json rendering does not start with {: %q", buf.String())
		}
	})
}

// TestStderrSink_UnknownFormatFallsBackToHuman asserts the documented
// fallback: a format other than FormatJSON dispatches to the human
// renderer. The switch in Emit is `case FormatJSON: ... default: ...`,
// so any non-JSON value (including a future enum member) renders as
// text rather than panicking or emitting nothing.
func TestStderrSink_UnknownFormatFallsBackToHuman(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	s := NewSink(&buf, Format(99))
	s.Emit(context.Background(), Record{
		Level:     LevelInfo,
		Component: ComponentCLI,
		Message:   "hello",
	})
	if !bytes.Contains(buf.Bytes(), []byte("[cli]")) {
		t.Fatalf("unknown format did not fall back to human: %q", buf.String())
	}
}

// =============================================================================
// StderrSink: cancelled context
// =============================================================================

// TestStderrSink_CancelledContextDropsRecord asserts that a
// cancelled context prevents the write. The check is the first
// thing Emit does; the record is discarded before the lock is
// taken.
func TestStderrSink_CancelledContextDropsRecord(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	s := NewSink(&buf, FormatHuman)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	s.Emit(ctx, Record{
		Level:     LevelInfo,
		Component: ComponentCLI,
		Message:   "should be dropped",
	})

	if buf.Len() != 0 {
		t.Fatalf("cancelled context produced output: %q", buf.String())
	}
}

// TestStderrSink_NilContextIsAccepted asserts that a nil context
// does not panic. The nil check in Emit is deliberate; a caller
// that constructs a Record outside an invocation may not have a
// context to pass.
func TestStderrSink_NilContextIsAccepted(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	s := NewSink(&buf, FormatHuman)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Emit(nil, ...) panicked: %v", r)
		}
	}()
	s.Emit(nil, Record{
		Level:     LevelInfo,
		Component: ComponentCLI,
		Message:   "hello",
	})
	if buf.Len() == 0 {
		t.Fatal("nil context suppressed the write; want it accepted")
	}
}

// =============================================================================
// resolve: sink precedence
// =============================================================================

// TestResolve_ExplicitSinkWins asserts the documented precedence:
// when both Writer and Sink are set, Sink wins. The rule is stated
// on Options and enforced in resolve.
func TestResolve_ExplicitSinkWins(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	var buf bytes.Buffer

	res := resolve(Options{
		Level:     LevelInfo,
		Format:    FormatHuman,
		Component: ComponentCLI,
		Writer:    &buf,
		Sink:      sink,
	})

	if res.sink != Sink(sink) {
		t.Fatalf("resolve chose Writer; want Sink")
	}
	// Exercise the resolved sink; the buffer must stay empty.
	res.sink.Emit(context.Background(), Record{
		Level:     LevelInfo,
		Component: ComponentCLI,
		Message:   "x",
	})
	if buf.Len() != 0 {
		t.Fatalf("resolve used Writer despite Sink being set: %q", buf.String())
	}
	if sink.len() != 1 {
		t.Fatalf("sink received %d records; want 1", sink.len())
	}
}

// TestResolve_WriterUsedWhenSinkNil asserts the complementary case:
// when Sink is nil, a Sink is constructed from Writer.
func TestResolve_WriterUsedWhenSinkNil(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	res := resolve(Options{
		Level:     LevelInfo,
		Format:    FormatHuman,
		Component: ComponentCLI,
		Writer:    &buf,
	})

	res.sink.Emit(context.Background(), Record{
		Level:     LevelInfo,
		Component: ComponentCLI,
		Message:   "hello",
	})
	if buf.Len() == 0 {
		t.Fatal("resolve did not construct a sink from Writer")
	}
}

// TestResolve_NilWriterNilSinkFallsBackToStderr asserts that when
// both are nil, resolve constructs a sink bound to os.Stderr. The
// test cannot capture os.Stderr safely, so it asserts the
// observable property: Emit does not panic.
func TestResolve_NilWriterNilSinkFallsBackToStderr(t *testing.T) {
	t.Parallel()

	res := resolve(Options{
		Level:     LevelInfo,
		Component: ComponentCLI,
	})
	if res.sink == nil {
		t.Fatal("resolve produced a nil sink")
	}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("resolved sink panicked: %v", r)
		}
	}()
	res.sink.Emit(context.Background(), Record{
		Level:     LevelInfo,
		Component: ComponentCLI,
		Message:   "to stderr",
	})
}

// =============================================================================
// resolve: fields are carried through
// =============================================================================

// TestResolve_PreservesFields asserts that resolve copies every
// field from Options to resolvedOptions. A field dropped in the
// copy would surface as a logger that ignores its configuration;
// this test catches that at the unit level, before any command
// runs.
func TestResolve_PreservesFields(t *testing.T) {
	t.Parallel()

	in := Options{
		Level:     LevelWarn,
		Format:    FormatJSON,
		Component: ComponentBlueprint,
	}
	out := resolve(in)

	if out.level != in.Level {
		t.Errorf("level = %v; want %v", out.level, in.Level)
	}
	if out.format != in.Format {
		t.Errorf("format = %v; want %v", out.format, in.Format)
	}
	if out.component != in.Component {
		t.Errorf("component = %v; want %v", out.component, in.Component)
	}
}

// TestResolve_ZeroOptions asserts the defaults. A zero Options must
// produce LevelInfo (the zero value), FormatHuman (the zero value),
// ComponentInvalid (the zero value), and a sink bound to os.Stderr.
//
// This test does not construct a Logger; it works on the resolved
// form directly, so the assertion is against the normalization
// rules, not against the logger's behavior.
func TestResolve_ZeroOptions(t *testing.T) {
	t.Parallel()

	out := resolve(Options{})
	if out.level != LevelInfo {
		t.Errorf("level = %v; want LevelInfo (zero)", out.level)
	}
	if out.format != FormatHuman {
		t.Errorf("format = %v; want FormatHuman (zero)", out.format)
	}
	if out.component != ComponentInvalid {
		t.Errorf("component = %v; want ComponentInvalid (zero)", out.component)
	}
	if out.sink == nil {
		t.Error("sink is nil; want a StderrSink")
	}
}

// =============================================================================
// New: panic on missing component
// =============================================================================

// TestNew_PanicsOnMissingComponentAtNonDebugLevel asserts the
// deliberate panic. The panic is the constructor's way of catching
// the common mistake of forgetting Component; the debug-level
// exception lets tests exercise the zero value.
func TestNew_PanicsOnMissingComponentAtNonDebugLevel(t *testing.T) {
	t.Parallel()

	levels := []Level{LevelInfo, LevelWarn, LevelError}
	for _, lvl := range levels {
		lvl := lvl
		t.Run(lvl.String(), func(t *testing.T) {
			t.Parallel()
			defer func() {
				if r := recover(); r == nil {
					t.Fatalf("New with zero component at %v did not panic", lvl)
				}
			}()
			_ = New(Options{Level: lvl})
		})
	}
}

// TestNew_ZeroComponentAllowedAtDebugLevel asserts the exception.
// The debug level permits a zero component so tests can exercise
// the zero value without bypassing the constructor.
func TestNew_ZeroComponentAllowedAtDebugLevel(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("New with zero component at debug panicked: %v", r)
		}
	}()
	l := New(Options{Level: LevelDebug})
	if l == nil {
		t.Fatal("New returned nil")
	}
}

// TestNew_NonZeroComponentAlwaysAllowed asserts that setting a
// component at any level is permitted.
func TestNew_NonZeroComponentAlwaysAllowed(t *testing.T) {
	t.Parallel()

	levels := []Level{LevelDebug, LevelInfo, LevelWarn, LevelError}
	for _, lvl := range levels {
		lvl := lvl
		t.Run(lvl.String(), func(t *testing.T) {
			t.Parallel()
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("New with component at %v panicked: %v", lvl, r)
				}
			}()
			_ = New(Options{Level: lvl, Component: ComponentCLI})
		})
	}
}

// =============================================================================
// New: the returned logger is usable
// =============================================================================

// TestNew_ReturnsUsableLogger asserts that the value returned by New
// is ready to use: it writes through the configured sink without
// further setup.
func TestNew_ReturnsUsableLogger(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	l := New(Options{
		Level:     LevelInfo,
		Component: ComponentCLI,
		Sink:      sink,
	})

	l.Info("hello")

	if sink.len() != 1 {
		t.Fatalf("sink received %d records; want 1", sink.len())
	}
}

// TestNew_RespectsLevel asserts that the level configured in
// Options is the level the returned logger uses. The check is
// behavioral: a Debug call at Info level produces no record.
func TestNew_RespectsLevel(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	l := New(Options{
		Level:     LevelWarn,
		Component: ComponentCLI,
		Sink:      sink,
	})

	l.Debug("d")
	l.Info("i")
	l.Warn("w")
	l.Error("e")

	if got := sink.len(); got != 2 {
		t.Fatalf("sink received %d records; want 2", got)
	}
}

// TestNew_RespectsFormat asserts that the format configured in
// Options is the format the returned logger uses. The check is
// behavioral: JSON output starts with `{`, human output does not.
func TestNew_RespectsFormat(t *testing.T) {
	t.Parallel()

	t.Run("human", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		l := New(Options{
			Level:     LevelInfo,
			Format:    FormatHuman,
			Component: ComponentCLI,
			Writer:    &buf,
		})
		l.Info("hello")
		if bytes.HasPrefix(buf.Bytes(), []byte("{")) {
			t.Fatalf("human format produced JSON: %q", buf.String())
		}
	})

	t.Run("json", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		l := New(Options{
			Level:     LevelInfo,
			Format:    FormatJSON,
			Component: ComponentCLI,
			Writer:    &buf,
		})
		l.Info("hello")
		if !bytes.HasPrefix(buf.Bytes(), []byte("{")) {
			t.Fatalf("json format produced non-JSON: %q", buf.String())
		}
	})
}

// TestNew_RespectsComponent asserts that the component configured
// in Options is carried on every record.
func TestNew_RespectsComponent(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	l := New(Options{
		Level:     LevelInfo,
		Component: ComponentFilesystem,
		Sink:      sink,
	})
	l.Info("x")

	got := sink.snapshot()
	if len(got) != 1 {
		t.Fatalf("sink received %d records; want 1", len(got))
	}
	if got[0].Component != ComponentFilesystem {
		t.Fatalf("component = %v; want Filesystem", got[0].Component)
	}
}

// =============================================================================
// writerOrDefault
// =============================================================================

// TestWriterOrDefault asserts the helper's contract: nil falls back
// to os.Stderr; a non-nil writer is returned unchanged.
func TestWriterOrDefault(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if got := writerOrDefault(&buf); got != &buf {
		t.Errorf("writerOrDefault(non-nil) = %p; want %p", got, &buf)
	}
	if got := writerOrDefault(nil); got != os.Stderr {
		t.Errorf("writerOrDefault(nil) = %p; want os.Stderr (%p)",
			got, os.Stderr)
	}
}
