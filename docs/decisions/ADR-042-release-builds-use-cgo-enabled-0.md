# ADR-042: Release Builds Use CGO_ENABLED=0

**Status:** Proposed
**Date:** 2026-10-15
**Deciders:** [@thapelomagqazana]
**Supersedes:** —
**Superseded by:** —

---

## 1. Context

Go programs that use only the standard library's pure-Go packages
compile to static binaries with no external runtime dependency.
Programs that use `net`, `os/user`, `os/signal`, or any package
whose implementation calls into a C library depend on cgo's
presence to link against the platform's C runtime. The Go
toolchain's `CGO_ENABLED` environment variable selects whether
cgo is available: `CGO_ENABLED=1` enables it, `CGO_ENABLED=0`
disables it and forces the pure-Go implementations. The choice is
per-build, not per-project; a developer's local build and the
project's release build can make different choices, and the
consequences differ per platform.

Forge's distribution model makes the choice consequential. A
release build cross-compiles the binary for six platforms — three
operating systems, two architectures each — on a Linux CI runner.
Cross-compiling with cgo is difficult: the toolchain needs a C
cross-compiler for each target (`aarch64-linux-gnu-gcc`,
`x86_64-w64-mingw32-gcc`, `o64-clang` for darwin, and so on),
and the resulting binary links against the target's C runtime.
The linked runtime's version becomes a property of the binary;
a binary built against glibc 2.31 does not run on a system with
glibc 2.28. The cross-compile's C-toolchain setup is the
traditional reason a Go project gives up on cross-compilation and
builds on a per-platform runner instead.

The linked runtime's *version* is the second force. A
`CGO_ENABLED=1` binary on Linux is dynamically linked against
glibc. glibc is forward-compatible — a binary built against 2.28
runs on 2.31 — but not backward-compatible: a binary built
against 2.31 does not run on 2.28. The CI runner's glibc version
is not the user's. A user on an older Linux distribution sees
`GLIBC_2.34 not found` when they run a binary the pipeline built.
The failure is not a bug in Forge; it is a property of the
toolchain, and the pipeline must not produce it.

The third force is Alpine Linux and similar distributions that
use musl as the C library instead of glibc. A `CGO_ENABLED=1`
binary linked against glibc does not run on Alpine. The user's
recourse is to build from source with `CGO_ENABLED=0` or to
install `gcompat`, neither of which is the experience the
distribution model promises. The Forge binary's promise — one
binary per platform, runnable everywhere that platform is
supported — is a property only `CGO_ENABLED=0` can deliver.

The fourth force is reproducibility. A `CGO_ENABLED=1` build's
output depends on the C compiler's version, the C library's
version, the system headers' version, and the link order the
toolchain chose. The Go toolchain's own version is one input
among many; pinning it does not pin the rest. A
`CGO_ENABLED=0` build's output depends on the Go toolchain and
the Go sources; both are pinned. The difference is the difference
between a build that can be reproduced and one that cannot.

Forge's source currently uses no package whose implementation
requires cgo. `net` is the standard library's most common
cgo-triggering package; Forge's network use, if any, is HTTP
client calls that work with the pure-Go resolver. `os/user` is
not used. A future change could add a dependency — a SQLite
driver, an image library, a YAML parser that wraps libyaml —
whose implementation requires cgo. The policy this ADR sets is
not about the current source; it is about what a future change
must do when it wants to add such a dependency.

Prior decisions this ADR relates to: ADR-040 establishes that
Forge is distributed as a binary project, which is what makes
the binary's runnability a user-facing property. ADR-041
separates local and release builds, which is what lets the
policy apply to the release family and not to the developer's
local loop. The reproducibility posture ADR names the flags the
release build applies; `CGO_ENABLED=0` is one of them. The
supply-chain ADR's proofs attest the binary's bytes; the bytes
are only reproducible if the build's inputs are pinned, and
`CGO_ENABLED` is one of them.

The decision affects every release build the pipeline runs,
every user who downloads an archive, and every future
contributor who proposes a dependency that uses cgo. The
policy's enforcement is the build script's check; its
consequences are the release's runtime behavior on the six
platforms and on the distributions the platforms include.

---

## 2. Options Considered

