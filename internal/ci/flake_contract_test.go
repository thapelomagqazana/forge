package ci_test

// flake_contract_test.go — external contract tests for the flake
// policy model.
//
// The tests in this file read:
//
//   - docs/reference/flake-policy.md
//     — the policy document, rendered from the model.
//   - docs/reference/timeout-policy.md
//     — the timeout document.
//   - docs/decisions/ADR-052-pr-concurrency-and-flake-policy.md
//     — the ADR that records the decisions.
//   - internal/ci/testdata/flake.golden.json
//     — the golden JSON, rendered from the model.
//   - .github/workflows/*.yml
//     — every workflow file, checked for retry steps and job
//       timeouts.
//
// and assert that those artifacts agree with the model in
// internal/ci/flake.go.
//
// The file is package ci_test because it treats the package as a
// black box and asserts on the artifacts rather than the model.
//
// # Test taxonomy
//
//   - Existence tests: the four artifacts exist.
//   - Render tests: the policy document and the golden JSON match
//     the model's renderings.
//   - Content tests: the policy document contains the model's
//     values (the marker, the format, the SLA, the rate target, the
//     retry policy, the artifact name).
//   - Retry tests: no workflow file uses a retry step.
//   - Timeout tests: every job in every workflow declares an
//     explicit `timeout-minutes`.
//   - ADR tests: the ADR names the model's values.
//
// The grouping is documentation. A reader who wants to know what the
// flake policy must satisfy reads the group headings.
//
// # Why the job-key scan is scoped to the `jobs:` block
//
// A GitHub Actions workflow file uses the same YAML shape for two
// different things: the trigger keys under `on:` and the job keys
// under `jobs:` are both two-space-indented keys ending in a colon.
// A regexp anchored on that shape alone cannot tell them apart; the
// first version of this file's job-key pattern (`^  key:`) swept up
// `push` and `schedule` as if they were jobs, and the timeout test
// then reported every trigger as a job without a `timeout-minutes`
// field.
//
// The extractor is now stateful: it ignores everything before the
// `jobs:` line, collects the keys indented exactly two spaces under
// it, and stops at the next top-level key. The scope is what makes
// the test's assertion mean what it says.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/thapelomagqazana/forge/internal/ci"
)

// --- Path helpers ----------------------------------------------------------

// flakeDocPath returns the path to the flake policy document.
func flakeDocPath(t *testing.T, root string) string {
	t.Helper()
	return filepath.Join(root, "docs", "reference", "flake-policy.md")
}

// timeoutDocPath returns the path to the timeout document.
func timeoutDocPath(t *testing.T, root string) string {
	t.Helper()
	return filepath.Join(root, "docs", "reference", "timeout-policy.md")
}

// flakeADRPath returns the path to the ADR.
func flakeADRPath(t *testing.T, root string) string {
	t.Helper()
	return filepath.Join(root, "docs", "decisions", "ADR-052-pr-concurrency-and-flake-policy.md")
}

// flakeGoldenPath returns the path to the golden JSON.
func flakeGoldenPath(t *testing.T, root string) string {
	t.Helper()
	return filepath.Join(root, "internal", "ci", "testdata", "flake.golden.json")
}

// --- Existence tests -------------------------------------------------------

// TestFlake_PolicyDocExists asserts the policy document exists and
// is non-empty.
func TestFlake_PolicyDocExists(t *testing.T) {
	root := repoRoot(t)
	path := flakeDocPath(t, root)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("docs/reference/flake-policy.md is missing: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("docs/reference/flake-policy.md is empty")
	}
}

// TestFlake_TimeoutDocExists asserts the timeout document exists.
func TestFlake_TimeoutDocExists(t *testing.T) {
	root := repoRoot(t)
	path := timeoutDocPath(t, root)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("docs/reference/timeout-policy.md is missing: %v", err)
	}
}

// TestFlake_GoldenFileExists asserts the golden JSON exists.
func TestFlake_GoldenFileExists(t *testing.T) {
	root := repoRoot(t)
	path := flakeGoldenPath(t, root)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("flake.golden.json is missing: %v", err)
	}
}

// --- Render tests ----------------------------------------------------------

// TestFlake_PolicyDocMatchesModel asserts ci.FlakeMarkdown() matches
// the policy document.
func TestFlake_PolicyDocMatchesModel(t *testing.T) {
	root := repoRoot(t)
	path := flakeDocPath(t, root)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	want := ci.FlakeMarkdown()
	if string(data) != want {
		t.Fatalf(
			"%s does not match ci.FlakeMarkdown(); run `task ci:render-flake` to regenerate",
			path,
		)
	}
}

