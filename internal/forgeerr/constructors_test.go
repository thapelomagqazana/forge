// Package forgeerr — constructor API tests.
//
// This file carries the full test suite for the frozen constructor
// surface defined in constructors.go (WBS 10.1.2), updated for the
// category taxonomy in WBS 10.2.1 and for the code registry in
// WBS 10.3.1.
//
// Every code used in a *positive* construction is a member of the
// registry in codes.go. The helper realCodeFor returns a real code
// for a given category so that table-driven tests can be written
// without repeating the mapping.
//
// Organisation:
//
//   - Every test is named after the acceptance criterion (AC1–AC9)
//     it verifies, so `go test -run 'TestAC'` selects the full
//     traceability set.
//   - Non-AC tests (BDD scenarios, concurrency, fuzz) are named by
//     their category and are cross-referenced in the AC9 coverage
//     comment.
//   - Test categories: TDD (unit), ATDD (AC-mapped), BDD
//     (scenario), POS/NEG (positive/negative), EDGE, CORN (corner),
//     BND (boundary), NF (non-functional).
package forgeerr

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
)

// --- Test helpers -----------------------------------------------------

// mustNew constructs a *Error with the given category, code, and
// message. It fails the test immediately if construction panics, so
// callers do not need to wrap every call in a recover.
func mustNew(tb testing.TB, category Category, code, message string) *Error {
	tb.Helper()
	return New(category, code, message)
}

// expectPanic runs f and asserts that it panics with a message
// containing want. It is the shared oracle for every negative test
// in this file, so the pattern of "defer recover, check substring"
// lives in exactly one place.
func expectPanic(t *testing.T, want string, f func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected panic containing %q, got none", want)
		}
		if !strings.Contains(fmt.Sprint(r), want) {
			t.Fatalf("panic %q does not contain %q", r, want)
		}
	}()
	f()
}

// realCodeFor returns a registered code for the given category.
//
// Phase 2 has at least one code per category except VALIDATION and
// TEMPLATE, which have none. realCodeFor fails the test for those
// two categories: a test that asks for "a real code in VALIDATION"
// is asking for something that does not exist, and the test should
// be rewritten rather than silently using an unregistered string.
//
// The helper exists so that table-driven tests can iterate over
// categories without hard-coding the (category, code) mapping in
// every row.
func realCodeFor(t *testing.T, cat Category) Code {
	t.Helper()
	switch cat {
	case CategoryCLI:
		return CodeCLIUnknownFlag
	case CategoryConfig:
		return CodeConfigMalformed
	case CategoryFilesystem:
		return CodeFilesystemNotFound
	case CategorySecurity:
		return CodeSecurityPathTraversal
	case CategoryInternal:
		return CodeInternalPanic
	default:
		t.Fatalf("no registered code for category %s", cat)
		return ""
	}
}

// --- AC1: New exists with frozen signature ----------------------------

// TestAC1_NewSignature is a compile-time assertion that New has the
// frozen signature. If a future refactor changes the parameters,
// this test will fail to compile — which is the intended signal.
func TestAC1_NewSignature(t *testing.T) {
	t.Parallel()

	var _ func(Category, string, string) *Error = New

	e := New(CategoryConfig, string(CodeConfigMalformed), "invalid YAML")
	if e == nil {
		t.Fatal("New returned nil")
	}
	if e.Code() != string(CodeConfigMalformed) {
		t.Fatalf("Code() = %q", e.Code())
	}
}

// --- AC2: Wrap exists with frozen signature ---------------------------

func TestAC2_WrapSignature(t *testing.T) {
	t.Parallel()

	var _ func(Category, string, string, error) *Error = Wrap

	e := Wrap(CategoryFilesystem, string(CodeFilesystemNotFound), "not found", errors.New("underlying"))
	if e == nil {
		t.Fatal("Wrap returned nil")
	}
	if e.Code() != string(CodeFilesystemNotFound) {
		t.Fatalf("Code() = %q", e.Code())
	}
}

// --- AC3: WithCause, WithContext, WithRemediation exist ---------------

func TestAC3_WithMethodsSignature(t *testing.T) {
	t.Parallel()

	var _ func(*Error, error) *Error = (*Error).WithCause
	var _ func(*Error, string, string) *Error = (*Error).WithContext
	var _ func(*Error, string) *Error = (*Error).WithRemediation

	e := New(CategoryConfig, string(CodeConfigMalformed), "invalid YAML")
	if e.WithCause(errors.New("c")) == nil {
		t.Fatal("WithCause returned nil")
	}
	if e.WithContext("k", "v") == nil {
		t.Fatal("WithContext returned nil")
	}
	if e.WithRemediation("Fix it.") == nil {
		t.Fatal("WithRemediation returned nil")
	}
}

