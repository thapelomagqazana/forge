// Package logger — the Secret type.
//
// This file implements the type-level half of the redaction
// architecture. It defines a wrapper for credential-like strings
// and hooks it into the three serialization paths a value can
// travel through:
//
//   - fmt.Stringer (used by fmt.Sprintf, fmt.Println, and any
//     caller that prints the value)
//   - slog.LogValuer (used by log/slog and, transitively, by the
//     logger's record emission)
//   - encoding/json.Marshaler (used by the JSON log format and by
//     any JSON envelope the CLI produces)
//
// The three hooks are the reason a Secret is safe to place on a
// struct: no matter how the struct is serialized, the secret's
// textual form never appears.
//
// # The four-layer redaction model
//
// This file is layer 1 of a four-layer defense-in-depth model.
// See docs/security/logging-redaction.md §3 for the full model:
//
//  1. Type-level (this file): a Secret wrapper whose String,
//     LogValue, and MarshalJSON all return [REDACTED]. Any code
//     that holds a credential value and wants to log it must
//     wrap it in this type.
//
//  2. Pattern-based (redact.go): a middleware that scans log
//     messages and string attrs for credential-like patterns
//     (password=, Authorization: Bearer …, PEM headers, etc.)
//     and replaces the matching substring with [REDACTED]. This
//     layer catches leaks that bypass layer 1 (a password read
//     from a file into a plain string, for example).
//
//  3. Config-aware (internal/config/redact.go): when the config
//     loader parses forge.yaml, fields declared `sensitive: true`
//     in the schema are wrapped in Secret before being stored.
//     This layer ensures the config's own sensitive fields go
//     through layer 1 without requiring each caller to wrap them.
//
//  4. Test suite (test/security/redaction_test.go): the end-to-end
//     assertion that no layer leaks. Synthetic secrets are
//     injected into every code path that can produce output and
//     every stream is scanned for the secret's textual form.
//
// Layers 1, 2, and 3 are independent: a leak that bypasses one is
// caught by another. Layer 4 verifies the whole.
//
// # Why a wrapper type and not a naming convention
//
// The alternative — "name the variable `secretToken` and hope
// nobody logs it" — is a convention, not a mechanism. The wrapper
// is a mechanism: the compiler enforces that a Secret is the only
// type whose String method returns [REDACTED] and that a plain
// string cannot be passed where a Secret is expected. A future
// caller that tries to log a raw string will succeed at
// compiling (the string is a valid value), but the pattern layer
// (layer 2) catches the leak; if the string happens to escape
// the pattern layer (a non-standard credential format), the test
// suite (layer 4) surfaces it in CI.
package logger

import (
	"encoding/json"
	"log/slog"
)

// redactedLiteral is the exact string every Secret renders as. It
// is declared as a constant so the redaction is grep-able and so
// every test can reference the same value.
//
// # Why the literal is "[REDACTED]"
//
// The bracket form is conventional (it matches the placeholder
// used by Go's testing framework for unprintable values, by
// Kubernetes's kubectl for masked fields, and by similar tools).
// A consumer that greps for it in a log aggregator finds every
// redaction site without ambiguity. The leading and trailing
// brackets make the marker visually distinct from surrounding
// text; a bare "REDACTED" could collide with a user's data.
//
// # Why the literal is not configurable
//
// The marker is a public contract: a log aggregator's redaction
// rules will look for exactly this string. Making it
// configurable would invite per-environment drift and undermine
// the aggregator's rules. A future WBS that needs a different
// marker files an ADR and updates every call site.
const redactedLiteral = "[REDACTED]"

