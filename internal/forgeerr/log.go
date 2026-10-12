// Package forgeerr — programmer-facing log format.
//
// This file implements slog.LogValuer on *Error. The interface is
// what makes slog.Any("error", err) produce a structured entry
// rather than a single formatted string. Without it, slog would
// call err.Error() and record the resulting string, which loses
// the structure — the code, the category, the cause, the context
// — that a log consumer needs.
//
// # The frozen log shape
//
// The value returned by LogValue is a slog group named "error".
// The group contains:
//
//   - error.code      — the stable machine-readable code.
//   - error.category  — the taxonomy category name.
//   - error.cause     — the cause's Error() string, if a cause
//     is present. Omitted if the cause is nil.
//   - error.<key>     — one attribute per context key-value pair,
//     with the keys sorted alphabetically.
//
// When slog renders the group with a text handler, the result is:
//
//	level=ERROR msg="<message>" error.code=<code> error.category=<category> error.cause="<cause>" error.<key>=<value> ...
//
// When slog renders the group with a JSON handler, the result is:
//
//	{"level":"ERROR","msg":"<message>","error":{"code":"<code>","category":"<category>","cause":"<cause>","<key>":"<value>"}}
//
// The two shapes are the same group rendered by two handlers;
// this file does not concern itself with the rendering.
//
// # Rules
//
//  1. The log format is structured. It is not a single string.
//  2. The cause is included as a string field when present.
//  3. The context is flattened: one attribute per key, not a
//     nested group.
//  4. The category is included as a string field.
//  5. The user-facing wrapper — "Error: ", the "Code:  " line,
//     the "Suggestion:" block — is not included. Those are
//     decorations for the human reader; the log consumer wants
//     the fields.
//  6. The context keys are sorted alphabetically, so that the
//     output is deterministic.
//  7. The value is safe for concurrent use: it reads only the
//     receiver's immutable fields.
//
// # Redaction
//
// Every context value is passed through redact (secret-typed
// values are replaced with the marker) and redactPath (a
// home-directory prefix is replaced with "~") before it is
// appended to the group. The two transformations are in
// redact.go and are frozen in WBS 10.4.3. The user-facing
// formatter (format.go / WBS 10.4.1) applies the same two
// functions, so the two outputs cannot diverge on what is
// redacted.
//
// The code, the category, and the cause are not redacted.
//
//   - The code and the category are registered identifiers
//     (WBS 10.3.1); they do not contain user data.
//   - The cause is a programmer concern. It is rendered only by
//     this formatter, never by the user-facing formatter. A
//     programmer debugging a log entry needs the cause; redacting
//     it would hide diagnostic information from the audience that
//     needs it most.
//
// A caller who places a secret in the cause's Error() string is
// responsible for the consequence. The recommended practice is
// to construct the cause from a *forgeerr.Error whose context
// uses secret-typed keys; that error's own LogValue would redact,
// but this formatter does not recurse into the cause.
//
// # Relationship to the user-facing format
//
// The user-facing format is frozen in WBS 10.4.1 and implemented
// by Format in format.go. The two formats are deliberately
// distinct:
//
//   - Format is for humans: it includes the "Error: " prefix,
//     the "Code:  " line, and the "Suggestion:" block. It is
//     written to stderr.
//   - LogValue is for log consumers: it includes the code, the
//     category, the cause, and the context as structured fields.
//     It is written to whatever destination the slog handler is
//     configured with.
//
// A caller chooses between them by choosing between
// `fmt.Fprint(stderr, forgeerr.Format(err, opts))` and
// `slog.Any("error", err)`.
//
// # Relationship to the slog handler
//
// The slog handler may apply an additional ReplaceAttr
// transformation, for example to redact values that match a
// secret pattern the package's key-based policy does not cover.
// The two layers are defence in depth:
//
//   - The key-based redaction in this file catches secrets whose
//     context keys are named in the frozen list.
//   - The handler-level redaction catches secrets whose values
//     happen to match a pattern the handler knows about.
//
// Neither layer is a substitute for the caller not placing
// secrets in the error in the first place. See the WithContext
// doc comment for the caller-side rule.
//
// Only the Go standard library is used (see
// docs/dependency-policy.md).
package forgeerr

import "log/slog"

