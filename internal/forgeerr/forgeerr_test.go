// Package forgeerr — tests for the canonical error type.
//
// This file covers the type itself: its unexported fields, its
// accessors, its Error() string, and its pure invariant helpers.
// It is deliberately independent of three adjacent concerns:
//
//   - The constructor API lives in constructors_test.go.
//   - The category taxonomy lives in category_test.go.
//   - The wrapping contract (Unwrap, Is, As and their interaction
//     with errors.Is, errors.As, fmt.Errorf, and errors.Join)
//     lives in wrap_test.go.
//
// A change to any of those three concerns should not require an
// edit here, and a change to the field layout, an accessor, or an
// invariant helper should not require an edit there.
//
// Every code used in a positive-construction assertion is a
// member of the registry in codes.go (WBS 10.3.1). The registry
// is the single source of truth for codes; the tests refer to the
// exported constants, not to string literals.
//
// Test categories:
//
//   - Unit:      accessors, Error() format
//   - Positive:  valid construction at boundaries
//   - Negative:  empty values, malformed inputs to the helpers
//   - Edge:      nil context vs empty context
//   - Boundary:  message-length edges
//   - Non-func:  concurrency (-race), allocation discipline
//   - Fuzz:      invariant-helper equivalence
package forgeerr

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// --- I8: type is defined with unexported fields -----------------------
//
// The WBS 10.1.1 invariant I8 ("Error is immutable after
// construction") is enforced by two mechanisms:
//
//  1. All fields are unexported, so external code cannot read or
//     write them directly.
//  2. Accessors that return maps return defensive copies.
//
// The first mechanism is enforced here, via reflection. A comment
// alone cannot enforce it: exporting a field is additive and
// would still compile. Reflection is the only reliable guard
// short of a build-tag-isolated AST check.
//
// The second mechanism is exercised in TestError_ContextIsCopy.

func TestError_FieldsAreUnexported(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeOf(Error{})

	wantFields := map[string]bool{
		"code":        true,
		"message":     true,
		"category":    true,
		"cause":       true,
		"context":     true,
		"remediation": true,
	}

	if typ.NumField() != len(wantFields) {
		t.Fatalf("Error has %d fields, want %d", typ.NumField(), len(wantFields))
	}

	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if f.PkgPath == "" {
			t.Errorf("field %q is exported; all fields must be unexported (I8)", f.Name)
		}
		if !wantFields[f.Name] {
			t.Errorf("unexpected field %q", f.Name)
		}
	}
}

// --- Accessors --------------------------------------------------------
//
// Each accessor is tested through the frozen constructor API. The
// tests here verify only that the accessor returns what was set;
// validation behaviour is the constructor's concern and is
// covered in constructors_test.go.

func TestError_Accessors(t *testing.T) {
	t.Parallel()

	e := New(CategoryConfig, string(CodeConfigMalformed), "invalid YAML").
		WithContext("file_path", "/etc/forge/config.yaml").
		WithRemediation("Fix the YAML syntax.")

	if got, want := e.Code(), string(CodeConfigMalformed); got != want {
		t.Errorf("Code() = %q, want %q", got, want)
	}
	if got, want := e.Message(), "invalid YAML"; got != want {
		t.Errorf("Message() = %q, want %q", got, want)
	}
	if got, want := e.Category(), CategoryConfig; got != want {
		t.Errorf("Category() = %v, want %v", got, want)
	}
	if got, want := e.Context()["file_path"], "/etc/forge/config.yaml"; got != want {
		t.Errorf("Context()[file_path] = %q, want %q", got, want)
	}
	if got, want := e.Remediation(), "Fix the YAML syntax."; got != want {
		t.Errorf("Remediation() = %q, want %q", got, want)
	}
}

// TestError_AccessorsEveryCategory asserts that the Category
// accessor round-trips every category that has a registered code.
// Categories without a registered code (VALIDATION and TEMPLATE in
// Phase 2) are skipped: a positive assertion is impossible for
// them until the registry grows.
//
// The code for each category is looked up from AllCodes(), not
// synthesised from the prefix. A synthetic code such as
// "FORGE_CLI_TEST" matches the shape and the prefix but is not
// registered, so the constructor rejects it.
func TestError_AccessorsEveryCategory(t *testing.T) {
	t.Parallel()

	for _, c := range AllCategories() {
		c := c
		t.Run(c.String(), func(t *testing.T) {
			t.Parallel()

			var code Code
			for _, k := range AllCodes() {
				if cat, ok := k.Category(); ok && cat == c {
					code = k
					break
				}
			}
			if code == "" {
				t.Skipf("no registered code for category %s", c)
			}

			e := New(c, string(code), "x")
			if got := e.Category(); got != c {
				t.Fatalf("Category() = %v, want %v", got, c)
			}
			if got := e.Code(); got != string(code) {
				t.Fatalf("Code() = %q, want %q", got, code)
			}
		})
	}
}

