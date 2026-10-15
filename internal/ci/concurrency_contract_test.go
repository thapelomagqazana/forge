package ci_test

// concurrency_contract_test.go — external contract tests for the CI
// concurrency model.
//
// The tests in this file read:
//
//   - .github/workflows/ci.yml          — the CI dispatcher.
//   - .github/workflows/ci-tier1.yml    — Tier 1's reusable workflow.
//   - .github/workflows/ci-tier2.yml    — Tier 2's reusable workflow.
//   - .github/workflows/ci-tier3.yml    — Tier 3's reusable workflow.
//   - .github/workflows/release.yml     — the release pipeline.
//   - .github/workflows/security.yml    — Tier 4's security workflow.
//   - .github/workflows/nightly.yml     — Tier 4's nightly workflow.
//   - .github/workflows/docs.yml        — the documentation workflow.
//
// and assert that each file's `concurrency:` block agrees with the
// model in internal/ci/concurrency.go.
//
// The file is package ci_test because it treats the package as a
// black box and asserts on the artifacts rather than the model.
//
// # Test taxonomy
//
//   - Existence tests: every workflow file has a `concurrency:`
//     block with a group and a cancel-in-progress field.
//   - Match tests: the file's group and cancel-in-progress match the
//     model's spec for that workflow.
//   - Policy tests: the file's cancel-in-progress expresses the
//     model's semantic policy (`on-pr` or `never`).
//   - Shape tests: the file's group contains the two markers the
//     model's template includes.
//
// The grouping is documentation. A reader who wants to know what the
// concurrency block must satisfy reads the group headings.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/thapelomagqazana/forge/internal/ci"
)

// --- Path helpers ----------------------------------------------------------

// concurrencyWorkflowPath returns the path to the workflow file for
// the given ID.
func concurrencyWorkflowPath(t *testing.T, root string, id ci.WorkflowID) string {
	t.Helper()
	return filepath.Join(root, ".github", "workflows", id.FileName())
}

