// Package release contains the supply-chain integrity pipeline's
// shared contract: the artifact-naming convention, the dist/ layout,
// the file-mode constants the drivers share, and the release
// workflow's job graph.
//
// # Why a shared package
//
// The archive name (forge-<goos>-<goarch>.<ext>) appears in the
// builder, the verifier, and the three supply-chain drivers. The
// naming convention lives here so a change to it touches one package,
// not four.
//
// # Why the workflow lives here too
//
// The release workflow's job graph is a contract: which jobs exist,
// which depend on which, which permissions each needs, which values
// each reads from the `secrets` context. The contract is expressed as
// Go values in this package so that a test can assert the workflow
// YAML satisfies it. A YAML file is not unit-testable; a Go model of
// its contract is. The model is the source of truth; the YAML is the
// implementation; test/release/workflow_test.go keeps the two in
// sync.
//
// # Classification
//
// This package is in the architecture.release group
// (docs/architecture/dependency-matrix.yaml). It is build-time
// metadata: it is imported by the three cmd/forge-internal-* drivers,
// by cmd/forge-internal-distribute, and by test/release, and by no
// production package. A production package that needs an artifact's
// name computes it from its own inputs; it does not import this
// package. The dependency-matrix's closed-world default denies every
// production group access to this group.
//
// # Three maps for three kinds of value
//
// The `secrets` context holds three kinds of value, and the rule
// about who may read each kind differs:
//
//   - JobSecrets: a signing credential. A private key, a
//     certificate, an OIDC identity, a notary profile, a token that
//     authorizes a write to a repository Forge does not own. It
//     authorizes a signature or a push. Only the jobs that sign or
//     push may read it.
//
//   - JobBuiltinTokens: a per-run token that GitHub mints. It
//     authenticates a tool call to the GitHub API. Only the jobs that
//     call such a tool may read it.
//
//   - JobPublicIdentifiers: a published value that the workflow
//     happens to store in the `secrets` context for convenience. The
//     GPG key's fingerprint is in SECURITY.md and on
//     keys.openpgp.org; storing it in a secret gives the workflow one
//     place to read it. Reading it does not authorize a signature.
//     Any job that needs to name the value may read it.
//
// Three maps express the three kinds. A reader who asks "what does
// Forge manage?" gets AllSecrets (the union of JobSecrets and
// JobPublicIdentifiers). A reader who asks "what does the workflow
// read?" gets AllJobSecrets (the union of all three maps). A reader
// who asks "which jobs sign or push?" gets the jobs with a non-empty
// JobSecrets entry. The three questions have three answers; the three
// maps make each answer expressible.
package release

import "sort"

// JobName is the name of a job in the release workflow. It is a
// string type so a typo is caught by the compiler rather than by a
// test.
type JobName string

// The jobs in the release workflow, in the order a reader should
// understand them. The order is not the execution order: the
// execution order is the dependency graph, which the Dependencies
// map encodes.
const (
	// JobRoute determines the release channel from the tag. It is
	// the only job that reads GITHUB_REF_NAME. Every other job
	// consumes its output.
	JobRoute JobName = "route"

	// JobBuild cross-compiles the Tier 1 and Tier 2 platforms. It
	// delegates to the reusable build workflow.
	JobBuild JobName = "build"

	// JobVerify runs the binary smoke suite (WBS 17.5.1) against
	// the freshly built artifacts. A broken binary is not signed.
	JobVerify JobName = "verify"

	// JobChecksums produces the per-binary .sha256 files, the
	// combined SHA256SUMS, and the GPG signatures (WBS 17.5.2).
	JobChecksums JobName = "checksums"

	// JobSBOM produces the per-binary CycloneDX SBOMs and the
	// combined index (WBS 17.5.2).
	JobSBOM JobName = "sbom"

	// JobSign produces the cosign signatures over every archive,
	// and (where the credentials are available) the macOS
	// notarization and the Windows Authenticode signatures
	// (WBS 17.5.2).
	JobSign JobName = "sign"

	// JobProvenance produces the SLSA Level 3 attestation via
	// slsa-github-generator (WBS 17.5.2).
	JobProvenance JobName = "provenance"

	// JobPublish creates the GitHub Release and attaches every
	// artifact. It is the first job that writes to GitHub.
	JobPublish JobName = "publish"

	// JobDistribute updates every distribution channel for the
	// release (WBS 17.5.4). It renders each channel's manifest and
	// commits it to the channel's repository. It runs after
	// publish, because a channel's manifest names the release's
	// artifacts, which exist only after the Release is created.
	//
	// The job runs for stable and prerelease releases; a nightly
	// is not published to any package manager, so the job is
	// skipped for a nightly. The job is also skipped in a
	// dry-run: a dry-run produces no Release to distribute.
	JobDistribute JobName = "distribute"

	// JobVerifyRelease runs the post-publish assertions: the GPG
	// signature over SHA256SUMS, the channel-distribution
	// invariant, and the artifact-presence check. It is the last
	// job; a failure here means the release exists but is
	// incomplete, and the rollback guide applies.
	JobVerifyRelease JobName = "verify-release"
)

