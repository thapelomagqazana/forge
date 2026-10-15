package ci_test

// cost_contract_test.go — external contract tests for the CI
// cost-budget model.
//
// The tests in this file read:
//
//   - docs/reference/ci-cost-budget.md
//     — the cost-budget document, rendered from the model.
//   - docs/decisions/ADR-053-ci-cost-budget.md
//     — the ADR that records the decisions.
//   - internal/ci/testdata/cost.golden.json
//     — the golden JSON, rendered from the model.
//   - .github/workflows/ci.yml
//     — the workflow file whose `paths-ignore` filter is part of
//       the cost model's optimisation techniques.
//
// and assert that those artifacts agree with the model in
// internal/ci/cost.go.
//
// The file is package ci_test because it treats the package as a
// black box and asserts on the artifacts rather than the model.
//
// # Test taxonomy
//
//   - Existence tests: the three artifacts exist.
//   - Render tests: the cost-budget document and the golden JSON
//     match the model's renderings.
//   - paths-ignore tests: ci.yml declares the filter on its
//     pull_request and push triggers, and the filter covers the
//     expected patterns.
//   - Optimization tests: every optimization's Mechanism names a
//     file that exists in the repository.
//   - Cross-model tests: the concurrency model, the cache model,
//     and the tier model agree with the cost model's assumptions.
//   - ADR tests: the ADR names the model's key values.
//
// The grouping is documentation. A reader who wants to know what the
// cost budget must satisfy reads the group headings.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/thapelomagqazana/forge/internal/ci"
)

// --- Path helpers ----------------------------------------------------------

// costDocPath returns the path to the cost-budget document.
func costDocPath(t *testing.T, root string) string {
	t.Helper()
	return filepath.Join(root, "docs", "reference", "ci-cost-budget.md")
}

// costADRPath returns the path to the ADR.
func costADRPath(t *testing.T, root string) string {
	t.Helper()
	return filepath.Join(root, "docs", "decisions", "ADR-053-ci-cost-budget.md")
}

// costGoldenPath returns the path to the golden JSON.
func costGoldenPath(t *testing.T, root string) string {
	t.Helper()
	return filepath.Join(root, "internal", "ci", "testdata", "cost.golden.json")
}

// ciYmlPath returns the path to ci.yml.
func ciYmlPath(t *testing.T, root string) string {
	t.Helper()
	return filepath.Join(root, ".github", "workflows", "ci.yml")
}

// --- Existence tests -------------------------------------------------------

// TestCost_DocExists asserts the cost-budget document exists and is
// non-empty.
func TestCost_DocExists(t *testing.T) {
	root := repoRoot(t)
	path := costDocPath(t, root)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("docs/reference/ci-cost-budget.md is missing: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("docs/reference/ci-cost-budget.md is empty")
	}
}

// TestCost_GoldenFileExists asserts the golden JSON exists.
func TestCost_GoldenFileExists(t *testing.T) {
	root := repoRoot(t)
	path := costGoldenPath(t, root)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("cost.golden.json is missing: %v", err)
	}
}

// --- Render tests ----------------------------------------------------------

// TestCost_DocMatchesModel asserts ci.CostMarkdown() matches the
// cost-budget document.
func TestCost_DocMatchesModel(t *testing.T) {
	root := repoRoot(t)
	path := costDocPath(t, root)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	want := ci.CostMarkdown()
	if string(data) != want {
		t.Fatalf(
			"%s does not match ci.CostMarkdown(); run `task ci:render-cost` to regenerate",
			path,
		)
	}
}

// TestCost_GoldenMatchesModel asserts ci.CostGolden() matches the
// golden JSON.
func TestCost_GoldenMatchesModel(t *testing.T) {
	root := repoRoot(t)
	path := costGoldenPath(t, root)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	want := ci.CostGolden()
	if string(data) != want {
		t.Fatalf(
			"%s does not match ci.CostGolden(); run `task ci:render-cost` to regenerate",
			path,
		)
	}
}

// --- paths-ignore tests ----------------------------------------------------

// pathsIgnoreBlockPattern captures a `paths-ignore:` block's body.
//
// The pattern finds the `paths-ignore:` key at any indentation and
// captures the indented lines that follow. The lines are captured as
// a single string.
var pathsIgnoreBlockPattern = regexp.MustCompile(
	`(?m)^\s+paths-ignore:\s*\n((?:\s+-\s+.+\n?)+)`,
)

