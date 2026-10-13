//go:build unix

// Package streams — Unix broken-pipe classification tests.
//
// This file is compiled only on Unix platforms. Its sibling
// brokenpipe_windows_test.go is compiled only on Windows. The two
// files cover the same contract for their respective platforms:
// the isBrokenPipePlatform function must recognize the platform's
// broken-pipe error, must recognize it in a wrapped form, and must
// not misclassify any other error.
//
// # Relationship to the implementation
//
// The Unix implementation is in brokenpipe_unix.go. It is a one-line
// errors.Is against syscall.EPIPE. The tests in this file exercise
// the line's contract from every angle that matters:
//
//   - The direct match.
//   - The wrapped match, at any depth.
//   - The negative table, which includes the errno values most
//     likely to be confused with EPIPE.
//   - HandleWriteError's mapping to the ErrBrokenPipe sentinel.
//   - An end-to-end test that produces EPIPE from a real pipe, so
//     the classifier is verified against the kernel's behavior,
//     not just against a synthesized sentinel.
//   - A boundary test for nil.
//
// # What the tests do not cover
//
// The end-to-end test uses os.Pipe directly. It does not spawn a
// subprocess; the subprocess shape was considered and removed
// because it added no coverage the os.Pipe test did not already
// provide, and it introduced a source of flakiness under -race.
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
// Direct match: syscall.EPIPE
// =============================================================================

// TestIsBrokenPipe_EPIPE asserts that a bare syscall.EPIPE is
// recognized as a broken pipe. This is the function's primary
// contract on Unix: the errno a write(2) returns when the read
// end of a pipe has closed.
func TestIsBrokenPipe_EPIPE(t *testing.T) {
	t.Parallel()

	if !IsBrokenPipe(syscall.EPIPE) {
		t.Fatal("IsBrokenPipe(EPIPE) = false; want true")
	}
}

// =============================================================================
// Wrapped EPIPE
// =============================================================================

// TestIsBrokenPipe_WrappedEPIPE asserts that an EPIPE wrapped by
// fmt.Errorf with %w is recognized. The wrapper preserves the
// error chain via errors.Unwrap; errors.Is traverses it; the
// function returns true.
//
// The property matters because a write site typically wraps the
// error before returning it. If the function only matched the bare
// errno, every wrapped form would be misclassified as an
// unclassified error, and the exit boundary would return the
// internal-error code instead of the broken-pipe code.
func TestIsBrokenPipe_WrappedEPIPE(t *testing.T) {
	t.Parallel()

	wrapped := fmt.Errorf("write stdout: %w", syscall.EPIPE)
	if !IsBrokenPipe(wrapped) {
		t.Fatal("IsBrokenPipe(wrapped EPIPE) = false; want true")
	}
}

// TestIsBrokenPipe_DeeplyWrappedEPIPE asserts that the function
// traverses an arbitrarily deep wrap chain. errors.Is handles the
// traversal; the test confirms the function benefits from it.
//
// The chain is three layers deep: render → flush → write. Each
// layer adds context; the innermost error is the bare errno. The
// function must still recognize it.
func TestIsBrokenPipe_DeeplyWrappedEPIPE(t *testing.T) {
	t.Parallel()

	inner := fmt.Errorf("write: %w", syscall.EPIPE)
	middle := fmt.Errorf("flush: %w", inner)
	outer := fmt.Errorf("render: %w", middle)

	if !IsBrokenPipe(outer) {
		t.Fatal("IsBrokenPipe(deeply wrapped EPIPE) = false; want true")
	}
}

// TestIsBrokenPipe_WrappedWithSentence asserts that a wrap in the
// middle of a sentence ("failed to write: %w") is still
// recognized. The property is the same as the single-layer wrap
// test; the case exists because a wrapper whose message contains a
// colon and a space is the most common real form and a naive
// implementation might string-match instead of using errors.Is.
func TestIsBrokenPipe_WrappedWithSentence(t *testing.T) {
	t.Parallel()

	wrapped := fmt.Errorf("forge: failed to write to stdout: %w", syscall.EPIPE)
	if !IsBrokenPipe(wrapped) {
		t.Fatal("IsBrokenPipe(sentence-wrapped EPIPE) = false; want true")
	}
}

// =============================================================================
// Non-matches
// =============================================================================

// TestIsBrokenPipe_NotBrokenPipe asserts that the function returns
// false for a representative set of non-broken-pipe errors. The
// table covers the common sentinels a caller might pass and the
// errno values that are often confused with EPIPE.
//
// The property is the function's negative contract: it must not
// misclassify an unrelated error as a broken pipe, because the
// exit boundary would then exit 141 for an error that should have
// produced a diagnostic.
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
		{"syscall.EINTR", syscall.EINTR},
		{"syscall.EAGAIN", syscall.EAGAIN},
		{"syscall.EBADF", syscall.EBADF},
		{"syscall.ECONNRESET", syscall.ECONNRESET},
		{"syscall.ECONNREFUSED", syscall.ECONNREFUSED},
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

// TestIsBrokenPipe_EINTRIsNotBrokenPipe is the boundary case the
// function's docstring calls out explicitly. An interrupted system
// call is not a broken pipe; the caller should retry the write,
// not exit 141.
func TestIsBrokenPipe_EINTRIsNotBrokenPipe(t *testing.T) {
	t.Parallel()

	if IsBrokenPipe(syscall.EINTR) {
		t.Fatal("IsBrokenPipe(EINTR) = true; want false; " +
			"EINTR is not a broken pipe")
	}
}