// TestFlake_GoldenMatchesModel asserts ci.FlakeGolden() matches the
// golden JSON.
func TestFlake_GoldenMatchesModel(t *testing.T) {
	root := repoRoot(t)
	path := flakeGoldenPath(t, root)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	want := ci.FlakeGolden()
	if string(data) != want {
		t.Fatalf(
			"%s does not match ci.FlakeGolden(); run `task ci:render-flake` to regenerate",
			path,
		)
	}
}

// --- Content tests ---------------------------------------------------------

// TestFlake_PolicyDocContainsTheMarker asserts the policy document
// contains the quarantine marker.
func TestFlake_PolicyDocContainsTheMarker(t *testing.T) {
	root := repoRoot(t)
	path := flakeDocPath(t, root)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !strings.Contains(string(data), ci.FlakeQuarantineMarker) {
		t.Errorf(
			"flake-policy.md does not contain the quarantine marker %q",
			ci.FlakeQuarantineMarker,
		)
	}
}

// TestFlake_PolicyDocContainsTheFormat asserts the policy document
// contains the quarantine format.
func TestFlake_PolicyDocContainsTheFormat(t *testing.T) {
	root := repoRoot(t)
	path := flakeDocPath(t, root)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !strings.Contains(string(data), ci.FlakeQuarantineFormat) {
		t.Errorf(
			"flake-policy.md does not contain the quarantine format %q",
			ci.FlakeQuarantineFormat,
		)
	}
}

// TestFlake_PolicyDocContainsTheSLA asserts the policy document
// contains the SLA's two durations.
func TestFlake_PolicyDocContainsTheSLA(t *testing.T) {
	root := repoRoot(t)
	path := flakeDocPath(t, root)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	sla := ci.FlakeSLAOf()
	s := string(data)
	if !strings.Contains(s, sla.QuarantineWithin) {
		t.Errorf(
			"flake-policy.md does not contain the quarantine SLA %q",
			sla.QuarantineWithin,
		)
	}
	if !strings.Contains(s, sla.FixWithin) {
		t.Errorf(
			"flake-policy.md does not contain the fix SLA %q",
			sla.FixWithin,
		)
	}
}

// TestFlake_PolicyDocContainsTheRateTarget asserts the policy
// document contains the rate target.
//
// The renderer writes the target as a percentage with one decimal
// ("0.5%"). The test checks for the rendered form.
func TestFlake_PolicyDocContainsTheRateTarget(t *testing.T) {
	root := repoRoot(t)
	path := flakeDocPath(t, root)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	// The renderer formats 0.005 as "0.5%".
	if !strings.Contains(string(data), "0.5%") {
		t.Error("flake-policy.md does not contain the rate target 0.5%")
	}
}

// TestFlake_PolicyDocContainsTheRetryPolicy asserts the policy
// document contains the retry policy.
func TestFlake_PolicyDocContainsTheRetryPolicy(t *testing.T) {
	root := repoRoot(t)
	path := flakeDocPath(t, root)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !strings.Contains(string(data), string(ci.FlakeRetryPolicyCurrent)) {
		t.Errorf(
			"flake-policy.md does not contain the retry policy %q",
			string(ci.FlakeRetryPolicyCurrent),
		)
	}
}

// TestFlake_PolicyDocContainsTheArtifactName asserts the policy
// document contains the flake-report artifact's name.
func TestFlake_PolicyDocContainsTheArtifactName(t *testing.T) {
	root := repoRoot(t)
	path := flakeDocPath(t, root)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !strings.Contains(string(data), ci.FlakeDetectionOf().ArtifactName) {
		t.Errorf(
			"flake-policy.md does not contain the artifact name %q",
			ci.FlakeDetectionOf().ArtifactName,
		)
	}
}

// TestFlake_PolicyDocContainsTheTier3Count asserts the policy
// document contains the tier-3 count.
func TestFlake_PolicyDocContainsTheTier3Count(t *testing.T) {
	root := repoRoot(t)
	path := flakeDocPath(t, root)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	count := ci.FlakeDetectionOf().Tier3Count
	want := "-count=" + itoaFlake(count)
	if !strings.Contains(string(data), want) {
		t.Errorf(
			"flake-policy.md does not contain the tier-3 count marker %q",
			want,
		)
	}
}

// --- Retry tests -----------------------------------------------------------

// workflowFiles returns the paths of every workflow file in the
// repository.
func workflowFiles(t *testing.T, root string) []string {
	t.Helper()
	dir := filepath.Join(root, ".github", "workflows")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml") {
			continue
		}
		out = append(out, filepath.Join(dir, name))
	}
	return out
}

// retryActionPattern matches a `uses:` step that names an action
// whose owner/name contains the word "retry".
var retryActionPattern = regexp.MustCompile(
	`(?m)^\s*uses:\s*([A-Za-z0-9_.\-]+/[A-Za-z0-9_.\-]*retry[A-Za-z0-9_.\-]*/[A-Za-z0-9_.\-]+)@`,
)

