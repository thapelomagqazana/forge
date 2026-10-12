// Package forgeerr — user-facing error format.
//
// This file defines Format, the single renderer that turns an
// error into the byte-exact string that a user sees on stderr.
// The format is frozen in WBS 10.4.1, extended in WBS 10.4.3
// (redaction) and WBS 10.4.4 (verbose mode), and enforced by
// golden-file tests. Any change to the format requires an ADR.
//
// # The non-verbose format
//
// The non-verbose format is the one frozen in WBS 10.4.1:
//
//	Error: <message>
//	Code:  <code>
//	       <context-key-1>: <value-1>
//	       <context-key-2>: <value-2>
//
//	Suggestion:
//	  <remediation>
//
// Byte-level rules (from WBS 10.4.1):
//
//	F1  First line: "Error: " + message.
//	F2  Second line: "Code:  " + code (two spaces after the colon).
//	F3  Context lines: 7 spaces + "<key>: <value>".
//	F4  Blank line before the Suggestion block.
//	F5  "Suggestion:" label on its own line.
//	F6  Suggestion content: 2-space indent, ends with a period.
//	F7  No trailing whitespace on any line.
//	F8  No colour, no ANSI escapes.
//	F9  Every line ends with "\n".
//	F10 Output ends with exactly one "\n" (no blank line at end).
//	F11 Context keys are sorted alphabetically.
//	F12 The Code prefix identifies the category (informational).
//
// # The verbose format (WBS 10.4.4)
//
// When Options.Verbose is true, Format renders three additional
// blocks and relaxes one rule:
//
//  1. A "Cause:" block follows the context lines. It contains
//     the cause chain, one line per cause, each indented two
//     spaces.
//
//  2. A "Stack:" block follows the Cause block, if Options.Stack
//     is non-empty. It contains the stack frames, one line per
//     frame, each indented two spaces.
//
//  3. The internal_-prefixed context keys are rendered. The
//     non-verbose mode filters them out.
//
//  4. Home-directory redaction is relaxed: a context value that
//     is a path under the user's home directory is rendered with
//     the full path, not the "~"-redacted form.
//
// Secret redaction is unchanged in both modes. The rule
// "Secrets are never shown" (WBS 10.4.3) is absolute.
//
// The verbose blocks are inserted between the context lines and
// the Suggestion block, so the output reads:
//
//	Error: <message>
//	Code:  <code>
//	       <context-key-1>: <value-1>
//
//	Cause:
//	  <cause-1>
//	  <cause-2>
//
//	Stack:
//	  <frame-1>
//	  <frame-2>
//
//	Suggestion:
//	  <remediation>
//
// # Redaction
//
// Both modes apply the redaction policy frozen in WBS 10.4.3.
// Every context value is passed through redact (which withholds
// secret-typed values) before it is written. The verbose mode
// relaxes the path-redaction step (redactPath) but not the
// secret-redaction step. See redact.go for the two functions.
//
// The message, the code, the category, and the remediation are
// not redacted in either mode. The constructor authors them; it
// is responsible for not placing secrets in them. See the
// WithContext doc comment for the caller-side rule on secrets.
//
// # Fallback
//
// The format applies only to *Error values. When Format is given
// an error that is not a *Error and does not wrap one, it falls
// back to a single-line rendering: "Error: <msg>\n". The
// fallback does not distinguish verbose from non-verbose; it has
// no structured fields to add.
//
// # Determinism
//
// The format is deterministic given the same *Error, Options,
// and (for verbose mode) Stack. The Stack is a slice of strings
// provided by the caller; the caller is responsible for its
// determinism.
//
// Only the Go standard library is used (see docs/dependency-policy.md).
package forgeerr

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Options controls Format's behaviour.
//
// The zero value produces the frozen non-verbose format. A caller
// that passes Options{} sees exactly the string WBS 10.4.1 and
// WBS 10.4.3 produce.
//
// The struct is extended by adding fields, not by changing the
// signature of Format. This is the same discipline as
// Dependencies in the CLI: a value passed through a stable
// boundary, extended by adding fields.
type Options struct {
	// NoColor disables colour in the output.
	//
	// The Phase 2 format never emits colour regardless of this
	// field: rule F8 forbids it unconditionally. The field exists
	// as a placeholder for a future phase that may emit colour
	// when stderr is a TTY. Do not read this field until that
	// phase lands.
	NoColor bool

	// Redact replaces context values that match a secret pattern
	// with "<redacted>".
	//
	// The Phase 2 format always performs the redaction defined in
	// WBS 10.4.3: it is not conditional on this field. The field
	// exists as a placeholder for a future phase that may make
	// the redaction policy configurable. Do not read this field
	// until that phase lands.
	Redact bool

	// Verbose enables the verbose mode frozen in WBS 10.4.4.
	//
	// In verbose mode, Format renders three additional blocks:
	// the cause chain, the stack trace (if a Stack is provided),
	// and the internal_-prefixed context keys. It also relaxes
	// home-directory redaction: full paths are shown for
	// debugging.
	//
	// Secret redaction is unchanged. The rule "Secrets are never
	// shown" is absolute.
	//
	// The field is set by the CLI's error path when the user
	// passes --verbose. It is not set in the default invocation.
	Verbose bool

	// Stack is the stack trace to render in verbose mode. It is a
	// slice of pre-formatted frame strings, one per line. The
	// caller is responsible for the format of each string and for
	// the slice's determinism.
	//
	// The field is ignored when Verbose is false. It is also
	// ignored when Verbose is true but Stack is empty; in that
	// case, no Stack block is rendered.
	//
	// The format renders the frames as given: it does not parse
	// them, does not reorder them, and does not truncate them. A
	// caller that wants to limit the number of frames does so
	// before passing them in.
	Stack []string
}

