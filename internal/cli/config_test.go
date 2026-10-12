package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// WBS 9.1.1 + WBS 9.2.1 + WBS 9.3.1 — Tests for the `forge config`
// command.
//
// This file covers three frozen contracts:
//
//   - WBS 9.1.1 froze the command's identity and metadata (Use, Short,
//     Long, Example, Args, Aliases, Hidden). Those assertions are
//     independent of the command's behavior and are unaffected by
//     WBS 9.2.1 or WBS 9.3.1.
//
//   - WBS 9.2.1 froze the command's default text output format and
//     attached behavior to RunE. The format's byte-level rules are
//     tested in config_print_test.go; the format's end-to-end
//     rendering is tested in config_golden_test.go. This file asserts
//     only that the dispatcher reaches the handler and produces the
//     expected stream boundary.
//
//   - WBS 9.3.1 froze the --path flag's metadata (presence, type,
//     default, scope) and its behavioral semantics. The behavioral
//     tests live in config_path_test.go (unit) and
//     config_golden_test.go (end-to-end); this file asserts the flag's
//     metadata.
//
// The tests use the two helpers provided by the package:
//
//   - testDependencies() — for tests that construct a command and
//     inspect its metadata without executing it (TDD / unit level).
//
//   - runCLI(t, args, env) — for tests that execute the command
//     through the real boundary (executeWithOptions → Dependencies →
//     command tree) and capture the observable output (ATDD / BDD
//     level). This is the helper the package docstring designates as
//     the single entry point for command execution tests.
//
// No direct Cobra construction appears in this file: the package
// convention is that every command test goes through one of the two
// helpers above, so that tests exercise the same wiring production
// code uses.
// ---------------------------------------------------------------------------

// =============================================================================
// WBS 9.1.1 — Frozen metadata.
// =============================================================================

// TestConfigCommand_Identity asserts AC1 and AC6: Use is exactly
// "config", and no aliases are declared.
//
// This is a metadata-only test: it constructs the command via
// newConfigCmd but does not execute it. testDependencies supplies a
// Dependencies value with non-nil writers and collaborators.
func TestConfigCommand_Identity(t *testing.T) {
	t.Parallel()

	cmd := newConfigCmd(testDependencies())

	// AC1: Use is exactly "config".
	assert.Equal(t, "config", cmd.Use,
		"Use must be exactly %q per WBS 9.1.1 AC1", "config")

	// AC6: no aliases.
	assert.Empty(t, cmd.Aliases,
		"aliases are forbidden per WBS 9.1.1 AC6 (cfg/conf rejected)")
}

// TestConfigCommand_ShortLength asserts AC2: Short is ≤ 60 runes and
// uses imperative mood (heuristic: first word does not end in "s").
//
// The contract says "60 characters"; the test uses len() on a string
// of ASCII. If Short ever contains non-ASCII, switch to
// utf8.RuneCountInString. The current value is ASCII, so len() is
// exact and cheaper.
func TestConfigCommand_ShortLength(t *testing.T) {
	t.Parallel()

	cmd := newConfigCmd(testDependencies())

	// AC2: ≤ 60 characters.
	assert.LessOrEqual(t, len(cmd.Short), 60,
		"Short must be ≤ 60 chars per WBS 7.2.1 / AC2; got %d: %q",
		len(cmd.Short), cmd.Short)

	// Imperative-mood heuristic: the first word should not be
	// third-person singular (i.e. must not end in "s"). This is a
	// soft check; the authoritative gate is the metadata linter
	// described in WBS 7.2.1.
	firstWord := strings.Fields(cmd.Short)[0]
	assert.False(t, strings.HasSuffix(firstWord, "s"),
		"Short should use imperative mood per WBS 7.2.1; got %q", cmd.Short)
}

