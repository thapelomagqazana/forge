// Package version contains unit tests for the version application
// service.
//
// This file pins the human-readable output format frozen by WBS
// 6.4.1. Every byte of the formatter's output is part of the
// contract; a change to any byte fails a test in this file.
//
// # Test organisation
//
// The tests are grouped by category:
//
//   - Contract   — the shape of the frozen format.
//   - Positive   — well-formed inputs produce the expected output.
//   - Negative   — write errors are propagated; invalid inputs are
//     handled without crashing.
//   - Edge       — empty, unicode, newline, and other unusual
//     inputs.
//   - Non-functional — alignment, determinism, allocation bounds.
//   - Golden     — the long-term byte-for-byte record.
//
// The tests do not construct a Cobra command. The service is pure;
// it is tested as a pure function. The handler is tested separately
// in internal/cli/version_test.go.
package version

import (
	"bytes"
	"errors"
	"io"
	"os"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// =============================================================================
// Contract — the frozen format
// =============================================================================

// TestFormat_FullOutput pins the exact byte-for-byte output of the
// formatter for a fully populated Info.
//
// This is the ATDD acceptance test for WBS 6.4.1. Every byte of
// the output is asserted; a change to the format fails this test
// and requires a new WBS item.
//
// The Go version and platform lines are read from the runtime, so
// the expected values are constructed from runtime.Version() and
// runtime.GOOS/runtime.GOARCH rather than hard-coded. The format's
// shape is pinned; the runtime values are not (they vary between
// hosts).
func TestFormat_FullOutput(t *testing.T) {
	t.Parallel()

	info := Info{
		Version:   "v1.2.3",
		Commit:    "abc1234",
		BuildDate: "2026-10-10T12:00:00Z",
		Dirty:     "false",
	}

	var buf bytes.Buffer
	if err := Format(&buf, info); err != nil {
		t.Fatalf("Format: %v", err)
	}

	want := "forge v1.2.3\n" +
		"  commit:      abc1234\n" +
		"  built:       2026-10-10T12:00:00Z\n" +
		"  dirty:       false\n" +
		"  go version:  " + runtime.Version() + "\n" +
		"  platform:    " + runtime.GOOS + "/" + runtime.GOARCH + "\n"

	if got := buf.String(); got != want {
		t.Errorf("output mismatch:\ngot:  %q\nwant: %q", got, want)
	}
}

// TestFormat_EmptyVersionRendersHeaderWithoutTrailingSpace pins the
// header contract for the uninstrumented case.
//
// The format string "forge %s\n" would produce "forge \n" (with a
// trailing space) for an empty version. The frozen format is
// exactly "forge" with no trailing space. This test catches a
// regression to the naive format string.
func TestFormat_EmptyVersionRendersHeaderWithoutTrailingSpace(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := Format(&buf, Info{}); err != nil {
		t.Fatalf("Format: %v", err)
	}

	header, _, found := strings.Cut(buf.String(), "\n")
	if !found {
		t.Fatalf("output has no newline: %q", buf.String())
	}
	if header != "forge" {
		t.Errorf("header: got %q, want %q", header, "forge")
	}
}

// TestFormat_AllFieldsEmpty pins the full output for a zero-value
// Info. This is the uninstrumented-build case, and it is the most
// common case under `go test`.
//
// Empty Commit and BuildDate render as empty values (with the
// trailing padding spaces from the alignment). Empty Dirty renders
// as "false" because the formatter parses strictly.
func TestFormat_AllFieldsEmpty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := Format(&buf, Info{}); err != nil {
		t.Fatalf("Format: %v", err)
	}

	want := "forge\n" +
		"  commit:      \n" +
		"  built:       \n" +
		"  dirty:       false\n" +
		"  go version:  " + runtime.Version() + "\n" +
		"  platform:    " + runtime.GOOS + "/" + runtime.GOARCH + "\n"

	if got := buf.String(); got != want {
		t.Errorf("output mismatch:\ngot:  %q\nwant: %q", got, want)
	}
}

// =============================================================================
// Positive — well-formed inputs
// =============================================================================