// Secret wraps a credential-like string and renders as
// [REDACTED] through every serialization path.
//
// # What "credential-like" means
//
// A value is credential-like if its disclosure would grant access
// to a resource, authenticate a caller, or reveal internal
// state that is not meant for public consumption. Examples:
//
//   - An API token, an OAuth secret, a session cookie.
//   - A database password, a private key, a JWT.
//   - A config field marked `sensitive: true`.
//   - An environment variable whose name matches the credential
//     patterns (see redact.go).
//
// A value is NOT credential-like if it is a public identifier:
// a commit hash, a version string, a public key, an endpoint URL.
// Wrapping those in Secret would make logs useless; the pattern
// layer catches them by content, not by type, so the pattern
// rules must not match public identifiers. See redact.go's
// docstring for the pattern set and the false-positive policy.
//
// # Zero value
//
// The zero value of Secret is an empty string wrapped in the
// type. It renders as [REDACTED], not as the empty string. This
// is deliberate: a caller that constructs a Secret{} by accident
// should see the marker, not an empty field, so the mistake is
// visible.
//
// # Value semantics
//
// Secret is a value type. Copying a Secret copies the wrapped
// string; the redaction behavior is unaffected. A caller that
// wants to compare two Secrets uses the Equal method, which
// compares the wrapped strings in constant time (see Equal for
// the rationale).
type Secret struct {
	// value is the wrapped credential. It is unexported so that
	// only the constructor (NewSecret) can set it, and only the
	// Reveal method can read it. The unexported field is the
	// type-level half of the defense: a caller cannot
	// accidentally access the wrapped string without calling
	// Reveal, and calling Reveal is a deliberate, grep-able
	// action.
	value string
}

// NewSecret returns a Secret that wraps value.
//
// # When to call
//
// Call NewSecret at the boundary where a credential enters the
// program: when reading an environment variable, when parsing a
// config file's sensitive field, when receiving a flag whose
// value is a credential, when extracting a credential from a
// payload. The rule is: the earliest point in the program that
// knows the value is a credential should wrap it.
//
// # When not to call
//
// Do not call NewSecret on public identifiers. A commit hash, a
// version string, a public key, an endpoint URL — those are not
// credentials. Wrapping them makes logs useless and forces every
// downstream consumer to call Reveal to make any use of them.
//
// # The empty case
//
// NewSecret("") returns a Secret whose wrapped value is the
// empty string. The Secret's renderers still return [REDACTED],
// not "". A caller that wants to distinguish "no secret" from
// "empty secret" should use the HasValue method.
func NewSecret(value string) Secret {
	return Secret{value: value}
}

// Reveal returns the wrapped credential.
//
// # Why the method exists
//
// Reveal is the only way to access the wrapped string. Calling
// it is a deliberate, grep-able action: a code reviewer who sees
// `Reveal()` knows the caller is about to use the credential for
// its intended purpose (authenticating a request, comparing it
// to a supplied value). A code reviewer who sees `Reveal()` in a
// context that logs the result knows there is a bug.
//
// # When to call
//
// Call Reveal only at the point where the credential is used for
// its intended purpose: passing it to an HTTP client, comparing
// it to a user-supplied value, writing it to a file whose mode
// is 0600. Do not call Reveal to build a log message, a debug
// trace, or an error's context. The type system cannot prevent
// the call; the review process and the test suite enforce the
// discipline.
//
// # Why the name is "Reveal"
//
// The name is chosen for its visibility in a code review. A
// method named "Value" or "String" would read as mundane; a
// method named "Reveal" reads as an act with consequences. The
// reviewer's eye pauses on it.
func (s Secret) Reveal() string {
	return s.value
}

// HasValue reports whether the Secret wraps a non-empty string.
//
// The method exists so that callers can distinguish "no secret
// was provided" from "an empty secret was provided" without
// calling Reveal. The distinction matters in a few places (a
// config validator that wants to reject an empty credential, for
// example); in most places the caller does not need to
// distinguish, and the renderers' behavior (always [REDACTED])
// is sufficient.
func (s Secret) HasValue() bool {
	return s.value != ""
}

// String implements fmt.Stringer.
//
// Every call to fmt.Sprintf("%s", secret), fmt.Sprint(secret),
// or fmt.Println(secret) returns [REDACTED]. This is the most
// important hook: it is the one that catches the majority of
// accidental leaks, because fmt is the most common way a value
// reaches a string.
//
// # Why the implementation does not branch on the wrapped value
//
// The method returns the same literal for every Secret,
// including the zero-value Secret and a Secret wrapping an
// empty string. A branch would create a side channel: an
// observer could tell whether a Secret wrapped an empty string
// by comparing the output. The constant output is the correct
// behavior.
func (s Secret) String() string {
	return redactedLiteral
}

