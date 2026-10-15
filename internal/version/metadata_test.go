package version_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/thapelomagqazana/forge/internal/version"
)

// --- MetadataOf ----------------------------------------------------------

// TestMetadataOf_NormalizesBuildKind asserts an unknown build kind
// becomes local. A binary built outside CI must always report a
// valid kind.
func TestMetadataOf_NormalizesBuildKind(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want string
	}{
		{"", string(version.BuildKindLocal)},
		{"release", string(version.BuildKindRelease)},
		{"ci", string(version.BuildKindCI)},
		{"local", string(version.BuildKindLocal)},
		{"unknown", string(version.BuildKindLocal)},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			m := version.MetadataOf("v1.0.0", "abc1234", "2026-06-01T12:00:00Z", "false", tc.in)
			if m.BuildType != tc.want {
				t.Errorf("buildType = %q, want %q", m.BuildType, tc.want)
			}
		})
	}
}

// TestMetadataOf_ParsesDirty asserts the linker's string form of the
// dirty flag becomes a Go bool. Only the exact string "true" yields
// true; every other value (including the empty string) yields false.
//
// The rule is deliberately strict: a binary that does not know it is
// dirty is safer reported as clean than as dirty. A future change
// that loosens the rule must update this test and
// docs/reference/version-output.md §"dirty".
func TestMetadataOf_ParsesDirty(t *testing.T) {
	t.Parallel()
	if !version.MetadataOf("v1.0.0", "abc1234", "2026-06-01T12:00:00Z", "true", "local").Dirty {
		t.Error("MetadataOf(dirty=\"true\").Dirty = false, want true")
	}
	for _, in := range []string{"", "false", "0", "no", "yes", "TRUE", "True", "unknown"} {
		m := version.MetadataOf("v1.0.0", "abc1234", "2026-06-01T12:00:00Z", in, "local")
		if m.Dirty {
			t.Errorf("MetadataOf(dirty=%q).Dirty = true, want false", in)
		}
	}
}

// TestMetadataOf_PreservesVersionVerbatim asserts the version string
// is not normalized. A dirty tree and a commit offset are both
// meaningful.
func TestMetadataOf_PreservesVersionVerbatim(t *testing.T) {
	t.Parallel()
	for _, v := range []string{
		"v1.4.2",
		"v1.4.2-dirty",
		"v1.4.2-3-gabc1234",
		"v1.4.2-3-gabc1234-dirty",
		"v0.0.0-unknown",
		"dev",
	} {
		m := version.MetadataOf(v, "abc1234", "2026-06-01T12:00:00Z", "false", "release")
		if m.Version != v {
			t.Errorf("version = %q, want %q", m.Version, v)
		}
	}
}

// TestMetadataOf_EmptyBuildKindIsLocal asserts the empty string is
// normalized to "local" without an error. This is the case for a
// binary built without -ldflags.
func TestMetadataOf_EmptyBuildKindIsLocal(t *testing.T) {
	t.Parallel()
	m := version.MetadataOf("dev", "none", "", "", "")
	if m.BuildType != string(version.BuildKindLocal) {
		t.Errorf("buildType = %q, want local", m.BuildType)
	}
}

// --- WriteJSON -----------------------------------------------------------

// TestWriteJSON_SingleLine asserts the output is exactly one line
// terminated by a newline.
func TestWriteJSON_SingleLine(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	m := version.Metadata{
		Version:   "v1.0.0",
		Commit:    "abc1234",
		BuildDate: "2026-06-01T12:00:00Z",
		Dirty:     false,
		BuildType: "release",
	}
	if err := version.WriteJSON(&buf, m); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	out := buf.String()
	if strings.Count(out, "\n") != 1 {
		t.Fatalf("output has %d newlines, want 1:\n%s", strings.Count(out, "\n"), out)
	}
	if !strings.HasSuffix(out, "\n") {
		t.Fatal("output does not end with a newline")
	}
}

// TestWriteJSON_FieldOrder asserts the field order matches the schema.
// A user's script may depend on this order. The five fields are
// emitted in the order declared on Metadata: version, commit,
// buildDate, dirty, buildType.
func TestWriteJSON_FieldOrder(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	m := version.Metadata{
		Version:   "v1.0.0",
		Commit:    "abc1234",
		BuildDate: "2026-06-01T12:00:00Z",
		Dirty:     false,
		BuildType: "release",
	}
	if err := version.WriteJSON(&buf, m); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	want := `{"version":"v1.0.0","commit":"abc1234","buildDate":"2026-06-01T12:00:00Z","dirty":false,"buildType":"release"}` + "\n"
	if buf.String() != want {
		t.Errorf("got  %q\nwant %q", buf.String(), want)
	}
}

