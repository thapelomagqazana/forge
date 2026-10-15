package ci_test

// workflows_contract_test.go — external contract tests for the CI
// workflow architecture.
//
// The tests in this file treat internal/ci as a black box and verify
// that the artifacts on disk agree with the model the package
// declares. The artifacts are:
//
//   - .github/workflows/ci.yml
//   - .github/workflows/release.yml
//   - .github/workflows/security.yml
//   - .github/workflows/nightly.yml
//   - .github/workflows/docs.yml
//   - .github/dependabot.yml
//
// The file is package ci_test because it must not depend on the
// package's unexported state. Every assertion below is stated in terms
// of the exported surface (AllWorkflows, Lookup, PinnedActions,
// IsPinnedSHA) or in terms of the raw bytes of the files.
//
// # Why regex over raw YAML
//
// Some assertions in this file could be written with a YAML parser.
// The tests below use regexes instead for three reasons:
//
//  1. The assertions are about the *text* of the YAML, not about the
//     data structure a parser would produce. "Every `uses:` is a full
//     SHA" is a statement about the token after `@`, not about the
//     value of a map key.
//
//  2. A regex is insensitive to anchors, aliases, multi-document
//     streams, and merge keys, all of which are legal YAML and all of
//     which would require careful handling in a parser-based check.
//
//  3. A regex failure message names the offending line, which is the
//     most useful thing a reviewer can see. A parser failure message
//     names a data structure.
//
// The regexes are anchored on the `uses:` key and on the `@`
// separator, which makes their false-positive risk negligible for the
// well-formed YAML the repository is expected to contain.
//
// # Why the tests read files instead of importing a YAML library
//
// The repository's dependency policy (docs/dependency-policy.md) is
// restrictive. gopkg.in/yaml.v3 is already a dependency (internal/
// platform and internal/config use it), but pulling it into this test
// file for four regex-shaped assertions would be a new transitive use
// of the library in a package that does not otherwise need it. The
// regexes are self-contained and cheaper to read.

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/thapelomagqazana/forge/internal/ci"
)

// --- Path helpers ----------------------------------------------------------

// repoRoot returns the absolute path to the repository root.
//
// The function walks up from the test's working directory until it
// finds a go.mod file. This is the pattern used throughout the
// repository (internal/arch/testhelpers_test.go,
// internal/cli/structure_test.go, internal/platform/schema_test.go,
// internal/version/version_test.go) and it is used here for the same
// reason: the repository has no environment variable that names its
// root, and go test runs with the package directory as the working
// directory, so a relative path would be wrong.
//
// The function fails the test if go.mod is not found before the
// filesystem root is reached. A test binary that runs outside a
// repository checkout is a configuration error, not a test failure,
// but t.Fatal is the closest the testing package offers.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found walking up from the test working directory")
		}
		dir = parent
	}
}

// workflowPath returns the absolute path to a workflow file.
//
// The path is derived from the model's FileName method, not from a
// hard-coded table. This is deliberate: the test that reads the file
// and the code that names it agree by construction, so a change to
// the naming convention requires a change to exactly one place.
func workflowPath(t *testing.T, root string, id ci.WorkflowID) string {
	t.Helper()
	return filepath.Join(root, ".github", "workflows", id.FileName())
}

// readWorkflow reads the workflow file for id and returns its bytes.
//
// The function fails the test if the file is missing. Every test in
// this file that reads a workflow uses this helper, so a missing file
// produces a single, well-named failure rather than a cascade.
func readWorkflow(t *testing.T, root string, id ci.WorkflowID) []byte {
	t.Helper()
	path := workflowPath(t, root, id)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

// --- Existence -------------------------------------------------------------

// TestContract_EveryWorkflowFileExists asserts every workflow in the
// closed set has a YAML file at the expected path.
//
// A missing file is the most basic possible violation: the model
// declares a workflow that the repository does not implement. The test
// is the first line of defence for the "five workflows defined"
// acceptance criterion.
func TestContract_EveryWorkflowFileExists(t *testing.T) {
	root := repoRoot(t)
	for _, id := range ci.AllWorkflows() {
		path := workflowPath(t, root, id)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("workflow file %s is missing: %v", path, err)
			continue
		}
		if info.IsDir() {
			t.Errorf("workflow path %s is a directory, not a file", path)
		}
		if info.Size() == 0 {
			t.Errorf("workflow file %s is empty", path)
		}
	}
}

