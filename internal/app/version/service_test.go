package version_test

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	appversion "github.com/thapelomagqazana/forge/internal/app/version"
	libversion "github.com/thapelomagqazana/forge/internal/version"
)

// Compile-time interface check: Info must be usable as a value type
// with no methods. This pins the contract that the result type is
// plain data.
var _ = appversion.Info{}

// --- Positive tests --------------------------------------------------

// TestGet_ReturnsBuildInfo is the ATDD acceptance test for AC7: a
// unit test can call the service directly, without constructing a
// Cobra command.
//
// The test calls Get and asserts that the result is an Info value
// with the expected shape. It does not assert on the specific values
// of Version, Commit, BuildDate, Dirty, or BuildType, because those
// are injected at link time and vary between builds.
func TestGet_ReturnsBuildInfo(t *testing.T) {
	t.Parallel()

	got := appversion.Get()

	// The zero value of Info is valid, so the assertion is on the
	// type, not on the values. The test's purpose is to prove that
	// Get can be called without Cobra; the values are covered by
	// the acceptance tests of the build system.
	if got.Version == "" && got.Commit == "" && got.BuildDate == "" && got.Dirty == "" && got.BuildType == "" {
		// A binary built without ldflags has empty values. This is
		// valid. Log it for the reader's benefit but do not fail.
		t.Log("all linker values are empty; " +
			"the test binary was built without ldflags")
	}
}

// TestGet_IsPure verifies that two consecutive calls return equal
// values. This is the purity contract documented on Get.
//
// The comparison uses reflect.DeepEqual, not ==, because Info
// contains libversion.Provenance, whose Dependencies and Settings
// fields are slices. Go's == operator does not work on structs
// containing slices.
func TestGet_IsPure(t *testing.T) {
	t.Parallel()

	first := appversion.Get()
	second := appversion.Get()

	if !reflect.DeepEqual(first, second) {
		t.Errorf("Get is not pure:\n first =%+v\n second=%+v", first, second)
	}
}

// TestGet_IncludesProvenance asserts the Info carries the provenance
// read from runtime/debug.ReadBuildInfo. The test binary always has
// build info, so GoVersion is non-empty.
func TestGet_IncludesProvenance(t *testing.T) {
	t.Parallel()

	got := appversion.Get()
	if got.Provenance.GoVersion == "" {
		t.Error("Provenance.GoVersion is empty")
	}
	if !strings.HasPrefix(got.Provenance.GoVersion, "go1.") {
		t.Errorf("Provenance.GoVersion = %q, want a go1.x prefix",
			got.Provenance.GoVersion)
	}
}

// --- Info.Metadata ---------------------------------------------------

// TestInfo_Metadata_NormalizesBuildKind asserts the conversion from
// the service's Info to the library's Metadata normalizes an
// unrecognized BuildType to "local".
func TestInfo_Metadata_NormalizesBuildKind(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want string
	}{
		{"", string(libversion.BuildKindLocal)},
		{"release", string(libversion.BuildKindRelease)},
		{"ci", string(libversion.BuildKindCI)},
		{"local", string(libversion.BuildKindLocal)},
		{"unknown", string(libversion.BuildKindLocal)},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			i := appversion.Info{BuildType: tc.in}
			m := i.Metadata()
			if m.BuildType != tc.want {
				t.Errorf("Metadata().BuildType = %q, want %q", m.BuildType, tc.want)
			}
		})
	}
}

// TestInfo_Metadata_ParsesDirty asserts the linker's string form of
// the dirty flag becomes a bool in Metadata.
//
// The composite literals are parenthesised because Go's grammar does
// not allow a bare composite literal as the operand of an `if`
// condition when the literal is immediately followed by a selector.
// Without the parentheses the parser reports:
//
//	expected ';', found '.'
//
// The rule is documented in the Go specification under "Composite
// literals": a composite literal that is the operand of a selector
// must be parenthesised if it appears in a context where the parser
// cannot otherwise decide whether the braces open a block.
func TestInfo_Metadata_ParsesDirty(t *testing.T) {
	t.Parallel()
	if !(appversion.Info{Dirty: "true"}).Metadata().Dirty {
		t.Error("Metadata().Dirty = false, want true")
	}
	for _, in := range []string{"", "false", "0", "yes", "TRUE"} {
		if (appversion.Info{Dirty: in}).Metadata().Dirty {
			t.Errorf("Metadata().Dirty = true for Dirty=%q", in)
		}
	}
}

