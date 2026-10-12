// Package forgeerr — redaction policy.
//
// This file implements the redaction policy frozen in WBS 10.4.3.
// The policy is a single choke point through which every context
// value and every path passes before it reaches a user or a log
// consumer. The two consumers are:
//
//   - Format (format.go), which renders the user-facing string
//     written to stderr.
//   - LogValue (log.go), which produces the structured log entry
//     written to whatever destination the slog handler uses.
//
// Both callers invoke the same functions, so the two outputs
// cannot diverge on what is redacted. This is the property the
// frozen policy requires: a secret is redacted regardless of
// whether it would have been rendered to a user or to a log.
//
// # What is redacted
//
// The policy is defined in WBS 10.4.3 and summarised here:
//
//	Data type                  User output           Log output
//	-------------------------  --------------------  --------------------
//	Error message              Shown as-is           Shown as-is
//	Error code                 Shown                 Shown
//	Context value (secret)     <redacted>            <redacted>
//	Context value (home path)  ~/...                 ~/...
//	Context value (other)      Shown as-is           Shown as-is
//	Cause                      Never shown           Shown as a field
//	Stack trace                Never shown           Never shown (Phase 2)
//	Internal paths             Never shown           Shown as <path>
//	Home directory             Redacted as ~         Redacted as ~
//	Secrets (any form)         Never shown           Never shown
//	Environment variable value Never shown           Never shown
//
// The four rows that this file is responsible for are the
// context-value rows. The cause and stack-trace rows are the
// formatters' concerns; this file provides the predicates and
// the value transformations they consult.
//
// # Why a separate file
//
// The policy is consulted by two callers with different
// presentations. Keeping it in one file means:
//
//  1. A change to the secret key list is a change in one place.
//  2. A change to the path-redaction rule is a change in one
//     place.
//  3. The two callers cannot drift: they call the same
//     functions.
//
// # Redaction is not encryption
//
// <redacted> is a marker that a value was withheld. It is not a
// reversible transformation. A caller that needs to inspect the
// original value must not route it through the redaction layer.
// The layer's purpose is to prevent leaks; it does not preserve
// the leaked value in any recoverable form.
//
// # What this file does not do
//
//   - It does not redact the error's message. The message is
//     authored by the error's constructor; the constructor is
//     responsible for not placing secrets in it. Redacting the
//     message would require parsing natural language, which is
//     not a well-defined operation.
//   - It does not redact the error's code. The code is a stable,
//     registered identifier; it does not contain user data.
//   - It does not redact the error's category. The category is a
//     member of a closed set; it does not contain user data.
//   - It does not consult a configuration. The policy is frozen;
//     it is the same for every process, every invocation, and
//     every output format. A future ADR may add configuration
//     options; until then, the policy is a constant.
//
// Only the Go standard library is used (see
// docs/dependency-policy.md).
package forgeerr

import (
	"os"
	"path/filepath"
	"strings"
)

// redactedMarker is the placeholder substituted for a secret
// value. The marker is ASCII-only, is enclosed in angle
// brackets, and does not contain whitespace, so it can be
// rendered by any handler without escaping.
//
// The marker is a constant so that it has exactly one
// definition. A future change to its spelling is a change to the
// redaction contract and requires an ADR.
const redactedMarker = "<redacted>"

