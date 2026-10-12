package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errWriter is an io.Writer that always returns err from Write.
//
// The type is used by tests that assert error propagation: they
// pass an errWriter to a function under test and assert that the
// function returns the same error. The type is small enough to
// inline in each test, but defining it once keeps the tests
// focused on the property they assert, not on the plumbing.
//
// # Why not use a real writer
//
// A real io.Writer (bytes.Buffer) never returns an error from
// Write. To test that a function propagates a writer's error, the
// writer must be able to produce one. errWriter is the minimal
// implementation.
//
// # Why a struct and not a function
//
// The io.Writer interface requires a Write method. A struct with
// the err field satisfies it; a closure would need to be wrapped
// in a type. The struct is the idiomatic choice.
type errWriter struct{ err error }

// Write always returns the configured error, with no bytes written.
func (w errWriter) Write(p []byte) (int, error) {
	return 0, w.err
}

// ---------------------------------------------------------------------------
// WBS 9.3.1 — --path writer functions.
//
// These tests exercise the three pure functions that implement
// `forge config --path`:
//
//   - writeConfigPath      — dispatch by --format value
//   - writeConfigPathText  — text rendering (<path> or <none>)
//   - writeConfigPathJSON  — JSON rendering ({"config_path": ...})
//
// The functions are pure: they take an io.Writer, a path string, and
// (for writeConfigPath) a format string, and return an error. The
// tests exercise them in isolation so that each byte-level rule from
// WBS 9.3.1 can be pinned by its own assertion.
//
// The end-to-end tests (through runCLI) live in
// config_golden_test.go; the --path flag's metadata is tested in
// config_test.go. This file is the unit layer between them.
//
// # JSON key
//
// The JSON key is configPathJSONKey (which is "config_path", per
// WBS 9.4.1's alignment of the two JSON surfaces). The tests use
// the constant rather than the string literal so that a future
// rename of the key is a single edit to the constant.
// ---------------------------------------------------------------------------

// =============================================================================
// Text format — writeConfigPathText
// =============================================================================

// TestWriteConfigPathText_WithPath asserts the output for a resolved
// path: the path alone, on one line, ending with \n.
//
// The frozen contract (WBS 9.3.1) says: "No label, no decoration —
// just the path or <none>." The test pins the "just the path" half.
func TestWriteConfigPathText_WithPath(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, writeConfigPathText(&buf,
		"/home/user/.config/forge/config.yaml"))

	assert.Equal(t,
		"/home/user/.config/forge/config.yaml\n",
		buf.String())
}

// TestWriteConfigPathText_WithoutPath asserts the output when no
// config file was resolved: the literal "<none>", on one line,
// ending with \n.
//
// The frozen contract (WBS 9.3.1) says: "Print <none>". The test
// pins the literal string, including the angle brackets.
func TestWriteConfigPathText_WithoutPath(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, writeConfigPathText(&buf, ""))

	assert.Equal(t, "<none>\n", buf.String())
}

// TestWriteConfigPathText_ExactlyOneLine asserts AC2 for the text
// format: the output is a single line.
//
// # What the test asserts
//
//   - The output contains exactly one newline.
//   - The output ends with that newline.
//
// Both properties together imply the output is one line followed by
// a line terminator. A future change that emitted a trailing blank
// line would produce two newlines and fail the test.
func TestWriteConfigPathText_ExactlyOneLine(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, writeConfigPathText(&buf, "/a/b/c.yaml"))

	out := buf.String()
	assert.Equal(t, 1, strings.Count(out, "\n"),
		"output must have exactly one newline; got %q", out)
	assert.True(t, strings.HasSuffix(out, "\n"),
		"output must end with a newline; got %q", out)
}

