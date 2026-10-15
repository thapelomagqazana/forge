// Package version contains unit tests for the version application
// service.
//
// This file pins the JSON output format. The shape is frozen by
// WBS 17.4.1: five fields (version, commit, buildDate, dirty,
// buildType), camelCase, in that order. A change to any field name,
// any JSON type, or the order fails a test in this file and
// requires a new ADR.
//
// # Test organisation
//
// The tests are grouped by category:
//
//   - Contract   — the shape of the frozen JSON schema.
//   - Positive   — well-formed inputs produce the expected output.
//   - Negative   — strict parsing, error propagation, unknown
//     formats.
//   - Edge       — unicode, quotes, control characters, very long
//     values.
//   - Dispatcher — FormatAs routes correctly.
//   - Golden     — the long-term byte-for-byte record.
//
// The tests do not construct a Cobra command. The service is pure;
// it is tested as a pure function. The handler is tested separately
// in internal/cli/version_test.go.
//
// # Relationship to the text format tests
//
// The text format (WBS 6.4.1) is tested in format_test.go. The two
// files are intentionally separate: the text format has runtime-
// dependent lines and needs normalisation for golden files; the
// JSON format does not. Keeping them in different files makes the
// difference visible.
package version

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// =============================================================================
// Contract — the frozen JSON schema
// =============================================================================

// TestWriteJSON_FullOutput pins the exact byte-for-byte output of
// the JSON encoder for a fully populated Info.
//
// This is the ATDD acceptance test for the WBS 17.4.1 shape. Every
// byte of the output is asserted; a change to the format fails this
// test and requires a new ADR.
//
// The field order is part of the contract: encoding/json emits
// struct fields in declaration order, and the library's Metadata
// declares the five fields in the schema's order (version, commit,
// buildDate, dirty, buildType).
func TestWriteJSON_FullOutput(t *testing.T) {
	t.Parallel()

	info := Info{
		Version:   "v1.2.3",
		Commit:    "abc1234",
		BuildDate: "2026-10-10T12:00:00Z",
		Dirty:     "false",
		BuildType: "release",
	}

	var buf bytes.Buffer
	if err := WriteJSON(&buf, info); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	want := `{"version":"v1.2.3","commit":"abc1234","buildDate":"2026-10-10T12:00:00Z","dirty":false,"buildType":"release"}` + "\n"

	if got := buf.String(); got != want {
		t.Errorf("output mismatch:\ngot:  %q\nwant: %q", got, want)
	}
}

// TestWriteJSON_FieldNames pins the five field names. The test
// decodes the output into a map and asserts that the keys are
// exactly the schema's five.
//
// This catches a renaming of a field (for example, buildDate to
// build_date, or buildType to build_type) even if the value is
// otherwise correct.
func TestWriteJSON_FieldNames(t *testing.T) {
	t.Parallel()

	info := Info{
		Version:   "v1.2.3",
		Commit:    "abc1234",
		BuildDate: "2026-10-10T12:00:00Z",
		Dirty:     "true",
		BuildType: "release",
	}

	var buf bytes.Buffer
	if err := WriteJSON(&buf, info); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}

	want := []string{"version", "commit", "buildDate", "dirty", "buildType"}
	if len(m) != len(want) {
		t.Errorf("field count: got %d, want %d\noutput: %s",
			len(m), len(want), buf.String())
	}
	for _, k := range want {
		if _, ok := m[k]; !ok {
			t.Errorf("field %q missing from output\noutput: %s", k, buf.String())
		}
	}
	for k := range m {
		found := false
		for _, w := range want {
			if k == w {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("unexpected field %q in output\noutput: %s", k, buf.String())
		}
	}
}

// TestWriteJSON_DirtyIsBoolean pins the type of the `dirty` field.
//
// The model stores Dirty as a string (WBS 6.1.1); the schema
// requires a boolean. The test decodes the output and asserts that
// the decoded value's Go type is bool, not string.
func TestWriteJSON_DirtyIsBoolean(t *testing.T) {
	t.Parallel()

	info := Info{Dirty: "true"}

	var buf bytes.Buffer
	if err := WriteJSON(&buf, info); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	v, ok := m["dirty"]
	if !ok {
		t.Fatalf("field 'dirty' missing\noutput: %s", buf.String())
	}
	if _, isBool := v.(bool); !isBool {
		t.Errorf("field 'dirty' is %T, want bool\noutput: %s", v, buf.String())
	}
}