// TestFormat_DirtyTrueRendersTrue pins the rendering of the Dirty
// flag in its true state.
func TestFormat_DirtyTrueRendersTrue(t *testing.T) {
	t.Parallel()

	info := Info{Version: "v1.2.3", Dirty: "true"}

	var buf bytes.Buffer
	if err := Format(&buf, info); err != nil {
		t.Fatalf("Format: %v", err)
	}

	if !strings.Contains(buf.String(), "  dirty:       true\n") {
		t.Errorf("output does not contain '  dirty:       true': %q", buf.String())
	}
}

// TestFormat_DirtyFalseRendersFalse pins the rendering of the Dirty
// flag in its false state.
func TestFormat_DirtyFalseRendersFalse(t *testing.T) {
	t.Parallel()

	info := Info{Version: "v1.2.3", Dirty: "false"}

	var buf bytes.Buffer
	if err := Format(&buf, info); err != nil {
		t.Fatalf("Format: %v", err)
	}

	if !strings.Contains(buf.String(), "  dirty:       false\n") {
		t.Errorf("output does not contain '  dirty:       false': %q", buf.String())
	}
}

// TestFormat_AllFieldsPopulated asserts that every key is present
// in the output when every field is populated. This is a weaker
// assertion than TestFormat_FullOutput but is useful when the
// exact format changes and the strict test needs to be updated:
// this test survives a format change as long as the keys are
// present.
func TestFormat_AllFieldsPopulated(t *testing.T) {
	t.Parallel()

	info := Info{
		Version:   "v1.2.3",
		Commit:    "abc1234",
		BuildDate: "2026-10-10T12:00:00Z",
		Dirty:     "true",
	}

	var buf bytes.Buffer
	if err := Format(&buf, info); err != nil {
		t.Fatalf("Format: %v", err)
	}

	out := buf.String()
	for _, key := range []string{
		"forge v1.2.3",
		"commit:",
		"abc1234",
		"built:",
		"2026-10-10T12:00:00Z",
		"dirty:",
		"true",
		"go version:",
		"platform:",
	} {
		if !strings.Contains(out, key) {
			t.Errorf("output missing %q: %q", key, out)
		}
	}
}

// =============================================================================
// Negative — error propagation and strict parsing
// =============================================================================

// TestFormat_WriterErrorIsPropagated verifies that Format returns
// the writer's error unchanged.
//
// The handler's only error path is the write error from Format. If
// Format swallowed the error, the handler would return nil and the
// CLI would exit with code 0 despite having produced truncated
// output. That is a silent-failure bug.
func TestFormat_WriterErrorIsPropagated(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("simulated write failure")
	w := &failingWriter{err: wantErr}

	err := Format(w, Info{Version: "v1.2.3"})
	if !errors.Is(err, wantErr) {
		t.Errorf("Format: got %v, want %v", err, wantErr)
	}
}

// failingWriter is an io.Writer that always returns err.
type failingWriter struct{ err error }

func (w *failingWriter) Write(p []byte) (int, error) { return 0, w.err }

// TestFormat_DirtyParsingRuleIsStrict verifies that only the
// literal "true" renders as true. Every other value — including
// "1", "TRUE", "yes", and the empty string — renders as false.
//
// This is the negative test for the strict parsing rule documented
// on parseDirty and enforced by WBS 6.1.2.
func TestFormat_DirtyParsingRuleIsStrict(t *testing.T) {
	t.Parallel()

	cases := []struct {
		raw  string
		want string
	}{
		{"true", "true"},
		{"false", "false"},
		{"1", "false"},      // negative
		{"TRUE", "false"},   // negative
		{"yes", "false"},    // negative
		{"", "false"},       // boundary: empty
		{" true", "false"},  // boundary: leading space
		{"true ", "false"},  // boundary: trailing space
		{"true\n", "false"}, // boundary: trailing newline
	}

	// Extract the dirty line from the output. The pattern is
	// anchored to the line start to avoid matching "true" in some
	// other field.
	dirtyRe := regexp.MustCompile(`(?m)^  dirty:\s+(\S*)$`)

	for _, tc := range cases {
		tc := tc
		t.Run("raw="+tc.raw, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			if err := Format(&buf, Info{Dirty: tc.raw}); err != nil {
				t.Fatalf("Format: %v", err)
			}

			m := dirtyRe.FindStringSubmatch(buf.String())
			if m == nil {
				t.Fatalf("dirty line not found in %q", buf.String())
			}
			if m[1] != tc.want {
				t.Errorf("Dirty=%q → %q, want %q", tc.raw, m[1], tc.want)
			}
		})
	}
}

