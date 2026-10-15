package release_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/thapelomagqazana/forge/internal/release"
)

// TestAllJobs_HasEveryJob asserts the model lists a non-empty set of
// jobs, every name is non-empty, and no name appears twice.
//
// The test deliberately does not assert a count. The model's job list
// grows as the pipeline grows; a count that matches today's pipeline
// is a count that fails tomorrow. The workflow test
// (test/release/workflow_test.go) asserts the set the workflow
// declares; this test asserts the model's own list's shape.
func TestAllJobs_HasEveryJob(t *testing.T) {
	t.Parallel()
	jobs := release.AllJobs()
	if len(jobs) == 0 {
		t.Fatalf("AllJobs() returned no jobs")
	}
	seen := map[release.JobName]bool{}
	for _, j := range jobs {
		if j == "" {
			t.Errorf("AllJobs() contains an empty name")
		}
		if seen[j] {
			t.Errorf("AllJobs() contains %q twice", j)
		}
		seen[j] = true
	}
}

// TestAllJobs_ReturnsCopy asserts AllJobs returns a fresh slice so a
// caller cannot mutate the canonical list.
func TestAllJobs_ReturnsCopy(t *testing.T) {
	t.Parallel()
	a := release.AllJobs()
	a[0] = "mutated"
	b := release.AllJobs()
	if b[0] == "mutated" {
		t.Errorf("AllJobs returns a shared slice")
	}
}

// TestDependencies_CoversEveryJob asserts every job has an entry in
// Dependencies, even if the entry is nil.
//
// The nil entry is meaningful: a job with no dependencies (route)
// and a job whose dependencies are not yet declared are both
// represented. The test asserts the key's presence, not the value.
func TestDependencies_CoversEveryJob(t *testing.T) {
	t.Parallel()
	for _, j := range release.AllJobs() {
		if _, ok := release.Dependencies[j]; !ok {
			t.Errorf("Dependencies has no entry for %q", j)
		}
	}
}

// TestDependencies_NamesAreKnownJobs asserts every dependency names a
// job in AllJobs. A dependency on a job that does not exist is a
// workflow that cannot run.
func TestDependencies_NamesAreKnownJobs(t *testing.T) {
	t.Parallel()
	known := map[release.JobName]bool{}
	for _, j := range release.AllJobs() {
		known[j] = true
	}
	for job, deps := range release.Dependencies {
		for _, d := range deps {
			if !known[d] {
				t.Errorf("job %q depends on %q which is not in AllJobs", job, d)
			}
		}
	}
}

// TestDependencies_NoSelfLoop asserts no job depends on itself. A
// self-loop is a one-node cycle; the acyclicity test
// (TestWorkflow_ModelIsAcyclic in the workflow test package) would
// catch it too, but this test names the failure precisely.
func TestDependencies_NoSelfLoop(t *testing.T) {
	t.Parallel()
	for job, deps := range release.Dependencies {
		for _, d := range deps {
			if d == job {
				t.Errorf("job %q depends on itself", job)
			}
		}
	}
}

// TestDependencies_NoDuplicateEntries asserts no job's dependency
// list names the same job twice. GitHub's workflow parser would
// tolerate a duplicate, but the model's slice is the contract; a
// duplicate is a documentation defect.
func TestDependencies_NoDuplicateEntries(t *testing.T) {
	t.Parallel()
	for job, deps := range release.Dependencies {
		seen := map[release.JobName]bool{}
		for _, d := range deps {
			if seen[d] {
				t.Errorf("job %q lists dependency %q twice", job, d)
			}
			seen[d] = true
		}
	}
}

// TestJobPermissions_CoversEveryJob asserts every job has a
// permissions entry. A job with no permissions entry gets GitHub's
// default, which is broader than any job needs; the model's entry is
// the job's declared subset.
func TestJobPermissions_CoversEveryJob(t *testing.T) {
	t.Parallel()
	for _, j := range release.AllJobs() {
		if _, ok := release.JobPermissions[j]; !ok {
			t.Errorf("JobPermissions has no entry for %q", j)
		}
	}
}

// TestJobPermissions_PublishNeedsWrite asserts the publish job has
// contents:write and packages:write. The two scopes are what the job
// needs to create the Release and push the container image.
func TestJobPermissions_PublishNeedsWrite(t *testing.T) {
	t.Parallel()
	p := release.JobPermissions[release.JobPublish]
	if p["contents"] != "write" {
		t.Errorf("publish contents = %q; want write", p["contents"])
	}
	if p["packages"] != "write" {
		t.Errorf("publish packages = %q; want write", p["packages"])
	}
}