// secretKeys is the frozen list of context-key names whose
// values are treated as secrets. The list is defined in WBS
// 10.4.3 and is exhaustive: a key not in this list is not a
// secret, even if its value looks like one.
//
// # Why a list of keys and not a list of patterns
//
// The alternative design — matching on the value's shape, the
// way secret scanners do — is fragile. A token whose shape is
// not in the pattern list leaks. A non-secret value whose shape
// happens to match the pattern is redacted unnecessarily. The
// key-based design is deterministic: the caller names the field
// when it constructs the error, and the policy treats the field
// as a secret by name.
//
// # Matching rules
//
//  1. Case-insensitive: "Token", "TOKEN", and "token" all match.
//  2. Word-boundary: the key is split on non-alphanumeric
//     characters (underscore, hyphen, dot), and each segment is
//     compared to the list. "api_token" and "api_token_v2" match
//     because one of their segments is "token". "monkey" does
//     not match, because "monkey" is a single segment that is
//     not equal to "key".
//  3. Exact match per segment: "tokens" does not match, because
//     "tokens" is not equal to "token". The rule is
//     deliberately strict: a pluralised form is a different key,
//     and the caller should name it explicitly.
var secretKeys = []string{
	"token",
	"password",
	"secret",
	"key",
	"credential",
	"auth",
}

// isSecretKey reports whether key is one of the frozen secret
// keys. The match is case-insensitive and word-boundary-based;
// see the secretKeys doc comment for the rules.
//
// The function is exported by convention as an internal helper:
// it is unexported because no caller outside the package needs
// it. A future caller that wants to apply the same policy to a
// non-forgeerr value can read the list through AllSecretKeys.
func isSecretKey(key string) bool {
	segments := splitKeySegments(key)
	for _, seg := range segments {
		for _, secret := range secretKeys {
			if strings.EqualFold(seg, secret) {
				return true
			}
		}
	}
	return false
}

// splitKeySegments splits a context key into its word segments.
// The separators are underscore, hyphen, and dot; the case of
// the original key is preserved so that the caller can compare
// with EqualFold.
//
// The function returns the key unchanged as a single segment
// when it contains no separators. This is the common case for
// snake_case keys, which the context-key rule already requires.
//
// Examples:
//
//	"token"               -> ["token"]
//	"api_token"           -> ["api", "token"]
//	"API_TOKEN"           -> ["API", "TOKEN"]
//	"api.token.v2"        -> ["api", "token", "v2"]
//	"monkey"              -> ["monkey"]
//	"auth-credential"     -> ["auth", "credential"]
func splitKeySegments(key string) []string {
	// Fast path: no separators means the key is its own segment.
	if !strings.ContainsAny(key, "_-.") {
		return []string{key}
	}
	return strings.FieldsFunc(key, func(r rune) bool {
		return r == '_' || r == '-' || r == '.'
	})
}

// AllSecretKeys returns a copy of the frozen secret-keys list.
//
// The copy protects the internal list from mutation: a caller
// that sorts or truncates the returned slice does not affect the
// package's policy. This is the same defensive-copy discipline
// as AllCategories and AllCodes.
//
// The function exists for tests, for documentation generators,
// and for any future caller that needs to apply the same policy
// to a value outside the package. The runtime path uses
// isSecretKey, which does not allocate.
func AllSecretKeys() []string {
	out := make([]string, len(secretKeys))
	copy(out, secretKeys)
	return out
}

// redact returns value unchanged when key is not a secret, and
// returns redactedMarker when key is a secret.
//
// The function is the primary entry point for context-value
// redaction. Both Format and LogValue call it once per context
// entry, passing the entry's key and value.
//
// # Order of operations
//
// The function checks the key first, then the value. The order
// matters: a secret-typed key with an empty value is still
// redacted, because the value's emptiness is not a signal that
// the field is safe to render. A caller that wants an empty
// value to be preserved should not use a secret-typed key.
//
// # Empty key
//
// An empty key is not a secret. The function returns value
// unchanged. This is consistent with the constructor's
// validation: WithContext rejects an empty key, so a *Error
// cannot have an empty-keyed context entry. The behaviour of
// redact on an empty key is defined for callers outside the
// package that use the function directly.
//
// # Non-secret keys
//
// A non-secret key's value is returned unchanged. The caller
// may then apply a further transformation (for example,
// redactPath) if the value is a path.
func redact(key, value string) string {
	if isSecretKey(key) {
		return redactedMarker
	}
	return value
}

