// Package cli contains white-box tests for the CLI package.
//
// This file tests the version command: its metadata, its delegation
// to the version application service, its error path, the
// --format and --verbose flags, and the structural invariants that
// keep the handler thin.
//
// The test file is declared in package cli, not package cli_test,
// because it needs to reach unexported symbols: newVersionCmd,
// Dependencies, and the source-file helpers used by the structural
// tests.
//
// # Relationship to internal/app/version
//
// This file tests the handler. The application service that the
// handler calls is tested in internal/app/version/service_test.go,
// internal/app/version/format_test.go,
// internal/app/version/format_json_test.go, and
// internal/app/version/buildinfo_test.go. The two layers are tested
// separately on purpose: the handler's job is to delegate, and the
// service's job is to produce and format data. A failure in one
// layer should not require reading the other layer's tests.
//
// # The handler / service pattern
//
// WBS 4.3.1 defines a boundary between Cobra handlers (this file's
// subject) and application services (internal/app/version). The
// handler:
//
//   - Parses flags.
//   - Collects arguments (none allowed).
//   - Calls the application service.
//   - Writes the result to deps.Streams.Out.
//   - Returns an error; never calls os.Exit.
//
// WBS 17.4.1 adds the --verbose output mode and two additional
// service entry points (WriteVerbose and WriteFullJSON). The
// handler's shape is unchanged: it still parses flags, dispatches
// once to the service, and returns.
//
// # Test organisation
//
// The behavioural tests go through the shared runCLI helper in
// testhelper_test.go rather than constructing a Cobra command
// directly. The helper exercises the command through the same
// boundary production code uses (runWithOptions), which is the
// property WBS 4.3.2 exists to establish. The structural tests at
// the bottom of the file read version.go as data and are
// indifferent to how the command is invoked.
//
// # Why the --verbose tests do not assert an empty stderr
//
// Under --verbose, the handler emits two DEBUG records to stderr
// (WBS 12.5.1). Their presence is correct and is asserted by the
// handler's own tests only insofar as the handler does not write
// to os.Stderr directly; the records go through the logger. The
// command-specific tests in this file assert the stdout product
// output and do not assert anything about stderr on success. The
// stream boundary — no DEBUG record on stdout, no product
// rendering on stderr — is asserted by
// TestStreamBoundary_VerboseDoesNotPolluteStdout in
// stream_boundary_test.go.
//
// # Why the structural tests strip comments
//
// The structural tests read version.go as data and assert that the
// handler body does not contain forbidden tokens such as os.Stdout
// or os.Exit. The docstring of the handler mentions those tokens to
// explain what the handler does *not* do. A naive strings.Contains
// over the whole file would match the docstring and produce a false
// positive.
//
// The stripComments helper removes both Go comment forms before the
// token scan. The scan then sees only code. The helper is
// deliberately simple: it does not parse the file with go/parser,
// and it does not attempt to distinguish strings from identifiers.
// It is sufficient for the file it is applied to, and the
// alternative (a full parser) would be more machinery than the
// tests need.
package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	appversion "github.com/thapelomagqazana/forge/internal/app/version"
	"github.com/thapelomagqazana/forge/internal/streams"
)

// =============================================================================
// Compile-time assertions
// =============================================================================

// Compile-time interface check: newVersionCmd must accept a
// Dependencies value and return a *cobra.Command.
//
// This assertion fails at compile time if the signature drifts. It is
// the cheapest possible regression test for the constructor's shape.
var _ func(Dependencies) *cobra.Command = newVersionCmd

// =============================================================================
// Positive tests — behaviour through the CLI boundary
// =============================================================================
//
// These tests exercise the version command the same way a user does:
// by invoking the CLI with arguments and inspecting the observable
// result. The shared runCLI helper (in testhelper_test.go) performs
// the invocation through runWithOptions, so the test is a
// complete end-to-end exercise of the command tree, the Dependencies
// transformation, and the handler.