// TestContract_NoOrphanedWorkflowFiles asserts there are no workflow
// files under .github/workflows/ that the model does not declare.
//
// The test is the complement of TestContract_EveryWorkflowFileExists:
// together they pin the file set to exactly the closed set. An orphan
// file — a workflow that exists but is not in the model — would run on
// GitHub without appearing in the required-checks policy, the
// architecture document, or the ownership table. That is the failure
// mode the test exists to prevent.
//
// The check is deliberately limited to *.yml and *.yaml files directly
// under .github/workflows/. GitHub does not read nested directories,
// and the two extensions are the only ones GitHub recognises.
func TestContract_NoOrphanedWorkflowFiles(t *testing.T) {
	root := repoRoot(t)
	dir := filepath.Join(root, ".github", "workflows")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}

	// Build the set of files the model declares.
	known := make(map[string]struct{}, len(ci.AllWorkflows()))
	for _, id := range ci.AllWorkflows() {
		known[id.FileName()] = struct{}{}
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml") {
			continue
		}
		if _, ok := known[name]; !ok {
			t.Errorf(".github/workflows/%s is not declared by the model", name)
		}
	}
}

// --- pull_request_target prohibition ---------------------------------------

// TestContract_NoPullRequestTarget asserts no workflow uses the
// pull_request_target event.
//
// The prohibition is recorded in ADR-043. The event grants write
// permissions and access to repository secrets to code from a fork,
// which is the root cause of a long history of CI supply-chain
// compromises. The test is deliberately broad: it matches the string
// anywhere in the file, including in comments, because a comment that
// mentions pull_request_target is a signal that someone is considering
// it and the reviewer should see the test fail and read the ADR.
//
// A future ADR that authorises pull_request_target would replace this
// test with a more precise one that asserts the safe pattern. Until
// then, the prohibition is absolute.
func TestContract_NoPullRequestTarget(t *testing.T) {
	root := repoRoot(t)
	for _, id := range ci.AllWorkflows() {
		data := readWorkflow(t, root, id)
		if strings.Contains(string(data), "pull_request_target") {
			t.Errorf(
				"%s references pull_request_target; this is forbidden by ADR-043",
				workflowPath(t, root, id),
			)
		}
	}
}

// --- Pinned-action policy --------------------------------------------------

// usesPattern matches a `uses:` line and captures the action reference
// and its ref.
//
// The pattern is deliberately narrow:
//
//   - `uses:` must be followed by optional whitespace and then an
//     owner/name reference. This excludes `uses: ./local/action` and
//     `uses: docker://image:tag`, which have no upstream SHA.
//
//   - The captured ref is everything after `@` up to whitespace or end
//     of line. It may be a tag, a branch, a short SHA, or a full SHA;
//     the caller decides which forms are acceptable.
//
// The pattern is compiled once at package level. Compiling inside a
// loop would allocate per iteration; compiling at package level makes
// the cost a one-time startup cost.
var usesPattern = regexp.MustCompile(
	`(?m)^\s*uses:\s*([A-Za-z0-9_.\-]+/[A-Za-z0-9_.\-]+)@([A-Za-z0-9_.\-]+)`,
)

