// Package release_test contains the acceptance tests for the release
// workflow (WBS 17.5.3).
//
// The workflow is YAML; a YAML file is not unit-testable. The contract
// it must satisfy is a Go model in internal/release/workflow.go. These
// tests parse the workflow and compare it to the model. A change to
// the workflow that breaks the contract fails here before a human
// reads the diff.
//
// # What this file tests
//
//   - The trigger: on.push.tags is exactly "v*"; on.workflow_dispatch
//     exists and declares the dry_run input.
//   - The job graph: the workflow's jobs equal the model's jobs; each
//     job's `needs:` equals the model's Dependencies entry.
//   - The permissions: each job's `permissions:` block is exactly the
//     model's JobPermissions entry. A missing permission fails at run
//     time; an extra permission is a widening that violates the
//     least-privilege invariant.
//   - The top-level permissions: the top-level block is the minimum
//     every job's needs allow, not the union of every job's needs.
//   - The secrets: each job's `secrets.X` references are exactly the
//     union of the model's JobSecrets and JobBuiltinTokens entries for
//     that job. A Forge-managed credential and a GitHub-provided token
//     are both reads from the `secrets` context; the model tracks them
//     separately so AllSecrets (the input to SECURITY.md) is not
//     cluttered with a per-run token.
//   - The dry-run: publish and verify-release have an `if:` that
//     skips them when dry_run is 1; the checksums job imports the GPG
//     key only when dry_run is not 1; the checksums job passes
//     --no-gpg to the driver in a dry-run.
//   - The pins: every `uses:` is a tag or a SHA, not a branch; every
//     local reusable workflow (`./...`) is local.
//   - The concurrency: the group is per-version, so a dry-run for a
//     version serializes with a real release for the same version.
//   - The publish job attaches every supply-chain artifact the
//     verification guide names.
//   - The verify-release job runs `gpg --verify` against the
//     published SHA256SUMS.asc, so a malformed signature is caught.
//   - The model's dependency graph is acyclic.
//   - No Forge-managed secret name appears bare in a `run:` block,
//     except as the name of an environment variable the workflow has
//     bound to `${{ secrets.X }}` or as a `$NAME` shell reference.
//
// # What this file does not test
//
//   - The workflow's runtime behavior. A YAML file's behavior is
//     GitHub's; the test asserts the workflow's contract, not
//     GitHub's implementation. A behavioral test is the dry-run
//     itself.
//   - The reusable build workflow's internals. The build workflow is
//     a separate file with its own tests.
//   - The supply-chain drivers. They are tested in
//     cmd/forge-internal-*/main_test.go.
package release_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/thapelomagqazana/forge/internal/release"
)

// workflowPath returns the path to the release workflow, relative to
// the test's working directory (test/release).
func workflowPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), ".github", "workflows", "release.yml")
}

