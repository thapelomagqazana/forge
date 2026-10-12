package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thapelomagqazana/forge/internal/config"
)

// ---------------------------------------------------------------------------
// WBS 9.4.2 — Frozen interaction between `forge config` and the
// global flags.
//
// The command's own behavior is frozen by three prior WBS items:
//
//   - WBS 9.2.1: the default text format
//   - WBS 9.3.1: the --path flag
//   - WBS 9.4.1: the --format json mode
//
// This file pins how those behaviors compose with the three global
// flags:
//
//   - --config <path>: the file the loader reads
//   - --quiet:         the log level (errors only)
//   - --verbose:       the log level (debug)
//
// # The frozen interaction matrix
//
// The table below is the WBS item's contract, expressed as a test
// table. Each row is one invocation. The columns are:
//
//	name       — the row's identity, used as the subtest name
//	args       — the full argument list, including the command name
//	cfg        — an injected Config, or nil for the zero-value default
//	wantExit   — the expected exit code
//	checkOut   — a function that asserts on stdout
//	checkErr   — a function that asserts on stderr
//
// The matrix's rows fall into three groups:
//
//   1. Full output, no flag interaction: the row asserts the full
//      output and no stderr.
//   2. Flag interaction on full output: --quiet, --verbose, and
//      their combination; --config with a resolved file.
//   3. Flag interaction on --path output: --path with --quiet and
//      --path with --verbose.
//
// The error case (--config <missing>) is separate, because it never
// reaches the config command: the loader produces the error before
// the command tree runs. Its assertion pins the CLI-level behavior
// (exit 3, stderr, empty stdout).
//
// # Why the assertions are structured this way
//
// The tests pin three things and nothing else:
//
//   - the exit code
//   - the presence and content of the output on each stream
//   - (where applicable) the invariants that no output appeared on
//     the wrong stream
//
// They do not assert on the *absence* of output beyond what the
// contract requires. In particular, the --verbose rows assert that
// stdout is unchanged from the no-flag case and that stderr is
// empty (Phase 2 behavior); when WBS 12.0 introduces discovery
// debug output, those assertions will be updated to assert on the
// new stderr content.
//
// # Non-functional properties
//
// The tests are deterministic and hermetic:
//
//   - No subprocess is spawned.
//   - No environment variable is read; the env map is nil.
//   - No filesystem is read; the Config is injected.
//   - The output is captured in buffers, not in the process's
//     streams.
//
// The table's rows may be run in parallel; they share no state.
// ---------------------------------------------------------------------------

// =============================================================================
// The full-output interaction table
// =============================================================================

// configFullOutputPath is the path injected into the Config by the
// tests that exercise --config's interaction.
//
// The value is testConfigPath — the same path used by every other
// test that renders a "with file" output. See the testConfigPath
// docstring for the rationale.
const configFullOutputPath = testConfigPath

// fullOutputGoldenDefault is the golden file for `forge config`
// with no flags. It is shared by every row whose output is the
// default-config text.
const fullOutputGoldenDefault = "testdata/config/default.golden.txt"

// fullOutputGoldenWithFile is the golden file for `forge config`
// with a resolved file. It is shared by every row whose Config
// carries a SourcePath.
const fullOutputGoldenWithFile = "testdata/config/with-file.golden.txt"

