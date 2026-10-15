<!--
  Rolling back a release
  ======================
  Public guide. The procedure a release engineer follows when a
  published release is found broken.

  Normative references:
    - Release process:  docs/reference/release-process.md
    - Release channels: docs/reference/release-channels.md
    - Security policy:  SECURITY.md
    - Verifying releases: docs/reference/verifying-releases.md

  Editing rules:
    1. Every command in this document MUST work against a real
       release. A command that fails is a defect in this document.
    2. The procedure MUST NOT instruct a reader to force-delete a
       tag. A force-deleted tag is indistinguishable from a tag
       that never existed; a retracted tag records the retraction.
    3. The procedure MUST NOT instruct a reader to rewrite a
       published release's artifacts. A published artifact is
       immutable. A corrected release is a new tag.
    4. The document MUST NOT be edited after a rollback without
       updating the lessons-learned section, if any. A rollback is
       a defect report against the process; the process is what
       the document records.
-->

# Rolling back a release

A rollback withdraws a published release after a defect is found.
The defect is one of:

- **A verification failure.** A user's `cosign verify-blob`,
  `gpg --verify`, or `slsa-verifier` fails against a published
  artifact. The verification guide is
  [`docs/reference/verifying-releases.md`](../reference/verifying-releases.md).
- **A functional defect** severe enough that the release must not
  be used. A crash on startup, a data-loss bug, or a regression
  that breaks a documented workflow.
- **A security defect.** The release contains a vulnerability
  that is being actively exploited, or a credential embedded in a
  release artifact.

A rollback is a security incident. Report it to
`security@forge.dev` **first**; the security team coordinates the
rollback. The reporting procedure is in
[`SECURITY.md`](../../SECURITY.md) §"Reporting a vulnerability".

## What a rollback is not

A rollback does not:

- **Rewrite the release's tag.** The tag stays. It is retracted,
  not deleted. A force-deleted tag is indistinguishable from a
  tag that never existed; a retracted tag records the retraction
  in the repository's history.
- **Delete the release's artifacts from a user's disk.** A user
  who already downloaded the release keeps it. The advisory tells
  them to stop using it. The release page's deletion removes the
  download URL; it does not reach into a user's filesystem.
- **Guarantee that a downstream mirror has removed the
  artifacts.** A mirror that has cached the artifacts continues
  to serve them until the mirror's cache expires or the mirror's
  operator acts. The advisory is the mechanism; a mirror that
  ignores it is the mirror's defect.
- **Replace the release with a corrected one under the same
  tag.** A corrected release is a new tag. A tag is immutable;
  the artifact a user downloaded under that tag is the artifact
  they got.
- **Undo a package manager's submission.** If the release was
  submitted to Homebrew, Scoop, winget, or apt before the defect
  was found, the submission is withdrawn by the procedure in the
  package manager's own documentation. The rollback's advisory
  names the submission; the release engineer opens the
  withdrawal PR with the package manager.

## The procedure

The procedure has seven steps. Steps 1 and 2 are investigation;
steps 3 through 6 are the withdrawal; step 7 is the documentation.
Do not skip a step. A withdrawal that deletes the Release without
recording the retraction or publishing an advisory leaves users
without the information they need to act.

### Step 1: Confirm the defect

Reproduce the defect against the published artifact. Record:

- **The artifact's name.** For example,
  `forge-linux-amd64.tar.gz`.
- **The artifact's SHA-256.** A user who reports the defect should
  include the hash; a mismatch is a different defect.
- **The command that fails.** For example,
  `cosign verify-blob --certificate ...`.
- **The complete output.** Including the exit code and any error
  message.
- **The environment.** The operating system, the architecture, the
  tool versions (`cosign version`, `gpg --version`,
  `slsa-verifier version`), and any relevant configuration.

A defect that cannot be reproduced is not a rollback. A defect
that is reproduced only in a modified environment is not a
rollback until the modification is understood. The most common
false positive is a user's `PATH` selecting a different `cosign`
than the one the verification guide assumes.

### Step 2: Notify the security team

Send the record from step 1 to `security@forge.dev`. The security
team decides whether the rollback is warranted and, if so,
coordinates it.

The security team's decision is not a formality. The team has
context the release engineer may not: an in-flight advisory, a
reporter's request to remain anonymous, a coordinated disclosure
window that is about to expire. A rollback performed without the
team's knowledge may be premature, or may interfere with a
disclosure in progress.

Do not skip this step. The cost of a five-minute email is
negligible compared to the cost of an uncoordinated withdrawal.

### Step 3: Delete the GitHub Release

```sh
gh release delete <tag> --yes
```

The release's artifacts are removed from the release page. A user
who has the URL receives a 404. The release's tag still exists;
step 4 handles it.

The `--yes` flag skips the confirmation prompt. Use it only after
step 2 has confirmed the rollback. Without `--yes`, `gh` prompts
for confirmation; a rollback that is run interactively does not
need the flag.

