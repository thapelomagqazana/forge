// Package forgeerr defines the canonical error type for Forge.
//
// Every user-facing error in Forge MUST be a *Error or wrap one.
// This guarantees consistent formatting, exit codes, and redaction
// across the entire CLI surface, regardless of which subsystem
// produced the failure.
//
// Design contract (WBS 10.1.1, extended by WBS 10.1.2, WBS 10.2.1,
// WBS 10.3.1, and WBS 10.3.2):
//
//   - A *Error is never nil.
//   - A *Error is immutable after construction.
//   - A *Error is safe for concurrent use.
//   - A *Error is chainable via WithCause and errors.Join.
//   - A *Error carries a stable, machine-readable Code that never
//     changes once shipped.
//
// Construction API:
//
//   - New(category, code, message) creates an error without a cause.
//   - Wrap(category, code, message, cause) creates an error with a
//     cause, or returns the cause unchanged if it is already a
//     *Error.
//   - WithCause, WithContext, and WithRemediation return copies of
//     the receiver, preserving immutability.
//
// Wrapping contract (WBS 10.3.2):
//
//   - Unwrap() returns the immediate cause only. The standard
//     library traverses the chain.
//   - Is(target) matches by Code, not by pointer identity, and
//     not by message. A typed-nil *Error target returns false.
//   - As(target) assigns the *Error to a **Error target. It does
//     not traverse the cause chain. A nil **Error target returns
//     false.
//
// Those constructors live in constructors.go; the taxonomy lives
// in category.go; the code registry lives in codes.go. This file
// defines the type, its unexported fields, its accessors, its
// chain methods, and the pure invariant helpers.
//
// The package depends only on the Go standard library (see
// docs/dependency-policy.md).
package forgeerr

import (
	"fmt"
	"regexp"
	"strings"
)

// codePattern enforces the machine-readable Code format:
//
//	^FORGE_[A-Z][A-Z0-9_]*$
//
// The pattern has two distinct parts:
//
//   - "FORGE_" (literal) followed by a mandatory [A-Z] — the first
//     character of the tail MUST be an uppercase ASCII letter.
//     Digits and underscores are rejected in this position.
//
//   - [A-Z0-9_]* — the remainder MAY contain uppercase letters,
//     digits, and underscores. Trailing and interior underscores
//     are therefore permitted; leading underscores are not.
//
// The pattern is compiled once at package init. Matching is
// allocation-free.
var codePattern = regexp.MustCompile(`^FORGE_[A-Z][A-Z0-9_]*$`)

// maxMessageLen is the inclusive upper bound on message length,
// measured in bytes (len). Messages are rendered on a single
// terminal line and must remain readable in narrow windows and CI
// logs. 80 bytes aligns with the project's CLI UX spec (see
// docs/cli-ux-spec.md).
//
// Note on units: the invariant is stated as "≤ 80 chars" in the
// WBS, but the implementation counts bytes. For ASCII-only messages
// these coincide. If Forge later admits non-ASCII messages, this
// constant and validateMessage must switch to utf8.RuneCountInString
// in the same commit that updates docs/architecture.md.
const maxMessageLen = 80

// Error is the canonical error type for Forge.
//
// A *Error is:
//   - never nil,
//   - immutable after construction,
//   - safe for concurrent use,
//   - chainable via WithCause,
//   - and compatible with errors.Is / errors.As.
//
// All fields are unexported. Construction MUST go through New or
// Wrap (see constructors.go) so that invariants I1–I6 are enforced
// exactly once, at the boundary where programmer error is still
// recoverable — a panic rather than a silent malformed error
// reaching the user.
type Error struct {
	// code is a stable, machine-readable identifier.
	// Format: "FORGE_<CATEGORY>_<SPECIFIC>".
	// Example: "FORGE_CONFIG_MALFORMED".
	// Never empty. Never changed once shipped (see WBS 10.3.1).
	code string

	// message is the user-facing description.
	// Single line, no trailing period, <= 80 bytes.
	// Never empty.
	message string

	// category classifies the error for exit-code mapping.
	// See category.go for the closed set.
	category Category

	// cause is the wrapped underlying error, if any.
	// May be nil. Preserved via Unwrap().
	cause error

	// context is structured key-value metadata.
	// May be nil. Keys and values are strings.
	// Never contains secrets (see WBS 10.4.1).
	context map[string]string

	// remediation is a suggested action.
	// May be empty. If non-empty, it ends with a period.
	remediation string
}

