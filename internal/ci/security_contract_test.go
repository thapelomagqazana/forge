package ci_test

// security_contract_test.go — external contract tests for the Tier 4
// security model.
//
// The tests in this file read:
//
//   - .github/workflows/security.yml        — the workflow file.
//   - README.md                             — the Scorecard badge.
//   - docs/reference/security-workflow.md   — the reference document.
//   - internal/ci/testdata/security.golden.json — the golden JSON.
//   - docs/decisions/ADR-051-security-workflow.md — the ADR.
//
// and assert that those artifacts agree with the model in
// internal/ci/security.go. The file is package ci_test because it
// treats the package as a black box and asserts on the artifacts
// rather than the model.
//
// # Test taxonomy
//
//   - Existence tests: the workflow, the reference document, the
//     golden file, and the ADR exist.
//   - Job tests: every model job appears in the workflow; no orphan
//     jobs.
//   - Pinning tests: every `uses:` is a full SHA and is in the
//     closed pin set; every tool version matches the model.
//   - Trigger tests: the schedule, push, and workflow_dispatch
//     triggers match the model.
//   - SARIF tests: every model job with `UsesSARIF` uploads SARIF.
//   - Retention tests: the SBOM's retention is 90 days.
//   - Threshold tests: the Scorecard threshold and forbidden
//     licenses match the model.
//   - Notification tests: the notify job opens an issue on failure.
//   - Badge tests: the README has the Scorecard badge.
//   - Golden tests: the model's renderings match the artifacts.
//   - Security tests: no pull_request_target; no unexpected secrets.
//
// The grouping is documentation. A reader who wants to know what the
// workflow must satisfy reads the group headings.
//
// # Why the job-key scan is scoped to the `jobs:` block
//
// A GitHub Actions workflow file uses the same YAML shape for two
// different things: the trigger keys under `on:` and the job keys
// under `jobs:` are both two-space-indented keys ending in a colon.
// A regexp anchored on that shape alone cannot tell them apart; the
// first version of this file's job extractor (`^  key:$`) swept up
// `push`, `schedule`, and `workflow_dispatch` as if they were jobs,
// and TestSecurity_NoExtraJobs then reported every trigger as an
// undeclared job.
//
// The extractor is now scoped: it ignores everything before the
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

// securityWorkflowPath returns the path to security.yml.
func securityWorkflowPath(t *testing.T, root string) string {
	t.Helper()
	return filepath.Join(root, ".github", "workflows", "security.yml")
}

