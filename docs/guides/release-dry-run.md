<!--
  Running a release dry-run
  =========================
  Public guide. How to run the release pipeline without publishing
  a release, and how to read the result.

  Normative references:
    - Release process:   docs/reference/release-process.md
    - Rollback guide:    docs/guides/rollback.md
    - Verifying releases: docs/reference/verifying-releases.md
    - Verifying binaries: docs/guides/verifying-binaries.md

  Editing rules:
    1. Every command in this document MUST work against the
       current release workflow. A command that fails is a defect
       in this document.
    2. The document MUST NOT instruct a reader to run a dry-run
       against a tag that is already published. A dry-run of a
       published tag produces a second artifact set that is not
       the published one; the two are confusing, and the second
       set is not attached to any Release.
    3. The document MUST NOT instruct a reader to set a secret.
       Secrets are set by the maintainers; a reader who is running
       a dry-run does not set them.
    4. The document MUST NOT be edited after a release without
       re-running every command in it against the current
       workflow. The commands are the contract; a stale command is
       worse than no command.
-->

# Running a release dry-run

A dry-run runs the release pipeline without publishing a release.
It produces the full artifact set and uploads it as a workflow
artifact. A reviewer inspects the set and confirms the release
would succeed.

Run a dry-run:

- **Before a real release**, to confirm the pipeline's inputs are
  valid. A tag that matches no channel, a missing release-notes
  file, or a dependency that fails to build are all caught here.
- **After a change to the pipeline**, to confirm the change works.
  The dry-run is the pipeline's regression test for its own
  configuration.
- **When a credential's availability is uncertain**, to confirm the
  pipeline records the right blocker. A dry-run never imports the
  GPG key; a dry-run's log shows the blocker the real release
  would record if the key were absent.
- **When a release's artifact set is in doubt**, to confirm every
  expected file is produced. A dry-run's artifacts are the
  authoritative list.

A dry-run does not replace a real release. It exercises the
pipeline; it does not exercise the artifact's runtime behavior on
every platform. The binary smoke suite
([`docs/guides/verifying-binaries.md`](verifying-binaries.md))
covers the runtime behavior that a smoke test can cover; a user's
report covers the rest. See §"When a dry-run is not enough".

---

## Before you start

You need:

- **The `workflow_dispatch` permission** on the repository. A
  contributor with the `triage` role does not have it. Ask a
  maintainer if you do not.
- **A branch with the workflow change.** A dry-run runs the
  workflow from the ref you select. A dry-run of `main` runs
  `main`'s workflow; a dry-run of a feature branch runs that
  branch's workflow. The choice matters: a dry-run of a branch
  that does not have the change you want to test will not test it.

You do **not** need:

- Any credential. The dry-run's `checksums` job runs with
  `--no-gpg`; the `sign` job records a blocker for each missing
  platform credential. A dry-run with no credentials is the
  expected case for a contributor.
- Write access to the repository. A dry-run does not write to the
  release page, the package registries, or the transparency logs.
- A local clone. The GitHub UI's **Run workflow** button is
  sufficient. A local clone is useful if you want to inspect the
  artifacts with the tools in
  [`docs/reference/verifying-releases.md`](../reference/verifying-releases.md).

---

## How to run a dry-run

### Via the GitHub UI

1. Open the repository's **Actions** tab.
2. Select **release** in the left sidebar. The workflow's name is
   `release`; the file is `.github/workflows/release.yml`.
3. Click **Run workflow** (the button on the right).
4. In the **Use workflow from** dropdown, select the branch you
   want to test. A dry-run of a feature branch tests that
   branch's workflow; a dry-run of `main` tests `main`'s.
5. Set **Run the pipeline without publishing a release** to
   `true`. This is the `dry_run` input.
6. Optionally set **Version tag to simulate** to the tag you want
   to simulate (for example, `v1.4.3`). Leave it empty to use the
   latest `v*` tag.
7. Click **Run workflow**.

### Via the GitHub CLI

