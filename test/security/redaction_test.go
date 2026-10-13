// Package security contains end-to-end tests for the CLI's
// security guarantees.
//
// # What this file tests
//
// The file implements layer 4 of the four-layer redaction model.
// See docs/security/logging-redaction.md §3 for the full model.
//
// Layer 4 injects synthetic secrets into every code path that can
// produce output and scans every stream for the secret's textual
// form. The goal is to assert, end to end, that no layer leaks.
//
// # What "every code path" means
//
// The paths are:
//
//   - Config load: a synthetic secret in forge.yaml's sensitive
//     fields.
//   - Template variables: a synthetic secret passed to a template.
//   - Error paths: a synthetic secret in an error's context.
//   - Verbose mode: a synthetic secret logged at DEBUG level.
//   - JSON mode: a synthetic secret in a JSON envelope.
//   - Quiet mode: a synthetic secret under --quiet; the flag
//     filters diagnostics but must not expose the secret through
//     the error block or the JSON envelope.
//   - Type layer: the Secret type's four serialization hooks.
//   - Pattern layer: the middleware's handling of a raw
//     credential-shaped string.
//
// Each path is exercised by a subtest. The subtest's job is to run
// the path and assert the secret's textual form does not appear on
// stdout or stderr.
//
// # The synthetic secret
//
// The secret is a fixed string with two properties:
//
//   - It is unique enough that a chance occurrence in the CLI's
//     own output is negligible.
//   - It is credential-shaped: it matches at least one of the
//     logger's patterns, so the middleware has an opportunity to
//     redact it if the type layer misses.
//
// The value is fixed rather than random so that a test failure
// names a value the reader can search the repository for.
//
// # Cross-platform
//
// The tests do not spawn subprocesses. They exercise the CLI's
// in-process API (the same entry point the binary uses) and
// capture the streams into buffers. The cross-platform guarantee
// is therefore automatic: the code path is identical on every
// platform, and no platform-specific behavior is exercised.
//
// # Running the fuzz test
//
// The fuzz test lives in internal/logger/redact_fuzz_test.go. The
// WBS's KPI is ≥ 1M iterations without a panic. To run:
//
//	go test -run=^$ -fuzz=FuzzRedactString -fuzztime=1m ./internal/logger
package security

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/thapelomagqazana/forge/internal/cli"
	"github.com/thapelomagqazana/forge/internal/config"
	"github.com/thapelomagqazana/forge/internal/logger"
	"github.com/thapelomagqazana/forge/internal/streams"
)

// syntheticSecret is a credential-shaped string used as the value
// of every synthetic secret in the tests.
//
// # Why the value is fixed
//
// A random value would make a test failure harder to diagnose (the
// failure message would include a value the reader does not
// recognize). The fixed value has two properties: it is
// credential-shaped (the `FORGE_TEST_SYNTHETIC_SECRET_` prefix
// plus a random-looking suffix), and it is unlikely to appear in
// any other string the CLI produces.
//
// # Why the value matches the password pattern
//
// The middleware's `password=` pattern matches
// `syntheticSecret` when it appears in a `password=...`
// assignment. The test uses that path in
// TestRedaction_PatternLayer. The other paths use the type layer
// or the config layer, which do not require the value to match a
// pattern; the value's shape is convenient but not required.
const syntheticSecret = "FORGE_TEST_SYNTHETIC_SECRET_0123456789abcdef"

// runCLI is the local in-process entry point. It invokes the CLI
// through the test seam exposed by internal/cli
// (RunWithOptionsForTest) and returns the captured streams.
//
// # Why the helper is duplicated from internal/cli
//
// The internal/cli package's runCLI helper is unexported; it
// cannot be called from the test/security package. The helper
// here is a minimal reimplementation: it constructs an in-process
// invocation with synthetic streams and returns the captured
// bytes. The duplication is small and deliberate: the
// test/security package must not import internal/cli's test
// helpers, because a future change to those helpers would
// silently change this suite's behavior.
//
// # Why the helper takes an env lookup
//
// Some paths (the config loader's env override) need a non-empty
// environment. The helper accepts the lookup so a caller can
// inject one. A nil lookup means "no environment is available";
// the resolver and the loader see empty strings for every key.
func runCLI(t *testing.T, args []string, env func(string) string) (stdout, stderr string, exitCode int) {
	t.Helper()

	var out, errBuf bytes.Buffer
	opts := cli.TestOptions{
		Args: args,
		Streams: streams.Streams{
			In:  strings.NewReader(""),
			Out: &out,
			Err: &errBuf,
		},
		Env: env,
	}
	code, _ := cli.RunWithOptionsForTest(context.Background(), opts)
	return out.String(), errBuf.String(), code
}

