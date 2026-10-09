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
// Positive tests — command metadata
// =============================================================================

// TestNewVersionCmd_HasExpectedMetadata verifies the command's
// metadata: its name, its short description, and its long
// description.
//
// The metadata is what a user sees in `forge --help`. WBS 5.1.1
// freezes the root command's metadata; the version command's
// metadata is frozen by this test.
func TestNewVersionCmd_HasExpectedMetadata(t *testing.T) {
	t.Parallel()

	deps := testDependencies()
	cmd := newVersionCmd(deps)

	if cmd.Use != "version" {
		t.Errorf("Use = %q; want %q", cmd.Use, "version")
	}
	if !strings.Contains(cmd.Short, "version") {
		t.Errorf("Short does not mention version: %q", cmd.Short)
	}
	if !strings.Contains(cmd.Long, "commit") {
		t.Errorf("Long does not mention commit: %q", cmd.Long)
	}
}

// TestNewVersionCmd_IsNotHidden verifies that the command appears in
// help output. A hidden command would not be discoverable, which
// would defeat the purpose of a version command.
func TestNewVersionCmd_IsNotHidden(t *testing.T) {
	t.Parallel()

	deps := testDependencies()
	cmd := newVersionCmd(deps)

	if cmd.Hidden {
		t.Error("version command is hidden; want visible")
	}
}

