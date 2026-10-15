<!--
  Security Policy
  ===============
  Public contract. The source of truth for:
    - the security contact,
    - the release key's fingerprint,
    - the cosign identity and OIDC issuer,
    - the SLSA provenance verifier,
    - the credential-handling policy for the release pipeline.

  Editing rules:
    1. Every fingerprint, identity, and issuer in this document
       MUST match the ones the release pipeline uses. The pipeline
       is the source of truth; this document copies.
    2. A rotation of the release key, the cosign identity, or the
       OIDC issuer MUST update this document in the same PR that
       changes the pipeline.
    3. The document MUST NOT instruct a reader to trust a value
       without a way to verify it independently.
-->

# Security Policy

## Reporting a vulnerability

Report suspected vulnerabilities to `security@forge.dev`. Do not
open a public issue. If you prefer an encrypted channel, use the
PGP key at `https://keys.openpgp.org/vks/v1/by-fingerprint/<SECURITY_FINGERPRINT>`;
the fingerprint is published in §"Security contact key" below.

We acknowledge reports within 48 hours and aim to publish a fix
within 30 days. We credit reporters in the release notes unless
they ask to remain anonymous.

We follow a 90-day coordinated disclosure window. If a fix is not
available within 90 days of a confirmed report, we publish an
advisory describing the issue and any available mitigations, and
we credit the reporter. We will extend the window if the reporter
and the maintainers agree that an extension serves users better.

## Supported versions

The most recent stable release is supported. Older minor releases
receive security fixes for 90 days after the next minor's release.
The support window is documented in
[`docs/reference/release-channels.md`](docs/reference/release-channels.md)
§"Support".

The `nightly` channel is not supported. A nightly build is a
snapshot of `main`; it is published for testing and is deleted
after 30 days. Do not deploy a nightly build. The `prerelease`
channel is supported for the current minor only; a prerelease
that has been superseded by a stable release of the same minor is
no longer supported.

## Release verification

Every Forge release is signed and accompanied by four proofs of
integrity. The verification procedure is in
[`docs/reference/verifying-releases.md`](docs/reference/verifying-releases.md).

The four proofs, and what each establishes:

| Proof | Establishes |
|---|---|
| SHA-256 checksums | The archive's bytes match the published checksum. |
| GPG signature over `SHA256SUMS` | The checksum list was signed by the Forge release key. |
| cosign signature over each archive | The archive was signed by a workload identity from the Forge repository on GitHub Actions. |
| SLSA Level 3 provenance | The archive was built by the Forge build workflow from a specific commit, on a specific runner, with the documented builder. |

The proofs are complementary. A user who verifies only the
checksum has checked one property; a user who verifies all four
has checked the archive at four different points in its lifecycle.
The verification guide explains which proof defends against which
threat.

---

## Release key

The Forge release key's fingerprint is:

```text
<FINGERPRINT>
```

The key's public half is published at
`https://keys.openpgp.org/vks/v1/by-fingerprint/<FINGERPRINT>`.
The key is used only for signing `SHA256SUMS` and its per-archive
companions; it is not used for any other purpose.

### Fetching the key

```sh
gpg --keyserver keys.openpgp.org --recv-keys <FINGERPRINT>
```

### Verifying the key's provenance

The fetch above is a convenience, not a proof. A user who wants to
verify the key's provenance should:

1. Fetch the fingerprint from this document at a tag they trust
   (for example, the tag they are verifying), or from a signed
   commit on `main` that they trust.
2. Fetch the key from `keys.openpgp.org`.
3. Compare the fingerprint of the fetched key to the fingerprint
   in step 1. The two must match.

A key fetched from `keys.openpgp.org` without step 1 proves only
that the keyserver served *some* key with that fingerprint. It does
not prove that the key is Forge's. The out-of-band comparison is
what establishes provenance.

### Rotation

The key is rotated when:

- a maintainer with access to the private half leaves the project;
- the key is suspected to be compromised;
- the key's algorithm is deprecated by the OpenPGP community.

A rotation is announced in the release notes of the first release
signed with the new key. The old key is retained in the keyring in
a revoked state so that releases signed with it remain verifiable.
`SECURITY.md` at the first release's tag records both the new
fingerprint and the old fingerprint's revocation.

A release that ships after a rotation but before this document is
updated is a defect. The pipeline's `route` job reads the
fingerprint from `FORGE_GPG_KEY_FINGERPRINT`; a mismatch between
the secret and this document fails the release.

### Key handling

The private half is stored in the repository's GitHub Actions
secrets as `FORGE_GPG_PRIVATE_KEY` (base64-encoded) and
`FORGE_GPG_KEY_FINGERPRINT`. The key is imported into an ephemeral
CI runner at release time and never written to disk in the
repository.

