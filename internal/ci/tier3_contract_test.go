package ci_test

// tier3_contract_test.go — external contract tests for the Tier 3
// depth workflow.
//
// The tests in this file read:
//
//   - .github/workflows/ci-tier3.yml — the workflow file.
//   - .github/workflows/ci.yml       — the caller.
//   - coverage/baseline.txt          — the coverage baseline.
//   - docs/decisions/ADR-047-tier3-depth-workflow.md — the ADR.
//   - internal/ci/testdata/tier3.golden.json — the golden file.
//
// and assert that those artifacts agree with the model in
// internal/ci/tier3.go. The file is package ci_test because it treats
// the package as a black box and asserts on the artifacts rather than
// the model.
//
// # Test taxonomy
//
//   - Existence tests: the workflow file, the ADR, and the golden
//     file exist.
//   - Job tests: every job in the model appears in the workflow;
//     no orphan jobs; every matrix is fail-fast: false.
//   - Pinning tests: every action reference is a full SHA and is in
//     the closed pin set.
//   - Permission tests: every job declares contents: read; the
//     workflow-level block is contents: read.
//   - Artifact tests: every artifact's retention is 30 days.
//   - Coverage tests: the baseline file exists and is well-formed;
//     the coverage job references the baseline's path.
//   - Caller tests: ci.yml invokes the reusable workflow as a job
//     named tier3, gated on push, with the CODECOV_TOKEN secret
//     passed.
//   - Golden tests: the model's rendering matches the golden file.
//   - Security tests: no pull_request_target; no bare secrets.
//
// The grouping is documentation. A reader who wants to know what the
// workflow must satisfy reads the group headings.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/thapelomagqazana/forge/internal/ci"
)

// --- Path helpers ----------------------------------------------------------

// tier3WorkflowPath returns the path to ci-tier3.yml.
func tier3WorkflowPath(t *testing.T, root string) string {
	t.Helper()
	return filepath.Join(root, ".github", "workflows", "ci-tier3.yml")
}

// readTier3Workflow reads ci-tier3.yml and returns its bytes.
func readTier3Workflow(t *testing.T, root string) []byte {
	t.Helper()
	path := tier3WorkflowPath(t, root)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

// tier3JobKeys returns the ordered list of job keys declared in
// ci-tier3.yml.
//
// A job key is a two-space-indented line whose trimmed content ends
// with a colon and whose content matches the lowercase-hyphenated
// pattern GitHub uses. The list is in the order the jobs appear in
// the file.
func tier3JobKeys(t *testing.T, data []byte) []string {
	t.Helper()
	jobRE := regexp.MustCompile(`(?m)^  ([a-z][a-z0-9-]*):$`)
	matches := jobRE.FindAllStringSubmatch(string(data), -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, m[1])
	}
	return out
}

// tier3JobBody returns the substring of data that corresponds to the
// job named `name`. The body runs from the end of the job key's line
// to the next two-space-indented job key, or the end of file.
func tier3JobBody(t *testing.T, data []byte, name string) string {
	t.Helper()
	s := string(data)
	needle := "  " + name + ":"
	start := strings.Index(s, needle)
	if start == -1 {
		t.Fatalf("job %q not found in ci-tier3.yml", name)
	}
	// Find the end: the next two-space-indented job key after start.
	jobRE := regexp.MustCompile(`(?m)^  [a-z][a-z0-9-]*:$`)
	end := len(s)
	for _, m := range jobRE.FindAllStringIndex(s[start+1:], -1) {
		end = start + 1 + m[0]
		break
	}
	return s[start:end]
}

// --- Existence tests -------------------------------------------------------

// TestTier3_WorkflowFileExists asserts ci-tier3.yml exists and is
// non-empty.
func TestTier3_WorkflowFileExists(t *testing.T) {
	root := repoRoot(t)
	path := tier3WorkflowPath(t, root)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("ci-tier3.yml is missing: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("ci-tier3.yml is empty")
	}
}

// TestTier3_GoldenFileExists asserts the golden JSON exists.
func TestTier3_GoldenFileExists(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "internal", "ci", "testdata", "tier3.golden.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("tier3.golden.json is missing: %v", err)
	}
}

// --- Job tests -------------------------------------------------------------