// TestWriteConfigPathText_NoTrailingWhitespace asserts that the line
// does not end with a space or a tab before the newline.
//
// The frozen format's F8 rule from WBS 9.2.1 ("no trailing
// whitespace on any line") applies to --path output as well; the
// WBS 9.3.1 contract inherits it. The test pins the rule for both
// the path case and the <none> case.
func TestWriteConfigPathText_NoTrailingWhitespace(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		path string
	}{
		{"with path", "/a/b/c.yaml"},
		{"without path", ""},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			require.NoError(t, writeConfigPathText(&buf, tc.path))

			line := strings.TrimSuffix(buf.String(), "\n")
			assert.False(t, strings.HasSuffix(line, " "),
				"line must not end with a space: %q", line)
			assert.False(t, strings.HasSuffix(line, "\t"),
				"line must not end with a tab: %q", line)
		})
	}
}

// TestWriteConfigPathText_NoDecoration asserts that the text output
// has no label, prefix, or trailing punctuation beyond the required
// newline.
//
// # What the test rejects
//
//   - A label separator (":"): "Path: /a/b/c.yaml" would fail.
//   - An equals sign ("="): "path=/a/b/c.yaml" would fail.
//   - A label word ("Path"): "Path /a/b/c.yaml" would fail.
//
// The frozen contract (WBS 9.3.1) is that the output is *just* the
// path. A shell script that pipes the output to `read` expects a
// single word (possibly with path separators) on the line, not a
// key-value pair.
//
// # What the test does not reject
//
// The test does not reject characters that a path legitimately
// contains: "/", ".", "-", "_", "~", spaces, and so on. A path
// containing a colon (e.g. on Windows, "C:\\foo") would technically
// match the "no colon" assertion; the test uses a Unix-style path
// with no colon, so the assertion is sound for that input.
func TestWriteConfigPathText_NoDecoration(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, writeConfigPathText(&buf, "/a/b/c.yaml"))

	out := buf.String()
	assert.NotContains(t, out, ":",
		"output must not contain a label separator")
	assert.NotContains(t, out, "=",
		"output must not contain an equals sign")
	assert.NotContains(t, out, "Path",
		"output must not contain a label word")
	assert.NotContains(t, out, "path:",
		"output must not contain a lower-case label")
}

// TestWriteConfigPathText_PathWithSpaces is an edge case: a path
// containing spaces is rendered verbatim, without quoting or
// escaping.
//
// # Why no quoting
//
// The path is the value the user asked for. Quoting it (with " or ')
// would change its content: a script that read the output would get
// the quotes as part of the path. The frozen contract is "just the
// path"; a path with spaces is still just the path.
//
// The consumer is responsible for handling the space (e.g. by
// quoting the variable after `read`). The producer does not add
// quoting that the consumer would have to strip.
func TestWriteConfigPathText_PathWithSpaces(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, writeConfigPathText(&buf,
		"/home/user/my configs/forge.yaml"))

	assert.Equal(t,
		"/home/user/my configs/forge.yaml\n",
		buf.String(),
		"the path is rendered verbatim; no quoting, no escaping")
}

// TestWriteConfigPathText_PathWithTilde is an edge case: a path
// starting with ~ (the shell's home-directory shorthand) is
// rendered verbatim.
//
// # Why the test exists
//
// The producer does not expand ~. Expanding it would require reading
// the user's home directory, which is a resolution concern (WBS
// 8.4.1), not a rendering concern. By the time the path reaches
// --path, it has been resolved; if the resolver left a ~ in the
// path, the renderer passes it through. The test pins the renderer's
// non-involvement.
func TestWriteConfigPathText_PathWithTilde(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, writeConfigPathText(&buf, "~/.config/forge/config.yaml"))

	assert.Equal(t,
		"~/.config/forge/config.yaml\n",
		buf.String(),
		"the renderer must not expand ~; expansion is the "+
			"resolver's responsibility (WBS 8.4.1)")
}