// TestVersionCommand_PrintsVersion is the reference positive test for
// the version command. It invokes `forge version` through the CLI
// boundary and asserts on the observable result.
//
// The assertions are deliberately broad: the test checks for the
// presence of the header prefix and the detail keys, not for specific
// values. The specific values are injected at link time and vary
// between builds; the format is what the command guarantees.
func TestVersionCommand_PrintsVersion(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"version"}, nil)

	if got.exitCode != ExitSuccess {
		t.Fatalf("exit code: got %d, want %d",
			got.exitCode, ExitSuccess)
	}
	// The header must be "forge" either alone (uninstrumented) or
	// followed by " <version>". A prefix check for "forge" is the
	// loosest correct assertion; the stricter format is pinned by
	// TestFormat_FullOutput in internal/app/version/format_test.go.
	if !strings.HasPrefix(got.stdout, "forge") {
		t.Errorf("stdout does not start with the version header: %q",
			got.stdout)
	}
	for _, key := range []string{
		"commit:", "built:", "dirty:", "go version:", "platform:",
	} {
		if !strings.Contains(got.stdout, key) {
			t.Errorf("stdout missing %q: %q", key, got.stdout)
		}
	}
	if got.stderr != "" {
		t.Errorf("stderr should be empty on success: %q", got.stderr)
	}
}

// TestVersionCommand_WithHelpFlag verifies that `forge version --help`
// prints the command's help to stdout and exits with success.
func TestVersionCommand_WithHelpFlag(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"version", "--help"}, nil)

	if got.exitCode != ExitSuccess {
		t.Fatalf("exit code: got %d, want %d",
			got.exitCode, ExitSuccess)
	}
	if !strings.Contains(got.stdout, "version") {
		t.Errorf("help text does not mention 'version': %q", got.stdout)
	}
	if got.stderr != "" {
		t.Errorf("stderr should be empty on success: %q", got.stderr)
	}
}

// =============================================================================
// Negative tests — behaviour through the CLI boundary
// =============================================================================

// TestVersionCommand_RejectsExtraArguments verifies that
// `forge version extra` fails with a non-success exit code and writes
// a diagnostic to stderr.
func TestVersionCommand_RejectsExtraArguments(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"version", "extra"}, nil)

	if got.exitCode == ExitSuccess {
		t.Errorf("exit code: got %d, want non-success", got.exitCode)
	}
	if got.stdout != "" {
		t.Errorf("stdout should be empty on failure: %q", got.stdout)
	}
	if got.stderr == "" {
		t.Error("stderr should contain a diagnostic on failure")
	}
}

// TestVersionCommand_GivenNilEnv_StillSucceeds verifies that a nil env
// map does not prevent the version command from running.
func TestVersionCommand_GivenNilEnv_StillSucceeds(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"version"}, nil)

	if got.exitCode != ExitSuccess {
		t.Fatalf("exit code: got %d, want %d",
			got.exitCode, ExitSuccess)
	}
	if got.stdout == "" {
		t.Error("stdout is empty; want version output")
	}
}

// TestVersionCommand_GivenEnvMap_StillSucceeds verifies that a
// non-nil env map does not prevent the version command from running.
func TestVersionCommand_GivenEnvMap_StillSucceeds(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"FORGE_TEST_VAR": "injected",
	}
	got := runCLI(t, []string{"version"}, env)

	if got.exitCode != ExitSuccess {
		t.Fatalf("exit code: got %d, want %d",
			got.exitCode, ExitSuccess)
	}
	if got.stdout == "" {
		t.Error("stdout is empty; want version output")
	}
}

// =============================================================================
// Stream separation
// =============================================================================

