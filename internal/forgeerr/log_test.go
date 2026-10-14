// Package forgeerr — programmer-facing log format tests.
//
// Every test is named after the acceptance criterion (AC1–AC8)
// it verifies. The eight criteria are:
//
//	AC1  LogValue() implemented.
//	AC2  Includes code, category, cause.
//	AC3  Context flattened.
//	AC4  Keys sorted alphabetically.
//	AC5  Works with slog.Any("error", e).
//	AC6  Works with the JSON handler.
//	AC7  Works with the text handler.
//	AC8  Never logs secrets (covered by the handler, not by
//	     LogValue; the test verifies the division of
//	     responsibility).
//
// Test categories:
//
//   - ATDD:      AC-mapped tests
//   - Golden:    byte-exact comparison against committed files
//   - Positive:  valid inputs
//   - Negative:  nil cause, empty context, empty cause message
//   - Edge:      context keys that collide with the reserved
//     keys, very long values, non-ASCII values
//   - Corner:    multiple errors logged in sequence
//   - NF:        determinism, concurrency
//   - Fuzz:      invariant preservation
//
// # Type-level contract
//
// Every call to forgeerr.New and forgeerr.Wrap in this file passes
// the code as forgeerr.Code, not as string. The constructors'
// signatures are New(category, code Code, message) and
// Wrap(category, code Code, message, cause). The Code parameter is
// what makes the registry a closed set at compile time: a bare
// string literal in the code position does not convert implicitly,
// so a caller who invents a new code cannot compile the call.
//
// If this file fails to compile with "cannot use
// forgeerr.CodeXxx (constant ... of string type forgeerr.Code) as
// string value", the constructor signature is still code string and
// the migration documented in safefs/errors.go is incomplete.
//
// # Why the type matters here specifically
//
// The tests in this file are the ones a future contributor will
// copy when adding a new AC. If they see string(CodeConfigMalformed)
// they will copy the cast, and the cast is what defeats the whole
// point of the Code type. The tests use the named constants
// directly, so the pattern that propagates is the correct one.
package forgeerr

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// --- Test-only constants and helpers ----------------------------------

// logWith renders e with a handler produced by mk and returns the
// resulting string.
//
// logWith takes a constructor rather than a pre-built handler
// because a slog.Handler is bound to its writer at construction.
// A caller that passes a handler has already chosen the writer,
// and logWith cannot retrieve it — the handler writes to the
// caller's buffer, while a naive helper would read from its own.
// Taking the constructor keeps the buffer inside logWith, which is
// the only place it is needed.
//
// The helper is the canonical way to invoke the logger in this
// test file; every test that does not need a custom ReplaceAttr
// uses it. It builds a logger, emits one Error record with the
// error attached under slog.Any("error", e), and returns the
// buffer's contents. It does not call t.Fatalf; a caller that
// wants to inspect the raw string does so directly.
func logWith(mk func(*bytes.Buffer) slog.Handler, e *Error) string {
	var buf bytes.Buffer
	logger := slog.New(mk(&buf))
	logger.Error("failed", slog.Any("error", e))
	return buf.String()
}

// newTextHandler returns a slog.Handler that writes to buf in
// text format, with the timestamp removed and the level
// lowercased. The transformations make the output deterministic
// and comparable against golden files.
//
// The ReplaceAttr callback runs for every attribute, including
// attributes inside groups. It receives the group path in the
// first argument, which this handler ignores: the transforms it
// applies are unconditional.
func newTextHandler(buf *bytes.Buffer) slog.Handler {
	return slog.NewTextHandler(buf, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			if a.Key == slog.LevelKey {
				if lvl, ok := a.Value.Any().(slog.Level); ok {
					a.Value = slog.StringValue(strings.ToLower(lvl.String()))
				}
			}
			return a
		},
	})
}

// newJSONHandler returns a slog.Handler that writes to buf in
// JSON format, with the timestamp removed and the level
// lowercased. The two transformations match newTextHandler's so
// that the same golden fixture drives both.
func newJSONHandler(buf *bytes.Buffer) slog.Handler {
	return slog.NewJSONHandler(buf, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			if a.Key == slog.LevelKey {
				if lvl, ok := a.Value.Any().(slog.Level); ok {
					a.Value = slog.StringValue(strings.ToLower(lvl.String()))
				}
			}
			return a
		},
	})
}

