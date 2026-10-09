package cli

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"unsafe"

	"github.com/thapelomagqazana/forge/internal/config"
)

// Compile-time interface conformance checks.
//
// These assertions fail at compile time if slogLogger or noopLogger
// drift from the Logger contract. They are the cheapest possible
// regression test for interface stability (AC2, AC6).
var (
	_ Logger = (*slogLogger)(nil)
	_ Logger = (noopLogger{})
	_ Logger = (*noopLogger)(nil)
)

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

// TestBuildDependencies_AllFieldsNonNil is the ATDD acceptance test
// for AC2 and AC4. It asserts that every field of a Dependencies
// value constructed by buildDependencies is non-nil. A nil field
// would panic on first use, so this test guards against the most
// common construction mistake.
func TestBuildDependencies_AllFieldsNonNil(t *testing.T) {
	t.Parallel()

	opts := options{
		stdout:   &bytes.Buffer{},
		stderr:   &bytes.Buffer{},
		env:      func(string) string { return "" },
		rootPath: t.TempDir(),
	}

	deps := buildDependencies(opts)

	if deps.Config == nil {
		t.Error("Config is nil; want non-nil")
	}
	if deps.Logger == nil {
		t.Error("Logger is nil; want non-nil")
	}
	if deps.FS == nil {
		t.Error("FS is nil; want non-nil")
	}
	if deps.Stdout == nil {
		t.Error("Stdout is nil; want non-nil")
	}
	if deps.Stderr == nil {
		t.Error("Stderr is nil; want non-nil")
	}
	if deps.Env == nil {
		t.Error("Env is nil; want non-nil")
	}
}

// TestBuildDependencies_StdoutIsInjectable is the ATDD acceptance
// test for AC5. It verifies that a test can substitute a bytes.Buffer
// for stdout and observe the command's output. This is the property
// that makes handlers testable without spawning a subprocess.
func TestBuildDependencies_StdoutIsInjectable(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	opts := options{
		stdout:   &stdout,
		stderr:   &bytes.Buffer{},
		env:      func(string) string { return "" },
		rootPath: t.TempDir(),
	}

	deps := buildDependencies(opts)

	// Simulate a command writing to stdout.
	const want = "hello, forge\n"
	if _, err := io.WriteString(deps.Stdout, want); err != nil {
		t.Fatalf("WriteString: %v", err)
	}

	if got := stdout.String(); got != want {
		t.Errorf("stdout = %q; want %q", got, want)
	}
}

// TestBuildDependencies_EnvIsInjectable verifies that the Env field
// returns whatever the injected lookup function returns. A test can
// therefore simulate environment variables without touching the
// process environment.
func TestBuildDependencies_EnvIsInjectable(t *testing.T) {
	t.Parallel()

	const key = "FORGE_TEST_VAR"
	const value = "injected"
	opts := options{
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		env: func(k string) string {
			if k == key {
				return value
			}
			return ""
		},
		rootPath: t.TempDir(),
	}

	deps := buildDependencies(opts)

	if got := deps.Env(key); got != value {
		t.Errorf("Env(%q) = %q; want %q", key, got, value)
	}
	if got := deps.Env("FORGE_UNSET"); got != "" {
		t.Errorf("Env(unset) = %q; want empty", got)
	}
}

// TestBuildDependencies_FSIsBoundToRootPath verifies that the
// filesystem abstraction is bound to the rootPath from the options.
// The boundary is informational in Phase 2, but it must be recorded
// so that tests can assert on it and so that WBS 13.0 has a value
// to enforce.
func TestBuildDependencies_FSIsBoundToRootPath(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	opts := options{
		stdout:   &bytes.Buffer{},
		stderr:   &bytes.Buffer{},
		env:      func(string) string { return "" },
		rootPath: root,
	}

	deps := buildDependencies(opts)

	if got := deps.FS.Boundary(); got != root {
		t.Errorf("FS.Boundary() = %q; want %q", got, root)
	}
}