// TestConfigCommand_ShortBoundary exercises the exact 60-char boundary
// of AC2. A 60-char Short must pass; 61 must fail. The assertions are
// on synthetic strings, not on the command's Short, so that the
// boundary itself is documented independently of the current value.
func TestConfigCommand_ShortBoundary(t *testing.T) {
	t.Parallel()

	// Exactly 60 chars (the boundary — inclusive).
	exact60 := strings.Repeat("a", 60)
	assert.LessOrEqual(t, len(exact60), 60,
		"60-char Short must be accepted (AC2 boundary is inclusive)")

	// 61 chars (just over the boundary).
	over61 := strings.Repeat("a", 61)
	assert.Greater(t, len(over61), 60,
		"61-char Short must be rejected (AC2 boundary is inclusive)")
}

// TestConfigCommand_LongEndsWithPeriod asserts AC3: Long ends with a
// period. Wrapped lines are allowed; only the final character matters.
func TestConfigCommand_LongEndsWithPeriod(t *testing.T) {
	t.Parallel()

	cmd := newConfigCmd(testDependencies())
	trimmed := strings.TrimRight(cmd.Long, "\n")

	require.NotEmpty(t, cmd.Long, "Long must be set per WBS 7.2.1")
	assert.True(t, strings.HasSuffix(trimmed, "."),
		"Long must end with a period per WBS 9.1.1 AC3; got %q", cmd.Long)
}

// TestConfigCommand_LongWrapsAt80 asserts the WBS 7.2.1 wrapping rule:
// no line in Long exceeds 80 columns.
func TestConfigCommand_LongWrapsAt80(t *testing.T) {
	t.Parallel()

	cmd := newConfigCmd(testDependencies())
	for i, line := range strings.Split(cmd.Long, "\n") {
		assert.LessOrEqual(t, len(line), 80,
			"Long line %d exceeds 80 cols (WBS 7.2.1): %q", i, line)
	}
}

// TestConfigCommand_LongHasParagraphBreak is a corner-case test:
// Long must contain at least one blank line separating the summary
// paragraph from the --path paragraph. Without it, the rendered help
// is a wall of text.
func TestConfigCommand_LongHasParagraphBreak(t *testing.T) {
	t.Parallel()

	cmd := newConfigCmd(testDependencies())
	lines := strings.Split(cmd.Long, "\n")

	hasBlank := false
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			hasBlank = true
			break
		}
	}
	assert.True(t, hasBlank, "Long must contain a paragraph break")
}

// TestConfigCommand_NoArgs asserts AC4: Args is cobra.NoArgs, meaning
// any positional argument is rejected.
func TestConfigCommand_NoArgs(t *testing.T) {
	t.Parallel()

	cmd := newConfigCmd(testDependencies())
	require.NotNil(t, cmd.Args, "Args must be set per AC4")

	// Positive: zero args accepted.
	err := cmd.Args(cmd, []string{})
	assert.NoError(t, err, "zero positional args must be accepted")

	// Negative: any positional arg rejected.
	err = cmd.Args(cmd, []string{"extra"})
	assert.Error(t, err, "positional args must be rejected per AC4")
	assert.Contains(t, err.Error(), "unknown command",
		"rejection must use Cobra's standard NoArgs message")
}

// TestConfigCommand_ExampleHasThreeLines asserts AC5: the Example
// field contains exactly three non-empty lines.
func TestConfigCommand_ExampleHasThreeLines(t *testing.T) {
	t.Parallel()

	cmd := newConfigCmd(testDependencies())
	require.NotEmpty(t, cmd.Example, "Example is mandatory per WBS 7.2.2")

	lines := strings.Split(strings.TrimRight(cmd.Example, "\n"), "\n")
	assert.Len(t, lines, 3,
		"Example must have exactly 3 lines per WBS 9.1.1 AC5; got %d: %q",
		len(lines), cmd.Example)

	for i, l := range lines {
		assert.NotEmpty(t, strings.TrimSpace(l),
			"Example line %d must not be empty", i)
	}
}