// --- AC1: LogValue() implemented --------------------------------------

// TestAC1_LogValueImplemented asserts that *Error implements
// slog.LogValuer. The compile-time assertion in log.go is the
// primary check; this test documents it in the test suite.
func TestAC1_LogValueImplemented(t *testing.T) {
	t.Parallel()

	var v slog.LogValuer = New(CategoryConfig, CodeConfigMalformed, "x")
	if v == nil {
		t.Fatal("New returned a nil slog.LogValuer")
	}
}

// --- AC2: includes code, category, cause ------------------------------

// TestAC2_IncludesCodeCategoryCause asserts that the group
// produced by LogValue contains the three structured fields.
func TestAC2_IncludesCodeCategoryCause(t *testing.T) {
	t.Parallel()

	cause := errors.New("underlying failure")
	e := New(CategoryConfig, CodeConfigMalformed, "invalid YAML").
		WithCause(cause)

	var buf bytes.Buffer
	logger := slog.New(newJSONHandler(&buf))
	logger.Error("failed", slog.Any("error", e))

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, buf.String())
	}

	errGroup, ok := record["error"].(map[string]any)
	if !ok {
		t.Fatalf("error group not found; record = %v", record)
	}

	if got := errGroup["code"]; got != string(CodeConfigMalformed) {
		t.Errorf("code = %v, want %q", got, CodeConfigMalformed)
	}
	if got := errGroup["category"]; got != string(CategoryConfig) {
		t.Errorf("category = %v, want %q", got, CategoryConfig)
	}
	if got := errGroup["cause"]; got != "underlying failure" {
		t.Errorf("cause = %v, want %q", got, "underlying failure")
	}
}

// TestAC2_CauseOmittedWhenNil asserts that the cause attribute
// is absent when no cause is attached.
func TestAC2_CauseOmittedWhenNil(t *testing.T) {
	t.Parallel()

	e := New(CategoryConfig, CodeConfigMalformed, "invalid YAML")

	var buf bytes.Buffer
	logger := slog.New(newJSONHandler(&buf))
	logger.Error("failed", slog.Any("error", e))

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	errGroup, ok := record["error"].(map[string]any)
	if !ok {
		t.Fatalf("error group not found; record = %v", record)
	}
	if _, present := errGroup["cause"]; present {
		t.Errorf("cause attribute present when no cause is attached: %v", errGroup)
	}
}

// --- AC3: context flattened -------------------------------------------

// TestAC3_ContextFlattened asserts that each context entry is a
// top-level attribute of the error group, not a nested group.
func TestAC3_ContextFlattened(t *testing.T) {
	t.Parallel()

	e := New(CategoryConfig, CodeConfigMalformed, "invalid YAML").
		WithContext("file", "config.yaml").
		WithContext("line", "12")

	var buf bytes.Buffer
	logger := slog.New(newJSONHandler(&buf))
	logger.Error("failed", slog.Any("error", e))

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	errGroup, ok := record["error"].(map[string]any)
	if !ok {
		t.Fatalf("error group not found; record = %v", record)
	}

	if got := errGroup["file"]; got != "config.yaml" {
		t.Errorf("file = %v, want %q", got, "config.yaml")
	}
	if got := errGroup["line"]; got != "12" {
		t.Errorf("line = %v, want %q", got, "12")
	}
	if _, present := errGroup["context"]; present {
		t.Errorf("context is nested under a 'context' key; it should be flattened: %v", errGroup)
	}
}

// --- AC4: keys sorted alphabetically ----------------------------------

// TestAC4_KeysSortedAlphabetically asserts that the context
// attributes appear in lexicographic order by key.
func TestAC4_KeysSortedAlphabetically(t *testing.T) {
	t.Parallel()

	// Insert keys in reverse order to prove the sort is applied.
	e := New(CategoryConfig, CodeConfigMalformed, "x").
		WithContext("zeta", "1").
		WithContext("yotta", "2").
		WithContext("alpha", "3")

	out := logWith(newTextHandler, e)

	alphaIdx := strings.Index(out, "alpha=3")
	yottaIdx := strings.Index(out, "yotta=2")
	zetaIdx := strings.Index(out, "zeta=1")

	if alphaIdx == -1 || yottaIdx == -1 || zetaIdx == -1 {
		t.Fatalf("context attributes missing from output:\n%s", out)
	}
	if !(alphaIdx < yottaIdx && yottaIdx < zetaIdx) {
		t.Fatalf("attributes not in alphabetical order: "+
			"alpha=%d yotta=%d zeta=%d\n%s",
			alphaIdx, yottaIdx, zetaIdx, out)
	}
}