// TestJobPermissions_SignNeedsOIDC asserts the sign job has
// id-token:write, which cosign keyless requires. Without it, cosign
// cannot obtain a Fulcio certificate and the archive is unsigned.
func TestJobPermissions_SignNeedsOIDC(t *testing.T) {
	t.Parallel()
	p := release.JobPermissions[release.JobSign]
	if p["id-token"] != "write" {
		t.Errorf("sign id-token = %q; want write", p["id-token"])
	}
}

// TestJobPermissions_NonSigningJobsHaveNoOIDC asserts no job other
// than sign and provenance has id-token:write.
//
// The two jobs that need the OIDC token are the two that use it:
// sign (cosign keyless) and provenance (slsa-github-generator). A
// compromised step in any other job cannot mint a token for the
// repository's identity; the per-job permission is the enforcement.
func TestJobPermissions_NonSigningJobsHaveNoOIDC(t *testing.T) {
	t.Parallel()
	for _, j := range release.AllJobs() {
		if j == release.JobSign || j == release.JobProvenance {
			continue
		}
		if release.JobPermissions[j]["id-token"] == "write" {
			t.Errorf("job %q has id-token:write but does not need it", j)
		}
	}
}

// TestJobPermissions_NoWriteScopeBeyondPublish asserts the only job
// with contents:write is publish and provenance. The two jobs are the
// two that write to the repository: publish creates the Release,
// provenance writes the attestation.
//
// A future job that needed contents:write would be added to the
// allow-list; the test's name says what the rule is, and the
// allow-list is the rule's exceptions.
func TestJobPermissions_NoWriteScopeBeyondPublish(t *testing.T) {
	t.Parallel()
	allowed := map[release.JobName]bool{
		release.JobPublish:    true,
		release.JobProvenance: true,
	}
	for _, j := range release.AllJobs() {
		if allowed[j] {
			continue
		}
		if release.JobPermissions[j]["contents"] == "write" {
			t.Errorf("job %q has contents:write but is not an allow-listed job", j)
		}
	}
}

// TestJobSecrets_CoversEveryJob asserts every job has a JobSecrets
// entry, possibly nil. The nil entry is meaningful: a job that reads
// no signing credential has an empty list, not a missing key.
func TestJobSecrets_CoversEveryJob(t *testing.T) {
	t.Parallel()
	for _, j := range release.AllJobs() {
		if _, ok := release.JobSecrets[j]; !ok {
			t.Errorf("JobSecrets has no entry for %q", j)
		}
	}
}

// TestJobSecrets_OnlyWritingJobsReadCredentials asserts the only jobs
// with a non-empty JobSecrets entry are the jobs that write to a
// place the workflow's own token does not authorize:
//
//   - checksums writes the checksum files and the GPG signature.
//   - sign writes the cosign signatures and the platform signatures.
//   - distribute writes to the channel repositories (the tap, the
//     bucket, the apt repo, the winget fork).
//
// Every other job's JobSecrets entry is nil. The invariant is what
// JobSecrets means: a credential that authorizes a write. A job that
// does not write to a place Forge does not own has no such
// credential.
//
// The test does not check JobBuiltinTokens or JobPublicIdentifiers: a
// per-run GitHub token and a public key's fingerprint are not
// credentials in this sense, and a job that reads one is not a
// writing job by virtue of that read.
func TestJobSecrets_OnlyWritingJobsReadCredentials(t *testing.T) {
	t.Parallel()
	allowed := map[release.JobName]bool{
		release.JobChecksums:  true,
		release.JobSign:       true,
		release.JobDistribute: true,
	}
	for _, j := range release.AllJobs() {
		if allowed[j] {
			continue
		}
		if len(release.JobSecrets[j]) > 0 {
			t.Errorf("job %q reads secrets %v but is not a writing job", j, release.JobSecrets[j])
		}
	}
}

// TestJobBuiltinTokens_CoversEveryJob asserts every job has a
// JobBuiltinTokens entry, possibly nil.
func TestJobBuiltinTokens_CoversEveryJob(t *testing.T) {
	t.Parallel()
	for _, j := range release.AllJobs() {
		if _, ok := release.JobBuiltinTokens[j]; !ok {
			t.Errorf("JobBuiltinTokens has no entry for %q", j)
		}
	}
}