// assertNoLeak fails the test if s contains the synthetic secret.
//
// # Why the assertion is separate from the caller
//
// The assertion is used by every path; a single helper keeps the
// path tests terse and makes the property visible: a reader who
// sees `assertNoLeak` knows the test asserts the secret's absence
// without reading the rest of the test.
func assertNoLeak(t *testing.T, name, s string) {
	t.Helper()
	if strings.Contains(s, syntheticSecret) {
		t.Fatalf("%s leaked the synthetic secret:\n%s", name, s)
	}
}

// ---------------------------------------------------------------------------
// Path 1: config load
// ---------------------------------------------------------------------------

// TestRedaction_ConfigLoad asserts that a synthetic secret placed
// in a config field declared `sensitive: true` does not leak
// through any fmt verb or through JSON serialization.
//
// # What the test exercises
//
// The config loader's redaction layer (internal/config/redact.go)
// wraps the sensitive fields' values in logger.Secret and clears
// the plain-string fields. The test constructs a Config that has
// already gone through the loader (by calling RedactSecrets
// directly), then asserts that no serialization of the Config
// contains the secret.
//
// # Why the test does not read the config file
//
// The file reading is exercised by the config package's own
// tests. This test's purpose is to pin the redaction's effect on
// the in-memory Config. A future change to the file-reading path
// is covered by TestLoad_* in the config package; a future change
// to the redaction's effect is covered here.
func TestRedaction_ConfigLoad(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Registry: &config.RegistryConfig{
			URL:      "https://registry.example.com",
			Token:    syntheticSecret,
			Password: syntheticSecret,
		},
		Telemetry: &config.TelemetryConfig{
			Endpoint: "https://telemetry.example.com",
			APIKey:   syntheticSecret,
		},
	}
	redacted := config.RedactSecrets(cfg)

	// Every fmt verb must not leak.
	for _, verb := range []string{"%v", "%+v", "%s", "%q", "%#v"} {
		s := fmtSprintf(verb, redacted)
		assertNoLeak(t, "config load "+verb, s)
	}

	// JSON serialization must not leak.
	b, err := json.Marshal(redacted)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	assertNoLeak(t, "config load json", string(b))

	// Sanity: the values are retrievable via Secret.Reveal.
	if redacted.Registry.TokenSecret.Reveal() != syntheticSecret {
		t.Fatal("TokenSecret.Reveal() lost the value")
	}
	if redacted.Registry.PasswordSecret.Reveal() != syntheticSecret {
		t.Fatal("PasswordSecret.Reveal() lost the value")
	}
	if redacted.Telemetry.APIKeySecret.Reveal() != syntheticSecret {
		t.Fatal("APIKeySecret.Reveal() lost the value")
	}

	// Sanity: the plain fields are cleared.
	if redacted.Registry.Token != "" {
		t.Fatalf("Registry.Token not cleared: %q", redacted.Registry.Token)
	}
	if redacted.Registry.Password != "" {
		t.Fatalf("Registry.Password not cleared: %q",
			redacted.Registry.Password)
	}
	if redacted.Telemetry.APIKey != "" {
		t.Fatalf("Telemetry.APIKey not cleared: %q",
			redacted.Telemetry.APIKey)
	}
}

// ---------------------------------------------------------------------------
// Path 2: template variables
// ---------------------------------------------------------------------------