// TestWriteJSON_BuildTypeIsString pins the type of the `buildType`
// field and asserts the three defined values round-trip. An
// unrecognized value in the Info is normalized to "local" by the
// service's Metadata conversion.
func TestWriteJSON_BuildTypeIsString(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want string
	}{
		{"release", "release"},
		{"ci", "ci"},
		{"local", "local"},
		{"", "local"},        // empty normalizes to local
		{"unknown", "local"}, // unrecognized normalizes to local
	}

	for _, tc := range cases {
		tc := tc
		t.Run("in="+tc.in, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			if err := WriteJSON(&buf, Info{BuildType: tc.in}); err != nil {
				t.Fatalf("WriteJSON: %v", err)
			}

			var m map[string]any
			if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
				t.Fatalf("output is not valid JSON: %v", err)
			}
			got, ok := m["buildType"].(string)
			if !ok {
				t.Fatalf("field 'buildType' is %T, want string\noutput: %s",
					m["buildType"], buf.String())
			}
			if got != tc.want {
				t.Errorf("buildType: got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestWriteJSON_IsSingleLine pins the "single line, terminated by
// \n" contract.
func TestWriteJSON_IsSingleLine(t *testing.T) {
	t.Parallel()

	info := Info{
		Version:   "v1.2.3",
		Commit:    "abc1234",
		BuildDate: "2026-10-10T12:00:00Z",
		Dirty:     "false",
		BuildType: "release",
	}

	var buf bytes.Buffer
	if err := WriteJSON(&buf, info); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	out := buf.String()

	if n := strings.Count(out, "\n"); n != 1 {
		t.Errorf("output contains %d newlines, want 1\noutput: %q", n, out)
	}
	if !strings.HasSuffix(out, "\n") {
		t.Errorf("output does not end in a newline: %q", out)
	}
}

// =============================================================================
// Positive — well-formed inputs
// =============================================================================

// TestWriteJSON_AllFieldsPopulated asserts that every field is
// present and correct when every field is populated.
func TestWriteJSON_AllFieldsPopulated(t *testing.T) {
	t.Parallel()

	info := Info{
		Version:   "v1.2.3",
		Commit:    "abc1234",
		BuildDate: "2026-10-10T12:00:00Z",
		Dirty:     "true",
		BuildType: "release",
	}

	var buf bytes.Buffer
	if err := WriteJSON(&buf, info); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if got["version"] != "v1.2.3" {
		t.Errorf("version: got %v, want v1.2.3", got["version"])
	}
	if got["commit"] != "abc1234" {
		t.Errorf("commit: got %v, want abc1234", got["commit"])
	}
	if got["buildDate"] != "2026-10-10T12:00:00Z" {
		t.Errorf("buildDate: got %v, want 2026-10-10T12:00:00Z", got["buildDate"])
	}
	if got["dirty"] != true {
		t.Errorf("dirty: got %v, want true", got["dirty"])
	}
	if got["buildType"] != "release" {
		t.Errorf("buildType: got %v, want release", got["buildType"])
	}
}

// TestWriteJSON_AllFieldsEmpty asserts that a zero-value Info
// produces a valid JSON object with all five keys present and
// their zero values.
//
// This is the uninstrumented-build case. The keys must be present
// even when their values are the empty string or false; a consumer
// that reads `obj.commit` should never see `undefined`. The
// buildType field is normalized to "local" for an empty input.
func TestWriteJSON_AllFieldsEmpty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := WriteJSON(&buf, Info{}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	want := `{"version":"","commit":"","buildDate":"","dirty":false,"buildType":"local"}` + "\n"

	if got := buf.String(); got != want {
		t.Errorf("output mismatch:\ngot:  %q\nwant: %q", got, want)
	}
}

// =============================================================================
// Negative — strict parsing and error propagation
// =============================================================================

// TestWriteJSON_DirtyParsingRuleIsStrict verifies that only the
// literal "true" in the model renders as JSON `true`. Every other
// value — including "1", "TRUE", "yes", and the empty string —
// renders as JSON `false`.
func TestWriteJSON_DirtyParsingRuleIsStrict(t *testing.T) {
	t.Parallel()

	cases := []struct {
		raw  string
		want bool
	}{
		{"true", true},
		{"false", false},
		{"1", false},      // negative
		{"TRUE", false},   // negative
		{"yes", false},    // negative
		{"", false},       // boundary: empty
		{" true", false},  // boundary: leading space
		{"true ", false},  // boundary: trailing space
		{"true\n", false}, // boundary: trailing newline
	}

	for _, tc := range cases {
		tc := tc
		t.Run("raw="+tc.raw, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			if err := WriteJSON(&buf, Info{Dirty: tc.raw}); err != nil {
				t.Fatalf("WriteJSON: %v", err)
			}

			var got map[string]any
			if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
				t.Fatalf("output is not valid JSON: %v", err)
			}
			if got["dirty"] != tc.want {
				t.Errorf("Dirty=%q → JSON %v, want %v", tc.raw, got["dirty"], tc.want)
			}
		})
	}
}

// TestWriteJSON_WriterErrorIsPropagated verifies that WriteJSON
// returns the writer's error, wrapped.
func TestWriteJSON_WriterErrorIsPropagated(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("simulated write failure")
	w := &failingWriter{err: wantErr}

	err := WriteJSON(w, Info{Version: "v1.2.3"})
	if !errors.Is(err, wantErr) {
		t.Errorf("WriteJSON: got %v, want %v", err, wantErr)
	}
}

// =============================================================================
// Edge — unusual but valid inputs
// =============================================================================

// TestWriteJSON_UnicodeIsPreserved verifies that non-ASCII values
// round-trip through the encoder.
func TestWriteJSON_UnicodeIsPreserved(t *testing.T) {
	t.Parallel()

	info := Info{Version: "1.0.0-α"}

	var buf bytes.Buffer
	if err := WriteJSON(&buf, info); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if got["version"] != "1.0.0-α" {
		t.Errorf("version round-trip: got %v, want %q", got["version"], "1.0.0-α")
	}
}

// TestWriteJSON_QuotesAreEscaped verifies that a value containing
// a double quote is escaped by encoding/json.
func TestWriteJSON_QuotesAreEscaped(t *testing.T) {
	t.Parallel()

	info := Info{Version: `v1.2.3"quoted"`}

	var buf bytes.Buffer
	if err := WriteJSON(&buf, info); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}
	if got["version"] != `v1.2.3"quoted"` {
		t.Errorf("version round-trip: got %v, want %q", got["version"], `v1.2.3"quoted"`)
	}
}