// --- AC4: Wrap(nil, ...) panics ---------------------------------------

// TestAC4_WrapNilCausePanics covers the negative case: wrapping nil
// is a programming error, because the caller almost certainly meant
// to call New.
func TestAC4_WrapNilCausePanics(t *testing.T) {
	t.Parallel()

	expectPanic(t, "nil cause", func() {
		_ = Wrap(CategoryFilesystem, string(CodeFilesystemNotFound), "not found", nil)
	})
}

// TestAC4_WithCauseNilPanics covers the same rule for the fluent
// API. WithCause(nil) is meaningless: a caller who has nothing to
// attach should not call the method at all.
func TestAC4_WithCauseNilPanics(t *testing.T) {
	t.Parallel()

	e := New(CategoryConfig, string(CodeConfigMalformed), "invalid YAML")
	expectPanic(t, "nil cause", func() {
		_ = e.WithCause(nil)
	})
}

// --- AC5: Wrap(<*Error>, ...) returns unchanged -----------------------

// TestAC5_WrapExistingErrorReturnsUnchanged asserts the no-double-
// wrap rule. The returned *Error must be the *same pointer* as the
// input, not a copy, so that errors.Is comparisons and formatter
// output see exactly one Code per chain.
func TestAC5_WrapExistingErrorReturnsUnchanged(t *testing.T) {
	t.Parallel()

	inner := New(CategoryConfig, string(CodeConfigMalformed), "inner")
	got := Wrap(CategoryFilesystem, string(CodeFilesystemNotFound), "outer", inner)

	if got != inner {
		t.Fatalf("Wrap returned a different *Error: got %p, want %p", got, inner)
	}
	if got.Code() != string(CodeConfigMalformed) {
		t.Fatalf("Code() = %q, want inner code preserved", got.Code())
	}
	if got.Category() != CategoryConfig {
		t.Fatalf("Category() = %v, want CategoryConfig preserved", got.Category())
	}
}

// TestAC5_WrapExistingErrorThroughFmtError asserts that the no-
// double-wrap rule still applies when the *Error is buried inside a
// fmt.Errorf %w chain. errors.As is used to unwrap, so this
// exercises the interaction between the standard library and the
// custom As method.
func TestAC5_WrapExistingErrorThroughFmtError(t *testing.T) {
	t.Parallel()

	inner := New(CategoryConfig, string(CodeConfigMalformed), "inner")
	wrapped := fmt.Errorf("context: %w", inner)

	got := Wrap(CategoryFilesystem, string(CodeFilesystemNotFound), "outer", wrapped)
	if got != inner {
		t.Fatalf("Wrap did not find inner *Error through fmt.Errorf wrap")
	}
}

// --- AC6: WithX methods return new instances, originals unchanged -----

// TestAC6_WithContextImmutability asserts the core immutability
// guarantee for WithContext: the receiver is not mutated, and the
// returned *Error is a distinct pointer carrying both the old and
// new keys.
func TestAC6_WithContextImmutability(t *testing.T) {
	t.Parallel()

	orig := New(CategoryConfig, string(CodeConfigMalformed), "invalid YAML").
		WithContext("a", "1")

	derived := orig.WithContext("b", "2")

	if _, ok := orig.Context()["b"]; ok {
		t.Fatalf("original gained key b: %v", orig.Context())
	}
	if orig.Context()["a"] != "1" {
		t.Fatalf("original lost key a: %v", orig.Context())
	}
	if derived.Context()["a"] != "1" || derived.Context()["b"] != "2" {
		t.Fatalf("derived context = %v", derived.Context())
	}
	if derived == orig {
		t.Fatal("WithContext returned the same pointer")
	}
}

// TestAC6_WithContextReplacesExistingKey asserts that setting an
// existing key replaces the value in the copy without mutating the
// original.
func TestAC6_WithContextReplacesExistingKey(t *testing.T) {
	t.Parallel()

	orig := New(CategoryConfig, string(CodeConfigMalformed), "invalid YAML").
		WithContext("k", "old")
	derived := orig.WithContext("k", "new")

	if orig.Context()["k"] != "old" {
		t.Fatalf("original mutated: %q", orig.Context()["k"])
	}
	if derived.Context()["k"] != "new" {
		t.Fatalf("derived = %q", derived.Context()["k"])
	}
}

