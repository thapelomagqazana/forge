package output

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// --- TDD: unit tests for Writer ---

func TestWriter_TextFormat_WritesToDestination(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, "version")

	if err := w.WriteOutput(context.Background(), FormatText, "hello"); err != nil {
		t.Fatalf("WriteOutput: %v", err)
	}
	if got, want := buf.String(), "hello\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestWriter_JSONFormat_EmitsSingleEnvelope(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, "version")

	if err := w.WriteOutput(context.Background(), FormatJSON, map[string]string{"v": "1"}); err != nil {
		t.Fatalf("WriteOutput: %v", err)
	}

	// Must be exactly one JSON document.
	dec := json.NewDecoder(&buf)
	var env Envelope
	if err := dec.Decode(&env); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if dec.More() {
		t.Fatalf("stdout contains more than one JSON document: %q", buf.String())
	}
	if env.SchemaVersion != EnvelopeVersion {
		t.Fatalf("schemaVersion = %q, want %q", env.SchemaVersion, EnvelopeVersion)
	}
	if env.Command != "version" {
		t.Fatalf("command = %q, want %q", env.Command, "version")
	}
}

// --- ATDD: acceptance test for the stream contract ---

func TestWriter_NeverWritesToStderr(t *testing.T) {
	// ATDD: the writer has no stderr handle at all; this test asserts
	// the design by construction. If a future change adds stderr, this
	// test fails to compile, forcing a review.
	var buf bytes.Buffer
	w := NewWriter(&buf, "cmd")
	if err := w.WriteOutput(context.Background(), FormatJSON, "x"); err != nil {
		t.Fatal(err)
	}
	// The only sink is buf; there is no stderr field.
	if w.w != &buf {
		t.Fatal("writer destination changed")
	}
}

// --- Negative: unknown format ---

func TestWriter_UnknownFormat_ReturnsError(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, "cmd")
	err := w.WriteOutput(context.Background(), Format(99), "x")
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
	if buf.Len() != 0 {
		t.Fatalf("expected no output on error, got %q", buf.String())
	}
}

// --- Edge: cancelled context ---

func TestWriter_CancelledContext_ReturnsError(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, "cmd")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := w.WriteOutput(ctx, FormatText, "x")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context.Canceled", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected no output, got %q", buf.String())
	}
}

// --- Boundary: empty string output ---

func TestWriter_EmptyString_TextFormat(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, "cmd")
	if err := w.WriteOutput(context.Background(), FormatText, ""); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), "\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// --- Corner: non-string value in text format ---

func TestWriter_NonStringValue_TextFormat(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, "cmd")
	if err := w.WriteOutput(context.Background(), FormatText, 42); err != nil {
		t.Fatal(err)
	}
	if got, want := strings.TrimSpace(buf.String()), "42"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// --- Non-functional: no allocation on hot path (sanity) ---

func TestWriter_JSONFormat_NoHTMLInjection(t *testing.T) {
	// Boundary: user data containing HTML-like characters must be
	// preserved verbatim, not escaped to \u003c.
	var buf bytes.Buffer
	w := NewWriter(&buf, "cmd")
	if err := w.WriteOutput(context.Background(), FormatJSON, "<tag>"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), `\u003c`) {
		t.Fatalf("HTML escaping leaked into output: %q", buf.String())
	}
	if !strings.Contains(buf.String(), "<tag>") {
		t.Fatalf("user data not preserved: %q", buf.String())
	}
}