// TestWriteConfigPathText_RelativePath is a boundary case: a
// relative path is rendered verbatim.
//
// The renderer does not distinguish absolute from relative paths.
// Both are strings; both are rendered. The test pins the
// non-distinction.
func TestWriteConfigPathText_RelativePath(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, writeConfigPathText(&buf, "configs/forge.yaml"))

	assert.Equal(t, "configs/forge.yaml\n", buf.String())
}

// TestWriteConfigPathText_EmptyIsNone is a boundary test: the empty
// string is the input that triggers the <none> placeholder. An
// empty path is what the resolver produces when no file was found.
//
// The test is the same as TestWriteConfigPathText_WithoutPath, but
// named differently to emphasize the boundary: "" is the specific
// input, "<none>" is the specific output, and the mapping between
// them is the contract.
func TestWriteConfigPathText_EmptyIsNone(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, writeConfigPathText(&buf, ""))

	assert.Equal(t, "<none>\n", buf.String(),
		"the empty path must render as the literal %q",
		"<none>")
}

// =============================================================================
// JSON format — writeConfigPathJSON
// =============================================================================

// TestWriteConfigPathJSON_WithPath asserts the JSON output for a
// resolved path: {"config_path":"<path>"}\n.
//
// # The key
//
// The key is configPathJSONKey, which after WBS 9.4.1 is
// "config_path" (it was "path" at WBS 9.3.1). The test reads the
// constant, not a string literal, so a future rename of the key
// does not require editing this test.
//
// # Why structural comparison
//
// The test parses the output as JSON and asserts on the decoded
// value, not on the raw string. This makes the test robust to
// whitespace or key-ordering variations that the encoder might
// introduce in a future version (though with one key, ordering is
// moot). The test still asserts the exact byte output for a
// canonical path, since the encoder's output for a fixed input is
// deterministic and the byte assertion localizes a change to the
// encoder's configuration.
func TestWriteConfigPathJSON_WithPath(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, writeConfigPathJSON(&buf,
		"/home/user/.config/forge/config.yaml"))

	// Structural assertion.
	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got),
		"output must be valid JSON: %q", buf.String())
	assert.Equal(t, "/home/user/.config/forge/config.yaml",
		got[configPathJSONKey])
	assert.Len(t, got, 1, "the JSON object must have exactly one key")

	// Byte assertion: the encoder's output for this canonical
	// input is deterministic.
	assert.Equal(t,
		`{"`+configPathJSONKey+`":"/home/user/.config/forge/config.yaml"}`+"\n",
		buf.String())
}

// TestWriteConfigPathJSON_WithoutPath asserts the null case:
// {"config_path":null}\n.
//
// # Why null and not ""
//
// The distinction matters: null expresses "no path resolved"; an
// empty string would suggest a valid but empty path. A consumer
// that checks `if path != nil` distinguishes the two; a consumer
// that checks `if path != ""` would treat them the same, but the
// JSON encoder writes null, and the frozen contract (WBS 9.3.1,
// confirmed by WBS 9.4.1) requires null.
func TestWriteConfigPathJSON_WithoutPath(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, writeConfigPathJSON(&buf, ""))

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got),
		"output must be valid JSON: %q", buf.String())
	assert.Nil(t, got[configPathJSONKey],
		"an empty path must encode as null, not as the empty string")
	assert.Len(t, got, 1)

	// Byte assertion: the null output.
	assert.Equal(t,
		`{"`+configPathJSONKey+`":null}`+"\n",
		buf.String())
}

// TestWriteConfigPathJSON_ExactlyOneLine asserts AC2 for JSON: a
// single line ending with \n.
//
// The json.Encoder writes a trailing newline by default. The test
// pins the property so that a future change to the encoder's
// configuration (e.g. calling SetIndent) is caught.
func TestWriteConfigPathJSON_ExactlyOneLine(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, writeConfigPathJSON(&buf, "/a/b/c.yaml"))

	out := buf.String()
	assert.Equal(t, 1, strings.Count(out, "\n"),
		"JSON output must have exactly one newline; got %q", out)
	assert.True(t, strings.HasSuffix(out, "\n"),
		"JSON output must end with a newline; got %q", out)
}