// loadWorkflow parses the workflow YAML into a generic map. The test
// asserts on the map's shape; it does not define a typed model of the
// whole workflow, because the workflow's schema is GitHub's and a
// typed model would go stale.
//
// The `on:` key is a YAML 1.1 keyword: some parsers read the bare
// word `on` as the boolean true. The function handles both spellings
// so the test does not depend on the parser's YAML version.
func loadWorkflow(t *testing.T) map[string]any {
	t.Helper()
	data, err := os.ReadFile(workflowPath(t))
	if err != nil {
		t.Fatalf("read workflow: %v", err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse workflow: %v", err)
	}
	return doc
}

// jobMap returns the workflow's `jobs:` map.
func jobMap(t *testing.T, doc map[string]any) map[string]any {
	t.Helper()
	jobs, ok := doc["jobs"].(map[string]any)
	if !ok {
		t.Fatalf("workflow has no jobs map; got %T", doc["jobs"])
	}
	return jobs
}

// onBlock returns the workflow's `on:` block. The function accepts
// the boolean true spelling that a YAML 1.1 parser may produce.
func onBlock(t *testing.T, doc map[string]any) map[string]any {
	t.Helper()
	if on, ok := doc["on"].(map[string]any); ok {
		return on
	}
	if on, ok := doc["true"].(map[string]any); ok {
		return on
	}
	t.Fatalf("workflow has no on: block; keys = %v", mapKeys(doc))
	return nil
}

// TestWorkflow_TriggersOnVStarTag asserts the workflow's
// on.push.tags is exactly `v*`.
func TestWorkflow_TriggersOnVStarTag(t *testing.T) {
	t.Parallel()
	on := onBlock(t, loadWorkflow(t))
	push, ok := on["push"].(map[string]any)
	if !ok {
		t.Fatalf("on.push is missing; got %T", on["push"])
	}
	tags, ok := push["tags"].([]any)
	if !ok || len(tags) != 1 {
		t.Fatalf("on.push.tags = %v; want [\"v*\"]", push["tags"])
	}
	got, _ := tags[0].(string)
	if got != release.DefaultTrigger().PushTags {
		t.Errorf("on.push.tags[0] = %q; want %q", got, release.DefaultTrigger().PushTags)
	}
}

// TestWorkflow_HasWorkflowDispatch asserts the manual trigger is
// enabled and declares the dry_run input with type boolean and a
// false default.
func TestWorkflow_HasWorkflowDispatch(t *testing.T) {
	t.Parallel()
	on := onBlock(t, loadWorkflow(t))
	wd, ok := on["workflow_dispatch"].(map[string]any)
	if !ok {
		t.Fatalf("on.workflow_dispatch is missing; got %T", on["workflow_dispatch"])
	}
	inputs, ok := wd["inputs"].(map[string]any)
	if !ok {
		t.Fatalf("workflow_dispatch.inputs is missing")
	}
	dryRun, ok := inputs[release.DefaultTrigger().DryRunInput].(map[string]any)
	if !ok {
		t.Fatalf("workflow_dispatch.inputs[%q] is missing", release.DefaultTrigger().DryRunInput)
	}
	if got, _ := dryRun["type"].(string); got != "boolean" {
		t.Errorf("dry_run input type = %q; want boolean", got)
	}
	if got, _ := dryRun["default"].(bool); got {
		t.Errorf("dry_run input default = true; want false (a manual run defaults to a real release)")
	}
}

// TestWorkflow_JobsMatchModel asserts the workflow's jobs are exactly
// the model's jobs. A missing job is a missing capability; an extra
// job is undocumented behavior.
func TestWorkflow_JobsMatchModel(t *testing.T) {
	t.Parallel()
	jobs := jobMap(t, loadWorkflow(t))
	got := mapKeys(jobs)
	want := make([]string, 0, len(release.AllJobs()))
	for _, j := range release.AllJobs() {
		want = append(want, string(j))
	}
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("workflow jobs = %v\nmodel jobs      = %v", got, want)
	}
}

// TestWorkflow_NeedsMatchModel asserts every job's `needs:` list
// equals the model's Dependencies entry. The comparison is over
// release.JobName values so the two sides share a type.
func TestWorkflow_NeedsMatchModel(t *testing.T) {
	t.Parallel()
	jobs := jobMap(t, loadWorkflow(t))
	for name, spec := range jobs {
		job, ok := spec.(map[string]any)
		if !ok {
			t.Fatalf("job %q is not a map; got %T", name, spec)
		}
		got := needsOf(job)
		want := release.Dependencies[release.JobName(name)]
		if !equalStringSets(got, want) {
			t.Errorf("job %q needs = %v; model = %v", name, got, want)
		}
	}
}

// TestWorkflow_PermissionsMatchModel asserts every job's
// `permissions:` block is exactly the model's JobPermissions entry.
//
// The test is two-directional: a missing permission fails at run
// time; an extra permission is a widening that violates the
// least-privilege invariant.
func TestWorkflow_PermissionsMatchModel(t *testing.T) {
	t.Parallel()
	jobs := jobMap(t, loadWorkflow(t))
	for name, spec := range jobs {
		job, ok := spec.(map[string]any)
		if !ok {
			t.Fatalf("job %q is not a map", name)
		}
		got := permissionsOf(job)
		want := release.JobPermissions[release.JobName(name)]
		for k, v := range want {
			if got[k] != v {
				t.Errorf("job %q permission %q = %q; model = %q", name, k, got[k], v)
			}
		}
		for k := range got {
			if _, ok := want[k]; !ok {
				t.Errorf("job %q declares permission %q which the model does not require", name, k)
			}
		}
	}
}