The `--cleanup-tag` flag is **not** used. `--cleanup-tag` deletes
the tag, which step 4 forbids. A rollback that uses
`--cleanup-tag` leaves no record of the tag's existence.

### Step 4: Retract the tag

A tag is not deleted. A deleted tag is indistinguishable from a
tag that never existed; a user who has the tag in their clone
cannot tell whether it was retracted or was never pushed. A
retracted tag records the retraction in the repository's history,
which is what a user needs to see when they run `git log --all`.

Create a retraction branch and push an empty, signed, annotated
commit:

```sh
git fetch --tags
git checkout -b retracted/<tag> <tag>
git commit --allow-empty -S -m "retract <tag>

This tag is retracted. See the advisory at
https://github.com/thapelomagqazana/forge/security/advisories/GHSA-xxxx-yyyy-zzzz
for the reason and the replacement release.

Do not use artifacts published under this tag.

The artifacts published under this tag are withdrawn from the
GitHub Release page. The tag itself is retained so that a clone
that already fetched it can see this retraction.

The replacement release is <new-tag>."
git push origin retracted/<tag>
```

The commit's body is the retraction's record. The advisory URL is
filled in after step 5; the retraction commit is created first so
that the tag's retraction is recorded even if step 5 is delayed.

The `-S` flag signs the commit with the release engineer's GPG
key. The signature is the release engineer's; it is recorded in
the repository's history and is independent of the release's
signatures. A user who wants to verify the retraction verifies
the commit's signature against the release engineer's public key,
which is published in the repository's `MAINTAINERS.md`.

The retraction branch's name is `retracted/<tag>`. It is not
merged. It exists so that a user who runs `git log --all` sees
the retraction.

If the tag was already fetched by a user, the user's local clone
has the tag. The user's `git fetch` will not see the retraction
unless they fetch the `retracted/<tag>` branch explicitly, or
until the retraction is in `main`. The advisory is the primary
notification; the retraction branch is the repository's record.

### Step 5: Publish a GitHub Security Advisory

Open a GitHub Security Advisory at:

```text
https://github.com/thapelomagqazana/forge/security/advisories/new
```

The advisory includes:

- **The affected version.** The tag, for example `v1.4.2`.
- **The defect's description.** What is wrong, in one paragraph.
- **The defect's impact.** Who is affected, and how. "A user who
  runs the binary on Windows receives a SmartScreen warning
  because the Authenticode signature is missing" is an impact;
  "the binary is unsigned" is a description.
- **The mitigation.** Usually "upgrade to the replacement
  release". If no replacement exists yet, say so and give the
  workaround.
- **The replacement release's tag.** If one exists.
- **The reporter's credit.** Unless they ask to remain anonymous.
  The credit goes in the advisory's acknowledgement section.

Publish the advisory. GitHub notifies the repository's watchers
and the advisory's subscribers. The advisory's URL is the one the
retraction commit references.

The advisory's severity is set by the security team. A
verification failure that does not lead to a user's running a
malicious binary is `moderate` or `low`; a defect that leads to
code execution is `critical` or `high`. The severity is
documented in the advisory and is the input to a downstream
consumer's decision.

### Step 6: Release a patch

The patch release follows the normal release process. Its tag is
a new tag; it does not replace the retracted tag. The patch's
release notes reference the advisory and state what was fixed.

The patch's version follows semver. A defect in `v1.4.2` produces
`v1.4.3` (if the fix is a patch) or `v1.5.0` (if the fix is a
minor). The version is chosen by the same rules the versioning
document applies to any release; a rollback does not change the
rules.

The patch's release notes are in
`docs/releases/<new-tag>.md`. The notes' "Security" section
names the advisory and the affected version.

The patch is the release a user upgrades to. The retracted
release's advisory names the patch; a user who follows the
advisory's mitigation reaches the patch.

### Step 7: Update the pipeline's documentation

If the defect is in the pipeline — a missing verification step, a
wrong permission, a missing artifact — update the pipeline's
documentation and the pipeline itself. The rollback is a defect
report against the process, not only against the release.

The documents to update:

- `docs/reference/release-process.md`, if the job graph, the
  credentials, or the channel mapping changes.
- `docs/reference/verifying-releases.md`, if the verification
  procedure changes. A verification command that did not catch
  the defect is a defect in the guide.
- `SECURITY.md`, if the credentials or their handling change.
- `.github/workflows/release.yml`, if the workflow changes.

The update's commit message references the advisory. The
documentation's change and the workflow's change are in the same
PR, if they are part of the same fix.

## What a user sees

A user who tries to download the retracted release's artifacts
after step 3 receives a 404. The user's GitHub client may show
the release as "deleted"; the GitHub web UI does not show the
release at all.