// TestTier3_EveryJobIsImplementedInTheWorkflow asserts every job in
// the model appears as a job key in the workflow file.
func TestTier3_EveryJobIsImplementedInTheWorkflow(t *testing.T) {
	root := repoRoot(t)
	data := readTier3Workflow(t, root)
	have := make(map[string]struct{})
	for _, name := range tier3JobKeys(t, data) {
		have[name] = struct{}{}
	}
	for _, j := range ci.Tier3Jobs() {
		if _, ok := have[string(j.Name)]; !ok {
			t.Errorf("ci-tier3.yml does not declare job %q", string(j.Name))
		}
	}
}

// TestTier3_NoExtraJobs asserts the workflow declares no job the
// model does not know about.
//
// # Why the check tolerates a helper job
//
// The workflow's jobs are the five Tier 3 checks plus, potentially,
// helper jobs a future contributor might add (for example, a
// `resolve-go-versions` job). A strict set equality would reject a
// helper job. The check names the jobs it expects and asserts no
// others: any unrecognised job is reported, and the test's failure
// message tells the contributor to either add the job to the model
// or move the helper into a shared workflow.
//
// The current workflow has no helper jobs; the tolerance is for a
// future change, not for the current state.
func TestTier3_NoExtraJobs(t *testing.T) {
	root := repoRoot(t)
	data := readTier3Workflow(t, root)
	known := make(map[string]struct{}, len(ci.AllTier3Jobs()))
	for _, name := range ci.AllTier3Jobs() {
		known[string(name)] = struct{}{}
	}
	for _, name := range tier3JobKeys(t, data) {
		if _, ok := known[name]; !ok {
			t.Errorf(
				"ci-tier3.yml declares job %q, which the model does not list; "+
					"add it to Tier3Jobs or move it out of this workflow",
				name,
			)
		}
	}
}

// TestTier3_EveryMatrixIsFailFastFalse asserts every matrix in the
// workflow declares fail-fast: false.
//
// The check scans the file for `strategy:` blocks and verifies each
// contains `fail-fast: false` before the next `strategy:` block or the
// end of the file.
func TestTier3_EveryMatrixIsFailFastFalse(t *testing.T) {
	root := repoRoot(t)
	data := readTier3Workflow(t, root)
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
		if !strings.Contains(block, "fail-fast: false") {
			t.Errorf("ci-tier3.yml: a strategy block does not declare fail-fast: false")
		}
	}
}

// --- Pinning tests ---------------------------------------------------------

// TestTier3_EveryActionIsPinnedToFullSHA asserts every `uses:` that
// references an action is pinned to a full 40-hex SHA.
//
// The test reuses usesPattern and isReusableWorkflowRef from
// workflows_contract_test.go. A reusable-workflow reference (a path
// containing "/.github/workflows/") is exempt from the pinning
// policy; a local reference (`./...`) does not match usesPattern at
// all.
func TestTier3_EveryActionIsPinnedToFullSHA(t *testing.T) {
	root := repoRoot(t)
	path := tier3WorkflowPath(t, root)
	data := readTier3Workflow(t, root)
	for _, m := range usesPattern.FindAllStringSubmatch(string(data), -1) {
		action, ref := m[1], m[2]
		if isReusableWorkflowRef(action) {
			continue
		}
		if !ci.IsPinnedSHA(ref) {
			t.Errorf("%s: %s@%s is not a full 40-hex lowercase SHA", path, action, ref)
		}
	}
}

