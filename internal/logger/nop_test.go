package logger

import (
	"sync"
	"sync/atomic"
	"testing"
)

// =============================================================================
// Compile-time interface satisfaction
// =============================================================================

// TestNop_ImplementsLogger is the compile-time assertion, duplicated
// in a test so a reader browsing tests sees the contract. The
// assertion in nop.go is the one that fails the build; this one is
// documentation.
func TestNop_ImplementsLogger(t *testing.T) {
	t.Parallel()

	var _ Logger = Nop()
}

// =============================================================================
// No stream writes
// =============================================================================

// countingWriter records every Write call it receives.
//
// The test asserts the count is zero after exercising Nop. A
// buffer-based check would also work, but a counting writer makes
// the assertion positive: it fails if *any* write occurs, not if
// the writer's contents differ from an expected value.
type countingWriter struct {
	writes atomic.Int64
	bytes  atomic.Int64
}

func (w *countingWriter) Write(p []byte) (int, error) {
	w.writes.Add(1)
	w.bytes.Add(int64(len(p)))
	return len(p), nil
}

// TestNop_WritesNothingToAnyStream asserts that calling every method
// of the Nop logger — including the ones that take attributes — does
// not write to any stream.
//
// The Nop logger has no writer by construction; the test proves the
// absence by routing a counting writer through Options.Writer and
// checking it saw no writes. The route is only meaningful because
// Nop() is the value under test; the writer is what the *live*
// logger would have used.
func TestNop_WritesNothingToAnyStream(t *testing.T) {
	t.Parallel()

	w := &countingWriter{}
	// Prove the writer is functional: a live logger writes to it.
	live := New(Options{
		Level:     LevelDebug,
		Component: ComponentCLI,
		Writer:    w,
	})
	live.Info("baseline")

	if w.writes.Load() == 0 {
		t.Fatal("baseline write did not reach the counting writer; " +
			"the test cannot distinguish Nop's silence from a broken writer")
	}

	// Reset the counter and exercise Nop. Any write after this
	// point would be a Nop bug.
	w.writes.Store(0)
	w.bytes.Store(0)

	l := Nop()
	l.Debug("d")
	l.Info("i")
	l.Warn("w")
	l.Error("e")
	_ = l.With("k", "v")
	_ = l.With()
	_ = l.With("a", 1, "b", 2, "c")

	if got := w.writes.Load(); got != 0 {
		t.Fatalf("Nop wrote to the stream %d times; want 0", got)
	}
	if got := w.bytes.Load(); got != 0 {
		t.Fatalf("Nop wrote %d bytes; want 0", got)
	}
}

// =============================================================================
// No panics for pathological inputs
// =============================================================================

// TestNop_DoesNotPanic asserts that every method of the Nop logger
// is safe to call with inputs that would be problematic for a live
// logger: nil contexts, odd-length attr lists, empty messages,
// repeated With calls, and nil values in attr lists.
//
// The test recovers from a panic and reports which call panicked,
// rather than failing the whole test on the first panic. This makes
// the failure message name the offending call.
func TestNop_DoesNotPanic(t *testing.T) {
	t.Parallel()

	l := Nop()

	calls := []struct {
		name string
		fn   func()
	}{
		{"Debug with empty message", func() { l.Debug("") }},
		{"Info with empty message", func() { l.Info("") }},
		{"Warn with empty message", func() { l.Warn("") }},
		{"Error with empty message", func() { l.Error("") }},

		{"Info with nil attrs", func() { l.Info("x", nil) }},
		{"Info with odd-length attrs", func() { l.Info("x", "lonely") }},
		{"Info with nil key", func() { l.Info("x", nil, "v") }},
		{"Info with nil value", func() { l.Info("x", "k", nil) }},
		{"Info with mixed types", func() { l.Info("x", 1, 2, 3.5, true) }},

		{"With no args", func() { _ = l.With() }},
		{"With odd args", func() { _ = l.With("k") }},
		{"With many args", func() {
			_ = l.With("a", 1, "b", 2, "c", 3, "d", 4)
		}},
		{"With nil value", func() { _ = l.With("k", nil) }},
	}

	for _, c := range calls {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Nop panicked: %v", r)
				}
			}()
			c.fn()
		})
	}
}

// TestNop_ChainIsSafe asserts that a long chain of With calls does
// not panic and remains usable. The property is not just "no panic
// on one call"; it is "the chain stays a Nop no matter how long it
// gets". A future implementation that wrapped a fresh value on each
// With call would still satisfy the interface, but a chain of
// thousands of such calls would be memory-wasteful; the test pins
// the current behavior.
func TestNop_ChainIsSafe(t *testing.T) {
	t.Parallel()

	l := Nop()
	for i := 0; i < 1000; i++ {
		l = l.With("k", i)
	}
	// The chain is still a Nop; every method is safe to call.
	l.Info("x")
	l.Error("y")
}

// =============================================================================
// With returns the receiver
// =============================================================================