// TestVersionCommand_WritesNothingToStderrOnSuccess verifies the
// stdout/stderr separation contract for the version command in the
// default (non-verbose) mode.
//
// The test runs `forge version` without --verbose, so the log level
// is the default (info) and the DEBUG records are filtered out.
// stderr is empty.
//
// The verbose path emits DEBUG records on stderr; its stream
// boundary is asserted separately by
// TestStreamBoundary_VerboseDoesNotPolluteStdout.
func TestVersionCommand_WritesNothingToStderrOnSuccess(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"version"}, nil)

	if got.exitCode != ExitSuccess {
		t.Fatalf("exit code: got %d, want %d",
			got.exitCode, ExitSuccess)
	}
	if got.stderr != "" {
		t.Errorf("stderr should be empty on success: %q", got.stderr)
	}
}

// =============================================================================
// Delegation test — handler output matches service output
// =============================================================================

// TestVersionCommand_DelegatesToService verifies that the handler's
// output for the default mode matches what the service produces
// directly.
//
// The test invokes the command through runCLI and captures its
// output, then calls the service's formatter on a fresh buffer and
// compares the two. They must be byte-identical because the handler's
// only job is to delegate.
func TestVersionCommand_DelegatesToService(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"version"}, nil)

	if got.exitCode != ExitSuccess {
		t.Fatalf("exit code: got %d, want %d",
			got.exitCode, ExitSuccess)
	}

	// Produce the same output by calling the service directly. The
	// version command's handler calls
	// appversion.FormatAs(deps.Streams.Out, appversion.Get(), format);
	// this test does the same on a fresh buffer with the default
	// format.
	var svcOut bytes.Buffer
	if err := appversion.FormatAs(&svcOut, appversion.Get(), appversion.FormatText); err != nil {
		t.Fatalf("appversion.FormatAs: %v", err)
	}

	if got.stdout != svcOut.String() {
		t.Errorf("handler output differs from service output\n"+
			"handler: %q\n"+
			"service: %q",
			got.stdout, svcOut.String())
	}
}

// TestVersionCommand_VerboseDelegatesToWriteVerbose verifies that
// `forge version --verbose` produces exactly the output of the
// service's WriteVerbose function.
//
// # What the test does not assert
//
// The test does not assert that stderr is empty. Under --verbose,
// the handler emits two DEBUG records to stderr (WBS 12.5.1); their
// presence is correct. The comparison is on stdout only. The stream
// boundary is asserted separately by
// TestStreamBoundary_VerboseDoesNotPolluteStdout.
func TestVersionCommand_VerboseDelegatesToWriteVerbose(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"version", "--verbose"}, nil)

	if got.exitCode != ExitSuccess {
		t.Fatalf("exit code: got %d, want %d\nstderr: %s",
			got.exitCode, ExitSuccess, got.stderr)
	}

	var svcOut bytes.Buffer
	if err := appversion.WriteVerbose(&svcOut, appversion.Get()); err != nil {
		t.Fatalf("appversion.WriteVerbose: %v", err)
	}

	if got.stdout != svcOut.String() {
		t.Errorf("handler output differs from WriteVerbose output\n"+
			"handler: %q\n"+
			"service: %q",
			got.stdout, svcOut.String())
	}
}

// =============================================================================
// --format flag
// =============================================================================

// TestVersionCommand_FormatJSON verifies that
// `forge version --format json` produces valid JSON with the five
// schema fields.
func TestVersionCommand_FormatJSON(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"version", "--format", "json"}, nil)

	if got.exitCode != ExitSuccess {
		t.Fatalf("exit code: got %d, want %d\nstderr: %s",
			got.exitCode, ExitSuccess, got.stderr)
	}
	if got.stderr != "" {
		t.Errorf("stderr should be empty on success: %q", got.stderr)
	}

	// The output must be valid JSON.
	var m map[string]any
	if err := json.Unmarshal([]byte(got.stdout), &m); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, got.stdout)
	}

	for _, key := range []string{"version", "commit", "buildDate", "dirty", "buildType"} {
		if _, ok := m[key]; !ok {
			t.Errorf("field %q missing from output: %s", key, got.stdout)
		}
	}
}