// TestAC6_WithContextNilVsEmpty asserts the nil-versus-empty
// distinction: a *Error constructed without context returns nil
// from Context(), and a *Error built with WithContext returns a
// non-nil map. Callers can therefore distinguish "no context" from
// "empty context".
func TestAC6_WithContextNilVsEmpty(t *testing.T) {
	t.Parallel()

	bare := New(CategoryConfig, string(CodeConfigMalformed), "invalid YAML")
	if bare.Context() != nil {
		t.Fatalf("bare Context() = %v, want nil", bare.Context())
	}

	withCtx := bare.WithContext("k", "")
	if withCtx.Context() == nil {
		t.Fatal("Context() = nil after WithContext")
	}
	if v, ok := withCtx.Context()["k"]; !ok || v != "" {
		t.Fatalf("Context()[k] = %q, ok = %v", v, ok)
	}
}

// TestAC6_WithCauseImmutability asserts WithCause does not mutate
// the receiver and that the cause chain is reachable from the copy.
func TestAC6_WithCauseImmutability(t *testing.T) {
	t.Parallel()

	root := errors.New("root")
	orig := New(CategoryFilesystem, string(CodeFilesystemNotFound), "not found")
	derived := orig.WithCause(root)

	if orig.Unwrap() != nil {
		t.Fatal("original gained a cause")
	}
	if !errors.Is(derived, root) {
		t.Fatal("derived does not wrap root")
	}
	if derived == orig {
		t.Fatal("WithCause returned the same pointer")
	}
}

// TestAC6_WithRemediationImmutability asserts WithRemediation does
// not mutate the receiver.
func TestAC6_WithRemediationImmutability(t *testing.T) {
	t.Parallel()

	orig := New(CategoryConfig, string(CodeConfigMalformed), "invalid YAML")
	derived := orig.WithRemediation("Fix the YAML syntax.")

	if orig.Remediation() != "" {
		t.Fatalf("original gained remediation: %q", orig.Remediation())
	}
	if derived.Remediation() != "Fix the YAML syntax." {
		t.Fatalf("derived = %q", derived.Remediation())
	}
	if derived == orig {
		t.Fatal("WithRemediation returned the same pointer")
	}
}

// TestAC6_ChainedBuildersPreserveBase asserts that chaining the
// three WithX methods preserves the base error's Code, Category,
// and Message, and does not mutate the base at any step.
func TestAC6_ChainedBuildersPreserveBase(t *testing.T) {
	t.Parallel()

	base := New(CategoryConfig, string(CodeConfigMalformed), "invalid YAML")
	built := base.
		WithCause(errors.New("yaml: line 12")).
		WithContext("file_path", "/etc/forge/config.yaml").
		WithContext("line_number", "12").
		WithRemediation("Check the YAML syntax at line 12.")

	if base.Unwrap() != nil || base.Context() != nil || base.Remediation() != "" {
		t.Fatalf("base was mutated: %+v", base)
	}
	if built.Code() != base.Code() {
		t.Fatalf("Code() changed: %q -> %q", base.Code(), built.Code())
	}
	if built.Category() != base.Category() {
		t.Fatalf("Category() changed: %v -> %v", base.Category(), built.Category())
	}
	if built.Message() != base.Message() {
		t.Fatalf("Message() changed: %q -> %q", base.Message(), built.Message())
	}
	if len(built.Context()) != 2 {
		t.Fatalf("Context() = %v", built.Context())
	}
	if built.Remediation() == "" {
		t.Fatal("Remediation() empty")
	}
}

// --- AC7: Code prefix derived from category ---------------------------