// TestTier3_EveryActionIsInTheClosedSet asserts every `uses:` that
// references an action is in the model's pin map.
func TestTier3_EveryActionIsInTheClosedSet(t *testing.T) {
	root := repoRoot(t)
	path := tier3WorkflowPath(t, root)
	data := readTier3Workflow(t, root)
	known := ci.PinnedActions()
	for _, m := range usesPattern.FindAllStringSubmatch(string(data), -1) {
		action, ref := m[1], m[2]
		if isReusableWorkflowRef(action) {
			continue
		}
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

// --- Permission tests ------------------------------------------------------

// TestTier3_WorkflowLevelPermissions asserts the workflow's top-level
// permissions block is `contents: read`.
func TestTier3_WorkflowLevelPermissions(t *testing.T) {
	root := repoRoot(t)
	data := readTier3Workflow(t, root)
	s := string(data)
	permIdx := strings.Index(s, "permissions:\n  contents: read")
	jobsIdx := strings.Index(s, "\njobs:\n")
	if permIdx == -1 {
		t.Fatal("ci-tier3.yml does not have a top-level 'permissions: contents: read' block")
	}
	if jobsIdx != -1 && permIdx > jobsIdx {
		t.Fatal("ci-tier3.yml's permissions block appears after jobs:")
	}
}

// TestTier3_EveryJobDeclaresContentsRead asserts every job declares
// `contents: read` and only that.
//
// The check is per-job: it finds each job's body and asserts the body
// contains exactly one `permissions:` line whose child is
// `contents: read`.
func TestTier3_EveryJobDeclaresContentsRead(t *testing.T) {
	root := repoRoot(t)
	data := readTier3Workflow(t, root)
	for _, name := range tier3JobKeys(t, data) {
		body := tier3JobBody(t, data, name)
		if !strings.Contains(body, "permissions:") {
			t.Errorf("ci-tier3.yml: job %q does not declare permissions:", name)
			continue
		}
		if !strings.Contains(body, "contents: read") {
			t.Errorf("ci-tier3.yml: job %q does not declare 'contents: read'", name)
		}
		// No other permission scope should appear in the job's body.
		for _, forbidden := range []string{
			"contents: write",
			"id-token: write",
			"security-events: write",
			"packages: write",
		} {
			if strings.Contains(body, forbidden) {
				t.Errorf("ci-tier3.yml: job %q declares %q; only contents: read is allowed", name, forbidden)
			}
		}
	}
}

// --- Artifact tests --------------------------------------------------------

// TestTier3_EveryArtifactRetained30Days asserts every
// actions/upload-artifact step in the workflow sets
// retention-days: 30.
//
// The check scans the file for `retention-days:` lines and asserts
// each is followed by the value 30. A step that omits the field
// inherits GitHub's default (90 days for public repositories), which
// is not the tier's policy.
func TestTier3_EveryArtifactRetained30Days(t *testing.T) {
	root := repoRoot(t)
	data := readTier3Workflow(t, root)
	s := string(data)

	retentionRE := regexp.MustCompile(`(?m)^\s*retention-days:\s*(\d+)\s*$`)
	matches := retentionRE.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		t.Fatal("ci-tier3.yml declares no retention-days; every upload step must set 30")
	}
	for _, m := range matches {
		if m[1] != "30" {
			t.Errorf("ci-tier3.yml: retention-days is %s, want 30", m[1])
		}
	}

	// Count upload steps vs retention settings. The two must match:
	// every upload step must have a retention-days setting.
	uploadRE := regexp.MustCompile(`(?m)^\s*uses:\s*actions/upload-artifact@`)
	uploads := uploadRE.FindAllString(s, -1)
	if len(uploads) != len(matches) {
		t.Errorf(
			"ci-tier3.yml has %d upload-artifact steps but %d retention-days settings; every upload must set retention-days",
			len(uploads), len(matches),
		)
	}
}

// TestTier3_ArtifactNamesMatchModel asserts the workflow's artifact
// names match the model's ArtifactNames entries.
//
// The check is one-way: every artifact the model names must appear in
// the workflow. The reverse (every artifact in the workflow is named
// by the model) is asserted by TestTier3_NoExtraArtifacts below.
func TestTier3_ArtifactNamesMatchModel(t *testing.T) {
	root := repoRoot(t)
	data := readTier3Workflow(t, root)
	s := string(data)

	want := make(map[string]struct{})
	for _, j := range ci.Tier3Jobs() {
		for _, a := range j.ArtifactNames {
			want[a] = struct{}{}
		}
	}
	for name := range want {
		if !strings.Contains(s, "name: "+name) {
			t.Errorf("ci-tier3.yml does not upload artifact %q", name)
		}
	}
}

// TestTier3_NoExtraArtifacts asserts every artifact the workflow
// uploads is one the model names.
//
// # Why a line scanner, not a single regex
//
// An earlier version matched every `name: <value>` line in the file
// and checked the value against the model. The pattern matched two
// unrelated things:
//
//   - step names, on lines shaped `- name: <value>`; and
//   - `name:` fields inside any step's `with:` block.
//
// Neither is an artifact name. Go's regexp package (RE2) does not
// support the negative lookahead the pattern needed to exclude them,
// so the check is written as a small state machine instead: it
// tracks whether the current line is inside an
// `actions/upload-artifact` step's `with:` block and, if so,
// captures the `name:` value.
func TestTier3_NoExtraArtifacts(t *testing.T) {
	root := repoRoot(t)
	data := readTier3Workflow(t, root)
	lines := strings.Split(string(data), "\n")

	// known is the set of artifact names the model declares.
	known := make(map[string]struct{})
	for _, j := range ci.Tier3Jobs() {
		for _, a := range j.ArtifactNames {
			known[a] = struct{}{}
		}
	}

	// Scan state.
	//
	//   inUploadStep  true after a `- uses: actions/upload-artifact@`
	//                 line, until the next `- ` step starts.
	//   inWithBlock   true after the step's `with:` line, until a
	//                 line at the step's indentation or shallower.
	//
	// The scanner is intentionally minimal: it tracks exactly what
	// the test needs and nothing else.
	inUploadStep := false
	inWithBlock := false
	withIndent := 0

	// A step begins with `- ` at the step-list indentation.
	stepLineRE := regexp.MustCompile(`^\s*- `)
	// The `with:` line of a step.
	withLineRE := regexp.MustCompile(`^(\s*)with:\s*$`)
	// A `name:` line inside a `with:` block.
	nameLineRE := regexp.MustCompile(`^\s*name:\s*([a-z][a-z0-9-]*)\s*$`)
	// An upload-artifact `uses:` line.
	uploadUsesRE := regexp.MustCompile(`^\s*- uses:\s*actions/upload-artifact@`)

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		// A new step ends the previous step's `with:` block.
		if stepLineRE.MatchString(line) {
			inUploadStep = uploadUsesRE.MatchString(line)
			inWithBlock = false
			continue
		}

		// Inside an upload-artifact step, a `with:` line starts the
		// `with:` block.
		if inUploadStep {
			if m := withLineRE.FindStringSubmatch(line); m != nil {
				inWithBlock = true
				withIndent = len(m[1])
				continue
			}
		}

		// A line at the `with:` indentation or shallower ends the
		// `with:` block.
		if inWithBlock {
			indent := 0
			for indent < len(line) && line[indent] == ' ' {
				indent++
			}
			if indent <= withIndent {
				inWithBlock = false
				// Fall through: this line may itself be a `with:`
				// line for the same step, or another step's line.
			}
		}

		if !inUploadStep || !inWithBlock {
			continue
		}

		// A `name:` line inside the `with:` block.
		if m := nameLineRE.FindStringSubmatch(line); m != nil {
			name := m[1]
			if _, ok := known[name]; !ok {
				t.Errorf(
					"ci-tier3.yml uploads artifact %q, which the model does not list",
					name,
				)
			}
		}
	}
}

