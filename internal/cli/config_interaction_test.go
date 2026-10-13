package cli

import (
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
//   2. Flag interaction on full output: --quiet, --verbose; --config
//      with a resolved file.
//   3. Flag interaction on --path output: --path with --quiet and
//      --path with --verbose.
//
// The error cases (--quiet --verbose conflict; --config <missing>)
// are separate, because they exercise the boundary's error path
// rather than the command's success path. Their assertions pin the
// CLI-level behavior: the exit code, the empty stdout, the
// diagnostic on stderr.
//
// # How the assertions changed when the contracts changed
//
// The original version of this file (WBS 9.4.2) asserted that
// --verbose left stderr empty and that --quiet --verbose produced
// exit 0 (the "--quiet wins" rule). Two later WBS items changed
// those contracts:
//
//   - WBS 12.1.2 made --quiet and --verbose mutually exclusive.
//     The combination is now a usage error (exit 64), not a
//     silent precedence tie.
//
//   - WBS 12.5.1 wired the resolved log level into the
//     invocation's logger and had the config handler emit three
//     DEBUG records under --verbose: "config: resolving
//     invocation", "config: resolved path", "config: selected
//     format". Under --verbose, stderr is no longer empty.
//
// The rows below reflect the current contracts. The change is a
// deliberate, one-time update recorded in the commit that
// introduced the new behavior.
//
// # Why the assertions are structured this way
//
// The tests pin three things and nothing else:
//
//   - the exit code
//   - the presence and content of the output on each stream
//   - the invariants that no output appeared on the wrong stream
//
// They do not assert on the exact content of the DEBUG records
// (timestamps and field values vary by invocation). They assert
// on the records' *message names*, which are stable and are the
// names the reference doc lists as the config subsystem's records
// (docs/reference/verbose-mode.md §3.4).
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

// configDebugRecords is the set of DEBUG record message names the
// config handler emits under --verbose (WBS 12.5.1).
//
// The three names are the handler's contribution to the verbose
// surface. They are documented in docs/reference/verbose-mode.md
// §3.4. A future change to the handler's records updates this
// slice, the reference doc, and the rows that use it.
//
// The names are the stable parts of the records. The full lines
// carry timestamps and field values (mode, path, format) that
// vary by invocation; asserting on the names pins the record's
// identity without coupling the test to its rendering.
var configDebugRecords = []string{
	"config: resolving invocation",
	"config: resolved path",
	"config: selected format",
}

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
// row can delegate to compareGolden, assertStderrEmpty, or a
// bespoke assertion without inflating the table's struct.
//
// # Why a single test with one table
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
// observable effect on the invocation's streams.
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
			// if it is missing) is covered by
			// TestConfigInteraction_MissingConfig below and by
			// the config package's tests.
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
			// command's output. The config handler emits no
			// records at ERROR level, so stderr stays empty.
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
			// the log level to debug. The config handler emits
			// three DEBUG records on stderr (WBS 12.5.1):
			// resolving invocation, resolved path, selected
			// format.
			//
			// The stdout assertion is unchanged: debug output
			// never goes to stdout. The stderr assertion is
			// updated from assertStderrEmpty to
			// assertDebugRecordsPresent.
			name:     "config --verbose",
			args:     []string{"--verbose", "config"},
			cfg:      noPath,
			wantExit: ExitSuccess,
			checkOut: func(t *testing.T, stdout string) {
				t.Helper()
				compareGolden(t, fullOutputGoldenDefault, stdout)
			},
			checkErr: assertDebugRecordsPresent(configDebugRecords...),
		},
		{
			// Row 5: `forge config --quiet --verbose` — both
			// flags set. WBS 12.1.2 made the flags mutually
			// exclusive: the invocation is rejected as a usage
			// error before the tree runs.
			//
			// The row's pre-12.1.2 expectation was exit 0 (the
			// "--quiet wins" rule). The current rule is mutual
			// exclusion; the boundary renders the conflict and
			// returns ExitUsage (64).
			//
			// The conflict's exact behavior is pinned by
			// TestFlagInteraction_QuietVerboseConflict in
			// flag_interaction_test.go and by
			// TestGlobalFlags_QuietVerboseConflict in
			// root_test.go. This row pins the composition with
			// the config command: the command's own output is
			// not produced, and the conflict message appears on
			// stderr.
			name:     "config --quiet --verbose",
			args:     []string{"--quiet", "--verbose", "config"},
			cfg:      noPath,
			wantExit: ExitUsage,
			checkOut: func(t *testing.T, stdout string) {
				t.Helper()
				assert.Empty(t, stdout,
					"the conflict must be detected before the tree "+
						"runs; stdout must be empty")
			},
			checkErr: func(t *testing.T, stderr string) {
				t.Helper()
				assert.Contains(t, stderr, "mutually exclusive",
					"stderr must name the conflict")
			},
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
			// Row 7: `forge config --path --verbose` — the path
			// mode with verbose. The config handler emits three
			// DEBUG records on stderr (WBS 12.5.1); the --path
			// branch emits the same three message shapes but
			// with mode=path in the second record and no
			// `accepted` field in the third.
			//
			// The stdout assertion is unchanged: debug output
			// never goes to stdout.
			name:     "config --path --verbose",
			args:     []string{"--verbose", "config", "--path"},
			cfg:      withPath,
			wantExit: ExitSuccess,
			checkOut: func(t *testing.T, stdout string) {
				t.Helper()
				assert.Equal(t, configFullOutputPath+"\n", stdout,
					"--verbose must not change the path output")
			},
			checkErr: assertDebugRecordsPresent(configDebugRecords...),
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
// # Why the test does not go through the loader's success path
//
// The test invokes `forge config --config <missing>`. In the
// current architecture, the loader runs in the PersistentPreRunE
// hook, which executes before the config command's RunE. The hook
// returns the loader's error; runWithOptions renders it and maps
// it to exit 3. The test therefore exercises the full path
// (runWithOptions → newRootCmd → hook → loader → error), not the
// command handler. No config injection is involved.
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
//
// # Which rows use it
//
// The rows whose args do not include --verbose or --quiet
// --verbose. Under those args, the config handler emits no
// records, so stderr is empty by the handler's construction.
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

// assertDebugRecordsPresent returns a checkErr closure that
// asserts stderr contains each of the given substrings.
//
// # Why a closure factory and not a plain function
//
// The rows have different sets of expected DEBUG records (today
// they all use configDebugRecords, but a future row may expect a
// subset). A factory lets each row name the substrings it expects
// without the table carrying a []string column.
//
// # What the helper asserts
//
// For each substring in want, the returned closure asserts the
// substring appears in stderr. The helper does not assert order,
// count, or absence of other lines; the config handler's records
// are the subject of its own tests, and this file's purpose is to
// pin composition.
//
// # Why the helper does not assert "stderr is non-empty"
//
// The per-substring assertion implies non-emptiness: if every
// substring is present, stderr cannot be empty. A separate
// emptiness check would be redundant.
//
// # Why the substrings are message names, not full lines
//
// The full lines contain timestamps and field values that vary
// by invocation. The message names ("config: resolving
// invocation", "config: resolved path", "config: selected
// format") are stable across invocations and are what the
// reference doc lists as the config subsystem's records (see
// docs/reference/verbose-mode.md §3.4). Asserting on them is
// asserting on the record's identity, not its rendering.
func assertDebugRecordsPresent(want ...string) func(*testing.T, string) {
	return func(t *testing.T, stderr string) {
		t.Helper()
		for _, w := range want {
			assert.Contains(t, stderr, w,
				"stderr must contain the DEBUG record %q", w)
		}
	}
}
