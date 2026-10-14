// Package forgeerr — constructor API.
//
// This file defines the only public ways to create a *Error. All
// invariants from forgeerr.go (I1–I8) are enforced here, at the
// single choke point through which every Forge error must pass.
//
// Design contract (WBS 10.1.2, updated by WBS 10.2.1):
//
//   - New creates errors that have no cause. Use it for errors the
//     program itself produced: validation failures, unknown
//     commands, forbidden states.
//
//   - Wrap creates errors that carry a cause. Use it when wrapping
//     a library error, an os error, or any error whose original
//     type or value a caller might want to inspect via errors.Is
//     or errors.As.
//
//   - WithCause, WithContext, and WithRemediation return *copies*
//     of the receiver. The receiver is never mutated. This is what
//     makes *Error safe for concurrent use without a mutex (I8).
//
//   - Wrap(nil, ...) panics. Wrapping nil is a programming error:
//     the caller almost certainly meant to call New.
//
//   - Wrap(<*Error>, ...) returns the existing *Error unchanged.
//     This prevents double-wrapping, which would corrupt Code
//     comparisons via errors.Is and confuse downstream formatters.
//
//   - The Code prefix MUST match the Category. New and Wrap obtain
//     the required prefix from Category.Prefix() (see category.go)
//     and panic on mismatch. The prefix rule has exactly one
//     source of truth, so adding a category requires an edit to
//     category.go and nothing here.
//
// Only the Go standard library is used (see docs/dependency-policy.md).
package forgeerr

import (
	"errors"
	"fmt"
	"strings"
)

// New creates a *Error with the given category, code, and message.
//
// The code parameter is forgeerr.Code, not string, so that the
// compiler rejects a call site that passes a message, a category,
// or any other string in the code position. The registry in
// codes.go declares every code as forgeerr.Code; the constructor
// matches.
func New(category Category, code Code, message string) *Error {
	validateCategory(category)
	validateCode(category, code)
	validateMessage(message)

	return &Error{
		code:     string(code),
		message:  message,
		category: category,
	}
}

// Wrap creates a *Error with the given category, code, message, and
// cause.
//
// The code parameter is forgeerr.Code for the same reason as in New.
func Wrap(category Category, code Code, message string, cause error) *Error {
	if cause == nil {
		panic("forgeerr: Wrap called with nil cause; use New instead")
	}

	var existing *Error
	if errors.As(cause, &existing) {
		return existing
	}

	validateCategory(category)
	validateCode(category, code)
	validateMessage(message)

	return &Error{
		code:     string(code),
		message:  message,
		category: category,
		cause:    cause,
	}
}

// WithCause returns a copy of e with cause attached.
//
// The receiver is not mutated. The returned *Error shares e's
// Code, Category, Message, Context, and Remediation; only the cause
// differs.
//
// WithCause panics if cause is nil. This is deliberate: a caller
// who has nothing to attach should not call WithCause at all, and
// silently accepting nil would hide the mistake.
//
// Example:
//
//	base := forgeerr.New(
//	    forgeerr.CategoryFilesystem,
//	    "FORGE_FILESYSTEM_READ",
//	    "read failed",
//	)
//	wrapped := base.WithCause(io.ErrUnexpectedEOF)
func (e *Error) WithCause(cause error) *Error {
	if cause == nil {
		panic("forgeerr: WithCause called with nil cause")
	}
	return &Error{
		code:        e.code,
		message:     e.message,
		category:    e.category,
		cause:       cause,
		context:     cloneContext(e.context),
		remediation: e.remediation,
	}
}

