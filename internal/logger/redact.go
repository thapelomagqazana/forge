// Package logger — pattern-based redaction middleware.
//
// This file implements layer 2 of the four-layer redaction model.
// See docs/security/logging-redaction.md §3 for the full model.
//
// # What the middleware does
//
// The middleware is a pure function that takes a Record and
// returns a Record whose Message and Attrs have had credential-like
// substrings replaced with [REDACTED]. It runs inside the logger's
// emission path, so every record the logger emits passes through
// it before reaching the sink.
//
// # Why a middleware in addition to the Secret type
//
// The Secret type (layer 1) requires every caller to wrap its
// credential. A caller that forgets — a function that reads a
// password into a plain string and then logs it — bypasses layer
// 1. The middleware (layer 2) catches that leak: it scans the
// record's text for patterns that look like credentials and
// redacts them, regardless of the caller's discipline.
//
// The two layers are independent: layer 1 is precise (it redacts
// exactly the value the caller marked); layer 2 is a safety net
// (it redacts anything that looks like a credential). A future
// change that loosens layer 1 does not weaken layer 2.
//
// # The pattern set
//
// The middleware recognizes these patterns. Each is documented
// with the false-positive policy: what legitimate strings might
// match, and why the match is acceptable.
//
//   - `password=<value>` — a password assigned in a key-value
//     context. The regex matches `password` case-insensitively
//     and captures the value up to whitespace or a delimiter.
//     False-positive policy: a user's data containing the literal
//     string "password=" followed by non-whitespace would be
//     redacted; the string is credential-shaped, and redacting
//     it is the safe choice.
//
//   - `token=<value>`, `secret=<value>`, `api_key=<value>`,
//     `apikey=<value>` — same shape as `password`. The alternative
//     spellings (`api_key` and `apikey`) are both recognized.
//
//   - `Authorization: <scheme> <value>` — an HTTP Authorization
//     header. The regex matches the header name, the scheme
//     (Bearer, Basic, etc.), and the credential. Only the
//     credential is replaced.
//
//   - `Bearer <value>` — the Bearer scheme on its own, without
//     the Authorization header name. Common in curl commands and
//     in HTTP client logs.
//
//   - `-----BEGIN <TYPE> PRIVATE KEY-----` — a PEM header. The
//     middleware redacts the entire PEM block (from BEGIN to
//     END) because the header alone is not enough; the payload
//     is the credential.
//
//   - High-entropy strings — a string that looks like a randomly
//     generated secret. The middleware computes a simple entropy
//     score (Shannon entropy per character) and redacts strings
//     whose score exceeds a threshold. False-positive policy: a
//     UUID, a SHA-256 hash, a base64-encoded non-secret might
//     exceed the threshold; the middleware accepts the
//     over-redaction because under-redaction is worse.
//
// # What the middleware does NOT do
//
// The middleware does not redact the Secret type's own rendering.
// A Secret's String() already returns [REDACTED]; the middleware
// sees the marker and leaves it. The two layers are disjoint in
// their effect: layer 1 redacts typed values; layer 2 redacts
// untyped text.
//
// The middleware does not perform config-aware redaction. That is
// layer 3 (internal/config/redact.go), which runs at config-load
// time and wraps sensitive fields in Secret.
//
// The middleware does not perform authorization or access
// control. It redacts credential-shaped text; it does not decide
// who may see the redacted output.
//
// # Performance
//
// The middleware runs on every log emission. The pattern set is
// small (a handful of regexes plus an entropy check) and the
// strings are short (log messages and field values). The cost is
// proportional to the length of the record's text; a record with
// a kilobyte of text costs a few microseconds. A benchmark
// pins the cost; a future WBS that grows the pattern set must
// keep the cost bounded.
package logger

import (
	"math"
	"regexp"
	"strings"
)

// The pattern set. Each regex is compiled once at package
// initialization; a compile failure is a programming error and
// panics at first use.
var (
	// credentialAssign matches `key=value` or `key: value` where
	// key is a credential-shaped name. The regex captures the
	// value in group 2. The key's case is preserved; the match
	// is case-insensitive.
	//
	// The list of keys is closed: adding a key is a deliberate
	// change to the redaction policy and requires an ADR. A
	// user-supplied key ("my_secret_token=") is not in the list
	// and is caught by the entropy check if its value looks
	// random; a user-supplied key with a non-random value is
	// not caught, which is the documented limitation.
	credentialAssign = regexp.MustCompile(
		`(?i)\b(password|passwd|pwd|token|secret|api[_-]?key|apikey|auth[_-]?token|access[_-]?token|refresh[_-]?token|client[_-]?secret)\s*[:=]\s*(\S+)`)

	// authorizationHeader matches an HTTP Authorization header.
	// Group 1 is the scheme (Bearer, Basic, etc.); group 2 is
	// the credential.
	authorizationHeader = regexp.MustCompile(
		`(?i)\bauthorization\s*:\s*(\w+)\s+(\S+)`)

	// bearerToken matches a bare Bearer scheme followed by a
	// credential. It is a subset of authorizationHeader but
	// appears without the header name in curl commands and in
	// HTTP client logs.
	bearerToken = regexp.MustCompile(
		`(?i)\bbearer\s+([A-Za-z0-9._~+/=-]{8,})`)

	// pemBlock matches a PEM block from BEGIN to END. The match
	// is greedy across lines; the caller passes a multi-line
	// string when the block is embedded in a larger message.
	pemBlock = regexp.MustCompile(
		`(?s)-----BEGIN [A-Z0-9 ]+-----.+?-----END [A-Z0-9 ]+-----`)
)