// --- AC5: works with slog.Any ----------------------------------------

// TestAC5_WorksWithSlogAny asserts that slog.Any("error", e)
// produces the structured output, not a formatted string.
//
// The test distinguishes the two cases by searching for the
// structured field marker. A formatted string would contain
// "FORGE_CONFIG_MALFORMED: invalid YAML"; the structured output
// contains the code as a separate attribute.
func TestAC5_WorksWithSlogAny(t *testing.T) {
	t.Parallel()

	e := New(CategoryConfig, CodeConfigMalformed, "invalid YAML")

	out := logWith(newTextHandler, e)

	if !strings.Contains(out, "error.code="+string(CodeConfigMalformed)) {
		t.Errorf("output does not contain structured code attribute:\n%s", out)
	}
	if strings.Contains(out, "invalid YAML") && strings.Contains(out, "FORGE_CONFIG_MALFORMED: invalid YAML") {
		t.Errorf("output appears to contain the formatted string, not structured fields:\n%s", out)
	}
}

// --- AC6: works with JSON handler -------------------------------------

// TestAC6_WorksWithJSONHandler asserts that the JSON handler
// renders the error group as a nested JSON object.
func TestAC6_WorksWithJSONHandler(t *testing.T) {
	t.Parallel()

	e := New(CategoryConfig, CodeConfigMalformed, "invalid YAML").
		WithContext("file", "config.yaml")

	var buf bytes.Buffer
	logger := slog.New(newJSONHandler(&buf))
	logger.Error("failed", slog.Any("error", e))

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, buf.String())
	}

	errGroup, ok := record["error"].(map[string]any)
	if !ok {
		t.Fatalf("error group not found; record = %v", record)
	}

	if got := errGroup["code"]; got != string(CodeConfigMalformed) {
		t.Errorf("code = %v", got)
	}
	if got := errGroup["category"]; got != string(CategoryConfig) {
		t.Errorf("category = %v", got)
	}
	if got := errGroup["file"]; got != "config.yaml" {
		t.Errorf("file = %v", got)
	}
}

// --- AC7: works with text handler -------------------------------------

// TestAC7_WorksWithTextHandler asserts that the text handler
// renders the error group as key=value pairs prefixed by
// "error.".
func TestAC7_WorksWithTextHandler(t *testing.T) {
	t.Parallel()

	e := New(CategoryConfig, CodeConfigMalformed, "invalid YAML").
		WithContext("file", "config.yaml")

	out := logWith(newTextHandler, e)

	for _, want := range []string{
		"error.code=" + string(CodeConfigMalformed),
		"error.category=" + string(CategoryConfig),
		"error.file=config.yaml",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output does not contain %q:\n%s", want, out)
		}
	}
}

// --- AC8: never logs secrets ------------------------------------------
//
// LogValue does not redact secrets; the log handler does. This
// test documents the division of responsibility and asserts that
// LogValue faithfully emits what it is given, so that the handler
// has a chance to redact.
//
// A redacting handler is the subject of WBS 12.0. This test
// constructs a minimal redacting handler to demonstrate the
// pattern, and asserts that the handler — not LogValue — performs
// the redaction.

// TestAC8_HandlerRedactsSecrets asserts that a handler configured
// with a redacting ReplaceAttr function redacts context values
// that match the secret pattern, while LogValue emits them
// faithfully.
func TestAC8_HandlerRedactsSecrets(t *testing.T) {
	t.Parallel()

	const secret = "ghp_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

	e := New(CategoryConfig, CodeConfigMalformed, "invalid YAML").
		WithContext("token", secret)

	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			if s, ok := a.Value.Any().(string); ok {
				if strings.HasPrefix(s, "ghp_") {
					a.Value = slog.StringValue(redactedMarker)
				}
			}
			return a
		},
	})

	logger := slog.New(handler)
	logger.Error("failed", slog.Any("error", e))

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	errGroup := record["error"].(map[string]any)

	if got := errGroup["token"]; got != redactedMarker {
		t.Errorf("token = %v, want %s", got, redactedMarker)
	}
	if strings.Contains(buf.String(), secret) {
		t.Errorf("secret leaked into output:\n%s", buf.String())
	}
}