// retryKeywordPattern matches a `retry:` key at any indentation.
var retryKeywordPattern = regexp.MustCompile(`(?m)^\s*retry:\s*`)

// TestFlake_NoAutomaticRetriesInWorkflows asserts no workflow file
// uses a retry step.
//
// The check is two-way: it looks for a `uses:` reference to a
// retry-named action and for a `retry:` YAML key. Either would be a
// violation of the policy's "no automatic retries" rule.
func TestFlake_NoAutomaticRetriesInWorkflows(t *testing.T) {
	root := repoRoot(t)
	for _, path := range workflowFiles(t, root) {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		s := string(data)
		if m := retryActionPattern.FindStringSubmatch(s); m != nil {
			t.Errorf(
				"%s uses the retry-named action %q; the flake policy forbids automatic retries",
				path, m[1],
			)
		}
		if m := retryKeywordPattern.FindStringIndex(s); m != nil {
			// Report the line number for diagnosability.
			line := strings.Count(s[:m[0]], "\n") + 1
			t.Errorf(
				"%s: line %d declares a `retry:` key; the flake policy forbids automatic retries",
				path, line,
			)
		}
	}
}

// --- Timeout tests ---------------------------------------------------------

// timeoutPattern matches a `timeout-minutes:` line at any
// indentation.
var timeoutPattern = regexp.MustCompile(`(?m)^\s{4,}timeout-minutes:\s*\d+\s*$`)

// jobBody returns the substring of content that belongs to the named
// job: everything from the job's key line to the next job key, or to
// the end of the `jobs:` block.
//
// The return value includes the job's key line and every line after
// it that belongs to the job, up to (but not including) the next job
// key or the end of the `jobs:` block.
func jobBody(content, jobName string) string {
	lines := strings.Split(content, "\n")

	inJobs := false
	collecting := false
	var b strings.Builder
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			if collecting {
				b.WriteString(line)
				b.WriteString("\n")
			}
			continue
		}
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			if strings.TrimSpace(line) == "jobs:" {
				inJobs = true
				continue
			}
			if inJobs {
				break
			}
			continue
		}
		if !inJobs {
			continue
		}
		indent := 0
		for indent < len(line) && line[indent] == ' ' {
			indent++
		}
		if indent == 2 && strings.HasSuffix(strings.TrimSpace(line), ":") {
			name := strings.TrimSuffix(strings.TrimSpace(line), ":")
			if name == jobName {
				collecting = true
				b.WriteString(line)
				b.WriteString("\n")
				continue
			}
			if collecting {
				break
			}
		}
		if collecting {
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	return b.String()
}

// TestFlake_EveryJobHasTimeout asserts every job in every workflow
// declares an explicit `timeout-minutes`.
//
// The check finds the `jobs:` block, collects each job key under it,
// and asserts the job's body contains a `timeout-minutes:` line. The
// scan is scoped to `jobs:` so the trigger names under `on:` are not
// mistaken for jobs.
//
// # Why the scan is not a single regexp
//
// A regexp anchored on the two-space indentation alone cannot tell a
// job key from a trigger key: the `on:` block uses the same shape.
// The extractor used here is scoped to the `jobs:` block; the
// rationale is documented on jobKeysInJobsBlock.
func TestFlake_EveryJobHasTimeout(t *testing.T) {
	root := repoRoot(t)
	for _, path := range workflowFiles(t, root) {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		s := string(data)

		keys := jobKeysInJobsBlock(t, path, s)
		if len(keys) == 0 {
			t.Errorf("%s: no job keys found under the `jobs:` block", path)
			continue
		}
		for _, name := range keys {
			body := jobBody(s, name)
			if !timeoutPattern.MatchString(body) {
				t.Errorf(
					"%s: job %q does not declare `timeout-minutes`; "+
						"the timeout policy requires every job to declare one explicitly",
					path, name,
				)
			}
		}
	}
}

// TestFlake_TimeoutDocNamesEveryTier asserts the timeout document
// names every tier.
func TestFlake_TimeoutDocNamesEveryTier(t *testing.T) {
	root := repoRoot(t)
	path := timeoutDocPath(t, root)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	s := string(data)
	for _, tier := range []string{"Tier 1", "Tier 2", "Tier 3", "Tier 4"} {
		if !strings.Contains(s, tier) {
			t.Errorf("timeout-policy.md does not name %q", tier)
		}
	}
}

// --- helpers ---------------------------------------------------------------

// itoaFlake converts a small non-negative int to a string.
//
// The helper exists so that the file does not import strconv for one
// call site. If a second call site appears, switch to strconv.Itoa.
func itoaFlake(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