// Format renders err as the user-facing error string.
//
// The returned string is byte-exact: it ends with exactly one
// "\n" (rule F10), contains no trailing whitespace (rule F7),
// and contains no ANSI escapes (rule F8). Format never panics:
// given any error, including nil, it returns a valid string.
//
// # The mode
//
// The mode is selected by opts.Verbose. The zero value produces
// the frozen non-verbose format. Setting Verbose to true produces
// the verbose format described in the package doc comment.
//
// # Extraction
//
// Format extracts the *Error from err's chain using errors.As.
// If err is a *Error, the *Error itself is rendered. If err wraps
// a *Error (for example, via fmt.Errorf("%w", ...)), the inner
// *Error is rendered. If err is nil, Format returns the empty
// string.
//
// If err is non-nil and no *Error is present in its chain, Format
// falls back to a minimal rendering: "Error: <msg>\n". The
// fallback has no Code line and no context; it exists only so
// that the CLI's error path does not produce a bare string. Every
// error path in Forge should produce a *Error, so the fallback
// should never be hit in practice; its existence is a safety net,
// not a feature.
//
// # Redaction
//
// Every context value is passed through redact (secret-typed
// values are replaced with the marker) and, in non-verbose mode,
// redactPath (a home-directory prefix is replaced with "~"). The
// transformations are applied in that order. See redact.go for
// the policy and the rules.
//
// The message, the code, the category, and the remediation are
// not redacted. The constructor authors them; it is responsible
// for not placing secrets in them.
func Format(err error, opts Options) string {
	if err == nil {
		return ""
	}

	var e *Error
	if !errors.As(err, &e) {
		// The error is not a *Error and does not wrap one. Render
		// a minimal fallback that preserves the shape of the
		// frozen format without a Code line.
		//
		// The message is err.Error() verbatim: the caller is
		// responsible for ensuring it is single-line and free of
		// trailing whitespace. If it is not, the fallback will
		// produce output that violates F7 or F9. The CLI's error
		// path validates its inputs; Format does not, because
		// Format is a pure renderer and validation is the
		// constructor's concern.
		//
		// The fallback does not apply redaction. A plain error
		// carries no context map; there is nothing to redact. If
		// a caller's plain error happens to embed a secret in its
		// message, redaction cannot recover it: the message is
		// not a key-value pair, and redacting prose is not a
		// well-defined operation.
		//
		// The fallback's output is not subject to the golden-file
		// tests. It is tested separately, in TestFormat_Fallback.
		return "Error: " + err.Error() + "\n"
	}

	var b strings.Builder

	// F1: first line.
	b.WriteString("Error: ")
	b.WriteString(e.message)
	b.WriteByte('\n')

	// F2: second line. Two spaces after "Code:".
	b.WriteString("Code:  ")
	b.WriteString(e.code)
	b.WriteByte('\n')

	// F3, F11: context lines, sorted alphabetically by key.
	// Each line is 7 spaces, the key, ": ", and the value.
	//
	// The context map is nil-safe: an *Error with no context has
	// e.context == nil, and the loop below is a no-op for nil.
	// The map's iteration order is randomized by Go, so we sort
	// the keys to make the output deterministic.
	//
	// WBS 10.4.3: each value is redacted before it is written.
	// The pipeline is:
	//
	//   1. redact(key, value)   — replaces a secret-typed value
	//                             with the marker.
	//   2. redactPathFor(...)   — replaces a home-directory
	//                             prefix with "~" (non-verbose)
	//                             or leaves the path unchanged
	//                             (verbose).
	//
	// WBS 10.4.4: the verbose mode renders the internal_-
	// prefixed context keys, which the non-verbose mode filters
	// out. The filter is applied before the sort, so that the
	// sort operates on the visible set.
	//
	// The log formatter (log.go) applies the same redact step,
	// so the two outputs cannot diverge on what is redacted.
	// The log formatter does not implement the verbose mode; its
	// rendering is the same in both modes (WBS 10.4.2).
	keys := sortedKeys(visibleContext(e.context, opts.Verbose))
	for _, k := range keys {
		v := redact(k, e.context[k])
		v = redactPathFor(v, opts.Verbose)

		b.WriteString("       ") // 7 spaces (F3)
		b.WriteString(k)
		if v != "" {
			b.WriteString(": ")
			b.WriteString(v)
		} else {
			// Empty value: emit "key:" with no trailing space (F7).
			//
			// Rule F3 specifies "7 spaces + <key>: <value>". When
			// the value is empty, writing the trailing space would
			// violate rule F7 (no trailing whitespace). Omitting
			// the space is the only shape that satisfies both
			// rules: the line still names the key, and it ends with
			// the colon, which is not whitespace.
			b.WriteString(":")
		}
		b.WriteByte('\n')
	}

	// WBS 10.4.4: the verbose mode renders the cause chain and
	// the stack (if one was provided). The two blocks are inserted
	// between the context lines and the Suggestion block.
	if opts.Verbose {
		renderCauseBlock(&b, e.cause)
		renderStackBlock(&b, opts.Stack)
	}

	// F4, F5, F6: Suggestion block.
	if e.remediation != "" {
		b.WriteByte('\n') // F4: blank line before the block
		b.WriteString("Suggestion:\n")
		b.WriteString("  ") // F6: 2-space indent
		b.WriteString(e.remediation)
		b.WriteByte('\n')
	}

	// F10: the output already ends with exactly one "\n",
	// because every line above writes its own newline and no
	// trailing blank line is added. No trimming is required; the
	// invariant is maintained by construction.
	//
	// NoColor and Redact are accepted but not read; see their
	// doc comments for the rationale.
	_, _ = opts.NoColor, opts.Redact

	return b.String()
}