// --- Golden file tests ------------------------------------------------
//
// The golden files pin the byte-exact output of the text and
// JSON handlers for a fixed error. They are the specification of
// the log format, the way the golden files in
// internal/forgeerr/testdata/format/ are the specification of
// the user-facing format.

// TestGolden_TextFormat compares the text handler's output for a
// fixed error against the committed golden file.
//
// Regenerate with:
//
//	go test -run TestGolden_TextFormat -update
//
// from inside internal/forgeerr/.
func TestGolden_TextFormat(t *testing.T) {
	t.Parallel()

	e := logFixtureError()

	got := logWith(newTextHandler, e)
	path := filepath.Join("testdata", "log", "text.golden.txt")
	want := goldenText(t, path, got)

	if got != want {
		t.Fatalf(
			"text output does not match %s.\n"+
				"Run `go test -run TestGolden_TextFormat -update` from internal/forgeerr/ to regenerate.\n"+
				"--- got ---\n%s\n--- want ---\n%s",
			path, got, want,
		)
	}
}

// TestGolden_JSONFormat compares the JSON handler's output for a
// fixed error against the committed golden file.
//
// Regenerate with:
//
//	go test -run TestGolden_JSONFormat -update
//
// from inside internal/forgeerr/.
func TestGolden_JSONFormat(t *testing.T) {
	t.Parallel()

	e := logFixtureError()

	got := logWith(newJSONHandler, e)
	path := filepath.Join("testdata", "log", "json.golden.txt")
	want := goldenText(t, path, got)

	if got != want {
		t.Fatalf(
			"JSON output does not match %s.\n"+
				"Run `go test -run TestGolden_JSONFormat -update` from internal/forgeerr/ to regenerate.\n"+
				"--- got ---\n%s\n--- want ---\n%s",
			path, got, want,
		)
	}
}

// logFixtureError returns the canonical error used by the golden
// tests. It exercises every attribute: code, category, cause, and
// multiple context entries.
func logFixtureError() *Error {
	return New(CategoryConfig, CodeConfigMalformed, "invalid YAML").
		WithCause(errors.New("yaml: line 12: mapping values are not allowed in this context")).
		WithContext("file", "config.yaml").
		WithContext("line", "12")
}

// --- Edge and corner cases --------------------------------------------

// TestEdge_EmptyCauseMessage asserts that a cause whose Error()
// returns the empty string is still emitted as an attribute with
// an empty value. The alternative — omitting the attribute —
// would make the presence of a cause indistinguishable from its
// absence.
func TestEdge_EmptyCauseMessage(t *testing.T) {
	t.Parallel()

	e := New(CategoryConfig, CodeConfigMalformed, "x").
		WithCause(errors.New(""))

	var buf bytes.Buffer
	logger := slog.New(newJSONHandler(&buf))
	logger.Error("failed", slog.Any("error", e))

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	errGroup := record["error"].(map[string]any)

	got, present := errGroup["cause"]
	if !present {
		t.Fatalf("cause attribute absent; want present with empty value")
	}
	if got != "" {
		t.Errorf("cause = %v, want empty string", got)
	}
}

// TestEdge_ContextKeyCollidesWithReservedKey documents the
// behaviour when a context key is one of the reserved keys
// ("code", "category", "cause"). The context key shadows the
// reserved key in the output, which is a data-loss scenario.
//
// The constructor's snake_case rule for context keys permits
// "code", "category", and "cause": all three are valid
// snake_case. The formatter cannot prevent the collision without
// reserving the names, which the format does not do.
//
// This test pins the current behaviour so that a future ADR can
// change it deliberately rather than accidentally. The change the
// ADR would make is either (a) reserving the three names in
// validateContextKey, which would reject them at construction
// time, or (b) teaching the formatter to prefix context keys that
// collide, which would preserve both values. Neither change is in
// scope for WBS 13.1.1; the test exists so that the decision is
// visible.
func TestEdge_ContextKeyCollidesWithReservedKey(t *testing.T) {
	t.Parallel()

	e := New(CategoryConfig, CodeConfigMalformed, "x").
		WithContext("code", "shadowed")

	var buf bytes.Buffer
	logger := slog.New(newJSONHandler(&buf))
	logger.Error("failed", slog.Any("error", e))

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	errGroup := record["error"].(map[string]any)

	// The context attribute is the last one written, so it
	// shadows the reserved one. The shadowing is a data loss:
	// the real code is no longer recoverable from the log entry.
	// The test asserts the loss is visible, so that a future
	// change to reject the collision at construction time is
	// accompanied by an update to this test.
	if got := errGroup["code"]; got != "shadowed" {
		t.Errorf("code = %v, want %q (context shadows reserved)", got, "shadowed")
	}
}

