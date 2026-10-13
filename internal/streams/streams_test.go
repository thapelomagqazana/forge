package streams

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// =============================================================================
// Test doubles
// =============================================================================

// compile-time assertion that *bytes.Buffer and *strings.Builder
// satisfy the interface fields the tests use. If a future change
// narrows Out or Err from io.Writer, this assertion fails at
// compile time and points at the reason.
var (
	_ io.Writer = (*bytes.Buffer)(nil)
	_ io.Writer = (*strings.Builder)(nil)
	_ io.Reader = (*strings.Reader)(nil)
)

// =============================================================================
// Default
// =============================================================================

// TestDefault_BindsProcessStreams asserts that Default returns the
// process's three standard streams. The check is by identity: the
// In, Out, and Err fields must be the os.Stdin, os.Stdout, and
// os.Stderr values, not copies or wrappers.
//
// The identity check is the strongest form of the assertion. A
// future change that returned, say, a buffered wrapper around
// os.Stdout would fail this test, which is correct: the type's
// docstring promises a binding to the process's streams, not a
// transformation of them.
func TestDefault_BindsProcessStreams(t *testing.T) {
	t.Parallel()

	s := Default()
	if s.In != os.Stdin {
		t.Errorf("In = %v; want os.Stdin", s.In)
	}
	if s.Out != os.Stdout {
		t.Errorf("Out = %v; want os.Stdout", s.Out)
	}
	if s.Err != os.Stderr {
		t.Errorf("Err = %v; want os.Stderr", s.Err)
	}
}

// TestDefault_SnapshotNotAlias asserts that Default captures the
// process streams at call time. Reassigning os.Stdout after
// Default returns does not change the returned Streams.
//
// The property is documented in the Default docstring: the
// returned value is a snapshot, not a live binding. The test pins
// it so that a future "optimization" that returned a lazy
// reference to the globals is caught.
func TestDefault_SnapshotNotAlias(t *testing.T) {
	// Not parallel: mutates os.Stdout, which is process-global.
	old := os.Stdout
	defer func() { os.Stdout = old }()

	s := Default()
	// Reassign os.Stdout to a different writer.
	replacement := os.NewFile(0, os.DevNull)
	if replacement == nil {
		t.Skip("could not open os.DevNull; skipping")
	}
	defer replacement.Close()
	os.Stdout = replacement

	// The returned Streams still points at the original os.Stdout.
	if s.Out != old {
		t.Errorf("Default().Out changed after os.Stdout reassignment; "+
			"want %p, got %p", old, s.Out)
	}
}

// =============================================================================
// FromFiles
// =============================================================================

// TestFromFiles_UsesFilesDirectly asserts that FromFiles stores the
// three *os.File values unchanged. The check is by identity.
func TestFromFiles_UsesFilesDirectly(t *testing.T) {
	t.Parallel()

	in, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("open in: %v", err)
	}
	defer in.Close()

	out, err := os.CreateTemp(t.TempDir(), "out-*")
	if err != nil {
		t.Fatalf("create out: %v", err)
	}
	defer out.Close()

	errf, err := os.CreateTemp(t.TempDir(), "err-*")
	if err != nil {
		t.Fatalf("create err: %v", err)
	}
	defer errf.Close()

	s := FromFiles(in, out, errf)
	if s.In != in {
		t.Errorf("In = %p; want %p", s.In, in)
	}
	if s.Out != out {
		t.Errorf("Out = %p; want %p", s.Out, out)
	}
	if s.Err != errf {
		t.Errorf("Err = %p; want %p", s.Err, errf)
	}
}

// TestFromFiles_AcceptsNils asserts that FromFiles does not reject
// nil files. The type's docstring does not promise validation; the
// caller that wants to detect a missing file uses Validate or
// OutOrEmpty.
func TestFromFiles_AcceptsNils(t *testing.T) {
	t.Parallel()

	s := FromFiles(nil, nil, nil)
	if s.In != nil {
		t.Errorf("In = %v; want nil", s.In)
	}
	if s.Out != nil {
		t.Errorf("Out = %v; want nil", s.Out)
	}
	if s.Err != nil {
		t.Errorf("Err = %v; want nil", s.Err)
	}
}

// =============================================================================
// OutOrEmpty / ErrOrEmpty
// =============================================================================

// TestOutOrEmpty asserts the two branches of OutOrEmpty. A non-nil
// Out is returned unchanged; a nil Out is replaced with
// io.Discard.
func TestOutOrEmpty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	cases := []struct {
		name string
		s    Streams
		want io.Writer
	}{
		{"non-nil", Streams{Out: &buf}, &buf},
		{"nil", Streams{}, io.Discard},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.s.OutOrEmpty(); got != tc.want {
				t.Errorf("OutOrEmpty() = %v; want %v", got, tc.want)
			}
		})
	}
}

