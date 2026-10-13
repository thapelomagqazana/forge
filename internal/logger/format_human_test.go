package logger

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestWriteHuman_BasicShape(t *testing.T) {
	var buf bytes.Buffer
	r := Record{
		Ts:            time.Now(),
		Level:         LevelInfo,
		Component:     ComponentCLI,
		Message:       "loaded configuration",
		CorrelationID: "x",
		Attrs:         map[string]any{"path": "/etc/forge/config.yaml"},
	}
	if err := writeHuman(&buf, r); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.HasPrefix(got, "INFO  [cli] loaded configuration ") {
		t.Fatalf("prefix mismatch: %q", got)
	}
	if !strings.Contains(got, "path=/etc/forge/config.yaml") {
		t.Fatalf("attr missing: %q", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Fatalf("missing trailing newline: %q", got)
	}
	if strings.Count(got, "\n") != 1 {
		t.Fatalf("expected exactly one newline: %q", got)
	}
}

func TestWriteHuman_LevelPadding(t *testing.T) {
	var buf bytes.Buffer
	_ = writeHuman(&buf, Record{Level: LevelDebug, Component: ComponentCLI, Message: "x"})
	if !strings.HasPrefix(buf.String(), "DEBUG [cli] x") {
		t.Fatalf("debug prefix: %q", buf.String())
	}

	buf.Reset()
	_ = writeHuman(&buf, Record{Level: LevelError, Component: ComponentCLI, Message: "x"})
	if !strings.HasPrefix(buf.String(), "ERROR [cli] x") {
		t.Fatalf("error prefix: %q", buf.String())
	}
}

func TestWriteHuman_SortsAttrsByKey(t *testing.T) {
	var buf bytes.Buffer
	r := Record{
		Level:     LevelInfo,
		Component: ComponentCLI,
		Message:   "x",
		Attrs: map[string]any{
			"z": 1,
			"a": 2,
			"m": 3,
		},
	}
	_ = writeHuman(&buf, r)
	got := buf.String()
	ia := strings.Index(got, "a=2")
	im := strings.Index(got, "m=3")
	iz := strings.Index(got, "z=1")
	if !(ia < im && im < iz) {
		t.Fatalf("attrs not sorted: %q", got)
	}
}

// --- Boundary: quoting ---

func TestWriteHuman_QuotesValuesWithWhitespace(t *testing.T) {
	var buf bytes.Buffer
	r := Record{
		Level:     LevelInfo,
		Component: ComponentCLI,
		Message:   "x",
		Attrs:     map[string]any{"msg": "hello world"},
	}
	_ = writeHuman(&buf, r)
	if !strings.Contains(buf.String(), `msg="hello world"`) {
		t.Fatalf("not quoted: %q", buf.String())
	}
}

func TestWriteHuman_DoesNotQuoteValuesWithoutWhitespace(t *testing.T) {
	var buf bytes.Buffer
	r := Record{
		Level:     LevelInfo,
		Component: ComponentCLI,
		Message:   "x",
		Attrs:     map[string]any{"path": "/tmp/out.go"},
	}
	_ = writeHuman(&buf, r)
	if !strings.Contains(buf.String(), `path=/tmp/out.go`) {
		t.Fatalf("unexpectedly quoted: %q", buf.String())
	}
}

// --- Corner: nil attr value ---

func TestWriteHuman_NilValue(t *testing.T) {
	var buf bytes.Buffer
	r := Record{
		Level:     LevelInfo,
		Component: ComponentCLI,
		Message:   "x",
		Attrs:     map[string]any{"k": nil},
	}
	_ = writeHuman(&buf, r)
	if !strings.Contains(buf.String(), "k=null") {
		t.Fatalf("nil not rendered as null: %q", buf.String())
	}
}

// --- Corner: error attr value ---

func TestWriteHuman_ErrorValue(t *testing.T) {
	var buf bytes.Buffer
	r := Record{
		Level:     LevelError,
		Component: ComponentFilesystem,
		Message:   "write failed",
		Attrs:     map[string]any{"err": errSentinel{}},
	}
	_ = writeHuman(&buf, r)
	if !strings.Contains(buf.String(), "err=boom") {
		t.Fatalf("error not rendered: %q", buf.String())
	}
}

type errSentinel struct{}

func (errSentinel) Error() string { return "boom" }

// --- Edge: no attrs ---

func TestWriteHuman_NoAttrs(t *testing.T) {
	var buf bytes.Buffer
	r := Record{
		Level:     LevelInfo,
		Component: ComponentCLI,
		Message:   "hello",
	}
	_ = writeHuman(&buf, r)
	want := "INFO  [cli] hello\n"
	if buf.String() != want {
		t.Fatalf("got %q; want %q", buf.String(), want)
	}
}
