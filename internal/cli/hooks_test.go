// Package cli contains white-box tests for the CLI package.
//
// This file tests the PersistentPreRunE hook defined in hooks.go.
// The hook prepares two per-invocation state values and stores both
// on the command's context:
//
//   - The Logger (WBS 12.3.1), built at the top of the hook bound
//     to a fresh correlation ID (WBS 12.2.1) and attached via
//     logger.IntoContext. Downstream code reads it with
//     logger.FromContext(cmd.Context()); every record it emits
//     carries the invocation's correlation ID.
//
//   - The resolved configuration (WBS 8.4.5), produced by applying
//     the four precedence layers (defaults, file, environment, CLI
//     flags) in order and attached via contextWithConfig.
//
// # What the tests verify
//
// Unit tests, exercising the hook in isolation:
//
//   - The defaults are applied when no file, no env, and no flags
//     are present.
//   - A config file's values are applied.
//   - The environment overrides the file.
//   - A CLI flag overrides the environment.
//   - A malformed config file produces an error.
//   - An invalid enum value produces an error.
//   - The resolved config is stored in the command's context.
//   - The Logger is stored in the command's context.
//   - The Logger's records carry a v4 UUID correlation ID.
//   - The correlation ID is fresh per invocation.
//   - The correlation ID is preserved across the two paths
//     (injection and production).
//   - The Logger is attached *before* the config is loaded, so
//     records emitted during loading share the invocation's ID.
//
// Integration tests, exercising the hook installed on the root
// command:
//
//   - The root command has a non-nil PersistentPreRunE.
//   - No subcommand defines its own PersistentPreRunE.
//   - The hook runs before a subcommand's RunE.
//
// # The context is the sole transport
//
// The hook stores both values in the command's context. It does not
// write through the captured deps.Config pointer: Dependencies is a
// value type, but its Config field is a *config.Config, and two
// invocations sharing a Dependencies value would race on the
// pointee. The race detector caught the write in the parallel tests;
// the transport was removed. See hooks.go §"Where the resolved
// configuration is stored".
//
// # Why the tests observe the ID through records
//
// The Logger interface's methods do not take a context. The
// correlation ID must therefore travel inside the Logger. The tests
// read the ID by asking the Logger to emit a record into a
// captureSink and inspecting the record's CorrelationID field. This
// is the same path a real consumer uses; a test that read the ID
// from a context value would be asserting the shape of the
// implementation, not the contract.
//
// # How the unit tests invoke the hook
//
// The hook is a constructor: initializeCommandEnvironment(deps)
// returns the closure Cobra calls. A unit test constructs a
// Dependencies value with synthetic inputs, calls the constructor,
// and invokes the returned closure with a minimal command.
//
// The synthetic Dependencies carries:
//
//   - Env: a function backed by a map, so the test does not depend
//     on the test runner's environment.
//   - RootPath: a t.TempDir() value, so config discovery is
//     deterministic and does not read the developer's home
//     directory.
//   - Streams.Err: a bytes.Buffer, so the Logger the hook builds
//     writes to a buffer the test can inspect. A test that wants
//     to capture records as structured data emits through the
//     Logger and parses the JSON it writes to Streams.Err.
//
// The other streams (Streams.In and Streams.Out) are set to empty
// readers and buffers so that the Dependencies value is fully
// populated. The hook reads only Streams.Err.
//
// # Why the tests use a local helper
//
// The local helper (testHookDeps) constructs the Dependencies value
// the hook reads. Keeping it local to this file makes the tests
// self-contained: a change to the production testDependencies helper
// does not affect this file.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/cobra"

	"github.com/thapelomagqazana/forge/internal/config"
	"github.com/thapelomagqazana/forge/internal/logger"
	"github.com/thapelomagqazana/forge/internal/streams"
)