// TestError_NilContext asserts that a *Error constructed without
// context returns nil from Context(), not an empty map. Callers
// can therefore distinguish "no context" from "empty context".
func TestError_NilContext(t *testing.T) {
	t.Parallel()

	e := New(CategoryConfig, string(CodeConfigMalformed), "invalid YAML")
	if e.Context() != nil {
		t.Fatalf("Context() = %v, want nil", e.Context())
	}
}

// TestError_ContextIsCopy asserts that mutating the map returned
// by Context() does not affect the *Error. This is the second
// half of I8's enforcement (the first half is the unexported
// fields, asserted in TestError_FieldsAreUnexported).
func TestError_ContextIsCopy(t *testing.T) {
	t.Parallel()

	e := New(CategoryConfig, string(CodeConfigMalformed), "invalid YAML").
		WithContext("k", "v")

	returned := e.Context()
	returned["k"] = "mutated"

	if e.Context()["k"] != "v" {
		t.Fatalf("Context() returned an aliased map: %q", e.Context()["k"])
	}
}

// --- Error() format ---------------------------------------------------
//
// Error() is the programmer-facing representation. It is
// intentionally not the user-facing format; user-facing rendering
// is a separate concern (WBS 10.4.1).

func TestError_ErrorString(t *testing.T) {
	t.Parallel()

	e := New(CategoryConfig, string(CodeConfigMalformed), "invalid YAML in config.yaml")
	want := "FORGE_CONFIG_MALFORMED: invalid YAML in config.yaml"
	if got := e.Error(); got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}

// TestError_ErrorStringIsStable asserts that repeated calls to
// Error() return the same string. This is trivially true for an
// immutable value, but the assertion guards against a future
// refactor that introduces lazy formatting with side effects.
func TestError_ErrorStringIsStable(t *testing.T) {
	t.Parallel()

	e := New(CategoryConfig, string(CodeConfigMalformed), "invalid YAML")
	first := e.Error()
	for i := 0; i < 100; i++ {
		if got := e.Error(); got != first {
			t.Fatalf("Error() changed on call %d: %q -> %q", i, first, got)
		}
	}
}

// TestError_ErrorStringUsesCodeAndMessage asserts that Error()
// renders exactly "<code>: <message>" and nothing else: no
// category, no context, no remediation. Those are rendered by the
// user-facing formatter (WBS 10.4.1), not by the programmer-facing
// Error() method.
//
// The assertion is exact equality, not a substring search. A
// substring search for the category name would always match,
// because the code embeds the category as its prefix
// (FORGE_CONFIG_ contains CONFIG). Exact equality is the only
// form that distinguishes "the category appears as part of the
// code" from "the category appears as a separate field".
func TestError_ErrorStringUsesCodeAndMessage(t *testing.T) {
	t.Parallel()

	e := New(CategoryConfig, string(CodeConfigMalformed), "invalid YAML").
		WithContext("file_path", "/etc/forge/config.yaml").
		WithRemediation("Fix the YAML syntax.")

	got := e.Error()
	want := "FORGE_CONFIG_MALFORMED: invalid YAML"
	if got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}

	// Belt-and-braces: if a future change adds a suffix or a
	// prefix that happens to leave the above check passing, these
	// assertions catch it. They are the specific "does not
	// contain" checks, applied to values that would be visible if
	// the formatter rendered them.
	if strings.Contains(got, "/etc/forge/config.yaml") {
		t.Fatalf("Error() = %q contains context; it must contain only code and message", got)
	}
	if strings.Contains(got, "Fix the YAML syntax") {
		t.Fatalf("Error() = %q contains remediation; it must contain only code and message", got)
	}
}

// --- Invariant helpers (white-box) -----------------------------------
//
// validateMessage and validateRemediation are unexported, so these
// tests live in the same package. They are white-box tests: they
// pin the ordering contract of each helper independently of the
// constructors that call them. In particular, they verify that
// when an input violates two rules at once, the *safety* rule
// (single line) fires before the *cosmetic* rule (trailing
// period). A regression in the ordering would otherwise be masked
// by the constructor-level tests, which only check that *some*
// panic occurred.
//
// These helpers do not consult the code registry, so the tests
// below use arbitrary strings — no registry code is required.

