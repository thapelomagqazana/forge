# ADR-041: Local and Release Builds Are Separate Target Families

**Status:** Proposed
**Date:** 2026-10-15
**Deciders:** [@thapelomagqazana]
**Supersedes:** —
**Superseded by:** —

---

## 1. Context

Forge is a Go project. A developer who clones the repository
and runs `go build ./...` gets a working binary. That path exists
today, is fast, and is what a developer uses while writing code.
The question this ADR answers is what the project's *other* build
is: the one that produces the binary a user downloads. The two
builds have different goals, and a single target that tries to
serve both serves neither.

A local build's goal is iteration speed. A developer changes a
file, runs the build, and runs the binary. The build's inputs are
the developer's working tree, which is usually dirty; its
toolchain is the developer's own Go, which may be newer or older
than the project's pinned version; its output is a binary on the
developer's machine that no one else will run. The build does not
need to be reproducible, cross-compiled, signed, or
version-stamped with anything more than `dev`.

A release build's goal is reproducibility. The build runs in CI,
on a runner that the project controls, from a commit that the
project has tagged. Its inputs must be pinned; its output must be
byte-for-byte identical across runs; its artifacts must be
checksummed, signed, and attested before they are published. The
build does not need to be fast; it needs to be the same every
time.

The two builds' requirements are in direct tension. A local build
that pins the toolchain wastes a developer's time when the
developer's newer Go is fine. A release build that skips
`-trimpath` embeds the developer's home directory path in the
binary, which is both a privacy leak and a reproducibility
failure. A local build that fails on a dirty tree stops a
developer from iterating. A release build that runs from a dirty
tree produces an artifact that cannot be traced to a commit.

The force at play is that the project has both audiences, and
each audience's build must be correct for that audience. If the
project has one build target, the target's behavior is
over-determined: it satisfies the release requirements only by
being slow and strict for the developer, or it satisfies the
developer's requirements only by being unreproducible for the
release.

Prior decisions this ADR relates to: ADR-040 establishes that
Forge is distributed as a binary project, which is what makes
the release build user-facing rather than an internal step. The
platform-matrix ADR names the six platforms the release build
must produce; the versioning ADR names the channels the release
build publishes to. The reproducibility ADR names the flags the
release build must apply. This ADR's decision is what makes
those companion decisions expressible as separate targets rather
than as a single over-loaded one.

The decision affects every developer who runs `make build` and
every release engineer who runs `make release`. It also affects
the CI pipeline, which runs the release build on every tag, and
the local development loop, which runs the local build on every
edit.

---

## 2. Options Considered

### Option A — One build target for both audiences

Forge has a single `make build` target. The target applies the
release pipeline's flags unconditionally: it pins the toolchain,
sets `SOURCE_DATE_EPOCH`, applies `-trimpath`, injects the
version from `git describe`, and fails on a dirty tree.

**Pros:**

- **One target to learn.** A contributor reads one command and
  knows how to build the project.
- **The build is always release-shaped.** A developer who runs
  `make build` and then hands the binary to a colleague hands
  a release-shaped artifact, not a local one.
- **No drift between the two shapes.** The build's flags cannot
  diverge from the release pipeline's because there is only one
  set of flags.
- **The CI pipeline uses the same target as the developer.**
  A change to the build is exercised by every developer's local
  build before it reaches CI.

**Cons:**

- **The developer's loop is slow.** A pinned toolchain may
  download a Go version the developer does not have; a
  `SOURCE_DATE_EPOCH` computation runs `git log` on every
  build; the dirty-tree check fails on every edit. The
  iteration cost is paid on every keystroke-to-binary cycle.
- **A dirty tree stops the developer.** A developer who wants
  to test a one-line change must commit or stash first. The
  friction is real and constant.
- **The version string is wrong for local use.** A local build
  that injects the last tag's version produces a binary whose
  `forge version` reports a version the developer is not
  running. A bug report against the local binary confuses the
  version's meaning.