// TestWriteConfigPathJSON_PathWithSpecialCharacters asserts that a
// path containing characters requiring JSON escaping is encoded
// correctly.
//
// # The specific characters
//
//   - `"` (double quote): must be escaped as `\"` in JSON.
//   - `\` (backslash): must be escaped as `\\` in JSON.
//   - Control characters (tab, newline): must be escaped as `\t`,
//     `\n`.
//
// A naive fmt.Sprintf(`{"config_path":%q}`, path) would handle most
// of these by accident (Go's %q is close to JSON's string escaping
// for many inputs), but not all: %q escapes some characters that
// JSON does not require, and its handling of certain Unicode
// codepoints differs. The json.Encoder handles all cases correctly.
// The test exercises the cases that diverge.
//
// # The key lookup
//
// The decoded map is read with configPathJSONKey, not with a string
// literal. The test's purpose is to assert that the value under the
// key survives the encoding; the key's identity is config.go's
// concern.
func TestWriteConfigPathJSON_PathWithSpecialCharacters(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		path string
	}{
		{"double quote", `/tmp/a"b.yaml`},
		{"backslash", `/tmp/a\b.yaml`},
		{"both", `/tmp/a\b"c.yaml`},
		{"tab", "/tmp/a\tb.yaml"},
		{"newline", "/tmp/a\nb.yaml"},
		{"unicode", "/tmp/日本語.yaml"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			require.NoError(t, writeConfigPathJSON(&buf, tc.path))

			var got map[string]any
			require.NoError(t, json.Unmarshal(buf.Bytes(), &got),
				"output must be valid JSON: %q", buf.String())
			assert.Equal(t, tc.path, got[configPathJSONKey],
				"the decoded path must equal the input path")
		})
	}
}

// TestWriteConfigPathJSON_ValidJSON asserts that the output is valid
// JSON for a variety of paths.
//
// The test is a broad sweep over path shapes: absolute, relative,
// empty, with spaces, with special characters, with non-ASCII. Every
// output must parse with encoding/json. A future change to the
// encoder that produced invalid JSON for some input would fail this
// test before the input's semantics were questioned.
func TestWriteConfigPathJSON_ValidJSON(t *testing.T) {
	t.Parallel()

	paths := []string{
		"/a/b/c.yaml",
		"",
		"relative/path.yaml",
		"/path with spaces/x.yaml",
		`/path\with\backslashes/x.yaml`,
		`/path"with"quotes/x.yaml`,
		"/path/with/<html>&chars/x.yaml",
		"/path/with/unicode/日本語.yaml",
		"/path/with/emoji/🎉.yaml",
	}

	for _, p := range paths {
		p := p
		t.Run(p, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			require.NoError(t, writeConfigPathJSON(&buf, p))
			assert.True(t, json.Valid(buf.Bytes()),
				"output is not valid JSON: %q", buf.String())
		})
	}
}

// TestWriteConfigPathJSON_NoHTMLEscaping asserts that the encoder
// does not escape <, >, and & to \u003c, \u003e, and \u0026.
//
// # Why the test exists
//
// The json.Encoder escapes these characters by default to protect
// against HTML injection when JSON is embedded in a <script> tag.
// The CLI's JSON output is not embedded in HTML; it is consumed by
// tools like jq. The encoder is configured with SetEscapeHTML(false)
// to keep the characters literal.
//
// A future change that removed the SetEscapeHTML(false) call would
// produce valid JSON (the escaped form is also valid) but would
// surprise users whose paths contain these characters. The test
// pins the non-escaping behavior.
func TestWriteConfigPathJSON_NoHTMLEscaping(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, writeConfigPathJSON(&buf,
		"/path/with/<html>&chars/x.yaml"))

	out := buf.String()

	// The raw characters must be present.
	assert.Contains(t, out, "<",
		"< must not be escaped to \\u003c: %q", out)
	assert.Contains(t, out, ">",
		"> must not be escaped to \\u003e: %q", out)
	assert.Contains(t, out, "&",
		"& must not be escaped to \\u0026: %q", out)

	// The escaped forms must not be present.
	assert.NotContains(t, out, `\u003c`,
		"< must not be escaped: %q", out)
	assert.NotContains(t, out, `\u003e`,
		"> must not be escaped: %q", out)
	assert.NotContains(t, out, `\u0026`,
		"& must not be escaped: %q", out)

	// The output must still be valid JSON.
	assert.True(t, json.Valid(buf.Bytes()),
		"output must be valid JSON: %q", out)
}

