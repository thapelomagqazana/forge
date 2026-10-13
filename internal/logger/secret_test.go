package logger

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// TDD: the four serialization hooks
// ---------------------------------------------------------------------------

func TestSecret_String(t *testing.T) {
	s := NewSecret("hunter2")
	if got := s.String(); got != redactedLiteral {
		t.Fatalf("String() = %q; want %q", got, redactedLiteral)
	}
}

func TestSecret_ZeroValue_String(t *testing.T) {
	var s Secret
	if got := s.String(); got != redactedLiteral {
		t.Fatalf("zero Secret String() = %q; want %q",
			got, redactedLiteral)
	}
}

func TestSecret_FmtSprintf(t *testing.T) {
	s := NewSecret("hunter2")
	if got := fmt.Sprintf("%s", s); strings.Contains(got, "hunter2") {
		t.Fatalf("Sprintf leaked the secret: %q", got)
	}
	if got := fmt.Sprintf("%v", s); strings.Contains(got, "hunter2") {
		t.Fatalf("Sprintf %%v leaked the secret: %q", got)
	}
}

func TestSecret_FmtGoString(t *testing.T) {
	s := NewSecret("hunter2")
	got := fmt.Sprintf("%#v", s)
	if strings.Contains(got, "hunter2") {
		t.Fatalf("%%#v leaked the secret: %q", got)
	}
	if !strings.Contains(got, redactedLiteral) {
		t.Fatalf("%%#v lost the redaction marker: %q", got)
	}
}

func TestSecret_LogValue(t *testing.T) {
	s := NewSecret("hunter2")
	v := s.LogValue()
	if v.Kind() != slog.KindString {
		t.Fatalf("LogValue kind = %v; want string", v.Kind())
	}
	if v.String() != redactedLiteral {
		t.Fatalf("LogValue = %q; want %q", v.String(), redactedLiteral)
	}
}

func TestSecret_MarshalJSON(t *testing.T) {
	s := NewSecret("hunter2")
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(b), "hunter2") {
		t.Fatalf("MarshalJSON leaked the secret: %q", b)
	}
	if string(b) != `"`+redactedLiteral+`"` {
		t.Fatalf("MarshalJSON = %q; want %q",
			string(b), `"`+redactedLiteral+`"`)
	}
}

func TestSecret_MarshalJSON_InStruct(t *testing.T) {
	type Credentials struct {
		User  string `json:"user"`
		Token Secret `json:"token"`
	}
	c := Credentials{User: "alice", Token: NewSecret("hunter2")}
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(b), "hunter2") {
		t.Fatalf("struct marshal leaked the secret: %q", b)
	}
	if !strings.Contains(string(b), redactedLiteral) {
		t.Fatalf("struct marshal lost the redaction marker: %q", b)
	}
}

// ---------------------------------------------------------------------------
// Reveal and HasValue
// ---------------------------------------------------------------------------

func TestSecret_Reveal(t *testing.T) {
	s := NewSecret("hunter2")
	if got := s.Reveal(); got != "hunter2" {
		t.Fatalf("Reveal() = %q; want %q", got, "hunter2")
	}
}

func TestSecret_HasValue(t *testing.T) {
	if !NewSecret("x").HasValue() {
		t.Fatal("HasValue() = false; want true")
	}
	if NewSecret("").HasValue() {
		t.Fatal("HasValue() on empty = true; want false")
	}
	if (Secret{}).HasValue() {
		t.Fatal("HasValue() on zero = true; want false")
	}
}

func TestSecret_Equal(t *testing.T) {
	a := NewSecret("x")
	b := NewSecret("x")
	c := NewSecret("y")
	if !a.Equal(b) {
		t.Fatal("Equal() = false for equal values; want true")
	}
	if a.Equal(c) {
		t.Fatal("Equal() = true for different values; want false")
	}
	if !(Secret{}).Equal(NewSecret("")) {
		t.Fatal("zero and empty should be equal")
	}
}

// ---------------------------------------------------------------------------
// ATDD: slog emission
// ---------------------------------------------------------------------------

func TestSecret_SlogEmission_HumanFormat(t *testing.T) {
	var buf strings.Builder
	h := slog.NewTextHandler(&buf, &slog.HandlerOptions{})
	sl := slog.New(h)
	sl.Info("auth", "token", NewSecret("hunter2"))

	out := buf.String()
	if strings.Contains(out, "hunter2") {
		t.Fatalf("slog human handler leaked the secret: %q", out)
	}
	if !strings.Contains(out, redactedLiteral) {
		t.Fatalf("slog human handler lost the marker: %q", out)
	}
}

func TestSecret_SlogEmission_JSONFormat(t *testing.T) {
	var buf strings.Builder
	h := slog.NewJSONHandler(&buf, &slog.HandlerOptions{})
	sl := slog.New(h)
	sl.Info("auth", "token", NewSecret("hunter2"))

	out := buf.String()
	if strings.Contains(out, "hunter2") {
		t.Fatalf("slog JSON handler leaked the secret: %q", out)
	}
	if !strings.Contains(out, redactedLiteral) {
		t.Fatalf("slog JSON handler lost the marker: %q", out)
	}
}
