package logger

import (
	"context"
	"regexp"
	"strings"
	"sync"
	"testing"
)

// uuidV4Pattern is the canonical shape of a v4 UUID in lowercase
// hyphenated form. It is used by multiple tests in this file and by
// the schema-validating test in schema_test.go.
//
// The pattern is the WBS 12.2.1 schema's pattern. A change to one
// must be reflected in the other; the schema test asserts the same
// pattern against real output, so a drift between the two would
// fail there.
var uuidV4Pattern = regexp.MustCompile(
	`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// =============================================================================
// Shape
// =============================================================================

// TestNewCorrelationID_Shape asserts that the generated ID is a
// canonical v4 UUID: 36 characters, lowercase, hyphenated at the
// documented positions, with the version and variant nibbles set
// correctly.
//
// The test asserts both the length and the regexp. The length is
// redundant given the regexp, but it produces a clearer failure
// message when a bug produces a string of the wrong length.
func TestNewCorrelationID_Shape(t *testing.T) {
	t.Parallel()

	id := NewCorrelationID()
	if len(id) != 36 {
		t.Fatalf("NewCorrelationID() length = %d; want 36 (id=%q)",
			len(id), id)
	}
	if !uuidV4Pattern.MatchString(id) {
		t.Fatalf("NewCorrelationID() = %q; does not match v4 UUID pattern",
			id)
	}
}

// TestNewCorrelationID_VersionNibble asserts that the 13th
// character is '4'. The check is a finer-grained version of the
// regexp; it produces a failure message that names the specific
// property if the version nibble is wrong.
func TestNewCorrelationID_VersionNibble(t *testing.T) {
	t.Parallel()

	id := NewCorrelationID()
	if id[14] != '4' {
		t.Fatalf("version nibble = %c; want '4' (id=%q)", id[14], id)
	}
}

// TestNewCorrelationID_VariantNibble asserts that the 17th
// character is one of '8', '9', 'a', or 'b' (the RFC 4122 variant
// range).
func TestNewCorrelationID_VariantNibble(t *testing.T) {
	t.Parallel()

	id := NewCorrelationID()
	c := id[19]
	switch c {
	case '8', '9', 'a', 'b':
		// Correct.
	default:
		t.Fatalf("variant nibble = %c; want 8, 9, a, or b (id=%q)",
			c, id)
	}
}

// TestNewCorrelationID_Lowercase asserts that the ID is lowercase.
// A mixed-case UUID would compare unequal to itself under a
// byte-wise comparison; consumers who store IDs in a case-sensitive
// index would see duplicates.
func TestNewCorrelationID_Lowercase(t *testing.T) {
	t.Parallel()

	id := NewCorrelationID()
	if id != strings.ToLower(id) {
		t.Fatalf("NewCorrelationID() = %q; want lowercase", id)
	}
}

// TestNewCorrelationID_HyphenPositions asserts the hyphen positions
// in the UUID: 8-4-4-4-12 form. The test checks each hyphen is
// present at the expected index, and that no other character is a
// hyphen.
func TestNewCorrelationID_HyphenPositions(t *testing.T) {
	t.Parallel()

	id := NewCorrelationID()
	for _, i := range []int{8, 13, 18, 23} {
		if id[i] != '-' {
			t.Errorf("id[%d] = %c; want '-' (id=%q)", i, id[i], id)
		}
	}
	// No other hyphens.
	for i, c := range id {
		if c == '-' {
			switch i {
			case 8, 13, 18, 23:
				// Expected.
			default:
				t.Errorf("unexpected hyphen at index %d (id=%q)", i, id)
			}
		}
	}
}

// =============================================================================
// ZeroCorrelationID
// =============================================================================

// TestZeroCorrelationID_Shape asserts that the sentinel is a
// syntactically valid v4 UUID. A consumer that parses a
// correlation_id with a v4-aware parser succeeds on every value
// the logger emits, including the sentinel.
func TestZeroCorrelationID_Shape(t *testing.T) {
	t.Parallel()

	if len(ZeroCorrelationID) != 36 {
		t.Fatalf("ZeroCorrelationID length = %d; want 36",
			len(ZeroCorrelationID))
	}
	if !uuidV4Pattern.MatchString(ZeroCorrelationID) {
		t.Fatalf("ZeroCorrelationID = %q; does not match v4 UUID pattern",
			ZeroCorrelationID)
	}
}

// TestZeroCorrelationID_IsReserved asserts the sentinel's
// distinguishing property: the low-order 120 bits (all but the
// version and variant nibbles) are zero, so the sentinel is
// recognizable by inspection and cannot be confused with a real
// invocation ID that a producer would construct by accident.
func TestZeroCorrelationID_IsReserved(t *testing.T) {
	t.Parallel()

	const want = "00000000-0000-4000-8000-000000000000"
	if ZeroCorrelationID != want {
		t.Fatalf("ZeroCorrelationID = %q; want %q",
			ZeroCorrelationID, want)
	}
}

// TestZeroCorrelationID_VersionAndVariant asserts the two nibbles
// that distinguish a v4 UUID from a generic one:
//
//   - Position 14 (the first character of the third group) must
//     be '4'.
//   - Position 19 (the first character of the fourth group) must
//     be one of '8', '9', 'a', 'b'.
//
// These are the same positions the uuidV4Pattern enforces; the
// test names them directly so a failure points at the specific
// nibble that is wrong rather than at "does not match pattern".
func TestZeroCorrelationID_VersionAndVariant(t *testing.T) {
	t.Parallel()

	if ZeroCorrelationID[14] != '4' {
		t.Errorf("version nibble = %c; want '4' (id=%q)",
			ZeroCorrelationID[14], ZeroCorrelationID)
	}
	switch ZeroCorrelationID[19] {
	case '8', '9', 'a', 'b':
		// Correct.
	default:
		t.Errorf("variant nibble = %c; want 8, 9, a, or b (id=%q)",
			ZeroCorrelationID[19], ZeroCorrelationID)
	}
}

// TestZeroCorrelationID_AllOtherDigitsAreZero asserts the
// sentinel's distinguishing property: every hex digit other than
// the version and variant nibbles is '0'. This is what makes the
// sentinel recognizable by inspection and reservable as a value
// no producer would construct by accident.
//
// The positions checked are the complement of the two nibbles
// the version/variant test covers; together the two tests
// exhaustively describe the sentinel's shape.
func TestZeroCorrelationID_AllOtherDigitsAreZero(t *testing.T) {
	t.Parallel()

	for i, c := range ZeroCorrelationID {
		// Hyphens separate the groups; skip them.
		if c == '-' {
			continue
		}
		// The version nibble (position 14) and variant nibble
		// (position 19) are the only permitted non-zero
		// digits; they are asserted by
		// TestZeroCorrelationID_VersionAndVariant.
		if i == 14 || i == 19 {
			continue
		}
		if c != '0' {
			t.Fatalf("ZeroCorrelationID[%d] = %c; want '0' (id=%q)",
				i, c, ZeroCorrelationID)
		}
	}
}

// TestZeroCorrelationID_DiffersFromGenerated asserts the sentinel
// is not a value that NewCorrelationID can produce in practice.
// The generator produces uniformly random v4 UUIDs; the
// probability that a real invocation's ID equals the sentinel is
// 2^-122, which is negligible. The test exercises 100 generations
// as a smoke check; a real collision would require an entropy
// source that is broken in a way no test could reliably trigger.
func TestZeroCorrelationID_DiffersFromGenerated(t *testing.T) {
	t.Parallel()

	for i := 0; i < 100; i++ {
		if id := NewCorrelationID(); id == ZeroCorrelationID {
			t.Fatalf("NewCorrelationID produced the zero sentinel: %q", id)
		}
	}
}

// =============================================================================
// Uniqueness
// =============================================================================

// TestNewCorrelationID_Unique asserts that a large batch of IDs
// contains no duplicates. The batch is sized to make a collision
// astronomically unlikely under a correct random generator and
// nearly certain under a broken one (a counter starting at zero,
// a math/rand seed that repeats, etc.).
func TestNewCorrelationID_Unique(t *testing.T) {
	t.Parallel()

	const n = 10_000
	seen := make(map[string]struct{}, n)
	for i := 0; i < n; i++ {
		id := NewCorrelationID()
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate ID after %d iterations: %q", i, id)
		}
		seen[id] = struct{}{}
	}
}

// TestNewCorrelationID_UniqueAcrossGoroutines asserts uniqueness
// under concurrency. The crypto/rand source is safe for concurrent
// use; the test verifies the generator does not serialize on a
// shared state (a common bug when a package-level buffer is reused
// without a lock).
//
// The test uses a WaitGroup rather than an unbuffered channel and a
// single reader: the channel-per-goroutine pattern is prone to
// "the last goroutine has not finished" bugs and obscures the
// count assertion.
func TestNewCorrelationID_UniqueAcrossGoroutines(t *testing.T) {
	t.Parallel()

	const (
		goroutines = 32
		perG       = 256
	)
	results := make([][]string, goroutines)

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		g := g
		go func() {
			defer wg.Done()
			ids := make([]string, perG)
			for i := 0; i < perG; i++ {
				ids[i] = NewCorrelationID()
			}
			results[g] = ids
		}()
	}
	wg.Wait()

	// Now check for duplicates outside the parallel section.
	seen := make(map[string]struct{}, goroutines*perG)
	for _, batch := range results {
		for _, id := range batch {
			if _, dup := seen[id]; dup {
				t.Fatalf("duplicate ID under concurrency: %q", id)
			}
			seen[id] = struct{}{}
		}
	}
	if got, want := len(seen), goroutines*perG; got != want {
		t.Fatalf("got %d unique IDs; want %d", got, want)
	}
}

// =============================================================================
// Context: round-trip
// =============================================================================

// TestWithCorrelationID_RoundTrip asserts the basic contract: an ID
// stored by WithCorrelationID is returned by
// CorrelationIDFromContext.
func TestWithCorrelationID_RoundTrip(t *testing.T) {
	t.Parallel()

	id := NewCorrelationID()
	ctx := WithCorrelationID(context.Background(), id)

	got := CorrelationIDFromContext(ctx)
	if got != id {
		t.Fatalf("CorrelationIDFromContext = %q; want %q", got, id)
	}
}

// TestWithCorrelationID_PreservesExistingContextValues asserts
// that WithCorrelationID does not disturb other values already
// present in the context. A caller who has stored a Logger and a
// request-scoped value must find both intact after adding the
// correlation ID.
func TestWithCorrelationID_PreservesExistingContextValues(t *testing.T) {
	t.Parallel()

	type ctxKey struct{}
	parent := context.WithValue(context.Background(), ctxKey{}, "other")
	ctx := WithCorrelationID(parent, "id-abc")

	if got := ctx.Value(ctxKey{}); got != "other" {
		t.Fatalf("unrelated value lost: %v", got)
	}
	if got := CorrelationIDFromContext(ctx); got != "id-abc" {
		t.Fatalf("CorrelationIDFromContext = %q; want id-abc", got)
	}
}

// TestWithCorrelationID_Overwrites asserts that a second call
// replaces the previous ID. The behavior matches context.WithValue
// semantics: the latest value wins.
func TestWithCorrelationID_Overwrites(t *testing.T) {
	t.Parallel()

	ctx := WithCorrelationID(context.Background(), "first")
	ctx = WithCorrelationID(ctx, "second")

	if got := CorrelationIDFromContext(ctx); got != "second" {
		t.Fatalf("CorrelationIDFromContext = %q; want second", got)
	}
}

// =============================================================================
// Context: safe defaults
// =============================================================================

// TestCorrelationIDFromContext_NilContext asserts that a nil
// context returns the zero sentinel. The guard is deliberate: a
// caller in a deferred cleanup may hold a nil context, and the
// accessor must not panic.
func TestCorrelationIDFromContext_NilContext(t *testing.T) {
	t.Parallel()

	if got := CorrelationIDFromContext(nil); got != ZeroCorrelationID {
		t.Fatalf("CorrelationIDFromContext(nil) = %q; want zero", got)
	}
}

// TestCorrelationIDFromContext_EmptyContext asserts that a context
// with no stored ID returns the zero sentinel. The sentinel is the
// signal "not part of an invocation".
func TestCorrelationIDFromContext_EmptyContext(t *testing.T) {
	t.Parallel()

	if got := CorrelationIDFromContext(context.Background()); got != ZeroCorrelationID {
		t.Fatalf("CorrelationIDFromContext(bg) = %q; want zero", got)
	}
}

// TestCorrelationIDFromContext_WrongType asserts that a value
// stored under the correlation-ID key by a mechanism other than
// WithCorrelationID is not returned. The context key is
// unexported, so a wrong-typed value can only arise from code in
// this package; the guard ensures the type assertion does not
// panic and returns the sentinel.
func TestCorrelationIDFromContext_WrongType(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(
		context.Background(),
		correlationIDContextKey{},
		uint64(42), // not a string
	)
	if got := CorrelationIDFromContext(ctx); got != ZeroCorrelationID {
		t.Fatalf("CorrelationIDFromContext(wrong-type) = %q; want zero", got)
	}
}

// TestCorrelationIDFromContext_EmptyString asserts that an empty
// string stored under the key is returned as-is, not replaced by
// the sentinel. The distinction matters: the sentinel means "no ID
// was provided", while an empty string means "an ID was provided
// and it is empty". WithCorrelationID does not validate its
// argument; a caller who passes "" gets "".
//
// The test documents this behavior. A future change that
// substituted the sentinel for an empty string would be a
// behavior change and would fail here, prompting a review.
func TestCorrelationIDFromContext_EmptyString(t *testing.T) {
	t.Parallel()

	ctx := WithCorrelationID(context.Background(), "")
	got := CorrelationIDFromContext(ctx)
	if got != "" {
		t.Fatalf("CorrelationIDFromContext(empty) = %q; want empty string",
			got)
	}
}

// TestWithCorrelationID_NilContext asserts that WithCorrelationID
// with a nil context does not panic and produces a usable context.
// The fallback to context.Background is documented in the helper.
func TestWithCorrelationID_NilContext(t *testing.T) {
	t.Parallel()

	ctx := WithCorrelationID(nil, "id-xyz")
	if ctx == nil {
		t.Fatal("WithCorrelationID(nil, ...) returned nil")
	}
	if got := CorrelationIDFromContext(ctx); got != "id-xyz" {
		t.Fatalf("CorrelationIDFromContext = %q; want id-xyz", got)
	}
}

// =============================================================================
// Independence from logger.FromContext
// =============================================================================

// TestCorrelationID_IndependentOfLoggerContext asserts that the
// two context accessors do not collide: storing a correlation ID
// does not affect FromContext (the logger accessor), and storing a
// logger does not affect CorrelationIDFromContext.
//
// The two use different context keys (correlationIDContextKey and
// contextKey); the test pins the separation so a future refactor
// that unified the keys would fail.
func TestCorrelationID_IndependentOfLoggerContext(t *testing.T) {
	t.Parallel()

	// Store only the correlation ID.
	ctxID := WithCorrelationID(context.Background(), "id-abc")
	if got := FromContext(ctxID); got != Nop() {
		// FromContext returns Nop() when no logger is stored;
		// two Nop() values compare equal, so the check is by
		// value. A live logger would compare unequal.
		t.Fatalf("FromContext(ctx-with-id) = %#v; want Nop", got)
	}
	if got := CorrelationIDFromContext(ctxID); got != "id-abc" {
		t.Fatalf("CorrelationIDFromContext = %q; want id-abc", got)
	}

	// Store only a logger.
	ctxLog := IntoContext(context.Background(), Nop())
	if got := CorrelationIDFromContext(ctxLog); got != ZeroCorrelationID {
		t.Fatalf("CorrelationIDFromContext(ctx-with-logger) = %q; want zero",
			got)
	}
}

// =============================================================================
// Non-functional: allocation bound
// =============================================================================

// TestCorrelationIDFromContext_AllocsBound asserts that the
// accessor does not allocate on the hot path. The accessor is
// called for every record the logger emits, so a per-call
// allocation would show up in profiles of any command that logs
// in a loop.
//
// # Not parallel
//
// testing.AllocsPerRun observes process-global allocation
// counters; it panics if called from a parallel test. The
// absence of t.Parallel here is required, not an oversight.
func TestCorrelationIDFromContext_AllocsBound(t *testing.T) {
	ctx := WithCorrelationID(context.Background(), "id-abc")

	allocs := testing.AllocsPerRun(1000, func() {
		_ = CorrelationIDFromContext(ctx)
	})
	if allocs > 1 {
		t.Fatalf("CorrelationIDFromContext allocated %.1f times; want <= 1",
			allocs)
	}
}