// TestAC7_CodePrefixDerivation is the positive/negative table for
// the prefix rule.
//
// Positive rows require a real registry code. Phase 2 has no codes
// in VALIDATION or TEMPLATE, so those two categories are absent
// from the positive table. When codes are added to those
// categories in a later phase, the rows should be added back.
//
// Negative rows require only a valid *category*; the code passed
// is deliberately from a different category, so the prefix check
// fires before the registry check. Each negative row asserts the
// prefix diagnostic specifically.
func TestAC7_CodePrefixDerivation(t *testing.T) {
	t.Parallel()

	t.Run("positive", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			name string
			cat  Category
			code Code
		}{
			{"cli", CategoryCLI, CodeCLIUnknownFlag},
			{"config", CategoryConfig, CodeConfigMalformed},
			{"filesystem", CategoryFilesystem, CodeFilesystemNotFound},
			{"security", CategorySecurity, CodeSecurityPathTraversal},
			{"internal", CategoryInternal, CodeInternalPanic},
		}
		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				e := New(tc.cat, string(tc.code), "x")
				if e.Code() != string(tc.code) {
					t.Fatalf("Code() = %q, want %q", e.Code(), tc.code)
				}
				if e.Category() != tc.cat {
					t.Fatalf("Category() = %v, want %v", e.Category(), tc.cat)
				}
			})
		}
	})

	t.Run("negative", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			name string
			cat  Category
			code Code
		}{
			{"config code on filesystem category", CategoryFilesystem, CodeConfigMalformed},
			{"filesystem code on config category", CategoryConfig, CodeFilesystemNotFound},
			{"cli code on internal category", CategoryInternal, CodeCLIUnknownFlag},
			{"security code on config category", CategoryConfig, CodeSecurityPathTraversal},
			{"internal code on cli category", CategoryCLI, CodeInternalPanic},
		}
		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				expectPanic(t, "does not start with", func() {
					_ = New(tc.cat, string(tc.code), "x")
				})
			})
		}
	})
}

// TestAC7_UnregisteredCodePanics asserts that a code which matches
// the regex and carries the correct prefix is rejected when it is
// not a member of the registry (WBS 10.3.1). This is distinct from
// the prefix check above: the code is well-formed and correctly
// categorised, but the specific string is not registered.
func TestAC7_UnregisteredCodePanics(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		cat  Category
		code string
	}{
		{"config placeholder", CategoryConfig, "FORGE_CONFIG_X"},
		{"config typo", CategoryConfig, "FORGE_CONFIG_MALFORM"},
		{"filesystem placeholder", CategoryFilesystem, "FORGE_FILESYSTEM_READ"},
		{"security placeholder", CategorySecurity, "FORGE_SECURITY_TRAVERSAL"},
		{"cli placeholder", CategoryCLI, "FORGE_CLI_X"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			expectPanic(t, "not in the registry", func() {
				_ = New(tc.cat, tc.code, "x")
			})
		})
	}
}

// TestAC7_WrapValidatesPrefix asserts that Wrap applies the same
// prefix rule as New.
func TestAC7_WrapValidatesPrefix(t *testing.T) {
	t.Parallel()

	expectPanic(t, "does not start with", func() {
		_ = Wrap(CategoryFilesystem, string(CodeConfigMalformed), "x", errors.New("c"))
	})
}

// TestAC7_WrapValidatesRegistry asserts that Wrap applies the same
// registry rule as New.
func TestAC7_WrapValidatesRegistry(t *testing.T) {
	t.Parallel()

	// FORGE_FILESYSTEM_X matches the regex and carries the correct
	// prefix, but is not registered.
	expectPanic(t, "not in the registry", func() {
		_ = Wrap(CategoryFilesystem, "FORGE_FILESYSTEM_X", "x", errors.New("c"))
	})
}

// TestAC7_PrefixMatchesTaxonomy asserts that every category that
// has at least one registered code can be paired with that code
// through the constructor. Categories without any registered code
// are skipped: a positive test is impossible for them until the
// registry grows.
func TestAC7_PrefixMatchesTaxonomy(t *testing.T) {
	t.Parallel()

	for _, c := range AllCategories() {
		c := c
		t.Run(c.String(), func(t *testing.T) {
			t.Parallel()

			// Find a registered code in this category, if any.
			var found Code
			for _, code := range AllCodes() {
				if cat, ok := code.Category(); ok && cat == c {
					found = code
					break
				}
			}
			if found == "" {
				t.Skipf("no registered code for category %s", c)
			}

			if !strings.HasPrefix(string(found), c.Prefix()) {
				t.Fatalf("code %q does not start with prefix %q", found, c.Prefix())
			}
			e := New(c, string(found), "x")
			if e.Code() != string(found) {
				t.Fatalf("Code() = %q, want %q", e.Code(), found)
			}
			if e.Category() != c {
				t.Fatalf("Category() = %v, want %v", e.Category(), c)
			}
		})
	}
}

// --- AC8: No public way to construct without category -----------------

// TestAC8_NoCategorylessConstructor is a compile-time assertion:
// the two frozen constructors both take a Category as their first
// parameter. If a future refactor adds a categoryless constructor,
// this test will not catch it directly — but any refactor that
// *removes* the category parameter from New or Wrap will fail to
// compile here.
func TestAC8_NoCategorylessConstructor(t *testing.T) {
	t.Parallel()

	var _ func(Category, string, string) *Error = New
	var _ func(Category, string, string, error) *Error = Wrap
}

