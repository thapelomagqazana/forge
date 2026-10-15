<!--
  Release process
  ===============
  Normative reference. The source of truth for:
    - the release workflow's job graph,
    - the credentials the pipeline reads,
    - the channel-to-distribution mapping,
    - the dry-run's semantics,
    - the rollback trigger.

  Normative references:
    - Versioning:             docs/reference/versioning.md
    - Release channels:       docs/reference/release-channels.md
    - Verifying releases:     docs/reference/verifying-releases.md
    - Supply chain ADR:       docs/decisions/ADR-048-supply-chain-integrity.md
    - Security policy:        SECURITY.md
    - Rollback guide:         docs/guides/rollback.md
    - Dry-run guide:          docs/guides/release-dry-run.md
    - Verifying binaries:     docs/guides/verifying-binaries.md

  Editing rules:
    1. The job graph in this document MUST match
       scripts/internal/release/workflow.go. The Go model is the
       source of truth; this document copies. test/release/
       workflow_test.go fails if the two diverge.
    2. The credentials in this document MUST match
       scripts/internal/release/workflow.go and SECURITY.md. The
       three are one contract.
    3. The channel table MUST match
       docs/reference/release-channels.md.
    4. Every job name in the tables MUST match the `jobs:` keys in
       .github/workflows/release.yml. A rename in the workflow that
       is not reflected here is a defect.
    5. The document MUST NOT instruct a reader to run a command
       that requires a credential the reader does not have. A
       command that requires a credential is marked as such.
-->

# Release process

A Forge release is produced by a single GitHub Actions workflow:
[`.github/workflows/release.yml`](../../.github/workflows/release.yml).
The workflow is triggered by a push of a tag matching `v*`, or by a
manual `workflow_dispatch` for a dry-run.

This document is the contract. The workflow is its implementation.
is the machine-readable form of the contract;
[`test/release/workflow_test.go`](../../test/release/workflow_test.go)
asserts the workflow satisfies it.

---

## The job graph

| Job | Needs | Runs in dry-run | Purpose |
|---|---|---|---|
| `route` | — | Yes | Determine the channel from the tag. The only job that reads the tag. |
| `build` | `route` | Yes | Cross-compile the Tier 1 and Tier 2 platforms. Delegates to `build.yml`. Exposes `hashes`. |
| `verify` | `route`, `build` | Yes | Run the binary smoke suite (WBS 17.5.1). A broken binary is not signed. |
| `checksums` | `route`, `verify` | Yes | Per-binary `.sha256`, `SHA256SUMS`, `SHA256SUMS.asc`, `SHA256SUMS.gpg` (WBS 17.5.2). |
| `sbom` | `route`, `verify` | Yes | Per-binary CycloneDX 1.5 SBOMs and `forge.sbom.json` (WBS 17.5.2). |
| `sign` | `route`, `verify`, `checksums` | Yes | cosign keyless, macOS notarization, Windows Authenticode (WBS 17.5.2). |
| `provenance` | `route`, `build` | Yes | SLSA Level 3 attestation via `slsa-github-generator` (WBS 17.5.2). |
| `publish` | `route`, `build`, `checksums`, `sbom`, `sign`, `provenance` | **No** | Create the GitHub Release, attach every artifact. |
| `verify-release` | `route`, `publish` | **No** | Post-publish assertions. |

The graph is acyclic. `scripts/internal/release/workflow.go`
encodes the edges in its `Dependencies` map;
`test/release/workflow_test.go` asserts the workflow's `needs:`
lists match.

### The two gates

Exactly two jobs have an `if:` that skips them in a dry-run:
`publish` and `verify-release`. Both check
`needs.route.outputs.dry_run != '1'`. The `checksums` job has a
third gate: the GPG key is imported only when
`dry_run != '1'`. Every other job runs unchanged in both modes.

This is the design's central invariant: a dry-run and a real
release execute the same pipeline, and the difference between them
is two `if:` expressions and one key-import guard. A change to the
pipeline automatically applies to both modes; a reviewer who reads
the pipeline sees the dry-run's behavior in the same file.