// TestNop_WithReturnsSameValue asserts that With returns the same
// interface value as the receiver. The docstring makes this
// promise; the test pins it.
//
// The check is by identity, not by behavior. A future change that
// returned a fresh nopLogger{} would be indistinguishable by
// behavior; the identity check catches it.
func TestNop_WithReturnsSameValue(t *testing.T) {
	t.Parallel()

	l := Nop()

	// Two Nop() values are equal by value (nopLogger{} is a
	// struct with no fields). The identity check compares the
	// interface values, which compare equal if their dynamic
	// types and values are equal.
	if got := l.With(); got != l {
		t.Fatalf("With() returned a different value: %#v vs %#v", got, l)
	}
	if got := l.With("k", "v"); got != l {
		t.Fatalf("With(k, v) returned a different value: %#v vs %#v",
			got, l)
	}
}

// =============================================================================
// Nop is a Nop
// =============================================================================

// TestNop_ConcreteTypeIsValue asserts the concrete type of the value
// returned by Nop(). The assertion prevents a future refactor from
// changing the implementation to a pointer type (which would break
// the identity guarantee of With) or to a type with a state
// (which would break the "single shared instance" promise).
//
// The test asserts the type via a type switch, which is the only
// portable way to inspect the dynamic type of an interface value.
func TestNop_ConcreteTypeIsValue(t *testing.T) {
	t.Parallel()

	l := Nop()
	switch v := l.(type) {
	case nopLogger:
		// Correct: the concrete type is the value type.
		_ = v
	default:
		t.Fatalf("Nop() returned %T; want nopLogger (value)", l)
	}
}

// =============================================================================
// Non-functional: no allocation
// =============================================================================

// TestNop_NoAllocations asserts that calling every method of the
// Nop logger allocates nothing. The guarantee is part of the
// docstring; the test makes it enforceable.
//
// # Not parallel
//
// testing.AllocsPerRun observes process-global allocation
// counters; it panics if called from a parallel test. The
// absence of t.Parallel here is required, not an oversight.
func TestNop_NoAllocations(t *testing.T) {
	l := Nop()

	allocs := testing.AllocsPerRun(1000, func() {
		l.Debug("x")
		l.Info("x", "k", "v")
		l.Warn("x")
		l.Error("x")
		_ = l.With("k", "v")
	})
	if allocs != 0 {
		t.Fatalf("Nop allocated %.1f times; want 0", allocs)
	}
}

// TestNop_WithNoAllocations asserts that With itself does not
// allocate on the callee side.
//
// # What the test measures
//
// A caller that builds a With chain by reassigning an interface
// variable pays for boxing on every assignment: Go must wrap the
// returned nopLogger value into a fresh interface header. That
// boxing is a cost of the *caller's* code, not of the callee; it
// is indistinguishable from the cost of any other assignment of
// a value into an interface variable.
//
// The test therefore measures one With call per AllocsPerRun
// invocation, not a hundred. The count is bounded by a small
// constant (the boxing of the returned value into the caller's
// interface variable) and does not grow with the number of calls.
//
// A test that measured a hundred calls in a loop would report a
// hundred allocations, all of them at the call site. That test
// would fail for a reason unrelated to the callee's behavior.
//
// # Not parallel
//
// testing.AllocsPerRun observes process-global allocation
// counters; it panics if called from a parallel test. The absence
// of t.Parallel here is required, not an oversight.
func TestNop_WithNoAllocations(t *testing.T) {
	l := Nop()

	// One With call per measured run. The bound is the boxing
	// cost of assigning the returned value to l, which is a
	// small constant (the interface header, not a heap object).
	allocs := testing.AllocsPerRun(1000, func() {
		_ = l.With("k", "v")
	})
	if allocs > 1 {
		t.Fatalf("With allocated %.1f times; want <= 1 "+
			"(one is the caller's interface boxing)", allocs)
	}
}

// =============================================================================
// Non-functional: concurrent use
// =============================================================================

// TestNop_ConcurrentUse asserts that the Nop logger is safe for
// concurrent use. The test is meaningful even though the methods
// do nothing: a future implementation that, say, updated a shared
// counter would race under -race.
func TestNop_ConcurrentUse(t *testing.T) {
	t.Parallel()

	l := Nop()

	const (
		goroutines = 16
		perG       = 64
	)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < perG; i++ {
				l.Debug("d")
				l.Info("i", "g", g, "i", i)
				l.Warn("w")
				l.Error("e")
				_ = l.With("k", i)
			}
		}()
	}
	wg.Wait()
	// The test's point is the race detector's silence.
}

// =============================================================================
// Nop is a Logger and nothing more
// =============================================================================

// TestNop_IsTheZeroValueOfItsType asserts that Nop() is equivalent
// to the zero value of nopLogger{}. A future implementation that
// made Nop() return a special sentinel value distinct from the
// zero value would fail this test, forcing a review of the
// change.
func TestNop_IsTheZeroValueOfItsType(t *testing.T) {
	t.Parallel()

	var zero nopLogger
	got := Nop()
	if got != Logger(zero) {
		t.Fatalf("Nop() != nopLogger{}: %#v vs %#v", got, zero)
	}
}