```sh
gh workflow run release.yml \
  --ref <branch> \
  -f dry_run=true \
  -f version=<tag>
```

The `--ref` is the branch to run from. The `-f` flags set the
inputs. Both are optional for `dry_run`? No: `dry_run=true` is
required for a dry-run. `version` is optional; without it, the
`route` job resolves the version from the latest `v*` tag.

Example, a dry-run of `feat/17.5.3-release-pipeline` simulating
`v1.4.3`:

```sh
gh workflow run release.yml \
  --ref feat/17.5.3-release-pipeline \
  -f dry_run=true \
  -f version=v1.4.3
```

### What happens

The workflow starts. Every job except `publish` and
`verify-release` runs. The jobs upload their artifacts as workflow
artifacts. The workflow's run page shows every job's status and
links to the uploaded artifacts.

The workflow's run page is at:

```text
https://github.com/thapelomagqazana/forge/actions/runs/<run-id>
```

The run ID is printed by `gh workflow run` and appears in the
Actions tab. The `gh run view <run-id>` command shows the run's
details:

```sh
gh run watch <run-id>
gh run view <run-id> --log
```

---

## Reading a dry-run

### The jobs

| Job | Expected result |
|---|---|
| `route` | Green. It resolves the version and the channel. In a dry-run, the version comes from the `version` input or the latest `v*` tag. |
| `build` | Green. Every Tier 1 and Tier 2 platform is built. |
| `verify` | Green. Every binary passes the smoke suite. |
| `checksums` | Green. `SHA256SUMS` is present; the GPG signatures are absent (the dry-run does not import the key). |
| `sbom` | Green. Every per-binary SBOM and `forge.sbom.json` is present. |
| `sign` | Green. Every archive's `.sig` and `.pem` are present. The macOS and Windows blockers are recorded if the credentials are absent. |
| `provenance` | Green. `forge.intoto.jsonl` is present. |
| `publish` | **Skipped.** The dry-run does not create a release. |
| `verify-release` | **Skipped.** The dry-run does not verify a release. |

A red job is a pipeline defect. Investigate it; do not retry it
without understanding the failure. A flaky test is a defect in the
test; report it rather than re-running the workflow.

The two skipped jobs are expected. A dry-run that shows them as
red is a bug in the workflow's `if:` expressions; report it.

### The blockers

The `sign` job's log contains one or more blocker lines when a
credential is absent:

```text
BLOCKER: notarization: FORGE_CODESIGN_IDENTITY is not set; darwin binaries are not signed. ...
BLOCKER: authenticode: FORGE_AUTHENTICODE_SUBJECT is not set; windows binaries are not signed. ...
2 blocker(s) recorded
```

A blocker is expected in a dry-run. It is not an error. It tells
you the real release would ship the same blocker. If you want to
test the pipeline's behavior with the credential present, ask a
maintainer to run a dry-run from a ref that has the secret. A
dry-run from a contributor's ref does not have the secrets; the
blockers are the expected output.

### The artifacts

Download the `release-checksums`, `release-sbom`, and
`release-signatures` artifacts. The `release-build` artifact is
also present; it is the archives.

```sh
gh run download <run-id>
```

The command creates a directory per artifact. The directories are
named after the artifact's `name:` in the workflow.

Inspect them:

```sh
# The combined checksum list has one line per archive.
wc -l release-checksums/SHA256SUMS

# The SBOM index names every per-binary SBOM.
jq '.components | length' release-sbom/forge.sbom.json

# The cosign signature verifies against its certificate.
cosign verify-blob \
  --certificate-identity-regexp='^https://github.com/thapelomagqazana/forge/' \
  --certificate-oidc-issuer='https://token.actions.githubusercontent.com' \
  --certificate release-signatures/forge-linux-amd64.tar.gz.pem \
  --signature   release-signatures/forge-linux-amd64.tar.gz.sig \
  release-build/forge-linux-amd64.tar.gz
```

The cosign verification in a dry-run uses the certificate that the
dry-run's `sign` job produced. The signature is over the archive
the dry-run's `build` job produced. The two are a consistent pair;
the dry-run's verification confirms they match. A mismatch is a
bug in the `sign` job or the `build` job; report it.

