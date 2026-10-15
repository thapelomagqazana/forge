# ADR-040: Forge Is Distributed as a Binary Project

**Status:** Proposed
**Date:** 2026-10-15
**Deciders:** [@thapelomagqazana]
**Supersedes:** —
**Superseded by:** —

---

## 1. Context

Forge is a CLI tool that modifies a user's project directory: it
scaffolds files, updates configuration, and runs project-level
commands. A user who runs Forge has already decided to trust it with
write access to their working tree. The threat model is therefore not
"a user who might run a malicious binary" but "a user who has already
run a binary and whose project directory is now at the binary's
mercy." The bar for what "the binary" means is correspondingly high.

Forge's source is public and permissively licensed. A user who wants
to build it can clone the repository and run `go build`. That path
exists today and will continue to exist. The question this ADR
answers is a different one: what is the *primary* channel by which a
user obtains Forge, and what obligations does that channel create?

The Go ecosystem offers two shapes. A source project tells users to
`go install example.com/tool@latest`; the user's toolchain fetches,
compiles, and installs the binary. A binary project tells users to
download a prebuilt archive from a release page, or install it via a
package manager; the user's machine runs bytes the project produced.
The two shapes create different obligations: a source project owes
its users a working module path, a stable API, and a `go.mod` that
resolves; a binary project owes its users a *reason to trust the
bytes*. That reason is the supply-chain proof: reproducible builds,
checksums, signatures, provenance, and an SBOM.

Forge's product brief treats the binary as the product: "a single
cross-platform binary." The brief also names `go install` as one
supported channel. The two statements are consistent only if the
binary is the primary channel and `go install` is a supported
alternative — the reverse ordering would invert the obligations and
leave the binary channel under-specified.

No decision means the obligations are ambiguous. A release pipeline
that produces artifacts without signatures, or a module path that
drifts without an ADR, or a user who downloads an unsigned binary
because nothing told them not to — each is a failure mode the
absence of a decision permits.

The decision affects every release-engineering task in Phase 2 and
every installation-facing document in Phase 6. It also affects the
project's security posture, which the release pipeline must enforce
rather than assume.

---

## 2. Options Considered

### Option A — Source project: `go install` is the primary channel

Forge is published as a Go module. Users obtain it by running
`go install github.com/forge-cli/forge/cmd/forge@latest`. The Go
toolchain fetches the source, verifies it against the module
proxy's checksum database, compiles it locally, and installs the
resulting binary. The project's obligations are the Go ecosystem's:
a stable module path, a resolvable `go.mod`, a documented minimum
Go version, and a semver discipline that respects Go's module
versioning rules.

**Pros:**

- **No release engineering.** The project does not need a build
  matrix, a signing key, a notarization credential, or a release
  pipeline. A tag and a `go.mod` are the whole release.
- **The user's toolchain verifies the source.** The Go module
  proxy's checksum database is a transparency log; a proxy that
  serves a modified module fails the checksum check. The trust
  anchor is the Go ecosystem's, not the project's.
- **The binary is always the user's own build.** A user who runs
  `go install` gets a binary compiled on their machine from
  source they can read. There is no "trust the downloaded bytes"
  problem.
- **The distribution channels are the Go ecosystem's.** No
  Homebrew tap, no Scoop bucket, no winget manifest, no apt
  repository. The channels that exist for Go modules work
  automatically.

**Cons:**

- **Every user needs a Go toolchain.** A user who does not have Go
  installed cannot install Forge. The audience is narrowed to Go
  developers and to users willing to install a toolchain for one
  CLI.
- **The user's toolchain must match the project's.** A user on an
  older Go version may fail to build a newer Forge. The project
  must maintain compatibility with a range of Go versions, which
  constrains its use of newer language features.
- **First-run cost is high.** `go install` compiles the project;
  the first install takes minutes. Subsequent installs are
  cached, but the first is the one a new user experiences.
- **Version skew is invisible.** A user's `go install` resolves
  `@latest` to whatever the proxy serves. The user has no
  artifact to compare against a release's checksum; the "release"
  is whatever the proxy's current `latest` is.
- **The signing obligation moves to the source.** The source tag
  itself must be signed, and the repository's access controls
  must be strong enough that a compromised maintainer cannot
  publish a malicious tag. The project has no way to publish a
  post-hoc revocation of a tag once the proxy has cached it.

### Option B — Binary project: a signed release archive is the primary channel