// TestVersionCommand_FormatJSONEqualsSign verifies that
// `--format=json` and `--format json` are equivalent.
func TestVersionCommand_FormatJSONEqualsSign(t *testing.T) {
	t.Parallel()

	spaced := runCLI(t, []string{"version", "--format", "json"}, nil)
	equals := runCLI(t, []string{"version", "--format=json"}, nil)

	if spaced.stdout != equals.stdout {
		t.Errorf("--format json and --format=json differ:\n"+
			"spaced: %q\nequals: %q", spaced.stdout, equals.stdout)
	}
	if spaced.exitCode != equals.exitCode {
		t.Errorf("exit codes differ: spaced=%d equals=%d",
			spaced.exitCode, equals.exitCode)
	}
}

// TestVersionCommand_UnknownFormat verifies that an unknown
// --format value exits with ExitUsage and writes a diagnostic to
// stderr that names the offending value.
func TestVersionCommand_UnknownFormat(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"version", "--format", "yaml"}, nil)

	if got.exitCode != ExitUsage {
		t.Errorf("exit code: got %d, want %d (ExitUsage)",
			got.exitCode, ExitUsage)
	}
	if got.stdout != "" {
		t.Errorf("stdout should be empty on failure: %q", got.stdout)
	}
	if got.stderr == "" {
		t.Error("stderr should contain a diagnostic")
	}
	if !strings.Contains(got.stderr, "yaml") {
		t.Errorf("stderr does not mention the offending value: %q", got.stderr)
	}
}

// TestVersionCommand_DefaultFormatIsText verifies that
// `forge version` without --format produces the text format from
// WBS 6.4.1, not the JSON format.
func TestVersionCommand_DefaultFormatIsText(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"version"}, nil)

	if got.exitCode != ExitSuccess {
		t.Fatalf("exit code: got %d, want %d",
			got.exitCode, ExitSuccess)
	}

	// The text format begins with "forge" and contains colons;
	// the JSON format begins with "{" and contains quoted keys.
	if !strings.HasPrefix(got.stdout, "forge") {
		t.Errorf("default output does not start with 'forge': %q", got.stdout)
	}
	if strings.HasPrefix(got.stdout, "{") {
		t.Errorf("default output is JSON, want text: %q", got.stdout)
	}
}

// =============================================================================
// --verbose flag
// =============================================================================
//
// The tests below exercise the version command's --verbose output.
// The flag is the root's persistent flag; it selects the verbose
// output format and sets the log level to debug. The DEBUG records
// the flag enables are emitted to stderr; the tests below assert
// the stdout product output only.
//
// The stream boundary — no DEBUG record on stdout, no product
// rendering on stderr — is asserted separately by
// TestStreamBoundary_VerboseDoesNotPolluteStdout in
// stream_boundary_test.go.

// TestVersionCommand_Verbose verifies that `forge version --verbose`
// emits the extended human format with the provenance sections.
//
// # What the test does not assert
//
// The test does not assert that stderr is empty. Under --verbose,
// the handler emits two DEBUG records to stderr (WBS 12.5.1); their
// presence is correct. The test asserts the verbose output on
// stdout only.
func TestVersionCommand_Verbose(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"version", "--verbose"}, nil)

	if got.exitCode != ExitSuccess {
		t.Fatalf("exit code: got %d, want %d\nstderr: %s",
			got.exitCode, ExitSuccess, got.stderr)
	}

	// The verbose format contains the five top-level fields and the
	// provenance sections. The dependency and settings sections are
	// optional (they appear only when the lists are non-empty); the
	// go and module lines are always present.
	for _, want := range []string{
		"version:", "commit:", "buildDate:", "dirty:", "buildType:",
		"go:", "module:",
	} {
		if !strings.Contains(got.stdout, want) {
			t.Errorf("verbose output missing %q:\n%s", want, got.stdout)
		}
	}
}