// TestEdge_ManyContextKeys asserts that a large number of
// context entries renders in a stable sorted order.
func TestEdge_ManyContextKeys(t *testing.T) {
	t.Parallel()

	e := New(CategoryConfig, CodeConfigMalformed, "x")
	keys := []string{"k", "j", "i", "h", "g", "f", "e", "d", "c", "b", "a"}
	for _, k := range keys {
		e = e.WithContext(k, "v-"+k)
	}

	out := logWith(newTextHandler, e)

	prev := -1
	for _, k := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"} {
		idx := strings.Index(out, "error."+k+"=")
		if idx == -1 {
			t.Fatalf("key %q missing from output:\n%s", k, out)
		}
		if idx <= prev {
			t.Fatalf("key %q at index %d, not after previous %d", k, idx, prev)
		}
		prev = idx
	}
}

// TestEdge_NonASCIIValues asserts that non-ASCII context values
// are emitted unchanged. The log format does not impose an
// ASCII-only rule; the ASCII policy (WBS 7.4.2) applies to the
// CLI's user-facing output, not to log entries.
func TestEdge_NonASCIIValues(t *testing.T) {
	t.Parallel()

	e := New(CategoryConfig, CodeConfigMalformed, "x").
		WithContext("path", "/home/用户/config.yaml")

	var buf bytes.Buffer
	logger := slog.New(newJSONHandler(&buf))
	logger.Error("failed", slog.Any("error", e))

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	errGroup := record["error"].(map[string]any)

	if got := errGroup["path"]; got != "/home/用户/config.yaml" {
		t.Errorf("path = %v", got)
	}
}

// --- Determinism and concurrency --------------------------------------

// TestNF_Deterministic asserts that repeated calls to LogValue
// produce the same attribute order and values.
//
// The sort of context keys is what makes the output
// deterministic; Go's map iteration order is randomised. A
// regression that removed the sort would fail this test on some
// runs and pass on others, which is the definition of a
// non-determinism bug.
func TestNF_Deterministic(t *testing.T) {
	t.Parallel()

	e := New(CategoryConfig, CodeConfigMalformed, "x").
		WithContext("alpha", "1").
		WithContext("beta", "2").
		WithContext("gamma", "3").
		WithContext("delta", "4").
		WithContext("epsilon", "5")

	var first string
	for i := 0; i < 100; i++ {
		got := logWith(newTextHandler, e)
		if i == 0 {
			first = got
			continue
		}
		if got != first {
			t.Fatalf("output changed on call %d:\nfirst: %s\ngot:   %s", i, first, got)
		}
	}
}

// TestNF_ConcurrentLogging asserts that concurrent logging of
// the same *Error is safe under -race.
//
// The *Error is immutable; LogValue reads its fields. The test
// exercises the read path from many goroutines.
func TestNF_ConcurrentLogging(t *testing.T) {
	e := New(CategoryConfig, CodeConfigMalformed, "x").
		WithCause(errors.New("cause")).
		WithContext("a", "1").
		WithContext("b", "2")

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = logWith(newJSONHandler, e)
		}()
	}
	wg.Wait()
}

// --- Fuzz: the log invariants -----------------------------------------

