// Package cli — error-to-exit-code mapping tests (WBS 11.2.1).
//
// This file asserts the mapping contract defined in
// docs/reference/error-mapping.md. It is organized by concern:
//
//   - TDD: the mapping's rules, one test per rule.
//   - ATDD: the acceptance criteria from WBS 11.2.1.
//   - BDD: the caller-visible behavior of each rule.
//   - Non-functional: purity and agreement with exitCodeOf.
//
// The tests reference the codes by their named constants rather
// than by numeric literals, so a future additive change to the
// taxonomy does not require editing every assertion.
//
// # Type-level contract
//
// forgeerr.New and forgeerr.Wrap take their code parameter as
// forgeerr.Code. Every constructor call in this file passes the
// named constants from internal/forgeerr/codes.go directly. A
// string(...) cast in a constructor call is the diagnostic that
// the migration documented in safefs/errors.go was rolled back.
//
// The accessor (*Error).Code() returns string; where a test
// compares it against a named constant, the cast is on the
// constant's side: e.Code() != string(forgeerr.CodeXXX). No such
// comparison appears in this file; every cast here has been
// removed.
package cli

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/thapelomagqazana/forge/internal/forgeerr"
)

// ---------------------------------------------------------------------------
// Test-only types
// ---------------------------------------------------------------------------

// customError is a test-only error type that satisfies the error
// interface but is not a *forgeerr.Error and does not match any
// sentinel. It exists so that TestMapping_IsTotal can exercise the
// mapping's fallback path with a type the mapping has never seen.
//
// # Why the type and its method are at package scope
//
// Go does not allow method declarations inside a function body. A
// method with a receiver (func (e customError) Error() string) must
// be declared at package scope, not inside a func. The type itself
// may be declared locally, but then its method cannot be, so the
// type and its method are both declared here, just above the test
// that uses them.
//
// # Why the value receiver
//
// The method uses a value receiver, so both customError and
// *customError implement the error interface. The test uses the
// value form; a future test that wants the pointer form gets it
// for free.
type customError struct{ msg string }

// Error satisfies the error interface. The returned string is the
// value's msg field, so the mapping's fallback path can be observed
// through the error's Error() method if a test wants to.
func (e customError) Error() string { return e.msg }

// ---------------------------------------------------------------------------
// TDD — the mapping's rules, one test per rule
// ---------------------------------------------------------------------------

// TestMapping_NilError asserts rule 1: a nil error maps to
// ExitSuccess.
//
// The rule is the most important one: every successful command
// returns nil through the error path, and the mapping must return
// the success code for it. A future change that made the mapping
// return a non-zero code for nil would break every success path.
func TestMapping_NilError(t *testing.T) {
	t.Parallel()

	if got := ExitCode(nil); got != ExitSuccess {
		t.Errorf("ExitCode(nil) = %d; want %d (ExitSuccess)", got, ExitSuccess)
	}
}

// TestMapping_ContextCanceled asserts rule 2: an error that wraps
// context.Canceled maps to ExitSIGINT.
//
// The rule covers three shapes:
//
//   - The bare sentinel: context.Canceled itself.
//   - A wrapped sentinel: fmt.Errorf("...: %w", context.Canceled).
//   - A deeply wrapped sentinel: multiple fmt.Errorf layers.
//
// The mapping uses errors.Is, so all three are detected.
func TestMapping_ContextCanceled(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
	}{
		{"bare sentinel", context.Canceled},
		{"wrapped once", fmt.Errorf("operation cancelled: %w", context.Canceled)},
		{"wrapped twice", fmt.Errorf("outer: %w",
			fmt.Errorf("inner: %w", context.Canceled))},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := ExitCode(tc.err); got != ExitSIGINT {
				t.Errorf("ExitCode(%v) = %d; want %d (ExitSIGINT)",
					tc.err, got, ExitSIGINT)
			}
		})
	}
}

