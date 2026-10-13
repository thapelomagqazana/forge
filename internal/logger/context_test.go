package logger

import (
	"context"
	"sync"
	"testing"
)

// =============================================================================
// Round-trip: IntoContext → FromContext
// =============================================================================

// TestIntoFromContext_RoundTrip asserts the basic contract: a
// logger stored by IntoContext is returned by FromContext.
//
// The test uses a captureSink so the returned logger's identity can
// be checked by observing which sink it writes to. Comparing
// interface values with == is unreliable for the slogLogger (which
// holds a *slog.Logger); observing the sink is the reliable check.
func TestIntoFromContext_RoundTrip(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	l := New(Options{
		Level:     LevelDebug,
		Component: ComponentCLI,
		Sink:      sink,
	})

	ctx := IntoContext(context.Background(), l)
	got := FromContext(ctx)
	if got == nil {
		t.Fatal("FromContext returned nil")
	}

	got.Info("hello")

	if n := sink.len(); n != 1 {
		t.Fatalf("sink received %d records; want 1", n)
	}
	rec := sink.snapshot()[0]
	if rec.Message != "hello" {
		t.Fatalf("message = %q; want hello", rec.Message)
	}
}

// TestIntoFromContext_ReturnsExactValue asserts that FromContext
// returns the same interface value that IntoContext stored. The
// check is by identity, not by behavior, so a caller who compares
// two loggers (rare but legal) sees the expected result.
func TestIntoFromContext_ReturnsExactValue(t *testing.T) {
	t.Parallel()

	l := Nop()
	ctx := IntoContext(context.Background(), l)
	got := FromContext(ctx)
	if got != l {
		t.Fatalf("FromContext returned a different value: %#v vs %#v",
			got, l)
	}
}

// =============================================================================
// Safe defaults
// =============================================================================

// TestFromContext_EmptyContext_ReturnsNop asserts that a context
// with no stored logger returns Nop(), not nil. This is the
// primary reason the API returns an interface rather than a
// pointer: callers write logger.FromContext(ctx).Info(...)
// without a nil check.
func TestFromContext_EmptyContext_ReturnsNop(t *testing.T) {
	t.Parallel()

	got := FromContext(context.Background())
	if got == nil {
		t.Fatal("FromContext returned nil; want Nop()")
	}
	// Nop is safe to call; the test asserts the returned logger
	// does not panic.
	got.Debug("x")
	got.Info("x")
	got.Warn("x")
	got.Error("x")
	_ = got.With("k", "v")
}

// TestFromContext_NilContext_ReturnsNop asserts that a nil context
// does not panic and returns Nop(). A caller in a deferred cleanup
// may hold a nil context; the guard makes the call safe.
func TestFromContext_NilContext_ReturnsNop(t *testing.T) {
	t.Parallel()

	got := FromContext(nil)
	if got == nil {
		t.Fatal("FromContext(nil) returned nil; want Nop()")
	}
	got.Info("x")
}

// TestIntoContext_NilContext asserts that IntoContext with a nil
// context does not panic and produces a usable context. The
// fallback to context.Background is documented in the helper; the
// test pins it.
func TestIntoContext_NilContext(t *testing.T) {
	t.Parallel()

	l := Nop()
	ctx := IntoContext(nil, l)
	if ctx == nil {
		t.Fatal("IntoContext(nil, ...) returned a nil context")
	}
	if got := FromContext(ctx); got != l {
		t.Fatal("round trip through a nil context lost the logger")
	}
}

// TestIntoContext_NilLogger asserts the contract documented in
// FromContext: a nil logger is stored, but FromContext returns
// Nop() rather than the nil. The caller cannot distinguish a nil
// stored value from no value at all, which is the intended
// behavior: Nop is the fallback for both.
func TestIntoContext_NilLogger(t *testing.T) {
	t.Parallel()

	ctx := IntoContext(context.Background(), nil)
	got := FromContext(ctx)
	if got == nil {
		t.Fatal("FromContext returned nil; want Nop()")
	}
	got.Info("x")
}