// redactPath replaces the user's home directory at the start of
// path with "~". Other paths are returned unchanged.
//
// The function is called by Format and LogValue after redact,
// for context values whose keys are not secrets but whose values
// may be paths. The two-step pipeline is:
//
//	value = redact(key, value)          // secrets
//	value = redactPath(value)           // home directory
//
// # Home directory resolution
//
// The user's home directory is read from the process's
// environment via os.UserHomeDir, which consults $HOME on Unix
// and %USERPROFILE% on Windows. A caller that wants a different
// home (for example, a test that installs a synthetic home
// directory) must arrange the environment before calling the
// function, or use redactPathWithHome.
//
// # Path shape
//
// The function replaces the home directory only when it is the
// *first* segment of the path. A path that contains the home
// directory in a non-leading position (for example,
// "/var/data/home/user/file") is not redacted. This is
// deliberate: the home directory is a prefix property, and
// replacing it wherever it appears would corrupt paths whose
// segments happen to coincide with the home path.
//
// # Trailing separator
//
// The function preserves the trailing separator, if any:
//
//	/home/user          -> ~
//	/home/user/file     -> ~/file
//	/home/user/         -> ~
//	/home/user/sub/     -> ~/sub/
//
// # Non-matching paths
//
// A path that does not start with the home directory is returned
// unchanged. This includes:
//
//   - Absolute paths outside the home directory.
//   - Relative paths ("foo", "./foo", "../foo").
//   - The "~" prefix itself ("~/foo").
//   - A path that is exactly the home directory's parent.
//
// # Symlinks
//
// The function does not resolve symlinks. Resolving a symlink
// requires a filesystem read, which is a side effect the
// redaction layer deliberately avoids: the layer is a pure
// function of its inputs, called from two formatters that may
// run in contexts where a filesystem read is undesirable.
//
// A caller that wants symlink-aware redaction resolves the
// symlink before constructing the *Error, so that the value in
// the context map is already the resolved path.
//
// The WBS's mention of "symlinks resolved before redaction" is
// a caller-side rule, documented in WithContext's doc comment.
// The redaction layer's contract is that it treats its input as
// a path string and performs a prefix match.
func redactPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		// No home directory is available. Return the path
		// unchanged. This is the correct behaviour when the
		// process has no HOME (for example, in a minimal
		// container): redacting against an unknown home would
		// require guessing.
		return path
	}
	return redactPathWithHome(path, home)
}

// redactPathWithHome is redactPath with an explicit home
// directory. It is unexported and exists so that tests can
// exercise the prefix logic without manipulating the process's
// environment.
//
// The function is the pure core of redactPath: given a path and
// a home directory, it returns the redacted path. It does not
// read the environment, does not consult the filesystem, and is
// deterministic in its inputs.
func redactPathWithHome(path, home string) string {
	if home == "" {
		return path
	}

	// Normalise the home directory to have no trailing
	// separator, so that the comparison is uniform.
	home = strings.TrimRight(home, string(filepath.Separator))

	// The path must start with the home directory followed by
	// either end-of-string or a separator. The separator check
	// prevents "/home/user" from matching "/home/username".
	if !strings.HasPrefix(path, home) {
		return path
	}
	rest := path[len(home):]
	if rest == "" {
		return "~"
	}
	if !strings.HasPrefix(rest, string(filepath.Separator)) {
		// The path is a sibling whose name shares the home
		// directory's prefix. Do not redact.
		return path
	}

	return "~" + rest
}

// redactPathFor returns the redacted form of path for the given
// mode.
//
// In non-verbose mode, the function behaves like redactPath:
// a path beginning with the user's home directory is replaced
// with the "~" prefix.
//
// In verbose mode, the function returns path unchanged. The
// verbose mode (WBS 10.4.4) relaxes path redaction so that full
// paths are visible for debugging. The relaxation is scoped to
// paths; secret redaction (the redact function) is unaffected.
func redactPathFor(path string, verbose bool) string {
	if verbose {
		return path
	}
	return redactPath(path)
}