- **The developer must install the release toolchain.** A
  developer who has Go 1.22.6 and the project pins Go 1.22.7
  sees the build download the pinned toolchain. The download
  is a one-time cost per toolchain version, but it is a cost
  the developer did not ask for.
- **The build's strictness is the wrong strictness.** A
  release build must fail on a dirty tree; a local build must
  not. One target cannot have both behaviors without a flag,
  and a flag that selects the behavior is the two-target
  design in disguise.

### Option B — Two target families, one per audience

Forge has two target families. The development family (`make
build`, `make test`, `make check`, `make clean`, `make run`)
serves the local loop: it builds for the host platform only, uses
the developer's Go, does not pin the toolchain, does not require
a clean tree, and injects the version `dev`. The release family
(`make release`, `make release-dry-run`, `make build-all`, `make
checksums`, `make sign`, `make sbom`) serves the release: it
cross-compiles six platforms, pins the toolchain, sets
`SOURCE_DATE_EPOCH`, applies `-trimpath` and `-buildvcs`, fails on
a dirty tree, and produces signed, checksummed artifacts.

**Pros:**

- **Each family is correct for its audience.** The development
  family is fast and forgiving; the release family is strict
  and reproducible. A developer who wants the release shape
  runs the release family's dry-run target.
- **The developer's loop stays fast.** The local build uses the
  developer's Go, does not compute `SOURCE_DATE_EPOCH`, and
  does not check the tree's cleanliness. The iteration cost is
  the compile's cost, not the release pipeline's.
- **A dirty tree is a local-build input, not an error.** A
  developer edits, builds, and runs without committing. The
  release family's clean-tree check is the release's, not the
  developer's.
- **The version string is honest for each.** The local build
  reports `dev`; the release build reports the tag. A bug
  report against a `dev` binary is understood to be a bug
  report against an unreleased build.
- **The release family's flags are testable in isolation.**
  The reproducibility flags' effects are tested against the
  release family's output; a developer who changes a
  development target cannot accidentally change the release
  shape.
- **The CI pipeline's behavior is explicit.** The CI runs the
  release family's targets; a developer who runs `make build`
  runs a different, faster target. The two are named
  differently, and a reader can tell which is which from the
  target's name.

**Cons:**

- **Two target families to learn.** A contributor reads the
  Makefile and sees two sets of targets. The Makefile's
  comments must name each family's purpose and its audience.
- **The two families' flags can drift.** A change to the
  release pipeline's flags that is not mirrored in the
  development family (or vice versa) is a defect the
  Makefile's review must catch. A future test that asserted
  the two families' shared flags match would catch the drift;
  the current design relies on the reviewer.
- **A developer who wants the release shape must know which
  target to run.** The release family's `release-dry-run`
  target is the one; a developer who runs `make build` and
  expects the release shape is surprised. The Makefile's
  documentation names the target; the developer's expectation
  is the developer's responsibility.
- **The two families' outputs live in different places.** The
  development family's binary is at `./forge` or `./bin/forge`;
  the release family's artifacts are in `dist/`. The two
  directories' purposes must be documented so a developer
  does not confuse them.
- **The release family's targets can be run locally, which
  some developers will do by accident.** A developer who runs
  `make release-dry-run` on a laptop pays the cross-compile
  cost and the toolchain-download cost. The target's name
  does not prevent the mistake; the documentation warns
  against it.

### Option C — One target with a `RELEASE=1` environment variable

Forge has a single `make build` target whose behavior is
selected by a `RELEASE` environment variable. When `RELEASE` is
unset or `0`, the build is the local shape; when `RELEASE` is
`1`, the build is the release shape. The two shapes' flags are
branches in the same recipe.

**Pros:**

- **One target name.** A developer runs `make build`; the CI
  runs `RELEASE=1 make build`. The two invocations are
  visibly the same target with different inputs.
- **The shared parts of the two shapes cannot drift.** The
  target's recipe is one place; a change to the common part is
  a change to both shapes.
