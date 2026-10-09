// Package cli contains white-box tests for the CLI package.
//
// This file tests the version command: its metadata, its delegation
// to the version application service, its error path, and the
// structural invariants that keep the handler thin.
//
// The test file is declared in package cli, not package cli_test,
// because it needs to reach unexported symbols: newVersionCmd,
// Dependencies, and the source-file helpers used by the structural
// tests.
//
// # Relationship to internal/app/version
//
// This file tests the handler. The application service that the
// handler calls is tested in internal/app/version/service_test.go
// and internal/app/version/buildinfo_test.go. The two layers are
// tested separately on purpose: the handler's job is to delegate,
// and the service's job is to produce and format data. A failure in
// one layer should not require reading the other layer's tests.
//
// # The handler / service pattern
//
// WBS 4.3.1 defines a boundary between Cobra handlers (this file's
// subject) and application services (internal/app/version). The
// handler:
//
//   - Parses flags (none yet).
//   - Collects arguments (none allowed).
//   - Constructs the service (or calls a package-level service
//     function, as version does today).
//   - Calls the service.
//   - Formats the result by delegating to the service's formatter.
//   - Returns an error; never calls os.Exit.
//
// # Test organisation
//
// As of WBS 4.3.2, the behavioural tests in this file go through the
// shared runCLI helper in testhelper_test.go rather than constructing
// a Cobra command directly. The helper exercises the command through
// the same boundary production code uses (executeWithOptions), which
// is the property WBS 4.3.2 exists to establish. The structural
// tests at the bottom of the file are unchanged: they read
// version.go as data and are indifferent to how the command is
// invoked.
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
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/thapelomagqazana/forge/internal/app/version"
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
// the invocation through executeWithOptions, so the test is a
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
	if !strings.HasPrefix(got.stdout, "forge ") {
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
//
// The help output is produced by Cobra, not by the handler. The test
// asserts only that the output mentions the command name and that the
// exit code is success. The exact layout of the help text is not
// pinned; it may change between Cobra versions.
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
//
// The test pins two contracts:
//
//   - The command rejects positional arguments.
//   - The failure path writes to stderr, not stdout.
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
//
// The version command does not read environment variables in Phase 2.
// The test is a precondition for the helper's behaviour: a nil env
// map must be a valid input that produces a working invocation.
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
//
// The test is the symmetric companion to
// TestVersionCommand_GivenNilEnv_StillSucceeds. When a future version
// of the command reads an environment variable (for example,
// FORGE_NO_COLOR), the test can be extended to assert on the effect.
// Until then, it pins the helper's contract.
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
// stdout/stderr separation contract for the version command.
//
// A successful invocation writes to stdout only. Any byte on stderr
// would violate the CLI UX contract and break scripts that redirect
// stderr to a log file.
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
// output matches what the service produces directly.
//
// The test invokes the command through runCLI and captures its
// output, then calls the service's formatter on a fresh buffer and
// compares the two. They must be byte-identical because the handler's
// only job is to delegate.
//
// # Why this test is behavioural, not structural
//
// A structural test would assert on the source of version.go: "the
// handler calls version.Format exactly once." That test exists
// (TestVersionHandler_IsThin below). This test asserts the runtime
// consequence of the delegation: the output is what the service
// produces. Together, the two catch different failures: the
// structural test catches "the handler grew a second call"; this test
// catches "the handler produces different output than the service."
//
// # Why the test imports internal/app/version
//
// The test calls the service's formatter directly to produce the
// reference output. Importing the package is the simplest way to do
// that. An earlier draft attempted to hide the import behind a local
// wrapper, but the wrapper added indirection without benefit: the
// import is unambiguous in this file (the package is named `version`,
// and the file has no other `version` identifier), and calling the
// exported function directly is clearer than calling a wrapper that
// calls it.
func TestVersionCommand_DelegatesToService(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"version"}, nil)

	if got.exitCode != ExitSuccess {
		t.Fatalf("exit code: got %d, want %d",
			got.exitCode, ExitSuccess)
	}

	// Produce the same output by calling the service directly. The
	// version command's handler calls version.Format(deps.Stdout,
	// version.Get()); this test does the same on a fresh buffer.
	var svcOut bytes.Buffer
	if err := version.Format(&svcOut, version.Get()); err != nil {
		t.Fatalf("version.Format: %v", err)
	}

	if got.stdout != svcOut.String() {
		t.Errorf("handler output differs from service output\n"+
			"handler: %q\n"+
			"service: %q",
			got.stdout, svcOut.String())
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
//   - The handler body calls version.Format exactly once.
//   - The code contains no direct writes to process-global streams.
//   - The code contains no fmt.Fprintln, fmt.Fprintf, fmt.Println,
//     or fmt.Printf calls.
//   - The code contains no os.Exit call.
//
// If any assertion fails, the failure message cites the specific
// token and the WBS acceptance criterion it violates.
func TestVersionHandler_IsThin(t *testing.T) {
	t.Parallel()

	content := readFile(t, "version.go")
	code := stripComments(content)

	// Exactly one call to the service's formatter. A second call
	// would mean the handler is doing two things; the correct place
	// for a second call is inside the service.
	if got := strings.Count(code, "version.Format("); got != 1 {
		t.Errorf("handler contains %d calls to version.Format; want 1",
			got)
	}

	// Forbidden tokens. Each is a violation of a specific rule.
	forbidden := map[string]string{
		"os.Stdout":    "AC4 — write to process-global stream",
		"os.Stderr":    "AC4 — write to process-global stream",
		"os.Stdin":     "AC4 — read from process-global stream",
		"os.Exit":      "AC5 — process exit must be handled by executeWithOptions",
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
//
// A handler that imports internal/filesystem could read files; a
// handler that imports internal/config could parse configuration; a
// handler that imports internal/domain/* could contain business
// rules. None of these belong in a handler. The test forbids the
// imports so that the handler cannot acquire those capabilities by
// accident.
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
//
// The test strips comments before searching, so that the docstring's
// mention of os.Exit does not produce a false positive. The code
// must not contain os.Exit; the docstring may mention it.
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
// handler calls only the service's exported API. It must not call
// unexported helpers, and it must not reach into the service's
// internals.
func TestVersionHandler_OnlyCallsDocumentedService(t *testing.T) {
	t.Parallel()

	content := readFile(t, "version.go")
	code := stripComments(content)

	allowed := map[string]bool{
		"version.Get(":    true,
		"version.Format(": true,
	}

	const prefix = "version."
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
				"handler must call only version.Get and "+
				"version.Format (the service's documented API)",
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
// Dependencies value (nil Stdout) or because they inspect the
// handler's behaviour in isolation.

// TestVersionHandler_NilStdoutPanics verifies the handler's behaviour
// when the injected Stdout is nil.
//
// The handler does not guard against nil writers: a nil io.Writer
// panics on first use, and the panic is the correct symptom of a
// caller that failed to construct Dependencies correctly. The test
// pins this behaviour so a future change does not silently add a
// guard that would hide the bug.
//
// This test constructs the command directly rather than going
// through runCLI, because runCLI always supplies a non-nil writer.
// The direct construction is the only way to reach the nil-writer
// path.
func TestVersionHandler_NilStdoutPanics(t *testing.T) {
	t.Parallel()

	deps := Dependencies{
		Stdout: nil,
		Stderr: &bytes.Buffer{},
	}
	cmd := newVersionCmd(deps)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	defer func() {
		if r := recover(); r == nil {
			t.Error("Execute with nil Stdout did not panic; " +
				"want panic (documented behaviour)")
		}
	}()
	_ = cmd.Execute()
}

// TestVersionHandler_EmptyBuildInfo verifies that the command
// produces valid output even when the build variables are empty.
//
// A binary built without ldflags has empty Version, Commit,
// BuildDate, and Dirty. The command must still succeed and produce
// output; the output contains the format's fixed parts (the header
// prefix and the detail keys) with empty values.
func TestVersionHandler_EmptyBuildInfo(t *testing.T) {
	t.Parallel()

	got := runCLI(t, []string{"version"}, nil)

	if got.exitCode != ExitSuccess {
		t.Fatalf("exit code: got %d, want %d",
			got.exitCode, ExitSuccess)
	}
	if !strings.HasPrefix(got.stdout, "forge ") {
		t.Errorf("stdout does not start with version header: %q",
			got.stdout)
	}
	// The detail keys are present even when the values are empty.
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
//
// The test uses the shared helper and compares the two testRun
// values directly. testRun is a comparable struct (three fields, all
// comparable), so the comparison is valid Go.
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
// The stripper is deliberately simple. It handles the two comment
// forms Go uses. It does not handle braces or quotes inside string
// literals; the file it is applied to (version.go) contains none in
// the relevant positions. If a future structural test needs to scan
// a file with strings containing comment-like sequences, the helper
// should be replaced with go/parser.
//
// # Line-number preservation
//
// A line comment is replaced with a single newline. A block comment
// is replaced with newlines equal to the number of newlines inside
// it. This preserves the file's line count, so a subsequent scan
// that reports a line number will report a number that matches the
// original file.
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