func TestValidateMessage(t *testing.T) {
	t.Parallel()

	valid := []string{
		"x",
		strings.Repeat("a", 80),
		"contains spaces",
		"contains-hyphens",
		"contains_underscores",
		"contains 123 digits",
		"contains punctuation! but not a trailing period",
	}
	for _, msg := range valid {
		msg := msg
		t.Run("valid/"+msg, func(t *testing.T) {
			t.Parallel()
			// Must not panic.
			validateMessage(msg)
		})
	}

	invalid := []struct {
		name string
		msg  string
		want string
	}{
		{"empty", "", "must not be empty"},
		{"81 bytes", strings.Repeat("a", 81), "exceeds 80"},
		{"trailing period", "bad.", "must not end with a period"},
		{"newline", "bad\nline", "single line"},
		{"CR", "bad\rline", "single line"},
		// Ordering: the single-line check runs before the trailing-
		// period check, so an input that violates both fires on the
		// safety-relevant rule.
		{"newline then period", "bad.\n", "single line"},
		{"period then newline", "bad\n.", "single line"},
	}
	for _, tc := range invalid {
		tc := tc
		t.Run("invalid/"+tc.name, func(t *testing.T) {
			t.Parallel()
			defer func() {
				r := recover()
				if r == nil {
					t.Fatalf("expected panic containing %q, got none", tc.want)
				}
				if !strings.Contains(fmt.Sprint(r), tc.want) {
					t.Fatalf("panic %q does not contain %q", r, tc.want)
				}
			}()
			validateMessage(tc.msg)
		})
	}
}

func TestValidateRemediation(t *testing.T) {
	t.Parallel()

	// The empty remediation is explicitly permitted.
	t.Run("valid/empty", func(t *testing.T) {
		t.Parallel()
		validateRemediation("")
	})

	valid := []string{
		"Fix it.",
		"Check the YAML syntax at line 12.",
		"See https://example.com/docs for details.",
	}
	for _, r := range valid {
		r := r
		t.Run("valid/"+r, func(t *testing.T) {
			t.Parallel()
			validateRemediation(r)
		})
	}

	invalid := []struct {
		name string
		rem  string
		want string
	}{
		{"no trailing period", "fix it", "must end with a period"},
		// Ordering: the single-line check runs before the trailing-
		// period check.
		{"newline then period", "fix\n.", "single line"},
		{"period then newline", "fix.\n", "single line"},
		{"CR then period", "fix\r.", "single line"},
	}
	for _, tc := range invalid {
		tc := tc
		t.Run("invalid/"+tc.name, func(t *testing.T) {
			t.Parallel()
			defer func() {
				r := recover()
				if r == nil {
					t.Fatalf("expected panic containing %q, got none", tc.want)
				}
				if !strings.Contains(fmt.Sprint(r), tc.want) {
					t.Fatalf("panic %q does not contain %q", r, tc.want)
				}
			}()
			validateRemediation(tc.rem)
		})
	}
}

// TestValidateCategory asserts the closed-set boundary through the
// helper that constructors actually call. The helper delegates to
// Category.valid(); the panic message must mention the closed set
// and must format the category as a quoted string (WBS 10.2.1
// changed Category from uint8 to string).
//
// validateCategory does not consult the code registry, so the
// category values below are the only thing under test.
func TestValidateCategory(t *testing.T) {
	t.Parallel()

	for _, c := range AllCategories() {
		c := c
		t.Run("valid/"+c.String(), func(t *testing.T) {
			t.Parallel()
			validateCategory(c) // must not panic
		})
	}

	invalid := []Category{
		"",              // zero value
		"UNKNOWN",       // not a member
		"OTHER",         // explicitly forbidden by WBS 10.2.1
		"cli",           // wrong case
		"CONFIGX",       // typo
		"FORGE_CONFIG_", // a prefix, not a category
	}
	for _, c := range invalid {
		c := c
		t.Run("invalid/"+fmt.Sprintf("%q", c), func(t *testing.T) {
			t.Parallel()
			defer func() {
				r := recover()
				if r == nil {
					t.Fatalf("validateCategory(%q) did not panic", c)
				}
				s := fmt.Sprint(r)
				if !strings.Contains(s, "closed set") {
					t.Fatalf("panic %q does not mention the closed set", s)
				}
				if !strings.Contains(s, fmt.Sprintf("%q", string(c))) {
					t.Fatalf("panic %q does not quote the category", s)
				}
			}()
			validateCategory(c)
		})
	}
}

// --- Non-functional: concurrency -------------------------------------
//
// *Error is immutable (I8), so concurrent reads must be safe. The
// test spawns readers and a single writer that mutates the map
// returned by Context(); under -race, a regression in which
// Context() aliased the internal map would be caught here.
//
// The chain methods (Unwrap, Is, As) are exercised concurrently in
// wrap_test.go's TestConcurrentChainMethods. This test focuses on
// the accessors.

