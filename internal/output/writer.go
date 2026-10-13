package output

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
)

// EnvelopeVersion is the schema version of the JSON output envelope.
//
// Bump this only for breaking changes to the envelope shape. Additive
// changes do not require a bump. See schemas/output-envelope.schema.json
// and ADR-022.
const EnvelopeVersion = "1"

// Envelope is the stable JSON wrapper for all machine-readable output.
//
// Every --format json command emits exactly one Envelope on stdout. The
// Data field carries the command-specific payload; SchemaVersion and
// Command allow downstream consumers to dispatch and version-check.
type Envelope struct {
	SchemaVersion string `json:"schemaVersion"`
	Command       string `json:"command"`
	Data          any    `json:"data"`
}

// Writer renders output to a destination stream.
//
// Writer is the injection seam for tests. The production implementation
// writes to os.Stdout. Implementations MUST NOT write to stderr.
type Writer interface {
	// WriteOutput renders v using the given format to the writer's
	// destination stream (stdout in production).
	//
	// For FormatText, v is rendered with fmt.Fprintln semantics.
	// For FormatJSON, v is wrapped in an Envelope and emitted as a
	// single JSON document followed by a newline.
	WriteOutput(ctx context.Context, format Format, v any) error
}

// StdoutWriter is the production Writer bound to os.Stdout.
//
// It is safe for concurrent use. It never writes to stderr.
type StdoutWriter struct {
	mu        sync.Mutex
	w         io.Writer
	command   string
	closeOnce sync.Once
}

// NewWriter returns a Writer bound to w.
//
// Pass os.Stdout in production; pass a bytes.Buffer in tests. If w is
// nil, os.Stdout is used. The command name is embedded in JSON envelopes
// to identify the producing command; pass an empty string if unknown.
func NewWriter(w io.Writer, command string) *StdoutWriter {
	if w == nil {
		w = os.Stdout
	}
	return &StdoutWriter{w: w, command: command}
}

// WriteOutput implements Writer.
//
// It is the single sanctioned path for product output. It never writes
// to stderr. For JSON, it guarantees exactly one JSON document.
func (s *StdoutWriter) WriteOutput(ctx context.Context, format Format, v any) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	switch format {
	case FormatText:
		_, err := fmt.Fprintln(s.w, renderText(v))
		return err

	case FormatJSON:
		env := Envelope{
			SchemaVersion: EnvelopeVersion,
			Command:       s.command,
			Data:          v,
		}
		enc := json.NewEncoder(s.w)
		enc.SetEscapeHTML(false) // preserve user data verbatim
		if err := enc.Encode(env); err != nil {
			return fmt.Errorf("encode json envelope: %w", err)
		}
		return nil

	default:
		return fmt.Errorf("output: unsupported format %s", format)
	}
}

// renderText converts a value to its text representation.
//
// Strings are emitted verbatim; other values use fmt.Sprint. This is the
// only place text output is shaped, keeping format decisions centralized.
func renderText(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

// ErrNoOutput indicates a command produced no output for the given format.
//
// Commands may return this to signal "nothing to print" without it being
// an error. Callers should treat it as a no-op.
var ErrNoOutput = errors.New("output: no output for format")

// Package-level default writer, bound to os.Stdout.
//
// Commands in internal/cli call output.Print via this default after
// wiring the command name. Tests inject their own Writer.
var defaultWriter Writer = NewWriter(os.Stdout, "")

// SetDefaultWriter replaces the package-level default writer.
//
// It is intended for wiring at process start and for tests. It is NOT
// safe for concurrent use with Print; wire it once in main before any
// command runs.
func SetDefaultWriter(w Writer) { defaultWriter = w }

// DefaultWriter returns the current package-level default writer.
func DefaultWriter() Writer { return defaultWriter }

// Print writes v to the default writer using format.
//
// This is the ergonomic entry point for commands. It never writes to
// stderr.
func Print(ctx context.Context, format Format, v any) error {
	return defaultWriter.WriteOutput(ctx, format, v)
}