// AllJobs returns every job in the workflow. The order is the order a
// reader should understand them, not the execution order.
//
// The function returns a fresh slice on each call so a caller cannot
// mutate the canonical list.
func AllJobs() []JobName {
	return []JobName{
		JobRoute,
		JobBuild,
		JobVerify,
		JobChecksums,
		JobSBOM,
		JobSign,
		JobProvenance,
		JobPublish,
		JobDistribute,
		JobVerifyRelease,
	}
}

// Dependencies maps each job to the jobs it needs. A job's `needs:`
// list in the workflow must equal this map's value for the job.
//
// The graph is acyclic. The test asserts that; a cycle would make the
// workflow unrunnable and is a defect.
//
// The shape encodes four design decisions:
//
//   - verify comes before checksums and sign, so a broken binary is
//     not signed.
//   - checksums comes before sign, so the cosign signature is over
//     the final archive.
//   - publish comes after every supply-chain job, so the Release
//     attaches every artifact.
//   - distribute comes after publish, because a channel's manifest
//     names the release's artifacts, which exist only after the
//     Release is created.
var Dependencies = map[JobName][]JobName{
	JobRoute:         nil,
	JobBuild:         {JobRoute},
	JobVerify:        {JobRoute, JobBuild},
	JobChecksums:     {JobRoute, JobVerify},
	JobSBOM:          {JobRoute, JobVerify},
	JobSign:          {JobRoute, JobVerify, JobChecksums},
	JobProvenance:    {JobRoute, JobBuild},
	JobPublish:       {JobRoute, JobBuild, JobChecksums, JobSBOM, JobSign, JobProvenance},
	JobDistribute:    {JobRoute, JobPublish},
	JobVerifyRelease: {JobRoute, JobPublish},
}

// Permissions is the set of GitHub Actions permissions a job
// declares. The workflow's `permissions:` block at the job level must
// equal this map's value for the job.
//
// A job that declares fewer permissions than it needs fails at run
// time; a job that declares more is a violation of least privilege
// and fails the test. The values are the canonical permission names:
// "contents", "packages", "id-token", "actions".
type Permissions map[string]string

// JobPermissions maps each job to the permissions it needs.
//
// The workflow's top-level `permissions:` block grants the minimum;
// each job declares its own subset. The top-level block is read-only,
// so a job that declares no permissions gets only read access. A job
// that needs write access (publish, provenance) or an OIDC token
// (sign, provenance) declares it explicitly.
//
// The split is the security rationale for per-job permissions: a
// compromised step in checksums cannot mint an OIDC token, because
// the job does not declare id-token: write.
//
// The distribute job declares contents: read. It checks out its own
// repository (the read is what the checkout action needs) and pushes
// to the channel repositories with per-channel tokens, not with the
// workflow's token. The per-channel pushes are authenticated by the
// FORGE_*_TOKEN secrets, which are JobSecrets entries; the workflow's
// contents: read does not authorize them.
var JobPermissions = map[JobName]Permissions{
	JobRoute:         {"contents": "read"},
	JobBuild:         {"contents": "read"},
	JobVerify:        {"contents": "read"},
	JobChecksums:     {"contents": "read"},
	JobSBOM:          {"contents": "read"},
	JobSign:          {"contents": "read", "id-token": "write"},
	JobProvenance:    {"contents": "write", "id-token": "write", "actions": "read"},
	JobPublish:       {"contents": "write", "packages": "write"},
	JobDistribute:    {"contents": "read"},
	JobVerifyRelease: {"contents": "read"},
}