### The expected artifact set

A dry-run's `release-checksums`, `release-sbom`, and
`release-signatures` artifacts contain, for a release with six
platforms (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64,
windows/amd64, windows/arm64):

```text
release-checksums/
  SHA256SUMS
  forge-darwin-amd64.tar.gz.sha256
  forge-darwin-arm64.tar.gz.sha256
  forge-linux-amd64.tar.gz.sha256
  forge-linux-arm64.tar.gz.sha256
  forge-windows-amd64.zip.sha256
  forge-windows-arm64.zip.sha256

release-sbom/
  forge.sbom.json
  forge-darwin-amd64.tar.gz.cdx.json
  forge-darwin-arm64.tar.gz.cdx.json
  forge-linux-amd64.tar.gz.cdx.json
  forge-linux-arm64.tar.gz.cdx.json
  forge-windows-amd64.zip.cdx.json
  forge-windows-arm64.zip.cdx.json

release-signatures/
  forge-darwin-amd64.tar.gz.sig
  forge-darwin-amd64.tar.gz.pem
  forge-darwin-arm64.tar.gz.sig
  forge-darwin-arm64.tar.gz.pem
  forge-linux-amd64.tar.gz.sig
  forge-linux-amd64.tar.gz.pem
  forge-linux-arm64.tar.gz.sig
  forge-linux-arm64.tar.gz.pem
  forge-windows-amd64.zip.sig
  forge-windows-amd64.zip.pem
  forge-windows-arm64.zip.sig
  forge-windows-arm64.zip.pem
```

A missing file is a pipeline defect. The `checksums` job's
`upload-artifact` uses `if-no-files-found: error`; the job fails
if a file is missing. The same applies to `sbom` and `sign`. A
green job with a missing artifact is impossible; a green job with
the wrong artifact count is possible if a platform's build
silently produced a different name. The artifact list is the
check; compare it to the list above.

The `release-build` artifact contains the archives:

```text
release-build/
  forge-darwin-amd64.tar.gz
  forge-darwin-arm64.tar.gz
  forge-linux-amd64.tar.gz
  forge-linux-arm64.tar.gz
  forge-windows-amd64.zip
  forge-windows-arm64.zip
```

The `release-provenance` artifact contains `forge.intoto.jsonl`:

```text
release-provenance/
  forge.intoto.jsonl
```

---

## What a dry-run does not do

A dry-run does not:

- **Import the GPG release key.** `SHA256SUMS` is unsigned. The
  real release's is signed. The dry-run's `checksums` job runs
  the driver with `--no-gpg`.
- **Create a GitHub Release.** `publish` is skipped. The dry-run's
  artifacts are workflow artifacts, not release assets.
- **Attach artifacts to a release.** There is no release.
- **Publish to a package manager or a container registry.** Those
  steps are inside `publish`, which is skipped. The dry-run's log
  shows `Would publish ghcr.io/...` only if `publish` ran; it does
  not.
- **Upload the SLSA attestation to the transparency log.**
  `provenance` runs with `upload-assets: false`; the attestation
  is a workflow artifact, not a release asset.
- **Exercise a real tag.** The dry-run resolves the version from
  the `version` input or the latest `v*` tag. It does not create
  a tag. A tag that does not exist is a valid input to a dry-run;
  the version is used only in the artifact names and the
  provenance's metadata.
- **Replace a real release.** A dry-run is a rehearsal. The real
  release is a tag push.

---

## When a dry-run is not enough

A dry-run does not catch:

- **A defect in the artifact's runtime behavior on a platform the
  smoke suite does not cover.** The smoke suite runs on
  `ubuntu-22.04`; a defect that reproduces only on a user's
  macOS or Windows host is caught by a user. The smoke suite's
  coverage is documented in
  [`docs/guides/verifying-binaries.md`](verifying-binaries.md).
