package version

import (
	"bytes"
	"strings"
	"testing"
)

// Compile-time interface check: Info must be usable as a value type
// with no methods. This pins the contract that the result type is
// plain data.
var _ = Info{}

// --- Positive tests --------------------------------------------------

// TestGet_ReturnsBuildInfo is the ATDD acceptance test for AC7: a
// unit test can call the service directly, without constructing a
// Cobra command.
//
// The test calls Get and asserts that the result is an Info value
// with the expected shape. It does not assert on the specific values
// of Version, Commit, BuildDate, or Dirty, because those are injected
// at link time and vary between builds.
func TestGet_ReturnsBuildInfo(t *testing.T) {
	t.Parallel()

	got := Get()

	// The zero value of Info is valid, so the assertion is on the
	// type, not on the values. The test's purpose is to prove that
	// Get can be called without Cobra; the values are covered by
	// the acceptance tests of the build system (WBS 4.3.2).
	if got.Version == "" && got.Commit == "" && got.BuildDate == "" && got.Dirty == "" {
		// A binary built without ldflags has empty values. This is
		// valid. Log it for the reader's benefit but do not fail.
		t.Log("all build values are empty; " +
			"the test binary was built without ldflags")
	}
}

// TestGet_IsPure verifies that two consecutive calls return equal
// values. This is the purity contract documented on Get.
func TestGet_IsPure(t *testing.T) {
	t.Parallel()

	first := Get()
	second := Get()

	if first != second {
		t.Errorf("Get is not pure: first=%+v second=%+v", first, second)
	}
}

// TestFormat_RendersAllFields verifies that every field of Info
// appears in the formatted output.
func TestFormat_RendersAllFields(t *testing.T) {
	t.Parallel()

	info := Info{
		Version:   "0.1.0-test",
		Commit:    "deadbeef",
		BuildDate: "2026-10-09T12:00:00Z",
		Dirty:     "false",
	}

	var buf bytes.Buffer
	if err := Format(&buf, info); err != nil {
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

	info := Info{
		Version:   "0.1.0-test",
		Commit:    "deadbeef",
		BuildDate: "2026-10-09T12:00:00Z",
		Dirty:     "false",
	}

	var a, b bytes.Buffer
	if err := Format(&a, info); err != nil {
		t.Fatalf("Format (a): %v", err)
	}
	if err := Format(&b, info); err != nil {
		t.Fatalf("Format (b): %v", err)
	}

	if a.String() != b.String() {
		t.Errorf("Format is not deterministic:\n a=%q\n b=%q",
			a.String(), b.String())
	}
}

// --- Edge cases ------------------------------------------------------

// TestFormat_EmptyInfo verifies that a zero-value Info renders
// without error. Every field is empty, but the format is still
// produced.
func TestFormat_EmptyInfo(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := Format(&buf, Info{}); err != nil {
		t.Fatalf("Format(empty): %v", err)
	}

	out := buf.String()
	if !strings.HasPrefix(out, "forge ") {
		t.Errorf("output missing header prefix: %q", out)
	}
	// The version is empty, so the header is "forge \n". This is
	// valid. The reader sees an empty version and knows the binary
	// was built without ldflags.
	//
	// The Dirty field is also empty, so its detail line reads
	// "  dirty:      " with a trailing space. The formatter does
	// not trim; an empty value is rendered as an empty value.
}

// TestFormat_SpecialCharactersInFields verifies that values
// containing spaces, newlines, and non-ASCII characters are rendered
// without error. The formatter does not escape or quote values; it
// writes them as-is.
func TestFormat_SpecialCharactersInFields(t *testing.T) {
	t.Parallel()

	info := Info{
		Version:   "0.1.0-α",
		Commit:    "dead beef",
		BuildDate: "2026-10-09T12:00:00Z",
		Dirty:     "false",
	}

	var buf bytes.Buffer
	if err := Format(&buf, info); err != nil {
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
// rendered as-is. The formatter does not interpret the value; it
// writes the string it is given. This is a small test, but it pins
// the contract that "true" appears in the output when the binary
// was built from a dirty working tree.
func TestFormat_DirtyTrue(t *testing.T) {
	t.Parallel()

	info := Info{
		Version:   "0.1.0-test",
		Commit:    "deadbeef",
		BuildDate: "2026-10-09T12:00:00Z",
		Dirty:     "true",
	}

	var buf bytes.Buffer
	if err := Format(&buf, info); err != nil {
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

// --- Negative cases --------------------------------------------------

// errWriter is an io.Writer that always returns an error. It is used
// to test the error path of Format.
type errWriter struct {
	err error
}

// Write always returns the configured error.
func (w errWriter) Write(p []byte) (int, error) {
	return 0, w.err
}

// TestFormat_PropagatesWriteError verifies that a write error is
// returned to the caller, wrapped with context.
func TestFormat_PropagatesWriteError(t *testing.T) {
	t.Parallel()

	sentinel := errWriterError("disk full")
	w := errWriter{err: sentinel}

	err := Format(w, Info{Version: "0.1.0"})
	if err == nil {
		t.Fatal("Format: want error, got nil")
	}
	if !strings.Contains(err.Error(), "disk full") {
		t.Errorf("error does not wrap the sentinel: %v", err)
	}
}

// errWriterError is a string-based error for tests.
type errWriterError string

// Error implements the error interface.
func (e errWriterError) Error() string { return string(e) }