Forge is published as a release. The release pipeline
cross-compiles the binary for six platforms, produces a signed
archive per platform, publishes the archives alongside their
checksums, signatures, provenance attestation, and SBOM, and
distributes the archives through the channels a user's package
manager reads. A user on macOS runs `brew install
forge-cli/tap/forge`; a user on Windows runs `scoop install forge`
or `winget install Forge.Forge`; a user on Linux runs `apt install
forge` or downloads the archive directly. `go install` remains
supported but is documented as a secondary channel.

**Pros:**

- **The audience is every user, not only Go developers.** A user
  who has never installed a Go toolchain can install Forge via
  their platform's package manager.
- **The release is a signed, versioned artifact.** Every archive
  has a checksum, a GPG signature over the checksum list, a
  cosign signature over the archive, a SLSA provenance
  attestation, and an SBOM. A user who wants to verify has the
  material to do so.
- **The release is reproducible.** The pipeline pins the
  toolchain, sets `SOURCE_DATE_EPOCH` to the commit's
  timestamp, applies `-trimpath`, and builds with
  `CGO_ENABLED=0`. Two builds from the same commit produce
  identical bytes. A user who wants to rebuild from source can
  compare the result to the released archive.
- **The package managers handle updates.** Homebrew, Scoop,
  winget, and apt each have an upgrade mechanism. A user who
  installs via one of them gets new versions without
  re-downloading an archive by hand.
- **The provenance attestation names the commit.** A user can
  read the attestation to see which commit, which workflow, and
  which runner produced the archive. The Go module proxy's
  checksum database does not give this.

**Cons:**

- **The release engineering is substantial.** The project needs a
  cross-compilation pipeline, a signing key, a notarization
  credential, an Authenticode certificate, and a release
  workflow. Each is a credential to manage and a failure mode to
  handle.
- **The user must trust the download.** A binary channel's
  central problem: the user runs bytes they did not compile.
  The pipeline's proofs are the mitigation, but they only work
  if the user runs them, and most users do not.