func TestError_ConcurrentReads(t *testing.T) {
	e := New(CategoryConfig, string(CodeConfigMalformed), "invalid YAML").
		WithContext("k", "v").
		WithCause(errors.New("cause")).
		WithRemediation("Fix it.")

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = e.Error()
			_ = e.Code()
			_ = e.Message()
			_ = e.Category()
			_ = e.Remediation()
			_ = e.Context()
		}()
	}
	wg.Wait()

	// Post-condition: the internal context is intact.
	if e.Context()["k"] != "v" {
		t.Fatalf("context was mutated by concurrent readers: %v", e.Context())
	}
}

// TestError_ConcurrentContextMutation asserts that mutating the
// map returned by Context() from many goroutines does not race
// with readers of the *Error itself. Because Context() returns a
// fresh copy on every call, each goroutine operates on its own map
// and the internal state is never touched.
func TestError_ConcurrentContextMutation(t *testing.T) {
	e := New(CategoryConfig, string(CodeConfigMalformed), "invalid YAML").
		WithContext("k", "v")

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			m := e.Context()
			m["k"] = fmt.Sprintf("mutated-%d", i)
		}(i)
	}
	wg.Wait()

	if e.Context()["k"] != "v" {
		t.Fatalf("context was mutated: %v", e.Context())
	}
}

// --- Non-functional: allocation discipline (informational) -----------

// BenchmarkError measures the cost of Error(), which is a single
// string concatenation and should allocate once.
func BenchmarkError(b *testing.B) {
	e := New(CategoryConfig, string(CodeConfigMalformed), "invalid YAML")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = e.Error()
	}
}

// BenchmarkContext measures the cost of the defensive copy that
// Context() performs on every call. The cost is O(len(context)),
// which is expected to be small (typically 1–3 keys).
func BenchmarkContext(b *testing.B) {
	e := New(CategoryConfig, string(CodeConfigMalformed), "invalid YAML").
		WithContext("file_path", "/etc/forge/config.yaml").
		WithContext("line_number", "12")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = e.Context()
	}
}

// --- Fuzz: invariant-helper equivalence ------------------------------
//
// These fuzzers target the pure helpers directly. They are
// complementary to the constructor-level fuzzers in
// constructors_test.go: those verify the *combination* of rules
// enforced by a constructor, while these verify each rule in
// isolation.
//
// The wrapping contract (Unwrap, Is, As) is not fuzzable in the
// same way: it is a set of structural operations, not a predicate
// on a string. Its coverage comes from the targeted tests in
// wrap_test.go.

// FuzzValidateMessage asserts that validateMessage accepts exactly
// the messages that satisfy all four rules: non-empty, <= 80
// bytes, no trailing period, no CR/LF. The predicate below is the
// hand-written mirror of the rule; the fuzzer's job is to detect
// divergence between the predicate and the helper.
func FuzzValidateMessage(f *testing.F) {
	seeds := []string{
		"x",
		"",
		"bad.",
		"bad\n",
		"bad\r",
		"bad.\n",
		"bad\n.",
		strings.Repeat("a", 80),
		strings.Repeat("a", 81),
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, msg string) {
		wantValid := msg != "" &&
			len(msg) <= 80 &&
			!strings.HasSuffix(msg, ".") &&
			!strings.ContainsAny(msg, "\n\r")

		defer func() {
			r := recover()
			switch {
			case wantValid && r != nil:
				t.Fatalf("valid message %q panicked: %v", msg, r)
			case !wantValid && r == nil:
				t.Fatalf("invalid message %q did not panic", msg)
			}
		}()
		validateMessage(msg)
	})
}

// FuzzValidateRemediation asserts that validateRemediation accepts
// exactly the remediations that satisfy: empty, or (single line
// and ends with a period).
func FuzzValidateRemediation(f *testing.F) {
	seeds := []string{
		"",
		"Fix it.",
		"fix it",
		"fix\n.",
		"fix.\n",
		"fix\r.",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, r string) {
		wantValid := r == "" ||
			(!strings.ContainsAny(r, "\n\r") && strings.HasSuffix(r, "."))

		defer func() {
			rec := recover()
			switch {
			case wantValid && rec != nil:
				t.Fatalf("valid remediation %q panicked: %v", r, rec)
			case !wantValid && rec == nil:
				t.Fatalf("invalid remediation %q did not panic", r)
			}
		}()
		validateRemediation(r)
	})
}