// entropyThreshold is the minimum Shannon entropy per character
// for a string to be considered a high-entropy secret.
//
// # Why 3.5 bits per character
//
// The theoretical maximum for a byte-based alphabet is 8 bits
// per character (uniform over 256 symbols); for a base64
// alphabet it is 6 bits per character; for a hex alphabet it is
// 4 bits per character. Legitimate English text scores around
// 4.0–4.5 bits per character (the redundancy of natural
// language). Base64-encoded secrets score close to 6. A
// threshold of 3.5 bits per character catches hex-encoded
// secrets (4.0) and base64-encoded secrets (5.5–6.0) while
// allowing legitimate English text.
//
// # False-positive policy
//
// The threshold is a heuristic, not a proof. A short English
// word with unusual letter distribution might exceed 3.5
// (rare). A UUID or a SHA-256 hash will exceed 3.5 (common in
// logs). The middleware accepts over-redaction of hashes and
// UUIDs because under-redaction of secrets is worse. A future
// WBS that refines the heuristic (for example, by excluding
// strings that match a UUID or hex-hash pattern) files an ADR
// and updates the threshold.
const entropyThreshold = 3.5

// minEntropyLen is the minimum length for the entropy check to
// run. Strings shorter than this are skipped because their
// entropy is dominated by length, not by randomness.
//
// # Why 16
//
// A 16-character string is the shortest that a typical
// credential can be and still carry enough entropy to be
// distinguishable from a word. Shorter strings (a 4-character
// PIN, an 8-character password) would be redacted by the
// pattern layer if they appear in a credential assignment; the
// entropy layer is for strings that appear without a key, and
// those are typically longer (a JWT, a PEM body, a base64
// secret).
const minEntropyLen = 16

// RedactRecord returns a copy of r with credential-shaped
// substrings in Message and in string Attrs replaced with
// [REDACTED].
//
// # What is redacted
//
//   - r.Message: scanned for every pattern in the set and for
//     high-entropy tokens; matching substrings are replaced.
//
//   - r.Attrs: each string value is scanned by the same rules;
//     non-string values are left untouched.
//
//   - Nested maps and slices in Attrs: not scanned. The logger
//     flattens attrs at construction time; the flattener's
//     behavior is documented in logger.go. A future WBS that
//     adds nested attrs extends this function.
//
// # What is preserved
//
// The Record's Time, Level, Component, and CorrelationID are
// unchanged. The redaction is a text transformation; it does not
// affect the record's metadata.
//
// # The copy
//
// The function returns a copy of r; the caller's record is not
// mutated. This is the correct behavior for a middleware that
// sits in the emission path: the record is shared with any
// caller that holds a reference (a test that asserts on the
// pre-redaction record, for example), and mutating it in place
// would surprise them.
//
// # Nil safety
//
// A nil Attrs map is fine; the function returns a Record with a
// nil Attrs map. A nil r.Message is fine; the function returns
// an empty string.
//
// # Errors
//
// The function cannot fail. Its inputs are strings and maps; its
// outputs are strings and maps. No I/O, no allocation beyond
// what the replacements require, no panics.
func RedactRecord(r Record) Record {
	out := r
	out.Message = redactString(r.Message)
	if r.Attrs != nil {
		out.Attrs = make(map[string]any, len(r.Attrs))
		for k, v := range r.Attrs {
			if s, ok := v.(string); ok {
				out.Attrs[k] = redactString(s)
			} else {
				out.Attrs[k] = v
			}
		}
	}
	return out
}

// redactString applies every pattern and the entropy check to s.
//
// # The order of operations
//
// The pattern-based rules run first; the entropy check runs
// second on the residual text (after pattern-based redaction).
// The order matters: a pattern-based rule is precise (it names
// the credential's key), while the entropy check is heuristic.
// Running the precise rules first means the entropy check sees
// less text and produces fewer false positives.
//
// # The [REDACTED] marker is idempotent
//
// If the input contains the marker already (because the caller
// passed a Secret's String() through, or because a previous
// redaction pass ran), the marker passes through unchanged. The
// entropy check skips it: the marker's entropy is below the
// threshold.
//
// # Nil and empty input
//
// The function returns "" for "". It does not panic.
func redactString(s string) string {
	if s == "" {
		return s
	}

	// Pattern 1: credential assignments.
	s = credentialAssign.ReplaceAllStringFunc(s, redactAssignment)

	// Pattern 2: Authorization headers.
	s = authorizationHeader.ReplaceAllStringFunc(s, redactAuthorization)

	// Pattern 3: bare Bearer tokens.
	s = bearerToken.ReplaceAllStringFunc(s, redactBearer)

	// Pattern 4: PEM blocks. The match is multi-line; the
	// replacement collapses the block to a single marker so the
	// surrounding text remains readable.
	s = pemBlock.ReplaceAllString(s, redactedLiteral)

	// Pattern 5: high-entropy tokens.
	s = redactHighEntropyTokens(s)

	return s
}