// Compile-time assertions: *Error must satisfy the standard error
// interfaces. These fail the build if a future refactor drops a
// method, which is cheaper and more reliable than a runtime check.
var (
	_ error                       = (*Error)(nil)
	_ interface{ Unwrap() error } = (*Error)(nil)
	_ interface{ Is(error) bool } = (*Error)(nil)
	_ interface{ As(any) bool }   = (*Error)(nil)
)

// Code returns the stable, machine-readable identifier.
// Never empty.
func (e *Error) Code() string { return e.code }

// Message returns the user-facing description.
// Never empty. Does not end with a period. Length <= 80 bytes.
func (e *Error) Message() string { return e.message }

// Category returns the classification used for exit-code mapping.
// Always a member of the closed set.
func (e *Error) Category() Category { return e.category }

// Context returns a copy of the structured metadata.
//
// Returns nil if no context was supplied. Callers may mutate the
// returned map without affecting the *Error. This is what makes
// *Error safe to share across goroutines without a mutex.
func (e *Error) Context() map[string]string {
	if len(e.context) == 0 {
		return nil
	}
	out := make(map[string]string, len(e.context))
	for k, v := range e.context {
		out[k] = v
	}
	return out
}

// Remediation returns the suggested action, or "" if none.
// If non-empty, ends with a period.
func (e *Error) Remediation() string { return e.remediation }

// Error implements the error interface.
//
// The returned string is programmer-facing, not user-facing. It is
// intended for logs, stack traces, and debugging. User-facing
// rendering is a separate concern handled by WBS 10.4.1.
//
// Format: "<code>: <message>"
// Example: "FORGE_CONFIG_MALFORMED: invalid YAML in config.yaml"
func (e *Error) Error() string {
	return e.code + ": " + e.message
}

// Unwrap returns the immediate cause, or nil if no cause was
// attached.
//
// Unwrap does NOT traverse the chain. It returns exactly what was
// passed to New or WithCause. The standard library's errors.Is and
// errors.As walk the chain by calling Unwrap repeatedly, so a
// chain of *Error values is traversed correctly by the standard
// library without Unwrap doing the traversal itself.
//
// Contract (WBS 10.3.2, rule "Unwrap returns the immediate cause"):
//
//	(*Error)(nil).Unwrap()                  == nil
//	New(..., no cause).Unwrap()             == nil
//	New(..., cause=x).Unwrap()              == x
//	New(..., cause=x).WithCause(y).Unwrap() == y
//
// This method is safe for concurrent use: it reads a single
// immutable field.
func (e *Error) Unwrap() error { return e.cause }

// Is reports whether the receiver and target are the same logical
// error, where "same" means "same Code".
//
// Contract (WBS 10.3.2, rules "Is matches by code" and "Is does
// NOT match by message"):
//
//   - If target is not a *Error, Is returns false. It does not
//     attempt to interpret the target in any other way.
//
//   - If target is a *Error with the same Code, Is returns true.
//     The message, category, cause, context, and remediation are
//     ignored.
//
//   - If target is a *Error with a different Code, Is returns
//     false.
//
//   - If target is a typed-nil *Error (a nil pointer stored in a
//     non-nil error interface), Is returns false. The type
//     assertion succeeds, but the pointer is nil, so there is no
//     code to compare. This is the case that distinguishes a
//     typed-nil *Error from a nil error: the former survives the
//     type assertion and requires an explicit nil check, the
//     latter fails the assertion.
//
// The comparison is shallow: Is does not traverse the receiver's
// cause chain and does not traverse the target's cause chain. The
// standard library's errors.Is is responsible for traversal: it
// calls Unwrap on the receiver repeatedly and, for each level,
// invokes the receiver's Is method. So a chain of *Error values
// is still matched correctly by errors.Is, but each individual
// call to Is compares exactly two *Error values.
//
// This method is safe for concurrent use: it reads two immutable
// fields (its own code and the target's code). The target must be
// a non-nil *Error; a typed-nil target is handled by the guard.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok || t == nil {
		return false
	}
	return e.code == t.code
}

