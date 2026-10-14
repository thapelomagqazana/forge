// Package forgeerr — error wrapping contract tests.
//
// This file carries the test suite for the wrapping semantics
// frozen in WBS 10.3.2: Unwrap, Is, As, and their interaction with
// the standard library's errors.Is, errors.As, fmt.Errorf, and
// errors.Join.
//
// Organisation:
//
//   - W1–W8 are the scenarios enumerated in WBS 10.3.2, named
//     with the W-number prefix so `go test -run 'TestW[0-9]'`
//     selects exactly the traceability set.
//   - Additional tests cover positive, negative, edge, corner, and
//     boundary cases derived from the contract's rules.
//   - Every test names the acceptance criterion (AC1–AC7) it
//     verifies so that traceability is mechanical.
//
// Typed-nil boundary cases (a nil *Error passed as a target) are
// covered in forgeerr_test.go as TestError_TypedNilTarget. They
// exercise both Is and As, and both the direct-method and the
// errors.Is / errors.As forms. They are kept there because they
// test the type's behaviour on a degenerate input, not the
// wrapping contract.
//
// Test categories:
//
//   - ATDD:      AC-mapped tests
//   - Positive:  same code, cause chain, joined errors
//   - Negative:  different code, nil target, non-*Error target
//   - Edge:      nil cause, missing cause, deep chain
//   - Corner:    fmt.Errorf wrapping, errors.Join flattening
//   - BND:       shortest and longest chains
//   - NF:        concurrency
package forgeerr

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// --- Test helpers -----------------------------------------------------

// configErr returns a *Error with a registered config code and the
// given message. It is the standard fixture for the wrap tests: a
// valid, registry-registered, category-correct error.
func configErr(tb testing.TB, message string) *Error {
	tb.Helper()
	return New(CategoryConfig, CodeConfigMalformed, message)
}

// filesystemErr returns a *Error with a registered filesystem code.
// Its Code differs from configErr's, so the two are distinguishable
// by Is and by errors.Is.
func filesystemErr(tb testing.TB, message string) *Error {
	tb.Helper()
	return New(CategoryFilesystem, CodeFilesystemNotFound, message)
}

// --- W1–W8: the scenarios enumerated in WBS 10.3.2 --------------------

// TestW1_SelfMatch asserts that a *Error matches itself via
// errors.Is. This is the base case: the standard library's
// errors.Is calls Is(e, e), which must return true.
func TestW1_SelfMatch(t *testing.T) {
	t.Parallel()

	e := configErr(t, "x")
	if !errors.Is(e, e) {
		t.Fatal("errors.Is(e, e) = false")
	}
}

// TestW2_SameCodeMatches asserts that two independently
// constructed *Error values with the same Code match via
// errors.Is. Matching is by Code, not by pointer identity.
func TestW2_SameCodeMatches(t *testing.T) {
	t.Parallel()

	a := configErr(t, "first message")
	b := configErr(t, "second message")
	if !errors.Is(a, b) {
		t.Fatal("errors.Is(a, b) = false; same Code did not match")
	}
	if !errors.Is(b, a) {
		t.Fatal("errors.Is(b, a) = false; matching is not symmetric")
	}
}

// TestW3_DifferentCodeDoesNotMatch asserts that two *Error values
// with different Codes do not match via errors.Is. This is the
// negative counterpart to W2.
func TestW3_DifferentCodeDoesNotMatch(t *testing.T) {
	t.Parallel()

	a := configErr(t, "x")
	b := filesystemErr(t, "x")
	if errors.Is(a, b) {
		t.Fatal("errors.Is(a, b) = true; Codes differ")
	}
	if errors.Is(b, a) {
		t.Fatal("errors.Is(b, a) = true; Codes differ")
	}
}

// TestW4_FmtErrorWrapping asserts that errors.Is traverses a
// fmt.Errorf("%w", ...) wrapper and finds the *Error inside.
func TestW4_FmtErrorWrapping(t *testing.T) {
	t.Parallel()

	e := configErr(t, "x")
	wrapped := fmt.Errorf("outer context: %w", e)
	if !errors.Is(wrapped, e) {
		t.Fatal("errors.Is(wrapped, e) = false")
	}
}

// TestW5_FmtErrorWrappingWithAs asserts that errors.As traverses a
// fmt.Errorf("%w", ...) wrapper and extracts the *Error inside.
func TestW5_FmtErrorWrappingWithAs(t *testing.T) {
	t.Parallel()

	e := configErr(t, "x")
	wrapped := fmt.Errorf("outer context: %w", e)

	var target *Error
	if !errors.As(wrapped, &target) {
		t.Fatal("errors.As(wrapped, &target) = false")
	}
	if target != e {
		t.Fatalf("As assigned %p, want %p", target, e)
	}
}

