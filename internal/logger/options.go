package logger

import (
	"context"
	"io"
	"os"
	"sync"
)

// =============================================================================
// Sink
// =============================================================================

// Sink is the destination for log records.
//
// Sink is the injection seam for tests. The production Sink writes
// to os.Stderr. Implementations MUST NOT write to stdout.
//
// # Concurrency
//
// Sink implementations MUST be safe for concurrent use. The default
// slogLogger calls Emit without holding its own lock, so multiple
// goroutines may call Emit simultaneously. Implementations must
// serialize their own state (e.g. with a sync.Mutex) and must not
// lose records under contention.
//
// The StderrSink in this package is safe for concurrent use; tests
// that inject a custom Sink (such as a record-capturing sink) must
// also be safe for concurrent use, even if the test itself is
// single-goroutine, because the Logger's contract permits
// concurrent calls.
type Sink interface {
	// Emit writes a single log record. Implementations are
	// responsible for serializing concurrent calls.
	//
	// Emit is called on the hot path; it should not block longer
	// than the underlying writer takes. A Sink that performs I/O
	// to a slow destination is the caller's choice, not the
	// Logger's concern.
	Emit(ctx context.Context, r Record)
}

// =============================================================================
// StderrSink
// =============================================================================

// StderrSink writes log records to a writer (os.Stderr by default)
// in the configured format.
//
// It never writes to stdout. It is safe for concurrent use.
//
// # Rendering
//
// FormatJSON dispatches to writeJSON (format_json.go). Any other
// value (including FormatHuman) dispatches to writeHuman
// (format_human.go). Both write exactly one line per record and end
// the line with a newline.
//
// # Write errors
//
// A write error on stderr is not actionable: the process cannot
// report a failure to report a failure. StderrSink drops the
// error. This is the deliberate exception to the "no silent error"
// rule; it is documented here so a reader does not "fix" it by
// panicking (which would abort the process during an unrelated
// failure) or by returning the error (which the Logger has no way
// to act on).
//
// # Naming
//
// The type is named after its production destination, not after
// the abstract concept of a sink. The name is a slight misnomer
// when it writes to a buffer (as tests do), but the production
// case dominates, and the alternative names ("LineSink",
// "WriterSink") are less obvious.
type StderrSink struct {
	mu     sync.Mutex
	w      io.Writer
	format Format
}

// NewStderrSink returns a Sink bound to os.Stderr with the given
// format.
//
// This is the production constructor. Tests use NewSink with a
// bytes.Buffer.
func NewStderrSink(format Format) *StderrSink {
	return &StderrSink{w: os.Stderr, format: format}
}

// NewSink returns a Sink bound to w. Tests use this with
// bytes.Buffer; production uses NewStderrSink.
//
// If w is nil, os.Stderr is used. A nil writer is a caller's
// mistake, but the fallback makes the common case (test that
// forgot to pass a buffer) safe rather than a nil-pointer panic
// in the middle of a log call.
func NewSink(w io.Writer, format Format) *StderrSink {
	if w == nil {
		w = os.Stderr
	}
	return &StderrSink{w: w, format: format}
}

// Emit implements Sink.
//
// The method takes the sink's mutex for the duration of the write.
// Two concurrent records serialize on this mutex; a slow writer
// therefore serializes all log calls. The alternative — one mutex
// per record and relying on the writer to be thread-safe — is not
// valid for io.Writer, which does not guarantee atomic multi-byte
// writes across goroutines.
//
// A cancelled context is a caller's signal that the operation is
// no longer wanted; the record is dropped. The check happens
// before taking the lock so a cancelled caller does not contend
// with live writers.
func (s *StderrSink) Emit(ctx context.Context, r Record) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var err error
	switch s.format {
	case FormatJSON:
		err = writeJSON(s.w, r)
	default:
		err = writeHuman(s.w, r)
	}
	// See the type docstring: stderr write errors are dropped.
	_ = err
}

// =============================================================================
// resolvedOptions
// =============================================================================

// resolvedOptions is the normalized form of Options.
//
// Options is the public constructor's input. resolvedOptions fills
// in the defaults and unifies Writer and Sink into a single Sink:
//
//   - If opts.Sink is non-nil, it is used directly.
//   - Otherwise, a StderrSink (or Sink-on-Writer) is constructed
//     from opts.Writer.
//
// An empty opts.CorrelationID is normalized to ZeroCorrelationID,
// the sentinel for "not part of an invocation". A consumer can
// filter out records that carry the sentinel.
//
// The normalization happens once, in New. Subsequent code uses the
// Sink interface and the normalized fields.
type resolvedOptions struct {
	level         Level
	format        Format
	component     Component
	correlationID string
	sink          Sink
}

// resolve normalizes opts.
//
// The function does not validate; validation is in New. Its only
// job is to fill defaults and unify the two sink representations
// into one Sink.
//
// # Level default
//
// The zero value of Level is LevelInfo (see level.go). A zero
// Options therefore resolves to a logger at LevelInfo, and a
// caller who wants a different level passes it explicitly. No
// normalization is needed: the type's zero value is already the
// documented default.
//
// # Correlation ID
//
// An empty opts.CorrelationID is replaced by
// ZeroCorrelationID. The replacement is deliberate: a record's
// correlation_id field must never be empty (the record schema
// requires the field), and the sentinel is the well-defined
// value for "no ID was bound".
func resolve(opts Options) resolvedOptions {
	sink := opts.Sink
	if sink == nil {
		w := writerOrDefault(opts.Writer)
		sink = NewSink(w, opts.Format)
	}
	corrID := opts.CorrelationID
	if corrID == "" {
		corrID = ZeroCorrelationID
	}
	return resolvedOptions{
		level:         opts.Level,
		format:        opts.Format,
		component:     opts.Component,
		correlationID: corrID,
		sink:          sink,
	}
}

// writerOrDefault returns w, or os.Stderr if w is nil.
//
// The helper exists so that a caller who wants "the production
// default writer" without constructing an Options can obtain it.
// It is used by resolve and is a candidate for inlining if a
// future linter flags it as trivial.
func writerOrDefault(w io.Writer) io.Writer {
	if w == nil {
		return os.Stderr
	}
	return w
}