// TestVersionCommand_VerboseJSON verifies that
// `forge version --format json --verbose` emits valid JSON with a
// provenance field.
//
// # What the test does not assert
//
// The test does not assert that stderr is empty. Under --verbose,
// the handler emits two DEBUG records to stderr (WBS 12.5.1); their
// presence is correct. The test asserts the JSON product output on
// stdout only.
func TestVersionCommand_VerboseJSON(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"version", "--format", "json", "--verbose"}, nil)

	if got.exitCode != ExitSuccess {
		t.Fatalf("exit code: got %d, want %d\nstderr: %s",
			got.exitCode, ExitSuccess, got.stderr)
	}

	var m map[string]any
	if err := json.Unmarshal([]byte(got.stdout), &m); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, got.stdout)
	}

	// The five linker fields are at the top level.
	for _, key := range []string{"version", "commit", "buildDate", "dirty", "buildType"} {
		if _, ok := m[key]; !ok {
			t.Errorf("field %q missing from output: %s", key, got.stdout)
		}
	}

	// The provenance field is present.
	prov, ok := m["provenance"]
	if !ok {
		t.Fatalf("provenance field missing from output: %s", got.stdout)
	}
	provMap, ok := prov.(map[string]any)
	if !ok {
		t.Fatalf("provenance is not an object: %T", prov)
	}
	for _, key := range []string{"goVersion", "module", "vcsModified"} {
		if _, ok := provMap[key]; !ok {
			t.Errorf("provenance field %q missing: %s", key, got.stdout)
		}
	}
}

// TestVersionCommand_VerboseTextIsNotJSON asserts that
// `--verbose` alone (without `--format json`) produces the text
// format, not JSON. The two flags are independent.
func TestVersionCommand_VerboseTextIsNotJSON(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"version", "--verbose"}, nil)

	if got.exitCode != ExitSuccess {
		t.Fatalf("exit code: got %d, want %d", got.exitCode, ExitSuccess)
	}
	if strings.HasPrefix(got.stdout, "{") {
		t.Errorf("--verbose alone produced JSON; want text:\n%s", got.stdout)
	}
	if !strings.Contains(got.stdout, "version:") {
		t.Errorf("--verbose output does not contain the verbose label:\n%s", got.stdout)
	}
}

// TestVersionCommand_VerboseJSONOrder verifies the flag order does
// not matter: `--verbose --format json` and
// `--format json --verbose` produce identical output.
func TestVersionCommand_VerboseJSONOrder(t *testing.T) {
	t.Parallel()

	a := runCLI(t, []string{"version", "--verbose", "--format", "json"}, nil)
	b := runCLI(t, []string{"version", "--format", "json", "--verbose"}, nil)

	if a.stdout != b.stdout {
		t.Errorf("flag order changed output:\n"+
			"--verbose --format json: %q\n"+
			"--format json --verbose: %q", a.stdout, b.stdout)
	}
	if a.exitCode != b.exitCode {
		t.Errorf("exit codes differ: %d vs %d", a.exitCode, b.exitCode)
	}
}

// TestVersionCommand_VerboseHelpDoesNotFail verifies that
// `forge version --verbose --help` prints help rather than version
// output. This is the standard Cobra behavior (help wins over
// execution), and the test pins it against a future change that
// might make --verbose interact with --help.
func TestVersionCommand_VerboseHelpDoesNotFail(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"version", "--verbose", "--help"}, nil)

	if got.exitCode != ExitSuccess {
		t.Fatalf("exit code: got %d, want %d", got.exitCode, ExitSuccess)
	}
	if !strings.Contains(got.stdout, "Usage:") {
		t.Errorf("help output does not contain 'Usage:':\n%s", got.stdout)
	}
}

// =============================================================================
// Structural tests — the handler / service boundary (WBS 4.3.1)
// =============================================================================
//
// The tests below read version.go as data, strip comments, and
// assert on the shape of the remaining code. They are the mechanical
// enforcement of AC3, AC4, and AC5 of WBS 4.3.1.
//
// The comment stripping is essential: the handler's docstring
// explains what the handler does *not* do, and it names the
// forbidden tokens (os.Stdout, os.Exit, fmt.Fprintln) in the process.
// Without stripping, every structural test would fail on the
// docstring rather than on the code.