// WithContext returns a copy of e with key set to value.
//
// The receiver is not mutated. If key already exists in e's
// context, its value is replaced in the returned copy; the
// receiver's context is unchanged.
//
// key must be non-empty and snake_case (per the frozen naming
// convention in WBS 10.1.2). value may be any string, including
// the empty string; empty values are permitted because they can be
// meaningful ("the flag was passed but empty").
//
// WithContext panics if key is empty or does not match the
// snake_case convention. Values are not validated for secrets;
// callers are responsible for redaction (see WBS 10.4.1).
//
// Example:
//
//	e := forgeerr.New(...).
//	    WithContext("file_path", "/etc/forge/config.yaml").
//	    WithContext("line_number", "12")
func (e *Error) WithContext(key, value string) *Error {
	validateContextKey(key)

	ctx := cloneContext(e.context)
	if ctx == nil {
		ctx = make(map[string]string, 1)
	}
	ctx[key] = value

	return &Error{
		code:        e.code,
		message:     e.message,
		category:    e.category,
		cause:       e.cause,
		context:     ctx,
		remediation: e.remediation,
	}
}

// WithRemediation returns a copy of e with the given remediation.
//
// The receiver is not mutated. The returned *Error shares e's
// Code, Category, Message, Cause, and Context; only the
// remediation differs.
//
// remediation must be a single line. If non-empty, it must end with
// a period (per the frozen naming convention in WBS 10.1.2).
// WithRemediation panics on violation.
//
// Example:
//
//	e := forgeerr.New(...).
//	    WithRemediation("Check the YAML syntax at line 12.")
func (e *Error) WithRemediation(remediation string) *Error {
	validateRemediation(remediation)

	return &Error{
		code:        e.code,
		message:     e.message,
		category:    e.category,
		cause:       e.cause,
		context:     cloneContext(e.context),
		remediation: remediation,
	}
}

// --- Internal helpers -------------------------------------------------