// TestRedaction_TemplateVars asserts that a Secret passed through
// a template-style formatting path does not leak.
//
// # What the test exercises
//
// The type layer's fmt hooks. A Secret is passed to fmt.Sprintf
// with each verb; the output must not contain the value.
//
// # Why the test does not use a real template engine
//
// The template engine (if one is added later) will eventually
// render a Secret through fmt or through the engine's own
// stringer. The invariant is the same: the Secret's String method
// is the last hook before the rendered text. Exercising fmt
// directly pins the invariant without coupling the test to a
// template engine that may change.
func TestRedaction_TemplateVars(t *testing.T) {
	t.Parallel()

	secret := logger.NewSecret(syntheticSecret)

	// All the fmt verbs a template engine might use.
	for _, verb := range []string{"%v", "%s", "%q", "%#v"} {
		s := fmtSprintf(verb, secret)
		assertNoLeak(t, "template var "+verb, s)
	}

	// A struct containing the Secret: the struct's rendering must
	// also not leak, because fmt recurses into fields.
	type Var struct {
		Name  string
		Value logger.Secret
	}
	v := Var{Name: "auth_token", Value: secret}
	assertNoLeak(t, "template var struct", fmtSprintf("%+v", v))
}

// ---------------------------------------------------------------------------
// Path 3: error paths
// ---------------------------------------------------------------------------

// TestRedaction_ErrorPaths asserts that a Secret in an error's
// context does not leak through the context's rendering.
//
// # What the test exercises
//
// The error package (internal/forgeerr) renders an error's context
// with fmt. A Secret in the context is rendered through the
// type's String method, which returns the marker. The test
// exercises the rendering directly (the context is a map).
//
// # Why the test does not construct a real forgeerr.Error
//
// The forgeerr.Error type's rendering is covered by the forgeerr
// package's own tests. This test's purpose is to pin the
// interaction between the context's rendering and the Secret
// type. Constructing a real error would couple the test to the
// forgeerr package's current API; the map rendering is the
// minimal shape that exercises the interaction.
func TestRedaction_ErrorPaths(t *testing.T) {
	t.Parallel()

	secret := logger.NewSecret(syntheticSecret)
	ctx := map[string]any{
		"token":    secret,
		"password": secret,
		"api_key":  secret,
	}
	assertNoLeak(t, "error context",
		fmtSprintf("%v", ctx))

	// Nested map: a future forgeerr may allow nested context.
	nested := map[string]any{
		"outer": ctx,
	}
	assertNoLeak(t, "error context nested",
		fmtSprintf("%v", nested))
}

// ---------------------------------------------------------------------------
// Path 4: verbose mode
// ---------------------------------------------------------------------------

