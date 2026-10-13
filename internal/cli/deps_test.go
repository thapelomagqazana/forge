package cli

import (
	"bytes"
	"io"
	"reflect"
	"strings"
	"testing"
	"unsafe"

	"github.com/thapelomagqazana/forge/internal/config"
	"github.com/thapelomagqazana/forge/internal/streams"
)

// =============================================================================
// Helpers
// =============================================================================

// configIsZeroSized reports whether config.Config is a zero-size
// type.
//
// In Phase 2, config.Config is an empty struct and this function
// returns true. When WBS 8.2.1 adds fields, it returns false, and
// tests that assert on pointer identity of *config.Config become
// meaningful.
//
// # Why this helper exists
//
// Taking the address of a zero-size value in Go returns the
// runtime's zero-base pointer, so every &config.Config{} literal
// has the same address. Tests that want to assert "two calls did
// not share a Config" must skip the check while the type is
// zero-size, or they will fail for a reason that has nothing to
// do with the code under test.
//
// The helper is deliberately not exported. It is a test-only
// concern; production code does not need to know the size of
// config.Config.
func configIsZeroSized() bool {
	return unsafe.Sizeof(config.Config{}) == 0
}

// mkOpts returns an options value with buffers for the three
// streams, an empty environment, and a temporary rootPath. It is
// the canonical setup for the tests in this file.
//
// The helper exists so a change to options' shape touches one
// line, not every test. The tests care about the properties of the
// constructed Dependencies, not about the options struct's field
// order.
//
// # Returned value
//
// The streams are in-memory: In is an empty strings.Reader, Out
// and Err are fresh *bytes.Buffer values. A test that wants to
// assert on the buffers calls the helper and then reads
// opts.streams.Out or opts.streams.Err. A test that wants to
// replace a stream sets the field directly:
//
//	opts := mkOpts(t)
//	opts.streams.Out = &myBuffer
//
// The helper does not return the two buffers separately because
// most tests in this file assert on the *properties* of the
// constructed Dependencies (non-nil, distinct, bound to the
// injected value), not on the bytes the buffers receive. A test
// that needs the buffers reads them from opts.streams.
func mkOpts(t *testing.T) options {
	t.Helper()
	return options{
		streams: streams.Streams{
			In:  strings.NewReader(""),
			Out: &bytes.Buffer{},
			Err: &bytes.Buffer{},
		},
		env:      func(string) string { return "" },
		rootPath: t.TempDir(),
	}
}

// =============================================================================
// ATDD: every field is non-nil
// =============================================================================

// TestBuildDependencies_AllFieldsNonNil asserts that every field of
// a Dependencies value constructed by buildDependencies is non-nil.
// A nil field would panic on first use, so this test guards against
// the most common construction mistake.
//
// The Logger is deliberately absent from the assertion: it is not a
// field of Dependencies. The invocation's logger is built by the
// PersistentPreRunE hook and stored on the command's context; see
// internal/cli/hooks.go. A test that wants to observe the logger
// runs the hook and reads from the context.
func TestBuildDependencies_AllFieldsNonNil(t *testing.T) {
	t.Parallel()

	deps := buildDependencies(mkOpts(t))

	if deps.Config == nil {
		t.Error("Config is nil; want non-nil")
	}
	if deps.FS == nil {
		t.Error("FS is nil; want non-nil")
	}
	if deps.Streams.In == nil {
		t.Error("Streams.In is nil; want non-nil")
	}
	if deps.Streams.Out == nil {
		t.Error("Streams.Out is nil; want non-nil")
	}
	if deps.Streams.Err == nil {
		t.Error("Streams.Err is nil; want non-nil")
	}
	if deps.Env == nil {
		t.Error("Env is nil; want non-nil")
	}
}

// =============================================================================
// ATDD: the three streams are injectable
// =============================================================================

