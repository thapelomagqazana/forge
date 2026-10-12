// Package forgeerr — redaction policy tests.
//
// Every test is named after the acceptance criterion (AC1–AC8)
// it verifies. The eight criteria are:
//
//	AC1  Secret keys list frozen.
//	AC2  Each secret key redacted.
//	AC3  Case-insensitive matching.
//	AC4  Word-boundary matching.
//	AC5  Home path redacted to ~.
//	AC6  Cause never shown to users (covered in format_test).
//	AC7  Stack traces only in verbose (verbose plumbing is WBS
//	     10.4.4; this file provides the predicate).
//	AC8  Documentation lists redaction rules.
//
// Test categories:
//
//   - ATDD:      AC-mapped tests
//   - Positive:  keys and paths that are redacted
//   - Negative:  keys and paths that are not redacted
//   - Edge:      empty keys, empty values, non-ASCII, very long
//   - Corner:    key collisions with the reserved names
//   - BND:       paths that share the home directory's prefix
//   - NF:        determinism, concurrency, allocation
//   - Fuzz:      invariant preservation
package forgeerr

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// --- AC1: secret keys list frozen -------------------------------------

// TestAC1_SecretKeysListFrozen asserts that the secret-keys list
// contains exactly the six keys named in WBS 10.4.3.
func TestAC1_SecretKeysListFrozen(t *testing.T) {
	t.Parallel()

	want := []string{"token", "password", "secret", "key", "credential", "auth"}
	got := AllSecretKeys()

	if len(got) != len(want) {
		t.Fatalf("AllSecretKeys() has %d entries, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("AllSecretKeys()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestAC1_AllSecretKeysReturnsCopy asserts that mutating the
// returned slice does not affect the package's list.
func TestAC1_AllSecretKeysReturnsCopy(t *testing.T) {
	t.Parallel()

	first := AllSecretKeys()
	first[0] = "MUTATED"

	second := AllSecretKeys()
	if second[0] == "MUTATED" {
		t.Fatal("AllSecretKeys returned an aliased slice")
	}
}

// --- AC2: each secret key redacted ------------------------------------

// TestAC2_EachSecretKeyRedacted asserts that every key in the
// frozen list produces the redaction marker.
func TestAC2_EachSecretKeyRedacted(t *testing.T) {
	t.Parallel()

	for _, key := range AllSecretKeys() {
		key := key
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			got := redact(key, "any-value")
			if got != redactedMarker {
				t.Fatalf("redact(%q, ...) = %q, want %q", key, got, redactedMarker)
			}
		})
	}
}

// --- AC3: case-insensitive matching -----------------------------------

// TestAC3_CaseInsensitiveMatching asserts that the keys match
// regardless of case.
func TestAC3_CaseInsensitiveMatching(t *testing.T) {
	t.Parallel()

	variants := []string{
		"token", "Token", "TOKEN", "tOkEn",
		"password", "Password", "PASSWORD",
		"secret", "Secret", "SECRET",
		"key", "Key", "KEY",
		"credential", "Credential", "CREDENTIAL",
		"auth", "Auth", "AUTH",
	}
	for _, key := range variants {
		key := key
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			if got := redact(key, "v"); got != redactedMarker {
				t.Fatalf("redact(%q, ...) = %q, want %q", key, got, redactedMarker)
			}
		})
	}
}

// --- AC4: word-boundary matching --------------------------------------