### Option A — Release builds use `CGO_ENABLED=1`

Forge's release builds enable cgo. The pipeline builds each
platform on a runner that matches the platform (a Linux runner
for linux targets, a macOS runner for darwin targets, a Windows
runner for windows targets), or it installs cross-compilers on
a Linux runner. The resulting binaries dynamically link against
the platform's C runtime.

**Pros:**

- **The cgo-using packages are available.** A dependency whose
  implementation requires cgo — a SQLite driver, a system
  library binding, a package that uses `os/user`'s cgo path —
  compiles without the pure-Go fallback's limitations.
- **The platform's native resolver is used on Linux.** The
  `net` package's cgo path uses the system's NSS resolver,
  which honors `/etc/nsswitch.conf`, LDAP, mDNS, and other
  configured sources. The pure-Go path does not.
- **The platform's native certificate store is used on macOS
  and Windows.** The `crypto/x509` package's cgo path reads
  the system's trust store; the pure-Go path reads a
  compiled-in set. A user whose organization installs a
  custom CA sees the cgo path honor it.
- **Some third-party packages assume cgo.** A package whose
  documentation says "requires cgo" is one the cgo-enabled
  build can use directly; the cgo-disabled build needs a
  pure-Go alternative or a fork.

**Cons:**

- **Cross-compilation requires per-target C toolchains.** The
  CI's Linux runner needs `aarch64-linux-gnu-gcc`,
  `x86_64-w64-mingw32-gcc`, `o64-clang` with the macOS SDK,
  and each target's headers. Each is an installation step
  the pipeline must maintain; a toolchain's version change is
  a release-engineering change.
- **The binary's glibc version becomes a support burden.**
  A Linux binary built on Ubuntu 22.04 (glibc 2.35) does not
  run on Ubuntu 20.04 (glibc 2.31), Debian 11 (glibc 2.31),
  RHEL 8 (glibc 2.28), or Alpine (musl). The support matrix's
  minimum OS versions become the binary's glibc floor, and
  every older distribution is a user the binary cannot serve.
- **Alpine Linux users cannot run the binary.** Alpine's
  musl is not glibc; a dynamically-linked glibc binary fails
  to start. The user installs `gcompat` or builds from
  source. Both are workarounds the distribution model exists
  to avoid.
- **Reproducibility is not achievable.** The C compiler's
  version, the C library's version, the system headers'
  version, and the link order are inputs the pipeline does
  not pin. Two builds from the same Go source produce
  different binaries; the reproducibility ADR's promise does
  not hold.
- **The build is slower.** A cgo-enabled build invokes the C
  compiler for every package that uses cgo, and the C
  compiler's startup cost is per-package. The release
  pipeline's build time increases; the local developer's
  build time increases too if the local build also enables
  cgo.
- **The binary's static-analysis story is weaker.** A
  `CGO_ENABLED=0` binary is fully static; a
  `CGO_ENABLED=1` binary's dynamic dependencies must be
  inspected with `ldd` and enumerated. The verification
  suite's "is the binary static?" check becomes
  "is the binary's dynamic dependency list exactly what we
  expect?" — a harder check with more failure modes.

### Option B — Release builds use `CGO_ENABLED=0`

Forge's release builds disable cgo. The pipeline builds every
platform on a single Linux runner; the Go toolchain's
cross-compilation support produces each platform's binary from
the same source. The resulting binaries are static: no C
runtime dependency, no glibc version constraint, no musl/glibc
distinction.

**Pros:**

- **Cross-compilation works from one runner.** The Go
  toolchain's `GOOS` and `GOARCH` environment variables
  select the target; no C cross-compiler is needed. The CI's
  build job is one runner, one Go toolchain, six `go build`
  invocations.
- **The binary runs on every supported Linux distribution.**
  A static binary has no glibc version dependency; it runs
  on Alpine, on RHEL 8, on Ubuntu 20.04, and on every
  distribution the platform matrix names. The matrix's
  minimum OS versions are the OS versions the binary's
  syscalls require, not the glibc versions the builder
  happened to have.