// TestBuildDependencies_StdoutIsInjectable asserts that a test can
// substitute a bytes.Buffer for stdout and observe the command's
// output. This is the property that makes handlers testable without
// spawning a subprocess.
func TestBuildDependencies_StdoutIsInjectable(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	opts := mkOpts(t)
	opts.streams.Out = &stdout

	deps := buildDependencies(opts)

	const want = "hello, forge\n"
	if _, err := deps.Streams.Out.Write([]byte(want)); err != nil {
		t.Fatalf("write to Streams.Out: %v", err)
	}
	if got := stdout.String(); got != want {
		t.Errorf("stdout = %q; want %q", got, want)
	}
}

// TestBuildDependencies_StderrIsInjectable asserts the same property
// for stderr.
func TestBuildDependencies_StderrIsInjectable(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	opts := mkOpts(t)
	opts.streams.Err = &stderr

	deps := buildDependencies(opts)

	const want = "diagnostic\n"
	if _, err := deps.Streams.Err.Write([]byte(want)); err != nil {
		t.Fatalf("write to Streams.Err: %v", err)
	}
	if got := stderr.String(); got != want {
		t.Errorf("stderr = %q; want %q", got, want)
	}
}

// TestBuildDependencies_StdinIsInjectable asserts that the input
// stream is the reader supplied in the options, not the process's
// stdin.
func TestBuildDependencies_StdinIsInjectable(t *testing.T) {
	t.Parallel()

	const payload = "piped input"
	opts := mkOpts(t)
	opts.streams.In = strings.NewReader(payload)

	deps := buildDependencies(opts)

	got, err := io.ReadAll(deps.Streams.In)
	if err != nil {
		t.Fatalf("read Streams.In: %v", err)
	}
	if string(got) != payload {
		t.Errorf("Streams.In = %q; want %q", got, payload)
	}
}

// =============================================================================
// ATDD: environment injection
// =============================================================================

// TestBuildDependencies_EnvIsInjectable verifies that the Env field
// returns whatever the injected lookup function returns.
func TestBuildDependencies_EnvIsInjectable(t *testing.T) {
	t.Parallel()

	const key = "FORGE_TEST_VAR"
	const value = "injected"
	opts := mkOpts(t)
	opts.env = func(k string) string {
		if k == key {
			return value
		}
		return ""
	}

	deps := buildDependencies(opts)

	if got := deps.Env(key); got != value {
		t.Errorf("Env(%q) = %q; want %q", key, got, value)
	}
	if got := deps.Env("FORGE_UNSET"); got != "" {
		t.Errorf("Env(unset) = %q; want empty", got)
	}
}

// =============================================================================
// ATDD: filesystem is bound to rootPath
// =============================================================================

// TestBuildDependencies_FSIsBoundToRootPath verifies that the
// filesystem abstraction is bound to the rootPath from the options.
func TestBuildDependencies_FSIsBoundToRootPath(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	opts := mkOpts(t)
	opts.rootPath = root

	deps := buildDependencies(opts)

	if got := deps.FS.Boundary(); got != root {
		t.Errorf("FS.Boundary() = %q; want %q", got, root)
	}
}

// =============================================================================
// ATDD: Config injection
// =============================================================================

// TestBuildDependencies_ConfigInjected verifies that a non-nil
// opts.config is carried into Dependencies unchanged. This is the
// test-injection path the PersistentPreRunE hook detects.
func TestBuildDependencies_ConfigInjected(t *testing.T) {
	t.Parallel()

	injected := &config.Config{}
	opts := mkOpts(t)
	opts.config = injected

	deps := buildDependencies(opts)

	if deps.Config != injected {
		t.Errorf("Config = %p; want the injected pointer %p",
			deps.Config, injected)
	}
}

// TestBuildDependencies_ConfigFallback verifies that a nil
// opts.config produces a non-nil Config in the returned Dependencies.
func TestBuildDependencies_ConfigFallback(t *testing.T) {
	t.Parallel()

	opts := mkOpts(t)
	opts.config = nil

	deps := buildDependencies(opts)

	if deps.Config == nil {
		t.Fatal("Config is nil; want the fallback non-nil value")
	}
}