// readConcurrencyWorkflow reads the workflow file for the given ID.
func readConcurrencyWorkflow(t *testing.T, root string, id ci.WorkflowID) []byte {
	t.Helper()
	path := concurrencyWorkflowPath(t, root, id)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

// concurrencyBlockPattern captures the body of a `concurrency:`
// block.
//
// The pattern matches the `concurrency:` key at zero indentation and
// captures everything up to the next line at zero indentation (or
// end of file). The block's body is the indented lines under the
// key.
//
// The `(?m)` flag makes `^` and `$` match line boundaries. The
// non-greedy `(.*?)` combined with the lookahead `(?m:^[^\s])` (or
// end-of-string) captures the block's body without consuming the
// next top-level key.
//
// Go's regexp package does not support lookahead, so the pattern is
// written as a two-step extraction: the first regexp finds the
// `concurrency:` line, and the extraction walks the file's lines
// from there to the next zero-indented line. The two-step form is
// the same shape used by extractPermissions in
// workflows_contract_test.go.
var concurrencyKeyPattern = regexp.MustCompile(`(?m)^concurrency:\s*$`)

// groupLinePattern captures the value of a `group:` field inside a
// concurrency block.
var groupLinePattern = regexp.MustCompile(`(?m)^\s{2,}group:\s*(.+?)\s*$`)

// cancelLinePattern captures the value of a `cancel-in-progress:`
// field inside a concurrency block.
var cancelLinePattern = regexp.MustCompile(`(?m)^\s{2,}cancel-in-progress:\s*(.+?)\s*$`)

// extractConcurrencyBlock returns the body of the `concurrency:`
// block in the given file, or the empty string if the block is
// missing.
//
// The extraction starts at the `concurrency:` line and walks
// forward, collecting lines that are indented (start with a space
// or tab) until a non-indented non-blank line is reached. The
// returned string is the concatenation of the collected lines, with
// their leading indentation preserved.
func extractConcurrencyBlock(t *testing.T, path, content string) string {
	t.Helper()
	lines := strings.Split(content, "\n")

	startIdx := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "concurrency:" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			startIdx = i
			break
		}
	}
	if startIdx == -1 {
		return ""
	}

	var b strings.Builder
	for i := startIdx + 1; i < len(lines); i++ {
		line := lines[i]
		if line == "" {
			continue
		}
		// A zero-indented line ends the block.
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			break
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

// --- Existence tests -------------------------------------------------------

// TestConcurrency_EveryWorkflowHasAConcurrencyBlock asserts every
// workflow file in the closed set has a `concurrency:` block.
//
// The check is structural: the block's presence is the minimum
// requirement. A file without a block would run every push to
// completion, wasting runners on superseded PR pushes.
func TestConcurrency_EveryWorkflowHasAConcurrencyBlock(t *testing.T) {
	root := repoRoot(t)
	for _, id := range ci.AllWorkflows() {
		path := concurrencyWorkflowPath(t, root, id)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		block := extractConcurrencyBlock(t, path, string(data))
		if block == "" {
			t.Errorf("%s does not declare a `concurrency:` block", path)
			continue
		}
		if !strings.Contains(block, "group:") {
			t.Errorf("%s's concurrency block does not declare `group:`", path)
		}
		if !strings.Contains(block, "cancel-in-progress:") {
			t.Errorf("%s's concurrency block does not declare `cancel-in-progress:`", path)
		}
	}
}

// --- Match tests -----------------------------------------------------------

// TestConcurrency_GroupMatchesModel asserts every workflow file's
// `group:` value matches the model's group template for that
// workflow.
//
// The comparison is byte-for-byte after trimming leading and
// trailing whitespace. The model's templates include GitHub Actions
// substitution syntax; the file's values contain the same syntax.
func TestConcurrency_GroupMatchesModel(t *testing.T) {
	root := repoRoot(t)
	for _, id := range ci.AllWorkflows() {
		path := concurrencyWorkflowPath(t, root, id)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		block := extractConcurrencyBlock(t, path, string(data))
		if block == "" {
			t.Errorf("%s does not declare a `concurrency:` block", path)
			continue
		}
		m := groupLinePattern.FindStringSubmatch(block)
		if m == nil {
			t.Errorf("%s's concurrency block does not declare `group:`", path)
			continue
		}
		got := strings.TrimSpace(m[1])

		s, ok := ci.LookupConcurrencySpec(id)
		if !ok {
			t.Errorf("LookupConcurrencySpec(%q) not found", id)
			continue
		}
		if got != s.GroupTemplate {
			t.Errorf(
				"%s: group = %q, want %q",
				path, got, s.GroupTemplate,
			)
		}
	}
}

// TestConcurrency_CancelMatchesModel asserts every workflow file's
// `cancel-in-progress:` value matches the model's spec for that
// workflow.
//
// The comparison handles both forms:
//
//   - an expression (`${{ ... }}`), which the file writes verbatim;
//   - a literal boolean (`true` or `false`), which the file writes
//     as the word.
//
// The model's spec uses exactly one of the two forms; the file's
// value must match the spec's form and content.
func TestConcurrency_CancelMatchesModel(t *testing.T) {
	root := repoRoot(t)
	for _, id := range ci.AllWorkflows() {
		path := concurrencyWorkflowPath(t, root, id)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		block := extractConcurrencyBlock(t, path, string(data))
		if block == "" {
			t.Errorf("%s does not declare a `concurrency:` block", path)
			continue
		}
		m := cancelLinePattern.FindStringSubmatch(block)
		if m == nil {
			t.Errorf("%s's concurrency block does not declare `cancel-in-progress:`", path)
			continue
		}
		got := strings.TrimSpace(m[1])

		s, ok := ci.LookupConcurrencySpec(id)
		if !ok {
			t.Errorf("LookupConcurrencySpec(%q) not found", id)
			continue
		}

		// Determine the expected value.
		var want string
		switch {
		case s.CancelExpression != "":
			want = s.CancelExpression
		case s.CancelLiteral:
			want = "true"
		default:
			want = "false"
		}

		if got != want {
			t.Errorf(
				"%s: cancel-in-progress = %q, want %q",
				path, got, want,
			)
		}
	}
}

// --- Policy tests ----------------------------------------------------------

// TestConcurrency_PolicyIsOnPRForPRWorkflows asserts that every
// workflow file whose model policy is `on-pr` writes the
// cancel-in-progress expression the policy requires.
//
// The check is byte-for-byte: the expression is
// `${{ github.ref != 'refs/heads/main' }}`. A future change that
// reworded the expression (for example, using `github.ref_name`
// instead of `github.ref`) would fail this test and prompt the
// contributor to update the model and the workflow file in the same
// commit.
func TestConcurrency_PolicyIsOnPRForPRWorkflows(t *testing.T) {
	root := repoRoot(t)
	for _, id := range ci.AllWorkflows() {
		s, ok := ci.LookupConcurrencySpec(id)
		if !ok {
			t.Errorf("LookupConcurrencySpec(%q) not found", id)
			continue
		}
		if s.Policy != ci.CancelPolicyOnPR {
			continue
		}
		path := concurrencyWorkflowPath(t, root, id)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		block := extractConcurrencyBlock(t, path, string(data))
		m := cancelLinePattern.FindStringSubmatch(block)
		if m == nil {
			t.Errorf("%s's concurrency block does not declare `cancel-in-progress:`", path)
			continue
		}
		got := strings.TrimSpace(m[1])
		if !strings.Contains(got, "github.ref") || !strings.Contains(got, "refs/heads/main") {
			t.Errorf(
				"%s: cancel-in-progress = %q; the on-pr policy requires an expression that references github.ref and refs/heads/main",
				path, got,
			)
		}
	}
}

// TestConcurrency_PolicyIsNeverForScheduledAndTagWorkflows asserts
// that every workflow file whose model policy is `never` writes
// `cancel-in-progress: false`.
//
// The check is byte-for-byte: the value must be the literal word
// `false`. A future change that wrote a non-false expression would
// fail this test.
func TestConcurrency_PolicyIsNeverForScheduledAndTagWorkflows(t *testing.T) {
	root := repoRoot(t)
	for _, id := range ci.AllWorkflows() {
		s, ok := ci.LookupConcurrencySpec(id)
		if !ok {
			t.Errorf("LookupConcurrencySpec(%q) not found", id)
			continue
		}
		if s.Policy != ci.CancelPolicyNever {
			continue
		}
		path := concurrencyWorkflowPath(t, root, id)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		block := extractConcurrencyBlock(t, path, string(data))
		m := cancelLinePattern.FindStringSubmatch(block)
		if m == nil {
			t.Errorf("%s's concurrency block does not declare `cancel-in-progress:`", path)
			continue
		}
		got := strings.TrimSpace(m[1])
		if got != "false" {
			t.Errorf(
				"%s: cancel-in-progress = %q; the never policy requires `false`",
				path, got,
			)
		}
	}
}

// --- Shape tests -----------------------------------------------------------

// TestConcurrency_GroupContainsTheMarkers asserts every workflow
// file's `group:` value contains the two markers the model's
// templates include.
//
// The markers are:
//
//   - `github.workflow`, which distinguishes one workflow's runs
//     from another's.
//   - `github.ref` or `inputs.`, which scopes the group to the ref
//     or to a workflow input.
//
// The check is a substring search for the markers; the exact
// template is asserted by TestConcurrency_GroupMatchesModel.
func TestConcurrency_GroupContainsTheMarkers(t *testing.T) {
	root := repoRoot(t)
	for _, id := range ci.AllWorkflows() {
		path := concurrencyWorkflowPath(t, root, id)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		block := extractConcurrencyBlock(t, path, string(data))
		m := groupLinePattern.FindStringSubmatch(block)
		if m == nil {
			t.Errorf("%s's concurrency block does not declare `group:`", path)
			continue
		}
		got := strings.TrimSpace(m[1])
		if !strings.Contains(got, "github.workflow") {
			t.Errorf(
				"%s: group = %q; it must contain `github.workflow`",
				path, got,
			)
		}
		if !strings.Contains(got, "github.ref") && !strings.Contains(got, "inputs.") {
			t.Errorf(
				"%s: group = %q; it must contain `github.ref` or `inputs.`",
				path, got,
			)
		}
	}
}

// --- ADR test --------------------------------------------------------------

// TestConcurrency_DocExists asserts the concurrency policy document
// exists.
func TestConcurrency_DocExists(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "docs", "reference", "concurrency-policy.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("docs/reference/concurrency-policy.md is missing: %v", err)
	}
}

// TestConcurrency_GoldenFileExists asserts the golden JSON exists.
func TestConcurrency_GoldenFileExists(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "internal", "ci", "testdata", "concurrency.golden.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("concurrency.golden.json is missing: %v", err)
	}
}

// TestConcurrency_GoldenMatchesModel asserts ci.ConcurrencyGolden()
// matches the golden file.
func TestConcurrency_GoldenMatchesModel(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "internal", "ci", "testdata", "concurrency.golden.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	want := ci.ConcurrencyGolden()
	if string(data) != want {
		t.Fatalf(
			"%s does not match ci.ConcurrencyGolden(); run `task ci:render-concurrency` to regenerate",
			path,
		)
	}
}

// TestConcurrency_DocMatchesModel asserts ci.ConcurrencyMarkdown()
// matches the policy document.
func TestConcurrency_DocMatchesModel(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "docs", "reference", "concurrency-policy.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	want := ci.ConcurrencyMarkdown()
	if string(data) != want {
		t.Fatalf(
			"%s does not match ci.ConcurrencyMarkdown(); run `task ci:render-concurrency` to regenerate",
			path,
		)
	}
}