// TestJobPublicIdentifiers_CoversEveryJob asserts every job has a
// JobPublicIdentifiers entry, possibly nil.
func TestJobPublicIdentifiers_CoversEveryJob(t *testing.T) {
	t.Parallel()
	for _, j := range release.AllJobs() {
		if _, ok := release.JobPublicIdentifiers[j]; !ok {
			t.Errorf("JobPublicIdentifiers has no entry for %q", j)
		}
	}
}

// TestJobPublicIdentifiers_IsSubsetOfAllSecrets asserts every public
// identifier appears in AllSecrets. The public identifier is a value
// Forge manages (the fingerprint rotates with the key), so it belongs
// in SECURITY.md's credential table and in AllSecrets.
func TestJobPublicIdentifiers_IsSubsetOfAllSecrets(t *testing.T) {
	t.Parallel()
	all := map[string]bool{}
	for _, s := range release.AllSecrets() {
		all[s] = true
	}
	for job, ids := range release.JobPublicIdentifiers {
		for _, id := range ids {
			if !all[id] {
				t.Errorf("job %q's public identifier %q is not in AllSecrets", job, id)
			}
		}
	}
}

// TestAllSecrets_IsSortedAndUnique asserts AllSecrets returns a
// sorted, deduplicated list. The function's contract is a
// deterministic list; a caller that compares two runs' output
// depends on it.
func TestAllSecrets_IsSortedAndUnique(t *testing.T) {
	t.Parallel()
	got := release.AllSecrets()
	if !sort.StringsAreSorted(got) {
		t.Errorf("AllSecrets is not sorted: %v", got)
	}
	seen := map[string]bool{}
	for _, s := range got {
		if seen[s] {
			t.Errorf("AllSecrets contains %q twice", s)
		}
		seen[s] = true
	}
}

// TestAllSecrets_MatchesJobSecrets asserts AllSecrets is the union of
// JobSecrets and JobPublicIdentifiers: every value Forge manages,
// deduplicated.
//
// Built-in tokens are excluded. GITHUB_TOKEN is minted per run by
// GitHub; there is nothing for Forge to rotate, and SECURITY.md's
// credential table is not the place to document it.
func TestAllSecrets_MatchesJobSecrets(t *testing.T) {
	t.Parallel()
	want := map[string]bool{}
	for _, secrets := range release.JobSecrets {
		for _, s := range secrets {
			want[s] = true
		}
	}
	for _, ids := range release.JobPublicIdentifiers {
		for _, s := range ids {
			want[s] = true
		}
	}

	got := map[string]bool{}
	for _, s := range release.AllSecrets() {
		got[s] = true
	}

	for s := range want {
		if !got[s] {
			t.Errorf("AllSecrets is missing %q", s)
		}
	}
	for s := range got {
		if !want[s] {
			t.Errorf("AllSecrets has extra %q", s)
		}
	}
}

// TestAllJobSecrets_IsSortedAndUnique asserts AllJobSecrets returns a
// sorted, deduplicated list.
func TestAllJobSecrets_IsSortedAndUnique(t *testing.T) {
	t.Parallel()
	got := release.AllJobSecrets()
	if !sort.StringsAreSorted(got) {
		t.Errorf("AllJobSecrets is not sorted: %v", got)
	}
	seen := map[string]bool{}
	for _, s := range got {
		if seen[s] {
			t.Errorf("AllJobSecrets contains %q twice", s)
		}
		seen[s] = true
	}
}

// TestAllJobSecrets_MatchesAllMaps asserts AllJobSecrets is the union
// of all three maps: every value the workflow reads from the `secrets`
// context, deduplicated.
//
// The union differs from AllSecrets by JobBuiltinTokens. A caller who
// wants "everything the workflow reads" uses this function; a caller
// who wants "everything Forge manages" uses AllSecrets.
func TestAllJobSecrets_MatchesAllMaps(t *testing.T) {
	t.Parallel()
	want := map[string]bool{}
	for _, secrets := range release.JobSecrets {
		for _, s := range secrets {
			want[s] = true
		}
	}
	for _, ids := range release.JobPublicIdentifiers {
		for _, s := range ids {
			want[s] = true
		}
	}
	for _, tokens := range release.JobBuiltinTokens {
		for _, s := range tokens {
			want[s] = true
		}
	}

	got := map[string]bool{}
	for _, s := range release.AllJobSecrets() {
		got[s] = true
	}

	for s := range want {
		if !got[s] {
			t.Errorf("AllJobSecrets is missing %q", s)
		}
	}
	for s := range got {
		if !want[s] {
			t.Errorf("AllJobSecrets has extra %q", s)
		}
	}
}