// =============================================================================
// Edge cases
// =============================================================================

// TestBuildDependencies_EmptyRootPath verifies that an empty
// rootPath is accepted.
func TestBuildDependencies_EmptyRootPath(t *testing.T) {
	t.Parallel()

	opts := mkOpts(t)
	opts.rootPath = ""

	deps := buildDependencies(opts)

	if deps.FS == nil {
		t.Fatal("FS is nil with empty rootPath")
	}
	if got := deps.FS.Boundary(); got != "" {
		t.Errorf("FS.Boundary() = %q; want empty", got)
	}
}

// TestBuildDependencies_NilEnvPanicsOnCall documents the nil-Env
// boundary case: buildDependencies does not guard against a nil
// function because the options struct is always constructed by
// defaultOptions or by the test helper, both of which set a
// non-nil function.
func TestBuildDependencies_NilEnvPanicsOnCall(t *testing.T) {
	t.Parallel()

	opts := mkOpts(t)
	opts.env = nil

	deps := buildDependencies(opts)
	if deps.Env != nil {
		t.Fatal("Env is non-nil; test setup is wrong")
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("Env() did not panic; want panic on nil function")
		}
	}()
	deps.Env("ANY")
}

// TestBuildDependencies_NilStreamsPanicOnUse documents the nil-
// stream boundary case for writes. A nil Out or Err causes a panic
// on the first Write, matching the behavior of any nil io.Writer.
func TestBuildDependencies_NilStreamsPanicOnUse(t *testing.T) {
	t.Parallel()

	opts := mkOpts(t)
	opts.streams.Out = nil
	opts.streams.Err = nil

	deps := buildDependencies(opts)

	t.Run("stdout", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("write to nil Streams.Out did not panic")
			}
		}()
		_, _ = deps.Streams.Out.Write([]byte("x"))
	})

	t.Run("stderr", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("write to nil Streams.Err did not panic")
			}
		}()
		_, _ = deps.Streams.Err.Write([]byte("x"))
	})
}

// =============================================================================
// Struct-level contracts
// =============================================================================

// TestDependencies_ZeroValueIsDocumented pins the documented
// zero-value behavior: Config is a nil pointer (safe to test for
// nil), and every other field is nil and unusable.
func TestDependencies_ZeroValueIsDocumented(t *testing.T) {
	t.Parallel()

	var deps Dependencies

	if deps.Config != nil {
		t.Error("zero-value Config is non-nil; want nil")
	}
	if deps.FS != nil {
		t.Error("zero-value FS is non-nil; want nil")
	}
	if deps.Streams.In != nil {
		t.Error("zero-value Streams.In is non-nil; want nil")
	}
	if deps.Streams.Out != nil {
		t.Error("zero-value Streams.Out is non-nil; want nil")
	}
	if deps.Streams.Err != nil {
		t.Error("zero-value Streams.Err is non-nil; want nil")
	}
	if deps.Env != nil {
		t.Error("zero-value Env is non-nil; want nil")
	}
}

// TestDependencies_IsPassedByValue verifies that copying a
// Dependencies value and mutating the copy does not affect the
// original.
func TestDependencies_IsPassedByValue(t *testing.T) {
	t.Parallel()

	original := Dependencies{
		Streams: streams.Streams{Out: &bytes.Buffer{}},
	}
	copy := original
	copy.Streams.Out = &bytes.Buffer{}

	if original.Streams.Out == copy.Streams.Out {
		t.Error("mutating the copy changed the original; want independent headers")
	}
}

// =============================================================================
// Non-functional: independence between calls
// =============================================================================

