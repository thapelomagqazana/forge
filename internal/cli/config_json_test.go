package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// WBS 9.4.1 — JSON output for `forge config`.
//
// These tests pin the frozen JSON schema. They call writeConfigJSON
// directly (it is a pure function of (io.Writer, ResolvedConfig) →
// error) so that each property can be asserted in isolation.
//
// The end-to-end tests (through runCLI with --format json) live in
// config_golden_test.go; the schema file's consistency with the
// struct is tested by TestJSONSchema_MatchesStruct.
// ---------------------------------------------------------------------------

// sampleResolved is a ResolvedConfig with non-default values, used
// by tests that want to assert the mapping of each field. The values
// are chosen to be distinct so that a mis-mapped field produces a
// visible failure.
func sampleResolved() ResolvedConfig {
	return ResolvedConfig{
		SourcePath:     "/home/user/.config/forge/config.yaml",
		LogLevel:       "debug",
		LogLevelSource: configSourceFlag,
		Output:         "json",
		OutputSource:   configSourceFile,
	}
}

// TestWriteConfigJSON_AllFields asserts that every field of the
// ResolvedConfig appears in the JSON output under its frozen key.
//
// The test parses the output and asserts each key's value. It does
// not assert the byte-for-byte form (that is the golden test's job);
// it asserts the structural contract.
func TestWriteConfigJSON_AllFields(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, writeConfigJSON(&buf, sampleResolved()))

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got),
		"output must be valid JSON: %q", buf.String())

	assert.Equal(t, "/home/user/.config/forge/config.yaml", got["config_path"])
	assert.Equal(t, "debug", got["log_level"])
	assert.Equal(t, "flag", got["log_level_source"])
	assert.Equal(t, "json", got["output_format"])
	assert.Equal(t, "file", got["output_format_source"])
	assert.Len(t, got, 5, "the JSON object must have exactly five keys")
}

// TestWriteConfigJSON_ConfigPathIsNullWhenEmpty asserts that an
// empty SourcePath encodes as null, not as the empty string.
//
// The distinction is part of the schema: null means "no file was
// resolved"; an empty string would suggest a valid but empty path.
func TestWriteConfigJSON_ConfigPathIsNullWhenEmpty(t *testing.T) {
	t.Parallel()

	cfg := sampleResolved()
	cfg.SourcePath = ""

	var buf bytes.Buffer
	require.NoError(t, writeConfigJSON(&buf, cfg))

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	assert.Nil(t, got["config_path"],
		"config_path must be null when SourcePath is empty")
}

// TestWriteConfigJSON_ConfigPathIsStringWhenSet asserts that a
// non-empty SourcePath encodes as a JSON string.
func TestWriteConfigJSON_ConfigPathIsStringWhenSet(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, writeConfigJSON(&buf, sampleResolved()))

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	path, ok := got["config_path"].(string)
	require.True(t, ok, "config_path must be a JSON string when set")
	assert.Equal(t, "/home/user/.config/forge/config.yaml", path)
}

// TestWriteConfigJSON_ExactlyOneLine asserts AC5: the output is a
// single line.
func TestWriteConfigJSON_ExactlyOneLine(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, writeConfigJSON(&buf, sampleResolved()))

	out := buf.String()
	assert.Equal(t, 1, strings.Count(out, "\n"),
		"output must have exactly one newline; got %q", out)
	assert.True(t, strings.HasSuffix(out, "\n"),
		"output must end with a newline; got %q", out)
}

// TestWriteConfigJSON_KeysAreAlphabetical asserts AC6: the keys are
// emitted in alphabetical order.
//
// # How the check works
//
// A map[string]any decoded from JSON loses key order. To check
// order, the test decodes the object's keys with a json.Decoder
// using Token(), which yields tokens in document order. It collects
// the object's keys and asserts they are sorted.
//
// # Why the test matters
//
// The struct's declaration order determines the wire order. A
// future reordering of the struct's fields (e.g. to group related
// fields) would change the wire order without changing the values.
// The test pins the alphabetical order the schema promises.
func TestWriteConfigJSON_KeysAreAlphabetical(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, writeConfigJSON(&buf, sampleResolved()))

	keys := collectObjectKeys(t, buf.Bytes())

	// Verify sorted.
	sorted := make([]string, len(keys))
	copy(sorted, keys)
	sort.Strings(sorted)
	assert.Equal(t, sorted, keys,
		"JSON keys must be emitted in alphabetical order")
}

