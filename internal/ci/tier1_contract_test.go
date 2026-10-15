package ci_test

// tier1_contract_test.go — external contract tests for the Tier 1
// fast-feedback workflow.
//
// The tests in this file read .github/workflows/ci-tier1.yml,
// .github/workflows/ci.yml, .github/workflows/cache.yml, and
// .golangci.yml, and assert that the artifacts agree with the model
// in internal/ci/tier1.go. The file is package ci_test because it
// treats internal/ci as a black box and asserts on the artifacts.
//
// # Why the job-name needle is scoped to the `jobs:` block
//
// A workflow's job keys and its trigger keys share the same YAML
// shape: both are two-space-indented lines ending in a colon. A
// regexp anchored on that shape alone cannot tell them apart, and
// an extractor that scans the whole file would return `push`,
// `schedule`, and `workflow_dispatch` alongside the real job keys.
// The extractor used here is stateful: it ignores everything before
// the `jobs:` line, collects the keys indented exactly two spaces
// under it, and stops at the next top-level key.
//
// # Why the contract compares display names, not job keys
//
// The workflow's job keys are `format-check`, `vet`, `lint`,
// `test-short`, `build-host`. The strings the model uses to name
// the tier's checks are `tier1-format`, `tier1-vet`, `tier1-lint`,
// `tier1-test-short`, `tier1-build`. Those strings appear in the
// workflow as the value of each job's `name:` field:
//
//	format-check:
//	  name: tier1-format
//
// The two tests that pin the job set — the "every model job
// appears" test and its complement, the "no extra jobs" test —
// compare the model's names against the workflow's `name:` values.
// The workflow's job keys are an implementation detail of the YAML;
// a future rename of `format-check` to `fmt` would not change what
// the tier does, and the contract does not depend on it.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/thapelomagqazana/forge/internal/ci"
)

// tier1WorkflowPath returns the path to ci-tier1.yml.
func tier1WorkflowPath(t *testing.T, root string) string {
	t.Helper()
	return filepath.Join(root, ".github", "workflows", "ci-tier1.yml")
}