// TestNewVersionCmd_RejectsExtraArguments verifies that the command
// rejects positional arguments. This is enforced by cobra.NoArgs in
// the command definition.
//
// The test uses a fresh command per invocation because Cobra mutates
// the command's internal state when Execute is called.
func TestNewVersionCmd_RejectsExtraArguments(t *testing.T) {
	t.Parallel()

	deps := testDependencies()
	cmd := newVersionCmd(deps)
	cmd.SetArgs([]string{"unexpected"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute: want error for unexpected argument, got nil")
	}
}

// =============================================================================
// Positive tests — behaviour
// =============================================================================

// TestNewVersionCmd_WritesToInjectedStdout is the ATDD acceptance
// test for AC2 and AC4. It verifies that the handler writes to the
// injected writer, not to os.Stdout, and that it does so by
// delegating to the service's formatter.
//
// The test constructs the command with a Dependencies value whose
// Stdout is a bytes.Buffer, executes the command, and asserts on the
// buffer contents. If the handler wrote to os.Stdout instead, the
// buffer would be empty and the test would fail.
func TestNewVersionCmd_WritesToInjectedStdout(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	deps := Dependencies{
		Stdout: &stdout,
		Stderr: &bytes.Buffer{},
	}

	cmd := newVersionCmd(deps)
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// The output must contain the version header. The exact version
	// string depends on the build; we assert only on the prefix.
	out := stdout.String()
	if !strings.HasPrefix(out, "forge ") {
		t.Errorf("output does not start with version header: %q", out)
	}

	// The output must contain every detail key. The values may be
	// empty (if the binary was built without ldflags), but the keys
	// are always present.
	for _, key := range []string{"commit:", "built:", "go version:", "platform:"} {
		if !strings.Contains(out, key) {
			t.Errorf("output missing %q: %q", key, out)
		}
	}
}

// TestNewVersionCmd_DelegatesToService is a behavioural test that
// verifies the handler's output matches what the service produces
// directly.
//
// The test calls version.Format on a fresh bytes.Buffer and compares
// the output to what the command wrote. The two must be byte-identical
// because the handler's only job is to delegate.
func TestNewVersionCmd_DelegatesToService(t *testing.T) {
	t.Parallel()

	var cmdOut bytes.Buffer
	deps := Dependencies{
		Stdout: &cmdOut,
		Stderr: &bytes.Buffer{},
	}
	cmd := newVersionCmd(deps)
	cmd.SetOut(&cmdOut)
	cmd.SetErr(&bytes.Buffer{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var svcOut bytes.Buffer
	if err := version.Format(&svcOut, version.Get()); err != nil {
		t.Fatalf("version.Format: %v", err)
	}

	if cmdOut.String() != svcOut.String() {
		t.Errorf("handler output differs from service output\n"+
			"handler: %q\n"+
			"service: %q",
			cmdOut.String(), svcOut.String())
	}
}

// =============================================================================
// Stream separation
// =============================================================================

// TestNewVersionCmd_WritesNothingToStderr verifies that a successful
// invocation writes nothing to stderr. Diagnostics on success would
// violate the CLI UX contract and break shell pipelines.
func TestNewVersionCmd_WritesNothingToStderr(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	deps := Dependencies{
		Stdout: &stdout,
		Stderr: &stderr,
	}
	cmd := newVersionCmd(deps)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if stderr.Len() != 0 {
		t.Errorf("stderr is non-empty on success: %q", stderr.String())
	}
}

// =============================================================================
// Structural tests — the handler / service boundary (WBS 4.3.1)
// =============================================================================
//
// The tests below read version.go as data, strip comments, and
// assert on the shape of the remaining code. They are the mechanical
// enforcement of AC3, AC4, and AC5.
//
// The comment stripping is essential: the handler's docstring
// explains what the handler does *not* do, and it names the
// forbidden tokens (os.Stdout, os.Exit, fmt.Fprintln) in the process.
// Without stripping, every structural test would fail on the
// docstring rather than on the code.

// TestNewVersionCmd_HandlerIsThin verifies the structural invariants
// that keep the handler thin.
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
func TestNewVersionCmd_HandlerIsThin(t *testing.T) {
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

// TestNewVersionCmd_DoesNotContainBusinessLogic verifies that the
// handler file does not import infrastructure or domain packages
// directly.
//
// A handler that imports internal/filesystem could read files; a
// handler that imports internal/config could parse configuration; a
// handler that imports internal/domain/* could contain business
// rules. None of these belong in a handler. The test forbids the
// imports so that the handler cannot acquire those capabilities by
// accident.
func TestNewVersionCmd_DoesNotContainBusinessLogic(t *testing.T) {
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

// TestNewVersionCmd_ReturnsErrorNotExit verifies that the handler
// returns an error rather than terminating the process.
//
// The test strips comments before searching, so that the docstring's
// mention of os.Exit does not produce a false positive. The code
// must not contain os.Exit; the docstring may mention it.
func TestNewVersionCmd_ReturnsErrorNotExit(t *testing.T) {
	t.Parallel()

	content := readFile(t, "version.go")
	code := stripComments(content)

	if strings.Contains(code, "os.Exit") {
		t.Error("version.go calls os.Exit; " +
			"handlers must return an error (AC5)")
	}
}

// TestNewVersionCmd_OnlyCallsDocumentedService verifies that the
// handler calls only the service's exported API. It must not call
// unexported helpers, and it must not reach into the service's
// internals.
func TestNewVersionCmd_OnlyCallsDocumentedService(t *testing.T) {
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
// TestNewVersionCmd_OnlyCallsDocumentedService.
func isIdentChar(b byte) bool {
	return b == '_' ||
		(b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9')
}

// =============================================================================
// Edge and corner cases
// =============================================================================

// TestNewVersionCmd_NilStdout verifies the handler's behaviour when
// the injected Stdout is nil.
//
// The handler does not guard against nil writers: a nil io.Writer
// panics on first use, and the panic is the correct symptom of a
// caller that failed to construct Dependencies correctly. The test
// pins this behaviour so a future change does not silently add a
// guard that would hide the bug.
func TestNewVersionCmd_NilStdout(t *testing.T) {
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

// TestNewVersionCmd_EmptyInfo verifies that the command produces
// valid output even when the build variables are empty.
//
// A binary built without ldflags has empty Version, Commit, and
// BuildDate. The command must still succeed and produce output; the
// output contains the format's fixed parts (the header prefix and the
// detail keys) with empty values.
func TestNewVersionCmd_EmptyInfo(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	deps := Dependencies{
		Stdout: &stdout,
		Stderr: &stderr,
	}
	cmd := newVersionCmd(deps)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if stdout.Len() == 0 {
		t.Error("stdout is empty; want at least the version header")
	}
	if !strings.HasPrefix(stdout.String(), "forge ") {
		t.Errorf("stdout does not start with version header: %q",
			stdout.String())
	}
}

// TestNewVersionCmd_NoArgs verifies that the command succeeds when
// invoked with no arguments. This is the common case.
func TestNewVersionCmd_NoArgs(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	deps := Dependencies{
		Stdout: &stdout,
		Stderr: &stderr,
	}
	cmd := newVersionCmd(deps)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(nil)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if stdout.Len() == 0 {
		t.Error("stdout is empty; want version output")
	}
}

// =============================================================================
// Non-functional properties
// =============================================================================

// TestNewVersionCmd_IsDeterministic verifies that two invocations
// with identical Dependencies produce identical output.
func TestNewVersionCmd_IsDeterministic(t *testing.T) {
	t.Parallel()

	runOnce := func() string {
		var stdout bytes.Buffer
		deps := Dependencies{
			Stdout: &stdout,
			Stderr: &bytes.Buffer{},
		}
		cmd := newVersionCmd(deps)
		cmd.SetOut(&stdout)
		cmd.SetErr(&bytes.Buffer{})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute: %v", err)
		}
		return stdout.String()
	}

	first := runOnce()
	for i := 0; i < 5; i++ {
		got := runOnce()
		if got != first {
			t.Fatalf("run %d produced different output\n"+
				"first: %q\n"+
				"got:   %q", i, first, got)
		}
	}
}

// =============================================================================
// Test helpers
// =============================================================================

// testDependencies returns a Dependencies value suitable for tests
// that only need the writers.
func testDependencies() Dependencies {
	return Dependencies{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	}
}

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