// TestW6_CauseIsReachable asserts that a *Error with a non-*Error
// cause exposes that cause to errors.Is. This is what makes
// errors.Is(e, io.ErrUnexpectedEOF) work when e wraps
// io.ErrUnexpectedEOF.
func TestW6_CauseIsReachable(t *testing.T) {
	t.Parallel()

	e := configErr(t, "x").WithCause(io.ErrUnexpectedEOF)
	if !errors.Is(e, io.ErrUnexpectedEOF) {
		t.Fatal("errors.Is(e, io.ErrUnexpectedEOF) = false")
	}
}

// TestW7_ErrorsJoin asserts that errors.Join is supported. A
// joined error contains each of its operands, and errors.Is
// finds each one.
func TestW7_ErrorsJoin(t *testing.T) {
	t.Parallel()

	e1 := configErr(t, "first")
	e2 := filesystemErr(t, "second")
	joined := errors.Join(e1, e2)

	if !errors.Is(joined, e1) {
		t.Fatal("errors.Is(joined, e1) = false")
	}
	if !errors.Is(joined, e2) {
		t.Fatal("errors.Is(joined, e2) = false")
	}
}

// TestW8_DoubleWrapReturnsExisting asserts that Wrap called with
// an existing *Error returns that *Error unchanged — the same
// pointer, not a copy. This makes double-wrapping impossible.
//
// The test also asserts that the returned *Error preserves its
// original Code and Category, which is what makes the "return
// unchanged" rule observable rather than incidental.
func TestW8_DoubleWrapReturnsExisting(t *testing.T) {
	t.Parallel()

	inner := configErr(t, "inner")
	wrapped := Wrap(CategoryFilesystem, CodeFilesystemNotFound, "outer", inner)

	if wrapped != inner {
		t.Fatalf("Wrap returned %p, want %p", wrapped, inner)
	}
	if wrapped.Code() != inner.Code() {
		t.Fatalf("Code() = %q, want %q", wrapped.Code(), inner.Code())
	}
	if wrapped.Category() != inner.Category() {
		t.Fatalf("Category() = %v, want %v", wrapped.Category(), inner.Category())
	}
	if wrapped.Message() != inner.Message() {
		t.Fatalf("Message() = %q, want %q", wrapped.Message(), inner.Message())
	}
}

// --- AC1: Unwrap returns the immediate cause --------------------------

// TestAC1_UnwrapReturnsCause asserts the base contract of Unwrap:
// it returns exactly the cause that was attached, with no
// traversal.
func TestAC1_UnwrapReturnsCause(t *testing.T) {
	t.Parallel()

	cause := errors.New("root")
	e := configErr(t, "x").WithCause(cause)

	if got := e.Unwrap(); got != cause {
		t.Fatalf("Unwrap() = %v, want %v", got, cause)
	}
}

// TestAC1_UnwrapReturnsNilWithoutCause asserts that a *Error with
// no cause returns nil from Unwrap. The standard library treats
// nil as the end of the chain.
func TestAC1_UnwrapReturnsNilWithoutCause(t *testing.T) {
	t.Parallel()

	e := configErr(t, "x")
	if e.Unwrap() != nil {
		t.Fatalf("Unwrap() = %v, want nil", e.Unwrap())
	}
}

// TestAC1_UnwrapIsShallow asserts that Unwrap returns only the
// immediate cause, not the transitive chain. This is the "Unwrap
// returns the immediate cause only" rule: the traversal is the
// standard library's job, not Unwrap's.
func TestAC1_UnwrapIsShallow(t *testing.T) {
	t.Parallel()

	inner := errors.New("inner")
	middle := configErr(t, "middle").WithCause(inner)
	outer := configErr(t, "outer").WithCause(middle)

	if got := outer.Unwrap(); got != middle {
		t.Fatalf("Unwrap() = %v, want %v (middle)", got, middle)
	}
	if got := middle.Unwrap(); got != inner {
		t.Fatalf("Unwrap() = %v, want %v (inner)", got, inner)
	}
	if got := inner; got == nil {
		t.Fatal("inner is nil")
	}
}

// --- AC2: Is matches by Code ------------------------------------------