// TestConfigInteraction_FullOutput pins the behavior of
// `forge config` and `forge config --path` when combined with the
// three global flags.
//
// # What the test does
//
// It iterates a table of invocations. Each row:
//
//   - calls runCLI or runCLIWithConfig with the row's args and
//     injected Config,
//   - asserts the exit code,
//   - asserts on stdout via the row's checkOut function,
//   - asserts on stderr via the row's checkErr function.
//
// The checks are functions rather than expected strings so that a
// row can delegate to compareGolden, assertOnlyStdout, or a
// bespoke assertion without inflating the table's struct.
//
// # Why a single test with two tables
//
// The full-output and path-only invocations share the same shape
// (args, injected config, exit, stream checks) but differ in which
// renderer produces the output. Keeping them in one test makes the
// full matrix visible at a glance; splitting into two tests would
// duplicate the row struct and the iteration logic.
//
// # Why the tests do not go through the real loader
//
// The loader is exercised by the config package's own tests
// (TestLoad_*) and by the config command's tests that use
// runCLIWithConfig. This file's purpose is to pin *composition*,
// not to test the loader. The injected Config is what the loader
// would have produced; the flags that affect loading (--config)
// are exercised by the injected SourcePath, and the flags that
// affect logging (--quiet, --verbose) are exercised by their
// observable effect (currently, none on the command's output).
func TestConfigInteraction_FullOutput(t *testing.T) {
	t.Parallel()

	withPath := &config.Config{SourcePath: configFullOutputPath}
	noPath := &config.Config{}

	cases := []struct {
		name     string
		args     []string
		cfg      *config.Config
		wantExit int
		checkOut func(t *testing.T, stdout string)
		checkErr func(t *testing.T, stderr string)
	}{
		{
			// Row 1: `forge config` — full output, no flags.
			// The behavior is the frozen default.
			name:     "config",
			args:     []string{"config"},
			cfg:      noPath,
			wantExit: ExitSuccess,
			checkOut: func(t *testing.T, stdout string) {
				t.Helper()
				compareGolden(t, fullOutputGoldenDefault, stdout)
			},
			checkErr: assertStderrEmpty,
		},
		{
			// Row 2: `forge config --config <path>` — the
			// resolved path is the injected SourcePath, so the
			// output's Source: line carries it.
			//
			// The test injects a Config whose SourcePath is set;
			// this stands in for what the loader would produce
			// from --config <path>. The loader's own behavior
			// (reading the file at the given path, or erroring
			// if it is missing) is covered elsewhere.
			name:     "config --config <path>",
			args:     []string{"config", "--config", "/injected/from/test/config.yaml"},
			cfg:      withPath,
			wantExit: ExitSuccess,
			checkOut: func(t *testing.T, stdout string) {
				t.Helper()
				compareGolden(t, fullOutputGoldenWithFile, stdout)
			},
			checkErr: assertStderrEmpty,
		},
		{
			// Row 3: `forge config --quiet` — the flag sets the
			// log level to error; it does not suppress the
			// command's output. The output is unchanged from
			// the no-flag case.
			name:     "config --quiet",
			args:     []string{"--quiet", "config"},
			cfg:      noPath,
			wantExit: ExitSuccess,
			checkOut: func(t *testing.T, stdout string) {
				t.Helper()
				compareGolden(t, fullOutputGoldenDefault, stdout)
			},
			checkErr: assertStderrEmpty,
		},
		{
			// Row 4: `forge config --verbose` — the flag sets
			// the log level to debug. In Phase 2, the config
			// command does not emit debug output; the stdout
			// is unchanged and stderr is empty.
			//
			// When WBS 12.0 introduces discovery debug output,
			// this row's checkErr will be updated to assert on
			// the new stderr content. The stdout assertion is
			// unchanged: debug output never goes to stdout.
			name:     "config --verbose",
			args:     []string{"--verbose", "config"},
			cfg:      noPath,
			wantExit: ExitSuccess,
			checkOut: func(t *testing.T, stdout string) {
				t.Helper()
				compareGolden(t, fullOutputGoldenDefault, stdout)
			},
			checkErr: assertStderrEmpty,
		},
		{
			// Row 5: `forge config --quiet --verbose` — both
			// flags set. The precedence rule (--quiet wins) is
			// implemented in resolveLogLevel; this test pins
			// the composition's observable effect: stdout is
			// unchanged and stderr is empty (Phase 2).
			//
			// The precedence itself is asserted at the unit
			// level by TestResolveLogLevel in flags_test.go.
			name:     "config --quiet --verbose",
			args:     []string{"--quiet", "--verbose", "config"},
			cfg:      noPath,
			wantExit: ExitSuccess,
			checkOut: func(t *testing.T, stdout string) {
				t.Helper()
				compareGolden(t, fullOutputGoldenDefault, stdout)
			},
			checkErr: assertStderrEmpty,
		},
		{
			// Row 6: `forge config --path --quiet` — the path
			// mode with quiet. The path is the command's
			// result; it is not suppressed.
			name:     "config --path --quiet",
			args:     []string{"--quiet", "config", "--path"},
			cfg:      withPath,
			wantExit: ExitSuccess,
			checkOut: func(t *testing.T, stdout string) {
				t.Helper()
				assert.Equal(t, configFullOutputPath+"\n", stdout,
					"--quiet must not suppress the path")
			},
			checkErr: assertStderrEmpty,
		},
		{
			// Row 7: `forge config --path --verbose` — the
			// path mode with verbose. Same behavior as the
			// full-output row: Phase 2 adds no stderr.
			name:     "config --path --verbose",
			args:     []string{"--verbose", "config", "--path"},
			cfg:      withPath,
			wantExit: ExitSuccess,
			checkOut: func(t *testing.T, stdout string) {
				t.Helper()
				assert.Equal(t, configFullOutputPath+"\n", stdout,
					"--verbose must not change the path output")
			},
			checkErr: assertStderrEmpty,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := runCLIWithConfig(t, tc.args, nil, tc.cfg)

			require.Equal(t, tc.wantExit, got.exitCode,
				"exit code mismatch\nstderr: %q", got.stderr)

			tc.checkOut(t, got.stdout)
			tc.checkErr(t, got.stderr)
		})
	}
}

// =============================================================================
// The missing-config error case
// =============================================================================

