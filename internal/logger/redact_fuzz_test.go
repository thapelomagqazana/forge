package logger

import (
	"strings"
	"testing"
)

// FuzzRedactString asserts two invariants for every input:
//
//  1. The function does not panic.
//  2. The function's output does not contain any of the
//     credential-shaped substrings that the input contained.
//
// # The invariant set
//
// The fuzzer seeds the corpus with strings that contain the
// documented credential patterns. For each input, the fuzz
// function asserts that every credential-shaped token in the
// input is absent from the output. The assertion is on the
// *tokens* the corpus injects, not on arbitrary substrings,
// because arbitrary substrings would false-positive on the
// redaction marker itself.
//
// # What the fuzzer does not assert
//
// The fuzzer does not assert that the output equals a specific
// value; the redaction is not a canonical transformation. It
// does not assert that no over-redaction occurred; the false-
// positive policy permits over-redaction.
//
// # Running
//
// By default, the Go fuzzer runs for a short time. To run the
// WBS's ≥ 1M iterations:
//
//	go test -run=^$ -fuzz=FuzzRedactString -fuzztime=1m ./internal/logger
//
// The one-minute duration is a proxy for 1M iterations on a
// typical developer machine; the exact count depends on the
// machine's speed.
func FuzzRedactString(f *testing.F) {
	// Seed the corpus.
	seeds := []string{
		"password=hunter2",
		"password: hunter2",
		"token=abcdef1234567890",
		"secret=s3cr3t",
		"api_key=abcdef",
		"apikey=abcdef",
		"Authorization: Bearer abcdef1234567890",
		"authorization: basic dXNlcjpwYXNz",
		"Bearer eyJhbGciOiJIUzI1NiJ9",
		"-----BEGIN PRIVATE KEY-----\nMIIE\n-----END PRIVATE KEY-----",
		"Zm9vYmFyYmF6cXV4Zm9vYmFyYmF6cXV4Zm9vYmFyYmF6",
		"",
		"a",
		" ",
		"\n",
		"password=",
		"Bearer ",
		"-----BEGIN ",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// Invariant 1: no panic. If the function panics, the
		// test fails with the panic's stack.
		out := redactString(input)

		// Invariant 2: no credential-shaped token survives.
		// The fuzzer does not know which substrings are
		// credentials, so it asserts a weaker property: the
		// output does not contain any of the *known* credential
		// patterns. A random input that happens to contain one
		// of those patterns must have it redacted.
		for _, leak := range knownLeakTokens(input) {
			if strings.Contains(out, leak) {
				t.Fatalf("redactString leaked %q:\n"+
					"  input:  %q\n  output: %q",
					leak, input, out)
			}
		}
	})
}

// knownLeakTokens returns the credential-shaped substrings that
// the fuzz function will check for. The set is derived from the
// input by the same regexes the middleware uses; a token that
// matches a documented pattern is a token the middleware must
// redact.
//
// # Why the fuzzer re-derives the tokens
//
// The fuzzer cannot ask the middleware "which tokens did you
// redact?" because the middleware is a text transformation, not
// a parser. The fuzzer instead computes the tokens itself, using
// the same regexes, and checks that none survive. The two
// computations must agree by construction; a disagreement is the
// bug the fuzzer finds.
func knownLeakTokens(input string) []string {
	var out []string
	for _, m := range credentialAssign.FindAllStringSubmatch(input, -1) {
		out = append(out, m[2])
	}
	for _, m := range authorizationHeader.FindAllStringSubmatch(input, -1) {
		out = append(out, m[2])
	}
	for _, m := range bearerToken.FindAllStringSubmatch(input, -1) {
		out = append(out, m[1])
	}
	return out
}