// TestWriteConfigPathJSON_KeyMatchesConstant asserts that the JSON
// key emitted by writeConfigPathJSON matches configPathJSONKey.
//
// # Why the test exists
//
// The key appears as a string literal in the function's struct
// tag (`json:"config_path"`). Go does not permit a constant in a
// struct tag, so the literal and the constant are kept in sync by
// hand. The test catches a mismatch: a future contributor who
// renamed the constant but missed the tag (or vice versa) sees the
// failure here.
//
// # What the test asserts
//
// The test parses the emitted JSON and checks that its single key
// equals configPathJSONKey. The check is structural; it does not
// depend on the encoder's byte output for any particular path.
func TestWriteConfigPathJSON_KeyMatchesConstant(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, writeConfigPathJSON(&buf, "/a/b/c.yaml"))

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	require.Len(t, got, 1, "the JSON object must have exactly one key")

	for k := range got {
		assert.Equal(t, configPathJSONKey, k,
			"the emitted key must equal configPathJSONKey; "+
				"the struct tag and the constant have drifted apart")
	}
}

// =============================================================================
// Format dispatch — writeConfigPath
// =============================================================================

// TestWriteConfigPath_FormatText asserts that format "text" dispatches
// to the text writer.
//
// The test's input is chosen so that the text and JSON writers
// produce visibly different outputs; the assertion distinguishes
// them.
func TestWriteConfigPath_FormatText(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, writeConfigPath(&buf, "/a/b/c.yaml", "text"))
	assert.Equal(t, "/a/b/c.yaml\n", buf.String(),
		"format %q must produce the text rendering", "text")
}

// TestWriteConfigPath_FormatJSON asserts that format "json"
// dispatches to the JSON writer.
//
// The byte assertion is built from configPathJSONKey, not from a
// string literal, so the test does not hard-code the key.
func TestWriteConfigPath_FormatJSON(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, writeConfigPath(&buf, "/a/b/c.yaml", "json"))
	assert.Equal(t,
		`{"`+configPathJSONKey+`":"/a/b/c.yaml"}`+"\n",
		buf.String(),
		"format %q must produce the JSON rendering", "json")
}

// TestWriteConfigPath_FormatEmptyDefaultsToText asserts that the
// empty format string is treated as text.
//
// # Why the test exists
//
// The global --format flag defaults to "text", so the handler
// should never see an empty string. But the defensive case is
// documented: the empty string falls through to text rather than
// producing an error. If a future change to the global default
// ever passes an empty format through, the writer behaves sensibly
// rather than failing.
//
// # Why not fail on empty
//
// An empty format is not a user error; it is a "no format specified"
// state. The default is text, so an empty format should produce
// text output. Failing would turn a benign state into a user-facing
// error.
func TestWriteConfigPath_FormatEmptyDefaultsToText(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, writeConfigPath(&buf, "/a/b/c.yaml", ""))
	assert.Equal(t, "/a/b/c.yaml\n", buf.String(),
		"an empty format must default to text")
}