// collectObjectKeys decodes a JSON object and returns its top-level
// keys in document order.
//
// The helper is used by TestWriteConfigJSON_KeysAreAlphabetical and
// by the --path JSON key-order test. It relies on json.Decoder's
// Token() method, which returns tokens in the order they appear in
// the input.
func collectObjectKeys(t *testing.T, data []byte) []string {
	t.Helper()

	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	require.NoError(t, err)
	require.Equal(t, json.Delim('{'), tok,
		"the top-level JSON value must be an object")

	var keys []string
	for dec.More() {
		keyTok, err := dec.Token()
		require.NoError(t, err)
		key, ok := keyTok.(string)
		require.True(t, ok, "object key must be a string, got %T", keyTok)
		keys = append(keys, key)

		// Skip the value (of any shape).
		var value any
		require.NoError(t, dec.Decode(&value))
	}
	// Consume the closing brace.
	_, err = dec.Token()
	require.NoError(t, err)

	return keys
}

// TestWriteConfigJSON_ValidJSON asserts AC1: the output is valid
// JSON for the shapes the resolver can produce.
func TestWriteConfigJSON_ValidJSON(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		cfg  ResolvedConfig
	}{
		{"full", sampleResolved()},
		{"no path", ResolvedConfig{
			LogLevel:       "info",
			LogLevelSource: configSourceDefault,
			Output:         "text",
			OutputSource:   configSourceDefault,
		}},
		{"zero", ResolvedConfig{}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			require.NoError(t, writeConfigJSON(&buf, tc.cfg))
			assert.True(t, json.Valid(buf.Bytes()),
				"output must be valid JSON: %q", buf.String())
		})
	}
}

// TestWriteConfigJSON_SourceValuesAreConstrained asserts AC4: the
// source fields carry one of the four frozen values.
func TestWriteConfigJSON_SourceValuesAreConstrained(t *testing.T) {
	t.Parallel()

	allowed := map[string]bool{
		"default": true,
		"file":    true,
		"env":     true,
		"flag":    true,
	}

	for src := range allowed {
		src := src
		t.Run(src, func(t *testing.T) {
			t.Parallel()
			cfg := ResolvedConfig{
				LogLevel:       "info",
				LogLevelSource: configSource(src),
				Output:         "text",
				OutputSource:   configSource(src),
			}
			var buf bytes.Buffer
			require.NoError(t, writeConfigJSON(&buf, cfg))

			var got map[string]any
			require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
			assert.Equal(t, src, got["log_level_source"])
			assert.Equal(t, src, got["output_format_source"])
		})
	}
}

// TestWriteConfigJSON_PropagatesWriteError asserts that a writer
// error is returned unchanged.
func TestWriteConfigJSON_PropagatesWriteError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("write failed")
	err := writeConfigJSON(errWriter{err: sentinel}, sampleResolved())

	require.Error(t, err)
	assert.ErrorIs(t, err, sentinel)
}

// TestWriteConfigJSON_NoHTMLEscaping asserts that the encoder does
// not escape <, >, and & to \u003c, \u003e, and \u0026.
//
// Paths and values rarely contain these characters, but a path may
// (a directory named "a&b"). The test pins the encoder's
// configuration.
func TestWriteConfigJSON_NoHTMLEscaping(t *testing.T) {
	t.Parallel()

	cfg := sampleResolved()
	cfg.SourcePath = "/tmp/a&b<c>d.yaml"

	var buf bytes.Buffer
	require.NoError(t, writeConfigJSON(&buf, cfg))

	out := buf.String()
	assert.Contains(t, out, "&", "& must not be escaped: %q", out)
	assert.NotContains(t, out, `\u0026`, "& must not be escaped: %q", out)
}