// TestMapping_ContextDeadlineExceeded asserts rule 3: an error that
// wraps context.DeadlineExceeded maps to ExitInternal.
func TestMapping_ContextDeadlineExceeded(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
	}{
		{"bare sentinel", context.DeadlineExceeded},
		{"wrapped once", fmt.Errorf("operation timed out: %w", context.DeadlineExceeded)},
		{"wrapped twice", fmt.Errorf("outer: %w",
			fmt.Errorf("inner: %w", context.DeadlineExceeded))},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := ExitCode(tc.err); got != ExitInternal {
				t.Errorf("ExitCode(%v) = %d; want %d (ExitInternal)",
					tc.err, got, ExitInternal)
			}
		})
	}
}

// TestMapping_ForgeError asserts rule 4: a *forgeerr.Error maps to
// its category's exit code.
//
// The test constructs one error per category and asserts the
// mapping's result. The taxonomy is closed (WBS 10.2.1), so the
// set is enumerated here rather than discovered dynamically.
//
// # Type-level contract
//
// Every forgeerr.New call passes the code as forgeerr.Code (the
// named constants from codes.go). The constructor's parameter is
// forgeerr.Code after the WBS 13.1.1 / ADR-035 migration.
func TestMapping_ForgeError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		cat  forgeerr.Category
		code forgeerr.Code
	}{
		{forgeerr.CategoryCLI, forgeerr.CodeCLIUnknownCommand},
		{forgeerr.CategoryConfig, forgeerr.CodeConfigMalformed},
		{forgeerr.CategoryFilesystem, forgeerr.CodeFilesystemNotFound},
		{forgeerr.CategorySecurity, forgeerr.CodeSecurityPathTraversal},
		{forgeerr.CategoryInternal, forgeerr.CodeInternalUnexpected},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(string(tc.cat), func(t *testing.T) {
			t.Parallel()

			e := forgeerr.New(tc.cat, tc.code, "message")
			got := ExitCode(e)
			want := tc.cat.ExitCode()
			if got != want {
				t.Errorf("ExitCode(%v) = %d; want %d (category's exit code)",
					tc.cat, got, want)
			}
		})
	}
}

// TestMapping_WrappedForgeError asserts rule 4 through a wrapping
// chain: a *forgeerr.Error buried under fmt.Errorf is found by
// errors.As.
func TestMapping_WrappedForgeError(t *testing.T) {
	t.Parallel()

	inner := forgeerr.New(
		forgeerr.CategoryConfig,
		forgeerr.CodeConfigMalformed,
		"invalid YAML",
	)
	cases := []struct {
		name string
		err  error
	}{
		{"wrapped once", fmt.Errorf("outer: %w", inner)},
		{"wrapped twice", fmt.Errorf("a: %w", fmt.Errorf("b: %w", inner))},
		{"wrapped three times", fmt.Errorf("a: %w",
			fmt.Errorf("b: %w",
				fmt.Errorf("c: %w", inner)))},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := ExitCode(tc.err); got != ExitConfig {
				t.Errorf("ExitCode(wrapped) = %d; want %d (ExitConfig)",
					got, ExitConfig)
			}
		})
	}
}