// TestIsBrokenPipe_WrappedNonBrokenPipe asserts that wrapping a
// non-broken-pipe error does not cause a false positive. The
// wrap chain is traversed, but no EPIPE is found.
func TestIsBrokenPipe_WrappedNonBrokenPipe(t *testing.T) {
	t.Parallel()

	wrapped := fmt.Errorf("outer: %w", syscall.EAGAIN)
	if IsBrokenPipe(wrapped) {
		t.Fatal("IsBrokenPipe(wrapped EAGAIN) = true; want false")
	}
}

// TestIsBrokenPipe_JoinedErrors asserts that a multi-error
// (errors.Join) is recognized when any of its members is EPIPE.
// errors.Is traverses joined errors; the function inherits the
// property. This matters because a future caller might combine
// several write errors and expect the classifier to find EPIPE
// among them.
func TestIsBrokenPipe_JoinedErrors(t *testing.T) {
	t.Parallel()

	joined := errors.Join(syscall.EAGAIN, syscall.EPIPE)
	if !IsBrokenPipe(joined) {
		t.Fatal("IsBrokenPipe(joined EAGAIN, EPIPE) = false; want true")
	}

	joinedNoEPIPE := errors.Join(syscall.EAGAIN, syscall.EINTR)
	if IsBrokenPipe(joinedNoEPIPE) {
		t.Fatal("IsBrokenPipe(joined EAGAIN, EINTR) = true; want false")
	}
}

// =============================================================================
// HandleWriteError: the classification helper
// =============================================================================

// TestHandleWriteError_EPIPE asserts that HandleWriteError maps a
// bare EPIPE to ErrBrokenPipe, the sentinel the exit boundary
// matches on.
func TestHandleWriteError_EPIPE(t *testing.T) {
	t.Parallel()

	got := HandleWriteError(syscall.EPIPE)
	if !errors.Is(got, ErrBrokenPipe) {
		t.Fatalf("HandleWriteError(EPIPE) = %v; want ErrBrokenPipe", got)
	}
}

// TestHandleWriteError_WrappedEPIPE asserts that a wrapped EPIPE
// is also mapped to the sentinel. The property matches the
// function's contract: any error whose chain contains EPIPE is a
// broken pipe regardless of how many layers the caller added.
func TestHandleWriteError_WrappedEPIPE(t *testing.T) {
	t.Parallel()

	wrapped := fmt.Errorf("write: %w", syscall.EPIPE)
	got := HandleWriteError(wrapped)
	if !errors.Is(got, ErrBrokenPipe) {
		t.Fatalf("HandleWriteError(wrapped EPIPE) = %v; want ErrBrokenPipe", got)
	}
}

// TestHandleWriteError_Nil asserts that nil in produces nil out.
// The function is a classifier; a nil input is not an error to be
// classified.
func TestHandleWriteError_Nil(t *testing.T) {
	t.Parallel()

	if got := HandleWriteError(nil); got != nil {
		t.Fatalf("HandleWriteError(nil) = %v; want nil", got)
	}
}

// TestHandleWriteError_NonBrokenPipePassesThrough asserts that a
// non-broken-pipe error is returned unchanged. The exit boundary
// classifies it through the regular error path.
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

// TestIsBrokenPipe_RealPipe is the ATDD test on Unix. It opens a
// real pipe, closes the read end, writes to the write end, and
// asserts that the resulting error is recognized as a broken pipe.
//
// # Why an end-to-end test
//
// The unit tests above exercise the classifier's input/output
// relationship with synthesized errors. They prove the function
// matches the right errno. They do not prove that a write to a
// closed pipe on the current kernel produces that errno. The
// end-to-end test closes that gap: it produces the error the way
// the kernel produces it and asserts the classifier recognizes it.
//
// # What the test observes
//
// The test uses os.Pipe to obtain a read/write pair. It closes the
// read end, writes to the write end, and inspects the error the
// write returns. On every Unix the test runs on, that error is a
// *os.PathError whose underlying Err is syscall.EPIPE. The test
// passes that error to IsBrokenPipe and to HandleWriteError, and
// asserts both recognize it.
//
// # Why no subprocess
//
// An earlier version of this test spawned a subprocess (`true`)
// to reproduce the pipeline shape. The subprocess's pipe was
// closed and the assertion was made against a fresh os.Pipe, so
// the subprocess contributed nothing to the assertion. It also
// caused flakiness under -race: the subprocess's exit and the
// test's read of its pipe raced. The os.Pipe test covers the
// kernel path the classifier cares about; the subprocess was
// removed.
func TestIsBrokenPipe_RealPipe(t *testing.T) {
	t.Parallel()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	// Close the read end so the next write to the write end
	// fails with EPIPE.
	if err := r.Close(); err != nil {
		t.Fatalf("close read end: %v", err)
	}
	defer w.Close()

	// Write to the write end. On Unix, the kernel returns EPIPE.
	_, writeErr := w.Write([]byte("x"))
	if writeErr == nil {
		t.Fatal("write to closed pipe succeeded; want an error")
	}

	// The classifier recognizes the error.
	if !IsBrokenPipe(writeErr) {
		t.Fatalf("IsBrokenPipe(%v) = false; want true "+
			"(a write to a closed pipe should produce EPIPE)",
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

// TestHandleWriteError_NilDoesNotPanic is the companion to
// TestIsBrokenPipe_NilDoesNotPanic for the classifier's other
// entry point.
func TestHandleWriteError_NilDoesNotPanic(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("HandleWriteError(nil) panicked: %v", r)
		}
	}()
	if got := HandleWriteError(nil); got != nil {
		t.Fatalf("HandleWriteError(nil) = %v; want nil", got)
	}
}