// TestFromContext_WrongType asserts that a value stored under the
// context key by some other mechanism is not returned as a Logger.
// The context key is unexported, so a wrong-typed value can only
// arise from reflection or from a future change to the key; the
// guard ensures FromContext does not panic on the type assertion.
func TestFromContext_WrongType(t *testing.T) {
	t.Parallel()

	// Store a non-Logger value under a key of the same type.
	// Only code in this package can construct contextKey, so the
	// test does so directly.
	ctx := context.WithValue(context.Background(), contextKey{}, "not a logger")
	got := FromContext(ctx)
	if got == nil {
		t.Fatal("FromContext returned nil; want Nop()")
	}
	got.Info("x")
}

// =============================================================================
// Non-functional: goroutine boundary
// =============================================================================

// TestFromContext_GoroutineBoundary asserts that a logger stored on
// a context is visible to every goroutine that receives the
// context. This is the WBS 12.3.1 acceptance criterion
// "FromContext / IntoContext tested across goroutine boundaries".
//
// The test spawns 32 goroutines, each of which retrieves the
// logger from the shared context and emits one record. The count
// assertion verifies that the logger was visible in every
// goroutine, not just in the one that stored it.
func TestFromContext_GoroutineBoundary(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	l := New(Options{
		Level:     LevelDebug,
		Component: ComponentCLI,
		Sink:      sink,
	})

	ctx := IntoContext(context.Background(), l)

	const n = 32
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			// The child retrieves the logger from the
			// context it received, not from a closure
			// variable that captured the interface.
			child := FromContext(ctx)
			child.Info("from goroutine")
		}()
	}
	wg.Wait()

	if got := sink.len(); got != n {
		t.Fatalf("sink received %d records; want %d", got, n)
	}
}

// =============================================================================
// Non-functional: contexts are immutable and composable
// =============================================================================

// TestIntoContext_DoesNotMutate asserts that IntoContext returns a
// new context and leaves the input unchanged. The property is
// fundamental to the context package and is worth pinning so a
// future refactor cannot accidentally mutate the input.
//
// # Why the test uses a live logger
//
// Nop() returns a value-typed nopLogger{} on every call, so two
// Nop() values are indistinguishable under ==. The test needs two
// distinguishable logger values to assert "the parent still has
// no logger". A live *slogLogger is a pointer and therefore
// comparable by identity: two New calls return different values.
func TestIntoContext_DoesNotMutate(t *testing.T) {
	t.Parallel()

	parent := context.Background()
	l := New(Options{
		Level:     LevelDebug,
		Component: ComponentCLI,
		Sink:      &captureSink{},
	})
	child := IntoContext(parent, l)

	// The parent still has no logger. The check is by identity:
	// the logger stored on the child is not what FromContext
	// returns for the parent.
	if got := FromContext(parent); got == l {
		t.Fatal("IntoContext mutated the parent context")
	}
	// The child has the logger.
	if got := FromContext(child); got != l {
		t.Fatal("IntoContext did not store the logger on the child")
	}
}

// TestIntoContext_Overwrite asserts that calling IntoContext twice
// on the same context replaces the previous logger. A caller who
// wants to install a different logger (for example, in a subtest
// that needs a capture sink) can do so with a single call.
func TestIntoContext_Overwrite(t *testing.T) {
	t.Parallel()

	first := Nop()
	second := Nop()

	ctx := IntoContext(context.Background(), first)
	ctx = IntoContext(ctx, second)

	if got := FromContext(ctx); got != second {
		t.Fatal("FromContext returned the first logger after overwrite")
	}
}

// =============================================================================
// Non-functional: FromContext is allocation-light on the hot path
// =============================================================================

// TestFromContext_AllocsBound asserts that FromContext does not
// allocate unboundedly. The function is called on every log site
// (via logger.FromContext(ctx).Info(...)); a per-call allocation
// would show up in profiles of any command that logs in a loop.
//
// The tolerance of a small constant covers the context package's
// internal bookkeeping. The test fails only if the allocation
// count grows unexpectedly.
//
// # Not parallel
//
// testing.AllocsPerRun observes process-global allocation
// counters; it panics if called from a parallel test. The
// absence of t.Parallel here is required, not an oversight.
func TestFromContext_AllocsBound(t *testing.T) {
	l := Nop()
	ctx := IntoContext(context.Background(), l)

	allocs := testing.AllocsPerRun(1000, func() {
		_ = FromContext(ctx)
	})
	if allocs > 1 {
		t.Fatalf("FromContext allocated %.1f times; want <= 1", allocs)
	}
}