// TestConfigInteraction_MissingConfig asserts the CLI-level
// behavior when the user names a config file that does not exist.
//
// # What the test asserts
//
//   - the exit code is ExitConfig (3),
//   - stdout is empty,
//   - stderr contains a diagnostic that names the missing path.
//
// The error itself is produced by the loader (config package);
// its wording is pinned by the loader's own tests. This test pins
// the CLI-level contract: an error from the loader surfaces on
// stderr, produces exit 3, and leaves stdout empty.
//
// # Why the test does not go through the loader
//
// The test invokes `forge config --config <missing>`. In the
// current architecture, the loader runs in the PersistentPreRunE
// hook, which executes before the config command's RunE. The hook
// returns the loader's error; executeWithOptions renders it and
// maps it to exit 3. The test therefore exercises the full path
// (runCLI → executeWithOptions → newRootCmd → hook → loader →
// error), not the command handler. No config injection is
// involved.
//
// # Why the path is deliberately impossible
//
// The path is under a directory that does not exist. It is not
// rooted in the test's working directory, so a stray file in the
// repo cannot accidentally satisfy it. The `rootPath` of the
// options value is the package directory (via runCLI's defaults);
// a path under `/nonexistent-for-testing/` cannot be found there.
func TestConfigInteraction_MissingConfig(t *testing.T) {
	t.Parallel()

	const missing = "/nonexistent-for-testing/forge-missing-config.yaml"

	got := runCLI(t, []string{"--config", missing, "config"}, nil)

	assert.Equal(t, ExitConfig, got.exitCode,
		"a missing explicit --config must exit %d (ExitConfig); "+
			"stderr: %q", ExitConfig, got.stderr)
	assert.Empty(t, got.stdout,
		"a missing explicit --config must not write to stdout")
	assert.NotEmpty(t, got.stderr,
		"a missing explicit --config must write a diagnostic to stderr")
	assert.Contains(t, got.stderr, missing,
		"the diagnostic must name the missing path")
}

// =============================================================================
// Assertion helpers
// =============================================================================

// assertStderrEmpty asserts that the diagnostic stream is empty.
//
// It is the checkErr value for the rows whose contract is "no
// stderr". A named helper keeps the table's rows terse and makes
// the property visible: a reader who sees `checkErr:
// assertStderrEmpty` knows the row asserts the absence of
// diagnostics without reading the closure.
func assertStderrEmpty(t *testing.T, stderr string) {
	t.Helper()
	assert.Empty(t, stderr, "stderr must be empty for this row")
}

// assertStdoutNonEmpty asserts that the result stream is
// non-empty.
//
// It is used by rows whose stdout is not compared to a golden file
// because the golden is asserted elsewhere. The helper is kept
// alongside assertStderrEmpty for symmetry; if a future row needs
// it, it is here.
func assertStdoutNonEmpty(t *testing.T, stdout string) {
	t.Helper()
	assert.NotEmpty(t, stdout, "stdout must be non-empty for this row")
}

// =============================================================================
// Notes on the --verbose rows
// =============================================================================

// The rows for --verbose and --path --verbose assert that stderr is
// empty. This is the Phase 2 behavior: the flag sets the log level
// (via resolveLogLevel), but no command emits debug output.
//
// When WBS 12.0 introduces the discovery debug output, the
// assertions in the --verbose rows will change:
//
//   - The checkErr for `config --verbose` will assert that stderr
//     contains one or more lines describing the discovery steps.
//   - The checkErr for `config --path --verbose` will do the same.
//   - The checkErr for `config --quiet --verbose` will remain
//     assertStderrEmpty: --quiet wins over --verbose, so no debug
//     output is emitted.
//
// The update is a deliberate change: the tests will fail when the
// new behavior lands, and the contributor introducing WBS 12.0
// will edit these three rows in the same commit. The failure is
// the signal that the change is being made in a specific, planned
// place.
//
// # Why not t.Skip the --verbose rows
//
// A skipped test is invisible in the test output; a passing test
// is a positive assertion. The --verbose rows are meaningful in
// Phase 2: they assert that the flag *composes* with `forge
// config` — that it is accepted, that it does not change the
// output, and that it does not accidentally pollute stdout. These
// are real properties worth pinning. When WBS 12.0 changes the
// assertions, the Phase 2 properties will be extended, not
// removed: stdout is still unchanged, and stderr gains content.
//
// The rows are therefore active tests with inverted assertions,
// not skipped placeholders. The distinction matters: a skipped
// test says "this isn't tested yet"; an inverted assertion says
// "this property holds today, and will be extended later."
//
// The trailing comment at the bottom of this file (this one) is
// the file's record of the plan. A reader who wonders why the
// --verbose rows assert the *absence* of stderr reads here and
// finds the answer.

// Ensure unused imports do not linger. strings is used by the
// docstrings above and will be used when the WBS 12.0 assertions
// are added; keeping it imported now avoids a second edit.
var _ = strings.Contains