// TestConfigCommand_ExampleCoversPrimaryInvocations is a BDD test:
// the three Example lines must correspond to the three primary use
// cases (default, --path, --format json).
func TestConfigCommand_ExampleCoversPrimaryInvocations(t *testing.T) {
	t.Parallel()

	cmd := newConfigCmd(testDependencies())

	for _, want := range []string{
		"forge config\n",             // default
		"forge config --path",        // path only
		"forge config --format json", // json output
	} {
		assert.Contains(t, cmd.Example, want,
			"Example must cover %q", strings.TrimSpace(want))
	}
}

// TestConfigCommand_NotHidden asserts that the placeholder's
// Hidden:true is removed. The command is now user-visible.
func TestConfigCommand_NotHidden(t *testing.T) {
	t.Parallel()

	cmd := newConfigCmd(testDependencies())
	assert.False(t, cmd.Hidden,
		"config command must be user-visible after WBS 9.1.1")
}

// =============================================================================
// WBS 9.2.1 — Behavior wiring.
//
// These tests assert that the dispatcher reaches the config handler
// and that the handler obeys the stream boundary. The format's
// byte-level rules are asserted in config_print_test.go and
// config_golden_test.go; this file asserts only the wiring.
// =============================================================================

// TestConfigCommand_RegisteredInRegistry asserts AC7: the command is
// reachable from the root command built through the real registry,
// and its handler produces the expected stream behavior.
//
// The test goes through runCLI so that the dispatch path (options →
// executeWithOptions → Dependencies → command tree) is exercised
// exactly as in production.
//
// # Why this test no longer asserts a sentinel
//
// At WBS 9.1.1, RunE returned a notImplementedError sentinel, and
// this test asserted that the sentinel surfaced on stderr. At WBS
// 9.2.1, RunE calls runConfig and prints the resolved configuration
// to stdout. The sentinel type no longer exists. The stream-boundary
// assertion flips accordingly: the command now succeeds, output goes
// to stdout, and stderr is empty.
func TestConfigCommand_RegisteredInRegistry(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"config"}, nil)

	require.Equal(t, ExitSuccess, got.exitCode,
		"config must be reachable and succeed; stderr: %q", got.stderr)
	assert.NotEmpty(t, got.stdout,
		"config must write its output to stdout")
	assert.Empty(t, got.stderr,
		"config must not write to stderr on success "+
			"(stream boundary, WBS 7.4.1)")
}

// TestConfigCommand_NoArgsIsEnforcedThroughDispatch asserts AC4 at the
// dispatch level: `forge config extra` is rejected before RunE runs.
//
// The metadata test (TestConfigCommand_NoArgs) checks the Args
// validator directly. This test checks that the dispatcher honors the
// validator — i.e. that a positional argument never reaches RunE.
//
// # How the test knows RunE did not run
//
// RunE writes the frozen output header to stdout. If a positional
// argument were passed through to RunE, the header would appear on
// stdout. The test asserts stdout is empty, which means the
// dispatcher rejected the invocation before invoking the handler.
func TestConfigCommand_NoArgsIsEnforcedThroughDispatch(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"config", "extra"}, nil)

	require.NotEqual(t, ExitSuccess, got.exitCode,
		"`forge config extra` must fail (AC4)")
	assert.Empty(t, got.stdout,
		"RunE must not run when a positional argument is supplied")
	assertOnlyStderr(t, got)
}

// =============================================================================
// WBS 9.3.1 — Frozen --path flag metadata.
//
// These tests assert the flag's presence, type, default, and scope on
// the command. The flag's behavioral semantics (what --path prints,
// in text and JSON, with and without a resolved file) are exercised
// in config_path_test.go (unit tests for the writer functions) and
// config_golden_test.go (end-to-end tests through runCLI). This
// section asserts only the flag's shape as metadata.
// =============================================================================