// TestFormat_DoesNotRetryOnWriterError verifies that when the
// writer fails, Format does not attempt to recover or retry. A
// single failed Write is the writer's contract; retrying would mask
// bugs.
func TestFormat_DoesNotRetryOnWriterError(t *testing.T) {
	t.Parallel()

	w := &countingWriter{failAfter: 0}
	err := Format(w, Info{Version: "v1.2.3"})
	if err == nil {
		t.Fatal("Format returned nil despite writer failure")
	}
	if w.writes != 1 {
		t.Errorf("Format wrote %d times; want exactly 1", w.writes)
	}
}

// countingWriter fails after failAfter successful writes. It is used
// to verify that Format does not retry.
type countingWriter struct {
	writes    int
	failAfter int
}

func (w *countingWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writes > w.failAfter {
		return 0, io.ErrClosedPipe
	}
	return len(p), nil
}

// =============================================================================
// Edge — unusual but valid inputs
// =============================================================================

// TestFormat_UnicodeIsPreserved verifies that the formatter does
// not escape or quote non-ASCII values. A version such as "1.0.0-α"
// round-trips unchanged.
//
// The injected values are constrained to ASCII by WBS 6.1.2, so
// this is a forward-compatibility test: if a future phase widens
// the charset, the formatter does not need to change.
func TestFormat_UnicodeIsPreserved(t *testing.T) {
	t.Parallel()

	info := Info{Version: "1.0.0-α"}

	var buf bytes.Buffer
	if err := Format(&buf, info); err != nil {
		t.Fatalf("Format: %v", err)
	}

	if !strings.Contains(buf.String(), "1.0.0-α") {
		t.Errorf("output did not preserve non-ASCII: %q", buf.String())
	}
}

// TestFormat_NewlineInFieldIsPreserved documents that the formatter
// does not sanitise values. A version string containing a newline
// produces a multi-line header.
//
// This is not defended against because the injected values are
// constrained by WBS 6.1.2 to the charset [A-Za-z0-9._:+-], which
// excludes newlines. The test documents the behaviour so a future
// reader knows the sanitisation is a deliberate non-goal.
func TestFormat_NewlineInFieldIsPreserved(t *testing.T) {
	t.Parallel()

	info := Info{Version: "v1.2.3\nmalicious"}

	var buf bytes.Buffer
	if err := Format(&buf, info); err != nil {
		t.Fatalf("Format: %v", err)
	}

	if !strings.Contains(buf.String(), "forge v1.2.3\nmalicious\n") {
		t.Errorf("output does not contain the raw version: %q", buf.String())
	}
}

// TestFormat_VeryLongVersionDoesNotPanic verifies that a very long
// version string does not cause the formatter to panic or truncate.
// The formatter is not a validator; it writes whatever it is given.
func TestFormat_VeryLongVersionDoesNotPanic(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("v", 100_000)
	info := Info{Version: long}

	var buf bytes.Buffer
	if err := Format(&buf, info); err != nil {
		t.Fatalf("Format: %v", err)
	}

	if !strings.HasPrefix(buf.String(), "forge "+long+"\n") {
		t.Errorf("output did not preserve long version; got %d bytes", buf.Len())
	}
}

// =============================================================================
// Non-functional — alignment, determinism, allocation
// =============================================================================