// redactAssignment replaces the value in a key=value match with
// the marker. The key is preserved so the log reader can see
// which field was redacted.
func redactAssignment(match string) string {
	// The match's shape is `key=value`. Split on the first
	// delimiter (`=` or `:`); keep the key and the delimiter;
	// replace the value.
	for i, r := range match {
		if r == '=' || r == ':' {
			return match[:i+1] + redactedLiteral
		}
	}
	return redactedLiteral
}

// redactAuthorization replaces the credential in an
// `Authorization: <scheme> <value>` match with the marker. The
// scheme is preserved so the reader can see that an
// Authorization header was present.
func redactAuthorization(match string) string {
	// The match's shape is `Authorization: Bearer <value>`. Split
	// on the first space after the scheme; replace the value.
	idx := strings.LastIndexAny(match, " \t")
	if idx < 0 {
		return redactedLiteral
	}
	return match[:idx+1] + redactedLiteral
}

// redactBearer replaces the credential in a `Bearer <value>`
// match with the marker. The scheme word is preserved.
func redactBearer(match string) string {
	idx := strings.IndexAny(match, " \t")
	if idx < 0 {
		return redactedLiteral
	}
	return match[:idx+1] + redactedLiteral
}

// redactHighEntropyTokens scans s for whitespace-delimited tokens
// whose Shannon entropy exceeds entropyThreshold and whose length
// is at least minEntropyLen, and replaces each such token with
// the marker.
//
// # The tokenization
//
// The function splits on whitespace and on the common punctuation
// that terminates a token in a log line: comma, semicolon, and
// the closing bracket of a quoted field. It does not split on
// hyphens or underscores, because those are common inside
// credential-shaped tokens.
//
// # The redaction
//
// A matching token is replaced by the marker. Surrounding
// whitespace and punctuation are preserved.
//
// # Why the function is a separate pass
//
// The entropy check is the most expensive part of the middleware
// (it computes Shannon entropy over each candidate token). By
// running it after the cheaper pattern-based rules, the function
// sees less text and does less work. The optimization is small
// but free: the pattern-based rules have already redacted the
// most obvious credentials.
func redactHighEntropyTokens(s string) string {
	// Split on whitespace and the punctuation that commonly
	// terminates a token in a log line. The set is deliberately
	// small: over-splitting reduces the length of candidate
	// tokens below minEntropyLen and misses secrets.
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == ',' || r == ';'
	})
	if len(fields) == 0 {
		return s
	}

	var b strings.Builder
	b.Grow(len(s))
	prevEnd := 0
	for _, tok := range fields {
		// Find the token in s starting from prevEnd. The
		// FieldsFunc split loses the original positions, so we
		// re-find each token. Tokens are unique enough in a log
		// line that Index from prevEnd is correct; a pathological
		// case (repeated tokens) would find the first occurrence,
		// which is still within the intended span.
		idx := strings.Index(s[prevEnd:], tok)
		if idx < 0 {
			// Should not happen; fall through safely.
			b.WriteString(s[prevEnd:])
			return b.String()
		}
		idx += prevEnd

		// Write the separator between prevEnd and idx.
		b.WriteString(s[prevEnd:idx])

		// Write the token, redacted if high-entropy.
		if len(tok) >= minEntropyLen && shannonEntropy(tok) >= entropyThreshold {
			b.WriteString(redactedLiteral)
		} else {
			b.WriteString(tok)
		}
		prevEnd = idx + len(tok)
	}
	b.WriteString(s[prevEnd:])
	return b.String()
}

// shannonEntropy returns the Shannon entropy of s in bits per
// character.
//
// # The formula
//
//	H(s) = -Σ p(c) log2 p(c)
//
// where p(c) is the frequency of character c in s. The sum is
// over the distinct characters in s.
//
// # Why the function does not normalize by length
//
// Shannon entropy is a per-character measure by construction:
// the sum is over probabilities, which sum to 1. A longer string
// with the same character distribution has the same entropy.
// The length filter (minEntropyLen) is what prevents short
// strings from being redacted by the entropy check.
//
// # Performance
//
// The function iterates over s once to count characters and once
// over the distinct characters (at most 256). The cost is
// O(len(s)), which is the same as any scan of the string.
func shannonEntropy(s string) float64 {
	if s == "" {
		return 0
	}
	var counts [256]int
	for i := 0; i < len(s); i++ {
		counts[s[i]]++
	}
	n := float64(len(s))
	var h float64
	for _, c := range counts {
		if c == 0 {
			continue
		}
		p := float64(c) / n
		h -= p * math.Log2(p)
	}
	return h
}