// TestErrOrEmpty asserts the two branches of ErrOrEmpty.
func TestErrOrEmpty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	cases := []struct {
		name string
		s    Streams
		want io.Writer
	}{
		{"non-nil", Streams{Err: &buf}, &buf},
		{"nil", Streams{}, io.Discard},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.s.ErrOrEmpty(); got != tc.want {
				t.Errorf("ErrOrEmpty() = %v; want %v", got, tc.want)
			}
		})
	}
}

// TestOutOrEmpty_WritesToDiscard is a behavioural test for the
// fallback. A write to the returned writer must succeed and must
// not panic.
//
// The property is the reason OutOrEmpty exists: a caller can write
// through it without checking whether Out is nil. If the fallback
// were a nil writer, the write would panic; the test pins the
// non-panicking behaviour.
func TestOutOrEmpty_WritesToDiscard(t *testing.T) {
	t.Parallel()

	s := Streams{} // Out is nil
	n, err := s.OutOrEmpty().Write([]byte("discarded"))
	if err != nil {
		t.Fatalf("write to discard: %v", err)
	}
	if n != len("discarded") {
		t.Errorf("wrote %d bytes; want %d", n, len("discarded"))
	}
}

// TestErrOrEmpty_WritesToDiscard is the companion test for
// ErrOrEmpty.
func TestErrOrEmpty_WritesToDiscard(t *testing.T) {
	t.Parallel()

	s := Streams{} // Err is nil
	n, err := s.ErrOrEmpty().Write([]byte("discarded"))
	if err != nil {
		t.Fatalf("write to discard: %v", err)
	}
	if n != len("discarded") {
		t.Errorf("wrote %d bytes; want %d", n, len("discarded"))
	}
}

// =============================================================================
// Validate
// =============================================================================

// TestValidate exercises every combination of nil/non-nil fields.
// The function returns nil when all three are non-nil and an error
// naming the first nil field otherwise.
//
// # Why the test enumerates each nil case
//
// The function's contract is that the returned error names the
// missing field. A test that only checked "some error was
// returned" would pass even if the error message named the wrong
// field. The table enumerates each case with its expected
// substring, so a future change that swapped the field names in
// the messages is caught.
func TestValidate(t *testing.T) {
	t.Parallel()

	nonNil := func() (io.Reader, io.Writer, io.Writer) {
		return strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}
	}

	cases := []struct {
		name      string
		in        io.Reader
		out       io.Writer
		err       io.Writer
		wantError bool
		wantField string
	}{
		{
			name: "all non-nil",
			in:   strings.NewReader(""),
			out:  &bytes.Buffer{},
			err:  &bytes.Buffer{},
		},
		{
			name:      "In nil",
			in:        nil,
			out:       &bytes.Buffer{},
			err:       &bytes.Buffer{},
			wantError: true,
			wantField: "In",
		},
		{
			name:      "Out nil",
			in:        strings.NewReader(""),
			out:       nil,
			err:       &bytes.Buffer{},
			wantError: true,
			wantField: "Out",
		},
		{
			name:      "Err nil",
			in:        strings.NewReader(""),
			out:       &bytes.Buffer{},
			err:       nil,
			wantError: true,
			wantField: "Err",
		},
		{
			name:      "all nil",
			in:        nil,
			out:       nil,
			err:       nil,
			wantError: true,
			wantField: "In",
		},
	}

	// Silence the unused nonNil function; the table above uses its
	// components directly to keep each row self-contained.
	_ = nonNil

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := Streams{In: tc.in, Out: tc.out, Err: tc.err}
			err := s.Validate()
			if !tc.wantError {
				if err != nil {
					t.Fatalf("Validate() = %v; want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatal("Validate() = nil; want an error")
			}
			if !strings.Contains(err.Error(), tc.wantField) {
				t.Errorf("Validate() error = %q; want it to name %q",
					err, tc.wantField)
			}
		})
	}
}

// TestValidate_DefaultIsValid asserts that the production value
// passes the validator. A Default value that failed Validate
// would be a contradiction between the two functions.
func TestValidate_DefaultIsValid(t *testing.T) {
	t.Parallel()

	if err := Default().Validate(); err != nil {
		t.Fatalf("Default().Validate() = %v; want nil", err)
	}
}

// =============================================================================
// WithOut / WithErr / WithIn
// =============================================================================