// cloneContext returns a defensive copy of src, or nil if src is
// nil or empty. Returning nil for an empty map preserves the
// "no context" vs "empty context" distinction that Context()
// exposes to callers.
func cloneContext(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

// validateCode enforces invariants I1 and I3, the AC7 rule that
// the code carries the prefix required by category, and the
// WBS 10.3.1 rule that the code is a member of the frozen
// registry.
//
// The four checks are ordered from the most specific failure to
// the least:
//
//  1. Empty code (I1).
//  2. Regex mismatch (I3). Catches any shape error, including
//     codes that look plausible but are malformed.
//  3. Prefix mismatch (AC7). Catches a well-formed code that
//     belongs to the wrong category.
//  4. Unregistered (WBS 10.3.1). Catches a well-formed code that
//     matches the category but is not part of the registry. This
//     is the only check that a regex cannot express, because the
//     registry is a finite set of specific strings.
//
// # Why the parameter is Code and not string
//
// The parameter type is forgeerr.Code, not string. This is the
// load-bearing half of the compile-time guarantee that the code
// registry exists to provide. A caller that passes a message, a
// category name, or any other string in the code position fails
// to compile, because Go does not implicitly convert an untyped
// string constant to a named string type in a function-call
// argument position unless the constant is a member of that named
// type's underlying type's untyped-constant set. In practice, the
// only way to satisfy the parameter is:
//
//   - a named constant from codes.go (the intended use), or
//   - an explicit string(...) conversion of such a constant
//     (the accommodation documented in safefs/errors.go), or
//   - a value of type forgeerr.Code read from a lookup table
//     (rare, and reviewed).
//
// A bare string literal — "FORGE_CONFIG_MALFORMED" — does not
// satisfy the parameter, because a string literal is an untyped
// constant of kind string, and Code is a *named* type whose
// underlying type is string. Go's assignability rules do not
// bridge that gap implicitly. The compiler rejects the call
// before the runtime check runs.
//
// # The category argument
//
// The category argument is assumed to have already been validated;
// callers must not pass an invalid Category. If they do,
// category.Prefix() panics with a message that identifies the
// invalid value, which is the correct diagnostic: the caller
// skipped a required check.
//
// # Why the checks are ordered this way
//
// The order is not arbitrary. Each check rules out a distinct
// class of failure and produces a distinct diagnostic:
//
//   - Empty (I1) is a caller bug so basic that it deserves its own
//     message, not a regex error. A caller who passes "" almost
//     always meant to pass a variable that turned out to be zero.
//
//   - Regex (I3) catches shape errors: lowercase letters, hyphens,
//     a leading digit, a trailing underscore, an empty specific
//     segment. These are cheap to explain ("does not match") and
//     cheap to fix.
//
//   - Prefix (AC7) catches a well-formed code that is in the
//     wrong category. This is the failure mode a code-review miss
//     produces most often: someone copies a code from another
//     category and changes the specific segment but forgets the
//     category segment. The message names both the required
//     prefix and the category, so the fix is obvious.
//
//   - Registry (WBS 10.3.1) catches the failure the regex cannot:
//     a code that is well-formed and in the right category but is
//     not a member of the closed set. This is the check that
//     makes "the registry is a closed set" real. Without it, a
//     new code string could be invented at any call site and the
//     stability policy of ADR-012 would be aspirational.
//
// The order is specific-to-general: each successive check assumes
// the previous ones passed. Reordering would produce misleading
// diagnostics — for example, checking the registry before the
// prefix would report a wrong-category code as "unregistered"
// rather than as "wrong prefix," which is less actionable.
//
// # Why the empty check is separate from the regex check
//
// codePattern requires a non-empty match, so an empty code would
// also fail the regex check. The empty check exists anyway so
// that the diagnostic for "" does not mention a regex. "Code
// must not be empty" is a clearer message than "code \"\" does
// not match ^FORGE_[A-Z][A-Z0-9_]*$" for the caller who most
// needs it — the one who has a zero-valued variable and no
// obvious reason it should be empty.
func validateCode(category Category, code Code) {
	if code == "" {
		panic("forgeerr: code must not be empty (I1)")
	}

	// Regex and prefix checks operate on the underlying string.
	// The conversion is a no-op at runtime — Code is a named
	// string, and the memory layout is identical — but it makes
	// the string operations explicit and keeps the compiler from
	// having to reason about named-type methods on the regexp
	// package's parameter list.
	s := string(code)

	if !codePattern.MatchString(s) {
		panic(fmt.Sprintf(
			"forgeerr: code %q does not match %s (I3)",
			s, codePattern.String(),
		))
	}

	want := category.Prefix()
	if !strings.HasPrefix(s, want) {
		panic(fmt.Sprintf(
			"forgeerr: code %q does not start with %q required by category %s (AC7)",
			s, want, category,
		))
	}

	// IsRegistered already takes a Code, so no conversion is
	// needed here. If a future refactor changes IsRegistered's
	// parameter type back to string, this call site breaks at
	// compile time — which is the desired behaviour.
	if !IsRegistered(code) {
		panic(fmt.Sprintf(
			"forgeerr: code %q is not in the registry (WBS 10.3.1)",
			s,
		))
	}
}

// validateContextKey enforces the snake_case convention for context
// keys, defined in WBS 10.1.2.
//
// The rule: a non-empty string of lowercase ASCII letters, digits,
// and underscores, starting with a letter. This is deliberately
// strict: context keys appear in JSON error envelopes (WBS 10.4.1)
// and in log lines, and mixed-case or dotted keys would force
// downstream consumers to normalise, which they will do
// inconsistently.
func validateContextKey(key string) {
	if key == "" {
		panic("forgeerr: context key must not be empty")
	}
	for i, r := range key {
		switch {
		case r >= 'a' && r <= 'z':
			// ok
		case r == '_' || (r >= '0' && r <= '9'):
			if i == 0 {
				panic(fmt.Sprintf(
					"forgeerr: context key %q must start with a lowercase letter",
					key,
				))
			}
		default:
			panic(fmt.Sprintf(
				"forgeerr: context key %q contains %q; only [a-z0-9_] are permitted",
				key, r,
			))
		}
	}
}