// TestVersionHandler_IsThin verifies the structural invariants that
// keep the handler thin.
//
// The test reads version.go from disk, strips comments, and asserts:
//
//   - The handler references the service's documented API at most
//     as many times as the four output modes require.
//   - The code contains no direct writes to process-global streams.
//   - The code contains no fmt.Fprintln, fmt.Fprintf, fmt.Println,
//     or fmt.Printf calls.
//   - The code contains no os.Exit call.
//
// If any assertion fails, the failure message cites the specific
// token and the WBS acceptance criterion it violates.
//
// # Why the "at most" assertions
//
// Before WBS 17.4.1, the handler called exactly one service
// function (FormatAs) because there was exactly one output shape.
// WBS 17.4.1 adds three more shapes (verbose text, verbose JSON,
// and the dispatch for them), so the handler now calls up to four
// service functions. The bound is written as "at most four" so the
// test still catches an accidental fifth call while allowing the
// four legitimate dispatches.
//
// The exact set of service functions the handler may call is
// pinned by TestVersionHandler_OnlyCallsDocumentedService below.
func TestVersionHandler_IsThin(t *testing.T) {
	t.Parallel()

	content := readFile(t, "version.go")
	code := stripComments(content)

	// At most four calls to the service's dispatchers, one per
	// output mode. The exact set is pinned by the allowlist test.
	dispatchCalls :=
		strings.Count(code, "appversion.FormatAs(") +
			strings.Count(code, "appversion.WriteVerbose(") +
			strings.Count(code, "appversion.WriteFullJSON(")
	if dispatchCalls > 4 {
		t.Errorf("handler contains %d service dispatch calls; want at most 4",
			dispatchCalls)
	}

	// The handler must not call the JSON writer for the simple
	// (non-verbose) JSON case; that path goes through FormatAs.
	// A direct call to WriteJSON would duplicate the dispatcher's
	// job.
	if got := strings.Count(code, "appversion.WriteJSON("); got != 0 {
		t.Errorf("handler contains %d calls to appversion.WriteJSON; want 0 "+
			"(the handler must go through appversion.FormatAs for simple JSON)", got)
	}

	// Forbidden tokens. Each is a violation of a specific rule.
	forbidden := map[string]string{
		"os.Stdout":    "AC4 — write to process-global stream",
		"os.Stderr":    "AC4 — write to process-global stream",
		"os.Stdin":     "AC4 — read from process-global stream",
		"os.Exit":      "AC5 — process exit must be handled by runWithOptions",
		"fmt.Fprintln": "AC4 — format output directly",
		"fmt.Fprintf":  "AC4 — format output directly",
		"fmt.Println":  "AC4 — format output directly",
		"fmt.Printf":   "AC4 — format output directly",
	}
	for token, reason := range forbidden {
		if strings.Contains(code, token) {
			t.Errorf("version.go references %q (%s)", token, reason)
		}
	}
}

// TestVersionHandler_DoesNotImportInfrastructure verifies that the
// handler file does not import infrastructure or domain packages
// directly.
func TestVersionHandler_DoesNotImportInfrastructure(t *testing.T) {
	t.Parallel()

	content := readFile(t, "version.go")

	forbiddenImports := []string{
		"internal/filesystem",
		"internal/config",
	}
	for _, imp := range forbiddenImports {
		if strings.Contains(content, imp) {
			t.Errorf("version.go imports %q; "+
				"handlers must not import infrastructure or "+
				"domain packages (AC2)", imp)
		}
	}
}

// TestVersionHandler_ReturnsErrorNotExit verifies that the handler
// returns an error rather than terminating the process.
func TestVersionHandler_ReturnsErrorNotExit(t *testing.T) {
	t.Parallel()

	content := readFile(t, "version.go")
	code := stripComments(content)

	if strings.Contains(code, "os.Exit") {
		t.Error("version.go calls os.Exit; " +
			"handlers must return an error (AC5)")
	}
}