// TestAC8_InvalidCategoryPanics asserts that the zero value, a
// string that looks like a category but is not a member, and any
// non-member are rejected before the prefix check runs. The panic
// message must reference the closed set, not the prefix, because
// the category is the more specific failure.
func TestAC8_InvalidCategoryPanics(t *testing.T) {
	t.Parallel()

	invalid := []Category{
		"", // zero value
		"UNKNOWN",
		"OTHER", // explicitly forbidden by WBS 10.2.1
		"cli",   // wrong case
		"CONFIGX",
		"FORGE_CONFIG_", // a prefix, not a category
	}
	for _, c := range invalid {
		c := c
		t.Run(fmt.Sprintf("%q", c), func(t *testing.T) {
			t.Parallel()
			expectPanic(t, "closed set", func() {
				_ = New(c, string(CodeConfigMalformed), "x")
			})
		})
	}
}

// --- AC9: Every constructor has tests ---------------------------------

// TestAC9_EveryConstructorHasTests is a documentation test: it
// makes the coverage requirement visible in CI output. The list
// below is the authoritative map from constructor to test name.
// Reviewers should reject a PR that removes a constructor's tests
// without updating this list.
//
// The test itself does no work; its value is the comment.
func TestAC9_EveryConstructorHasTests(t *testing.T) {
	t.Parallel()

	// Constructor         → Tests
	// --------------------|--------------------------------------------
	// New                 → AC1_NewSignature, AC7_CodePrefixDerivation,
	//                       AC7_UnregisteredCodePanics,
	//                       AC7_PrefixMatchesTaxonomy,
	//                       AC8_InvalidCategoryPanics,
	//                       New_InvariantPanics, New_MessageLengthBoundary
	// Wrap                → AC2_WrapSignature, AC4_WrapNilCausePanics,
	//                       AC5_WrapExistingErrorReturnsUnchanged,
	//                       AC5_WrapExistingErrorThroughFmtError,
	//                       AC7_WrapValidatesPrefix,
	//                       AC7_WrapValidatesRegistry
	// WithCause           → AC3_WithMethodsSignature, AC4_WithCauseNilPanics,
	//                       AC6_WithCauseImmutability
	// WithContext         → AC3_WithMethodsSignature, AC6_WithContextImmutability,
	//                       AC6_WithContextReplacesExistingKey,
	//                       AC6_WithContextNilVsEmpty,
	//                       WithContext_KeyPanics, WithContext_KeyAccepted
	// WithRemediation     → AC3_WithMethodsSignature, AC6_WithRemediationImmutability,
	//                       WithRemediation_Panics
	//
	// BDD and non-functional coverage:
	//
	//   BDD_ConfigLoadFailure, BDD_ChainPreservesBothCodes,
	//   TestConcurrentBuilders, BenchmarkNew, BenchmarkWithContext,
	//   FuzzCodeFormat, FuzzMessage, FuzzContextKey
	t.Log("constructor coverage is documented in this test's comment")
}

// --- Constructor invariant panic coverage -----------------------------
//
// These tests exercise the invariants that the constructors inherit
// from WBS 10.1.1 and extend with the WBS 10.3.1 registry check.
// They are negative/edge/boundary cases and are organised so that
// each vector violates exactly one invariant, making the panic
// message an unambiguous oracle.

// TestNew_InvariantPanics covers the constructor's invariant checks
// for New. Each row violates exactly one rule.
func TestNew_InvariantPanics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cat  Category
		code string
		msg  string
		want string
	}{
		// I1 / I3 — code-shape rules.
		{"empty code", CategoryConfig, "", "x", "code must not be empty"},
		{"code lowercase", CategoryConfig, "forge_config_malformed", "x", "does not match"},
		{"code missing prefix", CategoryConfig, "CONFIG_MALFORMED", "x", "does not match"},
		{"code leading digit", CategoryConfig, "FORGE_1X", "x", "does not match"},
		{"code leading underscore", CategoryConfig, "FORGE__X", "x", "does not match"},
		{"code lowercase in tail", CategoryConfig, "FORGE_Config", "x", "does not match"},
		{"code empty tail", CategoryConfig, "FORGE_", "x", "does not match"},
		// WBS 10.3.1 — registry membership. These codes are
		// well-formed and carry the config prefix, but are not
		// registered.
		{"code not in registry", CategoryConfig, "FORGE_CONFIG_X", "x", "not in the registry"},
		{"code typo", CategoryConfig, "FORGE_CONFIG_MALFORM", "x", "not in the registry"},
		// I2 / I4 / I5 — message rules. These rows use a registered
		// code so that the failure is unambiguously the message.
		{"empty message", CategoryConfig, string(CodeConfigMalformed), "", "message must not be empty"},
		{"message 81 bytes", CategoryConfig, string(CodeConfigMalformed), strings.Repeat("a", 81), "exceeds 80"},
		{"message ends with period", CategoryConfig, string(CodeConfigMalformed), "bad.", "must not end with a period"},
		{"message contains newline", CategoryConfig, string(CodeConfigMalformed), "bad\nline", "single line"},
		{"message contains CR", CategoryConfig, string(CodeConfigMalformed), "bad\rline", "single line"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			expectPanic(t, tc.want, func() {
				_ = New(tc.cat, tc.code, tc.msg)
			})
		})
	}
}