- **The two shapes' differences are visible in one place.**
  A reader sees the `if RELEASE` branches and knows exactly
  what differs.

**Cons:**

- **The environment variable is invisible in the command.**
  A developer who runs `RELEASE=1 make build` sees a longer
  command; a developer who runs `make build` sees the short
  one. The environment variable is easy to forget, and its
  absence is silent: the build succeeds, but with the local
  shape.
- **The recipe's branches are harder to read.** A recipe with
  conditionals is longer than a recipe without; a reader must
  trace both branches to understand the target's behavior.
- **A misconfigured environment produces the wrong shape.**
  A CI job that forgets `RELEASE=1` produces a local-shaped
  binary in the release pipeline. The failure is silent: the
  binary exists, it runs, and it does not have the release's
  flags. The release's proofs are absent or wrong.
- **The target's behavior depends on state outside its
  arguments.** A Make target's inputs are its prerequisites
  and its recipe's arguments; an environment variable is an
  implicit input. The implicit input is the source of the
  "forgot to set it" failure mode.
- **The two audiences' targets cannot be named separately.**
  A developer who wants the release shape cannot type a
  target that means "release shape"; the developer must
  remember the environment variable. The two-target design's
  `make release-dry-run` names the intent; the
  environment-variable design does not.

---

## 3. Decision

Forge has **two target families**, one per audience. The
development family (`make build`, `make test`, `make check`,
`make clean`, `make run`) serves the local loop and is fast and
forgiving: it builds for the host platform, uses the developer's
Go, does not pin the toolchain, and injects the version `dev`. The
release family (`make release`, `make release-dry-run`, `make
build-all`, `make checksums`, `make sign`, `make sbom`) serves the
release and is strict and reproducible: it cross-compiles six
platforms, pins the toolchain, sets `SOURCE_DATE_EPOCH`, applies
`-trimpath` and `-buildvcs`, fails on a dirty tree, and produces
signed, checksummed artifacts. The two families share the
project's source and its `go.mod`; they differ in every flag that
matters.

The decision was tipped by the tension the context names. A
single target's behavior is over-determined: it satisfies one
audience only by failing the other. The environment-variable
design moves the selection to a string a user must remember; the
two-target design moves it to the target's name, which the user
types. The target's name is the more visible input, and the
failure mode — "the developer ran `make build` and got the local
shape" — is not a failure at all; it is the developer's intent.

The decision accepts the two-target design's costs knowingly.
The two families' flags can drift; the Makefile's review and,
in a future change, a test that asserted the two families' shared
flags match, are the mitigations. A developer who wants the
release shape must know which target to run; the Makefile's
comments and the building guide name the target. The release
family's targets can be run locally; the documentation warns
against it, and a developer who runs them pays the cost once.

The decision depends on two assumptions that could be falsified:

- **The two families' shared flags do not drift.** A change to
  the release pipeline's `-trimpath` that is not mirrored in
  the development family's build would produce a developer
  binary with the developer's home path embedded. The
  drift is possible; the mitigation is a test, and the test
  is a follow-up decision (see §7).
- **The developers' Go versions are compatible with the
  project's source.** The development family uses the
  developer's Go; a developer on a Go version older than the
  source requires sees a compile failure. The failure is the
  developer's to resolve; the pinned toolchain in `go.mod`
  makes the failure's message name the version the release
  uses.

The decision is not provisional. A future change that reversed
it — "Forge has one build target" — would be a new ADR that
supersedes this one, and would need to answer which audience's
requirements the target satisfies and which it sacrifices.

---

## 4. Consequences

### 4.1 Positive

- **Each audience's build is correct for that audience.** The
  developer's loop is fast; the release pipeline is
  reproducible. Neither is compromised for the other.
- **The version string is honest for each build.** A `dev`
  binary's `forge version` reports `dev`; a release binary's
  reports the tag. A bug report's version field is meaningful
  in both cases.