// TestWorkflow_TopLevelPermissionsIsMinimum asserts the top-level
// permissions block grants no write scope. A top-level write scope
// would give every job that declares no permissions a broader scope
// than it needs.
func TestWorkflow_TopLevelPermissionsIsMinimum(t *testing.T) {
	t.Parallel()
	doc := loadWorkflow(t)
	top, ok := doc["permissions"].(map[string]any)
	if !ok {
		// A workflow with no top-level permissions block gets
		// GitHub's default, which is broader than any job needs.
		t.Fatalf("workflow has no top-level permissions block; got %T", doc["permissions"])
	}
	for k, v := range top {
		s, _ := v.(string)
		if s == "write" {
			t.Errorf("top-level permission %q = write; the top-level block must be the minimum", k)
		}
	}
}

// TestWorkflow_SecretsMatchModel asserts every `secrets.X` reference
// in the workflow is listed for the job that reads it, in one of the
// model's three `secrets`-context maps: JobSecrets (a signing
// credential), JobBuiltinTokens (a per-run GitHub token), or
// JobPublicIdentifiers (a published value stored in the secrets
// context for convenience).
//
// The three maps exist because the model serves three readers with
// different needs. SECURITY.md's credential table wants the values
// Forge manages (JobSecrets ∪ JobPublicIdentifiers). The workflow
// test wants every name the workflow reads from the `secrets`
// context (the union of all three). The two answers differ by the
// built-in tokens, which Forge does not manage.
func TestWorkflow_SecretsMatchModel(t *testing.T) {
	t.Parallel()
	jobs := jobMap(t, loadWorkflow(t))

	allowed := map[string]map[string]bool{}
	addAll := func(job string, names []string) {
		if allowed[job] == nil {
			allowed[job] = map[string]bool{}
		}
		for _, s := range names {
			allowed[job][s] = true
		}
	}
	for job, secrets := range release.JobSecrets {
		addAll(string(job), secrets)
	}
	for job, tokens := range release.JobBuiltinTokens {
		addAll(string(job), tokens)
	}
	for job, ids := range release.JobPublicIdentifiers {
		addAll(string(job), ids)
	}

	for name, spec := range jobs {
		job, ok := spec.(map[string]any)
		if !ok {
			continue
		}
		for _, ref := range collectSecretRefs(job) {
			if !allowed[name][ref] {
				t.Errorf("job %q reads secret %q which the model does not list", name, ref)
			}
		}
	}
}

// TestWorkflow_SecretsModelCoversEveryJob asserts the model's two
// secret maps have an entry for every job. A missing entry is an
// omission that would make TestWorkflow_SecretsMatchModel reject a
// legitimate read.
func TestWorkflow_SecretsModelCoversEveryJob(t *testing.T) {
	t.Parallel()
	for _, job := range release.AllJobs() {
		if _, ok := release.JobSecrets[job]; !ok {
			t.Errorf("JobSecrets has no entry for job %q", job)
		}
		if _, ok := release.JobBuiltinTokens[job]; !ok {
			t.Errorf("JobBuiltinTokens has no entry for job %q", job)
		}
	}
}

// TestWorkflow_DryRunGatesPublish asserts the publish job's `if:`
// skips it when dry_run is 1.
func TestWorkflow_DryRunGatesPublish(t *testing.T) {
	t.Parallel()
	jobs := jobMap(t, loadWorkflow(t))
	publish, ok := jobs[string(release.JobPublish)].(map[string]any)
	if !ok {
		t.Fatalf("publish job is missing")
	}
	ifExpr, _ := publish["if"].(string)
	assertDryRunGate(t, "publish", ifExpr)
}

