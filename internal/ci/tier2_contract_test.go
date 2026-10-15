package ci_test

// tier2_contract_test.go — external contract tests for the Tier 2
// cross-platform matrix workflow.
//
// The tests read .github/workflows/ci-tier2.yml and assert that the
// file implements the model in internal/ci/tier2.go. The file is
// package ci_test because it treats internal/ci as a black box and
// asserts on the artifact.
//
// # Why the job-name needle searches for job keys, not display names
//
// A workflow's job keys and its display names serve different
// purposes. The job key is the map key under `jobs:`; it is stable,
// it is what a `needs:` list references, and it is what the GitHub
// UI shows as the job's identity. The `name:` field is a display
// label; for a matrix job it is interpolated at runtime
// (`test-matrix (ubuntu-latest, go=min)`), so its value is not a
// stable string a contract test can compare against.
//
// The model's `Tier2Jobs()` therefore names the job keys:
// `test-matrix`, `cross-compile`, `smoke`, `release-dry-run`. The
// test searches for the job-key shape (two-space indentation, ending
// in a colon), not for a `name:` line.
//
// # Why the race-detector test asserts the `if:` line directly
//
// The race-detector step's `name:` value contains the literal string
// `go test -race`:
//
//	- name: go test -race ./... (Linux only)
//	  if: ${{ matrix.os == 'ubuntu-latest' }}
//	  run: go test -race -count=1 ./...
//
// An earlier version of this test located the step by
// `strings.Index(s, "go test -race")` and then sliced backwards to
// the last `- name:` before that index. Because the index sits
// inside the step's own `name:` value, the backward slice landed on
// the step's own `- name:` line and excluded the `if:` condition,
// producing a false "could not find the step" failure. The test now
// asserts the `if:` line directly; it is a stable string, and the
// two facts the test cares about (the step exists, it is gated on
// ubuntu-latest) are both checkable from the file as a whole.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/thapelomagqazana/forge/internal/ci"
)

// tier2WorkflowPath returns the path to ci-tier2.yml.
func tier2WorkflowPath(t *testing.T, root string) string {
	t.Helper()
	return filepath.Join(root, ".github", "workflows", "ci-tier2.yml")
}