// As assigns the receiver to a **Error target.
//
// Contract (WBS 10.3.2, rule "As assigns the entire *Error"):
//
//   - If target is not a **Error, As returns false and does not
//     modify the target.
//
//   - If target is a nil **Error (a nil pointer stored in a
//     non-nil any interface), As returns false. The type
//     assertion succeeds, but the pointer is nil, so there is no
//     location to write to.
//
//   - If target is a **Error with a nil inner pointer, As is
//     still valid: the assignment writes the receiver's pointer
//     into the caller's variable, replacing the nil. This is the
//     intended use, and it is not the same case as a nil **Error.
//
//   - If target is a **Error with a non-nil inner pointer, As
//     overwrites the existing value with the receiver and returns
//     true.
//
// The assignment is shallow: As does not traverse the receiver's
// cause chain. The standard library's errors.As is responsible
// for traversal: it calls Unwrap on the receiver repeatedly and,
// for each level, invokes the receiver's As method. So a chain of
// *Error values is still reachable by errors.As, but each
// individual call to As assigns exactly one *Error.
//
// Note on the assignment: *target is set to e, not to a copy.
// The receiver is immutable, so sharing the pointer is safe.
//
// This method is safe for concurrent use: it writes through the
// caller-supplied pointer (which is the caller's responsibility)
// and reads only its own pointer.
func (e *Error) As(target any) bool {
	t, ok := target.(**Error)
	if !ok || t == nil {
		return false
	}
	*t = e
	return true
}

// --- Invariant helpers ------------------------------------------------
//
// These helpers are called exclusively by the constructors in
// constructors.go. They are pure functions of their inputs and do
// not read or write package state. Each helper panics on violation,
// because a malformed *Error is a programming error, not a runtime
// error — the alternative (returning an error about an error)
// obscures the real failure and invites silent corruption of the
// user-facing contract.

// validateMessage enforces invariants I2, I4, and I5.
//
// Order matters: the single-line check runs before the trailing-
// period check, so that a message which violates both rules
// (e.g. "bad.\n") produces the more specific diagnostic
// ("single line") rather than the cosmetic one.
func validateMessage(msg string) {
	if msg == "" {
		panic("forgeerr: message must not be empty (I2)")
	}
	if len(msg) > maxMessageLen {
		panic(fmt.Sprintf(
			"forgeerr: message length %d exceeds %d (I4)",
			len(msg), maxMessageLen,
		))
	}
	if strings.ContainsAny(msg, "\n\r") {
		panic("forgeerr: message must be single line (I5)")
	}
	if strings.HasSuffix(msg, ".") {
		panic("forgeerr: message must not end with a period (I5)")
	}
}

// validateRemediation enforces the remediation-specific rules:
// single line, and if non-empty, ends with a period.
//
// The single-line check runs first for the same reason as in
// validateMessage: a remediation that violates both rules should
// produce the safety-relevant diagnostic, not the cosmetic one.
func validateRemediation(r string) {
	if r == "" {
		return
	}
	if strings.ContainsAny(r, "\n\r") {
		panic("forgeerr: remediation must be single line")
	}
	if !strings.HasSuffix(r, ".") {
		panic("forgeerr: remediation must end with a period")
	}
}

// validateCategory enforces invariant I6.
//
// The check is a method on Category (see category.go) so that the
// closed-set boundary is defined in exactly one place. Since
// WBS 10.2.1 the category is a string, so the diagnostic uses %q
// rather than %d.
func validateCategory(c Category) {
	if !c.valid() {
		panic(fmt.Sprintf(
			"forgeerr: category %q is not a member of the closed set (I6)",
			string(c),
		))
	}
}