// TestWithContext_KeyPanics covers the snake_case convention for
// context keys. Each row violates exactly one rule.
func TestWithContext_KeyPanics(t *testing.T) {
	t.Parallel()

	base := New(CategoryConfig, string(CodeConfigMalformed), "invalid YAML")

	tests := []struct {
		name string
		key  string
		want string
	}{
		{"empty key", "", "must not be empty"},
		{"uppercase", "FilePath", "only [a-z0-9_]"},
		{"hyphen", "file-path", "only [a-z0-9_]"},
		{"dot", "file.path", "only [a-z0-9_]"},
		{"leading digit", "1file", "must start with a lowercase letter"},
		{"leading underscore", "_file", "must start with a lowercase letter"},
		{"space", "file path", "only [a-z0-9_]"},
		{"colon", "file:path", "only [a-z0-9_]"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			expectPanic(t, tc.want, func() {
				_ = base.WithContext(tc.key, "v")
			})
		})
	}
}

// TestWithContext_KeyAccepted covers the positive side of the
// snake_case convention. Keys that satisfy the rule must be
// accepted, including keys composed entirely of underscores after
// the first character.
func TestWithContext_KeyAccepted(t *testing.T) {
	t.Parallel()

	base := New(CategoryConfig, string(CodeConfigMalformed), "invalid YAML")

	for _, key := range []string{
		"a",
		"file_path",
		"line_number",
		"a_b_c",
		"a1_b2",
		"a__b", // consecutive underscores are permitted after the first char
	} {
		key := key
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			e := base.WithContext(key, "v")
			if e.Context()[key] != "v" {
				t.Fatalf("Context()[%q] = %q", key, e.Context()[key])
			}
		})
	}
}

// TestWithRemediation_Panics covers the remediation rules. As with
// the other negative tables, each vector violates exactly one rule.
func TestWithRemediation_Panics(t *testing.T) {
	t.Parallel()

	base := New(CategoryConfig, string(CodeConfigMalformed), "invalid YAML")

	tests := []struct {
		name string
		rem  string
		want string
	}{
		{"no trailing period", "fix it", "must end with a period"},
		{"newline", "fix\n.", "single line"},
		{"CR", "fix\r.", "single line"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			expectPanic(t, tc.want, func() {
				_ = base.WithRemediation(tc.rem)
			})
		})
	}

	// Positive: the empty remediation is explicitly permitted, and
	// it clears any previously-set remediation.
	t.Run("empty clears", func(t *testing.T) {
		t.Parallel()
		e := base.WithRemediation("Fix it.").WithRemediation("")
		if e.Remediation() != "" {
			t.Fatalf("Remediation() = %q, want empty", e.Remediation())
		}
	})
}

// --- Boundary: message length -----------------------------------------

// TestNew_MessageLengthBoundary pins the exact accepted boundary of
// the message length rule. The limit is measured in bytes (len),
// not runes; this test documents that choice so a future switch to
// utf8.RuneCountInString is a deliberate change.
func TestNew_MessageLengthBoundary(t *testing.T) {
	t.Parallel()

	// Boundary: exactly 80 bytes is accepted.
	ok80 := strings.Repeat("a", 80)
	e := mustNew(t, CategoryConfig, string(CodeConfigMalformed), ok80)
	if len(e.Message()) != 80 {
		t.Fatalf("len = %d", len(e.Message()))
	}

	// Boundary: 81 bytes is the first invalid length.
	expectPanic(t, "exceeds 80", func() {
		_ = New(CategoryConfig, string(CodeConfigMalformed), strings.Repeat("a", 81))
	})

	// Boundary: a single-character message is accepted.
	e = mustNew(t, CategoryConfig, string(CodeConfigMalformed), "x")
	if e.Message() != "x" {
		t.Fatalf("Message() = %q", e.Message())
	}

	// Document the byte-vs-rune semantics: a 40-rune string of
	// 2-byte runes is 80 bytes and is therefore accepted.
	twoByte := strings.Repeat("é", 40)
	if len(twoByte) != 80 {
		t.Fatalf("test setup: byte length = %d", len(twoByte))
	}
	e = mustNew(t, CategoryConfig, string(CodeConfigMalformed), twoByte)
	if len(e.Message()) != 80 {
		t.Fatalf("byte length = %d", len(e.Message()))
	}
}