// TestAC4_WordBoundaryMatching asserts that the key is split on
// underscore, hyphen, and dot, and each segment is compared to
// the list. A key whose segment is not exactly a secret key is
// not redacted.
func TestAC4_WordBoundaryMatching(t *testing.T) {
	t.Parallel()

	redacted := []string{
		"api_token",
		"api.token",
		"api-token",
		"API_TOKEN",
		"token_v2",
		"v2.token",
		"auth_credential",
		"my_secret_key", // contains both "secret" and "key"
	}
	for _, key := range redacted {
		key := key
		t.Run("redacted/"+key, func(t *testing.T) {
			t.Parallel()
			if got := redact(key, "v"); got != redactedMarker {
				t.Fatalf("redact(%q, ...) = %q, want %q", key, got, redactedMarker)
			}
		})
	}

	notRedacted := []string{
		"monkey",         // "key" is not a separate segment
		"tokens",         // "tokens" != "token"
		"passwords",      // plural is a different key
		"keychain",       // "keychain" is not a segment
		"author",         // "auth" is not a separate segment
		"authentication", // "auth" is a prefix but not a segment
		"file_path",      // no segment matches
		"line_number",    // no segment matches
	}
	for _, key := range notRedacted {
		key := key
		t.Run("not-redacted/"+key, func(t *testing.T) {
			t.Parallel()
			if got := redact(key, "visible"); got != "visible" {
				t.Fatalf("redact(%q, ...) = %q, want unchanged", key, got)
			}
		})
	}
}

// --- AC5: home path redacted to ~ -------------------------------------

// TestAC5_HomePathRedacted asserts that a path starting with the
// home directory is redacted to "~...".
//
// The test uses redactPathWithHome so that the behaviour does not
// depend on the process's actual home directory. A separate test
// (TestAC5_RedactPathUsesProcessHome) verifies the public
// redactPath function against os.UserHomeDir.
// TestAC5_HomePathRedacted asserts that a path starting with the
// home directory is redacted to "~...".
//
// The test uses redactPathWithHome so that the behaviour does not
// depend on the process's actual home directory. A separate test
// (TestAC5_RedactPathUsesProcessHome) verifies the public
// redactPath function against os.UserHomeDir.
//
// # Trailing-separator rule
//
// When the path is exactly the home directory (no trailing
// separator), the output is "~". When the path is the home
// directory followed by a separator and nothing else, the
// output is "~/" — the separator is preserved.
//
// The rule is a prefix substitution, not a path normalisation.
// It is consistent with how tools like fzf and zoxide display
// paths: whatever follows the home prefix is preserved verbatim.
// A caller that wants "/home/alice/" to render as "~" must
// normalise the path before placing it in the context map.
func TestAC5_HomePathRedacted(t *testing.T) {
	t.Parallel()

	home := "/home/alice"
	cases := []struct {
		path string
		want string
	}{
		{"/home/alice", "~"},
		{"/home/alice/", "~/"},
		{"/home/alice/file", "~/file"},
		{"/home/alice/sub/file", "~/sub/file"},
		{"/home/alice/sub/", "~/sub/"},
		// Sibling whose name shares the prefix: not redacted.
		{"/home/alice2", "/home/alice2"},
		{"/home/alice2/file", "/home/alice2/file"},
		// Different home: not redacted.
		{"/home/bob", "/home/bob"},
		{"/home/bob/file", "/home/bob/file"},
		// Absolute outside home: not redacted.
		{"/var/data/file", "/var/data/file"},
		// Relative: not redacted.
		{"foo", "foo"},
		{"./foo", "./foo"},
		{"../foo", "../foo"},
		// Tilde already: not redacted.
		{"~/foo", "~/foo"},
		// Empty path: not redacted.
		{"", ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()
			got := redactPathWithHome(tc.path, home)
			if got != tc.want {
				t.Fatalf("redactPathWithHome(%q, %q) = %q, want %q",
					tc.path, home, got, tc.want)
			}
		})
	}
}

// TestAC5_RedactPathUsesProcessHome asserts that the public
// redactPath function consults the process's home directory.
//
// The test is skipped on platforms where the home directory is
// not available or where the path separator is not "/", because
// the assertion is built around a Unix-shaped home.
func TestAC5_RedactPathUsesProcessHome(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("test is Unix-shaped")
	}

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("no home directory available")
	}

	in := filepath.Join(home, "config.yaml")
	want := "~/config.yaml"

	if got := redactPath(in); got != want {
		t.Fatalf("redactPath(%q) = %q, want %q", in, got, want)
	}
}