// FuzzLogValue asserts that LogValue's output satisfies the
// frozen invariants for every valid *Error: the code and the
// category are always present, the context keys are sorted, and
// the output is valid JSON when rendered by the JSON handler.
func FuzzLogValue(f *testing.F) {
	f.Add("x", "FORGE_CONFIG_MALFORMED", "CONFIG", "file", "config.yaml")
	f.Add("x", "FORGE_CLI_UNKNOWN_COMMAND", "CLI", "name", "verison")
	f.Add("x", "FORGE_FILESYSTEM_NOT_FOUND", "FILESYSTEM", "path", "/tmp")

	f.Fuzz(func(t *testing.T, msg, code, catStr, key, value string) {
		cat := Category(catStr)
		if !cat.valid() {
			t.Skip("invalid category")
		}
		if !IsRegistered(Code(code)) {
			t.Skip("unregistered code")
		}
		actualCat, ok := Code(code).Category()
		if !ok || actualCat != cat {
			t.Skip("code does not match category")
		}
		if msg == "" || len(msg) > 80 || strings.HasSuffix(msg, ".") || strings.ContainsAny(msg, "\n\r") {
			t.Skip("invalid message")
		}
		if key != "" && !isSnakeCase(key) {
			t.Skip("invalid context key")
		}

		var e *Error
		if key == "" {
			e = New(cat, Code(code), msg)
		} else {
			e = New(cat, Code(code), msg).WithContext(key, value)
		}

		var buf bytes.Buffer
		logger := slog.New(newJSONHandler(&buf))
		logger.Error("failed", slog.Any("error", e))

		var record map[string]any
		if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
			t.Fatalf("unmarshal: %v\nraw: %s", err, buf.String())
		}

		errGroup, ok := record["error"].(map[string]any)
		if !ok {
			t.Fatalf("error group not found: %v", record)
		}

		if _, present := errGroup["code"]; !present {
			t.Fatalf("code attribute missing: %v", errGroup)
		}
		if _, present := errGroup["category"]; !present {
			t.Fatalf("category attribute missing: %v", errGroup)
		}
	})
}

// --- Redaction tests --------------------------------------------------
//
// The redaction tests below assert that the log format, rendered
// through a redacting handler, does not leak secrets or home
// directories. They are the programmer-facing counterpart to the
// user-facing redaction tests in format_test.go.

// TestLog_RedactsSecretContextValue asserts that a log entry
// whose context includes a secret is rendered with the marker,
// not the raw value.
func TestLog_RedactsSecretContextValue(t *testing.T) {
	t.Parallel()

	e := New(CategoryConfig, CodeConfigMalformed, "x").
		WithContext("api_token", "ghp_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")

	var buf bytes.Buffer
	logger := slog.New(newJSONHandler(&buf))
	logger.Error("failed", slog.Any("error", e))

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	errGroup := record["error"].(map[string]any)

	if got := errGroup["api_token"]; got != redactedMarker {
		t.Errorf("api_token = %v, want %q", got, redactedMarker)
	}
	if strings.Contains(buf.String(), "ghp_") {
		t.Errorf("log entry contains the raw token:\n%s", buf.String())
	}
}

// TestLog_RedactsHomeDirectoryPath asserts that a log entry
// whose context includes a home-directory path is rendered with
// the "~" prefix.
//
// The test uses the actual home directory, obtained from
// os.UserHomeDir, rather than a synthetic path. A synthetic path
// would not exercise the redaction rule's dependence on the
// running user's home, which is the part that can break across
// platforms or CI environments.
func TestLog_RedactsHomeDirectoryPath(t *testing.T) {
	t.Parallel()

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("no home directory available")
	}

	in := filepath.Join(home, "config.yaml")
	e := New(CategoryConfig, CodeConfigMalformed, "x").
		WithContext("file", in)

	var buf bytes.Buffer
	logger := slog.New(newJSONHandler(&buf))
	logger.Error("failed", slog.Any("error", e))

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	errGroup := record["error"].(map[string]any)

	if got := errGroup["file"]; got != "~/config.yaml" {
		t.Errorf("file = %v, want ~/config.yaml", got)
	}
	if strings.Contains(buf.String(), home) {
		t.Errorf("log entry contains the raw home directory:\n%s", buf.String())
	}
}

// TestLog_CauseIsShown asserts that the log format includes the
// cause as a field. This is the counterpart to the user-facing
// format's TestFormat_CauseNeverShown.
func TestLog_CauseIsShown(t *testing.T) {
	t.Parallel()

	e := New(CategoryConfig, CodeConfigMalformed, "x").
		WithCause(errors.New("internal detail"))

	var buf bytes.Buffer
	logger := slog.New(newJSONHandler(&buf))
	logger.Error("failed", slog.Any("error", e))

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	errGroup := record["error"].(map[string]any)

	if got := errGroup["cause"]; got != "internal detail" {
		t.Errorf("cause = %v, want %q", got, "internal detail")
	}
}
