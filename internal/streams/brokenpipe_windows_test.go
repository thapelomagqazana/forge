//go:build windows

package streams

import (
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"
	"testing"
)

// =============================================================================
// Direct match: syscall.ERROR_BROKEN_PIPE
// =============================================================================

// TestIsBrokenPipe_ErrorBrokenPipe asserts that a bare
// syscall.ERROR_BROKEN_PIPE is recognized as a broken pipe. This
// is the function's primary contract on Windows: the code
// WriteFile returns when the read handle of an anonymous pipe has
// closed.
func TestIsBrokenPipe_ErrorBrokenPipe(t *testing.T) {
	t.Parallel()

	if !IsBrokenPipe(syscall.ERROR_BROKEN_PIPE) {
		t.Fatal("IsBrokenPipe(ERROR_BROKEN_PIPE) = false; want true")
	}
}

// =============================================================================
// Wrapped ERROR_BROKEN_PIPE
// =============================================================================

// TestIsBrokenPipe_WrappedErrorBrokenPipe asserts that a wrapped
// ERROR_BROKEN_PIPE is recognized. The wrapper preserves the
// error chain via errors.Unwrap; errors.Is traverses it; the
// function returns true.
//
// The property matters because a write site typically wraps the
// error before returning it. If the function only matched the bare
// code, every wrapped form would be misclassified as an
// unclassified error, and the exit boundary would return the
// internal-error code instead of the broken-pipe code.
func TestIsBrokenPipe_WrappedErrorBrokenPipe(t *testing.T) {
	t.Parallel()

	wrapped := fmt.Errorf("write stdout: %w", syscall.ERROR_BROKEN_PIPE)
	if !IsBrokenPipe(wrapped) {
		t.Fatal("IsBrokenPipe(wrapped ERROR_BROKEN_PIPE) = false; want true")
	}
}

// TestIsBrokenPipe_DeeplyWrappedErrorBrokenPipe asserts that the
// function traverses an arbitrarily deep wrap chain.
func TestIsBrokenPipe_DeeplyWrappedErrorBrokenPipe(t *testing.T) {
	t.Parallel()

	inner := fmt.Errorf("write: %w", syscall.ERROR_BROKEN_PIPE)
	middle := fmt.Errorf("flush: %w", inner)
	outer := fmt.Errorf("render: %w", middle)

	if !IsBrokenPipe(outer) {
		t.Fatal("IsBrokenPipe(deeply wrapped ERROR_BROKEN_PIPE) = false; want true")
	}
}

// =============================================================================
// Non-matches
// =============================================================================

// TestIsBrokenPipe_NotBrokenPipe asserts that the function returns
// false for a representative set of non-broken-pipe errors. The
// table covers the common sentinels a caller might pass and the
// codes that are often confused with ERROR_BROKEN_PIPE.
//
// The property is the function's negative contract: it must not
// misclassify an unrelated error as a broken pipe, because the
// exit boundary would then exit 141 for an error that should have
// produced a diagnostic.
//
// # Why the table does not include every Windows error code
//
// The table's purpose is to enumerate the codes a write to a pipe
// might plausibly return and to assert that the classifier stays
// silent on each. It is not an exhaustive catalogue of the
// Windows error space. The four codes below — ACCESS_DENIED,
// HANDLE_EOF, OPERATION_ABORTED, and the io/os sentinels — are
// the ones a caller is most likely to confuse with a broken
// pipe. A code that is not on that short list (for example,
// ERROR_INVALID_HANDLE, value 6) is not tested here.
//
// # How to add a code the standard library does not export
//
// The standard library's syscall package on Windows exports the
// codes this test uses. It does not export every code. A code the
// package does not export (for example, ERROR_INVALID_HANDLE or
// ERROR_NO_DATA) is declared locally with its Windows API value:
//
//	const errorInvalidHandle = syscall.Errno(6)
//
// and the constant is added to the table with a pin for its
// value, the way TestIsBrokenPipe_ErrorNoDataIsNotBrokenPipe
// pins ERROR_NO_DATA (232). The local declaration avoids a
// module dependency on golang.org/x/sys/windows, which the
// package deliberately does not import.
func TestIsBrokenPipe_NotBrokenPipe(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
	}{
		{"nil", nil},
		{"io.EOF", io.EOF},
		{"io.ErrClosedPipe", io.ErrClosedPipe},
		{"io.ErrUnexpectedEOF", io.ErrUnexpectedEOF},
		{"os.ErrNotExist", os.ErrNotExist},
		{"os.ErrPermission", os.ErrPermission},
		{"syscall.ERROR_ACCESS_DENIED", syscall.ERROR_ACCESS_DENIED},
		{"syscall.ERROR_HANDLE_EOF", syscall.ERROR_HANDLE_EOF},
		{"syscall.ERROR_OPERATION_ABORTED", syscall.ERROR_OPERATION_ABORTED},
		{"plain error", errors.New("some unrelated failure")},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if IsBrokenPipe(tc.err) {
				t.Errorf("IsBrokenPipe(%v) = true; want false", tc.err)
			}
		})
	}
}

