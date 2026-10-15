<!--
  Verifying Forge releases
  ========================
  Public contract. Versioned with the Forge release.

  Normative references:
    - Release channels:     docs/reference/release-channels.md
    - Reproducible builds:  docs/guides/reproducible-builds.md
    - Verifying binaries:   docs/guides/verifying-binaries.md
    - Rollback guide:       docs/guides/rollback.md
    - Supply chain ADR:     docs/decisions/ADR-048-supply-chain-integrity.md
    - Release channels ADR: docs/decisions/ADR-043-versioning-and-release-channels.md
    - User surface ADR:     docs/decisions/ADR-050-user-verification-surface.md
    - Security policy:      SECURITY.md

  Editing rules:
    1. Every command in this document MUST work against a real
       release. A command that fails on the latest release is a
       defect in this document.
    2. Every fingerprint, certificate identity, and OIDC issuer
       MUST match the ones the release pipeline uses. The pipeline
       is the source of truth; this document copies.
    3. The document MUST NOT instruct a user to trust a key or a
       certificate without a way to verify it independently. A
       "download this key and trust it" instruction is a defect.
    4. The document MUST be re-run end-to-end against a real release
       after any change to the pipeline's job graph, the checksum
       driver, the signing driver, or the SBOM driver. A stale
       command is worse than no command.
    5. Every step MUST have a "What this establishes" subsection
       naming both the property the step proves and the property it
       does not prove. A step whose boundary is unstated is a step
       a user cannot reason about.
-->

# Verifying Forge releases

Every Forge release is accompanied by four general proofs of its
integrity, plus two platform-specific proofs, plus a
rebuild-from-source procedure. A user who runs the proofs has the
strongest available evidence that the archive they downloaded is
the archive the Forge maintainers intended to publish.

The four general proofs:

1. **SHA-256 checksums** — a `SHA256SUMS` file listing the hash of
   every archive, plus a per-archive `.sha256` file for the user
   who downloads a single archive.
2. **A GPG signature** over `SHA256SUMS` — proves the checksum list
   came from the Forge release key.
3. **A cosign signature** over each archive — proves the archive
   came from the Forge release pipeline on GitHub Actions.
4. **SLSA Level 3 provenance** — an in-toto attestation describing
   how the archive was built.

The two platform-specific proofs:

1. **macOS notarization** — a ticket stapled to the darwin binary,
   which Gatekeeper reads on first launch.
2. **Windows Authenticode** — a signature embedded in the windows
   binary, which SmartScreen reads on first launch.

The rebuild-from-source procedure is the seventh check: a user
clones the tagged source, rebuilds the archive, and compares the
result byte-for-byte against the released archive. It is the
strongest of the seven and the only one that defends against a
compromised build runner.

The proofs are complementary, not redundant. A user who verifies
only the checksum has checked one property; a user who verifies
all seven has checked the archive's bytes, the checksum list's
provenance, the archive's signer, the build's provenance, the two
platforms' runtime behavior, and the source's reproducibility.

---

## Before you start

You will need:

| Tool | Why | Install |
|---|---|---|
| `sha256sum` | Verify the checksum | Preinstalled on Linux. macOS: `shasum -a 256`. Windows: `certutil -hashfile <file> SHA256` or Git Bash. |
| `gpg` | Verify the GPG signature | `apt install gnupg`, `brew install gnupg`, or [gnupg.org](https://gnupg.org/download/) |
| `cosign` | Verify the cosign signature | [sigstore.dev](https://docs.sigstore.dev/cosign/installation/) |
| `slsa-verifier` | Verify the SLSA provenance | [slsa-verifier releases](https://github.com/slsa-framework/slsa-verifier/releases) |
| `git` | Rebuild from source | Any recent version |

You do not need every tool to verify every proof. The sections
below say which tool each step needs. A user who wants the
strongest verification runs all seven; a user who wants the
minimum runs step 2.

The `scripts/verify-release.sh` (Linux, macOS) and
`scripts/verify-release.ps1` (Windows) scripts automate steps 2
through 5. They are a convenience; the guide's commands are the
contract, and a user who wants to understand a check reads the
step, not the script.

---

## The release key

The Forge release key's fingerprint is:

```text
<FINGERPRINT>
```

It is recorded in [`SECURITY.md`](../../SECURITY.md) §"Release
key" alongside the key's public half. The key is published to
`keys.openpgp.org`; a user fetches it with:

```sh
gpg --keyserver keys.openpgp.org --recv-keys <FINGERPRINT>
```

The fetch is a convenience, not a proof. A user who wants to
verify the key's provenance should fetch the fingerprint from
`SECURITY.md` at a tag they trust (for example, the tag they are
verifying) and compare it to the fingerprint of the key they
fetched. The two must match.

The key is used only for signing `SHA256SUMS`. It is not used for
any other purpose. The key's rotation policy is in `SECURITY.md`
§"Release key".

---

## Step 1: Download the release and its proofs

Download the archives and the proof files from the GitHub Release
page. The names follow the convention:

```text
forge-<goos>-<goarch>.tar.gz         (or .zip on Windows)
forge-<goos>-<goarch>.<ext>.sha256
forge-<goos>-<goarch>.<ext>.cdx.json
forge-<goos>-<goarch>.<ext>.sig
forge-<goos>-<goarch>.<ext>.pem
SHA256SUMS
SHA256SUMS.asc
SHA256SUMS.gpg
forge.intoto.jsonl
forge.sbom.json
```

Not every file is present for every release:

| File | Present when |
|---|---|
| `SHA256SUMS` | Always. |
| `forge-*.sha256` | Always. |
| `SHA256SUMS.asc`, `SHA256SUMS.gpg` | The release key was available at build time. When it was not, the release notes record the blocker. |
| `forge-*.sig`, `forge-*.pem` | cosign was available and the runner had `id-token: write`. |
| `forge.intoto.jsonl` | The SLSA provenance job succeeded. |
| `forge.sbom.json`, `forge-*.cdx.json` | Always. |
| macOS notarization ticket | Embedded in the darwin binary inside the archive, not a separate file. Verified in step 6. |
| Windows Authenticode signature | Embedded in the windows binary inside the archive, not a separate file. Verified in step 7. |

Put everything you downloaded in one directory. Every command
below assumes you are in that directory.

The archive's name does not carry the version. The version is
embedded in the binary and appears in the release's URL path; the
archive's name is `forge-<goos>-<goarch>.<ext>` for every release.
The naming convention is frozen by
`internal/release/release.go`'s `ArchiveName`; a change to it is a
MAJOR change.

---

## Step 2: Verify the checksums

This step needs `sha256sum` (or `shasum -a 256` on macOS).

If you downloaded every archive:

```sh
sha256sum -c SHA256SUMS
```

Every archive must print `OK`. A line that prints `FAILED` is a
defect: the archive is not the one the release published. Do not
run it. Report the failure per §"If a verification fails".

If you downloaded only one archive, use the per-archive file:

```sh
sha256sum -c forge-linux-amd64.tar.gz.sha256
```

Both commands are equivalent for that archive. The per-archive
file is a convenience: it exists so a user who downloads one
archive does not have to download `SHA256SUMS` and find the right
line.

**What this establishes.** The archive's bytes match the published
checksum. It does not establish that the checksum came from Forge:
a checksum fetched from a compromised mirror would also match. Step
3 establishes the checksum list's provenance.

---

## Step 3: Verify the GPG signature

This step needs `gpg`.

```sh
gpg --verify SHA256SUMS.asc SHA256SUMS
```

The output must contain `Good signature from "Forge Release
Signing Key"` and the fingerprint must match the one in
`SECURITY.md`. A `BAD signature` or a fingerprint mismatch is a
defect.

If `SHA256SUMS.asc` is not present, the release was published
without the GPG signature. The release notes record why (the
release key was not available at build time) and the security
implications. A release without the GPG signature is verifiable by
steps 2, 4, and 5; the GPG proof is the only one missing.

The binary form of the signature is also published:

```sh
gpg --verify SHA256SUMS.gpg SHA256SUMS
```

The two forms are byte-for-byte equivalent signatures over the
same file. Either verifies `SHA256SUMS`. The armored form
(`.asc`) is ASCII-safe and can be pasted into an email; the binary
form (`.gpg`) is smaller. Publishing both costs two files and
avoids a downstream tool's having to convert.

**What this establishes.** The checksum list was signed by the
Forge release key. Combined with step 2, this establishes that the
archive's bytes match a checksum that Forge published. It does not
establish that the archive was built from the published source;
step 5 establishes that.

---

## Step 4: Verify the cosign signature

This step needs `cosign`.

For the archive you want to verify (the example uses
`forge-linux-amd64.tar.gz`; substitute your archive's name):

```sh
cosign verify-blob \
  --certificate-identity-regexp='^https://github.com/thapelomagqazana/forge/' \
  --certificate-oidc-issuer='https://token.actions.githubusercontent.com' \
  --certificate forge-linux-amd64.tar.gz.pem \
  --signature   forge-linux-amd64.tar.gz.sig \
  forge-linux-amd64.tar.gz
```

The output must contain `Verified OK`. The two `--certificate-*`
arguments pin the signer to the Forge repository on GitHub
Actions:

- `--certificate-identity-regexp` matches the workflow path and
  the tag the workflow ran on. Without it, `cosign` would accept
  any Sigstore signature, which would not prove the archive came
  from Forge.
- `--certificate-oidc-issuer` pins the identity provider to
  GitHub's. Without it, `cosign` would accept a signature from a
  different OIDC issuer.

The verification does not require a shared secret. It reads the
signature and the certificate from disk and checks them against
Sigstore's public transparency log. The certificate's identity and
the OIDC issuer are recorded in `SECURITY.md`.

If `forge-<goos>-<goarch>.<ext>.sig` is not present, the archive
was published without the cosign signature. The release notes
record why. A release without the cosign signature is verifiable
by steps 2, 3, and 5.

**What this establishes.** The archive was signed by a workload
identity from the Forge repository on GitHub Actions. It does not
establish that the archive was built from a specific commit; step
5 establishes that.

---

## Step 5: Verify the SLSA provenance

This step needs `slsa-verifier`.

```sh
slsa-verifier verify-artifact \
  --provenance-path forge.intoto.jsonl \
  --source-uri github.com/thapelomagqazana/forge \
  forge-linux-amd64.tar.gz
```

The output must contain `PASSED: SLSA verification passed`. The
`--source-uri` argument pins the source repository; a provenance
attestation from a different repository would not satisfy the
check.

If `forge.intoto.jsonl` is not present, the release was published
without the SLSA provenance attestation. The release notes record
why. A release without the provenance attestation is verifiable by
steps 2, 3, and 4.

**What this establishes.** The archive was built by the Forge
build workflow from a specific commit, on a specific runner, with
the documented builder. It does not establish that the build's
inputs were not compromised: SLSA Level 3 defends against some —
not all — build-time attacks. The provenance's `byproducts` field
records the build's inputs, which a user can inspect for
unexpected values.

---

## Step 6: Verify macOS notarization (darwin archives only)

This step needs a macOS host with Gatekeeper enabled. It does not
need any of the tools listed above.

### Before running the checks

Check the release notes for a notarization blocker. A release
published without the notarization credential is not a
verification failure: the release notes record the blocker, and
the binary is expected to fail `stapler validate` and to trigger
Gatekeeper's warning. The failure is a documented outcome, not a
defect. A user who reports a documented blocker as a security
incident wastes the security team's time; the release notes are
the first place to look.

If the release notes record no blocker, the binary is expected to
be notarized. The steps below verify that it is.

### The checks

Extract the archive and launch the binary:

```sh
tar -xzf forge-darwin-arm64.tar.gz
./forge version
```

The binary must launch without a Gatekeeper warning. If Gatekeeper
blocks it with a dialog that says the developer cannot be
verified, and the release notes record no blocker, the binary is
not notarized. Report the failure per §"If a verification fails".

The notarization ticket is stapled to the binary and is visible
to `stapler`:

```sh
stapler validate ./forge
```

The output must contain `The validate action worked!`. A binary
that is signed but not stapled passes `codesign --verify` but
fails `stapler validate`; the staple is what lets a macOS host
verify the ticket without a network round-trip.

To inspect the signature without launching the binary:

```sh
codesign --verify --deep --strict --verbose=2 ./forge
```

The output must contain `valid on disk` and `satisfies its
Designated Requirement`. The first line proves the signature is
intact; the second proves the signature matches the identity the
binary claims.

**What this establishes.** The binary inside the archive will run
on a macOS host with Gatekeeper enabled without a security
warning. It does not establish that the archive came from Forge;
steps 2 through 5 establish that.

---

## Step 7: Verify Windows Authenticode (windows archives only)

This step needs a Windows host with `signtool.exe` (Windows SDK).
It does not need any of the tools listed above.

### Before running the checks

Check the release notes for an Authenticode blocker. A release
published without the code-signing certificate is not a
verification failure: the release notes record the blocker, and
the binary is expected to trigger SmartScreen's warning. The
failure is a documented outcome, not a defect. A user who reports
a documented blocker as a security incident wastes the security
team's time; the release notes are the first place to look.

If the release notes record no blocker, the binary is expected to
be signed. The steps below verify that it is.

### The checks

Extract the archive and verify the signature:

```powershell
Expand-Archive forge-windows-amd64.zip -DestinationPath .
signtool verify /pa /v .\forge.exe
```

The output must contain `Successfully verified` and the signer's
name must match the one in the release notes. A binary whose
signature is invalid, or whose signer does not match, and whose
release notes record no blocker, is a defect. Report it per §"If a
verification fails".

To inspect the signature with PowerShell:

```powershell
Get-AuthenticodeSignature .\forge.exe | Format-List
```

The `Status` field must be `Valid`. The `SignerCertificate`
field's `Subject` must match the signer documented in the release
notes.

Launching the binary is the end-to-end check: an unsigned binary
triggers SmartScreen's "Windows protected your PC" dialog on
first launch. A signed binary does not.

**What this establishes.** The binary inside the archive will run
on a Windows host without a SmartScreen warning. It does not
establish that the archive came from Forge; steps 2 through 5
establish that.

---

## Step 8: Rebuild from source and compare (optional)

The strongest check is to rebuild the archive from the tagged
source and compare the result byte-for-byte against the released
archive. If the two match, the release is the source's exact
output, and no build-time attack that would alter the binary could
have occurred.

The procedure is in
[`docs/guides/reproducible-builds.md`](../guides/reproducible-builds.md).
The short version, for `linux/amd64`:

```sh
# 1. Clone the source at the tag.
git clone https://github.com/thapelomagqazana/forge.git
cd forge
git checkout v1.4.2

# 2. Run the reproducible build.
SOURCE_DATE_EPOCH=$(git log -1 --format=%ct) \
  task build --platform=linux/amd64

# 3. Compare the archive's SHA-256 against the released one.
sha256sum dist/forge-linux-amd64.tar.gz
```

The output's digest must equal the digest in the released
`SHA256SUMS`. A mismatch means the release was not produced from
the tagged source, or the rebuild's environment differs from the
pipeline's. The rebuild guide names the differences that matter
(the Go toolchain's version, `CGO_ENABLED`, the build host's
timezone) and the ones that do not (the build host's path, the
build's user).

A user who does not want to run the full rebuild can run the
released binary's `forge version --format=json` and compare its
`vcs.revision` field to the tag's commit. That is a weaker check —
it reads the binary's embedded metadata, which an attacker who
built the binary could forge — but it catches an archive from a
different tag.

**What this establishes.** The archive's bytes are the tagged
source's exact output under the pipeline's build configuration. It
is the strongest of the eight checks and the only one that defends
against a compromised build runner. It does not establish that the
source tag itself is honest; the repository's access controls and
the commit's signature are the defenses for that.

---

## What each proof establishes

| Proof | Establishes | Does not establish |
|---|---|---|
| SHA-256 checksum | The archive's bytes match the published checksum. | That the checksum came from Forge. A checksum from a compromised mirror would pass. |
| GPG signature | The checksum list was signed by the Forge release key. | That the archive was built from the published source. |
| cosign signature | The archive was signed by a workload identity from the Forge repository on GitHub Actions. | That the archive was built from a specific commit. |
| SLSA provenance | The archive was built by the Forge build workflow from a specific commit, on a specific runner, with the documented builder. | That the build's inputs were not compromised. SLSA Level 3 defends against some — not all — build-time attacks. |
| macOS notarization | The binary will run on a macOS host with Gatekeeper enabled without a warning. | That the archive came from Forge. |
| Windows Authenticode | The binary will run on a Windows host without a SmartScreen warning. | That the archive came from Forge. |
| Rebuild from source | The archive is the tagged source's exact output under the pipeline's build configuration. | That the source tag itself is honest. |
| `vcs.revision` in the binary | The archive names the commit it was built from. | That the commit's contents are what the archive contains. |

The eight are complementary. A user who verifies all eight has
checked the archive at eight different points in its lifecycle. A
user who verifies only the checksum has checked one.

---

## Verifying on Windows

The commands in steps 2 through 5 are the same on Windows. The
tools publish Windows binaries:

- `sha256sum` is available in Git Bash or WSL. Native Windows:
  `certutil -hashfile <file> SHA256`. The output format differs
  from `sha256sum`; compare the hex digest manually.
- `gpg`, `cosign`, and `slsa-verifier` publish Windows binaries.
  Download them from their release pages and put them on `PATH`.

Step 7 is Windows-specific and does not run on other hosts.
Step 6 is macOS-specific and does not run on Windows.
Step 8 runs on Windows with the same `task build` invocation as on
Linux and macOS, provided the Windows host's Go toolchain matches
the version in `go.mod`.

The `scripts/verify-release.ps1` script automates steps 2 through
5 on Windows. It uses the tools' Windows binaries and produces the
same output as the Unix script.

---

## Verifying on macOS

`sha256sum` is not preinstalled on macOS; use `shasum -a 256`:

```sh
shasum -a 256 -c SHA256SUMS
```

The rest of the commands in steps 2 through 5 are the same. `gpg`,
`cosign`, and `slsa-verifier` are available via Homebrew.

Step 6 is macOS-specific and does not run on other hosts.
Step 7 is Windows-specific and does not run on macOS.
Step 8 runs on macOS with the same `task build` invocation as on
Linux.

The `scripts/verify-release.sh` script automates steps 2 through 5
on macOS. It uses `shasum -a 256` when `sha256sum` is not on
`PATH`; the script's preflight detects which of the two the host
has.

---

## Verifying without network access

Steps 2 and 3 work offline once the release key is in the local
GPG keyring. Steps 4 and 5 require network access: `cosign` reads
Sigstore's Rekor log, and `slsa-verifier` reads the provenance's
transparency log entry. Step 8 requires network access for the
initial `git clone` and then runs offline.

A user who needs offline verification of steps 4 and 5 can cache
the tools' proofs and run them with their offline flags:

```sh
# cosign, offline, using the certificate in the release's .pem file.
cosign verify-blob \
  --certificate-identity-regexp='^https://github.com/thapelomagqazana/forge/' \
  --certificate-oidc-issuer='https://token.actions.githubusercontent.com' \
  --certificate forge-linux-amd64.tar.gz.pem \
  --signature   forge-linux-amd64.tar.gz.sig \
  --offline \
  forge-linux-amd64.tar.gz
```

```sh
# slsa-verifier, offline, using the release's provenance.
slsa-verifier verify-artifact \
  --provenance-path forge.intoto.jsonl \
  --source-uri github.com/thapelomagqazana/forge \
  --offline \
  forge-linux-amd64.tar.gz
```

The `--offline` flag reads the tool's cache instead of the
network. A user who runs the offline form without a populated cache
sees the tool report a missing entry; the failure's message names
the entry the tool looked for. The offline workflow is not the
default because it requires the user to populate the cache in
advance, which the default workflow does automatically.

A user who wants the strongest offline verification uses steps 2,
3, and 8: the checksum and the GPG signature are offline after the
key is fetched once, and the rebuild is offline after the source
is cloned once. The two skip the cosign and SLSA proofs, which are
the two that require network access. The tradeoff is the two
proofs' absence; the tradeoff's cost is the two proofs' strength.

---

## If a verification fails

Do not run the archive. Report the failure to
`security@forge.dev` with:

- The archive's name.
- The command you ran.
- The complete output.
- The fingerprint or certificate you saw.

A verification failure on a released artifact is a security
incident. The release is withdrawn from the distribution channels
until the failure is explained. The rollback procedure is in
[`docs/guides/rollback.md`](../guides/rollback.md). The release
notes and the GitHub Release page are updated with the
explanation; a corrected release is published under a new tag.

A verification failure is not a bug report against this document.
This document's commands are tested end-to-end in CI against every
release; a command that does not work is a defect in the release
pipeline, and the release is withdrawn.

---

## Why the release uses these proofs

The proofs defend against different threats:

| Threat | Proof that defends against it |
|---|---|
| A mirror serves a tampered archive. | SHA-256 checksum (step 2). |
| A mirror serves a tampered checksum list. | GPG signature (step 3). |
| An attacker publishes a signed archive from a fork. | cosign signature with the pinned identity (step 4). |
| An attacker builds the archive from a modified source tree. | SLSA provenance (step 5). |
| An attacker's darwin binary runs on a user's macOS host. | macOS notarization (step 6). |
| An attacker's windows binary runs on a user's Windows host. | Windows Authenticode (step 7). |
| An attacker compromises the build runner. | Rebuild from source (step 8). |

No single proof defends against all seven. A checksum without a
signature is only as trustworthy as the channel it arrived on. A
signature without provenance is only as trustworthy as the
signer's build process. The proofs together give a user a chain of
evidence from the bytes they downloaded to the source commit they
trust.

The security policy is in [`SECURITY.md`](../../SECURITY.md).

---

## Related documents

| Document | Role |
|---|---|
| [`SECURITY.md`](../../SECURITY.md) | The release key's fingerprint, the cosign identity, and the security contact. |
| [`docs/reference/release-channels.md`](release-channels.md) | The channels each artifact is published to, and the support window. |
| [`docs/guides/reproducible-builds.md`](../guides/reproducible-builds.md) | Step 8: rebuild from source. |
| [`docs/guides/verifying-binaries.md`](../guides/verifying-binaries.md) | The binary verification suite the release pipeline runs before signing. |
| [`docs/guides/rollback.md`](../guides/rollback.md) | The post-publish rollback procedure. |