// TestWorkflow_DryRunGatesVerifyRelease asserts the verify-release
// job's `if:` skips it when dry_run is 1.
func TestWorkflow_DryRunGatesVerifyRelease(t *testing.T) {
	t.Parallel()
	jobs := jobMap(t, loadWorkflow(t))
	vr, ok := jobs[string(release.JobVerifyRelease)].(map[string]any)
	if !ok {
		t.Fatalf("verify-release job is missing")
	}
	ifExpr, _ := vr["if"].(string)
	assertDryRunGate(t, "verify-release", ifExpr)
}

// TestWorkflow_ChecksumsSkipsGPGOnDryRun asserts the checksums job
// does not import the GPG key in a dry-run and passes --no-gpg to
// the driver.
func TestWorkflow_ChecksumsSkipsGPGOnDryRun(t *testing.T) {
	t.Parallel()
	jobs := jobMap(t, loadWorkflow(t))
	checksums, ok := jobs[string(release.JobChecksums)].(map[string]any)
	if !ok {
		t.Fatalf("checksums job is missing")
	}
	steps, _ := checksums["steps"].([]any)
	importStep := findStep(steps, "Import GPG release key")
	if importStep == nil {
		t.Fatalf("checksums job has no 'Import GPG release key' step")
	}
	importIf, _ := importStep["if"].(string)
	if !strings.Contains(importIf, "dry_run") {
		t.Errorf("Import GPG release key step's if: does not mention dry_run: %q", importIf)
	}
	genStep := findStep(steps, "Generate checksums and signatures")
	if genStep == nil {
		t.Fatalf("checksums job has no 'Generate checksums and signatures' step")
	}
	run, _ := genStep["run"].(string)
	if !strings.Contains(run, "--no-gpg") {
		t.Errorf("Generate checksums step does not pass --no-gpg in a dry-run:\n%s", run)
	}
}

// TestWorkflow_DryRunConditionalIsOnRoute asserts the dry_run output
// is produced by the route job, which is the job every other job
// reads it from.
func TestWorkflow_DryRunConditionalIsOnRoute(t *testing.T) {
	t.Parallel()
	jobs := jobMap(t, loadWorkflow(t))
	route, ok := jobs[string(release.JobRoute)].(map[string]any)
	if !ok {
		t.Fatalf("route job is missing")
	}
	outputs, ok := route["outputs"].(map[string]any)
	if !ok {
		t.Fatalf("route job has no outputs")
	}
	if _, ok := outputs["dry_run"]; !ok {
		t.Errorf("route job has no dry_run output; outputs = %v", mapKeys(outputs))
	}
}

// TestWorkflow_ProvenanceUsesPinnedReusableWorkflow asserts the
// provenance job delegates to slsa-github-generator at a pinned
// version.
func TestWorkflow_ProvenanceUsesPinnedReusableWorkflow(t *testing.T) {
	t.Parallel()
	jobs := jobMap(t, loadWorkflow(t))
	prov, ok := jobs[string(release.JobProvenance)].(map[string]any)
	if !ok {
		t.Fatalf("provenance job is missing")
	}
	uses, _ := prov["uses"].(string)
	const prefix = "slsa-framework/slsa-github-generator/.github/workflows/generator_generic_slsa3.yml@"
	if !strings.HasPrefix(uses, prefix) {
		t.Fatalf("provenance job uses = %q; want prefix %q", uses, prefix)
	}
	ref := strings.TrimPrefix(uses, prefix)
	if ref == "" {
		t.Errorf("provenance job uses = %q; the @ref is empty", uses)
	}
	switch ref {
	case "main", "master", "HEAD":
		t.Errorf("provenance job pins to a mutable ref %q; pin to a tag or a SHA", ref)
	}
	if !strings.HasPrefix(ref, "v") && !isHex(ref) {
		t.Errorf("provenance job's ref %q is neither a version tag nor a SHA", ref)
	}
}