- **A compromised signing key is catastrophic.** An attacker with
  the release key can sign a malicious archive. The mitigation
  is keyless signing (cosign's OIDC flow) and a short-lived
  certificate; the pipeline uses both, but the GPG key remains a
  long-lived credential that must be rotated carefully.
- **The package managers are third parties.** Homebrew, Scoop,
  winget, and apt each have their own review processes and their
  own failure modes. A manifest that fails the winget
  maintainers' review does not ship; a formula that a Homebrew
  user cannot install is a support burden the project owns.

### Option C — Both channels, with the binary as primary and the source as equal

Forge is published as both a signed release and a Go module, and
the two are documented as equally supported. The release page and
the module path are both first-class; a user picks the one that
fits their environment, and the documentation presents the two
without ranking them.

**Pros:**

- **The user picks the channel that fits.** A Go developer uses
  `go install`; a macOS user uses Homebrew; a Windows user uses
  Scoop or winget.
- **The two channels cross-check each other.** A user who
  installs via `go install` can compare the binary's
  `vcs.revision` to the release's provenance attestation. A
  user who installs via Homebrew can `go install` the same
  version and compare the binaries' behavior.

**Cons:**

- **Two obligations, not one.** The project must maintain the
  module path *and* the release pipeline. Each is a
  compatibility contract; each can break independently.
- **Documentation ranks nothing.** A user-facing guide that
  presents two channels as equal gives the reader no guidance.
  The reader must decide, without the project's help, which
  channel is the one to pick.
- **The obligations differ, and the documentation hides it.**
  The source channel owes the user a resolvable `go.mod`; the
  binary channel owes the user a reason to trust the bytes. A
  guide that presents the two as the same does not tell the
  user what each channel's trust story is.

---

## 3. Decision

Forge is distributed as a **binary project**. The primary channel
is the signed release archive, published alongside its checksums,
GPG signature, cosign signature, SLSA provenance, and SBOM, and
distributed through the six channels the platform matrix names
(direct download, Homebrew, Scoop, winget, apt, Docker). `go
install` remains a supported channel; it is documented as a
secondary channel in the installation guide, and the module path
is frozen by a companion ADR.

The decision was tipped by the audience. Forge's product brief
names adoption outside the Go community as a goal; a
`go install`-only project narrows the audience to users who already
have a Go toolchain. The binary channel's obligations — the release
pipeline, the signing key, the notarization credential — are real,
but they are obligations the project must meet anyway if it is to
ship a binary at all. The source channel's obligations are smaller,
but they do not substitute: a user who wants Forge and does not
have Go is a user the source channel cannot serve.

The trust story is the binary channel's central cost, and it is
the one the decision accepts knowingly. The pipeline's proofs are
the mitigation; they only work if the user runs them, and the
project cannot force the user to. What the project *can* do is make
the proofs easy to run — the verification guide and the two scripts
— and make the released archive reproducible so a user who wants to
rebuild from source can. The decision accepts that most users will
not run the proofs; the decision's job is to make the proofs
available and correct for the users who do.

The decision depends on two assumptions that could be falsified:

- **The signing credentials are acquirable.** macOS notarization
  requires an Apple Developer account; Windows Authenticode
  requires a code-signing certificate. If either is unavailable
  when the first public release is cut, the corresponding
  platform's archive ships without that proof. The pipeline
  treats the missing credential as a documented blocker rather
  than a failure; the release notes record it.
- **The reproducibility discipline holds.** Two builds from the
  same commit produce identical bytes only if the toolchain is
  pinned, the build environment is controlled, and the sources
  of nondeterminism are enumerated. The pipeline enforces the
  first two; the third is documented in a companion ADR.

The decision is not provisional. A future change that reversed it
— "Forge is a source project" — would be a new ADR that
supersedes this one, and would need to answer what happens to the
release pipeline and the users who installed via a package
manager.

---

## 4. Consequences

### 4.1 Positive

- **The audience includes every platform Forge supports, not only
  Go developers.** A user on macOS, Windows, or Linux installs
  Forge via a package manager they already use.
- **The release is a signed, verifiable artifact.** Every archive
  has four proofs; the verification guide and the scripts make
  the proofs runnable by a user who wants them.
- **The release is reproducible.** A user who wants to rebuild
  from source can, and can compare the result to the released
  archive. The reproducibility discipline is enforced by the
  pipeline, not asserted by the documentation.
- **The provenance attestation names the commit, workflow, and
  runner.** A user can read `forge.intoto.jsonl` to see exactly
  which build produced the archive.
- **The package managers handle updates.** A user who installs
  via Homebrew, Scoop, winget, or apt gets new versions without
  re-downloading a release archive by hand.
- **The decision's obligations are testable.** The release
  pipeline's jobs, the signature checks, the SBOM's shape, and
  the reproducibility check are each a test; a regression is
  caught before a release ships.

### 4.2 Negative

- **The release engineering is substantial.** The project
  maintains a cross-compilation pipeline, a signing key, an
  Apple Developer account, a Windows code-signing certificate,
  and a release workflow. Each is a credential to manage and a
  failure mode to handle. A small project would find this
  burden disproportionate; Forge accepts it because the audience
  demands it.
- **The user must trust the download, and most users will not
  verify.** The pipeline's proofs are only effective if the user
  runs them. The decision accepts that most users will not; the
  decision's mitigation is to make the proofs easy to run, not
  to make the user run them.
- **A compromised GPG key is catastrophic.** The release key
  signs `SHA256SUMS`; an attacker with the key can sign a
  malicious checksum list. The mitigation is the key's rotation
  policy and its storage in a secret store; the mitigation does
  not eliminate the risk.
- **A compromised Apple or Windows credential is catastrophic
  for the platform.** An attacker with the notarization
  credential can notarize a malicious darwin binary; an attacker
  with the Authenticode certificate can sign a malicious
  windows binary. The mitigation is the credentials' short
  lifetimes and their storage; the mitigation does not
  eliminate the risk.
- **`go install` users are second-class.** The module path is
  frozen, but the documentation ranks `go install` below the
  binary channels. A Go developer who wants the release's exact
  binary uses a package manager or the direct download; a `go
  install` build is the source's output, not the release's.
- **Two compatibility contracts must be maintained.** The module
  path's stability and the release's artifact naming are both
  contracts; a change to either is a MAJOR change. The project
  owns both.
- **The winget, Homebrew, Scoop, and apt channels each add a
  third-party dependency.** A channel whose maintainers change
  their review process, or whose manifest format changes, can
  break the release without a Forge commit.

### 4.3 Neutral

- **The module path is frozen by a companion ADR.** This ADR's
  decision implies the module path is a contract, but the path's
  exact value and its `/vN` suffix rule are the companion ADR's
  concern, not this one's.
- **The release pipeline's job graph is the companion ADR-048
  supply-chain decision's concern.** This ADR names "the pipeline
  produces the proofs"; the pipeline's exact jobs, their order,
  and their credentials are ADR-048's.
- **`go install` remains supported.** The decision does not
  remove it; it ranks it. A future change that removed it would
  be a MAJOR change to this ADR's consequences, and would need
  its own ADR.

---

## 5. Related Requirements

- `FR-REL-001` — The supported platform matrix is a frozen set;
  this ADR's decision is what makes the matrix a release
  concern rather than a source concern.
- `FR-REL-002` — The versioning scheme and release channels;
  this ADR names the binary channel as primary, and the
  versioning ADR names the channels the binary channel
  publishes to.
- `FR-REL-003` — The local-versus-release build separation;
  this ADR is what makes the separation necessary (a source
  project has only one build; a binary project has two).
- `FR-REL-004` — The reproducible cross-compilation pipeline;
  this ADR is what makes reproducibility a requirement.
- `FR-REL-005` — Version metadata injection; this ADR's binary
  channel is what makes the injected metadata user-visible.
- `FR-REL-006` — The binary verification smoke suite; this ADR
  is what makes the suite necessary.
- `FR-REL-007` — The release pipeline; this ADR is its
  justification.
- `FR-REL-008` — The distribution channels; this ADR names the
  set.
- `SEC-REL-001` through `SEC-REL-012` — The supply-chain proofs
  this ADR's decision requires.
- `NFR-PLATFORM-003` — The platform matrix's tiers; this ADR
  is what makes the tiers a release concern.
- `NFR-REPRODUCIBILITY-001`, `NFR-REPRODUCIBILITY-002` — The
  reproducibility requirements this ADR's decision imposes.
- `NFR-SECURITY-001` — The supply-chain security posture this
  ADR's decision establishes.
- `NFR-RELEASE-001`, `NFR-RELEASE-002` — The release process's
  requirements.
- `NFR-ADOPTION-001` — The adoption goal this ADR's decision
  serves.
- `NFR-TRUST-001` — The trust requirement this ADR's decision
  addresses.
- `FR-CLI-VERSION-001` — The version command's output contract;
  this ADR's decision is what makes the contract user-facing.

> The following requirements must be added to the register when
> WBS 1.4 is complete: `FR-REL-000` (the binary-channel decision
> itself, if the register does not already have it). The existing
> `FR-REL-001` through `FR-REL-008` cover the decision's
> consequences; the decision's own entry, if one is wanted, is
> `FR-REL-000`.

---

## 6. Related ADRs

- `ADR-041` — The supported platform matrix is a frozen six-platform
  set. This ADR's decision is what makes the matrix a release
  concern; the matrix ADR names the platforms the binary channel
  publishes to.
- `ADR-043` — Versioning and release channels. This ADR's binary
  channel publishes the three channels the versioning ADR names.
- `ADR-044` — Local and release builds are separate target
  families. This ADR's binary channel is what makes the
  separation necessary.
- `ADR-045` — Release builds use `CGO_ENABLED=0`. This ADR's
  cross-platform binary channel is what makes the `CGO_ENABLED=0`
  policy a release concern.
- `ADR-046` — Reproducibility posture. This ADR's binary channel
  is what makes reproducibility a user-facing requirement.
- `ADR-047` — Version metadata injection. This ADR's binary
  channel is what makes the injected metadata a release artifact.
- `ADR-048` — Supply-chain integrity. This ADR is what makes the
  supply-chain proofs a release concern; ADR-048 names the
  proofs.
- `ADR-049` — Distribution channels. This ADR names the binary
  channel as primary; ADR-049 names the six channels the binary
  channel publishes to.
- `ADR-050` — User verification surface. This ADR's binary
  channel is what makes the verification guide a user-facing
  artifact.

---

## 7. Notes

**Open questions this ADR defers:**

- **The exact module path and its `/vN` suffix rule.** This ADR
  names the module path as a contract; the path's value and its
  versioning rule are a companion ADR's concern. The companion
  ADR is not yet written; it should be drafted before the
  module path is published.
- **The signing credentials' acquisition.** This ADR's decision
  depends on the macOS notarization and Windows Authenticode
  credentials being available. If they are not available when
  the first public release is cut, the release's notes record
  the blocker and the release ships without the proof. The
  acquisition is a project-management task, not an ADR's;
  the ADR names the dependency.

**Follow-up decisions anticipated:**

- **The `go install` channel's documentation ranking.** This ADR
  ranks `go install` as a secondary channel. The installation
  guide's structure follows from the ranking; a future change
  to the ranking would change the guide.
- **The rollback procedure's shape.** This ADR's binary channel
  is what makes a rollback procedure necessary; the procedure's
  steps are the rollback guide's concern.
- **The reproducibility check's CI integration.** This ADR's
  decision is what makes the check a CI gate; the check's
  exact form is ADR-046's concern.

**Links to external discussion:**

- None yet. The decision's discussion is the ADR itself.

---

*End of ADR.*