// visibleContext returns the subset of m that should be rendered
// in the given mode. In non-verbose mode, keys with the
// "internal_" prefix are filtered out. In verbose mode, all keys
// are returned.
//
// The function returns the same map when no keys are filtered;
// the caller's sort operates on the returned map. A new map is
// allocated only when at least one key is filtered, which is the
// case for every *Error that has internal context and is rendered
// in non-verbose mode.
//
// The function does not mutate its argument.
func visibleContext(m map[string]string, verbose bool) map[string]string {
	if verbose || len(m) == 0 {
		return m
	}

	// Fast path: check whether any key has the prefix. If none
	// does, return the original map without allocating.
	hasInternal := false
	for k := range m {
		if strings.HasPrefix(k, "internal_") {
			hasInternal = true
			break
		}
	}
	if !hasInternal {
		return m
	}

	filtered := make(map[string]string, len(m))
	for k, v := range m {
		if !strings.HasPrefix(k, "internal_") {
			filtered[k] = v
		}
	}
	return filtered
}

// renderCauseBlock writes the "Cause:" block for the given cause.
//
// The block is written only if cause is non-nil. The block is:
//
//	\n
//	Cause:\n
//	  <cause-1>\n
//	  <cause-2>\n
//
// The block ends with a newline, so the caller can append the
// next block (or the Suggestion block) directly.
//
// # The chain
//
// The block renders the full cause chain reachable via
// errors.Unwrap. Each level is rendered on its own line, indented
// two spaces. The chain terminates when Unwrap returns nil.
//
// The function guards against cycles by capping the number of
// levels at maxCauseDepth. A cause chain longer than the cap is
// truncated; a diagnostic line is appended to the block.
func renderCauseBlock(b *strings.Builder, cause error) {
	if cause == nil {
		return
	}

	b.WriteByte('\n')
	b.WriteString("Cause:\n")

	depth := 0
	for cause != nil && depth < maxCauseDepth {
		b.WriteString("  ")
		b.WriteString(cause.Error())
		b.WriteByte('\n')

		cause = errors.Unwrap(cause)
		depth++
	}

	if cause != nil {
		// The chain is longer than maxCauseDepth. Add a
		// diagnostic line so the reader knows the chain is
		// truncated rather than complete.
		fmt.Fprintf(b, "  ... (cause chain truncated at %d levels)\n", maxCauseDepth)
	}
}