### Why this order

- **`verify` before `checksums` and `sign`.** A broken binary is
  not signed. Signing a broken binary wastes the signing
  credentials and publishes a signature over an artifact that will
  be withdrawn. The order makes the failure cheap.
- **`checksums` before `sign`.** The cosign signature is over the
  final archive. The `checksums` job does not modify the archives
  (it only reads them), but the graph must not depend on that
  invariant. A future change that made `checksums` rewrite an
  archive would leave the signature over a stale artifact if the
  order were reversed.
- **`provenance` after `build`, before `publish`.** The attestation
  describes what was built. It does not depend on the signatures;
  it depends on the build's subject hashes. Running it in parallel
  with `checksums` and `sign` is correct; the graph encodes that by
  making `provenance` need only `route` and `build`.
- **`publish` after every supply-chain job.** The Release must
  attach every artifact. A Release that is missing
  `forge.intoto.jsonl` is incomplete, and a user who follows the
  verification guide would find a missing step.

---

## The channel mapping

The `route` job computes the channel from the tag via
`cmd/forge-internal-version channel`. The channel determines what
`publish` does.

| Channel | Tag pattern | Published to | Signed | Marked prerelease | Deleted after |
|---|---|---|---|---|---|
| `stable` | `vX.Y.Z` | All channels: direct download, container image, Homebrew, Scoop, winget, apt | Yes | No | Never |
| `prerelease` | `vX.Y.Z-rc.N`, `vX.Y.Z-beta.N`, `vX.Y.Z-alpha.N` | Direct download, container image | Yes | Yes | Never |
| `nightly` | `vX.Y.Z-nightly.NNNNNNNN` | Direct download | No | Yes | 30 days |

The workflow's `route` job rejects a tag that matches no channel: a
tag that is not a release must not produce a release.

### Why nightly is unsigned

A nightly build is a snapshot of `main`. It is published for
testing and is deleted after 30 days. Signing a nightly would
require the same credentials a stable release uses, which would
mean every nightly exercises the release key. The release key's
rotation policy assumes the key is used for stable and
prerelease releases only; a nightly that exercised it would
expand the key's exposure for no user benefit, because a nightly
is not supported.

A user who wants a signed artifact uses a stable or a prerelease
release. The verification guide's steps apply to those; a nightly's
verification is the SHA-256 checksum, which is still produced.

---

## The credentials

The pipeline reads five credentials. Each is stored in a GitHub
Actions secret; each is read only by the job that needs it.

| Credential | Secret | Read by | Purpose |
|---|---|---|---|
| GPG release key | `FORGE_GPG_PRIVATE_KEY` (base64-encoded), `FORGE_GPG_KEY_FINGERPRINT` | `checksums` | Sign `SHA256SUMS` and its per-archive companions. |
| cosign keyless | — (uses OIDC) | `sign` | Sign every archive. |
| Apple Developer ID Application certificate | `FORGE_CODESIGN_IDENTITY` | `sign` | Sign darwin binaries. |
| Apple notary credentials | `FORGE_NOTARY_KEYCHAIN_PROFILE` | `sign` | Notarize darwin binaries. |
| Windows Authenticode certificate | `FORGE_AUTHENTICODE_SUBJECT`, `FORGE_AUTHENTICODE_SHA1` | `sign` | Sign windows binaries. |

The credentials' handling policy is in
[`SECURITY.md`](../../SECURITY.md) §"Signing keys and their
handling". The Go model's `JobSecrets` map is the machine-readable
form of this table; a test asserts the workflow's `secrets.*`
references match.

### Why cosign uses no long-lived key

cosign keyless obtains a short-lived certificate from Sigstore's
Fulcio CA, using the CI runner's OIDC token. The `sign` job
declares `id-token: write`; the token is minted per run and expires
with the job. There is no key to rotate, because there is no key.