// TestWriteJSON_DirtyTrue asserts a dirty build serializes to
// `"dirty":true`. This is the JSON a local build emits.
func TestWriteJSON_DirtyTrue(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	m := version.Metadata{
		Version:   "v1.4.2-3-gabc1234-dirty",
		Commit:    "abc1234",
		BuildDate: "2026-06-01T12:00:00Z",
		Dirty:     true,
		BuildType: "local",
	}
	if err := version.WriteJSON(&buf, m); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	if !strings.Contains(buf.String(), `"dirty":true`) {
		t.Errorf("output missing dirty:true:\n%s", buf.String())
	}
}

// TestWriteJSON_NoHTMLEscaping asserts <, >, and & are preserved.
func TestWriteJSON_NoHTMLEscaping(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	m := version.Metadata{
		Version:   "v1.0.0<>&",
		Commit:    "abc1234",
		BuildDate: "2026-06-01T12:00:00Z",
		Dirty:     false,
		BuildType: "release",
	}
	if err := version.WriteJSON(&buf, m); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	if strings.Contains(buf.String(), `\u003c`) {
		t.Fatalf("HTML escaping not disabled:\n%s", buf.String())
	}
}

// TestWriteJSON_ValidJSON asserts the output round-trips.
func TestWriteJSON_ValidJSON(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	m := version.Metadata{
		Version:   "v1.0.0",
		Commit:    "abc1234",
		BuildDate: "2026-06-01T12:00:00Z",
		Dirty:     true,
		BuildType: "ci",
	}
	if err := version.WriteJSON(&buf, m); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	var got version.Metadata
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("round-trip: %v", err)
	}
	if got != m {
		t.Errorf("round-trip = %+v, want %+v", got, m)
	}
}

// TestWriteJSON_AllBuildKinds asserts each of the three kinds
// round-trips through JSON.
func TestWriteJSON_AllBuildKinds(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"release", "ci", "local"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			m := version.Metadata{
				Version:   "v1.0.0",
				Commit:    "abc1234",
				BuildDate: "2026-06-01T12:00:00Z",
				Dirty:     false,
				BuildType: kind,
			}
			if err := version.WriteJSON(&buf, m); err != nil {
				t.Fatalf("WriteJSON: %v", err)
			}
			var got version.Metadata
			if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
				t.Fatalf("round-trip: %v", err)
			}
			if got.BuildType != kind {
				t.Errorf("buildType = %q, want %q", got.BuildType, kind)
			}
		})
	}
}

// --- Validate ------------------------------------------------------------