// TestAllSecrets_IsSubsetOfAllJobSecrets asserts AllSecrets is a
// subset of AllJobSecrets. The former is "what Forge manages"; the
// latter is "what the workflow reads." The former is the latter minus
// the built-in tokens.
func TestAllSecrets_IsSubsetOfAllJobSecrets(t *testing.T) {
	t.Parallel()
	all := map[string]bool{}
	for _, s := range release.AllJobSecrets() {
		all[s] = true
	}
	for _, s := range release.AllSecrets() {
		if !all[s] {
			t.Errorf("AllSecrets contains %q which AllJobSecrets does not", s)
		}
	}
}

// TestDefaultTrigger asserts the default trigger: a v* push and a
// manual workflow_dispatch with the dry_run input.
func TestDefaultTrigger(t *testing.T) {
	t.Parallel()
	tr := release.DefaultTrigger()
	if tr.PushTags != "v*" {
		t.Errorf("PushTags = %q; want v*", tr.PushTags)
	}
	if !tr.WorkflowDispatch {
		t.Errorf("WorkflowDispatch = false; want true")
	}
	if tr.DryRunInput != "dry_run" {
		t.Errorf("DryRunInput = %q; want dry_run", tr.DryRunInput)
	}
}

// TestDefaultDryRunContract asserts the dry-run's contract: it skips
// the Release, skips the provenance upload, and uploads every
// artifact as a workflow artifact.
func TestDefaultDryRunContract(t *testing.T) {
	t.Parallel()
	c := release.DefaultDryRunContract()
	if !c.SkipsPublish {
		t.Errorf("SkipsPublish = false; want true")
	}
	if !c.SkipsProvenanceUpload {
		t.Errorf("SkipsProvenanceUpload = false; want true")
	}
	if !c.UploadsArtifacts {
		t.Errorf("UploadsArtifacts = false; want true")
	}
}

// TestIsValidChannel asserts the channel predicate: every channel is
// valid; a string that is not one of the three is not.
func TestIsValidChannel(t *testing.T) {
	t.Parallel()
	for _, c := range release.Channels {
		if !release.IsValidChannel(c) {
			t.Errorf("IsValidChannel(%q) = false; want true", c)
		}
	}
	for _, c := range []string{"", "STABLE", "beta", "release"} {
		if release.IsValidChannel(c) {
			t.Errorf("IsValidChannel(%q) = true; want false", c)
		}
	}
}

// TestChannels_AreExactlyThree asserts the channel set is exactly
// {stable, prerelease, nightly}. A fourth channel requires an ADR and
// an update to docs/reference/release-channels.md; the test is the
// model's enforcement of that rule.
func TestChannels_AreExactlyThree(t *testing.T) {
	t.Parallel()
	if len(release.Channels) != 3 {
		t.Errorf("len(Channels) = %d; want 3", len(release.Channels))
	}
	joined := strings.Join(release.Channels, ",")
	for _, want := range []string{"stable", "prerelease", "nightly"} {
		if !strings.Contains(joined, want) {
			t.Errorf("Channels does not contain %q", want)
		}
	}
}

// TestChannels_AreUnique asserts no channel appears twice. A
// duplicate is a documentation defect; the set's size is fixed by the
// test above, and a duplicate would mean a missing channel.
func TestChannels_AreUnique(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for _, c := range release.Channels {
		if seen[c] {
			t.Errorf("Channels contains %q twice", c)
		}
		seen[c] = true
	}
}

// TestChannels_AreSortedWithinTheirContract asserts the channels are
// listed in the order the model documents: stable, prerelease,
// nightly. The order is the order the tests and the documentation
// present them; a reordering is a documentation change.
func TestChannels_AreSortedWithinTheirContract(t *testing.T) {
	t.Parallel()
	want := []string{"stable", "prerelease", "nightly"}
	if len(release.Channels) != len(want) {
		t.Fatalf("len(Channels) = %d; want %d", len(release.Channels), len(want))
	}
	for i, c := range release.Channels {
		if c != want[i] {
			t.Errorf("Channels[%d] = %q; want %q", i, c, want[i])
		}
	}
}
