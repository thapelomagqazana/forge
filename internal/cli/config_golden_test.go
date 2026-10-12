package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thapelomagqazana/forge/internal/config"
)

// ---------------------------------------------------------------------------
// Golden-file tests for `forge config`.
//
// This file pins the byte-exact output of `forge config` across the
// three frozen output surfaces:
//
//   - text (WBS 9.2.1):        `forge config`
//   - path (WBS 9.3.1):        `forge config --path`
//   - JSON (WBS 9.4.1):        `forge config --format json`
//
// The tests use the runCLI / runCLIWithConfig helpers, so the full
// dispatch path (options → executeWithOptions → Dependencies →
// command tree → runConfig → renderer) is exercised. They compare
// the captured stdout to a golden file in testdata/config/.
//
// # The golden files
//
//	testdata/config/default.golden.txt           text, no file
//	testdata/config/with-file.golden.txt         text, file resolved
//	testdata/config/path-with-file.golden.txt    path, file resolved
//	testdata/config/path-without-file.golden.txt path, no file
//	testdata/config/path-json.golden.txt         path, JSON mode
//	testdata/config/default.json                 JSON, no file
//	testdata/config/with-file.json               JSON, file resolved
//
// # How to regenerate
//
// Every golden test respects the package's -update flag. Regenerate
// a golden file only after confirming the new output is the
// intended output; the file's diff is the review surface.
//
//	go test ./internal/cli -run TestConfigGolden -update
//
// # Why golden tests here and not in config_test.go
//
// config_test.go asserts the command's metadata and dispatch
// behavior (stream boundary, exit codes). This file asserts the
// rendering: what bytes reach stdout. A change to the format
// fails here; a change to the dispatch fails in config_test.go.
// Keeping the two concerns separate makes each file's failure
// mode unambiguous.
// ---------------------------------------------------------------------------

// =============================================================================
// Text mode — WBS 9.2.1
// =============================================================================

// TestConfigGolden_TextDefault asserts the text output when no
// config file was resolved. This is the ATDD acceptance test for
// the minimal form of WBS 9.2.1's frozen format.
//
// The test uses the plain runCLI helper, which constructs the
// Dependencies from options. The Config in those Dependencies is
// the zero value, so SourcePath is empty and both value fields
// carry SourceDefault. The rendered output is the all-defaults
// case.
func TestConfigGolden_TextDefault(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"config"}, nil)
	require.Equal(t, ExitSuccess, got.exitCode,
		"`forge config` must succeed; stderr: %q", got.stderr)

	compareGolden(t, "testdata/config/default.golden.txt", got.stdout)
}

// TestConfigGolden_TextWithFile asserts the text output when a
// config file was resolved.
//
// The test uses runCLIWithConfig to inject a Config whose
// SourcePath is set. The values and their sources are the
// defaults, so the rendered output differs from the default
// case only in the Source: line.
func TestConfigGolden_TextWithFile(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		SourcePath: "/home/user/.config/forge/config.yaml",
	}
	got := runCLIWithConfig(t, []string{"config"}, nil, cfg)
	require.Equal(t, ExitSuccess, got.exitCode,
		"`forge config` must succeed; stderr: %q", got.stderr)

	compareGolden(t, "testdata/config/with-file.golden.txt", got.stdout)
}

// =============================================================================
// Path mode — WBS 9.3.1
// =============================================================================

// TestConfigGolden_PathWithFile asserts AC1: `forge config --path`
// with a resolved file prints the path on one line.
//
// The golden file contains the path and a trailing newline. No
// label, no decoration.
func TestConfigGolden_PathWithFile(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		SourcePath: "/home/user/.config/forge/config.yaml",
	}
	got := runCLIWithConfig(t, []string{"config", "--path"}, nil, cfg)
	require.Equal(t, ExitSuccess, got.exitCode,
		"`forge config --path` must succeed; stderr: %q", got.stderr)

	compareGolden(t, "testdata/config/path-with-file.golden.txt", got.stdout)
}

// TestConfigGolden_PathWithoutFile asserts AC3: `forge config
// --path` with no resolved file prints the literal "<none>".
//
// The golden file contains "<none>" and a trailing newline.
func TestConfigGolden_PathWithoutFile(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{} // SourcePath is empty
	got := runCLIWithConfig(t, []string{"config", "--path"}, nil, cfg)
	require.Equal(t, ExitSuccess, got.exitCode,
		"`forge config --path` must succeed even with no file; "+
			"stderr: %q", got.stderr)

	compareGolden(t, "testdata/config/path-without-file.golden.txt", got.stdout)
}