- **The binary is reproducible.** The Go toolchain and the Go
  sources are the binary's only inputs. Pinning the
  toolchain (via `go.mod`'s `toolchain` directive) and
  setting `SOURCE_DATE_EPOCH` are the pipeline's whole
  reproducibility setup. Two builds from the same commit
  produce identical bytes.
- **The build is fast.** No C compiler is invoked; the build
  is the Go compiler's cost only. The release pipeline's
  build time is bounded by the Go compiler's work, not by
  the C compiler's startup costs.
- **The verification suite's "is the binary static?" check is
  trivial.** `ldd` on a static Linux binary prints "not a
  dynamic executable"; the check is one line. A
  dynamically-linked binary's dependency list would need
  per-platform expected values, and each expected value
  would be a maintenance item.
- **The dependency policy is clear.** A dependency that
  requires cgo is rejected; the contributor proposes a
  pure-Go alternative or a fork. The policy is one rule
  with one exception path (an ADR), which is easier to
  enforce than a policy with per-dependency judgment calls.

**Cons:**

- **Some dependencies are unavailable.** A dependency whose
  implementation requires cgo — a SQLite driver that wraps
  libsqlite3, an image library that wraps libpng, a YAML
  parser that wraps libyaml — cannot be used. The
  contributor must find a pure-Go alternative or accept the
  dependency's absence.
- **The pure-Go resolver is used on Linux.** The `net`
  package's pure-Go path reads `/etc/resolv.conf` directly
  and does not honor `/etc/nsswitch.conf` or NSS plugins.
  A user whose DNS resolution depends on LDAP, mDNS, or a
  custom NSS module sees the pure-Go path fail where the
  cgo path would succeed. The failure is rare and the
  pure-Go path is the default for most users; the cgo
  path's users are a minority with a specific
  configuration.
- **The macOS and Windows certificate stores are not read
  by default on older Go versions.** Go 1.18 and later read
  the system's trust store on macOS and Windows without
  cgo, using the platform's native APIs. Go versions older
  than 1.18 read a compiled-in set. Forge pins a Go version
  newer than 1.18, so the issue does not apply; a future
  change that lowered the pinned version would reintroduce
  it.
- **A future dependency that requires cgo needs an ADR.**
  The policy is one rule with one exception path; the
  exception is expensive. The contributor who wants the
  dependency must write an ADR that answers why the
  dependency's value exceeds the policy's cost.

### Option C — Release builds use `CGO_ENABLED=0` on Linux and `CGO_ENABLED=1` on macOS and Windows

Forge's Linux release builds disable cgo; the macOS and Windows
release builds enable it. The policy splits by platform. The
Linux builds get the static binary and the Alpine
compatibility; the macOS and Windows builds get the native
resolver and trust store paths.

**Pros:**

- **Linux users get the static binary.** Alpine, RHEL 8, and
  the other older distributions can run Forge.
- **macOS and Windows users get the native paths.** The
  resolver and the certificate store are the platform's
  native ones.
- **The cross-compilation burden is reduced.** The Linux
  builds are cross-compiled from a Linux runner with
  `CGO_ENABLED=0`; the macOS and Windows builds run on
  their own runners with cgo enabled. The per-target C
  toolchain setup is needed only for the two non-Linux
  platforms' runners, which the CI matrix already has.

**Cons:**

- **The binary's reproducibility is platform-dependent.**
  The Linux binary is reproducible; the macOS and Windows
  binaries are not, because their cgo paths depend on the
  platform's C toolchain. The reproducibility promise
  holds for Linux and not for the other two platforms.
- **The user's expectation is platform-dependent.** A user
  who reads "Forge builds are reproducible" and rebuilds
  on macOS sees a different binary; a user who reads the
  same and rebuilds on Linux sees an identical one. The
  documentation must name the platform split, which is a
  caveat the simpler policies do not have.
- **The policy is two rules, not one.** A contributor who
  proposes a cgo-using dependency must reason about which
  platforms' builds can use it. A dependency usable on
  macOS but not Linux is a dependency the policy admits
  partially; the contributor's reasoning is
  platform-by-platform, which is more complex than the
  single rule's binary answer.
- **The Alpine compatibility is preserved on Linux only.**
  A macOS user who runs Forge inside a Linux container
  (Docker, a VM) uses the container's Linux binary, which
  is static. A macOS user who runs Forge on macOS uses the
  cgo-linked macOS binary, which is not. The two
  experiences' difference is invisible to the user until a
  failure occurs.
- **The verification suite's checks are platform-dependent.**
  The "is the binary static?" check applies to Linux only;
  the macOS and Windows checks must enumerate the dynamic
  dependencies instead. The suite's shape is not uniform
  across the six platforms.

---

## 3. Decision

Forge's **release builds use `CGO_ENABLED=0` on every
platform**. The release pipeline sets `CGO_ENABLED=0`
unconditionally; the build script verifies the value before
invoking the Go compiler, and the verification suite confirms
each Linux binary is static. A dependency that requires cgo is
not usable in a release build; a contributor who proposes one
must write an ADR that answers why the dependency's value
exceeds the policy's cost, and the ADR is the exception's
authorization. The decision applies to the release family of
builds only; a developer's local build may set `CGO_ENABLED=1`
or `CGO_ENABLED=0` as the developer's environment selects.

The decision was tipped by three forces whose combination has
no alternative. Cross-compilation from a single runner requires
`CGO_ENABLED=0`; the Go toolchain's cross-compilation support
is the pure-Go path's. The binary's runnability on every
supported Linux distribution requires `CGO_ENABLED=0`; the
glibc-version constraint is a property of the cgo path. The
binary's reproducibility requires `CGO_ENABLED=0`; the C
toolchain's inputs are not the pipeline's to pin. The three
forces point the same direction, and the option that satisfies
all three is the option that satisfies each.

The decision accepts the pure-Go resolver's limitations
knowingly. A Linux user whose DNS resolution depends on a
non-default NSS module sees the pure-Go path fail; the user's
recourse is to use a resolver configuration the pure-Go path
honors, or to build Forge from source with `CGO_ENABLED=1`
for their own use. The decision does not remove the user's
option; it removes the release binary's dependence on a
configuration the majority of users do not have.

The decision accepts the cgo-using dependencies' absence
knowingly. A future contributor who wants a dependency that
requires cgo — a SQLite driver, an image library, a YAML
parser — must find a pure-Go alternative or write the
exception ADR. The policy's cost is the contributor's
research time; the policy's benefit is the release binary's
properties. The tradeoff is real, and the decision accepts
it.

The decision depends on two assumptions that could be
falsified:

- **Forge's source does not require a cgo-using dependency.**
  The current source uses no package whose implementation
  requires cgo. A future change that added one would trigger
  the exception path; the assumption is that no such change
  is needed.
- **The pure-Go resolver is sufficient for Forge's users.**
  The `net` package's pure-Go path is the default for most
  Go programs and most users. A user whose DNS resolution
  requires NSS is a minority; the decision accepts that the
  release binary does not serve them, and their recourse is
  a source build.

The decision is not provisional. A future change that
reversed it — "Forge's release builds use `CGO_ENABLED=1`
on some platforms" — would be a new ADR that supersedes this
one, and would need to answer which of the three forces it
sacrifices.

---

## 4. Consequences

### 4.1 Positive

- **Cross-compilation works from a single Linux runner.** The
  release pipeline's build job is one runner, one Go
  toolchain, six `go build` invocations. No C cross-compiler
  is installed; no per-platform toolchain is maintained.
- **The Linux binary runs on every supported distribution.**
  Alpine, RHEL 8, Debian 11, Ubuntu 20.04, and every other
  distribution the platform matrix names can run the
  binary. The glibc-version constraint does not exist.
- **The binary is reproducible.** Two builds from the same
  commit produce identical bytes. The reproducibility
  ADR's promise holds for every platform, not a subset.
- **The build is fast.** The Go compiler is the only
  compiler invoked; the C compiler's startup costs are
  absent. The release pipeline's build time is bounded by
  the Go compiler's work.
- **The verification suite's static-binary check is one
  line.** `ldd` on a Linux binary prints "not a dynamic
  executable"; the check asserts the output. A
  dynamically-linked binary's dependency list would need
  per-platform expected values.
- **The dependency policy is one rule.** A cgo-using
  dependency is rejected; the exception path is an ADR.
  The contributor's research time is the policy's cost;
  the policy's answer is binary.
- **The binary's size is smaller.** A static Go binary
  links only the Go runtime and the packages it imports;
  a cgo-enabled binary links the C runtime and its
  dependencies. The size difference is measurable on
  Linux; the platform matrix's size budget has more
  headroom.

### 4.2 Negative

- **A cgo-using dependency cannot be used.** A SQLite driver
  that wraps libsqlite3, an image library that wraps
  libpng, a YAML parser that wraps libyaml — each is
  unavailable. The contributor must find a pure-Go
  alternative or write the exception ADR.
- **The pure-Go resolver is used on Linux.** The `net`
  package's pure-Go path does not honor `/etc/nsswitch.conf`
  or NSS plugins. A user whose DNS resolution depends on
  LDAP, mDNS, or a custom NSS module sees the pure-Go path
  fail where the cgo path would succeed. The user's
  recourse is a source build with `CGO_ENABLED=1`.
- **The macOS and Windows binaries' certificate handling
  depends on the Go version.** Go 1.18 and later read the
  system trust store without cgo; older versions do not.
  Forge pins a Go version newer than 1.18, so the issue
  does not apply today; a future change that lowered the
  pinned version would reintroduce it.
- **A contributor who needs a cgo-using dependency pays an
  ADR's cost.** The exception path is an ADR; the ADR's
  author must answer why the dependency's value exceeds
  the policy's cost. The cost is the contributor's time and
  the reviewer's attention.
- **The pure-Go resolver's limitations are invisible until
  they matter.** A user with a standard resolver
  configuration never sees the difference; a user with a
  non-standard configuration sees a failure whose cause is
  not obvious. The failure's diagnosis requires reading
  the Go documentation's `net` package section and
  correlating the user's configuration with the pure-Go
  path's behavior.
- **The binary's behavior may differ from a
  `CGO_ENABLED=1` build's.** A user who builds Forge from
  source with `CGO_ENABLED=1` gets a binary whose resolver
  and certificate handling use the cgo path; the user's
  bug reports against that binary describe behavior the
  release binary does not have. The two binaries' shapes
  differ, and the difference is not visible in `forge
  version`'s output.
- **A future platform that requires cgo cannot be added
  without an exception.** A hypothetical platform whose Go
  port requires cgo — a mobile platform, a WASM target
  with cgo-specific bindings — would need the exception
  ADR before it could be added to the matrix. The policy
  is a gate on the matrix's growth.

### 4.3 Neutral

- **The local build's cgo setting is the developer's
  choice.** A developer who wants the cgo path locally sets
  `CGO_ENABLED=1`; a developer who wants the release's
  shape sets `CGO_ENABLED=0`. The Makefile's development
  family does not set the variable; the developer's
  environment does. The choice is the developer's, not the
  project's.
- **The `go install` channel's cgo setting is the user's
  choice.** A user who runs `go install
  github.com/forge-cli/forge/cmd/forge@latest` gets a
  binary built with the user's `CGO_ENABLED` setting. The
  binary's properties differ from the release binary's if
  the setting differs. The documentation names the
  difference; the user's choice is the user's.
- **The policy's enforcement is the build script's check,
  not the Go toolchain's.** The Go toolchain accepts
  `CGO_ENABLED=1` and builds a cgo-enabled binary; the
  pipeline's build script verifies the variable before
  invoking the compiler. A future change that removed the
  check would let a `CGO_ENABLED=1` build through; the
  verification suite's static-binary check would catch it,
  but the check's position is downstream of the build.
- **The `net` package's behavior in a `CGO_ENABLED=0` build
  is documented by the Go project.** The Go documentation's
  `net` package section names the pure-Go path's
  limitations. The pipeline's documentation need not repeat
  them; it can link to the Go documentation and name the
  user's recourse.

---

## 5. Related Requirements

- `FR-REL-004` — The reproducible cross-compilation
  pipeline; this ADR's decision is what makes the
  cross-compilation possible from one runner and the
  reproducibility achievable.
- `FR-REL-001` — The platform matrix; this ADR's decision
  is what lets the matrix's six platforms be built from
  one runner.
- `FR-REL-003` — The local-versus-release build separation;
  this ADR's policy applies to the release family, not the
  local family.
- `NFR-PLATFORM-003` — The platform matrix's tiers; this
  ADR's decision is what makes the Linux binary runnable
  on every supported distribution, including Alpine.
- `NFR-REPRODUCIBILITY-001`, `NFR-REPRODUCIBILITY-002` —
  The reproducibility requirements; this ADR's decision
  is what makes them achievable.
- `NFR-BUILD-001` — The build's requirements; this ADR's
  decision is one of the build's policies.
- `NFR-SECURITY-001` — The supply-chain security posture;
  this ADR's decision is what makes the binary's
  reproducibility a property the proofs can attest to.
- `SEC-REL-001` through `SEC-REL-012` — The supply-chain
  proofs; this ADR's decision is what makes their
  attestation meaningful.
- `FR-REL-005` — Version metadata injection; this ADR's
  decision does not affect the injection's fields, but
  the version command's output is what a user reads to
  learn the binary's shape.
- `NFR-QUALITY-013` — The binary verification suite; this
  ADR's decision is what makes the suite's static-binary
  check one line.

> The following requirement must be added to the register
> when WBS 1.4 is complete: `NFR-BUILD-002` (the
> cgo-disabled policy). The requirement would name the
> policy and its exception path; the existing `NFR-BUILD-001`
> covers the build's general requirements.

---

## 6. Related ADRs

- `ADR-040` — Forge is distributed as a binary project.
  This ADR's decision is what makes the binary's
  runnability a user-facing property; the binary-channel
  decision is what makes the policy necessary.
- `ADR-041` — Local and release builds are separate target
  families. This ADR's policy applies to the release
  family; the separation is what lets the local family
  choose its own `CGO_ENABLED` setting.
- `ADR-043` — The supported platform matrix is a frozen
  six-platform set. This ADR's decision is what makes the
  matrix's platforms buildable from one runner and
  runnable on every distribution the matrix names.
- `ADR-046` — Reproducibility posture. This ADR's decision
  is one of the posture's flags; the posture ADR names the
  others.
- `ADR-047` — Version metadata injection. This ADR's
  decision does not affect the injection's fields; the
  two are independent.
- `ADR-048` — Supply-chain integrity. This ADR's decision
  is what makes the binary's bytes reproducible; the
  supply-chain ADR's proofs attest the bytes.
- `ADR-045` — Release builds use `-trimpath`. This ADR's
  decision is a companion policy; both are release-build
  flags the pipeline applies unconditionally.

---

## 7. Notes

**Open questions this ADR defers:**

- **The exception path's exact form.** A contributor who
  wants a cgo-using dependency writes an ADR. The ADR's
  shape is the decisions template's; the exception's
  substance is the contributor's. The path's exact
  procedure — who reviews the exception, how the pipeline
  branches on it — is a follow-up decision.
- **The pure-Go resolver's user-facing documentation.** A
  user whose DNS resolution requires NSS sees the pure-Go
  path fail. The user's recourse is a source build; the
  documentation's exact wording is a follow-up decision.
- **The macOS and Windows certificate handling's Go-version
  dependency.** Go 1.18 and later read the system trust
  store without cgo; older versions do not. Forge pins a
  Go version newer than 1.18; a future change that lowered
  the pinned version would need this ADR's reconsideration.

**Follow-up decisions anticipated:**

- **The build script's enforcement check.** The build
  script verifies `CGO_ENABLED=0` before invoking the Go
  compiler. The check's exact form — a case statement, an
  environment variable's comparison — is the build
  script's concern.
- **The verification suite's static-binary check.** The
  suite runs `ldd` on each Linux binary and asserts the
  output. The check's exact form is the suite's concern.
- **The exception path's ADR template's extension.** A
  cgo-using dependency's exception ADR may need additional
  sections — the dependency's value, the policy's cost,
  the alternative's unavailability. The template's
  extension is a follow-up decision.
- **The pure-Go path's DNS behavior's test.** A test that
  asserted the pure-Go resolver's behavior would catch a
  future change that switched the resolver's mode. The
  test's shape is a follow-up decision.

**Links to external discussion:**

- The Go documentation's `net` package section names the
  pure-Go resolver's limitations and the cgo path's
  behavior.
- The Go documentation's `cmd/cgo` section names the
  `CGO_ENABLED` variable's effect and the cross-compile
  requirements.

---

*End of ADR.*