// TestVersionHandler_OnlyCallsDocumentedService verifies that the
// handler references only the service's documented API.
//
// # The allowlist
//
// The allowed tokens are the handler's legitimate touch points on
// the service:
//
//   - appversion.Get              — the read accessor.
//   - appversion.FormatAs         — the text/JSON dispatcher.
//   - appversion.WriteVerbose     — the verbose human formatter
//     (WBS 17.4.1).
//   - appversion.WriteFullJSON    — the verbose JSON formatter
//     (WBS 17.4.1).
//   - appversion.ErrUnknownFormat — the sentinel error (kept for
//     symmetry; the current handler does not reference it because
//     the format pre-check uses the constants below).
//   - appversion.FormatText       — the text format constant.
//   - appversion.FormatJSON       — the JSON format constant.
//
// Any other reference to the service is a boundary violation.
func TestVersionHandler_OnlyCallsDocumentedService(t *testing.T) {
	t.Parallel()

	content := readFile(t, "version.go")
	code := stripComments(content)

	allowed := map[string]bool{
		"appversion.Get(":             true,
		"appversion.FormatAs(":        true,
		"appversion.WriteVerbose(":    true,
		"appversion.WriteFullJSON(":   true,
		"appversion.ErrUnknownFormat": true,
		"appversion.FormatText":       true,
		"appversion.FormatJSON":       true,
	}

	const prefix = "appversion."
	rest := code
	for {
		idx := strings.Index(rest, prefix)
		if idx < 0 {
			break
		}
		rest = rest[idx:]
		end := len(prefix)
		for end < len(rest) && isIdentChar(rest[end]) {
			end++
		}
		if end < len(rest) && rest[end] == '(' {
			end++
		}
		token := rest[:end]
		if !allowed[token] {
			t.Errorf("version.go references %q; "+
				"handler must call only the service's documented API "+
				"(appversion.Get, appversion.FormatAs, "+
				"appversion.WriteVerbose, appversion.WriteFullJSON, "+
				"appversion.ErrUnknownFormat, appversion.FormatText, "+
				"appversion.FormatJSON)",
				token)
		}
		rest = rest[end:]
	}
}

// isIdentChar reports whether b is a valid Go identifier character
// (letter, digit, or underscore). It is used by
// TestVersionHandler_OnlyCallsDocumentedService.
func isIdentChar(b byte) bool {
	return b == '_' ||
		(b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9')
}

// =============================================================================
// Edge and corner cases
// =============================================================================
//
// The tests below exercise edge cases that are not reachable through
// the shared runCLI helper, either because they require a specific
// Dependencies value (nil Out) or because they inspect the
// handler's behaviour in isolation.

// TestVersionHandler_NilStdoutPanics verifies the handler's behaviour
// when the injected output stream is nil.
//
// The handler does not guard against nil writers: a nil io.Writer
// panics on first use, and the panic is the correct symptom of a
// caller that failed to construct Dependencies correctly. The test
// pins this behaviour so a future change does not silently add a
// guard that would hide the bug.
func TestVersionHandler_NilStdoutPanics(t *testing.T) {
	t.Parallel()

	deps := Dependencies{
		Streams: streams.Streams{
			Out: nil,
			Err: &bytes.Buffer{},
		},
	}
	cmd := newVersionCmd(deps)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	defer func() {
		if r := recover(); r == nil {
			t.Error("Execute with nil Streams.Out did not panic; " +
				"want panic (documented behaviour)")
		}
	}()
	_ = cmd.Execute()
}

// TestVersionHandler_EmptyBuildInfo verifies that the command
// produces valid output even when the build variables are empty.
func TestVersionHandler_EmptyBuildInfo(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"version"}, nil)

	if got.exitCode != ExitSuccess {
		t.Fatalf("exit code: got %d, want %d",
			got.exitCode, ExitSuccess)
	}
	if !strings.HasPrefix(got.stdout, "forge") {
		t.Errorf("stdout does not start with version header: %q",
			got.stdout)
	}
	for _, key := range []string{
		"commit:", "built:", "dirty:", "go version:", "platform:",
	} {
		if !strings.Contains(got.stdout, key) {
			t.Errorf("stdout missing %q: %q", key, got.stdout)
		}
	}
}