The key's passphrase, if it has one, is not stored in the
repository. The `checksums` job imports the key with
`gpg --batch --import`; a passphrase-protected key would require
`--pinentry-mode loopback --passphrase-fd` and a `GPG_PASSPHRASE`
secret. Forge's release key has no passphrase, because the private
half is stored in a secret store that is already access-controlled
and audited; a passphrase would add a second secret with the same
access controls and no additional defense.

---

## cosign identity

Release archives are signed with cosign keyless. The signature's
certificate identity is:

```text
https://github.com/thapelomagqazana/forge/.github/workflows/release.yml@refs/tags/<version>
```

The `<version>` is the tag being released (for example,
`v1.4.2`). The OIDC issuer is:

```text
https://token.actions.githubusercontent.com
```

A user verifies with `cosign verify-blob` and the two
`--certificate-*` arguments documented in
[`docs/reference/verifying-releases.md`](docs/reference/verifying-releases.md)
§"Step 4". The verification does not require a shared secret; the
proof is the Sigstore transparency log.

### Why keyless

Keyless signing removes a long-lived signing key. The identity is
the CI's OIDC token; the proof is the Rekor log entry. There is no
signing key to leak, no rotation to schedule, and no keyring to
distribute. The tradeoff is a network dependency at signing time
and at verification time. A future ADR could add a KMS-backed
fallback for offline verification; the current policy is keyless
only.

### Certificate lifetime

The certificate cosign obtains from Fulcio is valid for ten
minutes. The signature and the certificate are written to disk
together; the verification in step 4 uses the certificate from the
release, not a freshly issued one. A certificate whose validity
window has passed still verifies because the Rekor log entry
proves the signature was made while the certificate was valid.

---

## SLSA provenance