// TestWithOut_ReturnsCopy asserts the immutability contract of
// WithOut. The receiver is unchanged; the returned value has the
// new field.
//
// The check is by identity on all three fields. A future change
// that returned the same struct after mutating it would pass a
// "field value" assertion but fail the "receiver unchanged"
// assertion, which is the one that matters.
func TestWithOut_ReturnsCopy(t *testing.T) {
	t.Parallel()

	var orig, replacement bytes.Buffer
	base := Streams{
		In:  strings.NewReader("in"),
		Out: &orig,
		Err: &bytes.Buffer{},
	}

	got := base.WithOut(&replacement)

	if base.Out != &orig {
		t.Errorf("receiver's Out changed: %p; want %p", base.Out, &orig)
	}
	if got.Out != &replacement {
		t.Errorf("returned Out = %p; want %p", got.Out, &replacement)
	}
	if got.In != base.In {
		t.Errorf("returned In changed; want the receiver's In")
	}
	if got.Err != base.Err {
		t.Errorf("returned Err changed; want the receiver's Err")
	}
}

// TestWithErr_ReturnsCopy is the companion test for WithErr.
func TestWithErr_ReturnsCopy(t *testing.T) {
	t.Parallel()

	var orig, replacement bytes.Buffer
	base := Streams{
		In:  strings.NewReader("in"),
		Out: &bytes.Buffer{},
		Err: &orig,
	}

	got := base.WithErr(&replacement)

	if base.Err != &orig {
		t.Errorf("receiver's Err changed: %p; want %p", base.Err, &orig)
	}
	if got.Err != &replacement {
		t.Errorf("returned Err = %p; want %p", got.Err, &replacement)
	}
	if got.In != base.In {
		t.Errorf("returned In changed; want the receiver's In")
	}
	if got.Out != base.Out {
		t.Errorf("returned Out changed; want the receiver's Out")
	}
}

// TestWithIn_ReturnsCopy is the companion test for WithIn.
func TestWithIn_ReturnsCopy(t *testing.T) {
	t.Parallel()

	orig := strings.NewReader("in")
	replacement := strings.NewReader("other")
	base := Streams{
		In:  orig,
		Out: &bytes.Buffer{},
		Err: &bytes.Buffer{},
	}

	got := base.WithIn(replacement)

	if base.In != orig {
		t.Errorf("receiver's In changed; want the original reader")
	}
	if got.In != replacement {
		t.Errorf("returned In changed; want the replacement reader")
	}
	if got.Out != base.Out {
		t.Errorf("returned Out changed; want the receiver's Out")
	}
	if got.Err != base.Err {
		t.Errorf("returned Err changed; want the receiver's Err")
	}
}

// =============================================================================
// ATDD: the injectability property
// =============================================================================

// TestStreams_InjectionRoundTrip is the ATDD acceptance test for
// the WBS 12.4.1 injectability requirement. It exercises the
// pattern a test uses to redirect a command's streams:
//
//	s := streams.Default().WithOut(&out).WithErr(&err).WithIn(in)
//
// The test then writes to s.Out and s.Err and reads from s.In,
// asserting that each operation reaches the injected value.
//
// The property is what makes every command testable in-process:
// a test can substitute buffers for the process's streams without
// touching os.Stdout, os.Stderr, or os.Stdin.
func TestStreams_InjectionRoundTrip(t *testing.T) {
	t.Parallel()

	var out, errBuf bytes.Buffer
	in := strings.NewReader("injected input")

	s := Default().WithOut(&out).WithErr(&errBuf).WithIn(in)

	// Write product output.
	if _, err := io.WriteString(s.Out, "product"); err != nil {
		t.Fatalf("write Out: %v", err)
	}
	// Write diagnostic output.
	if _, err := io.WriteString(s.Err, "diagnostic"); err != nil {
		t.Fatalf("write Err: %v", err)
	}
	// Read input.
	got, err := io.ReadAll(s.In)
	if err != nil {
		t.Fatalf("read In: %v", err)
	}

	if out.String() != "product" {
		t.Errorf("Out received %q; want %q", out.String(), "product")
	}
	if errBuf.String() != "diagnostic" {
		t.Errorf("Err received %q; want %q", errBuf.String(), "diagnostic")
	}
	if string(got) != "injected input" {
		t.Errorf("In yielded %q; want %q", got, "injected input")
	}
}

// TestStreams_ChainedWithMethods asserts that the With methods
// compose: three chained calls produce a Streams whose three
// fields are the three injected values.
//
// The property is the natural consequence of the methods being
// value-returning copies. The test pins it so that a future
// refactor to pointer receivers, or to a mutating implementation,
// is caught.
func TestStreams_ChainedWithMethods(t *testing.T) {
	t.Parallel()

	var out, errBuf bytes.Buffer
	in := strings.NewReader("in")

	s := Default().
		WithOut(&out).
		WithErr(&errBuf).
		WithIn(in)

	if s.Out != &out {
		t.Errorf("Out = %p; want %p", s.Out, &out)
	}
	if s.Err != &errBuf {
		t.Errorf("Err = %p; want %p", s.Err, &errBuf)
	}
	if s.In != in {
		t.Errorf("In = %p; want %p", s.In, in)
	}
}