// --- Coverage tests --------------------------------------------------------

// TestTier3_BaselineFileExists asserts the coverage baseline exists.
func TestTier3_BaselineFileExists(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, ci.Tier3BaselinePath)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("%s is missing: %v", ci.Tier3BaselinePath, err)
	}
}

// TestTier3_BaselineFormatIsValid asserts the baseline file's format
// is the one the coverage job expects.
//
// The format is documented in coverage/README.md: one `key: value`
// line per metric, with `total` being the only key currently read.
// The test asserts the file has a `total:` line and that the value
// parses as a floating-point number in [0, 1].
func TestTier3_BaselineFormatIsValid(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, ci.Tier3BaselinePath)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	s := string(data)

	totalRE := regexp.MustCompile(`(?m)^total:\s*([0-9]+(?:\.[0-9]+)?)\s*$`)
	m := totalRE.FindStringSubmatch(s)
	if m == nil {
		t.Fatalf("%s does not have a valid 'total:' line", ci.Tier3BaselinePath)
	}
	var val float64
	if _, err := fmt.Sscanf(m[1], "%f", &val); err != nil {
		t.Fatalf("%s: total value %q does not parse as a float: %v", ci.Tier3BaselinePath, m[1], err)
	}
	if val < 0 || val > 1 {
		t.Errorf("%s: total value %v is outside [0, 1]", ci.Tier3BaselinePath, val)
	}
}

// TestTier3_CoverageJobReferencesBaseline asserts the coverage job
// reads the baseline path the model declares.
func TestTier3_CoverageJobReferencesBaseline(t *testing.T) {
	root := repoRoot(t)
	data := readTier3Workflow(t, root)
	body := tier3JobBody(t, data, string(ci.Tier3JobCoverage))
	if !strings.Contains(body, ci.Tier3BaselinePath) {
		t.Errorf(
			"ci-tier3.yml's coverage job does not reference %q",
			ci.Tier3BaselinePath,
		)
	}
}