The certificate's identity is pinned to the Forge repository's
release workflow:

```text
https://github.com/thapelomagqazana/forge/.github/workflows/release.yml@refs/tags/<version>
```

The OIDC issuer is pinned to GitHub's:

```text
https://token.actions.githubusercontent.com
```

A user's `cosign verify-blob` supplies both as `--certificate-*`
arguments. Without them, `cosign` would accept any Sigstore
signature, which would not prove the archive came from Forge. See
[`docs/reference/verifying-releases.md`](verifying-releases.md)
§"Step 4".

### Why the GPG key is imported only for a real release

The `checksums` job imports the GPG key only when
`dry_run != '1'`. A dry-run must not import the release key into an
ephemeral runner: the key's passphrase-less import is the one
operation a dry-run's author does not have the authority to
perform. The dry-run's author has the authority to run the
pipeline; the authority to use the release key is separate, and is
reserved for a tag push.

The dry-run's `SHA256SUMS` is unsigned. The dry-run's `checksums`
job runs the driver with `--no-gpg`; the driver records
"GPG signing skipped (--no-gpg)" on stdout and exits 0. The real
release's `checksums` job runs the driver without `--no-gpg`; the
driver imports the key and produces both signatures.

### Why a missing credential is a blocker, not a failure

The `sign` job runs the driver without `--strict`. A missing
Apple or Windows credential is recorded as a documented blocker on
stderr; the driver exits 0; the release continues. The policy is
in [`SECURITY.md`](../../SECURITY.md) §"Signing keys and their
handling".

The rationale: a release with an unnotarized macOS binary and a
documented blocker is preferred over a release that fails the
pipeline and produces nothing. A release engineer who expects the
credential sees the blocker and investigates; a CI job that never
has the credential sees the blocker and continues. The `--strict`
flag turns every blocker into a failure; the release workflow does
not set it. A future workflow that needs strict behavior for a
specific channel can set it conditionally.

---

## The permissions

The workflow's top-level `permissions:` block grants the minimum:
`contents: read`. Each job declares the subset it needs. The Go
model's `JobPermissions` map is the source of truth.

| Job | `contents` | `packages` | `id-token` | `actions` |
|---|---|---|---|---|
| `route` | read | — | — | — |
| `build` | read | — | — | — |
| `verify` | read | — | — | — |
| `checksums` | read | — | — | — |
| `sbom` | read | — | — | — |
| `sign` | read | — | write | — |
| `provenance` | write | — | write | read |
| `publish` | write | write | — | — |
| `verify-release` | read | — | — | — |

A job that declares more than it needs is a violation of least
privilege. A job that declares less fails at run time. The test
asserts both directions.

The `sign` job's `id-token: write` is the OIDC token cosign
keyless uses. A compromised step in `checksums` cannot mint an
OIDC token because the job does not declare the permission. This
is the security rationale for per-job permissions: the top-level
block would grant `id-token: write` to every job, including the
ones that do not sign.

---

## The dry-run

A dry-run is a `workflow_dispatch` with `dry_run: true`. It runs
every job except `publish` and `verify-release`. It produces the
full artifact set in the job workspaces and uploads every artifact
as a workflow artifact.

### What a dry-run does

- Builds every Tier 1 and Tier 2 platform.
- Runs the binary smoke suite.
- Generates every per-binary `.sha256` file and `SHA256SUMS`.
- Generates every per-binary SBOM and `forge.sbom.json`.
- Runs cosign keyless over every archive.
- Generates the SLSA provenance attestation.
- Uploads every artifact as a workflow artifact.

### What a dry-run does not do

- **Import the GPG release key.** `SHA256SUMS` is unsigned. The
  real release's is signed.
- **Create a GitHub Release.** `publish` is skipped.
- **Attach artifacts to a release.** There is no release.
- **Publish to a package manager or a container registry.** Those
  steps are inside `publish`, which is skipped.
- **Upload the SLSA attestation to the transparency log.**
  `provenance` runs with `upload-assets: false`; the attestation
  is a workflow artifact, not a release asset.