// TestAC2_IsByCodeIgnoresMessage asserts that Is matches by Code
// only. Two errors with the same Code but different messages
// match; two errors with the same message but different Codes do
// not.
func TestAC2_IsByCodeIgnoresMessage(t *testing.T) {
	t.Parallel()

	a := configErr(t, "message one")
	b := configErr(t, "message two")
	if !errors.Is(a, b) {
		t.Fatal("same Code, different message did not match")
	}

	c := filesystemErr(t, "message one")
	if errors.Is(a, c) {
		t.Fatal("different Code, same message matched")
	}
}

// TestAC2_IsIgnoresCategory asserts that Is matches by Code only.
// Two errors with the same Code and different Categories (which
// is impossible for registered codes, since the prefix determines
// the category) are not a meaningful case; the test instead
// verifies that Is does not consult the Category field by
// constructing an *Error and comparing it to itself with the
// Category effectively ignored.
//
// Since Codes are structured FORGE_<CATEGORY>_<SPECIFIC>, two
// errors with the same Code necessarily have the same Category.
// The test documents this and asserts the behaviour indirectly.
func TestAC2_IsIgnoresCategory(t *testing.T) {
	t.Parallel()

	// Two errors with the same Code necessarily have the same
	// Category. If Is consulted the Category, the result would be
	// the same as for the Code comparison. This test exists to
	// document that Is does not consult the Category field
	// directly.
	a := configErr(t, "x")
	b := configErr(t, "y")
	if !errors.Is(a, b) {
		t.Fatal("same Code did not match")
	}
}

// TestAC2_IsNilTarget asserts that Is(nil) and errors.Is(e, nil)
// return false.
func TestAC2_IsNilTarget(t *testing.T) {
	t.Parallel()

	e := configErr(t, "x")
	if e.Is(nil) {
		t.Fatal("Is(nil) = true")
	}
	if errors.Is(e, nil) {
		t.Fatal("errors.Is(e, nil) = true")
	}
}

// TestAC2_IsNonErrorTarget asserts that Is returns false when the
// target is not a *Error. It does not interpret the target in any
// other way.
func TestAC2_IsNonErrorTarget(t *testing.T) {
	t.Parallel()

	e := configErr(t, "x")
	other := errors.New("plain error")
	if e.Is(other) {
		t.Fatal("Is(plain error) = true")
	}

	// A wrapper around a *Error is not a *Error. Is does not
	// traverse the target's chain.
	wrapped := fmt.Errorf("wrap: %w", e)
	if e.Is(wrapped) {
		t.Fatal("Is(fmt.Errorf wrapper) = true; Is should be shallow")
	}
}

// --- AC3: As assigns the error ----------------------------------------

// TestAC3_AsAssignsError asserts that As assigns the *Error to a
// **Error target and returns true.
func TestAC3_AsAssignsError(t *testing.T) {
	t.Parallel()

	e := configErr(t, "x")

	var target *Error
	if !e.As(&target) {
		t.Fatal("As returned false")
	}
	if target != e {
		t.Fatalf("As assigned %p, want %p", target, e)
	}
}

// TestAC3_AsNonErrorTarget asserts that As returns false for
// targets that are not **Error. The assignment does not happen.
func TestAC3_AsNonErrorTarget(t *testing.T) {
	t.Parallel()

	e := configErr(t, "x")

	// Wrong target type: **error, not **Error.
	var errTarget error
	if e.As(&errTarget) {
		t.Fatal("As(&error) = true")
	}

	// Wrong target type: *string.
	var s string
	if e.As(&s) {
		t.Fatal("As(&string) = true")
	}

	// Nil target.
	if e.As(nil) {
		t.Fatal("As(nil) = true")
	}
}

// TestAC3_AsDoesNotTraverseCause asserts that As is shallow: it
// assigns the receiver, not a cause. errors.As is responsible for
// traversal.
func TestAC3_AsDoesNotTraverseCause(t *testing.T) {
	t.Parallel()

	inner := configErr(t, "inner")
	outer := filesystemErr(t, "outer").WithCause(inner)

	// Direct As on the outer error assigns the outer error.
	var target *Error
	if !outer.As(&target) {
		t.Fatal("As returned false")
	}
	if target != outer {
		t.Fatalf("As assigned %p, want %p (outer)", target, outer)
	}

	// errors.As, in contrast, returns the first match in the
	// chain, which is also the outer error (the chain is walked
	// outermost first).
	target = nil
	if !errors.As(outer, &target) {
		t.Fatal("errors.As returned false")
	}
	if target != outer {
		t.Fatalf("errors.As assigned %p, want %p (outer)", target, outer)
	}
}