// JobSecrets maps each job to the Forge-managed signing credentials
// it reads. A signing credential is a value whose possession
// authorizes a signature or a write to a repository Forge does not
// own: a private key, a certificate, an OIDC identity, a notary
// profile, a repository-scoped token.
//
// The jobs that sign are checksums (GPG) and sign (cosign keyless,
// Apple notarization, Windows Authenticode). The job that writes to
// the channel repositories is distribute (the four FORGE_*_TOKEN
// secrets, each scoped to one channel's repository).
//
// A job that verifies a signature reads a public identifier
// (JobPublicIdentifiers), not a signing credential, and does not
// appear in this map.
//
// The values are the secret names, not the environment variables the
// workflow sets from them. A credential may be read by more than one
// job; the same name appears in each job's slice.
var JobSecrets = map[JobName][]string{
	JobRoute:      nil,
	JobBuild:      nil,
	JobVerify:     nil,
	JobChecksums:  {"FORGE_GPG_PRIVATE_KEY"},
	JobSBOM:       nil,
	JobSign:       {"FORGE_CODESIGN_IDENTITY", "FORGE_NOTARY_KEYCHAIN_PROFILE", "FORGE_AUTHENTICODE_SUBJECT", "FORGE_AUTHENTICODE_SHA1"},
	JobProvenance: nil,
	JobPublish:    nil,
	// The four channel tokens are Forge-managed: Forge creates them,
	// scopes them to one repository each, and rotates them. They are
	// not GitHub's per-run token (GITHUB_TOKEN, in
	// JobBuiltinTokens) and they are not public identifiers
	// (FORGE_GPG_KEY_FINGERPRINT, in JobPublicIdentifiers).
	JobDistribute:    {"FORGE_TAP_TOKEN", "FORGE_BUCKET_TOKEN", "FORGE_APT_TOKEN", "FORGE_WINGET_TOKEN"},
	JobVerifyRelease: nil,
}

// JobBuiltinTokens maps each job to the GitHub-provided tokens it
// reads from the `secrets` context. A builtin token is not a
// Forge-managed credential: GitHub mints it per run, and there is
// nothing for Forge to rotate. It authenticates a tool call to the
// GitHub API.
//
// The map is separate from JobSecrets so that:
//
//   - AllSecrets returns only the credentials SECURITY.md documents,
//     and
//   - the workflow test can assert every `secrets.*` read, including
//     the built-in tokens.
//
// The only builtin token today is GITHUB_TOKEN. A future workflow
// that used a different GitHub-provided token (for example,
// ACTIONS_RUNTIME_TOKEN) would add it here.
//
// The distribute job reads GITHUB_TOKEN to authenticate the Docker
// push to ghcr.io; the verify-release job reads it to authenticate
// the gh release download. Neither job rotates it; GitHub mints it
// per run.
var JobBuiltinTokens = map[JobName][]string{
	JobRoute:         nil,
	JobBuild:         nil,
	JobVerify:        nil,
	JobChecksums:     nil,
	JobSBOM:          nil,
	JobSign:          nil,
	JobProvenance:    nil,
	JobPublish:       nil,
	JobDistribute:    {"GITHUB_TOKEN"},
	JobVerifyRelease: {"GITHUB_TOKEN"},
}

// JobPublicIdentifiers maps each job to the non-secret identifiers it
// reads from the `secrets` context. A public identifier is a value
// that is published — the GPG key's fingerprint is in SECURITY.md and
// on keys.openpgp.org — but is stored in a GitHub secret so the
// workflow has one place to read it.
//
// It is not a signing credential: reading it does not authorize a
// signature; it identifies a key whose public half anyone can fetch.
// A job that reads a public identifier is not a signing job by virtue
// of that read.
//
// The map is separate from JobSecrets so that the "only signing jobs
// read signing credentials" invariant is precise.
//
//   - checksums reads the fingerprint to identify the key whose
//     private half it uses; the job signs, so its JobSecrets entry
//     is non-empty, and the fingerprint is an additional read.
//   - distribute reads the fingerprint to embed it in the apt
//     adapter's metadata; the job does not sign, and the fingerprint
//     is the only public identifier it reads.
//   - verify-release reads the fingerprint to fetch the public key
//     and run gpg --verify; the job does not sign.
//
// The only public identifier today is FORGE_GPG_KEY_FINGERPRINT. A
// future workflow that read a public key's identifier from the
// secrets context would add it here.
var JobPublicIdentifiers = map[JobName][]string{
	JobRoute:         nil,
	JobBuild:         nil,
	JobVerify:        nil,
	JobChecksums:     {"FORGE_GPG_KEY_FINGERPRINT"},
	JobSBOM:          nil,
	JobSign:          nil,
	JobProvenance:    nil,
	JobPublish:       nil,
	JobDistribute:    {"FORGE_GPG_KEY_FINGERPRINT"},
	JobVerifyRelease: {"FORGE_GPG_KEY_FINGERPRINT"},
}

