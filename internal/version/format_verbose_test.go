package version_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/thapelomagqazana/forge/internal/version"
)

// TestWriteVerbose_ContainsAllSections asserts the verbose output has
// the three sections in the documented order.
func TestWriteVerbose_ContainsAllSections(t *testing.T) {
	t.Parallel()
	m := version.Metadata{
		Version: "v1.0.0", Commit: "abc1234",
		BuildDate: "2026-06-01T12:00:00Z", Dirty: false, BuildType: "release",
	}
	p := version.Provenance{GoVersion: "go1.23.4", Module: "example.com/x"}
	var buf bytes.Buffer
	if err := version.WriteVerbose(&buf, m, p); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"version:", "commit:", "buildDate:", "dirty:", "buildType:", "go:", "module:"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

// TestWriteVerbose_EmptyFieldsUseDash asserts an empty top-level
// value renders as "-", not as a blank line.
//
// # The label column
//
// The verbose format pads the label column to a fixed width so the
// values align. The width is determined by the longest label
// ("buildDate:" is ten characters), plus one space. The value
// column therefore starts at offset eleven.
//
// The expected substrings below account for the padding. A change
// to the label width requires updating this test in the same
// commit as the formatter; the two must stay in sync.
func TestWriteVerbose_EmptyFieldsUseDash(t *testing.T) {
	t.Parallel()
	m := version.Metadata{
		Version:   "v1.0.0",
		Commit:    "abc1234",
		BuildDate: "2026-06-01T12:00:00Z",
		BuildType: "local",
	}
	var buf bytes.Buffer
	if err := version.WriteVerbose(&buf, m, version.Provenance{}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	// Each empty top-level field must render as a dash, not as a
	// blank value. The substrings include the label and the padding
	// up to the value column.
	for _, want := range []string{
		"go:        -",
		"module:    -",
		"vcs:       -",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}

	// The check above would also pass if the line were, for
	// example, "go:        -foo". Assert the dash is followed by
	// a newline (the value is exactly "-").
	for _, want := range []string{
		"go:        -\n",
		"module:    -\n",
		"vcs:       -\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

// TestWriteVerbose_DependenciesSorted asserts dependencies appear in
// sorted order regardless of input order.
func TestWriteVerbose_DependenciesSorted(t *testing.T) {
	t.Parallel()
	m := version.Metadata{Version: "v1.0.0", Commit: "a", BuildDate: "2026-06-01T12:00:00Z", BuildType: "local"}
	p := version.Provenance{
		Dependencies: []version.Dependency{
			{Path: "z.example", Version: "v1"},
			{Path: "a.example", Version: "v1"},
		},
	}
	var buf bytes.Buffer
	if err := version.WriteVerbose(&buf, m, p); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Index(out, "a.example") > strings.Index(out, "z.example") {
		t.Errorf("dependencies not sorted:\n%s", out)
	}
}

// TestWriteVerbose_Deterministic asserts two calls produce identical
// output.
func TestWriteVerbose_Deterministic(t *testing.T) {
	t.Parallel()
	m := version.Metadata{Version: "v1.0.0", Commit: "a", BuildDate: "2026-06-01T12:00:00Z", BuildType: "local"}
	var a, b bytes.Buffer
	_ = version.WriteVerbose(&a, m, version.Provenance{})
	_ = version.WriteVerbose(&b, m, version.Provenance{})
	if a.String() != b.String() {
		t.Fatal("verbose output is not deterministic")
	}
}
