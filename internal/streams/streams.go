package streams

import (
	"errors"
	"io"
	"os"
)

// Streams is the set of three standard streams a command reads
// from and writes to.
//
// # Fields
//
//   - In:  the input stream (stdin). Commands read from it when
//     they accept piped input.
//   - Out: the product output stream (stdout). Product output
//     goes here.
//   - Err: the diagnostic stream (stderr). Logger records and
//     error output go here.
//
// The three fields are independent io.Writer / io.Reader values.
// A Streams constructed from os.Stdin, os.Stdout, and os.Stderr
// is the production value; a Streams constructed from bytes.Buffers
// is the test value.
//
// # Why a struct and not three globals
//
// The CLI's stream contract is that every command uses the same
// three streams. A struct makes the three into one injectable
// value; a caller that wants to redirect the streams redirects
// all three at once, and a command that receives the struct
// cannot accidentally read a different global.
//
// # Why an interface field for Out and Err
//
// Out and Err are io.Writer, not *os.File. A test can supply a
// bytes.Buffer, a strings.Builder, or any other io.Writer. A
// command that writes through Streams.Out does not care whether
// the destination is a file, a pipe, or a buffer.
type Streams struct {
	// In is the input stream. A nil In is treated as an empty
	// reader by readers that check for it; a command that reads
	// from In should use ReadInput or check In for nil.
	In io.Reader

	// Out is the product output stream. Product output goes
	// here. The field is never nil for a value returned by
	// Default; a caller that constructs a Streams by hand must
	// set it.
	Out io.Writer

	// Err is the diagnostic stream. Logger records and error
	// output go here. The field is never nil for a value
	// returned by Default.
	Err io.Writer
}

// Default returns a Streams bound to the process's standard
// streams.
//
// The returned value reads from os.Stdin, writes product output
// to os.Stdout, and writes diagnostics to os.Stderr. It is the
// value production code uses.
//
// # Aliasing os.Stdin, os.Stdout, os.Stderr
//
// The returned value captures the *current* values of os.Stdin,
// os.Stdout, and os.Stderr, not the globals themselves. A caller
// that reassigns os.Stdout after calling Default (for example, a
// test that swaps the global) sees no change in the returned
// Streams. This is deliberate: the point of Streams is to remove
// the dependency on the process globals, and a value that tracked
// them would defeat the purpose.
//
// # When to call Default
//
// Once, at the process boundary (in cmd/forge/main.go). The
// returned Streams is passed down through the CLI's Dependencies
// value and read by every command. Calling Default inside a
// command would reintroduce the process-global dependency the
// package exists to remove.
func Default() Streams {
	return Streams{
		In:  os.Stdin,
		Out: os.Stdout,
		Err: os.Stderr,
	}
}

// FromFiles constructs a Streams from three *os.File values. It
// is a convenience for a caller that already has files (for
// example, a test harness that opened temporary files).
//
// The returned Streams uses the files directly; it does not
// duplicate them. A caller that closes a file after passing it to
// FromFiles invalidates the stream.
//
// # Normalizing typed nils
//
// A *os.File argument may be nil. Storing a typed nil directly
// into the Streams struct's interface fields would produce an
// interface value whose dynamic type is *os.File and whose
// dynamic value is nil. Such a value compares unequal to the
// untyped nil interface, which breaks the contract every helper
// in the package relies on:
//
//   - OutOrEmpty returns io.Discard only when Out == nil. A
//     typed nil in Out would make the comparison false, and the
//     helper would return the typed nil, causing a panic on the
//     first Write.
//   - Validate reports a field as nil only when the field == nil.
//     A typed nil would make Validate return nil, hiding the
//     caller's missing stream.
//
// The function normalizes: a nil *os.File produces the untyped
// nil interface in the corresponding field. A non-nil *os.File
// is stored as the interface value it already is.
func FromFiles(in *os.File, out *os.File, errf *os.File) Streams {
	s := Streams{}
	if in != nil {
		s.In = in
	}
	if out != nil {
		s.Out = out
	}
	if errf != nil {
		s.Err = errf
	}
	return s
}

// OutOrEmpty returns the Out stream, or io.Discard if Out is nil.
//
// The helper exists for callers that want to write to the output
// stream and are willing to discard the bytes when the stream is
// unset. A test that constructs a Streams with only Err set can
// call OutOrEmpty to get a black hole for writes it does not care
// to observe.
//
// # Why io.Discard and not nil
//
// A nil io.Writer causes a panic on the first Write. The package
// prefers a well-defined fallback (discard) over a nil-pointer
// panic. A caller that genuinely wants to detect an unset Out
// checks Out directly.
func (s Streams) OutOrEmpty() io.Writer {
	if s.Out == nil {
		return io.Discard
	}
	return s.Out
}

// ErrOrEmpty returns the Err stream, or io.Discard if Err is nil.
//
// The helper exists for the same reason OutOrEmpty does. Its
// common use is a test that cares about Out but not about Err.
func (s Streams) ErrOrEmpty() io.Writer {
	if s.Err == nil {
		return io.Discard
	}
	return s.Err
}

// Validate returns an error if any of the three streams is nil.
//
// The function is not called by Default (whose streams are
// non-nil by construction). It is called by a caller that accepts
// a Streams from outside and wants to fail fast on a
// misconstruction.
//
// The error names the nil field; a caller that logs the error
// sees exactly which stream is missing.
func (s Streams) Validate() error {
	if s.In == nil {
		return errors.New("streams: In is nil")
	}
	if s.Out == nil {
		return errors.New("streams: Out is nil")
	}
	if s.Err == nil {
		return errors.New("streams: Err is nil")
	}
	return nil
}

// WithOut returns a copy of s with Out replaced by w.
//
// The method supports the common test pattern of "use the default
// streams, but capture stdout":
//
//	s := streams.Default().WithOut(&buf)
//
// The copy is shallow: In and Err are shared with the receiver.
func (s Streams) WithOut(w io.Writer) Streams {
	s.Out = w
	return s
}

// WithErr returns a copy of s with Err replaced by w.
func (s Streams) WithErr(w io.Writer) Streams {
	s.Err = w
	return s
}

// WithIn returns a copy of s with In replaced by r.
func (s Streams) WithIn(r io.Reader) Streams {
	s.In = r
	return s
}