// TestConfigCommand_HasPathFlag asserts that the config command
// declares a --path flag of type bool, defaulting to false.
//
// The flag is command-local: it is not persistent, and no other
// command declares it. The test pins the flag's presence and its
// shape.
//
// # What the test does not assert
//
// The test does not assert the flag's usage string. The usage string
// is user-visible text; it is pinned by the help golden file
// (testdata/help/config.golden.txt), which is regenerated via the
// standard process when the usage changes. A separate assertion here
// would duplicate the golden file's job.
func TestConfigCommand_HasPathFlag(t *testing.T) {
	t.Parallel()

	cmd := newConfigCmd(testDependencies())
	flag := cmd.Flags().Lookup(configFlagPath)

	require.NotNil(t, flag,
		"config command must declare --%s (WBS 9.3.1)", configFlagPath)
	assert.Equal(t, "bool", flag.Value.Type(),
		"--%s must be a bool flag", configFlagPath)
	assert.Equal(t, "false", flag.DefValue,
		"--%s must default to false", configFlagPath)
	assert.False(t, flag.Changed,
		"--%s must not be marked as set at construction time",
		configFlagPath)
}

// TestConfigCommand_PathFlagIsLocal asserts that --path is declared
// on the command itself, not inherited from the root.
//
// # Why the test exists
//
// A future contributor who declared --path on the root command (for
// example, because they wanted it available everywhere) would be
// changing the CLI's surface: every command would accept --path.
// The test pins the flag's scope: it belongs to the config command.
//
// # What the test checks
//
//   - The flag is present in cmd.Flags() (the local flag set).
//   - The flag is not present in cmd.PersistentFlags() (the
//     inherited flag set).
//
// Both must hold. A flag that is only persistent is inherited, not
// local; a flag that is both is shadowed, which produces two entries
// in help output. The correct configuration is exactly one: local.
func TestConfigCommand_PathFlagIsLocal(t *testing.T) {
	t.Parallel()

	cmd := newConfigCmd(testDependencies())

	// Local flags: declared on the command itself.
	assert.NotNil(t, cmd.Flags().Lookup(configFlagPath),
		"--%s must be declared locally", configFlagPath)

	// Persistent flags: would be inherited by children. The config
	// command has no children, but the persistent flag set is a
	// distinct scope; a flag declared as persistent would appear
	// here instead of in Flags().
	assert.Nil(t, cmd.PersistentFlags().Lookup(configFlagPath),
		"--%s must not be declared as persistent", configFlagPath)
}

// =============================================================================
// Rendered help (BDD) — exercised through runCLI so the help system
// sees the same command tree production sees.
// =============================================================================

