package version_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/thapelomagqazana/forge/internal/version"
)

// --- WriteJSON --------------------------------------------------------------

// TestWriteJSON_WriteErrorPropagated asserts a failing writer returns
// its error.
func TestWriteJSON_WriteErrorPropagated(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	m := version.Metadata{Version: "v1.0.0", Commit: "abc", BuildDate: "2026-06-01T12:00:00Z", BuildType: "local"}
	err := version.WriteJSON(&failingWriter{err: boom}, m)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, boom) && !strings.Contains(err.Error(), "boom") {
		t.Errorf("error = %v, want boom", err)
	}
}

// --- ProvenanceToJSON -------------------------------------------------------

// TestProvenanceToJSON_ZeroTimeIsEmpty asserts a zero VCSTime becomes
// the empty string, which omitempty turns into an absent field.
func TestProvenanceToJSON_ZeroTimeIsEmpty(t *testing.T) {
	t.Parallel()
	out := version.ProvenanceToJSON(version.Provenance{})
	if out.VCSTime != "" {
		t.Errorf("VCSTime = %q, want empty", out.VCSTime)
	}
}

// TestProvenanceToJSON_TimeIsUTC asserts a non-UTC time is converted.
func TestProvenanceToJSON_TimeIsUTC(t *testing.T) {
	t.Parallel()
	loc := time.FixedZone("CEST", 2*60*60)
	tm := time.Date(2026, 5, 15, 11, 30, 0, 0, loc)
	out := version.ProvenanceToJSON(version.Provenance{VCSTime: tm})
	if out.VCSTime != "2026-05-15T09:30:00Z" {
		t.Errorf("VCSTime = %q, want 2026-05-15T09:30:00Z", out.VCSTime)
	}
}

// TestProvenanceToJSON_CopiesSlices asserts mutating the input after
// the conversion does not affect the output.
func TestProvenanceToJSON_CopiesSlices(t *testing.T) {
	t.Parallel()
	deps := []version.Dependency{{Path: "a"}, {Path: "b"}}
	settings := []version.Setting{{Key: "k", Value: "v"}}
	out := version.ProvenanceToJSON(version.Provenance{Dependencies: deps, Settings: settings})
	deps[0].Path = "mutated"
	settings[0].Value = "mutated"
	if out.Dependencies[0].Path != "a" {
		t.Errorf("Dependencies[0].Path = %q, want a", out.Dependencies[0].Path)
	}
	if out.Settings[0].Value != "v" {
		t.Errorf("Settings[0].Value = %q, want v", out.Settings[0].Value)
	}
}

// TestProvenanceToJSON_EmptySlicesAreNil asserts an empty input slice
// yields a nil output slice so omitempty can suppress the field.
func TestProvenanceToJSON_EmptySlicesAreNil(t *testing.T) {
	t.Parallel()
	out := version.ProvenanceToJSON(version.Provenance{})
	if out.Dependencies != nil {
		t.Errorf("Dependencies = %v, want nil", out.Dependencies)
	}
	if out.Settings != nil {
		t.Errorf("Settings = %v, want nil", out.Settings)
	}
}

// --- WriteFullJSON ----------------------------------------------------------

// TestWriteFullJSON_SingleLine asserts the output is one line.
func TestWriteFullJSON_SingleLine(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	m := version.Metadata{Version: "v1.0.0", Commit: "abc", BuildDate: "2026-06-01T12:00:00Z", BuildType: "release"}
	if err := version.WriteFullJSON(&buf, m, version.Provenance{}); err != nil {
		t.Fatal(err)
	}
	if strings.Count(buf.String(), "\n") != 1 {
		t.Fatalf("output has %d newlines, want 1:\n%s", strings.Count(buf.String(), "\n"), buf.String())
	}
}

// TestWriteFullJSON_FieldOrder asserts the five linker fields come
// first, followed by provenance.
func TestWriteFullJSON_FieldOrder(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	m := version.Metadata{Version: "v1.0.0", Commit: "abc", BuildDate: "2026-06-01T12:00:00Z", BuildType: "release"}
	p := version.Provenance{GoVersion: "go1.23.4", Module: "example.com/x", VCSModified: false}
	if err := version.WriteFullJSON(&buf, m, p); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	// The five linker fields must precede the provenance key.
	idxVersion := strings.Index(out, `"version":`)
	idxBuildType := strings.Index(out, `"buildType":`)
	idxProvenance := strings.Index(out, `"provenance":`)
	if idxVersion < 0 || idxBuildType < 0 || idxProvenance < 0 {
		t.Fatalf("output missing a key:\n%s", out)
	}
	if !(idxVersion < idxBuildType && idxBuildType < idxProvenance) {
		t.Errorf("field order wrong:\n%s", out)
	}
}