// TestConfigGolden_PathJSON asserts AC5: `forge config --path
// --format json` prints a JSON object with a single key.
//
// The key is `config_path` (WBS 9.4.1 renamed it from `path`).
// When the path is set, the value is a string; when it is absent,
// the value is null.
func TestConfigGolden_PathJSON(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		SourcePath: "/home/user/.config/forge/config.yaml",
	}
	got := runCLIWithConfig(t,
		[]string{"config", "--path", "--format", "json"}, nil, cfg)
	require.Equal(t, ExitSuccess, got.exitCode,
		"`forge config --path --format json` must succeed; "+
			"stderr: %q", got.stderr)

	compareGolden(t, "testdata/config/path-json.golden.txt", got.stdout)
}

// TestConfigGolden_PathJSONWithoutFile asserts the null case of
// `forge config --path --format json`: when no file was resolved,
// the JSON value is null, not the empty string.
//
// The test uses the zero Config, so SourcePath is empty. The
// output is short and deterministic; an inline assertion pins the
// exact bytes rather than a golden file.
func TestConfigGolden_PathJSONWithoutFile(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{} // SourcePath is empty
	got := runCLIWithConfig(t,
		[]string{"config", "--path", "--format", "json"}, nil, cfg)
	require.Equal(t, ExitSuccess, got.exitCode,
		"stderr: %q", got.stderr)

	assert.Equal(t, "{\"config_path\":null}\n", got.stdout,
		"an empty path must encode as null, not as the empty string")
	assert.Empty(t, got.stderr)
}

// TestConfigGolden_PathQuiet asserts AC7: --quiet does not suppress
// --path output.
//
// The --quiet flag lowers the logger's level; it does not affect
// the command's stdout. The test pins the property.
func TestConfigGolden_PathQuiet(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		SourcePath: "/home/user/.config/forge/config.yaml",
	}
	got := runCLIWithConfig(t,
		[]string{"--quiet", "config", "--path"}, nil, cfg)
	require.Equal(t, ExitSuccess, got.exitCode,
		"`forge --quiet config --path` must succeed; stderr: %q",
		got.stderr)

	assert.Equal(t, "/home/user/.config/forge/config.yaml\n", got.stdout,
		"--quiet must not suppress the path (it is a result, "+
			"not a diagnostic)")
}

// =============================================================================
// JSON mode — WBS 9.4.1
// =============================================================================

// TestConfigGolden_JSONDefault asserts the JSON output when no
// config file was resolved and all values are defaults.
//
// The output is a single-line JSON object with config_path = null
// and all sources = "default".
func TestConfigGolden_JSONDefault(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{} // zero value → defaults
	got := runCLIWithConfig(t, []string{"config", "--format", "json"}, nil, cfg)
	require.Equal(t, ExitSuccess, got.exitCode,
		"`forge config --format json` must succeed; stderr: %q",
		got.stderr)

	compareGolden(t, "testdata/config/default.json", got.stdout)
}

// TestConfigGolden_JSONWithFile asserts the JSON output when a
// config file was resolved.
//
// The test injects a Config with a resolved SourcePath and
// non-default value fields, so the JSON output differs from the
// default case in config_path, log_level, and output_format.
func TestConfigGolden_JSONWithFile(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		SourcePath:   "/home/user/.config/forge/config.yaml",
		LogLevel:     "debug",
		OutputFormat: "json",
	}
	got := runCLIWithConfig(t, []string{"config", "--format", "json"}, nil, cfg)
	require.Equal(t, ExitSuccess, got.exitCode,
		"stderr: %q", got.stderr)

	compareGolden(t, "testdata/config/with-file.json", got.stdout)
}

// TestConfigGolden_JSONSingleLine asserts AC5 for the JSON full
// output: the output is a single line.
//
// The golden file comparison already implies this, but the
// assertion is stated explicitly so that a future change to the
// encoder's configuration (e.g. calling SetIndent) fails here with
// a specific message rather than as a golden-file diff.
func TestConfigGolden_JSONSingleLine(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		SourcePath: "/a/b/c.yaml",
	}
	got := runCLIWithConfig(t, []string{"config", "--format", "json"}, nil, cfg)
	require.Equal(t, ExitSuccess, got.exitCode)

	assert.Equal(t, 1, strings.Count(got.stdout, "\n"),
		"JSON output must have exactly one newline; got %q", got.stdout)
	assert.True(t, strings.HasSuffix(got.stdout, "\n"),
		"JSON output must end with a newline; got %q", got.stdout)
}

// =============================================================================
// Cross-format invariants
// =============================================================================

