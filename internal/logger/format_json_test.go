package logger

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestWriteJSON_SingleLine(t *testing.T) {
	var buf bytes.Buffer
	r := Record{
		Ts:            time.Now().UTC(),
		Level:         LevelInfo,
		Component:     ComponentCLI,
		Message:       "loaded configuration",
		CorrelationID: "0192e2b4-1c3a-7f6e-9a2b-4d5e6f708192",
		Attrs:         map[string]any{"path": "/etc/forge/config.yaml"},
	}
	if err := writeJSON(&buf, r); err != nil {
		t.Fatal(err)
	}
	if strings.Count(buf.String(), "\n") != 1 {
		t.Fatalf("expected exactly one newline: %q", buf.String())
	}

	var decoded map[string]any
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, buf.String())
	}
	for _, key := range []string{"ts", "level", "component", "msg", "correlation_id", "attrs"} {
		if _, ok := decoded[key]; !ok {
			t.Errorf("missing field %q in %v", key, decoded)
		}
	}
}

// --- Corner: no attrs omits the field ---

func TestWriteJSON_NoAttrsOmitsField(t *testing.T) {
	var buf bytes.Buffer
	r := Record{
		Ts:            time.Now().UTC(),
		Level:         LevelInfo,
		Component:     ComponentCLI,
		Message:       "x",
		CorrelationID: "id",
	}
	_ = writeJSON(&buf, r)
	if strings.Contains(buf.String(), `"attrs"`) {
		t.Fatalf("attrs field present despite empty map: %q", buf.String())
	}
}

// --- Boundary: values with HTML-like characters are not escaped ---

func TestWriteJSON_NoHTMLEscaping(t *testing.T) {
	var buf bytes.Buffer
	r := Record{
		Ts:            time.Now().UTC(),
		Level:         LevelInfo,
		Component:     ComponentCLI,
		Message:       "x < y",
		CorrelationID: "id",
	}
	_ = writeJSON(&buf, r)
	if strings.Contains(buf.String(), `\u003c`) {
		t.Fatalf("HTML escaping leaked: %q", buf.String())
	}
	if !strings.Contains(buf.String(), "x < y") {
		t.Fatalf("message not preserved: %q", buf.String())
	}
}

// --- Boundary: newline in a value is escaped ---

func TestWriteJSON_NewlineInValueIsEscaped(t *testing.T) {
	var buf bytes.Buffer
	r := Record{
		Ts:            time.Now().UTC(),
		Level:         LevelInfo,
		Component:     ComponentCLI,
		Message:       "x",
		CorrelationID: "id",
		Attrs:         map[string]any{"multiline": "a\nb"},
	}
	_ = writeJSON(&buf, r)
	// The record itself must be one line; the newline must be
	// escaped inside the JSON string.
	if strings.Count(buf.String(), "\n") != 1 {
		t.Fatalf("expected one line: %q", buf.String())
	}
	if !strings.Contains(buf.String(), `a\nb`) {
		t.Fatalf("newline not escaped: %q", buf.String())
	}
}

// --- TDD: timestamp format ---

func TestWriteJSON_TimestampIsRFC3339UTC(t *testing.T) {
	var buf bytes.Buffer
	ts := time.Date(2026, 10, 13, 10, 0, 0, 123456789, time.UTC)
	_ = writeJSON(&buf, Record{
		Ts:            ts,
		Level:         LevelInfo,
		Component:     ComponentCLI,
		Message:       "x",
		CorrelationID: "id",
	})
	if !strings.Contains(buf.String(),
		`"ts":"2026-10-13T10:00:00.123456789Z"`) {
		t.Fatalf("timestamp format: %q", buf.String())
	}
}

// --- TDD: field order ---

func TestWriteJSON_FieldOrder(t *testing.T) {
	var buf bytes.Buffer
	_ = writeJSON(&buf, Record{
		Ts:            time.Now().UTC(),
		Level:         LevelInfo,
		Component:     ComponentCLI,
		Message:       "x",
		CorrelationID: "id",
	})
	line := buf.String()
	pos := []string{`"ts":`, `"level":`, `"component":`, `"msg":`, `"correlation_id":`}
	prev := -1
	for _, key := range pos {
		i := strings.Index(line, key)
		if i < 0 {
			t.Fatalf("missing %q: %q", key, line)
		}
		if i < prev {
			t.Fatalf("field order wrong at %q: %q", key, line)
		}
		prev = i
	}
}