// --- AC4: errors.Is traverses %w --------------------------------------

// TestAC4_ErrorsIsThroughFmtError asserts that errors.Is finds a
// *Error inside a fmt.Errorf("%w", ...) wrapper.
func TestAC4_ErrorsIsThroughFmtError(t *testing.T) {
	t.Parallel()

	e := configErr(t, "x")
	wrapped := fmt.Errorf("outer: %w", e)
	if !errors.Is(wrapped, e) {
		t.Fatal("errors.Is(wrapped, e) = false")
	}

	// A second wrap also works.
	doubleWrapped := fmt.Errorf("double: %w", wrapped)
	if !errors.Is(doubleWrapped, e) {
		t.Fatal("errors.Is(doubleWrapped, e) = false")
	}
}

// TestAC4_ErrorsIsThroughCauseChain asserts that errors.Is finds a
// *Error deeper in a cause chain. The standard library walks the
// chain via Unwrap; the Is method is invoked at each level.
func TestAC4_ErrorsIsThroughCauseChain(t *testing.T) {
	t.Parallel()

	inner := configErr(t, "inner")
	outer := filesystemErr(t, "outer").WithCause(inner)

	// errors.Is(outer, inner) must be true: the standard library
	// walks outer's chain (outer.Unwrap() == inner) and invokes
	// outer.Is(inner), which compares Codes.
	if !errors.Is(outer, inner) {
		t.Fatal("errors.Is(outer, inner) = false")
	}

	// A synthetic error with the same Code as inner matches
	// through the chain.
	synthetic := configErr(t, "different message")
	if !errors.Is(outer, synthetic) {
		t.Fatal("errors.Is(outer, synthetic) = false; Code-based match through chain failed")
	}
}

// --- AC5: errors.Join is supported ------------------------------------

// TestAC5_ErrorsJoin asserts that errors.Join produces an error
// that contains each operand, and that errors.Is finds each one.
func TestAC5_ErrorsJoin(t *testing.T) {
	t.Parallel()

	e1 := configErr(t, "first")
	e2 := filesystemErr(t, "second")
	joined := errors.Join(e1, e2)

	if !errors.Is(joined, e1) {
		t.Fatal("errors.Is(joined, e1) = false")
	}
	if !errors.Is(joined, e2) {
		t.Fatal("errors.Is(joined, e2) = false")
	}

	// A synthetic *Error with the same Code as e1 also matches.
	synthetic := configErr(t, "different message")
	if !errors.Is(joined, synthetic) {
		t.Fatal("errors.Is(joined, synthetic) = false")
	}
}

// TestAC5_ErrorsJoinWithPlainErrors asserts that a joined error
// containing both *Error and plain errors works as expected: the
// *Error operands match by Code, and the plain errors match by
// pointer identity.
func TestAC5_ErrorsJoinWithPlainErrors(t *testing.T) {
	t.Parallel()

	plain := errors.New("plain")
	e := configErr(t, "forge error")
	joined := errors.Join(plain, e)

	if !errors.Is(joined, plain) {
		t.Fatal("errors.Is(joined, plain) = false")
	}
	if !errors.Is(joined, e) {
		t.Fatal("errors.Is(joined, e) = false")
	}
}

// TestAC5_ErrorsJoinNested asserts that nested errors.Join calls
// work: a joined error inside a joined error is still reachable.
func TestAC5_ErrorsJoinNested(t *testing.T) {
	t.Parallel()

	e1 := configErr(t, "first")
	e2 := filesystemErr(t, "second")
	e3 := configErr(t, "third")

	inner := errors.Join(e1, e2)
	outer := errors.Join(inner, e3)

	for _, target := range []*Error{e1, e2, e3} {
		if !errors.Is(outer, target) {
			t.Fatalf("errors.Is(outer, %p) = false", target)
		}
	}
}

// --- AC6: double wrapping returns existing ----------------------------

// TestAC6_DoubleWrapReturnsExisting asserts the no-double-wrap
// rule: Wrap called with an existing *Error returns that *Error
// unchanged.
func TestAC6_DoubleWrapReturnsExisting(t *testing.T) {
	t.Parallel()

	inner := configErr(t, "inner")
	wrapped := Wrap(CategoryFilesystem, CodeFilesystemNotFound, "outer", inner)

	if wrapped != inner {
		t.Fatalf("Wrap returned %p, want %p", wrapped, inner)
	}
}