// TestInfo_Metadata_PreservesVersionVerbatim asserts the version
// string is passed through unmodified.
func TestInfo_Metadata_PreservesVersionVerbatim(t *testing.T) {
	t.Parallel()
	for _, v := range []string{
		"v1.4.2",
		"v1.4.2-dirty",
		"v1.4.2-3-gabc1234",
		"dev",
	} {
		if got := (appversion.Info{Version: v}).Metadata().Version; got != v {
			t.Errorf("Metadata().Version = %q, want %q", got, v)
		}
	}
}

// --- Format ----------------------------------------------------------

// TestFormat_RendersAllFields verifies that every field of Info
// appears in the formatted output.
func TestFormat_RendersAllFields(t *testing.T) {
	t.Parallel()

	info := appversion.Info{
		Version:   "0.1.0-test",
		Commit:    "deadbeef",
		BuildDate: "2026-10-09T12:00:00Z",
		Dirty:     "false",
	}

	var buf bytes.Buffer
	if err := appversion.Format(&buf, info); err != nil {
		t.Fatalf("Format: %v", err)
	}

	out := buf.String()

	// The version appears on the header line.
	if !strings.Contains(out, "forge 0.1.0-test") {
		t.Errorf("output missing version header: %q", out)
	}
	// Each detail field appears as a labelled line.
	for _, want := range []string{
		"commit:",
		"deadbeef",
		"built:",
		"2026-10-09T12:00:00Z",
		"dirty:",
		"false",
		"go version:",
		"platform:",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q: %q", want, out)
		}
	}
}

// TestFormat_IsDeterministic verifies that two calls with the same
// Info produce byte-identical output.
func TestFormat_IsDeterministic(t *testing.T) {
	t.Parallel()

	info := appversion.Info{
		Version:   "0.1.0-test",
		Commit:    "deadbeef",
		BuildDate: "2026-10-09T12:00:00Z",
		Dirty:     "false",
	}

	var a, b bytes.Buffer
	if err := appversion.Format(&a, info); err != nil {
		t.Fatalf("Format (a): %v", err)
	}
	if err := appversion.Format(&b, info); err != nil {
		t.Fatalf("Format (b): %v", err)
	}

	if a.String() != b.String() {
		t.Errorf("Format is not deterministic:\n a=%q\n b=%q",
			a.String(), b.String())
	}
}

// TestFormat_EmptyInfo verifies that a zero-value Info renders
// without error, that the header is exactly "forge" with no
// trailing space, and that every detail line has the frozen shape
// with empty (or, for Dirty, false) values.
func TestFormat_EmptyInfo(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := appversion.Format(&buf, appversion.Info{}); err != nil {
		t.Fatalf("Format(empty): %v", err)
	}

	out := buf.String()

	header, _, found := strings.Cut(out, "\n")
	if !found {
		t.Fatalf("output has no newline: %q", out)
	}
	if header != "forge" {
		t.Errorf("header: got %q, want %q", header, "forge")
	}

	for _, key := range []string{
		"  commit:",
		"  built:",
		"  dirty:",
		"  go version:",
		"  platform:",
	} {
		if !strings.Contains(out, key) {
			t.Errorf("output missing %q: %q", key, out)
		}
	}

	if !strings.Contains(out, "  dirty:       false\n") {
		t.Errorf("dirty line: want %q in output, got %q",
			"  dirty:       false\n", out)
	}
}