// =============================================================================
// Boundary: nil fields and the fallbacks
// =============================================================================

// TestStreams_ZeroValue asserts the documented zero-value
// behaviour: a zero Streams has all three fields nil, and the
// two OrEmpty methods produce io.Discard.
//
// The zero value is a valid but not useful Streams. The test pins
// the shape so that a future change to the struct's fields (for
// example, making the zero value useful by defaulting the fields)
// is caught.
func TestStreams_ZeroValue(t *testing.T) {
	t.Parallel()

	var s Streams

	if s.In != nil {
		t.Errorf("zero In = %v; want nil", s.In)
	}
	if s.Out != nil {
		t.Errorf("zero Out = %v; want nil", s.Out)
	}
	if s.Err != nil {
		t.Errorf("zero Err = %v; want nil", s.Err)
	}
	if s.OutOrEmpty() != io.Discard {
		t.Errorf("zero OutOrEmpty() = %v; want io.Discard", s.OutOrEmpty())
	}
	if s.ErrOrEmpty() != io.Discard {
		t.Errorf("zero ErrOrEmpty() = %v; want io.Discard", s.ErrOrEmpty())
	}
}

// TestStreams_ValidatesAfterWith pins the "With does not repair
// other fields" property. A caller that starts with a Streams
// missing Out and calls WithErr still has a Streams missing Out;
// the Validate call reports it. A future "convenience" change
// that defaulted the other fields would fail this test, which is
// the correct failure: the change would blur the difference
// between "the caller forgot to set Out" and "the caller
// deliberately left Out unset".
func TestStreams_ValidatesAfterWith(t *testing.T) {
	t.Parallel()

	// Start with a Streams missing Out and Err.
	s := Streams{In: strings.NewReader("")}

	// Replace Err. Out is still nil.
	s = s.WithErr(&bytes.Buffer{})

	// Declare err at function scope so both assertions below
	// can read it. The `if err := ...; err == nil` form would
	// scope err to the if statement, and the second assertion
	// would fail to compile.
	err := s.Validate()
	if err == nil {
		t.Fatal("Validate() = nil; want an error (Out is still nil)")
	}
	if !strings.Contains(err.Error(), "Out") {
		t.Errorf("Validate() error = %q; want it to name Out", err)
	}
}

// =============================================================================
// Non-functional: immutability under repeated use
// =============================================================================

// TestWithOut_DoesNotShareStateAcrossCalls asserts that two
// derivations from the same base produce independent Streams
// values. A change to one does not affect the other, and neither
// affects the base.
//
// The property is what makes the With methods safe to use in
// table-driven tests: each row can derive its own Streams from a
// shared base without coordinating with the other rows.
func TestWithOut_DoesNotShareStateAcrossCalls(t *testing.T) {
	t.Parallel()

	var a, b bytes.Buffer
	base := Default()

	sa := base.WithOut(&a)
	sb := base.WithOut(&b)

	// The two derived values have different Out fields.
	if sa.Out == sb.Out {
		t.Fatal("two derivations share Out; want independent buffers")
	}
	// The base's Out is unchanged.
	if base.Out != os.Stdout {
		t.Errorf("base.Out changed: %v; want os.Stdout", base.Out)
	}
}

// =============================================================================
// Non-functional: Default does not allocate the fallbacks
// =============================================================================

// TestDefault_NoFallbackAllocation asserts that Default's three
// fields are the process's streams and not synthesized fallbacks.
// The check is by identity: a Default value that returned
// io.Discard for Out would fail.
//
// The property matters because Default is on the hot path of a
// production invocation: it runs once per process and its three
// fields are read by every command. A future "safety" change that
// replaced a nil os.Stdout with io.Discard would silently change
// the destination of every command's output.
func TestDefault_NoFallbackAllocation(t *testing.T) {
	t.Parallel()

	s := Default()
	if s.Out != os.Stdout {
		t.Errorf("Default().Out = %v; want os.Stdout (not a fallback)", s.Out)
	}
	if s.Err != os.Stderr {
		t.Errorf("Default().Err = %v; want os.Stderr (not a fallback)", s.Err)
	}
	if s.In != os.Stdin {
		t.Errorf("Default().In = %v; want os.Stdin (not a fallback)", s.In)
	}
}