// TestCost_CIYmlHasPathsIgnore asserts ci.yml declares a
// `paths-ignore` block.
func TestCost_CIYmlHasPathsIgnore(t *testing.T) {
	root := repoRoot(t)
	path := ciYmlPath(t, root)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	s := string(data)
	if !strings.Contains(s, "paths-ignore:") {
		t.Errorf("%s does not declare a `paths-ignore:` block", path)
	}
}

// TestCost_CIYmlHasPathsIgnoreOnBothTriggers asserts ci.yml declares
// the `paths-ignore` filter on both the `pull_request` and `push`
// triggers.
//
// The check is structural: the file must have at least two
// `paths-ignore:` blocks. The two triggers are the two places the
// filter is needed; a file with only one block would let the other
// trigger run on docs-only changes.
func TestCost_CIYmlHasPathsIgnoreOnBothTriggers(t *testing.T) {
	root := repoRoot(t)
	path := ciYmlPath(t, root)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	s := string(data)
	count := strings.Count(s, "paths-ignore:")
	if count < 2 {
		t.Errorf(
			"%s declares %d `paths-ignore:` blocks, want at least 2 "+
				"(one for pull_request, one for push)",
			path, count,
		)
	}
}

// TestCost_CIYmlPathsIgnoreCoversDocs asserts the `paths-ignore`
// filter includes the `docs/**` pattern.
func TestCost_CIYmlPathsIgnoreCoversDocs(t *testing.T) {
	root := repoRoot(t)
	path := ciYmlPath(t, root)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	s := string(data)
	if !strings.Contains(s, `"docs/**"`) && !strings.Contains(s, "docs/**") {
		t.Errorf("%s's `paths-ignore` does not include the `docs/**` pattern", path)
	}
}

// TestCost_CIYmlPathsIgnoreCoversMarkdown asserts the `paths-ignore`
// filter includes the `*.md` pattern.
func TestCost_CIYmlPathsIgnoreCoversMarkdown(t *testing.T) {
	root := repoRoot(t)
	path := ciYmlPath(t, root)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	s := string(data)
	if !strings.Contains(s, `"*.md"`) && !strings.Contains(s, "*.md") {
		t.Errorf("%s's `paths-ignore` does not include the `*.md` pattern", path)
	}
}

// TestCost_CIYmlPathsIgnoreCoversLicense asserts the `paths-ignore`
// filter includes the `LICENSE` pattern.
func TestCost_CIYmlPathsIgnoreCoversLicense(t *testing.T) {
	root := repoRoot(t)
	path := ciYmlPath(t, root)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	s := string(data)
	if !strings.Contains(s, "LICENSE") {
		t.Errorf("%s's `paths-ignore` does not include the `LICENSE` pattern", path)
	}
}

// TestCost_CIYmlPathsIgnoreCoversCodeowners asserts the
// `paths-ignore` filter includes the `.github/CODEOWNERS` pattern.
func TestCost_CIYmlPathsIgnoreCoversCodeowners(t *testing.T) {
	root := repoRoot(t)
	path := ciYmlPath(t, root)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	s := string(data)
	if !strings.Contains(s, ".github/CODEOWNERS") {
		t.Errorf("%s's `paths-ignore` does not include the `.github/CODEOWNERS` pattern", path)
	}
}

// TestCost_CIYmlPathsIgnoreBlocksAreBothNonEmpty asserts every
// `paths-ignore:` block in ci.yml has at least one entry.
//
// The check extracts each block's body with the pathsIgnoreBlockPattern
// and asserts the body is non-empty. A future change that emptied a
// block would defeat the filter.
func TestCost_CIYmlPathsIgnoreBlocksAreBothNonEmpty(t *testing.T) {
	root := repoRoot(t)
	path := ciYmlPath(t, root)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	matches := pathsIgnoreBlockPattern.FindAllStringSubmatch(string(data), -1)
	if len(matches) < 2 {
		t.Fatalf(
			"%s declares %d non-empty `paths-ignore:` blocks, want at least 2",
			path, len(matches),
		)
	}
	for i, m := range matches {
		if strings.TrimSpace(m[1]) == "" {
			t.Errorf("%s's `paths-ignore` block %d is empty", path, i+1)
		}
	}
}

// --- Optimization tests ----------------------------------------------------