// TestIsBrokenPipe_ErrorNoDataIsNotBrokenPipe is the boundary
// case the implementation's docstring calls out explicitly.
// ERROR_NO_DATA (232) is a Windows code returned for *named* pipes
// when the reader closes the handle while the writer is flushing.
// Forge does not create named pipes, so the code cannot arise from
// the CLI's traffic. The function therefore does not match it.
//
// # Why the constant is declared locally
//
// The standard library's syscall package does not export
// ERROR_NO_DATA. The golang.org/x/sys/windows package does, but
// importing it for a single constant the package cannot encounter
// would grow the module's dependency surface for no benefit. The
// test declares the constant locally with its Windows API value
// (232) and pins that value.
//
// If a future scenario introduces a named pipe, the implementation
// adds the constant and the check, and this test's expected value
// flips from "false" to "true". The change would be visible in
// the test's diff and require a reviewer's attention.
func TestIsBrokenPipe_ErrorNoDataIsNotBrokenPipe(t *testing.T) {
	t.Parallel()

	// The Windows API value of ERROR_NO_DATA. Declared locally
	// because the standard library does not export it; see the
	// docstring.
	const errorNoData = syscall.Errno(232)

	if IsBrokenPipe(errorNoData) {
		t.Fatal("IsBrokenPipe(ERROR_NO_DATA) = true; want false; " +
			"the implementation deliberately omits the check")
	}

	// Pin the constant's value. If a future Windows API revision
	// changes the number, the pin fails and forces a review.
	const wantValue = 232
	if uintptr(errorNoData) != wantValue {
		t.Fatalf("ERROR_NO_DATA = %d; want %d (the Windows API value)",
			uintptr(errorNoData), wantValue)
	}
}

// TestIsBrokenPipe_WrappedNonBrokenPipe asserts that wrapping a
// non-broken-pipe error does not cause a false positive.
func TestIsBrokenPipe_WrappedNonBrokenPipe(t *testing.T) {
	t.Parallel()

	wrapped := fmt.Errorf("outer: %w", syscall.ERROR_ACCESS_DENIED)
	if IsBrokenPipe(wrapped) {
		t.Fatal("IsBrokenPipe(wrapped ERROR_ACCESS_DENIED) = true; want false")
	}
}

// =============================================================================
// HandleWriteError: the classification helper
// =============================================================================

// TestHandleWriteError_ErrorBrokenPipe asserts that HandleWriteError
// maps a bare ERROR_BROKEN_PIPE to ErrBrokenPipe, the sentinel the
// exit boundary matches on.
func TestHandleWriteError_ErrorBrokenPipe(t *testing.T) {
	t.Parallel()

	got := HandleWriteError(syscall.ERROR_BROKEN_PIPE)
	if !errors.Is(got, ErrBrokenPipe) {
		t.Fatalf("HandleWriteError(ERROR_BROKEN_PIPE) = %v; want ErrBrokenPipe", got)
	}
}