// TestNewLogger_WritesStructuredOutput verifies that newLogger
// produces a Logger whose output is structured (key=value pairs).
// This is the contract the CLI UX specification requires.
func TestNewLogger_WritesStructuredOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := newLogger(&buf)

	logger.Info("started", "project", "forge", "phase", 2)

	out := buf.String()
	if !strings.Contains(out, "msg=started") {
		t.Errorf("log output missing msg=started: %q", out)
	}
	if !strings.Contains(out, "project=forge") {
		t.Errorf("log output missing project=forge: %q", out)
	}
	if !strings.Contains(out, "phase=2") {
		t.Errorf("log output missing phase=2: %q", out)
	}
}

// TestNewLogger_WithReturnsIndependentLogger verifies that With
// returns a logger that carries additional fields without mutating
// the receiver. This is the "scope extension" contract from the
// Logger interface documentation.
//
// # Why the assertion inspects per-line content
//
// Parent and child share the same underlying writer (the slog
// handler is shared; only the accumulated attributes differ). The
// test cannot inspect the parent's and child's outputs through
// separate buffers. It must instead inspect the shared buffer
// line by line: each log record is one line, and the test asserts
// on the content of the line that carries each message.
//
// A naive `strings.Contains(buf.String(), "scope=child")` check
// would pass even if the parent message carried the scope, because
// the child's line already contains "scope=child" somewhere in the
// buffer. The per-line check is the only check that distinguishes
// the two records.
func TestNewLogger_WithReturnsIndependentLogger(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	parent := newLogger(&buf)
	child := parent.With("scope", "child")

	child.Info("child message")
	parent.Info("parent message")

	// Each log record is a line. Split the buffer into lines and
	// find the one for each message.
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")

	var childLine, parentLine string
	for _, line := range lines {
		switch {
		case strings.Contains(line, `msg="child message"`):
			childLine = line
		case strings.Contains(line, `msg="parent message"`):
			parentLine = line
		}
	}

	if childLine == "" {
		t.Fatalf("child message not found in output: %q", buf.String())
	}
	if parentLine == "" {
		t.Fatalf("parent message not found in output: %q", buf.String())
	}

	if !strings.Contains(childLine, "scope=child") {
		t.Errorf("child line missing scope=child: %q", childLine)
	}
	if strings.Contains(parentLine, "scope=child") {
		t.Errorf("parent line carried child's scope: %q", parentLine)
	}
}

// TestNoopLogger_DiscardsEverything verifies that the noopLogger
// writes nothing and does not panic on any method. It is the
// logger used by tests that do not care about output.
func TestNoopLogger_DiscardsEverything(t *testing.T) {
	t.Parallel()

	logger := newNoopLogger()

	// None of these should panic.
	logger.Debug("debug")
	logger.Info("info")
	logger.Warn("warn")
	logger.Error("error")
	scoped := logger.With("key", "value")
	scoped.Info("scoped")

	// The zero-value noopLogger must also work.
	var zero noopLogger
	zero.Debug("zero")
	zero.With("a", 1).Info("zero scoped")
}

// TestNoopLogger_WithReturnsReceiver verifies that With on a
// noopLogger returns the receiver, not a new value. This is an
// allocation optimisation and a documented contract.
func TestNoopLogger_WithReturnsReceiver(t *testing.T) {
	t.Parallel()

	var l noopLogger
	scoped := l.With("k", "v")
	if scoped != l {
		t.Errorf("With returned a different logger; want the receiver")
	}
}

// --- Negative / edge / boundary cases -------------------------------

// TestBuildDependencies_EmptyRootPath verifies that an empty
// rootPath is accepted. The OSFS constructor does not require the
// directory to exist. This is an edge case: a caller that forgets
// to set rootPath must not panic; it must produce a bound FS that
// resolves paths against the empty string.
func TestBuildDependencies_EmptyRootPath(t *testing.T) {
	t.Parallel()

	opts := options{
		stdout:   &bytes.Buffer{},
		stderr:   &bytes.Buffer{},
		env:      func(string) string { return "" },
		rootPath: "",
	}

	deps := buildDependencies(opts)

	if deps.FS == nil {
		t.Fatal("FS is nil with empty rootPath")
	}
	if got := deps.FS.Boundary(); got != "" {
		t.Errorf("FS.Boundary() = %q; want empty", got)
	}
}