// readTier1Workflow reads ci-tier1.yml and returns its bytes.
func readTier1Workflow(t *testing.T, root string) []byte {
	t.Helper()
	path := tier1WorkflowPath(t, root)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

// tier1JobKeys returns the ordered list of job keys declared in
// ci-tier1.yml.
//
// The scan is scoped to the top-level `jobs:` block. It ignores
// everything before the `jobs:` line, collects the keys indented
// exactly two spaces under it, and stops at the next top-level key.
// A job key is a lowercase identifier ending in `:` with nothing
// else on the line.
func tier1JobKeys(t *testing.T, data []byte) []string {
	t.Helper()
	lines := strings.Split(string(data), "\n")

	inJobs := false
	var keys []string
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
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
		if indent != 2 {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if !strings.HasSuffix(trimmed, ":") {
			continue
		}
		name := strings.TrimSuffix(trimmed, ":")
		if !isLowercaseJobKey(name) {
			continue
		}
		keys = append(keys, name)
	}
	return keys
}

// tier1DisplayNames returns the ordered list of `name:` values
// declared under the `jobs:` block in ci-tier1.yml.
//
// A display name is the value of a job's `name:` field:
//
//	format-check:
//	  name: tier1-format
//
// The function is scoped to the `jobs:` block; it ignores everything
// before the `jobs:` line and stops at the next top-level key.
func tier1DisplayNames(t *testing.T, data []byte) []string {
	t.Helper()
	lines := strings.Split(string(data), "\n")

	inJobs := false
	var names []string
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
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
		// A display name is the value of a `name:` key. It is
		// indented four spaces under the job key.
		indent := 0
		for indent < len(line) && line[indent] == ' ' {
			indent++
		}
		if indent != 4 {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "name: ") {
			continue
		}
		name := strings.TrimSpace(strings.TrimPrefix(trimmed, "name: "))
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	return names
}

// tier1JobBody returns the body of the named job: everything from
// the job's key line to the next job key in the `jobs:` block, or
// to the end of the block.
func tier1JobBody(data []byte, jobName string) string {
	lines := strings.Split(string(data), "\n")

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

// TestTier1_WorkflowFileExists asserts ci-tier1.yml exists.
func TestTier1_WorkflowFileExists(t *testing.T) {
	root := repoRoot(t)
	path := tier1WorkflowPath(t, root)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("ci-tier1.yml is missing: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("ci-tier1.yml is empty")
	}
}

// TestTier1_EveryJobIsImplementedInTheWorkflow asserts every job in
// the model appears as a `name:` value in the workflow file.
//
// # Why the needle searches for `name:` and not for a job key
//
// The model's job names (`tier1-format`, `tier1-vet`, etc.) are the
// values of each job's `name:` field. They are not the job keys;
// the job keys are `format-check`, `vet`, `lint`, `test-short`,
// `build-host`. The test searches for the display-name form.
func TestTier1_EveryJobIsImplementedInTheWorkflow(t *testing.T) {
	root := repoRoot(t)
	data := readTier1Workflow(t, root)
	s := string(data)
	for _, j := range ci.Tier1Jobs() {
		needle := "    name: " + string(j.Name)
		if !strings.Contains(s, needle) {
			t.Errorf(
				"ci-tier1.yml does not declare a job with `name: %s`",
				string(j.Name),
			)
		}
	}
}

// TestTier1_NoExtraJobs asserts the workflow declares no job whose
// display name the model does not know about.
//
// The check is the complement of
// TestTier1_EveryJobIsImplementedInTheWorkflow: together they pin
// the display-name set to the model's job set. The scan is scoped to
// the `jobs:` block; the trigger names under `on:` are not jobs and
// are not considered.
//
// # Why the comparison uses display names, not job keys
//
// The model's `AllTier1Jobs()` returns the tier's check names
// (`tier1-format`, `tier1-vet`, etc.), which appear in the workflow
// as the values of each job's `name:` field. The workflow's job keys
// (`format-check`, `vet`, etc.) are an implementation detail of the
// YAML; they are not the model's identifiers. The test therefore
// compares display names, matching the convention used by
// TestTier1_EveryJobIsImplementedInTheWorkflow.
func TestTier1_NoExtraJobs(t *testing.T) {
	root := repoRoot(t)
	data := readTier1Workflow(t, root)
	known := make(map[string]struct{}, len(ci.AllTier1Jobs()))
	for _, j := range ci.AllTier1Jobs() {
		known[string(j)] = struct{}{}
	}
	for _, name := range tier1DisplayNames(t, data) {
		if _, ok := known[name]; !ok {
			t.Errorf(
				"ci-tier1.yml declares a job with `name: %s`, which the model does not list; "+
					"add it to the model or remove it from the workflow",
				name,
			)
		}
	}
}

// TestTier1_EveryActionIsPinnedToFullSHA asserts every `uses:` in
// ci-tier1.yml references a full 40-hex SHA.
//
// The check reuses the usesPattern from workflows_contract_test.go.
// It is defined there because both files are in package ci_test.
func TestTier1_EveryActionIsPinnedToFullSHA(t *testing.T) {
	root := repoRoot(t)
	path := tier1WorkflowPath(t, root)
	data := readTier1Workflow(t, root)
	for _, m := range usesPattern.FindAllStringSubmatch(string(data), -1) {
		action, ref := m[1], m[2]
		if !ci.IsPinnedSHA(ref) {
			t.Errorf(
				"%s: %s@%s is not a full 40-hex lowercase SHA",
				path, action, ref,
			)
		}
	}
}

// TestTier1_EveryActionIsInTheClosedSet asserts every `uses:` in
// ci-tier1.yml references an action in the model's PinnedActions.
func TestTier1_EveryActionIsInTheClosedSet(t *testing.T) {
	root := repoRoot(t)
	path := tier1WorkflowPath(t, root)
	data := readTier1Workflow(t, root)
	known := ci.PinnedActions()
	for _, m := range usesPattern.FindAllStringSubmatch(string(data), -1) {
		action, ref := m[1], m[2]
		pinned, ok := known[action]
		if !ok {
			t.Errorf("%s: action %s is not in the closed pinned set", path, action)
			continue
		}
		if pinned != ref {
			t.Errorf(
				"%s: action %s is pinned to %s but the model declares %s",
				path, action, ref, pinned,
			)
		}
	}
}

// TestTier1_NoPullRequestTarget asserts ci-tier1.yml does not use
// the pull_request_target event.
func TestTier1_NoPullRequestTarget(t *testing.T) {
	root := repoRoot(t)
	data := readTier1Workflow(t, root)
	if strings.Contains(string(data), "pull_request_target") {
		t.Fatalf(
			"%s references pull_request_target; this is forbidden by ADR-043",
			tier1WorkflowPath(t, root),
		)
	}
}

// TestTier1_EveryJobHasTimeout asserts every job declares a
// timeout-minutes.
//
// The scan is scoped to the `jobs:` block. For each job key, the
// job's body is extracted and asserted to contain `timeout-minutes:`.
func TestTier1_EveryJobHasTimeout(t *testing.T) {
	root := repoRoot(t)
	data := readTier1Workflow(t, root)
	for _, name := range tier1JobKeys(t, data) {
		body := tier1JobBody(data, name)
		if !strings.Contains(body, "timeout-minutes:") {
			t.Errorf("ci-tier1.yml: job %q does not declare timeout-minutes", name)
		}
	}
}

// TestTier1_EveryJobDeclaresPermissions asserts every job declares
// `permissions: contents: read`.
func TestTier1_EveryJobDeclaresPermissions(t *testing.T) {
	root := repoRoot(t)
	data := readTier1Workflow(t, root)
	for _, name := range tier1JobKeys(t, data) {
		body := tier1JobBody(data, name)
		if !strings.Contains(body, "contents: read") {
			t.Errorf("ci-tier1.yml: job %q does not declare 'contents: read'", name)
		}
	}
}

// TestTier1_CacheKeyTemplate asserts the workflow invokes the
// reusable cache workflow with a `tier1-` suffix.
//
// # Why the cache-key templates are not asserted here
//
// The workflow delegates caching to .github/workflows/cache.yml; the
// key templates (`${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}`,
// etc.) live there, not in this file. This test asserts the
// invocation and the suffix; the key templates themselves are
// asserted by cache_contract_test.go's TestCache_EveryKeyMatchesModel,
// which reads cache.yml and compares each key to the model in
// internal/ci/cache.go.
func TestTier1_CacheKeyTemplate(t *testing.T) {
	root := repoRoot(t)
	data := readTier1Workflow(t, root)
	s := string(data)

	if !strings.Contains(s, "uses: ./.github/workflows/cache.yml") {
		t.Error("ci-tier1.yml does not invoke ./.github/workflows/cache.yml")
	}
	if !strings.Contains(s, "cache-key-suffix: tier1-") {
		t.Error("ci-tier1.yml's cache invocation does not pass a `tier1-` suffix")
	}
}

// TestTier1_WorkflowLevelPermissions asserts the workflow file's
// top-level permissions block is `contents: read`.
func TestTier1_WorkflowLevelPermissions(t *testing.T) {
	root := repoRoot(t)
	data := readTier1Workflow(t, root)
	s := string(data)
	// The top-level permissions block appears before any job. A
	// simple check: the string `permissions:\n  contents: read`
	// appears before the string `jobs:`.
	permIdx := strings.Index(s, "permissions:\n  contents: read")
	jobsIdx := strings.Index(s, "\njobs:\n")
	if permIdx == -1 {
		t.Fatal("ci-tier1.yml does not have a top-level 'permissions: contents: read' block")
	}
	if jobsIdx != -1 && permIdx > jobsIdx {
		t.Fatal("ci-tier1.yml's permissions block appears after jobs:")
	}
}

// TestTier1_WorkflowLevelGoFlags asserts the workflow file's
// top-level env sets GOFLAGS=-mod=readonly.
func TestTier1_WorkflowLevelGoFlags(t *testing.T) {
	root := repoRoot(t)
	data := readTier1Workflow(t, root)
	s := string(data)
	if !strings.Contains(s, "GOFLAGS: -mod=readonly") {
		t.Error("ci-tier1.yml does not set GOFLAGS: -mod=readonly at the workflow level")
	}
}

// TestTier1_CIYmlRequiresTheAggregate asserts ci.yml calls the
// tier1 reusable workflow as a job named "tier1".
//
// The aggregate name is the branch-protection anchor: a required
// check named "tier1" corresponds to the aggregate. Requiring the
// five individual jobs would couple the branch-protection config to
// the tier model's job list, which changes more often.
func TestTier1_CIYmlRequiresTheAggregate(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, ".github", "workflows", "ci.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	s := string(data)
	if !strings.Contains(s, "uses: ./.github/workflows/ci-tier1.yml") {
		t.Error("ci.yml does not call ./.github/workflows/ci-tier1.yml")
	}
	if !strings.Contains(s, "tier1:") {
		t.Error("ci.yml does not declare a job named tier1")
	}
}

// TestTier1_GolangciConfigExists asserts .golangci.yml exists.
func TestTier1_GolangciConfigExists(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, ".golangci.yml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf(".golangci.yml is missing: %v", err)
	}
}

// TestTier1_GoldenMatchesModel asserts ci.Tier1Golden() matches the
// golden file.
func TestTier1_GoldenMatchesModel(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "internal", "ci", "testdata", "tier1.golden.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	want := ci.Tier1Golden()
	if string(data) != want {
		t.Fatalf(
			"%s does not match ci.Tier1Golden(); run `task ci:render-tier1` to regenerate",
			path,
		)
	}
}

// TestTier1_NoSecrets asserts ci-tier1.yml references no secrets.
//
// Tier 1 runs on untrusted PR code; it must never have access to
// repository secrets. This is a structural property of the
// pull_request event, but the test asserts the workflow file does
// not reference a secret.
//
// The check looks for the two forms a reference takes:
//
//   - `${{ secrets.NAME }}` — an expression that reads a secret.
//   - `secrets:` — a block header on a `uses:` job that passes a
//     secret to a reusable workflow.
//
// A comment that contains the word "secrets." in prose is not a
// reference. The check is therefore narrower than a bare
// `strings.Contains(s, "secrets.")`.
func TestTier1_NoSecrets(t *testing.T) {
	root := repoRoot(t)
	data := readTier1Workflow(t, root)
	s := string(data)

	for _, needle := range []string{"${{ secrets.", "secrets:"} {
		if strings.Contains(s, needle) {
			t.Fatalf(
				"ci-tier1.yml references a secret (found %q); Tier 1 must not use secrets",
				needle,
			)
		}
	}
}

// Compile-time guard: the regexp import is used by the tests that
// are retained from the original file (usesPattern is defined in
// workflows_contract_test.go; regexp is referenced by the tests in
// this file only if they declare a pattern). If a future change
// removes the last use, delete the import.
var _ = regexp.MustCompile