// AllSecrets returns every Forge-managed value the pipeline reads,
// deduplicated and sorted. The list is the input to SECURITY.md's
// credential table; the two must agree.
//
// The union is JobSecrets and JobPublicIdentifiers: a signing
// credential or a channel token is Forge's to rotate, and so is the
// fingerprint that identifies the key it belongs to. A key rotation
// changes both.
//
// Built-in tokens are excluded: they are not Forge's to rotate, and
// the security document's table is a list of values Forge manages.
// The workflow test uses JobBuiltinTokens directly to assert the
// built-in reads.
func AllSecrets() []string {
	seen := map[string]bool{}
	var out []string
	add := func(names []string) {
		for _, s := range names {
			if seen[s] {
				continue
			}
			seen[s] = true
			out = append(out, s)
		}
	}
	for _, secrets := range JobSecrets {
		add(secrets)
	}
	for _, ids := range JobPublicIdentifiers {
		add(ids)
	}
	sort.Strings(out)
	return out
}

// AllJobSecrets returns every value a job reads from the `secrets`
// context, deduplicated and sorted. The function is the union of the
// three maps; the workflow test asserts against it.
//
// The function is not the input to SECURITY.md; AllSecrets is. The
// two exist so that a caller who wants "everything the workflow
// reads" and a caller who wants "everything Forge manages" each get
// the right answer.
func AllJobSecrets() []string {
	seen := map[string]bool{}
	var out []string
	add := func(names []string) {
		for _, s := range names {
			if seen[s] {
				continue
			}
			seen[s] = true
			out = append(out, s)
		}
	}
	for _, secrets := range JobSecrets {
		add(secrets)
	}
	for _, ids := range JobPublicIdentifiers {
		add(ids)
	}
	for _, tokens := range JobBuiltinTokens {
		add(tokens)
	}
	sort.Strings(out)
	return out
}

// Trigger is the workflow's trigger contract. The test asserts the
// workflow's `on:` block matches it.
type Trigger struct {
	// PushTags is the glob the workflow triggers on. It must be
	// exactly "v*": a tag that does not match is not a release.
	PushTags string

	// WorkflowDispatch enables the manual trigger. It is required
	// for the dry-run.
	WorkflowDispatch bool

	// DryRunInput is the name of the boolean input that selects the
	// dry-run. It must be "dry_run".
	DryRunInput string
}

// DefaultTrigger returns the trigger the workflow must declare.
func DefaultTrigger() Trigger {
	return Trigger{
		PushTags:         "v*",
		WorkflowDispatch: true,
		DryRunInput:      "dry_run",
	}
}

// DryRunContract describes what a dry-run does and does not do. The
// test asserts the workflow's conditional expressions match it.
//
// A dry-run produces the full artifact set in the job's workspace and
// uploads it as a workflow artifact, but it does not create a GitHub
// Release, does not push to a registry, and does not publish a
// provenance attestation to the transparency log. The first is the
// dry-run's product; the last three are the release's side effects.
//
// The distribute job is skipped in a dry-run for the same reason the
// publish job is: a dry-run produces no Release, and a channel's
// manifest names the Release's artifacts.
type DryRunContract struct {
	// SkipsPublish is true: a dry-run does not create a GitHub
	// Release.
	SkipsPublish bool

	// SkipsProvenanceUpload is true: a dry-run does not upload the
	// SLSA attestation to the transparency log. The attestation is
	// still generated; it is uploaded as a workflow artifact.
	SkipsProvenanceUpload bool

	// UploadsArtifacts is true: a dry-run uploads every artifact as
	// a workflow artifact so a reviewer can inspect the set.
	UploadsArtifacts bool
}

// DefaultDryRunContract returns the contract a dry-run must satisfy.
func DefaultDryRunContract() DryRunContract {
	return DryRunContract{
		SkipsPublish:          true,
		SkipsProvenanceUpload: true,
		UploadsArtifacts:      true,
	}
}

// Channels are the release channels a tag may route to. The test
// asserts the route job's channel check matches this set.
//
// The three channels are the closed set the route job accepts; a tag
// that matches no channel is rejected. A fourth channel requires an
// ADR and an update to docs/reference/release-channels.md.
var Channels = []string{"stable", "prerelease", "nightly"}

// IsValidChannel reports whether s is one of the channels.
func IsValidChannel(s string) bool {
	for _, c := range Channels {
		if c == s {
			return true
		}
	}
	return false
}