// TestBuildDependencies_NilEnvPanicsOnCall verifies that a nil Env
// function produces a Dependencies value whose Env field panics when
// called. This is the boundary case: buildDependencies does not
// guard against a nil function because the options struct is
// constructed by executeWithOptions, which always sets a non-nil
// function. The test documents the behaviour so that a future
// refactor does not silently change it.
func TestBuildDependencies_NilEnvPanicsOnCall(t *testing.T) {
	t.Parallel()

	opts := options{
		stdout:   &bytes.Buffer{},
		stderr:   &bytes.Buffer{},
		env:      nil,
		rootPath: t.TempDir(),
	}

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

// TestBuildDependencies_NilWritersPanicOnUse verifies that nil
// writers produce a Dependencies value whose Stdout and Stderr panic
// on write. As with Env, buildDependencies does not guard against
// nil because executeWithOptions always supplies writers.
func TestBuildDependencies_NilWritersPanicOnUse(t *testing.T) {
	t.Parallel()

	opts := options{
		stdout:   nil,
		stderr:   nil,
		env:      func(string) string { return "" },
		rootPath: t.TempDir(),
	}

	deps := buildDependencies(opts)

	t.Run("stdout", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("write to nil Stdout did not panic")
			}
		}()
		_, _ = deps.Stdout.Write([]byte("x"))
	})

	t.Run("stderr", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("write to nil Stderr did not panic")
			}
		}()
		_, _ = deps.Stderr.Write([]byte("x"))
	})
}

// TestDependencies_ZeroValueIsDocumented verifies the documented
// zero-value behaviour: Config is safe to read, but Logger, FS,
// Stdout, Stderr, and Env are unusable. This test pins the contract
// so that a future change to the struct cannot silently make the
// zero value useful (which would hide construction bugs).
func TestDependencies_ZeroValueIsDocumented(t *testing.T) {
	t.Parallel()

	var deps Dependencies

	// Config is a nil pointer; reading its (empty) fields is safe
	// because the struct has no fields. This assertion is a
	// compile-time check more than a runtime one.
	if deps.Config != nil {
		t.Error("zero-value Config is non-nil; want nil")
	}

	// Logger is nil and must panic on use.
	if deps.Logger != nil {
		t.Error("zero-value Logger is non-nil; want nil")
	}

	// FS is nil and must panic on use.
	if deps.FS != nil {
		t.Error("zero-value FS is non-nil; want nil")
	}

	// Writers are nil and must panic on use.
	if deps.Stdout != nil {
		t.Error("zero-value Stdout is non-nil; want nil")
	}
	if deps.Stderr != nil {
		t.Error("zero-value Stderr is non-nil; want nil")
	}

	// Env is nil and must panic on call.
	if deps.Env != nil {
		t.Error("zero-value Env is non-nil; want nil")
	}
}

// TestDependencies_IsPassedByValue verifies that copying a
// Dependencies value and mutating the copy does not affect the
// original. This is the contract that lets commands receive the
// struct by value without risking cross-command interference. Note
// that the fields themselves (Config, FS) are shared; only the
// struct header is copied.
func TestDependencies_IsPassedByValue(t *testing.T) {
	t.Parallel()

	original := Dependencies{
		Stdout: &bytes.Buffer{},
	}
	copy := original
	copy.Stdout = &bytes.Buffer{}

	if original.Stdout == copy.Stdout {
		t.Error("mutating the copy changed the original; want independent headers")
	}
}

// TestLogger_InterfaceStability is a BDD-style test written in
// Given/When/Then form. It documents the five-method contract that
// every Logger implementation must satisfy. If a future refactor
// adds a method to Logger, this test fails to compile, forcing the
// author to update every implementation.
func TestLogger_InterfaceStability(t *testing.T) {
	t.Parallel()

	// Given a Logger backed by a buffer.
	var buf bytes.Buffer
	var logger Logger = newLogger(&buf)

	// When every method is called.
	logger.Debug("d", "k", "v")
	logger.Info("i", "k", "v")
	logger.Warn("w", "k", "v")
	logger.Error("e", "k", "v")
	scoped := logger.With("k", "v")
	scoped.Info("s")

	// Then the output contains every message.
	out := buf.String()
	for _, want := range []string{"msg=d", "msg=i", "msg=w", "msg=e", "msg=s"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q: %q", want, out)
		}
	}
}