// renderStackBlock writes the "Stack:" block for the given
// frames.
//
// The block is written only if frames is non-empty. The block is:
//
//	\n
//	Stack:\n
//	  <frame-1>\n
//	  <frame-2>\n
//
// The block ends with a newline, so the caller can append the
// Suggestion block directly.
//
// # The frames
//
// Each frame is written as given, indented two spaces. The
// function does not parse the frames, does not reorder them, and
// does not truncate them. A caller that wants to limit the
// number of frames does so before passing them in.
func renderStackBlock(b *strings.Builder, frames []string) {
	if len(frames) == 0 {
		return
	}

	b.WriteByte('\n')
	b.WriteString("Stack:\n")
	for _, frame := range frames {
		b.WriteString("  ")
		b.WriteString(frame)
		b.WriteByte('\n')
	}
}

// maxCauseDepth is the maximum number of cause levels rendered
// by renderCauseBlock. The cap protects against a cause chain
// that contains a cycle (an error whose Unwrap returns itself or
// an ancestor). The value is a constant so that the truncation
// message and the loop bound share one definition.
//
// A future ADR may raise the cap or make it configurable. The
// value is deliberately generous: a cause chain longer than
// thirty-two levels is almost always a cycle, not a legitimate
// chain.
const maxCauseDepth = 32

// sortedKeys returns the keys of m in lexicographic order.
//
// Returns an empty slice if m is nil or empty. The caller may
// range over the result safely without a length check.
//
// The implementation allocates a slice of keys and sorts it. For
// an error with N context keys, this is O(N log N) time and O(N)
// space. N is expected to be small (typically 0–3), so the cost
// is negligible. If a future error carries hundreds of keys, the
// sort could be replaced by a different strategy; the function's
// signature would not change.
func sortedKeys(m map[string]string) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// compile-time assertion: Format has the frozen signature.
var _ func(error, Options) string = Format