// TestBuildDependencies_DoesNotShareState verifies that two calls
// with the same options produce independent Dependencies values.
// This guards against a future refactor that caches a package-level
// value (which would break test isolation).
//
// # Why the Config check is conditional
//
// In Phase 2, config.Config is an empty struct and
// &config.Config{} is a zero-size allocation, so all such pointers
// are equal. The pointer-inequality check for Config is therefore
// skipped while the type is zero-size, and re-enabled automatically
// when WBS 8.2.1 adds fields. See configIsZeroSized.
func TestBuildDependencies_DoesNotShareState(t *testing.T) {
	t.Parallel()

	a := buildDependencies(mkOpts(t))
	b := buildDependencies(mkOpts(t))

	if a.Streams.Out == b.Streams.Out {
		t.Error("two calls shared Streams.Out; want independent writers")
	}
	if a.Streams.Err == b.Streams.Err {
		t.Error("two calls shared Streams.Err; want independent writers")
	}
	if a.FS == b.FS {
		t.Error("two calls shared FS; want independent filesystems")
	}
	if !configIsZeroSized() && a.Config == b.Config {
		t.Error("two calls shared Config; want independent configs")
	}
}

// =============================================================================
// Non-functional: stream contract holds through Dependencies
// =============================================================================

// TestBuildDependencies_StreamsAreDistinct asserts that the three
// streams in the constructed Dependencies are the three the options
// supplied, and that Streams.Out and Streams.Err are not the same
// value.
//
// The property matters because the stream contract (ADR-022)
// depends on output and diagnostics being separated. If a caller
// accidentally passed the same writer for both, the constructed
// Dependencies would carry the same writer in both fields, and a
// command that logged a diagnostic would contaminate the product
// output.
//
// The test does not guard against that misuse (it is the caller's
// responsibility), but it pins the invariant that the fields are
// the values the caller supplied.
func TestBuildDependencies_StreamsAreDistinct(t *testing.T) {
	t.Parallel()

	var stdin, stdout, stderr bytes.Buffer
	opts := mkOpts(t)
	opts.streams.In = &stdin
	opts.streams.Out = &stdout
	opts.streams.Err = &stderr

	deps := buildDependencies(opts)

	if deps.Streams.Out != &stdout {
		t.Errorf("Streams.Out = %p; want %p", deps.Streams.Out, &stdout)
	}
	if deps.Streams.Err != &stderr {
		t.Errorf("Streams.Err = %p; want %p", deps.Streams.Err, &stderr)
	}
	if deps.Streams.In != &stdin {
		t.Errorf("Streams.In = %p; want %p", deps.Streams.In, &stdin)
	}
	if deps.Streams.Out == deps.Streams.Err {
		t.Error("Streams.Out and Streams.Err are the same value; " +
			"the caller supplied the same writer for both")
	}
}

// =============================================================================
// Logger no longer lives here
// =============================================================================

// TestDependencies_HasNoLoggerField is a structural pin: the
// Dependencies struct must not carry a Logger field. The invocation's
// logger travels on the command's context (see hooks.go and
// internal/logger). A future contributor who reintroduces the field
// breaks this test and is forced to read the docstring's rationale.
//
// # How the test detects the field
//
// The test uses reflection to inspect the struct's field set. It is
// the only way to assert the absence of a field at test time; the
// compiler would reject a direct reference.
func TestDependencies_HasNoLoggerField(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeOf(Dependencies{})
	for i := 0; i < typ.NumField(); i++ {
		name := typ.Field(i).Name
		if strings.EqualFold(name, "logger") {
			t.Fatalf("Dependencies has a %q field; the invocation's "+
				"logger travels on the command's context. See the "+
				"package docstring §'What is NOT a field'.", name)
		}
	}
}

// TestDependencies_HasNoDirectStreamFields is the companion pin:
// Dependencies must not carry separate Stdout or Stderr fields.
// The three streams travel together in Streams.
func TestDependencies_HasNoDirectStreamFields(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeOf(Dependencies{})
	for i := 0; i < typ.NumField(); i++ {
		name := typ.Field(i).Name
		switch name {
		case "Stdout", "Stderr", "Stdin":
			t.Fatalf("Dependencies has a %q field; the three streams "+
				"travel together in Streams. See the package "+
				"docstring §'What is NOT a field'.", name)
		}
	}
}