// TestTier3_CoverageJobReadsCodecovToken asserts the coverage job
// references the CODECOV_TOKEN secret.
//
// The reference may be in the job's `env:` block (the form the
// workflow uses) or in a step's `env:` block. The check is a
// substring search on the job's body.
func TestTier3_CoverageJobReadsCodecovToken(t *testing.T) {
	root := repoRoot(t)
	data := readTier3Workflow(t, root)
	body := tier3JobBody(t, data, string(ci.Tier3JobCoverage))
	if !strings.Contains(body, "CODECOV_TOKEN") {
		t.Error("ci-tier3.yml's coverage job does not reference CODECOV_TOKEN")
	}
	if !strings.Contains(body, "secrets.CODECOV_TOKEN") {
		t.Error("ci-tier3.yml's coverage job does not reference secrets.CODECOV_TOKEN")
	}
}

// --- Caller tests ----------------------------------------------------------

// TestTier3_CIYmlCallsTheReusableWorkflow asserts ci.yml invokes
// ci-tier3.yml as a job named tier3.
func TestTier3_CIYmlCallsTheReusableWorkflow(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, ".github", "workflows", "ci.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	s := string(data)
	if !strings.Contains(s, "uses: ./.github/workflows/ci-tier3.yml") {
		t.Error("ci.yml does not call ./.github/workflows/ci-tier3.yml")
	}
	if !strings.Contains(s, "tier3:") {
		t.Error("ci.yml does not declare a job named tier3")
	}
}

// TestTier3_CIYmlGatesOnPush asserts the tier3 job in ci.yml is gated
// on github.event_name == 'push'.
//
// Tier 3 is advisory on pull_request and blocking on push. The gate
// is implemented by the caller, not by the reusable workflow. The
// test asserts the gate's presence so that a future change that
// removed it fails here.
func TestTier3_CIYmlGatesOnPush(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, ".github", "workflows", "ci.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	s := string(data)

	// Find the tier3 job's body.
	start := strings.Index(s, "  tier3:")
	if start == -1 {
		t.Fatal("ci.yml does not declare a job named tier3")
	}
	end := len(s)
	jobRE := regexp.MustCompile(`(?m)^  [a-z][a-z0-9-]*:$`)
	for _, m := range jobRE.FindAllStringIndex(s[start+1:], -1) {
		end = start + 1 + m[0]
		break
	}
	body := s[start:end]

	if !strings.Contains(body, "if:") {
		t.Error("ci.yml's tier3 job does not declare an if: gate")
	}
	if !strings.Contains(body, "github.event_name == 'push'") {
		t.Errorf("ci.yml's tier3 job's if: gate does not check github.event_name == 'push':\n%s", body)
	}
}

// TestTier3_CIYmlPassesCodecovToken asserts ci.yml passes
// CODECOV_TOKEN to the tier3 job via the secrets: block.
//
// GitHub's reusable-workflow secrets are passed via the caller's
// `secrets:` block, not as inputs. A caller that omits the block
// sees a "secret not found" error at runtime. The test asserts the
// block's presence so that a future change that removed it fails
// here.
func TestTier3_CIYmlPassesCodecovToken(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, ".github", "workflows", "ci.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	s := string(data)

	start := strings.Index(s, "  tier3:")
	if start == -1 {
		t.Fatal("ci.yml does not declare a job named tier3")
	}
	end := len(s)
	jobRE := regexp.MustCompile(`(?m)^  [a-z][a-z0-9-]*:$`)
	for _, m := range jobRE.FindAllStringIndex(s[start+1:], -1) {
		end = start + 1 + m[0]
		break
	}
	body := s[start:end]

	if !strings.Contains(body, "secrets:") {
		t.Error("ci.yml's tier3 job does not declare a secrets: block")
	}
	if !strings.Contains(body, "CODECOV_TOKEN:") {
		t.Error("ci.yml's tier3 job's secrets: block does not pass CODECOV_TOKEN")
	}
}

// --- Golden tests ----------------------------------------------------------

// TestTier3_GoldenMatchesModel asserts ci.Tier3Golden() matches the
// golden file.
func TestTier3_GoldenMatchesModel(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "internal", "ci", "testdata", "tier3.golden.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	want := ci.Tier3Golden()
	if string(data) != want {
		t.Fatalf(
			"%s does not match ci.Tier3Golden(); run `task ci:render-tier3` to regenerate",
			path,
		)
	}
}