// TestAC5_HomePathPreservedWhenNoHome asserts that a path is
// returned unchanged when no home directory is available.
func TestAC5_HomePathPreservedWhenNoHome(t *testing.T) {
	t.Parallel()

	got := redactPathWithHome("/anything", "")
	if got != "/anything" {
		t.Fatalf("redactPathWithHome(_, \"\") = %q, want unchanged", got)
	}
}

// --- Positive and negative: non-secret values --------------------------

// TestRedact_NonSecretValueReturned asserts that a non-secret
// key's value is returned unchanged.
func TestRedact_NonSecretValueReturned(t *testing.T) {
	t.Parallel()

	cases := []struct {
		key   string
		value string
	}{
		{"file", "config.yaml"},
		{"line", "12"},
		{"path", "/etc/forge/config.yaml"},
		{"command", "version"},
		{"", "value-with-no-key"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.key+"/"+tc.value, func(t *testing.T) {
			t.Parallel()
			if got := redact(tc.key, tc.value); got != tc.value {
				t.Fatalf("redact(%q, %q) = %q, want unchanged",
					tc.key, tc.value, got)
			}
		})
	}
}

// TestRedact_EmptyValueWithSecretKey asserts that a secret-typed
// key with an empty value is still redacted. The emptiness of the
// value is not a signal that the field is safe to render.
func TestRedact_EmptyValueWithSecretKey(t *testing.T) {
	t.Parallel()

	for _, key := range AllSecretKeys() {
		key := key
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			if got := redact(key, ""); got != redactedMarker {
				t.Fatalf("redact(%q, \"\") = %q, want %q",
					key, got, redactedMarker)
			}
		})
	}
}

// TestRedact_NonASCIIValue asserts that a non-secret value with
// non-ASCII characters is returned unchanged. The redaction
// layer does not impose an ASCII-only rule; the ASCII policy
// (WBS 7.4.2) applies to the CLI's user-facing output, not to
// the redaction layer.
func TestRedact_NonASCIIValue(t *testing.T) {
	t.Parallel()

	value := "/home/用户/config.yaml"
	if got := redact("path", value); got != value {
		t.Fatalf("redact(path, %q) = %q, want unchanged", value, got)
	}
}

// --- Corner: key collisions with the reserved names -------------------

// TestCorner_KeyCollidesWithReservedName asserts the behaviour
// when a context key is one of the reserved names used by the
// log group ("code", "category", "cause"). The context key is
// not a secret, so it is not redacted by this layer. Its
// collision with the log group's reserved names is the log
// formatter's concern, documented in WBS 10.4.2.
func TestCorner_KeyCollidesWithReservedName(t *testing.T) {
	t.Parallel()

	for _, key := range []string{"code", "category", "cause"} {
		key := key
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			if got := redact(key, "v"); got != "v" {
				t.Fatalf("redact(%q, ...) = %q, want unchanged", key, got)
			}
		})
	}
}

// --- Boundary: paths at the home directory's edge ---------------------

// TestBoundary_HomeSiblingNotRedacted asserts that a path that
// shares the home directory's prefix but is a sibling (for
// example, "/home/alice2" when the home is "/home/alice") is not
// redacted.
func TestBoundary_HomeSiblingNotRedacted(t *testing.T) {
	t.Parallel()

	home := "/home/alice"
	siblings := []string{
		"/home/alice2",
		"/home/alice-bak",
		"/home/alice.old",
		"/home/aliceX",
	}
	for _, s := range siblings {
		s := s
		t.Run(s, func(t *testing.T) {
			t.Parallel()
			if got := redactPathWithHome(s, home); got != s {
				t.Fatalf("redactPathWithHome(%q, %q) = %q, want unchanged",
					s, home, got)
			}
		})
	}
}