- **A dirty tree is a local-build input, not an error.** A
  developer iterates without committing; the release family's
  clean-tree check is the release's, not the developer's.
- **The release pipeline's behavior is explicit.** The CI runs
  the release family's targets; a reader who sees `make
  release-dry-run` in the CI's log knows exactly which build
  ran.
- **The reproducibility flags' effects are isolated.** A
  change to a development target cannot accidentally change
  the release shape, because the two are different targets
  with different recipes.
- **A developer who wants the release shape can get it.** The
  release family's `release-dry-run` target produces the full
  artifact set without publishing; a developer who wants to
  test the release pipeline before the CI runs it can run the
  target locally.
- **The `make check` target's meaning is unambiguous.** The
  check target runs the development family's tests and the
  static analysis, not the release pipeline's cross-compile;
  a developer who wants the release's checks runs the release
  family's targets.

### 4.2 Negative

- **The Makefile is longer.** Two target families mean more
  targets and more comments. A reader who opens the Makefile
  for the first time must read the comments to tell the two
  families apart.
- **The two families' shared flags can drift.** A change to
  `-trimpath` in one family that is not mirrored in the other
  is a defect the Makefile's review must catch. The drift's
  failure mode — a developer binary with the developer's home
  path embedded — is a privacy leak, not a functional defect;
  the drift is easy to miss.
- **A developer who wants the release shape must know which
  target to run.** The two-target design's names are
  `build` and `release-dry-run`; a developer who runs
  `build` and expects the release shape is surprised. The
  Makefile's comments and the building guide name the
  targets; the developer's expectation is the developer's
  responsibility.
- **The release family's targets can be run locally by
  accident.** A developer who runs `make release` on a laptop
  pays the cross-compile cost and the toolchain-download
  cost. The target's name does not prevent the mistake; the
  documentation warns against it.
- **The two families' outputs live in different places.** The
  development family's binary is at `./forge`; the release
  family's artifacts are in `dist/`. A developer who looks
  for the release's archive and finds the development
  family's binary must read the Makefile to learn the two
  directories' purposes.
- **A future `go install` user and a future `make build` user
  get different binaries.** The `go install` channel builds
  from the module's source with the user's toolchain; the
  `make build` target builds from the developer's clone with
  the developer's toolchain. The two binaries' `forge
  version` outputs differ (`vX.Y.Z` from a tag versus `dev`
  from a clean clone); the two are the same shape, but their
  version strings differ. A user who files a bug against a
  `go install`-built binary and a developer who reproduces
  with `make build` see different version strings.

### 4.3 Neutral

- **The `make build` target's output path is a convention.**
  The development family's binary is at `./forge` on Unix and
  `./forge.exe` on Windows; a future change that moved it to
  `./bin/forge` would be a change to the target's recipe, not
  to this ADR. The convention's choice is the Makefile's
  concern.
- **The `make clean` target removes both families' outputs.**
  The clean target is the one target that serves both
  audiences. Its recipe removes `./forge`, `./forge.exe`, and
  `dist/`; the union is the target's contract.
- **The release family's `build-all` target produces the
  archives without signing them.** The target's output is
  the six archives in `dist/`; the signing targets read them
  and add the proofs. A developer who wants to inspect the
  archives without signing them runs `build-all`; a
  developer who wants the full release shape runs
  `release-dry-run`.

---

## 5. Related Requirements

- `FR-REL-003` — The local-versus-release build separation;
  this ADR is the decision the requirement names.
- `FR-REL-004` — The reproducible cross-compilation pipeline;
  this ADR's release family is what the requirement's
  pipeline implements.
- `FR-REL-005` — Version metadata injection; this ADR's
  separation is what makes the two version strings (`dev` and
  the tag) expressible.
- `FR-REL-001` — The platform matrix; this ADR's release
  family is what cross-compiles the matrix's six platforms.
- `FR-REL-002` — The versioning scheme; this ADR's release
  family is what injects the version the scheme names.
- `NFR-BUILD-001` — The build's requirements; this ADR is
  the build's shape.
- `NFR-REPRODUCIBILITY-001`, `NFR-REPRODUCIBILITY-002` — The
  reproducibility requirements; this ADR's release family is
  what enforces them.
- `NFR-QUALITY-013` — The binary verification suite; this
  ADR's release family is what the suite's targets run
  against.
- `SEC-REL-001` — The supply-chain integrity requirement;
  this ADR's release family is what the requirement's proofs
  attach to.
- `FR-CLI-VERSION-001` — The version command's output
  contract; this ADR's version-string separation is what
  makes the contract meaningful for both audiences.

> The following requirement must be added to the register when
> WBS 1.4 is complete: `NFR-BUILD-002` (the local build's speed
> target). The requirement would name a threshold — for
> example, "`make build` completes in under 10 seconds on a
> developer's laptop" — which the development family's recipe
> must meet. The threshold is a follow-up decision (see §7).

---

## 6. Related ADRs

- `ADR-040` — Forge is distributed as a binary project. This
  ADR's release family is the build the binary channel
  publishes; the binary-channel decision is what makes the
  release family user-facing.
- `ADR-042` — The supported platform matrix is a frozen
  six-platform set. This ADR's release family is what
  cross-compiles the matrix's platforms.
- `ADR-043` — Versioning and release channels. This ADR's
  release family is what injects the version the scheme
  names; the versioning ADR names the three channels the
  release family's artifacts publish to.
- `ADR-045` — Release builds use `CGO_ENABLED=0`. This ADR's
  release family is what the `CGO_ENABLED=0` policy applies
  to; the local family is not subject to the policy, because
  a local build is not cross-compiled.
- `ADR-046` — Reproducibility posture. This ADR's release
  family is what the posture's flags apply to; the local
  family does not apply them.
- `ADR-047` — Version metadata injection. This ADR's
  separation is what makes the two version strings
  expressible; the injection ADR names the fields each
  string appears in.
- `ADR-048` — Supply-chain integrity. This ADR's release
  family is what the supply-chain proofs attach to; the
  local family has no proofs to attach.

---

## 7. Notes

**Open questions this ADR defers:**

- **The development family's exact targets.** This ADR names
  `build`, `test`, `check`, `clean`, and `run`. A future
  change that added a development target (for example,
  `make bench` or `make cover`) would extend the family
  without a change to this ADR. The target set's shape is
  the Makefile's concern.
- **The release family's exact targets.** This ADR names
  `release`, `release-dry-run`, `build-all`, `checksums`,
  `sign`, and `sbom`. A future change that split one of the
  targets or added a new one (for example, `make provenance`
  or `make notarize`) would extend the family without a
  change to this ADR.
- **The `make check` target's scope.** The check target runs
  the development family's tests and the static analysis.
  A future change that made check run the release family's
  cross-compile would blur the two families' separation;
  the change would need an ADR.

**Follow-up decisions anticipated:**

- **The drift-check test.** A test that asserts the two
  families' shared flags match would catch the drift the
  negative consequences name. The test is a follow-up
  decision; its shape is the test's concern.
- **The local build's speed target.** The build's speed is
  the development family's primary goal; a threshold would
  make the goal testable. The threshold is a follow-up
  decision; it belongs in the requirements register (see
  §5's pending requirement).
- **The `dist/` directory's contents.** The release family's
  artifacts live in `dist/`; the directory's exact contents
  (the archives, their proofs, the SBOMs) are the
  supply-chain ADR's concern. This ADR names the directory's
  existence, not its contents.
- **The `make clean` target's behavior on a failed build.**
  A build that fails partway may leave partial artifacts in
  `dist/`. The clean target's contract says it removes
  `dist/` entirely; a future change that made clean
  preserve some artifacts would need an ADR.

**Links to external discussion:**

- None yet. The decision's discussion is the ADR itself.

---

*End of ADR.*