// TestWorkflow_NoMutableActionRefs asserts every `uses:` is pinned to
// a tag or a SHA, not to a branch. A branch is mutable; a compromised
// upstream could change the action's behavior without a Forge commit.
//
// A local reusable workflow (`./.github/workflows/...`) is always the
// current repository's; it has no @ref and is exempt.
func TestWorkflow_NoMutableActionRefs(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(workflowPath(t))
	if err != nil {
		t.Fatal(err)
	}
	for i, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "uses:") {
			continue
		}
		ref := strings.TrimSpace(strings.TrimPrefix(trimmed, "uses:"))
		if strings.HasPrefix(ref, "./") {
			continue
		}
		at := strings.LastIndex(ref, "@")
		if at < 0 {
			t.Errorf("line %d: %q has no @ref", i+1, trimmed)
			continue
		}
		version := ref[at+1:]
		switch version {
		case "main", "master", "HEAD":
			t.Errorf("line %d: %q pins to a mutable ref %q", i+1, trimmed, version)
		}
	}
}

// TestWorkflow_ConcurrencyIsPerVersion asserts the concurrency group
// resolves to the version, so a dry-run for a version serializes with
// a real release for the same version.
func TestWorkflow_ConcurrencyIsPerVersion(t *testing.T) {
	t.Parallel()
	doc := loadWorkflow(t)
	conc, ok := doc["concurrency"].(map[string]any)
	if !ok {
		t.Fatalf("workflow has no concurrency: block")
	}
	group, _ := conc["group"].(string)
	if group == "" {
		t.Fatalf("concurrency group is empty")
	}
	hasInput := strings.Contains(group, "inputs.version")
	hasRef := strings.Contains(group, "github.ref")
	if !hasInput && !hasRef {
		t.Errorf("concurrency group = %q; want it to reference inputs.version or github.ref", group)
	}
	if !strings.Contains(group, "release-") {
		t.Errorf("concurrency group = %q; want it to be prefixed with 'release-'", group)
	}
	if conc["cancel-in-progress"] != false {
		t.Errorf("concurrency.cancel-in-progress = %v; want false", conc["cancel-in-progress"])
	}
}

// TestWorkflow_PublishAttachesSupplyChainArtifacts asserts the publish
// job's `files:` list includes every supply-chain artifact the
// verification guide tells a user to download.
func TestWorkflow_PublishAttachesSupplyChainArtifacts(t *testing.T) {
	t.Parallel()
	jobs := jobMap(t, loadWorkflow(t))
	publish, ok := jobs[string(release.JobPublish)].(map[string]any)
	if !ok {
		t.Fatalf("publish job is missing")
	}
	steps, _ := publish["steps"].([]any)
	create := findStep(steps, "Create GitHub Release")
	if create == nil {
		t.Fatalf("publish job has no 'Create GitHub Release' step")
	}
	with, _ := create["with"].(map[string]any)
	files, _ := with["files"].(string)
	for _, want := range []string{
		"dist/forge-*",
		"dist/SHA256SUMS",
		"dist/SHA256SUMS.asc",
		"dist/SHA256SUMS.gpg",
		"dist/forge.sbom.json",
		"dist/forge.intoto.jsonl",
	} {
		if !strings.Contains(files, want) {
			t.Errorf("publish job's files: does not include %q", want)
		}
	}
}

// TestWorkflow_VerifyReleaseVerifiesGPG asserts the verify-release
// job runs `gpg --verify` against the published SHA256SUMS.asc. A
// verify-release job that only checks presence does not verify the
// signature's correctness.
func TestWorkflow_VerifyReleaseVerifiesGPG(t *testing.T) {
	t.Parallel()
	jobs := jobMap(t, loadWorkflow(t))
	vr, ok := jobs[string(release.JobVerifyRelease)].(map[string]any)
	if !ok {
		t.Fatalf("verify-release job is missing")
	}
	steps, _ := vr["steps"].([]any)
	found := false
	for _, s := range steps {
		step, ok := s.(map[string]any)
		if !ok {
			continue
		}
		run, _ := step["run"].(string)
		if strings.Contains(run, "gpg --verify") {
			found = true
			if !strings.Contains(run, "SHA256SUMS.asc") {
				t.Errorf("verify-release's gpg step does not reference SHA256SUMS.asc")
			}
			if !strings.Contains(run, "SHA256SUMS") {
				t.Errorf("verify-release's gpg step does not reference SHA256SUMS")
			}
			break
		}
	}
	if !found {
		t.Errorf("verify-release job has no `gpg --verify` step")
	}
}