// TestFormat_ValueColumnIsAligned verifies that every detail line
// starts its value at the same column.
//
// This is the alignment contract of the frozen format; a future
// change to the field width that breaks alignment fails this test.
// The test asserts the alignment structurally rather than by byte
// offset, so it survives a change to the padding width as long as
// the alignment holds.
func TestFormat_ValueColumnIsAligned(t *testing.T) {
	t.Parallel()

	info := Info{
		Version:   "v1.2.3",
		Commit:    "abc1234",
		BuildDate: "2026-10-10T12:00:00Z",
		Dirty:     "false",
	}

	var buf bytes.Buffer
	if err := Format(&buf, info); err != nil {
		t.Fatalf("Format: %v", err)
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 6 {
		t.Fatalf("line count: got %d, want 6\n%s", len(lines), buf.String())
	}

	// Find the column of the value on each detail line (lines 1..5).
	// A value starts at the first non-space character after the
	// colon.
	var cols []int
	for i, line := range lines[1:] {
		colon := strings.Index(line, ":")
		if colon < 0 {
			t.Fatalf("line %d has no colon: %q", i+1, line)
		}
		j := colon + 1
		for j < len(line) && line[j] == ' ' {
			j++
		}
		cols = append(cols, j)
	}

	for i := 1; i < len(cols); i++ {
		if cols[i] != cols[0] {
			t.Errorf("value column misaligned: line %d starts at col %d, line 1 at col %d\n%s",
				i+1, cols[i], cols[0], buf.String())
		}
	}
}

// TestFormat_AllocationBounded verifies that Format does not
// allocate unboundedly. The bound is generous and documented; the
// point of the test is to catch a future change that adds a
// per-call allocation in a loop or builds an intermediate string,
// not to pin a specific allocation count.
//
// # Why the bound is what it is
//
// Format makes six calls to fmt.Fprintf: one for the header and
// five in the loop over detail lines. Each call with format verbs
// allocates a small number of times for the reflection machinery
// and the formatting buffer. Measured on Go 1.25, the total is
// about 29 allocations per call. The bound below is set to 60,
// which is roughly twice the measured value. The extra headroom
// absorbs:
//
//   - Minor allocation changes between Go patch releases.
//   - A future refactor that adds a sixth detail line (one more
//     Fprintf, roughly 5 more allocations).
//   - A future refactor that replaces the loop with a slice of
//     pre-formatted lines (adds a few allocations for the slice).
//
// The bound is deliberately loose. A tight bound would fail on a
// future Go release without indicating any real regression, and a
// contributor would be forced to raise the number and rerun. A
// loose bound fails only if the allocation count grows by an order
// of magnitude, which is the signal the test is designed to catch.
//
// # Why this test is not parallel
//
// testing.AllocsPerRun measures allocations on the current
// goroutine. Running it in parallel with other tests that also
// allocate would produce a noisy measurement. The test is fast
// (a few milliseconds), so serial execution is acceptable.
func TestFormat_AllocationBounded(t *testing.T) {
	info := Info{
		Version:   "v1.2.3",
		Commit:    "abc1234",
		BuildDate: "2026-10-10T12:00:00Z",
		Dirty:     "false",
	}

	allocs := testing.AllocsPerRun(1000, func() {
		var buf bytes.Buffer
		_ = Format(&buf, info)
	})

	// Measured on Go 1.25: ~29 allocations per call.
	// Bound set to 60 for headroom; see the docstring above.
	const maxAllocs = 60
	if allocs > maxAllocs {
		t.Errorf("Format allocated %v times per run; want <= %d. "+
			"If this is a real regression, investigate; "+
			"if it is a Go release change, raise the bound "+
			"and record the new measurement in the docstring.",
			allocs, maxAllocs)
	}
}

// =============================================================================
// Golden files — the long-term stability test
// =============================================================================

// TestFormat_Golden compares the formatter's output against golden
// files under testdata/format/.
//
// # The golden files' contract
//
// The golden files are committed and version-controlled. A change
// to the format requires updating the golden file and citing a WBS
// item in the commit message.
//
// # Runtime line normalisation
//
// The last two lines of the output — "go version" and "platform" —
// are read from the runtime and therefore vary between hosts and
// Go patch releases. The golden files contain the literal text
// "<placeholder>" on those two lines. The test normalises the
// formatter's actual output to use the same placeholders before
// comparing, so the comparison is byte-for-byte on the four stable
// lines and placeholder-for-placeholder on the two runtime lines.
//
// # Why the golden files use placeholders
//
// A golden file that hard-codes "go1.25.13" and "linux/amd64" would
// fail on a contributor's macOS laptop and on a CI runner with a
// different Go patch. Normalising the two runtime lines is simpler
// than splitting the format into two golden files and reassembling
// them at test time.
//
// # How to update a golden file
//
// Edit the file by hand. Do not add an -update flag to this test:
// updating golden files by hand forces the contributor to read the
// diff, which is the point of a golden file.
func TestFormat_Golden(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		info   Info
		golden string
	}{
		{
			name:   "empty",
			info:   Info{},
			golden: "testdata/format/empty.golden",
		},
		{
			name: "full",
			info: Info{
				Version:   "v1.2.3",
				Commit:    "abc1234",
				BuildDate: "2026-10-10T12:00:00Z",
				Dirty:     "false",
			},
			golden: "testdata/format/full.golden",
		},
		{
			name: "dirty",
			info: Info{
				Version:   "v1.2.3-4-gabc1234-dirty",
				Commit:    "abc1234",
				BuildDate: "2026-10-10T12:00:00Z",
				Dirty:     "true",
			},
			golden: "testdata/format/dirty.golden",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			if err := Format(&buf, tc.info); err != nil {
				t.Fatalf("Format: %v", err)
			}

			// The last two lines of the output are read from the
			// runtime and vary by host and Go patch release. The
			// golden files contain the literal text "<placeholder>"
			// on those two lines. normaliseRuntimeLines rewrites
			// the actual output to use the same placeholders, so
			// the comparison is byte-for-byte on the four stable
			// lines and placeholder-for-placeholder on the two
			// runtime lines.
			got := normaliseRuntimeLines(buf.String())
			want := readGolden(t, tc.golden)

			if got != want {
				t.Errorf("golden mismatch for %s:\ngot:  %q\nwant: %q\n"+
					"if the change is intentional, update the golden file "+
					"by hand and cite WBS 6.4.1 in the commit message",
					tc.golden, got, want)
			}
		})
	}
}