// logGroupName is the name of the slog group that LogValue
// returns. The name appears in the output of both the text and
// the JSON handler:
//
//	error.code=FORGE_CONFIG_MALFORMED
//	{"error":{"code":"FORGE_CONFIG_MALFORMED",...}}
//
// The name is a constant so that it has exactly one definition.
// A future change to the group name is a change to the log
// contract and requires an ADR.
const logGroupName = "error"

// LogValue implements slog.LogValuer.
//
// The method returns a slog group whose attributes are the
// error's structured fields: code, category, cause, and one
// attribute per context key-value pair. The context keys are
// sorted alphabetically, so that the output is deterministic
// across calls and across runs. The context values are redacted
// (WBS 10.4.3) before they are appended.
//
// # When the method is called
//
// slog calls LogValue when a *Error is passed to a log call via
// slog.Any, slog.Log, or any other function that accepts a
// slog.Value. The method is not called by the user-facing
// Format function; Format reads the error's fields directly.
//
// # The group
//
// The group contains the following attributes, in this order:
//
//  1. code      — always present; never empty.
//  2. category  — always present; never empty.
//  3. cause     — present only if e.cause != nil. The value is
//     e.cause.Error(), which is the cause's message.
//     A cause whose message is empty is still emitted;
//     the attribute's value is the empty string.
//  4. <key>=<value> — one attribute per context entry, with the
//     keys sorted alphabetically and the values
//     redacted.
//
// # What the group does not contain
//
//   - The error's message. The message is the primary log
//     message, not a field of the error group. slog renders the
//     message via the "msg" key, which is separate from the
//     "error" group.
//   - The error's remediation. The remediation is a suggestion
//     for a human reader; the log consumer does not need it.
//   - The error's type. The type is always *Error, so recording
//     it would be redundant.
//   - The user-facing wrapper. The "Error: " prefix, the
//     "Code:  " line, and the "Suggestion:" block are
//     decorations for the human reader; the log consumer wants
//     the fields.
//
// # Concurrency
//
// The method is safe for concurrent use. It reads the receiver's
// fields, which are immutable after construction (WBS 10.1.1,
// invariant I8). It does not write to the receiver, to a global,
// or to any shared state.
//
// The method does call e.cause.Error() if the cause is present.
// The cause is an arbitrary error, and its Error method may not
// be safe for concurrent use. This is the caller's
// responsibility: if the caller shares a *Error whose cause is
// not concurrency-safe, the caller must synchronise access. The
// package does not add a lock, because doing so would impose a
// cost on every *Error regardless of whether its cause needs
// it.
//
// The method also calls redact and redactPath, both of which are
// pure and safe for concurrent use.
func (e *Error) LogValue() slog.Value {
	attrs := make([]slog.Attr, 0, 3+len(e.context))

	// code is always present. The constructor guarantees a
	// non-empty value (invariant I1).
	attrs = append(attrs, slog.String("code", e.code))

	// category is always present. The constructor guarantees a
	// member of the closed set (invariant I6).
	attrs = append(attrs, slog.String("category", string(e.category)))

	// cause is present only when a cause was attached. The
	// attribute's value is the cause's message, not the cause
	// itself: slog cannot render an arbitrary error as a
	// structured field without knowing its type, and the
	// message is the only piece of information every error
	// shares.
	//
	// The cause is not redacted. See the package doc comment
	// for the rationale and the caller-side rule.
	if e.cause != nil {
		attrs = append(attrs, slog.String("cause", e.cause.Error()))
	}

	// context is flattened: one attribute per key, sorted
	// alphabetically. The sort makes the output deterministic;
	// Go's map iteration order is randomised.
	//
	// WBS 10.4.3: each value is redacted before it is appended.
	// The pipeline is:
	//
	//   1. redact(key, value)   — replaces a secret-typed value
	//                             with the marker.
	//   2. redactPath(value)    — replaces a home-directory
	//                             prefix with "~".
	//
	// The two steps are applied to every context value,
	// regardless of the key. The user-facing formatter
	// (format.go) applies the same two functions, so the two
	// outputs cannot diverge on what is redacted.
	for _, k := range sortedKeys(e.context) {
		v := redact(k, e.context[k])
		v = redactPath(v)
		attrs = append(attrs, slog.String(k, v))
	}

	return slog.GroupValue(attrs...)
}

// compile-time assertion: *Error implements slog.LogValuer.
//
// The interface has a single method, LogValue() slog.Value. If a
// future refactor changes the method's signature, this assertion
// fails the build, which is cheaper and more reliable than a
// runtime check.
var _ slog.LogValuer = (*Error)(nil)