// TestWriteConfigPath_FormatUnknownErrors asserts that an unknown
// format value produces an error.
//
// # What the error must name
//
//   - The offending value (so the user knows what they passed).
//   - At least one supported value (so the user knows what is
//     accepted).
//
// # Why the test asserts the buffer is empty
//
// The unsupported format is detected before any output is written;
// the test asserts the writer received no bytes. This pins the
// "validate before write" property: a future change that started
// writing partial output before discovering the unsupported format
// would fail the test.
func TestWriteConfigPath_FormatUnknownErrors(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := writeConfigPath(&buf, "/a/b/c.yaml", "xml")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "xml",
		"error must name the unsupported value")
	assert.Contains(t, err.Error(), "text",
		"error must name at least one supported value")
	assert.Contains(t, err.Error(), "json",
		"error must name at least one supported value")
	assert.Empty(t, buf.String(),
		"an unsupported format must not write partial output")
}

// TestWriteConfigPath_FormatCaseSensitive asserts that the format
// value is matched case-sensitively: "TEXT" and "Json" are not
// recognized.
//
// # Why case-sensitive
//
// The global --format flag's value set is frozen by WBS 6.4.1 as
// lowercase (text, json). Accepting other cases would widen the
// set beyond what the spec permits and would create a discrepancy
// between what --format accepts and what the spec documents. The
// test pins the case-sensitivity.
func TestWriteConfigPath_FormatCaseSensitive(t *testing.T) {
	t.Parallel()

	for _, format := range []string{"TEXT", "Text", "JSON", "Json"} {
		format := format
		t.Run(format, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			err := writeConfigPath(&buf, "/a/b/c.yaml", format)
			require.Error(t, err,
				"format %q must be rejected (case-sensitive match)",
				format)
		})
	}
}

// =============================================================================
// Error propagation
// =============================================================================

// TestWriteConfigPathText_PropagatesWriteError asserts that a writer
// error is returned unchanged from the text path.
//
// # Why the error must be returned
//
// The caller (runConfig) wraps the error with the "forge config:"
// prefix and returns it to the dispatcher. The dispatcher maps the
// error to a non-success exit code and writes the diagnostic to
// stderr. If the writer's error were swallowed, the user would see
// successful exit codes with no output — a silent failure.
func TestWriteConfigPathText_PropagatesWriteError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("write failed")
	err := writeConfigPathText(errWriter{err: sentinel}, "/a/b/c.yaml")

	require.Error(t, err)
	assert.ErrorIs(t, err, sentinel,
		"the writer's error must be returned unchanged")
}

// TestWriteConfigPathJSON_PropagatesWriteError asserts that a writer
// error is returned unchanged from the JSON path.
//
// # The distinction from the text case
//
// The JSON path uses json.Encoder, which wraps write errors in its
// own error type in some cases. The test asserts that the sentinel
// is still reachable via errors.Is, which means the encoder
// preserved the error chain (or that the error is not wrapped at
// all — either satisfies the assertion).
func TestWriteConfigPathJSON_PropagatesWriteError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("write failed")
	err := writeConfigPathJSON(errWriter{err: sentinel}, "/a/b/c.yaml")

	require.Error(t, err)
	assert.ErrorIs(t, err, sentinel,
		"the writer's error must be returned unchanged")
}

// TestWriteConfigPath_PropagatesWriteError asserts that the format
// dispatch layer also propagates the underlying writer error.
//
// The test is redundant with the two above in one sense — the
// dispatch layer just calls the writers — but it pins the
// end-to-end property: a caller that goes through writeConfigPath
// sees the writer's error, not a generic "render failed" error.
func TestWriteConfigPath_PropagatesWriteError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("write failed")

	for _, format := range []string{"text", "json"} {
		format := format
		t.Run(format, func(t *testing.T) {
			t.Parallel()
			err := writeConfigPath(errWriter{err: sentinel},
				"/a/b/c.yaml", format)
			require.Error(t, err)
			assert.ErrorIs(t, err, sentinel)
		})
	}
}