// TestWriteFullJSON_ValidJSON asserts the output round-trips.
func TestWriteFullJSON_ValidJSON(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	m := version.Metadata{Version: "v1.0.0", Commit: "abc", BuildDate: "2026-06-01T12:00:00Z", BuildType: "release"}
	p := version.Provenance{
		GoVersion: "go1.23.4",
		Module:    "example.com/x",
		Dependencies: []version.Dependency{
			{Path: "github.com/spf13/cobra", Version: "v1.8.1"},
		},
		Settings: []version.Setting{
			{Key: "-trimpath", Value: "true"},
		},
	}
	if err := version.WriteFullJSON(&buf, m, p); err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if _, ok := doc["provenance"]; !ok {
		t.Errorf("output has no provenance field:\n%s", buf.String())
	}
	prov, ok := doc["provenance"].(map[string]any)
	if !ok {
		t.Fatalf("provenance is not an object: %T", doc["provenance"])
	}
	if prov["goVersion"] != "go1.23.4" {
		t.Errorf("goVersion = %v", prov["goVersion"])
	}
}

// TestWriteFullJSON_OmitEmptyFields asserts empty optional fields are
// absent.
func TestWriteFullJSON_OmitEmptyFields(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	m := version.Metadata{Version: "v1.0.0", Commit: "abc", BuildDate: "2026-06-01T12:00:00Z", BuildType: "release"}
	if err := version.WriteFullJSON(&buf, m, version.Provenance{GoVersion: "go1.23.4", Module: "example.com/x"}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, absent := range []string{`"moduleVersion"`, `"vcsRevision"`, `"vcsTime"`, `"dependencies"`, `"settings"`} {
		if strings.Contains(out, absent) {
			t.Errorf("output contains %s:\n%s", absent, out)
		}
	}
	for _, present := range []string{`"goVersion"`, `"module"`, `"vcsModified"`} {
		if !strings.Contains(out, present) {
			t.Errorf("output missing %s:\n%s", present, out)
		}
	}
}

// TestWriteFullJSON_VCSModifiedPresentEvenWhenFalse asserts a false
// boolean is not omitted.
func TestWriteFullJSON_VCSModifiedPresentEvenWhenFalse(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	m := version.Metadata{Version: "v1.0.0", Commit: "abc", BuildDate: "2026-06-01T12:00:00Z", BuildType: "release"}
	if err := version.WriteFullJSON(&buf, m, version.Provenance{GoVersion: "go1.23.4", VCSModified: false}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"vcsModified":false`) {
		t.Errorf("vcsModified:false absent:\n%s", buf.String())
	}
}

// TestWriteFullJSON_Deterministic asserts two calls produce
// byte-identical output.
func TestWriteFullJSON_Deterministic(t *testing.T) {
	t.Parallel()
	m := version.Metadata{Version: "v1.0.0", Commit: "abc", BuildDate: "2026-06-01T12:00:00Z", BuildType: "release"}
	p := version.Provenance{GoVersion: "go1.23.4", Module: "example.com/x"}
	var a, b bytes.Buffer
	_ = version.WriteFullJSON(&a, m, p)
	_ = version.WriteFullJSON(&b, m, p)
	if a.String() != b.String() {
		t.Fatal("WriteFullJSON is not deterministic")
	}
}

// TestWriteFullJSON_WriteErrorPropagated asserts a failing writer
// returns its error.
func TestWriteFullJSON_WriteErrorPropagated(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	m := version.Metadata{Version: "v1.0.0", Commit: "abc", BuildDate: "2026-06-01T12:00:00Z", BuildType: "release"}
	err := version.WriteFullJSON(&failingWriter{err: boom}, m, version.Provenance{})
	if err == nil {
		t.Fatal("expected error")
	}
}

// failingWriter is a writer whose Write always returns an error. It
// is defined once in the version test package; if another test file
// defines it, keep only one definition.
type failingWriter struct{ err error }

func (w *failingWriter) Write(p []byte) (int, error) { return 0, w.err }