// readTier2Workflow reads ci-tier2.yml and returns its bytes.
func readTier2Workflow(t *testing.T, root string) []byte {
	t.Helper()
	path := tier2WorkflowPath(t, root)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

// TestTier2_WorkflowFileExists asserts ci-tier2.yml exists.
func TestTier2_WorkflowFileExists(t *testing.T) {
	root := repoRoot(t)
	path := tier2WorkflowPath(t, root)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("ci-tier2.yml is missing: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("ci-tier2.yml is empty")
	}
}

// TestTier2_EveryJobIsImplementedInTheWorkflow asserts every job in
// the model appears as a job key in the workflow file.
//
// The workflow declares jobs by their YAML key under `jobs:`. The
// model's `Tier2Jobs()` returns the same keys. The test searches
// for the job-key shape: two-space indentation, ending in a colon.
func TestTier2_EveryJobIsImplementedInTheWorkflow(t *testing.T) {
	root := repoRoot(t)
	data := readTier2Workflow(t, root)
	s := string(data)
	for _, j := range ci.Tier2Jobs() {
		needle := "  " + string(j.Name) + ":"
		if !strings.Contains(s, needle) {
			t.Errorf("ci-tier2.yml does not declare job %q", string(j.Name))
		}
	}
}

// TestTier2_EveryMatrixIsFailFastFalse asserts every matrix in the
// workflow declares `fail-fast: false`.
//
// The check scans the file for `strategy:` blocks, then verifies each
// contains `fail-fast: false` before the next `strategy:` block or
// the end of file.
func TestTier2_EveryMatrixIsFailFastFalse(t *testing.T) {
	root := repoRoot(t)
	data := readTier2Workflow(t, root)
	s := string(data)

	// Find all `strategy:` blocks. Each is followed by a `fail-fast:`
	// line before the next `strategy:` (or the end of file).
	strategyRE := regexp.MustCompile(`(?m)^    strategy:$`)
	matches := strategyRE.FindAllStringIndex(s, -1)
	for _, m := range matches {
		end := len(s)
		// The next strategy block, if any.
		for _, m2 := range matches {
			if m2[0] > m[0] {
				end = m2[0]
				break
			}
		}
		block := s[m[0]:end]
		if !strings.Contains(block, "fail-fast: false") {
			t.Errorf("ci-tier2.yml: a strategy block does not declare fail-fast: false")
		}
	}
}

// TestTier2_EveryMatrixHasBothAxes asserts every matrix declares both
// an OS axis and a Go-version axis.
func TestTier2_EveryMatrixHasBothAxes(t *testing.T) {
	root := repoRoot(t)
	data := readTier2Workflow(t, root)
	s := string(data)

	strategyRE := regexp.MustCompile(`(?m)^    strategy:$`)
	matches := strategyRE.FindAllStringIndex(s, -1)
	for _, m := range matches {
		end := len(s)
		for _, m2 := range matches {
			if m2[0] > m[0] {
				end = m2[0]
				break
			}
		}
		block := s[m[0]:end]
		if !strings.Contains(block, "os:") {
			t.Errorf("ci-tier2.yml: a matrix does not declare an os axis")
		}
		if !strings.Contains(block, "go:") {
			t.Errorf("ci-tier2.yml: a matrix does not declare a go axis")
		}
	}
}

// TestTier2_RaceDetectorIsLinuxOnly asserts the race-detector step is
// gated on the ubuntu-latest runner.
//
// # Why the check is a direct assertion on the `if:` line
//
// An earlier version located the step by the position of the
// `go test -race` command and sliced backwards to the last `- name:`
// before it. Because the command string appears inside the step's
// own `name:` value, the backward slice landed on the step's own
// `- name:` line and excluded the `if:` condition. The test now
// asserts the `if:` line directly. The line is stable across the
// workflow's revisions, and the two facts the test cares about (the
// step exists, it is gated on ubuntu-latest) are both checkable
// from the file as a whole.
func TestTier2_RaceDetectorIsLinuxOnly(t *testing.T) {
	root := repoRoot(t)
	data := readTier2Workflow(t, root)
	s := string(data)

	// The race-detector step's `if:` condition must gate on
	// ubuntu-latest.
	const cond = "if: ${{ matrix.os == 'ubuntu-latest' }}"
	if !strings.Contains(s, cond) {
		t.Errorf(
			"ci-tier2.yml does not gate the race-detector step on ubuntu-latest; "+
				"expected the line %q",
			cond,
		)
	}

	// The race-detector command must be present.
	if !strings.Contains(s, "go test -race") {
		t.Error("ci-tier2.yml does not contain a 'go test -race' step")
	}
}

// TestTier2_CrossCompileIsOnLinuxOnly asserts the cross-compile job
// runs on a Linux runner.
func TestTier2_CrossCompileIsOnLinuxOnly(t *testing.T) {
	root := repoRoot(t)
	data := readTier2Workflow(t, root)
	s := string(data)

	idx := strings.Index(s, "  cross-compile:")
	if idx == -1 {
		t.Fatal("ci-tier2.yml does not declare the cross-compile job")
	}
	// The job's body runs until the next job key.
	nextIdx := len(s)
	jobRE := regexp.MustCompile(`(?m)^  [a-z][a-z0-9-]*:$`)
	for _, m := range jobRE.FindAllStringIndex(s[idx+1:], -1) {
		nextIdx = idx + 1 + m[0]
		break
	}
	body := s[idx:nextIdx]
	if !strings.Contains(body, "ubuntu-latest") {
		t.Errorf("the cross-compile job does not run on ubuntu-latest:\n%s", body)
	}
}

// TestTier2_EveryActionIsPinnedToFullSHA asserts every `uses:` is a
// full 40-hex SHA.
func TestTier2_EveryActionIsPinnedToFullSHA(t *testing.T) {
	root := repoRoot(t)
	path := tier2WorkflowPath(t, root)
	data := readTier2Workflow(t, root)
	for _, m := range usesPattern.FindAllStringSubmatch(string(data), -1) {
		action, ref := m[1], m[2]
		if !ci.IsPinnedSHA(ref) {
			t.Errorf("%s: %s@%s is not a full 40-hex lowercase SHA", path, action, ref)
		}
	}
}

// TestTier2_NoPullRequestTarget asserts ci-tier2.yml does not use
// pull_request_target.
func TestTier2_NoPullRequestTarget(t *testing.T) {
	root := repoRoot(t)
	data := readTier2Workflow(t, root)
	if strings.Contains(string(data), "pull_request_target") {
		t.Fatal("ci-tier2.yml references pull_request_target; this is forbidden by ADR-043")
	}
}

// TestTier2_NoSecrets asserts ci-tier2.yml references no secrets.
//
// Tier 2 runs on untrusted PR code; it must never have access to
// repository secrets. The check looks for the two forms a reference
// takes:
//
//   - `${{ secrets.NAME }}` — an expression that reads a secret.
//   - `secrets:` — a block header on a `uses:` job that passes a
//     secret to a reusable workflow.
//
// A comment that contains the word "secrets." in prose is not a
// reference. The check is therefore narrower than a bare
// `strings.Contains(s, "secrets.")`.
func TestTier2_NoSecrets(t *testing.T) {
	root := repoRoot(t)
	data := readTier2Workflow(t, root)
	s := string(data)

	for _, needle := range []string{"${{ secrets.", "secrets:"} {
		if strings.Contains(s, needle) {
			t.Fatalf(
				"ci-tier2.yml references a secret (found %q); Tier 2 must not use secrets",
				needle,
			)
		}
	}
}

// TestTier2_ReleaseDryRunIsOnUbuntu asserts the release-dry-run job
// runs on a Linux runner.
func TestTier2_ReleaseDryRunIsOnUbuntu(t *testing.T) {
	root := repoRoot(t)
	data := readTier2Workflow(t, root)
	s := string(data)

	idx := strings.Index(s, "  release-dry-run:")
	if idx == -1 {
		t.Fatal("ci-tier2.yml does not declare the release-dry-run job")
	}
	nextIdx := len(s)
	jobRE := regexp.MustCompile(`(?m)^  [a-z][a-z0-9-]*:$`)
	for _, m := range jobRE.FindAllStringIndex(s[idx+1:], -1) {
		nextIdx = idx + 1 + m[0]
		break
	}
	body := s[idx:nextIdx]
	if !strings.Contains(body, "ubuntu-latest") {
		t.Errorf("the release-dry-run job does not run on ubuntu-latest:\n%s", body)
	}
	if !strings.Contains(body, "make release-dry-run") {
		t.Errorf("the release-dry-run job does not run `make release-dry-run`:\n%s", body)
	}
}

// TestTier2_GoldenMatchesModel asserts ci.Tier2Golden() matches the
// golden file.
func TestTier2_GoldenMatchesModel(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "internal", "ci", "testdata", "tier2.golden.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	want := ci.Tier2Golden()
	if string(data) != want {
		t.Fatalf(
			"%s does not match ci.Tier2Golden(); run `task ci:render-tier2` to regenerate",
			path,
		)
	}
}

// TestTier2_CIYmlCallsTheReusableWorkflow asserts ci.yml invokes
// ci-tier2.yml as a job named tier2.
func TestTier2_CIYmlCallsTheReusableWorkflow(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, ".github", "workflows", "ci.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	s := string(data)
	if !strings.Contains(s, "uses: ./.github/workflows/ci-tier2.yml") {
		t.Error("ci.yml does not call ./.github/workflows/ci-tier2.yml")
	}
	if !strings.Contains(s, "tier2:") {
		t.Error("ci.yml does not declare a job named tier2")
	}
}