// TestValidate_HappyPath asserts a complete metadata validates.
func TestValidate_HappyPath(t *testing.T) {
	t.Parallel()
	m := version.Metadata{
		Version:   "v1.0.0",
		Commit:    "abc1234",
		BuildDate: "2026-06-01T12:00:00Z",
		Dirty:     false,
		BuildType: "release",
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

// TestValidate_Table drives every required field through its failure
// mode. Dirty is not required: a zero Metadata with a valid version,
// commit, and buildDate but Dirty=false is valid.
func TestValidate_Table(t *testing.T) {
	t.Parallel()
	base := version.Metadata{
		Version:   "v1.0.0",
		Commit:    "abc1234",
		BuildDate: "2026-06-01T12:00:00Z",
		Dirty:     false,
		BuildType: "release",
	}
	cases := []struct {
		name string
		mut  func(*version.Metadata)
		want string
	}{
		{"empty version", func(m *version.Metadata) { m.Version = "" }, "version is empty"},
		{"empty commit", func(m *version.Metadata) { m.Commit = "" }, "commit is empty"},
		{"empty buildDate", func(m *version.Metadata) { m.BuildDate = "" }, "buildDate is empty"},
		{"bad buildDate", func(m *version.Metadata) { m.BuildDate = "yesterday" }, "RFC 3339"},
		{"bad buildType", func(m *version.Metadata) { m.BuildType = "prod" }, "buildType"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := base
			tc.mut(&m)
			err := m.Validate()
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

// TestValidate_AllBuildKinds asserts each of the three kinds
// validates.
func TestValidate_AllBuildKinds(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"release", "ci", "local"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			m := version.Metadata{
				Version:   "v1.0.0",
				Commit:    "abc1234",
				BuildDate: "2026-06-01T12:00:00Z",
				BuildType: kind,
			}
			if err := m.Validate(); err != nil {
				t.Errorf("Validate: %v", err)
			}
		})
	}
}

// TestValidate_BuildDateFormatIsRFC3339UTC asserts UTC is required.
func TestValidate_BuildDateFormatIsRFC3339UTC(t *testing.T) {
	t.Parallel()
	for _, ok := range []string{
		"2026-06-01T12:00:00Z",
		"2026-06-01T12:00:00.000Z",
	} {
		m := version.Metadata{Version: "v1.0.0", Commit: "abc1234", BuildDate: ok, BuildType: "release"}
		if err := m.Validate(); err != nil {
			t.Errorf("Validate(%q): %v", ok, err)
		}
	}
	for _, bad := range []string{
		"2026-06-01T12:00:00",       // no Z
		"2026-06-01T12:00:00+02:00", // non-UTC offset
		"2026-06-01 12:00:00Z",      // space instead of T
	} {
		m := version.Metadata{Version: "v1.0.0", Commit: "abc1234", BuildDate: bad, BuildType: "release"}
		if err := m.Validate(); err == nil {
			t.Errorf("Validate(%q) = nil, want error", bad)
		}
	}
}

// --- BuildKind -----------------------------------------------------------

// TestBuildKind_Valid asserts the truth table.
func TestBuildKind_Valid(t *testing.T) {
	t.Parallel()
	for _, k := range []version.BuildKind{
		version.BuildKindRelease,
		version.BuildKindCI,
		version.BuildKindLocal,
	} {
		if !k.Valid() {
			t.Errorf("%q.Valid() = false", k)
		}
	}
	for _, k := range []version.BuildKind{"", "prod", "Release", "RELEASE", "production"} {
		if k.Valid() {
			t.Errorf("%q.Valid() = true", k)
		}
	}
}

// TestBuildKind_String asserts the string rendering.
func TestBuildKind_String(t *testing.T) {
	t.Parallel()
	for _, k := range []version.BuildKind{
		version.BuildKindRelease,
		version.BuildKindCI,
		version.BuildKindLocal,
	} {
		if k.String() != string(k) {
			t.Errorf("%q.String() = %q", k, k.String())
		}
	}
}

// TestParseBuildKind asserts empty becomes local and an unknown value
// is rejected.
func TestParseBuildKind(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]version.BuildKind{
		"":        version.BuildKindLocal,
		"release": version.BuildKindRelease,
		"ci":      version.BuildKindCI,
		"local":   version.BuildKindLocal,
	} {
		got, err := version.ParseBuildKind(in)
		if err != nil {
			t.Errorf("ParseBuildKind(%q): %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ParseBuildKind(%q) = %q, want %q", in, got, want)
		}
	}
	if _, err := version.ParseBuildKind("prod"); err == nil {
		t.Fatal("expected error for prod")
	}
}

// TestParseBuildKind_CaseSensitive asserts the parser does not accept
// an uppercase value. The linker injects lowercase; an uppercase
// value is a typo, not an alias.
func TestParseBuildKind_CaseSensitive(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"Release", "RELEASE", "Ci", "Local"} {
		if _, err := version.ParseBuildKind(in); err == nil {
			t.Errorf("ParseBuildKind(%q) = nil error, want error", in)
		}
	}
}

// --- Non-functional ------------------------------------------------------

// TestWriteJSON_Concurrent asserts concurrent writers do not race.
func TestWriteJSON_Concurrent(t *testing.T) {
	t.Parallel()
	m := version.Metadata{
		Version:   "v1.0.0",
		Commit:    "abc1234",
		BuildDate: "2026-06-01T12:00:00Z",
		Dirty:     false,
		BuildType: "release",
	}
	done := make(chan struct{})
	for i := 0; i < 8; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 100; j++ {
				var buf bytes.Buffer
				_ = version.WriteJSON(&buf, m)
			}
		}()
	}
	for i := 0; i < 8; i++ {
		<-done
	}
}

// TestWriteJSON_Deterministic asserts two writes of the same value
// produce byte-identical output.
func TestWriteJSON_Deterministic(t *testing.T) {
	t.Parallel()
	m := version.Metadata{
		Version:   "v1.0.0",
		Commit:    "abc1234",
		BuildDate: "2026-06-01T12:00:00Z",
		Dirty:     true,
		BuildType: "ci",
	}
	var a, b bytes.Buffer
	_ = version.WriteJSON(&a, m)
	_ = version.WriteJSON(&b, m)
	if a.String() != b.String() {
		t.Fatal("WriteJSON is not deterministic")
	}
}

// TestBuildDate_ParsesAsUTC asserts the embedded date is UTC.
func TestBuildDate_ParsesAsUTC(t *testing.T) {
	t.Parallel()
	const s = "2026-06-01T12:00:00Z"
	got, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.Location() != time.UTC {
		t.Errorf("location = %v, want UTC", got.Location())
	}
}