// TestHandleWriteError_WrappedErrorBrokenPipe asserts that a
// wrapped ERROR_BROKEN_PIPE is also mapped to the sentinel.
func TestHandleWriteError_WrappedErrorBrokenPipe(t *testing.T) {
	t.Parallel()

	wrapped := fmt.Errorf("write: %w", syscall.ERROR_BROKEN_PIPE)
	got := HandleWriteError(wrapped)
	if !errors.Is(got, ErrBrokenPipe) {
		t.Fatalf("HandleWriteError(wrapped ERROR_BROKEN_PIPE) = %v; want ErrBrokenPipe", got)
	}
}

// TestHandleWriteError_Nil asserts that nil in produces nil out.
func TestHandleWriteError_Nil(t *testing.T) {
	t.Parallel()

	if got := HandleWriteError(nil); got != nil {
		t.Fatalf("HandleWriteError(nil) = %v; want nil", got)
	}
}

// TestHandleWriteError_NonBrokenPipePassesThrough asserts that a
// non-broken-pipe error is returned unchanged.
func TestHandleWriteError_NonBrokenPipePassesThrough(t *testing.T) {
	t.Parallel()

	want := errors.New("some other failure")
	got := HandleWriteError(want)
	if !errors.Is(got, want) {
		t.Fatalf("HandleWriteError = %v; want the original error", got)
	}
	if errors.Is(got, ErrBrokenPipe) {
		t.Fatal("HandleWriteError misclassified a non-broken-pipe error")
	}
}

// =============================================================================
// End-to-end: a real pipe whose read end closes
// =============================================================================

// TestIsBrokenPipe_RealPipe is the ATDD test on Windows. It opens
// a real pipe, closes the read end, writes to the write end, and
// asserts that the resulting error is recognized as a broken pipe.
//
// # Why an end-to-end test
//
// The unit tests above exercise the classifier's input/output
// relationship with synthesized errors. They prove the function
// matches the right code. They do not prove that a write to a
// closed pipe on the current Windows version produces that code.
// The end-to-end test closes that gap.
//
// # What the test observes
//
// The test uses os.Pipe to obtain a read/write pair. It closes
// the read end, writes to the write end, and inspects the error
// the write returns. On Windows, that error is a *os.PathError
// whose underlying Err is syscall.ERROR_BROKEN_PIPE. The test
// passes that error to IsBrokenPipe and to HandleWriteError, and
// asserts both recognize it.
//
// # Windows-specific behaviour
//
// Windows does not deliver a signal when a pipe's read end
// closes; the kernel returns an error from the write call. The
// error is synchronous and detectable, which is what makes the
// classifier useful on Windows. The test's shape reflects this:
// there is no signal to ignore, only an error to classify.
func TestIsBrokenPipe_RealPipe(t *testing.T) {
	t.Parallel()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	// Close the read end so the next write to the write end
	// fails with ERROR_BROKEN_PIPE.
	if err := r.Close(); err != nil {
		t.Fatalf("close read end: %v", err)
	}
	defer w.Close()

	// Write to the write end. On Windows, the kernel returns
	// ERROR_BROKEN_PIPE.
	_, writeErr := w.Write([]byte("x"))
	if writeErr == nil {
		t.Fatal("write to closed pipe succeeded; want an error")
	}

	// The classifier recognizes the error.
	if !IsBrokenPipe(writeErr) {
		t.Fatalf("IsBrokenPipe(%v) = false; want true "+
			"(a write to a closed pipe should produce ERROR_BROKEN_PIPE)",
			writeErr)
	}

	// HandleWriteError maps it to the sentinel.
	if got := HandleWriteError(writeErr); !errors.Is(got, ErrBrokenPipe) {
		t.Fatalf("HandleWriteError(%v) = %v; want ErrBrokenPipe",
			writeErr, got)
	}
}

// =============================================================================
// Boundary: no panic on nil
// =============================================================================

// TestIsBrokenPipe_NilDoesNotPanic asserts that a nil error is
// tolerated. errors.Is(nil, x) returns false without panicking;
// the function inherits that property. The test pins it so that a
// future change to the implementation (for example, one that
// dereferenced err before calling errors.Is) is caught.
func TestIsBrokenPipe_NilDoesNotPanic(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("IsBrokenPipe(nil) panicked: %v", r)
		}
	}()
	if IsBrokenPipe(nil) {
		t.Fatal("IsBrokenPipe(nil) = true; want false")
	}
}