// TestConfigGolden_PathModesShareKey asserts that the two JSON
// modes (--path and the full output) use the same key for the
// resolved config file path.
//
// # Why the test exists
//
// WBS 9.3.1 originally named the --path mode's key "path". WBS
// 9.4.1 renamed it to "config_path" to align with the full
// output's schema. The alignment is a deliberate decision; the
// test pins it. A future change that renamed one mode's key
// without the other would fail this test.
func TestConfigGolden_PathModesShareKey(t *testing.T) {
	t.Parallel()

	const path = "/a/b/c.yaml"
	cfg := &config.Config{SourcePath: path}

	// --path mode.
	pathGot := runCLIWithConfig(t,
		[]string{"config", "--path", "--format", "json"}, nil, cfg)
	require.Equal(t, ExitSuccess, pathGot.exitCode)

	// Full output mode.
	fullGot := runCLIWithConfig(t,
		[]string{"config", "--format", "json"}, nil, cfg)
	require.Equal(t, ExitSuccess, fullGot.exitCode)

	// Both must contain the key with the path as its value. The
	// path mode's object has only that key; the full output's
	// object has that key among five. The substring check is
	// sufficient: the JSON encoding has no spaces and no
	// reordering, so the exact byte sequence is deterministic.
	assert.Contains(t, pathGot.stdout,
		`"`+configJSONKeyConfigPath+`":"`+path+`"`,
		"--path mode must use the shared key %q",
		configJSONKeyConfigPath)
	assert.Contains(t, fullGot.stdout,
		`"`+configJSONKeyConfigPath+`":"`+path+`"`,
		"full output must use the shared key %q",
		configJSONKeyConfigPath)
}

// TestConfigGolden_TextAndJSONAgree asserts that the text and JSON
// outputs describe the same configuration.
//
// # What "agree" means
//
// The two formats render the same ResolvedConfig. The test does
// not compare the outputs byte-for-byte (they are different
// formats), but it checks that the values the two formats carry
// match: a change to the resolution pipeline that affected only
// one format would fail this test.
//
// The test uses a Config with non-default values, so the
// agreement is not trivially satisfied by both outputs reporting
// the same defaults.
//
// # Why the check is substring-based
//
// A stricter check would parse the JSON and compare each field to
// the text output's corresponding line. That is the job of the
// per-field tests in config_json_test.go and the per-line tests in
// config_print_test.go. This test is a smoke check across the two
// renderers; it ensures the resolution pipeline feeds both of them
// the same values.
func TestConfigGolden_TextAndJSONAgree(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		SourcePath:   "/a/b/c.yaml",
		LogLevel:     "debug",
		OutputFormat: "json",
	}

	textGot := runCLIWithConfig(t, []string{"config"}, nil, cfg)
	jsonGot := runCLIWithConfig(t,
		[]string{"config", "--format", "json"}, nil, cfg)

	require.Equal(t, ExitSuccess, textGot.exitCode)
	require.Equal(t, ExitSuccess, jsonGot.exitCode)

	for _, value := range []string{
		cfg.SourcePath,
		cfg.LogLevel,
		cfg.OutputFormat,
	} {
		assert.Contains(t, textGot.stdout, value,
			"text output must contain %q", value)
		assert.Contains(t, jsonGot.stdout, value,
			"JSON output must contain %q", value)
	}
}

// =============================================================================
// Test helper
// =============================================================================

// compareGolden reads the golden file at path (relative to the test
// package directory) and asserts that got matches it byte for byte.
//
// # Regeneration
//
// If the package's -update flag is set, the function rewrites the
// golden file with got and skips the comparison. Regeneration is
// opt-in so that a contributor who runs the tests without -update
// sees the drift as a failure, not as a silent rewrite.
//
// # Missing golden files
//
// If the golden file does not exist and -update is not set, the
// test fails with a message naming the file and the flag. The
// first run after adding a new golden test must use -update to
// create the file; subsequent runs use the file as the expected
// value.
//
// # Path handling
//
// The path is joined with the test's working directory (the
// package directory, by go test's convention). A caller that
// passes a relative path gets the expected behavior; an absolute
// path is used as-is.
//
// # Why a helper and not a framework
//
// The helper is deliberately minimal: read, compare, optionally
// rewrite. A more elaborate golden-testing library would add a
// dependency for a small amount of saved code. The helper's three
// behaviors (read, compare, update) are the entire contract.
func compareGolden(t *testing.T, path, got string) {
	t.Helper()

	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create golden dir: %v", err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden %s: %v", path, err)
		}
		t.Logf("updated golden file: %s", path)
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v\n"+
			"(run with -update to generate the file)", path, err)
	}

	require.Equal(t, string(want), got,
		"output drifted from golden %s\n"+
			"(run with -update if the change is intentional)", path)
}