// TestWriteJSON_ControlCharactersAreEscaped verifies that a value
// containing a newline is escaped. The result must remain a
// single-line JSON object.
func TestWriteJSON_ControlCharactersAreEscaped(t *testing.T) {
	t.Parallel()

	info := Info{Version: "v1.2.3\nmalicious"}

	var buf bytes.Buffer
	if err := WriteJSON(&buf, info); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	out := buf.String()
	if n := strings.Count(out, "\n"); n != 1 {
		t.Errorf("output contains %d newlines, want 1\noutput: %q", n, out)
	}

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if got["version"] != "v1.2.3\nmalicious" {
		t.Errorf("version round-trip: got %v, want %q", got["version"], "v1.2.3\nmalicious")
	}
}

// TestWriteJSON_VeryLongValueDoesNotPanic verifies that a very long
// value does not cause the encoder to panic or truncate.
func TestWriteJSON_VeryLongValueDoesNotPanic(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("v", 100_000)
	info := Info{Version: long}

	var buf bytes.Buffer
	if err := WriteJSON(&buf, info); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	v, _ := got["version"].(string)
	if len(v) != len(long) {
		t.Errorf("version length: got %d, want %d", len(v), len(long))
	}
}

// =============================================================================
// Dispatcher — FormatAs routes correctly
// =============================================================================

// TestFormatAs_TextDispatchesToWriteText verifies that FormatAs
// with FormatText produces the same output as WriteText.
func TestFormatAs_TextDispatchesToWriteText(t *testing.T) {
	t.Parallel()

	info := Info{Version: "v1.2.3", Commit: "abc1234"}

	var a, b bytes.Buffer
	if err := WriteText(&a, info); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	if err := FormatAs(&b, info, FormatText); err != nil {
		t.Fatalf("FormatAs(text): %v", err)
	}

	if a.String() != b.String() {
		t.Errorf("FormatAs(text) differs from WriteText:\nWriteText: %q\nFormatAs:  %q",
			a.String(), b.String())
	}
}