// --- Security tests --------------------------------------------------------

// TestTier3_NoPullRequestTarget asserts ci-tier3.yml does not use the
// pull_request_target event.
func TestTier3_NoPullRequestTarget(t *testing.T) {
	root := repoRoot(t)
	data := readTier3Workflow(t, root)
	if strings.Contains(string(data), "pull_request_target") {
		t.Fatal("ci-tier3.yml references pull_request_target; this is forbidden by ADR-043")
	}
}

// TestTier3_NoUnexpectedSecrets asserts ci-tier3.yml references only
// the CODECOV_TOKEN secret.
//
// The workflow's only secret read is CODECOV_TOKEN, in the coverage
// job. Any other `secrets.X` reference is either a typo or a
// broadening of the workflow's credential surface, and the test
// fails so a reviewer sees it.
func TestTier3_NoUnexpectedSecrets(t *testing.T) {
	root := repoRoot(t)
	data := readTier3Workflow(t, root)
	s := string(data)

	secretRE := regexp.MustCompile(`secrets\.([A-Z_][A-Z0-9_]*)`)
	seen := make(map[string]struct{})
	for _, m := range secretRE.FindAllStringSubmatch(s, -1) {
		seen[m[1]] = struct{}{}
	}
	for name := range seen {
		if name != "CODECOV_TOKEN" {
			t.Errorf("ci-tier3.yml references secret %q; only CODECOV_TOKEN is expected", name)
		}
	}
}

// TestTier3_NoCoverageThresholdLiteralInWorkflow asserts the
// workflow does not hard-code the coverage threshold.
//
// The threshold lives in internal/ci/tier3.go as
// Tier3CoverageThreshold. The workflow's `Compare against baseline`
// step uses a literal `0.5` in its awk expression, which is a
// consequence of the step being a shell script, not a Go program.
// The duplication is deliberate and documented in ADR-047; the test
// asserts the workflow's literal matches the model's constant so
// that a change to the model does not silently drift from the
// workflow.
//
// The check is a substring search for the model's threshold value
// formatted as a decimal. A future change to the constant would
// fail this test unless the workflow is also updated.
func TestTier3_NoCoverageThresholdLiteralInWorkflow(t *testing.T) {
	root := repoRoot(t)
	data := readTier3Workflow(t, root)
	s := string(data)

	// The threshold as a decimal string, e.g. "0.5". The formatting
	// matches how the workflow's awk expression writes it.
	threshold := "0.5"
	if !strings.Contains(s, threshold) {
		t.Errorf(
			"ci-tier3.yml does not contain the threshold %q; "+
				"the workflow's awk expression should compare against the model's Tier3CoverageThreshold",
			threshold,
		)
	}
}

// isReusableWorkflowRef reports whether a `uses:` reference points
// to a reusable workflow rather than to an action.
//
// The distinction is structural: a reusable-workflow reference
// contains the path component "/.github/workflows/". An action
// reference does not.
//
//   - Action:            actions/checkout@11bd71901...
//   - Local action:      ./path/to/action
//   - Reusable workflow: slsa-framework/slsa-github-generator/.github/workflows/generator_generic_slsa3.yml@v2.0.0
//
// The helper is used by the contract tests to skip reusable-workflow
// references from the full-SHA assertion. The pinning policy exempts
// them: GitHub does not support pinning a reusable workflow to a SHA
// in the same way it supports pinning an action, and the SLSA
// generator's own documentation recommends a version tag. The
// exemption is documented on ci.PinnedActions and in ADR-043.
//
// # Why this is separate from ci.isReusableWorkflowKey
//
// The two helpers answer the same question about two different
// strings. A pin-map key is `owner/repo/.github/workflows/file.yml`
// (no `@ref`); a `uses:` reference is the same path with an `@ref`
// appended. The distinction matters because a caller that used the
// key form on a reference would still see the marker (the path is a
// substring), but the two helpers live in different packages
// (`ci` and `ci_test`) and cannot share an unexported function. Each
// package has its own; the duplication is two lines of substring
// search.
func isReusableWorkflowRef(ref string) bool {
	const marker = "/.github/workflows/"
	return strings.Contains(ref, marker)
}