// --- Non-functional cases -------------------------------------------

// TestBuildDependencies_DoesNotAllocatePerCall is a non-functional
// test that verifies buildDependencies does not retain state across
// calls. Two calls with the same options must produce independent
// Dependencies values. This guards against a future refactor that
// caches a package-level value (which would break test isolation).
//
// # Why the Config check is conditional
//
// In Phase 2, config.Config is an empty struct and
// &config.Config{} is a zero-size allocation, so all such pointers
// are equal. The pointer-inequality check for Config is therefore
// skipped while the type is zero-size, and re-enabled automatically
// when WBS 8.2.1 adds fields. See configIsZeroSized.
func TestBuildDependencies_DoesNotAllocatePerCall(t *testing.T) {
	t.Parallel()

	mkOpts := func() options {
		return options{
			stdout:   &bytes.Buffer{},
			stderr:   &bytes.Buffer{},
			env:      func(string) string { return "" },
			rootPath: t.TempDir(),
		}
	}

	a := buildDependencies(mkOpts())
	b := buildDependencies(mkOpts())

	if a.Stdout == b.Stdout {
		t.Error("two calls shared Stdout; want independent writers")
	}
	if a.FS == b.FS {
		t.Error("two calls shared FS; want independent filesystems")
	}
	if !configIsZeroSized() && a.Config == b.Config {
		t.Error("two calls shared Config; want independent configs")
	}
}

// TestBuildDependencies_LoggerWritesToStderr is a non-functional
// test that verifies the logger writes to the injected stderr, not
// to the process's real stderr. This is the property that keeps
// stdout clean for command output and makes tests hermetic.
func TestBuildDependencies_LoggerWritesToStderr(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	opts := options{
		stdout:   &stdout,
		stderr:   &stderr,
		env:      func(string) string { return "" },
		rootPath: t.TempDir(),
	}

	deps := buildDependencies(opts)
	deps.Logger.Info("diagnostic")

	if stdout.Len() != 0 {
		t.Errorf("logger wrote to stdout; want stderr only: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "diagnostic") {
		t.Errorf("logger did not write to stderr: %q", stderr.String())
	}
}

// TestBuildDependencies_ConfigIsIndependentOfLogger verifies that
// constructing a Dependencies value does not cache state across
// calls.
//
// # Why pointer identity is not asserted
//
// In Phase 2, config.Config is an empty struct. Taking the address
// of an empty struct literal (&config.Config{}) returns Go's
// runtime.zerobase — the same address for every zero-size
// allocation. Asserting first.Config != second.Config would
// therefore fail, not because the code shares state, but because
// Go's runtime gives all zero-size allocations the same address.
//
// When WBS 8.2.1 adds fields to config.Config, the struct becomes
// non-zero-size, and the two pointers become distinct. At that
// point, configIsZeroSized returns false and the pointer-inequality
// assertion below becomes meaningful.
func TestBuildDependencies_ConfigIsIndependentOfLogger(t *testing.T) {
	t.Parallel()

	opts := options{
		stdout:   &bytes.Buffer{},
		stderr:   &bytes.Buffer{},
		env:      func(string) string { return "" },
		rootPath: t.TempDir(),
	}

	first := buildDependencies(opts)
	second := buildDependencies(opts)

	// Every call must produce a non-nil Config, regardless of the
	// struct's size.
	if first.Config == nil {
		t.Error("first.Config is nil; want non-nil")
	}
	if second.Config == nil {
		t.Error("second.Config is nil; want non-nil")
	}

	// The pointer-inequality check is meaningful only when the
	// struct is non-zero-size. It is written as a conditional so
	// that it becomes active automatically when WBS 8.2.1 lands.
	if !configIsZeroSized() && first.Config == second.Config {
		t.Error("Config pointer shared between calls; want independent values")
	}

	_ = errors.New // keep the errors import for future negative cases
}