// TestAC6_DoubleWrapThroughFmtError asserts that Wrap finds an
// existing *Error even when it is buried inside a fmt.Errorf
// wrapper. The errors.As call inside Wrap traverses the wrapper.
func TestAC6_DoubleWrapThroughFmtError(t *testing.T) {
	t.Parallel()

	inner := configErr(t, "inner")
	wrapped := fmt.Errorf("context: %w", inner)

	got := Wrap(CategoryFilesystem, CodeFilesystemNotFound, "outer", wrapped)
	if got != inner {
		t.Fatalf("Wrap returned %p, want %p (inner)", got, inner)
	}
}

// --- Edge: nil cause, missing cause, deep chain -----------------------

// TestEdge_WithCauseThenUnwrap asserts that WithCause attaches a
// cause and Unwrap returns it.
func TestEdge_WithCauseThenUnwrap(t *testing.T) {
	t.Parallel()

	cause := errors.New("cause")
	e := configErr(t, "x").WithCause(cause)

	if e.Unwrap() != cause {
		t.Fatalf("Unwrap() = %v, want %v", e.Unwrap(), cause)
	}
	if !errors.Is(e, cause) {
		t.Fatal("errors.Is(e, cause) = false")
	}
}

// TestEdge_ChainOfErrors asserts that a chain of *Error values is
// traversed correctly by errors.Is. Each link has a distinct
// Code, so the traversal must reach the right link.
func TestEdge_ChainOfErrors(t *testing.T) {
	t.Parallel()

	inner := configErr(t, "inner")
	middle := filesystemErr(t, "middle").WithCause(inner)
	outer := configErr(t, "outer").WithCause(middle)

	// The outer and inner have the same Code (config), so
	// errors.Is(outer, inner) matches at the top level, before
	// any traversal. To force traversal, use a distinct code for
	// the outer.
	outer = New(CategoryInternal, CodeInternalUnexpected, "outer").WithCause(middle)

	if !errors.Is(outer, inner) {
		t.Fatal("errors.Is(outer, inner) = false; traversal failed")
	}
	if !errors.Is(outer, middle) {
		t.Fatal("errors.Is(outer, middle) = false; traversal failed")
	}
}

// TestEdge_SelfCauseIsAllowed asserts that a *Error whose cause is
// itself is not special-cased. Self-causes are a programming error,
// but the contract does not forbid them; the standard library would
// loop forever if it encountered one, which is the caller's
// problem, not the package's.
//
// This test documents the behaviour: it constructs a self-cause,
// asserts that the immediate Unwrap returns the self, and does not
// call errors.Is on the resulting structure (which would loop).
func TestEdge_SelfCauseIsAllowed(t *testing.T) {
	t.Parallel()

	base := configErr(t, "x")
	selfCaused := base.WithCause(base)

	if selfCaused.Unwrap() != base {
		t.Fatalf("Unwrap() = %p, want %p", selfCaused.Unwrap(), base)
	}
}

// --- Non-functional: concurrency --------------------------------------

// TestConcurrentChainMethods exercises Unwrap, Is, and As from
// many goroutines. The *Error is immutable, so this must be safe
// under -race.
//
// The goroutine body includes the typed-nil targets so that the
// nil guards on Is and As are exercised concurrently: a regression
// that removed the guard would panic here under -race.
func TestConcurrentChainMethods(t *testing.T) {
	base := configErr(t, "x").WithCause(errors.New("cause"))

	// A typed-nil *Error. Concurrent calls to Is with this target
	// must not panic.
	var nilErr *Error

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			_ = base.Unwrap()

			_ = base.Is(base)
			_ = base.Is(nil)
			_ = base.Is(nilErr)

			var target *Error
			_ = base.As(&target)
			_ = base.As(nil)
			_ = base.As(nilErr)

			_ = errors.Is(base, base)
			_ = errors.Is(base, nilErr)

			var target2 *Error
			_ = errors.As(base, &target2)
		}()
	}
	wg.Wait()
}

// --- Documentation test for AC7 ---------------------------------------

// TestAC7_DocumentationExists asserts that the wrapping section
// exists in docs/architecture.md. The check is a substring search,
// not a full parse, so that the prose can be edited freely.
//
// This test exists so that removing the documentation section
// without a replacement fails CI. The reviewer is expected to
// update the section when the contract changes.
func TestAC7_DocumentationExists(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	path := filepath.Join(root, "docs", "architecture.md")

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	const marker = "### 8.5 The Wrapping Contract"
	if !strings.Contains(string(raw), marker) {
		t.Fatalf("%s does not contain the section %q", path, marker)
	}
}