// TestContract_EveryUsesIsPinnedToFullSHA asserts every action
// reference in every workflow is pinned to a full 40-hex-character
// lowercase SHA.
//
// The test iterates every `uses:` that matches the owner/name pattern
// and asserts the captured ref satisfies ci.IsPinnedSHA. A ref that is
// a tag (`v4`), a branch (`main`), a short SHA (`11bd719`), or an
// uppercase SHA fails the assertion.
//
// The failure message names the file, the action, and the ref, so a
// reviewer can locate the offending line without opening the file.
func TestContract_EveryUsesIsPinnedToFullSHA(t *testing.T) {
	root := repoRoot(t)
	for _, id := range ci.AllWorkflows() {
		path := workflowPath(t, root, id)
		data := readWorkflow(t, root, id)
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
}

// TestContract_EveryUsesIsInTheClosedSet asserts every action
// reference in every workflow appears in the model's PinnedActions
// map.
//
// TestContract_EveryUsesIsPinnedToFullSHA checks the *shape* of the
// ref. This test checks the *identity* of the action: a workflow that
// references an action the model does not declare has introduced a new
// upstream dependency without an ADR, and the test fails so that the
// change is reviewed.
func TestContract_EveryUsesIsInTheClosedSet(t *testing.T) {
	root := repoRoot(t)
	known := ci.PinnedActions()
	for _, id := range ci.AllWorkflows() {
		path := workflowPath(t, root, id)
		data := readWorkflow(t, root, id)
		for _, m := range usesPattern.FindAllStringSubmatch(string(data), -1) {
			action, ref := m[1], m[2]
			pinned, ok := known[action]
			if !ok {
				t.Errorf(
					"%s: action %s is not in the closed pinned set",
					path, action,
				)
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
}

// TestContract_NoMutableTagReferences asserts no `uses:` references a
// tag or a branch by name.
//
// This is a stronger, more specific form of
// TestContract_EveryUsesIsPinnedToFullSHA. It exists so that a failure
// produces the message "this is a mutable tag" rather than "this is
// not a 40-hex SHA", which is more actionable for a contributor who
// copied an example from GitHub's own documentation.
func TestContract_NoMutableTagReferences(t *testing.T) {
	root := repoRoot(t)
	// Matches `@v1`, `@v1.2`, `@v1.2.3`, `@main`, `@master`, `@HEAD`,
	// and `@latest`. The ref must be the whole ref, not a prefix, so
	// that a full SHA is never matched.
	tagPattern := regexp.MustCompile(
		`(?m)^\s*uses:\s*([A-Za-z0-9_.\-]+/[A-Za-z0-9_.\-]+)@(v\d+(\.\d+)*|main|master|HEAD|latest)(\s|$)`,
	)
	for _, id := range ci.AllWorkflows() {
		path := workflowPath(t, root, id)
		data := readWorkflow(t, root, id)
		for _, m := range tagPattern.FindAllStringSubmatch(string(data), -1) {
			t.Errorf(
				"%s: %s@%s is a mutable tag; pin to a full SHA",
				path, m[1], m[2],
			)
		}
	}
}

// --- Permissions -----------------------------------------------------------

// TestContract_EveryWorkflowDeclaresPermissions asserts every workflow
// declares a top-level permissions block.
//
// The check is a substring match on "permissions:". A YAML parser
// would be more precise, but the substring is sufficient: a workflow
// that lacks the key cannot contain the string except in a comment,
// and a workflow that contains the string in a comment would still
// need the real key to satisfy GitHub's behaviour.
//
// The check is deliberately shallow. The *content* of the permissions
// block is asserted by TestContract_PermissionsMatchModel, which
// parses the block.
func TestContract_EveryWorkflowDeclaresPermissions(t *testing.T) {
	root := repoRoot(t)
	for _, id := range ci.AllWorkflows() {
		path := workflowPath(t, root, id)
		data := readWorkflow(t, root, id)
		if !strings.Contains(string(data), "permissions:") {
			t.Errorf("%s does not declare a permissions block", path)
		}
	}
}

// permissionLinePattern matches a permission line inside a
// permissions block. It captures the scope and the level.
//
// The pattern is used only inside TestContract_PermissionsMatchModel.
// It is deliberately permissive on whitespace so that both
// `contents: read` and `contents:read` are captured; the test then
// normalises the value to the model's canonical form.
var permissionLinePattern = regexp.MustCompile(
	`(?m)^\s*([a-z][a-z0-9-]*):\s*(read|write|none)\s*$`,
)

// TestContract_PermissionsMatchModel asserts each workflow's
// permissions block contains exactly the scopes the model declares.
//
// The test does not parse YAML. It extracts the permissions block by
// scanning for the top-level "permissions:" key and then collecting
// the indented lines that follow, stopping at the first line that is
// not indented. This is a small, well-defined subset of YAML
// (block-style mappings at a fixed indentation) and the workflows in
// this repository use only that subset.
//
// The check is two-way: a permission the model declares but the file
// does not grant fails, and a permission the file grants but the model
// does not declare fails. The two-way check is the point: the model is
// the specification, and the file must match it exactly.
func TestContract_PermissionsMatchModel(t *testing.T) {
	root := repoRoot(t)
	for _, id := range ci.AllWorkflows() {
		path := workflowPath(t, root, id)
		data := readWorkflow(t, root, id)

		w, ok := ci.Lookup(id)
		if !ok {
			t.Fatalf("Lookup(%q) not found; the model is inconsistent", id)
		}

		// want is the model's view, normalised to the canonical form
		// "scope: level" with exactly one space after the colon.
		want := make(map[string]struct{}, len(w.Permissions))
		for _, p := range w.Permissions {
			want[string(p)] = struct{}{}
		}

		got := extractPermissions(t, path, string(data))
		if !sameStringSet(want, got) {
			t.Errorf(
				"%s: permissions do not match the model\n  want: %v\n  got:  %v",
				path, sortedKeys(want), sortedKeys(got),
			)
		}
	}
}

// extractPermissions returns the set of "scope: level" entries in the
// top-level permissions block of a workflow file.
//
// The function scans the file line by line. It finds the first line
// whose trimmed content is "permissions:" and then collects the
// indented lines that follow until a line is reached that is not
// indented (or a line that begins a new top-level key).
//
// The function is deliberately simple. It does not handle:
//
//   - `permissions: read-all` (the shorthand form)
//   - `permissions: {}` (the deny-all shorthand)
//   - `permissions: write-all`
//
// A workflow that uses one of those forms would produce an empty
// extraction and the test would fail with a clear message. The
// repository's workflows use the block form exclusively; a future
// change to a shorthand form should be accompanied by an update to
// this function and to the test's docstring.
func extractPermissions(t *testing.T, path, content string) map[string]struct{} {
	t.Helper()
	out := make(map[string]struct{})

	lines := strings.Split(content, "\n")
	inBlock := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "permissions:" {
			inBlock = true
			continue
		}
		if !inBlock {
			continue
		}

		// A top-level key (no leading whitespace, non-empty) ends the
		// block. The blank-line case is also handled by this branch:
		// an empty trimmed string has no leading whitespace, but it
		// also has no content, so it cannot be a top-level key. The
		// explicit check keeps the intent obvious.
		if trimmed == "" {
			continue
		}
		if line[0] != ' ' && line[0] != '\t' {
			// A new top-level key; the permissions block has ended.
			break
		}

		m := permissionLinePattern.FindStringSubmatch(line)
		if m == nil {
			// A continuation line that does not look like a permission.
			// YAML does not have continuation lines, so this is a
			// syntax error in the file; report it so the test fails
			// with a useful message rather than silently ignoring it.
			t.Errorf("%s: unexpected line in permissions block: %q", path, line)
			continue
		}
		out[m[1]+": "+m[2]] = struct{}{}
	}
	if !inBlock {
		t.Errorf("%s: no permissions block found", path)
	}
	return out
}

// --- Trigger checks --------------------------------------------------------

// TestContract_TriggersMatchModel asserts each workflow's `on:` block
// contains the triggers the model declares.
//
// The check is one-way: the file must contain every trigger the model
// declares. It does not assert that the file contains *only* those
// triggers, because a workflow may legitimately add a trigger that the
// model does not model (for example, a `paths:` filter on a trigger
// that is already present). The model's role is to name the *events*,
// not to enumerate every filter under each event.
func TestContract_TriggersMatchModel(t *testing.T) {
	root := repoRoot(t)
	for _, id := range ci.AllWorkflows() {
		path := workflowPath(t, root, id)
		data := readWorkflow(t, root, id)

		w, ok := ci.Lookup(id)
		if !ok {
			t.Fatalf("Lookup(%q) not found; the model is inconsistent", id)
		}
		for _, tr := range w.Triggers {
			// The model uses "push:tags" for the tag trigger, but the
			// YAML file uses "push:" with a "tags:" filter. The
			// normalisation below maps the model's name to the string
			// that must appear in the file.
			needle := triggerYAMLName(tr)
			if !strings.Contains(string(data), needle) {
				t.Errorf(
					"%s: trigger %q (model name %q) is not declared",
					path, needle, tr,
				)
			}
		}
	}
}

// triggerYAMLName maps a model Trigger to the string that must appear
// in the YAML file.
//
// For most triggers the mapping is the identity: the model uses the
// same spelling GitHub uses. The exception is TriggerTag, which the
// model spells "push:tags" and the file spells "push:" followed by a
// "tags:" filter. The mapping normalises that one case.
func triggerYAMLName(t ci.Trigger) string {
	if t == ci.TriggerTag {
		return "push:"
	}
	return string(t)
}

// --- Dependabot ------------------------------------------------------------

// TestContract_DependabotConfigExists asserts the dependabot
// configuration file exists at the expected path.
func TestContract_DependabotConfigExists(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, ".github", "dependabot.yml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf(".github/dependabot.yml is missing: %v", err)
	}
}

// TestContract_DependabotGroupsGitHubActions asserts the dependabot
// configuration groups github-actions updates.
//
// The assertions are:
//
//   - the github-actions ecosystem is declared,
//   - the schedule interval is weekly,
//   - a groups: block is present with a patterns: entry.
//
// The exact group name and the exact pattern values are not asserted.
// The property that matters is that updates are grouped, not what the
// group is called. A rename of the group is a cosmetic change that
// should not fail a test.
func TestContract_DependabotGroupsGitHubActions(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, ".github", "dependabot.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	s := string(data)

	// The github-actions ecosystem must be declared. Both quoted and
	// unquoted forms are accepted; the repository's file uses the
	// quoted form, but a future edit should not fail on style alone.
	if !strings.Contains(s, `package-ecosystem: "github-actions"`) &&
		!strings.Contains(s, "package-ecosystem: github-actions") {
		t.Error("dependabot.yml does not declare the github-actions ecosystem")
	}

	if !strings.Contains(s, `interval: "weekly"`) &&
		!strings.Contains(s, "interval: weekly") {
		t.Error("dependabot.yml does not use a weekly interval")
	}

	if !strings.Contains(s, "groups:") {
		t.Error("dependabot.yml does not declare any groups")
	}
	if !strings.Contains(s, "patterns:") {
		t.Error("dependabot.yml does not declare any patterns")
	}
}

// --- Documentation and ADR -------------------------------------------------

// TestContract_CIArchitectureDocExists asserts the architecture
// document exists at the expected path.
func TestContract_CIArchitectureDocExists(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "docs", "architecture", "ci-architecture.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("docs/architecture/ci-architecture.md is missing: %v", err)
	}
}

// TestContract_CIArchitectureDocListsEveryWorkflow asserts the
// architecture document names every workflow in the closed set.
//
// The check is a substring match on the workflow's file name. A
// workflow that is added to the model but not to the document would
// produce a document that describes a different architecture from the
// one the code enforces. This is the drift the test prevents.
func TestContract_CIArchitectureDocListsEveryWorkflow(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "docs", "architecture", "ci-architecture.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	s := string(data)
	for _, id := range ci.AllWorkflows() {
		if !strings.Contains(s, id.FileName()) {
			t.Errorf(
				"docs/architecture/ci-architecture.md does not name %s",
				id.FileName(),
			)
		}
	}
}

// --- Helpers ---------------------------------------------------------------

// sameStringSet reports whether two string sets are equal.
//
// The function is used by TestContract_PermissionsMatchModel. It is
// a set comparison, not a slice comparison, so order does not matter.
func sameStringSet(a, b map[string]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}

// sortedKeys returns the keys of a set, sorted. The helper exists so
// that a failure message is deterministic.
func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