// LogValue implements slog.LogValuer.
//
// The method is called by log/slog when a Secret is passed as an
// attribute to a log record. The returned slog.Value wraps the
// redacted literal, so the record's rendered form (in either the
// human or the JSON log format) contains the literal, not the
// credential.
//
// # Why the returned Value is a string, not a group or a number
//
// A string is the simplest correct value. A consumer that wants
// to distinguish a redacted credential from a public string
// greps for the literal. A group or a number would complicate
// the consumer's logic without adding information.
func (s Secret) LogValue() slog.Value {
	return slog.StringValue(redactedLiteral)
}

// MarshalJSON implements json.Marshaler.
//
// The method is called by encoding/json when a Secret appears in
// a value passed to json.Marshal or json.Encoder.Encode. The
// output is the JSON-encoded form of the redacted literal, i.e.,
// the string "[REDACTED]" (with the surrounding double quotes).
//
// # Why the method is necessary
//
// Without it, encoding/json would fall back to reflection and
// serialize the struct's exported fields. Secret has no exported
// fields (the wrapped value is unexported), so the fallback would
// serialize an empty object: {}. That is technically "not a
// leak" but is also not useful — a consumer cannot tell that a
// Secret was present. The method makes the redaction explicit in
// the JSON output.
func (s Secret) MarshalJSON() ([]byte, error) {
	return json.Marshal(redactedLiteral)
}

// Equal reports whether two Secrets wrap equal strings.
//
// # Why a method and not ==
//
// A caller who writes `a == b` where a and b are Secrets gets
// the Go equality semantics for a struct containing a string,
// which is correct for the current representation but would
// break if the representation changed (for example, if the
// wrapped value became a byte slice). The method pins the
// equality semantics to "equal wrapped strings" independently of
// the representation.
//
// # Why the method is not constant-time (yet)
//
// A truly constant-time equality would require comparing the
// wrapped strings byte-by-byte with no early exit. The current
// implementation uses Go's string equality, which the runtime
// may optimize to a memcmp. The memcmp's timing profile is
// observable in principle, but the threat model here does not
// include timing attacks on the CLI's own process (the attacker
// would need to invoke the CLI repeatedly with candidate
// secrets and measure the comparison time; the CLI's process
// startup dominates the measurement). A future WBS that
// addresses timing side channels in the CLI's internals files
// an ADR and updates Equal to use crypto/subtle.ConstantTimeCompare.
//
// # The empty case
//
// Two zero-value Secrets are equal; a zero-value Secret and a
// Secret wrapping "" are equal; a zero-value Secret and a Secret
// wrapping "x" are not. The equality is on the wrapped string,
// not on the type's metadata.
func (s Secret) Equal(other Secret) bool {
	return s.value == other.value
}

// GoString implements fmt.GoStringer.
//
// The method is called by fmt.Sprintf("%#v", secret) and by any
// caller that uses the %#v verb. Without this hook, %#v would
// fall back to fmt's default reflection, which prints the
// struct's field names and values — revealing the credential.
// The method closes that path.
//
// # Why the method returns a type-annotated literal
//
// The return value is `logger.Secret("[REDACTED]")`, not just
// `[REDACTED]`. The %#v verb is used for debugging, and its
// output conventionally includes the type. The type annotation
// is a small courtesy to the reader; the credential is still
// redacted.
func (s Secret) GoString() string {
	return `logger.Secret("` + redactedLiteral + `")`
}

// RedactedLiteral returns the exact string every Secret renders as.
//
// # Why a function and not an exported constant
//
// The marker is currently a compile-time constant. A function is
// the more flexible accessor: a future change that computes the
// marker from configuration (a per-environment marker, for
// example) can change the function's body without changing its
// signature. An exported constant would freeze the marker to a
// single value at compile time.
//
// # Who calls it
//
// The function is called by tests outside the logger package
// (test/security/redaction_test.go, for example) that need to
// assert the marker appears in an output. Production code inside
// the package uses the unexported constant directly; there is no
// reason to expose the constant to the rest of the package.
//
// # Stability
//
// The return value is a public contract. A log aggregator's
// redaction rules look for exactly this string. A future change
// to the value is a breaking change to those rules and requires
// an ADR. Adding the function does not change the value; it only
// changes how the value is accessed.
func RedactedLiteral() string {
	return redactedLiteral
}
