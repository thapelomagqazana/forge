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
// New is the primary constructor for errors the program produces
// itself. It never attaches a cause; use Wrap for that.
//
// New panics if any invariant is violated:
//
//   - category is not a member of the closed set (I6),
//   - code is empty (I1),
//   - code does not match ^FORGE_[A-Z][A-Z0-9_]*$ (I3),
//   - code does not carry the prefix required by category (AC7),
//   - message is empty (I2),
//   - message is longer than 80 bytes (I4),
//   - message ends with a period (I5),
//   - message contains CR or LF (I5).
//
// New never returns nil. Callers may use the result without a nil
// check.
//
// Example:
//
//	return forgeerr.New(
//	    forgeerr.CategoryConfig,
//	    "FORGE_CONFIG_MALFORMED",
//	    "invalid YAML in config.yaml",
//	)
func New(category Category, code, message string) *Error {
	validateCategory(category)
	validateCode(category, code)
	validateMessage(message)

	return &Error{
		code:     code,
		message:  message,
		category: category,
	}
}

// Wrap creates a *Error with the given category, code, message, and
// cause.
//
// Behaviour:
//
//   - If cause is nil, Wrap panics. Wrapping nil is a programming
//     error: the caller almost certainly meant New.
//
//   - If cause is already a *Error, Wrap returns cause unchanged.
//     The category, code, and message arguments are ignored. This
//     prevents double-wrapping, which would corrupt Code-based
//     errors.Is comparisons and produce misleading formatter output.
//
//   - Otherwise, Wrap constructs a new *Error whose Cause is cause.
//     All of New's invariant checks apply.
//
// Wrap never returns nil. Callers may use the result without a nil
// check.
//
// Example:
//
//	if err := os.ReadFile(path); err != nil {
//	    return forgeerr.Wrap(
//	        forgeerr.CategoryFilesystem,
//	        "FORGE_FILESYSTEM_READ",
//	        "failed to read config file",
//	        err,
//	    )
//	}
func Wrap(category Category, code, message string, cause error) *Error {
	if cause == nil {
		panic("forgeerr: Wrap called with nil cause; use New instead")
	}

	// Fast path: the cause is already a *Error. Return it unchanged
	// so that errors.Is comparisons on Code remain stable and the
	// formatter sees exactly one Code per chain.
	//
	// We intentionally return the cause by value (it is a pointer,
	// so the receiver is shared). The receiver is immutable, so
	// sharing is safe.
	var existing *Error
	if errors.As(cause, &existing) {
		return existing
	}

	validateCategory(category)
	validateCode(category, code)
	validateMessage(message)

	return &Error{
		code:     code,
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
// The three checks are ordered from the most specific failure to
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
// The category argument is assumed to have already been validated;
// callers must not pass an invalid Category. If they do,
// category.Prefix() panics with a message that identifies the
// invalid value, which is the correct diagnostic: the caller
// skipped a required check.
func validateCode(category Category, code string) {
	if code == "" {
		panic("forgeerr: code must not be empty (I1)")
	}
	if !codePattern.MatchString(code) {
		panic(fmt.Sprintf(
			"forgeerr: code %q does not match %s (I3)",
			code, codePattern.String(),
		))
	}
	want := category.Prefix()
	if !strings.HasPrefix(code, want) {
		panic(fmt.Sprintf(
			"forgeerr: code %q does not start with %q required by category %s (AC7)",
			code, want, category,
		))
	}
	if !IsRegistered(Code(code)) {
		panic(fmt.Sprintf(
			"forgeerr: code %q is not in the registry (WBS 10.3.1)",
			code,
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