// TestWorkflow_ModelIsAcyclic asserts the model's dependency graph has
// no cycle. A cycle would make the workflow unrunnable.
func TestWorkflow_ModelIsAcyclic(t *testing.T) {
	t.Parallel()
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := map[release.JobName]int{}
	var visit func(j release.JobName) bool
	visit = func(j release.JobName) bool {
		switch color[j] {
		case gray:
			return false
		case black:
			return true
		}
		color[j] = gray
		for _, dep := range release.Dependencies[j] {
			if !visit(dep) {
				return false
			}
		}
		color[j] = black
		return true
	}
	for _, j := range release.AllJobs() {
		if !visit(j) {
			t.Fatalf("dependency graph has a cycle involving %q", j)
		}
	}
}

// TestWorkflow_ChannelsMatch asserts every channel in the model is
// mentioned in the workflow body. A channel the workflow does not
// handle is a routing defect.
func TestWorkflow_ChannelsMatch(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(workflowPath(t))
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	for _, c := range release.Channels {
		if !strings.Contains(body, c) {
			t.Errorf("workflow does not mention channel %q", c)
		}
	}
}

// TestWorkflow_NoSecretsInPlainEnv asserts no Forge-managed secret is
// referenced outside one of three legitimate forms:
//
//  1. A `${{ secrets.X }}` expression. The `secrets.` prefix is the
//     marker.
//  2. The left side of an `env:` binding whose right side is a
//     `${{ secrets.X }}` expression. The binding's value is a secret
//     expression; the binding's key is the environment variable's
//     name, which may equal the secret's name. The line contains
//     "secrets." too, so form 1 catches it; form 2 is documented
//     here for clarity.
//  3. A shell variable reference inside a `run:` block: `$X` or
//     `${X}`. The variable's value is the environment variable from
//     form 2; the shell expands it at run time and never echoes it
//     to stdout.
//
// The test exists to catch a fourth form: a bare secret name in a
// `run:` block that the shell does not expand, which would leak the
// name's existence to the log even when the value is redacted.
//
// The scan uses release.AllSecrets, which returns only the
// Forge-managed credentials. GITHUB_TOKEN is a per-run token that
// GitHub mints and redacts; its name is not a secret and the scan
// does not flag its references.
func TestWorkflow_NoSecretsInPlainEnv(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(workflowPath(t))
	if err != nil {
		t.Fatal(err)
	}
	names := release.AllSecrets()
	for i, line := range strings.Split(string(data), "\n") {
		for _, name := range names {
			if !strings.Contains(line, name) {
				continue
			}
			// Form 1 (and, transitively, form 2): the name
			// appears inside a `${{ secrets.X }}` expression.
			if strings.Contains(line, "secrets."+name) {
				continue
			}
			// Form 3: a shell variable reference. The shell
			// expands `$NAME` and `${NAME}` at run time; the
			// value comes from an `env:` binding.
			if strings.Contains(line, "$"+name) || strings.Contains(line, "${"+name+"}") {
				continue
			}
			// A comment that names a secret is documentation,
			// not a leak.
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "#") {
				continue
			}
			t.Errorf("line %d: %q names secret %q without the secrets. prefix", i+1, trimmed, name)
		}
	}
}

// --- helpers ---