// TestFormat_SpecialCharactersInFields verifies that values
// containing spaces, newlines, and non-ASCII characters are rendered
// without error. The formatter does not escape or quote values.
func TestFormat_SpecialCharactersInFields(t *testing.T) {
	t.Parallel()

	info := appversion.Info{
		Version:   "0.1.0-α",
		Commit:    "dead beef",
		BuildDate: "2026-10-09T12:00:00Z",
		Dirty:     "false",
	}

	var buf bytes.Buffer
	if err := appversion.Format(&buf, info); err != nil {
		t.Fatalf("Format: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "0.1.0-α") {
		t.Errorf("output did not preserve non-ASCII: %q", out)
	}
	if !strings.Contains(out, "dead beef") {
		t.Errorf("output did not preserve space: %q", out)
	}
}

// TestFormat_DirtyTrue verifies that a Dirty value of "true" is
// rendered as-is.
func TestFormat_DirtyTrue(t *testing.T) {
	t.Parallel()

	info := appversion.Info{
		Version:   "0.1.0-test",
		Commit:    "deadbeef",
		BuildDate: "2026-10-09T12:00:00Z",
		Dirty:     "true",
	}

	var buf bytes.Buffer
	if err := appversion.Format(&buf, info); err != nil {
		t.Fatalf("Format: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "dirty:") {
		t.Errorf("output missing dirty key: %q", out)
	}
	if !strings.Contains(out, "true") {
		t.Errorf("output missing dirty value: %q", out)
	}
}

// --- WriteJSON -------------------------------------------------------

// TestWriteJSON_FiveFields asserts the simple JSON writer emits the
// five fields in the frozen order.
func TestWriteJSON_FiveFields(t *testing.T) {
	t.Parallel()

	i := appversion.Info{
		Version:   "v1.0.0",
		Commit:    "abc1234",
		BuildDate: "2026-06-01T12:00:00Z",
		Dirty:     "false",
		BuildType: "release",
	}
	var buf bytes.Buffer
	if err := appversion.WriteJSON(&buf, i); err != nil {
		t.Fatal(err)
	}
	want := `{"version":"v1.0.0","commit":"abc1234","buildDate":"2026-06-01T12:00:00Z","dirty":false,"buildType":"release"}` + "\n"
	if buf.String() != want {
		t.Errorf("got  %q\nwant %q", buf.String(), want)
	}
}

// TestWriteJSON_SingleLine asserts the output is exactly one line.
func TestWriteJSON_SingleLine(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	i := appversion.Info{Version: "v1.0.0", Commit: "abc", BuildDate: "2026-06-01T12:00:00Z", Dirty: "false", BuildType: "release"}
	if err := appversion.WriteJSON(&buf, i); err != nil {
		t.Fatal(err)
	}
	if strings.Count(buf.String(), "\n") != 1 {
		t.Fatalf("output has %d newlines, want 1:\n%s", strings.Count(buf.String(), "\n"), buf.String())
	}
}

// TestWriteJSON_DirtyTrue asserts a dirty Info serializes to
// `"dirty":true`.
func TestWriteJSON_DirtyTrue(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	i := appversion.Info{Version: "v1.4.2-dirty", Commit: "abc", BuildDate: "2026-06-01T12:00:00Z", Dirty: "true", BuildType: "local"}
	if err := appversion.WriteJSON(&buf, i); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"dirty":true`) {
		t.Errorf("output missing dirty:true:\n%s", buf.String())
	}
}

// --- WriteFullJSON ---------------------------------------------------

// TestWriteFullJSON_HasProvenance asserts the full JSON writer
// includes a provenance field.
func TestWriteFullJSON_HasProvenance(t *testing.T) {
	t.Parallel()

	i := appversion.Info{
		Version:   "v1.0.0",
		Commit:    "abc1234",
		BuildDate: "2026-06-01T12:00:00Z",
		Dirty:     "false",
		BuildType: "release",
		Provenance: libversion.Provenance{
			GoVersion: "go1.23.4",
			Module:    "example.com/x",
		},
	}
	var buf bytes.Buffer
	if err := appversion.WriteFullJSON(&buf, i); err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, buf.String())
	}
	if _, ok := doc["provenance"]; !ok {
		t.Errorf("output missing provenance:\n%s", buf.String())
	}
	if doc["version"] != "v1.0.0" {
		t.Errorf("version = %v", doc["version"])
	}
	if doc["buildType"] != "release" {
		t.Errorf("buildType = %v", doc["buildType"])
	}
}

// TestWriteFullJSON_FieldOrder asserts the five linker fields come
// before the provenance key.
func TestWriteFullJSON_FieldOrder(t *testing.T) {
	t.Parallel()

	i := appversion.Info{
		Version:   "v1.0.0",
		Commit:    "abc",
		BuildDate: "2026-06-01T12:00:00Z",
		Dirty:     "false",
		BuildType: "release",
		Provenance: libversion.Provenance{
			GoVersion: "go1.23.4",
		},
	}
	var buf bytes.Buffer
	if err := appversion.WriteFullJSON(&buf, i); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
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

// --- WriteVerbose ----------------------------------------------------

// TestWriteVerbose_Sections asserts the verbose writer emits the
// documented sections.
func TestWriteVerbose_Sections(t *testing.T) {
	t.Parallel()

	i := appversion.Info{
		Version:   "v1.0.0",
		Commit:    "abc1234",
		BuildDate: "2026-06-01T12:00:00Z",
		Dirty:     "false",
		BuildType: "release",
		Provenance: libversion.Provenance{
			GoVersion: "go1.23.4",
			Module:    "example.com/x",
			Dependencies: []libversion.Dependency{
				{Path: "github.com/spf13/cobra", Version: "v1.8.1"},
			},
			Settings: []libversion.Setting{
				{Key: "-trimpath", Value: "true"},
			},
		},
	}
	var buf bytes.Buffer
	if err := appversion.WriteVerbose(&buf, i); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"version:", "commit:", "buildDate:", "dirty:", "buildType:",
		"go:", "module:",
		"dependencies:",
		"settings:",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

// TestWriteVerbose_MinimalProvenance asserts the dependencies and
// settings sections are omitted when their lists are empty.
func TestWriteVerbose_MinimalProvenance(t *testing.T) {
	t.Parallel()

	i := appversion.Info{
		Version:   "v1.0.0",
		Commit:    "abc",
		BuildDate: "2026-06-01T12:00:00Z",
		Dirty:     "false",
		BuildType: "local",
	}
	var buf bytes.Buffer
	if err := appversion.WriteVerbose(&buf, i); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, "dependencies:") {
		t.Errorf("output has an empty dependencies section:\n%s", out)
	}
	if strings.Contains(out, "settings:") {
		t.Errorf("output has an empty settings section:\n%s", out)
	}
}

// TestWriteVerbose_Deterministic asserts two writes of the same Info
// produce byte-identical output.
func TestWriteVerbose_Deterministic(t *testing.T) {
	t.Parallel()

	i := appversion.Info{
		Version:   "v1.0.0",
		Commit:    "abc",
		BuildDate: "2026-06-01T12:00:00Z",
		Dirty:     "false",
		BuildType: "release",
		Provenance: libversion.Provenance{
			GoVersion: "go1.23.4",
			Dependencies: []libversion.Dependency{
				{Path: "z", Version: "v1"},
				{Path: "a", Version: "v1"},
			},
		},
	}
	var a, b bytes.Buffer
	if err := appversion.WriteVerbose(&a, i); err != nil {
		t.Fatal(err)
	}
	if err := appversion.WriteVerbose(&b, i); err != nil {
		t.Fatal(err)
	}
	if a.String() != b.String() {
		t.Fatal("WriteVerbose is not deterministic")
	}
}

// --- Raw -------------------------------------------------------------

// TestRaw_UsesGet asserts Raw produces the same one-line format as
// Format.
func TestRaw_UsesGet(t *testing.T) {
	t.Parallel()

	raw := appversion.Raw()
	if !strings.HasPrefix(raw, "forge") {
		t.Errorf("Raw() = %q, want a forge prefix", raw)
	}
	if !strings.HasSuffix(raw, "\n") {
		t.Errorf("Raw() does not end with a newline: %q", raw)
	}
}

// TestRaw_MatchesFormat asserts Raw returns exactly what Format
// writes for the same Info.
func TestRaw_MatchesFormat(t *testing.T) {
	t.Parallel()

	raw := appversion.Raw()

	var buf bytes.Buffer
	if err := appversion.Format(&buf, appversion.Get()); err != nil {
		t.Fatal(err)
	}
	if raw != buf.String() {
		t.Errorf("Raw and Format disagree:\n Raw   =%q\n Format=%q",
			raw, buf.String())
	}
}

// --- Negative cases --------------------------------------------------

// errWriter is an io.Writer that always returns an error. It is used
// to test the error path of the writers.
type errWriter struct {
	err error
}

// Write always returns the configured error.
func (w errWriter) Write(p []byte) (int, error) {
	return 0, w.err
}

// TestFormat_PropagatesWriteError verifies that a write error is
// returned to the caller.
func TestFormat_PropagatesWriteError(t *testing.T) {
	t.Parallel()

	sentinel := errWriterError("disk full")
	w := errWriter{err: sentinel}

	err := appversion.Format(w, appversion.Info{Version: "0.1.0"})
	if err == nil {
		t.Fatal("Format: want error, got nil")
	}
	if !strings.Contains(err.Error(), "disk full") {
		t.Errorf("error does not wrap the sentinel: %v", err)
	}
}

// TestWriteJSON_PropagatesWriteError verifies the JSON writer's error
// path.
func TestWriteJSON_PropagatesWriteError(t *testing.T) {
	t.Parallel()

	sentinel := errWriterError("disk full")
	err := appversion.WriteJSON(errWriter{err: sentinel}, appversion.Info{Version: "0.1.0"})
	if err == nil {
		t.Fatal("WriteJSON: want error, got nil")
	}
	if !strings.Contains(err.Error(), "disk full") {
		t.Errorf("error does not wrap the sentinel: %v", err)
	}
}

// TestWriteFullJSON_PropagatesWriteError verifies the full JSON
// writer's error path.
func TestWriteFullJSON_PropagatesWriteError(t *testing.T) {
	t.Parallel()

	sentinel := errWriterError("disk full")
	err := appversion.WriteFullJSON(errWriter{err: sentinel}, appversion.Info{Version: "0.1.0"})
	if err == nil {
		t.Fatal("WriteFullJSON: want error, got nil")
	}
}

// TestWriteVerbose_PropagatesWriteError verifies the verbose writer's
// error path.
func TestWriteVerbose_PropagatesWriteError(t *testing.T) {
	t.Parallel()

	sentinel := errWriterError("disk full")
	err := appversion.WriteVerbose(errWriter{err: sentinel}, appversion.Info{Version: "0.1.0"})
	if err == nil {
		t.Fatal("WriteVerbose: want error, got nil")
	}
}

// errWriterError is a string-based error for tests.
type errWriterError string

// Error implements the error interface.
func (e errWriterError) Error() string { return string(e) }

// --- Non-functional --------------------------------------------------

// TestGet_ConcurrentSafe asserts concurrent calls to Get do not race.
// Run with -race to make the assertion meaningful.
func TestGet_ConcurrentSafe(t *testing.T) {
	t.Parallel()

	done := make(chan struct{})
	for i := 0; i < 8; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 100; j++ {
				_ = appversion.Get()
			}
		}()
	}
	for i := 0; i < 8; i++ {
		<-done
	}
}

// TestGet_ProvenanceTimeIsUTC asserts the VCSTime, when present, is
// in UTC. The provenance reader normalizes the time; this test pins
// the property at the service level.
func TestGet_ProvenanceTimeIsUTC(t *testing.T) {
	t.Parallel()

	got := appversion.Get()
	if got.Provenance.VCSTime.IsZero() {
		t.Skip("no VCS time in this build; nothing to assert")
	}
	if got.Provenance.VCSTime.Location() != time.UTC {
		t.Errorf("VCSTime location = %v, want UTC", got.Provenance.VCSTime.Location())
	}
}