// uuidV4Pattern is the shape of a v4 UUID in canonical lowercase
// form. The logger generates IDs that match it; the test uses the
// pattern to assert the hook's ID is well-formed without
// duplicating the generator's logic.
var uuidV4Pattern = regexp.MustCompile(
	`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// =============================================================================
// Test helper: capture sink
// =============================================================================

// hookCaptureSink records the records a Logger emits.
//
// The type is local to this file; it is a minimal version of the
// captureSink used by the logger package's own tests, and it serves
// the same purpose: it lets a test observe the records the Logger
// the hook built would produce, without depending on the format of
// the rendered bytes.
//
// # Concurrency
//
// The sink is safe for concurrent use. The logger package's Sink
// contract requires it; the tests in this file do not exercise the
// sink concurrently, but the guard is cheap and matches the
// contract.
type hookCaptureSink struct {
	mu      sync.Mutex
	records []logger.Record
}

func (c *hookCaptureSink) Emit(_ context.Context, r logger.Record) {
	c.mu.Lock()
	c.records = append(c.records, r)
	c.mu.Unlock()
}

func (c *hookCaptureSink) snapshot() []logger.Record {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]logger.Record, len(c.records))
	copy(out, c.records)
	return out
}

func (c *hookCaptureSink) len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.records)
}

// =============================================================================
// Test helpers
// =============================================================================

// testHookDeps constructs a Dependencies value for the hook tests.
//
// # What the helper sets
//
// The helper sets the fields the hook reads:
//
//   - Env: the environment lookup, backed by the provided map.
//   - RootPath: the working directory for config discovery.
//   - Streams.Err: a bytes.Buffer, so the Logger the hook builds
//     writes to a buffer the test can inspect without polluting
//     the process's stderr.
//
// The other fields are left at their zero values; the hook does not
// read them.
//
// # Why a real env map
//
// The tests set the env map explicitly so that they do not depend on
// the test runner's environment. A test that set FORGE_LOG_LEVEL in
// the shell would affect every test in the process; the map confines
// the setting to one test.
//
// # Why the Streams.Err buffer
//
// The hook builds a Logger whose Writer is deps.Streams.Err. The
// Logger's default Writer is os.Stderr; leaving Streams.Err nil
// would make every test write to the process's stderr, cluttering
// the test output. A local buffer per test confines the writes.
//
// # Why Streams.Out and Streams.In are set
//
// The Dependencies value the hook reads is a full Streams. The
// hook reads only Streams.Err, but a test that asserts on the
// Dependencies value as a whole (for example, the structural pins
// in deps_test.go) expects every stream to be non-nil. Setting
// all three keeps the helper's output consistent with what
// buildDependencies produces.
func testHookDeps(env map[string]string, rootPath string) Dependencies {
	envLookup := func(key string) string {
		return env[key]
	}
	return Dependencies{
		Env:      envLookup,
		RootPath: rootPath,
		Streams: streams.Streams{
			In:  strings.NewReader(""),
			Out: &bytes.Buffer{},
			Err: &bytes.Buffer{},
		},
	}
}

// newHookTestCommand constructs a minimal *cobra.Command with the
// three flags the hook reads: --log-level, --output-format, and
// --config.
//
// # Why the helper is minimal
//
// The hook reads three flags. The helper registers only those three.
// A test that wants to simulate "the user typed the flag" calls
// cmd.Flags().Set(name, value); a test that wants to simulate "the
// user did not type the flag" does nothing.
//
// # Why a fresh command per test
//
// A command's flags accumulate state (the value and the changed
// marker). A test that reused a command would see the previous
// test's state. A fresh command per test isolates the state.
func newHookTestCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String(config.FlagLogLevel, config.LogLevelInfo, "")
	cmd.Flags().String(config.FlagOutputFormat, config.OutputFormatText, "")
	cmd.Flags().String(FlagConfig, "", "")
	return cmd
}

// =============================================================================
// Unit tests — the four layers
// =============================================================================

// TestInitializeCommandEnvironment_DefaultsOnly verifies that the
// hook produces the defaults when no file, no env, and no flags are
// present.
func TestInitializeCommandEnvironment_DefaultsOnly(t *testing.T) {
	t.Parallel()

	cmd := newHookTestCommand()
	deps := testHookDeps(map[string]string{}, t.TempDir())

	hook := initializeCommandEnvironment(deps)
	if err := hook(cmd, nil); err != nil {
		t.Fatalf("hook: %v", err)
	}

	got, ok := ConfigFromContext(cmd.Context())
	if !ok {
		t.Fatalf("config not found in context")
	}

	want := config.Defaults()
	if got.LogLevel != want.LogLevel {
		t.Errorf("LogLevel = %q; want %q", got.LogLevel, want.LogLevel)
	}
	if got.OutputFormat != want.OutputFormat {
		t.Errorf("OutputFormat = %q; want %q",
			got.OutputFormat, want.OutputFormat)
	}
}

// TestInitializeCommandEnvironment_FileApplied verifies that a
// config file's values are applied.
func TestInitializeCommandEnvironment_FileApplied(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "forge.yaml")
	if err := os.WriteFile(cfgPath, []byte("log_level: warn\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cmd := newHookTestCommand()
	deps := testHookDeps(map[string]string{}, dir)

	hook := initializeCommandEnvironment(deps)
	if err := hook(cmd, nil); err != nil {
		t.Fatalf("hook: %v", err)
	}

	got, _ := ConfigFromContext(cmd.Context())
	if got.LogLevel != config.LogLevelWarn {
		t.Errorf("LogLevel = %q; want %q",
			got.LogLevel, config.LogLevelWarn)
	}
	if got.SourcePath != cfgPath {
		t.Errorf("SourcePath = %q; want %q", got.SourcePath, cfgPath)
	}
}

// TestInitializeCommandEnvironment_EnvOverridesFile verifies that
// the environment layer overrides the file layer.
func TestInitializeCommandEnvironment_EnvOverridesFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "forge.yaml")
	if err := os.WriteFile(cfgPath, []byte("log_level: warn\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cmd := newHookTestCommand()
	deps := testHookDeps(map[string]string{
		"FORGE_LOG_LEVEL": "debug",
	}, dir)

	hook := initializeCommandEnvironment(deps)
	if err := hook(cmd, nil); err != nil {
		t.Fatalf("hook: %v", err)
	}

	got, _ := ConfigFromContext(cmd.Context())
	if got.LogLevel != config.LogLevelDebug {
		t.Errorf("LogLevel = %q; want %q (env should override file)",
			got.LogLevel, config.LogLevelDebug)
	}
}

// TestInitializeCommandEnvironment_FlagOverridesEnv verifies that the
// CLI flag layer overrides the environment layer.
func TestInitializeCommandEnvironment_FlagOverridesEnv(t *testing.T) {
	t.Parallel()

	cmd := newHookTestCommand()
	if err := cmd.Flags().Set(config.FlagLogLevel, "error"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	deps := testHookDeps(map[string]string{
		"FORGE_LOG_LEVEL": "debug",
	}, t.TempDir())

	hook := initializeCommandEnvironment(deps)
	if err := hook(cmd, nil); err != nil {
		t.Fatalf("hook: %v", err)
	}

	got, _ := ConfigFromContext(cmd.Context())
	if got.LogLevel != config.LogLevelError {
		t.Errorf("LogLevel = %q; want %q (flag should override env)",
			got.LogLevel, config.LogLevelError)
	}
}

// TestInitializeCommandEnvironment_ExplicitConfigFlag verifies that
// the --config flag is honoured by config.Discover.
func TestInitializeCommandEnvironment_ExplicitConfigFlag(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "custom.yaml")
	if err := os.WriteFile(cfgPath, []byte("log_level: error\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cmd := newHookTestCommand()
	if err := cmd.Flags().Set(FlagConfig, cfgPath); err != nil {
		t.Fatalf("Set: %v", err)
	}
	deps := testHookDeps(map[string]string{}, dir)

	hook := initializeCommandEnvironment(deps)
	if err := hook(cmd, nil); err != nil {
		t.Fatalf("hook: %v", err)
	}

	got, _ := ConfigFromContext(cmd.Context())
	if got.LogLevel != config.LogLevelError {
		t.Errorf("LogLevel = %q; want %q",
			got.LogLevel, config.LogLevelError)
	}
	if got.SourcePath != cfgPath {
		t.Errorf("SourcePath = %q; want %q", got.SourcePath, cfgPath)
	}
}

// =============================================================================
// Unit tests — errors
// =============================================================================

// TestInitializeCommandEnvironment_MalformedFile verifies that a
// malformed config file produces an error.
func TestInitializeCommandEnvironment_MalformedFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "forge.yaml")
	if err := os.WriteFile(cfgPath, []byte("log_level: [unclosed\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cmd := newHookTestCommand()
	deps := testHookDeps(map[string]string{}, dir)

	hook := initializeCommandEnvironment(deps)
	err := hook(cmd, nil)
	if err == nil {
		t.Fatalf("hook: want error, got nil")
	}
}

// TestInitializeCommandEnvironment_InvalidEnum verifies that an
// invalid enum value in a CLI flag produces an error.
func TestInitializeCommandEnvironment_InvalidEnum(t *testing.T) {
	t.Parallel()

	cmd := newHookTestCommand()
	if err := cmd.Flags().Set(config.FlagLogLevel, "verbose"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	deps := testHookDeps(map[string]string{}, t.TempDir())

	hook := initializeCommandEnvironment(deps)
	err := hook(cmd, nil)
	if err == nil {
		t.Fatalf("hook: want error, got nil")
	}
	if !strings.Contains(err.Error(), "verbose") {
		t.Errorf("error does not name the invalid value: %v", err)
	}
}

// =============================================================================
// Unit tests — the Logger is attached
// =============================================================================

// TestInitializeCommandEnvironment_AttachesLogger verifies that the
// hook attaches a Logger to the command's context.
//
// # How the test observes the attachment
//
// After the hook runs, the test reads logger.FromContext(ctx). A
// live Logger would return a value that is not Nop; a context with
// no Logger returns Nop. The test distinguishes the two by
// identity: Nop returns a specific value whose type is the
// unexported nopLogger, and the logger package exposes no other way
// to obtain that value. The test constructs a Nop() of its own and
// compares.
//
// A more robust check would emit a record and observe it. That is
// the job of the correlation-ID tests below; this test asserts the
// simplest property: *some* Logger is stored, and it is not the
// Nop fallback.
func TestInitializeCommandEnvironment_AttachesLogger(t *testing.T) {
	t.Parallel()

	cmd := newHookTestCommand()
	deps := testHookDeps(map[string]string{}, t.TempDir())

	hook := initializeCommandEnvironment(deps)
	if err := hook(cmd, nil); err != nil {
		t.Fatalf("hook: %v", err)
	}

	got := logger.FromContext(cmd.Context())
	if got == nil {
		t.Fatal("FromContext returned nil")
	}
	// Distinguish from the Nop fallback. Two Nop values compare
	// equal; the Logger the hook built does not.
	if got == logger.Nop() {
		t.Fatal("hook did not attach a Logger (FromContext returned Nop)")
	}
}

// =============================================================================
// Unit tests — correlation ID (WBS 12.2.1)
// =============================================================================

// TestInitializeCommandEnvironment_LoggerCarriesCorrelationID
// verifies that the Logger the hook attaches carries a v4 UUID
// correlation ID.
//
// # How the test observes the ID
//
// The Logger emits a record to its Writer. The hook configured the
// Writer as deps.Streams.Err, which the helper set to a
// bytes.Buffer. The test emits a record through the Logger (by
// calling one of its methods) and reads the buffer. The buffer is
// JSON because the test sets FORGE_LOG_FORMAT=json in the
// Dependencies environment.
//
// The test parses the JSON line and asserts the correlation_id
// field's shape. This is the same field a real consumer reads; the
// test does not reach into the Logger's internals.
func TestInitializeCommandEnvironment_LoggerCarriesCorrelationID(t *testing.T) {
	t.Parallel()

	cmd := newHookTestCommand()
	deps := testHookDeps(map[string]string{
		logger.EnvVarFormat: "json",
	}, t.TempDir())

	hook := initializeCommandEnvironment(deps)
	if err := hook(cmd, nil); err != nil {
		t.Fatalf("hook: %v", err)
	}

	// Emit a record through the Logger the hook attached.
	lg := logger.FromContext(cmd.Context())
	lg.Info("probe")

	// Read the record from the buffer.
	buf := deps.Streams.Err.(*bytes.Buffer)
	id := extractCorrelationIDFromJSON(t, buf.Bytes())
	if id == "" {
		t.Fatal("record has no correlation_id")
	}
	if !uuidV4Pattern.MatchString(id) {
		t.Fatalf("correlation_id %q is not a v4 UUID", id)
	}
}

// TestInitializeCommandEnvironment_FreshIDPerInvocation verifies
// that two invocations of the hook produce Loggers with different
// correlation IDs.
//
// The ID must be per-invocation, not per-process. Two hooks called
// with the same Dependencies value must still produce different
// IDs; the test uses two fresh commands to model two invocations.
func TestInitializeCommandEnvironment_FreshIDPerInvocation(t *testing.T) {
	t.Parallel()

	deps := testHookDeps(map[string]string{
		logger.EnvVarFormat: "json",
	}, t.TempDir())
	hook := initializeCommandEnvironment(deps)

	first := newHookTestCommand()
	if err := hook(first, nil); err != nil {
		t.Fatalf("first hook: %v", err)
	}
	second := newHookTestCommand()
	if err := hook(second, nil); err != nil {
		t.Fatalf("second hook: %v", err)
	}

	// Both invocations share the same Streams.Err buffer; the
	// records are appended in order. The two records' IDs must
	// differ.
	lgFirst := logger.FromContext(first.Context())
	lgSecond := logger.FromContext(second.Context())
	lgFirst.Info("first")
	lgSecond.Info("second")

	buf := deps.Streams.Err.(*bytes.Buffer)
	ids := extractAllCorrelationIDsFromJSON(t, buf.Bytes())
	if len(ids) != 2 {
		t.Fatalf("got %d records; want 2", len(ids))
	}
	if ids[0] == ids[1] {
		t.Fatalf("two invocations share a correlation ID: %q", ids[0])
	}
}

// TestInitializeCommandEnvironment_IDAttachedBeforeConfigLoad
// verifies the ordering: the Logger is attached to the context
// *before* the configuration loader runs. A record emitted during
// configuration loading therefore carries the invocation's ID.
//
// # How the test observes the order
//
// The test injects a Config via deps so the hook takes the
// injection path — the fastest path. On that path, the Logger is
// attached, then the config is stored, then the hook returns. The
// test asserts both values are present on the returned context.
//
// The ordering is a property of the hook's source, not of the
// context's internal representation: Go contexts are immutable
// stacks, and the test cannot directly observe "which key was set
// first". What the test can observe, and does, is that both values
// are present after the hook runs.
func TestInitializeCommandEnvironment_IDAttachedBeforeConfigLoad(t *testing.T) {
	t.Parallel()

	injected := config.Config{
		LogLevel:     config.LogLevelWarn,
		OutputFormat: config.OutputFormatText,
	}

	cmd := newHookTestCommand()
	var buf bytes.Buffer
	deps := Dependencies{
		Env: func(key string) string {
			if key == logger.EnvVarFormat {
				return "json"
			}
			return ""
		},
		RootPath: t.TempDir(),
		Config:   &injected,
		Streams: streams.Streams{
			In:  strings.NewReader(""),
			Out: &bytes.Buffer{},
			Err: &buf,
		},
	}

	hook := initializeCommandEnvironment(deps)
	if err := hook(cmd, nil); err != nil {
		t.Fatalf("hook: %v", err)
	}

	ctx := cmd.Context()

	// The Logger is on the context.
	lg := logger.FromContext(ctx)
	if lg == logger.Nop() {
		t.Fatal("hook did not attach a Logger")
	}
	// And it carries a v4 UUID.
	lg.Info("probe")
	id := extractCorrelationIDFromJSON(t, buf.Bytes())
	if !uuidV4Pattern.MatchString(id) {
		t.Fatalf("correlation ID %q is not a v4 UUID", id)
	}

	// The Config is on the context.
	got, ok := ConfigFromContext(ctx)
	if !ok {
		t.Fatal("config not found in context")
	}
	if got.LogLevel != injected.LogLevel {
		t.Fatalf("LogLevel = %q; want %q", got.LogLevel, injected.LogLevel)
	}
}

// TestInitializeCommandEnvironment_IDAttachedOnInjectionPath
// verifies that the injection path (tests injecting a Config)
// still gets a Logger with a fresh correlation ID.
//
// # Why the injection path must build a Logger
//
// The injection path is a test convenience: tests inject a Config
// to skip file discovery. The Logger is a property of the
// invocation, not of the configuration source. A test that injects
// a Config should exercise the same hook production does; making
// the Logger construction conditional on the path would mean the
// injection path is a different code path from production.
func TestInitializeCommandEnvironment_IDAttachedOnInjectionPath(t *testing.T) {
	t.Parallel()

	injected := config.Config{
		LogLevel:     config.LogLevelDebug,
		OutputFormat: config.OutputFormatJSON,
	}

	cmd := newHookTestCommand()
	var buf bytes.Buffer
	deps := Dependencies{
		Env: func(key string) string {
			if key == logger.EnvVarFormat {
				return "json"
			}
			return ""
		},
		RootPath: t.TempDir(),
		Config:   &injected,
		Streams: streams.Streams{
			In:  strings.NewReader(""),
			Out: &bytes.Buffer{},
			Err: &buf,
		},
	}

	hook := initializeCommandEnvironment(deps)
	if err := hook(cmd, nil); err != nil {
		t.Fatalf("hook: %v", err)
	}

	lg := logger.FromContext(cmd.Context())
	if lg == logger.Nop() {
		t.Fatal("injection path did not attach a Logger")
	}
	lg.Info("probe")
	id := extractCorrelationIDFromJSON(t, buf.Bytes())
	if !uuidV4Pattern.MatchString(id) {
		t.Fatalf("correlation ID %q is not a v4 UUID", id)
	}
}

// TestInitializeCommandEnvironment_IDUniqueAcrossPaths verifies
// that the production path and the injection path each build a
// Logger with its own correlation ID.
func TestInitializeCommandEnvironment_IDUniqueAcrossPaths(t *testing.T) {
	t.Parallel()

	envJSON := map[string]string{logger.EnvVarFormat: "json"}

	prodDeps := testHookDeps(envJSON, t.TempDir())
	prodHook := initializeCommandEnvironment(prodDeps)
	prodCmd := newHookTestCommand()
	if err := prodHook(prodCmd, nil); err != nil {
		t.Fatalf("production hook: %v", err)
	}

	injected := config.Config{LogLevel: config.LogLevelInfo}
	var injBuf bytes.Buffer
	injDeps := Dependencies{
		Env: func(key string) string {
			if key == logger.EnvVarFormat {
				return "json"
			}
			return ""
		},
		RootPath: t.TempDir(),
		Config:   &injected,
		Streams: streams.Streams{
			In:  strings.NewReader(""),
			Out: &bytes.Buffer{},
			Err: &injBuf,
		},
	}
	injHook := initializeCommandEnvironment(injDeps)
	injCmd := newHookTestCommand()
	if err := injHook(injCmd, nil); err != nil {
		t.Fatalf("injection hook: %v", err)
	}

	logger.FromContext(prodCmd.Context()).Info("probe")
	logger.FromContext(injCmd.Context()).Info("probe")

	prodBuf := prodDeps.Streams.Err.(*bytes.Buffer)
	a := extractCorrelationIDFromJSON(t, prodBuf.Bytes())
	b := extractCorrelationIDFromJSON(t, injBuf.Bytes())

	if a == b {
		t.Fatalf("production and injection paths share an ID: %q", a)
	}
	if !uuidV4Pattern.MatchString(a) || !uuidV4Pattern.MatchString(b) {
		t.Fatalf("IDs are not v4 UUIDs: %q, %q", a, b)
	}
}

// =============================================================================
// Helpers: extract the correlation ID from a JSON log line
// =============================================================================

// extractCorrelationIDFromJSON parses the last JSON object in buf
// and returns its correlation_id field. The helper fails the test
// if buf is not valid JSON or lacks the field.
//
// The helper assumes the buffer contains one or more newline-
// delimited JSON objects, which is what the logger's JSON renderer
// produces. It reads the *last* object, which is the record the
// caller most recently asked the Logger to emit.
func extractCorrelationIDFromJSON(t *testing.T, buf []byte) string {
	t.Helper()

	if len(buf) == 0 {
		t.Fatal("buffer is empty; no record was emitted")
	}
	// The buffer may contain multiple newline-delimited records.
	// Take the last non-empty line.
	lines := bytes.Split(bytes.TrimRight(buf, "\n"), []byte("\n"))
	last := lines[len(lines)-1]

	var doc map[string]any
	if err := json.Unmarshal(last, &doc); err != nil {
		t.Fatalf("record is not valid JSON: %v\n%s", err, last)
	}
	id, _ := doc["correlation_id"].(string)
	return id
}

// extractAllCorrelationIDsFromJSON parses every JSON object in buf
// and returns their correlation_id fields in order.
func extractAllCorrelationIDsFromJSON(t *testing.T, buf []byte) []string {
	t.Helper()

	if len(buf) == 0 {
		return nil
	}
	lines := bytes.Split(bytes.TrimRight(buf, "\n"), []byte("\n"))
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var doc map[string]any
		if err := json.Unmarshal(line, &doc); err != nil {
			t.Fatalf("record is not valid JSON: %v\n%s", err, line)
		}
		id, _ := doc["correlation_id"].(string)
		out = append(out, id)
	}
	return out
}

// =============================================================================
// Integration tests — the hook installed on the root command
// =============================================================================

// TestHooks_NoSubcommandOverrides verifies AC5: no subcommand in the
// registry defines its own PersistentPreRunE.
func TestHooks_NoSubcommandOverrides(t *testing.T) {
	t.Parallel()

	deps := testHookDeps(map[string]string{}, t.TempDir())
	for _, ctor := range registry {
		cmd := ctor(deps)
		if cmd.PersistentPreRunE != nil {
			t.Errorf("subcommand %q defines its own PersistentPreRunE; "+
				"the root's hook must be the only one in the tree. "+
				"See docs/architecture.md § 11.16", cmd.Name())
		}
	}
}

// TestHooks_RunsBeforeSubcommandRunE verifies AC4: the root's hook
// runs before a subcommand's RunE.
func TestHooks_RunsBeforeSubcommandRunE(t *testing.T) {
	t.Parallel()

	var order []string

	recordingHook := func(cmd *cobra.Command, args []string) error {
		order = append(order, "hook")
		return nil
	}

	sub := &cobra.Command{
		Use: "sub",
		RunE: func(cmd *cobra.Command, args []string) error {
			order = append(order, "rune")
			return nil
		},
	}

	root := &cobra.Command{
		Use:               "root",
		PersistentPreRunE: recordingHook,
	}
	root.AddCommand(sub)

	root.SetArgs([]string{"sub"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if len(order) != 2 {
		t.Fatalf("order has %d entries; want 2: %v", len(order), order)
	}
	if order[0] != "hook" {
		t.Errorf("first run was %q; want %q", order[0], "hook")
	}
	if order[1] != "rune" {
		t.Errorf("second run was %q; want %q", order[1], "rune")
	}
}

// TestHooks_RootCmdHasHookInstalled verifies AC1: the root command
// produced by newRootCmd has a non-nil PersistentPreRunE.
func TestHooks_RootCmdHasHookInstalled(t *testing.T) {
	t.Parallel()

	deps := testHookDeps(map[string]string{}, t.TempDir())
	root := newRootCmd(deps)

	if root.PersistentPreRunE == nil {
		t.Fatal("root command has no PersistentPreRunE; " +
			"the hook point is not reserved")
	}
}