// --- BDD: end-to-end construction scenarios ---------------------------

// TestBDD_ConfigLoadFailure walks through the exact scenario a
// config loader would follow when the YAML file is malformed. It is
// the user-observable acceptance test for the constructor surface.
func TestBDD_ConfigLoadFailure(t *testing.T) {
	t.Parallel()

	// Given: a config file that fails to parse.
	yamlErr := errors.New("yaml: line 12: mapping values are not allowed in this context")

	// When: the config loader wraps the parse failure.
	e := Wrap(
		CategoryConfig,
		string(CodeConfigMalformed),
		"invalid YAML in config.yaml",
		yamlErr,
	).WithContext("file_path", "/etc/forge/config.yaml").
		WithContext("line_number", "12").
		WithRemediation("Check the YAML syntax at line 12.")

	// Then: the error is inspectable, self-describing, and complete.
	if e.Code() != string(CodeConfigMalformed) {
		t.Fatalf("Code() = %q", e.Code())
	}
	if e.Category() != CategoryConfig {
		t.Fatalf("Category() = %v", e.Category())
	}
	if !errors.Is(e, yamlErr) {
		t.Fatal("errors.Is(e, yamlErr) = false")
	}
	if e.Context()["file_path"] != "/etc/forge/config.yaml" {
		t.Fatalf("context = %v", e.Context())
	}
	if e.Remediation() != "Check the YAML syntax at line 12." {
		t.Fatalf("remediation = %q", e.Remediation())
	}
	if want := "FORGE_CONFIG_MALFORMED: invalid YAML in config.yaml"; e.Error() != want {
		t.Fatalf("Error() = %q, want %q", e.Error(), want)
	}
}

// TestBDD_ChainPreservesBothCodes asserts that wrapping two errors
// of different categories preserves both, so callers can match on
// either via errors.Is.
func TestBDD_ChainPreservesBothCodes(t *testing.T) {
	t.Parallel()

	inner := New(CategoryFilesystem, string(CodeFilesystemNotFound), "read failed")
	outer := New(CategoryConfig, string(CodeConfigMalformed), "config invalid").
		WithCause(inner)

	if !errors.Is(outer, inner) {
		t.Fatal("errors.Is did not traverse the cause chain")
	}

	var target *Error
	if !errors.As(outer, &target) {
		t.Fatal("errors.As failed")
	}
	if target.Code() != string(CodeConfigMalformed) {
		t.Fatalf("As target Code() = %q", target.Code())
	}

	// errors.Is on a synthetic target with the same Code as inner
	// must match through the chain.
	synthetic := New(CategoryFilesystem, string(CodeFilesystemNotFound), "different message")
	if !errors.Is(outer, synthetic) {
		t.Fatal("errors.Is did not match inner by Code through chain")
	}
}

// --- Non-functional: concurrency --------------------------------------

// TestConcurrentBuilders exercises the fluent API under -race with
// concurrent readers and writers. Each goroutine operates on its
// own derived *Error; the shared base is only read. A reader-only
// test would not catch a regression in which a WithX method
// mutated the receiver, so each branch both reads and writes.
func TestConcurrentBuilders(t *testing.T) {
	base := New(CategoryConfig, string(CodeConfigMalformed), "invalid YAML").
		WithContext("k", "v")

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			switch i % 4 {
			case 0:
				_ = base.Code()
				_ = base.Message()
				_ = base.Category()
				_ = base.Context()
				_ = base.Remediation()
				_ = base.Error()
			case 1:
				// Reading the returned map must not affect base.
				m := base.Context()
				if m != nil {
					m["k"] = "mutated"
				}
			case 2:
				_ = base.WithContext(fmt.Sprintf("k%d", i), "v")
			case 3:
				_ = base.WithCause(errors.New("c")).
					WithRemediation("Fix it.")
			}
		}(i)
	}
	wg.Wait()

	// Post-condition: base is unchanged.
	if base.Context()["k"] != "v" {
		t.Fatalf("base was mutated: %v", base.Context())
	}
	if base.Remediation() != "" {
		t.Fatalf("base gained remediation: %q", base.Remediation())
	}
	if base.Unwrap() != nil {
		t.Fatal("base gained a cause")
	}
}