// normaliseRuntimeLines replaces the two runtime-dependent lines of
// the formatter's output with the literal text "<placeholder>", so
// that the output can be compared against a golden file whose two
// runtime lines also contain "<placeholder>".
//
// # The lines that are replaced
//
// The formatter produces six lines. The last two are:
//
//	go version:  <runtime.Version()>
//	platform:    <runtime.GOOS>/<runtime.GOARCH>
//
// Both vary between hosts and Go patch releases. The function
// replaces the value portion of each line with "<placeholder>",
// preserving the key portion and the surrounding whitespace. This
// keeps the comparison sensitive to any change in the key's
// spelling or the padding around it, and insensitive to the
// runtime value.
//
// # Why not a regex
//
// A regex would also work, but the line-by-line match is simpler to
// read and does not require escaping the two keys' colons. The
// function does its work in a single pass over the six lines.
//
// # Why the golden files use placeholders
//
// A golden file that hard-codes "go1.25.13" and "linux/amd64" would
// fail on a contributor's macOS laptop and on a CI runner with a
// different Go patch. Normalising the two runtime lines is simpler
// than splitting the format into two golden files and reassembling
// them at test time.
//
// # Relationship to the golden files
//
// The golden files contain "<placeholder>" on the two runtime
// lines. The function is called on the formatter's actual output,
// not on the golden file. This asymmetry is deliberate: the golden
// file is the source of truth, and the function makes the actual
// output conform to it.
func normaliseRuntimeLines(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "  go version:  "):
			lines[i] = "  go version:  <placeholder>"
		case strings.HasPrefix(line, "  platform:    "):
			lines[i] = "  platform:    <placeholder>"
		}
	}
	return strings.Join(lines, "\n")
}

// readGolden reads a golden file from the test's working directory
// and fails the test if the read errors.
//
// The helper is used by TestFormat_Golden. It does not strip
// whitespace or normalise line endings; the golden file must match
// the formatter's output (after normalisation by
// normaliseRuntimeLines) byte for byte.
func readGolden(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", path, err)
	}
	return string(b)
}