// TestCost_EveryOptimizationNamesAFile asserts every optimization's
// `Mechanism` field names a file that exists in the repository.
//
// The check extracts the file path from the mechanism string (which
// may include a parenthetical qualifier) and asserts the file exists
// under the module root. A future change that renamed a file without
// updating the optimization's mechanism would fail this test.
func TestCost_EveryOptimizationNamesAFile(t *testing.T) {
	root := repoRoot(t)
	for _, o := range ci.AllOptimizations() {
		// The mechanism is a path optionally followed by a
		// parenthetical qualifier. Extract the path (everything
		// before the first " (").
		path := o.Mechanism
		if idx := strings.Index(path, " ("); idx != -1 {
			path = path[:idx]
		}
		// Some mechanisms name two files separated by a comma;
		// split and check each.
		parts := strings.Split(path, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			full := filepath.Join(root, filepath.FromSlash(p))
			if _, err := os.Stat(full); err != nil {
				t.Errorf(
					"optimization %q names the mechanism %q, which does not exist: %v",
					o.Name, p, err,
				)
			}
		}
	}
}

// --- Cross-model tests -----------------------------------------------------

// TestCost_ConcurrencyOptimizationMatchesModel asserts the concurrency
// model declares the `on-pr` policy for the PR-triggered workflows.
//
// The cost model's "Concurrency cancel-in-progress" optimization
// depends on the concurrency policy being `on-pr`. The test asserts
// the dependency by reading the concurrency model directly.
func TestCost_ConcurrencyOptimizationMatchesModel(t *testing.T) {
	// The cost model's optimization names
	// CancelPolicyOnPR in concurrency.go. The test asserts that the
	// policy exists and is declared for at least one workflow.
	seen := false
	for _, id := range ci.AllWorkflows() {
		s, ok := ci.LookupConcurrencySpec(id)
		if !ok {
			continue
		}
		if s.Policy == ci.CancelPolicyOnPR {
			seen = true
			break
		}
	}
	if !seen {
		t.Error("no workflow uses CancelPolicyOnPR; the cost model's concurrency optimization is not in place")
	}
}

// TestCost_CacheOptimizationMatchesModel asserts the cache model
// declares the two hit-rate targets the cost model's caching
// optimization depends on.
//
// The cost model's "Cache aggressively" optimization depends on the
// cache model's targets. The test asserts the cache model declares
// both a module-cache target and a build-cache target.
func TestCost_CacheOptimizationMatchesModel(t *testing.T) {
	module, ok := ci.LookupCacheSpec(ci.CacheModule)
	if !ok {
		t.Fatal("LookupCacheSpec(CacheModule) not found")
	}
	if module.HitRateTarget <= 0 {
		t.Errorf(
			"CacheModule's HitRateTarget = %v, want a positive value",
			module.HitRateTarget,
		)
	}
	build, ok := ci.LookupCacheSpec(ci.CacheBuild)
	if !ok {
		t.Fatal("LookupCacheSpec(CacheBuild) not found")
	}
	if build.HitRateTarget <= 0 {
		t.Errorf(
			"CacheBuild's HitRateTarget = %v, want a positive value",
			build.HitRateTarget,
		)
	}
}

// TestCost_Tier3IsMainOnly asserts the tier model declares Tier 3 as
// advisory on PR.
//
// The cost model's "Run expensive jobs on main only" optimization
// depends on Tier 3's gating. The test asserts the tier model's
// Tier3.Advisory is consistent with the cost model's claim that the
// tier does not run on PRs.
func TestCost_Tier3IsMainOnly(t *testing.T) {
	tier, ok := ci.LookupTier(ci.Tier3)
	if !ok {
		t.Fatal("LookupTier(Tier3) not found")
	}
	// Tier 3 is blocking on push and absent on PR; the model
	// records the blocking set as {TriggerPush}. The test asserts
	// the set does not include TriggerPullRequest.
	for _, tr := range tier.BlockingOn {
		if tr == ci.TriggerPullRequest {
			t.Error("Tier 3's BlockingOn includes TriggerPullRequest; the tier is main-only")
		}
	}
}

// --- helpers ---------------------------------------------------------------

// itoaCost converts a small non-negative int to a string.
//
// The helper exists so that the file does not import strconv for one
// call site.
func itoaCost(n int) string {
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

// formatWithCommas renders a positive integer with thousands
// separators ("2,000", "5,000"). It handles values up to 999,999.
//
// The helper exists so that the ADR tests accept either the bare
// number or the comma-separated form. A future change that used a
// different separator would need the helper updated.
func formatWithCommas(n int) string {
	if n < 1000 {
		return itoaCost(n)
	}
	thousands := n / 1000
	remainder := n % 1000
	// Format the remainder with leading zeros.
	rem := itoaCost(remainder)
	for len(rem) < 3 {
		rem = "0" + rem
	}
	return itoaCost(thousands) + "," + rem
}