// TestConfigCommand_HelpContainsFrozenMetadata verifies that
// `forge config --help` advertises every frozen field that Cobra
// actually renders in a command's own help output.
//
// # What Cobra renders where
//
// Cobra's `UsageString()` for a leaf command contains:
//
//   - the Long description (as the header),
//   - the Usage line,
//   - the Examples block,
//   - the command-specific Flags block,
//   - the Global Flags block.
//
// It does NOT contain Short: Cobra renders Short in the *parent's*
// "Available Commands:" section, not in the command's own help.
// Asserting Short here would be asserting on the wrong surface.
// Short is asserted separately in
// TestConfigCommand_ShortVisibleInRootHelp.
//
// # What this test asserts about --path
//
// The test asserts that the string "--path" appears in the help
// output. Since WBS 9.3.1, "--path" appears in two places:
//
//   - the Long description ("Use --path to print only the resolved
//     config file location."),
//   - the command's Flags block (the flag's own declaration).
//
// The test passes if either is present; it does not distinguish
// between them. The stronger assertion — that the flag is declared,
// not merely mentioned — is pinned by TestConfigCommand_HasPathFlag
// on the command's flag set, and by the help golden file
// (testdata/help/config.golden.txt), which locks the exact rendered
// help.
//
// # Why the alias assertions are removed
//
// The previous version of this test asserted that the help output
// does not contain "cfg" or "conf". Those assertions were false
// positives: the Global Flags block contains
// "--config string   path to the configuration file ...", which
// contains the substring "conf". The authoritative check for
// "no aliases" is on cmd.Aliases (see TestConfigCommand_Identity).
// If a help-level check is desired, it must inspect the command
// *names* in the "Available Commands:" section, not raw substrings.
// That check lives in TestConfigCommand_NoAliasesInRootHelp.
func TestConfigCommand_HelpContainsFrozenMetadata(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"config", "--help"}, nil)
	require.Equal(t, ExitSuccess, got.exitCode,
		"--help must exit successfully; stderr: %q", got.stderr)

	help := got.stdout

	// Positive: every frozen field that Cobra renders in a leaf
	// command's own help must be present.
	//
	// We assert on:
	//   - the command name (Usage line),
	//   - the Long description's opening sentence,
	//   - the Long description's body phrases,
	//   - the --path mention (documented in Long),
	//   - each of the three Example lines.
	for _, want := range []string{
		"config",
		"Display the resolved CLI configuration",
		"precedence layers",
		"--path",
		"forge config",
		"forge config --path",
		"forge config --format json",
	} {
		assert.Contains(t, help, want,
			"`forge config --help` must advertise %q (WBS 9.1.1)", want)
	}
}

// TestConfigCommand_HelpListsPathFlag asserts that the --path flag
// appears in the command's own help output under the "Flags:"
// section.
//
// # Why this is a separate test from HasPathFlag
//
// TestConfigCommand_HasPathFlag checks the flag's presence on the
// command's flag set (a metadata assertion). This test checks that
// the flag is *rendered* in the help output — i.e. that Cobra's
// help-rendering pipeline includes it. The two are different
// surfaces: the flag can be declared but not rendered (if, say, the
// flag's `Hidden` attribute were set), and the help can mention
// "--path" in its Long description without the flag being declared.
//
// The test asserts on the flag's help line, which Cobra renders as
// "      --path   <usage>" (with the usage string following). The
// assertion looks for the flag's name followed by whitespace, which
// distinguishes the flag's declaration from a mention in prose.
//
// # Why this is not a golden test
//
// The golden file for `forge config --help` (config.golden.txt) locks
// the entire help output byte-for-byte. This test complements the
// golden file by asserting the property ("the flag's declaration is
// present") in a form that survives a regeneration of the golden
// file. If the golden file is regenerated after an intentional help
// change, this test still verifies that the flag's declaration is
// present in the new golden.
func TestConfigCommand_HelpListsPathFlag(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"config", "--help"}, nil)
	require.Equal(t, ExitSuccess, got.exitCode,
		"--help must exit successfully; stderr: %q", got.stderr)

	help := got.stdout

	// The flag's declaration line in Cobra's help output begins with
	// whitespace, then "--path", then whitespace, then the usage
	// string. A substring match on "--path " (with a trailing space)
	// is a reliable proxy: the Long description mentions "--path to
	// print", which also contains the substring, so the test would
	// pass on the Long mention alone. To distinguish, we assert that
	// the substring appears on a line that also contains the flag's
	// usage text.
	lines := strings.Split(help, "\n")
	foundFlagDeclaration := false
	for _, line := range lines {
		// The declaration line contains "--path" followed by
		// whitespace followed by usage text. The Long
		// description's mention is "--path to print only the
		// resolved config file location." — which contains
		// "--path " but is preceded by "Use " and followed by
		// a lowercase phrase, not by the flag column's
		// formatting.
		if !strings.Contains(line, "--path") {
			continue
		}
		// The declaration line is indented and the flag name is
		// followed by two or more spaces (Cobra's flag column
		// alignment). The prose mention is part of a sentence.
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--path") &&
			strings.Contains(trimmed, "  ") {
			foundFlagDeclaration = true
			break
		}
	}

	assert.True(t, foundFlagDeclaration,
		"`forge config --help` must declare --path in its Flags block\n"+
			"help output:\n%s", help)
}

