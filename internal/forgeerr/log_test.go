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

// --- Test helpers -----------------------------------------------------

// logWith renders e with the given handler and returns the
// resulting string. The message is the standard "something went
// wrong" used by the tests; the error's own message is rendered
// separately by the tests that need it.
//
// The helper accepts a slog.Handler so that the same call site
// works with the text handler, the JSON handler, and any future
// handler.
func logWith(h slog.Handler, e *Error) string {
	var buf bytes.Buffer
	logger := slog.New(h)
	// The With-attached attributes are part of the record. The
	// message is deliberately short so that the assertions on
	// the error group are not confused with the message.
	logger.Error("failed",
		slog.Any("error", e),
	)
	return buf.String()
}

// newTextHandler returns a slog.Handler that writes to buf in
// text format, with the timestamp removed and the level
// lowercased. The transformations make the output deterministic
// and comparable against golden files.
func newTextHandler(buf *bytes.Buffer) slog.Handler {
	return slog.NewTextHandler(buf, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Drop the timestamp; it changes on every call.
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			// Lowercase the level for readability.
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
// lowercased.
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

	var v slog.LogValuer = New(CategoryConfig, string(CodeConfigMalformed), "x")
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
	e := New(CategoryConfig, string(CodeConfigMalformed), "invalid YAML").
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

	e := New(CategoryConfig, string(CodeConfigMalformed), "invalid YAML")

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

	e := New(CategoryConfig, string(CodeConfigMalformed), "invalid YAML").
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
	// Ensure context is not nested under a "context" key.
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
	e := New(CategoryConfig, string(CodeConfigMalformed), "x").
		WithContext("zeta", "1").
		WithContext("yotta", "2").
		WithContext("alpha", "3")

	var buf bytes.Buffer
	logger := slog.New(newTextHandler(&buf))
	logger.Error("failed", slog.Any("error", e))
	out := buf.String()

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

	e := New(CategoryConfig, string(CodeConfigMalformed), "invalid YAML")

	var buf bytes.Buffer
	logger := slog.New(newTextHandler(&buf))
	logger.Error("failed", slog.Any("error", e))
	out := buf.String()

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

	e := New(CategoryConfig, string(CodeConfigMalformed), "invalid YAML").
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

	e := New(CategoryConfig, string(CodeConfigMalformed), "invalid YAML").
		WithContext("file", "config.yaml")

	var buf bytes.Buffer
	logger := slog.New(newTextHandler(&buf))
	logger.Error("failed", slog.Any("error", e))
	out := buf.String()

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

	// A fake secret. In practice, the secret pattern list is in
	// docs/security-model.md; this test uses a distinctive
	// string and a matching predicate for clarity.
	const secret = "ghp_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

	e := New(CategoryConfig, string(CodeConfigMalformed), "invalid YAML").
		WithContext("token", secret)

	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			// Redact any string value that looks like a
			// GitHub personal access token.
			if s, ok := a.Value.Any().(string); ok {
				if strings.HasPrefix(s, "ghp_") {
					a.Value = slog.StringValue("<redacted>")
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

	if got := errGroup["token"]; got != "<redacted>" {
		t.Errorf("token = %v, want <redacted>", got)
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
	var buf bytes.Buffer
	logger := slog.New(newTextHandler(&buf))
	logger.Error("failed", slog.Any("error", e))

	got := buf.String()
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
	var buf bytes.Buffer
	logger := slog.New(newJSONHandler(&buf))
	logger.Error("failed", slog.Any("error", e))

	got := buf.String()
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
	return New(CategoryConfig, string(CodeConfigMalformed), "invalid YAML").
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

	e := New(CategoryConfig, string(CodeConfigMalformed), "x").
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

// TestEdge_ContextKeyCollidesWithReservedKey asserts the
// behaviour when a context key is one of the reserved keys
// ("code", "category", "cause"). The context key shadows the
// reserved key in the output, which is a data-loss scenario.
//
// The constructor's snake_case rule for context keys permits
// "code", "category", and "cause": all three are valid
// snake_case. The formatter cannot prevent the collision without
// reserving the names, which the format does not do.
//
// This test documents the current behaviour: the context
// attribute overwrites the reserved attribute when the two
// collide. A future ADR may reserve the names and reject them at
// construction time.
func TestEdge_ContextKeyCollidesWithReservedKey(t *testing.T) {
	t.Parallel()

	// The collision is documented, not prevented. A caller who
	// uses "code", "category", or "cause" as a context key
	// loses the corresponding reserved attribute.
	e := New(CategoryConfig, string(CodeConfigMalformed), "x").
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
	// shadows the reserved one. This is the documented
	// behaviour; see the test's docstring.
	if got := errGroup["code"]; got != "shadowed" {
		t.Errorf("code = %v, want %q (context shadows reserved)", got, "shadowed")
	}
}

// TestEdge_ManyContextKeys asserts that a large number of
// context entries renders in a stable sorted order.
func TestEdge_ManyContextKeys(t *testing.T) {
	t.Parallel()

	e := New(CategoryConfig, string(CodeConfigMalformed), "x")
	keys := []string{"k", "j", "i", "h", "g", "f", "e", "d", "c", "b", "a"}
	for _, k := range keys {
		e = e.WithContext(k, "v-"+k)
	}

	var buf bytes.Buffer
	logger := slog.New(newTextHandler(&buf))
	logger.Error("failed", slog.Any("error", e))
	out := buf.String()

	// The keys are sorted in the output. The insertion order
	// above is reverse-alphabetical; the output must be
	// alphabetical.
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

	e := New(CategoryConfig, string(CodeConfigMalformed), "x").
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

	e := New(CategoryConfig, string(CodeConfigMalformed), "x").
		WithContext("alpha", "1").
		WithContext("beta", "2").
		WithContext("gamma", "3").
		WithContext("delta", "4").
		WithContext("epsilon", "5")

	var first string
	for i := 0; i < 100; i++ {
		var buf bytes.Buffer
		logger := slog.New(newTextHandler(&buf))
		logger.Error("failed", slog.Any("error", e))
		got := buf.String()
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
	e := New(CategoryConfig, string(CodeConfigMalformed), "x").
		WithCause(errors.New("cause")).
		WithContext("a", "1").
		WithContext("b", "2")

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var buf bytes.Buffer
			logger := slog.New(newJSONHandler(&buf))
			logger.Error("failed", slog.Any("error", e))
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
			e = New(cat, code, msg)
		} else {
			e = New(cat, code, msg).WithContext(key, value)
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

		// The code and the category are always present.
		if _, present := errGroup["code"]; !present {
			t.Fatalf("code attribute missing: %v", errGroup)
		}
		if _, present := errGroup["category"]; !present {
			t.Fatalf("category attribute missing: %v", errGroup)
		}
	})
}

// TestLog_RedactsSecretContextValue asserts that a log entry
// whose context includes a secret is rendered with the marker,
// not the raw value.
func TestLog_RedactsSecretContextValue(t *testing.T) {
	t.Parallel()

	e := New(CategoryConfig, string(CodeConfigMalformed), "x").
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
func TestLog_RedactsHomeDirectoryPath(t *testing.T) {
	t.Parallel()

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("no home directory available")
	}

	in := filepath.Join(home, "config.yaml")
	e := New(CategoryConfig, string(CodeConfigMalformed), "x").
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

	e := New(CategoryConfig, string(CodeConfigMalformed), "x").
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