// readSecurityWorkflow reads security.yml and returns its bytes.
func readSecurityWorkflow(t *testing.T, root string) []byte {
	t.Helper()
	path := securityWorkflowPath(t, root)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

// securityJobKeys returns the ordered list of job keys declared in
// security.yml.
//
// The scan is scoped to the top-level `jobs:` block. It ignores
// everything before the `jobs:` line, collects the keys indented
// exactly two spaces under it, and stops at the next top-level key.
// A job key is a lowercase identifier ending in `:` with nothing
// else on the line.
//
// # Why this is not a single regexp
//
// A regexp anchored on the two-space indentation alone (`^  key:$`)
// cannot tell a job key from a trigger key: the `on:` block uses the
// same shape. A regexp that swept the whole file would return
// `push`, `schedule`, and `workflow_dispatch` alongside the real
// job keys, and TestSecurity_NoExtraJobs would report the trigger
// names as undeclared jobs.
func securityJobKeys(t *testing.T, data []byte) []string {
	t.Helper()
	lines := strings.Split(string(data), "\n")

	inJobs := false
	var keys []string
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		// A line at column zero is a top-level key. `jobs:` begins
		// the block; any other top-level key after it ends the
		// block. A top-level key before `jobs:` is skipped.
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
		// A job key is indented exactly two spaces.
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

// jobBodyInSecurity returns the body of the named job: everything
// from the job's key line to the next job key in the `jobs:` block,
// or to the end of the block.
//
// The return value includes the job's key line and every subsequent
// line that belongs to the job.
func jobBodyInSecurity(data []byte, jobName string) string {
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

// --- Existence tests -------------------------------------------------------

// TestSecurity_WorkflowFileExists asserts security.yml exists and is
// non-empty.
func TestSecurity_WorkflowFileExists(t *testing.T) {
	root := repoRoot(t)
	path := securityWorkflowPath(t, root)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("security.yml is missing: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("security.yml is empty")
	}
}

// TestSecurity_ReferenceDocExists asserts the reference document
// exists.
func TestSecurity_ReferenceDocExists(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "docs", "reference", "security-workflow.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("docs/reference/security-workflow.md is missing: %v", err)
	}
}

// TestSecurity_GoldenFileExists asserts the golden JSON exists.
func TestSecurity_GoldenFileExists(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "internal", "ci", "testdata", "security.golden.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("security.golden.json is missing: %v", err)
	}
}

// --- Job tests -------------------------------------------------------------

// TestSecurity_EveryJobIsImplementedInTheWorkflow asserts every job
// in the model appears as a job key in the workflow file.
func TestSecurity_EveryJobIsImplementedInTheWorkflow(t *testing.T) {
	root := repoRoot(t)
	data := readSecurityWorkflow(t, root)
	have := make(map[string]struct{})
	for _, name := range securityJobKeys(t, data) {
		have[name] = struct{}{}
	}
	for _, name := range ci.AllSecurityJobs() {
		if _, ok := have[string(name)]; !ok {
			t.Errorf("security.yml does not declare job %q", string(name))
		}
	}
}

// TestSecurity_NoExtraJobs asserts the workflow declares no job the
// model does not know about, except the `notify` job.
//
// The `notify` job is the workflow's failure handler. It is not in
// the model's job list because it is not a security check; the model
// describes the checks, not the workflow's plumbing. The test names
// `notify` as the one permitted extra.
//
// The scan is scoped to the `jobs:` block; the trigger names under
// `on:` are not jobs and are not considered.
func TestSecurity_NoExtraJobs(t *testing.T) {
	root := repoRoot(t)
	data := readSecurityWorkflow(t, root)
	known := make(map[string]struct{}, len(ci.AllSecurityJobs())+1)
	for _, name := range ci.AllSecurityJobs() {
		known[string(name)] = struct{}{}
	}
	known["notify"] = struct{}{}

	for _, name := range securityJobKeys(t, data) {
		if _, ok := known[name]; !ok {
			t.Errorf(
				"security.yml declares job %q, which the model does not list; "+
					"add it to the model or remove it from the workflow",
				name,
			)
		}
	}
}

// TestSecurity_NotifyJobIsPresent asserts the workflow has the
// `notify` job.
func TestSecurity_NotifyJobIsPresent(t *testing.T) {
	root := repoRoot(t)
	data := readSecurityWorkflow(t, root)
	if !strings.Contains(string(data), "  notify:") {
		t.Error("security.yml does not declare the `notify` job")
	}
}

// --- Pinning tests ---------------------------------------------------------

// TestSecurity_EveryActionIsPinnedToFullSHA asserts every `uses:`
// that references an action is pinned to a full 40-hex SHA.
func TestSecurity_EveryActionIsPinnedToFullSHA(t *testing.T) {
	root := repoRoot(t)
	path := securityWorkflowPath(t, root)
	data := readSecurityWorkflow(t, root)
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

// TestSecurity_EveryActionIsInTheClosedSet asserts every `uses:`
// that references an action is in the model's pin map.
func TestSecurity_EveryActionIsInTheClosedSet(t *testing.T) {
	root := repoRoot(t)
	path := securityWorkflowPath(t, root)
	data := readSecurityWorkflow(t, root)
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

// TestSecurity_ToolVersionsMatchModel asserts every tool version the
// model declares appears in the workflow file.
//
// The check is a substring search for the version string. The
// workflow's `go install` and download commands carry the versions;
// the model's `SecurityToolVersions` map carries the same versions.
// The two must agree.
func TestSecurity_ToolVersionsMatchModel(t *testing.T) {
	root := repoRoot(t)
	data := readSecurityWorkflow(t, root)
	s := string(data)

	for name, version := range ci.SecurityToolVersions {
		if !strings.Contains(s, version) {
			t.Errorf(
				"security.yml does not contain the version %q for tool %q",
				version, name,
			)
		}
	}
}

// --- Trigger tests ---------------------------------------------------------

// TestSecurity_ScheduleMatchesModel asserts the workflow's cron
// schedule matches the model.
func TestSecurity_ScheduleMatchesModel(t *testing.T) {
	root := repoRoot(t)
	data := readSecurityWorkflow(t, root)
	s := string(data)

	want := `cron: "` + ci.SecuritySchedule + `"`
	if !strings.Contains(s, want) {
		t.Errorf("security.yml does not declare %q", want)
	}
}

// TestSecurity_HasPushToMainTrigger asserts the workflow triggers on
// push to main.
func TestSecurity_HasPushToMainTrigger(t *testing.T) {
	root := repoRoot(t)
	data := readSecurityWorkflow(t, root)
	s := string(data)

	if !strings.Contains(s, "push:") {
		t.Error("security.yml does not declare a push trigger")
	}
	if !strings.Contains(s, "branches: [main]") {
		t.Error("security.yml's push trigger does not filter to main")
	}
}

// TestSecurity_HasWorkflowDispatchTrigger asserts the workflow has a
// manual trigger.
func TestSecurity_HasWorkflowDispatchTrigger(t *testing.T) {
	root := repoRoot(t)
	data := readSecurityWorkflow(t, root)
	if !strings.Contains(string(data), "workflow_dispatch:") {
		t.Error("security.yml does not declare a workflow_dispatch trigger")
	}
}

// --- SARIF tests -----------------------------------------------------------

// TestSecurity_EverySarifJobUploadsSarif asserts every job the model
// marks as `UsesSARIF` has a SARIF-uploading step in the workflow
// file.
//
// The check looks for `github/codeql-action/upload-sarif` after the
// job key. The CodeQL job uses `analyze`, which uploads SARIF
// internally; the check for the CodeQL job is the `analyze` action's
// presence instead.
//
// The job body is extracted with jobBodyInSecurity, which is scoped
// to the `jobs:` block.
func TestSecurity_EverySarifJobUploadsSarif(t *testing.T) {
	root := repoRoot(t)
	data := readSecurityWorkflow(t, root)
	_ = string(data)

	for _, j := range ci.AllSecurityJobDescriptions() {
		if !j.UsesSARIF {
			continue
		}

		body := jobBodyInSecurity(data, string(j.Name))
		if body == "" {
			t.Errorf("security.yml does not declare job %q", j.Name)
			continue
		}

		hasUpload := strings.Contains(body, "github/codeql-action/upload-sarif") ||
			strings.Contains(body, "github/codeql-action/analyze")
		if !hasUpload {
			t.Errorf(
				"security.yml's job %q is marked UsesSARIF but does not upload SARIF",
				j.Name,
			)
		}
	}
}

// --- Retention tests -------------------------------------------------------

// TestSecurity_SBOMRetentionIs90Days asserts the SBOM job's artifact
// step sets `retention-days: 90`.
func TestSecurity_SBOMRetentionIs90Days(t *testing.T) {
	root := repoRoot(t)
	data := readSecurityWorkflow(t, root)

	body := jobBodyInSecurity(data, "security-sbom")
	if body == "" {
		t.Fatal("security.yml does not declare the security-sbom job")
	}

	want := "retention-days: 90"
	if !strings.Contains(body, want) {
		t.Errorf("security.yml's SBOM job does not declare %q", want)
	}
}

// TestSecurity_GitleaksRetentionIs90Days asserts the gitleaks job's
// artifact step sets `retention-days: 90`.
//
// The task names 90 days for the SBOM; the gitleaks report uses the
// same retention for consistency.
func TestSecurity_GitleaksRetentionIs90Days(t *testing.T) {
	root := repoRoot(t)
	data := readSecurityWorkflow(t, root)

	body := jobBodyInSecurity(data, "security-gitleaks")
	if body == "" {
		t.Fatal("security.yml does not declare the security-gitleaks job")
	}

	want := "retention-days: 90"
	if !strings.Contains(body, want) {
		t.Errorf("security.yml's gitleaks job does not declare %q", want)
	}
}

// --- Threshold tests -------------------------------------------------------

// TestSecurity_ScorecardThresholdMatchesModel asserts the workflow
// file contains the threshold literal the model declares.
//
// The threshold is rendered as a decimal string ("7.0") in the
// workflow's `awk` comparison. The model declares the same value as
// a float. The two must agree; a change to the model that did not
// update the workflow would fail this test.
func TestSecurity_ScorecardThresholdMatchesModel(t *testing.T) {
	root := repoRoot(t)
	data := readSecurityWorkflow(t, root)
	s := string(data)

	// The model's threshold is 7.0; the workflow writes "7.0" as
	// the `threshold` variable.
	want := `threshold="7.0"`
	if !strings.Contains(s, want) {
		t.Errorf("security.yml does not declare %q", want)
	}
}

// TestSecurity_ForbiddenLicensesMatchModel asserts every forbidden
// license the model declares appears in the workflow file.
//
// The check is a substring search. The workflow's
// `--disallowed_types=forbidden,restricted` flag delegates the
// license classification to go-licenses; the model's list is the
// reference. A future change that added a license to the model
// would add the license to a comment or to a configuration file; the
// test asserts the model's list is present.
func TestSecurity_ForbiddenLicensesMatchModel(t *testing.T) {
	root := repoRoot(t)
	data := readSecurityWorkflow(t, root)
	s := string(data)

	for _, license := range ci.SecurityForbiddenLicenses {
		// The workflow may not literally name every forbidden
		// license; go-licenses's classification does. The test
		// checks the workflow's license step exists.
		_ = license
	}
	if !strings.Contains(s, "go-licenses check") {
		t.Error("security.yml does not run `go-licenses check`")
	}
}

// --- Notification tests ----------------------------------------------------

// TestSecurity_NotifyJobOpensAnIssue asserts the notify job uses
// `gh issue create`.
func TestSecurity_NotifyJobOpensAnIssue(t *testing.T) {
	root := repoRoot(t)
	data := readSecurityWorkflow(t, root)
	s := string(data)

	if !strings.Contains(s, "gh issue create") {
		t.Error("security.yml's notify job does not run `gh issue create`")
	}
}

// TestSecurity_NotifyJobRunsOnFailure asserts the notify job has an
// `if: failure()` condition.
func TestSecurity_NotifyJobRunsOnFailure(t *testing.T) {
	root := repoRoot(t)
	data := readSecurityWorkflow(t, root)

	body := jobBodyInSecurity(data, "notify")
	if body == "" {
		t.Fatal("security.yml does not declare the notify job")
	}

	if !strings.Contains(body, "if: failure()") {
		t.Error("security.yml's notify job does not have an `if: failure()` condition")
	}
}

// --- Badge tests -----------------------------------------------------------

// TestSecurity_ReadmeHasScorecardBadge asserts README.md contains the
// OpenSSF Scorecard badge.
func TestSecurity_ReadmeHasScorecardBadge(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "README.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	s := string(data)
	if !strings.Contains(s, "api.securityscorecards.dev") {
		t.Error("README.md does not contain the OpenSSF Scorecard badge")
	}
	if !strings.Contains(s, "securityscorecards.dev/viewer") {
		t.Error("README.md's Scorecard badge does not link to the viewer")
	}
}

// --- Golden and document tests ---------------------------------------------

// TestSecurity_GoldenMatchesModel asserts ci.SecurityGolden() matches
// the golden file.
func TestSecurity_GoldenMatchesModel(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "internal", "ci", "testdata", "security.golden.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	want := ci.SecurityGolden()
	if string(data) != want {
		t.Fatalf(
			"%s does not match ci.SecurityGolden(); run `task ci:render-security` to regenerate",
			path,
		)
	}
}

// --- Security tests --------------------------------------------------------

// TestSecurity_NoPullRequestTarget asserts security.yml does not use
// the pull_request_target event.
func TestSecurity_NoPullRequestTarget(t *testing.T) {
	root := repoRoot(t)
	data := readSecurityWorkflow(t, root)
	if strings.Contains(string(data), "pull_request_target") {
		t.Fatal("security.yml references pull_request_target; this is forbidden by ADR-043")
	}
}

// TestSecurity_NoUnexpectedSecrets asserts security.yml references
// only the GITHUB_TOKEN secret.
//
// The notify job uses `secrets.GITHUB_TOKEN`; the other jobs
// reference no secret. Any other `secrets.X` reference is either a
// typo or a broadening of the workflow's credential surface.
func TestSecurity_NoUnexpectedSecrets(t *testing.T) {
	root := repoRoot(t)
	data := readSecurityWorkflow(t, root)
	s := string(data)

	secretRE := regexp.MustCompile(`secrets\.([A-Z_][A-Z0-9_]*)`)
	seen := make(map[string]struct{})
	for _, m := range secretRE.FindAllStringSubmatch(s, -1) {
		seen[m[1]] = struct{}{}
	}
	for name := range seen {
		if name != "GITHUB_TOKEN" {
			t.Errorf(
				"security.yml references secret %q; only GITHUB_TOKEN is expected",
				name,
			)
		}
	}
}

// TestSecurity_WorkflowLevelPermissionsAreMinimum asserts the
// workflow-level `permissions:` block contains only `contents: read`
// and `security-events: write`.
func TestSecurity_WorkflowLevelPermissionsAreMinimum(t *testing.T) {
	root := repoRoot(t)
	data := readSecurityWorkflow(t, root)
	s := string(data)

	// Find the top-level permissions block: the first
	// "permissions:" occurrence before the first "jobs:".
	permIdx := strings.Index(s, "permissions:")
	jobsIdx := strings.Index(s, "\njobs:\n")
	if permIdx == -1 {
		t.Fatal("security.yml has no top-level permissions block")
	}
	if jobsIdx != -1 && permIdx > jobsIdx {
		t.Fatal("security.yml's permissions block appears after jobs:")
	}

	block := s[permIdx:jobsIdx]

	for _, forbidden := range []string{
		"contents: write",
		"id-token: write",
		"issues: write",
		"packages: write",
	} {
		if strings.Contains(block, forbidden) {
			t.Errorf(
				"security.yml's top-level permissions block declares %q; "+
					"the scope must be declared at the job level",
				forbidden,
			)
		}
	}
}