A user who has the artifacts already can still verify them. The
`SHA256SUMS` and `SHA256SUMS.asc` are on the user's disk. A
user who runs `gpg --verify SHA256SUMS.asc SHA256SUMS` sees a
good signature, because the artifacts have not been tampered
with; they are the artifacts that were published. The advisory
tells the user not to use them; the verification does not. This
is the asymmetry the advisory exists to bridge: the artifacts
are cryptographically valid, but they are functionally or
security-wise defective.

A user who wants to know why the release was retracted reads the
advisory. The retraction commit's body links to the advisory; the
advisory is the explanation. The release notes of the patch link
to the advisory again, so a user who lands on the patch's page
without the context has a path to the context.

## The dry-run's role

A dry-run does not prevent a rollback. A dry-run exercises the
pipeline; it does not exercise the artifact's runtime behavior on
every platform. A defect that a dry-run would catch is caught
before the release; a defect that a dry-run would not catch is
caught by a user after the release, and the rollback procedure
applies.

The dry-run's purpose is to catch pipeline defects (a missing
job, a wrong permission, a missing artifact). The artifact's
runtime behavior is caught by the binary smoke suite
([`docs/guides/verifying-binaries.md`](verifying-binaries.md))
before the release and by a user's report after.

The split is not a gap; it is the division of labor. The dry-run
checks the pipeline's shape; the smoke suite checks the binary's
shape; the user checks the runtime's shape. A defect that only a
user's environment exhibits is caught by the user, which is the
only place it can be caught.

## What a rollback does not catch

A rollback is a reaction, not a prevention. A rollback does not
catch:

- **A defect that was never reported.** A user who finds a defect
  and does not report it is not part of the rollback's input. The
  reporting procedure is in
  [`SECURITY.md`](../../SECURITY.md) §"Reporting a
  vulnerability"; a report is the first step of every rollback.
- **A defect in a mirror's copy.** A mirror that has cached a
  retracted release continues to serve it. The rollback's
  advisory names the mirror if the mirror is known; an unknown
  mirror is the mirror operator's responsibility.
- **A defect in a user's environment.** A user's `PATH`, a
  user's `$HOME`, a user's shell aliases: none of these are the
  release's defect. The release engineer's job is to determine
  which is which; step 1's record is the input to that
  determination.

## A worked example

A release `v1.4.2` is published. A user reports that
`forge version --format=json` on `linux/arm64` prints
`"dirty": true` even though the release was built from a clean
tree.

**Step 1.** The release engineer downloads
`forge-linux-arm64.tar.gz`, verifies its SHA-256, extracts it,
and runs `forge version --format=json`. The output contains
`"dirty": true`. The engineer records the artifact's name, its
SHA-256, the command, the output, and the environment.

**Step 2.** The engineer sends the record to
`security@forge.dev`. The security team confirms the defect is in
the release, not in the user's environment, and authorizes the
rollback.

**Step 3.** The engineer runs
`gh release delete v1.4.2 --yes`. The release page returns 404.

**Step 4.** The engineer creates the retraction branch:

```sh
git checkout -b retracted/v1.4.2 v1.4.2
git commit --allow-empty -S -m "retract v1.4.2

This tag is retracted. The linux/arm64 binary was built with a
dirty tree; its version metadata reports dirty=true.

See the advisory at
https://github.com/thapelomagqazana/forge/security/advisories/GHSA-xxxx-yyyy-zzzz

The replacement release is v1.4.3."
git push origin retracted/v1.4.2
```

**Step 5.** The engineer opens a GitHub Security Advisory. The
description names the affected platform, the defect, and the
impact ("CI pipelines that branch on `dirty` mis-classify the
release as a development build"). The mitigation is "upgrade to
`v1.4.3`".

**Step 6.** The engineer fixes the build (the `dirty` flag's
source is a `git status --porcelain` invocation that ran before
the build artifacts were staged; the fix is to run it before any
file is written to the workspace). The fix is a patch; the new
tag is `v1.4.3`. The release notes reference the advisory.

**Step 7.** The engineer updates
`docs/reference/release-process.md` to note that the `dirty`
flag is computed from the source tree, not the build workspace,
and adds a test to
`scripts/internal/release/` that asserts the flag's source. The
test is in the same PR as the fix.

## Related documents

| Document | Role |
|---|---|
| [`docs/reference/release-process.md`](../reference/release-process.md) | The pipeline's contract. |
| [`docs/guides/release-dry-run.md`](release-dry-run.md) | How to run and read a dry-run. |
| [`docs/guides/verifying-binaries.md`](verifying-binaries.md) | The binary smoke suite the pipeline runs before signing. |
| [`docs/reference/verifying-releases.md`](../reference/verifying-releases.md) | The verification procedure a user follows. |
| [`SECURITY.md`](../../SECURITY.md) | How to report a defect and the credentials' handling. |