Releases are accompanied by a SLSA Level 3 provenance attestation,
generated by
[`slsa-github-generator`](https://github.com/slsa-framework/slsa-github-generator)
at version `v2.0.0`. The attestation is verified with
[`slsa-verifier`](https://github.com/slsa-framework/slsa-verifier).
The procedure is in
[`docs/reference/verifying-releases.md`](docs/reference/verifying-releases.md)
§"Step 5".

The provenance's `builder.id` is the
`slsa-github-generator` workflow's identity. The `materials` field
records the source commit and the workflow file. A user who wants
to inspect the provenance without verifying the artifact can read
`forge.intoto.jsonl` directly; it is a JSON Lines document, one
attestation per line.

### What SLSA Level 3 establishes

SLSA Level 3 establishes that the artifact was built by a
specific, reproducible build process on a specific runner. It does
not establish that the build's inputs were not compromised: an
attacker who can modify the source tree before the workflow runs,
or who can modify the workflow itself, can produce a valid
provenance attestation for a malicious artifact. The source
repository's access controls and the workflow's review requirements
are the defenses against that class of attack; the provenance
makes the attack observable after the fact.

---

## Signing keys and their handling

The release pipeline uses five credentials. Each is stored in a
GitHub Actions secret; each has a documented scope and rotation
policy.

| Credential | Secret | Scope | Rotation |
|---|---|---|---|
| GPG release key | `FORGE_GPG_PRIVATE_KEY` (base64), `FORGE_GPG_KEY_FINGERPRINT` | Signs `SHA256SUMS` and its per-archive companions. | On maintainer departure, suspected compromise, or algorithm deprecation. |
| Apple Developer ID Application certificate | `FORGE_CODESIGN_IDENTITY`, `FORGE_NOTARY_KEYCHAIN_PROFILE` | Signs and notarizes darwin binaries. | On certificate expiry (Apple certificates are valid for one year). |
| Apple notary credentials | `FORGE_NOTARY_KEYCHAIN_PROFILE` | Submits binaries to Apple's notary service. | On Apple ID password change or keychain profile rotation. |
| Windows Authenticode certificate | `FORGE_AUTHENTICODE_SUBJECT`, `FORGE_AUTHENTICODE_SHA1` | Signs windows binaries. | On certificate expiry (code-signing certificates are valid for one to three years). |
| cosign keyless | None. | Signs archives. | Not applicable: keyless uses a short-lived certificate per run. |

### Why the GPG key is not passphrase-protected

The private half is stored in a GitHub Actions secret, which is
already access-controlled and audited. A passphrase would add a
second secret with the same access controls and no additional
defense: an attacker who can read the private key from the secret
store can also read the passphrase from the same store. A
passphrase defends against a different threat model — a private
key stolen from a developer's laptop — which does not apply to a
key that exists only in the secret store.

A future change that moved the key to a developer's machine would
require a passphrase; the current pipeline does not.

### Why the cosign flow has no key to rotate

The cosign keyless flow uses no long-lived key. cosign obtains a
short-lived certificate from Sigstore's Fulcio CA, using the CI
runner's OIDC token. The certificate is valid for ten minutes. The
signature is recorded in Sigstore's Rekor transparency log. There
is no key to rotate, because there is no key.

### Apple and Windows certificates

The Apple and Windows certificates are issued by their respective
platform vendors. Their rotation is driven by the vendor's expiry
policy, not by Forge. A certificate that is about to expire is
renewed and the new certificate's identity is updated in the
secret store; the first release after the renewal records the new
identity in its release notes.

A release that ships after a certificate expiry is a defect. The
`sign` job probes for the certificate before signing; a missing or
expired certificate is recorded as a documented blocker, not as a
silent skip.

---

## Supply-chain security

The build pipeline enforces:

- **Reproducible builds.** A user can rebuild an archive from
  source and obtain a byte-identical result. The procedure is in
  [`docs/guides/reproducible-builds.md`](docs/guides/reproducible-builds.md).
- **`CGO_ENABLED=0` for every release binary.** A static binary has
  no dynamic-library dependency that could differ between the
  build host and the user's host.
- **`-trimpath`, `-buildid=`, and `-buildvcs=true` for every
  build.** `-trimpath` removes the build host's paths from the
  binary; `-buildid=` removes a non-deterministic build ID;
  `-buildvcs=true` embeds the source commit for provenance.
- **A binary verification suite** that inspects every artifact
  before it is signed. The suite is in
  [`docs/guides/verifying-binaries.md`](docs/guides/verifying-binaries.md).
- **A pinned toolchain.** The `go.mod` file pins the Go version;
  the `toolchain` directive pins the exact patch version. A build
  that uses a different toolchain fails the pipeline.
- **A pinned dependency graph.** The `go.sum` file pins every
  module's hash. A build whose `go.sum` does not match fails
  before compilation begins.
- **A minimal dependency set.** The `scripts/deps/` checks assert
  that the module graph contains only the documented direct and
  indirect dependencies. A new dependency requires a documented
  review; the check fails otherwise.

---

## Security contact key

The PGP key for encrypted reports is:

```text
<SECURITY_FINGERPRINT>
```

The key's public half is published at
`https://keys.openpgp.org/vks/v1/by-fingerprint/<SECURITY_FINGERPRINT>`.
This key is distinct from the release key. The release key signs
releases; the security contact key decrypts reports. They are
separate so that a compromise of one does not compromise the
other.

The security contact key is rotated on the same schedule as the
release key and for the same reasons. A rotation is announced in
the release notes.

---

## Threat model

The supply-chain integrity pipeline defends against four threats:

| Threat | Defense |
|---|---|
| A mirror serves a tampered archive. | SHA-256 checksums (release verification §"Step 2"). |
| A mirror serves a tampered checksum list. | GPG signature over `SHA256SUMS` (release verification §"Step 3"). |
| An attacker publishes a signed archive from a fork. | cosign signature with the pinned identity (release verification §"Step 4"). |
| An attacker builds the archive from a modified source tree. | SLSA provenance (release verification §"Step 5"). |

The pipeline does not defend against:

- **A compromised maintainer.** A maintainer who can push to the
  release workflow can produce a signed artifact from a modified
  source tree. The SLSA provenance makes the modification
  observable; it does not prevent it. The defense is the
  repository's access controls and the workflow's review
  requirements.
- **A compromised GitHub Actions runner.** A runner that is
  compromised at build time can produce a valid provenance
  attestation for a malicious artifact. GitHub's hosted runners
  are the current mitigation; a self-hosted runner would require
  additional controls.
- **A compromised Go module proxy.** The `go.sum` file pins every
  module's hash; a proxy that serves a different module fails the
  build. A proxy that serves the pinned module but has modified
  its source before hashing is not defended against by `go.sum`;
  the module's own provenance and the module's upstream access
  controls are the defense.
- **A compromised Sigstore.** cosign's verification reads the
  Rekor transparency log. A compromised Sigstore that served a
  forged log would defeat the verification. The defense is
  Sigstore's own auditability and the community's monitoring of
  the log; a future ADR could add a second, independent
  transparency log.

The threat model is reviewed when a new signing method is added,
when a credential's storage location changes, and when a new
platform tier is added.

---

## Contact

- Security reports: `security@forge.dev`
- Release engineering: `release@forge.dev`
- General: `hello@forge.dev`

For a security report, use the PGP key in §"Security contact key"
if the report contains exploit details or other sensitive
material. A report without sensitive material can be sent in
plaintext.

Do not open a public issue for a suspected vulnerability. A
public issue that describes an unpatched vulnerability puts every
user at risk; the coordinated disclosure window exists to give
maintainers time to publish a fix before the issue is public.