// --- Non-functional: allocation discipline (informational) ------------

// BenchmarkNew measures the cost of the minimal constructor.
func BenchmarkNew(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = New(CategoryConfig, string(CodeConfigMalformed), "invalid YAML")
	}
}

// BenchmarkWithContext measures the cost of one fluent step. The
// context map is copied on each call, so this is expected to
// allocate O(len(context)) per step.
func BenchmarkWithContext(b *testing.B) {
	base := New(CategoryConfig, string(CodeConfigMalformed), "invalid YAML").
		WithContext("a", "1").
		WithContext("b", "2")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = base.WithContext("c", "3")
	}
}

// --- Fuzz: constructor inputs -----------------------------------------

// FuzzCodeFormat asserts an exact equivalence between the full
// acceptance predicate of New (code matches the regex, carries the
// config prefix, and is a member of the registry) and "constructs
// without panicking". The category is fixed to CategoryConfig.
func FuzzCodeFormat(f *testing.F) {
	seeds := []string{
		string(CodeConfigMalformed),
		"FORGE_CONFIG_X",             // well-formed, unregistered
		"FORGE_CONFIG_MALFORM",       // typo
		"FORGE_FILESYSTEM_NOT_FOUND", // registered, wrong category
		"FORGE_CONFIG_A",             // well-formed, unregistered
		"FORGE_CONFIG_X_",            // well-formed, unregistered
		"FORGE_CONFIG__X",            // fails the regex
		"FORGE_A",                    // fails the prefix
		"",                           // empty
		"forge_config_malformed",     // lowercase
		"FORGE_",                     // empty tail
		"FORGE_1",                    // digit after prefix
		"FORGE_Config",               // lowercase in tail
		"FORGE_ config",              // space
		"FORGE_CONFIG_MALFORMED\n",   // trailing newline
		"FORGE_Ä",                    // non-ASCII
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, code string) {
		wantPrefix := CategoryConfig.Prefix()
		valid := codePattern.MatchString(code) &&
			strings.HasPrefix(code, wantPrefix) &&
			IsRegistered(Code(code))

		defer func() {
			r := recover()
			switch {
			case valid && r != nil:
				t.Fatalf("valid code %q panicked: %v", code, r)
			case !valid && r == nil:
				t.Fatalf("invalid code %q did not panic", code)
			}
		}()
		_ = New(CategoryConfig, code, "x")
	})
}

// FuzzMessage asserts that the constructor's acceptance of a
// message is exactly characterised by: non-empty, <= 80 bytes,
// no trailing period, no CR/LF. This pins the byte-based limit.
func FuzzMessage(f *testing.F) {
	seeds := []string{
		"x",
		"",
		"bad.",
		"bad\n",
		"bad\r",
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
		_ = New(CategoryConfig, string(CodeConfigMalformed), msg)
	})
}

// FuzzContextKey asserts that the snake_case rule for context keys
// is exactly characterised by: non-empty, first char in [a-z],
// remaining chars in [a-z0-9_]. Any future change to the rule
// should be accompanied by a change to this predicate.
func FuzzContextKey(f *testing.F) {
	seeds := []string{
		"a",
		"file_path",
		"line_number",
		"",
		"FilePath",
		"file-path",
		"_file",
		"1file",
		"file path",
		"file.path",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	base := New(CategoryConfig, string(CodeConfigMalformed), "invalid YAML")

	f.Fuzz(func(t *testing.T, key string) {
		wantValid := isSnakeCase(key)

		defer func() {
			r := recover()
			switch {
			case wantValid && r != nil:
				t.Fatalf("valid key %q panicked: %v", key, r)
			case !wantValid && r == nil:
				t.Fatalf("invalid key %q did not panic", key)
			}
		}()
		_ = base.WithContext(key, "v")
	})
}

// isSnakeCase is the test-side predicate that mirrors the rule
// enforced by validateContextKey. The two must agree; the fuzzer
// exists precisely to detect divergence.
func isSnakeCase(key string) bool {
	if key == "" {
		return false
	}
	for i, r := range key {
		switch {
		case r >= 'a' && r <= 'z':
			// ok
		case r == '_' || (r >= '0' && r <= '9'):
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}