// TestFormatAs_JSONDispatchesToWriteJSON verifies that FormatAs
// with FormatJSON produces the same output as WriteJSON.
func TestFormatAs_JSONDispatchesToWriteJSON(t *testing.T) {
	t.Parallel()

	info := Info{Version: "v1.2.3", Commit: "abc1234"}

	var a, b bytes.Buffer
	if err := WriteJSON(&a, info); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	if err := FormatAs(&b, info, FormatJSON); err != nil {
		t.Fatalf("FormatAs(json): %v", err)
	}

	if a.String() != b.String() {
		t.Errorf("FormatAs(json) differs from WriteJSON:\nWriteJSON: %q\nFormatAs:  %q",
			a.String(), b.String())
	}
}

// TestFormatAs_UnknownFormat verifies that an unknown format
// returns an error that wraps ErrUnknownFormat and does not write
// anything to the writer.
func TestFormatAs_UnknownFormat(t *testing.T) {
	t.Parallel()

	cases := []string{
		"",
		"yaml",
		"JSON",  // case-sensitive
		"text ", // trailing space
		" json", // leading space
	}

	for _, format := range cases {
		format := format
		t.Run("format="+format, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			err := FormatAs(&buf, Info{}, format)
			if err == nil {
				t.Fatalf("FormatAs(%q) = nil, want error", format)
			}
			if !errors.Is(err, ErrUnknownFormat) {
				t.Errorf("FormatAs(%q) = %v, want ErrUnknownFormat", format, err)
			}
			if buf.Len() != 0 {
				t.Errorf("FormatAs(%q) wrote %d bytes before failing: %q",
					format, buf.Len(), buf.String())
			}
		})
	}
}

// =============================================================================
// Golden files — the long-term record of the JSON schema
// =============================================================================

// TestWriteJSON_Golden compares the JSON encoder's output against
// golden files under testdata/format/.
//
// The golden files are the long-term record of the JSON schema. A
// change to the schema requires updating the golden files, opening
// an ADR, and citing WBS 17.4.1 in the commit message.
//
// The JSON format has no runtime-dependent line, so the golden
// files contain the exact bytes the encoder produces.
//
// # Regenerating the golden files
//
// Run with the package-level -update flag:
//
//	go test ./internal/app/version/ -run TestWriteJSON_Golden -update
//
// The flag causes each subtest to write its actual output to the
// golden file instead of comparing. Inspect the diff after the
// command; the files are expected to change only when the schema
// changed. A change to a golden file without a corresponding change
// to the encoder, the schema, or the ADR is a defect.
//
// # Why the flag is package-level
//
// Go registers flags per test binary, not per test function. Every
// golden test in this package reads the same -update flag (declared
// in testhelper_test.go). The flag's name and semantics match the
// ones used in internal/arch and internal/forgeerr.
//
// # Why the test is parallel
//
// The subtests read the golden files but do not share state with
// each other. The only write happens under -update, and the test
// runner serializes writes to distinct files by subtest name; two
// subtests never write the same file. Marking the test parallel is
// safe and keeps the suite fast.
func TestWriteJSON_Golden(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		info   Info
		golden string
	}{
		{
			name:   "empty",
			info:   Info{},
			golden: "testdata/format/empty.json.golden",
		},
		{
			name: "full",
			info: Info{
				Version:   "v1.2.3",
				Commit:    "abc1234",
				BuildDate: "2026-10-10T12:00:00Z",
				Dirty:     "false",
				BuildType: "release",
			},
			golden: "testdata/format/full.json.golden",
		},
		{
			name: "dirty",
			info: Info{
				Version:   "v1.2.3-4-gabc1234-dirty",
				Commit:    "abc1234",
				BuildDate: "2026-10-10T12:00:00Z",
				Dirty:     "true",
				BuildType: "local",
			},
			golden: "testdata/format/dirty.json.golden",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			if err := WriteJSON(&buf, tc.info); err != nil {
				t.Fatalf("WriteJSON: %v", err)
			}

			got := buf.String()

			// Under -update, write the actual output to the
			// golden file and stop. The write is atomic from the
			// test runner's perspective: each subtest writes a
			// distinct file, and no other subtest reads the file
			// it writes.
			if *update {
				writeGolden(t, tc.golden, got)
				return
			}

			want := readGolden(t, tc.golden)

			if got != want {
				t.Errorf("golden mismatch for %s:\ngot:  %q\nwant: %q\n"+
					"if the change is intentional, regenerate the golden "+
					"file with `go test ./internal/app/version/ "+
					"-run TestWriteJSON_Golden -update`, open an ADR for "+
					"the schema change, and cite WBS 17.4.1 in the commit "+
					"message",
					tc.golden, got, want)
			}
		})
	}
}