The dry-run's purpose is to answer "would this release succeed?"
without the irreversible side effects. A reviewer inspects the
workflow artifacts and confirms the set is complete. The procedure
is in [`docs/guides/release-dry-run.md`](../guides/release-dry-run.md).

### The dry-run's version resolution

A `workflow_dispatch` on a branch has no `GITHUB_REF_NAME` tag.
The `route` job resolves the version from, in order:

1. The `version` input, if non-empty.
2. The latest `v*` tag, if any.
3. Failure.

The `route` job's `dry_run` output is read by every downstream
job. A dry-run's `channel` is computed the same way a real
release's is: from the resolved tag. A dry-run of `v1.4.2`
computes the `stable` channel; a dry-run of `v1.5.0-rc.1` computes
the `prerelease` channel. The dry-run does not change the channel
logic; it changes the version's source.

---

## The rollback trigger

A release that is found broken after `publish` is withdrawn by the
procedure in [`docs/guides/rollback.md`](../guides/rollback.md).
The short version:

1. Confirm the defect and notify `security@forge.dev`.
2. Delete the GitHub Release (`gh release delete <tag>`).
3. Retract the tag: push a signed, annotated, empty commit to a
   `retracted/<tag>` branch. Do not force-delete the tag.
4. Publish a GitHub Security Advisory.
5. Release a patch under a new tag.
6. Update the pipeline's documentation if the defect is in the
   pipeline.

A rollback is a security incident. The release's verification
failure is reported to `security@forge.dev` per
[`SECURITY.md`](../../SECURITY.md) §"Reporting a vulnerability".

### What triggers a rollback

- A user's `cosign verify-blob`, `gpg --verify`, or
  `slsa-verifier` fails against a published artifact.
- A functional defect severe enough that the release must not be
  used.
- A security defect: the release contains a vulnerability that is
  being actively exploited.

### What does not trigger a rollback

- A blocker in the release's log. A blocker is documented; the
  release is usable. A user who needs notarization or Authenticode
  sees the blocker in the release notes and decides whether to
  use the release.
- A defect in a nightly. A nightly is not supported; a defective
  nightly is superseded by the next nightly. It is deleted after
  30 days.
- A defect in a prerelease that is superseded by a stable release
  of the same minor. The prerelease is not supported; the stable
  release is.

---

## Failure modes

| Failure | Effect | Recovery |
|---|---|---|
| `route` rejects the tag. | No job runs. | Fix the tag or the channel pattern; re-tag. |
| `build` fails. | No artifacts. | Fix the build; re-tag. |
| `verify` fails. | No signing. | Fix the binary; re-tag. |
| `checksums` fails. | No `SHA256SUMS`. | Fix the driver; re-run the job. |
| `sbom` fails. | No SBOMs. | Fix the driver; re-run the job. |
| `sign` fails (cosign absent). | A blocker is recorded; the release continues. | Accept the release as unsigned, or install cosign and re-tag. |
| `sign` fails (a platform credential absent). | A blocker is recorded; the release continues. | Accept the release with the blocker, or supply the credential and re-tag. |
| `provenance` fails. | No attestation. | Re-run the job. The attestation is independent of the artifacts. |
| `publish` fails. | No Release. | Re-run the job. |
| `verify-release` fails. | The Release exists but is incomplete. | Apply the rollback procedure. |

A failure in `route`, `build`, `verify`, `checksums`, or `sbom`
stops the pipeline before `publish`. The release is not created.
The tag remains; the failure is recorded in the workflow run; the
fix is a new tag.

A failure in `sign` is a blocker, not a stop. The release is
created with the artifacts that exist. The blocker is in the
workflow run's log and in the release notes.

A failure in `provenance` stops `publish` (which needs it). The
tag remains; the attestation is re-generated by re-running the
job. The `build` job's artifacts are cached; a re-run is fast.