// TestBoundary_HomeWithTrailingSeparator asserts that a home
// directory with a trailing separator is handled uniformly.
func TestBoundary_HomeWithTrailingSeparator(t *testing.T) {
	t.Parallel()

	cases := []struct {
		home string
		path string
		want string
	}{
		{"/home/alice", "/home/alice/file", "~/file"},
		{"/home/alice/", "/home/alice/file", "~/file"},
		{"/home/alice//", "/home/alice/file", "~/file"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.home, func(t *testing.T) {
			t.Parallel()
			got := redactPathWithHome(tc.path, tc.home)
			if got != tc.want {
				t.Fatalf("redactPathWithHome(%q, %q) = %q, want %q",
					tc.path, tc.home, got, tc.want)
			}
		})
	}
}

// --- Non-functional: determinism, concurrency, allocation --------------

// TestNF_Concurrent asserts that redact and redactPath are safe
// for concurrent use.
func TestNF_Concurrent(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = redact("token", "v")
			_ = redact("file", "config.yaml")
			_ = redactPath("/home/alice/file")
		}()
	}
	wg.Wait()
}

// BenchmarkRedact measures the cost of redacting a non-secret
// value. The function is called once per context entry in both
// Format and LogValue, so its cost is on the hot path of error
// construction output.
func BenchmarkRedact(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = redact("file", "config.yaml")
	}
}

// BenchmarkRedactSecret measures the cost of redacting a secret
// value. The check is the same as for a non-secret value; the
// only difference is the returned string.
func BenchmarkRedactSecret(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = redact("token", "v")
	}
}

// --- Fuzz: the redaction invariants -----------------------------------

// FuzzRedact asserts that redact's output is either the input
// value unchanged or the redaction marker, and that a key whose
// segments include a secret key always produces the marker.
func FuzzRedact(f *testing.F) {
	// Seeds cover the positive and negative cases.
	for _, k := range AllSecretKeys() {
		f.Add(k, "value")
	}
	f.Add("file", "config.yaml")
	f.Add("api_token", "value")
	f.Add("monkey", "value")
	f.Add("", "value")
	f.Add("token", "")

	f.Fuzz(func(t *testing.T, key, value string) {
		got := redact(key, value)

		// The output is either the marker or the value.
		if got != redactedMarker && got != value {
			t.Fatalf("redact(%q, %q) = %q; want marker or value", key, value, got)
		}

		// If the key has a secret segment, the output is the
		// marker.
		if isSecretKey(key) && got != redactedMarker {
			t.Fatalf("isSecretKey(%q) = true but redact returned %q", key, got)
		}

		// If the key has no secret segment, the output is the
		// value.
		if !isSecretKey(key) && got != value {
			t.Fatalf("isSecretKey(%q) = false but redact returned %q", key, got)
		}
	})
}

// FuzzRedactPath asserts that redactPathWithHome's output is
// either the input path unchanged or a "~"-prefixed path.
func FuzzRedactPath(f *testing.F) {
	f.Add("/home/alice", "/home/alice")
	f.Add("/home/alice", "/home/alice/file")
	f.Add("/home/alice", "/home/bob")
	f.Add("/home/alice", "/")
	f.Add("/home/alice", "")
	f.Add("", "/any/path")

	f.Fuzz(func(t *testing.T, home, path string) {
		got := redactPathWithHome(path, home)

		// The output is either the path or starts with "~".
		if got != path && !strings.HasPrefix(got, "~") {
			t.Fatalf("redactPathWithHome(%q, %q) = %q; want unchanged or ~-prefixed",
				path, home, got)
		}

		// When home is empty, the path is unchanged.
		if home == "" && got != path {
			t.Fatalf("empty home: redactPathWithHome(%q, \"\") = %q", path, got)
		}
	})
}