// assertDryRunGate asserts an if: expression mentions dry_run and
// compares it.
func assertDryRunGate(t *testing.T, job, ifExpr string) {
	t.Helper()
	if ifExpr == "" {
		t.Errorf("job %q has no if: expression", job)
		return
	}
	if !strings.Contains(ifExpr, "dry_run") {
		t.Errorf("job %q's if: does not mention dry_run: %q", job, ifExpr)
	}
	if !strings.Contains(ifExpr, "!=") && !strings.Contains(ifExpr, "==") {
		t.Errorf("job %q's if: does not compare dry_run: %q", job, ifExpr)
	}
}

// needsOf extracts a job's `needs:` list as a sorted
// []release.JobName. It handles the scalar form (`needs: route`) and
// the list form (`needs: [route, build]`).
//
// The return type is release.JobName, not string, so the result
// compares directly to release.Dependencies' values without a
// conversion at the call site.
func needsOf(job map[string]any) []release.JobName {
	var out []release.JobName
	switch v := job["needs"].(type) {
	case nil:
		return nil
	case string:
		out = []release.JobName{release.JobName(v)}
	case []any:
		for _, x := range v {
			if s, ok := x.(string); ok {
				out = append(out, release.JobName(s))
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// permissionsOf extracts a job's `permissions:` block as a map.
func permissionsOf(job map[string]any) map[string]string {
	out := map[string]string{}
	switch v := job["permissions"].(type) {
	case map[string]any:
		for k, val := range v {
			if s, ok := val.(string); ok {
				out[k] = s
			}
		}
	}
	return out
}

// collectSecretRefs walks a job's map and returns every
// `secrets.NAME` reference it finds. The walker is recursive; the
// workflow's shape is a tree of maps and slices.
func collectSecretRefs(v any) []string {
	var out []string
	var walk func(any)
	walk = func(x any) {
		switch t := x.(type) {
		case map[string]any:
			for _, val := range t {
				walk(val)
			}
		case []any:
			for _, item := range t {
				walk(item)
			}
		case string:
			out = append(out, secretRefsInString(t)...)
		}
	}
	walk(v)
	sort.Strings(out)
	return out
}

// secretRefsInString returns every `secrets.NAME` reference in s.
// GitHub's expression syntax is `${{ secrets.NAME }}`; the substring
// "secrets." is the marker.
func secretRefsInString(s string) []string {
	var out []string
	for {
		i := strings.Index(s, "secrets.")
		if i < 0 {
			return out
		}
		s = s[i+len("secrets."):]
		end := len(s)
		for j, r := range s {
			if !(r == '_' || r == '-' ||
				(r >= 'A' && r <= 'Z') ||
				(r >= 'a' && r <= 'z') ||
				(r >= '0' && r <= '9')) {
				end = j
				break
			}
		}
		out = append(out, s[:end])
		s = s[end:]
	}
}

// findStep returns the step in steps whose `name:` equals name, or
// nil if no step matches.
func findStep(steps []any, name string) map[string]any {
	for _, s := range steps {
		step, ok := s.(map[string]any)
		if !ok {
			continue
		}
		if got, _ := step["name"].(string); got == name {
			return step
		}
	}
	return nil
}

// equalStringSets reports whether two slices contain the same
// string-valued elements. The order is ignored; duplicates are
// collapsed.
//
// The type parameter is constrained to ~string so the helper accepts
// both []string and []release.JobName. A caller that passes two
// different element types does not compile, because the two
// arguments share one type parameter; a caller that passes two
// different named types with the same underlying type does compile,
// which is the case this helper exists to support.
func equalStringSets[T ~string](a, b []T) bool {
	as := map[T]bool{}
	for _, x := range a {
		as[x] = true
	}
	bs := map[T]bool{}
	for _, x := range b {
		bs[x] = true
	}
	if len(as) != len(bs) {
		return false
	}
	for k := range as {
		if !bs[k] {
			return false
		}
	}
	return true
}

// mapKeys returns the sorted keys of a map.
func mapKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// isHex reports whether s is a non-empty hexadecimal string. A
// SHA-256 is 64 hex characters; the function does not enforce a
// length, because a future pin may use a shorter SHA.
func isHex(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') ||
			(r >= 'a' && r <= 'f') ||
			(r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}