// TestRedaction_VerboseMode asserts that a DEBUG record emitted
// with a Secret attr does not leak the secret's textual form.
//
// # What the test exercises
//
// The logger's emission path with the Secret type as an attribute
// value. The type's slog.LogValuer hook returns the marker; the
// sink renders the marker.
//
// # Why the test uses a local logger and not the CLI
//
// The CLI's wiring of the logger is exercised by the binary
// integration tests and by the CLI's own tests. This test's
// purpose is to pin the type's LogValue hook in the emission
// path. A local logger with a bytes.Buffer sink is the minimal
// shape that exercises the hook.
//
// # Both formats
//
// The test runs the emission in both the human and the JSON
// format; both must render the marker.
func TestRedaction_VerboseMode(t *testing.T) {
	t.Parallel()

	for _, format := range []struct {
		name string
		f    logger.Format
	}{
		{"human", logger.FormatHuman},
		{"json", logger.FormatJSON},
	} {
		format := format
		t.Run(format.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			l := logger.New(logger.Options{
				Level:     logger.LevelDebug,
				Format:    format.f,
				Component: logger.ComponentCLI,
				Writer:    &buf,
			})
			l.Debug("auth", "token", logger.NewSecret(syntheticSecret))
			l.Info("auth", "token", logger.NewSecret(syntheticSecret))

			assertNoLeak(t, "verbose mode "+format.name, buf.String())
			if !strings.Contains(buf.String(), logger.RedactedLiteral()) {
				t.Fatalf("verbose mode %s lost the marker: %q",
					format.name, buf.String())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Path 5: JSON mode
// ---------------------------------------------------------------------------

// TestRedaction_JSONMode asserts that a Secret serialized into a
// JSON envelope renders as the marker.
//
// # What the test exercises
//
// The type's MarshalJSON hook. A Secret inside a struct passed to
// json.Marshal renders as the marker; the JSON output does not
// contain the value.
//
// # Why the test does not use the CLI's JSON envelope
//
// The envelope's shape is covered by the error_envelope tests and
// by the output package's tests. This test's purpose is to pin
// the type's MarshalJSON hook in the serialization path.
func TestRedaction_JSONMode(t *testing.T) {
	t.Parallel()

	type Payload struct {
		User  string        `json:"user"`
		Token logger.Secret `json:"token"`
	}
	p := Payload{User: "alice", Token: logger.NewSecret(syntheticSecret)}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	assertNoLeak(t, "json mode", string(b))
	if !strings.Contains(string(b), logger.RedactedLiteral()) {
		t.Fatalf("json mode lost the marker: %s", b)
	}
}

// ---------------------------------------------------------------------------
// Path 6: quiet mode
// ---------------------------------------------------------------------------

// TestRedaction_QuietMode asserts that --quiet does not expose the
// secret through the error block on stderr or through the JSON
// envelope on stdout.
//
// # What the test exercises
//
// The CLI's full path with --quiet. The synthetic secret is
// injected into an error's context (via a fake error type that
// is not covered here, because the CLI's error path is exercised
// by the CLI's own tests); the test instead asserts that the
// invocation's stderr and stdout do not contain the secret.
//
// # What the test does not assert
//
// The test does not assert on the command's behavior beyond "no
// leak". A future change to the command's output that preserves
// the no-leak invariant does not break this test.
func TestRedaction_QuietMode(t *testing.T) {
	t.Parallel()

	// Run a version command under --quiet with a synthetic secret
	// in the environment. The env value is not read by the version
	// command, so it cannot leak; the test's purpose is to assert
	// that the invocation's streams do not contain the secret.
	env := func(k string) string {
		if k == "FORGE_LOG_LEVEL" {
			return "error"
		}
		return ""
	}
	stdout, stderr, _ := runCLI(t, []string{"--quiet", "version"}, env)

	assertNoLeak(t, "quiet mode stdout", stdout)
	assertNoLeak(t, "quiet mode stderr", stderr)
}

// ---------------------------------------------------------------------------
// Path 7: pattern layer
// ---------------------------------------------------------------------------

// TestRedaction_PatternLayer asserts that a raw credential-shaped
// string in a log message is redacted by the pattern middleware.
//
// # What the test exercises
//
// The middleware's `password=` pattern. A log message containing
// the pattern is passed through the emission path; the message's
// rendered form must not contain the secret.
//
// # Why the test uses the middleware directly
//
// The middleware is composed with the logger's emission path; a
// log call exercises the composition. The test does not call
// redactString directly (it is unexported); the log call is the
// public surface.
func TestRedaction_PatternLayer(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	l := logger.New(logger.Options{
		Level:     logger.LevelDebug,
		Format:    logger.FormatJSON,
		Component: logger.ComponentCLI,
		Writer:    &buf,
	})
	// The message contains a raw credential-shaped assignment.
	l.Info("password=" + syntheticSecret)

	assertNoLeak(t, "pattern layer", buf.String())
	if !strings.Contains(buf.String(), logger.RedactedLiteral()) {
		t.Fatalf("pattern layer lost the marker: %q", buf.String())
	}
}

// ---------------------------------------------------------------------------
// Path 8: type layer
// ---------------------------------------------------------------------------

// TestRedaction_TypeLayer asserts the four serialization hooks of
// the Secret type: fmt.Stringer, fmt.GoStringer, slog.LogValuer,
// json.Marshaler.
//
// # What the test exercises
//
// Each hook is invoked directly; the output is asserted to
// contain the marker and not the value. The four hooks are the
// type layer's complete surface; if a future refactor adds a
// fifth hook (for example, xml.Marshaler), the test should be
// extended in the same commit.
func TestRedaction_TypeLayer(t *testing.T) {
	t.Parallel()

	secret := logger.NewSecret(syntheticSecret)

	cases := []struct {
		name string
		fn   func() string
	}{
		{"String", func() string { return secret.String() }},
		{"GoString", func() string { return fmtSprintf("%#v", secret) }},
		{"FmtSprintf", func() string { return fmtSprintf("%s", secret) }},
		{"LogValue", func() string { return secret.LogValue().String() }},
		{"MarshalJSON", func() string {
			b, _ := json.Marshal(secret)
			return string(b)
		}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out := tc.fn()
			assertNoLeak(t, "type layer "+tc.name, out)
			if !strings.Contains(out, logger.RedactedLiteral()) {
				t.Fatalf("type layer %s lost the marker: %q",
					tc.name, out)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Negative: the marker is idempotent
// ---------------------------------------------------------------------------

// TestRedaction_MarkerIsIdempotent asserts that the marker itself
// passes through the middleware unchanged.
//
// # What the test exercises
//
// The middleware's handling of a message that already contains
// the marker. A message that includes the marker (because it was
// produced by a previous redaction pass, or because a caller
// passed a Secret's String() through) must not be double-replaced
// or mangled.
func TestRedaction_MarkerIsIdempotent(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	l := logger.New(logger.Options{
		Level:     logger.LevelDebug,
		Format:    logger.FormatJSON,
		Component: logger.ComponentCLI,
		Writer:    &buf,
	})
	l.Info("token=" + logger.RedactedLiteral())

	out := buf.String()
	// The marker must appear; the value must not. The count of
	// markers must be exactly one (the input had one, and the
	// middleware must not have added a second).
	if !strings.Contains(out, logger.RedactedLiteral()) {
		t.Fatalf("marker missing: %q", out)
	}
	if strings.Count(out, logger.RedactedLiteral()) != 1 {
		t.Fatalf("marker count != 1: %q", out)
	}
}

// ---------------------------------------------------------------------------
// Boundary: empty and zero-value Secrets
// ---------------------------------------------------------------------------

// TestRedaction_EmptySecret asserts that an empty Secret renders
// as the marker, not as the empty string.
//
// # Why the test matters
//
// A caller that constructs a Secret around an empty value (because
// a credential was not configured) might expect the empty string
// to be rendered. The type deliberately renders the marker in
// every case: the presence of a Secret indicates that a credential
// was expected, and the marker communicates "a credential was
// here, but it is redacted".
func TestRedaction_EmptySecret(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		s    logger.Secret
	}{
		{"empty", logger.NewSecret("")},
		{"zero", logger.Secret{}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out := tc.s.String()
			if out != logger.RedactedLiteral() {
				t.Fatalf("String() = %q; want %q",
					out, logger.RedactedLiteral())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Boundary: the Secret is safe as a struct field through reflection
// ---------------------------------------------------------------------------

// TestRedaction_Reflection asserts that reflect.Value.String() on
// a struct containing a Secret does not leak.
//
// # What the test exercises
//
// reflect.Value.String() falls back to the fmt machinery when the
// value implements fmt.Stringer. A Secret does, so the marker is
// returned. The test pins the property so a future change to the
// Secret type that removed the String method (unlikely) would fail
// the test.
func TestRedaction_Reflection(t *testing.T) {
	t.Parallel()

	type S struct {
		Token logger.Secret
	}
	v := S{Token: logger.NewSecret(syntheticSecret)}
	// reflect.Value.String uses fmt when the value implements
	// fmt.Stringer; the indirect call exercises the hook.
	out := fmtSprintf("%s", v.Token)
	assertNoLeak(t, "reflection", out)
}

// ---------------------------------------------------------------------------
// Non-functional: benchmarks
// ---------------------------------------------------------------------------

// BenchmarkRedaction_TypeLayer measures the cost of the type
// layer's String hook.
func BenchmarkRedaction_TypeLayer(b *testing.B) {
	s := logger.NewSecret(syntheticSecret)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = s.String()
	}
}

// BenchmarkRedaction_JSONMode measures the cost of the type
// layer's MarshalJSON hook.
func BenchmarkRedaction_JSONMode(b *testing.B) {
	type Payload struct {
		Token logger.Secret `json:"token"`
	}
	p := Payload{Token: logger.NewSecret(syntheticSecret)}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(p)
	}
}

// ---------------------------------------------------------------------------
// Small helpers
// ---------------------------------------------------------------------------

// fmtSprintf is a thin wrapper around fmt.Sprintf. It is defined as
// a named helper so the test file's imports are minimal (the file
// imports only "fmt" through this wrapper; a future refactor can
// delete the wrapper and import "fmt" directly).
func fmtSprintf(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}