A failure in `publish` stops `verify-release`. The tag remains;
the release may or may not exist depending on where `publish`
failed. The rollback guide's step 1 (confirm the defect) applies.

A failure in `verify-release` means the Release exists but is
incomplete: a supply-chain artifact is missing, or the GPG
signature does not verify. This is the most serious failure: the
release is published and a user can download it. The rollback
guide applies.

---

## The reusable build workflow

`release.yml`'s `build` job delegates to
[`.github/workflows/build.yml`](../../.github/workflows/build.yml).
The reusable workflow:

- Declares its own permissions (`contents: read`); a caller cannot
  widen them.
- Declares an `outputs.hashes` value: the base64-encoded
  `sha256sum forge-*` over every archive. `slsa-github-generator`
  consumes it as `base64-subjects`.
- Uploads the archives as a workflow artifact.

The `hashes` output is the contract between the build and the
provenance. A change to the archive naming convention changes the
`hashes` input's contents; the two must be updated together.

---

## The provenance's verification

The `provenance` job produces `forge.intoto.jsonl`, which
`publish` attaches to the Release. A user verifies the attestation
with `slsa-verifier`:

```sh
slsa-verifier verify-artifact \
  --provenance-path forge.intoto.jsonl \
  --source-uri github.com/thapelomagqazana/forge \
  forge-linux-amd64.tar.gz
```

The `slsa-github-generator` reusable workflow signs its own
attestation. The attestation is not separately cosign-signed; its
proof is its own signature, checked by `slsa-verifier`. This is
the correct design: a second signature over a file whose integrity
is already established by the first would add a verification step
without adding a proof.

The full procedure is in
[`docs/reference/verifying-releases.md`](verifying-releases.md)
§"Step 5".

---

## The workflow's invariants

The following invariants hold for every run. A change to the
workflow that breaks one is a defect.

1. **The tag is the only input to the channel.** No branch name,
   environment variable, or human approval influences the
   channel. The `route` job is the only job that reads the tag.
2. **A broken binary is not signed.** The `sign` job needs
   `verify`, and `verify` needs `build`. A `build` failure or a
   `verify` failure stops the pipeline before any signature is
   produced.
3. **A dry-run and a real release run the same pipeline.** The
   only differences are two `if:` expressions and one key-import
   guard.
4. **A job declares only the permissions it needs.** The
   top-level block is the minimum; each job's block is its
   subset. No job has `id-token: write` except `sign` and
   `provenance`.
5. **A secret is read only by the job that needs it.** The Go
   model's `JobSecrets` map is the source of truth; the test
   asserts the workflow's `secrets.*` references match.
6. **Every external action is pinned.** A `uses:` reference is a
   tag or a SHA, not a branch. A `./.github/workflows/...`
   reference is local and has no ref.
7. **The Release attaches every supply-chain artifact.** A
   missing `SHA256SUMS`, `forge.sbom.json`, or
   `forge.intoto.jsonl` fails `verify-release`, which triggers
   the rollback.
8. **The published GPG signature verifies.** `verify-release`
   downloads `SHA256SUMS` and `SHA256SUMS.asc` and runs
   `gpg --verify`. A malformed signature is caught before the
   release is considered complete.
9. **A retracted tag is not a deleted tag.** The rollback guide's
   step 3 pushes a retraction commit; it does not force-delete
   the tag.

---

## Related documents

| Document | Role |
|---|---|
| [`docs/guides/rollback.md`](../guides/rollback.md) | The post-publish rollback procedure. |
| [`docs/guides/release-dry-run.md`](../guides/release-dry-run.md) | How to run and read a dry-run. |
| [`docs/guides/verifying-binaries.md`](../guides/verifying-binaries.md) | The binary smoke suite the `verify` job runs. |
| [`docs/reference/verifying-releases.md`](verifying-releases.md) | The user-facing verification procedure. |
| [`docs/reference/versioning.md`](versioning.md) | The tag-to-version contract. |
| [`SECURITY.md`](../../SECURITY.md) | The credentials and their handling. |