// =============================================================================
// Non-functional properties
// =============================================================================

// TestVersionCommand_IsDeterministic verifies that two invocations
// with identical inputs produce identical output.
func TestVersionCommand_IsDeterministic(t *testing.T) {
	t.Parallel()

	first := runCLI(t, []string{"version"}, nil)
	for i := 0; i < 5; i++ {
		got := runCLI(t, []string{"version"}, nil)
		if got != first {
			t.Fatalf("run %d produced different output:\n"+
				"first: %+v\n"+
				"got:   %+v", i, first, got)
		}
	}
}

// TestVersionCommand_IsDeterministicAcrossModes verifies that each
// output mode is deterministic across invocations. A mode whose
// output depends on iteration order (for example, a map) would fail
// this test.
func TestVersionCommand_IsDeterministicAcrossModes(t *testing.T) {
	t.Parallel()

	modes := [][]string{
		{"version"},
		{"version", "--format", "json"},
		{"version", "--verbose"},
		{"version", "--format", "json", "--verbose"},
	}
	for _, args := range modes {
		args := args
		name := strings.Join(args[1:], " ")
		if name == "" {
			name = "(default)"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			first := runCLI(t, args, nil)
			for i := 0; i < 3; i++ {
				got := runCLI(t, args, nil)
				if got != first {
					t.Fatalf("run %d differs:\nfirst: %+v\ngot:   %+v",
						i, first, got)
				}
			}
		})
	}
}

// =============================================================================
// Test helpers
// =============================================================================

// stripComments removes Go line comments (// ...) and block comments
// (/* ... */) from src, preserving newlines so that line numbers in
// subsequent error messages remain approximately correct.
//
// # Why this helper exists
//
// The structural tests in this file search the source of version.go
// for forbidden tokens such as "os.Stdout" and "os.Exit". The
// handler's docstring mentions those tokens to explain what the
// handler does *not* do. A naive scan over the whole file matches
// the docstring and reports a false positive.
//
// Stripping comments before the scan separates "the code references
// os.Stdout" from "the docstring mentions os.Stdout". The rule is
// about code, not prose.
//
// # Why not use go/parser
//
// go/parser would be more correct: it distinguishes comments,
// strings, and identifiers unambiguously. But it requires importing
// go/ast, go/token, and go/parser, and it returns a syntax tree
// rather than source text. The callers of this helper want source
// text with comments removed; extracting that from an AST is more
// work than the simple stripper below.
//
// # Line-number preservation
//
// A line comment is replaced with a single newline. A block comment
// is replaced with newlines equal to the number of newlines inside
// it. This preserves the file's line count.
func stripComments(src string) string {
	var b strings.Builder
	b.Grow(len(src))

	i := 0
	for i < len(src) {
		// Line comment: skip to end of line, emit the newline.
		if i+1 < len(src) && src[i] == '/' && src[i+1] == '/' {
			j := i + 2
			for j < len(src) && src[j] != '\n' {
				j++
			}
			if j < len(src) {
				b.WriteByte('\n')
				i = j + 1
			} else {
				i = j
			}
			continue
		}

		// Block comment: skip to "*/", emit one newline per newline
		// inside the comment.
		if i+1 < len(src) && src[i] == '/' && src[i+1] == '*' {
			j := i + 2
			for j+1 < len(src) && !(src[j] == '*' && src[j+1] == '/') {
				if src[j] == '\n' {
					b.WriteByte('\n')
				}
				j++
			}
			if j+1 < len(src) {
				i = j + 2
			} else {
				i = len(src)
			}
			continue
		}

		b.WriteByte(src[i])
		i++
	}

	return b.String()
}