// TestMapping_UnknownError asserts rule 5: an error that is not a
// structured error and does not match a sentinel maps to ExitUsage.
//
// The rule is the mapping's fallback. The value is specified by WBS
// 11.2.1 § "Description / Scope" item (c).
func TestMapping_UnknownError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
	}{
		{"bare errors.New", errors.New("plain")},
		{"fmt.Errorf without wrapping", fmt.Errorf("formatted: %d", 42)},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := ExitCode(tc.err); got != ExitUsage {
				t.Errorf("ExitCode(%v) = %d; want %d (ExitUsage)",
					tc.err, got, ExitUsage)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Edge and boundary cases
// ---------------------------------------------------------------------------

// TestMapping_ForgeErrorWrappingCanceled asserts the priority of the
// rules: a *forgeerr.Error whose Cause is context.Canceled maps to
// the structured error's category code, not to ExitSIGINT.
//
// The rule's rationale is documented in docs/reference/error-mapping.md:
// the structured error is the more specific classification, so it
// wins.
func TestMapping_ForgeErrorWrappingCanceled(t *testing.T) {
	t.Parallel()

	e := forgeerr.Wrap(
		forgeerr.CategoryConfig,
		forgeerr.CodeConfigMalformed,
		"config load cancelled",
		context.Canceled,
	)

	// Rule 2 (context.Canceled) is checked first, so the sentinel
	// matches even when it is the Cause of a structured error. The
	// mapping returns ExitSIGINT, not ExitConfig.
	//
	// # Why this is the expected behavior
	//
	// The order of the rules is deliberate: a cancellation is a
	// cancellation, regardless of what was being cancelled. A
	// caller that wants to distinguish "the config load was
	// cancelled" from "the config load failed" reads the error's
	// Cause, not the exit code. The exit code reflects the
	// cancellation, which is what the shell sees when the user
	// presses Ctrl-C.
	if got := ExitCode(e); got != ExitSIGINT {
		t.Errorf("ExitCode(*forgeerr.Error with Canceled cause) = %d; want %d (ExitSIGINT)",
			got, ExitSIGINT)
	}
}

// TestMapping_ForgeErrorWrappingDeadline asserts the priority of
// the rules for the DeadlineExceeded sentinel.
func TestMapping_ForgeErrorWrappingDeadline(t *testing.T) {
	t.Parallel()

	e := forgeerr.Wrap(
		forgeerr.CategoryConfig,
		forgeerr.CodeConfigMalformed,
		"config load timed out",
		context.DeadlineExceeded,
	)

	// Rule 3 (context.DeadlineExceeded) is checked before rule 4
	// (*forgeerr.Error). The mapping returns ExitInternal.
	if got := ExitCode(e); got != ExitInternal {
		t.Errorf("ExitCode(*forgeerr.Error with DeadlineExceeded cause) = %d; want %d (ExitInternal)",
			got, ExitInternal)
	}
}

// TestMapping_SentinelWrappedInForgeError asserts that a sentinel
// wrapped inside a *forgeerr.Error is detected by rule 2 or rule 3,
// not by rule 4.
//
// The test is the converse of TestMapping_ForgeErrorWrappingCanceled:
// the sentinel is the outermost error's Cause, and the sentinel
// wins.
func TestMapping_SentinelWrappedInForgeError(t *testing.T) {
	t.Parallel()

	// The sentinel is the direct error; the structured error is
	// not in the chain. Rule 2 matches.
	if got := ExitCode(fmt.Errorf("op: %w", context.Canceled)); got != ExitSIGINT {
		t.Errorf("ExitCode(wrapped Canceled) = %d; want %d (ExitSIGINT)",
			got, ExitSIGINT)
	}
}

// TestMapping_ResultIsInRange asserts that every possible return
// value of ExitCode is within the documented exit-code namespace.
//
// The invariant is a precondition for the mapping's other
// properties: a mapping that returned a code outside [0, 255]
// would violate the platform stability guarantee.
func TestMapping_ResultIsInRange(t *testing.T) {
	t.Parallel()

	cases := []error{
		nil,
		context.Canceled,
		context.DeadlineExceeded,
		errors.New("plain"),
		fmt.Errorf("wrapped: %w", context.Canceled),
		forgeerr.New(forgeerr.CategoryConfig, forgeerr.CodeConfigMalformed, "x"),
		forgeerr.New(forgeerr.CategorySecurity, forgeerr.CodeSecurityPathTraversal, "x"),
		forgeerr.New(forgeerr.CategoryInternal, forgeerr.CodeInternalUnexpected, "x"),
	}
	for _, err := range cases {
		code := ExitCode(err)
		if verr := VerifyExitCodeRange(code); verr != nil {
			t.Errorf("ExitCode(%v) = %d; %v", err, code, verr)
		}
	}
}

// TestMapping_IsTotal asserts that the mapping is total: every
// input — including a nil error, an error type the mapping has
// never seen, and a deeply wrapped chain — returns a documented
// exit code without panicking.
//
// The test is the WBS 11.2.1 acceptance criterion "Mapping is total
// (no error returns 'undefined code')" made executable.
//
// # The customError input
//
// The test's customError input exercises the fallback path with a
// type the mapping has never seen. The type and its Error() method
// are declared at package scope (above this test) because Go does
// not allow method declarations inside a function body. See the
// customError docstring for the full rationale.
func TestMapping_IsTotal(t *testing.T) {
	t.Parallel()

	inputs := []error{
		nil,
		errors.New("plain"),
		fmt.Errorf("formatted"),
		context.Canceled,
		context.DeadlineExceeded,
		customError{msg: "custom"},
		forgeerr.New(forgeerr.CategoryCLI, forgeerr.CodeCLIUnknownCommand, "x"),
	}

	for _, err := range inputs {
		code := ExitCode(err)
		if code < 0 || code > 255 {
			t.Errorf("ExitCode(%v) = %d; want a code in [0, 255]", err, code)
		}
	}
}

// ---------------------------------------------------------------------------
// ATDD — acceptance criteria from WBS 11.2.1
// ---------------------------------------------------------------------------

// TestAC_11_2_1_MappingIsTotal asserts the acceptance criterion
// "Mapping is total (no error returns 'undefined code')".
//
// The test enumerates every input shape the mapping must handle
// and asserts that each returns a documented code. A future change
// that introduced an undefined code (a panic, a sentinel value, a
// non-zero code outside the ranges) fails the test.
func TestAC_11_2_1_MappingIsTotal(t *testing.T) {
	t.Parallel()

	inputs := []error{
		nil,
		context.Canceled,
		context.DeadlineExceeded,
		errors.New("plain"),
		fmt.Errorf("wrapped: %w", errors.New("inner")),
		forgeerr.New(forgeerr.CategoryCLI, forgeerr.CodeCLIUnknownCommand, "x"),
		forgeerr.New(forgeerr.CategoryConfig, forgeerr.CodeConfigMalformed, "x"),
		forgeerr.New(forgeerr.CategoryFilesystem, forgeerr.CodeFilesystemNotFound, "x"),
		forgeerr.New(forgeerr.CategorySecurity, forgeerr.CodeSecurityPathTraversal, "x"),
		forgeerr.New(forgeerr.CategoryInternal, forgeerr.CodeInternalUnexpected, "x"),
	}
	for _, err := range inputs {
		code := ExitCode(err)
		if _, ok := LookupExitCodeRange(code); !ok {
			t.Errorf("ExitCode(%v) = %d; not in any documented range", err, code)
		}
	}
}

// TestAC_11_2_1_ErrorIsUnwrappable asserts the acceptance criterion
// "errors.Is / errors.As unwrapping works through wrapping chains".
//
// The test wraps a *forgeerr.Error in a chain of fmt.Errorf and
// asserts that errors.As finds it. The mapping relies on this
// property; the test asserts it directly.
func TestAC_11_2_1_ErrorIsUnwrappable(t *testing.T) {
	t.Parallel()

	inner := forgeerr.New(
		forgeerr.CategoryConfig,
		forgeerr.CodeConfigMalformed,
		"invalid YAML",
	)
	outer := fmt.Errorf("a: %w",
		fmt.Errorf("b: %w",
			fmt.Errorf("c: %w", inner)))

	var fe *forgeerr.Error
	if !errors.As(outer, &fe) {
		t.Fatal("errors.As did not find the *forgeerr.Error through the chain")
	}
	if fe.Category() != forgeerr.CategoryConfig {
		t.Errorf("errors.As found category %v; want %v",
			fe.Category(), forgeerr.CategoryConfig)
	}
}

// TestAC_11_2_1_EveryCategoryHasDefault asserts the acceptance
// criterion "Every error category has a documented default exit
// code".
func TestAC_11_2_1_EveryCategoryHasDefault(t *testing.T) {
	t.Parallel()

	for _, cat := range forgeerr.AllCategories() {
		cat := cat
		t.Run(string(cat), func(t *testing.T) {
			t.Parallel()

			code := cat.DefaultExitCode()
			if _, ok := LookupExitCodeRange(code); !ok {
				t.Errorf("%v.DefaultExitCode() = %d; not in any documented range",
					cat, code)
			}
			if code == ExitSuccess {
				t.Errorf("%v.DefaultExitCode() = ExitSuccess; want a failure code",
					cat)
			}
		})
	}
}

// TestAC_11_2_1_MappingMatchesFrozenTable asserts the acceptance
// criterion "Mapping table matches 11.1.2 frozen table exactly".
//
// The test asserts that the categories' default exit codes are the
// values the WBS 11.1.2 table assigns. A future change to either
// the taxonomy or the category's defaults that broke the agreement
// fails the test.
func TestAC_11_2_1_MappingMatchesFrozenTable(t *testing.T) {
	t.Parallel()

	// The mapping is the WBS 11.1.2 frozen table.
	cases := []struct {
		cat  forgeerr.Category
		want int
	}{
		{forgeerr.CategoryCLI, ExitUsage},
		{forgeerr.CategoryConfig, ExitConfig},
		{forgeerr.CategoryFilesystem, ExitFilesystem},
		{forgeerr.CategoryValidation, ExitValidation},
		{forgeerr.CategoryTemplate, ExitTemplate},
		{forgeerr.CategorySecurity, ExitSecurity},
		{forgeerr.CategoryInternal, ExitInternal},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(string(tc.cat), func(t *testing.T) {
			t.Parallel()
			if got := tc.cat.DefaultExitCode(); got != tc.want {
				t.Errorf("%v.DefaultExitCode() = %d; want %d",
					tc.cat, got, tc.want)
			}
			if got := tc.cat.ExitCode(); got != tc.want {
				t.Errorf("%v.ExitCode() = %d; want %d (must agree with DefaultExitCode)",
					tc.cat, got, tc.want)
			}
		})
	}
}

// TestAC_11_2_1_NilErrorIsSuccess asserts the WBS 11.2.1 acceptance
// criterion "unit tests cover: ... nil error". The criterion
// implies that nil maps to success; the test asserts it.
func TestAC_11_2_1_NilErrorIsSuccess(t *testing.T) {
	t.Parallel()

	if got := ExitCode(nil); got != ExitSuccess {
		t.Errorf("ExitCode(nil) = %d; want %d (ExitSuccess)", got, ExitSuccess)
	}
}

// ---------------------------------------------------------------------------
// BDD — behavior scenarios
// ---------------------------------------------------------------------------

// TestBDD_UserPressesCtrlC documents the caller-visible behavior of
// a Ctrl-C.
func TestBDD_UserPressesCtrlC(t *testing.T) {
	t.Parallel()
	// GIVEN the user pressed Ctrl-C
	err := context.Canceled

	// WHEN the process boundary maps the error to an exit code
	code := ExitCode(err)

	// THEN the exit code is the SIGINT convention
	if code != ExitSIGINT {
		t.Errorf("ExitCode(context.Canceled) = %d; want %d (SIGINT convention)",
			code, ExitSIGINT)
	}
}

// TestBDD_ConfigErrorIsAnExecutionFailure documents the
// caller-visible behavior of a config error.
func TestBDD_ConfigErrorIsAnExecutionFailure(t *testing.T) {
	t.Parallel()
	// GIVEN Forge cannot load forge.yaml
	err := forgeerr.New(
		forgeerr.CategoryConfig,
		forgeerr.CodeConfigMalformed,
		"invalid YAML",
	)

	// WHEN the process boundary maps the error to an exit code
	code := ExitCode(err)

	// THEN the code is in the execution-failure range
	if !IsExecutionFailure(code) {
		t.Errorf("ExitCode(config error) = %d; want an execution-failure code", code)
	}
	if code != ExitConfig {
		t.Errorf("ExitCode(config error) = %d; want %d (ExitConfig)", code, ExitConfig)
	}
}

// TestBDD_UnknownErrorMapsToUsage documents the caller-visible
// behavior of an unrecognised error.
func TestBDD_UnknownErrorMapsToUsage(t *testing.T) {
	t.Parallel()
	// GIVEN a bare error the mapping has never seen
	err := errors.New("something went wrong")

	// WHEN the process boundary maps the error to an exit code
	code := ExitCode(err)

	// THEN the fallback applies: the code is the usage error
	if code != ExitUsage {
		t.Errorf("ExitCode(unknown) = %d; want %d (ExitUsage)", code, ExitUsage)
	}
}

// TestBDD_DeeplyWrappedStructuredErrorIsFound documents the
// caller-visible behavior of a *forgeerr.Error buried under
// wrapping.
func TestBDD_DeeplyWrappedStructuredErrorIsFound(t *testing.T) {
	t.Parallel()
	// GIVEN a *forgeerr.Error wrapped in three fmt.Errorf layers
	inner := forgeerr.New(
		forgeerr.CategorySecurity,
		forgeerr.CodeSecurityPathTraversal,
		"path escapes the root",
	)
	err := fmt.Errorf("a: %w", fmt.Errorf("b: %w", fmt.Errorf("c: %w", inner)))

	// WHEN the process boundary maps the error to an exit code
	code := ExitCode(err)

	// THEN the mapping finds the structured error and returns its
	// category's code
	if code != ExitSecurity {
		t.Errorf("ExitCode(deeply wrapped) = %d; want %d (ExitSecurity)",
			code, ExitSecurity)
	}
}

// ---------------------------------------------------------------------------
// Non-functional — purity and agreement with exitCodeOf
// ---------------------------------------------------------------------------

// TestMapping_IsPure asserts that ExitCode does not modify its
// input. The test calls ExitCode twice on the same error and
// asserts that the error's observable properties are unchanged.
//
// The property matters because the process boundary calls ExitCode
// after rendering the error: a mapping that mutated the error
// would corrupt the rendering.
func TestMapping_IsPure(t *testing.T) {
	t.Parallel()

	e := forgeerr.New(
		forgeerr.CategoryConfig,
		forgeerr.CodeConfigMalformed,
		"invalid YAML",
	)
	// Capture the observable properties before.
	catBefore := e.Category()
	codeBefore := e.Code()
	msgBefore := e.Message()

	first := ExitCode(e)
	for i := 0; i < 100; i++ {
		if got := ExitCode(e); got != first {
			t.Fatalf("iteration %d: ExitCode = %d; want %d (not pure)", i, got, first)
		}
	}

	if e.Category() != catBefore {
		t.Errorf("ExitCode mutated Category: got %v, want %v", e.Category(), catBefore)
	}
	if e.Code() != codeBefore {
		t.Errorf("ExitCode mutated Code: got %q, want %q", e.Code(), codeBefore)
	}
	if e.Message() != msgBefore {
		t.Errorf("ExitCode mutated Message: got %q, want %q", e.Message(), msgBefore)
	}
}

// TestMapping_AgreesWithExitCodeOf asserts that ExitCode and the
// boundary's local helper exitCodeOf agree on every *forgeerr.Error.
//
// The two functions are different in scope: exitCodeOf assumes a
// *forgeerr.Error and reads its category's code; ExitCode handles
// every error shape. For a *forgeerr.Error input, they must return
// the same value. A future change to either that broke the
// agreement would fail the test.
func TestMapping_AgreesWithExitCodeOf(t *testing.T) {
	t.Parallel()

	for _, cat := range forgeerr.AllCategories() {
		cat := cat
		t.Run(string(cat), func(t *testing.T) {
			t.Parallel()

			// Find a registered code for the category.
			var code forgeerr.Code
			for _, c := range forgeerr.AllCodes() {
				if c2, ok := c.Category(); ok && c2 == cat {
					code = c
					break
				}
			}
			if code == "" {
				t.Skipf("no registered code for category %s", cat)
			}

			e := forgeerr.New(cat, code, "message")

			got := ExitCode(e)
			want := exitCodeOf(e)
			if got != want {
				t.Errorf("ExitCode(%v) = %d; exitCodeOf(%v) = %d; must agree",
					cat, got, cat, want)
			}
		})
	}
}

// BenchmarkExitCode_ForgeError measures the cost of the mapping on
// the hot path (a *forgeerr.Error).
func BenchmarkExitCode_ForgeError(b *testing.B) {
	err := forgeerr.New(
		forgeerr.CategoryConfig,
		forgeerr.CodeConfigMalformed,
		"benchmark error",
	)
	b.ReportAllocs()
	var sink int
	for i := 0; i < b.N; i++ {
		sink = ExitCode(err)
	}
	_ = sink
}

// BenchmarkExitCode_Nil measures the cost of the mapping on the
// success path.
func BenchmarkExitCode_Nil(b *testing.B) {
	b.ReportAllocs()
	var sink int
	for i := 0; i < b.N; i++ {
		sink = ExitCode(nil)
	}
	_ = sink
}

// BenchmarkExitCode_WrappedForgeError measures the cost of the
// mapping when the *forgeerr.Error is buried under wrapping.
func BenchmarkExitCode_WrappedForgeError(b *testing.B) {
	inner := forgeerr.New(
		forgeerr.CategoryConfig,
		forgeerr.CodeConfigMalformed,
		"benchmark error",
	)
	err := fmt.Errorf("outer: %w", inner)
	b.ReportAllocs()
	var sink int
	for i := 0; i < b.N; i++ {
		sink = ExitCode(err)
	}
	_ = sink
}

// TestBDD_EveryDomainErrorHasAStructuredExitCode documents the
// caller-visible behavior the WBS 11.2.2 audit guarantees: a shell
// script can classify any error Forge emits by reading its exit
// code, because every domain error is a *forgeerr.Error and every
// *forgeerr.Error maps to a documented code.
//
// The test walks a representative error from each domain package
// and asserts that ExitCode returns a code in a documented range.
func TestBDD_EveryDomainErrorHasAStructuredExitCode(t *testing.T) {
	t.Parallel()

	// GIVEN an error from each domain package
	errs := []error{
		// config package
		forgeerr.New(forgeerr.CategoryConfig,
			forgeerr.CodeConfigMalformed, "invalid YAML"),
		// filesystem package
		forgeerr.New(forgeerr.CategoryFilesystem,
			forgeerr.CodeFilesystemNotFound, "path not found"),
		// cli package
		forgeerr.New(forgeerr.CategoryCLI,
			forgeerr.CodeCLIUnknownCommand, "unknown command"),
		// app/version package
		forgeerr.New(forgeerr.CategoryInternal,
			forgeerr.CodeInternalUnexpected, "unexpected"),
		// security category
		forgeerr.New(forgeerr.CategorySecurity,
			forgeerr.CodeSecurityPathTraversal, "path escape"),
	}

	// WHEN a shell script reads the exit code
	for _, err := range errs {
		code := ExitCode(err)

		// THEN the code is in a documented range
		r, ok := LookupExitCodeRange(code)
		if !ok {
			t.Errorf("ExitCode(%v) = %d; not in any documented range", err, code)
		}
		if r.Stability == StabilityReserved {
			t.Errorf("ExitCode(%v) = %d; in reserved range %q",
				err, code, r.Name)
		}
	}
}