// TestConfigCommand_ShortVisibleInRootHelp asserts that the Short
// description is rendered in `forge --help`, under "Available
// Commands:".
//
// # Why this test exists separately
//
// Cobra renders a command's Short in the *parent's* help, not in
// the command's own help. The command's own help uses Long. Testing
// Short against `forge config --help` would therefore fail (the
// string is not present there), even though Short is correctly set.
//
// This test asserts Short on the correct surface, so that:
//
//   - Short's length/mood contract (WBS 7.2.1) is verified as it is
//     actually displayed to users,
//   - the frozen metadata is observable end-to-end.
func TestConfigCommand_ShortVisibleInRootHelp(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"--help"}, nil)
	require.Equal(t, ExitSuccess, got.exitCode,
		"`forge --help` must exit successfully; stderr: %q", got.stderr)

	assert.Contains(t, got.stdout, "Display resolved CLI configuration",
		"Short must be rendered in `forge --help` (Cobra renders Short "+
			"on the parent's command list, not on the command's own help)")
}

// TestConfigCommand_NoAliasesInRootHelp asserts that the forbidden
// aliases `cfg` and `conf` do not appear as command names in
// `forge --help`.
//
// # Why this replaces substring assertions
//
// Asserting `!strings.Contains(help, "conf")` is unsound: the Global
// Flags block contains `--config string   path to the configuration
// file ...`, which contains the substring "conf". That is a false
// positive, not a violation of WBS 9.1.1.
//
// The correct check is on the *command-name surface*: the
// "Available Commands:" section. The shared helpCommandNames helper
// parses exactly that section, so this test asserts the property on
// the surface where it is meaningful.
//
// # Redundancy note
//
// TestConfigCommand_Identity already asserts cmd.Aliases is empty,
// which is the authoritative check. This test is a defensive
// end-to-end assertion: even if a future refactor accidentally
// attached an alias via AddCommand rather than via the Aliases
// field, this test would catch it in the rendered help.
func TestConfigCommand_NoAliasesInRootHelp(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"--help"}, nil)
	require.Equal(t, ExitSuccess, got.exitCode,
		"`forge --help` must exit successfully; stderr: %q", got.stderr)

	names := helpCommandNames(got.stdout)

	assert.NotContains(t, names, "cfg",
		"`cfg` is a forbidden alias per WBS 9.1.1 (AC6)")
	assert.NotContains(t, names, "conf",
		"`conf` is a forbidden alias per WBS 9.1.1 (AC6)")
}

// TestConfigCommand_HelpVisibleInRoot asserts that `forge --help` now
// lists config, i.e. Hidden:false is effective.
//
// The test parses the "Available Commands:" section via the shared
// helpCommandNames helper rather than substring-matching the whole
// output, so that a "config" appearing elsewhere (e.g. in the
// description of another command) does not produce a false positive.
func TestConfigCommand_HelpVisibleInRoot(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"--help"}, nil)
	require.Equal(t, ExitSuccess, got.exitCode,
		"--help must exit successfully; stderr: %q", got.stderr)

	names := helpCommandNames(got.stdout)
	assert.Contains(t, names, "config",
		"config must appear in `forge --help` after WBS 9.1.1")

	// Ordering: within the user-facing group, commands are
	// alphabetical. config precedes version.
	ci, vi := -1, -1
	for i, n := range names {
		switch n {
		case "config":
			ci = i
		case "version":
			vi = i
		}
	}
	require.NotEqual(t, -1, ci, "config must be listed in root help")
	require.NotEqual(t, -1, vi, "version must be listed in root help")
	assert.Less(t, ci, vi,
		"config must precede version in help (alphabetical within group)")
}