- **A defect in a package manager's submission process.** The
  submission is in `publish`, which the dry-run skips. A defect
  in the submission (a wrong formula, a wrong version string) is
  caught by the package manager's review or by a user.
- **A defect in a container image.** The image is published in
  `publish`, which the dry-run skips.
- **A defect in the release notes.** The dry-run's `publish` job
  is skipped, so the release notes' validation is skipped. The
  validation is a `go run ./cmd/forge-internal-version notes
  check` invocation; run it locally to test the notes:

  ```sh
  go run ./cmd/forge-internal-version notes check \
    docs/releases/v1.4.3.md
  ```

- **A defect in the GPG signing step.** The dry-run runs the
  `checksums` driver with `--no-gpg`; a defect that only
  manifests with a real key is caught by a real release. The
  driver's GPG path is tested in
  `cmd/forge-internal-checksums/main_test.go` against a
  temporary keyring; the dry-run does not exercise the path
  against the release key.
- **A defect in the macOS notarization or the Windows
  Authenticode step.** Those steps are in the `sign` job's
  platform-specific code, which is compiled only on the
  platform. A dry-run on `ubuntu-22.04` runs the driver, which
  probes for the credential and records a blocker. The actual
  `codesign`/`notarytool`/`stapler` and `signtool` invocations
  are not exercised. A defect in the platform-specific code is
  caught by a real release on a macOS host or a Windows host,
  which is the only environment where the code runs.

Those are caught by a real release and, if necessary, by the
rollback procedure.

---

## A worked example

A contributor is adding a new platform, `linux/riscv64`, to the
build matrix. The platform is Tier 2; it ships. The contributor
wants to confirm the pipeline produces the new platform's
artifacts.

**1. The contributor's branch.** The branch is
`feat/add-riscv64`. It adds the platform to
`internal/platform/platforms.go`'s canonical matrix and to
`docs/reference/platforms.yaml`. It does not change the workflow.

**2. The dry-run.** The contributor runs:

```sh
gh workflow run release.yml \
  --ref feat/add-riscv64 \
  -f dry_run=true \
  -f version=v1.5.0-rc.1
```

**3. The jobs.** Every job except `publish` and `verify-release`
runs. The `build` job builds the new platform. The `verify` job
runs the smoke suite against the new platform's binary. The
`checksums` job produces a `.sha256` for the new platform's
archive. The `sbom` job produces a `.cdx.json` for it. The `sign`
job produces a `.sig` and a `.pem` for it.

**4. The artifacts.** The contributor downloads the artifacts
and confirms the new platform's files are present:

```sh
gh run download <run-id>
ls release-checksums/ | grep riscv64
ls release-sbom/ | grep riscv64
ls release-signatures/ | grep riscv64
```

Each command prints two lines: the archive's `.sha256` (or
`.cdx.json`, or `.sig` and `.pem`). The contributor confirms the
count is right.

**5. The review.** The contributor opens a PR. The PR's
description names the dry-run's run ID. The reviewer reads the
run's log and confirms the new platform's files are present. The
reviewer merges the PR.

**6. The real release.** A maintainer tags `v1.5.0-rc.1`. The
real release runs the same pipeline. The dry-run's artifacts and
the release's artifacts are the same set; the release's
`SHA256SUMS.asc` is the only file that differs, because the
release imports the GPG key and the dry-run does not.

The dry-run's run ID is the PR's evidence. The real release's
run is the pipeline's production run. The two are separate runs
of the same workflow; the dry-run is the rehearsal.

---

## Related documents

| Document | Role |
|---|---|
| [`docs/reference/release-process.md`](../reference/release-process.md) | The pipeline's contract. |
| [`docs/guides/rollback.md`](rollback.md) | The post-publish rollback procedure. |
| [`docs/guides/verifying-binaries.md`](verifying-binaries.md) | The binary smoke suite the `verify` job runs. |
| [`docs/reference/verifying-releases.md`](../reference/verifying-releases.md) | The user-facing verification procedure. |
| [`SECURITY.md`](../../SECURITY.md) | The credentials and their handling. |
